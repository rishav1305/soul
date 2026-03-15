# Chat Product Routing Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Give Soul's chat tool-calling capability to interact with product servers (Tasks, Tutor, Projects, Observe) through Claude's function-calling API, activated per-session via a tool button.

**Architecture:** New `internal/chat/context/` package provides system prompts and tool definitions per product. Handler injects these into Claude API requests, intercepts tool_use responses, dispatches them to product REST APIs, and loops until Claude produces a final text response. Product selection is per-session, persisted in chat.db.

**Tech Stack:** Go 1.24, React 19, TypeScript 5.9, Claude API tool-use, SQLite

**Spec:** `docs/superpowers/specs/2026-03-15-chat-product-routing-design.md`

---

## File Structure

```
internal/chat/context/           NEW — product context provider
  context.go                     ProductContext struct, ForProduct(), Default()
  tasks.go                       Tasks system prompt + 6 tool definitions
  tutor.go                       Tutor system prompt + 7 tool definitions
  projects.go                    Projects system prompt + 6 tool definitions
  observe.go                     Observe system prompt + 4 tool definitions
  dispatch.go                    Tool call dispatcher — routes to product REST APIs
  context_test.go                Tests for context provider + dispatcher

internal/chat/session/
  store.go                       MODIFY — add product column, SetProduct(), product in Session struct
  iface.go                       MODIFY — add SetProduct to StoreInterface
  timed_store.go                 MODIFY — add SetProduct delegation

internal/chat/stream/
  types.go                       MODIFY — relax Validate() for tool-use sequences

internal/chat/ws/
  handler.go                     MODIFY — product context injection, tool loop, setProduct handler
  message.go                     MODIFY — add session.setProduct type, product field on InboundMessage

web/src/
  hooks/useChat.ts               MODIFY — product state, setProduct/productSet handlers
  components/ChatInput.tsx        MODIFY — tool button with product selector popover
```

---

## Chunk 1: Backend Foundation

### Task 1: Session Product Column

Add `product` field to sessions table and Session struct.

**Files:**
- Modify: `internal/chat/session/store.go:59-68` (Session struct), `store.go:103-128` (schema/migration)
- Modify: `internal/chat/session/iface.go:7-21` (StoreInterface)
- Modify: `internal/chat/session/timed_store.go` (TimedStore delegation)

- [ ] **Step 1: Add product field to Session struct**

In `store.go`, add `Product` field to the Session struct (after line 66):

```go
type Session struct {
	ID           string    `json:"id"`
	Title        string    `json:"title"`
	Status       Status    `json:"status"`
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
	MessageCount int       `json:"messageCount"`
	LastMessage  string    `json:"lastMessage"`
	UnreadCount  int       `json:"unreadCount"`
	Product      string    `json:"product"`
}
```

- [ ] **Step 2: Add product column to schema**

In `store.go`, update the `createTableSQL` to include the product column. Add migration in `Open()` to alter existing tables:

```go
// In createTableSQL, sessions table — add after unread_count:
//   product TEXT NOT NULL DEFAULT ''

// Migration (after existing migrations in Open()):
_, _ = db.Exec(`ALTER TABLE sessions ADD COLUMN product TEXT NOT NULL DEFAULT ''`)
```

- [ ] **Step 3: Update session scan helpers**

Update all `Scan()` calls that read Session rows to include `&s.Product`. Key locations:
- `GetSession()` scan
- `ListSessions()` scan
- `CreateSession()` return scan
- Any other method returning a Session

- [ ] **Step 4: Add SetProduct method to Store**

```go
func (s *Store) SetProduct(sessionID, product string) error {
	valid := map[string]bool{"": true, "tasks": true, "tutor": true, "projects": true, "observe": true}
	if !valid[product] {
		return fmt.Errorf("invalid product: %q", product)
	}
	result, err := s.db.Exec(
		`UPDATE sessions SET product = ?, updated_at = ? WHERE id = ?`,
		product, time.Now().UTC(), sessionID,
	)
	if err != nil {
		return fmt.Errorf("set product: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("session not found: %s", sessionID)
	}
	return nil
}
```

- [ ] **Step 5: Add SetProduct to StoreInterface**

In `iface.go`, add to the interface:

```go
SetProduct(sessionID, product string) error
```

- [ ] **Step 6: Add SetProduct to TimedStore**

In `timed_store.go`, add delegation:

```go
func (ts *TimedStore) SetProduct(sessionID, product string) error {
	start := time.Now()
	err := ts.inner.SetProduct(sessionID, product)
	ts.logQuery("SetProduct", start, sessionID)
	return err
}
```

- [ ] **Step 7: Verify compilation**

Run: `go build ./internal/chat/...`
Expected: Clean build

- [ ] **Step 8: Commit**

```bash
git add internal/chat/session/
git commit -m "feat: add product column to sessions for chat-product routing"
```

---

### Task 2: Product Context Provider

Create the context package with system prompts and tool definitions.

**Files:**
- Create: `internal/chat/context/context.go`
- Create: `internal/chat/context/tasks.go`
- Create: `internal/chat/context/tutor.go`
- Create: `internal/chat/context/projects.go`
- Create: `internal/chat/context/observe.go`

- [ ] **Step 1: Create context.go — core types and dispatcher**

```go
package context

import "github.com/rishav1305/soul-v2/internal/chat/stream"

// ProductContext holds system prompt and tool definitions for a product.
type ProductContext struct {
	System string
	Tools  []stream.Tool
}

// ForProduct returns the context for a named product.
// Returns Default() for empty or unknown product names.
func ForProduct(product string) ProductContext {
	switch product {
	case "tasks":
		return tasksContext()
	case "tutor":
		return tutorContext()
	case "projects":
		return projectsContext()
	case "observe":
		return observeContext()
	default:
		return Default()
	}
}

// Default returns a lightweight system prompt with no tools.
func Default() ProductContext {
	return ProductContext{
		System: `You are Soul, an AI development assistant. You are part of Soul v2 — a platform with 4 products: Tasks (autonomous task execution), Tutor (interview prep with spaced repetition), Projects (skill-building project tracking), and Observe (observability metrics dashboard). The user can select a product using the tool button to enable product-specific actions. Without a product selected, you are a general-purpose assistant.`,
	}
}
```

