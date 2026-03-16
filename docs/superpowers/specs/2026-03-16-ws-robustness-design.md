# WebSocket Robustness & Observability Design

**Date:** 2026-03-16
**Status:** Draft
**Scope:** Connection resilience, reconnect state recovery, observability instrumentation, token coalescing, tool progress wire format

## Problem Statement

The WebSocket connection between the frontend and chat server is fragile. Connections break without clear cause, reconnects lose session state, auth failures trigger infinite retry loops, and disconnect metrics are too coarse to diagnose root causes.

### Root Causes Identified

1. **Ticket fetch blocks connection** — `fetchWSTicket()` in `ws.ts:14` has no timeout/abort. A stalled HTTP call leaves the connection in `connecting` indefinitely.
2. **No close/error diagnosis** — `useWebSocket.ts:95` ignores `CloseEvent.code/reason/wasClean`. Auth failure, origin rejection, server shutdown, and slow-client drops all look identical.
3. **Slow-client disconnect** — `client.go:107` closes the connection when the 256-slot send channel fills. Token streaming to slow/backgrounded tabs hits this.
4. **Coarse disconnect metrics** — `client.go:162` always logs `ws.disconnect` reason as `"read_pump_exit"`. `server.go:1060` excludes `/ws` from request logging, making failed upgrades invisible.
5. **Reconnect state desync** — `useChat.ts:174` only sends `session.switch` on first load (`!sessionIDRef.current`). On reconnect, the session ID is already set, so no re-subscription or history refresh occurs.
6. **Session stuck running on disconnect** — When a client disconnects, `OnClientDisconnect` cancels the agent context via `entry.cancel()`. If `Stream()` at `handler.go:508` is still in progress, it returns `context.Canceled`. The error path returns without calling `completeSession`, leaving the session stuck in `running` state.

## Approach: Layered Hardening

Four independently shippable layers, each building on the previous:

1. **Observability** — instrument before fixing, so improvements are measurable
2. **Reconnect Recovery** — fix the worst UX issue (stale state after reconnect)
3. **Connection Resilience** — prevent unnecessary disconnects and infinite retries
4. **Token Coalescing** — reduce message volume 10x, eliminate slow-client drops

## Layer 1: Observability Instrumentation

### 1.1 Backend Disconnect Reason Taxonomy

**File:** `internal/chat/ws/client.go`

Add a `closeReason` field to `Client` struct, set at the actual point of failure. The `ws.disconnect` metric emits the classified reason instead of the blanket `"read_pump_exit"`.

| Reason | Set where | Trigger |
|--------|-----------|---------|
| `client_normal_close` | `ReadPump` | `CloseStatus == StatusNormalClosure` |
| `ping_timeout` | `WritePump` | `Ping()` returns error |
| `write_error` | `WritePump` | `Write()` returns error |
| `slow_client_queue_full` | `Send()` | Send channel full, connection closed |
| `context_cancelled` | `ReadPump`/`WritePump` | `ctx.Done()` fires |
| `server_shutdown` | `Hub.Run` shutdown path | Hub context cancelled, clients closed |

The `ws.disconnect` metric payload becomes:

```json
{
  "client_id": "ws-00000001-a1b2c3d4",
  "reason": "ping_timeout",
  "close_code": -1,
  "duration_seconds": 342.5
}
```

Note: `close_code` uses the value from `websocket.CloseStatus(err)`. A value of `-1` means no close frame was received (abnormal close, ping timeout, write error). Only `client_normal_close` will have close code 1000.

The `server_shutdown` reason requires a small change in `Hub.Run`'s shutdown path: before calling `client.closeSend()` and `client.Close()`, set `client.closeReason = "server_shutdown"` so the disconnect metric emitted by the ReadPump/WritePump exit captures the correct reason.

### 1.2 WS Upgrade Metric

**Files:** `internal/chat/ws/hub.go`, `internal/chat/server/server.go`

Emit `ws.upgrade` metric at two points to capture the full upgrade lifecycle:

**Point 1 — Auth middleware** (`server.go:878`): The auth middleware rejects `/ws` requests with HTTP 401 *before* `HandleUpgrade` is called. Add a `ws.upgrade` metric emission in the auth middleware's `/ws` rejection path with `outcome: "auth_rejected"`. This is the most important failure to capture.

**Point 2 — HandleUpgrade** (`hub.go:167`): Emit `ws.upgrade` for origin rejection and upgrade success/failure.

| Field | Values |
|-------|--------|
| `outcome` | `auth_rejected`, `origin_rejected`, `upgrade_failed`, `success` |
| `origin` | Request origin header |
| `client_id` | Assigned client ID (on success, empty otherwise) |

This captures the full spectrum of upgrade failures. Auth rejections (the most common failure) are instrumented at the middleware level where they actually occur, not just in `HandleUpgrade` where they never arrive.

### 1.3 Frontend WS Lifecycle Telemetry

**File:** `web/src/hooks/useWebSocket.ts`, `web/src/lib/telemetry.ts`

New telemetry functions emitting `frontend.ws` events via the existing `POST /api/telemetry` endpoint:

| Event | Data | Trigger |
|-------|------|---------|
| `connect_attempt` | `{attempt, ticketUsed}` | Before opening WebSocket |
| `ticket_fetch` | `{status, duration_ms, fallback}` | After ticket fetch completes/fails |
| `open` | `{attempt, duration_ms}` | WebSocket `onopen` fires |
| `close` | `{code, reason, wasClean, duration_ms}` | WebSocket `onclose` fires |
| `error` | `{attempt}` | WebSocket `onerror` fires |

The telemetry allowlist in `server.go:692` already accepts `frontend.ws`. These new lifecycle events use the same `frontend.ws` type as the existing `reportWSLatency` helper (which sends `event: 'round_trip'`). The `event` sub-field within the `data` payload disambiguates them. `reportWSLifecycle` is a new helper alongside (not replacing) `reportWSLatency`.

### 1.4 Telemetry Allowlist Expansion

**File:** `internal/chat/server/server.go`

No changes needed to the allowlist — `frontend.ws` is already accepted. The `handleTelemetry` handler logs whatever `data` map it receives. Both `reportWSLatency` (`event: 'round_trip'`) and the new `reportWSLifecycle` (`event: 'close'`, `'open'`, etc.) use the `event` sub-field for disambiguation within the same `frontend.ws` bucket.

### 1.5 Observe Integration

**Files:** `internal/chat/metrics/aggregator.go`, `internal/observe/server/handlers.go`

The observe server's `buildResilientPillar` currently returns static constraints only (hardcoded strings at handlers.go:340-348). There is no dynamic `ConnectionHealthReport` type or live metric consumption. Making the resilient pillar consume the new WS metrics requires:

1. **New aggregator report type** — Add a `ConnectionHealthReport` struct to `aggregator.go` that scans the JSONL log for `ws.disconnect` and `ws.upgrade` events and computes:
   - Disconnect reason counts (grouped by `reason` field)
   - Upgrade outcome counts (grouped by `outcome` field)
   - Mean connection duration from `ws.disconnect` `duration_seconds`

2. **Resilient pillar upgrade** — Change `buildResilientPillar()` from static constraints to dynamic constraints that consume `ConnectionHealthReport`. Flag abnormal disconnect ratios and non-zero auth rejections.

This is new work beyond the minimal observability layer. **Defer to a separate observe spec** if the 4-layer plan needs to ship incrementally. The JSONL events from Layer 1 are independently valuable — they can be queried manually (`grep` / `jq`) even before the observe dashboard consumes them.

### Metric Budget

~4-6 new metric events per connection lifecycle (upgrade, connect, open, close, ticket_fetch, error). Under normal operation, this is negligible I/O.

