package analyzers

import (
	"os"
	"path/filepath"
	"testing"
)

func configRules() []Rule {
	return []Rule{
		{ID: "CFG-001", Title: "Env not gitignored", Severity: "high", Analyzer: "config", Pattern: "env-not-gitignored"},
		{ID: "CFG-002", Title: "Docker latest tag", Severity: "medium", Analyzer: "config", Pattern: "docker-latest-tag"},
		{ID: "CFG-003", Title: "Docker root user", Severity: "high", Analyzer: "config", Pattern: "docker-root-user"},
		{ID: "CFG-004", Title: "Docker no healthcheck", Severity: "medium", Analyzer: "config", Pattern: "docker-no-healthcheck"},
		{ID: "CFG-005", Title: "CORS wildcard", Severity: "high", Analyzer: "config", Pattern: "wildcard-cors"},
		{ID: "CFG-006", Title: "No CI config", Severity: "medium", Analyzer: "config", Pattern: "no-ci-config"},
	}
}

func TestConfigChecker_EnvNotGitignored(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, ".env", "SECRET=foo")
	writeFile(t, dir, ".gitignore", "node_modules\n")

	files := scanDir(t, dir)
	c := &ConfigChecker{}
	findings, err := c.Analyze(files, configRules())
	if err != nil {
		t.Fatal(err)
	}

	found := findByPattern(findings, "CFG-001")
	if len(found) == 0 {
		t.Error("expected env-not-gitignored finding")
	}
}

func TestConfigChecker_EnvGitignored(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, ".env", "SECRET=foo")
	writeFile(t, dir, ".gitignore", ".env\nnode_modules\n")

	files := scanDir(t, dir)
	c := &ConfigChecker{}
	findings, err := c.Analyze(files, configRules())
	if err != nil {
		t.Fatal(err)
	}

	found := findByPattern(findings, "CFG-001")
	if len(found) != 0 {
		t.Error("expected no env-not-gitignored finding when .env is in .gitignore")
	}
}

func TestConfigChecker_DockerLatestTag(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "Dockerfile", "FROM node:latest\nRUN echo hello\n")
	writeFile(t, dir, ".gitignore", ".env\n")

	files := scanDir(t, dir)
	c := &ConfigChecker{}
	findings, err := c.Analyze(files, configRules())
	if err != nil {
		t.Fatal(err)
	}

	found := findByPattern(findings, "CFG-002")
	if len(found) == 0 {
		t.Error("expected docker-latest-tag finding")
	}
}

func TestConfigChecker_DockerNoTag(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "Dockerfile", "FROM node\nRUN echo hello\n")
	writeFile(t, dir, ".gitignore", ".env\n")

	files := scanDir(t, dir)
	c := &ConfigChecker{}
	findings, err := c.Analyze(files, configRules())
	if err != nil {
		t.Fatal(err)
	}

	found := findByPattern(findings, "CFG-002")
	if len(found) == 0 {
		t.Error("expected docker-latest-tag finding for untagged FROM")
	}
}

func TestConfigChecker_DockerPinnedTag(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "Dockerfile", "FROM node:20-alpine\nUSER app\nHEALTHCHECK CMD curl -f http://localhost/\n")
	writeFile(t, dir, ".gitignore", ".env\n")

	files := scanDir(t, dir)
	c := &ConfigChecker{}
	findings, err := c.Analyze(files, configRules())
	if err != nil {
		t.Fatal(err)
	}

	found := findByPattern(findings, "CFG-002")
	if len(found) != 0 {
		t.Error("expected no docker-latest-tag finding for pinned tag")
	}
}

func TestConfigChecker_DockerRootUser(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "Dockerfile", "FROM node:20\nRUN echo hello\n")
	writeFile(t, dir, ".gitignore", ".env\n")

	files := scanDir(t, dir)
	c := &ConfigChecker{}
	findings, err := c.Analyze(files, configRules())
	if err != nil {
		t.Fatal(err)
	}

	found := findByPattern(findings, "CFG-003")
	if len(found) == 0 {
		t.Error("expected docker-root-user finding")
	}
}

func TestConfigChecker_DockerNoHealthcheck(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "Dockerfile", "FROM node:20\nUSER app\n")
	writeFile(t, dir, ".gitignore", ".env\n")

	files := scanDir(t, dir)
	c := &ConfigChecker{}
	findings, err := c.Analyze(files, configRules())
	if err != nil {
		t.Fatal(err)
	}

	found := findByPattern(findings, "CFG-004")
	if len(found) == 0 {
		t.Error("expected docker-no-healthcheck finding")
	}
}

func TestConfigChecker_CORSWildcard(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "server.js", `app.use(cors({ origin: "*" }));`)
	writeFile(t, dir, ".gitignore", ".env\n")

	files := scanDir(t, dir)
	c := &ConfigChecker{}
	findings, err := c.Analyze(files, configRules())
	if err != nil {
		t.Fatal(err)
	}

	found := findByPattern(findings, "CFG-005")
	if len(found) == 0 {
		t.Error("expected wildcard-cors finding")
	}
}

func TestConfigChecker_NoCIConfig(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "main.go", "package main")
	writeFile(t, dir, ".gitignore", ".env\n")

	files := scanDir(t, dir)
	c := &ConfigChecker{}
	findings, err := c.Analyze(files, configRules())
	if err != nil {
		t.Fatal(err)
	}

	found := findByPattern(findings, "CFG-006")
	if len(found) == 0 {
		t.Error("expected no-ci-config finding")
	}
}

func TestConfigChecker_HasCIConfig(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, ".github", "workflows"), 0o755)
	writeFile(t, dir, ".github/workflows/ci.yml", "name: CI")
	writeFile(t, dir, ".gitignore", ".env\n")

	files := scanDir(t, dir)
	c := &ConfigChecker{}
	findings, err := c.Analyze(files, configRules())
	if err != nil {
		t.Fatal(err)
	}

	found := findByPattern(findings, "CFG-006")
	if len(found) != 0 {
		t.Error("expected no no-ci-config finding when CI config exists")
	}
}

func TestConfigChecker_EmptyFiles(t *testing.T) {
	c := &ConfigChecker{}
	findings, err := c.Analyze(nil, configRules())
	if err != nil {
		t.Fatal(err)
	}
	if len(findings) != 0 {
		t.Error("expected no findings for empty file list")
	}
}

// --- helpers ---

func writeFile(t *testing.T, dir, rel, content string) {
	t.Helper()
	path := filepath.Join(dir, rel)
	os.MkdirAll(filepath.Dir(path), 0o755)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func scanDir(t *testing.T, dir string) []ScannedFile {
	t.Helper()
	var files []ScannedFile
	filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}
		rel, _ := filepath.Rel(dir, path)
		files = append(files, ScannedFile{
			Path:         path,
			RelativePath: rel,
			Extension:    filepath.Ext(path),
			Size:         info.Size(),
		})
		return nil
	})
	return files
}

func findByPattern(findings []Finding, id string) []Finding {
	var result []Finding
	for _, f := range findings {
		if f.ID == id {
			result = append(result, f)
		}
	}
	return result
}
