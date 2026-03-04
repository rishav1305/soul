# Soul Web UI — Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Build a Go core binary that serves a React SPA, runs a Claude AI agent loop, and routes tool calls to product binaries via gRPC — starting with the compliance product rewritten in Go.

**Architecture:** Go microkernel core (HTTP/WS server + Claude client + product manager) embeds a Vite React SPA. Products are standalone binaries implementing a gRPC `ProductService`. The compliance product is the first product, rewritten from TypeScript to Go with identical logic. Single binary distribution: `soul serve` starts everything.

**Tech Stack:** Go 1.22+, gRPC/protobuf, Vite + React 18 + TypeScript + TailwindCSS, WebSocket, Claude Messages API

**Prerequisites discovered:** Go and protoc are NOT installed on this machine (aarch64 Ubuntu 24.04). Node.js v22.22.0 + npm 10.9.4 available.

---

## Phase 1: Environment & Scaffold

### Task 1: Install Go

**Files:**
- None (system-level install)

**Step 1: Download and install Go for linux/arm64**

Run:
```bash
curl -fsSL https://go.dev/dl/go1.22.12.linux-arm64.tar.gz -o /tmp/go1.22.12.linux-arm64.tar.gz
sudo tar -C /usr/local -xzf /tmp/go1.22.12.linux-arm64.tar.gz
```

**Step 2: Add Go to PATH**

Add to `~/.bashrc`:
```bash
export PATH=$PATH:/usr/local/go/bin:$HOME/go/bin
```

Then:
```bash
source ~/.bashrc
```

**Step 3: Verify**

Run: `go version`
Expected: `go version go1.22.12 linux/arm64`

**Step 4: Commit**

No commit needed (system-level change).

---

### Task 2: Install protoc and Go gRPC plugins

**Files:**
- None (system-level install)

**Step 1: Install protoc**

Run:
```bash
sudo apt-get update && sudo apt-get install -y protobuf-compiler
```

**Step 2: Install Go protobuf/gRPC plugins**

Run:
```bash
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
```

**Step 3: Verify**

Run:
```bash
protoc --version
which protoc-gen-go
which protoc-gen-go-grpc
```
Expected: protoc version 3.x+, both plugins found in `$HOME/go/bin/`

**Step 4: Commit**

No commit needed (system-level change).

---

### Task 3: Initialize Go module and project skeleton

**Files:**
- Create: `go.mod`
- Create: `cmd/soul/main.go`
- Create: `Makefile`

**Step 1: Initialize Go module**

Run:
```bash
cd /home/rishav/soul
go mod init github.com/rishav1305/soul
```

**Step 2: Create entry point**

Create `cmd/soul/main.go`:
```go
package main

import (
	"fmt"
	"os"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "--version" {
		fmt.Println("Soul v0.2.0-alpha")
		return
	}

	fmt.Println("◆ Soul v0.2.0-alpha")
	fmt.Println("Usage:")
	fmt.Println("  soul serve            Start web UI")
	fmt.Println("  soul --version        Show version")
}
```

**Step 3: Verify it compiles**

Run: `go build -o /tmp/soul-test ./cmd/soul && /tmp/soul-test --version`
Expected: `Soul v0.2.0-alpha`

**Step 4: Create Makefile**

Create `Makefile`:
```makefile
.PHONY: build dev clean test proto web

VERSION := 0.2.0-alpha

# Build the full binary (React SPA + Go)
build: web
	go build -ldflags="-s -w -X main.version=$(VERSION)" -o dist/soul ./cmd/soul

# Build just Go (no frontend rebuild)
build-go:
	go build -ldflags="-s -w -X main.version=$(VERSION)" -o dist/soul ./cmd/soul

# Build React SPA
web:
	cd web && npm ci && npm run build

# Generate protobuf code
proto:
	protoc --go_out=. --go_opt=paths=source_relative \
		--go-grpc_out=. --go-grpc_opt=paths=source_relative \
		proto/soul/v1/product.proto

# Run Go tests
test:
	go test ./... -v

# Run all tests (Go + React)
test-all: test
	cd web && npm test

# Development mode (Go hot reload + Vite dev server)
dev:
	@echo "Start in two terminals:"
	@echo "  Terminal 1: cd web && npm run dev"
	@echo "  Terminal 2: go run ./cmd/soul serve --dev"

# Clean build artifacts
clean:
	rm -rf dist/ web/dist/
```

**Step 5: Verify Makefile**

Run: `make build-go`
Expected: Binary at `dist/soul`

**Step 6: Commit**

```bash
git add go.mod cmd/soul/main.go Makefile
git commit -m "feat: initialize Go module with entry point and Makefile"
```

---

### Task 4: Define protobuf service and generate Go code

**Files:**
- Create: `proto/soul/v1/product.proto`
- Generated: `proto/soul/v1/product.pb.go`
- Generated: `proto/soul/v1/product_grpc.pb.go`

**Step 1: Create proto directory**

Run: `mkdir -p proto/soul/v1`

**Step 2: Write product.proto**

Create `proto/soul/v1/product.proto`:
```protobuf
syntax = "proto3";

package soul.v1;

option go_package = "github.com/rishav1305/soul/proto/soul/v1;soulv1";

service ProductService {
  rpc GetManifest(Empty) returns (Manifest);
  rpc ExecuteTool(ToolRequest) returns (ToolResponse);
  rpc ExecuteToolStream(ToolRequest) returns (stream ToolEvent);
  rpc Health(Empty) returns (HealthResponse);
}

message Empty {}

message Manifest {
  string name = 1;
  string version = 2;
  repeated Tool tools = 3;
  repeated string tiers = 4;
}

message Tool {
  string name = 1;
  string description = 2;
  bool requires_approval = 3;
  string input_schema_json = 4;
  string tier = 5;
}

message ToolRequest {
  string tool = 1;
  string input_json = 2;
  string session_id = 3;
}

message ToolResponse {
  bool success = 1;
  string output = 2;
  string structured_json = 3;
  repeated Artifact artifacts = 4;
}

message ToolEvent {
  oneof event {
    ProgressUpdate progress = 1;
    FindingEvent finding = 2;
    ToolResponse complete = 3;
    ErrorEvent error = 4;
  }
}

message ProgressUpdate {
  string analyzer = 1;
  double percent = 2;
  string message = 3;
}

message FindingEvent {
  string id = 1;
  string title = 2;
  string severity = 3;
  string file = 4;
  int32 line = 5;
  string evidence = 6;
}

message Artifact {
  string type = 1;
  string path = 2;
  bytes content = 3;
}

message ErrorEvent {
  string code = 1;
  string message = 2;
}

message HealthResponse {
  bool healthy = 1;
  string status = 2;
}
```

**Step 3: Generate Go code**

Run:
```bash
protoc --go_out=. --go_opt=paths=source_relative \
  --go-grpc_out=. --go-grpc_opt=paths=source_relative \
  proto/soul/v1/product.proto
```

**Step 4: Verify generated files exist**

Run: `ls proto/soul/v1/*.go`
Expected: `product.pb.go` and `product_grpc.pb.go`

**Step 5: Tidy modules**

Run: `go mod tidy`

**Step 6: Verify it compiles**

Run: `go build ./proto/soul/v1/`
Expected: No errors

**Step 7: Commit**

```bash
git add proto/ go.mod go.sum
git commit -m "feat: add gRPC ProductService protobuf definition"
```

---

## Phase 2: Go Core Server

### Task 5: HTTP server with SPA serving

**Files:**
- Create: `internal/config/config.go`
- Create: `internal/server/server.go`
- Create: `internal/server/spa.go`
- Create: `internal/server/routes.go`
- Create: `web/dist/index.html` (placeholder)
- Modify: `cmd/soul/main.go`
- Test: `internal/server/server_test.go`

**Step 1: Write server tests**

Create `internal/server/server_test.go`:
```go
package server_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rishav1305/soul/internal/config"
	"github.com/rishav1305/soul/internal/server"
)

func TestHealthEndpoint(t *testing.T) {
	cfg := config.Default()
	srv := server.New(cfg)

	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestSPAFallback(t *testing.T) {
	cfg := config.Default()
	srv := server.New(cfg)

	req := httptest.NewRequest(http.MethodGet, "/some/spa/route", nil)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	// SPA fallback should serve index.html (200) for non-API routes
	if w.Code != http.StatusOK {
		t.Errorf("expected 200 for SPA fallback, got %d", w.Code)
	}
}

func TestAPINotFoundReturns404(t *testing.T) {
	cfg := config.Default()
	srv := server.New(cfg)

	req := httptest.NewRequest(http.MethodGet, "/api/nonexistent", nil)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404 for unknown API route, got %d", w.Code)
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `go test ./internal/server/ -v`
Expected: FAIL (packages don't exist yet)

**Step 3: Create config package**

Create `internal/config/config.go`:
```go
package config

import (
	"os"
	"strconv"
)

type Config struct {
	Port       int
	Host       string
	DevMode    bool
	DevUIAddr  string
	APIKey     string
	Model      string
	DataDir    string
}

func Default() Config {
	return Config{
		Port:      3000,
		Host:      "0.0.0.0",
		DevMode:   false,
		DevUIAddr: "http://localhost:5173",
		APIKey:    os.Getenv("ANTHROPIC_API_KEY"),
		Model:     envOr("SOUL_MODEL", "claude-sonnet-4-20250514"),
		DataDir:   envOr("SOUL_DATA_DIR", homeDir()+"/.soul"),
	}
}

