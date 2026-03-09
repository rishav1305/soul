// Package ws provides a WebSocket hub for managing client connections,
// session-scoped message routing, and real-time communication.
package ws

import (
	"context"
	"crypto/rand"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"
	"sync/atomic"
	"time"

	"nhooyr.io/websocket"

	"github.com/rishav1305/soul-v2/internal/metrics"
	"github.com/rishav1305/soul-v2/internal/session"
)

// Hub manages the set of active WebSocket clients and serializes
// register/unregister operations via channels in a single goroutine.
type Hub struct {
	clients        map[*Client]bool
	register       chan *Client
	unregister     chan *Client
	countReq       chan chan int
	findReq        chan chan []*Client
	metrics        *metrics.EventLogger
	sessionStore   *session.Store
	allowedOrigins []string
	clientCounter  uint64
}

// HubOption configures a Hub.
type HubOption func(*Hub)

// WithMetricsLogger sets the event logger for WebSocket events.
func WithMetricsLogger(l *metrics.EventLogger) HubOption {
	return func(h *Hub) { h.metrics = l }
}

// WithSessionStore sets the session store for session operations.
func WithSessionStore(s *session.Store) HubOption {
	return func(h *Hub) { h.sessionStore = s }
}

// WithAllowedOrigins sets the list of allowed origins for WebSocket upgrades.
// Each entry is matched case-insensitively against the request origin host.
// If empty, only localhost origins are allowed by default.
func WithAllowedOrigins(origins []string) HubOption {
	return func(h *Hub) { h.allowedOrigins = origins }
}

// defaultAllowedOrigins returns the default origin patterns that allow
// localhost connections on any port.
func defaultAllowedOrigins() []string {
	return []string{
		"localhost",
		"localhost:*",
		"127.0.0.1",
		"127.0.0.1:*",
	}
}

// NewHub creates a new Hub with the given options.
func NewHub(opts ...HubOption) *Hub {
	h := &Hub{
		clients:    make(map[*Client]bool),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		countReq:   make(chan chan int),
		findReq:    make(chan chan []*Client),
	}

	for _, opt := range opts {
		opt(h)
	}

	if h.allowedOrigins == nil {
		h.allowedOrigins = defaultAllowedOrigins()
	}

	return h
}

// Run starts the hub event loop. It blocks until the provided context
// is cancelled. All register/unregister operations are serialized here.
func (h *Hub) Run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			// Close all remaining clients on shutdown.
			for client := range h.clients {
				close(client.send)
				client.Close()
				delete(h.clients, client)
			}
			return
		case client := <-h.register:
			h.clients[client] = true
		case client := <-h.unregister:
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)
			}
		case reply := <-h.countReq:
			reply <- len(h.clients)
		case reply := <-h.findReq:
			clients := make([]*Client, 0, len(h.clients))
			for c := range h.clients {
				clients = append(clients, c)
			}
			reply <- clients
		}
	}
}

// HandleUpgrade upgrades an HTTP request to a WebSocket connection and
// registers the new client with the hub. It validates the request origin
// before accepting the connection.
func (h *Hub) HandleUpgrade(w http.ResponseWriter, r *http.Request) {
	// Validate origin before upgrade.
	if !h.isOriginAllowed(r) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		InsecureSkipVerify: true, // We do our own origin validation above.
	})
	if err != nil {
		log.Printf("ws: upgrade failed: %v", err)
		return
	}

	// Set read limit to 1MB.
	conn.SetReadLimit(1 << 20)

	clientID := h.generateClientID()
	// Use Background context — the request context is cancelled when the
	// HTTP handler returns, but the WebSocket connection (hijacked) lives
	// beyond the handler's lifetime.
	ctx, cancel := context.WithCancel(context.Background())

	client := &Client{
		id:       clientID,
		conn:     conn,
		hub:      h,
		send:     make(chan []byte, sendChannelCap),
		cancel:   cancel,
		ctx:      ctx,
		connTime: time.Now(),
	}

	h.register <- client

	origin := r.Header.Get("Origin")
	if h.metrics != nil {
		_ = h.metrics.Log(metrics.EventWSConnect, map[string]interface{}{
			"client_id": clientID,
			"origin":    origin,
		})
	}

	// Start read and write pumps in separate goroutines.
	go client.WritePump()
	go client.ReadPump()
}

// ClientCount returns the number of currently connected clients.
// It sends a request through the hub event loop to ensure thread-safety.
func (h *Hub) ClientCount() int {
	reply := make(chan int, 1)
	h.countReq <- reply
	return <-reply
}

// Clients returns a snapshot of all currently connected clients.
// Safe to call from any goroutine — the request is serialized through the hub event loop.
func (h *Hub) Clients() []*Client {
	reply := make(chan []*Client, 1)
	h.findReq <- reply
	return <-reply
}

// isOriginAllowed checks whether the request origin is in the allowed list.
// If no Origin header is present (non-browser clients), the request is allowed.
func (h *Hub) isOriginAllowed(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		// No origin header — non-browser client, allow by default.
		return true
	}

	u, err := url.Parse(origin)
	if err != nil {
		return false
	}

	host := u.Host
	if host == "" {
		return false
	}

	// Check if origin matches the request host (same-origin).
	if strings.EqualFold(r.Host, host) {
		return true
	}

	// Check against allowed origins.
	for _, pattern := range h.allowedOrigins {
		if matchOrigin(pattern, host) {
			return true
		}
	}

	return false
}

// matchOrigin checks if a host matches a pattern. Supports wildcard port
// matching with "host:*" pattern.
func matchOrigin(pattern, host string) bool {
	// Exact match (case insensitive).
	if strings.EqualFold(pattern, host) {
		return true
	}

	// Wildcard port: "localhost:*" matches "localhost:3000", "localhost:8080", etc.
	if strings.HasSuffix(pattern, ":*") {
		prefix := strings.TrimSuffix(pattern, ":*")
		// Host must be "prefix:port" format.
		if idx := strings.LastIndex(host, ":"); idx != -1 {
			hostName := host[:idx]
			if strings.EqualFold(prefix, hostName) {
				return true
			}
		}
	}

	return false
}

// generateClientID creates a unique client identifier using a counter + random bytes.
func (h *Hub) generateClientID() string {
	n := atomic.AddUint64(&h.clientCounter, 1)
	b := make([]byte, 4)
	_, _ = rand.Read(b)
	return fmt.Sprintf("ws-%08x-%x", n, b)
}
