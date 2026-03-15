package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/rishav1305/soul-v2/internal/quality/compliance"
)

// Server is the quality HTTP server.
type Server struct {
	mux        *http.ServeMux
	httpServer *http.Server
	host       string
	port       int
	startTime  time.Time
	compliance *compliance.Service
}

// Option configures the Server.
type Option func(*Server)

func WithHost(h string) Option { return func(s *Server) { s.host = h } }
func WithPort(p int) Option   { return func(s *Server) { s.port = p } }

// products lists the products served by this server.
var products = []string{"compliance", "qa", "analytics"}

// validTools maps tool names to their product.
var validTools = map[string]string{
	"compliance__scan":   "compliance",
	"compliance__fix":    "compliance",
	"compliance__badge":  "compliance",
	"compliance__report": "compliance",
	"qa__analyze":        "qa",
	"qa__report":         "qa",
	"analytics__analyze": "analytics",
	"analytics__report":  "analytics",
}

// New creates a new quality Server.
func New(opts ...Option) *Server {
	s := &Server{
		mux:        http.NewServeMux(),
		host:       "127.0.0.1",
		port:       3014,
		startTime:  time.Now(),
		compliance: &compliance.Service{},
	}
	for _, opt := range opts {
		opt(s)
	}

	// Register routes.
	s.mux.HandleFunc("GET /api/health", s.handleHealth)
	s.mux.HandleFunc("POST /api/tools/{name}/execute", s.handleToolExecute)

	// Build middleware chain.
	handler := http.Handler(s.mux)
	handler = cspMiddleware(handler)
	handler = bodyLimitMiddleware(64 * 1024)(handler)
	handler = recoveryMiddleware(handler)

	s.httpServer = &http.Server{
		Addr:              net.JoinHostPort(s.host, strconv.Itoa(s.port)),
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
	}

	return s
}

// ServeHTTP implements http.Handler for testing.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

// Start begins listening.
func (s *Server) Start() error {
	log.Printf("soul-quality listening on http://%s", s.httpServer.Addr)
	return s.httpServer.ListenAndServe()
}

// Shutdown gracefully stops the server.
func (s *Server) Shutdown(ctx context.Context) error {
	return s.httpServer.Shutdown(ctx)
}

// --- Handlers ---

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	uptime := time.Since(s.startTime).Round(time.Second).String()
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":   "ok",
		"uptime":   uptime,
		"products": products,
	})
}

func (s *Server) handleToolExecute(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	product, ok := validTools[name]
	if !ok {
		names := make([]string, 0, len(validTools))
		for k := range validTools {
			names = append(names, k)
		}
		writeError(w, http.StatusBadRequest, fmt.Sprintf("unknown tool %q, valid tools: %s", name, strings.Join(names, ", ")))
		return
	}

	// Read request body as tool input.
	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeError(w, http.StatusBadRequest, "failed to read request body")
		return
	}

	var input json.RawMessage
	if len(body) > 0 {
		input = json.RawMessage(body)
	} else {
		input = json.RawMessage(`{}`)
	}

	// Extract the tool name after the product prefix.
	tool := name
	if idx := strings.Index(name, "__"); idx >= 0 {
		tool = name[idx+2:]
	}

	// Route compliance tools to the service; qa/analytics are stubs.
	if product == "compliance" {
		result, execErr := s.compliance.ExecuteTool(tool, input)
		if execErr != nil {
			writeError(w, http.StatusInternalServerError, execErr.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"success": true,
			"data":    result,
		})
		return
	}

	// Stub response for qa and analytics products.
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"data": map[string]string{
			"status":  "not_yet_implemented",
			"product": product,
			"tool":    tool,
		},
	})
}

// --- Middleware ---

// recoveryMiddleware catches panics and returns a 500 JSON error.
func recoveryMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				log.Printf("panic recovered: %v", err)
				writeError(w, http.StatusInternalServerError, "internal server error")
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// bodyLimitMiddleware limits request body size.
func bodyLimitMiddleware(maxBytes int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Body != nil {
				r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
			}
			next.ServeHTTP(w, r)
		})
	}
}

// cspMiddleware sets Content-Security-Policy headers.
func cspMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Security-Policy", "default-src 'self'")
		next.ServeHTTP(w, r)
	})
}

// --- Helpers ---

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
