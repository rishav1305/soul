# Autonomous Execution Engine — Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the autonomous task execution pipeline — worktree-isolated agent loop with Claude API tool calling, workflow classification, per-step verification, and start/stop lifecycle management.

**Architecture:** The tasks server (`internal/tasks/`) gains an executor package that processes tasks in background goroutines. Each task gets a git worktree for isolation, a workflow classification (micro/quick/full) that controls iteration limits, and an agent loop that streams Claude API responses, dispatches tool calls, and runs verification after each step. The executor owns the full lifecycle: classify → worktree → agent loop → verify → commit → update stage.

**Tech Stack:** Go 1.24, `internal/chat/stream` (Claude API client), `os/exec` for git/build commands, `internal/tasks/store` for state persistence, SSE broadcaster for real-time activity streaming.

**Deferred to later plans:**
- Product system (gRPC plugins)
- Advanced verification (E2E, visual regression, load tests)
- Model routing (Haiku/Sonnet/Opus per phase) — starts with single model
- Context window management / emergency cutoff
- Skill system injection
- Git merge to main (executor commits to worktree branch; merge is manual for now)

---

## File Structure

| File | Responsibility |
|------|----------------|
| `internal/tasks/executor/executor.go` | Task processor — lifecycle orchestration, start/stop, concurrent task tracking |
| `internal/tasks/executor/executor_test.go` | Unit tests for executor |
| `internal/tasks/executor/worktree.go` | Git worktree creation, cleanup, commit |
| `internal/tasks/executor/worktree_test.go` | Worktree tests (using temp git repos) |
| `internal/tasks/executor/classify.go` | Workflow classifier — micro/quick/full |
| `internal/tasks/executor/classify_test.go` | Classifier tests |
| `internal/tasks/executor/agent.go` | Agent loop — Claude API tool-calling iteration |
| `internal/tasks/executor/agent_test.go` | Agent loop tests (mock Claude client) |
| `internal/tasks/executor/tools.go` | Tool definitions and dispatch |
| `internal/tasks/executor/tools_test.go` | Tool tests |
| `internal/tasks/executor/verify.go` | Per-step verification (L1: go vet + tsc) |
| `internal/tasks/executor/verify_test.go` | Verification tests |
| `internal/tasks/server/server.go` | MODIFY — wire executor, implement start/stop handlers |
| `cmd/tasks/main.go` | MODIFY — create executor, pass to server |

---

## Chunk 1: Foundation — Classifier + Worktree + Executor Shell

### Task 1: Workflow Classifier

**Files:**
- Create: `internal/tasks/executor/classify.go`
- Test: `internal/tasks/executor/classify_test.go`

- [ ] **Step 1: Write classifier tests**

```go
package executor

import "testing"

func TestClassifyWorkflow(t *testing.T) {
	tests := []struct {
		name     string
		title    string
		desc     string
		want     string
	}{
		{"micro: add button", "Add save button to toolbar", "", "micro"},
		{"micro: fix typo", "Fix typo in header", "", "micro"},
		{"micro: change color", "Change sidebar color to blue", "", "micro"},
		{"micro: rename", "Rename ProductRail component", "", "micro"},
		{"micro: add tooltip", "Add tooltip to Settings button", "", "micro"},
		{"quick: default", "Improve error messages", "", "quick"},
		{"quick: add feature", "Add search filtering", "", "quick"},
		{"full: refactor", "Refactor authentication flow", "", "full"},
		{"full: new feature", "New feature: task dependencies", "", "full"},
		{"full: add api", "Add API endpoint for reports", "", "full"},
		{"full: database", "Database migration for audit logs", "", "full"},
		{"override in desc", "Update UI", "refactor the whole panel", "full"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ClassifyWorkflow(tt.title, tt.desc)
			if got != tt.want {
				t.Errorf("ClassifyWorkflow(%q, %q) = %q, want %q", tt.title, tt.desc, got, tt.want)
			}
		})
	}
}

func TestWorkflowIterationLimit(t *testing.T) {
	tests := []struct {
		workflow string
		want     int
	}{
		{"micro", 15},
		{"quick", 30},
		{"full", 40},
		{"unknown", 40},
	}
	for _, tt := range tests {
		t.Run(tt.workflow, func(t *testing.T) {
			got := IterationLimit(tt.workflow)
			if got != tt.want {
				t.Errorf("IterationLimit(%q) = %d, want %d", tt.workflow, got, tt.want)
			}
		})
	}
}
```

- [ ] **Step 2: Run tests — verify they fail**

Run: `cd /home/rishav/soul-v2 && go test ./internal/tasks/executor/ -run TestClassify -v`
Expected: compilation error — package doesn't exist yet

- [ ] **Step 3: Implement classifier**

```go
package executor

import "strings"

// ClassifyWorkflow determines the workflow type from task title and description.
// Returns "micro", "quick", or "full".
func ClassifyWorkflow(title, description string) string {
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

// IterationLimit returns the maximum agent iterations for a workflow type.
func IterationLimit(workflow string) int {
	switch workflow {
	case "micro":
		return 15
	case "quick":
		return 30
	default:
		return 40
	}
}
```

- [ ] **Step 4: Run tests — verify they pass**

Run: `cd /home/rishav/soul-v2 && go test ./internal/tasks/executor/ -run TestClassify -v && go test ./internal/tasks/executor/ -run TestWorkflowIteration -v`
Expected: all PASS

- [ ] **Step 5: Commit**

```bash
git add internal/tasks/executor/classify.go internal/tasks/executor/classify_test.go
git commit -m "feat: add workflow classifier for autonomous execution"
```

---

### Task 2: Worktree Manager

**Files:**
- Create: `internal/tasks/executor/worktree.go`
- Test: `internal/tasks/executor/worktree_test.go`

The worktree manager creates isolated git worktrees for each task, commits changes when done, and cleans up.

- [ ] **Step 1: Write worktree tests**

