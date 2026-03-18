# Individual Freelance Strategy — Design Spec

**Date:** 2026-03-18
**Status:** Approved
**Scope:** `docs/scout/freelance.md` — individual freelance work (you do the work). Team contracts and expert consulting are separate specs.

---

## Overview

Multi-channel freelance strategy with phased discovery: platform bootstrapping (Month 1-2) → networking conversion (Month 2-4) → inbound takes over (Month 4+). AI-driven proposal pipeline with scoring and human gate. 15-20 hrs/week commitment as serious parallel track alongside job search.

## Design Decisions

| Decision | Choice | Why |
|---|---|---|
| Experience level | Informal via network, no platform reviews | Need platform bootstrapping before premium channels |
| Primary services | AI/LLM app development + AI consulting/architecture | Highest-demand, highest-rate niche. Builder + advisor combo covers $50/hr gigs AND $200/hr consulting. |
| Secondary services | MLOps, full stack with AI | Take when offered, don't headline |
| Rate strategy | Split by channel | Competitive on platforms (build reviews), premium on vetted/direct (maximize income) |
| Discovery | Phased (platforms → networking → inbound) | Can't wait for inbound — need revenue signals from day one while slower channels compound |
| Commitment | 15-20 hrs/week (serious parallel track) | Real income, not just reviews. Could become full-time. |
| Automation | AI drafts for score > 80 only | Save Claude calls for high-quality matches. Lower scored gigs shown as "maybe" list. |

---

## 5.1: Service Positioning

### Primary Services

1. **AI/LLM Application Development** — RAG pipelines, AI agents, chatbots, LLM integrations. "I turn your AI prototype into a production system."
2. **AI Strategy & Architecture Consulting** — Evaluate vendor options, design AI systems, tech audits. "I help you make the right AI decisions before you build."

### Secondary Services

3. AI Infrastructure / MLOps
4. Full Stack with AI features

### Niche Statement (use across all profiles)

"Senior AI Engineer (8+ yrs) — I build production LLM applications and advise on AI architecture. Specializing in RAG, agents, and enterprise AI systems."

### Profile Setup

**Upwork:**
- Title: "Senior AI/LLM Engineer | RAG, Agents, Enterprise AI | 8+ Yrs"
- Overview: niche statement + 3 bullet proof points + "available now"
- Skills: Python, LLM, RAG, AI, Machine Learning, NLP, Go, React
- Portfolio: 3-4 project descriptions from past network work (anonymize client names if needed)

**Contra:** Same positioning, portfolio-focused layout, zero commission.

---

## 5.2: Rate Strategy

### Rate Tiers by Channel

| Channel | Rate | Commission | Effective Rate |
|---|---|---|---|
| **Upwork (first 5 projects)** | $35-45/hr | 20% → 10% | $28-40/hr |
| **Upwork (after 10+ reviews)** | $60-80/hr | 10% → 5% | $54-76/hr |
| **Freelancer** | $35-50/hr | 10% | $31-45/hr |
| **Contra** | $50-80/hr | 0% | $50-80/hr |
| **Toptal** | $100-180/hr | 0% (client pays) | $100-180/hr |
| **Flexiple** | $80-150/hr | 0% (client pays) | $80-150/hr |
| **Gun.io / Arc.dev** | $80-150/hr | varies | $70-140/hr |
| **Direct clients** | $150-250/hr | 0% | $150-250/hr |

### Project-Based Pricing

| Size | Range | Payment Model |
|---|---|---|
| Small (< 20 hrs) | $1K-3K | Fixed bid |
| Medium (20-80 hrs) | $3K-10K | Milestone-based |
| Large (80+ hrs) | $10K-30K+ | Monthly retainer |

### Pricing Rules

- Never go below $35/hr (signals incompetence, attracts bad clients)
- Project bids: estimated hours × hourly rate × 1.3 (30% scope creep buffer)
- Milestone-based for projects > $2K (protects both sides)
- Retainer deals: discount 10-15% vs hourly (guaranteed income worth the discount)

### Rate Progression

| Period | Platform Rate | Vetted Rate | Direct Rate |
|---|---|---|---|
| Month 1-2 | $35-45/hr | N/A (pending) | N/A |
| Month 3-4 | $60-80/hr | $100+/hr | N/A |
| Month 5-6 | $80/hr | $100-150/hr | $150+/hr |
| Month 6+ | Cherry-pick only | $100-180/hr | $150-250/hr |

