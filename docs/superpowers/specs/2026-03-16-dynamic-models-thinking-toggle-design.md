# Dynamic Model Discovery + Thinking Toggle

**Date:** 2026-03-16
**Status:** Approved
**Scope:** Replace hardcoded model list with dynamic API discovery, replace /think command with cycling thinking toggle button

## Overview

Add a `GET /api/models` endpoint to the chat server that queries the Claude API's `GET /v1/models`, caches the result (1h TTL), and returns a curated list with max_tokens metadata. Replace the frontend's hardcoded model dropdown with a dynamic fetch. Replace the `/think` slash command with a 3-state cycling button (Off → Auto → Max) that maps to the Claude API's thinking types (disabled, adaptive, enabled).

## Backend: `/api/models` Endpoint

### Handler (`internal/chat/server/models.go`)

New file with:

**Cache struct:**
```go
type modelCache struct {
    mu        sync.RWMutex
    models    []ModelInfo
    fetchedAt time.Time
    ttl       time.Duration // 1 hour
}
```

**ModelInfo response type:**
```go
type ModelInfo struct {
    ID         string `json:"id"`
    Name       string `json:"name"`
    CreatedAt  string `json:"created_at"`
    MaxTokens  int    `json:"max_tokens"`
}
```

**Fetch logic:**
1. Check cache — if valid (within TTL), return cached
2. Call `GET https://api.anthropic.com/v1/models?limit=100` with OAuth token from the server's existing `auth.OAuthTokenSource`
3. Filter results to current-generation models (ID prefix: `claude-opus-4`, `claude-sonnet-4`, `claude-haiku-4`)
4. Map `display_name` to `Name`, look up `max_tokens` from known mapping
5. Sort by `created_at` descending (newest first)
6. Cache and return

**Known max_tokens mapping:**
```go
var knownMaxTokens = map[string]int{
    "claude-opus-4":   64000,
    "claude-sonnet-4": 64000,
    "claude-haiku-4":  64000,
}
```
Prefix match: `claude-opus-4-6` matches `claude-opus-4`. Unknown models default to 16384.

**Response format:**
```json
{
    "models": [
        {"id": "claude-opus-4-6", "name": "Opus 4.6", "created_at": "...", "max_tokens": 64000},
        {"id": "claude-sonnet-4-6", "name": "Sonnet 4.6", "created_at": "...", "max_tokens": 64000}
    ],
    "thinking_types": ["disabled", "adaptive", "enabled"]
}
```

**Error handling:** If the Claude API call fails (network, auth), return cached data if available. If no cache, return HTTP 503 with `{"error": "unable to fetch models"}`.

**Nil guard:** `s.auth` can be nil (auth is optional via `WithAuth`). The handler must check `s.auth != nil` before calling `s.auth.Token()`. If nil, return 503 with `{"error": "authentication not configured"}`.

### Route Registration (`internal/chat/server/server.go`)

Add `s.mux.HandleFunc("GET /api/models", s.handleModels)` alongside existing API routes.

## Backend: Thinking Config in Stream

### Message Types (`internal/chat/ws/message.go`)

Add to the ws package:
```go
type ThinkingConfig struct {
    Type         string `json:"type"`                     // "disabled", "adaptive", "enabled"
    BudgetTokens int    `json:"budget_tokens,omitempty"`  // only for type="enabled"
}
```

Add `Thinking *ThinkingConfig` field to `InboundMessage`.

### Stream Request (`internal/chat/stream/types.go`)

Add `Thinking` field to the `Request` struct:
```go
type ThinkingParam struct {
    Type         string `json:"type"`
    BudgetTokens int    `json:"budget_tokens,omitempty"`
}

// In Request:
Thinking *ThinkingParam `json:"thinking,omitempty"`
```

### Stream Client (`internal/chat/stream/client.go`)

The thinking param is part of the Request struct and will be marshaled into the API request body automatically.

**Required beta header:** The Claude API requires `interleaved-thinking-2025-05-14` in the `anthropic-beta` header for thinking support. Update `cmd/chat/main.go` where the stream client is created — add the thinking beta to the existing beta header string:
```go
stream.WithBetaHeader("prompt-caching-2024-07-31," + auth.OAuthBetaHeader + ",interleaved-thinking-2025-05-14")
```

### WS Handler (`internal/chat/ws/handler.go`)

In `handleChatSend`, when building `stream.Request`, pass the thinking config:
```go
if msg.Thinking != nil && msg.Thinking.Type != "disabled" {
    req.Thinking = &stream.ThinkingParam{
        Type:         msg.Thinking.Type,
        BudgetTokens: msg.Thinking.BudgetTokens,
    }
}
```

