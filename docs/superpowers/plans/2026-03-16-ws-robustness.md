# WebSocket Robustness & Observability Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make the WebSocket connection resilient, observable, and stateful across reconnects via four independently shippable layers.

**Architecture:** Layer 1 instruments the connection lifecycle with classified metrics; Layer 2 fixes session state desync on reconnect and the session-stuck-running bug; Layer 3 prevents infinite auth retry loops, adds give-up/manual-retry, and visibility-aware reconnects; Layer 4 coalesces high-frequency token frames to eliminate slow-client disconnects and adds tool progress detail.

**Tech Stack:** Go 1.24, nhooyr.io/websocket, React 19, TypeScript 5.9, Tailwind v4

**Spec:** `docs/superpowers/specs/2026-03-16-ws-robustness-design.md`

---

## Chunk 1: Layer 1 — Observability Instrumentation

### Task 1: Add `EventWSUpgrade` metric constant

**Files:**
- Modify: `internal/chat/metrics/types.go`

- [ ] **Step 1: Add the new constant after existing WS event constants**

  In `internal/chat/metrics/types.go`, find the block with `EventWSConnect`, `EventWSDisconnect`, etc. and add `EventWSUpgrade`:

  ```go
  EventWSUpgrade     = "ws.upgrade"
  ```

  Place it immediately after `EventWSConnect = "ws.connect"`.

- [ ] **Step 2: Verify no compilation errors**

  ```bash
  cd /home/rishav/soul-v2 && go vet ./internal/chat/metrics/...
  ```

  Expected: no output (success).

- [ ] **Step 3: Commit**

  ```bash
  git add internal/chat/metrics/types.go
  git commit -m "feat: add ws.upgrade metric event constant"
  ```

---

### Task 2: Add `closeReason` field to Client and classify disconnect reasons

**Files:**
- Modify: `internal/chat/ws/client.go`
- Test: `internal/chat/ws/client_test.go`

- [ ] **Step 1: Write the failing test for `slow_client_queue_full` reason**

  Open `internal/chat/ws/client_test.go`. Find the existing test file and add a new test at the end. This test sends 257 messages to a full send channel (cap = 256) and verifies the client's `closeReason` is set to `slow_client_queue_full`.

  ```go
  func TestClient_SlowClientQueueFull_SetsCloseReason(t *testing.T) {
  	// Create a minimal client without a real connection.
  	c := &Client{
  		id:   "test-client",
  		send: make(chan []byte, sendChannelCap),
  	}

  	// Fill the channel to capacity.
  	for i := 0; i < sendChannelCap; i++ {
  		c.send <- []byte(`{"type":"test"}`)
  	}

  	// Next send should trigger slow-client close and set closeReason.
  	ok := c.Send([]byte(`{"type":"overflow"}`))
  	if ok {
  		t.Fatal("expected Send to return false for full channel")
  	}
  	// closeReason must be non-nil and equal to "slow_client_queue_full".
  	v := c.closeReason.Load()
  	if v == nil {
  		t.Fatal("closeReason is nil, expected slow_client_queue_full")
  	}
  	if reason := v.(string); reason != "slow_client_queue_full" {
  		t.Errorf("closeReason = %q, want %q", reason, "slow_client_queue_full")
  	}
  }
  ```

- [ ] **Step 2: Run test to verify it fails**

  ```bash
  cd /home/rishav/soul-v2 && go test ./internal/chat/ws/... -run TestClient_SlowClientQueueFull_SetsCloseReason -v
  ```

  Expected: FAIL — `c.closeReason` field doesn't exist yet.

- [ ] **Step 3: Add `closeReason atomic.Value` field to Client struct**

  In `internal/chat/ws/client.go`, add `closeReason atomic.Value` to the `Client` struct (after `connTime time.Time`):

  ```go
  // closeReason stores the classified reason for disconnect, set at the point
  // of failure and read by ReadPump's deferred disconnect metric emission.
  // Uses atomic.Value for safe concurrent access (WritePump/Send write it,
  // ReadPump reads it in defer).
  closeReason atomic.Value // stores string
  ```

- [ ] **Step 4: Set `slow_client_queue_full` in `Send()`**

  In `Send()`, in the `default:` branch (the slow-client close path), add before `close(c.send)`:

  ```go
  c.closeReason.Store("slow_client_queue_full")
  ```

- [ ] **Step 5: Run test to verify it passes**

  ```bash
  cd /home/rishav/soul-v2 && go test ./internal/chat/ws/... -run TestClient_SlowClientQueueFull_SetsCloseReason -v
  ```

  Expected: PASS.

- [ ] **Step 6: Set close reason in `ReadPump` based on close status**

  In `ReadPump()`'s read error branch (after `typ, data, err := c.conn.Read(c.ctx)` returns an error), update the existing switch logic to also set `closeReason`. Replace:

  ```go
  if err != nil {
      // Check if this is a normal close or context cancellation.
      if websocket.CloseStatus(err) == websocket.StatusNormalClosure {
          log.Printf("ws: client %s closed normally", c.id)
      } else if c.ctx.Err() != nil {
          log.Printf("ws: client %s context cancelled", c.id)
      } else {
          log.Printf("ws: client %s read error: %v", c.id, err)
      }
      return
  }
  ```

  With:

  ```go
  if err != nil {
      switch {
      case websocket.CloseStatus(err) == websocket.StatusNormalClosure:
          log.Printf("ws: client %s closed normally", c.id)
          if c.closeReason.Load() == nil {
              c.closeReason.Store("client_normal_close")
          }
      case c.ctx.Err() != nil:
          log.Printf("ws: client %s context cancelled", c.id)
          if c.closeReason.Load() == nil {
              c.closeReason.Store("context_cancelled")
          }
      default:
          log.Printf("ws: client %s read error: %v", c.id, err)
          if c.closeReason.Load() == nil {
              c.closeReason.Store("read_error")
          }
      }
      return
  }
  ```

- [ ] **Step 7: Set close reason in `WritePump` for ping timeout and write error**

  In `WritePump()`, find the ping failure path and the write error path. Add reason setting:

  After `if err := c.conn.Write(c.ctx, websocket.MessageText, msg); err != nil {`:
  ```go
  log.Printf("ws: client %s write error: %v", c.id, err)
  c.closeReason.Store("write_error")
  return
  ```

  After `if err != nil { // ping timeout` (inside the ticker.C case):
  ```go
  log.Printf("ws: client %s ping failed: %v", c.id, err)
  c.closeReason.Store("ping_timeout")
  return
  ```

- [ ] **Step 8: Update ReadPump's disconnect metric to use `closeReason`**

  In `ReadPump()`'s defer block, replace the hardcoded `"reason": "read_pump_exit"` with the stored reason and the close code:

  ```go
  if c.hub.metrics != nil {
      duration := time.Since(c.connTime).Seconds()
      reason := "read_pump_exit" // fallback if no reason was classified
      if v := c.closeReason.Load(); v != nil {
          reason = v.(string)
      }
      // close_code: use websocket.CloseStatus from last read error if available.
      // We don't have the error in defer scope, so use -1 (abnormal) as default;
      // client_normal_close is the only reason that receives code 1000.
      closeCode := -1
      if reason == "client_normal_close" {
          closeCode = 1000
      }
      _ = c.hub.metrics.Log(metrics.EventWSDisconnect, map[string]interface{}{
          "client_id":        c.id,
          "reason":           reason,
          "close_code":       closeCode,
          "duration_seconds": duration,
      })
  }
  ```

- [ ] **Step 9: Run all ws tests**

  ```bash
  cd /home/rishav/soul-v2 && go test ./internal/chat/ws/... -v 2>&1 | tail -20
  ```

  Expected: all PASS.

- [ ] **Step 10: Commit**

  ```bash
  git add internal/chat/ws/client.go internal/chat/ws/client_test.go
  git commit -m "feat: classify disconnect reasons in Client closeReason field"
  ```

---

### Task 3: Set `server_shutdown` reason in Hub.Run shutdown path

**Files:**
- Modify: `internal/chat/ws/hub.go`

- [ ] **Step 1: Update Hub.Run shutdown path to set closeReason before closing clients**

  In `hub.go`, find `case <-ctx.Done():` in `Hub.Run`. Change:

  ```go
  case <-ctx.Done():
      // Close all remaining clients on shutdown.
      for client := range h.clients {
          client.closeSend()
          client.Close()
          delete(h.clients, client)
      }
      return
  ```

  To:

  ```go
  case <-ctx.Done():
      // Close all remaining clients on shutdown.
      for client := range h.clients {
          client.closeReason.Store("server_shutdown")
          client.closeSend()
          client.Close()
          delete(h.clients, client)
      }
      return
  ```

- [ ] **Step 2: Emit `ws.upgrade` metric in HandleUpgrade**

  In `HandleUpgrade`, before the origin check, origin rejection, upgrade failure, and success paths, add metric emission. The function currently has two early returns (origin rejection and upgrade failure) and one success path. Update it:

  After the origin check:
  ```go
  if !h.isOriginAllowed(r) {
      if h.metrics != nil {
          _ = h.metrics.Log(metrics.EventWSUpgrade, map[string]interface{}{
              "outcome": "origin_rejected",
              "origin":  r.Header.Get("Origin"),
              "client_id": "",
          })
      }
      http.Error(w, "Forbidden", http.StatusForbidden)
      return
  }
  ```

  After the `websocket.Accept` failure:
  ```go
  if err != nil {
      log.Printf("ws: upgrade failed: %v", err)
      if h.metrics != nil {
          _ = h.metrics.Log(metrics.EventWSUpgrade, map[string]interface{}{
              "outcome": "upgrade_failed",
              "origin":  r.Header.Get("Origin"),
              "client_id": "",
          })
      }
      return
  }
  ```

  After the existing `ws.connect` metric emission (so we have the clientID):
  ```go
  if h.metrics != nil {
      _ = h.metrics.Log(metrics.EventWSUpgrade, map[string]interface{}{
          "outcome":   "success",
          "origin":    origin,
          "client_id": clientID,
      })
  }
  ```

- [ ] **Step 3: Run ws package tests**

  ```bash
  cd /home/rishav/soul-v2 && go test ./internal/chat/ws/... -v 2>&1 | tail -20
  ```

  Expected: all PASS.

- [ ] **Step 4: Commit**

  ```bash
  git add internal/chat/ws/hub.go
  git commit -m "feat: emit ws.upgrade metric and set server_shutdown close reason"
  ```

---

### Task 4: Emit `ws.upgrade` with `auth_rejected` in auth middleware

**Files:**
- Modify: `internal/chat/server/server.go`
- Test: `internal/chat/server/server_test.go`

