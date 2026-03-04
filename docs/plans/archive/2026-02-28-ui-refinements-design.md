# UI Refinements Design — 2026-02-28

## Overview

Six targeted improvements to the Soul webapp: navbar rename, InputBar redesign with toolbar, bigger Soul logo with enhanced animation, loading splash page, TaskPanel navbar fixes, and TaskRail redesign.

---

## 1. ChatNavbar: "Soul Chat" → "Soul"

**File:** `web/src/components/chat/ChatNavbar.tsx`

Change the title span from "Soul Chat" to "Soul". No other changes.

Also replace the `[−]` collapse button with an SVG chevron-left icon in a `w-7 h-7` rounded button with hover background.

---

## 2. InputBar: Two-Section Redesign

**File:** `web/src/components/chat/InputBar.tsx` (full rewrite)

### Layout

```
┌─────────────────────────────────────────────────┐
│  Message Soul...                                │  textarea
│                                                 │  (auto-resize, NO scrollbar via
│                                                 │   overflow-y: hidden + JS height calc)
├─────────────────────────────────────────────────┤
│ [◆ Model ▾] [Type ▾] [🔧▾] [📎] [📷]    [Send]│  toolbar
└─────────────────────────────────────────────────┘
```

### Container
- Single `glass` container with rounded-2xl
- No max-width constraint — fills panel width with `mx-5` horizontal margin
- Divider between sections: `border-t border-border-subtle`

### Top Section: Textarea
- Full-width, no visible border (transparent bg)
- `overflow-y: hidden` — NO scrollbar, auto-resize via JS up to max 200px
- Placeholder: "Message Soul..."
- `font-body text-fg` styling
- `resize-none` to prevent manual resize

### Bottom Section: Toolbar Row
- `flex items-center gap-2 px-3 py-2`
- Left-aligned controls, Send button right-aligned via `ml-auto`

### Toolbar Controls

**Model Selector** (dropdown):
- Compact button: `◆ {modelName} ▾`
- Fetches from `GET /api/models` (new endpoint)
- Dropdown shows available models with descriptions
- Selected model sent as `model` field in WS message
- Default: configured model from backend
- Styling: `bg-elevated/60 rounded-lg px-2.5 py-1.5 text-xs font-display`

**Chat Type** (dropdown):
- Button: `{typeName} ▾`
- Options: "Chat" (default), "Code", "Planner"
- Sent as `chat_type` field in WS message
- Same styling as model selector

**Tool Permissions** (popover):
- Button: wrench SVG icon
- Click opens a popover listing tools from `GET /api/tools`
- Each tool: name + toggle switch
- Tools with `requires_approval=true` get a shield icon
- Disabled tools sent as `disabled_tools[]` in WS message
- Badge showing count of disabled tools when > 0

**File Attach** (button):
- Paperclip SVG icon
- Opens native file picker via hidden `<input type="file">`
- Shows attached filename as a pill/chip above the textarea
- File stored in state, uploaded on send via `POST /api/upload`

**Image Attach** (button):
- Image SVG icon
- Same as file attach but with `accept="image/*"`
- Shows thumbnail preview above textarea

**Send Button**:
- Gold `bg-soul` rounded button with arrow-up SVG icon (no text label)
- `w-8 h-8` circle
- Disabled when empty or streaming

### New Props
```ts
interface InputBarProps {
  onSend: (message: string, options?: {
    model?: string;
    chatType?: string;
    disabledTools?: string[];
    files?: File[];
  }) => void;
  disabled: boolean;
}
```

### Backend Changes Needed (stubs first)
- `GET /api/models` — returns `[{id, name, description}]` from config
- `POST /api/upload` — multipart file upload, returns `{id, url, name, size}`
- Extend WS `chat.send` to accept `model`, `chat_type`, `disabled_tools` fields (ignored initially)

---

## 3. Soul Logo: Bigger + More Prominent Animation

**File:** `web/src/components/chat/ChatView.tsx`

### Changes
- Diamond size: `text-5xl` → `text-8xl` (3rem → 6rem)
- Add radial glow behind it: pseudo-element or wrapper div with `bg-soul/20 blur-3xl rounded-full w-32 h-32 absolute`
- Animation: keep `animate-float` but increase movement range (8px → 16px)
- Add `animate-soul-pulse` as a secondary glow animation on the background ring
- Greeting text: `text-lg` → `text-xl`, add `mt-6` for more spacing

### Updated CSS (globals.css)
Increase float keyframe amplitude:
```css
@keyframes float {
  0%, 100% { transform: translateY(0); }
  50% { transform: translateY(-16px); }
}
```

---

## 4. Loading/Splash Page

