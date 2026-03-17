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
- **Incremental polling:** `discovered_at_gte` filter with cursor advanced by +1 second past last `discovered_at` to avoid boundary re-fetch (see §3 cursor logic)
- **Pagination:** Up to 500 per page, offset-based

### TheirStack response → Lead mapping

The `leads` table is redesigned from scratch to match TheirStack's response schema plus Scout pipeline fields.

**Required filter (at least one):** `posted_at_max_age_days`, `posted_at_gte`, `posted_at_lte`, `company_domain_or`, `company_linkedin_url_or`, `company_name_or`.

### Migration strategy

The migration checks the existing leads table and drops it only if empty:

```go
var count int
err := db.QueryRow("SELECT COUNT(*) FROM leads").Scan(&count)
if err == nil && count > 0 {
    log.Fatalf("scout: leads table has %d rows — refusing destructive migration. "+
        "Back up scout.db and delete it manually to proceed.", count)
}
// Safe to drop — table is empty or doesn't exist
db.Exec("DROP TABLE IF EXISTS leads")
```

Then creates the new schema below. Future schema changes must use `ALTER TABLE ADD COLUMN`.

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
    theirstack_id INTEGER,
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

CREATE UNIQUE INDEX idx_leads_theirstack_id ON leads(theirstack_id) WHERE theirstack_id IS NOT NULL;
CREATE INDEX idx_leads_stage ON leads(stage);
CREATE INDEX idx_leads_match_score ON leads(match_score);
CREATE INDEX idx_leads_country_code ON leads(country_code);
CREATE INDEX idx_leads_seniority ON leads(seniority);
CREATE INDEX idx_leads_remote ON leads(remote);
CREATE INDEX idx_leads_pipeline ON leads(pipeline);
CREATE INDEX idx_leads_company_domain ON leads(company_domain);
CREATE INDEX idx_leads_date_posted ON leads(date_posted);
```

**Dedup:** Partial unique index on `theirstack_id WHERE NOT NULL` — `INSERT OR IGNORE` prevents re-importing the same TheirStack job. Manual leads have `theirstack_id` NULL (unlimited NULLs allowed since partial index excludes them).

**Pipeline inference from `employment_statuses`:**

Deterministic rules applied in order, first match wins:

1. Array contains `"contract"` → pipeline `"contract"`
2. Array contains `"full_time"` → pipeline `"job"`
3. Array contains `"part_time"` or `"temporary"` → pipeline `"freelance"`
4. Array contains `"internship"` → pipeline `"job"`
5. Empty array or unknown values → pipeline `"job"` (safe default — most TheirStack results are job postings)

This handles mixed arrays like `["full_time", "contract"]` (→ `"contract"`, since contract check is first) and empty arrays (→ `"job"`). Manual override via `PATCH /api/leads/{id}` always possible.

**Metadata JSON blob** stores rarely-queried fields: latitude, longitude, postal_code, long_location, state_code, company.founded_year, company.annual_revenue_usd, company.long_description, company.seo_description, company.alexa_ranking, company.publicly_traded_symbol, company.investors, company.yc_batch, matching_phrases, matching_words, reposted, date_reposted, full hiring_team array beyond [0]. (Note: `short_location`, `normalized_title`, and `company_logo` have dedicated columns — they are NOT stored in metadata.)

### Sweep package structure

```
internal/scout/sweep/
  theirstack.go    — TheirStack HTTP client (accepts http.Client for testability)
  scheduler.go     — Cron-like sweep scheduler
  sweep.go         — Sweep orchestrator (replaces current stub)
  config.go        — Sweep config loading/saving
```

**Testability:** `TheirStackClient` accepts an `*http.Client` in its constructor. Tests inject a client with a custom `http.RoundTripper` that returns canned JSON responses — no network calls needed. This follows standard Go HTTP testing patterns.

**Delete:** `cdp.go` — CDP is no longer needed.

### Sweep config

Stored at `$SOUL_V2_DATA_DIR/scout/sweep-config.json` (resolves to `~/.soul-v2/scout/sweep-config.json` by default). All references to this path use `filepath.Join(dataDir, "scout", "sweep-config.json")` — never a hardcoded `~/` path.

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
  "credit_budget": 50,
  "auto_score_threshold": 70
}
```

