# PM Agent Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add always-on project management intelligence to Soul — rule-based backend checks, LLM enrichment, frontend validation, and a PM chat skill.

**Architecture:** Three layers — (1) Go backend `PMService` with after-create hook + periodic sweep, (2) agent system prompt + hard validation in `toolTaskCreate`, (3) frontend form validation + `pm.notification` rendering in chat.

**Tech Stack:** Go 1.24, React 19 + TypeScript + Tailwind v4, SQLite, Claude API (`CompleteSimple`)

---

## Task 1: Agent System Prompt PM Rules

**Files:**
- Modify: `internal/server/agent.go:39-46` (systemPrompt `# Task management` section)

**Step 1: Add task creation standards to systemPrompt**

In `agent.go`, replace the existing `# Task management` section (lines 39-46) with an expanded version that includes PM rules:

```go
# Task management
- You have built-in tools to manage the task board: task_create, task_list, task_update, quick_task.
- When the user asks to create, add, or track tasks — use the task_create tool. Do not say you cannot manage tasks.
- When the user asks to see or list tasks — use the task_list tool.
- When the user asks to update, move, or change a task — use the task_update tool.
- When the user wants a quick code change done autonomously — use the quick_task tool. It creates and immediately starts an autonomous agent to make the change.
- Tasks are created in the "backlog" stage by default. The user can ask to move them to other stages.
- Always confirm what you did after creating or updating tasks.

# Task creation standards
When creating tasks with task_create, ALWAYS follow these rules:
- Title must be specific and actionable (verb + object, 10+ chars). Bad: "Fix bug". Good: "Fix login timeout on session expiry"
- Description is REQUIRED. Include: what needs to change, why, and acceptance criteria as a markdown checklist (- [ ]).
- Set priority based on impact: 3=Critical (broken/blocking), 2=High (important), 1=Normal (default), 0=Low (nice-to-have).
- Set product when known (soul, scout, compliance).
- Before creating, check if a similar task already exists: use task_list to verify.
- For large tasks (3+ files, multiple concerns), create a parent task + subtasks instead of one monolith.
- When decomposing, create a parent task first, then subtasks referencing the parent.

# Board management
When the user asks to groom, triage, plan, or review the board:
- Use task_list to get current board state.
- Flag stale tasks (>7 days in backlog) — suggest removing or reprioritizing.
- Flag stuck tasks (active >48h with no progress, blocked >5 days).
- Identify tasks missing descriptions or acceptance criteria.
- For sprint planning: recommend 3-5 tasks based on priority (highest first), then age (oldest first).
- Act directly on safe actions (create subtasks, add comments, fix priorities). Ask permission for destructive actions (delete, merge duplicates).
```

**Step 2: Verify build**

Run: `go build ./...`
Expected: Success (no compilation errors — only changed a string constant)

---

## Task 2: Hard Validation in toolTaskCreate

**Files:**
- Modify: `internal/server/agent.go:1619-1654` (toolTaskCreate function)

**Step 1: Add title length validation**

In `toolTaskCreate`, after the existing `title == ""` check (line 1621-1623), add a length check:

```go
func (a *AgentLoop) toolTaskCreate(input map[string]any, sendEvent func(WSMessage)) string {
	title, _ := input["title"].(string)
	if title == "" {
		return "Error: title is required"
	}
	if len(strings.TrimSpace(title)) < 10 {
		return "Error: Title too short (minimum 10 chars). Use a specific, actionable title with a verb and object. Example: 'Add logout button to sidebar'"
	}
	description, _ := input["description"].(string)
	// ... rest unchanged
```

The `strings` package is already imported in agent.go.

**Step 2: Verify build**

Run: `go build ./...`
Expected: Success

---

## Task 3: PMService Struct + Levenshtein + Constructor

**Files:**
- Create: `internal/server/pm.go`

**Step 1: Create pm.go with the PMService struct, constructor, Levenshtein, and helpers**

