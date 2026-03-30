package prompts

import (
	"testing"
)

func TestLoadCategory_SmokeTest(t *testing.T) {
	prompts, err := LoadCategory("smoke-test")
	if err != nil {
		t.Fatalf("LoadCategory(smoke-test): %v", err)
	}
	if len(prompts) == 0 {
		t.Error("expected at least one smoke-test prompt")
	}
	for _, p := range prompts {
		if p.ID == "" {
			t.Error("prompt ID should not be empty")
		}
		if p.Prompt == "" {
			t.Error("prompt text should not be empty")
		}
	}
}

func TestLoadCategory_Invalid(t *testing.T) {
	_, err := LoadCategory("nonexistent-category")
	if err == nil {
		t.Fatal("expected error for nonexistent category")
	}
}

func TestLoadAll(t *testing.T) {
	prompts, err := LoadAll()
	if err != nil {
		t.Fatalf("LoadAll: %v", err)
	}
	if len(prompts) == 0 {
		t.Error("expected at least one prompt from LoadAll")
	}
	// Should have prompts from multiple categories.
	categories := make(map[string]bool)
	for _, p := range prompts {
		// Derive category from ID prefix.
		for i, c := range p.ID {
			if c == '-' {
				categories[p.ID[:i]] = true
				break
			}
		}
	}
	if len(categories) < 2 {
		t.Errorf("expected prompts from multiple categories, got %d", len(categories))
	}
}

func TestLoadSmoke(t *testing.T) {
	prompts, err := LoadSmoke()
	if err != nil {
		t.Fatalf("LoadSmoke: %v", err)
	}
	if len(prompts) == 0 {
		t.Error("expected at least one smoke-test prompt")
	}
}

func TestCategories(t *testing.T) {
	cats := Categories()
	if len(cats) == 0 {
		t.Fatal("expected at least one category")
	}
	// Should include smoke-test.
	found := false
	for _, c := range cats {
		if c == "smoke-test" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected 'smoke-test' in categories")
	}

	// Should be sorted.
	for i := 1; i < len(cats); i++ {
		if cats[i] < cats[i-1] {
			t.Errorf("categories not sorted: %q < %q", cats[i], cats[i-1])
			break
		}
	}
}

func TestLoadCategory_AllCategoriesLoadable(t *testing.T) {
	for _, cat := range allCategories {
		prompts, err := LoadCategory(cat)
		if err != nil {
			t.Errorf("LoadCategory(%s): %v", cat, err)
			continue
		}
		if len(prompts) == 0 {
			t.Errorf("category %s has no prompts", cat)
		}
	}
}
