# Soul Chat — Claude Code Parity Design

**Date:** 2026-03-04
**Status:** Approved
**Approach:** Soul-native with Claude Code UX patterns (Approach C)

---

## Problem Statement

Soul's chat has six gaps compared to Claude Code:

1. **Tool call UI is broken** — each tool call renders as a full-height bordered accordion card, taking up the entire screen and burying the actual response text
2. **Thinking is not surfaced** — extended thinking tokens are silently consumed; the UI shows only "Soul is thinking..."
3. **No interactive pre-implementation clarification** — agent executes immediately without asking questions, unlike Claude Code which clarifies intent first
4. **Session history not persisted** — messages live in-memory only; restarting Soul loses all history; switching sessions starts fresh
5. **No skills/superpowers integration** — Soul's agent doesn't load or follow the skills in `~/.claude/plugins/`
6. **No slash commands** — no `/brainstorm`, `/commit`, `/review` etc; no command palette in InputBar

---

## Design

### 1. Compact Tool Call UI

**Current:** Each tool call is a bordered `<div>` card (~80px tall, expanded by default) with full output visible. For an agent run with 8 tool calls this takes 640px — pushing the text response off-screen.

**New:** Tool calls render as compact one-line pills, collapsed by default.

```
✓ task_list  ·  23 tasks                              ▸
✓ task_create  ·  Task #39 created                    ▸
```

- **Pill anatomy:** `[status icon] [tool name]  ·  [brief summary]  [expand chevron]`
- **Status icons:** `◌` running (animated), `✓` complete, `✗` error
- **Height:** 28px collapsed, expands to scrollable panel (max 240px) on click
- **Group:** Multiple consecutive tool calls in one assistant turn collapse into a single `▸ 3 tool calls` summary line, expandable to show individual pills
- **Color:** muted by default (`text-fg-muted`), status color only on the icon
- **Message layout:** Text content renders first (full width, normal weight), tool pills render below as a tight stack

### 2. Thinking Block UI

**Current:** Extended thinking tokens are discarded server-side (not streamed to client); UI shows "Soul is thinking..." generic spinner.

**New:** Thinking blocks stream as a separate content type and render in a collapsible "Thinking" section above the response.

- Server: parse `thinking` content block type from Claude SSE stream, emit `chat.thinking` WS event
- Client: `useChat` accumulates `thinkingContent` alongside `content`; renders as a collapsed `💭 Thinking (N tokens)` block above the response
- Collapsed by default; user can expand to read the full reasoning chain
- Works for all models (shows nothing for non-thinking models)

### 3. Clarification-First Agent Behavior

**Two layers:**

**Agent-level (system prompt):** Add a new `clarify` chatType prompt extension that instructs the agent:
- Before writing any code or creating tasks, ask at least one clarifying question
- Use the Socratic questioning pattern from Claude Code: purpose → constraints → success criteria
- Only proceed to implementation after the user responds with enough context
- This is triggered automatically when `chatType=brainstorm` or when the user message contains implementation intent but is ambiguous

**UI-level (structured mode):** A `Clarify` toggle in the InputBar that sets `chatType=clarify`. When active:
- The InputBar shows a purple tint / different placeholder: "Describe what you want to build..."
- The agent response is structured: question first, then waits for answer before proceeding
- The mode persists for the session until toggled off

### 4. Session History Persistence

**Current state:** `planner.AddMessage()` exists but is never called from the WS handler. `session.Store` is in-memory only. Sessions in the DB have no messages.

**Fix:**

**Server side (ws.go / agent.go):**
- `handleChatSend`: after parsing `session_id`, resolve it to a DB session ID (create if new, look up if existing numeric ID)
- After agent completes: call `planner.AddMessage(dbSessionID, "user", content)` and `planner.AddMessage(dbSessionID, "assistant", fullResponse)`
- On session resume: load messages from `planner.GetSessionMessages(dbSessionID)` and inject into the agent's message history before the new turn

