# Scout Setup Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace Scout's stubbed CDP sweep with TheirStack API for job discovery, add 7 Claude-powered AI tools, and implement a cron scheduler with auto-scoring.

**Architecture:** TheirStack HTTP client fetches jobs → leads table redesigned with 45 columns → AI service scores leads via Sender interface → scheduler runs daily sweeps → async model for long-running tools. No new Go dependencies.

**Tech Stack:** Go 1.24, net/http, SQLite (modernc.org/sqlite), pgx/v5, os/exec

**Spec:** `docs/superpowers/specs/2026-03-17-scout-setup-design.md`

---

## File Map

### Store rewrite
| File | Action | Responsibility |
|------|--------|---------------|
| `internal/scout/store/store.go` | Rewrite | Lead struct (45 fields), migration with safety check, scanLead, leadColumns, allowedLeadFields |
| `internal/scout/store/leads.go` | Rewrite | AddLead, GetLead, ListLeads(pipelineFilter), ScoredLeads, AddLeadIfNotExists, RecordStageHistory |
| `internal/scout/store/analytics.go` | Rewrite | All `type` → `pipeline`. AggregateStats adds seniority/remote/country_code groupings |
| `internal/scout/store/store_test.go` | Rewrite | Tests for new Lead struct: CRUD, dedup, scoring, analytics |

### Sweep package
| File | Action | Responsibility |
|------|--------|---------------|
| `internal/scout/sweep/config.go` | Create | SweepConfig struct, LoadConfig, SaveConfig, DefaultConfig |
| `internal/scout/sweep/theirstack.go` | Create | TheirStackClient, Search method, response types, pipeline inference |
| `internal/scout/sweep/sweep.go` | Rewrite | SweepResult, RunSweep (3-phase: fetch → score → finalize) |
| `internal/scout/sweep/scheduler.go` | Create | Scheduler struct, Start/Stop/RunNow, cron goroutine |
| `internal/scout/sweep/cdp.go` | Delete | No longer needed |

### AI package
| File | Action | Responsibility |
|------|--------|---------------|
| `internal/scout/ai/ai.go` | Create | Sender interface, Service struct, shared helpers (fetchLeadAndProfile) |
| `internal/scout/ai/match.go` | Create | ResumeMatch — score resume vs JD |
| `internal/scout/ai/proposal.go` | Create | ProposalGen — platform-aware proposals |
| `internal/scout/ai/cover.go` | Create | CoverLetter — tailored cover letters |
| `internal/scout/ai/outreach.go` | Create | ColdOutreach — personalized email drafts |
| `internal/scout/ai/salary.go` | Create | SalaryLookup — market rate estimates |
| `internal/scout/ai/referral.go` | Create | ReferralFinder — async subprocess |
| `internal/scout/ai/pitch.go` | Create | CompanyPitch — async subprocess |

### Agent launcher
| File | Action | Responsibility |
|------|--------|---------------|
| `internal/scout/agent/launcher.go` | Rewrite | LaunchConfig, LaunchResult, Launch() via exec.Command |

### Server & integration
| File | Action | Responsibility |
|------|--------|---------------|
| `internal/scout/server/server.go` | Rewrite | Add aiService/scheduler fields, 9 new endpoints, CORS PUT, remove CDP |
| `cmd/scout/main.go` | Rewrite | New init order: store → profiledb → stream → ai → sweep config → server → scheduler |
| `internal/chat/context/scout.go` | Modify | Add 7 AI tool definitions |
| `internal/chat/context/dispatch.go` | Modify | Add 7 AI dispatch routes |
| `CLAUDE.md` | Modify | Update tool counts, env vars |

---

## Task 1: Store — Lead Struct & Migration

**Files:** `internal/scout/store/store.go`, `internal/scout/store/store_test.go`

- [ ] **Step 1: Write failing test for new Lead struct and migration**

```go
// In store_test.go — replace existing test helper and add migration test
func newTestStore(t *testing.T) *Store {
	t.Helper()
	dir := t.TempDir()
	st, err := Open(filepath.Join(dir, "test-scout.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { st.Close() })
	return st
}

func TestMigration_NewSchema(t *testing.T) {
	st := newTestStore(t)

	// Verify leads table has new columns
	var count int
	err := st.DB().QueryRow("SELECT COUNT(*) FROM pragma_table_info('leads') WHERE name = 'theirstack_id'").Scan(&count)
	if err != nil {
		t.Fatalf("query pragma: %v", err)
	}
	if count != 1 {
		t.Fatal("theirstack_id column not found in leads table")
	}

	// Verify partial unique index exists
	err = st.DB().QueryRow("SELECT COUNT(*) FROM pragma_index_list('leads') WHERE name = 'idx_leads_theirstack_id'").Scan(&count)
	if err != nil {
		t.Fatalf("query index: %v", err)
	}
	if count != 1 {
		t.Fatal("idx_leads_theirstack_id index not found")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/scout/store/ -run TestMigration_NewSchema -v`
Expected: FAIL — old schema lacks `theirstack_id` column

- [ ] **Step 3: Rewrite store.go with new Lead struct and migration**

Rewrite `internal/scout/store/store.go`:
- Replace `Lead` struct with 45 fields matching spec §1 schema. JSON tags use camelCase. Fields: ID, Source, Pipeline, Stage, MatchScore, NextAction, NextDate, Notes, CreatedAt, UpdatedAt, ClosedAt, TheirStackID, JobTitle, URL, FinalURL, SourceURL, DatePosted, DiscoveredAt, Description, NormalizedTitle, Location, ShortLocation, Country, CountryCode, Remote, Hybrid, SalaryString, MinAnnualSalaryUSD, MaxAnnualSalaryUSD, SalaryCurrency, Seniority, EmploymentStatuses, EasyApply, TechnologySlugs, KeywordSlugs, Company, CompanyDomain, CompanyIndustry, CompanyEmployeeCount, CompanyLinkedInURL, CompanyTotalFundingUSD, CompanyFundingStage, CompanyLogo, CompanyCountry, HiringManager, HiringManagerLinkedIn, Metadata.
- Remove old `Type`, `Title` → `JobTitle`, `Company` stays, `SourceURL` stays, etc.
- `TheirStackID` is `*int64` (nullable for manual leads).
- `Remote`, `Hybrid`, `EasyApply` are `bool` in Go (INTEGER in SQLite).
- `migrate()` checks row count before DROP TABLE (spec §1 migration strategy). Then creates new schema with all indexes.
- Update `scanLead`, `leadColumns`, `allowedLeadFields` for new columns.
- Keep all other tables (stage_history, sync_results, sync_meta, optimizations, agent_runs, platform_trust) unchanged.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/scout/store/ -run TestMigration_NewSchema -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/scout/store/store.go internal/scout/store/store_test.go
git commit -m "feat: rewrite scout leads table schema with 45 columns for TheirStack"
```

---

## Task 2: Store — Leads CRUD

**Files:** `internal/scout/store/leads.go`, `internal/scout/store/store_test.go`

- [ ] **Step 1: Write failing tests for AddLead and GetLead**

```go
func makeTestLead() Lead {
	return Lead{
		Source:      "theirstack",
		Pipeline:    "job",
		Stage:       "discovered",
		JobTitle:    "Senior Go Engineer",
		Company:     "Stripe",
		Location:    "Remote",
		CountryCode: "US",
		Remote:      true,
		Seniority:   "senior",
		SalaryString: "$180k-220k",
		MinAnnualSalaryUSD: 180000,
		MaxAnnualSalaryUSD: 220000,
		SalaryCurrency:     "USD",
		EmploymentStatuses: `["full_time"]`,
		TechnologySlugs:    `["go","react"]`,
		Description:        "Build distributed systems...",
		CompanyDomain:      "stripe.com",
	}
}

