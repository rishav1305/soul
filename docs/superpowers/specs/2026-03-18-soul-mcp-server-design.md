# Soul MCP Server Design

**Date:** 2026-03-18
**Status:** Approved
**Scope:** Standalone MCP server exposing Soul v2 product tools to Claude.ai

## Overview

`soul-mcp` is a standalone Go binary on port 3028 that speaks MCP (Streamable HTTP transport) on a single endpoint. Claude.ai connects via `https://soul.rishavchatterjee.com/mcp`. It reuses the existing `internal/chat/context/` package for tool definitions and the `Dispatcher` for routing tool calls to product REST APIs.

**Final state:** 1 new server binary, ~6 new Go files, 90 product tools exposed to Claude.ai.

## Architecture

```
Claude.ai → HTTPS → Cloudflare → Tailscale → nginx → :3028
                                                        ↓
                                             soul-mcp (MCP Streamable HTTP)
                                                        ↓
                                             Dispatcher.Execute()
                                                        ↓
                                   soul-{tasks,tutor,scout,...} REST APIs
```

No new dependencies beyond standard library. The MCP server is a thin protocol translation layer over existing infrastructure.

## MCP Transport

**Protocol:** MCP Streamable HTTP (2025-03-26 spec), recommended for remote servers.

**Endpoint:** Both `POST` and `GET` on the root path `/`.

- **POST:** Client sends JSON-RPC 2.0 requests. Server responds with `Content-Type: application/json` for single responses.
- **GET:** Returns `405 Method Not Allowed` (this server is stateless and does not offer server-initiated SSE streams).

Per the MCP spec, the server MUST support both HTTP methods on the same path.

**Stateless:** No session management, no persistent connections. Works naturally behind Cloudflare.

## Authentication

Claude.ai remote MCP servers require **OAuth 2.1** (Authorization Code + PKCE). The server implements:

**Required OAuth 2.1 endpoints:**
- `GET /.well-known/oauth-protected-resource` — returns `{ "resource": "https://soul.rishavchatterjee.com/mcp", "authorization_servers": ["https://soul.rishavchatterjee.com/mcp"] }`
- `GET /.well-known/oauth-authorization-server` — returns OAuth metadata (issuer, auth endpoint, token endpoint, supported grant types, PKCE methods)
- `GET /authorize` — Authorization endpoint (shows consent page or auto-approves for configured users)
- `POST /token` — Token endpoint (issues JWT access tokens, handles refresh)
- `POST /register` — Dynamic Client Registration (RFC 7591) — registers Claude.ai as a client

**Simplified flow for single-user homelab:**
Since this is a personal server, the authorize endpoint can auto-approve after verifying a pre-configured password (`SOUL_MCP_ADMIN_PASSWORD` env var). No user database needed — single-user mode.

**Token format:** JWT signed with `SOUL_MCP_SECRET` (HS256). Contains `sub: "soul-admin"`, `exp`, `iat`. Access token TTL: 1 hour. Refresh token TTL: 30 days.

**All MCP endpoints** (`POST /`, `GET /`) validate the `Authorization: Bearer <token>` header. Missing or invalid token → `401 Unauthorized`.

**Security headers:**
- `Origin` header validation on all requests (required by MCP spec to prevent DNS rebinding). Allowlist: `https://claude.ai`, `https://console.anthropic.com`, and requests with no Origin (server-to-server).

## MCP Methods

| Method | Purpose | Response |
|--------|---------|----------|
| `initialize` | Handshake — capabilities exchange | Server name ("soul-v2"), version, `capabilities: {tools: {}}` |
| `initialized` | Client notification after handshake | `202 Accepted`, no body |
| `notifications/initialized` | Alt notification form | `202 Accepted`, no body |
| `tools/list` | List available tools | Array of 90 product tool definitions |
| `tools/call` | Invoke a tool | Tool result string from Dispatcher.Execute() |
| `ping` | Keep-alive | `{}` result |

**Notification handling:** If the POST body contains only JSON-RPC notifications (no `id` field), the server MUST return `202 Accepted` with no body per the MCP Streamable HTTP spec.

**Not implemented:** resources, prompts, sampling, logging, roots — not relevant for a tools-only server.

## Tool Registration

On startup, collect tools from **canonical product names only** (one per grouped set, not all 21 aliases):

