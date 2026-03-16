package server

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/rishav1305/soul-v2/pkg/auth"
	"github.com/rishav1305/soul-v2/internal/chat/metrics"
	"github.com/rishav1305/soul-v2/internal/chat/session"
	"github.com/rishav1305/soul-v2/internal/chat/ws"
)

const version = "0.1.0"

// Server is the soul-v2 HTTP server. It serves the SPA, health endpoint,
// auth status, session CRUD, and WebSocket routes.
type Server struct {
	port         int
	host         string
	mux          *http.ServeMux
	auth         *auth.OAuthTokenSource
	metrics      *metrics.EventLogger
	sessionStore session.StoreInterface
	hub          *ws.Hub
	staticDir    string
	authToken    string   // bearer token for API auth (empty = disabled)
	wsTickets    sync.Map // map[string]time.Time — one-time short-lived WS tickets
	httpServer   *http.Server
	startTime    time.Time
	tlsCert      string // path to TLS certificate
	tlsKey       string // path to TLS private key
	tasksProxy    *tasksProxy
	tutorProxy    *tutorProxy
	projectsProxy *projectsProxy
	observeProxy  *observeProxy
	infraProxy    *simpleProxy
	qualityProxy  *simpleProxy
	dataProxy     *simpleProxy
	docsProxy     *simpleProxy
	scoutProxy    *simpleProxy
	sentinelProxy *simpleProxy
	meshProxy     *simpleProxy
	benchProxy    *simpleProxy
	modelCache    modelCache
}

// Option configures a Server.
type Option func(*Server)

// WithPort sets the listen port.
func WithPort(port int) Option {
	return func(s *Server) { s.port = port }
}

// WithHost sets the bind address.
func WithHost(host string) Option {
	return func(s *Server) { s.host = host }
}

// WithAuthToken sets the bearer token for API authentication.
// If empty, auth middleware is disabled (local dev mode).
func WithAuthToken(token string) Option {
	return func(s *Server) { s.authToken = token }
}

// WithAuth sets the OAuth token source for the auth status endpoint.
func WithAuth(a *auth.OAuthTokenSource) Option {
	return func(s *Server) { s.auth = a }
}

// WithMetrics sets the event logger for system events.
func WithMetrics(l *metrics.EventLogger) Option {
	return func(s *Server) { s.metrics = l }
}

// WithSessionStore sets the session store for session CRUD endpoints.
func WithSessionStore(store session.StoreInterface) Option {
	return func(s *Server) { s.sessionStore = store }
}

// WithStaticDir sets the directory for SPA static files.
func WithStaticDir(dir string) Option {
	return func(s *Server) { s.staticDir = dir }
}

// WithHub sets the WebSocket hub for real-time communication.
func WithHub(hub *ws.Hub) Option {
	return func(s *Server) { s.hub = hub }
}

// WithTLS enables HTTPS with the given certificate and key files.
func WithTLS(certFile, keyFile string) Option {
	return func(s *Server) {
		s.tlsCert = certFile
		s.tlsKey = keyFile
	}
}

// WithTasksProxy enables the reverse proxy to the tasks server.
func WithTasksProxy(hub hubBroadcaster) Option {
	return func(s *Server) {
		s.tasksProxy = newTasksProxy(hub)
	}
}

// WithTutorProxy enables the reverse proxy to the tutor server.
func WithTutorProxy() Option {
	return func(s *Server) {
		s.tutorProxy = newTutorProxy()
	}
}

// WithProjectsProxy enables the reverse proxy to the projects server.
func WithProjectsProxy() Option {
	return func(s *Server) {
		s.projectsProxy = newProjectsProxy()
	}
}

// WithObserveProxy enables the reverse proxy to the observe server.
func WithObserveProxy(target string) Option {
	return func(s *Server) {
		s.observeProxy = newObserveProxy(target)
	}
}

// WithInfraProxy enables the reverse proxy to the infra server.
func WithInfraProxy() Option {
	return func(s *Server) {
		s.infraProxy = newSimpleProxy("SOUL_INFRA_URL", "http://127.0.0.1:3012", "/api/infra", "infra")
	}
}

// WithQualityProxy enables the reverse proxy to the quality server.
func WithQualityProxy() Option {
	return func(s *Server) {
		s.qualityProxy = newSimpleProxy("SOUL_QUALITY_URL", "http://127.0.0.1:3014", "/api/quality", "quality")
	}
}

