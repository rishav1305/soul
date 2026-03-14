# Chat Feature Parity (v1 → v2) Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Bring Soul v2 chat to full feature parity with Soul v1, adding ~30 missing features incrementally while satisfying all 5 Pillars (performant, robust, resilient, secure, sovereign).

**Architecture:** Each phase adds one layer of capability. Every new component is a standalone file under `web/src/components/`. No refactoring existing components until the new code is proven. Each task builds on the previous — the app is functional after every commit.

**Tech Stack:** React 19, TypeScript 5.9 (strict), Tailwind CSS v4, Vite 7, react-syntax-highlighter (PrismLight), mermaid (lazy).

**Bundle Budget:** Currently 119KB gzipped. Pillar target < 300KB. Reserve 50KB for future features. Hard ceiling for this work: 250KB gzipped.

**Reference:** Soul v1 source at `/home/rishav/soul/web/src/` — adapt patterns, don't copy-paste (v2 has different component structure).

---

## Pillar Compliance Checklist (apply to EVERY task)

| Pillar | Check Before Commit |
|--------|---------------------|
| **Performant** | `cd web && npx vite build` — verify gzip totals < 250KB. No unnecessary re-renders (useCallback/useMemo where needed). |
| **Robust** | `cd web && npx tsc --noEmit` — zero errors. Every component handles: empty/null content, extremely long content, missing optional fields. |
| **Resilient** | New features degrade gracefully — if data is missing, component renders nothing or a sensible fallback. No crashes on unexpected WS message shapes. |
| **Secure** | No `dangerouslySetInnerHTML` (exception: mermaid SVG with sanitization). Copy uses `navigator.clipboard` with fallback. No raw user HTML rendering. |
| **Sovereign** | Zero CDN imports. All assets bundled. No external fonts/analytics/telemetry. `npx vite build` produces self-contained dist/. |

**Verification after every task:**
```bash
cd /home/rishav/soul-v2/web && npx tsc --noEmit && npx vite build 2>&1 | tail -8
```

---

## Phase 1: Core Message Quality (zero new deps)

### Task 1: Enter-to-send

The most common chat convention. Currently v2 uses Cmd/Ctrl+Enter. Change to Enter=send, Shift+Enter=newline.

**Files:**
- Modify: `web/src/components/ChatInput.tsx`

**Step 1: Change keyboard handler**

In `ChatInput.tsx`, replace the `onKeyDown` handler:

```tsx
const handleKeyDown = (e: React.KeyboardEvent<HTMLTextAreaElement>) => {
  if (e.key === 'Enter' && !e.shiftKey) {
    e.preventDefault();
    handleSend();
  }
};
```

Remove any Cmd/Ctrl+Enter logic. Shift+Enter is default textarea behavior (newline) — no code needed.

**Step 2: Update placeholder text**

Change the placeholder from "Type a message... (Cmd+Enter to send)" to:
```
"Message Soul... (Shift+Enter for new line)"
```

**Step 3: Verify**
```bash
cd /home/rishav/soul-v2/web && npx tsc --noEmit && npx vite build 2>&1 | tail -3
```

**Step 4: Commit**
```bash
git add web/src/components/ChatInput.tsx
git commit -m "feat(chat): enter-to-send, shift+enter for newline"
```

**Pillar notes:** No bundle impact. Robust: handles empty textarea (existing trim check). Resilient: N/A.

---

### Task 2: Stop Generation Button

Users need to cancel long streams. Add a stop button that replaces the send button during streaming.

**Files:**
- Modify: `web/src/components/ChatInput.tsx`
- Modify: `web/src/hooks/useChat.ts`
- Modify: `web/src/hooks/useWebSocket.ts`
- Modify: `web/src/lib/types.ts` (add `chat.stop` inbound type)

**Step 1: Add `chat.stop` to types**

In `types.ts`, add to `InboundMessageType`:
```typescript
type InboundMessageType = 'chat.send' | 'chat.stop' | 'session.switch' | 'session.create' | 'session.delete';
```

**Step 2: Add stopGeneration to useChat**

In `useChat.ts`, add:
```typescript
const stopGeneration = useCallback(() => {
  sendRef.current('chat.stop', {});
  setIsStreaming(false);
}, []);
```

Return `stopGeneration` from the hook.

**Step 3: Wire stop button in ChatInput**

Add `onStop` and `isStreaming` to ChatInput props:
```typescript
interface ChatInputProps {
  onSend: (content: string) => void;
  onStop: () => void;
  disabled: boolean;
  isStreaming: boolean;
}
```

Conditionally render stop button when streaming:
```tsx
{isStreaming ? (
  <button
    onClick={onStop}
    data-testid="stop-button"
    className="px-5 py-3 text-base shrink-0 bg-red-600 hover:bg-red-500 text-white rounded-lg font-medium transition-colors"
  >
    Stop
  </button>
) : (
  <button ... > {/* existing send button */} </button>
)}
```

**Step 4: Add Escape key handler**

In ChatInput's `handleKeyDown`:
```tsx
if (e.key === 'Escape' && isStreaming) {
  onStop();
  return;
}
```

**Step 5: Pass props from Shell**

In `Shell.tsx`, pass `onStop={stopGeneration}` and `isStreaming` to ChatInput.

**Step 6: Handle chat.stop server-side**

Check if `internal/ws/handler.go` handles `chat.stop`. If not, add handling — the server should cancel the in-flight stream context. (Reference: v1 uses context cancellation.)

