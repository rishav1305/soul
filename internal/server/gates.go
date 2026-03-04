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
// via SSH to titan-pc. Returns structured results or error.
func RunSmokeTest(serverURL string) (*SmokeResult, error) {
	log.Printf("[gate] running smoke test against %s", serverURL)

	argsJSON, _ := json.Marshal(map[string]string{
		"action": "smoke",
		"url":    serverURL,
	})

	command := fmt.Sprintf(
		"echo %s | ssh titan-pc 'cat > /tmp/soul-e2e-args.json && cd ~/soul-e2e && node test-runner.js --json /tmp/soul-e2e-args.json'",
		shellescape(string(argsJSON)),
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
