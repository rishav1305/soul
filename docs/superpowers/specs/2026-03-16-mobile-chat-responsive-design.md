# Mobile Chat Responsive Design — Spec

**Date:** 2026-03-16
**Status:** Approved
**Scope:** Chat window only (< 640px breakpoint), no backend changes
**Approach:** Mobile-first responsive — add Tailwind `max-sm:` / `sm:` variants to existing components

## Context

The Soul v2 chat interface was designed desktop-first. On mobile (< 640px):
- ChatInput toolbar (7 items) overflows and wraps badly
- Top bar buttons with text labels consume too much horizontal space
- Hard-coded pixel widths (ThinkingBlock `380px`, dropdowns `288px`, sidebar `260px`) overflow
- No safe-area handling — ChatInput hidden behind iPhone home indicator
- Message bubbles too narrow (85% max) with excessive padding for small screens
- Touch targets too small (send button 32px, tool expand 28px)
- Chat input doesn't follow keyboard on mobile (gets hidden behind soft keyboard)

## Current Component Structure (Reference)

Understanding the actual DOM before making changes:

- **`TopBar.tsx`** — Renders static `title="Chat"` (not session title), diamond logo, connection status dot, and child buttons in a `flex` row. Height `h-10`.
- **`ChatTopBar.tsx`** — Calls `<TopBar title="Chat">` and passes 4 child buttons: `+ New`, `Running ▾`, `Unread ▾`, `History`. Each button uses `btnBase` class with `h-8 px-2.5` and text labels.
- **`Sidebar.tsx`** — Renders a `fixed top-3 left-3 z-50` hamburger button on mobile (`md:hidden`). The sidebar itself is a `md:hidden fixed` overlay with `w-[260px]`.
- **`SessionsPanel.tsx`** — A flex-row sibling of the main chat content. Uses `style={{ width: open ? 220 : 0 }}` with CSS transition. Not an overlay.
- **`ChatInput.tsx`** — `CHAT_MODES` has 4 items: Chat, Code, Architect, Brainstorm (not 3).
- **`MessageBubble.tsx`** — Has `max-w-[85%]` in two places: line ~220 (legacy tool_use path with `ml-[34px]`) and line ~278 (main message bubble).
- **`ThinkingToggle.tsx`** — Text "Think · {label}" is in a single `<span>`, not a separate element from the icon.
- **`app.css`** — Already has an unconditional `.safe-bottom { padding-bottom: env(safe-area-inset-bottom, 0px); }` rule at line 201.

## Design Decisions

### 1. Top Bar — Icon-Only on Mobile

**Current structure:** `TopBar` renders: diamond + "Chat" + status dot | child buttons.

**Mobile changes (< 640px):**
- The existing `Sidebar.tsx` hamburger button (`fixed top-3 left-3 z-50`) stays as-is — do NOT add another hamburger to the top bar. The hamburger overlays the top-left area naturally.
- `TopBar` title "Chat" stays. No session title in top bar (session title is shown in the sessions panel / right panel).
- The 4 child buttons in `ChatTopBar` become icon-only: hide all `<span>` text labels with `hidden sm:inline`.
- Button base class: `h-8 px-2.5` → `h-8 px-1.5 sm:px-2.5` on mobile for tighter spacing.
- Badge overlays (Running count, Unread count) stay as-is — they're small enough.

**Dropdown width fix:**
Current `Dropdown` component uses `absolute top-full right-0 w-72`. On mobile 320px screens, `w-72` (288px) overflows.

Fix in `Dropdown` component:
```tsx
// Replace:
className="absolute top-full right-0 mt-1.5 z-50 w-72 ..."
// With:
className="mt-1.5 z-50 w-72 max-sm:fixed max-sm:left-2 max-sm:right-2 max-sm:w-auto sm:absolute sm:top-full sm:right-0 ..."
```
On mobile: `fixed` positioning with left/right margins (no `absolute`). On desktop: original `absolute` positioning. The parent `relative` wrapper no longer acts as positioning context on mobile, which is correct since `fixed` positions relative to viewport.

