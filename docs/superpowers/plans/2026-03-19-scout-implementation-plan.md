# Scout Strategy Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement the full Scout job search & career management system — 35 AI tools, pipeline runner, tier classification, gate UIs, and metrics dashboards — from the 8 design specs.

**Architecture:** Extend existing Scout module (`internal/scout/`) with: schema migrations (8 new columns + 4 new tables), pipeline runner (`internal/scout/runner/`), 28 new AI tools (`internal/scout/ai/`), and frontend gate UIs (`web/src/components/scout/`). All tools follow existing patterns in `ai/match.go` (sync) and `ai/referral.go` (async).

**Tech Stack:** Go 1.24, SQLite, React 19, TypeScript 5.9, Tailwind CSS v4, Claude API via `internal/chat/stream/`

**Specs:**
- `docs/superpowers/specs/2026-03-18-job-application-strategy-design.md`
- `docs/superpowers/specs/2026-03-18-networking-strategy-design.md`
- `docs/superpowers/specs/2026-03-18-freelance-strategy-design.md`
- `docs/superpowers/specs/2026-03-18-contracts-strategy-design.md`
- `docs/superpowers/specs/2026-03-18-consulting-strategy-design.md`
- `docs/superpowers/specs/2026-03-18-content-strategy-design.md`
- `docs/superpowers/specs/2026-03-18-profile-presence-design.md`
- `docs/superpowers/specs/2026-03-19-weekly-schedule-design.md`

**Resource constraint:** RPi 5, 4 cores, 16GB RAM. Max 6 parallel agents. Run `bash tools/resource-check.sh` before each batch.

**Agent mandate (include in EVERY agent prompt):** See CLAUDE.md → "Agent Mandate (30 rules)"

---

## Pre-flight Checks

- [ ] **Check 1:** Run `bash tools/resource-check.sh` — confirm 6 agents capacity
- [ ] **Check 2:** Verify `~/.soul-v2/resume-baseline.md` exists (user creates)
- [ ] **Check 3:** Verify `~/.soul-v2/dream-companies.json` exists (user creates)
- [ ] **Check 4:** Run `make verify` — confirm baseline is green
- [ ] **Check 5:** Create implementation branch: `git checkout -b feat/scout-strategy`

---

## HOUR 1: Foundation + Batch 1

### Task 1: Foundation — Schema + Pipelines + Validation (SEQUENTIAL)

**Files:**
- Modify: `internal/scout/store/store.go` (Lead struct, scanLead, schema, migration)
- Modify: `internal/scout/store/leads.go` (AddLead defaults)
- Modify: `internal/scout/store/analytics.go` (knownPipelines)
- Modify: `internal/scout/pipelines/pipelines.go` (new pipelines + updated stages)
- Modify: `internal/scout/server/server.go` (ValidateTransition enforcement)
- Test: `internal/scout/store/store_test.go`
- Test: `internal/scout/pipelines/pipelines_test.go`
- Test: `internal/scout/server/server_test.go`

- [ ] **Step 1: Add new columns to Lead struct**

Add to `Lead` struct in `store.go`:
```go
// New fields for strategy implementation
Tier             int    `json:"tier"`
ContactType      string `json:"contactType"`
Intent           string `json:"intent"`
Warmth           string `json:"warmth"`
InteractionCount int    `json:"interactionCount"`
Channels         string `json:"channels"`
LastInteractionAt string `json:"lastInteractionAt"`
SourceRefID      *int64 `json:"sourceRefId"`
```

Add to `scanLead` (after `&l.Metadata`):
```go
&l.Tier, &l.ContactType, &l.Intent, &l.Warmth,
&l.InteractionCount, &l.Channels, &l.LastInteractionAt, &l.SourceRefID,
```

Add to `leadColumns` (after `metadata`):
```
tier, contact_type, intent, warmth,
interaction_count, channels, last_interaction_at, source_ref_id
```

Add to `allowedLeadFields`:
```go
"tier": true, "contact_type": true, "intent": true, "warmth": true,
"interaction_count": true, "channels": true, "last_interaction_at": true,
"source_ref_id": true,
```

