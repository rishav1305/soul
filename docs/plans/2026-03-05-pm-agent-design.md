# PM Agent — Project Management Intelligence for Soul

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add always-on project management intelligence so Soul creates well-structured tasks, maintains board hygiene, and supports on-demand scrum workflows — all optimized for a solo developer.

**Architecture:** Three reinforcing layers — frontend validation, backend rule engine + LLM enrichment, and chat skill for interactive PM commands.

**Tech Stack:** Go 1.24 backend, React 19 + TypeScript + Tailwind v4 frontend, SQLite, Claude API

---

## Design Principles

- **Solo dev scrum:** No ceremonies for ceremonies' sake. Focus on task quality, board hygiene, and "what should I work on next?"
- **Free by default:** Rule-based checks cost zero tokens. LLM only for decomposition and enrichment.
- **Always-on:** PM service runs regardless of chat state. Every task gets checked.
- **Additive = auto, destructive = ask:** PM creates subtasks and posts comments freely. Deleting, deprioritizing, or merging duplicates requires user confirmation.

---

## Layer 1: Frontend Validation (NewTaskForm)

Client-side checks in `NewTaskForm.tsx` before submit. Soft gates (warnings) and hard gates (errors).

### Checks

| Check | Rule | Gate |
|-------|------|------|
| Empty description | `description.trim() === ""` | Soft — warning: "Tasks without descriptions are harder to execute. Add details?" |
| Short description | `description.length < 30` | Soft — "Consider adding acceptance criteria (use `- [ ]` checklists)" |
| Critical + no description | `priority === 3 && description.trim() === ""` | Hard — disable submit: "Critical tasks require a description" |
| Vague title | `title.length < 10` and no verb detected | Soft — "Try a more specific title (e.g., 'Add logout button to sidebar')" |

### Implementation

A `validate()` function returning `{warnings: string[], errors: string[]}`. Warnings render in amber below the form. Errors render in red and disable submit.

---

## Layer 2: Backend PM Service

New file: `internal/server/pm.go`

### PMService Struct

```go
type PMService struct {
    planner   *planner.Store
    broadcast func(WSMessage)
    ai        *ai.Client
    model     string
    ticker    *time.Ticker
    stopCh    chan struct{}
}
```

Initialized in `server.go`, started alongside the server. Stopped on shutdown.

### 2a: After-Create Hook

Called from `handleTaskCreate` in tasks.go and from `toolTaskCreate` in agent.go — runs on ALL task creation regardless of source.

```go
func (pm *PMService) AfterCreate(task planner.Task)
```

| Check | Rule | Action |
|-------|------|--------|
| Missing description | `len(task.Description) < 20` | Post comment: "Consider adding a description with acceptance criteria." If source is "ai", auto-enrich via LLM. |
| Missing AC | Description lacks `- [ ]` or "acceptance" | Post comment with suggested AC template based on title keywords |
| Duplicate detection | Levenshtein similarity > 70% against open tasks (backlog/brainstorm/active) | Post comment: "Possible duplicate of Task #N: {title}" |
| Priority sanity | Priority = Critical (3) but no description | Post comment: "Critical priority with no description — add details or lower priority." |
| Large task detection | Description > 500 chars OR title contains decomposition keywords | Trigger LLM decomposition — create subtasks with `parent_id`, post comment on parent |

**Decomposition keywords:** "refactor", "redesign", "implement full", "add feature with", "new feature", "migration", "overhaul"

**LLM enrichment prompt** (for auto-generating descriptions):
```
Given this task title: "{title}"
Product: {product}

Generate a 2-3 line description with acceptance criteria as a markdown checklist.
Return ONLY the description text, no preamble.
```

**LLM decomposition prompt:**
```
Break this task into 2-5 focused subtasks. Each subtask should be completable in one autonomous agent session (1-3 files).

Task: {title}
Description: {description}

Return a JSON array: [{"title": "...", "description": "...", "priority": N}]
No preamble, just JSON.
```

