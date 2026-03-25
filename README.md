# soul

Multi-agent AI system powering 21 products from a single WebSocket interface.

## What it does

Soul is a production-grade AI platform built on 13 independent Go microservices and 62 packages, coordinated through a central WebSocket hub. A single React SPA with an AppShell panel layout routes Claude tool-use calls to any of 21 product servers — chat, autonomous task execution, interview prep, lead pipeline CRM, LLM benchmarking, CTF challenges, distributed compute, compliance scanning, and more. Every server owns its own SQLite database; no shared state, no single point of failure.

498 commits. 127 Claude tools. 7-layer verification stack gates every merge.

## Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                    React SPA (AppShell)                         │
│  Dashboard · Chat · Tasks · Tutor · Projects · Observe          │
│  Scout · Sentinel · Mesh · Bench · Infra · Quality · Data       │
└──────────────────────────┬──────────────────────────────────────┘
                           │ WebSocket
                           ▼
┌─────────────────────────────────────────────────────────────────┐
│                  Chat Server :3002                               │
│  ┌────────────┐  ┌──────────────┐  ┌────────────────────────┐  │
│  │  WS Hub    │  │ Stream (SSE) │  │  Product Context (21)  │  │
│  │ multi-sess │  │ Claude OAuth │  │  tool defs + dispatch  │  │
│  └─────┬──────┘  └──────────────┘  └───────────┬────────────┘  │
└────────┼─────────────────────────────────────────┼──────────────┘
         │ tool_use dispatch                        │ HTTP proxy
         ▼                                          ▼
┌────────────────────────────────────────────────────────────────┐
│             Product Servers (independent, SQLite)              │
│  Tasks:3004  Tutor:3006  Projects:3008  Observe:3010           │
│  Infra:3012  Quality:3014  Data:3016   Docs:3018               │
│  Scout:3020  Sentinel:3022 Mesh:3024   Bench:3026              │
└────────────────────────────────────────────────────────────────┘
```

Claude responds with `tool_use` blocks → WS hub dispatches via product context → product REST API → `tool_result` → follow-up response. Up to 5 tool rounds per message.

## Services

| Service | Port | Description |
|---------|------|-------------|
| chat | :3002 | Claude streaming interface — WebSocket hub, multi-session routing, SPA host, 8 product proxies |
| tasks | :3004 | Autonomous task executor — 3-phase pipeline (impl → review → fix), merge gates, comment watcher |
| tutor | :3006 | Interview prep platform — SM-2 spaced repetition, 5 modules, mock interview sessions |
| projects | :3008 | Implementation guide browser — 11 embedded markdown guides, milestone tracking |
| observe | :3010 | Pillar-based observability — 6 metrics (Performant, Robust, Accurate, Readable, Scalable, Secure) |
| infra | :3012 | Infrastructure tooling — DevOps analysis, DBA health checks, migration planning |
| quality | :3014 | Code quality — compliance scanner (SOC2/HIPAA/GDPR), QA analysis, usage analytics |
| data | :3016 | Data products — data engineering, cost operations, visualization generation |
| docs | :3018 | Documentation — technical docs generation, API reference generation |
| scout | :3020 | Lead pipeline CRM — TheirStack sweeps, 35 AI outreach tools, 7 pipeline types |
| sentinel | :3022 | CTF challenge platform — 14 embedded challenges, sandbox, weakness levels |
| mesh | :3024 | Distributed compute — Tailscale + mDNS discovery, hub election, node linking |
| bench | :3026 | LLM benchmarking — CARS metric, 33 prompt tasks, 10 categories, multi-model compare |

## Tech Stack

| Layer | Technology |
|-------|-----------|
| Backend | Go 1.24 — 13 servers, 62 packages, standard library preferred |
| Frontend | React 19, TypeScript 5.9, Tailwind CSS v4, Vite 7 |
| Real-time | WebSocket hub — multi-session, per-session product context |
| AI | Claude API via OAuth — streaming SSE, tool-use, up to 5 tool rounds |
| Database | SQLite per server — no shared state, atomic operations |
| Auth | Claude OAuth (`pkg/auth`) — shared credential, 0600 permissions |
| Testing | Go test + race detector, Vitest, Playwright — 7-layer verification stack |

## Running Locally

```bash
# Install dependencies
go mod download
cd web && npm install && cd ..

# Build all 13 binaries + frontend
make build

# Start everything
make serve

# Or start an individual server
go run cmd/chat/main.go      # Chat on :3002
go run cmd/tasks/main.go     # Tasks on :3004
go run cmd/scout/main.go     # Scout on :3020
```

**Requires:** Go 1.24+, Node 18+, Claude Max OAuth credentials at `~/.claude/.credentials.json`

```bash
# Verification
make verify-static   # go vet + tsc --noEmit + secret scan + dep audit (L1)
make verify          # L1–L3: static + unit + integration
```

## Products

21 products accessible from a single chat interface via the tool selector:

| # | Product | Description |
|---|---------|-------------|
| 1 | **Chat** | Claude streaming with multi-session support, custom tools, and subagent dispatch |
| 2 | **Tasks** | Autonomous executor — 3-phase pipeline (impl → review → fix), hooks, merge gates |
| 3 | **Tutor** | Interview prep — SM-2 spaced repetition, DSA/AI/Behavioral/Mock/Planner modules |
| 4 | **Projects** | Skill-building guides — 11 embedded implementation projects with milestone tracking |
| 5 | **Observe** | Pillar metrics — real-time observability across 6 quality dimensions |
| 6 | **Scout** | Lead pipeline CRM — TheirStack job discovery, 7 pipeline types, 35 AI tools |
| 7 | **Sentinel** | CTF challenge platform — 14 embedded security challenges with sandbox sessions |
| 8 | **Mesh** | Distributed compute — link multiple machines, elect hub, run LLM inference at scale |
| 9 | **Bench** | LLM benchmarking — run CARS benchmark across models, compare efficiency scores |
| 10 | **Compliance** | SOC2/HIPAA/GDPR scanner — 5 analyzers, auto-fix engine, badge reporter |
| 11 | **QA** | Code quality analysis — static analysis + structured reporting |
| 12 | **Analytics** | Usage analytics — event aggregation, cost tracking, performance reporting |
| 13 | **DevOps** | Infrastructure analysis — system health checks and actionable reports |
| 14 | **DBA** | Database health — schema analysis, query patterns, index recommendations |
| 15 | **Migrate** | Migration planning — schema diff analysis and migration sequencing |
| 16 | **DataEng** | Data engineering — pipeline scaffolding and data quality tooling |
| 17 | **CostOps** | Cost optimization — resource utilization analysis and reduction recommendations |
| 18 | **Viz** | Visualization — generate charts and dashboards from structured data |
| 19 | **Docs** | Documentation generation — technical docs from code and specs |
| 20 | **API** | API reference generation — OpenAPI docs from route definitions |
| 21 | **Built-in** | Memory management, custom tool definitions, and subagent dispatch (available in all contexts) |

---

*Production-grade agent intelligence. Self-hosted, sovereign, zero external dependencies at runtime.*
