package analyzers

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/rishav1305/soul/products/compliance-go/rules"
)

// DepAuditor implements the Analyzer interface for auditing dependency
// configurations: unpinned versions, missing lockfiles, missing engines
// field, and copyleft license detection.
type DepAuditor struct{}

// Name returns the analyzer identifier used to match rules.
func (d *DepAuditor) Name() string {
	return "dep-auditor"
}

// packageJSON represents the relevant fields of a package.json file.
type packageJSON struct {
	Name            string            `json:"name"`
	Engines         json.RawMessage   `json:"engines"`
	Dependencies    map[string]string `json:"dependencies"`
	DevDependencies map[string]string `json:"devDependencies"`
	License         string            `json:"license"`
}

// Analyze inspects package.json files and lockfile presence.
func (d *DepAuditor) Analyze(files []ScannedFile, allRules []rules.Rule) ([]Finding, error) {
	myRules := filterRules(allRules, d.Name())
	if len(myRules) == 0 {
		return nil, nil
	}

	rulesByPattern := groupRulesByPattern(myRules)
	var findings []Finding

	// Build path index
	pathSet := make(map[string]ScannedFile)
	for _, f := range files {
		pathSet[f.RelativePath] = f
	}

	// Find all package.json files
	var packageFiles []ScannedFile
	for _, f := range files {
		if filepath.Base(f.RelativePath) == "package.json" {
			packageFiles = append(packageFiles, f)
		}
	}

	for _, pkgFile := range packageFiles {
		findings = append(findings, d.analyzePackageJSON(pkgFile, files, rulesByPattern)...)
	}

	// If no package.json at all, skip lockfile/engines checks
	if len(packageFiles) == 0 {
		return findings, nil
	}

	// Check for lockfile presence (project-level)
	findings = append(findings, d.checkLockfile(files, rulesByPattern)...)

	return findings, nil
}

// analyzePackageJSON parses and inspects a single package.json file.
func (d *DepAuditor) analyzePackageJSON(pkgFile ScannedFile, files []ScannedFile, rulesByPattern map[string][]rules.Rule) []Finding {
	var findings []Finding

	content, err := os.ReadFile(pkgFile.Path)
	if err != nil {
		return nil
	}

	var pkg packageJSON
	if err := json.Unmarshal(content, &pkg); err != nil {
		return nil
	}

	// Check for unpinned dependencies
	findings = append(findings, d.checkUnpinnedDeps(pkgFile, pkg.Dependencies, "dependencies", rulesByPattern)...)
	findings = append(findings, d.checkUnpinnedDeps(pkgFile, pkg.DevDependencies, "devDependencies", rulesByPattern)...)

	// Check for missing engines field
	if len(pkg.Engines) == 0 || string(pkg.Engines) == "null" {
		for _, rule := range rulesByPattern["missing-engines"] {
			findings = append(findings, Finding{
				ID:          rule.ID,
				Title:       rule.Title,
				Description: rule.Description,
				Severity:    rule.Severity,
				Framework:   rule.Framework,
				ControlIDs:  rule.Controls,
				File:        pkgFile.RelativePath,
				Evidence:    "package.json is missing the engines field",
				Analyzer:    d.Name(),
				Fixable:     rule.Fixable,
			})
		}
	}

	// Check for copyleft license in the package itself
	if isCopyleftLicense(pkg.License) {
		for _, rule := range rulesByPattern["copyleft-license"] {
			findings = append(findings, Finding{
				ID:          rule.ID,
				Title:       rule.Title,
				Description: rule.Description,
				Severity:    rule.Severity,
				Framework:   rule.Framework,
				ControlIDs:  rule.Controls,
				File:        pkgFile.RelativePath,
				Evidence:    fmt.Sprintf("Package uses copyleft license: %s", pkg.License),
				Analyzer:    d.Name(),
				Fixable:     rule.Fixable,
			})
		}
	}

	return findings
}

// checkUnpinnedDeps checks for dependencies using ^ ~ or * version prefixes.
func (d *DepAuditor) checkUnpinnedDeps(pkgFile ScannedFile, deps map[string]string, section string, rulesByPattern map[string][]rules.Rule) []Finding {
	var findings []Finding

	for name, version := range deps {
		if isUnpinnedVersion(version) {
			for _, rule := range rulesByPattern["unpinned-deps"] {
				findings = append(findings, Finding{
					ID:          rule.ID,
					Title:       rule.Title,
					Description: fmt.Sprintf("%s (%s.%s: %s)", rule.Description, section, name, version),
					Severity:    rule.Severity,
					Framework:   rule.Framework,
					ControlIDs:  rule.Controls,
					File:        pkgFile.RelativePath,
					Evidence:    fmt.Sprintf("%s: %s@%s", section, name, version),
					Analyzer:    d.Name(),
					Fixable:     rule.Fixable,
				})
			}
		}
	}

	return findings
}

// checkLockfile checks for the presence of a lockfile in the project.
func (d *DepAuditor) checkLockfile(files []ScannedFile, rulesByPattern map[string][]rules.Rule) []Finding {
	lockfileNames := map[string]bool{
		"package-lock.json": true,
		"yarn.lock":         true,
		"pnpm-lock.yaml":    true,
	}

	for _, f := range files {
		base := filepath.Base(f.RelativePath)
		if lockfileNames[base] {
			return nil
		}
	}

	var findings []Finding
	for _, rule := range rulesByPattern["missing-lockfile"] {
		findings = append(findings, Finding{
			ID:          rule.ID,
			Title:       rule.Title,
			Description: rule.Description,
			Severity:    rule.Severity,
			Framework:   rule.Framework,
			ControlIDs:  rule.Controls,
			Evidence:    "No lockfile (package-lock.json, yarn.lock, pnpm-lock.yaml) found",
			Analyzer:    d.Name(),
			Fixable:     rule.Fixable,
		})
	}
	return findings
}

// isUnpinnedVersion returns true if the version string uses a range prefix.
func isUnpinnedVersion(version string) bool {
	v := strings.TrimSpace(version)
	if v == "" {
		return false
	}
	return strings.HasPrefix(v, "^") || strings.HasPrefix(v, "~") || v == "*"
}

// isCopyleftLicense checks if a license string indicates a copyleft license.
func isCopyleftLicense(license string) bool {
	upper := strings.ToUpper(strings.TrimSpace(license))
	copyleftKeywords := []string{"GPL", "AGPL", "LGPL", "EUPL", "MPL", "SSPL", "OSL"}
	for _, kw := range copyleftKeywords {
		if strings.Contains(upper, kw) {
			return true
		}
	}
	return false
}
