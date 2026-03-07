package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/rishav1305/soul/internal/planner"
	"github.com/rishav1305/soul/internal/products"
	"github.com/rishav1305/soul/internal/session"
)

// TaskProcessor handles autonomous task execution in the background.
type TaskProcessor struct {
	server      *Server
	products    *products.Manager
	sessions    *session.Store
	planner     *planner.Store
	broadcast   func(WSMessage)
	model       string
	projectRoot string
	worktrees   *WorktreeManager
	hooks       *HookRunner

	mu      sync.Mutex
	running map[int64]context.CancelFunc

	mergeMu    sync.Mutex     // serializes merge operations
	maxWorkers int            // max concurrent tasks
	workerSem  chan struct{}   // semaphore to limit concurrency

	listenerMu sync.Mutex
	listeners  map[int64][]func(string, string) // taskID -> callbacks(type, content)
}

// NewTaskProcessor creates a new autonomous task processor.
func NewTaskProcessor(srv *Server, pm *products.Manager, sessions *session.Store, plannerStore *planner.Store, broadcast func(WSMessage), model, projectRoot string, worktrees *WorktreeManager, maxWorkers int) *TaskProcessor {
	if maxWorkers <= 0 {
		maxWorkers = 2
	}
	if maxWorkers > 5 {
		maxWorkers = 5
	}
	return &TaskProcessor{
		server:      srv,
		products:    pm,
		sessions:    sessions,
		planner:     plannerStore,
		broadcast:   broadcast,
		model:       model,
		projectRoot: projectRoot,
		worktrees:   worktrees,
		hooks:       NewHookRunner(),
		running:     make(map[int64]context.CancelFunc),
		maxWorkers:  maxWorkers,
		workerSem:   make(chan struct{}, maxWorkers),
		listeners:   make(map[int64][]func(string, string)),
	}
}

