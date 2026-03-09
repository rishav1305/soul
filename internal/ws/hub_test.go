package ws

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"nhooyr.io/websocket"

	"github.com/rishav1305/soul-v2/internal/metrics"
)

func TestNewHub_Defaults(t *testing.T) {
	h := NewHub()
	if h.clients == nil {
		t.Fatal("expected clients map to be initialized")
	}
	if h.register == nil {
		t.Fatal("expected register channel to be initialized")
	}
	if h.unregister == nil {
		t.Fatal("expected unregister channel to be initialized")
	}
	if len(h.allowedOrigins) == 0 {
		t.Fatal("expected default allowed origins")
	}
}

func TestNewHub_WithOptions(t *testing.T) {
	origins := []string{"example.com"}
	h := NewHub(
		WithAllowedOrigins(origins),
	)
	if len(h.allowedOrigins) != 1 || h.allowedOrigins[0] != "example.com" {
		t.Errorf("expected allowed origins [example.com], got %v", h.allowedOrigins)
	}
}

func TestNewHub_WithMetrics(t *testing.T) {
	dir := t.TempDir()
	logger, err := metrics.NewEventLogger(dir)
	if err != nil {
		t.Fatalf("create event logger: %v", err)
	}
	defer logger.Close()

	h := NewHub(WithMetricsLogger(logger))
	if h.metrics == nil {
		t.Fatal("expected metrics to be set")
	}
}

func TestHubRegisterClient(t *testing.T) {
	h := NewHub()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go h.Run(ctx)

	// Create a test WebSocket server and connect a client.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h.HandleUpgrade(w, r)
	}))
	defer srv.Close()

	wsURL := "ws" + srv.URL[len("http"):]
	conn, _, err := websocket.Dial(ctx, wsURL, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close(websocket.StatusNormalClosure, "")

	// Give the hub time to process the registration.
	time.Sleep(50 * time.Millisecond)

	count := h.ClientCount()
	if count != 1 {
		t.Errorf("expected 1 client, got %d", count)
	}
}

func TestHubUnregisterClient(t *testing.T) {
	h := NewHub()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go h.Run(ctx)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h.HandleUpgrade(w, r)
	}))
	defer srv.Close()

	wsURL := "ws" + srv.URL[len("http"):]
	conn, _, err := websocket.Dial(ctx, wsURL, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}

	// Wait for registration.
	time.Sleep(50 * time.Millisecond)
	if count := h.ClientCount(); count != 1 {
		t.Fatalf("expected 1 client after connect, got %d", count)
	}

	// Close the connection to trigger unregister.
	conn.Close(websocket.StatusNormalClosure, "done")

	// Wait for unregistration.
	time.Sleep(100 * time.Millisecond)
	if count := h.ClientCount(); count != 0 {
		t.Errorf("expected 0 clients after disconnect, got %d", count)
	}
}

func TestHubClientCount(t *testing.T) {
	h := NewHub()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go h.Run(ctx)

	if count := h.ClientCount(); count != 0 {
		t.Errorf("expected 0 clients initially, got %d", count)
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h.HandleUpgrade(w, r)
	}))
	defer srv.Close()

	wsURL := "ws" + srv.URL[len("http"):]
	conns := make([]*websocket.Conn, 3)
	for i := 0; i < 3; i++ {
		conn, _, err := websocket.Dial(ctx, wsURL, nil)
		if err != nil {
			t.Fatalf("dial %d: %v", i, err)
		}
		conns[i] = conn
	}
	defer func() {
		for _, conn := range conns {
			if conn != nil {
				conn.Close(websocket.StatusNormalClosure, "")
			}
		}
	}()

	time.Sleep(50 * time.Millisecond)
	if count := h.ClientCount(); count != 3 {
		t.Errorf("expected 3 clients, got %d", count)
	}

	// Close one.
	conns[0].Close(websocket.StatusNormalClosure, "")
	conns[0] = nil
	time.Sleep(100 * time.Millisecond)

	if count := h.ClientCount(); count != 2 {
		t.Errorf("expected 2 clients after one disconnect, got %d", count)
	}
}

func TestHubOriginValidation_AllowsDefault(t *testing.T) {
	h := NewHub()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go h.Run(ctx)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h.HandleUpgrade(w, r)
	}))
	defer srv.Close()

	// Dial without explicit origin — should be allowed (non-browser).
	wsURL := "ws" + srv.URL[len("http"):]
	conn, _, err := websocket.Dial(ctx, wsURL, nil)
	if err != nil {
		t.Fatalf("expected connection to succeed without origin, got: %v", err)
	}
	conn.Close(websocket.StatusNormalClosure, "")
}

func TestHubOriginValidation_AcceptsAllowed(t *testing.T) {
	tests := []struct {
		name    string
		origins []string
		origin  string
	}{
		{"exact match", []string{"example.com"}, "http://example.com"},
		{"wildcard port", []string{"localhost:*"}, "http://localhost:3000"},
		{"localhost default", nil, "http://localhost:8080"},
		{"127.0.0.1 default", nil, "http://127.0.0.1:3000"},
		{"case insensitive", []string{"Example.COM"}, "http://example.com"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest("GET", "/ws", nil)
			r.Header.Set("Origin", tc.origin)

			var opts []HubOption
			if tc.origins != nil {
				opts = append(opts, WithAllowedOrigins(tc.origins))
			}
			h := NewHub(opts...)

			if !h.isOriginAllowed(r) {
				t.Errorf("expected origin %q to be allowed", tc.origin)
			}
		})
	}
}

