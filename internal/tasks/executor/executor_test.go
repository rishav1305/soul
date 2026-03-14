package executor

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/rishav1305/soul-v2/internal/tasks/store"
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