**Reconnect storm behavior:** During reconnect storms, the frontend can generate bursts of lifecycle telemetry. The `/api/telemetry` endpoint is rate-limited at 60 RPM per IP (`server.go:945`) and intentionally NOT exempt (each write causes a synchronous fsync). To avoid dropping diagnostics during the incidents when they matter most, the frontend should batch lifecycle events client-side: accumulate events in an array and flush in a single `POST /api/telemetry` every 5 seconds, or on page unload via `fetch(..., {keepalive: true})` (not `sendBeacon`, which cannot set `Authorization` headers).

**Batched telemetry payload schema:** The backend `handleTelemetry` accepts two formats, distinguished by the presence of the `batch` field:

```json
// Single event (existing, unchanged):
{ "type": "frontend.ws", "data": { "event": "close", "code": 1006 } }

// Batched events (new):
{ "batch": [
    { "type": "frontend.ws", "data": { "event": "close", "code": 1006 } },
    { "type": "frontend.ws", "data": { "event": "connect_attempt", "attempt": 3 } }
  ]
}
```

The handler checks for `batch` first. If present, it iterates and validates each entry against the allowlist individually. If absent, it processes the single `type`/`data` as before. Each entry in a batch is logged as a separate JSONL event. Partial ingestion: if one entry in a batch has an invalid type, it is skipped (logged as warning), remaining entries are still processed.

## Layer 2: Reconnect State Recovery

### 2.1 Reconnect Handshake

**File:** `web/src/hooks/useChat.ts`

When `connection.ready` arrives and `sessionIDRef.current` is already set, this is a reconnect (not first load). Send `session.switch` to re-subscribe and refresh history.

```
on connection.ready:
  if sessionIDRef.current is set:
    // RECONNECT — re-subscribe to current session
    setIsStreaming(false)
    isStreamingRef.current = false
    // Remove orphaned streaming placeholder from messages
    setMessages(prev => prev.filter(m => m.id !== STREAMING_MESSAGE_ID))
    send('session.switch', { sessionId: sessionIDRef.current })
  else if savedId in localStorage:
    // FIRST LOAD — restore from storage
    sessionIDRef.current = savedId
    setCurrentSessionID(savedId)
    send('session.switch', { sessionId: savedId })
```

The server's existing `handleSessionSwitch` handler subscribes the client, sends `session.history`, and sends `session.list`. No backend changes needed for this.

### 2.2 isStreaming Guard Reset

**File:** `web/src/hooks/useChat.ts`

The guard at line 288 (`if (isStreamingRef.current) break;`) blocks `session.history` during streaming. On reconnect, `isStreaming` may still be true from the pre-disconnect state. The reconnect handler must:

1. Set both `setIsStreaming(false)` and `isStreamingRef.current = false` directly (not waiting for the useEffect sync at line 167, which runs asynchronously after render)
2. Remove the orphaned `STREAMING_MESSAGE_ID` placeholder from messages state — no `chat.done` or `chat.error` will arrive for it since the old stream context is gone
3. Only then send `session.switch` to load fresh history

Both side effects are included in the reconnect pseudocode in Section 2.1.

### 2.3 Session Completion on Stream Error

**File:** `internal/chat/ws/handler.go`

When a client disconnects, `OnClientDisconnect` (line 999) cancels the agent context via `entry.cancel()`. If `Stream()` at line 508 is still in progress (e.g., waiting for the first SSE event from Claude), it returns `context.Canceled`. The error path calls `sendClassifiedError` and returns without calling `completeSession`. The session stays `running` permanently.

Note: the agent context is derived from `context.Background()` (line 305), not the client context — so normal client disconnection does NOT cancel the stream. Only `OnClientDisconnect`'s explicit `entry.cancel()` does.

Fix: `context.Canceled` fires for three distinct reasons — `chat.stop`, superseded streams (new message in same session), and `OnClientDisconnect`. Each requires different session lifecycle handling. But `runStream` cannot distinguish them because all three cancel the same `agentCtx`.

**Solution: move session lifecycle to the callers, not `runStream`.** The callers know WHY the cancel happened:

In `runStream` — do NOT call `completeSession` for `context.Canceled`. Just return silently:

```go
if err != nil {
    if errors.Is(err, context.Canceled) {
        // Expected: chat.stop, superseded stream, or client disconnect.
        // Caller handles session lifecycle — they know the intent.
        return
    }
    if errors.Is(err, context.DeadlineExceeded) {
        // 5-minute stream timeout — complete session and notify user.
        h.completeSession(client, sessionID)
        h.logAPIError(sessionID, err)
        h.sendClassifiedError(client, sessionID, err)
        return
    }
    // Real API error — log and notify.
    h.logAPIError(sessionID, err)
    h.sendClassifiedError(client, sessionID, err)
    return
}
```

In the callers, after `<-entry.done`:

- **`handleChatStop`** (line 966): after `<-entry.done`, call `h.completeSession(client, msg.SessionID)`. User intentionally stopped — session should complete.
- **`handleChatSend` superseded** (line 298): after `<-existing.done`, call `h.sessionStore.UpdateSessionStatus(sessionID, session.StatusRunning)`. The old stream is gone; the new one is taking over — session should stay/return to running.
- **`OnClientDisconnect`** (line 1019): after `<-entry.done`, call `h.completeSession(client, sessionID)`. Client is gone — session should complete so it's not stuck in running.

### Streaming During Disconnect

When a client disconnects while a stream is in progress:
- `OnClientDisconnect` (handler.go:999) cancels all agent contexts for the client via `entry.cancel()`
- The stream goroutine receives `context.Canceled` from `Stream()` and returns silently (per the fix in 2.3)
- `OnClientDisconnect` waits on `<-entry.done`, then calls `completeSession` to transition the session out of `running`
- Any partial text already stored in SQLite is preserved

On reconnect:
- `session.switch` (Section 2.1) fetches `session.history` which includes any messages stored before cancellation
- The session shows as `completed` (not stuck in `running`)
- Tokens that were streamed to the old client object but not yet stored are lost

This is acceptable — the user sees the completed partial response. Full token recovery would require a replay buffer, which is out of scope.

## Layer 3: Connection Resilience

### 3.1 Ticket Fetch Timeout + Abort

**File:** `web/src/lib/ws.ts`

Add `AbortController` with 5-second timeout to `fetchWSTicket()`:

```
const controller = new AbortController()
const timeout = setTimeout(() => controller.abort(), 5000)
fetch('/api/ws-ticket', { signal: controller.signal, ... })
  .finally(() => clearTimeout(timeout))
```

On timeout or error, returns `null` (falls back to raw token). Layer 1's `ticket_fetch` telemetry captures the failure for diagnostics.

### 3.2 Close Code Awareness

**File:** `web/src/hooks/useWebSocket.ts`

Inspect `CloseEvent` in `onclose` and classify:

| Code | Meaning | Action |
|------|---------|--------|
| 1000 | Normal close | Reconnect (normal backoff) |
| 1001 | Going away | Reconnect (normal backoff) |
| 1006 | Abnormal (no close frame) | Reconnect (network blip). Note: auth failures (HTTP 401 before upgrade) also produce 1006 — handled by the circuit breaker in 3.3, not here. |
| 1011 | Server error | Reconnect with longer initial backoff |

### 3.3 Auth Circuit Breaker

**Files:** `web/src/lib/ws.ts`, `web/src/hooks/useWebSocket.ts`

Auth failures return HTTP 401 before the WebSocket upgrade, so the browser fires `onerror` + `onclose` with code 1006 — indistinguishable from a network blip via close codes alone. A generic "pre-open failure" counter would false-positive on server restarts, DNS blips, and proxy issues, blocking auto-recovery for non-auth outages.

**Solution: use the ticket fetch as the auth probe.** The ticket endpoint (`GET /api/ws-ticket`) requires the same auth as the WS upgrade. Its HTTP response code is the definitive auth signal:

```
fetchWSTicket() enhanced to return { ticket, status }:
  - status 200 → auth OK, use ticket
  - status 401 → auth failure, increment authFailureCount
  - status 0/timeout/network error → not auth, just use fallback
  - other status → not auth, just use fallback

In connect():
  result = fetchWSTicket()
  if result.status === 401:
    consecutiveAuthFailures++
    if consecutiveAuthFailures >= 2:
      status = 'error', stop reconnecting
      // User must reauth or use manual reconnect()
    return  // don't even attempt WS upgrade
  else:
    consecutiveAuthFailures = 0  // any non-401 resets
    proceed with WebSocket(getWebSocketURL(result.ticket))
```

**Why this is better:** Only actual 401 responses trigger the circuit breaker. Server restarts (connection refused → status 0), DNS failures (network error → status 0), and proxy issues all reset the counter and continue normal reconnection. The circuit breaker fires after just 2 consecutive 401s (not 3 generic failures) because 401 is unambiguous.

**Interaction with ticket fetch timeout (3.1):** A ticket fetch timeout aborts with status 0, which does NOT increment `consecutiveAuthFailures`. Only explicit 401 responses count.

### 3.4 Reconnect Give-Up with Manual Retry

**File:** `web/src/hooks/useWebSocket.ts`

After 10 consecutive reconnect failures, stop auto-reconnecting and set status to `error`. Expose a `reconnect()` function from `useWebSocket` that the UI can wire to a "Retry Connection" button.

The `useWebSocket` return type gains:

```ts
interface UseWebSocketReturn {
  status: ConnectionState;
  send: (type: string, payload: Record<string, unknown>) => void;
  reconnectAttempt: number;
  reconnect: () => void;  // NEW — manual reconnect trigger
}
```

### 3.5 Visibility-Aware Reconnect

**File:** `web/src/hooks/useWebSocket.ts`

When the tab is hidden, don't attempt reconnects — they'll likely fail or be throttled. On visibility restore, reconnect immediately if disconnected:

```
document.addEventListener('visibilitychange', () => {
  if (document.visibilityState === 'visible' && status === 'disconnected') {
    resetBackoff()
    connect()
  }
})
```

If the connection drops while backgrounded, reconnect is deferred until the tab becomes visible. Avoids wasted reconnect attempts and ticket fetch overhead.

**Interaction with circuit breaker (3.3/3.4):** Visibility restore only triggers reconnect when `status === 'disconnected'`, NOT when `status === 'error'` (circuit breaker has fired). The user must use the explicit `reconnect()` button to clear error state and retry.

## Layer 4: Token Coalescing + Tool Progress Wire Format

### 4.1 WritePump Token Coalescing

**File:** `internal/chat/ws/client.go`

Replace the current one-message-per-frame WritePump with a coalescing loop:

```
WritePump loop:
  select:
    case msg from send channel:
      // Got first message — enter coalesce drain
      batch = [msg]
      drain loop (non-blocking):
        select:
          case msg from send channel:
            batch = append(batch, msg)
            if len(batch) >= 32 OR time.Since(batchStart) >= 5ms:
              break drain loop
          default:
            break drain loop  // channel empty, send immediately
      write batch as single WS frame

    case ticker.C (30s ping):
      send ping, wait for pong (existing behavior)

    case ctx.Done():
      return
```

**Key constraint:** The coalesce drain uses non-blocking channel reads (`select` with `default`) and a 5ms wall-clock check (`time.Since`), NOT a blocking `time.After` channel. This ensures ping handling and context cancellation are never deferred by more than one drain cycle. The outer `select` still handles all three cases (messages, pings, shutdown) exactly as the current WritePump does.

**Parameters:**
- Coalesce window: 5ms (imperceptible, lets ~50 tokens/sec batch into groups of 2-5)
- Max batch size: 32 messages (caps latency if channel is flooding)
- Single-message optimization: no array wrapper when batch size is 1

**Impact:**
- At ~100 tokens/sec peak, reduces WS frames from 100/sec to ~20/sec
- 256-slot send channel effectively becomes 2560+ tokens of headroom
- Slow-client disconnect becomes a non-issue for realistic scenarios

### 4.2 Frontend Array Frame Parser

**File:** `web/src/hooks/useWebSocket.ts`

The `onmessage` handler parses both single objects and arrays:

```ts
socket.onmessage = (event) => {
  const parsed = JSON.parse(event.data);
  const messages = Array.isArray(parsed) ? parsed : [parsed];
  for (const msg of messages) {
    dispatch(msg);
  }
};
```

**Compatibility scope:** The only WS consumer is the web frontend, deployed atomically with the backend (same `make build`). No external WS clients or protocol versioning needed.

**Test impact:** Existing handler tests (`handler_test.go`) use a `readMessage` helper that reads one frame and parses it as a single JSON object. The bootstrap sequence sends `connection.ready` then `session.list` as separate `client.Send()` calls. With coalescing, if both arrive in the send channel before WritePump processes them (likely with fast in-memory test databases), they get batched into one array frame. This breaks ~25 test call sites that expect separate reads (e.g., `handler_test.go:137-148`, `handler_test.go:164-166`).

**Fix:** Update the `readMessage` test helper to unwrap array frames: if the frame is a JSON array, buffer the messages and return them one at a time across calls. This is a single helper change, not per-test changes. Alternatively, the single-message optimization (batch=1 → plain object) preserves compatibility when messages are spaced apart, but the coalescing window makes this unreliable in tests where sends happen back-to-back.

### 4.3 Tool Progress Wire Format

**Extended outbound message type:** `tool.progress` — already partially implemented in `useChat.ts:457-470` with `{ id: string; progress: number }` payload. The existing handler must be augmented (not replaced) to support the new fields while preserving the current `progress` number handling.

Extended payload for future tool detail UI:

```json
{
  "type": "tool.progress",
  "sessionId": "uuid",
  "data": {
    "id": "toolu_xxx",
    "event": "step",
    "detail": "Searching handler.go:107-129",
    "progress": 0.4,
    "ts": 1710600000000
  }
}
```

| Field | Type | Purpose |
|-------|------|---------|
| `id` | string | Tool call ID (matches `tool.call` id) |
| `event` | enum | `"step"` (activity), `"warning"` (non-fatal), `"metric"` (timing) |
| `detail` | string | Human-readable activity description |
| `progress` | number? | 0.0-1.0 fractional progress (optional) |
| `ts` | number | Unix ms timestamp for ordering |

**Backend emission points** (`handler.go` tool dispatch loop, lines 684-731):
- Before dispatch: `tool.progress` with `detail: "Calling {toolName}..."`
- After dispatch: `tool.progress` with `detail: "Completed in {duration}ms"` or `"Error: {msg}"`

Product-specific progress (e.g., "Searching file X") comes from product servers in a future iteration. The wire format supports it now.

**Frontend storage:** Extend `ToolCallData` type with `steps: ProgressStep[]` array. `useChat.ts` tool.progress handler appends to the array. Collapse/expand UI is a separate spec.

```ts
interface ProgressStep {
  event: 'step' | 'warning' | 'metric';
  detail: string;
  progress?: number;
  ts: number;
}

interface ToolCallData {
  id: string;
  name: string;
  input: Record<string, unknown>;
  status: 'running' | 'complete' | 'error';
  output?: string;
  progress?: number;
  steps?: ProgressStep[];  // NEW
}
```

**Coalescing interaction:** Tool progress events (~200 bytes each) batch naturally with token events in the coalescing window. No special handling needed.

## Files Modified

### Layer 1 (Observability)
| File | Change |
|------|--------|
| `internal/chat/ws/client.go` | Add `closeReason` field, classify disconnect reasons, emit in metric |
| `internal/chat/ws/hub.go` | Emit `ws.upgrade` metric in `HandleUpgrade`; set `server_shutdown` reason in `Run` shutdown path |
| `internal/chat/server/server.go` | Emit `ws.upgrade` with `auth_rejected` in auth middleware `/ws` path; accept batched telemetry payloads in `handleTelemetry` |
| `web/src/hooks/useWebSocket.ts` | Emit lifecycle telemetry events |
| `web/src/lib/telemetry.ts` | Add `reportWSLifecycle` helper; add client-side telemetry batching (5s flush / `fetch({keepalive})` on unload) |
| `internal/observe/server/handlers.go` | Deferred — requires new `ConnectionHealthReport` type and dynamic resilient pillar (see Section 1.5). Separate observe spec recommended. |
| `internal/chat/metrics/aggregator.go` | Deferred — requires new report struct for WS disconnect/upgrade aggregation (see Section 1.5). |

