package reporters

import (
	"encoding/json"

	"github.com/rishav1305/soul/products/compliance/scan"
)

// FormatJSON serializes the scan result to a pretty-printed JSON string.
func FormatJSON(result *scan.ScanResult) (string, error) {
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}
