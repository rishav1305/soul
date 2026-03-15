package prompts

import (
	"embed"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/rishav1305/soul-v2/internal/bench/scoring"
)

//go:embed *.json
var promptFS embed.FS

// allCategories lists every prompt category (excluding smoke-test).
var allCategories = []string{
	"system-health",
	"code-generation",
	"classification",
	"knowledge-qa",
	"task-planning",
	"email-drafting",
	"contact-research",
	"campaign-planning",
	"reply-classification",
	"infra-management",
}

// LoadAll loads prompts from every category (excluding smoke-test).
func LoadAll() ([]scoring.PromptData, error) {
	var all []scoring.PromptData
	for _, cat := range allCategories {
		prompts, err := LoadCategory(cat)
		if err != nil {
			return nil, fmt.Errorf("load category %s: %w", cat, err)
		}
		all = append(all, prompts...)
	}
	return all, nil
}

// LoadCategory loads prompts from a single category file.
func LoadCategory(name string) ([]scoring.PromptData, error) {
	filename := name + ".json"
	data, err := promptFS.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", filename, err)
	}
	var prompts []scoring.PromptData
	if err := json.Unmarshal(data, &prompts); err != nil {
		return nil, fmt.Errorf("parse %s: %w", filename, err)
	}
	return prompts, nil
}

// LoadSmoke loads the smoke-test prompts for quick verification.
func LoadSmoke() ([]scoring.PromptData, error) {
	return LoadCategory("smoke-test")
}

// Categories returns the sorted list of available category names.
func Categories() []string {
	entries, err := promptFS.ReadDir(".")
	if err != nil {
		return allCategories
	}
	var cats []string
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		name := strings.TrimSuffix(e.Name(), ".json")
		cats = append(cats, name)
	}
	sort.Strings(cats)
	return cats
}
