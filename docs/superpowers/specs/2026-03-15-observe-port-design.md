# Observe Product — Design Spec

## Goal

Migrate Soul's CLI-based observability metrics to the Soul v2 web UI as a new "Observe" product. Surfaces all 12 existing CLI reports through a pillar-based dashboard that maps every metric to Soul's 6 design pillars (Performant, Robust, Resilient, Secure, Sovereign, Transparent). Adds per-product filtering so metrics can be scoped to Chat, Tasks, Tutor, or Projects independently.

## Architecture

Observe follows the same pattern as Tasks, Tutor, and Projects: a standalone Go server on its own port, proxied through the chat server, with a React frontend page.

**No new database.** Observe reads existing JSONL metrics files directly. The aggregation logic in `internal/chat/metrics/` is reused — the Observe server imports it and exposes the same 12 CLI reports as HTTP JSON endpoints.

**Product tagging** enables per-product metric filtering. Each product server gets its own `EventLogger` writing to a tagged JSONL file (`metrics-chat.jsonl`, `metrics-tasks.jsonl`, etc.). The Observe server reads all files and can filter by product.

**Polling refresh** — manual Refresh button, consistent with Tutor/Projects. No real-time streaming.

## File Structure

```
cmd/observe/main.go                    Observe server CLI (:3010)
internal/observe/
  server/server.go                     HTTP server — 13 JSON API endpoints
internal/chat/metrics/
  logger.go                            MODIFY — add product field to EventLogger
  reader.go                            MODIFY — add ReadAllProducts() for multi-file reading
  aggregator.go                        MODIFY — add product filter param to aggregation functions
cmd/chat/main.go                       MODIFY — tag logger with product:"chat", rename file
cmd/tasks/main.go                      MODIFY — add EventLogger, TimedStore, request middleware
cmd/tutor/main.go                      MODIFY — add EventLogger, TimedStore, request middleware
cmd/projects/main.go                   MODIFY — add EventLogger, TimedStore, request middleware
internal/chat/server/server.go         MODIFY — add WithObserveProxy()
web/src/
  pages/ObservePage.tsx                Pillar-based dashboard — 8 tabs
  hooks/useObserve.ts                  Data fetching hook with product filter
  lib/types.ts                         MODIFY (via specgen) — add Observe types
  layouts/AppLayout.tsx                MODIFY — add Observe to nav
  router.tsx                           MODIFY — add /observe route
Makefile                               MODIFY — add soul-observe binary
```

## Backend API Endpoints

All endpoints on the Observe server (`:3010`), proxied through chat at `/api/observe/*`.

All endpoints accept `?product=chat|tasks|tutor|projects` query param. Default: all products combined.

| Endpoint | CLI Equivalent | Returns |
|----------|---------------|---------|
| `GET /api/health` | — | `{"status":"ok"}` |
| `GET /api/overview` | `status` + `cost` + `alerts` | Uptime, sessions, events, cost breakdown, active alerts |
| `GET /api/latency` | `latency` | First-token P50/P95/P99, stream duration P50/P95/P99 |
| `GET /api/alerts` | `alerts` | All threshold breaches with metric, value, severity, timestamp |
| `GET /api/db` | `db` | Per-method query stats (count, P50, P95, P99), slow query log |
| `GET /api/requests` | `requests` | Per-path HTTP latencies, status code distribution |
| `GET /api/frontend` | `frontend` | Errors by component, slow render entries |
| `GET /api/usage` | `usage` | Page view counts, feature action counts |
| `GET /api/quality` | `quality` | Error taxonomy counts, quality ratings |
| `GET /api/layers` | `layers` | Gate pass/fail/retry rates with percentages |
| `GET /api/system` | `status` (subset) | Memory, goroutines, GC stats from system.sample events |
| `GET /api/tail` | `tail` | Last N events. Params: `?type=prefix&limit=50` |
| `GET /api/pillars` | — (new) | Computed pillar health: pass/warn/fail per pillar |

### Pillars Endpoint

`GET /api/pillars` maps each pillar's constraints to relevant metrics and computes health:

- **Performant**: first-token latency vs 200ms target, server memory vs 100MB, DB P50, HTTP P50
- **Robust**: error taxonomy counts, frontend error count
- **Resilient**: WS disconnect rate, oauth refresh failures
- **Secure**: rate limit violations, auth failures (runtime). Static constraints marked "enforced at build"
- **Sovereign**: all static — marked "enforced at build"
- **Transparent**: event count > 0, usage tracking active, gate coverage

Each constraint returns: `{name, target, enforcement, status: "pass"|"warn"|"fail"|"static", value?, measured_at?}`

## Frontend — Tab Structure

**8 tabs**: Overview, Performant, Robust, Resilient, Secure, Sovereign, Transparent, Tail

