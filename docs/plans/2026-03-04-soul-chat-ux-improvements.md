# Soul Chat — Claude Code Parity Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Make Soul's chat experience match Claude Code quality: compact tool call UI, persistent session history, thinking block rendering, slash commands with skill loading, and clarification-first agent behavior.

**Architecture:** Six independent improvements layered on top of the existing WebSocket + AgentLoop + SQLite stack. Frontend changes are React/TypeScript in `web/src/`; backend changes are Go in `internal/`. No new external dependencies needed.

**Tech Stack:** Go 1.24, React 19, TypeScript, Tailwind CSS v4, SQLite (via planner store), WebSocket (nhooyr.io/websocket).

**Design doc:** `docs/plans/2026-03-04-soul-chat-claude-code-parity-design.md`

---

## Task 1: Compact Tool Call UI (ToolCall.tsx + Message.tsx)

**Problem:** Each tool call renders as a large expanded card (~80px). 8 tool calls = 640px, burying the text response.

**Goal:** One-line pill, collapsed by default. Text content always above tools.

**Files:**
- Modify: `web/src/components/chat/ToolCall.tsx`
- Modify: `web/src/components/chat/Message.tsx`

### Step 1: Redesign ToolCall.tsx as a compact pill

Replace the entire file with:

```tsx
// web/src/components/chat/ToolCall.tsx
import { useState } from 'react';
import type { ToolCallMessage } from '../../lib/types.ts';

interface ToolCallProps {
  toolCall: ToolCallMessage;
}

function briefSummary(toolCall: ToolCallMessage): string {
  if (toolCall.status === 'running') return 'running…';
  if (toolCall.status === 'error') return 'failed';
  const findings = toolCall.findings?.length ?? 0;
  if (findings > 0) return `${findings} issue${findings !== 1 ? 's' : ''}`;
  // Extract a short summary from output (first non-empty line, max 60 chars)
  if (toolCall.output) {
    const first = toolCall.output.split('\n').find(l => l.trim());
    if (first) return first.length > 60 ? first.slice(0, 57) + '…' : first;
  }
  return 'done';
}

export default function ToolCall({ toolCall }: ToolCallProps) {
  const [expanded, setExpanded] = useState(false);
  const isRunning = toolCall.status === 'running';
  const isError = toolCall.status === 'error';

  const statusIcon = isRunning ? '◌' : isError ? '✗' : '✓';
  const statusColor = isRunning
    ? 'text-fg-muted'
    : isError
    ? 'text-stage-blocked'
    : 'text-stage-done';

  const summary = briefSummary(toolCall);
  const hasDetails = !!(toolCall.output || (toolCall.findings?.length ?? 0) > 0);

  return (
    <div className="font-mono text-xs">
      <button
        type="button"
        onClick={() => hasDetails && setExpanded(!expanded)}
        className={`flex items-center gap-1.5 text-left w-full group py-0.5 ${hasDetails ? 'cursor-pointer' : 'cursor-default'}`}
      >
        <span className={`${statusColor} ${isRunning ? 'animate-pulse' : ''} shrink-0`}>
          {statusIcon}
        </span>
        <span className="text-fg-secondary">{toolCall.name}</span>
        <span className="text-fg-muted">·</span>
        <span className="text-fg-muted truncate flex-1">{summary}</span>
        {hasDetails && (
          <span className="text-fg-muted shrink-0 opacity-0 group-hover:opacity-100 transition-opacity text-[10px]">
            {expanded ? '▾' : '▸'}
          </span>
        )}
      </button>

      {expanded && hasDetails && (
        <div className="ml-4 mt-1 mb-1 max-h-60 overflow-y-auto rounded border border-border-subtle">
          {toolCall.output && (
            <pre className="p-2 text-fg-muted text-[11px] whitespace-pre-wrap leading-relaxed">
              {toolCall.output.length > 2000
                ? toolCall.output.slice(0, 2000) + '\n… (truncated)'
                : toolCall.output}
            </pre>
          )}
          {(toolCall.findings?.length ?? 0) > 0 && (
            <div className="p-2 space-y-0.5">
              {toolCall.findings!.map((f) => (
                <div key={f.id} className="flex items-center gap-2 text-[11px]">
                  <span className={`shrink-0 px-1 rounded text-[9px] uppercase font-medium ${
                    f.severity === 'critical' || f.severity === 'high'
                      ? 'bg-stage-blocked/20 text-stage-blocked'
                      : f.severity === 'medium'
                      ? 'bg-stage-validation/20 text-stage-validation'
                      : 'bg-overlay text-fg-muted'
                  }`}>{f.severity}</span>
                  <span className="text-fg flex-1 truncate">{f.title}</span>
                  {f.file && (
                    <span className="text-fg-muted shrink-0">
                      {f.file}{f.line != null ? `:${f.line}` : ''}
                    </span>
                  )}
                </div>
              ))}
            </div>
          )}
        </div>
      )}
    </div>
  );
}
```

