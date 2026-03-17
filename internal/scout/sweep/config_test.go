package sweep

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfig_Default(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sweep-config.json")

	// File does not exist — should get defaults and create the file
	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig returned error: %v", err)
	}

	// Verify defaults
	if len(cfg.JobTitleOr) == 0 {
		t.Error("expected non-empty JobTitleOr")
	}
	if cfg.Remote == nil || !*cfg.Remote {
		t.Error("expected Remote to be true")
	}
	if cfg.Limit != 50 {
		t.Errorf("expected Limit=50, got %d", cfg.Limit)
	}
	if cfg.PostedAtMaxAgeDays != 7 {
		t.Errorf("expected PostedAtMaxAgeDays=7, got %d", cfg.PostedAtMaxAgeDays)
	}
	if cfg.IntervalHours != 24 {
		t.Errorf("expected IntervalHours=24, got %d", cfg.IntervalHours)
	}
	if cfg.CreditBudget != 50 {
		t.Errorf("expected CreditBudget=50, got %d", cfg.CreditBudget)
	}
	if cfg.AutoScoreThreshold != 70 {
		t.Errorf("expected AutoScoreThreshold=70, got %f", cfg.AutoScoreThreshold)
	}

	// Verify file was created
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Error("expected config file to be created, but it does not exist")
	}
}

func TestLoadConfig_Existing(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sweep-config.json")

	minSalary := 80000.0
	remote := false
	custom := &SweepConfig{
		JobTitleOr:          []string{"site reliability engineer"},
		JobCountryCodeOr:    []string{"US"},
		JobTechnologySlugOr: []string{"kubernetes", "terraform"},
		Remote:              &remote,
		MinSalaryUSD:        &minSalary,
		PostedAtMaxAgeDays:  14,
		Limit:               25,
		IntervalHours:       12,
		CreditBudget:        100,
		AutoScoreThreshold:  85,
	}

	data, err := json.MarshalIndent(custom, "", "  ")
	if err != nil {
		t.Fatalf("failed to marshal custom config: %v", err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatalf("failed to write custom config: %v", err)
	}

	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig returned error: %v", err)
	}

	if len(cfg.JobTitleOr) != 1 || cfg.JobTitleOr[0] != "site reliability engineer" {
		t.Errorf("unexpected JobTitleOr: %v", cfg.JobTitleOr)
	}
	if cfg.Remote == nil || *cfg.Remote != false {
		t.Error("expected Remote to be false")
	}
	if cfg.MinSalaryUSD == nil || *cfg.MinSalaryUSD != 80000.0 {
		t.Error("expected MinSalaryUSD=80000")
	}
	if cfg.PostedAtMaxAgeDays != 14 {
		t.Errorf("expected PostedAtMaxAgeDays=14, got %d", cfg.PostedAtMaxAgeDays)
	}
	if cfg.Limit != 25 {
		t.Errorf("expected Limit=25, got %d", cfg.Limit)
	}
	if cfg.AutoScoreThreshold != 85 {
		t.Errorf("expected AutoScoreThreshold=85, got %f", cfg.AutoScoreThreshold)
	}
}

func TestSaveConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "subdir", "sweep-config.json")

	minSalary := 120000.0
	remote := true
	original := &SweepConfig{
		JobTitleOr:          []string{"platform engineer", "devops engineer"},
		JobTitleNot:         []string{"intern"},
		JobCountryCodeOr:    []string{"DE", "NL"},
		JobTechnologySlugOr: []string{"go", "kubernetes"},
		Remote:              &remote,
		MinSalaryUSD:        &minSalary,
		SeniorityOr:         []string{"senior", "lead"},
		PostedAtMaxAgeDays:  3,
		Limit:               100,
		IntervalHours:       6,
		CreditBudget:        200,
		AutoScoreThreshold:  90,
	}

	if err := SaveConfig(path, original); err != nil {
		t.Fatalf("SaveConfig returned error: %v", err)
	}

	// Reload and verify roundtrip
	loaded, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig returned error: %v", err)
	}

	if len(loaded.JobTitleOr) != 2 || loaded.JobTitleOr[0] != "platform engineer" {
		t.Errorf("unexpected JobTitleOr after roundtrip: %v", loaded.JobTitleOr)
	}
	if len(loaded.JobTitleNot) != 1 || loaded.JobTitleNot[0] != "intern" {
		t.Errorf("unexpected JobTitleNot after roundtrip: %v", loaded.JobTitleNot)
	}
	if loaded.Remote == nil || !*loaded.Remote {
		t.Error("expected Remote=true after roundtrip")
	}
	if loaded.MinSalaryUSD == nil || *loaded.MinSalaryUSD != 120000.0 {
		t.Error("expected MinSalaryUSD=120000 after roundtrip")
	}
	if len(loaded.SeniorityOr) != 2 {
		t.Errorf("unexpected SeniorityOr after roundtrip: %v", loaded.SeniorityOr)
	}
	if loaded.PostedAtMaxAgeDays != 3 {
		t.Errorf("expected PostedAtMaxAgeDays=3, got %d", loaded.PostedAtMaxAgeDays)
	}
	if loaded.CreditBudget != 200 {
		t.Errorf("expected CreditBudget=200, got %d", loaded.CreditBudget)
	}
	if loaded.AutoScoreThreshold != 90 {
		t.Errorf("expected AutoScoreThreshold=90, got %f", loaded.AutoScoreThreshold)
	}
}