Add to `AddLead` defaults:
```go
if lead.Tier == 0 { lead.Tier = 3 }
if lead.Warmth == "" { lead.Warmth = "new" }
if lead.Channels == "" { lead.Channels = "[]" }
```

Add to INSERT statement (both column list and VALUES placeholders + args).

- [ ] **Step 2: Add schema migration**

Add `ensureMigrations()` method to `Store` (called from `New()`):
```go
func (s *Store) ensureMigrations() error {
    // Check if tier column exists
    columns := map[string]string{
        "tier": "INTEGER NOT NULL DEFAULT 3",
        "contact_type": "TEXT DEFAULT ''",
        "intent": "TEXT DEFAULT ''",
        "warmth": "TEXT DEFAULT 'new'",
        "interaction_count": "INTEGER DEFAULT 0",
        "channels": "TEXT DEFAULT '[]'",
        "last_interaction_at": "TEXT DEFAULT ''",
        "source_ref_id": "INTEGER",
    }
    for col, typedef := range columns {
        var exists bool
        rows, err := s.db.Query("PRAGMA table_info(leads)")
        if err != nil { return err }
        for rows.Next() {
            var cid int; var name, typ string; var notnull int; var dflt *string; var pk int
            rows.Scan(&cid, &name, &typ, &notnull, &dflt, &pk)
            if name == col { exists = true }
        }
        rows.Close()
        if !exists {
            _, err := s.db.Exec("ALTER TABLE leads ADD COLUMN " + col + " " + typedef)
            if err != nil { return fmt.Errorf("migrate %s: %w", col, err) }
        }
    }
    return nil
}
```

- [ ] **Step 3: Create new tables**

Add to schema creation in `New()`:
```sql
CREATE TABLE IF NOT EXISTS lead_artifacts (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    lead_id INTEGER NOT NULL REFERENCES leads(id),
    type TEXT NOT NULL,
    content TEXT NOT NULL,
    created_at TEXT NOT NULL DEFAULT (datetime('now'))
);
CREATE INDEX IF NOT EXISTS idx_lead_artifacts_lead ON lead_artifacts(lead_id);

CREATE TABLE IF NOT EXISTS interactions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    lead_id INTEGER NOT NULL REFERENCES leads(id),
    type TEXT NOT NULL,
    channel TEXT NOT NULL,
    description TEXT,
    created_at TEXT NOT NULL DEFAULT (datetime('now'))
);
CREATE INDEX IF NOT EXISTS idx_interactions_lead ON interactions(lead_id);

CREATE TABLE IF NOT EXISTS content_posts (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    platform TEXT NOT NULL,
    pillar TEXT NOT NULL,
    topic TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'draft',
    content TEXT NOT NULL,
    hook TEXT,
    scheduled_date TEXT,
    published_at TEXT,
    impressions INTEGER DEFAULT 0,
    likes INTEGER DEFAULT 0,
    comments INTEGER DEFAULT 0,
    shares INTEGER DEFAULT 0,
    saves INTEGER DEFAULT 0,
    profile_views INTEGER DEFAULT 0,
    inbound_leads INTEGER DEFAULT 0,
    post_url TEXT,
    created_at TEXT NOT NULL DEFAULT (datetime('now'))
);
CREATE INDEX IF NOT EXISTS idx_content_platform ON content_posts(platform);
CREATE INDEX IF NOT EXISTS idx_content_pillar ON content_posts(pillar);

CREATE TABLE IF NOT EXISTS content_backlog (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    topic TEXT NOT NULL,
    pillar TEXT NOT NULL,
    source TEXT NOT NULL,
    angle TEXT,
    status TEXT DEFAULT 'pending',
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    archived_at TEXT
);
```

- [ ] **Step 4: Update pipeline definitions**

