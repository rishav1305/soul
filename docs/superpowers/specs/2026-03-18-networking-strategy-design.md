# Networking & Referral Strategy — Design Spec

**Date:** 2026-03-18
**Status:** Approved
**Scope:** Section 4 of `docs/scout-strategy.md` — unified networking across LinkedIn, X, email, and communities for all intents (job, freelance, contract, consulting, relationship)

---

## Overview

Unified relationship system with channel-aware playbooks. Every meaningful contact tracked in Scout as a lead in the `referral` pipeline. AI automates target identification, outreach drafting, interaction tracking, and warmth scoring. Human reviews in batches (2x/week outreach, 1x/week networking brief). Communities follow an observe-first approach with no timeline pressure.

## Design Decisions

| Decision | Choice | Why |
|---|---|---|
| Networking intent | Hybrid B+C: lead with value always, know preferred outcome per target type | Builds genuine relationships while being strategically aware |
| Channel strategy | LinkedIn primary outbound, X engagement, email freelance/consulting, communities observe-first | Each channel has different norms and strengths |
| Tracking | Full CRM in Scout (Approach A) — every contact in referral pipeline | With AI doing the work, full tracking isn't burdensome. Nothing falls through cracks. |
| Community approach | Observe-first, no timeline pressure (Approach C) | User has zero community engagement experience. Avoid missteps and bans. |
| Referral timing | 5-6 value exchanges minimum for AI labs/dream companies, 2-3 for startup founders | Natural relationship building, never transactional |
| Volume | Aggressive on controlled channels (LinkedIn 15-20/wk, X 10-15/wk, email 3-5/wk) | AI handles drafting, human only reviews |

---

## 4.1: Contact Types & Intent Mapping

### Contact Taxonomy

| Contact Type | Primary Intent | What Success Looks Like |
|---|---|---|
| Hiring Manager (at AI lab/startup) | Job | Referral or direct interview |
| Founder / CTO (startup, Series A-C) | Freelance / Contract | Project or team engagement |
| Peer Engineer (senior IC at target co) | Referral + Mutual | They refer you, you help them |
| AI Researcher | Relationship | Collaboration, they think of you when lab hires |
| VC / Investor | Intel | They tell you who's hiring before it's posted |
| Community Leader (moderator, organizer) | Visibility | They amplify your work, introduce you to people |
| Dev Advocate (at AI companies) | Amplification | They share your content, connect you internally |
| Content Creator (AI/tech journalists) | Cross-promotion | Mutual audience growth, inbound opportunities |

### Approach Per Contact Type

Always lead with value. Intent determines where you steer when conversation opens naturally.

| Contact Type | Min Value Exchanges Before Ask | The Ask (when natural) |
|---|---|---|
| AI Lab employee | 5-6 | "I'd love a full-time role" |
| Startup founder/CTO | 2-3 (they appreciate efficiency) | "I can help as contractor/team" |
| Peer engineer | 4-5 | "I'm exploring opportunities" |
| VC / Community leader / Dev advocate | Never ask directly | Stay visible, they connect people naturally |

### Implementation

**Modified:** `internal/scout/store/` — add fields to leads table for contacts:
- `contact_type TEXT` — hiring_manager, founder, peer, researcher, vc, community_leader, dev_advocate, content_creator
- `intent TEXT` — job, freelance, contract, consulting, relationship, intel, visibility
- `warmth TEXT DEFAULT 'new'` — new, active, warm, ready, dormant
- `interaction_count INTEGER DEFAULT 0`
- `channels TEXT` — JSON array of channels: `["linkedin", "x", "email", "discord"]`
- `last_interaction_at TEXT` — ISO-8601 timestamp

All 6 new fields must be added to `allowedLeadFields` for PATCH updates. `AddLead` must set defaults for these fields.

**Migration:** These columns are added in the same `ALTER TABLE` migration as the `tier` column from the job application spec. Single combined migration — both specs' schema changes in one pass:
```sql
ALTER TABLE leads ADD COLUMN tier INTEGER NOT NULL DEFAULT 3;
ALTER TABLE leads ADD COLUMN contact_type TEXT DEFAULT '';
ALTER TABLE leads ADD COLUMN intent TEXT DEFAULT '';
ALTER TABLE leads ADD COLUMN warmth TEXT DEFAULT 'new';
ALTER TABLE leads ADD COLUMN interaction_count INTEGER DEFAULT 0;
ALTER TABLE leads ADD COLUMN channels TEXT DEFAULT '[]';
ALTER TABLE leads ADD COLUMN last_interaction_at TEXT DEFAULT '';
```
Migration runs once at startup if columns don't exist (use `PRAGMA table_info` to check).

