package ws

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/rishav1305/soul/internal/chat/session"
)

func openTestStore(t *testing.T) *session.Store {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	store, err := session.Open(dbPath)
	if err != nil {
		t.Fatalf("open session store: %v", err)
	}
	t.Cleanup(func() { store.Close() })
	return store
}

func TestBuiltinExecutor_CanHandle(t *testing.T) {
	be := NewBuiltinExecutor(nil)

	tests := []struct {
		name   string
		tool   string
		expect bool
	}{
		{"memory_store", "memory_store", true},
		{"memory_search", "memory_search", true},
		{"memory_list", "memory_list", true},
		{"memory_delete", "memory_delete", true},
		{"tool_create", "tool_create", true},
		{"tool_list", "tool_list", true},
		{"tool_delete", "tool_delete", true},
		{"custom_x", "custom_x", true},
		{"custom_my_tool", "custom_my_tool", true},
		{"subagent", "subagent", true},
		{"list_tasks", "list_tasks", false},
		{"empty", "", false},
		{"get_project", "get_project", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := be.CanHandle(tt.tool)
			if got != tt.expect {
				t.Errorf("CanHandle(%q) = %v, want %v", tt.tool, got, tt.expect)
			}
		})
	}
}

func TestBuiltinExecutor_MemoryStore(t *testing.T) {
	store := openTestStore(t)
	be := NewBuiltinExecutor(store)

	input, _ := json.Marshal(map[string]string{
		"key":     "test-key",
		"content": "test content",
		"tags":    "tag1,tag2",
	})

	result, err := be.Execute(context.Background(), "memory_store", input)
	if err != nil {
		t.Fatalf("memory_store: %v", err)
	}
	if result == "" {
		t.Fatal("memory_store returned empty result")
	}

	var mem session.Memory
	if err := json.Unmarshal([]byte(result), &mem); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if mem.Key != "test-key" {
		t.Errorf("key = %q, want %q", mem.Key, "test-key")
	}
	if mem.Content != "test content" {
		t.Errorf("content = %q, want %q", mem.Content, "test content")
	}
}

func TestBuiltinExecutor_MemorySearch(t *testing.T) {
	store := openTestStore(t)
	be := NewBuiltinExecutor(store)

	// Upsert via store directly.
	if _, err := store.UpsertMemory("search-key", "findme content", ""); err != nil {
		t.Fatalf("upsert: %v", err)
	}

	input, _ := json.Marshal(map[string]string{"query": "findme"})
	result, err := be.Execute(context.Background(), "memory_search", input)
	if err != nil {
		t.Fatalf("memory_search: %v", err)
	}

	var memories []session.Memory
	if err := json.Unmarshal([]byte(result), &memories); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(memories) == 0 {
		t.Fatal("memory_search returned no results")
	}
	if memories[0].Key != "search-key" {
		t.Errorf("key = %q, want %q", memories[0].Key, "search-key")
	}
}

func TestBuiltinExecutor_MemoryList(t *testing.T) {
	store := openTestStore(t)
	be := NewBuiltinExecutor(store)

	if _, err := store.UpsertMemory("list-key", "list content", ""); err != nil {
		t.Fatalf("upsert: %v", err)
	}

	input, _ := json.Marshal(map[string]interface{}{"limit": 10})
	result, err := be.Execute(context.Background(), "memory_list", input)
	if err != nil {
		t.Fatalf("memory_list: %v", err)
	}

	var memories []session.Memory
	if err := json.Unmarshal([]byte(result), &memories); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(memories) == 0 {
		t.Fatal("memory_list returned no results")
	}
}

func TestBuiltinExecutor_MemoryDelete(t *testing.T) {
	store := openTestStore(t)
	be := NewBuiltinExecutor(store)

	if _, err := store.UpsertMemory("del-key", "to delete", ""); err != nil {
		t.Fatalf("upsert: %v", err)
	}

	input, _ := json.Marshal(map[string]string{"key": "del-key"})
	result, err := be.Execute(context.Background(), "memory_delete", input)
	if err != nil {
		t.Fatalf("memory_delete: %v", err)
	}

	var resp map[string]string
	if err := json.Unmarshal([]byte(result), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp["status"] != "deleted" {
		t.Errorf("status = %q, want %q", resp["status"], "deleted")
	}

	// Verify actually deleted.
	_, err = store.SearchMemories("del-key")
	if err != nil {
		t.Fatalf("search after delete: %v", err)
	}
}

func TestBuiltinExecutor_ToolCreate(t *testing.T) {
	store := openTestStore(t)
	be := NewBuiltinExecutor(store)

	input, _ := json.Marshal(map[string]string{
		"name":             "my_tool",
		"description":      "A test tool",
		"command_template": "echo hello",
	})

	result, err := be.Execute(context.Background(), "tool_create", input)
	if err != nil {
		t.Fatalf("tool_create: %v", err)
	}

	var tool session.CustomTool
	if err := json.Unmarshal([]byte(result), &tool); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if tool.Name != "my_tool" {
		t.Errorf("name = %q, want %q", tool.Name, "my_tool")
	}
	if tool.Description != "A test tool" {
		t.Errorf("description = %q, want %q", tool.Description, "A test tool")
	}
}

func TestBuiltinExecutor_ToolList(t *testing.T) {
	store := openTestStore(t)
	be := NewBuiltinExecutor(store)

	// Create via store directly.
	if _, err := store.CreateCustomTool("list_tool", "desc", "{}", "echo hi"); err != nil {
		t.Fatalf("create: %v", err)
	}

	result, err := be.Execute(context.Background(), "tool_list", []byte("{}"))
	if err != nil {
		t.Fatalf("tool_list: %v", err)
	}

	var tools []session.CustomTool
	if err := json.Unmarshal([]byte(result), &tools); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(tools) == 0 {
		t.Fatal("tool_list returned no results")
	}
	if tools[0].Name != "list_tool" {
		t.Errorf("name = %q, want %q", tools[0].Name, "list_tool")
	}
}

func TestBuiltinExecutor_ToolDelete(t *testing.T) {
	store := openTestStore(t)
	be := NewBuiltinExecutor(store)

	if _, err := store.CreateCustomTool("del_tool", "desc", "{}", "echo bye"); err != nil {
		t.Fatalf("create: %v", err)
	}

	input, _ := json.Marshal(map[string]string{"name": "del_tool"})
	result, err := be.Execute(context.Background(), "tool_delete", input)
	if err != nil {
		t.Fatalf("tool_delete: %v", err)
	}

	var resp map[string]string
	if err := json.Unmarshal([]byte(result), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp["status"] != "deleted" {
		t.Errorf("status = %q, want %q", resp["status"], "deleted")
	}
}

func TestBuiltinExecutor_UnknownTool(t *testing.T) {
	store := openTestStore(t)
	be := NewBuiltinExecutor(store)

	_, err := be.Execute(context.Background(), "memory_unknown", []byte("{}"))
	if err == nil {
		t.Fatal("expected error for unknown memory tool")
	}
}

func TestBuiltinExecutor_Subagent(t *testing.T) {
	be := NewBuiltinExecutor(nil)

	_, err := be.Execute(context.Background(), "subagent", []byte("{}"))
	if err == nil {
		t.Fatal("expected error for subagent")
	}
	if err.Error() != "subagent not available: no sender configured" {
		t.Errorf("error = %q, want %q", err.Error(), "subagent not available: no sender configured")
	}
}