```go
package executor

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// setupTestRepo creates a temporary git repo for testing.
func setupTestRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	run := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@test.com",
			"GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@test.com")
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v failed: %v\n%s", args, err, out)
		}
	}
	run("init", "-b", "main")
	run("config", "user.email", "test@test.com")
	run("config", "user.name", "test")
	os.WriteFile(filepath.Join(dir, "README.md"), []byte("# Test"), 0644)
	run("add", ".")
	run("commit", "-m", "init")
	return dir
}

func TestWorktreeCreateAndCleanup(t *testing.T) {
	repo := setupTestRepo(t)
	wt, err := CreateWorktree(repo, 42)
	if err != nil {
		t.Fatalf("CreateWorktree: %v", err)
	}
	defer wt.Cleanup()

	// Verify worktree directory exists.
	if _, err := os.Stat(wt.Dir); err != nil {
		t.Fatalf("worktree dir should exist: %v", err)
	}

	// Verify branch name.
	want := "task/42"
	if wt.Branch != want {
		t.Errorf("branch = %q, want %q", wt.Branch, want)
	}

	// Verify files from main are present.
	readme := filepath.Join(wt.Dir, "README.md")
	if _, err := os.Stat(readme); err != nil {
		t.Errorf("README.md should exist in worktree: %v", err)
	}

	// Cleanup and verify removal.
	if err := wt.Cleanup(); err != nil {
		t.Fatalf("Cleanup: %v", err)
	}
	if _, err := os.Stat(wt.Dir); !os.IsNotExist(err) {
		t.Errorf("worktree dir should be removed after cleanup")
	}
}

func TestWorktreeCommit(t *testing.T) {
	repo := setupTestRepo(t)
	wt, err := CreateWorktree(repo, 99)
	if err != nil {
		t.Fatalf("CreateWorktree: %v", err)
	}
	defer wt.Cleanup()

	// Create a new file in the worktree.
	newFile := filepath.Join(wt.Dir, "feature.go")
	os.WriteFile(newFile, []byte("package main\n"), 0644)

	// Commit should succeed.
	hash, err := wt.Commit("feat: add feature")
	if err != nil {
		t.Fatalf("Commit: %v", err)
	}
	if hash == "" {
		t.Error("commit hash should not be empty")
	}
}

func TestWorktreeCommitNoChanges(t *testing.T) {
	repo := setupTestRepo(t)
	wt, err := CreateWorktree(repo, 100)
	if err != nil {
		t.Fatalf("CreateWorktree: %v", err)
	}
	defer wt.Cleanup()

	// Commit with no changes should return empty hash, no error.
	hash, err := wt.Commit("empty commit")
	if err != nil {
		t.Fatalf("Commit with no changes: %v", err)
	}
	if hash != "" {
		t.Errorf("expected empty hash for no-change commit, got %q", hash)
	}
}

func TestWorktreeHasChanges(t *testing.T) {
	repo := setupTestRepo(t)
	wt, err := CreateWorktree(repo, 101)
	if err != nil {
		t.Fatalf("CreateWorktree: %v", err)
	}
	defer wt.Cleanup()

	// No changes initially.
	if wt.HasChanges() {
		t.Error("should have no changes initially")
	}

	// Create a file.
	os.WriteFile(filepath.Join(wt.Dir, "new.txt"), []byte("hello"), 0644)
	if !wt.HasChanges() {
		t.Error("should have changes after creating file")
	}
}
```

- [ ] **Step 2: Run tests — verify they fail**

Run: `cd /home/rishav/soul-v2 && go test ./internal/tasks/executor/ -run TestWorktree -v`
Expected: compilation error — Worktree type doesn't exist

- [ ] **Step 3: Implement worktree manager**

```go
package executor

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Worktree represents an isolated git worktree for task execution.
type Worktree struct {
	Dir    string // absolute path to the worktree directory
	Branch string // branch name (e.g., "task/42")
	repo   string // path to the main repository
}

// CreateWorktree creates a new git worktree for the given task ID.
// The worktree is created in {repo}/.worktrees/task-{id}/ on branch task/{id}.
func CreateWorktree(repoDir string, taskID int64) (*Worktree, error) {
	branch := fmt.Sprintf("task/%d", taskID)
	wtDir := filepath.Join(repoDir, ".worktrees", fmt.Sprintf("task-%d", taskID))

	// Remove stale worktree if it exists.
	if _, err := os.Stat(wtDir); err == nil {
		gitCmd(repoDir, "worktree", "remove", "--force", wtDir)
		os.RemoveAll(wtDir)
	}

	// Create the worktree with a new branch.
	if err := gitCmd(repoDir, "worktree", "add", "-b", branch, wtDir, "HEAD"); err != nil {
		return nil, fmt.Errorf("create worktree: %w", err)
	}

	return &Worktree{
		Dir:    wtDir,
		Branch: branch,
		repo:   repoDir,
	}, nil
}

// HasChanges returns true if the worktree has uncommitted changes.
func (wt *Worktree) HasChanges() bool {
	cmd := exec.Command("git", "status", "--porcelain")
	cmd.Dir = wt.Dir
	out, err := cmd.Output()
	if err != nil {
		return false
	}
	return len(strings.TrimSpace(string(out))) > 0
}

// Commit stages all changes and commits with the given message.
// Returns the commit hash, or empty string if there were no changes.
func (wt *Worktree) Commit(message string) (string, error) {
	if !wt.HasChanges() {
		return "", nil
	}

	if err := gitCmd(wt.Dir, "add", "-A"); err != nil {
		return "", fmt.Errorf("git add: %w", err)
	}

	if err := gitCmd(wt.Dir, "commit", "-m", message); err != nil {
		return "", fmt.Errorf("git commit: %w", err)
	}

	cmd := exec.Command("git", "rev-parse", "HEAD")
	cmd.Dir = wt.Dir
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git rev-parse: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

// Cleanup removes the worktree and its branch.
func (wt *Worktree) Cleanup() error {
	// Remove worktree.
	_ = gitCmd(wt.repo, "worktree", "remove", "--force", wt.Dir)
	// Clean up directory if remove didn't fully work.
	os.RemoveAll(wt.Dir)
	return nil
}

// gitCmd runs a git command in the given directory.
func gitCmd(dir string, args ...string) error {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git %s: %s", strings.Join(args, " "), strings.TrimSpace(string(out)))
	}
	return nil
}
```

- [ ] **Step 4: Run tests — verify they pass**

Run: `cd /home/rishav/soul-v2 && go test ./internal/tasks/executor/ -run TestWorktree -v`
Expected: all PASS

- [ ] **Step 5: Commit**

```bash
git add internal/tasks/executor/worktree.go internal/tasks/executor/worktree_test.go
git commit -m "feat: add git worktree manager for task isolation"
```

---

### Task 3: Executor Shell — Lifecycle Management

**Files:**
- Create: `internal/tasks/executor/executor.go`
- Test: `internal/tasks/executor/executor_test.go`

The executor manages concurrent task execution, enforces limits, and owns the start/stop lifecycle.

- [ ] **Step 1: Write executor tests**

```go
package executor

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/rishav1305/soul-v2/internal/tasks/store"
)

func TestExecutorStartStop(t *testing.T) {
	s := openTestStore(t)
	e := New(Config{
		Store:       s,
		MaxParallel: 2,
	})

	task, _ := s.Create("Test task", "description", "")

	// Start the task.
	err := e.Start(context.Background(), task.ID)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}

	// Verify task is tracked as running.
	if !e.IsRunning(task.ID) {
		t.Error("task should be running after Start")
	}

	// Stop the task.
	err = e.Stop(task.ID)
	if err != nil {
		t.Fatalf("Stop: %v", err)
	}

	// Wait briefly for cleanup.
	time.Sleep(100 * time.Millisecond)

	if e.IsRunning(task.ID) {
		t.Error("task should not be running after Stop")
	}
}

func TestExecutorRejectsOverLimit(t *testing.T) {
	s := openTestStore(t)
	e := New(Config{
		Store:       s,
		MaxParallel: 1,
	})

	t1, _ := s.Create("Task 1", "", "")
	t2, _ := s.Create("Task 2", "", "")

	err := e.Start(context.Background(), t1.ID)
	if err != nil {
		t.Fatalf("Start t1: %v", err)
	}
	defer e.Stop(t1.ID)

	err = e.Start(context.Background(), t2.ID)
	if err == nil {
		t.Error("expected error when exceeding MaxParallel")
	}
}

func TestExecutorRejectsDuplicateStart(t *testing.T) {
	s := openTestStore(t)
	e := New(Config{
		Store:       s,
		MaxParallel: 2,
	})

	task, _ := s.Create("Dup task", "", "")
	err := e.Start(context.Background(), task.ID)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer e.Stop(task.ID)

	err = e.Start(context.Background(), task.ID)
	if err == nil {
		t.Error("expected error for duplicate start")
	}
}

func TestExecutorRunningCount(t *testing.T) {
	s := openTestStore(t)
	e := New(Config{
		Store:       s,
		MaxParallel: 5,
	})

	if e.RunningCount() != 0 {
		t.Errorf("RunningCount = %d, want 0", e.RunningCount())
	}

	task, _ := s.Create("Count task", "", "")
	e.Start(context.Background(), task.ID)
	defer e.Stop(task.ID)

	if e.RunningCount() != 1 {
		t.Errorf("RunningCount = %d, want 1", e.RunningCount())
	}
}

// openTestStore creates a temp store for executor tests.
func openTestStore(t *testing.T) *store.Store {
	t.Helper()
	dir := t.TempDir()
	s, err := store.Open(dir + "/test.db")
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}
```

