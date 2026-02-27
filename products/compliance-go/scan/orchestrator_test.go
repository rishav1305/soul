package scan

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rishav1305/soul/products/compliance-go/analyzers"
)

// TestScanDirectorySkipsDotGit verifies that files inside .git are excluded.
func TestScanDirectorySkipsDotGit(t *testing.T) {
	dir := t.TempDir()

	// Create a .git directory with a file inside it.
	gitDir := filepath.Join(dir, ".git")
	if err := os.MkdirAll(gitDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(gitDir, "HEAD"), []byte("ref: refs/heads/main\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Create a normal file that should be included.
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	files, err := ScanDirectory(dir, nil)
	if err != nil {
		t.Fatalf("ScanDirectory returned error: %v", err)
	}

	for _, f := range files {
		if strings.Contains(f.RelativePath, ".git") {
			t.Errorf("expected .git files to be excluded, but found %q", f.RelativePath)
		}
	}

	// Should find at least the main.go file
	found := false
	for _, f := range files {
		if f.RelativePath == "main.go" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected main.go to be included in scan results")
	}
}

// TestScanDirectoryCollectsFiles verifies that ScannedFile fields are populated correctly.
func TestScanDirectoryCollectsFiles(t *testing.T) {
	dir := t.TempDir()

	// Create files with different extensions.
	files := map[string]string{
		"app.ts":            "export const x = 1;",
		"config.yaml":       "key: value",
		"src/handler.go":    "package handler\n",
		"noext":             "plain file",
	}
	for name, content := range files {
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	scanned, err := ScanDirectory(dir, nil)
	if err != nil {
		t.Fatalf("ScanDirectory returned error: %v", err)
	}

	if len(scanned) == 0 {
		t.Fatal("expected files to be collected, got 0")
	}

	// Build lookup by relative path.
	byRel := make(map[string]analyzers.ScannedFile)
	for _, f := range scanned {
		byRel[f.RelativePath] = f
	}

	// Check app.ts
	if f, ok := byRel["app.ts"]; ok {
		if f.Extension != "ts" {
			t.Errorf("expected extension 'ts', got %q", f.Extension)
		}
		if f.Size != int64(len("export const x = 1;")) {
			t.Errorf("expected size %d, got %d", len("export const x = 1;"), f.Size)
		}
		if !strings.HasSuffix(f.Path, filepath.Join(dir, "app.ts")) {
			t.Errorf("expected absolute path ending with app.ts, got %q", f.Path)
		}
	} else {
		t.Error("expected app.ts in scan results")
	}

	// Check config.yaml
	if f, ok := byRel["config.yaml"]; ok {
		if f.Extension != "yaml" {
			t.Errorf("expected extension 'yaml', got %q", f.Extension)
		}
	} else {
		t.Error("expected config.yaml in scan results")
	}

	// Check nested file
	rel := filepath.Join("src", "handler.go")
	if f, ok := byRel[rel]; ok {
		if f.Extension != "go" {
			t.Errorf("expected extension 'go', got %q", f.Extension)
		}
	} else {
		t.Errorf("expected %s in scan results", rel)
	}

	// Check file without extension
	if f, ok := byRel["noext"]; ok {
		if f.Extension != "" {
			t.Errorf("expected empty extension, got %q", f.Extension)
		}
	} else {
		t.Error("expected 'noext' in scan results")
	}
}

// TestScanDirectorySkipsExcludedDirs verifies custom exclude list works.
func TestScanDirectorySkipsExcludedDirs(t *testing.T) {
	dir := t.TempDir()

	// Create node_modules and vendor directories (default excludes)
	for _, d := range []string{"node_modules", "vendor", "custom_skip"} {
		subDir := filepath.Join(dir, d)
		if err := os.MkdirAll(subDir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(subDir, "file.js"), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	// Create a normal file
	if err := os.WriteFile(filepath.Join(dir, "index.js"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	scanned, err := ScanDirectory(dir, []string{"custom_skip"})
	if err != nil {
		t.Fatalf("ScanDirectory returned error: %v", err)
	}

	for _, f := range scanned {
		if strings.Contains(f.RelativePath, "node_modules") {
			t.Error("expected node_modules to be excluded")
		}
		if strings.Contains(f.RelativePath, "vendor") {
			t.Error("expected vendor to be excluded")
		}
		if strings.Contains(f.RelativePath, "custom_skip") {
			t.Error("expected custom_skip to be excluded")
		}
	}
}

// TestScanDirectorySkipsLargeFiles verifies files > 1MB are skipped.
func TestScanDirectorySkipsLargeFiles(t *testing.T) {
	dir := t.TempDir()

	// Create a file larger than 1MB
	largeContent := make([]byte, 1024*1024+1)
	if err := os.WriteFile(filepath.Join(dir, "large.txt"), largeContent, 0o644); err != nil {
		t.Fatal(err)
	}

	// Create a small file
	if err := os.WriteFile(filepath.Join(dir, "small.txt"), []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}

	scanned, err := ScanDirectory(dir, nil)
	if err != nil {
		t.Fatalf("ScanDirectory returned error: %v", err)
	}

	for _, f := range scanned {
		if f.RelativePath == "large.txt" {
			t.Error("expected large file (>1MB) to be skipped")
		}
	}

	found := false
	for _, f := range scanned {
		if f.RelativePath == "small.txt" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected small.txt to be included")
	}
}

// TestRunScanOnVulnerableProject creates a fixture with known vulnerabilities
// and verifies findings are returned.
func TestRunScanOnVulnerableProject(t *testing.T) {
	dir := t.TempDir()

	// Create a .ts file with an AWS key and a weak hash usage (createHash('md5')).
	tsContent := "const key = \"AKIAIOSFODNN7EXAMPLE\";\nconst hash = createHash('md5');\n"
	if err := os.WriteFile(filepath.Join(dir, "app.ts"), []byte(tsContent), 0o644); err != nil {
		t.Fatal(err)
	}

	// Create a .gitignore that is missing .env entries.
	gitignoreContent := "node_modules/\ndist/\n"
	if err := os.WriteFile(filepath.Join(dir, ".gitignore"), []byte(gitignoreContent), 0o644); err != nil {
		t.Fatal(err)
	}

	// Create a .env file (should trigger env-not-gitignored)
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte("DB_URL=postgres://localhost/db"), 0o644); err != nil {
		t.Fatal(err)
	}

	result, err := RunScan(ScanOptions{
		Directory: dir,
	})
	if err != nil {
		t.Fatalf("RunScan returned error: %v", err)
	}

	if result == nil {
		t.Fatal("expected non-nil ScanResult")
	}

	if result.Summary.Total == 0 {
		t.Fatal("expected findings from vulnerable project, got 0")
	}

	// Verify we found the AWS key (secret-scanner finding)
	foundSecret := false
	for _, f := range result.Findings {
		if f.Analyzer == "secret-scanner" && strings.Contains(f.Evidence, "AKIA") {
			foundSecret = true
			break
		}
	}
	if !foundSecret {
		t.Error("expected secret-scanner to detect AWS key")
	}

	// Verify the weak hash was detected (ast-analyzer finding)
	foundWeakHash := false
	for _, f := range result.Findings {
		if f.Analyzer == "ast-analyzer" && strings.Contains(f.Evidence, "createHash") {
			foundWeakHash = true
			break
		}
	}
	if !foundWeakHash {
		t.Error("expected ast-analyzer to detect weak hash usage (createHash('md5'))")
	}

	// Verify metadata is populated.
	if result.Metadata.Directory != dir {
		t.Errorf("expected metadata directory %q, got %q", dir, result.Metadata.Directory)
	}
	if result.Metadata.Duration <= 0 {
		t.Error("expected positive duration")
	}
	if len(result.Metadata.AnalyzersRun) == 0 {
		t.Error("expected analyzers_run to be populated")
	}
	if result.Metadata.Timestamp == "" {
		t.Error("expected timestamp to be set")
	}

	// Verify summary has severity counts.
	if len(result.Summary.BySeverity) == 0 {
		t.Error("expected by_severity map to have entries")
	}
}

// TestRunScanFrameworkFilter verifies that framework filtering limits findings.
func TestRunScanFrameworkFilter(t *testing.T) {
	dir := t.TempDir()

	// Create a file with a secret that triggers multi-framework rules.
	tsContent := "const key = \"AKIAIOSFODNN7EXAMPLE\";"
	if err := os.WriteFile(filepath.Join(dir, "app.ts"), []byte(tsContent), 0o644); err != nil {
		t.Fatal(err)
	}

	// Run with SOC2 filter only.
	result, err := RunScan(ScanOptions{
		Directory:  dir,
		Frameworks: []string{"SOC2"},
	})
	if err != nil {
		t.Fatalf("RunScan returned error: %v", err)
	}

	// All findings should have SOC2 in their framework list.
	for _, f := range result.Findings {
		hasSoc2 := false
		for _, fw := range f.Framework {
			if strings.EqualFold(fw, "SOC2") {
				hasSoc2 = true
				break
			}
		}
		if !hasSoc2 {
			t.Errorf("expected all findings to have SOC2 framework, but finding %q has %v",
				f.ID, f.Framework)
		}
	}

	// Verify metadata reflects the framework filter.
	if len(result.Metadata.Frameworks) != 1 || !strings.EqualFold(result.Metadata.Frameworks[0], "SOC2") {
		t.Errorf("expected metadata frameworks [SOC2], got %v", result.Metadata.Frameworks)
	}
}

// TestRunScanAnalyzerFilter verifies that analyzer filtering limits which analyzers run.
func TestRunScanAnalyzerFilter(t *testing.T) {
	dir := t.TempDir()

	// Create a file that would trigger both secret-scanner and ast-analyzer.
	tsContent := "const key = \"AKIAIOSFODNN7EXAMPLE\";\nconst hash = createHash('md5');\n"
	if err := os.WriteFile(filepath.Join(dir, "app.ts"), []byte(tsContent), 0o644); err != nil {
		t.Fatal(err)
	}

	// Run only secret-scanner.
	result, err := RunScan(ScanOptions{
		Directory: dir,
		Analyzers: []string{"secret-scanner"},
	})
	if err != nil {
		t.Fatalf("RunScan returned error: %v", err)
	}

	// Verify only secret-scanner findings are present.
	for _, f := range result.Findings {
		if f.Analyzer != "secret-scanner" {
			t.Errorf("expected only secret-scanner findings, got analyzer %q", f.Analyzer)
		}
	}

	// Verify metadata shows only secret-scanner was run.
	if len(result.Metadata.AnalyzersRun) != 1 || result.Metadata.AnalyzersRun[0] != "secret-scanner" {
		t.Errorf("expected analyzers_run [secret-scanner], got %v", result.Metadata.AnalyzersRun)
	}
}

// TestRunScanDeduplication verifies that duplicate findings (same file:line:id) are removed.
func TestRunScanDeduplication(t *testing.T) {
	dir := t.TempDir()

	// Create a file that might trigger multiple rules for the same pattern.
	tsContent := "const key = \"AKIAIOSFODNN7EXAMPLE\";"
	if err := os.WriteFile(filepath.Join(dir, "app.ts"), []byte(tsContent), 0o644); err != nil {
		t.Fatal(err)
	}

	result, err := RunScan(ScanOptions{
		Directory: dir,
	})
	if err != nil {
		t.Fatalf("RunScan returned error: %v", err)
	}

	// Verify no duplicate file:line:id combinations.
	seen := make(map[string]bool)
	for _, f := range result.Findings {
		key := f.File + ":" + string(rune(f.Line)) + ":" + f.ID
		if seen[key] {
			t.Errorf("duplicate finding: file=%s line=%d id=%s", f.File, f.Line, f.ID)
		}
		seen[key] = true
	}
}

// TestRunScanSeverityFilter verifies that severity filtering works.
func TestRunScanSeverityFilter(t *testing.T) {
	dir := t.TempDir()

	// Create a file with a high-severity secret.
	tsContent := "const key = \"AKIAIOSFODNN7EXAMPLE\";\n"
	if err := os.WriteFile(filepath.Join(dir, "app.ts"), []byte(tsContent), 0o644); err != nil {
		t.Fatal(err)
	}

	// Run with only critical severity filter.
	result, err := RunScan(ScanOptions{
		Directory: dir,
		Severity:  []string{"critical"},
	})
	if err != nil {
		t.Fatalf("RunScan returned error: %v", err)
	}

	for _, f := range result.Findings {
		if !strings.EqualFold(f.Severity, "critical") {
			t.Errorf("expected only critical findings, got severity %q for %q", f.Severity, f.ID)
		}
	}
}
