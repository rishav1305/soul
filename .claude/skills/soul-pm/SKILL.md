---
name: soul-pm
description: Use BEFORE any implementation work on soul-v2. Manages sprint execution with parallel agents, phase verification, Asana/Slack updates, resource checks, and pillar compliance. Augments superpowers PM skills with soul-v2 specific workflow.
---

# Soul PM — Project Management for soul-v2

## Overview

This skill wraps the superpowers PM skills (writing-plans, executing-plans, dispatching-parallel-agents) with soul-v2 specific context: Asana integration, Slack updates, phase testing, resource management, titan-pc offloading, and the 30-rule agent mandate.

**Use this INSTEAD of raw superpowers skills when working on soul-v2.**

## When to Use

- Before starting any implementation sprint
- Before launching parallel agents
- After completing any phase/batch
- When responding to inbound job/freelance leads
- When updating project status

## Pre-Sprint Checklist

Before ANY implementation work:

```
1. bash tools/resource-check.sh          → confirm agent capacity
2. make verify-static                     → confirm baseline is green
3. Check Asana for current task status    → what's done, what's next
4. Check docs/scout/implementation-status.md → persistent progress
5. Read CLAUDE.md "Agent Mandate" section → 30 rules for every agent
```

## Launching Parallel Agents

### Rule: Split shared file modifications

When multiple agents need to modify the SAME file (server.go, scout.go, dispatch.go):
- Agents ONLY create their own files (ai/*.go, runner/*.go, components/*.tsx)
- ONE integration agent modifies shared files AFTER all tool agents merge
- This prevents merge conflicts

### Agent Prompt Template

Every agent gets this context (adapt per task):

```
You are working on soul-v2. Read CLAUDE.md for full conventions.

YOUR TASK: [specific task from Asana]

FILES TO CREATE: [list — these are YOURS, no one else touches them]
FILES TO READ (for patterns): ai/match.go, ai/referral.go
DO NOT MODIFY: server.go, scout.go, dispatch.go (handled by integration task)

SPEC: [specific spec file path]
PATTERN: [which existing file to copy]

RULES (from CLAUDE.md Agent Mandate):
- Unit test every public function
- Parameterized SQL only
- Claude calls through internal/chat/stream/
- Error returns not panics
- go test -race must pass
- make verify-static before committing
- One logical commit per change
- Commit prefix: feat/fix/test

ACCEPTANCE: [from Asana task description]

When done, commit your changes and ensure make verify-static passes.
```

### Resource Check Before Launch

```bash
bash tools/resource-check.sh
# Read RECOMMENDED MAX PARALLEL
# Launch that many agents, no more
```

### titan-pc Build Offloading

When RPi CPU is busy with agents, offload verification:

```bash
# Sync latest code
rsync -az --exclude='node_modules' /home/rishav/soul-v2/ titan-pc:/home/rishav/soul-v2/

# Run tests on titan-pc
ssh titan-pc "cd ~/soul-v2 && go test -race -count=1 ./internal/scout/..."

# Run full build on titan-pc
ssh titan-pc "cd ~/soul-v2 && go build ./..."
```

## After Each Phase

### 1. Run Phase Tests

```bash
bash tools/phase-tests.sh <foundation|batch1|batch2|batch3|integration>
```

Print the FULL output. If any FAIL, fix before proceeding.

### 2. Update Asana

Use `mcp__claude_ai_Asana__update_task` to mark tasks complete:
```
task_id: [from Asana]
completed: true
```

### 3. Update Slack

Post to #all-soul-labs (channel ID: C0AM75YNF26):
```
[PHASE NAME] Complete ✓
- Tasks completed: X/Y
- Tests: [phase-tests.sh result summary]
- Next: [what's launching next]
```

### 4. Update Progress Tracker

Edit `docs/scout/implementation-status.md` — check off completed items.

## Responding to Inbound Leads

When user receives a job/freelance/consulting inquiry:

1. **Assess against strategy** — which tier? which pipeline?
2. **Research market rates** — never quote below current ($8K/mo)
3. **Draft response** — humanized, not corporate. No "sorry for delay."
4. **Present to user for review** — explain strategy reasoning
5. **Track in Scout** — add to docs/scout/leads-backlog.md with full fields

Rate guidance:
- Current: $8,000/month (Andela/IBM-TWC baseline)
- Principal/Senior upgrade: $10,000-12,000/month
- Direct clients: $150-250/hr
- Expert calls: ₹10-30K/hr

## Key References

| What | Where |
|---|---|
| Implementation plan | docs/superpowers/plans/2026-03-19-scout-implementation-plan.md |
| Progress tracker | docs/scout/implementation-status.md |
| Phase tests | tools/phase-tests.sh |
| Resource check | tools/resource-check.sh |
| Design specs (8) | docs/superpowers/specs/2026-03-18-*.md |
| Strategy docs (13) | docs/scout/*.md |
| Leads backlog | docs/scout/leads-backlog.md |
| Agent mandate | CLAUDE.md → "Agent Mandate (30 rules)" |
| Quick references | CLAUDE.md → "Scout — Quick Reference for AI Developers" |
| Asana project | gid: 1213733744154117 |
| Slack channel | #all-soul-labs (C0AM75YNF26) |
| Asana user | rishav.chatt@gmail.com (gid: 1200478425211836) |
| titan-pc | SSH reachable, Go installed, repo synced via rsync |

## Skill Chain

This skill orchestrates other skills:

```
soul-pm (this skill)
  ├── superpowers:writing-plans      → create implementation plans
  ├── superpowers:executing-plans    → execute step by step
  ├── superpowers:dispatching-parallel-agents → launch agent batches
  ├── superpowers:subagent-driven-development → independent subtasks
  ├── superpowers:verification-before-completion → make verify before claims
  ├── superpowers:requesting-code-review → review before merge
  ├── superpowers:finishing-a-development-branch → merge/PR decisions
  ├── incremental-decomposition      → break complex UI into steps
  └── e2e-quality-gate               → verify frontend after each step
```

## Identity Management

All agents run as user "rishav". To distinguish:
- Branch names include task: `scout/agent-2-resume-tailor`
- Commits include agent role: `feat(agent-2): ResumeTailor AI tool`
- Only PM updates Asana/Slack — agents never touch external platforms
- Merge one agent at a time — sequential merges, never parallel
