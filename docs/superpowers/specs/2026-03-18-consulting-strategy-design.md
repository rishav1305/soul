# Expert Consulting Strategy — Design Spec

**Date:** 2026-03-18
**Status:** Approved
**Scope:** `docs/scout/consulting.md` — expert consulting (you give advice). Individual freelance and team contracts are separate specs.

---

## Overview

Three-tier consulting strategy: expert network calls (entry) → advisory engagements (grows from calls) → project consulting (grows from advisory). Mass-apply to 12 expert networks, AI assists full pipeline including call prep, follow-up, advisory proposals, and proactive lead identification. Each tier feeds the next, and consulting feeds the contracts pipeline ("can your team build this?"). Scale based on demand, no cap.

## Design Decisions

| Decision | Choice | Why |
|---|---|---|
| Consulting types | All three tiers (calls, advisory, project) | Each feeds the next. Expert calls are top of funnel. |
| Experience | Starting from zero | Need onboarding phase — mass-apply to networks first |
| Domains | AI/LLM production + strategy + market landscape + verticals (legal, healthcare, sales, e-commerce) | Wide expert surface = more matching opportunities across client types |
| Time commitment | Start minimal, scale based on demand, no cap | Don't over-commit upfront. Let market pull dictate. |
| Automation | Full assist (prep, track, generate leads, draft proposals) | Long-term relationship pipeline benefits from AI keeping everything warm |

---

## 5E.1: Three Consulting Tiers

### Tier 1: Expert Network Calls (Entry Point)

- **What:** 1-hour paid calls. Investors, analysts, consultants want your opinion.
- **Effort:** Show up prepared, answer questions.
- **Rate:** ₹10-30K/hr ($120-350/hr)
- **How:** Apply to networks → get accepted → they match you to calls
- **Volume:** 2-8 calls/month (depends on demand)

### Tier 2: Advisory Engagements (Grows from Tier 1)

- **What:** Ongoing advisor. "Help us figure out our AI strategy." Monthly check-ins.
- **Effort:** 2-5 hrs/month per client
- **Rate:** ₹50K-2L/month per advisory client
- **How:** Expert call goes well → client asks for more → advisory proposal. OR networking contact → advisory pitch.
- **Volume:** 1-3 concurrent advisory clients

### Tier 3: Project Consulting (Grows from Tier 1 + 2)

- **What:** Scoped engagement. AI audit, architecture review, vendor evaluation, technical roadmap. Deliverable is a document/presentation, not code.
- **Effort:** 10-40 hrs per project
- **Rate:** ₹2-10L per project
- **How:** Advisory client needs deeper work → scoped project. OR direct inbound from content/networking.
- **Volume:** 1-2 projects/quarter

### Progression Funnel

```
Expert call → "that was valuable" → advisory retainer
Advisory → "we need you to go deeper" → project consulting
Project consulting → "can your team build this?" → contracts pipeline (docs/scout/contracts.md)
```

Each tier feeds the next. Consulting is a lead generation engine for the entire Scout system.

---

## 5E.2: Onboarding — Getting Accepted (Week 1-4)

### Week 1-2: Mass Application

Apply to all 12 expert networks simultaneously. AI generates tailored applications per network.

