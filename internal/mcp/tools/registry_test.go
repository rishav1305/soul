package tools

import (
	"context"
	"encoding/json"
	"testing"
)

func TestNewRegistry_CollectsTools(t *testing.T) {
	r := NewRegistry(nil)
	count := len(r.List())
	t.Logf("collected %d product tools", count)
	if count < 80 {
		t.Errorf("expected >= 80 tools, got %d", count)
	}
}

func TestNewRegistry_NoDuplicates(t *testing.T) {
	r := NewRegistry(nil)
	seen := make(map[string]bool)
	for _, tool := range r.List() {
		if seen[tool.Name] {
			t.Errorf("duplicate tool name: %s", tool.Name)
		}
		seen[tool.Name] = true
	}
}

func TestNewRegistry_ExcludesBuiltins(t *testing.T) {
	r := NewRegistry(nil)
	builtins := []string{"memory_store", "memory_search", "memory_list", "memory_delete",
		"tool_create", "tool_list", "tool_delete", "subagent"}
	for _, name := range builtins {
		if r.Has(name) {
			t.Errorf("built-in tool %q should not be in registry", name)
		}
	}
}

func TestRegistry_HasExpectedTools(t *testing.T) {
	r := NewRegistry(nil)
	expected := []string{"list_tasks", "compliance__scan", "lead_add", "challenge_list"}
	for _, name := range expected {
		if !r.Has(name) {
			t.Errorf("expected tool %q to be registered", name)
		}
	}
}

func TestRegistry_Call_UnknownTool(t *testing.T) {
	r := NewRegistry(nil)
	_, err := r.Call(context.Background(), "nonexistent_tool", json.RawMessage(`{}`))
	if err == nil {
		t.Fatal("expected error for unknown tool, got nil")
	}
	if got := err.Error(); got != "unknown tool: nonexistent_tool" {
		t.Errorf("unexpected error message: %s", got)
	}
}

func TestRegistry_Call_NilDispatcher(t *testing.T) {
	r := NewRegistry(nil)
	// Pick a tool that exists.
	tools := r.List()
	if len(tools) == 0 {
		t.Fatal("no tools registered")
	}
	name := tools[0].Name
	_, err := r.Call(context.Background(), name, json.RawMessage(`{}`))
	if err == nil {
		t.Fatalf("expected error for nil dispatcher calling %s, got nil", name)
	}
	expected := "dispatcher is nil: cannot execute tool " + name
	if got := err.Error(); got != expected {
		t.Errorf("unexpected error message: %q, want %q", got, expected)
	}
}
