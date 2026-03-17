# Scout Setup: TheirStack Integration + AI Tools

> **Goal:** Replace Scout's stubbed sweep (CDP/go-rod) with TheirStack API for job discovery, add 7 Claude-powered AI tools for lead scoring and action generation, and implement a cron scheduler with auto-scoring.

**Architecture shift:** Custom browser scraping → single REST API. Freelance platform tracking remains manual entry via existing endpoints.

**Approach:** Poll + Auto-Score + Manual Action. TheirStack discovers leads on schedule, Claude auto-scores each lead against the user's profile (profiledb on PostgreSQL/titan-pc), high-match leads surface in a digest. User triggers action tools (cover letter, proposal, outreach) manually.

---

## 1. Sweep — TheirStack Integration

### TheirStack API

- **Endpoint:** `POST https://api.theirstack.com/v1/jobs/search`
- **Auth:** Bearer token via `SOUL_SCOUT_THEIRSTACK_KEY` env var
- **Cost:** 1 API credit per job returned
- **Free tier:** 200 credits/month
- **Dedup:** TheirStack deduplicates the same job across platforms upstream
- **Incremental polling:** `discovered_at_gte` filter fetches only new jobs since last sweep
- **Pagination:** Up to 500 per page, offset-based

### TheirStack response → Lead mapping

The `leads` table is redesigned from scratch to match TheirStack's response schema plus Scout pipeline fields.

**Required filter (at least one):** `posted_at_max_age_days`, `posted_at_gte`, `posted_at_lte`, `company_domain_or`, `company_linkedin_url_or`, `company_name_or`.

### New leads table schema

```sql
CREATE TABLE leads (
    -- Scout internal
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    source TEXT NOT NULL DEFAULT 'theirstack',
    pipeline TEXT NOT NULL DEFAULT '',
    stage TEXT NOT NULL DEFAULT 'discovered',
    match_score REAL DEFAULT 0,
    next_action TEXT NOT NULL DEFAULT 'review',
    next_date TEXT NOT NULL DEFAULT '',
    notes TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    closed_at TEXT NOT NULL DEFAULT '',

    -- TheirStack job identity
    theirstack_id INTEGER UNIQUE,
    job_title TEXT NOT NULL DEFAULT '',
    url TEXT NOT NULL DEFAULT '',
    final_url TEXT NOT NULL DEFAULT '',
    source_url TEXT NOT NULL DEFAULT '',
    date_posted TEXT NOT NULL DEFAULT '',
    discovered_at TEXT NOT NULL DEFAULT '',
    description TEXT NOT NULL DEFAULT '',
    normalized_title TEXT NOT NULL DEFAULT '',

    -- Location
    location TEXT NOT NULL DEFAULT '',
    short_location TEXT NOT NULL DEFAULT '',
    country TEXT NOT NULL DEFAULT '',
    country_code TEXT NOT NULL DEFAULT '',
    remote INTEGER DEFAULT 0,
    hybrid INTEGER DEFAULT 0,

    -- Compensation
    salary_string TEXT NOT NULL DEFAULT '',
    min_annual_salary_usd REAL DEFAULT 0,
    max_annual_salary_usd REAL DEFAULT 0,
    salary_currency TEXT NOT NULL DEFAULT '',

    -- Job attributes
    seniority TEXT NOT NULL DEFAULT '',
    employment_statuses TEXT NOT NULL DEFAULT '[]',
    easy_apply INTEGER DEFAULT 0,
    technology_slugs TEXT NOT NULL DEFAULT '[]',
    keyword_slugs TEXT NOT NULL DEFAULT '[]',

    -- Company
    company TEXT NOT NULL DEFAULT '',
    company_domain TEXT NOT NULL DEFAULT '',
    company_industry TEXT NOT NULL DEFAULT '',
    company_employee_count INTEGER DEFAULT 0,
    company_linkedin_url TEXT NOT NULL DEFAULT '',
    company_total_funding_usd REAL DEFAULT 0,
    company_funding_stage TEXT NOT NULL DEFAULT '',
    company_logo TEXT NOT NULL DEFAULT '',
    company_country TEXT NOT NULL DEFAULT '',

    -- Hiring team
    hiring_manager TEXT NOT NULL DEFAULT '',
    hiring_manager_linkedin TEXT NOT NULL DEFAULT '',

    -- Everything else from TheirStack response
    metadata TEXT NOT NULL DEFAULT '{}'
);

CREATE UNIQUE INDEX idx_leads_theirstack_id ON leads(theirstack_id);
CREATE INDEX idx_leads_stage ON leads(stage);
CREATE INDEX idx_leads_match_score ON leads(match_score);
CREATE INDEX idx_leads_country_code ON leads(country_code);
CREATE INDEX idx_leads_seniority ON leads(seniority);
CREATE INDEX idx_leads_remote ON leads(remote);
CREATE INDEX idx_leads_pipeline ON leads(pipeline);
CREATE INDEX idx_leads_company_domain ON leads(company_domain);
CREATE INDEX idx_leads_date_posted ON leads(date_posted);
```

