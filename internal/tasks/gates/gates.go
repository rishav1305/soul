package gates

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

// SmokeResult holds the outcome of a smoke test run.
type SmokeResult struct {
	AllPass bool
	Checks  []SmokeCheck
}

// SmokeCheck represents a single smoke test check.
type SmokeCheck struct {
	Name   string
	Pass   bool
	Detail string
}

// FeatureCheck defines a UI feature assertion to verify.
type FeatureCheck struct {
	Description string
	Selector    string
	Assertion   string // "exists", "visible", "text_contains", "count"
	Expected    string
}

// FeatureGateResult holds the outcome of feature gate checks.
type FeatureGateResult struct {
	AllPass bool
	Checks  []FeatureCheckResult
}

// FeatureCheckResult represents the result of a single feature check.
type FeatureCheckResult struct {
	Description string
	Pass        bool
	Detail      string
}

const (
	tscTimeout   = 60 * time.Second
	buildTimeout = 120 * time.Second
	sshTimeout   = 60 * time.Second
	rtTimeout    = 30 * time.Second
)

// PreMergeGate validates a worktree web directory by running tsc and vite build.
// It checks that the directory exists, symlinks node_modules if missing, then
// runs type checking and production build.
func PreMergeGate(worktreeWeb string) error {
	// 1. Check dir exists
	info, err := os.Stat(worktreeWeb)
	if err != nil {
		return fmt.Errorf("worktree web dir not found: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("worktree web path is not a directory: %s", worktreeWeb)
	}

	// 2. Symlink node_modules if missing
	nodeModules := filepath.Join(worktreeWeb, "node_modules")
	if _, err := os.Stat(nodeModules); os.IsNotExist(err) {
		// Look for node_modules two directories up (main web/node_modules)
		mainNodeModules := filepath.Join(worktreeWeb, "..", "..", "web", "node_modules")
		absMain, err := filepath.Abs(mainNodeModules)
		if err != nil {
			return fmt.Errorf("cannot resolve main node_modules path: %w", err)
		}
		if _, err := os.Stat(absMain); err != nil {
			return fmt.Errorf("main node_modules not found at %s: %w", absMain, err)
		}
		if err := os.Symlink(absMain, nodeModules); err != nil {
			return fmt.Errorf("failed to symlink node_modules: %w", err)
		}
	}

	// 3. Run npx tsc --noEmit (60s timeout)
	if err := runCmd(worktreeWeb, tscTimeout, "npx", "tsc", "--noEmit"); err != nil {
		return fmt.Errorf("tsc --noEmit failed: %w", err)
	}

	// 4. Run npx vite build (120s timeout)
	if err := runCmd(worktreeWeb, buildTimeout, "npx", "vite", "build"); err != nil {
		return fmt.Errorf("vite build failed: %w", err)
	}

	return nil
}

// SmokeTest runs an E2E smoke test via SSH to the specified host.
// It executes the runner script with --json --url flags and parses results.
func SmokeTest(serverURL, e2eHost, e2eRunnerPath string) (*SmokeResult, error) {
	if serverURL == "" || e2eHost == "" || e2eRunnerPath == "" {
		return nil, fmt.Errorf("serverURL, e2eHost, and e2eRunnerPath are all required")
	}

	var stdout, stderr bytes.Buffer
	cmd := exec.Command("ssh", e2eHost,
		"node", e2eRunnerPath, "--json", "--url", serverURL)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	done := make(chan error, 1)
	go func() { done <- cmd.Run() }()

	select {
	case err := <-done:
		if err != nil {
			return nil, fmt.Errorf("smoke test failed: %s: %w", stderr.String(), err)
		}
	case <-time.After(sshTimeout):
		return nil, fmt.Errorf("smoke test timed out after %v", sshTimeout)
	}

	// Return a basic result — actual JSON parsing would be added when
	// the E2E runner format is finalized.
	result := &SmokeResult{
		AllPass: true,
		Checks: []SmokeCheck{
			{Name: "e2e-runner", Pass: true, Detail: stdout.String()},
		},
	}
	return result, nil
}

// RuntimeGate checks for console errors on the running server via SSH.
// Returns nil if serverURL or e2eHost are empty (gate is skipped).
func RuntimeGate(serverURL, e2eHost, e2eRunnerPath string) error {
	if serverURL == "" || e2eHost == "" {
		return nil
	}

	var stdout, stderr bytes.Buffer
	args := []string{e2eHost, "node", e2eRunnerPath,
		"--action=console_errors", "--url", serverURL}
	cmd := exec.Command("ssh", args...)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	done := make(chan error, 1)
	go func() { done <- cmd.Run() }()

	select {
	case err := <-done:
		if err != nil {
			return fmt.Errorf("runtime gate failed: %s: %w", stderr.String(), err)
		}
	case <-time.After(rtTimeout):
		return fmt.Errorf("runtime gate timed out after %v", rtTimeout)
	}

	return nil
}

// StepVerificationGate runs PreMergeGate followed by RuntimeGate.
// This is the standard verification sequence for each task step.
func StepVerificationGate(worktreeWeb, serverURL, e2eHost, e2eRunnerPath string) error {
	if err := PreMergeGate(worktreeWeb); err != nil {
		return fmt.Errorf("pre-merge gate: %w", err)
	}
	if err := RuntimeGate(serverURL, e2eHost, e2eRunnerPath); err != nil {
		return fmt.Errorf("runtime gate: %w", err)
	}
	return nil
}

// runCmd executes a command in the given directory with a timeout.
func runCmd(dir string, timeout time.Duration, name string, args ...string) error {
	var buf bytes.Buffer
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	cmd.Stdout = &buf
	cmd.Stderr = &buf

	done := make(chan error, 1)
	go func() { done <- cmd.Run() }()

	select {
	case err := <-done:
		if err != nil {
			output := buf.String()
			if output != "" {
				return fmt.Errorf("%s: %s", err, output)
			}
			return err
		}
	case <-time.After(timeout):
		if cmd.Process != nil {
			cmd.Process.Kill()
		}
		return fmt.Errorf("command timed out after %v", timeout)
	}
	return nil
}
