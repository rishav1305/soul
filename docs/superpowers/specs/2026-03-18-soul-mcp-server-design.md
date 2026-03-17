# Soul MCP Server Design

**Date:** 2026-03-18
**Status:** Approved
**Scope:** Standalone MCP server exposing Soul v2 product tools to Claude.ai

## Overview

`soul-mcp` is a standalone Go binary on port 3028 that speaks MCP (Streamable HTTP transport) on a single `POST /mcp` endpoint. Claude.ai connects via `https://soul.rishavchatterjee.com/mcp/`. It reuses the existing `internal/chat/context/` package for tool definitions and the `Dispatcher` for routing tool calls to product REST APIs.

**Final state:** 1 new server binary, ~4 new Go files, 84 product tools exposed to Claude.ai.

## Architecture

```
Claude.ai → HTTPS → Cloudflare → Tailscale → nginx → :3028/mcp
                                                         ↓
                                              soul-mcp (MCP Streamable HTTP)
                                                         ↓
                                              Dispatcher.Execute()
                                                         ↓
                                    soul-{tasks,tutor,scout,...} REST APIs
```

No new dependencies. The MCP server is a thin protocol translation layer over existing infrastructure.

## MCP Transport

**Protocol:** MCP Streamable HTTP (the newer standard, recommended for remote servers).

**Single endpoint:** `POST /mcp`
- Client sends JSON-RPC 2.0 requests as POST body
- Server responds with JSON-RPC 2.0 response (`Content-Type: application/json`)
- Stateless — no persistent connections, works naturally behind Cloudflare

**Authentication:** Bearer token via `Authorization` header. Token set via `SOUL_MCP_TOKEN` env var. Rejects requests without valid token (401).

## MCP Methods

| Method | Purpose | Response |
|--------|---------|----------|
| `initialize` | Handshake — capabilities exchange | Server name ("soul-v2"), version, capabilities: {tools: {}} |
| `initialized` | Client notification after handshake | No response (notification) |
| `tools/list` | List available tools | Array of 84 product tool definitions |
| `tools/call` | Invoke a tool | Tool result string from Dispatcher.Execute() |
| `ping` | Keep-alive | Empty pong |

**Not implemented:** resources, prompts, sampling, logging, roots — not relevant for a tools-only server.

## Tool Registration

On startup:
1. Iterate all 21 products via `context.ForProduct(product)`
2. Extract product-specific tools (skip 8 built-in tools prepended by ForProduct)
3. Convert `stream.Tool` → MCP tool format (name, description, inputSchema)
4. Deduplicate — grouped products (devops/dba/migrate) share context, extract unique tools only

Tools keep existing names (`list_tasks`, `compliance__scan`, `lead_add`, etc.) — already unique across products.

**Tool counts (84 total):**
- Tasks: 6, Tutor: 7, Projects: 6, Observe: 4
- Scout: 21, Sentinel: 7, Mesh: 4, Bench: 4
- Compliance: 4, QA: 2, Analytics: 2
- DevOps: 2, DBA: 2, Migrate: 2
- DataEng: 2, CostOps: 2, Viz: 2
- Docs: 2, API: 2

**Excluded:** 8 built-in tools (memory_*, tool_*, subagent) — these depend on chat session state.

## Tool Call Routing

When Claude.ai invokes `tools/call`:
1. Parse tool name and arguments from MCP request
2. Marshal arguments to JSON
3. Call `Dispatcher.Execute(ctx, toolName, inputJSON)`
4. Dispatcher routes to the correct product server via HTTP
5. Return response string as MCP tool result content

Error handling: network errors and HTTP errors are returned as text in the tool result (not MCP errors), matching how the chat dispatcher works today.

## File Structure

```
cmd/mcp/main.go                    Entrypoint (:3028)
internal/mcp/
  server/server.go                 HTTP server, POST /mcp endpoint, auth middleware
  protocol/protocol.go             JSON-RPC 2.0 message types, MCP request/response structs
  tools/registry.go                Collect tools from context package, handle list/call
  tools/registry_test.go           Tool collection and dispatch tests
```

## Deployment

**Environment variables:**
- `SOUL_MCP_HOST` (default `127.0.0.1`)
- `SOUL_MCP_PORT` (default `3028`)
- `SOUL_MCP_TOKEN` (required — bearer token for auth)
- All existing `SOUL_*_URL` vars for product server routing (inherited by Dispatcher)

**Nginx:** Add `location /mcp/ { proxy_pass http://127.0.0.1:3028/; }` to titan-services.conf.

**SystemD:** `deploy/soul-v2-mcp.service` following existing pattern.

**Makefile:** Add `build-mcp` target, update `build`/`serve`/`clean`.

**Claude.ai configuration:** Add as remote MCP server with URL `https://soul.rishavchatterjee.com/mcp/mcp` and bearer token.