func TestAddLead(t *testing.T) {
	st := newTestStore(t)
	lead := makeTestLead()
	id, err := st.AddLead(lead)
	if err != nil {
		t.Fatalf("add lead: %v", err)
	}
	if id < 1 {
		t.Fatal("expected positive ID")
	}
}

func TestGetLead(t *testing.T) {
	st := newTestStore(t)
	lead := makeTestLead()
	id, _ := st.AddLead(lead)
	got, err := st.GetLead(id)
	if err != nil {
		t.Fatalf("get lead: %v", err)
	}
	if got.JobTitle != "Senior Go Engineer" {
		t.Errorf("title = %q, want Senior Go Engineer", got.JobTitle)
	}
	if !got.Remote {
		t.Error("expected remote = true")
	}
	if got.Pipeline != "job" {
		t.Errorf("pipeline = %q, want job", got.Pipeline)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/scout/store/ -run "TestAddLead|TestGetLead" -v`
Expected: FAIL — AddLead/GetLead not updated

- [ ] **Step 3: Write failing tests for ListLeads, ScoredLeads, AddLeadIfNotExists**

```go
func TestListLeads_PipelineFilter(t *testing.T) {
	st := newTestStore(t)
	job := makeTestLead()
	job.Pipeline = "job"
	st.AddLead(job)

	contract := makeTestLead()
	contract.Pipeline = "contract"
	contract.JobTitle = "Contract Dev"
	st.AddLead(contract)

	leads, err := st.ListLeads("job", false)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(leads) != 1 {
		t.Fatalf("got %d leads, want 1", len(leads))
	}
	if leads[0].Pipeline != "job" {
		t.Errorf("pipeline = %q, want job", leads[0].Pipeline)
	}
}

func TestScoredLeads(t *testing.T) {
	st := newTestStore(t)
	low := makeTestLead()
	low.MatchScore = 30
	low.JobTitle = "Low Match"
	st.AddLead(low)

	high := makeTestLead()
	high.MatchScore = 90
	high.JobTitle = "High Match"
	st.AddLead(high)

	leads, err := st.ScoredLeads(2)
	if err != nil {
		t.Fatalf("scored: %v", err)
	}
	if leads[0].MatchScore < leads[1].MatchScore {
		t.Error("expected descending match_score order")
	}
}

func TestAddLeadIfNotExists_Dedup(t *testing.T) {
	st := newTestStore(t)
	tsID := int64(12345)
	lead := makeTestLead()
	lead.TheirStackID = &tsID

	id1, created1, err := st.AddLeadIfNotExists(lead)
	if err != nil {
		t.Fatalf("first insert: %v", err)
	}
	if !created1 {
		t.Error("expected created=true on first insert")
	}

	id2, created2, err := st.AddLeadIfNotExists(lead)
	if err != nil {
		t.Fatalf("second insert: %v", err)
	}
	if created2 {
		t.Error("expected created=false on duplicate")
	}
	if id2 != id1 {
		t.Errorf("expected same ID, got %d vs %d", id2, id1)
	}
}
```

- [ ] **Step 4: Rewrite leads.go**

Rewrite `internal/scout/store/leads.go`:
- `AddLead(lead Lead) (int64, error)` — INSERT with all new columns. Sets `created_at` and `updated_at` to `now()`.
- `GetLead(id int64) (*Lead, error)` — SELECT using updated `leadColumns`.
- `ListLeads(pipelineFilter string, activeOnly bool) ([]Lead, error)` — filter on `pipeline` (not `type`). `activeOnly` excludes `closed_at != ''`.
- `ScoredLeads(limit int) ([]Lead, error)` — ORDER BY match_score DESC.
- `AddLeadIfNotExists(lead Lead) (id int64, created bool, err error)` — INSERT OR IGNORE on theirstack_id. Returns existing ID + created=false if duplicate. Uses `SELECT id FROM leads WHERE theirstack_id = ?` after INSERT OR IGNORE to get the ID.
- Keep `RecordStageHistory` and `GetStageHistory` unchanged (they reference lead_id FK which still works).

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test ./internal/scout/store/ -run "TestAddLead|TestGetLead|TestListLeads|TestScored|TestAddLeadIfNotExists" -v`
Expected: ALL PASS

- [ ] **Step 6: Commit**

```bash
git add internal/scout/store/leads.go internal/scout/store/store_test.go
git commit -m "feat: rewrite scout leads CRUD for new 45-column schema"
```

---

## Task 3: Store — Analytics Rewrite

**Files:** `internal/scout/store/analytics.go`, `internal/scout/store/store_test.go`

- [ ] **Step 1: Write failing test for updated analytics**

```go
func TestGetAnalytics_Pipeline(t *testing.T) {
	st := newTestStore(t)

	job := makeTestLead()
	job.Pipeline = "job"
	job.Stage = "applied"
	st.AddLead(job)

	freelance := makeTestLead()
	freelance.Pipeline = "freelance"
	freelance.Stage = "found"
	freelance.JobTitle = "Freelance Gig"
	st.AddLead(freelance)

	analytics, err := st.GetAnalytics("")
	if err != nil {
		t.Fatalf("analytics: %v", err)
	}

	// Should group by pipeline (not type)
	if analytics.Stats.ByPipeline == nil {
		t.Fatal("ByPipeline is nil")
	}
	if analytics.Stats.ByPipeline["job"] != 1 {
		t.Errorf("ByPipeline[job] = %d, want 1", analytics.Stats.ByPipeline["job"])
	}
	if analytics.Stats.ByPipeline["freelance"] != 1 {
		t.Errorf("ByPipeline[freelance] = %d, want 1", analytics.Stats.ByPipeline["freelance"])
	}

	// Filter by pipeline
	filtered, err := st.GetAnalytics("job")
	if err != nil {
		t.Fatalf("filtered analytics: %v", err)
	}
	if filtered.Stats.Active != 1 {
		t.Errorf("active = %d, want 1", filtered.Stats.Active)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/scout/store/ -run TestGetAnalytics_Pipeline -v`
Expected: FAIL — `ByPipeline` field doesn't exist

- [ ] **Step 3: Rewrite analytics.go**

- Rename `AggregateStats.ByType` → `ByPipeline`. Add `BySeniority map[string]int`, `ByCountry map[string]int`, `RemoteCount int`.
- All SQL queries: change `type` column references to `pipeline`.
- `GetAnalytics(pipelineFilter string)` — parameter was `typeFilter`, now `pipelineFilter`.
- `getAggregateStats`: GROUP BY pipeline, seniority, country_code. Count remote=1.
- `getConversionMetrics`: `SELECT DISTINCT pipeline FROM leads` (was `type`).
- `TypeFunnel` → rename to `PipelineFunnel`, field `Type` → `Pipeline`.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/scout/store/ -run TestGetAnalytics -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/scout/store/analytics.go internal/scout/store/store_test.go
git commit -m "feat: rewrite scout analytics to use pipeline instead of type"
```

---

## Task 4: Sweep — Config

**Files:** `internal/scout/sweep/config.go`, `internal/scout/sweep/config_test.go`

- [ ] **Step 1: Write failing test**

```go
package sweep

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfig_Default(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sweep-config.json")

	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	if len(cfg.JobTitleOr) == 0 {
		t.Fatal("expected default job titles")
	}
	if cfg.CreditBudget != 50 {
		t.Errorf("credit_budget = %d, want 50", cfg.CreditBudget)
	}
	if cfg.IntervalHours != 24 {
		t.Errorf("interval_hours = %d, want 24", cfg.IntervalHours)
	}

	// File should have been created
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Fatal("config file not created")
	}
}

func TestLoadConfig_Existing(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sweep-config.json")
	os.WriteFile(path, []byte(`{"job_title_or":["devops"],"credit_budget":10}`), 0644)

	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(cfg.JobTitleOr) != 1 || cfg.JobTitleOr[0] != "devops" {
		t.Errorf("job_title_or = %v, want [devops]", cfg.JobTitleOr)
	}
	if cfg.CreditBudget != 10 {
		t.Errorf("credit_budget = %d, want 10", cfg.CreditBudget)
	}
}

func TestSaveConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sweep-config.json")

	cfg := DefaultConfig()
	cfg.CreditBudget = 99
	if err := SaveConfig(path, cfg); err != nil {
		t.Fatalf("save: %v", err)
	}

	loaded, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if loaded.CreditBudget != 99 {
		t.Errorf("credit_budget = %d, want 99", loaded.CreditBudget)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/scout/sweep/ -run TestLoadConfig -v`
Expected: FAIL

- [ ] **Step 3: Implement config.go**

```go
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
	return &cfg, nil
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
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/scout/sweep/ -run "TestLoadConfig|TestSaveConfig" -v`
Expected: ALL PASS

- [ ] **Step 5: Commit**

```bash
git add internal/scout/sweep/config.go internal/scout/sweep/config_test.go
git commit -m "feat: add sweep config with load/save and defaults"
```

---

## Task 5: Sweep — TheirStack Client

**Files:** `internal/scout/sweep/theirstack.go`, `internal/scout/sweep/theirstack_test.go`

- [ ] **Step 1: Write failing test with mock HTTP transport**

```go
package sweep

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

type mockTransport struct {
	response string
	status   int
	lastBody string
}

func (m *mockTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	body, _ := io.ReadAll(req.Body)
	m.lastBody = string(body)
	return &http.Response{
		StatusCode: m.status,
		Body:       io.NopCloser(strings.NewReader(m.response)),
		Header:     make(http.Header),
	}, nil
}

func TestTheirStackClient_Search(t *testing.T) {
	transport := &mockTransport{
		status: 200,
		response: `{
			"data": [
				{
					"id": 12345,
					"job_title": "Senior Go Engineer",
					"company": "Stripe",
					"url": "https://jobs.lever.co/stripe/123",
					"final_url": "https://stripe.com/jobs/123",
					"source_url": "https://linkedin.com/jobs/123",
					"location": "San Francisco, CA",
					"remote": true,
					"seniority": "senior",
					"salary_string": "$180k-220k",
					"min_annual_salary_usd": 180000,
					"max_annual_salary_usd": 220000,
					"employment_statuses": ["full_time"],
					"technology_slugs": ["go", "react"],
					"description": "Build distributed systems",
					"discovered_at": "2026-03-17T10:00:00Z",
					"company_object": {
						"domain": "stripe.com",
						"industry": "Financial Services",
						"employee_count": 8000,
						"linkedin_url": "https://linkedin.com/company/stripe"
					}
				}
			],
			"metadata": {"total_results": 1}
		}`,
	}

	client := NewTheirStackClient("test-key", &http.Client{Transport: transport})
	cfg := DefaultConfig()
	result, err := client.Search(cfg, "", 0)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(result.Jobs) != 1 {
		t.Fatalf("got %d jobs, want 1", len(result.Jobs))
	}
	job := result.Jobs[0]
	if job.ID != 12345 {
		t.Errorf("id = %d, want 12345", job.ID)
	}
	if job.JobTitle != "Senior Go Engineer" {
		t.Errorf("title = %q", job.JobTitle)
	}
	if !job.Remote {
		t.Error("expected remote=true")
	}

	// Verify auth header was sent
	if !strings.Contains(transport.lastBody, "job_title_or") {
		t.Error("expected request body to contain job_title_or filter")
	}
}

func TestInferPipeline(t *testing.T) {
	tests := []struct {
		statuses []string
		want     string
	}{
		{[]string{"full_time"}, "job"},
		{[]string{"contract"}, "contract"},
		{[]string{"part_time"}, "freelance"},
		{[]string{"temporary"}, "freelance"},
		{[]string{"internship"}, "job"},
		{[]string{"full_time", "contract"}, "contract"},
		{[]string{}, "job"},
		{[]string{"unknown"}, "job"},
		{nil, "job"},
	}
	for _, tt := range tests {
		got := InferPipeline(tt.statuses)
		if got != tt.want {
			t.Errorf("InferPipeline(%v) = %q, want %q", tt.statuses, got, tt.want)
		}
	}
}

func TestTheirStackClient_Search_RateLimit(t *testing.T) {
	transport := &mockTransport{
		status:   429,
		response: `{"error":{"code":"rate_limit","message":"too many requests"}}`,
	}
	client := NewTheirStackClient("test-key", &http.Client{Transport: transport})
	_, err := client.Search(DefaultConfig(), "", 0)
	if err == nil {
		t.Fatal("expected error on 429")
	}
	if !IsRateLimitError(err) {
		t.Errorf("expected rate limit error, got: %v", err)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/scout/sweep/ -run "TestTheirStack|TestInferPipeline" -v`
Expected: FAIL

- [ ] **Step 3: Implement theirstack.go**

Create `internal/scout/sweep/theirstack.go`:
- `TheirStackClient` struct with `apiKey string` and `httpClient *http.Client`.
- `NewTheirStackClient(apiKey string, httpClient *http.Client) *TheirStackClient`.
- TheirStack response types: `SearchResponse{Data []Job, Metadata ResponseMetadata}`, `Job` struct with all fields from spec, nested `CompanyInfo` struct.
- `Search(cfg *SweepConfig, cursor string, offset int) (*SearchResponse, error)` — builds request body from config + cursor + offset, POSTs to `https://api.theirstack.com/v1/jobs/search`, returns parsed response. Error types: `RateLimitError` (429), `CreditsExhaustedError` (402), `ServerError` (5xx).
- `InferPipeline(statuses []string) string` — deterministic rules per spec: contract > full_time > part_time/temporary > internship > fallback "job".
- `IsRateLimitError(err)`, `IsCreditsExhaustedError(err)` helpers.

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/scout/sweep/ -run "TestTheirStack|TestInferPipeline" -v`
Expected: ALL PASS

- [ ] **Step 5: Commit**

```bash
git add internal/scout/sweep/theirstack.go internal/scout/sweep/theirstack_test.go
git commit -m "feat: add TheirStack API client with pipeline inference"
```

---

## Task 6: Sweep — Orchestrator

**Files:** `internal/scout/sweep/sweep.go`

- [ ] **Step 1: Rewrite sweep.go with new SweepResult and RunSweep**

Replace the current stub with:

```go
package sweep

import (
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/rishav1305/soul-v2/internal/scout/store"
)

// SweepResult holds the outcome of a sweep run.
type SweepResult struct {
	NewLeads    int      `json:"newLeads"`
	Duplicates  int      `json:"duplicates"`
	Scored      int      `json:"scored"`
	HighMatches int      `json:"highMatches"`
	Errors      []string `json:"errors"`
}

// Scorer scores a lead by ID. Implemented by ai.Service.ResumeMatch.
type Scorer interface {
	ScoreLead(leadID int64) (float64, error)
}

// RunSweep executes a 3-phase sweep: fetch → score → finalize.
func RunSweep(client *TheirStackClient, st *store.Store, cfg *SweepConfig, scorer Scorer) (*SweepResult, error) {
	result := &SweepResult{}

	// Load cursor (empty string on first run is correct — fetches all)
	cursor, err := st.GetSyncMeta("theirstack_cursor")
	if err != nil {
		log.Printf("scout: load cursor: %v", err)
	}

	// Phase 1: Fetch all pages
	var newLeadIDs []int64
	var maxDiscoveredAt string
	creditsUsed := 0
	offset := 0
	hadError := false

	for {
		resp, err := client.Search(cfg, cursor, offset)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("page offset=%d: %v", offset, err))
			hadError = true
			break
		}

		for _, job := range resp.Jobs {
			// Track max discovered_at for cursor advancement
			if job.DiscoveredAt > maxDiscoveredAt {
				maxDiscoveredAt = job.DiscoveredAt
			}

			lead := jobToLead(job)
			id, created, err := st.AddLeadIfNotExists(lead)
			if err != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("insert job %d: %v", job.ID, err))
				continue
			}
			if created {
				result.NewLeads++
				newLeadIDs = append(newLeadIDs, id)
			} else {
				result.Duplicates++
			}
		}

		creditsUsed += len(resp.Jobs)
		if len(resp.Jobs) < cfg.Limit || creditsUsed >= cfg.CreditBudget {
			break
		}
		offset += cfg.Limit
	}

	// Phase 2: Auto-score
	if scorer != nil {
		for _, leadID := range newLeadIDs {
			score, err := scorer.ScoreLead(leadID)
			if err != nil {
				log.Printf("scout: score lead %d: %v", leadID, err)
				continue
			}
			result.Scored++
			if score >= cfg.AutoScoreThreshold {
				result.HighMatches++
			}
		}
	}

	// Phase 3: Finalize
	if !hadError && maxDiscoveredAt != "" {
		// Advance cursor: max(discovered_at) + 1 second to avoid boundary re-fetch
		t, err := time.Parse(time.RFC3339, maxDiscoveredAt)
		if err == nil {
			newCursor := t.Add(1 * time.Second).Format(time.RFC3339)
			st.SetSyncMeta("theirstack_cursor", newCursor)
		}
	}
	st.SetSyncMeta("sweep_last_run", time.Now().UTC().Format(time.RFC3339))

	// Build and store digest
	digest := buildDigest(result, st, cfg)
	digestBytes, _ := json.Marshal(digest)
	st.SetSyncMeta("sweep_last_digest", string(digestBytes))

	return result, nil
}

