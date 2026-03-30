package scan

import (
	"os"
	"path/filepath"
	"testing"
)

func TestScanDirectory_EmptyDirectory(t *testing.T) {
	_, err := ScanDirectory(ScanOptions{Directory: ""})
	if err == nil {
		t.Error("expected error for empty directory")
	}
}


func TestScanDirectory_FilterAnalyzers(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "main.go"), []byte(`package main
const key = "AKIAIOSFODNN7EXAMPLE"
`), 0644)

	result, err := ScanDirectory(ScanOptions{
		Directory: dir,
		Analyzers: []string{"secret-scanner"},
	})
	if err != nil {
		t.Fatal(err)
	}

	// Only secret-scanner should run.
	if len(result.Metadata.AnalyzersRun) != 1 {
		t.Errorf("expected 1 analyzer, got %d: %v", len(result.Metadata.AnalyzersRun), result.Metadata.AnalyzersRun)
	}
	if result.Metadata.AnalyzersRun[0] != "secret-scanner" {
		t.Errorf("expected secret-scanner, got %q", result.Metadata.AnalyzersRun[0])
	}
}

func TestScanDirectory_SeverityFilter(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "main.go"), []byte(`package main
const key = "AKIAIOSFODNN7EXAMPLE"
`), 0644)

	result, err := ScanDirectory(ScanOptions{
		Directory: dir,
		Severity:  []string{"critical"},
	})
	if err != nil {
		t.Fatal(err)
	}

	for _, f := range result.Findings {
		if f.Severity != "critical" {
			t.Errorf("expected only critical findings, got severity %q for %q", f.Severity, f.Title)
		}
	}
}

func TestScanDirectory_SeverityFilter_NoMatch(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "main.go"), []byte(`package main
const key = "AKIAIOSFODNN7EXAMPLE"
`), 0644)

	result, err := ScanDirectory(ScanOptions{
		Directory: dir,
		Severity:  []string{"info"}, // unlikely to match secret findings
	})
	if err != nil {
		t.Fatal(err)
	}

	// May or may not find matches — just ensure no panic.
	_ = result
}

func TestScanDirectory_LargeFileSkipped(t *testing.T) {
	dir := t.TempDir()

	// Create a file larger than 1MB.
	bigContent := make([]byte, maxScanFileSize+1)
	for i := range bigContent {
		bigContent[i] = 'A'
	}
	os.WriteFile(filepath.Join(dir, "big.txt"), bigContent, 0644)

	result, err := ScanDirectory(ScanOptions{Directory: dir})
	if err != nil {
		t.Fatal(err)
	}

	// Big file should be skipped — no findings from it.
	for _, f := range result.Findings {
		if filepath.Base(f.File) == "big.txt" {
			t.Error("expected big.txt to be skipped")
		}
	}
}

func TestScanDirectory_SkipsDotGit(t *testing.T) {
	dir := t.TempDir()

	gitDir := filepath.Join(dir, ".git")
	os.MkdirAll(gitDir, 0755)
	os.WriteFile(filepath.Join(gitDir, "config"), []byte(`secret = "AKIAIOSFODNN7EXAMPLE"`), 0644)

	result, err := ScanDirectory(ScanOptions{Directory: dir})
	if err != nil {
		t.Fatal(err)
	}

	for _, f := range result.Findings {
		if filepath.Dir(f.File) == gitDir {
			t.Error("expected .git directory to be skipped")
		}
	}
}

func TestScanDirectory_Frameworks(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n"), 0644)

	// Scan with specific frameworks.
	result, err := ScanDirectory(ScanOptions{
		Directory:  dir,
		Frameworks: []string{"soc2"},
	})
	if err != nil {
		t.Fatal(err)
	}

	// Should complete without error.
	if result.Metadata.Directory == "" {
		t.Error("expected metadata directory")
	}
}

func TestScanDirectory_ByFrameworkSummary(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "main.go"), []byte(`package main
const key = "AKIAIOSFODNN7EXAMPLE"
`), 0644)

	result, err := ScanDirectory(ScanOptions{Directory: dir})
	if err != nil {
		t.Fatal(err)
	}

	if result.Summary.BySeverity == nil {
		t.Error("BySeverity should not be nil")
	}
	if result.Summary.ByAnalyzer == nil {
		t.Error("ByAnalyzer should not be nil")
	}
	if result.Summary.ByFramework == nil {
		t.Error("ByFramework should not be nil")
	}
}

func TestScanDirectory_FixableSummary(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "main.go"), []byte(`package main
const key = "AKIAIOSFODNN7EXAMPLE"
`), 0644)

	result, err := ScanDirectory(ScanOptions{Directory: dir})
	if err != nil {
		t.Fatal(err)
	}

	// Check that fixable count is counted.
	fixableCount := 0
	for _, f := range result.Findings {
		if f.Fixable {
			fixableCount++
		}
	}
	if result.Summary.Fixable != fixableCount {
		t.Errorf("Summary.Fixable = %d, expected %d", result.Summary.Fixable, fixableCount)
	}
}
