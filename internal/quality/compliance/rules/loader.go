package rules

import (
	"embed"
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/rishav1305/soul/internal/quality/compliance/analyzers"
)

//go:embed *.yaml
var ruleFS embed.FS

type ruleFile struct {
	Rules []analyzers.Rule `yaml:"rules"`
}

// LoadAll loads compliance rules from embedded YAML files.
// If frameworks is nil or empty, all rules are returned.
// Otherwise, only rules matching at least one requested framework are included.
func LoadAll(frameworks []string) ([]analyzers.Rule, error) {
	var allRules []analyzers.Rule

	err := fs.WalkDir(ruleFS, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		ext := strings.ToLower(filepath.Ext(path))
		if ext != ".yaml" && ext != ".yml" {
			return nil
		}

		data, readErr := ruleFS.ReadFile(path)
		if readErr != nil {
			return fmt.Errorf("reading %s: %w", path, readErr)
		}

		var rf ruleFile
		if parseErr := yaml.Unmarshal(data, &rf); parseErr != nil {
			return fmt.Errorf("parsing %s: %w", path, parseErr)
		}

		allRules = append(allRules, rf.Rules...)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walking rule files: %w", err)
	}

	if len(frameworks) == 0 {
		return allRules, nil
	}

	want := make(map[string]bool, len(frameworks))
	for _, f := range frameworks {
		want[strings.ToLower(f)] = true
	}

	var filtered []analyzers.Rule
	for _, r := range allRules {
		for _, fw := range r.Framework {
			if want[strings.ToLower(fw)] {
				filtered = append(filtered, r)
				break
			}
		}
	}
	return filtered, nil
}
