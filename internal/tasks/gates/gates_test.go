package gates

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPreMergeGate_MissingDir(t *testing.T) {
	err := PreMergeGate("/nonexistent/path/that/does/not/exist")
	if err == nil {
		t.Fatal("expected error for nonexistent directory, got nil")
	}
	if !strings.Contains(err.Error(), "worktree web dir not found") {
		t.Fatalf("unexpected error message: %s", err)
	}
}

func TestSmokeResult_AllPass(t *testing.T) {
	result := SmokeResult{
		AllPass: true,
		Checks: []SmokeCheck{
			{Name: "health", Pass: true, Detail: "200 OK"},
			{Name: "ws", Pass: true, Detail: "connected"},
		},
	}
	if !result.AllPass {
		t.Fatal("expected AllPass to be true")
	}
	if len(result.Checks) != 2 {
		t.Fatalf("expected 2 checks, got %d", len(result.Checks))
	}
	if result.Checks[0].Name != "health" {
		t.Fatalf("expected first check name 'health', got %q", result.Checks[0].Name)
	}
	if !result.Checks[1].Pass {
		t.Fatal("expected second check to pass")
	}
}

func TestFeatureCheck_Types(t *testing.T) {
	check := FeatureCheck{
		Description: "sidebar visible",
		Selector:    "[data-testid='sidebar']",
		Assertion:   "visible",
		Expected:    "true",
	}
	if check.Description != "sidebar visible" {
		t.Fatalf("unexpected description: %s", check.Description)
	}
	if check.Assertion != "visible" {
		t.Fatalf("unexpected assertion: %s", check.Assertion)
	}

	result := FeatureGateResult{
		AllPass: true,
		Checks: []FeatureCheckResult{
			{Description: "sidebar visible", Pass: true, Detail: "element found"},
		},
	}
	if !result.AllPass {
		t.Fatal("expected AllPass to be true")
	}
	if len(result.Checks) != 1 {
		t.Fatalf("expected 1 check result, got %d", len(result.Checks))
	}
}

func TestPreMergeGate_WithTempDir(t *testing.T) {
	// Create a minimal web directory — PreMergeGate should fail (no tsc/vite)
	// but should not panic.
	tmpDir := t.TempDir()
	webDir := filepath.Join(tmpDir, "web")
	if err := os.MkdirAll(webDir, 0o755); err != nil {
		t.Fatalf("failed to create temp web dir: %v", err)
	}

	err := PreMergeGate(webDir)
	if err == nil {
		t.Fatal("expected error for dir without tsc/vite setup, got nil")
	}
	// Should fail at tsc or node_modules stage, not panic
	t.Logf("expected error: %s", err)
}

func TestStepVerificationGate_MissingDir(t *testing.T) {
	err := StepVerificationGate("/nonexistent/dir", "", "", "")
	if err == nil {
		t.Fatal("expected error for nonexistent directory, got nil")
	}
	if !strings.Contains(err.Error(), "pre-merge gate") {
		t.Fatalf("expected pre-merge gate error, got: %s", err)
	}
}
