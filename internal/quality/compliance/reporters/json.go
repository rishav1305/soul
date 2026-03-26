package reporters

import (
	"encoding/json"

	"github.com/rishav1305/soul/internal/quality/compliance/scan"
)

// GenerateJSON returns the scan result as pretty-printed JSON.
func GenerateJSON(result *scan.ScanResult) (string, error) {
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}
