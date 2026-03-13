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
	broadcastCh    chan []byte
	sessionBcastCh chan sessionBroadcast
	metrics        *metrics.EventLogger
	sessionStore   session.StoreInterface
	handler        *MessageHandler
	allowedOrigins []string
	clientCounter  uint64
}

// sessionBroadcast wraps a session-scoped broadcast request.
type sessionBroadcast struct {
	sessionID string
	msg       []byte
}

// HubOption configures a Hub.
type HubOption func(*Hub)

// WithMetricsLogger sets the event logger for WebSocket events.
func WithMetricsLogger(l *metrics.EventLogger) HubOption {
	return func(h *Hub) { h.metrics = l }
}

// WithSessionStore sets the session store for session operations.
func WithSessionStore(s session.StoreInterface) HubOption {
	return func(h *Hub) { h.sessionStore = s }
}

// WithMessageHandler sets the message handler for dispatching inbound messages.
func WithMessageHandler(mh *MessageHandler) HubOption {
	return func(h *Hub) { h.handler = mh }
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
		clients:        make(map[*Client]bool),
		register:       make(chan *Client),
		unregister:     make(chan *Client),
		countReq:       make(chan chan int),
		findReq:        make(chan chan []*Client),
		broadcastCh:    make(chan []byte, 1024),
		sessionBcastCh: make(chan sessionBroadcast, 1024),
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
				client.closeSend()
				client.Close()
				delete(h.clients, client)
			}
			return
		case client := <-h.register:
			h.clients[client] = true
		case client := <-h.unregister:
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				client.closeSend()
			}
		case reply := <-h.countReq:
			reply <- len(h.clients)
		case reply := <-h.findReq:
			clients := make([]*Client, 0, len(h.clients))
			for c := range h.clients {
				clients = append(clients, c)
			}
			reply <- clients
		case msg := <-h.broadcastCh:
			for client := range h.clients {
				client.Send(msg)
			}
		case sb := <-h.sessionBcastCh:
			for client := range h.clients {
				if client.SessionID() == sb.sessionID {
					client.Send(sb.msg)
				}
			}
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

	// Send connection.ready message with the client ID.
	readyMsg := NewConnectionReady(clientID)
	if data, err := MarshalOutbound(readyMsg); err == nil {
		client.Send(data)
	}

	// Send session.list (excluding empty sessions) if a session store is configured.
	if h.sessionStore != nil {
		allSessions, err := h.sessionStore.ListSessions()
		if err != nil {
			log.Printf("ws: failed to list sessions for new client %s: %v", clientID, err)
		} else {
			nonEmpty := make([]*session.Session, 0, len(allSessions))
			for _, s := range allSessions {
				if s.MessageCount > 0 {
					nonEmpty = append(nonEmpty, s)
				}
			}
			listMsg := NewSessionList(nonEmpty)
			if data, err := MarshalOutbound(listMsg); err == nil {
				client.Send(data)
			}
		}
	}
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

// Broadcast sends a message to all connected clients. The operation is
// serialized through the hub event loop to avoid data races on the clients map.
// Non-blocking: if the broadcast channel is full, the message is dropped.
func (h *Hub) Broadcast(msg []byte) {
	select {
	case h.broadcastCh <- msg:
	default:
		log.Printf("ws: broadcast channel full, message dropped")
	}
}

// BroadcastToSession sends a message only to clients subscribed to the given
// session. The operation is serialized through the hub event loop.
// Non-blocking: if the channel is full, the message is dropped.
func (h *Hub) BroadcastToSession(sessionID string, msg []byte) {
	select {
	case h.sessionBcastCh <- sessionBroadcast{sessionID: sessionID, msg: msg}:
	default:
		log.Printf("ws: session broadcast channel full, message dropped for session %s", sessionID)
	}
}

// SetHandler sets the message handler for dispatching inbound messages.
// Must be called before Run() starts — not safe for concurrent use.
func (h *Hub) SetHandler(mh *MessageHandler) {
	h.handler = mh
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