**New file:** `web/src/components/layout/SplashScreen.tsx`

### Behavior
- Shown while WebSocket is connecting AND initial session hasn't loaded
- Full-screen overlay on top of AppShell (position: fixed, z-50)
- Fades out with `animate-fade-out` (opacity 1→0, 500ms) once ready
- Removed from DOM after fade completes

### Layout
```
┌──────────────────────────────────┐
│                                  │
│                                  │
│           ◆  (animated)          │  large diamond with glow
│          Soul                    │  display font, tracking-wide
│                                  │
│       ● ● ● (shimmer dots)      │  loading indicator
│                                  │
└──────────────────────────────────┘
```

- Background: `bg-deep` with noise texture
- Diamond: `text-9xl text-soul` with float + pulse glow (same as ChatView but larger)
- "Soul" text: `font-display text-2xl tracking-[0.3em] text-fg-secondary mt-8`
- Loading dots: 3 small dots with staggered soul-pulse animation
- Entire content centered with flexbox

### Integration
- `App.tsx` wraps AppShell + SplashScreen
- SplashScreen receives `ready: boolean` prop
- `useChat` or `useWebSocket` hook exposes `connected` state
- Once connected + first session loaded → `ready = true` → fade out → unmount

---

## 5. TaskPanel Navbar Fixes

**File:** `web/src/components/planner/TaskPanel.tsx`

### Problems & Fixes

**`+ New Task` button height issue:**
- Add `whitespace-nowrap shrink-0 h-7 flex items-center` to prevent height growth
- At narrow widths (< 200px panel), collapse text to just `+` icon

**Ugly icon buttons (`↻`, `×`):**

Replace with proper SVG icons in consistent `w-7 h-7` button containers:

| Current | New | Purpose |
|---------|-----|---------|
| `↻` (HTML entity) | SVG: arrow-counterclockwise (16px) | Reset panel width |
| `×` (HTML entity) | SVG: chevron-right (16px) | Collapse task panel |
| `[−]` (ChatNavbar) | SVG: chevron-left (16px) | Collapse chat panel |

All icon buttons: `w-7 h-7 flex items-center justify-center rounded hover:bg-elevated text-fg-muted hover:text-fg transition-colors`

---

## 6. TaskRail Redesign

**File:** `web/src/components/layout/TaskRail.tsx`

### Current
```
[⊞]           expand icon
dot + 9px num  per stage (6 stages)
```

### New
```
[⊞]           expand icon (top)

[+]           new task button (gold bg, w-7 h-7)

┌──┐
│ 3│          backlog (colored bg square)
└──┘
┌──┐
│ 1│          brainstorm
└──┘
... etc for all 6 stages
```

**Stage count boxes:**
- `w-7 h-7 rounded-sm flex items-center justify-center`
- Background: `bg-stage-{name}` (using existing design tokens)
- Text: `text-deep font-mono text-[11px] font-bold`
- If count is 0: `opacity-30` to dim empty stages

**`+` New Task button:**
- `w-7 h-7 rounded bg-soul/80 hover:bg-soul text-deep flex items-center justify-center`
- Shows `+` character or small plus SVG
- Clicking opens NewTaskForm modal (needs callback prop added)

### Props Change
```ts
interface TaskRailProps {
  tasksByStage: Record<TaskStage, PlannerTask[]>;
  onExpand: () => void;
  onNewTask: () => void;  // NEW
}
```

---

## File Change Summary

| File | Change |
|------|--------|
| `web/src/components/chat/ChatNavbar.tsx` | Rename + SVG collapse icon |
| `web/src/components/chat/InputBar.tsx` | Full rewrite — two-section toolbar |
| `web/src/components/chat/ChatView.tsx` | Bigger logo + glow animation |
| `web/src/components/layout/SplashScreen.tsx` | NEW — loading splash |
| `web/src/components/layout/AppShell.tsx` | Wire splash + TaskRail onNewTask |
| `web/src/components/layout/TaskRail.tsx` | Colored squares + new task button |
| `web/src/components/planner/TaskPanel.tsx` | Fix button + SVG icons |
| `web/src/styles/globals.css` | Enhanced float animation + fade-out keyframe |
| `web/src/App.tsx` | Wire SplashScreen with ready state |
| `internal/server/routes.go` | Add GET /api/models endpoint |
| `internal/server/routes.go` | Add POST /api/upload stub |

### Backend additions (minimal)
- `GET /api/models` returns `[{id: "claude-sonnet-4-6", name: "Sonnet", description: "Fast & capable"}]` from server config
- `POST /api/upload` stub that returns 501 (wired later)
- WS `chat.send` ignores extra fields gracefully (already does via `json.RawMessage`)