**Dismiss handler fix:** The `Dropdown` component uses `document.addEventListener('mousedown', handler)` to close on outside click. On mobile, taps on non-interactive areas don't reliably fire `mousedown` — they fire `touchstart`. Replace `mousedown` with `pointerdown` (fires for both mouse and touch):
```typescript
// In Dropdown useEffect, replace:
document.addEventListener('mousedown', handler);
return () => document.removeEventListener('mousedown', handler);
// With:
document.addEventListener('pointerdown', handler);
return () => document.removeEventListener('pointerdown', handler);
```

**Width budget (mobile top bar):**
Hamburger is external (`Sidebar.tsx`), so the top bar flex row contains:
- Diamond + "Chat" + status dot: ~80px
- Gap + flex spacer
- 4 buttons × (32px icon + 3px padding × 2) = ~152px
- Gaps: 3 × 6px = 18px
- Total: ~250px — fits in 320px with 70px spare

**File:** `web/src/components/ChatTopBar.tsx`
**File:** `web/src/components/TopBar.tsx` (no changes needed)

### 2. Navigation Panels

#### Left Panel (Products) — Existing Sidebar
- **Trigger:** Existing hamburger button (already `fixed top-3 left-3 z-50` on mobile) OR swipe from left screen edge
- **Content:** Existing `Sidebar.tsx` — products grouped by category, unchanged
- **Width fix:** `w-[260px]` → `w-[min(85vw,280px)]`
- **New: Swipe gesture** — detect touch starting within 20px of left screen edge, require 50px horizontal movement. Only active on mobile (< 640px).

#### Right Panel (Session History) — New Mobile Overlay Mode
- **Trigger:** History button tap OR swipe from right screen edge
- **Content:** Existing `SessionsPanel.tsx` content (search, filters, session list)

**Structural change required for `SessionsPanel`:**
Currently it's a flex-row sibling using `style={{ width: open ? 220 : 0 }}` with CSS width transition. On mobile, this needs to become a `fixed` overlay:

```tsx
// In SessionsPanel, add mobile detection:
const isMobile = window.innerWidth < 640; // or use a useMediaQuery hook

// Desktop: current behavior (flex sibling with width transition)
// Mobile: fixed right-0 top-0 bottom-0 w-[min(85vw,300px)] z-40 + backdrop
```

**New props needed in `ChatPage.tsx`:**
- None — `sessionsOpen` and `onToggleSessions` already exist and control the panel. The structural change is internal to `SessionsPanel.tsx` (conditionally render as fixed overlay vs flex sibling based on viewport width).

**Backdrop:** Semi-transparent `bg-black/50 fixed inset-0 z-30` behind the panel, tap to close. Same pattern as existing `Sidebar.tsx` mobile overlay.

**Swipe gesture:** Detect touch starting within 20px of right screen edge, require 50px horizontal movement leftward.

**Swipe conflict resolution:**
- Only detect swipe from screen edges (first 20px). This avoids conflicts with `MessageList` scroll and `ChatInput` textarea.
- Set `touch-action: pan-y` only on the edge-detection zones, not on the full document.
- If touch starts inside `textarea`, `select`, or any element with `data-no-swipe`, ignore it.
- Gesture requires > 50px horizontal and < 30px vertical movement to activate (angle threshold).

**File:** `web/src/components/Sidebar.tsx` — responsive width, left-edge swipe
**File:** `web/src/components/SessionsPanel.tsx` — conditional fixed overlay on mobile, right-edge swipe
**File:** `web/src/pages/ChatPage.tsx` — swipe gesture hook (shared between both panels)

### 3. ChatInput Toolbar

**Layout (both desktop and mobile):** Textarea + Send on top row. Toolbar below. Same spatial order.