// buildDigest constructs the sweep digest from results and DB state.
func buildDigest(result *SweepResult, st *store.Store, cfg *SweepConfig) map[string]interface{} {
	lastRun, _ := st.GetSyncMeta("sweep_last_run")
	highLeads, _ := st.ScoredLeads(10) // top 10 by match_score
	var highMatchLeads []map[string]interface{}
	for _, l := range highLeads {
		if l.MatchScore >= cfg.AutoScoreThreshold {
			highMatchLeads = append(highMatchLeads, map[string]interface{}{
				"id": l.ID, "job_title": l.JobTitle, "company": l.Company,
				"match_score": l.MatchScore, "salary_string": l.SalaryString,
			})
		}
	}
	return map[string]interface{}{
		"last_run":         lastRun,
		"new_leads":        result.NewLeads,
		"duplicates":       result.Duplicates,
		"high_matches":     result.HighMatches,
		"high_match_leads": highMatchLeads,
	}
}

// jobToLead converts a TheirStack Job to a store.Lead.
func jobToLead(job Job) store.Lead {
	// Map all fields from Job to Lead struct
	// InferPipeline from employment_statuses
	// Set stage="discovered", source="theirstack"
	// Build metadata JSON from remaining fields
	// ... (full implementation)
}
```

- [ ] **Step 2: Write sweep tests**

Add `internal/scout/sweep/sweep_test.go`:

```go
func TestRunSweep_FetchAndDedup(t *testing.T) {
	st := newTestSweepStore(t) // helper that opens a test scout.db
	transport := &mockTransport{
		status: 200,
		response: twoJobsResponse, // mock JSON with 2 jobs
	}
	client := NewTheirStackClient("key", &http.Client{Transport: transport})
	cfg := DefaultConfig()
	cfg.Limit = 50
	cfg.CreditBudget = 200

	result, err := RunSweep(client, st, cfg, nil) // nil scorer
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
}