- [ ] **Step 2: Create tasks.go — Tasks product context**

```go
package context

import (
	"encoding/json"

	"github.com/rishav1305/soul-v2/internal/chat/stream"
)

func tasksContext() ProductContext {
	return ProductContext{
		System: `You are Soul's Tasks assistant. Tasks is an autonomous task execution engine — users create tasks, and an AI agent executes them in isolated git worktrees with verification gates.

Available tools let you list, create, update, start, and stop tasks. Use list_tasks to check current state before creating duplicates. Use start_task only when the user explicitly requests execution.

Key concepts: Tasks have stages (backlog → active → validation → done/blocked). Each task belongs to a product. The executor runs autonomously with step-verify-fix loops.

The user may reference other Soul products. If the question is about a different product, suggest they switch using the tool button.`,
		Tools: []stream.Tool{
			{
				Name:        "list_tasks",
				Description: "List tasks, optionally filtered by stage (backlog, active, validation, done, blocked) and/or product.",
				InputSchema: mustJSON(`{"type":"object","properties":{"stage":{"type":"string","enum":["backlog","active","validation","done","blocked"],"description":"Filter by stage"},"product":{"type":"string","description":"Filter by product name"}}}`),
			},
			{
				Name:        "create_task",
				Description: "Create a new task in the backlog.",
				InputSchema: mustJSON(`{"type":"object","properties":{"title":{"type":"string","description":"Task title"},"description":{"type":"string","description":"Detailed task description"}},"required":["title","description"]}`),
			},
			{
				Name:        "get_task",
				Description: "Get full details of a specific task by ID.",
				InputSchema: mustJSON(`{"type":"object","properties":{"task_id":{"type":"integer","description":"Task ID"}},"required":["task_id"]}`),
			},
			{
				Name:        "update_task",
				Description: "Update a task's fields (title, description, stage, product).",
				InputSchema: mustJSON(`{"type":"object","properties":{"task_id":{"type":"integer","description":"Task ID"},"title":{"type":"string"},"description":{"type":"string"},"stage":{"type":"string","enum":["backlog","active","validation","done","blocked"]},"product":{"type":"string"}},"required":["task_id"]}`),
			},
			{
				Name:        "start_task",
				Description: "Start autonomous execution of a task. Only use when user explicitly requests it.",
				InputSchema: mustJSON(`{"type":"object","properties":{"task_id":{"type":"integer","description":"Task ID"}},"required":["task_id"]}`),
			},
			{
				Name:        "stop_task",
				Description: "Stop a currently running task.",
				InputSchema: mustJSON(`{"type":"object","properties":{"task_id":{"type":"integer","description":"Task ID"}},"required":["task_id"]}`),
			},
		},
	}
}

func mustJSON(s string) json.RawMessage {
	var js json.RawMessage = []byte(s)
	if !json.Valid(js) {
		panic("invalid JSON in tool schema: " + s)
	}
	return js
}
```

- [ ] **Step 3: Create tutor.go — Tutor product context**

```go
package context

import "github.com/rishav1305/soul-v2/internal/chat/stream"

func tutorContext() ProductContext {
	return ProductContext{
		System: `You are Soul's Tutor assistant. Tutor is an interview preparation system with 5 modules: DSA, AI/ML, Behavioral, Mock Interview, and Study Planner. It uses SM-2 spaced repetition for drill scheduling.

Available tools let you view progress, browse topics, run drills, and manage mock interviews. Use tutor_dashboard first to understand the user's current progress before suggesting actions. Use due_reviews to check what's ready for review.

Key concepts: Topics belong to modules. Drills use spaced repetition (SM-2) — questions come due based on past performance. Mock interviews have dimension scoring.

The user may reference other Soul products. If the question is about a different product, suggest they switch using the tool button.`,
		Tools: []stream.Tool{
			{
				Name:        "tutor_dashboard",
				Description: "Get the tutor dashboard showing overall progress across all modules.",
				InputSchema: mustJSON(`{"type":"object","properties":{}}`),
			},
			{
				Name:        "list_topics",
				Description: "List topics, optionally filtered by module (dsa, ai, behavioral).",
				InputSchema: mustJSON(`{"type":"object","properties":{"module":{"type":"string","enum":["dsa","ai","behavioral"],"description":"Filter by module"}}}`),
			},
			{
				Name:        "start_drill",
				Description: "Start a spaced-repetition drill session for a topic.",
				InputSchema: mustJSON(`{"type":"object","properties":{"topic_id":{"type":"integer","description":"Topic ID to drill"}},"required":["topic_id"]}`),
			},
			{
				Name:        "answer_drill",
				Description: "Submit an answer to a drill question.",
				InputSchema: mustJSON(`{"type":"object","properties":{"question_id":{"type":"integer","description":"Question ID"},"answer":{"type":"string","description":"User's answer"}},"required":["question_id","answer"]}`),
			},
			{
				Name:        "due_reviews",
				Description: "Get topics and questions that are due for review based on SM-2 schedule.",
				InputSchema: mustJSON(`{"type":"object","properties":{}}`),
			},
			{
				Name:        "create_mock",
				Description: "Create a new mock interview session.",
				InputSchema: mustJSON(`{"type":"object","properties":{"type":{"type":"string","description":"Interview type (e.g., technical, behavioral, system-design)"}},"required":["type"]}`),
			},
			{
				Name:        "list_mocks",
				Description: "List all mock interview sessions with scores.",
				InputSchema: mustJSON(`{"type":"object","properties":{}}`),
			},
		},
	}
}
```

- [ ] **Step 4: Create projects.go — Projects product context**

```go
package context

import "github.com/rishav1305/soul-v2/internal/chat/stream"

func projectsContext() ProductContext {
	return ProductContext{
		System: `You are Soul's Projects assistant. Projects is a skill-building project tracker with milestones, metrics, readiness assessments, and implementation guides. Each project targets specific technical skills.

Available tools let you view dashboards, get project details, update progress, record metrics, and access implementation guides. Use projects_dashboard first to see all projects before diving into specifics.

