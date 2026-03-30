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

func TestToolListFiles(t *testing.T) {
	ts := newTestToolSet(t)
	// Create some files in the root dir.
	os.WriteFile(filepath.Join(ts.rootDir, "a.go"), []byte("package a"), 0644)
	os.WriteFile(filepath.Join(ts.rootDir, "b.go"), []byte("package b"), 0644)

	got, err := ts.Execute("list_files", mustMarshal(t, map[string]string{"path": "."}))
	if err != nil {
		t.Fatalf("list_files: %v", err)
	}
	if !strings.Contains(got, "a.go") {
		t.Errorf("expected a.go in output, got: %q", got)
	}
	if !strings.Contains(got, "b.go") {
		t.Errorf("expected b.go in output, got: %q", got)
	}
}

func TestToolListFiles_Recursive(t *testing.T) {
	ts := newTestToolSet(t)
	subDir := filepath.Join(ts.rootDir, "sub")
	os.MkdirAll(subDir, 0755)
	os.WriteFile(filepath.Join(subDir, "nested.go"), []byte("package sub"), 0644)

	got, err := ts.Execute("list_files", mustMarshal(t, map[string]interface{}{
		"path":      ".",
		"recursive": true,
	}))
	if err != nil {
		t.Fatalf("list_files recursive: %v", err)
	}
	if !strings.Contains(got, "nested.go") {
		t.Errorf("expected nested.go in recursive listing, got: %q", got)
	}
}

func TestToolListFiles_Empty(t *testing.T) {
	ts := newTestToolSet(t)
	emptyDir := filepath.Join(ts.rootDir, "empty")
	os.MkdirAll(emptyDir, 0755)

	got, err := ts.Execute("list_files", mustMarshal(t, map[string]string{"path": "empty"}))
	if err != nil {
		t.Fatalf("list_files empty: %v", err)
	}
	if got != "(empty directory)" {
		t.Errorf("expected '(empty directory)', got: %q", got)
	}
}

func TestToolListFiles_EmptyInput(t *testing.T) {
	ts := newTestToolSet(t)
	os.WriteFile(filepath.Join(ts.rootDir, "file.txt"), []byte("test"), 0644)

	got, err := ts.Execute("list_files", "{}")
	if err != nil {
		t.Fatalf("list_files empty input: %v", err)
	}
	if !strings.Contains(got, "file.txt") {
		t.Errorf("expected file.txt in output, got: %q", got)
	}
}

func TestToolTaskUpdate(t *testing.T) {
	ts := newTestToolSet(t)

	got, err := ts.Execute("task_update", mustMarshal(t, map[string]string{
		"stage": "active",
		"note":  "starting work",
	}))
	if err != nil {
		t.Fatalf("task_update: %v", err)
	}
	if !strings.Contains(got, "stage=active") {
		t.Errorf("expected 'stage=active' in output, got: %q", got)
	}
	if !strings.Contains(got, "starting work") {
		t.Errorf("expected note in output, got: %q", got)
	}
}

func TestToolTaskUpdate_StageOnly(t *testing.T) {
	ts := newTestToolSet(t)

	got, err := ts.Execute("task_update", mustMarshal(t, map[string]string{"stage": "validation"}))
	if err != nil {
		t.Fatalf("task_update: %v", err)
	}
	if !strings.Contains(got, "stage=validation") {
		t.Errorf("expected 'stage=validation' in output, got: %q", got)
	}
}

func TestToolTaskUpdate_EmptyInput(t *testing.T) {
	ts := newTestToolSet(t)

	got, err := ts.Execute("task_update", "{}")
	if err != nil {
		t.Fatalf("task_update empty: %v", err)
	}
	if !strings.Contains(got, "no changes") {
		t.Errorf("expected 'no changes' in output, got: %q", got)
	}
}

// --- Additional tool coverage tests ---

func TestToolFileRead_InvalidJSON(t *testing.T) {
	ts := newTestToolSet(t)
	_, err := ts.Execute("file_read", "not json")
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestToolFileRead_EmptyPath(t *testing.T) {
	ts := newTestToolSet(t)
	_, err := ts.Execute("file_read", `{"path":""}`)
	if err == nil {
		t.Fatal("expected error for empty path")
	}
}

func TestToolFileRead_Truncation(t *testing.T) {
	ts := newTestToolSet(t)

	// Create a file larger than 100KB.
	bigContent := strings.Repeat("x", 150*1024)
	if err := os.WriteFile(filepath.Join(ts.rootDir, "big.txt"), []byte(bigContent), 0644); err != nil {
		t.Fatal(err)
	}

	got, err := ts.Execute("file_read", mustMarshal(t, map[string]string{"path": "big.txt"}))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "[truncated at 100KB]") {
		t.Error("expected truncation marker in output")
	}
}