### Step 2: Update Message.tsx — text first, tool pills as a tight group below

```tsx
// web/src/components/chat/Message.tsx
import { useState } from 'react';
import ReactMarkdown from 'react-markdown';
import remarkGfm from 'remark-gfm';
import type { ChatMessage, ToolCallMessage } from '../../lib/types.ts';
import ToolCall from './ToolCall.tsx';

interface MessageProps {
  message: ChatMessage;
}

function ToolCallGroup({ toolCalls }: { toolCalls: ToolCallMessage[] }) {
  const [groupExpanded, setGroupExpanded] = useState(true);
  const allDone = toolCalls.every(tc => tc.status !== 'running');
  const runningCount = toolCalls.filter(tc => tc.status === 'running').length;
  const errorCount = toolCalls.filter(tc => tc.status === 'error').length;

  if (toolCalls.length === 1) {
    return (
      <div className="mt-2 pl-1 border-l border-border-subtle">
        <ToolCall toolCall={toolCalls[0]} />
      </div>
    );
  }

  const groupLabel = !allDone
    ? `${runningCount} running…`
    : errorCount > 0
    ? `${toolCalls.length} calls (${errorCount} failed)`
    : `${toolCalls.length} tool calls`;

  return (
    <div className="mt-2 pl-1 border-l border-border-subtle">
      <button
        type="button"
        onClick={() => setGroupExpanded(!groupExpanded)}
        className="flex items-center gap-1.5 text-xs text-fg-muted hover:text-fg transition-colors cursor-pointer py-0.5 font-mono"
      >
        <span className={!allDone ? 'animate-pulse text-fg-muted' : errorCount > 0 ? 'text-stage-blocked' : 'text-stage-done'}>
          {!allDone ? '◌' : errorCount > 0 ? '✗' : '✓'}
        </span>
        <span>{groupLabel}</span>
        <span className="text-[10px]">{groupExpanded ? '▾' : '▸'}</span>
      </button>
      {groupExpanded && (
        <div className="ml-3 mt-0.5 space-y-0.5">
          {toolCalls.map(tc => <ToolCall key={tc.id} toolCall={tc} />)}
        </div>
      )}
    </div>
  );
}

export default function Message({ message }: MessageProps) {
  const isUser = message.role === 'user';
  const hasTools = (message.toolCalls?.length ?? 0) > 0;
  const hasText = !!message.content;

  return (
    <div className={`flex ${isUser ? 'justify-end' : 'justify-start'} animate-fade-in`}>
      <div className={`max-w-[80%] px-4 py-3 ${
        isUser
          ? 'bg-elevated border-l-2 border-soul/40 text-fg rounded-2xl rounded-br-md'
          : 'text-fg rounded-2xl rounded-bl-md'
      }`}>
        {/* Text content always first */}
        {hasText && (
          isUser ? (
            <div className="whitespace-pre-wrap break-words text-sm leading-relaxed">
              {message.content}
            </div>
          ) : (
            <div className="prose prose-sm prose-soul max-w-none break-words">
              <ReactMarkdown remarkPlugins={[remarkGfm]}>{message.content}</ReactMarkdown>
            </div>
          )
        )}

        {/* Tool calls as compact group below text */}
        {hasTools && <ToolCallGroup toolCalls={message.toolCalls!} />}
      </div>
    </div>
  );
}
```

### Step 3: Build frontend and verify

```bash
cd /home/rishav/soul/web && npx vite build 2>&1 | tail -5
```

Expected: `✓ built in Xs`

### Step 4: Visual test via titan-pc screenshot

```bash
# From titan-pi, run the playwright script at /tmp/soul-test/test2.js again
ssh titan-pc "node /tmp/soul-test/test2.js 2>&1" && scp titan-pc:/tmp/soul-with-tools.png /tmp/soul-after-task1.png
```

Verify: tool calls are now one-line pills, not large cards.

### Step 5: Rebuild and restart Soul

```bash
cd /home/rishav/soul && go build -o soul ./cmd/soul && kill $(lsof -ti :3000) 2>/dev/null; sleep 1; SOUL_HOST=0.0.0.0 ./soul serve >> /tmp/soul.log 2>&1 &
```

### Step 6: Commit

```bash
cd /home/rishav/soul && git add web/src/components/chat/ToolCall.tsx web/src/components/chat/Message.tsx && git commit -m "feat: compact tool call pills — collapsed one-line chips instead of expanded cards"
```

