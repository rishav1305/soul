# Chat Window Polish — Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Polish the chat window — golden branding, SVG-only icons, better message bubbles, collapsible thinking/tool states, "General" product default, labeled toolbar buttons.

**Architecture:** Incremental restyling of existing React components. No new state management or backend changes. Each task modifies one component file and is independently deployable.

**Tech Stack:** React 19, Tailwind CSS v4, TypeScript, SVG icons

**Spec:** `docs/superpowers/specs/2026-03-16-ui-redesign-design.md` (Part 3-6: chat window only)

**Existing design tokens (app.css):** `--color-soul: #e8a849` (golden), `--color-deep: #08080d`, `--color-surface: #111118`, `--color-elevated: #1a1a24`

---

## File Structure

| File | Responsibility | Action |
|------|---------------|--------|
| `web/src/app.css` | Add cursor blink keyframe, thinking border token | Modify |
| `web/src/components/ThinkingBlock.tsx` | Golden lightbulb icon, expanded/collapsed states | Rewrite |
| `web/src/components/ToolCallBlock.tsx` | Replace emoji icons with SVG, compact styling | Modify |
| `web/src/components/MessageBubble.tsx` | New bubble styles, golden diamond, no "Soul" text, metadata layout | Modify |
| `web/src/components/ChatInput.tsx` | "General" default, labeled buttons, unified container, SVG icons | Modify |

---

## Task 1: CSS Tokens and Keyframes

**Files:**
- Modify: `web/src/app.css`

- [ ] **Step 1: Add cursor blink keyframe and thinking border token**

Add after the existing `diamond-breathe` keyframe block in `app.css`:

```css
@keyframes cursor-blink {
  0%, 100% { opacity: 1; }
  50% { opacity: 0; }
}

.streaming-cursor {
  display: inline-block;
  width: 2px;
  height: 1em;
  background: var(--color-soul);
  border-radius: 1px;
  margin-left: 1px;
  vertical-align: text-bottom;
  animation: cursor-blink 1s step-end infinite;
}
```

- [ ] **Step 2: Verify build**

```bash
cd /home/rishav/soul-v2/web && npx vite build 2>&1 | tail -3
```
Expected: build succeeds

- [ ] **Step 3: Commit**

```bash
git add web/src/app.css
git commit -m "feat: add streaming cursor keyframe and CSS class"
```

---

## Task 2: ThinkingBlock — Golden Lightbulb States

**Files:**
- Rewrite: `web/src/components/ThinkingBlock.tsx`

- [ ] **Step 1: Rewrite ThinkingBlock with two states**

Replace the entire file with:

