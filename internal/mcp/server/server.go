// Package server implements the MCP Streamable HTTP server for Soul v2.
// It exposes product tools via JSON-RPC 2.0 over HTTP with OAuth 2.1 auth.
package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"runtime/debug"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/rishav1305/soul-v2/internal/mcp/auth"
	"github.com/rishav1305/soul-v2/internal/mcp/protocol"
	"github.com/rishav1305/soul-v2/internal/mcp/tools"
)

// Server is the soul-mcp HTTP server. It serves the MCP Streamable HTTP
// endpoint, OAuth 2.1 discovery/auth, and health checks.
type Server struct {
	mux        *http.ServeMux
	httpServer *http.Server
	registry   *tools.Registry
	oauth      *auth.OAuthHandler
	host       string
	port       int
	secret     string
	startTime  time.Time
}

// Option configures a Server.
type Option func(*Server)

// WithHost sets the bind address.
func WithHost(host string) Option {
	return func(s *Server) { s.host = host }
}

// WithPort sets the listen port.
func WithPort(port int) Option {
	return func(s *Server) { s.port = port }
}

// WithSecret sets the JWT signing secret for auth middleware.
func WithSecret(secret string) Option {
	return func(s *Server) { s.secret = secret }
}

// WithRegistry sets the MCP tool registry.
func WithRegistry(r *tools.Registry) Option {
	return func(s *Server) { s.registry = r }
}

// WithOAuth sets the OAuth 2.1 handler for discovery and token endpoints.
func WithOAuth(h *auth.OAuthHandler) Option {
	return func(s *Server) { s.oauth = h }
}

// WithBaseURL sets the external base URL on the OAuth handler.
func WithBaseURL(url string) Option {
	return func(s *Server) {
		if s.oauth != nil {
			s.oauth.SetBaseURL(url)
		}
	}
}

// New creates a configured Server. Defaults: port 3028, host 127.0.0.1.
func New(opts ...Option) *Server {
	s := &Server{
		port: 3028,
		host: "127.0.0.1",
		mux:  http.NewServeMux(),
	}

	for _, opt := range opts {
		opt(s)
	}

	// --- Register routes ---

	// OAuth discovery (no auth required).
	if s.oauth != nil {
		s.mux.HandleFunc("GET /.well-known/oauth-protected-resource", s.oauth.HandleProtectedResource)
		s.mux.HandleFunc("GET /.well-known/oauth-authorization-server", s.oauth.HandleAuthorizationServer)
		s.mux.HandleFunc("GET /authorize", s.oauth.HandleAuthorize)
		s.mux.HandleFunc("POST /authorize", s.oauth.HandleAuthorize)
		s.mux.HandleFunc("POST /token", s.oauth.HandleToken)
		s.mux.HandleFunc("POST /register", s.oauth.HandleRegister)
	}

	// Health (no auth required).
	s.mux.HandleFunc("GET /health", s.handleHealth)

	// MCP endpoint (auth required — handled by middleware).
	s.mux.HandleFunc("POST /", s.handleMCP)
	s.mux.HandleFunc("GET /", s.handleMCPGet)

	// Build middleware chain (outermost runs first).
	// Recovery -> BodyLimit -> RateLimit -> Origin -> Auth -> mux
	handler := http.Handler(s.mux)

	skipPaths := []string{
		"/.well-known/",
		"/authorize",
		"/token",
		"/register",
		"/health",
	}
	handler = auth.AuthMiddleware(s.secret, skipPaths)(handler)
	handler = auth.OriginMiddleware([]string{
		"https://claude.ai",
		"https://console.anthropic.com",
	})(handler)
	handler = rateLimitMiddleware(60)(handler)
	handler = bodyLimitMiddleware(1 << 20)(handler) // 1MB
	handler = recoveryMiddleware(handler)

	s.httpServer = &http.Server{
		Addr:              net.JoinHostPort(s.host, strconv.Itoa(s.port)),
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
	}

	return s
}

// Start begins listening and blocks until the server shuts down.
func (s *Server) Start() error {
	s.startTime = time.Now()
	toolCount := 0
	if s.registry != nil {
		toolCount = len(s.registry.List())
	}
	log.Printf("soul-mcp server listening on http://%s (%d tools registered)", s.httpServer.Addr, toolCount)
	return s.httpServer.ListenAndServe()
}

// Shutdown gracefully stops the server with a 10-second timeout.
func (s *Server) Shutdown(ctx context.Context) error {
	shutdownCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	return s.httpServer.Shutdown(shutdownCtx)
}

