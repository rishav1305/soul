package analyzers

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/rishav1305/soul/products/compliance/rules"
)

// ASTAnalyzer implements the Analyzer interface for regex-based source code
// anti-pattern detection. Despite the name, it uses regex patterns rather
// than actual AST parsing.
type ASTAnalyzer struct{}

// Name returns the analyzer identifier used to match rules.
func (a *ASTAnalyzer) Name() string {
	return "ast-analyzer"
}

// astPattern links a compiled regex to a rule pattern and descriptive name.
type astPattern struct {
	name        string
	regex       *regexp.Regexp
	rulePattern string
	// skipFunc optionally filters out false positives on matched lines.
	skipFunc func(line string, match string) bool
}

// astPatterns contains all regex patterns for anti-pattern detection.
// Note: these patterns detect security anti-patterns in scanned codebases.
var astPatterns = []astPattern{
	// 1. Detects dangerous eval() calls in scanned code
	{
		name:        "eval() usage",
		regex:       regexp.MustCompile(`\beval\s*\(`),
		rulePattern: "eval-usage",
	},
	// 2. SQL string concatenation
	{
		name:        "SQL injection risk",
		regex:       regexp.MustCompile(`(?i)(?:query.*\+.*req\.|SELECT.*\+)`),
		rulePattern: "sql-injection",
	},
	// 3. Weak crypto (MD5/SHA1)
	{
		name:        "Weak crypto hash",
		regex:       regexp.MustCompile(`createHash\s*\(\s*['"](?:md5|sha1)['"]\s*\)`),
		rulePattern: "weak-hash",
	},
	// 4. Insecure random (Math.random())
	{
		name:        "Insecure random",
		regex:       regexp.MustCompile(`Math\.random\(\)`),
		rulePattern: "insecure-random",
	},
	// 5. XSS risk (innerHTML / dangerouslySetInnerHTML)
	{
		name:        "XSS risk",
		regex:       regexp.MustCompile(`(?:innerHTML|dangerouslySetInnerHTML)`),
		rulePattern: "xss-risk",
	},
	// 6. Disabled SSL verification
	{
		name:        "SSL verification disabled",
		regex:       regexp.MustCompile(`rejectUnauthorized\s*:\s*false`),
		rulePattern: "ssl-disabled",
	},
	// 7. Hardcoded IP addresses (skip 0.0.0.0 and 127.0.0.1)
	{
		name:        "Hardcoded IP address",
		regex:       regexp.MustCompile(`\b\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}\b`),
		rulePattern: "hardcoded-ip",
		skipFunc: func(line string, match string) bool {
			return match == "0.0.0.0" || match == "127.0.0.1"
		},
	},
	// 8. Empty catch blocks
	{
		name:        "Empty catch block",
		regex:       regexp.MustCompile(`catch\s*\([^)]*\)\s*\{\s*\}`),
		rulePattern: "empty-catch",
	},
}

// astSourceExtensions lists file extensions the AST analyzer inspects.
var astSourceExtensions = map[string]bool{
	"ts":   true,
	"js":   true,
	"py":   true,
	"go":   true,
	"java": true,
	"rb":   true,
}

// Analyze scans source files line by line for anti-patterns.
func (a *ASTAnalyzer) Analyze(files []ScannedFile, allRules []rules.Rule) ([]Finding, error) {
	myRules := filterRules(allRules, a.Name())
	if len(myRules) == 0 {
		return nil, nil
	}

	rulesByPattern := groupRulesByPattern(myRules)
	var findings []Finding

	for _, file := range files {
		if !astSourceExtensions[strings.ToLower(file.Extension)] {
			continue
		}
		if file.Size > maxFileSize {
			continue
		}

		content, err := os.ReadFile(file.Path)
		if err != nil {
			continue
		}

		if containsNullByte(content) {
			continue
		}

		lines := strings.Split(string(content), "\n")
		for lineIdx, line := range lines {
			for _, pat := range astPatterns {
				matches := pat.regex.FindAllString(line, -1)
				for _, m := range matches {
					// Apply skip function if defined
					if pat.skipFunc != nil && pat.skipFunc(line, m) {
						continue
					}

					matchedRules, ok := rulesByPattern[pat.rulePattern]
					if !ok || len(matchedRules) == 0 {
						continue
					}

					for _, rule := range matchedRules {
						findings = append(findings, Finding{
							ID:          rule.ID,
							Title:       rule.Title,
							Description: fmt.Sprintf("%s (%s)", rule.Description, pat.name),
							Severity:    rule.Severity,
							Framework:   rule.Framework,
							ControlIDs:  rule.Controls,
							File:        file.RelativePath,
							Line:        lineIdx + 1,
							Column:      strings.Index(line, m) + 1,
							Evidence:    strings.TrimSpace(line),
							Analyzer:    a.Name(),
							Fixable:     rule.Fixable,
						})
					}
				}
			}
		}
	}

	return findings, nil
}
