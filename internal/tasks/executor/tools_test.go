package executor

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func newTestToolSet(t *testing.T) *ToolSet {
	t.Helper()
	dir := t.TempDir()
	return NewToolSet(dir, nil)
}

func mustMarshal(t *testing.T, v interface{}) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return string(b)
}

func TestToolFileRead(t *testing.T) {
	ts := newTestToolSet(t)
	content := "hello world\nline two\n"
	if err := os.WriteFile(filepath.Join(ts.rootDir, "test.txt"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := ts.Execute("file_read", mustMarshal(t, map[string]string{"path": "test.txt"}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != content {
		t.Errorf("expected %q, got %q", content, got)
	}
}

func TestToolFileReadPathTraversal(t *testing.T) {
	ts := newTestToolSet(t)

	_, err := ts.Execute("file_read", mustMarshal(t, map[string]string{"path": "../../etc/passwd"}))
	if err == nil {
		t.Fatal("expected error for path traversal, got nil")
	}
	if !strings.Contains(err.Error(), "path traversal") {
		t.Errorf("expected 'path traversal' in error, got: %v", err)
	}
}

func TestToolFileWrite(t *testing.T) {
	ts := newTestToolSet(t)
	content := "package main\n\nfunc main() {}\n"

	got, err := ts.Execute("file_write", mustMarshal(t, map[string]string{
		"path":    "main.go",
		"content": content,
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(got, "wrote") {
		t.Errorf("expected 'wrote' in output, got: %q", got)
	}

	data, err := os.ReadFile(filepath.Join(ts.rootDir, "main.go"))
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	if string(data) != content {
		t.Errorf("expected %q, got %q", content, string(data))
	}
}

func TestToolFileWriteCreatesDirectories(t *testing.T) {
	ts := newTestToolSet(t)
	content := "package mypackage\n"

	_, err := ts.Execute("file_write", mustMarshal(t, map[string]string{
		"path":    "sub/dir/file.go",
		"content": content,
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(ts.rootDir, "sub", "dir", "file.go"))
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	if string(data) != content {
		t.Errorf("expected %q, got %q", content, string(data))
	}
}

func TestToolBash(t *testing.T) {
	ts := newTestToolSet(t)

	got, err := ts.Execute("bash", mustMarshal(t, map[string]string{"command": "echo hello"}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(got, "hello") {
		t.Errorf("expected 'hello' in output, got: %q", got)
	}
}

func TestToolBashTimeout(t *testing.T) {
	ts := newTestToolSet(t)

	// Use a very short sleep that is well within timeout to just verify no hang.
	got, err := ts.Execute("bash", mustMarshal(t, map[string]string{"command": "sleep 0.1 && echo done"}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(got, "done") {
		t.Errorf("expected 'done' in output, got: %q", got)
	}
}

func TestToolBashFailureReturnsOutput(t *testing.T) {
	ts := newTestToolSet(t)

	// A failing command should return output + exit status, not a Go error.
	got, err := ts.Execute("bash", mustMarshal(t, map[string]string{"command": "echo oops && exit 1"}))
	if err != nil {
		t.Fatalf("bash failure should not return Go error, got: %v", err)
	}
	if !strings.Contains(got, "oops") {
		t.Errorf("expected output to contain 'oops', got: %q", got)
	}
	if !strings.Contains(got, "exit status") {
		t.Errorf("expected output to contain 'exit status', got: %q", got)
	}
}

func TestToolUnknown(t *testing.T) {
	ts := newTestToolSet(t)

	_, err := ts.Execute("nonexistent_tool", "{}")
	if err == nil {
		t.Fatal("expected error for unknown tool, got nil")
	}
	if !strings.Contains(err.Error(), "unknown tool") {
		t.Errorf("expected 'unknown tool' in error, got: %v", err)
	}
}

func TestToolDefinitions(t *testing.T) {
	ts := newTestToolSet(t)
	defs := ts.Definitions()

	if len(defs) == 0 {
		t.Fatal("expected non-empty tool definitions")
	}

	for i, d := range defs {
		if d.Name == "" {
			t.Errorf("definition %d: empty name", i)
		}
		if d.Description == "" {
			t.Errorf("definition %d (%s): empty description", i, d.Name)
		}
		if len(d.InputSchema) == 0 {
			t.Errorf("definition %d (%s): empty input_schema", i, d.Name)
		}
		// Verify schema is valid JSON.
		var v interface{}
		if err := json.Unmarshal(d.InputSchema, &v); err != nil {
			t.Errorf("definition %d (%s): invalid JSON schema: %v", i, d.Name, err)
		}
	}

	// Verify expected tools are present.
	names := make(map[string]bool)
	for _, d := range defs {
		names[d.Name] = true
	}
	expected := []string{"file_read", "file_write", "bash", "list_files", "task_update"}
	for _, name := range expected {
		if !names[name] {
			t.Errorf("expected tool %q in definitions", name)
		}
	}
}