---

## 5.3: Phased Discovery Strategy

### Phase 1: Platform Bootstrapping (Month 1-2)

**Goal:** 5-10 reviews + Rising Talent / Top Rated badge

**Platforms (priority order):** Upwork → Contra → Freelancer

**Daily routine:**
- Scout monitors Upwork for relevant gigs
- AI scores and drafts proposals for top matches (score > 80)
- You review 3-5 proposals per day (~15 min)

**Gig selection criteria:**

| Accept | Skip |
|---|---|
| AI/LLM projects (RAG, agents, chatbots) | "Build me ChatGPT for $100" |
| Fixed-price $500-3K (quick delivery, fast review) | Clients with no payment history |
| Hourly with clear scope | Vague scope, no budget |
| Client 4+ stars, payment verified | Projects requiring 40+ hrs/week |

**Review acceleration:**
- Ask for review immediately after each project
- Deliver slightly over-scope (small bonus feature)
- Respond within 2 hours during active projects
- Target: 5 reviews in first 6-8 weeks

### Phase 2: Networking Conversion (Month 2-4)

**Goal:** First direct clients from networking pipeline

**Sources:**
- Section 4 founder/CTO contacts at ACTIVE+ warmth → freelance angle
- Vetted networks (Toptal/Flexiple — applied in Phase 1, accepted by now)
- Cold email outreach (Section 4 email playbook) → freelance angle
- Peer engineers → "Know anyone who needs AI help?"

**Transition:** Platform proposals reduce to 2-3/day. Upwork reviews now support credibility everywhere.

### Phase 3: Inbound Takes Over (Month 4+)

**Goal:** Clients find you

**Sources:**
- Content (Section 6): "I built X" LinkedIn posts → DMs → projects
- LinkedIn Services page: inbound from LinkedIn search
- Referrals: past clients, network contacts, community members
- Vetted networks: ongoing matched gigs

**Transition:** Platform proposals reduce to 1-2/week (maintain presence, cherry-pick premium gigs only). Direct clients become primary channel at highest rates.

### Revenue Trajectory (estimated)

| Period | Monthly Revenue | Primary Source |
|---|---|---|
| Month 1-2 | ₹50K-1.5L | Platform gigs (lower rates) |
| Month 3-4 | ₹1.5L-3L | Mix of platform + networking |
| Month 5-6 | ₹3L-5L | Direct clients + vetted networks |
| Month 6+ | ₹5L+ | Premium rates, select clients |

---

## 5.4: AI-Driven Freelance Pipeline

### Discovery

**Upwork monitoring keywords:**
```
[LLM, RAG, AI agent, chatbot, NLP, machine learning,
 AI architect, AI consultant, GPT, Claude, Python AI,
 AI development, AI integration, AI strategy]
```

**Filters:**
- Budget > $500
- Client rating > 4 stars
- Payment verified
- Posted < 24 hours (speed advantage)

**Additional sources:**
- Section 4 networking contacts (founders/CTOs at ACTIVE+)
- Inbound DMs/emails (Phase 3+)
- Vetted network matches

All leads stored in Scout with `pipeline = "freelance"`.

### Scoring (Claude via OAuth)

Score 0-100 based on:
- Skill match (your profile vs requirements)
- Budget alignment (rate tier for this channel)
- Scope clarity (vague = lower score)
- Client quality (rating, payment history, past reviews given)
- Time fit (hours/week vs your current availability)

| Score | Action |
|---|---|
| > 80 | AI drafts proposal, queued for Gate F1 |
| 60-80 | Shown in "maybe" list, no draft |
| < 60 | Auto-skipped |

### Proposal Generation (Claude via OAuth)

Platform-specific formatting:
- **Upwork:** Short, punchy, under 200 words. Opens with specific reference to their project. Shows relevant experience. Proposes approach in 2-3 sentences. States rate and availability.
- **Contra:** Portfolio-focused. Links to relevant past work. Clean and professional.
- **Freelancer:** Competitive angle. Emphasize value and fast delivery. Specific timeline.
- **Email (direct):** Value-first. Identify specific gap. No pitch in first email.

