package analyzers

import (
	"testing"

	"github.com/rishav1305/soul/products/compliance/rules"
)

func TestGitAnalyzerMissingFiles(t *testing.T) {
	dir := t.TempDir()

	// Only a .gitignore with minimal content — no CODEOWNERS, SECURITY.md, LICENSE
	gitignore := tempFile(t, dir, ".gitignore", "node_modules/\n*.log\n")

	allRules := rules.Load(nil)
	analyzer := &GitAnalyzer{}
	findings, err := analyzer.Analyze([]ScannedFile{gitignore}, allRules)
	if err != nil {
		t.Fatalf("Analyze returned error: %v", err)
	}

	expectedIDs := map[string]bool{
		"ACCESS-001": false, // missing CODEOWNERS
		"VENDOR-006": false, // missing SECURITY.md
		"VENDOR-007": false, // missing LICENSE
	}

	for _, f := range findings {
		if _, ok := expectedIDs[f.ID]; ok {
			expectedIDs[f.ID] = true
		}
	}

	for id, found := range expectedIDs {
		if !found {
			t.Errorf("expected finding %s, not found in results", id)
		}
	}

	// Verify analyzer field
	for _, f := range findings {
		if f.Analyzer != "git-analyzer" {
			t.Errorf("expected analyzer 'git-analyzer', got %q", f.Analyzer)
		}
	}
}

func TestGitAnalyzerAllFilesPresent(t *testing.T) {
	dir := t.TempDir()

	gitignore := tempFile(t, dir, ".gitignore", ".env\nnode_modules/\n*.pem\n*.key\n*.p12\n*.pfx\ncredentials.json\n")
	codeowners := tempFile(t, dir, "CODEOWNERS", "* @team-lead\n")
	security := tempFile(t, dir, "SECURITY.md", "# Security Policy\n")
	license := tempFile(t, dir, "LICENSE", "MIT License\n")

	files := []ScannedFile{gitignore, codeowners, security, license}

	allRules := rules.Load(nil)
	analyzer := &GitAnalyzer{}
	findings, err := analyzer.Analyze(files, allRules)
	if err != nil {
		t.Fatalf("Analyze returned error: %v", err)
	}

	// Should NOT have missing-codeowners, no-security-policy, or missing-license
	unwantedIDs := map[string]bool{
		"ACCESS-001": true,
		"VENDOR-006": true,
		"VENDOR-007": true,
	}

	for _, f := range findings {
		if unwantedIDs[f.ID] {
			t.Errorf("did not expect finding %s (%s) when all files are present", f.ID, f.Title)
		}
	}
}

func TestGitAnalyzerMissingGitignore(t *testing.T) {
	dir := t.TempDir()

	// No .gitignore at all
	readme := tempFile(t, dir, "README.md", "# Project\n")

	allRules := rules.Load(nil)
	analyzer := &GitAnalyzer{}
	findings, err := analyzer.Analyze([]ScannedFile{readme}, allRules)
	if err != nil {
		t.Fatalf("Analyze returned error: %v", err)
	}

	foundIncompleteGitignore := false
	for _, f := range findings {
		if f.ID == "CHANGE-007" {
			foundIncompleteGitignore = true
			break
		}
	}

	if !foundIncompleteGitignore {
		t.Error("expected CHANGE-007 (incomplete-gitignore) when .gitignore is missing")
	}
}

func TestGitAnalyzerGitignoreMissingEnv(t *testing.T) {
	dir := t.TempDir()

	// .gitignore without .env entry
	gitignore := tempFile(t, dir, ".gitignore", "node_modules/\n*.log\n")

	allRules := rules.Load(nil)
	analyzer := &GitAnalyzer{}
	findings, err := analyzer.Analyze([]ScannedFile{gitignore}, allRules)
	if err != nil {
		t.Fatalf("Analyze returned error: %v", err)
	}

	foundEnvMissing := false
	for _, f := range findings {
		if f.ID == "ACCESS-002" {
			foundEnvMissing = true
			break
		}
	}

	if !foundEnvMissing {
		t.Error("expected ACCESS-002 (env-not-gitignored) when .env not in .gitignore")
	}
}
