package server

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"
)

// tasksProxy manages the reverse proxy and SSE relay to the tasks server.
type tasksProxy struct {
	targetURL    *url.URL
	reverseProxy *httputil.ReverseProxy
	hub          hubBroadcaster
	mu           sync.Mutex
	connected    bool
}

// hubBroadcaster is the interface the proxy needs to send events to WS clients.
type hubBroadcaster interface {
	BroadcastJSON(msgType string, data interface{})
}

func newTasksProxy(hub hubBroadcaster) *tasksProxy {
	tasksURL := os.Getenv("SOUL_TASKS_URL")
	if tasksURL == "" {
		tasksURL = "http://127.0.0.1:3004"
	}

	target, err := url.Parse(tasksURL)
	if err != nil {
		log.Printf("warn: invalid SOUL_TASKS_URL %q: %v", tasksURL, err)
		return nil
	}

	rp := httputil.NewSingleHostReverseProxy(target)

	// Custom transport with timeout.
	rp.Transport = &http.Transport{
		ResponseHeaderTimeout: 5 * time.Second,
	}

	// Custom error handler — return 503 if tasks server is down.
	rp.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		log.Printf("tasks proxy error: %v", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		fmt.Fprintf(w, `{"error":"tasks server unavailable"}`)
	}

	return &tasksProxy{
		targetURL:    target,
		reverseProxy: rp,
		hub:          hub,
	}
}

// ServeHTTP forwards requests to the tasks server.
func (tp *tasksProxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	tp.reverseProxy.ServeHTTP(w, r)
}

// StartSSERelay connects to the tasks server SSE stream and relays events to WS clients.
func (tp *tasksProxy) StartSSERelay(ctx context.Context) {
	if tp == nil || tp.hub == nil {
		return
	}

	backoff := []time.Duration{
		1 * time.Second, 2 * time.Second, 4 * time.Second,
		8 * time.Second, 15 * time.Second, 30 * time.Second,
	}
	attempt := 0

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		err := tp.connectSSE(ctx)
		if err != nil && ctx.Err() == nil {
			delay := backoff[attempt]
			if attempt < len(backoff)-1 {
				attempt++
			}
			log.Printf("tasks SSE relay disconnected: %v (retry in %s)", err, delay)

			tp.mu.Lock()
			tp.connected = false
			tp.mu.Unlock()

			select {
			case <-time.After(delay):
			case <-ctx.Done():
				return
			}
		} else {
			attempt = 0
		}
	}
}

func (tp *tasksProxy) connectSSE(ctx context.Context) error {
	streamURL := tp.targetURL.String() + "/api/stream"
	req, err := http.NewRequestWithContext(ctx, "GET", streamURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "text/event-stream")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("SSE stream returned %d", resp.StatusCode)
	}

	tp.mu.Lock()
	tp.connected = true
	tp.mu.Unlock()

	log.Printf("tasks SSE relay connected to %s", streamURL)

	scanner := bufio.NewScanner(resp.Body)
	var eventType, eventData string

	for scanner.Scan() {
		line := scanner.Text()

		if strings.HasPrefix(line, "event: ") {
			eventType = strings.TrimPrefix(line, "event: ")
		} else if strings.HasPrefix(line, "data: ") {
			eventData = strings.TrimPrefix(line, "data: ")
		} else if line == "" && eventType != "" {
			// Empty line = end of event.
			if tp.hub != nil && eventType != "connected" {
				// Parse the data string — it could be JSON or plain text.
				var parsed interface{}
				if err := json.Unmarshal([]byte(eventData), &parsed); err != nil {
					parsed = eventData
				}
				tp.hub.BroadcastJSON(eventType, parsed)
			}
			eventType = ""
			eventData = ""
		}
	}

	return scanner.Err()
}

// IsConnected returns whether the SSE relay is connected.
func (tp *tasksProxy) IsConnected() bool {
	if tp == nil {
		return false
	}
	tp.mu.Lock()
	defer tp.mu.Unlock()
	return tp.connected
}

// tutorProxy manages the reverse proxy to the tutor server.
// Unlike tasksProxy it does not relay SSE events.
type tutorProxy struct {
	reverseProxy *httputil.ReverseProxy
}

func newTutorProxy() *tutorProxy {
	tutorURL := os.Getenv("SOUL_TUTOR_URL")
	if tutorURL == "" {
		tutorURL = "http://127.0.0.1:3006"
	}

	target, err := url.Parse(tutorURL)
	if err != nil {
		log.Printf("warn: invalid SOUL_TUTOR_URL %q: %v", tutorURL, err)
		return nil
	}

	rp := httputil.NewSingleHostReverseProxy(target)

	// Custom transport with timeout.
	rp.Transport = &http.Transport{
		ResponseHeaderTimeout: 5 * time.Second,
	}

	// Custom error handler — return 503 if tutor server is down.
	rp.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		log.Printf("tutor proxy error: %v", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		fmt.Fprintf(w, `{"error":"tutor server unavailable"}`)
	}

	return &tutorProxy{
		reverseProxy: rp,
	}
}

// ServeHTTP forwards requests to the tutor server.
func (tp *tutorProxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	tp.reverseProxy.ServeHTTP(w, r)
}
