package ws

import (
	"context"
	"log"
	"sync"
	"sync/atomic"
	"time"

	"nhooyr.io/websocket"

	"github.com/rishav1305/soul-v2/internal/chat/metrics"
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

	// msgRateBurst is the max messages allowed in the rate window.
	msgRateBurst = 10

	// msgRateWindow is the sliding window for rate limiting.
	msgRateWindow = 5 * time.Second
)

// Client represents a single WebSocket connection managed by the Hub.
type Client struct {
	id        string
	conn      *websocket.Conn
	hub       *Hub
	sessionID atomic.Value // stores string — safe for concurrent read/write
	send      chan []byte
	sendMu    sync.Mutex // protects send channel against concurrent send/close
	sendDone  bool       // true once closeSend has been called
	cancel    context.CancelFunc
	ctx       context.Context
	connTime  time.Time

	// closeReason stores the classified reason for disconnect, set at the point
	// of failure and read by ReadPump's deferred disconnect metric emission.
	// Uses atomic.Value for safe concurrent access (WritePump/Send write it,
	// ReadPump reads it in defer).
	closeReason atomic.Value // stores string

	// Rate limiting: sliding window of inbound message timestamps.
	rateMu   sync.Mutex
	msgTimes []time.Time
}

// ID returns the client's unique identifier.
func (c *Client) ID() string {
	return c.id
}

// Context returns the client's context, which is cancelled when the client
// disconnects or is closed. This allows external code (e.g. an agent) to
// observe when the client goes away and cancel in-flight work.
func (c *Client) Context() context.Context {
	return c.ctx
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

// checkRate returns true if the message should be allowed, false if rate limited.
// Uses a sliding window of msgRateBurst messages over msgRateWindow.
func (c *Client) checkRate() bool {
	c.rateMu.Lock()
	defer c.rateMu.Unlock()

	now := time.Now()
	cutoff := now.Add(-msgRateWindow)

	// Compact: remove timestamps outside the window.
	valid := 0
	for _, t := range c.msgTimes {
		if t.After(cutoff) {
			c.msgTimes[valid] = t
			valid++
		}
	}
	c.msgTimes = c.msgTimes[:valid]

	if len(c.msgTimes) >= msgRateBurst {
		return false
	}

	c.msgTimes = append(c.msgTimes, now)
	return true
}

// Send queues a message for delivery to the client. If the send channel
// is full, the client is considered too slow — the connection is closed
// so the client can reconnect and receive history.
// Safe to call after the channel has been closed — returns false in that case.
func (c *Client) Send(msg []byte) bool {
	c.sendMu.Lock()
	defer c.sendMu.Unlock()

	if c.sendDone {
		return false
	}

	select {
	case c.send <- msg:
		return true
	default:
		// Channel full — close slow client instead of silently dropping messages.
		log.Printf("ws: client %s send channel full, closing slow client", c.id)
		c.closeReason.Store("slow_client_queue_full")
		c.sendDone = true
		close(c.send)
		return false
	}
}

// closeSend closes the send channel, signalling WritePump to stop.
// Safe to call multiple times — only the first call has any effect.
func (c *Client) closeSend() {
	c.sendMu.Lock()
	defer c.sendMu.Unlock()

	if !c.sendDone {
		c.sendDone = true
		close(c.send)
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
	}()

	for {
		typ, data, err := c.conn.Read(c.ctx)
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

		// Reject binary frames.
		if typ == websocket.MessageBinary {
			log.Printf("ws: client %s sent binary frame, rejecting", c.id)
			c.conn.Close(websocket.StatusUnsupportedData, "binary frames not supported")
			return
		}

		// Per-client rate limiting.
		if !c.checkRate() {
			log.Printf("ws: client %s rate limited", c.id)
			errMsg := NewChatError("", "rate limited — slow down")
			if errData, err := MarshalOutbound(errMsg); err == nil {
				c.Send(errData)
			}
			continue
		}

		// Dispatch inbound message to the handler if one is configured.
		if c.hub.handler != nil {
			c.hub.handler.HandleMessage(c, data)
		} else {
			log.Printf("ws: client %s received %d bytes (no handler)", c.id, len(data))
		}
	}
}

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

// Close terminates the client's connection and cancels its context.
func (c *Client) Close() {
	c.cancel()
	c.conn.Close(websocket.StatusGoingAway, "server shutting down")
}