func TestToolFileRead_NonexistentFile(t *testing.T) {
	ts := newTestToolSet(t)
	_, err := ts.Execute("file_read", mustMarshal(t, map[string]string{"path": "does_not_exist.txt"}))
	if err == nil {
		t.Fatal("expected error for nonexistent file")
	}
}

func TestToolFileWrite_InvalidJSON(t *testing.T) {
	ts := newTestToolSet(t)
	_, err := ts.Execute("file_write", "not json")
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestToolFileWrite_EmptyPath(t *testing.T) {
	ts := newTestToolSet(t)
	_, err := ts.Execute("file_write", `{"path":"","content":"hello"}`)
	if err == nil {
		t.Fatal("expected error for empty path")
	}
}

func TestToolFileWrite_PathTraversal(t *testing.T) {
	ts := newTestToolSet(t)
	_, err := ts.Execute("file_write", mustMarshal(t, map[string]string{
		"path":    "../../etc/evil",
		"content": "bad",
	}))
	if err == nil {
		t.Fatal("expected error for path traversal")
	}
	if !strings.Contains(err.Error(), "path traversal") {
		t.Errorf("error = %q, want path traversal mention", err.Error())
	}
}

func TestToolBash_InvalidJSON(t *testing.T) {
	ts := newTestToolSet(t)
	_, err := ts.Execute("bash", "not json")
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestToolBash_EmptyCommand(t *testing.T) {
	ts := newTestToolSet(t)
	_, err := ts.Execute("bash", `{"command":""}`)
	if err == nil {
		t.Fatal("expected error for empty command")
	}
}

func TestToolListFiles_InvalidJSON(t *testing.T) {
	ts := newTestToolSet(t)
	_, err := ts.Execute("list_files", "not valid json at all")
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestToolListFiles_SkipsDirs(t *testing.T) {
	ts := newTestToolSet(t)

	// Create directories that should be skipped in recursive listing.
	os.MkdirAll(filepath.Join(ts.rootDir, ".git", "objects"), 0755)
	os.WriteFile(filepath.Join(ts.rootDir, ".git", "objects", "hidden.txt"), []byte("x"), 0644)
	os.MkdirAll(filepath.Join(ts.rootDir, "node_modules", "pkg"), 0755)
	os.WriteFile(filepath.Join(ts.rootDir, "node_modules", "pkg", "index.js"), []byte("x"), 0644)
	os.MkdirAll(filepath.Join(ts.rootDir, "dist"), 0755)
	os.WriteFile(filepath.Join(ts.rootDir, "dist", "bundle.js"), []byte("x"), 0644)
	os.WriteFile(filepath.Join(ts.rootDir, "visible.go"), []byte("package main"), 0644)

	got, err := ts.Execute("list_files", mustMarshal(t, map[string]interface{}{
		"path":      ".",
		"recursive": true,
	}))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(got, "hidden.txt") {
		t.Error("expected .git contents to be skipped")
	}
	if strings.Contains(got, "index.js") {
		t.Error("expected node_modules contents to be skipped")
	}
	if strings.Contains(got, "bundle.js") {
		t.Error("expected dist contents to be skipped")
	}
	if !strings.Contains(got, "visible.go") {
		t.Error("expected visible.go to be present")
	}
}

func TestToolListFiles_PathTraversal(t *testing.T) {
	ts := newTestToolSet(t)
	_, err := ts.Execute("list_files", mustMarshal(t, map[string]string{"path": "../../.."}))
	if err == nil {
		t.Fatal("expected error for path traversal")
	}
}

func TestToolListFiles_NonexistentDir(t *testing.T) {
	ts := newTestToolSet(t)
	_, err := ts.Execute("list_files", mustMarshal(t, map[string]string{"path": "no_such_dir"}))
	if err == nil {
		t.Fatal("expected error for nonexistent directory")
	}
}

func TestToolTaskUpdate_NoteOnly(t *testing.T) {
	ts := newTestToolSet(t)
	got, err := ts.Execute("task_update", mustMarshal(t, map[string]string{"note": "just a note"}))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "just a note") {
		t.Errorf("output = %q, want note content", got)
	}
}

func TestToolTaskUpdate_InvalidJSON(t *testing.T) {
	ts := newTestToolSet(t)
	_, err := ts.Execute("task_update", "bad json")
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestResolvePath_RootItself(t *testing.T) {
	ts := newTestToolSet(t)
	got, err := ts.resolvePath("")
	if err != nil {
		t.Fatalf("resolvePath empty: %v", err)
	}
	if got != ts.rootDir {
		t.Errorf("resolvePath('') = %q, want %q", got, ts.rootDir)
	}
}

func TestResolvePath_ValidSubpath(t *testing.T) {
	ts := newTestToolSet(t)
	got, err := ts.resolvePath("sub/file.go")
	if err != nil {
		t.Fatalf("resolvePath: %v", err)
	}
	expected := filepath.Join(ts.rootDir, "sub", "file.go")
	if got != expected {
		t.Errorf("resolvePath = %q, want %q", got, expected)
	}
}
