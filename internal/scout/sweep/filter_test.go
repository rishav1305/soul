package sweep

import (
	"net/http"
	"path/filepath"
	"testing"

	"github.com/rishav1305/soul-v2/internal/scout/store"
)

// makeJob creates a Job with sensible defaults that passes all filters.
// Tests override individual fields as needed.
func makeJob() Job {
	return Job{
		ID:              999,
		JobTitle:        "Senior AI Engineer",
		Company:         "AcmeLLM",
		CompanyDomain:   "acmellm.io",
		Seniority:       "senior",
		TechnologySlugs: []string{"langchain", "openai", "python"},
		CompanyObject: &CompanyInfo{
			EmployeeCount: 200,
			Industry:      "Software",
			Domain:        "acmellm.io",
		},
	}
}

func newFilterTestStore(t *testing.T) *store.Store {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "filter_test.db"))
	if err != nil {
		t.Fatalf("open filter test store: %v", err)
	}
	t.Cleanup(func() { st.Close() })
	return st
}

// --- Filter 1: Minimum Company Size ---

func TestShouldFilter_CompanySizeBelow50(t *testing.T) {
	st := newFilterTestStore(t)
	job := makeJob()
	job.CompanyObject.EmployeeCount = 30

	filtered, reason := ShouldFilter(job, st)
	if !filtered {
		t.Errorf("expected filtered=true for employee_count=30, got false")
	}
	if reason != FilterReasonCompanyTooSmall {
		t.Errorf("expected reason %q, got %q", FilterReasonCompanyTooSmall, reason)
	}
}

func TestShouldFilter_CompanySizeExactly50(t *testing.T) {
	st := newFilterTestStore(t)
	job := makeJob()
	job.CompanyObject.EmployeeCount = 50

	filtered, _ := ShouldFilter(job, st)
	if filtered {
		t.Errorf("expected filtered=false for employee_count=50 (boundary), got true")
	}
}

func TestShouldFilter_CompanySizeZeroMeansUnknown(t *testing.T) {
	st := newFilterTestStore(t)
	job := makeJob()
	job.CompanyObject.EmployeeCount = 0

	filtered, _ := ShouldFilter(job, st)
	if filtered {
		t.Errorf("expected filtered=false for employee_count=0 (unknown), got true")
	}
}

func TestShouldFilter_NoCompanyObject_NotFiltered(t *testing.T) {
	st := newFilterTestStore(t)
	job := makeJob()
	job.CompanyObject = nil

	filtered, _ := ShouldFilter(job, st)
	if filtered {
		t.Errorf("expected filtered=false when CompanyObject is nil, got true")
	}
}

// --- Filter 2: LLM/GenAI Technology Filter ---

func TestShouldFilter_NoLLMTech(t *testing.T) {
	st := newFilterTestStore(t)
	job := makeJob()
	job.TechnologySlugs = []string{"react", "postgresql", "kubernetes"}

	filtered, reason := ShouldFilter(job, st)
	if !filtered {
		t.Errorf("expected filtered=true for no LLM tech slugs, got false")
	}
	if reason != FilterReasonNoLLMTech {
		t.Errorf("expected reason %q, got %q", FilterReasonNoLLMTech, reason)
	}
}

func TestShouldFilter_HasLangChain(t *testing.T) {
	st := newFilterTestStore(t)
	job := makeJob()
	job.TechnologySlugs = []string{"langchain"}

	filtered, _ := ShouldFilter(job, st)
	if filtered {
		t.Errorf("expected filtered=false for langchain slug, got true")
	}
}

func TestShouldFilter_HasOpenAI(t *testing.T) {
	st := newFilterTestStore(t)
	job := makeJob()
	job.TechnologySlugs = []string{"openai", "react"}

	filtered, _ := ShouldFilter(job, st)
	if filtered {
		t.Errorf("expected filtered=false for openai slug, got true")
	}
}

func TestShouldFilter_HasHuggingFace(t *testing.T) {
	st := newFilterTestStore(t)
	job := makeJob()
	job.TechnologySlugs = []string{"hugging-face"}

	filtered, _ := ShouldFilter(job, st)
	if filtered {
		t.Errorf("expected filtered=false for hugging-face slug, got true")
	}
}

