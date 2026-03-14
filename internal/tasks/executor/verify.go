package executor

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const verifyTimeout = 60 * time.Second

// VerifyResult holds the outcome of an L1 verification run.
type VerifyResult struct {
	Passed bool
	Errors []string
}

// String returns a human-readable summary of the verification result.
func (vr *VerifyResult) String() string {
	if vr.Passed {
		return "L1 verification: PASSED"
	}
	return "L1 verification: FAILED\n" + strings.Join(vr.Errors, "\n")
}

// VerifyL1 runs static analysis checks appropriate for the project in dir.
// It runs go vet if a go.mod is present, and tsc --noEmit if a tsconfig.json
// is present (checking web/tsconfig.json as a fallback for monorepos).
func VerifyL1(ctx context.Context, dir string) *VerifyResult {
	ctx, cancel := context.WithTimeout(ctx, verifyTimeout)
	defer cancel()

	result := &VerifyResult{Passed: true}

	// Go vet check
	if fileExists(filepath.Join(dir, "go.mod")) {
		var buf bytes.Buffer
		cmd := exec.CommandContext(ctx, "go", "vet", "./...")
		cmd.Dir = dir
		cmd.Stdout = &buf
		cmd.Stderr = &buf
		if err := cmd.Run(); err != nil {
			result.Passed = false
			output := strings.TrimSpace(buf.String())
			msg := "go vet failed"
			if output != "" {
				msg += ": " + output
			}
			result.Errors = append(result.Errors, msg)
		}
	}

	// TypeScript check
	tsconfigDirect := filepath.Join(dir, "tsconfig.json")
	tsconfigWeb := filepath.Join(dir, "web", "tsconfig.json")

	var tscDir string
	if fileExists(tsconfigDirect) {
		tscDir = dir
	} else if fileExists(tsconfigWeb) {
		tscDir = filepath.Join(dir, "web")
	}

	if tscDir != "" {
		var buf bytes.Buffer
		cmd := exec.CommandContext(ctx, "npx", "tsc", "--noEmit")
		cmd.Dir = tscDir
		cmd.Stdout = &buf
		cmd.Stderr = &buf
		if err := cmd.Run(); err != nil {
			result.Passed = false
			output := strings.TrimSpace(buf.String())
			msg := "tsc --noEmit failed"
			if output != "" {
				msg += ": " + output
			}
			result.Errors = append(result.Errors, msg)
		}
	}

	return result
}

// fileExists reports whether a regular file exists at path.
func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
