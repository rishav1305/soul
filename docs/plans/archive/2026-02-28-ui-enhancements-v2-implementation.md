# UI Enhancements v2 Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Implement 10 UI improvements: multi-model + extended thinking, hybrid chat types, paste-to-upload, voice input, navbar logo, SoulRail/SoulPanel, chat visibility, TaskPanel icons, responsive filters, VS Code panel icons.

**Architecture:** Backend gains model override + thinking + chatType support in WS handler. Frontend adds SoulRail/SoulPanel as a third column in AppShell. InputBar gains voice input (Web Speech API) and paste-to-upload. All panel toggle icons change to VS Code sidebar style.

**Tech Stack:** Go 1.24, React 19, TypeScript, Tailwind CSS v4, Web Speech API (Chrome), nhooyr.io/websocket

---

### Task 1: Backend — Multi-Model + Extended Thinking + ChatType Wire-Up

This task wires the backend to actually use the `model`, `chatType`, `disabledTools`, and `thinking` fields that the frontend sends in `msg.Data`.

**Files:**
- Modify: `internal/server/routes.go:204-209` — return all 3 models
- Modify: `internal/server/ws.go:66-86` — parse msg.Data, pass options to agent
- Modify: `internal/server/agent.go:50-65,83,101,119-124` — accept options, vary prompt, filter tools, set thinking
- Modify: `internal/ai/client.go:64-71` — add Thinking field to Request

**Step 1: Update /api/models to return all 3 models**

In `internal/server/routes.go`, replace `handleModelsList`:

```go
func (s *Server) handleModelsList(w http.ResponseWriter, r *http.Request) {
	models := []modelInfo{
		{ID: "claude-sonnet-4-6", Name: "Sonnet", Description: "Fast & capable"},
		{ID: "claude-opus-4-6", Name: "Opus", Description: "Most capable"},
		{ID: "claude-haiku-4-5-20251001", Name: "Haiku", Description: "Fast & lightweight"},
	}
	writeJSON(w, http.StatusOK, models)
}
```

**Step 2: Add Thinking field to ai.Request**

In `internal/ai/client.go`, update the `Request` struct:

```go
type ThinkingConfig struct {
	Type        string `json:"type"`
	BudgetTokens int   `json:"budget_tokens"`
}

type Request struct {
	Model     string          `json:"model"`
	MaxTokens int             `json:"max_tokens"`
	System    string          `json:"system,omitempty"`
	Messages  []Message       `json:"messages"`
	Tools     []ClaudeTool    `json:"tools,omitempty"`
	Stream    bool            `json:"stream"`
	Thinking  *ThinkingConfig `json:"thinking,omitempty"`
}
```

**Step 3: Add chatOptions struct and parse msg.Data in ws.go**

In `internal/server/ws.go`, add a struct and update `handleChatSend`:

```go
// chatOptions are parsed from the WebSocket message's Data field.
type chatOptions struct {
	Model         string   `json:"model"`
	ChatType      string   `json:"chatType"`
	DisabledTools []string `json:"disabledTools"`
	Thinking      bool     `json:"thinking"`
}

func (s *Server) handleChatSend(ctx context.Context, conn *websocket.Conn, msg *WSMessage) {
	sessionID := msg.SessionID
	if sessionID == "" {
		sessionID = "default"
	}

	log.Printf("[ws] chat.send session=%s content=%q", sessionID, msg.Content)

	// Parse options from msg.Data.
	var opts chatOptions
	if len(msg.Data) > 0 {
		_ = json.Unmarshal(msg.Data, &opts)
	}

	sendEvent := func(wsMsg WSMessage) {
		if err := wsjson.Write(ctx, conn, wsMsg); err != nil {
			log.Printf("[ws] write error type=%s: %v", wsMsg.Type, err)
		}
	}

	// Use model from options, fall back to server config.
	model := s.cfg.Model
	if opts.Model != "" {
		model = opts.Model
	}

	agent := NewAgentLoop(s.ai, s.products, s.sessions, model)
	agent.Run(ctx, sessionID, msg.Content, opts.ChatType, opts.DisabledTools, opts.Thinking, sendEvent)
	log.Printf("[ws] chat.send complete session=%s", sessionID)
}
```

**Step 4: Update AgentLoop.Run to accept options**

In `internal/server/agent.go`, update the `Run` signature and use the options:

```go
func (a *AgentLoop) Run(ctx context.Context, sessionID, userMessage, chatType string, disabledTools []string, thinking bool, sendEvent func(WSMessage)) {
	// ... existing validation ...

	sess := a.sessions.GetOrCreate(sessionID)
	sess.AddMessage("user", userMessage)

	// Build Claude tools, filtering out disabled ones.
	var claudeTools []ai.ClaudeTool
	disabledSet := make(map[string]bool)
	for _, t := range disabledTools {
		disabledSet[t] = true
	}
	if a.products != nil {
		registry := a.products.Registry()
		for _, entry := range registry.AllTools() {
			qualName := entry.ProductName + "__" + entry.Tool.GetName()
			if disabledSet[qualName] {
				continue
			}
			tools := ai.BuildClaudeTools(entry.ProductName, []*soulv1.Tool{entry.Tool})
			claudeTools = append(claudeTools, tools...)
		}
	}

	// Build system prompt with chatType variation.
	sysPrompt := systemPrompt + chatTypePrompt(chatType)
	sysPrompt += fmt.Sprintf("\n\nYou are powered by %s.", a.model)
	// ... rest of existing tool names logic ...

	// In the agent loop, set thinking and maxTokens:
	maxTokens := 4096
	var thinkingCfg *ai.ThinkingConfig
	if thinking && strings.Contains(a.model, "opus") {
		thinkingCfg = &ai.ThinkingConfig{Type: "enabled", BudgetTokens: 10000}
		maxTokens = 16000
	}

	// ... existing loop, but use maxTokens and thinkingCfg in req ...
}
```

Add the `chatTypePrompt` function:

```go
func chatTypePrompt(chatType string) string {
	switch strings.ToLower(chatType) {
	case "code":
		return "\n\n# Mode: Code\nFocus on code generation. Be concise. Show code blocks. Minimize prose."
	case "architect":
		return "\n\n# Mode: Architect\nFocus on system design and architecture. Think about scalability, trade-offs, and structure. Use diagrams when helpful."
	case "debug":
		return "\n\n# Mode: Debug\nSystematic debugging workflow. Ask for the error first. Reproduce. Diagnose step by step. Identify root cause before suggesting fixes."
	case "review":
		return "\n\n# Mode: Code Review\nReview code for bugs, security issues, performance, and style. Give structured feedback with severity levels."
	case "tdd":
		return "\n\n# Mode: TDD\nTest-driven development. Write the failing test first, then the minimal implementation to pass it. Red-green-refactor."
	case "brainstorm":
		return "\n\n# Mode: Brainstorm\nOpen-ended exploration. Ask clarifying questions. Propose 2-3 approaches with trade-offs. Don't jump to implementation."
	default:
		return ""
	}
}
```

**Step 5: Build and verify**

Run: `cd /home/rishav/soul && go build -o soul ./cmd/soul`
Expected: builds with no errors

**Step 6: Commit**

```bash
git add internal/server/routes.go internal/server/ws.go internal/server/agent.go internal/ai/client.go
git commit -m "feat: wire model override, extended thinking, chatType prompts, tool filtering"
```

---

### Task 2: Frontend — Extended Thinking Toggle + Chat Types Update

Update InputBar to show the thinking toggle and the new chat types.

**Files:**
- Modify: `web/src/components/chat/InputBar.tsx:23,170-198` — chat types + thinking toggle
- Modify: `web/src/lib/types.ts:99-103` — add thinking to SendOptions

**Step 1: Update SendOptions in types.ts**

In `web/src/lib/types.ts`, add `thinking` field:

```ts
export interface SendOptions {
  model?: string;
  chatType?: string;
  disabledTools?: string[];
  thinking?: boolean;
}
```

**Step 2: Update CHAT_TYPES and add thinking state in InputBar**

In `web/src/components/chat/InputBar.tsx`:

Replace the `CHAT_TYPES` const:

```ts
const CHAT_TYPES = [
  { value: 'Chat', label: 'Chat', group: 'mode' },
  { value: 'Code', label: 'Code', group: 'mode' },
  { value: 'Architect', label: 'Architect', group: 'mode' },
  { value: 'Debug', label: 'Debug', group: 'skill' },
  { value: 'Review', label: 'Review', group: 'skill' },
  { value: 'TDD', label: 'TDD', group: 'skill' },
  { value: 'Brainstorm', label: 'Brainstorm', group: 'skill' },
] as const;
```

Add `thinking` state after the `chatType` state:

```ts
const [thinking, setThinking] = useState(false);
```

**Step 3: Add thinking toggle and update the select**

In the toolbar JSX, replace the chat type `<select>` with a grouped one:

```tsx
<select
  value={chatType}
  onChange={(e) => setChatType(e.target.value)}
  className="soul-select pr-6 text-[11px]"
>
  <optgroup label="Modes">
    {CHAT_TYPES.filter(t => t.group === 'mode').map((t) => (
      <option key={t.value} value={t.value}>{t.label}</option>
    ))}
  </optgroup>
  <optgroup label="Workflows">
    {CHAT_TYPES.filter(t => t.group === 'skill').map((t) => (
      <option key={t.value} value={t.value}>{t.label}</option>
    ))}
  </optgroup>
</select>
```

After the chat type select, add the thinking toggle (visible only when model contains "opus"):

```tsx
{model.includes('opus') && (
  <button
    type="button"
    onClick={() => setThinking(!thinking)}
    className={`w-7 h-7 flex items-center justify-center rounded transition-colors cursor-pointer ${
      thinking ? 'bg-soul/20 text-soul' : 'text-fg-muted hover:text-fg hover:bg-elevated'
    }`}
    title={thinking ? 'Extended thinking ON' : 'Extended thinking OFF'}
  >
    <svg width="14" height="14" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
      <path d="M8 2a5 5 0 0 1 3 9v1.5a1.5 1.5 0 0 1-1.5 1.5h-3A1.5 1.5 0 0 1 5 12.5V11a5 5 0 0 1 3-9z" />
      <path d="M6 14.5h4" />
    </svg>
  </button>
)}
```