**Step 7: Verify + commit**
```bash
cd /home/rishav/soul-v2/web && npx tsc --noEmit && npx vite build 2>&1 | tail -3
git add web/src/
git commit -m "feat(chat): stop generation button and escape key"
```

**Pillar notes:** Robust: button only shows during streaming. Resilient: if stop fails, streaming timeout still fires.

---

### Task 3: Thinking Blocks

Claude Opus returns thinking content. Display it as expandable/collapsible blocks.

**Files:**
- Create: `web/src/components/ThinkingBlock.tsx`
- Modify: `web/src/components/MessageBubble.tsx`
- Modify: `web/src/hooks/useChat.ts` (handle `chat.thinking` events)
- Modify: `web/src/lib/types.ts` (add thinking to Message, add `chat.thinking` outbound type)

**Step 1: Extend Message type**

In `types.ts`:
```typescript
interface Message {
  id: string;
  role: 'user' | 'assistant' | 'tool_use' | 'tool_result';
  content: string;
  thinking?: string;  // NEW — accumulated thinking content
  sessionID: string;
  createdAt: string;
}
```

Add `chat.thinking` to `OutboundMessageType`.

**Step 2: Handle chat.thinking in useChat**

In the `onMessage` handler, add case for `chat.thinking`:
```typescript
case 'chat.thinking': {
  const text = (data as { text?: string }).text ?? '';
  setMessages(prev => {
    const last = prev[prev.length - 1];
    if (last?.id === STREAMING_MESSAGE_ID) {
      return [...prev.slice(0, -1), { ...last, thinking: (last.thinking ?? '') + text }];
    }
    // New streaming message with thinking
    return [...prev, {
      id: STREAMING_MESSAGE_ID,
      role: 'assistant',
      content: '',
      thinking: text,
      sessionID: sessionIDRef.current ?? '',
      createdAt: new Date().toISOString(),
    }];
  });
  break;
}
```

**Step 3: Create ThinkingBlock component**

```tsx
// web/src/components/ThinkingBlock.tsx
import { useState } from 'react';

interface ThinkingBlockProps {
  content: string;
  isStreaming: boolean;
}

export function ThinkingBlock({ content, isStreaming }: ThinkingBlockProps) {
  const [expanded, setExpanded] = useState(false);
  const lineCount = content.split('\n').length;

  return (
    <div className="mb-2 border border-zinc-700 rounded-lg overflow-hidden">
      <button
        onClick={() => setExpanded(!expanded)}
        className="w-full flex items-center gap-2 px-3 py-2 text-sm text-zinc-400 hover:bg-zinc-800/50 transition-colors"
        data-testid="thinking-toggle"
      >
        <span className="text-xs">{expanded ? '▼' : '▶'}</span>
        <span>Thinking{isStreaming ? '...' : ''} ({lineCount} lines)</span>
      </button>
      {expanded && (
        <div className="px-3 py-2 text-sm text-zinc-500 max-h-48 overflow-y-auto border-t border-zinc-700 whitespace-pre-wrap">
          {content}
        </div>
      )}
    </div>
  );
}
```

**Step 4: Render in MessageBubble**

In `MessageBubble.tsx`, before the markdown content for assistant messages:
```tsx
{message.thinking && (
  <ThinkingBlock content={message.thinking} isStreaming={isStreaming && !message.content} />
)}
```

**Step 5: Verify + commit**
```bash
cd /home/rishav/soul-v2/web && npx tsc --noEmit && npx vite build 2>&1 | tail -3
git add web/src/
git commit -m "feat(chat): expandable thinking blocks for Opus"
```

**Pillar notes:** Robust: handles empty thinking, very long thinking (max-h-48 with scroll). Performant: collapsed by default, no re-render overhead.

---

### Task 4: Copy Message Button

Hover-reveal copy button on every assistant message.

**Files:**
- Modify: `web/src/components/MessageBubble.tsx`

**Step 1: Add copy function and state**

```tsx
const [copied, setCopied] = useState(false);

const handleCopy = async () => {
  try {
    await navigator.clipboard.writeText(message.content);
  } catch {
    // Fallback for older browsers
    const ta = document.createElement('textarea');
    ta.value = message.content;
    ta.style.position = 'fixed';
    ta.style.opacity = '0';
    document.body.appendChild(ta);
    ta.select();
    document.execCommand('copy');
    document.body.removeChild(ta);
  }
  setCopied(true);
  setTimeout(() => setCopied(false), 1500);
};
```

**Step 2: Add copy button to assistant messages**

Wrap the assistant message bubble in a `group` div, add hover-reveal button:
```tsx
{message.role === 'assistant' && (
  <button
    onClick={handleCopy}
    data-testid="copy-message-btn"
    className="absolute top-2 right-2 opacity-0 group-hover:opacity-100 transition-opacity px-2 py-1 text-xs text-zinc-400 hover:text-zinc-200 bg-zinc-800 rounded border border-zinc-700"
  >
    {copied ? 'Copied' : 'Copy'}
  </button>
)}
```

Add `group relative` to the assistant message container div.

**Step 3: Verify + commit**
```bash
cd /home/rishav/soul-v2/web && npx tsc --noEmit && npx vite build 2>&1 | tail -3
git add web/src/components/MessageBubble.tsx
git commit -m "feat(chat): hover-reveal copy button on assistant messages"
```

**Pillar notes:** Secure: clipboard API with fallback, no eval. Robust: works on all browsers.

---

### Task 5: Rich Empty State

Replace "No messages yet" with Soul diamond + prompt suggestions.

**Files:**
- Modify: `web/src/components/MessageList.tsx`
- Modify: `web/src/components/Shell.tsx` (pass onSend to MessageList)