// WithDataProxy enables the reverse proxy to the data server.
func WithDataProxy() Option {
	return func(s *Server) {
		s.dataProxy = newSimpleProxy("SOUL_DATA_URL", "http://127.0.0.1:3016", "/api/data", "data")
	}
}

// WithDocsProxy enables the reverse proxy to the docs server.
func WithDocsProxy() Option {
	return func(s *Server) {
		s.docsProxy = newSimpleProxy("SOUL_DOCS_URL", "http://127.0.0.1:3018", "/api/docs", "docs")
	}
}

// WithScoutProxy enables the reverse proxy to the scout server.
func WithScoutProxy() Option {
	return func(s *Server) {
		s.scoutProxy = newSimpleProxy("SOUL_SCOUT_URL", "http://127.0.0.1:3020", "/api/scout", "scout")
	}
}

// WithSentinelProxy enables the reverse proxy to the sentinel server.
func WithSentinelProxy() Option {
	return func(s *Server) {
		s.sentinelProxy = newSimpleProxy("SOUL_SENTINEL_URL", "http://127.0.0.1:3022", "/api/sentinel", "sentinel")
	}
}

// WithMeshProxy enables the reverse proxy to the mesh server.
func WithMeshProxy() Option {
	return func(s *Server) {
		s.meshProxy = newSimpleProxy("SOUL_MESH_URL", "http://127.0.0.1:3024", "", "mesh")
	}
}

// WithBenchProxy enables the reverse proxy to the bench server.
func WithBenchProxy() Option {
	return func(s *Server) {
		s.benchProxy = newSimpleProxy("SOUL_BENCH_URL", "http://127.0.0.1:3026", "", "bench")
	}
}

