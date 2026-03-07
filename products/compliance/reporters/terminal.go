package reporters

import (
	"fmt"
	"sort"
	"strings"

	"github.com/rishav1305/soul/products/compliance/analyzers"
	"github.com/rishav1305/soul/products/compliance/scan"
)

// ANSI color codes for terminal output.
const (
	colorReset  = "\033[0m"
	colorRed    = "\033[31m"
	colorYellow = "\033[33m"
	colorCyan   = "\033[36m"
	colorWhite  = "\033[37m"
	colorDim    = "\033[2m"
	colorBold   = "\033[1m"
)

// severityOrder defines the display order for severities.
var severityOrder = []string{"critical", "high", "medium", "low", "info"}

// severityColor returns the ANSI color code for a given severity level.
func severityColor(severity string) string {
	switch strings.ToLower(severity) {
	case "critical":
		return colorRed
	case "high":
		return colorYellow
	case "medium":
		return colorCyan
	case "low":
		return colorWhite
	case "info":
		return colorDim
	default:
		return colorReset
	}
}

// FormatTerminal formats scan results with ANSI colors for terminal display.
// It includes a header, summary counts, findings grouped by severity, and score.
func FormatTerminal(result *scan.ScanResult) string {
	var b strings.Builder

	// Header
	b.WriteString(fmt.Sprintf("\n%s%s=== Compliance Scan Results ===%s\n\n",
		colorBold, colorCyan, colorReset))

	// Metadata
	b.WriteString(fmt.Sprintf("  Directory:  %s\n", result.Metadata.Directory))
	b.WriteString(fmt.Sprintf("  Duration:   %.2fs\n", result.Metadata.Duration))
	b.WriteString(fmt.Sprintf("  Analyzers:  %s\n", strings.Join(result.Metadata.AnalyzersRun, ", ")))
	b.WriteString(fmt.Sprintf("  Timestamp:  %s\n\n", result.Metadata.Timestamp))

	// Summary counts
	b.WriteString(fmt.Sprintf("%s%sSummary%s\n", colorBold, colorWhite, colorReset))
	b.WriteString(fmt.Sprintf("  Total findings: %d\n", result.Summary.Total))
	b.WriteString(fmt.Sprintf("  Fixable:        %d\n", result.Summary.Fixable))

	for _, sev := range severityOrder {
		count, ok := result.Summary.BySeverity[sev]
		if !ok || count == 0 {
			continue
		}
		color := severityColor(sev)
		b.WriteString(fmt.Sprintf("  %s%-10s%s %d\n", color, strings.ToUpper(sev), colorReset, count))
	}
	b.WriteString("\n")

	// Score
	score := CalculateScore(result)
	scoreColor := colorRed
	if score >= 90 {
		scoreColor = "\033[32m" // green
	} else if score >= 70 {
		scoreColor = colorYellow
	} else if score >= 50 {
		scoreColor = "\033[38;5;208m" // orange
	}
	b.WriteString(fmt.Sprintf("  %sCompliance Score: %d%%%s\n\n", scoreColor, score, colorReset))

	// Findings grouped by severity
	if result.Summary.Total > 0 {
		b.WriteString(fmt.Sprintf("%s%sFindings%s\n", colorBold, colorWhite, colorReset))

		// Group findings by severity
		grouped := make(map[string][]analyzers.Finding)
		for _, f := range result.Findings {
			sev := strings.ToLower(f.Severity)
			grouped[sev] = append(grouped[sev], f)
		}

		for _, sev := range severityOrder {
			findings, ok := grouped[sev]
			if !ok || len(findings) == 0 {
				continue
			}

			color := severityColor(sev)
			b.WriteString(fmt.Sprintf("\n  %s%s--- %s (%d) ---%s\n",
				colorBold, color, strings.ToUpper(sev), len(findings), colorReset))

			// Sort findings by file and line within each severity
			sort.Slice(findings, func(i, j int) bool {
				if findings[i].File != findings[j].File {
					return findings[i].File < findings[j].File
				}
				return findings[i].Line < findings[j].Line
			})

			for _, f := range findings {
				fixLabel := ""
				if f.Fixable {
					fixLabel = " [fixable]"
				}
				b.WriteString(fmt.Sprintf("  %s[%s]%s %s%s\n",
					color, f.ID, colorReset, f.Title, fixLabel))
				if f.File != "" {
					location := f.File
					if f.Line > 0 {
						location = fmt.Sprintf("%s:%d", f.File, f.Line)
					}
					b.WriteString(fmt.Sprintf("    %sFile: %s%s\n", colorDim, location, colorReset))
				}
				if f.Evidence != "" {
					b.WriteString(fmt.Sprintf("    %sEvidence: %s%s\n", colorDim, f.Evidence, colorReset))
				}
			}
		}
		b.WriteString("\n")
	} else {
		b.WriteString(fmt.Sprintf("  %sNo findings detected. Great job!%s\n\n",
			"\033[32m", colorReset))
	}

	return b.String()
}