**Step 4: Update handleSend to include thinking**

In the `handleSend` callback, add thinking to options:

```ts
if (thinking) options.thinking = true;
```

**Step 5: Build and verify**

Run: `cd /home/rishav/soul/web && npx vite build`
Expected: builds with no errors

**Step 6: Commit**

```bash
git add web/src/components/chat/InputBar.tsx web/src/lib/types.ts
git commit -m "feat: extended thinking toggle + hybrid chat types"
```

---

### Task 3: Paste-to-Upload in InputBar

Add clipboard paste handling for files and images.

**Files:**
- Modify: `web/src/components/chat/InputBar.tsx` — add paste event handler

**Step 1: Add paste handler**

In `InputBar`, add a `handlePaste` callback after `handleFileSelect`:

```ts
const handlePaste = useCallback((e: React.ClipboardEvent) => {
  const items = e.clipboardData?.items;
  if (!items) return;
  const pastedFiles: File[] = [];
  for (let i = 0; i < items.length; i++) {
    const item = items[i];
    if (item.kind === 'file') {
      const file = item.getAsFile();
      if (file) pastedFiles.push(file);
    }
  }
  if (pastedFiles.length > 0) {
    e.preventDefault();
    setFiles((prev) => [...prev, ...pastedFiles]);
  }
}, []);
```

**Step 2: Attach handler to the container div**

On the outer `<div className="glass rounded-2xl ...">`, add `onPaste={handlePaste}`:

```tsx
<div className="glass rounded-2xl overflow-hidden" onPaste={handlePaste}>
```

**Step 3: Build and verify**

Run: `cd /home/rishav/soul/web && npx vite build`
Expected: builds with no errors

**Step 4: Commit**

```bash
git add web/src/components/chat/InputBar.tsx
git commit -m "feat: paste-to-upload for images and files in InputBar"
```

---

### Task 4: Voice Input (Web Speech API)

Add microphone button that replaces send when textarea is empty.

**Files:**
- Modify: `web/src/components/chat/InputBar.tsx` — voice input logic + mic button

**Step 1: Add speech recognition state and refs**

After existing state declarations in InputBar, add:

```ts
const [isListening, setIsListening] = useState(false);
const [interimText, setInterimText] = useState('');
const recognitionRef = useRef<any>(null);
const speechSupported = typeof window !== 'undefined' && ('webkitSpeechRecognition' in window || 'SpeechRecognition' in window);
```

**Step 2: Add start/stop listening functions**

After the `removeFile` callback:

```ts
const startListening = useCallback(() => {
  if (!speechSupported) return;
  const SpeechRecognition = (window as any).webkitSpeechRecognition || (window as any).SpeechRecognition;
  const recognition = new SpeechRecognition();
  recognition.continuous = true;
  recognition.interimResults = true;
  recognition.lang = 'en-US';

  recognition.onresult = (event: any) => {
    let interim = '';
    let final = '';
    for (let i = event.resultIndex; i < event.results.length; i++) {
      const transcript = event.results[i][0].transcript;
      if (event.results[i].isFinal) {
        final += transcript;
      } else {
        interim += transcript;
      }
    }
    if (final) {
      setValue((prev) => prev + final);
      setInterimText('');
    } else {
      setInterimText(interim);
    }
  };

  recognition.onerror = () => {
    setIsListening(false);
    setInterimText('');
  };

  recognition.onend = () => {
    setIsListening(false);
    setInterimText('');
  };

  recognitionRef.current = recognition;
  recognition.start();
  setIsListening(true);
}, [speechSupported]);

const stopListening = useCallback(() => {
  if (recognitionRef.current) {
    recognitionRef.current.stop();
    recognitionRef.current = null;
  }
  setIsListening(false);
  setInterimText('');
}, []);
```

**Step 3: Add Escape key handler to stop listening**

Update `handleKeyDown` to handle Escape:

```ts
const handleKeyDown = useCallback(
  (e: React.KeyboardEvent<HTMLTextAreaElement>) => {
    if (e.key === 'Escape' && isListening) {
      e.preventDefault();
      stopListening();
      return;
    }
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault();
      if (isListening) stopListening();
      handleSend();
    }
  },
  [handleSend, isListening, stopListening],
);
```

**Step 4: Show interim text in textarea area**

Below the textarea, add interim text display (only when listening):

```tsx
{isListening && interimText && (
  <div className="px-4 pb-1 text-fg-muted text-sm italic truncate">{interimText}</div>
)}
```

**Step 5: Replace send button with mic when empty**

Replace the send button block with conditional rendering:

```tsx
{/* Send or Mic button */}
{!value.trim() && !disabled && speechSupported ? (
  <button
    type="button"
    onClick={isListening ? stopListening : startListening}
    className={`w-8 h-8 rounded-full flex items-center justify-center transition-colors shrink-0 cursor-pointer ${
      isListening
        ? 'bg-stage-blocked text-white animate-soul-pulse'
        : 'bg-elevated text-fg-muted hover:text-fg hover:bg-overlay'
    }`}
    title={isListening ? 'Stop listening' : 'Voice input'}
  >
    <svg width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
      <rect x="5" y="1" width="6" height="8" rx="3" />
      <path d="M3 7v1a5 5 0 0 0 10 0V7" />
      <path d="M8 13v2" />
    </svg>
  </button>
) : (
  <button
    onClick={handleSend}
    disabled={disabled || !value.trim()}
    className="w-8 h-8 bg-soul text-deep rounded-full flex items-center justify-center hover:bg-soul/85 disabled:opacity-20 disabled:cursor-not-allowed transition-colors shrink-0 cursor-pointer"
    title="Send"
  >
    <svg width="16" height="16" viewBox="0 0 16 16" fill="currentColor">
      <path d="M8 3l-1 1 3.3 3.3H3v1.4h7.3L7 12l1 1 5-5-5-5z" />
    </svg>
  </button>
)}
```

**Step 6: Build and verify**

Run: `cd /home/rishav/soul/web && npx vite build`
Expected: builds with no errors

**Step 7: Commit**

```bash
git add web/src/components/chat/InputBar.tsx
git commit -m "feat: voice input via Web Speech API — mic button when textarea empty"
```

---

### Task 5: ChatNavbar Logo Enhancement

Increase the diamond size and add a pulse glow.

**Files:**
- Modify: `web/src/components/chat/ChatNavbar.tsx:19-21`

**Step 1: Update the logo span**

Replace the current logo span:

```tsx
<span className="font-display text-sm font-semibold text-fg flex items-center gap-2">
  <span className="relative">
    <span className="absolute inset-0 -m-1 bg-soul/15 rounded-full blur-md animate-soul-pulse" />
    <span className="relative text-xl text-soul">&#9670;</span>
  </span>
  Soul
</span>
```

**Step 2: Build and verify**

Run: `cd /home/rishav/soul/web && npx vite build`
Expected: builds with no errors

**Step 3: Commit**

```bash
git add web/src/components/chat/ChatNavbar.tsx
git commit -m "feat: bigger navbar logo with pulse glow animation"
```

---

### Task 6: VS Code-Style Panel Toggle Icons

Replace all chevron panel icons with VS Code sidebar layout icons.

**Files:**
- Modify: `web/src/components/chat/ChatNavbar.tsx:25-35` — left sidebar icon
- Modify: `web/src/components/planner/TaskPanel.tsx:123-134` — right sidebar icon
- Modify: `web/src/components/layout/TaskRail.tsx` — right sidebar expand icon
- Modify: `web/src/components/layout/ChatRail.tsx:26-28` — left sidebar expand icon

The VS Code sidebar-left icon (a rectangle split with a narrow left strip):

```tsx
<svg width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.2">
  <rect x="1.5" y="2.5" width="13" height="11" rx="1.5" />
  <line x1="5.5" y1="2.5" x2="5.5" y2="13.5" />
</svg>
```

The VS Code sidebar-right icon (a rectangle split with a narrow right strip):

```tsx
<svg width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.2">
  <rect x="1.5" y="2.5" width="13" height="11" rx="1.5" />
  <line x1="10.5" y1="2.5" x2="10.5" y2="13.5" />
</svg>
```

**Step 1: Update ChatNavbar collapse button**

Replace the chevron-left SVG in `ChatNavbar.tsx` with the sidebar-left SVG shown above.

**Step 2: Update TaskPanel collapse button**

Replace the chevron-right SVG in `TaskPanel.tsx` (the collapse button around line 124-134) with the sidebar-right SVG shown above.

**Step 3: Update TaskRail expand icon**

In `TaskRail.tsx`, find the expand chevron button and replace its SVG with the sidebar-right SVG.

**Step 4: Update ChatRail expand icon**

In `ChatRail.tsx`, replace the chat bubble emoji icon (line 26-28) with the sidebar-left SVG.

**Step 5: Build and verify**

Run: `cd /home/rishav/soul/web && npx vite build`
Expected: builds with no errors

**Step 6: Commit**

```bash
git add web/src/components/chat/ChatNavbar.tsx web/src/components/planner/TaskPanel.tsx web/src/components/layout/TaskRail.tsx web/src/components/layout/ChatRail.tsx
git commit -m "feat: VS Code-style sidebar panel toggle icons"
```

---

### Task 7: Chat Visibility Improvements

Improve message styling, InputBar contrast, and constrain chat width.

**Files:**
- Modify: `web/src/components/chat/Message.tsx` — user/assistant styling
- Modify: `web/src/components/chat/InputBar.tsx:130-131` — container styling
- Modify: `web/src/components/chat/ChatView.tsx:18` — already has max-w-3xl (verify)

**Step 1: Update Message styling**

In `Message.tsx`, update the message container classes:

```tsx
export default function Message({ message }: MessageProps) {
  const isUser = message.role === 'user';

  return (
    <div className={`flex ${isUser ? 'justify-end' : 'justify-start'} animate-fade-in`}>
      <div
        className={`max-w-[80%] px-4 py-3 ${
          isUser
            ? 'bg-elevated border-l-2 border-soul/40 text-fg rounded-2xl rounded-br-md'
            : 'text-fg rounded-2xl rounded-bl-md'
        }`}
      >
```

Key changes:
- User: add `border-l-2 border-soul/40` accent
- Assistant: remove `bg-surface border border-border-subtle` → transparent background, just padding

**Step 2: Update InputBar container**

In `InputBar.tsx`, change the outer container class from `glass` to more prominent styling:

```tsx
<div className="bg-elevated border border-border-default rounded-2xl overflow-hidden shadow-lg shadow-black/20" onPaste={handlePaste}>
```

**Step 3: Verify ChatView already constrains width**

Check that `ChatView.tsx` already has `max-w-3xl mx-auto` on the message container (it should from previous work — line 19). If not, add it.

**Step 4: Build and verify**

Run: `cd /home/rishav/soul/web && npx vite build`
Expected: builds with no errors

**Step 5: Commit**

```bash
git add web/src/components/chat/Message.tsx web/src/components/chat/InputBar.tsx
git commit -m "feat: improved chat visibility — message accents, InputBar contrast"
```

---

### Task 8: TaskPanel View Icons — Bigger SVGs

Replace unicode view icons with proper SVGs at larger size.

**Files:**
- Modify: `web/src/components/planner/TaskPanel.tsx:32-36,87-101`

**Step 1: Replace VIEW_BUTTONS with SVG icons**

Replace the `VIEW_BUTTONS` array and the rendering:

```ts
const VIEW_BUTTONS: { view: TaskView; title: string }[] = [
  { view: 'list', title: 'List view' },
  { view: 'kanban', title: 'Kanban view' },
  { view: 'grid', title: 'Grid view' },
];

function ViewIcon({ view }: { view: TaskView }) {
  switch (view) {
    case 'list':
      return (
        <svg width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round">
          <path d="M3 4h10M3 8h10M3 12h10" />
        </svg>
      );
    case 'kanban':
      return (
        <svg width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round">
          <path d="M3 3v10M8 3v7M13 3v10" />
          <circle cx="3" cy="3" r="0.8" fill="currentColor" stroke="none" />
          <circle cx="8" cy="3" r="0.8" fill="currentColor" stroke="none" />
          <circle cx="13" cy="3" r="0.8" fill="currentColor" stroke="none" />
        </svg>
      );
    case 'grid':
      return (
        <svg width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round">
          <rect x="2" y="2" width="5" height="5" rx="1" />
          <rect x="9" y="2" width="5" height="5" rx="1" />
          <rect x="2" y="9" width="5" height="5" rx="1" />
          <rect x="9" y="9" width="5" height="5" rx="1" />
        </svg>
      );
  }
}
```

Update the button rendering to use `w-8 h-8`:

```tsx
{VIEW_BUTTONS.map(({ view, title }) => (
  <button
    key={view}
    type="button"
    onClick={() => setTaskView(view)}
    title={title}
    className={`w-8 h-8 flex items-center justify-center rounded cursor-pointer transition-colors ${
      taskView === view
        ? 'bg-overlay text-fg'
        : 'text-fg-muted hover:text-fg-secondary hover:bg-elevated'
    }`}
  >
    <ViewIcon view={view} />
  </button>
))}
```

**Step 2: Build and verify**

Run: `cd /home/rishav/soul/web && npx vite build`
Expected: builds with no errors

**Step 3: Commit**

```bash
git add web/src/components/planner/TaskPanel.tsx
git commit -m "feat: SVG view icons at w-8 h-8 for TaskPanel"
```

---

### Task 9: TaskPanel Responsive Filters

Move filters into navbar, collapse to popover at narrow widths.

**Files:**
- Modify: `web/src/components/planner/TaskPanel.tsx` — inline filters in navbar
- Modify: `web/src/components/planner/FilterBar.tsx` — add compact mode

**Step 1: Add width detection to TaskPanel**

At the top of the TaskPanel component, add a ref and state for width:

```ts
const panelRef = useRef<HTMLDivElement>(null);
const [isNarrow, setIsNarrow] = useState(false);
const [showFilterPopover, setShowFilterPopover] = useState(false);
const filterPopoverRef = useRef<HTMLDivElement>(null);

useEffect(() => {
  const el = panelRef.current;
  if (!el) return;
  const observer = new ResizeObserver((entries) => {
    for (const entry of entries) {
      setIsNarrow(entry.contentRect.width < 400);
    }
  });
  observer.observe(el);
  return () => observer.disconnect();
}, []);

// Close filter popover on outside click
useEffect(() => {
  if (!showFilterPopover) return;
  const handler = (e: MouseEvent) => {
    if (filterPopoverRef.current && !filterPopoverRef.current.contains(e.target as Node)) {
      setShowFilterPopover(false);
    }
  };
  document.addEventListener('mousedown', handler);
  return () => document.removeEventListener('mousedown', handler);
}, [showFilterPopover]);
```