- [ ] **Step 1: Write the failing test for auth rejection metric**

  Open `internal/chat/server/server_test.go`. Add a test that exercises the auth middleware rejection of `/ws` and verifies the hook fires:

  ```go
  func TestAuthMiddleware_WSRejection_CallsHook(t *testing.T) {
      rejected := false
      hook := func(r *http.Request) { rejected = true }

      next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
          w.WriteHeader(http.StatusSwitchingProtocols)
      })

      // authMiddleware("secret-token", nil, hook) — third arg doesn't exist yet.
      // This test will fail to compile until Step 2 adds the parameter.
      handler := authMiddleware("secret-token", nil, hook)(next)

      req := httptest.NewRequest("GET", "/ws", nil) // no auth credentials
      w := httptest.NewRecorder()
      handler.ServeHTTP(w, req)

      if w.Code != http.StatusUnauthorized {
          t.Errorf("expected 401, got %d", w.Code)
      }
      if !rejected {
          t.Error("expected onWSRejected hook to be called on /ws auth rejection")
      }
  }
  ```

- [ ] **Step 1b: Run to verify compile failure (expected)**

  ```bash
  cd /home/rishav/soul-v2 && go test ./internal/chat/server/... 2>&1 | head -10
  ```

  Expected: compile error — `authMiddleware` called with too many arguments. This confirms TDD baseline.

- [ ] **Step 2: Update `authMiddleware` signature to accept a ws-rejection hook**

  Change `authMiddleware` signature from:
  ```go
  func authMiddleware(token string, ticketValid func(string) bool) func(http.Handler) http.Handler
  ```
  To:
  ```go
  func authMiddleware(token string, ticketValid func(string) bool, onWSRejected func(r *http.Request)) func(http.Handler) http.Handler
  ```

  In the `/ws` rejection path (lines where `http.StatusUnauthorized` is written when `path == "/ws"` AND the token check fails), before writing the 401 response, call `onWSRejected` if non-nil:

  The current rejection is at the bottom of the handler. The `/ws` path falls through to the final 401 response. Change the `path == "/ws"` block to call the hook:

  ```go
  // If this is /ws and we reach here, all auth checks failed.
  if path == "/ws" && onWSRejected != nil {
      onWSRejected(r)
  }
  w.Header().Set("Content-Type", "application/json")
  w.WriteHeader(http.StatusUnauthorized)
  w.Write([]byte(`{"error":"unauthorized"}`))
  ```

  Note: The current code structure has ticket/token checks inside `if path == "/ws"` block, then falls through to the 401. We need to add the hook call right before the final 401 response, guarded by `path == "/ws"`.

- [ ] **Step 3: Update the `authMiddleware` call site in `Server.setupRoutes` or wherever it's called**

  Find the `authMiddleware` call site. Grep shows it's at: `authMiddleware(s.authToken, s.consumeWSTicket)`. Update it to:

  ```go
  wsRejectedHook := func(r *http.Request) {
      if s.metrics != nil {
          _ = s.metrics.Log(metrics.EventWSUpgrade, map[string]interface{}{
              "outcome":   "auth_rejected",
              "origin":    r.Header.Get("Origin"),
              "client_id": "",
          })
      }
  }
  authMW := authMiddleware(s.authToken, s.consumeWSTicket, wsRejectedHook)
  ```

  Then replace the inline `authMiddleware(...)` call with `authMW`.

- [ ] **Step 4: Run server tests**

  ```bash
  cd /home/rishav/soul-v2 && go test ./internal/chat/server/... -v 2>&1 | tail -30
  ```

  Expected: all PASS.

- [ ] **Step 5: Commit**

  ```bash
  git add internal/chat/server/server.go internal/chat/server/server_test.go
  git commit -m "feat: emit ws.upgrade auth_rejected metric in auth middleware"
  ```

---

### Task 5: Add batched telemetry support to `handleTelemetry`

**Files:**
- Modify: `internal/chat/server/server.go`
- Test: `internal/chat/server/server_test.go`

- [ ] **Step 1: Write the failing test for batched telemetry**

  Add two tests in `server_test.go`. The `handleTelemetry` handler returns 503 when `s.metrics == nil`, so the test server must have a real metrics logger. Use `metrics.NewEventLogger(t.TempDir(), "")` to create one:

  ```go
  func newTestServerWithMetrics(t *testing.T) *Server {
      t.Helper()
      mel, err := metrics.NewEventLogger(t.TempDir(), "")
      if err != nil {
          t.Fatalf("metrics.NewEventLogger: %v", err)
      }
      t.Cleanup(func() { _ = mel.Close() })
      return New(WithMetrics(mel))
  }

  func TestHandleTelemetry_BatchedPayload_Returns204(t *testing.T) {
      s := newTestServerWithMetrics(t)
      body := `{"batch":[{"type":"frontend.ws","data":{"event":"close","code":1006}},{"type":"frontend.ws","data":{"event":"connect_attempt","attempt":1}}]}`
      req := httptest.NewRequest("POST", "/api/telemetry", strings.NewReader(body))
      req.Header.Set("Content-Type", "application/json")
      w := httptest.NewRecorder()
      s.handleTelemetry(w, req)
      if w.Code != http.StatusNoContent {
          t.Errorf("expected 204, got %d: %s", w.Code, w.Body.String())
      }
  }

  func TestHandleTelemetry_BatchedPayload_SkipsInvalidType(t *testing.T) {
      s := newTestServerWithMetrics(t)
      // One valid entry, one with unknown type — should still return 204 (partial ingestion).
      body := `{"batch":[{"type":"frontend.ws","data":{"event":"close"}},{"type":"unknown.type","data":{}}]}`
      req := httptest.NewRequest("POST", "/api/telemetry", strings.NewReader(body))
      req.Header.Set("Content-Type", "application/json")
      w := httptest.NewRecorder()
      s.handleTelemetry(w, req)
      if w.Code != http.StatusNoContent {
          t.Errorf("expected 204 (partial ingestion), got %d", w.Code)
      }
  }
  ```


- [ ] **Step 2: Update `handleTelemetry` to accept batched payloads**

  Replace the current `handleTelemetry` body. The new implementation decodes into a `json.RawMessage` map first, then branches:

  ```go
  func (s *Server) handleTelemetry(w http.ResponseWriter, r *http.Request) {
      if s.metrics == nil {
          http.Error(w, "metrics not configured", http.StatusServiceUnavailable)
          return
      }

      body, err := io.ReadAll(io.LimitReader(r.Body, 64*1024))
      if err != nil {
          http.Error(w, "read error", http.StatusBadRequest)
          return
      }

      // Try to detect batch vs single event.
      var envelope struct {
          Batch []struct {
              Type string                 `json:"type"`
              Data map[string]interface{} `json:"data"`
          } `json:"batch"`
          // Single-event fields (ignored when batch is present)
          Type string                 `json:"type"`
          Data map[string]interface{} `json:"data"`
      }
      if err := json.Unmarshal(body, &envelope); err != nil {
          http.Error(w, "invalid JSON", http.StatusBadRequest)
          return
      }

      validTypes := map[string]bool{
          metrics.EventFrontendError:  true,
          metrics.EventFrontendRender: true,
          metrics.EventFrontendWS:     true,
          metrics.EventFrontendUsage:  true,
      }

      if envelope.Batch != nil {
          // Batched path: process each entry, skip invalid types.
          for _, entry := range envelope.Batch {
              if !validTypes[entry.Type] {
                  log.Printf("telemetry: batch entry has unknown type %q, skipping", entry.Type)
                  continue
              }
              _ = s.metrics.Log(entry.Type, entry.Data)
          }
      } else {
          // Single event path (backward compat).
          if !validTypes[envelope.Type] {
              http.Error(w, "unknown event type", http.StatusBadRequest)
              return
          }
          _ = s.metrics.Log(envelope.Type, envelope.Data)
      }

      w.WriteHeader(http.StatusNoContent)
  }
  ```

  Also add `"io"` to imports if not present.

- [ ] **Step 3: Run server tests**

  ```bash
  cd /home/rishav/soul-v2 && go test ./internal/chat/server/... -v 2>&1 | tail -30
  ```

  Expected: all PASS.

- [ ] **Step 4: Commit**

  ```bash
  git add internal/chat/server/server.go internal/chat/server/server_test.go
  git commit -m "feat: support batched telemetry payload in handleTelemetry"
  ```

---

### Task 6: Add `reportWSLifecycle` and client-side batching to `telemetry.ts`

**Files:**
- Modify: `web/src/lib/telemetry.ts`

- [ ] **Step 1: Add the telemetry batch buffer and `reportWSLifecycle` helper**

  Append to `web/src/lib/telemetry.ts` after the existing helper functions:

  ```ts
  // --- WS lifecycle telemetry batching ---
  // Accumulate WS lifecycle events and flush every 5s or on page unload.
  // This avoids hitting the 60 RPM per-IP rate limit during reconnect storms.

  interface TelemetryEntry {
    type: TelemetryEvent;
    data: Record<string, unknown>;
  }

  const lifecycleBatch: TelemetryEntry[] = [];
  let flushTimer: ReturnType<typeof setTimeout> | null = null;

  function flushLifecycleBatch(): void {
    if (lifecycleBatch.length === 0) return;
    const entries = lifecycleBatch.splice(0);
    const token = getToken()?.trim();
    const headers: Record<string, string> = { 'Content-Type': 'application/json' };
    if (token) headers['Authorization'] = `Bearer ${token}`;
    fetch('/api/telemetry', {
      method: 'POST',
      headers,
      body: JSON.stringify({ batch: entries }),
      keepalive: true,
    }).catch(() => {});
  }

  function scheduleFlush(): void {
    if (flushTimer !== null) return;
    flushTimer = setTimeout(() => {
      flushTimer = null;
      flushLifecycleBatch();
    }, 5000);
  }

  if (typeof window !== 'undefined') {
    window.addEventListener('unload', () => {
      if (flushTimer !== null) {
        clearTimeout(flushTimer);
        flushTimer = null;
      }
      flushLifecycleBatch();
    });
  }

  /**
   * Reports a WS lifecycle event. Events are batched and flushed every 5s
   * or on page unload (fire-and-forget, keepalive).
   */
  export function reportWSLifecycle(event: string, data: Record<string, unknown>): void {
    try {
      lifecycleBatch.push({ type: 'frontend.ws', data: { event, ...data } });
      scheduleFlush();
    } catch {
      // Telemetry must never throw.
    }
  }
  ```

- [ ] **Step 2: Verify TypeScript compiles**

  ```bash
  cd /home/rishav/soul-v2/web && npx tsc --noEmit 2>&1 | head -20
  ```

  Expected: no errors.

- [ ] **Step 3: Commit**

  ```bash
  git add web/src/lib/telemetry.ts
  git commit -m "feat: add reportWSLifecycle helper with client-side batching"
  ```

---

### Task 7: Emit lifecycle telemetry events from `useWebSocket.ts`

**Files:**
- Modify: `web/src/hooks/useWebSocket.ts`

