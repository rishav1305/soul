package analyzers

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/rishav1305/soul/products/compliance/rules"
)

// ConfigChecker implements the Analyzer interface for detecting configuration
// issues: exposed .env files, Docker misconfigurations, missing security
// headers, CORS wildcards, and missing HTTPS redirects.
type ConfigChecker struct{}

// Name returns the analyzer identifier used to match rules.
func (c *ConfigChecker) Name() string {
	return "config-checker"
}

// config checker regex patterns compiled once.
var (
	corsWildcardRegex  = regexp.MustCompile(`(?i)origin\s*:\s*['"]?\*['"]?`)
	dockerLatestRegex  = regexp.MustCompile(`(?i)^FROM\s+\S+:latest`)
	dockerFromRegex    = regexp.MustCompile(`(?i)^FROM\s+(\S+)`)
	dockerUserRegex    = regexp.MustCompile(`(?i)^USER\s+`)
	dockerHealthRegex  = regexp.MustCompile(`(?i)^HEALTHCHECK\s+`)
	httpsRedirectRegex = regexp.MustCompile(`(?i)(https?.*redirect|redirect.*https|force.*ssl|ssl.*redirect|http.*https)`)
)

// Analyze scans files for configuration-level compliance issues.
func (c *ConfigChecker) Analyze(files []ScannedFile, allRules []rules.Rule) ([]Finding, error) {
	myRules := filterRules(allRules, c.Name())
	if len(myRules) == 0 {
		return nil, nil
	}

	rulesByPattern := groupRulesByPattern(myRules)
	var findings []Finding

	// Build a set of file relative paths for quick existence checks.
	fileIndex := make(map[string]ScannedFile)
	for _, f := range files {
		fileIndex[f.RelativePath] = f
	}

	// Check: .env file exists but not in .gitignore
	findings = append(findings, c.checkEnvExposed(files, fileIndex, rulesByPattern)...)

	// Check Dockerfiles
	findings = append(findings, c.checkDockerfiles(files, rulesByPattern)...)

	// Check source files for CORS wildcards
	findings = append(findings, c.checkCORSWildcard(files, rulesByPattern)...)

	// Check for CI/CD configuration
	findings = append(findings, c.checkCIConfig(files, rulesByPattern)...)

	return findings, nil
}

// checkEnvExposed checks if .env exists but is not covered in .gitignore.
func (c *ConfigChecker) checkEnvExposed(files []ScannedFile, fileIndex map[string]ScannedFile, rulesByPattern map[string][]rules.Rule) []Finding {
	var findings []Finding

	// Check if .env file exists
	hasEnv := false
	for _, f := range files {
		base := filepath.Base(f.RelativePath)
		if base == ".env" || strings.HasPrefix(base, ".env.") {
			hasEnv = true
			break
		}
	}
	if !hasEnv {
		return nil
	}

	// Check if .gitignore exists and contains .env
	envGitignored := false
	if gi, ok := fileIndex[".gitignore"]; ok {
		content, err := os.ReadFile(gi.Path)
		if err == nil {
			lines := strings.Split(string(content), "\n")
			for _, line := range lines {
				trimmed := strings.TrimSpace(line)
				if trimmed == ".env" || trimmed == ".env*" || trimmed == ".env.*" {
					envGitignored = true
					break
				}
			}
		}
	}

	if !envGitignored {
		matchedRules := rulesByPattern["env-not-gitignored"]
		for _, rule := range matchedRules {
			findings = append(findings, Finding{
				ID:          rule.ID,
				Title:       rule.Title,
				Description: rule.Description,
				Severity:    rule.Severity,
				Framework:   rule.Framework,
				ControlIDs:  rule.Controls,
				File:        ".env",
				Evidence:    ".env file exists but is not in .gitignore",
				Analyzer:    c.Name(),
				Fixable:     rule.Fixable,
			})
		}
	}

	return findings
}