Add `ref={panelRef}` to the outer div.

**Step 2: Count active filters**

```ts
const activeFilterCount = [
  filters.stage !== 'all',
  filters.priority !== 'all',
  filters.product !== 'all',
].filter(Boolean).length;
```

**Step 3: In the navbar, after view buttons, add inline filters or filter button**

When wide (not narrow), render the 3 selects inline in the navbar (before the spacer). When narrow, render a filter button with popover:

```tsx
{/* Filters — inline when wide, popover when narrow */}
{isNarrow ? (
  <div className="relative" ref={filterPopoverRef}>
    <button
      type="button"
      onClick={() => setShowFilterPopover(!showFilterPopover)}
      className="relative w-7 h-7 flex items-center justify-center rounded hover:bg-elevated text-fg-muted hover:text-fg transition-colors cursor-pointer"
      title="Filters"
    >
      <svg width="14" height="14" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round">
        <path d="M2 3h12M4 8h8M6 13h4" />
      </svg>
      {activeFilterCount > 0 && (
        <span className="absolute -top-1 -right-1 w-4 h-4 bg-soul text-deep text-[9px] font-bold rounded-full flex items-center justify-center">
          {activeFilterCount}
        </span>
      )}
    </button>
    {showFilterPopover && (
      <div className="absolute top-full right-0 mt-1 bg-surface border border-border-default rounded-xl shadow-xl z-40 p-3 min-w-[200px]">
        <FilterBar filters={filters} products={products} onChange={setFilters} />
      </div>
    )}
  </div>
) : (
  <FilterBar filters={filters} products={products} onChange={setFilters} />
)}
```

**Step 4: Update FilterBar for inline mode**

In `FilterBar.tsx`, make it work both inline (in a flex row) and in the popover:

```tsx
export default function FilterBar({ filters, products, onChange }: FilterBarProps) {
  return (
    <div className="flex items-center gap-2 text-xs">
      {/* selects stay the same but remove bg/border/padding from container */}
      <select className="soul-select" value={filters.stage} onChange={...}>
        ...
      </select>
      <select className="soul-select" value={...} onChange={...}>
        ...
      </select>
      <select className="soul-select" value={filters.product} onChange={...}>
        ...
      </select>
    </div>
  );
}
```

Remove the old `bg-surface/50 border-b border-border-subtle px-4 py-2 shrink-0` wrapper styling (now controlled by the parent).

**Step 5: Remove the separate FilterBar row from TaskPanel body**

