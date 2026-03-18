# Job Application Strategy — Design Spec

**Date:** 2026-03-18
**Status:** Approved
**Scope:** Section 3 of `docs/scout/README.md` — jobs only (freelance/contract/consulting covered separately in Section 5, see `docs/scout/freelance.md`, `docs/scout/contracts.md`, `docs/scout/consulting.md`)

---

## Overview

AI-driven job application pipeline with human gates. Scout automates discovery, classification, scoring, resume tailoring, and outreach drafting. Human reviews in daily batches and executes sends. Three gates protect quality while enabling aggressive throughput (5-8 applications/week, 3 Tier 1 warm approaches).

## Design Decisions

| Decision | Choice | Why |
|---|---|---|
| Classification approach | Hybrid A+B (rules first, scoring second) | Rules are instant and free; scoring is expensive (Claude call per lead). Layer them. |
| Pipeline stages | 9 stages (added: qualified, preparing, outreach-sent, skipped) | Current 6 stages have no visibility between discovered → applied |
| Automation model | AI-driven with human gates (Approach B) | Scales to aggressive volume without sacrificing quality. AI proposes, human disposes. |
| Time-boxing | Split: lead-attached = strict cadence, relationship = soft tracking | Job postings expire; relationships don't. Different rules for each. |
| Application volume | Aggressive (C): 20-30 reviewed, 5-8 applied/week | AI handles prep work; human bottleneck is only review + send |
| Auth | Claude OAuth token (Max sub) via internal/chat/stream/ | No separate API key. Same auth as soul-chat. |

---

## 3.1: Tier Classification Engine

### Step 1 — Instant Classification (on discovery, zero API cost)

**Tier 1 (warm-only):**
- Dream company list match (`~/.soul-v2/dream-companies.json`)
- OR `company_total_funding_usd` > $50M
- OR `company_employee_count` > 500 AND `company_industry` contains "AI"
- OR `lead.TechnologySlugs` intersects `["anthropic", "openai", "deepmind"]` (tech stack is per-job, not per-company)

**Dream company list (user-maintained):**
Anthropic, DeepMind, OpenAI, Google AI, Meta AI (FAIR), Microsoft Research, Apple ML, Amazon AGI, Cohere, Mistral, Databricks, Hugging Face, Stability AI, Adept, Character.AI, Inflection, xAI, Perplexity, Runway, Midjourney

**Tier 2 (career page + outreach):**
- `company_funding_stage` in `[seed, series_a, series_b, series_c]`
- AND `lead.TechnologySlugs` intersects AI keywords
- AND `company_employee_count` 50-500

**Tier 3 (speed apply):**
- Everything else that passed TheirStack filters

### Step 2 — Match Scoring (async, Claude via OAuth — Tier 1/2 only)

- Tier 1/2 leads → queued for `resume_match` scoring
- Score > 85: promote tier (Tier 2 → Tier 1 treatment)
- Score 70-85: keep current tier, proceed
- Score < 70: demote (Tier 1 → Tier 2) or skip (Tier 2/3 → "skipped")
- Tier 3 leads → scored only if batch has spare capacity

### Implementation

**New file:** `internal/scout/sweep/classify.go`
- `ClassifyTier(lead)` — rule-based, returns tier 1/2/3
- `LoadDreamCompanies(path)` — reads `~/.soul-v2/dream-companies.json`

**New file:** `~/.soul-v2/dream-companies.json`
- Array of `{"name": "Anthropic", "domain": "anthropic.com"}` objects
- User editable, loaded at sweep time

**Modified:** `internal/scout/sweep/sweep.go`
- After `JobToLead()`, call `ClassifyTier()` to set `lead.Tier`
- After classification, queue Tier 1/2 for scoring

**Modified:** `internal/scout/store/` — add `tier` column to leads table
- Column: `tier INTEGER NOT NULL DEFAULT 3` (1=Tier 1, 2=Tier 2, 3=Tier 3)
- Add `"tier"` to `allowedLeadFields` for dynamic updates via PATCH
- Add `Tier int` field to Lead struct, include in `scanLead`
- Existing leads get default tier 3 (migration-safe)

