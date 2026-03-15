# V1 Migration Phase 3: Smart Agent Products

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Create 4 individual smart agent product servers: soul-sentinel (:3022), soul-bench (:3026), soul-mesh (:3024), soul-scout (:3020). Each is a faithful port from Soul v1.

**Architecture:** 4 new HTTP servers. Sentinel and Scout use SQLite stores. Mesh uses SQLite + WebSocket + JWT. Bench embeds JSON prompts and implements scoring in Go. Scout additionally uses PostgreSQL (pgx/v5) and browser automation (go-rod).

**Tech Stack:** Go 1.24, net/http, SQLite, Claude API (stream.Client), pgx/v5, go-rod, golang-jwt/jwt/v5, nhooyr.io/websocket

**Spec:** `docs/superpowers/specs/2026-03-15-soul-v1-migration-design.md` §Smart Agent Products

---

## File Map

### soul-sentinel (:3022)

| File | Action | Responsibility |
|------|--------|---------------|
| `cmd/sentinel/main.go` | Create | Entrypoint (:3022) |
| `internal/sentinel/store/store.go` | Create | SQLite: 6 tables (challenges, attempts, completions, guardrails, scan_results, sandbox_configs) |
| `internal/sentinel/store/store_test.go` | Create | Store CRUD tests |
| `internal/sentinel/engine/engine.go` | Create | Challenge sessions, Claude API chatbot simulation, weakness filtering |
| `internal/sentinel/engine/engine_test.go` | Create | Engine tests with mock sender |
| `internal/sentinel/engine/sandbox.go` | Create | Configurable sandbox chatbot |
| `internal/sentinel/challenges/challenges.json` | Create | 14 embedded CTF challenges (go:embed) |
| `internal/sentinel/server/server.go` | Create | HTTP server: 8 tool endpoints + health |

### soul-bench (:3026)

| File | Action | Responsibility |
|------|--------|---------------|
| `cmd/bench/main.go` | Create | Entrypoint (:3026) |
| `internal/bench/prompts/prompts.go` | Create | Embedded 33 JSON prompts (go:embed) |
| `internal/bench/prompts/*.json` | Create | 10 category files + smoke-test.json |
| `internal/bench/scoring/scoring.go` | Create | 7 scoring methods (json_schema, keywords, code, steps, label, number, function) |
| `internal/bench/scoring/scoring_test.go` | Create | Scoring method tests |
| `internal/bench/harness/harness.go` | Create | Benchmark runner + CARS metric calculation |
| `internal/bench/harness/harness_test.go` | Create | Harness tests |
| `internal/bench/results/results.go` | Create | Result storage + comparison |
| `internal/bench/server/server.go` | Create | HTTP server: 6 endpoints + tool execute + health |

### soul-mesh (:3024)

| File | Action | Responsibility |
|------|--------|---------------|
| `cmd/mesh/main.go` | Create | Entrypoint (:3024) |
| `internal/mesh/store/store.go` | Create | SQLite: 4 tables (nodes, heartbeats, peers, linking_codes) |
| `internal/mesh/store/store_test.go` | Create | Store tests |
| `internal/mesh/node/node.go` | Create | NodeInfo, capability scoring (0-60), stable UUID |
| `internal/mesh/node/node_test.go` | Create | Scoring tests |
| `internal/mesh/election/election.go` | Create | Hub election with 20% hysteresis |
| `internal/mesh/election/election_test.go` | Create | Election tests |
| `internal/mesh/discovery/discovery.go` | Create | Tailscale + mDNS peer discovery |
| `internal/mesh/transport/transport.go` | Create | WebSocket + JWT auth, exponential backoff |
| `internal/mesh/hub/hub.go` | Create | Node registry, heartbeat aggregation |
| `internal/mesh/agent/agent.go` | Create | Heartbeat loop, command execution |
| `internal/mesh/server/server.go` | Create | HTTP server + WebSocket: 6 HTTP + 1 WS endpoint |

### soul-scout (:3020)