In `pipelines.go`, update existing and add new:
```go
var Pipelines = map[string]Pipeline{
    "job":         {Stages: []string{"discovered", "qualified", "preparing", "outreach-sent", "applied", "screening", "interview", "offer", "joined"}, Terminal: []string{"joined", "rejected", "withdrawn", "skipped"}},
    "freelance":   {Stages: []string{"found", "proposal-ready", "proposal-sent", "shortlisted", "awarded", "delivering", "completed"}, Terminal: []string{"completed", "lost", "withdrawn"}},
    "contract":    {Stages: []string{"discovered", "applied", "screening", "interview", "offer", "engaged", "completed"}, Terminal: []string{"completed", "rejected", "withdrawn"}},
    "consulting":  {Stages: []string{"lead", "discovery-call", "proposal-sent", "negotiating", "engaged", "delivered"}, Terminal: []string{"delivered", "lost", "declined"}},
    "product-dev": {Stages: []string{"lead", "scoping", "proposal-sent", "negotiating", "building", "delivered"}, Terminal: []string{"delivered", "lost", "declined"}},
    "referral":    {Stages: []string{"identified", "connected", "conversation", "referral-asked", "referred", "interviewing", "offer"}, Terminal: []string{"offer", "declined", "no-response"}},
    "networking":  {Stages: []string{"identified", "connected", "engaging", "warm", "converting", "converted"}, Terminal: []string{"converted", "inactive", "not-relevant"}},
}
```

- [ ] **Step 5: Enforce ValidateTransition in server.go**

In `handleRecordAction` (line ~447), add before stage update:
```go
if body.Action != "" && body.Action != lead.Stage {
    if err := pipelines.ValidateTransition(lead.Pipeline, lead.Stage, body.Action); err != nil {
        writeError(w, http.StatusBadRequest, err.Error())
        return
    }
    // ... existing RecordStageHistory + UpdateLead code
}
```

In `handleToolExecute` case `"lead_action"` (line ~1164), add same validation:
```go
if action != "" && action != lead.Stage {
    if err := pipelines.ValidateTransition(lead.Pipeline, lead.Stage, action); err != nil {
        writeError(w, http.StatusBadRequest, err.Error())
        return
    }
    // ... existing code
}
```

- [ ] **Step 6: Update knownPipelines in analytics.go**

```go
var knownPipelines = []string{"job", "freelance", "contract", "consulting", "product-dev", "referral", "networking"}
```

- [ ] **Step 7: Write tests**

```go
// pipelines_test.go — test new pipelines and stages
func TestJobPipelineNewStages(t *testing.T) {
    assert.NoError(t, ValidateTransition("job", "discovered", "qualified"))
    assert.NoError(t, ValidateTransition("job", "qualified", "preparing"))
    assert.NoError(t, ValidateTransition("job", "preparing", "applied")) // skip outreach-sent
    assert.NoError(t, ValidateTransition("job", "discovered", "skipped")) // terminal from any
    assert.Error(t, ValidateTransition("job", "applied", "discovered")) // backward
}
func TestNetworkingPipeline(t *testing.T) {
    assert.NoError(t, ValidateTransition("networking", "identified", "connected"))
    assert.NoError(t, ValidateTransition("networking", "engaging", "converted")) // terminal
    assert.Error(t, ValidateTransition("networking", "warm", "connected")) // backward
}
func TestReferralPipeline(t *testing.T) {
    assert.NoError(t, ValidateTransition("referral", "identified", "connected"))
    assert.NoError(t, ValidateTransition("referral", "connected", "no-response")) // terminal
}

// store_test.go — test migration + new fields
func TestMigrationAddsNewColumns(t *testing.T) { /* verify all 8 columns exist after New() */ }
func TestLeadWithTierField(t *testing.T) { /* AddLead with Tier=1, GetLead, verify Tier=1 */ }
func TestLeadArtifactsCRUD(t *testing.T) { /* insert + query lead_artifacts */ }
func TestInteractionsCRUD(t *testing.T) { /* insert + query interactions */ }
func TestContentPostsCRUD(t *testing.T) { /* insert + query content_posts */ }
```

- [ ] **Step 8: Run verify**

Run: `make verify`
Expected: ALL tests pass. No regressions.

- [ ] **Step 9: Commit**

```bash
git add internal/scout/store/ internal/scout/pipelines/ internal/scout/server/
git commit -m "feat: foundation — schema migration, new tables, pipeline defs, ValidateTransition enforcement"
```

---

### Task 2-7: Batch 1 — 6 Parallel Agents (LAUNCH AFTER TASK 1)

**Pre-launch:** `bash tools/resource-check.sh` — confirm 6 agents available.