Existing AI tool: `ProposalGen(ctx, leadID, platform)` in `internal/scout/ai/proposal.go` already supports upwork, freelancer, and general formats.

### Pipeline Stages

```
found → proposal-ready → proposal-sent → shortlisted → awarded → delivering → completed
Terminal: completed, lost, withdrawn
```

New stage vs current: `proposal-ready` added between `found` and `proposal-sent` (AI has drafted proposal, waiting for Gate F1 review).

### Human Gates

**Gate F1: PROPOSAL REVIEW (daily, ~15 min)**

Scout Actions shows freelance batch: "3 proposals ready, 5 in maybe list"

For each scored gig:
- Project title, client, budget, match score
- AI-drafted proposal
- Client history (past hires, reviews given, spend)
- Estimated hours and your rate for this channel

Actions: `[Send]` `[Edit & Send]` `[Skip]` `[Move to Maybe]`

**Gate F2: POST-PROJECT REVIEW (per project completion)**

Triggered when a gig moves to "completed":
- Request review from client?
- Ask for testimonial for portfolio?
- Upsell more work (follow-on project)?
- Add to portfolio/case studies?
- Ask for referral to other potential clients?

### Follow-up Cadence

| Event | Timeline | Action |
|---|---|---|
| Proposal sent, no response | Day 3 | AI drafts brief follow-up ("just checking if you had questions") |
| Proposal sent, no response | Day 7 | Mark as "lost", move on |
| Shortlisted, client went silent | Day 5 | AI drafts gentle nudge |
| Shortlisted, client went silent | Day 10 | Mark as "lost" |
| Project completed | Day 0 | Request review immediately |
| Project completed | Day 7 | If no review, gentle reminder |
| Project completed | Day 14 | Send referral request |

### Implementation

**Modified:** `internal/scout/pipelines/pipelines.go`
- Add `proposal-ready` stage to freelance pipeline:
  ```
  "freelance": {
    Stages: []string{"found", "proposal-ready", "proposal-sent", "shortlisted", "awarded", "delivering", "completed"},
    Terminal: []string{"completed", "lost", "withdrawn"},
  }
  ```

**New AI tool:** `internal/scout/ai/freelance_score.go`
- `FreelanceScore(ctx, leadID)` — scores freelance gig on 5 criteria, returns 0-100
- Different from `ResumeMatch` — evaluates gig quality and fit, not JD-to-resume match

**Modified:** `internal/scout/ai/proposal.go`
- Add `"contra"` to `validPlatforms` map
- Add Contra-specific proposal format

**Modified:** `internal/scout/pipeline/runner.go`
- Add freelance pipeline phases alongside job pipeline phases:
  - DISCOVER: monitor Upwork keywords (via API/RSS or manual input)
  - SCORE: score gigs with `FreelanceScore`
  - DRAFT: generate proposals for score > 80 via `ProposalGen`
  - CADENCE: follow-up reminders for proposal-sent and completed leads

**Modified:** Scout frontend Actions tab
- "Freelance Proposals" section (Gate F1 batch)
- "Post-Project Actions" section (Gate F2 prompts)
- "Freelance Metrics" in Analytics tab (revenue, proposals, conversion)

---

## 5.5: Weekly Throughput

### Phase 1 (Month 1-2)

| Activity | Volume/week | Your Time |
|---|---|---|
| Review proposals (Gate F1) | 15-25 scored, 3-5 sent | 1.5 hrs/week |
| Client conversations | 2-3/week | 30 min/week |
| Active delivery | 1-2 concurrent gigs | 12-15 hrs/week |
| Post-project (Gate F2) | As needed | 15 min/project |
| **TOTAL** | | **~15-18 hrs/week** |

### Phase 3 (Month 5+)

| Activity | Volume/week | Your Time |
|---|---|---|
| Platform proposals (cherry-pick) | 1-2/week | 15 min/week |
| Inbound leads (respond, qualify) | 2-4/week | 30 min/week |
| Active delivery | 2-3 gigs or 1 retainer | 15-20 hrs/week |
| **TOTAL** | | **~16-21 hrs/week** |

### Combined Time Budget (All Strategies)

