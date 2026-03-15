# Soul v1 → v2 Full Migration Design

**Date:** 2026-03-15
**Status:** Approved
**Scope:** Migrate all 21 Soul v1 products + 10 cross-cutting capabilities to Soul v2

## Overview

Port the entire Soul v1 product ecosystem into Soul v2's microservice architecture. This includes 12 data products (grouped into 4 servers), 4 smart agent products (individual servers), and 10 cross-cutting capabilities added to existing packages.

**Final state:** 13 servers, 21 products, 93 chat tools, 4 new frontend pages.

## Architecture: Server Layout

| Server | Port | Products | DB |
|--------|------|----------|----|
| `soul-chat` | 3002 | chat | chat.db |
| `soul-tasks` | 3004 | tasks | tasks.db |
| `soul-tutor` | 3006 | tutor | tutor.db |
| `soul-projects` | 3008 | projects | projects.db |
| `soul-observe` | 3010 | observe | (reads event logs) |
| `soul-infra` | 3012 | devops, dba, migrate | (stateless) |
| `soul-quality` | 3014 | qa, compliance, analytics | (stateless) |
| `soul-data` | 3016 | dataeng, costops, viz | (stateless) |
| `soul-docs` | 3018 | docs, api | (stateless) |
| `soul-scout` | 3020 | scout | scout.db + PostgreSQL |
| `soul-sentinel` | 3022 | sentinel | sentinel.db |
| `soul-mesh` | 3024 | mesh | mesh.db |
| `soul-bench` | 3026 | bench | (JSON prompts) |

### Directory Structure (new)

```
cmd/infra/main.go
cmd/quality/main.go
cmd/data/main.go
cmd/docs/main.go
cmd/scout/main.go
cmd/sentinel/main.go
cmd/mesh/main.go
cmd/bench/main.go

internal/infra/
internal/quality/
internal/data/
internal/docs/
internal/scout/
internal/sentinel/
internal/mesh/
internal/bench/
```

Each server follows v2's existing pattern: `server/server.go` (HTTP router + middleware), `store/store.go` (SQLite CRUD where applicable), product-specific packages for business logic.

### Product Registration Updates

**Session store (`internal/chat/session/store.go`):** The `SetProduct()` method's `valid` map must be expanded from the current 5 entries (empty, tasks, tutor, projects, observe) to include all 19 non-chat products (20 total entries including empty string): existing 4 + scout, sentinel, mesh, bench, compliance, qa, analytics, devops, dba, migrate, dataeng, costops, viz, docs, api.

**Dispatcher (`internal/chat/context/dispatch.go`):** Replace the current hardcoded 4-product constructor with a registration-based approach. `NewDispatcher()` accepts a `map[string]ProductConfig` where each entry has `baseURL string` and `tools []ToolDef`. Products register via env-var-sourced URLs:
- Grouped products share a base URL but route to different `/api/{product}/tools/{name}/execute` paths within their server.
- Smart agent products get their own base URL.
- This avoids 17 individual case blocks — the dispatcher does a map lookup on product name, constructs the URL, and POSTs to `/api/tools/{name}/execute`.

**Context provider (`internal/chat/context/context.go`):** `ForProduct()` gains entries for all 17 new products with their system prompts and tool definitions.

### Environment Variables (new)

```
SOUL_INFRA_HOST/PORT/URL     (default 127.0.0.1:3012)
SOUL_QUALITY_HOST/PORT/URL   (default 127.0.0.1:3014)
SOUL_DATA_HOST/PORT/URL      (default 127.0.0.1:3016)
SOUL_DOCS_HOST/PORT/URL      (default 127.0.0.1:3018)
SOUL_SCOUT_HOST/PORT/URL     (default 127.0.0.1:3020)
SOUL_SENTINEL_HOST/PORT/URL  (default 127.0.0.1:3022)
SOUL_MESH_HOST/PORT/URL      (default 127.0.0.1:3024)
SOUL_BENCH_HOST/PORT/URL     (default 127.0.0.1:3026)
SOUL_SCOUT_PG_URL            (PostgreSQL connection string, titan-pc)
SOUL_SCOUT_CDP_URL            (Chrome DevTools endpoint, ws://127.0.0.1:9222)
SOUL_SCOUT_DATA_DIR           (~/.soul-v2/scout/)
SOUL_MESH_NAME/ROLE/PORT/SECRET/HUB
```

