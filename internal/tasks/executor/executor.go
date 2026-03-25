package executor

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"

	"github.com/rishav1305/soul/internal/tasks/hooks"
	"github.com/rishav1305/soul/internal/tasks/phases"
	"github.com/rishav1305/soul/internal/tasks/store"
)

// Config holds the configuration for an Executor.
type Config struct {
	Store       *store.Store
	MaxParallel int
	RepoDir     string
	Client          Sender // Claude API client (nil = skip agent loop)
	HooksConfigPath string // Path to hooks JSON config file (optional)
}

// Executor manages the lifecycle of running tasks.
type Executor struct {
	store           *store.Store
	maxParallel     int
	repoDir         string
	client          Sender
	hooksConfigPath string
	mu              sync.Mutex
	running         map[int64]context.CancelFunc
}

// New creates a new Executor. MaxParallel defaults to 3 if not set.
func New(cfg Config) *Executor {
	if cfg.MaxParallel <= 0 {
		cfg.MaxParallel = 3
	}
	return &Executor{
		store:           cfg.Store,
		maxParallel:     cfg.MaxParallel,
		repoDir:         cfg.RepoDir,
		client:          cfg.Client,
		hooksConfigPath: cfg.HooksConfigPath,
		running:         make(map[int64]context.CancelFunc),
	}
}

// Start begins execution of the task with the given ID.
// The task must be in "backlog" or "blocked" stage.
func (e *Executor) Start(ctx context.Context, taskID int64) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if _, ok := e.running[taskID]; ok {
		return fmt.Errorf("task %d is already running", taskID)
	}

	if len(e.running) >= e.maxParallel {
		return fmt.Errorf("max parallel tasks (%d) reached", e.maxParallel)
	}

	task, err := e.store.Get(taskID)
	if err != nil {
		return err
	}

	if task.Stage == "brainstorm" {
		return fmt.Errorf("task %d is in brainstorm stage — user-driven only", taskID)
	}

	if task.Stage != "backlog" && task.Stage != "blocked" {
		return fmt.Errorf("task %d has stage %q — must be backlog or blocked to start", taskID, task.Stage)
	}

	if _, err := e.store.Update(taskID, map[string]interface{}{"stage": "active"}); err != nil {
		return err
	}

	workflow := ClassifyWorkflow(task.Title, task.Description)
	if _, err := e.store.AddActivity(taskID, "task.started", map[string]interface{}{"workflow": workflow}); err != nil {
		return err
	}

	taskCtx, cancel := context.WithCancel(ctx)
	e.running[taskID] = cancel

	go e.run(taskCtx, taskID)
	return nil
}

// Stop cancels the running task with the given ID and moves it to "blocked".
func (e *Executor) Stop(taskID int64) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	cancel, ok := e.running[taskID]
	if !ok {
		return fmt.Errorf("task %d is not running", taskID)
	}

	delete(e.running, taskID)
	cancel()

	if _, err := e.store.Update(taskID, map[string]interface{}{"stage": "blocked"}); err != nil {
		return err
	}
	if _, err := e.store.AddActivity(taskID, "task.stopped", nil); err != nil {
		return err
	}
	return nil
}

// IsRunning reports whether the task with the given ID is currently running.
func (e *Executor) IsRunning(taskID int64) bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	_, ok := e.running[taskID]
	return ok
}

// RunningCount returns the number of currently running tasks.
func (e *Executor) RunningCount() int {
	e.mu.Lock()
	defer e.mu.Unlock()
	return len(e.running)
}

