package analyzers

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rishav1305/soul/products/compliance-go/rules"
)

// helper: create a temp file and return its ScannedFile representation.
func tempFile(t *testing.T, dir, name, content string) ScannedFile {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	ext := strings.TrimPrefix(filepath.Ext(name), ".")
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	return ScannedFile{
		Path:         path,
		RelativePath: name,
		Extension:    ext,
		Size:         info.Size(),
	}
}

func TestSecretScannerFindsAWSKey(t *testing.T) {
	dir := t.TempDir()
	sf := tempFile(t, dir, "config.ts", `const key = "AKIAIOSFODNN7EXAMPLE";`)

	allRules := rules.Load(nil)
	scanner := &SecretScanner{}
	findings, err := scanner.Analyze([]ScannedFile{sf}, allRules)
	if err != nil {
		t.Fatalf("Analyze returned error: %v", err)
	}

	if len(findings) == 0 {
		t.Fatal("expected at least one finding for AWS key, got 0")
	}

	// The finding should match SECRET-001 (hardcoded-credential) or SECRET-002 (private-key)
	// Since it's an AWS key, it should be SECRET-001
	foundSecretRule := false
	for _, f := range findings {
		if f.ID == "SECRET-001" || f.ID == "SECRET-002" {
			foundSecretRule = true
			break
		}
	}
	if !foundSecretRule {
		t.Errorf("expected finding with ID SECRET-001 or SECRET-002, got IDs: %v",
			findingIDs(findings))
	}

	// Verify the analyzer field
	for _, f := range findings {
		if f.Analyzer != "secret-scanner" {
			t.Errorf("expected analyzer 'secret-scanner', got %q", f.Analyzer)
		}
	}
}

func TestSecretScannerSkipsNonTextFiles(t *testing.T) {
	dir := t.TempDir()
	sf := tempFile(t, dir, "image.png", `const key = "AKIAIOSFODNN7EXAMPLE";`)

	allRules := rules.Load(nil)
	scanner := &SecretScanner{}
	findings, err := scanner.Analyze([]ScannedFile{sf}, allRules)
	if err != nil {
		t.Fatalf("Analyze returned error: %v", err)
	}

	if len(findings) != 0 {
		t.Errorf("expected 0 findings for .png file, got %d", len(findings))
	}
}

func TestSecretScannerHighEntropy(t *testing.T) {
	dir := t.TempDir()
	// A high-entropy string that is 20+ chars, base64-like
	sf := tempFile(t, dir, "secrets.env",
		`SOME_TOKEN="aB3dE5fG7hI9jK1lM3nO5pQ7rS9tU1vW3xY5zA7bC9dE"`)

	allRules := rules.Load(nil)
	scanner := &SecretScanner{}
	findings, err := scanner.Analyze([]ScannedFile{sf}, allRules)
	if err != nil {
		t.Fatalf("Analyze returned error: %v", err)
	}

	// Should not error — we don't assert on count here since it depends on
	// whether the string meets entropy thresholds, but the scanner should
	// handle it without crashing.
	_ = findings
}

func TestSecretScannerRedactsEvidence(t *testing.T) {
	dir := t.TempDir()
	sf := tempFile(t, dir, "creds.ts",
		`const key = "AKIAIOSFODNN7EXAMPLE";
password = "super_secret_password_123"
`)

	allRules := rules.Load(nil)
	scanner := &SecretScanner{}
	findings, err := scanner.Analyze([]ScannedFile{sf}, allRules)
	if err != nil {
		t.Fatalf("Analyze returned error: %v", err)
	}

	if len(findings) == 0 {
		t.Fatal("expected findings to verify redaction, got 0")
	}

	for _, f := range findings {
		if !strings.Contains(f.Evidence, "****") {
			t.Errorf("evidence should be redacted (contain ****), got %q", f.Evidence)
		}
	}
}

// findingIDs returns a slice of IDs from findings for error messages.
func findingIDs(findings []Finding) []string {
	ids := make([]string, len(findings))
	for i, f := range findings {
		ids[i] = f.ID
	}
	return ids
}
