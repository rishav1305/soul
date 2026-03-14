package executor

import (
	"context"
	"fmt"
	"sync"

	"github.com/rishav1305/soul-v2/internal/tasks/store"
)

// Config holds the configuration for an Executor.
type Config struct {
	Store       *store.Store
	MaxParallel int
	RepoDir     string
}

// Executor manages the lifecycle of running tasks.
type Executor struct {
	store       *store.Store
	maxParallel int
	repoDir     string
	mu          sync.Mutex
	running     map[int64]context.CancelFunc
}

// New creates a new Executor. MaxParallel defaults to 3 if not set.
func New(cfg Config) *Executor {
	if cfg.MaxParallel <= 0 {
		cfg.MaxParallel = 3
	}
	return &Executor{
		store:       cfg.Store,
		maxParallel: cfg.MaxParallel,
		repoDir:     cfg.RepoDir,
		running:     make(map[int64]context.CancelFunc),
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

	if task.Stage != "backlog" && task.Stage != "blocked" {
		return fmt.Errorf("task %d has stage %q — must be backlog or blocked to start", taskID, task.Stage)
	}

	if _, err := e.store.Update(taskID, map[string]interface{}{"stage": "active"}); err != nil {
		return err
	}

	workflow := ClassifyWorkflow(task.Title, task.Description)
	if err := e.store.AddActivity(taskID, "task.started", map[string]interface{}{"workflow": workflow}); err != nil {
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
	if err := e.store.AddActivity(taskID, "task.stopped", nil); err != nil {
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

// run is the goroutine that executes the task pipeline.
// This is a placeholder that will be replaced in Task 8 with the full pipeline.
func (e *Executor) run(ctx context.Context, taskID int64) {
	defer func() {
		e.mu.Lock()
		delete(e.running, taskID)
		e.mu.Unlock()
	}()

	task, err := e.store.Get(taskID)
	if err != nil {
		return
	}

	workflow := ClassifyWorkflow(task.Title, task.Description)
	iterLimit := IterationLimit(workflow)

	_ = e.store.AddActivity(taskID, "executor.classify", map[string]interface{}{
		"workflow":        workflow,
		"iteration_limit": iterLimit,
	})

	if ctx.Err() != nil {
		return
	}

	_, _ = e.store.Update(taskID, map[string]interface{}{"stage": "validation"})
	_ = e.store.AddActivity(taskID, "executor.complete", map[string]interface{}{"workflow": workflow})
}
