package analyzers

import (
	"testing"
)

func depsRules() []Rule {
	return []Rule{
		{ID: "DEP-001", Title: "Unpinned deps", Severity: "medium", Analyzer: "deps", Pattern: "unpinned-deps"},
		{ID: "DEP-002", Title: "Missing lockfile", Severity: "high", Analyzer: "deps", Pattern: "missing-lockfile"},
		{ID: "DEP-003", Title: "Missing engines", Severity: "low", Analyzer: "deps", Pattern: "missing-engines"},
		{ID: "DEP-004", Title: "Copyleft license", Severity: "high", Analyzer: "deps", Pattern: "copyleft-license"},
	}
}

func TestDepAuditor_UnpinnedDeps(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "package.json", `{
		"dependencies": {
			"express": "^4.18.0",
			"lodash": "~4.17.0",
			"react": "*"
		},
		"engines": {"node": ">=18"}
	}`)
	writeFile(t, dir, "package-lock.json", "{}")

	files := scanDir(t, dir)
	d := &DepAuditor{}
	findings, err := d.Analyze(files, depsRules())
	if err != nil {
		t.Fatal(err)
	}

	found := findByPattern(findings, "DEP-001")
	if len(found) != 3 {
		t.Errorf("expected 3 unpinned-deps findings, got %d", len(found))
	}
}

func TestDepAuditor_PinnedDeps(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "package.json", `{
		"dependencies": {
			"express": "4.18.2"
		},
		"engines": {"node": ">=18"}
	}`)
	writeFile(t, dir, "package-lock.json", "{}")

	files := scanDir(t, dir)
	d := &DepAuditor{}
	findings, err := d.Analyze(files, depsRules())
	if err != nil {
		t.Fatal(err)
	}

	found := findByPattern(findings, "DEP-001")
	if len(found) != 0 {
		t.Error("expected no unpinned-deps findings for pinned versions")
	}
}

func TestDepAuditor_MissingLockfile(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "package.json", `{"dependencies": {"express": "4.18.2"}, "engines": {"node": ">=18"}}`)

	files := scanDir(t, dir)
	d := &DepAuditor{}
	findings, err := d.Analyze(files, depsRules())
	if err != nil {
		t.Fatal(err)
	}

	found := findByPattern(findings, "DEP-002")
	if len(found) == 0 {
		t.Error("expected missing-lockfile finding")
	}
}

func TestDepAuditor_HasLockfile(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "package.json", `{"dependencies": {"express": "4.18.2"}, "engines": {"node": ">=18"}}`)
	writeFile(t, dir, "yarn.lock", "# yarn lockfile")

	files := scanDir(t, dir)
	d := &DepAuditor{}
	findings, err := d.Analyze(files, depsRules())
	if err != nil {
		t.Fatal(err)
	}

	found := findByPattern(findings, "DEP-002")
	if len(found) != 0 {
		t.Error("expected no missing-lockfile finding when yarn.lock exists")
	}
}

func TestDepAuditor_MissingEngines(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "package.json", `{"dependencies": {"express": "4.18.2"}}`)
	writeFile(t, dir, "package-lock.json", "{}")

	files := scanDir(t, dir)
	d := &DepAuditor{}
	findings, err := d.Analyze(files, depsRules())
	if err != nil {
		t.Fatal(err)
	}

	found := findByPattern(findings, "DEP-003")
	if len(found) == 0 {
		t.Error("expected missing-engines finding")
	}
}

func TestDepAuditor_CopyleftLicense(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "package.json", `{"license": "GPL-3.0", "engines": {"node": ">=18"}}`)
	writeFile(t, dir, "package-lock.json", "{}")

	files := scanDir(t, dir)
	d := &DepAuditor{}
	findings, err := d.Analyze(files, depsRules())
	if err != nil {
		t.Fatal(err)
	}

	found := findByPattern(findings, "DEP-004")
	if len(found) == 0 {
		t.Error("expected copyleft-license finding")
	}
}

func TestDepAuditor_MITLicense(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "package.json", `{"license": "MIT", "engines": {"node": ">=18"}}`)
	writeFile(t, dir, "package-lock.json", "{}")

	files := scanDir(t, dir)
	d := &DepAuditor{}
	findings, err := d.Analyze(files, depsRules())
	if err != nil {
		t.Fatal(err)
	}

	found := findByPattern(findings, "DEP-004")
	if len(found) != 0 {
		t.Error("expected no copyleft-license finding for MIT")
	}
}

func TestDepAuditor_NoPackageJSON(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "main.go", "package main")

	files := scanDir(t, dir)
	d := &DepAuditor{}
	findings, err := d.Analyze(files, depsRules())
	if err != nil {
		t.Fatal(err)
	}

	if len(findings) != 0 {
		t.Error("expected no findings when no package.json exists")
	}
}

func TestDepAuditor_EmptyFiles(t *testing.T) {
	d := &DepAuditor{}
	findings, err := d.Analyze(nil, depsRules())
	if err != nil {
		t.Fatal(err)
	}
	if len(findings) != 0 {
		t.Error("expected no findings for empty file list")
	}
}