**Tier promotion/demotion:**
- Tier changes from scoring are permanent until manually overridden
- If Tier 1 warm outreach falls back at Day 8, tier stays 1 but approach changes to Tier 2 behavior
- User can always override tier via `[Change Tier]` in Gate 1

---

## 3.2: Pipeline Stages (Job Pipeline)

### Current

```
discovered → applied → screening → interview → offer → joined
Terminal: joined, rejected, withdrawn
```

### Proposed

```
discovered → qualified → preparing → outreach-sent → applied → screening → interview → offer → joined
Terminal: joined, rejected, withdrawn, skipped
```

| Stage | Meaning | Who/What Triggers |
|---|---|---|
| `discovered` | Lead ingested from TheirStack. Tier auto-assigned. | Automated (sweep) |
| `qualified` | Match scored >70. Decision to pursue. | Automated (scoring) |
| `preparing` | AI generated tailored resume + cover letter. Ready for review. | Automated (AI tools) |
| `outreach-sent` | LinkedIn connection/message sent (Tier 1/2). Cadence starts. | Human (Gate 1 approval) |
| `applied` | Application submitted on career page/portal/via referral. | Human |
| `screening` | Recruiter/HM responded. In active process. | Human (update on response) |
| `interview` | Interview(s) scheduled or in progress. | Human |
| `offer` | Offer received. Negotiation phase. | Human |
| `joined` | Accepted and started. Terminal. | Human |
| `skipped` | Score <70, or manually decided not to pursue. Terminal. | Automated or Human |

**Stage skipping rules:**
- Tier 3: skip `outreach-sent` (discovered → qualified → preparing → applied)
- Referral path: can skip `preparing` (outreach-sent → applied via referrer)
- Any lead can be moved to `skipped` from any non-terminal stage

Note: Forward skipping is already supported by `ValidateTransition` — it checks `toIdx > fromIdx`, so jumping from `preparing` directly to `applied` (skipping `outreach-sent`) is valid with no code change to validation logic. The skip rules above are application-level guidance, not enforcement constraints.

### Implementation

**Modified:** `internal/scout/pipelines/pipelines.go`
- Update `"job"` pipeline stages to include new stages
- Add `"skipped"` to terminal states
- No changes needed to `ValidateTransition` — forward skips are inherently valid

**Modified:** `internal/scout/server/server.go`
- `handleRecordAction` and tool dispatch must call `pipelines.ValidateTransition` before allowing stage changes (currently bypassed)

---

## 3.3: AI-Driven Pipeline with Human Gates

### Automated Flow

**Phase 1: DISCOVER (fully automated, daily)**
- TheirStack sweep ingests new leads
- Tier auto-classified (rule-based, instant)
- Duplicates filtered by `theirstack_id` (primary, existing) + `company+title+location` (secondary, catches cross-source dupes)
- Leads stored as "discovered"

**Phase 2: QUALIFY (automated, Tier 1/2 only)**
- Tier 1/2 leads → Claude `resume_match` scoring
- Score > 70 → move to "qualified"
- Score < 70 → move to "skipped"
- Score > 85 on Tier 2 → promote to Tier 1 treatment
- Tier 3 leads → scored if spare capacity, else auto-qualified

**Phase 3: PREPARE (automated)**
- Qualified leads → Claude generates:
  - Tailored resume (keywords matched to JD)
  - Cover letter (company-specific)
  - Outreach draft (if Tier 1/2)
  - Salary estimate (market context)
- Lead moves to "preparing"

### Human Gates

**Gate 1: MORNING REVIEW (daily, ~30 min)**

Scout Actions tab shows today's batch. For each lead:
- Company name, tier, match score
- AI-tailored resume diff (what changed)
- Cover letter draft
- Outreach message draft (Tier 1/2)
- Salary range estimate
- Hiring manager name + LinkedIn (if available)

