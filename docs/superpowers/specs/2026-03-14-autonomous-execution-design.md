# Soul v2 — Autonomous Execution Layer Design

## Goal

Add autonomous task execution to Soul v2's spec-driven chat interface. Two isolated servers — chat (stable control plane) and tasks (execution engine) — in one monorepo with shared interfaces. Chat is Claude Code in a browser; tasks handles Kanban, autonomous agents, products, and progressive verification. Satisfies all 6 design pillars.

## Architecture Overview

```
┌─────────────────────────────────────────────────────────────┐
│                     Monorepo (soul-v2)                       │
│                                                             │
│  cmd/chat/main.go  ──►  soul chat (:3002)                   │
│  cmd/tasks/main.go ──►  soul tasks (:3003)                  │
│                                                             │
│  pkg/auth/         shared OAuth (stable interface)          │
│  pkg/types/        generated types (from specs)             │
│  pkg/events/       structured event types + emitter         │
│                                                             │
│  internal/chat/    chat server internals (isolated)         │
│  internal/tasks/   tasks server internals (isolated)        │
│                                                             │
│  web/src/          single frontend, routes to both servers  │
│  specs/            YAML module specs                        │
│  skills/           default skill files                      │
└─────────────────────────────────────────────────────────────┘
```

**Key principle:** Chat server never goes down. Tasks server can crash, be restarted from chat, and be developed by the chat agent. Changes to chat are made via Claude Code (human), never by Soul itself.

## Server Separation

### Chat Server (`soul chat`, port 3002)

The stable control plane. Three responsibilities:

1. **Conversation** — existing WebSocket chat with Claude, session management, streaming, smart titles. No changes to this core.
2. **Code Agent** — new. Executes tools server-side (read/write/edit files, bash, search, grep) scoped to project directories. Chat server's own source code is read-only unless explicitly unlocked. This makes Soul Chat equivalent to Claude Code in a browser.
3. **Tasks server bridge** — proxies task/product requests to the tasks server. Browser only connects to :3002. If tasks server is down, chat still works — task requests return errors, UI shows tasks server as unavailable.

```
Browser ←WebSocket→ Chat Server (:3002) ←REST/SSE→ Tasks Server (:3003)
```

### Chat-to-Tasks Proxy

The chat server reverse-proxies all `/api/tasks/*` and `/api/products/*` routes to the tasks server. Configuration via env var:

```
SOUL_TASKS_URL=http://127.0.0.1:3003   # default
```

**Request proxying:** Chat server forwards HTTP requests to the tasks server using `httputil.ReverseProxy`. Routes:
- `/api/tasks/*` → `SOUL_TASKS_URL/api/tasks/*`
- `/api/products/*` → `SOUL_TASKS_URL/api/products/*`

**SSE relay:** A background goroutine in the chat server maintains a persistent HTTP connection to `SOUL_TASKS_URL/api/stream` (SSE). Events received are forwarded to relevant browser WebSocket clients as `task.status`, `task.activity`, and `product.health` messages. If the SSE connection drops, reconnect with exponential backoff (1s, 2s, 4s, max 30s). During disconnection, the frontend shows tasks server as "unavailable."

**Failure behavior:**
- Tasks server down → proxy returns HTTP 503 with `{"error": "tasks server unavailable"}`
- Tasks server slow (>5s) → proxy times out, returns 504
- SSE stream disconnected → frontend shows degraded state, chat continues normally

### Tasks Server (`soul tasks`, port 3003)

The execution engine. Manages tasks, runs autonomous agents, handles products, performs verification.

**REST API:**

| Method | Path | Purpose |
|--------|------|---------|
| GET | /api/tasks | List tasks (filterable by stage, product) |
| POST | /api/tasks | Create task |
| GET | /api/tasks/:id | Get task detail |
| PATCH | /api/tasks/:id | Update task (title, description, stage) |
| POST | /api/tasks/:id/start | Trigger autonomous execution |
| POST | /api/tasks/:id/stop | Abort running task |
| GET | /api/tasks/:id/activity | Task activity log (structured events) |
| GET | /api/products | List products with health status |
| POST | /api/products/:name/restart | Restart a product |
| GET | /api/stream | SSE stream (task status, activity, product health) |

### Database Isolation