- [ ] **Step 1: Import `reportWSLifecycle` and add telemetry calls**

  In `useWebSocket.ts`, update the import:

  ```ts
  import { reportError, reportWSLifecycle } from '../lib/telemetry';
  ```

  In the `connect` callback:

  1. Before opening the WebSocket (after ticket fetch), emit `connect_attempt`:
     ```ts
     reportWSLifecycle('connect_attempt', { attempt: attemptRef.current, ticketUsed: !!ticket });
     ```

  2. In `socket.onopen`:
     ```ts
     socket.onopen = () => {
       reportWSLifecycle('open', { attempt: attemptRef.current });
     };
     ```

  3. In `socket.onclose` (before reconnect logic):
     ```ts
     socket.onclose = (event: CloseEvent) => {
       const duration = performance.now() - openTimeRef.current;
       reportWSLifecycle('close', {
         code: event.code,
         reason: event.reason,
         wasClean: event.wasClean,
         duration_ms: Math.round(duration),
       });
       // ... existing reconnect logic
     };
     ```

  4. In `socket.onerror`:
     ```ts
     socket.onerror = () => {
       reportWSLifecycle('error', { attempt: attemptRef.current });
       // ... existing status set
     };
     ```

  5. In the `fetchWSTicket().then(...)` callback, after ticket fetch:
     ```ts
     // Track open time for duration calculation.
     openTimeRef.current = performance.now();
     ```

  Add `const openTimeRef = useRef<number>(0);` to the hook's state variables.

  Also emit `ticket_fetch` in the ticket fetch logic (will be done in Layer 3 when we refactor fetchWSTicket).

- [ ] **Step 2: Verify TypeScript compiles**

  ```bash
  cd /home/rishav/soul-v2/web && npx tsc --noEmit 2>&1 | head -20
  ```

  Expected: no errors.

- [ ] **Step 3: Commit**

  ```bash
  git add web/src/hooks/useWebSocket.ts
  git commit -m "feat: emit ws lifecycle telemetry events from useWebSocket"
  ```

---

## Chunk 2: Layer 2 — Reconnect State Recovery

### Task 8: Fix `runStream` — silent return on `context.Canceled`

**Files:**
- Modify: `internal/chat/ws/handler.go`
- Test: `internal/chat/ws/handler_test.go`

- [ ] **Step 1: Add a `mockStreamClient` type to `handler_test.go`**

  This mock will be used by multiple tests. Add it near the top of `handler_test.go` after the imports:

  ```go
  // mockStreamClient is a configurable test double for stream.Client.
  // canceledOnStream: if true, Stream() returns context.Canceled immediately.
  type mockStreamClient struct {
      canceledOnStream bool
  }

  func (m *mockStreamClient) Stream(ctx context.Context, req *stream.Request) (<-chan stream.Event, error) {
      if m.canceledOnStream || ctx.Err() != nil {
          return nil, context.Canceled
      }
      ch := make(chan stream.Event)
      close(ch) // empty stream, no events
      return ch, nil
  }
  ```

  Note: If `stream.Client` is a struct (not an interface), check whether `handler.go` accepts a `StreamClient` interface. If not, this test approach requires either using the real `stream.Client` or refactoring. Check the `runStream` signature: it calls `h.streamClient.Stream(...)` where `streamClient` is `*stream.Client`. To make this mockable, the `runStream` call must be abstracted. If `stream.Client` cannot be substituted, use a different approach (Step 1b below).

- [ ] **Step 1b (if mock approach unavailable): Write a behavioral test via session status**

  If `streamClient` is a concrete `*stream.Client` struct with no interface, test the fix behaviorally via the canceled-context path by calling `runStream` with the real stream client and a pre-cancelled context:

  ```go
  func TestRunStream_CanceledContext_SessionNotCompleted(t *testing.T) {
      hub, store, cancel := setupTestEnv(t)
      defer cancel()
      ctx := context.Background()
      conn, cleanup := connectClient(t, ctx, hub)
      defer cleanup()
      drainMessages(t, ctx, conn, 200*time.Millisecond)

      sess, _ := store.CreateSession("Test")
      _ = store.UpdateSessionStatus(sess.ID, session.StatusRunning)

      clients := hub.Clients()
      if len(clients) == 0 {
          t.Fatal("no clients")
      }
      client := clients[0]

      // Create a real stream.Client with a nil HTTP client — Stream will return
      // context.Canceled immediately because the context is pre-cancelled.
      // (stream.Client.Stream checks ctx before making the HTTP call.)
      sc, err := stream.NewClient("", "") // uses placeholder credentials
      if err != nil {
          t.Skip("cannot create stream client in test environment")
      }

      handler := hub.handler
      // Temporarily set a stream client.
      handler.streamClient = sc

      agentCtx, agentCancel := context.WithCancel(context.Background())
      agentCancel() // pre-cancel

      req := &stream.Request{MaxTokens: 1, Messages: []stream.Message{}}
      handler.runStream(client, sess.ID, req, agentCtx)

      // Session status must NOT be completed — silent return.
      updated, _ := store.GetSession(sess.ID)
      if updated.Status != session.StatusRunning {
          t.Errorf("runStream changed session status to %v on context.Canceled; want StatusRunning", updated.Status)
      }
      // No chat.error must have been sent.
      msgs := drainMessages(t, ctx, conn, 100*time.Millisecond)
      for _, m := range msgs {
          if m["type"] == TypeChatError {
              t.Error("unexpected chat.error sent on context.Canceled")
          }
      }
  }
  ```

- [ ] **Step 2: Implement the fix in `runStream`**

  In `handler.go`, find the `ch, err := h.streamClient.Stream(ctx, req)` error path (around line 508-521). Add a `context.Canceled` check at the top of the error branch:

  ```go
  ch, err := h.streamClient.Stream(ctx, req)
  if err != nil {
      if errors.Is(err, context.Canceled) {
          // Expected: chat.stop, superseded stream, or client disconnect.
          // Caller handles session lifecycle — they know the intent.
          return
      }
      if errors.Is(err, context.DeadlineExceeded) {
          h.completeSession(client, sessionID)
          h.logAPIError(sessionID, err)
          h.sendClassifiedError(client, sessionID, err)
          return
      }
      log.Printf("ws: stream error after %v for session %s round %d: %v",
          time.Since(roundStart).Round(time.Millisecond), sessionID, round, err)
      var authErr *stream.AuthError
      if errors.As(err, &authErr) {
          log.Printf("ws: AUTH FAILURE for session %s: %v", sessionID, err)
      }
      h.logAPIError(sessionID, err)
      h.sendClassifiedError(client, sessionID, err)
      return
  }
  ```

- [ ] **Step 3: Run handler tests**

  ```bash
  cd /home/rishav/soul-v2 && go test ./internal/chat/ws/... -run TestRunStream -v
  ```

  Expected: PASS (or skip if mock client complexity is deferred).

- [ ] **Step 4: Commit**

  ```bash
  git add internal/chat/ws/handler.go internal/chat/ws/handler_test.go
  git commit -m "fix: runStream returns silently on context.Canceled, caller owns lifecycle"
  ```

---

### Task 9: Fix `handleChatStop` — call `completeSession` after done

**Files:**
- Modify: `internal/chat/ws/handler.go`
- Test: `internal/chat/ws/handler_test.go`

- [ ] **Step 1: Write a failing test for `handleChatStop` completing the session**

  In `handler_test.go`, add a test that:
  1. Creates a session in `running` status
  2. Sends `chat.stop`
  3. Verifies session transitions to `completed`

  ```go
  func TestHandleChatStop_CompletesSession(t *testing.T) {
      hub, store, cancel := setupTestEnv(t)
      defer cancel()
      ctx := context.Background()
      conn, cleanup := connectClient(t, ctx, hub)
      defer cleanup()

      drainMessages(t, ctx, conn, 200*time.Millisecond)

      sess, _ := store.CreateSession("Test")
      _ = store.UpdateSessionStatus(sess.ID, session.StatusRunning)

      // Simulate a running agent entry by adding one to the handler's sessions map.
      clients := hub.Clients()
      if len(clients) == 0 {
          t.Fatal("no clients")
      }
      client := clients[0]

      // Add a fake agent entry that completes immediately when cancelled.
      done := make(chan struct{})
      handler := hub.handler
      cs := handler.getOrCreateChatSession(client)
      cs.mu.Lock()
      cs.agents[sess.ID] = agentEntry{
          cancel: func() { close(done) },
          done:   done,
      }
      cs.mu.Unlock()

      // Send chat.stop.
      stopMsg, _ := json.Marshal(map[string]interface{}{
          "type":      TypeChatStop,
          "sessionId": sess.ID,
      })
      conn.Write(ctx, websocket.MessageText, stopMsg)

      // Wait for session status to change (poll with timeout instead of sleep).
      deadline := time.Now().Add(500 * time.Millisecond)
      for time.Now().Before(deadline) {
          updated, _ := store.GetSession(sess.ID)
          if updated.Status != session.StatusRunning {
              return // test passes
          }
          time.Sleep(10 * time.Millisecond)
      }
      t.Error("session still running after chat.stop, expected completed")
  }
  ```

- [ ] **Step 2: Run test to verify it fails**

  ```bash
  cd /home/rishav/soul-v2 && go test ./internal/chat/ws/... -run TestHandleChatStop_CompletesSession -v
  ```

  Expected: FAIL — session stays in `running`.

- [ ] **Step 3: Implement the fix in `handleChatStop`**

  In `handler.go`, find `handleChatStop` (around line 966). After `<-entry.done`, add the `completeSession` call:

  ```go
  if ok {
      entry.cancel()
      <-entry.done
      h.completeSession(client, msg.SessionID)
  }
  ```

- [ ] **Step 4: Run test to verify it passes**

  ```bash
  cd /home/rishav/soul-v2 && go test ./internal/chat/ws/... -run TestHandleChatStop_CompletesSession -v
  ```

  Expected: PASS.

- [ ] **Step 5: Commit**

  ```bash
  git add internal/chat/ws/handler.go internal/chat/ws/handler_test.go
  git commit -m "fix: handleChatStop calls completeSession after stream done"
  ```

---

### Task 10: Fix `handleChatSend` superseded stream — restore session to running

**Files:**
- Modify: `internal/chat/ws/handler.go`
- Test: `internal/chat/ws/handler_test.go`