// checkDockerfiles inspects Dockerfiles for root user and :latest tag usage.
func (c *ConfigChecker) checkDockerfiles(files []ScannedFile, rulesByPattern map[string][]rules.Rule) []Finding {
	var findings []Finding

	for _, file := range files {
		base := filepath.Base(file.RelativePath)
		if !strings.EqualFold(base, "Dockerfile") && !strings.HasPrefix(strings.ToLower(base), "dockerfile.") {
			continue
		}

		content, err := os.ReadFile(file.Path)
		if err != nil {
			continue
		}

		lines := strings.Split(string(content), "\n")
		hasUser := false
		hasHealthcheck := false

		for lineIdx, line := range lines {
			trimmed := strings.TrimSpace(line)

			// Check for USER directive
			if dockerUserRegex.MatchString(trimmed) {
				hasUser = true
			}

			// Check for HEALTHCHECK directive
			if dockerHealthRegex.MatchString(trimmed) {
				hasHealthcheck = true
			}

			// Check for :latest tag in FROM
			if dockerFromRegex.MatchString(trimmed) {
				// Check if it uses :latest or has no tag (implicit latest)
				if dockerLatestRegex.MatchString(trimmed) {
					for _, rule := range rulesByPattern["docker-latest-tag"] {
						findings = append(findings, Finding{
							ID:          rule.ID,
							Title:       rule.Title,
							Description: rule.Description,
							Severity:    rule.Severity,
							Framework:   rule.Framework,
							ControlIDs:  rule.Controls,
							File:        file.RelativePath,
							Line:        lineIdx + 1,
							Evidence:    trimmed,
							Analyzer:    c.Name(),
							Fixable:     rule.Fixable,
						})
					}
				}
			}
		}

		// Missing USER directive → running as root
		if !hasUser {
			for _, rule := range rulesByPattern["docker-root-user"] {
				findings = append(findings, Finding{
					ID:          rule.ID,
					Title:       rule.Title,
					Description: rule.Description,
					Severity:    rule.Severity,
					Framework:   rule.Framework,
					ControlIDs:  rule.Controls,
					File:        file.RelativePath,
					Evidence:    "Dockerfile does not contain a USER directive",
					Analyzer:    c.Name(),
					Fixable:     rule.Fixable,
				})
			}
		}

		// Missing HEALTHCHECK
		if !hasHealthcheck {
			for _, rule := range rulesByPattern["docker-no-healthcheck"] {
				findings = append(findings, Finding{
					ID:          rule.ID,
					Title:       rule.Title,
					Description: rule.Description,
					Severity:    rule.Severity,
					Framework:   rule.Framework,
					ControlIDs:  rule.Controls,
					File:        file.RelativePath,
					Evidence:    "Dockerfile does not contain a HEALTHCHECK directive",
					Analyzer:    c.Name(),
					Fixable:     rule.Fixable,
				})
			}
		}
	}

	return findings
}

// checkCORSWildcard scans source files for CORS wildcard origin patterns.
func (c *ConfigChecker) checkCORSWildcard(files []ScannedFile, rulesByPattern map[string][]rules.Rule) []Finding {
	var findings []Finding

	sourceExts := map[string]bool{
		"ts": true, "js": true, "py": true, "go": true,
		"java": true, "rb": true, "json": true, "yaml": true, "yml": true,
	}

	for _, file := range files {
		if !sourceExts[strings.ToLower(file.Extension)] {
			continue
		}
		if file.Size > maxFileSize {
			continue
		}

		content, err := os.ReadFile(file.Path)
		if err != nil {
			continue
		}

		lines := strings.Split(string(content), "\n")
		for lineIdx, line := range lines {
			if corsWildcardRegex.MatchString(line) {
				for _, rule := range rulesByPattern["wildcard-cors"] {
					findings = append(findings, Finding{
						ID:          rule.ID,
						Title:       rule.Title,
						Description: fmt.Sprintf("%s (found CORS wildcard)", rule.Description),
						Severity:    rule.Severity,
						Framework:   rule.Framework,
						ControlIDs:  rule.Controls,
						File:        file.RelativePath,
						Line:        lineIdx + 1,
						Evidence:    strings.TrimSpace(line),
						Analyzer:    c.Name(),
						Fixable:     rule.Fixable,
					})
				}
			}
		}
	}

	return findings
}

// checkCIConfig checks for the presence of CI/CD configuration files.
func (c *ConfigChecker) checkCIConfig(files []ScannedFile, rulesByPattern map[string][]rules.Rule) []Finding {
	var findings []Finding

	ciPatterns := []string{
		".github/workflows/",
		".gitlab-ci.yml",
		"Jenkinsfile",
		".circleci/",
		".travis.yml",
		"azure-pipelines.yml",
		"bitbucket-pipelines.yml",
	}

	hasCI := false
	for _, file := range files {
		for _, pattern := range ciPatterns {
			if strings.HasPrefix(file.RelativePath, pattern) || file.RelativePath == pattern {
				hasCI = true
				break
			}
		}
		if hasCI {
			break
		}
	}

	if !hasCI {
		for _, rule := range rulesByPattern["no-ci-config"] {
			findings = append(findings, Finding{
				ID:          rule.ID,
				Title:       rule.Title,
				Description: rule.Description,
				Severity:    rule.Severity,
				Framework:   rule.Framework,
				ControlIDs:  rule.Controls,
				Evidence:    "No CI/CD configuration detected",
				Analyzer:    c.Name(),
				Fixable:     rule.Fixable,
			})
		}
	}

	return findings
}