**Dedup:** `theirstack_id UNIQUE` — `INSERT OR IGNORE` prevents re-importing. Manual leads have `theirstack_id` NULL.

**Pipeline inference from `employment_statuses`:**
- `["full_time"]` → pipeline `"job"`
- `["contract"]` → pipeline `"contract"`
- `["temporary"]`, `["part_time"]` → pipeline `"freelance"`
- Manual override always possible

**Metadata JSON blob** stores rarely-queried fields: latitude, longitude, postal_code, short_location, long_location, state_code, company.logo, company.founded_year, company.annual_revenue_usd, company.long_description, company.seo_description, company.alexa_ranking, company.publicly_traded_symbol, company.investors, company.yc_batch, matching_phrases, matching_words, normalized_title, reposted, date_reposted, full hiring_team array beyond [0].

### Sweep package structure

```
internal/scout/sweep/
  theirstack.go    — TheirStack HTTP client
  scheduler.go     — Cron-like sweep scheduler
  sweep.go         — Sweep orchestrator (replaces current stub)
  config.go        — Sweep config loading/saving
```

**Delete:** `cdp.go` — CDP is no longer needed.

### Sweep config

Stored at `~/.soul-v2/scout/sweep-config.json`:

```go
type SweepConfig struct {
    // TheirStack filters
    JobTitleOr           []string `json:"job_title_or"`
    JobTitleNot          []string `json:"job_title_not"`
    JobCountryCodeOr     []string `json:"job_country_code_or"`
    JobTechnologySlugOr  []string `json:"job_technology_slug_or"`
    JobLocationPatternOr []string `json:"job_location_pattern_or"`
    Remote               *bool    `json:"remote,omitempty"`
    SeniorityOr          []string `json:"seniority_or,omitempty"`
    MinSalaryUSD         *float64 `json:"min_salary_usd,omitempty"`
    PostedAtMaxAgeDays   int      `json:"posted_at_max_age_days"`
    Limit                int      `json:"limit"`

    // Scout settings
    IntervalHours        int      `json:"interval_hours"`
    CreditBudget         int      `json:"credit_budget"`
    AutoScoreThreshold   float64  `json:"auto_score_threshold"`
}
```

**Default config** (created on first run if file missing):

```json
{
  "job_title_or": ["software engineer", "full stack developer", "backend engineer", "golang developer"],
  "job_country_code_or": ["IN", "US", "GB", "DE", "NL", "SG"],
  "job_technology_slug_or": ["go", "react", "typescript", "python", "postgresql"],
  "remote": true,
  "posted_at_max_age_days": 7,
  "limit": 50,
  "interval_hours": 24,
  "credit_budget": 200,
  "auto_score_threshold": 70
}
```

---

## 2. AI Tools

Seven Claude-powered tools across three tiers. Quick tools run in-process via `stream.Client`. Heavy tools spawn a Claude subprocess via the agent launcher.

All tools that score or tailor content fetch the user's profile from `profiledb.GetFullProfile()` (experience, projects, skills, education, certifications on PostgreSQL/titan-pc) and inject it as context.

### Package structure

```
internal/scout/ai/
  ai.go           — Service struct (store, profiledb, stream.Client), shared helpers
  match.go        — resume_match
  proposal.go     — proposal_gen
  cover.go        — cover_letter
  outreach.go     — cold_outreach
  salary.go       — salary_lookup
  referral.go     — referral_finder (subprocess)
  pitch.go        — company_pitch (subprocess)
```

### Tier 1 — Core (in-process via stream.Client)

**`resume_match(lead_id)`**
- Fetches lead from store (job_title, description, technology_slugs, seniority, location)
- Fetches profile from profiledb
- Claude prompt: "Score this resume against this JD. Return JSON: `{score: 0-100, strengths: [...], gaps: [...], suggestions: [...]}`"
- Updates `lead.match_score` in store
- Called automatically during sweep for every new lead
- ~1 Claude call, small context

