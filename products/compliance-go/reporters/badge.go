package reporters

import (
	"fmt"
	"strings"

	"github.com/rishav1305/soul/products/compliance-go/scan"
)

// CalculateScore computes a compliance score starting at 100 and subtracting
// points for each finding based on severity:
//   - critical: -10
//   - high:     -5
//   - medium:   -2
//   - low:      -1
//   - info:      0
//
// The score is clamped to a minimum of 0.
func CalculateScore(result *scan.ScanResult) int {
	score := 100
	for _, f := range result.Findings {
		switch strings.ToLower(f.Severity) {
		case "critical":
			score -= 10
		case "high":
			score -= 5
		case "medium":
			score -= 2
		case "low":
			score -= 1
		}
	}
	if score < 0 {
		score = 0
	}
	return score
}

// badgeColor returns a hex color string based on the score value.
//
//	score >= 90 -> green  (#4c1)
//	score >= 70 -> yellow (#dfb317)
//	score >= 50 -> orange (#fe7d37)
//	score <  50 -> red    (#e05d44)
func badgeColor(score int) string {
	switch {
	case score >= 90:
		return "#4c1"
	case score >= 70:
		return "#dfb317"
	case score >= 50:
		return "#fe7d37"
	default:
		return "#e05d44"
	}
}

// GenerateBadge returns an SVG badge string showing the compliance score
// with an appropriate color.
func GenerateBadge(result *scan.ScanResult) string {
	score := CalculateScore(result)
	color := badgeColor(score)
	scoreText := fmt.Sprintf("%d%%", score)

	// Calculate text widths for SVG positioning
	labelWidth := 90
	valueWidth := 50
	totalWidth := labelWidth + valueWidth

	svg := fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" width="%d" height="20">
  <linearGradient id="b" x2="0" y2="100%%">
    <stop offset="0" stop-color="#bbb" stop-opacity=".1"/>
    <stop offset="1" stop-opacity=".1"/>
  </linearGradient>
  <clipPath id="a">
    <rect width="%d" height="20" rx="3" fill="#fff"/>
  </clipPath>
  <g clip-path="url(#a)">
    <path fill="#555" d="M0 0h%dv20H0z"/>
    <path fill="%s" d="M%d 0h%dv20H%dz"/>
    <path fill="url(#b)" d="M0 0h%dv20H0z"/>
  </g>
  <g fill="#fff" text-anchor="middle" font-family="DejaVu Sans,Verdana,Geneva,sans-serif" font-size="11">
    <text x="%d" y="15" fill="#010101" fill-opacity=".3">compliance</text>
    <text x="%d" y="14">compliance</text>
    <text x="%d" y="15" fill="#010101" fill-opacity=".3">%s</text>
    <text x="%d" y="14">%s</text>
  </g>
</svg>`,
		totalWidth,
		totalWidth,
		labelWidth,
		color,
		labelWidth, valueWidth, labelWidth,
		totalWidth,
		labelWidth/2, // label text x
		labelWidth/2, // label text x (shadow)
		labelWidth+valueWidth/2, // value text x
		scoreText,
		labelWidth+valueWidth/2, // value text x (shadow)
		scoreText,
	)

	return svg
}