func TestRunSweep_CursorNotAdvancedOnError(t *testing.T) {
	st := newTestSweepStore(t)
	transport := &mockTransport{status: 429, response: `{"error":{"message":"rate limited"}}`}
	client := NewTheirStackClient("key", &http.Client{Transport: transport})
	cfg := DefaultConfig()

	// Set initial cursor
	st.SetSyncMeta("theirstack_cursor", "2026-03-17T00:00:00Z")

	RunSweep(client, st, cfg, nil)

	// Cursor should NOT have advanced
	cursor, _ := st.GetSyncMeta("theirstack_cursor")
	if cursor != "2026-03-17T00:00:00Z" {
		t.Errorf("cursor advanced to %q on error — should stay unchanged", cursor)
	}
}
```

- [ ] **Step 3: Delete cdp.go**

```bash
rm internal/scout/sweep/cdp.go
```

- [ ] **Step 4: Run tests**

Run: `go test ./internal/scout/sweep/ -run TestRunSweep -v`
Expected: ALL PASS

- [ ] **Step 5: Commit**

```bash
git add internal/scout/sweep/sweep.go internal/scout/sweep/sweep_test.go
git rm internal/scout/sweep/cdp.go
git commit -m "feat: rewrite sweep orchestrator with TheirStack 3-phase flow, delete CDP"
```

---

## Task 7: Sweep — Scheduler

**Files:** `internal/scout/sweep/scheduler.go`

- [ ] **Step 1: Implement scheduler.go**

```go
package sweep