---

## Cross-Cutting Capabilities

### 1. Agent Memories

Added to `internal/chat/session/`.

**New table:**
```sql
CREATE TABLE IF NOT EXISTS memories (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    key TEXT NOT NULL UNIQUE,
    content TEXT NOT NULL,
    tags TEXT DEFAULT '',
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_memories_key ON memories(key);
```

**Store methods:** UpsertMemory (ON CONFLICT DO UPDATE), GetMemory, SearchMemories (LIKE on key/content/tags), ListMemories (limit, ordered by updated_at DESC), DeleteMemory.

**Chat integration:** 4 built-in tools (memory_store, memory_search, memory_list, memory_delete). System prompt: "You have persistent memory across conversations." On session start: load recent 20 memories into context.

**Built-in tool dispatch:** A new `BuiltinExecutor` is composed before the product `Dispatcher` in the tool call loop inside `ws/handler.go`. The tool loop in `runStream` checks `BuiltinExecutor.CanHandle(toolName)` first (prefix match: `memory_`, `tool_`, `custom_`, `subagent`). If handled, execution stays in-process and the result is returned directly. Only unhandled tools fall through to `Dispatcher.Execute()` for product REST routing. This avoids modifying the Dispatcher interface and keeps built-in tools zero-latency.

### 2. Custom Tools

Added to `internal/chat/session/`.

**New table:**
```sql
CREATE TABLE IF NOT EXISTS custom_tools (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL UNIQUE,
    description TEXT NOT NULL,
    input_schema TEXT NOT NULL,
    command_template TEXT NOT NULL,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_custom_tools_name ON custom_tools(name);
```

**Store methods:** CreateCustomTool, ListCustomTools, GetCustomTool, DeleteCustomTool.

**Execution:** Tools with `custom_` prefix → extract name → load from DB → parse input JSON → parameters passed as environment variables (`PARAM_<name>`) to `bash -c "$command_template"` — NOT string-interpolated into the command string. This prevents shell injection. 60s timeout, truncate output to 5000 chars.

**Built-in tools:** tool_create, tool_list, tool_delete — routed via `tool_` prefix through `BuiltinExecutor` (see §1 above).

### 3. Subagent

Added to `internal/chat/ws/`.

**New tool:** `subagent` — input: task (string, required), max_iterations (int, optional, default 5, max 10).

**Implementation:** Creates fresh `stream.Client` call with read-only tool set (file_read, file_search, file_grep, file_glob — NO write/exec). Runs tool loop: Claude call → tool_use → execute read-only tool → tool_result → repeat. Max iterations capped at 10. Truncate final result to 3000 chars.

### 4. Task Dependencies

Added to `internal/tasks/store/`.

**New table:**
```sql
CREATE TABLE IF NOT EXISTS task_dependencies (
    task_id INTEGER REFERENCES tasks(id) ON DELETE CASCADE,
    depends_on INTEGER REFERENCES tasks(id) ON DELETE CASCADE,
    PRIMARY KEY (task_id, depends_on)
);
```

**Store methods:** AddDependency, RemoveDependency.

**NextReady query:** SELECT from tasks WHERE stage='backlog' AND NOT EXISTS (unresolved dependencies where dep.stage != 'done'), ORDER BY priority DESC, created_at ASC.

**API endpoints:** POST `/api/tasks/{id}/dependencies` (add), DELETE `/api/tasks/{id}/dependencies/{depId}` (remove).

### 5. Task Substeps

Added to `internal/tasks/store/`.

**New column:** `tasks.substep TEXT DEFAULT ''`

**Substep enum:** tdd → implementing → reviewing → qa_test → e2e_test → security_review. Canonical ordering with Next() method. Tracked during active stage. Updated via task PATCH endpoint and executor.

**Store updates required:** Add `"substep"` to the `allowed` fields map in `store.Update()` so PATCH requests can set it. Add `substep` to SELECT columns in `Get()`, `List()`.

### 6. Brainstorm Stage

Added to `internal/tasks/`.

Stage enum gains `brainstorm` between backlog and active: backlog → {brainstorm, active}, brainstorm → {active}.