// New creates a configured Server. Defaults: port 3002, host 127.0.0.1.
// Environment variables SOUL_V2_PORT and SOUL_V2_HOST override defaults
// but are overridden by explicit options.
func New(opts ...Option) *Server {
	s := &Server{
		port: 3002,
		host: "127.0.0.1",
		mux:  http.NewServeMux(),
	}

	// Env overrides for defaults.
	if p := os.Getenv("SOUL_V2_PORT"); p != "" {
		if v, err := strconv.Atoi(p); err == nil {
			s.port = v
		}
	}
	if h := os.Getenv("SOUL_V2_HOST"); h != "" {
		s.host = h
	}

	// Functional options override env.
	for _, opt := range opts {
		opt(s)
	}

	s.modelCache = modelCache{ttl: time.Hour}

	// Register routes.
	s.mux.HandleFunc("GET /api/health", s.handleHealth)
	s.mux.HandleFunc("GET /api/auth/status", s.handleAuthStatus)
	s.mux.HandleFunc("/api/reauth", s.handleReauth)
	s.mux.HandleFunc("GET /api/ca.crt", s.handleCACert)

	// Session routes.
	s.mux.HandleFunc("GET /api/sessions", s.handleListSessions)
	s.mux.HandleFunc("POST /api/sessions", s.handleCreateSession)
	s.mux.HandleFunc("DELETE /api/sessions/{id}", s.handleDeleteSession)
	s.mux.HandleFunc("GET /api/sessions/{id}/messages", s.handleGetMessages)

	// Telemetry route.
	s.mux.HandleFunc("POST /api/telemetry", s.handleTelemetry)

	// WS ticket route — exchanges a bearer token for a short-lived one-time ticket
	// so the real auth token is never sent in the WebSocket upgrade URL.
	s.mux.HandleFunc("GET /api/ws-ticket", s.handleWSTicket)

	// Models route.
	s.mux.HandleFunc("GET /api/models", s.handleModels)

	// Tasks server proxy — forward /api/tasks/* and /api/products/* to tasks server.
	if s.tasksProxy != nil {
		s.mux.Handle("/api/tasks/", s.tasksProxy)
		s.mux.Handle("/api/tasks", s.tasksProxy)
		s.mux.Handle("/api/products/", s.tasksProxy)
		s.mux.Handle("/api/products", s.tasksProxy)
	}

	// Tutor server proxy — forward /api/tutor/* to tutor server.
	if s.tutorProxy != nil {
		s.mux.Handle("/api/tutor/", s.tutorProxy)
		s.mux.Handle("/api/tutor", s.tutorProxy)
	}

	// Projects server proxy — forward /api/projects/* to projects server.
	if s.projectsProxy != nil {
		s.mux.Handle("/api/projects/", s.projectsProxy)
		s.mux.Handle("/api/projects", s.projectsProxy)
	}

	// Observe server proxy — forward /api/observe/* to observe server.
	if s.observeProxy != nil {
		s.mux.Handle("/api/observe/", s.observeProxy)
		s.mux.Handle("/api/observe", s.observeProxy)
	}

	// Infra server proxy — forward /api/infra/* to infra server.
	if s.infraProxy != nil {
		s.mux.Handle("/api/infra/", s.infraProxy)
		s.mux.Handle("/api/infra", s.infraProxy)
	}

	// Quality server proxy — forward /api/quality/* to quality server.
	if s.qualityProxy != nil {
		s.mux.Handle("/api/quality/", s.qualityProxy)
		s.mux.Handle("/api/quality", s.qualityProxy)
	}

	// Data server proxy — forward /api/data/* to data server.
	if s.dataProxy != nil {
		s.mux.Handle("/api/data/", s.dataProxy)
		s.mux.Handle("/api/data", s.dataProxy)
	}

	// Docs server proxy — forward /api/docs/* to docs server.
	if s.docsProxy != nil {
		s.mux.Handle("/api/docs/", s.docsProxy)
		s.mux.Handle("/api/docs", s.docsProxy)
	}

	// Scout server proxy — forward /api/scout/* to scout server.
	if s.scoutProxy != nil {
		s.mux.Handle("/api/scout/", s.scoutProxy)
		s.mux.Handle("/api/scout", s.scoutProxy)
	}

	// Sentinel server proxy — forward /api/sentinel/* to sentinel server.
	if s.sentinelProxy != nil {
		s.mux.Handle("/api/sentinel/", s.sentinelProxy)
		s.mux.Handle("/api/sentinel", s.sentinelProxy)
	}

	// Mesh server proxy — forward /api/mesh/* to mesh server.
	if s.meshProxy != nil {
		s.mux.Handle("/api/mesh/", s.meshProxy)
		s.mux.Handle("/api/mesh", s.meshProxy)
	}

	// Bench server proxy — forward /api/bench/* to bench server.
	if s.benchProxy != nil {
		s.mux.Handle("/api/bench/", s.benchProxy)
		s.mux.Handle("/api/bench", s.benchProxy)
	}

	// Unauthenticated health probe — used by load balancers and monitoring.
	s.mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		wsCount := 0
		if s.hub != nil {
			wsCount = s.hub.ClientCount()
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":         "ok",
			"uptime_s":       time.Since(s.startTime).Seconds(),
			"ws_connections": wsCount,
		})
	})

	// WebSocket route — must be registered before SPA fallback.
	if s.hub != nil {
		s.mux.HandleFunc("/ws", s.hub.HandleUpgrade)
	}

	// SPA fallback — all other paths.
	s.mux.Handle("/", s.spaHandler())

	// Build middleware chain: outermost runs first.
	// Recovery → RequestID → CSP → BodyLimit → RateLimit → Auth → RequestLogger → mux
	handler := http.Handler(s.mux)
	if s.metrics != nil {
		handler = requestLoggerMiddleware(s.metrics)(handler)
	}
	if s.authToken != "" {
		handler = authMiddleware(s.authToken, s.metrics, s.consumeWSTicket)(handler)
	}
	handler = rateLimitMiddleware(60)(handler)
	handler = bodyLimitMiddleware(64 << 10)(handler) // 64KB
	handler = securityHeadersMiddleware(handler)
	handler = requestIDMiddleware(handler)
	handler = recoveryMiddleware(handler)

	s.httpServer = &http.Server{
		Addr:              net.JoinHostPort(s.host, strconv.Itoa(s.port)),
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
	}

	return s
}

// Start begins listening. It logs a system.start event, records the start time,
// and blocks until the server shuts down.
func (s *Server) Start() error {
	s.startTime = time.Now()

	if s.metrics != nil {
		_ = s.metrics.Log(metrics.EventSystemStart, map[string]interface{}{
			"port":    s.port,
			"host":    s.host,
			"version": version,
		})
	}

	if s.tlsCert != "" && s.tlsKey != "" {
		// Start a minimal HTTP server that serves the CA cert and redirects
		// everything else to HTTPS. This lets devices download the CA cert
		// before they trust it.
		go s.startHTTPRedirect()

		log.Printf("soul-v2 server listening on https://%s", s.httpServer.Addr)
		err := s.httpServer.ListenAndServeTLS(s.tlsCert, s.tlsKey)
		if err == http.ErrServerClosed {
			return nil
		}
		return err
	}

	log.Printf("soul-v2 server listening on http://%s", s.httpServer.Addr)
	err := s.httpServer.ListenAndServe()
	if err == http.ErrServerClosed {
		return nil
	}
	return err
}

