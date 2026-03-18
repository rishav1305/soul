# Soul MCP Server Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a standalone MCP server (`soul-mcp` on :3028) that exposes 90 Soul v2 product tools to Claude.ai via the MCP Streamable HTTP transport with OAuth 2.1 authentication.

**Architecture:** Thin protocol translation layer — reuses existing `internal/chat/context/` for tool definitions and `Dispatcher` for routing tool calls to product REST APIs. OAuth 2.1 for Claude.ai authentication. JSON-RPC 2.0 over HTTP.

**Tech Stack:** Go 1.24, net/http, crypto/hmac (JWT), standard library only

**Spec:** `docs/superpowers/specs/2026-03-18-soul-mcp-server-design.md`

---

## File Map

| File | Action | Responsibility |
|------|--------|---------------|
| `internal/mcp/protocol/protocol.go` | Create | JSON-RPC 2.0 types, MCP request/response structs |
| `internal/mcp/protocol/protocol_test.go` | Create | Protocol parsing tests |
| `internal/mcp/auth/auth.go` | Create | OAuth 2.1 endpoints, JWT management, client registration |
| `internal/mcp/auth/auth_test.go` | Create | OAuth flow tests |
| `internal/mcp/tools/registry.go` | Create | Collect tools from context package, handle list/call |
| `internal/mcp/tools/registry_test.go` | Create | Tool collection + dispatch tests |
| `internal/mcp/server/server.go` | Create | HTTP server, MCP endpoint, middleware chain |
| `cmd/mcp/main.go` | Create | Entrypoint (:3028) |
| `Makefile` | Modify | Add build-mcp target |
| `deploy/soul-v2-mcp.service` | Create | SystemD service |

---

## Task 1: MCP Protocol Types

**Files:**
- Create: `internal/mcp/protocol/protocol.go`
- Create: `internal/mcp/protocol/protocol_test.go`

- [ ] **Step 1: Write failing tests for protocol parsing**

Create `internal/mcp/protocol/protocol_test.go`:

```go
package protocol

import (
	"encoding/json"
	"testing"
)

func TestParseRequest_ToolsCall(t *testing.T) {
	raw := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"list_tasks","arguments":{"status":"active"}}}`
	req, err := ParseRequest([]byte(raw))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if req.Method != "tools/call" {
		t.Errorf("method = %q, want tools/call", req.Method)
	}
	if req.ID == nil {
		t.Error("expected non-nil ID")
	}
}

func TestParseRequest_Notification(t *testing.T) {
	raw := `{"jsonrpc":"2.0","method":"notifications/initialized"}`
	req, err := ParseRequest([]byte(raw))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if !req.IsNotification() {
		t.Error("expected notification (no id)")
	}
}

func TestNewResult(t *testing.T) {
	resp := NewResult(json.RawMessage(`1`), map[string]string{"status": "ok"})
	data, _ := json.Marshal(resp)
	if string(data) == "" {
		t.Error("expected non-empty response")
	}
}

func TestNewError(t *testing.T) {
	resp := NewError(json.RawMessage(`1`), -32601, "Method not found")
	data, _ := json.Marshal(resp)
	s := string(data)
	if s == "" {
		t.Error("expected non-empty error response")
	}
}

func TestParseToolCallParams(t *testing.T) {
	params := json.RawMessage(`{"name":"list_tasks","arguments":{"status":"active"}}`)
	name, args, err := ParseToolCallParams(params)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if name != "list_tasks" {
		t.Errorf("name = %q, want list_tasks", name)
	}
	if len(args) == 0 {
		t.Error("expected non-empty arguments")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /home/rishav/soul-v2 && go test ./internal/mcp/protocol/ -v`
Expected: FAIL

- [ ] **Step 3: Implement protocol.go**

Create `internal/mcp/protocol/protocol.go`:

```go
package protocol

import (
	"encoding/json"
	"fmt"
)

// JSON-RPC 2.0 request
type Request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"` // nil for notifications
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

func (r *Request) IsNotification() bool {
	return r.ID == nil || len(r.ID) == 0
}

// JSON-RPC 2.0 response
type Response struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Result  interface{}     `json:"result,omitempty"`
	Error   *RPCError       `json:"error,omitempty"`
}

type RPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// MCP-specific types
type InitializeResult struct {
	ProtocolVersion string       `json:"protocolVersion"`
	ServerInfo      ServerInfo   `json:"serverInfo"`
	Capabilities    Capabilities `json:"capabilities"`
}

type ServerInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type Capabilities struct {
	Tools *ToolsCapability `json:"tools,omitempty"`
}

type ToolsCapability struct{}

type MCPTool struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	InputSchema json.RawMessage `json:"inputSchema"`
}