Each agent gets the full agent mandate from CLAUDE.md + the specific spec reference + the patterns from the Explore agent above.

#### Task 2: Pipeline Runner (Agent 1)

**Files:**
- Create: `internal/scout/runner/runner.go`
- Create: `internal/scout/runner/job.go`
- Create: `internal/scout/runner/runner_test.go`
- Test: `internal/scout/runner/runner_test.go`

**Agent prompt includes:**
- Job application spec Section 3.3 (pipeline phases)
- Runner spec: single goroutine, 5-min polling, 4 phases (QUALIFY, PREPARE, CADENCE, STALE)
- Must be race-safe (go test -race)
- Must handle errors gracefully (retry on next cycle, surface after 3 failures)

- [ ] Write failing test for runner start/stop lifecycle
- [ ] Implement runner skeleton (Start, Stop, polling loop)
- [ ] Write failing test for QUALIFY phase (discovered → qualified)
- [ ] Implement QUALIFY: find tier 1/2 leads at "discovered", call ResumeMatch, advance stage
- [ ] Write failing test for CADENCE phase (next_date checks)
- [ ] Implement CADENCE: find leads with next_date <= now, surface follow-ups
- [ ] Write failing test for STALE phase (7-day + 14-day)
- [ ] Implement STALE: find old "preparing" leads, flag/auto-skip
- [ ] Run: `go test -race -count=1 ./internal/scout/runner/...`
- [ ] Run: `make verify-static`
- [ ] Commit: `feat: pipeline runner with job QUALIFY, CADENCE, STALE phases`

#### Task 3: ResumeTailor + FreelanceScore (Agent 2)

**Files:**
- Create: `internal/scout/ai/resume.go`
- Create: `internal/scout/ai/resume_test.go`
- Create: `internal/scout/ai/freelance_score.go`
- Create: `internal/scout/ai/freelance_score_test.go`
- Modify: `internal/scout/server/server.go` (add cases to handleToolExecute)
- Modify: `internal/chat/context/scout.go` (register tools)
- Modify: `internal/chat/context/dispatch.go` (add routes)

**Agent prompt includes:**
- Follow `ai/match.go` pattern exactly (sync tool)
- ResumeTailor: reads `~/.soul-v2/resume-baseline.md`, tailors against lead JD
- FreelanceScore: scores gig 0-100 on 5 criteria (skill match, budget, scope clarity, client quality, time fit)
- Store results in `lead_artifacts` table
- Register both tools in scout.go + dispatch.go

- [ ] Write test for ResumeTailor (mock sender, verify output structure)
- [ ] Implement ResumeTailor following match.go pattern
- [ ] Write test for FreelanceScore
- [ ] Implement FreelanceScore
- [ ] Add `resume_tailor` and `freelance_score` to handleToolExecute
- [ ] Register in scout.go scoutTools + dispatch.go routes
- [ ] Run: `make verify-static`
- [ ] Commit 1: `feat: ResumeTailor AI tool — tailors resume against JD`
- [ ] Commit 2: `feat: FreelanceScore AI tool — scores gigs on 5 criteria`

#### Task 4: NetworkingDraft + WeeklyNetworkingBrief (Agent 3)

**Files:**
- Create: `internal/scout/ai/networking.go`
- Create: `internal/scout/ai/networking_test.go`
- Modify: `internal/scout/server/server.go`
- Modify: `internal/chat/context/scout.go`
- Modify: `internal/chat/context/dispatch.go`

**Agent prompt includes:**
- Networking spec Section 4.4
- NetworkingDraft: takes contactID + channel + activityContext, generates outreach draft
- WeeklyNetworkingBrief: aggregates all contacts, returns warmth changes + dormant + ready
- Uses `interactions` table for interaction history
- Follow match.go pattern (sync)

- [ ] Write test for NetworkingDraft
- [ ] Implement NetworkingDraft
- [ ] Write test for WeeklyNetworkingBrief
- [ ] Implement WeeklyNetworkingBrief
- [ ] Register in server.go + scout.go + dispatch.go
- [ ] Run: `make verify-static`
- [ ] Commit 1: `feat: NetworkingDraft AI tool — channel-aware outreach drafts`
- [ ] Commit 2: `feat: WeeklyNetworkingBrief AI tool — warmth summary`