func TestShouldFilter_AllLLMSlugsAccepted(t *testing.T) {
	st := newFilterTestStore(t)
	llmSlugs := []string{
		"langchain", "langgraph", "claude", "anthropic", "openai",
		"llm", "rag", "vector", "embeddings", "transformers", "hugging-face",
	}
	for _, slug := range llmSlugs {
		job := makeJob()
		job.TechnologySlugs = []string{slug}
		filtered, reason := ShouldFilter(job, st)
		if filtered {
			t.Errorf("slug %q: expected filtered=false, got true (reason: %q)", slug, reason)
		}
	}
}

func TestShouldFilter_EmptyTechSlugs(t *testing.T) {
	st := newFilterTestStore(t)
	job := makeJob()
	job.TechnologySlugs = []string{}

	filtered, reason := ShouldFilter(job, st)
	if !filtered {
		t.Errorf("expected filtered=true for empty tech slugs, got false")
	}
	if reason != FilterReasonNoLLMTech {
		t.Errorf("expected reason %q, got %q", FilterReasonNoLLMTech, reason)
	}
}

// --- Filter 3: Staffing/Recruiting Exclusion ---

func TestShouldFilter_StaffingIndustry(t *testing.T) {
	st := newFilterTestStore(t)
	job := makeJob()
	job.CompanyObject.Industry = "Staffing and Recruiting"

	filtered, reason := ShouldFilter(job, st)
	if !filtered {
		t.Errorf("expected filtered=true for Staffing and Recruiting, got false")
	}
	if reason != FilterReasonStaffingExcluded {
		t.Errorf("expected reason %q, got %q", FilterReasonStaffingExcluded, reason)
	}
}

func TestShouldFilter_EngineeringServicesSmallCompany(t *testing.T) {
	st := newFilterTestStore(t)
	job := makeJob()
	job.CompanyObject.Industry = "Engineering Services"
	job.CompanyObject.EmployeeCount = 50 // < 100

	filtered, reason := ShouldFilter(job, st)
	if !filtered {
		t.Errorf("expected filtered=true for Engineering Services + <100 employees, got false")
	}
	if reason != FilterReasonStaffingExcluded {
		t.Errorf("expected reason %q, got %q", FilterReasonStaffingExcluded, reason)
	}
}

func TestShouldFilter_EngineeringServicesLargeCompany(t *testing.T) {
	st := newFilterTestStore(t)
	job := makeJob()
	job.CompanyObject.Industry = "Engineering Services"
	job.CompanyObject.EmployeeCount = 100 // >= 100, not filtered

	filtered, _ := ShouldFilter(job, st)
	if filtered {
		t.Errorf("expected filtered=false for Engineering Services + 100 employees, got true")
	}
}

func TestShouldFilter_EngineeringServicesExactly100(t *testing.T) {
	st := newFilterTestStore(t)
	job := makeJob()
	job.CompanyObject.Industry = "Engineering Services"
	job.CompanyObject.EmployeeCount = 100

	filtered, _ := ShouldFilter(job, st)
	if filtered {
		t.Errorf("expected filtered=false for Engineering Services + exactly 100 employees (boundary), got true")
	}
}

func TestShouldFilter_EngineeringServicesUnknownCount(t *testing.T) {
	// EmployeeCount=0 means unknown — treat as not <100 for engineering services
	st := newFilterTestStore(t)
	job := makeJob()
	job.CompanyObject.Industry = "Engineering Services"
	job.CompanyObject.EmployeeCount = 0

	filtered, _ := ShouldFilter(job, st)
	if filtered {
		t.Errorf("expected filtered=false for Engineering Services + unknown (0) employee count, got true")
	}
}

// --- Filter 4: Duplicate Company Domain Check ---