// Shutdown gracefully stops the server with a 10-second timeout.
func (s *Server) Shutdown(ctx context.Context) error {
	if s.metrics != nil {
		uptime := time.Since(s.startTime).String()
		_ = s.metrics.Log(metrics.EventSystemStop, map[string]interface{}{
			"uptime": uptime,
		})
	}

	shutdownCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	return s.httpServer.Shutdown(shutdownCtx)
}

// StartSSERelay starts the SSE relay to the tasks server in the background.
func (s *Server) StartSSERelay(ctx context.Context) {
	if s.tasksProxy != nil {
		s.tasksProxy.StartSSERelay(ctx)
	}
}

// --- Route handlers ---

// handleHealth returns server health status.
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	uptime := time.Since(s.startTime).String()
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":  "ok",
		"version": version,
		"uptime":  uptime,
	})
}

// handleAuthStatus returns the current auth state.
func (s *Server) handleAuthStatus(w http.ResponseWriter, r *http.Request) {
	if s.auth == nil {
		writeJSON(w, http.StatusOK, auth.AuthStatus{State: "missing"})
		return
	}
	writeJSON(w, http.StatusOK, s.auth.Status())
}

// handleReauth reloads OAuth credentials from disk and returns the new auth status.
func (s *Server) handleReauth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "POST")
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{
			"error": "method not allowed",
		})
		return
	}

	if s.auth == nil {
		writeJSON(w, http.StatusOK, map[string]string{
			"error": "authentication not configured",
		})
		return
	}

	s.auth.ReloadFromDisk()

	if s.metrics != nil {
		_ = s.metrics.Log(metrics.EventOAuthReload, map[string]interface{}{
			"source": "api",
		})
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status": "ok",
		"auth":   s.auth.Status(),
	})
}

// startHTTPRedirect runs a plain HTTP server on port+1 that serves the CA cert
// at /ca.crt and redirects everything else to HTTPS.
func (s *Server) startHTTPRedirect() {
	httpPort := s.port + 1
	addr := net.JoinHostPort(s.host, strconv.Itoa(httpPort))

	mux := http.NewServeMux()

	// Serve CA cert over plain HTTP so devices can download it.
	mux.HandleFunc("GET /ca.crt", s.handleCACert)

	// Redirect everything else to HTTPS.
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		target := fmt.Sprintf("https://%s:%d%s", r.Host, s.port, r.URL.RequestURI())
		// Strip the HTTP redirect port from Host if present.
		if host, _, err := net.SplitHostPort(r.Host); err == nil {
			target = fmt.Sprintf("https://%s:%d%s", host, s.port, r.URL.RequestURI())
		}
		http.Redirect(w, r, target, http.StatusMovedPermanently)
	})

	httpServer := &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}

	log.Printf("HTTP redirect server on http://%s (CA cert + redirect to HTTPS)", addr)
	if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Printf("HTTP redirect server error: %v", err)
	}
}

// handleCACert serves the CA certificate for device trust installation.
func (s *Server) handleCACert(w http.ResponseWriter, r *http.Request) {
	if s.tlsCert == "" {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "TLS not configured"})
		return
	}
	caPath := filepath.Join(filepath.Dir(s.tlsCert), "ca.crt")
	data, err := os.ReadFile(caPath)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "CA certificate not found"})
		return
	}
	w.Header().Set("Content-Type", "application/x-x509-ca-cert")
	w.Header().Set("Content-Disposition", "attachment; filename=\"soul-v2-ca.crt\"")
	w.Write(data)
}

// --- Session handlers ---

// handleListSessions returns all sessions ordered by UpdatedAt descending.
func (s *Server) handleListSessions(w http.ResponseWriter, r *http.Request) {
	if s.sessionStore == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{
			"error": "session store not configured",
		})
		return
	}

	sessions, err := s.sessionStore.ListSessions()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "failed to list sessions",
		})
		return
	}

	// Return empty array instead of null when no sessions exist.
	if sessions == nil {
		sessions = []*session.Session{}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"sessions": sessions,
	})
}

