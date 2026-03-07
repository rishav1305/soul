package rules

import (
	"strings"
	"testing"
)

func TestLoadAllRules(t *testing.T) {
	rules := Load(nil)
	if len(rules) < 80 {
		t.Errorf("Load(nil) returned %d rules, want >= 80", len(rules))
	}
	t.Logf("Load(nil) returned %d rules", len(rules))
}

func TestLoadRulesFilterByFramework(t *testing.T) {
	all := Load(nil)
	soc2Only := Load([]string{"soc2"})

	if len(soc2Only) == 0 {
		t.Fatal("Load([soc2]) returned 0 rules, want > 0")
	}
	if len(soc2Only) >= len(all) {
		t.Errorf("Load([soc2]) returned %d rules, expected fewer than Load(nil) which returned %d",
			len(soc2Only), len(all))
	}

	for _, r := range soc2Only {
		found := false
		for _, fw := range r.Framework {
			if strings.EqualFold(fw, "soc2") {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("rule %s has frameworks %v, expected to contain soc2", r.ID, r.Framework)
		}
	}

	t.Logf("Load([soc2]) returned %d rules out of %d total", len(soc2Only), len(all))
}

func TestLoadRulesFilterByMultipleFrameworks(t *testing.T) {
	hipaa := Load([]string{"hipaa"})
	gdpr := Load([]string{"gdpr"})
	both := Load([]string{"hipaa", "gdpr"})

	if len(both) == 0 {
		t.Fatal("Load([hipaa, gdpr]) returned 0 rules")
	}

	// The combined filter should return at least as many as the larger single filter
	max := len(hipaa)
	if len(gdpr) > max {
		max = len(gdpr)
	}
	if len(both) < max {
		t.Errorf("Load([hipaa, gdpr]) returned %d rules, expected >= %d", len(both), max)
	}

	for _, r := range both {
		hasHipaa := false
		hasGdpr := false
		for _, fw := range r.Framework {
			if strings.EqualFold(fw, "hipaa") {
				hasHipaa = true
			}
			if strings.EqualFold(fw, "gdpr") {
				hasGdpr = true
			}
		}
		if !hasHipaa && !hasGdpr {
			t.Errorf("rule %s has frameworks %v, expected to contain hipaa or gdpr", r.ID, r.Framework)
		}
	}

	t.Logf("hipaa=%d, gdpr=%d, both=%d", len(hipaa), len(gdpr), len(both))
}

func TestLoadEmptyFrameworkSlice(t *testing.T) {
	all := Load(nil)
	empty := Load([]string{})

	if len(empty) != len(all) {
		t.Errorf("Load([]) returned %d rules, expected %d (same as Load(nil))", len(empty), len(all))
	}
}

func TestRuleFields(t *testing.T) {
	rules := Load(nil)
	for _, r := range rules {
		if r.ID == "" {
			t.Errorf("rule has empty ID: %+v", r)
		}
		if r.Severity == "" {
			t.Errorf("rule %s has empty Severity", r.ID)
		}
		if r.Analyzer == "" {
			t.Errorf("rule %s has empty Analyzer", r.ID)
		}
		if len(r.Framework) == 0 {
			t.Errorf("rule %s has no Framework entries", r.ID)
		}
	}
}
