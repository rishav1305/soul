# Soul Improvement Roadmap — Agent Quality, Feature Parity & Productivity

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Transform Soul from a working autonomous platform into a high-quality, Claude-Code-competitive development environment through smarter agent behavior, better tools, and productivity features.

**Architecture:** Four tiers of improvement, each building on the previous. Tier 1 (agent quality) is the foundation — every subsequent tier is more valuable when the agent makes better decisions.

**Tech Stack:** Go 1.24 backend, React 19 + TypeScript + Tailwind v4 frontend, SQLite, Claude API

---

## Tier 1: Agent Quality

### 1.1 Adaptive Planning

Before the agent touches any code, it gets a mandatory planning phase injected into the system prompt. The agent must output a structured plan as its first response:

```
## Plan
- Files to modify: web/src/components/chat/InputBar.tsx
- Files to read first: web/src/components/chat/ChatPanel.tsx (for context)
- Approach: Add voice button next to send button, reuse existing mic icon
- Estimated complexity: 1 file, ~10 lines changed
- Risk: None — additive change
```

The system parses this plan from the first assistant response. If the agent skips planning (jumps straight to tool calls), the system injects a warning: "You must plan before acting."

For micro tasks, the plan is 2-3 lines. For full tasks, a detailed multi-step breakdown. The plan constrains the agent's behavior by making its intent explicit.

### 1.2 Context Budget

Track cumulative token usage across iterations. Four thresholds:

| Context Usage | Action |
|---|---|
| < 50% | Normal operation |
| 50-70% | Compress tool results: large outputs (>500 chars) get summarized to first/last 5 lines + "... (N lines omitted)" |
| 70-85% | Aggressive compression: drop all tool call content from messages older than 5 iterations, keep only assistant text summaries |
| > 85% | Emergency: summarize entire conversation into a 500-word brief, restart agent loop with brief as context + current task state |

Prevents the degradation cascade — context quality stays high throughout execution.

### 1.3 Stuck Detection & Strategy Pivots

A `LoopDetector` struct in agent.go that tracks the last 10 tool calls. Detection rules checked after every tool call:

| Pattern | Signal | Action |
|---|---|---|
| Same tool called 3+ times with similar args (Levenshtein distance < 30%) | Search loop | Inject: "You've searched for this 3 times. Use what you have or try a completely different approach." |
| code_edit on same file 3+ times in last 5 iterations | Edit thrashing | Inject: "You've edited this file 3 times. Stop, re-read it fully, then make one correct edit." |
| code_exec returns same error 2+ times | Build loop | Inject: "Same error twice. Read the error carefully, identify the root cause, don't retry the same fix." |
| 5 consecutive tool calls with no assistant text between them | Blind execution | Inject: "Pause. Summarize what you've learned so far and what you're trying to do next." |
| Iteration count > 70% of max with task still active | Running out of time | Inject: "You're at {n}/{max} iterations. Wrap up: commit what works, document what's left." |

When a loop is detected, the injected message suggests a concrete alternative strategy, not just "stop."

### 1.4 Tool Result Compression

Applied retroactively as context grows via a `compressHistory()` function called before each API request:

| Age of tool result | Compression |
|---|---|
| Last 3 iterations | Full output (unchanged) |
| 4-8 iterations ago | Keep first 10 + last 5 lines, replace middle with summary |
| 9+ iterations ago | Replace entire tool result with 1-line deterministic summary |

Summaries are generated deterministically (extract filenames from grep, line counts from reads, exit codes from exec). Fast and free — no LLM call.

---

## Tier 2: Better Tools & Subagents

### 2.1 code_glob Tool

Dedicated file pattern matching:
```
Input:  { "pattern": "**/*.tsx", "path": "web/src/components" }
Output: ["layout/AppShell.tsx", "chat/InputBar.tsx", ...] (sorted, max 50)
```
Uses Go's filepath.Glob with ** support via doublestar library. Faster than shelling out to find.

### 2.2 code_grep Improvements

- `context_lines` param (default 0, max 3) for surrounding lines
- `max_results` param (default 50, allow up to 200)
- Group results by file in output
- Return file:line format for easy code_read follow-up

### 2.3 code_read Improvements

- `summary` mode: returns function/type signatures only (Go: via go doc, TS: regex for export lines)
- When file > 200 lines and no line range specified, auto-return summary + hint to use line ranges

### 2.4 Subagent Tool

Agent can spawn a focused sub-agent for exploration:
```json
{
  "name": "subagent",
  "input": {
    "task": "Find all components that import useChat hook and list their file paths",
    "max_iterations": 5
  }
}
```

Creates a new AgentLoop with minimal system prompt (code tools only, no task management). Fresh context, no pollution from parent. Limited to 5 iterations and read-only tools (no code_write, code_edit, code_exec). Returns final assistant text as tool result.

### 2.5 Task-Scoped Memory

New tool `task_memory` with store/recall actions:
- In-memory map scoped to current task execution
- Prepopulated with pre-loaded file paths from task processor
- Persists across retries (agent keeps what it learned)
- System prompt: "Before searching, check task_memory first. After discovering something important, store it."

### 2.6 Cross-Task Learning

