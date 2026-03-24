# Soul

AI development platform with autonomous task execution — Go backend, React frontend, multi-agent pipeline.

## What it is

Soul is a production multi-agent development platform. It runs a team of 9 specialized AI agents across two machines (Raspberry Pi + x86 server), coordinated through a courier messaging system with structured inboxes, role boundaries, and a shared skill library.

The platform includes 13 product servers (chat, tasks, tutor, projects, scout, sentinel, bench, and more), a React frontend, and a 7-layer verification stack that gates every merge.

## Tech Stack

| Layer | Technology |
|-------|-----------|
| Backend | Go 1.24 — 13 independent HTTP servers |
| Frontend | React 19, TypeScript, Tailwind v4, Vite |
| AI | Claude API (OAuth), streaming SSE, tool-use |
| Database | SQLite per product (no shared state) |
| Real-time | WebSocket hub with multi-session routing |
| Auth | Claude OAuth (`pkg/auth`) shared across servers |
| Testing | Go test + race detector, Vitest, Playwright |

## Architecture

```
soul/
  cmd/           13 server entrypoints (chat, tasks, tutor, projects, ...)
  internal/      Per-product server logic, store, and handlers
  pkg/           Shared: auth, events
  web/           React SPA — single frontend, 8 product proxies
  scout/         Lead pipeline CRM (TheirStack integration)
  bench/         LLM benchmarking harness (CARS metric, 52 models)
  sentinel/      CTF challenge platform with embedded challenges
  tools/         Build, verification, and phase test scripts
```

The 13 servers run on fixed ports (`:3002` – `:3026`) and are proxied through the chat server's SPA. Each server is independently deployable and owns its own SQLite database.

## Key Products

- **Chat** — Claude streaming interface with multi-session support, custom tools, and subagent dispatch
- **Tasks** — Autonomous task executor with 3-phase pipeline (impl → review → fix) and merge gates
- **Tutor** — Interview prep platform with SM-2 spaced repetition and mock interviews
- **Scout** — Lead research and outreach pipeline with AI-powered job board sweeps
- **Bench** — LLM benchmarking tool — 30 tasks, 10 categories, CARS efficiency scoring
- **Sentinel** — CTF security challenge platform with 14 embedded challenges
- **Projects** — Implementation guide browser with embedded markdown content
- **Observe** — Pillar-based metrics (Performant, Robust, Accurate, Readable, Scalable, Secure)

## Verification

```bash
make verify-static   # Go vet + tsc --noEmit + secret scan + dep audit
make verify          # L1–L3: static + unit + integration
make build           # Build all 13 binaries + frontend
make serve           # Build and run everything
```

Six design pillars enforced on every merge: Performant, Robust, Accurate, Readable, Scalable, Secure.

## Running

```bash
# Install dependencies
go mod download
cd web && npm install

# Start all servers
make serve

# Or individual server
go run cmd/chat/main.go
```

Requires Go 1.24+ and Node 18+.
