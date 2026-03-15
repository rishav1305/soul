package server

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"nhooyr.io/websocket"

	"github.com/rishav1305/soul-v2/internal/mesh/hub"
	"github.com/rishav1305/soul-v2/internal/mesh/node"
	"github.com/rishav1305/soul-v2/internal/mesh/store"
	"github.com/rishav1305/soul-v2/internal/mesh/transport"
)

// Server is the mesh HTTP/WS server.
type Server struct {
	nodeInfo node.NodeInfo
	store    *store.Store
	hub      *hub.Hub
	secret   string
	srv      *http.Server
}

// New creates a mesh server.
func New(info node.NodeInfo, s *store.Store, h *hub.Hub, secret string) *Server {
	return &Server{
		nodeInfo: info,
		store:    s,
		hub:      h,
		secret:   secret,
	}
}

// ListenAndServe starts the HTTP server on host:port.
func (s *Server) ListenAndServe(host string, port int) error {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/health", s.handleHealth)
	mux.HandleFunc("GET /api/mesh/identity", s.handleIdentity)
	mux.HandleFunc("GET /api/mesh/nodes", s.handleListNodes)
	mux.HandleFunc("GET /api/mesh/status", s.handleStatus)
	mux.HandleFunc("POST /api/mesh/link", s.handleLink)
	mux.HandleFunc("GET /api/mesh/heartbeats", s.handleHeartbeats)
	mux.HandleFunc("/ws/mesh", s.handleWebSocket)
	mux.HandleFunc("POST /api/tools/{name}/execute", s.handleToolExecute)

	addr := net.JoinHostPort(host, strconv.Itoa(port))
	s.srv = &http.Server{
		Addr:         addr,
		Handler:      mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	log.Printf("mesh server listening on %s", addr)
	return s.srv.ListenAndServe()
}

// Shutdown gracefully shuts down the server.
func (s *Server) Shutdown(ctx context.Context) error {
	return s.srv.Shutdown(ctx)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleIdentity(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"id":       s.nodeInfo.ID,
		"name":     s.nodeInfo.Name,
		"host":     s.nodeInfo.Host,
		"port":     s.nodeInfo.Port,
		"platform": s.nodeInfo.Platform,
		"arch":     s.nodeInfo.Arch,
		"cpuCores": s.nodeInfo.CPUCores,
		"isHub":    s.nodeInfo.IsHub,
	})
}

func (s *Server) handleListNodes(w http.ResponseWriter, r *http.Request) {
	nodes, err := s.store.ListNodes()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if nodes == nil {
		nodes = []store.Node{}
	}
	writeJSON(w, http.StatusOK, nodes)
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	resources, err := s.hub.AggregateResources()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"nodeId":    s.nodeInfo.ID,
		"nodeName":  s.nodeInfo.Name,
		"isHub":     s.nodeInfo.IsHub,
		"resources": resources,
	})
}

func (s *Server) handleLink(w http.ResponseWriter, r *http.Request) {
	code := make([]byte, 4)
	if _, err := rand.Read(code); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "generate code"})
		return
	}
	codeStr := hex.EncodeToString(code)
	expiresAt := time.Now().UTC().Add(10 * time.Minute).Format(time.RFC3339)

	if err := s.store.CreateLinkingCode(codeStr, s.nodeInfo.ID, "", expiresAt); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"code":      codeStr,
		"expiresAt": expiresAt,
	})
}

func (s *Server) handleHeartbeats(w http.ResponseWriter, r *http.Request) {
	nodeID := r.URL.Query().Get("node_id")
	if nodeID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "node_id required"})
		return
	}
	limit := 50
	if l := r.URL.Query().Get("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 {
			limit = n
		}
	}

	hbs, err := s.store.GetRecentHeartbeats(nodeID, limit)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if hbs == nil {
		hbs = []store.Heartbeat{}
	}
	writeJSON(w, http.StatusOK, hbs)
}

func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	// JWT auth on upgrade.
	tokenStr := r.URL.Query().Get("token")
	if tokenStr == "" {
		// Also check Authorization header.
		auth := r.Header.Get("Authorization")
		if strings.HasPrefix(auth, "Bearer ") {
			tokenStr = strings.TrimPrefix(auth, "Bearer ")
		}
	}

	if tokenStr == "" {
		http.Error(w, "missing token", http.StatusUnauthorized)
		return
	}

	nodeID, err := transport.VerifyToken(tokenStr, s.secret)
	if err != nil {
		http.Error(w, "invalid token", http.StatusUnauthorized)
		return
	}

	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		InsecureSkipVerify: true,
	})
	if err != nil {
		log.Printf("mesh ws: accept error: %v", err)
		return
	}
	defer conn.Close(websocket.StatusNormalClosure, "closing")

	log.Printf("mesh ws: node %s connected", nodeID)

	ctx := r.Context()
	for {
		_, data, err := conn.Read(ctx)
		if err != nil {
			log.Printf("mesh ws: read error from %s: %v", nodeID, err)
			return
		}

		var msg transport.Message
		if err := json.Unmarshal(data, &msg); err != nil {
			log.Printf("mesh ws: invalid message from %s: %v", nodeID, err)
			continue
		}

		switch msg.Type {
		case "heartbeat":
			if err := s.hub.HandleHeartbeat(msg.NodeID, msg.Payload); err != nil {
				log.Printf("mesh ws: heartbeat error from %s: %v", msg.NodeID, err)
			}
		case "register":
			var info node.NodeInfo
			if err := json.Unmarshal(msg.Payload, &info); err != nil {
				log.Printf("mesh ws: register parse error: %v", err)
				continue
			}
			n := store.Node{
				ID:             info.ID,
				Name:           info.Name,
				Host:           info.Host,
				Port:           info.Port,
				Role:           "agent",
				Platform:       info.Platform,
				Arch:           info.Arch,
				CPUCores:       info.CPUCores,
				RAMTotalMB:     info.RAMTotalMB,
				StorageTotalGB: info.StorageTotalGB,
				Status:         "online",
			}
			if err := s.store.RegisterNode(n); err != nil {
				log.Printf("mesh ws: register error: %v", err)
			}
		case "command_result":
			log.Printf("mesh ws: command result from %s", msg.NodeID)
		default:
			log.Printf("mesh ws: unknown message type %q from %s", msg.Type, nodeID)
		}
	}
}

func (s *Server) handleToolExecute(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if name == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "tool name required"})
		return
	}

	var params json.RawMessage
	if err := json.NewDecoder(r.Body).Decode(&params); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": fmt.Sprintf("invalid body: %v", err)})
		return
	}

	// Tool dispatch placeholder — actual tool implementations will be added.
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"tool":   name,
		"status": "not_implemented",
	})
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