**Per application:**
- Domain expertise description (tailored per network's client base)
- Industry verticals covered
- Years of experience + credentials
- LinkedIn profile URL
- Availability

| Network | Client Base | Your Angle |
|---|---|---|
| **GLG** | Investment firms | AI market intelligence, competitive landscape |
| **Guidepoint** | Consultancies | AI strategy, build-vs-buy decisions |
| **AlphaSights** | PE/VC firms | AI due diligence, company evaluation |
| **Third Bridge** | Analysts | AI competitive landscape, market sizing |
| **Expert Collective** | Consulting firms | Enterprise AI implementation |
| **Tegus** | Investors | AI company technical evaluation |
| **Silverlight** | Institutional | AI market sizing, trend analysis |
| **Zintro** | Mixed | All domains — broad coverage |
| **Maven** | Mixed | Surveys + calls + projects |
| **Atheneum** | Europe-heavy | Global AI perspective, cross-market |
| **Catalant** | Enterprise | Project consulting, fractional advisory |
| **Business Talent Group** | Enterprise | Fractional CTO, advisory |

**Expected:** 6-8 acceptances out of 12 within 4-6 weeks.

### Week 3-4: Profile Optimization

Once accepted, optimize expert profile per network:
- Specific topics, not generic "AI". Examples:
  - "Production RAG architectures for enterprise"
  - "LLM vendor evaluation (OpenAI vs Anthropic vs open-source)"
  - "AI in legal tech: contract analysis and compliance automation"
  - "Healthcare AI: clinical NLP and HIPAA-compliant systems"
  - "E-commerce AI: recommendation engines and personalization at scale"
  - "Sales AI: conversational AI and CRM intelligence"
- Set availability windows (mornings IST = afternoon US East)
- Set minimum rate: ₹10K/hr (signals seniority)
- Preferred call length: 45-60 min

### Month 2+: First Calls Start

Networks match you to opportunities via email. Accept or decline per call.

### Implementation

**New AI tool:** `internal/scout/ai/expert_application.go`
- `ExpertApplication(ctx, networkName, networkFocus)` — generates tailored expertise description per network
- Sync execution, short prompt

---

## 5E.3: Expert Domains

### Horizontal (AI/Tech — applies across industries)

| Domain | Topics You Cover |
|---|---|
| AI/LLM in Production | Deployment, scaling, RAG, agents, evaluation, cost optimization, tool-use, prompt engineering |
| AI Strategy & Architecture | Build vs buy, vendor selection (OpenAI/Anthropic/open-source), architecture patterns, team structure |
| AI Market & Landscape | Competitive intelligence, what's real vs hype, market sizing, AI startup evaluation |

### Vertical (Industry-Specific AI)

| Vertical | Topics You Cover |
|---|---|
| Legal | Contract analysis, document review, compliance automation, legal LLM applications |
| Healthcare | Clinical NLP, diagnostics support, patient data, HIPAA considerations, medical AI evaluation |
| Sales | Lead scoring, conversational AI, CRM intelligence, forecasting, sales automation |
| E-commerce | Recommendation engines, search relevance, pricing optimization, personalization, product discovery |

---

## 5E.4: AI-Driven Consulting Pipeline

### Opportunity Sources

**Expert network calls (passive — they come to you):**
- Networks send call requests via email
- Log in Scout as consulting lead
- AI generates prep brief

**TheirStack intelligence (proactive — you identify them):**
- Companies with "AI confusion" signals:
  - Hiring 5+ AI roles across different functions (no clear strategy, building piecemeal)
  - Job descriptions mention "AI transformation", "AI roadmap", "AI strategy"
  - Recently funded (Series A-B) + no AI team lead (have money, need direction)
- Flagged as advisory/consulting targets

**Cross-pipeline conversion:**
- Freelance client asks strategic questions → advisory upsell
- Contract client needs roadmap before building → project consulting
- Networking contact discusses AI challenges → advisory lead

### Pipeline Stages

Uses existing `consulting` pipeline from `pipelines.go`:

```
lead → discovery-call → proposal-sent → negotiating → engaged → delivered
Terminal: delivered, lost, declined
```

| Stage | Consulting Meaning |
|---|---|
| `lead` | Opportunity identified (network call request, TheirStack signal, cross-pipeline flag) |
| `discovery-call` | Expert call completed OR initial conversation held |
| `proposal-sent` | Advisory/project proposal sent |
| `negotiating` | Terms discussion in progress |
| `engaged` | Active advisory or project consulting engagement |
| `delivered` | Engagement completed. Terminal. |

### AI Tools

| Tool | Execution | Input | Output | Storage |
|---|---|---|---|---|
| `ExpertApplication` | Sync | Network name + focus area | Tailored application text | N/A (one-time use) |
| `CallPrepBrief` | Sync | Lead data (company, topic, context) | Prep brief: company background, likely questions, your relevant experience, key data points | `lead_artifacts` type=`"call_prep"` |
| `ConsultingFollowUp` | Sync | Lead + call notes | Follow-up email referencing specific discussion points + resources mentioned | `lead_artifacts` type=`"consulting_followup"` |
| `AdvisoryProposal` | Sync | Lead + call notes + signals | Advisory proposal: problem diagnosis, scope, monthly hours, rate, your experience | `lead_artifacts` type=`"advisory_proposal"` |
| `ProjectProposal` | Sync | Lead + advisory history + project scope | Project proposal: audit scope, deliverables, timeline, fixed fee | `lead_artifacts` type=`"consulting_proposal"` |
| `UpsellEvaluator` | Sync | Call notes + lead context | Assessment: is there an upsell to advisory/project/contracts? + draft proposal if yes | `lead_artifacts` type=`"upsell_evaluation"` |

### Human Gates

**Gate E1: CONSULTING REVIEW (as calls come in + weekly for proactive leads)**

For expert calls:
- Accept/decline the call
- Review prep brief (10 min before call)

For advisory/project proposals:
- Review AI-drafted proposal
- Edit and send (or skip)
- ~15 min per proposal

Actions: `[Accept Call]` `[Decline]` `[Send Proposal]` `[Edit & Send]` `[Skip]`

**Gate E2: POST-ENGAGEMENT (per completion)**

After each expert call:
- Input call notes into Scout
- Review AI-generated follow-up email → send
- Review AI upsell evaluation → pursue advisory if flagged

After advisory/project completion:
- Request testimonial
- Convert to next tier? (advisory → project, project → contracts)
- Ask for referrals
- Add to case study portfolio

Actions: `[Send Follow-up]` `[Pursue Upsell]` `[Request Testimonial]` `[Skip]`

### Follow-up Cadence

| Event | next_action | next_date |
|---|---|---|
| Call completed | `"send_followup"` | Day 0 (immediate) |
| Follow-up sent | `"evaluate_upsell"` | Day 3 |
| Upsell flagged, proposal sent | `"check_proposal_response"` | Day 5 |
| No response to proposal | `"send_reminder"` | Day 10 |
| No response to reminder | `"move_dormant"` | Day 7 |
| Advisory session completed | `"schedule_next_session"` | Day 25 (monthly cycle) |
| Project delivered | `"request_testimonial"` | Day 2 |
| Testimonial received | `"request_referral"` | Day 7 |

### Implementation

**New AI tools:** `internal/scout/ai/call_prep.go`, `internal/scout/ai/expert_application.go`, `internal/scout/ai/consulting_followup.go`, `internal/scout/ai/advisory_proposal.go`, `internal/scout/ai/project_proposal.go`, `internal/scout/ai/upsell_evaluator.go`

**Modified:** `internal/scout/runner/` — add consulting pipeline phases:
- IDENTIFY: monitor for cross-pipeline upsell signals + TheirStack "AI confusion" filter
- PREPARE: generate call prep briefs for upcoming calls
- CADENCE: follow-up reminders, advisory session scheduling
- POST-ENGAGE: surface Gate E2 items

**Modified:** Scout frontend Actions tab:
- "Consulting Calls" section (upcoming calls with prep briefs)
- "Consulting Proposals" section (advisory/project proposals for review)
- "Post-Call Actions" section (follow-ups and upsell evaluations)

---

## 5E.5: Revenue & Time

### Revenue by Tier

**Tier 1 — Expert Calls:**

| Period | Calls/month | Rate | Revenue |
|---|---|---|---|
| Month 1-2 | 0 | — | ₹0 (onboarding) |
| Month 3-4 | 2-4 | ₹10-15K/hr | ₹20K-60K |
| Month 5+ | 4-6 | ₹15-25K/hr | ₹60K-1.5L |
| Month 8+ | 4-8 | ₹20-30K/hr | ₹80K-2.4L |

**Tier 2 — Advisory:**

| Period | Clients | Rate | Revenue |
|---|---|---|---|
| Month 1-4 | 0 | — | ₹0 |
| Month 5-6 | 1 | ₹50K-1L/mo | ₹50K-1L |
| Month 7+ | 1-2 | ₹75K-1.5L/mo | ₹75K-3L |
| Month 12+ | 2-3 | ₹1-2L/mo | ₹2-6L |

**Tier 3 — Project Consulting:**

| Period | Projects/quarter | Fee | Quarterly Revenue |
|---|---|---|---|
| Month 1-6 | 0 | — | ₹0 |
| Month 7-9 | 1 | ₹2-5L | ₹2-5L |
| Month 10+ | 1-2 | ₹3-10L | ₹3-20L |

### Combined Consulting Revenue

| Period | Monthly | Source |
|---|---|---|
| Month 1-2 | ₹0 | Onboarding |
| Month 3-4 | ₹20K-60K | Expert calls only |
| Month 5-6 | ₹70K-2.5L | Calls + first advisory |
| Month 7-9 | ₹1.5L-5L | Calls + advisory + first project |
| Month 12+ | ₹3L-10L | All three tiers active |

### Time Commitment

| Activity | Hours/month |
|---|---|
| Expert calls | 4-8 hrs |
| Call prep (AI-assisted) | 1-2 hrs |
| Follow-up emails | 30 min |
| Advisory sessions | 2-5 hrs |
| Advisory prep | 1-2 hrs |
| Project consulting | 10-40 hrs (episodic) |
| Pipeline review (Gates) | 1-2 hrs |
| **TOTAL (no active project)** | **~10-18 hrs/month (~2.5-4.5 hrs/week)** |
| **TOTAL (with active project)** | **~20-55 hrs/month** |

### Full Revenue Picture (All Streams, Month 6+)

| Stream | Monthly Revenue | Your Time |
|---|---|---|
| Freelance | ₹3-5L | 15-18 hrs/week |
| Team contracts | ₹3-8L | 5-7 hrs/week |
| Expert consulting | ₹1.5-5L | 2.5-4.5 hrs/week |
| **TOTAL** | **₹7.5-18L/month** | **23-30 hrs/week** |
| | **(~₹90L-2.2Cr/yr)** | |

Job search pipeline runs in parallel (~7 hrs/week unpaid). If full-time job is accepted, consulting can continue as side income — expert calls and advisory are low-commitment and don't conflict with employment.

---

## Prerequisites

**Blocking:**
1. Pipeline runner (`internal/scout/runner/`) — created by job application spec
2. `lead_artifacts` table — created by job application spec. Consulting uses types: `"call_prep"`, `"consulting_followup"`, `"advisory_proposal"`, `"consulting_proposal"`, `"upsell_evaluation"`
3. `ValidateTransition` enforcement in server handlers

**Non-blocking:**
- Networking pipeline — cross-pipeline upsell detection benefits from networking contacts. Falls back to TheirStack-only if not yet implemented.
- Freelance pipeline — freelance client upsells to advisory. Works manually without automated detection.

**Already exists:**
- `consulting` pipeline in `pipelines.go`

## Relationship to Other Specs

- **Freelance (`docs/scout/freelance.md`):** Freelance clients asking strategic questions = advisory upsell. `UpsellEvaluator` monitors for this.
- **Contracts (`docs/scout/contracts.md`):** Project consulting → "can your team build this?" = warm handoff to contracts pipeline. Consulting is a lead gen engine for contracts.
- **Networking (`docs/scout/networking.md`):** Networking contacts discussing AI challenges = advisory leads. Expert calls build your network (each call = new contact in networking pipeline).
- **Jobs (`docs/scout/jobs.md`):** If job offer accepted, expert calls + advisory continue as side income. No conflict.
- **Content (Section 6):** "How I evaluated AI vendors for Company X" posts drive consulting inbound. Deep analysis content positions you as an advisor, not just a builder.