**Step 1: Update MessageList props**

```typescript
interface MessageListProps {
  messages: Message[];
  isStreaming: boolean;
  onSend?: (content: string) => void;
}
```

**Step 2: Replace empty state**

```tsx
if (messages.length === 0) {
  return (
    <div data-testid="message-list" className="flex-1 flex flex-col items-center justify-center gap-6 px-4">
      <span className="text-5xl text-amber-500 animate-pulse" style={{ textShadow: '0 0 20px rgba(232,168,73,0.3)' }}>
        &#9670;
      </span>
      <div className="grid grid-cols-2 gap-2 max-w-md w-full">
        {[
          'Explain this codebase',
          'Find bugs in my code',
          'Write a test for...',
          'Refactor this function',
        ].map(prompt => (
          <button
            key={prompt}
            onClick={() => onSend?.(prompt)}
            className="px-3 py-2.5 text-sm text-zinc-400 hover:text-zinc-200 bg-zinc-800/50 hover:bg-zinc-800 rounded-lg border border-zinc-700/50 transition-colors text-left"
          >
            {prompt}
          </button>
        ))}
      </div>
    </div>
  );
}
```

**Step 3: Pass onSend from Shell**

In `Shell.tsx`, pass `onSend={sendMessage}` to `<MessageList>`.

**Step 4: Verify + commit**
```bash
cd /home/rishav/soul-v2/web && npx tsc --noEmit && npx vite build 2>&1 | tail -3
git add web/src/components/MessageList.tsx web/src/components/Shell.tsx
git commit -m "feat(chat): rich empty state with diamond and prompt suggestions"
```

**Pillar notes:** Sovereign: diamond is Unicode, no external assets. Performant: zero overhead (only renders when empty).

---

## Phase 2: Code Block Excellence (new dep: react-syntax-highlighter)

### Task 6: Syntax Highlighting + Copy + Line Numbers

Replace basic code blocks with PrismLight highlighting, copy button, and line numbers for blocks > 5 lines.

**Files:**
- Create: `web/src/components/CodeBlock.tsx`
- Modify: `web/src/components/Markdown.tsx` (wire CodeBlock into react-markdown components)
- Modify: `web/package.json` (add react-syntax-highlighter)
- Modify: `web/vite.config.ts` (add vendor-syntax chunk)

**Step 1: Install dependency**
```bash
cd /home/rishav/soul-v2/web
npm install react-syntax-highlighter
npm install -D @types/react-syntax-highlighter
```

**Step 2: Create CodeBlock component**

Reference v1: `/home/rishav/soul/web/src/components/chat/CodeBlock.tsx` (177 lines)

Create `web/src/components/CodeBlock.tsx`:
- Import PrismLight + oneDark theme
- Register languages: tsx, typescript, javascript, go, python, bash, json, yaml, css, sql, markdown, rust, java, docker (14 languages — keep it lean)
- Language label header (top-right badge)
- Copy button with 2s confirmation
- Line numbers for blocks > 5 lines
- Fallback: plain `<code>` for inline code
- Max height with overflow-y-auto for very long blocks

**Step 3: Wire into Markdown.tsx**

Replace the `code` component override in Markdown.tsx to use CodeBlock:
```tsx
code({ className, children, ...props }) {
  const match = /language-(\w+)/.exec(className || '');
  const isInline = !match && !String(children).includes('\n');
  if (isInline) {
    return <code className="px-1.5 py-0.5 bg-zinc-800 rounded text-sm font-mono" {...props}>{children}</code>;
  }
  return <CodeBlock language={match?.[1] ?? ''} code={String(children).replace(/\n$/, '')} />;
}
```

**Step 4: Add vendor chunk in vite.config.ts**

```typescript
manualChunks: {
  'vendor-react': ['react', 'react-dom'],
  'vendor-markdown': ['react-markdown', 'remark-gfm'],
  'vendor-syntax': ['react-syntax-highlighter'],
},
```

**Step 5: Verify bundle size**
```bash
cd /home/rishav/soul-v2/web && npx vite build 2>&1 | tail -10
# Check vendor-syntax chunk gzipped size
# PrismLight with 14 languages should be ~25-35KB gzipped
# Total must remain < 250KB gzipped
```

**Step 6: Commit**
```bash
git add web/src/components/CodeBlock.tsx web/src/components/Markdown.tsx web/package.json web/package-lock.json web/vite.config.ts
git commit -m "feat(chat): syntax-highlighted code blocks with copy and line numbers"
```

**Pillar notes:** Performant: PrismLight (not full Prism) + selective language imports. Sovereign: bundled, no CDN. Robust: fallback to plain code if language unknown.

---

## Phase 3: Rich Tool Calls + Diffs

### Task 7: Rich Tool Call Display

Replace the basic JSON dump with v1-style tool calls: icons, status indicators, context extraction, truncated output.

**Files:**
- Rewrite: `web/src/components/ToolCallBlock.tsx`
- Modify: `web/src/components/MessageBubble.tsx`
- Modify: `web/src/lib/types.ts` (extend tool call data)

**Step 1: Extend types for tool call metadata**

If the WS protocol already sends tool status, progress, and output via `tool.call`, `tool.progress`, `tool.complete` — wire those into useChat. If not, add handling.

Add to types.ts:
```typescript
interface ToolCallData {
  id: string;
  name: string;
  input: Record<string, unknown>;
  status: 'running' | 'complete' | 'error';
  output?: string;
  progress?: number;
}
```