```go
package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/rishav1305/soul/internal/ai"
	"github.com/rishav1305/soul/internal/planner"
)

// PMService provides always-on project management intelligence.
// It runs rule-based checks on task creation and periodic board sweeps.
type PMService struct {
	planner   *planner.Store
	broadcast func(WSMessage)
	ai        *ai.Client
	model     string
	ticker    *time.Ticker
	stopCh    chan struct{}
}

// NewPMService creates a PMService. ai may be nil (disables LLM features).
func NewPMService(store *planner.Store, broadcast func(WSMessage), aiClient *ai.Client, model string) *PMService {
	return &PMService{
		planner:   store,
		broadcast: broadcast,
		ai:        aiClient,
		model:     model,
		stopCh:    make(chan struct{}),
	}
}

// Start begins the periodic sweep ticker (every 30 minutes).
func (pm *PMService) Start() {
	pm.ticker = time.NewTicker(30 * time.Minute)
	go func() {
		for {
			select {
			case <-pm.ticker.C:
				pm.sweep()
			case <-pm.stopCh:
				return
			}
		}
	}()
	log.Println("[pm] service started (sweep every 30m)")
}

// Stop halts the periodic sweep.
func (pm *PMService) Stop() {
	if pm.ticker != nil {
		pm.ticker.Stop()
	}
	close(pm.stopCh)
}

// levenshtein computes the edit distance between two strings.
func levenshtein(a, b string) int {
	la, lb := len(a), len(b)
	if la == 0 {
		return lb
	}
	if lb == 0 {
		return la
	}
	prev := make([]int, lb+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= la; i++ {
		curr := make([]int, lb+1)
		curr[0] = i
		for j := 1; j <= lb; j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			curr[j] = min(curr[j-1]+1, min(prev[j]+1, prev[j-1]+cost))
		}
		prev = curr
	}
	return prev[lb]
}

// titleSimilarity returns a 0.0–1.0 score of how similar two titles are.
func titleSimilarity(a, b string) float64 {
	a, b = strings.ToLower(strings.TrimSpace(a)), strings.ToLower(strings.TrimSpace(b))
	maxLen := max(len(a), len(b))
	if maxLen == 0 {
		return 1.0
	}
	return 1.0 - float64(levenshtein(a, b))/float64(maxLen)
}

// postComment is a helper to create a PM comment on a task.
func (pm *PMService) postComment(taskID int64, body string) {
	c := planner.Comment{
		TaskID:    taskID,
		Author:    "pm",
		Type:      "feedback",
		Body:      body,
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
	}
	if _, err := pm.planner.CreateComment(c); err != nil {
		log.Printf("[pm] failed to post comment on task %d: %v", taskID, err)
	}
}

// broadcastPM sends a pm.notification message to all connected WebSocket clients.
func (pm *PMService) broadcastPM(severity, content string, taskIDs []int64, check string) {
	if pm.broadcast == nil {
		return
	}
	data, _ := json.Marshal(map[string]any{
		"severity": severity,
		"task_ids": taskIDs,
		"check":    check,
	})
	pm.broadcast(WSMessage{
		Type:    "pm.notification",
		Content: content,
		Data:    data,
	})
}
```

**Step 2: Verify build**

Run: `go build ./...`
Expected: Success

---

## Task 4: AfterCreate Hook — Rule-Based Checks

**Files:**
- Modify: `internal/server/pm.go` (append to file)

**Step 1: Add the AfterCreate method**

Append to `pm.go`:

