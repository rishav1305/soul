package analyzers

import (
	"regexp"
	"strings"
)

// ASTAnalyzer performs regex-based static analysis on source code files.
type ASTAnalyzer struct{}

// Name returns the analyzer name.
func (a *ASTAnalyzer) Name() string { return "ast" }

// astPattern pairs a compiled regex with its rule pattern name.
type astPattern struct {
	pattern string
	re      *regexp.Regexp
	skip    func(string) bool
}

var astPatterns = []astPattern{
	{pattern: "eval-usage", re: regexp.MustCompile(`\beval\s*\(`)},
	{pattern: "sql-injection", re: regexp.MustCompile(`(?i)(?:query.*\+.*req\.|SELECT.*\+)`)},
	{pattern: "weak-hash", re: regexp.MustCompile(`createHash\s*\(\s*['"](?:md5|sha1)['"]\s*\)`)},
	{pattern: "insecure-random", re: regexp.MustCompile(`Math\.random\(\)`)},
	{pattern: "xss-risk", re: regexp.MustCompile(`(?:innerHTML|dangerouslySetInnerHTML)`)},
	{pattern: "ssl-disabled", re: regexp.MustCompile(`rejectUnauthorized\s*:\s*false`)},
	{pattern: "hardcoded-ip", re: regexp.MustCompile(`\b\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}\b`), skip: func(match string) bool {
		return match == "0.0.0.0" || match == "127.0.0.1"
	}},
	{pattern: "empty-catch", re: regexp.MustCompile(`catch\s*\([^)]*\)\s*\{\s*\}`)},
}

// isASTSourceFile checks if the extension is scannable by the AST analyzer.
func isASTSourceFile(ext string) bool {
	switch ext {
	case ".ts", ".tsx", ".js", ".jsx", ".py", ".go", ".java", ".rb":
		return true
	}
	return false
}

// Analyze scans source files for security and code quality patterns.
// NOTE: This is a DETECTION-only analyzer — it identifies potential vulnerabilities
// in source code using regex patterns. It does not execute any user code.
func (a *ASTAnalyzer) Analyze(files []ScannedFile, rules []Rule) ([]Finding, error) {
	ruleMap := buildRuleMap(rules, "ast")
	if len(ruleMap) == 0 {
		return nil, nil
	}

	var findings []Finding

	for _, sf := range files {
		if !isASTSourceFile(sf.Extension) {
			continue
		}

		lines, err := readLines(sf.Path)
		if err != nil {
			continue
		}

		for i, line := range lines {
			for _, ap := range astPatterns {
				r, ok := ruleMap[ap.pattern]
				if !ok {
					continue
				}
				if !ap.re.MatchString(line) {
					continue
				}
				// For hardcoded-ip, check if all matches are in the skip list.
				if ap.skip != nil {
					matches := ap.re.FindAllString(line, -1)
					allSkipped := true
					for _, m := range matches {
						if !ap.skip(m) {
							allSkipped = false
							break
						}
					}
					if allSkipped {
						continue
					}
				}
				findings = append(findings, makeFinding(r, a.Name(), sf.Path, i+1, strings.TrimSpace(line)))
			}
		}
	}

	return findings, nil
}
