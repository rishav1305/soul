package modules

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/rishav1305/soul/internal/tutor/store"
)

func openModuleTestStore(t *testing.T) *store.Store {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.db")
	s, err := store.Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

// ---------------------------------------------------------------------------
// Registry tests
// ---------------------------------------------------------------------------

func TestNewRegistry(t *testing.T) {
	s := openModuleTestStore(t)
	reg := NewRegistry(s, "/tmp/content", nil)

	if reg == nil {
		t.Fatal("expected non-nil Registry")
	}
	if reg.DSA == nil {
		t.Error("DSA module is nil")
	}
	if reg.AI == nil {
		t.Error("AI module is nil")
	}
	if reg.Behavioral == nil {
		t.Error("Behavioral module is nil")
	}
	if reg.Mock == nil {
		t.Error("Mock module is nil")
	}
	if reg.Planner == nil {
		t.Error("Planner module is nil")
	}
	if reg.Progress == nil {
		t.Error("Progress module is nil")
	}
	if reg.SystemDesign == nil {
		t.Error("SystemDesign module is nil")
	}
}

// ---------------------------------------------------------------------------
// ProgressModule tests
// ---------------------------------------------------------------------------

func newProgressModule(t *testing.T) *ProgressModule {
	t.Helper()
	s := openModuleTestStore(t)
	return &ProgressModule{store: s}
}

func TestProgressModule_DefaultView(t *testing.T) {
	pm := newProgressModule(t)

	result, err := pm.Progress(map[string]interface{}{})
	if err != nil {
		t.Fatalf("Progress({}): unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	// Default view is dashboard — summary should mention "dashboard"
	if !strings.Contains(strings.ToLower(result.Summary), "dashboard") {
		t.Errorf("expected Summary to contain 'dashboard', got: %q", result.Summary)
	}
}

func TestProgressModule_DashboardView(t *testing.T) {
	pm := newProgressModule(t)

	result, err := pm.Progress(map[string]interface{}{"view": "dashboard"})
	if err != nil {
		t.Fatalf("Progress(dashboard): %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}

	data, ok := result.Data.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map data, got %T", result.Data)
	}
	if _, ok := data["readinessPct"]; !ok {
		t.Error("expected 'readinessPct' key in data")
	}

	// Summary should contain "readiness"
	if !strings.Contains(strings.ToLower(result.Summary), "readiness") {
		t.Errorf("expected Summary to contain 'readiness', got: %q", result.Summary)
	}
}

func TestProgressModule_AnalyticsView(t *testing.T) {
	pm := newProgressModule(t)

	result, err := pm.Progress(map[string]interface{}{"view": "analytics"})
	if err != nil {
		t.Fatalf("Progress(analytics): %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}

	data, ok := result.Data.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map data, got %T", result.Data)
	}
	if _, ok := data["last30Days"]; !ok {
		t.Error("expected 'last30Days' key in analytics data")
	}
}

func TestProgressModule_TopicsView(t *testing.T) {
	pm := newProgressModule(t)

	result, err := pm.Progress(map[string]interface{}{"view": "topics"})
	if err != nil {
		t.Fatalf("Progress(topics): %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}

	data, ok := result.Data.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map data, got %T", result.Data)
	}
	if _, ok := data["topics"]; !ok {
		t.Error("expected 'topics' key in data")
	}
}

func TestProgressModule_TopicsWithModule(t *testing.T) {
	pm := newProgressModule(t)

	result, err := pm.Progress(map[string]interface{}{"view": "topics", "module": "dsa"})
	if err != nil {
		t.Fatalf("Progress(topics, module=dsa): %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}

	data, ok := result.Data.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map data, got %T", result.Data)
	}
	modVal, ok := data["module"]
	if !ok {
		t.Fatal("expected 'module' key in data")
	}
	if modVal != "dsa" {
		t.Errorf("expected module='dsa', got %v", modVal)
	}
}

func TestProgressModule_MocksView(t *testing.T) {
	pm := newProgressModule(t)

	result, err := pm.Progress(map[string]interface{}{"view": "mocks"})
	if err != nil {
		t.Fatalf("Progress(mocks): %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}

	data, ok := result.Data.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map data, got %T", result.Data)
	}
	if _, ok := data["sessions"]; !ok {
		t.Error("expected 'sessions' key in mocks data")
	}
}

func TestProgressModule_InvalidView(t *testing.T) {
	pm := newProgressModule(t)

	result, err := pm.Progress(map[string]interface{}{"view": "invalid"})
	if err == nil {
		t.Fatal("expected error for invalid view, got nil")
	}
	if result != nil {
		t.Error("expected nil result on error")
	}
	if !strings.Contains(err.Error(), "invalid view") {
		t.Errorf("expected error to contain 'invalid view', got: %v", err)
	}
}