### Layer 2 (Reconnect Recovery)
| File | Change |
|------|--------|
| `web/src/hooks/useChat.ts` | Re-send `session.switch` on reconnect, reset `isStreaming` |
| `internal/chat/ws/handler.go` | `runStream`: return silently on `context.Canceled`; `handleChatStop`: call `completeSession` after done; `handleChatSend` superseded: re-set status to running after done; `OnClientDisconnect`: call `completeSession` after done |

### Layer 3 (Connection Resilience)
| File | Change |
|------|--------|
| `web/src/lib/ws.ts` | AbortController + 5s timeout on ticket fetch; return `{ ticket, status }` for auth circuit breaker |
| `web/src/hooks/useWebSocket.ts` | Close code inspection, auth circuit breaker (401-based), give-up logic, visibility handling, expose `reconnect()` |
| `web/src/hooks/useChat.ts` | Thread `reconnect` through `UseChatReturn` so UI can wire "Retry Connection" button |

### Layer 4 (Token Coalescing + Tool Progress)
| File | Change |
|------|--------|
| `internal/chat/ws/client.go` | Coalescing WritePump loop |
| `web/src/hooks/useWebSocket.ts` | Array frame parser |
| `internal/chat/ws/handler.go` | Emit `tool.progress` events during dispatch |
| `internal/chat/ws/message.go` | Add `NewToolProgress` constructor |
| `web/src/hooks/useChat.ts` | Extend tool.progress handler, add `steps` to `ToolCallData` |
| `web/src/lib/types.ts` | Add `ProgressStep` type (generated from spec) |

## Testing Strategy

### Layer 1
- Unit test: `client_test.go` — verify each disconnect path sets correct `closeReason`
- Unit test: `hub_test.go` — verify `ws.upgrade` metric emitted on success/rejection
- Integration test: verify frontend telemetry events arrive at `/api/telemetry`

### Layer 2
- Unit test: `handler_test.go` — verify `completeSession` called on `context.Canceled`
- Manual test: disconnect WiFi mid-stream, reconnect, verify history refreshes

### Layer 3
- Unit test: `useWebSocket` — verify auth circuit breaker stops after 2 consecutive 401s from ticket fetch
- Unit test: `useWebSocket` — verify non-401 errors (network, timeout, server restart) do NOT trigger circuit breaker
- Unit test: `fetchWSTicket` — verify 5s abort, verify fallback to raw token, verify `{ ticket, status }` return
- Unit test: `useWebSocket` — verify give-up after 10 consecutive reconnect failures
- Manual test: stop server, verify client reconnects after restart; invalidate auth, verify circuit breaker fires after 2 attempts

### Layer 4
- Unit test: `client_test.go` — verify coalescing batches messages, single-message passthrough, 32-message cap
- Unit test: `useWebSocket` — verify array frame parsing
- Unit test: `handler_test.go` — verify `tool.progress` emitted before/after dispatch
- Test helper: update `readMessage` in `handler_test.go` to unwrap array frames (buffer and return one at a time)
- Load test: stream 2000 tokens, verify no slow-client disconnect

## Out of Scope

- **Replay buffer** — recovering tokens lost during disconnect. Messages are persisted to SQLite; the user gets them on reconnect via `session.history`.
- **Tool detail collapse/expand UI** — frontend rendering of `tool.progress` steps. Wire format is defined here; UI is a separate spec.
- **Send queue** — queuing outbound messages during disconnect for replay after reconnect. The deferred session creation pattern (`pendingMessageRef`) handles the most common case. A full send queue adds complexity without proportional value for a single-user setup.
- **HTTP/2 multiplexing** — replacing WebSocket with HTTP/2 SSE + POST. Architectural change beyond current scope.