import (
	"log"
	"sync"
	"time"

	"github.com/rishav1305/soul-v2/internal/scout/store"
)

type Scheduler struct {
	interval time.Duration
	config   *SweepConfig
	store    *store.Store
	scorer   Scorer
	client   *TheirStackClient
	stopCh   chan struct{}
	mu       sync.Mutex
	running    bool
	lastRun    time.Time
	lastResult *SweepResult
	runCounter int64
}

func NewScheduler(cfg *SweepConfig, st *store.Store, scorer Scorer, client *TheirStackClient) *Scheduler {
	return &Scheduler{
		interval: time.Duration(cfg.IntervalHours) * time.Hour,
		config:   cfg,
		store:    st,
		scorer:   scorer,
		client:   client,
		stopCh:   make(chan struct{}),
	}
}

func (s *Scheduler) Start() {
	go s.loop()
}

func (s *Scheduler) Stop() {
	close(s.stopCh)
}

func (s *Scheduler) loop() {
	// Run immediately on start
	s.runSweep()

	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.runSweep()
		case <-s.stopCh:
			return
		}
	}
}

func (s *Scheduler) runSweep() {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return
	}
	s.running = true
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		s.running = false
		s.mu.Unlock()
	}()

	log.Println("scout: starting sweep...")
	result, err := RunSweep(s.client, s.store, s.config, s.scorer)
	if err != nil {
		log.Printf("scout: sweep error: %v", err)
		return
	}

	s.mu.Lock()
	s.lastRun = time.Now()
	s.lastResult = result
	s.mu.Unlock()

	log.Printf("scout: sweep complete — %d new, %d dupes, %d scored, %d high matches",
		result.NewLeads, result.Duplicates, result.Scored, result.HighMatches)
}

// RunNow triggers an immediate sweep in a background goroutine.
// Returns a sweep run counter and false if a sweep is already in progress.
func (s *Scheduler) RunNow() (runID int64, started bool) {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return 0, false
	}
	s.runCounter++
	id := s.runCounter
	s.mu.Unlock()

	go s.runSweep()
	return id, true
}

// Status returns the current scheduler state.
func (s *Scheduler) Status() map[string]interface{} {
	s.mu.Lock()
	defer s.mu.Unlock()
	status := map[string]interface{}{
		"running":  s.running,
		"interval": s.interval.String(),
		"last_run": "",
		"next_run": "",
	}
	if !s.lastRun.IsZero() {
		status["last_run"] = s.lastRun.Format(time.RFC3339)
		status["next_run"] = s.lastRun.Add(s.interval).Format(time.RFC3339)
	}
	return status
}
```

- [ ] **Step 2: Add scheduler tests**

Add to `internal/scout/sweep/scheduler_test.go`:

```go
func TestScheduler_RunNowRejectsWhileRunning(t *testing.T) {
	st := newTestSweepStore(t)
	transport := &mockTransport{status: 200, response: emptyJobsResponse}
	client := NewTheirStackClient("key", &http.Client{Transport: transport})
	cfg := DefaultConfig()
	cfg.IntervalHours = 999 // don't auto-trigger

	sched := NewScheduler(cfg, st, nil, client)

	// Simulate running state
	sched.mu.Lock()
	sched.running = true
	sched.mu.Unlock()

	_, started := sched.RunNow()
	if started {
		t.Error("RunNow should return false while already running")
	}
}

func TestScheduler_StatusEmptyBeforeFirstRun(t *testing.T) {
	st := newTestSweepStore(t)
	transport := &mockTransport{status: 200, response: emptyJobsResponse}
	client := NewTheirStackClient("key", &http.Client{Transport: transport})
	cfg := DefaultConfig()

	sched := NewScheduler(cfg, st, nil, client)
	status := sched.Status()

	if status["last_run"] != "" {
		t.Errorf("last_run = %q, want empty", status["last_run"])
	}
	if status["next_run"] != "" {
		t.Errorf("next_run = %q, want empty", status["next_run"])
	}
}
```

- [ ] **Step 3: Run tests**

Run: `go test ./internal/scout/sweep/ -run TestScheduler -v`
Expected: ALL PASS

- [ ] **Step 4: Commit**

```bash
git add internal/scout/sweep/scheduler.go internal/scout/sweep/scheduler_test.go
git commit -m "feat: add sweep scheduler with cron goroutine and RunNow"
```

---

## Task 8: AI — Service & ResumeMatch

**Files:** `internal/scout/ai/ai.go`, `internal/scout/ai/match.go`, `internal/scout/ai/ai_test.go`

- [ ] **Step 1: Create ai.go with Sender interface and Service struct**

```go
package ai

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/rishav1305/soul-v2/internal/chat/stream"
	"github.com/rishav1305/soul-v2/internal/scout/profiledb"
	"github.com/rishav1305/soul-v2/internal/scout/store"
)

// Sender sends a non-streaming request to Claude and returns the response.
type Sender interface {
	Send(ctx context.Context, req *stream.Request) (*stream.Response, error)
}

// Service provides AI-powered tools for scout leads.
type Service struct {
	store     *store.Store
	profileDB *profiledb.Client
	sender    Sender
}

// New creates a new AI service. profileDB may be nil.
func New(st *store.Store, pdb *profiledb.Client, sender Sender) *Service {
	return &Service{store: st, profileDB: pdb, sender: sender}
}

