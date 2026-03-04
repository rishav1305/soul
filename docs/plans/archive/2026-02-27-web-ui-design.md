# Soul Web UI — Design Document

**Date:** 2026-02-27
**Status:** Approved

## Overview

Soul becomes a polyglot microkernel platform. A Go core binary serves a React SPA, manages an AI agent loop with Claude, and routes tool calls to product binaries via gRPC. Each product is a standalone binary in whatever language suits it, communicating over a language-agnostic gRPC interface.

User experience: download one binary, run `soul serve`, open browser.

## Architecture

```
┌─────────────────────────────────────────────────────┐
│                    Browser (React SPA)               │
│  ┌──────────────────────┬────────────────────────┐  │
│  │     AI Chat          │   Product Panels       │  │
│  │  (WebSocket stream)  │   (live updates)       │  │
│  └──────────────────────┴────────────────────────┘  │
└──────────────────────┬──────────────────────────────┘
                       │ HTTP + WebSocket
┌──────────────────────▼──────────────────────────────┐
│              Soul Core (Go binary)                   │
│                                                      │
│  ┌─────────┐ ┌──────────┐ ┌────────┐ ┌──────────┐  │
│  │ HTTP/WS │ │ Claude   │ │ Product│ │ Embedded │  │
│  │ Server  │ │ AI Client│ │ Manager│ │ React UI │  │
│  └─────────┘ └──────────┘ └───┬────┘ └──────────┘  │
└────────────────────────────────┼─────────────────────┘
                                 │ gRPC
           ┌─────────────────────┼─────────────────┐
           │                     │                  │
    ┌──────▼──────┐    ┌────────▼────────┐   ┌────▼─────┐
    │ compliance  │    │ future-product  │   │ another  │
    │ (Go)        │    │ (Python/Rust/?) │   │ (any)    │
    └─────────────┘    └─────────────────┘   └──────────┘
```

## 1. Project Structure

```
soul/
├── cmd/soul/              # Go binary entry point
│   └── main.go
├── internal/              # Go packages
│   ├── server/            # HTTP + WebSocket server
│   ├── ai/                # Claude API client + streaming
│   ├── products/          # Product lifecycle management
│   ├── session/           # Chat session state
│   ├── tier/              # Tier gating
│   └── config/            # CLI flags, env vars
├── web/                   # React SPA (Vite)
│   ├── src/
│   │   ├── components/    # Chat, panels, common UI
│   │   ├── hooks/         # useChat, useWebSocket, useProduct
│   │   ├── lib/           # API client, WebSocket client, types
│   │   └── styles/
│   ├── package.json
│   └── vite.config.ts
├── products/
│   └── compliance-go/     # Compliance product (Go rewrite)
├── proto/
│   └── soul/v1/           # Protobuf definitions
│       └── product.proto
├── go.mod
├── go.sum
├── Makefile
├── packages/              # Existing TS packages (kept for legacy CLI)
├── apps/cli/              # Existing TS CLI (kept, backward compat)
└── docs/plans/
```

Vite (not Next.js) for the SPA — no SSR needed since Go serves static files and handles all API routes.

**Build flow:**
```
make build
  1. cd web && npm run build     → web/dist/
  2. go build -o soul cmd/soul/  → embeds web/dist/* via go:embed

Result: single binary (~20-25MB)
```

## 2. gRPC Product Interface

Every product implements one protobuf service. Language-agnostic contract.

```protobuf
syntax = "proto3";
package soul.v1;

service ProductService {
  rpc GetManifest(Empty) returns (Manifest);
  rpc ExecuteTool(ToolRequest) returns (ToolResponse);
  rpc ExecuteToolStream(ToolRequest) returns (stream ToolEvent);
  rpc Health(Empty) returns (HealthResponse);
}

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

message Artifact {
  string type = 1;
  string path = 2;
  bytes content = 3;
}
```

Key choices:
- `ExecuteToolStream` enables real-time UI updates (scan progress, findings appearing live)
- `input_schema_json` is JSON Schema so the UI can dynamically render forms for any tool
- `Manifest` lets core auto-discover capabilities without hardcoding

## 3. Go Core Server

The core is intentionally thin — it doesn't know about compliance or any specific product. It's a router, proxy, and AI client.

