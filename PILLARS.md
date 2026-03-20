# Soul v2 — Design Pillars

Six non-negotiable constraints. Every feature, every line of code, every commit is measured against these pillars. If a change violates any constraint, it does not ship.

## 1. PERFORMANT

Fast and resource-efficient.

| Constraint | Enforcement |
|------------|-------------|
| First token to screen < 200ms | E2E timing assertion |
| WebSocket overhead < 50 bytes | Integration test |
| Frontend bundle < 300KB gzipped | Build gate |
| Server memory < 100MB at 10 sessions | Monitoring alert |
| Zero unnecessary re-renders | E2E React profiler |
| Exact token budget | Opus review |

## 2. ROBUST

Handles every edge case without crashing.

| Constraint | Enforcement |
|------------|-------------|
| No panic on any input (fuzz-tested) | Property-based tests |
| Defined behavior for nil/empty/oversized | Unit tests per invariant |
| Atomic DB operations | Integration test |
| Graceful render with 0 sessions, 1000 messages | E2E boundary tests |
| Every error path produces user-visible message | Opus review |
| Type system prevents invalid states | Static analysis |

## 3. RESILIENT

Recovers automatically, degrades gracefully.

| Constraint | Enforcement |
|------------|-------------|
| API down → UI shows status, retains state, reconnects | E2E simulated outage |
| WS disconnect → auto-reconnect with backoff | Integration test |
| Token refresh fails → disk fallback → retry → alert | Unit test |
| Server restart → sessions restored | Integration test |
| Corrupted DB → detected, backup or clean error | Unit test |
| OOM → graceful shed, never crash | Load test |

## 4. SECURE

Hardened, minimal attack surface.

| Constraint | Enforcement |
|------------|-------------|
| Zero secrets in code/config/logs | SAST scan + CI gate |
| All input sanitized (XSS-proof) | E2E + property tests |
| WS origin validation | Integration test |
| Parameterized SQL | Static pattern scan |
| OAuth tokens 0600, never logged | File permission + log scan |
| Dependencies audited | npm audit + govulncheck |
| Rate limiting | Integration test |
| CSP headers | E2E header check |

## 5. SOVEREIGN

You own everything.

| Constraint | Enforcement |
|------------|-------------|
| Zero external CDNs/fonts/assets | E2E network audit |
| No SaaS dependencies | Spec review |
| SQLite local | Architecture constraint |
| Gitea hosting | Push workflow |
| No telemetry/analytics | E2E network monitor |
| All artifacts in repo | Spec review |
| Claude API abstracted, swappable | Opus review |
| Offline reading (service worker) | E2E offline test |

## 6. TRANSPARENT

Every action is observable, every state change is traceable.

| Constraint | Enforcement |
|------------|-------------|
| Every state transition logged as structured event | Unit tests per event emitter |
| Metrics queryable via CLI (status, cost, latency, errors) | Integration test |
| Frontend errors and slow renders reported to backend | E2E telemetry check |
| Alert thresholds fire on anomalies | Unit test |
| No silent failures — all errors surface to user or log | Opus review |
| API cost and token usage tracked per request | Integration test |
| DB query performance profiled per method | Integration test |
| Daily log rotation with retention | Unit test |
| Every feature has observability: render timing, error reporting, usage tracking | Opus review |
| Page views and feature actions tracked as `frontend.usage` events | CLI `metrics usage` report |

### Implementation Details

**Frontend Telemetry Pipeline:**
- `reportUsage(action, data)` → `POST /api/telemetry` → JSONL metrics file → CLI aggregation
- `reportError(source, error)` → same pipeline with `frontend.error` event type
- `usePerformance(component)` → reports render timing >50ms as `frontend.perf` events

**Page View Tracking** (`frontend.usage` → `page.view`):
- ChatPage, DashboardPage, TasksPage, TaskDetailPage — all report on mount

