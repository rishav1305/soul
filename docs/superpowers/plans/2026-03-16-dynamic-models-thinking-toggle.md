# Dynamic Model Discovery + Thinking Toggle Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace hardcoded model list with dynamic Claude API discovery, replace /think command with a 3-state cycling thinking toggle (Off → Auto → Max).

**Architecture:** New `GET /api/models` endpoint queries Claude API `/v1/models`, caches 1h. Frontend `useModels` hook fetches on mount. `ThinkingToggle` component cycles disabled/adaptive/enabled. Thinking config flows through WS → handler → stream → Claude API.

**Tech Stack:** Go 1.24, React 19, TypeScript 5.9, Claude API `/v1/models`

**Spec:** `docs/superpowers/specs/2026-03-16-dynamic-models-thinking-toggle-design.md`

---

## File Map

| File | Action | Responsibility |
|------|--------|---------------|
| `internal/chat/server/models.go` | Create | `/api/models` handler, model cache, max_tokens map |
| `internal/chat/server/server.go` | Modify | Register GET /api/models route |
| `internal/chat/stream/types.go` | Modify | Add ThinkingParam to Request |
| `internal/chat/ws/message.go` | Modify | Add ThinkingConfig to InboundMessage |
| `internal/chat/ws/handler.go` | Modify | Pass thinking to stream.Request, adjust MaxTokens |
| `cmd/chat/main.go` | Modify | Add thinking beta header |
| `web/src/hooks/useModels.ts` | Create | Fetch /api/models |
| `web/src/components/ThinkingToggle.tsx` | Create | 3-state cycling button |
| `web/src/components/ChatInput.tsx` | Modify | Dynamic models, remove /think, integrate toggle |
| `web/src/lib/types.ts` | Modify | ThinkingType, ThinkingConfig |
| `web/src/hooks/useChat.ts` | Modify | Send ThinkingConfig in WS payload |

---

## Task 1: Stream — ThinkingParam on Request

**Files:**
- Modify: `internal/chat/stream/types.go`

- [ ] **Step 1: Read stream/types.go to find the Request struct**

- [ ] **Step 2: Add ThinkingParam type and field to Request**

```go
// ThinkingParam configures Claude's extended thinking.
type ThinkingParam struct {
	Type         string `json:"type"`                    // "enabled", "adaptive"
	BudgetTokens int    `json:"budget_tokens,omitempty"` // required for type="enabled"
}

// Add to Request struct:
Thinking *ThinkingParam `json:"thinking,omitempty"`
```

- [ ] **Step 3: Verify compilation**

Run: `cd /home/rishav/soul-v2 && go build ./internal/chat/stream/`
Expected: SUCCESS

- [ ] **Step 4: Commit**

```bash
git add internal/chat/stream/types.go
git commit -m "feat: add ThinkingParam to stream.Request"
```

---

## Task 2: WS — ThinkingConfig on InboundMessage

**Files:**
- Modify: `internal/chat/ws/message.go`
- Modify: `internal/chat/ws/handler.go`

- [ ] **Step 1: Read message.go to find InboundMessage struct**

- [ ] **Step 2: Add ThinkingConfig type and field**

In `message.go`:
```go
// ThinkingConfig is the thinking configuration from the frontend.
type ThinkingConfig struct {
	Type         string `json:"type"`                    // "disabled", "adaptive", "enabled"
	BudgetTokens int    `json:"budget_tokens,omitempty"` // only for "enabled"
}

// Add to InboundMessage:
Thinking *ThinkingConfig `json:"thinking,omitempty"`
```

- [ ] **Step 3: Read handler.go to find where stream.Request is built in handleChatSend/runStream**

- [ ] **Step 4: Pass thinking config to stream.Request**

Find where `req := &stream.Request{...}` is built. After the model assignment, add:

```go
// Pass thinking config
if msg.Thinking != nil && msg.Thinking.Type != "" && msg.Thinking.Type != "disabled" {
	req.Thinking = &stream.ThinkingParam{
		Type:         msg.Thinking.Type,
		BudgetTokens: msg.Thinking.BudgetTokens,
	}
	// Adjust MaxTokens — API requires max_tokens > budget_tokens
	if msg.Thinking.Type == "enabled" && msg.Thinking.BudgetTokens > 0 {
		needed := msg.Thinking.BudgetTokens + 1024
		if needed > req.MaxTokens {
			req.MaxTokens = needed
		}
	}
	if msg.Thinking.Type == "adaptive" {
		req.MaxTokens = 64000 // allow room for adaptive thinking
	}
}
```

Note: `msg` here is the inbound WS message — find the actual variable name by reading the code.

- [ ] **Step 5: Verify compilation**

Run: `cd /home/rishav/soul-v2 && go build ./internal/chat/ws/`
Expected: SUCCESS

- [ ] **Step 6: Commit**

```bash
git add internal/chat/ws/message.go internal/chat/ws/handler.go
git commit -m "feat: add thinking config to WS message and stream request"
```

---

## Task 3: Beta Header for Thinking

**Files:**
- Modify: `cmd/chat/main.go`

- [ ] **Step 1: Read cmd/chat/main.go to find where stream.NewClient is called with WithBetaHeader**

- [ ] **Step 2: Add the thinking beta header**

Find the `stream.WithBetaHeader(...)` call and append `,interleaved-thinking-2025-05-14` to the existing header string.

- [ ] **Step 3: Verify compilation**

Run: `cd /home/rishav/soul-v2 && go build ./cmd/chat/`
Expected: SUCCESS

- [ ] **Step 4: Commit**

```bash
git add cmd/chat/main.go
git commit -m "feat: add interleaved-thinking beta header"
```

---

## Task 4: Backend — /api/models Endpoint

**Files:**
- Create: `internal/chat/server/models.go`
- Modify: `internal/chat/server/server.go`

- [ ] **Step 1: Create models.go with cache and handler**

```go
package server

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

// ModelInfo is a model returned by /api/models.
type ModelInfo struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	CreatedAt string `json:"created_at"`
	MaxTokens int    `json:"max_tokens"`
}

// modelsResponse is the /api/models response body.
type modelsResponse struct {
	Models        []ModelInfo `json:"models"`
	ThinkingTypes []string    `json:"thinking_types"`
}

// modelCache caches the model list from the Claude API.
type modelCache struct {
	mu        sync.RWMutex
	models    []ModelInfo
	fetchedAt time.Time
	ttl       time.Duration
}

// knownMaxTokens maps model family prefixes to their max output tokens.
var knownMaxTokens = map[string]int{
	"claude-opus-4":   64000,
	"claude-sonnet-4": 64000,
	"claude-haiku-4":  64000,
}

// currentGenPrefixes filters to current-generation models only.
var currentGenPrefixes = []string{"claude-opus-4", "claude-sonnet-4", "claude-haiku-4"}

var defaultThinkingTypes = []string{"disabled", "adaptive", "enabled"}

func maxTokensForModel(modelID string) int {
	for prefix, tokens := range knownMaxTokens {
		if strings.HasPrefix(modelID, prefix) {
			return tokens
		}
	}
	return 16384
}

func isCurrentGen(modelID string) bool {
	for _, prefix := range currentGenPrefixes {
		if strings.HasPrefix(modelID, prefix) {
			return true
		}
	}
	return false
}
```

- [ ] **Step 2: Add the fetch and handler methods**

