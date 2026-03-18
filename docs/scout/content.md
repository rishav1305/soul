# Scout Strategy: Social Media & Content

**Parent doc:** `docs/scout/README.md` Section 6
**Status:** Detailed

---

## Purpose

Content serves three functions simultaneously:
1. **Attract recruiters & HMs** → job pipeline inbound
2. **Build credibility with researchers** → networking pipeline contacts
3. **Generate consulting/freelance inbound** → freelance + consulting pipelines

Content is NOT a separate activity — it's the inbound engine that feeds ALL other Scout pipelines.

```
Your work (research, analysis, building)
  → AI packages into content (posts, threads, carousels)
  → You review and publish
  → Audience engages
  → Inbound leads flow into Scout pipelines (job, freelance, consulting)
  → Metrics feed back into what to create more of
```

---

## 6.1: Platform Strategy

| Platform | Role | Audience | Frequency |
|---|---|---|---|
| **LinkedIn** | Primary — publishing + engagement | Recruiters, HMs, enterprise clients, founders | 3 posts/week |
| **X/Twitter** | Secondary — research community | AI researchers, lab engineers, builders | 2-3 posts/week |
| **Substack** | Long-term asset — SEO + depth | Google searchers, newsletter subscribers | 1 deep post/month (vertical-focused) |
| **GitHub** | Proof layer — show, don't tell | Technical evaluators, hiring managers | Always linked from posts |
| **Dev.to** | Cross-post for reach | Broader developer audience | Cross-post Substack articles |

**Skip:** Instagram, YouTube, TikTok — wrong audience, wrong ROI for current goals. YouTube becomes relevant only if building a developer education brand later (6+ months).

### Platform-Specific Rules

**LinkedIn:**
- Long-form text posts (150-400 words) perform best
- Carousels (PDF slideshows) get 3-5x more impressions than text
- First line = everything (only 2 lines visible before "see more")
- Native video (60-90s) is algo-favored — screen record + voiceover, no editing needed
- Comments in first 60 minutes = biggest algorithm signal
- Never use LinkedIn articles (buried by algorithm). Always use posts.
- Post between 8-10 AM IST (catches US evening, EU morning, India morning)

**X/Twitter:**
- Threads (5-10 tweets) for deep content — researchers love threads
- Single punchy tweets for hot takes and reactions
- Quote-tweet with your angle > plain retweet
- Engage in replies before posting your own content (warms the algorithm)
- Follow-to-engagement ratio matters — don't follow thousands, engage with dozens

**Substack:**
- One deep post per month — quality over frequency
- 1500-3000 words with charts, code snippets, architecture diagrams
- SEO-optimized: title includes searchable keywords
- Cross-post to Dev.to for broader reach
- Every Substack article = canonical reference for cold outreach ("here's my analysis on this")

**GitHub:**
- Notebooks/code for reproducible analyses = massive credibility
- README quality matters — treat repos as portfolio pieces
- Link from every post that references your work
- Stars/forks are social proof — include in LinkedIn profile

---

## 6.2: Content Model — Weekly Analysis + Vertical Deep-Dives

### Weekly: 3-Part Horizontal Series

One analysis per week from your real work, split into 3 standalone posts.

```
Sunday    → Pick topic + jot down raw insights (or AI picks from backlog)
Tuesday   → Part 1: The Hook Post
Thursday  → Part 2: The Deep Dive
Saturday  → Part 3: The Takeaway/Opinion
```

#### Part 1 — Hook Post (Tuesday)

**Purpose:** Stop the scroll. Create curiosity. Drive follows.
**Format:** 100-150 words on LinkedIn. Single tweet on X.

```
Structure:
  Provocative finding or counterintuitive number
  → 1-line context (what you analyzed, why)
  → 2-3 bullet teaser of findings
  → "Breaking this down across 3 posts this week."
```

#### Part 2 — Deep Dive (Thursday)

**Purpose:** Deliver substance. Build credibility.
**Format:** 300-500 words on LinkedIn + carousel or chart. Full thread on X.

