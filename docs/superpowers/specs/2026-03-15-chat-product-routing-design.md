# Chat Product Routing — Design Spec

## Goal

Give Soul's chat the ability to interact with product servers (Tasks, Tutor, Projects, Observe) through Claude's tool-calling API, activated explicitly by the user via a tool button in the chat input.

## Architecture

### Product Context Provider

New package `internal/chat/context/` owns system prompts and tool definitions per product.

```
internal/chat/context/
  context.go       — ProductContext struct, ForProduct() dispatcher, Default()
  tasks.go         — Tasks system prompt + tool definitions
  tutor.go         — Tutor system prompt + tool definitions
  projects.go      — Projects system prompt + tool definitions
  observe.go       — Observe system prompt + tool definitions
  dispatch.go      — Tool call dispatcher (calls product REST APIs, returns results)
```

### Data Flow

1. User clicks tool button in chat input, selects "Tutor"
2. Frontend sends `session.setProduct` WS message
3. Backend stores `product` on session row in chat.db (persisted)
4. User sends chat message — handler calls `context.ForProduct("tutor")`
5. Returns `ProductContext{System: "...", Tools: [...]}`
6. Handler sets `req.System` and `req.Tools` on stream request to Claude
7. Claude responds with `tool_use` content blocks
8. Handler calls `dispatch.Execute(product, toolName, input)` — REST call to product server
9. Tool result sent back to Claude as `tool_result`
10. Claude continues — may call more tools or produce final text response
11. Final text streamed to user via existing WebSocket token flow

### Session-Level Product Binding

Product selection is **per-session** and persisted. Switching sessions restores the product. Users can change or clear the product at any time.

Schema change:
```sql
ALTER TABLE sessions ADD COLUMN product TEXT NOT NULL DEFAULT '';
```

Values: empty string (default/general), `tasks`, `tutor`, `projects`, `observe`.

## System Prompts

### Default Mode (no product selected)

```
You are Soul, an AI development assistant. You are part of Soul v2 — a platform
with 4 products: Tasks (autonomous task execution), Tutor (interview prep with
spaced repetition), Projects (skill-building project tracking), and Observe
(observability metrics dashboard). The user can select a product using the tool
button to enable product-specific actions. Without a product selected, you are
a general-purpose assistant.
```

### Per-Product Prompts

Each product prompt includes:
1. What the product does (2-3 sentences)
2. Available tools and when to use them
3. Key domain concepts
4. Constraints

Target: ~200-400 tokens per prompt.

Cross-product awareness line included in all product prompts:
> "The user may reference other Soul products. If the question is about a different product, suggest they switch using the tool button."

### Prompt Caching

The stream client already sends `prompt-caching-2024-07-31` as a beta header. Since product system prompts are static per product, they benefit from prompt caching automatically — the same system prompt within a session will be cached after the first request, avoiding re-tokenization on subsequent messages.

## Tool Definitions

### Tasks (6 tools)

| Tool | Method | Endpoint | Purpose |
|------|--------|----------|---------|
| `list_tasks` | GET | /api/tasks | List tasks, filter by stage/product |
| `create_task` | POST | /api/tasks | Create task (title, description) |
| `get_task` | GET | /api/tasks/{id} | Get single task |
| `update_task` | PATCH | /api/tasks/{id} | Update task fields |
| `start_task` | POST | /api/tasks/{id}/start | Start task execution |
| `stop_task` | POST | /api/tasks/{id}/stop | Stop running task |

### Tutor (7 tools)

| Tool | Method | Endpoint | Purpose |
|------|--------|----------|---------|
| `tutor_dashboard` | GET | /api/tutor/dashboard | Dashboard progress |
| `list_topics` | GET | /api/tutor/topics | List topics by module |
| `start_drill` | POST | /api/tutor/drill/start | Start spaced-repetition drill |
| `answer_drill` | POST | /api/tutor/drill/answer | Submit drill answer |
| `due_reviews` | GET | /api/tutor/drill/due | Get due SM-2 reviews |
| `create_mock` | POST | /api/tutor/mocks | Create mock interview |
| `list_mocks` | GET | /api/tutor/mocks | List mock sessions |

### Projects (6 tools)

| Tool | Method | Endpoint | Purpose |
|------|--------|----------|---------|
| `projects_dashboard` | GET | /api/projects/dashboard | Dashboard with summaries |
| `get_project` | GET | /api/projects/{id} | Full project detail |
| `update_project` | PATCH | /api/projects/{id} | Update project status/hours |
| `update_milestone` | PATCH | /api/projects/{id}/milestones/{mid} | Update milestone |
| `record_metric` | POST | /api/projects/{id}/metrics | Record a metric |
| `get_guide` | GET | /api/projects/{id}/guide | Get implementation guide |

