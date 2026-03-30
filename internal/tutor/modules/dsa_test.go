package modules

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rishav1305/soul/internal/tutor/eval"
	"github.com/rishav1305/soul/internal/tutor/store"
)

func openDSATestStore(t *testing.T) *store.Store {
	t.Helper()
	path := filepath.Join(t.TempDir(), "dsa_test.db")
	s, err := store.Open(path)
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func newDSAModule(t *testing.T) (*DSAModule, string) {
	t.Helper()
	s := openDSATestStore(t)
	contentDir := t.TempDir()
	return &DSAModule{
		store:      s,
		contentDir: contentDir,
		evaluator:  eval.New(nil), // word-overlap fallback
	}, contentDir
}

// ---------------------------------------------------------------------------
// Learn tests
// ---------------------------------------------------------------------------

func TestDSALearn_ByTopicID(t *testing.T) {
	m, _ := newDSAModule(t)

	topic, err := m.store.CreateTopic("dsa", "arrays", "Two Sum", "easy", "")
	if err != nil {
		t.Fatalf("CreateTopic: %v", err)
	}

	result, err := m.Learn(map[string]interface{}{
		"topic_id": float64(topic.ID),
	})
	if err != nil {
		t.Fatalf("Learn: %v", err)
	}
	if result == nil {
		t.Fatal("Learn returned nil result")
	}
	if !strings.Contains(result.Summary, "Two Sum") {
		t.Errorf("summary should contain topic name, got %q", result.Summary)
	}

	data := result.Data.(map[string]interface{})
	if _, ok := data["topic"]; !ok {
		t.Error("missing 'topic' key in data")
	}

	// Verify status updated to "learning".
	updated, err := m.store.GetTopic(topic.ID)
	if err != nil {
		t.Fatalf("GetTopic: %v", err)
	}
	if updated.Status != "learning" {
		t.Errorf("expected status 'learning', got %q", updated.Status)
	}
}

func TestDSALearn_ByNameAndCategory(t *testing.T) {
	m, _ := newDSAModule(t)

	_, err := m.store.CreateTopic("dsa", "trees", "Binary Search Tree", "medium", "")
	if err != nil {
		t.Fatalf("CreateTopic: %v", err)
	}

	result, err := m.Learn(map[string]interface{}{
		"topic":    "Binary Search Tree",
		"category": "trees",
	})
	if err != nil {
		t.Fatalf("Learn by name+category: %v", err)
	}
	if result == nil {
		t.Fatal("Learn returned nil")
	}
}

func TestDSALearn_ByNameOnly(t *testing.T) {
	m, _ := newDSAModule(t)

	_, err := m.store.CreateTopic("dsa", "graphs", "BFS", "medium", "")
	if err != nil {
		t.Fatalf("CreateTopic: %v", err)
	}

	result, err := m.Learn(map[string]interface{}{
		"topic": "BFS",
	})
	if err != nil {
		t.Fatalf("Learn by name-only: %v", err)
	}
	if result == nil {
		t.Fatal("Learn returned nil")
	}
}

func TestDSALearn_WithContentFile(t *testing.T) {
	m, contentDir := newDSAModule(t)

	// Write a content file.
	relPath := filepath.Join("dsa", "arrays", "merge_sort.md")
	fullPath := filepath.Join(contentDir, relPath)
	if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(fullPath, []byte("# Merge Sort\nDivide and conquer."), 0644); err != nil {
		t.Fatal(err)
	}

	topic, err := m.store.CreateTopic("dsa", "arrays", "Merge Sort", "medium", relPath)
	if err != nil {
		t.Fatal(err)
	}

	result, err := m.Learn(map[string]interface{}{
		"topic_id": float64(topic.ID),
	})
	if err != nil {
		t.Fatalf("Learn with content: %v", err)
	}

	data := result.Data.(map[string]interface{})
	content, ok := data["content"].(string)
	if !ok || !strings.Contains(content, "Merge Sort") {
		t.Errorf("expected content file contents, got %q", content)
	}
}

func TestDSALearn_ContentFileMissing(t *testing.T) {
	m, _ := newDSAModule(t)

	topic, err := m.store.CreateTopic("dsa", "arrays", "Quick Sort", "medium", "nonexistent/path.md")
	if err != nil {
		t.Fatal(err)
	}

	result, err := m.Learn(map[string]interface{}{
		"topic_id": float64(topic.ID),
	})
	if err != nil {
		t.Fatalf("Learn should not fail for missing content file: %v", err)
	}

	data := result.Data.(map[string]interface{})
	content, _ := data["content"].(string)
	if !strings.Contains(content, "Content file not found") {
		t.Errorf("expected fallback message, got %q", content)
	}
}

func TestDSALearn_MissingInput(t *testing.T) {
	m, _ := newDSAModule(t)
	_, err := m.Learn(map[string]interface{}{})
	if err == nil {
		t.Fatal("expected error for empty input")
	}
}

func TestDSALearn_TopicNotFound(t *testing.T) {
	m, _ := newDSAModule(t)
	_, err := m.Learn(map[string]interface{}{
		"topic": "NonexistentAlgorithm",
	})
	if err == nil {
		t.Fatal("expected error for unknown topic")
	}
}

// ---------------------------------------------------------------------------
// Build tests
// ---------------------------------------------------------------------------

func TestDSABuild_HappyPath(t *testing.T) {
	m, _ := newDSAModule(t)

	result, err := m.Build(map[string]interface{}{
		"topic": "Hash Map",
	})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if result == nil {
		t.Fatal("Build returned nil")
	}
	if !strings.Contains(result.Summary, "Hash Map") {
		t.Errorf("summary should contain topic name, got %q", result.Summary)
	}

	data := result.Data.(map[string]interface{})
	guide, ok := data["guide"].(string)
	if !ok {
		t.Fatal("missing 'guide' in data")
	}
	if !strings.Contains(guide, "Step 1") {
		t.Error("guide missing Step 1")
	}
	if !strings.Contains(guide, "Step 5") {
		t.Error("guide missing Step 5")
	}
}

func TestDSABuild_EmptyTopic(t *testing.T) {
	m, _ := newDSAModule(t)
	_, err := m.Build(map[string]interface{}{})
	if err == nil {
		t.Fatal("expected error for empty topic")
	}
}

// ---------------------------------------------------------------------------
// Drill tests
// ---------------------------------------------------------------------------

func TestDSADrill_StartMode(t *testing.T) {
	m, _ := newDSAModule(t)

	topic, err := m.store.CreateTopic("dsa", "arrays", "Sliding Window", "medium", "")
	if err != nil {
		t.Fatal(err)
	}
	_, err = m.store.CreateQuizQuestion(topic.ID, "medium",
		"What is the sliding window technique?",
		"Sliding window maintains a window that slides over data to track a subset efficiently.",
		"", "")
	if err != nil {
		t.Fatal(err)
	}

	result, err := m.Drill(map[string]interface{}{
		"topic_id": float64(topic.ID),
	})
	if err != nil {
		t.Fatalf("Drill start: %v", err)
	}

	data := result.Data.(map[string]interface{})
	if data["mode"] != "question" {
		t.Errorf("expected mode 'question', got %v", data["mode"])
	}
	if _, ok := data["question"]; !ok {
		t.Error("missing 'question' key")
	}
}

func TestDSADrill_AnswerMode_Correct(t *testing.T) {
	m, _ := newDSAModule(t)

	topic, err := m.store.CreateTopic("dsa", "arrays", "Two Pointers", "easy", "")
	if err != nil {
		t.Fatal(err)
	}
	q, err := m.store.CreateQuizQuestion(topic.ID, "easy",
		"What is the two pointer technique?",
		"Two pointers technique uses two pointers to iterate through data, often from opposite ends, to solve problems in linear time.",
		"", "")
	if err != nil {
		t.Fatal(err)
	}

	result, err := m.Drill(map[string]interface{}{
		"question_id": float64(q.ID),
		"answer":      "Two pointers iterate from opposite ends through data to solve problems in linear time.",
	})
	if err != nil {
		t.Fatalf("Drill answer: %v", err)
	}

	data := result.Data.(map[string]interface{})
	if data["mode"] != "result" {
		t.Errorf("expected mode 'result', got %v", data["mode"])
	}
	if _, ok := data["score"]; !ok {
		t.Error("missing 'score'")
	}
	if _, ok := data["correct"]; !ok {
		t.Error("missing 'correct'")
	}
	if _, ok := data["nextReview"]; !ok {
		t.Error("missing 'nextReview'")
	}
}

func TestDSADrill_AnswerMode_Incorrect(t *testing.T) {
	m, _ := newDSAModule(t)

	topic, err := m.store.CreateTopic("dsa", "arrays", "Kadane", "medium", "")
	if err != nil {
		t.Fatal(err)
	}
	q, err := m.store.CreateQuizQuestion(topic.ID, "medium",
		"Explain Kadane's algorithm.",
		"Kadane's algorithm finds the maximum subarray sum in O(n) by keeping track of current and global maximum.",
		"", "")
	if err != nil {
		t.Fatal(err)
	}

	result, err := m.Drill(map[string]interface{}{
		"question_id": float64(q.ID),
		"answer":      "I have no idea what this is about.",
	})
	if err != nil {
		t.Fatalf("Drill answer: %v", err)
	}

	data := result.Data.(map[string]interface{})
	if data["status"] != "incorrect" {
		t.Errorf("expected status 'incorrect', got %v", data["status"])
	}
}

func TestDSADrill_EmptyAnswer(t *testing.T) {
	m, _ := newDSAModule(t)

	topic, err := m.store.CreateTopic("dsa", "arrays", "Prefix Sum", "easy", "")
	if err != nil {
		t.Fatal(err)
	}
	q, err := m.store.CreateQuizQuestion(topic.ID, "easy",
		"What is prefix sum?", "Prefix sum precomputes cumulative sums.", "", "")
	if err != nil {
		t.Fatal(err)
	}

	_, err = m.Drill(map[string]interface{}{
		"question_id": float64(q.ID),
		// missing "answer"
	})
	if err == nil {
		t.Fatal("expected error for empty answer")
	}
}

func TestDSADrill_MissingInput(t *testing.T) {
	m, _ := newDSAModule(t)
	_, err := m.Drill(map[string]interface{}{})
	if err == nil {
		t.Fatal("expected error for missing topic_id and question_id")
	}
}

func TestDSADrill_NilEvaluator(t *testing.T) {
	s := openDSATestStore(t)
	m := &DSAModule{store: s, evaluator: nil}

	topic, err := m.store.CreateTopic("dsa", "sort", "Heap Sort", "hard", "")
	if err != nil {
		t.Fatal(err)
	}
	q, err := m.store.CreateQuizQuestion(topic.ID, "hard",
		"Explain heap sort.",
		"Heap sort builds a max heap then extracts elements for O(n log n) sorting.",
		"", "")
	if err != nil {
		t.Fatal(err)
	}

	result, err := m.Drill(map[string]interface{}{
		"question_id": float64(q.ID),
		"answer":      "Heap sort uses a max heap and extracts elements one by one for sorting in O(n log n).",
	})
	if err != nil {
		t.Fatalf("Drill nil evaluator: %v", err)
	}

	data := result.Data.(map[string]interface{})
	if data["mode"] != "result" {
		t.Errorf("expected mode 'result', got %v", data["mode"])
	}
}

// ---------------------------------------------------------------------------
// Solve tests
// ---------------------------------------------------------------------------

func TestDSASolve_HappyPath(t *testing.T) {
	m, _ := newDSAModule(t)

	result, err := m.Solve(map[string]interface{}{
		"topic":   "Binary Search",
		"problem": "Find target in sorted array.",
	})
	if err != nil {
		t.Fatalf("Solve: %v", err)
	}
	if result == nil {
		t.Fatal("Solve returned nil")
	}

	data := result.Data.(map[string]interface{})
	walkthrough, ok := data["walkthrough"].(string)
	if !ok {
		t.Fatal("missing 'walkthrough' in data")
	}
	if !strings.Contains(walkthrough, "Binary Search") {
		t.Error("walkthrough missing topic name")
	}
	if !strings.Contains(walkthrough, "Step 1") {
		t.Error("walkthrough missing Step 1")
	}
	if !strings.Contains(walkthrough, "Step 4") {
		t.Error("walkthrough missing Step 4")
	}
}

func TestDSASolve_MissingTopic(t *testing.T) {
	m, _ := newDSAModule(t)
	_, err := m.Solve(map[string]interface{}{
		"problem": "Find target",
	})
	if err == nil {
		t.Fatal("expected error for missing topic")
	}
}

func TestDSASolve_MissingProblem(t *testing.T) {
	m, _ := newDSAModule(t)
	_, err := m.Solve(map[string]interface{}{
		"topic": "BFS",
	})
	if err == nil {
		t.Fatal("expected error for missing problem")
	}
}

// ---------------------------------------------------------------------------
// GenerateContent tests
// ---------------------------------------------------------------------------

func TestDSAGenerateContent_HappyPath(t *testing.T) {
	m, contentDir := newDSAModule(t)

	result, err := m.GenerateContent(map[string]interface{}{
		"category": "sorting",
		"name":     "Merge Sort",
	})
	if err != nil {
		t.Fatalf("GenerateContent: %v", err)
	}
	if result == nil {
		t.Fatal("GenerateContent returned nil")
	}

	data := result.Data.(map[string]interface{})

	// Verify topic was created in store.
	topic, ok := data["topic"].(*store.Topic)
	if !ok {
		t.Fatalf("topic is %T, want *store.Topic", data["topic"])
	}
	if topic.Module != "dsa" {
		t.Errorf("expected module 'dsa', got %q", topic.Module)
	}
	if topic.Category != "sorting" {
		t.Errorf("expected category 'sorting', got %q", topic.Category)
	}
	if topic.Difficulty != "medium" {
		t.Errorf("expected default difficulty 'medium', got %q", topic.Difficulty)
	}

	// Verify file was written.
	contentPath, ok := data["contentPath"].(string)
	if !ok {
		t.Fatal("missing 'contentPath' in data")
	}
	fullPath := filepath.Join(contentDir, contentPath)
	fileData, err := os.ReadFile(fullPath)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if !strings.Contains(string(fileData), "# Merge Sort") {
		t.Error("content file missing heading")
	}
}

func TestDSAGenerateContent_CustomDifficulty(t *testing.T) {
	m, _ := newDSAModule(t)

	result, err := m.GenerateContent(map[string]interface{}{
		"category":   "graphs",
		"name":       "Dijkstra",
		"difficulty": "hard",
	})
	if err != nil {
		t.Fatalf("GenerateContent: %v", err)
	}

	data := result.Data.(map[string]interface{})
	topic := data["topic"].(*store.Topic)
	if topic.Difficulty != "hard" {
		t.Errorf("expected difficulty 'hard', got %q", topic.Difficulty)
	}
}

func TestDSAGenerateContent_CustomModule(t *testing.T) {
	m, _ := newDSAModule(t)

	result, err := m.GenerateContent(map[string]interface{}{
		"module":   "algorithms",
		"category": "dp",
		"name":     "Knapsack",
	})
	if err != nil {
		t.Fatalf("GenerateContent: %v", err)
	}

	data := result.Data.(map[string]interface{})
	topic := data["topic"].(*store.Topic)
	if topic.Module != "algorithms" {
		t.Errorf("expected module 'algorithms', got %q", topic.Module)
	}
}

func TestDSAGenerateContent_MissingCategory(t *testing.T) {
	m, _ := newDSAModule(t)
	_, err := m.GenerateContent(map[string]interface{}{
		"name": "Topic",
	})
	if err == nil {
		t.Fatal("expected error for missing category")
	}
}

func TestDSAGenerateContent_MissingName(t *testing.T) {
	m, _ := newDSAModule(t)
	_, err := m.GenerateContent(map[string]interface{}{
		"category": "sorting",
	})
	if err == nil {
		t.Fatal("expected error for missing name")
	}
}

// ---------------------------------------------------------------------------
// resolveTopic tests
// ---------------------------------------------------------------------------

func TestDSAResolveTopic_ByTopicID(t *testing.T) {
	m, _ := newDSAModule(t)

	created, err := m.store.CreateTopic("dsa", "arrays", "Rotate Array", "easy", "")
	if err != nil {
		t.Fatal(err)
	}

	topic, err := m.resolveTopic(map[string]interface{}{
		"topic_id": float64(created.ID),
	})
	if err != nil {
		t.Fatalf("resolveTopic by ID: %v", err)
	}
	if topic.Name != "Rotate Array" {
		t.Errorf("expected name 'Rotate Array', got %q", topic.Name)
	}
}

func TestDSAResolveTopic_ByNameAndCategory(t *testing.T) {
	m, _ := newDSAModule(t)

	_, err := m.store.CreateTopic("dsa", "strings", "KMP Algorithm", "hard", "")
	if err != nil {
		t.Fatal(err)
	}

	topic, err := m.resolveTopic(map[string]interface{}{
		"topic":    "KMP Algorithm",
		"category": "strings",
	})
	if err != nil {
		t.Fatalf("resolveTopic by name+category: %v", err)
	}
	if topic.Name != "KMP Algorithm" {
		t.Errorf("expected 'KMP Algorithm', got %q", topic.Name)
	}
}

func TestDSAResolveTopic_ByNameSearchFallback(t *testing.T) {
	m, _ := newDSAModule(t)

	_, err := m.store.CreateTopic("dsa", "trees", "AVL Tree", "hard", "")
	if err != nil {
		t.Fatal(err)
	}

	topic, err := m.resolveTopic(map[string]interface{}{
		"topic": "AVL Tree",
	})
	if err != nil {
		t.Fatalf("resolveTopic name-only: %v", err)
	}
	if topic.Name != "AVL Tree" {
		t.Errorf("expected 'AVL Tree', got %q", topic.Name)
	}
}

func TestDSAResolveTopic_NotFound(t *testing.T) {
	m, _ := newDSAModule(t)
	_, err := m.resolveTopic(map[string]interface{}{
		"topic": "Nonexistent",
	})
	if err == nil {
		t.Fatal("expected error for unknown topic")
	}
}

func TestDSAResolveTopic_NoInput(t *testing.T) {
	m, _ := newDSAModule(t)
	_, err := m.resolveTopic(map[string]interface{}{})
	if err == nil {
		t.Fatal("expected error for empty input")
	}
}

// ---------------------------------------------------------------------------
// getInt64 edge cases
// ---------------------------------------------------------------------------

func TestGetInt64_Int(t *testing.T) {
	v, ok := getInt64(map[string]interface{}{"k": int(42)}, "k")
	if !ok || v != 42 {
		t.Errorf("expected (42, true), got (%d, %v)", v, ok)
	}
}

func TestGetInt64_Int64(t *testing.T) {
	v, ok := getInt64(map[string]interface{}{"k": int64(99)}, "k")
	if !ok || v != 99 {
		t.Errorf("expected (99, true), got (%d, %v)", v, ok)
	}
}

func TestGetInt64_String(t *testing.T) {
	_, ok := getInt64(map[string]interface{}{"k": "not a number"}, "k")
	if ok {
		t.Error("expected false for string value")
	}
}

func TestGetInt64_Missing(t *testing.T) {
	_, ok := getInt64(map[string]interface{}{}, "k")
	if ok {
		t.Error("expected false for missing key")
	}
}