---

## Task 2: Session History Persistence

**Problem:** Messages stored only in-memory `session.Store`. Restart loses all history. Session switching shows empty chat.

**Goal:** Every message written to SQLite. Session resume loads full history into agent context.

**Files:**
- Modify: `internal/server/ws.go` — resolve DB session ID, persist messages after agent run
- Modify: `internal/server/agent.go` — accept pre-loaded history; load from DB on session resume
- Modify: `web/src/hooks/useChat.ts` — use DB session ID (integer), hydrate on session switch

### Step 1: Add session ID resolution to ws.go handleChatSend

The WS message now carries either a string UUID (new session) or a stringified integer (existing DB session). Resolve to a DB session.

In `internal/server/ws.go`, replace `handleChatSend`:

```go
func (s *Server) handleChatSend(ctx context.Context, conn *websocket.Conn, msg *WSMessage) {
    // Resolve DB session ID: numeric string = existing, anything else = create new.
    var dbSessionID int64
    if id, err := strconv.ParseInt(msg.SessionID, 10, 64); err == nil && id > 0 {
        dbSessionID = id
        // Update session status to running.
        if s.planner != nil {
            _ = s.planner.UpdateSessionStatus(dbSessionID, "running")
        }
    } else if s.planner != nil {
        // Create new DB session, title from first 100 chars of message.
        title := msg.Content
        if len(title) > 100 { title = title[:100] }
        sess, err := s.planner.CreateSession(title)
        if err == nil {
            dbSessionID = sess.ID
        }
        // Tell the client the new session ID so it can switch to it.
        idData, _ := json.Marshal(map[string]any{"session_id": dbSessionID})
        sendEvent(WSMessage{Type: "session.created", Data: idData})
    }

    // Build sendEvent callback.
    sendEvent := func(wsMsg WSMessage) {
        if err := wsjson.Write(ctx, conn, wsMsg); err != nil {
            log.Printf("[ws] write error type=%s: %v", wsMsg.Type, err)
        }
    }

    // ... existing model/opts parsing ...

    // Load prior history for this session.
    var priorMessages []ai.Message
    if dbSessionID > 0 && s.planner != nil {
        records, _ := s.planner.GetSessionMessages(dbSessionID)
        for _, r := range records {
            priorMessages = append(priorMessages, ai.Message{Role: r.Role, Content: r.Content})
        }
        // Truncate to last 50 messages to stay within context limits.
        if len(priorMessages) > 50 {
            priorMessages = priorMessages[len(priorMessages)-50:]
        }
    }

    // Persist user message.
    if dbSessionID > 0 && s.planner != nil {
        _ = s.planner.AddMessage(dbSessionID, "user", msg.Content)
    }

    // Run agent, capture full response for persistence.
    var fullResponse strings.Builder
    wrappedSendEvent := func(wsMsg WSMessage) {
        if wsMsg.Type == "chat.token" {
            fullResponse.WriteString(wsMsg.Content)
        }
        sendEvent(wsMsg)
    }

    agent := NewAgentLoop(s.ai, s.products, s.sessions, s.planner, s.broadcast, model, s.projectRoot)
    agent.RunWithHistory(ctx, fmt.Sprintf("db-%d", dbSessionID), msg.Content, opts.ChatType, opts.DisabledTools, opts.Thinking, priorMessages, wrappedSendEvent)

    // Persist assistant response.
    if dbSessionID > 0 && s.planner != nil && fullResponse.Len() > 0 {
        _ = s.planner.AddMessage(dbSessionID, "assistant", fullResponse.String())
        _ = s.planner.UpdateSessionStatus(dbSessionID, "idle")
    }

    log.Printf("[ws] chat.send complete session=%s db_session=%d", msg.SessionID, dbSessionID)
}
```

Note: `sendEvent` is defined before `dbSessionID` — move the declaration before the `if` block. Also needs `"strconv"` and `"strings"` imports.

### Step 2: Add RunWithHistory to agent.go

In `internal/server/agent.go`, add a new method that accepts pre-loaded history:

```go
// RunWithHistory is like Run but accepts pre-loaded message history to inject
// before the new user message. Used for session resume.
func (a *AgentLoop) RunWithHistory(
    ctx context.Context,
    sessionID, userMessage, chatType string,
    disabledTools []string, thinking bool,
    history []ai.Message,
    sendEvent func(WSMessage),
) {
    // Same as Run but seed the session with history before adding the new user message.
    if a.ai == nil {
        sendEvent(WSMessage{Type: "chat.token", SessionID: sessionID, Content: "AI not configured."})
        sendEvent(WSMessage{Type: "chat.done", SessionID: sessionID})
        return
    }

    log.Printf("[agent] run-with-history session=%s history=%d msg=%q", sessionID, len(history), userMessage)

    sess := a.sessions.GetOrCreate(sessionID)

    // Seed from DB history if session is empty (fresh reconnect).
    if len(sess.Messages) == 0 && len(history) > 0 {
        for _, h := range history {
            if content, ok := h.Content.(string); ok {
                sess.AddMessage(h.Role, content)
            }
        }
    }
    sess.AddMessage("user", userMessage)

    // Delegate to the existing agent loop logic.
    // (Refactor: extract the core loop from Run into runLoop, call it here.)
    // For now, just call Run with the seeded session.
    a.runLoop(ctx, sessionID, chatType, disabledTools, thinking, sendEvent)
}
```

Also refactor `Run` to extract `runLoop` (the tool-calling loop), so both `Run` and `RunWithHistory` share it. The refactor is:

```go
func (a *AgentLoop) Run(ctx context.Context, sessionID, userMessage, chatType string, disabledTools []string, thinking bool, sendEvent func(WSMessage)) {
    // ... validation ...
    sess := a.sessions.GetOrCreate(sessionID)
    sess.AddMessage("user", userMessage)
    a.runLoop(ctx, sessionID, chatType, disabledTools, thinking, sendEvent)
}

func (a *AgentLoop) runLoop(ctx context.Context, sessionID, chatType string, disabledTools []string, thinking bool, sendEvent func(WSMessage)) {
    // ... everything from "Build claude tools" to "Signal completion" ...
}
```

### Step 3: Update useChat.ts — use DB integer session ID

```typescript
// In useChat.ts:
// Replace sessionIdRef (random UUID) with a state that starts null,
// creates a DB session on first send, and accepts session switch.

export function useChat(initialSessionId?: number) {
  const { send, onMessage, connected } = useWebSocket();
  const [messages, setMessages] = useState<ChatMessage[]>([]);
  const [isStreaming, setIsStreaming] = useState(false);
  const [sessionId, setSessionId] = useState<number | null>(initialSessionId ?? null);

  // Handle session.created event from server (server assigns DB ID for new sessions).
  // Handle session switching: when sessionId changes, load messages from API.
  useEffect(() => {
    if (!initialSessionId) return;
    fetch(`/api/sessions/${initialSessionId}/messages`)
      .then(r => r.json())
      .then((records: { role: string; content: string; id: number }[]) => {
        const hydrated: ChatMessage[] = records.map(r => ({
          id: String(r.id),
          role: r.role as 'user' | 'assistant',
          content: r.content,
          timestamp: new Date(),
        }));
        setMessages(hydrated);
      })
      .catch(() => {});
  }, [initialSessionId]);

  // In sendMessage: use sessionId as string (or 'new' if null).
  const sendMessage = useCallback((content: string, options?: SendOptions) => {
    // ... add user message to local state ...
    send({
      type: 'chat.message',
      session_id: sessionId ? String(sessionId) : 'new',
      content,
      data: options,
    });
  }, [send, sessionId]);

  // Listen for session.created to capture new session ID.
  // (Add case in the onMessage switch):
  // case 'session.created': setSessionId(msg.data.session_id); break;

  return { messages, sendMessage, isStreaming, connected, sessionId, setSessionId };
}
```

### Step 4: Go build check

```bash
cd /home/rishav/soul && go build ./... 2>&1
```

Expected: no errors.

### Step 5: Manual test

1. Send a message, note the session ID logged in `/tmp/soul.log`
2. Restart Soul: `kill $(lsof -ti :3000) && SOUL_HOST=0.0.0.0 ./soul serve >> /tmp/soul.log 2>&1 &`
3. Open Soul, switch to the previous session
4. Verify: old messages appear, new message has full context

### Step 6: Commit

```bash
cd /home/rishav/soul && git add internal/server/ws.go internal/server/agent.go web/src/hooks/useChat.ts && git commit -m "feat: persist chat messages to SQLite, resume full context on session switch"
```

---

## Task 3: Thinking Block UI

**Problem:** Extended thinking tokens are consumed but never shown. `processStream` ignores `thinking` content blocks.

**Goal:** Stream thinking content to client, render as collapsible "Thinking" block above response.

**Files:**
- Modify: `internal/server/agent.go` — emit `chat.thinking` events for thinking blocks
- Modify: `web/src/hooks/useChat.ts` — accumulate `thinkingContent` per message
- Modify: `web/src/lib/types.ts` — add `thinking?: string` to ChatMessage
- Modify: `web/src/components/chat/Message.tsx` — render ThinkingBlock above text