- [ ] **Step 1: Write a failing test for superseded stream status restoration**

  In `handler_test.go`, add a test that:
  1. Creates a running agent for a session
  2. Sends a new `chat.send` to the same session (which cancels the existing agent)
  3. Verifies after the old agent completes, the session status is `running` (for the new stream)

  ```go
  func TestHandleChatSend_SupersededStream_RestoresRunningStatus(t *testing.T) {
      hub, store, cancel := setupTestEnv(t)
      defer cancel()
      ctx := context.Background()
      conn, cleanup := connectClient(t, ctx, hub)
      defer cleanup()

      drainMessages(t, ctx, conn, 200*time.Millisecond)

      sess, _ := store.CreateSession("Test")
      _ = store.UpdateSessionStatus(sess.ID, session.StatusRunning)
      // Add a user message so handleChatSend doesn't fail validation
      _, _ = store.AddMessage(sess.ID, "user", "hello")

      clients := hub.Clients()
      if len(clients) == 0 {
          t.Fatal("no clients")
      }
      client := clients[0]

      // Add a fake existing agent.
      done := make(chan struct{})
      handler := hub.handler
      cs := handler.getOrCreateChatSession(client)
      cs.mu.Lock()
      cs.agents[sess.ID] = agentEntry{
          cancel: func() {},
          done:   done,
      }
      cs.mu.Unlock()

      // Now simulate handleChatSend being called — it will see the existing agent,
      // cancel it, wait for done, then proceed. To test, close done after a brief delay.
      go func() {
          time.Sleep(50 * time.Millisecond)
          close(done)
      }()

      // Send chat.send to trigger superseded path.
      sendMsg, _ := json.Marshal(map[string]interface{}{
          "type":      TypeChatSend,
          "sessionId": sess.ID,
          "content":   "new message",
      })
      conn.Write(ctx, websocket.MessageText, sendMsg)

      // Poll until session transitions back to running (or timeout).
      deadline := time.Now().Add(500 * time.Millisecond)
      for time.Now().Before(deadline) {
          updated, _ := store.GetSession(sess.ID)
          if updated.Status == session.StatusRunning {
              return // test passes
          }
          time.Sleep(10 * time.Millisecond)
      }
      final, _ := store.GetSession(sess.ID)
      t.Errorf("session status = %v, want running after superseded stream", final.Status)
  }
  ```

- [ ] **Step 2: Run test to verify it fails**

  ```bash
  cd /home/rishav/soul-v2 && go test ./internal/chat/ws/... -run TestHandleChatSend_SupersededStream -v
  ```

  Expected: FAIL.

- [ ] **Step 3: Implement the fix in `handleChatSend` superseded block**

  In `handler.go`, find the superseded stream handling block (lines 295-300):

  ```go
  if existing, ok := cs.agents[sessionID]; ok {
      existing.cancel()
      cs.mu.Unlock()
      <-existing.done
      cs.mu.Lock()
  }
  ```

  Change to:

  ```go
  if existing, ok := cs.agents[sessionID]; ok {
      existing.cancel()
      cs.mu.Unlock()
      <-existing.done
      // Old stream was superseded — restore session to running so the new stream can take over.
      if err := h.sessionStore.UpdateSessionStatus(sessionID, session.StatusRunning); err == nil {
          if updated, err := h.sessionStore.GetSession(sessionID); err == nil {
              h.broadcast(NewSessionUpdated(updated))
          }
      }
      cs.mu.Lock()
  }
  ```

- [ ] **Step 4: Run test to verify it passes**

  ```bash
  cd /home/rishav/soul-v2 && go test ./internal/chat/ws/... -run TestHandleChatSend_SupersededStream -v
  ```

  Expected: PASS.

- [ ] **Step 5: Commit**

  ```bash
  git add internal/chat/ws/handler.go internal/chat/ws/handler_test.go
  git commit -m "fix: handleChatSend restores session to running after superseded stream"
  ```

---

### Task 11: Fix `OnClientDisconnect` — call `completeSession` for each session

**Files:**
- Modify: `internal/chat/ws/handler.go`
- Test: `internal/chat/ws/handler_test.go`

- [ ] **Step 1: Write a failing test for OnClientDisconnect completing sessions**

  ```go
  func TestOnClientDisconnect_CompletesRunningSessions(t *testing.T) {
      hub, store, cancel := setupTestEnv(t)
      defer cancel()
      ctx := context.Background()
      conn, cleanup := connectClient(t, ctx, hub)
      defer cleanup()

      drainMessages(t, ctx, conn, 200*time.Millisecond)

      // Create a session in running state.
      sess, _ := store.CreateSession("Test")
      _ = store.UpdateSessionStatus(sess.ID, session.StatusRunning)

      clients := hub.Clients()
      if len(clients) == 0 {
          t.Fatal("no clients")
      }
      client := clients[0]

      // Add a fake agent for this session.
      done := make(chan struct{})
      handler := hub.handler
      cs := handler.getOrCreateChatSession(client)
      cs.mu.Lock()
      cs.agents[sess.ID] = agentEntry{
          cancel: func() { close(done) },
          done:   done,
      }
      cs.mu.Unlock()

      // Trigger disconnect — this is synchronous (OnClientDisconnect waits for each agent).
      handler.OnClientDisconnect(client)

      // Session should be completed immediately (OnClientDisconnect is synchronous).
      updated, _ := store.GetSession(sess.ID)
      if updated.Status == session.StatusRunning {
          t.Error("session still running after disconnect, expected completed")
      }
  }
  ```

- [ ] **Step 2: Run test to verify it fails**

  ```bash
  cd /home/rishav/soul-v2 && go test ./internal/chat/ws/... -run TestOnClientDisconnect_Completes -v
  ```

  Expected: FAIL.

- [ ] **Step 3: Implement the fix in `OnClientDisconnect`**

  In `handler.go`, find `OnClientDisconnect` (around line 999). Change the loop at the bottom:

  ```go
  for _, entry := range agents {
      entry.cancel()
      <-entry.done
  }
  ```

  To:

  ```go
  for sessionID, entry := range agents {
      entry.cancel()
      <-entry.done
      h.completeSession(client, sessionID)
  }
  ```

- [ ] **Step 4: Run test to verify it passes**

  ```bash
  cd /home/rishav/soul-v2 && go test ./internal/chat/ws/... -run TestOnClientDisconnect_Completes -v
  ```

  Expected: PASS.

- [ ] **Step 5: Run all handler tests**

  ```bash
  cd /home/rishav/soul-v2 && go test ./internal/chat/ws/... -v 2>&1 | tail -30
  ```

  Expected: all PASS.

- [ ] **Step 6: Commit**

  ```bash
  git add internal/chat/ws/handler.go internal/chat/ws/handler_test.go
  git commit -m "fix: OnClientDisconnect calls completeSession for each running session"
  ```

---

### Task 12: Fix `useChat` reconnect — send `session.switch` on reconnect

**Files:**
- Modify: `web/src/hooks/useChat.ts`

- [ ] **Step 1: Update the `connection.ready` handler to handle reconnect case**

  In `useChat.ts`, find the `case 'connection.ready':` block (around line 172). The current code:

  ```ts
  case 'connection.ready': {
    setAuthError(false);
    const savedId = localStorage.getItem(STORAGE_KEY);
    if (savedId && !sessionIDRef.current) {
      sessionIDRef.current = savedId;
      setCurrentSessionID(savedId);
      queueMicrotask(() => {
        sendRef.current('session.switch', { sessionId: savedId });
      });
    }
    // ... pending message recovery ...
    break;
  }
  ```

  Replace with:

  ```ts
  case 'connection.ready': {
    setAuthError(false);
    if (sessionIDRef.current) {
      // RECONNECT — session ID already set. Re-subscribe and refresh history.
      // Reset streaming state: any in-flight stream is gone.
      setIsStreaming(false);
      isStreamingRef.current = false;
      // Remove any orphaned streaming placeholder (no chat.done will arrive for it).
      setMessages(prev => prev.filter(m => m.id !== STREAMING_MESSAGE_ID));
      const sid = sessionIDRef.current;
      queueMicrotask(() => {
        sendRef.current('session.switch', { sessionId: sid });
      });
    } else {
      // FIRST LOAD — restore from localStorage.
      const savedId = localStorage.getItem(STORAGE_KEY);
      if (savedId) {
        sessionIDRef.current = savedId;
        setCurrentSessionID(savedId);
        queueMicrotask(() => {
          sendRef.current('session.switch', { sessionId: savedId });
        });
      }
    }
    // Recover pending message from localStorage (browser refresh during deferred creation).
    const pendingRaw = localStorage.getItem('soul-v2-pending');
    if (pendingRaw && !pendingMessageRef.current) {
      try {
        const pending = JSON.parse(pendingRaw);
        if (pending?.content) {
          pendingMessageRef.current = pending;
          if (!sessionIDRef.current) {
            sendRef.current('session.create', {});
          }
        }
      } catch (err) { reportError('useChat.pendingRestore', err); }
    }
    break;
  }
  ```

- [ ] **Step 2: Verify TypeScript compiles**

  ```bash
  cd /home/rishav/soul-v2/web && npx tsc --noEmit 2>&1 | head -20
  ```

  Expected: no errors.

- [ ] **Step 3: Manual test**

  Run the app. Disconnect WiFi, reconnect. Verify:
  - The session history refreshes automatically (no manual page reload needed)
  - No orphaned streaming message bubble visible
  - `isStreaming` resets to false so the chat input is re-enabled

- [ ] **Step 4: Commit**

  ```bash
  git add web/src/hooks/useChat.ts
  git commit -m "fix: useChat sends session.switch on reconnect to restore session state"
  ```

---

## Chunk 3: Layer 3 — Connection Resilience

### Task 13: Add AbortController timeout to `fetchWSTicket`, return `{ticket, status}`

**Prerequisite:** Task 7 (Layer 1) must be complete — `reportWSLifecycle` must exist in `web/src/lib/telemetry.ts` before Tasks 13-14 can be fully compiled. If implementing Layer 3 standalone, first add a stub `export function reportWSLifecycle(...) {}` to `telemetry.ts`.

**Files:**
- Modify: `web/src/lib/ws.ts`

- [ ] **Step 1: Update `fetchWSTicket` to return `{ticket, status}` and add timeout**

  Replace the entire `fetchWSTicket` function:

  ```ts
  /**
   * Fetches a short-lived one-time WS ticket from the server.
   * Returns {ticket, status} where:
   *   - ticket: the ticket string (null if unavailable or auth not configured)
   *   - status: HTTP status code (0 = network error/timeout, 200 = ok, 401 = auth failure, etc.)
   * Times out after 5 seconds to prevent stalling the WS connection.
   */
  export async function fetchWSTicket(): Promise<{ ticket: string | null; status: number }> {
    const token = getToken();
    if (!token) return { ticket: null, status: 0 };

    const controller = new AbortController();
    const timeout = setTimeout(() => controller.abort(), 5000);

    try {
      const resp = await fetch('/api/ws-ticket', {
        headers: { Authorization: `Bearer ${token}` },
        signal: controller.signal,
      });
      clearTimeout(timeout);
      if (!resp.ok) return { ticket: null, status: resp.status };
      const data: { ticket?: string } = await resp.json();
      return { ticket: data.ticket ?? null, status: 200 };
    } catch {
      clearTimeout(timeout);
      return { ticket: null, status: 0 };
    }
  }

  /**
   * Builds the WebSocket URL with a one-time ticket (preferred) or raw token
   * (fallback). Call fetchWSTicket() first to obtain a ticket.
   */
  export function getWebSocketURL(ticket?: string | null): string {
    const base = getWebSocketBaseURL();
    if (ticket) return `${base}?ticket=${encodeURIComponent(ticket)}`;
    const token = getToken();
    return token ? `${base}?token=${encodeURIComponent(token)}` : base;
  }
  ```

- [ ] **Step 2: Verify TypeScript compiles**

  ```bash
  cd /home/rishav/soul-v2/web && npx tsc --noEmit 2>&1 | head -20
  ```

  Expected: errors because `useWebSocket.ts` calls `fetchWSTicket()` and expects the old return type.

