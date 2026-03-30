package executor

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestVerifyL1_ValidGo(t *testing.T) {
	dir := t.TempDir()

	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module test\ngo 1.24\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\nfunc main() {}\n"), 0644); err != nil {
		t.Fatal(err)
	}

	result := VerifyL1(t.Context(), dir)
	if !result.Passed {
		t.Errorf("expected Passed=true, got false; errors: %v", result.Errors)
	}
}

func TestVerifyL1_InvalidGo(t *testing.T) {
	dir := t.TempDir()

	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module test\ngo 1.24\n"), 0644); err != nil {
		t.Fatal(err)
	}
	// unused variable — go vet will flag this
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\nfunc main() { x := 1 }\n"), 0644); err != nil {
		t.Fatal(err)
	}

	result := VerifyL1(t.Context(), dir)
	if result.Passed {
		t.Error("expected Passed=false for invalid Go code")
	}
	if len(result.Errors) == 0 {
		t.Error("expected non-empty Errors for invalid Go code")
	}
}

func TestVerifyL1_NoGoMod(t *testing.T) {
	dir := t.TempDir()

	if err := os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("hello\n"), 0644); err != nil {
		t.Fatal(err)
	}

	result := VerifyL1(t.Context(), dir)
	if !result.Passed {
		t.Errorf("expected Passed=true for dir with no go.mod, got false; errors: %v", result.Errors)
	}
}

func TestVerifyResult_String(t *testing.T) {
	vr := &VerifyResult{
		Passed: false,
		Errors: []string{"go vet failed: something went wrong"},
	}
	s := vr.String()
	if s == "" {
		t.Error("expected non-empty String() for failed VerifyResult")
	}
	if !strings.Contains(s, "FAILED") {
		t.Errorf("expected String() to contain 'FAILED', got: %q", s)
	}
}

// --- Additional verify coverage tests ---

func TestVerifyResult_StringPassed(t *testing.T) {
	vr := &VerifyResult{Passed: true}
	s := vr.String()
	if !strings.Contains(s, "PASSED") {
		t.Errorf("expected PASSED in string, got %q", s)
	}
}

func TestVerifyL1_WithTsconfig(t *testing.T) {
	dir := t.TempDir()

	// Create a tsconfig.json to exercise the direct-path detection branch.
	if err := os.WriteFile(filepath.Join(dir, "tsconfig.json"), []byte(`{"compilerOptions":{}}`), 0644); err != nil {
		t.Fatal(err)
	}

	result := VerifyL1(t.Context(), dir)
	// tsc may or may not be installed — just verify the path is exercised without panic.
	_ = result
}

func TestVerifyL1_WithWebTsconfig(t *testing.T) {
	dir := t.TempDir()

	// Create web/tsconfig.json to exercise the fallback detection branch.
	webDir := filepath.Join(dir, "web")
	if err := os.MkdirAll(webDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(webDir, "tsconfig.json"), []byte(`{"compilerOptions":{}}`), 0644); err != nil {
		t.Fatal(err)
	}

	result := VerifyL1(t.Context(), dir)
	_ = result
}

func TestFileExists(t *testing.T) {
	dir := t.TempDir()

	// Non-existent file.
	if fileExists(filepath.Join(dir, "nope.txt")) {
		t.Error("fileExists returned true for non-existent file")
	}

	// Existing file.
	f := filepath.Join(dir, "exists.txt")
	os.WriteFile(f, []byte("hi"), 0644)
	if !fileExists(f) {
		t.Error("fileExists returned false for existing file")
	}

	// Directory (should return false — fileExists checks !IsDir).
	if fileExists(dir) {
		t.Error("fileExists returned true for a directory")
	}
}

func TestVerifyL1_ContextTimeout(t *testing.T) {
	dir := t.TempDir()

	// Create go.mod so go vet runs.
	os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module test\ngo 1.24\n"), 0644)
	os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\nfunc main() {}\n"), 0644)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	result := VerifyL1(ctx, dir)
	// Cancelled context may cause go vet to fail — that's expected.
	_ = result
}
