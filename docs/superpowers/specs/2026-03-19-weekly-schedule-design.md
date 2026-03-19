# Weekly Operating Schedule — Design Spec

**Date:** 2026-03-19
**Status:** Approved
**Scope:** `docs/scout/schedule.md` — daily rhythm, day-by-day task distribution (AI vs human), weekly/monthly targets, adaptive trust model, Scout dashboard design.

---

## Overview

Distributed daily schedule where AI works continuously and queues outputs for async human review. No rigid time blocks — gates are checkpoints, not appointments. Seven days a week (aggressive growth mode). Priority-ordered dashboard with per-section tabs. Leading + lagging metrics per section. Adaptive trust model: full review Month 1 → selective Month 2-3 → trust with audits Month 4+.

## Design Decisions

| Decision | Choice | Why |
|---|---|---|
| Schedule structure | Distributed throughout day | AI works continuously. You verify asynchronously when available. Gates are checkpoints. |
| Weekends | No days off | Aggressive growth. AI doesn't rest. Content publishes Sat. Planning on Sun. |
| Monitoring | Adaptive trust (B+C) | Full review Month 1 (build trust). Loosen over time. Tighten if quality drops. |
| Dashboard | Priority Queue tab + per-section tabs | One screen to see everything urgent. Deep-dive per pipeline when needed. |
| Success metrics | Leading + lagging per section | Leading = "did I do the work?" Lagging = "did it produce results?" Track both. |

---

## 8.1: Daily Operating Rhythm

### Morning (8-10 AM IST)

**Optimal for:** Publishing (LinkedIn audience awake), reviewing overnight AI outputs

**AI has prepared overnight:**
- Job leads scored + resumes tailored (Gate 1)
- Freelance gigs scored + proposals drafted (Gate F1)
- Networking outreach drafts (Gate N1, on Tue/Thu)
- Content ready to publish (Gate P2, on Tue/Thu/Sat)
- Contract opportunities pitched (Gate C1, on Mon)
- Consulting call prep briefs (Gate E1, when scheduled)

**You do:**
1. Open Priority Queue tab — work top to bottom
2. Review + approve/edit AI outputs
3. Publish content (if scheduled today)
4. Send approved outreach (LinkedIn, X, email)

**Time:** 45-60 min

### Midday (12-2 PM IST)

**Optimal for:** Engagement (morning post has comments), applications (career pages active)

**AI has prepared:**
- Comment reply drafts on your posts (Gate P2)
- Application materials ready (resume + cover letter)
- Follow-up reminders (cadence alerts)

**You do:**
1. Reply to comments on your posts (algorithm boost)
2. Submit job applications (career pages)
3. Send freelance proposals (Upwork, Contra)
4. Handle any follow-ups due today

**Time:** 20-30 min

### Afternoon-Evening (flexible)

**Focus:** PAID WORK (freelance delivery, consulting, contracts)

AI doesn't interrupt except for urgent items:
- Expert call in 2 hours → prep brief ready
- Client message needs response
- Inbound DM from hot lead

**Time:** 4-6 hrs (freelance delivery)

### Evening (9-10 PM IST)

**Optimal for:** US daytime engagement, planning

**AI has prepared:**
- End-of-day summary: what happened today
- Tomorrow's priority queue preview
- Stale items needing attention

**You do:**
1. Quick scan of what moved today
2. Log interactions (networking contacts, calls done)
3. Input data for AI (call notes, project updates)
4. Research/analysis for content (optional)

**Time:** 15-20 min

### Daily Total: ~1.5-2 hrs admin + 4-6 hrs paid work

---

## 8.2: Weekly Schedule — Day by Day

### Monday — Discovery Day

| Who | Task | Expected Output |
|---|---|---|
| **AI** (overnight) | TheirStack sweep + tier classification | 20-30 new leads with tier 1/2/3 |
| **AI** (overnight) | Score Tier 1/2 leads | Match scores on all T1/T2 |
| **AI** (overnight) | Generate job prep artifacts | Tailored resumes + covers + outreach |
| **AI** (overnight) | Score freelance gigs | Top matches with proposals |
| **AI** (overnight) | Generate content series | Week's 3-part series ready |
| **AI** (overnight) | Identify contract targets | Companies hiring 3+ AI roles |
| **YOU** (morning) | Gate 1: Job batch review | 3-5 leads approved |
| **YOU** (morning) | Gate C1: Contract review | 2-3 targets pursued or skipped |
| **YOU** (morning) | Gate P1: Content batch review | Week's posts approved/scheduled |
| **YOU** (morning) | Gate F1: Freelance proposals | 3-5 proposals approved |
| **YOU** (midday) | Send proposals + submit apps | Proposals out, 1-2 apps submitted |
| **YOU** (afternoon) | Freelance delivery | Client work progress |
| **YOU** (evening) | Log interactions, input data | Scout data updated |