Key concepts: Projects have milestones (deliverables), metrics (quantitative measurements), readiness scores (can-explain, can-demo, can-tradeoffs), and embedded implementation guides.

The user may reference other Soul products. If the question is about a different product, suggest they switch using the tool button.`,
		Tools: []stream.Tool{
			{
				Name:        "projects_dashboard",
				Description: "Get dashboard showing all projects with status summaries.",
				InputSchema: mustJSON(`{"type":"object","properties":{}}`),
			},
			{
				Name:        "get_project",
				Description: "Get full project detail including milestones, metrics, readiness, and sync history.",
				InputSchema: mustJSON(`{"type":"object","properties":{"project_id":{"type":"integer","description":"Project ID"}},"required":["project_id"]}`),
			},
			{
				Name:        "update_project",
				Description: "Update a project's status or hours.",
				InputSchema: mustJSON(`{"type":"object","properties":{"project_id":{"type":"integer","description":"Project ID"},"status":{"type":"string","enum":["not_started","in_progress","completed"]},"hours_actual":{"type":"number"}},"required":["project_id"]}`),
			},
			{
				Name:        "update_milestone",
				Description: "Update a milestone's status.",
				InputSchema: mustJSON(`{"type":"object","properties":{"project_id":{"type":"integer","description":"Project ID"},"milestone_id":{"type":"integer","description":"Milestone ID"},"status":{"type":"string","enum":["not_started","in_progress","done"]}},"required":["project_id","milestone_id","status"]}`),
			},
			{
				Name:        "record_metric",
				Description: "Record a metric value for a project.",
				InputSchema: mustJSON(`{"type":"object","properties":{"project_id":{"type":"integer","description":"Project ID"},"name":{"type":"string","description":"Metric name"},"value":{"type":"number","description":"Metric value"},"unit":{"type":"string","description":"Unit of measurement"}},"required":["project_id","name","value"]}`),
			},
			{
				Name:        "get_guide",
				Description: "Get the implementation guide for a project (markdown).",
				InputSchema: mustJSON(`{"type":"object","properties":{"project_id":{"type":"integer","description":"Project ID"}},"required":["project_id"]}`),
			},
		},
	}
}
```

- [ ] **Step 5: Create observe.go — Observe product context**

```go
package context

import "github.com/rishav1305/soul-v2/internal/chat/stream"

func observeContext() ProductContext {
	return ProductContext{
		System: `You are Soul's Observe assistant. Observe is the observability dashboard showing system health across 6 design pillars: Performant, Robust, Resilient, Secure, Sovereign, and Transparent. It aggregates metrics from all Soul products.

Available tools let you check system overview, pillar health, recent events, and active alerts. Use observe_overview for a quick health check. Use observe_pillars for detailed constraint compliance.

Key concepts: Each pillar has constraints with targets (e.g., "first-token < 200ms"). Constraints are pass/warn/fail. Events are stored as JSONL and filterable by product and type.

The user may reference other Soul products. If the question is about a different product, suggest they switch using the tool button.`,
		Tools: []stream.Tool{
			{
				Name:        "observe_overview",
				Description: "Get system overview: status, cost summary, and active alerts.",
				InputSchema: mustJSON(`{"type":"object","properties":{"product":{"type":"string","description":"Filter by product (chat, tasks, tutor, projects)"}}}`),
			},
			{
				Name:        "observe_pillars",
				Description: "Get pillar constraint health across all 6 design pillars.",
				InputSchema: mustJSON(`{"type":"object","properties":{"product":{"type":"string","description":"Filter by product"}}}`),
			},
			{
				Name:        "observe_tail",
				Description: "Get recent events, newest first. Optionally filter by event type prefix.",
				InputSchema: mustJSON(`{"type":"object","properties":{"type":{"type":"string","description":"Event type prefix filter (e.g., 'api.request')"},"limit":{"type":"integer","description":"Max events to return (default 50, max 500)"},"product":{"type":"string","description":"Filter by product"}}}`),
			},
			{
				Name:        "observe_alerts",
				Description: "Get active alerts — threshold violations detected in metrics.",
				InputSchema: mustJSON(`{"type":"object","properties":{"product":{"type":"string","description":"Filter by product"}}}`),
			},
		},
	}
}
```

- [ ] **Step 6: Verify compilation**

Run: `go build ./internal/chat/...`
Expected: Clean build

- [ ] **Step 7: Write tests for context package**

Create `internal/chat/context/context_test.go`:

```go
package context

import (
	"encoding/json"
	"testing"
)

func TestForProduct_ReturnsCorrectContext(t *testing.T) {
	products := []string{"tasks", "tutor", "projects", "observe"}
	for _, p := range products {
		ctx := ForProduct(p)
		if ctx.System == "" {
			t.Errorf("ForProduct(%q): empty system prompt", p)
		}
		if len(ctx.Tools) == 0 {
			t.Errorf("ForProduct(%q): no tools defined", p)
		}
		for _, tool := range ctx.Tools {
			if tool.Name == "" {
				t.Errorf("ForProduct(%q): tool with empty name", p)
			}
			if !json.Valid(tool.InputSchema) {
				t.Errorf("ForProduct(%q): tool %q has invalid input schema", p, tool.Name)
			}
		}
	}
}

func TestForProduct_UnknownReturnsDefault(t *testing.T) {
	ctx := ForProduct("unknown")
	def := Default()
	if ctx.System != def.System {
		t.Error("unknown product should return default context")
	}
	if len(ctx.Tools) != 0 {
		t.Error("default context should have no tools")
	}
}

func TestForProduct_EmptyReturnsDefault(t *testing.T) {
	ctx := ForProduct("")
	if len(ctx.Tools) != 0 {
		t.Error("empty product should return default context with no tools")
	}
}

func TestToolCounts(t *testing.T) {
	expected := map[string]int{
		"tasks": 6, "tutor": 7, "projects": 6, "observe": 4,
	}
	for product, count := range expected {
		ctx := ForProduct(product)
		if len(ctx.Tools) != count {
			t.Errorf("%s: expected %d tools, got %d", product, count, len(ctx.Tools))
		}
	}
}
```

Run: `go test ./internal/chat/context/ -v`
Expected: All tests pass

