# UI Redesign — Design Spec

**Date:** 2026-03-16
**Status:** Approved
**Scope:** Navigation restructure + chat window polish (Phase 1: chat page only)

## Context

Soul v2 has 10 navigation items in a horizontal top navbar and 19 products in the chat product selector. Adding more products will break the layout. The chat window needs visual polish — better contrast, professional symbols, clearer streaming/thinking states.

## Design System

- **Style:** Dark Mode (OLED), minimal, professional
- **Colors:** Background `#08081a`, cards `#0e0e24`, borders `#1a1a38`, code `#080816`
- **Accent:** Purple→blue gradient (`#7c3aed → #2563eb`) for primary actions
- **Brand:** Golden diamond logo (`#f0c040 → #d4a018 → #b8860b`) with glow
- **Typography:** Inter, monospace for metadata/code
- **Icons:** SVG only (Lucide-style). No emoji. Consistent 1.5px stroke weight.
- **Contrast:** 4.5:1 minimum for text. Three depth levels for card separation.

---

## Part 1: Navigation — Left Sidebar

### Layout: Three-Panel Desktop

```
┌──────────┬─────────────────────────┬──────────┐
│ Products │      Active Page        │ Sessions │
│ Sidebar  │    (Chat, Tasks, etc.)  │  Panel   │
│  200px   │        flex-1           │  220px   │
│ ← rail → │                         │ ← hide → │
└──────────┴─────────────────────────┴──────────┘
```

- **Left sidebar:** Scrollable flat product list (no category groups), collapses to 52px icon rail
- **Right panel:** Sessions list (chat-only), collapses to hidden
- **Top navbar:** Removed. Replaced by context-aware top bar per product.
- **Both sidebars:** Persist expanded/collapsed state in `localStorage`
- **Transition:** 200ms ease CSS transition between expanded/collapsed

### Sidebar Content (Expanded, 200px)

1. Logo + collapse button (`«`)
2. Search bar (`⌘K` shortcut hint)
3. Flat product list — all items visible, scrollable:
   - Each item: SVG icon + label (e.g., `💬 Chat` becomes `[diamond] Chat`)
   - Active item: gradient background (`#7c3aed22 → #2563eb22`) with accent border
   - Inactive: `color: #888`, hover: `#bbb` with subtle bg

### Sidebar Content (Collapsed, 52px icon rail)

- Logo icon (golden diamond, click to expand)
- SVG icons only, `34×34px` click targets with `7px` border-radius
- Active icon: gradient background
- Tooltip on hover showing product name

### Mobile (< 768px)

- Left hamburger: opens product sidebar as overlay (260px width, backdrop `#000a`)
- Right hamburger: opens sessions panel as overlay
- Tap outside or select item to dismiss
- No bottom navigation bar

### Sessions Panel (Right, 220px)

- Header: "Sessions" + collapse button (`»`)
- Session list: title, relative time, unread dot
- Active session: subtle background highlight
- `+ New` button moved to top bar (not in sessions panel)

---

## Part 2: Chat Top Bar

The top bar is context-aware — each product page exports its own actions. For chat:

```
┌─────────────────────────────────────────────────────────┐
│ [◆] · Connected    [+ New] [⟳ Running 2] [● Unread 5] [🕓 History] │
└─────────────────────────────────────────────────────────┘
```

### Elements (left to right)

| Element | Description |
|---------|-------------|
| Golden diamond | Brand logo (SVG, gold gradient + glow) |
| Connection status | Green dot + "Connected" text |
| `+ New` | Purple accent button. Creates new session. SVG plus icon + "New" label. |
| `Running` | Amber spinner icon + "Running" label + count badge. Click opens dropdown. |
| `Unread` | Purple dot icon + "Unread" label + count badge. Click opens dropdown. |
| `History` | Clock SVG icon + "History" label. Toggles right sessions panel. |

### Running Dropdown

- Title: "Active Streams"
- Each item: amber dot + session title + stream status (Streaming/Tool call) + model + elapsed time
- Click switches to that session

### Unread Dropdown

- Title: "Unread Sessions" + "Mark all read" link
- Each item: purple dot + session title + message preview + relative time
- Click switches to session and marks as read

### No model selector in top bar

Model selector stays in the input box toolbar. The top bar is for session management only.

---

## Part 3: Message Bubbles

### User Messages (right-aligned)

```
                              ┌──────────────────────┐
                              │ User message text     │
                              └──────────────────┬───┘
                                    2 min ago  ✏️
```

- Background: `#12123a`, border: `1px solid #1e1e50`
- Border-radius: `16px 16px 4px 16px` (sharp bottom-right)
- No avatar — right alignment is sufficient context
- Metadata below: time + edit icon (hover-visible)

### Assistant Messages (left-aligned)

```
  ◆ Opus 4.6 · 1.2k → 340
  ┌──────────────────────────────────┐
  │ Response text with markdown      │
  │ ┌──────────────────────────┐     │
  │ │ code block          [📋] │     │
  │ └──────────────────────────┘     │
  └──────────────────────────────────┘
  2 min ago  📋  🔄
```