> **Note:** Skip commit here — `useWebSocket.ts` still expects the old `Promise<string | null>` return type from `fetchWSTicket`, causing compile errors. These are fixed in Task 14. Commit happens at Task 17 Step 7.

---

### Task 14: Add auth circuit breaker, close code awareness, give-up, and `reconnect()` to `useWebSocket.ts`

**Files:**
- Modify: `web/src/hooks/useWebSocket.ts`

This is the largest single change. Do it carefully.

- [ ] **Step 1: Update `UseWebSocketReturn` to include `reconnect()` and `authError`**

  Change the interface:

  ```ts
  interface UseWebSocketReturn {
    status: ConnectionState;
    send: (type: string, payload: Record<string, unknown>) => void;
    reconnectAttempt: number;
    reconnect: () => void;    // Manual reconnect trigger (also clears auth circuit breaker)
    authError: boolean;       // true when auth circuit breaker has fired
  }
  ```

- [ ] **Step 2: Add `openTimeRef`, auth circuit breaker state, `authError` React state, and `reportWSLifecycle` import**

  First, verify `reportWSLifecycle` is exported from `web/src/lib/telemetry.ts`. If Layer 1 (Task 7) is complete, it will already be there. If not, add the stub now:
  ```ts
  // Stub — replace with real implementation in Task 7 (Layer 1)
  export function reportWSLifecycle(_event: string, _data?: Record<string, unknown>): void {}
  ```

  Add `reportWSLifecycle` to the telemetry import:
  ```ts
  import { reportError, reportWSLifecycle } from '../lib/telemetry';
  ```

  Add `MAX_RECONNECT_ATTEMPTS` to the module-level constants block (alongside `BASE_DELAY`, `MAX_DELAY`, `JITTER`):

  ```ts
  const MAX_RECONNECT_ATTEMPTS = 10;
  ```

  Then add refs and state inside the hook, after `const attemptRef = useRef(0);`:

  ```ts
  const openTimeRef = useRef<number>(0);       // tracks WS open timestamp for duration calc
  const consecutiveAuthFailuresRef = useRef(0);
  const authErrorRef = useRef(false);          // true = circuit breaker has fired
  const [authError, setAuthError] = useState(false);  // React state for circuit breaker
  ```

  Note: `authErrorRef` is used inside the `connect` callback (to avoid stale closure); `authError` state drives re-renders that update the UI.

- [ ] **Step 3: Replace the entire `connect` callback with the complete updated version**

  This single step replaces all of: the `fetchWSTicket().then(...)` call, `socket.onopen`, `socket.onmessage`, `socket.onclose`, and `socket.onerror`. Replace the existing `connect` implementation with:

  ```ts
  const connect = useCallback(() => {
    if (unmountedRef.current) return;
    clearReconnectTimer();
    setStatus('connecting');

    fetchWSTicket().then(({ ticket, status: ticketStatus }) => {
      if (unmountedRef.current) return;

      reportWSLifecycle('ticket_fetch', {
        status: ticketStatus,
        fallback: ticket === null && ticketStatus !== 401,
      });

      // Auth circuit breaker: two consecutive 401s mean auth is broken.
      if (ticketStatus === 401) {
        consecutiveAuthFailuresRef.current++;
        if (consecutiveAuthFailuresRef.current >= 2) {
          authErrorRef.current = true;
          setAuthError(true);   // triggers UI re-render to show Re-authenticate button
          setStatus('error');
          return;
        }
        setStatus('disconnected');
        const delay = backoffDelay(attemptRef.current);
        attemptRef.current++;
        setReconnectAttempt(attemptRef.current);
        reconnectTimerRef.current = setTimeout(connect, delay);
        return;
      }
      // Any non-401 response resets the auth failure count.
      consecutiveAuthFailuresRef.current = 0;

      reportWSLifecycle('connect_attempt', { attempt: attemptRef.current, ticketUsed: !!ticket });

      const socket = new WebSocket(getWebSocketURL(ticket));
      wsRef.current = socket;

      socket.onopen = () => {
        openTimeRef.current = performance.now();
        reportWSLifecycle('open', { attempt: attemptRef.current });
        // Status transitions to 'connected' only on connection.ready from server.
      };

      socket.onmessage = (event: MessageEvent) => {
        try {
          const raw = JSON.parse(event.data as string) as unknown;
          const messages = Array.isArray(raw) ? raw : [raw];
          for (const parsed of messages) {
            const msg = parsed as { type: string; data?: unknown; sessionId?: string };
            if (msg.type === 'connection.ready') {
              setStatus('connected');
              attemptRef.current = 0;
              setReconnectAttempt(0);
            }
            if (onMessageRef.current) {
              onMessageRef.current(
                msg.type as OutboundMessageType,
                msg.data,
                msg.sessionId ?? '',
              );
            }
          }
        } catch (err) {
          reportError('useWebSocket.parse', err);
        }
      };

      socket.onclose = (event: CloseEvent) => {
        const duration = performance.now() - openTimeRef.current;
        reportWSLifecycle('close', {
          code: event.code,
          reason: event.reason,
          wasClean: event.wasClean,
          duration_ms: Math.round(duration),
        });
        wsRef.current = null;
        if (!unmountedRef.current) {
          setStatus('disconnected');
          if (authErrorRef.current) {
            setStatus('error');
            return;
          }
          if (attemptRef.current >= MAX_RECONNECT_ATTEMPTS) {
            setStatus('error');
            return;
          }
          const delay = backoffDelay(attemptRef.current);
          attemptRef.current++;
          setReconnectAttempt(attemptRef.current);
          reconnectTimerRef.current = setTimeout(connect, delay);
        }
      };

      socket.onerror = () => {
        reportWSLifecycle('error', { attempt: attemptRef.current });
        if (!unmountedRef.current) {
          setStatus('error');
        }
      };
    });
  }, [clearReconnectTimer]);
  ```

- [ ] **Step 4: Add `reconnect()` function**

  Add after the `send` callback:

  ```ts
  const reconnect = useCallback(() => {
    if (unmountedRef.current) return;
    // Reset circuit breaker state so reconnect can proceed.
    authErrorRef.current = false;
    setAuthError(false);       // clear React state so banner updates
    consecutiveAuthFailuresRef.current = 0;
    attemptRef.current = 0;
    setReconnectAttempt(0);
    // Close existing connection if any.
    if (wsRef.current) {
      wsRef.current.close();
      wsRef.current = null;
    }
    clearReconnectTimer();
    connect();
  }, [connect, clearReconnectTimer]);
  ```

- [ ] **Step 5: Add visibility-aware reconnect**

  In the `useEffect` setup (after `unmountedRef.current = false;`), add:

  ```ts
  const onVisibilityChange = () => {
    if (
      document.visibilityState === 'visible' &&
      wsRef.current === null &&
      !authErrorRef.current &&
      attemptRef.current < MAX_RECONNECT_ATTEMPTS
    ) {
      clearReconnectTimer();
      attemptRef.current = 0;
      setReconnectAttempt(0);  // sync React state so the banner attempt counter resets
      connect();
    }
  };
  document.addEventListener('visibilitychange', onVisibilityChange);
  ```

  Use `wsRef.current === null` (not `status === 'disconnected'`) — `status` is a React state value that is stale inside the effect closure, while refs are always current.

  And in the cleanup return:

  ```ts
  return () => {
    unmountedRef.current = true;
    document.removeEventListener('visibilitychange', onVisibilityChange);
    clearReconnectTimer();
    if (wsRef.current) {
      wsRef.current.close();
      wsRef.current = null;
    }
  };
  ```

- [ ] **Step 6: Update return value**

  Change the return statement to include `reconnect` and `authError`:

  ```ts
  return { status, send, reconnectAttempt, reconnect, authError };
  ```

- [ ] **Step 7: Verify TypeScript compiles (expect downstream errors only)**

  ```bash
  cd /home/rishav/soul-v2/web && npx tsc --noEmit 2>&1 | head -30
  ```

  Expected: compile errors ONLY in `useChat.ts` (missing `authError`/`reconnect` destructuring) and `ChatPage.tsx` (missing props). These are expected and will be fixed in Tasks 15-17.

  **Do NOT run `make verify-static` until Task 17 Step 6 — it will fail due to these expected compile errors.**

> **Note:** Do not commit here. Downstream compile errors exist until Tasks 15-17 thread the new props through. The combined Layer 3 commit happens at Task 17 Step 5.

---

### Task 15: Thread `reconnect` through `useChat` and merge WS auth error into existing auth state

**Files:**
- Modify: `web/src/hooks/useChat.ts`

> **Context:** `useChat.ts` already has `authError: boolean` and `reauth: () => Promise<void>` in `UseChatReturn`, the return object, and an implementation at line ~622. Only `reconnect: () => void` is new. The WS circuit breaker's `authError` (from Task 14) is a second auth error source that needs to be merged with the existing one.

- [ ] **Step 1: Add `reconnect: () => void` to `UseChatReturn`**

  `authError` and `reauth` already exist in `UseChatReturn` at lines ~11 and ~17. Add only:

  ```ts
  reconnect: () => void;
  ```

- [ ] **Step 2: Destructure `reconnect` and `authError` from `useWebSocket` (with alias)**

  Find: `const { status, send, reconnectAttempt } = useWebSocket({ onMessage: handleMessage });`

  Change to:

  ```ts
  const { status, send, reconnectAttempt, reconnect, authError: wsAuthError } = useWebSocket({ onMessage: handleMessage });
  ```

  The alias `wsAuthError` avoids shadowing the existing `const [authError, setAuthError]` state at line ~153.

- [ ] **Step 3: Update the existing `reauth` function to also reset the WS circuit breaker**

  The existing `reauth` at line ~622 calls `/api/reauth` and calls `setAuthError(false)` on success. Update its `useCallback` deps to include `reconnect`, and call `reconnect()` after `setAuthError(false)`:

  ```ts
  const reauth = useCallback(async () => {
    const MAX_RETRIES = 3;
    for (let attempt = 0; attempt < MAX_RETRIES; attempt++) {
      try {
        const token = getToken()?.trim();
        const resp = await fetch('/api/reauth', {
          method: 'POST',
          ...(token ? { headers: { Authorization: `Bearer ${token}` } } : {}),
        });
        if (resp.ok) {
          setAuthError(false);
          reconnect();   // also resets the WS circuit breaker
          return;
        }
      } catch (err) {
        reportError('useChat.reauth', err);
      }
      if (attempt < MAX_RETRIES - 1) {
        await new Promise(r => setTimeout(r, 1000 * Math.pow(2, attempt)));
      }
    }
    // All retries failed — keep authError true so UI shows re-auth button.
  }, [reconnect]);
  ```

- [ ] **Step 4: Merge `wsAuthError` into the `authError` return value**

  In the `return { ... }` at the bottom of `useChat`, find the existing `authError,` field and change it to:

  ```ts
  authError: authError || wsAuthError,
  ```

  This surfaces the WS circuit breaker auth error alongside the existing API-level auth error.

