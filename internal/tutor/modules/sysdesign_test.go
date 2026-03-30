package modules

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rishav1305/soul/internal/tutor/eval"
	"github.com/rishav1305/soul/internal/tutor/store"
)

func openTestStoreForSysDesign(t *testing.T) *store.Store {
	t.Helper()
	path := filepath.Join(t.TempDir(), "sysdesign_test.db")
	s, err := store.Open(path)
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func newSysDesignModule(t *testing.T) *SystemDesignModule {
	t.Helper()
	s := openTestStoreForSysDesign(t)
	return &SystemDesignModule{
		store:     s,
		evaluator: eval.New(nil), // word-overlap fallback
	}
}

// TestSystemDesignLearn verifies that Learn marks a topic as "learning" and returns a framework.
func TestSystemDesignLearn(t *testing.T) {
	m := newSysDesignModule(t)

	// Create a topic first.
	topic, err := m.store.CreateTopic("sysdesign", "distributed", "Consistent Hashing", "hard", "")
	if err != nil {
		t.Fatalf("CreateTopic: %v", err)
	}

	// Call Learn with topic_id.
	result, err := m.Learn(map[string]interface{}{
		"topic_id": float64(topic.ID),
	})
	if err != nil {
		t.Fatalf("Learn: %v", err)
	}
	if result == nil {
		t.Fatal("Learn returned nil result")
	}

	// Verify summary contains topic name.
	if result.Summary == "" {
		t.Error("Learn returned empty summary")
	}

	// Verify data has topic and framework.
	data, ok := result.Data.(map[string]interface{})
	if !ok {
		t.Fatal("Learn data is not map[string]interface{}")
	}
	if _, ok := data["topic"]; !ok {
		t.Error("Learn data missing 'topic' key")
	}
	if _, ok := data["framework"]; !ok {
		t.Error("Learn data missing 'framework' key")
	}

	// Verify topic status was updated to "learning".
	updated, err := m.store.GetTopic(topic.ID)
	if err != nil {
		t.Fatalf("GetTopic after Learn: %v", err)
	}
	if updated.Status != "learning" {
		t.Errorf("expected status 'learning', got %q", updated.Status)
	}
}

// TestSystemDesignLearnByName verifies that Learn resolves a topic by name+category.
func TestSystemDesignLearnByName(t *testing.T) {
	m := newSysDesignModule(t)

	_, err := m.store.CreateTopic("sysdesign", "databases", "Sharding", "hard", "")
	if err != nil {
		t.Fatalf("CreateTopic: %v", err)
	}

	result, err := m.Learn(map[string]interface{}{
		"topic":    "Sharding",
		"category": "databases",
	})
	if err != nil {
		t.Fatalf("Learn by name: %v", err)
	}
	if result == nil {
		t.Fatal("Learn returned nil")
	}
}

// TestSystemDesignDrill tests both start mode (returns question) and answer mode (returns result with score).
func TestSystemDesignDrill(t *testing.T) {
	m := newSysDesignModule(t)

	// Create topic and question.
	topic, err := m.store.CreateTopic("sysdesign", "distributed", "Load Balancer", "hard", "")
	if err != nil {
		t.Fatalf("CreateTopic: %v", err)
	}

	question, err := m.store.CreateQuizQuestion(
		topic.ID,
		"hard",
		"What is a load balancer and why is it used?",
		"A load balancer distributes incoming network traffic across multiple servers to ensure reliability and availability. It prevents any single server from becoming a bottleneck.",
		"Load balancers improve fault tolerance and horizontal scalability.",
		"sysdesign-lb-q1",
	)
	if err != nil {
		t.Fatalf("CreateQuizQuestion: %v", err)
	}

	// Start mode: returns question.
	startResult, err := m.Drill(map[string]interface{}{
		"topic_id": float64(topic.ID),
	})
	if err != nil {
		t.Fatalf("Drill start mode: %v", err)
	}
	if startResult == nil {
		t.Fatal("Drill start mode returned nil")
	}
	startData, ok := startResult.Data.(map[string]interface{})
	if !ok {
		t.Fatal("Drill start data is not map[string]interface{}")
	}
	if startData["mode"] != "question" {
		t.Errorf("expected mode 'question', got %v", startData["mode"])
	}
	if _, ok := startData["question"]; !ok {
		t.Error("Drill start data missing 'question' key")
	}

	// Answer mode: evaluate a correct-ish answer.
	answerResult, err := m.Drill(map[string]interface{}{
		"question_id": float64(question.ID),
		"answer":      "A load balancer distributes traffic across multiple servers for reliability and availability.",
	})
	if err != nil {
		t.Fatalf("Drill answer mode: %v", err)
	}
	if answerResult == nil {
		t.Fatal("Drill answer mode returned nil")
	}
	answerData, ok := answerResult.Data.(map[string]interface{})
	if !ok {
		t.Fatal("Drill answer data is not map[string]interface{}")
	}
	if answerData["mode"] != "result" {
		t.Errorf("expected mode 'result', got %v", answerData["mode"])
	}
	if _, ok := answerData["score"]; !ok {
		t.Error("Drill answer data missing 'score' key")
	}
	if _, ok := answerData["correct"]; !ok {
		t.Error("Drill answer data missing 'correct' key")
	}
	if _, ok := answerData["nextReview"]; !ok {
		t.Error("Drill answer data missing 'nextReview' key")
	}
}

// TestSystemDesignDrillRequiresAnswer verifies that answer mode rejects empty answers.
func TestSystemDesignDrillRequiresAnswer(t *testing.T) {
	m := newSysDesignModule(t)

	topic, err := m.store.CreateTopic("sysdesign", "distributed", "CAP Theorem", "hard", "")
	if err != nil {
		t.Fatalf("CreateTopic: %v", err)
	}

	q, err := m.store.CreateQuizQuestion(topic.ID, "hard", "Explain CAP theorem.", "CAP: Consistency, Availability, Partition tolerance.", "", "")
	if err != nil {
		t.Fatalf("CreateQuizQuestion: %v", err)
	}

	_, err = m.Drill(map[string]interface{}{
		"question_id": float64(q.ID),
		// no "answer"
	})
	if err == nil {
		t.Error("expected error for empty answer, got nil")
	}
}

// TestSystemDesignGenerate verifies that GenerateContent creates a topic with module="sysdesign" and difficulty="hard".
func TestSystemDesignGenerate(t *testing.T) {
	m := newSysDesignModule(t)

	result, err := m.GenerateContent(map[string]interface{}{
		"category": "caching",
		"name":     "Redis Architecture",
	})
	if err != nil {
		t.Fatalf("GenerateContent: %v", err)
	}
	if result == nil {
		t.Fatal("GenerateContent returned nil")
	}
	if result.Summary == "" {
		t.Error("GenerateContent returned empty summary")
	}

	// Verify topic in data.
	data, ok := result.Data.(map[string]interface{})
	if !ok {
		t.Fatal("GenerateContent data is not map[string]interface{}")
	}
	topicRaw, ok := data["topic"]
	if !ok {
		t.Fatal("GenerateContent data missing 'topic' key")
	}

	// The topic should have module="sysdesign" and difficulty="hard".
	topic, ok := topicRaw.(*store.Topic)
	if !ok {
		t.Fatalf("topic is not *store.Topic, got %T", topicRaw)
	}
	if topic.Module != "sysdesign" {
		t.Errorf("expected module 'sysdesign', got %q", topic.Module)
	}
	if topic.Difficulty != "hard" {
		t.Errorf("expected difficulty 'hard', got %q", topic.Difficulty)
	}
	if topic.Category != "caching" {
		t.Errorf("expected category 'caching', got %q", topic.Category)
	}
	if topic.Name != "Redis Architecture" {
		t.Errorf("expected name 'Redis Architecture', got %q", topic.Name)
	}
}

// TestSystemDesignGenerateCustomDifficulty verifies that an explicit difficulty is respected.
func TestSystemDesignGenerateCustomDifficulty(t *testing.T) {
	m := newSysDesignModule(t)

	result, err := m.GenerateContent(map[string]interface{}{
		"category":   "networking",
		"name":       "DNS Resolution",
		"difficulty": "medium",
	})
	if err != nil {
		t.Fatalf("GenerateContent: %v", err)
	}
	data := result.Data.(map[string]interface{})
	topic := data["topic"].(*store.Topic)
	if topic.Difficulty != "medium" {
		t.Errorf("expected difficulty 'medium', got %q", topic.Difficulty)
	}
}

// TestSystemDesignGenerateRequiresFields verifies that missing fields return an error.
func TestSystemDesignGenerateRequiresFields(t *testing.T) {
	m := newSysDesignModule(t)

	_, err := m.GenerateContent(map[string]interface{}{
		"category": "caching",
		// missing "name"
	})
	if err == nil {
		t.Error("expected error for missing 'name', got nil")
	}

	_, err = m.GenerateContent(map[string]interface{}{
		"name": "Some Topic",
		// missing "category"
	})
	if err == nil {
		t.Error("expected error for missing 'category', got nil")
	}
}

// TestSystemDesignEvaluatorFallback verifies word-overlap fallback when evaluator is nil.
func TestSystemDesignEvaluatorFallback(t *testing.T) {
	s := openTestStoreForSysDesign(t)
	// Module with nil evaluator.
	m := &SystemDesignModule{store: s, evaluator: nil}

	topic, err := m.store.CreateTopic("sysdesign", "distributed", "Replication", "hard", "")
	if err != nil {
		t.Fatalf("CreateTopic: %v", err)
	}
	q, err := m.store.CreateQuizQuestion(topic.ID, "hard",
		"What is database replication?",
		"Database replication copies data from one database to another to increase availability and fault tolerance.",
		"", "")
	if err != nil {
		t.Fatalf("CreateQuizQuestion: %v", err)
	}

	result, err := m.Drill(map[string]interface{}{
		"question_id": float64(q.ID),
		"answer":      "Replication copies data across databases for availability.",
	})
	if err != nil {
		t.Fatalf("Drill with nil evaluator: %v", err)
	}
	if result == nil {
		t.Fatal("Drill returned nil")
	}
	data := result.Data.(map[string]interface{})
	if data["mode"] != "result" {
		t.Errorf("expected mode 'result', got %v", data["mode"])
	}
}

// --- Template functions ---

func TestSysDesignContentTemplate(t *testing.T) {
	content := sysDesignContentTemplate("Load Balancer")
	if content == "" {
		t.Fatal("sysDesignContentTemplate returned empty string")
	}
	if !strings.Contains(content, "# Load Balancer") {
		t.Error("template missing topic heading")
	}
	if !strings.Contains(content, "Key Components") {
		t.Error("template missing Key Components section")
	}
	if !strings.Contains(content, "Scalability") {
		t.Error("template missing Scalability section")
	}
}

func TestWriteSysDesignContent_HappyPath(t *testing.T) {
	contentDir := t.TempDir()
	relPath := writeSysDesignContent(contentDir, "distributed", "Load Balancer")
	if relPath == "" {
		t.Fatal("writeSysDesignContent returned empty path")
	}
	expected := filepath.Join("sysdesign", "distributed", "Load Balancer.md")
	if relPath != expected {
		t.Errorf("relPath = %q, want %q", relPath, expected)
	}

	// Verify file exists and has content.
	fullPath := filepath.Join(contentDir, relPath)
	data, err := os.ReadFile(fullPath)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if len(data) == 0 {
		t.Error("written file is empty")
	}
	if !strings.Contains(string(data), "# Load Balancer") {
		t.Error("file content missing heading")
	}
}

func TestWriteSysDesignContent_EmptyContentDir(t *testing.T) {
	relPath := writeSysDesignContent("", "distributed", "LB")
	if relPath != "" {
		t.Errorf("expected empty path for empty contentDir, got %q", relPath)
	}
}

func TestGenerateSysDesignFramework(t *testing.T) {
	framework := generateSysDesignFramework("URL Shortener")
	if framework == "" {
		t.Fatal("generateSysDesignFramework returned empty")
	}
	if !strings.Contains(framework, "URL Shortener") {
		t.Error("framework missing topic name")
	}
	if !strings.Contains(framework, "Requirements Clarification") {
		t.Error("framework missing Requirements section")
	}
	if !strings.Contains(framework, "Trade-offs") {
		t.Error("framework missing Trade-offs section")
	}
	if !strings.Contains(framework, "Interview Tips") {
		t.Error("framework missing Interview Tips section")
	}
}

// TestSystemDesignResolveTopicBySearch verifies name-only resolution (no category).
func TestSystemDesignResolveTopicBySearch(t *testing.T) {
	m := newSysDesignModule(t)

	// Create topic without category match.
	_, err := m.store.CreateTopic("sysdesign", "storage", "HDFS", "hard", "")
	if err != nil {
		t.Fatalf("CreateTopic: %v", err)
	}

	// Resolve by name only (no category, no topic_id).
	result, err := m.Learn(map[string]interface{}{
		"topic": "HDFS",
	})
	if err != nil {
		t.Fatalf("Learn by name-only: %v", err)
	}
	if result == nil {
		t.Fatal("Learn returned nil")
	}
}

// TestSystemDesignResolveTopicNotFound verifies error for unknown topic.
func TestSystemDesignResolveTopicNotFound(t *testing.T) {
	m := newSysDesignModule(t)

	_, err := m.Learn(map[string]interface{}{
		"topic": "Nonexistent Topic",
	})
	if err == nil {
		t.Fatal("expected error for unknown topic")
	}
}

// TestSystemDesignResolveTopicMissing verifies error for no identifiers.
func TestSystemDesignResolveTopicMissing(t *testing.T) {
	m := newSysDesignModule(t)
	_, err := m.Learn(map[string]interface{}{})
	if err == nil {
		t.Fatal("expected error for empty input")
	}
}