### Step 1: Emit chat.thinking events from processStream in agent.go

In `processStream`, the `content_block_start` handler currently only tracks `"text"` and `"tool_use"` blocks. Add `"thinking"`:

```go
// In the content_block_start case:
currentBlock = wrapper.ContentBlock.Type
if currentBlock == "tool_use" {
    currentToolID = wrapper.ContentBlock.ID
    currentTool = wrapper.ContentBlock.Name
    toolInputBuf.Reset()
} else if currentBlock == "thinking" {
    thinkingBuf.Reset()  // add thinkingBuf strings.Builder to processStream vars
}

// In the content_block_delta case, add:
case "thinking_delta":
    thinkingBuf.WriteString(wrapper.Delta.Thinking)
    // Stream thinking tokens to client.
    sendEvent(WSMessage{
        Type:      "chat.thinking",
        SessionID: sessionID,
        Content:   wrapper.Delta.Thinking,
    })
```

Also update `ContentBlockDelta` struct in `internal/ai/client.go` to include:

```go
type ContentBlockDelta struct {
    Type        string `json:"type"`
    Text        string `json:"text"`
    PartialJSON string `json:"partial_json"`
    Thinking    string `json:"thinking"`  // ADD THIS
}
```

### Step 2: Add thinking to ChatMessage type

```typescript
// In web/src/lib/types.ts, add to ChatMessage:
thinking?: string;  // accumulated thinking block content
```

### Step 3: Accumulate thinking in useChat.ts

```typescript
// In the onMessage switch, add case:
case 'chat.thinking': {
  const token = msg.content ?? '';
  setMessages(prev => {
    const last = prev[prev.length - 1];
    if (last?.role === 'assistant') {
      return [...prev.slice(0, -1), { ...last, thinking: (last.thinking ?? '') + token }];
    }
    return [...prev, {
      id: uuid(), role: 'assistant', content: '',
      thinking: token, toolCalls: [], timestamp: new Date(),
    }];
  });
  break;
}
```

### Step 4: Render ThinkingBlock in Message.tsx

Add a `ThinkingBlock` component and render it before text content:

```tsx
function ThinkingBlock({ content }: { content: string }) {
  const [expanded, setExpanded] = useState(false);
  const lines = content.split('\n').length;
  return (
    <div className="mb-3">
      <button
        type="button"
        onClick={() => setExpanded(!expanded)}
        className="flex items-center gap-1.5 text-xs text-fg-muted hover:text-fg transition-colors cursor-pointer font-mono"
      >
        <span className="text-soul">💭</span>
        <span>Thinking ({lines} lines)</span>
        <span className="text-[10px]">{expanded ? '▾' : '▸'}</span>
      </button>
      {expanded && (
        <div className="mt-1 p-3 rounded border border-border-subtle bg-elevated/40 max-h-48 overflow-y-auto">
          <pre className="text-xs text-fg-muted font-mono whitespace-pre-wrap leading-relaxed">
            {content}
          </pre>
        </div>
      )}
    </div>
  );
}

// In Message component, add before text content:
{message.thinking && <ThinkingBlock content={message.thinking} />}
```

### Step 5: Build and verify

```bash
cd /home/rishav/soul && go build ./... && cd web && npx vite build 2>&1 | tail -3
```

### Step 6: Commit

```bash
cd /home/rishav/soul && git add internal/server/agent.go internal/ai/client.go web/src/lib/types.ts web/src/hooks/useChat.ts web/src/components/chat/Message.tsx && git commit -m "feat: stream and render thinking blocks from extended thinking"
```

---

## Task 4: Skills Loader (Backend)

**Problem:** Soul doesn't know about `~/.claude/plugins/` skill files. The superpowers behaviors are not available.

**Goal:** Load skill files at startup, inject active skill content into agent system prompt.

**Files:**
- Create: `internal/skills/loader.go`
- Modify: `internal/server/server.go` — init SkillStore
- Modify: `internal/server/agent.go` — accept and inject skill content into system prompt
- Modify: `internal/server/ws.go` — pass skill content from chatOptions to agent

### Step 1: Create internal/skills/loader.go