- Chat: `~/.soul-v2/chat.db` (sessions, messages)
- Tasks: `~/.soul-v2/tasks.db` (tasks, stages, worktrees, activity)

Separate databases ensure chat survives even if tasks DB is corrupted during a bad migration.

## Repository Structure

```
cmd/
  chat/main.go                    → soul chat binary (port 3002)
  tasks/main.go                   → soul tasks binary (port 3003)

pkg/                              → shared, stable, interface-driven
  auth/                           → OAuth token loading/refresh
  types/                          → Generated types (from specs)
  events/                         → Structured event types + emitter interface

internal/
  chat/                           → chat server internals
    server/                       → HTTP server, SPA serving, middleware
    session/                      → SQLite session CRUD (chat.db)
    stream/                       → Claude API streaming, SSE parse
    ws/                           → WebSocket hub, handler, message types
    agent/                        → Chat agent: tools, skill loader, scoped execution
    metrics/                      → Chat-side metrics

  tasks/                          → tasks server internals
    server/                       → HTTP server, REST API, SSE broadcaster
    store/                        → SQLite task CRUD (tasks.db)
    executor/                     → Autonomous pipeline: worktree, agent loop, step-verify-fix
    gates/                        → Progressive verification gates (L1-L7)
    products/                     → Product manager, gRPC lifecycle, health checks
    worktree/                     → Git worktree manager

web/src/
  pages/
    Chat/                         → Landing page (existing)
    Dashboard/                    → /dashboard — overview hub
    Tasks/                        → /tasks — Kanban board
    Products/                     → /p/:product — product pages
  components/                     → Shared UI components
  hooks/                          → Shared hooks
  lib/                            → types.ts (generated), api.ts, ws.ts

specs/                            → YAML module specs (source of truth)
skills/                           → Default skill files (plan, execute, tdd, verify, review)
```

### Key moves from current structure

- `internal/auth/` → `pkg/auth/` (shared by both servers). Current auth depends on `*metrics.EventLogger` — refactor to accept a `Logger` interface defined in `pkg/events/`. Each server injects its own logger implementation.
- `internal/session/` → `internal/chat/session/`. Current DB file `sessions.db` is renamed to `chat.db`. On first startup, if `chat.db` doesn't exist but `sessions.db` does, rename it automatically (zero-downtime migration).
- `internal/stream/` → `internal/chat/stream/`
- `internal/ws/` → `internal/chat/ws/`
- `internal/server/` → `internal/chat/server/`
- `internal/metrics/` → `internal/chat/metrics/`
- `cmd/soul/main.go` → `cmd/chat/main.go`. The existing `soul serve` command becomes `soul chat`. `soul metrics` is preserved and reads from both `chat.jsonl` and `tasks.jsonl`.
- New: `internal/tasks/` tree, `internal/chat/agent/`, `cmd/tasks/main.go`

### Build targets

```makefile
build:       go build -o soul-chat ./cmd/chat && go build -o soul-tasks ./cmd/tasks
build-web:   cd web && npx vite build
serve:       make build && ./soul-chat & ./soul-tasks
```

Both binaries share the same `pkg/` layer. Each can be built and deployed independently.

## Chat Agent — Code Tools

The chat agent gives Soul the capabilities of Claude Code in a browser.

```
internal/chat/agent/
  agent.go          → Agent loop (prompt → Claude API → tool calls → results → repeat)
  tools.go          → Tool registry + dispatch
  tools_file.go     → read, write, edit, search, grep (scoped to project dir)
  tools_bash.go     → bash execution (scoped, path-protected)
  tools_tasks.go    → REST calls to tasks server (create/start/stop tasks)
  skills.go         → Skill loader: reads ~/.soul-v2/skills/*.md, injects into prompt
  scope.go          → Path scoping + protection rules
```

### Scoping rules

- Agent can read/write/exec within any configured project directory
- Chat server source (`internal/chat/`, `cmd/chat/`) is **read-only** unless explicitly unlocked
- Tasks server source (`internal/tasks/`, `cmd/tasks/`) and product code — fully writable
- Skills directory (`~/.soul-v2/skills/`) — readable and writable

## Skill System

Skills are markdown files in `~/.soul-v2/skills/` loaded at runtime.

### Default skills

