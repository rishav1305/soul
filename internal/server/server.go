package server

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"net"
	"net/http"
	"strconv"
	"sync"

	"nhooyr.io/websocket"
	"nhooyr.io/websocket/wsjson"

	"github.com/rishav1305/soul/internal/ai"
	"github.com/rishav1305/soul/internal/config"
	"github.com/rishav1305/soul/internal/planner"
	"github.com/rishav1305/soul/internal/products"
	"github.com/rishav1305/soul/internal/session"
)

// Server is the core HTTP server for the Soul platform.
type Server struct {
	cfg       config.Config
	mux       *http.ServeMux
	sessions  *session.Store
	products  *products.Manager
	ai        *ai.Client
	planner   *planner.Store
	processor *TaskProcessor
	webFS     fs.FS // embedded SPA files (nil = use placeholder)

	// wsClients tracks connected WebSocket clients for broadcasting.
	wsMu      sync.Mutex
	wsClients map[*websocket.Conn]context.Context
}

// New creates a Server with all routes registered.
// The products manager and AI client may be nil if not configured.
func New(cfg config.Config, pm *products.Manager, aiClient *ai.Client, plannerStore *planner.Store) *Server {
	mux := http.NewServeMux()
	sessions := session.NewStore()
	s := &Server{
		cfg:       cfg,
		mux:       mux,
		sessions:  sessions,
		products:  pm,
		ai:        aiClient,
		planner:   plannerStore,
		wsClients: make(map[*websocket.Conn]context.Context),
	}
	s.processor = NewTaskProcessor(aiClient, pm, sessions, plannerStore, s.broadcast, cfg.Model)
	s.registerRoutes()
	return s
}

// NewWithWebFS creates a Server that serves the SPA from the given embedded FS.
// webDist should be the top-level embed.FS containing "web/dist/".
func NewWithWebFS(cfg config.Config, pm *products.Manager, aiClient *ai.Client, plannerStore *planner.Store, webDist embed.FS) *Server {
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
	sessions := session.NewStore()
	s := &Server{
		cfg:       cfg,
		mux:       mux,
		sessions:  sessions,
		products:  pm,
		ai:        aiClient,
		planner:   plannerStore,
		webFS:     webFS,
		wsClients: make(map[*websocket.Conn]context.Context),
	}
	s.processor = NewTaskProcessor(aiClient, pm, sessions, plannerStore, s.broadcast, cfg.Model)
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

// registerWSClient adds a WebSocket connection to the tracked clients map.
func (s *Server) registerWSClient(conn *websocket.Conn, ctx context.Context) {
	s.wsMu.Lock()
	defer s.wsMu.Unlock()
	s.wsClients[conn] = ctx
}

// unregisterWSClient removes a WebSocket connection from the tracked clients map.
func (s *Server) unregisterWSClient(conn *websocket.Conn) {
	s.wsMu.Lock()
	defer s.wsMu.Unlock()
	delete(s.wsClients, conn)
}

// broadcast sends a WSMessage to all connected WebSocket clients.
func (s *Server) broadcast(msg WSMessage) {
	s.wsMu.Lock()
	clients := make(map[*websocket.Conn]context.Context, len(s.wsClients))
	for conn, ctx := range s.wsClients {
		clients[conn] = ctx
	}
	s.wsMu.Unlock()

	for conn, ctx := range clients {
		if err := wsjson.Write(ctx, conn, msg); err != nil {
			log.Printf("[ws] broadcast write error: %v", err)
		}
	}
}

// broadcastTaskEvent marshals data to JSON and broadcasts a task event.
func (s *Server) broadcastTaskEvent(eventType string, data any) {
	raw, err := json.Marshal(data)
	if err != nil {
		log.Printf("[ws] failed to marshal task event: %v", err)
		return
	}
	s.broadcast(WSMessage{
		Type: eventType,
		Data: raw,
	})
}