Add `tool.call`, `tool.progress`, `tool.complete`, `tool.error` to OutboundMessageType.

**Step 2: Handle tool events in useChat**

Track tool calls in a ref/state map. On `tool.call`: add entry. On `tool.progress`: update progress. On `tool.complete`: update output + status. On `tool.error`: update error.

Associate tool calls with the current streaming assistant message.

**Step 3: Rewrite ToolCallBlock**

Reference v1: `/home/rishav/soul/web/src/components/chat/ToolCall.tsx` (233 lines)

Key features:
- **Tool icons**: emoji per tool type (code_read: 📖, code_write: ✏️, code_edit: 📝, code_search: 🔍, code_grep: 🔎, code_exec: ⚡, task_update: 📋, e2e_assert: 🧪, e2e_dom: 🌐, default: 🔧)
- **Context extraction**: show file path for code_read/write/edit, query for grep/search, command for exec
- **Status**: spinner (running), green check (complete), red X (error)
- **Progress bar**: thin bar under header when running + progress > 0
- **Output**: truncated to 3000 chars, scrollable max-h-60, line count indicator
- **Expandable**: collapsed by default, click header to expand

**Step 4: Render tool calls in MessageBubble**

Instead of parsing tool messages as JSON, render ToolCallBlock components from the tool call data associated with the message.

**Step 5: Verify + commit**
```bash
cd /home/rishav/soul-v2/web && npx tsc --noEmit && npx vite build 2>&1 | tail -3
git add web/src/
git commit -m "feat(chat): rich tool call display with icons, status, progress"
```

---

### Task 8: Diff Blocks

Color-coded diff rendering in tool call output.

**Files:**
- Create: `web/src/components/DiffBlock.tsx`
- Modify: `web/src/components/ToolCallBlock.tsx` (detect and render diffs)

**Step 1: Create DiffBlock**

Reference v1: `/home/rishav/soul/web/src/components/chat/DiffBlock.tsx` (24 lines)

```tsx
// web/src/components/DiffBlock.tsx
interface DiffBlockProps {
  content: string;
}

export function DiffBlock({ content }: DiffBlockProps) {
  return (
    <pre className="text-[11px] font-mono max-h-60 overflow-y-auto p-2">
      {content.split('\n').map((line, i) => {
        let cls = 'text-zinc-400';
        if (line.startsWith('+')) cls = 'text-green-400 bg-green-400/10';
        else if (line.startsWith('-')) cls = 'text-red-400 bg-red-400/10';
        else if (line.startsWith('@@')) cls = 'text-blue-400/70';
        return (
          <div key={i} className={cls}>
            {line || '\u00a0'}
          </div>
        );
      })}
    </pre>
  );
}
```

**Step 2: Detect diffs in ToolCallBlock**

In ToolCallBlock, when rendering output:
```tsx
const isDiff = output && (output.includes('\n+') || output.includes('\n-')) && (output.includes('@@') || output.includes('---'));
if (isDiff) return <DiffBlock content={output} />;
```

**Step 3: Verify + commit**
```bash
cd /home/rishav/soul-v2/web && npx tsc --noEmit && npx vite build 2>&1 | tail -3
git add web/src/components/DiffBlock.tsx web/src/components/ToolCallBlock.tsx
git commit -m "feat(chat): color-coded diff blocks in tool output"
```

**Pillar notes:** Zero dependencies. Robust: handles empty lines, non-diff content falls through.

---

## Phase 4: Message Actions

### Task 9: Edit and Resend Message

Allow users to edit a sent message and resend (deletes messages after it, resends edited version).

**Files:**
- Modify: `web/src/components/MessageBubble.tsx`
- Modify: `web/src/hooks/useChat.ts` (add editAndResend function)

**Step 1: Add editAndResend to useChat**

```typescript
const editAndResend = useCallback((messageId: string, newContent: string) => {
  setMessages(prev => {
    const idx = prev.findIndex(m => m.id === messageId);
    if (idx === -1) return prev;
    return prev.slice(0, idx);  // Remove this message and everything after
  });
  // Small delay to let state update, then send as new message
  setTimeout(() => sendMessage(newContent), 50);
}, [sendMessage]);
```

Return `editAndResend` from useChat. Pass through Shell to MessageList to MessageBubble.

**Step 2: Add edit UI to MessageBubble**

For user messages, add hover-reveal edit button. On click, show textarea with current content + Submit/Cancel buttons:

```tsx
const [editing, setEditing] = useState(false);
const [editText, setEditText] = useState('');

// Edit button (user messages only, not during streaming)
{message.role === 'user' && !isStreaming && (
  <button onClick={() => { setEditing(true); setEditText(message.content); }}
    className="opacity-0 group-hover:opacity-100 ...">
    Edit
  </button>
)}

// Edit textarea
{editing && (
  <div className="mt-2 flex flex-col gap-2">
    <textarea value={editText} onChange={e => setEditText(e.target.value)}
      className="w-full bg-zinc-800 text-zinc-100 rounded p-2 text-sm resize-none" rows={3} />
    <div className="flex gap-2 justify-end">
      <button onClick={() => setEditing(false)} className="text-xs text-zinc-400">Cancel</button>
      <button onClick={() => { onEdit?.(message.id, editText.trim()); setEditing(false); }}
        className="text-xs bg-indigo-600 px-2 py-1 rounded">Submit</button>
    </div>
  </div>
)}
```

**Step 3: Verify + commit**
```bash
cd /home/rishav/soul-v2/web && npx tsc --noEmit && npx vite build 2>&1 | tail -3
git add web/src/
git commit -m "feat(chat): edit and resend user messages"
```