**Feature Action Tracking** (`frontend.usage`):
- Task operations: `task.create`, `task.update`, `task.delete`, `task.start`, `task.stop`
- Session management: `session.create`, `session.switch`, `session.delete`, `session.rename`
- Chat: `chat.send` (with model, thinking, hasAttachments metadata)

**Render Performance Monitoring** (`frontend.perf`):
- Components: ChatPage, DashboardPage, TasksPage, TaskDetailPage, TaskCard, ActivityTimeline

**Error Reporting** (`frontend.error`):
- API layer: all HTTP and network errors via `api.ts`
- Task hooks: `useTasks.refresh`, task event parse errors
- Task detail: load, start, stop failures

**CLI Reporting:**
- `soul-chat metrics usage` — page views by page, feature actions by type, total event count

---

## Adding a New Product

Every product in soul-v2 follows the same architecture: a standalone Go server with SQLite storage, REST API, Claude tool definitions, and a frontend page — all wired through the chat server's product routing system.

### Current Products (14 servers)

```
cmd/chat     :3002   cmd/tasks    :3004   cmd/tutor    :3006   cmd/projects :3008
cmd/observe  :3010   cmd/infra    :3012   cmd/quality  :3014   cmd/data     :3016
cmd/docs     :3018   cmd/scout    :3020   cmd/sentinel :3022   cmd/mesh     :3024
cmd/bench    :3026   cmd/mcp      :3028
```

Next available port: **3030**

### Step-by-Step Checklist

#### 1. Backend — Server Binary

Create the server entrypoint and internal package:

```
cmd/{product}/main.go                    Server CLI entrypoint
internal/{product}/
  server/server.go                       HTTP server + REST API
  store/store.go                         SQLite CRUD (optional)
```

**Pattern to follow:** Copy from `internal/bench/server/server.go` for a simple product, or `internal/scout/server/server.go` for a complex one.

Server must:
- Read `SOUL_{PRODUCT}_HOST` and `SOUL_{PRODUCT}_PORT` env vars
- Serve REST API at `/api/tools/{tool_name}/execute`
- Return JSON responses

#### 2. Backend — Tool Definitions + Context

Create the product context file:

```
internal/chat/context/{product}.go
```

**Pattern to follow:** Copy from `internal/chat/context/bench.go`

```go
package context

import "github.com/rishav1305/soul-v2/internal/chat/stream"

func {product}Context() ProductContext {
    return ProductContext{
        System: `You are connected to {Product Name}. {Description}.

Key capabilities:
- {Capability 1}
- {Capability 2}`,
        Tools: []stream.Tool{
            {
                Name:        "{tool_name}",
                Description: "{what it does}",
                InputSchema: mustJSON(`{"type":"object","properties":{...},"required":[...]}`),
            },
        },
    }
}
```

#### 3. Backend — Register in Context Provider

Edit `internal/chat/context/context.go` — add the product to the provider map:

```go
"{product}": {product}Context(),
```

#### 4. Backend — Register in Dispatcher

Edit `internal/chat/context/dispatch.go` — add dispatch cases for each tool:

```go
case "{tool_name}":
    // Route to product server REST API
```

#### 5. Backend — Chat Server Proxy

Edit `internal/chat/server/server.go`:

1. Add proxy field: `{product}Proxy *simpleProxy`
2. Add option function: `WithProductProxy() Option`
3. Register API route: `/api/{product}/` → proxy to product server

#### 6. Backend — Wire in Main

Edit `cmd/chat/main.go` — add the proxy option when starting the chat server.

#### 7. Environment Variables

Add to `CLAUDE.md` environment table and to your shell:

```
SOUL_{PRODUCT}_HOST   127.0.0.1     {Product} server bind address
SOUL_{PRODUCT}_PORT   {next port}   {Product} server port
SOUL_{PRODUCT}_URL    http://127.0.0.1:{port}   {Product} server URL (for chat proxy)
```

#### 8. Frontend — Page + Route

```
web/src/pages/{Product}Page.tsx          Product page component
web/src/hooks/use{Product}.ts            API hook (optional)
web/src/components/{product}/            Product-specific components (optional)
```