// fetchProfile returns the full profile or an error if profiledb is nil.
func (s *Service) fetchProfile() (map[string]interface{}, error) {
	if s.profileDB == nil {
		return nil, fmt.Errorf("profiledb not configured — set SOUL_SCOUT_PG_URL")
	}
	return s.profileDB.GetFullProfile()
}

// sendAndExtractText sends a request and returns the text content.
func (s *Service) sendAndExtractText(ctx context.Context, system string, userMsg string) (string, error) {
	req := &stream.Request{
		Model:     "claude-sonnet-4-6",
		MaxTokens: 4096,
		System:    system,
		Messages: []stream.Message{
			{Role: "user", Content: []stream.ContentBlock{{Type: "text", Text: userMsg}}},
		},
	}
	resp, err := s.sender.Send(ctx, req)
	if err != nil {
		return "", err
	}
	for _, block := range resp.Content {
		if block.Type == "text" {
			return block.Text, nil
		}
	}
	return "", fmt.Errorf("no text content in response")
}
```

- [ ] **Step 2: Write failing test for ResumeMatch**

```go
package ai

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/rishav1305/soul-v2/internal/chat/stream"
	"github.com/rishav1305/soul-v2/internal/scout/store"
)

type mockSender struct {
	response string
}

func (m *mockSender) Send(ctx context.Context, req *stream.Request) (*stream.Response, error) {
	return &stream.Response{
		Content: []stream.ContentBlock{{Type: "text", Text: m.response}},
	}, nil
}

func newTestStore(t *testing.T) *store.Store {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { st.Close() })
	return st
}

func TestResumeMatch(t *testing.T) {
	st := newTestStore(t)

	lead := store.Lead{
		Source:      "theirstack",
		Pipeline:    "job",
		Stage:       "discovered",
		JobTitle:    "Senior Go Engineer",
		Description: "Build distributed systems in Go",
		Seniority:   "senior",
	}
	id, _ := st.AddLead(lead)

	sender := &mockSender{
		response: `{"score": 85, "strengths": ["Go expertise"], "gaps": ["AWS"], "suggestions": ["Add AWS certs"]}`,
	}

	svc := New(st, nil, sender) // nil profileDB — test without profile
	result, err := svc.ResumeMatch(context.Background(), id)
	if err != nil {
		t.Fatalf("resume match: %v", err)
	}
	if result.Score != 85 {
		t.Errorf("score = %d, want 85", result.Score)
	}

	// Verify match_score updated in DB
	got, _ := st.GetLead(id)
	if got.MatchScore != 85 {
		t.Errorf("db match_score = %f, want 85", got.MatchScore)
	}
}
```

- [ ] **Step 3: Run test to verify it fails**

Run: `go test ./internal/scout/ai/ -run TestResumeMatch -v`
Expected: FAIL

- [ ] **Step 4: Implement match.go**

```go
package ai

import (
	"context"
	"encoding/json"
	"fmt"
)

type MatchResult struct {
	Score       int      `json:"score"`
	Strengths   []string `json:"strengths"`
	Gaps        []string `json:"gaps"`
	Suggestions []string `json:"suggestions"`
}

func (s *Service) ResumeMatch(ctx context.Context, leadID int64) (*MatchResult, error) {
	lead, err := s.store.GetLead(leadID)
	if err != nil {
		return nil, fmt.Errorf("get lead: %w", err)
	}

	// Build context — profile is optional for scoring
	var profileCtx string
	if s.profileDB != nil {
		profile, err := s.fetchProfile()
		if err == nil {
			pJSON, _ := json.Marshal(profile)
			profileCtx = fmt.Sprintf("\n\nCandidate Profile:\n%s", string(pJSON))
		}
	}

	system := "You are a resume-to-JD matching expert. Score the candidate against the job description. Return ONLY valid JSON: {\"score\": 0-100, \"strengths\": [...], \"gaps\": [...], \"suggestions\": [...]}"

	userMsg := fmt.Sprintf("Job Title: %s\nSeniority: %s\nLocation: %s\nTechnologies: %s\n\nJob Description:\n%s%s",
		lead.JobTitle, lead.Seniority, lead.Location, lead.TechnologySlugs,
		lead.Description, profileCtx)

	text, err := s.sendAndExtractText(ctx, system, userMsg)
	if err != nil {
		return nil, err
	}

	var result MatchResult
	if err := json.Unmarshal([]byte(text), &result); err != nil {
		return nil, fmt.Errorf("parse match result: %w", err)
	}

	// Update match_score in DB
	s.store.UpdateLead(leadID, map[string]interface{}{"match_score": float64(result.Score)})

	return &result, nil
}

// ScoreLead implements sweep.Scorer interface.
func (s *Service) ScoreLead(leadID int64) (float64, error) {
	result, err := s.ResumeMatch(context.Background(), leadID)
	if err != nil {
		return 0, err
	}
	return float64(result.Score), nil
}
```

- [ ] **Step 5: Run test to verify it passes**

Run: `go test ./internal/scout/ai/ -run TestResumeMatch -v`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/scout/ai/ai.go internal/scout/ai/match.go internal/scout/ai/ai_test.go
git commit -m "feat: add scout AI service with Sender interface and resume_match"
```

---

## Task 9: AI — Tier 1-2 Tools

**Files:** `internal/scout/ai/proposal.go`, `internal/scout/ai/cover.go`, `internal/scout/ai/outreach.go`, `internal/scout/ai/salary.go`

- [ ] **Step 1: Implement proposal.go**

`ProposalGen(ctx, leadID, platform)` — validates platform (`"upwork"`, `"freelancer"`, `"general"`), fetches lead + profile, platform-specific system prompt, returns proposal text.

- [ ] **Step 2: Implement cover.go**

`CoverLetter(ctx, leadID)` — fetches lead + profile (required — returns error if profileDB nil), generates tailored cover letter.

- [ ] **Step 3: Implement outreach.go**

`ColdOutreach(ctx, leadID)` — fetches lead only (no profileDB needed), uses company columns (industry, employee_count, funding) to draft personalized email.

- [ ] **Step 4: Implement salary.go**

`SalaryLookup(ctx, leadID)` — fetches lead only, returns `SalaryResult{Min, Median, Max, Currency, Reasoning, Sources}`.

- [ ] **Step 5: Add tests for each tool**

Add to `ai_test.go`:
- `TestProposalGen` — mock sender, verify platform validation (400 on invalid), verify output.
- `TestCoverLetter_NoProfileDB` — verify error when profileDB is nil.
- `TestColdOutreach` — works without profileDB.
- `TestSalaryLookup` — verify JSON parse.

- [ ] **Step 6: Run tests**

Run: `go test ./internal/scout/ai/ -v`
Expected: ALL PASS

- [ ] **Step 7: Commit**

