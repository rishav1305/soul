# Team & Company Contracts Strategy — Design Spec

**Date:** 2026-03-18
**Status:** Approved
**Scope:** `docs/scout/contracts.md` — team/company contract work (your team does the work). Individual freelance and expert consulting are separate specs.

---

## Overview

Freelance-upsell-first contracts strategy, scaling from small (₹1-5L/month) to large (₹15L+/month) deals. Three engagement models: staff augmentation, project delivery, and dedicated team retainers. Team assembled per-project from network + freelancer pool. AI assists full sales pipeline: identification, pitch generation, SOW drafting, follow-up, and case study creation. Three human gates: opportunity review, deal decision, and post-contract actions.

## Design Decisions

| Decision | Choice | Why |
|---|---|---|
| Team model | Solo + assemble per-project from network (no bench) | Zero overhead until contract signed. No payroll risk. |
| Contract types | Staff aug + project delivery + retainer (all three) | Different clients need different models. Flexibility wins deals. |
| Entity | Company entity for large contracts, sole proprietor for small | Flexible invoicing based on deal size and client requirements. |
| Deal size | Start small (₹1-5L), scale to medium (₹5-15L), then large (₹15L+) | Build track record before going after enterprise deals. |
| Discovery | Phased: freelance upsell → targeted outreach → B2B inbound | Warmest leads first (existing clients), coldest last (directory inbound). |
| Automation | Full pipeline assist (identify → pitch → SOW → follow-up → case study) | Long sales cycle needs AI to keep pipeline moving without manual tracking. |

---

## 5C.1: Contract Types & Positioning

### Three Engagement Models

| Model | What | Billing | Duration | Your Role |
|---|---|---|---|---|
| **Staff Augmentation** | Client has team, needs more hands. You provide 1-3 engineers embedded in their team. | Per-person monthly | 3-12 months | Resource provider + QA |
| **Project Delivery** | Client has a problem. Your team delivers end-to-end. You scope, manage, own quality. | Milestone or fixed-price | 1-6 months | Tech lead + PM + delivery |
| **Retainer / Dedicated Team** | Client hires your team ongoing. Mix of projects, maintenance, support. You're their AI department. | Monthly retainer | 6+ months, open-ended | Fractional CTO / AI lead |

### Positioning

"We build and run AI systems for companies that don't have an in-house AI team — or need to scale one fast."

---

## 5C.2: Team Assembly Model

### Per-Project Assembly (No Bench)