func FromEnv() Config {
	cfg := Default()
	if p := os.Getenv("SOUL_PORT"); p != "" {
		if port, err := strconv.Atoi(p); err == nil {
			cfg.Port = port
		}
	}
	if h := os.Getenv("SOUL_HOST"); h != "" {
		cfg.Host = h
	}
	if os.Getenv("SOUL_DEV") == "true" {
		cfg.DevMode = true
	}
	return cfg
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func homeDir() string {
	if h, err := os.UserHomeDir(); err == nil {
		return h
	}
	return "/tmp"
}
```

**Step 4: Create SPA handler**

Create `internal/server/spa.go`:
```go
package server

import (
	"embed"
	"io/fs"
	"net/http"
	"strings"
)

//go:embed all:static
var staticFiles embed.FS

func spaHandler() http.Handler {
	subFS, err := fs.Sub(staticFiles, "static")
	if err != nil {
		panic("embedded static files missing: " + err.Error())
	}

	fileServer := http.FileServer(http.FS(subFS))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Try to serve the actual file first
		path := r.URL.Path
		if path == "/" {
			path = "index.html"
		} else {
			path = strings.TrimPrefix(path, "/")
		}

		// Check if file exists in embedded FS
		if f, err := subFS.Open(path); err == nil {
			f.Close()
			fileServer.ServeHTTP(w, r)
			return
		}

		// SPA fallback: serve index.html for non-file routes
		r.URL.Path = "/"
		fileServer.ServeHTTP(w, r)
	})
}
```

Create placeholder for embedded files:
```bash
mkdir -p internal/server/static
```

Create `internal/server/static/index.html`:
```html
<!DOCTYPE html>
<html><head><title>Soul</title></head>
<body><div id="root">Loading Soul...</div></body>
</html>
```

**Step 5: Create server and routes**

Create `internal/server/server.go`:
```go
package server

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/rishav1305/soul/internal/config"
)

type Server struct {
	cfg    config.Config
	mux    *http.ServeMux
}

func New(cfg config.Config) *Server {
	s := &Server{cfg: cfg, mux: http.NewServeMux()}
	s.registerRoutes()
	return s
}

func (s *Server) Handler() http.Handler {
	return s.mux
}

func (s *Server) ListenAndServe() error {
	addr := fmt.Sprintf("%s:%d", s.cfg.Host, s.cfg.Port)
	log.Printf("◆ Soul serving on http://%s", addr)
	return http.ListenAndServe(addr, s.mux)
}
```

Create `internal/server/routes.go`:
```go
package server

import (
	"encoding/json"
	"net/http"
)

func (s *Server) registerRoutes() {
	// API routes
	s.mux.HandleFunc("GET /api/health", s.handleHealth)

	// Catch-all for unknown /api/ routes → 404
	s.mux.HandleFunc("/api/", s.handleAPINotFound)

	// SPA catch-all (must be last)
	s.mux.Handle("/", spaHandler())
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"status": "ok",
	})
}

func (s *Server) handleAPINotFound(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNotFound)
	json.NewEncoder(w).Encode(map[string]any{
		"error": "not found",
	})
}
```

**Step 6: Update cmd/soul/main.go with serve command**

Replace `cmd/soul/main.go`:
```go
package main

import (
	"fmt"
	"os"
	"strconv"

	"github.com/rishav1305/soul/internal/config"
	"github.com/rishav1305/soul/internal/server"
)

var version = "0.2.0-alpha"

func main() {
	args := os.Args[1:]

	if hasFlag(args, "--version", "-v") {
		fmt.Printf("Soul v%s\n", version)
		return
	}

	if hasFlag(args, "--help", "-h") {
		printHelp()
		return
	}

	if len(args) > 0 && args[0] == "serve" {
		runServe(args[1:])
		return
	}

	printHelp()
}