**New table: `interactions`** — tracks individual interaction events for audit trail:
```sql
CREATE TABLE interactions (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  lead_id INTEGER NOT NULL REFERENCES leads(id),
  type TEXT NOT NULL,      -- "value_giving", "value_receiving", "milestone"
  channel TEXT NOT NULL,   -- "linkedin", "x", "email", "discord", "in_person"
  description TEXT,        -- brief note of what happened
  created_at TEXT NOT NULL DEFAULT (datetime('now'))
);
CREATE INDEX idx_interactions_lead ON interactions(lead_id);
```
This enables the AI to reason about interaction balance ("4 value-giving, 0 receiving — relationship may be one-sided").

**Channel filtering:** Use SQLite `json_each` for channel queries:
```sql
SELECT * FROM leads WHERE EXISTS (SELECT 1 FROM json_each(channels) WHERE value = 'linkedin')
```

**Networking pipeline** — separate from the job `referral` pipeline:

The `referral` pipeline (from job application spec) tracks job-specific referral outreach: `identified → connected → conversation → referral-asked → referred → interviewing → offer`. This is for contacts tied to a specific job lead.

Networking contacts use a NEW `networking` pipeline suited to general relationship building:
```go
"networking": {
  Stages:   []string{"identified", "connected", "engaging", "warm", "converting", "converted"},
  Terminal: []string{"converted", "inactive", "not-relevant"},
}
```

| Stage | Meaning |
|---|---|
| `identified` | Contact created. No outreach yet. |
| `connected` | Connection accepted (LinkedIn) or first response (email/DM). |
| `engaging` | Active value exchanges happening. |
| `warm` | Real rapport established. Coffee chat territory. |
| `converting` | Ask made or opportunity surfaced. In discussion. |
| `converted` | Outcome achieved (job referral, freelance gig, consulting call, etc.). Terminal. |
| `inactive` | Lost interest or no longer relevant. Terminal. |
| `not-relevant` | Contact type mismatch or not useful. Terminal. |

**Relationship between warmth and pipeline stage:**
- Warmth is **computed** from `interaction_count` and `last_interaction_at`. It is a derived metric, not a manually managed state.
- Pipeline stage is **explicit** — advanced by the pipeline runner or human action.
- They correlate but don't govern each other:
  - A contact can be at stage `engaging` with warmth `warm` (normal — engaging for a while)
  - A contact can be at stage `connected` with warmth `active` (just connected, had a couple exchanges)
  - A contact CANNOT be at stage `converting` with warmth `new` (the runner prevents advancing to `converting` unless warmth >= `ready` for job/peer intents, or >= `active` for founder/CTO intents)
- The pipeline runner uses warmth as a **gate for stage advancement**, not as the stage itself.
- The AI tool `NetworkingDraft` uses BOTH: warmth determines tone/aggression, stage determines what type of draft to generate.

**Update `knownPipelines`** in `internal/scout/store/analytics.go` to include `"networking"` alongside `"referral"`.

These fields coexist with existing lead fields. Job leads use the `job` pipeline. Job-specific referrals use the `referral` pipeline. General networking contacts use the `networking` pipeline. Same table, different pipeline types.

---

## 4.2: Channel Strategy

### LinkedIn Playbook (Aggressive, AI-Driven)

**Volume:** 15-20 connection requests/week + follow-ups + comments

**Target Identification:**
- TheirStack leads → extract HM name + LinkedIn URL (already enriched)
- Dream company list → AI identifies Head of AI, senior IC, mutual connections
- Startup founders/CTOs → for freelance/contract angle

**Connection Request (AI-drafted, you review):**

Rules:
- NEVER mention job/work intent in the request
- ALWAYS reference something specific from their profile/posts
- Under 300 characters

Templates by contact type:

| Contact Type | Template |
|---|---|
| Researcher | "Hi [Name] — your work on [specific paper/project] is really interesting. I'm working on similar problems in production. Would love to connect." |
| Engineer | "Hi [Name] — noticed you're building [specific thing] at [Company]. I'm deep in the same space. Would be great to exchange notes." |
| Founder | "Hi [Name] — [Company]'s approach to [specific product angle] caught my eye. Building in a similar direction. Would love to connect." |
| VC/Advocate/Leader | "Hi [Name] — your [post/talk/article] on [topic] resonated with my work. Would love to have you in my network." |

**Post Engagement (AI-drafted):**
- AI monitors targets' recent LinkedIn posts
- Drafts meaningful comments (not "great post!" — adds genuine insight or asks a smart question)
- 10-15 comments/week across target contacts

### X/Twitter Playbook (Strategic Engagement)

**Volume:** 10-15 thoughtful replies/week

**Strategy:** Public engagement that builds familiarity. NOT cold DMs.

1. Follow 30-50 target people (researchers, founders, AI lab employees)
2. AI monitors their tweets daily
3. AI drafts thoughtful replies that add genuine insight from your experience
4. You review and post in 5-min batches (Mon/Wed/Fri)
5. After 3-4 public interactions → DM is natural

**AI Draft Quality Rule:** Every reply must add information, a counterpoint, or a specific experience. Never generic agreement.

### Email Playbook (Freelance/Consulting Outreach)

**Volume:** 3-5 cold emails/week

**Targets:** Startups that need AI help but haven't posted a role. Companies from TheirStack where contract/freelance fits better than full-time.

**Strategy:**
1. AI researches company → identifies specific gap or opportunity
2. AI drafts email leading with a "quick win" observation
3. You review and send (10 min Thu)

**Email Template:**
```
Subject: Quick thought on [Company]'s [specific thing]

Hi [Name],

I noticed [specific observation about their product/tech —
e.g., "your docs search could benefit from RAG" or
"your API response times suggest an opportunity for caching"].

I've been building [relevant thing] and thought this might
be useful context. Happy to share more if interesting.

[Your name]
```

Rule: No pitch in email 1. Just value. If they respond, THEN discuss engagement.

### Community Playbook (Observe-First, Your Pace)

**No volume targets. No timeline. Your pace.**

| Phase | When You're Ready | Actions |
|---|---|---|
| Observe | Weeks 1-2 | Join. Read conversations. Note culture: what gets upvoted? Who are regulars? |
| Low-risk | Weeks 3-4 | Answer a technical question you're confident about. React/emoji to good posts. |
| Natural | Week 5+ | Share insights from your work. Help debug problems. Post in job channels if they exist. |
| Connected | When comfortable | DM someone whose question you answered publicly. Share Substack if relevant to a discussion. |

**Never:**
- Cold DM someone you haven't interacted with publicly first
- Self-promote in general channels
- Post "I'm looking for work" outside designated channels
- Copy-paste the same message across communities

---

## 4.3: The Referral Journey (Value-First, Multi-Intent)

### Interaction Types

**Value-giving (you → them):**
- Comment on their LinkedIn post
- Reply to their X thread with insight
- Share a resource relevant to their work
- Answer their question in a community
- Introduce them to someone in your network
- Share analysis/insight relevant to their domain

**Value-receiving (them → you):**
- Comment on your post
- Accept connection request
- Respond to your message
- Share your content
- Introduce you to someone

**Milestone (relationship deepeners):**
- Coffee chat / video call
- Collaborated on something
- Met in person
- They proactively reached out to you

### Warmth Scoring

| Level | Interaction Count | Meaning | Suggested Action |
|---|---|---|---|
| NEW | 0-1 | Just connected. No relationship. | Engage with their content, add value |
| ACTIVE | 2-3 | They know your name. Some back-and-forth. | Keep engaging, look for deeper conversation. Founder/CTO ask OK at this level. |
| WARM | 4-6 | Real rapport. They engage proactively. | Coffee chat appropriate. Peer engineer ask OK at this level. |
| READY | 7+ | Genuine relationship. Mutual respect. | Natural opening for the right ask. AI lab / dream company ask OK. They may offer first. |
| DORMANT | No new interaction in 30+ days | Relationship cooling. Based on `last_interaction_at` being older than 30 days, regardless of total `interaction_count`. | AI surfaces re-engagement: react to recent post, share something relevant |

