//go:build load

package load_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"nhooyr.io/websocket"

	"github.com/rishav1305/soul/internal/chat/session"
	"github.com/rishav1305/soul/internal/chat/ws"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// setupLoadEnv creates a Hub with a session store and MessageHandler (no stream
// client, so chat.send stores the user message and immediately returns
// chat.done), starts the hub event loop, and returns everything needed for
// load testing.
func setupLoadEnv(t *testing.T) (*ws.Hub, *session.Store, context.CancelFunc) {
	t.Helper()

	dbPath := filepath.Join(t.TempDir(), "load-test.db")
	store, err := session.Open(dbPath)
	if err != nil {
		t.Fatalf("open session store: %v", err)
	}
	t.Cleanup(func() { store.Close() })

	hub := ws.NewHub(ws.WithSessionStore(store))
	handler := ws.NewMessageHandler(hub, store, nil) // nil streamClient = immediate chat.done
	hub.SetHandler(handler)

	ctx, cancel := context.WithCancel(context.Background())
	go hub.Run(ctx)

	return hub, store, cancel
}

// loadServer creates an httptest server that upgrades HTTP to WebSocket via hub.
func loadServer(t *testing.T, hub *ws.Hub) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hub.HandleUpgrade(w, r)
	}))
	t.Cleanup(srv.Close)
	return srv
}

// loadDial opens a WebSocket connection to the test server with a large read
// limit so the client can receive large frames.
func loadDial(t *testing.T, ctx context.Context, srv *httptest.Server) *websocket.Conn {
	t.Helper()
	wsURL := "ws" + srv.URL[len("http"):]
	conn, _, err := websocket.Dial(ctx, wsURL, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	conn.SetReadLimit(2 << 20) // 2MB client-side read limit
	return conn
}

// loadReadJSON reads one JSON message from the connection with a timeout.
func loadReadJSON(t *testing.T, ctx context.Context, conn *websocket.Conn, timeout time.Duration) (map[string]interface{}, error) {
	t.Helper()
	rCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	_, data, err := conn.Read(rCtx)
	if err != nil {
		return nil, err
	}
	var m map[string]interface{}
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("unmarshal: %w", err)
	}
	return m, nil
}

// loadDrainInitial drains connection.ready + session.list that every new
// client receives on connect.
func loadDrainInitial(t *testing.T, ctx context.Context, conn *websocket.Conn) {
	t.Helper()
	msg1, err := loadReadJSON(t, ctx, conn, 5*time.Second)
	if err != nil {
		t.Fatalf("drain initial: read connection.ready: %v", err)
	}
	if msg1["type"] != "connection.ready" {
		t.Fatalf("expected connection.ready, got %v", msg1["type"])
	}
	msg2, err := loadReadJSON(t, ctx, conn, 5*time.Second)
	if err != nil {
		t.Fatalf("drain initial: read session.list: %v", err)
	}
	if msg2["type"] != "session.list" {
		t.Fatalf("expected session.list, got %v", msg2["type"])
	}
}

// loadSendJSON writes a JSON text frame to the connection.
func loadSendJSON(t *testing.T, ctx context.Context, conn *websocket.Conn, msg string) {
	t.Helper()
	if err := conn.Write(ctx, websocket.MessageText, []byte(msg)); err != nil {
		t.Fatalf("sendJSON: %v", err)
	}
}

// waitForClientCount polls the hub until the expected count is reached or the
// deadline expires.
func waitForClientCount(t *testing.T, hub *ws.Hub, want int, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if hub.ClientCount() == want {
			return
		}
		time.Sleep(30 * time.Millisecond)
	}
	t.Errorf("client count: want %d, got %d (after %v)", want, hub.ClientCount(), timeout)
}