// StartTask begins autonomous processing of a task in a background goroutine.
// If all worker slots are busy, the task goroutine blocks until a slot opens.
func (tp *TaskProcessor) StartTask(taskID int64) {
	tp.mu.Lock()
	if _, exists := tp.running[taskID]; exists {
		tp.mu.Unlock()
		log.Printf("[autonomous] task %d already running", taskID)
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	tp.running[taskID] = cancel
	tp.mu.Unlock()

	go func() {
		// Acquire worker slot (blocks if all workers busy).
		tp.workerSem <- struct{}{}
		defer func() { <-tp.workerSem }()

		tp.processTask(ctx, taskID)
	}()
}

// StartNext finds the highest-priority pending autonomous task and starts it.
// Priority order: critical(3) > high(2) > normal(1) > low(0), then oldest first.
func (tp *TaskProcessor) StartNext() {
	tasks, err := tp.planner.List(planner.TaskFilter{Stage: planner.StageActive})
	if err != nil {
		return
	}

	// Sort by priority desc, then created_at asc.
	sort.Slice(tasks, func(i, j int) bool {
		if tasks[i].Priority != tasks[j].Priority {
			return tasks[i].Priority > tasks[j].Priority
		}
		return tasks[i].CreatedAt < tasks[j].CreatedAt
	})

	for _, t := range tasks {
		tp.mu.Lock()
		_, running := tp.running[t.ID]
		tp.mu.Unlock()
		if !running {
			// Check if it's autonomous.
			var meta map[string]any
			if t.Metadata != "" {
				if err := json.Unmarshal([]byte(t.Metadata), &meta); err == nil {
					if auto, _ := meta["autonomous"].(bool); auto {
						log.Printf("[autonomous] StartNext: picking task %d (priority=%d)", t.ID, t.Priority)
						tp.StartTask(t.ID)
						return
					}
				}
			}
		}
	}
}

// StopTask cancels an in-progress autonomous task.
func (tp *TaskProcessor) StopTask(taskID int64) {
	tp.mu.Lock()
	if cancel, ok := tp.running[taskID]; ok {
		cancel()
		delete(tp.running, taskID)
	}
	tp.mu.Unlock()
	log.Printf("[autonomous] stopped task %d", taskID)
}

// IsRunning reports whether a task is currently being processed.
func (tp *TaskProcessor) IsRunning(taskID int64) bool {
	tp.mu.Lock()
	defer tp.mu.Unlock()
	_, ok := tp.running[taskID]
	return ok
}

func (tp *TaskProcessor) processTask(ctx context.Context, taskID int64) {
	defer func() {
		tp.mu.Lock()
		delete(tp.running, taskID)
		tp.mu.Unlock()
	}()

	task, err := tp.planner.Get(taskID)
	if err != nil {
		log.Printf("[autonomous] failed to fetch task %d: %v", taskID, err)
		return
	}

	log.Printf("[autonomous] === starting task %d: %s ===", taskID, task.Title)
	tp.sendActivity(taskID, "status", "Starting autonomous processing...")
	tp.hooks.RunWorkflowHook("before:task_start", map[string]string{"task_id": fmt.Sprintf("%d", taskID)})

	// Move to active stage.
	active := planner.StageActive
	now := time.Now().UTC().Format(time.RFC3339)
	agentID := fmt.Sprintf("auto-%d", taskID)
	tp.planner.Update(taskID, planner.TaskUpdate{
		Stage:     &active,
		AgentID:   &agentID,
		StartedAt: &now,
	})
	tp.broadcastTaskUpdate(taskID)
	tp.sendActivity(taskID, "stage", "active")

	// Create isolated worktree for this task.
	var taskRoot string
	if tp.worktrees != nil {
		tp.sendActivity(taskID, "status", "Creating isolated worktree...")
		var err error
		taskRoot, err = tp.worktrees.Create(taskID, task.Title)
		if err != nil {
			log.Printf("[autonomous] failed to create worktree for task %d: %v", taskID, err)
			tp.sendActivity(taskID, "status", fmt.Sprintf("Worktree failed: %v — falling back to main repo", err))
			taskRoot = tp.projectRoot
		} else {
			tp.sendActivity(taskID, "status", fmt.Sprintf("Working in isolated branch: %s", tp.worktrees.branchName(taskID, task.Title)))
		}
	} else {
		taskRoot = tp.projectRoot
	}

	// Determine workflow mode.
	workflow := ""
	var meta map[string]any
	if task.Metadata != "" {
		if err := json.Unmarshal([]byte(task.Metadata), &meta); err == nil {
			if w, ok := meta["workflow"].(string); ok && (w == "micro" || w == "quick" || w == "full") {
				workflow = w
			}
		}
	}
	if workflow == "" {
		workflow = classifyWorkflow(task.Title, task.Description)
	}
	log.Printf("[autonomous] task %d workflow=%s", taskID, workflow)
	tp.sendActivity(taskID, "status", fmt.Sprintf("Workflow: %s", workflow))

	// Build the task prompt.
	prompt := tp.buildTaskPrompt(task, taskRoot, workflow)

	// Create a dedicated session for this task.
	sessionID := fmt.Sprintf("auto-task-%d", taskID)

	// Accumulate the output from the agent's responses.
	var outputBuf strings.Builder

	// sendEvent wraps agent events as task.activity broadcasts.
	sendEvent := func(msg WSMessage) {
		switch msg.Type {
		case "chat.token":
			outputBuf.WriteString(msg.Content)
			tp.sendActivity(taskID, "token", msg.Content)
			// Periodically flush output to the task record so the UI can show it.
			if outputBuf.Len()%200 < len(msg.Content) {
				out := outputBuf.String()
				tp.planner.Update(taskID, planner.TaskUpdate{Output: &out})
				tp.broadcastTaskUpdate(taskID)
			}
		case "tool.call":
			tp.sendActivity(taskID, "tool_call", string(msg.Data))
		case "tool.complete":
			tp.sendActivity(taskID, "tool_complete", string(msg.Data))
		case "tool.progress":
			tp.sendActivity(taskID, "tool_progress", string(msg.Data))
		case "tool.error":
			tp.sendActivity(taskID, "tool_error", string(msg.Data))
		case "chat.done":
			// Handled below after Run returns.
		}
	}

	// Run the agent loop, then E2E verify. Retry up to maxE2ERetries times
	// if verification fails, feeding the gap report back into the agent each
	// time so it can fix what it missed.
	var maxE2ERetries int
	switch workflow {
	case "micro":
		maxE2ERetries = 1
	case "quick":
		maxE2ERetries = 2
	default:
		maxE2ERetries = 3
	}

	agent := NewAgentLoop(tp.server.ai, tp.products, tp.sessions, tp.planner, tp.broadcast, tp.model, taskRoot)
	agent.autonomous = true
	agent.pm = tp.server.pm
	switch workflow {
	case "micro":
		agent.maxIter = 15
	case "quick":
		agent.maxIter = 30
	default:
		agent.maxIter = 40
	}

	hasChanges := false

	for attempt := 0; attempt <= maxE2ERetries; attempt++ {
		// On retry attempts, prepend the gap report to the prompt so the
		// agent knows exactly what E2E checks are still failing.
		runPrompt := prompt
		if attempt > 0 {
			tp.sendActivity(taskID, "status", fmt.Sprintf("E2E retry %d/%d — re-running agent with gap report...", attempt, maxE2ERetries))
		}

		agent.Run(ctx, sessionID, runPrompt, "code", nil, false, sendEvent)

		// Check if cancelled mid-run.
		if ctx.Err() != nil {
			log.Printf("[autonomous] task %d was cancelled", taskID)
			tp.sendActivity(taskID, "status", "Autonomous processing cancelled")
			return
		}

		// Save accumulated output after each attempt.
		finalOutput := outputBuf.String()
		tp.planner.Update(taskID, planner.TaskUpdate{Output: &finalOutput})

		// Commit and merge changes.
		hasChanges = false
		if tp.worktrees != nil && taskRoot != tp.projectRoot {
			tp.sendActivity(taskID, "status", "Committing changes...")
			commitErr := tp.worktrees.CommitInWorktree(taskID, task.Title)
			if commitErr == ErrNothingToCommit {
				log.Printf("[autonomous] task %d attempt %d: agent made no file changes", taskID, attempt)
				tp.sendActivity(taskID, "status", "Warning: agent made no file changes")
			} else if commitErr != nil {
				log.Printf("[autonomous] commit failed for task %d attempt %d: %v", taskID, attempt, commitErr)
				tp.sendActivity(taskID, "status", fmt.Sprintf("Commit warning: %v", commitErr))
			} else {
				hasChanges = true
			}

			if hasChanges {
				// Pre-merge gate: tsc + vite build in worktree (runs in task's
				// own worktree, so no serialization needed).
				tp.sendActivity(taskID, "status", "Running pre-merge type check...")
				worktreeWeb := filepath.Join(taskRoot, "web")
				if gateErr := PreMergeGate(worktreeWeb); gateErr != nil {
					log.Printf("[autonomous] pre-merge gate failed for task %d: %v", taskID, gateErr)
					tp.sendActivity(taskID, "status", fmt.Sprintf("Pre-merge gate failed: %v", gateErr))
					prompt = prompt + fmt.Sprintf("\n\n## Pre-Merge Gate Failed\n\n```\n%s\n```\n\nFix these type/build errors before the code can be merged.\n", gateErr.Error())
					hasChanges = false
					continue
				}

				// Serialize merge/build/test — only one task merges to dev at a time.
				tp.mergeMu.Lock()
				mergeRetry := false // set true if smoke test fails and we need to continue the retry loop

				tp.hooks.RunWorkflowHook("before:merge_to_dev", map[string]string{"task_id": fmt.Sprintf("%d", taskID)})
				tp.sendActivity(taskID, "status", "Merging to dev branch...")
				if err := tp.worktrees.MergeToDev(taskID, task.Title); err != nil {
					log.Printf("[autonomous] merge to dev failed for task %d: %v", taskID, err)
					tp.sendActivity(taskID, "status", fmt.Sprintf("Merge to dev warning: %v", err))
				} else {
					tp.hooks.RunWorkflowHook("after:merge_to_dev", map[string]string{"task_id": fmt.Sprintf("%d", taskID)})
					tp.sendActivity(taskID, "status", "Changes merged to dev — rebuilding frontend...")
					if tp.server != nil {
						if err := tp.server.RebuildDevFrontend(); err != nil {
							log.Printf("[autonomous] dev frontend rebuild failed for task %d: %v", taskID, err)
							tp.sendActivity(taskID, "status", fmt.Sprintf("Dev rebuild warning: %v", err))
						} else {
							tp.sendActivity(taskID, "status", "Dev frontend rebuilt — running smoke test...")
							devURL := fmt.Sprintf("http://localhost:%d", tp.server.cfg.Port+1)
							smokeResult, smokeErr := RunSmokeTest(devURL, tp.server.cfg.E2EHost, tp.server.cfg.E2ERunnerPath)
							if smokeErr != nil {
								log.Printf("[autonomous] dev smoke test error for task %d: %v", taskID, smokeErr)
								tp.sendActivity(taskID, "status", fmt.Sprintf("Smoke test error: %v", smokeErr))
							} else if !smokeResult.AllPass {
								log.Printf("[autonomous] dev smoke test FAILED for task %d", taskID)
								tp.sendActivity(taskID, "status", "Dev smoke test FAILED — reverting merge...")
								devWT := filepath.Join(tp.projectRoot, ".worktrees", "dev-server")
								if revErr := RevertLastMerge(devWT); revErr != nil {
									log.Printf("[autonomous] revert failed: %v", revErr)
								}
								tp.server.RebuildDevFrontend()
								prompt = prompt + "\n\n## Dev Smoke Test Failed\n\n" + FormatSmokeFailure(smokeResult) + "\n"
								hasChanges = false
								mergeRetry = true
							} else {
								tp.sendActivity(taskID, "status", "Dev smoke test PASSED — all checks green")
							}
						}
					}
				}

				tp.mergeMu.Unlock()
				if mergeRetry {
					continue
				}
			}
		}

		// If we got here without `continue`, the smoke test passed (or was skipped).
		if hasChanges {
			tp.postVerificationComment(taskID, "**E2E Verification: PASSED**\n\nAll smoke checks passed.")
		}
		break
	}

	// Re-read the task to see what stage the agent left it in.
	updated, err := tp.planner.Get(taskID)
	if err != nil {
		log.Printf("[autonomous] failed to re-read task %d: %v", taskID, err)
		return
	}

	completedAt := time.Now().UTC().Format(time.RFC3339)

	if updated.Stage == planner.StageActive {
		if !hasChanges {
			// Agent ran but made no changes (or smoke test failed all retries) — move to blocked.
			blocked := planner.StageBlocked
			tp.planner.Update(taskID, planner.TaskUpdate{
				Stage:       &blocked,
				CompletedAt: &completedAt,
			})
			tp.broadcastTaskUpdate(taskID)
			tp.sendActivity(taskID, "stage", "blocked")
			tp.sendActivity(taskID, "status", "Moved to blocked — no successful changes merged")
			tp.hooks.RunWorkflowHook("after:task_blocked", map[string]string{"task_id": fmt.Sprintf("%d", taskID)})
			tp.postVerificationComment(taskID, "**Task blocked**: Agent ran but no changes were successfully merged. The task may need a more detailed description or the agent may have encountered issues it couldn't resolve.")
		} else {
			// Changes made and smoke test passed — extract fingerprint and advance to validation.
			tp.extractFingerprint(task, taskRoot, workflow, agent.filesRead, agent.iterationsUsed)
			validation := planner.StageValidation
			tp.planner.Update(taskID, planner.TaskUpdate{
				Stage:       &validation,
				CompletedAt: &completedAt,
			})
			tp.broadcastTaskUpdate(taskID)
			tp.sendActivity(taskID, "stage", "validation")
			tp.sendActivity(taskID, "status", "Processing complete — moved to validation")
			tp.hooks.RunWorkflowHook("after:task_done", map[string]string{"task_id": fmt.Sprintf("%d", taskID)})
		}
	} else {
		// Agent already moved the task (e.g., to blocked or done). Respect it.
		tp.planner.Update(taskID, planner.TaskUpdate{CompletedAt: &completedAt})
		tp.broadcastTaskUpdate(taskID)
		tp.sendActivity(taskID, "status", fmt.Sprintf("Processing complete — task is in %s", updated.Stage))
	}

	tp.sendActivity(taskID, "done", "")
	log.Printf("[autonomous] === completed task %d (stage=%s) ===", taskID, updated.Stage)

	// Check for next queued autonomous task.
	tp.StartNext()
}

func (tp *TaskProcessor) buildTaskPrompt(task planner.Task, taskRoot, workflow string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "You are autonomously working on task #%d.\n\n", task.ID)
	fmt.Fprintf(&b, "**Title:** %s\n", task.Title)
	if task.Description != "" {
		fmt.Fprintf(&b, "**Description:** %s\n", task.Description)
	}
	if task.Acceptance != "" {
		fmt.Fprintf(&b, "**Acceptance Criteria:** %s\n", task.Acceptance)
	}
	if task.Product != "" {
		fmt.Fprintf(&b, "**Product:** %s\n", task.Product)
	}

	// Project context from CLAUDE.md is injected below.
	fmt.Fprintf(&b, "\n## Project Root\n`%s`\n", taskRoot)

	// Inject CLAUDE.md conventions if present.
	claudeMDPath := filepath.Join(taskRoot, "CLAUDE.md")
	if claudeMD, err := os.ReadFile(claudeMDPath); err == nil {
		b.WriteString("\n## Project Conventions (from CLAUDE.md)\n")
		content := string(claudeMD)
		if len(content) > 4000 {
			content = content[:4000] + "\n...(truncated)"
		}
		b.WriteString(content)
		b.WriteString("\n")
	}

	// Pre-load relevant files so agent doesn't need to search.
	files := findRelevantFiles(taskRoot, task.Title, task.Description)
	if len(files) > 0 {
		b.WriteString("\n## Pre-loaded Files\n")
		b.WriteString("These files are relevant to your task — no need to search.\n\n")
		for path, content := range files {
			fmt.Fprintf(&b, "### `%s`\n```\n%s\n```\n\n", path, content)
		}
	}

	// Cross-task learning: suggest files from similar past tasks.
	if tp.planner != nil {
		fingerprints := tp.matchFingerprints(task.Title, task.Description)
		if len(fingerprints) > 0 {
			b.WriteString("\n## Similar Past Tasks\n")
			b.WriteString("These past tasks had similar scope — their files may be relevant:\n\n")
			for _, fp := range fingerprints {
				fmt.Fprintf(&b, "- **Task #%d**: %s\n  Files: %s\n", fp.TaskID, fp.Title, strings.Join(fp.Files, ", "))
			}
			b.WriteString("\n")
		}
	}

	// Mandatory planning instruction for all workflows.
	b.WriteString("\n## MANDATORY: Plan First\n")
	b.WriteString("Before making ANY code changes, you MUST output a plan as your FIRST response:\n\n")
	b.WriteString("```\n## Plan\n")
	b.WriteString("- Files to modify: [list specific file paths]\n")
	b.WriteString("- Files to read first: [list files for context]\n")
	b.WriteString("- Approach: [1-2 sentences describing your strategy]\n")
	b.WriteString("- Estimated changes: [N files, ~M lines]\n")
	b.WriteString("```\n\n")
	b.WriteString("Do NOT call any code tools before outputting this plan. If you skip the plan, you will be warned.\n\n")

	// Workflow-specific instructions.
	if workflow == "full" {
		b.WriteString("\n## Workflow: Full (7-step)\n")
		b.WriteString("Follow these steps in order:\n\n")
		b.WriteString("### Step 1: Plan\n")
		b.WriteString("Analyze the task. Search the codebase to understand the affected areas. Document your approach.\n\n")
		b.WriteString("### Step 2: Write Tests\n")
		b.WriteString("If test files exist for the affected area, write failing tests first (TDD). Use `code_exec` to run them and confirm they fail.\n\n")
		b.WriteString("### Step 3: Implement\n")
		b.WriteString("Make the minimal changes needed. Use `code_edit` for surgical changes, `code_write` only for new files.\n\n")
		b.WriteString("### Step 4: Build & Verify\n")
		b.WriteString("Run `go build ./...` for Go changes. Run `cd web && npx vite build` for frontend changes. Run tests with `code_exec`.\n\n")
		b.WriteString("### Step 5: Security Review\n")
		b.WriteString("Check your changes for: SQL injection (use parameterized queries), hardcoded secrets (use env vars), unsafe patterns.\n\n")
		b.WriteString("### Step 6: Summary\n")
		b.WriteString("Write a clear summary of what you changed and why in the task output.\n\n")
		b.WriteString("### Step 7: Update Task\n")
		b.WriteString("Use `task_update` to move the task to `validation` with your summary. If blocked, move to `blocked` with the reason.\n")
	} else if workflow == "micro" {
		b.WriteString("\n## Workflow: Micro (minimal)\n")
		b.WriteString("This is a trivial change. The pipeline handles builds and verification.\n\n")
		b.WriteString("### Step 1: Implement\n")
		b.WriteString("Make the change directly. Use `code_edit` for modifications.\n")
		b.WriteString("Do NOT run `vite build`, `go build`, or any E2E/verification commands.\n\n")
		b.WriteString("### Step 2: Update Task\n")
		b.WriteString("Use `task_update` to move the task to `validation` with a one-line summary.\n")
	} else {
		b.WriteString("\n## Workflow: Quick (5-step)\n")
		b.WriteString("Follow these steps in order:\n\n")
		b.WriteString("### Step 1: Search & Understand\n")
		b.WriteString("Use `code_search` and `code_grep` to find relevant files. Use `code_read` to understand the code.\n\n")
		b.WriteString("### Step 2: Implement\n")
		b.WriteString("Make the minimal changes needed. Use `code_edit` for modifications, `code_write` for new files.\n\n")
		b.WriteString("### Step 3: Build & Verify\n")
		b.WriteString("Run `go build ./...` for Go changes. Run `cd web && npx vite build` for frontend changes.\n\n")
		b.WriteString("### Step 4: Summary\n")
		b.WriteString("Write a clear summary of what you changed and why.\n\n")
		b.WriteString("### Step 5: Update Task\n")
		b.WriteString("Use `task_update` to move the task to `validation` with your summary. If blocked, move to `blocked` with the reason.\n")
	}

	// Inject quality gate for UI tasks.
	if isUITask(task.Title, task.Description) {
		b.WriteString("\n## UI Quality Standards (pipeline verifies — you must implement correctly)\n")
		b.WriteString("This task affects the UI. The pipeline runs E2E verification AFTER your changes.\n")
		b.WriteString("To pass verification, your implementation MUST handle:\n\n")
		b.WriteString("1. **All state permutations**: If a component has toggle states, handle ALL combinations in the code.\n")
		b.WriteString("   - Expanded/collapsed, open/closed — every combination must render correctly.\n")
		b.WriteString("   - Don't assume only the default state will be tested.\n")
		b.WriteString("2. **Visual quality**: Proper alignment, spacing, sizing, borders.\n")
		b.WriteString("   - Use consistent Tailwind classes matching the existing theme (bg-surface, border-border-subtle, etc.).\n")
		b.WriteString("   - If it would look 'stuck on' or out of place, it's wrong — fix before submitting.\n")
		b.WriteString("3. **Interaction correctness**: Click handlers wired, hover states set, transitions smooth.\n")
		b.WriteString("   - Every button/clickable MUST have an onClick and a cursor-pointer class.\n")
		b.WriteString("4. **Edge cases**: Empty state, overflow with many items, min/max width constraints.\n")
		b.WriteString("5. **Regression prevention**: Don't break existing layout or interactions when adding new ones.\n\n")
	}

	// Inject incremental approach for complex tasks.
	if isComplexTask(task.Title, task.Description) {
		b.WriteString("\n## Incremental Implementation\n")
		b.WriteString("This looks like a complex task. Work incrementally:\n\n")
		b.WriteString("1. **One capability at a time**: Add ONE thing, verify it works, then add the next.\n")
		b.WriteString("2. **Build on confirmed state**: Don't build feature B until feature A is verified working.\n")
		b.WriteString("3. **Wire before build**: Add settings/toggles first (wired to existing behavior), then implement new behavior.\n")
		b.WriteString("4. **Post progress**: Use `task_comment` after each verified increment so the board shows real progress.\n")
		b.WriteString("5. **If something breaks, stop and fix it** before adding more changes on top.\n\n")
	}

	b.WriteString("\n## Rules\n")
	b.WriteString("- Be precise. Make minimal changes. Do not refactor unrelated code.\n")
	b.WriteString("- Do NOT run git commands — the system handles commits and merges automatically.\n")
	b.WriteString("- All file paths are relative to the project root shown above.\n")
	b.WriteString("- Do NOT modify the task description — it is the original plan and must be preserved.\n")
	b.WriteString("- Post all findings, gaps, and status notes as comments using `task_comment`, not `task_update`.\n")
	b.WriteString("- Use task_memory to store key facts you discover (file locations, patterns, conventions). Check task_memory before re-searching.\n")

	return b.String()
}