- [ ] **Step 2: Run tests — verify they fail**

Run: `cd /home/rishav/soul-v2 && go test ./internal/tasks/executor/ -run TestExecutor -v`
Expected: compilation error — Executor type doesn't exist

- [ ] **Step 3: Implement executor**

```go
package executor

import (
	"context"
	"fmt"
	"log"
	"sync"

	"github.com/rishav1305/soul-v2/internal/tasks/store"
)

// Broadcaster sends events to SSE subscribers.
type Broadcaster interface {
	Broadcast(event interface{})
}

// Config configures the Executor.
type Config struct {
	Store       *store.Store
	MaxParallel int // max concurrent task executions (default 3)
	RepoDir     string // project root for worktrees
}

// Executor manages autonomous task execution.
type Executor struct {
	store       *store.Store
	maxParallel int
	repoDir     string

	mu       sync.Mutex
	running  map[int64]context.CancelFunc
}

// New creates a new Executor.
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

// Start begins autonomous execution of a task.
func (e *Executor) Start(ctx context.Context, taskID int64) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if _, ok := e.running[taskID]; ok {
		return fmt.Errorf("task %d is already running", taskID)
	}
	if len(e.running) >= e.maxParallel {
		return fmt.Errorf("max parallel tasks (%d) reached", e.maxParallel)
	}

	// Verify task exists and is startable.
	task, err := e.store.Get(taskID)
	if err != nil {
		return fmt.Errorf("get task: %w", err)
	}
	if task.Stage != "backlog" && task.Stage != "blocked" {
		return fmt.Errorf("task %d has stage %q — must be backlog or blocked to start", taskID, task.Stage)
	}

	// Move to active.
	if _, err := e.store.Update(taskID, map[string]interface{}{"stage": "active"}); err != nil {
		return fmt.Errorf("update stage: %w", err)
	}
	e.store.AddActivity(taskID, "task.started", map[string]interface{}{
		"workflow": ClassifyWorkflow(task.Title, task.Description),
	})

	taskCtx, cancel := context.WithCancel(ctx)
	e.running[taskID] = cancel

	go e.run(taskCtx, taskID)
	return nil
}

// Stop cancels a running task execution.
func (e *Executor) Stop(taskID int64) error {
	e.mu.Lock()
	cancel, ok := e.running[taskID]
	if !ok {
		e.mu.Unlock()
		return fmt.Errorf("task %d is not running", taskID)
	}
	delete(e.running, taskID)
	e.mu.Unlock()

	cancel()

	e.store.Update(taskID, map[string]interface{}{"stage": "blocked"})
	e.store.AddActivity(taskID, "task.stopped", map[string]interface{}{
		"reason": "manual stop",
	})
	return nil
}

// IsRunning returns whether a task is currently executing.
func (e *Executor) IsRunning(taskID int64) bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	_, ok := e.running[taskID]
	return ok
}

// RunningCount returns the number of currently executing tasks.
func (e *Executor) RunningCount() int {
	e.mu.Lock()
	defer e.mu.Unlock()
	return len(e.running)
}

// run is the main execution goroutine for a task.
func (e *Executor) run(ctx context.Context, taskID int64) {
	defer func() {
		e.mu.Lock()
		delete(e.running, taskID)
		e.mu.Unlock()
	}()

	task, err := e.store.Get(taskID)
	if err != nil {
		log.Printf("[executor] task %d: get failed: %v", taskID, err)
		return
	}

	workflow := ClassifyWorkflow(task.Title, task.Description)
	log.Printf("[executor] task %d: workflow=%s title=%q", taskID, workflow, task.Title)

	e.store.AddActivity(taskID, "executor.classify", map[string]interface{}{
		"workflow":        workflow,
		"iteration_limit": IterationLimit(workflow),
	})

	// TODO: In Task 4+5, this will create a worktree and run the agent loop.
	// For now, just move to validation to prove the lifecycle works.
	if ctx.Err() != nil {
		return
	}

	e.store.Update(taskID, map[string]interface{}{"stage": "validation"})
	e.store.AddActivity(taskID, "executor.complete", map[string]interface{}{
		"workflow": workflow,
	})
}
```

- [ ] **Step 4: Run tests — verify they pass**

Run: `cd /home/rishav/soul-v2 && go test ./internal/tasks/executor/ -run TestExecutor -v`
Expected: all PASS

- [ ] **Step 5: Commit**

```bash
git add internal/tasks/executor/executor.go internal/tasks/executor/executor_test.go
git commit -m "feat: add executor with start/stop lifecycle management"
```

---

## Chunk 2: Tools + Agent Loop

### Task 4: Tool Definitions and Dispatch

**Files:**
- Create: `internal/tasks/executor/tools.go`
- Test: `internal/tasks/executor/tools_test.go`

Agent tools let the Claude model interact with the filesystem and task state. Each tool has a name, JSON schema, and execute function.

- [ ] **Step 1: Write tool tests**