**Keyboard behavior (mobile):** The ChatInput container should stay docked above the soft keyboard when it opens, similar to how Gemini and iMessage work. Implementation:
- Use `visualViewport` API: listen to `visualViewport.resize` and `visualViewport.scroll` events.
- When keyboard opens, `visualViewport.height` shrinks. Calculate the offset: `window.innerHeight - visualViewport.height - visualViewport.offsetTop`.
- Apply `transform: translateY(-${offset}px)` to the ChatInput container OR use `position: fixed; bottom: 0` with the viewport height.
- Alternative (simpler): ensure the `ChatInput` wrapper uses `position: sticky; bottom: 0` inside a flex column with `overflow: auto` on the message list — this is the current layout and should work if the parent container uses `dvh` units.
- Fallback: add `<meta name="interactive-widget" content="resizes-content">` to `index.html` — this tells the browser to resize the layout viewport when the keyboard opens (Chrome 108+, Safari 15+). With this, `100dvh` naturally excludes the keyboard and the flex layout pushes ChatInput up.

**Recommended approach:** Add `interactive-widget=resizes-content` meta tag + ensure layout uses `min-h-dvh` or `h-dvh`. This is the zero-JS solution that matches Gemini's behavior. Test on iOS Safari and Chrome Android.

**Mobile toolbar items (< 640px):**

| Item | Desktop | Mobile | Width |
|------|---------|--------|-------|
| Product | Name badge ("General") | Same — name text | ~64px |
| Chat mode | 4-button slider (Chat/Code/Architect/Brainstorm) | **Dropdown** with chevron showing current mode | ~75px |
| Model | `<select>` element (already shows full name natively) | Same `<select>` — native mobile picker | ~68px |
| Think | SVG + "Think · Auto" text | **Icon-only** (lightbulb SVG) | 32px |
| Attach | SVG + "Attach" text | **Icon-only** (paperclip SVG) | 32px |
| Camera | Separate button | **Hidden** (merged into attach) | 0px |

**Total width:** ~271px fits in 304px available (320px minus 16px container padding).

**ThinkingToggle text split:** Currently renders `<span>Think · {current.label}</span>` as one element. Must split into:
```tsx
<svg .../>
<span className="hidden sm:inline">Think ·</span>
<span>{current.label}</span>
```
On mobile: shows only "Auto" / "Max" / "Off" next to the icon. On desktop: full "Think · Auto".

**Chat mode dropdown:** On mobile (< 640px), replace the 4-button slider with a dropdown:
```tsx
{/* Desktop: button group */}
<div className="hidden sm:flex ...">
  {CHAT_MODES.map(m => <button ...>{m.label}</button>)}
</div>
{/* Mobile: dropdown */}
<div className="sm:hidden relative">
  <button onClick={toggleModeMenu} className="...">
    {currentMode.label} <ChevronDown />
  </button>
  {modeMenuOpen && (
    <div className="absolute bottom-full mb-1 ...">
      {CHAT_MODES.map(m => <button onClick={() => { setMode(m.id); closeModeMenu(); }} ...>{m.label}</button>)}
    </div>
  )}
</div>
```
Note: dropdown opens **upward** (bottom-full) since the toolbar is at the bottom of the screen.

**Send button:** `w-8 h-8` → `w-10 h-10 sm:w-8 sm:h-8`. 40px touch target on mobile.

**Safe area:** Apply `.safe-bottom` class (already exists in `app.css`) to the ChatInput outermost container. Do NOT add `env()` inline — use the existing utility class. No other parent should also have `.safe-bottom` to avoid double-padding.

**data-testid additions:**
- `data-testid="chat-mode-dropdown"` on the mobile dropdown button
- `data-testid="chat-mode-option-{id}"` on each dropdown option

**File:** `web/src/components/ChatInput.tsx`
**File:** `web/src/components/ThinkingToggle.tsx`

### 4. Message Bubbles

