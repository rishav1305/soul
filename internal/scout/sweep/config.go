package sweep

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type SweepConfig struct {
	JobTitleOr           []string `json:"job_title_or"`
	JobTitleNot          []string `json:"job_title_not,omitempty"`
	JobCountryCodeOr     []string `json:"job_country_code_or"`
	JobTechnologySlugOr  []string `json:"job_technology_slug_or"`
	JobLocationPatternOr []string `json:"job_location_pattern_or,omitempty"`
	Remote               *bool    `json:"remote,omitempty"`
	SeniorityOr          []string `json:"seniority_or,omitempty"`
	MinSalaryUSD         *float64 `json:"min_salary_usd,omitempty"`
	PostedAtMaxAgeDays   int      `json:"posted_at_max_age_days"`
	Limit                int      `json:"limit"`
	IntervalHours        int      `json:"interval_hours"`
	CreditBudget         int      `json:"credit_budget"`
	AutoScoreThreshold   float64  `json:"auto_score_threshold"`
}

func DefaultConfig() *SweepConfig {
	remote := true
	return &SweepConfig{
		JobTitleOr:          []string{"software engineer", "full stack developer", "backend engineer", "golang developer"},
		JobCountryCodeOr:    []string{"IN", "US", "GB", "DE", "NL", "SG"},
		JobTechnologySlugOr: []string{"go", "react", "typescript", "python", "postgresql"},
		Remote:              &remote,
		PostedAtMaxAgeDays:  7,
		Limit:               50,
		IntervalHours:       24,
		CreditBudget:        50,
		AutoScoreThreshold:  70,
	}
}

func LoadConfig(path string) (*SweepConfig, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		cfg := DefaultConfig()
		if err := SaveConfig(path, cfg); err != nil {
			return nil, err
		}
		return cfg, nil
	}
	if err != nil {
		return nil, err
	}
	var cfg SweepConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	cfg.backfillDefaults()
	return &cfg, nil
}

// backfillDefaults ensures critical fields have safe values.
func (c *SweepConfig) backfillDefaults() {
	if c.IntervalHours <= 0 {
		c.IntervalHours = 24
	}
	if c.Limit <= 0 {
		c.Limit = 50
	}
	if c.CreditBudget <= 0 {
		c.CreditBudget = 50
	}
	if c.PostedAtMaxAgeDays <= 0 {
		c.PostedAtMaxAgeDays = 7
	}
}

func SaveConfig(path string, cfg *SweepConfig) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}