```go
package executor

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestToolFileRead(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "hello.txt"), []byte("world"), 0644)

	tools := NewToolSet(dir, nil)
	result, err := tools.Execute("file_read", `{"path": "hello.txt"}`)
	if err != nil {
		t.Fatalf("Execute file_read: %v", err)
	}
	if result != "world" {
		t.Errorf("file_read = %q, want %q", result, "world")
	}
}

func TestToolFileReadPathTraversal(t *testing.T) {
	dir := t.TempDir()
	tools := NewToolSet(dir, nil)
	_, err := tools.Execute("file_read", `{"path": "../../etc/passwd"}`)
	if err == nil {
		t.Error("expected error for path traversal")
	}
}

func TestToolFileWrite(t *testing.T) {
	dir := t.TempDir()
	tools := NewToolSet(dir, nil)

	result, err := tools.Execute("file_write", `{"path": "new.txt", "content": "hello world"}`)
	if err != nil {
		t.Fatalf("Execute file_write: %v", err)
	}
	if !strings.Contains(result, "wrote") {
		t.Errorf("unexpected result: %q", result)
	}

	data, _ := os.ReadFile(filepath.Join(dir, "new.txt"))
	if string(data) != "hello world" {
		t.Errorf("file content = %q, want %q", string(data), "hello world")
	}
}

func TestToolFileWriteCreatesDirectories(t *testing.T) {
	dir := t.TempDir()
	tools := NewToolSet(dir, nil)

	_, err := tools.Execute("file_write", `{"path": "sub/dir/file.go", "content": "package main"}`)
	if err != nil {
		t.Fatalf("Execute file_write nested: %v", err)
	}

	data, _ := os.ReadFile(filepath.Join(dir, "sub", "dir", "file.go"))
	if string(data) != "package main" {
		t.Errorf("file content = %q", string(data))
	}
}

func TestToolBash(t *testing.T) {
	dir := t.TempDir()
	tools := NewToolSet(dir, nil)

	result, err := tools.Execute("bash", `{"command": "echo hello"}`)
	if err != nil {
		t.Fatalf("Execute bash: %v", err)
	}
	if !strings.Contains(result, "hello") {
		t.Errorf("bash result = %q, want to contain 'hello'", result)
	}
}

func TestToolBashTimeout(t *testing.T) {
	dir := t.TempDir()
	tools := NewToolSet(dir, nil)

	// This should not hang — bash tool has a timeout.
	_, err := tools.Execute("bash", `{"command": "sleep 0.1 && echo done"}`)
	if err != nil {
		t.Fatalf("Execute bash: %v", err)
	}
}

func TestToolUnknown(t *testing.T) {
	dir := t.TempDir()
	tools := NewToolSet(dir, nil)

	_, err := tools.Execute("nonexistent_tool", `{}`)
	if err == nil {
		t.Error("expected error for unknown tool")
	}
}

func TestToolDefinitions(t *testing.T) {
	tools := NewToolSet(t.TempDir(), nil)
	defs := tools.Definitions()
	if len(defs) == 0 {
		t.Error("expected at least one tool definition")
	}

	// Check that all definitions have required fields.
	for _, d := range defs {
		if d.Name == "" {
			t.Error("tool definition has empty name")
		}
		if d.Description == "" {
			t.Errorf("tool %q has empty description", d.Name)
		}
		if len(d.InputSchema) == 0 {
			t.Errorf("tool %q has empty input schema", d.Name)
		}
	}
}
```

- [ ] **Step 2: Run tests — verify they fail**

Run: `cd /home/rishav/soul-v2 && go test ./internal/tasks/executor/ -run TestTool -v`
Expected: compilation error

- [ ] **Step 3: Implement tools**

```go
package executor

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/rishav1305/soul-v2/internal/chat/stream"
	"github.com/rishav1305/soul-v2/internal/tasks/store"
)

const bashTimeout = 30 * time.Second

// ToolSet manages available tools scoped to a working directory.
type ToolSet struct {
	rootDir string
	store   *store.Store
}

// NewToolSet creates a new ToolSet scoped to the given directory.
func NewToolSet(rootDir string, s *store.Store) *ToolSet {
	return &ToolSet{rootDir: rootDir, store: s}
}

// Definitions returns the tool definitions for the Claude API.
func (ts *ToolSet) Definitions() []stream.Tool {
	return []stream.Tool{
		{
			Name:        "file_read",
			Description: "Read the contents of a file at the given path, relative to the project root.",
			InputSchema: json.RawMessage(`{"type":"object","properties":{"path":{"type":"string","description":"Relative file path"}},"required":["path"]}`),
		},
		{
			Name:        "file_write",
			Description: "Write content to a file at the given path, relative to the project root. Creates parent directories if needed.",
			InputSchema: json.RawMessage(`{"type":"object","properties":{"path":{"type":"string","description":"Relative file path"},"content":{"type":"string","description":"File content to write"}},"required":["path","content"]}`),
		},
		{
			Name:        "bash",
			Description: "Execute a bash command in the project directory. Use for builds, tests, git operations, etc. Commands time out after 30 seconds.",
			InputSchema: json.RawMessage(`{"type":"object","properties":{"command":{"type":"string","description":"The bash command to execute"}},"required":["command"]}`),
		},
		{
			Name:        "list_files",
			Description: "List files in a directory relative to the project root. Returns one path per line.",
			InputSchema: json.RawMessage(`{"type":"object","properties":{"path":{"type":"string","description":"Relative directory path (empty for project root)"},"recursive":{"type":"boolean","description":"List recursively (default false)"}},"required":[]}`),
		},
		{
			Name:        "task_update",
			Description: "Update the current task's stage or add a note. Use stage 'validation' when implementation is complete. Use stage 'blocked' if stuck.",
			InputSchema: json.RawMessage(`{"type":"object","properties":{"stage":{"type":"string","enum":["validation","blocked"],"description":"New stage"},"note":{"type":"string","description":"Activity note"}},"required":[]}`),
		},
	}
}

// Execute runs a tool with the given JSON input and returns the result string.
func (ts *ToolSet) Execute(name, input string) (string, error) {
	switch name {
	case "file_read":
		return ts.execFileRead(input)
	case "file_write":
		return ts.execFileWrite(input)
	case "bash":
		return ts.execBash(input)
	case "list_files":
		return ts.execListFiles(input)
	case "task_update":
		return ts.execTaskUpdate(input)
	default:
		return "", fmt.Errorf("unknown tool: %s", name)
	}
}

func (ts *ToolSet) resolvePath(relPath string) (string, error) {
	clean := filepath.Clean(relPath)
	abs := filepath.Join(ts.rootDir, clean)
	absRoot, _ := filepath.Abs(ts.rootDir)
	absPath, _ := filepath.Abs(abs)
	if !strings.HasPrefix(absPath, absRoot+string(filepath.Separator)) && absPath != absRoot {
		return "", fmt.Errorf("path traversal blocked: %s", relPath)
	}
	return abs, nil
}

func (ts *ToolSet) execFileRead(input string) (string, error) {
	var args struct {
		Path string `json:"path"`
	}
	if err := json.Unmarshal([]byte(input), &args); err != nil {
		return "", fmt.Errorf("invalid input: %w", err)
	}
	path, err := ts.resolvePath(args.Path)
	if err != nil {
		return "", err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read %s: %w", args.Path, err)
	}
	content := string(data)
	if len(content) > 100_000 {
		content = content[:100_000] + "\n...(truncated at 100KB)"
	}
	return content, nil
}

func (ts *ToolSet) execFileWrite(input string) (string, error) {
	var args struct {
		Path    string `json:"path"`
		Content string `json:"content"`
	}
	if err := json.Unmarshal([]byte(input), &args); err != nil {
		return "", fmt.Errorf("invalid input: %w", err)
	}
	path, err := ts.resolvePath(args.Path)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return "", fmt.Errorf("mkdir: %w", err)
	}
	if err := os.WriteFile(path, []byte(args.Content), 0644); err != nil {
		return "", fmt.Errorf("write %s: %w", args.Path, err)
	}
	return fmt.Sprintf("wrote %d bytes to %s", len(args.Content), args.Path), nil
}

func (ts *ToolSet) execBash(input string) (string, error) {
	var args struct {
		Command string `json:"command"`
	}
	if err := json.Unmarshal([]byte(input), &args); err != nil {
		return "", fmt.Errorf("invalid input: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), bashTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "bash", "-c", args.Command)
	cmd.Dir = ts.rootDir
	out, err := cmd.CombinedOutput()
	result := string(out)
	if len(result) > 50_000 {
		result = result[:50_000] + "\n...(truncated at 50KB)"
	}
	if err != nil {
		return fmt.Sprintf("%s\n\nexit status: %v", result, err), nil
	}
	return result, nil
}

func (ts *ToolSet) execListFiles(input string) (string, error) {
	var args struct {
		Path      string `json:"path"`
		Recursive bool   `json:"recursive"`
	}
	if err := json.Unmarshal([]byte(input), &args); err != nil {
		return "", fmt.Errorf("invalid input: %w", err)
	}

	dir := ts.rootDir
	if args.Path != "" {
		resolved, err := ts.resolvePath(args.Path)
		if err != nil {
			return "", err
		}
		dir = resolved
	}

	var files []string
	if args.Recursive {
		filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			if d.IsDir() {
				name := d.Name()
				if name == ".git" || name == "node_modules" || name == ".worktrees" || name == "dist" {
					return filepath.SkipDir
				}
				return nil
			}
			rel, _ := filepath.Rel(ts.rootDir, path)
			files = append(files, rel)
			if len(files) >= 500 {
				return filepath.SkipAll
			}
			return nil
		})
	} else {
		entries, err := os.ReadDir(dir)
		if err != nil {
			return "", fmt.Errorf("readdir: %w", err)
		}
		for _, e := range entries {
			name := e.Name()
			if e.IsDir() {
				name += "/"
			}
			files = append(files, name)
		}
	}

	return strings.Join(files, "\n"), nil
}

func (ts *ToolSet) execTaskUpdate(input string) (string, error) {
	var args struct {
		Stage string `json:"stage"`
		Note  string `json:"note"`
	}
	if err := json.Unmarshal([]byte(input), &args); err != nil {
		return "", fmt.Errorf("invalid input: %w", err)
	}

	// This is a placeholder — the actual task ID is set by the agent loop.
	return fmt.Sprintf("task_update: stage=%s note=%s", args.Stage, args.Note), nil
}
```

