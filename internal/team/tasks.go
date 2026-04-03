package team

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"time"
)

// TaskBridge wraps the PA backlog CLI for task CRUD operations.
type TaskBridge struct {
	cliPath string
}

// NewTaskBridge creates a TaskBridge pointing to the backlog CLI.
func NewTaskBridge(cliPath string) *TaskBridge {
	return &TaskBridge{cliPath: cliPath}
}

// Dashboard returns the full task dashboard from the backlog CLI.
func (tb *TaskBridge) Dashboard(ctx context.Context) (*TaskDashboard, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "python3", tb.cliPath, "dashboard")
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("backlog dashboard: %w", err)
	}

	var dashboard TaskDashboard
	if err := json.Unmarshal(out, &dashboard); err != nil {
		return nil, fmt.Errorf("parse dashboard: %w", err)
	}

	return &dashboard, nil
}

// ListTasks returns tasks filtered by optional project and status.
func (tb *TaskBridge) ListTasks(ctx context.Context, project, status string) ([]TaskItem, error) {
	dashboard, err := tb.Dashboard(ctx)
	if err != nil {
		return nil, err
	}

	var tasks []TaskItem
	for _, p := range dashboard.Projects {
		if project != "" && p.Project != project {
			continue
		}
		for _, t := range p.TopTasks {
			if status != "" && t.Status != status {
				continue
			}
			t.Project = p.Project
			tasks = append(tasks, t)
		}
	}

	return tasks, nil
}

// CreateTask creates a new task via the backlog CLI.
func (tb *TaskBridge) CreateTask(ctx context.Context, req CreateTaskRequest) error {
	if req.Title == "" {
		return fmt.Errorf("task title is required")
	}
	if req.Project == "" {
		return fmt.Errorf("task project is required")
	}

	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	args := []string{tb.cliPath, "create", "--project", req.Project, "--title", req.Title}
	if req.Priority != "" {
		args = append(args, "--priority", req.Priority)
	}
	if req.DueDate != "" {
		args = append(args, "--due", req.DueDate)
	}
	if req.Description != "" {
		args = append(args, "--description", req.Description)
	}

	cmd := exec.CommandContext(ctx, "python3", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("create task: %s: %w", string(out), err)
	}

	return nil
}

// UpdateTask updates an existing task's status via the backlog CLI.
func (tb *TaskBridge) UpdateTask(ctx context.Context, taskFile string, req UpdateTaskRequest) error {
	if taskFile == "" {
		return fmt.Errorf("task file/id is required")
	}

	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	args := []string{tb.cliPath, "update", "--file", taskFile}
	if req.Status != "" {
		args = append(args, "--status", req.Status)
	}
	if req.Priority != "" {
		args = append(args, "--priority", req.Priority)
	}

	cmd := exec.CommandContext(ctx, "python3", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("update task: %s: %w", string(out), err)
	}

	return nil
}
