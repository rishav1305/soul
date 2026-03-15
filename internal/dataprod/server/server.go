package server

import (
	"context"
	"encoding/json"
	"log"
	"net"
	"net/http"
	"strconv"
	"time"
)

// Server is the data HTTP server.
type Server struct {
	mux        *http.ServeMux
	httpServer *http.Server
	host       string
	port       int
	startTime  time.Time
}

// Option configures the Server.
type Option func(*Server)

func WithHost(h string) Option { return func(s *Server) { s.host = h } }
func WithPort(p int) Option   { return func(s *Server) { s.port = p } }

// products and validTools define the data server's capabilities.
var (
	products   = []string{"dataeng", "costops", "viz"}
	validTools = map[string]bool{
		"dataeng__analyze": true,
		"dataeng__report":  true,
		"costops__analyze": true,
		"costops__report":  true,
		"viz__analyze":     true,
		"viz__report":      true,
	}
)

// New creates a new data Server.
func New(opts ...Option) *Server {
	s := &Server{
		mux:       http.NewServeMux(),
		host:      "127.0.0.1",
		port:      3016,
		startTime: time.Now(),
	}
	for _, opt := range opts {
		opt(s)
	}

	// Register routes.
	s.mux.HandleFunc("GET /api/health", s.handleHealth)
	s.mux.HandleFunc("GET /api/products", s.handleProducts)
	s.mux.HandleFunc("GET /api/tools", s.handleTools)

	// Build middleware chain.
	handler := http.Handler(s.mux)
	handler = corsMiddleware(handler)
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
	log.Printf("soul-data listening on http://%s", s.httpServer.Addr)
	return s.httpServer.ListenAndServe()
}

// Shutdown gracefully stops the server.
func (s *Server) Shutdown(ctx context.Context) error {
	return s.httpServer.Shutdown(ctx)
}

// --- Handlers ---

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status": "ok",
		"uptime": time.Since(s.startTime).Seconds(),
	})
}

func (s *Server) handleProducts(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"products": products,
	})
}

func (s *Server) handleTools(w http.ResponseWriter, r *http.Request) {
	tools := make([]string, 0, len(validTools))
	for t := range validTools {
		tools = append(tools, t)
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"tools": tools,
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

// corsMiddleware sets CORS headers for proxied access.
func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
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
