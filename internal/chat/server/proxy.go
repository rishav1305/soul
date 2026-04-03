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

// projectsProxy manages the reverse proxy to the projects server.
type projectsProxy struct {
	reverseProxy *httputil.ReverseProxy
}

func newProjectsProxy() *projectsProxy {
	projectsURL := os.Getenv("SOUL_PROJECTS_URL")
	if projectsURL == "" {
		projectsURL = "http://127.0.0.1:3008"
	}

	target, err := url.Parse(projectsURL)
	if err != nil {
		log.Printf("warn: invalid SOUL_PROJECTS_URL %q: %v", projectsURL, err)
		return nil
	}

	rp := httputil.NewSingleHostReverseProxy(target)

	rp.Transport = &http.Transport{
		ResponseHeaderTimeout: 5 * time.Second,
	}

	rp.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		log.Printf("projects proxy error: %v", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		fmt.Fprintf(w, `{"error":"projects server unavailable"}`)
	}

	return &projectsProxy{
		reverseProxy: rp,
	}
}

// ServeHTTP forwards requests to the projects server.
func (pp *projectsProxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	pp.reverseProxy.ServeHTTP(w, r)
}

// observeProxy manages the reverse proxy to the observe server.
type observeProxy struct {
	reverseProxy *httputil.ReverseProxy
}

func newObserveProxy(target string) *observeProxy {
	parsed, err := url.Parse(target)
	if err != nil {
		log.Printf("warn: invalid observe URL %q: %v", target, err)
		return nil
	}

	rp := httputil.NewSingleHostReverseProxy(parsed)

	rp.Transport = &http.Transport{
		ResponseHeaderTimeout: 5 * time.Second,
	}

	rp.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		log.Printf("observe proxy error: %v", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		fmt.Fprintf(w, `{"error":"observe server unavailable"}`)
	}

	return &observeProxy{
		reverseProxy: rp,
	}
}

// ServeHTTP forwards requests to the observe server, rewriting /api/observe → /api.
func (op *observeProxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	r.URL.Path = strings.Replace(r.URL.Path, "/api/observe", "/api", 1)
	if r.URL.Path == "" {
		r.URL.Path = "/"
	}
	op.reverseProxy.ServeHTTP(w, r)
}

// simpleProxy manages a reverse proxy to a product server with path rewriting.
type simpleProxy struct {
	reverseProxy *httputil.ReverseProxy
	pathPrefix   string // e.g., "/api/infra"
}

func newSimpleProxy(envKey, defaultURL, pathPrefix, productName string) *simpleProxy {
	rawURL := os.Getenv(envKey)
	if rawURL == "" {
		rawURL = defaultURL
	}

	target, err := url.Parse(rawURL)
	if err != nil {
		log.Printf("warn: invalid %s %q: %v", envKey, rawURL, err)
		return nil
	}

	rp := httputil.NewSingleHostReverseProxy(target)

	rp.Transport = &http.Transport{
		ResponseHeaderTimeout: 5 * time.Second,
	}

	rp.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		log.Printf("%s proxy error: %v", productName, err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		fmt.Fprintf(w, `{"error":"%s server unavailable"}`, productName)
	}

	return &simpleProxy{
		reverseProxy: rp,
		pathPrefix:   pathPrefix,
	}
}

// ServeHTTP forwards requests, rewriting the path prefix to /api.
// If pathPrefix is empty, the path passes through unchanged.
func (sp *simpleProxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if sp.pathPrefix != "" {
		r.URL.Path = strings.Replace(r.URL.Path, sp.pathPrefix, "/api", 1)
		if r.URL.Path == "" {
			r.URL.Path = "/"
		}
	}
	sp.reverseProxy.ServeHTTP(w, r)
}

// teamProxy manages the reverse proxy to the Soul Control team server.
// It handles both standard HTTP and WebSocket upgrades transparently.
type teamProxy struct {
	targetURL    *url.URL
	reverseProxy *httputil.ReverseProxy
}

func newTeamProxy() *teamProxy {
	teamURL := os.Getenv("SOUL_TEAM_URL")
	if teamURL == "" {
		teamURL = "http://127.0.0.1:3028"
	}

	target, err := url.Parse(teamURL)
	if err != nil {
		log.Printf("warn: invalid SOUL_TEAM_URL %q: %v", teamURL, err)
		return nil
	}

	rp := httputil.NewSingleHostReverseProxy(target)

	// Use a transport that supports WebSocket upgrades via Hop-by-hop passthrough.
	rp.Transport = &http.Transport{
		ResponseHeaderTimeout: 30 * time.Second,
	}

	// Custom error handler — return 503 if team server is down.
	rp.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		log.Printf("team proxy error: %v", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		fmt.Fprintf(w, `{"error":"team server unavailable"}`)
	}

	return &teamProxy{
		targetURL:    target,
		reverseProxy: rp,
	}
}

// ServeHTTP forwards requests to the team server.
// For WebSocket upgrade requests (/ws/team), it uses a raw TCP hijack to
// transparently proxy the upgraded connection.
func (tp *teamProxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Check if this is a WebSocket upgrade request.
	if isWebSocketUpgrade(r) {
		tp.proxyWebSocket(w, r)
		return
	}
	// Standard HTTP — pass through with no path rewriting.
	tp.reverseProxy.ServeHTTP(w, r)
}

func isWebSocketUpgrade(r *http.Request) bool {
	for _, v := range r.Header.Values("Connection") {
		if strings.EqualFold(strings.TrimSpace(v), "upgrade") {
			return strings.EqualFold(r.Header.Get("Upgrade"), "websocket")
		}
	}
	return false
}

// proxyWebSocket hijacks the connection and proxies WebSocket frames to the backend.
func (tp *teamProxy) proxyWebSocket(w http.ResponseWriter, r *http.Request) {
	// Build the backend URL.
	backendURL := *tp.targetURL
	backendURL.Path = r.URL.Path
	backendURL.RawQuery = r.URL.RawQuery
	if backendURL.Scheme == "https" {
		backendURL.Scheme = "wss"
	} else {
		backendURL.Scheme = "ws"
	}

	// Dial the backend directly using net, then do the HTTP upgrade manually.
	// We use the standard reverse proxy for WebSocket by ensuring the hop-by-hop
	// headers are forwarded. Go's httputil.ReverseProxy handles WebSocket upgrades
	// when the backend returns 101 Switching Protocols.
	//
	// Set FlushInterval to -1 so it flushes immediately (needed for streaming/WS).
	tp.reverseProxy.FlushInterval = -1

	// Forward the Origin header so the backend's origin check passes.
	// The reverse proxy strips hop-by-hop headers, so we need to ensure the
	// Upgrade and Connection headers pass through.
	director := tp.reverseProxy.Director
	tp.reverseProxy.Director = func(req *http.Request) {
		director(req)
		// Restore hop-by-hop headers that ReverseProxy strips.
		req.Header.Set("Connection", "Upgrade")
		req.Header.Set("Upgrade", "websocket")
		if origin := r.Header.Get("Origin"); origin != "" {
			req.Header.Set("Origin", origin)
		}
	}

	tp.reverseProxy.ServeHTTP(w, r)

	// Restore original director.
	tp.reverseProxy.Director = director
}
