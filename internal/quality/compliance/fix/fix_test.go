package fix

import (
	"os"
	"strings"
	"testing"

	"github.com/rishav1305/soul/internal/quality/compliance/analyzers"
)

func TestApplyFixes_DryRun(t *testing.T) {
	findings := []analyzers.Finding{
		{
			ID:       "sec-001",
			Title:    "Hardcoded secret",
			Severity: "critical",
			File:     "config.go",
			Line:     10,
			Evidence: `apiKey := "sk-secret-1234"`,
			Analyzer: "secret",
			Fixable:  true,
		},
		{
			ID:       "info-001",
			Title:    "Not fixable",
			Severity: "info",
			File:     "readme.md",
			Line:     1,
			Evidence: "some info",
			Analyzer: "info",
			Fixable:  false,
		},
	}

	results, err := ApplyFixes(findings, true)
	if err != nil {
		t.Fatalf("ApplyFixes returned error: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 result (non-fixable skipped), got %d", len(results))
	}

	r := results[0]
	if !r.Fixed {
		t.Errorf("expected Fixed=true, got false; error: %s", r.Error)
	}
	if r.Patch == "" {
		t.Error("expected non-empty patch in dry run")
	}
	if r.Strategy != "secret-to-env" {
		t.Errorf("expected strategy secret-to-env, got %s", r.Strategy)
	}
	// Verify patch contains diff markers
	if !strings.Contains(r.Patch, "---") || !strings.Contains(r.Patch, "+++") {
		t.Error("patch should contain unified diff markers")
	}
}

func TestApplyFixes_SecretToEnv(t *testing.T) {
	tests := []struct {
		name     string
		file     string
		evidence string
		wantEnv  string
	}{
		{
			name:     "Go file",
			file:     "main.go",
			evidence: `apiKey := "sk-secret-1234"`,
			wantEnv:  `os.Getenv`,
		},
		{
			name:     "Python file",
			file:     "app.py",
			evidence: `api_key = "sk-secret-1234"`,
			wantEnv:  `os.environ`,
		},
		{
			name:     "JS file",
			file:     "config.js",
			evidence: `const apiKey = "sk-secret-1234"`,
			wantEnv:  `process.env`,
		},
		{
			name:     "TS file",
			file:     "config.ts",
			evidence: `const apiKey = "sk-secret-1234"`,
			wantEnv:  `process.env`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			findings := []analyzers.Finding{
				{
					ID:       "sec-001",
					File:     tc.file,
					Line:     5,
					Evidence: tc.evidence,
					Analyzer: "secret",
					Fixable:  true,
				},
			}

			results, err := ApplyFixes(findings, true)
			if err != nil {
				t.Fatalf("ApplyFixes error: %v", err)
			}
			if len(results) != 1 {
				t.Fatalf("expected 1 result, got %d", len(results))
			}
			if !strings.Contains(results[0].Patch, tc.wantEnv) {
				t.Errorf("patch should contain %s, got:\n%s", tc.wantEnv, results[0].Patch)
			}
		})
	}
}

func TestSelectStrategy(t *testing.T) {
	tests := []struct {
		name     string
		finding  analyzers.Finding
		expected string
	}{
		{
			name:     "secret finding",
			finding:  analyzers.Finding{ID: "sec-001", Analyzer: "secret"},
			expected: "secret-to-env",
		},
		{
			name:     "hash finding",
			finding:  analyzers.Finding{ID: "crypto-hash-weak", Analyzer: "ast"},
			expected: "weak-hash-upgrade",
		},
		{
			name:     "dangerous exec finding",
			finding:  analyzers.Finding{ID: "dangerous-exec", Analyzer: "ast"},
			expected: "dangerous-code-removal",
		},
		{
			name:     "cors finding",
			finding:  analyzers.Finding{ID: "cors-wildcard", Analyzer: "config"},
			expected: "cors-restrict",
		},
		{
			name:     "eval finding",
			finding:  analyzers.Finding{ID: "eval-usage", Analyzer: "ast"},
			expected: "dangerous-code-removal",
		},
		{
			name:     "unknown finding",
			finding:  analyzers.Finding{ID: "unknown-thing", Analyzer: "unknown"},
			expected: "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := selectStrategy(tc.finding)
			if got != tc.expected {
				t.Errorf("selectStrategy() = %q, want %q", got, tc.expected)
			}
		})
	}
}

