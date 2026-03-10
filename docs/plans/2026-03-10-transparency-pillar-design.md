# Transparency Pillar — Design

## Goal

Add deep observability to every layer of soul-v2: database, HTTP, WebSocket, and frontend. Threshold-based alerting logged to the existing JSONL event system. Four new CLI report commands. Zero external dependencies.

## Architecture

Wrapper-based instrumentation. New structs decorate existing interfaces with timing and logging, keeping business logic untouched. An `AlertChecker` hooks into the event logger to check every event against configurable thresholds. Frontend sends errors and performance data to a new `/api/telemetry` endpoint.

## Phase 1: Database Instrumentation

**`TimedStore`** in `internal/session/timed_store.go` wraps `*Store`:

```go
type TimedStore struct {
    inner   *Store
    logger  *metrics.EventLogger
    slowMs  int64  // default 100
}
```

Every public `Store` method gets a corresponding `TimedStore` method that:
1. Records `time.Now()` before calling `inner.Method()`
2. Logs `db.query` event: `{method, duration_ms, session_id}`
3. If `duration_ms > slowMs`, also logs `db.slow`: `{method, duration_ms, session_id, threshold_ms}`

Methods wrapped: `CreateSession`, `GetSession`, `ListSessions`, `DeleteSession`, `AddMessage`, `AddMessageTx`, `GetMessages`, `UpdateSessionStatus`, `UpdateSessionTitle`, `RunInTransaction`.

Swap happens in `cmd/soul/main.go` — construct `TimedStore` around `Store`, pass to hub/handler.

## Phase 2: HTTP Request Instrumentation

**`requestLogger` middleware** in `internal/server/server.go`:

```go
func (s *Server) requestLogger(next http.Handler) http.Handler
```

Wraps `http.ResponseWriter` with `statusRecorder` to capture response status code. Logs:
- `api.request` on every request: `{method, path, status, duration_ms}`
- `api.slow` if `duration_ms > 500ms`

Excludes `/api/health` to avoid noise. Applied once at the top of `setupRoutes()`.

## Phase 3: Alert Threshold Checker

**`AlertChecker`** in `internal/metrics/alerts.go`:

```go
type AlertChecker struct {
    logger     *EventLogger
    thresholds map[string]Threshold
}

type Threshold struct {
    Metric    string
    Field     string
    MaxValue  float64
    Severity  string  // "warning" or "critical"
}
```

Default thresholds:

| Metric | Field | Max | Severity |
|--------|-------|-----|----------|
| `db.query` | `duration_ms` | 100 | warning |
| `db.query` | `duration_ms` | 500 | critical |
| `api.request` | `duration_ms` | 500 | warning |
| `api.request` | `duration_ms` | 2000 | critical |
| `ws.stream.end` | `duration_ms` | 300000 | critical |
| `system.sample` | `heap_mb` | 256 | warning |
| `system.sample` | `goroutines` | 100 | warning |

Hooks into `EventLogger.Log()` — after writing an event, runs threshold checks. On breach, logs `alert.threshold`: `{metric, field, value, threshold, severity}`.

## Phase 4: Frontend Error Capture

**4a. Error Boundary** — `ErrorBoundary.tsx` wrapping the app root. Renders fallback UI on crash, sends error to `POST /api/telemetry` as `frontend.error` event.

**4b. Replace silent catches** — All `catch {}` blocks in `useChat.ts` and `useWebSocket.ts` replaced with `reportError(context, err)` calls. `reportError` is a fire-and-forget `fetch` to `/api/telemetry`.

**4c. `/api/telemetry` endpoint** — Accepts POST with `{type, data}`. Validates event type is one of: `frontend.error`, `frontend.render`, `frontend.ws`. Rate-limited to 10 events/second per client IP. Writes to metrics logger.

## Phase 5: Frontend Performance & WebSocket Timing

**5a. `usePerformance` hook** — Reports Long Tasks (>50ms) for heavy components (`MessageBubble`, `SessionList`, `Markdown`). Logs `frontend.render`: `{component, duration_ms}`.

**5b. WebSocket round-trip timing** — In `useWebSocket.ts`:
- Record `performance.now()` on `chat.send`
- Calculate `first_token_ms` on first `chat.token`
- Calculate `total_ms` on `chat.done`
- Report as `frontend.ws`: `{event: "round_trip", first_token_ms, total_ms}`

One event per conversation turn.

## Phase 6: New Event Types & CLI Commands

**New event types** in `internal/metrics/types.go`:

| Event | Source | Data Fields |
|-------|--------|-------------|
| `db.query` | TimedStore | `method, duration_ms, session_id` |
| `db.slow` | TimedStore | `method, duration_ms, session_id, threshold_ms` |
| `api.request` | HTTP middleware | `method, path, status, duration_ms` |
| `api.slow` | HTTP middleware | `method, path, status, duration_ms` |
| `alert.threshold` | AlertChecker | `metric, field, value, threshold, severity` |
| `frontend.error` | Error boundary / catches | `component, error, stack` |
| `frontend.render` | usePerformance | `component, duration_ms` |
| `frontend.ws` | useWebSocket | `event, first_token_ms, total_ms` |

**New CLI commands:**

- `soul metrics alerts` — Recent threshold breaches, filterable by severity
- `soul metrics db` — DB query timing: P50/P95/P99 per method, slow query log
- `soul metrics requests` — HTTP request timing: P50/P95/P99 per path, status code distribution
- `soul metrics frontend` — Frontend errors + render performance + WS round-trip latency

Reuse existing `aggregator.go` pattern.

## Key Files

| File | Phase |
|------|-------|
| `internal/session/timed_store.go` | 1 (create) |
| `cmd/soul/main.go` | 1 (wire TimedStore) |
| `internal/server/server.go` | 2 (middleware) |
| `internal/metrics/alerts.go` | 3 (create) |
| `internal/metrics/logger.go` | 3 (hook AlertChecker) |
| `internal/metrics/types.go` | 6 (new events) |
| `internal/metrics/aggregator.go` | 6 (new reports) |
| `cmd/soul/metrics.go` | 6 (new CLI commands) |
| `internal/server/server.go` | 4c (telemetry endpoint) |
| `web/src/components/ErrorBoundary.tsx` | 4a (create) |
| `web/src/lib/telemetry.ts` | 4b (create) |
| `web/src/hooks/useChat.ts` | 4b (replace silent catches) |
| `web/src/hooks/useWebSocket.ts` | 4b, 5b (error logging, WS timing) |
| `web/src/hooks/usePerformance.ts` | 5a (create) |
| `web/src/components/MessageBubble.tsx` | 5a (add hook) |
| `web/src/components/SessionList.tsx` | 5a (add hook) |
| `web/src/components/Markdown.tsx` | 5a (add hook) |

## Verification

| Phase | Static | Unit Test | Integration |
|-------|--------|-----------|-------------|
| 1 | go vet | timed_store_test.go | slow query logging |
| 2 | go vet | middleware_test.go | request timing |
| 3 | go vet | alerts_test.go | threshold breach |
| 4 | tsc | — | telemetry endpoint |
| 5 | tsc | — | — |
| 6 | go vet | aggregator_test.go | CLI output |