// run is the goroutine that executes the full task pipeline:
// classify → worktree → agent loop → verify → commit → validation.
func (e *Executor) run(ctx context.Context, taskID int64) {
	defer func() {
		e.mu.Lock()
		delete(e.running, taskID)
		e.mu.Unlock()
	}()

	task, err := e.store.Get(taskID)
	if err != nil {
		log.Printf("[executor] task %d: get failed: %v", taskID, err)
		e.markBlocked(taskID, fmt.Sprintf("failed to get task: %v", err))
		return
	}

	workflow := ClassifyWorkflow(task.Title, task.Description)
	iterLimit := phases.MaxIterations(workflow)
	log.Printf("[executor] task %d: workflow=%s limit=%d title=%q", taskID, workflow, iterLimit, task.Title)

	_, _ = e.store.AddActivity(taskID, "executor.classify", map[string]interface{}{
		"workflow":        workflow,
		"iteration_limit": iterLimit,
	})

	// Create hook runner if configured.
	var hookRunner *hooks.HookRunner
	if e.hooksConfigPath != "" {
		hookRunner = hooks.NewHookRunner(e.hooksConfigPath)
		_ = hookRunner // Will be wired into per-tool-call dispatch later.
	}

	if ctx.Err() != nil {
		return
	}

	// Create worktree if repo is configured.
	var workDir string
	var wt *Worktree
	if e.repoDir != "" {
		wt, err = CreateWorktree(e.repoDir, taskID)
		if err != nil {
			log.Printf("[executor] task %d: worktree failed: %v", taskID, err)
			e.markBlocked(taskID, fmt.Sprintf("worktree creation failed: %v", err))
			return
		}
		defer wt.Cleanup()
		workDir = wt.Dir
		_, _ = e.store.AddActivity(taskID, "executor.worktree", map[string]interface{}{
			"dir":    wt.Dir,
			"branch": wt.Branch,
		})
	} else {
		workDir = "."
	}

	if ctx.Err() != nil {
		return
	}

	// Run agent loop if a Claude client is configured.
	if e.client != nil {
		tools := NewToolSet(workDir, e.store)
		agent := NewAgentLoop(e.client, tools, taskID, iterLimit, func(eventType string, data map[string]interface{}) {
			_, _ = e.store.AddActivity(taskID, "agent."+eventType, data)
		})

		sysPrompt := buildSystemPrompt(task, workflow)
		userPrompt := buildUserPrompt(task)

		_, _ = e.store.AddActivity(taskID, "executor.agent_start", nil)
		result, err := agent.Run(ctx, sysPrompt, userPrompt)
		if err != nil {
			if ctx.Err() != nil {
				return // Cancelled — Stop() handles the stage update.
			}
			log.Printf("[executor] task %d: agent error: %v", taskID, err)
			e.markBlocked(taskID, fmt.Sprintf("agent error: %v", err))
			return
		}

		_, _ = e.store.AddActivity(taskID, "executor.agent_done", map[string]interface{}{
			"iterations":    result.Iterations,
			"hit_limit":     result.HitLimit,
			"input_tokens":  result.TotalInputTokens,
			"output_tokens": result.TotalOutputTokens,
			"tool_calls":    len(result.ToolCalls),
		})

		if result.HitLimit {
			e.markBlocked(taskID, fmt.Sprintf("hit iteration limit (%d)", iterLimit))
			return
		}
	}

	// Run L1 verification if we have a real working directory.
	if workDir != "." {
		vr := VerifyL1(ctx, workDir)
		_, _ = e.store.AddActivity(taskID, "executor.verify_l1", map[string]interface{}{
			"passed": vr.Passed,
			"errors": vr.Errors,
		})
		if !vr.Passed {
			log.Printf("[executor] task %d: L1 failed: %s", taskID, strings.Join(vr.Errors, "; "))
			e.markBlocked(taskID, fmt.Sprintf("L1 verification failed: %s", strings.Join(vr.Errors, "; ")))
			return
		}
	}

	// Commit changes in worktree.
	if wt != nil && wt.HasChanges() {
		hash, err := wt.Commit(fmt.Sprintf("feat(task-%d): %s", taskID, task.Title))
		if err != nil {
			e.markBlocked(taskID, fmt.Sprintf("commit failed: %v", err))
			return
		}
		_, _ = e.store.AddActivity(taskID, "executor.commit", map[string]interface{}{
			"hash": hash,
		})
		log.Printf("[executor] task %d: committed %s", taskID, hash)
	}

	// Move to validation.
	_, _ = e.store.Update(taskID, map[string]interface{}{"stage": "validation"})
	_, _ = e.store.AddActivity(taskID, "executor.complete", map[string]interface{}{
		"workflow": workflow,
	})
	log.Printf("[executor] task %d: complete → validation", taskID)
}

// markBlocked transitions a task to blocked with a reason.
func (e *Executor) markBlocked(taskID int64, reason string) {
	_, _ = e.store.Update(taskID, map[string]interface{}{"stage": "blocked"})
	_, _ = e.store.AddActivity(taskID, "task.blocked", map[string]interface{}{"reason": reason})
}

// buildSystemPrompt constructs the system prompt for the agent.
func buildSystemPrompt(task *store.Task, workflow string) string {
	var b strings.Builder
	b.WriteString("You are an autonomous software engineering agent. Complete the task described below.\n\n")
	b.WriteString("## Task\n")
	fmt.Fprintf(&b, "Title: %s\n", task.Title)
	if task.Description != "" {
		fmt.Fprintf(&b, "Description: %s\n", task.Description)
	}
	fmt.Fprintf(&b, "\nWorkflow: %s\n", workflow)

	switch workflow {
	case "micro":
		b.WriteString("\nThis is a trivial change. Make it directly and use task_update to move to validation.\n")
		b.WriteString("Do NOT run builds or verification — the pipeline handles that.\n")
	case "quick":
		b.WriteString("\nThis is a straightforward change. Implement it, then use task_update to move to validation.\n")
	default:
		b.WriteString("\nThis is a complex task. Plan your approach, implement step by step, and verify.\n")
	}

	b.WriteString("\n## Available Tools\n")
	b.WriteString("- file_read: Read files\n- file_write: Write/create files\n- bash: Run shell commands\n")
	b.WriteString("- list_files: List directory contents\n- task_update: Update task stage/notes\n")

	return b.String()
}

// buildUserPrompt constructs the initial user message for the agent.
func buildUserPrompt(task *store.Task) string {
	if task.Description != "" {
		return fmt.Sprintf("Implement: %s\n\n%s", task.Title, task.Description)
	}
	return fmt.Sprintf("Implement: %s", task.Title)
}