```go
// AfterCreate runs PM checks on a newly created task. Called from both
// handleTaskCreate (API/frontend) and toolTaskCreate (agent).
func (pm *PMService) AfterCreate(task planner.Task) {
	go pm.afterCreateAsync(task)
}

func (pm *PMService) afterCreateAsync(task planner.Task) {
	// 1. Missing/short description
	if len(strings.TrimSpace(task.Description)) < 20 {
		if task.Source == "ai" && pm.ai != nil {
			// Auto-enrich via LLM
			pm.enrichDescription(task)
		} else {
			pm.postComment(task.ID, "**[PM]** This task has no detailed description. Consider adding:\n- What needs to change and why\n- Acceptance criteria as a checklist (`- [ ]`)")
		}
	} else if !strings.Contains(task.Description, "- [ ]") && !strings.Contains(strings.ToLower(task.Description), "acceptance") {
		// Has description but no AC
		pm.postComment(task.ID, "**[PM]** Consider adding acceptance criteria as a checklist (`- [ ]`) to make validation clear.")
	}

	// 2. Priority sanity — critical with no description
	if task.Priority >= 3 && len(strings.TrimSpace(task.Description)) < 20 {
		pm.postComment(task.ID, "**[PM]** Critical-priority task with no description. Add details or lower the priority.")
		pm.broadcastPM("error",
			fmt.Sprintf("**[PM]** Task #%d (%s) is critical but has no description.", task.ID, task.Title),
			[]int64{task.ID}, "critical_no_desc")
	}

	// 3. Duplicate detection
	pm.checkDuplicate(task)

	// 4. Large task detection — trigger decomposition
	if pm.ai != nil && pm.shouldDecompose(task) {
		pm.decompose(task)
	}
}

// checkDuplicate compares the new task title against open tasks.
func (pm *PMService) checkDuplicate(task planner.Task) {
	for _, stage := range []planner.Stage{planner.StageBacklog, planner.StageBrainstorm, planner.StageActive} {
		tasks, err := pm.planner.List(planner.TaskFilter{Stage: stage})
		if err != nil {
			continue
		}
		for _, existing := range tasks {
			if existing.ID == task.ID {
				continue
			}
			if titleSimilarity(task.Title, existing.Title) > 0.7 {
				pm.postComment(task.ID,
					fmt.Sprintf("**[PM]** Possible duplicate of Task #%d: %s (%.0f%% similar title)",
						existing.ID, existing.Title, titleSimilarity(task.Title, existing.Title)*100))
				return // only flag one duplicate
			}
		}
	}
}

// shouldDecompose returns true if the task looks too large for a single agent session.
func (pm *PMService) shouldDecompose(task planner.Task) bool {
	if task.ParentID != nil {
		return false // already a subtask
	}
	if len(task.Description) > 500 {
		return true
	}
	titleLower := strings.ToLower(task.Title)
	keywords := []string{"refactor", "redesign", "implement full", "add feature with", "new feature", "migration", "overhaul"}
	for _, kw := range keywords {
		if strings.Contains(titleLower, kw) {
			return true
		}
	}
	return false
}
```

**Step 2: Verify build**

Run: `go build ./...`
Expected: Success

---

## Task 5: LLM Enrichment + Decomposition

**Files:**
- Modify: `internal/server/pm.go` (append to file)

**Step 1: Add enrichDescription and decompose methods**

Append to `pm.go`:

```go
// enrichDescription uses LLM to generate a description for a task created without one.
func (pm *PMService) enrichDescription(task planner.Task) {
	prompt := fmt.Sprintf(`Given this task title: "%s"
Product: %s

Generate a 2-3 line description with acceptance criteria as a markdown checklist (- [ ] items).
Return ONLY the description text, no preamble.`, task.Title, task.Product)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	result, err := pm.ai.CompleteSimple(ctx, "claude-haiku-4-5-20251001", prompt)
	if err != nil {
		log.Printf("[pm] failed to enrich task %d: %v", task.ID, err)
		pm.postComment(task.ID, "**[PM]** This task has no description. Consider adding details and acceptance criteria.")
		return
	}

	result = strings.TrimSpace(result)
	if result == "" {
		return
	}

	// Update the task description.
	if err := pm.planner.Update(task.ID, planner.TaskUpdate{Description: &result}); err != nil {
		log.Printf("[pm] failed to update task %d description: %v", task.ID, err)
		return
	}

	pm.postComment(task.ID, "**[PM]** Auto-generated description from title. Review and edit if needed.")
	log.Printf("[pm] enriched task %d description", task.ID)
}

// decompose breaks a large task into subtasks using LLM.
func (pm *PMService) decompose(task planner.Task) {
	prompt := fmt.Sprintf(`Break this task into 2-5 focused subtasks. Each subtask should be completable in one autonomous agent session (1-3 files, under 30 minutes).

