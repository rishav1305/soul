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
