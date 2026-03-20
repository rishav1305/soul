# AI Leadership Team — Org Structure Design

**Date:** 2026-03-20
**Status:** Draft
**Author:** Rishav (CEO) + Claude (Architect)

## 1. Problem Statement

Soul-v2 has 13 servers, 127 tools, 65 skills, and 21 products. Running everything through a single generalist Claude session produces 70% quality across the board — too many tools, too much context, too little focus. The user needs specialized AI agents that operate as a virtual leadership team, each deeply focused on their domain, with the ability to collaborate on cross-cutting decisions.

### Goals

1. **Specialization** — Each role has focused skills, knowledge, and memory. No context bleed.
2. **Isolation** — Separate memory per role. Marketing doesn't store code architecture; Dev PM doesn't store SEO rankings.
3. **Collaboration** — Multi-persona conferences with real debate, research-backed positions, and natural conversation flow.
4. **CEO Control** — User plays CEO. Can consult any role from anywhere, convene conferences, and override any decision.
5. **Research-Backed** — No hallucination. Every persona must verify claims before stating them.

### Non-Goals

- Building a new framework or Python-based orchestration system
- Replacing the soul-v2 chat product (currently down; this is Claude Code only)
- Automating external actions (all outreach, emails, Slack messages require CEO approval)

## 2. Team Roster

| Role | ID | Type | Core Mission |
|------|----|------|-------------|
| **CEO** | (human) | Human | Cross-domain decisions, final authority, vision |
| **Scout PM** | `scout-pm` | Product Manager | Operate Scout pipeline daily — sweeps, gates, leads, targets. Zero dev knowledge. |
| **Dev PM** | `dev-pm` | Technical PM | Receive specs → plan → build → ship. Manages sprints, agents, code quality. |
| **Tutor** | `tutor` | Coach | Interview preparation — drilling, mocks, progress tracking, study plans. |
| **Marketing Head** | `marketing` | Marketing | Positioning, content, SEO, outreach copy, portfolio optimization. |
| **Strategy Expert** | `strategy` | Advisor | Cross-domain strategy reviews, upgrade recommendations, market analysis. |

### Key Distinction: Scout PM vs Dev PM

- **Scout PM** = Product user. Reviews gates, makes pipeline decisions, tracks metrics. Never touches code.
- **Dev PM** = Builder. Receives specs, plans sprints, ships code. Doesn't decide product direction.

Workflow for a new Scout feature:
```
Strategy: "We should add contract pipeline support"
Scout PM: "My gate reviews show 30% of leads are contract-type with no pipeline"
→ Conference produces spec
→ Dev PM receives spec, plans sprint, ships it
→ Scout PM starts using the new feature operationally
```

## 3. Interaction Modes

Three modes of interaction, each with different isolation and persistence characteristics:

### 3.1 Solo Session (Deep Work)

**How:** Bash alias → `cd ~/soul-roles/{role} && claude`

Full session with the persona. Own CLAUDE.md, own skills, own memory. Multi-turn, persistent. Used for daily routines, extended work, deep domain tasks.

```bash
alias scout-pm='cd ~/soul-roles/scout-pm && claude'
alias dev-pm='cd ~/soul-roles/dev-pm && claude'
alias tutor='cd ~/soul-roles/tutor && claude'
alias marketing='cd ~/soul-roles/marketing && claude'
alias strategy='cd ~/soul-roles/strategy && claude'
```

**Memory:** Reads and writes to own memory namespace.
**Isolation:** Full — only sees own CLAUDE.md, own skills, own knowledge sources.

### 3.2 Consult (Quick Pull)

**How:** Global skill invoked from any directory — `/scout-pm`, `/dev-pm`, `/tutor`, `/marketing`, `/strategy`

One-shot subagent dispatch. Reads the persona's CLAUDE.md and memory, answers the question with research, returns to the current session. Does NOT write to the persona's memory.

```
You (in ~/soul-v2): /scout-pm what's the pipeline status this week?

📣 SCOUT PM:
"Pipeline this week: 52 active leads, 8 tier-1. 3 follow-ups due today..."
```

**Memory:** Reads persona's memory, does not write.
**Isolation:** Full — subagent has only persona context + research tools.