- [ ] **Step 4: Run tests — verify they pass**

Run: `cd /home/rishav/soul-v2 && go test ./internal/tasks/executor/ -run TestTool -v`
Expected: all PASS

- [ ] **Step 5: Commit**

```bash
git add internal/tasks/executor/tools.go internal/tasks/executor/tools_test.go
git commit -m "feat: add agent tool definitions and dispatch"
```

---

### Task 5: Agent Loop

**Files:**
- Create: `internal/tasks/executor/agent.go`
- Test: `internal/tasks/executor/agent_test.go`

The agent loop sends messages to Claude, parses tool calls from the response, executes them, and continues until the model produces a final text response or hits the iteration limit.

- [ ] **Step 1: Write agent loop tests**

```go
package executor

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/rishav1305/soul-v2/internal/chat/stream"
)

// mockSender implements a fake Claude API for testing the agent loop.
type mockSender struct {
	responses []*stream.Response
	calls     int
}

func (m *mockSender) Send(ctx context.Context, req *stream.Request) (*stream.Response, error) {
	if m.calls >= len(m.responses) {
		return &stream.Response{
			StopReason: "end_turn",
			Content:    []stream.ContentBlock{{Type: "text", Text: "done"}},
			Usage:      &stream.Usage{InputTokens: 10, OutputTokens: 5},
		}, nil
	}
	resp := m.responses[m.calls]
	m.calls++
	return resp, nil
}

func TestAgentLoopSimpleResponse(t *testing.T) {
	mock := &mockSender{
		responses: []*stream.Response{
			{
				StopReason: "end_turn",
				Content:    []stream.ContentBlock{{Type: "text", Text: "Task complete."}},
				Usage:      &stream.Usage{InputTokens: 100, OutputTokens: 50},
			},
		},
	}

	agent := &AgentLoop{
		sender:  mock,
		tools:   NewToolSet(t.TempDir(), nil),
		taskID:  1,
		maxIter: 10,
	}

	result, err := agent.Run(context.Background(), "system prompt", "Do something simple")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if result.Text != "Task complete." {
		t.Errorf("result.Text = %q, want %q", result.Text, "Task complete.")
	}
	if result.Iterations != 1 {
		t.Errorf("result.Iterations = %d, want 1", result.Iterations)
	}
}

func TestAgentLoopToolCall(t *testing.T) {
	toolInput, _ := json.Marshal(map[string]string{"command": "echo hello"})

	mock := &mockSender{
		responses: []*stream.Response{
			{
				StopReason: "tool_use",
				Content: []stream.ContentBlock{
					{Type: "text", Text: "Let me run a command."},
					{
						Type:  "tool_use",
						ID:    "call_1",
						Name:  "bash",
						Input: json.RawMessage(toolInput),
					},
				},
				Usage: &stream.Usage{InputTokens: 100, OutputTokens: 50},
			},
			{
				StopReason: "end_turn",
				Content:    []stream.ContentBlock{{Type: "text", Text: "Done."}},
				Usage:      &stream.Usage{InputTokens: 200, OutputTokens: 30},
			},
		},
	}

	agent := &AgentLoop{
		sender:  mock,
		tools:   NewToolSet(t.TempDir(), nil),
		taskID:  1,
		maxIter: 10,
	}

	result, err := agent.Run(context.Background(), "system", "run echo")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if result.Iterations != 2 {
		t.Errorf("iterations = %d, want 2", result.Iterations)
	}
	if result.TotalInputTokens != 300 {
		t.Errorf("input tokens = %d, want 300", result.TotalInputTokens)
	}
}

func TestAgentLoopHitsIterationLimit(t *testing.T) {
	toolInput, _ := json.Marshal(map[string]string{"command": "echo loop"})

	// Every response is a tool call — agent should hit limit.
	resp := &stream.Response{
		StopReason: "tool_use",
		Content: []stream.ContentBlock{
			{Type: "tool_use", ID: "call_x", Name: "bash", Input: json.RawMessage(toolInput)},
		},
		Usage: &stream.Usage{InputTokens: 50, OutputTokens: 20},
	}
	mock := &mockSender{
		responses: []*stream.Response{resp, resp, resp, resp, resp},
	}

	agent := &AgentLoop{
		sender:  mock,
		tools:   NewToolSet(t.TempDir(), nil),
		taskID:  1,
		maxIter: 3,
	}

	result, err := agent.Run(context.Background(), "system", "loop forever")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if result.Iterations != 3 {
		t.Errorf("iterations = %d, want 3 (limit)", result.Iterations)
	}
	if !result.HitLimit {
		t.Error("expected HitLimit to be true")
	}
}

func TestAgentLoopContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately.

	mock := &mockSender{}
	agent := &AgentLoop{
		sender:  mock,
		tools:   NewToolSet(t.TempDir(), nil),
		taskID:  1,
		maxIter: 10,
	}

	_, err := agent.Run(ctx, "system", "do something")
	if err == nil {
		t.Error("expected error for cancelled context")
	}
}
```

- [ ] **Step 2: Run tests — verify they fail**

Run: `cd /home/rishav/soul-v2 && go test ./internal/tasks/executor/ -run TestAgentLoop -v`
Expected: compilation error