```go
// claudeModelsResponse matches the Claude API /v1/models response.
type claudeModelsResponse struct {
	Data []struct {
		ID          string `json:"id"`
		DisplayName string `json:"display_name"`
		CreatedAt   string `json:"created_at"`
		Type        string `json:"type"`
	} `json:"data"`
	HasMore bool   `json:"has_more"`
	LastID  string `json:"last_id"`
}

func (s *Server) fetchModels() ([]ModelInfo, error) {
	if s.auth == nil {
		return nil, fmt.Errorf("authentication not configured")
	}
	token, err := s.auth.Token()
	if err != nil {
		return nil, fmt.Errorf("get token: %w", err)
	}

	req, err := http.NewRequest("GET", "https://api.anthropic.com/v1/models?limit=100", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("anthropic-version", "2023-06-01")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch models: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("models API returned %d: %s", resp.StatusCode, body)
	}

	var apiResp claudeModelsResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("decode models: %w", err)
	}

	var models []ModelInfo
	for _, m := range apiResp.Data {
		if !isCurrentGen(m.ID) {
			continue
		}
		models = append(models, ModelInfo{
			ID:        m.ID,
			Name:      m.DisplayName,
			CreatedAt: m.CreatedAt,
			MaxTokens: maxTokensForModel(m.ID),
		})
	}
	return models, nil
}

func (s *Server) handleModels(w http.ResponseWriter, r *http.Request) {
	s.modelCache.mu.RLock()
	cached := s.modelCache.models
	valid := time.Since(s.modelCache.fetchedAt) < s.modelCache.ttl
	s.modelCache.mu.RUnlock()

	if valid && cached != nil {
		writeJSON(w, http.StatusOK, modelsResponse{
			Models:        cached,
			ThinkingTypes: defaultThinkingTypes,
		})
		return
	}

	models, err := s.fetchModels()
	if err != nil {
		// Fall back to cache if available
		if cached != nil {
			writeJSON(w, http.StatusOK, modelsResponse{
				Models:        cached,
				ThinkingTypes: defaultThinkingTypes,
			})
			return
		}
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{
			"error": "unable to fetch models",
		})
		return
	}

	s.modelCache.mu.Lock()
	s.modelCache.models = models
	s.modelCache.fetchedAt = time.Now()
	s.modelCache.mu.Unlock()

	writeJSON(w, http.StatusOK, modelsResponse{
		Models:        models,
		ThinkingTypes: defaultThinkingTypes,
	})
}
```

- [ ] **Step 3: Add modelCache field to Server struct in server.go**

Read `server.go`, find the `Server` struct. Add:
```go
modelCache modelCache
```

Initialize in `New()`:
```go
s.modelCache = modelCache{ttl: time.Hour}
```

- [ ] **Step 4: Register the route in server.go**

Find where API routes are registered (look for `s.mux.HandleFunc`). Add:
```go
s.mux.HandleFunc("GET /api/models", s.handleModels)
```

- [ ] **Step 5: Verify compilation**

Run: `cd /home/rishav/soul-v2 && go build ./cmd/chat/`
Expected: SUCCESS

- [ ] **Step 6: Commit**

```bash
git add internal/chat/server/models.go internal/chat/server/server.go
git commit -m "feat: add /api/models endpoint with Claude API discovery and caching"
```

---

## Task 5: Frontend Types

**Files:**
- Modify: `web/src/lib/types.ts`

- [ ] **Step 1: Read types.ts to find ChatProduct and ChatMode types**

- [ ] **Step 2: Add thinking types**

After the existing `ChatMode` type, add:
```typescript
// ── Thinking ─────────────────────────────────────────
export type ThinkingType = 'disabled' | 'adaptive' | 'enabled';

export interface ThinkingConfig {
  type: ThinkingType;
  budget_tokens?: number;
}

export interface ModelInfo {
  id: string;
  name: string;
  created_at: string;
  max_tokens: number;
}
```

Also update `ChatInputProps.onSend` — find it and change `thinking?: boolean` to `thinking?: ThinkingConfig`.

- [ ] **Step 3: Verify**

Run: `cd /home/rishav/soul-v2/web && npx tsc --noEmit`
Expected: May have errors in ChatInput.tsx and useChat.ts (they still use boolean) — that's expected, fixed in Tasks 7-8.

- [ ] **Step 4: Commit**

```bash
git add web/src/lib/types.ts
git commit -m "feat: add ThinkingType and ModelInfo types"
```

