package fix

import (
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