```
Structure:
  Methodology (2-3 lines, shows rigor)
  → Key findings (3-5, with data/examples)
  → What surprised you
  → "Tomorrow: what this means and what I'd do differently"
```

Charts, even rough matplotlib outputs, signal authenticity. Raw > polished.

#### Part 3 — Opinion/Takeaway (Saturday)

**Purpose:** Convert readers into followers and leads. Most shareable.
**Format:** 200-300 words on LinkedIn. Punchy thread on X.

```
Structure:
  "After [analysis], here's my honest take:"
  → 3-5 actionable conclusions
  → One contrarian opinion (most shareable element)
  → Open question to audience (drives comments → algorithm boost)
```

### Monthly: Vertical Deep-Dive (Substack)

Rotate quarterly through your consulting verticals:

| Quarter | Vertical | Example Titles |
|---|---|---|
| Q1 | Legal AI | "Why RAG Fails for Legal Document Review (and What Works)", "AI Compliance Automation: Architecture Deep-Dive" |
| Q2 | Healthcare AI | "LLM Guardrails for Clinical Data: A Production Approach", "HIPAA-Compliant AI Architecture Patterns" |
| Q3 | Sales AI | "Building an AI Lead Scoring System That Actually Works", "Conversational AI for Sales: Beyond the Chatbot" |
| Q4 | E-commerce AI | "Recommendation Engines in 2026: What's Changed", "AI-Powered Search That Converts" |

Each quarter = 3 deep-dives in one vertical = domain authority in that space.

**Why vertical matters:** A healthcare CTO searching "HIPAA AI architecture" finds your Substack. That's a consulting lead that no LinkedIn post could generate. SEO-driven inbound is the longest game but highest quality.

### Cross-Platform Adaptation

Same analysis, different packaging per platform:

```
LinkedIn Part 1  → Hook post (broad audience, max reach)
LinkedIn Part 2  → Deep dive + carousel (credibility builder)
LinkedIn Part 3  → Opinion post (engagement driver, comments)

X Part 1         → Single punchy tweet (awareness)
X Part 2         → Full technical thread (researcher engagement)
X Part 3         → Hot take tweet (most RT-able, virality potential)

Monthly Substack → Full analysis combined (SEO + newsletter)
Dev.to           → Cross-post Substack (broader dev reach)
GitHub           → Notebook/code if reproducible (proof layer)
```

---

## 6.3: Content Pillars

| Pillar | % | What | Drives |
|---|---|---|---|
| **Builder Insights** | 40% | Real lessons from building. Architecture decisions, production failures, scale challenges, cost breakdowns. Your most differentiated content — nobody else has your specific war stories. | Job + freelance inbound |
| **Curated Technical Takes** | 25% | React to papers, model releases, AI news with your own sharp angle. Don't summarize — evaluate. Add data from your experience. | Research community visibility, networking |
| **Career/Consulting Lessons** | 20% | Senior practitioner perspective. Project scoping, enterprise AI lessons, build vs buy, how to evaluate AI vendors. Business judgment, not just technical depth. | Consulting inbound |
| **Personal/Process** | 15% | Makes you human. Hardware builds, working from India targeting global opportunities, weekly updates, behind-the-scenes of your projects. People hire people, not resumes. | Follower retention, trust |

### Analysis Types by Pillar

**Tier 1 — Highest impact (Builder Insights + Technical Takes):**
- LLM benchmark deconstructions (re-run benchmarks with production-style prompts, compare to published results)
- Cost/performance analysis (model comparisons at real scale with actual numbers — tokens, latency, cost/query)
- Failure pattern analysis (how production AI systems actually break — not theoretical, from your experience)
- Architecture deep-dives (how you built something, with diagrams and trade-off analysis)
- Tool/framework comparisons with actual usage data

**Tier 2 — Consulting inbound (Career/Consulting):**
- Vendor landscape maps ("The LLM evaluation tools landscape in 2026 — who does what")
- Build vs buy frameworks ("When to build your own RAG vs use a managed service — decision tree")
- ROI frameworks ("How to calculate actual ROI of an AI agent deployment")
- Vertical-specific analyses (legal AI compliance, healthcare AI architecture, etc.)