Warmth is computed automatically: `interaction_count` determines the level, `last_interaction_at` determines dormancy. Dormancy overrides any other level — a contact with 10 past interactions but no activity in 30 days is DORMANT until re-engaged.

### The Ask — Contextual, Never Scripted

| Situation | Min Warmth | Min Interactions | Ask Style |
|---|---|---|---|
| AI lab / dream company (job) | READY | 7+ | Mention role naturally. "I saw [Company] is hiring for [X]. Is the team good?" Let them offer. |
| Company with open role (job) | WARM | 5+ | Same approach, slightly lower bar for non-dream companies. |
| Company with open role | < WARM | <5 | Don't ask yet. Keep building. Day 8 fallback to career page (Section 3). |
| Dream company, no open role | Any | Any | Build relationship. No ask. When role opens, you're already warm. |
| Startup founder/CTO (freelance/contract) | ACTIVE | 2-3 | "I noticed you're scaling the AI team. I do [X] as a contractor — happy to chat if useful." |
| Peer engineer (referral) | WARM | 4-5 | "I'm exploring opportunities in this space." Let them connect the dots. |
| VC / Community leader | Never | N/A | Never ask directly. Stay visible. They connect people naturally. |
| Inbound (they reached out) | N/A | N/A | Respond within 24h. Understand their needs. Match your offering to their situation. |

### Implementation

**New AI tool:** `internal/scout/ai/networking.go`

`NetworkingDraft(ctx, contactID, channel, activityContext string)` — generates channel-appropriate outreach/engagement draft based on:
- Contact type and intent
- Warmth level and interaction history (from `interactions` table)
- Activity context (passed as string — see below)
- Relationship context

**External activity sourcing:** The AI tools do NOT fetch LinkedIn posts or X tweets directly. Instead:
- The pipeline runner's GENERATE phase calls a lightweight helper that constructs a prompt asking Claude to "draft outreach for [person] at [company] who works on [their role/description]"
- For LinkedIn comments: the human provides the post URL/content when reviewing at Gate N1, or the draft is generic ("comment on their recent post about [topic from their profile]")
- For X replies: the pipeline runner includes the contact's last known activity (stored in `interactions.description` when the user logs an interaction)
- No external API integration required. Activity context comes from what's already stored + Claude's knowledge of the person/company.

`WeeklyNetworkingBrief(ctx)` — generates weekly summary:
- Contacts that moved to WARM (coffee chat candidates)
- Contacts that went DORMANT (re-engagement suggestions with re-engagement draft)
- Contacts at READY (ask window open, with suggested ask per intent)
- Interaction balance alerts ("4 value-giving, 0 receiving — consider different approach")
- This week's target list with drafted outreach

**Pipeline runner ownership:** The `internal/scout/pipeline/` package is created by the job application spec (defines `runner.go` with the main polling loop). This networking spec extends it with `networking.go` which adds networking-specific phases to the same runner loop. The runner calls both job pipeline phases and networking pipeline phases in sequence.

**Prerequisite from job application spec:** The pipeline runner, `lead_artifacts` table, `ValidateTransition` enforcement in server handlers, and `allowedLeadFields` updates must be implemented first. This networking spec depends on all of those.

---

## 4.4: AI-Driven Networking Pipeline

### Pipeline Flow

**Phase 1: IDENTIFY TARGETS (automated, weekly)**

Sources:
- TheirStack leads → extract HM, company employees
- Dream company list → identify key people
- X/Twitter → researchers posting about relevant topics
- Communities → people asking/answering AI questions
- Inbound → profile viewers, post engagers

AI creates contact records: name, company, role, channel, contact_type, intent, warmth="new"

**Phase 2: GENERATE OUTREACH (automated, daily)**

- LinkedIn: connection requests, follow-ups, comment drafts for targets' posts
- X/Twitter: thoughtful reply drafts for targets' tweets
- Email: "quick win" outreach for freelance/consulting targets
- All drafts queued for review

**Gate N1: NETWORKING REVIEW (you, ~20 min, Tue/Thu)**

Scout shows today's networking batch: "8 LinkedIn drafts, 5 X replies, 2 emails ready"

