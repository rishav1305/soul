package sweep

import (
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rishav1305/soul-v2/internal/scout/store"
)

func newTestSweepStore(t *testing.T) *store.Store {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { st.Close() })
	return st
}

const twoJobsResponse = `{
	"data": [
		{"id": 1, "job_title": "LLM Engineer", "company": "Acme", "discovered_at": "2026-03-17T10:00:00Z", "employment_statuses": ["full_time"], "technology_slugs": ["langchain", "go"], "keyword_slugs": [], "company_domain": "acme-llm.io", "seniority": "senior", "company_object": {"employee_count": 100, "industry": "Software", "domain": "acme-llm.io"}},
		{"id": 2, "job_title": "AI Engineer", "company": "Beta", "discovered_at": "2026-03-17T11:00:00Z", "employment_statuses": ["contract"], "technology_slugs": ["openai", "react"], "keyword_slugs": [], "company_domain": "beta-ai.io", "seniority": "senior", "company_object": {"employee_count": 200, "industry": "Software", "domain": "beta-ai.io"}}
	],
	"metadata": {"total_results": 2}
}`

const emptyJobsResponse = `{"data": [], "metadata": {"total_results": 0}}`

func TestRunSweep_FetchAndDedup(t *testing.T) {
	st := newTestSweepStore(t)
	transport := &mockTransport{status: 200, response: twoJobsResponse}
	client := NewTheirStackClient("key", &http.Client{Transport: transport})
	cfg := DefaultConfig()
	cfg.Limit = 50
	cfg.CreditBudget = 200

	result, err := RunSweep(client, st, cfg, nil)
	if err != nil {
		t.Fatalf("sweep: %v", err)
	}
	if result.NewLeads != 2 {
		t.Errorf("new_leads = %d, want 2", result.NewLeads)
	}

	// Run again — should dedup
	result2, _ := RunSweep(client, st, cfg, nil)
	if result2.Duplicates != 2 {
		t.Errorf("duplicates = %d, want 2", result2.Duplicates)
	}
	if result2.NewLeads != 0 {
		t.Errorf("new_leads on re-run = %d, want 0", result2.NewLeads)
	}
}

func TestRunSweep_CursorNotAdvancedOnError(t *testing.T) {
	st := newTestSweepStore(t)
	transport := &mockTransport{status: 429, response: `{"error":{"message":"rate limited"}}`}
	client := NewTheirStackClient("key", &http.Client{Transport: transport})
	cfg := DefaultConfig()

	st.SetSyncMeta("theirstack_cursor", "2026-03-17T00:00:00Z")

	RunSweep(client, st, cfg, nil)

	cursor, _ := st.GetSyncMeta("theirstack_cursor")
	if cursor != "2026-03-17T00:00:00Z" {
		t.Errorf("cursor advanced to %q on error — should stay unchanged", cursor)
	}
}

func TestRunSweep_CursorAdvancedOnSuccess(t *testing.T) {
	st := newTestSweepStore(t)
	transport := &mockTransport{status: 200, response: twoJobsResponse}
	client := NewTheirStackClient("key", &http.Client{Transport: transport})
	cfg := DefaultConfig()

	RunSweep(client, st, cfg, nil)

	cursor, _ := st.GetSyncMeta("theirstack_cursor")
	// Max discovered_at is "2026-03-17T11:00:00Z" + 1s = "2026-03-17T11:00:01Z"
	if cursor != "2026-03-17T11:00:01Z" {
		t.Errorf("cursor = %q, want 2026-03-17T11:00:01Z", cursor)
	}
}

func TestJobToLead(t *testing.T) {
	job := Job{
		ID:                 42,
		JobTitle:           "Go Engineer",
		Company:            "Stripe",
		EmploymentStatuses: []string{"full_time"},
		Remote:             true,
		Seniority:          "senior",
		CompanyObject: &CompanyInfo{
			Domain:   "stripe.com",
			Industry: "Fintech",
		},
		HiringTeam: []HiringPerson{
			{FullName: "Jane Doe", LinkedInURL: "https://linkedin.com/in/jane"},
		},
	}
	lead := JobToLead(job)
	if lead.Pipeline != "job" {
		t.Errorf("pipeline = %q, want job", lead.Pipeline)
	}
	if lead.Stage != "discovered" {
		t.Errorf("stage = %q, want discovered", lead.Stage)
	}
	if lead.Source != "theirstack" {
		t.Errorf("source = %q, want theirstack", lead.Source)
	}
	if lead.HiringManager != "Jane Doe" {
		t.Errorf("hiring_manager = %q, want Jane Doe", lead.HiringManager)
	}
	if lead.CompanyIndustry != "Fintech" {
		t.Errorf("company_industry = %q, want Fintech", lead.CompanyIndustry)
	}
}

func TestRunSweep_403AppearsInErrors(t *testing.T) {
	st := newTestSweepStore(t)
	transport := &mockTransport{status: 403, response: `{"error":"forbidden"}`}
	client := NewTheirStackClient("bad-key", &http.Client{Transport: transport})
	cfg := DefaultConfig()

	result, err := RunSweep(client, st, cfg, nil)
	if err != nil {
		t.Fatalf("RunSweep should not return error on 403: %v", err)
	}
	if len(result.Errors) == 0 {
		t.Fatal("expected 403 error in result.Errors, got none")
	}
	if !strings.Contains(result.Errors[0], "403") {
		t.Errorf("expected error to mention 403, got: %q", result.Errors[0])
	}
}

func TestRunSweep_403CursorNotAdvanced(t *testing.T) {
	st := newTestSweepStore(t)
	transport := &mockTransport{status: 403, response: `{"error":"forbidden"}`}
	client := NewTheirStackClient("bad-key", &http.Client{Transport: transport})
	cfg := DefaultConfig()

	st.SetSyncMeta("theirstack_cursor", "2026-03-17T00:00:00Z")
	RunSweep(client, st, cfg, nil)

	cursor, _ := st.GetSyncMeta("theirstack_cursor")
	if cursor != "2026-03-17T00:00:00Z" {
		t.Errorf("cursor advanced to %q on 403 — should stay unchanged", cursor)
	}
}

// Ensure unused imports are satisfied (io, strings used by mockTransport in theirstack_test.go).
var _ = io.NopCloser
var _ = strings.NewReader