**`proposal_gen(lead_id, platform)`**
- Fetches lead + profile
- Platform-aware: Upwork (short, punchy, mention budget), Freelancer (competitive bid), general (cover letter style)
- Returns tailored proposal text
- ~1 Claude call, medium context

### Tier 2 — High Value (in-process via stream.Client)

**`cover_letter(lead_id)`**
- Fetches lead + profile
- Generates tailored cover letter matching JD keywords to experience
- Returns formatted text ready to paste

**`cold_outreach(lead_id)`**
- Fetches lead + company info (industry, employee_count, funding, domain from lead columns)
- Claude identifies specific value proposition from available data
- Returns personalized email draft

**`salary_lookup(lead_id)`**
- Fetches lead (job_title, location, seniority, company_employee_count, company_industry)
- Claude estimates market rate
- Returns JSON: `{min, median, max, currency, reasoning, sources}`

### Tier 3 — Complex (subprocess via agent launcher)

**`referral_finder(lead_id)`**
- Fetches lead (company, company_linkedin_url, hiring_manager_linkedin)
- Spawns Claude subprocess with web search capability
- Searches for mutual LinkedIn connections at the target company
- Returns: `{connections: [{name, role, relationship}], referral_template: "..."}`
- Subprocess because it may need multiple web searches

**`company_pitch(lead_id)`**
- Fetches lead + full profile + company data
- Spawns Claude subprocess
- Generates multi-section team augmentation pitch: company research, pain points, proposed engagement, relevant portfolio projects, pricing
- Returns structured pitch document
- Subprocess because it's multi-step research + long-form generation

### Auto-scoring in sweep

When sweep creates new leads, it calls `resume_match` in-process for each lead. The sweep result includes how many leads were scored and how many exceeded the threshold (default 70).

---

## 3. Scheduler & Digest

### Scheduler

A goroutine in the scout server process. Runs TheirStack sweeps on a configurable interval.

**`internal/scout/sweep/scheduler.go`:**

```go
type Scheduler struct {
    interval  time.Duration      // default 24h
    config    *SweepConfig
    store     *store.Store
    ai        *ai.Service
    client    *TheirStackClient
    ticker    *time.Ticker
    stopCh    chan struct{}
    mu        sync.Mutex
    running   bool
    lastRun   time.Time
}
```

- `Start()` — launches goroutine, runs sweep immediately on first start, then on interval
- `Stop()` — signals goroutine to exit
- `RunNow()` — triggers an immediate sweep (for `POST /api/sweep/now`)
- Stores last run timestamp in `sync_meta` (key: `sweep_last_run`)
- Stores last TheirStack `discovered_at` in `sync_meta` (key: `theirstack_cursor`)

### Sweep flow per run

```
1. Load sweep config from ~/.soul-v2/scout/sweep-config.json
2. Load cursor from sync_meta (theirstack_cursor)
3. POST TheirStack API with config filters + discovered_at_gte cursor
4. For each job in response:
   a. Check theirstack_id uniqueness (INSERT OR IGNORE)
   b. Infer pipeline from employment_statuses
   c. Set stage = "discovered", source = "theirstack"
   d. Insert lead
5. For each newly inserted lead:
   a. Call ai.ResumeMatch(lead_id) — in-process
   b. Update match_score in DB
6. Update theirstack_cursor to max(discovered_at) from response
7. Update sweep_last_run timestamp
8. Paginate if total_results > limit (offset-based)
9. Return SweepResult: {new_leads, duplicates, scored, high_matches, errors}
```

**Pagination:** TheirStack returns up to 500 per page. Stop when returned count < limit or credit budget exceeded (default 200 credits per sweep = 200 jobs).

### Digest

**`GET /api/sweep/digest`** returns:

```json
{
  "last_run": "2026-03-17T06:00:00Z",
  "next_run": "2026-03-18T06:00:00Z",
  "new_leads": 34,
  "duplicates": 12,
  "high_matches": 5,
  "high_match_leads": [
    {"id": 42, "job_title": "Senior Go Engineer", "company": "Stripe", "match_score": 89, "salary_string": "$180k-220k"}
  ],
  "score_distribution": {"90+": 2, "80-89": 3, "70-79": 8, "below_70": 21}
}
```

