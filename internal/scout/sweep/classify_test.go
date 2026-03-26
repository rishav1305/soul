package sweep

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/rishav1305/soul/internal/scout/store"
)

func TestLoadDreamCompanies(t *testing.T) {
	companies := []DreamCompany{
		{Name: "Anthropic", Domain: "anthropic.com"},
		{Name: "OpenAI", Domain: "openai.com"},
	}
	data, _ := json.Marshal(companies)
	path := filepath.Join(t.TempDir(), "dream.json")
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatal(err)
	}

	got, err := LoadDreamCompanies(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d companies, want 2", len(got))
	}
	if got[0].Name != "Anthropic" || got[0].Domain != "anthropic.com" {
		t.Errorf("company[0] = %+v", got[0])
	}
	if got[1].Name != "OpenAI" || got[1].Domain != "openai.com" {
		t.Errorf("company[1] = %+v", got[1])
	}
}

func TestLoadDreamCompanies_FileNotFound(t *testing.T) {
	_, err := LoadDreamCompanies("/nonexistent/path.json")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestLoadDreamCompanies_InvalidJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bad.json")
	if err := os.WriteFile(path, []byte("not json"), 0644); err != nil {
		t.Fatal(err)
	}
	_, err := LoadDreamCompanies(path)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func dreamCompanies() []DreamCompany {
	return []DreamCompany{
		{Name: "Anthropic", Domain: "anthropic.com"},
		{Name: "Google DeepMind", Domain: "deepmind.google"},
		{Name: "OpenAI", Domain: "openai.com"},
	}
}

func TestClassifyTier_DreamCompanyDomain(t *testing.T) {
	lead := store.Lead{CompanyDomain: "anthropic.com"}
	tier := ClassifyTier(lead, dreamCompanies())
	if tier != 1 {
		t.Errorf("Anthropic domain = tier %d, want 1", tier)
	}
}

func TestClassifyTier_DreamCompanyDomain_CaseInsensitive(t *testing.T) {
	lead := store.Lead{CompanyDomain: "Anthropic.COM"}
	tier := ClassifyTier(lead, dreamCompanies())
	if tier != 1 {
		t.Errorf("Anthropic domain (mixed case) = tier %d, want 1", tier)
	}
}

func TestClassifyTier_HighFunding(t *testing.T) {
	lead := store.Lead{
		CompanyTotalFundingUSD: 100_000_000,
		CompanyDomain:          "somecompany.com",
	}
	tier := ClassifyTier(lead, dreamCompanies())
	if tier != 1 {
		t.Errorf("$100M funding = tier %d, want 1", tier)
	}
}

func TestClassifyTier_FundingBoundary(t *testing.T) {
	// Exactly $50M should NOT be tier 1 (must be > $50M).
	lead := store.Lead{
		CompanyTotalFundingUSD: 50_000_000,
		CompanyDomain:          "somecompany.com",
	}
	tier := ClassifyTier(lead, dreamCompanies())
	if tier == 1 {
		t.Errorf("exactly $50M funding = tier %d, want 2 or 3", tier)
	}
}

func TestClassifyTier_LargeAICompany(t *testing.T) {
	lead := store.Lead{
		CompanyEmployeeCount: 1000,
		CompanyIndustry:      "AI/ML Research",
		CompanyDomain:        "someai.com",
	}
	tier := ClassifyTier(lead, dreamCompanies())
	if tier != 1 {
		t.Errorf("1000 employees + AI industry = tier %d, want 1", tier)
	}
}

func TestClassifyTier_LargeAICompany_CaseVariants(t *testing.T) {
	tests := []struct {
		industry string
	}{
		{"AI Research"},
		{"Enterprise AI"},
		{"ai platform"},
		{"Applied AI/ML"},
	}
	for _, tt := range tests {
		t.Run(tt.industry, func(t *testing.T) {
			lead := store.Lead{
				CompanyEmployeeCount: 600,
				CompanyIndustry:      tt.industry,
				CompanyDomain:        "test.com",
			}
			tier := ClassifyTier(lead, dreamCompanies())
			if tier != 1 {
				t.Errorf("industry %q + 600 employees = tier %d, want 1", tt.industry, tier)
			}
		})
	}
}

func TestClassifyTier_LargeNonAICompany(t *testing.T) {
	lead := store.Lead{
		CompanyEmployeeCount: 1000,
		CompanyIndustry:      "Finance",
		CompanyDomain:        "bigbank.com",
	}
	tier := ClassifyTier(lead, dreamCompanies())
	if tier == 1 {
		t.Errorf("1000 employees + Finance = tier %d, want 2 or 3", tier)
	}
}

func TestClassifyTier_FrontierAITech(t *testing.T) {
	tests := []struct {
		name string
		tech string
	}{
		{"anthropic", `["anthropic","python"]`},
		{"openai", `["openai","typescript"]`},
		{"deepmind", `["deepmind","jax"]`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lead := store.Lead{
				TechnologySlugs: tt.tech,
				CompanyDomain:   "unknown.com",
			}
			tier := ClassifyTier(lead, dreamCompanies())
			if tier != 1 {
				t.Errorf("%s tech = tier %d, want 1", tt.name, tier)
			}
		})
	}
}