| File | Action | Responsibility |
|------|--------|---------------|
| `cmd/scout/main.go` | Create | Entrypoint (:3020) |
| `internal/scout/store/store.go` | Create | SQLite: 7 tables (leads, stage_history, sync_results, sync_meta, optimizations, agent_runs, platform_trust) |
| `internal/scout/store/leads.go` | Create | Lead CRUD + scored leads |
| `internal/scout/store/analytics.go` | Create | 3-layer analytics |
| `internal/scout/store/store_test.go` | Create | Store tests |
| `internal/scout/pipelines/pipelines.go` | Create | 5 pipeline definitions, stage validation |
| `internal/scout/pipelines/pipelines_test.go` | Create | Pipeline tests |
| `internal/scout/sweep/sweep.go` | Create | Platform crawler (URL dedup, auto-create leads) |
| `internal/scout/sweep/cdp.go` | Create | Chrome DevTools Protocol connection |
| `internal/scout/profiledb/client.go` | Create | PostgreSQL client (pgx/v5) |
| `internal/scout/agent/launcher.go` | Create | Claude subprocess for profile optimization |
| `internal/scout/server/server.go` | Create | HTTP server: 23 REST endpoints + tool execute |

### Chat Context Integration

| File | Action | Responsibility |
|------|--------|---------------|
| `internal/chat/context/sentinel.go` | Create | 7 tool definitions |
| `internal/chat/context/bench.go` | Create | 4 tool definitions |
| `internal/chat/context/mesh.go` | Create | 4 tool definitions |
| `internal/chat/context/scout.go` | Create | 23 tool definitions |
| `internal/chat/context/context.go` | Modify | Add 4 products to ForProduct() |
| `internal/chat/context/dispatch.go` | Modify | Add routes for 38 tools |
| `internal/chat/server/proxy.go` | Modify | Add 4 proxies |

---

## Task 1: Sentinel — Store

**Files:** `internal/sentinel/store/store.go`, `internal/sentinel/store/store_test.go`

- [ ] **Step 1: Write failing tests**

Test: SeedChallenges loads 14 challenges, ListChallenges with filters, RecordAttempt, CountAttempts, RecordCompletion, IsCompleted, GetProgress, SaveGuardrail, ListGuardrails, SaveScanResult, SaveSandboxConfig, GetDefaultSandboxConfig.

- [ ] **Step 2: Implement store.go**

6 tables (exact schema from v1): challenges, attempts, completions, guardrails, scan_results, sandbox_configs. Open() creates tables + seeds challenges from embedded JSON. All CRUD methods ported from v1.

Key methods: `GetChallenge(id)`, `ListChallenges(category, difficulty, phase)`, `RecordAttempt(challengeID, payload, response, success)`, `RecordCompletion(challengeID, points, turns, hints)` (INSERT OR IGNORE), `GetProgress()` (total_points, completed, categories), `SaveSandboxConfig(sc)`, `GetDefaultSandboxConfig()`.

- [ ] **Step 3: Run tests → ALL PASS**

- [ ] **Step 4: Commit:** `feat: add sentinel store with 6 SQLite tables`

---

## Task 2: Sentinel — Challenges & Engine

**Files:** `internal/sentinel/challenges/challenges.json`, `internal/sentinel/engine/engine.go`, `internal/sentinel/engine/sandbox.go`, `internal/sentinel/engine/engine_test.go`

- [ ] **Step 1: Create challenges.json**

Port all 14 challenges from v1. 7 categories: prompt_injection (3), jailbreaking (3), data_exfiltration (2), indirect_injection (2), tool_abuse (1), multi_turn (2), defense_bypass (2). Each has: id, category, phase, difficulty, points, title, description, objective, system_prompt, flag, tools, hints, max_turns, learn_more.

- [ ] **Step 2: Write engine tests with mock sender**

```go
type mockSender struct{ response string }
func (m *mockSender) Send(ctx context.Context, req *stream.Request) (*stream.Response, error) {
    return &stream.Response{
        StopReason: "end_turn",
        Content: []stream.ContentBlock{{Type: "text", Text: m.response}},
        Usage: &stream.Usage{InputTokens: 50, OutputTokens: 30},
    }, nil
}
```

Test: StartSession creates session, AttackChallenge calls sender and records attempt, submit with correct flag records completion with scoring (base + bonus - hints, min 50%), sandbox attack applies weakness filtering.

- [ ] **Step 3: Implement engine.go**

