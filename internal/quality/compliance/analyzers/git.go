package analyzers

import (
	"path/filepath"
)

// GitAnalyzer checks git-related project structure for compliance issues.
type GitAnalyzer struct{}

// Name returns the analyzer name.
func (g *GitAnalyzer) Name() string { return "git" }

// Analyze scans the project structure for git-related compliance issues.
func (g *GitAnalyzer) Analyze(files []ScannedFile, rules []Rule) ([]Finding, error) {
	ruleMap := buildRuleMap(rules, "git")
	if len(ruleMap) == 0 {
		return nil, nil
	}

	root := projectRoot(files)
	if root == "" {
		return nil, nil
	}

	var findings []Finding
	gitignorePath := filepath.Join(root, ".gitignore")
	hasGitignore := fileExists(files, ".gitignore")

	// .gitignore itself missing
	if r, ok := ruleMap["incomplete-gitignore"]; ok {
		if !hasGitignore {
			findings = append(findings, makeFinding(r, g.Name(), gitignorePath, 0, ".gitignore file is missing"))
		} else {
			// Check for node_modules entry
			if !gitignoreContains(root, "node_modules") {
				findings = append(findings, makeFinding(r, g.Name(), gitignorePath, 0, ".gitignore missing node_modules entry"))
			}
		}
	}

	// .env not gitignored
	if r, ok := ruleMap["env-not-gitignored"]; ok {
		if hasGitignore && !gitignoreContains(root, ".env") {
			findings = append(findings, makeFinding(r, g.Name(), gitignorePath, 0, ".gitignore missing .env entry"))
		}
	}

	// Sensitive patterns not gitignored
	if r, ok := ruleMap["sensitive-not-gitignored"]; ok && hasGitignore {
		sensitivePatterns := []string{"*.pem", "*.key", "*.p12", "credentials.json"}
		for _, pattern := range sensitivePatterns {
			if !gitignoreContains(root, pattern) {
				findings = append(findings, makeFinding(r, g.Name(), gitignorePath, 0, ".gitignore missing "+pattern+" entry"))
			}
		}
	}

	// Missing CODEOWNERS
	if r, ok := ruleMap["missing-codeowners"]; ok {
		if !fileExists(files, "CODEOWNERS") && !fileExists(files, ".github/CODEOWNERS") && !fileExists(files, "docs/CODEOWNERS") {
			findings = append(findings, makeFinding(r, g.Name(), root, 0, "CODEOWNERS file is missing"))
		}
	}

	// Missing SECURITY.md
	if r, ok := ruleMap["no-security-policy"]; ok {
		if !fileExists(files, "SECURITY.md") && !fileExists(files, ".github/SECURITY.md") {
			findings = append(findings, makeFinding(r, g.Name(), root, 0, "SECURITY.md file is missing"))
		}
	}

	// Missing LICENSE
	if r, ok := ruleMap["missing-license"]; ok {
		if !fileExists(files, "LICENSE") && !fileExists(files, "LICENSE.md") && !fileExists(files, "LICENSE.txt") {
			findings = append(findings, makeFinding(r, g.Name(), root, 0, "LICENSE file is missing"))
		}
	}

	return findings, nil
}
