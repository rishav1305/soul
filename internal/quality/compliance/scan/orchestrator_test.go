package scan

import (
	"os"
	"path/filepath"
	"testing"
)

func TestScanDirectory_FindsSecrets(t *testing.T) {
	dir := t.TempDir()

	// Create a file containing an AWS access key.
	content := `package main

const awsKey = "AKIAIOSFODNN7EXAMPLE"
`
	err := os.WriteFile(filepath.Join(dir, "main.go"), []byte(content), 0644)
	if err != nil {
		t.Fatalf("writing test file: %v", err)
	}

	result, err := ScanDirectory(ScanOptions{
		Directory: dir,
	})
	if err != nil {
		t.Fatalf("ScanDirectory: %v", err)
	}

	if len(result.Findings) == 0 {
		t.Fatal("expected at least one finding for AWS key, got 0")
	}

	found := false
	for _, f := range result.Findings {
		if f.Analyzer == "secret-scanner" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected finding from secret-scanner analyzer")
	}

	if result.Summary.Total != len(result.Findings) {
		t.Errorf("summary total %d != findings count %d", result.Summary.Total, len(result.Findings))
	}

	if result.Metadata.Directory == "" {
		t.Error("metadata directory should not be empty")
	}

	if len(result.Metadata.AnalyzersRun) != 5 {
		t.Errorf("expected 5 analyzers run, got %d", len(result.Metadata.AnalyzersRun))
	}
}

func TestScanDirectory_CleanDir(t *testing.T) {
	dir := t.TempDir()

	result, err := ScanDirectory(ScanOptions{
		Directory: dir,
	})
	if err != nil {
		t.Fatalf("ScanDirectory: %v", err)
	}

	if len(result.Findings) != 0 {
		t.Errorf("expected 0 findings for empty dir, got %d", len(result.Findings))
	}

	if result.Summary.Total != 0 {
		t.Errorf("expected summary total 0, got %d", result.Summary.Total)
	}
}

func TestScanDirectory_SkipsExcluded(t *testing.T) {
	dir := t.TempDir()

	// Create a subdirectory that will be excluded.
	excludeDir := filepath.Join(dir, "secrets")
	err := os.MkdirAll(excludeDir, 0755)
	if err != nil {
		t.Fatalf("creating exclude dir: %v", err)
	}

	content := `package main

const awsKey = "AKIAIOSFODNN7EXAMPLE"
`
	err = os.WriteFile(filepath.Join(excludeDir, "creds.go"), []byte(content), 0644)
	if err != nil {
		t.Fatalf("writing test file: %v", err)
	}

	result, err := ScanDirectory(ScanOptions{
		Directory: dir,
		Exclude:   []string{"secrets"},
	})
	if err != nil {
		t.Fatalf("ScanDirectory: %v", err)
	}

	for _, f := range result.Findings {
		if f.Analyzer == "secret-scanner" {
			t.Errorf("expected no secret-scanner findings from excluded path, got: %s at %s", f.Title, f.File)
		}
	}
}