### 3.3 Conference (Multi-Persona Debate)

**How:** Global skill — `/conference`

Structured multi-persona collaboration with research phase, round-based debate, CEO interject, and consensus detection. Details in Section 7.

```
You: /conference website-redesign --invite marketing,strategy,dev-pm
```

**Memory:** Reads persona research only. Writes consensus to shared/decisions/.
**Isolation:** Each persona is a separate subagent with only their research + full transcript.

## 4. Directory Structure

```
~/soul-roles/
├── scout-pm/
│   ├── CLAUDE.md                  ← Persona definition
│   ├── .claude/
│   │   └── skills/
│   │       └── daily-planner/     ← symlink
│   └── soul-v2 → ../../soul-v2   ← symlink (CLAUDE.md instructs read-only; not enforced by filesystem)
│
├── dev-pm/
│   ├── CLAUDE.md
│   ├── .claude/
│   │   └── skills/
│   │       └── soul-pm/           ← symlink
│   └── soul-v2 → ../../soul-v2   ← symlink (read-write, primary dev workspace)
│
├── tutor/
│   ├── CLAUDE.md
│   ├── .claude/
│   │   └── skills/                ← empty initially
│   └── soul-v2 → ../../soul-v2   ← CLAUDE.md instructs read-only
│
├── marketing/
│   ├── CLAUDE.md
│   ├── .claude/
│   │   └── skills/
│   │       └── ui-ux-pro-max/     ← symlink
│   └── soul-v2 → ../../soul-v2   ← CLAUDE.md instructs read-only
│
├── strategy/
│   ├── CLAUDE.md
│   ├── .claude/
│   │   └── skills/                ← no local skills
│   └── soul-v2 → ../../soul-v2   ← CLAUDE.md instructs read-only
│
└── shared/
    ├── decisions/                 ← Conference outputs, consensus docs
    ├── briefs/                    ← Strategy briefs, marketing plans, reports
    └── inbox/
        ├── scout-pm/              ← Action items for Scout PM
        ├── dev-pm/                ← Specs & build requests for Dev PM
        ├── marketing/             ← Content/SEO tasks for Marketing
        ├── tutor/                 ← Study plan updates for Tutor
        └── strategy/             ← Review requests for Strategy
```

### Isolation Mechanism

| Mechanism | What It Provides |
|-----------|-----------------|
| Separate directories | Unique `~/.claude/projects/-home-rishav-soul-roles-{role}/memory/` per role |
| Per-role CLAUDE.md | Persona identity, allowed skills, memory charter, knowledge sources |
| Per-role `.claude/skills/` | Only role-relevant custom skills physically present |
| Plugin skills (global) | Available everywhere, but CLAUDE.md whitelists specific ones |
| `shared/` directory | Only cross-role communication channel — structured docs only |
| `shared/inbox/{role}/` | Per-persona action items from conferences |

### Codebase Access

**Note:** The soul-v2 symlink gives filesystem read-write access to all roles. "Read-only" below is enforced via CLAUDE.md instructions, not filesystem permissions. This is a soft constraint — Claude generally follows CLAUDE.md directives but may occasionally violate them. Compliance should be tested in Phase 1 and monitored. If enforcement is insufficient, consider removing symlinks for non-dev roles and providing access only via absolute paths listed in CLAUDE.md.

| Role | soul-v2 Access | Enforcement |
|------|---------------|-------------|
| Dev PM | Full read-write (primary developer) | CLAUDE.md permits writes |
| Scout PM | Read-only: `docs/scout/`, `internal/scout/server/`, `web/src/components/scout/` | CLAUDE.md advisory |
| Marketing | Read-only: `portfolio_app/`, `docs/scout/content.md`, `docs/scout/platforms.md` | CLAUDE.md advisory |
| Strategy | Read-only: `docs/`, `docs/superpowers/specs/` | CLAUDE.md advisory |
| Tutor | Read-only: `internal/tutor/`, `web/src/pages/Tutor*.tsx` | CLAUDE.md advisory |

## 5. Skills Assignment

### Skills Inventory: 65 Skills Across 5 Sources

