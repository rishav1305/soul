package modules

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/rishav1305/soul-v2/internal/tutor/store"
)

func openBehavioralTestStore(t *testing.T) *store.Store {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.db")
	s, err := store.Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func newBehavioralModule(t *testing.T) *BehavioralModule {
	t.Helper()
	return &BehavioralModule{store: openBehavioralTestStore(t)}
}

// ---------------------------------------------------------------------------
// BuildNarrative tests
// ---------------------------------------------------------------------------

func TestBehavioralModule_BuildNarrative_NoFocus(t *testing.T) {
	m := newBehavioralModule(t)
	result, err := m.BuildNarrative(map[string]interface{}{})
	if err != nil {
		t.Fatalf("BuildNarrative(no focus): %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	data, ok := result.Data.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map data, got %T", result.Data)
	}
	if _, ok := data["narrative"]; !ok {
		t.Error("expected 'narrative' key in data")
	}
	narrative, _ := data["narrative"].(string)
	if !strings.Contains(narrative, "Tell Me About Yourself") {
		t.Error("expected narrative to contain 'Tell Me About Yourself'")
	}
}

func TestBehavioralModule_BuildNarrative_WithFocus(t *testing.T) {
	m := newBehavioralModule(t)
	result, err := m.BuildNarrative(map[string]interface{}{"focus": "leadership"})
	if err != nil {
		t.Fatalf("BuildNarrative(focus=leadership): %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	data, ok := result.Data.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map data, got %T", result.Data)
	}
	if data["focus"] != "leadership" {
		t.Errorf("expected focus='leadership', got %v", data["focus"])
	}
}

// ---------------------------------------------------------------------------
// BuildStar tests
// ---------------------------------------------------------------------------

func TestBehavioralModule_BuildStar_NoCompetency(t *testing.T) {
	m := newBehavioralModule(t)
	_, err := m.BuildStar(map[string]interface{}{})
	if err == nil {
		t.Fatal("expected error when competency is missing")
	}
}

func TestBehavioralModule_BuildStar_InvalidCompetency(t *testing.T) {
	m := newBehavioralModule(t)
	_, err := m.BuildStar(map[string]interface{}{"competency": "dancing"})
	if err == nil {
		t.Fatal("expected error for invalid competency")
	}
	if !strings.Contains(err.Error(), "invalid competency") {
		t.Errorf("expected 'invalid competency' in error, got: %v", err)
	}
}

func TestBehavioralModule_BuildStar_ValidCompetency_Template(t *testing.T) {
	m := newBehavioralModule(t)
	result, err := m.BuildStar(map[string]interface{}{"competency": "leadership"})
	if err != nil {
		t.Fatalf("BuildStar(leadership): %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	data, ok := result.Data.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map data, got %T", result.Data)
	}
	// Fresh store — no existing story, should return template
	if data["existing"] != false {
		t.Errorf("expected existing=false for empty store, got %v", data["existing"])
	}
	if _, ok := data["template"]; !ok {
		t.Error("expected 'template' key in data")
	}
}

// ---------------------------------------------------------------------------
// DrillHR tests
// ---------------------------------------------------------------------------

func TestBehavioralModule_DrillHR_NoCategory(t *testing.T) {
	m := newBehavioralModule(t)
	_, err := m.DrillHR(map[string]interface{}{})
	if err == nil {
		t.Fatal("expected error when category is missing")
	}
}

func TestBehavioralModule_DrillHR_InvalidCategory(t *testing.T) {
	m := newBehavioralModule(t)
	_, err := m.DrillHR(map[string]interface{}{"category": "unknown_category"})
	if err == nil {
		t.Fatal("expected error for invalid category")
	}
	if !strings.Contains(err.Error(), "invalid HR category") {
		t.Errorf("expected 'invalid HR category' in error, got: %v", err)
	}
}

func TestBehavioralModule_DrillHR_QuestionMode(t *testing.T) {
	m := newBehavioralModule(t)
	result, err := m.DrillHR(map[string]interface{}{"category": "motivation"})
	if err != nil {
		t.Fatalf("DrillHR(motivation): %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	data, ok := result.Data.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map data, got %T", result.Data)
	}
	if data["mode"] != "question" {
		t.Errorf("expected mode='question', got %v", data["mode"])
	}
	if _, ok := data["question"]; !ok {
		t.Error("expected 'question' key in data")
	}
}

func TestBehavioralModule_DrillHR_EvaluateMode(t *testing.T) {
	m := newBehavioralModule(t)
	answer := "I am motivated by learning new technologies and achieving goals. For example, when I improved system performance by 30 percent, it drove me to keep learning."
	result, err := m.DrillHR(map[string]interface{}{
		"category": "motivation",
		"answer":   answer,
	})
	if err != nil {
		t.Fatalf("DrillHR(evaluate): %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	data, ok := result.Data.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map data, got %T", result.Data)
	}
	if data["mode"] != "result" {
		t.Errorf("expected mode='result', got %v", data["mode"])
	}
	if _, ok := data["score"]; !ok {
		t.Error("expected 'score' key in data")
	}
	if _, ok := data["feedback"]; !ok {
		t.Error("expected 'feedback' key in data")
	}
}