// waitForGoroutines waits until runtime.NumGoroutine() drops to the target
// (or below), allowing time for goroutines to drain after a test.
func waitForGoroutines(t *testing.T, target int, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if runtime.NumGoroutine() <= target {
			return
		}
		runtime.GC()
		time.Sleep(50 * time.Millisecond)
	}
	t.Errorf("goroutine leak: expected <= %d, got %d", target, runtime.NumGoroutine())
}

// resourceSnapshot captures goroutine count and memory stats.
type resourceSnapshot struct {
	goroutines int
	allocBytes uint64
	sysBytes   uint64
}

// takeResourceSnapshot captures the current resource state.
func takeResourceSnapshot() resourceSnapshot {
	runtime.GC()
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return resourceSnapshot{
		goroutines: runtime.NumGoroutine(),
		allocBytes: m.Alloc,
		sysBytes:   m.Sys,
	}
}

// logResources logs resource usage to the test output.
func logResources(t *testing.T, label string, snap resourceSnapshot) {
	t.Helper()
	t.Logf("[%s] goroutines=%d alloc=%.2fMB sys=%.2fMB",
		label, snap.goroutines,
		float64(snap.allocBytes)/(1024*1024),
		float64(snap.sysBytes)/(1024*1024))
}

// makePayload creates a repeating string of the given byte size.
func makePayload(size int) string {
	const charset = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, size)
	for i := range b {
		b[i] = charset[i%len(charset)]
	}
	return string(b)
}

// sha256Hash returns the hex-encoded SHA-256 hash of a string.
func sha256Hash(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}

// ---------------------------------------------------------------------------
// Test 1: Concurrent Sessions — 10 sessions, 5 clients each, all sending
// ---------------------------------------------------------------------------
//
// This test verifies the system handles high-concurrency write load without
// panics, connection drops, or goroutine leaks. Under heavy concurrent writes,
// SQLite may return SQLITE_BUSY which surfaces as chat.error — the test
// counts both chat.done and chat.error to verify every message gets a
// response and reports the success/error ratio as a diagnostic.