#### Task 5: ContentSeriesGen + HookWriter + ContentTopicGen (Agent 4)

**Files:**
- Create: `internal/scout/ai/content_series.go`
- Create: `internal/scout/ai/content_topic.go`
- Create: `internal/scout/ai/hook_writer.go`
- Create: `internal/scout/ai/content_test.go`
- Modify: `internal/scout/server/server.go`
- Modify: `internal/chat/context/scout.go`
- Modify: `internal/chat/context/dispatch.go`

**Agent prompt includes:**
- Content spec Section 6.4
- ContentSeriesGen: topic + raw insights → 3 LinkedIn posts + 3 X versions + carousel outline
- HookWriter: draft post → 5 hook variations using 8 formulas
- ContentTopicGen: work + backlog + news → 3 topic suggestions
- Store drafts in `lead_artifacts` type="content_draft"
- Content backlog queries `content_backlog` table

- [ ] Write tests for all 3 tools
- [ ] Implement ContentSeriesGen
- [ ] Implement HookWriter
- [ ] Implement ContentTopicGen
- [ ] Register all in server.go + scout.go + dispatch.go
- [ ] Run: `make verify-static`
- [ ] Commit: `feat: content AI tools — series gen, hook writer, topic gen`

#### Task 6: ExpertApplication + CallPrepBrief (Agent 5)

**Files:**
- Create: `internal/scout/ai/expert_application.go`
- Create: `internal/scout/ai/call_prep.go`
- Create: `internal/scout/ai/consulting_test.go`
- Modify: `internal/scout/server/server.go`
- Modify: `internal/chat/context/scout.go`
- Modify: `internal/chat/context/dispatch.go`

**Agent prompt includes:**
- Consulting spec Section 5E.2 and 5E.4
- ExpertApplication: network name + focus → tailored application text
- CallPrepBrief: lead data → company background, likely questions, relevant experience
- ExpertApplication stores to filesystem (`~/.soul-v2/expert-applications/{name}.md`)
- CallPrepBrief stores to `lead_artifacts` type="call_prep"

- [ ] Write tests for both tools
- [ ] Implement ExpertApplication
- [ ] Implement CallPrepBrief
- [ ] Register in server.go + scout.go + dispatch.go
- [ ] Run: `make verify-static`
- [ ] Commit: `feat: consulting AI tools — expert application, call prep brief`

#### Task 7: TierClassifier + DreamCompanies (Agent 6)

**Files:**
- Create: `internal/scout/sweep/classify.go`
- Create: `internal/scout/sweep/classify_test.go`
- Modify: `internal/scout/sweep/sweep.go` (call ClassifyTier after JobToLead)

**Agent prompt includes:**
- Job application spec Section 3.1
- ClassifyTier: rule-based instant classification (dream list, funding, employee count, tech stack)
- LoadDreamCompanies: reads `~/.soul-v2/dream-companies.json`
- Must set lead.Tier after classification
- Remove Scorer call from RunSweep (scoring moves to pipeline runner)

- [ ] Write test for ClassifyTier (tier 1/2/3 rules)
- [ ] Write test for LoadDreamCompanies (JSON parsing)
- [ ] Implement ClassifyTier
- [ ] Implement LoadDreamCompanies
- [ ] Modify RunSweep to call ClassifyTier, remove Scorer
- [ ] Run: `make verify-static`
- [ ] Commit: `feat: tier classification engine — dream companies, funding rules`

---

### Post-Batch 1 Merge

- [ ] Merge all 6 agent branches one by one
- [ ] After EACH merge: `make verify-static`
- [ ] After ALL merged: `make verify` (full L1-L3)
- [ ] Fix any integration issues
- [ ] Commit any merge fixes

---

## HOUR 2: Batch 2 — Remaining AI Tools

**Pre-launch:** `bash tools/resource-check.sh`

#### Task 8: SOWGenerator + ContractFollowUp + CaseStudyDraft (Agent 7)

**Files:** Create `ai/sow.go`, `ai/case_study.go`, `ai/contract_followup.go` + tests
**Spec:** Contracts spec Section 5C.4
- [ ] Implement 3 tools following patterns + tests + register
- [ ] Commit per tool

