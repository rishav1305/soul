package executor

import (
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
