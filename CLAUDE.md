# Soul v2 — Spec-Driven Chat Interface

Go + React/TypeScript monorepo. AI-agent-maintained, spec-driven chat interface with Claude OAuth, multi-session support, and 7-layer verification stack.

## Quick Commands

```bash
make build          # Build all 13 server binaries + frontend
make serve          # Build and run all 13 servers
make verify         # Run L1-L3 verification (static + unit + integration)
make verify-static  # Go vet + tsc --noEmit + secret scan + dep audit
make types          # Generate types.ts from specs
make clean          # Remove build artifacts
```

## Architecture

```
cmd/
  chat/main.go                Chat server CLI entrypoint (:3002)
  tasks/main.go               Tasks server CLI entrypoint (:3004)
  tutor/main.go               Tutor server CLI entrypoint (:3006)
  projects/main.go            Projects server CLI entrypoint (:3008)
  observe/main.go             Observe server CLI entrypoint (:3010)
  infra/main.go               Infra server CLI entrypoint (:3012) — devops, dba, migrate
  quality/main.go             Quality server CLI entrypoint (:3014) — compliance, qa, analytics
  data/main.go                Data server CLI entrypoint (:3016) — dataeng, costops, viz
  docs/main.go                Docs server CLI entrypoint (:3018) — docs, api
  scout/main.go               Scout server CLI entrypoint (:3020) — lead pipeline CRM
  sentinel/main.go            Sentinel server CLI entrypoint (:3022) — CTF challenge platform
  mesh/main.go                Mesh server CLI entrypoint (:3024) — distributed compute
  bench/main.go               Bench server CLI entrypoint (:3026) — LLM benchmarking
pkg/
  auth/                       Claude OAuth — shared by all servers
  events/                     Logger interface + Event type
internal/chat/
  server/                     HTTP server + SPA serving + product proxy (8 proxies)
  session/                    SQLite session CRUD (chat.db) — memories, custom tools, sessions
  stream/                     Claude API streaming — SSE parse, tool-use accumulation
  ws/                         WebSocket hub — multi-session routing, tool dispatch, BuiltinExecutor, subagent
  context/                    Product context provider — 21 products, system prompts, tool defs, dispatcher
  metrics/                    Event logging, aggregation, CLI reporting
internal/tasks/
  server/                     HTTP server, REST API, SSE broadcaster, comments API
  store/                      SQLite task CRUD (tasks.db) — dependencies, substeps, brainstorm, comments
  executor/                   Autonomous execution engine with hooks, phases, gates
  watcher/                    Comment watcher — background polling, mini-agent dispatch
  hooks/                      Tool/workflow lifecycle hooks from ~/.soul-v2/hooks.json
  phases/                     PhaseRunner — 3-phase pipeline (impl → review → fix)
  gates/                      Merge gates — PreMerge, SmokeTest, RuntimeGate, FeatureGate
internal/tutor/
  server/                     HTTP server, REST API, tool execution
  store/                      SQLite CRUD (tutor.db) — 11 tables
  modules/                    5 modules (DSA, AI, Behavioral, Mock, Planner) + SM-2 + importer
internal/projects/
  server/                     HTTP server, REST API, tool execution
  store/                      SQLite CRUD (projects.db) — 7 tables
  content/                    Embedded implementation guides (go:embed, 11 markdown files)
internal/observe/
  server/                     HTTP server — pillar-based metrics API (13 endpoints)
internal/infra/
  server/                     HTTP server — devops, dba, migrate stub tools
internal/quality/
  server/                     HTTP server — compliance engine + qa/analytics stubs
  compliance/                 5 analyzers, fix engine, 4 reporters, YAML rules (SOC2/HIPAA/GDPR)
internal/dataprod/
  server/                     HTTP server — dataeng, costops, viz stub tools
internal/docsprod/
  server/                     HTTP server — docs, api stub tools
internal/sentinel/
  store/                      SQLite (sentinel.db) — challenges, attempts, completions, guardrails
  engine/                     Challenge sessions, Claude API chatbot, sandbox with weakness levels
  challenges/                 14 embedded CTF challenges (go:embed)
  server/                     HTTP server — 8 tool endpoints
internal/bench/
  scoring/                    7 scoring methods (json_schema, keywords, code, steps, label, number, function)
  prompts/                    33 embedded JSON prompts (go:embed), 10 categories + smoke tests
  harness/                    Benchmark runner + CARS metric calculation
  results/                    Result storage and comparison
  server/                     HTTP server — 6 endpoints
internal/mesh/
  store/                      SQLite (mesh.db) — nodes, heartbeats, peers, linking codes
  node/                       NodeInfo, capability scoring (0-60), stable UUID
  election/                   Hub election with 20% hysteresis
  discovery/                  Tailscale + mDNS peer discovery
  transport/                  WebSocket + JWT auth, exponential backoff
  hub/                        Node registry, heartbeat aggregation
  agent/                      Heartbeat loop, command execution
  server/                     HTTP + WebSocket server
internal/scout/
  store/                      SQLite (scout.db) — 7 tables (leads, stage_history, sync, optimizations, agents)
  pipelines/                  5 pipeline types (job, freelance, contract, consulting, product-dev)
  sweep/                      Platform crawler + Chrome DevTools Protocol
  profiledb/                  PostgreSQL client (pgx/v5) for portfolio data
  agent/                      Claude subprocess for profile optimization
  server/                     HTTP server — 23 REST endpoints
web/src/
  main.tsx                    Entry — RouterProvider with lazy-loaded routes
  router.tsx                  14 routes (/, /chat, /tasks, /tutor, /projects, /observe, /scout, /sentinel, /mesh, /bench)
  layouts/
    AppLayout.tsx             Shared header + nav (10 items) + Outlet
  pages/
    ChatPage.tsx              Chat interface
    DashboardPage.tsx         System overview
    TasksPage.tsx             Kanban board — Backlog/Active/Validation/Done/Blocked/Brainstorm
    TaskDetailPage.tsx        Single task view with activity timeline
    TutorPage.tsx             Interview prep — 5 tabs
    DrillPage.tsx             Interactive quiz drill
    MockPage.tsx              Mock interview session
    ProjectsPage.tsx          Skill-building projects — 4 tabs
    ProjectDetailPage.tsx     Single project detail
    ObservePage.tsx           Pillar-based observability — 8 tabs
    ScoutPage.tsx             Lead pipeline CRM — 5 tabs (Pipeline, Analytics, Actions, Profile, Intelligence)
    SentinelPage.tsx          CTF challenge platform — 3 tabs (Challenges, Sandbox, Progress)
    MeshPage.tsx              Distributed compute — 2 tabs (Cluster, Nodes)
    BenchPage.tsx             LLM benchmarking — 3 tabs (Run, Results, Compare)
  components/
    scout/                    13 components (PipelineBoard, LeadCard, LeadDetail, etc.)
    sentinel/                 6 components (ChallengeList, ChallengeSession, SandboxConfig, etc.)
    mesh/                     4 components (ClusterStatus, NodeList, NodeDetail, LinkingPanel)
    bench/                    5 components (BenchRunner, SmokeTest, ResultsTable, ResultDetail, CompareView)
  hooks/                      useChat, useTasks, useTutor, useProjects, useObserve, useScout, useSentinel, useMesh, useBench
  lib/                        types.ts (generated), ws.ts, api.ts
specs/                        YAML module specs (source of truth)
tests/                        Integration, E2E, load, verification
tools/                        specgen, monitor
```