For each draft:
- Contact name, company, type, warmth level
- Draft message
- Context (why this person, what they posted recently)

Actions: `[Send]` `[Edit & Send]` `[Skip]` `[Snooze 1 week]`

**Phase 3: TRACK & RE-ENGAGE (automated)**

After each interaction:
- interaction_count incremented
- warmth level recalculated
- next suggested action generated

**Gate N2: WEEKLY NETWORKING REVIEW (you, 10 min, Fri)**

Weekly networking brief:
- "3 contacts moved to WARM — consider coffee chat"
- "5 contacts DORMANT — re-engagement suggestions ready"
- "2 contacts at READY — ask window open"

Approve coffee chat requests (AI-drafted).

**Phase 4: CONVERT (human, when ready)**

When contact reaches READY + natural opening:
- AI suggests the right ask based on contact_type and intent
- AI drafts the message
- You review and decide: make the ask, keep building, or not a fit

**Gate N3: CONVERSION DECISION (you, as needed)**

Review AI suggestion. Decide and send.

### Implementation

**New package:** `internal/scout/pipeline/networking.go`

Extends the pipeline runner from the job application spec:
- Runs alongside job pipeline phases
- Manages contact warmth progression
- Generates daily outreach queue
- Generates weekly networking brief
- Surfaces Gate N1/N2/N3 items in Scout Actions tab

**Modified:** Scout frontend Actions tab
- "Networking Outreach" section (drafts for review)
- "Networking Brief" section (weekly warmth report)
- "Conversion Opportunities" section (READY contacts with suggested asks)

---

## 4.5: Weekly Throughput

### Channel Volume

| Channel | Volume/week | Your Time |
|---|---|---|
| LinkedIn connects | 15-20 | 10 min review |
| LinkedIn follow-ups | all pending | 5 min review |
| LinkedIn comments | 10-15 | 5 min review |
| X/Twitter replies | 10-15 | 10 min review |
| Cold emails | 3-5 | 5 min review |
| Coffee chat requests | 1-2 | 10 min review + schedule |
| Community | your pace | when comfortable |
| **TOTAL** | **40-60 touchpoints** | **~50 min/week** |

### Contact Growth Targets (Monthly)

| Contact Type | Monthly | Intent |
|---|---|---|
| Hiring Managers | 10-15 | Job |
| Founders / CTOs | 8-10 | Freelance / Contract |
| Peer Engineers | 10-15 | Referral + mutual |
| Researchers | 5-8 | Relationship |
| VCs / Investors | 3-5 | Intel |
| Community Leaders | 3-5 | Visibility |
| Dev Advocates | 3-5 | Amplification |
| **TOTAL** | **42-63** | |

### Expected Warmth Funnel

| Month | Total | NEW | ACTIVE | WARM | READY | DORMANT |
|---|---|---|---|---|---|---|
| 1 | ~50 | 40 | 8 | 2 | 0 | 0 |
| 2 | ~100 | 50 | 30 | 15 | 5 | 0 |
| 3 | ~150 | 50 | 40 | 35 | 20 | 5 |

By month 3: 20 contacts at READY = 20 potential conversations across job/freelance/contract/consulting.

### Combined Time Budget (All Strategies)

| Activity | Time/week | Source |
|---|---|---|
| Job Application Pipeline | 3.5 hrs | Section 3 |
| Networking Pipeline | 50 min | Section 4 |
| Content (3-part series) | 2-3 hrs | Section 7 |
| **TOTAL** | **~7 hrs/week** | **~1 hr/day** |

Everything else is AI-automated.

---

## Relationship to Other Sections

- **Section 3 (Job Applications):** Lead-attached outreach follows Section 3's time-boxing cadence. Relationship outreach follows Section 4's warmth-based approach. Same contact can be in both flows.
- **Section 5 (Closed Market):** Section 4 IS the strategy for cracking the closed market. Section 5 becomes the "why" (data/stats), Section 4 becomes the "how."
- **Section 6 (Freelance/Contract):** Section 4 handles the networking angle (finding clients via relationships). Section 6 handles platform-specific execution (Upwork proposals, etc.).
- **Section 7 (Content):** Content drives inbound contacts. Section 4 tracks and nurtures those inbound relationships.