func TestClassifyTier_Tier2_FundedAIStartup(t *testing.T) {
	lead := store.Lead{
		CompanyFundingStage:  "series_a",
		CompanyEmployeeCount: 100,
		TechnologySlugs:      `["python","pytorch","langchain"]`,
		CompanyDomain:        "aistart.io",
	}
	tier := ClassifyTier(lead, dreamCompanies())
	if tier != 2 {
		t.Errorf("series_a + 100 employees + AI tech = tier %d, want 2", tier)
	}
}

func TestClassifyTier_Tier2_SeriesB(t *testing.T) {
	lead := store.Lead{
		CompanyFundingStage:  "series_b",
		CompanyEmployeeCount: 250,
		TechnologySlugs:      `["python","huggingface"]`,
		CompanyDomain:        "mlcompany.ai",
	}
	tier := ClassifyTier(lead, dreamCompanies())
	if tier != 2 {
		t.Errorf("series_b + 250 employees + AI tech = tier %d, want 2", tier)
	}
}

func TestClassifyTier_Tier2_SeedWithAI(t *testing.T) {
	lead := store.Lead{
		CompanyFundingStage:  "seed",
		CompanyEmployeeCount: 50,
		TechnologySlugs:      `["python","tensorflow"]`,
		CompanyDomain:        "seedai.io",
	}
	tier := ClassifyTier(lead, dreamCompanies())
	if tier != 2 {
		t.Errorf("seed + 50 employees + AI tech = tier %d, want 2", tier)
	}
}

func TestClassifyTier_NotTier2_TooSmall(t *testing.T) {
	lead := store.Lead{
		CompanyFundingStage:  "series_a",
		CompanyEmployeeCount: 10,
		TechnologySlugs:      `["python","pytorch"]`,
		CompanyDomain:        "tiny.io",
	}
	tier := ClassifyTier(lead, dreamCompanies())
	if tier != 3 {
		t.Errorf("series_a + 10 employees (too small) = tier %d, want 3", tier)
	}
}

func TestClassifyTier_NotTier2_TooLarge(t *testing.T) {
	lead := store.Lead{
		CompanyFundingStage:  "series_a",
		CompanyEmployeeCount: 600,
		TechnologySlugs:      `["python","pytorch"]`,
		CompanyDomain:        "bigstartup.io",
	}
	tier := ClassifyTier(lead, dreamCompanies())
	// 600 employees > 500, but no AI in industry and funding < $50M => tier 3
	if tier == 2 {
		t.Errorf("series_a + 600 employees (too large for tier 2) = tier %d, want 3", tier)
	}
}

