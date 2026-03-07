# Chat UI Phase 2-3 Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Complete all remaining Chat UI improvements from the gap analysis -- P1 through P3 features covering message actions, diff rendering, progress bars, mermaid/image rendering, keyboard shortcuts, conversation search, drag-and-drop, streaming word count, and context window usage.

**Architecture:** All changes are frontend-only except two small backend additions: (1) streaming token count events and (2) context usage percentage in chat.done. New npm dependencies: mermaid for diagram rendering. All new components go in web/src/components/chat/. Existing components are extended incrementally -- each task is independently testable.

**Tech Stack:** React 19, TypeScript, Tailwind CSS v4, Vite, react-markdown, react-syntax-highlighter (PrismLight), mermaid.js

---

## Task 1: User Message Markdown Rendering

User messages currently render as whitespace-pre-wrap plain text. They should render through ReactMarkdown like assistant messages, so pasted code/lists/links display properly.

**Files:**
- Modify: `web/src/components/chat/Message.tsx:223-227`

**Step 1: Implement**

In Message.tsx, replace the user message content block (lines 223-227):

```tsx
// BEFORE:
{message.content && (
  isUser ? (
    <div className="whitespace-pre-wrap break-words text-sm leading-relaxed">
      {message.content}
    </div>
  ) : (

// AFTER:
{message.content && (
  isUser ? (
    <div className="prose prose-sm prose-soul max-w-none break-words prose-p:my-1 prose-li:my-0">
      <ReactMarkdown remarkPlugins={[remarkGfm]} components={markdownComponents}>{message.content}</ReactMarkdown>
    </div>
  ) : (
```

**Step 2: Build and verify**

```bash
cd web && npx vite build
```

Open chat, send a message with markdown (backticks, bullet list, link). Verify it renders with formatting -- code gets highlighted, lists are styled, links are clickable.

**Step 3: Commit**

```bash
git add web/src/components/chat/Message.tsx
git commit -m "feat(chat): render user messages through ReactMarkdown"
```

---

## Task 2: Message Retry and Edit Actions

Add retry (resend the user message above) and edit (open user message in textarea, resubmit) to the message action row. These appear on hover alongside the existing copy button.

**Files:**
- Modify: `web/src/components/chat/Message.tsx`
- Modify: `web/src/components/chat/ChatView.tsx`
- Modify: `web/src/hooks/useChat.ts`

**Step 1: Add retry/edit callbacks to useChat**

In `web/src/hooks/useChat.ts`, add two new functions before the return statement (after sendMessage):

```typescript
const retryFromMessage = useCallback(
  (messageId: string) => {
    setMessages((prev) => {
      const idx = prev.findIndex((m) => m.id === messageId && m.role === 'user');
      if (idx === -1) return prev;
      const userMsg = prev[idx];
      const truncated = prev.slice(0, idx);
      setMessages(truncated);
      setTimeout(() => sendMessage(userMsg.content), 0);
      return prev;
    });
  },
  [sendMessage],
);

const editMessage = useCallback(
  (messageId: string, newContent: string) => {
    setMessages((prev) => {
      const idx = prev.findIndex((m) => m.id === messageId && m.role === 'user');
      if (idx === -1) return prev;
      return prev.slice(0, idx);
    });
    setTimeout(() => sendMessage(newContent), 0);
  },
  [sendMessage],
);
```

Update the return to include them:

```typescript
return { messages, setMessages, sendMessage, isStreaming, connected, sessionId, setSessionId, tokenUsage, retryFromMessage, editMessage };
```

**Step 2: Wire through ChatView**

In `web/src/components/chat/ChatView.tsx`, destructure the new functions:

```typescript
const { messages, sendMessage, isStreaming, sessionId, setSessionId, tokenUsage, retryFromMessage, editMessage } = useChat();
```

Pass them to Message:

```tsx
{messages.map((msg) => (
  <Message key={msg.id} message={msg} onRetry={retryFromMessage} onEdit={editMessage} isStreaming={isStreaming} />
))}
```

**Step 3: Add action buttons to Message component**

In Message.tsx, update MessageProps:

```tsx
interface MessageProps {
  message: ChatMessage;
  onRetry?: (messageId: string) => void;
  onEdit?: (messageId: string, newContent: string) => void;
  isStreaming?: boolean;
}
```

Add a RetryBtn component after CopyMessageBtn:

```tsx
function RetryBtn({ onClick }: { onClick: () => void }) {
  return (
    <button
      type="button"
      onClick={onClick}
      className="opacity-0 group-hover:opacity-100 transition-opacity text-fg-muted hover:text-fg cursor-pointer"
      title="Retry"
    >
      <svg width="12" height="12" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
        <path d="M1 4v5h5" /><path d="M3.5 11A6 6 0 1 0 3 5l-2 2" />
      </svg>
    </button>
  );
}
```

Add an EditBtn that toggles an inline textarea:

```tsx
function EditBtn({ content, onEdit }: { content: string; onEdit: (text: string) => void }) {
  const [editing, setEditing] = useState(false);
  const [text, setText] = useState(content);

  if (editing) {
    return (
      <div className="mt-2 w-full">
        <textarea
          value={text}
          onChange={(e) => setText(e.target.value)}
          className="w-full bg-elevated border border-border-subtle rounded-lg px-3 py-2 text-sm text-fg resize-none focus:outline-none focus:border-soul/40"
          rows={3}
          autoFocus
        />
        <div className="flex gap-2 mt-1">
          <button type="button" onClick={() => { onEdit(text); setEditing(false); }}
            className="text-xs text-soul hover:underline cursor-pointer">Submit</button>
          <button type="button" onClick={() => { setText(content); setEditing(false); }}
            className="text-xs text-fg-muted hover:text-fg cursor-pointer">Cancel</button>
        </div>
      </div>
    );
  }

  return (
    <button
      type="button"
      onClick={() => setEditing(true)}
      className="opacity-0 group-hover:opacity-100 transition-opacity text-fg-muted hover:text-fg cursor-pointer"
      title="Edit and resend"
    >
      <svg width="12" height="12" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
        <path d="M11.5 1.5l3 3L5 14H2v-3L11.5 1.5z" />
      </svg>
    </button>
  );
}
```

In MessageMeta, add retry for user messages and pass onRetry/onEdit:

```tsx
function MessageMeta({ message, onRetry, onEdit, isStreaming }: {
  message: ChatMessage;
  onRetry?: (id: string) => void;
  onEdit?: (id: string, text: string) => void;
  isStreaming?: boolean;
}) {
  const timeStr = useMemo(() => formatRelativeTime(message.timestamp), [message.timestamp]);
  const badge = modelLabel(message.model);
  const isUser = message.role === 'user';

  return (
    <div className={`flex items-center gap-2 mt-1.5 text-[10px] text-fg-muted ${isUser ? 'justify-end' : 'justify-start'}`}>
      <span>{timeStr}</span>
      {badge && !isUser && (
        <span className="px-1.5 py-0.5 rounded bg-soul/10 text-soul font-mono">{badge}</span>
      )}
      {message.content && <CopyMessageBtn content={message.content} />}
      {isUser && onRetry && !isStreaming && (
        <RetryBtn onClick={() => onRetry(message.id)} />
      )}
      {isUser && onEdit && !isStreaming && (
        <EditBtn content={message.content} onEdit={(text) => onEdit(message.id, text)} />
      )}
    </div>
  );
}
```

Update the Message component signature and MessageMeta call:

```tsx
export default function Message({ message, onRetry, onEdit, isStreaming }: MessageProps) {
  // ...
  <MessageMeta message={message} onRetry={onRetry} onEdit={onEdit} isStreaming={isStreaming} />
}
```

**Step 4: Build and verify**

```bash
cd web && npx vite build
```

Send a message, hover over user message to see retry + edit icons. Click retry to re-send. Click edit to open inline textarea, modify, submit. Verify conversation truncates and resends correctly.

**Step 5: Commit**

```bash
git add web/src/components/chat/Message.tsx web/src/components/chat/ChatView.tsx web/src/hooks/useChat.ts
git commit -m "feat(chat): add retry and edit message actions on hover"
```

---

## Task 3: Tool Call Progress Bar

The progress field on tool calls is already tracked but only shown as text. Render a small progress bar when progress is between 0-100 and status is running.

**Files:**
- Modify: `web/src/components/chat/ToolCall.tsx:82-105`

**Step 1: Implement**

In ToolCall.tsx, add a progress bar inside the pill content, after the status icon span (line 86) and before the icon span (line 87). Insert between them:

```tsx
{isRunning && typeof toolCall.progress === 'number' && toolCall.progress > 0 && (
  <div className="w-12 h-1 rounded-full bg-overlay shrink-0 overflow-hidden">
    <div
      className="h-full rounded-full bg-soul transition-all duration-300"
      style={{ width: `${Math.min(toolCall.progress, 100)}%` }}
    />
  </div>
)}
```

**Step 2: Build and verify**

```bash
cd web && npx vite build
```

Trigger a tool call that has progress events. Verify the progress bar fills as events stream in.

**Step 3: Commit**

```bash
git add web/src/components/chat/ToolCall.tsx
git commit -m "feat(chat): add progress bar to running tool calls"
```

---

## Task 4: Diff Rendering for code_edit Tool Calls

When a code_edit or code_write tool call completes, its output often contains before/after content. Parse the output to detect unified diff format and render with red/green line coloring.

**Files:**
- Create: `web/src/components/chat/DiffBlock.tsx`
- Modify: `web/src/components/chat/ToolCall.tsx`

**Step 1: Create DiffBlock component**

Create `web/src/components/chat/DiffBlock.tsx`:

```tsx
interface DiffBlockProps {
  content: string;
}

export default function DiffBlock({ content }: DiffBlockProps) {
  const lines = content.split('\n');
  return (
    <div className="max-h-60 overflow-y-auto">
      <pre className="p-2 text-[11px] font-mono leading-relaxed">
        {lines.map((line, i) => {
          let cls = 'text-fg-muted';
          if (line.startsWith('+') && !line.startsWith('+++')) cls = 'text-green-400 bg-green-400/10';
          else if (line.startsWith('-') && !line.startsWith('---')) cls = 'text-red-400 bg-red-400/10';
          else if (line.startsWith('@@')) cls = 'text-soul/70';
          return (
            <div key={i} className={`px-2 ${cls}`}>
              {line || '\u00A0'}
            </div>
          );
        })}
      </pre>
    </div>
  );
}
```

**Step 2: Use DiffBlock in ToolCall output**

In ToolCall.tsx, import DiffBlock and add a helper:

```tsx
import DiffBlock from './DiffBlock.tsx';

function isDiffOutput(name: string, output: string): boolean {
  if (name !== 'code_edit' && name !== 'code_write') return false;
  return output.includes('\n+') && output.includes('\n-') && (output.includes('@@') || output.includes('---'));
}
```

Then in the expanded output section (lines 126-133), replace the output pre block:

```tsx
{toolCall.output && (
  isDiffOutput(toolCall.name, toolCall.output) ? (
    <DiffBlock content={toolCall.output.length > 3000
      ? toolCall.output.slice(0, 3000) + '\n... (truncated)'
      : toolCall.output} />
  ) : (
    <div className="max-h-60 overflow-y-auto">
      <pre className="p-2 text-fg-muted text-[11px] whitespace-pre-wrap leading-relaxed">
        {toolCall.output.length > 3000
          ? toolCall.output.slice(0, 3000) + `\n... (${outputLines} lines total)`
          : toolCall.output}
      </pre>
    </div>
  )
)}
```

**Step 3: Build and verify**

```bash
cd web && npx vite build
```

Trigger a code_edit tool call. Expand the output. Verify added lines are green and removed lines are red.

**Step 4: Commit**

```bash
git add web/src/components/chat/DiffBlock.tsx web/src/components/chat/ToolCall.tsx
git commit -m "feat(chat): diff rendering for code_edit tool call outputs"
```

---

## Task 5: File Path Links in Tool Calls

Make file paths in tool call context clickable -- clicking copies the path to clipboard.

**Files:**
- Modify: `web/src/components/chat/ToolCall.tsx:89-91`

**Step 1: Implement**

In ToolCall.tsx, replace the plain text context display (lines 89-91):

```tsx
{context && (
  <span
    className="text-soul/70 truncate cursor-pointer hover:text-soul hover:underline"
    title={`Click to copy: ${context}`}
    onClick={(e) => {
      e.stopPropagation();
      const path = context.replace(/^"|"$/g, '');
      const fallback = () => {
        const ta = document.createElement('textarea');
        ta.value = path;
        ta.style.position = 'fixed';
        ta.style.opacity = '0';
        document.body.appendChild(ta);
        ta.select();
        try { document.execCommand('copy'); } catch (_) {}
        document.body.removeChild(ta);
      };
      if (navigator.clipboard?.writeText) {
        navigator.clipboard.writeText(path).catch(fallback);
      } else {
        fallback();
      }
    }}
  >
    {context}
  </span>
)}
```