| Property | Desktop | Mobile (< 640px) |
|----------|---------|-------------------|
| max-width | 85% | **92%** |
| Padding | `px-4 py-3` (16px 12px) | **`px-3 py-2.5`** (12px 10px) |
| Body font | inherited (16px) | **13px** |
| Metadata font | 9px | **10px** |
| Token display | 9px | **10px** |
| Message gap (MessageList inner div) | 16px (`gap-4`) | **12px** (`gap-3`) |

**Both `max-w-[85%]` occurrences must be updated:**
1. Line ~278 (main message bubble): `max-w-[85%]` → `max-w-[92%] sm:max-w-[85%]`
2. Line ~220 (legacy tool_use path): `max-w-[85%]` → `max-w-[92%] sm:max-w-[85%]`, and `ml-[34px]` → `ml-4 sm:ml-[34px]` (tighter left margin on mobile)

**MessageList gap:** The `gap-4` class is on the inner `<div className="flex flex-col gap-4">` (not the outer scrollable container). Change to `gap-3 sm:gap-4`.

**MessageList suggestions grid:** `grid-cols-2` → `grid-cols-1 sm:grid-cols-2` for single-column on narrow screens.

**data-testid:** No new elements, existing testids sufficient.

**File:** `web/src/components/MessageBubble.tsx`
**File:** `web/src/components/MessageList.tsx`

### 5. ThinkingBlock

| Property | Desktop | Mobile (< 640px) |
|----------|---------|-------------------|
| max-width (expanded) | 380px | **full width** |
| Content font | 11px | **12px** |
| Header font | 11px | **12px** |

**File:** `web/src/components/ThinkingBlock.tsx`
- Expanded box: `max-w-[380px]` → `max-w-full sm:max-w-[380px]`
- Content pre: `text-[11px]` → `text-xs sm:text-[11px]`
- Header: `text-[11px]` → `text-xs sm:text-[11px]`

### 6. ToolCallBlock

| Property | Desktop | Mobile (< 640px) |
|----------|---------|-------------------|
| Pill button height | 28px (`h-7`) | **32px** (`h-8`) |
| Output font | 11px | **12px** |
| Tool name | full | truncate |