Engine struct with: store, sessions map (mutex-protected), sender (stream.Client or mock). Methods:
- `StartSession(challengeID, reset)` → creates in-memory Session with system prompt + history
- `AttackChallenge(challengeID, payload)` → appends to history, calls Claude, records attempt, returns response
- `AttackSandbox(payload)` → uses sandbox config, applies guardrails
- `ClearSession(challengeID)` → removes from map

Scoring on submit: `pointsEarned = base + (bonus if turns <= max) - (hints * 5), min = base/2`

- [ ] **Step 4: Implement sandbox.go**

SandboxConfig with weakness levels (none/low/medium/high). Low: relaxed system prompt. Medium: hints at secret. High: includes secret in system prompt context.

- [ ] **Step 5: Run tests → ALL PASS**

- [ ] **Step 6: Commit:** `feat: add sentinel engine with challenge sessions and sandbox`

---

## Task 3: Sentinel — Server

**Files:** `cmd/sentinel/main.go`, `internal/sentinel/server/server.go`

- [ ] **Step 1: Create server.go**

8 tool endpoints mapped to REST:
- GET `/api/challenges` → challenge_list (query: category, difficulty, show_completed)
- POST `/api/challenges/start` → challenge_start (body: challenge_id, reset)
- POST `/api/challenges/submit` → challenge_submit (body: challenge_id, flag)
- POST `/api/attack` → attack (body: mode=challenge|sandbox|product, challenge_id|payload)
- POST `/api/sandbox/config` → sandbox_config
- POST `/api/defend` → defend (body: name, rule, test_against, challenge_id)
- POST `/api/scan` → scan (body: product)
- GET `/api/progress` → progress (query: include_scans, include_guardrails)
- POST `/api/tools/{name}/execute` → chat tool dispatch
- GET `/api/health`

Port 3022, env: SOUL_SENTINEL_HOST/PORT. Requires stream.Client for Claude API calls.

- [ ] **Step 2: Create main.go**

Load OAuth credentials, create stream.Client, create store (open DB in dataDir), create engine, create server.

- [ ] **Step 3: Verify compilation:** `go build ./cmd/sentinel/`

- [ ] **Step 4: Commit:** `feat: add soul-sentinel server with CTF challenge platform`

---

## Task 4: Bench — Scoring

**Files:** `internal/bench/scoring/scoring.go`, `internal/bench/scoring/scoring_test.go`

- [ ] **Step 1: Write failing tests**

Test all 7 scoring methods:
- json_schema: valid JSON with required keys → fractional score
- contains_keywords: partial keyword match → fraction
- code_executes: valid Go syntax with function → 1.0, invalid → 0.0
- ordered_steps: correct order → 1.0, partial → fraction
- exact_match_label: case-insensitive match → 1.0
- exact_match_number: last number extraction → 1.0
- contains_function: substring present → 1.0

- [ ] **Step 2: Implement scoring.go**

7 methods, each returns float64 in [0.0, 1.0]. Dispatcher: `ScoreResult(response string, prompt PromptData) float64`.

Note: v1 uses Python `compile()` for code_executes. In Go, use `go/parser.ParseFile()` to check Go syntax validity, or `go/ast` to find function declarations.

- [ ] **Step 3: Run tests → ALL PASS**

- [ ] **Step 4: Commit:** `feat: add bench scoring with 7 methods`

---

## Task 5: Bench — Prompts, Harness, Server

**Files:** `internal/bench/prompts/`, `internal/bench/harness/`, `internal/bench/results/`, `internal/bench/server/server.go`, `cmd/bench/main.go`

- [ ] **Step 1: Create embedded prompt files**

Port 33 prompts from v1 across 10 categories + smoke-test.json. Each prompt has: id, task, prompt, expected_answer, scoring, scoring_config.

- [ ] **Step 2: Implement prompts.go**

`go:embed *.json`, `LoadAll() []PromptData`, `LoadCategory(name) []PromptData`, `LoadSmoke() []PromptData`.

- [ ] **Step 3: Implement harness.go**

`RunBenchmark(config BenchConfig) (*BenchResult, error)` — loads prompts, sends to LLM endpoint (configurable URL), scores responses, computes CARS metrics. Hardware detection via /proc. Result struct with per-prompt and summary data.

