package server

import (
	"embed"
	"fmt"
	"io/fs"
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
	webFS    fs.FS // embedded SPA files (nil = use placeholder)
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

// NewWithWebFS creates a Server that serves the SPA from the given embedded FS.
// webDist should be the top-level embed.FS containing "web/dist/".
func NewWithWebFS(cfg config.Config, pm *products.Manager, aiClient *ai.Client, webDist embed.FS) *Server {
	mux := http.NewServeMux()
	// Extract web/dist/ subtree from the embed.FS
	var webFS fs.FS
	sub, err := fs.Sub(webDist, "web/dist")
	if err == nil {
		// Check if index.html exists in the subtree
		if f, err2 := sub.Open("index.html"); err2 == nil {
			f.Close()
			webFS = sub
		}
	}
	s := &Server{
		cfg:      cfg,
		mux:      mux,
		sessions: session.NewStore(),
		products: pm,
		ai:       aiClient,
		webFS:    webFS,
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
