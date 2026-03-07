package reporters

import (
	"fmt"
	"html"
	"strings"

	"github.com/rishav1305/soul/products/compliance/scan"
)

// GenerateHTML returns a standalone HTML page containing the scan results
// with inline CSS styling, a summary table, findings table, and score display.
func GenerateHTML(result *scan.ScanResult) string {
	score := CalculateScore(result)
	scoreColor := "#e05d44"
	switch {
	case score >= 90:
		scoreColor = "#4c1"
	case score >= 70:
		scoreColor = "#dfb317"
	case score >= 50:
		scoreColor = "#fe7d37"
	}

	var b strings.Builder

	b.WriteString(`<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>Compliance Scan Report</title>
<style>
  * { margin: 0; padding: 0; box-sizing: border-box; }
  body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; background: #f5f5f5; color: #333; padding: 2rem; }
  .container { max-width: 1200px; margin: 0 auto; }
  h1 { color: #1a1a2e; margin-bottom: 1rem; }
  h2 { color: #16213e; margin: 1.5rem 0 0.5rem; }
  .score-badge { display: inline-block; padding: 0.5rem 1.5rem; border-radius: 8px; color: white; font-size: 1.5rem; font-weight: bold; margin: 1rem 0; }
  .meta { background: white; border-radius: 8px; padding: 1rem 1.5rem; margin-bottom: 1.5rem; box-shadow: 0 1px 3px rgba(0,0,0,0.1); }
  .meta p { margin: 0.25rem 0; color: #555; }
  .meta strong { color: #333; }
  table { width: 100%; border-collapse: collapse; background: white; border-radius: 8px; overflow: hidden; box-shadow: 0 1px 3px rgba(0,0,0,0.1); margin-bottom: 1.5rem; }
  th { background: #1a1a2e; color: white; padding: 0.75rem 1rem; text-align: left; font-weight: 600; }
  td { padding: 0.75rem 1rem; border-bottom: 1px solid #eee; }
  tr:last-child td { border-bottom: none; }
  tr:hover td { background: #f8f9fa; }
  .severity-critical { color: #e05d44; font-weight: bold; }
  .severity-high { color: #dfb317; font-weight: bold; }
  .severity-medium { color: #0e7490; font-weight: bold; }
  .severity-low { color: #666; }
  .severity-info { color: #999; }
  .fixable { color: #4c1; font-size: 0.85rem; }
  .evidence { font-family: monospace; font-size: 0.85rem; color: #666; max-width: 300px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .summary-grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(150px, 1fr)); gap: 1rem; margin-bottom: 1.5rem; }
  .summary-card { background: white; border-radius: 8px; padding: 1rem; text-align: center; box-shadow: 0 1px 3px rgba(0,0,0,0.1); }
  .summary-card .count { font-size: 2rem; font-weight: bold; }
  .summary-card .label { font-size: 0.85rem; color: #666; text-transform: uppercase; }
  footer { text-align: center; color: #999; margin-top: 2rem; font-size: 0.85rem; }
</style>
</head>
<body>
<div class="container">
`)

	// Header and score
	b.WriteString(`<h1>Compliance Scan Report</h1>`)
	b.WriteString(fmt.Sprintf(`<div class="score-badge" style="background:%s">Score: %d%%</div>`,
		scoreColor, score))

	// Metadata
	b.WriteString(`<div class="meta">`)
	b.WriteString(fmt.Sprintf(`<p><strong>Directory:</strong> %s</p>`, html.EscapeString(result.Metadata.Directory)))
	b.WriteString(fmt.Sprintf(`<p><strong>Duration:</strong> %.2fs</p>`, result.Metadata.Duration))
	b.WriteString(fmt.Sprintf(`<p><strong>Analyzers:</strong> %s</p>`, html.EscapeString(strings.Join(result.Metadata.AnalyzersRun, ", "))))
	b.WriteString(fmt.Sprintf(`<p><strong>Timestamp:</strong> %s</p>`, html.EscapeString(result.Metadata.Timestamp)))
	b.WriteString(`</div>`)

	// Summary cards
	b.WriteString(`<h2>Summary</h2>`)
	b.WriteString(`<div class="summary-grid">`)
	b.WriteString(fmt.Sprintf(`<div class="summary-card"><div class="count">%d</div><div class="label">Total</div></div>`, result.Summary.Total))
	b.WriteString(fmt.Sprintf(`<div class="summary-card"><div class="count">%d</div><div class="label">Fixable</div></div>`, result.Summary.Fixable))

	for _, sev := range severityOrder {
		count := result.Summary.BySeverity[sev]
		if count == 0 {
			continue
		}
		b.WriteString(fmt.Sprintf(`<div class="summary-card"><div class="count severity-%s">%d</div><div class="label">%s</div></div>`,
			sev, count, strings.ToUpper(sev)))
	}
	b.WriteString(`</div>`)

	// Findings table
	if result.Summary.Total > 0 {
		b.WriteString(`<h2>Findings</h2>`)
		b.WriteString(`<table>`)
		b.WriteString(`<thead><tr><th>ID</th><th>Severity</th><th>Title</th><th>File</th><th>Line</th><th>Evidence</th><th>Fixable</th></tr></thead>`)
		b.WriteString(`<tbody>`)

		for _, f := range result.Findings {
			fixLabel := ""
			if f.Fixable {
				fixLabel = `<span class="fixable">Yes</span>`
			}
			location := html.EscapeString(f.File)
			lineStr := ""
			if f.Line > 0 {
				lineStr = fmt.Sprintf("%d", f.Line)
			}

			b.WriteString(fmt.Sprintf(`<tr><td>%s</td><td class="severity-%s">%s</td><td>%s</td><td>%s</td><td>%s</td><td class="evidence">%s</td><td>%s</td></tr>`,
				html.EscapeString(f.ID),
				strings.ToLower(f.Severity),
				html.EscapeString(strings.ToUpper(f.Severity)),
				html.EscapeString(f.Title),
				location,
				lineStr,
				html.EscapeString(f.Evidence),
				fixLabel,
			))
		}

		b.WriteString(`</tbody></table>`)
	}

	// Footer
	b.WriteString(`<footer>Generated by compliance-go</footer>`)
	b.WriteString(`</div></body></html>`)

	return b.String()
}