// findRelevantFiles searches the project for files matching PascalCase component
// names or explicit file paths mentioned in the task title/description. Returns
// up to 3 files with their content (capped at 3000 chars each).
func findRelevantFiles(taskRoot, title, description string) map[string]string {
	text := title + " " + description
	results := make(map[string]string)

	// Extract PascalCase component names (e.g., ProductRail, ChatPanel).
	compRe := regexp.MustCompile(`[A-Z][a-z]+(?:[A-Z][a-z]+)+`)
	components := compRe.FindAllString(text, -1)

	// Also extract explicit file paths.
	pathRe := regexp.MustCompile(`[\w/.-]+\.(tsx?|go|css)`)
	paths := pathRe.FindAllString(text, -1)

	// Search for component files.
	for _, comp := range components {
		if len(results) >= 3 {
			break
		}
		filepath.WalkDir(taskRoot, func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				if d != nil && d.IsDir() {
					name := d.Name()
					if name == "node_modules" || name == ".git" || name == "dist" || name == ".worktrees" {
						return filepath.SkipDir
					}
				}
				return nil
			}
			base := strings.TrimSuffix(d.Name(), filepath.Ext(d.Name()))
			if base == comp && len(results) < 3 {
				rel, _ := filepath.Rel(taskRoot, path)
				data, err := os.ReadFile(path)
				if err == nil {
					content := string(data)
					if len(content) > 3000 {
						content = content[:3000] + "\n...(truncated)"
					}
					results[rel] = content
				}
				return filepath.SkipAll
			}
			return nil
		})
	}

	// Check explicit paths.
	for _, p := range paths {
		if len(results) >= 3 {
			break
		}
		abs := filepath.Join(taskRoot, p)
		if data, err := os.ReadFile(abs); err == nil {
			content := string(data)
			if len(content) > 3000 {
				content = content[:3000] + "\n...(truncated)"
			}
			results[p] = content
		}
	}

	return results
}

