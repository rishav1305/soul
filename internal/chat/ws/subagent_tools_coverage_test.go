package ws

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// --- execROFileRead edge cases ---

func TestExecROFileRead_EmptyPath(t *testing.T) {
	dir := t.TempDir()
	result := execROFileRead(dir, `{"path":""}`)
	if !strings.Contains(result, "error: path is required") {
		t.Errorf("expected path required error, got %q", result)
	}
}

func TestExecROFileRead_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	result := execROFileRead(dir, `{bad json}`)
	if !strings.Contains(result, "error: invalid input") {
		t.Errorf("expected invalid input error, got %q", result)
	}
}

// --- execROFileSearch edge cases ---

func TestExecROFileSearch_EmptyQuery(t *testing.T) {
	dir := t.TempDir()
	result := execROFileSearch(dir, `{"query":""}`)
	if !strings.Contains(result, "error: query is required") {
		t.Errorf("expected query required error, got %q", result)
	}
}

func TestExecROFileSearch_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	result := execROFileSearch(dir, `{invalid}`)
	if !strings.Contains(result, "error: invalid input") {
		t.Errorf("expected invalid input error, got %q", result)
	}
}

func TestExecROFileSearch_WithDirectory(t *testing.T) {
	dir := t.TempDir()
	subDir := filepath.Join(dir, "sub")
	if err := os.MkdirAll(subDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(subDir, "target.go"), []byte("package main"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Also put a file in root to verify directory scoping.
	if err := os.WriteFile(filepath.Join(dir, "other.go"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	result := execROFileSearch(dir, `{"query":"target","directory":"sub"}`)
	if !strings.Contains(result, "target.go") {
		t.Errorf("expected target.go, got %q", result)
	}
}

func TestExecROFileSearch_NoResults(t *testing.T) {
	dir := t.TempDir()
	result := execROFileSearch(dir, `{"query":"nonexistent_file_xyz"}`)
	if !strings.Contains(result, "no files found") {
		t.Errorf("expected 'no files found', got %q", result)
	}
}

func TestExecROFileSearch_PathTraversal(t *testing.T) {
	dir := t.TempDir()
	result := execROFileSearch(dir, `{"query":"test","directory":"../../etc"}`)
	if !strings.Contains(result, "path traversal not allowed") {
		t.Errorf("expected path traversal error, got %q", result)
	}
}

// --- execROFileGrep edge cases ---

func TestExecROFileGrep_EmptyPattern(t *testing.T) {
	dir := t.TempDir()
	result := execROFileGrep(dir, `{"pattern":""}`)
	if !strings.Contains(result, "error: pattern is required") {
		t.Errorf("expected pattern required error, got %q", result)
	}
}

func TestExecROFileGrep_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	result := execROFileGrep(dir, `{invalid}`)
	if !strings.Contains(result, "error: invalid input") {
		t.Errorf("expected invalid input error, got %q", result)
	}
}

func TestExecROFileGrep_WithDirectory(t *testing.T) {
	dir := t.TempDir()
	subDir := filepath.Join(dir, "src")
	if err := os.MkdirAll(subDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(subDir, "main.go"), []byte("func main() { println(\"hello\") }"), 0o644); err != nil {
		t.Fatal(err)
	}

	result := execROFileGrep(dir, `{"pattern":"println","directory":"src"}`)
	if !strings.Contains(result, "println") {
		t.Errorf("expected grep match, got %q", result)
	}
}

func TestExecROFileGrep_WithInclude(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "code.go"), []byte("func hello() {}"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "note.txt"), []byte("func hello"), 0o644); err != nil {
		t.Fatal(err)
	}

	result := execROFileGrep(dir, `{"pattern":"hello","include":"*.go"}`)
	if !strings.Contains(result, "code.go") {
		t.Errorf("expected code.go in results, got %q", result)
	}
	// note.txt should be excluded by the include filter.
	if strings.Contains(result, "note.txt") {
		t.Errorf("expected note.txt to be excluded, got %q", result)
	}
}

func TestExecROFileGrep_NoMatches(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}

	result := execROFileGrep(dir, `{"pattern":"zzz_nonexistent_pattern_zzz"}`)
	if !strings.Contains(result, "no matches found") {
		t.Errorf("expected 'no matches found', got %q", result)
	}
}

func TestExecROFileGrep_PathTraversal(t *testing.T) {
	dir := t.TempDir()
	result := execROFileGrep(dir, `{"pattern":"test","directory":"../../etc"}`)
	if !strings.Contains(result, "path traversal not allowed") {
		t.Errorf("expected path traversal error, got %q", result)
	}
}

// --- execROFileGlob edge cases ---

func TestExecROFileGlob_EmptyPattern(t *testing.T) {
	dir := t.TempDir()
	result := execROFileGlob(dir, `{"pattern":""}`)
	if !strings.Contains(result, "error: pattern is required") {
		t.Errorf("expected pattern required error, got %q", result)
	}
}

func TestExecROFileGlob_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	result := execROFileGlob(dir, `{bad}`)
	if !strings.Contains(result, "error: invalid input") {
		t.Errorf("expected invalid input error, got %q", result)
	}
}

func TestExecROFileGlob_NoMatches(t *testing.T) {
	dir := t.TempDir()
	result := execROFileGlob(dir, `{"pattern":"*.xyz"}`)
	if !strings.Contains(result, "no files found") {
		t.Errorf("expected 'no files found', got %q", result)
	}
}

func TestExecROFileGlob_WithDirectory(t *testing.T) {
	dir := t.TempDir()
	subDir := filepath.Join(dir, "pkg")
	if err := os.MkdirAll(subDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(subDir, "lib.go"), []byte("package pkg"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main"), 0o644); err != nil {
		t.Fatal(err)
	}

	result := execROFileGlob(dir, `{"pattern":"*.go","directory":"pkg"}`)
	if !strings.Contains(result, "lib.go") {
		t.Errorf("expected lib.go, got %q", result)
	}
}

func TestExecROFileGlob_InvalidGlobPattern(t *testing.T) {
	dir := t.TempDir()
	// filepath.Glob returns error for malformed patterns like "["
	result := execROFileGlob(dir, `{"pattern":"["}`)
	if !strings.Contains(result, "error: invalid glob pattern") {
		t.Errorf("expected invalid glob error, got %q", result)
	}
}

func TestExecROFileGlob_PathTraversal(t *testing.T) {
	dir := t.TempDir()
	result := execROFileGlob(dir, `{"pattern":"*.go","directory":"../../etc"}`)
	if !strings.Contains(result, "path traversal not allowed") {
		t.Errorf("expected path traversal error, got %q", result)
	}
}

// --- executeReadOnlyTool dispatch edge cases ---

func TestExecuteReadOnlyTool_AllTools(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "test.go"), []byte("package main\nfunc hello() {}"), 0o644); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name  string
		tool  string
		input string
		want  string
	}{
		{"file_read", "file_read", `{"path":"test.go"}`, "package main"},
		{"file_search", "file_search", `{"query":"test"}`, "test.go"},
		{"file_grep", "file_grep", `{"pattern":"hello"}`, "hello"},
		{"file_glob", "file_glob", `{"pattern":"*.go"}`, "test.go"},
		{"unknown", "file_delete", `{}`, "not available in read-only mode"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := executeReadOnlyTool(dir, tt.tool, tt.input)
			if !strings.Contains(result, tt.want) {
				t.Errorf("tool %q: expected %q in result, got %q", tt.tool, tt.want, result)
			}
		})
	}
}
