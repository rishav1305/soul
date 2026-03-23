package sweep

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/rishav1305/soul-v2/internal/scout/store"
)

// DreamCompany represents a target company for tier-1 classification.
type DreamCompany struct {
	Name   string `json:"name"`
	Domain string `json:"domain"`
}

// LoadDreamCompanies reads a JSON file and returns the list of dream companies.
func LoadDreamCompanies(path string) ([]DreamCompany, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("load dream companies: %w", err)
	}
	var companies []DreamCompany
	if err := json.Unmarshal(data, &companies); err != nil {
		return nil, fmt.Errorf("parse dream companies: %w", err)
	}
	return companies, nil
}

// aiTechSlugs are technology slugs that indicate AI-related work.
var aiTechSlugs = []string{
	"anthropic", "openai", "deepmind",
	"python", "pytorch", "tensorflow", "jax",
	"huggingface", "transformers", "langchain", "llamaindex",
	"llm", "rag", "vector-database",
	"pinecone", "weaviate", "chromadb", "qdrant",
	"mlflow", "kubeflow", "ray", "wandb",
}

// ClassifyTier assigns a tier (1, 2, or 3) to a lead based on company signals.
//
// Tier 1 rules:
//   - Company domain matches a dream company
//   - Total funding > $50M
//   - Employee count > 500 AND industry contains "AI" (case-insensitive)
//   - Technology slugs contain "anthropic", "openai", or "deepmind"
//
// Tier 2 rules:
//   - Funding stage is seed/series_a/series_b/series_c AND has AI tech slugs AND 50-500 employees
//
// Tier 3: everything else.
func ClassifyTier(lead store.Lead, dreamCompanies []DreamCompany) int {
	// Tier 1: dream company domain match.
	if lead.CompanyDomain != "" {
		domain := strings.ToLower(lead.CompanyDomain)
		for _, dc := range dreamCompanies {
			if strings.ToLower(dc.Domain) == domain {
				return 1
			}
		}
	}

	// Tier 1: high funding.
	if lead.CompanyTotalFundingUSD > 50_000_000 {
		return 1
	}

	// Slugs are needed by both the large-company check and the frontier-tech check below.
	slugs := strings.ToLower(lead.TechnologySlugs)

	// Tier 1: large company with AI signals.
	// Matches if industry explicitly says "AI" OR if the technology stack contains
	// any AI-related tooling — catches enterprises like financial/banking/professional
	// services companies that hire for AI without calling themselves "AI companies".
	if lead.CompanyEmployeeCount > 500 && (containsCI(lead.CompanyIndustry, "AI") || hasAITechSlugs(slugs)) {
		return 1
	}

	// Tier 1: uses frontier AI provider tech.
	if strings.Contains(slugs, "anthropic") || strings.Contains(slugs, "openai") || strings.Contains(slugs, "deepmind") {
		return 1
	}

	// Tier 2: funded startup with AI tech and medium size.
	tier2Stages := map[string]bool{
		"seed":     true,
		"series_a": true,
		"series_b": true,
		"series_c": true,
	}
	if tier2Stages[strings.ToLower(lead.CompanyFundingStage)] &&
		lead.CompanyEmployeeCount >= 50 && lead.CompanyEmployeeCount <= 500 &&
		hasAITechSlugs(slugs) {
		return 2
	}

	return 3
}

// containsCI checks if s contains substr, case-insensitive.
func containsCI(s, substr string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(substr))
}

// hasAITechSlugs checks if the technology slugs string contains any AI-related tech.
func hasAITechSlugs(slugs string) bool {
	for _, tech := range aiTechSlugs {
		if strings.Contains(slugs, tech) {
			return true
		}
	}
	return false
}
