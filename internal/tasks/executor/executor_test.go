package executor

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

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