**Step 2: Build and verify**

```bash
cd web && npx vite build
```

Trigger a code_read tool call. Verify the file path has hover underline. Click it -- verify path copied.

**Step 3: Commit**

```bash
git add web/src/components/chat/ToolCall.tsx
git commit -m "feat(chat): make file paths in tool calls clickable (copy to clipboard)"
```

---

## Task 6: Streaming Token Count

Show a live word count while the assistant is streaming, replacing the bouncing dots when content is flowing.

**Files:**
- Modify: `web/src/components/chat/ChatView.tsx:174-182`

**Step 1: Implement**

In ChatView.tsx, replace the streaming indicator (lines 174-182):

```tsx
{isStreaming && (() => {
  const lastMsg = messages[messages.length - 1];
  if (lastMsg?.role === 'assistant' && lastMsg.content) {
    const words = lastMsg.content.trim().split(/\s+/).length;
    return (
      <div className="flex items-center gap-2 py-2 px-1">
        <span className="w-1.5 h-1.5 rounded-full bg-soul animate-pulse" />
        <span className="text-[10px] font-mono text-fg-muted">
          {words} word{words !== 1 ? 's' : ''}
        </span>
      </div>
    );
  }
  if (lastMsg?.role !== 'assistant') {
    return (
      <div className="flex gap-3">
        <div className="flex gap-1 items-center py-2 px-1">
          <span className="w-1.5 h-1.5 rounded-full bg-soul animate-bounce [animation-delay:0ms]" />
          <span className="w-1.5 h-1.5 rounded-full bg-soul animate-bounce [animation-delay:150ms]" />
          <span className="w-1.5 h-1.5 rounded-full bg-soul animate-bounce [animation-delay:300ms]" />
        </div>
      </div>
    );
  }
  return null;
})()}
```

**Step 2: Build and verify**

```bash
cd web && npx vite build
```

Send a message. While streaming, verify you see a pulsing dot + word count that increments. Before first token arrives, verify bouncing dots still show.

**Step 3: Commit**

```bash
git add web/src/components/chat/ChatView.tsx
git commit -m "feat(chat): show live word count while streaming"
```

---

## Task 7: Context Window Usage Indicator

Show an estimated context usage percentage in the token badge. Requires a small backend change to include context_pct in the chat.done event.

**Files:**
- Modify: `internal/server/agent.go:540-548`
- Modify: `web/src/hooks/useChat.ts`
- Modify: `web/src/lib/types.ts`
- Modify: `web/src/components/chat/ChatView.tsx`

**Step 1: Backend -- add context_pct to chat.done**

In agent.go, update the usageJSON marshaling (around line 540). Replace the existing block:

```go
usagePctFinal := 0
if a.contextBudget > 0 {
    usage := estimateTokens(messages, sysPrompt)
    usagePctFinal = int(float64(usage) / float64(a.contextBudget) * 100)
}
usageJSON, _ := json.Marshal(map[string]int{
    "input_tokens":  a.totalInputTokens,
    "output_tokens": a.totalOutputTokens,
    "context_pct":   usagePctFinal,
})
```

**Step 2: Frontend -- update types and useChat**

In `web/src/lib/types.ts`, add contextPct to TokenUsage:

```typescript
export interface TokenUsage {
  inputTokens: number;
  outputTokens: number;
  contextPct: number;
}
```

In `web/src/hooks/useChat.ts`, update the chat.done handler:

```typescript
case 'chat.done': {
  const data = msg.data as { input_tokens?: number; output_tokens?: number; context_pct?: number } | undefined;
  if (data?.input_tokens || data?.output_tokens) {
    setTokenUsage({
      inputTokens: data.input_tokens ?? 0,
      outputTokens: data.output_tokens ?? 0,
      contextPct: data.context_pct ?? 0,
    });
  }
  setIsStreaming(false);
  break;
}
```

**Step 3: Frontend -- display context usage**

In ChatView.tsx, update the token usage display:

```tsx
{!isStreaming && tokenUsage && (
  <div className="flex justify-center">
    <span className="text-[10px] font-mono text-fg-muted px-2 py-0.5 rounded bg-elevated/50">
      {formatTokens(tokenUsage.inputTokens)} in . {formatTokens(tokenUsage.outputTokens)} out
      {tokenUsage.contextPct > 0 && (
        <> . <span className={tokenUsage.contextPct > 70 ? 'text-stage-blocked' : tokenUsage.contextPct > 50 ? 'text-stage-validation' : ''}>
          {tokenUsage.contextPct}% ctx
        </span></>
      )}
    </span>
  </div>
)}
```

**Step 4: Build and verify**

```bash
go build -o soul ./cmd/soul && cd web && npx vite build
```

Restart server, send a message. Verify token badge shows "X% ctx" with color coding (red >70%, amber >50%).

**Step 5: Commit**

```bash
git add internal/server/agent.go web/src/lib/types.ts web/src/hooks/useChat.ts web/src/components/chat/ChatView.tsx
git commit -m "feat(chat): context window usage indicator with color coding"
```

---

## Task 8: Mermaid Diagram Rendering

Detect mermaid fenced code blocks and render them as SVG diagrams inline.

**Files:**
- Create: `web/src/components/chat/MermaidBlock.tsx`
- Modify: `web/src/components/chat/Message.tsx`
- Modify: `web/package.json` (add mermaid dependency)

**Step 1: Install mermaid**

```bash
cd web && npm install mermaid
```

**Step 2: Create MermaidBlock component**

Create `web/src/components/chat/MermaidBlock.tsx`:

```tsx
import { useEffect, useRef, useState } from 'react';
import mermaid from 'mermaid';

mermaid.initialize({
  startOnLoad: false,
  theme: 'dark',
  themeVariables: {
    darkMode: true,
    background: 'transparent',
    primaryColor: 'var(--color-soul)',
    primaryTextColor: 'var(--color-fg)',
    lineColor: 'var(--color-fg-muted)',
    secondaryColor: 'var(--color-elevated)',
  },
});

let mermaidId = 0;

interface MermaidBlockProps {
  children: string;
}

export default function MermaidBlock({ children }: MermaidBlockProps) {
  const containerRef = useRef<HTMLDivElement>(null);
  const [error, setError] = useState<string | null>(null);
  const [svg, setSvg] = useState<string>('');

  useEffect(() => {
    const id = `mermaid-${++mermaidId}`;
    let cancelled = false;

    mermaid.render(id, children.trim()).then(({ svg: rendered }) => {
      if (!cancelled) setSvg(rendered);
    }).catch((err) => {
      if (!cancelled) setError(String(err));
    });

    return () => { cancelled = true; };
  }, [children]);

  if (error) {
    return (
      <div className="my-3 p-3 rounded-lg border border-border-subtle bg-elevated">
        <div className="text-[10px] text-fg-muted mb-1">Mermaid parse error</div>
        <pre className="text-xs text-stage-blocked whitespace-pre-wrap">{error}</pre>
        <pre className="mt-2 text-xs text-fg-muted whitespace-pre-wrap">{children}</pre>
      </div>
    );
  }

  if (!svg) return null;

  return (
    <div
      ref={containerRef}
      className="my-3 p-4 rounded-lg border border-border-subtle bg-elevated/40 overflow-x-auto [&_svg]:max-w-full"
      dangerouslySetInnerHTML={{ __html: svg }}
    />
  );
}
```

**Step 3: Wire into Message markdown renderer**

In Message.tsx, import MermaidBlock and update the code component in markdownComponents:

```tsx
import MermaidBlock from './MermaidBlock.tsx';

const markdownComponents = {
  code({ className, children, ...props }: any) {
    const match = /language-(\w+)/.exec(className || '');
    const lang = match?.[1];
    const isInline = !match && !className;
    const text = String(children).replace(/\n$/, '');

    if (isInline) {
      return <CodeBlock inline>{text}</CodeBlock>;
    }

    if (lang === 'mermaid') {
      return <MermaidBlock>{text}</MermaidBlock>;
    }

    return <CodeBlock language={lang}>{text}</CodeBlock>;
  },
  pre({ children }: any) {
    return <>{children}</>;
  },
};
```

**Step 4: Build and verify**

```bash
cd web && npx vite build
```

Send a message asking for a mermaid diagram. Verify the diagram renders as SVG inline with dark theme. Verify invalid mermaid syntax shows the error + raw text.

**Step 5: Commit**

