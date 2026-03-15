package reporters

import (
	"fmt"
	"strings"

	"github.com/rishav1305/soul-v2/internal/quality/compliance/scan"
)

// GenerateHTML returns a complete HTML document with inline CSS showing the scan report.
func GenerateHTML(result *scan.ScanResult) string {
	score := CalculateScore(result)

	var b strings.Builder

	b.WriteString(`<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>Compliance Scan Report</title>
<style>
  * { margin: 0; padding: 0; box-sizing: border-box; }
  body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; background: #18181b; color: #e4e4e7; padding: 2rem; }
  .container { max-width: 1200px; margin: 0 auto; }
  h1 { font-size: 1.5rem; margin-bottom: 1rem; color: #fafafa; }
  .grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(200px, 1fr)); gap: 1rem; margin-bottom: 2rem; }
  .card { background: #27272a; border-radius: 8px; padding: 1rem; }
  .card-label { font-size: 0.75rem; color: #a1a1aa; text-transform: uppercase; letter-spacing: 0.05em; }
  .card-value { font-size: 1.5rem; font-weight: 700; margin-top: 0.25rem; }
  .severity-critical { color: #ef4444; }
  .severity-high { color: #eab308; }
  .severity-medium { color: #06b6d4; }
  .severity-low { color: #e4e4e7; }
  .severity-info { color: #71717a; }
  table { width: 100%%; border-collapse: collapse; margin-top: 1rem; }
  th { text-align: left; padding: 0.75rem; background: #27272a; color: #a1a1aa; font-size: 0.75rem; text-transform: uppercase; }
  td { padding: 0.75rem; border-bottom: 1px solid #3f3f46; font-size: 0.875rem; }
  tr:hover { background: #27272a; }
  .badge { display: inline-block; padding: 0.125rem 0.5rem; border-radius: 9999px; font-size: 0.75rem; font-weight: 600; }
  .badge-critical { background: #7f1d1d; color: #fca5a5; }
  .badge-high { background: #713f12; color: #fde047; }
  .badge-medium { background: #164e63; color: #67e8f9; }
  .badge-low { background: #3f3f46; color: #e4e4e7; }
  .badge-info { background: #27272a; color: #a1a1aa; }
  .meta { color: #71717a; font-size: 0.875rem; margin-bottom: 1.5rem; }
</style>
</head>
<body>
<div class="container">
`)

	b.WriteString(`<h1>Compliance Scan Report</h1>`)
	b.WriteString(fmt.Sprintf(`<p class="meta">Target: %s | Timestamp: %s | Duration: %.2fs</p>`,
		htmlEscape(result.Metadata.Directory), htmlEscape(result.Metadata.Timestamp), result.Metadata.Duration))

	// Summary grid
	b.WriteString(`<div class="grid">`)
	b.WriteString(fmt.Sprintf(`<div class="card"><div class="card-label">Score</div><div class="card-value">%d%%</div></div>`, score))
	sev := result.Summary.BySeverity
	b.WriteString(fmt.Sprintf(`<div class="card"><div class="card-label">Critical</div><div class="card-value severity-critical">%d</div></div>`, sev["critical"]))
	b.WriteString(fmt.Sprintf(`<div class="card"><div class="card-label">High</div><div class="card-value severity-high">%d</div></div>`, sev["high"]))
	b.WriteString(fmt.Sprintf(`<div class="card"><div class="card-label">Medium</div><div class="card-value severity-medium">%d</div></div>`, sev["medium"]))
	b.WriteString(fmt.Sprintf(`<div class="card"><div class="card-label">Low</div><div class="card-value severity-low">%d</div></div>`, sev["low"]))
	b.WriteString(fmt.Sprintf(`<div class="card"><div class="card-label">Total / Fixable</div><div class="card-value">%d / %d</div></div>`, result.Summary.Total, result.Summary.Fixable))
	b.WriteString(`</div>`)

	// Findings table
	if len(result.Findings) > 0 {
		b.WriteString(`<h2 style="font-size:1.25rem; margin-bottom:0.5rem;">Findings</h2>`)
		b.WriteString(`<table><thead><tr><th>Severity</th><th>ID</th><th>Title</th><th>File</th><th>Line</th><th>Fixable</th></tr></thead><tbody>`)

		for _, f := range result.Findings {
			sev := strings.ToLower(f.Severity)
			fixable := "No"
			if f.Fixable {
				fixable = "Yes"
			}
			b.WriteString(fmt.Sprintf(`<tr><td><span class="badge badge-%s">%s</span></td><td>%s</td><td>%s</td><td>%s</td><td>%d</td><td>%s</td></tr>`,
				sev, htmlEscape(f.Severity), htmlEscape(f.ID), htmlEscape(f.Title), htmlEscape(f.File), f.Line, fixable))
		}

		b.WriteString(`</tbody></table>`)
	}

	b.WriteString(`</div></body></html>`)

	return b.String()
}

// htmlEscape performs basic HTML escaping.
func htmlEscape(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, `"`, "&quot;")
	return s
}
