package sweep

import (
	"github.com/rishav1305/soul-v2/internal/scout/store"
)

// SweepResult holds the outcome of a single-platform sweep.
type SweepResult struct {
	Platform   string   `json:"platform"`
	NewLeads   int      `json:"newLeads"`
	Duplicates int      `json:"duplicates"`
	Errors     []string `json:"errors"`
}

// Sweep runs a lead discovery sweep across the given platforms.
// For each platform, it would crawl for opportunities and deduplicate
// against existing leads by source_url. Currently a stub that returns
// empty results — actual crawling requires CDP integration.
func Sweep(platforms []string, st *store.Store) ([]SweepResult, error) {
	var results []SweepResult
	for _, p := range platforms {
		results = append(results, SweepResult{
			Platform:   p,
			NewLeads:   0,
			Duplicates: 0,
			Errors:     []string{"sweep not yet implemented — CDP integration deferred"},
		})
	}
	return results, nil
}

// SweepStatus returns the current sweep status for each known platform.
// All platforms report "idle" until real sweep scheduling is implemented.
func SweepStatus() map[string]string {
	return map[string]string{
		"linkedin":  "idle",
		"indeed":    "idle",
		"upwork":    "idle",
		"toptal":    "idle",
		"wellfound": "idle",
	}
}
