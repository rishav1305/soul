package context

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestDispatcher_Execute_GET(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Errorf("expected GET, got %s", r.Method)
		}
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"tasks":[]}`)
	}))
	defer srv.Close()

	d := NewDispatcher()
	d.urls["tasks"] = srv.URL
	result, err := d.Execute(context.Background(), "list_tasks", nil)
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	if !strings.Contains(result, "tasks") {
		t.Errorf("expected tasks in response, got: %s", result)
	}
}

func TestDispatcher_Execute_POST(t *testing.T) {
	var receivedBody map[string]interface{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("expected json content type, got %s", r.Header.Get("Content-Type"))
		}
		json.NewDecoder(r.Body).Decode(&receivedBody)
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"id":42}`)
	}))
	defer srv.Close()

	d := NewDispatcher()
	d.urls["tasks"] = srv.URL
	input := json.RawMessage(`{"title":"test task","description":"test"}`)
	result, err := d.Execute(context.Background(), "create_task", input)
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	if !strings.Contains(result, "42") {
		t.Errorf("expected id 42 in response, got: %s", result)
	}
	if receivedBody["title"] != "test task" {
		t.Errorf("expected title 'test task' in body, got: %v", receivedBody["title"])
	}
}

func TestDispatcher_Execute_PathParams(t *testing.T) {
	var requestPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"status":"ok"}`)
	}))
	defer srv.Close()

	d := NewDispatcher()
	d.urls["tasks"] = srv.URL
	input := json.RawMessage(`{"task_id":99}`)
	_, err := d.Execute(context.Background(), "get_task", input)
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	if requestPath != "/api/tasks/99" {
		t.Errorf("expected path /api/tasks/99, got %s", requestPath)
	}
}

func TestDispatcher_Execute_GETQueryParams(t *testing.T) {
	var queryValues string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		queryValues = r.URL.RawQuery
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `[]`)
	}))
	defer srv.Close()

	d := NewDispatcher()
	d.urls["tutor"] = srv.URL
	input := json.RawMessage(`{"module":"dsa"}`)
	_, err := d.Execute(context.Background(), "list_topics", input)
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	if !strings.Contains(queryValues, "module=dsa") {
		t.Errorf("expected module=dsa in query, got %s", queryValues)
	}
}

func TestDispatcher_Execute_PATCH(t *testing.T) {
	var receivedMethod string
	var requestPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedMethod = r.Method
		requestPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"updated":true}`)
	}))
	defer srv.Close()

	d := NewDispatcher()
	d.urls["tasks"] = srv.URL
	input := json.RawMessage(`{"task_id":5,"status":"done"}`)
	_, err := d.Execute(context.Background(), "update_task", input)
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	if receivedMethod != "PATCH" {
		t.Errorf("expected PATCH, got %s", receivedMethod)
	}
	if requestPath != "/api/tasks/5" {
		t.Errorf("expected path /api/tasks/5, got %s", requestPath)
	}
}

func TestDispatcher_Execute_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprint(w, `{"error":"not found"}`)
	}))
	defer srv.Close()

	d := NewDispatcher()
	d.urls["tasks"] = srv.URL
	result, err := d.Execute(context.Background(), "list_tasks", nil)
	if err != nil {
		t.Fatalf("Execute should not return Go error for HTTP errors, got: %v", err)
	}
	if !strings.Contains(result, "Error from list_tasks") {
		t.Errorf("expected error message in result, got: %s", result)
	}
	if !strings.Contains(result, "404") {
		t.Errorf("expected HTTP 404 in result, got: %s", result)
	}
}

func TestDispatcher_Execute_NetworkError(t *testing.T) {
	d := NewDispatcher()
	// Point to a closed server — connection refused.
	d.urls["tasks"] = "http://127.0.0.1:1" // port 1 is typically unavailable
	result, err := d.Execute(context.Background(), "list_tasks", nil)
	if err != nil {
		t.Fatalf("Execute should not return Go error for network errors, got: %v", err)
	}
	if !strings.Contains(result, "Error calling list_tasks") {
		t.Errorf("expected network error message, got: %s", result)
	}
}

func TestDispatcher_Execute_Truncation(t *testing.T) {
	// Create a response larger than maxToolResultBytes.
	largeBody := strings.Repeat("x", maxToolResultBytes+100)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, largeBody)
	}))
	defer srv.Close()

	d := NewDispatcher()
	d.urls["tasks"] = srv.URL
	result, err := d.Execute(context.Background(), "list_tasks", nil)
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	if !strings.HasSuffix(result, "...(truncated)") {
		t.Error("expected truncation marker in response")
	}
	if len(result) > maxToolResultBytes+50 {
		t.Errorf("result should be capped near maxToolResultBytes, got %d bytes", len(result))
	}
}

func TestDispatcher_Execute_InvalidInput(t *testing.T) {
	d := NewDispatcher()
	_, err := d.Execute(context.Background(), "list_tasks", json.RawMessage(`{invalid`))
	if err == nil {
		t.Error("expected error for invalid JSON input")
	}
	if !strings.Contains(err.Error(), "invalid tool input") {
		t.Errorf("expected 'invalid tool input' error, got: %v", err)
	}
}

func TestDispatcher_Execute_NilInput(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"ok":true}`)
	}))
	defer srv.Close()

	d := NewDispatcher()
	d.urls["tasks"] = srv.URL
	result, err := d.Execute(context.Background(), "list_tasks", nil)
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	if !strings.Contains(result, "ok") {
		t.Errorf("expected ok in response, got: %s", result)
	}
}

func TestDispatcher_Execute_EmptyInput(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{}`)
	}))
	defer srv.Close()

	d := NewDispatcher()
	d.urls["tasks"] = srv.URL
	result, err := d.Execute(context.Background(), "list_tasks", json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	if result != "{}" {
		t.Errorf("expected empty JSON, got: %s", result)
	}
}

