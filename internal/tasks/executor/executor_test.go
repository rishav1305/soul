package executor

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rishav1305/soul/internal/chat/stream"
	"github.com/rishav1305/soul/internal/tasks/store"
)

func openTestStore(t *testing.T) *store.Store {
	t.Helper()
	dir := t.TempDir()
	s, err := store.Open(filepath.Join(dir, "tasks.db"))
	if err != nil {
		t.Fatalf("openTestStore: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestExecutorStartStop(t *testing.T) {
	s := openTestStore(t)
	e := New(Config{Store: s})

	task, err := s.Create("fix typo in readme", "", "")
	if err != nil {
		t.Fatalf("create task: %v", err)
	}

	if err := e.Start(context.Background(), task.ID); err != nil {
		t.Fatalf("Start: %v", err)
	}

	if !e.IsRunning(task.ID) {
		t.Fatal("expected task to be running after Start")
	}

	if err := e.Stop(task.ID); err != nil {
		t.Fatalf("Stop: %v", err)
	}

	// Give the goroutine time to exit.
	time.Sleep(100 * time.Millisecond)

	if e.IsRunning(task.ID) {
		t.Fatal("expected task to not be running after Stop")
	}
}

func TestExecutorRejectsOverLimit(t *testing.T) {
	s := openTestStore(t)
	e := New(Config{Store: s, MaxParallel: 1})

	task1, err := s.Create("fix typo one", "", "")
	if err != nil {
		t.Fatalf("create task1: %v", err)
	}
	task2, err := s.Create("fix typo two", "", "")
	if err != nil {
		t.Fatalf("create task2: %v", err)
	}

	if err := e.Start(context.Background(), task1.ID); err != nil {
		t.Fatalf("Start task1: %v", err)
	}

	err = e.Start(context.Background(), task2.ID)
	if err == nil {
		t.Fatal("expected error when starting beyond MaxParallel, got nil")
	}
}

func TestExecutorRejectsDuplicateStart(t *testing.T) {
	s := openTestStore(t)
	e := New(Config{Store: s})

	task, err := s.Create("fix typo dup", "", "")
	if err != nil {
		t.Fatalf("create task: %v", err)
	}

	if err := e.Start(context.Background(), task.ID); err != nil {
		t.Fatalf("first Start: %v", err)
	}

	err = e.Start(context.Background(), task.ID)
	if err == nil {
		t.Fatal("expected error on duplicate Start, got nil")
	}
}

func TestExecutorRunningCount(t *testing.T) {
	s := openTestStore(t)
	e := New(Config{Store: s})

	if got := e.RunningCount(); got != 0 {
		t.Fatalf("expected RunningCount 0, got %d", got)
	}

	task, err := s.Create("fix typo count", "", "")
	if err != nil {
		t.Fatalf("create task: %v", err)
	}

	if err := e.Start(context.Background(), task.ID); err != nil {
		t.Fatalf("Start: %v", err)
	}

	if got := e.RunningCount(); got != 1 {
		t.Fatalf("expected RunningCount 1, got %d", got)
	}
}

func TestBuildSystemPrompt_Micro(t *testing.T) {
	task := &store.Task{Title: "Fix typo", Description: "Correct spelling in README"}
	prompt := buildSystemPrompt(task, "micro")
	if !strings.Contains(prompt, "Fix typo") {
		t.Error("expected task title in prompt")
	}
	if !strings.Contains(prompt, "Correct spelling in README") {
		t.Error("expected description in prompt")
	}
	if !strings.Contains(prompt, "trivial change") {
		t.Error("expected micro workflow guidance")
	}
	if !strings.Contains(prompt, "file_read") {
		t.Error("expected tool listing")
	}
}

func TestBuildSystemPrompt_Quick(t *testing.T) {
	task := &store.Task{Title: "Add endpoint"}
	prompt := buildSystemPrompt(task, "quick")
	if !strings.Contains(prompt, "straightforward change") {
		t.Error("expected quick workflow guidance")
	}
}

func TestBuildSystemPrompt_Complex(t *testing.T) {
	task := &store.Task{Title: "Refactor auth", Description: "Major overhaul"}
	prompt := buildSystemPrompt(task, "full")
	if !strings.Contains(prompt, "complex task") {
		t.Error("expected complex workflow guidance")
	}
}

func TestBuildSystemPrompt_NoDescription(t *testing.T) {
	task := &store.Task{Title: "Quick fix"}
	prompt := buildSystemPrompt(task, "micro")
	if strings.Contains(prompt, "Description:") {
		t.Error("expected no Description line when description is empty")
	}
}

func TestBuildUserPrompt_WithDescription(t *testing.T) {
	task := &store.Task{Title: "Add tests", Description: "Cover the auth module"}
	prompt := buildUserPrompt(task)
	if !strings.Contains(prompt, "Add tests") {
		t.Error("expected title in user prompt")
	}
	if !strings.Contains(prompt, "Cover the auth module") {
		t.Error("expected description in user prompt")
	}
}

func TestBuildUserPrompt_NoDescription(t *testing.T) {
	task := &store.Task{Title: "Fix bug"}
	prompt := buildUserPrompt(task)
	if prompt != "Implement: Fix bug" {
		t.Errorf("prompt = %q, want 'Implement: Fix bug'", prompt)
	}
}

func TestMarkBlocked(t *testing.T) {
	s := openTestStore(t)
	e := New(Config{Store: s})

	task, err := s.Create("block me", "", "")
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	e.markBlocked(task.ID, "dependency missing")

	got, err := s.Get(task.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Stage != "blocked" {
		t.Errorf("stage = %q, want blocked", got.Stage)
	}
}

// --- Stage rejection tests ---

func TestExecutor_Start_RejectsBrainstormStage(t *testing.T) {
	s := openTestStore(t)
	e := New(Config{Store: s})

	task, _ := s.Create("brainstorm task", "", "")
	s.Update(task.ID, map[string]interface{}{"stage": "brainstorm"})

	err := e.Start(context.Background(), task.ID)
	if err == nil {
		t.Fatal("expected error for brainstorm stage")
	}
	if !strings.Contains(err.Error(), "brainstorm") {
		t.Errorf("error = %q, should mention brainstorm", err.Error())
	}
}

func TestExecutor_Start_RejectsActiveStage(t *testing.T) {
	s := openTestStore(t)
	e := New(Config{Store: s})

	task, _ := s.Create("active task", "", "")
	s.Update(task.ID, map[string]interface{}{"stage": "active"})

	err := e.Start(context.Background(), task.ID)
	if err == nil {
		t.Fatal("expected error for active stage")
	}
}

func TestExecutor_Start_RejectsValidationStage(t *testing.T) {
	s := openTestStore(t)
	e := New(Config{Store: s})

	task, _ := s.Create("validation task", "", "")
	s.Update(task.ID, map[string]interface{}{"stage": "validation"})

	err := e.Start(context.Background(), task.ID)
	if err == nil {
		t.Fatal("expected error for validation stage")
	}
}

func TestExecutor_Start_NonexistentTask(t *testing.T) {
	s := openTestStore(t)
	e := New(Config{Store: s})

	err := e.Start(context.Background(), 99999)
	if err == nil {
		t.Fatal("expected error for nonexistent task")
	}
}

func TestExecutor_Start_AllowsBlockedStage(t *testing.T) {
	s := openTestStore(t)
	e := New(Config{Store: s})

	task, _ := s.Create("blocked task", "", "")
	s.Update(task.ID, map[string]interface{}{"stage": "blocked"})

	err := e.Start(context.Background(), task.ID)
	if err != nil {
		t.Fatalf("Start from blocked should succeed: %v", err)
	}
	// Give goroutine time to finish.
	time.Sleep(100 * time.Millisecond)
}

func TestExecutor_Stop_NotRunning(t *testing.T) {
	s := openTestStore(t)
	e := New(Config{Store: s})

	err := e.Stop(999)
	if err == nil {
		t.Fatal("expected error for non-running task")
	}
	if !strings.Contains(err.Error(), "not running") {
		t.Errorf("error = %q, should mention not running", err.Error())
	}
}

// --- Constructor tests ---

func TestExecutor_New_DefaultMaxParallel(t *testing.T) {
	s := openTestStore(t)
	e := New(Config{Store: s})

	if e.maxParallel != 3 {
		t.Errorf("default maxParallel = %d, want 3", e.maxParallel)
	}
}

func TestExecutor_New_CustomMaxParallel(t *testing.T) {
	s := openTestStore(t)
	e := New(Config{Store: s, MaxParallel: 5})

	if e.maxParallel != 5 {
		t.Errorf("maxParallel = %d, want 5", e.maxParallel)
	}
}

func TestExecutor_New_ZeroMaxParallel(t *testing.T) {
	s := openTestStore(t)
	e := New(Config{Store: s, MaxParallel: 0})

	if e.maxParallel != 3 {
		t.Errorf("zero MaxParallel should default to 3, got %d", e.maxParallel)
	}
}

// --- run() direct tests ---

func TestExecutor_Run_WithClient(t *testing.T) {
	s := openTestStore(t)

	sender := &mockSender{
		responses: []*stream.Response{
			newEndTurnResponse("task complete", 100, 50),
		},
	}

	e := New(Config{Store: s, Client: sender})

	task, _ := s.Create("client test task", "desc", "")
	s.Update(task.ID, map[string]interface{}{"stage": "active"})

	e.run(context.Background(), task.ID)

	got, _ := s.Get(task.ID)
	if got.Stage != "validation" {
		t.Errorf("stage = %q, want validation", got.Stage)
	}
}

func TestExecutor_Run_ContextCancelledBeforeWorktree(t *testing.T) {
	s := openTestStore(t)
	e := New(Config{Store: s})

	task, _ := s.Create("cancel test", "", "")
	s.Update(task.ID, map[string]interface{}{"stage": "active"})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	e.run(ctx, task.ID)

	got, _ := s.Get(task.ID)
	if got.Stage == "validation" {
		t.Error("task should not reach validation when context is cancelled")
	}
}

func TestExecutor_Run_AgentHitLimit(t *testing.T) {
	s := openTestStore(t)

	// All responses are tool_use — forces hit limit.
	responses := make([]*stream.Response, 50)
	for i := range responses {
		responses[i] = newToolUseResponse("t1", "echo loop", 10, 5)
	}

	sender := &mockSender{responses: responses}
	e := New(Config{Store: s, Client: sender})

	task, _ := s.Create("fix typo limit test", "", "")
	s.Update(task.ID, map[string]interface{}{"stage": "active"})

	e.run(context.Background(), task.ID)

	got, _ := s.Get(task.ID)
	if got.Stage != "blocked" {
		t.Errorf("stage = %q, want blocked (hit limit)", got.Stage)
	}
}

func TestExecutor_Run_AgentError(t *testing.T) {
	s := openTestStore(t)

	sender := &failSender{}
	e := New(Config{Store: s, Client: sender})

	task, _ := s.Create("error task", "", "")
	s.Update(task.ID, map[string]interface{}{"stage": "active"})

	e.run(context.Background(), task.ID)

	got, _ := s.Get(task.ID)
	if got.Stage != "blocked" {
		t.Errorf("stage = %q, want blocked (agent error)", got.Stage)
	}
}

func TestExecutor_Run_WithRepoDir(t *testing.T) {
	s := openTestStore(t)
	repo := setupTestRepo(t)

	sender := &mockSender{
		responses: []*stream.Response{
			newEndTurnResponse("done", 100, 50),
		},
	}

	e := New(Config{Store: s, Client: sender, RepoDir: repo})

	task, _ := s.Create("repo test", "with worktree", "")
	s.Update(task.ID, map[string]interface{}{"stage": "active"})

	e.run(context.Background(), task.ID)

	got, _ := s.Get(task.ID)
	// Agent returned end_turn with no file writes → no changes to commit
	// L1: no go.mod/tsconfig in worktree → passes → moves to validation
	if got.Stage != "validation" {
		t.Errorf("stage = %q, want validation", got.Stage)
	}
}

func TestExecutor_Run_WithHooksConfig(t *testing.T) {
	s := openTestStore(t)

	sender := &mockSender{
		responses: []*stream.Response{
			newEndTurnResponse("done", 10, 5),
		},
	}

	hooksDir := t.TempDir()
	hooksPath := filepath.Join(hooksDir, "hooks.json")
	os.WriteFile(hooksPath, []byte(`{"hooks":[]}`), 0644)

	e := New(Config{Store: s, Client: sender, HooksConfigPath: hooksPath})

	task, _ := s.Create("hooks test", "", "")
	s.Update(task.ID, map[string]interface{}{"stage": "active"})

	e.run(context.Background(), task.ID)

	got, _ := s.Get(task.ID)
	if got.Stage != "validation" {
		t.Errorf("stage = %q, want validation", got.Stage)
	}
}

func TestExecutor_Run_GetTaskFails(t *testing.T) {
	dir := t.TempDir()
	s, _ := store.Open(filepath.Join(dir, "fail.db"))

	task, _ := s.Create("fail task", "", "")
	s.Update(task.ID, map[string]interface{}{"stage": "active"})

	e := New(Config{Store: s})

	s.Close() // Close store to force Get to fail.

	// Should not panic even with closed store.
	e.run(context.Background(), task.ID)
}