type ToolsListResult struct {
	Tools []MCPTool `json:"tools"`
}

type ToolCallResult struct {
	Content []ToolContent `json:"content"`
	IsError bool          `json:"isError,omitempty"`
}

type ToolContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

func ParseRequest(data []byte) (*Request, error) {
	var req Request
	if err := json.Unmarshal(data, &req); err != nil {
		return nil, fmt.Errorf("parse request: %w", err)
	}
	if req.JSONRPC != "2.0" {
		return nil, fmt.Errorf("unsupported jsonrpc version: %q", req.JSONRPC)
	}
	if req.Method == "" {
		return nil, fmt.Errorf("missing method")
	}
	return &req, nil
}

func ParseToolCallParams(params json.RawMessage) (name string, args json.RawMessage, err error) {
	var p struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
	}
	if err := json.Unmarshal(params, &p); err != nil {
		return "", nil, fmt.Errorf("parse tool call params: %w", err)
	}
	if p.Name == "" {
		return "", nil, fmt.Errorf("missing tool name")
	}
	if p.Arguments == nil {
		p.Arguments = json.RawMessage(`{}`)
	}
	return p.Name, p.Arguments, nil
}

func NewResult(id json.RawMessage, result interface{}) *Response {
	return &Response{JSONRPC: "2.0", ID: id, Result: result}
}

func NewError(id json.RawMessage, code int, message string) *Response {
	return &Response{JSONRPC: "2.0", ID: id, Error: &RPCError{Code: code, Message: message}}
}

// Standard JSON-RPC error codes
const (
	ParseError     = -32700
	InvalidRequest = -32600
	MethodNotFound = -32601
	InvalidParams  = -32602
	InternalError  = -32603
)
```

- [ ] **Step 4: Run tests → ALL PASS**

- [ ] **Step 5: Commit**

```bash
git add internal/mcp/protocol/
git commit -m "feat: add MCP JSON-RPC 2.0 protocol types"
```

---

## Task 2: OAuth 2.1 Authentication

**Files:**
- Create: `internal/mcp/auth/auth.go`
- Create: `internal/mcp/auth/auth_test.go`

- [ ] **Step 1: Write failing tests**

Create `internal/mcp/auth/auth_test.go`:

```go
package auth

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestCreateAndVerifyToken(t *testing.T) {
	secret := "test-secret-key"
	token, err := CreateAccessToken("soul-admin", secret)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	sub, err := VerifyToken(token, secret)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if sub != "soul-admin" {
		t.Errorf("sub = %q, want soul-admin", sub)
	}
}

func TestVerifyToken_Invalid(t *testing.T) {
	_, err := VerifyToken("garbage", "secret")
	if err == nil {
		t.Error("expected error for invalid token")
	}
}

func TestVerifyToken_WrongSecret(t *testing.T) {
	token, _ := CreateAccessToken("admin", "secret1")
	_, err := VerifyToken(token, "secret2")
	if err == nil {
		t.Error("expected error for wrong secret")
	}
}

func TestAuthMiddleware_ValidToken(t *testing.T) {
	secret := "test-secret"
	token, _ := CreateAccessToken("admin", secret)

	handler := AuthMiddleware(secret, []string{})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))

	req := httptest.NewRequest("POST", "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != 200 {
		t.Errorf("expected 200, got %d", rec.Code)
	}
}

func TestAuthMiddleware_MissingToken(t *testing.T) {
	handler := AuthMiddleware("secret", []string{})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))

	req := httptest.NewRequest("POST", "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != 401 {
		t.Errorf("expected 401, got %d", rec.Code)
	}
}

