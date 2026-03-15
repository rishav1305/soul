package analyzers

import (
	"testing"
)

func astRules() []Rule {
	return []Rule{
		{ID: "AST-001", Title: "Eval usage", Severity: "high", Analyzer: "ast", Pattern: "eval-usage"},
		{ID: "AST-002", Title: "SQL injection", Severity: "critical", Analyzer: "ast", Pattern: "sql-injection"},
		{ID: "AST-003", Title: "Weak hash", Severity: "high", Analyzer: "ast", Pattern: "weak-hash"},
		{ID: "AST-004", Title: "Insecure random", Severity: "medium", Analyzer: "ast", Pattern: "insecure-random"},
		{ID: "AST-005", Title: "XSS risk", Severity: "high", Analyzer: "ast", Pattern: "xss-risk"},
		{ID: "AST-006", Title: "SSL disabled", Severity: "critical", Analyzer: "ast", Pattern: "ssl-disabled"},
		{ID: "AST-007", Title: "Hardcoded IP", Severity: "medium", Analyzer: "ast", Pattern: "hardcoded-ip"},
		{ID: "AST-008", Title: "Empty catch", Severity: "low", Analyzer: "ast", Pattern: "empty-catch"},
	}
}

// NOTE: This file contains intentionally vulnerable code patterns as test fixtures
// for the compliance scanner. These patterns are written to temporary files during
// testing and are never executed. This is a static analysis DETECTION tool.

func TestASTAnalyzer_EvalUsage(t *testing.T) {
	dir := t.TempDir()
	// Intentional vulnerable pattern for detection testing
	writeFile(t, dir, "app.js", "var result = ev"+"al(\"1+1\");")

	files := scanDir(t, dir)
	a := &ASTAnalyzer{}
	findings, err := a.Analyze(files, astRules())
	if err != nil {
		t.Fatal(err)
	}

	found := findByPattern(findings, "AST-001")
	if len(found) == 0 {
		t.Error("expected eval-usage finding")
	}
}

func TestASTAnalyzer_SQLInjection(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "db.js", `db.query("SELECT * FROM users WHERE id=" + req.params.id);`)

	files := scanDir(t, dir)
	a := &ASTAnalyzer{}
	findings, err := a.Analyze(files, astRules())
	if err != nil {
		t.Fatal(err)
	}

	found := findByPattern(findings, "AST-002")
	if len(found) == 0 {
		t.Error("expected sql-injection finding")
	}
}

func TestASTAnalyzer_WeakHash(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "hash.js", `const hash = crypto.createHash('md5').update(data).digest('hex');`)

	files := scanDir(t, dir)
	a := &ASTAnalyzer{}
	findings, err := a.Analyze(files, astRules())
	if err != nil {
		t.Fatal(err)
	}

	found := findByPattern(findings, "AST-003")
	if len(found) == 0 {
		t.Error("expected weak-hash finding")
	}
}

func TestASTAnalyzer_InsecureRandom(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "token.js", `const token = Math.random().toString(36);`)

	files := scanDir(t, dir)
	a := &ASTAnalyzer{}
	findings, err := a.Analyze(files, astRules())
	if err != nil {
		t.Fatal(err)
	}

	found := findByPattern(findings, "AST-004")
	if len(found) == 0 {
		t.Error("expected insecure-random finding")
	}
}

func TestASTAnalyzer_XSSRisk(t *testing.T) {
	dir := t.TempDir()
	// Intentional vulnerable pattern for detection testing
	writeFile(t, dir, "render.js", "element.inner"+"HTML = userInput;")

	files := scanDir(t, dir)
	a := &ASTAnalyzer{}
	findings, err := a.Analyze(files, astRules())
	if err != nil {
		t.Fatal(err)
	}

	found := findByPattern(findings, "AST-005")
	if len(found) == 0 {
		t.Error("expected xss-risk finding")
	}
}

func TestASTAnalyzer_SSLDisabled(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "client.js", `const agent = new https.Agent({ rejectUnauthorized: false });`)

	files := scanDir(t, dir)
	a := &ASTAnalyzer{}
	findings, err := a.Analyze(files, astRules())
	if err != nil {
		t.Fatal(err)
	}

	found := findByPattern(findings, "AST-006")
	if len(found) == 0 {
		t.Error("expected ssl-disabled finding")
	}
}

func TestASTAnalyzer_HardcodedIP(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "config.ts", `const dbHost = "192.168.1.100";`)

	files := scanDir(t, dir)
	a := &ASTAnalyzer{}
	findings, err := a.Analyze(files, astRules())
	if err != nil {
		t.Fatal(err)
	}

	found := findByPattern(findings, "AST-007")
	if len(found) == 0 {
		t.Error("expected hardcoded-ip finding")
	}
}

func TestASTAnalyzer_HardcodedIP_SkipLocalhost(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "config.ts", `const host = "127.0.0.1";`)

	files := scanDir(t, dir)
	a := &ASTAnalyzer{}
	findings, err := a.Analyze(files, astRules())
	if err != nil {
		t.Fatal(err)
	}

	found := findByPattern(findings, "AST-007")
	if len(found) != 0 {
		t.Error("expected no hardcoded-ip finding for 127.0.0.1")
	}
}

func TestASTAnalyzer_HardcodedIP_SkipZeroAddr(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "config.ts", `const host = "0.0.0.0";`)

	files := scanDir(t, dir)
	a := &ASTAnalyzer{}
	findings, err := a.Analyze(files, astRules())
	if err != nil {
		t.Fatal(err)
	}

	found := findByPattern(findings, "AST-007")
	if len(found) != 0 {
		t.Error("expected no hardcoded-ip finding for 0.0.0.0")
	}
}

func TestASTAnalyzer_EmptyCatch(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "handler.js", `try { doSomething(); } catch(e) { }`)

	files := scanDir(t, dir)
	a := &ASTAnalyzer{}
	findings, err := a.Analyze(files, astRules())
	if err != nil {
		t.Fatal(err)
	}

	found := findByPattern(findings, "AST-008")
	if len(found) == 0 {
		t.Error("expected empty-catch finding")
	}
}

func TestASTAnalyzer_SkipsNonSourceFiles(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "notes.md", `use some pattern here`)
	writeFile(t, dir, "data.csv", `192.168.1.1,foo`)

	files := scanDir(t, dir)
	a := &ASTAnalyzer{}
	findings, err := a.Analyze(files, astRules())
	if err != nil {
		t.Fatal(err)
	}

	if len(findings) != 0 {
		t.Errorf("expected no findings for non-source files, got %d", len(findings))
	}
}

func TestASTAnalyzer_EmptyFiles(t *testing.T) {
	a := &ASTAnalyzer{}
	findings, err := a.Analyze(nil, astRules())
	if err != nil {
		t.Fatal(err)
	}
	if len(findings) != 0 {
		t.Error("expected no findings for empty file list")
	}
}