```
~/.soul-v2/skills/
  plan.md              → Decompose task into sub-steps before executing
  execute.md           → Step-verify-fix loop, iteration budgets
  tdd.md               → Write test → fail → implement → pass
  verify.md            → Run progressive gates, report results
  review.md            → Self-review diff before marking complete
```

### Skill selection by context

| Context | Skills loaded |
|---------|--------------|
| Chat conversation (no task) | None — raw Claude conversation |
| User says "plan this" | `plan.md` |
| Autonomous task — micro | `execute.md` |
| Autonomous task — quick | `plan.md`, `execute.md`, `verify.md` |
| Autonomous task — full | `plan.md`, `tdd.md`, `execute.md`, `review.md`, `verify.md` |

### Skill format

```markdown
---
name: plan
description: Decompose a task into incremental sub-steps before executing
trigger: autonomous task with quick or full workflow
---

## Planning Phase
...
```

The `trigger` field helps the skill loader decide relevance. Users can also invoke skills explicitly from chat (`/plan`, `/tdd`). Users can add custom skills — the agent discovers them automatically.

## Autonomous Execution Pipeline

### Workflow classification

```
micro  → trivial changes (add button, fix typo, rename) → 15 iterations
quick  → moderate changes (add feature, fix bug) → 30 iterations
full   → complex changes (refactor, new API, migration) → 40 iterations
```

Classifier uses keyword matching on task title/description. Task metadata can override.

### Execution flow

```
Task started
  → Classify workflow (micro/quick/full)
  → Create worktree: .worktrees/task-<id>/ (branch: task/<id>-<slug>)
  → Inject: CLAUDE.md + relevant files + skill context
  → IF full: Planning phase (agent decomposes into 3-7 sub-steps)
  → IF quick: Lightweight decomposition (2-4 steps)
  → IF micro: Skip planning, just execute
  → Agent loop with step-verify-fix:
      For each step:
        → Agent implements
        → L1-L3 run in worktree (go vet, tsc, unit tests)
        → If fail → agent gets exact error, fixes, re-verify
        → If 3 consecutive failures on same step → task blocked
        → If pass → next step
  → All steps done → progressive gates
  → All gates pass → commit in worktree
  → stage=validation (user reviews from /tasks)
  → User approves → merge to main branch
  → User rejects → task back to active with feedback
```

### Fixes over Soul v1

| Soul v1 Gap | Soul v2 Fix |
|-------------|-------------|
| Merge-then-verify | Verify-then-merge — all gates in worktree, main never touched until pass |
| Unrestricted code_exec | Scoped bash with path protection |
| No step-level verification | L1-L3 after every implementation step |
| Revert failures cascade | No revert needed — worktree is disposable |
| No cost tracking | Every API call, tool call, gate result is a structured event |
| No stuck detection | Loop detector + iteration budget per workflow |
| No cleanup on failure | defer worktree.Cleanup() — always runs |
| Race condition on merge | Single merge queue with mutex |
| No planning phase | Agent decomposes before executing (quick/full workflows) |
| No model routing | Workflow-based: Opus for review, Sonnet for implementation, Haiku for micro |

### Model routing

Different phases use different models to balance cost and capability:

| Phase | Model | Rationale |
|-------|-------|-----------|
| Micro workflow (full task) | Haiku | Trivial changes, minimize cost |
| Quick/Full — planning | Sonnet | Decomposition needs decent reasoning |
| Quick/Full — implementation | Sonnet | Code generation, good balance |
| Opus diff review (Gate 2) | Opus | Catches logic errors machines miss |
| Fix attempts after review | Sonnet | Targeted fixes, not broad reasoning |

Model selection is configurable via `~/.soul-v2/config.yaml`. If a model is rate-limited or unavailable, fall back to the next tier (Opus → Sonnet → Haiku). Rate limit errors are logged as `agent.rate_limit` events.

### Context window management

The agent maintains a context budget to avoid exceeding Claude's context window:

- **Token tracking:** After each API response, record actual input/output tokens from the response `usage` field (not estimated).
- **Budget threshold:** When cumulative context reaches 80% of model limit, trigger compression.
- **Compression strategy:** Summarize tool outputs older than 5 iterations into a single "context summary" message. Preserve the last 3 iterations and all system prompt content intact.
- **Emergency cutoff:** At 95% of limit, stop the current step, commit progress, and emit `agent.context_budget` event with `action: "emergency_stop"`.

