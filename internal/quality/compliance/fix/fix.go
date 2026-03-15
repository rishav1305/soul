package fix

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/rishav1305/soul-v2/internal/quality/compliance/analyzers"
)

// FixResult represents the outcome of attempting to fix a single finding.
type FixResult struct {
	Finding  analyzers.Finding `json:"finding"`
	Fixed    bool              `json:"fixed"`
	Patch    string            `json:"patch"`
	Strategy string            `json:"strategy"`
	Error    string            `json:"error,omitempty"`
}

// ApplyFixes attempts to fix each fixable finding. If dryRun is true, patches
// are generated but files are not modified.
func ApplyFixes(findings []analyzers.Finding, dryRun bool) ([]FixResult, error) {
	var results []FixResult

	for _, f := range findings {
		if !f.Fixable {
			continue
		}

		strategy := selectStrategy(f)
		if strategy == "" {
			results = append(results, FixResult{
				Finding: f,
				Fixed:   false,
				Error:   "no applicable fix strategy",
			})
			continue
		}

		patch, newLine, err := generateFix(f, strategy)
		if err != nil {
			results = append(results, FixResult{
				Finding:  f,
				Fixed:    false,
				Strategy: strategy,
				Error:    err.Error(),
			})
			continue
		}

		if !dryRun && newLine != "" {
			if err := applyPatch(f.File, f.Line, newLine); err != nil {
				results = append(results, FixResult{
					Finding:  f,
					Fixed:    false,
					Patch:    patch,
					Strategy: strategy,
					Error:    err.Error(),
				})
				continue
			}
		}

		results = append(results, FixResult{
			Finding:  f,
			Fixed:    true,
			Patch:    patch,
			Strategy: strategy,
		})
	}

	return results, nil
}

// selectStrategy returns the fix strategy name based on the finding's ID prefix
// and analyzer type.
func selectStrategy(f analyzers.Finding) string {
	id := strings.ToLower(f.ID)
	analyzer := strings.ToLower(f.Analyzer)

	switch {
	case strings.HasPrefix(id, "sec-") && analyzer == "secret":
		return "secret-to-env"
	case strings.Contains(id, "hash") || strings.Contains(id, "crypto"):
		return "weak-hash-upgrade"
	case strings.Contains(id, "dangerous") || strings.Contains(id, "exec") || strings.Contains(id, "eval"):
		return "dangerous-code-removal"
	case strings.Contains(id, "cors"):
		return "cors-restrict"
	default:
		// Fallback based on analyzer
		switch analyzer {
		case "secret":
			return "secret-to-env"
		case "config":
			if strings.Contains(strings.ToLower(f.Evidence), "*") {
				return "cors-restrict"
			}
			return ""
		default:
			return ""
		}
	}
}

// generateFix produces a unified diff patch and the replacement line.
func generateFix(f analyzers.Finding, strategy string) (string, string, error) {
	oldLine := f.Evidence
	if oldLine == "" {
		return "", "", fmt.Errorf("no evidence to fix")
	}

	var newLine string
	ext := strings.ToLower(filepath.Ext(f.File))

	switch strategy {
	case "secret-to-env":
		newLine = applySecretToEnv(oldLine, ext)
	case "weak-hash-upgrade":
		newLine = applyWeakHashUpgrade(oldLine)
	case "dangerous-code-removal":
		newLine = applyDangerousCodeRemoval(oldLine, ext)
	case "cors-restrict":
		newLine = applyCorsRestrict(oldLine)
	default:
		return "", "", fmt.Errorf("unknown strategy: %s", strategy)
	}

	patch := formatUnifiedDiff(f.File, f.Line, oldLine, newLine)
	return patch, newLine, nil
}

