package fix

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rishav1305/soul/products/compliance-go/analyzers"
)

// TestApplyFixesDryRun creates a temp file with an AWS key, runs a dry-run fix,
// and verifies a patch is generated but the file remains unchanged.
func TestApplyFixesDryRun(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "config.ts")

	original := `const awsKey = "AKIAIOSFODNN7EXAMPLE";
const region = "us-east-1";
`
	if err := os.WriteFile(filePath, []byte(original), 0644); err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}

	findings := []analyzers.Finding{
		{
			ID:       "SECRET-001",
			Title:    "Hardcoded credentials detected",
			Severity: "critical",
			File:     filePath,
			Line:     1,
			Evidence: "AKIA****MPLE",
			Analyzer: "secret-scanner",
			Fixable:  true,
		},
	}

	results, err := ApplyFixes(findings, true)
	if err != nil {
		t.Fatalf("ApplyFixes returned error: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	r := results[0]
	if !r.Fixed {
		t.Error("expected Fixed=true for dry-run patch generation")
	}
	if r.Patch == "" {
		t.Error("expected non-empty patch in dry-run mode")
	}
	if r.Strategy != "secret-to-env" {
		t.Errorf("expected strategy 'secret-to-env', got %q", r.Strategy)
	}

	// Verify file is unchanged
	content, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}
	if string(content) != original {
		t.Error("file was modified during dry-run; expected no changes")
	}
}

// TestApplyFixesActual creates a temp file, applies a fix, and verifies
// the file content is changed.
func TestApplyFixesActual(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "config.ts")

	original := `const awsKey = "AKIAIOSFODNN7EXAMPLE";
const region = "us-east-1";
`
	if err := os.WriteFile(filePath, []byte(original), 0644); err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}

	findings := []analyzers.Finding{
		{
			ID:       "SECRET-001",
			Title:    "Hardcoded credentials detected",
			Severity: "critical",
			File:     filePath,
			Line:     1,
			Evidence: "AKIA****MPLE",
			Analyzer: "secret-scanner",
			Fixable:  true,
		},
	}

	results, err := ApplyFixes(findings, false)
	if err != nil {
		t.Fatalf("ApplyFixes returned error: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	r := results[0]
	if !r.Fixed {
		t.Error("expected Fixed=true")
	}
	if r.Patch == "" {
		t.Error("expected non-empty patch")
	}

	// Verify file was changed
	content, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}
	if string(content) == original {
		t.Error("file was not modified; expected changes to be applied")
	}
	if !strings.Contains(string(content), "process.env.") {
		t.Error("expected file to contain process.env. reference after fix")
	}
}

// TestNonFixableFindingsSkipped passes non-fixable findings and verifies
// no results are returned.
func TestNonFixableFindingsSkipped(t *testing.T) {
	findings := []analyzers.Finding{
		{
			ID:       "AUTH-001",
			Title:    "Missing authentication middleware",
			Severity: "high",
			File:     "routes.ts",
			Line:     10,
			Analyzer: "ast-analyzer",
			Fixable:  false,
		},
		{
			ID:       "LOG-001",
			Title:    "No logging framework detected",
			Severity: "medium",
			File:     "app.ts",
			Line:     1,
			Analyzer: "ast-analyzer",
			Fixable:  false,
		},
	}

	results, err := ApplyFixes(findings, true)
	if err != nil {
		t.Fatalf("ApplyFixes returned error: %v", err)
	}

	if len(results) != 0 {
		t.Errorf("expected 0 results for non-fixable findings, got %d", len(results))
	}
}

// TestWeakHashStrategy verifies weak hash replacement strategy.
func TestWeakHashStrategy(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "crypto.ts")

	original := "const hash = crypto.createHash('md5').update(data).digest('hex');\n"
	if err := os.WriteFile(filePath, []byte(original), 0644); err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}

	findings := []analyzers.Finding{
		{
			ID:       "CRYPTO-001",
			Title:    "Weak hashing algorithm (MD5/SHA1)",
			Severity: "critical",
			File:     filePath,
			Line:     1,
			Evidence: "createHash('md5')",
			Analyzer: "ast-analyzer",
			Fixable:  true,
		},
	}

	results, err := ApplyFixes(findings, false)
	if err != nil {
		t.Fatalf("ApplyFixes returned error: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	if results[0].Strategy != "weak-hash-upgrade" {
		t.Errorf("expected strategy 'weak-hash-upgrade', got %q", results[0].Strategy)
	}

	content, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}
	if !strings.Contains(string(content), "sha256") {
		t.Error("expected sha256 after weak hash fix")
	}
}

// TestCORSStrategy verifies CORS wildcard replacement strategy.
func TestCORSStrategy(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "server.ts")

	original := "app.use(cors({ origin: '*' }));\n"
	if err := os.WriteFile(filePath, []byte(original), 0644); err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}

	findings := []analyzers.Finding{
		{
			ID:       "CRYPTO-008",
			Title:    "Wildcard CORS origin",
			Severity: "high",
			File:     filePath,
			Line:     1,
			Evidence: "origin: '*'",
			Analyzer: "config-checker",
			Fixable:  true,
		},
	}

	results, err := ApplyFixes(findings, false)
	if err != nil {
		t.Fatalf("ApplyFixes returned error: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	if results[0].Strategy != "cors-restrict" {
		t.Errorf("expected strategy 'cors-restrict', got %q", results[0].Strategy)
	}

	content, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}
	if strings.Contains(string(content), "'*'") {
		t.Error("expected wildcard to be replaced after CORS fix")
	}
}