// handleCreateSession creates a new session.
func (s *Server) handleCreateSession(w http.ResponseWriter, r *http.Request) {
	if s.sessionStore == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{
			"error": "session store not configured",
		})
		return
	}

	var body struct {
		Title string `json:"title"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "invalid JSON body",
		})
		return
	}

	sess, err := s.sessionStore.CreateSession(body.Title)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "failed to create session",
		})
		return
	}

	writeJSON(w, http.StatusCreated, map[string]interface{}{
		"session": sess,
	})
}

// handleDeleteSession deletes a session and its messages.
func (s *Server) handleDeleteSession(w http.ResponseWriter, r *http.Request) {
	if s.sessionStore == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{
			"error": "session store not configured",
		})
		return
	}

	id := r.PathValue("id")
	if !isValidUUID(id) {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "invalid session ID",
		})
		return
	}

	err := s.sessionStore.DeleteSession(id)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			writeJSON(w, http.StatusNotFound, map[string]string{
				"error": "session not found",
			})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "failed to delete session",
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]bool{
		"deleted": true,
	})
}

// handleGetMessages returns all messages for a session.
func (s *Server) handleGetMessages(w http.ResponseWriter, r *http.Request) {
	if s.sessionStore == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{
			"error": "session store not configured",
		})
		return
	}

	id := r.PathValue("id")
	if !isValidUUID(id) {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "invalid session ID",
		})
		return
	}

	// Verify session exists first.
	_, err := s.sessionStore.GetSession(id)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			writeJSON(w, http.StatusNotFound, map[string]string{
				"error": "session not found",
			})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "failed to get session",
		})
		return
	}

	messages, err := s.sessionStore.GetMessages(id)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "failed to get messages",
		})
		return
	}

	// Return empty array instead of null when no messages exist.
	if messages == nil {
		messages = []*session.Message{}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"messages": messages,
	})
}

// handleTelemetry accepts frontend events and logs them to the metrics logger.
func (s *Server) handleTelemetry(w http.ResponseWriter, r *http.Request) {
	if s.metrics == nil {
		http.Error(w, "metrics not configured", http.StatusServiceUnavailable)
		return
	}

	var payload struct {
		Type string                 `json:"type"`
		Data map[string]interface{} `json:"data"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}

	switch payload.Type {
	case metrics.EventFrontendError, metrics.EventFrontendRender, metrics.EventFrontendWS, metrics.EventFrontendUsage,
		metrics.EventFrontendWSDisconnect, metrics.EventFrontendWSReconnect, metrics.EventFrontendAuthFail:
		// OK — known event types
	default:
		http.Error(w, "unknown event type", http.StatusBadRequest)
		return
	}

	_ = s.metrics.Log(payload.Type, payload.Data)
	w.WriteHeader(http.StatusNoContent)
}

// handleWSTicket issues a short-lived one-time ticket for WebSocket authentication.
// Clients exchange their bearer token (via Authorization header) for a ticket,
// then use ?ticket=<hex> in the WebSocket URL so the real token never appears in
// proxy/server logs.
func (s *Server) handleWSTicket(w http.ResponseWriter, r *http.Request) {
	ticket := s.issueWSTicket()
	writeJSON(w, http.StatusOK, map[string]string{"ticket": ticket})
}

// issueWSTicket generates a cryptographically random one-time ticket valid for 30s.
// It also sweeps expired tickets to prevent unbounded growth.
func (s *Server) issueWSTicket() string {
	now := time.Now()
	s.wsTickets.Range(func(k, v any) bool {
		if now.After(v.(time.Time)) {
			s.wsTickets.Delete(k)
		}
		return true
	})

	var b [16]byte
	_, _ = rand.Read(b[:])
	ticket := fmt.Sprintf("%x", b)
	s.wsTickets.Store(ticket, now.Add(30*time.Second))
	return ticket
}

// consumeWSTicket validates and atomically consumes a one-time WS ticket.
// Returns false if the ticket is unknown or expired.
func (s *Server) consumeWSTicket(ticket string) bool {
	v, ok := s.wsTickets.LoadAndDelete(ticket)
	if !ok {
		return false
	}
	return time.Now().Before(v.(time.Time))
}

// isValidUUID checks if a string is a valid UUID v4 format.
func isValidUUID(s string) bool {
	if len(s) != 36 {
		return false
	}
	for i, c := range s {
		if i == 8 || i == 13 || i == 18 || i == 23 {
			if c != '-' {
				return false
			}
			continue
		}
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return false
		}
	}
	return true
}

// --- SPA handler ---