**Monday Output:** Week planned, leads classified, content scheduled, contracts reviewed, proposals sent.
**Your Time:** Admin ~1.5 hrs | Paid work 4-6 hrs

### Tuesday — Outreach Day

| Who | Task | Expected Output |
|---|---|---|
| **AI** | Networking outreach drafts | 8-10 LinkedIn notes + follow-ups |
| **AI** | X reply drafts | 5-7 thoughtful replies |
| **AI** | Score new freelance gigs | Fresh gigs scored + drafted |
| **AI** | Comment reply drafts | Replies for any prior post comments |
| **YOU** (morning) | Publish Part 1: Hook Post | LinkedIn + X tweet live |
| **YOU** (morning) | Gate N1: Networking review | 8-10 connections + follow-ups sent |
| **YOU** (morning) | Gate F1: Freelance proposals | 2-3 more proposals sent |
| **YOU** (midday) | Gate P2: Reply to comments | 5-10 comment replies (algo boost) |
| **YOU** (midday) | Send X replies | 5-7 engagement replies on X |
| **YOU** (midday) | Submit Tier 2 applications | 1-2 career page apps + HM outreach |
| **YOU** (afternoon) | Freelance delivery | Client work progress |

**Tuesday Output:** Content Part 1 live, outreach sent, X engagement active, proposals submitted.
**Your Time:** Admin ~1.5 hrs | Paid work 4-6 hrs

### Wednesday — Follow-up Day

| Who | Task | Expected Output |
|---|---|---|
| **AI** | Follow-up drafts for accepted connections | Messages for last week's accepts |
| **AI** | Cadence checks (all pipelines) | Day 3/5/8 reminders surfaced |
| **AI** | Score new freelance gigs | Fresh gigs scored |
| **YOU** (morning) | Publish X thread (Part 2 preview) | Technical thread live |
| **YOU** (morning) | Gate 2: Job follow-up review | Follow-ups sent + status updated |
| **YOU** (morning) | Gate F1: Freelance proposals | 2-3 proposals sent |
| **YOU** (midday) | Send networking follow-ups | Follow-up messages sent |
| **YOU** (midday) | Respond to freelance clients | Conversations progressed |
| **YOU** (afternoon) | Freelance delivery | Client work progress |
| **YOU** (evening) | Research/analysis for content | Raw insights (optional) |

**Wednesday Output:** Follow-ups sent, X thread live, cadence progressed.
**Your Time:** Admin ~1 hr | Paid work 4-6 hrs

### Thursday — Application Day

| Who | Task | Expected Output |
|---|---|---|
| **AI** | Networking outreach drafts | 5-7 outreach drafts |
| **AI** | Cold email drafts | 3-5 freelance/consulting emails |
| **AI** | Score freelance gigs | Fresh gigs scored |
| **AI** | Comment reply drafts | Replies for Tue post comments |
| **YOU** (morning) | Publish Part 2: Deep Dive | LinkedIn deep dive + carousel live |
| **YOU** (morning) | Gate N1: Networking review | 5-7 outreach + cold emails sent |
| **YOU** (morning) | Gate F1: Freelance proposals | 2-3 proposals sent |
| **YOU** (midday) | Gate P2: Reply to comments | Comment replies posted |
| **YOU** (midday) | Submit Tier 2/3 applications | 2-3 career page applications |
| **YOU** (midday) | Send cold emails | 3-5 emails sent |
| **YOU** (afternoon) | Freelance delivery | Client work progress |
| **YOU** (evening) | Consulting calls (if scheduled) | Call completed |

**Thursday Output:** Deep dive live, applications submitted, outreach sent, cold emails out.
**Your Time:** Admin ~1.5 hrs | Paid work 4-6 hrs

### Friday — Review Day

