package analyzers

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/rishav1305/soul/products/compliance/rules"
)

func TestConfigCheckerDockerNoUser(t *testing.T) {
	dir := t.TempDir()

	// Dockerfile without USER directive
	dockerfile := tempFile(t, dir, "Dockerfile", `FROM node:18
WORKDIR /app
COPY . .
RUN npm install
CMD ["node", "server.js"]
`)

	allRules := rules.Load(nil)
	checker := &ConfigChecker{}
	findings, err := checker.Analyze([]ScannedFile{dockerfile}, allRules)
	if err != nil {
		t.Fatalf("Analyze returned error: %v", err)
	}

	// Should find docker-root-user and docker-no-healthcheck
	foundRoot := false
	foundHealthcheck := false
	for _, f := range findings {
		if f.ID == "CONFIG-001" {
			foundRoot = true
		}
		if f.ID == "CONFIG-002" {
			foundHealthcheck = true
		}
	}

	if !foundRoot {
		t.Error("expected CONFIG-001 (docker-root-user) finding, not found")
	}
	if !foundHealthcheck {
		t.Error("expected CONFIG-002 (docker-no-healthcheck) finding, not found")
	}

	// Verify analyzer field
	for _, f := range findings {
		if f.Analyzer != "config-checker" {
			t.Errorf("expected analyzer 'config-checker', got %q", f.Analyzer)
		}
	}
}

func TestConfigCheckerDockerCompliant(t *testing.T) {
	dir := t.TempDir()

	// Compliant Dockerfile with USER and HEALTHCHECK
	dockerfile := tempFile(t, dir, "Dockerfile", `FROM node:18-alpine
WORKDIR /app
COPY . .
RUN npm install
USER appuser
HEALTHCHECK CMD curl -f http://localhost:3000/health || exit 1
CMD ["node", "server.js"]
`)

	allRules := rules.Load(nil)
	checker := &ConfigChecker{}
	findings, err := checker.Analyze([]ScannedFile{dockerfile}, allRules)
	if err != nil {
		t.Fatalf("Analyze returned error: %v", err)
	}

	// Should NOT find docker-root-user or docker-no-healthcheck
	for _, f := range findings {
		if f.ID == "CONFIG-001" {
			t.Error("did not expect CONFIG-001 finding for Dockerfile with USER directive")
		}
		if f.ID == "CONFIG-002" {
			t.Error("did not expect CONFIG-002 finding for Dockerfile with HEALTHCHECK")
		}
	}
}

func TestConfigCheckerEnvNotGitignored(t *testing.T) {
	dir := t.TempDir()

	// Create .env file and .gitignore without .env
	envFile := tempFile(t, dir, ".env", "DB_PASSWORD=secret")
	gitignore := tempFile(t, dir, ".gitignore", "node_modules/\n*.log\n")

	allRules := rules.Load(nil)
	checker := &ConfigChecker{}
	findings, err := checker.Analyze([]ScannedFile{envFile, gitignore}, allRules)
	if err != nil {
		t.Fatalf("Analyze returned error: %v", err)
	}

	foundEnvExposed := false
	for _, f := range findings {
		if f.ID == "CONFIG-003" {
			foundEnvExposed = true
			break
		}
	}
	if !foundEnvExposed {
		t.Error("expected CONFIG-003 (env-not-gitignored) finding, not found")
	}
}

func TestConfigCheckerEnvGitignored(t *testing.T) {
	dir := t.TempDir()

	// Create .env file and .gitignore WITH .env
	envFile := tempFile(t, dir, ".env", "DB_PASSWORD=secret")
	gitignore := tempFile(t, dir, ".gitignore", "node_modules/\n.env\n*.log\n")

	allRules := rules.Load(nil)
	checker := &ConfigChecker{}
	findings, err := checker.Analyze([]ScannedFile{envFile, gitignore}, allRules)
	if err != nil {
		t.Fatalf("Analyze returned error: %v", err)
	}

	for _, f := range findings {
		if f.ID == "CONFIG-003" {
			t.Error("did not expect CONFIG-003 finding when .env is in .gitignore")
		}
	}
}

func TestConfigCheckerCORSWildcard(t *testing.T) {
	dir := t.TempDir()

	sf := tempFile(t, dir, "server.ts", `
import cors from 'cors';
app.use(cors({
  origin: "*",
  credentials: true,
}));
`)

	allRules := rules.Load(nil)
	checker := &ConfigChecker{}
	findings, err := checker.Analyze([]ScannedFile{sf}, allRules)
	if err != nil {
		t.Fatalf("Analyze returned error: %v", err)
	}

	foundCORS := false
	for _, f := range findings {
		if f.ID == "CRYPTO-008" {
			foundCORS = true
			break
		}
	}
	if !foundCORS {
		t.Error("expected CRYPTO-008 (wildcard-cors) finding, not found")
	}
}

func TestConfigCheckerNoCIConfig(t *testing.T) {
	dir := t.TempDir()

	// Just a random file, no CI config
	sf := tempFile(t, dir, "README.md", "# Hello")

	// Change extension to something known so it shows up
	sf.Extension = "md"

	allRules := rules.Load(nil)
	checker := &ConfigChecker{}
	findings, err := checker.Analyze([]ScannedFile{sf}, allRules)
	if err != nil {
		t.Fatalf("Analyze returned error: %v", err)
	}

	foundNoCI := false
	for _, f := range findings {
		if f.ID == "CHANGE-001" {
			foundNoCI = true
			break
		}
	}
	if !foundNoCI {
		t.Error("expected CHANGE-001 (no-ci-config) finding, not found")
	}
}

func TestConfigCheckerWithCIConfig(t *testing.T) {
	dir := t.TempDir()

	// Create .github/workflows directory and a workflow file
	workflowDir := filepath.Join(dir, ".github", "workflows")
	if err := os.MkdirAll(workflowDir, 0o755); err != nil {
		t.Fatal(err)
	}

	ciFile := tempFile(t, dir, ".github/workflows/ci.yml", "name: CI\non: push\n")

	allRules := rules.Load(nil)
	checker := &ConfigChecker{}
	findings, err := checker.Analyze([]ScannedFile{ciFile}, allRules)
	if err != nil {
		t.Fatalf("Analyze returned error: %v", err)
	}

	for _, f := range findings {
		if f.ID == "CHANGE-001" {
			t.Error("did not expect CHANGE-001 finding when CI config exists")
		}
	}
}
