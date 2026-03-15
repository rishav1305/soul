package analyzers

import (
	"os"
	"path/filepath"
	"testing"
)

func writeTestFile(t *testing.T, dir, name, content string) ScannedFile {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	ext := filepath.Ext(name)
	if len(ext) > 0 {
		ext = ext[1:] // strip leading dot
	}
	info, _ := os.Stat(path)
	return ScannedFile{
		Path:         path,
		RelativePath: name,
		Extension:    ext,
		Size:         info.Size(),
	}
}

func TestSecretScanner_AWSKey(t *testing.T) {
	dir := t.TempDir()
	f := writeTestFile(t, dir, "config.go", `package config
const accessKey = "AKIAIOSFODNN7EXAMPLE"
`)
	scanner := &SecretScanner{}
	findings, err := scanner.Analyze([]ScannedFile{f}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(findings) == 0 {
		t.Fatal("expected finding for AWS access key, got none")
	}
	found := false
	for _, f := range findings {
		if f.Title == "AWS Access Key" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected AWS Access Key finding")
	}
}

func TestSecretScanner_GitHubToken(t *testing.T) {
	dir := t.TempDir()
	f := writeTestFile(t, dir, "deploy.yaml", `token: ghp_ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmn
`)
	scanner := &SecretScanner{}
	findings, err := scanner.Analyze([]ScannedFile{f}, nil)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, f := range findings {
		if f.Title == "GitHub Token" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected GitHub Token finding, got %d findings: %+v", len(findings), findings)
	}
}

func TestSecretScanner_PrivateKey(t *testing.T) {
	dir := t.TempDir()
	f := writeTestFile(t, dir, "key.conf", `-----BEGIN RSA PRIVATE KEY-----
MIIEpAIBAAKCAQEA0Z3VS5JJcds3xfn/ygWyF068wMEbp
-----END RSA PRIVATE KEY-----
`)
	scanner := &SecretScanner{}
	findings, err := scanner.Analyze([]ScannedFile{f}, nil)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, f := range findings {
		if f.Title == "Private Key" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected Private Key finding, got %d findings: %+v", len(findings), findings)
	}
}

func TestSecretScanner_NoFalsePositive(t *testing.T) {
	dir := t.TempDir()
	f := writeTestFile(t, dir, "clean.go", `package main

import "fmt"

func main() {
	fmt.Println("Hello, world!")
	x := 42
	name := "alice"
	fmt.Printf("User %s has ID %d\n", name, x)
}
`)
	scanner := &SecretScanner{}
	findings, err := scanner.Analyze([]ScannedFile{f}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(findings) != 0 {
		t.Errorf("expected no findings for clean file, got %d: %+v", len(findings), findings)
	}
}

func TestSecretScanner_Redaction(t *testing.T) {
	dir := t.TempDir()
	f := writeTestFile(t, dir, "creds.env", `API_KEY="sk-ant-super-secret-key-12345"
`)
	scanner := &SecretScanner{}
	rules := []Rule{
		{
			ID:       "SEC-001",
			Analyzer: "secret-scanner",
			Pattern:  "api-token",
			Severity: "critical",
			Title:    "API Token Detected",
		},
	}
	findings, err := scanner.Analyze([]ScannedFile{f}, rules)
	if err != nil {
		t.Fatal(err)
	}
	if len(findings) == 0 {
		t.Fatal("expected findings, got none")
	}
	for _, f := range findings {
		if f.Evidence == "" {
			t.Error("evidence should not be empty")
			continue
		}
		// Evidence must be redacted — should not contain the full secret.
		if f.Evidence == "sk-ant-super-secret-key-12345" {
			t.Error("evidence should be redacted, but found full secret")
		}
		// Redacted form should contain "****".
		if len(f.Evidence) > 8 && !contains(f.Evidence, "****") {
			t.Errorf("expected redacted evidence to contain ****, got %q", f.Evidence)
		}
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchSubstr(s, substr)
}

func searchSubstr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestShannonEntropy(t *testing.T) {
	// High entropy — random-looking string.
	highEntropy := "aB3$xK9!mZ7@qR5&wL2#"
	he := shannonEntropy(highEntropy)
	if he < 4.0 {
		t.Errorf("expected high entropy > 4.0, got %.2f for %q", he, highEntropy)
	}

	// Low entropy — repeated character.
	lowEntropy := "aaaaaaaaaa"
	le := shannonEntropy(lowEntropy)
	if le >= 1.0 {
		t.Errorf("expected low entropy < 1.0, got %.2f for %q", le, lowEntropy)
	}

	// Zero entropy — empty string.
	ze := shannonEntropy("")
	if ze != 0 {
		t.Errorf("expected zero entropy for empty string, got %.2f", ze)
	}
}

func TestRedact(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"AKIAIOSFODNN7EXAMPLE", "AKIA****MPLE"},
		{"short", "*****"},
		{"12345678", "1234****5678"},
		{"abc", "***"},
	}
	for _, tt := range tests {
		got := redact(tt.input)
		if got != tt.expected {
			t.Errorf("redact(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func TestSecretScanner_Name(t *testing.T) {
	s := &SecretScanner{}
	if s.Name() != "secret-scanner" {
		t.Errorf("expected name 'secret-scanner', got %q", s.Name())
	}
}

func TestSecretScanner_SkipsNonTextFiles(t *testing.T) {
	dir := t.TempDir()
	f := writeTestFile(t, dir, "binary.exe", `AKIAIOSFODNN7EXAMPLE`)
	// Extension "exe" is not in textExtensions, so it should be skipped.
	scanner := &SecretScanner{}
	findings, err := scanner.Analyze([]ScannedFile{f}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(findings) != 0 {
		t.Errorf("expected no findings for non-text file, got %d", len(findings))
	}
}

func TestSecretScanner_WithRules(t *testing.T) {
	dir := t.TempDir()
	f := writeTestFile(t, dir, "config.yaml", `aws_secret = "ABCDEFGHIJKLMNOPQRSTUVWXYZ01234567890123"
`)
	rules := []Rule{
		{
			ID:          "SEC-002",
			Analyzer:    "secret-scanner",
			Pattern:     "hardcoded-credential",
			Severity:    "critical",
			Title:       "Hardcoded Credential",
			Description: "A hardcoded credential was detected",
			Framework:   []string{"SOC2"},
			Controls:    []string{"CC6.1"},
			Fixable:     true,
		},
	}
	scanner := &SecretScanner{}
	findings, err := scanner.Analyze([]ScannedFile{f}, rules)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, f := range findings {
		if f.ID == "SEC-002" && f.Severity == "critical" {
			found = true
			if f.Description != "A hardcoded credential was detected" {
				t.Errorf("unexpected description: %q", f.Description)
			}
			if !f.Fixable {
				t.Error("expected fixable to be true")
			}
			break
		}
	}
	if !found {
		t.Errorf("expected finding with rule SEC-002, got: %+v", findings)
	}
}