| Who | Task | Expected Output |
|---|---|---|
| **AI** | Weekly networking brief | Warmth changes, dormant, ready |
| **AI** | Content weekly pulse | Post performance + recommendations |
| **AI** | Identify stale leads | Leads needing decision |
| **AI** | Coffee chat request drafts | 1-2 drafts for Tier 1 targets |
| **AI** | Score freelance gigs | Fresh gigs scored |
| **YOU** (morning) | Gate F1: Freelance proposals | 2-3 proposals sent |
| **YOU** (morning) | Gate 1: Job leads (catch-up) | Any remaining leads reviewed |
| **YOU** (midday) | **FRIDAY REVIEW SESSION (30 min):** | |
| | Gate N2: Networking brief (10 min) | Warmth decisions made |
| | P-metrics: Content pulse (5 min) | Performance noted, adjustments |
| | Gate 3: Stale lead decisions (15 min) | Stale resolved: fallback/skip/withdraw |
| | Coffee chat requests reviewed | 1-2 requests sent |
| **YOU** (afternoon) | Freelance delivery | Client work progress |
| **YOU** (evening) | Weekly reflection (5 min) | What worked, what to adjust |

**Friday Output:** Week reviewed, stale resolved, networking actioned, performance noted.
**Your Time:** Admin ~1.5 hrs | Paid work 4-6 hrs

### Saturday — Content + Build

| Who | Task | Expected Output |
|---|---|---|
| **AI** | Score new freelance gigs | Fresh gigs scored |
| **AI** | Comment reply drafts | Replies for Thu post comments |
| **YOU** (morning) | Publish Part 3: Takeaway | Opinion post + X hot take live |
| **YOU** (morning) | Gate P2: Reply to comments | Comment replies posted |
| **YOU** (morning) | Gate F1: Freelance (light) | 1-2 cherry-pick proposals |
| **YOU** (afternoon) | Freelance delivery OR GitHub | Client work OR repo polished |
| **YOU** (evening) | Community participation (opt.) | Read/participate when comfortable |

**Saturday Output:** Content Part 3 live, GitHub progress, community engagement.
**Your Time:** Admin ~30 min | Paid/build work 4-8 hrs

### Sunday — Planning + Deep Work

| Who | Task | Expected Output |
|---|---|---|
| **AI** | 3 topic suggestions for next week | Topics with pillar + angle + hooks |
| **AI** | Refresh content backlog | 2-3 new ideas from this week's work |
| **AI** | Score any new freelance gigs | Weekend gigs scored |
| **YOU** (morning) | Pick next week's content topic | Topic confirmed, raw insights to AI |
| **YOU** (morning) | Gate F1 (light): hot freelance | 1-2 proposals if strong matches |
| **YOU** (afternoon) | Deep work: research, analysis, experiments, building | Data/results that feed content + GitHub |
| **YOU** (evening) | Freelance delivery (if active) | Client work progress |

**Sunday Output:** Next week's topic confirmed, deep work done, AI generating series overnight.
**Your Time:** Admin ~30 min | Deep/paid work 4-8 hrs

---

## 8.3: Weekly Targets (Leading + Lagging, Per Section)

### Section 3: Jobs

| Leading (activity) | Target | Lagging (outcomes) | Target |
|---|---|---|---|
| Leads reviewed | 20-30 | Responses received | 3-5 |
| Applications submitted | 5-8 | Interviews scheduled | 1-2 |
| Tier 1 warm approaches | 3 | Referrals obtained | 1 |
| Tier 2 HM outreaches | 4-5 | Screening calls | 1-2 |
| Resumes tailored | 8-10 | | |
| Cover letters generated | 5-8 | | |

### Section 4: Networking

| Leading | Target | Lagging | Target |
|---|---|---|---|
| LinkedIn connects sent | 15-20 | Connections accepted | 8-12 |
| LinkedIn comments posted | 10-15 | Follow-up responses | 3-5 |
| X replies posted | 10-15 | Coffee chats completed | 1 |
| Cold emails sent | 3-5 | Warmth promotions | 2-3 |
| Follow-ups sent | All pending | Email responses | 1-2 |
| Coffee chat requests | 1-2 | | |

### Section 5: Freelance

| Leading | Target | Lagging | Target |
|---|---|---|---|
| Gigs reviewed | 15-25 | Proposals shortlisted | 2-3 |
| Proposals sent | 3-5/day | Gigs awarded | 1-2 |
| Client conversations | 2-3 | Revenue | tracked |
| Delivery hours | 12-15 | Reviews received | tracked |

### Section 5: Contracts