- [ ] **Step 8: Commit**

```bash
git add internal/chat/context/
git commit -m "feat: add product context provider with system prompts and tool definitions"
```

---

### Task 3: Tool Call Dispatcher

Create the dispatcher that routes tool calls to product server REST APIs.

**Files:**
- Create: `internal/chat/context/dispatch.go`
- Add tests to: `internal/chat/context/context_test.go`

- [ ] **Step 1: Create dispatch.go**

```go
package context

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

// ToolRoute maps a tool name to an HTTP endpoint on a product server.
type ToolRoute struct {
	Method  string // GET, POST, PATCH, DELETE
	Path    string // e.g., "/api/tasks/{task_id}"
	Product string // tasks, tutor, projects, observe
}

// Dispatcher routes tool calls to product server REST APIs.
type Dispatcher struct {
	client *http.Client
	routes map[string]ToolRoute
	urls   map[string]string // product → base URL
}

// NewDispatcher creates a dispatcher with a shared HTTP client.
func NewDispatcher() *Dispatcher {
	d := &Dispatcher{
		client: &http.Client{
			Transport: &http.Transport{
				MaxIdleConns:        20,
				MaxIdleConnsPerHost: 5,
				IdleConnTimeout:     90 * time.Second,
			},
		},
		urls: map[string]string{
			"tasks":    envOr("SOUL_TASKS_URL", "http://127.0.0.1:3004"),
			"tutor":    envOr("SOUL_TUTOR_URL", "http://127.0.0.1:3006"),
			"projects": envOr("SOUL_PROJECTS_URL", "http://127.0.0.1:3008"),
			"observe":  envOr("SOUL_OBSERVE_URL", "http://127.0.0.1:3010"),
		},
		routes: map[string]ToolRoute{
			// Tasks
			"list_tasks":  {Method: "GET", Path: "/api/tasks", Product: "tasks"},
			"create_task": {Method: "POST", Path: "/api/tasks", Product: "tasks"},
			"get_task":    {Method: "GET", Path: "/api/tasks/{task_id}", Product: "tasks"},
			"update_task": {Method: "PATCH", Path: "/api/tasks/{task_id}", Product: "tasks"},
			"start_task":  {Method: "POST", Path: "/api/tasks/{task_id}/start", Product: "tasks"},
			"stop_task":   {Method: "POST", Path: "/api/tasks/{task_id}/stop", Product: "tasks"},
			// Tutor
			"tutor_dashboard": {Method: "GET", Path: "/api/tutor/dashboard", Product: "tutor"},
			"list_topics":     {Method: "GET", Path: "/api/tutor/topics", Product: "tutor"},
			"start_drill":     {Method: "POST", Path: "/api/tutor/drill/start", Product: "tutor"},
			"answer_drill":    {Method: "POST", Path: "/api/tutor/drill/answer", Product: "tutor"},
			"due_reviews":     {Method: "GET", Path: "/api/tutor/drill/due", Product: "tutor"},
			"create_mock":     {Method: "POST", Path: "/api/tutor/mocks", Product: "tutor"},
			"list_mocks":      {Method: "GET", Path: "/api/tutor/mocks", Product: "tutor"},
			// Projects
			"projects_dashboard": {Method: "GET", Path: "/api/projects/dashboard", Product: "projects"},
			"get_project":        {Method: "GET", Path: "/api/projects/{project_id}", Product: "projects"},
			"update_project":     {Method: "PATCH", Path: "/api/projects/{project_id}", Product: "projects"},
			"update_milestone":   {Method: "PATCH", Path: "/api/projects/{project_id}/milestones/{milestone_id}", Product: "projects"},
			"record_metric":      {Method: "POST", Path: "/api/projects/{project_id}/metrics", Product: "projects"},
			"get_guide":          {Method: "GET", Path: "/api/projects/{project_id}/guide", Product: "projects"},
			// Observe
			"observe_overview": {Method: "GET", Path: "/api/overview", Product: "observe"},
			"observe_pillars":  {Method: "GET", Path: "/api/pillars", Product: "observe"},
			"observe_tail":     {Method: "GET", Path: "/api/tail", Product: "observe"},
			"observe_alerts":   {Method: "GET", Path: "/api/alerts", Product: "observe"},
		},
	}
	return d
}

const maxToolResultBytes = 50 * 1024 // 50KB

// Execute dispatches a tool call to the appropriate product server.
// Returns the response body as a string for use as a tool_result.
func (d *Dispatcher) Execute(ctx context.Context, toolName string, input json.RawMessage) (string, error) {
	route, ok := d.routes[toolName]
	if !ok {
		return "", fmt.Errorf("unknown tool: %s", toolName)
	}

	baseURL, ok := d.urls[route.Product]
	if !ok {
		return "", fmt.Errorf("no URL configured for product: %s", route.Product)
	}

	// Parse input.
	var params map[string]interface{}
	if len(input) > 0 {
		if err := json.Unmarshal(input, &params); err != nil {
			return "", fmt.Errorf("invalid tool input: %w", err)
		}
	}
	if params == nil {
		params = make(map[string]interface{})
	}

	// Substitute path parameters and collect remaining for query/body.
	path := route.Path
	remaining := make(map[string]interface{})
	for k, v := range params {
		placeholder := "{" + k + "}"
		if strings.Contains(path, placeholder) {
			path = strings.ReplaceAll(path, placeholder, fmt.Sprintf("%v", v))
		} else {
			remaining[k] = v
		}
	}

	fullURL := baseURL + path

	var body io.Reader
	if route.Method == "GET" {
		// Remaining params become query string.
		if len(remaining) > 0 {
			q := url.Values{}
			for k, v := range remaining {
				q.Set(k, fmt.Sprintf("%v", v))
			}
			fullURL += "?" + q.Encode()
		}
	} else {
		// POST/PATCH — remaining params become JSON body.
		if len(remaining) > 0 {
			b, err := json.Marshal(remaining)
			if err != nil {
				return "", fmt.Errorf("marshal body: %w", err)
			}
			body = bytes.NewReader(b)
		}
	}

	// Create request with timeout.
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, route.Method, fullURL, body)
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := d.client.Do(req)
	if err != nil {
		return fmt.Sprintf("Error calling %s: %v", toolName, err), nil
	}
	defer resp.Body.Close()

	// Read response, truncate if too large.
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxToolResultBytes+1))
	if err != nil {
		return fmt.Sprintf("Error reading response from %s: %v", toolName, err), nil
	}

	result := string(data)
	if len(data) > maxToolResultBytes {
		result = result[:maxToolResultBytes] + "\n...(truncated)"
	}

	if resp.StatusCode >= 400 {
		return fmt.Sprintf("Error from %s (HTTP %d): %s", toolName, resp.StatusCode, result), nil
	}

	return result, nil
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
```