```go
package skills

import (
    "os"
    "path/filepath"
    "strings"
)

// Skill represents a loaded skill file.
type Skill struct {
    Name    string
    Content string
}

// Store holds all loaded skills indexed by name (lowercase).
type Store struct {
    skills map[string]Skill
}

// Load scans ~/.claude/plugins/cache for SKILL.md files and indexes them.
func Load() *Store {
    store := &Store{skills: make(map[string]Skill)}

    home, err := os.UserHomeDir()
    if err != nil {
        return store
    }

    cacheDir := filepath.Join(home, ".claude", "plugins", "cache")
    // Pattern: cache/<plugin>/<version>/skills/<name>/SKILL.md
    pattern := filepath.Join(cacheDir, "*", "*", "skills", "*", "SKILL.md")
    matches, _ := filepath.Glob(pattern)

    seen := make(map[string]bool)
    for _, path := range matches {
        // Skill name is the directory containing SKILL.md.
        name := strings.ToLower(filepath.Base(filepath.Dir(path)))
        if seen[name] {
            continue // take first (newest by glob order)
        }
        data, err := os.ReadFile(path)
        if err != nil {
            continue
        }
        store.skills[name] = Skill{Name: name, Content: string(data)}
        seen[name] = true
    }

    return store
}

// Get returns skill content by name. Returns ("", false) if not found.
func (s *Store) Get(name string) (string, bool) {
    skill, ok := s.skills[strings.ToLower(name)]
    return skill.Content, ok
}

// Names returns all loaded skill names.
func (s *Store) Names() []string {
    names := make([]string, 0, len(s.skills))
    for n := range s.skills {
        names = append(names, n)
    }
    return names
}
```

### Step 2: Init SkillStore in server.go

In `internal/server/server.go`, add `skillStore *skills.Store` field to `Server` struct and initialize in `New()`:

```go
import "github.com/rishav1305/soul/internal/skills"

// In Server struct:
skillStore *skills.Store

// In New() or serve startup:
srv.skillStore = skills.Load()
log.Printf("[skills] loaded %d skills: %v", len(srv.skillStore.Names()), srv.skillStore.Names())
```

### Step 3: Pass skill content through chatOptions

In `internal/server/ws.go`, add `SkillContent string` to `chatOptions`:

```go
type chatOptions struct {
    Model         string   `json:"model"`
    ChatType      string   `json:"chatType"`
    DisabledTools []string `json:"disabledTools"`
    Thinking      bool     `json:"thinking"`
    SkillContent  string   `json:"skillContent"` // ADD
}
```

In `handleChatSend`, after parsing opts, also resolve skill content from the server's store if not provided:

```go
// If client didn't send skillContent, try loading from chatType name.
if opts.SkillContent == "" && s.skillStore != nil && opts.ChatType != "" {
    if content, ok := s.skillStore.Get(opts.ChatType); ok {
        opts.SkillContent = content
    }
}
```

### Step 4: Inject skill content into agent system prompt

In `internal/server/agent.go`, update `Run`/`runLoop` signature to accept `skillContent string` and append it to the system prompt:

```go
if skillContent != "" {
    sysPrompt += "\n\n---\n# Active Skill\n\n" + skillContent
}
```

### Step 5: Build check

```bash
cd /home/rishav/soul && go build ./... 2>&1
```

### Step 6: Verify skill loading in logs

```bash
grep "skills\]" /tmp/soul.log | head -5
```

Expected: `[skills] loaded N skills: [brainstorming commit ...]`

### Step 7: Commit

```bash
cd /home/rishav/soul && git add internal/skills/ internal/server/server.go internal/server/agent.go internal/server/ws.go && git commit -m "feat: load ~/.claude/plugins skills at startup, inject into agent system prompt"
```

---

## Task 5: Slash Commands (Frontend)

**Problem:** No `/command` support. Users can't invoke skills or built-in commands from chat.

**Goal:** Type `/` → autocomplete palette appears. Select command → routes to skill or built-in behavior.

**Files:**
- Modify: `web/src/components/chat/InputBar.tsx` — add /command detection, palette UI
- Create: `web/src/hooks/useSlashCommands.ts` — command registry fetched from server
- Modify: `internal/server/routes.go` + new handler — `GET /api/skills` endpoint

### Step 1: Add GET /api/skills endpoint

In `internal/server/routes.go`:
```go
s.mux.HandleFunc("GET /api/skills", s.handleSkillsList)
```

New handler in a new file `internal/server/skills_handler.go`:
```go
package server

import "net/http"

func (s *Server) handleSkillsList(w http.ResponseWriter, r *http.Request) {
    if s.skillStore == nil {
        writeJSON(w, http.StatusOK, []any{})
        return
    }
    type skillInfo struct {
        Name string `json:"name"`
    }
    var result []skillInfo
    for _, name := range s.skillStore.Names() {
        result = append(result, skillInfo{Name: name})
    }
    writeJSON(w, http.StatusOK, result)
}
```

### Step 2: Create useSlashCommands.ts hook

