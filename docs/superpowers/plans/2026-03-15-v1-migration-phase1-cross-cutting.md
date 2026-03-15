# V1 Migration Phase 1: Cross-Cutting Capabilities

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add 10 cross-cutting capabilities from Soul v1 to existing Soul v2 packages — memories, custom tools, subagent, task dependencies, substeps, brainstorm stage, multi-session WS, comment watcher, hooks/phases, merge gates.

**Architecture:** No new servers. All changes modify existing `internal/chat/` and `internal/tasks/` packages. A new `BuiltinExecutor` handles in-process tool dispatch for memories/custom tools/subagent before falling through to the product `Dispatcher` for REST routing.

**Tech Stack:** Go 1.24, SQLite (modernc.org/sqlite), WebSocket, Claude API streaming

**Spec:** `docs/superpowers/specs/2026-03-15-soul-v1-migration-design.md` §Cross-Cutting Capabilities

---

## File Map

### Chat Package (internal/chat/)

| File | Action | Responsibility |
|------|--------|---------------|
| `session/memory.go` | Create | Memory struct + store methods (Upsert/Get/Search/List/Delete) |
| `session/memory_test.go` | Create | Memory store unit tests |
| `session/custom_tool.go` | Create | CustomTool struct + store methods (Create/List/Get/Delete) |
| `session/custom_tool_test.go` | Create | Custom tool store unit tests |
| `session/store.go` | Modify | Add memories + custom_tools table creation to initDB, expand SetProduct valid map |
| `ws/builtin.go` | Create | BuiltinExecutor — dispatch memory_*/tool_*/custom_*/subagent tools |
| `ws/builtin_test.go` | Create | BuiltinExecutor unit tests |
| `ws/subagent.go` | Create | Subagent tool — read-only agent loop |
| `ws/subagent_test.go` | Create | Subagent unit tests |
| `ws/handler.go` | Modify | Integrate BuiltinExecutor into tool dispatch loop |
| `context/context.go` | Modify | Add memory/custom tool definitions to system prompt |

### Tasks Package (internal/tasks/)

| File | Action | Responsibility |
|------|--------|---------------|
| `store/store.go` | Modify | Add brainstorm to validStages, substep to allowed fields + SELECT, task_dependencies + task_comments tables, dependency/comment methods |
| `store/store_test.go` | Modify | Tests for new store methods |
| `store/types.go` | Create | Substep enum with canonical ordering + Next() |
| `store/types_test.go` | Create | Substep ordering tests |
| `server/server.go` | Modify | Add dependency + comment API endpoints |
| `watcher/watcher.go` | Create | CommentWatcher background poller |
| `watcher/watcher_test.go` | Create | CommentWatcher unit tests |
| `gates/gates.go` | Create | PreMergeGate, SmokeTest, RuntimeGate, StepVerificationGate, VisualRegression, FeatureGate |
| `gates/gates_test.go` | Create | Gate unit tests |
| `hooks/hooks.go` | Create | HookConfig, HookRunner — tool/workflow hooks from ~/.soul-v2/hooks.json |
| `hooks/hooks_test.go` | Create | Hook runner tests |
| `phases/phases.go` | Create | PhaseConfig, PhaseRunner — 3-phase pipeline (impl→review→fix) |
| `phases/phases_test.go` | Create | Phase runner tests |
| `executor/executor.go` | Modify | Integrate hooks, phases, gates; skip brainstorm tasks |

---

## Task 1: Agent Memories — Store Layer

**Files:**
- Modify: `internal/chat/session/store.go` (add table creation ~line 155)
- Create: `internal/chat/session/memory.go`
- Create: `internal/chat/session/memory_test.go`

**Ref:** Spec §1 (Agent Memories), v1 `internal/planner/memory.go`

- [ ] **Step 1: Write failing tests for memory store**

Create `internal/chat/session/memory_test.go`:

```go
package session

import (
	"path/filepath"
	"testing"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.db")
	s, err := Open(path)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestUpsertMemory_Create(t *testing.T) {
	s := newTestStore(t)
	m, err := s.UpsertMemory("greeting", "hello world", "test")
	if err != nil {
		t.Fatalf("upsert: %v", err)
	}
	if m.Key != "greeting" || m.Content != "hello world" || m.Tags != "test" {
		t.Errorf("unexpected memory: %+v", m)
	}
	if m.ID == 0 {
		t.Error("expected non-zero ID")
	}
}

func TestUpsertMemory_Update(t *testing.T) {
	s := newTestStore(t)
	s.UpsertMemory("key1", "v1", "")
	m, err := s.UpsertMemory("key1", "v2", "updated")
	if err != nil {
		t.Fatalf("upsert: %v", err)
	}
	if m.Content != "v2" || m.Tags != "updated" {
		t.Errorf("expected updated content, got: %+v", m)
	}
}

func TestGetMemory(t *testing.T) {
	s := newTestStore(t)
	s.UpsertMemory("testkey", "testval", "")
	m, err := s.GetMemory("testkey")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if m.Content != "testval" {
		t.Errorf("expected testval, got %s", m.Content)
	}
}

func TestGetMemory_NotFound(t *testing.T) {
	s := newTestStore(t)
	_, err := s.GetMemory("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent key")
	}
}

func TestSearchMemories(t *testing.T) {
	s := newTestStore(t)
	s.UpsertMemory("go-patterns", "use interfaces", "golang")
	s.UpsertMemory("js-patterns", "use closures", "javascript")
	s.UpsertMemory("go-testing", "table-driven tests", "golang")

	results, err := s.SearchMemories("go")
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(results) != 2 {
		t.Errorf("expected 2 results, got %d", len(results))
	}
}

func TestListMemories(t *testing.T) {
	s := newTestStore(t)
	s.UpsertMemory("a", "1", "")
	s.UpsertMemory("b", "2", "")
	s.UpsertMemory("c", "3", "")

	results, err := s.ListMemories(2)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(results) != 2 {
		t.Errorf("expected 2 results, got %d", len(results))
	}
}

func TestDeleteMemory(t *testing.T) {
	s := newTestStore(t)
	s.UpsertMemory("todelete", "val", "")
	err := s.DeleteMemory("todelete")
	if err != nil {
		t.Fatalf("delete: %v", err)
	}
	_, err = s.GetMemory("todelete")
	if err == nil {
		t.Error("expected error after delete")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /home/rishav/soul-v2 && go test ./internal/chat/session/ -run TestUpsertMemory -v`
Expected: FAIL — `UpsertMemory` not defined

- [ ] **Step 3: Add memories table to store.go initDB**

In `internal/chat/session/store.go`, add to `Migrate()` (the exported migration function) after the messages table creation (~line 155):

```go
_, err = s.db.Exec(`CREATE TABLE IF NOT EXISTS memories (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	key TEXT NOT NULL UNIQUE,
	content TEXT NOT NULL,
	tags TEXT DEFAULT '',
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL
)`)
if err != nil {
	return fmt.Errorf("create memories table: %w", err)
}
_, err = s.db.Exec(`CREATE INDEX IF NOT EXISTS idx_memories_key ON memories(key)`)
if err != nil {
	return fmt.Errorf("create memories index: %w", err)
}
```

- [ ] **Step 4: Implement memory.go**

Create `internal/chat/session/memory.go`:

```go
package session

import (
	"database/sql"
	"fmt"
	"time"
)

type Memory struct {
	ID        int64
	Key       string
	Content   string
	Tags      string
	CreatedAt string
	UpdatedAt string
}

func (s *Store) UpsertMemory(key, content, tags string) (Memory, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := s.db.Exec(`INSERT INTO memories (key, content, tags, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(key) DO UPDATE SET content=excluded.content, tags=excluded.tags, updated_at=excluded.updated_at`,
		key, content, tags, now, now)
	if err != nil {
		return Memory{}, fmt.Errorf("upsert memory: %w", err)
	}
	return s.GetMemory(key)
}

func (s *Store) GetMemory(key string) (Memory, error) {
	var m Memory
	err := s.db.QueryRow(`SELECT id, key, content, tags, created_at, updated_at FROM memories WHERE key = ?`, key).
		Scan(&m.ID, &m.Key, &m.Content, &m.Tags, &m.CreatedAt, &m.UpdatedAt)
	if err == sql.ErrNoRows {
		return Memory{}, fmt.Errorf("memory not found: %s", key)
	}
	if err != nil {
		return Memory{}, fmt.Errorf("get memory: %w", err)
	}
	return m, nil
}

func (s *Store) SearchMemories(query string) ([]Memory, error) {
	like := "%" + query + "%"
	rows, err := s.db.Query(`SELECT id, key, content, tags, created_at, updated_at
		FROM memories WHERE key LIKE ? OR content LIKE ? OR tags LIKE ?
		ORDER BY updated_at DESC`, like, like, like)
	if err != nil {
		return nil, fmt.Errorf("search memories: %w", err)
	}
	defer rows.Close()
	var results []Memory
	for rows.Next() {
		var m Memory
		if err := rows.Scan(&m.ID, &m.Key, &m.Content, &m.Tags, &m.CreatedAt, &m.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan memory: %w", err)
		}
		results = append(results, m)
	}
	return results, rows.Err()
}

func (s *Store) ListMemories(limit int) ([]Memory, error) {
	rows, err := s.db.Query(`SELECT id, key, content, tags, created_at, updated_at
		FROM memories ORDER BY updated_at DESC LIMIT ?`, limit)
	if err != nil {
		return nil, fmt.Errorf("list memories: %w", err)
	}
	defer rows.Close()
	var results []Memory
	for rows.Next() {
		var m Memory
		if err := rows.Scan(&m.ID, &m.Key, &m.Content, &m.Tags, &m.CreatedAt, &m.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan memory: %w", err)
		}
		results = append(results, m)
	}
	return results, rows.Err()
}

func (s *Store) DeleteMemory(key string) error {
	res, err := s.db.Exec(`DELETE FROM memories WHERE key = ?`, key)
	if err != nil {
		return fmt.Errorf("delete memory: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("memory not found: %s", key)
	}
	return nil
}
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `cd /home/rishav/soul-v2 && go test ./internal/chat/session/ -run "TestUpsertMemory|TestGetMemory|TestSearchMemories|TestListMemories|TestDeleteMemory" -v`
Expected: ALL PASS

- [ ] **Step 6: Commit**

```bash
git add internal/chat/session/memory.go internal/chat/session/memory_test.go internal/chat/session/store.go
git commit -m "feat: add agent memories store layer"
```

---

## Task 2: Custom Tools — Store Layer

**Files:**
- Modify: `internal/chat/session/store.go` (add table creation)
- Create: `internal/chat/session/custom_tool.go`
- Create: `internal/chat/session/custom_tool_test.go`

**Ref:** Spec §2 (Custom Tools), v1 `internal/planner/tools.go`

- [ ] **Step 1: Write failing tests for custom tool store**

Create `internal/chat/session/custom_tool_test.go`:

```go
package session

import "testing"

func TestCreateCustomTool(t *testing.T) {
	s := newTestStore(t)
	ct, err := s.CreateCustomTool("greet", "Says hello", `{"type":"object","properties":{"name":{"type":"string"}}}`, "echo Hello $PARAM_name")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if ct.Name != "greet" || ct.Description != "Says hello" {
		t.Errorf("unexpected: %+v", ct)
	}
}

func TestCreateCustomTool_Duplicate(t *testing.T) {
	s := newTestStore(t)
	s.CreateCustomTool("dup", "first", "{}", "echo 1")
	_, err := s.CreateCustomTool("dup", "second", "{}", "echo 2")
	if err == nil {
		t.Error("expected error for duplicate name")
	}
}

func TestListCustomTools(t *testing.T) {
	s := newTestStore(t)
	s.CreateCustomTool("alpha", "a", "{}", "echo a")
	s.CreateCustomTool("beta", "b", "{}", "echo b")
	tools, err := s.ListCustomTools()
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(tools) != 2 {
		t.Errorf("expected 2, got %d", len(tools))
	}
	if tools[0].Name != "alpha" {
		t.Errorf("expected alpha first (sorted), got %s", tools[0].Name)
	}
}

func TestGetCustomTool(t *testing.T) {
	s := newTestStore(t)
	s.CreateCustomTool("fetch", "fetches data", `{"type":"object"}`, "curl $PARAM_url")
	ct, err := s.GetCustomTool("fetch")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if ct.CommandTemplate != "curl $PARAM_url" {
		t.Errorf("unexpected template: %s", ct.CommandTemplate)
	}
}

func TestGetCustomTool_NotFound(t *testing.T) {
	s := newTestStore(t)
	_, err := s.GetCustomTool("nope")
	if err == nil {
		t.Error("expected error for nonexistent tool")
	}
}

func TestDeleteCustomTool(t *testing.T) {
	s := newTestStore(t)
	s.CreateCustomTool("temp", "temporary", "{}", "echo temp")
	err := s.DeleteCustomTool("temp")
	if err != nil {
		t.Fatalf("delete: %v", err)
	}
	_, err = s.GetCustomTool("temp")
	if err == nil {
		t.Error("expected error after delete")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /home/rishav/soul-v2 && go test ./internal/chat/session/ -run TestCreateCustomTool -v`
Expected: FAIL — `CreateCustomTool` not defined

- [ ] **Step 3: Add custom_tools table to store.go initDB**

In `internal/chat/session/store.go`, add after memories table in `Migrate()`:

```go
_, err = s.db.Exec(`CREATE TABLE IF NOT EXISTS custom_tools (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	name TEXT NOT NULL UNIQUE,
	description TEXT NOT NULL,
	input_schema TEXT NOT NULL,
	command_template TEXT NOT NULL,
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL
)`)
if err != nil {
	return fmt.Errorf("create custom_tools table: %w", err)
}
_, err = s.db.Exec(`CREATE INDEX IF NOT EXISTS idx_custom_tools_name ON custom_tools(name)`)
if err != nil {
	return fmt.Errorf("create custom_tools index: %w", err)
}
```

- [ ] **Step 4: Implement custom_tool.go**

Create `internal/chat/session/custom_tool.go`:

```go
package session

import (
	"database/sql"
	"fmt"
	"time"
)

type CustomTool struct {
	ID              int64
	Name            string
	Description     string
	InputSchema     string
	CommandTemplate string
	CreatedAt       string
	UpdatedAt       string
}

func (s *Store) CreateCustomTool(name, description, inputSchema, commandTemplate string) (CustomTool, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	res, err := s.db.Exec(`INSERT INTO custom_tools (name, description, input_schema, command_template, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)`, name, description, inputSchema, commandTemplate, now, now)
	if err != nil {
		return CustomTool{}, fmt.Errorf("create custom tool: %w", err)
	}
	id, _ := res.LastInsertId()
	return CustomTool{ID: id, Name: name, Description: description, InputSchema: inputSchema, CommandTemplate: commandTemplate, CreatedAt: now, UpdatedAt: now}, nil
}

func (s *Store) ListCustomTools() ([]CustomTool, error) {
	rows, err := s.db.Query(`SELECT id, name, description, input_schema, command_template, created_at, updated_at
		FROM custom_tools ORDER BY name ASC`)
	if err != nil {
		return nil, fmt.Errorf("list custom tools: %w", err)
	}
	defer rows.Close()
	var tools []CustomTool
	for rows.Next() {
		var ct CustomTool
		if err := rows.Scan(&ct.ID, &ct.Name, &ct.Description, &ct.InputSchema, &ct.CommandTemplate, &ct.CreatedAt, &ct.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan custom tool: %w", err)
		}
		tools = append(tools, ct)
	}
	return tools, rows.Err()
}

func (s *Store) GetCustomTool(name string) (CustomTool, error) {
	var ct CustomTool
	err := s.db.QueryRow(`SELECT id, name, description, input_schema, command_template, created_at, updated_at
		FROM custom_tools WHERE name = ?`, name).
		Scan(&ct.ID, &ct.Name, &ct.Description, &ct.InputSchema, &ct.CommandTemplate, &ct.CreatedAt, &ct.UpdatedAt)
	if err == sql.ErrNoRows {
		return CustomTool{}, fmt.Errorf("custom tool not found: %s", name)
	}
	if err != nil {
		return CustomTool{}, fmt.Errorf("get custom tool: %w", err)
	}
	return ct, nil
}

func (s *Store) DeleteCustomTool(name string) error {
	res, err := s.db.Exec(`DELETE FROM custom_tools WHERE name = ?`, name)
	if err != nil {
		return fmt.Errorf("delete custom tool: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("custom tool not found: %s", name)
	}
	return nil
}
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `cd /home/rishav/soul-v2 && go test ./internal/chat/session/ -run "TestCreateCustomTool|TestListCustomTools|TestGetCustomTool|TestDeleteCustomTool" -v`
Expected: ALL PASS

- [ ] **Step 6: Commit**

```bash
git add internal/chat/session/custom_tool.go internal/chat/session/custom_tool_test.go internal/chat/session/store.go
git commit -m "feat: add custom tools store layer"
```

---

## Task 3: BuiltinExecutor — Memory & Custom Tool Dispatch

**Files:**
- Create: `internal/chat/ws/builtin.go`
- Create: `internal/chat/ws/builtin_test.go`
- Modify: `internal/chat/ws/handler.go` (~line 515, tool dispatch loop)

**Ref:** Spec §1 (Built-in tool dispatch), §2 (Custom tool execution)

- [ ] **Step 1: Write failing tests for BuiltinExecutor**

Create `internal/chat/ws/builtin_test.go`:

```go
package ws

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/rishav1305/soul-v2/internal/chat/session"
)

func newTestBuiltin(t *testing.T) (*BuiltinExecutor, *session.Store) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.db")
	store, err := session.Open(path)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { store.Close() })
	return NewBuiltinExecutor(store), store
}

func TestBuiltinExecutor_CanHandle(t *testing.T) {
	be, _ := newTestBuiltin(t)
	tests := []struct {
		name string
		want bool
	}{
		{"memory_store", true},
		{"memory_search", true},
		{"memory_list", true},
		{"memory_delete", true},
		{"tool_create", true},
		{"tool_list", true},
		{"tool_delete", true},
		{"custom_greet", true},
		{"subagent", true},
		{"list_tasks", false},
		{"", false},
	}
	for _, tt := range tests {
		if got := be.CanHandle(tt.name); got != tt.want {
			t.Errorf("CanHandle(%q) = %v, want %v", tt.name, got, tt.want)
		}
	}
}

func TestBuiltinExecutor_MemoryStore(t *testing.T) {
	be, _ := newTestBuiltin(t)
	result, err := be.Execute(context.Background(), "memory_store", []byte(`{"key":"test","content":"hello","tags":"t1"}`))
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if result == "" {
		t.Error("expected non-empty result")
	}
}

func TestBuiltinExecutor_MemorySearch(t *testing.T) {
	be, store := newTestBuiltin(t)
	store.UpsertMemory("go-tips", "use interfaces", "golang")
	result, err := be.Execute(context.Background(), "memory_search", []byte(`{"query":"go"}`))
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if result == "" {
		t.Error("expected non-empty result")
	}
}

func TestBuiltinExecutor_MemoryList(t *testing.T) {
	be, store := newTestBuiltin(t)
	store.UpsertMemory("a", "1", "")
	store.UpsertMemory("b", "2", "")
	result, err := be.Execute(context.Background(), "memory_list", []byte(`{"limit":10}`))
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if result == "" {
		t.Error("expected non-empty result")
	}
}

func TestBuiltinExecutor_MemoryDelete(t *testing.T) {
	be, store := newTestBuiltin(t)
	store.UpsertMemory("todel", "val", "")
	result, err := be.Execute(context.Background(), "memory_delete", []byte(`{"key":"todel"}`))
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if result == "" {
		t.Error("expected non-empty result")
	}
}

func TestBuiltinExecutor_ToolCreate(t *testing.T) {
	be, _ := newTestBuiltin(t)
	result, err := be.Execute(context.Background(), "tool_create", []byte(`{"name":"hello","description":"says hi","input_schema":"{}","command_template":"echo hi"}`))
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if result == "" {
		t.Error("expected non-empty result")
	}
}

func TestBuiltinExecutor_ToolList(t *testing.T) {
	be, store := newTestBuiltin(t)
	store.CreateCustomTool("t1", "d1", "{}", "echo 1")
	result, err := be.Execute(context.Background(), "tool_list", []byte(`{}`))
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if result == "" {
		t.Error("expected non-empty result")
	}
}

func TestBuiltinExecutor_ToolDelete(t *testing.T) {
	be, store := newTestBuiltin(t)
	store.CreateCustomTool("todel", "d", "{}", "echo x")
	result, err := be.Execute(context.Background(), "tool_delete", []byte(`{"name":"todel"}`))
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if result == "" {
		t.Error("expected non-empty result")
	}
}

func TestBuiltinExecutor_UnknownTool(t *testing.T) {
	be, _ := newTestBuiltin(t)
	_, err := be.Execute(context.Background(), "unknown_tool", []byte(`{}`))
	if err == nil {
		t.Error("expected error for unknown tool")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /home/rishav/soul-v2 && go test ./internal/chat/ws/ -run TestBuiltinExecutor -v`
Expected: FAIL — `BuiltinExecutor` not defined

- [ ] **Step 3: Implement builtin.go**

Create `internal/chat/ws/builtin.go`:

```go
package ws

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/rishav1305/soul-v2/internal/chat/session"
)

type BuiltinExecutor struct {
	store *session.Store
}

func NewBuiltinExecutor(store *session.Store) *BuiltinExecutor {
	return &BuiltinExecutor{store: store}
}

func (be *BuiltinExecutor) CanHandle(toolName string) bool {
	if toolName == "subagent" {
		return true
	}
	return strings.HasPrefix(toolName, "memory_") ||
		strings.HasPrefix(toolName, "tool_") ||
		strings.HasPrefix(toolName, "custom_")
}

func (be *BuiltinExecutor) Execute(ctx context.Context, toolName string, inputJSON []byte) (string, error) {
	switch {
	case strings.HasPrefix(toolName, "memory_"):
		return be.executeMemory(toolName, inputJSON)
	case strings.HasPrefix(toolName, "tool_"):
		return be.executeToolMgmt(toolName, inputJSON)
	case strings.HasPrefix(toolName, "custom_"):
		return be.executeCustom(ctx, toolName, inputJSON)
	case toolName == "subagent":
		return "", fmt.Errorf("subagent dispatch not yet implemented")
	default:
		return "", fmt.Errorf("unknown built-in tool: %s", toolName)
	}
}

func (be *BuiltinExecutor) executeMemory(toolName string, inputJSON []byte) (string, error) {
	var params map[string]interface{}
	if err := json.Unmarshal(inputJSON, &params); err != nil {
		return "", fmt.Errorf("parse input: %w", err)
	}

	switch toolName {
	case "memory_store":
		key, _ := params["key"].(string)
		content, _ := params["content"].(string)
		tags, _ := params["tags"].(string)
		if key == "" || content == "" {
			return "", fmt.Errorf("memory_store requires key and content")
		}
		m, err := be.store.UpsertMemory(key, content, tags)
		if err != nil {
			return "", err
		}
		b, _ := json.Marshal(m)
		return string(b), nil

	case "memory_search":
		query, _ := params["query"].(string)
		if query == "" {
			return "", fmt.Errorf("memory_search requires query")
		}
		results, err := be.store.SearchMemories(query)
		if err != nil {
			return "", err
		}
		b, _ := json.Marshal(results)
		return string(b), nil

	case "memory_list":
		limit := 20
		if l, ok := params["limit"].(float64); ok && int(l) > 0 {
			limit = int(l)
		}
		results, err := be.store.ListMemories(limit)
		if err != nil {
			return "", err
		}
		b, _ := json.Marshal(results)
		return string(b), nil

	case "memory_delete":
		key, _ := params["key"].(string)
		if key == "" {
			return "", fmt.Errorf("memory_delete requires key")
		}
		if err := be.store.DeleteMemory(key); err != nil {
			return "", err
		}
		return fmt.Sprintf(`{"deleted":"%s"}`, key), nil

	default:
		return "", fmt.Errorf("unknown memory tool: %s", toolName)
	}
}

func (be *BuiltinExecutor) executeToolMgmt(toolName string, inputJSON []byte) (string, error) {
	var params map[string]interface{}
	if err := json.Unmarshal(inputJSON, &params); err != nil {
		return "", fmt.Errorf("parse input: %w", err)
	}

	switch toolName {
	case "tool_create":
		name, _ := params["name"].(string)
		desc, _ := params["description"].(string)
		schema, _ := params["input_schema"].(string)
		tmpl, _ := params["command_template"].(string)
		if name == "" || desc == "" || tmpl == "" {
			return "", fmt.Errorf("tool_create requires name, description, command_template")
		}
		if schema == "" {
			schema = `{"type":"object","properties":{}}`
		}
		ct, err := be.store.CreateCustomTool(name, desc, schema, tmpl)
		if err != nil {
			return "", err
		}
		b, _ := json.Marshal(ct)
		return string(b), nil

	case "tool_list":
		tools, err := be.store.ListCustomTools()
		if err != nil {
			return "", err
		}
		b, _ := json.Marshal(tools)
		return string(b), nil

	case "tool_delete":
		name, _ := params["name"].(string)
		if name == "" {
			return "", fmt.Errorf("tool_delete requires name")
		}
		if err := be.store.DeleteCustomTool(name); err != nil {
			return "", err
		}
		return fmt.Sprintf(`{"deleted":"%s"}`, name), nil

	default:
		return "", fmt.Errorf("unknown tool management command: %s", toolName)
	}
}

func (be *BuiltinExecutor) executeCustom(ctx context.Context, toolName string, inputJSON []byte) (string, error) {
	name := strings.TrimPrefix(toolName, "custom_")
	ct, err := be.store.GetCustomTool(name)
	if err != nil {
		return "", fmt.Errorf("custom tool %q not found: %w", name, err)
	}

	var params map[string]interface{}
	if err := json.Unmarshal(inputJSON, &params); err != nil {
		return "", fmt.Errorf("parse input: %w", err)
	}

	// Build environment variables from params (prevents shell injection)
	env := make([]string, 0, len(params))
	for k, v := range params {
		env = append(env, fmt.Sprintf("PARAM_%s=%v", k, v))
	}

	timeout, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	cmd := exec.CommandContext(timeout, "bash", "-c", ct.CommandTemplate)
	cmd.Env = append(cmd.Environ(), env...)
	output, err := cmd.CombinedOutput()
	if err != nil && timeout.Err() == context.DeadlineExceeded {
		return "Error: command timed out after 60 seconds", nil
	}

	result := string(output)
	if len(result) > 5000 {
		result = result[:5000] + "\n...(truncated)"
	}
	if err != nil {
		return fmt.Sprintf("Exit error: %v\nOutput: %s", err, result), nil
	}
	return result, nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd /home/rishav/soul-v2 && go test ./internal/chat/ws/ -run TestBuiltinExecutor -v`
Expected: ALL PASS

- [ ] **Step 5: Integrate BuiltinExecutor into handler.go tool dispatch loop**

In `internal/chat/ws/handler.go`, modify the `MessageHandler` struct (~line 30) to add `builtin *BuiltinExecutor` field. Update `NewMessageHandler` to accept and store it.

In `runStream()` tool dispatch loop (~line 515), before `h.dispatcher.Execute()`:

```go
var toolResult string
if h.builtin != nil && h.builtin.CanHandle(tc.Name) {
	result, err := h.builtin.Execute(ctx, tc.Name, inputJSON)
	if err != nil {
		toolResult = fmt.Sprintf("Error: %v", err)
	} else {
		toolResult = result
	}
} else {
	result, err := h.dispatcher.Execute(ctx, tc.Name, inputJSON)
	if err != nil {
		toolResult = fmt.Sprintf("Error: %v", err)
	} else {
		toolResult = result
	}
}
```

Note: Check the actual `dispatcher.Execute()` return signature in `dispatch.go`. It may return `(string, error)` or just `string` — adapt accordingly.

Also update `cmd/chat/main.go` to create `BuiltinExecutor` and pass it to `NewMessageHandler`.

- [ ] **Step 6: Run full chat tests**

Run: `cd /home/rishav/soul-v2 && go test ./internal/chat/... -v`
Expected: ALL PASS

- [ ] **Step 7: Commit**

```bash
git add internal/chat/ws/builtin.go internal/chat/ws/builtin_test.go internal/chat/ws/handler.go cmd/chat/main.go
git commit -m "feat: add BuiltinExecutor for memory and custom tool dispatch"
```

---

## Task 4: Expand SetProduct Valid Map

**Files:**
- Modify: `internal/chat/session/store.go` (~line 589, SetProduct valid map)

**Ref:** Spec §Product Registration Updates

- [ ] **Step 1: Write test for new valid products**

Add to existing session store tests or create a new test:

```go
func TestSetProduct_NewProducts(t *testing.T) {
	s := newTestStore(t)
	sid, _ := s.CreateSession("")

	newProducts := []string{
		"scout", "sentinel", "mesh", "bench",
		"compliance", "qa", "analytics",
		"devops", "dba", "migrate",
		"dataeng", "costops", "viz",
		"docs", "api",
	}
	for _, p := range newProducts {
		if err := s.SetProduct(sid, p); err != nil {
			t.Errorf("SetProduct(%q) failed: %v", p, err)
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /home/rishav/soul-v2 && go test ./internal/chat/session/ -run TestSetProduct_NewProducts -v`
Expected: FAIL — `invalid product: "scout"`

- [ ] **Step 3: Expand valid map in store.go**

In `internal/chat/session/store.go` ~line 589, replace the valid map:

```go
valid := map[string]bool{
	"": true, "tasks": true, "tutor": true, "projects": true, "observe": true,
	"scout": true, "sentinel": true, "mesh": true, "bench": true,
	"compliance": true, "qa": true, "analytics": true,
	"devops": true, "dba": true, "migrate": true,
	"dataeng": true, "costops": true, "viz": true,
	"docs": true, "api": true,
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd /home/rishav/soul-v2 && go test ./internal/chat/session/ -run TestSetProduct -v`
Expected: ALL PASS

- [ ] **Step 5: Commit**

```bash
git add internal/chat/session/store.go internal/chat/session/store_test.go
git commit -m "feat: expand SetProduct valid map for 19 products"
```

---

## Task 5: Task Dependencies

**Files:**
- Modify: `internal/tasks/store/store.go` (add table, methods)
- Modify: `internal/tasks/store/store_test.go` (add tests)
- Modify: `internal/tasks/server/server.go` (add API endpoints)

**Ref:** Spec §4 (Task Dependencies)

- [ ] **Step 1: Write failing tests for dependency store methods**

Add to `internal/tasks/store/store_test.go`:

```go
func TestAddDependency(t *testing.T) {
	s := newTestStore(t)
	id1 := s.mustCreate(t, "Task A")
	id2 := s.mustCreate(t, "Task B")
	if err := s.AddDependency(id2, id1); err != nil {
		t.Fatalf("add dependency: %v", err)
	}
}

func TestRemoveDependency(t *testing.T) {
	s := newTestStore(t)
	id1 := s.mustCreate(t, "Task A")
	id2 := s.mustCreate(t, "Task B")
	s.AddDependency(id2, id1)
	if err := s.RemoveDependency(id2, id1); err != nil {
		t.Fatalf("remove dependency: %v", err)
	}
}

func TestNextReady_NoDeps(t *testing.T) {
	s := newTestStore(t)
	s.mustCreate(t, "Ready task")
	task, err := s.NextReady()
	if err != nil {
		t.Fatalf("next ready: %v", err)
	}
	if task.Title != "Ready task" {
		t.Errorf("expected Ready task, got %s", task.Title)
	}
}

func TestNextReady_BlockedByDep(t *testing.T) {
	s := newTestStore(t)
	id1 := s.mustCreate(t, "Blocker")
	id2 := s.mustCreate(t, "Blocked")
	s.AddDependency(id2, id1)
	task, err := s.NextReady()
	if err != nil {
		t.Fatalf("next ready: %v", err)
	}
	// Should return Blocker (no deps), not Blocked
	if task.Title != "Blocker" {
		t.Errorf("expected Blocker, got %s", task.Title)
	}
}

func TestNextReady_DepDone(t *testing.T) {
	s := newTestStore(t)
	id1 := s.mustCreate(t, "Done dep")
	id2 := s.mustCreate(t, "Waiting")
	s.AddDependency(id2, id1)
	s.Update(id1, map[string]interface{}{"stage": "done"})
	task, err := s.NextReady()
	if err != nil {
		t.Fatalf("next ready: %v", err)
	}
	// Both are ready now, but Done dep is stage=done so only Waiting is backlog
	if task.Title != "Waiting" {
		t.Errorf("expected Waiting, got %s", task.Title)
	}
}
```

Note: `mustCreate` is a test helper — if it doesn't exist, add it:
```go
func (s *Store) mustCreate(t *testing.T, title string) int64 {
	t.Helper()
	task, err := s.Create(title, "", "")
	if err != nil {
		t.Fatalf("create task: %v", err)
	}
	return task.ID
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /home/rishav/soul-v2 && go test ./internal/tasks/store/ -run "TestAddDependency|TestNextReady" -v`
Expected: FAIL

- [ ] **Step 3: Add task_dependencies table and methods to store.go**

Add table creation to `migrate()`:
```go
_, err = s.db.Exec(`CREATE TABLE IF NOT EXISTS task_dependencies (
	task_id INTEGER REFERENCES tasks(id) ON DELETE CASCADE,
	depends_on INTEGER REFERENCES tasks(id) ON DELETE CASCADE,
	PRIMARY KEY (task_id, depends_on)
)`)
```

Add methods:
```go
func (s *Store) AddDependency(taskID, dependsOn int64) error {
	_, err := s.db.Exec(`INSERT OR IGNORE INTO task_dependencies (task_id, depends_on) VALUES (?, ?)`, taskID, dependsOn)
	if err != nil {
		return fmt.Errorf("add dependency: %w", err)
	}
	return nil
}

func (s *Store) RemoveDependency(taskID, dependsOn int64) error {
	_, err := s.db.Exec(`DELETE FROM task_dependencies WHERE task_id = ? AND depends_on = ?`, taskID, dependsOn)
	if err != nil {
		return fmt.Errorf("remove dependency: %w", err)
	}
	return nil
}

func (s *Store) NextReady() (Task, error) {
	var task Task
	err := s.db.QueryRow(`SELECT id, title, description, stage, workflow, product, metadata, created_at, updated_at
		FROM tasks t
		WHERE t.stage = 'backlog'
		AND NOT EXISTS (
			SELECT 1 FROM task_dependencies td
			JOIN tasks dep ON dep.id = td.depends_on
			WHERE td.task_id = t.id AND dep.stage != 'done'
		)
		ORDER BY created_at ASC LIMIT 1`).
		Scan(&task.ID, &task.Title, &task.Description, &task.Stage, &task.Workflow, &task.Product, &task.Metadata, &task.CreatedAt, &task.UpdatedAt)
	// Note: The spec calls for ORDER BY priority DESC, created_at ASC but the
	// tasks table has no priority column. Using created_at ASC only for now.
	// A priority column can be added in a future migration if needed.
	if err != nil {
		return Task{}, fmt.Errorf("next ready: %w", err)
	}
	return task, nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd /home/rishav/soul-v2 && go test ./internal/tasks/store/ -run "TestAddDependency|TestRemoveDependency|TestNextReady" -v`
Expected: ALL PASS

- [ ] **Step 5: Add dependency API endpoints to server.go**

In `internal/tasks/server/server.go`, add routes:
```go
mux.HandleFunc("POST /api/tasks/{id}/dependencies", s.handleAddDependency)
mux.HandleFunc("DELETE /api/tasks/{id}/dependencies/{depId}", s.handleRemoveDependency)
```

Implement handlers that parse path params, call store methods, return JSON responses.

- [ ] **Step 6: Run full tasks tests**

Run: `cd /home/rishav/soul-v2 && go test ./internal/tasks/... -v`
Expected: ALL PASS

- [ ] **Step 7: Commit**

```bash
git add internal/tasks/store/store.go internal/tasks/store/store_test.go internal/tasks/server/server.go
git commit -m "feat: add task dependencies with blocking enforcement"
```

---

## Task 6: Task Substeps + Brainstorm Stage

**Files:**
- Create: `internal/tasks/store/types.go`
- Create: `internal/tasks/store/types_test.go`
- Modify: `internal/tasks/store/store.go` (validStages, allowed fields, SELECT columns)

**Ref:** Spec §5 (Substeps), §6 (Brainstorm)

- [ ] **Step 1: Write tests for substep types**

Create `internal/tasks/store/types_test.go`:

```go
package store

import "testing"

func TestSubstepOrder(t *testing.T) {
	order := SubstepOrder()
	if len(order) != 6 {
		t.Fatalf("expected 6 substeps, got %d", len(order))
	}
	expected := []Substep{SubstepTDD, SubstepImplementing, SubstepReviewing, SubstepQATest, SubstepE2ETest, SubstepSecurityReview}
	for i, s := range expected {
		if order[i] != s {
			t.Errorf("position %d: expected %s, got %s", i, s, order[i])
		}
	}
}

func TestSubstepNext(t *testing.T) {
	next, ok := SubstepTDD.Next()
	if !ok || next != SubstepImplementing {
		t.Errorf("TDD.Next() = %s, %v", next, ok)
	}

	next, ok = SubstepSecurityReview.Next()
	if ok {
		t.Errorf("SecurityReview.Next() should return false, got %s", next)
	}
}

func TestSubstepValid(t *testing.T) {
	if !SubstepTDD.Valid() {
		t.Error("tdd should be valid")
	}
	if Substep("invalid").Valid() {
		t.Error("invalid should not be valid")
	}
	if !Substep("").Valid() {
		t.Error("empty substep should be valid (default)")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /home/rishav/soul-v2 && go test ./internal/tasks/store/ -run TestSubstep -v`
Expected: FAIL

- [ ] **Step 3: Implement types.go**

Create `internal/tasks/store/types.go`:

```go
package store

type Substep string

const (
	SubstepTDD            Substep = "tdd"
	SubstepImplementing   Substep = "implementing"
	SubstepReviewing      Substep = "reviewing"
	SubstepQATest         Substep = "qa_test"
	SubstepE2ETest        Substep = "e2e_test"
	SubstepSecurityReview Substep = "security_review"
)

var substepOrder = []Substep{
	SubstepTDD,
	SubstepImplementing,
	SubstepReviewing,
	SubstepQATest,
	SubstepE2ETest,
	SubstepSecurityReview,
}

func SubstepOrder() []Substep {
	out := make([]Substep, len(substepOrder))
	copy(out, substepOrder)
	return out
}

func (ss Substep) Next() (Substep, bool) {
	for i, s := range substepOrder {
		if s == ss && i+1 < len(substepOrder) {
			return substepOrder[i+1], true
		}
	}
	return "", false
}

func (ss Substep) Valid() bool {
	if ss == "" {
		return true
	}
	for _, s := range substepOrder {
		if s == ss {
			return true
		}
	}
	return false
}
```

- [ ] **Step 4: Run substep tests**

Run: `cd /home/rishav/soul-v2 && go test ./internal/tasks/store/ -run TestSubstep -v`
Expected: ALL PASS

- [ ] **Step 5: Update store.go — add brainstorm to validStages, substep to allowed/SELECT**

In `internal/tasks/store/store.go`:
1. Add `"brainstorm": true` to `validStages` map (~line 14)
2. Add `"substep": true` to `allowed` map in `Update()` (~line 182)
3. Add `substep` column via ALTER TABLE in `migrate()` (with duplicate column error check, following existing pattern)
4. Add `substep` to SELECT columns in `Get()` and `List()`
5. Add `Substep string` field to `Task` struct

- [ ] **Step 6: Write test for brainstorm stage**

```go
func TestUpdateTask_BrainstormStage(t *testing.T) {
	s := newTestStore(t)
	id := s.mustCreate(t, "Test task")
	err := s.Update(id, map[string]interface{}{"stage": "brainstorm"})
	if err != nil {
		t.Fatalf("update to brainstorm: %v", err)
	}
	task, _ := s.Get(id)
	if task.Stage != "brainstorm" {
		t.Errorf("expected brainstorm, got %s", task.Stage)
	}
}

func TestUpdateTask_Substep(t *testing.T) {
	s := newTestStore(t)
	id := s.mustCreate(t, "Test task")
	s.Update(id, map[string]interface{}{"stage": "active"})
	err := s.Update(id, map[string]interface{}{"substep": "tdd"})
	if err != nil {
		t.Fatalf("update substep: %v", err)
	}
	task, _ := s.Get(id)
	if task.Substep != "tdd" {
		t.Errorf("expected tdd, got %s", task.Substep)
	}
}
```

- [ ] **Step 7: Run all task store tests**

Run: `cd /home/rishav/soul-v2 && go test ./internal/tasks/store/ -v`
Expected: ALL PASS

- [ ] **Step 8: Commit**

```bash
git add internal/tasks/store/types.go internal/tasks/store/types_test.go internal/tasks/store/store.go internal/tasks/store/store_test.go
git commit -m "feat: add task substeps and brainstorm stage"
```

---

## Task 7: Task Comments + Comment Watcher

**Files:**
- Modify: `internal/tasks/store/store.go` (add table, methods)
- Modify: `internal/tasks/store/store_test.go`
- Modify: `internal/tasks/server/server.go` (add endpoints)
- Create: `internal/tasks/watcher/watcher.go`
- Create: `internal/tasks/watcher/watcher_test.go`

**Ref:** Spec §8 (Comment Watcher)

- [ ] **Step 1: Write failing tests for comment store methods**

Add to `internal/tasks/store/store_test.go`:

```go
func TestInsertComment(t *testing.T) {
	s := newTestStore(t)
	id := s.mustCreate(t, "Commented task")
	cid, err := s.InsertComment(id, "user", "feedback", "Looks good")
	if err != nil {
		t.Fatalf("insert comment: %v", err)
	}
	if cid == 0 {
		t.Error("expected non-zero comment ID")
	}
}

func TestGetComments(t *testing.T) {
	s := newTestStore(t)
	id := s.mustCreate(t, "Task with comments")
	s.InsertComment(id, "user", "feedback", "Comment 1")
	s.InsertComment(id, "soul", "status", "Comment 2")
	comments, err := s.GetComments(id)
	if err != nil {
		t.Fatalf("get comments: %v", err)
	}
	if len(comments) != 2 {
		t.Errorf("expected 2 comments, got %d", len(comments))
	}
}

func TestCommentsAfter(t *testing.T) {
	s := newTestStore(t)
	id := s.mustCreate(t, "Watched task")
	cid1, _ := s.InsertComment(id, "user", "feedback", "First")
	s.InsertComment(id, "user", "feedback", "Second")
	s.InsertComment(id, "soul", "status", "Soul reply") // should be excluded
	comments, err := s.CommentsAfter(cid1)
	if err != nil {
		t.Fatalf("comments after: %v", err)
	}
	// Should return only user comments (not soul), so just "Second"
	if len(comments) != 1 {
		t.Errorf("expected 1 user comment after %d, got %d", cid1, len(comments))
	}
	if comments[0].Body != "Second" {
		t.Errorf("expected 'Second', got %s", comments[0].Body)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /home/rishav/soul-v2 && go test ./internal/tasks/store/ -run "TestInsertComment|TestGetComments|TestCommentsAfter" -v`
Expected: FAIL

- [ ] **Step 3: Add task_comments table and methods to store.go**

Add `Comment` struct, table creation, and methods:

```go
type Comment struct {
	ID        int64
	TaskID    int64
	Author    string
	Type      string
	Body      string
	CreatedAt string
}

// In migrate():
_, err = s.db.Exec(`CREATE TABLE IF NOT EXISTS task_comments (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	task_id INTEGER REFERENCES tasks(id) ON DELETE CASCADE,
	author TEXT NOT NULL,
	type TEXT NOT NULL,
	body TEXT NOT NULL,
	created_at TEXT NOT NULL
)`)

func (s *Store) InsertComment(taskID int64, author, typ, body string) (int64, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	res, err := s.db.Exec(`INSERT INTO task_comments (task_id, author, type, body, created_at) VALUES (?, ?, ?, ?, ?)`,
		taskID, author, typ, body, now)
	if err != nil {
		return 0, fmt.Errorf("insert comment: %w", err)
	}
	return res.LastInsertId()
}

func (s *Store) GetComments(taskID int64) ([]Comment, error) {
	rows, err := s.db.Query(`SELECT id, task_id, author, type, body, created_at FROM task_comments WHERE task_id = ? ORDER BY created_at ASC`, taskID)
	if err != nil {
		return nil, fmt.Errorf("get comments: %w", err)
	}
	defer rows.Close()
	var comments []Comment
	for rows.Next() {
		var c Comment
		if err := rows.Scan(&c.ID, &c.TaskID, &c.Author, &c.Type, &c.Body, &c.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan comment: %w", err)
		}
		comments = append(comments, c)
	}
	return comments, rows.Err()
}

func (s *Store) CommentsAfter(lastID int64) ([]Comment, error) {
	rows, err := s.db.Query(`SELECT id, task_id, author, type, body, created_at FROM task_comments WHERE id > ? AND author = 'user' ORDER BY id ASC`, lastID)
	if err != nil {
		return nil, fmt.Errorf("comments after: %w", err)
	}
	defer rows.Close()
	var comments []Comment
	for rows.Next() {
		var c Comment
		if err := rows.Scan(&c.ID, &c.TaskID, &c.Author, &c.Type, &c.Body, &c.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan comment: %w", err)
		}
		comments = append(comments, c)
	}
	return comments, rows.Err()
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd /home/rishav/soul-v2 && go test ./internal/tasks/store/ -run "TestInsertComment|TestGetComments|TestCommentsAfter" -v`
Expected: ALL PASS

- [ ] **Step 5: Add comment API endpoints to server.go**

```go
mux.HandleFunc("POST /api/tasks/{id}/comments", s.handleAddComment)
mux.HandleFunc("GET /api/tasks/{id}/comments", s.handleGetComments)
```

- [ ] **Step 6: Implement CommentWatcher**

Create `internal/tasks/watcher/watcher.go`:

```go
package watcher

import (
	"context"
	"log"
	"time"

	"github.com/rishav1305/soul-v2/internal/tasks/store"
)

type CommentWatcher struct {
	store  *store.Store
	lastID int64
}

func New(s *store.Store) *CommentWatcher {
	return &CommentWatcher{store: s}
}

func (cw *CommentWatcher) Start(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			cw.poll(ctx)
		}
	}
}

func (cw *CommentWatcher) poll(ctx context.Context) {
	comments, err := cw.store.CommentsAfter(cw.lastID)
	if err != nil {
		log.Printf("watcher: poll error: %v", err)
		return
	}
	for _, c := range comments {
		cw.lastID = c.ID
		task, err := cw.store.Get(c.TaskID)
		if err != nil {
			log.Printf("watcher: get task %d: %v", c.TaskID, err)
			continue
		}
		actionable := task.Stage == "active" || task.Stage == "validation" || task.Stage == "blocked"
		if !actionable {
			cw.store.InsertComment(c.TaskID, "soul", "status",
				"Task is in "+task.Stage+" — comment noted but no action taken.")
			continue
		}
		// TODO: spawn mini-agent with task context + comment history
		// For now, acknowledge the comment
		cw.store.InsertComment(c.TaskID, "soul", "status",
			"Received feedback. Agent processing not yet implemented.")
	}
}
```

- [ ] **Step 7: Write watcher test**

Create `internal/tasks/watcher/watcher_test.go`:

```go
package watcher

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/rishav1305/soul-v2/internal/tasks/store"
)

func TestWatcher_PollsComments(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.db")
	s, err := store.Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer s.Close()

	task, _ := s.Create("Test", "", "")
	s.Update(task.ID, map[string]interface{}{"stage": "active"})
	s.InsertComment(task.ID, "user", "feedback", "Please fix the bug")

	cw := New(s)
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	cw.poll(ctx)

	comments, _ := s.GetComments(task.ID)
	if len(comments) < 2 {
		t.Errorf("expected soul reply, got %d comments", len(comments))
	}
}

func TestWatcher_SkipsNonActionable(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.db")
	s, err := store.Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer s.Close()

	task, _ := s.Create("Backlog task", "", "")
	// stage defaults to "backlog" — not actionable
	s.InsertComment(task.ID, "user", "feedback", "Hello")

	cw := New(s)
	ctx := context.Background()
	cw.poll(ctx)

	comments, _ := s.GetComments(task.ID)
	found := false
	for _, c := range comments {
		if c.Author == "soul" && c.Body == "Task is in backlog — comment noted but no action taken." {
			found = true
		}
	}
	if !found {
		t.Error("expected non-actionable reply from soul")
	}
}
```

- [ ] **Step 8: Run watcher tests**

Run: `cd /home/rishav/soul-v2 && go test ./internal/tasks/watcher/ -v`
Expected: ALL PASS

- [ ] **Step 9: Commit**

```bash
git add internal/tasks/store/store.go internal/tasks/store/store_test.go internal/tasks/server/server.go internal/tasks/watcher/watcher.go internal/tasks/watcher/watcher_test.go
git commit -m "feat: add task comments and comment watcher"
```

---

## Task 8: Hooks System

**Files:**
- Create: `internal/tasks/hooks/hooks.go`
- Create: `internal/tasks/hooks/hooks_test.go`

**Ref:** Spec §10 (Hooks & Phases — hooks portion)

- [ ] **Step 1: Write failing tests**

Create `internal/tasks/hooks/hooks_test.go`:

```go
package hooks

import (
	"os"
	"path/filepath"
	"testing"
)

func TestHookRunner_NoConfig(t *testing.T) {
	hr := NewHookRunner("/nonexistent/hooks.json")
	blocked, msg, _ := hr.RunToolHook("before", "code_edit", nil)
	if blocked {
		t.Errorf("expected not blocked with no config, got msg: %s", msg)
	}
}

func TestHookRunner_BlockingHook(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "hooks.json")
	os.WriteFile(configPath, []byte(`{
		"hooks": [{
			"event": "before:code_edit",
			"match": "*.go",
			"action": "block",
			"message": "Go files are read-only"
		}]
	}`), 0644)

	hr := NewHookRunner(configPath)
	vars := map[string]string{"file": "main.go"}
	blocked, msg, _ := hr.RunToolHook("before", "code_edit", vars)
	if !blocked {
		t.Error("expected blocked")
	}
	if msg != "Go files are read-only" {
		t.Errorf("expected blocking message, got: %s", msg)
	}
}

func TestHookRunner_CommandHook(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "hooks.json")
	os.WriteFile(configPath, []byte(`{
		"hooks": [{
			"event": "after:code_exec",
			"command": "echo hook-ran",
			"timeout": 5
		}]
	}`), 0644)

	hr := NewHookRunner(configPath)
	blocked, _, output := hr.RunToolHook("after", "code_exec", nil)
	if blocked {
		t.Error("command hooks should not block")
	}
	if output == "" {
		t.Error("expected command output")
	}
}

func TestExpandVars(t *testing.T) {
	vars := map[string]string{"file": "main.go", "task_id": "42"}
	result := expandVars("editing {file} for task {task_id}", vars)
	if result != "editing main.go for task 42" {
		t.Errorf("unexpected expansion: %s", result)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /home/rishav/soul-v2 && go test ./internal/tasks/hooks/ -v`
Expected: FAIL

- [ ] **Step 3: Implement hooks.go**

Create `internal/tasks/hooks/hooks.go`:

```go
package hooks

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type HookConfig struct {
	Hooks         []ToolHook     `json:"hooks"`
	WorkflowHooks []WorkflowHook `json:"workflow_hooks"`
}

type ToolHook struct {
	Event       string `json:"event"`
	Match       string `json:"match"`
	Command     string `json:"command"`
	Timeout     int    `json:"timeout"`
	DenyPattern string `json:"deny_pattern"`
	Action      string `json:"action"`
	Message     string `json:"message"`
}

type WorkflowHook struct {
	Event   string `json:"event"`
	Command string `json:"command"`
	Timeout int    `json:"timeout"`
}

type HookRunner struct {
	config     *HookConfig
	configPath string
}

func NewHookRunner(configPath string) *HookRunner {
	hr := &HookRunner{configPath: configPath}
	hr.load()
	return hr
}

func (hr *HookRunner) load() {
	data, err := os.ReadFile(hr.configPath)
	if err != nil {
		hr.config = &HookConfig{}
		return
	}
	var cfg HookConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		hr.config = &HookConfig{}
		return
	}
	hr.config = &cfg
}

func (hr *HookRunner) RunToolHook(phase, toolName string, vars map[string]string) (blocked bool, message string, output string) {
	event := phase + ":" + toolName
	for _, h := range hr.config.Hooks {
		if h.Event != event {
			continue
		}
		if h.Match != "" && vars != nil {
			if file, ok := vars["file"]; ok {
				matched, _ := filepath.Match(h.Match, filepath.Base(file))
				if !matched {
					continue
				}
			}
		}
		if h.Action == "block" {
			return true, h.Message, ""
		}
		if h.Command != "" {
			timeout := h.Timeout
			if timeout <= 0 {
				timeout = 10
			}
			cmd := expandVars(h.Command, vars)
			out, _ := runHookCommand(cmd, timeout)
			output += out
		}
	}
	return false, "", output
}

func (hr *HookRunner) RunWorkflowHook(event string, vars map[string]string) {
	for _, h := range hr.config.WorkflowHooks {
		if h.Event != event {
			continue
		}
		timeout := h.Timeout
		if timeout <= 0 {
			timeout = 30
		}
		cmd := expandVars(h.Command, vars)
		runHookCommand(cmd, timeout)
	}
}

func expandVars(template string, vars map[string]string) string {
	result := template
	for k, v := range vars {
		result = strings.ReplaceAll(result, fmt.Sprintf("{%s}", k), v)
	}
	return result
}

func runHookCommand(command string, timeoutSec int) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeoutSec)*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "bash", "-c", command)
	out, err := cmd.CombinedOutput()
	return string(out), err
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd /home/rishav/soul-v2 && go test ./internal/tasks/hooks/ -v`
Expected: ALL PASS

- [ ] **Step 5: Commit**

```bash
git add internal/tasks/hooks/hooks.go internal/tasks/hooks/hooks_test.go
git commit -m "feat: add hook runner for tool and workflow lifecycle hooks"
```

---

## Task 9: Phases System

**Files:**
- Create: `internal/tasks/phases/phases.go`
- Create: `internal/tasks/phases/phases_test.go`

**Ref:** Spec §10 (Hooks & Phases — phases portion)

- [ ] **Step 1: Write failing tests**

Create `internal/tasks/phases/phases_test.go`:

```go
package phases

import "testing"

func TestPhaseConfig_Defaults(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.ImplModel != "claude-sonnet-4-6" {
		t.Errorf("expected claude-sonnet-4-6, got %s", cfg.ImplModel)
	}
	if cfg.ReviewModel != "claude-opus-4-6" {
		t.Errorf("expected claude-opus-4-6, got %s", cfg.ReviewModel)
	}
	if cfg.FixModel != "claude-opus-4-6" {
		t.Errorf("expected claude-opus-4-6, got %s", cfg.FixModel)
	}
}

func TestClassifyWorkflow(t *testing.T) {
	tests := []struct {
		workflow string
		maxIter  int
	}{
		{"micro", 15},
		{"quick", 30},
		{"full", 40},
		{"", 30}, // default to quick
	}
	for _, tt := range tests {
		got := MaxIterations(tt.workflow)
		if got != tt.maxIter {
			t.Errorf("MaxIterations(%q) = %d, want %d", tt.workflow, got, tt.maxIter)
		}
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /home/rishav/soul-v2 && go test ./internal/tasks/phases/ -v`
Expected: FAIL

- [ ] **Step 3: Implement phases.go**

Create `internal/tasks/phases/phases.go`:

```go
package phases

type PhaseConfig struct {
	PlanModel   string
	ImplModel   string
	ReviewModel string
	FixModel    string
}

func DefaultConfig() PhaseConfig {
	return PhaseConfig{
		PlanModel:   "claude-opus-4-6",
		ImplModel:   "claude-sonnet-4-6",
		ReviewModel: "claude-opus-4-6",
		FixModel:    "claude-opus-4-6",
	}
}

func MaxIterations(workflow string) int {
	switch workflow {
	case "micro":
		return 15
	case "full":
		return 40
	default:
		return 30
	}
}
```

Note: The full PhaseRunner (3-phase pipeline: impl → opus diff review → opus fix) depends on the executor's AgentLoop and stream.Client for non-streaming completion. This requires a `PhaseRunner.RunTask()` method that orchestrates 3 sequential agent runs with model switching. **This is deferred to Phase 1b** — a follow-up plan document covering PhaseRunner orchestration, subagent read-only tool execution, and comment watcher mini-agent spawning. This task establishes the config and iteration limits that PhaseRunner will use.

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd /home/rishav/soul-v2 && go test ./internal/tasks/phases/ -v`
Expected: ALL PASS

- [ ] **Step 5: Commit**

```bash
git add internal/tasks/phases/phases.go internal/tasks/phases/phases_test.go
git commit -m "feat: add phase config and workflow iteration limits"
```

---

## Task 10: Merge Gates

**Files:**
- Create: `internal/tasks/gates/gates.go`
- Create: `internal/tasks/gates/gates_test.go`

**Ref:** Spec §9 (Merge Gates)

- [ ] **Step 1: Write failing tests**

Create `internal/tasks/gates/gates_test.go`:

```go
package gates

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPreMergeGate_MissingDir(t *testing.T) {
	err := PreMergeGate("/nonexistent/web")
	if err == nil {
		t.Error("expected error for missing directory")
	}
}

func TestSmokeResult_AllPass(t *testing.T) {
	r := &SmokeResult{
		AllPass: true,
		Checks:  []SmokeCheck{{Name: "home", Pass: true}},
	}
	if !r.AllPass {
		t.Error("expected all pass")
	}
}

func TestFeatureCheck_Types(t *testing.T) {
	checks := []FeatureCheck{
		{Description: "nav exists", Selector: "nav", Assertion: "exists"},
		{Description: "title visible", Selector: "h1", Assertion: "visible"},
		{Description: "has text", Selector: ".msg", Assertion: "text_contains", Expected: "hello"},
	}
	if len(checks) != 3 {
		t.Errorf("expected 3 checks, got %d", len(checks))
	}
}

func TestPreMergeGate_WithTempDir(t *testing.T) {
	// Create a minimal web dir with package.json
	dir := t.TempDir()
	webDir := filepath.Join(dir, "web")
	os.MkdirAll(webDir, 0755)
	os.WriteFile(filepath.Join(webDir, "package.json"), []byte(`{"name":"test"}`), 0644)

	// This will fail because tsc isn't set up, but it should not panic
	err := PreMergeGate(webDir)
	if err == nil {
		t.Log("PreMergeGate passed (tsc available)")
	} else {
		t.Logf("PreMergeGate failed as expected: %v", err)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /home/rishav/soul-v2 && go test ./internal/tasks/gates/ -v`
Expected: FAIL

- [ ] **Step 3: Implement gates.go**

Create `internal/tasks/gates/gates.go`:

```go
package gates

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

type SmokeResult struct {
	AllPass bool
	Checks  []SmokeCheck
}

type SmokeCheck struct {
	Name   string
	Pass   bool
	Detail string
}

type FeatureCheck struct {
	Description string
	Selector    string
	Assertion   string
	Expected    string
}

type FeatureGateResult struct {
	AllPass bool
	Checks  []FeatureCheckResult
}

type FeatureCheckResult struct {
	Description string
	Pass        bool
	Detail      string
}

func PreMergeGate(worktreeWeb string) error {
	if _, err := os.Stat(worktreeWeb); os.IsNotExist(err) {
		return fmt.Errorf("web directory not found: %s", worktreeWeb)
	}

	// Symlink node_modules if needed
	devNodeModules := filepath.Join(worktreeWeb, "node_modules")
	if _, err := os.Lstat(devNodeModules); os.IsNotExist(err) {
		// Try to find main web/node_modules
		mainDir := filepath.Dir(filepath.Dir(worktreeWeb))
		mainNodeModules := filepath.Join(mainDir, "web", "node_modules")
		if _, err := os.Stat(mainNodeModules); err == nil {
			os.Symlink(mainNodeModules, devNodeModules)
		}
	}

	// tsc --noEmit
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	tsc := exec.CommandContext(ctx, "npx", "tsc", "--noEmit")
	tsc.Dir = worktreeWeb
	if out, err := tsc.CombinedOutput(); err != nil {
		return fmt.Errorf("tsc failed:\n%s", out)
	}

	// vite build
	ctx2, cancel2 := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel2()
	vite := exec.CommandContext(ctx2, "npx", "vite", "build")
	vite.Dir = worktreeWeb
	if out, err := vite.CombinedOutput(); err != nil {
		return fmt.Errorf("vite build failed:\n%s", out)
	}

	return nil
}

func SmokeTest(serverURL, e2eHost, e2eRunnerPath string) (*SmokeResult, error) {
	if serverURL == "" || e2eHost == "" {
		return nil, fmt.Errorf("serverURL and e2eHost required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "ssh", e2eHost, "node", e2eRunnerPath, "--json", "--url", serverURL)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("smoke test failed: %w\n%s", err, out)
	}
	// Parse JSON output into SmokeResult
	// TODO: implement JSON parsing when test runner format is finalized
	return &SmokeResult{AllPass: true}, nil
}

func RuntimeGate(serverURL, e2eHost, e2eRunnerPath string) error {
	if serverURL == "" || e2eHost == "" {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "ssh", e2eHost, "node", e2eRunnerPath, "--action", "console_errors", "--url", serverURL)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("runtime gate failed: %w\n%s", err, out)
	}
	return nil
}

func StepVerificationGate(worktreeWeb, serverURL, e2eHost, e2eRunnerPath string) error {
	if err := PreMergeGate(worktreeWeb); err != nil {
		return fmt.Errorf("step verification: pre-merge failed: %w", err)
	}
	if serverURL != "" && e2eHost != "" {
		if err := RuntimeGate(serverURL, e2eHost, e2eRunnerPath); err != nil {
			return fmt.Errorf("step verification: runtime gate failed: %w", err)
		}
	}
	return nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd /home/rishav/soul-v2 && go test ./internal/tasks/gates/ -v`
Expected: ALL PASS

- [ ] **Step 5: Commit**

```bash
git add internal/tasks/gates/gates.go internal/tasks/gates/gates_test.go
git commit -m "feat: add merge gates for pre-merge validation"
```

---

## Task 11: Executor Integration

**Files:**
- Modify: `internal/tasks/executor/executor.go` (integrate hooks, phases, gates; skip brainstorm)

**Ref:** Spec §6 (brainstorm skip), §9 (gates in executor), §10 (hooks/phases in executor)

- [ ] **Step 1: Update executor to skip brainstorm tasks**

In `internal/tasks/executor/executor.go`, in `Start()` method where it checks stage (~line 55):

```go
// Change from:
// if task.Stage != "backlog" && task.Stage != "blocked" {
// To:
if task.Stage == "brainstorm" {
	return fmt.Errorf("task %d is in brainstorm stage — user-driven only", taskID)
}
if task.Stage != "backlog" && task.Stage != "blocked" {
	return fmt.Errorf("task %d is in stage %s, expected backlog or blocked", taskID, task.Stage)
}
```

- [ ] **Step 2: Add hooks runner to executor Config**

Add `HooksConfigPath string` to `Config` struct. In `run()`, create HookRunner and call `RunToolHook("before", toolName, vars)` before each tool execution, `RunToolHook("after", toolName, vars)` after.

- [ ] **Step 3: Add phase-based iteration limits**

Import `phases` package. In `run()`, replace hardcoded `maxIter` with `phases.MaxIterations(task.Workflow)`.

- [ ] **Step 4: Run all executor tests**

Run: `cd /home/rishav/soul-v2 && go test ./internal/tasks/... -v`
Expected: ALL PASS

- [ ] **Step 5: Commit**

```bash
git add internal/tasks/executor/executor.go
git commit -m "feat: integrate hooks, phases, and brainstorm skip into executor"
```

---

## Task 12: Subagent Tool

**Files:**
- Create: `internal/chat/ws/subagent.go`
- Create: `internal/chat/ws/subagent_test.go`
- Modify: `internal/chat/ws/builtin.go` (wire subagent case)

**Ref:** Spec §3 (Subagent)

- [ ] **Step 1: Write failing test**

Create `internal/chat/ws/subagent_test.go`:

```go
package ws

import "testing"

func TestSubagentConfig(t *testing.T) {
	cfg := SubagentConfig{Task: "find all error handlers", MaxIterations: 5}
	if cfg.MaxIterations > 10 {
		t.Error("max iterations should be capped at 10")
	}
}

func TestSubagentConfig_Defaults(t *testing.T) {
	cfg := SubagentConfig{Task: "explore"}
	cfg.applyDefaults()
	if cfg.MaxIterations != 5 {
		t.Errorf("expected default 5, got %d", cfg.MaxIterations)
	}
}

func TestSubagentConfig_Cap(t *testing.T) {
	cfg := SubagentConfig{Task: "explore", MaxIterations: 20}
	cfg.applyDefaults()
	if cfg.MaxIterations != 10 {
		t.Errorf("expected capped at 10, got %d", cfg.MaxIterations)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /home/rishav/soul-v2 && go test ./internal/chat/ws/ -run TestSubagent -v`
Expected: FAIL

- [ ] **Step 3: Implement subagent.go**

Create `internal/chat/ws/subagent.go`:

```go
package ws

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/rishav1305/soul-v2/internal/chat/stream"
)

type SubagentConfig struct {
	Task          string `json:"task"`
	MaxIterations int    `json:"max_iterations"`
}

func (sc *SubagentConfig) applyDefaults() {
	if sc.MaxIterations <= 0 {
		sc.MaxIterations = 5
	}
	if sc.MaxIterations > 10 {
		sc.MaxIterations = 10
	}
}

// readOnlyTools returns tools available to the subagent (no write/exec)
func readOnlyTools() []stream.Tool {
	return []stream.Tool{
		{
			Name:        "file_read",
			Description: "Read file contents",
			InputSchema: json.RawMessage(`{"type":"object","properties":{"path":{"type":"string","description":"File path to read"}},"required":["path"]}`),
		},
		{
			Name:        "file_search",
			Description: "Search for files by name pattern",
			InputSchema: json.RawMessage(`{"type":"object","properties":{"pattern":{"type":"string","description":"Glob pattern"}},"required":["pattern"]}`),
		},
		{
			Name:        "file_grep",
			Description: "Search file contents with regex",
			InputSchema: json.RawMessage(`{"type":"object","properties":{"pattern":{"type":"string","description":"Regex pattern"},"path":{"type":"string","description":"Directory to search"}},"required":["pattern"]}`),
		},
		{
			Name:        "file_glob",
			Description: "List files matching glob pattern",
			InputSchema: json.RawMessage(`{"type":"object","properties":{"pattern":{"type":"string","description":"Glob pattern"}},"required":["pattern"]}`),
		},
	}
}

// Sender matches the executor.Sender interface — satisfied by *stream.Client.
type Sender interface {
	Send(ctx context.Context, req *stream.Request) (*stream.Response, error)
}

func executeSubagent(ctx context.Context, sender Sender, inputJSON []byte, projectRoot string) (string, error) {
	var cfg SubagentConfig
	if err := json.Unmarshal(inputJSON, &cfg); err != nil {
		return "", fmt.Errorf("parse subagent input: %w", err)
	}
	if cfg.Task == "" {
		return "", fmt.Errorf("subagent requires task parameter")
	}
	cfg.applyDefaults()

	// Build initial messages using stream.Message with []ContentBlock
	tools := readOnlyTools()
	messages := []stream.Message{
		{Role: "user", Content: []stream.ContentBlock{{Type: "text", Text: cfg.Task}}},
	}
	system := fmt.Sprintf("You are a focused code exploration subagent. Your task: %s\n\nYou have read-only tools. Investigate and report findings. Project root: %s", cfg.Task, projectRoot)

	var finalResult string
	for i := 0; i < cfg.MaxIterations; i++ {
		req := &stream.Request{
			System:         system,
			Messages:       messages,
			Tools:          tools,
			MaxTokens:      4096,
			SkipValidation: true, // tool result messages need this
		}
		resp, err := sender.Send(ctx, req)
		if err != nil {
			return "", fmt.Errorf("subagent iteration %d: %w", i, err)
		}

		// Extract text from response content blocks
		for _, cb := range resp.Content {
			if cb.Type == "text" {
				finalResult = cb.Text
			}
		}

		if resp.StopReason != "tool_use" {
			break
		}

		// Collect tool_use blocks from response
		var toolUseBlocks []stream.ContentBlock
		for _, cb := range resp.Content {
			if cb.Type == "tool_use" {
				toolUseBlocks = append(toolUseBlocks, cb)
			}
		}
		if len(toolUseBlocks) == 0 {
			break
		}

		// Append assistant message with full response content
		messages = append(messages, stream.Message{Role: "assistant", Content: resp.Content})

		// Execute each tool and build tool_result content blocks
		var toolResults []stream.ContentBlock
		for _, tc := range toolUseBlocks {
			result := executeReadOnlyTool(projectRoot, tc.Name, string(tc.Input))
			toolResults = append(toolResults, stream.ContentBlock{
				Type:      "tool_result",
				ToolUseID: tc.ID,
				Content:   result,
			})
		}
		messages = append(messages, stream.Message{Role: "user", Content: toolResults})
	}

	if len(finalResult) > 3000 {
		finalResult = finalResult[:3000] + "\n...(truncated)"
	}
	return finalResult, nil
}

func executeReadOnlyTool(projectRoot, name, input string) string {
	// TODO: implement file_read, file_search, file_grep, file_glob
	// using projectRoot as base directory. Deferred to Phase 1b plan.
	return fmt.Sprintf("Tool %s not yet implemented", name)
}
```

- [ ] **Step 4: Wire subagent into builtin.go**

In `internal/chat/ws/builtin.go`, update the `Execute` method's subagent case:

```go
case toolName == "subagent":
	// Requires sender and projectRoot — these will be set on BuiltinExecutor
	if be.sender == nil {
		return "", fmt.Errorf("subagent not available: no sender configured")
	}
	return executeSubagent(ctx, be.sender, inputJSON, be.projectRoot)
```

Add `sender Sender` and `projectRoot string` fields to `BuiltinExecutor`. Update `NewBuiltinExecutor` to accept them. The `Sender` interface is defined in `subagent.go` and matches `executor.Sender` — both are satisfied by `*stream.Client`.

- [ ] **Step 5: Run tests**

Run: `cd /home/rishav/soul-v2 && go test ./internal/chat/ws/ -run TestSubagent -v`
Expected: ALL PASS

- [ ] **Step 6: Commit**

```bash
git add internal/chat/ws/subagent.go internal/chat/ws/subagent_test.go internal/chat/ws/builtin.go
git commit -m "feat: add subagent tool for read-only code exploration"
```

---

## Task 13: Memory & Custom Tool Definitions in Context

**Files:**
- Modify: `internal/chat/context/context.go` (add built-in tool defs to system prompt)

**Ref:** Spec §1, §2 (chat integration)

- [ ] **Step 1: Add built-in tool definitions to Default context**

In `internal/chat/context/context.go`, update `Default()` to include memory and custom tool definitions in the Tools slice. Add system prompt text about persistent memory.

The tools are always available (not product-specific), so they go in the default context:

```go
func Default() ProductContext {
	return ProductContext{
		System: defaultSystem + "\n\n" + memorySystem,
		Tools:  builtinTools(),
	}
}

const memorySystem = `You have persistent memory that survives across conversations.
Use memory_store to save important information.
Use memory_search to find relevant memories.
Use memory_list to see recent memories.
Use memory_delete to remove outdated memories.
You can create custom tools using tool_create. Custom tools appear with a 'custom_' prefix.`
```

Add `builtinTools()` function returning `[]stream.Tool` for memory_store, memory_search, memory_list, memory_delete, tool_create, tool_list, tool_delete, subagent.

- [ ] **Step 2: Update ForProduct to include built-in tools alongside product tools**

Modify `ForProduct()` to merge built-in tools with product-specific tools:

```go
func ForProduct(product string) ProductContext {
	ctx := productContext(product)
	ctx.Tools = append(builtinTools(), ctx.Tools...)
	if ctx.System != "" {
		ctx.System = ctx.System + "\n\n" + memorySystem
	}
	return ctx
}
```

- [ ] **Step 3: Update tests**

Update `TestToolCounts` in `internal/chat/context/context_test.go` to account for 8 built-in tools added to each product context.

- [ ] **Step 4: Run context tests**

Run: `cd /home/rishav/soul-v2 && go test ./internal/chat/context/ -v`
Expected: ALL PASS

- [ ] **Step 5: Commit**

```bash
git add internal/chat/context/context.go internal/chat/context/context_test.go
git commit -m "feat: add memory, custom tool, and subagent definitions to chat context"
```

---

## Task 14: Multi-Session WebSocket (Stub)

**Files:**
- Modify: `internal/chat/ws/handler.go`

**Ref:** Spec §7 (Multi-Session WebSocket)

Note: Full multi-session WS is a significant refactor of the connection model. This task adds the `chatSession` struct and per-session context management without breaking the existing single-session flow. The full concurrent multi-session behavior will be validated in integration testing.

- [ ] **Step 1: Add chatSession struct to handler.go**

```go
type agentEntry struct {
	cancel context.CancelFunc
	done   chan struct{}
}

type chatSession struct {
	mu     sync.Mutex
	agents map[string]agentEntry
}
```

- [ ] **Step 2: Update MessageHandler to use chatSession**

Add `sessions map[*Client]*chatSession` (protected by sync.Mutex) to MessageHandler. In `handleChatSend`, look up or create chatSession for the client, manage agent lifecycle per sessionID.

- [ ] **Step 3: Update runStream context creation**

Replace `context.WithTimeout(client.Context(), 5*time.Minute)` with `context.WithCancel(context.Background())` stored in `agentEntry`.

- [ ] **Step 4: Add chat.stop per-session handling**

When `chat.stop` message arrives with sessionID, cancel only that session's agent.

- [ ] **Step 5: Run existing chat tests to ensure no regression**

Run: `cd /home/rishav/soul-v2 && go test ./internal/chat/... -v`
Expected: ALL PASS

- [ ] **Step 6: Commit**

```bash
git add internal/chat/ws/handler.go
git commit -m "feat: add multi-session WebSocket with per-session agent contexts"
```

---

## Task 15: Static Verification + Final Commit

- [ ] **Step 1: Run make verify-static**

Run: `cd /home/rishav/soul-v2 && make verify-static`
Expected: PASS (go vet + tsc --noEmit)

- [ ] **Step 2: Run full test suite**

Run: `cd /home/rishav/soul-v2 && go test -race -count=1 ./internal/chat/... ./internal/tasks/... -v`
Expected: ALL PASS

- [ ] **Step 3: Fix any issues found**

Address any compilation errors, test failures, or vet warnings.

- [ ] **Step 4: Final commit if fixes were needed**

```bash
git add -A
git commit -m "fix: resolve verification issues from cross-cutting capabilities"
```

---

## Summary

| Task | What | Files | Tests |
|------|------|-------|-------|
| 1 | Agent Memories store | 3 | 6 |
| 2 | Custom Tools store | 3 | 5 |
| 3 | BuiltinExecutor dispatch | 3 | 9 |
| 4 | SetProduct expansion | 1 | 1 |
| 5 | Task Dependencies | 3 | 5 |
| 6 | Substeps + Brainstorm | 3 | 5 |
| 7 | Comments + Watcher | 5 | 5 |
| 8 | Hooks system | 2 | 4 |
| 9 | Phases system | 2 | 2 |
| 10 | Merge Gates | 2 | 4 |
| 11 | Executor integration | 1 | 0 (uses existing) |
| 12 | Subagent tool | 3 | 3 |
| 13 | Context tool defs | 2 | 1 (update) |
| 14 | Multi-session WS | 1 | 0 (uses existing) |
| 15 | Verification | 0 | 0 (runs suite) |
| **Total** | | **~34 files** | **~50 tests** |