| Source | Count |
|--------|-------|
| Superpowers plugin | 14 |
| Marketing plugin (requires enablement — see prerequisite below) | 33 |
| Claude-Mem plugin | 4 |
| Dev plugins (code-review, feature-dev, frontend-design, hookify, commit-commands, simplify, context7, claude-md-management, pr-review-toolkit) | 9 |
| Custom local (soul-pm, ui-ux-pro-max, incremental-decomposition, e2e-quality-gate, daily-planner) | 5 |

**Prerequisite:** The `marketingskills` marketplace is configured in `~/.claude/settings.json` but NOT enabled in `enabledPlugins`. Before Phase 1, add `"marketingskills@marketingskills": true` to the `enabledPlugins` object.

**Note on `daily-planner`:** This skill exists in `~/soul-old/.claude/skills/daily-planner/` and will be symlinked to relevant persona directories. It displays daily task blocks with progress tracking from a daily planner markdown file. A soul-v2 equivalent may be created in Phase 3.

### Per-Role Assignment

**Scout PM — 8 skills (pipeline ops focus):**
- Local: daily-planner
- Marketing plugin: cold-email, email-sequence, competitor-alternatives, pricing-strategy, sales-enablement
- Claude-Mem: mem-search
- Superpowers: using-superpowers

**Dev PM — 32 skills (full dev lifecycle):**
- Local: soul-pm, ui-ux-pro-max, incremental-decomposition, e2e-quality-gate
- Superpowers: brainstorming, writing-plans, executing-plans, dispatching-parallel-agents, subagent-driven-development, systematic-debugging, TDD, verification-before-completion, finishing-a-development-branch, requesting-code-review, receiving-code-review, using-git-worktrees, writing-skills, using-superpowers
- Dev plugins: feature-dev, code-review, review-pr, simplify, commit, commit-push-pr, hookify, claude-md-improver, context7, frontend-design
- Claude-Mem: mem-search, make-plan, do, smart-explore

**Tutor — 4 skills (minimal, focused):**
- Local: daily-planner
- Claude-Mem: mem-search
- Dev plugins: context7
- Superpowers: using-superpowers

**Marketing Head — 42 skills (full marketing suite):**
- Local: ui-ux-pro-max, daily-planner
- All 33 marketing plugin skills
- Superpowers: brainstorming, writing-plans, executing-plans, using-superpowers
- Claude-Mem: mem-search, make-plan
- Dev plugins: frontend-design

**Strategy Expert — 12 skills (analysis + planning):**
- Superpowers: brainstorming, writing-plans, verification-before-completion, using-superpowers
- Marketing plugin: competitor-alternatives, pricing-strategy, product-marketing-context, analytics-tracking, content-strategy, launch-strategy
- Claude-Mem: mem-search, smart-explore

### Enforcement

Plugin skills are globally installed (`~/.claude/settings.json`) and available to ALL sessions regardless of working directory. There is no Claude Code mechanism to prevent a skill from being invoked in a given session. Enforcement is advisory via CLAUDE.md:

```markdown
## Skills
USE THESE ONLY: [list]
DO NOT USE (even if available): [list]
```

**Compliance model:** This is a soft constraint. Claude generally follows CLAUDE.md directives but may occasionally invoke a blocked skill if it seems highly relevant. Test compliance rates in Phase 1. In practice, the consequence of a violation is mild (e.g., Scout PM accidentally uses a dev skill) — the memory charter prevents storing out-of-domain results, limiting the downstream impact.

## 6. Memory Charter System

Each persona's CLAUDE.md contains a Memory Charter defining what to remember, what to ignore, and what to read.

### Charter Structure

```markdown
## Memory Charter

### STORE (your domain — save to your memory)
- [categories with examples]

### IGNORE (not your domain — never save)
- [what to skip]

### READ (knowledge sources — read but don't memorize)
- [files/directories/URLs]

### INBOX (check on startup)
- Read ~/soul-roles/shared/inbox/{role}/
- Store actionable items, archive processed files
```

### Per-Persona Charters