- [ ] **Step 3: Implement agent loop**

```go
package executor

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"github.com/rishav1305/soul-v2/internal/chat/stream"
)

// Sender abstracts the Claude API client for testing.
type Sender interface {
	Send(ctx context.Context, req *stream.Request) (*stream.Response, error)
}

// AgentResult holds the outcome of an agent loop execution.
type AgentResult struct {
	Text              string
	Iterations        int
	TotalInputTokens  int
	TotalOutputTokens int
	HitLimit          bool
	ToolCalls         []ToolCallRecord
}

// ToolCallRecord records a single tool invocation.
type ToolCallRecord struct {
	Name   string
	Input  string
	Output string
}

// AgentLoop runs the iterative tool-calling loop with Claude.
type AgentLoop struct {
	sender    Sender
	tools     *ToolSet
	taskID    int64
	maxIter   int
	onActivity func(eventType string, data map[string]interface{})
}

// Run executes the agent loop with the given system prompt and initial user message.
func (a *AgentLoop) Run(ctx context.Context, systemPrompt, userMessage string) (*AgentResult, error) {
	messages := []stream.Message{
		{Role: "user", Content: []stream.ContentBlock{{Type: "text", Text: userMessage}}},
	}

	result := &AgentResult{}
	toolDefs := a.tools.Definitions()

	for iteration := 0; iteration < a.maxIter; iteration++ {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}

		result.Iterations = iteration + 1

		req := &stream.Request{
			MaxTokens: 16384,
			System:    systemPrompt,
			Messages:  messages,
			Tools:     toolDefs,
		}

		log.Printf("[agent] task %d: iteration %d/%d", a.taskID, iteration+1, a.maxIter)

		resp, err := a.sender.Send(ctx, req)
		if err != nil {
			return nil, fmt.Errorf("claude api: %w", err)
		}

		if resp.Usage != nil {
			result.TotalInputTokens += resp.Usage.InputTokens
			result.TotalOutputTokens += resp.Usage.OutputTokens
		}

		// Append assistant response to conversation.
		messages = append(messages, stream.Message{
			Role:    "assistant",
			Content: resp.Content,
		})

		// Check if the model wants to call tools.
		if resp.StopReason == "tool_use" {
			toolResults := a.executeToolCalls(resp.Content, result)
			messages = append(messages, stream.Message{
				Role:    "user",
				Content: toolResults,
			})
			continue
		}

		// end_turn — extract final text.
		var textParts []string
		for _, block := range resp.Content {
			if block.Type == "text" {
				textParts = append(textParts, block.Text)
			}
		}
		result.Text = strings.Join(textParts, "\n")
		return result, nil
	}

	// Hit iteration limit.
	result.HitLimit = true
	log.Printf("[agent] task %d: hit iteration limit (%d)", a.taskID, a.maxIter)
	return result, nil
}

// executeToolCalls processes tool_use blocks and returns tool_result blocks.
func (a *AgentLoop) executeToolCalls(content []stream.ContentBlock, result *AgentResult) []stream.ContentBlock {
	var results []stream.ContentBlock
	for _, block := range content {
		if block.Type != "tool_use" {
			continue
		}

		input := string(block.Input)
		log.Printf("[agent] task %d: tool=%s", a.taskID, block.Name)

		output, err := a.tools.Execute(block.Name, input)
		if err != nil {
			output = fmt.Sprintf("error: %v", err)
		}

		record := ToolCallRecord{Name: block.Name, Input: input, Output: output}
		result.ToolCalls = append(result.ToolCalls, record)

		if a.onActivity != nil {
			a.onActivity("agent.tool_call", map[string]interface{}{
				"tool":   block.Name,
				"input":  truncate(input, 200),
				"output": truncate(output, 500),
			})
		}

		results = append(results, stream.ContentBlock{
			Type:      "tool_result",
			ToolUseID: block.ID,
			Content:   output,
		})
	}
	return results
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
```

Note: The `Sender` interface uses `Send()` (non-streaming) rather than `Stream()` for the autonomous agent loop. This simplifies tool-call handling since we don't need to parse SSE events — we get the complete response at once. The `stream.Client` already has a `Send()` method that returns `*stream.Response`.

- [ ] **Step 4: Run tests — verify they pass**

Run: `cd /home/rishav/soul-v2 && go test ./internal/tasks/executor/ -run TestAgentLoop -v`
Expected: all PASS

- [ ] **Step 5: Commit**

```bash
git add internal/tasks/executor/agent.go internal/tasks/executor/agent_test.go
git commit -m "feat: add agent loop with tool-calling iteration"
```

---

## Chunk 3: Verification + Wiring

### Task 6: Per-Step Verification

**Files:**
- Create: `internal/tasks/executor/verify.go`
- Test: `internal/tasks/executor/verify_test.go`

L1 verification runs `go vet` and `tsc --noEmit` after each agent step (when the agent produces file changes).

- [ ] **Step 1: Write verification tests**

```go
package executor

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestVerifyL1_ValidGo(t *testing.T) {
	dir := t.TempDir()
	// Create a valid Go file + go.mod.
	os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module test\ngo 1.24\n"), 0644)
	os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\nfunc main() {}\n"), 0644)

	result := VerifyL1(context.Background(), dir)
	if !result.Passed {
		t.Errorf("expected L1 to pass for valid Go: %v", result.Errors)
	}
}

func TestVerifyL1_InvalidGo(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module test\ngo 1.24\n"), 0644)
	os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\nfunc main() { x := 1 }\n"), 0644)

	result := VerifyL1(context.Background(), dir)
	if result.Passed {
		t.Error("expected L1 to fail for unused variable")
	}
	if len(result.Errors) == 0 {
		t.Error("expected errors to be non-empty")
	}
}

func TestVerifyL1_NoGoMod(t *testing.T) {
	dir := t.TempDir()
	// No go.mod — Go checks should be skipped gracefully.
	os.WriteFile(filepath.Join(dir, "readme.txt"), []byte("hello"), 0644)

	result := VerifyL1(context.Background(), dir)
	// Should pass — no Go project to vet.
	if !result.Passed {
		t.Errorf("expected L1 to pass when no Go project exists: %v", result.Errors)
	}
}

func TestVerifyResult_String(t *testing.T) {
	r := &VerifyResult{
		Passed: false,
		Errors: []string{"go vet failed: unused variable"},
	}
	s := r.String()
	if s == "" {
		t.Error("String() should not be empty")
	}
}
```

- [ ] **Step 2: Run tests — verify they fail**

Run: `cd /home/rishav/soul-v2 && go test ./internal/tasks/executor/ -run TestVerify -v`
Expected: compilation error

- [ ] **Step 3: Implement verification**

