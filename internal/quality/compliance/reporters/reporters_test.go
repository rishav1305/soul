package reporters

import (
	"strings"
	"testing"

	"github.com/rishav1305/soul/internal/quality/compliance/analyzers"
	"github.com/rishav1305/soul/internal/quality/compliance/scan"
)

func sampleResult() *scan.ScanResult {
	return &scan.ScanResult{
		Metadata: scan.ScanMetadata{
			Directory:    "/tmp/test-project",
			Timestamp:    "2026-03-15T12:00:00Z",
			Duration:     1.5,
			AnalyzersRun: []string{"secret", "ast", "config", "deps"},
		},
		Summary: scan.ScanSummary{
			Total: 5,
			BySeverity: map[string]int{
				"critical": 1,
				"high":     1,
				"medium":   1,
				"low":      1,
				"info":     1,
			},
			ByFramework: map[string]int{},
			ByAnalyzer:  map[string]int{},
			Fixable:     3,
		},
		Findings: []analyzers.Finding{
			{ID: "sec-001", Title: "Hardcoded secret", Severity: "critical", File: "config.go", Line: 10, Analyzer: "secret", Fixable: true},
			{ID: "hash-001", Title: "Weak hash", Severity: "high", File: "auth.go", Line: 25, Analyzer: "ast", Fixable: true},
			{ID: "cors-001", Title: "Wildcard CORS", Severity: "medium", File: "server.go", Line: 5, Analyzer: "config", Fixable: true},
			{ID: "dep-001", Title: "Outdated dependency", Severity: "low", File: "go.mod", Line: 3, Analyzer: "deps", Fixable: false},
			{ID: "info-001", Title: "Large file", Severity: "info", File: "data.json", Line: 1, Analyzer: "config", Fixable: false},
		},
	}
}

func TestGenerateJSON(t *testing.T) {
	result := sampleResult()
	output, err := GenerateJSON(result)
	if err != nil {
		t.Fatalf("GenerateJSON returned error: %v", err)
	}
	if output == "" {
		t.Fatal("GenerateJSON returned empty output")
	}
	if !strings.Contains(output, `"directory"`) {
		t.Error("JSON output should contain directory field")
	}
	if !strings.Contains(output, `"sec-001"`) {
		t.Error("JSON output should contain finding ID")
	}
}

func TestGenerateTerminal(t *testing.T) {
	result := sampleResult()
	output := GenerateTerminal(result)
	if output == "" {
		t.Fatal("GenerateTerminal returned empty output")
	}
	if !strings.Contains(output, "Compliance Scan Report") {
		t.Error("terminal output should contain header")
	}
	if !strings.Contains(output, "Critical") {
		t.Error("terminal output should contain severity labels")
	}
}

func TestGenerateBadge(t *testing.T) {
	result := sampleResult()
	output := GenerateBadge(result)
	if !strings.Contains(output, "<svg") {
		t.Error("badge output should contain svg tag")
	}
	if !strings.Contains(output, "compliance") {
		t.Error("badge should contain compliance label")
	}
}

func TestCalculateScore(t *testing.T) {
	tests := []struct {
		name     string
		summary  scan.ScanSummary
		expected int
	}{
		{
			name:     "perfect score",
			summary:  scan.ScanSummary{BySeverity: map[string]int{}},
			expected: 100,
		},
		{
			name: "mixed findings",
			summary: scan.ScanSummary{BySeverity: map[string]int{
				"critical": 1, "high": 1, "medium": 1, "low": 1,
			}},
			expected: 100 - (10 + 5 + 2 + 1), // 82
		},
		{
			name: "many critical",
			summary: scan.ScanSummary{BySeverity: map[string]int{
				"critical": 15,
			}},
			expected: 0, // clamped
		},
		{
			name: "only low",
			summary: scan.ScanSummary{BySeverity: map[string]int{
				"low": 3,
			}},
			expected: 97,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := &scan.ScanResult{Summary: tc.summary}
			got := CalculateScore(result)
			if got != tc.expected {
				t.Errorf("CalculateScore() = %d, want %d", got, tc.expected)
			}
		})
	}
}

func TestGenerate_UnknownFormat(t *testing.T) {
	result := sampleResult()
	_, err := Generate(result, "pdf")
	if err == nil {
		t.Fatal("expected error for unknown format")
	}
	if !strings.Contains(err.Error(), "unknown") {
		t.Errorf("error should mention unknown format, got: %v", err)
	}
}

func TestGenerate_ValidFormats(t *testing.T) {
	result := sampleResult()

	for _, format := range []string{"json", "terminal", "html"} {
		t.Run(format, func(t *testing.T) {
			output, err := Generate(result, format)
			if err != nil {
				t.Fatalf("Generate(%q) error: %v", format, err)
			}
			if output == "" {
				t.Fatalf("Generate(%q) returned empty output", format)
			}
		})
	}
}