### Task lifecycle and recovery

**Blocked tasks:**
- Worktree is **preserved** for manual inspection. User can examine the state from chat.
- "Retry" button on `/tasks/:id` creates a new worktree from a fresh branch, copies the task spec, and re-runs with a clean slate.
- "Resume" button re-uses the existing worktree and continues from the last successful step.

**Server crash recovery:**
- On startup, the tasks server scans `tasks.db` for tasks with `stage=active`.
- Tasks that were `active` but have no running agent process are marked `blocked` with reason `"server restart — execution interrupted"`.
- User can retry from `/tasks` UI or from chat.

**Cleanup policy:**
- `defer worktree.Cleanup()` runs on normal completion (success or blocked).
- Blocked tasks: worktree preserved for 24 hours, then auto-cleaned by a daily cleanup goroutine.
- Merged tasks: worktree + branch deleted immediately after merge.

### Git merge strategy

Squash merge to main — one clean commit per task with message format:

```
feat(task-<id>): <task title>

<task description summary>
Workflow: <micro|quick|full>
Gates: L1 ✓ L2 ✓ L3 ✓ L6 ✓ [L4 ✓] [L5 ✓] [L7 ✓]
```

### Product tool safety

Product tools called from chat require explicit user confirmation for destructive operations. Each product manifest declares tool categories:

```yaml
tools:
  - name: scout_sweep
    category: read      # no confirmation needed
  - name: scout_apply
    category: write     # requires confirmation in chat
```

