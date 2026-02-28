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
	"os"
	"os/exec"
	"path/filepath"
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
	cfg         config.Config
	mux         *http.ServeMux
	sessions    *session.Store
	products    *products.Manager
	ai          *ai.Client
	planner     *planner.Store
	processor   *TaskProcessor
	projectRoot string           // working directory for code tools
	worktrees   *WorktreeManager // manages per-task git worktrees
	webFS       fs.FS            // embedded SPA files (nil = use placeholder)

	// wsClients tracks connected WebSocket clients for broadcasting.
	wsMu      sync.Mutex
	wsClients map[*websocket.Conn]context.Context
}

// New creates a Server with all routes registered.
// The products manager and AI client may be nil if not configured.
func New(cfg config.Config, pm *products.Manager, aiClient *ai.Client, plannerStore *planner.Store) *Server {
	mux := http.NewServeMux()
	sessions := session.NewStore()
	projectRoot, _ := os.Getwd()
	wm := NewWorktreeManager(projectRoot)
	if err := wm.EnsureSetup(); err != nil {
		log.Printf("WARNING: worktree setup failed: %v", err)
	}
	s := &Server{
		cfg:         cfg,
		mux:         mux,
		sessions:    sessions,
		products:    pm,
		ai:          aiClient,
		planner:     plannerStore,
		projectRoot: projectRoot,
		worktrees:   wm,
		wsClients:   make(map[*websocket.Conn]context.Context),
	}
	s.processor = NewTaskProcessor(aiClient, pm, sessions, plannerStore, s.broadcast, cfg.Model, projectRoot, wm)
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
	projectRoot, _ := os.Getwd()
	wm := NewWorktreeManager(projectRoot)
	if wmErr := wm.EnsureSetup(); wmErr != nil {
		log.Printf("WARNING: worktree setup failed: %v", wmErr)
	}
	s := &Server{
		cfg:         cfg,
		mux:         mux,
		sessions:    sessions,
		products:    pm,
		ai:          aiClient,
		planner:     plannerStore,
		projectRoot: projectRoot,
		worktrees:   wm,
		webFS:       webFS,
		wsClients:   make(map[*websocket.Conn]context.Context),
	}
	s.processor = NewTaskProcessor(aiClient, pm, sessions, plannerStore, s.broadcast, cfg.Model, projectRoot, wm)
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

// StartDevServer starts a second HTTP server on devPort, serving from the
// dev branch's web/dist/ directory. It shares the same API/WS state.
func (s *Server) StartDevServer(devPort int) {
	devRoot := filepath.Join(s.projectRoot, ".worktrees", "dev-server")

	// Create a worktree for the dev branch to serve from.
	cmd := exec.Command("git", "worktree", "add", devRoot, "dev")
	cmd.Dir = s.projectRoot
	if out, err := cmd.CombinedOutput(); err != nil {
		// Might already exist — try to update it.
		cmd = exec.Command("git", "-C", devRoot, "checkout", "dev")
		if out2, err2 := cmd.CombinedOutput(); err2 != nil {
			log.Printf("[dev-server] failed to set up dev worktree: %s / %s", out, out2)
			return
		}
	}

	// Build frontend in dev worktree.
	cmd = exec.Command("npx", "vite", "build")
	cmd.Dir = filepath.Join(devRoot, "web")
	if out, err := cmd.CombinedOutput(); err != nil {
		log.Printf("[dev-server] frontend build failed: %s — %v", out, err)
		return
	}

	// Serve dev frontend from disk.
	devDist := filepath.Join(devRoot, "web", "dist")
	devMux := http.NewServeMux()
	devMux.Handle("/", newSPAFileServer(os.DirFS(devDist)))

	// Share API and WS routes with prod.
	devMux.HandleFunc("GET /api/health", handleHealth)
	devMux.HandleFunc("GET /api/tasks", s.handleTaskList)
	devMux.HandleFunc("GET /api/tasks/{id}", s.handleTaskGet)

	addr := net.JoinHostPort(s.cfg.Host, strconv.Itoa(devPort))
	fmt.Printf("◆ Soul dev server listening on %s\n", addr)
	go func() {
		if err := http.ListenAndServe(addr, devMux); err != nil {
			log.Printf("[dev-server] error: %v", err)
		}
	}()
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
