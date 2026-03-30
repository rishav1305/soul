package hooks

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestHookRunner_NoConfig(t *testing.T) {
	hr := NewHookRunner("/nonexistent/hooks.json")

	blocked, message, output := hr.RunToolHook("before", "code_edit", map[string]string{
		"file": "main.go",
	})

	if blocked {
		t.Error("expected not blocked with no config")
	}
	if message != "" {
		t.Errorf("expected empty message, got %q", message)
	}
	if output != "" {
		t.Errorf("expected empty output, got %q", output)
	}
}

func TestHookRunner_BlockingHook(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "hooks.json")

	cfg := HookConfig{
		Hooks: []ToolHook{
			{
				Event:   "before:code_edit",
				Match:   "*.go",
				Action:  "block",
				Message: "Go files are read-only",
			},
		},
	}
	data, err := json.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(configPath, data, 0644); err != nil {
		t.Fatal(err)
	}

	hr := NewHookRunner(configPath)

	blocked, message, _ := hr.RunToolHook("before", "code_edit", map[string]string{
		"file": "server.go",
	})

	if !blocked {
		t.Error("expected blocked for .go file")
	}
	if message != "Go files are read-only" {
		t.Errorf("expected blocking message, got %q", message)
	}

	// Non-matching file should not be blocked.
	blocked, _, _ = hr.RunToolHook("before", "code_edit", map[string]string{
		"file": "README.md",
	})
	if blocked {
		t.Error("expected not blocked for .md file")
	}
}

func TestHookRunner_CommandHook(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "hooks.json")

	cfg := HookConfig{
		Hooks: []ToolHook{
			{
				Event:   "after:code_exec",
				Command: "echo hook-ran",
				Timeout: 5,
			},
		},
	}
	data, err := json.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(configPath, data, 0644); err != nil {
		t.Fatal(err)
	}

	hr := NewHookRunner(configPath)

	blocked, _, output := hr.RunToolHook("after", "code_exec", map[string]string{})

	if blocked {
		t.Error("expected not blocked for command hook")
	}
	if output != "hook-ran" {
		t.Errorf("expected %q, got %q", "hook-ran", output)
	}
}

func TestRunWorkflowHook_MatchingEvent(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "hooks.json")
	outFile := filepath.Join(dir, "wf_out.txt")

	cfg := HookConfig{
		WorkflowHooks: []WorkflowHook{
			{
				Event:   "after:task_done",
				Command: "echo workflow-ran > " + outFile,
				Timeout: 5,
			},
		},
	}
	data, _ := json.Marshal(cfg)
	os.WriteFile(configPath, data, 0644)

	hr := NewHookRunner(configPath)
	hr.RunWorkflowHook("after:task_done", nil)

	got, err := os.ReadFile(outFile)
	if err != nil {
		t.Fatalf("expected output file: %v", err)
	}
	if s := string(got); s != "workflow-ran\n" {
		t.Errorf("output = %q, want %q", s, "workflow-ran\n")
	}
}

func TestRunWorkflowHook_NonMatchingEvent(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "hooks.json")
	outFile := filepath.Join(dir, "wf_out.txt")

	cfg := HookConfig{
		WorkflowHooks: []WorkflowHook{
			{
				Event:   "after:merge_to_dev",
				Command: "echo should-not-run > " + outFile,
				Timeout: 5,
			},
		},
	}
	data, _ := json.Marshal(cfg)
	os.WriteFile(configPath, data, 0644)

	hr := NewHookRunner(configPath)
	hr.RunWorkflowHook("after:task_done", nil)

	if _, err := os.Stat(outFile); err == nil {
		t.Error("expected no output file for non-matching event")
	}
}

func TestRunWorkflowHook_VarExpansion(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "hooks.json")
	outFile := filepath.Join(dir, "wf_vars.txt")

	cfg := HookConfig{
		WorkflowHooks: []WorkflowHook{
			{
				Event:   "after:task_done",
				Command: "echo {task_id} > " + outFile,
				Timeout: 5,
			},
		},
	}
	data, _ := json.Marshal(cfg)
	os.WriteFile(configPath, data, 0644)

	hr := NewHookRunner(configPath)
	hr.RunWorkflowHook("after:task_done", map[string]string{"task_id": "99"})

	got, err := os.ReadFile(outFile)
	if err != nil {
		t.Fatalf("expected output file: %v", err)
	}
	if s := string(got); s != "99\n" {
		t.Errorf("output = %q, want %q", s, "99\n")
	}
}

