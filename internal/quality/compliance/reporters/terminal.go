package reporters

import (
	"fmt"
	"strings"

	"github.com/rishav1305/soul/internal/quality/compliance/scan"
)

const (
	colorReset  = "\033[0m"
	colorRed    = "\033[31m"
	colorYellow = "\033[33m"
	colorCyan   = "\033[36m"
	colorWhite  = "\033[37m"
	colorDim    = "\033[2m"
	colorBold   = "\033[1m"
)

// GenerateTerminal returns an ANSI-colored terminal report.
func GenerateTerminal(result *scan.ScanResult) string {
	var b strings.Builder

	// Header
	b.WriteString(colorBold)
	b.WriteString("Compliance Scan Report")
	b.WriteString(colorReset)
	b.WriteString("\n")
	b.WriteString(strings.Repeat("=", 60))
	b.WriteString("\n\n")

	// Metadata
	b.WriteString(fmt.Sprintf("Target:    %s\n", result.Metadata.Directory))
	b.WriteString(fmt.Sprintf("Timestamp: %s\n", result.Metadata.Timestamp))
	b.WriteString(fmt.Sprintf("Duration:  %.2fs\n", result.Metadata.Duration))
	b.WriteString(fmt.Sprintf("Analyzers: %s\n", strings.Join(result.Metadata.AnalyzersRun, ", ")))
	b.WriteString("\n")

	// Summary
	sev := result.Summary.BySeverity
	b.WriteString(colorBold)
	b.WriteString("Summary\n")
	b.WriteString(colorReset)
	b.WriteString(strings.Repeat("-", 40))
	b.WriteString("\n")
	b.WriteString(fmt.Sprintf("  %sCritical: %d%s\n", colorRed, sev["critical"], colorReset))
	b.WriteString(fmt.Sprintf("  %sHigh:     %d%s\n", colorYellow, sev["high"], colorReset))
	b.WriteString(fmt.Sprintf("  %sMedium:   %d%s\n", colorCyan, sev["medium"], colorReset))
	b.WriteString(fmt.Sprintf("  %sLow:      %d%s\n", colorWhite, sev["low"], colorReset))
	b.WriteString(fmt.Sprintf("  %sInfo:     %d%s\n", colorDim, sev["info"], colorReset))
	b.WriteString(fmt.Sprintf("  Total:    %d (fixable: %d)\n", result.Summary.Total, result.Summary.Fixable))
	b.WriteString("\n")

	// Findings grouped by severity
	severities := []struct {
		name  string
		color string
	}{
		{"critical", colorRed},
		{"high", colorYellow},
		{"medium", colorCyan},
		{"low", colorWhite},
		{"info", colorDim},
	}

	for _, s := range severities {
		var findings []int
		for i, f := range result.Findings {
			if strings.ToLower(f.Severity) == s.name {
				findings = append(findings, i)
			}
		}
		if len(findings) == 0 {
			continue
		}

		b.WriteString(fmt.Sprintf("%s%s[%s] %d finding(s)%s\n", colorBold, s.color,
			strings.ToUpper(s.name), len(findings), colorReset))

		for _, i := range findings {
			f := result.Findings[i]
			b.WriteString(fmt.Sprintf("  %s%s%s %s\n", s.color, f.ID, colorReset, f.Title))
			b.WriteString(fmt.Sprintf("    File: %s:%d\n", f.File, f.Line))
			if f.Evidence != "" {
				b.WriteString(fmt.Sprintf("    Evidence: %s\n", f.Evidence))
			}
			if f.Fixable {
				b.WriteString("    Fixable: yes\n")
			}
			b.WriteString("\n")
		}
	}

	return b.String()
}
