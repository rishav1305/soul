package server

import (
	"bufio"
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"nhooyr.io/websocket"

	"github.com/rishav1305/soul/internal/chat/metrics"
)

// ── hijackableRecorder ────────────────────────────────────────────────────────

// hijackableRecorder is a ResponseRecorder that also implements http.Hijacker.
// It records whether Hijack() was called, which lets us verify statusRecorder
// delegates the call rather than swallowing it.
type hijackableRecorder struct {
	*httptest.ResponseRecorder
	hijackCalled bool
	conn         net.Conn
	rw           *bufio.ReadWriter
}

func (h *hijackableRecorder) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	h.hijackCalled = true
	// Return stub values so callers don't panic.
	conn1, _ := net.Pipe()
	br := bufio.NewReadWriter(bufio.NewReader(conn1), bufio.NewWriter(conn1))
	h.conn = conn1
	h.rw = br
	return conn1, br, nil
}

// ── Unit tests for statusRecorder ─────────────────────────────────────────────

// TestStatusRecorder_ImplementsHijacker verifies statusRecorder satisfies the
// http.Hijacker interface at compile time and at runtime.
func TestStatusRecorder_ImplementsHijacker(t *testing.T) {
	base := &hijackableRecorder{ResponseRecorder: httptest.NewRecorder()}
	sr := &statusRecorder{ResponseWriter: base, status: 200}

	// Compile-time assertion: statusRecorder must implement http.Hijacker.
	var _ http.Hijacker = sr

	// Runtime assertion: type switch must succeed.
	if _, ok := any(sr).(http.Hijacker); !ok {
		t.Fatal("statusRecorder does not implement http.Hijacker at runtime")
	}
}

// TestStatusRecorder_HijackDelegates verifies Hijack() is forwarded to the
// underlying ResponseWriter, not silently dropped.
func TestStatusRecorder_HijackDelegates(t *testing.T) {
	base := &hijackableRecorder{ResponseRecorder: httptest.NewRecorder()}
	sr := &statusRecorder{ResponseWriter: base, status: 200}

	conn, brw, err := sr.Hijack()
	if err != nil {
		t.Fatalf("Hijack() returned error: %v", err)
	}
	if conn == nil {
		t.Fatal("Hijack() returned nil conn")
	}
	if brw == nil {
		t.Fatal("Hijack() returned nil bufio.ReadWriter")
	}
	if !base.hijackCalled {
		t.Fatal("Hijack() did not delegate to the underlying ResponseWriter")
	}
}

// TestStatusRecorder_HijackErrorsWhenBaseNotHijackable verifies the error path
// when the underlying ResponseWriter does not implement http.Hijacker.
func TestStatusRecorder_HijackErrorsWhenBaseNotHijackable(t *testing.T) {
	// httptest.NewRecorder() does NOT implement http.Hijacker.
	base := httptest.NewRecorder()
	sr := &statusRecorder{ResponseWriter: base, status: 200}

	_, _, err := sr.Hijack()
	if err == nil {
		t.Fatal("expected error when base ResponseWriter is not a Hijacker, got nil")
	}
}

// TestStatusRecorder_WriteHeaderCapturesCode verifies WriteHeader still works.
func TestStatusRecorder_WriteHeaderCapturesCode(t *testing.T) {
	base := httptest.NewRecorder()
	sr := &statusRecorder{ResponseWriter: base, status: 200}

	sr.WriteHeader(http.StatusTeapot)

	if sr.status != http.StatusTeapot {
		t.Errorf("expected status %d, got %d", http.StatusTeapot, sr.status)
	}
	if base.Code != http.StatusTeapot {
		t.Errorf("expected underlying recorder code %d, got %d", http.StatusTeapot, base.Code)
	}
}

// ── Integration test: WS upgrade through requestLoggerMiddleware ───────────────

// TestRequestLoggerMiddleware_WebSocketUpgradeSucceeds is a regression test for
// the statusRecorder http.Hijacker bug. Before the fix, nhooyr.io/websocket's
// Accept() would fail with "upstream ResponseWriter does not implement
// http.Hijacker" because the logging middleware wrapped w in statusRecorder
// which did not forward Hijack(). After the fix, the upgrade must succeed.
func TestRequestLoggerMiddleware_WebSocketUpgradeSucceeds(t *testing.T) {
	dir := t.TempDir()
	logger, err := metrics.NewEventLogger(dir, "")
	if err != nil {
		t.Fatalf("NewEventLogger: %v", err)
	}

	// WS handler: accept the upgrade and immediately close.
	wsHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
			InsecureSkipVerify: true, // test server has no TLS cert
		})
		if err != nil {
			// If the Hijacker fix is absent, this error is:
			// "upstream ResponseWriter does not implement http.Hijacker"
			t.Errorf("websocket.Accept failed (Hijacker regression): %v", err)
			return
		}
		conn.Close(websocket.StatusNormalClosure, "test done")
	})

	// Apply the request logger middleware (this wraps w in statusRecorder).
	wrapped := requestLoggerMiddleware(logger)(wsHandler)

	srv := httptest.NewServer(wrapped)
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	wsURL := "ws" + srv.URL[len("http"):]
	conn, _, err := websocket.Dial(ctx, wsURL, nil)
	if err != nil {
		t.Fatalf("websocket.Dial failed — WS upgrade through requestLoggerMiddleware broken: %v", err)
	}
	conn.Close(websocket.StatusNormalClosure, "")
}
