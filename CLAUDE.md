# Soul — AI-Powered Development Platform

Go + React/TypeScript monorepo. Self-hosted task planner with autonomous Claude agent execution, git worktree isolation, and dual-server dev/prod preview.

## IMPORTANT: Server Management (for autonomous agents)

Soul runs as a **systemd service** (`soul.service`). Autonomous agents (task execution) must **NEVER** run `go build`, `vite build`, `sudo systemctl restart soul`, or any server restart/rebuild commands. The autonomous pipeline handles builds and deploys separately.

## Quick Commands

```bash
# Build & run (prod on :3000, dev on :3001) — USER runs this, not Claude
go build -o soul ./cmd/soul && sudo systemctl restart soul

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
| `SOUL_<NAME>_BIN` | — | Override binary path for any product |

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

## Adding a New Product

Products are config-driven — zero code changes for basic functionality (tools + task board).

### 1. Implement the gRPC interface

Create a binary in any language that implements `ProductService` from `proto/soul/v1/product.proto`:

```protobuf
service ProductService {
  rpc GetManifest(Empty) returns (Manifest);
  rpc ExecuteTool(ToolRequest) returns (ToolResponse);
  rpc ExecuteToolStream(ToolRequest) returns (stream ToolResponse);
}
```

The binary must accept `--socket <path>` and start a gRPC server on that Unix socket. See `products/compliance-go/` or `products/scout/` for Go examples.

### 2. Register in `~/.soul/products.yaml`

```yaml
products:
  - name: my-product          # required — used as product identifier everywhere
    binary: /path/to/binary   # required — absolute path to the built binary
    label: My Product          # optional — display name (defaults to name)
    color: active              # optional — stage color token: active|brainstorm|validation|done|blocked|backlog
```

### 3. Restart Soul

```bash
go build -o soul ./cmd/soul && ./soul serve
```

The product appears in the rail, its tools are available to the agent, and it gets a task board automatically.

### 4. (Optional) Add a dedicated frontend panel

For products that need custom UI beyond the generic task dashboard:

1. Create a panel component in `web/src/components/panels/MyProductPanel.tsx`
2. Add one line to the `DEDICATED_PANELS` map in `web/src/components/layout/ProductView.tsx`:
   ```tsx
   const DEDICATED_PANELS: Record<string, React.ComponentType> = {
     // ...existing entries...
     'my-product': MyProductPanel,
   };
   ```
3. Rebuild frontend: `cd web && npx vite build`

### Key files

| File | Purpose |
|------|---------|
| `~/.soul/products.yaml` | Product config (name, binary, label, color) |
| `internal/config/products.go` | Config parser — `LoadProducts()` |
| `internal/products/manager.go` | Starts binaries, gRPC connection, manifest retrieval |
| `internal/products/registry.go` | Tool catalog — `AllTools()`, `FindTool()`, `Names()` |
| `internal/server/routes.go` | `GET /api/products` — returns product metadata to frontend |
| `web/src/components/layout/ProductView.tsx` | `DEDICATED_PANELS` registry for custom UI |

### Env var / CLI flag overrides

Even with `products.yaml`, you can override binary paths:
- Env var: `SOUL_MY_PRODUCT_BIN=/other/path` (name uppercased, hyphens → underscores)
- CLI flag: `--my-product-bin /other/path`

If no `products.yaml` exists, Soul falls back to legacy env vars (`SOUL_COMPLIANCE_BIN`, `SOUL_SCOUT_BIN`) for backwards compatibility.

## Known Gotchas

- Worktrees need `web/node_modules` symlinked for vite builds to work
- `MergeToDev` must merge inside the dev-server worktree (dev branch is checked out there)
- Dev server delegates all `/api/` and `/ws` to prod mux — it only overrides frontend files
- SPA serving is disk-based (not Go embed) so agent's `vite build` takes effect immediately
- Port 3001 may be occupied by other services — kill them before starting