```tsx
import { useState } from 'react';

interface ThinkingBlockProps {
  content: string;
  isStreaming: boolean;
}

function LightbulbIcon({ streaming }: { streaming: boolean }) {
  return (
    <svg
      width="12" height="12" viewBox="0 0 16 16" fill="none"
      stroke="currentColor" strokeWidth="1.5" strokeLinecap="round"
      className={streaming ? 'text-soul drop-shadow-[0_0_3px_var(--color-soul-glow)]' : 'text-fg-muted'}
    >
      <path d="M8 2a4.5 4.5 0 0 1 2.5 8.2V12h-5v-1.8A4.5 4.5 0 0 1 8 2z" />
      <path d="M6 14h4" />
    </svg>
  );
}

export function ThinkingBlock({ content, isStreaming }: ThinkingBlockProps) {
  const [expanded, setExpanded] = useState(isStreaming);

  // Collapsed pill (completed thinking)
  if (!isStreaming && !expanded) {
    const duration = content.length > 500 ? `${(content.length / 200).toFixed(1)}s` : `${content.split('\n').length} lines`;
    return (
      <div className="mb-2">
        <button
          type="button"
          onClick={() => setExpanded(true)}
          data-testid="thinking-toggle"
          className="flex items-center gap-1.5 text-[11px] text-fg-muted hover:text-fg-secondary transition-colors cursor-pointer bg-surface border border-border-subtle rounded-md px-2.5 py-1"
        >
          <LightbulbIcon streaming={false} />
          <span className="font-mono">Extended thinking</span>
          <span className="text-fg-muted">·</span>
          <span className="font-mono">{duration}</span>
          <svg width="8" height="8" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round">
            <path d="M4 6l4 4 4-4" />
          </svg>
        </button>
      </div>
    );
  }

  // Expanded (streaming or user expanded)
  return (
    <div className="mb-2">
      <div className={`border rounded-lg p-3 max-w-[380px] ${
        isStreaming ? 'border-soul/25 bg-surface' : 'border-border-subtle bg-surface'
      }`}>
        <button
          type="button"
          onClick={() => !isStreaming && setExpanded(false)}
          data-testid="thinking-toggle"
          className="flex items-center gap-1.5 w-full text-left cursor-pointer mb-1.5"
        >
          <LightbulbIcon streaming={isStreaming} />
          <span className={`text-[11px] font-mono font-semibold tracking-wide ${isStreaming ? 'text-soul' : 'text-fg-muted'}`}>
            Extended thinking{isStreaming ? '...' : ''}
          </span>
          <div className="flex-1" />
          {!isStreaming && (
            <svg width="8" height="8" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" className="text-fg-muted">
              <path d="M12 10l-4-4-4 4" />
            </svg>
          )}
        </button>
        <div className="border-l-2 border-soul/20 pl-2 max-h-48 overflow-y-auto">
          <pre className="text-[11px] text-fg-muted font-mono whitespace-pre-wrap leading-relaxed">
            {content}
            {isStreaming && <span className="inline-block w-1 h-1 rounded-full bg-soul ml-1 align-middle animate-pulse" />}
          </pre>
        </div>
      </div>
    </div>
  );
}
```

- [ ] **Step 2: Verify TypeScript**

```bash
cd /home/rishav/soul-v2/web && npx tsc --noEmit 2>&1
```
Expected: no errors

- [ ] **Step 3: Build**

```bash
npx vite build 2>&1 | tail -3
```
Expected: build succeeds

- [ ] **Step 4: Commit**

```bash
git add web/src/components/ThinkingBlock.tsx
git commit -m "feat: restyle ThinkingBlock — golden lightbulb, collapsed/expanded states"
```

---

## Task 3: ToolCallBlock — SVG Icons, No Emoji

**Files:**
- Modify: `web/src/components/ToolCallBlock.tsx`

- [ ] **Step 1: Replace TOOL_ICONS emoji map with SVG function**

Replace lines 5-21 (the TOOL_ICONS record and toolIcon function) with:

```tsx
function ToolIcon({ name, className }: { name: string; className?: string }) {
  const cls = className ?? 'text-fg-muted';
  const props = { width: 10, height: 10, viewBox: '0 0 16 16', fill: 'none', stroke: 'currentColor', strokeWidth: '1.5', strokeLinecap: 'round' as const, strokeLinejoin: 'round' as const, className: cls };

  switch (name) {
    case 'code_read':
      return <svg {...props}><path d="M3 2h7l3 3v9H3z" /><path d="M10 2v3h3" /></svg>;
    case 'code_write':
    case 'code_edit':
      return <svg {...props}><path d="M11.5 1.5l3 3L5 14H2v-3z" /></svg>;
    case 'code_search':
    case 'code_grep':
      return <svg {...props}><circle cx="7" cy="7" r="4.5" /><path d="M10.5 10.5L14 14" /></svg>;
    case 'code_exec':
      return <svg {...props}><path d="M4 4l4 4-4 4" /><path d="M10 12h4" /></svg>;
    case 'task_update':
    case 'task_create':
      return <svg {...props}><rect x="2" y="2" width="12" height="12" rx="2" /><path d="M5 8h6M8 5v6" /></svg>;
    case 'e2e_assert':
    case 'e2e_dom':
      return <svg {...props}><path d="M3 8l3 3 7-7" /></svg>;
    case 'e2e_screenshot':
      return <svg {...props}><rect x="2" y="3" width="12" height="10" rx="1" /><circle cx="8" cy="8" r="2" /></svg>;
    default:
      return <svg {...props}><circle cx="8" cy="8" r="5.5" /><path d="M8 5v3l2 1" /></svg>;
  }
}
```

