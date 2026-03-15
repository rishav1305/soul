package rules

import (
	"testing"
)

func TestLoadAll_NoFilter(t *testing.T) {
	rules, err := LoadAll(nil)
	if err != nil {
		t.Fatalf("LoadAll(nil) error: %v", err)
	}
	if len(rules) == 0 {
		t.Fatal("LoadAll(nil) returned 0 rules, expected > 0")
	}
	// We have 10 soc2 + 5 hipaa + 5 gdpr = 20 rules
	if len(rules) != 20 {
		t.Errorf("LoadAll(nil) returned %d rules, expected 20", len(rules))
	}
}

func TestLoadAll_FilterSOC2(t *testing.T) {
	rules, err := LoadAll([]string{"soc2"})
	if err != nil {
		t.Fatalf("LoadAll(soc2) error: %v", err)
	}
	if len(rules) != 10 {
		t.Errorf("LoadAll(soc2) returned %d rules, expected 10", len(rules))
	}
	for _, r := range rules {
		found := false
		for _, fw := range r.Framework {
			if fw == "soc2" {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("rule %s has frameworks %v, expected soc2", r.ID, r.Framework)
		}
	}
}

func TestLoadAll_FilterHIPAA(t *testing.T) {
	rules, err := LoadAll([]string{"hipaa"})
	if err != nil {
		t.Fatalf("LoadAll(hipaa) error: %v", err)
	}
	if len(rules) != 5 {
		t.Errorf("LoadAll(hipaa) returned %d rules, expected 5", len(rules))
	}
}

func TestLoadAll_FilterGDPR(t *testing.T) {
	rules, err := LoadAll([]string{"gdpr"})
	if err != nil {
		t.Fatalf("LoadAll(gdpr) error: %v", err)
	}
	if len(rules) != 5 {
		t.Errorf("LoadAll(gdpr) returned %d rules, expected 5", len(rules))
	}
}

func TestLoadAll_EmptyFilter(t *testing.T) {
	rules, err := LoadAll([]string{})
	if err != nil {
		t.Fatalf("LoadAll([]) error: %v", err)
	}
	if len(rules) != 20 {
		t.Errorf("LoadAll([]) returned %d rules, expected 20", len(rules))
	}
}

func TestLoadAll_RuleFields(t *testing.T) {
	rules, err := LoadAll(nil)
	if err != nil {
		t.Fatalf("LoadAll(nil) error: %v", err)
	}
	for _, r := range rules {
		if r.ID == "" {
			t.Error("rule has empty ID")
		}
		if r.Title == "" {
			t.Errorf("rule %s has empty Title", r.ID)
		}
		if r.Severity == "" {
			t.Errorf("rule %s has empty Severity", r.ID)
		}
		if r.Analyzer == "" {
			t.Errorf("rule %s has empty Analyzer", r.ID)
		}
		if r.Pattern == "" {
			t.Errorf("rule %s has empty Pattern", r.ID)
		}
		if len(r.Controls) == 0 {
			t.Errorf("rule %s has no Controls", r.ID)
		}
		if len(r.Framework) == 0 {
			t.Errorf("rule %s has no Framework", r.ID)
		}
		if r.Description == "" {
			t.Errorf("rule %s has empty Description", r.ID)
		}
		validSeverities := map[string]bool{
			"critical": true, "high": true, "medium": true, "low": true, "info": true,
		}
		if !validSeverities[r.Severity] {
			t.Errorf("rule %s has invalid severity %q", r.ID, r.Severity)
		}
	}
}
