package scan

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/rishav1305/soul/products/compliance-go/analyzers"
	"github.com/rishav1305/soul/products/compliance-go/rules"
)

// ScanOptions configures a compliance scan run.
type ScanOptions struct {
	Directory  string   `json:"directory"`
	Frameworks []string `json:"frameworks,omitempty"`
	Severity   []string `json:"severity,omitempty"`
	Analyzers  []string `json:"analyzers,omitempty"`
	Exclude    []string `json:"exclude,omitempty"`
}

// ScanResult holds the complete output of a compliance scan.
type ScanResult struct {
	Findings []analyzers.Finding `json:"findings"`
	Summary  ScanSummary         `json:"summary"`
	Metadata ScanMetadata        `json:"metadata"`
}

// ScanSummary provides aggregate counts of the scan findings.
type ScanSummary struct {
	Total       int            `json:"total"`
	BySeverity  map[string]int `json:"by_severity"`
	ByFramework map[string]int `json:"by_framework"`
	ByAnalyzer  map[string]int `json:"by_analyzer"`
	Fixable     int            `json:"fixable"`
}

// ScanMetadata records information about the scan execution itself.
type ScanMetadata struct {
	Directory        string            `json:"directory"`
	Duration         float64           `json:"duration"`
	AnalyzersRun     []string          `json:"analyzers_run"`
	AnalyzerFailures []AnalyzerFailure `json:"analyzer_failures,omitempty"`
	Frameworks       []string          `json:"frameworks"`
	Timestamp        string            `json:"timestamp"`
}

// AnalyzerFailure records an analyzer that returned an error during the scan.
type AnalyzerFailure struct {
	Analyzer string `json:"analyzer"`
	Error    string `json:"error"`
}

// allAnalyzers returns the full set of compliance analyzers.
func allAnalyzers() []analyzers.Analyzer {
	return []analyzers.Analyzer{
		&analyzers.SecretScanner{},
		&analyzers.ConfigChecker{},
		&analyzers.GitAnalyzer{},
		&analyzers.DepAuditor{},
		&analyzers.ASTAnalyzer{},
	}
}

// RunScan executes a full compliance scan with the given options.
// It walks the directory, loads rules, runs all selected analyzers in parallel,
// deduplicates findings, applies filters, and returns a unified result.
func RunScan(opts ScanOptions) (*ScanResult, error) {
	start := time.Now()

	// 1. Scan the directory to collect files.
	files, err := ScanDirectory(opts.Directory, opts.Exclude)
	if err != nil {
		return nil, fmt.Errorf("scan directory: %w", err)
	}

	// 2. Load rules (filtered by frameworks if specified).
	loadedRules := rules.Load(opts.Frameworks)

	// 3. Determine which analyzers to run.
	active := allAnalyzers()
	if len(opts.Analyzers) > 0 {
		wantSet := make(map[string]bool, len(opts.Analyzers))
		for _, name := range opts.Analyzers {
			wantSet[name] = true
		}
		var filtered []analyzers.Analyzer
		for _, a := range active {
			if wantSet[a.Name()] {
				filtered = append(filtered, a)
			}
		}
		active = filtered
	}

	// 4. Run all active analyzers in parallel.
	var (
		mu              sync.Mutex
		wg              sync.WaitGroup
		allFindings     []analyzers.Finding
		analyzerNames   []string
		analyzerFails   []AnalyzerFailure
	)

	for _, a := range active {
		analyzerNames = append(analyzerNames, a.Name())
	}

	for _, a := range active {
		wg.Add(1)
		go func(analyzer analyzers.Analyzer) {
			defer wg.Done()

			findings, err := analyzer.Analyze(files, loadedRules)

			mu.Lock()
			defer mu.Unlock()

			if err != nil {
				analyzerFails = append(analyzerFails, AnalyzerFailure{
					Analyzer: analyzer.Name(),
					Error:    err.Error(),
				})
				return
			}
			allFindings = append(allFindings, findings...)
		}(a)
	}

	wg.Wait()

	// 5. Deduplicate findings on file:line:id.
	allFindings = deduplicateFindings(allFindings)

	// 6. Filter by severity if specified.
	if len(opts.Severity) > 0 {
		severitySet := make(map[string]bool, len(opts.Severity))
		for _, s := range opts.Severity {
			severitySet[strings.ToLower(s)] = true
		}
		var filtered []analyzers.Finding
		for _, f := range allFindings {
			if severitySet[strings.ToLower(f.Severity)] {
				filtered = append(filtered, f)
			}
		}
		allFindings = filtered
	}

	// 7. Build summary counts.
	summary := buildSummary(allFindings)

	// 8. Assemble the result.
	duration := time.Since(start).Seconds()

	result := &ScanResult{
		Findings: allFindings,
		Summary:  summary,
		Metadata: ScanMetadata{
			Directory:        opts.Directory,
			Duration:         duration,
			AnalyzersRun:     analyzerNames,
			AnalyzerFailures: analyzerFails,
			Frameworks:       opts.Frameworks,
			Timestamp:        time.Now().UTC().Format(time.RFC3339),
		},
	}

	return result, nil
}

// deduplicateFindings removes duplicate findings based on the key file:line:id.
func deduplicateFindings(findings []analyzers.Finding) []analyzers.Finding {
	seen := make(map[string]bool)
	var unique []analyzers.Finding

	for _, f := range findings {
		key := fmt.Sprintf("%s:%d:%s", f.File, f.Line, f.ID)
		if seen[key] {
			continue
		}
		seen[key] = true
		unique = append(unique, f)
	}

	return unique
}

// buildSummary computes aggregate counts from the list of findings.
func buildSummary(findings []analyzers.Finding) ScanSummary {
	summary := ScanSummary{
		Total:       len(findings),
		BySeverity:  make(map[string]int),
		ByFramework: make(map[string]int),
		ByAnalyzer:  make(map[string]int),
	}

	for _, f := range findings {
		summary.BySeverity[f.Severity]++
		summary.ByAnalyzer[f.Analyzer]++
		for _, fw := range f.Framework {
			summary.ByFramework[fw]++
		}
		if f.Fixable {
			summary.Fixable++
		}
	}

	return summary
}