**Store updates required:** Add `"brainstorm"` to `validStages` map in `store.go` so stage transitions to brainstorm are accepted by `Update()`.

System prompt override: "You are in brainstorming mode. Ask clarifying questions, propose approaches. No code generation." Executor skips brainstorm tasks (user-driven only).

### 7. Multi-Session WebSocket

Modify `internal/chat/ws/`.

**Current:** One agent per WS connection. `Client` holds a single `sessionID` via `Subscribe()/SessionID()`.
**New:** `chatSession` struct with `agents map[string]agentEntry` and `sync.Mutex`, owned by `MessageHandler` (one per WS connection).

**Concurrency model:**
- `agents map[string]agentEntry` is protected by `chatSession.mu sync.Mutex`.
- Each agent goroutine gets its own `context.WithCancel(context.Background())` — NOT derived from `client.Context()`. This allows per-session cancellation without killing the connection. The current `runStream` call using `context.WithTimeout(client.Context(), ...)` must be replaced — the new goroutine spawning in the `chatSession` becomes the sole owner of agent context lifecycle.
- `client.Subscribe(sessionID)` becomes a no-op for routing purposes — message routing uses the sessionID in each WS message frame instead of connection-level state.
- `client.Send()` is already goroutine-safe (serialized by the hub's write pump), so concurrent agents can send to the same connection.

Per `chat.send`: lock mu → extract sessionID → cancel previous agent for THAT session only → unlock → spawn new agent goroutine keyed by sessionID.

Per `chat.stop`: lock mu → cancel specific session's agent by sessionID → remove from map → unlock.

**New WS inbound message:** `chat.stop` gains a `sessionId` field to target a specific session's agent.

Session resumption: load last 50 messages from DB on reconnect. Session summary: after agent completes, call haiku for title+summary generation.

### 8. Comment Watcher

New `internal/tasks/watcher/`.

**CommentWatcher:** Background goroutine, polls every 5s. Fetches new user comments via CommentsAfter(lastID). Checks task is in actionable stage (active, validation, blocked). If not actionable: post reply. If actionable: spawn mini-agent with task context + comment history, read-only tools + task_comment, post response as author="soul".

**New store methods:** CommentsAfter(lastID), InsertComment(taskID, author, type, body).

**New table:**
```sql
CREATE TABLE IF NOT EXISTS task_comments (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    task_id INTEGER REFERENCES tasks(id) ON DELETE CASCADE,
    author TEXT NOT NULL,
    type TEXT NOT NULL,
    body TEXT NOT NULL,
    created_at TEXT NOT NULL
);
```

**API:** POST/GET `/api/tasks/{id}/comments`.

### 9. Merge Gates

New `internal/tasks/gates/`.

- **PreMergeGate(worktreeWeb):** Symlink node_modules, run tsc --noEmit, run vite build.
- **SmokeTest(serverURL, e2eHost, e2eRunnerPath):** SSH to e2eHost, run node test-runner.js. Returns AllPass + []SmokeCheck.
- **RuntimeGate:** Check for JS console errors.
- **StepVerificationGate:** Runs PreMerge + RuntimeGate.
- **VisualRegression:** Screenshot diff with threshold.
- **FeatureGate:** CSS selector assertions (exists, visible, text_contains, count, eval).

Integrated into executor after task completion, before stage → validation.

### 10. Hooks & Phases

New `internal/tasks/hooks/` and `internal/tasks/phases/`.

**Hooks** (from `~/.soul-v2/hooks.json`):
- ToolHook: event (before:\*/after:\*), match (glob), command, timeout, denyPattern, action, message.
- WorkflowHook: event (after:merge_to_dev, after:task_done), command, timeout.
- HookRunner: load config, RunToolHook(phase, toolName, vars), RunWorkflowHook(event, vars).
- Template expansion: {file}, {worktree}, {task_id}, {tool_name}, {input}.

**Phases:**
- PhaseConfig: planModel, implModel, reviewModel, fixModel.
- PhaseRunner: RunTask dispatches by workflow type.
  - micro → single agent run (15 iterations).
  - quick/full → 3-phase pipeline: implementation (sonnet, 30-40 iter) → opus diff review (15k char cap, "LGTM" or issues) → opus fix agent (20 iter with thinking, if issues found).

---

## Grouped Data Products

### soul-quality (port 3014)

```
internal/quality/
  server/server.go
  compliance/
    service.go            — scan, fix, badge, report tools
    scan/orchestrator.go  — 5 analyzers, SSE progress
    analyzers/
      secret.go           — 16 regex patterns
      config.go           — YAML/JSON misconfiguration
      git.go              — git history scanning
      deps.go             — dependency auditing
      ast.go              — AST pattern analysis
    fix/fix.go            — auto-remediation (dry-run or apply)
    reporters/
      terminal.go, json.go, html.go, badge.go
    rules/                — HIPAA, SOC2, GDPR (embedded YAML)
  qa/service.go           — analyze + report placeholders
  analytics/service.go    — analyze + report placeholders
```

**Endpoints:**
- POST `/api/compliance/scan` (SSE), `/api/compliance/fix`, GET `/api/compliance/badge`, POST `/api/compliance/report`
- POST `/api/qa/analyze`, `/api/qa/report`
- POST `/api/analytics/analyze`, `/api/analytics/report`
- POST `/api/tools/{name}/execute`, GET `/api/health`

### soul-infra (port 3012)

```
internal/infra/
  server/server.go
  devops/service.go       — analyze + report placeholders
  dba/service.go          — analyze + report placeholders
  migrate/service.go      — analyze + report placeholders
```

**Endpoints:** POST `/api/{devops,dba,migrate}/{analyze,report}`, POST `/api/tools/{name}/execute`, GET `/api/health`.

### soul-data (port 3016)

```
internal/data/
  server/server.go
  dataeng/service.go      — analyze + report placeholders
  costops/service.go      — analyze + report placeholders
  viz/service.go          — analyze + report placeholders
```

**Endpoints:** POST `/api/{dataeng,costops,viz}/{analyze,report}`, POST `/api/tools/{name}/execute`, GET `/api/health`.

### soul-docs (port 3018)

```
internal/docs/
  server/server.go
  docs/service.go         — analyze + report placeholders
  api/service.go          — analyze + report placeholders
```

**Endpoints:** POST `/api/{docs,api}/{analyze,report}`, POST `/api/tools/{name}/execute`, GET `/api/health`.

### Common Pattern

Each product module has a `Service` struct with `ExecuteTool(name string, input json.RawMessage) (string, error)`. Server routes `POST /api/tools/{name}/execute` by splitting on `__` prefix. Placeholder services return `{"status": "not_yet_implemented", "product": "<name>", "tool": "<tool>"}`.

### Chat Context (24 new tools)

- compliance: scan, fix, badge, report (4)
- qa, analytics, devops, dba, migrate, dataeng, costops, viz, docs, api: analyze + report each (20)

---

## Smart Agent Products

### soul-scout (port 3020)

```
internal/scout/
  server/server.go
  store/
    store.go              — 7 tables: leads, stage_history, sync_results,
                            sync_meta, optimizations, agent_runs, platform_trust
    leads.go              — AddLead, GetLead, UpdateLead, ListLeads, ScoredLeads
    analytics.go          — 3-layer: stats, conversion, insights
    sync.go               — SyncResult CRUD, SyncMeta get/set
    optimizations.go      — Optimization CRUD
    agents.go             — AgentRun CRUD
  pipelines/
    pipelines.go          — 5 types (job, freelance, contract, consulting, product-dev)
                            Stage validation per type, terminal states
  sweep/
    sweep.go              — Platform crawler (LinkedIn, GitHub, Naukri, Wellfound)
                            URL dedup, auto-create leads
    cdp.go                — Chrome DevTools Protocol (ws://127.0.0.1:9222)
  profiledb/
    client.go             — PostgreSQL (pgx/v5) to titan-pc portfolio_backup
                            Tables: experience, projects, skill_categories, site_config,
                            education, certifications
  agent/
    launcher.go           — Claude subprocess for profile optimization
                            Playwright MCP, per-platform browser profile
```

**23 REST endpoints** mapping 1:1 to v1 tools: lead CRUD, analytics, sync, sweep, profile, optimizations, agent status/history, scored leads.

**Dependencies:** pgx/v5 (PostgreSQL), go-rod/rod (CDP).

**Note on conventions:** pgx/v5 deviates from v2's "standard library preferred" convention. This is a necessary product requirement — scout's profile data lives in PostgreSQL on titan-pc. Tests that depend on `SOUL_SCOUT_PG_URL` must be skipped when the env var is unset (use `t.Skip("SOUL_SCOUT_PG_URL not set")`). Similarly, CDP/browser tests skip when `SOUL_SCOUT_CDP_URL` is unset.

### soul-sentinel (port 3022)

```
internal/sentinel/
  server/server.go
  store/
    store.go              — 6 tables: challenges, attempts, completions,
                            guardrails, scan_results, sandbox_configs
  engine/
    engine.go             — Challenge sessions, Claude API (haiku) chatbot simulation
                            OAuth from ~/.claude/.credentials.json
    sandbox.go            — Configurable chatbot (system_prompt, guardrails, weakness_level)
  challenges/
    challenges.json       — 15 CTF challenges (go:embed), 7 categories, 10-40 points
```

**8 endpoints:** challenge list/start/submit, attack, sandbox config, defend, scan, progress.

### soul-mesh (port 3024) — Go rewrite

```
internal/mesh/
  server/server.go        — HTTP + WebSocket hub
  store/store.go          — 4 tables: nodes, heartbeats, peers, linking_codes
  node/node.go            — NodeInfo, capability scoring (0-60 pts)
  election/election.go    — Hub election, 20% hysteresis
  discovery/discovery.go  — Tailscale + mDNS peer discovery
  transport/transport.go  — WebSocket + JWT auth, exponential backoff
  hub/hub.go              — Node registry, heartbeat aggregation
  agent/agent.go          — Heartbeat loop (10s), command execution
```

**6 HTTP + 1 WS endpoint:** identity, nodes, status, link, heartbeats, WS /ws/mesh.

**Chat tools (4):** cluster_status, list_nodes, node_info, link_node.

**Dependencies:** golang-jwt/jwt/v5, nhooyr.io/websocket, golang.org/x/net (mDNS).

### soul-bench (port 3026) — Go rewrite

```
internal/bench/
  server/server.go
  prompts/prompts.go      — 33 embedded JSON prompts (go:embed), 10 categories + 3 smoke
  scoring/scoring.go      — 7 methods: json_schema, contains_keywords, code_executes,
                            ordered_steps, exact_match_label, exact_match_number, contains_function
  harness/harness.go      — Load → send to LLM → score → CARS metrics
                            CARS_RAM = Accuracy / (Peak_RAM_GB x Latency_s)
                            CARS_Size = Accuracy / (Model_Size_GB x Latency_s)
                            CARS_VRAM = Accuracy / (Peak_VRAM_GB x Latency_s)
  results/results.go      — Result storage and comparison
```

**6 endpoints:** prompts list/by-category, run, smoke, results list/detail, compare.

**Chat tools (4):** run_benchmark, run_smoke, list_results, compare_results.

---

## Frontend Changes

### New Routes

```
/scout     → ScoutPage
/sentinel  → SentinelPage
/mesh      → MeshPage
/bench     → BenchPage
```

Data products have no dedicated pages — accessed only via chat tools.

### ScoutPage

5-tab layout (Pipeline, Analytics, Actions, Profile, Intelligence).

13 components in `components/scout/`: PipelineBoard, LeadCard, LeadDetail, AnalyticsView, ActionsView, SyncStatus, ProfilePanel, ApprovalDialog, AgentActivity, AgentCards, HotLeadsTable, DigestSummary, IntelligenceView.

Hook: `useScout.ts` — leads CRUD, analytics, sweep, sync, profile, optimizations, agent status.

### SentinelPage

3-tab layout (Challenges, Sandbox, Progress).

6 components in `components/sentinel/`: ChallengeList, ChallengeSession, SandboxConfig, SandboxChat, ProgressBoard, ScanResults.

Hook: `useSentinel.ts` — challenges, attack, submit, sandbox, progress, scan.

### MeshPage

2-tab layout (Cluster, Nodes).

4 components in `components/mesh/`: ClusterStatus, NodeList, NodeDetail, LinkingPanel.

Hook: `useMesh.ts` — nodes, status, heartbeats, link.

### BenchPage

3-tab layout (Run, Results, Compare).

5 components in `components/bench/`: BenchRunner, SmokeTest, ResultsTable, ResultDetail, CompareView.

Hook: `useBench.ts` — prompts, run, smoke, results, compare.

### Navigation

AppLayout.tsx nav gains: Scout, Sentinel, Mesh, Bench.

ChatInput product selector gains all 21 products (existing 4 + 17 new).

### Chat Tool Counts

- Existing: tasks (6) + tutor (7) + projects (6) + observe (4) = 23
- Smart agents: scout (23) + sentinel (7: challenge_list, challenge_start, challenge_submit, attack, sandbox_config, defend, scan) + mesh (4: cluster_status, list_nodes, node_info, link_node) + bench (4) = 38
- Data products: compliance (4) + 10 scaffolded x 2 = 24
- Built-in: memories (4) + custom tools (3) + subagent (1) = 8
- **Total: 93 tools**

---

## Build, Deploy & Configuration

### Makefile

New binary targets: soul-infra, soul-quality, soul-data, soul-docs, soul-scout, soul-sentinel, soul-mesh, soul-bench. `make build` builds all 13 + frontend. `make serve` runs all 13.

### SystemD Services (8 new)

`deploy/soul-v2-{infra,quality,data,docs,scout,sentinel,mesh,bench}.service` — all follow existing pattern with security hardening.

### Nginx

`/etc/nginx/sites-enabled/titan-services.conf` gains proxy_pass entries for 8 new servers.

### Chat Server Proxies

`internal/chat/server/proxy.go` gains 8 new reverse proxies to product servers.

### Go Dependencies (new)

- `github.com/jackc/pgx/v5` — PostgreSQL (scout)
- `github.com/go-rod/rod` — Browser automation CDP (scout)
- `github.com/golang-jwt/jwt/v5` — JWT auth (mesh)
- `nhooyr.io/websocket` — WebSocket transport (mesh)
- `golang.org/x/net` — mDNS (mesh)

### Data Directories

```
~/.soul-v2/
  scout/scout.db, scout/browser-profile/
  sentinel.db
  mesh/mesh.db, mesh/node_id, mesh/config.yaml
  bench/results/
  hooks.json
```

---

## Migration Order

### Phase 1: Cross-Cutting Capabilities

1.1 Agent memories, 1.2 Custom tools, 1.3 Subagent, 1.4 Task dependencies, 1.5 Task substeps, 1.6 Brainstorm stage, 1.7 Multi-session WebSocket, 1.8 Comment watcher, 1.9 Hooks & phases, 1.10 Merge gates.

### Phase 2: Grouped Data Products

2.1 soul-quality (compliance real + qa/analytics stubs), 2.2 soul-infra (stubs), 2.3 soul-data (stubs), 2.4 soul-docs (stubs), 2.5 Chat context for 11 products, 2.6 Makefile + systemd + nginx.

### Phase 3: Smart Agent Products

3.1 soul-sentinel, 3.2 soul-bench (Go rewrite), 3.3 soul-mesh (Go rewrite), 3.4 soul-scout, 3.5 Chat context for 4 products.

### Phase 4: Frontend

4.1 ScoutPage + 13 components, 4.2 SentinelPage + 6 components, 4.3 MeshPage + 4 components, 4.4 BenchPage + 5 components, 4.5 Navigation + product selector, 4.6 Router.

### Phase 5: Integration & Deploy

5.1 Build system, 5.2 SystemD services, 5.3 Nginx, 5.4 Chat proxies, 5.5 Env vars, 5.6 Full verification.

### Dependency Chain

- Phase 1: no external dependencies
- Phase 2: independent of Phase 1
- Phase 3: loosely depends on Phase 1.9 (hooks/phases)
- Phase 4: depends on Phase 2+3 backends
- Phase 5: depends on everything

### Scope Estimate

- ~40 new Go files across 8 `internal/` packages
- ~15 modified Go files in existing packages
- ~30 new React components + 4 pages + 5 hooks
- 8 new systemd services
- ~12,000-15,000 lines Go, ~4,000-5,000 lines TypeScript