```
internal/
├── server/
│   ├── server.go             # HTTP mux + WebSocket upgrader
│   ├── routes.go             # Route registration
│   ├── middleware.go         # CORS, auth, logging
│   └── spa.go               # Serve embedded React SPA
├── ai/
│   ├── client.go             # Claude API client (Messages API)
│   ├── stream.go             # SSE → WebSocket bridge
│   └── tools.go              # Convert product tools to Claude tool_use format
├── products/
│   ├── manager.go            # Discover, start, stop product binaries
│   ├── registry.go           # Track running products + their manifests
│   └── proxy.go              # Route tool calls to correct product via gRPC
├── session/
│   ├── session.go            # Chat session state (messages, context)
│   └── store.go              # In-memory store (SQLite later)
├── tier/
│   └── tier.go               # Read ~/.soul/credentials.json, enforce tiers
└── config/
    └── config.go             # CLI flags, env vars, defaults
```

**Chat message flow:**
1. Browser sends message via WebSocket
2. Core calls Claude Messages API with session history + available tools (streaming)
3. Claude streams tokens → forwarded to browser as `chat.token`
4. Claude emits `tool_use` → core routes to product via gRPC
5. Product streams progress → forwarded as `tool.progress` and `tool.finding`
6. Product completes → core feeds `tool_result` back to Claude
7. Claude generates final response → streamed to browser
8. Done → `chat.done`

**Go core dependencies:** grpc, websocket, yaml. 3 external deps.

## 4. Compliance Product (Go Rewrite)

Standalone Go binary implementing `ProductService`. Same 5 analyzers, same 83 YAML rules, same logic.

```
products/compliance-go/
├── main.go                    # gRPC server on unix socket
├── analyzers/
│   ├── analyzer.go            # Analyzer interface
│   ├── secret_scanner.go      # 16 regex patterns + Shannon entropy
│   ├── config_checker.go      # .env, Dockerfile, package.json, CORS
│   ├── git_analyzer.go        # .gitignore, CODEOWNERS, SECURITY.md, LICENSE
│   ├── dep_auditor.go         # Unpinned deps, lockfile, engines, copyleft
│   └── ast_analyzer.go        # 8 regex patterns for code anti-patterns
├── rules/
│   ├── loader.go              # YAML → []Rule, framework filtering
│   ├── soc2.yaml              # Embedded via go:embed
│   ├── hipaa.yaml
│   └── gdpr.yaml
├── scan/
│   ├── orchestrator.go        # Parallel goroutines, dedup, summary
│   └── scanner.go             # Directory walker
├── fix/
│   ├── fix.go                 # Patch generation
│   └── strategies.go          # Secret→env, weak hash→sha256, etc.
├── reporters/
│   ├── terminal.go
│   ├── json.go
│   ├── badge.go               # SVG generation
│   ├── html.go
│   └── pdf.go                 # Optional: wkhtmltopdf
├── monitor/
│   └── monitor.go             # fsnotify watcher + debounce + diff
└── go.mod                     # Deps: grpc, yaml, fsnotify (3 total)
```

What stays identical from TS: all 83 YAML rules, all regex patterns, score formula, color thresholds, SVG template, dedup key (`file:line:id`), tier gating logic.

Go advantages for compliance: `filepath.WalkDir` faster than Node.js, goroutines simpler than `Promise.allSettled`, `go:embed` bakes rules into binary.

## 5. React SPA (Frontend)

Vite + React + TypeScript. Chat-centric layout with live side panels. Dark theme.

```
web/src/
├── main.tsx
├── App.tsx                       # Layout: chat + resizable panel sidebar
├── components/
│   ├── chat/
│   │   ├── ChatView.tsx          # Message list + input
│   │   ├── Message.tsx           # User or AI message bubble
│   │   ├── ToolCall.tsx          # Expandable tool execution block
│   │   ├── StreamingText.tsx     # Token-by-token rendering
│   │   └── InputBar.tsx          # Text input + send
│   ├── panels/
│   │   ├── PanelContainer.tsx    # Resizable right sidebar
│   │   ├── CompliancePanel.tsx   # Score, findings, live updates
│   │   ├── FindingsTable.tsx     # Sortable/filterable findings
│   │   ├── ScanProgress.tsx      # Real-time analyzer progress
│   │   └── DiffPreview.tsx       # Unified diff viewer
│   ├── common/
│   │   ├── Badge.tsx
│   │   ├── Spinner.tsx
│   │   └── Header.tsx
│   └── products/
│       └── ProductSwitcher.tsx   # Switch between product panels
├── hooks/
│   ├── useWebSocket.ts
│   ├── useChat.ts
│   ├── useProduct.ts
│   └── useScanResult.ts
├── lib/
│   ├── api.ts
│   ├── ws.ts                     # WebSocket with auto-reconnect
│   └── types.ts
└── styles/
    └── globals.css
```