When thinking is `disabled` or nil, omit the field entirely (don't send `{"type":"disabled"}` — just omit it).

**MaxTokens adjustment:** The handler currently hardcodes `MaxTokens: 4096`. When thinking is enabled with a large `budget_tokens`, the API requires `max_tokens > budget_tokens`. Override `MaxTokens` when thinking is set:
```go
if msg.Thinking != nil && msg.Thinking.Type == "enabled" && msg.Thinking.BudgetTokens > 0 {
    req.MaxTokens = msg.Thinking.BudgetTokens + 1024 // budget + room for response
}
```
For `adaptive` type, set `MaxTokens` to the model's max (passed from frontend or looked up from the known mapping).

## Frontend: useModels Hook

### `web/src/hooks/useModels.ts`

```typescript
interface ModelInfo {
    id: string;
    name: string;
    created_at: string;
    max_tokens: number;
}

interface ModelsResponse {
    models: ModelInfo[];
    thinking_types: string[];
}

function useModels(): {
    models: ModelInfo[];
    thinkingTypes: string[];
    loading: boolean;
}
```

- Fetches `GET /api/models` on mount
- Caches in component state (re-fetches on window focus after 1 hour)
- Returns empty array while loading

## Frontend: Model Selector Update

### `web/src/components/ChatInput.tsx`

**Remove:**
- Hardcoded `MODELS` array
- `/think` entry from `SLASH_COMMANDS` array
- `thinkingEnabled` boolean state variable
- `handleThinkingToggle` function
- The `think` branch in `handleSlashSelect`
- The old thinking toggle button (the binary on/off one)

**Add:** Call `useModels()` to get dynamic model list. Integrate `ThinkingToggle` component (replaces old binary toggle).

**Model dropdown:** Populated from `models` array. Each option shows `model.name` with value `model.id`.

**Persistence:** `localStorage.getItem('soul-model')` still used. If stored model ID not in fetched list, fall back to first model.

## Frontend: Thinking Toggle Button

### `web/src/components/ThinkingToggle.tsx`

New component — a button that cycles through thinking states on click.

**Props:**
```typescript
interface ThinkingToggleProps {
    value: ThinkingType;           // current state
    onChange: (t: ThinkingType) => void;
    maxTokens: number;             // from selected model, for "enabled" budget
}

type ThinkingType = 'disabled' | 'adaptive' | 'enabled';
```

**Cycle order:** disabled → adaptive → enabled → disabled

**Visual states:**

| State | Label | Tailwind Classes |
|-------|-------|-----------------|
| disabled | Off | `bg-zinc-700 text-zinc-400` |
| adaptive | Auto | `bg-blue-500/20 text-blue-400` |
| enabled | Max | `bg-amber-500/20 text-amber-400` |

**Rendering:** Compact button with brain icon (unicode or SVG) + label text. `data-testid="thinking-toggle"`.

### Integration in ChatInput

The ThinkingToggle sits next to the model `<select>`. When the user sends a message:

```typescript
const thinkingConfig = thinkingType === 'disabled' ? undefined : {
    type: thinkingType,
    ...(thinkingType === 'enabled' ? { budget_tokens: selectedModelMaxTokens - 1024 } : {}),
};
```

This is sent in the WS payload alongside the model.

### `web/src/hooks/useChat.ts`

Update the WS `chat.send` payload to include thinking:
```typescript
if (options?.thinking) payload.thinking = options.thinking;
```

### `web/src/lib/types.ts`

Add:
```typescript
export type ThinkingType = 'disabled' | 'adaptive' | 'enabled';

export interface ThinkingConfig {
    type: ThinkingType;
    budget_tokens?: number;
}
```

Update `ChatInputProps.onSend` options to include `thinking?: ThinkingConfig`.

## Files Changed

| File | Action | Responsibility |
|------|--------|---------------|
| `internal/chat/server/models.go` | Create | `/api/models` handler, cache, max_tokens map |
| `internal/chat/server/server.go` | Modify | Register GET /api/models route |
| `internal/chat/ws/message.go` | Modify | Add ThinkingConfig struct + field |
| `internal/chat/ws/handler.go` | Modify | Pass thinking config to stream.Request |
| `internal/chat/stream/types.go` | Modify | Add ThinkingParam + field to Request |
| `internal/chat/stream/client.go` | Modify | Ensure thinking marshaled in API body (verify) |
| `cmd/chat/main.go` | Modify | Add interleaved-thinking beta header to stream client |
| `web/src/hooks/useModels.ts` | Create | Fetch /api/models, cache |
| `web/src/components/ChatInput.tsx` | Modify | Dynamic models, integrate ThinkingToggle |
| `web/src/components/ThinkingToggle.tsx` | Create | Cycling toggle button |
| `web/src/lib/types.ts` | Modify | ThinkingType, ThinkingConfig types |
| `web/src/hooks/useChat.ts` | Modify | Send thinking in WS payload |