**Tier 3 — Community engagement (Technical Takes + Personal):**
- Paper reproduction with your own commentary and extensions
- Public dataset analysis with an interesting finding
- Trend tracking ("I tracked AI Engineer job postings for 4 weeks — here's what changed")
- "What I'm building this week" updates

---

## 6.4: AI-Driven Content Pipeline

### Content Operating System

```
┌──────────────────────────────────────────────────────────┐
│ PHASE 1: TOPIC SELECTION (Sunday, AI-assisted)           │
│                                                          │
│ Source 1: Your work this week                            │
│   AI scans: git commits, project notes, experiments run  │
│   → "You built X this week. Here are 3 content angles."  │
│                                                          │
│ Source 2: Evergreen backlog                              │
│   AI maintains queue of 10-15 unused content ideas       │
│   Generated from: past work, saved papers, analysis      │
│   angles not yet covered, industry trends                │
│   → "5 ideas in your backlog. Top pick: [X]"            │
│                                                          │
│ Source 3: Reactive opportunities                         │
│   AI monitors: new model releases, viral AI posts,       │
│   papers in your domain, competitor claims               │
│   → "Anthropic released X yesterday. React?"            │
│                                                          │
│ Source 4: Vertical calendar                              │
│   Monthly Substack due → current quarter's vertical      │
│   → "Q1 Legal AI: 2 of 3 deep-dives done.              │
│      Suggested topic for third: [X]"                    │
│                                                          │
│ AI recommends topic + pillar + angle                     │
│ You confirm or pick different direction                  │
└──────────────┬───────────────────────────────────────────┘
               │
┌──────────────▼───────────────────────────────────────────┐
│ PHASE 2: GENERATE (automated after topic confirmed)      │
│                                                          │
│ You provide: raw insights (bullet points, data, findings)│
│ AI generates:                                            │
│   1. LinkedIn Part 1 — Hook post (5 hook variations)     │
│   2. LinkedIn Part 2 — Deep dive + carousel outline      │
│   3. LinkedIn Part 3 — Opinion/takeaway post             │
│   4. X thread version (converted from Part 2)            │
│   5. X single tweet versions (Part 1 + Part 3)           │
│   6. Monthly: Substack article (expanded from best post) │
│   7. Hashtag suggestions per platform                    │
│                                                          │
│ All drafts stored in lead_artifacts type="content_draft" │
│ with metadata: pillar, platform, scheduled_date          │
└──────────────┬───────────────────────────────────────────┘
               │
┌──────────────▼───────────────────────────────────────────┐
│ ★ GATE P1: CONTENT REVIEW (you, ~30 min Sunday/Monday)   │
│                                                          │
│ Scout shows this week's content batch:                   │
│   "3 LinkedIn posts, 3 X versions, 1 carousel outline"  │
│                                                          │
│ For each draft:                                          │
│   • Pillar tag, platform, scheduled day                  │
│   • Full draft text                                      │
│   • 5 hook variations for Part 1 (pick best)             │
│   • Carousel slide outline (if applicable)               │
│                                                          │
│ Actions: [Approve] [Edit] [Reschedule] [Drop]            │
│                                                          │
│ After approval: posts queued with scheduled dates        │
└──────────────┬───────────────────────────────────────────┘
               │
┌──────────────▼───────────────────────────────────────────┐
│ PHASE 3: PUBLISH & ENGAGE (you, per scheduled day)       │
│                                                          │
│ Tuesday morning:                                         │
│   Scout reminder: "Part 1 ready to publish"              │
│   You copy-paste to LinkedIn + X (2 min)                 │
│                                                          │
│ Tuesday afternoon (within 60 min of posting):            │
│   AI drafts replies to first comments on YOUR post       │
│   → ★ GATE P2: ENGAGEMENT REVIEW (5 min)                │
│   Actions: [Reply] [Edit & Reply] [Skip]                 │
│   Why: first-hour comments = biggest algorithm signal     │
│                                                          │
│ Thursday + Saturday: same publish + engage cycle          │
│                                                          │
│ Section 4 (Networking) handles:                          │
│   Commenting on OTHERS' posts — separate gate (N1)       │
│   Scout coordinates: publish first, then engage others   │
└──────────────┬───────────────────────────────────────────┘
               │
┌──────────────▼───────────────────────────────────────────┐
│ PHASE 4: TRACK & LEARN (automated)                       │
│                                                          │
│ Per post tracked in Scout:                               │
│   - Platform, pillar, topic, publish date                │
│   - Impressions, likes, comments, shares, saves          │
│   - Profile views generated                              │
│   - DMs/connection requests received (inbound leads)     │
│   - Link clicks (if Substack/GitHub linked)              │
│                                                          │
│ Inbound leads auto-created:                              │
│   New DM after post → Scout lead with                    │
│   source="content-linkedin" or "content-x"               │
│   → routed to appropriate pipeline (job/freelance/       │
│     consulting) based on who they are                    │
│                                                          │
│ Weekly pulse (Friday, part of Gate N2):                   │
│   "This week: 12K impressions, 340 engagements,          │
│    3 new DMs, 15 profile views. Builder Insights          │
│    posts outperformed Career posts 3:1."                  │
│                                                          │
│ Monthly retrospective (auto-generated):                   │
│   "Top post: [X] (4.2K impressions). Worst: [Y].        │
│    Pattern: posts with specific numbers in hooks          │
│    get 2.1x engagement. RAG topics drive most DMs.       │
│    Recommendation: increase Builder Insights to 50%,     │
│    reduce Career/Consulting to 15%."                     │
│                                                          │
│ Quarterly vertical assessment:                           │
│   "Q1 Legal AI deep-dives: 3 published, 450 total views,│
│    2 consulting leads, 1 advisory conversion.            │
│    ROI: strong. Continue Q2 Healthcare AI as planned."   │
└──────────────────────────────────────────────────────────┘
```