## Chat Product Routing

Chat sessions can be bound to any of 21 products via the tool selector in ChatInput. When bound:
- `internal/chat/context/` injects product-specific system prompt + Claude tool definitions
- Claude responds with `tool_use` blocks → `ws/handler.go` dispatches via `context/dispatch.go` to product REST APIs
- Built-in tools (memory_*, tool_*, custom_*, subagent) are handled in-process by `BuiltinExecutor` before product dispatch
- Multi-turn tool loop: up to 5 rounds of tool_use → tool_result → follow-up per message
- Multi-session WebSocket: concurrent sessions per connection, per-session agent contexts
- WS protocol: `session.setProduct` (inbound), `session.productSet` / `tool.call` / `tool.complete` (outbound)
- Product stored per-session in SQLite (`sessions.product` column)
- Default mode (no product): built-in tools only (memories, custom tools, subagent)

Tool counts by product:
- Core: Tasks (6), Tutor (7), Projects (6), Observe (4)
- Smart Agents: Scout (21), Sentinel (7), Mesh (4), Bench (4)
- Quality: Compliance (4), QA (2), Analytics (2)
- Infrastructure: DevOps (2), DBA (2), Migrate (2)
- Data: DataEng (2), CostOps (2), Viz (2)
- Documentation: Docs (2), API (2)
- Built-in (all contexts): Memories (4), Custom Tools (3), Subagent (1)

Total: 85 product tools + 8 built-in = 93 tools.

## Environment Variables

