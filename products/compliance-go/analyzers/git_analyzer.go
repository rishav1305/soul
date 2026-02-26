package analyzers

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/rishav1305/soul/products/compliance-go/rules"
)

// GitAnalyzer implements the Analyzer interface for detecting project-level
// git hygiene issues: missing .gitignore entries, CODEOWNERS, SECURITY.md,
// LICENSE, and CI configuration.
type GitAnalyzer struct{}

// Name returns the analyzer identifier used to match rules.
func (g *GitAnalyzer) Name() string {
	return "git-analyzer"
}

// Analyze checks project-level git configuration and file presence.
func (g *GitAnalyzer) Analyze(files []ScannedFile, allRules []rules.Rule) ([]Finding, error) {
	myRules := filterRules(allRules, g.Name())
	if len(myRules) == 0 {
		return nil, nil
	}

	rulesByPattern := groupRulesByPattern(myRules)
	var findings []Finding

	// Build a set of relative paths for quick lookups.
	pathSet := make(map[string]ScannedFile)
	for _, f := range files {
		pathSet[f.RelativePath] = f
	}

	// Check .gitignore for missing entries
	findings = append(findings, g.checkGitignore(pathSet, rulesByPattern)...)

	// Check for CODEOWNERS
	findings = append(findings, g.checkFileExists(pathSet, rulesByPattern,
		"missing-codeowners",
		[]string{"CODEOWNERS", ".github/CODEOWNERS", "docs/CODEOWNERS"},
		"No CODEOWNERS file found",
	)...)

	// Check for SECURITY.md
	findings = append(findings, g.checkFileExists(pathSet, rulesByPattern,
		"no-security-policy",
		[]string{"SECURITY.md", ".github/SECURITY.md"},
		"No SECURITY.md file found",
	)...)

	// Check for LICENSE
	findings = append(findings, g.checkFileExists(pathSet, rulesByPattern,
		"missing-license",
		[]string{"LICENSE", "LICENSE.md", "LICENSE.txt", "LICENCE", "LICENCE.md", "LICENCE.txt"},
		"No LICENSE file found",
	)...)

	return findings, nil
}

// checkGitignore checks if .gitignore exists and contains important entries.
func (g *GitAnalyzer) checkGitignore(pathSet map[string]ScannedFile, rulesByPattern map[string][]rules.Rule) []Finding {
	var findings []Finding

	gi, ok := pathSet[".gitignore"]
	if !ok {
		// .gitignore itself is missing — report as incomplete
		for _, rule := range rulesByPattern["incomplete-gitignore"] {
			findings = append(findings, Finding{
				ID:          rule.ID,
				Title:       rule.Title,
				Description: rule.Description,
				Severity:    rule.Severity,
				Framework:   rule.Framework,
				ControlIDs:  rule.Controls,
				Evidence:    "No .gitignore file found in the repository",
				Analyzer:    g.Name(),
				Fixable:     rule.Fixable,
			})
		}
		return findings
	}

	content, err := os.ReadFile(gi.Path)
	if err != nil {
		return nil
	}

	gitignoreContent := string(content)
	lines := strings.Split(gitignoreContent, "\n")

	// Check for .env entry
	hasEnv := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == ".env" || trimmed == ".env*" || trimmed == ".env.*" {
			hasEnv = true
			break
		}
	}
	if !hasEnv {
		for _, rule := range rulesByPattern["env-not-gitignored"] {
			findings = append(findings, Finding{
				ID:          rule.ID,
				Title:       rule.Title,
				Description: rule.Description,
				Severity:    rule.Severity,
				Framework:   rule.Framework,
				ControlIDs:  rule.Controls,
				File:        ".gitignore",
				Evidence:    ".gitignore does not contain .env entry",
				Analyzer:    g.Name(),
				Fixable:     rule.Fixable,
			})
		}
	}

	// Check for node_modules entry
	hasNodeModules := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "node_modules" || trimmed == "node_modules/" {
			hasNodeModules = true
			break
		}
	}

	// Check for common secret patterns (*.pem, *.key, etc.)
	sensitivePatterns := []string{"*.pem", "*.key", "*.p12", "*.pfx", "credentials.json"}
	missingSensitive := []string{}
	for _, pat := range sensitivePatterns {
		found := false
		for _, line := range lines {
			trimmed := strings.TrimSpace(line)
			if trimmed == pat {
				found = true
				break
			}
		}
		if !found {
			missingSensitive = append(missingSensitive, pat)
		}
	}

	if len(missingSensitive) > 0 {
		for _, rule := range rulesByPattern["sensitive-not-gitignored"] {
			findings = append(findings, Finding{
				ID:          rule.ID,
				Title:       rule.Title,
				Description: rule.Description,
				Severity:    rule.Severity,
				Framework:   rule.Framework,
				ControlIDs:  rule.Controls,
				File:        ".gitignore",
				Evidence:    "Missing entries: " + strings.Join(missingSensitive, ", "),
				Analyzer:    g.Name(),
				Fixable:     rule.Fixable,
			})
		}
	}

	// Check for incomplete .gitignore (missing node_modules or other common entries)
	if !hasNodeModules {
		for _, rule := range rulesByPattern["incomplete-gitignore"] {
			findings = append(findings, Finding{
				ID:          rule.ID,
				Title:       rule.Title,
				Description: rule.Description,
				Severity:    rule.Severity,
				Framework:   rule.Framework,
				ControlIDs:  rule.Controls,
				File:        ".gitignore",
				Evidence:    ".gitignore is missing node_modules entry",
				Analyzer:    g.Name(),
				Fixable:     rule.Fixable,
			})
		}
	}

	return findings
}

// checkFileExists checks if at least one of the given file paths exists.
// If none exist, findings are created using the given pattern.
func (g *GitAnalyzer) checkFileExists(pathSet map[string]ScannedFile, rulesByPattern map[string][]rules.Rule, pattern string, paths []string, evidence string) []Finding {
	for _, p := range paths {
		// Check both exact match and case-insensitive
		if _, ok := pathSet[p]; ok {
			return nil
		}
	}

	// Also check by basename (case-insensitive) for flexibility
	for _, f := range pathSet {
		base := filepath.Base(f.RelativePath)
		for _, p := range paths {
			if strings.EqualFold(base, filepath.Base(p)) {
				return nil
			}
		}
	}

	var findings []Finding
	for _, rule := range rulesByPattern[pattern] {
		findings = append(findings, Finding{
			ID:          rule.ID,
			Title:       rule.Title,
			Description: rule.Description,
			Severity:    rule.Severity,
			Framework:   rule.Framework,
			ControlIDs:  rule.Controls,
			Evidence:    evidence,
			Analyzer:    g.Name(),
			Fixable:     rule.Fixable,
		})
	}
	return findings
}