### Human Gates

- **Gate P1** (Sunday/Monday, 30 min): Review week's content batch — approve, edit, reschedule, drop
- **Gate P2** (Tue/Thu/Sat, 5 min each): Reply to comments on your posts — first-hour engagement boost

### AI Tools for Content

| Tool | Input | Output | Storage |
|---|---|---|---|
| `ContentTopicGen` | Week's work (commits, notes) + backlog + news | 3 topic suggestions with pillar + angle | Backlog in `~/.soul-v2/content-backlog.json` |
| `ContentSeriesGen` | Confirmed topic + raw insights/bullets | 3 LinkedIn posts + 3 X versions + carousel outline | `lead_artifacts` type=`"content_draft"` |
| `HookWriter` | Draft post | 5 hook variations using proven formulas | Inline with content draft |
| `ThreadConverter` | LinkedIn deep-dive post | X thread version (5-10 tweets, more technical) | `lead_artifacts` type=`"content_draft"` |
| `SubstackExpander` | Best-performing LinkedIn post of the month | Full 1500-3000 word article with SEO title | `lead_artifacts` type=`"content_draft"` |
| `EngagementReply` | Comment on your post + post context | Thoughtful reply that continues the conversation | Shown at Gate P2 |
| `ContentMetrics` | Post performance data | Weekly pulse + monthly retrospective + quarterly assessment | `lead_artifacts` type=`"content_metrics"` |
| `ReactiveContentGen` | News/paper/release + your expertise | Hot-take post draft for quick publishing | `lead_artifacts` type=`"content_draft"` |

---

## 6.5: Content Backlog & Topic Generation

### Evergreen Backlog

AI maintains a queue of 10-15 unused content ideas at `~/.soul-v2/content-backlog.json`.

**Sources for backlog ideas:**
- Past work not yet written about
- Papers saved but not yet commented on
- Analysis angles started but not finished
- Industry trends and predictions
- "What I wish I'd known" topics from experience
- Questions asked in communities (DMs, Discord, Reddit)