```bash
git add web/src/components/chat/MermaidBlock.tsx web/src/components/chat/Message.tsx web/package.json web/package-lock.json
git commit -m "feat(chat): inline mermaid diagram rendering"
```

---

## Task 9: Inline Image Rendering

When tool output includes screenshot paths or assistant messages contain image URLs, render them inline.

**Files:**
- Modify: `web/src/components/chat/ToolCall.tsx`
- Modify: `web/src/components/chat/Message.tsx`

**Step 1: Add image rendering to markdown**

In Message.tsx, add an img component to markdownComponents:

```tsx
img({ src, alt, ...props }: any) {
  if (!src) return null;
  return (
    <div className="my-3 rounded-lg overflow-hidden border border-border-subtle inline-block max-w-full">
      <img src={src} alt={alt || ''} className="max-w-full max-h-96 object-contain" loading="lazy" {...props} />
      {alt && <div className="px-2 py-1 text-[10px] text-fg-muted bg-elevated/60">{alt}</div>}
    </div>
  );
},
```

**Step 2: Add image detection in tool output**

In ToolCall.tsx, add a helper to detect image paths:

```tsx
function extractImagePath(output: string): string | null {
  const match = output.match(/\/(api\/screenshots\/[^\s]+|[^\s]+\.(png|jpg|jpeg|gif|webp|svg))/i);
  return match ? (match[0].startsWith('/') ? match[0] : '/' + match[0]) : null;
}
```

In the expanded output section, before the pre/DiffBlock, add:

```tsx
{toolCall.output && (() => {
  const imgSrc = extractImagePath(toolCall.output);
  if (imgSrc) {
    return (
      <div className="p-2">
        <img src={imgSrc} alt="Tool output" className="max-w-full max-h-60 rounded border border-border-subtle" loading="lazy" />
      </div>
    );
  }
  return null;
})()}
```

**Step 3: Build and verify**

```bash
cd web && npx vite build
```

Test with a screenshot tool call output -- verify image renders inline. Test markdown images in assistant messages.

**Step 4: Commit**

```bash
git add web/src/components/chat/ToolCall.tsx web/src/components/chat/Message.tsx
git commit -m "feat(chat): inline image rendering in messages and tool outputs"
```

---

## Task 10: Conversation Search

Add a search bar in the chat header that filters/highlights messages within the current session.

**Files:**
- Modify: `web/src/components/chat/ChatView.tsx`
- Modify: `web/src/components/chat/Message.tsx`

**Step 1: Add search state to ChatView**

In ChatView.tsx, add search state:

```tsx
const [searchQuery, setSearchQuery] = useState('');
const [showSearch, setShowSearch] = useState(false);

const filteredMessages = useMemo(() => {
  if (!searchQuery.trim()) return messages;
  const q = searchQuery.toLowerCase();
  return messages.filter((m) =>
    m.content.toLowerCase().includes(q) ||
    m.toolCalls?.some(tc => tc.name.includes(q) || tc.output?.toLowerCase().includes(q))
  );
}, [messages, searchQuery]);
```

**Step 2: Add search bar UI**

Above the messages scroll div, add a collapsible search bar:

```tsx
{showSearch && (
  <div className="px-5 py-2 border-b border-border-subtle flex items-center gap-2">
    <svg width="14" height="14" viewBox="0 0 16 16" fill="none" stroke="var(--color-fg-muted)" strokeWidth="1.5">
      <circle cx="6.5" cy="6.5" r="5" /><path d="M11 11l3.5 3.5" />
    </svg>
    <input
      type="text"
      value={searchQuery}
      onChange={(e) => setSearchQuery(e.target.value)}
      placeholder="Search messages..."
      className="flex-1 bg-transparent text-sm text-fg placeholder:text-fg-muted focus:outline-none"
      autoFocus
    />
    <span className="text-[10px] text-fg-muted font-mono">
      {searchQuery ? `${filteredMessages.length}/${messages.length}` : ''}
    </span>
    <button type="button" onClick={() => { setShowSearch(false); setSearchQuery(''); }}
      className="text-fg-muted hover:text-fg cursor-pointer">
      <svg width="12" height="12" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round">
        <path d="M4 4l8 8M12 4l-8 8" />
      </svg>
    </button>
  </div>
)}
```

**Step 3: Add keyboard shortcut**

Add a global keydown listener in ChatView:

```tsx
useEffect(() => {
  const handler = (e: KeyboardEvent) => {
    if ((e.ctrlKey || e.metaKey) && e.key === 'f') {
      e.preventDefault();
      setShowSearch(true);
    }
    if (e.key === 'Escape' && showSearch) {
      setShowSearch(false);
      setSearchQuery('');
    }
  };
  document.addEventListener('keydown', handler);
  return () => document.removeEventListener('keydown', handler);
}, [showSearch]);
```

**Step 4: Use filteredMessages for rendering**

Replace messages.map with filteredMessages.map in the message list. Pass searchQuery to Message for highlighting:

```tsx
{filteredMessages.map((msg) => (
  <Message key={msg.id} message={msg} onRetry={retryFromMessage} onEdit={editMessage}
    isStreaming={isStreaming} searchQuery={searchQuery} />
))}
```

**Step 5: Highlight matches in Message**

In Message.tsx, add searchQuery to MessageProps and a highlightText helper:

```tsx
function highlightText(text: string, query: string): React.ReactNode {
  if (!query.trim()) return text;
  const escaped = query.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
  const parts = text.split(new RegExp(`(${escaped})`, 'gi'));
  return parts.map((part, i) =>
    part.toLowerCase() === query.toLowerCase()
      ? <mark key={i} className="bg-soul/30 text-fg rounded px-0.5">{part}</mark>
      : part
  );
}
```

Apply it to user messages when searchQuery is present (for user plain-text rendering as a wrapper around the text).

**Step 6: Build and verify**

```bash
cd web && npx vite build
```

Press Ctrl+F -- verify search bar appears. Type a query -- verify messages filter. Verify match count shows. Press Escape -- search closes.

**Step 7: Commit**

```bash
git add web/src/components/chat/ChatView.tsx web/src/components/chat/Message.tsx
git commit -m "feat(chat): conversation search with Ctrl+F, filtering, and match highlighting"
```

---

## Task 11: Drag-and-Drop Files

Allow dropping files anywhere in the chat area to attach them.

**Files:**
- Modify: `web/src/components/chat/ChatView.tsx`
- Modify: `web/src/components/chat/InputBar.tsx`

**Step 1: Add drop zone to ChatView**

In ChatView.tsx, add drag-and-drop handlers:

```tsx
const [isDragging, setIsDragging] = useState(false);
const [droppedFiles, setDroppedFiles] = useState<File[]>([]);

const handleDragOver = useCallback((e: React.DragEvent) => {
  e.preventDefault();
  setIsDragging(true);
}, []);

const handleDragLeave = useCallback((e: React.DragEvent) => {
  if (e.currentTarget === e.target) setIsDragging(false);
}, []);

const handleDrop = useCallback((e: React.DragEvent) => {
  e.preventDefault();
  setIsDragging(false);
  const files = Array.from(e.dataTransfer.files);
  if (files.length > 0) setDroppedFiles(files);
}, []);
```

Wrap the outer div with drag handlers and add a drop zone overlay:

```tsx
<div className="flex flex-col h-full relative"
  onDragOver={handleDragOver} onDragLeave={handleDragLeave} onDrop={handleDrop}>
  {isDragging && (
    <div className="absolute inset-0 z-50 flex items-center justify-center bg-deep/80 border-2 border-dashed border-soul rounded-lg">
      <span className="text-soul font-mono text-sm">Drop files to attach</span>
    </div>
  )}
```

**Step 2: Pass dropped files to InputBar**

```tsx
<InputBar onSend={handleSend} disabled={isStreaming}
  contextChip={contextChipProduct} onInjectContext={handleInjectContext} onDismissChip={handleDismissChip}
  droppedFiles={droppedFiles} onDroppedFilesConsumed={() => setDroppedFiles([])} />
```

In InputBar.tsx, add the new props:

```tsx
interface InputBarProps {
  // ...existing...
  droppedFiles?: File[];
  onDroppedFilesConsumed?: () => void;
}
```

Add an effect to consume dropped files:

```tsx
useEffect(() => {
  if (droppedFiles && droppedFiles.length > 0) {
    setFiles((prev) => [...prev, ...droppedFiles]);
    onDroppedFilesConsumed?.();
  }
}, [droppedFiles, onDroppedFilesConsumed]);
```

**Step 3: Build and verify**