```bash
git add internal/scout/ai/proposal.go internal/scout/ai/cover.go internal/scout/ai/outreach.go internal/scout/ai/salary.go internal/scout/ai/ai_test.go
git commit -m "feat: add scout AI tools — proposal_gen, cover_letter, cold_outreach, salary_lookup"
```

---

## Task 10: Agent Launcher

**Files:** `internal/scout/agent/launcher.go`

- [ ] **Step 1: Rewrite launcher.go**

Replace the stub with actual subprocess launcher:

```go
package agent

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/rishav1305/soul-v2/internal/scout/store"
)

type LaunchConfig struct {
	Mode    string
	LeadID  int64
	Prompt  string
	DataDir string // ~/.soul-v2/scout/agent-runs/
}

type LaunchResult struct {
	RunID      int64
	Output     string
	TokensUsed int
	Duration   time.Duration
	Error      string
}

// Launch spawns a Claude CLI subprocess and tracks the run in agent_runs.
func Launch(ctx context.Context, st *store.Store, cfg LaunchConfig) (*LaunchResult, error) {
	// Create agent_runs record
	run := store.AgentRun{
		Platform:  "claude",
		Mode:      cfg.Mode,
		LeadID:    cfg.LeadID,
		Status:    "running",
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
	}
	runID, err := st.AddAgentRun(run)
	if err != nil {
		return nil, fmt.Errorf("create agent run: %w", err)
	}

	// 120s timeout
	ctx, cancel := context.WithTimeout(ctx, 120*time.Second)
	defer cancel()

	start := time.Now()

	// SAFETY: exec.Command directly, never shell
	cmd := exec.CommandContext(ctx, "claude", "--print", "--model", "claude-sonnet-4-6", "--max-turns", "5")
	cmd.Stdin = strings.NewReader(cfg.Prompt)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err = cmd.Run()
	duration := time.Since(start)

	result := &LaunchResult{
		RunID:    runID,
		Duration: duration,
	}

	if ctx.Err() == context.DeadlineExceeded {
		result.Error = "timeout after 120s"
		st.UpdateAgentRun(runID, "timeout", fmt.Sprintf(`{"error":%q}`, result.Error))
		return result, nil
	}
	if err != nil {
		result.Error = fmt.Sprintf("exec: %v — stderr: %s", err, stderr.String())
		st.UpdateAgentRun(runID, "failed", fmt.Sprintf(`{"error":%q}`, result.Error))
		return result, nil
	}

	result.Output = stdout.String()
	st.UpdateAgentRun(runID, "completed", result.Output)

	return result, nil
}
```

- [ ] **Step 2: Add UpdateAgentRun to store**

Add to `internal/scout/store/agent_runs.go`:

```go
// UpdateAgentRun updates an agent run's status and result.
// On failure/timeout, the error is stored in result as JSON: {"error": "..."}.
// The recommendations field is reserved for parsed actionable items (populated by caller on success).
func (s *Store) UpdateAgentRun(id int64, status, result string) error {
	completedAt := ""
	if status == "completed" || status == "failed" || status == "timeout" {
		completedAt = now()
	}
	_, err := s.db.Exec(
		"UPDATE agent_runs SET status = ?, result = ?, completed_at = ? WHERE id = ?",
		status, result, completedAt, id,
	)
	return err
}
```

The launcher stores errors in `result` as `{"error": "..."}` JSON (not in `recommendations`). This keeps `recommendations` clean for actionable items parsed from successful runs.
```

- [ ] **Step 3: Verify compilation**

Run: `go build ./internal/scout/agent/`
Expected: BUILD SUCCESS

- [ ] **Step 4: Commit**

```bash
git add internal/scout/agent/launcher.go internal/scout/store/agent_runs.go
git commit -m "feat: rewrite agent launcher with exec.Command subprocess"
```

---

## Task 11: AI — Tier 3 Async Tools

**Files:** `internal/scout/ai/referral.go`, `internal/scout/ai/pitch.go`

- [ ] **Step 1: Implement referral.go**

`ReferralFinder(ctx, leadID)` — fetches lead, assembles prompt with company/hiring manager data, calls `agent.Launch` in a goroutine, returns `{run_id, status: "running"}` immediately (async pattern).

- [ ] **Step 2: Implement pitch.go**

`CompanyPitch(ctx, leadID)` — fetches lead + profile, assembles prompt, calls `agent.Launch` in a goroutine, returns `{run_id, status: "running"}`.

- [ ] **Step 3: Verify compilation**

Run: `go build ./internal/scout/ai/`
Expected: BUILD SUCCESS

- [ ] **Step 4: Commit**

```bash
git add internal/scout/ai/referral.go internal/scout/ai/pitch.go
git commit -m "feat: add async AI tools — referral_finder, company_pitch"
```

---

## Task 12: Server — New Endpoints & CORS

**Files:** `internal/scout/server/server.go`

- [ ] **Step 1: Add new fields and options to Server struct**

Add to `Server` struct:
```go
aiService   *ai.Service
scheduler   *sweep.Scheduler
configPath  string
```

Add options:
```go
func WithAIService(svc *ai.Service) Option { return func(s *Server) { s.aiService = svc } }
func WithScheduler(sc *sweep.Scheduler) Option { return func(s *Server) { s.scheduler = sc } }
func WithConfigPath(p string) Option { return func(s *Server) { s.configPath = p } }
func WithStore(st *store.Store) Option { return func(s *Server) { s.store = st } }
```

Remove: `cdpURL` field, `cdpClient` field, `WithCdpURL` option. Remove `WithStreamClient`, `WithTheirStackKey` (not needed — server receives the pre-built `aiService` and `scheduler`). Remove CDP setup in `Start()`. Store is now opened in `main()` and passed via `WithStore`.

- [ ] **Step 2: Update CORS middleware**

Change allowed methods from `"GET, POST, PATCH, OPTIONS"` to `"GET, POST, PATCH, PUT, OPTIONS"`.

- [ ] **Step 3: Register 9 new endpoints**

Add to `registerRoutes()`:
```go
// AI tools
s.mux.HandleFunc("POST /api/ai/match", s.handleAIMatch)
s.mux.HandleFunc("POST /api/ai/proposal", s.handleAIProposal)
s.mux.HandleFunc("POST /api/ai/cover-letter", s.handleAICoverLetter)
s.mux.HandleFunc("POST /api/ai/outreach", s.handleAIOutreach)
s.mux.HandleFunc("POST /api/ai/salary", s.handleAISalary)
s.mux.HandleFunc("POST /api/ai/referral", s.handleAIReferral)
s.mux.HandleFunc("POST /api/ai/pitch", s.handleAIPitch)

