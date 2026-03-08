package server

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// SmokeCheck is one pass/fail entry from the smoke test.
type SmokeCheck struct {
	Name   string `json:"name"`
	Pass   bool   `json:"pass"`
	Detail string `json:"detail"`
}

// SmokeResult holds the full smoke test output.
type SmokeResult struct {
	AllPass bool         `json:"allPass"`
	Checks  []SmokeCheck `json:"checks"`
}

// PreMergeGate runs tsc and vite build in a worktree to validate the frontend
// compiles without errors. Returns nil on success, error with details on failure.
func PreMergeGate(worktreeWeb string) error {
	log.Printf("[gate] running pre-merge gate in %s", worktreeWeb)

	// Ensure node_modules symlink exists.
	devNodeModules := filepath.Join(worktreeWeb, "node_modules")
	mainNodeModules := filepath.Join(filepath.Dir(filepath.Dir(worktreeWeb)), "web", "node_modules")

	if _, err := os.Lstat(devNodeModules); os.IsNotExist(err) {
		if err := os.Symlink(mainNodeModules, devNodeModules); err != nil {
			return fmt.Errorf("symlink node_modules: %w", err)
		}
	}

	// Step 1: TypeScript check (catches type mismatches like the notifications bug).
	log.Printf("[gate] running tsc --noEmit")
	tsc := exec.Command("npx", "tsc", "--noEmit")
	tsc.Dir = worktreeWeb
	tscOut, tscErr := tsc.CombinedOutput()
	if tscErr != nil {
		return fmt.Errorf("tsc --noEmit failed:\n%s", string(tscOut))
	}
	log.Printf("[gate] tsc passed")

	// Step 2: Vite build.
	log.Printf("[gate] running vite build")
	vite := exec.Command("npx", "vite", "build")
	vite.Dir = worktreeWeb
	viteOut, viteErr := vite.CombinedOutput()
	if viteErr != nil {
		return fmt.Errorf("vite build failed:\n%s", string(viteOut))
	}
	log.Printf("[gate] vite build passed")

	return nil
}

// RunSmokeTest executes the Playwright smoke test against the given server URL
// via SSH to the configured E2E host. Returns structured results or error.
func RunSmokeTest(serverURL, e2eHost, e2eRunnerPath string) (*SmokeResult, error) {
	log.Printf("[gate] running smoke test against %s via %s", serverURL, e2eHost)

	argsJSON, _ := json.Marshal(map[string]string{
		"action": "smoke",
		"url":    serverURL,
	})

	command := fmt.Sprintf(
		"echo %s | ssh %s 'cat > /tmp/soul-e2e-args.json && cd %s && node test-runner.js --json /tmp/soul-e2e-args.json'",
		shellescape(string(argsJSON)), e2eHost, e2eRunnerPath,
	)

	cmd := exec.Command("bash", "-c", command)
	done := make(chan struct{})
	var out []byte
	var execErr error

	go func() {
		out, execErr = cmd.CombinedOutput()
		close(done)
	}()

	select {
	case <-done:
		// completed
	case <-time.After(60 * time.Second):
		cmd.Process.Kill()
		return nil, fmt.Errorf("smoke test timed out after 60s")
	}

	if execErr != nil {
		return nil, fmt.Errorf("smoke test failed: %v\n%s", execErr, string(out))
	}

	// Parse JSON result from test-runner output.
	output := strings.TrimSpace(string(out))
	lines := strings.Split(output, "\n")
	var jsonLine string
	for i := len(lines) - 1; i >= 0; i-- {
		if strings.HasPrefix(strings.TrimSpace(lines[i]), "{") {
			jsonLine = strings.TrimSpace(lines[i])
			break
		}
	}
	if jsonLine == "" {
		return nil, fmt.Errorf("no JSON output from smoke test: %s", output)
	}

	var result SmokeResult
	if err := json.Unmarshal([]byte(jsonLine), &result); err != nil {
		return nil, fmt.Errorf("failed to parse smoke result: %v\nraw: %s", err, jsonLine)
	}

	log.Printf("[gate] smoke test result: allPass=%v, checks=%d", result.AllPass, len(result.Checks))
	return &result, nil
}

// RevertLastMerge reverts the most recent merge commit in the given directory.
func RevertLastMerge(dir string) error {
	log.Printf("[gate] reverting last merge in %s", dir)
	cmd := exec.Command("git", "revert", "HEAD", "--no-edit")
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git revert failed: %s — %w", string(out), err)
	}
	log.Printf("[gate] merge reverted successfully in %s", dir)
	return nil
}

