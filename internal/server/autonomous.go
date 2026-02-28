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
	ai        *ai.Client
	products  *products.Manager
	sessions  *session.Store
	planner   *planner.Store
	broadcast func(WSMessage)
	model     string

	mu      sync.Mutex
	running map[int64]context.CancelFunc
}

// NewTaskProcessor creates a new autonomous task processor.
func NewTaskProcessor(aiClient *ai.Client, pm *products.Manager, sessions *session.Store, plannerStore *planner.Store, broadcast func(WSMessage), model string) *TaskProcessor {
	return &TaskProcessor{
		ai:        aiClient,
		products:  pm,
		sessions:  sessions,
		planner:   plannerStore,
		broadcast: broadcast,
		model:     model,
		running:   make(map[int64]context.CancelFunc),
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

	// Build the task prompt.
	prompt := tp.buildTaskPrompt(task)

	// Create a dedicated session for this task.
	sessionID := fmt.Sprintf("auto-task-%d", taskID)

	// Accumulate the plan/output from the agent's responses.
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

	// Run the agent loop.
	agent := NewAgentLoop(tp.ai, tp.products, tp.sessions, tp.planner, tp.broadcast, tp.model)
	agent.Run(ctx, sessionID, prompt, "brainstorm", nil, false, sendEvent)

	// Check if cancelled.
	if ctx.Err() != nil {
		log.Printf("[autonomous] task %d was cancelled", taskID)
		tp.sendActivity(taskID, "status", "Autonomous processing cancelled")
		return
	}

	// Finalize — save the full output and move to validation.
	finalOutput := outputBuf.String()
	validation := planner.StageValidation
	completedAt := time.Now().UTC().Format(time.RFC3339)

	tp.planner.Update(taskID, planner.TaskUpdate{
		Output:      &finalOutput,
		Stage:       &validation,
		CompletedAt: &completedAt,
	})
	tp.broadcastTaskUpdate(taskID)

	tp.sendActivity(taskID, "stage", "validation")
	tp.sendActivity(taskID, "status", "Processing complete — moved to validation")
	tp.sendActivity(taskID, "done", "")

	log.Printf("[autonomous] === completed task %d ===", taskID)
}

func (tp *TaskProcessor) buildTaskPrompt(task planner.Task) string {
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
	b.WriteString("\n## Instructions\n")
	b.WriteString("1. Analyze this task and think through what needs to be done.\n")
	b.WriteString("2. Create a step-by-step plan.\n")
	b.WriteString("3. Use available tools to execute the plan.\n")
	b.WriteString("4. Update the task with your findings using task_update.\n")
	b.WriteString("5. Provide a summary of what was accomplished.\n")
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