func TestTokenEndpoint(t *testing.T) {
	h := NewOAuthHandler("secret", "admin-pass")
	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("code", "")  // will test with real code flow

	req := httptest.NewRequest("POST", "/token", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	h.HandleToken(rec, req)

	// Without valid code, should fail
	if rec.Code == 200 {
		t.Error("expected error without valid auth code")
	}
}

func TestProtectedResourceMetadata(t *testing.T) {
	h := NewOAuthHandler("secret", "pass")
	req := httptest.NewRequest("GET", "/.well-known/oauth-protected-resource", nil)
	rec := httptest.NewRecorder()
	h.HandleProtectedResource(rec, req)

	if rec.Code != 200 {
		t.Errorf("expected 200, got %d", rec.Code)
	}
}

func TestOriginValidation(t *testing.T) {
	handler := OriginMiddleware([]string{"https://claude.ai"})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))

	// Valid origin
	req := httptest.NewRequest("POST", "/", nil)
	req.Header.Set("Origin", "https://claude.ai")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Errorf("valid origin: expected 200, got %d", rec.Code)
	}

	// No origin (server-to-server) — allowed
	req2 := httptest.NewRequest("POST", "/", nil)
	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, req2)
	if rec2.Code != 200 {
		t.Errorf("no origin: expected 200, got %d", rec2.Code)
	}

	// Bad origin
	req3 := httptest.NewRequest("POST", "/", nil)
	req3.Header.Set("Origin", "https://evil.com")
	rec3 := httptest.NewRecorder()
	handler.ServeHTTP(rec3, req3)
	if rec3.Code != 403 {
		t.Errorf("bad origin: expected 403, got %d", rec3.Code)
	}
}
```

- [ ] **Step 2: Implement auth.go**

Create `internal/mcp/auth/auth.go` with:

- `CreateAccessToken(sub, secret string) (string, error)` — HS256 JWT, 1h expiry
- `CreateRefreshToken(sub, secret string) (string, error)` — HS256 JWT, 30d expiry
- `VerifyToken(tokenStr, secret string) (string, error)` — returns subject
- `AuthMiddleware(secret string, skipPaths []string) func(http.Handler) http.Handler` — validates Bearer token, skips for listed paths
- `OriginMiddleware(allowedOrigins []string) func(http.Handler) http.Handler` — validates Origin header
- `OAuthHandler` struct with methods:
  - `HandleProtectedResource` — `GET /.well-known/oauth-protected-resource`
  - `HandleAuthorizationServer` — `GET /.well-known/oauth-authorization-server`
  - `HandleAuthorize` — `GET /authorize` (shows form or auto-approves)
  - `HandleToken` — `POST /token` (exchanges code for tokens, handles refresh)
  - `HandleRegister` — `POST /register` (Dynamic Client Registration)

JWT implementation uses `crypto/hmac` + `crypto/sha256` + `encoding/base64` — no external dependency.

Auth codes stored in-memory map with 5-minute expiry. Single-user mode: authorize endpoint verifies `SOUL_MCP_ADMIN_PASSWORD`.

- [ ] **Step 3: Run tests → ALL PASS**

- [ ] **Step 4: Commit**

```bash
git add internal/mcp/auth/
git commit -m "feat: add OAuth 2.1 auth for MCP server"
```

---

## Task 3: Tool Registry

**Files:**
- Create: `internal/mcp/tools/registry.go`
- Create: `internal/mcp/tools/registry_test.go`

- [ ] **Step 1: Write failing tests**

Create `internal/mcp/tools/registry_test.go`:

```go
package tools

import (
	"context"
	"encoding/json"
	"testing"
)

func TestNewRegistry_CollectsTools(t *testing.T) {
	reg := NewRegistry(nil) // nil dispatcher = list-only mode
	tools := reg.List()
	if len(tools) == 0 {
		t.Fatal("expected tools to be collected")
	}
	// Should have exactly 90 product tools (no built-ins)
	if len(tools) != 90 {
		t.Errorf("expected 90 tools, got %d", len(tools))
	}
}

func TestNewRegistry_NoDuplicates(t *testing.T) {
	reg := NewRegistry(nil)
	tools := reg.List()
	seen := make(map[string]bool)
	for _, tool := range tools {
		if seen[tool.Name] {
			t.Errorf("duplicate tool: %s", tool.Name)
		}
		seen[tool.Name] = true
	}
}

func TestNewRegistry_ExcludesBuiltins(t *testing.T) {
	reg := NewRegistry(nil)
	tools := reg.List()
	builtins := []string{"memory_store", "memory_search", "memory_list", "memory_delete", "tool_create", "tool_list", "tool_delete", "subagent"}
	for _, tool := range tools {
		for _, b := range builtins {
			if tool.Name == b {
				t.Errorf("built-in tool %q should not be in MCP registry", b)
			}
		}
	}
}