### Observe (4 tools)

| Tool | Method | Endpoint | Purpose |
|------|--------|----------|---------|
| `observe_overview` | GET | /api/observe/overview | Status + cost + alerts |
| `observe_pillars` | GET | /api/observe/pillars | Pillar constraint health |
| `observe_tail` | GET | /api/observe/tail | Recent events (type filter, limit) |
| `observe_alerts` | GET | /api/observe/alerts | Active alerts |

## Tool Call Dispatch

### Dispatcher

`dispatch.go` maintains a registry: tool name → HTTP config (method, URL pattern, body template).

```go
type ToolRoute struct {
    Method  string            // GET, POST, PATCH
    Path    string            // "/api/tasks/{id}" — {id} substituted from input
    Product string            // which product server to call
}
```

Dispatcher resolves product server URL from environment variables (`SOUL_TASKS_URL`, `SOUL_TUTOR_URL`, `SOUL_PROJECTS_URL`, `SOUL_OBSERVE_URL`).

### HTTP Client

A single shared `http.Client` with connection pooling, initialized once at startup. Timeout is per-request via `context.WithTimeout` (10s), not on the client itself. This avoids TCP handshake overhead on repeated tool calls to the same product server.

### Execution

1. Extract path params from tool input JSON (e.g., `task_id` → `{id}`)
2. Build HTTP request with remaining input as JSON body (for POST/PATCH) or query params (for GET)
3. Call product server REST endpoint with 10s context timeout
4. Return response JSON as tool result string
5. Truncate oversized responses at 50KB

## Streaming & Tool Loop

### Existing SSE Parser Support

The stream client (`internal/chat/stream/sse.go`) already parses `content_block_start` (including `tool_use` type with ID/name fields), `content_block_delta` (including `input_json_delta` with `PartialJSON`), and `content_block_stop`. The `ContentBlock` struct in `types.go` already has `ID`, `Name`, and `Input` fields. **No changes needed to the SSE parser or stream client.**

### Handler Changes

The **handler** (`internal/chat/ws/handler.go`) needs modification to:
1. Accumulate `tool_use` content blocks during streaming (using existing parsed events)
2. Detect `stop_reason: "tool_use"` on `message_delta` event to know tool dispatch is needed
3. Dispatch tool calls and build `tool_result` messages
4. Send a follow-up streaming request with tool results appended

### Multi-Turn Tool Loop

1. Stream Claude response, forwarding text deltas to WS client as usual
2. Accumulate any `tool_use` content blocks (name, ID, input JSON)
3. When stream ends with `stop_reason: "tool_use"`, pause text streaming
4. Dispatch all tool calls via dispatcher
5. Build follow-up request: append assistant message (with tool_use blocks) + user message (with tool_result blocks) to conversation
6. Stream Claude's next response
7. Repeat until `stop_reason: "end_turn"` (no more tool calls)

### Request Validation

The existing `Request.Validate()` enforces strict role alternation. Tool-use conversations follow: `[...user, assistant(tool_use), user(tool_result)]`. Since `tool_result` uses role `user` in the Claude API, this naturally alternates with the preceding `assistant` role. However, `Validate()` must be updated to allow the case where the last message is role `user` (tool_result) when building the follow-up request mid-loop. Add a `SkipValidation bool` field on `Request` for internal tool-loop requests, or relax the validation to permit consecutive user messages when one contains tool results.

## WebSocket Protocol

### New Message Types

**Inbound — `session.setProduct`:**
```json
{"type": "session.setProduct", "sessionId": "abc", "product": "tutor"}
```
Sets or clears (empty string) the active product for the session. Uses `sessionId` (camelCase) consistent with all existing WS messages.

**Outbound — `session.productSet`:**
```json
{"type": "session.productSet", "sessionId": "abc", "product": "tutor"}
```
Confirmation broadcast after product is set.

**Reuse existing tool event types:**

The frontend already handles `tool.call`, `tool.progress`, `tool.complete`, and `tool.error` (defined in `types.ts` OutboundMessageType, handled in `useChat.ts`). Tool execution events reuse these existing types:
```json
{"type": "tool.call", "sessionId": "abc", "name": "list_tasks"}
{"type": "tool.complete", "sessionId": "abc", "name": "list_tasks", "result": "..."}
```
No new `chat.toolUse` type needed — the existing `tool.*` family covers this.

## Message Storage Model

### Persisting Tool-Use Conversations