**Always visible:**
- Pillar health strip — 6 cards at top showing pass/total per pillar, color-coded (green/amber/red)
- Product filter dropdown (All | Chat | Tasks | Tutor | Projects) — top-right next to Refresh
- Refresh button

### Tab Content

| Tab | Data Sources | Content |
|-----|-------------|---------|
| **Overview** | `/api/overview` | Uptime, session count, event count, cost card ($X today, token breakdown), active alerts list |
| **Performant** | `/api/latency` + `/api/db` + `/api/requests` + `/api/system` | Constraint rows: first-token latency, bundle size (static badge), server memory, WS overhead, DB percentiles, HTTP percentiles |
| **Robust** | `/api/quality` + `/api/frontend` | Error taxonomy breakdown, frontend errors by component, slow renders |
| **Resilient** | `/api/overview` (subset) + `/api/tail?type=ws.` | WS connect/disconnect counts, oauth refresh events, stream recovery stats. Static constraints as "enforced at build" badges |
| **Secure** | `/api/requests` (subset) + `/api/tail?type=oauth.` | Rate limit hits, auth failures. Most constraints build-time — static badges |
| **Sovereign** | — | All constraints build-time enforced. Static badge list: "No external CDNs", "SQLite local", "Gitea hosted", etc. |
| **Transparent** | `/api/usage` + `/api/layers` | Event counts, page view stats, feature action counts, gate pass rates |
| **Tail** | `/api/tail` | Scrollable event list, type filter dropdown, limit selector (50/100/200) |

### Constraint Row Design

Each pillar tab shows constraints as rows:
- Left: constraint name + target (from PILLARS.md)
- Right: current measured value, color-coded (green = pass, amber = warning, red = fail)
- OR: "Enforced at build" badge for static constraints
- Color border-left matching status

## Product Tagging — Changes to Existing Servers

### EventLogger Changes (`internal/chat/metrics/`)

- `EventLogger` gets `product string` field, auto-injected into every event's `data` map
- Constructor: `NewEventLogger(dataDir, product string)` — writes to `metrics-{product}.jsonl`
- `Reader` gets `ReadAllProducts(dataDir string)` — globs `metrics-*.jsonl`, merges and sorts by timestamp
- Aggregation functions accept optional `product` filter param

### Per-Server Wiring

| Server | File | Changes |
|--------|------|---------|
| **Chat** | `cmd/chat/main.go` | Tag existing logger `product:"chat"`. File becomes `metrics-chat.jsonl`. Migration: rename existing `metrics.jsonl` → `metrics-chat.jsonl` on startup if old file exists. |
| **Tasks** | `cmd/tasks/main.go` | Create `EventLogger` with `product:"tasks"`, file `metrics-tasks.jsonl`. Add `WithMetrics()` to server. Add `requestLoggerMiddleware`. Wrap store with `TimedStore`. |
| **Tutor** | `cmd/tutor/main.go` | Same pattern — `product:"tutor"`, `metrics-tutor.jsonl`. Request middleware + TimedStore. |
| **Projects** | `cmd/projects/main.go` | Same pattern — `product:"projects"`, `metrics-projects.jsonl`. Request middleware + TimedStore. |

### What stays unchanged
- Alert thresholds — remain on chat server only (it has system samples)
- Sampler — stays on chat server
- Frontend telemetry endpoint — stays on chat server, tagged `product:"chat"`

## Navigation & Routing

- **Nav item**: "Observe" with icon `👁`, added after Projects
- **Route**: `/observe` in `router.tsx`, lazy-loaded
- **No detail page** — single page with tabs, no `/observe/:id`
- **Mobile**: 6th item in bottom nav bar
- **Desktop**: 6th link in header nav

## Infrastructure

- **Binary**: `soul-observe` built from `cmd/observe/main.go`
- **Makefile**: add to `build`, `serve`, `clean` targets
- **Env vars**: `SOUL_OBSERVE_HOST` (default `127.0.0.1`), `SOUL_OBSERVE_PORT` (default `3010`), `SOUL_OBSERVE_URL` (default `http://127.0.0.1:3010`)
- **Chat proxy**: `WithObserveProxy(url)` option, proxies `/api/observe/*` → Observe server
- **Systemd**: add to existing `soul-v2.service` or create separate `soul-observe.service`
- **Data**: reads from `~/.soul-v2/metrics-*.jsonl` — no new database

## Design Tokens

Uses existing Obsidian Command theme tokens:
- `bg-deep`, `bg-surface`, `bg-elevated`, `bg-overlay`
- `text-fg`, `text-fg-secondary`, `text-fg-muted`
- `border-border-subtle`
- Status colors: `emerald-400/500` (pass), `amber-400/500` (warning), `red-400/500` (fail)
- Soul accent: `text-soul` for active nav

## Out of Scope

- Real-time streaming (polling only)
- Historical trending / time-series charts
- Custom alert threshold configuration from UI
- Metric retention / cleanup policies
- Cross-product correlation views
