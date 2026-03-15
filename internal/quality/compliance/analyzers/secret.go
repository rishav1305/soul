package analyzers

import (
	"math"
	"os"
	"regexp"
	"strings"
)

// maxFileSize is the maximum file size to scan (500KB).
const maxFileSize = 500 * 1024

// textExtensions are the file extensions eligible for secret scanning.
var textExtensions = map[string]bool{
	"ts": true, "js": true, "py": true, "go": true, "java": true, "rb": true,
	"yaml": true, "yml": true, "json": true, "toml": true, "env": true,
	"cfg": true, "conf": true, "ini": true, "xml": true, "properties": true,
}

// secretPattern holds a compiled regex and the rule pattern it maps to.
type secretPattern struct {
	regex       *regexp.Regexp
	rulePattern string
	name        string
}

// quotedStringRe matches quoted strings of 20+ chars for entropy checking.
var quotedStringRe = regexp.MustCompile(`['"]([A-Za-z0-9+/=\-_]{20,})['"]`)

// patterns are the 16 compiled regex patterns for secret detection.
var patterns = []secretPattern{
	{regexp.MustCompile(`AKIA[0-9A-Z]{16}`), "hardcoded-credential", "AWS Access Key"},
	{regexp.MustCompile(`(?:aws_secret|AWS_SECRET|secret_key|SECRET_KEY)\s*[=:]\s*['"]?([A-Za-z0-9/+=]{40})['"]?`), "hardcoded-credential", "AWS Secret Key"},
	{regexp.MustCompile(`gh[ps]_[A-Za-z0-9_]{36,}`), "api-token", "GitHub Token"},
	{regexp.MustCompile(`-----BEGIN\s+(?:RSA|EC|DSA|PGP)?\s*PRIVATE KEY-----`), "private-key", "Private Key"},
	{regexp.MustCompile(`eyJ[A-Za-z0-9\-_]+\.eyJ[A-Za-z0-9\-_]+\.[A-Za-z0-9\-_.+/=]+`), "api-token", "JWT Token"},
	{regexp.MustCompile(`xox[bpras]-[0-9a-zA-Z-]+`), "api-token", "Slack Token"},
	{regexp.MustCompile(`sk_(?:live|test)_[0-9a-zA-Z]{24,}`), "api-token", "Stripe Key"},
	{regexp.MustCompile(`sk-ant-[a-zA-Z0-9-]+`), "api-token", "Anthropic Key"},
	{regexp.MustCompile(`(?i)(?:password|passwd|pwd|secret)\s*[=:]\s*['"][^'"]{4,}['"]`), "hardcoded-credential", "Generic Password"},
	{regexp.MustCompile(`(?i)(?:api[_\-]?key|apikey)\s*[=:]\s*['"][^'"]{8,}['"]`), "hardcoded-credential", "Generic API Key"},
	{regexp.MustCompile(`(?:mongodb|postgres|mysql|redis)://[^\s'"]+:[^\s'"]+@`), "hardcoded-credential", "Database URL"},
	{regexp.MustCompile(`AIza[0-9A-Za-z\-_]{35}`), "api-token", "Google API Key"},
	{regexp.MustCompile(`[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}`), "api-token", "Heroku API Key"},
	{regexp.MustCompile(`SG\.[0-9A-Za-z\-_]{22,}\.[0-9A-Za-z\-_]{22,}`), "api-token", "SendGrid API Key"},
	{regexp.MustCompile(`SK[0-9a-fA-F]{32}`), "api-token", "Twilio API Key"},
	{regexp.MustCompile(`key-[0-9a-zA-Z]{32}`), "api-token", "Mailgun API Key"},
}

// SecretScanner detects hardcoded secrets, API tokens, private keys, and
// high-entropy strings in source files.
type SecretScanner struct{}

// Name returns the analyzer name.
func (s *SecretScanner) Name() string { return "secret-scanner" }