// applySecretToEnv replaces a quoted secret value with an environment variable
// lookup appropriate for the file's language.
func applySecretToEnv(line, ext string) string {
	varName := "SECRET_VALUE"

	// Try to extract variable name from assignment
	for _, sep := range []string{"=", ":"} {
		if idx := strings.Index(line, sep); idx > 0 {
			candidate := strings.TrimSpace(line[:idx])
			// Remove leading keywords
			for _, kw := range []string{"const ", "let ", "var ", "export "} {
				candidate = strings.TrimPrefix(candidate, kw)
			}
			candidate = strings.TrimSpace(candidate)
			if candidate != "" {
				varName = strings.ToUpper(strings.ReplaceAll(strings.ReplaceAll(candidate, "-", "_"), ".", "_"))
			}
			break
		}
	}

	switch ext {
	case ".go":
		return replaceQuotedValue(line, fmt.Sprintf(`os.Getenv("%s")`, varName))
	case ".py":
		return replaceQuotedValue(line, fmt.Sprintf(`os.environ["%s"]`, varName))
	case ".js", ".ts", ".jsx", ".tsx":
		return replaceQuotedValue(line, fmt.Sprintf(`process.env.%s`, varName))
	default:
		return replaceQuotedValue(line, fmt.Sprintf(`os.Getenv("%s")`, varName))
	}
}

// replaceQuotedValue replaces the first quoted string in a line with the replacement.
func replaceQuotedValue(line, replacement string) string {
	for _, q := range []byte{'"', '\''} {
		start := strings.IndexByte(line, q)
		if start < 0 {
			continue
		}
		end := strings.IndexByte(line[start+1:], q)
		if end < 0 {
			continue
		}
		end += start + 1
		return line[:start] + replacement + line[end+1:]
	}
	return line
}

// applyWeakHashUpgrade replaces md5/sha1 with sha256 in createHash calls.
func applyWeakHashUpgrade(line string) string {
	result := line
	for _, old := range []string{"'md5'", `"md5"`, "'sha1'", `"sha1"`} {
		q := string(old[0])
		result = strings.ReplaceAll(result, old, q+"sha256"+q)
	}
	// Also handle Go-style imports or references
	result = strings.ReplaceAll(result, "crypto/md5", "crypto/sha256")
	result = strings.ReplaceAll(result, "crypto/sha1", "crypto/sha256")
	result = strings.ReplaceAll(result, "md5.New()", "sha256.New()")
	result = strings.ReplaceAll(result, "sha1.New()", "sha256.New()")
	return result
}

// applyDangerousCodeRemoval comments out the line with a TODO marker.
func applyDangerousCodeRemoval(line, ext string) string {
	trimmed := strings.TrimSpace(line)
	indent := line[:len(line)-len(strings.TrimLeft(line, " \t"))]

	switch ext {
	case ".py":
		return indent + "# TODO: dangerous code removed by compliance fix\n" + indent + "# " + trimmed
	default:
		// Go, JS, TS, etc.
		return indent + "// TODO: dangerous code removed by compliance fix\n" + indent + "// " + trimmed
	}
}

// applyCorsRestrict replaces wildcard CORS origins with a placeholder domain.
func applyCorsRestrict(line string) string {
	result := line
	result = strings.ReplaceAll(result, `"*"`, `"https://your-domain.com"`)
	result = strings.ReplaceAll(result, `'*'`, `'https://your-domain.com'`)
	return result
}

// formatUnifiedDiff produces a minimal unified diff for a single line change.
func formatUnifiedDiff(file string, lineNum int, oldLine, newLine string) string {
	return fmt.Sprintf("--- a/%s\n+++ b/%s\n@@ -%d,1 +%d,1 @@\n-%s\n+%s\n",
		file, file, lineNum, lineNum, oldLine, newLine)
}

// applyPatch writes the fixed line to the file at the specified line number.
func applyPatch(filePath string, lineNum int, newLine string) error {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("reading file: %w", err)
	}

	lines := strings.Split(string(data), "\n")
	if lineNum < 1 || lineNum > len(lines) {
		return fmt.Errorf("line %d out of range (file has %d lines)", lineNum, len(lines))
	}

	lines[lineNum-1] = newLine

	return os.WriteFile(filePath, []byte(strings.Join(lines, "\n")), 0644)
}

// ExtForFile returns the lowercase file extension.
func ExtForFile(path string) string {
	return strings.ToLower(filepath.Ext(path))
}