Delete the standalone `<FilterBar ... />` that was between the navbar and body sections (it's now inline in the navbar or in the popover).

**Step 6: Build and verify**

Run: `cd /home/rishav/soul/web && npx vite build`
Expected: builds with no errors

**Step 7: Commit**

```bash
git add web/src/components/planner/TaskPanel.tsx web/src/components/planner/FilterBar.tsx
git commit -m "feat: responsive TaskPanel filters — inline when wide, popover when narrow"
```

---

### Task 10: SoulRail + SoulPanel (Left Sidebar)

New left sidebar with session history, navigation, settings.

**Files:**
- Create: `web/src/components/layout/SoulRail.tsx`
- Create: `web/src/components/layout/SoulPanel.tsx`
- Modify: `web/src/components/layout/AppShell.tsx` — 3-column layout
- Modify: `web/src/hooks/useLayoutStore.ts` — add soulState
- Modify: `web/src/lib/types.ts` — update LayoutState
- Modify: `web/src/components/chat/ChatPanel.tsx` — remove hamburger/session drawer (moved to SoulPanel)

**Step 1: Add soulState to LayoutState type**

In `web/src/lib/types.ts`, update LayoutState:

```ts
export interface LayoutState {
  soulState: PanelState;  // NEW
  chatState: PanelState;
  taskState: PanelState;
  taskView: TaskView;
  gridSubView: GridSubView;
  panelWidth: number | null;
  filters: TaskFilters;
}
```

**Step 2: Update useLayoutStore**

In `web/src/hooks/useLayoutStore.ts`:

- Add `soulState: 'rail'` to `DEFAULT_STATE`
- Add `setSoulState` callback (same pattern as setChatState/setTaskState but independent — soulState doesn't block other panels)
- Add to the return object

```ts
const DEFAULT_STATE: LayoutState = {
  soulState: 'rail',
  chatState: 'open',
  taskState: 'open',
  // ... rest same
};

const setSoulState = useCallback(
  (s: PanelState) => {
    setState((prev) => ({ ...prev, soulState: s }));
  },
  [setState],
);
```

**Step 3: Create SoulRail.tsx**

```tsx
interface SoulRailProps {
  onExpand: () => void;
}

export default function SoulRail({ onExpand }: SoulRailProps) {
  return (
    <div className="w-10 h-full bg-surface border-r border-border-subtle flex flex-col items-center py-3 gap-3 shrink-0">
      {/* Soul logo */}
      <span className="relative">
        <span className="absolute inset-0 -m-0.5 bg-soul/10 rounded-full blur-sm animate-soul-pulse" />
        <span className="relative text-lg text-soul">&#9670;</span>
      </span>

      <div className="w-5 border-t border-border-subtle" />

      {/* Chat indicator */}
      <button type="button" className="w-7 h-7 flex items-center justify-center rounded text-fg-muted hover:text-fg hover:bg-elevated transition-colors cursor-pointer" title="Chat">
        <svg width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
          <path d="M2 3h12v8H5l-3 3V3z" />
        </svg>
      </button>

      {/* Tasks shortcut */}
      <button type="button" className="w-7 h-7 flex items-center justify-center rounded text-fg-muted hover:text-fg hover:bg-elevated transition-colors cursor-pointer" title="Tasks">
        <svg width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
          <path d="M3 4l2 2 4-4" />
          <path d="M3 10l2 2 4-4" />
        </svg>
      </button>

      <div className="flex-1" />

      {/* Settings */}
      <button type="button" className="w-7 h-7 flex items-center justify-center rounded text-fg-muted hover:text-fg hover:bg-elevated transition-colors cursor-pointer" title="Settings">
        <svg width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
          <circle cx="8" cy="8" r="2" />
          <path d="M8 1v2M8 13v2M1 8h2M13 8h2M3.05 3.05l1.41 1.41M11.54 11.54l1.41 1.41M3.05 12.95l1.41-1.41M11.54 4.46l1.41-1.41" />
        </svg>
      </button>

      {/* Expand */}
      <button
        type="button"
        onClick={onExpand}
        className="w-7 h-7 flex items-center justify-center rounded hover:bg-elevated text-fg-muted hover:text-fg transition-colors cursor-pointer"
        title="Expand sidebar"
      >
        <svg width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.2">
          <rect x="1.5" y="2.5" width="13" height="11" rx="1.5" />
          <line x1="5.5" y1="2.5" x2="5.5" y2="13.5" />
        </svg>
      </button>
    </div>
  );
}
```

**Step 4: Create SoulPanel.tsx**

```tsx
import type { ChatSession } from '../../lib/types.ts';

interface SoulPanelProps {
  onCollapse: () => void;
  sessions: ChatSession[];
  activeSessionId: number | null;
  onSessionSelect: (id: number) => void;
  onNewChat: () => void;
  connected: boolean;
}

export default function SoulPanel({
  onCollapse,
  sessions,
  activeSessionId,
  onSessionSelect,
  onNewChat,
  connected,
}: SoulPanelProps) {
  return (
    <div className="w-60 h-full bg-surface border-r border-border-subtle flex flex-col shrink-0">
      {/* Header */}
      <div className="glass flex items-center gap-2 h-11 px-3 shrink-0">
        <span className="relative">
          <span className="absolute inset-0 -m-0.5 bg-soul/10 rounded-full blur-sm animate-soul-pulse" />
          <span className="relative text-lg text-soul">&#9670;</span>
        </span>
        <span className="font-display text-sm font-semibold text-fg">Soul</span>
        <div className="flex-1" />
        <button
          type="button"
          onClick={onCollapse}
          className="w-7 h-7 flex items-center justify-center rounded hover:bg-elevated text-fg-muted hover:text-fg transition-colors cursor-pointer"
          title="Collapse sidebar"
        >
          <svg width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.2">
            <rect x="1.5" y="2.5" width="13" height="11" rx="1.5" />
            <line x1="5.5" y1="2.5" x2="5.5" y2="13.5" />
          </svg>
        </button>
      </div>

      {/* New Chat button */}
      <div className="px-3 py-2">
        <button
          type="button"
          onClick={onNewChat}
          className="w-full bg-soul/10 hover:bg-soul/20 text-soul font-display font-semibold text-xs rounded-lg px-3 py-2 flex items-center gap-2 transition-colors cursor-pointer"
        >
          <svg width="12" height="12" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round">
            <path d="M8 3v10M3 8h10" />
          </svg>
          New Chat
        </button>
      </div>

      {/* Sessions */}
      <div className="flex-1 overflow-y-auto px-1">
        <div className="px-2 py-1 text-[10px] font-display uppercase tracking-widest text-fg-muted">
          Sessions
        </div>
        {sessions.map((s) => (
          <button
            key={s.id}
            type="button"
            onClick={() => onSessionSelect(s.id)}
            className={`w-full text-left px-3 py-1.5 rounded-lg text-xs truncate cursor-pointer transition-colors ${
              s.id === activeSessionId
                ? 'bg-elevated text-fg'
                : 'text-fg-secondary hover:bg-elevated/50 hover:text-fg'
            }`}
          >
            {s.title || `Session ${s.id}`}
          </button>
        ))}
      </div>

      {/* Footer */}
      <div className="px-3 py-2 border-t border-border-subtle">
        <div className="flex items-center gap-2 text-[10px] text-fg-muted">
          <span className={`w-2 h-2 rounded-full ${connected ? 'bg-stage-done' : 'bg-stage-blocked'}`} />
          <span>{connected ? 'Connected' : 'Disconnected'}</span>
        </div>
      </div>
    </div>
  );
}
```

**Step 5: Update AppShell for 3-column layout**

In `AppShell.tsx`, add imports and wire the SoulRail/SoulPanel:

Add imports:
```ts
import SoulRail from './SoulRail.tsx';
import SoulPanel from './SoulPanel.tsx';
import { useSessions } from '../../hooks/useSessions.ts';
import { useWebSocket } from '../../hooks/useWebSocket.ts';
```

Inside the component, add:
```ts
const { sessions, activeSessionId, createSession, switchSession } = useSessions();
const { connected } = useWebSocket();

const handleSoulExpand = useCallback(() => {
  layout.setSoulState('open');
}, [layout.setSoulState]);

const handleSoulCollapse = useCallback(() => {
  layout.setSoulState('rail');
}, [layout.setSoulState]);
```

Update the return JSX to add the SoulRail/SoulPanel as the first column:

```tsx
return (
  <div className="h-screen bg-deep text-fg font-body noise flex overflow-hidden">
    {/* Soul: rail or panel */}
    {layout.soulState === 'rail' ? (
      <SoulRail onExpand={handleSoulExpand} />
    ) : (
      <SoulPanel
        onCollapse={handleSoulCollapse}
        sessions={sessions}
        activeSessionId={activeSessionId}
        onSessionSelect={switchSession}
        onNewChat={createSession}
        connected={connected}
      />
    )}

    {/* Chat: rail or panel */}
    {layout.chatState === 'rail' ? (
      <ChatRail unreadCount={unreadCount} onExpand={handleChatExpand} />
    ) : (
      <div
        className="h-full overflow-hidden flex-1 min-w-0"
        style={bothOpen ? { flex: `0 0 ${chatPercent}%` } : undefined}
      >
        <ChatPanel
          onCollapse={handleChatCollapse}
          canCollapse={layout.canCollapse('chat')}
          onUnreadChange={handleUnreadChange}
        />
      </div>
    )}

    {/* Resize divider */}
    {bothOpen && <ResizeDivider onResize={handleResize} />}

    {/* Task: rail or panel */}
    {/* ... same as before ... */}
  </div>
);
```

**Step 6: Remove hamburger + SessionDrawer from ChatPanel**

In `ChatPanel.tsx`, remove:
- The `useSessions` import and hook call
- The `drawerOpen` state
- The `SessionDrawer` component rendering
- The `onToggleDrawer` prop from ChatNavbar

In `ChatNavbar.tsx`, remove:
- The `onToggleDrawer` prop
- The hamburger button

**Step 7: Build and verify**

Run: `cd /home/rishav/soul/web && npx vite build`
Expected: builds with no errors

**Step 8: Commit**

```bash
git add web/src/components/layout/SoulRail.tsx web/src/components/layout/SoulPanel.tsx web/src/components/layout/AppShell.tsx web/src/hooks/useLayoutStore.ts web/src/lib/types.ts web/src/components/chat/ChatPanel.tsx web/src/components/chat/ChatNavbar.tsx
git commit -m "feat: SoulRail + SoulPanel — 3-column layout with session history"
```

---

### Task 11: Build, Deploy, Browser E2E Test

Build everything, restart the server, and verify all 10 changes in the browser.

**Files:** None (testing only)

**Step 1: Build frontend**

Run: `cd /home/rishav/soul/web && npx vite build`
Expected: builds with no errors

**Step 2: Build Go binary**

Run: `cd /home/rishav/soul && go build -o soul ./cmd/soul`
Expected: builds with no errors

**Step 3: Restart server**

```bash
pkill -f './soul serve' 2>/dev/null; sleep 1
SOUL_HOST=0.0.0.0 nohup ./soul serve > /tmp/soul-server.log 2>&1 &
sleep 2 && cat /tmp/soul-server.log
```
Expected: "Soul server listening on 0.0.0.0:3000"

**Step 4: Test /api/models returns 3 models**

```bash
curl -s http://localhost:3000/api/models | python3 -m json.tool
```
Expected: 3 models (Sonnet, Opus, Haiku)

**Step 5: Browser E2E test**

Navigate to `http://192.168.0.128:3000/` and verify:
1. SoulRail visible on left (collapsed state) with diamond, chat, tasks, settings icons
2. Click expand → SoulPanel shows with session list, New Chat button, connection status
3. ChatNavbar shows bigger diamond with glow, no hamburger menu
4. Panel toggle icons are VS Code sidebar style (rectangle with strip, not chevrons)
5. InputBar shows 3 models in dropdown, 7 chat types with grouping, thinking toggle appears when Opus selected
6. When textarea empty: mic button shows instead of send
7. Paste an image → shows as attachment pill
8. Message styling: user has gold left border, assistant is transparent bg
9. TaskPanel has SVG view icons (list/kanban/grid) at w-8 h-8
10. TaskPanel filters inline when wide, collapse to filter button when narrow

**Step 6: Fix any issues found**

If any issues found, fix and rebuild.

**Step 7: Commit fixes if needed**

```bash
git add -A && git commit -m "fix: E2E test fixes for UI enhancements v2"
```