| Leading | Target | Lagging | Target |
|---|---|---|---|
| Targets identified | 3-5 | Discovery calls booked | 1 |
| Outreach sequences active | 3-5 | SOWs sent | tracked |
| Upsell opps reviewed | All | Contracts signed | tracked |
| Pitches generated | 2-3 | Revenue | tracked |

### Section 5: Consulting

| Leading | Target | Lagging | Target |
|---|---|---|---|
| Expert calls accepted | All | Calls completed | 2-4/month |
| Prep briefs reviewed | All | Advisory proposals accepted | tracked |
| Follow-ups sent | All post-call | Project consulting booked | tracked |
| Upsell evaluations | All | Revenue | tracked |

### Section 6: Content

| Leading | Target | Lagging | Target |
|---|---|---|---|
| LinkedIn posts published | 3 | Total impressions | tracked |
| X posts/threads published | 3 | Engagement rate | 3-5% |
| Comments replied | All within 60 min | Profile views | tracked |
| Substack | 1/month | Inbound DMs | tracked |
| Carousel | 1/week | Content → leads | tracked |

### Section 7: Profile

| Leading | Target | Lagging | Target |
|---|---|---|---|
| Profile updates applied | All triggered | LinkedIn search appearances | tracked |
| GitHub commits | 4+ days/week | Portfolio pageviews | tracked |
| | | GitHub stars/forks | tracked |

**Note:** "tracked" = no weekly target. Lagging metrics take time to compound. Leading metrics should be hit EVERY week. Don't panic if lagging metrics are slow in Month 1-2.

---

## 8.4: Monthly Targets

| Section | Key Monthly Metrics |
|---|---|
| **Jobs** | 20-32 applications, 4-8 interviews, 4+ referrals |
| **Networking** | 42-63 new contacts, 8-12 warmth promotions, 4 coffee chats |
| **Freelance** | 60-100 proposals, 4-8 gigs awarded, 4-5 reviews |
| **Contracts** | 12-20 targets pitched, 2-4 discovery calls |
| **Consulting** | 12 network applications (month 1), 2-4 expert calls (month 3+) |
| **Content** | 12-15 LinkedIn posts, 12-15 X posts, 1 Substack, 2 videos |
| **Profile** | 16+ commit days, case studies as completed, testimonials as received |

### Revenue Trajectory

| Period | Freelance | Contracts | Consulting | Total |
|---|---|---|---|---|
| Month 1-2 | ₹50K-1.5L | ₹0 | ₹0 | ₹50K-1.5L |
| Month 3-4 | ₹1.5L-3L | ₹1-3L | ₹20-60K | ₹2.7L-6.6L |
| Month 5-6 | ₹3-5L | ₹3-8L | ₹70K-2.5L | ₹4.7L-15.5L |
| Month 6+ | ₹3-5L | ₹8-15L | ₹1.5-5L | ₹7.5L-18L+ |

---

## 8.5: Adaptive Trust Model

### Month 1: Full Review

Review EVERY AI output before it goes out.

**Watch for:** Resume keyword matching quality, proposal specificity vs generic, outreach human-ness, hook provocativeness, scoring accuracy.

**Keep notes:** Which AI tools produce consistently good output? Which need heavy editing?

**Time:** ~12 hrs/week admin (maximum overhead)

### Month 2-3: Selective Review

**Review all:** Tier 1 outreach, freelance proposals > $2K, contract pitches, consulting proposals, all content.

**Spot-check:** Tier 2/3 applications (1 in 3), networking follow-ups (1 in 3), X replies (1 in 5), comment replies (1 in 3).

**Auto-trust:** Tier 3 applications with score > 85, freelance proposals < $500 with score > 90.

**Time:** ~9 hrs/week admin

### Month 4+: Trust with Audits

**Review all:** Tier 1 outreach (always), contract/consulting proposals (always), content (always).

**Auto-trust (with audit):** Everything else.

**Weekly quality audit (Friday, 10 min):** Randomly sample 5 auto-sent items. Grade: Good / Needs improvement / Bad. If 2+ "Bad" → tighten review for that tool next week.

**Tightening triggers:**
- Response rate drops → review that output type
- Client complaint → full review for 1 week
- Connection acceptance drops → review outreach drafts
- Content engagement drops → review hooks and content
- Embarrassing output sent → full review for that tool

**Time:** ~6 hrs/week admin

---

## 8.6: Weekly Time Summary

### Your Time

