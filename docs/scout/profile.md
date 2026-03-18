# Scout Strategy: Profile & Online Presence

**Parent doc:** `docs/scout/README.md` Section 7
**Spec:** `docs/superpowers/specs/2026-03-18-profile-presence-design.md`
**Status:** Spec'd

---

## Scope

Three-platform presence: LinkedIn (discovery) → Portfolio (depth) → GitHub (proof). AI continuously syncs all three when work completes in any Scout pipeline.

## The Funnel

```
LinkedIn → "I solve X. Here's proof." → recruiters, HMs, business
  ↓
Portfolio → "Here's HOW I solved X." → clients, founders, evaluators
  ↓
GitHub → "Here's the actual CODE." → engineers, researchers, interviewers
```

## LinkedIn Search Optimization

| Section | Keyword Type | Example |
|---|---|---|
| Headline | Role-based | "Senior AI Engineer \| LLM, RAG, AI Agents" |
| About | Industry-based | "healthcare AI", "legal AI", "enterprise AI" |
| Experience | Skill-based | "Built RAG pipeline serving 10K queries/day" |
| Skills (50) | All searchable tags | ML, LLM, PyTorch, LangChain, Claude, Go, Python |
| Open to Work | Recruiter-only | AI Engineer, ML Engineer, AI Architect |

## Portfolio App

| Section | Purpose |
|---|---|
| Hero | Hook in 5 seconds — name, value prop, metrics, CTAs |
| What I Do | 3 service cards linking to case studies |
| Case Studies | 4-6 detailed breakdowns (problem, approach, architecture, results) |
| Expertise | Skills organized by domain (visual) |
| Testimonials | Client quotes (target: 3-5 in 6 months) |
| Writing | Substack articles + best LinkedIn posts |
| Contact | CTAs per intent: hire / contract / consult |

## GitHub Curation

**6 Pinned Repos (strategic):**
1. RAG System → most in-demand skill
2. AI Agent → agentic AI is the hot topic
3. LLM Evaluation → shows rigor
4. Full System → architecture at scale
5. Analysis Notebook → research credibility
6. Industry-Specific → consulting credibility

**Action Plan:**
- Month 1: Audit + polish existing, pin best 6
- Month 2-3: Build 1 strategic repo (RAG or agent)
- Month 4-6: Build 1 more (evaluation or vertical)
- Target: 4+ commit days/week (real work, not fake)

## Continuous AI Sync (2 Gates)

- **Gate PR1** (as triggered): Project completed → AI drafts updates for all 3 platforms → you review and apply
- **Gate PR2** (quarterly): AI audits all 3 → headline freshness, skills count, case study age, pinned repo activity, testimonial count

**Time: ~0 hrs/week** (triggered by other pipelines) + **1 hr/quarter** (audit)
