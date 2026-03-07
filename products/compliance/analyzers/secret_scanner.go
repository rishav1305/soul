package analyzers

import (
	"fmt"
	"math"
	"os"
	"regexp"
	"strings"

	"github.com/rishav1305/soul/products/compliance/rules"
)

// secretPattern links a compiled regex to a rule pattern identifier.
type secretPattern struct {
	name        string
	regex       *regexp.Regexp
	rulePattern string
}

// textExtensions lists file extensions that the scanner will inspect.
var textExtensions = map[string]bool{
	"ts":         true,
	"js":         true,
	"py":         true,
	"go":         true,
	"java":       true,
	"rb":         true,
	"yaml":       true,
	"yml":        true,
	"json":       true,
	"toml":       true,
	"env":        true,
	"cfg":        true,
	"conf":       true,
	"ini":        true,
	"xml":        true,
	"properties": true,
}

const maxFileSize int64 = 500 * 1024 // 500 KB

// secretPatterns contains all 16 regex patterns compiled at package init time.
var secretPatterns = []secretPattern{
	// 1. AWS Access Key
	{
		name:        "AWS Access Key",
		regex:       regexp.MustCompile(`AKIA[0-9A-Z]{16}`),
		rulePattern: "hardcoded-credential",
	},
	// 2. AWS Secret Key
	{
		name:        "AWS Secret Key",
		regex:       regexp.MustCompile(`(?:aws_secret|AWS_SECRET|secret_key|SECRET_KEY)\s*[=:]\s*['"]?([A-Za-z0-9/+=]{40})['"]?`),
		rulePattern: "hardcoded-credential",
	},
	// 3. GitHub Token
	{
		name:        "GitHub Token",
		regex:       regexp.MustCompile(`gh[ps]_[A-Za-z0-9_]{36,}`),
		rulePattern: "api-token",
	},
	// 4. Private Key
	{
		name:        "Private Key",
		regex:       regexp.MustCompile(`-----BEGIN\s+(?:RSA|EC|DSA|PGP)?\s*PRIVATE KEY-----`),
		rulePattern: "private-key",
	},
	// 5. JWT Token
	{
		name:        "JWT Token",
		regex:       regexp.MustCompile(`eyJ[A-Za-z0-9\-_]+\.eyJ[A-Za-z0-9\-_]+\.[A-Za-z0-9\-_.+/=]+`),
		rulePattern: "api-token",
	},
	// 6. Slack Token
	{
		name:        "Slack Token",
		regex:       regexp.MustCompile(`xox[bpras]-[0-9a-zA-Z-]+`),
		rulePattern: "api-token",
	},
	// 7. Stripe Key
	{
		name:        "Stripe Key",
		regex:       regexp.MustCompile(`sk_(?:live|test)_[0-9a-zA-Z]{24,}`),
		rulePattern: "api-token",
	},
	// 8. Anthropic Key
	{
		name:        "Anthropic Key",
		regex:       regexp.MustCompile(`sk-ant-[a-zA-Z0-9-]+`),
		rulePattern: "api-token",
	},
	// 9. Generic Password (case insensitive)
	{
		name:        "Generic Password",
		regex:       regexp.MustCompile(`(?i)(?:password|passwd|pwd|secret)\s*[=:]\s*['"][^'"]{4,}['"]`),
		rulePattern: "hardcoded-credential",
	},
	// 10. Generic API Key (case insensitive)
	{
		name:        "Generic API Key",
		regex:       regexp.MustCompile(`(?i)(?:api[_\-]?key|apikey)\s*[=:]\s*['"][^'"]{8,}['"]`),
		rulePattern: "hardcoded-credential",
	},
	// 11. Database URL
	{
		name:        "Database URL",
		regex:       regexp.MustCompile(`(?:mongodb|postgres|mysql|redis)://[^\s'"]+:[^\s'"]+@`),
		rulePattern: "hardcoded-credential",
	},
	// 12. Google API Key
	{
		name:        "Google API Key",
		regex:       regexp.MustCompile(`AIza[0-9A-Za-z\-_]{35}`),
		rulePattern: "api-token",
	},
	// 13. Heroku API Key (UUID)
	{
		name:        "Heroku API Key",
		regex:       regexp.MustCompile(`[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}`),
		rulePattern: "api-token",
	},
	// 14. SendGrid API Key
	{
		name:        "SendGrid API Key",
		regex:       regexp.MustCompile(`SG\.[0-9A-Za-z\-_]{22,}\.[0-9A-Za-z\-_]{22,}`),
		rulePattern: "api-token",
	},
	// 15. Twilio API Key
	{
		name:        "Twilio API Key",
		regex:       regexp.MustCompile(`SK[0-9a-fA-F]{32}`),
		rulePattern: "api-token",
	},
	// 16. Mailgun API Key
	{
		name:        "Mailgun API Key",
		regex:       regexp.MustCompile(`key-[0-9a-zA-Z]{32}`),
		rulePattern: "api-token",
	},
}

