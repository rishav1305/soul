package analyzers

import (
	"testing"

	"github.com/rishav1305/soul/products/compliance/rules"
)

func TestDepAuditorUnpinnedDeps(t *testing.T) {
	dir := t.TempDir()

	pkgJSON := tempFile(t, dir, "package.json", `{
  "name": "test-app",
  "dependencies": {
    "express": "^4.18.0",
    "lodash": "~4.17.21",
    "uuid": "*"
  },
  "devDependencies": {
    "jest": "^29.0.0"
  }
}`)

	allRules := rules.Load(nil)
	auditor := &DepAuditor{}
	findings, err := auditor.Analyze([]ScannedFile{pkgJSON}, allRules)
	if err != nil {
		t.Fatalf("Analyze returned error: %v", err)
	}

	// Should find unpinned-deps for express (^), lodash (~), uuid (*), jest (^)
	unpinnedCount := 0
	for _, f := range findings {
		if f.ID == "CHANGE-003" {
			unpinnedCount++
		}
	}

	if unpinnedCount < 4 {
		t.Errorf("expected at least 4 unpinned-deps findings, got %d", unpinnedCount)
	}

	// Should also find missing-engines
	foundMissingEngines := false
	for _, f := range findings {
		if f.ID == "VENDOR-004" {
			foundMissingEngines = true
			break
		}
	}
	if !foundMissingEngines {
		t.Error("expected VENDOR-004 (missing-engines) finding, not found")
	}

	// Should find missing-lockfile (no lockfile in files list)
	foundMissingLockfile := false
	for _, f := range findings {
		if f.ID == "CHANGE-002" {
			foundMissingLockfile = true
			break
		}
	}
	if !foundMissingLockfile {
		t.Error("expected CHANGE-002 (missing-lockfile) finding, not found")
	}

	// Verify analyzer field
	for _, f := range findings {
		if f.Analyzer != "dep-auditor" {
			t.Errorf("expected analyzer 'dep-auditor', got %q", f.Analyzer)
		}
	}
}

func TestDepAuditorCompliant(t *testing.T) {
	dir := t.TempDir()

	pkgJSON := tempFile(t, dir, "package.json", `{
  "name": "compliant-app",
  "engines": {
    "node": ">=18.0.0"
  },
  "dependencies": {
    "express": "4.18.2",
    "lodash": "4.17.21"
  },
  "devDependencies": {
    "jest": "29.7.0"
  }
}`)

	lockfile := tempFile(t, dir, "package-lock.json", `{"lockfileVersion": 3}`)

	allRules := rules.Load(nil)
	auditor := &DepAuditor{}
	findings, err := auditor.Analyze([]ScannedFile{pkgJSON, lockfile}, allRules)
	if err != nil {
		t.Fatalf("Analyze returned error: %v", err)
	}

	// Should NOT find unpinned-deps, missing-engines, or missing-lockfile
	unwantedIDs := map[string]bool{
		"CHANGE-002": true, // missing-lockfile
		"CHANGE-003": true, // unpinned-deps
		"VENDOR-004": true, // missing-engines
	}

	for _, f := range findings {
		if unwantedIDs[f.ID] {
			t.Errorf("did not expect finding %s (%s) for compliant package.json", f.ID, f.Title)
		}
	}
}

func TestDepAuditorCopyleftLicense(t *testing.T) {
	dir := t.TempDir()

	pkgJSON := tempFile(t, dir, "package.json", `{
  "name": "gpl-app",
  "license": "GPL-3.0",
  "engines": { "node": ">=18" },
  "dependencies": {
    "express": "4.18.2"
  }
}`)

	lockfile := tempFile(t, dir, "package-lock.json", `{"lockfileVersion": 3}`)

	allRules := rules.Load(nil)
	auditor := &DepAuditor{}
	findings, err := auditor.Analyze([]ScannedFile{pkgJSON, lockfile}, allRules)
	if err != nil {
		t.Fatalf("Analyze returned error: %v", err)
	}

	foundCopyleft := false
	for _, f := range findings {
		if f.ID == "VENDOR-002" {
			foundCopyleft = true
			break
		}
	}
	if !foundCopyleft {
		t.Error("expected VENDOR-002 (copyleft-license) finding, not found")
	}
}

func TestDepAuditorNoPackageJSON(t *testing.T) {
	dir := t.TempDir()

	// No package.json at all, just a random file
	sf := tempFile(t, dir, "main.go", `package main`)

	allRules := rules.Load(nil)
	auditor := &DepAuditor{}
	findings, err := auditor.Analyze([]ScannedFile{sf}, allRules)
	if err != nil {
		t.Fatalf("Analyze returned error: %v", err)
	}

	// Should have no findings since there's no package.json
	if len(findings) != 0 {
		t.Errorf("expected 0 findings when no package.json exists, got %d", len(findings))
	}
}

func TestIsUnpinnedVersion(t *testing.T) {
	tests := []struct {
		version  string
		expected bool
	}{
		{"^4.18.0", true},
		{"~4.17.21", true},
		{"*", true},
		{"4.18.2", false},
		{"1.0.0", false},
		{"", false},
	}

	for _, tt := range tests {
		got := isUnpinnedVersion(tt.version)
		if got != tt.expected {
			t.Errorf("isUnpinnedVersion(%q) = %v, want %v", tt.version, got, tt.expected)
		}
	}
}

func TestIsCopyleftLicense(t *testing.T) {
	tests := []struct {
		license  string
		expected bool
	}{
		{"GPL-3.0", true},
		{"AGPL-3.0-only", true},
		{"LGPL-2.1", true},
		{"MIT", false},
		{"Apache-2.0", false},
		{"ISC", false},
		{"", false},
	}

	for _, tt := range tests {
		got := isCopyleftLicense(tt.license)
		if got != tt.expected {
			t.Errorf("isCopyleftLicense(%q) = %v, want %v", tt.license, got, tt.expected)
		}
	}
}