CARS: `accuracy / (resource_metric * latency_s)` — three variants (RAM, Size, VRAM).

- [ ] **Step 4: Implement results.go**

Store results as JSON files in `~/.soul-v2/bench/results/`. `SaveResult()`, `ListResults()`, `GetResult(id)`, `CompareResults(id1, id2)`.

- [ ] **Step 5: Create server.go**

6 endpoints:
- GET `/api/bench/prompts` → list categories
- GET `/api/bench/prompts/{category}` → prompts for category
- POST `/api/bench/run` → run benchmark
- POST `/api/bench/smoke` → run smoke tests
- GET `/api/bench/results` → list results
- GET `/api/bench/results/{id}` → result detail
- GET `/api/bench/compare` → compare two results
- POST `/api/tools/{name}/execute` → chat dispatch
- GET `/api/health`

Port 3026, env: SOUL_BENCH_HOST/PORT.

- [ ] **Step 6: Create main.go and verify compilation**

- [ ] **Step 7: Commit:** `feat: add soul-bench server with benchmark harness and scoring`

---

## Task 6: Mesh — Store & Node

**Files:** `internal/mesh/store/store.go`, `internal/mesh/store/store_test.go`, `internal/mesh/node/node.go`, `internal/mesh/node/node_test.go`

- [ ] **Step 1: Write store tests**

Test: RegisterNode, GetNode, ListNodes, RecordHeartbeat, GetRecentHeartbeats, CreateLinkingCode, ValidateLinkingCode.

- [ ] **Step 2: Implement store.go**

4 tables: nodes (id, name, host, port, role, platform, arch, cpu_cores, ram_total_mb, storage_total_gb, status, last_heartbeat, account_id), heartbeats (node_id, cpu_usage_percent, cpu_load_1m, ram_available_mb, ram_used_percent, storage_free_gb, timestamp), peers (peer_id, last_seen, host, port, is_hub), linking_codes (code, node_id, account_id, created_at, expires_at).

- [ ] **Step 3: Write node tests**

Test: capability scoring (RAM 0-40 + storage 0-20 - 50% battery), stable UUID persistence, system snapshot.

- [ ] **Step 4: Implement node.go**

NodeInfo struct, `CapabilityScore()` (0-60 pts), `LoadOrCreateID(path)` for stable UUID, `SystemSnapshot()` reading /proc/meminfo and df.

- [ ] **Step 5: Run tests → ALL PASS**

- [ ] **Step 6: Commit:** `feat: add mesh store and node identity with capability scoring`

---

## Task 7: Mesh — Election & Discovery

**Files:** `internal/mesh/election/election.go`, `internal/mesh/election/election_test.go`, `internal/mesh/discovery/discovery.go`

- [ ] **Step 1: Write election tests**

Test: highest capability wins, 20% hysteresis (incumbent keeps hub unless challenger exceeds by 20%), tiebreak by name then id.

- [ ] **Step 2: Implement election.go**

Pure function: `ElectHub(nodes []NodeInfo, currentHubID string) string` — returns winner ID.

- [ ] **Step 3: Implement discovery.go**

Two discovery methods:
- Tailscale: `tailscale status --json` → parse peers → HTTP probe `/api/mesh/identity`
- mDNS: announce `_soul-mesh._tcp.local.` on LAN

Both return `[]DiscoveredPeer{ID, Host, Port, IsHub}`.

- [ ] **Step 4: Run tests → ALL PASS**

- [ ] **Step 5: Commit:** `feat: add mesh hub election and peer discovery`

---

## Task 8: Mesh — Transport, Hub, Agent, Server

**Files:** `internal/mesh/transport/transport.go`, `internal/mesh/hub/hub.go`, `internal/mesh/agent/agent.go`, `internal/mesh/server/server.go`, `cmd/mesh/main.go`

- [ ] **Step 1: Implement transport.go**

WebSocket client/server with JWT auth. `CreateToken(nodeID, secret)`, `VerifyToken(tokenStr, secret)`. Message types: heartbeat, register, command_result. Exponential backoff: base 1s, max 300s, factor 2x. Max message: 1 MiB. Heartbeat interval: 30s.

Dependencies: `go get github.com/golang-jwt/jwt/v5 nhooyr.io/websocket`