The `messages` table already supports roles `user`, `assistant`, `tool_use`, and `tool_result` (via `validRoles` in `store.go`). Tool-use exchanges are stored as separate message rows:

1. **User message** — role `user`, content is the user's text (as today)
2. **Assistant tool_use** — role `tool_use`, content is JSON: `{"id": "toolu_xxx", "name": "list_tasks", "input": {...}}`
3. **Tool result** — role `tool_result`, content is JSON: `{"tool_use_id": "toolu_xxx", "content": "..."}`
4. **Assistant text** — role `assistant`, content is Claude's final text response

Multiple tool calls in a single turn produce multiple `tool_use` + `tool_result` pairs.

### History Reconstruction

The current handler builds `apiMessages` by treating every stored message as a single text `ContentBlock`. This must be updated to reconstruct the multi-content-block format Claude expects:

- `tool_use` messages → marshal as `ContentBlock{Type: "tool_use", ID: ..., Name: ..., Input: ...}` within an assistant message
- `tool_result` messages → marshal as `ContentBlock{Type: "tool_result", ToolUseID: ..., Content: ...}` within a user message
- Consecutive `tool_use` rows are grouped into a single assistant message with multiple content blocks
- Consecutive `tool_result` rows are grouped into a single user message with multiple content blocks

This ensures session restore produces valid Claude API requests that maintain the full tool-use context.

### Content Block Types

Extend the stream `ContentBlock` struct (or the message conversion logic) to support multi-block messages:
```go
// A message can contain multiple content blocks
type ContentBlock struct {
    Type      string          `json:"type"`                 // "text", "tool_use", "tool_result"
    Text      string          `json:"text,omitempty"`       // for type "text"
    ID        string          `json:"id,omitempty"`         // for type "tool_use"
    Name      string          `json:"name,omitempty"`       // for type "tool_use"
    Input     json.RawMessage `json:"input,omitempty"`      // for type "tool_use"
    ToolUseID string          `json:"tool_use_id,omitempty"`// for type "tool_result"
    Content   string          `json:"content,omitempty"`    // for type "tool_result"
}
```

## Frontend Changes

### Chat Input — Tool Button

- New button in chat input bar (left of send), wrench/plug icon
- Click opens popover with 4 product options + "None" to clear
- Active product shown as pill/badge next to button (e.g., `[Tutor]`)
- Sends `session.setProduct` WS message on selection

### Visual Indicators

- Active product badge in chat input area
- During tool calls: inline status below assistant message ("Using list_tasks...")
- After completion: tool usage invisible — Claude's response incorporates data naturally
- Uses existing design tokens: `bg-elevated`, `text-fg-muted`

### State Management

- `useChat` hook tracks active product per session
- Product restored on `session.switch` — the `Session` struct (Go and TypeScript) includes the `product` field, so switching sessions automatically restores the correct product context
- `session.create` starts with no product (empty string)
- `session.history` response includes the session's product field for reconnect recovery

### No Changes To

- Product pages (Tasks, Tutor, Projects, Observe)
- Router, navigation, AppLayout
- Product server code

## Error Handling

| Scenario | Behavior |
|----------|----------|
| Product server down (503) | Dispatcher returns error text as tool result; Claude communicates naturally |
| Bad tool input | Dispatcher returns validation error; Claude self-corrects |
| Tool call limit (max 5/message) | Handler injects "limit reached" tool result; Claude responds with available data |
| Timeout (10s per call) | Error tool result returned |
| Cross-product request | Claude suggests switching products via tool button |

## Pillar Compliance

### 1. PERFORMANT

| Constraint | How This Design Complies |
|------------|--------------------------|
| First token to screen < 200ms | Tool button interaction is local state — no server round-trip until selection confirmed. System prompt injection adds ~200-400 tokens, well within Claude's prompt cache window. |
| Frontend bundle < 300KB gzipped | Tool button + popover is minimal UI — no new dependencies. Estimated <1KB additional JS. |
| Server memory < 100MB at 10 sessions | Product contexts are static Go structs, allocated once at startup. Dispatcher uses a shared `http.Client` with connection pooling (reuses TCP connections to localhost product servers). No per-session memory growth. |
| Exact token budget | System prompts capped at 400 tokens per product. Tool definitions are compact JSON schemas. Total injection per request ~600-800 tokens. |

### 2. ROBUST