func TestRunWorkflowHook_DefaultTimeout(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "hooks.json")
	outFile := filepath.Join(dir, "wf_default.txt")

	cfg := HookConfig{
		WorkflowHooks: []WorkflowHook{
			{
				Event:   "after:deploy",
				Command: "echo default-timeout > " + outFile,
				// Timeout omitted — should default to 30
			},
		},
	}
	data, _ := json.Marshal(cfg)
	os.WriteFile(configPath, data, 0644)

	hr := NewHookRunner(configPath)
	hr.RunWorkflowHook("after:deploy", nil)

	got, err := os.ReadFile(outFile)
	if err != nil {
		t.Fatalf("expected output file: %v", err)
	}
	if s := string(got); s != "default-timeout\n" {
		t.Errorf("output = %q, want %q", s, "default-timeout\n")
	}
}

func TestRunWorkflowHook_MultipleHooks(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "hooks.json")
	outFile := filepath.Join(dir, "wf_multi.txt")

	cfg := HookConfig{
		WorkflowHooks: []WorkflowHook{
			{
				Event:   "after:build",
				Command: "echo first >> " + outFile,
				Timeout: 5,
			},
			{
				Event:   "after:build",
				Command: "echo second >> " + outFile,
				Timeout: 5,
			},
			{
				Event:   "after:test",
				Command: "echo skip >> " + outFile,
				Timeout: 5,
			},
		},
	}
	data, _ := json.Marshal(cfg)
	os.WriteFile(configPath, data, 0644)

	hr := NewHookRunner(configPath)
	hr.RunWorkflowHook("after:build", nil)

	got, err := os.ReadFile(outFile)
	if err != nil {
		t.Fatalf("expected output file: %v", err)
	}
	if s := string(got); s != "first\nsecond\n" {
		t.Errorf("output = %q, want %q", s, "first\nsecond\n")
	}
}

func TestNewHookRunner_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "hooks.json")
	os.WriteFile(configPath, []byte("not-json{"), 0644)

	hr := NewHookRunner(configPath)
	// Should get empty config, not panic.
	if len(hr.config.Hooks) != 0 {
		t.Errorf("expected empty hooks for invalid JSON, got %d", len(hr.config.Hooks))
	}
	if len(hr.config.WorkflowHooks) != 0 {
		t.Errorf("expected empty workflow hooks, got %d", len(hr.config.WorkflowHooks))
	}
}

func TestRunToolHook_MatchNoFileVar(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "hooks.json")

	cfg := HookConfig{
		Hooks: []ToolHook{
			{
				Event:   "before:code_edit",
				Match:   "*.go",
				Action:  "block",
				Message: "blocked",
			},
		},
	}
	data, _ := json.Marshal(cfg)
	os.WriteFile(configPath, data, 0644)

	hr := NewHookRunner(configPath)
	// No "file" in vars — match should be skipped.
	blocked, _, _ := hr.RunToolHook("before", "code_edit", map[string]string{})
	if blocked {
		t.Error("expected not blocked when file var is missing")
	}
}

func TestRunToolHook_CommandDefaultTimeout(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "hooks.json")

	cfg := HookConfig{
		Hooks: []ToolHook{
			{
				Event:   "after:code_exec",
				Command: "echo default-to",
				// Timeout 0 — should default to 10
			},
		},
	}
	data, _ := json.Marshal(cfg)
	os.WriteFile(configPath, data, 0644)

	hr := NewHookRunner(configPath)
	_, _, output := hr.RunToolHook("after", "code_exec", map[string]string{})
	if output != "default-to" {
		t.Errorf("output = %q, want %q", output, "default-to")
	}
}

func TestExpandVars(t *testing.T) {
	tests := []struct {
		template string
		vars     map[string]string
		want     string
	}{
		{
			template: "echo {file}",
			vars:     map[string]string{"file": "main.go"},
			want:     "echo main.go",
		},
		{
			template: "task {task_id} file {file}",
			vars:     map[string]string{"task_id": "42", "file": "server.go"},
			want:     "task 42 file server.go",
		},
		{
			template: "no vars here",
			vars:     map[string]string{},
			want:     "no vars here",
		},
		{
			template: "{unknown} stays",
			vars:     map[string]string{"other": "val"},
			want:     "{unknown} stays",
		},
	}

	for _, tt := range tests {
		got := expandVars(tt.template, tt.vars)
		if got != tt.want {
			t.Errorf("expandVars(%q, %v) = %q, want %q", tt.template, tt.vars, got, tt.want)
		}
	}
}