// highEntropyRegex matches quoted strings of 20+ alphanumeric/base64 characters.
var highEntropyRegex = regexp.MustCompile(`['"]([A-Za-z0-9+/=\-_]{20,})['"]`)

// hexLikeRegex checks if a string is composed entirely of hex characters.
var hexLikeRegex = regexp.MustCompile(`^[0-9a-fA-F]+$`)

// base64LikeRegex checks if a string is composed entirely of base64 characters.
var base64LikeRegex = regexp.MustCompile(`^[A-Za-z0-9+/=\-_]+$`)

// matchResult holds a single pattern match within a file.
type matchResult struct {
	patternName string
	rulePattern string
	matched     string
	line        int
	column      int
}

// SecretScanner implements the Analyzer interface for detecting hardcoded
// secrets, credentials, API tokens, and high-entropy strings.
type SecretScanner struct{}

// Name returns the analyzer identifier used to match rules.
func (s *SecretScanner) Name() string {
	return "secret-scanner"
}

// Analyze scans the provided files for secrets using regex patterns and
// Shannon entropy detection, returning findings mapped to the relevant rules.
func (s *SecretScanner) Analyze(files []ScannedFile, allRules []rules.Rule) ([]Finding, error) {
	myRules := filterRules(allRules, s.Name())
	if len(myRules) == 0 {
		return nil, nil
	}

	rulesByPattern := groupRulesByPattern(myRules)
	var findings []Finding

	for _, file := range files {
		// Skip non-text files
		if !isTextExtension(file.Extension) {
			continue
		}
		// Skip large files
		if file.Size > maxFileSize {
			continue
		}

		content, err := os.ReadFile(file.Path)
		if err != nil {
			// Skip files that can't be read
			continue
		}

		// Quick binary check: if content contains null bytes, skip
		if containsNullByte(content) {
			continue
		}

		text := string(content)
		lines := strings.Split(text, "\n")
		var matches []matchResult

		for lineIdx, line := range lines {
			// Run each secret pattern against the line
			for _, pat := range secretPatterns {
				locs := pat.regex.FindAllStringIndex(line, -1)
				for _, loc := range locs {
					matches = append(matches, matchResult{
						patternName: pat.name,
						rulePattern: pat.rulePattern,
						matched:     line[loc[0]:loc[1]],
						line:        lineIdx + 1,
						column:      loc[0] + 1,
					})
				}
			}

			// Check for high-entropy strings
			heMatches := detectHighEntropyStrings(line)
			for _, heMatch := range heMatches {
				// Avoid duplicate findings: skip if already matched by a specific pattern
				alreadyMatched := false
				for _, m := range matches {
					if m.line == lineIdx+1 && strings.Contains(line, m.matched) && strings.Contains(heMatch, m.matched) {
						alreadyMatched = true
						break
					}
				}
				if !alreadyMatched {
					col := strings.Index(line, heMatch)
					matches = append(matches, matchResult{
						patternName: "High-entropy string",
						rulePattern: "high-entropy",
						matched:     heMatch,
						line:        lineIdx + 1,
						column:      col + 1,
					})
				}
			}
		}

		// Convert matches to findings
		for _, m := range matches {
			matchedRules, ok := rulesByPattern[m.rulePattern]
			if !ok || len(matchedRules) == 0 {
				continue
			}

			for _, rule := range matchedRules {
				findings = append(findings, Finding{
					ID:          rule.ID,
					Title:       rule.Title,
					Description: fmt.Sprintf("%s (%s: %s)", rule.Description, m.patternName, redact(m.matched)),
					Severity:    rule.Severity,
					Framework:   rule.Framework,
					ControlIDs:  rule.Controls,
					File:        file.RelativePath,
					Line:        m.line,
					Column:      m.column,
					Evidence:    redact(m.matched),
					Analyzer:    s.Name(),
					Fixable:     rule.Fixable,
				})
			}
		}
	}

	return findings, nil
}

