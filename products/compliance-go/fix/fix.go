package fix

import (
	"fmt"
	"os"
	"strings"

	"github.com/rishav1305/soul/products/compliance-go/analyzers"
)

// FixResult holds the outcome of applying a fix to a single finding.
type FixResult struct {
	Finding  analyzers.Finding `json:"finding"`
	Fixed    bool              `json:"fixed"`
	Patch    string            `json:"patch,omitempty"`
	Strategy string            `json:"strategy"`
	Error    string            `json:"error,omitempty"`
}

// strategyFunc is the signature for all fix strategies.
// It takes a Finding and returns a unified diff patch string.
type strategyFunc func(finding analyzers.Finding) (patch string, newContent string, err error)

// selectStrategy picks the appropriate fix strategy based on finding ID prefix
// and analyzer/evidence metadata.
func selectStrategy(f analyzers.Finding) (name string, fn strategyFunc) {
	// SECRET-* findings -> replace hardcoded value with env var reference
	if strings.HasPrefix(f.ID, "SECRET-") {
		return "secret-to-env", secretToEnvStrategy
	}

	// CRYPTO-001 (weak-hash from ast-analyzer) -> upgrade to sha256
	if f.ID == "CRYPTO-001" || (strings.HasPrefix(f.ID, "CRYPTO-") && f.Analyzer == "ast-analyzer" &&
		(strings.Contains(strings.ToLower(f.Evidence), "md5") || strings.Contains(strings.ToLower(f.Evidence), "sha1"))) {
		return "weak-hash-upgrade", weakHashStrategy
	}

	// AST-* or any ast-analyzer finding with eval pattern -> comment out eval
	if strings.HasPrefix(f.ID, "AST-") && strings.Contains(strings.ToLower(f.Evidence), "eval") {
		return "eval-removal", evalRemovalStrategy
	}
	// Also match by analyzer + evidence for eval-usage pattern findings
	if f.Analyzer == "ast-analyzer" && strings.Contains(strings.ToLower(f.Evidence), "eval") {
		return "eval-removal", evalRemovalStrategy
	}

	// CONFIG-* or config-checker with CORS wildcard -> restrict origin
	if strings.HasPrefix(f.ID, "CONFIG-") && strings.Contains(strings.ToLower(f.Evidence), "cors") {
		return "cors-restrict", corsStrategy
	}
	// Also match CRYPTO-008 (wildcard CORS from config-checker)
	if f.Analyzer == "config-checker" && (strings.Contains(strings.ToLower(f.Evidence), "origin") ||
		strings.Contains(strings.ToLower(f.Title), "cors")) {
		return "cors-restrict", corsStrategy
	}

	return "", nil
}

// ApplyFixes iterates findings where Fixable is true, selects a strategy for
// each, generates a unified diff patch, and optionally applies the fix to the
// file.
//
// If dryRun is true, patches are generated but files are not modified.
// If dryRun is false, patches are generated and applied to the files.
func ApplyFixes(findings []analyzers.Finding, dryRun bool) ([]FixResult, error) {
	var results []FixResult

	for _, f := range findings {
		if !f.Fixable {
			continue
		}

		strategyName, strategyFn := selectStrategy(f)
		if strategyFn == nil {
			// No strategy available for this finding type; skip
			continue
		}

		patch, newContent, err := strategyFn(f)
		if err != nil {
			results = append(results, FixResult{
				Finding:  f,
				Fixed:    false,
				Strategy: strategyName,
				Error:    err.Error(),
			})
			continue
		}

		// Apply the fix if not a dry run
		if !dryRun && newContent != "" {
			if writeErr := os.WriteFile(f.File, []byte(newContent), 0644); writeErr != nil {
				results = append(results, FixResult{
					Finding:  f,
					Fixed:    false,
					Patch:    patch,
					Strategy: strategyName,
					Error:    fmt.Sprintf("failed to write file: %v", writeErr),
				})
				continue
			}
		}

		results = append(results, FixResult{
			Finding:  f,
			Fixed:    true,
			Patch:    patch,
			Strategy: strategyName,
		})
	}

	return results, nil
}