Task: %s
Description: %s

Return a JSON array only, no markdown fences, no preamble:
[{"title": "...", "description": "...", "priority": 1}]`, task.Title, task.Description)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	result, err := pm.ai.CompleteSimple(ctx, "claude-haiku-4-5-20251001", prompt)
	if err != nil {
		log.Printf("[pm] failed to decompose task %d: %v", task.ID, err)
		return
	}

	// Strip markdown fences if present.
	result = strings.TrimSpace(result)
	result = strings.TrimPrefix(result, "```json")
	result = strings.TrimPrefix(result, "```")
	result = strings.TrimSuffix(result, "```")
	result = strings.TrimSpace(result)

	var subtasks []struct {
		Title       string `json:"title"`
		Description string `json:"description"`
		Priority    int    `json:"priority"`
	}
	if err := json.Unmarshal([]byte(result), &subtasks); err != nil {
		log.Printf("[pm] failed to parse decomposition for task %d: %v", task.ID, err)
		return
	}

	if len(subtasks) < 2 {
		return // not worth decomposing
	}

	var createdIDs []int64
	for _, st := range subtasks {
		sub := planner.NewTask(st.Title, st.Description)
		sub.Priority = st.Priority
		sub.Product = task.Product
		sub.Source = "ai"
		sub.ParentID = &task.ID
		id, err := pm.planner.Create(sub)
		if err != nil {
			log.Printf("[pm] failed to create subtask for task %d: %v", task.ID, err)
			continue
		}
		createdIDs = append(createdIDs, id)
	}

	if len(createdIDs) > 0 {
		ids := make([]string, len(createdIDs))
		for i, id := range createdIDs {
			ids[i] = fmt.Sprintf("#%d", id)
		}
		pm.postComment(task.ID,
			fmt.Sprintf("**[PM]** Decomposed into %d subtasks: %s", len(createdIDs), strings.Join(ids, ", ")))
		log.Printf("[pm] decomposed task %d into %d subtasks", task.ID, len(createdIDs))
	}
}
```

**Step 2: Verify build**

Run: `go build ./...`
Expected: Success

---

## Task 6: Periodic Sweep

**Files:**
- Modify: `internal/server/pm.go` (append to file)

**Step 1: Add the sweep method**

Append to `pm.go`:

```go
// sweep checks the board for hygiene issues and broadcasts findings.
func (pm *PMService) sweep() {
	log.Println("[pm] running periodic sweep")
	var findings []string
	var taskIDs []int64
	now := time.Now().UTC()

	// Check each stage for issues.
	for _, check := range []struct {
		stage    planner.Stage
		maxAge   time.Duration
		message  string
		severity string
		action   string // "comment", "decay", or ""
	}{
		{planner.StageBacklog, 7 * 24 * time.Hour, "stale in backlog %d days", "info", "comment"},
		{planner.StageActive, 48 * time.Hour, "active %d hours with no recent activity", "warning", "comment"},
		{planner.StageBlocked, 5 * 24 * time.Hour, "blocked for %d days", "warning", ""},
		{planner.StageValidation, 3 * 24 * time.Hour, "awaiting review for %d days", "info", ""},
	} {
		tasks, err := pm.planner.List(planner.TaskFilter{Stage: check.stage})
		if err != nil {
			continue
		}
		for _, task := range tasks {
			created, err := time.Parse(time.RFC3339, task.CreatedAt)
			if err != nil {
				continue
			}
			// For active tasks, use StartedAt if available.
			ref := created
			if check.stage == planner.StageActive && task.StartedAt != "" {
				if started, err := time.Parse(time.RFC3339, task.StartedAt); err == nil {
					ref = started
				}
			}
			age := now.Sub(ref)
			if age < check.maxAge {
				continue
			}

			var unit string
			var count int
			if check.maxAge >= 24*time.Hour {
				count = int(age.Hours() / 24)
				unit = "days"
			} else {
				count = int(age.Hours())
				unit = "hours"
			}
			_ = unit // used in message formatting
			msg := fmt.Sprintf("Task #%d (%s) — "+check.message, task.ID, task.Title, count)
			findings = append(findings, msg)
			taskIDs = append(taskIDs, task.ID)

			if check.action == "comment" {
				pm.postComment(task.ID, fmt.Sprintf("**[PM]** "+check.message+". Still relevant?", count))
			}
		}
	}

	// Priority decay: normal priority + backlog > 14 days → low
	backlog, _ := pm.planner.List(planner.TaskFilter{Stage: planner.StageBacklog})
	for _, task := range backlog {
		if task.Priority != 1 {
			continue
		}
		created, err := time.Parse(time.RFC3339, task.CreatedAt)
		if err != nil {
			continue
		}
		if now.Sub(created) > 14*24*time.Hour {
			lowPriority := 0
			if err := pm.planner.Update(task.ID, planner.TaskUpdate{Priority: &lowPriority}); err == nil {
				pm.postComment(task.ID, "**[PM]** Priority decayed to Low — in backlog for 14+ days with no action.")
				findings = append(findings, fmt.Sprintf("Task #%d (%s) — priority decayed to low (14+ days in backlog)", task.ID, task.Title))
				taskIDs = append(taskIDs, task.ID)
			}
		}
	}

	// Broadcast batched findings.
	if len(findings) > 0 {
		var b strings.Builder
		b.WriteString("**[PM Board Review]**\n")
		for _, f := range findings {
			fmt.Fprintf(&b, "- %s\n", f)
		}
		pm.broadcastPM("warning", b.String(), taskIDs, "periodic_sweep")
		log.Printf("[pm] sweep found %d issues", len(findings))
	} else {
		log.Println("[pm] sweep: board is clean")
	}
}
```

**Step 2: Verify build**

Run: `go build ./...`
Expected: Success

---

## Task 7: Wire PMService into Server

**Files:**
- Modify: `internal/server/server.go:31-50` (Server struct)
- Modify: `internal/server/server.go:54-95` (New constructor)
- Modify: `internal/server/tasks.go:55-69` (handleTaskCreate)
- Modify: `internal/server/agent.go:1636-1654` (toolTaskCreate)

**Step 1: Add pm field to Server struct**

In `server.go`, add to the Server struct (after line 39, the `commentWatcher` field):

```go
pm             *PMService
```

**Step 2: Initialize PMService in New()**

In the `New()` function in `server.go`, after the `s.processor = ...` line (line 75) and before `s.registerRoutes()`:

```go
s.pm = NewPMService(plannerStore, s.broadcast, aiClient, cfg.Model)
s.pm.Start()
```

**Step 3: Also initialize in NewWithWebFS()**

Find the equivalent position in `NewWithWebFS()` and add the same two lines after `s.processor = ...`.

**Step 4: Call pm.AfterCreate in handleTaskCreate**

In `tasks.go`, after the `s.broadcastTaskEvent("task.created", created)` line (line 68), add:

```go
// Run PM checks asynchronously.
if s.pm != nil {
    s.pm.AfterCreate(*created)
}
```

**Step 5: Call pm.AfterCreate in toolTaskCreate**

In `agent.go`, in `toolTaskCreate`, after the broadcast block (after line 1646), add:

```go
// Run PM checks asynchronously.
// Access pm via the processor field (set when processor is wired).
if a.processor != nil && a.processor.server != nil && a.processor.server.pm != nil {
    a.processor.server.pm.AfterCreate(task)
}
```

Wait — the AgentLoop doesn't have direct access to the server. Let me check how to wire this. The agent has `a.broadcast` and `a.planner`. The simplest approach: add a `pm *PMService` field to AgentLoop.

Actually, let's add a pm field to AgentLoop instead:

In `agent.go`, add to the AgentLoop struct (after the `processor` field, line 100):

```go
pm            *PMService    // PM service for after-create hooks
```

Then in `toolTaskCreate`, after the broadcast block:

```go
if a.pm != nil {
    a.pm.AfterCreate(task)
}
```

And wire it in `ws.go` where the agent is created (the `handleChatSend` function), after `agent.processor = s.processor`:

```go
agent.pm = s.pm
```

And in `autonomous.go` where the agent is created in `processTask`, add the same:

```go
agent.pm = tp.server.pm
```

Wait — TaskProcessor doesn't have a `server` field. Let me check what it has access to. The TaskProcessor has `planner`, `broadcast`, `ai`, `model`. We should add pm to TaskProcessor instead.

Simpler approach: add `pm *PMService` to TaskProcessor, wire it in `New()`.

In `server.go`, after `s.pm = NewPMService(...)` and before `s.registerRoutes()`:

```go
s.processor.pm = s.pm
```

In `autonomous.go`, add `pm *PMService` to the TaskProcessor struct. Then in `processTask` where the agent is created:

```go
agent.pm = tp.pm
```

**Step 6: Verify build**

Run: `go build ./...`
Expected: Success

---

## Task 8: Frontend — pm.notification Handler in useChat

**Files:**
- Modify: `web/src/hooks/useChat.ts` (add pm.notification case)
- Modify: `web/src/lib/types.ts` (add system role to ChatMessage if needed)

**Step 1: Check ChatMessage type**

Read `web/src/lib/types.ts` to see the ChatMessage interface and determine if we need a `system` role.

**Step 2: Add pm.notification handler in useChat.ts**

In the `onMessage` switch block (after the `chat.done` case around line 110), add:

```typescript
case 'pm.notification': {
  const data = msg.data as { severity: string; task_ids: number[]; check: string };
  setMessages((prev) => [
    ...prev,
    {
      id: uuid(),
      role: 'assistant' as const,
      content: msg.content ?? '',
      toolCalls: [],
      timestamp: new Date(),
      pmNotification: {
        severity: data.severity,
        taskIds: data.task_ids,
        check: data.check,
      },
    },
  ]);
  break;
}
```

**Step 3: Add pmNotification to ChatMessage type**

In `types.ts`, add to the ChatMessage interface:

```typescript
pmNotification?: {
  severity: 'info' | 'warning' | 'error';
  taskIds: number[];
  check: string;
};
```

**Step 4: Verify build**

Run: `cd web && npx tsc --noEmit`
Expected: Success

---

## Task 9: Frontend — Render pm.notification in Chat

**Files:**
- Modify: `web/src/components/chat/MessageBubble.tsx` (or wherever messages are rendered)

**Step 1: Check how messages are rendered**

Read the message rendering component to understand the structure.

**Step 2: Add PM notification styling**

In the message rendering component, add a check: if `message.pmNotification` exists, render with PM-specific styling:

```tsx
{message.pmNotification && (
  <div className={`rounded-lg px-3 py-2 text-xs font-body border-l-2 ${
    message.pmNotification.severity === 'error'
      ? 'border-stage-blocked bg-stage-blocked/5 text-stage-blocked'
      : message.pmNotification.severity === 'warning'
      ? 'border-amber-500 bg-amber-500/5 text-amber-400'
      : 'border-fg-muted bg-overlay text-fg-secondary'
  }`}>
    <div className="prose prose-invert prose-sm max-w-none">
      <ReactMarkdown remarkPlugins={[remarkGfm]}>
        {message.content}
      </ReactMarkdown>
    </div>
  </div>
)}
```

If the message has `pmNotification`, render it as a styled system card instead of the normal assistant bubble.

**Step 3: Verify build**

Run: `cd web && npx tsc --noEmit && npx vite build`
Expected: Success

---

## Task 10: Frontend — NewTaskForm Validation

**Files:**
- Modify: `web/src/components/planner/NewTaskForm.tsx`

**Step 1: Add validation function and warning/error state**

Add state and validation logic before the `handleSubmit`:

```tsx
const [warnings, setWarnings] = useState<string[]>([]);
const [errors, setErrors] = useState<string[]>([]);

