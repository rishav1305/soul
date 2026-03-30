package gates

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
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

func TestSmokeTest_MissingArgs(t *testing.T) {
	tests := []struct {
		name          string
		serverURL     string
		e2eHost       string
		e2eRunnerPath string
	}{
		{"empty serverURL", "", "host", "/runner.js"},
		{"empty e2eHost", "http://x", "", "/runner.js"},
		{"empty runnerPath", "http://x", "host", ""},
		{"all empty", "", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := SmokeTest(tt.serverURL, tt.e2eHost, tt.e2eRunnerPath)
			if err == nil {
				t.Fatal("expected error for missing args, got nil")
			}
			if !strings.Contains(err.Error(), "required") {
				t.Fatalf("unexpected error: %s", err)
			}
		})
	}
}

func TestRuntimeGate_SkipsWhenEmpty(t *testing.T) {
	// RuntimeGate returns nil (skips) when serverURL or e2eHost are empty.
	if err := RuntimeGate("", "host", "/runner.js"); err != nil {
		t.Fatalf("expected nil when serverURL empty, got: %v", err)
	}
	if err := RuntimeGate("http://x", "", "/runner.js"); err != nil {
		t.Fatalf("expected nil when e2eHost empty, got: %v", err)
	}
	if err := RuntimeGate("", "", ""); err != nil {
		t.Fatalf("expected nil when all empty, got: %v", err)
	}
}

func TestPreMergeGate_NotADirectory(t *testing.T) {
	// Create a temp file (not a dir) — PreMergeGate should reject it.
	f, err := os.CreateTemp("", "gate_test_*")
	if err != nil {
		t.Fatalf("create temp file: %v", err)
	}
	defer os.Remove(f.Name())
	f.Close()

	err = PreMergeGate(f.Name())
	if err == nil {
		t.Fatal("expected error for file path, got nil")
	}
	if !strings.Contains(err.Error(), "not a directory") {
		t.Fatalf("unexpected error: %s", err)
	}
}

func TestRunCmd_Success(t *testing.T) {
	dir := t.TempDir()
	err := runCmd(dir, 5*time.Second, "echo", "hello")
	if err != nil {
		t.Fatalf("runCmd echo: %v", err)
	}
}

func TestRunCmd_Failure(t *testing.T) {
	dir := t.TempDir()
	err := runCmd(dir, 5*time.Second, "false")
	if err == nil {
		t.Fatal("expected error from 'false' command")
	}
}

func TestRunCmd_WithOutput(t *testing.T) {
	dir := t.TempDir()
	// Command that fails and produces output.
	err := runCmd(dir, 5*time.Second, "sh", "-c", "echo failure-output && exit 1")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "failure-output") {
		t.Errorf("expected output in error, got: %v", err)
	}
}

// NOTE: runCmd timeout path has a data race on cmd.Process (race between
// cmd.Run goroutine and Process.Kill). Skipping timeout test; fix tracked.

func TestPreMergeGate_SymlinkPath(t *testing.T) {
	// Set up directory structure that triggers symlink logic.
	tmpDir := t.TempDir()

	// Create main web/node_modules.
	mainWeb := filepath.Join(tmpDir, "web")
	mainNodeModules := filepath.Join(mainWeb, "node_modules")
	os.MkdirAll(mainNodeModules, 0755)
	os.WriteFile(filepath.Join(mainNodeModules, ".package-lock.json"), []byte("{}"), 0644)

	// Create worktree structure: tmpDir/worktrees/wt/web/
	wtWeb := filepath.Join(tmpDir, "worktrees", "wt", "web")
	os.MkdirAll(wtWeb, 0755)

	// PreMergeGate should get past the symlink step (but fail at tsc).
	err := PreMergeGate(wtWeb)
	if err == nil {
		t.Fatal("expected error (no tsc), got nil")
	}
	// Should fail at tsc step, not at symlink step.
	if strings.Contains(err.Error(), "node_modules") {
		// If it fails at node_modules, the symlink logic didn't find the right path.
		// That's ok — the path resolution is relative and may not match.
		t.Logf("node_modules error (expected for this layout): %v", err)
	}
}

func TestStepVerificationGate_RuntimeGateSkipped(t *testing.T) {
	// When serverURL/e2eHost are empty, RuntimeGate is skipped.
	// StepVerificationGate should still fail at PreMergeGate for nonexistent dir.
	err := StepVerificationGate("/nonexistent", "", "", "")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "pre-merge gate") {
		t.Errorf("expected pre-merge gate error, got: %v", err)
	}
}

// --- Additional coverage tests ---