- [ ] **Step 2: Implement hub.go**

Node registry, heartbeat processing (update last_heartbeat, insert into heartbeats table), resource aggregation (sum CPU/RAM/storage across cluster).

- [ ] **Step 3: Implement agent.go**

Heartbeat loop (10s interval): sends node identity + resource snapshot. Command execution: receives `run_command`, executes locally, returns stdout/stderr/exit_code.

- [ ] **Step 4: Create server.go**

Endpoints:
- GET `/api/mesh/identity` → node info
- GET `/api/mesh/nodes` → list cluster nodes
- GET `/api/mesh/status` → cluster status + aggregation
- POST `/api/mesh/link` → pairing via code
- GET `/api/mesh/heartbeats` → recent heartbeats
- WS `/ws/mesh` → hub↔agent transport
- POST `/api/tools/{name}/execute` → chat dispatch
- GET `/api/health`

Port 3024, env: SOUL_MESH_HOST/PORT/NAME/ROLE/SECRET/HUB.

- [ ] **Step 5: Create main.go, add dependencies, verify compilation**

Run: `go build ./cmd/mesh/`

- [ ] **Step 6: Commit:** `feat: add soul-mesh server with hub/agent WebSocket transport`

---

## Task 9: Scout — Store & Pipelines

**Files:** `internal/scout/store/store.go`, `internal/scout/store/leads.go`, `internal/scout/store/analytics.go`, `internal/scout/store/store_test.go`, `internal/scout/pipelines/pipelines.go`, `internal/scout/pipelines/pipelines_test.go`

- [ ] **Step 1: Write store tests**

Test: AddLead, GetLead, UpdateLead, ListLeads (with type/active filters), ScoredLeads, RecordStageHistory, analytics (3-layer), sync CRUD, optimization CRUD, agent run CRUD.

- [ ] **Step 2: Implement store.go**

7 tables: leads (id, title, company, type, source, source_url, pipeline, stage, compensation, currency, contact, location, tags, notes, metadata, variant, next_action, next_date, created_at, updated_at, closed_at, match_score, job_description), stage_history, sync_results, sync_meta, optimizations, agent_runs, platform_trust.

- [ ] **Step 3: Implement leads.go**

AddLead, GetLead, UpdateLead, ListLeads (filters), ScoredLeads (ORDER BY match_score DESC).

- [ ] **Step 4: Implement analytics.go**

3 layers: AggregateStats (by_type, by_source, by_stage, active/closed/stale), ConversionMetrics (funnels, win_rate, avg_days_to_close), ActionableInsights (stale_leads, follow_ups_due, pipeline_gaps).

- [ ] **Step 5: Implement pipelines.go**

5 pipeline definitions: job (discovered→applied→screening→interview→offer→joined), freelance, contract, consulting, product-dev. Each with valid stages and terminal states. `ValidateTransition(pipelineType, fromStage, toStage) error`.

- [ ] **Step 6: Run tests → ALL PASS**

- [ ] **Step 7: Commit:** `feat: add scout store, leads, analytics, and pipeline definitions`

---

## Task 10: Scout — Sweep, ProfileDB, Agent, Server

**Files:** `internal/scout/sweep/`, `internal/scout/profiledb/`, `internal/scout/agent/`, `internal/scout/server/server.go`, `cmd/scout/main.go`

- [ ] **Step 1: Implement sweep.go**

Platform crawler for LinkedIn, GitHub, Naukri, Wellfound. URL deduplication against existing leads. Auto-create leads on discovery.

- [ ] **Step 2: Implement cdp.go**