func runServe(args []string) {
	cfg := config.FromEnv()

	// Parse --port flag
	for i, arg := range args {
		if arg == "--port" && i+1 < len(args) {
			if p, err := strconv.Atoi(args[i+1]); err == nil {
				cfg.Port = p
			}
		}
		if arg == "--dev" {
			cfg.DevMode = true
		}
	}

	srv := server.New(cfg)
	if err := srv.ListenAndServe(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func printHelp() {
	fmt.Printf("◆ Soul v%s\n\n", version)
	fmt.Println("Usage:")
	fmt.Println("  soul serve [--port PORT] [--dev]   Start web UI")
	fmt.Println("  soul --version                     Show version")
	fmt.Println("  soul --help                        Show this help")
}

func hasFlag(args []string, flags ...string) bool {
	for _, arg := range args {
		for _, flag := range flags {
			if arg == flag {
				return true
			}
		}
	}
	return false
}
```

**Step 7: Run tests**

Run: `go test ./internal/... -v`
Expected: 3 tests pass

**Step 8: Verify binary runs**

Run: `go build -o /tmp/soul-test ./cmd/soul && /tmp/soul-test --version`
Expected: `Soul v0.2.0-alpha`

**Step 9: Commit**

```bash
git add internal/ cmd/ go.mod go.sum
git commit -m "feat: add Go HTTP server with SPA serving and health endpoint"
```

---

### Task 6: WebSocket server for chat streaming

**Files:**
- Create: `internal/server/ws.go`
- Create: `internal/session/session.go`
- Create: `internal/session/store.go`
- Modify: `internal/server/routes.go`
- Test: `internal/server/ws_test.go`

**Step 1: Install WebSocket dependency**

Run: `go get nhooyr.io/websocket`

**Step 2: Write WebSocket tests**

Create `internal/server/ws_test.go`:
```go
package server_test

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"testing"
	"time"

	"nhooyr.io/websocket"
	"nhooyr.io/websocket/wsjson"

	"github.com/rishav1305/soul/internal/config"
	"github.com/rishav1305/soul/internal/server"
)

func TestWebSocketConnect(t *testing.T) {
	cfg := config.Default()
	srv := server.New(cfg)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	wsURL := "ws" + ts.URL[4:] + "/ws"
	conn, _, err := websocket.Dial(ctx, wsURL, nil)
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer conn.CloseNow()

	// Send a chat.send message
	msg := map[string]any{
		"type":       "chat.send",
		"session_id": "test-session",
		"content":    "hello",
	}
	if err := wsjson.Write(ctx, conn, msg); err != nil {
		t.Fatalf("failed to send: %v", err)
	}

	// Should receive at least a chat.done (even without AI client)
	var resp map[string]any
	if err := wsjson.Read(ctx, conn, &resp); err != nil {
		t.Fatalf("failed to read: %v", err)
	}
	if resp["type"] == nil {
		t.Error("response missing type field")
	}
}

func TestWebSocketInvalidMessage(t *testing.T) {
	cfg := config.Default()
	srv := server.New(cfg)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	wsURL := "ws" + ts.URL[4:] + "/ws"
	conn, _, err := websocket.Dial(ctx, wsURL, nil)
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer conn.CloseNow()

	// Send invalid message
	msg := map[string]any{"type": "invalid.type"}
	wsjson.Write(ctx, conn, msg)

	var resp map[string]any
	if err := wsjson.Read(ctx, conn, &resp); err != nil {
		t.Fatalf("failed to read: %v", err)
	}
	if resp["type"] != "error" {
		t.Errorf("expected error type, got %v", resp["type"])
	}
}
```

**Step 3: Run tests to verify they fail**

Run: `go test ./internal/server/ -v`
Expected: FAIL

**Step 4: Create session package**

Create `internal/session/session.go`:
```go
package session

import "time"

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type Session struct {
	ID        string    `json:"id"`
	Messages  []Message `json:"messages"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func New(id string) *Session {
	now := time.Now()
	return &Session{
		ID:        id,
		Messages:  []Message{},
		CreatedAt: now,
		UpdatedAt: now,
	}
}

func (s *Session) AddMessage(role, content string) {
	s.Messages = append(s.Messages, Message{Role: role, Content: content})
	s.UpdatedAt = time.Now()
}
```

Create `internal/session/store.go`:
```go
package session

import "sync"

type Store struct {
	mu       sync.RWMutex
	sessions map[string]*Session
}

func NewStore() *Store {
	return &Store{sessions: make(map[string]*Session)}
}

func (s *Store) Get(id string) *Session {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.sessions[id]
}

func (s *Store) GetOrCreate(id string) *Session {
	s.mu.Lock()
	defer s.mu.Unlock()
	if sess, ok := s.sessions[id]; ok {
		return sess
	}
	sess := New(id)
	s.sessions[id] = sess
	return sess
}
```

**Step 5: Create WebSocket handler**

Create `internal/server/ws.go`:
```go
package server

import (
	"context"
	"encoding/json"
	"log"
	"net/http"

	"nhooyr.io/websocket"
	"nhooyr.io/websocket/wsjson"
)

// WSMessage is the envelope for all WebSocket messages.
type WSMessage struct {
	Type      string          `json:"type"`
	SessionID string          `json:"session_id,omitempty"`
	Content   string          `json:"content,omitempty"`
	Data      json.RawMessage `json:"data,omitempty"`
}

func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		OriginPatterns: []string{"*"},
	})
	if err != nil {
		log.Printf("websocket accept error: %v", err)
		return
	}
	defer conn.CloseNow()

	ctx := r.Context()

	for {
		var msg WSMessage
		if err := wsjson.Read(ctx, conn, &msg); err != nil {
			// Client disconnected
			return
		}

		switch msg.Type {
		case "chat.send":
			s.handleChatSend(ctx, conn, msg)
		default:
			wsjson.Write(ctx, conn, WSMessage{
				Type:    "error",
				Content: "unknown message type: " + msg.Type,
			})
		}
	}
}

func (s *Server) handleChatSend(ctx context.Context, conn *websocket.Conn, msg WSMessage) {
	sessionID := msg.SessionID
	if sessionID == "" {
		sessionID = "default"
	}

	sess := s.sessions.GetOrCreate(sessionID)
	sess.AddMessage("user", msg.Content)

	// TODO: call Claude AI client here
	// For now, echo back a placeholder response
	wsjson.Write(ctx, conn, WSMessage{
		Type:      "chat.token",
		SessionID: sessionID,
		Content:   "AI responses coming soon. You said: " + msg.Content,
	})

	wsjson.Write(ctx, conn, WSMessage{
		Type:      "chat.done",
		SessionID: sessionID,
	})
}
```

**Step 6: Update server.go to include session store**

Update `internal/server/server.go` — add session store to Server struct:
```go
package server

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/rishav1305/soul/internal/config"
	"github.com/rishav1305/soul/internal/session"
)

type Server struct {
	cfg      config.Config
	mux      *http.ServeMux
	sessions *session.Store
}

func New(cfg config.Config) *Server {
	s := &Server{
		cfg:      cfg,
		mux:      http.NewServeMux(),
		sessions: session.NewStore(),
	}
	s.registerRoutes()
	return s
}

func (s *Server) Handler() http.Handler {
	return s.mux
}

func (s *Server) ListenAndServe() error {
	addr := fmt.Sprintf("%s:%d", s.cfg.Host, s.cfg.Port)
	log.Printf("◆ Soul serving on http://%s", addr)
	return http.ListenAndServe(addr, s.mux)
}
```

**Step 7: Register WebSocket route**

Update `internal/server/routes.go` — add WebSocket route:
```go
func (s *Server) registerRoutes() {
	// API routes
	s.mux.HandleFunc("GET /api/health", s.handleHealth)
	s.mux.HandleFunc("GET /ws", s.handleWebSocket)

	// Catch-all for unknown /api/ routes → 404
	s.mux.HandleFunc("/api/", s.handleAPINotFound)

	// SPA catch-all (must be last)
	s.mux.Handle("/", spaHandler())
}
```

**Step 8: Run tests**

Run: `go test ./internal/... -v`
Expected: All tests pass (including previous server tests + new WS tests)

**Step 9: Commit**

```bash
git add internal/ go.mod go.sum
git commit -m "feat: add WebSocket server with session management"
```

---

### Task 7: Product manager and gRPC proxy

**Files:**
- Create: `internal/products/manager.go`
- Create: `internal/products/registry.go`
- Create: `internal/products/proxy.go`
- Test: `internal/products/manager_test.go`
- Test: `internal/products/registry_test.go`

**Step 1: Install gRPC dependency**

Run: `go get google.golang.org/grpc`

**Step 2: Write registry tests**

Create `internal/products/registry_test.go`:
```go
package products_test

import (
	"testing"

	"github.com/rishav1305/soul/internal/products"
	soulv1 "github.com/rishav1305/soul/proto/soul/v1"
)

func TestRegistryAddAndGet(t *testing.T) {
	reg := products.NewRegistry()
	manifest := &soulv1.Manifest{
		Name:    "compliance",
		Version: "0.2.0",
		Tools: []*soulv1.Tool{
			{Name: "scan", Description: "Scan a directory"},
			{Name: "fix", Description: "Fix issues"},
		},
	}

	reg.Register("compliance", manifest)

	// Get product
	m, ok := reg.Get("compliance")
	if !ok {
		t.Fatal("expected compliance product to be registered")
	}
	if m.Name != "compliance" {
		t.Errorf("expected name compliance, got %s", m.Name)
	}

	// List all tools (flattened with product prefix)
	tools := reg.AllTools()
	if len(tools) != 2 {
		t.Fatalf("expected 2 tools, got %d", len(tools))
	}
	if tools[0].ProductName != "compliance" || tools[0].Tool.Name != "scan" {
		t.Errorf("unexpected tool: %+v", tools[0])
	}
}

func TestRegistryFindTool(t *testing.T) {
	reg := products.NewRegistry()
	manifest := &soulv1.Manifest{
		Name:    "compliance",
		Version: "0.2.0",
		Tools: []*soulv1.Tool{
			{Name: "scan", Description: "Scan a directory"},
		},
	}
	reg.Register("compliance", manifest)

	// Find tool by qualified name (compliance__scan)
	entry, ok := reg.FindTool("compliance__scan")
	if !ok {
		t.Fatal("expected to find compliance__scan")
	}
	if entry.Tool.Name != "scan" {
		t.Errorf("expected scan, got %s", entry.Tool.Name)
	}

	// Not found
	_, ok = reg.FindTool("compliance__nonexistent")
	if ok {
		t.Error("expected not found")
	}
}
```

**Step 3: Run tests to verify they fail**

Run: `go test ./internal/products/ -v`
Expected: FAIL

**Step 4: Create registry**

Create `internal/products/registry.go`:
```go
package products

import (
	"strings"
	"sync"

	soulv1 "github.com/rishav1305/soul/proto/soul/v1"
)

type ToolEntry struct {
	ProductName string
	Tool        *soulv1.Tool
}

type Registry struct {
	mu        sync.RWMutex
	manifests map[string]*soulv1.Manifest
}

func NewRegistry() *Registry {
	return &Registry{manifests: make(map[string]*soulv1.Manifest)}
}

func (r *Registry) Register(name string, manifest *soulv1.Manifest) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.manifests[name] = manifest
}

func (r *Registry) Get(name string) (*soulv1.Manifest, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	m, ok := r.manifests[name]
	return m, ok
}

func (r *Registry) AllTools() []ToolEntry {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var entries []ToolEntry
	for name, m := range r.manifests {
		for _, tool := range m.Tools {
			entries = append(entries, ToolEntry{ProductName: name, Tool: tool})
		}
	}
	return entries
}

// FindTool looks up a tool by its qualified name (product__tool).
func (r *Registry) FindTool(qualifiedName string) (ToolEntry, bool) {
	parts := strings.SplitN(qualifiedName, "__", 2)
	if len(parts) != 2 {
		return ToolEntry{}, false
	}
	productName, toolName := parts[0], parts[1]

	r.mu.RLock()
	defer r.mu.RUnlock()
	m, ok := r.manifests[productName]
	if !ok {
		return ToolEntry{}, false
	}
	for _, tool := range m.Tools {
		if tool.Name == toolName {
			return ToolEntry{ProductName: productName, Tool: tool}, true
		}
	}
	return ToolEntry{}, false
}
```

**Step 5: Create product manager**

Create `internal/products/manager.go`:
```go
package products

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	soulv1 "github.com/rishav1305/soul/proto/soul/v1"
)

type ProductProcess struct {
	Name       string
	BinaryPath string
	SocketPath string
	Cmd        *exec.Cmd
	Client     soulv1.ProductServiceClient
	Conn       *grpc.ClientConn
}

type Manager struct {
	mu        sync.Mutex
	registry  *Registry
	products  map[string]*ProductProcess
	dataDir   string
}

func NewManager(registry *Registry, dataDir string) *Manager {
	return &Manager{
		registry: registry,
		products: make(map[string]*ProductProcess),
		dataDir:  dataDir,
	}
}

func (m *Manager) Registry() *Registry {
	return m.registry
}

// StartProduct launches a product binary and connects via gRPC.
func (m *Manager) StartProduct(ctx context.Context, name, binaryPath string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	socketDir := filepath.Join(m.dataDir, "sockets")
	os.MkdirAll(socketDir, 0o755)
	socketPath := filepath.Join(socketDir, name+".sock")

	// Remove stale socket
	os.Remove(socketPath)

	cmd := exec.CommandContext(ctx, binaryPath, "--socket", socketPath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start product %s: %w", name, err)
	}

	// Wait for socket to appear
	for i := 0; i < 50; i++ {
		if _, err := os.Stat(socketPath); err == nil {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	// Connect gRPC
	conn, err := grpc.NewClient(
		"unix://"+socketPath,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		cmd.Process.Kill()
		return fmt.Errorf("connect to product %s: %w", name, err)
	}

	client := soulv1.NewProductServiceClient(conn)

	// Get manifest
	manifest, err := client.GetManifest(ctx, &soulv1.Empty{})
	if err != nil {
		conn.Close()
		cmd.Process.Kill()
		return fmt.Errorf("get manifest from %s: %w", name, err)
	}

	m.registry.Register(name, manifest)

	m.products[name] = &ProductProcess{
		Name:       name,
		BinaryPath: binaryPath,
		SocketPath: socketPath,
		Cmd:        cmd,
		Client:     client,
		Conn:       conn,
	}

	log.Printf("Product %s started (tools: %d)", name, len(manifest.Tools))
	return nil
}

// GetClient returns the gRPC client for a product.
func (m *Manager) GetClient(name string) (soulv1.ProductServiceClient, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	p, ok := m.products[name]
	if !ok {
		return nil, false
	}
	return p.Client, true
}

// StopAll stops all product processes.
func (m *Manager) StopAll() {
	m.mu.Lock()
	defer m.mu.Unlock()
	for name, p := range m.products {
		log.Printf("Stopping product %s", name)
		p.Conn.Close()
		p.Cmd.Process.Kill()
		os.Remove(p.SocketPath)
	}
}
```

**Step 6: Create gRPC proxy**

Create `internal/products/proxy.go`:
```go
package products

import (
	"context"
	"fmt"

	soulv1 "github.com/rishav1305/soul/proto/soul/v1"
)

// ExecuteTool routes a tool call to the correct product via gRPC.
func (m *Manager) ExecuteTool(ctx context.Context, qualifiedName string, inputJSON string, sessionID string) (*soulv1.ToolResponse, error) {
	entry, ok := m.registry.FindTool(qualifiedName)
	if !ok {
		return nil, fmt.Errorf("tool not found: %s", qualifiedName)
	}

	client, ok := m.GetClient(entry.ProductName)
	if !ok {
		return nil, fmt.Errorf("product not running: %s", entry.ProductName)
	}

	return client.ExecuteTool(ctx, &soulv1.ToolRequest{
		Tool:      entry.Tool.Name,
		InputJson: inputJSON,
		SessionId: sessionID,
	})
}

// ExecuteToolStream routes a streaming tool call to the correct product via gRPC.
func (m *Manager) ExecuteToolStream(ctx context.Context, qualifiedName string, inputJSON string, sessionID string) (soulv1.ProductService_ExecuteToolStreamClient, error) {
	entry, ok := m.registry.FindTool(qualifiedName)
	if !ok {
		return nil, fmt.Errorf("tool not found: %s", qualifiedName)
	}

	client, ok := m.GetClient(entry.ProductName)
	if !ok {
		return nil, fmt.Errorf("product not running: %s", entry.ProductName)
	}

	return client.ExecuteToolStream(ctx, &soulv1.ToolRequest{
		Tool:      entry.Tool.Name,
		InputJson: inputJSON,
		SessionId: sessionID,
	})
}
```

**Step 7: Run tests**

Run: `go test ./internal/products/ -v`
Expected: Registry tests pass

**Step 8: Tidy and verify**

Run: `go mod tidy && go build ./...`
Expected: Compiles without errors

**Step 9: Commit**

```bash
git add internal/products/ go.mod go.sum
git commit -m "feat: add product manager with gRPC registry and proxy"
```

---

### Task 8: Claude AI client with streaming

**Files:**
- Create: `internal/ai/client.go`
- Create: `internal/ai/stream.go`
- Create: `internal/ai/tools.go`
- Test: `internal/ai/client_test.go`

**Step 1: Write AI client tests**

Create `internal/ai/client_test.go`:
```go
package ai_test

import (
	"testing"

	"github.com/rishav1305/soul/internal/ai"
	soulv1 "github.com/rishav1305/soul/proto/soul/v1"
)

func TestBuildToolsFromRegistry(t *testing.T) {
	tools := []*soulv1.Tool{
		{
			Name:            "scan",
			Description:     "Scan a directory",
			InputSchemaJson: `{"type":"object","properties":{"directory":{"type":"string"}}}`,
		},
	}

	claudeTools := ai.BuildClaudeTools("compliance", tools)
	if len(claudeTools) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(claudeTools))
	}
	if claudeTools[0].Name != "compliance__scan" {
		t.Errorf("expected compliance__scan, got %s", claudeTools[0].Name)
	}
}

func TestNewClient(t *testing.T) {
	// Should not panic with empty key (just won't work for actual calls)
	client := ai.NewClient("", "claude-sonnet-4-20250514")
	if client == nil {
		t.Fatal("expected non-nil client")
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `go test ./internal/ai/ -v`
Expected: FAIL

**Step 3: Create tool conversion**

Create `internal/ai/tools.go`:
```go
package ai

import (
	"encoding/json"

	soulv1 "github.com/rishav1305/soul/proto/soul/v1"
)

// ClaudeTool represents a tool definition for the Claude Messages API.
type ClaudeTool struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"input_schema"`
}

// BuildClaudeTools converts product tools to Claude tool_use format.
// Tool names are prefixed with product name: {product}__{tool}
func BuildClaudeTools(productName string, tools []*soulv1.Tool) []ClaudeTool {
	var result []ClaudeTool
	for _, tool := range tools {
		schema := json.RawMessage(tool.InputSchemaJson)
		if !json.Valid(schema) {
			schema = json.RawMessage(`{"type":"object"}`)
		}
		result = append(result, ClaudeTool{
			Name:        productName + "__" + tool.Name,
			Description: tool.Description,
			InputSchema: schema,
		})
	}
	return result
}
```

**Step 4: Create AI client**

Create `internal/ai/client.go`:
```go
package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

const claudeAPIURL = "https://api.anthropic.com/v1/messages"
const claudeAPIVersion = "2023-06-01"

type Client struct {
	apiKey     string
	model      string
	httpClient *http.Client
}

func NewClient(apiKey, model string) *Client {
	return &Client{
		apiKey:     apiKey,
		model:      model,
		httpClient: &http.Client{},
	}
}

// Message represents a conversation message.
type Message struct {
	Role    string `json:"role"`
	Content any    `json:"content"`
}

// Request represents a Claude Messages API request.
type Request struct {
	Model     string      `json:"model"`
	MaxTokens int         `json:"max_tokens"`
	System    string      `json:"system,omitempty"`
	Messages  []Message   `json:"messages"`
	Tools     []ClaudeTool `json:"tools,omitempty"`
	Stream    bool        `json:"stream"`
}

// SendStream sends a streaming request to the Claude Messages API.
// Returns a reader for SSE events.
func (c *Client) SendStream(ctx context.Context, req Request) (io.ReadCloser, error) {
	req.Stream = true
	if req.Model == "" {
		req.Model = c.model
	}
	if req.MaxTokens == 0 {
		req.MaxTokens = 4096
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", claudeAPIURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("X-API-Key", c.apiKey)
	httpReq.Header.Set("anthropic-version", claudeAPIVersion)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		defer resp.Body.Close()
		errBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("claude API error %d: %s", resp.StatusCode, string(errBody))
	}

	return resp.Body, nil
}
```

**Step 5: Create SSE stream parser**

Create `internal/ai/stream.go`:
```go
package ai

import (
	"bufio"
	"encoding/json"
	"io"
	"strings"
)

// StreamEvent represents a parsed SSE event from Claude.
type StreamEvent struct {
	Type string
	Data json.RawMessage
}

// ParseSSEStream reads SSE events from a reader and sends them on a channel.
func ParseSSEStream(r io.Reader, events chan<- StreamEvent) {
	defer close(events)
	scanner := bufio.NewScanner(r)
	var eventType string

	for scanner.Scan() {
		line := scanner.Text()

		if strings.HasPrefix(line, "event: ") {
			eventType = strings.TrimPrefix(line, "event: ")
			continue
		}

		if strings.HasPrefix(line, "data: ") {
			data := strings.TrimPrefix(line, "data: ")
			events <- StreamEvent{
				Type: eventType,
				Data: json.RawMessage(data),
			}
			continue
		}
	}
}

// ContentBlockDelta is the shape of a content_block_delta event.
type ContentBlockDelta struct {
	Type  string `json:"type"`
	Index int    `json:"index"`
	Delta struct {
		Type string `json:"type"`
		Text string `json:"text,omitempty"`

		// For tool_use input
		PartialJSON string `json:"partial_json,omitempty"`
	} `json:"delta"`
}

// ContentBlockStart is the shape of a content_block_start event.
type ContentBlockStart struct {
	Type         string `json:"type"`
	Index        int    `json:"index"`
	ContentBlock struct {
		Type  string `json:"type"`
		ID    string `json:"id,omitempty"`
		Name  string `json:"name,omitempty"`
		Text  string `json:"text,omitempty"`
		Input json.RawMessage `json:"input,omitempty"`
	} `json:"content_block"`
}
```

**Step 6: Run tests**

Run: `go test ./internal/ai/ -v`
Expected: All tests pass

**Step 7: Commit**

```bash
git add internal/ai/ go.mod go.sum
git commit -m "feat: add Claude AI client with SSE streaming and tool conversion"
```

---

## Phase 3: Compliance Product (Go Rewrite)

### Task 9: Compliance Go scaffold and rule loader

**Files:**
- Create: `products/compliance-go/main.go`
- Create: `products/compliance-go/rules/loader.go`
- Create: `products/compliance-go/rules/soc2.yaml` (copy from TS)
- Create: `products/compliance-go/rules/hipaa.yaml` (copy from TS)
- Create: `products/compliance-go/rules/gdpr.yaml` (copy from TS)
- Create: `products/compliance-go/go.mod`
- Test: `products/compliance-go/rules/loader_test.go`

**Step 1: Initialize compliance Go module**

Run:
```bash
mkdir -p products/compliance-go/rules
cd /home/rishav/soul/products/compliance-go
go mod init github.com/rishav1305/soul/products/compliance-go
```

**Step 2: Copy YAML rules from TS product**

Run:
```bash
cp /home/rishav/soul/products/compliance/src/rules/soc2.yaml /home/rishav/soul/products/compliance-go/rules/
cp /home/rishav/soul/products/compliance/src/rules/hipaa.yaml /home/rishav/soul/products/compliance-go/rules/
cp /home/rishav/soul/products/compliance/src/rules/gdpr.yaml /home/rishav/soul/products/compliance-go/rules/
```

**Step 3: Write rule loader tests**

Create `products/compliance-go/rules/loader_test.go`:
```go
package rules_test

import (
	"testing"

	"github.com/rishav1305/soul/products/compliance-go/rules"
)

func TestLoadAllRules(t *testing.T) {
	all := rules.Load(nil)
	if len(all) == 0 {
		t.Fatal("expected rules to load, got 0")
	}
	// Design doc says 83 rules total
	if len(all) < 80 {
		t.Errorf("expected ~83 rules, got %d", len(all))
	}
}

func TestLoadRulesFilterByFramework(t *testing.T) {
	soc2Only := rules.Load([]string{"soc2"})
	all := rules.Load(nil)

	if len(soc2Only) >= len(all) {
		t.Errorf("soc2-only (%d) should be fewer than all (%d)", len(soc2Only), len(all))
	}

	for _, r := range soc2Only {
		found := false
		for _, fw := range r.Framework {
			if fw == "soc2" {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("rule %s should have soc2 framework", r.ID)
		}
	}
}

func TestRuleFields(t *testing.T) {
	all := rules.Load(nil)
	for _, r := range all {
		if r.ID == "" {
			t.Error("rule missing ID")
		}
		if r.Severity == "" {
			t.Error("rule missing severity")
		}
		if r.Analyzer == "" {
			t.Errorf("rule %s missing analyzer", r.ID)
		}
		if len(r.Framework) == 0 {
			t.Errorf("rule %s missing framework", r.ID)
		}
	}
}
```

**Step 4: Run tests to verify they fail**

Run: `cd /home/rishav/soul/products/compliance-go && go test ./rules/ -v`
Expected: FAIL

**Step 5: Create rule loader with go:embed**

Create `products/compliance-go/rules/loader.go`:
```go
package rules

import (
	"embed"

	"gopkg.in/yaml.v3"
)

//go:embed soc2.yaml hipaa.yaml gdpr.yaml
var ruleFiles embed.FS

type Rule struct {
	ID          string   `yaml:"id"`
	Title       string   `yaml:"title"`
	Severity    string   `yaml:"severity"`
	Analyzer    string   `yaml:"analyzer"`
	Pattern     string   `yaml:"pattern"`
	Controls    []string `yaml:"controls"`
	Framework   []string `yaml:"framework"`
	Description string   `yaml:"description"`
	Fixable     bool     `yaml:"fixable"`
}

// Load reads all embedded YAML rules, optionally filtering by framework.
func Load(frameworks []string) []Rule {
	files := []string{"soc2.yaml", "hipaa.yaml", "gdpr.yaml"}
	var all []Rule

	for _, name := range files {
		data, err := ruleFiles.ReadFile(name)
		if err != nil {
			continue
		}
		var rules []Rule
		if err := yaml.Unmarshal(data, &rules); err != nil {
			continue
		}
		all = append(all, rules...)
	}

	if len(frameworks) == 0 {
		return all
	}

	fwSet := make(map[string]bool, len(frameworks))
	for _, fw := range frameworks {
		fwSet[fw] = true
	}

	var filtered []Rule
	for _, r := range all {
		for _, fw := range r.Framework {
			if fwSet[fw] {
				filtered = append(filtered, r)
				break
			}
		}
	}
	return filtered
}
```

**Step 6: Install yaml dependency and tidy**

Run:
```bash
cd /home/rishav/soul/products/compliance-go
go get gopkg.in/yaml.v3
go mod tidy
```

**Step 7: Run tests**

Run: `cd /home/rishav/soul/products/compliance-go && go test ./rules/ -v`
Expected: All 3 tests pass

**Step 8: Commit**

```bash
cd /home/rishav/soul
git add products/compliance-go/
git commit -m "feat(compliance-go): add rule loader with embedded YAML files"
```

---

### Task 10: Secret scanner analyzer (Go)

**Files:**
- Create: `products/compliance-go/analyzers/types.go`
- Create: `products/compliance-go/analyzers/secret_scanner.go`
- Test: `products/compliance-go/analyzers/secret_scanner_test.go`

**Step 1: Write types**

Create `products/compliance-go/analyzers/types.go`:
```go
package analyzers

import (
	"github.com/rishav1305/soul/products/compliance-go/rules"
)

type Finding struct {
	ID          string   `json:"id"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Severity    string   `json:"severity"`
	Framework   []string `json:"framework"`
	ControlIDs  []string `json:"control_ids"`
	File        string   `json:"file,omitempty"`
	Line        int      `json:"line,omitempty"`
	Column      int      `json:"column,omitempty"`
	Evidence    string   `json:"evidence,omitempty"`
	Analyzer    string   `json:"analyzer"`
	Fixable     bool     `json:"fixable"`
}

type ScannedFile struct {
	Path         string
	RelativePath string
	Extension    string
	Size         int64
}

type Analyzer interface {
	Name() string
	Analyze(files []ScannedFile, rules []rules.Rule) ([]Finding, error)
}
```

**Step 2: Write secret scanner tests**

Create `products/compliance-go/analyzers/secret_scanner_test.go`:
```go
package analyzers_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/rishav1305/soul/products/compliance-go/analyzers"
	"github.com/rishav1305/soul/products/compliance-go/rules"
)