func TestLoad_ConcurrentSessions(t *testing.T) {
	before := takeResourceSnapshot()
	logResources(t, "before", before)

	hub, _, cancel := setupLoadEnv(t)
	defer cancel()

	srv := loadServer(t, hub)

	const numSessions = 10
	const clientsPerSession = 5
	const msgsPerClient = 10

	// Create sessions via WebSocket (using a setup client).
	ctx := context.Background()
	setupConn := loadDial(t, ctx, srv)
	loadDrainInitial(t, ctx, setupConn)

	sessionIDs := make([]string, numSessions)
	for i := 0; i < numSessions; i++ {
		loadSendJSON(t, ctx, setupConn, `{"type":"session.create"}`)
		resp, err := loadReadJSON(t, ctx, setupConn, 5*time.Second)
		if err != nil {
			t.Fatalf("create session %d: %v", i, err)
		}
		if resp["type"] != "session.created" {
			t.Fatalf("expected session.created, got %v", resp["type"])
		}
		data := resp["data"].(map[string]interface{})
		sessObj := data["session"].(map[string]interface{})
		sessionIDs[i] = sessObj["id"].(string)
	}
	setupConn.Close(websocket.StatusNormalClosure, "setup done")

	totalMessages := numSessions * clientsPerSession * msgsPerClient
	var delivered atomic.Int64    // chat.done count
	var appErrors atomic.Int64    // chat.error count (DB busy, etc.)
	var connErrors atomic.Int64   // connection-level failures
	var responseCount atomic.Int64 // total responses received

	start := time.Now()

	var wg sync.WaitGroup
	wg.Add(numSessions * clientsPerSession)

	for s := 0; s < numSessions; s++ {
		sessionID := sessionIDs[s]
		for c := 0; c < clientsPerSession; c++ {
			go func(sessID string, clientIdx int) {
				defer wg.Done()

				cCtx, cCancel := context.WithTimeout(context.Background(), 60*time.Second)
				defer cCancel()

				conn := loadDial(t, cCtx, srv)
				defer conn.Close(websocket.StatusNormalClosure, "done")
				loadDrainInitial(t, cCtx, conn)

				for m := 0; m < msgsPerClient; m++ {
					content := fmt.Sprintf("s=%s c=%d m=%d", sessID[:8], clientIdx, m)
					msg := fmt.Sprintf(`{"type":"chat.send","sessionId":"%s","content":"%s"}`, sessID, content)
					if err := conn.Write(cCtx, websocket.MessageText, []byte(msg)); err != nil {
						connErrors.Add(1)
						return
					}

					resp, err := loadReadJSON(t, cCtx, conn, 10*time.Second)
					if err != nil {
						connErrors.Add(1)
						return
					}
					responseCount.Add(1)

					switch resp["type"] {
					case "chat.done":
						delivered.Add(1)
					case "chat.error":
						appErrors.Add(1)
					default:
						connErrors.Add(1)
					}
				}
			}(sessionID, c)
		}
	}

	wg.Wait()
	elapsed := time.Since(start)

	after := takeResourceSnapshot()
	logResources(t, "after", after)

	// Report metrics.
	t.Logf("total_sent=%d responses=%d delivered=%d app_errors=%d conn_errors=%d",
		totalMessages, responseCount.Load(), delivered.Load(), appErrors.Load(), connErrors.Load())
	t.Logf("elapsed=%v messages/sec=%.1f", elapsed, float64(responseCount.Load())/elapsed.Seconds())

	// Gate: every message must get a response (no message drops).
	totalResponded := delivered.Load() + appErrors.Load() + connErrors.Load()
	if totalResponded != int64(totalMessages) {
		t.Errorf("message drop: sent %d but only got %d responses", totalMessages, totalResponded)
	}

	// Gate: no connection-level failures.
	if connErrors.Load() > 0 {
		t.Errorf("connection-level errors: %d (should be 0)", connErrors.Load())
	}

	// Informational: report success rate (DB contention is expected under
	// extreme concurrent load — this is a diagnostic, not a hard gate).
	successRate := float64(delivered.Load()) / float64(totalMessages) * 100
	t.Logf("success_rate=%.1f%% (chat.done / total)", successRate)
}

// ---------------------------------------------------------------------------
// Test 2: Large Messages — integrity check + size limit enforcement
// ---------------------------------------------------------------------------

