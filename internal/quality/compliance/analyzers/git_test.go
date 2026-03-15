package analyzers

import (
	"testing"
)

func gitRules() []Rule {
	return []Rule{
		{ID: "GIT-001", Title: "Env not gitignored", Severity: "high", Analyzer: "git", Pattern: "env-not-gitignored"},
		{ID: "GIT-002", Title: "Incomplete gitignore", Severity: "medium", Analyzer: "git", Pattern: "incomplete-gitignore"},
		{ID: "GIT-003", Title: "Sensitive not gitignored", Severity: "high", Analyzer: "git", Pattern: "sensitive-not-gitignored"},
		{ID: "GIT-004", Title: "Missing CODEOWNERS", Severity: "low", Analyzer: "git", Pattern: "missing-codeowners"},
		{ID: "GIT-005", Title: "No security policy", Severity: "medium", Analyzer: "git", Pattern: "no-security-policy"},
		{ID: "GIT-006", Title: "Missing LICENSE", Severity: "medium", Analyzer: "git", Pattern: "missing-license"},
	}
}

func TestGitAnalyzer_MissingGitignore(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "main.go", "package main")

	files := scanDir(t, dir)
	g := &GitAnalyzer{}
	findings, err := g.Analyze(files, gitRules())
	if err != nil {
		t.Fatal(err)
	}

	found := findByPattern(findings, "GIT-002")
	if len(found) == 0 {
		t.Error("expected incomplete-gitignore finding when .gitignore is missing")
	}
}

func TestGitAnalyzer_MissingNodeModules(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, ".gitignore", ".env\n")

	files := scanDir(t, dir)
	g := &GitAnalyzer{}
	findings, err := g.Analyze(files, gitRules())
	if err != nil {
		t.Fatal(err)
	}

	found := findByPattern(findings, "GIT-002")
	if len(found) == 0 {
		t.Error("expected incomplete-gitignore finding for missing node_modules")
	}
}

func TestGitAnalyzer_EnvNotGitignored(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, ".gitignore", "node_modules\n")

	files := scanDir(t, dir)
	g := &GitAnalyzer{}
	findings, err := g.Analyze(files, gitRules())
	if err != nil {
		t.Fatal(err)
	}

	found := findByPattern(findings, "GIT-001")
	if len(found) == 0 {
		t.Error("expected env-not-gitignored finding")
	}
}

func TestGitAnalyzer_SensitiveNotGitignored(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, ".gitignore", ".env\nnode_modules\n")

	files := scanDir(t, dir)
	g := &GitAnalyzer{}
	findings, err := g.Analyze(files, gitRules())
	if err != nil {
		t.Fatal(err)
	}

	found := findByPattern(findings, "GIT-003")
	if len(found) < 4 {
		t.Errorf("expected at least 4 sensitive-not-gitignored findings, got %d", len(found))
	}
}

func TestGitAnalyzer_MissingCODEOWNERS(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, ".gitignore", ".env\nnode_modules\n")

	files := scanDir(t, dir)
	g := &GitAnalyzer{}
	findings, err := g.Analyze(files, gitRules())
	if err != nil {
		t.Fatal(err)
	}

	found := findByPattern(findings, "GIT-004")
	if len(found) == 0 {
		t.Error("expected missing-codeowners finding")
	}
}

func TestGitAnalyzer_HasCODEOWNERS(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, ".gitignore", ".env\nnode_modules\n")
	writeFile(t, dir, "CODEOWNERS", "* @team")

	files := scanDir(t, dir)
	g := &GitAnalyzer{}
	findings, err := g.Analyze(files, gitRules())
	if err != nil {
		t.Fatal(err)
	}

	found := findByPattern(findings, "GIT-004")
	if len(found) != 0 {
		t.Error("expected no missing-codeowners finding when CODEOWNERS exists")
	}
}

func TestGitAnalyzer_MissingLICENSE(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, ".gitignore", ".env\nnode_modules\n")

	files := scanDir(t, dir)
	g := &GitAnalyzer{}
	findings, err := g.Analyze(files, gitRules())
	if err != nil {
		t.Fatal(err)
	}

	found := findByPattern(findings, "GIT-006")
	if len(found) == 0 {
		t.Error("expected missing-license finding")
	}
}

func TestGitAnalyzer_HasLICENSE(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, ".gitignore", ".env\nnode_modules\n")
	writeFile(t, dir, "LICENSE", "MIT")

	files := scanDir(t, dir)
	g := &GitAnalyzer{}
	findings, err := g.Analyze(files, gitRules())
	if err != nil {
		t.Fatal(err)
	}

	found := findByPattern(findings, "GIT-006")
	if len(found) != 0 {
		t.Error("expected no missing-license finding when LICENSE exists")
	}
}

func TestGitAnalyzer_MissingSECURITY(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, ".gitignore", ".env\nnode_modules\n")

	files := scanDir(t, dir)
	g := &GitAnalyzer{}
	findings, err := g.Analyze(files, gitRules())
	if err != nil {
		t.Fatal(err)
	}

	found := findByPattern(findings, "GIT-005")
	if len(found) == 0 {
		t.Error("expected no-security-policy finding")
	}
}

func TestGitAnalyzer_EmptyFiles(t *testing.T) {
	g := &GitAnalyzer{}
	findings, err := g.Analyze(nil, gitRules())
	if err != nil {
		t.Fatal(err)
	}
	if len(findings) != 0 {
		t.Error("expected no findings for empty file list")
	}
}