// isUITask returns true if the task likely involves frontend/UI changes.
func isUITask(title, description string) bool {
	text := strings.ToLower(title + " " + description)
	uiKW := []string{
		"button", "panel", "layout", "sidebar", "drawer", "rail", "modal",
		"dialog", "form", "input", "dropdown", "menu", "navbar", "header",
		"footer", "card", "grid", "table", "list", "tab", "icon", "badge",
		"tooltip", "toast", "notification", "style", "css", "tailwind",
		"component", "view", "page", "screen", "ui", "ux", "frontend",
		"expand", "collapse", "toggle", "resize", "responsive", "theme",
		".tsx", ".css", "vite", "react",
	}
	for _, kw := range uiKW {
		if strings.Contains(text, kw) {
			return true
		}
	}
	return false
}

// isComplexTask returns true if the task touches multiple interacting systems.
func isComplexTask(title, description string) bool {
	text := strings.ToLower(title + " " + description)
	complexKW := []string{
		"layout", "refactor", "redesign", "multiple", "panel", "position",
		"flexible", "configurable", "system", "overhaul", "architecture",
		"split", "independent", "decouple", "wire", "integrate",
	}
	matches := 0
	for _, kw := range complexKW {
		if strings.Contains(text, kw) {
			matches++
		}
	}
	// Need at least 2 keyword matches to be considered complex.
	return matches >= 2
}