const validate = useCallback(() => {
  const w: string[] = [];
  const e: string[] = [];

  if (description.trim() === '') {
    w.push('Tasks without descriptions are harder to execute. Add details?');
  } else if (description.trim().length < 30) {
    w.push('Consider adding acceptance criteria (use - [ ] checklists)');
  }

  if (priority === 3 && description.trim() === '') {
    e.push('Critical tasks require a description');
  }

  if (title.trim().length > 0 && title.trim().length < 10) {
    w.push('Try a more specific title (e.g., "Add logout button to sidebar")');
  }

  setWarnings(w);
  setErrors(e);
  return e.length === 0;
}, [title, description, priority]);
```

**Step 2: Run validation on field change**

Add a `useEffect` that runs validation when fields change:

```tsx
useEffect(() => { validate(); }, [validate]);
```

**Step 3: Gate submit on errors**

Update the submit button's `disabled` condition:

```tsx
disabled={!title.trim() || !product.trim() || submitting || errors.length > 0}
```

Update `handleSubmit` to check validation:

```tsx
const handleSubmit = async (e: React.FormEvent) => {
  e.preventDefault();
  if (!validate()) return;
  if (!title.trim() || !product.trim()) return;
  // ... rest unchanged
```

**Step 4: Render warnings and errors above the buttons**

Before the button row (between the priority/product row and the tip/buttons row), add:

```tsx
{(warnings.length > 0 || errors.length > 0) && (
  <div className="space-y-1">
    {errors.map((e, i) => (
      <p key={`e-${i}`} className="text-xs text-stage-blocked font-body">{e}</p>
    ))}
    {warnings.map((w, i) => (
      <p key={`w-${i}`} className="text-xs text-amber-400 font-body">{w}</p>
    ))}
  </div>
)}
```

**Step 5: Verify build**

Run: `cd web && npx tsc --noEmit && npx vite build`
Expected: Success

---

## Task 11: Build, Restart, Smoke Test

**Step 1: Build Go binary**

Run: `go build -o soul ./cmd/soul`
Expected: Success

**Step 2: Build frontend**

Run: `cd web && npx vite build`
Expected: Success

**Step 3: Restart server**

Run: `pkill -f './soul serve'; sleep 3 && SOUL_HOST=0.0.0.0 ./soul serve &`

**Step 4: Verify PM service started**

Check logs for: `[pm] service started (sweep every 30m)`

**Step 5: Test via API — create a task with short title**

```bash
curl -s http://localhost:3000/api/tasks -X POST \
  -H 'Content-Type: application/json' \
  -d '{"title": "Fix stuff", "product": "soul"}' | python3 -m json.tool
```

Expected: Task created, but PM posts a comment about missing description.

**Step 6: Test via API — create a task with good details**

```bash
curl -s http://localhost:3000/api/tasks -X POST \
  -H 'Content-Type: application/json' \
  -d '{"title": "Add logout button to user settings sidebar", "description": "Add a logout button at the bottom of the sidebar settings panel.\n\n- [ ] Button renders below settings list\n- [ ] Clicking it clears session and redirects to login", "priority": 1, "product": "soul"}' | python3 -m json.tool
```

Expected: Task created, no PM comments (clean task).

---

## Key Files Reference

| File | Lines | What |
|------|-------|------|
| `internal/server/pm.go` | NEW | PMService: struct, Levenshtein, AfterCreate, sweep, enrichment, decomposition |
| `internal/server/server.go` | 31-95 | Server struct + New() — add pm field + initialization |
| `internal/server/tasks.go` | 55-69 | handleTaskCreate — add pm.AfterCreate call |
| `internal/server/agent.go` | 22-80 | systemPrompt — add PM rules |
| `internal/server/agent.go` | 86-101 | AgentLoop struct — add pm field |
| `internal/server/agent.go` | 1619-1654 | toolTaskCreate — add title validation + pm.AfterCreate |
| `internal/server/autonomous.go` | TaskProcessor struct | Add pm field |
| `internal/server/ws.go` | handleChatSend | Wire agent.pm |
| `web/src/hooks/useChat.ts` | ~110 | Add pm.notification handler |
| `web/src/lib/types.ts` | ChatMessage | Add pmNotification field |
| `web/src/components/chat/MessageBubble.tsx` | rendering | PM notification styling |
| `web/src/components/planner/NewTaskForm.tsx` | full file | Validation warnings/errors |
