# Social Media & Content Strategy — Design Spec

**Date:** 2026-03-18
**Status:** Approved
**Scope:** `docs/scout/content.md` — content creation, publishing, engagement, metrics, and content-to-lead tracking across LinkedIn, X/Twitter, Substack, and GitHub.

---

## Overview

AI-driven content pipeline that turns your real work (research, analysis, building) into polished multi-platform content. You provide raw insights, AI generates posts/threads/articles, you review and publish. Fully tracked: every post logged with metrics, correlated to inbound leads. Weekly pulse + monthly retrospective + quarterly vertical assessment create a self-improving feedback loop.

## Design Decisions

| Decision | Choice | Why |
|---|---|---|
| AI automation | You research/analyze, AI packages into content, you review | Authentic insights + AI writing speed. Not AI-generated opinions — AI-packaged genuine experience. |
| Lead tracking | Fully tracked — posts logged with metrics, correlated to inbound leads | Content is the inbound engine for all pipelines. Must measure ROI. |
| Engagement ownership | Content owns publishing + own-post replies. Networking (Section 4) owns engagement on others' posts. Scout coordinates timing. | Clean separation. Different intent: content engagement = algorithm boost, networking engagement = relationship building. |
| Topic generation | Evergreen backlog + reactive content as backup, AI manages both | No blank-page weeks. Always something to post. |
| Vertical content | Quarterly rotation on Substack (legal → healthcare → sales → e-commerce) | SEO-driven consulting inbound. 3 deep-dives per quarter = domain authority. |
| Profile/Portfolio | Separate section (7) — content drives visits, profile converts | Different cadence: content = weekly, profile = quarterly updates. User also has portfolio app. |
| Metrics | Weekly pulse + monthly strategy + quarterly vertical assessment | Automated feedback loop. AI recommends pillar rebalancing based on what drives leads. |

---

## 6.1: Platform Strategy

| Platform | Role | Audience | Frequency |
|---|---|---|---|
| **LinkedIn** | Primary publishing | Recruiters, HMs, enterprise clients, founders | 3 posts/week |
| **X/Twitter** | Research community | AI researchers, lab engineers, builders | 2-3 posts/week |
| **Substack** | SEO + depth | Google searchers, newsletter subscribers | 1 deep post/month (vertical) |
| **GitHub** | Proof layer | Technical evaluators | Always linked |
| **Dev.to** | Cross-post reach | Broader developers | Cross-post Substack |

Skip: Instagram, YouTube, TikTok.

### Platform-Specific Rules

**LinkedIn:**
- Text posts (150-400 words) and carousels (PDF, 3-5x more impressions) perform best
- First line = everything (2 lines visible before "see more")
- Native video (60-90s) algo-favored
- Comments in first 60 minutes = biggest algorithm signal
- Never use LinkedIn Articles (buried). Always posts.
- Post 8-10 AM IST (catches US evening, EU morning, India morning)

**X/Twitter:**
- Threads (5-10 tweets) for depth — researchers love threads
- Single tweets for hot takes
- Quote-tweet with angle > plain retweet
- Engage in replies before posting (warms algorithm)

**Substack:**
- 1500-3000 words with charts, code, diagrams
- SEO-optimized titles with searchable keywords
- Cross-post to Dev.to
- Each article = canonical reference for cold outreach

**GitHub:**
- Notebooks for reproducible analyses = credibility
- README quality matters — treat repos as portfolio
- Stars/forks are social proof

---

## 6.2: Content Model

### Weekly: 3-Part Horizontal Series

One analysis per week from real work, split into 3 standalone posts.