// classifyWorkflow auto-classifies a task as micro/quick/full based on keywords.
func classifyWorkflow(title, description string) string {
	text := strings.ToLower(title + " " + description)
	microKW := []string{
		"add button", "add icon", "change color", "fix typo", "rename",
		"update text", "update label", "change text", "toggle", "hide",
		"show", "move button", "add tooltip", "remove button", "add link",
		"change icon", "fix spacing", "fix padding", "fix margin",
		"add class", "change style", "update style", "add prop",
	}
	for _, kw := range microKW {
		if strings.Contains(text, kw) {
			return "micro"
		}
	}
	fullKW := []string{
		"refactor", "redesign", "new feature", "add api", "add endpoint",
		"database", "migration", "security", "authentication", "pipeline",
		"architect",
	}
	for _, kw := range fullKW {
		if strings.Contains(text, kw) {
			return "full"
		}
	}
	return "quick"
}

// AddListener registers a callback for task activity events.
func (tp *TaskProcessor) AddListener(taskID int64, cb func(string, string)) {
	tp.listenerMu.Lock()
	tp.listeners[taskID] = append(tp.listeners[taskID], cb)
	tp.listenerMu.Unlock()
}

// RemoveListeners removes all listeners for a task.
func (tp *TaskProcessor) RemoveListeners(taskID int64) {
	tp.listenerMu.Lock()
	delete(tp.listeners, taskID)
	tp.listenerMu.Unlock()
}

