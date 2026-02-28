# UI Enhancements v2 Design — 2026-02-28

## Overview

Ten targeted improvements: multi-model support with extended thinking, hybrid chat types, paste-to-upload, voice input via Web Speech API, navbar logo enhancement, new SoulRail/SoulPanel left sidebar, chat visibility improvements, TaskPanel icon fixes, responsive filter integration, and VS Code-style panel toggle icons.

---

## 1. Multi-Model Support + Extended Thinking

**Backend:** `/api/models` returns all 3 Claude models:
```json
[
  {"id": "claude-sonnet-4-6", "name": "Sonnet", "description": "Fast & capable"},
  {"id": "claude-opus-4-6", "name": "Opus", "description": "Most capable"},
  {"id": "claude-haiku-4-5-20251001", "name": "Haiku", "description": "Fast & lightweight"}
]
```

**Extended thinking:** Brain icon toggle in InputBar toolbar, visible only when Opus is selected. When enabled, backend sends `thinking: {type: "enabled", budget_tokens: 10000}` in the Anthropic API request. MaxTokens increases from 4096 to 16000.

**Backend wire-up:** Parse `msg.Data` in `handleChatSend` to extract `model`, `chatType`, `disabledTools`, and `thinking` fields. Pass model override to `AgentLoop`. The `ai.Request` struct gains a `Thinking` field.

### Files
- `internal/server/routes.go` — return all 3 models
- `internal/server/ws.go` — parse `msg.Data`, extract options
- `internal/server/agent.go` — accept model override, pass thinking config
- `internal/ai/client.go` — add `Thinking` field to `Request`, include in API call
- `web/src/components/chat/InputBar.tsx` — add thinking toggle (brain icon)
- `web/src/lib/types.ts` — add `thinking` to `SendOptions`

---

## 2. Hybrid Chat Types (Prompt Modes + Skill Triggers)

Replace `Chat/Code/Planner` with a richer set grouped into two categories:

**Prompt modes** (change system prompt):
- **Chat** (default) — general assistant
- **Code** — code-focused, concise, shows code blocks
- **Architect** — system design, architecture, planning

**Skill triggers** (inject skill-specific system prompt + behavior):
- **Debug** — systematic debugging workflow
- **Review** — code review mode
- **TDD** — test-driven development
- **Brainstorm** — open-ended exploration

Backend maps each type to a system prompt prefix appended to the base prompt.

### Files
- `web/src/components/chat/InputBar.tsx` — update CHAT_TYPES with grouping
- `internal/server/agent.go` — system prompt variations per chat type
- `internal/server/ws.go` — pass chatType to agent

---

## 3. Paste-to-Upload

Add `paste` event listener on the InputBar container div.

**Behavior:**
- Clipboard contains files/images → add to `files[]` state, show as attachment pills
- Images get small thumbnail preview via `URL.createObjectURL()`
- No backend upload yet — visual feedback only (upload wiring comes later)

### Files
- `web/src/components/chat/InputBar.tsx` — add paste handler

---

## 4. Voice Input (Web Speech API)

**When textarea is empty AND not streaming:** Replace send button with microphone icon.

**Behavior:**
- Click mic → start `webkitSpeechRecognition` (`continuous: true`, `interimResults: true`)
- Mic button turns red with `animate-soul-pulse` to indicate recording
- Interim transcript appears in textarea (muted color)
- Final transcript replaces interim in normal color
- Click mic again or press Enter → stop recognition, auto-send
- Escape → stop recognition, keep text, don't send

**Fallback:** If `webkitSpeechRecognition` unavailable, mic button hidden (graceful degradation).

**No TTS** — STT only for now.

### Files
- `web/src/components/chat/InputBar.tsx` — voice input logic + mic button
- `web/src/lib/types.ts` — add `SpeechRecognition` type declarations

---

## 5. ChatNavbar Logo Enhancement

- Increase diamond from inline text to `text-xl`
- Wrap in container with subtle `animate-soul-pulse` glow (scaled-down amber aura)
- No float animation (too distracting at navbar size)

### Files
- `web/src/components/chat/ChatNavbar.tsx` — bigger logo + pulse glow

---

## 6. SoulRail + SoulPanel (New Left Sidebar)

**New components:** `SoulRail.tsx` + `SoulPanel.tsx`

### Rail state (collapsed, 40px):
```
┌────┐
│ ◆  │  Soul logo
│    │
│ 💬 │  Chat indicator
│ ✅ │  Tasks shortcut
│    │
│    │  spacer
│ ⚙  │  Settings
│ ▶  │  Expand (sidebar icon)
└────┘
```

### Panel state (expanded, ~240px):
```
┌──────────────────────────┐
│ ◆ Soul          [◁ col]  │  header
├──────────────────────────┤
│ + New Chat               │
│ ─── Sessions ──────────  │
│  Today                   │
│    session title          │
│  Yesterday               │
│    session title          │
│ ─── Navigation ────────  │
│  💬 Chat                 │
│  ✅ Tasks                │
├──────────────────────────┤
│  ⚙ Settings             │
│  Connection: ●           │
└──────────────────────────┘
```