---

### Task 10: Retry + Timestamps + Model Badge

Three small message enhancements bundled together.

**Files:**
- Modify: `web/src/components/MessageBubble.tsx`
- Modify: `web/src/hooks/useChat.ts` (add retry, track model per message)
- Modify: `web/src/lib/types.ts` (add model to Message)

**Step 1: Add model field to Message type**

```typescript
interface Message {
  // ...existing...
  model?: string;  // e.g., "claude-opus-4-20250115"
}
```

In useChat, capture model from `chat.done` data and write it to the finalized message.

**Step 2: Add retry function to useChat**

```typescript
const retryMessage = useCallback((messageId: string) => {
  setMessages(prev => {
    const idx = prev.findIndex(m => m.id === messageId);
    if (idx === -1) return prev;
    const userMsg = prev[idx];
    if (userMsg.role !== 'user') return prev;
    // Keep messages before this one, resend
    setTimeout(() => sendMessage(userMsg.content), 50);
    return prev.slice(0, idx);
  });
}, [sendMessage]);
```

**Step 3: Add timestamp + model badge to MessageBubble**

Below each message, add a metadata line:
```tsx
<div className="flex items-center gap-2 mt-1 text-[11px] text-zinc-500">
  <span>{formatRelativeTime(message.createdAt)}</span>
  {message.model && message.role === 'assistant' && (
    <span className="px-1.5 py-0.5 bg-zinc-800 rounded text-[10px]">
      {message.model.includes('opus') ? 'Opus' :
       message.model.includes('sonnet') ? 'Sonnet' :
       message.model.includes('haiku') ? 'Haiku' : message.model.split('-')[0]}
    </span>
  )}
</div>
```

Extract `formatRelativeTime` to a shared util (already implemented in SessionList — extract to `web/src/lib/utils.ts`).

**Step 4: Add retry button for user messages**

Next to the edit button, add a retry button (visible on hover):
```tsx
<button onClick={() => onRetry?.(message.id)} ...>Retry</button>
```

**Step 5: Verify + commit**
```bash
cd /home/rishav/soul-v2/web && npx tsc --noEmit && npx vite build 2>&1 | tail -3
git add web/src/
git commit -m "feat(chat): retry messages, timestamps, model badges"
```

---

## Phase 5: Navigation and Search

### Task 11: Scroll-to-Bottom FAB

Floating action button that appears when user scrolls up, smooth-scrolls back to bottom.

**Files:**
- Modify: `web/src/components/MessageList.tsx`

**Step 1: Track scroll position**

```tsx
const [showScrollBtn, setShowScrollBtn] = useState(false);
const containerRef = useRef<HTMLDivElement>(null);

const handleScroll = useCallback(() => {
  const el = containerRef.current;
  if (!el) return;
  const isNearBottom = el.scrollHeight - el.scrollTop - el.clientHeight < 80;
  setShowScrollBtn(!isNearBottom);
}, []);
```

**Step 2: Render FAB**

```tsx
{showScrollBtn && (
  <button
    onClick={() => containerRef.current?.scrollTo({ top: containerRef.current.scrollHeight, behavior: 'smooth' })}
    data-testid="scroll-to-bottom"
    className="absolute bottom-20 right-4 w-10 h-10 bg-zinc-800 hover:bg-zinc-700 border border-zinc-600 rounded-full flex items-center justify-center text-zinc-400 hover:text-zinc-200 shadow-lg transition-all z-10"
  >
    ↓
  </button>
)}
```

**Step 3: Only auto-scroll when near bottom**

Update the auto-scroll useEffect to check `showScrollBtn`:
```tsx
useEffect(() => {
  if (!showScrollBtn) {
    endRef.current?.scrollIntoView({ behavior: 'smooth' });
  }
}, [messages, isStreaming, showScrollBtn]);
```

**Step 4: Verify + commit**
```bash
cd /home/rishav/soul-v2/web && npx tsc --noEmit && npx vite build 2>&1 | tail -3
git add web/src/components/MessageList.tsx
git commit -m "feat(chat): scroll-to-bottom FAB when scrolled up"
```

---

### Task 12: Message Search

Ctrl/Cmd+F to search messages by content. Highlight matches.

**Files:**
- Create: `web/src/components/SearchBar.tsx`
- Modify: `web/src/components/Shell.tsx` (keyboard shortcut, search state, filtering)
- Modify: `web/src/components/MessageList.tsx` (accept searchQuery prop for highlighting)
- Modify: `web/src/components/MessageBubble.tsx` (highlight matching text)

**Step 1: Create SearchBar component**

```tsx
interface SearchBarProps {
  query: string;
  onChange: (q: string) => void;
  onClose: () => void;
  matchCount: number;
}
```

Sticky bar at top of message area. Auto-focus. Escape to close. Shows match count.

**Step 2: Add keyboard shortcut in Shell**

```tsx
useEffect(() => {
  const handler = (e: KeyboardEvent) => {
    if ((e.metaKey || e.ctrlKey) && e.key === 'f') {
      e.preventDefault();
      setSearchOpen(true);
    }
    if (e.key === 'Escape' && searchOpen) {
      setSearchOpen(false);
      setSearchQuery('');
    }
  };
  window.addEventListener('keydown', handler);
  return () => window.removeEventListener('keydown', handler);
}, [searchOpen]);
```

**Step 3: Filter and highlight**

Pass `searchQuery` to MessageList. In MessageBubble, wrap matching text in `<mark>` elements with `bg-amber-500/30 text-inherit rounded` styling.