### 2b: Periodic Sweep

Runs every 30 minutes via `time.Ticker`.

```go
func (pm *PMService) sweep()
```

| Check | Rule | Action |
|-------|------|--------|
| Stale backlog | In backlog > 7 days | Post comment: "Stale — 7+ days in backlog. Still relevant?" |
| Stuck active | In active > 48 hours, no recent comment/activity | Post comment + broadcast warning |
| Stuck blocked | In blocked > 5 days | Broadcast warning to chat |
| Stuck validation | In validation > 3 days | Broadcast info to chat |
| Priority decay | Normal priority (1) + in backlog > 14 days | Auto-lower to 0 (low), post comment explaining |

**Batched broadcast:** All sweep findings are collected into a single `pm.notification` message:

```
**[PM Board Review]**
- Task #38 blocked 5 days — resolve or deprioritize?
- Task #42 active 48h with no activity — stuck?
- Task #29 stale in backlog 10 days
- 3 tasks in validation awaiting review
```

### 2c: Levenshtein Implementation

~15 lines of Go, no external dependency:

```go
func levenshtein(a, b string) int {
    // Standard dynamic programming implementation
    // Returns edit distance
}

func similarity(a, b string) float64 {
    dist := levenshtein(strings.ToLower(a), strings.ToLower(b))
    maxLen := max(len(a), len(b))
    if maxLen == 0 { return 1.0 }
    return 1.0 - float64(dist)/float64(maxLen)
}
```

Duplicate check: compare new task title against all open task titles. Flag if similarity > 0.7.

---

## Layer 3: Agent System Prompt Enhancement

### 3a: Always-On PM Rules in systemPrompt

Added to the `systemPrompt` constant in agent.go, under a new `# Task creation standards` section:

```
# Task creation standards
When creating tasks with task_create, ALWAYS follow these rules:
- Title must be specific and actionable (verb + object, 10+ chars). Bad: "Fix bug". Good: "Fix login timeout on session expiry"
- Description is REQUIRED. Include: what needs to change, why, and acceptance criteria as a checklist (- [ ])
- Set priority based on impact: 3=Critical (broken/blocking), 2=High (important), 1=Normal (default), 0=Low (nice-to-have)
- Set product when known (soul, scout, compliance)
- Before creating, check if a similar task already exists: use task_list to verify.
- For large tasks (3+ files, multiple concerns), create a parent task + subtasks instead of one monolith.
- When decomposing, set parent_id on subtasks to link them to the parent.
```

### 3b: toolTaskCreate Validation

Hard validation in the Go function before database insert:

```go
func (a *AgentLoop) toolTaskCreate(input map[string]any, sendEvent func(WSMessage)) string {
    title := stringVal(input, "title")
    if len(strings.TrimSpace(title)) < 10 {
        return "Error: Title too short (minimum 10 chars). Use a specific, actionable title (verb + object)."
    }
    // ... existing creation logic
}
```

The agent gets an error response and must fix the title. This is the hardest gate — no way to bypass.

---

## Layer 4: PM Chat Skill

### Skill File

Loaded from skills system. Auto-activates when `chatType == "pm"` or when PM keywords detected.

### On-Demand Commands

| User says | Agent does |
|-----------|-----------|
| "groom backlog" | `task_list(stage=backlog)` → analyze each task → flag stale/vague/duplicate → offer to clean up |
| "plan next sprint" / "what should I work on" | `task_list()` → rank by priority + age + dependencies → recommend 3-5 tasks to activate |
| "decompose task #N" | Read task → LLM breaks into subtasks → create with parent_id → post comment |
| "review board" | `task_list()` for all stages → report: stage distribution, stuck tasks, priority spread, product coverage |
| "retro" / "retrospective" | Query done tasks from last N days → summarize: avg time per stage, common blockers, files most modified |
| "triage" | Run full PM sweep immediately (same as periodic but interactive) |

### Skill Content