func TestSecretScannerFindsAWSKey(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "config.ts")
	os.WriteFile(filePath, []byte(`const key = "AKIAIOSFODNN7EXAMPLE";`), 0o644)

	scanner := &analyzers.SecretScanner{}
	files := []analyzers.ScannedFile{
		{Path: filePath, RelativePath: "config.ts", Extension: "ts", Size: 100},
	}
	allRules := rules.Load(nil)

	findings, err := scanner.Analyze(files, allRules)
	if err != nil {
		t.Fatal(err)
	}
	if len(findings) == 0 {
		t.Fatal("expected at least one finding for AWS key")
	}

	found := false
	for _, f := range findings {
		if f.ID == "SECRET-001" || f.ID == "SECRET-002" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected SECRET-001 or SECRET-002 finding")
	}
}

func TestSecretScannerSkipsNonTextFiles(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "image.png")
	os.WriteFile(filePath, []byte(`AKIAIOSFODNN7EXAMPLE`), 0o644)

	scanner := &analyzers.SecretScanner{}
	files := []analyzers.ScannedFile{
		{Path: filePath, RelativePath: "image.png", Extension: "png", Size: 100},
	}
	allRules := rules.Load(nil)

	findings, err := scanner.Analyze(files, allRules)
	if err != nil {
		t.Fatal(err)
	}
	if len(findings) != 0 {
		t.Errorf("expected 0 findings for non-text file, got %d", len(findings))
	}
}

