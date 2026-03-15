package ws

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadOnlyTool_FileRead(t *testing.T) {
	dir := t.TempDir()
	content := "hello world\nline two\n"
	if err := os.WriteFile(filepath.Join(dir, "test.txt"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	result := executeReadOnlyTool(dir, "file_read", `{"path":"test.txt"}`)
	if result != content {
		t.Errorf("expected %q, got %q", content, result)
	}
}

func TestReadOnlyTool_FileRead_NotFound(t *testing.T) {
	dir := t.TempDir()
	result := executeReadOnlyTool(dir, "file_read", `{"path":"nonexistent.txt"}`)
	if !strings.Contains(result, "error:") {
		t.Errorf("expected error message, got %q", result)
	}
}

func TestReadOnlyTool_FileRead_PathTraversal(t *testing.T) {
	dir := t.TempDir()
	result := executeReadOnlyTool(dir, "file_read", `{"path":"../../etc/passwd"}`)
	if !strings.Contains(result, "path traversal not allowed") {
		t.Errorf("expected path traversal error, got %q", result)
	}
}

func TestReadOnlyTool_FileRead_SizeLimit(t *testing.T) {
	dir := t.TempDir()
	// Create a 200KB file.
	data := make([]byte, 200*1024)
	for i := range data {
		data[i] = 'A'
	}
	if err := os.WriteFile(filepath.Join(dir, "big.txt"), data, 0o644); err != nil {
		t.Fatal(err)
	}

	result := executeReadOnlyTool(dir, "file_read", `{"path":"big.txt"}`)
	if !strings.Contains(result, "[truncated at 100KB]") {
		t.Error("expected truncation notice")
	}
	// Should be 100KB + truncation message length, not 200KB.
	if len(result) > 110*1024 {
		t.Errorf("result too large: %d bytes", len(result))
	}
}

func TestReadOnlyTool_FileSearch(t *testing.T) {
	dir := t.TempDir()
	// Create some files.
	for _, name := range []string{"main.go", "handler.go", "readme.md"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("content"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	result := executeReadOnlyTool(dir, "file_search", `{"query":".go"}`)
	if !strings.Contains(result, "main.go") {
		t.Errorf("expected main.go in results, got %q", result)
	}
	if !strings.Contains(result, "handler.go") {
		t.Errorf("expected handler.go in results, got %q", result)
	}
	if strings.Contains(result, "readme.md") {
		t.Errorf("did not expect readme.md in results, got %q", result)
	}
}

func TestReadOnlyTool_FileGrep(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "code.go"), []byte("func main() {\n\tfmt.Println(\"hello\")\n}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	result := executeReadOnlyTool(dir, "file_grep", `{"pattern":"Println"}`)
	if !strings.Contains(result, "Println") {
		t.Errorf("expected grep match, got %q", result)
	}
	if !strings.Contains(result, "code.go") {
		t.Errorf("expected file name in results, got %q", result)
	}
}

func TestReadOnlyTool_FileGlob(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"a.txt", "b.txt", "c.go"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	result := executeReadOnlyTool(dir, "file_glob", `{"pattern":"*.txt"}`)
	if !strings.Contains(result, "a.txt") {
		t.Errorf("expected a.txt, got %q", result)
	}
	if !strings.Contains(result, "b.txt") {
		t.Errorf("expected b.txt, got %q", result)
	}
	if strings.Contains(result, "c.go") {
		t.Errorf("did not expect c.go, got %q", result)
	}
}

func TestReadOnlyTool_Unknown(t *testing.T) {
	result := executeReadOnlyTool("/tmp", "file_write", `{"path":"x","content":"y"}`)
	if !strings.Contains(result, "not available in read-only mode") {
		t.Errorf("expected read-only error, got %q", result)
	}
}

func TestResolvePath_Basic(t *testing.T) {
	root := "/home/user/project"
	abs, err := resolvePath(root, "src/main.go")
	if err != nil {
		t.Fatal(err)
	}
	if abs != "/home/user/project/src/main.go" {
		t.Errorf("unexpected path: %s", abs)
	}
}

func TestResolvePath_Empty(t *testing.T) {
	root := "/home/user/project"
	abs, err := resolvePath(root, "")
	if err != nil {
		t.Fatal(err)
	}
	if abs != root {
		t.Errorf("expected root, got %s", abs)
	}
}

func TestResolvePath_Traversal(t *testing.T) {
	root := "/home/user/project"
	_, err := resolvePath(root, "../../etc/passwd")
	if err == nil {
		t.Error("expected error for path traversal")
	}
}
