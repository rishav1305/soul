package reporters

import (
	"strings"
	"testing"

	"github.com/rishav1305/soul/products/compliance-go/analyzers"
	"github.com/rishav1305/soul/products/compliance-go/scan"
)

// TestBadgeScoreCalculation passes results with known findings and verifies
// the SVG contains the expected score.
func TestBadgeScoreCalculation(t *testing.T) {
	result := &scan.ScanResult{
		Findings: []analyzers.Finding{
			{ID: "SECRET-001", Severity: "critical"}, // -10
			{ID: "SECRET-003", Severity: "high"},     // -5
			{ID: "LOG-004", Severity: "medium"},      // -2
		},
		Summary: scan.ScanSummary{
			Total:      3,
			BySeverity: map[string]int{"critical": 1, "high": 1, "medium": 1},
		},
	}

	svg := GenerateBadge(result)

	if !strings.Contains(svg, "<svg") {
		t.Error("expected SVG output")
	}

	// Score: 100 - 10 - 5 - 2 = 83
	if !strings.Contains(svg, "83%") {
		t.Errorf("expected score 83%% in SVG, got:\n%s", svg)
	}
}

// TestBadgeColorGreen passes clean results (score >= 90) and verifies
// green color in SVG.
func TestBadgeColorGreen(t *testing.T) {
	result := &scan.ScanResult{
		Findings: []analyzers.Finding{
			{ID: "LOG-005", Severity: "low"}, // -1
		},
		Summary: scan.ScanSummary{
			Total:      1,
			BySeverity: map[string]int{"low": 1},
		},
	}

	svg := GenerateBadge(result)

	// Score: 100 - 1 = 99, which is >= 90 so should be green
	if !strings.Contains(svg, "#4c1") {
		t.Errorf("expected green color (#4c1) in SVG for score 99, got:\n%s", svg)
	}
	if !strings.Contains(svg, "99%") {
		t.Errorf("expected score 99%% in SVG, got:\n%s", svg)
	}
}

// TestBadgeColorRed passes many critical findings (score < 50) and verifies
// red color in SVG.
func TestBadgeColorRed(t *testing.T) {
	findings := make([]analyzers.Finding, 6)
	for i := range findings {
		findings[i] = analyzers.Finding{
			ID:       "SECRET-001",
			Severity: "critical", // each -10, total -60
		}
	}

	result := &scan.ScanResult{
		Findings: findings,
		Summary: scan.ScanSummary{
			Total:      6,
			BySeverity: map[string]int{"critical": 6},
		},
	}

	svg := GenerateBadge(result)

	// Score: 100 - 60 = 40, which is < 50 so should be red
	if !strings.Contains(svg, "#e05d44") {
		t.Errorf("expected red color (#e05d44) in SVG for score 40, got:\n%s", svg)
	}
	if !strings.Contains(svg, "40%") {
		t.Errorf("expected score 40%% in SVG, got:\n%s", svg)
	}
}

// TestBadgeMinimumScore ensures the score doesn't go below 0.
func TestBadgeMinimumScore(t *testing.T) {
	findings := make([]analyzers.Finding, 15)
	for i := range findings {
		findings[i] = analyzers.Finding{
			ID:       "SECRET-001",
			Severity: "critical", // each -10, total -150 -> clamped to 0
		}
	}

	result := &scan.ScanResult{
		Findings: findings,
		Summary: scan.ScanSummary{
			Total:      15,
			BySeverity: map[string]int{"critical": 15},
		},
	}

	svg := GenerateBadge(result)

	if !strings.Contains(svg, "0%") {
		t.Errorf("expected score 0%% in SVG, got:\n%s", svg)
	}
}

// TestBadgePerfectScore ensures a clean scan produces 100%.
func TestBadgePerfectScore(t *testing.T) {
	result := &scan.ScanResult{
		Findings: nil,
		Summary: scan.ScanSummary{
			Total:      0,
			BySeverity: map[string]int{},
		},
	}

	svg := GenerateBadge(result)

	if !strings.Contains(svg, "100%") {
		t.Errorf("expected score 100%% in SVG, got:\n%s", svg)
	}
	if !strings.Contains(svg, "#4c1") {
		t.Errorf("expected green color for perfect score, got:\n%s", svg)
	}
}