| Activity | Hrs/week | Type |
|---|---|---|
| Morning gate reviews | 5-7 | Unpaid (admin) |
| Midday execution | 2-3 | Unpaid (admin) |
| Evening wrap-up | 1.5-2 | Unpaid (admin) |
| Friday review session | 0.5 | Unpaid (admin) |
| **Admin subtotal** | **9-12** | **Unpaid** |
| Freelance delivery | 12-18 | Paid |
| Contract management | 0-5 | Paid (when active) |
| Consulting calls/work | 0-4 | Paid (when active) |
| Deep work / GitHub / research | 4-8 | Investment |
| **Paid + Investment subtotal** | **16-35** | |
| **GRAND TOTAL** | **25-47** | **Per week** |

Admin reduces over time: Month 1 ~12 hrs → Month 2-3 ~9 hrs → Month 4+ ~6 hrs.

### AI's Work (continuous, no human cost)

~200-400 automated actions per week including: sweeps, scoring, resume tailoring, cover letters, outreach drafts, proposal drafts, content generation, comment replies, metrics analysis, cadence management, stale detection, warmth calculation, backlog refresh, quality sampling.

**Cost:** ₹0 (Claude OAuth, included in Max subscription)

---

## 8.7: Scout Dashboard Design

### Priority Queue Tab

All pending actions ranked by urgency/importance:

| Priority | Item Type | Example |
|---|---|---|
| **Urgent** | Expert call in 2 hours | Prep brief ready, review now |
| **Urgent** | Client message pending | Freelance client waiting for response |
| **High** | Tier 1 outreach approved | Send before connection window closes |
| **High** | Content publish time | Part 1 ready, optimal publish window now |
| **High** | Inbound DM from hot lead | Respond within 24 hours |
| **Medium** | Tier 2/3 applications | Ready to submit today |
| **Medium** | Freelance proposals | Score > 80, send today |
| **Medium** | Networking follow-ups | Accepted connections to message |
| **Low** | Profile updates | Triggered, no urgency |
| **Low** | Stale leads | Review at Friday session |

### Per-Section Tabs

| Tab | Shows |
|---|---|
| **Jobs** | Gate 1 batch, follow-up reminders, stale leads, application history |
| **Networking** | Gate N1 outreach queue, networking brief, warmth dashboard, conversion opportunities |
| **Freelance** | Gate F1 proposals, maybe list, active gigs, revenue tracker |
| **Contracts** | Gate C1 targets, active negotiations, upsell opportunities |
| **Consulting** | Upcoming calls + prep briefs, advisory schedule, proposals |
| **Content** | Gate P1 content queue, publish calendar, metrics dashboard, backlog |
| **Profile** | Gate PR1 pending updates, quarterly audit, GitHub activity |

---

## Implementation Notes

### Runner Integration

The weekly schedule is NOT a new component — it's the consolidated view of all pipeline runners:

- Job runner phases (DISCOVER, QUALIFY, PREPARE, CADENCE)
- Networking runner phases (IDENTIFY, GENERATE, TRACK, CONVERT)
- Freelance runner phases (DISCOVER, SCORE, DRAFT, CADENCE)
- Contracts runner phases (IDENTIFY, PITCH, CADENCE, POST-CONTRACT)
- Consulting runner phases (IDENTIFY, PREPARE, CADENCE, POST-ENGAGE)
- Content runner phases (TOPIC, GENERATE, ENGAGE, METRICS)
- Profile runner phases (DETECT, GENERATE)

All run in the same `internal/scout/runner/` polling loop. The schedule doc describes WHEN humans interact with the outputs, not new automation.

### Frontend Changes

**Modified:** Scout frontend Actions tab
- Add Priority Queue as first tab
- Existing per-section views become tabs
- Each tab shows pending gates + metrics for that section
- Priority Queue aggregates all tabs, sorted by urgency

### Prerequisites

- All 7 pipeline specs implemented (each contributes runner phases)
- Scout frontend Actions tab redesigned with tabs
- No new AI tools needed (schedule uses outputs from all existing specs)

## Relationship to Other Specs

This spec does not define new AI tools, new tables, or new pipelines. It is a **coordination layer** that describes:
- WHEN human gates from all 7 specs are checked
- IN WHAT ORDER tasks are prioritized
- HOW quality is monitored over time
- WHAT success looks like (metrics)

All automation is defined in the individual section specs. This spec orchestrates the human side.