**Client side (useChat.ts):**
- Session ID is now the DB integer ID (not a random UUID)
- On mount: create a new session via `POST /api/sessions` and use the returned `id`
- On session switch (SessionDrawer): load messages via `GET /api/sessions/{id}/messages` and hydrate the chat state
- The `sessionIdRef` becomes a `useState<number>` that persists across reconnects

**Context window management:** For very long sessions (>50 messages), truncate to the most recent 50 messages when building the API request. Add a `[Session truncated — showing last 50 messages]` system note. Full history still stored in DB.

### 5. Skills / Superpowers Integration

**Two layers:**

**File-based (dynamic):** Soul reads skill files from `~/.claude/plugins/cache/*/skills/` at startup and indexes them by name. When the agent runs, the active skill content is injected into the system prompt.

- `internal/skills/loader.go`: scan the plugins cache, parse skill markdown files
- `Skill` struct: `{ Name, Description, Content string }`
- Skills are loaded once at startup, stored in a `SkillStore`
- The active skill for a session is set via the chatType (e.g., `chatType=brainstorm` → loads `brainstorming.md` skill content)
- Skill content is appended to the system prompt: `\n\n# Active Skill: Brainstorming\n{content}`

**Built-in fallbacks:** For key skills that should always work even without plugin files, hard-code the behavior in `chatTypePrompt()` in `agent.go`. Current `brainstorm`, `code`, `debug` modes get expanded prompts matching the skill content.

### 6. Slash Commands

**Two layers:**

**InputBar parsing:** When the user types `/`, show an autocomplete palette listing available commands. Commands are populated from:
- Loaded skill files (e.g., `/brainstorming`, `/commit`, `/review-pr`)
- Built-in commands (e.g., `/new` for new session, `/clear` for clear history, `/think` to toggle thinking)

**Command routing:**
- `/brainstorming` → sets `chatType=brainstorm`, pre-fills the skill content
- `/commit` → sets `chatType=code`, injects commit skill prompt, sends immediately
- `/new` → creates new session
- `/clear` → clears client-side message history (session still in DB)
- `/think` → toggles extended thinking
- Unknown `/foo` → sends message with the skill name injected as context

**UI:** As user types `/`, a floating palette appears above the input showing matching commands with descriptions. Arrow keys navigate, Enter selects, Escape dismisses.

---

## Data Flow

```
User types "/brainstorming build a task planner"
  → InputBar detects /brainstorming prefix
  → Sets chatType=brainstorm, strips prefix from message
  → Loads brainstorming skill content from SkillStore
  → Sends WS: { type: "chat.message", content: "build a task planner", data: { chatType: "brainstorm", skillContent: "..." } }

Server receives:
  → Resolves DB session ID, loads message history
  → Builds system prompt: base + model identity + skill content
  → Instructs agent: clarify first, don't implement yet
  → Streams response tokens back
  → On completion: persists user + assistant messages to DB

Client renders:
  → Text content (clarifying question) at top, full width
  → Tool calls (if any) as compact pills below text
  → Thinking block (if any) collapsed above text
  → Session persisted — reload resumes from here
```

---

## Files to Create / Modify

| File | Change |
|------|--------|
| `web/src/components/chat/ToolCall.tsx` | Rewrite: compact pill, grouped, collapsed by default |
| `web/src/components/chat/Message.tsx` | Reorder: text first, tools below; add thinking block |
| `web/src/hooks/useChat.ts` | Add thinking content accumulation; DB session ID; message hydration |
| `web/src/components/chat/InputBar.tsx` | Add /command parsing, autocomplete palette |
| `web/src/components/chat/ChatView.tsx` | Wire session switching |
| `internal/server/ws.go` | Persist messages to DB; load history on resume |
| `internal/server/agent.go` | Emit `chat.thinking` events; expanded chatType prompts |
| `internal/skills/loader.go` | NEW: scan plugin cache, index skill files |
| `internal/server/server.go` | Init SkillStore; pass to agent |

---

## Out of Scope (this iteration)

- Message editing / regeneration
- Branching conversations
- Multi-model routing per message
- Voice transcription improvements
- Mobile layout