func (tp *TaskProcessor) sendActivity(taskID int64, activityType, content string) {
	// Notify any registered listeners (e.g., quick_task chat stream).
	tp.listenerMu.Lock()
	cbs := tp.listeners[taskID]
	tp.listenerMu.Unlock()
	for _, cb := range cbs {
		cb(activityType, content)
	}

	data, _ := json.Marshal(map[string]any{
		"task_id": taskID,
		"type":    activityType,
		"content": content,
		"time":    time.Now().UTC().Format(time.RFC3339),
	})
	tp.broadcast(WSMessage{
		Type: "task.activity",
		Data: data,
	})
}

func (tp *TaskProcessor) postVerificationComment(taskID int64, body string) {
	comment := planner.Comment{
		TaskID:      taskID,
		Author:      "soul",
		Type:        "verification",
		Body:        body,
		Attachments: []string{},
		CreatedAt:   time.Now().UTC().Format(time.RFC3339),
	}
	if id, err := tp.planner.CreateComment(comment); err == nil {
		comment.ID = id
		raw, _ := json.Marshal(comment)
		tp.broadcast(WSMessage{Type: "task.comment.added", Data: raw})
	}
}

func (tp *TaskProcessor) broadcastTaskUpdate(taskID int64) {
	task, err := tp.planner.Get(taskID)
	if err != nil {
		return
	}
	raw, _ := json.Marshal(task)
	tp.broadcast(WSMessage{Type: "task.updated", Data: raw})
}