func TestRegistry_HasExpectedTools(t *testing.T) {
	reg := NewRegistry(nil)
	expected := []string{"list_tasks", "create_task", "tutor_dashboard", "compliance__scan", "lead_add", "challenge_list", "run_benchmark", "cluster_status"}
	for _, name := range expected {
		if !reg.Has(name) {
			t.Errorf("expected tool %q in registry", name)
		}
	}
}

func TestRegistry_Call_UnknownTool(t *testing.T) {
	reg := NewRegistry(nil)
	_, err := reg.Call(context.Background(), "nonexistent_tool", json.RawMessage(`{}`))
	if err == nil {
		t.Error("expected error for unknown tool")
	}
}
```

- [ ] **Step 2: Implement registry.go**

Create `internal/mcp/tools/registry.go`:

```go
package tools

import (
	"context"
	"encoding/json"
	"fmt"

	prodctx "github.com/rishav1305/soul-v2/internal/chat/context"
	"github.com/rishav1305/soul-v2/internal/mcp/protocol"
)

type Registry struct {
	tools      []protocol.MCPTool
	toolSet    map[string]bool
	dispatcher *prodctx.Dispatcher
}

func NewRegistry(dispatcher *prodctx.Dispatcher) *Registry {
	r := &Registry{
		toolSet:    make(map[string]bool),
		dispatcher: dispatcher,
	}
	r.collect()
	return r
}

func (r *Registry) collect() {
	builtinCount := len(prodctx.Default().Tools)

	canonical := []string{
		"tasks", "tutor", "projects", "observe",
		"devops", "compliance", "dataeng", "docs",
		"sentinel", "bench", "mesh", "scout",
	}

	for _, product := range canonical {
		ctx := prodctx.ForProduct(product)
		// Skip built-in tools (prepended by ForProduct)
		productTools := ctx.Tools
		if len(productTools) > builtinCount {
			productTools = productTools[builtinCount:]
		}
		for _, t := range productTools {
			if r.toolSet[t.Name] {
				continue // dedup
			}
			r.toolSet[t.Name] = true
			r.tools = append(r.tools, protocol.MCPTool{
				Name:        t.Name,
				Description: t.Description,
				InputSchema: t.InputSchema,
			})
		}
	}
}

func (r *Registry) List() []protocol.MCPTool {
	return r.tools
}

func (r *Registry) Has(name string) bool {
	return r.toolSet[name]
}

func (r *Registry) Call(ctx context.Context, name string, args json.RawMessage) (string, error) {
	if !r.toolSet[name] {
		return "", fmt.Errorf("unknown tool: %s", name)
	}
	if r.dispatcher == nil {
		return "", fmt.Errorf("dispatcher not configured")
	}
	return r.dispatcher.Execute(ctx, name, args)
}
```

- [ ] **Step 3: Run tests → ALL PASS**

- [ ] **Step 4: Commit**

```bash
git add internal/mcp/tools/
git commit -m "feat: add MCP tool registry collecting 90 product tools"
```

---

## Task 4: MCP Server + HTTP Endpoint

**Files:**
- Create: `internal/mcp/server/server.go`
- Create: `cmd/mcp/main.go`

- [ ] **Step 1: Implement server.go**

Create `internal/mcp/server/server.go`:

```go
package server

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"net"
	"net/http"
	"runtime/debug"
	"strconv"
	"time"

	"github.com/rishav1305/soul-v2/internal/mcp/auth"
	"github.com/rishav1305/soul-v2/internal/mcp/protocol"
	"github.com/rishav1305/soul-v2/internal/mcp/tools"
)

type Server struct {
	mux        *http.ServeMux
	httpServer *http.Server
	registry   *tools.Registry
	oauth      *auth.OAuthHandler
	host       string
	port       int
	secret     string
	baseURL    string
	startTime  time.Time
}

type Option func(*Server)

func WithHost(h string) Option    { return func(s *Server) { s.host = h } }
func WithPort(p int) Option       { return func(s *Server) { s.port = p } }
func WithSecret(s string) Option  { return func(srv *Server) { srv.secret = s } }
func WithBaseURL(u string) Option { return func(s *Server) { s.baseURL = u } }
func WithRegistry(r *tools.Registry) Option { return func(s *Server) { s.registry = r } }
func WithOAuth(o *auth.OAuthHandler) Option { return func(s *Server) { s.oauth = o } }