Categories: `read` (no side effects), `write` (modifiable state), `destructive` (irreversible — always confirm). The autonomous agent running tasks can call `write` tools without confirmation (it's already in an isolated context), but `destructive` always confirms.

## Progressive Verification Gates

All verification runs **inside the worktree** before any merge.

### Per-step verification (during execution)

After every implementation step:

```
Agent implements step N
  → L1: go vet + tsc --noEmit (~5s)
  → L2: Run affected unit tests (~10s)
  → L3: Run affected integration tests (~15s)
  → If fail → agent gets exact error output, fixes, re-verify
  → If 3 consecutive failures → task blocked
  → If pass → next step
```

### Post-completion gates

Progressive — each layer only runs if previous passed and task warrants it.

```
Gate 1 (always):  L1-L3 full suite — all static analysis, all tests (~30s)
Gate 2 (always):  L6 Opus diff review — diff + task spec → PASS/FAIL
                  If FAIL → agent fixes → re-run Gates 1+2 (max 2 cycles)
Gate 3 (if applicable): L4 E2E tests — if task touches frontend/API/WS
Gate 4 (if applicable): L5 Load tests — if task touches streaming/WS hub/DB
Gate 5 (if applicable): L7 Visual regression — if task touches UI/CSS
```

### Gate applicability detection

The executor analyzes the git diff to tag which layers are relevant:

```go
func classifyGates(diff string) []Gate {
    gates := []Gate{GateBuild, GateOpusReview}  // always
    if touchesFrontend(diff) || touchesAPI(diff) || touchesWS(diff) {
        gates = append(gates, GateE2E)
    }
    if touchesStreaming(diff) || touchesDB(diff) || touchesWSHub(diff) {
        gates = append(gates, GateLoad)
    }
    if touchesUI(diff) || touchesCSS(diff) {
        gates = append(gates, GateVisual)
    }
    return gates
}
```

## Product System

Each product runs as a **separate OS process** (gRPC binary). The tasks server manages lifecycle.

```
internal/tasks/products/
  manager.go        → Start/stop/restart product binaries, health checks
  registry.go       → Tool catalog from product manifests
  health.go         → Circuit breaker: 3 failures in 5 min → mark failed, stop retrying
```

Products are configured in `~/.soul-v2/products.yaml`. Manager pings each product every 30s. Health status broadcast via SSE.

Products are available everywhere — both chat agent and autonomous agent can call product tools.

## Structured Event Logging (Transparent Pillar)

Every action emits a structured event. No log.Printf, no ad-hoc strings.

### Event categories

```
task.created          → task metadata, workflow classification
task.started          → worktree path, branch name, iteration budget
task.step.start       → step number, description
task.step.verify      → L1-L3 results per step, pass/fail, duration
task.step.fix         → what failed, agent's fix attempt number
task.step.complete    → step done, tokens used in this step
task.completed        → total tokens, total duration, gate summary
task.blocked          → reason, last error, tokens consumed
task.merged           → branch merged, commit SHA

agent.iteration       → iteration number, tool called, duration
agent.tool_call       → tool name, input summary, output size, duration
agent.stuck           → repeated pattern detected, warning injected
agent.context_budget  → current usage, compression triggered

gate.start            → which gate, why triggered
gate.result           → pass/fail, duration, details

product.started       → product name, PID, binary path
product.health        → product name, status, latency
product.failed        → product name, error, restart count
product.circuit_open  → product name, reason, cooldown period
```

### Storage

- Chat: `~/.soul-v2/metrics/chat.jsonl`
- Tasks: `~/.soul-v2/metrics/tasks.jsonl`

Daily rotation, same JSONL format as existing metrics system.

### CLI commands

```bash
soul metrics task 5          # full event timeline for task #5
soul metrics cost --by-task  # token cost breakdown per task
soul metrics gates           # gate pass/fail rates
soul metrics stuck           # stuck detection frequency
```

### Real-time streaming

Task events broadcast via SSE to chat server, relayed to browser WebSocket. `/tasks` UI shows live activity feed per task.

## Frontend

### Routing

```
/                → Chat (landing page, existing)
/dashboard       → Dashboard hub (overview of everything)
/tasks           → Full Kanban board
/tasks/:id       → Task detail (activity, gates, diff, approve/reject)
/p/:product      → Product page (e.g., /p/scout, /p/compliance)
```

### Navigation

Sidebar rail (left edge):

```
📊 = Dashboard (/dashboard)
💬 = Chat (/)
📋 = Tasks (/tasks)
🔍 = Scout (/p/scout)     ← dynamic from products.yaml
⚙  = Settings
```

Products register in the rail dynamically. No code change to add a product.

### Dashboard (`/dashboard`)

Overview hub with three panels, all reusing existing components:

**Tasks Panel** — compact view of active/blocked tasks
- Reuses `TaskCard` component (shared between /dashboard and /tasks)
- Shows only active + blocked (not full Kanban)
- Click card → `/tasks/:id`

**Chat Panel** — recent sessions with previews
- Reuses `SessionItem` from `SessionList.tsx`
- Shows last 5 sessions by `updatedAt`
- Click session → `/` with session selected

**Products Panel** — health overview
- Renders from `products.yaml` (dynamic)
- Health dot, uptime, tool count per product
- Circuit breaker state visible
- Click product → `/p/:product`

### Component reuse

| Dashboard uses | Source | Strategy |
|----------------|--------|----------|
| `SessionItem` | `SessionList.tsx` | Extract to shared component, `compact` prop |
| `TaskCard` | New shared component | `components/tasks/TaskCard.tsx` |
| `StatusDot` | `SessionList.tsx` | Already extracted |
| `getTimeGroup` | `lib/utils.ts` | Already shared |
| Product health | Tasks server `GET /api/products` | Same endpoint |

### Tasks page (`/tasks`)

Kanban board with columns: Backlog → Active → Validation → Done / Blocked

Task cards show: title, workflow badge, step progress, token cost, status dot.

Task detail (`/tasks/:id`): structured activity timeline, gate results, git diff viewer, approve/reject buttons.

### Product pages (`/p/:product`)

Generic dashboard per product: health status, available tools, recent tool calls, product-specific tasks. Products with dedicated panels can override.

### Frontend structure note

The current frontend has no `pages/` directory — all components live in `web/src/components/`. The new `pages/` directory is introduced for route-level components. Existing chat components (`Shell.tsx`, `ChatInput.tsx`, `MessageList.tsx`, etc.) are moved into `pages/Chat/` or stay in `components/` if shared. Each page is lazy-loaded via `React.lazy()` for code splitting.

## Pillar Compliance

### Performant
- Chat server unchanged — same streaming path, first token < 200ms
- WebSocket overhead unchanged — new SSE relay adds no browser-side WS frames (events are batched)
- Frontend bundle managed via code splitting — `/tasks`, `/dashboard`, `/p/*` are lazy-loaded routes. Chat page bundle stays under 300KB gzipped. Total app budget: 500KB gzipped.
- Server memory: chat server stays < 100MB. Tasks server budget: < 200MB at 5 concurrent tasks (worktrees + agent contexts).
- Iteration budgets prevent runaway execution (micro=15, quick=30, full=40)
- Step-verify-fix catches failures early, doesn't waste tokens
- Skill loading is selective — only relevant skills injected

### Robust
- Verify-before-merge — main branch never touched until all gates pass
- 3-failure step limit — agent can't loop forever
- Merge queue with mutex — no race conditions
- defer worktree.Cleanup() — always runs
- Separate DBs — corrupted tasks.db can't take down chat.db
- Agent tools fuzz-tested for panic-safety (nil paths, empty inputs, oversized content)
- Task views handle boundary conditions: 0 tasks, 100+ tasks, 0 activity events

### Resilient
- Chat survives tasks server crash — separate processes, separate DBs
- Product circuit breaker — 3 failures in 5 min → stop retrying, surface in UI
- Tasks server restartable from chat — agent can restart it
- No revert needed — worktree approach means main stays clean
- Opus review retry budget — max 2 fix cycles then block
- OOM protection: agent context budget with emergency cutoff at 95%
- Server crash recovery: active tasks detected on restart, marked blocked

### Secure
- Scoped execution — tools restricted to project directories
- Chat server code protected — read-only unless unlocked
- Auth shared via pkg/auth/ — single Logger interface, no concrete dependencies
- Browser never hits tasks server — chat proxies, single attack surface
- Inter-server communication: tasks server binds to 127.0.0.1 only (not exposed to network)
- Rate limiting on tasks API (via chat proxy middleware)
- CSP headers preserved through proxy
- All task descriptions, skill content, and agent outputs sanitized before rendering (XSS-proof)
- Product tools categorized: read/write/destructive with confirmation gates

### Sovereign
- Both servers self-hosted
- Products are local gRPC binaries over Unix sockets
- Skills are local markdown files
- SQLite for both DBs
- Claude API abstracted and swappable
- Service worker updated for new routes — offline reading for dashboard and task history

### Transparent
- Every action is a structured event — full taxonomy
- Separate event logs per server with daily rotation
- CLI queryable — task timelines, cost breakdowns, gate rates
- Real-time SSE streaming to browser
- Per-task cost tracking — tokens per step, per gate
- Dashboard shows live state at a glance
- Model usage tracked per phase (which model, how many tokens, cost)

## Health Check Endpoints

Both servers expose `GET /api/health`:

**Chat server** (`GET :3002/api/health`) — existing, unchanged:
```json
{"status": "ok", "uptime": "2h30m", "sessions": 5}
```

**Tasks server** (`GET :3003/api/health`):
```json
{
  "status": "ok",
  "uptime": "2h30m",
  "active_tasks": 2,
  "products": {"scout": "healthy", "compliance": "degraded"},
  "worktrees": 3
}
```

Chat server pings tasks server health every 10s. Status is available to the frontend via `GET :3002/api/tasks/health` (proxied).

## Products Configuration

`~/.soul-v2/products.yaml` schema:

```yaml
products:
  - name: scout              # required — identifier
    binary: /path/to/scout   # required — absolute path to gRPC binary
    label: Scout              # optional — display name (defaults to name)
    color: active             # optional — UI color token
    tools:
      - name: scout_sweep
        category: read        # read | write | destructive
      - name: scout_apply
        category: write
```

Env var override: `SOUL_SCOUT_BIN=/other/path` (name uppercased, hyphens → underscores).

## Iteration Budget Rationale

Budgets derived from Soul v1 empirical data:
- **micro (15):** v1 tasks like "add button" completed in 5-8 iterations. 15 gives 2x headroom.
- **quick (30):** v1 "add feature" tasks completed in 12-20 iterations. 30 gives ~1.5x headroom.
- **full (40):** v1 complex tasks used 25-35 iterations. 40 is the practical upper bound before context window pressure.

These are configurable via `~/.soul-v2/config.yaml` under `execution.iteration_limits`.