#### Task 9: ConsultingFollowUp + AdvisoryProposal + ProjectProposal + ConsultingUpsellEvaluator (Agent 8)

**Files:** Create `ai/consulting_followup.go`, `ai/advisory_proposal.go`, `ai/project_proposal.go`, `ai/upsell_evaluator.go` + tests
**Spec:** Consulting spec Section 5E.4
- [ ] Implement 4 tools + tests + register
- [ ] Commit per tool

#### Task 10: ThreadConverter + SubstackExpander + ReactiveContentGen + EngagementReply (Agent 9)

**Files:** Create `ai/thread_converter.go`, `ai/substack_expander.go`, `ai/reactive_content.go`, `ai/engagement_reply.go` + tests
**Spec:** Content spec Section 6.4
- [ ] Implement 4 tools + tests + register
- [ ] Commit per tool

#### Task 11: ContentMetrics + LinkedInUpdate + GitHubREADMEGen (Agent 10)

**Files:** Create `ai/content_metrics.go`, `ai/linkedin_update.go`, `ai/github_readme.go` + tests
**Spec:** Content spec 6.4, Profile spec 7.5
- [ ] Implement 3 tools + tests + register
- [ ] Commit per tool

#### Task 12: ProfileAudit + TestimonialRequest + PinRecommendation (Agent 11)

**Files:** Create `ai/profile_audit.go`, `ai/testimonial_request.go`, `ai/pin_recommendation.go` + tests
**Spec:** Profile spec Section 7.5
- [ ] Implement 3 tools + tests + register
- [ ] Commit per tool

#### Task 13: ContractUpsellDetector + Runner Wiring (Agent 12)

**Files:** Create `ai/upsell.go`, `runner/networking.go`, `runner/freelance.go`, `runner/contracts.go`, `runner/consulting.go`, `runner/content.go`, `runner/profile.go` + tests
**Spec:** All runner phases from all specs
- [ ] Implement ContractUpsellDetector + test
- [ ] Wire networking phases into runner
- [ ] Wire freelance phases into runner
- [ ] Wire contracts + consulting phases into runner
- [ ] Wire content + profile phases into runner
- [ ] Run: `go test -race ./internal/scout/runner/...`
- [ ] Commit: `feat: runner wiring — all pipeline phases operational`

---

### Post-Batch 2 Merge

- [ ] Merge all 6 agents
- [ ] `make verify` — all green
- [ ] Fix integration issues
- [ ] **CHECKPOINT:** All 35 AI tools built. Runner operational. Backend COMPLETE.

---

## HOUR 3: Batch 3 — Frontend

**Pre-launch:** `bash tools/resource-check.sh`

Frontend agents use `incremental-decomposition` and `e2e-quality-gate` skills.

#### Task 14: Priority Queue Tab + Gate Framework (Agent 13)

**Files:**
- Create: `web/src/components/scout/PriorityQueue.tsx`
- Create: `web/src/components/scout/GateAction.tsx`
- Modify: `web/src/pages/ScoutPage.tsx` (add Priority Queue as first tab)
- Modify: `web/src/hooks/useScout.ts` (add API calls for artifacts, content_posts)

**Mandate:**
- data-testid on every button/interactive element
- Zinc dark theme
- Priority Queue: fetch all pending items across pipelines, sort by urgency
- GateAction: reusable component with [Approve] [Edit] [Skip] [Send] buttons
- tsc --noEmit must pass

- [ ] Priority Queue component with urgency sorting
- [ ] GateAction reusable component
- [ ] Wire into ScoutPage as first tab
- [ ] tsc --noEmit
- [ ] Commit: `feat: Priority Queue tab + gate action framework`

#### Task 15: Jobs + Freelance + Networking Gate UIs (Agent 14)

**Files:**
- Create: `web/src/components/scout/JobGate.tsx`
- Create: `web/src/components/scout/FreelanceGate.tsx`
- Create: `web/src/components/scout/NetworkingGate.tsx`

- [ ] JobGate: shows resume diff, cover letter, outreach draft, tier, score
- [ ] FreelanceGate: shows proposal, score, client info, maybe list
- [ ] NetworkingGate: shows outreach drafts, warmth level, channel
- [ ] All with data-testid, zinc theme
- [ ] Commit: `feat: job, freelance, networking gate UI components`