Note: Add `"context"` to the import block.

- [ ] **Step 2: Add dispatcher tests**

Append to `context_test.go`:

```go
func TestDispatcher_UnknownTool(t *testing.T) {
	d := NewDispatcher()
	_, err := d.Execute(context.Background(), "nonexistent_tool", nil)
	if err == nil || !strings.Contains(err.Error(), "unknown tool") {
		t.Errorf("expected unknown tool error, got: %v", err)
	}
}

func TestDispatcher_RoutesExist(t *testing.T) {
	d := NewDispatcher()
	// Every tool defined in any product context should have a route.
	for _, product := range []string{"tasks", "tutor", "projects", "observe"} {
		ctx := ForProduct(product)
		for _, tool := range ctx.Tools {
			if _, ok := d.routes[tool.Name]; !ok {
				t.Errorf("tool %q (product %s) has no dispatcher route", tool.Name, product)
			}
		}
	}
}
```

Add `"context"` and `"strings"` to test imports.

Run: `go test ./internal/chat/context/ -v`
Expected: All tests pass

- [ ] **Step 3: Commit**

```bash
git add internal/chat/context/
git commit -m "feat: add tool call dispatcher routing to product REST APIs"
```

---

### Task 4: Stream Validation Relaxation

Update `Request.Validate()` to allow tool-use message sequences.

**Files:**
- Modify: `internal/chat/stream/types.go:21-51` (Validate method)

- [ ] **Step 1: Update Validate() to handle tool-use sequences**

The current validation enforces strict role alternation. In tool-use conversations, the sequence is `[...user, assistant, user(tool_result), assistant]` which already alternates. However, the validator may fail when building mid-loop requests. Add a bypass flag:

In `types.go`, add field to Request struct (after line 17):

```go
type Request struct {
	Model          string    `json:"model,omitempty"`
	MaxTokens      int       `json:"max_tokens"`
	System         string    `json:"system,omitempty"`
	Messages       []Message `json:"messages"`
	Tools          []Tool    `json:"tools,omitempty"`
	Stream         bool      `json:"stream"`
	SkipValidation bool      `json:"-"` // internal: skip role alternation check for tool loops
}
```

In `Validate()`, wrap the alternation check:

```go
// In Validate(), replace the role alternation loop with:
if !r.SkipValidation {
	for i := 1; i < len(r.Messages); i++ {
		if r.Messages[i].Role == r.Messages[i-1].Role {
			return fmt.Errorf("messages must alternate roles: message %d and %d both have role %q", i-1, i, r.Messages[i].Role)
		}
	}
}
```

- [ ] **Step 2: Verify existing tests still pass**

Run: `go test ./internal/chat/stream/ -v`
Expected: All existing tests pass (SkipValidation defaults to false)

- [ ] **Step 3: Commit**

```bash
git add internal/chat/stream/types.go
git commit -m "feat: add SkipValidation flag to stream.Request for tool-use loops"
```

---

## Chunk 2: Handler Integration

### Task 5: WebSocket Protocol — New Message Types

Add `session.setProduct` message type and product field.

**Files:**
- Modify: `internal/chat/ws/message.go:11-17` (types), `message.go:51-58` (struct)

- [ ] **Step 1: Add new message type constants**

In `message.go`, add to the inbound types (after line 17):

```go
TypeSessionSetProduct = "session.setProduct"
```

Add to the outbound types (after line 42):

```go
TypeSessionProductSet = "session.productSet"
```

- [ ] **Step 2: Add product field to InboundMessage**

In `message.go`, add to InboundMessage struct (after line 56):

```go
Product string `json:"product,omitempty"`
```

- [ ] **Step 3: Verify compilation**

Run: `go build ./internal/chat/ws/...`
Expected: Clean build

- [ ] **Step 4: Commit**

```bash
git add internal/chat/ws/message.go
git commit -m "feat: add session.setProduct WS message type and product field"
```

---

### Task 6: Handler — Product Context Injection and Tool Loop

The core handler changes: inject product context, handle tool_use responses, dispatch tools, loop.

**Files:**
- Modify: `internal/chat/ws/handler.go:21-26` (struct), `handler.go:53-77` (switch), `handler.go:82-205` (handleChatSend), `handler.go:207-369` (runStream)

- [ ] **Step 1: Add context provider and dispatcher to MessageHandler**

In `handler.go`, add imports:

```go
prodctx "github.com/rishav1305/soul-v2/internal/chat/context"
```

Add fields to MessageHandler struct (after line 25):

```go
type MessageHandler struct {
	hub          *Hub
	sessionStore session.StoreInterface
	streamClient *stream.Client
	metrics      *metrics.EventLogger
	dispatcher   *prodctx.Dispatcher
}
```

Update `NewMessageHandler()` to initialize the dispatcher:

```go
// In NewMessageHandler, after setting other fields:
h.dispatcher = prodctx.NewDispatcher()
```

- [ ] **Step 2: Add handleSessionSetProduct handler**

Add new handler method:

```go
func (h *MessageHandler) handleSessionSetProduct(client *Client, msg *InboundMessage) {
	if msg.SessionID == "" {
		h.sendError(client, "", "sessionId required")
		return
	}

	if err := h.sessionStore.SetProduct(msg.SessionID, msg.Product); err != nil {
		h.sendError(client, msg.SessionID, err.Error())
		return
	}

	// Broadcast confirmation.
	h.broadcast(TypeSessionProductSet, msg.SessionID, map[string]string{
		"product": msg.Product,
	})

	if h.metrics != nil {
		h.metrics.Log("session.setProduct", map[string]interface{}{
			"session_id": msg.SessionID,
			"product":    msg.Product,
		})
	}
}
```