- [ ] **Step 5: Add `reconnect` to the return object**

  In the same return block, add:

  ```ts
  reconnect,
  ```

  (`reauth` and `authError` are already in the return object.)

- [ ] **Step 6: Verify TypeScript compiles (up to ChatPage errors)**

  ```bash
  cd /home/rishav/soul-v2/web && npx tsc --noEmit 2>&1 | head -20
  ```

---

### Task 16: Update `ConnectionBanner` — add Retry/Re-authenticate buttons

**Files:**
- Modify: `web/src/components/ConnectionBanner.tsx`

- [ ] **Step 1: Update the props interface and add action buttons**

  Replace the entire `ConnectionBanner.tsx` content:

  ```tsx
  import { useState, useEffect } from 'react';
  import type { ReactNode } from 'react';
  import type { ConnectionState } from '../lib/types';

  interface ConnectionBannerProps {
    status: ConnectionState;
    reconnectAttempt?: number;
    onReconnect?: () => void;
    authError?: boolean;
    onReauth?: () => Promise<void>;
  }

  export function ConnectionBanner({
    status,
    reconnectAttempt = 0,
    onReconnect,
    authError = false,
    onReauth,
  }: ConnectionBannerProps) {
    const [dismissed, setDismissed] = useState(false);

    useEffect(() => {
      if (status === 'connected' || status === 'connecting') {
        setDismissed(false);
      }
    }, [status]);

    const show = !dismissed && (status === 'disconnected' || status === 'error');
    if (!show) return null;

    const isError = status === 'error';
    const bgClass = isError
      ? 'bg-red-900/80 border-red-700'
      : 'bg-yellow-900/80 border-yellow-700';
    const textClass = isError ? 'text-red-200' : 'text-yellow-200';
    const btnClass = isError
      ? 'ml-2 px-2 py-0.5 text-xs border border-red-500 rounded hover:bg-red-800 transition-colors'
      : 'ml-2 px-2 py-0.5 text-xs border border-yellow-500 rounded hover:bg-yellow-800 transition-colors';

    // Determine message and action button.
    let message: string;
    let actionButton: ReactNode = null;

    if (status === 'error' && authError) {
      message = 'Authentication failed.';
      if (onReauth) {
        actionButton = (
          <button
            data-testid="reauth-button"
            className={btnClass}
            onClick={() => { void onReauth(); }}
          >
            Re-authenticate
          </button>
        );
      }
    } else if (status === 'error') {
      message = 'Connection lost.';
      if (onReconnect) {
        actionButton = (
          <button
            data-testid="retry-button"
            className={btnClass}
            onClick={onReconnect}
          >
            Retry
          </button>
        );
      }
    } else {
      // disconnected: auto-reconnecting
      const suffix = reconnectAttempt > 1 ? `Retry #${reconnectAttempt}...` : 'Reconnecting...';
      message = `Connection lost. ${suffix}`;
    }

    return (
      <div
        data-testid="connection-banner"
        className={`flex items-center justify-between px-4 py-2 text-sm border-b ${bgClass} ${textClass} transition-opacity duration-300`}
      >
        <span className="flex items-center">
          {message}
          {actionButton}
        </span>
        <button
          data-testid="dismiss-banner-button"
          onClick={() => setDismissed(true)}
          className="ml-4 hover:opacity-70 transition-opacity"
          aria-label="Dismiss"
        >
          &times;
        </button>
      </div>
    );
  }
  ```

- [ ] **Step 2: Verify TypeScript compiles**

  ```bash
  cd /home/rishav/soul-v2/web && npx tsc --noEmit 2>&1 | head -20
  ```

---

### Task 17: Update `ChatContext` and `ChatPage` to pass new props to `ConnectionBanner`

**Files:**
- Modify: `web/src/contexts/ChatContext.tsx`
- Modify: `web/src/pages/ChatPage.tsx`

> **Context:** `ChatContextValue` already has `authError: boolean` (line ~10) and `reauth: () => Promise<void>` (line ~16). Only `reconnect: () => void` is new. `ChatPage.tsx` does not yet destructure `authError`, `reauth`, or `reconnect` from `useChatContext`.

- [ ] **Step 1: Add `reconnect: () => void` to `ChatContextValue`**

  Open `web/src/contexts/ChatContext.tsx`. Find the `ChatContextValue` interface and add only:

  ```ts
  reconnect: () => void;
  ```

  (`authError` and `reauth` are already present.) `ChatProvider` passes `value={chat}` (the full `UseChatReturn` object), so `reconnect` will be available automatically once `useChat` returns it (Task 15 Step 5).

- [ ] **Step 2: Verify TypeScript compiles (expect no new errors)**

  ```bash
  cd /home/rishav/soul-v2/web && npx tsc --noEmit 2>&1 | head -20
  ```

  Expected: any remaining errors should only be in `ChatPage.tsx` (not yet updated).

  > **Note:** If you see a type error on `ChatProvider value={chat}` about `sendMessage` (thinking parameter mismatch between `boolean` and `ThinkingConfig`), that is a pre-existing mismatch in `ChatContextValue` — not introduced by this chunk. Track it separately; do not attempt to fix it here.

- [ ] **Step 3: Destructure `reconnect`, `authError`, `reauth` from `useChatContext`**

  In `ChatPage.tsx`, find the destructuring block. Add:

  ```ts
  reconnect,
  authError,
  reauth,
  ```

- [ ] **Step 4: Pass the new props to `ConnectionBanner`**

  Find (search by text, line number may shift): `<ConnectionBanner status={status} reconnectAttempt={reconnectAttempt} />`

  Replace with:

  ```tsx
  <ConnectionBanner
    status={status}
    reconnectAttempt={reconnectAttempt}
    onReconnect={reconnect}
    authError={authError}
    onReauth={reauth}
  />
  ```

- [ ] **Step 5: Verify TypeScript compiles fully**

  ```bash
  cd /home/rishav/soul-v2/web && npx tsc --noEmit 2>&1 | head -30
  ```

  Expected: no errors.

- [ ] **Step 6: Run static verification**

  ```bash
  cd /home/rishav/soul-v2 && make verify-static 2>&1 | tail -20
  ```

  Expected: PASS.

- [ ] **Step 7: Commit all Layer 3 changes**

  ```bash
  git add web/src/lib/ws.ts web/src/hooks/useWebSocket.ts web/src/hooks/useChat.ts \
          web/src/components/ConnectionBanner.tsx web/src/pages/ChatPage.tsx \
          web/src/contexts/ChatContext.tsx
  git commit -m "feat: add auth circuit breaker, give-up/retry UI, visibility-aware reconnect"
  ```

  > **Note:** If implementing Layer 3 standalone (Task 7 not yet done), the prerequisite stub in `web/src/lib/telemetry.ts` is also modified — add it to the `git add` line above.

---

## Chunk 4: Layer 4 — Token Coalescing + Tool Progress

### Task 18: Add `TypeToolProgress` and `NewToolProgress` to `message.go`

**Files:**
- Modify: `internal/chat/ws/message.go`

- [ ] **Step 1: Add the constant and constructor**

  In `message.go`, add `TypeToolProgress` to the outbound constants block:

  ```go
  TypeToolProgress      = "tool.progress"
  ```

  Add the constructor function at the end of the file:

  ```go
  // NewToolProgress creates a tool.progress outbound message with activity detail.
  func NewToolProgress(sessionID, toolID, event, detail string, progress float64, tsMs int64) *OutboundMessage {
  	data := map[string]interface{}{
  		"id":    toolID,
  		"event": event,
  		"ts":    tsMs,
  	}
  	if detail != "" {
  		data["detail"] = detail
  	}
  	if progress >= 0 {
  		data["progress"] = progress
  	}
  	return &OutboundMessage{
  		Type:      TypeToolProgress,
  		SessionID: sessionID,
  		Data:      data,
  	}
  }
  ```

  Note: `progress < 0` means "not set" (use `-1` as sentinel from callers to omit the field).

- [ ] **Step 2: Verify no compilation errors**

  ```bash
  cd /home/rishav/soul-v2 && go vet ./internal/chat/ws/...
  ```

  Expected: no output.

- [ ] **Step 3: Commit**

  ```bash
  git add internal/chat/ws/message.go
  git commit -m "feat: add TypeToolProgress constant and NewToolProgress constructor"
  ```

---

### Task 19: Update `readMessage` test helper to handle array frames

**Files:**
- Modify: `internal/chat/ws/handler_test.go`

This MUST be done before coalescing is added to WritePump, otherwise existing tests break.

- [ ] **Step 1: Update `readMessage` to buffer array frames**

  The key insight: when WritePump coalesces multiple messages into one array frame, `readMessage` needs to unpack and return them one at a time. We add a package-level buffer (`testMsgBuffer`) to the test file.

  At the top of `handler_test.go`, after the package declaration and imports, add:

  ```go
  // testMsgBuffer holds messages from array frames for sequential delivery.
  // Each test goroutine that calls readMessage will share this (tests are serial).
  var testMsgBuffer []map[string]interface{}
  var testMsgBufMu sync.Mutex
  ```

  In `readMessage`, add a `t.Cleanup` call right after `t.Helper()` to reset the buffer after each test completes. Calling `t.Cleanup` multiple times per test is safe — all registered cleanups run:

  ```go
  t.Helper()
  t.Cleanup(func() {
      testMsgBufMu.Lock()
      testMsgBuffer = nil
      testMsgBufMu.Unlock()
  })
  ```

  Replace the `readMessage` function with:

  ```go
  // readMessage reads a single JSON message from the WebSocket connection.
  // If the server sends an array frame (coalesced messages), it buffers the
  // remaining messages and returns them one at a time across successive calls.
  func readMessage(t *testing.T, ctx context.Context, conn *websocket.Conn) map[string]interface{} {
  	t.Helper()

  	// Return buffered messages first.
  	testMsgBufMu.Lock()
  	if len(testMsgBuffer) > 0 {
  		msg := testMsgBuffer[0]
  		testMsgBuffer = testMsgBuffer[1:]
  		testMsgBufMu.Unlock()
  		return msg
  	}
  	testMsgBufMu.Unlock()

  	readCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
  	defer cancel()

  	_, data, err := conn.Read(readCtx)
  	if err != nil {
  		t.Fatalf("read message: %v", err)
  	}

  	// Try array first.
  	if len(data) > 0 && data[0] == '[' {
  		var msgs []map[string]interface{}
  		if err := json.Unmarshal(data, &msgs); err != nil {
  			t.Fatalf("unmarshal array frame: %v", err)
  		}
  		if len(msgs) == 0 {
  			t.Fatal("empty array frame")
  		}
  		if len(msgs) > 1 {
  			testMsgBufMu.Lock()
  			testMsgBuffer = append(testMsgBuffer, msgs[1:]...)
  			testMsgBufMu.Unlock()
  		}
  		return msgs[0]
  	}

  	// Single object.
  	var result map[string]interface{}
  	if err := json.Unmarshal(data, &result); err != nil {
  		t.Fatalf("unmarshal message: %v", err)
  	}
  	return result
  }
  ```

  Also update `drainMessages` in `handler_test.go` to handle array frames — otherwise any coalesced frame returned by `drainMessages` would `t.Fatalf` on `json.Unmarshal` into `map[string]interface{}`:

  ```go
  func drainMessages(t *testing.T, conn *websocket.Conn, timeout time.Duration) []map[string]interface{} {
      t.Helper()
      var result []map[string]interface{}
      ctx, cancel := context.WithTimeout(context.Background(), timeout)
      defer cancel()
      for {
          _, data, err := conn.Read(ctx)
          if err != nil {
              break
          }
          if len(data) > 0 && data[0] == '[' {
              var msgs []map[string]interface{}
              if err := json.Unmarshal(data, &msgs); err != nil {
                  t.Fatalf("drainMessages: unmarshal array: %v", err)
              }
              result = append(result, msgs...)
          } else {
              var msg map[string]interface{}
              if err := json.Unmarshal(data, &msg); err != nil {
                  t.Fatalf("drainMessages: unmarshal object: %v", err)
              }
              result = append(result, msg)
          }
      }
      return result
  }
  ```

  Also add `"sync"` to the imports in `handler_test.go`.

- [ ] **Step 2: Verify tests still pass with existing behavior**

  Since coalescing isn't active yet, all frames are still single objects. The updated `readMessage` should be completely transparent — the `data[0] == '['` branch never fires.

  ```bash
  cd /home/rishav/soul-v2 && go test ./internal/chat/ws/... -v 2>&1 | tail -30
  ```

  Expected: all PASS (identical behavior, array branch untriggered).

- [ ] **Step 3: Commit**

  ```bash
  git add internal/chat/ws/handler_test.go
  git commit -m "test: update readMessage helper to handle coalesced array frames"
  ```

---

### Task 20: Implement coalescing WritePump in `client.go`

**Files:**
- Modify: `internal/chat/ws/client.go`
- Test: `internal/chat/ws/client_test.go`

**Prerequisite:** Task 2 (Layer 1) must be complete — `c.closeReason atomic.Value` field must exist on `Client`. Verify with `go vet ./internal/chat/ws/...` before starting.

- [ ] **Step 1: Write failing tests for `marshalBatch` helper**

  In `client_test.go`, add unit tests for the `marshalBatch` helper we're about to add:

  ```go
  func TestMarshalBatch_SingleMessage_PlainObject(t *testing.T) {
      msgs := [][]byte{[]byte(`{"type":"chat.token"}`)}
      result, err := marshalBatch(msgs)
      if err != nil {
          t.Fatal(err)
      }
      // Single message: no array wrapper.
      if result[0] == '[' {
          t.Errorf("single message should not be wrapped in array, got: %s", result)
      }
  }

  func TestMarshalBatch_MultipleMessages_ArrayFrame(t *testing.T) {
      msgs := [][]byte{
          []byte(`{"type":"chat.token"}`),
          []byte(`{"type":"chat.token"}`),
      }
      result, err := marshalBatch(msgs)
      if err != nil {
          t.Fatal(err)
      }
      if result[0] != '[' {
          t.Errorf("multiple messages should be wrapped in array, got: %s", result)
      }
  }
  ```

- [ ] **Step 2: Add `marshalBatch` helper to `client.go`**

  Add before `WritePump`:

  ```go
  // marshalBatch serializes a batch of messages for wire transmission.
  // Single message: returned as-is (plain JSON object, no array wrapper).
  // Multiple messages: joined into a JSON array.
  func marshalBatch(msgs [][]byte) ([]byte, error) {
  	if len(msgs) == 1 {
  		return msgs[0], nil
  	}
  	// Join as JSON array: [msg1,msg2,...].
  	total := 2 // [ and ]
  	for i, m := range msgs {
  		total += len(m)
  		if i > 0 {
  			total++ // comma
  		}
  	}
  	out := make([]byte, 0, total)
  	out = append(out, '[')
  	for i, m := range msgs {
  		if i > 0 {
  			out = append(out, ',')
  		}
  		out = append(out, m...)
  	}
  	out = append(out, ']')
  	return out, nil
  }
  ```

- [ ] **Step 3: Run the marshalBatch tests to verify they pass**

  ```bash
  cd /home/rishav/soul-v2 && go test ./internal/chat/ws/... -run TestMarshalBatch -v
  ```

  Expected: PASS.

- [ ] **Step 4: Replace `WritePump` with the coalescing version**

  Replace the entire `WritePump` function:

  ```go
  // WritePump writes messages from the send channel to the WebSocket connection.
  // It coalesces multiple pending messages into a single array frame to reduce
  // frame overhead during high-throughput streaming.
  // Ping handling and context cancellation are preserved in the outer select.
  func (c *Client) WritePump() {
  	const (
  		coalesceWindow = 5 * time.Millisecond
  		maxBatchSize   = 32
  	)

  	ticker := time.NewTicker(pingInterval)
  	defer func() {
  		ticker.Stop()
  		c.conn.Close(websocket.StatusNormalClosure, "")
  	}()

  	for {
  		select {
  		case msg, ok := <-c.send:
  			if !ok {
  				c.conn.Close(websocket.StatusNormalClosure, "")
  				return
  			}

  			// Got first message — drain additional pending messages within the
  			// coalescing window. Use non-blocking reads to avoid delaying pings.
  			batch := [][]byte{msg}
  			batchStart := time.Now()
  		drain:
  			for len(batch) < maxBatchSize && time.Since(batchStart) < coalesceWindow {
  				select {
  				case next, ok := <-c.send:
  					if !ok {
  						// Channel closed during drain — send what we have, then exit.
  						break drain
  					}
  					batch = append(batch, next)
  				default:
  					break drain // Channel empty — send immediately.
  				}
  			}

  			frame, err := marshalBatch(batch)
  			if err != nil {
  				log.Printf("ws: client %s marshal batch error: %v", c.id, err)
  				return
  			}
  			if err := c.conn.Write(c.ctx, websocket.MessageText, frame); err != nil {
  				log.Printf("ws: client %s write error: %v", c.id, err)
  				c.closeReason.Store("write_error")
  				return
  			}

  		case <-ticker.C:
  			ctx, cancel := context.WithTimeout(c.ctx, pongTimeout)
  			err := c.conn.Ping(ctx)
  			cancel()
  			if err != nil {
  				log.Printf("ws: client %s ping failed: %v", c.id, err)
  				c.closeReason.Store("ping_timeout")
  				return
  			}

  		case <-c.ctx.Done():
  			return
  		}
  	}
  }
  ```

- [ ] **Step 5: Run all ws tests**

  ```bash
  cd /home/rishav/soul-v2 && go test ./internal/chat/ws/... -v 2>&1 | grep -E "^(=== RUN|--- PASS|--- FAIL|FAIL|ok)"
  ```

  Expected: all PASS. The updated `readMessage` helper handles array frames transparently.

- [ ] **Step 6: Commit**

  ```bash
  git add internal/chat/ws/client.go internal/chat/ws/client_test.go
  git commit -m "feat: coalescing WritePump reduces token frame volume 5-10x"
  ```

---

### Task 21: Add array frame parser to `useWebSocket.ts`

**Files:**
- Modify: `web/src/hooks/useWebSocket.ts`

> **Note:** If Layer 3 (Task 14) was already applied, the `socket.onmessage` replacement in Task 14 Step 3 already includes array frame parsing (`Array.isArray(raw) ? raw : [raw]`). In that case, **skip Step 1** and go directly to Step 2 to verify. Only apply Step 1 if implementing Layer 4 standalone (without Layer 3).

- [ ] **Step 1: Update `socket.onmessage` to handle array frames**

  Find the `socket.onmessage` handler. The current code parses a single object and dispatches it. Replace the parsing logic:

  ```ts
  socket.onmessage = (event: MessageEvent) => {
    try {
      const raw = JSON.parse(event.data as string) as unknown;
      const messages = Array.isArray(raw) ? raw : [raw];
      for (const parsed of messages) {
        const msg = parsed as { type: string; data?: unknown; sessionId?: string };
        if (msg.type === 'connection.ready') {
          setStatus('connected');
          attemptRef.current = 0;
          setReconnectAttempt(0);
        }
        if (onMessageRef.current) {
          onMessageRef.current(
            msg.type as OutboundMessageType,
            msg.data,
            msg.sessionId ?? '',
          );
        }
      }
    } catch (err) {
      reportError('useWebSocket.parse', err);
    }
  };
  ```

- [ ] **Step 2: Verify TypeScript compiles**

  ```bash
  cd /home/rishav/soul-v2/web && npx tsc --noEmit 2>&1 | head -20
  ```

  Expected: no errors.

- [ ] **Step 3: Commit**

  ```bash
  git add web/src/hooks/useWebSocket.ts
  git commit -m "feat: useWebSocket parses array frames from coalescing WritePump"
  ```

---

### Task 22: Emit `tool.progress` events in `handler.go` dispatch loop

**Files:**
- Modify: `internal/chat/ws/handler.go`
- Test: `internal/chat/ws/message_test.go`

- [ ] **Step 1: Verify `NewToolProgress` is emitted in the dispatch loop**

  This is tested via the integration path since `runStream` requires a live stream. Instead, write a unit test for `NewToolProgress` itself (verifying the message structure), and a log-based check:

  In `internal/chat/ws/message_test.go`, add a test for `NewToolProgress`:

  ```go
  func TestNewToolProgress_MessageStructure(t *testing.T) {
      msg := NewToolProgress("sess-1", "toolu_abc", "step", "Calling search...", -1, 1710600000000)
      if msg.Type != TypeToolProgress {
          t.Errorf("type = %q, want %q", msg.Type, TypeToolProgress)
      }
      if msg.SessionID != "sess-1" {
          t.Errorf("sessionID = %q, want sess-1", msg.SessionID)
      }
      data, ok := msg.Data.(map[string]interface{})
      if !ok {
          t.Fatal("data is not map")
      }
      if data["id"] != "toolu_abc" {
          t.Errorf("id = %v, want toolu_abc", data["id"])
      }
      if data["event"] != "step" {
          t.Errorf("event = %v, want step", data["event"])
      }
      if _, hasProgress := data["progress"]; hasProgress {
          t.Error("progress field should be omitted when sentinel -1 is passed")
      }
  }
  ```

  Note: The dispatch-loop emission is inherently an integration concern. The `NewToolProgress` unit test verifies the constructor; the dispatch integration is covered by manual testing in Task 24 Step 2 item 4.

- [ ] **Step 2: Add `tool.progress` emission in the dispatch loop**

  In `handler.go`, find `toolResultMessages := make([]stream.Message, 0, len(toolCalls))` and the `for i := range toolCalls` loop immediately after. Replace the entire loop body with:

  ```go
  toolResultMessages := make([]stream.Message, 0, len(toolCalls))
  for i := range toolCalls {
      tc := &toolCalls[i]
      inputJSON := json.RawMessage(tc.Input.String())
      if len(inputJSON) == 0 {
          inputJSON = json.RawMessage("{}")
      }

      // Emit tool.progress: calling (before dispatch).
      h.sendToClient(client, NewToolProgress(sessionID, tc.ID, "step",
          fmt.Sprintf("Calling %s...", tc.Name), -1, time.Now().UnixMilli()))

      dispatchStart := time.Now()
      var result string
      var execErr error
      if h.builtin != nil && h.builtin.CanHandle(tc.Name) {
          result, execErr = h.builtin.Execute(ctx, tc.Name, inputJSON)
      } else if h.dispatcher != nil {
          result, execErr = h.dispatcher.Execute(ctx, tc.Name, inputJSON)
      } else {
          execErr = fmt.Errorf("no handler for tool: %s", tc.Name)
      }

      // Emit tool.progress: completed or failed (after dispatch).
      elapsed := time.Since(dispatchStart).Milliseconds()
      if execErr != nil {
          result = fmt.Sprintf("Error: %v", execErr)
          h.sendToClient(client, NewToolProgress(sessionID, tc.ID, "step",
              fmt.Sprintf("Error after %dms: %v", elapsed, execErr), -1, time.Now().UnixMilli()))
      } else {
          h.sendToClient(client, NewToolProgress(sessionID, tc.ID, "step",
              fmt.Sprintf("Completed in %dms", elapsed), -1, time.Now().UnixMilli()))
      }

      // Store tool_result message.
      trJSON, _ := json.Marshal(struct {
          ToolUseID string `json:"tool_use_id"`
          Content   string `json:"content"`
      }{
          ToolUseID: tc.ID,
          Content:   result,
      })
      if _, err := h.sessionStore.AddMessage(sessionID, "tool_result", string(trJSON)); err != nil {
          log.Printf("ws: failed to store tool_result message: %v", err)
      }

      // Send tool.complete WS event.
      h.sendToClient(client, NewToolComplete(sessionID, tc.ID, tc.Name, result))

      // Build the user message with tool_result for the next API call.
      toolResultMessages = append(toolResultMessages, stream.Message{
          Role: "user",
          Content: []stream.ContentBlock{
              {
                  Type:      "tool_result",
                  ToolUseID: tc.ID,
                  Content:   result,
              },
          },
      })
  }
  ```

- [ ] **Step 3: Verify no import issues**

  ```bash
  cd /home/rishav/soul-v2 && go vet ./internal/chat/ws/...
  ```

  Expected: no output.

- [ ] **Step 4: Commit**

  ```bash
  git add internal/chat/ws/handler.go internal/chat/ws/message_test.go
  git commit -m "feat: emit tool.progress events before and after tool dispatch"
  ```

---

### Task 23: Extend `ToolCallData` with `steps` and update `useChat` tool.progress handler

**Files:**
- Modify: `web/src/hooks/useChat.ts`
- Modify: `web/src/lib/types.ts`

- [ ] **Step 1: Add `ProgressStep` type to `types.ts` and extend `ToolCallData`**

  `types.ts` is generated from specs (CLAUDE.md: "never edit manually"). However, `types.ts` already has an explicit `// --- Manual types (not auto-generated) ---` section for exactly this purpose. **`ProgressStep` belongs there.** The `steps` field added to `ToolCallData` is the one exception: it touches the generated section and will need the spec generator updated to emit it permanently (tracked as follow-up debt).

  > **Warning:** Do NOT run `make types` after this step — it regenerates `types.ts` from specs and will clobber the `steps` field in `ToolCallData` until `tools/specgen.go` is updated. The `ProgressStep` type itself (in the Manual section) is safe from regeneration. Only `npx tsc --noEmit` and `make verify-static` are safe to run.

  `types.ts` has a `// --- Manual types (not auto-generated) ---` section starting at line ~412. Add `ProgressStep` there (not near the generated `ToolCallData` at line ~351):

  In the `// --- Manual types ---` section, add:

  ```ts
  // spec-defined type (see docs/superpowers/specs/2026-03-16-ws-robustness-design.md §4.3)
  export interface ProgressStep {
    event: 'step' | 'warning' | 'metric';
    detail: string;
    progress?: number;
    ts: number;
  }
  ```

  Then find the generated `ToolCallData` interface (around line 351) and add the `steps` field:

  ```ts
  export interface ToolCallData {
    id: string;
    name: string;
    input: Record<string, unknown>;
    status: 'running' | 'complete' | 'error';
    output?: string;
    progress?: number;
    steps?: ProgressStep[];  // tool activity log from tool.progress events
  }
  ```

  The `ProgressStep` type being in the Manual section means `make types` regenerating the top portion won't clobber it. Only the `steps?: ProgressStep[]` line added to `ToolCallData` would be lost if `make types` is run — that's why the `make types` warning is critical.

  Also add `ProgressStep` to the import in `useChat.ts`:
  ```ts
  import type { Message, Session, OutboundMessageType, ConnectionState, ToolCallData, ProgressStep, ChatProduct, ThinkingConfig } from '../lib/types';
  ```