func New(opts ...Option) *Server {
	s := &Server{
		mux:       http.NewServeMux(),
		host:      "127.0.0.1",
		port:      3028,
		startTime: time.Now(),
	}
	for _, opt := range opts {
		opt(s)
	}

	// OAuth discovery endpoints (no auth required)
	if s.oauth != nil {
		s.mux.HandleFunc("GET /.well-known/oauth-protected-resource", s.oauth.HandleProtectedResource)
		s.mux.HandleFunc("GET /.well-known/oauth-authorization-server", s.oauth.HandleAuthorizationServer)
		s.mux.HandleFunc("GET /authorize", s.oauth.HandleAuthorize)
		s.mux.HandleFunc("POST /authorize", s.oauth.HandleAuthorize)
		s.mux.HandleFunc("POST /token", s.oauth.HandleToken)
		s.mux.HandleFunc("POST /register", s.oauth.HandleRegister)
	}

	// Health (no auth)
	s.mux.HandleFunc("GET /health", s.handleHealth)

	// MCP endpoint (auth required)
	s.mux.HandleFunc("POST /", s.handleMCP)
	s.mux.HandleFunc("GET /", s.handleMCPGet)

	// Middleware chain
	handler := http.Handler(s.mux)
	if s.secret != "" {
		skipPaths := []string{
			"/.well-known/", "/authorize", "/token", "/register", "/health",
		}
		handler = auth.AuthMiddleware(s.secret, skipPaths)(handler)
	}
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

func (s *Server) Start() error {
	log.Printf("soul-mcp listening on http://%s (%d tools)", s.httpServer.Addr, len(s.registry.List()))
	return s.httpServer.ListenAndServe()
}

func (s *Server) Shutdown(ctx context.Context) error {
	return s.httpServer.Shutdown(ctx)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, map[string]interface{}{
		"status": "ok",
		"uptime": time.Since(s.startTime).Round(time.Second).String(),
		"tools":  len(s.registry.List()),
	})
}

func (s *Server) handleMCPGet(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Allow", "POST")
	http.Error(w, "Method Not Allowed — use POST for MCP requests", http.StatusMethodNotAllowed)
}

func (s *Server) handleMCP(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeJSON(w, 400, protocol.NewError(nil, protocol.ParseError, "failed to read body"))
		return
	}

	req, err := protocol.ParseRequest(body)
	if err != nil {
		writeJSON(w, 400, protocol.NewError(nil, protocol.ParseError, err.Error()))
		return
	}

	// Notifications return 202 with no body
	if req.IsNotification() {
		w.WriteHeader(http.StatusAccepted)
		return
	}

	var resp *protocol.Response

	switch req.Method {
	case "initialize":
		resp = protocol.NewResult(req.ID, protocol.InitializeResult{
			ProtocolVersion: "2025-03-26",
			ServerInfo:      protocol.ServerInfo{Name: "soul-v2", Version: "1.0.0"},
			Capabilities:    protocol.Capabilities{Tools: &protocol.ToolsCapability{}},
		})

	case "ping":
		resp = protocol.NewResult(req.ID, map[string]interface{}{})

	case "tools/list":
		resp = protocol.NewResult(req.ID, protocol.ToolsListResult{Tools: s.registry.List()})

	case "tools/call":
		name, args, err := protocol.ParseToolCallParams(req.Params)
		if err != nil {
			resp = protocol.NewError(req.ID, protocol.InvalidParams, err.Error())
			break
		}

		ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
		defer cancel()

		start := time.Now()
		result, execErr := s.registry.Call(ctx, name, args)
		duration := time.Since(start)

		if execErr != nil {
			log.Printf("tool %s failed (%v): %v", name, duration, execErr)
			resp = protocol.NewResult(req.ID, protocol.ToolCallResult{
				Content: []protocol.ToolContent{{Type: "text", Text: fmt.Sprintf("Error: %v", execErr)}},
				IsError: true,
			})
		} else {
			log.Printf("tool %s ok (%v)", name, duration)
			resp = protocol.NewResult(req.ID, protocol.ToolCallResult{
				Content: []protocol.ToolContent{{Type: "text", Text: result}},
			})
		}

	default:
		resp = protocol.NewError(req.ID, protocol.MethodNotFound, fmt.Sprintf("unknown method: %s", req.Method))
	}

	writeJSON(w, 200, resp)
}
```

Add fmt import and standard middleware functions (recoveryMiddleware, bodyLimitMiddleware, rateLimitMiddleware, writeJSON) — follow the existing patterns from other servers.

- [ ] **Step 2: Create cmd/mcp/main.go**

```go
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strconv"
	"syscall"

	prodctx "github.com/rishav1305/soul-v2/internal/chat/context"
	"github.com/rishav1305/soul-v2/internal/mcp/auth"
	"github.com/rishav1305/soul-v2/internal/mcp/server"
	"github.com/rishav1305/soul-v2/internal/mcp/tools"
)