```typescript
// web/src/hooks/useSlashCommands.ts
import { useState, useEffect } from 'react';

export interface SlashCommand {
  name: string;       // e.g. "brainstorming"
  description: string;
  chatType?: string;  // maps to chatType for the message
  builtin?: boolean;
}

const BUILTIN_COMMANDS: SlashCommand[] = [
  { name: 'new', description: 'Start a new chat session', builtin: true },
  { name: 'clear', description: 'Clear current chat history', builtin: true },
  { name: 'think', description: 'Toggle extended thinking', builtin: true },
  { name: 'brainstorm', description: 'Brainstorm mode — clarify before implementing', chatType: 'brainstorm' },
  { name: 'code', description: 'Code generation mode', chatType: 'code' },
  { name: 'debug', description: 'Debug mode — systematic root cause analysis', chatType: 'debug' },
  { name: 'review', description: 'Code review mode', chatType: 'review' },
  { name: 'tdd', description: 'Test-driven development mode', chatType: 'tdd' },
];

export function useSlashCommands() {
  const [commands, setCommands] = useState<SlashCommand[]>(BUILTIN_COMMANDS);

  useEffect(() => {
    fetch('/api/skills')
      .then(r => r.json())
      .then((skills: { name: string }[]) => {
        const skillCommands: SlashCommand[] = skills
          .filter(s => !BUILTIN_COMMANDS.find(b => b.name === s.name))
          .map(s => ({ name: s.name, description: `${s.name} skill`, chatType: s.name }));
        setCommands([...BUILTIN_COMMANDS, ...skillCommands]);
      })
      .catch(() => {});
  }, []);

  return commands;
}
```

### Step 3: Add slash command parsing and palette to InputBar.tsx

Add to InputBar state:
```typescript
const [slashQuery, setSlashQuery] = useState('');
const [showSlashPalette, setShowSlashPalette] = useState(false);
const [paletteIndex, setPaletteIndex] = useState(0);
const commands = useSlashCommands();
```

Update `handleChange` to detect `/` prefix:
```typescript
const handleChange = useCallback((e: React.ChangeEvent<HTMLTextAreaElement>) => {
  const val = e.target.value;
  setValue(val);
  // Slash command detection: only on first word starting with /
  if (val.startsWith('/') && !val.includes(' ')) {
    setSlashQuery(val.slice(1).toLowerCase());
    setShowSlashPalette(true);
    setPaletteIndex(0);
  } else {
    setShowSlashPalette(false);
  }
  // ... height auto ...
}, []);
```

Filter commands in render:
```typescript
const filteredCommands = commands.filter(c =>
  c.name.toLowerCase().startsWith(slashQuery)
);
```

Add palette UI above the textarea (inside the rounded container):
```tsx
{showSlashPalette && filteredCommands.length > 0 && (
  <div className="absolute bottom-full left-0 right-0 mb-1 bg-surface border border-border-default rounded-xl shadow-xl z-50 overflow-hidden max-h-64 overflow-y-auto">
    <div className="px-3 py-1.5 text-[10px] font-display uppercase tracking-widest text-fg-muted border-b border-border-subtle">
      Commands
    </div>
    {filteredCommands.map((cmd, i) => (
      <button
        key={cmd.name}
        type="button"
        onMouseDown={(e) => { e.preventDefault(); selectCommand(cmd); }}
        className={`w-full flex items-center gap-3 px-3 py-2 text-left transition-colors cursor-pointer ${
          i === paletteIndex ? 'bg-elevated' : 'hover:bg-elevated/50'
        }`}
      >
        <span className="font-mono text-soul text-sm">/{cmd.name}</span>
        <span className="text-xs text-fg-muted flex-1">{cmd.description}</span>
      </button>
    ))}
  </div>
)}
```

Add `selectCommand` handler:
```typescript
const selectCommand = useCallback((cmd: SlashCommand) => {
  setShowSlashPalette(false);
  if (cmd.builtin) {
    handleBuiltinCommand(cmd.name);
    setValue('');
    return;
  }
  if (cmd.chatType) setChatType(cmd.chatType.charAt(0).toUpperCase() + cmd.chatType.slice(1));
  setValue(''); // Clear /command, user types their message next
  textareaRef.current?.focus();
}, []);
```

Add keyboard navigation to `handleKeyDown`:
```typescript
if (showSlashPalette) {
  if (e.key === 'ArrowDown') { e.preventDefault(); setPaletteIndex(i => Math.min(i+1, filteredCommands.length-1)); return; }
  if (e.key === 'ArrowUp') { e.preventDefault(); setPaletteIndex(i => Math.max(i-1, 0)); return; }
  if (e.key === 'Enter') { e.preventDefault(); if (filteredCommands[paletteIndex]) selectCommand(filteredCommands[paletteIndex]); return; }
  if (e.key === 'Escape') { setShowSlashPalette(false); return; }
  if (e.key === 'Tab') { e.preventDefault(); if (filteredCommands[paletteIndex]) selectCommand(filteredCommands[paletteIndex]); return; }
}
```