// --- MCP handlers ---

// handleMCP processes JSON-RPC 2.0 requests on POST /.
func (s *Server) handleMCP(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, protocol.NewError(nil, protocol.ParseError, "failed to read request body"))
		return
	}

	req, err := protocol.ParseRequest(body)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, protocol.NewError(nil, protocol.ParseError, fmt.Sprintf("invalid request: %v", err)))
		return
	}

	// Notifications (no ID) get 202 Accepted with no body.
	if req.IsNotification() {
		w.WriteHeader(http.StatusAccepted)
		return
	}

	var resp *protocol.Response

	switch req.Method {
	case "initialize":
		resp = protocol.NewResult(req.ID, protocol.InitializeResult{
			ProtocolVersion: "2025-03-26",
			ServerInfo: protocol.ServerInfo{
				Name:    "soul-v2",
				Version: "1.0.0",
			},
			Capabilities: protocol.Capabilities{
				Tools: &protocol.ToolsCapability{},
			},
		})

	case "ping":
		resp = protocol.NewResult(req.ID, map[string]interface{}{})

	case "tools/list":
		var toolsList []protocol.MCPTool
		if s.registry != nil {
			toolsList = s.registry.List()
		}
		if toolsList == nil {
			toolsList = []protocol.MCPTool{}
		}
		resp = protocol.NewResult(req.ID, protocol.ToolsListResult{
			Tools: toolsList,
		})

	case "tools/call":
		resp = s.handleToolCall(r.Context(), req)

	default:
		resp = protocol.NewError(req.ID, protocol.MethodNotFound, fmt.Sprintf("method not found: %s", req.Method))
	}

	writeJSON(w, http.StatusOK, resp)
}

// handleToolCall parses tool call params, executes the tool, and returns the result.
func (s *Server) handleToolCall(ctx context.Context, req *protocol.Request) *protocol.Response {
	if s.registry == nil {
		return protocol.NewError(req.ID, protocol.InternalError, "tool registry not configured")
	}

	name, args, err := protocol.ParseToolCallParams(req.Params)
	if err != nil {
		return protocol.NewError(req.ID, protocol.InvalidParams, fmt.Sprintf("invalid tool call params: %v", err))
	}

	if !s.registry.Has(name) {
		return protocol.NewError(req.ID, protocol.InvalidParams, fmt.Sprintf("unknown tool: %s", name))
	}

	// Execute with a 60-second timeout.
	callCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	start := time.Now()
	result, err := s.registry.Call(callCtx, name, args)
	duration := time.Since(start)

	log.Printf("tool call: %s (%s)", name, duration.Round(time.Millisecond))

	if err != nil {
		return protocol.NewResult(req.ID, protocol.ToolCallResult{
			Content: []protocol.ToolContent{
				{Type: "text", Text: fmt.Sprintf("Error: %v", err)},
			},
			IsError: true,
		})
	}

	return protocol.NewResult(req.ID, protocol.ToolCallResult{
		Content: []protocol.ToolContent{
			{Type: "text", Text: result},
		},
	})
}

// handleMCPGet returns 405 Method Not Allowed for GET / requests.
func (s *Server) handleMCPGet(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Allow", "POST")
	writeJSON(w, http.StatusMethodNotAllowed, map[string]string{
		"error": "method not allowed, use POST for MCP requests",
	})
}

// handleHealth returns server health status.
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	toolCount := 0
	if s.registry != nil {
		toolCount = len(s.registry.List())
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status": "ok",
		"uptime": time.Since(s.startTime).String(),
		"tools":  toolCount,
	})
}

// --- Middleware ---

// recoveryMiddleware catches panics, logs the stack trace, and returns a 500 JSON error.
func recoveryMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				log.Printf("panic recovered: %v\n%s", err, debug.Stack())
				writeJSON(w, http.StatusInternalServerError, map[string]string{
					"error": "internal server error",
				})
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// bodyLimitMiddleware limits request body size on POST/PUT/PATCH routes.
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

// rateLimitMiddleware implements a per-IP sliding window rate limiter.
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
			// Exempt health probes from rate limiting.
			if r.URL.Path == "/health" {
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

// --- Helpers ---

// writeJSON writes a JSON response with the given status code.
func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// clientIP extracts the client IP from the request.
// Only trusts X-Forwarded-For from loopback (reverse proxy on same host).
func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
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