func TestClassifyTier_NotTier2_NoAITech(t *testing.T) {
	lead := store.Lead{
		CompanyFundingStage:  "series_a",
		CompanyEmployeeCount: 100,
		TechnologySlugs:      `["java","spring"]`,
		CompanyDomain:        "noai.io",
	}
	tier := ClassifyTier(lead, dreamCompanies())
	if tier != 3 {
		t.Errorf("series_a + no AI tech = tier %d, want 3", tier)
	}
}

func TestClassifyTier_Tier3_Unknown(t *testing.T) {
	lead := store.Lead{
		CompanyDomain:        "random.com",
		CompanyEmployeeCount: 20,
		CompanyIndustry:      "Retail",
		TechnologySlugs:      `["java","mysql"]`,
	}
	tier := ClassifyTier(lead, dreamCompanies())
	if tier != 3 {
		t.Errorf("unknown small company = tier %d, want 3", tier)
	}
}

func TestClassifyTier_EmptyLead(t *testing.T) {
	lead := store.Lead{}
	tier := ClassifyTier(lead, dreamCompanies())
	if tier != 3 {
		t.Errorf("empty lead = tier %d, want 3", tier)
	}
}

func TestClassifyTier_NoDreamCompanies(t *testing.T) {
	lead := store.Lead{CompanyDomain: "anthropic.com"}
	// Without dream companies list, domain match doesn't trigger tier 1 via domain.
	// But anthropic.com doesn't match by funding/size/tech rules here.
	tier := ClassifyTier(lead, nil)
	if tier != 3 {
		t.Errorf("no dream companies + anthropic domain = tier %d, want 3", tier)
	}
}

// Regression tests for large enterprise companies (Deutsche Bank, Amazon, Wells Fargo
// style) that were incorrectly classified as Tier 3 because their industry field does
// not contain "AI" even though they use AI technology and have 500+ employees.

func TestClassifyTier_LargeEnterpriseWithAITech_Tier1(t *testing.T) {
	tests := []struct {
		name     string
		industry string
		slugs    string
	}{
		{"financial services with python/pytorch", "Financial Services", `["python","pytorch","langchain"]`},
		{"banking with openai SDK", "Banking", `["python","openai","typescript"]`},
		{"professional services with AI tools", "Professional Services", `["python","tensorflow","huggingface"]`},
		{"technology with LLM stack", "Technology", `["python","llm","langchain","qdrant"]`},
		{"retail tech with AI infra", "Retail Technology", `["python","pytorch","mlflow","ray"]`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lead := store.Lead{
				CompanyEmployeeCount: 1000,
				CompanyIndustry:      tt.industry,
				TechnologySlugs:      tt.slugs,
				CompanyDomain:        "bigcorp.com",
			}
			tier := ClassifyTier(lead, dreamCompanies())
			if tier != 1 {
				t.Errorf("1000 employees + %q industry + AI tech slugs = tier %d, want 1", tt.industry, tier)
			}
		})
	}
}

func TestClassifyTier_LargeEnterpriseNoAITech_NotTier1(t *testing.T) {
	// Large company with no AI tech signals should NOT be Tier 1.
	lead := store.Lead{
		CompanyEmployeeCount: 1000,
		CompanyIndustry:      "Financial Services",
		TechnologySlugs:      `["java","spring","oracle"]`,
		CompanyDomain:        "legacy-bank.com",
	}
	tier := ClassifyTier(lead, dreamCompanies())
	if tier == 1 {
		t.Errorf("1000 employees + Financial Services + NO AI tech = tier %d, want 2 or 3", tier)
	}
}

func TestClassifyTier_LargeEnterpriseAITech_BoundaryEmployee(t *testing.T) {
	// Exactly 500 employees with AI tech — boundary: must be > 500 to qualify.
	lead := store.Lead{
		CompanyEmployeeCount: 500,
		CompanyIndustry:      "Financial Services",
		TechnologySlugs:      `["python","pytorch"]`,
		CompanyDomain:        "boundary.com",
	}
	tier := ClassifyTier(lead, dreamCompanies())
	if tier == 1 {
		t.Errorf("exactly 500 employees + AI tech = tier %d, want 2 or 3 (boundary is >500)", tier)
	}
}
