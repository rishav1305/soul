# Soul — AI-Powered Development Platform

Go + React/TypeScript monorepo. Self-hosted task planner with autonomous Claude agent execution, git worktree isolation, and dual-server dev/prod preview.

## Quick Commands

```bash
# Build & run (prod on :3000, dev on :3001)
go build -o soul ./cmd/soul && SOUL_HOST=0.0.0.0 ANTHROPIC_API_KEY=... ./soul serve

# Frontend only (dev mode with HMR)
cd web && npm run dev

# Build frontend (embedded into Go binary)
cd web && npx vite build

# Go build check
go build ./...

# Run with dev flag
./soul serve --dev --port 3000
```

## Architecture

```
cmd/soul/main.go          CLI entrypoint, flag parsing, server startup
internal/
  server/
    server.go             HTTP server, WebSocket hub, dev server (:port+1)
    routes.go             All API route registration
    tasks.go              Task CRUD + stage transitions, merge-to-master gate
    autonomous.go         TaskProcessor — background agent execution
    agent.go              AgentLoop — Claude API agentic loop with tool use
    codetools.go          6 code tools (read/write/edit/search/grep/exec)
    worktree.go           WorktreeManager — per-task git worktree isolation
    spa.go                SPA file server (disk-based, not embedded)
    handlers.go           Chat, session, product handlers
    ws.go                 WebSocket message types
  ai/
    client.go             Claude API client (API key + OAuth)
    oauth.go              Claude Max/Pro OAuth token source
  config/config.go        Config struct, env var parsing
  planner/store.go        SQLite task store (CRUD, stages, metadata)
  session/store.go        In-memory chat session store
  products/               Product plugin system (MCP-style stdio)
web/src/
  components/
    layout/               SoulRail, TopBar, MainContent
    chat/                 ChatPanel, MessageBubble, ToolCallBlock
    planner/              PlannerPanel, TaskDetail, TaskCard, KanbanBoard
    panels/               ProductPanel, SettingsPanel
  hooks/                  useChat, usePlanner, useWebSocket, useProducts
  lib/                    types.ts, ws.ts, utils
products/                 Product plugins (compliance-go, etc.)
```

## Environment Variables

| Variable | Default | Purpose |
|----------|---------|---------|
| `SOUL_PORT` | `3000` | Server port (dev server runs on port+1) |
| `SOUL_HOST` | `127.0.0.1` | Bind address (use `0.0.0.0` for LAN) |
| `SOUL_DEV` | — | Enable dev mode (`1` or `true`) |
| `SOUL_MODEL` | `claude-sonnet-4-6` | Claude model for agent |
| `SOUL_DATA_DIR` | `~/.soul` | Data directory (planner.db lives here) |
| `ANTHROPIC_API_KEY` | — | Claude API key |
| `SOUL_COMPLIANCE_BIN` | — | Path to compliance product binary |

Authentication priority: `ANTHROPIC_API_KEY` > `~/.claude/.credentials.json` (Claude Max OAuth)

## Autonomous Execution Pipeline

```
Task created (backlog)
  → User clicks "Start" → stage=active
  → TaskProcessor.StartTask(id)
    → Create worktree: .worktrees/task-<id>/ (branch: task/<id>-<slug>)
    → Symlink web/node_modules into worktree
    → Agent runs in worktree with code tools
    → git add -A && git commit in worktree
    → Merge task branch → dev (inside dev-server worktree)
    → stage=validation
  → User reviews on dev server (:3001)
  → User moves to "Done"
    → Merge task branch → master
    → vite build in main repo (updates prod)
    → Cleanup worktree + branch
```

## Branch Strategy

- `master` — production, served on `:3000`
- `dev` — integration, served on `:3001` (dev server)
- `task/<id>-<slug>` — per-task branches, auto-created/cleaned by WorktreeManager

## Git Remotes

- `origin` — `ssh://git@git.titan.local:222/admin/soul.git` (Gitea, private)
- `github` — `git@github.com:rishav1305/soul.git` (GitHub, public)

Push to both: `git push origin master && git push github master`

## Conventions

### Go
- Go 1.24+, standard library preferred
- All AI calls through `internal/ai/` client (never direct SDK imports)
- Parameterized SQL queries only (`?` placeholders) — never string concat
- No hardcoded secrets — env vars or Vaultwarden
- Error returns, not panics

### Frontend
- React 19, Vite, TypeScript, Tailwind CSS v4
- Dark theme (zinc/gray palette)
- WebSocket for real-time updates (task activity, chat tokens)
- `react-markdown` + `remark-gfm` for markdown rendering

### Security
- Never hardcode API keys or secrets
- Never concat SQL — use parameterized queries
- All tool execution in agent is sandboxed to worktree paths
- Agent cannot run git commands — system handles commits/merges

## Dev Server

The dev server (`:3001`) serves the frontend built from the `dev` branch worktree at `.worktrees/dev-server/`. API and WebSocket requests are delegated to the prod server's mux — only the SPA files differ between prod and dev.

## Workflow Modes

Tasks support two workflow modes (set in task metadata):
- **quick** (default): 5 steps — search, implement, build, summary, update
- **full**: 7 steps — plan, write tests, implement, build, security review, summary, update

## Data

- SQLite database: `~/.soul/planner.db` (tasks, stages, metadata)
- Session memory: in-process (not persisted across restarts)
- Worktrees: `.worktrees/` (gitignored)

## Known Gotchas

- Worktrees need `web/node_modules` symlinked for vite builds to work
- `MergeToDev` must merge inside the dev-server worktree (dev branch is checked out there)
- Dev server delegates all `/api/` and `/ws` to prod mux — it only overrides frontend files
- SPA serving is disk-based (not Go embed) so agent's `vite build` takes effect immediately
- Port 3001 may be occupied by other services — kill them before starting