**Scout PM STORE:** Lead status changes, gate outcomes, pipeline metrics, sweep results, cadence state, outreach feedback.
**Scout PM IGNORE:** Code architecture, SEO rankings, interview scores, sprint progress.
**Scout PM READ:** `docs/scout/*.md`, `~/.soul-v2/dream-companies.json`, scout API endpoints.

**Dev PM STORE:** Sprint decisions, tech blockers, architecture decisions, merge outcomes, test state, build performance.
**Dev PM IGNORE:** Lead statuses, SEO data, interview scores, strategy rationale.
**Dev PM READ:** Full codebase (read-write), CLAUDE.md, specs, plans, tools.

**Tutor STORE:** Topic accuracy, weak areas, mock outcomes, study plan state, spaced repetition due dates, interview feedback patterns.
**Tutor IGNORE:** Everything not related to interview preparation.
**Tutor READ:** `internal/tutor/`, question banks, web search for interview resources.

**Marketing Head STORE:** SEO state, content performance, audit findings, competitor positioning, campaign results, brand decisions.
**Marketing Head IGNORE:** Codebase internals, lead pipeline details, interview scores, sprint progress.
**Marketing Head READ:** `docs/scout/content.md`, `docs/scout/platforms.md`, `portfolio_app/`, web search for SEO/competitor data.

**Strategy Expert STORE:** Strategy decisions with rationale, review cycle outcomes, market shifts, cross-domain insights, recommendations given, risk flags.
**Strategy Expert IGNORE:** Implementation details, individual lead statuses, drill scores, sprint progress.
**Strategy Expert READ:** All `docs/scout/*.md`, all specs, `shared/decisions/`, web search for market data.

### Inbox Protocol

Files in `shared/inbox/{role}/` follow a structured format:

```markdown
---
from: marketing
date: 2026-03-20
type: action          # action | info | decision
status: new           # new | processed
conference: website-redesign  # optional, links to decision doc
---

## Action Items

- [ ] Create 2 landing pages for top keywords ("AI consulting services", "ML engineer freelance")
- [ ] Implement JSON-LD schema markup on case study pages

## Context

Decision from website-redesign conference (2026-03-20). Hub-and-spoke model adopted.
See: shared/decisions/2026-03-20-website-redesign.md
```

**Naming:** `{date}-{slug}.md` (e.g., `2026-03-20-website-redesign.md`)
**Processing:** Persona reads all `status: new` items, changes front-matter to `status: processed` after handling.
**Archival:** Move processed files to `shared/inbox/{role}/archive/`.
**Conflicts:** If two conferences write to the same inbox simultaneously, unique slugs prevent collision.

### Memory Lifecycle Rules (All Personas)

1. **On session start:** Check inbox for `status: new` items, store actionable items in memory, change status to `processed`, move to archive.
2. **On session end:** Save key outcomes. Write action items for other personas to `shared/inbox/{role}/`.
3. **Staleness:** Metrics >30 days → verify before citing. Decision memories → valid until superseded.
4. **Conflicts:** Current file/data overrides stale memory. Update the memory.

## 7. Conference Protocol

### Invocation

```
/conference website-redesign --invite marketing,strategy,dev-pm
```

Or natural language:
```
"I want to redesign the portfolio. Bring in Marketing, Strategy, and Dev PM."
```

### Protocol Flow