Stored in `sync_meta` as `sweep_last_digest` (JSON blob), updated after each sweep run.

---

## 4. Agent Launcher (Subprocess Tools)

`referral_finder` and `company_pitch` need multi-step reasoning with web access. The launcher spawns a Claude CLI subprocess.

### Launcher design

**`internal/scout/agent/launcher.go`** (replaces current stub):

```go
type LaunchConfig struct {
    Mode    string // "referral" or "pitch"
    LeadID  int64
    Prompt  string // assembled by the calling ai/ function
    DataDir string // ~/.soul-v2/scout/agent-runs/
}

type LaunchResult struct {
    RunID      int64
    Output     string
    TokensUsed int
    Duration   time.Duration
    Error      string
}
```

**Flow:**
1. `ai/referral.go` or `ai/pitch.go` assembles the full prompt (lead data + profile + instructions)
2. Calls `agent.Launch(config)` which:
   - Creates an `agent_runs` record with status `"running"`
   - Spawns `claude` CLI subprocess: `echo "<prompt>" | claude --print --model claude-sonnet-4-6 --max-turns 5`
   - Captures stdout
   - Updates `agent_runs` with status `"completed"` or `"failed"` + result
   - Returns `LaunchResult`
3. The calling function parses the output and returns structured JSON

**Subprocess details:**
- `--print` for non-interactive mode
- `--max-turns 5` caps tool use loops
- Claude CLI inherits OAuth credentials from `~/.claude/.credentials.json`
- Timeout: 120 seconds. Kill process on exceed, mark run as `"timeout"`

**Why subprocess over stream.Client:** The subprocess gets Claude Code's built-in tools (web search, file operations) that `stream.Client` doesn't have. `referral_finder` needs web search for LinkedIn. `company_pitch` needs web search for company research.

### Agent run tracking

Existing `agent_runs` table schema works as-is:
- `platform` → `"claude"`
- `mode` → `"referral"` or `"pitch"`
- `lead_id` → target lead
- `status` → `"running"` / `"completed"` / `"failed"` / `"timeout"`
- `result` → Claude's JSON output
- `recommendations` → parsed actionable items

Existing endpoints `GET /api/agent/status` and `GET /api/agent/history` work unchanged.

---

## 5. Chat Context & Server Integration

### Server struct changes

New fields:
```go
type Server struct {
    // ... existing fields ...
    aiService   *ai.Service
    scheduler   *sweep.Scheduler
}
```

New options:
```go
WithStreamClient(c *stream.Client) Option
WithTheirStackKey(key string) Option
```

Remove: `cdpURL` field, `cdpClient` field, `WithCdpURL` option.

### New REST endpoints (9 new, 32 total)

```
POST /api/ai/match          — resume_match
POST /api/ai/proposal       — proposal_gen
POST /api/ai/cover-letter   — cover_letter
POST /api/ai/outreach       — cold_outreach
POST /api/ai/salary         — salary_lookup
POST /api/ai/referral       — referral_finder
POST /api/ai/pitch          — company_pitch
GET  /api/sweep/config      — read sweep config
PUT  /api/sweep/config      — update sweep config
```

Existing sweep endpoints stay at the same paths, call new TheirStack implementation.

### cmd/scout/main.go changes

```go
theirStackKey := os.Getenv("SOUL_SCOUT_THEIRSTACK_KEY")
creds := auth.LoadCredentials()
streamClient := stream.NewClient(creds)
aiSvc := ai.New(st, profileDB, streamClient)

srv := server.New(
    server.WithStreamClient(streamClient),
    server.WithTheirStackKey(theirStackKey),
    // ... existing options ...
)

if theirStackKey != "" {
    scheduler := sweep.NewScheduler(sweepCfg, st, aiSvc, theirStackKey)
    scheduler.Start()
    defer scheduler.Stop()
}
```

### Chat context additions

**`internal/chat/context/scout.go`** — 7 new tool definitions (28 total):

| Tool name | Input schema |
|---|---|
| `resume_match` | `{lead_id: int}` |
| `proposal_gen` | `{lead_id: int, platform: string}` |
| `cover_letter` | `{lead_id: int}` |
| `cold_outreach` | `{lead_id: int}` |
| `salary_lookup` | `{lead_id: int}` |
| `referral_finder` | `{lead_id: int}` |
| `company_pitch` | `{lead_id: int}` |

