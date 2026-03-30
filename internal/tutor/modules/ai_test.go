package modules

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/rishav1305/soul/internal/tutor/eval"
	"github.com/rishav1305/soul/internal/tutor/store"
)

func openAITestStore(t *testing.T) *store.Store {
	t.Helper()
	path := filepath.Join(t.TempDir(), "ai_test.db")
	s, err := store.Open(path)
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func newAIModule(t *testing.T) *AIModule {
	t.Helper()
	s := openAITestStore(t)
	return &AIModule{
		store:      s,
		contentDir: t.TempDir(),
		evaluator:  eval.New(nil), // word-overlap fallback
	}
}

// ---------------------------------------------------------------------------
// LearnTheory tests
// ---------------------------------------------------------------------------

func TestAILearnTheory_HappyPath(t *testing.T) {
	m := newAIModule(t)

	result, err := m.LearnTheory(map[string]interface{}{
		"topic": "Backpropagation",
	})
	if err != nil {
		t.Fatalf("LearnTheory: %v", err)
	}
	if result == nil {
		t.Fatal("LearnTheory returned nil")
	}
	if !strings.Contains(result.Summary, "Backpropagation") {
		t.Errorf("summary should contain topic name, got %q", result.Summary)
	}

	data := result.Data.(map[string]interface{})
	if _, ok := data["topic"]; !ok {
		t.Error("missing 'topic' key")
	}
	if data["depth"] != "overview" {
		t.Errorf("expected default depth 'overview', got %v", data["depth"])
	}
	content, ok := data["content"].(string)
	if !ok || content == "" {
		t.Error("expected non-empty content")
	}
}

func TestAILearnTheory_DeepDepth(t *testing.T) {
	m := newAIModule(t)

	result, err := m.LearnTheory(map[string]interface{}{
		"topic": "Transformer Architecture",
		"depth": "deep",
	})
	if err != nil {
		t.Fatalf("LearnTheory(deep): %v", err)
	}

	data := result.Data.(map[string]interface{})
	if data["depth"] != "deep" {
		t.Errorf("expected depth 'deep', got %v", data["depth"])
	}
	content := data["content"].(string)
	if !strings.Contains(content, "Deep Dive") {
		t.Error("deep content should contain 'Deep Dive'")
	}
}

func TestAILearnTheory_DetailedDepth(t *testing.T) {
	m := newAIModule(t)

	result, err := m.LearnTheory(map[string]interface{}{
		"topic": "Gradient Descent",
		"depth": "detailed",
	})
	if err != nil {
		t.Fatalf("LearnTheory(detailed): %v", err)
	}

	data := result.Data.(map[string]interface{})
	content := data["content"].(string)
	if !strings.Contains(content, "Detailed Overview") {
		t.Error("detailed content should contain 'Detailed Overview'")
	}
}

func TestAILearnTheory_CustomCategory(t *testing.T) {
	m := newAIModule(t)

	result, err := m.LearnTheory(map[string]interface{}{
		"topic":    "CNN",
		"category": "deep_learning",
	})
	if err != nil {
		t.Fatalf("LearnTheory(custom category): %v", err)
	}
	if result == nil {
		t.Fatal("returned nil")
	}
}

func TestAILearnTheory_AutoCreatesTopic(t *testing.T) {
	m := newAIModule(t)

	// Topic doesn't exist yet — should be auto-created.
	result, err := m.LearnTheory(map[string]interface{}{
		"topic": "Attention Mechanism",
	})
	if err != nil {
		t.Fatalf("LearnTheory auto-create: %v", err)
	}

	data := result.Data.(map[string]interface{})
	topic, ok := data["topic"].(*store.Topic)
	if !ok {
		t.Fatalf("topic is %T, want *store.Topic", data["topic"])
	}
	if topic.Module != "ai" {
		t.Errorf("expected module 'ai', got %q", topic.Module)
	}

	// Verify topic is marked as learning.
	updated, _ := m.store.GetTopic(topic.ID)
	if updated.Status != "learning" {
		t.Errorf("expected status 'learning', got %q", updated.Status)
	}
}

func TestAILearnTheory_ExistingTopic(t *testing.T) {
	m := newAIModule(t)

	// Pre-create topic.
	created, err := m.store.CreateTopic("ai", "theory", "RNN", "medium", "")
	if err != nil {
		t.Fatal(err)
	}

	result, err := m.LearnTheory(map[string]interface{}{
		"topic":    "RNN",
		"category": "theory",
	})
	if err != nil {
		t.Fatalf("LearnTheory existing: %v", err)
	}

	data := result.Data.(map[string]interface{})
	topic := data["topic"].(*store.Topic)
	if topic.ID != created.ID {
		t.Errorf("expected same topic ID %d, got %d", created.ID, topic.ID)
	}
}

func TestAILearnTheory_MissingTopic(t *testing.T) {
	m := newAIModule(t)
	_, err := m.LearnTheory(map[string]interface{}{})
	if err == nil {
		t.Fatal("expected error for missing topic")
	}
}

// ---------------------------------------------------------------------------
// DrillTheory tests
// ---------------------------------------------------------------------------

func TestAIDrillTheory_StartMode(t *testing.T) {
	m := newAIModule(t)

	topic, err := m.store.CreateTopic("ai", "theory", "Bias-Variance", "medium", "")
	if err != nil {
		t.Fatal(err)
	}
	_, err = m.store.CreateQuizQuestion(topic.ID, "medium",
		"What is the bias-variance tradeoff?",
		"Bias-variance tradeoff is the balance between model complexity and generalization. High bias underfits, high variance overfits.",
		"", "")
	if err != nil {
		t.Fatal(err)
	}

	result, err := m.DrillTheory(map[string]interface{}{
		"topic_id": float64(topic.ID),
	})
	if err != nil {
		t.Fatalf("DrillTheory start: %v", err)
	}

	data := result.Data.(map[string]interface{})
	if data["mode"] != "question" {
		t.Errorf("expected mode 'question', got %v", data["mode"])
	}
}

func TestAIDrillTheory_AnswerMode_Correct(t *testing.T) {
	m := newAIModule(t)

	topic, err := m.store.CreateTopic("ai", "theory", "Regularization", "medium", "")
	if err != nil {
		t.Fatal(err)
	}
	q, err := m.store.CreateQuizQuestion(topic.ID, "medium",
		"What is regularization?",
		"Regularization adds a penalty term to the loss function to prevent overfitting by constraining model complexity.",
		"", "")
	if err != nil {
		t.Fatal(err)
	}

	result, err := m.DrillTheory(map[string]interface{}{
		"question_id": float64(q.ID),
		"answer":      "Regularization adds a penalty to the loss function to prevent overfitting and constrains model complexity.",
	})
	if err != nil {
		t.Fatalf("DrillTheory answer: %v", err)
	}

	data := result.Data.(map[string]interface{})
	if data["mode"] != "result" {
		t.Errorf("expected mode 'result', got %v", data["mode"])
	}
	if _, ok := data["score"]; !ok {
		t.Error("missing 'score'")
	}
	if _, ok := data["nextReview"]; !ok {
		t.Error("missing 'nextReview'")
	}
}

func TestAIDrillTheory_AnswerMode_Incorrect(t *testing.T) {
	m := newAIModule(t)

	topic, err := m.store.CreateTopic("ai", "theory", "Dropout", "medium", "")
	if err != nil {
		t.Fatal(err)
	}
	q, err := m.store.CreateQuizQuestion(topic.ID, "medium",
		"What is dropout?",
		"Dropout randomly deactivates neurons during training to prevent overfitting and improve generalization.",
		"", "")
	if err != nil {
		t.Fatal(err)
	}

	result, err := m.DrillTheory(map[string]interface{}{
		"question_id": float64(q.ID),
		"answer":      "Dropout is a type of database operation.",
	})
	if err != nil {
		t.Fatal(err)
	}

	data := result.Data.(map[string]interface{})
	if data["status"] != "incorrect" {
		t.Errorf("expected 'incorrect', got %v", data["status"])
	}
}

func TestAIDrillTheory_EmptyAnswer(t *testing.T) {
	m := newAIModule(t)

	topic, err := m.store.CreateTopic("ai", "theory", "BatchNorm", "medium", "")
	if err != nil {
		t.Fatal(err)
	}
	q, err := m.store.CreateQuizQuestion(topic.ID, "medium",
		"What is batch normalization?",
		"Batch normalization normalizes layer inputs during training.",
		"", "")
	if err != nil {
		t.Fatal(err)
	}

	_, err = m.DrillTheory(map[string]interface{}{
		"question_id": float64(q.ID),
		// missing answer
	})
	if err == nil {
		t.Fatal("expected error for empty answer")
	}
}

func TestAIDrillTheory_MissingInput(t *testing.T) {
	m := newAIModule(t)
	_, err := m.DrillTheory(map[string]interface{}{})
	if err == nil {
		t.Fatal("expected error for missing input")
	}
}

func TestAIDrillTheory_NilEvaluator(t *testing.T) {
	s := openAITestStore(t)
	m := &AIModule{store: s, evaluator: nil}

	topic, err := m.store.CreateTopic("ai", "theory", "GAN", "hard", "")
	if err != nil {
		t.Fatal(err)
	}
	q, err := m.store.CreateQuizQuestion(topic.ID, "hard",
		"What is a GAN?",
		"A GAN is a generative adversarial network with a generator and discriminator competing against each other.",
		"", "")
	if err != nil {
		t.Fatal(err)
	}

	result, err := m.DrillTheory(map[string]interface{}{
		"question_id": float64(q.ID),
		"answer":      "A GAN has a generator and discriminator that compete against each other in a generative adversarial network.",
	})
	if err != nil {
		t.Fatalf("DrillTheory nil evaluator: %v", err)
	}

	data := result.Data.(map[string]interface{})
	if data["mode"] != "result" {
		t.Errorf("expected mode 'result', got %v", data["mode"])
	}
}

// ---------------------------------------------------------------------------
// GenerateAIContent tests
// ---------------------------------------------------------------------------

func TestAIGenerateContent_HappyPath(t *testing.T) {
	m := newAIModule(t)

	result, err := m.GenerateAIContent(map[string]interface{}{
		"category": "deep_learning",
		"name":     "Transformer",
	})
	if err != nil {
		t.Fatalf("GenerateAIContent: %v", err)
	}
	if result == nil {
		t.Fatal("GenerateAIContent returned nil")
	}

	data := result.Data.(map[string]interface{})
	outline, ok := data["outline"].(string)
	if !ok || outline == "" {
		t.Fatal("missing or empty outline")
	}
	if !strings.Contains(outline, "Transformer") {
		t.Error("outline missing topic name")
	}
	if !strings.Contains(outline, "Mathematical Foundations") {
		t.Error("outline missing Mathematical Foundations section")
	}
	if !strings.Contains(outline, "Interview Relevance") {
		t.Error("outline missing Interview Relevance section")
	}
}

func TestAIGenerateContent_MissingCategory(t *testing.T) {
	m := newAIModule(t)
	_, err := m.GenerateAIContent(map[string]interface{}{
		"name": "Topic",
	})
	if err == nil {
		t.Fatal("expected error for missing category")
	}
}

func TestAIGenerateContent_MissingName(t *testing.T) {
	m := newAIModule(t)
	_, err := m.GenerateAIContent(map[string]interface{}{
		"category": "nlp",
	})
	if err == nil {
		t.Fatal("expected error for missing name")
	}
}

// ---------------------------------------------------------------------------
// generateTheoryContent tests
// ---------------------------------------------------------------------------

func TestGenerateTheoryContent_Overview(t *testing.T) {
	content := generateTheoryContent("Neural Networks", "overview")
	if content == "" {
		t.Fatal("returned empty string")
	}
	if !strings.Contains(content, "Neural Networks") {
		t.Error("missing topic name")
	}
	if !strings.Contains(content, "Overview") {
		t.Error("missing Overview heading")
	}
}

func TestGenerateTheoryContent_Detailed(t *testing.T) {
	content := generateTheoryContent("Attention", "detailed")
	if !strings.Contains(content, "Detailed Overview") {
		t.Error("missing Detailed Overview heading")
	}
	if !strings.Contains(content, "Implementation Guide") {
		t.Error("missing Implementation Guide section")
	}
}

func TestGenerateTheoryContent_Deep(t *testing.T) {
	content := generateTheoryContent("LSTM", "deep")
	if !strings.Contains(content, "Deep Dive") {
		t.Error("missing Deep Dive heading")
	}
	if !strings.Contains(content, "Research Frontiers") {
		t.Error("missing Research Frontiers section")
	}
	if !strings.Contains(content, "Common Pitfalls") {
		t.Error("missing Common Pitfalls section")
	}
}

func TestGenerateTheoryContent_DefaultIsOverview(t *testing.T) {
	content := generateTheoryContent("Topic", "")
	if !strings.Contains(content, "Overview") {
		t.Error("empty depth should default to overview")
	}
}

func TestGenerateTheoryContent_UnknownDepthDefaultsToOverview(t *testing.T) {
	content := generateTheoryContent("Topic", "superficial")
	if !strings.Contains(content, "Overview") {
		t.Error("unknown depth should default to overview")
	}
}