func TestSecretScannerHighEntropy(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "secrets.ts")
	// A high-entropy string that should trigger the entropy detector
	os.WriteFile(filePath, []byte(`const token = "aB3cD4eF5gH6iJ7kL8mN9oP0qR1sT2u";`), 0o644)

	scanner := &analyzers.SecretScanner{}
	files := []analyzers.ScannedFile{
		{Path: filePath, RelativePath: "secrets.ts", Extension: "ts", Size: 200},
	}
	allRules := rules.Load(nil)

	findings, err := scanner.Analyze(files, allRules)
	if err != nil {
		t.Fatal(err)
	}
	// May or may not find depending on entropy threshold, but should not error
	_ = findings
}

func TestSecretScannerRedactsEvidence(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "app.ts")
	os.WriteFile(filePath, []byte(`const key = "AKIAIOSFODNN7EXAMPLE";`), 0o644)

	scanner := &analyzers.SecretScanner{}
	files := []analyzers.ScannedFile{
		{Path: filePath, RelativePath: "app.ts", Extension: "ts", Size: 100},
	}
	allRules := rules.Load(nil)

	findings, err := scanner.Analyze(files, allRules)
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range findings {
		if f.Evidence != "" && len(f.Evidence) > 12 {
			// Should contain **** (redacted)
			if f.Evidence == "AKIAIOSFODNN7EXAMPLE" {
				t.Error("evidence should be redacted, not raw secret")
			}
		}
	}
}
```

**Step 3: Run tests to verify they fail**

Run: `cd /home/rishav/soul/products/compliance-go && go test ./analyzers/ -v`
Expected: FAIL

**Step 4: Implement secret scanner**

Create `products/compliance-go/analyzers/secret_scanner.go`:
```go
package analyzers

import (
	"math"
	"os"
	"regexp"
	"strings"

	"github.com/rishav1305/soul/products/compliance-go/rules"
)

var textExtensions = map[string]bool{
	"ts": true, "js": true, "py": true, "go": true, "java": true, "rb": true,
	"yaml": true, "yml": true, "json": true, "toml": true,
	"env": true, "cfg": true, "conf": true, "ini": true, "xml": true, "properties": true,
}

const maxFileSize = 500 * 1024 // 500 KB

type secretPattern struct {
	name        string
	regex       *regexp.Regexp
	rulePattern string
}

var secretPatterns = []secretPattern{
	{"AWS Access Key", regexp.MustCompile(`AKIA[0-9A-Z]{16}`), "hardcoded-credential"},
	{"AWS Secret Key", regexp.MustCompile(`(?:aws_secret|AWS_SECRET|secret_key|SECRET_KEY)\s*[=:]\s*['"]?([A-Za-z0-9/+=]{40})['"]?`), "hardcoded-credential"},
	{"GitHub Token", regexp.MustCompile(`gh[ps]_[A-Za-z0-9_]{36,}`), "api-token"},
	{"Private Key", regexp.MustCompile(`-----BEGIN\s+(?:RSA|EC|DSA|PGP)?\s*PRIVATE KEY-----`), "private-key"},
	{"JWT Token", regexp.MustCompile(`eyJ[A-Za-z0-9\-_]+\.eyJ[A-Za-z0-9\-_]+\.[A-Za-z0-9\-_.+/=]+`), "api-token"},
	{"Slack Token", regexp.MustCompile(`xox[bpras]-[0-9a-zA-Z-]+`), "api-token"},
	{"Stripe Key", regexp.MustCompile(`sk_(?:live|test)_[0-9a-zA-Z]{24,}`), "api-token"},
	{"Anthropic Key", regexp.MustCompile(`sk-ant-[a-zA-Z0-9-]+`), "api-token"},
	{"Generic Password", regexp.MustCompile(`(?i)(?:password|passwd|pwd|secret)\s*[=:]\s*['"][^'"]{4,}['"]`), "hardcoded-credential"},
	{"Generic API Key", regexp.MustCompile(`(?i)(?:api[_\-]?key|apikey)\s*[=:]\s*['"][^'"]{8,}['"]`), "hardcoded-credential"},
	{"Database URL", regexp.MustCompile(`(?:mongodb|postgres|mysql|redis)://[^\s'"]+:[^\s'"]+@`), "hardcoded-credential"},
	{"Google API Key", regexp.MustCompile(`AIza[0-9A-Za-z\-_]{35}`), "api-token"},
	{"Heroku API Key", regexp.MustCompile(`[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}`), "api-token"},
	{"SendGrid API Key", regexp.MustCompile(`SG\.[0-9A-Za-z\-_]{22,}\.[0-9A-Za-z\-_]{22,}`), "api-token"},
	{"Twilio API Key", regexp.MustCompile(`SK[0-9a-fA-F]{32}`), "api-token"},
	{"Mailgun API Key", regexp.MustCompile(`key-[0-9a-zA-Z]{32}`), "api-token"},
}