- Header row: golden diamond (20px) + model tag (monospace, `#444`) + token count (arrow notation `1.2k → 340`)
- No "Soul" label — understood from context
- Body: background `#0e0e24`, border `1px solid #1a1a38`
- Border-radius: `4px 14px 14px 14px` (sharp top-left near avatar)
- Code blocks: background `#080816`, border `#14142e`, copy button right-aligned
- Metadata below: time + copy icon + retry icon (hover-visible)
- Line-height: 1.7 for readability

---

## Part 4: Thinking & Tool Call States

### Thinking — Completed (collapsed)

```
  [💡] Extended thinking · 2.4s  ▾
```

- Pill with lightbulb SVG icon, duration, expand chevron
- Background: `#0e0e24`, border: `#1a1a38`
- Click to expand and show thinking content

### Thinking — In Progress (expanded)

```
  ┌─────────────────────────────────────┐
  │ 💡 Extended thinking          3.1s  │
  │ ┃ Analyzing the configuration       │
  │ ┃ needs for a reverse proxy...  ●   │
  └─────────────────────────────────────┘
```

- Golden border (`#d4a01833`), lightbulb icon with golden glow
- Live text preview with left border accent (`2px solid #d4a01833`)
- Animated dot at end of text
- Timer (monospace, right-aligned)

### Tool Call — Running

```
  [⟳] tunnel.create · running
```

- Amber spinning SVG circle + tool name (monospace) + "running" label
- Compact single-line pill

### Tool Calls — Completed (collapsed group)

```
  [✓] 3 tools completed · tunnel.create, dns.route, service.install · 1.2s  ▾
```

- Green checkmark + count + tool names (monospace, truncated) + total duration + expand chevron
- Click to expand individual tool call details

---

## Part 5: Chat Input Box

### Structure

```
┌──────────────────────────────────────────────────┐
│ Message...                                        │
├──────────────────────────────────────────────────┤
│ [≡ General ▾] [Chat|Code|Arch] [Opus 4.6] [💡 Think] ··· [📎 Attach] [→] │
└──────────────────────────────────────────────────┘
```

- Single unified container: `border-radius: 14px`, background `#0e0e24`, border `#1e1e40`
- Textarea on top, toolbar below inside the same container
- Toolbar separated by `1px solid #14142e`

### Toolbar Elements

| Element | Description |
|---------|-------------|
| Product selector | SVG filter icon + "General" (default) + chevron. Dropdown shows flat product list. |
| Mode pills | Chat / Code / Arch / Brain segmented control. Active has subtle bg. |
| Model selector | Monospace label, dropdown. |
| Think toggle | Lightbulb SVG + "Think" label. Toggles extended thinking. |
| Attach | Paperclip SVG + "Attach" label. Opens file picker. |
| Send | 28px gradient circle (purple→blue) with arrow SVG. |

### Product Selector Default

- Default label: **"General"** (not "None" or "— None (general)")
- When a product is selected: shows product icon + name (e.g., `[≡ Scout ▾]`)

---

## Part 6: Streaming Animation

### Diamond Logo

- Default: golden gradient with subtle `drop-shadow(0 0 4px #d4a01844)`
- Streaming: opacity breathes (0.7→1→0.7, 2s cycle), glow intensifies to `6px #d4a01866`

### Text Cursor

- Golden blinking bar (`2×15px`, `#d4a018`) at end of streaming text
- `animation: blink 1s step-end infinite`

### Thinking Dots

- Three dots inside lightbulb icon with staggered opacity animation (0.2s offset each)
- Golden color matching the brand

### Word Count

- Below streaming message: `24 words` in monospace `#444`
- Updates live during streaming

---

## Files Modified

| File | Changes |
|------|---------|
| `web/src/layouts/AppLayout.tsx` | Replace top navbar with sidebar + right panel layout |
| `web/src/components/Sidebar.tsx` | New: collapsible product sidebar with icon rail |
| `web/src/components/SessionsPanel.tsx` | New: right-side sessions panel (extracted from ChatPage) |
| `web/src/components/TopBar.tsx` | New: context-aware top bar shell |
| `web/src/components/ChatTopBar.tsx` | New: chat-specific top bar actions (new, running, unread, history) |
| `web/src/components/RunningDropdown.tsx` | New: running sessions dropdown |
| `web/src/components/UnreadDropdown.tsx` | New: unread sessions dropdown |
| `web/src/components/MessageBubble.tsx` | Restyle bubbles, golden avatar, remove "Soul" label, new thinking/tool states |
| `web/src/components/ThinkingBlock.tsx` | Restyle: golden lightbulb, expanded/collapsed states |
| `web/src/components/ToolCallBlock.tsx` | Restyle: compact pills, collapsed group summary |
| `web/src/components/ChatInput.tsx` | Unified container, "General" default, labeled buttons, golden theme |
| `web/src/pages/ChatPage.tsx` | Remove inline session list, use SessionsPanel + ChatTopBar |
| `web/src/app.css` | New color tokens, golden gradient vars, animation keyframes |

## Mockups

Visual mockups saved in `.superpowers/brainstorm/559959-1773635711/`:
- `nav-pattern-v3.html` — Three-panel layout (sidebar + content + sessions)
- `chat-topbar-v1.html` — Context-aware top bars per product
- `chat-polish-v3.html` — Final chat window design (golden diamond, SVG icons, thinking states)