```go
canonicalProducts := []string{
    "tasks", "tutor", "projects", "observe",
    "devops",    // represents infra group (devops + dba + migrate)
    "compliance", // represents quality group (compliance + qa + analytics)
    "dataeng",    // represents data group (dataeng + costops + viz)
    "docs",       // represents docs group (docs + api)
    "sentinel", "bench", "mesh", "scout",
}
```

For each:
1. Call `context.ForProduct(product)` → get ProductContext with tools
2. Skip first N tools where N = `len(builtinTools())` (currently 8, but derived dynamically, not hardcoded)
3. Collect remaining product-specific tools into a `map[string]stream.Tool` (dedup by name)
4. Convert `stream.Tool` → MCP tool format (name, description, inputSchema — structurally identical)

**Tool counts (90 total):**
- Tasks: 6, Tutor: 7, Projects: 6, Observe: 4
- Scout: 28 (21 operational + 7 AI tools), Sentinel: 7, Mesh: 4, Bench: 4
- Compliance: 4, QA: 2, Analytics: 2
- DevOps: 2, DBA: 2, Migrate: 2
- DataEng: 2, CostOps: 2, Viz: 2
- Docs: 2, API: 2

**Excluded:** 8 built-in tools (memory_*, tool_*, subagent) — these depend on chat session state.

## Tool Call Routing

When Claude.ai invokes `tools/call`:
1. Parse tool name and arguments from MCP request
2. Marshal arguments to `json.RawMessage`
3. Create context with **60-second timeout** (`context.WithTimeout`) — covers the longest tools (Scout AI at 30s) with margin
4. Call `Dispatcher.Execute(ctx, toolName, inputJSON)`
5. Dispatcher routes to the correct product server via HTTP
6. Return response string as MCP tool result content

**Error handling:** Network errors and HTTP errors are returned as text in the tool result (not MCP protocol errors), matching how the chat dispatcher works today. Product server downtime returns a clear error message to Claude.

## Security

**Rate limiting:** 60 requests per minute per token, enforced in the auth middleware. Returns `429 Too Many Requests` with `Retry-After` header.

**Request body size:** 1 MB maximum via `http.MaxBytesReader`. Rejects oversized payloads with `413 Payload Too Large`.

**Origin validation:** Required by MCP spec. Allowlist: `https://claude.ai`, `https://console.anthropic.com`. Requests without Origin header are allowed (server-to-server calls). All others rejected with `403 Forbidden`.

**Tool safety:** The MCP server is a pass-through — it does not add any capabilities beyond what the product servers already expose. The same tools are available through the chat frontend. Product servers enforce their own authorization and input validation.

## File Structure

```
cmd/mcp/main.go                    Entrypoint (:3028)
internal/mcp/
  server/server.go                 HTTP server, MCP endpoint, middleware chain
  protocol/protocol.go             JSON-RPC 2.0 message types, MCP request/response structs
  protocol/protocol_test.go        Protocol parsing tests
  auth/auth.go                     OAuth 2.1 endpoints, JWT token management, client registration
  auth/auth_test.go                OAuth flow tests
  tools/registry.go                Collect tools from context package, handle list/call
  tools/registry_test.go           Tool collection and dispatch tests
```

## Deployment

**Environment variables:**
- `SOUL_MCP_HOST` (default `127.0.0.1`)
- `SOUL_MCP_PORT` (default `3028`)
- `SOUL_MCP_SECRET` (required — JWT signing key for access tokens)
- `SOUL_MCP_ADMIN_PASSWORD` (required — password for OAuth authorize flow)
- All existing `SOUL_*_URL` vars for product server routing (inherited by Dispatcher)

**Nginx:** `location /mcp { proxy_pass http://127.0.0.1:3028; }` in titan-services.conf. No trailing slashes — the server handles `/` as the MCP endpoint.

**SystemD:** `deploy/soul-v2-mcp.service` following existing pattern.

**Makefile:** Add `build-mcp` target, update `build`/`serve`/`clean`.

**Claude.ai configuration:** Add as remote MCP server with URL `https://soul.rishavchatterjee.com/mcp`. Claude.ai will discover OAuth endpoints automatically and drive the authorization flow.

## Operational

**Health check:** `GET /health` returns `{"status":"ok","uptime":"...","tools":90}` — excluded from auth.

**Logging:** Log every `tools/call` invocation (tool name, duration, success/error) to stdout. No sensitive data in logs.

**Graceful shutdown:** SIGINT/SIGTERM → drain active requests (10s timeout) → exit.