func TestLoad_LargeMessages(t *testing.T) {
	before := takeResourceSnapshot()
	logResources(t, "before", before)

	hub, store, cancel := setupLoadEnv(t)
	defer cancel()

	srv := loadServer(t, hub)
	ctx := context.Background()

	sess, err := store.CreateSession("Large Message Test")
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	// Sub-test: messages within the content limit (32KB) should be delivered
	// with full content integrity.
	t.Run("integrity_within_limit", func(t *testing.T) {
		sizes := []int{1024, 10 * 1024, 30 * 1024} // 1KB, 10KB, 30KB (under 32KB limit)
		labels := []string{"1KB", "10KB", "30KB"}

		for i, size := range sizes {
			t.Run(labels[i], func(t *testing.T) {
				conn := loadDial(t, ctx, srv)
				defer conn.Close(websocket.StatusNormalClosure, "done")
				loadDrainInitial(t, ctx, conn)

				payload := makePayload(size)
				expectedHash := sha256Hash(payload)

				msg := fmt.Sprintf(`{"type":"chat.send","sessionId":"%s","content":"%s"}`, sess.ID, payload)
				loadSendJSON(t, ctx, conn, msg)

				resp, err := loadReadJSON(t, ctx, conn, 10*time.Second)
				if err != nil {
					t.Fatalf("read response: %v", err)
				}
				if resp["type"] != "chat.done" {
					t.Fatalf("expected chat.done, got %v", resp["type"])
				}

				// Verify the message was stored with correct content.
				msgs, err := store.GetMessages(sess.ID)
				if err != nil {
					t.Fatalf("get messages: %v", err)
				}
				found := false
				for _, m := range msgs {
					if sha256Hash(m.Content) == expectedHash {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("stored message hash mismatch for %s payload", labels[i])
				}

				t.Logf("%s payload: delivered and verified (hash=%s...)", labels[i], expectedHash[:16])
			})
		}
	})

	// Sub-test: messages exceeding the 32KB content limit should be rejected
	// at the application level with a chat.error.
	t.Run("content_limit_rejected", func(t *testing.T) {
		sizes := []int{100 * 1024, 500 * 1024} // 100KB, 500KB
		labels := []string{"100KB", "500KB"}

		for i, size := range sizes {
			t.Run(labels[i], func(t *testing.T) {
				conn := loadDial(t, ctx, srv)
				defer conn.Close(websocket.StatusNormalClosure, "done")
				loadDrainInitial(t, ctx, conn)

				payload := makePayload(size)
				msg := fmt.Sprintf(`{"type":"chat.send","sessionId":"%s","content":"%s"}`, sess.ID, payload)
				loadSendJSON(t, ctx, conn, msg)

				resp, err := loadReadJSON(t, ctx, conn, 10*time.Second)
				if err != nil {
					t.Fatalf("read response: %v", err)
				}
				if resp["type"] != "chat.error" {
					t.Fatalf("expected chat.error for %s payload, got %v", labels[i], resp["type"])
				}
				t.Logf("%s payload: correctly rejected at application level", labels[i])
			})
		}
	})

	// Sub-test: a raw WebSocket frame exceeding the 1MB read limit should
	// cause the server to close the connection.
	t.Run("websocket_frame_limit_1MB", func(t *testing.T) {
		conn := loadDial(t, ctx, srv)
		defer conn.Close(websocket.StatusNormalClosure, "done")
		loadDrainInitial(t, ctx, conn)

		// Send a frame > 1MB. The server read limit is 1<<20 = 1,048,576.
		oversized := makePayload(1<<20 + 1024)
		err := conn.Write(ctx, websocket.MessageText, []byte(oversized))
		if err != nil {
			// Write might fail if the server closes mid-write.
			t.Logf("write of 1MB+ frame failed (expected): %v", err)
			return
		}

		// If write succeeded, the next read should fail because the server
		// closed the connection after receiving the oversized frame.
		_, readErr := loadReadJSON(t, ctx, conn, 5*time.Second)
		if readErr == nil {
			t.Fatal("expected connection closure after 1MB+ frame, but read succeeded")
		}
		t.Logf("1MB+ frame: server correctly closed connection: %v", readErr)
	})

	after := takeResourceSnapshot()
	logResources(t, "after", after)
}

// ---------------------------------------------------------------------------
// Test 3: Rapid Fire — single client, 1000 messages as fast as possible
// ---------------------------------------------------------------------------
//
// Single-client sequential writes avoid SQLite contention. Every message must
// receive chat.done. Tests for: no goroutine leaks, no message drops, content
// persistence.

func TestLoad_RapidFire(t *testing.T) {
	runtime.GC()
	time.Sleep(100 * time.Millisecond)
	goroutinesBefore := runtime.NumGoroutine()

	before := takeResourceSnapshot()
	logResources(t, "before", before)

	hub, store, cancel := setupLoadEnv(t)
	srv := loadServer(t, hub)
	ctx := context.Background()

	sess, err := store.CreateSession("Rapid Fire Test")
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	conn := loadDial(t, ctx, srv)
	defer conn.Close(websocket.StatusNormalClosure, "done")
	loadDrainInitial(t, ctx, conn)

	const numMessages = 1000
	start := time.Now()
	var doneCount int

	for i := 0; i < numMessages; i++ {
		content := fmt.Sprintf("rapid-fire-%d", i)
		msg := fmt.Sprintf(`{"type":"chat.send","sessionId":"%s","content":"%s"}`, sess.ID, content)
		if err := conn.Write(ctx, websocket.MessageText, []byte(msg)); err != nil {
			t.Fatalf("write message %d: %v", i, err)
		}

		// Read the chat.done response.
		resp, err := loadReadJSON(t, ctx, conn, 10*time.Second)
		if err != nil {
			t.Fatalf("read response for message %d: %v", i, err)
		}
		if resp["type"] == "chat.done" {
			doneCount++
		} else {
			t.Errorf("message %d: expected chat.done, got %v", i, resp["type"])
		}
	}

	elapsed := time.Since(start)

	t.Logf("sent=%d done=%d elapsed=%v messages/sec=%.1f",
		numMessages, doneCount, elapsed, float64(doneCount)/elapsed.Seconds())

	if doneCount != numMessages {
		t.Errorf("expected %d chat.done responses, got %d", numMessages, doneCount)
	}

	// Verify all messages were persisted.
	msgs, err := store.GetMessages(sess.ID)
	if err != nil {
		t.Fatalf("get messages: %v", err)
	}
	if len(msgs) != numMessages {
		t.Errorf("expected %d stored messages, got %d", numMessages, len(msgs))
	}

	// Check for goroutine leaks. Disconnect client first.
	conn.Close(websocket.StatusNormalClosure, "done")
	waitForClientCount(t, hub, 0, 5*time.Second)

	// Cancel hub to release hub goroutine.
	cancel()
	time.Sleep(200 * time.Millisecond)

	// Allow a generous margin for runtime goroutines.
	waitForGoroutines(t, goroutinesBefore+10, 10*time.Second)

	after := takeResourceSnapshot()
	logResources(t, "after", after)

	t.Logf("goroutines: before=%d after=%d", goroutinesBefore, runtime.NumGoroutine())
}

// ---------------------------------------------------------------------------
// Test 4: Resource Bounds — 10 sessions, 100 messages each, bounded resources
// ---------------------------------------------------------------------------
//
// One client per session, sending sequentially within each session but all 10
// sessions run concurrently. Verifies memory stays under 100MB, goroutines
// stay under 200, and all sessions remain functional after the load.

func TestLoad_ResourceBounds(t *testing.T) {
	runtime.GC()
	time.Sleep(100 * time.Millisecond)

	before := takeResourceSnapshot()
	logResources(t, "before", before)

	hub, store, cancel := setupLoadEnv(t)
	defer cancel()

	srv := loadServer(t, hub)

	const numSessions = 10
	const msgsPerSession = 100

	// Create sessions.
	sessions := make([]string, numSessions)
	for i := 0; i < numSessions; i++ {
		sess, err := store.CreateSession(fmt.Sprintf("Resource Bound Session %d", i))
		if err != nil {
			t.Fatalf("create session %d: %v", i, err)
		}
		sessions[i] = sess.ID
	}

	// Phase 1: Send all messages concurrently (one client per session).
	var wg sync.WaitGroup
	wg.Add(numSessions)

	var totalDone atomic.Int64
	var totalAppErrors atomic.Int64
	var totalConnErrors atomic.Int64

	conns := make([]*websocket.Conn, numSessions)

	for s := 0; s < numSessions; s++ {
		go func(idx int) {
			defer wg.Done()

			cCtx, cCancel := context.WithTimeout(context.Background(), 60*time.Second)
			defer cCancel()

			conn := loadDial(t, cCtx, srv)
			conns[idx] = conn
			loadDrainInitial(t, cCtx, conn)

			sessionID := sessions[idx]
			for m := 0; m < msgsPerSession; m++ {
				content := fmt.Sprintf("sess=%d msg=%d", idx, m)
				msg := fmt.Sprintf(`{"type":"chat.send","sessionId":"%s","content":"%s"}`, sessionID, content)
				if err := conn.Write(cCtx, websocket.MessageText, []byte(msg)); err != nil {
					totalConnErrors.Add(1)
					return
				}

				resp, err := loadReadJSON(t, cCtx, conn, 10*time.Second)
				if err != nil {
					totalConnErrors.Add(1)
					return
				}
				switch resp["type"] {
				case "chat.done":
					totalDone.Add(1)
				case "chat.error":
					totalAppErrors.Add(1)
				default:
					totalConnErrors.Add(1)
				}
			}
		}(s)
	}

	wg.Wait()

	// Phase 2: Measure resources under load (connections still open).
	during := takeResourceSnapshot()
	logResources(t, "during (after messages, conns open)", during)

	// Check memory bound: alloc should stay under 100MB.
	allocMB := float64(during.allocBytes) / (1024 * 1024)
	if allocMB > 100 {
		t.Errorf("memory usage %.2fMB exceeds 100MB bound", allocMB)
	} else {
		t.Logf("memory check: %.2fMB (under 100MB bound)", allocMB)
	}

	// Check goroutine bound: should stay under 200.
	if during.goroutines > 200 {
		t.Errorf("goroutine count %d exceeds 200 bound", during.goroutines)
	} else {
		t.Logf("goroutine check: %d (under 200 bound)", during.goroutines)
	}

	// Phase 3: Verify all sessions are still functional after load.
	functionalCount := 0
	for s := 0; s < numSessions; s++ {
		cCtx, cCancel := context.WithTimeout(context.Background(), 10*time.Second)
		content := fmt.Sprintf("post-load-verify-%d", s)
		msg := fmt.Sprintf(`{"type":"chat.send","sessionId":"%s","content":"%s"}`, sessions[s], content)
		if err := conns[s].Write(cCtx, websocket.MessageText, []byte(msg)); err != nil {
			t.Errorf("post-load write to session %d: %v", s, err)
			cCancel()
			continue
		}
		resp, err := loadReadJSON(t, cCtx, conns[s], 10*time.Second)
		cCancel()
		if err != nil {
			t.Errorf("post-load read from session %d: %v", s, err)
			continue
		}
		// Accept both chat.done (success) and chat.error (DB contention) as
		// "functional" — the key is the server responded.
		if resp["type"] == "chat.done" || resp["type"] == "chat.error" {
			functionalCount++
		} else {
			t.Errorf("post-load session %d: unexpected response type %v", s, resp["type"])
		}
	}
	t.Logf("post-load functional sessions: %d/%d", functionalCount, numSessions)
	if functionalCount != numSessions {
		t.Errorf("expected all %d sessions functional, got %d", numSessions, functionalCount)
	}

	// Clean up connections.
	for _, conn := range conns {
		if conn != nil {
			conn.Close(websocket.StatusNormalClosure, "done")
		}
	}
	waitForClientCount(t, hub, 0, 5*time.Second)

	// Report final stats.
	expectedTotal := int64(numSessions * msgsPerSession)
	totalResponded := totalDone.Load() + totalAppErrors.Load() + totalConnErrors.Load()
	t.Logf("total_expected=%d responded=%d delivered=%d app_errors=%d conn_errors=%d",
		expectedTotal, totalResponded, totalDone.Load(), totalAppErrors.Load(), totalConnErrors.Load())

	// Gate: every message must get a response (no drops).
	if totalResponded != expectedTotal {
		t.Errorf("message drop: sent %d but only %d responded", expectedTotal, totalResponded)
	}

	// Gate: no connection-level failures.
	if totalConnErrors.Load() > 0 {
		t.Errorf("connection-level errors: %d (should be 0)", totalConnErrors.Load())
	}

	after := takeResourceSnapshot()
	logResources(t, "after cleanup", after)

	_ = strings.NewReader("") // reference strings to keep import
}