var highEntropyRegex = regexp.MustCompile(`['"]([A-Za-z0-9+/=\-_]{20,})['"]`)

type SecretScanner struct{}

func (s *SecretScanner) Name() string { return "secret-scanner" }

func (s *SecretScanner) Analyze(files []ScannedFile, allRules []rules.Rule) ([]Finding, error) {
	myRules := filterRules(allRules, s.Name())
	if len(myRules) == 0 {
		return nil, nil
	}

	rulesByPattern := groupRulesByPattern(myRules)
	var findings []Finding

	for _, file := range files {
		if !textExtensions[strings.ToLower(file.Extension)] {
			continue
		}
		if file.Size > maxFileSize {
			continue
		}

		content, err := os.ReadFile(file.Path)
		if err != nil {
			continue
		}

		// Skip binary files
		if strings.Contains(string(content), "\x00") {
			continue
		}

		lines := strings.Split(string(content), "\n")
		for lineIdx, line := range lines {
			// Check each secret pattern
			for _, sp := range secretPatterns {
				matches := sp.regex.FindAllStringIndex(line, -1)
				for _, loc := range matches {
					matched := line[loc[0]:loc[1]]
					matchedRules := rulesByPattern[sp.rulePattern]
					for _, r := range matchedRules {
						findings = append(findings, Finding{
							ID:          r.ID,
							Title:       r.Title,
							Description: r.Description + " (" + sp.name + ": " + redact(matched) + ")",
							Severity:    r.Severity,
							Framework:   r.Framework,
							ControlIDs:  r.Controls,
							File:        file.RelativePath,
							Line:        lineIdx + 1,
							Column:      loc[0] + 1,
							Evidence:    redact(matched),
							Analyzer:    s.Name(),
							Fixable:     r.Fixable,
						})
					}
				}
			}

			// High-entropy string detection
			heMatches := highEntropyRegex.FindAllStringSubmatchIndex(line, -1)
			for _, loc := range heMatches {
				if loc[2] < 0 || loc[3] < 0 {
					continue
				}
				candidate := line[loc[2]:loc[3]]
				if len(candidate) < 20 {
					continue
				}
				threshold := 5.0
				if isHexLike(candidate) {
					threshold = 4.5
				}
				if shannonEntropy(candidate) > threshold {
					matchedRules := rulesByPattern["high-entropy"]
					for _, r := range matchedRules {
						findings = append(findings, Finding{
							ID:          r.ID,
							Title:       r.Title,
							Description: r.Description + " (High-entropy string: " + redact(candidate) + ")",
							Severity:    r.Severity,
							Framework:   r.Framework,
							ControlIDs:  r.Controls,
							File:        file.RelativePath,
							Line:        lineIdx + 1,
							Column:      loc[2] + 1,
							Evidence:    redact(candidate),
							Analyzer:    s.Name(),
							Fixable:     r.Fixable,
						})
					}
				}
			}
		}
	}

	return findings, nil
}

func shannonEntropy(s string) float64 {
	freq := make(map[rune]int)
	for _, ch := range s {
		freq[ch]++
	}
	length := float64(len([]rune(s)))
	var entropy float64
	for _, count := range freq {
		p := float64(count) / length
		entropy -= p * math.Log2(p)
	}
	return entropy
}

func redact(secret string) string {
	if len(secret) <= 8 {
		return "****"
	}
	return secret[:4] + "****" + secret[len(secret)-4:]
}

func isHexLike(s string) bool {
	for _, ch := range s {
		if !((ch >= '0' && ch <= '9') || (ch >= 'a' && ch <= 'f') || (ch >= 'A' && ch <= 'F')) {
			return false
		}
	}
	return true
}

func filterRules(allRules []rules.Rule, analyzerName string) []rules.Rule {
	var result []rules.Rule
	for _, r := range allRules {
		if r.Analyzer == analyzerName {
			result = append(result, r)
		}
	}
	return result
}

func groupRulesByPattern(rr []rules.Rule) map[string][]rules.Rule {
	m := make(map[string][]rules.Rule)
	for _, r := range rr {
		m[r.Pattern] = append(m[r.Pattern], r)
	}
	return m
}
```

**Step 5: Run tests**

Run: `cd /home/rishav/soul/products/compliance-go && go test ./analyzers/ -v`
Expected: All 4 tests pass

**Step 6: Commit**

```bash
cd /home/rishav/soul
git add products/compliance-go/analyzers/
git commit -m "feat(compliance-go): add secret scanner with 16 patterns + entropy detection"
```

---

### Task 11: Config checker analyzer (Go)

**Files:**
- Create: `products/compliance-go/analyzers/config_checker.go`
- Test: `products/compliance-go/analyzers/config_checker_test.go`

**Step 1: Write config checker tests**

Create `products/compliance-go/analyzers/config_checker_test.go`:
```go
package analyzers_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/rishav1305/soul/products/compliance-go/analyzers"
	"github.com/rishav1305/soul/products/compliance-go/rules"
)

func TestConfigCheckerFindsDockerRunAsRoot(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "Dockerfile")
	os.WriteFile(filePath, []byte("FROM node:18\nRUN npm install\nCMD [\"node\", \"app.js\"]\n"), 0o644)

	checker := &analyzers.ConfigChecker{}
	files := []analyzers.ScannedFile{
		{Path: filePath, RelativePath: "Dockerfile", Extension: "", Size: 100},
	}
	allRules := rules.Load(nil)

	findings, err := checker.Analyze(files, allRules)
	if err != nil {
		t.Fatal(err)
	}

	// Should find missing USER directive
	found := false
	for _, f := range findings {
		if strings.Contains(f.ID, "DOCKER") || strings.Contains(f.Title, "root") || strings.Contains(f.Title, "USER") {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected finding about Docker running as root")
	}
}

func TestConfigCheckerFindsCORSWildcard(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "server.ts")
	os.WriteFile(filePath, []byte(`app.use(cors({ origin: "*" }));`), 0o644)

	checker := &analyzers.ConfigChecker{}
	files := []analyzers.ScannedFile{
		{Path: filePath, RelativePath: "server.ts", Extension: "ts", Size: 100},
	}
	allRules := rules.Load(nil)

	findings, err := checker.Analyze(files, allRules)
	if err != nil {
		t.Fatal(err)
	}

	found := false
	for _, f := range findings {
		if strings.Contains(f.Title, "CORS") || strings.Contains(f.ID, "CORS") {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected CORS wildcard finding")
	}
}
```

Note: Add `"strings"` import at top of test file.

**Step 2: Run tests to verify they fail**

Run: `cd /home/rishav/soul/products/compliance-go && go test ./analyzers/ -v -run TestConfigChecker`
Expected: FAIL

**Step 3: Implement config checker**

Create `products/compliance-go/analyzers/config_checker.go` — port directly from TS `config-checker.ts`. Checks:
- `.env` file existence (should be gitignored)
- Dockerfile: missing USER directive, using latest tag
- `package.json`: missing engines field
- CORS wildcard origin in source files
- Missing HTTPS redirect patterns

The implementation follows the same pattern as secret scanner: iterate files, match patterns against config-checker rules, emit findings.

**Step 4: Run tests**

Run: `cd /home/rishav/soul/products/compliance-go && go test ./analyzers/ -v -run TestConfigChecker`
Expected: Tests pass

**Step 5: Commit**

```bash
cd /home/rishav/soul
git add products/compliance-go/analyzers/config_checker*
git commit -m "feat(compliance-go): add config checker analyzer"
```

---

### Task 12: Git analyzer (Go)

**Files:**
- Create: `products/compliance-go/analyzers/git_analyzer.go`
- Test: `products/compliance-go/analyzers/git_analyzer_test.go`

Same pattern: port from TS `git-analyzer.ts`. Checks for `.gitignore` (must exclude `.env`, `node_modules`, secrets), `CODEOWNERS`, `SECURITY.md`, `LICENSE`, CI config. Write tests first, then implement.

**Commit message:** `feat(compliance-go): add git analyzer`

---

### Task 13: Dependency auditor analyzer (Go)

**Files:**
- Create: `products/compliance-go/analyzers/dep_auditor.go`
- Test: `products/compliance-go/analyzers/dep_auditor_test.go`

Port from TS `dep-auditor.ts`. Checks for unpinned dependencies, missing lockfile, missing engines field, copyleft licenses. Write tests first, then implement.

**Commit message:** `feat(compliance-go): add dependency auditor analyzer`

---

### Task 14: AST analyzer (Go)

**Files:**
- Create: `products/compliance-go/analyzers/ast_analyzer.go`
- Test: `products/compliance-go/analyzers/ast_analyzer_test.go`

Port from TS `ast-analyzer.ts`. Uses 8 regex patterns to detect code anti-patterns: eval usage, SQL injection, weak crypto, insecure random, etc. Write tests first, then implement.

**Commit message:** `feat(compliance-go): add AST analyzer with 8 anti-patterns`

---

### Task 15: Scan orchestrator (Go)

**Files:**
- Create: `products/compliance-go/scan/orchestrator.go`
- Create: `products/compliance-go/scan/scanner.go`
- Test: `products/compliance-go/scan/orchestrator_test.go`

**Step 1: Write orchestrator tests**

Test that:
- All 5 analyzers run in parallel (goroutines)
- Findings are deduplicated on `file:line:id`
- Severity and framework filters work
- Failed analyzers are tracked (not silently dropped)
- Summary counts are correct

**Step 2: Implement**

`scanner.go` — walks directory with `filepath.WalkDir`, collects `ScannedFile` structs (skip `.git`, `node_modules`, `dist`, binary files).

`orchestrator.go` — runs all 5 analyzers in goroutines via `errgroup`, collects findings, deduplicates, filters, builds summary. Same logic as TS `scan.ts`.

**Commit message:** `feat(compliance-go): add scan orchestrator with parallel goroutines`

---

### Task 16: Fix engine and reporters (Go)

**Files:**
- Create: `products/compliance-go/fix/fix.go`
- Create: `products/compliance-go/fix/strategies.go`
- Create: `products/compliance-go/reporters/terminal.go`
- Create: `products/compliance-go/reporters/json.go`
- Create: `products/compliance-go/reporters/badge.go`
- Create: `products/compliance-go/reporters/html.go`
- Test: `products/compliance-go/fix/fix_test.go`
- Test: `products/compliance-go/reporters/badge_test.go`

Port from TS. Fix engine generates unified diffs for 4 strategies (secret→env, weak hash→sha256, eval→safe alternative, CORS wildcard→specific origin). Badge generator creates SVG with score calculation. HTML reporter generates standalone page.

**Commit message:** `feat(compliance-go): add fix engine, reporters (terminal, JSON, badge, HTML)`

---

### Task 17: Compliance gRPC server (Go)

**Files:**
- Modify: `products/compliance-go/main.go`
- Create: `products/compliance-go/service.go`
- Test: `products/compliance-go/service_test.go`

**Step 1: Write service tests**

Test via gRPC test server:
- `GetManifest` returns correct name, version, 5 tools
- `ExecuteTool` with tool="scan" returns scan results
- `ExecuteToolStream` with tool="scan" sends progress events then complete
- `Health` returns healthy=true

**Step 2: Implement service**

`service.go` — implements `ProductServiceServer` interface. Routes tool names to scan/fix/badge/report/monitor functions.

`main.go` — starts gRPC server on unix socket passed via `--socket` flag.

**Step 3: Verify end-to-end**

Build and run:
```bash
cd /home/rishav/soul/products/compliance-go
go build -o /tmp/soul-compliance .
/tmp/soul-compliance --socket /tmp/test.sock &
# Test with grpcurl or unit test
kill %1
```

**Commit message:** `feat(compliance-go): add gRPC ProductService server`

---

## Phase 4: React SPA

### Task 18: Initialize Vite + React + TailwindCSS project

**Files:**
- Create: `web/` (entire Vite project)
- Create: `web/package.json`
- Create: `web/vite.config.ts`
- Create: `web/tsconfig.json`
- Create: `web/tailwind.config.js`
- Create: `web/src/main.tsx`
- Create: `web/src/App.tsx`
- Create: `web/index.html`
- Create: `web/src/styles/globals.css`

**Step 1: Scaffold Vite project**

Run:
```bash
cd /home/rishav/soul
npm create vite@latest web -- --template react-ts
cd web
npm install
npm install -D tailwindcss @tailwindcss/vite
```

**Step 2: Configure Vite**

Update `web/vite.config.ts`:
```typescript
import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';
import tailwindcss from '@tailwindcss/vite';