func TestDispatcher_Execute_Timeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	d := NewDispatcher()
	d.urls["tasks"] = srv.URL
	// Override route to have a very short timeout.
	d.routes["list_tasks"] = ToolRoute{Method: "GET", Path: "/api/tasks", Product: "tasks", Timeout: 50 * time.Millisecond}

	result, err := d.Execute(context.Background(), "list_tasks", nil)
	if err != nil {
		t.Fatalf("Execute should not return Go error for timeouts, got: %v", err)
	}
	if !strings.Contains(result, "Error calling list_tasks") {
		t.Errorf("expected timeout error message, got: %s", result)
	}
}

func TestDispatcher_Execute_UnknownProduct(t *testing.T) {
	d := NewDispatcher()
	d.routes["fake_tool"] = ToolRoute{Method: "GET", Path: "/api/fake", Product: "nonexistent"}
	_, err := d.Execute(context.Background(), "fake_tool", nil)
	if err == nil || !strings.Contains(err.Error(), "no URL configured") {
		t.Errorf("expected 'no URL configured' error, got: %v", err)
	}
}

func TestDispatcher_Execute_ContextCancellation(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(5 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	d := NewDispatcher()
	d.urls["tasks"] = srv.URL

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately.

	result, err := d.Execute(ctx, "list_tasks", nil)
	if err != nil {
		t.Fatalf("Execute should not return Go error for cancelled context, got: %v", err)
	}
	if !strings.Contains(result, "Error calling") {
		t.Errorf("expected error message for cancelled context, got: %s", result)
	}
}

func TestDispatcher_Execute_MultiplePathParams(t *testing.T) {
	var requestPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"ok":true}`)
	}))
	defer srv.Close()

	d := NewDispatcher()
	d.urls["projects"] = srv.URL
	input := json.RawMessage(`{"project_id":10,"milestone_id":3}`)
	_, err := d.Execute(context.Background(), "update_milestone", input)
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	if requestPath != "/api/projects/10/milestones/3" {
		t.Errorf("expected /api/projects/10/milestones/3, got %s", requestPath)
	}
}

func TestNewDispatcher_AllProductsHaveURLs(t *testing.T) {
	d := NewDispatcher()
	// Every route's product should have a URL.
	for name, route := range d.routes {
		if _, ok := d.urls[route.Product]; !ok {
			t.Errorf("route %q references product %q which has no URL", name, route.Product)
		}
	}
}

func TestNewDispatcher_RouteCount(t *testing.T) {
	d := NewDispatcher()
	// Should have a substantial number of routes (93+ product tools).
	if len(d.routes) < 90 {
		t.Errorf("expected at least 90 routes, got %d", len(d.routes))
	}
}

func TestEnvOr(t *testing.T) {
	// Test with unset variable — should return fallback.
	result := envOr("SOUL_TEST_UNSET_VAR_XYZ", "fallback")
	if result != "fallback" {
		t.Errorf("expected fallback, got %s", result)
	}

	// Test with set variable.
	t.Setenv("SOUL_TEST_SET_VAR_XYZ", "custom")
	result = envOr("SOUL_TEST_SET_VAR_XYZ", "fallback")
	if result != "custom" {
		t.Errorf("expected custom, got %s", result)
	}
}

func TestDefault_HasBuiltinTools(t *testing.T) {
	ctx := Default()
	if len(ctx.Tools) != 8 {
		t.Errorf("expected 8 built-in tools, got %d", len(ctx.Tools))
	}
	if ctx.System == "" {
		t.Error("default context should have a system prompt")
	}
	if !strings.Contains(ctx.System, "Soul") {
		t.Error("default system prompt should mention Soul")
	}
	if !strings.Contains(ctx.System, "memory_store") {
		t.Error("default system prompt should mention memory tools")
	}
}

func TestBuiltinTools_Names(t *testing.T) {
	tools := builtinTools()
	expected := []string{
		"memory_store", "memory_search", "memory_list", "memory_delete",
		"tool_create", "tool_list", "tool_delete",
		"subagent",
	}
	if len(tools) != len(expected) {
		t.Fatalf("expected %d built-in tools, got %d", len(expected), len(tools))
	}
	for i, tool := range tools {
		if tool.Name != expected[i] {
			t.Errorf("tool[%d]: expected %q, got %q", i, expected[i], tool.Name)
		}
		if tool.Description == "" {
			t.Errorf("tool %q has empty description", tool.Name)
		}
		if !json.Valid(tool.InputSchema) {
			t.Errorf("tool %q has invalid InputSchema", tool.Name)
		}
	}
}

func TestForProduct_IncludesBuiltins(t *testing.T) {
	// Every product context should include all 8 built-in tools.
	products := []string{"tasks", "tutor", "scout", "sentinel", "mesh"}
	builtinNames := make(map[string]bool)
	for _, bt := range builtinTools() {
		builtinNames[bt.Name] = true
	}

	for _, p := range products {
		ctx := ForProduct(p)
		foundBuiltins := 0
		for _, tool := range ctx.Tools {
			if builtinNames[tool.Name] {
				foundBuiltins++
			}
		}
		if foundBuiltins != 8 {
			t.Errorf("ForProduct(%q): expected 8 built-in tools, found %d", p, foundBuiltins)
		}
	}
}

func TestForProduct_SystemPromptIncludesMemory(t *testing.T) {
	products := []string{"tasks", "tutor", "scout", "projects"}
	for _, p := range products {
		ctx := ForProduct(p)
		if !strings.Contains(ctx.System, "memory_store") {
			t.Errorf("ForProduct(%q): system prompt missing memory instructions", p)
		}
	}
}
