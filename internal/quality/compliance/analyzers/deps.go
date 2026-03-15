package analyzers

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// DepAuditor checks dependency configuration for compliance issues.
type DepAuditor struct{}

// Name returns the analyzer name.
func (d *DepAuditor) Name() string { return "deps" }

// Analyze scans dependency files for compliance issues.
func (d *DepAuditor) Analyze(files []ScannedFile, rules []Rule) ([]Finding, error) {
	ruleMap := buildRuleMap(rules, "deps")
	if len(ruleMap) == 0 {
		return nil, nil
	}

	root := projectRoot(files)
	if root == "" {
		return nil, nil
	}

	var findings []Finding

	// Find package.json files
	for _, sf := range files {
		if filepath.Base(sf.RelativePath) != "package.json" {
			continue
		}

		pkg, err := readPackageJSON(sf.Path)
		if err != nil {
			continue
		}

		// Unpinned deps
		if r, ok := ruleMap["unpinned-deps"]; ok {
			allDeps := mergeMaps(pkg.Dependencies, pkg.DevDependencies)
			for name, version := range allDeps {
				if isUnpinned(version) {
					findings = append(findings, makeFinding(r, d.Name(), sf.Path, 0,
						fmt.Sprintf("%s: %s", name, version)))
				}
			}
		}

		// Missing engines
		if r, ok := ruleMap["missing-engines"]; ok {
			if len(pkg.Engines) == 0 {
				findings = append(findings, makeFinding(r, d.Name(), sf.Path, 0, "package.json missing engines field"))
			}
		}

		// Copyleft license
		if r, ok := ruleMap["copyleft-license"]; ok {
			if pkg.License != "" && isCopyleft(pkg.License) {
				findings = append(findings, makeFinding(r, d.Name(), sf.Path, 0,
					fmt.Sprintf("license: %s", pkg.License)))
			}
		}
	}

	// Missing lockfile
	if r, ok := ruleMap["missing-lockfile"]; ok {
		if fileExists(files, "package.json") {
			lockfiles := []string{"package-lock.json", "yarn.lock", "pnpm-lock.yaml"}
			hasLockfile := false
			for _, lf := range lockfiles {
				if fileExists(files, lf) {
					hasLockfile = true
					break
				}
			}
			if !hasLockfile {
				findings = append(findings, makeFinding(r, d.Name(), filepath.Join(root, "package.json"), 0, "No lockfile found (package-lock.json, yarn.lock, or pnpm-lock.yaml)"))
			}
		}
	}

	return findings, nil
}

type packageJSON struct {
	Dependencies    map[string]string `json:"dependencies"`
	DevDependencies map[string]string `json:"devDependencies"`
	Engines         map[string]string `json:"engines"`
	License         string            `json:"license"`
}

func readPackageJSON(path string) (*packageJSON, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var pkg packageJSON
	if err := json.Unmarshal(data, &pkg); err != nil {
		return nil, err
	}
	return &pkg, nil
}

func isUnpinned(version string) bool {
	return strings.HasPrefix(version, "^") ||
		strings.HasPrefix(version, "~") ||
		version == "*"
}

func isCopyleft(license string) bool {
	upper := strings.ToUpper(license)
	copyleftPrefixes := []string{"GPL", "AGPL", "LGPL", "EUPL", "MPL", "SSPL", "OSL"}
	for _, prefix := range copyleftPrefixes {
		if strings.Contains(upper, prefix) {
			return true
		}
	}
	return false
}

func mergeMaps(a, b map[string]string) map[string]string {
	result := make(map[string]string)
	for k, v := range a {
		result[k] = v
	}
	for k, v := range b {
		result[k] = v
	}
	return result
}