| Activity | Month 1-2 | Month 5+ |
|---|---|---|
| Job Applications (Section 3) | 3.5 hrs | 2 hrs |
| Networking (Section 4) | 50 min | 50 min |
| Freelance delivery (Section 5) | 15-18 hrs | 16-21 hrs |
| Content (Section 6) | 2-3 hrs | 2-3 hrs |
| **TOTAL** | **~22-26 hrs/week** | **~21-27 hrs/week** |

Note: Freelance delivery (12-20 hrs) is PAID WORK, not job search overhead. Actual unpaid job search time: ~7 hrs/week.

### Metrics

**Platform:** Proposals sent/week, proposal-to-shortlist rate (target 20%+), shortlist-to-award rate (target 30%+), reviews accumulated (target 5 in 6 weeks), average rating (target 4.8+)

**Client:** Revenue/month, active gig count, repeat client rate (target 30%+ by month 6), average effective hourly rate, client source breakdown

**Pipeline:** Gigs discovered/week, score > 80 count, found → awarded conversion time (target < 7 days), revenue per source

---

## Prerequisites

Implementation depends on (from job application spec):
- Pipeline runner package (`internal/scout/runner/`) — see note below on naming
- `lead_artifacts` table (for storing generated proposals)
- `ValidateTransition` enforcement in server handlers
- `allowedLeadFields` updates

Implementation depends on (from networking spec):
- Networking pipeline registered (for Phase 2 contact sources)
- `interactions` table (for tracking freelance client relationships)

## Implementation Notes

### Pipeline runner package naming

The existing `internal/scout/pipelines/` package holds type definitions. The new runner should use a distinct name to avoid confusion: `internal/scout/runner/` (not `internal/scout/pipeline/`). All three specs (job, networking, freelance) share this package. Job spec creates `runner.go`, networking spec extends with `networking.go`, freelance spec extends with `freelance.go`.

### Stage mismatch fix

`JobToLead` in `sweep/theirstack.go` hardcodes `Stage: "discovered"`. But the freelance pipeline starts at `"found"`. The runner's DISCOVER phase must set `Stage` to the pipeline's default stage (`pipelines.DefaultStage(pipelineType)`) after `InferPipeline` determines the pipeline type. This fixes both freelance (`"found"`) and any future pipeline with a non-`"discovered"` initial stage.

### Follow-up cadence storage

Uses same mechanism as job application spec: `next_action` and `next_date` fields on Lead struct.

| Event | next_action | next_date |
|---|---|---|
| Proposal sent | `"check_proposal_response"` | Day 3 |
| Day 3 no response | `"send_followup"` | Day 7 |
| Day 7 no response | `"mark_lost"` | immediate |
| Project completed | `"request_review"` | Day 0 |
| Review received | `"request_referral"` | Day 14 |

### Gate F2 integration

Gate F2 is surfaced in the Actions tab by querying: leads with `pipeline = "freelance"` AND `stage = "completed"` AND no `lead_artifact` of type `"post_project_review"`. This is a display query, not a runner phase. The runner does NOT automate Gate F2 — it's purely human-driven.

### "Maybe" list lifecycle

Gigs scoring 60-80 shown in "maybe" list. Lifecycle:
- After 7 days with no user action → auto-removed (gig is stale)
- User can promote to proposal queue (triggers AI draft)
- User can dismiss (removed immediately)

### Deduplication for non-TheirStack leads

Freelance leads from networking contacts or manual input lack `theirstack_id`. Dedup uses: `company + title + source` as secondary key. If a platform-discovered gig matches a networking-sourced lead, the platform lead is merged into the existing record (adds platform URL as additional source).

## Relationship to Other Sections

- **Section 3 (Jobs):** Freelance and job search run in parallel. If job offer comes, wind down freelance gracefully (finish active gigs, stop new proposals).
- **Section 4 (Networking):** Founder/CTO contacts at ACTIVE+ warmth are freelance lead sources. Same contact can appear in both networking and freelance pipelines.
- **Section 6 (Content):** "I built X" posts drive freelance inbound from Month 4+. Content compounding = freelance lead generation.
- **Consulting (`docs/scout/consulting.md`):** Expert calls (GLG/Guidepoint) are separate. Different pipeline, rate, platforms.
- **Contracts (`docs/scout/contracts.md`):** Team contracts are separate. Freelance = you do the work. Contracts = your team does the work.