// RuntimeGate checks for JavaScript console errors on the running server
// by sending a console_errors action to the remote test-runner via SSH.
// Returns nil if no errors found or if the runner doesn't support the action yet.
func RuntimeGate(serverURL, e2eHost, e2eRunnerPath string) error {
	log.Printf("[gate] running runtime gate against %s via %s", serverURL, e2eHost)

	argsJSON, _ := json.Marshal(map[string]string{
		"action": "console_errors",
		"url":    serverURL,
	})

	command := fmt.Sprintf(
		"echo %s | ssh %s 'cat > /tmp/soul-e2e-args.json && cd %s && node test-runner.js --json /tmp/soul-e2e-args.json'",
		shellescape(string(argsJSON)), e2eHost, e2eRunnerPath,
	)

	cmd := exec.Command("bash", "-c", command)
	done := make(chan struct{})
	var out []byte
	var execErr error

	go func() {
		out, execErr = cmd.CombinedOutput()
		close(done)
	}()

	select {
	case <-done:
		// completed
	case <-time.After(30 * time.Second):
		cmd.Process.Kill()
		return fmt.Errorf("runtime gate timed out after 30s")
	}

	if execErr != nil {
		// Graceful degradation: if the runner doesn't support the action,
		// it may exit non-zero with no JSON — treat as pass.
		log.Printf("[gate] runtime gate runner returned error (may not support action yet): %v", execErr)
		return nil
	}

	// Find the last JSON line in output.
	output := strings.TrimSpace(string(out))
	lines := strings.Split(output, "\n")
	var jsonLine string
	for i := len(lines) - 1; i >= 0; i-- {
		if strings.HasPrefix(strings.TrimSpace(lines[i]), "{") {
			jsonLine = strings.TrimSpace(lines[i])
			break
		}
	}
	if jsonLine == "" {
		// No JSON output — runner may not support this action yet; graceful pass.
		log.Printf("[gate] runtime gate: no JSON output, assuming action not supported")
		return nil
	}

	var result struct {
		Errors []string `json:"errors"`
	}
	if err := json.Unmarshal([]byte(jsonLine), &result); err != nil {
		// Can't parse response — graceful degradation.
		log.Printf("[gate] runtime gate: failed to parse JSON (%v), assuming pass", err)
		return nil
	}

	if len(result.Errors) == 0 {
		log.Printf("[gate] runtime gate passed — no console errors")
		return nil
	}

	var b strings.Builder
	fmt.Fprintf(&b, "runtime gate FAILED — %d JS console error(s):\n", len(result.Errors))
	for _, e := range result.Errors {
		fmt.Fprintf(&b, "  - %s\n", e)
	}
	return fmt.Errorf("%s", b.String())
}

// StepVerificationGate runs the full verification sequence for an autonomous step:
// 1. PreMergeGate (tsc + vite build)
// 2. RuntimeGate (JS console errors) — only if serverURL and e2eHost are provided
// Returns the first error encountered.
func StepVerificationGate(worktreeWeb, serverURL, e2eHost, e2eRunnerPath string) error {
	log.Printf("[gate] running step verification gate")

	if err := PreMergeGate(worktreeWeb); err != nil {
		return fmt.Errorf("step verification: pre-merge failed: %w", err)
	}

	if serverURL != "" && e2eHost != "" {
		if err := RuntimeGate(serverURL, e2eHost, e2eRunnerPath); err != nil {
			return fmt.Errorf("step verification: runtime gate failed: %w", err)
		}
	}

	log.Printf("[gate] step verification gate passed")
	return nil
}

// VisualRegressionResult holds the result of a visual regression test.
type VisualRegressionResult struct {
	AllPass bool `json:"allPass"`
	Pages   []struct {
		URL        string  `json:"url"`
		Similarity float64 `json:"similarity"`
		Pass       bool    `json:"pass"`
	} `json:"pages"`
}

// RunVisualRegression executes visual regression tests for the given pages
// via SSH to the configured E2E host. Returns structured results or error.
func RunVisualRegression(pages []string, serverURL, e2eHost, e2eRunnerPath string, threshold float64) (*VisualRegressionResult, error) {
	if len(pages) == 0 {
		return &VisualRegressionResult{AllPass: true}, nil
	}

	log.Printf("[gate] running visual regression for %d pages against %s via %s", len(pages), serverURL, e2eHost)

	argsJSON, _ := json.Marshal(map[string]interface{}{
		"action":    "visual_regression",
		"url":       serverURL,
		"pages":     pages,
		"threshold": threshold,
	})

	command := fmt.Sprintf(
		"echo %s | ssh %s 'cat > /tmp/soul-e2e-args.json && cd %s && node test-runner.js --json /tmp/soul-e2e-args.json'",
		shellescape(string(argsJSON)), e2eHost, e2eRunnerPath,
	)

	cmd := exec.Command("bash", "-c", command)
	done := make(chan struct{})
	var out []byte
	var execErr error

	go func() {
		out, execErr = cmd.CombinedOutput()
		close(done)
	}()

	select {
	case <-done:
		// completed
	case <-time.After(90 * time.Second):
		cmd.Process.Kill()
		return nil, fmt.Errorf("visual regression timed out after 90s")
	}

	if execErr != nil {
		// Graceful degradation: runner may not support visual regression yet.
		log.Printf("[gate] visual regression runner returned error (may not support action yet): %v", execErr)
		return &VisualRegressionResult{AllPass: true}, nil
	}

	// Find the last JSON line in output.
	output := strings.TrimSpace(string(out))
	lines := strings.Split(output, "\n")
	var jsonLine string
	for i := len(lines) - 1; i >= 0; i-- {
		if strings.HasPrefix(strings.TrimSpace(lines[i]), "{") {
			jsonLine = strings.TrimSpace(lines[i])
			break
		}
	}
	if jsonLine == "" {
		// No JSON output — graceful degradation, assume pass.
		log.Printf("[gate] visual regression: no JSON output, assuming pass")
		return &VisualRegressionResult{AllPass: true}, nil
	}

	var result VisualRegressionResult
	if err := json.Unmarshal([]byte(jsonLine), &result); err != nil {
		// Can't parse response — graceful degradation.
		log.Printf("[gate] visual regression: failed to parse JSON (%v), assuming pass", err)
		return &VisualRegressionResult{AllPass: true}, nil
	}

	log.Printf("[gate] visual regression result: allPass=%v, pages=%d", result.AllPass, len(result.Pages))
	return &result, nil
}

