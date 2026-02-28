package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
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
func NewTaskProcessor(aiClient *ai.Client, pm *products.Manager, sessions *session.Store, plannerStore *planner.Store, broadcast func(WSMessage), model, projectRoot string, worktrees *WorktreeManager) *TaskProcessor {
	return &TaskProcessor{
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

	// Build the task prompt.
	prompt := tp.buildTaskPrompt(task, taskRoot)

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

	// Run the agent loop with code tools enabled.
	agent := NewAgentLoop(tp.ai, tp.products, tp.sessions, tp.planner, tp.broadcast, tp.model, taskRoot)
	agent.Run(ctx, sessionID, prompt, "code", nil, false, sendEvent)

	// Check if cancelled.
	if ctx.Err() != nil {
		log.Printf("[autonomous] task %d was cancelled", taskID)
		tp.sendActivity(taskID, "status", "Autonomous processing cancelled")
		return
	}

	// Save the full output.
	finalOutput := outputBuf.String()
	tp.planner.Update(taskID, planner.TaskUpdate{Output: &finalOutput})

	// Commit changes in the worktree and merge to dev.
	if tp.worktrees != nil && taskRoot != tp.projectRoot {
		tp.sendActivity(taskID, "status", "Committing changes...")
		if err := tp.worktrees.CommitInWorktree(taskID, task.Title); err != nil {
			log.Printf("[autonomous] commit failed for task %d: %v", taskID, err)
			tp.sendActivity(taskID, "status", fmt.Sprintf("Commit warning: %v", err))
		}

		tp.sendActivity(taskID, "status", "Merging to dev branch...")
		if err := tp.worktrees.MergeToDev(taskID, task.Title); err != nil {
			log.Printf("[autonomous] merge to dev failed for task %d: %v", taskID, err)
			tp.sendActivity(taskID, "status", fmt.Sprintf("Merge to dev warning: %v", err))
		} else {
			tp.sendActivity(taskID, "status", "Changes merged to dev — visible on dev server")
		}
	}

	// Re-read the task to see what stage the agent left it in.
	// The agent may have moved it to blocked, done, etc. via task_update.
	// Only advance to validation if the agent left it in active.
	updated, err := tp.planner.Get(taskID)
	if err != nil {
		log.Printf("[autonomous] failed to re-read task %d: %v", taskID, err)
		return
	}

	completedAt := time.Now().UTC().Format(time.RFC3339)

	if updated.Stage == planner.StageActive {
		// Agent didn't move the task — advance to validation.
		validation := planner.StageValidation
		tp.planner.Update(taskID, planner.TaskUpdate{
			Stage:       &validation,
			CompletedAt: &completedAt,
		})
		tp.broadcastTaskUpdate(taskID)
		tp.sendActivity(taskID, "stage", "validation")
		tp.sendActivity(taskID, "status", "Processing complete — moved to validation")
	} else {
		// Agent already moved the task (e.g., to blocked or done). Respect it.
		tp.planner.Update(taskID, planner.TaskUpdate{CompletedAt: &completedAt})
		tp.broadcastTaskUpdate(taskID)
		tp.sendActivity(taskID, "status", fmt.Sprintf("Processing complete — task is in %s", updated.Stage))
	}

	tp.sendActivity(taskID, "done", "")
	log.Printf("[autonomous] === completed task %d (stage=%s) ===", taskID, updated.Stage)
}

func (tp *TaskProcessor) buildTaskPrompt(task planner.Task, taskRoot string) string {
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

	// Add project context.
	b.WriteString("\n## Project Context\n")
	fmt.Fprintf(&b, "Project root: `%s`\n", taskRoot)
	b.WriteString("This is a Go + React/TypeScript monorepo:\n")
	b.WriteString("- `cmd/soul/` — Go entrypoint\n")
	b.WriteString("- `internal/server/` — Go HTTP server, WebSocket, agent loop, task APIs\n")
	b.WriteString("- `internal/planner/` — Task store (SQLite)\n")
	b.WriteString("- `internal/ai/` — Claude API client\n")
	b.WriteString("- `internal/session/` — Chat session memory\n")
	b.WriteString("- `web/src/` — React frontend (Vite + TypeScript + Tailwind)\n")
	b.WriteString("- `web/src/components/` — React components (chat/, layout/, planner/, panels/)\n")
	b.WriteString("- `web/src/hooks/` — Custom hooks (useChat, usePlanner, useWebSocket, etc.)\n")
	b.WriteString("- `web/src/lib/` — Types, WebSocket client, utilities\n")
	b.WriteString("- `products/` — Product plugins (compliance-go, etc.)\n")

	b.WriteString("\n## Instructions\n")
	b.WriteString("You have code tools to read, write, search, and execute commands.\n")
	b.WriteString("1. Use `code_search` and `code_grep` to find relevant files.\n")
	b.WriteString("2. Use `code_read` to understand the code.\n")
	b.WriteString("3. Use `code_write` to make changes.\n")
	b.WriteString("4. Use `code_exec` to build and verify (e.g., `cd web && npx vite build`).\n")
	b.WriteString("5. Update the task with your results using `task_update`.\n")
	b.WriteString("6. If you cannot complete the task, move it to `blocked` with a description of why.\n")
	b.WriteString("7. If you complete it, move it to `validation`.\n")
	b.WriteString("\n## Rules\n")
	b.WriteString("- Be precise. Make minimal changes. Do not refactor unrelated code.\n")
	b.WriteString("- Do NOT run git commands — the system handles commits and merges automatically.\n")
	b.WriteString("- All file paths are relative to the project root shown above.\n")
	return b.String()
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

func (tp *TaskProcessor) broadcastTaskUpdate(taskID int64) {
	task, err := tp.planner.Get(taskID)
	if err != nil {
		return
	}
	raw, _ := json.Marshal(task)
	tp.broadcast(WSMessage{Type: "task.updated", Data: raw})
}