**Backlog lifecycle:**
- AI generates 2-3 new ideas per week (from your work + news)
- Ideas older than 60 days auto-archived (stale = not interesting enough)
- You can manually add ideas anytime
- When no fresh topic on Sunday → AI picks top backlog item

### Reactive Content Queue

AI monitors for reactive opportunities:
- New model releases (Anthropic, OpenAI, Google, Meta)
- Viral AI posts/threads (high engagement in your network)
- Papers in your domain (arXiv, conferences)
- Industry news (funding rounds, acquisitions, launches)
- Competitor claims you can counter with data

**Reactive posts are bonus content** — not part of the 3-post weekly series. When something big drops, you publish a quick reaction PLUS your scheduled posts. This positions you as a go-to voice for AI news.

---

## 6.6: Hook Writing System

The first line determines everything. LinkedIn shows only 2 lines before "see more."

### Proven Hook Formulas

| Formula | Template | Example |
|---|---|---|
| **Counterintuitive claim** | "[Common belief] is wrong. Here's why." | "More context doesn't make LLMs smarter. It makes them lazier." |
| **Specific number** | "[Big number]. [Time period]. Here's what happened." | "10,000 API calls. 3 models. The cost difference will shock you." |
| **Hard-won lesson** | "I [made expensive mistake] so you don't have to." | "I spent ₹40L on GPU infrastructure before learning this one thing." |
| **Provocative question** | "Is your [thing] actually [what it claims]?" | "Is your RAG pipeline actually retrieval-augmented — or just expensive search?" |
| **Confession** | "I [did something wrong] for [long time]." | "I over-engineered every AI project for 3 years before learning to ship." |
| **Contrarian** | "Everyone says [X]. I think the opposite." | "Everyone wants to fine-tune. Most of you shouldn't." |
| **Data reveal** | "I analyzed [X]. [Unexpected finding]." | "I analyzed 50 RAG deployments. 60% retrieve the wrong chunks." |
| **Before/after** | "Before: [bad state]. After: [good state]. Here's how." | "Before: 3-second API latency. After: 200ms. One architectural change." |

### Hook Rules