| Variable | Default | Purpose |
|----------|---------|---------|
| `SOUL_V2_PORT` | `3002` | Server port |
| `SOUL_V2_HOST` | `127.0.0.1` | Bind address |
| `SOUL_V2_DATA_DIR` | `~/.soul-v2` | Data directory |
| `SOUL_TASKS_HOST` | `127.0.0.1` | Tasks server bind address |
| `SOUL_TASKS_PORT` | `3004` | Tasks server port |
| `SOUL_TASKS_URL` | `http://127.0.0.1:3004` | Tasks server URL (for chat proxy) |
| `SOUL_TUTOR_HOST` | `127.0.0.1` | Tutor server bind address |
| `SOUL_TUTOR_PORT` | `3006` | Tutor server port |
| `SOUL_TUTOR_URL` | `http://127.0.0.1:3006` | Tutor server URL (for chat proxy) |
| `SOUL_PROJECTS_HOST` | `127.0.0.1` | Projects server bind address |
| `SOUL_PROJECTS_PORT` | `3008` | Projects server port |
| `SOUL_PROJECTS_URL` | `http://127.0.0.1:3008` | Projects server URL (for chat proxy) |
| `SOUL_OBSERVE_HOST` | `127.0.0.1` | Observe server bind address |
| `SOUL_OBSERVE_PORT` | `3010` | Observe server port |
| `SOUL_OBSERVE_URL` | `http://127.0.0.1:3010` | Observe server URL (for chat proxy) |
| `SOUL_INFRA_HOST` | `127.0.0.1` | Infra server bind address |
| `SOUL_INFRA_PORT` | `3012` | Infra server port |
| `SOUL_INFRA_URL` | `http://127.0.0.1:3012` | Infra server URL (for chat proxy) |
| `SOUL_QUALITY_HOST` | `127.0.0.1` | Quality server bind address |
| `SOUL_QUALITY_PORT` | `3014` | Quality server port |
| `SOUL_QUALITY_URL` | `http://127.0.0.1:3014` | Quality server URL (for chat proxy) |
| `SOUL_DATA_HOST` | `127.0.0.1` | Data server bind address |
| `SOUL_DATA_PORT` | `3016` | Data server port |
| `SOUL_DATA_URL` | `http://127.0.0.1:3016` | Data server URL (for chat proxy) |
| `SOUL_DOCS_HOST` | `127.0.0.1` | Docs server bind address |
| `SOUL_DOCS_PORT` | `3018` | Docs server port |
| `SOUL_DOCS_URL` | `http://127.0.0.1:3018` | Docs server URL (for chat proxy) |
| `SOUL_SCOUT_HOST` | `127.0.0.1` | Scout server bind address |
| `SOUL_SCOUT_PORT` | `3020` | Scout server port |
| `SOUL_SCOUT_URL` | `http://127.0.0.1:3020` | Scout server URL (for chat proxy) |
| `SOUL_SCOUT_PG_URL` | *(none)* | PostgreSQL connection for scout profile DB |
| `SOUL_SCOUT_CDP_URL` | *(none)* | Chrome DevTools Protocol endpoint for browser automation |
| `SOUL_SENTINEL_HOST` | `127.0.0.1` | Sentinel server bind address |
| `SOUL_SENTINEL_PORT` | `3022` | Sentinel server port |
| `SOUL_SENTINEL_URL` | `http://127.0.0.1:3022` | Sentinel server URL (for chat proxy) |
| `SOUL_MESH_HOST` | `127.0.0.1` | Mesh server bind address |
| `SOUL_MESH_PORT` | `3024` | Mesh server port |
| `SOUL_MESH_URL` | `http://127.0.0.1:3024` | Mesh server URL (for chat proxy) |
| `SOUL_BENCH_HOST` | `127.0.0.1` | Bench server bind address |
| `SOUL_BENCH_PORT` | `3026` | Bench server port |
| `SOUL_BENCH_URL` | `http://127.0.0.1:3026` | Bench server URL (for chat proxy) |
| `SOUL_V2_REPO_DIR` | `(cwd)` | Project root for worktree creation |

Auth: `~/.claude/.credentials.json` (Claude Max OAuth, read-only)

## Conventions — Go

- Go 1.24+, standard library preferred
- All Claude API calls through `internal/chat/stream/` — never direct HTTP
- Parameterized SQL queries only (`?` placeholders) — never string concat
- No hardcoded secrets — env vars or Vaultwarden
- Error returns, not panics
- Every public function tested

## Conventions — Frontend

- React 19, Vite 7, TypeScript 5.9, Tailwind CSS v4
- Dark theme (zinc palette)
- `data-testid` on every interactive/verifiable element
- `types.ts` is generated from specs — never edit manually
- Never set inner HTML directly — use React components
- Warnings are errors

## Conventions — Testing

- Unit tests for every public function
- Property-based tests for all parsers
- Integration tests for API contracts
- E2E via Playwright on titan-pc
- No self-reported success — machine output only

## Conventions — Security

- Never hardcode secrets
- Never concat SQL
- OAuth tokens: 0600 permissions, never logged, never sent to frontend
- CSP headers on all responses
- Origin validation on WebSocket upgrade

## Conventions — Commits

- Prefix: init, feat, fix, refactor, test, spec, chore, docs
- One logical change per commit
- Every commit must pass `make verify-static`