**File:** `web/src/components/ToolCallBlock.tsx`
- Pill button: `h-7` → `h-8 sm:h-7`
- Output: `text-[11px]` → `text-xs sm:text-[11px]`
- Tool name `<span>`: add `truncate max-w-[120px] sm:max-w-none` (class doesn't exist yet, this adds it)

### 7. Global CSS & Viewport

**File:** `web/src/app.css`
- The existing `.safe-bottom` rule (line 201) is already unconditional and correct. Do NOT redefine it inside a media query.
- Add only the new utilities that don't already exist:
```css
.safe-top { padding-top: env(safe-area-inset-top, 0px); }
.safe-x {
  padding-left: env(safe-area-inset-left, 0px);
  padding-right: env(safe-area-inset-right, 0px);
}
```
These are unconditional (no media query needed — `env()` returns `0px` on non-notch devices).

**File:** `web/index.html`
- Current: `<meta name="viewport" content="width=device-width, initial-scale=1.0">`
- Change to: `<meta name="viewport" content="width=device-width, initial-scale=1.0, viewport-fit=cover, interactive-widget=resizes-content">`
- `viewport-fit=cover` enables `env(safe-area-inset-*)` values
- `interactive-widget=resizes-content` makes the browser resize the layout viewport when the soft keyboard opens — this is the key to making ChatInput stay above the keyboard without any JS

### 8. Sidebar Width Fix

**File:** `web/src/components/Sidebar.tsx`
- Mobile overlay: `w-[260px]` → `w-[min(85vw,280px)]`

### 9. Keyboard-Aware ChatInput (Mobile)

**Goal:** ChatInput stays docked at the bottom of the visible area and moves up when the soft keyboard opens, identical to Gemini/iMessage behavior.

**Primary approach (zero-JS):** The `interactive-widget=resizes-content` meta tag (Section 7) tells the browser to resize the layout viewport when the keyboard opens. Combined with the existing flex layout (`flex-col` with `MessageList` as `flex-1 overflow-y-auto` and `ChatInput` at the bottom), the ChatInput naturally stays above the keyboard.

**CSS requirement:** Ensure the outermost chat container uses `h-dvh` (dynamic viewport height) instead of `h-screen` or `100vh`. `dvh` accounts for browser chrome and keyboard.

**Fallback (if `interactive-widget` not supported):** Add a `useVisualViewport` hook:
```typescript
function useVisualViewport() {
  const [keyboardHeight, setKeyboardHeight] = useState(0);
  useEffect(() => {
    const vv = window.visualViewport;
    if (!vv) return;
    const onResize = () => {
      const offset = window.innerHeight - vv.height;
      setKeyboardHeight(offset > 50 ? offset : 0); // threshold to ignore small changes
    };
    vv.addEventListener('resize', onResize);
    return () => vv.removeEventListener('resize', onResize);
  }, []);
  return keyboardHeight;
}
```
Apply `style={{ paddingBottom: keyboardHeight }}` to the chat page container when `keyboardHeight > 0`. Only apply on mobile (check `'ontouchstart' in window`).

**File:** `web/src/pages/ChatPage.tsx` — use `h-dvh`, add fallback hook if needed
**File:** `web/index.html` — meta tag (covered in Section 7)

## Breakpoint Strategy

| Range | Label | Behavior |
|-------|-------|----------|
| < 640px | Mobile | All mobile optimizations active |
| 640px–767px | Tablet | Icon-only top bar, full sidebar visible |
| ≥ 768px | Desktop | Current layout unchanged |

Uses Tailwind's `sm:` (640px) and `md:` (768px) breakpoints. Mobile styles are default, desktop styles use `sm:` or `md:` prefix. New `max-sm:` variant used for mobile-only overrides where the default is the desktop style.

## Files Modified (Complete List)

| File | Changes |
|------|---------|
| `web/index.html` | `viewport-fit=cover`, `interactive-widget=resizes-content` |
| `web/src/app.css` | Add `.safe-top`, `.safe-x` utilities (`.safe-bottom` already exists) |
| `web/src/components/ChatTopBar.tsx` | Icon-only buttons on mobile (`hidden sm:inline` on text), dropdown positioning fix (`max-sm:fixed`) |
| `web/src/components/ChatInput.tsx` | Chat mode dropdown (4 modes), send 40px, `.safe-bottom`, camera hidden on mobile |
| `web/src/components/ThinkingToggle.tsx` | Split text span for responsive hiding |
| `web/src/components/MessageBubble.tsx` | Both `max-w-[85%]` → responsive, tighter padding, bumped metadata font |
| `web/src/components/ThinkingBlock.tsx` | `max-w-full sm:max-w-[380px]`, font bump |
| `web/src/components/ToolCallBlock.tsx` | Taller expand (`h-8`), font bump, tool name truncate |
| `web/src/components/MessageList.tsx` | `gap-3 sm:gap-4` on inner div, single-col suggestion grid on mobile |
| `web/src/components/Sidebar.tsx` | `w-[min(85vw,280px)]`, left-edge swipe gesture |
| `web/src/components/SessionsPanel.tsx` | Conditional fixed overlay on mobile, right-edge swipe gesture, backdrop |
| `web/src/pages/ChatPage.tsx` | `h-dvh`, swipe gesture hook, `useVisualViewport` fallback |

## Not In Scope

- Non-chat pages (Tasks, Tutor, Scout, etc.) — separate effort
- Backend changes — none needed
- New components — all changes are responsive variants on existing components
- Dark/light mode — already dark-only
- Tablet-specific layout — inherits from mobile or desktop depending on breakpoint
- Adding session title to top bar — stays as static "Chat" title