```go
package executor

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const verifyTimeout = 60 * time.Second

// VerifyResult holds the outcome of a verification run.
type VerifyResult struct {
	Passed bool
	Errors []string
}

// String returns a human-readable summary.
func (vr *VerifyResult) String() string {
	if vr.Passed {
		return "L1 verification: PASSED"
	}
	return fmt.Sprintf("L1 verification: FAILED\n%s", strings.Join(vr.Errors, "\n"))
}

// VerifyL1 runs L1 static checks: go vet (if Go project) and tsc --noEmit (if TS project).
func VerifyL1(ctx context.Context, dir string) *VerifyResult {
	ctx, cancel := context.WithTimeout(ctx, verifyTimeout)
	defer cancel()

	result := &VerifyResult{Passed: true}

	// Go vet — only if go.mod exists.
	if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
		cmd := exec.CommandContext(ctx, "go", "vet", "./...")
		cmd.Dir = dir
		out, err := cmd.CombinedOutput()
		if err != nil {
			result.Passed = false
			result.Errors = append(result.Errors, fmt.Sprintf("go vet: %s", strings.TrimSpace(string(out))))
		}
	}

	// tsc --noEmit — only if tsconfig.json exists.
	tsconfig := filepath.Join(dir, "tsconfig.json")
	if _, err := os.Stat(tsconfig); err == nil {
		// Check web/ subdirectory too (monorepo pattern).
		cmd := exec.CommandContext(ctx, "npx", "tsc", "--noEmit")
		cmd.Dir = dir
		out, err := cmd.CombinedOutput()
		if err != nil {
			result.Passed = false
			result.Errors = append(result.Errors, fmt.Sprintf("tsc: %s", strings.TrimSpace(string(out))))
		}
	} else {
		// Check web/tsconfig.json for monorepo layout.
		webTsconfig := filepath.Join(dir, "web", "tsconfig.json")
		if _, err := os.Stat(webTsconfig); err == nil {
			cmd := exec.CommandContext(ctx, "npx", "tsc", "--noEmit")
			cmd.Dir = filepath.Join(dir, "web")
			out, err := cmd.CombinedOutput()
			if err != nil {
				result.Passed = false
				result.Errors = append(result.Errors, fmt.Sprintf("tsc: %s", strings.TrimSpace(string(out))))
			}
		}
	}

	return result
}
```

- [ ] **Step 4: Run tests — verify they pass**

Run: `cd /home/rishav/soul-v2 && go test ./internal/tasks/executor/ -run TestVerify -v`
Expected: all PASS

- [ ] **Step 5: Commit**

```bash
git add internal/tasks/executor/verify.go internal/tasks/executor/verify_test.go
git commit -m "feat: add L1 verification gate (go vet + tsc)"
```

---

### Task 7: Wire Executor into Tasks Server

**Files:**
- Modify: `internal/tasks/server/server.go` — add executor field, implement start/stop handlers
- Modify: `cmd/tasks/main.go` — create executor, pass to server
- Test: Update `internal/tasks/server/server_test.go`

- [ ] **Step 1: Add executor option and wire start/stop handlers**

In `internal/tasks/server/server.go`:

1. Add import for executor package
2. Add `executor *executor.Executor` field to `Server` struct
3. Add `WithExecutor(e *executor.Executor) Option` function
4. Replace `handleStartTask` stub with real implementation that calls `s.executor.Start()`
5. Replace `handleStopTask` stub with real implementation that calls `s.executor.Stop()`
6. Add executor status to health endpoint

```go
// In server.go, add to Server struct:
// executor *executor.Executor

// Add option:
// func WithExecutor(e *executor.Executor) Option { return func(srv *Server) { srv.executor = e } }

// Replace handleStartTask:
func (s *Server) handleStartTask(w http.ResponseWriter, r *http.Request) {
	if s.executor == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{
			"error": "executor not configured",
		})
		return
	}

	id, err := parseID(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid id"})
		return
	}

	if err := s.executor.Start(r.Context(), id); err != nil {
		status := http.StatusInternalServerError
		if strings.Contains(err.Error(), "already running") || strings.Contains(err.Error(), "max parallel") {
			status = http.StatusConflict
		}
		if strings.Contains(err.Error(), "not found") {
			status = http.StatusNotFound
		}
		if strings.Contains(err.Error(), "must be backlog or blocked") {
			status = http.StatusBadRequest
		}
		writeJSON(w, status, map[string]string{"error": err.Error()})
		return
	}

	// Log activity + broadcast.
	s.store.AddActivity(id, "task.started", nil)
	task, _ := s.store.Get(id)
	if task != nil {
		data, _ := json.Marshal(task)
		s.broadcaster.Broadcast(Event{Type: "task.started", Data: string(data)})
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "started"})
}

// Replace handleStopTask:
func (s *Server) handleStopTask(w http.ResponseWriter, r *http.Request) {
	if s.executor == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{
			"error": "executor not configured",
		})
		return
	}

	id, err := parseID(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid id"})
		return
	}

	if err := s.executor.Stop(id); err != nil {
		status := http.StatusInternalServerError
		if strings.Contains(err.Error(), "not running") {
			status = http.StatusConflict
		}
		writeJSON(w, status, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "stopped"})
}
```

- [ ] **Step 2: Update cmd/tasks/main.go to create executor**

```go
// In runServe(), after opening task store and before creating server:
exec := executor.New(executor.Config{
	Store:       taskStore,
	MaxParallel: 3,
	RepoDir:     "", // TODO: configure via env var in future
})

// Add to server options:
opts := []server.Option{
	server.WithStore(taskStore),
	server.WithLogger(events.NopLogger{}),
	server.WithHost(host),
	server.WithPort(port),
	server.WithExecutor(exec),
}
```

- [ ] **Step 3: Update server tests for start/stop**

Add tests in `internal/tasks/server/server_test.go`:

```go
func TestStartTask(t *testing.T) {
	// Create a task in backlog stage, then POST /api/tasks/{id}/start.
	// Verify 200 response and task moves to active stage.
}

func TestStopTask(t *testing.T) {
	// Start a task, then POST /api/tasks/{id}/stop.
	// Verify 200 response.
}

func TestStartTask_NoExecutor(t *testing.T) {
	// Server without executor — should return 503.
}
```

- [ ] **Step 4: Run all tests**

Run: `cd /home/rishav/soul-v2 && go test ./internal/tasks/... -v`
Expected: all PASS

- [ ] **Step 5: Commit**

```bash
git add internal/tasks/server/server.go internal/tasks/server/server_test.go cmd/tasks/main.go
git commit -m "feat: wire executor into tasks server, implement start/stop handlers"
```

---

### Task 8: Wire Agent Loop into Executor

**Files:**
- Modify: `internal/tasks/executor/executor.go` — update `run()` to use worktree + agent loop
- Modify: `cmd/tasks/main.go` — pass Claude client to executor

This is the integration task that connects all the pieces: the executor's `run()` method creates a worktree, builds a prompt, runs the agent loop, verifies, and commits.

- [ ] **Step 1: Add Claude client to executor config**

```go
// In executor.go, add to Config:
type Config struct {
	Store       *store.Store
	MaxParallel int
	RepoDir     string
	Client      Sender // Claude API client
	Model       string // model identifier
}
```

- [ ] **Step 2: Update run() to use full pipeline**

Replace the placeholder `run()` in executor.go with:

