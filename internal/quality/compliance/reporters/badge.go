package reporters

import (
	"fmt"

	"github.com/rishav1305/soul/internal/quality/compliance/scan"
)

// CalculateScore computes a compliance score from 0-100 based on findings.
// Formula: 100 - (critical*10 + high*5 + medium*2 + low*1), clamped to [0,100].
func CalculateScore(result *scan.ScanResult) int {
	s := result.Summary.BySeverity
	score := 100 - (s["critical"]*10 + s["high"]*5 + s["medium"]*2 + s["low"]*1)
	if score < 0 {
		score = 0
	}
	if score > 100 {
		score = 100
	}
	return score
}

// GenerateBadge returns an SVG badge showing the compliance score.
func GenerateBadge(result *scan.ScanResult) string {
	score := CalculateScore(result)

	var color string
	switch {
	case score >= 90:
		color = "#4c1"    // green
	case score >= 70:
		color = "#dfb317" // yellow
	case score >= 50:
		color = "#fe7d37" // orange
	default:
		color = "#e05d44" // red
	}

	return fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" width="150" height="20">
  <linearGradient id="b" x2="0" y2="100%%">
    <stop offset="0" stop-color="#bbb" stop-opacity=".1"/>
    <stop offset="1" stop-opacity=".1"/>
  </linearGradient>
  <mask id="a">
    <rect width="150" height="20" rx="3" fill="#fff"/>
  </mask>
  <g mask="url(#a)">
    <rect width="80" height="20" fill="#555"/>
    <rect x="80" width="70" height="20" fill="%s"/>
    <rect width="150" height="20" fill="url(#b)"/>
  </g>
  <g fill="#fff" text-anchor="middle" font-family="DejaVu Sans,Verdana,Geneva,sans-serif" font-size="11">
    <text x="40" y="15" fill="#010101" fill-opacity=".3">compliance</text>
    <text x="40" y="14">compliance</text>
    <text x="115" y="15" fill="#010101" fill-opacity=".3">%d%%</text>
    <text x="115" y="14">%d%%</text>
  </g>
</svg>`, color, score, score)
}