Add route in `web/src/router.tsx`:

```tsx
{
    path: '{product}',
    lazy: () => import('./pages/{Product}Page'),
},
```

Add nav item in `web/src/layouts/AppLayout.tsx`.

#### 9. Frontend — Types

If the product has new types, add to `specs/{product}.yaml` and run `make types` to generate `web/src/lib/types.ts`.

#### 10. Build System

Add to `Makefile`:
- Build target for the new server binary
- Include in `make build` and `make serve`

#### 11. Update CLAUDE.md

Add to the Architecture section:
- New `cmd/` and `internal/` entries
- New env vars
- New route in the routes list
- Tool count update

#### 12. Tests

Required tests before merge:

| Test | File | What |
|------|------|------|
| Unit tests | `internal/{product}/server/*_test.go` | Every public function |
| Unit tests | `internal/{product}/store/*_test.go` | CRUD operations |
| Context test | `internal/chat/context/context_test.go` | Product registered, tools defined |
| Integration | `tests/integration/{product}_test.go` | REST API contracts |

#### 13. Role Integration

After the product ships, integrate with the role system (see `~/soul-roles/GUIDE.md`):

1. **Decide:** New role or existing role operates this product?
2. **Update CLAUDE.md** of the operating role — add knowledge sources, KPIs
3. **Update Dev PM** — add to codebase knowledge
4. **Notify** the operating role via `~/soul-roles/shared/inbox/{role}/`
5. **Update conference skill** — add to persona domain descriptions

### Product Pillar Compliance

Every new product must satisfy all 6 pillars before merge:

| Pillar | Product Requirement |
|--------|-------------------|
| **Performant** | Server memory < 100MB, API response < 200ms |
| **Robust** | No panic on any input, defined behavior for nil/empty |
| **Resilient** | Graceful error responses, no silent failures |
| **Secure** | Parameterized SQL, no hardcoded secrets, input validation |
| **Sovereign** | SQLite local storage, no external SaaS dependencies |
| **Transparent** | Structured event logging, metrics queryable via CLI |

### Quick Reference: File Checklist

```
New files:
  cmd/{product}/main.go
  internal/{product}/server/server.go
  internal/{product}/server/server_test.go
  internal/{product}/store/store.go          (if stateful)
  internal/{product}/store/store_test.go     (if stateful)
  internal/chat/context/{product}.go
  web/src/pages/{Product}Page.tsx
  web/src/hooks/use{Product}.ts              (optional)
  tests/integration/{product}_test.go

Modified files:
  internal/chat/context/context.go           (register product)
  internal/chat/context/dispatch.go          (add tool routes)
  internal/chat/server/server.go             (add proxy)
  cmd/chat/main.go                           (wire proxy)
  web/src/router.tsx                         (add route)
  web/src/layouts/AppLayout.tsx              (add nav item)
  Makefile                                   (add build target)
  CLAUDE.md                                  (update architecture + env vars + tool counts)
```

---

## Pillar Verification Matrix

| Layer | What it checks | Pillars covered | When it runs |
|-------|---------------|-----------------|--------------|
| L1: Static Analysis | `go vet`, `tsc --noEmit`, secret scan, dep audit | Robust, Secure | Every commit |
| L2: Unit Tests | Public functions, edge cases, error paths | Robust, Resilient, Transparent | Every commit |
| L3: Integration Tests | API contracts, WS protocol, DB operations | Performant, Robust, Resilient, Secure, Transparent | Every commit |
| L4: E2E Tests | Full user flows, timing, rendering | Performant, Robust, Sovereign | Pre-merge |
| L5: Load Tests | Concurrency, memory, degradation | Performant, Resilient | Weekly |
| L6: Opus Review | Diff review, architecture alignment, pillar compliance | All six pillars | Pre-merge |
| L7: Visual Regression | Screenshot comparison, layout shifts | Performant, Robust | Pre-merge (UI changes) |