```bash
cd web && npx vite build
```

Drag a file over the chat area -- verify overlay appears. Drop the file -- verify it appears as attachment in InputBar.

**Step 4: Commit**

```bash
git add web/src/components/chat/ChatView.tsx web/src/components/chat/InputBar.tsx
git commit -m "feat(chat): drag-and-drop file attachment"
```

---

## Task 12: Keyboard Shortcuts

Add essential keyboard shortcuts: Ctrl+K for quick command palette, Ctrl+Shift+N for new chat.

**Files:**
- Modify: `web/src/components/chat/ChatView.tsx`

**Step 1: Implement**

Extend the keydown handler from Task 10 (or add a new one if Task 10 was skipped):

```tsx
useEffect(() => {
  const handler = (e: KeyboardEvent) => {
    // Ctrl+F -- conversation search
    if ((e.ctrlKey || e.metaKey) && e.key === 'f') {
      e.preventDefault();
      setShowSearch(true);
    }
    // Escape -- close search
    if (e.key === 'Escape' && showSearch) {
      setShowSearch(false);
      setSearchQuery('');
    }
    // Ctrl+K -- focus input with slash for command palette
    if ((e.ctrlKey || e.metaKey) && e.key === 'k') {
      e.preventDefault();
      const textarea = document.querySelector('textarea[placeholder*="Message"]') as HTMLTextAreaElement;
      if (textarea) {
        textarea.focus();
        textarea.value = '/';
        textarea.dispatchEvent(new Event('input', { bubbles: true }));
      }
    }
    // Ctrl+Shift+N -- new chat
    if (e.key === 'n' && (e.ctrlKey || e.metaKey) && e.shiftKey) {
      e.preventDefault();
      setSessionId(null);
      setMessages([]);
    }
  };
  document.addEventListener('keydown', handler);
  return () => document.removeEventListener('keydown', handler);
}, [showSearch]);
```

Note: setMessages and setSessionId need to be destructured from useChat -- add them if not already.

**Step 2: Build and verify**

```bash
cd web && npx vite build
```

Press Ctrl+K -- verify input focuses with / prefix and slash palette opens. Press Ctrl+Shift+N -- verify new chat session starts.

**Step 3: Commit**

```bash
git add web/src/components/chat/ChatView.tsx
git commit -m "feat(chat): keyboard shortcuts -- Ctrl+K (commands), Ctrl+F (search), Ctrl+Shift+N (new chat)"
```

---

## Execution Order

These tasks are ordered by dependency:

1. **Task 1** (user markdown) -- standalone, no deps
2. **Task 2** (retry/edit) -- standalone, modifies Message + ChatView
3. **Task 3** (progress bar) -- standalone, ToolCall only
4. **Task 4** (diff rendering) -- new file + ToolCall
5. **Task 5** (file path links) -- ToolCall only
6. **Task 6** (streaming word count) -- ChatView only
7. **Task 7** (context window) -- backend + frontend
8. **Task 8** (mermaid) -- new dependency + new file + Message
9. **Task 9** (inline images) -- Message + ToolCall
10. **Task 10** (conversation search) -- ChatView + Message (depends on Task 2 for Message props)
11. **Task 11** (drag-and-drop) -- ChatView + InputBar
12. **Task 12** (keyboard shortcuts) -- ChatView (depends on Task 10 for search state)

Tasks 1-5 can be parallelized. Tasks 6-9 can be parallelized. Tasks 10-12 are sequential.

---

## Final Verification

After all tasks:

```bash
go build -o soul ./cmd/soul
cd web && npx vite build
```

Restart server. Full test checklist:
- User markdown: send bold and code -- renders styled
- Retry: hover user message, click retry icon -- resends
- Edit: hover user message, click edit icon -- inline textarea, modify, submit
- Progress bar: trigger long tool call -- bar fills
- Diff: trigger code_edit -- green/red lines in output
- File paths: click path in tool call -- copies to clipboard
- Streaming count: during response -- see word count incrementing
- Context pct: after response -- see X pct ctx in token badge
- Mermaid: ask for flowchart -- SVG renders inline
- Images: tool output with screenshot -- renders inline
- Search: Ctrl+F -- search bar, filtering, highlighting
- Drag-and-drop: drag file over chat -- drop zone, attach
- Shortcuts: Ctrl+K, Ctrl+F, Ctrl+Shift+N all work
