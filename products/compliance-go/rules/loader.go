package rules

import (
	"embed"
	"strings"

	"gopkg.in/yaml.v3"
)

//go:embed soc2.yaml hipaa.yaml gdpr.yaml
var ruleFS embed.FS

// Rule represents a single compliance rule loaded from a YAML definition.
type Rule struct {
	ID          string   `yaml:"id"`
	Title       string   `yaml:"title"`
	Severity    string   `yaml:"severity"`
	Analyzer    string   `yaml:"analyzer"`
	Pattern     string   `yaml:"pattern"`
	Controls    []string `yaml:"controls"`
	Framework   []string `yaml:"framework"`
	Description string   `yaml:"description"`
	Fixable     bool     `yaml:"fixable"`
}

// ruleFiles lists the embedded YAML file names in the order they are loaded.
var ruleFiles = []string{"soc2.yaml", "hipaa.yaml", "gdpr.yaml"}

// Load reads and parses all embedded YAML rule files.
// If frameworks is nil or empty, all rules are returned.
// If frameworks is specified, only rules whose Framework field contains
// at least one of the specified frameworks (case-insensitive) are returned.
func Load(frameworks []string) []Rule {
	var all []Rule

	for _, name := range ruleFiles {
		data, err := ruleFS.ReadFile(name)
		if err != nil {
			// Embedded files are guaranteed to exist at compile time,
			// so this should never happen. Panic to surface build issues.
			panic("rules: failed to read embedded file " + name + ": " + err.Error())
		}

		var batch []Rule
		if err := yaml.Unmarshal(data, &batch); err != nil {
			panic("rules: failed to parse " + name + ": " + err.Error())
		}
		all = append(all, batch...)
	}

	if len(frameworks) == 0 {
		return all
	}

	// Build a set of lowercase framework names for fast lookup.
	want := make(map[string]struct{}, len(frameworks))
	for _, fw := range frameworks {
		want[strings.ToLower(fw)] = struct{}{}
	}

	var filtered []Rule
	for _, r := range all {
		for _, fw := range r.Framework {
			if _, ok := want[strings.ToLower(fw)]; ok {
				filtered = append(filtered, r)
				break
			}
		}
	}
	return filtered
}