func TestShouldFilter_DuplicateDomain(t *testing.T) {
	st := newFilterTestStore(t)

	// Insert an existing lead with domain "duplicate.com"
	existing := store.Lead{
		Source:        "theirstack",
		Pipeline:      "job",
		Stage:         "discovered",
		NextAction:    "review",
		JobTitle:      "ML Engineer",
		Company:       "Duplicate Corp",
		CompanyDomain: "duplicate.com",
	}
	_, err := st.AddLead(existing)
	if err != nil {
		t.Fatalf("setup: add existing lead: %v", err)
	}

	job := makeJob()
	job.CompanyDomain = "duplicate.com"
	job.CompanyObject.Domain = "duplicate.com"

	filtered, reason := ShouldFilter(job, st)
	if !filtered {
		t.Errorf("expected filtered=true for duplicate domain, got false")
	}
	if reason != FilterReasonDuplicateDomain {
		t.Errorf("expected reason %q, got %q", FilterReasonDuplicateDomain, reason)
	}
}

func TestShouldFilter_NoDuplicateDomain(t *testing.T) {
	st := newFilterTestStore(t)

	job := makeJob()
	job.CompanyDomain = "brandnew-domain.com"

	filtered, _ := ShouldFilter(job, st)
	if filtered {
		t.Errorf("expected filtered=false for new domain, got true")
	}
}

func TestShouldFilter_EmptyDomain_NotFilteredByDomain(t *testing.T) {
	st := newFilterTestStore(t)
	job := makeJob()
	job.CompanyDomain = ""
	job.CompanyObject.Domain = ""

	// Empty domain should not be deduplicated (could be many jobs without domain)
	filtered, reason := ShouldFilter(job, st)
	// If filtered, must not be for domain reason
	if filtered && reason == FilterReasonDuplicateDomain {
		t.Errorf("expected empty domain not to trigger FilterReasonDuplicateDomain, got reason %q", reason)
	}
}

// --- Filter 5: Role Seniority Filter ---

func TestShouldFilter_SeniorityIntern(t *testing.T) {
	st := newFilterTestStore(t)
	job := makeJob()
	job.Seniority = "intern"

	filtered, reason := ShouldFilter(job, st)
	if !filtered {
		t.Errorf("expected filtered=true for seniority=intern, got false")
	}
	if reason != FilterReasonSeniorityExcluded {
		t.Errorf("expected reason %q, got %q", FilterReasonSeniorityExcluded, reason)
	}
}

func TestShouldFilter_SeniorityEntryLevel(t *testing.T) {
	st := newFilterTestStore(t)
	job := makeJob()
	job.Seniority = "entry_level"

	filtered, reason := ShouldFilter(job, st)
	if !filtered {
		t.Errorf("expected filtered=true for seniority=entry_level, got false")
	}
	if reason != FilterReasonSeniorityExcluded {
		t.Errorf("expected reason %q, got %q", FilterReasonSeniorityExcluded, reason)
	}
}

func TestShouldFilter_SeniorityCLevel(t *testing.T) {
	st := newFilterTestStore(t)
	job := makeJob()
	job.Seniority = "c_level"

	filtered, reason := ShouldFilter(job, st)
	if !filtered {
		t.Errorf("expected filtered=true for seniority=c_level, got false")
	}
	if reason != FilterReasonSeniorityExcluded {
		t.Errorf("expected reason %q, got %q", FilterReasonSeniorityExcluded, reason)
	}
}

func TestShouldFilter_SeniorityContainsIntern(t *testing.T) {
	st := newFilterTestStore(t)
	job := makeJob()
	job.Seniority = "intern_summer" // contains "intern"

	filtered, reason := ShouldFilter(job, st)
	if !filtered {
		t.Errorf("expected filtered=true for seniority containing intern, got false")
	}
	if reason != FilterReasonSeniorityExcluded {
		t.Errorf("expected reason %q, got %q", FilterReasonSeniorityExcluded, reason)
	}
}

func TestShouldFilter_SenioritySenior_NotFiltered(t *testing.T) {
	st := newFilterTestStore(t)
	job := makeJob()
	job.Seniority = "senior"

	filtered, _ := ShouldFilter(job, st)
	if filtered {
		t.Errorf("expected filtered=false for seniority=senior, got true")
	}
}

func TestShouldFilter_SeniorityStaff_NotFiltered(t *testing.T) {
	st := newFilterTestStore(t)
	job := makeJob()
	job.Seniority = "staff"

	filtered, _ := ShouldFilter(job, st)
	if filtered {
		t.Errorf("expected filtered=false for seniority=staff, got true")
	}
}