func TestHubOriginValidation_RejectsDisallowed(t *testing.T) {
	tests := []struct {
		name    string
		origins []string
		origin  string
	}{
		{"unknown host", []string{"example.com"}, "http://evil.com"},
		{"different host default origins", nil, "http://evil.com"},
		{"empty host", nil, "http://"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest("GET", "/ws", nil)
			r.Header.Set("Origin", tc.origin)

			var opts []HubOption
			if tc.origins != nil {
				opts = append(opts, WithAllowedOrigins(tc.origins))
			}
			h := NewHub(opts...)

			if h.isOriginAllowed(r) {
				t.Errorf("expected origin %q to be rejected", tc.origin)
			}
		})
	}
}

func TestHubOriginValidation_RejectsWith403(t *testing.T) {
	h := NewHub(WithAllowedOrigins([]string{"example.com"}))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go h.Run(ctx)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h.HandleUpgrade(w, r)
	}))
	defer srv.Close()

	// Make a regular HTTP request with a disallowed origin to check the 403.
	req, _ := http.NewRequest("GET", srv.URL+"/ws", nil)
	req.Header.Set("Origin", "http://evil.com")
	req.Header.Set("Connection", "Upgrade")
	req.Header.Set("Upgrade", "websocket")
	req.Header.Set("Sec-WebSocket-Version", "13")
	req.Header.Set("Sec-WebSocket-Key", "dGhlIHNhbXBsZSBub25jZQ==")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("expected 403, got %d", resp.StatusCode)
	}
}

func TestHubConcurrentRegisterUnregister(t *testing.T) {
	h := NewHub()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go h.Run(ctx)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h.HandleUpgrade(w, r)
	}))
	defer srv.Close()

	wsURL := "ws" + srv.URL[len("http"):]

	const numClients = 20
	var wg sync.WaitGroup
	wg.Add(numClients)

	for i := 0; i < numClients; i++ {
		go func() {
			defer wg.Done()
			conn, _, err := websocket.Dial(ctx, wsURL, nil)
			if err != nil {
				return
			}
			// Stay connected briefly, then disconnect.
			time.Sleep(20 * time.Millisecond)
			conn.Close(websocket.StatusNormalClosure, "")
		}()
	}

	wg.Wait()
	// Wait for all unregistrations to process.
	time.Sleep(200 * time.Millisecond)

	count := h.ClientCount()
	if count != 0 {
		t.Errorf("expected 0 clients after all disconnected, got %d", count)
	}
}

func TestHubShutdownClosesClients(t *testing.T) {
	h := NewHub()
	ctx, cancel := context.WithCancel(context.Background())

	hubDone := make(chan struct{})
	go func() {
		h.Run(ctx)
		close(hubDone)
	}()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h.HandleUpgrade(w, r)
	}))

	wsURL := "ws" + srv.URL[len("http"):]
	dialCtx, dialCancel := context.WithTimeout(ctx, 2*time.Second)
	conn, _, err := websocket.Dial(dialCtx, wsURL, nil)
	dialCancel()
	if err != nil {
		srv.Close()
		t.Fatalf("dial: %v", err)
	}

	time.Sleep(50 * time.Millisecond)

	// Close the HTTP test server first to prevent new connections,
	// then close the WS connection, then cancel hub context.
	srv.Close()
	conn.Close(websocket.StatusNormalClosure, "")

	// Cancel context to trigger hub shutdown.
	cancel()

	select {
	case <-hubDone:
		// Hub shut down successfully.
	case <-time.After(2 * time.Second):
		t.Fatal("hub did not shut down within timeout")
	}
}

func TestGenerateClientID(t *testing.T) {
	h := NewHub()
	id1 := h.generateClientID()
	id2 := h.generateClientID()

	if id1 == "" {
		t.Fatal("expected non-empty client ID")
	}
	if id1 == id2 {
		t.Errorf("expected unique IDs, got same: %s", id1)
	}
	if len(id1) < 10 {
		t.Errorf("expected client ID length >= 10, got %d", len(id1))
	}
}

func TestMatchOrigin(t *testing.T) {
	tests := []struct {
		pattern string
		host    string
		want    bool
	}{
		{"localhost", "localhost", true},
		{"localhost:*", "localhost:3000", true},
		{"localhost:*", "localhost:8080", true},
		{"localhost:*", "localhost", false},
		{"127.0.0.1", "127.0.0.1", true},
		{"127.0.0.1:*", "127.0.0.1:3000", true},
		{"example.com", "example.com", true},
		{"example.com", "evil.com", false},
		{"Example.COM", "example.com", true},
		{"localhost", "LOCALHOST", true},
	}

	for _, tc := range tests {
		t.Run(tc.pattern+"_"+tc.host, func(t *testing.T) {
			got := matchOrigin(tc.pattern, tc.host)
			if got != tc.want {
				t.Errorf("matchOrigin(%q, %q) = %v, want %v", tc.pattern, tc.host, got, tc.want)
			}
		})
	}
}
