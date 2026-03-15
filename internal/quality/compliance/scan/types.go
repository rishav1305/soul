package scan

import "github.com/rishav1305/soul-v2/internal/quality/compliance/analyzers"

// ScanOptions configures a compliance scan.
type ScanOptions struct {
	Directory  string
	Frameworks []string
	Severity   []string // filter results
	Analyzers  []string // run specific analyzers
	Exclude    []string // exclude paths
}

// ScanResult holds the complete output of a compliance scan.
type ScanResult struct {
	Findings []analyzers.Finding `json:"findings"`
	Summary  ScanSummary         `json:"summary"`
	Metadata ScanMetadata        `json:"metadata"`
}

// ScanSummary provides aggregate counts.
type ScanSummary struct {
	Total       int            `json:"total"`
	BySeverity  map[string]int `json:"by_severity"`
	ByFramework map[string]int `json:"by_framework"`
	ByAnalyzer  map[string]int `json:"by_analyzer"`
	Fixable     int            `json:"fixable"`
}

// ScanMetadata records scan execution details.
type ScanMetadata struct {
	Directory    string   `json:"directory"`
	Duration     float64  `json:"duration_seconds"`
	AnalyzersRun []string `json:"analyzers_run"`
	Timestamp    string   `json:"timestamp"`
}