```go
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
	iterLimit := IterationLimit(workflow)
	log.Printf("[executor] task %d: workflow=%s limit=%d", taskID, workflow, iterLimit)

	e.addActivity(taskID, "executor.classify", map[string]interface{}{
		"workflow":        workflow,
		"iteration_limit": iterLimit,
	})

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
		e.addActivity(taskID, "executor.worktree", map[string]interface{}{
			"dir":    wt.Dir,
			"branch": wt.Branch,
		})
	} else {
		workDir = "."
	}

	if ctx.Err() != nil {
		return
	}

	// Build tools and agent.
	tools := NewToolSet(workDir, e.store)
	agent := &AgentLoop{
		sender:  e.client,
		tools:   tools,
		taskID:  taskID,
		maxIter: iterLimit,
		onActivity: func(eventType string, data map[string]interface{}) {
			e.addActivity(taskID, eventType, data)
		},
	}

	// Build system prompt.
	sysPrompt := buildSystemPrompt(task, workflow)

	// Run agent loop.
	e.addActivity(taskID, "executor.agent_start", nil)
	result, err := agent.Run(ctx, sysPrompt, buildUserPrompt(task))
	if err != nil {
		if ctx.Err() != nil {
			return // Cancelled — Stop() handles the stage update.
		}
		log.Printf("[executor] task %d: agent error: %v", taskID, err)
		e.markBlocked(taskID, fmt.Sprintf("agent error: %v", err))
		return
	}

	e.addActivity(taskID, "executor.agent_done", map[string]interface{}{
		"iterations":    result.Iterations,
		"hit_limit":     result.HitLimit,
		"input_tokens":  result.TotalInputTokens,
		"output_tokens": result.TotalOutputTokens,
		"tool_calls":    len(result.ToolCalls),
	})

	// Run L1 verification.
	if workDir != "." {
		vr := VerifyL1(ctx, workDir)
		e.addActivity(taskID, "executor.verify_l1", map[string]interface{}{
			"passed": vr.Passed,
			"errors": vr.Errors,
		})
		if !vr.Passed {
			e.markBlocked(taskID, fmt.Sprintf("L1 verification failed: %s", strings.Join(vr.Errors, "; ")))
			return
		}
	}

	// Commit changes if any.
	if wt != nil && wt.HasChanges() {
		hash, err := wt.Commit(fmt.Sprintf("feat(task-%d): %s", taskID, task.Title))
		if err != nil {
			e.markBlocked(taskID, fmt.Sprintf("commit failed: %v", err))
			return
		}
		e.addActivity(taskID, "executor.commit", map[string]interface{}{
			"hash": hash,
		})
	}

	// Move to validation.
	e.store.Update(taskID, map[string]interface{}{"stage": "validation"})
	e.addActivity(taskID, "executor.complete", map[string]interface{}{
		"workflow":   workflow,
		"iterations": result.Iterations,
	})
}

func (e *Executor) markBlocked(taskID int64, reason string) {
	e.store.Update(taskID, map[string]interface{}{"stage": "blocked"})
	e.addActivity(taskID, "task.blocked", map[string]interface{}{"reason": reason})
}

func (e *Executor) addActivity(taskID int64, eventType string, data map[string]interface{}) {
	e.store.AddActivity(taskID, eventType, data)
}

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
		b.WriteString("\nThis is a straightforward change. Implement, then use task_update to move to validation.\n")
	default:
		b.WriteString("\nThis is a complex task. Plan your approach, implement step by step, and verify.\n")
	}

	b.WriteString("\n## Available Tools\n")
	b.WriteString("- file_read: Read files\n- file_write: Write/create files\n- bash: Run commands\n")
	b.WriteString("- list_files: List directory contents\n- task_update: Update task stage/notes\n")

	return b.String()
}

func buildUserPrompt(task *store.Task) string {
	if task.Description != "" {
		return fmt.Sprintf("Implement: %s\n\n%s", task.Title, task.Description)
	}
	return fmt.Sprintf("Implement: %s", task.Title)
}
```

- [ ] **Step 3: Update cmd/tasks/main.go to pass Claude client**

```go
// In runServe(), add Claude client creation:
// Import auth and stream packages.
// Read credentials, create token source, create stream client.
// Pass to executor config.

import (
	"github.com/rishav1305/soul-v2/internal/tasks/executor"
	"github.com/rishav1305/soul-v2/internal/chat/stream"
	"github.com/rishav1305/soul-v2/pkg/auth"
)

// In runServe():
tokenSource := auth.NewOAuthTokenSource()
claudeClient := stream.NewClient(tokenSource)

repoDir := os.Getenv("SOUL_V2_REPO_DIR")
if repoDir == "" {
	// Default to current working directory.
	repoDir, _ = os.Getwd()
}

exec := executor.New(executor.Config{
	Store:       taskStore,
	MaxParallel: 3,
	RepoDir:     repoDir,
	Client:      claudeClient,
})
```

- [ ] **Step 4: Run full test suite**

Run: `cd /home/rishav/soul-v2 && go test ./... -v`
Expected: all PASS

- [ ] **Step 5: Build both binaries**

Run: `cd /home/rishav/soul-v2 && make build`
Expected: `soul-chat` and `soul-tasks` build successfully

- [ ] **Step 6: Commit**

```bash
git add internal/tasks/executor/executor.go cmd/tasks/main.go
git commit -m "feat: wire agent loop into executor with full pipeline"
```

---

### Task 9: Update CLAUDE.md and Full Verification

**Files:**
- Modify: `CLAUDE.md` — add executor documentation
- Modify: `.gitignore` — add `.worktrees/`

- [ ] **Step 1: Update CLAUDE.md**

Add to Architecture section:
```
internal/tasks/
  executor/                   Autonomous execution engine
    executor.go               Lifecycle management — start/stop/track
    agent.go                  Tool-calling agent loop with Claude API
    tools.go                  Agent tool definitions (file_read/write, bash, etc.)
    classify.go               Workflow classifier (micro/quick/full)
    worktree.go               Git worktree isolation for task execution
    verify.go                 Per-step verification gates (L1: go vet + tsc)
```

Add to Environment Variables:
```
| `SOUL_V2_REPO_DIR` | `(cwd)` | Project root for worktree creation |
```

- [ ] **Step 2: Add .worktrees/ to .gitignore**

- [ ] **Step 3: Run full verification**

```bash
cd /home/rishav/soul-v2 && go vet ./... && go test ./... && make build
```

- [ ] **Step 4: Commit**

```bash
git add CLAUDE.md .gitignore
git commit -m "docs: update CLAUDE.md with executor architecture"
```

---

## Key Design Decisions

1. **Non-streaming agent loop**: Uses `stream.Client.Send()` (synchronous) instead of `Stream()` (SSE). Simpler tool-call handling — get complete response, extract tool_use blocks, execute, continue. No need to accumulate partial JSON from SSE deltas in the autonomous context.

2. **Sender interface**: The agent loop depends on `Sender` interface, not `*stream.Client` directly. This enables clean testing with `mockSender` and future model routing (different Sender implementations for Haiku/Sonnet/Opus).

3. **Path-scoped tools**: All file tools are scoped to the worktree directory with path traversal prevention. The agent cannot read/write outside its worktree.

4. **Executor-level lifecycle**: Start/Stop are on the Executor, not individual tasks. The Executor tracks running tasks, enforces concurrency limits, and handles cleanup.

5. **Deferred git merge**: The executor commits to the task branch in the worktree but does NOT merge to main. Merging is a separate manual step for now (or a later plan).

6. **Activity-first observability**: Every significant event is logged via `store.AddActivity()`. The frontend can display a live timeline of what the agent is doing.