// Analyze scans files for secrets using regex patterns and entropy detection.
func (s *SecretScanner) Analyze(files []ScannedFile, rules []Rule) ([]Finding, error) {
	// Build rule lookup by pattern field.
	rulesByPattern := make(map[string][]Rule)
	for _, r := range rules {
		if r.Analyzer == "secret-scanner" {
			rulesByPattern[r.Pattern] = append(rulesByPattern[r.Pattern], r)
		}
	}

	var findings []Finding

	for _, f := range files {
		if !isTextFile(f.Extension) || f.Size > maxFileSize {
			continue
		}

		data, err := os.ReadFile(f.Path)
		if err != nil {
			continue
		}

		lines := strings.Split(string(data), "\n")
		for lineNum, line := range lines {
			// Pattern-based detection.
			for _, p := range patterns {
				match := p.regex.FindString(line)
				if match == "" {
					continue
				}

				finding := Finding{
					Title:    p.name,
					Analyzer: "secret-scanner",
					File:     f.RelativePath,
					Line:     lineNum + 1,
					Column:   strings.Index(line, match) + 1,
					Evidence: redact(match),
				}

				// Enrich from matching rules.
				if matched, ok := rulesByPattern[p.rulePattern]; ok && len(matched) > 0 {
					r := matched[0]
					finding.ID = r.ID
					finding.Description = r.Description
					finding.Severity = r.Severity
					finding.Framework = r.Framework
					finding.ControlIDs = r.Controls
					finding.Fixable = r.Fixable
				} else {
					finding.ID = "secret-" + p.rulePattern
					finding.Description = "Detected " + p.name
					finding.Severity = "high"
				}

				findings = append(findings, finding)
			}

			// High-entropy string detection.
			for _, submatch := range quotedStringRe.FindAllStringSubmatch(line, -1) {
				if len(submatch) < 2 {
					continue
				}
				val := submatch[1]
				threshold := entropyThreshold(val)
				if shannonEntropy(val) >= threshold {
					finding := Finding{
						Title:    "High-Entropy String",
						Analyzer: "secret-scanner",
						File:     f.RelativePath,
						Line:     lineNum + 1,
						Column:   strings.Index(line, submatch[0]) + 1,
						Evidence: redact(val),
					}

					if matched, ok := rulesByPattern["high-entropy"]; ok && len(matched) > 0 {
						r := matched[0]
						finding.ID = r.ID
						finding.Description = r.Description
						finding.Severity = r.Severity
						finding.Framework = r.Framework
						finding.ControlIDs = r.Controls
						finding.Fixable = r.Fixable
					} else {
						finding.ID = "secret-high-entropy"
						finding.Description = "High-entropy string detected"
						finding.Severity = "medium"
					}

					findings = append(findings, finding)
				}
			}
		}
	}

	return findings, nil
}

// shannonEntropy calculates Shannon entropy of a string in bits per character.
func shannonEntropy(s string) float64 {
	if len(s) == 0 {
		return 0
	}

	freq := make(map[rune]int)
	for _, c := range s {
		freq[c]++
	}

	length := float64(len([]rune(s)))
	var entropy float64
	for _, count := range freq {
		p := float64(count) / length
		if p > 0 {
			entropy -= p * math.Log2(p)
		}
	}

	return entropy
}

// entropyThreshold returns the entropy threshold based on the character set.
func entropyThreshold(s string) float64 {
	isHex := true
	for _, c := range s {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			isHex = false
			break
		}
	}
	if isHex {
		return 4.5
	}
	return 5.0
}

// redact masks a secret string, showing only the first 4 and last 4 characters.
func redact(s string) string {
	if len(s) < 8 {
		return strings.Repeat("*", len(s))
	}
	return s[:4] + "****" + s[len(s)-4:]
}

// isTextFile checks if the extension is in the allowed text extensions set.
func isTextFile(ext string) bool {
	return textExtensions[ext]
}