export default defineConfig({
  plugins: [react(), tailwindcss()],
  server: {
    proxy: {
      '/api': 'http://localhost:3000',
      '/ws': {
        target: 'ws://localhost:3000',
        ws: true,
      },
    },
  },
  build: {
    outDir: 'dist',
  },
});
```

**Step 3: Configure Tailwind**

Update `web/src/styles/globals.css`:
```css
@import "tailwindcss";
```

**Step 4: Create minimal App**

Replace `web/src/App.tsx`:
```tsx
export default function App() {
  return (
    <div className="h-screen bg-zinc-950 text-zinc-100 flex items-center justify-center">
      <h1 className="text-2xl font-bold">◆ Soul</h1>
    </div>
  );
}
```

Update `web/src/main.tsx`:
```tsx
import { StrictMode } from 'react';
import { createRoot } from 'react-dom/client';
import './styles/globals.css';
import App from './App';

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <App />
  </StrictMode>,
);
```

**Step 5: Verify dev server**

Run: `cd /home/rishav/soul/web && npm run dev`
Expected: Vite dev server starts, page renders "◆ Soul"

**Step 6: Verify build**

Run: `cd /home/rishav/soul/web && npm run build`
Expected: `web/dist/` created with index.html + JS bundles

**Step 7: Commit**

```bash
cd /home/rishav/soul
git add web/
git commit -m "feat: initialize Vite + React + TailwindCSS SPA"
```

---

### Task 19: WebSocket client hook

**Files:**
- Create: `web/src/lib/types.ts`
- Create: `web/src/lib/ws.ts`
- Create: `web/src/hooks/useWebSocket.ts`

**Step 1: Define types**

Create `web/src/lib/types.ts`:
```typescript
export interface WSMessage {
  type: string;
  session_id?: string;
  content?: string;
  data?: unknown;
}

export interface ChatMessage {
  id: string;
  role: 'user' | 'assistant';
  content: string;
  toolCalls?: ToolCallMessage[];
  timestamp: Date;
}

export interface ToolCallMessage {
  id: string;
  name: string;
  input: unknown;
  status: 'running' | 'complete' | 'error';
  progress?: number;
  output?: string;
  findings?: FindingMessage[];
}

export interface FindingMessage {
  id: string;
  title: string;
  severity: string;
  file?: string;
  line?: number;
  evidence?: string;
}
```

**Step 2: Create WebSocket client with auto-reconnect**

Create `web/src/lib/ws.ts`:
```typescript
type MessageHandler = (msg: WSMessage) => void;

export class WSClient {
  private ws: WebSocket | null = null;
  private url: string;
  private handlers: MessageHandler[] = [];
  private reconnectDelay = 1000;
  private maxReconnectDelay = 30000;
  private shouldReconnect = true;

  constructor(url: string) {
    this.url = url;
  }

  connect() {
    this.shouldReconnect = true;
    this.ws = new WebSocket(this.url);
    this.ws.onmessage = (e) => {
      const msg = JSON.parse(e.data);
      this.handlers.forEach((h) => h(msg));
    };
    this.ws.onclose = () => {
      if (this.shouldReconnect) {
        setTimeout(() => this.connect(), this.reconnectDelay);
        this.reconnectDelay = Math.min(this.reconnectDelay * 2, this.maxReconnectDelay);
      }
    };
    this.ws.onopen = () => {
      this.reconnectDelay = 1000;
    };
  }

  send(msg: WSMessage) {
    if (this.ws?.readyState === WebSocket.OPEN) {
      this.ws.send(JSON.stringify(msg));
    }
  }

  onMessage(handler: MessageHandler) {
    this.handlers.push(handler);
    return () => {
      this.handlers = this.handlers.filter((h) => h !== handler);
    };
  }

  disconnect() {
    this.shouldReconnect = false;
    this.ws?.close();
  }
}
```

**Step 3: Create useWebSocket hook**

Create `web/src/hooks/useWebSocket.ts`:
```typescript
import { useEffect, useRef, useCallback, useState } from 'react';
import { WSClient } from '../lib/ws';
import type { WSMessage } from '../lib/types';

export function useWebSocket() {
  const clientRef = useRef<WSClient | null>(null);
  const [connected, setConnected] = useState(false);

  useEffect(() => {
    const protocol = window.location.protocol === 'https:' ? 'wss' : 'ws';
    const client = new WSClient(`${protocol}://${window.location.host}/ws`);
    clientRef.current = client;
    client.connect();
    // Track connection state via a handler that fires on any message
    client.onMessage(() => setConnected(true));
    return () => client.disconnect();
  }, []);

  const send = useCallback((msg: WSMessage) => {
    clientRef.current?.send(msg);
  }, []);

  const onMessage = useCallback((handler: (msg: WSMessage) => void) => {
    return clientRef.current?.onMessage(handler) ?? (() => {});
  }, []);

  return { send, onMessage, connected };
}
```

**Step 4: Commit**

```bash
cd /home/rishav/soul
git add web/src/lib/ web/src/hooks/
git commit -m "feat: add WebSocket client with auto-reconnect and useWebSocket hook"
```

---

### Task 20: Chat components (useChat hook + ChatView + InputBar)

**Files:**
- Create: `web/src/hooks/useChat.ts`
- Create: `web/src/components/chat/ChatView.tsx`
- Create: `web/src/components/chat/Message.tsx`
- Create: `web/src/components/chat/StreamingText.tsx`
- Create: `web/src/components/chat/ToolCall.tsx`
- Create: `web/src/components/chat/InputBar.tsx`
- Modify: `web/src/App.tsx`

**Step 1: Create useChat hook**

Manages chat state: messages array, streaming text accumulator, session ID. Listens for `chat.token`, `chat.tool_call`, `chat.done`, `tool.progress`, `tool.finding`, `tool.complete` events.

**Step 2: Create ChatView**

Scrollable message list + InputBar at bottom. Messages rendered as user bubbles (right-aligned, zinc-800) and assistant bubbles (left-aligned, zinc-900). Tool calls rendered inline as expandable blocks.

**Step 3: Create InputBar**

Text input with Enter to send. Shift+Enter for newline. Send button on right.

**Step 4: Create StreamingText**

Renders markdown text token-by-token as it arrives. Install `react-markdown`:
```bash
cd /home/rishav/soul/web && npm install react-markdown
```

**Step 5: Create Message and ToolCall components**

Message renders user/assistant bubbles with markdown. ToolCall renders expandable card showing tool name, progress bar, findings list, output.

**Step 6: Update App.tsx with layout**

```tsx
import ChatView from './components/chat/ChatView';