---

## Task 6: useModels Hook

**Files:**
- Create: `web/src/hooks/useModels.ts`

- [ ] **Step 1: Create the hook**

```typescript
import { useState, useEffect, useCallback } from 'react';
import { api } from '../lib/api';
import type { ModelInfo } from '../lib/types';

interface ModelsData {
  models: ModelInfo[];
  thinking_types: string[];
}

export function useModels() {
  const [models, setModels] = useState<ModelInfo[]>([]);
  const [thinkingTypes, setThinkingTypes] = useState<string[]>(['disabled', 'adaptive', 'enabled']);
  const [loading, setLoading] = useState(true);

  const fetchModels = useCallback(async () => {
    try {
      const data = await api.get<ModelsData>('/api/models');
      if (data.models && data.models.length > 0) {
        setModels(data.models);
      }
      if (data.thinking_types) {
        setThinkingTypes(data.thinking_types);
      }
    } catch {
      // Keep defaults — models dropdown will be empty but functional
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    fetchModels();
  }, [fetchModels]);

  return { models, thinkingTypes, loading };
}
```

- [ ] **Step 2: Verify**

Run: `cd /home/rishav/soul-v2/web && npx tsc --noEmit`
Expected: Compiles (hook is unused so far)

- [ ] **Step 3: Commit**

```bash
git add web/src/hooks/useModels.ts
git commit -m "feat: add useModels hook for dynamic model discovery"
```

---

## Task 7: ThinkingToggle Component

**Files:**
- Create: `web/src/components/ThinkingToggle.tsx`

- [ ] **Step 1: Create the component**

```typescript
import type { ThinkingType } from '../lib/types';

const THINKING_STATES: { type: ThinkingType; label: string; className: string }[] = [
  { type: 'disabled', label: 'Off', className: 'bg-zinc-700 text-zinc-400' },
  { type: 'adaptive', label: 'Auto', className: 'bg-blue-500/20 text-blue-400' },
  { type: 'enabled', label: 'Max', className: 'bg-amber-500/20 text-amber-400' },
];

interface ThinkingToggleProps {
  value: ThinkingType;
  onChange: (value: ThinkingType) => void;
}

export function ThinkingToggle({ value, onChange }: ThinkingToggleProps) {
  const currentIndex = THINKING_STATES.findIndex(s => s.type === value);
  const current = THINKING_STATES[currentIndex >= 0 ? currentIndex : 0];

  const cycle = () => {
    const nextIndex = (currentIndex + 1) % THINKING_STATES.length;
    onChange(THINKING_STATES[nextIndex].type);
  };

  return (
    <button
      onClick={cycle}
      className={`flex items-center gap-1 px-2 py-1 text-xs rounded transition-colors ${current.className}`}
      title={`Thinking: ${current.label} (click to cycle)`}
      data-testid="thinking-toggle"
    >
      <span aria-hidden="true">🧠</span>
      <span>{current.label}</span>
    </button>
  );
}
```

- [ ] **Step 2: Verify**

Run: `cd /home/rishav/soul-v2/web && npx tsc --noEmit`

- [ ] **Step 3: Commit**

```bash
git add web/src/components/ThinkingToggle.tsx
git commit -m "feat: add ThinkingToggle cycling button component"
```

---

## Task 8: ChatInput — Dynamic Models + Thinking Integration

**Files:**
- Modify: `web/src/components/ChatInput.tsx`
- Modify: `web/src/hooks/useChat.ts`

This is the largest task — read carefully before making changes.

- [ ] **Step 1: Read ChatInput.tsx fully**

Understand the current structure: MODELS array, SLASH_COMMANDS, thinkingEnabled state, handleThinkingToggle, the model select, the old thinking button.

- [ ] **Step 2: In ChatInput.tsx, remove old thinking infrastructure**