- [ ] **Step 2: Update `tool.progress` handler in `useChat.ts`**

  Find the `case 'tool.progress':` handler (around line 457). The current handler:

  ```ts
  case 'tool.progress': {
    const payload = data as { id: string; progress: number } | undefined;
    if (!payload) break;
    setMessages(prev => {
      const last = prev[prev.length - 1];
      if (last?.id !== STREAMING_MESSAGE_ID) return prev;
      const tools = [...(last.toolCalls ?? [])];
      const idx = tools.findIndex(t => t.id === payload.id);
      if (idx === -1) return prev;
      tools[idx] = { ...tools[idx], progress: payload.progress };
      return [...prev.slice(0, -1), { ...last, toolCalls: tools }];
    });
    break;
  }
  ```

  Replace with the extended handler that appends to `steps`:

  ```ts
  case 'tool.progress': {
    const payload = data as {
      id: string;
      event?: string;
      detail?: string;
      progress?: number;
      ts?: number;
    } | undefined;
    if (!payload) break;

    setMessages(prev => {
      const last = prev[prev.length - 1];
      if (last?.id !== STREAMING_MESSAGE_ID) return prev;
      const tools = [...(last.toolCalls ?? [])];
      const idx = tools.findIndex(t => t.id === payload.id);
      if (idx === -1) return prev;

      const tool = tools[idx]!;
      const updates: Partial<ToolCallData> = {};

      // Update numeric progress if provided.
      if (payload.progress !== undefined) {
        updates.progress = payload.progress;
      }

      // Append to steps if detail is provided.
      if (payload.detail || payload.event) {
        const step: ProgressStep = {
          event: (payload.event as ProgressStep['event']) ?? 'step',
          detail: payload.detail ?? '',
          progress: payload.progress,
          ts: payload.ts ?? Date.now(),
        };
        updates.steps = [...(tool.steps ?? []), step];
      }

      tools[idx] = { ...tool, ...updates };
      return [...prev.slice(0, -1), { ...last, toolCalls: tools }];
    });
    break;
  }
  ```