- First line must stand alone — makes sense without reading more
- No greeting ("Hey everyone!"), no preamble ("I've been thinking about...")
- Numbers > adjectives ("10,000 queries" > "a lot of queries")
- Tension > information (create curiosity gap, don't answer in the hook)
- Test: would you stop scrolling if you saw this line?

---

## 6.7: Content Formats by ROI

| Format | ROI | Effort (with AI) | Frequency | Best For |
|---|---|---|---|---|
| **Text posts** (LinkedIn) | Highest | 10 min review | 3x/week | All pillars |
| **Carousels** (LinkedIn PDF) | Very high (3-5x impressions) | 20 min review + design | 1x/week | Architecture diagrams, frameworks, comparisons |
| **X threads** | High for researchers | 5 min review | 2x/week | Technical deep-dives, paper reactions |
| **X single tweets** | Medium | 2 min review | 3x/week | Hot takes, reactions, hooks |
| **Short video** (LinkedIn native) | High (algo-favored) | 15 min record + review | 1x every 2 weeks | Demos, "here's what I built", screen recordings |
| **Substack article** | Long-term compounding (SEO) | 45 min review | 1x/month | Vertical deep-dives, comprehensive analyses |
| **GitHub repo/notebook** | Proof layer | Varies | When analysis is reproducible | Credibility signal for technical audience |

### Repurposing Pipeline

One piece of work becomes 7+ pieces of content:

```
Original analysis / experiment / build
  │
  ├→ LinkedIn Part 1 (hook)                   Tuesday
  ├→ LinkedIn Part 2 (deep dive + carousel)   Thursday
  ├→ LinkedIn Part 3 (opinion + question)     Saturday
  ├→ X thread (technical version of Part 2)   Wednesday
  ├→ X tweet (punchy version of Part 1)       Tuesday
  ├→ X hot take (version of Part 3)           Saturday
  ├→ Substack (monthly, expanded from best)   End of month
  ├→ Dev.to cross-post (Substack article)     Day after Substack
  ├→ GitHub notebook (if reproducible)         With Part 2
  └→ Short video (if demo-able)               Every 2 weeks
```

---

## 6.8: 30-Day Kickstart Plan

### Week 1: Foundation

| Day | Action |
|---|---|
| 1 | Set up Substack (name, branding, about page) |
| 1 | Optimize LinkedIn profile (see Section 7) |
| 2 | Write first 3 LinkedIn posts from recent work insights |
| 3 | Publish first LinkedIn post (Part 1 — hook) |
| 4 | Create LinkedIn carousel (architecture diagram or comparison) |
| 5 | Publish first X thread |
| 6-7 | Review engagement, respond to all comments |

### Week 2: Rhythm

| Day | Action |
|---|---|
| 8 | Publish Part 1 of second analysis |
| 9 | Start X thread habit (converted from LinkedIn Part 2) |
| 10 | Publish Part 2 with carousel |
| 11 | Follow 30-50 target people on X (researchers, founders, AI lab employees) |
| 12 | Publish Part 3 with open question |
| 13-14 | Review first week's metrics. What worked? |

### Week 3: Depth

| Day | Action |
|---|---|
| 15-17 | Third 3-part analysis series (full cycle) |
| 18 | Publish first Substack deep-dive (current quarter's vertical) |
| 19 | Cross-post to Dev.to |
| 20-21 | Engage in communities where your Substack topic is relevant |

### Week 4: Expand

| Day | Action |
|---|---|
| 22-24 | Fourth analysis series |
| 25 | First short LinkedIn video (60-90s demo/screen recording) |
| 26 | First reactive post (respond to news/paper/release) |
| 27 | Review month's metrics. Adjust pillar percentages if needed. |
| 28 | Generate next month's content calendar from backlog + vertical plan |

**By Day 30 you'll have:**
- 12+ LinkedIn posts published
- 8+ X threads/tweets
- 1 Substack article (SEO-indexed)
- 1 video
- Baseline metrics to iterate on
- Likely 3-5 recruiter/HM DMs from organic discovery

**Compounding kicks in at week 6-8.** Don't measure ROI before that.

---

## 6.9: Weekly Time Budget

| Activity | Time | When |
|---|---|---|
| Topic selection + raw insights | 30 min | Sunday |
| Review AI-generated content batch (Gate P1) | 30 min | Monday |
| Publish + engage Part 1 (Gate P2) | 10 min | Tuesday |
| Publish + engage Part 2 | 10 min | Thursday |
| Publish + engage Part 3 | 10 min | Saturday |
| Review weekly metrics (part of Friday review) | 5 min | Friday |
| Monthly: review Substack draft | 45 min | Last week of month |
| **TOTAL (weekly average)** | **~2 hrs/week** | |

### Combined Time Budget (All Strategies)

| Activity | Time/week |
|---|---|
| Job Applications (Section 3) | 3.5 hrs |
| Networking (Section 4) | 50 min |
| Freelance delivery (Section 5) | 15-18 hrs |
| Content (Section 6) | 2 hrs |
| **TOTAL** | **~22-25 hrs/week** |

Unpaid overhead (job search + networking + content): ~6.5 hrs/week.
Everything else is paid work or AI-automated.

---

## 6.10: Metrics & Feedback Loop

### Weekly Pulse (Friday, 5 min — part of existing Gate N2)

Scout shows:
- Total impressions (LinkedIn + X) this week
- Engagement rate per post
- Profile views generated
- New DMs/connection requests (flagged as inbound leads)
- Top-performing post and why (AI analysis)

### Monthly Retrospective (auto-generated)

Scout generates:
- All posts ranked by performance
- Pillar performance comparison (Builder Insights vs Technical Takes vs Career vs Personal)
- Pattern analysis ("posts with numbers in hooks get 2.1x engagement")
- Content → lead correlation ("RAG posts → 4 DMs, Career posts → 0 DMs")
- Recommended pillar rebalancing for next month
- Next month's content calendar (AI-generated, you review)

### Quarterly Vertical Assessment

Scout generates:
- This quarter's vertical deep-dives: views, leads, conversions
- SEO performance: which articles rank? For what keywords?
- Consulting leads traced to vertical content
- Recommendation: continue vertical / pivot / adjust
- Next quarter's vertical confirmation or change

### Content-to-Lead Tracking

Every post logged in Scout with:
- `type: "content"`, `platform`, `pillar`, `topic`, `publish_date`
- Metrics: `impressions`, `likes`, `comments`, `shares`, `saves`, `profile_views`
- Inbound leads generated (Scout auto-creates leads with `source: "content-{platform}"`)
- Lead pipeline routing: DM from recruiter → job pipeline, DM from founder → freelance/contract, DM from analyst → consulting

Scout correlates: "posts about RAG" → "4 inbound leads" → "do more RAG content."

---

## Key Rules

1. **"Good enough to publish beats perfect and unpublished."** Your analysis doesn't need to be peer-reviewed. It needs to be honest, specific, and based on real work.

2. **Start with work you're already doing.** Don't create new work just for content. Building something? Write about it. Ran an experiment? Share findings. Read a paper? Share your take.

3. **One idea, many formats.** Never create content from scratch for each platform. One analysis → 7+ pieces of content through the repurposing pipeline.

4. **Engagement > publishing.** A post with 10 thoughtful comment replies outperforms a post with 100 likes and no engagement. Reply to every comment in the first hour.

5. **Consistency > virality.** One post going viral means nothing if you disappear next week. 3 posts/week for 6 months beats 1 viral post.

---

## Implementation Notes

### Scout Integration

**New table: `content_posts`**
```sql
CREATE TABLE content_posts (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  platform TEXT NOT NULL,           -- "linkedin", "x", "substack", "devto"
  pillar TEXT NOT NULL,             -- "builder", "technical", "career", "personal"
  topic TEXT NOT NULL,
  content TEXT NOT NULL,
  hook TEXT,                        -- first line (for analysis)
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

Content metrics are stored per-post, not in the leads table. Inbound leads reference `content_posts.id` as their source.

**Content backlog:** `~/.soul-v2/content-backlog.json`
```json
[
  {
    "id": "uuid",
    "topic": "RAG chunk size impact on retrieval quality",
    "pillar": "builder",
    "source": "work",
    "angle": "Counter-intuitive: smaller chunks aren't always better",
    "created_at": "2026-03-18",
    "status": "pending"
  }
]
```

**Modified:** `internal/scout/runner/` — add content pipeline phases:
- TOPIC: generate topic suggestions from work + backlog + news
- GENERATE: create content series from confirmed topic
- ENGAGE: surface comment replies after publishing
- METRICS: update post metrics, generate weekly pulse

**New AI tools:** `internal/scout/ai/content_topic.go`, `internal/scout/ai/content_series.go`, `internal/scout/ai/hook_writer.go`, `internal/scout/ai/thread_converter.go`, `internal/scout/ai/substack_expander.go`, `internal/scout/ai/engagement_reply.go`, `internal/scout/ai/content_metrics.go`, `internal/scout/ai/reactive_content.go`

### Prerequisites

- Pipeline runner (`internal/scout/runner/`) — from job application spec
- `lead_artifacts` table — for storing content drafts
- Networking pipeline operational — Section 4 handles engagement on others' posts, coordinated timing with content publishing

### Relationship to Other Sections

- **Section 3 (Jobs):** Content drives recruiter/HM DMs → job pipeline inbound leads
- **Section 4 (Networking):** Content publishing is coordinated with networking engagement. Scout ensures: publish first → engage on your comments (Section 6) → engage on others' posts (Section 4). Same platforms, different ownership.
- **Section 5 (Freelance/Contracts/Consulting):** "I built X" posts → freelance inbound. Vertical Substack articles → consulting inbound. Case study posts → contract inbound. Content feeds ALL revenue streams.
- **Section 7 (Profile/Portfolio):** Content drives profile visits. Profile must convert visitors. Section 7 handles the conversion layer.