Count matches across all messages for the SearchBar display.

**Step 4: Verify + commit**
```bash
cd /home/rishav/soul-v2/web && npx tsc --noEmit && npx vite build 2>&1 | tail -3
git add web/src/
git commit -m "feat(chat): message search with Ctrl+F and highlighting"
```

**Pillar notes:** Performant: useMemo for filtered/highlighted messages. Robust: handles regex-special characters in search (escape them).

---

### Task 13: Keyboard Shortcuts

Global shortcuts for power users.

**Files:**
- Modify: `web/src/components/Shell.tsx`

**Step 1: Add global keydown handler**

```tsx
useEffect(() => {
  const handler = (e: KeyboardEvent) => {
    // Ctrl+K: Focus input
    if ((e.metaKey || e.ctrlKey) && e.key === 'k') {
      e.preventDefault();
      inputRef.current?.focus();
    }
    // Ctrl+Shift+N: New session
    if ((e.metaKey || e.ctrlKey) && e.shiftKey && e.key === 'N') {
      e.preventDefault();
      createSession();
    }
  };
  window.addEventListener('keydown', handler);
  return () => window.removeEventListener('keydown', handler);
}, [createSession]);
```

**Step 2: Forward input ref**

ChatInput needs to `forwardRef` or expose a ref to the textarea. Pass ref from Shell.

**Step 3: Verify + commit**
```bash
cd /home/rishav/soul-v2/web && npx tsc --noEmit && npx vite build 2>&1 | tail -3
git add web/src/
git commit -m "feat(chat): keyboard shortcuts (Ctrl+K focus, Ctrl+Shift+N new session)"
```

---

## Phase 6: Session and Token Polish

### Task 14: Session Status Dots + Search

Add status indicators and search to the session list.

**Files:**
- Modify: `web/src/components/SessionList.tsx`

**Step 1: Add status dot**

In SessionItem, before the title:
```tsx
const statusDot = (status: SessionStatus) => {
  switch (status) {
    case 'running':
      return <span className="w-2 h-2 rounded-full bg-green-500 animate-pulse shrink-0" />;
    case 'completed_unread':
      return <span className="w-2 h-2 rounded-full bg-indigo-500 ring-2 ring-indigo-500/30 shrink-0" />;
    default:
      return <span className="w-2 h-2 rounded-full bg-zinc-600 shrink-0" />;
  }
};
```

**Step 2: Add search input**

At the top of session list, below the header:
```tsx
<input
  type="text"
  value={searchQuery}
  onChange={e => setSearchQuery(e.target.value)}
  placeholder="Search sessions..."
  className="w-full px-2 py-1.5 text-sm bg-zinc-800 border border-zinc-700 rounded text-zinc-300 placeholder:text-zinc-500"
  data-testid="session-search"
/>
```

Filter sessions: `sessions.filter(s => s.title.toLowerCase().includes(searchQuery.toLowerCase()))`.

**Step 3: Verify + commit**
```bash
cd /home/rishav/soul-v2/web && npx tsc --noEmit && npx vite build 2>&1 | tail -3
git add web/src/components/SessionList.tsx
git commit -m "feat(chat): session status dots and search"
```

---

### Task 15: Token Usage Display

Show input/output token counts after assistant messages finish.

**Files:**
- Modify: `web/src/lib/types.ts` (add usage to Message)
- Modify: `web/src/hooks/useChat.ts` (capture usage from chat.done)
- Modify: `web/src/components/MessageBubble.tsx` (render token counts)

**Step 1: Extend Message type**

```typescript
interface Message {
  // ...existing...
  usage?: { inputTokens: number; outputTokens: number; cacheReadInputTokens?: number };
}
```

**Step 2: Capture in useChat**

In the `chat.done` handler, copy usage from the event data to the message.

**Step 3: Render in MessageBubble**

Below assistant messages (after timestamp):
```tsx
{message.usage && (
  <span className="text-[10px] text-zinc-600">
    {formatTokens(message.usage.inputTokens)} in · {formatTokens(message.usage.outputTokens)} out
    {message.usage.cacheReadInputTokens ? ` · ${formatTokens(message.usage.cacheReadInputTokens)} cached` : ''}
  </span>
)}
```

Helper: `formatTokens(n)` returns "1.2k" or "245" for small values.

**Step 4: Verify + commit**
```bash
cd /home/rishav/soul-v2/web && npx tsc --noEmit && npx vite build 2>&1 | tail -3
git add web/src/
git commit -m "feat(chat): token usage display on assistant messages"
```

---

## Phase 7: Input Enrichment

### Task 16: Model Selector

Dropdown to choose Claude model, saved to localStorage.

**Files:**
- Modify: `web/src/components/ChatInput.tsx`
- Modify: `web/src/hooks/useChat.ts` (send model with chat.send)

**Step 1: Fetch models**

In ChatInput, fetch available models on mount:
```tsx
const [models, setModels] = useState<{id: string; name: string}[]>([]);
const [selectedModel, setSelectedModel] = useState(() => localStorage.getItem('soul-model') || '');

useEffect(() => {
  fetch('/api/models').then(r => r.json()).then(data => {
    setModels(data);
    if (!selectedModel && data.length > 0) setSelectedModel(data[0].id);
  }).catch(() => {});
}, []);
```

Check if `/api/models` endpoint exists in the Go server. If not, add it (returns list of available Claude models).

**Step 2: Render dropdown**