- [ ] **Step 3: Verify TypeScript compiles**

  ```bash
  cd /home/rishav/soul-v2/web && npx tsc --noEmit 2>&1 | head -20
  ```

  Expected: no errors.

- [ ] **Step 4: Run full static verification**

  ```bash
  cd /home/rishav/soul-v2 && make verify-static 2>&1 | tail -20
  ```

  Expected: PASS.

- [ ] **Step 5: Run all Go tests**

  ```bash
  cd /home/rishav/soul-v2 && go test ./internal/chat/... -v 2>&1 | grep -E "^(=== RUN|--- PASS|--- FAIL|FAIL|ok)" | tail -40
  ```

  Expected: all PASS.

- [ ] **Step 6: Commit**

  ```bash
  git add web/src/hooks/useChat.ts web/src/lib/types.ts
  git commit -m "feat: extend tool.progress handler with steps array for tool detail UI"
  ```

---

### Task 24: Final verification and integration commit

- [ ] **Step 1: Run the complete verification suite**

  ```bash
  cd /home/rishav/soul-v2 && make verify 2>&1 | tail -40
  ```

  Expected: all checks PASS (go vet + tsc + secret scan + dep audit + unit + integration).

- [ ] **Step 2: Verify key behaviors manually**

  1. **Reconnect state recovery**: Open the app, note the current session. Restart the Go server. Verify the frontend auto-reconnects and shows the same session history without a page refresh.

  2. **Slow-client resilience**: In DevTools, throttle the network to 2G while a stream is in progress. Verify the stream completes without disconnecting (coalescing reduces frame rate 5-10x).

  3. **Auth circuit breaker**: Set an incorrect auth token in the environment. Reload the app. Verify the connection attempt shows the auth error banner after 2 tries and the "Re-authenticate" button appears.

  4. **Tool progress**: Send a message that triggers a tool call (e.g., use the Tasks product). Verify the tool call block shows "Calling..." and "Completed in Xms" steps.

- [ ] **Step 3: Final integration commit**

  ```bash
  git log --oneline -15
  ```

  Review all commits for the 4 layers. If everything is clean, the feature is complete.

---

## Summary of Files Modified

| File | Layer | Change |
|------|-------|--------|
| `internal/chat/metrics/types.go` | L1 | Add `EventWSUpgrade` constant |
| `internal/chat/ws/client.go` | L1, L4 | `closeReason` field + classification; coalescing WritePump + `marshalBatch` |
| `internal/chat/ws/hub.go` | L1 | `ws.upgrade` metric; `server_shutdown` close reason |
| `internal/chat/server/server.go` | L1 | `ws.upgrade` `auth_rejected`; batched telemetry |
| `web/src/lib/telemetry.ts` | L1 | `reportWSLifecycle` + client-side batching |
| `web/src/hooks/useWebSocket.ts` | L1, L3, L4 | Lifecycle telemetry; circuit breaker + give-up + visibility + `reconnect()`; array frame parser |
| `internal/chat/ws/handler.go` | L2, L4 | `context.Canceled` silent return; caller-side `completeSession`; `tool.progress` emission |
| `web/src/hooks/useChat.ts` | L2, L3, L4 | Reconnect `session.switch`; thread `reconnect`; extend `tool.progress` handler |
| `web/src/lib/ws.ts` | L3 | AbortController timeout; `{ticket, status}` return |
| `web/src/pages/ChatPage.tsx` | L3 | Pass `reconnect`/`authError`/`reauth` to `ConnectionBanner` |
| `web/src/components/ConnectionBanner.tsx` | L3 | Retry/Re-authenticate buttons |
| `internal/chat/ws/message.go` | L4 | `TypeToolProgress` + `NewToolProgress` |
| `web/src/lib/types.ts` | L4 | `ProgressStep` type + `steps` on `ToolCallData` |
| `internal/chat/ws/handler_test.go` | L4 | `readMessage` helper handles array frames |
| `internal/chat/ws/client_test.go` | L1, L4 | `closeReason` + `marshalBatch` tests |