When a task completes successfully, extract a task fingerprint:
```json
{
  "task_id": 34,
  "files_modified": ["AppShell.tsx", "globals.css"],
  "files_read": ["ProductRail.tsx", "HorizontalRail.tsx"],
  "iterations_used": 12,
  "keywords": ["layout", "panel", "css", "overlap"]
}
```

Stored in SQLite memories table. When a new task starts, keyword-match against fingerprints and inject file suggestions: "Similar task #34 modified AppShell.tsx and globals.css. Consider starting there."

---

## Tier 3: Hooks System

### 3.1 Pre/Post Tool Hooks

File-based configuration in ~/.soul/hooks.json:

```json
{
  "hooks": [
    {
      "event": "after:code_edit",
      "match": "*.tsx",
      "command": "cd {worktree} && npx eslint --fix {file}",
      "timeout": 10
    },
    {
      "event": "after:code_edit",
      "match": "*.go",
      "command": "cd {worktree} && gofmt -w {file}",
      "timeout": 5
    },
    {
      "event": "before:code_exec",
      "deny_pattern": "rm -rf|git push|git reset",
      "action": "block",
      "message": "Dangerous command blocked"
    }
  ]
}
```

Event types:
- `before:{tool}` — runs before execution. Non-zero exit or block action rejects the tool call.
- `after:{tool}` — runs after success. Output appended to tool result.

Template variables: {file}, {worktree}, {task_id}, {tool_name}

A HookRunner in the agent loop checks hooks.json for matching events after tool dispatch.

### 3.2 Workflow Hooks

```json
{
  "workflow_hooks": [
    { "event": "after:merge_to_dev", "command": "curl -X POST http://localhost:9000/api/webhook/deploy-dev" },
    { "event": "after:task_done", "command": "echo 'Task {task_id} complete' >> ~/.soul/completed.log" }
  ]
}
```

Events: before:task_start, after:plan, before:merge_to_dev, after:merge_to_dev, before:merge_to_master, after:merge_to_master, after:task_done, after:task_blocked

---

## Tier 4: Scale & Productivity

### 4.1 Parallel Task Execution

Replace single-task processor with a worker pool:

```
TaskScheduler
  +-- Worker 1: task/34 (worktree: .worktrees/task-34/)
  +-- Worker 2: task/35 (worktree: .worktrees/task-35/)
  +-- Worker 3: (idle)
```

Configuration: SOUL_MAX_WORKERS=3 (default: 2, max: 5). Each worker gets its own worktree and AgentLoop. SQLite handles concurrency via WAL mode. Merge serialized via mergeMutex — workers queue for merge access.

Scheduling priority: priority (critical > high > normal > low), then created_at (oldest first). Active tasks preferred over backlog.

### 4.2 Quick Actions from Chat

Chat agent gets a `quick_task` tool that:
1. Creates task with the message as title
2. Sets autonomous + micro workflow
3. Starts execution immediately
4. Streams progress in the chat thread

One message -> code changes on dev server.

### 4.3 Keyboard Shortcuts

| Shortcut | Action |
|---|---|
| Cmd/Ctrl+K | Focus chat input |
| Cmd/Ctrl+N | New task dialog |
| Cmd/Ctrl+J | Toggle drawer |
| Cmd/Ctrl+1-4 | Switch drawer tab |
| Esc | Close any modal/drawer |

Global useHotkeys hook in AppShell.

### 4.4 Task Templates

Pre-fill patterns from "+ New" dropdown:
- Bug fix: priority=high, workflow=quick, "Steps to reproduce" template
- UI change: workflow=micro, product=soul
- Feature: workflow=full, "User story / Acceptance criteria" template
- Refactor: workflow=full, "Current state / Desired state / Files" template

Stored in ~/.soul/templates.json. Editable, extensible.

### 4.5 Batch Operations

Multi-select tasks on kanban board for bulk: move to stage, set priority, delete, assign product.

---

## Implementation Priority

| Order | Section | Files | Effort |
|---|---|---|---|
| 1 | Adaptive Planning (1.1) | agent.go, autonomous.go | Medium |
| 2 | Stuck Detection (1.3) | agent.go (new LoopDetector) | Medium |
| 3 | Tool Compression (1.4) | agent.go (compressHistory) | Small |
| 4 | Context Budget (1.2) | agent.go, ai/client.go | Medium |
| 5 | code_glob + grep improvements (2.1, 2.2) | codetools.go | Small |
| 6 | code_read summary mode (2.3) | codetools.go | Small |
| 7 | Task-scoped memory (2.5) | agent.go (new tool) | Small |
| 8 | Subagent tool (2.4) | agent.go (new tool + loop) | Large |
| 9 | Cross-task learning (2.6) | autonomous.go, planner/store.go | Medium |
| 10 | Tool hooks (3.1) | agent.go (new HookRunner) | Medium |
| 11 | Workflow hooks (3.2) | autonomous.go, tasks.go | Small |
| 12 | Parallel workers (4.1) | autonomous.go (rewrite scheduler) | Large |
| 13 | Quick actions (4.2) | agent.go, ws.go | Medium |
| 14 | Keyboard shortcuts (4.3) | AppShell.tsx | Small |
| 15 | Task templates (4.4) | HorizontalRail.tsx, new JSON | Small |
| 16 | Batch operations (4.5) | planner components | Medium |
