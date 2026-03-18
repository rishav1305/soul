# Scout Strategy: Job Discovery & Application

**Parent doc:** `docs/scout/README.md` Sections 2 & 3
**Specs:** `docs/superpowers/specs/2026-03-18-job-application-strategy-design.md`

---

## Job Discovery (Section 2)

### 3-Layer Discovery

```
Layer 1: TheirStack API (free tier — PRIMARY)
├── 321,000+ sources including LinkedIn, Indeed, Naukri, Glassdoor
├── 16,000+ ATS career pages (Greenhouse, Lever, Workable, Ashby)
├── 175,000+ company career sites
├── Auto-deduplication, tech stack intelligence, company enrichment
├── Hiring manager identification (name + LinkedIn URL)
└── 200 free API credits/month

Layer 2: Manual Platform Browsing
├── Wellfound, LinkedIn (Premium), Naukri (FastForward)
└── Community job channels (Discord/Slack)

Layer 3: Inbound (compounds over time)
├── Recruiter DMs, Naukri contacts, X/Twitter DMs
├── Community referrals, Wellfound, vetted networks
└── Content-driven inbound
```

### TheirStack Filter (configured in `~/.soul-v2/sweep-config.json`)

- **36 AI/ML job titles** across 6 categories (core, platform, senior, hybrid, data science, broader engineering)
- **8 title exclusions** (intern, junior, fresher, trainee, etc.)
- **Seniority:** senior, lead, staff, principal, director, vp, head, manager
- **35+ tech slugs:** pytorch, anthropic, langchain, openai, go, react, kubernetes, etc.
- **Countries:** IN (primary), US, CA, GB, DE, NL, SG, AE, AU
- **Location patterns:** remote + 10 Indian cities
- **7-day window, daily sweep, 50 credits/sweep**

Full filter details in parent doc Section 2.

---

## Job Application Strategy (Section 3)

### Tier Classification Engine

**Step 1 — Instant (zero API cost):**

| Tier | Rule | Strategy |
|---|---|---|
| **Tier 1** | Dream company list OR funding > $50M OR 500+ employees in AI OR tech stack includes anthropic/openai/deepmind | Warm only. Never cold apply. |
| **Tier 2** | Series A+ AND AI tech stack AND 50-500 employees | Career page + LinkedIn HM outreach same day |
| **Tier 3** | Everything else | Speed apply on portal |

Dream companies: Anthropic, DeepMind, OpenAI, Google AI, Meta AI, Microsoft Research, Apple ML, Amazon AGI, Cohere, Mistral, Databricks, Hugging Face, Stability AI, Adept, Character.AI, Inflection, xAI, Perplexity, Runway, Midjourney

**Step 2 — Match Scoring (Claude via OAuth, Tier 1/2 only):**
- Score > 85 → promote tier
- Score 70-85 → keep tier
- Score < 70 → demote or skip

### Pipeline Stages

```
discovered → qualified → preparing → outreach-sent → applied →
screening → interview → offer → joined

Terminal: joined, rejected, withdrawn, skipped
```

### AI-Driven Pipeline with Human Gates

```
PHASE 1: DISCOVER (automated) → TheirStack sweep, tier classify
PHASE 2: QUALIFY (automated) → Claude match scoring
PHASE 3: PREPARE (automated) → tailored resume, cover letter, outreach draft, salary estimate

  ★ GATE 1: MORNING REVIEW (30 min daily)
  Review batch: [Approve] [Edit] [Skip] [Change Tier]

PHASE 4: EXECUTE (you)
  Tier 1 → LinkedIn outreach → "outreach-sent"
  Tier 2 → career page + HM outreach → "applied"
  Tier 3 → portal → "applied"

PHASE 5: FOLLOW-UP (automated cadence)
  Day 0 → Day 3 → Day 5 → Day 8 (fallback) → Day 14 (withdrawn)
  ★ GATE 2: follow-up review (Wed)
  ★ GATE 3: stale decisions (Fri)
```

### Weekly Targets (Aggressive)

| Metric | Target |
|---|---|
| Leads reviewed | 20-30 |
| Qualified | 12-15 |
| Prepared by AI | 8-10 |
| Applications submitted | 5-8 |
| Tier 1 warm approaches | 3 |
| Tier 2 HM outreaches | 4-5 |

**Your time: ~3.5 hrs/week**

### Where to Apply

- Has referral → referrer submits internally
- Tier 1, no referral → outreach first, Day 8 fallback to career page
- Tier 2 → company career page + LinkedIn HM outreach
- Tier 3 → portal where found
- Never: LinkedIn Easy Apply, generic resume, multiple portals for same company