| Constraint | How This Design Complies |
|------------|--------------------------|
| No panic on any input | Product validation rejects unknown values. Dispatcher handles nil/empty input gracefully. Tool results are always strings — no type assertions that could panic. |
| Defined behavior for nil/empty/oversized | Empty product = default mode. Unknown product = error response. Oversized tool results truncated at 50KB. |
| Atomic DB operations | `product` column update is a single UPDATE statement — atomic by SQLite default. |
| Every error path produces user-visible message | All tool dispatch errors become tool results → Claude translates to natural language for user. |
| Type system prevents invalid states | `ProductContext` struct enforces system prompt + tools are always paired. Product enum validated server-side. |

### 3. RESILIENT

| Constraint | How This Design Complies |
|------------|--------------------------|
| API down → UI shows status, retains state | If product server is down, dispatcher returns error as tool result. Chat remains functional — only tool calls degrade. Product selection persisted in DB, survives restarts. |
| WS disconnect → auto-reconnect with backoff | Existing reconnect logic preserved. Product state restored from session data on reconnect. |
| Server restart → sessions restored | Product binding stored in chat.db `sessions.product` column — survives server restart. |
| Tool dispatch timeout | 10s timeout per call. Timeout produces error tool result, not a hang. Handler continues streaming. |

### 4. SECURE

| Constraint | How This Design Complies |
|------------|--------------------------|
| All input sanitized | Product name validated against allowlist (`tasks`, `tutor`, `projects`, `observe`, empty). Tool inputs are JSON — no string interpolation in SQL or shell. |
| Parameterized SQL | Product column update uses `?` placeholder: `UPDATE sessions SET product = ? WHERE id = ?`. |
| No secrets in logs | Tool dispatch logs tool name and status, never input/output data. System prompts contain no secrets. |
| CSP headers | No new endpoints served to browser. Tool calls are server-to-server (chat → product server). |
| WS origin validation | New WS message types go through existing origin-validated connection. |

### 5. SOVEREIGN

| Constraint | How This Design Complies |
|------------|--------------------------|
| Zero external CDNs/fonts/assets | No new external dependencies. Tool button uses inline SVG icon. |
| No SaaS dependencies | Tool dispatch calls internal product servers on localhost. No external services. |
| SQLite local | Product binding stored in existing chat.db. |
| Claude API abstracted, swappable | Tool definitions use `stream.Tool` struct — same abstraction layer. Dispatch is product-server-agnostic. |

### 6. TRANSPARENT

| Constraint | How This Design Complies |
|------------|--------------------------|
| Every state transition logged | `session.setProduct` logged as event. Each tool dispatch logged with tool name, duration, status. |
| Frontend errors reported to backend | Tool button errors (WS send failure) reported via existing `reportError()` pipeline. |
| No silent failures | Dispatch errors surface as tool results → Claude communicates to user. WS message failures logged server-side. |
| Feature actions tracked | `session.setProduct` tracked as `frontend.usage` event. Tool calls tracked as `chat.toolUse` events. |
| Every feature has observability | Tool dispatch timing logged. Product selection changes logged. Error rates measurable via existing metrics CLI. |

## Files Changed

| File | Action |
|------|--------|
| `internal/chat/context/context.go` | CREATE — ProductContext struct, ForProduct(), Default() |
| `internal/chat/context/tasks.go` | CREATE — Tasks system prompt + 6 tool definitions |
| `internal/chat/context/tutor.go` | CREATE — Tutor system prompt + 7 tool definitions |
| `internal/chat/context/projects.go` | CREATE — Projects system prompt + 6 tool definitions |
| `internal/chat/context/observe.go` | CREATE — Observe system prompt + 4 tool definitions |
| `internal/chat/context/dispatch.go` | CREATE — Tool call dispatcher with shared http.Client |
| `internal/chat/ws/handler.go` | MODIFY — Product context injection, tool call loop, new WS message types, tool_use accumulation, stop_reason detection |
| `internal/chat/ws/message.go` | MODIFY — Add product field to session messages |
| `internal/chat/session/store.go` | MODIFY — Add product column, SetProduct method, product in Session struct |
| `internal/chat/session/iface.go` | MODIFY — Add SetProduct to StoreInterface |
| `internal/chat/session/timed_store.go` | MODIFY — Add SetProduct delegation to TimedStore |
| `internal/chat/stream/types.go` | MODIFY — Relax Validate() for tool-use message sequences |
| `specs/session.yaml` | MODIFY — Add product field to Session spec |
| `specs/ws.yaml` | MODIFY — Add session.setProduct and session.productSet message types |
| `web/src/lib/types.ts` | REGENERATE — via `make types` after spec changes |
| `web/src/hooks/useChat.ts` | MODIFY — Track product state, handle setProduct/productSet messages, tool-use history reconstruction |
| `web/src/components/ChatInput.tsx` | MODIFY — Add tool button with product selector popover |