// spaHandler serves static files from staticDir. If a file doesn't exist,
// it falls back to index.html for client-side routing. API routes that miss
// all registered handlers get a 404 JSON response.
func (s *Server) spaHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// API paths that didn't match a registered handler → 404 JSON.
		if strings.HasPrefix(r.URL.Path, "/api/") {
			writeJSON(w, http.StatusNotFound, map[string]string{
				"error": "not found",
			})
			return
		}

		// No static directory configured.
		if s.staticDir == "" {
			writeJSON(w, http.StatusNotFound, map[string]string{
				"error": "no static directory configured",
			})
			return
		}

		// Check if static directory exists.
		if _, err := os.Stat(s.staticDir); os.IsNotExist(err) {
			writeJSON(w, http.StatusNotFound, map[string]string{
				"error": "static directory not found",
			})
			return
		}

		// Clean the path to prevent directory traversal.
		cleanPath := filepath.Clean(r.URL.Path)
		if cleanPath == "/" {
			cleanPath = "/index.html"
		}

		// Resolve to absolute path and verify it's within staticDir.
		absDir, _ := filepath.Abs(s.staticDir)
		filePath := filepath.Join(s.staticDir, cleanPath)
		absPath, _ := filepath.Abs(filePath)
		if !strings.HasPrefix(absPath, absDir+string(filepath.Separator)) && absPath != absDir {
			// Path escapes static directory — serve SPA fallback.
			s.serveIndexHTML(w, r)
			return
		}

		// Try to serve the requested file.
		info, err := os.Stat(filePath)
		if err != nil || info.IsDir() {
			// File not found — serve index.html (SPA fallback).
			s.serveIndexHTML(w, r)
			return
		}

		// File exists — serve it with appropriate cache headers.
		s.setCacheHeaders(w, cleanPath)
		http.ServeFile(w, r, filePath)
	})
}

// serveIndexHTML serves the SPA index.html with no-cache headers.
func (s *Server) serveIndexHTML(w http.ResponseWriter, r *http.Request) {
	indexPath := filepath.Join(s.staticDir, "index.html")
	if _, err := os.Stat(indexPath); os.IsNotExist(err) {
		writeJSON(w, http.StatusNotFound, map[string]string{
			"error": "index.html not found",
		})
		return
	}
	w.Header().Set("Cache-Control", "no-cache")
	http.ServeFile(w, r, indexPath)
}

// setCacheHeaders sets cache headers based on the file path.
// Hashed assets get long cache; index.html gets no-cache.
func (s *Server) setCacheHeaders(w http.ResponseWriter, path string) {
	base := filepath.Base(path)
	if base == "index.html" {
		w.Header().Set("Cache-Control", "no-cache")
		return
	}
	// Hashed assets typically look like: main.a1b2c3d4.js or style.abc123.css
	// If the filename (minus extension) contains a dot, treat it as hashed.
	nameWithoutExt := strings.TrimSuffix(base, filepath.Ext(base))
	if strings.Contains(nameWithoutExt, ".") {
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		return
	}
	// Default: short cache for non-hashed static files.
	w.Header().Set("Cache-Control", "public, max-age=3600")
}

// --- Middleware ---

// recoveryMiddleware catches panics and returns a 500 JSON error.
func recoveryMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				log.Printf("panic recovered: %v", err)
				writeJSON(w, http.StatusInternalServerError, map[string]string{
					"error": "internal server error",
				})
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// authMiddleware rejects requests to /api/* and /ws without a valid bearer token.
// Static assets (/, /assets/*, /manifest.json, /favicon.ico, /sw.js) are exempt.
// ticketValid, if non-nil, accepts one-time WS tickets issued by /api/ws-ticket.
func authMiddleware(token string, logger *metrics.EventLogger, ticketValid func(string) bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if token == "" {
				next.ServeHTTP(w, r)
				return
			}

			path := r.URL.Path

			// Only require auth for /api/* and /ws — everything else is SPA
			// (static assets, SPA routes like /chat, /tasks, /tutor, etc.)
			if !strings.HasPrefix(path, "/api/") && path != "/ws" {
				next.ServeHTTP(w, r)
				return
			}

			// Check Authorization header (all routes).
			if r.Header.Get("Authorization") == "Bearer "+token {
				next.ServeHTTP(w, r)
				return
			}

			// /ws only: browsers cannot set Authorization headers on WebSocket upgrades.
			// Accept a short-lived one-time ticket (preferred) or the raw token (fallback).
			// Neither is accepted on /api/* to prevent token leakage in logs/referrers.
			if path == "/ws" {
				if t := r.URL.Query().Get("ticket"); t != "" && ticketValid != nil && ticketValid(t) {
					next.ServeHTTP(w, r)
					return
				}
				if r.URL.Query().Get("token") == token {
					next.ServeHTTP(w, r)
					return
				}
			}

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte(`{"error":"unauthorized"}`))
			if logger != nil {
				_ = logger.Log(metrics.EventAuthFail, map[string]interface{}{
					"source":    "api",
					"reason":    "missing_or_invalid_token",
					"client_ip": r.RemoteAddr,
					"path":      r.URL.Path,
				})
			}
		})
	}
}