func TestShouldFilter_SeniorityEmpty_NotFiltered(t *testing.T) {
	st := newFilterTestStore(t)
	job := makeJob()
	job.Seniority = ""

	filtered, _ := ShouldFilter(job, st)
	if filtered {
		t.Errorf("expected filtered=false for empty seniority, got true")
	}
}

// --- Integration: passing job is not filtered ---

func TestShouldFilter_PassingJob(t *testing.T) {
	st := newFilterTestStore(t)
	job := makeJob() // All fields pass by default

	filtered, reason := ShouldFilter(job, st)
	if filtered {
		t.Errorf("expected filtered=false for a good job, got true (reason: %q)", reason)
	}
}

// --- SweepResult fields ---

func TestSweepResult_HasFilteredFields(t *testing.T) {
	result := &SweepResult{
		Filtered:      3,
		FilterReasons: map[string]int{"no_llm_tech": 2, "company_too_small": 1},
	}
	if result.Filtered != 3 {
		t.Errorf("Filtered = %d, want 3", result.Filtered)
	}
	if result.FilterReasons["no_llm_tech"] != 2 {
		t.Errorf("FilterReasons[no_llm_tech] = %d, want 2", result.FilterReasons["no_llm_tech"])
	}
}

// --- RunSweep integration: filtered jobs not inserted ---

func TestRunSweep_FilteredJobsNotInserted(t *testing.T) {
	st := newTestSweepStore(t)

	// Two jobs: one with no LLM tech (filtered), one with langchain (passes)
	response := `{
		"data": [
			{"id": 101, "job_title": "React Dev", "company": "WebCo",
			 "discovered_at": "2026-03-20T10:00:00Z",
			 "employment_statuses": ["full_time"],
			 "technology_slugs": ["react", "typescript"],
			 "keyword_slugs": [],
			 "company_object": {"employee_count": 200, "industry": "Software", "domain": "webco.com"}},
			{"id": 102, "job_title": "AI Engineer", "company": "LLMco",
			 "discovered_at": "2026-03-20T11:00:00Z",
			 "employment_statuses": ["full_time"],
			 "technology_slugs": ["langchain", "openai"],
			 "keyword_slugs": [],
			 "company_object": {"employee_count": 150, "industry": "Software", "domain": "llmco.io"}}
		],
		"metadata": {"total_results": 2}
	}`

	transport := &mockTransport{status: 200, response: response}
	client := NewTheirStackClient("key", &http.Client{Transport: transport})
	cfg := DefaultConfig()

	result, err := RunSweep(client, st, cfg, nil)
	if err != nil {
		t.Fatalf("RunSweep: %v", err)
	}

	// Only the LLM job should be inserted
	if result.NewLeads != 1 {
		t.Errorf("NewLeads = %d, want 1 (only LLM job)", result.NewLeads)
	}
	if result.Filtered != 1 {
		t.Errorf("Filtered = %d, want 1 (React job filtered)", result.Filtered)
	}
	if result.FilterReasons[string(FilterReasonNoLLMTech)] != 1 {
		t.Errorf("FilterReasons[no_llm_tech] = %d, want 1", result.FilterReasons[string(FilterReasonNoLLMTech)])
	}
}

func TestRunSweep_SmallCompanyFiltered(t *testing.T) {
	st := newTestSweepStore(t)

	response := `{
		"data": [
			{"id": 201, "job_title": "LLM Engineer", "company": "TinyLLM",
			 "discovered_at": "2026-03-20T10:00:00Z",
			 "employment_statuses": ["full_time"],
			 "technology_slugs": ["langchain"],
			 "keyword_slugs": [],
			 "company_object": {"employee_count": 10, "industry": "Software", "domain": "tinyllm.io"}}
		],
		"metadata": {"total_results": 1}
	}`

	transport := &mockTransport{status: 200, response: response}
	client := NewTheirStackClient("key", &http.Client{Transport: transport})
	cfg := DefaultConfig()

	result, err := RunSweep(client, st, cfg, nil)
	if err != nil {
		t.Fatalf("RunSweep: %v", err)
	}

	if result.NewLeads != 0 {
		t.Errorf("NewLeads = %d, want 0 (small company filtered)", result.NewLeads)
	}
	if result.Filtered != 1 {
		t.Errorf("Filtered = %d, want 1", result.Filtered)
	}
}
