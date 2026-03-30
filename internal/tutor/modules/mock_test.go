package modules

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/rishav1305/soul/internal/tutor/store"
)

func openMockTestStore(t *testing.T) *store.Store {
	t.Helper()
	path := filepath.Join(t.TempDir(), "mock_test.db")
	s, err := store.Open(path)
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func newMockModule(t *testing.T) *MockModule {
	t.Helper()
	return &MockModule{store: openMockTestStore(t)}
}

// ---------------------------------------------------------------------------
// MockInterview tests
// ---------------------------------------------------------------------------

func TestMockInterview_Technical(t *testing.T) {
	m := newMockModule(t)

	result, err := m.MockInterview(map[string]interface{}{
		"type": "technical",
	})
	if err != nil {
		t.Fatalf("MockInterview(technical): %v", err)
	}
	if result == nil {
		t.Fatal("MockInterview returned nil")
	}
	if !strings.Contains(result.Summary, "technical") {
		t.Errorf("summary should mention type, got %q", result.Summary)
	}

	data := result.Data.(map[string]interface{})
	if _, ok := data["session"]; !ok {
		t.Error("missing 'session' key")
	}
	questions, ok := data["questions"].([]map[string]string)
	if !ok {
		t.Fatal("questions is not []map[string]string")
	}
	if len(questions) == 0 {
		t.Error("expected questions, got none")
	}
}

func TestMockInterview_Behavioral(t *testing.T) {
	m := newMockModule(t)

	result, err := m.MockInterview(map[string]interface{}{
		"type": "behavioral",
	})
	if err != nil {
		t.Fatalf("MockInterview(behavioral): %v", err)
	}

	data := result.Data.(map[string]interface{})
	questions := data["questions"].([]map[string]string)
	if len(questions) != 5 {
		t.Errorf("expected 5 behavioral questions, got %d", len(questions))
	}
	// All behavioral questions should be STAR type.
	for _, q := range questions {
		if q["type"] != "star" {
			t.Errorf("expected type 'star', got %q", q["type"])
		}
	}
}

func TestMockInterview_FullLoop(t *testing.T) {
	m := newMockModule(t)

	result, err := m.MockInterview(map[string]interface{}{
		"type": "full_loop",
	})
	if err != nil {
		t.Fatalf("MockInterview(full_loop): %v", err)
	}

	data := result.Data.(map[string]interface{})
	questions := data["questions"].([]map[string]string)
	if len(questions) != 5 {
		t.Errorf("expected 5 full-loop questions, got %d", len(questions))
	}

	// Should have mix of types.
	types := make(map[string]int)
	for _, q := range questions {
		types[q["type"]]++
	}
	if types["coding"] == 0 {
		t.Error("full_loop missing coding questions")
	}
	if types["star"] == 0 {
		t.Error("full_loop missing star questions")
	}
}

func TestMockInterview_DefaultType(t *testing.T) {
	m := newMockModule(t)

	// No type specified — should default to "technical".
	result, err := m.MockInterview(map[string]interface{}{})
	if err != nil {
		t.Fatalf("MockInterview(default): %v", err)
	}
	if !strings.Contains(result.Summary, "technical") {
		t.Errorf("expected default type 'technical' in summary, got %q", result.Summary)
	}
}

func TestMockInterview_InvalidType(t *testing.T) {
	m := newMockModule(t)

	_, err := m.MockInterview(map[string]interface{}{
		"type": "invalid_type",
	})
	if err == nil {
		t.Fatal("expected error for invalid type")
	}
	if !strings.Contains(err.Error(), "invalid type") {
		t.Errorf("error should mention 'invalid type', got: %v", err)
	}
}

func TestMockInterview_WithJobDescription(t *testing.T) {
	m := newMockModule(t)

	result, err := m.MockInterview(map[string]interface{}{
		"type":            "technical",
		"job_description": "Senior Go developer with distributed systems experience.",
	})
	if err != nil {
		t.Fatalf("MockInterview(with JD): %v", err)
	}
	if result == nil {
		t.Fatal("MockInterview returned nil")
	}

	data := result.Data.(map[string]interface{})
	if data["session"] == nil {
		t.Error("session should not be nil")
	}
}

// ---------------------------------------------------------------------------
// AnalyzeJD tests
// ---------------------------------------------------------------------------

func TestAnalyzeJD_HappyPath(t *testing.T) {
	m := newMockModule(t)

	result, err := m.AnalyzeJD(map[string]interface{}{
		"text": "Looking for a senior engineer with Python, machine learning, and leadership experience. Must know distributed systems, Kubernetes, and have strong communication skills.",
	})
	if err != nil {
		t.Fatalf("AnalyzeJD: %v", err)
	}
	if result == nil {
		t.Fatal("AnalyzeJD returned nil")
	}

	data := result.Data.(map[string]interface{})

	// Should find skills across all modules.
	skillsFound, ok := data["skillsFound"].(int)
	if !ok || skillsFound == 0 {
		t.Error("expected non-zero skillsFound")
	}

	// Verify readinessPct is between 0-100.
	readiness, ok := data["readinessPct"].(float64)
	if !ok {
		t.Fatal("missing readinessPct")
	}
	if readiness < 0 || readiness > 100 {
		t.Errorf("readinessPct should be 0-100, got %f", readiness)
	}

	// Should have gaps array.
	if _, ok := data["gaps"]; !ok {
		t.Error("missing 'gaps' key")
	}

	// Should have suggestedPlan.
	if _, ok := data["suggestedPlan"]; !ok {
		t.Error("missing 'suggestedPlan' key")
	}
}

func TestAnalyzeJD_EmptyText(t *testing.T) {
	m := newMockModule(t)
	_, err := m.AnalyzeJD(map[string]interface{}{})
	if err == nil {
		t.Fatal("expected error for missing text")
	}
}

func TestAnalyzeJD_NoSkillsFound(t *testing.T) {
	m := newMockModule(t)

	result, err := m.AnalyzeJD(map[string]interface{}{
		"text": "Seeking a gardener with excellent pruning abilities.",
	})
	if err != nil {
		t.Fatalf("AnalyzeJD: %v", err)
	}

	data := result.Data.(map[string]interface{})
	skillsFound := data["skillsFound"].(int)
	if skillsFound != 0 {
		t.Errorf("expected 0 skills for unrelated JD, got %d", skillsFound)
	}
}

func TestAnalyzeJD_MasteredTopics(t *testing.T) {
	m := newMockModule(t)

	// Create a mastered topic that matches a keyword.
	topic, err := m.store.CreateTopic("dsa", "languages", "python", "easy", "")
	if err != nil {
		t.Fatal(err)
	}
	m.store.UpdateTopicStatus(topic.ID, "mastered")

	result, err := m.AnalyzeJD(map[string]interface{}{
		"text": "Requires Python experience.",
	})
	if err != nil {
		t.Fatal(err)
	}

	data := result.Data.(map[string]interface{})
	masteredCount, ok := data["masteredCount"].(int)
	if !ok {
		t.Fatal("missing masteredCount")
	}
	if masteredCount < 1 {
		t.Errorf("expected at least 1 mastered, got %d", masteredCount)
	}
}

func TestAnalyzeJD_DSASkills(t *testing.T) {
	m := newMockModule(t)

	result, err := m.AnalyzeJD(map[string]interface{}{
		"text": "Must know Go, algorithms, data structures, and Docker.",
	})
	if err != nil {
		t.Fatal(err)
	}

	data := result.Data.(map[string]interface{})
	skillsFound := data["skillsFound"].(int)
	if skillsFound < 3 {
		t.Errorf("expected at least 3 DSA skills, got %d", skillsFound)
	}
}

func TestAnalyzeJD_AISkills(t *testing.T) {
	m := newMockModule(t)

	result, err := m.AnalyzeJD(map[string]interface{}{
		"text": "Experience with machine learning, deep learning, and PyTorch required. NLP skills preferred.",
	})
	if err != nil {
		t.Fatal(err)
	}

	data := result.Data.(map[string]interface{})
	skillsFound := data["skillsFound"].(int)
	if skillsFound < 3 {
		t.Errorf("expected at least 3 AI skills, got %d", skillsFound)
	}
}

func TestAnalyzeJD_BehavioralSkills(t *testing.T) {
	m := newMockModule(t)

	result, err := m.AnalyzeJD(map[string]interface{}{
		"text": "Strong leadership and communication skills. Must be great at teamwork and stakeholder management.",
	})
	if err != nil {
		t.Fatal(err)
	}

	data := result.Data.(map[string]interface{})
	skillsFound := data["skillsFound"].(int)
	if skillsFound < 3 {
		t.Errorf("expected at least 3 behavioral skills, got %d", skillsFound)
	}
}

// ---------------------------------------------------------------------------
// generateQuestions tests
// ---------------------------------------------------------------------------

func TestGenerateQuestions_Technical(t *testing.T) {
	questions := generateQuestions("technical")
	if len(questions) != 3 {
		t.Errorf("expected 3 technical questions, got %d", len(questions))
	}
	for _, q := range questions {
		if q["question"] == "" {
			t.Error("question text should not be empty")
		}
	}
}

func TestGenerateQuestions_Behavioral(t *testing.T) {
	questions := generateQuestions("behavioral")
	if len(questions) != 5 {
		t.Errorf("expected 5 behavioral questions, got %d", len(questions))
	}
}

func TestGenerateQuestions_FullLoop(t *testing.T) {
	questions := generateQuestions("full_loop")
	if len(questions) != 5 {
		t.Errorf("expected 5 full_loop questions, got %d", len(questions))
	}
}

func TestGenerateQuestions_UnknownType(t *testing.T) {
	questions := generateQuestions("unknown")
	if questions != nil {
		t.Errorf("expected nil for unknown type, got %v", questions)
	}
}
