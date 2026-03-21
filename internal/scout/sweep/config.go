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
	return &SweepConfig{
		JobTitleOr: []string{
			// AI/ML Core
			"ai engineer", "ml engineer", "machine learning engineer",
			"artificial intelligence engineer", "deep learning engineer",
			"llm engineer", "generative ai engineer", "nlp engineer",
			"applied scientist", "research engineer", "ai research engineer",
			"applied ml engineer", "computer vision engineer",
			// Platform & Architecture
			"ai architect", "ml architect", "ai solutions architect",
			"ai platform engineer", "ml platform engineer",
			"mlops engineer", "ai infrastructure engineer",
			// Senior/Staff/Lead
			"senior ai engineer", "staff ml engineer", "principal ml engineer",
			"ai tech lead", "ai team lead", "lead ml engineer",
			"head of ai", "head of machine learning",
			// Full Stack + AI Hybrid
			"full stack ai engineer", "ai product engineer",
			"ai developer", "ai software engineer",
			// Data Science (senior)
			"senior data scientist", "lead data scientist",
			"staff data scientist", "principal data scientist",
			// Broader senior engineering (catches AI-adjacent)
			"senior software engineer", "staff software engineer",
			"principal engineer", "senior backend engineer",
			"senior full stack engineer",
		},
		JobTitleNot: []string{
			"intern", "internship", "junior", "entry level",
			"fresher", "trainee", "associate", "graduate",
		},
		// SeniorityOr omitted — TheirStack filters by title match, not seniority field.
		// Senior/staff/lead already covered by JobTitleOr entries.
		JobCountryCodeOr: []string{
			"IN",             // India — primary
			"US", "CA",       // North America — remote
			"GB", "DE", "NL", // Europe — remote-friendly
			"SG", "AE",       // Asia — accessible timezone
			"AU",             // Pacific
		},
		JobTechnologySlugOr: []string{
			// AI/ML Frameworks
			"python", "pytorch", "tensorflow", "jax",
			"huggingface", "transformers", "langchain", "llamaindex",
			// LLM & GenAI
			"openai", "anthropic", "claude", "gpt",
			"llm", "rag", "vector-database",
			"pinecone", "weaviate", "chromadb", "qdrant",
			// ML Infrastructure
			"mlflow", "kubeflow", "ray", "dvc",
			"wandb", "weights-and-biases",
			// Backend (your strengths)
			"go", "golang", "fastapi", "flask",
			"postgresql", "redis", "elasticsearch",
			// Infrastructure
			"kubernetes", "docker", "aws", "gcp", "azure",
			"terraform",
			// Frontend (your stack)
			"react", "typescript", "nextjs",
		},
		JobLocationPatternOr: []string{
			"remote", "india", "delhi", "bangalore", "bengaluru",
			"hyderabad", "mumbai", "pune", "gurgaon", "gurugram",
			"noida",
		},
		PostedAtMaxAgeDays: 7,
		Limit:              50,
		IntervalHours:      24,
		CreditBudget:       50,
		AutoScoreThreshold: 70,
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