### Step 4: Build and verify

```bash
cd /home/rishav/soul/web && npx vite build 2>&1 | tail -3
```

Screenshot test: type `/` in Soul chat, verify palette appears with commands listed.

### Step 5: Commit

```bash
cd /home/rishav/soul && git add web/src/components/chat/InputBar.tsx web/src/hooks/useSlashCommands.ts internal/server/skills_handler.go internal/server/routes.go && git commit -m "feat: slash command palette — /command autocomplete with skill routing"
```

---

## Task 6: Clarification-First Agent Behavior

**Problem:** Agent implements immediately without asking questions. Claude Code asks clarifying questions first.

**Goal:** In brainstorm/clarify modes, agent asks ≥1 clarifying question before writing any code or creating tasks.

**Files:**
- Modify: `internal/server/agent.go` — expand chatTypePrompt for brainstorm/clarify
- Modify: `internal/server/autonomous.go` — keep autonomous pipeline unchanged (it should still execute directly)

### Step 1: Expand chatTypePrompt in agent.go

Replace the `brainstorm` and add `clarify` cases:

```go
case "brainstorm":
    return `

# Mode: Brainstorm — Clarify Before Acting

You are in brainstorming mode. Your ONLY job right now is to understand what the user wants to build.

**Rules:**
- NEVER write code, create files, or create tasks until the user has answered at least one clarifying question.
- Ask ONE focused question per response. Not a list — just one.
- Prefer multiple-choice questions over open-ended when possible.
- After each answer, ask the next question OR present 2-3 approaches with trade-offs.
- Only move to implementation AFTER the user explicitly approves an approach.
- Use YAGNI: remove unnecessary features from all designs.

**Question sequence:**
1. What is the core purpose / who is the user?
2. What are the constraints? (tech stack, timeline, scale)
3. What does success look like? (acceptance criteria)

Begin by understanding the request, then ask your first clarifying question.`

case "clarify":
    return `

# Mode: Clarify

Before taking any action, ask one clarifying question to understand the user's intent.
After they answer, proceed with the most sensible interpretation.
Do NOT ask more than 2 questions before acting.`
```

### Step 2: Add "Clarify" to CHAT_TYPES in InputBar.tsx

```typescript
const CHAT_TYPES = [
  { value: 'Chat', label: 'Chat', group: 'mode' },
  { value: 'Code', label: 'Code', group: 'mode' },
  { value: 'Architect', label: 'Architect', group: 'mode' },
  { value: 'Clarify', label: 'Clarify', group: 'mode' },  // ADD
  { value: 'Debug', label: 'Debug', group: 'skill' },
  // ...
] as const;
```

### Step 3: Update placeholder text when clarify/brainstorm mode is active

In InputBar.tsx, make placeholder dynamic:
```typescript
const placeholder = chatType === 'Brainstorm' || chatType === 'Clarify'
  ? 'Describe what you want to build…'
  : 'Message Soul…';
```

Apply to textarea: `placeholder={placeholder}`

### Step 4: Build, restart, and test

```bash
cd /home/rishav/soul && go build -o soul ./cmd/soul && cd web && npx vite build 2>&1 | tail -3
cd /home/rishav/soul && kill $(lsof -ti :3000) 2>/dev/null; sleep 1; SOUL_HOST=0.0.0.0 ./soul serve >> /tmp/soul.log 2>&1 &
```

Test: switch to "Brainstorm" mode, type "build me a dashboard". Verify Soul asks a clarifying question instead of immediately generating code.

### Step 5: Commit

```bash
cd /home/rishav/soul && git add internal/server/agent.go web/src/components/chat/InputBar.tsx && git commit -m "feat: clarification-first agent in brainstorm/clarify modes"
```

---

## Final Integration Test

After all 6 tasks:

```bash
# Screenshot full chat flow with tools
ssh titan-pc "cd /tmp/soul-test && node test2.js" && scp titan-pc:/tmp/soul-with-tools.png /tmp/soul-final.png
```

Verify checklist:
- [ ] Tool calls are one-line pills, not cards
- [ ] Text response appears above tool pills
- [ ] Thinking block appears (collapsible) when Opus + thinking enabled
- [ ] Session history survives restart
- [ ] `/brainstorming` opens palette and sets brainstorm mode
- [ ] Brainstorm mode asks clarifying question, not immediate code

## Push to Remotes

```bash
cd /home/rishav/soul && git push origin master && git push github master
```
