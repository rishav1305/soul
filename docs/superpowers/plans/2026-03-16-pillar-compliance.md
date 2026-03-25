# Pillar-Compliant Chat Observability & Resilience Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Close observability and resilience gaps so the 6-pillar system reflects real-time chat health — not just static constraints and event counts.

**Architecture:** Add structured event emission at every failure/recovery boundary (server middleware, WS lifecycle, stream errors, frontend reconnect). Replace static resilient pillar with live constraint scoring derived from JSONL metrics (the observe server reads metrics files, not in-memory state). Add client-side send queue with idempotent message IDs for zero-loss reconnects. Add server-side replay window for interrupted streams.

**Tech Stack:** Go 1.24 (nhooyr.io/websocket), React 19, TypeScript 5.9, SQLite WAL, SSE streaming

**Key API contracts (read before implementing):**
- `EventLogger.Log` signature: `func (l *EventLogger) Log(eventType string, data map[string]interface{}) error`
- WebSocket library: `nhooyr.io/websocket` — use `websocket.CloseStatus(err)` (returns `websocket.StatusCode`), NOT gorilla APIs
- Client fields: `connTime` (not `connectedAt`), `SessionID()` method (not `sessionID` field — it's `atomic.Value`)
- Hub uses channel-based serialization, not mutexes. `ClientCount` already exists via `countReq` channel.
- Observe server (`internal/observe/server/`) is a **separate process** on port 3010. It reads JSONL metrics files from `~/.soul-v2/` via `metrics.Aggregator`. It has NO access to the chat server's Hub or in-memory state.
- `authMiddleware` in `server.go:829` is a **free function** `authMiddleware(token string, logger *metrics.EventLogger) func(http.Handler) http.Handler` after Task 4 refactor. Before refactor it takes only `token string`.
- **JSONL format**: Fields are `ts` (RFC3339Nano) and `event` (not `timestamp`/`event_type`). See `types.go:88-91` and `reader.go:230-233`.
- **Aggregator constructors**: `NewAggregator(dataDir string)` (1 arg) or `NewAggregatorForProduct(dataDir, product string)` (2 args). NOT `NewAggregator(dir, "")`.
- **Aggregator reader**: Use `a.readProductEvents(typePrefix)` (not `a.readEvents()`). Pass `""` for all events.
- **Telemetry allowlist** (`server.go:687`): Only accepts `frontend.error|frontend.render|frontend.ws|frontend.usage`. New frontend event types must be added to this switch.
- **Outbound message structure**: All outbound WS messages nest data under `data.*`. E.g. `session.created` → `data.session`, `session.history` → `data.messages`. See `message.go:195-233`.
- **`sendToClient`** (`handler.go:1128`): Calls `MarshalOutbound(msg)` then `client.Send(data)`. The marshaled `[]byte` is available for replay storage by intercepting after marshal.

---

## Current State Summary

| Pillar | Current Scoring | Gap |
|--------|----------------|-----|
| **Transparent** | Event count >0, action types >0 | No connection reliability signals (drop rate, reconnect latency, auth failures) |
| **Resilient** | 3 static constraints (auto-reconnect, stream-recovery, graceful-shutdown) | No live WS drop/reconnect health scoring |
| **Performant** | First-token P50, DB P50, HTTP P50 | Missing stream-total latency |
| **Robust** | Frontend error count + 2 static | No chaos/fault-injection verification |

## File Structure

### New Files

| File | Responsibility |
|------|---------------|
| `internal/chat/metrics/connection_health.go` | Sliding-window aggregation for connection health metrics |
| `internal/chat/metrics/connection_health_test.go` | Unit tests for connection health |
| `internal/chat/ws/replay.go` | Server-side message replay buffer for interrupted streams |
| `internal/chat/ws/replay_test.go` | Unit tests for replay buffer |
| `web/src/lib/sendQueue.ts` | Durable send queue with idempotent message IDs |
| `web/src/lib/sendQueue.test.ts` | Unit tests for send queue |
| `tests/integration/resilience_test.go` | Integration tests: kill mid-stream, WS disconnect storms, auth expiry |

### Modified Files

| File | Changes |
|------|---------|
| `internal/chat/metrics/types.go` | Add new event types for auth, WS lifecycle, reconnect |
| `internal/chat/server/server.go` | Refactor authMiddleware to accept metrics logger, emit auth events, add /healthz, extend telemetry allowlist |
| `internal/chat/ws/hub.go` | Emit `ws.close` with classified reason, wire ConnectionHealth, add ReplayBuffer |
| `internal/chat/ws/client.go` | Add `closeCode` field, capture close status from nhooyr.io/websocket |
| `internal/chat/ws/handler.go` | Add `session.resume` handler, store outbound messages in replay buffer, add product/model to stream events |
| `internal/chat/ws/message.go` | Add `TypeSessionResume` constant, `LastMessageID` field to `InboundMessage` |
| `internal/chat/metrics/aggregator.go` | Add methods to count events by type and compute connection health from JSONL |
| `internal/observe/server/handlers.go` | Replace static resilient constraints with JSONL-derived live scoring, enhance transparent |
| `web/src/hooks/useWebSocket.ts` | Classify disconnect reasons, emit structured reconnect events |
| `web/src/hooks/useChat.ts` | Integrate send queue, add idempotent message IDs |
| `web/src/lib/telemetry.ts` | Extend `TelemetryEvent` union, add `reportReconnect()`, `reportDisconnect()`, `reportAuthFailure()` |

---

## Chunk 1: Event Taxonomy & Metric Types

### Task 1: Add New Event Types

**Files:**
- Modify: `internal/chat/metrics/types.go:10-70`
- Test: `internal/chat/metrics/types_test.go` (create if not exists)

- [ ] **Step 1: Write test for new event type constants**

```go
// internal/chat/metrics/types_test.go
package metrics

import "testing"

func TestEventTypeConstants(t *testing.T) {
	events := []string{
		EventSystemExit,
		EventAuthFail,
		EventAuthOK,
		EventWSClose,
		EventWSReconnectAttempt,
		EventWSReconnectSuccess,
		EventWSReconnectFail,
		EventWSStreamResume,
	}
	for _, e := range events {
		if e == "" {
			t.Error("event type constant must not be empty")
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /home/rishav/soul-v2 && go test ./internal/chat/metrics/ -run TestEventTypeConstants -v`
Expected: FAIL — undefined constants

- [ ] **Step 3: Add new event type constants**

Add to `internal/chat/metrics/types.go` after the existing event constants (around line 70):

```go
// System lifecycle
EventSystemExit = "system.exit"

// Auth events
EventAuthFail = "auth.fail"
EventAuthOK   = "auth.ok"

// WebSocket lifecycle (extended)
EventWSClose            = "ws.close"
EventWSReconnectAttempt = "ws.reconnect.attempt"
EventWSReconnectSuccess = "ws.reconnect.success"
EventWSReconnectFail    = "ws.reconnect.fail"

// Stream lifecycle (extended)
EventWSStreamResume = "ws.stream.resume"

// Frontend telemetry (extended) — must also be added to server.go telemetry allowlist
EventFrontendWSDisconnect = "frontend.ws.disconnect"
EventFrontendWSReconnect  = "frontend.ws.reconnect"
EventFrontendAuthFail     = "frontend.auth.fail"
```

Note: `ws.reconnect.success` and `ws.reconnect.fail` are emitted by the **Hub** when a client re-registers after disconnect (see Task 7). The Hub can detect reconnects by tracking client IPs or by the frontend sending a `session.resume` message that signals "this is a reconnect". The frontend also emits `frontend.ws.reconnect` via the telemetry endpoint. Both paths feed into `ConnectionHealthReport` via JSONL.

- [ ] **Step 4: Run test to verify it passes**

Run: `cd /home/rishav/soul-v2 && go test ./internal/chat/metrics/ -run TestEventTypeConstants -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/chat/metrics/types.go internal/chat/metrics/types_test.go
git commit -m "feat: add event type constants for auth, WS lifecycle, reconnect observability"
```

---

### Task 2: Add Close Code Classification Helper

**Files:**
- Modify: `internal/chat/ws/client.go`
- Test: `internal/chat/ws/client_test.go` (add test)

Note: nhooyr.io/websocket uses `websocket.CloseStatus(err)` which returns `websocket.StatusCode` (an int). The existing code at `client.go:174` already uses this API.

- [ ] **Step 1: Write test for close code classifier**

```go
// Add to internal/chat/ws/client_test.go
func TestClassifyCloseCode(t *testing.T) {
	tests := []struct {
		code     int
		expected string
	}{
		{1000, "normal"},
		{1001, "client_nav"},
		{1006, "network"},
		{1008, "auth"},
		{1011, "server_error"},
		{1012, "server_restart"},
		{1013, "server_restart"},
		{4001, "auth"},
		{4000, "unknown"},
		{0, "unknown"},
	}
	for _, tt := range tests {
		got := classifyCloseCode(tt.code)
		if got != tt.expected {
			t.Errorf("classifyCloseCode(%d) = %q, want %q", tt.code, got, tt.expected)
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /home/rishav/soul-v2 && go test ./internal/chat/ws/ -run TestClassifyCloseCode -v`
Expected: FAIL — undefined function

- [ ] **Step 3: Implement close code classifier**

Add to `internal/chat/ws/client.go`:

```go
// classifyCloseCode maps WebSocket close codes to human-readable reason classes.
func classifyCloseCode(code int) string {
	switch code {
	case 1000:
		return "normal"
	case 1001:
		return "client_nav"
	case 1006:
		return "network"
	case 1008, 4001:
		return "auth"
	case 1011:
		return "server_error"
	case 1012, 1013:
		return "server_restart"
	default:
		return "unknown"
	}
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd /home/rishav/soul-v2 && go test ./internal/chat/ws/ -run TestClassifyCloseCode -v`
Expected: PASS

- [ ] **Step 5: Add closeCode field to Client and capture it in ReadPump**

Add a `closeCode` field to the `Client` struct in `client.go`:

```go
type Client struct {
	// ... existing fields ...
	closeCode int // WebSocket close code from last disconnect
}
```

In the `ReadPump` method, after `c.conn.Read(c.ctx)` returns an error (around line 172), capture the close status using the nhooyr.io/websocket API:

```go
if err != nil {
	c.closeCode = int(websocket.CloseStatus(err))
	// ... existing error handling (lines 174-181) ...
```

- [ ] **Step 6: Emit ws.close event in ReadPump's deferred cleanup**

In the deferred cleanup function in `ReadPump` (around line 160-167), enhance the existing `EventWSDisconnect` log to include close code classification. Replace the existing log call:

```go
if c.hub.metrics != nil {
	duration := time.Since(c.connTime).Seconds()
	_ = c.hub.metrics.Log(metrics.EventWSClose, map[string]interface{}{
		"client_id":        c.id,
		"close_code":       c.closeCode,
		"reason_class":     classifyCloseCode(c.closeCode),
		"duration_seconds": duration,
	})
}
```

- [ ] **Step 7: Run full ws package tests**

Run: `cd /home/rishav/soul-v2 && go test ./internal/chat/ws/ -v`
Expected: PASS

- [ ] **Step 8: Commit**

```bash
git add internal/chat/ws/client.go internal/chat/ws/client_test.go
git commit -m "feat: add WebSocket close code classifier and ws.close event emission"
```

---

## Chunk 2: Server-Side Auth Event Emission & Telemetry Allowlist

### Task 3: Extend Telemetry Endpoint Allowlist

**Files:**
- Modify: `internal/chat/server/server.go:686-692`

The telemetry endpoint (`/api/telemetry`) has a switch allowlist that only accepts 4 event types. New frontend telemetry events (`frontend.ws.disconnect`, `frontend.ws.reconnect`, `frontend.auth.fail`) will be rejected with 400. We must add them.

- [ ] **Step 1: Add new event types to the allowlist**

In `server.go:686-692`, change the switch to:

```go
switch payload.Type {
case metrics.EventFrontendError, metrics.EventFrontendRender, metrics.EventFrontendWS, metrics.EventFrontendUsage,
	metrics.EventFrontendWSDisconnect, metrics.EventFrontendWSReconnect, metrics.EventFrontendAuthFail:
	// OK — known event types
default:
	http.Error(w, "unknown event type", http.StatusBadRequest)
	return
}
```

This requires adding the new constants to `types.go` (done in Task 1). Add these constants in Task 1:

```go
// Frontend telemetry (extended)
EventFrontendWSDisconnect = "frontend.ws.disconnect"
EventFrontendWSReconnect  = "frontend.ws.reconnect"
EventFrontendAuthFail     = "frontend.auth.fail"
```

- [ ] **Step 2: Verify build compiles**

Run: `cd /home/rishav/soul-v2 && go build ./cmd/chat/`
Expected: Success

- [ ] **Step 3: Commit**

```bash
git add internal/chat/server/server.go internal/chat/metrics/types.go
git commit -m "feat: extend telemetry allowlist with frontend disconnect/reconnect/auth events"
```

---

### Task 4: Refactor authMiddleware to Accept Metrics Logger (was Task 3)

**Files:**
- Modify: `internal/chat/server/server.go:829-867`
- Test: `internal/chat/server/server_test.go`

The current `authMiddleware` is a free function `authMiddleware(token string)` with no access to the server or metrics. We need to add an optional metrics parameter.

- [ ] **Step 1: Write test for auth event emission**

```go
// Add to internal/chat/server/server_test.go
func TestAuthMiddleware_EmitsAuthFailEvent(t *testing.T) {
	tmpDir := t.TempDir()
	logger, err := metrics.NewEventLogger(tmpDir, "")
	if err != nil {
		t.Fatal(err)
	}

	handler := authMiddleware("test-token", logger)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Request to protected path with no token
	req := httptest.NewRequest("GET", "/api/sessions", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}

	// Read metrics file and check for auth.fail event
	events := readMetricsFile(t, tmpDir)
	found := false
	for _, e := range events {
		if e.EventType == "auth.fail" {
			found = true
			if e.Data["source"] != "api" {
				t.Errorf("expected source=api, got %v", e.Data["source"])
			}
		}
	}
	if !found {
		t.Error("auth.fail event not emitted on 401")
	}
}

// readMetricsFile reads all events from the metrics JSONL file in the given directory.
func readMetricsFile(t *testing.T, dir string) []metrics.Event {
	t.Helper()
	files, _ := filepath.Glob(filepath.Join(dir, "metrics*.jsonl"))
	var events []metrics.Event
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		for _, line := range strings.Split(string(data), "\n") {
			if line == "" {
				continue
			}
			var ev metrics.Event
			if err := json.Unmarshal([]byte(line), &ev); err == nil {
				events = append(events, ev)
			}
		}
	}
	return events
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /home/rishav/soul-v2 && go test ./internal/chat/server/ -run TestAuthMiddleware_EmitsAuthFailEvent -v`
Expected: FAIL — wrong number of arguments to authMiddleware

- [ ] **Step 3: Refactor authMiddleware signature**

Change `authMiddleware` in `server.go:829` from:

```go
func authMiddleware(token string) func(http.Handler) http.Handler {
```

To:

```go
func authMiddleware(token string, logger *metrics.EventLogger) func(http.Handler) http.Handler {
```

After writing the 401 response (around line 862-864), add:

```go
w.Header().Set("Content-Type", "application/json")
w.WriteHeader(http.StatusUnauthorized)
w.Write([]byte(`{"error":"unauthorized"}`))

// Emit auth failure event
if logger != nil {
	_ = logger.Log(metrics.EventAuthFail, map[string]interface{}{
		"source":    "api",
		"reason":    "missing_or_invalid_token",
		"client_ip": r.RemoteAddr,
		"path":      r.URL.Path,
	})
}
```

- [ ] **Step 4: Update the call site in setupMiddleware**

Find where `authMiddleware(token)` is called (in `Server.setupMiddleware` or wherever the middleware chain is built) and add the metrics logger:

```go
authMiddleware(s.authToken, s.metrics)
```

- [ ] **Step 5: Run test to verify it passes**

Run: `cd /home/rishav/soul-v2 && go test ./internal/chat/server/ -run TestAuthMiddleware_EmitsAuthFailEvent -v`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/chat/server/server.go internal/chat/server/server_test.go
git commit -m "feat: refactor authMiddleware to emit auth.fail events via metrics logger"
```

---

### Task 4: Emit WS Auth Failure Events

**Files:**
- Modify: `internal/chat/ws/hub.go` (HandleUpgrade method)

The WS auth check happens in `authMiddleware` (server.go:857) via query param check before the request reaches the Hub. But the Hub's `HandleUpgrade` also validates the origin. We should emit `auth.fail` for origin rejection since those failures are also invisible.

- [ ] **Step 1: Find the origin validation failure path in HandleUpgrade**

Read `hub.go`'s `HandleUpgrade` to find where origin is rejected.

- [ ] **Step 2: Add auth.fail/origin event on origin rejection**

In the origin validation failure path within `HandleUpgrade`, add:

```go
if h.metrics != nil {
	_ = h.metrics.Log(metrics.EventAuthFail, map[string]interface{}{
		"source":    "ws",
		"reason":    "origin_rejected",
		"client_ip": r.RemoteAddr,
		"origin":    r.Header.Get("Origin"),
	})
}
```

- [ ] **Step 3: Verify build compiles**

Run: `cd /home/rishav/soul-v2 && go build ./cmd/chat/`
Expected: Success

- [ ] **Step 4: Commit**

```bash
git add internal/chat/ws/hub.go
git commit -m "feat: emit auth.fail events on WebSocket origin rejection"
```

---

### Task 5: Emit system.exit on Server Shutdown (Graceful + Panic)

**Files:**
- Modify: `cmd/chat/main.go`

- [ ] **Step 1: Read current main.go**

Read `cmd/chat/main.go` to understand the current shutdown flow and where the signal handler is.

- [ ] **Step 2: Add system.exit event emission for graceful shutdown**

In the signal handler / graceful shutdown section, before the server stops, add:

```go
if logger != nil {
	_ = logger.Log(metrics.EventSystemExit, map[string]interface{}{
		"signal":    sig.String(),
		"exit_code": 0,
		"reason":    "graceful_shutdown",
		"uptime_s":  time.Since(startTime).Seconds(),
	})
}
```

Where `logger` is the `*metrics.EventLogger` and `startTime` is captured at startup.

- [ ] **Step 3: Add panic recovery wrapper at main() level**

Wrap the entire main body in a deferred panic handler that logs before crashing. This catches panics that escape goroutines (the ones that crash the process):

```go
func main() {
	startTime := time.Now()
	var logger *metrics.EventLogger

	defer func() {
		if r := recover(); r != nil {
			if logger != nil {
				_ = logger.Log(metrics.EventSystemExit, map[string]interface{}{
					"exit_code": 2,
					"reason":    "panic",
					"panic":     fmt.Sprintf("%v", r),
					"stack":     string(debug.Stack()),
					"uptime_s":  time.Since(startTime).Seconds(),
				})
			}
			// Re-panic to preserve original crash behavior
			panic(r)
		}
	}()

	// ... rest of main ...
}
```

This requires `import "runtime/debug"`.

- [ ] **Step 4: Add startup event to detect OOM/SIGKILL restarts**

At startup, after the logger is initialized, emit a `system.start` event. Downstream analysis can detect OOM/SIGKILL patterns by finding `system.start` events NOT preceded by `system.exit` — the "gap" indicates a non-graceful termination:

```go
_ = logger.Log(metrics.EventSystemStart, map[string]interface{}{
	"pid":     os.Getpid(),
	"version": version, // build version if available
})
```

Note: `EventSystemStart` already exists in types.go as `"system.start"`.

- [ ] **Step 5: Verify build compiles**

Run: `cd /home/rishav/soul-v2 && go build ./cmd/chat/`
Expected: Success

- [ ] **Step 6: Commit**

```bash
git add cmd/chat/main.go
git commit -m "feat: emit system.exit on graceful/panic shutdown, system.start for gap detection"
```

---

## Chunk 3: Sliding-Window Connection Health Aggregator

### Task 6: Build Connection Health Aggregator

**Files:**
- Create: `internal/chat/metrics/connection_health.go` (NOT `aggregator.go` — that file already exists with the JSONL `Aggregator` type)
- Create: `internal/chat/metrics/connection_health_test.go`

This tracks sliding-window connection health in memory on the chat server. The observe server will derive equivalent metrics from JSONL (Task 8).

- [ ] **Step 1: Write tests for ConnectionHealth**

```go
// internal/chat/metrics/connection_health_test.go
package metrics

import (
	"testing"
	"time"
)

func TestConnectionHealth_DropRate(t *testing.T) {
	ch := NewConnectionHealth(1 * time.Hour)

	for i := 0; i < 100; i++ {
		ch.RecordConnect()
	}
	for i := 0; i < 99; i++ {
		ch.RecordDisconnect("normal")
	}
	ch.RecordDisconnect("network")

	rate := ch.DropRate()
	if rate < 0.009 || rate > 0.011 {
		t.Errorf("expected drop rate ~0.01, got %f", rate)
	}
}

func TestConnectionHealth_ReconnectLatency(t *testing.T) {
	ch := NewConnectionHealth(1 * time.Hour)

	ch.RecordReconnectLatency(100 * time.Millisecond)
	ch.RecordReconnectLatency(200 * time.Millisecond)
	ch.RecordReconnectLatency(3000 * time.Millisecond)

	p50 := ch.ReconnectP50()
	p95 := ch.ReconnectP95()

	if p50 < 100*time.Millisecond || p50 > 300*time.Millisecond {
		t.Errorf("expected p50 ~200ms, got %v", p50)
	}
	if p95 < 2*time.Second {
		t.Errorf("expected p95 >= 2s, got %v", p95)
	}
}

func TestConnectionHealth_ReconnectSuccessRate(t *testing.T) {
	ch := NewConnectionHealth(1 * time.Hour)

	ch.RecordReconnectAttempt(true)
	ch.RecordReconnectAttempt(true)
	ch.RecordReconnectAttempt(false)

	rate := ch.ReconnectSuccessRate()
	if rate < 0.65 || rate > 0.68 {
		t.Errorf("expected success rate ~0.667, got %f", rate)
	}
}

func TestConnectionHealth_WindowExpiry(t *testing.T) {
	ch := NewConnectionHealth(100 * time.Millisecond)

	ch.RecordConnect()
	ch.RecordDisconnect("network")

	time.Sleep(150 * time.Millisecond)

	rate := ch.DropRate()
	if rate != 0 {
		t.Errorf("expected 0 after window expiry, got %f", rate)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /home/rishav/soul-v2 && go test ./internal/chat/metrics/ -run TestConnectionHealth -v`
Expected: FAIL — type not defined

- [ ] **Step 3: Implement ConnectionHealth**

```go
// internal/chat/metrics/connection_health.go
package metrics

import (
	"sort"
	"sync"
	"time"
)

// ConnectionHealth tracks sliding-window connection health metrics.
type ConnectionHealth struct {
	mu     sync.Mutex
	window time.Duration

	connects    []time.Time
	disconnects []disconnectRecord
	reconnects  []reconnectRecord
	latencies   []latencyRecord
}

type disconnectRecord struct {
	at     time.Time
	reason string
}

type reconnectRecord struct {
	at      time.Time
	success bool
}

type latencyRecord struct {
	at      time.Time
	latency time.Duration
}

func NewConnectionHealth(window time.Duration) *ConnectionHealth {
	return &ConnectionHealth{window: window}
}

func (ch *ConnectionHealth) RecordConnect() {
	ch.mu.Lock()
	defer ch.mu.Unlock()
	ch.connects = append(ch.connects, time.Now())
}

func (ch *ConnectionHealth) RecordDisconnect(reason string) {
	ch.mu.Lock()
	defer ch.mu.Unlock()
	ch.disconnects = append(ch.disconnects, disconnectRecord{at: time.Now(), reason: reason})
}

func (ch *ConnectionHealth) RecordReconnectAttempt(success bool) {
	ch.mu.Lock()
	defer ch.mu.Unlock()
	ch.reconnects = append(ch.reconnects, reconnectRecord{at: time.Now(), success: success})
}

func (ch *ConnectionHealth) RecordReconnectLatency(d time.Duration) {
	ch.mu.Lock()
	defer ch.mu.Unlock()
	ch.latencies = append(ch.latencies, latencyRecord{at: time.Now(), latency: d})
}

// DropRate returns the fraction of connections that ended abnormally within the window.
func (ch *ConnectionHealth) DropRate() float64 {
	ch.mu.Lock()
	defer ch.mu.Unlock()
	cutoff := time.Now().Add(-ch.window)

	total := 0
	for _, c := range ch.connects {
		if !c.Before(cutoff) {
			total++
		}
	}
	if total == 0 {
		return 0
	}

	abnormal := 0
	for _, d := range ch.disconnects {
		if d.at.Before(cutoff) {
			continue
		}
		if d.reason != "normal" && d.reason != "client_nav" {
			abnormal++
		}
	}
	return float64(abnormal) / float64(total)
}

// ReconnectSuccessRate returns the fraction of reconnect attempts that succeeded.
func (ch *ConnectionHealth) ReconnectSuccessRate() float64 {
	ch.mu.Lock()
	defer ch.mu.Unlock()
	cutoff := time.Now().Add(-ch.window)

	var total, success int
	for _, r := range ch.reconnects {
		if r.at.Before(cutoff) {
			continue
		}
		total++
		if r.success {
			success++
		}
	}
	if total == 0 {
		return 1.0
	}
	return float64(success) / float64(total)
}

// ReconnectP50 returns the median reconnect latency within the window.
func (ch *ConnectionHealth) ReconnectP50() time.Duration {
	return ch.reconnectPercentile(0.50)
}

// ReconnectP95 returns the 95th percentile reconnect latency.
func (ch *ConnectionHealth) ReconnectP95() time.Duration {
	return ch.reconnectPercentile(0.95)
}

func (ch *ConnectionHealth) reconnectPercentile(p float64) time.Duration {
	ch.mu.Lock()
	defer ch.mu.Unlock()
	cutoff := time.Now().Add(-ch.window)

	var durations []time.Duration
	for _, l := range ch.latencies {
		if l.at.Before(cutoff) {
			continue
		}
		durations = append(durations, l.latency)
	}
	if len(durations) == 0 {
		return 0
	}

	sort.Slice(durations, func(i, j int) bool { return durations[i] < durations[j] })
	idx := int(float64(len(durations)-1) * p)
	return durations[idx]
}

// Prune removes records older than 2x the window to bound memory.
func (ch *ConnectionHealth) Prune() {
	ch.mu.Lock()
	defer ch.mu.Unlock()
	cutoff := time.Now().Add(-2 * ch.window)

	pruned := ch.connects[:0]
	for _, c := range ch.connects {
		if !c.Before(cutoff) {
			pruned = append(pruned, c)
		}
	}
	ch.connects = pruned

	prunedD := ch.disconnects[:0]
	for _, d := range ch.disconnects {
		if !d.at.Before(cutoff) {
			prunedD = append(prunedD, d)
		}
	}
	ch.disconnects = prunedD

	prunedR := ch.reconnects[:0]
	for _, r := range ch.reconnects {
		if !r.at.Before(cutoff) {
			prunedR = append(prunedR, r)
		}
	}
	ch.reconnects = prunedR

	prunedL := ch.latencies[:0]
	for _, l := range ch.latencies {
		if !l.at.Before(cutoff) {
			prunedL = append(prunedL, l)
		}
	}
	ch.latencies = prunedL
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd /home/rishav/soul-v2 && go test ./internal/chat/metrics/ -run TestConnectionHealth -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/chat/metrics/connection_health.go internal/chat/metrics/connection_health_test.go
git commit -m "feat: add sliding-window ConnectionHealth aggregator for live pillar scoring"
```

---

### Task 7: Wire ConnectionHealth into Hub

**Files:**
- Modify: `internal/chat/ws/hub.go`

- [ ] **Step 1: Add connHealth field to Hub struct**

In `hub.go`, add to the `Hub` struct (after line 39):

```go
connHealth *metrics.ConnectionHealth
```

- [ ] **Step 2: Add HubOption and getter**

```go
// WithConnectionHealth sets the connection health tracker.
func WithConnectionHealth(ch *metrics.ConnectionHealth) HubOption {
	return func(h *Hub) { h.connHealth = ch }
}

// ConnectionHealth returns the hub's connection health tracker.
func (h *Hub) ConnectionHealth() *metrics.ConnectionHealth {
	return h.connHealth
}
```

- [ ] **Step 3: Record events in the Run() event loop**

In the `Run()` method's event loop, in the register case:

```go
case client := <-h.register:
	h.clients[client] = true
	if h.connHealth != nil {
		h.connHealth.RecordConnect()
	}
	// ... existing register logic ...
```

In the unregister case:

```go
case client := <-h.unregister:
	if _, ok := h.clients[client]; ok {
		delete(h.clients, client)
		client.closeSend()
		if h.connHealth != nil {
			h.connHealth.RecordDisconnect(classifyCloseCode(client.closeCode))
		}
	}
```

- [ ] **Step 3b: Emit ws.reconnect.success/fail events from session.resume handler**

The server detects reconnects when a client sends `session.resume` (Task 17, Step 3). In the `handleSessionResume` method, emit the reconnect events:

```go
func (h *MessageHandler) handleSessionResume(client *Client, msg *InboundMessage) {
	sessionID := msg.SessionID
	lastMsgID := msg.LastMessageID
	if sessionID == "" || lastMsgID == "" {
		return
	}

	// Attempt replay — Replay returns (msgs, found) where found indicates
	// whether the session+anchor existed (found=true with 0 msgs = caught up)
	var replayed int
	replayOK := false
	if h.hub.replay != nil {
		msgs, found := h.hub.replay.Replay(sessionID, lastMsgID)
		replayOK = found
		replayed = len(msgs)
		for _, m := range msgs {
			client.Send(m)
		}
	}

	// Emit reconnect success or fail depending on whether replay buffer had the session
	if h.metrics != nil {
		if replayOK {
			_ = h.metrics.Log(metrics.EventWSReconnectSuccess, map[string]interface{}{
				"session_id": sessionID,
				"client_id":  client.ID(),
				"replayed":   replayed,
			})
		} else {
			// No replay buffer or session not found — reconnect succeeded but state was lost
			_ = h.metrics.Log(metrics.EventWSReconnectFail, map[string]interface{}{
				"session_id": sessionID,
				"client_id":  client.ID(),
				"reason":     "replay_buffer_miss",
			})
		}
	}
	if h.hub.connHealth != nil {
		h.hub.connHealth.RecordReconnectAttempt(replayOK)
	}

	if h.metrics != nil && replayed > 0 {
		_ = h.metrics.Log(metrics.EventWSStreamResume, map[string]interface{}{
			"session_id":  sessionID,
			"last_msg_id": lastMsgID,
			"replayed":    replayed,
		})
	}
}
```

This ensures both `ws.reconnect.success` AND `ws.reconnect.fail` events exist in JSONL for `ConnectionHealthReport` to count.

- [ ] **Step 4: Initialize ConnectionHealth where Hub is created**

Find where `NewHub` is called (in `cmd/chat/main.go` or `server.go`) and add:

```go
ws.WithConnectionHealth(metrics.NewConnectionHealth(1 * time.Hour))
```

- [ ] **Step 5: Verify build compiles**

Run: `cd /home/rishav/soul-v2 && go build ./cmd/chat/`
Expected: Success

- [ ] **Step 6: Commit**

```bash
git add internal/chat/ws/hub.go cmd/chat/main.go
git commit -m "feat: wire ConnectionHealth into Hub for live connection tracking"
```

---

## Chunk 4: Live Pillar Scoring via JSONL

The observe server is a separate process on port 3010. It reads metrics JSONL files via `metrics.Aggregator`. To get connection health data, we add new methods to `Aggregator` that count `ws.close`, `auth.fail`, `ws.reconnect.*` events from the JSONL files. This keeps the observe server stateless and file-based.

### Task 8: Add Connection Health Methods to Aggregator

**Files:**
- Modify: `internal/chat/metrics/aggregator.go`
- Test: `internal/chat/metrics/aggregator_test.go`

- [ ] **Step 1: Write tests for new Aggregator methods**

```go
// Add to internal/chat/metrics/aggregator_test.go
func TestAggregator_ConnectionHealth(t *testing.T) {
	dir := t.TempDir()

	// Write test metrics JSONL — format: "ts" (RFC3339Nano) + "event" (not "timestamp"/"event_type")
	f, _ := os.Create(filepath.Join(dir, "metrics.jsonl"))
	lines := []string{
		`{"ts":"2026-03-16T10:00:00Z","event":"ws.close","data":{"reason_class":"normal"}}`,
		`{"ts":"2026-03-16T10:00:01Z","event":"ws.close","data":{"reason_class":"network"}}`,
		`{"ts":"2026-03-16T10:00:02Z","event":"ws.close","data":{"reason_class":"normal"}}`,
		`{"ts":"2026-03-16T10:00:03Z","event":"ws.connect","data":{}}`,
		`{"ts":"2026-03-16T10:00:03.001Z","event":"ws.connect","data":{}}`,
		`{"ts":"2026-03-16T10:00:03.002Z","event":"ws.connect","data":{}}`,
		`{"ts":"2026-03-16T10:00:04Z","event":"auth.fail","data":{"source":"api"}}`,
		`{"ts":"2026-03-16T10:00:05Z","event":"auth.ok","data":{"source":"api"}}`,
		`{"ts":"2026-03-16T10:00:06Z","event":"ws.reconnect.success","data":{}}`,
		`{"ts":"2026-03-16T10:00:07Z","event":"ws.reconnect.fail","data":{}}`,
	}
	for _, l := range lines {
		f.WriteString(l + "\n")
	}
	f.Close()

	agg := NewAggregator(dir)

	report, err := agg.ConnectionHealthReport()
	if err != nil {
		t.Fatal(err)
	}

	if report.TotalConnects != 3 {
		t.Errorf("expected 3 connects, got %d", report.TotalConnects)
	}
	if report.AbnormalDisconnects != 1 {
		t.Errorf("expected 1 abnormal disconnect, got %d", report.AbnormalDisconnects)
	}
	if report.AuthFailures != 1 {
		t.Errorf("expected 1 auth failure, got %d", report.AuthFailures)
	}
	if report.ReconnectSuccesses != 1 {
		t.Errorf("expected 1 reconnect success, got %d", report.ReconnectSuccesses)
	}
	if report.ReconnectFailures != 1 {
		t.Errorf("expected 1 reconnect failure, got %d", report.ReconnectFailures)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /home/rishav/soul-v2 && go test ./internal/chat/metrics/ -run TestAggregator_ConnectionHealth -v`
Expected: FAIL — method not defined

- [ ] **Step 3: Add ConnectionHealthReport to Aggregator**

Add to `internal/chat/metrics/aggregator.go`:

```go
// ConnectionHealthReport contains connection reliability metrics derived from JSONL events.
type ConnectionHealthReport struct {
	TotalConnects        int
	TotalDisconnects     int
	AbnormalDisconnects  int
	AuthFailures         int
	AuthSuccesses        int
	ReconnectSuccesses   int
	ReconnectFailures    int
}

// DropRate returns the fraction of abnormal disconnects vs total connects.
func (r *ConnectionHealthReport) DropRate() float64 {
	if r.TotalConnects == 0 {
		return 0
	}
	return float64(r.AbnormalDisconnects) / float64(r.TotalConnects)
}

// ReconnectSuccessRate returns the fraction of successful reconnects.
func (r *ConnectionHealthReport) ReconnectSuccessRate() float64 {
	total := r.ReconnectSuccesses + r.ReconnectFailures
	if total == 0 {
		return 1.0
	}
	return float64(r.ReconnectSuccesses) / float64(total)
}

// ConnectionHealthReport reads JSONL metrics and computes connection health.
func (a *Aggregator) ConnectionHealthReport() (*ConnectionHealthReport, error) {
	events, err := a.readProductEvents("") // empty prefix = all event types
	if err != nil {
		return nil, err
	}

	report := &ConnectionHealthReport{}
	for _, ev := range events {
		switch ev.EventType {
		case EventWSConnect:
			report.TotalConnects++
		case EventWSClose:
			report.TotalDisconnects++
			if reason, ok := ev.Data["reason_class"].(string); ok {
				if reason != "normal" && reason != "client_nav" {
					report.AbnormalDisconnects++
				}
			}
		case EventAuthFail:
			report.AuthFailures++
		case EventAuthOK:
			report.AuthSuccesses++
		case EventWSReconnectSuccess:
			report.ReconnectSuccesses++
		case EventWSReconnectFail:
			report.ReconnectFailures++
		}
	}
	return report, nil
}
```

Note: Use the existing `a.readProductEvents("")` method (empty prefix = all events). Do NOT call `a.readEvents()` which doesn't exist.

- [ ] **Step 4: Run test to verify it passes**

Run: `cd /home/rishav/soul-v2 && go test ./internal/chat/metrics/ -run TestAggregator_ConnectionHealth -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/chat/metrics/aggregator.go internal/chat/metrics/aggregator_test.go
git commit -m "feat: add ConnectionHealthReport to Aggregator for JSONL-based connection health"
```

---

### Task 9: Replace Static Resilient Pillar with Live Constraints

**Files:**
- Modify: `internal/observe/server/handlers.go:208-229` (handlePillars) and `340-349` (buildResilientPillar)
- Test: `internal/observe/server/handlers_test.go`

- [ ] **Step 1: Write test for live resilient pillar**

```go
func TestBuildResilientPillar_LiveConstraints(t *testing.T) {
	report := &metrics.ConnectionHealthReport{
		TotalConnects:       100,
		TotalDisconnects:    100,
		AbnormalDisconnects: 0,
		ReconnectSuccesses:  5,
		ReconnectFailures:   0,
	}

	result := buildResilientPillar(report)

	staticCount := 0
	for _, c := range result.Constraints {
		if c.Status == "static" {
			staticCount++
		}
	}
	if staticCount == len(result.Constraints) {
		t.Error("all constraints are static — expected live constraints")
	}

	for _, c := range result.Constraints {
		if c.Name == "chat-drop-rate" && c.Status != "pass" {
			t.Errorf("expected chat-drop-rate to pass, got %s", c.Status)
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /home/rishav/soul-v2 && go test ./internal/observe/server/ -run TestBuildResilientPillar_LiveConstraints -v`
Expected: FAIL — wrong signature

- [ ] **Step 3: Replace buildResilientPillar**

In `handlers.go`, replace `buildResilientPillar()` (lines 340-349):

```go
func buildResilientPillar(ch *metrics.ConnectionHealthReport) pillarResult {
	p := pillarResult{Name: "resilient"}

	// Live: chat drop rate < 0.5%
	dropRate := 0.0
	dropValue := "no data"
	if ch != nil && ch.TotalConnects > 0 {
		dropRate = ch.DropRate()
		dropValue = strconv.FormatFloat(dropRate*100, 'f', 3, 64) + "%"
	}
	dropStatus := "pass"
	if dropRate > 0.005 {
		dropStatus = "fail"
	} else if dropRate > 0.002 {
		dropStatus = "warn"
	}
	p.Constraints = append(p.Constraints, pillarConstraint{
		Name:        "chat-drop-rate",
		Target:      "< 0.5% sessions/hour",
		Enforcement: "runtime metric",
		Status:      dropStatus,
		Value:       dropValue,
	})

	// Live: reconnect success rate > 95%
	reconnectRate := 1.0
	reconnectValue := "no data"
	if ch != nil && (ch.ReconnectSuccesses+ch.ReconnectFailures) > 0 {
		reconnectRate = ch.ReconnectSuccessRate()
		reconnectValue = strconv.FormatFloat(reconnectRate*100, 'f', 1, 64) + "%"
	}
	rateStatus := "pass"
	if reconnectRate < 0.95 {
		rateStatus = "fail"
	} else if reconnectRate < 0.98 {
		rateStatus = "warn"
	}
	p.Constraints = append(p.Constraints, pillarConstraint{
		Name:        "reconnect-success-rate",
		Target:      "> 95%",
		Enforcement: "runtime metric",
		Status:      rateStatus,
		Value:       reconnectValue,
	})

	// Static: graceful shutdown (kept)
	p.Constraints = append(p.Constraints, pillarConstraint{
		Name:        "graceful-shutdown",
		Target:      "SIGTERM handler on all servers",
		Enforcement: "enforced at build",
		Status:      "static",
		Value:       "SIGTERM handler",
	})

	countStatuses(&p)
	return p
}
```

- [ ] **Step 4: Update handlePillars to pass ConnectionHealthReport**

In `handlePillars` (around line 208-229), add:

```go
func (s *Server) handlePillars(w http.ResponseWriter, r *http.Request) {
	agg := s.aggregator(r)

	latency, _ := agg.Latency()
	db, _ := agg.DB()
	requests, _ := agg.Requests()
	frontend, _ := agg.Frontend()
	usage, _ := agg.Usage()
	connHealth, _ := agg.ConnectionHealthReport() // NEW

	pillars := []pillarResult{
		buildPerformantPillar(latency, db, requests),
		buildRobustPillar(frontend),
		buildResilientPillar(connHealth),   // CHANGED: was buildResilientPillar()
		buildSecurePillar(),
		buildSovereignPillar(),
		buildTransparentPillar(usage, connHealth), // CHANGED: added connHealth
	}
	// ... rest unchanged
}
```

- [ ] **Step 5: Run test to verify it passes**

Run: `cd /home/rishav/soul-v2 && go test ./internal/observe/server/ -run TestBuildResilientPillar_LiveConstraints -v`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/observe/server/handlers.go internal/observe/server/handlers_test.go
git commit -m "feat: replace static resilient pillar with live drop-rate and reconnect-success constraints"
```

---

### Task 10: Enhance Transparent Pillar with Connection Reliability Signals

**Files:**
- Modify: `internal/observe/server/handlers.go:374-420`
- Test: `internal/observe/server/handlers_test.go`

- [ ] **Step 1: Write test for enhanced transparent pillar**

```go
func TestBuildTransparentPillar_IncludesReliabilitySignals(t *testing.T) {
	usage := &metrics.UsageReport{
		TotalEvents: 10,
		Actions:     map[string]int{"chat.send": 5},
	}
	ch := &metrics.ConnectionHealthReport{
		TotalConnects:       5,
		AuthFailures:        1,
		AuthSuccesses:       4,
		TotalDisconnects:    5,
		AbnormalDisconnects: 0,
	}

	result := buildTransparentPillar(usage, ch)

	constraintNames := make(map[string]bool)
	for _, c := range result.Constraints {
		constraintNames[c.Name] = true
	}

	required := []string{"event-tracking", "usage-tracking", "auth-event-coverage", "ws-lifecycle-coverage"}
	for _, name := range required {
		if !constraintNames[name] {
			t.Errorf("missing required constraint: %s", name)
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /home/rishav/soul-v2 && go test ./internal/observe/server/ -run TestBuildTransparentPillar_IncludesReliabilitySignals -v`
Expected: FAIL — wrong signature

- [ ] **Step 3: Update buildTransparentPillar signature and add constraints**

Change the function signature from `buildTransparentPillar(usage *metrics.UsageReport)` to:

```go
func buildTransparentPillar(usage *metrics.UsageReport, ch *metrics.ConnectionHealthReport) pillarResult {
```

After the existing usage-tracking constraint (around line 410), add:

```go
// Auth event coverage
authEventCount := 0
authStatus := "warn"
if ch != nil {
	authEventCount = ch.AuthFailures + ch.AuthSuccesses
	if authEventCount > 0 {
		authStatus = "pass"
	}
}
p.Constraints = append(p.Constraints, pillarConstraint{
	Name:        "auth-event-coverage",
	Target:      "> 0 auth events tracked",
	Enforcement: "runtime metric",
	Status:      authStatus,
	Value:       strconv.Itoa(authEventCount) + " events",
})

// WS lifecycle coverage
wsEventCount := 0
wsStatus := "warn"
if ch != nil {
	wsEventCount = ch.TotalConnects + ch.TotalDisconnects
	if wsEventCount > 0 {
		wsStatus = "pass"
	}
}
p.Constraints = append(p.Constraints, pillarConstraint{
	Name:        "ws-lifecycle-coverage",
	Target:      "> 0 WS lifecycle events tracked",
	Enforcement: "runtime metric",
	Status:      wsStatus,
	Value:       strconv.Itoa(wsEventCount) + " events",
})
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd /home/rishav/soul-v2 && go test ./internal/observe/server/ -run TestBuildTransparentPillar_IncludesReliabilitySignals -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/observe/server/handlers.go internal/observe/server/handlers_test.go
git commit -m "feat: enhance transparent pillar with auth-event-coverage and ws-lifecycle-coverage"
```

---

## Chunk 5: Frontend Classified Observability

### Task 11: Extend TelemetryEvent Type and Add Telemetry Functions

**Files:**
- Modify: `web/src/lib/telemetry.ts`

- [ ] **Step 1: Extend the TelemetryEvent union type**

In `telemetry.ts:3`, change:

```typescript
type TelemetryEvent = 'frontend.error' | 'frontend.render' | 'frontend.ws' | 'frontend.usage';
```

To:

```typescript
type TelemetryEvent =
  | 'frontend.error'
  | 'frontend.render'
  | 'frontend.ws'
  | 'frontend.usage'
  | 'frontend.ws.disconnect'
  | 'frontend.ws.reconnect'
  | 'frontend.auth.fail';
```

- [ ] **Step 2: Add new telemetry functions**

Append to `telemetry.ts`:

```typescript
export function reportDisconnect(data: {
  closeCode: number;
  reasonClass: string;
  connectionDurationMs?: number;
}): void {
  sendTelemetry('frontend.ws.disconnect', data);
}

export function reportReconnect(data: {
  attempt: number;
  backoffMs: number;
  success: boolean;
  totalDowntimeMs?: number;
}): void {
  sendTelemetry('frontend.ws.reconnect', data);
}

export function reportAuthFailure(data: {
  source: 'ws' | 'api';
  reason: string;
}): void {
  sendTelemetry('frontend.auth.fail', data);
}
```

- [ ] **Step 3: Verify TypeScript compiles**

Run: `cd /home/rishav/soul-v2 && npx tsc --noEmit`
Expected: No errors

- [ ] **Step 4: Commit**

```bash
git add web/src/lib/telemetry.ts
git commit -m "feat: extend TelemetryEvent type and add disconnect/reconnect/authFailure telemetry"
```

---

### Task 12: Add Classified Disconnect/Reconnect Observability to useWebSocket

**Files:**
- Modify: `web/src/hooks/useWebSocket.ts`

- [ ] **Step 1: Add classifyCloseCode function**

Add at the top of `useWebSocket.ts` (after imports):

```typescript
import { reportDisconnect, reportReconnect, reportError } from '../lib/telemetry';

function classifyCloseCode(code: number): string {
  switch (code) {
    case 1000: return 'normal';
    case 1001: return 'client_nav';
    case 1006: return 'network';
    case 1008: return 'auth';
    case 1011: return 'server_error';
    case 1012:
    case 1013: return 'server_restart';
    default: return 'unknown';
  }
}
```

- [ ] **Step 2: Add timing refs to the hook**

Inside the `useWebSocket` hook, add:

```typescript
const connectTimeRef = useRef<number | null>(null);
const disconnectTimeRef = useRef<number | null>(null);
```

- [ ] **Step 3: Track connect time in connection.ready handler**

In the `connection.ready` handler (around line 73-77), add:

```typescript
connectTimeRef.current = Date.now();

// Report successful reconnect if this is a reconnection
if (attemptRef.current > 0) {
  reportReconnect({
    attempt: attemptRef.current,
    backoffMs: 0,
    success: true,
    totalDowntimeMs: disconnectTimeRef.current
      ? Date.now() - disconnectTimeRef.current
      : undefined,
  });
}
```

- [ ] **Step 4: Update socket.onclose to capture CloseEvent and emit telemetry**

The current `socket.onclose = () => { ... }` doesn't capture the event parameter. Change to:

```typescript
socket.onclose = (event: CloseEvent) => {
  if (unmountedRef.current) return;

  const reasonClass = classifyCloseCode(event.code);
  disconnectTimeRef.current = Date.now();

  reportDisconnect({
    closeCode: event.code,
    reasonClass,
    connectionDurationMs: connectTimeRef.current
      ? Date.now() - connectTimeRef.current
      : undefined,
  });

  setStatus('disconnected');
  attemptRef.current++;
  const delay = backoffDelay(attemptRef.current);

  reportReconnect({
    attempt: attemptRef.current,
    backoffMs: delay,
    success: false,
  });

  reconnectTimerRef.current = window.setTimeout(connect, delay);
};
```

- [ ] **Step 5: Verify TypeScript compiles**

Run: `cd /home/rishav/soul-v2 && npx tsc --noEmit`
Expected: No errors

- [ ] **Step 6: Commit**

```bash
git add web/src/hooks/useWebSocket.ts
git commit -m "feat: add classified disconnect/reconnect telemetry to frontend WebSocket"
```

---

### Task 13: Add Auth Failure Reporting in useChat

**Files:**
- Modify: `web/src/hooks/useChat.ts`

- [ ] **Step 1: Import reportAuthFailure**

Add to imports in `useChat.ts`:

```typescript
import { reportAuthFailure } from '../lib/telemetry';
```

- [ ] **Step 2: Enhance chat.error handler with structured auth detection**

In the `chat.error` handler (around line 395-425), find the auth detection logic. The current code checks `errorContent.toLowerCase().includes('authentication')`. Enhance it:

```typescript
case 'chat.error': {
  // ... existing code to extract errorContent ...

  const errorLower = errorContent.toLowerCase();
  const isAuth = errorLower.includes('authentication') ||
                 errorLower.includes('unauthorized') ||
                 errorLower.includes('401');
  if (isAuth) {
    setAuthError(true);
    reportAuthFailure({
      source: 'api',
      reason: errorContent,
    });
  }
  // ... rest of existing error handling ...
}
```

- [ ] **Step 3: Verify TypeScript compiles**

Run: `cd /home/rishav/soul-v2 && npx tsc --noEmit`
Expected: No errors

- [ ] **Step 4: Commit**

```bash
git add web/src/hooks/useChat.ts
git commit -m "feat: add structured auth failure detection and reporting to useChat"
```

---

## Chunk 6: Durable Send Queue & Idempotent Messages

### Task 14: Build Client-Side Durable Send Queue

**Files:**
- Create: `web/src/lib/sendQueue.ts`
- Create: `web/src/lib/sendQueue.test.ts`

- [ ] **Step 1: Write tests for SendQueue**

```typescript
// web/src/lib/sendQueue.test.ts
import { describe, it, expect, vi } from 'vitest';
import { SendQueue } from './sendQueue';

describe('SendQueue', () => {
  it('generates idempotent message IDs', () => {
    const queue = new SendQueue();
    const id1 = queue.enqueue({ type: 'chat.send', content: 'hello' });
    const id2 = queue.enqueue({ type: 'chat.send', content: 'hello' });
    expect(id1).not.toBe(id2);
    expect(id1).toMatch(/^msg-/);
  });

  it('flushes pending messages via sender', () => {
    const queue = new SendQueue();
    const sender = vi.fn();
    queue.enqueue({ type: 'chat.send', content: 'hello' });
    queue.enqueue({ type: 'chat.send', content: 'world' });
    queue.flush(sender);
    expect(sender).toHaveBeenCalledTimes(2);
  });

  it('marks messages as sent after flush', () => {
    const queue = new SendQueue();
    const sender = vi.fn();
    queue.enqueue({ type: 'chat.send', content: 'hello' });
    queue.flush(sender);
    expect(queue.pending()).toBe(0);
  });

  it('retains messages if sender throws', () => {
    const queue = new SendQueue();
    const sender = vi.fn().mockImplementation(() => { throw new Error('offline'); });
    queue.enqueue({ type: 'chat.send', content: 'hello' });
    try { queue.flush(sender); } catch { /* expected */ }
    expect(queue.pending()).toBe(1);
  });

  it('deduplicates by message ID on markSent', () => {
    const queue = new SendQueue();
    const sender = vi.fn();
    const id = queue.enqueue({ type: 'chat.send', content: 'hello' });
    queue.markSent(id);
    queue.flush(sender);
    expect(sender).not.toHaveBeenCalled();
  });
});
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /home/rishav/soul-v2 && npx vitest run web/src/lib/sendQueue.test.ts`
Expected: FAIL — module not found

- [ ] **Step 3: Implement SendQueue**

```typescript
// web/src/lib/sendQueue.ts

interface QueuedMessage {
  id: string;
  payload: Record<string, unknown>;
  enqueuedAt: number;
  sent: boolean;
}

let counter = 0;

export class SendQueue {
  private messages: QueuedMessage[] = [];
  private storageKey: string;

  constructor(storageKey = 'soul-v2-send-queue') {
    this.storageKey = storageKey;
  }

  enqueue(payload: Record<string, unknown>): string {
    const id = `msg-${Date.now()}-${++counter}`;
    this.messages.push({
      id,
      payload: { ...payload, messageId: id },
      enqueuedAt: Date.now(),
      sent: false,
    });
    return id;
  }

  flush(sender: (payload: Record<string, unknown>) => void): void {
    const pending = this.messages.filter((m) => !m.sent);
    for (const msg of pending) {
      sender(msg.payload); // throws on failure → message stays unsent
      msg.sent = true;
    }
    this.messages = this.messages.filter((m) => !m.sent);
  }

  // Note: messages include `messageId` in payload. Server-side deduplication
  // is handled in handler.go's handleChatSend — see Task 17a below.

  markSent(id: string): void {
    const msg = this.messages.find((m) => m.id === id);
    if (msg) msg.sent = true;
  }

  pending(): number {
    return this.messages.filter((m) => !m.sent).length;
  }

  persist(): void {
    try {
      const pending = this.messages.filter((m) => !m.sent);
      localStorage.setItem(this.storageKey, JSON.stringify(pending));
    } catch {
      // localStorage may be unavailable
    }
  }

  restore(): void {
    try {
      const raw = localStorage.getItem(this.storageKey);
      if (raw) {
        this.messages = JSON.parse(raw);
        localStorage.removeItem(this.storageKey);
      }
    } catch {
      // corrupted data — ignore
    }
  }

  clear(): void {
    this.messages = [];
    try { localStorage.removeItem(this.storageKey); } catch { /* ignore */ }
  }
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd /home/rishav/soul-v2 && npx vitest run web/src/lib/sendQueue.test.ts`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add web/src/lib/sendQueue.ts web/src/lib/sendQueue.test.ts
git commit -m "feat: add durable SendQueue with idempotent message IDs"
```

---

### Task 15: Integrate SendQueue into useChat

**Files:**
- Modify: `web/src/hooks/useChat.ts`

- [ ] **Step 1: Read useChat.ts to understand the send flow**

Read `web/src/hooks/useChat.ts` to find:
1. How `send` is obtained from `useWebSocket` (around line 512)
2. How `sendMessage` calls `send` (around line 518-556)
3. How `connection.ready` restores state (around line 172-197)
4. Whether `sendRef` already exists for deferred sends

- [ ] **Step 2: Import and initialize SendQueue**

```typescript
import { SendQueue } from '../lib/sendQueue';

// Inside useChat():
const sendQueueRef = useRef(new SendQueue());
```

- [ ] **Step 3: Wrap sendMessage with queue**

In `sendMessage`, instead of calling `send('chat.send', ...)` directly, enqueue first and flush if connected:

```typescript
const messageId = sendQueueRef.current.enqueue({
  type: 'chat.send',
  sessionId: sessionIDRef.current,
  content,
});

if (status === 'connected') {
  sendQueueRef.current.flush((payload) => {
    const { type, ...data } = payload;
    send(type as string, data);
  });
}
```

Note: The flush callback destructures `type` out of the payload so it's passed as the first argument to `send`, with the rest as the data argument. This matches the `useWebSocket.send(type, data)` signature.

- [ ] **Step 4: Track last received message ID for replay**

Add a ref to track the last received outbound message anchor:

```typescript
const lastMessageIdRef = useRef<string | null>(null);
```

In the `chat.token` handler (around line 333), update the anchor on each token:

```typescript
// After processing the token, track the message ID for replay
if (msg.data?.messageId) {
  lastMessageIdRef.current = msg.data.messageId;
}
```

- [ ] **Step 5: Send session.resume on reconnect and flush queue**

In the `connection.ready` handler (around line 172-197), after the existing restoration logic:

```typescript
// Request replay of any missed messages since last disconnect
if (lastMessageIdRef.current && sessionIDRef.current) {
  send('session.resume', {
    sessionId: sessionIDRef.current,
    lastMessageId: lastMessageIdRef.current,
  });
}

// Flush any queued messages from before disconnect
sendQueueRef.current.restore();
if (sendQueueRef.current.pending() > 0) {
  sendQueueRef.current.flush((payload) => {
    const { type, ...data } = payload;
    send(type as string, data);
  });
}
```

- [ ] **Step 5: Persist queue on disconnect**

Add a `useEffect` watching status:

```typescript
useEffect(() => {
  if (status === 'disconnected') {
    sendQueueRef.current.persist();
  }
}, [status]);
```

- [ ] **Step 6: Verify TypeScript compiles**

Run: `cd /home/rishav/soul-v2 && npx tsc --noEmit`
Expected: No errors

- [ ] **Step 7: Commit**

```bash
git add web/src/hooks/useChat.ts
git commit -m "feat: integrate SendQueue into useChat for zero-loss message delivery"
```

---

## Chunk 7: Server-Side Replay Window

### Task 16: Build Server-Side Message Replay Buffer

**Files:**
- Create: `internal/chat/ws/replay.go`
- Create: `internal/chat/ws/replay_test.go`

- [ ] **Step 1: Write tests for ReplayBuffer**

```go
// internal/chat/ws/replay_test.go
package ws

import "testing"

func TestReplayBuffer_StoreAndReplay(t *testing.T) {
	rb := NewReplayBuffer(100, 50)

	rb.Store("session-1", "msg-1", []byte(`{"type":"chat.token","text":"hello"}`))
	rb.Store("session-1", "msg-2", []byte(`{"type":"chat.token","text":"world"}`))

	msgs, found := rb.Replay("session-1", "msg-1")
	if !found {
		t.Fatal("expected found=true")
	}
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message after msg-1, got %d", len(msgs))
	}
	if string(msgs[0]) != `{"type":"chat.token","text":"world"}` {
		t.Errorf("unexpected: %s", msgs[0])
	}
}

func TestReplayBuffer_EmptyReplay(t *testing.T) {
	rb := NewReplayBuffer(100, 50)
	msgs, found := rb.Replay("session-1", "msg-nonexistent")
	if found {
		t.Fatal("expected found=false for unknown session")
	}
	if len(msgs) != 0 {
		t.Fatalf("expected 0 for unknown ID, got %d", len(msgs))
	}
}

func TestReplayBuffer_CaughtUp(t *testing.T) {
	rb := NewReplayBuffer(100, 50)
	rb.Store("s1", "m1", []byte("1"))

	// Replay from the latest message — found but nothing new
	msgs, found := rb.Replay("s1", "m1")
	if !found {
		t.Fatal("expected found=true (anchor exists)")
	}
	if len(msgs) != 0 {
		t.Fatalf("expected 0 msgs (caught up), got %d", len(msgs))
	}
}

func TestReplayBuffer_CapacityEviction(t *testing.T) {
	rb := NewReplayBuffer(3, 2)

	rb.Store("s1", "m1", []byte("1"))
	rb.Store("s1", "m2", []byte("2"))
	rb.Store("s1", "m3", []byte("3"))
	rb.Store("s1", "m4", []byte("4")) // evicts m1

	// m1 anchor evicted — found=false
	msgs, found := rb.Replay("s1", "m1")
	if found {
		t.Fatal("expected found=false (anchor evicted)")
	}
	if len(msgs) != 0 {
		t.Fatalf("expected 0 (anchor evicted), got %d", len(msgs))
	}

	// m2 still exists, should return m3, m4
	msgs, found = rb.Replay("s1", "m2")
	if !found {
		t.Fatal("expected found=true for m2")
	}
	if len(msgs) != 2 {
		t.Fatalf("expected 2 after m2, got %d", len(msgs))
	}
}

func TestReplayBuffer_SessionEviction(t *testing.T) {
	rb := NewReplayBuffer(10, 2)

	rb.Store("s1", "m1", []byte("1"))
	rb.Store("s2", "m1", []byte("2"))
	rb.Store("s3", "m1", []byte("3")) // evicts s1

	// s1 should be evicted — found=false
	_, found := rb.Replay("s1", "m1")
	if found {
		t.Fatal("expected found=false (session evicted)")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /home/rishav/soul-v2 && go test ./internal/chat/ws/ -run TestReplayBuffer -v`
Expected: FAIL — type not defined

- [ ] **Step 3: Implement ReplayBuffer**

```go
// internal/chat/ws/replay.go
package ws

import "sync"

type replayEntry struct {
	messageID string
	data      []byte
}

// ReplayBuffer maintains a bounded per-session message buffer for replay after reconnect.
type ReplayBuffer struct {
	mu            sync.Mutex
	maxPerSession int
	maxSessions   int
	sessions      map[string][]replayEntry
	sessionOrder  []string // LRU order
}

func NewReplayBuffer(maxPerSession, maxSessions int) *ReplayBuffer {
	return &ReplayBuffer{
		maxPerSession: maxPerSession,
		maxSessions:   maxSessions,
		sessions:      make(map[string][]replayEntry),
	}
}

func (rb *ReplayBuffer) Store(sessionID, messageID string, data []byte) {
	rb.mu.Lock()
	defer rb.mu.Unlock()

	if _, ok := rb.sessions[sessionID]; !ok {
		if len(rb.sessionOrder) >= rb.maxSessions {
			oldest := rb.sessionOrder[0]
			rb.sessionOrder = rb.sessionOrder[1:]
			delete(rb.sessions, oldest)
		}
		rb.sessionOrder = append(rb.sessionOrder, sessionID)
		rb.sessions[sessionID] = nil
	}

	entries := rb.sessions[sessionID]
	entries = append(entries, replayEntry{messageID: messageID, data: data})
	if len(entries) > rb.maxPerSession {
		entries = entries[len(entries)-rb.maxPerSession:]
	}
	rb.sessions[sessionID] = entries

	// Move to end of LRU
	for i, s := range rb.sessionOrder {
		if s == sessionID {
			rb.sessionOrder = append(rb.sessionOrder[:i], rb.sessionOrder[i+1:]...)
			rb.sessionOrder = append(rb.sessionOrder, sessionID)
			break
		}
	}
}

// Replay returns all messages after the anchor, plus a bool indicating whether
// the session+anchor was found. (found=true, msgs=nil) means "caught up, nothing missed".
// (found=false) means the session was evicted or the anchor expired.
func (rb *ReplayBuffer) Replay(sessionID, afterMessageID string) ([][]byte, bool) {
	rb.mu.Lock()
	defer rb.mu.Unlock()

	entries, ok := rb.sessions[sessionID]
	if !ok || afterMessageID == "" {
		return nil, false
	}

	anchorIdx := -1
	for i, e := range entries {
		if e.messageID == afterMessageID {
			anchorIdx = i
			break
		}
	}
	if anchorIdx == -1 {
		return nil, false // anchor evicted
	}

	var result [][]byte
	for i := anchorIdx + 1; i < len(entries); i++ {
		cp := make([]byte, len(entries[i].data))
		copy(cp, entries[i].data)
		result = append(result, cp)
	}
	return result, true
}

func (rb *ReplayBuffer) Clear(sessionID string) {
	rb.mu.Lock()
	defer rb.mu.Unlock()
	delete(rb.sessions, sessionID)
	for i, s := range rb.sessionOrder {
		if s == sessionID {
			rb.sessionOrder = append(rb.sessionOrder[:i], rb.sessionOrder[i+1:]...)
			break
		}
	}
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd /home/rishav/soul-v2 && go test ./internal/chat/ws/ -run TestReplayBuffer -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/chat/ws/replay.go internal/chat/ws/replay_test.go
git commit -m "feat: add bounded per-session ReplayBuffer for stream resume"
```

---

### Task 17: Wire ReplayBuffer into Hub and Handler

**Files:**
- Modify: `internal/chat/ws/hub.go`
- Modify: `internal/chat/ws/handler.go`
- Modify: `internal/chat/ws/message.go`

- [ ] **Step 1: Add replay field to Hub**

In `hub.go`, add to the `Hub` struct:

```go
replay *ReplayBuffer
```

Add option and getter:

```go
func WithReplayBuffer(rb *ReplayBuffer) HubOption {
	return func(h *Hub) { h.replay = rb }
}

func (h *Hub) ReplayBuffer() *ReplayBuffer {
	return h.replay
}
```

- [ ] **Step 2: Add session.resume message type constant**

In `message.go`, add the constant to the inbound message types:

```go
TypeSessionResume = "session.resume"
```

Note: The `LastMessageID` field on `InboundMessage` was already added in Task 17a Step 1 (dedupe task).

- [ ] **Step 3: Add session.resume handler in handler.go**

In `HandleMessage`'s switch statement (around line 91-119), add a new case:

```go
case TypeSessionResume:
	h.handleSessionResume(client, msg)
```

Add the handler method:

```go
func (h *MessageHandler) handleSessionResume(client *Client, msg *InboundMessage) {
	sessionID := msg.SessionID
	lastMsgID := msg.LastMessageID
	if sessionID == "" || lastMsgID == "" || h.hub.replay == nil {
		return
	}

	msgs := h.hub.replay.Replay(sessionID, lastMsgID)
	for _, m := range msgs {
		client.Send(m)
	}

	if h.metrics != nil {
		_ = h.metrics.Log(metrics.EventWSStreamResume, map[string]interface{}{
			"session_id":  sessionID,
			"last_msg_id": lastMsgID,
			"replayed":    len(msgs),
		})
	}
}
```

- [ ] **Step 4: Store outbound messages in replay buffer during streaming**

The replay anchor must be the SAME value in both the replay buffer key and the serialized `messageId` field the client receives. The flow is:

1. Generate anchor ID
2. Set it on the `OutboundMessage.MessageID` field
3. Marshal (now includes `messageId` in JSON)
4. Send to client (client tracks `lastMessageIdRef`)
5. Store in replay buffer keyed by the SAME anchor ID

Modify `sendToClient` in `handler.go:1128`:

```go
func (h *MessageHandler) sendToClient(client *Client, msg *OutboundMessage) {
	// Assign replay anchor BEFORE marshaling so it's included in the JSON the client receives
	if h.hub.replay != nil && msg.SessionID != "" && msg.MessageID == "" {
		msg.MessageID = msg.SessionID + "-" + strconv.FormatInt(time.Now().UnixNano(), 36)
	}

	data, err := MarshalOutbound(msg)
	if err != nil {
		log.Printf("ws: failed to marshal outbound message: %v", err)
		return
	}
	client.Send(data)

	// Store in replay buffer using the same ID that was just serialized to the client
	if h.hub.replay != nil && msg.SessionID != "" && msg.MessageID != "" {
		h.hub.replay.Store(msg.SessionID, msg.MessageID, data)
	}
}
```

Add `MessageID` to `OutboundMessage` in `message.go`:

```go
type OutboundMessage struct {
	Type      string      `json:"type"`
	SessionID string      `json:"sessionId,omitempty"`
	MessageID string      `json:"messageId,omitempty"` // replay anchor for session.resume
	Data      interface{} `json:"data,omitempty"`
}
```

This guarantees anchor consistency: the ID in the replay buffer key === the `messageId` in the JSON the client received and stored in `lastMessageIdRef`.

- [ ] **Step 5: Initialize ReplayBuffer where Hub is created**

In `cmd/chat/main.go` or wherever `NewHub` is called:

```go
ws.WithReplayBuffer(ws.NewReplayBuffer(200, 100))
```

- [ ] **Step 6: Verify build compiles**

Run: `cd /home/rishav/soul-v2 && go build ./cmd/chat/`
Expected: Success

- [ ] **Step 7: Commit**

```bash
git add internal/chat/ws/hub.go internal/chat/ws/handler.go internal/chat/ws/message.go cmd/chat/main.go
git commit -m "feat: wire ReplayBuffer into handler with session.resume support"
```

---

### Task 17a: Add Server-Side Message Deduplication

**Files:**
- Modify: `internal/chat/ws/handler.go`

The client send queue includes a `messageId` in every `chat.send` payload. The server must deduplicate so that messages replayed after reconnect aren't processed twice.

- [ ] **Step 1: Add MessageID field to InboundMessage**

In `message.go`, add to the `InboundMessage` struct (after the existing fields):

```go
type InboundMessage struct {
	Type        string          `json:"type"`
	SessionID   string          `json:"sessionId,omitempty"`
	Content     string          `json:"content,omitempty"`
	Model       string          `json:"model,omitempty"`
	Attachments []Attachment    `json:"attachments,omitempty"`
	Product     string          `json:"product,omitempty"`
	Thinking    *ThinkingConfig `json:"thinking,omitempty"`
	MessageID   string          `json:"messageId,omitempty"`   // idempotency key from client send queue
	LastMessageID string        `json:"lastMessageId,omitempty"` // for session.resume replay anchor
}
```

Note: This also adds `LastMessageID` needed by Task 17's `session.resume` handler, avoiding a separate step.

- [ ] **Step 2: Add a dedup cache to MessageHandler**

```go
// In handler.go, add to MessageHandler struct:
seenMessages map[string]time.Time // messageId → first seen time
seenMu       sync.Mutex
```

Initialize in the constructor:

```go
seenMessages: make(map[string]time.Time),
```

- [ ] **Step 3: Check messageId in handleChatSend**

At the top of `handleChatSend`, before any processing. Uses `msg.MessageID` (the typed struct field), not a map lookup:

```go
if msg.MessageID != "" {
	h.seenMu.Lock()
	if _, seen := h.seenMessages[msg.MessageID]; seen {
		h.seenMu.Unlock()
		log.Printf("ws: dedup — skipping already-seen message %s", msg.MessageID)
		return
	}
	h.seenMessages[msg.MessageID] = time.Now()
	h.seenMu.Unlock()
}
```

- [ ] **Step 3: Add periodic cleanup of old entries**

In a background goroutine (started in Hub.Run or handler init), prune entries older than 5 minutes:

```go
// Every 60 seconds, remove entries older than 5 minutes
go func() {
	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()
	for range ticker.C {
		h.seenMu.Lock()
		cutoff := time.Now().Add(-5 * time.Minute)
		for id, ts := range h.seenMessages {
			if ts.Before(cutoff) {
				delete(h.seenMessages, id)
			}
		}
		h.seenMu.Unlock()
	}
}()
```

- [ ] **Step 4: Verify build compiles**

Run: `cd /home/rishav/soul-v2 && go build ./cmd/chat/`
Expected: Success

- [ ] **Step 5: Commit**

```bash
git add internal/chat/ws/handler.go
git commit -m "feat: add server-side message deduplication for idempotent send queue"
```

---

## Chunk 8: Performance Pillar Hardening

### Task 18: Add Product/Model Dimensions to Stream Events

**Files:**
- Modify: `internal/chat/ws/handler.go`

- [ ] **Step 1: Read the current stream event emission code**

Read `handler.go` around the `EventWSStreamStart` and `EventWSStreamEnd` log calls to understand what's already being logged and what variables are available in scope.

- [ ] **Step 2: Add product and model to stream start/end events**

Where `EventWSStreamStart` is logged, add `product` and `model` fields:

```go
_ = h.metrics.Log(metrics.EventWSStreamStart, map[string]interface{}{
	"session_id": sessionID,
	"client_id":  client.ID(),  // use exported method
	"product":    product,
	"model":      model,
})
```

Where `EventWSStreamEnd` is logged, add:

```go
_ = h.metrics.Log(metrics.EventWSStreamEnd, map[string]interface{}{
	"session_id":     sessionID,
	"client_id":      client.ID(),
	"product":        product,
	"model":          model,
	"first_token_ms": firstTokenMs,
	"total_ms":       totalMs,
	"input_tokens":   inputTokens,
	"output_tokens":  outputTokens,
})
```

Note: Use `client.ID()` (exported getter), not `client.id` (unexported). Check that `product` and `model` variables are in scope in `runStream`. If `product` isn't available, thread it from `handleChatSend` into `runStream` as a parameter.

- [ ] **Step 3: Verify build compiles**

Run: `cd /home/rishav/soul-v2 && go build ./cmd/chat/`
Expected: Success

- [ ] **Step 4: Commit**

```bash
git add internal/chat/ws/handler.go
git commit -m "feat: add product/model dimensions to stream latency events"
```

---

### Task 19: Add Stream Total P95 to Performant Pillar

**Files:**
- Modify: `internal/chat/metrics/aggregator.go`
- Modify: `internal/observe/server/handlers.go`
- Test: `internal/observe/server/handlers_test.go`

Note: `first-token-p50` already exists in `buildPerformantPillar` (line 234-252). Only add `stream-total-p95`.

- [ ] **Step 1: Add StreamTotalP95 method to Aggregator**

Read the existing `Latency()` method in `aggregator.go` to understand how it reads events. Add a new method that extracts `total_ms` from `ws.stream.end` events and computes P95:

```go
// StreamTotalP95 reads ws.stream.end events and returns the P95 total stream duration in ms.
func (a *Aggregator) StreamTotalP95() (float64, error) {
	events, err := a.readProductEvents("ws.stream.")
	if err != nil {
		return 0, err
	}

	var durations []float64
	for _, ev := range events {
		if ev.EventType == EventWSStreamEnd {
			if totalMs, ok := ev.Data["total_ms"].(float64); ok {
				durations = append(durations, totalMs)
			}
		}
	}
	if len(durations) == 0 {
		return 0, nil
	}

	sort.Float64s(durations)
	idx := int(float64(len(durations)-1) * 0.95)
	return durations[idx], nil
}
```

- [ ] **Step 2: Add stream-total-p95 constraint to buildPerformantPillar**

In `handlers.go`, add after the existing HTTP request P50 constraint in `buildPerformantPillar`:

Note: The function signature is `buildPerformantPillar(latency *metrics.LatencyReport, db *metrics.DBReport, requests *metrics.RequestsReport)`. We need to add the aggregator as a parameter. Change the signature to:

```go
func buildPerformantPillar(latency *metrics.LatencyReport, db *metrics.DBReport, requests *metrics.RequestsReport, agg *metrics.Aggregator) pillarResult {
```

Then add the constraint:

```go
// Stream total P95 < 30s
stP95, _ := agg.StreamTotalP95()
stStatus := "pass"
stValue := "no data"
if stP95 > 0 {
	stValue = strconv.FormatFloat(stP95, 'f', 0, 64) + "ms"
	if stP95 > 30000 {
		stStatus = "fail"
	} else if stP95 > 20000 {
		stStatus = "warn"
	}
}
p.Constraints = append(p.Constraints, pillarConstraint{
	Name:        "stream-total-p95",
	Target:      "< 30s",
	Enforcement: "runtime metric",
	Status:      stStatus,
	Value:       stValue,
})
```

Update the call site in `handlePillars`:

```go
buildPerformantPillar(latency, db, requests, agg)
```

- [ ] **Step 3: Verify build compiles**

Run: `cd /home/rishav/soul-v2 && go build ./cmd/observe/`
Expected: Success

- [ ] **Step 4: Commit**

```bash
git add internal/chat/metrics/aggregator.go internal/observe/server/handlers.go
git commit -m "feat: add stream-total-p95 constraint to performant pillar"
```

---

## Chunk 9: Integration & Chaos Tests

### Task 20: Build Resilience Integration Tests

**Files:**
- Create: `tests/integration/resilience_test.go`

Note: Use `nhooyr.io/websocket` (already in go.mod), NOT gorilla/websocket.

- [ ] **Step 1: Create test helpers**

```go
// tests/integration/resilience_test.go
package integration

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"testing"
	"time"

	"nhooyr.io/websocket"
	"nhooyr.io/websocket/wsjson"
)

// connectWS connects to the test server's WebSocket endpoint.
func connectWS(t *testing.T, url, token string) *websocket.Conn {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	conn, _, err := websocket.Dial(ctx, url+"?token="+token, nil)
	if err != nil {
		t.Fatalf("ws connect: %v", err)
	}
	return conn
}

// readUntilType reads WS messages until one with the given type is found.
func readUntilType(t *testing.T, conn *websocket.Conn, msgType string, timeout time.Duration) map[string]interface{} {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	for {
		var msg map[string]interface{}
		if err := wsjson.Read(ctx, conn, &msg); err != nil {
			t.Fatalf("reading for %s: %v", msgType, err)
		}
		if msg["type"] == msgType {
			return msg
		}
	}
}

// sendJSON sends a JSON message over WebSocket.
func sendJSON(t *testing.T, conn *websocket.Conn, msg interface{}) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := wsjson.Write(ctx, conn, msg); err != nil {
		t.Fatalf("ws write: %v", err)
	}
}
```

- [ ] **Step 2: Write mid-stream disconnect test**

```go
func TestResilience_MidStreamDisconnect(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}

	// Assumes server is running at localhost:3002 with token from env
	baseURL := "ws://localhost:3002/ws"
	token := os.Getenv("SOUL_V2_TOKEN")
	if token == "" {
		t.Skip("SOUL_V2_TOKEN not set")
	}

	conn := connectWS(t, baseURL, token)
	defer conn.Close(websocket.StatusNormalClosure, "")

	readUntilType(t, conn, "connection.ready", 5*time.Second)

	sendJSON(t, conn, map[string]string{"type": "session.create"})
	created := readUntilType(t, conn, "session.created", 5*time.Second)
	// session.created nests session under data.session (see message.go:195-202)
	createdData := created["data"].(map[string]interface{})
	session := createdData["session"].(map[string]interface{})
	sessionID := session["id"].(string)

	sendJSON(t, conn, map[string]interface{}{
		"type":      "chat.send",
		"sessionId": sessionID,
		"content":   "Count from 1 to 10",
	})

	readUntilType(t, conn, "chat.token", 10*time.Second)

	// Abruptly close mid-stream
	conn.Close(websocket.StatusGoingAway, "test disconnect")

	// Reconnect
	conn2 := connectWS(t, baseURL, token)
	defer conn2.Close(websocket.StatusNormalClosure, "")

	readUntilType(t, conn2, "connection.ready", 5*time.Second)

	sendJSON(t, conn2, map[string]interface{}{
		"type":      "session.switch",
		"sessionId": sessionID,
	})

	history := readUntilType(t, conn2, "session.history", 5*time.Second)
	// session.history nests messages under data.messages (see message.go:225-233)
	historyData := history["data"].(map[string]interface{})
	messages := historyData["messages"].([]interface{})
	if len(messages) == 0 {
		t.Error("expected messages after mid-stream disconnect")
	}
}
```

- [ ] **Step 3: Write disconnect storm test**

```go
func TestResilience_DisconnectStorm(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}

	baseURL := "ws://localhost:3002/ws"
	token := os.Getenv("SOUL_V2_TOKEN")
	if token == "" {
		t.Skip("SOUL_V2_TOKEN not set")
	}

	// Rapid connect/disconnect 20 times
	for i := 0; i < 20; i++ {
		conn := connectWS(t, baseURL, token)
		conn.Close(websocket.StatusNormalClosure, "")
	}

	// Server should still accept connections
	conn := connectWS(t, baseURL, token)
	defer conn.Close(websocket.StatusNormalClosure, "")
	readUntilType(t, conn, "connection.ready", 5*time.Second)
}
```

- [ ] **Step 4: Write auth failure test**

```go
func TestResilience_AuthFailure(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, resp, err := websocket.Dial(ctx, "ws://localhost:3002/ws?token=invalid-token", nil)
	if err == nil {
		t.Fatal("expected connection to fail with invalid token")
	}
	// nhooyr.io/websocket returns the HTTP response on failure
	if resp != nil && resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", resp.StatusCode)
	}
}
```

- [ ] **Step 5: Commit**

```bash
git add tests/integration/resilience_test.go
git commit -m "test: add resilience integration tests — mid-stream disconnect, storm, auth failure"
```

---

### Task 21: Add SLO Verification Gate

**Files:**
- Create: `tests/integration/slo_test.go`

- [ ] **Step 1: Write SLO threshold test**

```go
// tests/integration/slo_test.go
package integration

import (
	"encoding/json"
	"net/http"
	"os"
	"sort"
	"testing"
	"time"

	"nhooyr.io/websocket"
)

func TestSLO_ChatDropRate(t *testing.T) {
	if testing.Short() {
		t.Skip("SLO test")
	}

	token := os.Getenv("SOUL_V2_TOKEN")
	if token == "" {
		t.Skip("SOUL_V2_TOKEN not set")
	}

	// Simulate 50 normal sessions
	for i := 0; i < 50; i++ {
		conn := connectWS(t, "ws://localhost:3002/ws", token)
		readUntilType(t, conn, "connection.ready", 5*time.Second)
		conn.Close(websocket.StatusNormalClosure, "")
	}

	time.Sleep(500 * time.Millisecond)

	// Check pillars via observe server (port 3010)
	req, _ := http.NewRequest("GET", "http://localhost:3010/api/pillars?product=chat", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("pillars request: %v", err)
	}
	defer resp.Body.Close()

	var result struct {
		Pillars []struct {
			Name        string `json:"name"`
			Constraints []struct {
				Name   string `json:"name"`
				Status string `json:"status"`
				Value  string `json:"value"`
			} `json:"constraints"`
		} `json:"pillars"`
	}
	json.NewDecoder(resp.Body).Decode(&result)

	for _, p := range result.Pillars {
		if p.Name == "resilient" {
			for _, c := range p.Constraints {
				if c.Name == "chat-drop-rate" && c.Status == "fail" {
					t.Errorf("SLO violated: chat-drop-rate = %s", c.Value)
				}
			}
		}
	}
}

func TestSLO_ReconnectLatency(t *testing.T) {
	if testing.Short() {
		t.Skip("SLO test")
	}

	token := os.Getenv("SOUL_V2_TOKEN")
	if token == "" {
		t.Skip("SOUL_V2_TOKEN not set")
	}

	var latencies []time.Duration
	for i := 0; i < 10; i++ {
		conn := connectWS(t, "ws://localhost:3002/ws", token)
		readUntilType(t, conn, "connection.ready", 5*time.Second)
		conn.Close(websocket.StatusNormalClosure, "")

		start := time.Now()
		conn2 := connectWS(t, "ws://localhost:3002/ws", token)
		readUntilType(t, conn2, "connection.ready", 5*time.Second)
		latencies = append(latencies, time.Since(start))
		conn2.Close(websocket.StatusNormalClosure, "")
	}

	sort.Slice(latencies, func(i, j int) bool { return latencies[i] < latencies[j] })
	p95 := latencies[int(float64(len(latencies)-1)*0.95)]
	if p95 > 3*time.Second {
		t.Errorf("SLO violated: reconnect P95 = %v (must be < 3s)", p95)
	}
}
```

- [ ] **Step 2: Commit**

```bash
git add tests/integration/slo_test.go
git commit -m "test: add SLO verification gates for chat-drop-rate and reconnect latency"
```

---

## Chunk 10: Health Endpoint & Final Verification

### Task 22: Add Internal Health Endpoint

**Files:**
- Modify: `internal/chat/server/server.go`
- Test: `internal/chat/server/server_test.go`

Note: `/healthz` doesn't start with `/api/`, so it's already NOT protected by `authMiddleware` — no auth skip needed. Hub already has `ClientCount()` via the `countReq` channel — do NOT add a duplicate.

- [ ] **Step 1: Write test for /healthz**

```go
func TestHealthEndpoint(t *testing.T) {
	// Start test server using existing test setup
	srv := newTestServer(t)

	req := httptest.NewRequest("GET", "/healthz", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req) // Use the server's ServeHTTP method

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var body map[string]interface{}
	json.NewDecoder(w.Body).Decode(&body)

	if body["status"] != "ok" {
		t.Errorf("expected status=ok, got %v", body["status"])
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /home/rishav/soul-v2 && go test ./internal/chat/server/ -run TestHealthEndpoint -v`
Expected: FAIL — 404

- [ ] **Step 3: Add /healthz route**

In `server.go`, where routes are registered (before the SPA fallback), add:

```go
s.mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
	wsCount := 0
	if s.hub != nil {
		wsCount = s.hub.ClientCount()
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":         "ok",
		"uptime_s":       time.Since(s.startTime).Seconds(),
		"ws_connections": wsCount,
	})
})
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd /home/rishav/soul-v2 && go test ./internal/chat/server/ -run TestHealthEndpoint -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/chat/server/server.go internal/chat/server/server_test.go
git commit -m "feat: add unauthenticated /healthz endpoint with uptime and WS connection count"
```

---

### Task 23: Final Verification

- [ ] **Step 1: Run full static verification**

Run: `cd /home/rishav/soul-v2 && make verify-static`
Expected: All pass

- [ ] **Step 2: Run unit tests**

Run: `cd /home/rishav/soul-v2 && go test ./internal/... -v -count=1`
Expected: All pass

- [ ] **Step 3: Run frontend type check**

Run: `cd /home/rishav/soul-v2 && npx tsc --noEmit`
Expected: No errors

- [ ] **Step 4: Build all binaries**

Run: `cd /home/rishav/soul-v2 && make build`
Expected: All 13 binaries built

- [ ] **Step 5: Manual pillar check against observe server (port 3010)**

Run: `curl -s -H "Authorization: Bearer $TOKEN" http://localhost:3010/api/pillars?product=chat | jq '.pillars[] | select(.name == "resilient" or .name == "transparent")'`
Expected: Live constraints with pass/warn/fail (not all "static")

- [ ] **Step 6: Commit all remaining changes**

```bash
git add internal/ web/ cmd/ tests/
git commit -m "feat: pillar-compliant chat observability and resilience — complete"
```

---

## Summary

| Chunk | Tasks | What it delivers |
|-------|-------|-----------------|
| 1 | 1-2 | Event taxonomy (incl. frontend telemetry types), close code classifier, ws.close emission |
| 2 | 3-6 | Telemetry allowlist expansion, auth events from middleware/WS, system.exit (graceful + panic + gap detection) |
| 3 | 6-7 | In-memory ConnectionHealth aggregator, wired to Hub with reconnect event emission |
| 4 | 8-10 | JSONL-based ConnectionHealthReport (uses `readProductEvents`, `ts`/`event` format), live resilient + enhanced transparent pillars |
| 5 | 11-13 | Frontend classified disconnect/reconnect/auth telemetry (with correct `TelemetryEvent` types) |
| 6 | 14-15 | Durable send queue with idempotent messages, frontend `session.resume` on reconnect |
| 7 | 16-17a | Server-side replay buffer with `session.resume`, replay anchor IDs in outbound messages, server-side message deduplication |
| 8 | 18-19 | Stream event dimensions (product/model), stream-total-p95 constraint |
| 9 | 20-21 | Integration/chaos tests (nhooyr.io/websocket, correct payload parsing) + SLO gates |
| 10 | 22-23 | Health endpoint (`s.mux`, reuses existing `ClientCount`) + final verification |

**SLO targets enforced:**
- `chat_drop_rate < 0.5%` sessions/hour
- `reconnect_success_rate > 95%`
- `first_token_p50 < 200ms` (existing)
- `stream_total_p95 < 30s` (new)
- `reconnect_p95 < 3s` (tested in SLO gate)

**End-to-end event flow (verified complete):**
1. Server emits `ws.close` (classified), `auth.fail`, `auth.ok`, `ws.reconnect.success`, `system.exit`, `system.start` → JSONL
2. Frontend emits `frontend.ws.disconnect`, `frontend.ws.reconnect`, `frontend.auth.fail` → `/api/telemetry` (allowlisted) → JSONL
3. Observe server reads JSONL via `readProductEvents("")` → `ConnectionHealthReport` → live pillar constraints
4. Client tracks `lastMessageId` → sends `session.resume` on reconnect → server replays missed messages
5. Client send queue persists unsent messages → flushes on reconnect → server deduplicates by `messageId`
6. Gap detection: `system.start` without preceding `system.exit` = panic/OOM/SIGKILL
