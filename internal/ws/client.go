package ws

import (
	"context"
	"log"
	"sync/atomic"
	"time"

	"nhooyr.io/websocket"

	"github.com/rishav1305/soul-v2/internal/metrics"
)

const (
	// sendChannelCap is the capacity of the outbound message buffer.
	sendChannelCap = 256

	// pingInterval is how often we send pings to the client.
	pingInterval = 30 * time.Second

	// pongTimeout is how long we wait for a pong response.
	// Two missed pongs (2 * 30s = 60s) means the client is dead.
	pongTimeout = 60 * time.Second

	// maxMessageSize is the maximum inbound message size (1MB).
	maxMessageSize = 1 << 20
)

// Client represents a single WebSocket connection managed by the Hub.
type Client struct {
	id        string
	conn      *websocket.Conn
	hub       *Hub
	sessionID atomic.Value // stores string — safe for concurrent read/write
	send      chan []byte
	cancel    context.CancelFunc
	ctx       context.Context
	connTime  time.Time
}

// ID returns the client's unique identifier.
func (c *Client) ID() string {
	return c.id
}

// SessionID returns the currently subscribed session ID.
func (c *Client) SessionID() string {
	v := c.sessionID.Load()
	if v == nil {
		return ""
	}
	return v.(string)
}

// Subscribe switches the client's session subscription to the given session ID.
func (c *Client) Subscribe(sessionID string) {
	c.sessionID.Store(sessionID)
}

// Send queues a message for delivery to the client. If the send channel
// is full, the oldest message is dropped and a warning is logged.
func (c *Client) Send(msg []byte) {
	select {
	case c.send <- msg:
	default:
		// Channel full — drop oldest message.
		select {
		case <-c.send:
			log.Printf("ws: client %s send channel full, dropped oldest message", c.id)
		default:
		}
		// Try to send again.
		select {
		case c.send <- msg:
		default:
			log.Printf("ws: client %s send channel still full after drop, message lost", c.id)
		}
	}
}

// ReadPump reads messages from the WebSocket connection. It handles
// ping/pong, enforces message size limits, and rejects binary frames.
// ReadPump blocks until the connection is closed or an error occurs.
func (c *Client) ReadPump() {
	defer func() {
		// Always attempt to unregister with the hub. Use a timeout rather
		// than the client context — the context may already be cancelled
		// (e.g. from client.Close()) but the hub still needs to remove us.
		unregCtx, unregCancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer unregCancel()
		select {
		case c.hub.unregister <- c:
		case <-unregCtx.Done():
		}
		c.conn.Close(websocket.StatusNormalClosure, "")
		c.cancel()

		if c.hub.metrics != nil {
			duration := time.Since(c.connTime).Seconds()
			_ = c.hub.metrics.Log(metrics.EventWSDisconnect, map[string]interface{}{
				"client_id":        c.id,
				"reason":           "read_pump_exit",
				"duration_seconds": duration,
			})
		}
	}()

	for {
		typ, data, err := c.conn.Read(c.ctx)
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

		// Reject binary frames.
		if typ == websocket.MessageBinary {
			log.Printf("ws: client %s sent binary frame, rejecting", c.id)
			c.conn.Close(websocket.StatusUnsupportedData, "binary frames not supported")
			return
		}

		// Dispatch inbound message to the handler if one is configured.
		if c.hub.handler != nil {
			c.hub.handler.HandleMessage(c, data)
		} else {
			log.Printf("ws: client %s received %d bytes (no handler)", c.id, len(data))
		}
	}
}

// WritePump writes messages from the send channel to the WebSocket connection.
// It also handles periodic ping/pong to detect dead connections.
// WritePump blocks until the send channel is closed or an error occurs.
func (c *Client) WritePump() {
	ticker := time.NewTicker(pingInterval)
	defer func() {
		ticker.Stop()
		c.conn.Close(websocket.StatusNormalClosure, "")
	}()

	for {
		select {
		case msg, ok := <-c.send:
			if !ok {
				// Send channel was closed by the hub.
				c.conn.Close(websocket.StatusNormalClosure, "")
				return
			}

			err := c.conn.Write(c.ctx, websocket.MessageText, msg)
			if err != nil {
				log.Printf("ws: client %s write error: %v", c.id, err)
				return
			}

		case <-ticker.C:
			// Send a ping to check if the client is still alive.
			ctx, cancel := context.WithTimeout(c.ctx, pongTimeout)
			err := c.conn.Ping(ctx)
			cancel()
			if err != nil {
				log.Printf("ws: client %s ping failed: %v", c.id, err)
				return
			}

		case <-c.ctx.Done():
			return
		}
	}
}

// Close terminates the client's connection and cancels its context.
func (c *Client) Close() {
	c.cancel()
	c.conn.Close(websocket.StatusGoingAway, "server shutting down")
}