// extractFingerprint captures what the agent learned from a completed task.
func (tp *TaskProcessor) extractFingerprint(task planner.Task, taskRoot, workflow string, filesRead map[string]bool, iterationsUsed int) {
	modifiedFiles := getModifiedFiles(taskRoot)
	if len(modifiedFiles) == 0 {
		return
	}

	keywords := extractKeywords(task.Title + " " + task.Description)

	var readFiles []string
	for f := range filesRead {
		readFiles = append(readFiles, f)
	}
	sort.Strings(readFiles)

	fingerprint := map[string]any{
		"task_id":         task.ID,
		"title":           task.Title,
		"files_modified":  modifiedFiles,
		"files_read":      readFiles,
		"iterations_used": iterationsUsed,
		"keywords":        keywords,
		"workflow":        workflow,
	}

	data, err := json.Marshal(fingerprint)
	if err != nil {
		return
	}

	key := fmt.Sprintf("task_fp_%d", task.ID)
	tp.planner.UpsertMemory(key, string(data), "task_fingerprint")
	log.Printf("[autonomous] stored fingerprint for task %d: %d files, %d keywords", task.ID, len(modifiedFiles), len(keywords))
}

// getModifiedFiles returns files changed in the worktree vs its base.
func getModifiedFiles(taskRoot string) []string {
	cmd := exec.Command("git", "diff", "--name-only", "HEAD~1")
	cmd.Dir = taskRoot
	out, err := cmd.Output()
	if err != nil {
		// Fallback: get all tracked files that are different from dev.
		cmd2 := exec.Command("git", "diff", "--name-only", "dev")
		cmd2.Dir = taskRoot
		out, err = cmd2.Output()
		if err != nil {
			return nil
		}
	}
	var files []string
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line != "" {
			files = append(files, line)
		}
	}
	return files
}

