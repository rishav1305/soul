package server

import (
	"fmt"
	"net"
	"net/http"
	"strconv"

	"github.com/rishav1305/soul/internal/ai"
	"github.com/rishav1305/soul/internal/config"
	"github.com/rishav1305/soul/internal/products"
	"github.com/rishav1305/soul/internal/session"
)

// Server is the core HTTP server for the Soul platform.
type Server struct {
	cfg      config.Config
	mux      *http.ServeMux
	sessions *session.Store
	products *products.Manager
	ai       *ai.Client
}

// New creates a Server with all routes registered.
// The products manager and AI client may be nil if not configured.
func New(cfg config.Config, pm *products.Manager, aiClient *ai.Client) *Server {
	mux := http.NewServeMux()
	s := &Server{
		cfg:      cfg,
		mux:      mux,
		sessions: session.NewStore(),
		products: pm,
		ai:       aiClient,
	}
	s.registerRoutes()
	return s
}

// Handler returns the underlying http.Handler (for testing).
func (s *Server) Handler() http.Handler {
	return s.mux
}

// ListenAndServe starts the HTTP server.
func (s *Server) ListenAndServe() error {
	addr := net.JoinHostPort(s.cfg.Host, strconv.Itoa(s.cfg.Port))
	fmt.Printf("◆ Soul server listening on %s\n", addr)
	return http.ListenAndServe(addr, s.mux)
}