**Layout:**
```
┌────────────────────────────┬────────────────────────┐
│ ◆ Soul                     │ Compliance    Score     │
├────────────────────────────┤ ━━━━━━━━━━━━  87%      │
│                            │                        │
│  AI Chat                   │ Findings (12)          │
│  (streaming responses,     │ ✗ CRIT AWS key         │
│   tool calls inline,       │ ✗ CRIT SQL injection   │
│   markdown rendered)       │ ⚠ HIGH Docker root     │
│                            │ ⚠ HIGH Unpinned deps   │
│                            │                        │
│                            │ [Fix All] [Export]     │
├────────────────────────────┤────────────────────────│
│ Ask Soul anything...   [↵] │ Products               │
└────────────────────────────┴────────────────────────┘
```

Design principles: dark theme, minimal deps (react, react-router, react-markdown, tailwindcss), responsive panels, real-time first (WebSocket, no polling).

## 6. WebSocket Protocol

Single WebSocket connection. Typed JSON messages.

**Client → Server:**
- `chat.send` — user message with session_id and content
- `tool.execute` — direct tool execution (UI button click)

**Server → Client:**
- `chat.token` — streamed AI token
- `chat.tool_call` — AI wants to use a tool
- `chat.done` — AI finished response
- `tool.progress` — scan progress percentage and message
- `tool.finding` — individual finding discovered in real-time
- `tool.complete` — tool finished with full result
- `error` — error message

Reconnection: auto-reconnect with exponential backoff. Session state is server-side so reconnecting resumes the conversation.

## 7. Product Discovery & Lifecycle

Products are discovered from `~/.soul/products/*/manifest.yaml` at startup.

**manifest.yaml:**
```yaml
name: compliance
version: 0.2.0
description: Security & compliance scanning for SOC2, HIPAA, GDPR
binary: soul-compliance
tier: free
language: go
```

**Lifecycle:**
1. Scan `~/.soul/products/*/manifest.yaml`
2. Start each product binary as subprocess on a unix domain socket
3. Connect gRPC, call `GetManifest()` to get tools
4. Register tools in core registry
5. Start HTTP/WS server, serve UI

**Crash handling:** Health checks every 10s, auto-restart with exponential backoff (1s→30s max), UI shows product status.

**Built-in product:** Compliance binary embedded in the main soul binary via `go:embed`. Works out of the box with zero setup. Can be overridden by placing a binary in `~/.soul/products/compliance/`.

## 8. Claude AI Integration

Go core runs the agent loop: call Claude → handle tool_use → execute via gRPC → feed result back → loop.

**Tool name mapping:** Products declare tools as `scan`, `fix`, etc. Core registers them with Claude as `{product}__{tool}` (e.g. `compliance__scan`). Double underscore separator.

**Authentication:** API key from `~/.soul/credentials.json` or `ANTHROPIC_API_KEY` env var. OAuth support carried over.

**Model:** Default `claude-sonnet-4-20250514`. User configurable via config or `--model` flag.

**Context:** Session history in memory. Large tool results summarized before adding to context. SQLite persistence is future work.

## 9. Testing Strategy

Three layers:

**Go core:** `go test ./...` with stdlib `testing` + `httptest`. Mock gRPC servers for product tests. Tests core without real product binaries.

**Compliance product (Go):** Same fixture-based approach as TS version. `testdata/vulnerable-app/` and `testdata/compliant-app/` directories. Port same assertions.

**React SPA:** Vitest + React Testing Library. Mock WebSocket connections. Component tests for chat, panels, diff viewer.

**Integration:** Build full binary, start `soul serve` with compliance, WebSocket client sends chat message, assert streaming events received, assert findings match expected.

## 10. Build & Distribution

```makefile
build:
    cd web && npm ci && npm run build
    go build -o dist/soul cmd/soul/main.go

release:
    # Cross-compile for linux/darwin/windows × amd64/arm64
    GOOS=linux GOARCH=amd64 go build -o dist/soul-linux-amd64 cmd/soul/main.go
    # ... (5 platform targets)

dev:
    # Hot reload: air for Go, vite for React
    air & cd web && npm run dev
```

**Installation:**
```bash
curl -fsSL https://soul.dev/install.sh | sh    # Download binary
go install github.com/rishav1305/soul/cmd/soul@latest  # Go install
git clone ... && make build                      # From source
```

**CLI (v0.2):**
```bash
soul serve                    # Start web UI on localhost:3000
soul serve --port 8080        # Custom port
soul compliance scan ./dir    # Non-interactive (backward compat)
soul --version
soul --help
```

**Backward compatibility:** Existing TS CLI stays as `npx @soul/cli`. Go binary takes over the `soul` command name.