Actions: `[Approve]` `[Edit]` `[Skip]` `[Change Tier]`

**Phase 4: EXECUTE (human, per tier after Gate 1)**

- Tier 1 (approved): Send LinkedIn connection request → move to "outreach-sent"
- Tier 2 (approved): Apply on career page + LinkedIn outreach to HM → move to "applied"
- Tier 3 (approved): Apply on portal/career page → move to "applied"

**Phase 5: FOLLOW-UP (automated cadence + human gates)**

Lead-attached outreach (Tier 1/2):
- Day 0: Connection request sent
- Day 3: Accepted? → **Gate 2:** review follow-up draft. Not accepted? → engage with their content
- Day 5: Second touch (comment on post, re-approach)
- Day 8: No movement → **Gate 3:** fall back to Tier 2 approach?
- Day 14: No response → auto-mark "withdrawn"

Relationship outreach (not lead-attached):
- No cadence. No auto-demotion. Long game.
- Tracked in "referral" pipeline separately.

### Implementation

#### Artifact Storage

AI-generated artifacts (tailored resume, cover letter, outreach draft) are stored in a new `lead_artifacts` table:

```sql
CREATE TABLE lead_artifacts (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  lead_id INTEGER NOT NULL REFERENCES leads(id),
  type TEXT NOT NULL,         -- "resume", "cover_letter", "outreach_draft", "salary_estimate"
  content TEXT NOT NULL,      -- full generated text
  created_at TEXT NOT NULL DEFAULT (datetime('now'))
);
CREATE INDEX idx_lead_artifacts_lead ON lead_artifacts(lead_id);
```

"Resume diff" in Gate 1 = diff of `lead_artifacts.type='resume'` content against the baseline resume stored at `~/.soul-v2/resume-baseline.md` (user maintains this file).

New AI tool: `ResumeTailor(ctx, leadID)` in `internal/scout/ai/resume.go`
- Input: baseline resume + lead JD + technology slugs
- Output: tailored resume text with keywords matched to JD
- Stored in `lead_artifacts` with `type="resume"`

#### Follow-up Cadence Storage

Cadence state tracked via existing `next_action` and `next_date` fields on the Lead struct:
- Day 0 clock starts when lead enters `outreach-sent` stage
- `next_date` set to Day 3 automatically
- `next_action` set to `"check_connection_accepted"`
- On each cadence check, `next_date` and `next_action` advance per the cadence rules

Connection acceptance is external state (LinkedIn). User marks it manually via Gate 2, or the pipeline runner surfaces "Day 3 check: was connection accepted?" as a prompt.

Leads at Day 14 with no response auto-move to `withdrawn`. To recover a Day 15 response, user can manually create a new lead or reopen (add `reopened` as a valid transition from `withdrawn` to `screening`).

#### Background Pipeline Runner

**New package:** `internal/scout/pipeline/runner.go`

Single goroutine, polling-based:

```
Runner loop (every 5 minutes):
  1. QUALIFY: Find leads at "discovered" with tier 1/2 → score via resume_match
     - Batch up to 5 leads per cycle (rate limiting for OAuth token)
     - Score > 70 → advance to "qualified"
     - Score < 70 → advance to "skipped"

  2. PREPARE: Find leads at "qualified" → generate artifacts
     - Call ResumeTailor, CoverLetter, ColdOutreach (Tier 1/2), SalaryLookup
     - All artifacts stored in lead_artifacts table
     - Advance to "preparing"

  3. CADENCE: Find leads at "outreach-sent" with next_date <= now
     - Surface in Actions tab as "follow-up due"
     - Day 14 auto-transition to "withdrawn"

  4. STALE: Find leads at "preparing" older than 7 days with no Gate 1 action
     - Surface as "stale prepared leads" in Actions tab
     - After 14 days → auto-skip (prepared but never reviewed = not worth pursuing)
```

