package team

import (
	"context"
	"encoding/json"
	"log"
	"sync"
	"time"

	"nhooyr.io/websocket"
)

// Hub manages WebSocket connections for the team dashboard.
// It supports multiplexed message types and per-agent pane subscriptions.
type Hub struct {
	mu      sync.RWMutex
	clients map[*Client]bool
}

// Client represents a single WebSocket connection.
type Client struct {
	conn   *websocket.Conn
	ctx    context.Context
	cancel context.CancelFunc
	send   chan []byte
	subs   map[string]bool // subscribed message type prefixes (e.g., "pane.shuri")
	mu     sync.RWMutex
}

// NewHub creates a new WebSocket hub.
func NewHub() *Hub {
	return &Hub{
		clients: make(map[*Client]bool),
	}
}

// Register adds a client to the hub.
func (h *Hub) Register(client *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.clients[client] = true
}

// Unregister removes a client from the hub.
func (h *Hub) Unregister(client *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.clients, client)
	client.cancel()
	close(client.send)
}

// Broadcast sends a message to all connected clients that are subscribed to the type.
func (h *Hub) Broadcast(msg WSMessage) {
	data, err := json.Marshal(msg)
	if err != nil {
		log.Printf("team hub: marshal error: %v", err)
		return
	}

	h.mu.RLock()
	defer h.mu.RUnlock()

	for client := range h.clients {
		if client.isSubscribed(msg.Type) {
			select {
			case client.send <- data:
			default:
				// Client is slow, skip this message.
				log.Printf("team hub: dropping message for slow client")
			}
		}
	}
}

// BroadcastAll sends a message to ALL connected clients regardless of subscription.
func (h *Hub) BroadcastAll(msg WSMessage) {
	data, err := json.Marshal(msg)
	if err != nil {
		log.Printf("team hub: marshal error: %v", err)
		return
	}

	h.mu.RLock()
	defer h.mu.RUnlock()

	for client := range h.clients {
		select {
		case client.send <- data:
		default:
			log.Printf("team hub: dropping message for slow client")
		}
	}
}

// ClientCount returns the number of connected clients.
func (h *Hub) ClientCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}

// NewClient creates a new client from a websocket connection.
func NewClient(conn *websocket.Conn, ctx context.Context) *Client {
	ctx, cancel := context.WithCancel(ctx)
	return &Client{
		conn:   conn,
		ctx:    ctx,
		cancel: cancel,
		send:   make(chan []byte, 256),
		subs:   make(map[string]bool),
	}
}

// Subscribe adds a message type subscription.
func (c *Client) Subscribe(msgType string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.subs[msgType] = true
}

// SubscribeAll subscribes to all message types (wildcard).
func (c *Client) SubscribeAll() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.subs["*"] = true
}

func (c *Client) isSubscribed(msgType string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	// Wildcard subscription.
	if c.subs["*"] {
		return true
	}

	// Exact match.
	if c.subs[msgType] {
		return true
	}

	// Prefix match (e.g., "pane." matches "pane.shuri").
	for sub := range c.subs {
		if len(sub) > 0 && sub[len(sub)-1] == '.' {
			if len(msgType) >= len(sub) && msgType[:len(sub)] == sub {
				return true
			}
		}
	}

	return false
}

// WritePump sends messages from the send channel to the websocket.
func (c *Client) WritePump() {
	defer c.conn.Close(websocket.StatusNormalClosure, "closing")

	for {
		select {
		case <-c.ctx.Done():
			return
		case msg, ok := <-c.send:
			if !ok {
				return
			}
			ctx, cancel := context.WithTimeout(c.ctx, 5*time.Second)
			err := c.conn.Write(ctx, websocket.MessageText, msg)
			cancel()
			if err != nil {
				return
			}
		}
	}
}

// ReadPump reads messages from the websocket (for subscription commands).
func (c *Client) ReadPump(h *Hub) {
	defer h.Unregister(c)

	for {
		_, data, err := c.conn.Read(c.ctx)
		if err != nil {
			return
		}

		// Parse subscription commands.
		var cmd struct {
			Action string `json:"action"` // subscribe, unsubscribe
			Type   string `json:"type"`   // message type to sub/unsub
		}
		if err := json.Unmarshal(data, &cmd); err != nil {
			continue
		}

		switch cmd.Action {
		case "subscribe":
			c.Subscribe(cmd.Type)
		case "unsubscribe":
			c.mu.Lock()
			delete(c.subs, cmd.Type)
			c.mu.Unlock()
		}
	}
}