Chrome DevTools Protocol connection to `SOUL_SCOUT_CDP_URL` (default ws://127.0.0.1:9222). Uses go-rod for browser automation.

Dependency: `go get github.com/go-rod/rod`

- [ ] **Step 3: Implement profiledb/client.go**

PostgreSQL client to titan-pc portfolio_backup DB. Methods: `GetFullProfile()`, `GetSection(name)`, `PullAll()`, `PushAll()`. Tables: experience, projects, skill_categories, site_config, education, certifications.

Dependency: `go get github.com/jackc/pgx/v5`

Tests skip when `SOUL_SCOUT_PG_URL` is unset.

- [ ] **Step 4: Implement agent/launcher.go**

Claude subprocess spawner for profile optimization. Configures Playwright MCP, per-platform browser profile at `~/.soul-v2/scout/browser-profile/`.

- [ ] **Step 5: Create server.go**

23 REST endpoints mapping 1:1 to v1 tools:
- Lead CRUD: POST/GET/PATCH /api/leads, GET /api/leads/{id}, GET /api/leads/scored
- Analytics: GET /api/analytics
- Sync: POST /api/sync
- Sweep: POST /api/sweep, POST /api/sweep/now, GET /api/sweep/status, GET /api/sweep/digest
- Profile: GET /api/profile, POST /api/profile/pull, POST /api/profile/push
- Optimizations: POST/GET /api/optimizations, POST /api/optimize, POST /api/optimize/apply
- Actions: POST /api/leads/{id}/action
- Agent: GET /api/agent/status, GET /api/agent/history
- POST `/api/tools/{name}/execute`, GET `/api/health`

Port 3020, env: SOUL_SCOUT_HOST/PORT/PG_URL/CDP_URL/DATA_DIR.

- [ ] **Step 6: Create main.go and verify compilation**

- [ ] **Step 7: Commit:** `feat: add soul-scout server with full lead pipeline`

---

## Task 11: Chat Context — 4 Smart Agent Products

**Files:** Create 4 context files, modify context.go + dispatch.go + proxy.go

- [ ] **Step 1: Create sentinel.go** — 7 tools: challenge_list, challenge_start, challenge_submit, attack, sandbox_config, defend, scan

- [ ] **Step 2: Create bench.go** — 4 tools: run_benchmark, run_smoke, list_results, compare_results

- [ ] **Step 3: Create mesh.go** — 4 tools: cluster_status, list_nodes, node_info, link_node

- [ ] **Step 4: Create scout.go** — 23 tools (all lead/analytics/sweep/profile/optimization/agent tools)

- [ ] **Step 5: Update ForProduct(), dispatcher, and context tests**

- [ ] **Step 6: Add 4 proxies to proxy.go** (sentinel→3022, bench→3026, mesh→3024, scout→3020)

- [ ] **Step 7: Run tests → ALL PASS**

- [ ] **Step 8: Commit:** `feat: add chat context and proxies for 4 smart agent products`

---

## Task 12: Build & Deploy

**Files:** Makefile, 4 systemd service files

- [ ] **Step 1: Makefile** — build-sentinel, build-bench, build-mesh, build-scout. Update build/serve/clean.

- [ ] **Step 2: SystemD** — `deploy/soul-v2-{sentinel,bench,mesh,scout}.service`

- [ ] **Step 3: go.sum** — `go mod tidy` to resolve all new dependencies (pgx, rod, jwt, websocket)

- [ ] **Step 4: `make build`** → 13 binaries

- [ ] **Step 5: `make verify-static`** → PASS

- [ ] **Step 6: Commit:** `feat: add build targets and services for 4 smart agent products`

---

## Task 13: Full Verification

- [ ] **Step 1:** `go test -race -count=1 ./internal/... -v` → ALL PASS

- [ ] **Step 2:** `make build` → 13 binaries

- [ ] **Step 3:** Smoke test all 4 servers

- [ ] **Step 4:** Fix and commit any issues

---

## Summary

| Task | What | Files | Tests |
|------|------|-------|-------|
| 1 | Sentinel store | 2 | ~12 |
| 2 | Sentinel engine + challenges | 4 | ~6 |
| 3 | Sentinel server | 2 | 0 |
| 4 | Bench scoring | 2 | ~7 |
| 5 | Bench prompts + harness + server | 8+ | ~4 |
| 6 | Mesh store + node | 4 | ~8 |
| 7 | Mesh election + discovery | 3 | ~4 |
| 8 | Mesh transport + hub + agent + server | 5 | 0 |
| 9 | Scout store + pipelines | 6 | ~10 |
| 10 | Scout sweep + profile + server | 6 | ~4 |
| 11 | Chat context (38 tools) | 6 | 1 |
| 12 | Build & deploy | 5 | 0 |
| 13 | Verification | 0 | 0 |
| **Total** | | **~53 files** | **~56 tests** |