func TestApplyWeakHashUpgrade(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"md5 double-quoted", `createHash("md5")`, `createHash("sha256")`},
		{"md5 single-quoted", `createHash('md5')`, `createHash('sha256')`},
		{"sha1 double-quoted", `createHash("sha1")`, `createHash("sha256")`},
		{"sha1 single-quoted", `createHash('sha1')`, `createHash('sha256')`},
		{"go crypto/md5 import", `"crypto/md5"`, `"crypto/sha256"`},
		{"go crypto/sha1 import", `"crypto/sha1"`, `"crypto/sha256"`},
		{"go md5.New()", `h := md5.New()`, `h := sha256.New()`},
		{"go sha1.New()", `h := sha1.New()`, `h := sha256.New()`},
		{"no match passthrough", `createHash("sha256")`, `createHash("sha256")`},
		{"empty string", "", ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := applyWeakHashUpgrade(tc.in)
			if got != tc.want {
				t.Errorf("applyWeakHashUpgrade(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestApplyDangerousCodeRemoval(t *testing.T) {
	tests := []struct {
		name string
		line string
		ext  string
		want string
	}{
		{
			"go file",
			"    unsafeCall(data)",
			".go",
			"    // TODO: dangerous code removed by compliance fix\n    // unsafeCall(data)",
		},
		{
			"js file",
			"\triskyFunc(input)",
			".js",
			"\t// TODO: dangerous code removed by compliance fix\n\t// riskyFunc(input)",
		},
		{
			"python file",
			"    dangerous_call(payload)",
			".py",
			"    # TODO: dangerous code removed by compliance fix\n    # dangerous_call(payload)",
		},
		{
			"ts file no indent",
			"runUnsafe(x)",
			".ts",
			"// TODO: dangerous code removed by compliance fix\n// runUnsafe(x)",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := applyDangerousCodeRemoval(tc.line, tc.ext)
			if got != tc.want {
				t.Errorf("applyDangerousCodeRemoval() =\n%q\nwant:\n%q", got, tc.want)
			}
		})
	}
}

func TestApplyCorsRestrict(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"double-quoted wildcard", `origin: "*"`, `origin: "https://your-domain.com"`},
		{"single-quoted wildcard", `origin: '*'`, `origin: 'https://your-domain.com'`},
		{"both wildcards", `"*" and '*'`, `"https://your-domain.com" and 'https://your-domain.com'`},
		{"no wildcard", `origin: "example.com"`, `origin: "example.com"`},
		{"empty string", "", ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := applyCorsRestrict(tc.in)
			if got != tc.want {
				t.Errorf("applyCorsRestrict(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestExtForFile(t *testing.T) {
	tests := []struct {
		path string
		want string
	}{
		{"main.go", ".go"},
		{"app.py", ".py"},
		{"config.JS", ".js"},
		{"styles.CSS", ".css"},
		{"Makefile", ""},
		{"path/to/file.tsx", ".tsx"},
		{"archive.tar.gz", ".gz"},
	}
	for _, tc := range tests {
		t.Run(tc.path, func(t *testing.T) {
			got := ExtForFile(tc.path)
			if got != tc.want {
				t.Errorf("ExtForFile(%q) = %q, want %q", tc.path, got, tc.want)
			}
		})
	}
}

func TestApplyPatch(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		dir := t.TempDir()
		path := dir + "/test.go"
		if err := os.WriteFile(path, []byte("line1\nline2\nline3\n"), 0644); err != nil {
			t.Fatal(err)
		}
		if err := applyPatch(path, 2, "REPLACED"); err != nil {
			t.Fatalf("applyPatch: %v", err)
		}
		data, _ := os.ReadFile(path)
		if !strings.Contains(string(data), "REPLACED") {
			t.Errorf("expected REPLACED in file, got: %s", data)
		}
	})

	t.Run("out of range", func(t *testing.T) {
		dir := t.TempDir()
		path := dir + "/small.go"
		if err := os.WriteFile(path, []byte("only\n"), 0644); err != nil {
			t.Fatal(err)
		}
		err := applyPatch(path, 99, "REPLACED")
		if err == nil {
			t.Error("expected error for out-of-range line number")
		}
	})

	t.Run("nonexistent file", func(t *testing.T) {
		err := applyPatch("/tmp/does-not-exist-soul-test.go", 1, "REPLACED")
		if err == nil {
			t.Error("expected error for nonexistent file")
		}
	})
}

func TestReplaceQuotedValue(t *testing.T) {
	tests := []struct {
		name        string
		line        string
		replacement string
		want        string
	}{
		{"double quotes", `key = "secret123"`, "ENV_VAR", `key = ENV_VAR`},
		{"single quotes", `key = 'secret123'`, "ENV_VAR", `key = ENV_VAR`},
		{"no quotes", "key = secret123", "ENV_VAR", "key = secret123"},
		{"empty replacement", `key = "secret"`, "", `key = `},
		{"multiple quotes takes first", `"a" and "b"`, "X", `X and "b"`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := replaceQuotedValue(tc.line, tc.replacement)
			if got != tc.want {
				t.Errorf("replaceQuotedValue(%q, %q) = %q, want %q", tc.line, tc.replacement, got, tc.want)
			}
		})
	}
}

