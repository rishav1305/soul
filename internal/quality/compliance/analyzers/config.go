package analyzers

import (
	"bufio"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// ConfigChecker checks project configuration for compliance issues.
type ConfigChecker struct{}

// Name returns the analyzer name.
func (c *ConfigChecker) Name() string { return "config" }

// Analyze scans files and project structure for configuration issues.
func (c *ConfigChecker) Analyze(files []ScannedFile, rules []Rule) ([]Finding, error) {
	ruleMap := buildRuleMap(rules, "config")
	if len(ruleMap) == 0 {
		return nil, nil
	}

	root := projectRoot(files)
	if root == "" {
		return nil, nil
	}

	var findings []Finding

	// .env exposed: .env exists but not in .gitignore
	if r, ok := ruleMap["env-not-gitignored"]; ok {
		if fileExists(files, ".env") && !gitignoreContains(root, ".env") {
			findings = append(findings, makeFinding(r, c.Name(), filepath.Join(root, ".env"), 0, ".env file exists but is not listed in .gitignore"))
		}
	}

	// Dockerfile checks
	for _, sf := range files {
		base := filepath.Base(sf.RelativePath)
		if base != "Dockerfile" && !strings.HasPrefix(base, "Dockerfile.") {
			continue
		}

		lines, err := readLines(sf.Path)
		if err != nil {
			continue
		}

		if r, ok := ruleMap["docker-latest-tag"]; ok {
			for i, line := range lines {
				trimmed := strings.TrimSpace(line)
				if strings.HasPrefix(strings.ToUpper(trimmed), "FROM ") {
					image := strings.Fields(trimmed)[1]
					// Skip scratch and ARG references
					if image == "scratch" || strings.HasPrefix(image, "$") {
						continue
					}
					if !strings.Contains(image, ":") || strings.HasSuffix(image, ":latest") {
						findings = append(findings, makeFinding(r, c.Name(), sf.Path, i+1, trimmed))
					}
				}
			}
		}

		if r, ok := ruleMap["docker-root-user"]; ok {
			hasUser := false
			for _, line := range lines {
				if strings.HasPrefix(strings.TrimSpace(strings.ToUpper(line)), "USER ") {
					hasUser = true
					break
				}
			}
			if !hasUser {
				findings = append(findings, makeFinding(r, c.Name(), sf.Path, 0, "Dockerfile missing USER directive"))
			}
		}

		if r, ok := ruleMap["docker-no-healthcheck"]; ok {
			hasHealthcheck := false
			for _, line := range lines {
				if strings.HasPrefix(strings.TrimSpace(strings.ToUpper(line)), "HEALTHCHECK ") {
					hasHealthcheck = true
					break
				}
			}
			if !hasHealthcheck {
				findings = append(findings, makeFinding(r, c.Name(), sf.Path, 0, "Dockerfile missing HEALTHCHECK directive"))
			}
		}
	}

	// CORS wildcard
	if r, ok := ruleMap["wildcard-cors"]; ok {
		corsRe := regexp.MustCompile(`(?i)origin\s*:\s*['"]?\*['"]?`)
		for _, sf := range files {
			if !isSourceFile(sf.Extension) {
				continue
			}
			lines, err := readLines(sf.Path)
			if err != nil {
				continue
			}
			for i, line := range lines {
				if corsRe.MatchString(line) {
					findings = append(findings, makeFinding(r, c.Name(), sf.Path, i+1, strings.TrimSpace(line)))
				}
			}
		}
	}

	// CI/CD missing
	if r, ok := ruleMap["no-ci-config"]; ok {
		ciPaths := []string{
			".github/workflows",
			".gitlab-ci.yml",
			"Jenkinsfile",
			".circleci",
			".travis.yml",
		}
		hasCI := false
		for _, sf := range files {
			for _, ci := range ciPaths {
				if strings.HasPrefix(sf.RelativePath, ci) || sf.RelativePath == ci {
					hasCI = true
					break
				}
			}
			if hasCI {
				break
			}
		}
		if !hasCI {
			findings = append(findings, makeFinding(r, c.Name(), root, 0, "No CI/CD configuration found"))
		}
	}

	return findings, nil
}

// buildRuleMap filters rules by analyzer name and indexes by pattern.
func buildRuleMap(rules []Rule, analyzer string) map[string]Rule {
	m := make(map[string]Rule)
	for _, r := range rules {
		if r.Analyzer == analyzer {
			m[r.Pattern] = r
		}
	}
	return m
}

// projectRoot derives the common root directory from the file list.
func projectRoot(files []ScannedFile) string {
	if len(files) == 0 {
		return ""
	}
	root := filepath.Dir(files[0].Path)
	for _, f := range files[1:] {
		dir := filepath.Dir(f.Path)
		for !strings.HasPrefix(dir, root) {
			root = filepath.Dir(root)
			if root == "/" || root == "." {
				return root
			}
		}
	}
	return root
}

// fileExists checks if a relative path exists in the scanned file list.
func fileExists(files []ScannedFile, rel string) bool {
	for _, f := range files {
		if f.RelativePath == rel {
			return true
		}
	}
	return false
}

// gitignoreContains checks if .gitignore contains a given entry.
func gitignoreContains(root, entry string) bool {
	path := filepath.Join(root, ".gitignore")
	lines, err := readLines(path)
	if err != nil {
		return false
	}
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == entry || trimmed == "/"+entry || trimmed == entry+"/" {
			return true
		}
	}
	return false
}

// readLines reads all lines from a file.
func readLines(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var lines []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	return lines, scanner.Err()
}

// isSourceFile checks if the extension belongs to a source code file.
func isSourceFile(ext string) bool {
	switch ext {
	case ".ts", ".tsx", ".js", ".jsx", ".py", ".go", ".java", ".rb", ".rs", ".yaml", ".yml", ".json", ".toml":
		return true
	}
	return false
}

// makeFinding creates a Finding from a Rule.
func makeFinding(r Rule, analyzer, file string, line int, evidence string) Finding {
	return Finding{
		ID:          r.ID,
		Title:       r.Title,
		Description: r.Description,
		Severity:    r.Severity,
		Framework:   r.Framework,
		ControlIDs:  r.Controls,
		File:        file,
		Line:        line,
		Evidence:    evidence,
		Analyzer:    analyzer,
		Fixable:     r.Fixable,
	}
}