```
PHASE 1: SETUP
├── Parse invited personas (validate against ~/soul-roles/)
├── Read IDENTITY + EXPERTISE from each persona's CLAUDE.md
├── Read shared/decisions/ for prior context
├── Present agenda to CEO for confirmation
└── CEO confirms or adjusts scope

PHASE 2: RESEARCH (parallel subagents)
├── Dispatch ALL personas simultaneously
├── Each gets: persona identity + agenda + CEO's framing
├── Each researches independently using their tools (WebSearch, Read, Grep)
├── Each returns: position (300 words max) + sources cited
└── Collect all research outputs

PHASE 3: ROUND TABLE
├── Round 1 (sequential — strategic → tactical):
│   ├── Speaking order priority: Strategy > Marketing > Scout PM > Tutor > Dev PM
│   │   (most strategic persona first; if Strategy absent, next most strategic leads)
│   ├── First speaker sees: CEO question + own research only
│   ├── Second speaker sees: first speaker's response + own research
│   └── Third+ speaker sees: all prior responses + own research
│
├── Round 2+ (parallel — all at once):
│   ├── ALL personas dispatched simultaneously
│   ├── Each receives: full transcript of ALL prior rounds + own research
│   ├── Each decides: RESPOND / CHALLENGE / ASK / CHANGE / RAISE / PASS
│   ├── Within a round, personas cannot react to each other (parallel)
│   ├── Next round, they see everything from this round
│   └── CEO can interject between any round
│
├── Convergence: all personas PASS in same round → consensus
├── Stalemate: same conflict for 3 rounds with no new evidence → warning
└── Escalation: round 10 → CEO decision required

PHASE 4: CONSENSUS
├── Present decision summary with rationale and sources
├── Per-persona action items
├── Note dissenting opinions if any persona was overruled
└── CEO approves or adjusts

PHASE 5: DISTRIBUTE
├── Write decision doc → shared/decisions/{date}-{topic-slug}.md
├── Write action items → shared/inbox/{persona}/{date}-{topic-slug}.md
└── Confirm distribution
```

### Subagent Prompt Template (Per Round)

```
You are {NAME}, the {TITLE}.

YOUR RESEARCH (from Phase 2):
{only your own research findings}

FULL TRANSCRIPT SO FAR:
{all prior rounds, verbatim}
[CEO interjected: "..." if any]

INSTRUCTIONS:
Review the full discussion. You may:
- RESPOND to a challenge directed at you
- CHALLENGE another persona's claim (with evidence)
- ASK a question to a specific persona
- CHANGE your position — concede a point or shift stance (explain why with evidence)
- RAISE a new concern nobody mentioned
- PASS if you have nothing to add

Rules:
- Stay within your domain expertise
- Cite evidence for new claims (research first)
- If someone asked you a question, ANSWER IT
- Don't repeat what you already said
- 200 words max per round
- If all your concerns are resolved: PASS
```

### Mid-Conference Join

When CEO says "bring in dev-pm" during an active conference:

1. Read Dev PM's CLAUDE.md (identity + expertise)
2. Build context injection:
   - Topic, current attendees, round number
   - Decisions made so far (✅ agreed items)
   - Active conflict they're joining (⚠️ with both positions)
   - CEO's specific question for them
3. Dispatch Dev PM subagent with context injection + research mandate
4. Present their response labeled as "joined Round N"
5. Include them in all subsequent rounds

### CEO Interaction

After each round:

```
━━━ ROUND N COMPLETE ━━━

📣 PERSONA_A: [summary]
📣 PERSONA_B: [summary]
📣 PERSONA_C: [passed]

✅ Agreed: [list]
⚠️  Unresolved: [list]

CEO, your turn (or press Enter to continue):
```

CEO can:
- Type a decision to resolve a conflict
- Redirect the conversation
- Ask a specific persona a question
- Bring in a new persona
- Press Enter to let debate continue

### Conference State File

To manage state across rounds and enable crash recovery, the conference skill writes a state file after each round:

```
~/soul-roles/shared/.conference-state/{topic-slug}/state.json
{
  "topic": "website-redesign",
  "attendees": ["marketing", "strategy", "dev-pm"],
  "round": 3,
  "status": "in_progress",
  "agreed": ["hub-and-spoke architecture", "case studies as primary content"],
  "unresolved": ["number of landing pages in Phase 1"],
  "transcript": [
    {"round": 1, "speaker": "strategy", "content": "..."},
    {"round": 1, "speaker": "marketing", "content": "..."},
    {"round": 1, "speaker": "dev-pm", "content": "..."},
    {"round": 2, "speakers": ["strategy", "marketing", "dev-pm"], "content": {...}},
    {"round": 2, "ceo_interjection": "..."}
  ],
  "research": {
    "marketing": "...",
    "strategy": "...",
    "dev-pm": "..."
  }
}
```

**On startup:** If `/conference` is invoked and an incomplete state file exists for a topic, offer to resume: "Found incomplete conference on '{topic}' (Round {N}). Resume or start fresh?"

**On completion:** Move state file to `shared/.conference-state/{topic-slug}/completed/` and write the decision doc.