// filterRules returns only the rules that belong to the named analyzer.
func filterRules(allRules []rules.Rule, analyzerName string) []rules.Rule {
	var filtered []rules.Rule
	for _, r := range allRules {
		if r.Analyzer == analyzerName {
			filtered = append(filtered, r)
		}
	}
	return filtered
}

// groupRulesByPattern builds a map from pattern string to the rules that use it.
func groupRulesByPattern(ruleList []rules.Rule) map[string][]rules.Rule {
	m := make(map[string][]rules.Rule)
	for _, r := range ruleList {
		m[r.Pattern] = append(m[r.Pattern], r)
	}
	return m
}

// redact keeps the first 4 and last 4 characters, replacing the middle with ****.
// If the string is 8 characters or fewer, it returns "****".
func redact(secret string) string {
	if len(secret) <= 8 {
		return "****"
	}
	return secret[:4] + "****" + secret[len(secret)-4:]
}

// isTextExtension checks whether the extension is one that should be scanned.
func isTextExtension(ext string) bool {
	return textExtensions[strings.ToLower(ext)]
}

// containsNullByte checks if the content contains any null bytes (binary indicator).
func containsNullByte(data []byte) bool {
	for _, b := range data {
		if b == 0 {
			return true
		}
	}
	return false
}

// shannonEntropy computes the Shannon entropy of a string.
func shannonEntropy(s string) float64 {
	if len(s) == 0 {
		return 0
	}
	freq := make(map[rune]int)
	for _, ch := range s {
		freq[ch]++
	}
	length := float64(len([]rune(s)))
	entropy := 0.0
	for _, count := range freq {
		p := float64(count) / length
		entropy -= p * math.Log2(p)
	}
	return entropy
}

// isHexLike checks if a string is composed entirely of hex characters.
func isHexLike(s string) bool {
	return hexLikeRegex.MatchString(s)
}

// isBase64Like checks if a string is composed entirely of base64 characters.
func isBase64Like(s string) bool {
	return base64LikeRegex.MatchString(s)
}

// detectHighEntropyStrings finds quoted strings with high Shannon entropy
// that may be secrets.
func detectHighEntropyStrings(line string) []string {
	var results []string
	submatches := highEntropyRegex.FindAllStringSubmatchIndex(line, -1)
	for _, loc := range submatches {
		// loc[2] and loc[3] are the start and end of capture group 1
		candidate := line[loc[2]:loc[3]]
		if len(candidate) < 20 {
			continue
		}

		var threshold float64
		if isHexLike(candidate) {
			threshold = 4.5
		} else if isBase64Like(candidate) {
			threshold = 5.0
		} else {
			threshold = 5.0
		}

		if shannonEntropy(candidate) > threshold {
			results = append(results, candidate)
		}
	}
	return results
}