- [ ] **Step 2: Update pillContent to use ToolIcon component**

Replace line 119 (`<span className="shrink-0">{icon}</span>`) with:

```tsx
<ToolIcon name={tool.name} className={`shrink-0 ${statusColor}`} />
```

And remove the unused `icon` variable from the component body (line 90: `const icon = toolIcon(tool.name);`).

- [ ] **Step 3: Update the running status icon to SVG spinner**

Replace line 87-88:
```tsx
const statusIcon = isRunning ? '\u25CC' : isError ? '\u2717' : '\u2713';
const statusColor = isRunning ? 'text-fg-muted' : isError ? 'text-red-500' : 'text-green-500';
```

With:
```tsx
const statusColor = isRunning ? 'text-soul' : isError ? 'text-red-500' : 'text-green-500';
```

And replace the status icon span (line 108) with an SVG:
```tsx
{isRunning ? (
  <svg width="10" height="10" viewBox="0 0 16 16" fill="none" className="shrink-0 animate-spin">
    <circle cx="8" cy="8" r="5.5" stroke="currentColor" strokeWidth="1.5" strokeDasharray="7 4" className="text-soul" />
  </svg>
) : isError ? (
  <svg width="10" height="10" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" className="shrink-0 text-red-500">
    <path d="M4 4l8 8M12 4l-8 8" />
  </svg>
) : (
  <svg width="10" height="10" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" className="shrink-0 text-green-500">
    <path d="M3 8l3 3 7-7" />
  </svg>
)}
```

- [ ] **Step 4: Verify TypeScript + build**

```bash
cd /home/rishav/soul-v2/web && npx tsc --noEmit && npx vite build 2>&1 | tail -3
```
Expected: no errors, build succeeds

- [ ] **Step 5: Commit**

```bash
git add web/src/components/ToolCallBlock.tsx
git commit -m "feat: replace emoji tool icons with SVG, add spinner for running state"
```

---

## Task 4: MessageBubble — New Styling

**Files:**
- Modify: `web/src/components/MessageBubble.tsx`

- [ ] **Step 1: Replace user avatar with no-avatar right-aligned bubble**

In the return block (line 244-333), replace the entire `<div data-testid="message-bubble" ...>` with the new structure. Key changes:

**a)** Remove the avatar column for user messages — right alignment is sufficient context.

**b)** For assistant messages, replace the round avatar with a golden diamond SVG header row:

```tsx
{/* Assistant header: diamond + model + tokens */}
{!isUser && (
  <div className="flex items-center gap-1.5 mb-1">
    <svg width="12" height="12" viewBox="0 0 16 16" fill="none"
      className={isStreaming ? 'drop-shadow-[0_0_6px_var(--color-soul-glow)]' : ''}>
      <defs>
        <linearGradient id="diamond-gold" x1="2" y1="0" x2="14" y2="16">
          <stop offset="0%" stopColor="#f0c040" />
          <stop offset="50%" stopColor="#d4a018" />
          <stop offset="100%" stopColor="#b8860b" />
        </linearGradient>
      </defs>
      <path d="M8 0L14 8L8 16L2 8Z" fill="url(#diamond-gold)">
        {isStreaming && <animate attributeName="opacity" values="0.7;1;0.7" dur="2s" repeatCount="indefinite" />}
      </path>
    </svg>
    {badge && <span className="text-[9px] font-mono text-fg-muted">{badge}</span>}
    {message.usage && (
      <>
        <span className="text-[9px] text-fg-muted/50">·</span>
        <span className="text-[9px] font-mono text-fg-muted/70">
          {formatTokens(message.usage.inputTokens)} → {formatTokens(message.usage.outputTokens)}
        </span>
      </>
    )}
  </div>
)}
```

**c)** Update bubble classes:

User bubble:
```
bg-[#12123a] border border-[#1e1e50] rounded-[16px_16px_4px_16px] px-4 py-3
```

Assistant bubble:
```
bg-surface border border-border-subtle rounded-[4px_14px_14px_14px] px-4 py-3
```

**d)** Replace the streaming indicator text "Soul is thinking..." with just the animated dot + word count:

```tsx
{isStreaming && !isUser && message.content && (
  <div className="flex items-center gap-2 mt-1.5">
    <span className="streaming-cursor" />
    <span className="text-[10px] font-mono text-fg-muted">
      {message.content.split(/\s+/).filter(Boolean).length} words
    </span>
  </div>
)}
```

**e)** Move token count from metadata footer to the assistant header row (done in step b), remove duplicate from footer.

- [ ] **Step 2: Verify TypeScript + build**

```bash
cd /home/rishav/soul-v2/web && npx tsc --noEmit && npx vite build 2>&1 | tail -3
```

- [ ] **Step 3: Commit**

```bash
git add web/src/components/MessageBubble.tsx
git commit -m "feat: restyle message bubbles — golden diamond, no avatar, new contrast"
```

---

## Task 5: ChatInput — General Default, Labeled Buttons, SVG Icons

**Files:**
- Modify: `web/src/components/ChatInput.tsx`

- [ ] **Step 1: Change "None (general)" to "General"**

Replace line 456 (the None option button text):
```tsx
<span>None (general)</span>
```
With:
```tsx
<span>General</span>
```

- [ ] **Step 2: Replace emoji in PRODUCTS with SVG approach**

This is a larger change. For Phase 1, keep the emoji in the PRODUCTS array but use them only in the dropdown. The toolbar button itself uses SVG:

Replace the product selector button icon (lines 431-436, the SVG with filter lines) — keep as is (it's already SVG). Just update the badge to not show emoji:

```tsx
{activeProduct && (
  <span data-testid="product-badge" className="text-xs font-mono text-soul">
    {PRODUCTS.find(p => p.id === activeProduct)?.name ?? activeProduct}
  </span>
)}
```

And when no product is active, show "General":
```tsx
{!activeProduct && (
  <span className="text-xs text-fg-muted">General</span>
)}
```

- [ ] **Step 3: Add labels to thinking toggle and attach button**

Update the thinking toggle button to include "Think" label:

```tsx
<ThinkingToggle value={thinkingType} onChange={setThinkingType} label="Think" />
```

(If ThinkingToggle doesn't accept a label prop, add the label adjacent to it.)

Update the attach button to include "Attach" label:
```tsx
<span className="text-xs text-fg-secondary">Attach</span>
```

- [ ] **Step 4: Verify TypeScript + build**

```bash
cd /home/rishav/soul-v2/web && npx tsc --noEmit && npx vite build 2>&1 | tail -3
```

- [ ] **Step 5: Commit**

```bash
git add web/src/components/ChatInput.tsx
git commit -m "feat: ChatInput — General default, labeled buttons, SVG icons"
```

---

## Task 6: Build, Deploy, Verify

**Files:** None (deploy only)

- [ ] **Step 1: Full frontend build**

```bash
cd /home/rishav/soul-v2/web && npx vite build 2>&1 | tail -5
```
Expected: build succeeds

- [ ] **Step 2: Deploy**

```bash
cd /home/rishav/soul-v2
sudo kill $(pgrep soul-chat) 2>/dev/null
sleep 1
sudo systemctl start soul-v2
sleep 2
ss -tlnp | grep 3002
```
Expected: `*:3002`

- [ ] **Step 3: Visual verification**

Open `http://localhost:3002/chat` and verify:
- [ ] Golden diamond logo (not "S", not purple) appears above assistant messages
- [ ] No "Soul" text label in chat area
- [ ] User messages are right-aligned with `#12123a` background, no avatar
- [ ] Assistant messages have `bg-surface` with border, rounded corners
- [ ] Thinking block shows golden lightbulb icon, collapses to pill when done
- [ ] Tool calls use SVG icons (no emoji), running shows amber spinner
- [ ] Streaming shows blinking golden cursor + word count
- [ ] ChatInput toolbar shows "General" when no product selected
- [ ] All toolbar buttons have text labels (Think, Attach)
- [ ] Code blocks are darker (`#080816` region) with copy icon

- [ ] **Step 4: Commit deploy**

```bash
git add -A
git commit -m "chore: build chat window polish"
```