**`internal/chat/context/dispatch.go`** — 7 new routes:

```go
"resume_match":    {Product: "scout", Method: "POST", Path: "/api/ai/match"},
"proposal_gen":    {Product: "scout", Method: "POST", Path: "/api/ai/proposal"},
"cover_letter":    {Product: "scout", Method: "POST", Path: "/api/ai/cover-letter"},
"cold_outreach":   {Product: "scout", Method: "POST", Path: "/api/ai/outreach"},
"salary_lookup":   {Product: "scout", Method: "POST", Path: "/api/ai/salary"},
"referral_finder": {Product: "scout", Method: "POST", Path: "/api/ai/referral"},
"company_pitch":   {Product: "scout", Method: "POST", Path: "/api/ai/pitch"},
```

### New environment variable

| Variable | Default | Purpose |
|---|---|---|
| `SOUL_SCOUT_THEIRSTACK_KEY` | *(none)* | TheirStack API bearer token |

`SOUL_SCOUT_CDP_URL` becomes unused — removed from main.go and server options.

### Go dependency changes

- **No new deps** — TheirStack is plain HTTP (`net/http`), subprocess uses `os/exec`
- **Keep:** `pgx/v5` (profiledb)
- **Never added:** `go-rod/rod` (no longer needed)

---

## 6. Change Inventory

### Delete

| File | Reason |
|---|---|
| `internal/scout/sweep/cdp.go` | CDP replaced by TheirStack API |

### Rewrite

| File | What changes |
|---|---|
| `internal/scout/store/store.go` | New `leads` table schema (45 columns). Lead struct rewritten. `scanLead`, `leadColumns`, `allowedLeadFields` updated. Other 6 tables unchanged. |
| `internal/scout/store/leads.go` | Same API signatures, new columns. Add `AddLeadIfNotExists(theirstack_id)` for dedup insert. |
| `internal/scout/store/analytics.go` | `AggregateStats` adds group-by seniority, remote, country_code. `ConversionMetrics` and `ActionableInsights` mostly unchanged. |
| `internal/scout/store/store_test.go` | Tests rewritten for new Lead struct. Same coverage: CRUD, scoring, analytics, dedup. |
| `internal/scout/sweep/sweep.go` | Complete rewrite. `Sweep()` calls TheirStack, creates leads, triggers auto-scoring. |
| `internal/scout/agent/launcher.go` | Rewrite stub to actual subprocess launcher using `os/exec`. |
| `internal/scout/server/server.go` | Add AI + scheduler fields, 9 new endpoints, remove CDP references. |
| `cmd/scout/main.go` | Add stream.Client, TheirStack key, scheduler startup. Remove CDP. |
| `internal/chat/context/scout.go` | Add 7 new tool definitions (28 total). |
| `internal/chat/context/dispatch.go` | Add 7 new dispatch routes. |

### New files

| File | Purpose |
|---|---|
| `internal/scout/sweep/theirstack.go` | TheirStack HTTP client |
| `internal/scout/sweep/scheduler.go` | Cron-like sweep scheduler |
| `internal/scout/sweep/config.go` | Sweep config load/save |
| `internal/scout/ai/ai.go` | AI service struct, shared helpers |
| `internal/scout/ai/match.go` | resume_match |
| `internal/scout/ai/proposal.go` | proposal_gen |
| `internal/scout/ai/cover.go` | cover_letter |
| `internal/scout/ai/outreach.go` | cold_outreach |
| `internal/scout/ai/salary.go` | salary_lookup |
| `internal/scout/ai/referral.go` | referral_finder |
| `internal/scout/ai/pitch.go` | company_pitch |

### Unchanged

| File/Package | Why |
|---|---|
| `internal/scout/pipelines/` | 5 pipeline types + validation untouched |
| `internal/scout/profiledb/` | Already works, no changes needed |
| `internal/scout/store/optimizations.go` | Unchanged |
| `internal/scout/store/agent_runs.go` | Schema fits launcher needs as-is |
| `internal/scout/store/sync.go` | Used for cursor/timestamp storage as-is |
| `internal/chat/server/proxy.go` | Scout proxy already configured |

### Summary counts

| | Count |
|---|---|
| Files deleted | 1 |
| Files rewritten | 10 |
| Files created | 11 |
| Files unchanged | 8+ |
| New REST endpoints | 9 |
| New chat tools | 7 |
| New env vars | 1 |
| New Go deps | 0 |