Small select above or beside the textarea:
```tsx
<select value={selectedModel} onChange={e => { setSelectedModel(e.target.value); localStorage.setItem('soul-model', e.target.value); }}
  className="text-xs bg-zinc-800 text-zinc-400 border border-zinc-700 rounded px-2 py-1">
  {models.map(m => <option key={m.id} value={m.id}>{m.name}</option>)}
</select>
```

**Step 3: Send model with message**

Pass selected model to onSend: `onSend(content, selectedModel)`. Update useChat to include model in `chat.send` payload.

**Step 4: Verify + commit**
```bash
cd /home/rishav/soul-v2/web && npx tsc --noEmit && npx vite build 2>&1 | tail -3
git add web/src/
git commit -m "feat(chat): model selector dropdown with localStorage persistence"
```

**Pillar notes:** Resilient: fallback to server default if model fetch fails. Sovereign: models come from own server, not external API.

---

### Task 17: Extended Thinking Toggle

Toggle button to enable/disable extended thinking (Opus only).

**Files:**
- Modify: `web/src/components/ChatInput.tsx`
- Modify: `web/src/hooks/useChat.ts` (send thinking flag)

**Step 1: Add toggle state**

```tsx
const [thinkingEnabled, setThinkingEnabled] = useState(() =>
  localStorage.getItem('soul-thinking') === 'true'
);
const isOpus = selectedModel.includes('opus');
```

**Step 2: Render toggle (only for Opus)**

```tsx
{isOpus && (
  <button
    onClick={() => { const next = !thinkingEnabled; setThinkingEnabled(next); localStorage.setItem('soul-thinking', String(next)); }}
    className={`text-xs px-2 py-1 rounded border transition-colors ${
      thinkingEnabled ? 'bg-amber-600/20 border-amber-600/50 text-amber-400' : 'bg-zinc-800 border-zinc-700 text-zinc-500'
    }`}
    data-testid="thinking-toggle"
  >
    Thinking {thinkingEnabled ? 'ON' : 'OFF'}
  </button>
)}
```

**Step 3: Pass to chat.send**

Include `thinking: thinkingEnabled` in the send payload. Backend should use this to include `thinking` parameter in Claude API request.

**Step 4: Verify + commit**
```bash
cd /home/rishav/soul-v2/web && npx tsc --noEmit && npx vite build 2>&1 | tail -3
git add web/src/
git commit -m "feat(chat): extended thinking toggle for Opus models"
```

---

### Task 18: Slash Commands

Command palette when typing `/` at the start of input.

**Files:**
- Create: `web/src/hooks/useSlashCommands.ts`
- Create: `web/src/components/CommandPalette.tsx`
- Modify: `web/src/components/ChatInput.tsx`

**Step 1: Create useSlashCommands hook**

Reference v1: `/home/rishav/soul/web/src/hooks/useSlashCommands.ts` (41 lines)

```typescript
interface SlashCommand {
  name: string;
  description: string;
}

const BUILTIN_COMMANDS: SlashCommand[] = [
  { name: 'think', description: 'Toggle extended thinking' },
  { name: 'code', description: 'Code generation mode' },
  { name: 'architect', description: 'Architecture discussion' },
  { name: 'brainstorm', description: 'Brainstorm ideas' },
  { name: 'review', description: 'Code review mode' },
  { name: 'debug', description: 'Debug an issue' },
];

export function useSlashCommands(): SlashCommand[] {
  // Return builtin commands. Can fetch /api/skills dynamically later.
  return BUILTIN_COMMANDS;
}
```

**Step 2: Create CommandPalette component**

Floating list above input. Arrow keys to navigate, Enter/Tab to select, Escape to close.

```tsx
interface CommandPaletteProps {
  commands: SlashCommand[];
  filter: string;
  onSelect: (cmd: SlashCommand) => void;
  onClose: () => void;
}
```

**Step 3: Wire into ChatInput**

Detect when input starts with `/` and no space after. Show palette filtered by text after `/`. On select, replace input content or trigger command action.

**Step 4: Verify + commit**
```bash
cd /home/rishav/soul-v2/web && npx tsc --noEmit && npx vite build 2>&1 | tail -3
git add web/src/
git commit -m "feat(chat): slash command palette with keyboard navigation"
```

---

## Phase 8: Advanced Features (new deps, deferred)

### Task 19: Mermaid Diagrams (lazy loaded)

Render mermaid code blocks as SVG diagrams.

**Files:**
- Create: `web/src/components/MermaidBlock.tsx`
- Modify: `web/src/components/CodeBlock.tsx` (detect mermaid language, render MermaidBlock)
- Modify: `web/package.json` (add mermaid)

**Step 1: Install mermaid**
```bash
cd /home/rishav/soul-v2/web && npm install mermaid
```

**Step 2: Create MermaidBlock with lazy loading**

Reference v1: `/home/rishav/soul/web/src/components/chat/MermaidBlock.tsx` (70 lines)

Key: Dynamic `import('mermaid')` — not statically imported. This keeps mermaid out of the initial bundle.

```tsx
import { useEffect, useRef, useState } from 'react';

let mermaidPromise: Promise<typeof import('mermaid')> | null = null;

function getMermaid() {
  if (!mermaidPromise) {
    mermaidPromise = import('mermaid').then(m => {
      m.default.initialize({
        startOnLoad: false,
        theme: 'dark',
        themeVariables: {
          primaryColor: '#a78bfa',
          secondaryColor: '#27272a',
          lineColor: '#71717a',
          primaryTextColor: '#e4e4e7',
          secondaryTextColor: '#a1a1aa',
        },
      });
      return m;
    });
  }
  return mermaidPromise;
}
```