### Layout impact
- AppShell becomes 3-column: `SoulRail/Panel | Chat | TaskRail/Panel`
- SoulPanel expands to `w-60` (240px), chat shrinks (not overlapping)
- SoulRail uses `w-10` (40px) like TaskRail
- `useLayoutStore` gains `soulState: PanelState` field

### Session management
- Fetch sessions from `GET /api/sessions` (existing endpoint)
- Click session → load its messages, update `useChat` session ID
- "+ New Chat" → reset messages, generate new session UUID

### Files
- NEW `web/src/components/layout/SoulRail.tsx`
- NEW `web/src/components/layout/SoulPanel.tsx`
- `web/src/components/layout/AppShell.tsx` — 3-column layout
- `web/src/hooks/useLayoutStore.ts` — add `soulState`
- `web/src/hooks/useChat.ts` — session switching support
- `web/src/lib/types.ts` — layout type updates

---

## 7. Chat Visibility Improvements

**User messages:**
- `bg-elevated rounded-xl px-4 py-3`
- Left border accent: `border-l-2 border-soul/40`

**Assistant messages:**
- Transparent background, `px-4 py-3` padding
- Consistent spacing with user messages

**InputBar container:**
- Change from `glass` to `bg-elevated border border-border-default`
- Add `shadow-lg shadow-black/20` for depth

**Chat area:**
- `mx-auto max-w-3xl` to center/constrain message width on wide screens

### Files
- `web/src/components/chat/Message.tsx` — user/assistant message styling
- `web/src/components/chat/InputBar.tsx` — container styling update
- `web/src/components/chat/ChatView.tsx` — max-width constraint on message area

---

## 8. TaskPanel View Icons

Replace unicode characters with proper SVG icons at `w-8 h-8`:

| Current | New SVG | Meaning |
|---------|---------|---------|
| `≡` | 3 horizontal lines | List view |
| `⊞` | 3 vertical columns with header dots | Kanban/Board view |
| `⊟` | 2x2 grid squares | Grid view |

### Files
- `web/src/components/planner/TaskPanel.tsx` — SVG view icons

---

## 9. TaskPanel Filters — Responsive Navbar

Move FilterBar controls into the TaskPanel navbar row:
- Wide panel (>400px): filters show inline in navbar
- Narrow panel (<400px): collapse into "Filter" button with popover
- Badge shows active filter count when collapsed

### Files
- `web/src/components/planner/TaskPanel.tsx` — inline filters
- `web/src/components/planner/FilterBar.tsx` — refactor to inline mode
- May need `useResizeObserver` or similar for width detection

---

## 10. VS Code-Style Panel Toggle Icons

Replace chevron arrows with VS Code sidebar panel icons:

| Location | Current | New |
|----------|---------|-----|
| Chat collapse | `<` chevron-left | Layout sidebar left icon (rect with left strip) |
| Task collapse | `>` chevron-right | Layout sidebar right icon (rect with right strip) |
| SoulRail expand | — (new) | Layout sidebar left icon |
| TaskRail expand | `>` chevron | Layout sidebar right icon |

The icon: a rectangle split into two sections — narrow strip on one side = the sidebar. Filled when open, outlined when collapsed.

### Files
- `web/src/components/chat/ChatNavbar.tsx` — sidebar left icon
- `web/src/components/planner/TaskPanel.tsx` — sidebar right icon
- `web/src/components/layout/TaskRail.tsx` — sidebar right icon
- `web/src/components/layout/SoulRail.tsx` — sidebar left icon

---

## File Change Summary

| File | Change |
|------|--------|
| `internal/server/routes.go` | Return all 3 models |
| `internal/server/ws.go` | Parse msg.Data, extract options |
| `internal/server/agent.go` | Model override, chatType prompts, thinking config |
| `internal/ai/client.go` | Thinking field in Request |
| `web/src/components/chat/InputBar.tsx` | Thinking toggle, voice input, paste handler, chat types, styling |
| `web/src/components/chat/ChatNavbar.tsx` | Bigger logo + glow, VS Code panel icon |
| `web/src/components/chat/ChatView.tsx` | Max-width constraint |
| `web/src/components/chat/Message.tsx` | User/assistant message styling |
| `web/src/components/layout/SoulRail.tsx` | NEW — left sidebar rail |
| `web/src/components/layout/SoulPanel.tsx` | NEW — left sidebar panel |
| `web/src/components/layout/AppShell.tsx` | 3-column layout |
| `web/src/components/layout/TaskRail.tsx` | VS Code panel icon |
| `web/src/components/planner/TaskPanel.tsx` | SVG view icons, inline filters, VS Code panel icon |
| `web/src/components/planner/FilterBar.tsx` | Refactor for inline mode |
| `web/src/hooks/useLayoutStore.ts` | soulState field |
| `web/src/hooks/useChat.ts` | Session switching |
| `web/src/lib/types.ts` | SendOptions.thinking, layout types, speech types |
| `web/src/styles/globals.css` | Any new utility classes needed |