| Day | Part | Format (LinkedIn) | Format (X) |
|---|---|---|---|
| Tuesday | Hook Post | 100-150 words, provocative opening | Single punchy tweet |
| Wednesday | — | — | X thread (technical version of Thursday's deep dive, published 1 day early to warm up engagement) |
| Thursday | Deep Dive | 300-500 words + carousel/chart (LinkedIn 3000 char limit — split into carousel if exceeds) | — (thread already published Wed) |
| Saturday | Takeaway/Opinion | 200-300 words + open question | Hot take tweet |

**Part 1 structure:** Provocative finding → 1-line context → 2-3 bullet teaser → "More this week."
**Part 2 structure:** Methodology (2-3 lines) → findings (3-5) → surprise → "Tomorrow: my take."
**Part 3 structure:** "Here's my honest take:" → 3-5 conclusions → contrarian opinion → open question for comments.

### Monthly: Vertical Deep-Dive (Substack)

| Quarter | Vertical | Drives |
|---|---|---|
| Q1 | Legal AI | Legal tech consulting leads |
| Q2 | Healthcare AI | Healthcare consulting leads |
| Q3 | Sales AI | Sales tech consulting leads |
| Q4 | E-commerce AI | E-commerce consulting leads |

3 deep-dives per quarter = domain authority. SEO-indexed = long-term inbound.

### Repurposing Pipeline

One analysis → 7+ content pieces:

```
Analysis → LinkedIn Part 1 (Tue) + X tweet (Tue)
        → LinkedIn Part 2 + carousel (Thu) + X thread (Wed)
        → LinkedIn Part 3 (Sat) + X hot take (Sat)
        → Substack (monthly, expanded from best post)
        → Dev.to cross-post + GitHub notebook (if reproducible)
        → Short video (every 2 weeks, if demo-able)
```

---

## 6.3: Content Pillars

| Pillar | % | Content Type | Drives |
|---|---|---|---|
| **Builder Insights** | 40% | Architecture decisions, production failures, scale challenges, cost breakdowns | Job + freelance inbound |
| **Curated Technical Takes** | 25% | Paper reactions, model comparisons, news analysis with data | Researcher visibility, networking |
| **Career/Consulting Lessons** | 20% | Build vs buy, vendor evaluation, project scoping, enterprise AI | Consulting inbound |
| **Personal/Process** | 15% | Behind-the-scenes, hardware, working from India, weekly updates | Follower retention, trust |

### Analysis Types

**Tier 1 (Builder + Technical):** Benchmark deconstructions, cost/performance comparisons, failure pattern analysis, architecture deep-dives, tool/framework comparisons with data.

**Tier 2 (Consulting):** Vendor landscape maps, build vs buy frameworks, ROI calculators, vertical-specific analyses.

**Tier 3 (Community):** Paper reproductions, dataset analyses, trend tracking, "what I'm building" updates.

---

## 6.4: AI-Driven Content Pipeline

### Flow

```
PHASE 1: TOPIC SELECTION (Sunday)
  Sources: your work this week + evergreen backlog + reactive news + vertical calendar
  AI recommends topic + pillar + angle
  You confirm or redirect

PHASE 2: GENERATE (automated after confirmation)
  You provide: raw insights (bullet points, data, findings)
  AI generates: 3 LinkedIn posts + 3 X versions + carousel outline + 5 hook variations
  Stored in lead_artifacts type="content_draft"

  ★ GATE P1: CONTENT REVIEW (Monday, 30 min)
  Review batch: [Approve] [Edit] [Reschedule] [Drop]

PHASE 3: PUBLISH & ENGAGE (Tue/Thu/Sat)
  Scout reminder → you publish (copy-paste, 2 min)
  Within 60 min: AI drafts replies to comments on YOUR post
  ★ GATE P2: ENGAGEMENT REVIEW (5 min per post day)
  [Reply] [Edit & Reply] [Skip]

  Section 4 (Networking) handles engagement on OTHERS' posts separately.
  Scout coordinates: publish first → engage own comments → engage others.

PHASE 4: TRACK & LEARN (automated)
  Per-post metrics: impressions, likes, comments, shares, saves, profile views
  Inbound leads auto-created: DM after post → lead with source="content-{platform}"
  Lead routing: recruiter → job pipeline, founder → freelance/contract, analyst → consulting
```

### Human Gates

- **Gate P1** (Monday, 30 min): Review week's content batch
- **Gate P2** (Tue/Thu/Sat, 5 min each): Reply to comments on your posts

### AI Tools

| Tool | Execution | Input | Output | Storage |
|---|---|---|---|---|
| `ContentTopicGen` | Sync | Week's work + backlog + news | 3 topic suggestions with pillar + angle | Backlog: `~/.soul-v2/content-backlog.json` |
| `ContentSeriesGen` | Sync | Topic + raw insights | 3 LinkedIn posts + 3 X versions + carousel outline | `lead_artifacts` type=`"content_draft"` |
| `HookWriter` | Sync | Draft post | 5 hook variations (8 proven formulas) | Inline with draft |
| `ThreadConverter` | Sync | LinkedIn deep-dive | X thread (5-10 tweets, more technical) | `lead_artifacts` type=`"content_draft"` |
| `SubstackExpander` | Sync | Best LinkedIn post of month | 1500-3000 word article with SEO title | `lead_artifacts` type=`"content_draft"` |
| `EngagementReply` | Sync | Comment + post context | Thoughtful reply continuing conversation | Shown at Gate P2 |
| `ContentMetrics` | Sync | `mode` param ("weekly"/"monthly"/"quarterly") + reads from `content_posts` table directly. Metrics data entered manually by user (paste platform analytics) or via future API integration. | Weekly: top post + engagement summary + lead count. Monthly: ranked posts + pillar comparison + patterns + calendar. Quarterly: vertical assessment + SEO + consulting leads. | `lead_artifacts` type=`"content_metrics"` |
| `ReactiveContentGen` | Sync | News/paper + your expertise | Hot-take post draft | `lead_artifacts` type=`"content_draft"` |

---

## 6.5: Content Backlog & Topic Generation

### Evergreen Backlog

Location: `~/.soul-v2/content-backlog.json`
Format: array of `{id, topic, pillar, source, angle, created_at, status}`

**Sources:** Past work not yet written about, saved papers, unfinished analysis angles, industry trends, community questions.

**Lifecycle:**
- AI generates 2-3 new ideas per week
- Ideas older than 60 days auto-archived
- User can manually add anytime
- No fresh topic on Sunday → AI picks top backlog item

### Reactive Content Queue

AI monitors: new model releases, viral AI posts, papers in your domain, competitor claims.
Reactive posts = bonus content on top of weekly 3-part series. Positions you as go-to voice for AI news.

---

## 6.6: Hook Writing System

### 8 Proven Formulas

| Formula | Template |
|---|---|
| Counterintuitive claim | "[Common belief] is wrong. Here's why." |
| Specific number | "[Big number]. [Time period]. Here's what happened." |
| Hard-won lesson | "I [expensive mistake] so you don't have to." |
| Provocative question | "Is your [thing] actually [what it claims]?" |
| Confession | "I [did something wrong] for [long time]." |
| Contrarian | "Everyone says [X]. I think the opposite." |
| Data reveal | "I analyzed [X]. [Unexpected finding]." |
| Before/after | "Before: [bad state]. After: [good state]. One change." |

### Rules

- First line must stand alone
- No greetings, no preamble
- Numbers > adjectives
- Tension > information (curiosity gap)

---

## 6.7: Metrics & Feedback Loop

### Data Model

**New table: `content_posts`**

```sql
CREATE TABLE content_posts (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  platform TEXT NOT NULL,
  pillar TEXT NOT NULL,
  topic TEXT NOT NULL,
  status TEXT NOT NULL DEFAULT 'draft',  -- draft, scheduled, published, archived
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
CREATE INDEX idx_content_platform ON content_posts(platform);
CREATE INDEX idx_content_pillar ON content_posts(pillar);
```

**Content-to-lead tracking:**

Add `source_ref_id INTEGER` column to `leads` table (part of combined migration). When an inbound lead is created from content, `source_ref_id` references `content_posts.id`. This enables per-post ROI calculation:
```sql
SELECT cp.topic, cp.pillar, COUNT(l.id) as leads_generated
FROM content_posts cp
JOIN leads l ON l.source_ref_id = cp.id
GROUP BY cp.id
```

The `content_posts.inbound_leads` column is a denormalized counter — maintained by the runner when new leads are created with `source LIKE 'content-%'`. This avoids JOIN queries for dashboard display. The authoritative count comes from the JOIN above.

**Content backlog table** (replaces flat file for transactional consistency):

```sql
CREATE TABLE content_backlog (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  topic TEXT NOT NULL,
  pillar TEXT NOT NULL,
  source TEXT NOT NULL,       -- "work", "paper", "trend", "community", "reactive"
  angle TEXT,
  status TEXT DEFAULT 'pending',  -- pending, selected, archived
  created_at TEXT NOT NULL DEFAULT (datetime('now')),
  archived_at TEXT
);
```

Auto-archival: items older than 60 days with `status='pending'` move to `status='archived'` by the runner.
The flat file `~/.soul-v2/content-backlog.json` is no longer used. SQLite gives query capability, transactional consistency, and date filtering.

### Weekly Pulse (Friday, 5 min — part of Gate N2)

- Total impressions (LinkedIn + X)
- Engagement rate per post
- Profile views generated
- New DMs/connection requests (inbound leads)
- Top-performing post + AI analysis of why

### Monthly Retrospective (auto-generated)

- Posts ranked by performance
- Pillar comparison (which drives most engagement + leads)
- Pattern analysis ("numbers in hooks = 2.1x engagement")
- Content → lead correlation ("RAG posts → 4 DMs")
- AI-recommended pillar rebalancing
- Next month's content calendar

### Quarterly Vertical Assessment

- Vertical deep-dives: views, leads, conversions
- SEO performance: ranking keywords
- Consulting leads traced to vertical content
- Continue / pivot / adjust recommendation

---

## 6.8: 30-Day Kickstart

| Week | Key Actions |
|---|---|
| **Week 1** | Set up Substack. Optimize LinkedIn profile. Write + publish first 3 posts. First carousel. |
| **Week 2** | Full 3-part series cycle. Start X thread habit. Follow 30-50 targets on X. Review first metrics. |
| **Week 3** | Third series. First Substack deep-dive (current quarter's vertical). Cross-post Dev.to. |
| **Week 4** | Fourth series. First short video. First reactive post. Monthly metrics review. Generate month 2 calendar. |

By day 30: 12+ LinkedIn posts, 8+ X threads/tweets, 1 Substack article, 1 video, baseline metrics.
Compounding kicks in week 6-8.

---

## 6.9: Time Budget

| Activity | Time | When |
|---|---|---|
| Topic selection + raw insights | 30 min | Sunday |
| Review content batch (Gate P1) | 30 min | Monday |
| Publish + engage (Gate P2) | 10 min × 3 days | Tue/Thu/Sat |
| Weekly metrics review | 5 min | Friday |
| Monthly Substack review | 45 min | Last week of month |
| **TOTAL (weekly average)** | **~2 hrs/week** | |

---

## Error Handling

**AI generation failure:**
- If `ContentSeriesGen` fails on Sunday/Monday → runner promotes top backlog item and retries on next cycle
- If retry also fails → surface in Actions tab: "Content generation failed. Manual override: write post or skip this week."
- After 3 consecutive failures for same topic → auto-archive topic, move to next backlog item

**Metrics data entry:**
- Platform metrics (impressions, likes, etc.) are entered manually by user after publishing
- Scout prompts: "Update metrics for Tuesday's post?" in Friday review
- Missing metrics = zero — doesn't block pipeline, just gaps in reporting
- Future: LinkedIn/X API integration for automatic metrics fetch (not in v1)

## Content Lifecycle (not a pipeline — separate from lead pipelines)

Content does NOT use `pipelines.go` stages. Content lifecycle is managed via `content_posts.status`:

```
draft → scheduled → published → archived
```

The runner manages content phases (TOPIC, GENERATE, ENGAGE, METRICS) independently from lead pipeline phases. This is intentional: content is not a lead — it's a publishing workflow that GENERATES leads.

## Weekly Pulse Timing

The weekly pulse is part of the existing Friday review session but is NOT embedded inside Gate N2. It runs as a separate 5-min block during the same Friday review timeslot:

```
Friday Review Session (~30 min total):
  Gate N2: Networking weekly brief (10 min)
  Gate P-metrics: Content weekly pulse (5 min)
  Gate 3: Job stale lead decisions (15 min)
```

Each gate is independent. Content metrics do not depend on networking gate implementation.

## Prerequisites

**Blocking:**
1. Pipeline runner (`internal/scout/runner/`) — from job application spec. Content adds its own phases (TOPIC, GENERATE, ENGAGE, METRICS) to the shared runner loop.
2. `lead_artifacts` table — for storing content drafts
3. `ValidateTransition` enforcement — content creates inbound leads that enter lead pipelines. Stage integrity required.

**Non-blocking:**
- Networking pipeline — coordinates engagement timing. Content works standalone without it.

**New infrastructure:**
- `content_posts` table (defined in Section 6.7)
- `content_backlog` table (defined in Section 6.7 — replaces flat file)
- `source_ref_id` column on `leads` table (for content-to-lead FK)
- 8 new AI tools in `internal/scout/ai/`

**Platform constraints for AI tools:**
- LinkedIn posts: 3000 character limit. Deep dives exceeding this → use carousel (PDF) format.
- X tweets: 280 character limit per tweet. Threads = chained replies.
- `ContentSeriesGen` and `ThreadConverter` must respect these limits during generation.

**Vertical rotation configuration:**
- Quarterly vertical schedule is a default, not hardcoded
- Stored in `~/.soul-v2/content-config.json`: `{"current_vertical": "legal", "rotation": ["legal", "healthcare", "sales", "ecommerce"]}`
- Quarterly assessment can recommend pivot → user updates config
- Runner reads config at start of each month's Substack generation

## Relationship to Other Specs

- **Jobs (Section 3):** Content drives recruiter/HM DMs → job inbound leads
- **Networking (Section 4):** Publishing coordinated with networking engagement. Scout ensures timing: publish → engage own comments → engage others' posts.
- **Freelance (Section 5):** "I built X" posts → freelance inbound
- **Contracts (Section 5):** Case study posts → contract inbound
- **Consulting (Section 5):** Vertical Substack articles → consulting inbound. Consulting work generates content ideas (virtuous cycle).
- **Profile/Portfolio (Section 7):** Content drives profile visits. Profile converts visitors. Different sections, coordinated purpose.