func main() {
	if len(os.Args) < 2 || os.Args[1] != "serve" {
		fmt.Println("usage: soul-mcp serve")
		os.Exit(1)
	}

	secret := os.Getenv("SOUL_MCP_SECRET")
	if secret == "" {
		log.Fatal("SOUL_MCP_SECRET is required")
	}
	adminPass := os.Getenv("SOUL_MCP_ADMIN_PASSWORD")
	if adminPass == "" {
		log.Fatal("SOUL_MCP_ADMIN_PASSWORD is required")
	}

	host := envOr("SOUL_MCP_HOST", "127.0.0.1")
	port := 3028
	if p := os.Getenv("SOUL_MCP_PORT"); p != "" {
		if v, err := strconv.Atoi(p); err == nil {
			port = v
		}
	}
	baseURL := envOr("SOUL_MCP_BASE_URL", fmt.Sprintf("https://soul.rishavchatterjee.com/mcp"))

	dispatcher := prodctx.NewDispatcher()
	registry := tools.NewRegistry(dispatcher)
	oauth := auth.NewOAuthHandler(secret, adminPass)
	oauth.SetBaseURL(baseURL)

	srv := server.New(
		server.WithHost(host),
		server.WithPort(port),
		server.WithSecret(secret),
		server.WithBaseURL(baseURL),
		server.WithRegistry(registry),
		server.WithOAuth(oauth),
	)

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		srv.Shutdown(context.Background())
	}()

	if err := srv.Start(); err != nil && err.Error() != "http: Server closed" {
		log.Fatalf("server: %v", err)
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
```

- [ ] **Step 3: Verify compilation**

Run: `cd /home/rishav/soul-v2 && go build ./cmd/mcp/`
Expected: SUCCESS

- [ ] **Step 4: Commit**

```bash
git add internal/mcp/server/ cmd/mcp/
git commit -m "feat: add soul-mcp server with MCP Streamable HTTP endpoint"
```

---

## Task 5: Build, Deploy & Verify

**Files:**
- Modify: `Makefile`
- Create: `deploy/soul-v2-mcp.service`

- [ ] **Step 1: Add Makefile target**

Add `build-mcp: go build -o soul-mcp ./cmd/mcp` and update build/serve/clean.

- [ ] **Step 2: Create systemd service**

Follow existing pattern. Environment: SOUL_MCP_HOST, SOUL_MCP_PORT, SOUL_MCP_SECRET, SOUL_MCP_ADMIN_PASSWORD.

- [ ] **Step 3: Build and verify**

Run: `make build` — all 14 binaries

- [ ] **Step 4: Run tests**

```bash
go test ./internal/mcp/... -v
```

- [ ] **Step 5: Smoke test**

```bash
SOUL_MCP_SECRET=test SOUL_MCP_ADMIN_PASSWORD=test ./soul-mcp serve &
sleep 1

# Health (no auth)
curl -s http://127.0.0.1:3028/health

# OAuth discovery
curl -s http://127.0.0.1:3028/.well-known/oauth-protected-resource

# MCP initialize (with token)
TOKEN=$(curl -s -X POST http://127.0.0.1:3028/token -d 'grant_type=...' | jq -r .access_token)
curl -s -X POST http://127.0.0.1:3028/ -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" -d '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-03-26","capabilities":{}}}'

# Tools list
curl -s -X POST http://127.0.0.1:3028/ -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" -d '{"jsonrpc":"2.0","id":2,"method":"tools/list"}'

kill %1
```

- [ ] **Step 6: Commit**

```bash
git add Makefile deploy/soul-v2-mcp.service
git commit -m "feat: add build target and systemd service for soul-mcp"
```

---

## Summary

| Task | What | Files | Tests |
|------|------|-------|-------|
| 1 | MCP Protocol types | 2 | 5 |
| 2 | OAuth 2.1 auth | 2 | 7 |
| 3 | Tool registry | 2 | 5 |
| 4 | MCP server + main | 2 | 0 (smoke) |
| 5 | Build & deploy | 2 | 0 (smoke) |
| **Total** | | **~10 files** | **~17 tests** |