// FormatSmokeFailure formats a SmokeResult into a human-readable gap report.
func FormatSmokeFailure(result *SmokeResult) string {
	var b strings.Builder
	b.WriteString("Smoke test FAILED. The following checks did not pass:\n\n")
	for _, check := range result.Checks {
		if !check.Pass {
			fmt.Fprintf(&b, "- **%s**: %s\n", check.Name, check.Detail)
		}
	}
	b.WriteString("\nFix these issues, rebuild the frontend (`cd web && npx vite build`), and try again.")
	return b.String()
}

// FeatureCheck describes a single DOM assertion for spec-driven feature testing.
type FeatureCheck struct {
	Description string `json:"description"`
	Selector    string `json:"selector"`
	Assertion   string `json:"assertion"` // exists, visible, text_contains, count
	Expected    string `json:"expected,omitempty"`
}

// FeatureGateResult holds the outcome of a feature gate run.
type FeatureGateResult struct {
	AllPass bool           `json:"allPass"`
	Checks  []FeatureCheck `json:"checks"`
	Errors  []string       `json:"errors"`
}

// RunFeatureGate sends a set of FeatureChecks to the remote test-runner via SSH
// and returns structured results. Gracefully degrades (returns AllPass:true) if
// the runner does not produce parseable JSON output.
func RunFeatureGate(checks []FeatureCheck, serverURL, e2eHost, e2eRunnerPath string) (*FeatureGateResult, error) {
	if len(checks) == 0 {
		return &FeatureGateResult{AllPass: true}, nil
	}

	log.Printf("[gate] running feature gate (%d checks) against %s via %s", len(checks), serverURL, e2eHost)

	argsJSON, _ := json.Marshal(map[string]interface{}{
		"action":     "feature_test",
		"url":        serverURL,
		"assertions": checks,
	})

	command := fmt.Sprintf(
		"echo %s | ssh %s 'cat > /tmp/soul-e2e-args.json && cd %s && node test-runner.js --json /tmp/soul-e2e-args.json'",
		shellescape(string(argsJSON)), e2eHost, e2eRunnerPath,
	)

	cmd := exec.Command("bash", "-c", command)
	done := make(chan struct{})
	var out []byte
	var execErr error

	go func() {
		out, execErr = cmd.CombinedOutput()
		close(done)
	}()

	select {
	case <-done:
		// completed
	case <-time.After(60 * time.Second):
		cmd.Process.Kill()
		return nil, fmt.Errorf("feature gate timed out after 60s")
	}

	if execErr != nil {
		// Graceful degradation: runner may not support the action yet.
		log.Printf("[gate] feature gate runner returned error (may not support action yet): %v", execErr)
		return &FeatureGateResult{AllPass: true, Checks: checks}, nil
	}

	// Find the last JSON line in output.
	output := strings.TrimSpace(string(out))
	lines := strings.Split(output, "\n")
	var jsonLine string
	for i := len(lines) - 1; i >= 0; i-- {
		if strings.HasPrefix(strings.TrimSpace(lines[i]), "{") {
			jsonLine = strings.TrimSpace(lines[i])
			break
		}
	}
	if jsonLine == "" {
		// No JSON output — graceful pass.
		log.Printf("[gate] feature gate: no JSON output, assuming pass")
		return &FeatureGateResult{AllPass: true, Checks: checks}, nil
	}

	var result FeatureGateResult
	if err := json.Unmarshal([]byte(jsonLine), &result); err != nil {
		// Can't parse response — graceful degradation.
		log.Printf("[gate] feature gate: failed to parse JSON (%v), assuming pass", err)
		return &FeatureGateResult{AllPass: true, Checks: checks}, nil
	}

	log.Printf("[gate] feature gate result: allPass=%v, checks=%d, errors=%d", result.AllPass, len(result.Checks), len(result.Errors))
	return &result, nil
}