// Sweep config
s.mux.HandleFunc("GET /api/sweep/config", s.handleGetSweepConfig)
s.mux.HandleFunc("PUT /api/sweep/config", s.handlePutSweepConfig)
```

- [ ] **Step 4: Implement AI handlers**

Each synchronous AI handler follows this pattern:
```go
func (s *Server) handleAIMatch(w http.ResponseWriter, r *http.Request) {
	var body struct{ LeadID int64 `json:"lead_id"` }
	if err := decodeBody(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	result, err := s.aiService.ResumeMatch(r.Context(), body.LeadID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, result)
}
```

Async handlers (referral, pitch) return 202:
```go
func (s *Server) handleAIReferral(w http.ResponseWriter, r *http.Request) {
	var body struct{ LeadID int64 `json:"lead_id"` }
	if err := decodeBody(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	runID, err := s.aiService.ReferralFinder(r.Context(), body.LeadID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]interface{}{"run_id": runID, "status": "running"})
}
```

- [ ] **Step 5: Update sweep handlers to use scheduler**

Update `handleSweep`, `handleSweepNow`, `handleSweepStatus`, `handleSweepDigest` to call scheduler/TheirStack instead of the old stub.

`handleSweepNow`: calls `scheduler.RunNow()`, returns 202 Accepted or 409 if running.
`handleSweepStatus`: calls `scheduler.Status()`.
`handleSweepDigest`: reads `sweep_last_digest` from `sync_meta`, returns zero-value on missing key.

- [ ] **Step 6: Implement config handlers**

```go
func (s *Server) handleGetSweepConfig(w http.ResponseWriter, r *http.Request) {
	cfg, err := sweep.LoadConfig(s.configPath)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, cfg)
}

func (s *Server) handlePutSweepConfig(w http.ResponseWriter, r *http.Request) {
	var cfg sweep.SweepConfig
	if err := decodeBody(r, &cfg); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := sweep.SaveConfig(s.configPath, &cfg); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "saved"})
}
```

- [ ] **Step 7: Verify compilation**

Run: `go build ./internal/scout/server/`
Expected: BUILD SUCCESS

- [ ] **Step 8: Commit**

```bash
git add internal/scout/server/server.go
git commit -m "feat: add 9 scout endpoints (7 AI + 2 sweep config), update CORS"
```

---

## Task 13: Main.go — New Init Order

**Files:** `cmd/scout/main.go`

- [ ] **Step 1: Rewrite main.go**

Follow the init order from spec §5:
1. Open store in `main()` (not inside Server.Start)
2. Connect profiledb (optional)
3. Create stream.Client
4. Create AI service
5. Load sweep config
6. Create server with all dependencies
7. Create TheirStack client (if key present)
8. Start scheduler (if client present)
9. Graceful shutdown stops scheduler first, then server

Remove all CDP references. Add `SOUL_SCOUT_THEIRSTACK_KEY` env var.

- [ ] **Step 2: Verify compilation**

Run: `go build ./cmd/scout/`
Expected: BUILD SUCCESS

- [ ] **Step 3: Commit**

```bash
git add cmd/scout/main.go
git commit -m "feat: rewrite scout main with TheirStack, AI service, scheduler init"
```

---

## Task 14: Chat Context — Tool Definitions & Dispatch

**Files:** `internal/chat/context/scout.go`, `internal/chat/context/dispatch.go`

- [ ] **Step 1: Add 7 tool definitions to scout.go**

Append 7 new `stream.Tool` entries to the `scoutContext()` Tools slice. Follow the existing pattern. Async tools include description note: "This tool runs asynchronously. Check agent_status for results."

Tool names: `resume_match`, `proposal_gen`, `cover_letter`, `cold_outreach`, `salary_lookup`, `referral_finder`, `company_pitch`.

Input schemas per spec §5 chat context table.

- [ ] **Step 2: Add 7 dispatch routes to dispatch.go**

Add after existing scout routes:
```go
// Scout AI tools
"resume_match":    {Method: "POST", Path: "/api/ai/match", Product: "scout"},
"proposal_gen":    {Method: "POST", Path: "/api/ai/proposal", Product: "scout"},
"cover_letter":    {Method: "POST", Path: "/api/ai/cover-letter", Product: "scout"},
"cold_outreach":   {Method: "POST", Path: "/api/ai/outreach", Product: "scout"},
"salary_lookup":   {Method: "POST", Path: "/api/ai/salary", Product: "scout"},
"referral_finder": {Method: "POST", Path: "/api/ai/referral", Product: "scout"},
"company_pitch":   {Method: "POST", Path: "/api/ai/pitch", Product: "scout"},
```

- [ ] **Step 3: Verify compilation**

Run: `go build ./internal/chat/context/`
Expected: BUILD SUCCESS

- [ ] **Step 4: Commit**

```bash
git add internal/chat/context/scout.go internal/chat/context/dispatch.go
git commit -m "feat: add 7 AI tool definitions and dispatch routes for scout"
```

---

## Task 15: CLAUDE.md & Final Verification

**Files:** `CLAUDE.md`

- [ ] **Step 1: Update CLAUDE.md**

- Change Scout tool count: `Scout (21)` → `Scout (28)`
- Update total: `85 product tools + 8 built-in = 93 tools` → `92 product tools + 8 built-in = 100 tools`
- Add env var: `SOUL_SCOUT_THEIRSTACK_KEY | *(none)* | TheirStack API bearer token`
- Remove env var: `SOUL_SCOUT_CDP_URL`

- [ ] **Step 2: Run static verification**

Run: `make verify-static`
Expected: PASS (go vet, tsc --noEmit, secret scan)

- [ ] **Step 3: Run all scout tests**

Run: `go test -race -count=1 ./internal/scout/... -v`
Expected: ALL PASS

- [ ] **Step 4: Build all binaries**

Run: `make build`
Expected: 13 binaries including soul-scout

- [ ] **Step 5: Commit**

```bash
git add CLAUDE.md
git commit -m "docs: update CLAUDE.md with scout AI tools and TheirStack env var"
```

---

## Summary

| Task | What | Files | Tests |
|------|------|-------|-------|
| 1 | Store — Lead struct & migration | 2 | ~2 |
| 2 | Store — Leads CRUD | 2 | ~5 |
| 3 | Store — Analytics rewrite | 2 | ~1 |
| 4 | Sweep — Config | 2 | ~3 |
| 5 | Sweep — TheirStack client | 2 | ~3 |
| 6 | Sweep — Orchestrator | 2 | ~2 |
| 7 | Sweep — Scheduler | 2 | ~2 |
| 8 | AI — Service & ResumeMatch | 3 | ~2 |
| 9 | AI — Tier 1-2 tools | 5 | ~4 |
| 10 | Agent launcher | 2 | 0 |
| 11 | AI — Tier 3 async tools | 2 | 0 |
| 12 | Server — New endpoints | 1 | 0 |
| 13 | Main.go rewrite | 1 | 0 |
| 14 | Chat context | 2 | 0 |
| 15 | CLAUDE.md & verification | 1 | 0 |
| **Total** | | **~29 files** | **~20 tests** |