Remove:
- `{ name: 'think', description: 'Toggle extended thinking' }` from `SLASH_COMMANDS`
- The `thinkingEnabled` state variable (likely `useState(false)`)
- The `handleThinkingToggle` function
- The `think` branch in `handleSlashSelect` (the case that toggles thinking)
- The old thinking toggle button element (look for `data-testid="thinking-toggle"` or similar)
- The hardcoded `MODELS` array

- [ ] **Step 3: Add new state and imports**

```typescript
import { ThinkingToggle } from './ThinkingToggle';
import { useModels } from '../hooks/useModels';
import type { ThinkingType, ThinkingConfig } from '../lib/types';

// Inside the component:
const { models } = useModels();
const [thinkingType, setThinkingType] = useState<ThinkingType>('disabled');
```

- [ ] **Step 4: Replace model dropdown**

Find the existing `<select>` for models. Replace its options source from the hardcoded array to:
```tsx
{models.map(m => (
  <option key={m.id} value={m.id}>{m.name}</option>
))}
```

Keep the localStorage persistence logic but update the fallback — if stored model not in list, use first model.

- [ ] **Step 5: Add ThinkingToggle next to model select**

Find where the model `<select>` is rendered. Place the toggle adjacent:
```tsx
<ThinkingToggle value={thinkingType} onChange={setThinkingType} />
```

- [ ] **Step 6: Update onSend to pass ThinkingConfig**

Find where `onSend` is called (in the submit handler). Change the thinking option from boolean to ThinkingConfig:

```typescript
// Old: if (thinkingEnabled) opts.thinking = true;
// New:
if (thinkingType !== 'disabled') {
  const selectedModel = models.find(m => m.id === selectedModel);
  const maxTokens = selectedModel?.max_tokens ?? 64000;
  opts.thinking = {
    type: thinkingType,
    ...(thinkingType === 'enabled' ? { budget_tokens: maxTokens - 1024 } : {}),
  };
}
```

- [ ] **Step 7: Read useChat.ts and update thinking payload**

In `useChat.ts`, find where `payload.thinking` is set (currently `payload.thinking = true`). Change to:
```typescript
if (options?.thinking) payload.thinking = options.thinking;
```

This sends the full ThinkingConfig object instead of a boolean.

- [ ] **Step 8: Verify**

Run: `cd /home/rishav/soul-v2/web && npx tsc --noEmit`
Expected: PASS (zero errors)

Run: `cd /home/rishav/soul-v2/web && npx vite build`
Expected: SUCCESS

- [ ] **Step 9: Commit**

```bash
git add web/src/components/ChatInput.tsx web/src/hooks/useChat.ts
git commit -m "feat: dynamic model selector and thinking toggle in ChatInput"
```

---

## Task 9: Full Verification

- [ ] **Step 1: Backend**

Run: `cd /home/rishav/soul-v2 && go vet ./internal/chat/... && go build ./cmd/chat/`
Expected: PASS

- [ ] **Step 2: Frontend**

Run: `cd /home/rishav/soul-v2/web && npx tsc --noEmit && npx vite build`
Expected: PASS

- [ ] **Step 3: Full build**

Run: `cd /home/rishav/soul-v2 && make build`
Expected: All 13 binaries + frontend built

- [ ] **Step 4: Live test**

Start chat server, hit `/api/models`:
```bash
./soul-chat serve &
sleep 2
curl -s http://127.0.0.1:3002/api/models | python3 -m json.tool
```
Expected: JSON response with models array and thinking_types.

- [ ] **Step 5: Fix any issues and commit**

---

## Summary

| Task | What | Files |
|------|------|-------|
| 1 | ThinkingParam on stream.Request | 1 |
| 2 | ThinkingConfig on WS message + handler | 2 |
| 3 | Beta header for thinking | 1 |
| 4 | /api/models endpoint + cache | 2 |
| 5 | Frontend types | 1 |
| 6 | useModels hook | 1 |
| 7 | ThinkingToggle component | 1 |
| 8 | ChatInput integration | 2 |
| 9 | Full verification | 0 |
| **Total** | | **~11 files** |
