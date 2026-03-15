package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"nhooyr.io/websocket"

	"github.com/rishav1305/soul-v2/internal/mesh/node"
	"github.com/rishav1305/soul-v2/internal/mesh/transport"
)

const (
	heartbeatInterval = 10 * time.Second
	minBackoff        = 1 * time.Second
	maxBackoff        = 300 * time.Second
	backoffFactor     = 2
)

// Agent connects to a hub via WebSocket and sends periodic heartbeats.
type Agent struct {
	nodeInfo node.NodeInfo
	hubURL   string
	secret   string
}

// New creates an Agent that will connect to the given hub URL.
func New(info node.NodeInfo, hubURL, secret string) *Agent {
	return &Agent{
		nodeInfo: info,
		hubURL:   hubURL,
		secret:   secret,
	}
}

// HeartbeatLoop connects to the hub and sends heartbeats every 10 seconds.
// On connection failure it retries with exponential backoff (1s to 300s).
// It blocks until ctx is cancelled.
func (a *Agent) HeartbeatLoop(ctx context.Context) error {
	backoff := minBackoff

	for {
		err := a.connectAndSend(ctx)
		if ctx.Err() != nil {
			return ctx.Err()
		}
		log.Printf("mesh agent: connection lost: %v, retrying in %s", err, backoff)

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(backoff):
		}

		backoff *= backoffFactor
		if backoff > maxBackoff {
			backoff = maxBackoff
		}
	}
}

func (a *Agent) connectAndSend(ctx context.Context) error {
	token, err := transport.CreateToken(a.nodeInfo.ID, a.secret)
	if err != nil {
		return fmt.Errorf("agent: create token: %w", err)
	}

	wsURL := a.hubURL + "?token=" + token
	conn, _, err := websocket.Dial(ctx, wsURL, nil)
	if err != nil {
		return fmt.Errorf("agent: dial hub: %w", err)
	}
	defer conn.Close(websocket.StatusNormalClosure, "shutdown")

	// Send register message.
	regPayload, _ := json.Marshal(a.nodeInfo)
	regMsg := transport.Message{
		Type:    "register",
		NodeID:  a.nodeInfo.ID,
		Payload: regPayload,
	}
	regData, _ := json.Marshal(regMsg)
	if err := conn.Write(ctx, websocket.MessageText, regData); err != nil {
		return fmt.Errorf("agent: send register: %w", err)
	}

	ticker := time.NewTicker(heartbeatInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if err := a.sendHeartbeat(ctx, conn); err != nil {
				return err
			}
		}
	}
}

func (a *Agent) sendHeartbeat(ctx context.Context, conn *websocket.Conn) error {
	snap, _ := node.SystemSnapshot()

	payload, _ := json.Marshal(map[string]interface{}{
		"cpuCores":      snap.CPUCores,
		"ramTotalMB":    snap.RAMTotalMB,
		"storageTotalGB": snap.StorageTotalGB,
	})

	msg := transport.Message{
		Type:    "heartbeat",
		NodeID:  a.nodeInfo.ID,
		Payload: payload,
	}
	data, _ := json.Marshal(msg)
	return conn.Write(ctx, websocket.MessageText, data)
}