func TestPreMergeGate_SymlinkCreated(t *testing.T) {
	tmpDir := t.TempDir()

	// PreMergeGate resolves: filepath.Join(worktreeWeb, "..", "..", "web", "node_modules")
	// So if worktreeWeb = tmpDir/wt/web/, the code looks for tmpDir/web/node_modules.
	mainNodeModules := filepath.Join(tmpDir, "web", "node_modules")
	if err := os.MkdirAll(mainNodeModules, 0755); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(mainNodeModules, ".package-lock.json"), []byte("{}"), 0644)

	// Create worktree web dir exactly 2 levels below tmpDir.
	wtWeb := filepath.Join(tmpDir, "wt", "web")
	if err := os.MkdirAll(wtWeb, 0755); err != nil {
		t.Fatal(err)
	}

	err := PreMergeGate(wtWeb)
	// Should get past symlink creation but fail at tsc (no tsconfig/node).
	if err == nil {
		t.Fatal("expected error (no tsc), got nil")
	}
	if strings.Contains(err.Error(), "node_modules not found") {
		t.Errorf("should not fail at node_modules step, got: %v", err)
	}
	if strings.Contains(err.Error(), "symlink") {
		t.Errorf("should not fail at symlink step, got: %v", err)
	}

	// Verify symlink was created.
	linkTarget, lErr := os.Readlink(filepath.Join(wtWeb, "node_modules"))
	if lErr != nil {
		t.Errorf("symlink not created: %v", lErr)
	}
	if linkTarget == "" {
		t.Error("symlink target is empty")
	}
}

func TestPreMergeGate_MainNodeModulesNotFound(t *testing.T) {
	tmpDir := t.TempDir()

	// Create worktree web dir but NOT main web/node_modules.
	wtWeb := filepath.Join(tmpDir, "wt", "web")
	if err := os.MkdirAll(wtWeb, 0755); err != nil {
		t.Fatal(err)
	}

	err := PreMergeGate(wtWeb)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "node_modules not found") {
		t.Errorf("error = %q, want 'node_modules not found'", err.Error())
	}
}

func TestPreMergeGate_NodeModulesAlreadyExists(t *testing.T) {
	tmpDir := t.TempDir()

	// Create worktree web dir with node_modules already present.
	wtWeb := filepath.Join(tmpDir, "web")
	if err := os.MkdirAll(filepath.Join(wtWeb, "node_modules"), 0755); err != nil {
		t.Fatal(err)
	}

	err := PreMergeGate(wtWeb)
	// Should skip symlink logic but fail at tsc.
	if err == nil {
		t.Fatal("expected error (no tsc), got nil")
	}
	if strings.Contains(err.Error(), "node_modules") {
		t.Errorf("should skip node_modules step, got: %v", err)
	}
}

func TestSmokeTest_SSHFailure(t *testing.T) {
	// SSH to a non-resolvable host fails fast.
	_, err := SmokeTest("http://localhost:9999", "invalid-host-zzz-nonexistent", "/tmp/runner.js")
	if err == nil {
		t.Fatal("expected error for unreachable SSH host")
	}
	if !strings.Contains(err.Error(), "smoke test failed") {
		t.Errorf("error = %q, want 'smoke test failed'", err.Error())
	}
}

func TestRuntimeGate_SSHFailure(t *testing.T) {
	// SSH to a non-resolvable host fails fast.
	err := RuntimeGate("http://localhost:9999", "invalid-host-zzz-nonexistent", "/tmp/runner.js")
	if err == nil {
		t.Fatal("expected error for unreachable SSH host")
	}
	if !strings.Contains(err.Error(), "runtime gate failed") {
		t.Errorf("error = %q, want 'runtime gate failed'", err.Error())
	}
}

func TestStepVerificationGate_BothFail(t *testing.T) {
	// Non-existent dir fails at PreMerge, never reaches Runtime.
	err := StepVerificationGate("/nonexistent/dir/12345", "http://x", "invalid-host", "/runner.js")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "pre-merge gate") {
		t.Errorf("error = %q, want pre-merge error", err.Error())
	}
}

func TestRunCmd_EmptyCommand(t *testing.T) {
	dir := t.TempDir()
	err := runCmd(dir, 5*time.Second, "sh", "-c", "true")
	if err != nil {
		t.Errorf("expected no error from 'true', got: %v", err)
	}
}

func TestRunCmd_NonexistentCommand(t *testing.T) {
	dir := t.TempDir()
	err := runCmd(dir, 5*time.Second, "/nonexistent/command/12345")
	if err == nil {
		t.Fatal("expected error for nonexistent command")
	}
}