#### Task 16: Content + Consulting + Contracts + Profile Gate UIs (Agent 15)

**Files:**
- Create: `web/src/components/scout/ContentGate.tsx`
- Create: `web/src/components/scout/ConsultingGate.tsx`
- Create: `web/src/components/scout/ContractGate.tsx`
- Create: `web/src/components/scout/ProfileGate.tsx`
- Create: `web/src/components/scout/MetricsDashboard.tsx`

- [ ] ContentGate: publish calendar, content batch, engagement replies
- [ ] ConsultingGate: call prep, proposals, upsell evaluations
- [ ] ContractGate: company pitches, SOWs, upsell alerts
- [ ] ProfileGate: pending updates, quarterly audit
- [ ] MetricsDashboard: per-section metrics widgets
- [ ] Commit: `feat: content, consulting, contracts, profile gate UIs + metrics`

---

### Post-Batch 3 Merge

- [ ] Merge all 3 frontend agents
- [ ] `cd web && npx tsc --noEmit` — zero errors
- [ ] `make verify-static` — all green
- [ ] Visual spot-check each component
- [ ] **CHECKPOINT:** Frontend COMPLETE. All gates usable.

---

## HOUR 4: Integration + Ship

### Task 17: Integration Testing (SEQUENTIAL)

- [ ] `make build` — all 13 binaries compile
- [ ] `make verify` — full L1-L3 green
- [ ] Start scout server: `./soul-scout serve`
- [ ] Test TheirStack sweep with real API key
- [ ] Verify: leads ingested → tier classified → scored → artifacts generated
- [ ] Test each gate UI with real data
- [ ] Verify: Priority Queue shows items in correct priority order
- [ ] Fix any integration bugs
- [ ] Re-run `make verify`

### Task 18: First Real Run

- [ ] Run real sweep → review first batch of AI outputs
- [ ] Check: resume tailoring quality (are keywords matched?)
- [ ] Check: proposal draft quality (specific to gig?)
- [ ] Check: outreach draft quality (sounds human?)
- [ ] Check: scoring accuracy (do high-scored leads look like good fits?)
- [ ] Tune prompts if needed
- [ ] Re-verify after prompt changes

### Task 19: Ship

- [ ] Final `make verify` — all green
- [ ] `check-secrets` — no secrets in code
- [ ] Update `docs/scout/implementation-status.md` — mark all complete
- [ ] Commit: `feat: scout strategy implementation complete`
- [ ] Merge to master

---

## Progress Tracking

Update `docs/scout/implementation-status.md` after each batch:

```markdown
# Scout Implementation Status

## Foundation (Task 1)
- [ ] Schema migration
- [ ] New tables
- [ ] Pipeline definitions
- [ ] ValidateTransition fix

## Batch 1 — Hour 1 (Tasks 2-7)
- [ ] Pipeline runner
- [ ] ResumeTailor + FreelanceScore
- [ ] NetworkingDraft + WeeklyNetworkingBrief
- [ ] ContentSeriesGen + HookWriter + ContentTopicGen
- [ ] ExpertApplication + CallPrepBrief
- [ ] TierClassifier + DreamCompanies

## Batch 2 — Hour 2 (Tasks 8-13)
- [ ] SOWGenerator + ContractFollowUp + CaseStudyDraft
- [ ] ConsultingFollowUp + AdvisoryProposal + ProjectProposal + ConsultingUpsellEvaluator
- [ ] ThreadConverter + SubstackExpander + ReactiveContentGen + EngagementReply
- [ ] ContentMetrics + LinkedInUpdate + GitHubREADMEGen
- [ ] ProfileAudit + TestimonialRequest + PinRecommendation
- [ ] ContractUpsellDetector + Runner Wiring

## Batch 3 — Hour 3 (Tasks 14-16)
- [ ] Priority Queue + Gate Framework
- [ ] Jobs + Freelance + Networking Gate UIs
- [ ] Content + Consulting + Contracts + Profile Gate UIs

## Hour 4 (Tasks 17-19)
- [ ] Integration testing
- [ ] First real run
- [ ] Ship
```