func TestFormatUnifiedDiff(t *testing.T) {
	patch := formatUnifiedDiff("config.go", 10, "old line", "new line")
	if !strings.Contains(patch, "--- a/config.go") {
		t.Error("expected --- a/ header")
	}
	if !strings.Contains(patch, "+++ b/config.go") {
		t.Error("expected +++ b/ header")
	}
	if !strings.Contains(patch, "@@ -10,1 +10,1 @@") {
		t.Error("expected @@ hunk header")
	}
	if !strings.Contains(patch, "-old line") {
		t.Error("expected -old line")
	}
	if !strings.Contains(patch, "+new line") {
		t.Error("expected +new line")
	}
}

func TestGenerateFix_UnknownStrategy(t *testing.T) {
	f := analyzers.Finding{
		ID: "test-1", File: "test.go", Line: 1, Evidence: "some code",
		Analyzer: "test", Fixable: true,
	}
	_, _, err := generateFix(f, "nonexistent-strategy")
	if err == nil {
		t.Error("expected error for unknown strategy")
	}
}

func TestGenerateFix_EmptyEvidence(t *testing.T) {
	f := analyzers.Finding{
		ID: "test-1", File: "test.go", Line: 1, Evidence: "",
		Analyzer: "test", Fixable: true,
	}
	_, _, err := generateFix(f, "secret-to-env")
	if err == nil {
		t.Error("expected error for empty evidence")
	}
}

func TestApplyFixes_NoFixStrategy(t *testing.T) {
	findings := []analyzers.Finding{
		{
			ID: "unknown-id", File: "test.go", Line: 1,
			Evidence: "something", Analyzer: "unknown", Fixable: true,
		},
	}
	results, err := ApplyFixes(findings, true)
	if err != nil {
		t.Fatalf("ApplyFixes: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Fixed {
		t.Error("expected Fixed=false for unknown strategy")
	}
	if results[0].Error != "no applicable fix strategy" {
		t.Errorf("unexpected error: %s", results[0].Error)
	}
}

func TestApplyFixes_EmptyEvidence(t *testing.T) {
	findings := []analyzers.Finding{
		{
			ID: "sec-001", File: "test.go", Line: 1,
			Evidence: "", Analyzer: "secret", Fixable: true,
		},
	}
	results, err := ApplyFixes(findings, true)
	if err != nil {
		t.Fatalf("ApplyFixes: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Fixed {
		t.Error("expected Fixed=false for empty evidence")
	}
}