Add to HandleMessage switch (after line 77):

```go
case TypeSessionSetProduct:
	h.handleSessionSetProduct(client, msg)
```

- [ ] **Step 3: Inject product context into handleChatSend**

In `handleChatSend`, after building the request (line 196), inject product context:

```go
// After: req := &stream.Request{...}

// Inject product context.
sess, _ := h.sessionStore.GetSession(sessionID)
if sess != nil {
	ctx := prodctx.ForProduct(sess.Product)
	req.System = ctx.System
	if len(ctx.Tools) > 0 {
		req.Tools = ctx.Tools
	}
}
```

- [ ] **Step 4: Add tool loop to runStream**

This is the most complex change. In `runStream`, after the existing event loop (line 301), add tool dispatch logic. The approach:

1. Track accumulated tool_use blocks during streaming
2. After stream ends, check if `stop_reason == "tool_use"`
3. If so, dispatch tools, build follow-up request, stream again

Add before the event loop setup:

```go
// Tool-use tracking.
var toolCalls []stream.ContentBlock
var currentToolInput strings.Builder
var stopReason string
```

In the event loop, add handling for tool_use events:

```go
case "content_block_start":
	if evt.ContentBlock != nil && evt.ContentBlock.Type == "tool_use" {
		toolCalls = append(toolCalls, *evt.ContentBlock)
		currentToolInput.Reset()
		// Send tool.call event to frontend.
		h.sendToClient(client, &OutboundMessage{
			Type:      "tool.call",
			SessionID: sessionID,
			Data: map[string]string{
				"name": evt.ContentBlock.Name,
				"id":   evt.ContentBlock.ID,
			},
		})
	}
```

```go
case "content_block_delta":
	if evt.Delta != nil && evt.Delta.PartialJSON != "" {
		currentToolInput.WriteString(evt.Delta.PartialJSON)
	} else if evt.Delta != nil && evt.Delta.Text != "" {
		// Existing text handling...
	}
```

```go
case "content_block_stop":
	// Finalize tool input if we have accumulated JSON.
	if currentToolInput.Len() > 0 && len(toolCalls) > 0 {
		last := &toolCalls[len(toolCalls)-1]
		last.Input = json.RawMessage(currentToolInput.String())
		currentToolInput.Reset()
	}
```

```go
case "message_delta":
	if evt.StopReason != "" {
		stopReason = evt.StopReason
	}
	// Existing output token tracking...
```

After the event loop, add the tool dispatch logic:

```go
// Tool dispatch loop.
maxToolRounds := 5
for round := 0; stopReason == "tool_use" && len(toolCalls) > 0 && round < maxToolRounds; round++ {
	// Store assistant message with tool_use blocks.
	assistantContent, _ := json.Marshal(toolCalls)
	h.sessionStore.AddMessage(sessionID, "tool_use", string(assistantContent))

	// Dispatch each tool call.
	var toolResults []map[string]interface{}
	for _, tc := range toolCalls {
		h.sendToClient(client, &OutboundMessage{
			Type:      "tool.call",
			SessionID: sessionID,
			Data:      map[string]string{"name": tc.Name, "id": tc.ID, "status": "running"},
		})

		result, err := h.dispatcher.Execute(streamCtx, tc.Name, tc.Input)
		if err != nil {
			result = fmt.Sprintf("Error: %v", err)
		}

		toolResults = append(toolResults, map[string]interface{}{
			"type":        "tool_result",
			"tool_use_id": tc.ID,
			"content":     result,
		})

		// Store tool result.
		resultJSON, _ := json.Marshal(map[string]string{
			"tool_use_id": tc.ID,
			"content":     result,
		})
		h.sessionStore.AddMessage(sessionID, "tool_result", string(resultJSON))

		h.sendToClient(client, &OutboundMessage{
			Type:      "tool.complete",
			SessionID: sessionID,
			Data:      map[string]string{"name": tc.Name, "id": tc.ID},
		})
	}

	// Build follow-up request with tool results.
	// Append assistant message with tool_use content blocks.
	var assistantBlocks []stream.ContentBlock
	if fullText.Len() > 0 {
		assistantBlocks = append(assistantBlocks, stream.ContentBlock{Type: "text", Text: fullText.String()})
	}
	assistantBlocks = append(assistantBlocks, toolCalls...)
	apiMessages = append(apiMessages, stream.Message{Role: "assistant", Content: assistantBlocks})

	// Append user message with tool_result content blocks.
	var resultBlocks []stream.ContentBlock
	for _, tr := range toolResults {
		resultBlocks = append(resultBlocks, stream.ContentBlock{
			Type:      "tool_result",
			ToolUseID: tr["tool_use_id"].(string),
			Content:   tr["content"].(string),
		})
	}
	apiMessages = append(apiMessages, stream.Message{Role: "user", Content: resultBlocks})

	// Create follow-up request.
	followUp := &stream.Request{
		MaxTokens:      4096,
		Messages:       apiMessages,
		Model:          req.Model,
		System:         req.System,
		Tools:          req.Tools,
		SkipValidation: true,
	}

	// Stream again.
	toolCalls = nil
	stopReason = ""
	fullText.Reset()

	ch2, err := h.streamClient.Stream(streamCtx, followUp)
	if err != nil {
		h.sendClassifiedError(client, sessionID, err)
		return
	}

	for evt := range ch2 {
		switch evt.Type {
		case "content_block_start":
			if evt.ContentBlock != nil && evt.ContentBlock.Type == "tool_use" {
				toolCalls = append(toolCalls, *evt.ContentBlock)
				currentToolInput.Reset()
			}
		case "content_block_delta":
			if evt.Delta != nil && evt.Delta.PartialJSON != "" {
				currentToolInput.WriteString(evt.Delta.PartialJSON)
			} else if evt.Delta != nil && evt.Delta.Text != "" {
				fullText.WriteString(evt.Delta.Text)
				h.sendToClient(client, NewChatToken(sessionID, evt.Delta.Text))
			}
		case "content_block_stop":
			if currentToolInput.Len() > 0 && len(toolCalls) > 0 {
				last := &toolCalls[len(toolCalls)-1]
				last.Input = json.RawMessage(currentToolInput.String())
				currentToolInput.Reset()
			}
		case "message_delta":
			if evt.StopReason != "" {
				stopReason = evt.StopReason
			}
		case "error":
			if evt.Error != nil {
				h.sendClassifiedError(client, sessionID, fmt.Errorf("%s: %s", evt.Error.Type, evt.Error.Message))
				return
			}
		}
	}
}

// If tool loop hit limit, inject warning.
if stopReason == "tool_use" {
	fullText.WriteString("\n\n(Tool call limit reached. Responding with available information.)")
}
```