```markdown
# PM / Scrum Master

You are acting as a project manager for a solo developer. Principles:
- Keep the board clean. No vague tasks, no duplicates, no stale items.
- Every task needs: specific title, description with AC, correct priority, product.
- Prefer small focused tasks (1-3 files) over monoliths. Decompose aggressively.
- When suggesting priorities: user impact > blocking other work > effort vs value.
- Act directly on safe/additive actions (create subtasks, add comments, fix priorities).
- Ask permission for destructive actions (delete tasks, merge duplicates, deprioritize critical).

## Commands
[detailed instructions for each command with tool call sequences]
```

---

## WebSocket Notification Format

New message type `pm.notification`:

```json
{
  "type": "pm.notification",
  "content": "**[PM]** Task #38 blocked 5 days — resolve or deprioritize?",
  "data": {
    "severity": "warning",
    "task_ids": [38],
    "check": "stuck_blocked"
  }
}
```

**Severity levels:**

| Severity | Use case | Frontend style |
|----------|----------|----------------|
| `info` | Stale backlog, duplicate detected, retro summary | Subtle gray text |
| `warning` | Stuck active/blocked, missing AC on high-priority | Yellow accent |
| `error` | Critical with no description, validation stuck > 5 days | Red accent |

**Frontend rendering:** `pm.notification` renders as a system message in chat with a PM badge and severity-appropriate styling.

---

## Data Flow

```
TASK CREATION (any source)
  Soul Chat → toolTaskCreate() ──▶ Hard gate: reject vague titles
  API/Frontend ─────────────────▶ planner.Create()
  Autonomous Agent ─────────────▶      │
                                       ▼
                                 PMService.AfterCreate()
                                 ├─ Flag missing description
                                 ├─ Duplicate check (Levenshtein)
                                 ├─ Priority sanity
                                 ├─ Auto-enrich description (LLM, if source=ai)
                                 └─ Decompose if large (LLM)
                                       │
                                       ▼
                                 Post comments + broadcast to chat

PERIODIC SWEEP (every 30 min)
  PMService.sweep()
  ├─ Stale backlog (>7 days)
  ├─ Stuck active (>48h)
  ├─ Stuck blocked (>5 days)
  ├─ Stuck validation (>3 days)
  ├─ Priority decay (normal + >14 days → low)
  └─ Batched broadcast to chat

ON-DEMAND (Chat)
  User: "groom backlog" / "plan sprint" / "review board"
  Agent uses task_list + task_create + task_update + task_comment
  guided by system prompt PM rules + optional PM skill
```

---

## Files to Create/Modify

| File | Action | Purpose |
|------|--------|---------|
| `internal/server/pm.go` | NEW | PMService: AfterCreate hook, periodic sweep, Levenshtein, LLM decomposition |
| `internal/server/server.go` | MODIFY | Initialize PMService, start/stop sweep ticker |
| `internal/server/tasks.go` | MODIFY | Call `pm.AfterCreate()` after task creation |
| `internal/server/agent.go` | MODIFY | PM rules in systemPrompt, title validation in toolTaskCreate |
| `web/src/components/planner/NewTaskForm.tsx` | MODIFY | Client-side validation (warnings/errors) |
| `web/src/components/chat/MessageBubble.tsx` | MODIFY | Render `pm.notification` with severity styling |
| `web/src/hooks/useChat.ts` or `lib/ws.ts` | MODIFY | Handle `pm.notification` WebSocket type |

---

## Implementation Priority

| Order | What | Effort | LLM cost |
|-------|------|--------|----------|
| 1 | Agent systemPrompt PM rules + toolTaskCreate validation | Small | None |
| 2 | PMService struct + AfterCreate hook (rule-based checks) | Medium | None |
| 3 | Periodic sweep | Medium | None |
| 4 | LLM enrichment + decomposition | Medium | Per-task |
| 5 | WebSocket pm.notification + frontend rendering | Small | None |
| 6 | Frontend NewTaskForm validation | Small | None |
| 7 | PM chat skill | Small | Per-use |
