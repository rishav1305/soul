package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/rishav1305/soul/internal/ai"
	"github.com/rishav1305/soul/internal/planner"
	"github.com/rishav1305/soul/internal/products"
	"github.com/rishav1305/soul/internal/session"
)

// TaskProcessor handles autonomous task execution in the background.
type TaskProcessor struct {
	server      *Server
	ai          *ai.Client
	products    *products.Manager
	sessions    *session.Store
	planner     *planner.Store
	broadcast   func(WSMessage)
	model       string
	projectRoot string
	worktrees   *WorktreeManager

	mu      sync.Mutex
	running map[int64]context.CancelFunc
}

// NewTaskProcessor creates a new autonomous task processor.
func NewTaskProcessor(srv *Server, aiClient *ai.Client, pm *products.Manager, sessions *session.Store, plannerStore *planner.Store, broadcast func(WSMessage), model, projectRoot string, worktrees *WorktreeManager) *TaskProcessor {
	return &TaskProcessor{
		server:      srv,
		ai:          aiClient,
		products:    pm,
		sessions:    sessions,
		planner:     plannerStore,
		broadcast:   broadcast,
		model:       model,
		projectRoot: projectRoot,
		worktrees:   worktrees,
		running:     make(map[int64]context.CancelFunc),
	}
}

// StartTask begins autonomous processing of a task in a background goroutine.
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

	go tp.processTask(ctx, taskID)
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

	agent := NewAgentLoop(tp.ai, tp.products, tp.sessions, tp.planner, tp.broadcast, tp.model, taskRoot)
	agent.autonomous = true
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
				// Pre-merge gate: tsc + vite build in worktree.
				tp.sendActivity(taskID, "status", "Running pre-merge type check...")
				worktreeWeb := filepath.Join(taskRoot, "web")
				if gateErr := PreMergeGate(worktreeWeb); gateErr != nil {
					log.Printf("[autonomous] pre-merge gate failed for task %d: %v", taskID, gateErr)
					tp.sendActivity(taskID, "status", fmt.Sprintf("Pre-merge gate failed: %v", gateErr))
					prompt = prompt + fmt.Sprintf("\n\n## Pre-Merge Gate Failed\n\n```\n%s\n```\n\nFix these type/build errors before the code can be merged.\n", gateErr.Error())
					hasChanges = false
					continue
				}

				tp.sendActivity(taskID, "status", "Merging to dev branch...")
				if err := tp.worktrees.MergeToDev(taskID, task.Title); err != nil {
					log.Printf("[autonomous] merge to dev failed for task %d: %v", taskID, err)
					tp.sendActivity(taskID, "status", fmt.Sprintf("Merge to dev warning: %v", err))
				} else {
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
								continue
							} else {
								tp.sendActivity(taskID, "status", "Dev smoke test PASSED — all checks green")
							}
						}
					}
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
			tp.postVerificationComment(taskID, "**Task blocked**: Agent ran but no changes were successfully merged. The task may need a more detailed description or the agent may have encountered issues it couldn't resolve.")
		} else {
			// Changes made and smoke test passed — advance to validation.
			validation := planner.StageValidation
			tp.planner.Update(taskID, planner.TaskUpdate{
				Stage:       &validation,
				CompletedAt: &completedAt,
			})
			tp.broadcastTaskUpdate(taskID)
			tp.sendActivity(taskID, "stage", "validation")
			tp.sendActivity(taskID, "status", "Processing complete — moved to validation")
		}
	} else {
		// Agent already moved the task (e.g., to blocked or done). Respect it.
		tp.planner.Update(taskID, planner.TaskUpdate{CompletedAt: &completedAt})
		tp.broadcastTaskUpdate(taskID)
		tp.sendActivity(taskID, "status", fmt.Sprintf("Processing complete — task is in %s", updated.Stage))
	}

	tp.sendActivity(taskID, "done", "")
	log.Printf("[autonomous] === completed task %d (stage=%s) ===", taskID, updated.Stage)
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

	b.WriteString("\n## Rules\n")
	b.WriteString("- Be precise. Make minimal changes. Do not refactor unrelated code.\n")
	b.WriteString("- Do NOT run git commands — the system handles commits and merges automatically.\n")
	b.WriteString("- All file paths are relative to the project root shown above.\n")
	b.WriteString("- Do NOT modify the task description — it is the original plan and must be preserved.\n")
	b.WriteString("- Post all findings, gaps, and status notes as comments using `task_comment`, not `task_update`.\n")

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

func (tp *TaskProcessor) sendActivity(taskID int64, activityType, content string) {
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
