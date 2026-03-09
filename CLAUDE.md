# Soul v2 — Spec-Driven Chat Interface

Go + React/TypeScript monorepo. AI-agent-maintained, spec-driven chat interface with Claude OAuth, multi-session support, and 7-layer verification stack.

## Quick Commands

```bash
make build          # Build Go binary + frontend
make serve          # Build and run on :3002
make verify         # Run L1-L3 verification (static + unit + integration)
make verify-static  # Go vet + tsc --noEmit + secret scan + dep audit
make types          # Generate types.ts from specs
make clean          # Remove build artifacts
```

## Architecture

```
cmd/soul/main.go              CLI entrypoint
internal/
  auth/                       Claude OAuth — load, refresh, persist tokens
  session/                    SQLite session CRUD
  stream/                     Claude API streaming — SSE parse
  ws/                         WebSocket hub — session-scoped routing
  server/                     HTTP server + SPA serving
  metrics/                    Event logging, aggregation, CLI reporting
web/src/
  components/                 React components (Shell, Chat, Sessions)
  hooks/                      Custom hooks (useWebSocket, useSessions)
  lib/                        types.ts (generated), ws.ts, api.ts
specs/                        YAML module specs (source of truth)
tests/                        Integration, E2E, load, verification
tools/                        specgen, classifier, reviewer, monitor
```

## Environment Variables

| Variable | Default | Purpose |
|----------|---------|---------|
| `SOUL_V2_PORT` | `3002` | Server port |
| `SOUL_V2_HOST` | `127.0.0.1` | Bind address |
| `SOUL_V2_DATA_DIR` | `~/.soul-v2` | Data directory |

Auth: `~/.claude/.credentials.json` (Claude Max OAuth, read-only)

## Conventions — Go

- Go 1.24+, standard library preferred
- All Claude API calls through `internal/stream/` — never direct HTTP
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