// securityHeadersMiddleware sets all security headers on every response.
// connect-src allows ws:/wss: broadly because the frontend derives the WS URL
// from window.location.host (supports localhost, LAN IP, etc.). Origin validation
// at WebSocket upgrade provides the actual access control.
func securityHeadersMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Security-Policy",
			"default-src 'self'; script-src 'self'; style-src 'self'; img-src 'self' data:; connect-src 'self' ws: wss:; frame-ancestors 'none'; base-uri 'self'")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")

		// HSTS only when behind TLS (Cloudflare tunnel or direct TLS)
		if r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https" {
			w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		}

		next.ServeHTTP(w, r)
	})
}

// bodyLimitMiddleware limits request body size on POST/PUT/PATCH routes.
// Returns 413 Payload Too Large if the body exceeds maxBytes.
func bodyLimitMiddleware(maxBytes int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == "POST" || r.Method == "PUT" || r.Method == "PATCH" {
				r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
			}
			next.ServeHTTP(w, r)
		})
	}
}

// rateLimitMiddleware implements a simple per-IP sliding window rate limiter.
// It only applies to /api/ routes; static files are not rate-limited.
func rateLimitMiddleware(rpm int) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		var clients sync.Map // map[string]*clientWindow

		// Background cleanup: remove stale entries every minute.
		go func() {
			ticker := time.NewTicker(time.Minute)
			defer ticker.Stop()
			for range ticker.C {
				cutoff := time.Now().Add(-time.Minute)
				clients.Range(func(key, value interface{}) bool {
					cw := value.(*clientWindow)
					cw.mu.Lock()
					if cw.lastSeen.Before(cutoff) {
						clients.Delete(key)
					}
					cw.mu.Unlock()
					return true
				})
			}
		}()

		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Only rate-limit API routes (skip static files and high-frequency endpoints).
			if !strings.HasPrefix(r.URL.Path, "/api/") {
				next.ServeHTTP(w, r)
				return
			}
			// Exempt health and auth-status probes — they are called by monitoring
			// systems and must never be throttled. Telemetry is NOT exempt:
			// each write causes a synchronous fsync, so rate-limiting prevents I/O amplification.
			switch r.URL.Path {
			case "/api/health", "/api/auth/status", "/api/models":
				next.ServeHTTP(w, r)
				return
			}

			ip := clientIP(r)
			now := time.Now()

			val, _ := clients.LoadOrStore(ip, &clientWindow{})
			cw := val.(*clientWindow)

			cw.mu.Lock()
			cw.lastSeen = now
			// Remove timestamps older than 1 minute.
			cutoff := now.Add(-time.Minute)
			valid := 0
			for _, t := range cw.timestamps {
				if t.After(cutoff) {
					cw.timestamps[valid] = t
					valid++
				}
			}
			cw.timestamps = cw.timestamps[:valid]

			if len(cw.timestamps) >= rpm {
				cw.mu.Unlock()
				w.Header().Set("Retry-After", "60")
				writeJSON(w, http.StatusTooManyRequests, map[string]string{
					"error": "rate limit exceeded",
				})
				return
			}

			cw.timestamps = append(cw.timestamps, now)
			cw.mu.Unlock()

			next.ServeHTTP(w, r)
		})
	}
}

// clientWindow tracks request timestamps for a single client IP.
type clientWindow struct {
	mu         sync.Mutex
	timestamps []time.Time
	lastSeen   time.Time
}

// requestIDMiddleware adds a unique X-Request-ID header to each response.
func requestIDMiddleware(next http.Handler) http.Handler {
	var counter uint64
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := generateRequestID(&counter)
		w.Header().Set("X-Request-ID", id)
		next.ServeHTTP(w, r)
	})
}