| Role Needed | Source | Priority |
|---|---|---|
| Backend / AI eng | Your network contacts | First choice (trusted, known quality) |
| Frontend eng | Your network contacts | First choice |
| MLOps / Infra | Your network contacts | First choice |
| Any gap | Upwork freelancers (you've built reviews, know what good looks like) | Fill gaps |
| Junior / support | Upwork, Freelancer (lower rate) | Cost-effective for support work |

You are always the tech lead / AI architect. Client sees a unified team; you manage internally.

### Margin Model

| Role | Client Pays | You Pay Team | Your Margin |
|---|---|---|---|
| Junior eng | ₹1-1.5L/mo ($30-40/hr) | ₹40-60K/mo ($15-20/hr) | ₹60-90K/mo |
| Mid eng | ₹1.5-2.5L/mo ($45-65/hr) | ₹70-1.2L/mo ($25-40/hr) | ₹80-1.3L/mo |
| Senior eng | ₹2.5-4L/mo ($70-100/hr) | ₹1.2-2L/mo ($40-60/hr) | ₹1.3-2L/mo |
| You (lead/architect) | ₹4-6L/mo ($100-150/hr) | N/A | Full rate |

Margin: 40-60% per team member + your own billing at full rate.

### Your Role Evolves

| Phase | Your Split |
|---|---|
| Month 1-3 | 70% writing code + 30% managing |
| Month 3-6 | 50% architecting + 50% managing |
| Month 6+ | 30% architecting + 70% managing |
| Month 12+ | Selling + managing. Team delivers. |

---

## 5C.3: Phased Discovery & Sales

### Phase 1: Freelance-to-Team Conversion (Month 1-3)

**Goal:** First 1-2 team contracts from existing freelance clients.

**How it happens:** Individual freelance gig goes well → client needs more scope → you propose your team.

**Upsell triggers (AI detects and surfaces):**
- Client asks about features beyond original scope
- Client mentions they're hiring for the same role
- Project scope grows beyond solo capacity
- Client satisfaction is high (positive messages, repeat requests)

**AI generates:**
- Team proposal: "Here's how a 2-3 person team could handle your full AI roadmap"
- SOW draft based on expanded scope
- Rate comparison: solo vs team engagement

**Target:** Convert 1 in 5 freelance clients.

**Parallel setup:**
- Register on Clutch, GoodFirms, TopDevelopers (takes time to build profile)
- Set up company LinkedIn page
- Apply to staff aug platforms (Uplers, Crewmate, Wisemonk)
- Collect testimonials from freelance clients

### Phase 2: Targeted Outreach with Intelligence (Month 3-6)

**Goal:** Win contracts from companies that need a team.

**TheirStack intelligence filter:**
- Companies hiring 3+ roles matching AI keywords simultaneously
- Company size 50-500 employees
- Series A-C funded
- India-friendly or remote

**AI generates company-specific pitch (`CompanyPitch` tool):**
1. Company research — what they do, recent news, growth signals
2. Pain points — inferred from open job descriptions ("you're hiring 3 AI engineers; that takes 6 months. Our team can start in 2 weeks.")
3. Proposed engagement — staff aug / project / retainer
4. Relevant case studies — matched from Phase 1 portfolio
5. Pricing — rate range for proposed team composition

**Outreach sequence:**
- Day 0: LinkedIn connect to CTO/VP Eng (research angle, not pitch)
- Day 3: If accepted → value message (insight about their tech/hiring)
- Day 7: Share relevant case study or analysis
- Day 10: Pitch — "I noticed you're hiring 3 AI engineers. We help companies like yours move faster — 15 min chat?"
- Day 14: Follow up once
- Day 21: No response → dormant, re-engage in 30 days

**Volume:** 3-5 new company outreach sequences/week. AI drafts every message. Review at Gate N1 (Section 4 networking).

### Phase 3: B2B Directories + Inbound (Month 6+)

**Goal:** Clients find you.

**By now you have:** 2-4 completed team engagements, client testimonials on Clutch/GoodFirms, LinkedIn company page with project highlights, content positioning you as a team.

**Inbound sources:**
- Clutch/GoodFirms → high-intent clients searching for AI teams
- LinkedIn company page → inbound from company searches
- Content → "How we built X for Company Y" → DMs
- Referrals → past team clients recommend you
- Staff aug platforms → ongoing matched opportunities

---

## 5C.4: AI-Driven Contracts Pipeline

### Pipeline Stages

```
discovered → applied → screening → interview → offer → engaged → completed
Terminal: completed, rejected, withdrawn
```

Uses existing `contract` pipeline from `pipelines.go`. No stage changes needed.

Stage mapping to sales process:
| Stage | Sales Meaning |
|---|---|
| `discovered` | Company identified as target (TheirStack/upsell/inbound) |
| `applied` | Outreach sent or inquiry responded to |
| `screening` | Discovery call completed, understanding needs |
| `interview` | Proposal/SOW sent, in negotiation |
| `offer` | Terms agreed, contract being signed |
| `engaged` | Contract active, team delivering |
| `completed` | Contract delivered, post-contract actions |

### Human Gates

**Gate C1: CONTRACT REVIEW (weekly, 30-45 min)**

Scout shows contract opportunities: "2 upsell opportunities, 3 outreach targets, 1 inbound inquiry"

For each:
- Company, size, funding, tech stack, # AI roles open
- AI-generated pitch / proposal
- Outreach sequence drafts
- Recommended engagement model
- Estimated deal size

Actions: `[Pursue]` `[Edit Pitch]` `[Skip]` `[Snooze]`

**Gate C2: DEAL DECISION (per opportunity)**

After discovery call + negotiation:
- AI-generated SOW for review
- Pricing breakdown
- Team composition recommendation
- Risk assessment

Actions: `[Accept]` `[Counter]` `[Walk Away]`

**Gate C3: POST-CONTRACT (per completion)**

- Request testimonial for Clutch/GoodFirms
- AI drafts case study from project notes
- Propose follow-on engagement
- Ask for referrals to other companies

### New AI Tools

| Tool | Exists? | Input | Output |
|---|---|---|---|
| `CompanyPitch` | Yes (`ai/pitch.go`) | Lead data + portfolio | 5-section pitch document |
| `SOWGenerator` | New | Discovery call notes + lead data | Draft SOW: scope, team, timeline, pricing, terms |
| `ContractFollowUp` | New | Lead + negotiation stage + last message | Follow-up email appropriate to negotiation phase |
| `CaseStudyDraft` | New | Completed project notes + results | Case study for Clutch/LinkedIn/portfolio |
| `UpsellDetector` | New | Active freelance gig data | Flag: upsell opportunity + team proposal draft |

### Implementation

**New AI tools:** `internal/scout/ai/sow.go`, `internal/scout/ai/case_study.go`, `internal/scout/ai/upsell.go`, `internal/scout/ai/contract_followup.go`

**Modified:** `internal/scout/runner/` — add contracts pipeline phases:
- IDENTIFY: TheirStack multi-hire filter + freelance upsell detection
- PITCH: generate CompanyPitch for qualified targets
- CADENCE: outreach sequence management (Day 0/3/7/10/14/21)
- POST-CONTRACT: surface Gate C3 items for completed engagements

**Modified:** Scout frontend Actions tab:
- "Contract Opportunities" section (Gate C1)
- "Active Negotiations" section (pending SOWs)
- "Post-Contract Actions" section (Gate C3)

---

## 5C.5: Deal Size Progression & Pricing

### By Phase

| Phase | Deal Size | Typical Setup |
|---|---|---|
| **Phase 1** (Month 1-3) | ₹1-5L/mo | You + 1 support eng, or 1 embedded staff aug |
| **Phase 2** (Month 3-6) | ₹5-15L/mo | 2-3 engineers, project delivery or staff aug |
| **Phase 3** (Month 6+) | ₹15L+/mo | 4+ engineers, retainer or large project |

### Pricing Rules

- Quote per-person monthly, not hourly (professional positioning)
- Retainer discount: 10-15% for 6+ month commitment
- Project pricing: team hours × per-person rate × 1.4 (40% buffer for coordination + scope changes)
- Payment terms: 30% upfront, 40% midpoint, 30% on delivery (project-based). Monthly invoicing for staff aug/retainer.
- Never start work without signed SOW + first payment received.

### Revenue Trajectory

| Period | Monthly Revenue | Source |
|---|---|---|
| Month 1-3 | ₹1-3L | 1 small contract (freelance upsell) |
| Month 3-6 | ₹3-8L | 1-2 medium contracts (outreach + upsell) |
| Month 6+ | ₹8-15L | 1-2 medium/large (all channels) |
| Month 12+ | ₹15-30L | Multiple concurrent retainers + projects |

Combined with individual freelance: ₹10-20L/month potential by Month 6.

### Weekly Time Commitment

| Phase | Sales Time | Delivery/Management | Total |
|---|---|---|---|
| Phase 1 (Month 1-3) | ~1 hr/week | 2-3 hrs/week (if active) | ~3-4 hrs/week |
| Phase 2 (Month 3-6) | ~2 hrs/week | 3-5 hrs/week | ~5-7 hrs/week |
| Phase 3 (Month 6+) | ~2 hrs/week | 5-8 hrs/week | ~7-10 hrs/week |

---

## Prerequisites

- Individual freelance strategy implemented (Phase 1 clients are the upsell source)
- `CompanyPitch` tool already exists in `internal/scout/ai/pitch.go`
- Pipeline runner from job application spec
- Networking pipeline from networking spec (contact sources at WARM+)

## Relationship to Other Specs

- **Freelance (`docs/scout/freelance.md`):** Freelance clients are the primary source for Phase 1 contracts. `UpsellDetector` monitors active freelance gigs.
- **Networking (`docs/scout/networking.md`):** Founder/CTO contacts at WARM+ warmth are contract lead sources. Contract outreach sequences integrate with Gate N1 from networking pipeline.
- **Jobs (`docs/scout/jobs.md`):** If a full-time job offer is accepted, wind down contracts gracefully (finish active engagements, stop new outreach).
- **Content (Section 6):** "How we built X for Company Y" posts drive contract inbound from Phase 3+.
- **Consulting (`docs/scout/consulting.md`):** Expert calls are separate. A consulting relationship can lead to a contract engagement (advisory → "can your team build this?").
