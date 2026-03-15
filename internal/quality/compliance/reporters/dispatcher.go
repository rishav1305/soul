package reporters

import (
	"fmt"

	"github.com/rishav1305/soul-v2/internal/quality/compliance/scan"
)

// Generate produces a report in the specified format.
// Supported formats: "json", "terminal", "html".
func Generate(result *scan.ScanResult, format string) (string, error) {
	switch format {
	case "json":
		return GenerateJSON(result)
	case "terminal":
		return GenerateTerminal(result), nil
	case "html":
		return GenerateHTML(result), nil
	default:
		return "", fmt.Errorf("unknown report format: %q", format)
	}
}