Note: This requires `apiMessages` to be accessible after the initial request is built — it needs to be in scope for the tool loop. Ensure the variable is declared at the `runStream` level, not just in `handleChatSend`.

- [ ] **Step 5: Update history reconstruction for tool messages**

In `handleChatSend`, update the message-to-API conversion loop (lines 160-171) to handle `tool_use` and `tool_result` roles:

```go
for _, m := range messages {
	switch m.Role {
	case "user":
		apiMessages = append(apiMessages, stream.Message{
			Role:    "user",
			Content: []stream.ContentBlock{{Type: "text", Text: m.Content}},
		})
	case "assistant":
		apiMessages = append(apiMessages, stream.Message{
			Role:    "assistant",
			Content: []stream.ContentBlock{{Type: "text", Text: m.Content}},
		})
	case "tool_use":
		// Reconstruct assistant message with tool_use blocks.
		var blocks []stream.ContentBlock
		if err := json.Unmarshal([]byte(m.Content), &blocks); err == nil {
			apiMessages = append(apiMessages, stream.Message{Role: "assistant", Content: blocks})
		}
	case "tool_result":
		// Reconstruct user message with tool_result block.
		var result struct {
			ToolUseID string `json:"tool_use_id"`
			Content   string `json:"content"`
		}
		if err := json.Unmarshal([]byte(m.Content), &result); err == nil {
			apiMessages = append(apiMessages, stream.Message{
				Role: "user",
				Content: []stream.ContentBlock{{
					Type:      "tool_result",
					ToolUseID: result.ToolUseID,
					Content:   result.Content,
				}},
			})
		}
	}
}
```

- [ ] **Step 6: Verify compilation**

Run: `go build ./internal/chat/...`
Expected: Clean build

- [ ] **Step 7: Run existing tests**

Run: `go test ./internal/chat/... -v`
Expected: All existing tests still pass

- [ ] **Step 8: Commit**

```bash
git add internal/chat/ws/ internal/chat/context/
git commit -m "feat: integrate product context injection and tool call loop into chat handler"
```

---

## Chunk 3: Frontend

### Task 7: Frontend Types and Hook Changes

Update TypeScript types and useChat hook for product state.

**Files:**
- Modify: `specs/session.yaml` (if exists, otherwise `web/src/lib/types.ts`)
- Modify: `web/src/hooks/useChat.ts`

- [ ] **Step 1: Add product field to Session type**

In `web/src/lib/types.ts`, add `product` to the Session interface:

```typescript
// In the Session interface, add:
product: string;
```

Add product type:

```typescript
export type ChatProduct = '' | 'tasks' | 'tutor' | 'projects' | 'observe';
```

Note: If `types.ts` is auto-generated, add these to the YAML spec first and run `make types`. If manual additions are needed for types not covered by specs, add them at the end of the file.

- [ ] **Step 2: Add product state to useChat**

In `useChat.ts`, add state for tracking active product:

```typescript
// After existing state declarations:
const [activeProduct, setActiveProduct] = useState<ChatProduct>('');
```

- [ ] **Step 3: Add session.productSet handler**

In the `handleMessage` callback, add a new case:

```typescript
case 'session.productSet': {
	const { product } = parsed.data as { product: string };
	setActiveProduct(product as ChatProduct);
	break;
}
```

- [ ] **Step 4: Restore product on session.switch and session.history**

In the `session.history` handler, after restoring messages, also restore product:

```typescript
// After setting messages from history:
if (parsed.data?.session?.product !== undefined) {
	setActiveProduct(parsed.data.session.product as ChatProduct);
}
```

- [ ] **Step 5: Add setProduct send function**

```typescript
const setProduct = useCallback((product: ChatProduct) => {
	if (!sessionIDRef.current) return;
	send('session.setProduct', {
		sessionId: sessionIDRef.current,
		product,
	});
	setActiveProduct(product); // Optimistic update.
}, [send]);
```

- [ ] **Step 6: Export new state and function**

Add to the hook's return value:

```typescript
return {
	// ...existing...
	activeProduct,
	setProduct,
};
```

- [ ] **Step 7: Verify TypeScript compilation**

Run: `cd web && npx tsc --noEmit`
Expected: No errors

- [ ] **Step 8: Commit**

```bash
git add web/src/lib/types.ts web/src/hooks/useChat.ts
git commit -m "feat: add product state tracking to useChat hook"
```

---

### Task 8: Chat Input — Tool Button

Add the product selector button to the chat input toolbar.

**Files:**
- Modify: `web/src/components/ChatInput.tsx`

- [ ] **Step 1: Add product definitions**

At the top of ChatInput.tsx, add product options:

```typescript
const PRODUCTS: { id: ChatProduct; name: string; icon: string }[] = [
	{ id: 'tasks', name: 'Tasks', icon: '\u2611' },    // ☑
	{ id: 'tutor', name: 'Tutor', icon: '\u{1F393}' }, // 🎓
	{ id: 'projects', name: 'Projects', icon: '\u{1F4CB}' }, // 📋
	{ id: 'observe', name: 'Observe', icon: '\u{1F441}' },   // 👁
];
```

- [ ] **Step 2: Add product props to ChatInput**

Add to the component props:

```typescript
activeProduct?: ChatProduct;
onSetProduct?: (product: ChatProduct) => void;
```

- [ ] **Step 3: Add product popover state**

```typescript
const [showProductMenu, setShowProductMenu] = useState(false);
```

