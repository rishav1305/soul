package team

import (
	"context"
	"testing"
)

func TestNewTaskBridge(t *testing.T) {
	tb := NewTaskBridge("/some/path/backlog_cli.py")
	if tb.cliPath != "/some/path/backlog_cli.py" {
		t.Errorf("expected cli path, got %s", tb.cliPath)
	}
}

func TestTaskBridge_CreateTask_Validation(t *testing.T) {
	tb := NewTaskBridge("/nonexistent")

	// Missing title.
	err := tb.CreateTask(context.Background(), CreateTaskRequest{Project: "test"})
	if err == nil {
		t.Error("expected error for missing title")
	}

	// Missing project.
	err = tb.CreateTask(context.Background(), CreateTaskRequest{Title: "Test"})
	if err == nil {
		t.Error("expected error for missing project")
	}
}

func TestTaskBridge_UpdateTask_Validation(t *testing.T) {
	tb := NewTaskBridge("/nonexistent")

	err := tb.UpdateTask(context.Background(), "", UpdateTaskRequest{Status: "done"})
	if err == nil {
		t.Error("expected error for empty task file")
	}
}