// statusRecorder wraps http.ResponseWriter to capture the response status code.
type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (sr *statusRecorder) WriteHeader(code int) {
	sr.status = code
	sr.ResponseWriter.WriteHeader(code)
}

// requestLoggerMiddleware times every HTTP request and logs api.request events.
// Requests exceeding 500ms also produce an api.slow event.
// Health-check requests are passed through without logging.
func requestLoggerMiddleware(logger *metrics.EventLogger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Skip health checks (noise) and WebSocket upgrades
			// (statusRecorder doesn't implement http.Hijacker).
			if r.URL.Path == "/api/health" || r.URL.Path == "/ws" {
				next.ServeHTTP(w, r)
				return
			}

			start := time.Now()
			sr := &statusRecorder{ResponseWriter: w, status: 200}
			next.ServeHTTP(sr, r)
			duration := time.Since(start).Milliseconds()

			data := map[string]interface{}{
				"method":      r.Method,
				"path":        r.URL.Path,
				"status":      sr.status,
				"duration_ms": duration,
			}
			_ = logger.Log(metrics.EventAPIRequest, data)

			if duration > 500 {
				_ = logger.Log(metrics.EventAPISlow, data)
			}
		})
	}
}

// --- Helpers ---

// writeJSON writes a JSON response with the given status code.
func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// clientIP extracts the client IP from the request.
// Only trusts X-Forwarded-For from loopback (nginx/cloudflared on same host).
// This prevents LAN/Tailscale nodes from spoofing source IP to bypass rate limits.
func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	// Only trust X-Forwarded-For from loopback proxies.
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		if ip := net.ParseIP(host); ip != nil && ip.IsLoopback() {
			if idx := strings.Index(xff, ","); idx != -1 {
				return strings.TrimSpace(xff[:idx])
			}
			return strings.TrimSpace(xff)
		}
	}
	return host
}

// generateRequestID creates a unique request identifier using a counter + random bytes.
func generateRequestID(counter *uint64) string {
	n := atomic.AddUint64(counter, 1)
	b := make([]byte, 4)
	_, _ = rand.Read(b)
	return fmt.Sprintf("%08x-%x", n, b)
}

// Addr returns the configured listen address. Useful for tests.
func (s *Server) Addr() string {
	return s.httpServer.Addr
}

// Handler returns the server's HTTP handler. Useful for tests with httptest.
func (s *Server) Handler() http.Handler {
	return s.httpServer.Handler
}

// ServeHTTP implements http.Handler, delegating to the configured handler chain.
// This allows passing the Server directly to httptest.NewServer.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.httpServer.Handler.ServeHTTP(w, r)
}

// StaticDirExists reports whether the configured static directory exists on disk.
func StaticDirExists(dir string) bool {
	info, err := os.Stat(dir)
	return err == nil && info.IsDir()
}

// IsHashedAsset returns true if a filename contains a hash segment
// (e.g., main.a1b2c3d4.js). Used for cache header decisions.
func IsHashedAsset(name string) bool {
	ext := filepath.Ext(name)
	base := strings.TrimSuffix(name, ext)
	return strings.Contains(base, ".")
}

// EnsureStaticDir checks that dir exists and contains index.html.
// Returns a descriptive error if not. Used by main.go for early validation.
func EnsureStaticDir(dir string) error {
	info, err := os.Stat(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("static directory %s does not exist (run 'make web' to build)", dir)
		}
		return fmt.Errorf("stat static directory: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("%s is not a directory", dir)
	}
	indexPath := filepath.Join(dir, "index.html")
	if _, err := os.Stat(indexPath); os.IsNotExist(err) {
		return fmt.Errorf("index.html not found in %s", dir)
	}
	return nil
}

// FileExistsInDir checks if a specific file path exists within the given directory
// and is a regular file (not a directory). Prevents directory traversal.
func FileExistsInDir(dir, path string) (bool, fs.FileInfo, error) {
	fullPath := filepath.Join(dir, filepath.Clean(path))
	// Ensure the resolved path is still within the directory.
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return false, nil, err
	}
	absPath, err := filepath.Abs(fullPath)
	if err != nil {
		return false, nil, err
	}
	if !strings.HasPrefix(absPath, absDir+string(filepath.Separator)) && absPath != absDir {
		return false, nil, fmt.Errorf("path traversal attempt: %s", path)
	}

	info, err := os.Stat(fullPath)
	if err != nil {
		return false, nil, nil
	}
	if info.IsDir() {
		return false, nil, nil
	}
	return true, info, nil
}
