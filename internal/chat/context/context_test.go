package context

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestForProduct_ReturnsCorrectContext(t *testing.T) {
	products := []string{
		"tasks", "tutor", "projects", "observe",
		"devops", "dba", "migrate",
		"compliance", "qa", "analytics",
		"dataeng", "costops", "viz",
		"docs", "api",
		"sentinel", "bench", "mesh", "scout",
	}
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
	if len(ctx.Tools) != len(builtinTools()) {
		t.Errorf("default context should have %d built-in tools, got %d", len(builtinTools()), len(ctx.Tools))
	}
}

func TestForProduct_EmptyReturnsDefault(t *testing.T) {
	ctx := ForProduct("")
	if len(ctx.Tools) != len(builtinTools()) {
		t.Errorf("empty product should return default context with %d built-in tools, got %d", len(builtinTools()), len(ctx.Tools))
	}
}

func TestToolCounts(t *testing.T) {
	// Each product gets its product-specific tools + 8 built-in tools.
	// tasks=6+8=14, tutor=7+8=15, projects=6+8=14, observe=4+8=12
	// infra(devops/dba/migrate)=6+8=14, quality(compliance/qa/analytics)=8+8=16
	// dataprod(dataeng/costops/viz)=6+8=14, docsprod(docs/api)=4+8=12
	// sentinel=7+8=15, bench=4+8=12, mesh=4+8=12, scout=55+8=63
	expected := map[string]int{
		"tasks": 14, "tutor": 15, "projects": 14, "observe": 12,
		"devops": 14, "dba": 14, "migrate": 14,
		"compliance": 16, "qa": 16, "analytics": 16,
		"dataeng": 14, "costops": 14, "viz": 14,
		"docs": 12, "api": 12,
		"sentinel": 15, "bench": 12, "mesh": 12, "scout": 63,
	}
	for product, count := range expected {
		ctx := ForProduct(product)
		if len(ctx.Tools) != count {
			t.Errorf("%s: expected %d tools, got %d", product, count, len(ctx.Tools))
		}
	}
}

func TestDispatcher_UnknownTool(t *testing.T) {
	d := NewDispatcher()
	_, err := d.Execute(context.Background(), "nonexistent_tool", nil)
	if err == nil || !strings.Contains(err.Error(), "unknown tool") {
		t.Errorf("expected unknown tool error, got: %v", err)
	}
}

func TestDispatcher_RoutesExist(t *testing.T) {
	d := NewDispatcher()
	// Build set of built-in tool names to skip.
	builtinSet := make(map[string]bool)
	for _, tool := range builtinTools() {
		builtinSet[tool.Name] = true
	}
	// Every product-specific tool should have a matching dispatcher route.
	for _, product := range []string{
		"tasks", "tutor", "projects", "observe",
		"devops", "dba", "migrate",
		"compliance", "qa", "analytics",
		"dataeng", "costops", "viz",
		"docs", "api",
		"sentinel", "bench", "mesh", "scout",
	} {
		ctx := ForProduct(product)
		for _, tool := range ctx.Tools {
			if builtinSet[tool.Name] {
				continue // built-in tools are dispatched separately
			}
			if _, ok := d.routes[tool.Name]; !ok {
				t.Errorf("tool %q (product %s) has no dispatcher route", tool.Name, product)
			}
		}
	}
}