Render: call `mermaid.render(id, content)`, set SVG via ref. Show error + source on parse failure.

**Important:** This uses `dangerouslySetInnerHTML` for the SVG — acceptable because mermaid generates sanitized SVG. Document this pillar exception in a code comment.

**Step 3: Detect in CodeBlock**

```tsx
if (language === 'mermaid') {
  return <MermaidBlock content={code} />;
}
```

**Step 4: Verify bundle — mermaid must NOT appear in initial chunks**
```bash
cd /home/rishav/soul-v2/web && npx vite build 2>&1 | grep -i mermaid
# Should show a separate lazy chunk, not in vendor-* or index-*
```

**Step 5: Commit**
```bash
git add web/src/components/MermaidBlock.tsx web/src/components/CodeBlock.tsx web/package.json web/package-lock.json
git commit -m "feat(chat): lazy-loaded mermaid diagram rendering"
```

**Pillar notes:** Performant: lazy-loaded, zero initial bundle cost. Secure: SVG from mermaid library (trusted). Sovereign: bundled, no CDN.

---

### Task 20: File Attachments (drag and drop, paste)

Allow users to attach files and images to messages.

**Files:**
- Modify: `web/src/components/ChatInput.tsx` (drop zone, paste handler, file chips)
- Modify: `web/src/hooks/useChat.ts` (send files as base64 or FormData)
- Modify: `web/src/components/Shell.tsx` (drag overlay)

**Step 1: Add paste handler**

In ChatInput, on the textarea:
```tsx
onPaste={(e) => {
  const files = Array.from(e.clipboardData.files);
  if (files.length > 0) {
    e.preventDefault();
    addFiles(files);
  }
}}
```

**Step 2: Add drag and drop**

In Shell or ChatInput, add dragover/drop handlers with visual overlay.

**Step 3: File preview chips**

Show attached files as removable chips above the textarea.

**Step 4: Send files**

Convert files to base64 and include in the `chat.send` payload. Backend needs to handle file content in the Claude API request (as image content blocks for images, or text for code files).

**Step 5: Verify + commit**
```bash
cd /home/rishav/soul-v2/web && npx tsc --noEmit && npx vite build 2>&1 | tail -3
git add web/src/
git commit -m "feat(chat): file attachments via drag-drop and paste"
```

**Pillar notes:** Secure: validate file types, size limits client-side (5MB max). Robust: handle paste without files gracefully.

---

### Task 21: Speech Recognition (browser API)

Optional microphone input using the Web Speech API.

**Files:**
- Modify: `web/src/components/ChatInput.tsx`

**Step 1: Check browser support and add toggle**

```tsx
const hasSpeech = 'webkitSpeechRecognition' in window || 'SpeechRecognition' in window;
```

Only show mic button if supported. Use Web Speech API for recognition with interim results.

**Step 2: Render mic button**

```tsx
{hasSpeech && (
  <button onClick={toggleSpeech}
    className={`... ${listening ? 'text-red-400 animate-pulse' : 'text-zinc-400'}`}
    data-testid="speech-button">
    mic icon
  </button>
)}
```

**Step 3: Verify + commit**
```bash
cd /home/rishav/soul-v2/web && npx tsc --noEmit && npx vite build 2>&1 | tail -3
git add web/src/components/ChatInput.tsx
git commit -m "feat(chat): speech recognition input via Web Speech API"
```

**Pillar notes:** Sovereign: uses browser-native API, no external service. Resilient: graceful hide if not supported. Secure: microphone requires user permission.

---

## Execution Order and Dependencies

```
Phase 1 (zero deps):  T1 -> T2 -> T3 -> T4 -> T5
Phase 2 (syntax dep): T6
Phase 3 (tool calls): T7 -> T8
Phase 4 (msg actions): T9 -> T10
Phase 5 (navigation):  T11 -> T12 -> T13
Phase 6 (sessions):    T14 -> T15
Phase 7 (input):       T16 -> T17 -> T18
Phase 8 (advanced):    T19, T20, T21 (independent)
```

Tasks within a phase are sequential. Phases 1-7 should be done in order. Phase 8 tasks are independent and can be done in any order.

## Bundle Budget Tracking

| After Phase | Estimated Gzipped | Notes |
|-------------|-------------------|-------|
| Current     | 119KB | Baseline |
| Phase 1     | ~120KB | Zero new deps, only component code |
| Phase 2     | ~155KB | PrismLight + 14 languages (~35KB) |
| Phase 3-7   | ~160KB | Only component code additions |
| Phase 8 T19 | ~160KB initial | Mermaid lazy-loaded (~250KB on-demand, not in initial) |
| Phase 8 T20 | ~162KB | File handling code |
| Phase 8 T21 | ~163KB | Speech API (browser-native) |

**Final estimated: ~163KB gzipped initial** (well under 250KB ceiling).

## Server-Side Dependencies

Some tasks need backend changes. Check/add these endpoints:

| Task | Backend Need | File |
|------|-------------|------|
| T2 | Handle `chat.stop` — cancel stream context | `internal/ws/handler.go` |
| T3 | Send `chat.thinking` events | `internal/ws/handler.go` |
| T7 | Send `tool.call/progress/complete/error` events | `internal/ws/handler.go` |
| T15 | Include `usage` in `chat.done` | `internal/ws/handler.go` |
| T16 | `/api/models` endpoint | `internal/server/server.go` |
| T20 | Handle file content in `chat.send` | `internal/ws/handler.go` |