- [ ] **Step 4: Add tool button to toolbar**

In the toolbar area (before the model selector, around line 376), add:

```tsx
{/* Product selector */}
<div className="relative">
	<button
		type="button"
		onClick={() => setShowProductMenu(!showProductMenu)}
		className={`p-1.5 rounded transition-colors ${
			activeProduct
				? 'text-blue-400 bg-blue-400/10'
				: 'text-fg-muted hover:text-fg'
		}`}
		title="Select product tool"
		data-testid="product-selector-button"
	>
		<svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
			<path strokeLinecap="round" strokeLinejoin="round" d="M11.42 15.17L17.25 21A2.652 2.652 0 0021 17.25l-5.877-5.877M11.42 15.17l2.496-3.03c.317-.384.74-.626 1.208-.766M11.42 15.17l-4.655 5.653a2.548 2.548 0 11-3.586-3.586l6.837-5.63m5.108-.233c.55-.164 1.163-.188 1.743-.14a4.5 4.5 0 004.486-6.336l-3.276 3.277a3.004 3.004 0 01-2.25-2.25l3.276-3.276a4.5 4.5 0 00-6.336 4.486c.091 1.076-.071 2.264-.904 2.95l-.102.085" />
		</svg>
	</button>
	{showProductMenu && (
		<div className="absolute bottom-full left-0 mb-1 bg-elevated border border-white/10 rounded-lg shadow-lg py-1 min-w-[140px] z-50">
			{activeProduct && (
				<button
					className="w-full px-3 py-1.5 text-left text-sm text-fg-muted hover:bg-white/5 transition-colors"
					onClick={() => { onSetProduct?.(''); setShowProductMenu(false); }}
					data-testid="product-option-none"
				>
					None (general)
				</button>
			)}
			{PRODUCTS.map(p => (
				<button
					key={p.id}
					className={`w-full px-3 py-1.5 text-left text-sm transition-colors ${
						activeProduct === p.id
							? 'text-blue-400 bg-blue-400/10'
							: 'text-fg hover:bg-white/5'
					}`}
					onClick={() => { onSetProduct?.(p.id); setShowProductMenu(false); }}
					data-testid={`product-option-${p.id}`}
				>
					{p.icon} {p.name}
				</button>
			))}
		</div>
	)}
</div>
{activeProduct && (
	<span className="px-2 py-0.5 text-xs rounded-full bg-blue-400/10 text-blue-400" data-testid="product-badge">
		{PRODUCTS.find(p => p.id === activeProduct)?.name}
	</span>
)}
```

- [ ] **Step 5: Close popover on outside click**

Add effect to close on outside click:

```typescript
useEffect(() => {
	if (!showProductMenu) return;
	const close = () => setShowProductMenu(false);
	document.addEventListener('click', close);
	return () => document.removeEventListener('click', close);
}, [showProductMenu]);
```

- [ ] **Step 6: Wire props from ChatPage**

In `ChatPage.tsx` (or wherever ChatInput is rendered), pass the new props:

```tsx
<ChatInput
	// ...existing props...
	activeProduct={activeProduct}
	onSetProduct={setProduct}
/>
```

- [ ] **Step 7: Verify TypeScript compilation and build**

Run: `cd web && npx tsc --noEmit && npx vite build`
Expected: Clean compilation, no warnings

- [ ] **Step 8: Commit**

```bash
git add web/src/components/ChatInput.tsx web/src/pages/ChatPage.tsx
git commit -m "feat: add product selector tool button to chat input"
```

---

## Chunk 4: Deploy and Verify

### Task 9: Build, Deploy, CLAUDE.md, Verify

Build all binaries, verify the full stack, update docs.

**Files:**
- Modify: `CLAUDE.md`
- Build: all binaries

- [ ] **Step 1: Run full static verification**

```bash
make verify-static
```
Expected: All checks pass (go vet, tsc, secret scan, dep audit)

- [ ] **Step 2: Run unit tests**

```bash
go test ./internal/chat/... -v
```
Expected: All tests pass, including new context package tests

- [ ] **Step 3: Build all binaries**

```bash
make build
```
Expected: soul-chat, soul-tasks, soul-tutor, soul-projects, soul-observe all build

- [ ] **Step 4: Build frontend**

```bash
cd web && npx vite build
```
Expected: Clean build, bundle < 300KB gzipped

- [ ] **Step 5: Start all servers and test tool routing**

```bash
# Start all servers
make serve

# Test: set product on a session
# (via WebSocket — manual test or curl the health endpoints)
curl -s http://127.0.0.1:3002/api/observe/health
```
Expected: Health check returns `{"status":"ok"}`

- [ ] **Step 6: Update CLAUDE.md**

Add to Architecture section:

```
internal/chat/context/            Product context provider — system prompts + tool definitions + dispatcher
```

Add to hooks list:

```
useChat now supports: activeProduct, setProduct (per-session product tool binding)
```

- [ ] **Step 7: Commit CLAUDE.md**

```bash
git add CLAUDE.md
git commit -m "docs: update CLAUDE.md with chat product routing architecture"
```

- [ ] **Step 8: Push to Gitea**

```bash
git push origin master
```

---

## Key Implementation Notes

### Message Storage Format

- `tool_use` messages store JSON array of ContentBlock structs: `[{"type":"tool_use","id":"...","name":"...","input":{...}}]`
- `tool_result` messages store JSON object: `{"tool_use_id":"...","content":"..."}`
- History reconstruction groups consecutive tool_use rows into one assistant message, consecutive tool_result rows into one user message

### Tool Loop Safety

- Max 5 tool rounds per message (configurable via `maxToolRounds`)
- 10s timeout per tool dispatch call
- 50KB max tool result size (truncated)
- Errors returned as tool results, not panics

### Product Server URLs

Reuses existing env vars: `SOUL_TASKS_URL`, `SOUL_TUTOR_URL`, `SOUL_PROJECTS_URL`, `SOUL_OBSERVE_URL` with same defaults as the reverse proxies.

### Observe Tool Routing

Observe tools route directly to the observe server (`:3010`), not through the chat server proxy. Paths use `/api/*` directly (e.g., `/api/overview` not `/api/observe/overview`).