**Credit budget rationale:** Default 50 credits/sweep × daily = 50/day. Free tier (200 credits/month) supports ~4 days of sweeps before exhaustion. For sustained free-tier use, set `credit_budget: 7` (200 ÷ 30 days ≈ 7/day). Paid tier ($59/mo) has much higher limits — raise `credit_budget` accordingly. The `GET /api/sweep/status` endpoint reports remaining credits so the user can tune this.

---

## 2. AI Tools

Seven Claude-powered tools across three tiers. Quick tools run in-process via `stream.Client`. Heavy tools spawn a Claude subprocess via the agent launcher.

All tools that score or tailor content fetch the user's profile from `profiledb.GetFullProfile()` (experience, projects, skills, education, certifications on PostgreSQL/titan-pc) and inject it as context.

**profiledb nil handling:** If `profiledb` is not configured (`SOUL_SCOUT_PG_URL` unset), AI tools that require profile data (resume_match, proposal_gen, cover_letter) return a clear error: `{"error": "profiledb not configured — set SOUL_SCOUT_PG_URL"}`. Tools that only use lead data (cold_outreach, salary_lookup) work without profiledb. Auto-scoring during sweep is skipped when profiledb is nil — leads are created with `match_score = 0`.

**Testability:** `ai.Service` accepts a `Sender` interface (matching `stream.Client`'s Send method) rather than a concrete `*stream.Client`. Tests inject a mock sender that returns canned responses. This follows the same pattern as `internal/sentinel/engine/engine_test.go`.

### Package structure

```
internal/scout/ai/
  ai.go           — Service struct (store, profiledb, Sender interface), shared helpers
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
- `platform` must be one of: `"upwork"`, `"freelancer"`, `"general"`. Server validates and returns 400 for invalid values.
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
- `RunNow() (SweepResult, error)` — triggers an immediate sweep, blocks until complete
- Stores last run timestamp in `sync_meta` (key: `sweep_last_run`)
- Stores cursor in `sync_meta` (key: `theirstack_cursor`) — stored as `max(discovered_at) + 1 second` to avoid boundary re-fetch on `>=` filter (see cursor logic below)

### Sweep flow per run

```
Phase 1 — Fetch all pages:
  1. Load sweep config from ~/.soul-v2/scout/sweep-config.json
  2. Load cursor from sync_meta (theirstack_cursor)
  3. creditsUsed = 0
  4. Loop:
     a. POST TheirStack API with config filters + discovered_at_gte cursor + offset
     b. If HTTP error:
        - 429 (rate limit): log, stop pagination, DO NOT update cursor
        - 402 (credits exhausted): log, stop pagination, DO NOT update cursor
        - 5xx / timeout: log, stop pagination, DO NOT update cursor
        - Process any jobs already fetched in prior pages normally
     c. For each job in response:
        - INSERT OR IGNORE (theirstack_id dedup)
        - Infer pipeline from employment_statuses
        - Set stage = "discovered", source = "theirstack"
        - Track: newLeadIDs[] for successfully inserted leads
     d. creditsUsed += len(response.data)
     e. If len(response.data) < limit OR creditsUsed >= credit_budget: stop
     f. Else: offset += limit, continue loop

Phase 2 — Auto-score (after all pages fetched):
  5. If profiledb is nil: skip scoring, leave match_score = 0
  6. For each lead_id in newLeadIDs:
     a. Call ai.ResumeMatch(lead_id) — in-process
     b. Update match_score in DB
     c. If ResumeMatch fails: log error, continue (don't abort scoring)

Phase 3 — Finalize:
  7. Compute newCursor = max(discovered_at from all fetched jobs) + 1 second
     (the +1s ensures discovered_at_gte on next run excludes the boundary row)
  8. Update theirstack_cursor to newCursor ONLY if Phase 1 had no errors
  9. Update sweep_last_run timestamp
  10. Build and store digest in sync_meta (sweep_last_digest)
  11. Return SweepResult: {new_leads, duplicates, scored, high_matches, errors}
```

**Cursor boundary logic:** TheirStack only supports `discovered_at_gte` (>=), not strict `>`. Storing `max + 1s` as the cursor converts >= into effective >. The 1-second granularity matches TheirStack's timestamp precision. Without this, every sweep re-fetches and re-charges for the boundary row(s).

**Pagination:** TheirStack returns up to 500 per page. Stop when returned count < limit or credit budget exceeded (default 50 credits per sweep).

**Error handling:** Cursor is only advanced on successful completion of all pages. On partial failure (e.g. page 2 of 4 returns 429), leads from pages 1-2 are kept but the cursor stays at the pre-sweep position — next sweep re-fetches from the same point, dedup via `INSERT OR IGNORE` prevents duplicate DB inserts (but credits are re-consumed). This is the safe tradeoff: no data loss at the cost of some redundant credits on retry.

**Auto-score latency:** For 50 leads at ~1-2s per Claude call, scoring takes ~1-2 minutes. This runs in the background scheduler goroutine — not blocking any HTTP handler. The sweep result is stored in `sync_meta` and retrieved via `GET /api/sweep/digest`.

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

**First-run behavior:** If `sweep_last_digest` key doesn't exist in `sync_meta`, `GET /api/sweep/digest` returns a zero-value response: `{"last_run": "", "next_run": "", "new_leads": 0, "duplicates": 0, "high_matches": 0, "high_match_leads": [], "score_distribution": {}}`.

### `POST /api/sweep/now` semantics

**Async with run tracking.** The handler calls `scheduler.RunNow()` which:
1. If already running, returns `409 Conflict` with `{"error": "sweep already in progress"}`
2. Otherwise, starts the sweep in a background goroutine
3. Returns immediately with `202 Accepted` and `{"run_id": 42, "status": "running"}`
4. Client polls `GET /api/sweep/status` to check completion, or waits for digest

This avoids the chat dispatch 10s timeout problem. The sweep runs in the scheduler goroutine regardless of how it was triggered (cron or manual).

### Long-running tool execution model

**Problem:** Chat tool dispatch enforces a ~10s request timeout. AI tools (resume_match ~2s, referral_finder ~120s) and sweep/now (minutes) exceed this.

**Solution:** Two execution models based on expected latency:

| Tool | Expected latency | Model |
|---|---|---|
| resume_match | ~2s | Synchronous (fits in 10s) |
| proposal_gen | ~3s | Synchronous |
| cover_letter | ~3s | Synchronous |
| salary_lookup | ~2s | Synchronous |
| cold_outreach | ~3s | Synchronous |
| referral_finder | ~60-120s | **Async** — returns `{run_id, status: "running"}`, poll `GET /api/agent/status` |
| company_pitch | ~60-120s | **Async** — returns `{run_id, status: "running"}`, poll `GET /api/agent/status` |
| sweep_now | ~minutes | **Async** — returns `{run_id, status: "running"}`, poll `GET /api/sweep/status` |

Async tools use the existing `agent_runs` table for tracking. The chat context tool descriptions include a note: "This tool runs asynchronously. Poll agent_status to check completion."

Synchronous in-process tools (Tier 1-2) complete well within the 10s dispatch timeout. No change needed to `dispatch.go` timeouts for these.

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
   - Spawns Claude CLI subprocess via `exec.Command` (see safety note below)
   - Pipes prompt to stdin, captures stdout
   - Updates `agent_runs` with status `"completed"` or `"failed"` + result
   - Returns `LaunchResult`
3. The calling function parses the output and returns structured JSON

**Subprocess invocation — SAFETY CRITICAL:**

Must use `exec.Command` directly — **never** invoke via shell (`sh -c`):

```go
cmd := exec.CommandContext(ctx, "claude", "--print", "--model", "claude-sonnet-4-6", "--max-turns", "5")
cmd.Stdin = strings.NewReader(prompt)  // prompt piped via stdin, not shell interpolation
var stdout, stderr bytes.Buffer
cmd.Stdout = &stdout
cmd.Stderr = &stderr
```

This avoids command injection from lead data (company names, job titles) that may contain shell metacharacters. The prompt is written to stdin as raw bytes — never passed through shell expansion.

**Subprocess details:**
- `--print` for non-interactive mode
- `--max-turns 5` caps tool use loops
- Claude CLI inherits OAuth credentials from `~/.claude/.credentials.json`
- Timeout: 120 seconds via `context.WithTimeout`. Kill process on exceed, mark run as `"timeout"`

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

**CORS update:** Add `PUT` to allowed methods in `corsMiddleware` (currently allows `GET, POST, PATCH, OPTIONS`). Required for `PUT /api/sweep/config` to work from browser/frontend.

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

**Initialization order** — store is opened in `main()` (not inside `Server.Start()`) so both server and scheduler receive the same store instance:

```go
// 1. Open store first
st, err := store.Open(filepath.Join(dataDir, "scout.db"))

// 2. Connect profiledb (optional — nil if SOUL_SCOUT_PG_URL unset)
var profileDB *profiledb.Client
if pgURL != "" {
    profileDB, err = profiledb.New(pgURL)
}

// 3. Create stream client for AI tools
theirStackKey := os.Getenv("SOUL_SCOUT_THEIRSTACK_KEY")
creds := auth.LoadCredentials()
streamClient := stream.NewClient(creds)

// 4. Create AI service (profileDB may be nil — service handles gracefully)
aiSvc := ai.New(st, profileDB, streamClient)

// 5. Load sweep config (creates default if missing)
sweepCfg, err := sweep.LoadConfig(filepath.Join(dataDir, "scout", "sweep-config.json"))

// 6. Create server with all dependencies
srv := server.New(
    server.WithStore(st),
    server.WithAIService(aiSvc),
    server.WithStreamClient(streamClient),
    server.WithTheirStackKey(theirStackKey),
    // ... existing options ...
)

// 7. Create TheirStack client (injectable http.Client for testability)
var tsClient *sweep.TheirStackClient
if theirStackKey != "" {
    tsClient = sweep.NewTheirStackClient(theirStackKey, http.DefaultClient)
}

// 8. Start scheduler (only if TheirStack client configured)
if tsClient != nil {
    scheduler := sweep.NewScheduler(sweepCfg, st, aiSvc, tsClient)
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
| `internal/scout/store/leads.go` | `ListLeads(pipelineFilter, activeOnly)` — `type` filter renamed to `pipeline` filter. `AddLead` validates `pipeline` instead of `type`. Add `AddLeadIfNotExists(theirstack_id)` for dedup insert. All scan functions updated for new columns. |
| `internal/scout/store/analytics.go` | All `type` column references changed to `pipeline`. `AggregateStats` groups by pipeline, seniority, remote, country_code. `ConversionMetrics` filters by pipeline. `ActionableInsights` unchanged. |
| `internal/scout/store/store_test.go` | Tests rewritten for new Lead struct. Same coverage: CRUD, scoring, analytics, dedup. |
| `internal/scout/sweep/sweep.go` | Complete rewrite. `Sweep()` calls TheirStack, creates leads, triggers auto-scoring. |
| `internal/scout/agent/launcher.go` | Rewrite stub to actual subprocess launcher using `os/exec`. |
| `internal/scout/server/server.go` | Add AI + scheduler fields, 9 new endpoints, remove CDP references. Add `PUT` to CORS allowed methods. |
| `cmd/scout/main.go` | Add stream.Client, TheirStack key, scheduler startup. Remove CDP. |
| `internal/chat/context/scout.go` | Add 7 new tool definitions (28 total). All 21 existing tools preserved unchanged. New AI tools use dedicated `/api/ai/*` paths (not `/api/tools/{name}/execute`). |
| `internal/chat/context/dispatch.go` | Add 7 new dispatch routes for AI tools. Existing 21 routes unchanged. |
| `CLAUDE.md` | Update Scout tool count from 21 to 28. Update total tools from 93 to 100. Add `SOUL_SCOUT_THEIRSTACK_KEY` to env vars table. Remove `SOUL_SCOUT_CDP_URL`. |

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