// extractKeywords extracts meaningful words from text (lowercased, deduped, no stopwords).
func extractKeywords(text string) []string {
	stopwords := map[string]bool{
		"the": true, "a": true, "an": true, "and": true, "or": true, "but": true,
		"in": true, "on": true, "at": true, "to": true, "for": true, "of": true,
		"with": true, "by": true, "from": true, "is": true, "are": true, "was": true,
		"be": true, "has": true, "have": true, "had": true, "do": true, "does": true,
		"this": true, "that": true, "it": true, "not": true, "no": true,
	}

	words := regexp.MustCompile(`[a-zA-Z]+`).FindAllString(strings.ToLower(text), -1)
	seen := make(map[string]bool)
	var result []string
	for _, w := range words {
		if len(w) < 3 || stopwords[w] || seen[w] {
			continue
		}
		seen[w] = true
		result = append(result, w)
	}
	return result
}

type taskFingerprint struct {
	TaskID int64
	Title  string
	Files  []string
}

// matchFingerprints finds past task fingerprints with keyword overlap.
func (tp *TaskProcessor) matchFingerprints(title, description string) []taskFingerprint {
	memories, err := tp.planner.SearchMemories("task_fingerprint")
	if err != nil {
		return nil
	}

	newKeywords := extractKeywords(title + " " + description)
	if len(newKeywords) == 0 {
		return nil
	}
	newSet := make(map[string]bool)
	for _, kw := range newKeywords {
		newSet[kw] = true
	}

	type scored struct {
		fp    taskFingerprint
		score int
	}
	var candidates []scored

	for _, mem := range memories {
		var data map[string]any
		if err := json.Unmarshal([]byte(mem.Content), &data); err != nil {
			continue
		}

		// Count keyword overlap.
		kwRaw, _ := data["keywords"].([]any)
		overlap := 0
		for _, kw := range kwRaw {
			if s, ok := kw.(string); ok && newSet[s] {
				overlap++
			}
		}
		if overlap < 2 {
			continue // need at least 2 keyword matches
		}

		taskID, _ := data["task_id"].(float64)
		taskTitle, _ := data["title"].(string)
		filesRaw, _ := data["files_modified"].([]any)
		var files []string
		for _, f := range filesRaw {
			if s, ok := f.(string); ok {
				files = append(files, s)
			}
		}

		candidates = append(candidates, scored{
			fp:    taskFingerprint{TaskID: int64(taskID), Title: taskTitle, Files: files},
			score: overlap,
		})
	}

	// Sort by score descending, take top 3.
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].score > candidates[j].score
	})

	var result []taskFingerprint
	for i, c := range candidates {
		if i >= 3 {
			break
		}
		result = append(result, c.fp)
	}
	return result
}