export default function App() {
  return (
    <div className="h-screen bg-zinc-950 text-zinc-100 flex flex-col">
      <header className="h-12 border-b border-zinc-800 flex items-center px-4">
        <span className="text-lg font-bold">◆ Soul</span>
      </header>
      <main className="flex-1 flex overflow-hidden">
        <ChatView />
      </main>
    </div>
  );
}
```

**Step 7: Verify in browser**

Run: `cd /home/rishav/soul/web && npm run dev`
Expected: Chat UI renders, can type messages, sends via WebSocket

**Step 8: Commit**

```bash
cd /home/rishav/soul
git add web/src/
git commit -m "feat: add chat UI with streaming messages and tool call rendering"
```

---

### Task 21: Side panel components (CompliancePanel + FindingsTable)

**Files:**
- Create: `web/src/components/panels/PanelContainer.tsx`
- Create: `web/src/components/panels/CompliancePanel.tsx`
- Create: `web/src/components/panels/FindingsTable.tsx`
- Create: `web/src/components/panels/ScanProgress.tsx`
- Create: `web/src/hooks/useScanResult.ts`
- Modify: `web/src/App.tsx`

**Step 1: Create PanelContainer**

Resizable right sidebar. Uses CSS resize or a drag handle. Default width: 400px. Collapsible.

**Step 2: Create CompliancePanel**

Shows: compliance score (percentage + color), summary counts by severity, "Fix All" and "Export" buttons. Updates live via WebSocket events.

**Step 3: Create FindingsTable**

Sortable and filterable table of findings. Columns: severity icon, ID, title, file:line, fixable. Click to expand details.

**Step 4: Create ScanProgress**

Shows real-time analyzer progress bars (one per analyzer). Updates from `tool.progress` WebSocket events.

**Step 5: Create useScanResult hook**

Manages scan state from WebSocket events. Collects findings as they arrive from `tool.finding`, builds running summary.

**Step 6: Update App.tsx layout**

```tsx
<main className="flex-1 flex overflow-hidden">
  <div className="flex-1">
    <ChatView />
  </div>
  <PanelContainer>
    <CompliancePanel />
  </PanelContainer>
</main>
```

**Step 7: Commit**

```bash
cd /home/rishav/soul
git add web/src/
git commit -m "feat: add compliance side panel with findings table and live progress"
```

---

## Phase 5: Integration

### Task 22: Wire Go core to compliance product

**Files:**
- Modify: `cmd/soul/main.go`
- Modify: `internal/server/server.go`
- Modify: `internal/server/ws.go`

**Step 1: Update main.go serve command**

Wire up product manager: discover compliance binary (embedded or from `~/.soul/products/`), start it, connect gRPC, register tools.

**Step 2: Update WebSocket handler**

When `chat.send` received:
1. Build Claude tools from registry
2. Call Claude Messages API with streaming
3. Parse SSE events → forward `chat.token` to browser
4. On `tool_use` → route to product via gRPC proxy
5. On streaming tool events → forward `tool.progress` and `tool.finding` to browser
6. Feed `tool_result` back to Claude
7. Continue until Claude finishes → send `chat.done`

**Step 3: Add `tool.execute` handler**

For direct tool execution from UI buttons (Fix All, Export), bypass Claude and route directly to product gRPC.

**Step 4: Test end-to-end**

Build full binary, start `soul serve`, open browser, type "scan my project", verify:
- AI responds with tool_use for compliance__scan
- Side panel shows scan progress
- Findings appear in real-time
- Final response summarizes results

**Step 5: Commit**

```bash
git add cmd/ internal/
git commit -m "feat: wire AI agent loop with product gRPC proxy and WebSocket streaming"
```

---

### Task 23: Embed React SPA in Go binary

**Files:**
- Modify: `internal/server/spa.go`
- Modify: `Makefile`

**Step 1: Update spa.go to embed from web/dist**

Change the `//go:embed` directive to point to `web/dist` (built by Vite):

```go
//go:embed all:../../web/dist
var staticFiles embed.FS
```

Note: go:embed paths are relative to the Go source file. If this is in `internal/server/`, the path needs adjustment. Alternative: use a top-level `embed.go` in the project root.

Better approach — create `web.go` at project root:
```go
package soul

import "embed"

//go:embed web/dist
var WebDist embed.FS
```

Then reference it from server package.

**Step 2: Update Makefile build target**

Ensure `make build` runs `npm run build` in `web/` first, then `go build`.

**Step 3: Verify single binary**

```bash
make build
ls -lh dist/soul
dist/soul serve --port 8888 &
curl http://localhost:8888/
kill %1
```

Expected: HTML from React SPA returned, binary is ~20-25MB

**Step 4: Commit**

```bash
git add internal/server/spa.go web.go Makefile
git commit -m "feat: embed React SPA in Go binary via go:embed"
```

---

### Task 24: Dev mode with hot reload

**Files:**
- Modify: `internal/server/spa.go`
- Modify: `cmd/soul/main.go`

**Step 1: Add dev mode proxy**

When `--dev` flag is set, instead of serving embedded files, proxy all non-API requests to Vite dev server at `localhost:5173`:

```go
func devProxyHandler(viteAddr string) http.Handler {
	target, _ := url.Parse(viteAddr)
	proxy := httputil.NewSingleHostReverseProxy(target)
	return proxy
}
```

**Step 2: Update route registration**

In dev mode, use `devProxyHandler` instead of `spaHandler()`.

**Step 3: Verify**

Terminal 1: `cd web && npm run dev`
Terminal 2: `go run ./cmd/soul serve --dev`
Open `http://localhost:3000` — should render React SPA from Vite with hot reload.

**Step 4: Commit**

```bash
git add internal/ cmd/
git commit -m "feat: add dev mode with Vite proxy for hot reload"
```

---

## Phase 6: Testing & Polish

### Task 25: Go core tests

**Files:**
- Test: `internal/server/server_test.go` (expand)
- Test: `internal/ai/client_test.go` (expand)
- Test: `internal/session/session_test.go`

Write comprehensive tests for:
- Server: all API endpoints, CORS middleware, SPA fallback
- Session: create, add messages, concurrent access
- AI tools: tool conversion edge cases

Run: `go test ./internal/... -v`

**Commit message:** `test: expand Go core test coverage`

---

### Task 26: Compliance Go product tests

**Files:**
- Test: `products/compliance-go/scan/orchestrator_test.go` (expand)
- Test: `products/compliance-go/service_test.go` (expand)

Use same fixture-based approach as TS: `testdata/vulnerable-app/` and `testdata/compliant-app/`. Port exact assertions from TS tests.

Run: `cd products/compliance-go && go test ./... -v`

**Commit message:** `test: add comprehensive compliance-go fixture-based tests`

---

### Task 27: React SPA tests

**Files:**
- Create: `web/src/components/chat/__tests__/ChatView.test.tsx`
- Create: `web/src/hooks/__tests__/useChat.test.ts`
- Create: `web/src/lib/__tests__/ws.test.ts`

Install Vitest + React Testing Library:
```bash
cd web && npm install -D vitest @testing-library/react @testing-library/jest-dom jsdom
```

Test: mock WebSocket, verify chat message rendering, verify tool call rendering, verify input behavior.

Run: `cd web && npx vitest run`

**Commit message:** `test: add React SPA component and hook tests`

---

### Task 28: Integration test (full pipeline)

**Files:**
- Create: `tests/integration_test.go`

Build full binary, start `soul serve`, connect WebSocket client, send chat message, assert streaming events received. If Claude API key available, do full AI roundtrip. If not, test WebSocket protocol with mock responses.

Run: `go test ./tests/ -v -tags integration`

**Commit message:** `test: add full-pipeline integration test`

---

### Task 29: Final build and verification

**Step 1: Run all tests**

```bash
go test ./... -v
cd products/compliance-go && go test ./... -v
cd web && npm test
```

**Step 2: Build single binary**

```bash
make build
ls -lh dist/soul
```

**Step 3: Smoke test**

```bash
dist/soul --version
dist/soul serve &
# Open browser to http://localhost:3000
# Verify: SPA loads, chat works, compliance panel visible
kill %1
```

**Step 4: Tag release**

```bash
git tag v0.2.0-alpha
```

**Step 5: Final commit**

```bash
git commit --allow-empty -m "chore: Soul v0.2.0-alpha — Go core + React SPA + compliance product"
```

---

## Task Dependency Graph

```
Phase 1: Environment
  Task 1 (Go) → Task 2 (protoc) → Task 3 (scaffold) → Task 4 (proto)

Phase 2: Go Core (depends on Task 4)
  Task 5 (HTTP) → Task 6 (WebSocket) → Task 7 (products) → Task 8 (AI client)

Phase 3: Compliance Go (depends on Task 4)
  Task 9 (rules) → Task 10 (secrets) → Task 11 (config) → Task 12 (git) → Task 13 (deps) → Task 14 (AST) → Task 15 (orchestrator) → Task 16 (fix+reporters) → Task 17 (gRPC server)

Phase 4: React SPA (independent of Go, needs Node.js)
  Task 18 (Vite scaffold) → Task 19 (WS client) → Task 20 (chat UI) → Task 21 (panels)

Phase 5: Integration (depends on Phase 2 + 3 + 4)
  Task 22 (wire core↔product) → Task 23 (embed SPA) → Task 24 (dev mode)

Phase 6: Testing (depends on Phase 5)
  Task 25 (core tests) ∥ Task 26 (compliance tests) ∥ Task 27 (SPA tests) → Task 28 (integration) → Task 29 (final)
```

**Parallelizable:** Phase 2 and Phase 3 can run in parallel after Task 4. Phase 4 can start after Task 18 independently. Tasks 25, 26, 27 can run in parallel.
