package integration_test

import (
	"encoding/json"
	"net/http"
	"os"
	"sort"
	"testing"
	"time"

	"nhooyr.io/websocket"
)

func TestSLO_ChatDropRate(t *testing.T) {
	if testing.Short() {
		t.Skip("SLO test")
	}

	token := os.Getenv("SOUL_V2_TOKEN")
	if token == "" {
		t.Skip("SOUL_V2_TOKEN not set")
	}

	// Simulate 50 normal sessions
	for i := 0; i < 50; i++ {
		conn := connectWS(t, "ws://localhost:3002/ws", token)
		readUntilType(t, conn, "connection.ready", 5*time.Second)
		conn.Close(websocket.StatusNormalClosure, "")
	}

	time.Sleep(500 * time.Millisecond)

	// Check pillars via observe server (port 3010)
	req, _ := http.NewRequest("GET", "http://localhost:3010/api/pillars?product=chat", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("pillars request: %v", err)
	}
	defer resp.Body.Close()

	var result struct {
		Pillars []struct {
			Name        string `json:"name"`
			Constraints []struct {
				Name   string `json:"name"`
				Status string `json:"status"`
				Value  string `json:"value"`
			} `json:"constraints"`
		} `json:"pillars"`
	}
	json.NewDecoder(resp.Body).Decode(&result)

	for _, p := range result.Pillars {
		if p.Name == "resilient" {
			for _, c := range p.Constraints {
				if c.Name == "chat-drop-rate" && c.Status == "fail" {
					t.Errorf("SLO violated: chat-drop-rate = %s", c.Value)
				}
			}
		}
	}
}

func TestSLO_ReconnectLatency(t *testing.T) {
	if testing.Short() {
		t.Skip("SLO test")
	}

	token := os.Getenv("SOUL_V2_TOKEN")
	if token == "" {
		t.Skip("SOUL_V2_TOKEN not set")
	}

	var latencies []time.Duration
	for i := 0; i < 10; i++ {
		conn := connectWS(t, "ws://localhost:3002/ws", token)
		readUntilType(t, conn, "connection.ready", 5*time.Second)
		conn.Close(websocket.StatusNormalClosure, "")

		start := time.Now()
		conn2 := connectWS(t, "ws://localhost:3002/ws", token)
		readUntilType(t, conn2, "connection.ready", 5*time.Second)
		latencies = append(latencies, time.Since(start))
		conn2.Close(websocket.StatusNormalClosure, "")
	}

	sort.Slice(latencies, func(i, j int) bool { return latencies[i] < latencies[j] })
	p95 := latencies[int(float64(len(latencies)-1)*0.95)]
	if p95 > 3*time.Second {
		t.Errorf("SLO violated: reconnect P95 = %v (must be < 3s)", p95)
	}
}