Concurrency: single goroutine, no locking needed (SQLite serializes writes). Integrates with existing `sweep.Scheduler` — runner starts after scheduler completes each sweep cycle.

Error handling: on Claude API error, lead stays at current stage. Runner retries on next cycle. After 3 consecutive failures for a lead, mark `next_action = "scoring_failed"` and surface for manual review.

#### Scoring Interaction with Sweep

The existing `RunSweep` scoring phase is replaced by the pipeline runner's QUALIFY phase. Sweep only does: fetch → ingest → classify tier. Scoring happens asynchronously in the runner, respecting tier priority (Tier 1 first, then Tier 2, then Tier 3 if capacity).

Remove the `Scorer` interface call from `RunSweep` to avoid double-scoring.

**Modified:** Scout frontend Actions tab
- "Ready for Review" batch view with approve/edit/skip/change-tier controls
- Shows artifact previews (resume diff, cover letter, outreach draft)
- "Follow-up Due" section for cadence alerts
- "Stale Leads" section for prepared-but-unreviewed leads

#### New Pipeline: Referral

Add to `internal/scout/pipelines/pipelines.go`:
```go
"referral": {
  Stages:   []string{"identified", "connected", "conversation", "referral-asked", "referred", "interviewing", "offer"},
  Terminal: []string{"offer", "declined", "no-response"},
}
```
This tracks relationship outreach NOT attached to specific job leads. No cadence, no auto-demotion.

---

## 3.4: Weekly Throughput Targets

### Volume (Aggressive, AI-Assisted)

| Metric | Weekly Target |
|---|---|
| Leads reviewed (Gate 1) | 20-30 |
| Qualified (score >70) | 12-15 |
| Prepared by AI | 8-10 |
| Applications submitted | 5-8 |
| Tier 1 warm approaches | 3 |
| Tier 2 HM outreaches | 4-5 |
| Follow-ups sent | All pending from prior weeks |

### Time Commitment

| Activity | Time | When |
|---|---|---|
| Morning batch review (Gate 1) | 30 min | Daily |
| Send outreach on LinkedIn | 20 min | Tue/Thu |
| Apply on career pages | 30 min | Thu |
| Review follow-ups (Gate 2) | 15 min | Wed |
| Stale lead decisions (Gate 3) | 15 min | Fri |
| Tier 1 coffee chat research + request | 20 min | Fri |
| **TOTAL** | **~3.5 hrs** | **Per week** |

---

## 3.5: Where to Apply (Decision Matrix)

### Decision Tree

```
Lead qualified (score > 70)
  │
  ├─ Has referral contact?
  │   └─ YES → Referrer submits internally. Do NOT apply on portal.
  │
  ├─ Tier 1 (no referral yet)?
  │   └─ Do NOT apply yet. Outreach first.
  │       → Wait for warm path (up to Day 8)
  │       → Day 8 fallback: apply on company career page
  │
  ├─ Tier 2?
  │   └─ Apply on company career page (same day)
  │      + LinkedIn outreach to HM (same day)
  │      ⚠ Never use LinkedIn Easy Apply
  │
  └─ Tier 3?
      └─ Apply on portal where found (speed matters)
         + Find CTO/VP Eng on LinkedIn if possible
```

### Career Page Discovery

TheirStack provides `final_url` (company career page) + `source_url` (portal where discovered).
Priority: `final_url` > `source_url`. Direct ATS submission > portal repost.

### What to Submit Per Tier

| Tier | Materials |
|---|---|
| Tier 1 | Tailored resume + cover letter (only after warm path exhausted) |
| Tier 2 | Tailored resume + cover letter + LinkedIn note to HM referencing the application |
| Tier 3 | Tailored resume (cover letter optional) |

### What to Avoid

- LinkedIn Easy Apply (3% response rate)
- Indeed/Glassdoor/Monster for senior AI roles (stale reposts)
- Generic resume (tailored gets 78% higher response)
- Applying on multiple portals for same company
- Applying before exhausting warm path for Tier 1