**Token budget estimate:** 5 personas × 6 rounds × ~2K tokens per subagent = ~60K tokens of subagent calls, plus orchestrator context. A typical 3-persona, 4-round conference ≈ 25K tokens. Set a soft ceiling of 100K tokens per conference and warn if approaching it.

### Edge Cases

| Situation | Handling |
|-----------|---------|
| Only 1 persona invited | "Conference needs 2+. For solo work, use `{persona}` directly." |
| Persona comments outside domain | Flag: "{persona} is outside their domain — advisory only." |
| CEO contradicts all personas | Record as "CEO override" in decision doc. |
| Conflicting research data | Both data points presented with reliability assessment. |
| Session interrupted | State file preserves progress. On next `/conference` invocation, offer to resume from last saved round. |
| 10 rounds reached | Escalation with both sides' best arguments + facilitator's synthesis. |

## 8. Consult Mode — Global Skills

Each persona has a global skill at `~/.claude/skills/{role}/SKILL.md` for quick consultations from any directory.

### Skill Behavior

```
/scout-pm [question]     → dispatch Scout PM subagent, return answer
/dev-pm [question]       → dispatch Dev PM subagent, return answer
/tutor [question]        → dispatch Tutor subagent, return answer
/marketing [question]    → dispatch Marketing subagent, return answer
/strategy [question]     → dispatch Strategy Expert subagent, return answer
```

### Memory Loading in Consult Mode

Auto-memory is scoped by working directory. When `/scout-pm` is invoked from `~/soul-v2/`, the auto-memory path is `~/.claude/projects/-home-rishav-soul-v2/memory/` — NOT the Scout PM's memory.

**Fix:** Each consult SKILL.md must explicitly instruct the subagent to read memory from the persona's directory:

```
Before answering, read all memory files from:
~/.claude/projects/-home-rishav-soul-roles-scout-pm/memory/
This is YOUR memory from prior solo sessions. Use it as context.
```

This ensures consult mode has access to the persona's accumulated knowledge even when invoked from a different working directory.

### Consult vs Solo

| Aspect | Solo (bash alias) | Consult (global skill) |
|--------|-------------------|----------------------|
| Entry | `scout-pm` → new Claude session | `/scout-pm` from any session |
| Turns | Multi-turn, persistent | One-shot, returns to caller |
| Memory write | Yes | No |
| Memory read | Yes | Yes |
| Research | Full tools | Full tools |
| Use for | Deep work, daily routines | Quick questions, mid-session advice |

## 9. CLAUDE.md Template

Each persona's CLAUDE.md follows this structure:

```markdown
# {Role Name} — {Title}

## Identity
Who you are, operating philosophy. 3-5 sentences.

## Mandate
DO: [explicit list of responsibilities]
DO NOT: [explicit boundaries]

## KPIs & Targets
Daily: [measurable goals]
Weekly: [measurable goals]
Monthly: [measurable goals]

## Skills
USE THESE ONLY: [whitelist]
DO NOT USE: [blacklist]

## Memory Charter
### STORE
### IGNORE
### READ
### INBOX

## Daily Routine
Steps to follow on every session start.

## Weekly Routine
Recurring weekly tasks.

## Research Requirement
BEFORE making any claim about [domain topics]:
- Use [specific tools] to verify
- NEVER state assumptions as facts
- ALWAYS cite source

## Escalation Rules
Handle autonomously: [list]
Escalate to CEO: [list]

## Codebase Access
READ ONLY: [paths]
DO NOT ACCESS: [paths]
```

## 10. Prior Art & Design Influences

This design draws patterns from battle-tested frameworks while building natively in Claude Code:

| Pattern | Source | How We Use It |
|---------|--------|--------------|
| Round-based debate protocol | [AutoGen MAD](https://microsoft.github.io/autogen/stable//user-guide/core-user-guide/design-patterns/multi-agent-debate.html) | Conference rounds with convergence detection |
| Role/goal/backstory/tools model | [CrewAI](https://github.com/crewAIInc/crewAI) | CLAUDE.md persona template structure |
| Persona management commands | [claude-personas](https://github.com/mushfoo/claude-personas) | `/scout-pm`, `/marketing` global skills |
| 5-layer system architecture | [MindStudio Agentic OS](https://www.mindstudio.ai/blog/agentic-business-os-claude-code-architecture-guide) | Context + memory + skills + orchestration + maintenance |
| Bash persona launcher | [claude-persona](https://github.com/kellyredding/claude-persona) | Solo session bash aliases |
| Research shows 2-3 rounds optimal | [MAD Research (ICML 2024)](https://github.com/composable-models/llm_multiagent_debate) | 10-round ceiling is generous; expect convergence by round 3-4 |

### Why Not Adopt a Framework Wholesale

None of the existing frameworks (AutoGen, CrewAI, LangGraph) run natively in Claude Code. Porting them would require maintaining a separate Python runtime. Our design uses Claude Code's native primitives:

- **Skills** (`~/.claude/skills/`) for `/consult` commands
- **Subagents** (Agent tool) for conference rounds
- **Separate directories** for solo session isolation
- **CLAUDE.md** for persona definition
- **Auto-memory** (`~/.claude/projects/{path}/memory/`) for per-role persistence
- **Shared filesystem** (`shared/decisions/`, `shared/inbox/`) for cross-collaboration

## 11. Implementation Scope

### Phase 1: Foundation (Solo Sessions + Consult)

0. **Prerequisites:**
   - Enable marketingskills plugin: add `"marketingskills@marketingskills": true` to `enabledPlugins` in `~/.claude/settings.json`
   - Initialize version control: `cd ~/soul-roles && git init`
1. Create `~/soul-roles/` directory structure with symlinks
2. Write CLAUDE.md for each of the 5 personas (following template in Section 9)
3. Set up `.claude/skills/` with symlinked role-specific skills per directory
4. Create 5 global consult skills at `~/.claude/skills/{role}/SKILL.md` (with explicit memory path loading per Section 8)
5. Create `/conference` global skill (stub — Phase 2)
6. Add bash aliases to `~/.bashrc`
7. Create `shared/` directory structure (decisions/, briefs/, inbox/{role}/, inbox/{role}/archive/)
8. Test: solo session for each persona, verify memory isolation (check `~/.claude/projects/` paths)
9. Test: consult from soul-v2 root for each persona, verify memory loading works
10. Test: skill whitelist compliance — measure how often personas respect "DO NOT USE" directives

### Phase 2: Conference Protocol

1. Implement `/conference` skill with full protocol:
   - Phase 1 (Setup): parse invitation, load personas, present agenda
   - Phase 2 (Research): parallel subagent dispatch with research mandate
   - Phase 3 (Round Table): sequential Round 1, parallel Round 2+, convergence detection
   - Phase 4 (Consensus): structured summary, action items
   - Phase 5 (Distribute): write to shared/decisions/ and shared/inbox/
2. Implement mid-conference join (bring in new persona)
3. Implement CEO interject protocol
4. Test: 3-persona conference with real debate topic

### Phase 3: Operational Polish

1. Daily routine automation (inbox checking, brief generation)
2. Weekly report templates per persona
3. Staleness detection for memory entries
4. Conference history and decision search
5. Metrics: track conference outcomes, consult frequency, memory growth

## 12. Versioning

The `~/soul-roles/` directory should be tracked under version control (either `git init` in soul-roles, or symlink persona CLAUDE.md files from the soul-v2 repo). This enables:
- Tracking persona evolution over time
- Debugging behavioral changes by diffing CLAUDE.md history
- Rolling back persona changes that degrade quality

## 13. Open Questions

1. **Persona evolution:** As roles mature, their CLAUDE.md and memory charters will need updating. Who maintains them — the persona itself or CEO?
2. **soul-v2 chat integration:** When the chat product comes back online, should personas be available as chat products? Or keep them Claude Code only?

### Resolved

- **Token budget:** Addressed in Section 7 (Conference State File). Soft ceiling of 100K tokens per conference. Typical conference ≈ 25K tokens.
- **Conflict of interest (solo + conference same day):** Resolved — solo work before a conference is preparation, not bias. The persona brings informed context to the debate, same as a human preparing for a meeting.
