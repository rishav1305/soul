package modules

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/rishav1305/soul/internal/tutor/store"
)

func openTestStoreForImporter(t *testing.T) *store.Store {
	t.Helper()
	path := filepath.Join(t.TempDir(), "importer_test.db")
	s, err := store.Open(path)
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestNewImporter(t *testing.T) {
	s := openTestStoreForImporter(t)
	imp := NewImporter(s, "/tmp/content")
	if imp == nil {
		t.Fatal("NewImporter returned nil")
	}
	if imp.store != s {
		t.Error("store not set")
	}
	if imp.contentDir != "/tmp/content" {
		t.Errorf("contentDir = %q, want '/tmp/content'", imp.contentDir)
	}
}

func TestImportStdlib_MissingSourceDir(t *testing.T) {
	s := openTestStoreForImporter(t)
	imp := NewImporter(s, t.TempDir())

	// Override home to a temp dir that won't have interview-prep/.
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", t.TempDir())
	defer os.Setenv("HOME", origHome)

	_, err := imp.ImportStdlib()
	if err == nil {
		t.Fatal("expected error for missing source dir")
	}
}

func TestImportStdlib_WithSyntheticPythonFiles(t *testing.T) {
	s := openTestStoreForImporter(t)
	contentDir := filepath.Join(t.TempDir(), "content")
	imp := NewImporter(s, contentDir)

	// Create fake source directory structure.
	homeDir := t.TempDir()
	srcDir := filepath.Join(homeDir, "interview-prep", "week1", "stdlib_cheatsheet")
	if err := os.MkdirAll(srcDir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	// Write a synthetic .py file with EVALUATION_QUESTIONS.
	pyContent := `"""Heapq module cheatsheet."""
import heapq

EVALUATION_QUESTIONS = [
    {"question": "What is a heap?", "answer": "A binary tree with the heap property", "difficulty": "easy"},
    {"question": "How does heappush work?", "answer": "It adds an element and maintains the heap invariant", "difficulty": "medium"},
]

def example():
    h = []
    heapq.heappush(h, 3)
    return h
`
	if err := os.WriteFile(filepath.Join(srcDir, "01_heapq.py"), []byte(pyContent), 0644); err != nil {
		t.Fatalf("write py: %v", err)
	}

	// Write a non-.py file that should be skipped.
	if err := os.WriteFile(filepath.Join(srcDir, "README.md"), []byte("# README"), 0644); err != nil {
		t.Fatalf("write readme: %v", err)
	}

	// Write a .py file without EVALUATION_QUESTIONS.
	if err := os.WriteFile(filepath.Join(srcDir, "02_collections.py"), []byte("import collections\n"), 0644); err != nil {
		t.Fatalf("write py: %v", err)
	}

	// Override HOME so ImportStdlib finds our fake dir.
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", homeDir)
	defer os.Setenv("HOME", origHome)

	stats, err := imp.ImportStdlib()
	if err != nil {
		t.Fatalf("ImportStdlib: %v", err)
	}

	// Should have copied 2 .py files.
	if stats.FilesCopied != 2 {
		t.Errorf("FilesCopied = %d, want 2", stats.FilesCopied)
	}
	// Should have created 2 topics.
	if stats.TopicsCreated != 2 {
		t.Errorf("TopicsCreated = %d, want 2", stats.TopicsCreated)
	}
	// Should have imported 2 questions from first file, 0 from second.
	if stats.QuestionsImported != 2 {
		t.Errorf("QuestionsImported = %d, want 2", stats.QuestionsImported)
	}

	// Verify the copied file exists.
	copiedPath := filepath.Join(contentDir, "dsa", "stdlib", "01_heapq.py")
	if _, err := os.Stat(copiedPath); os.IsNotExist(err) {
		t.Error("expected copied file to exist")
	}

	// Verify topic name derivation (01_heapq → "heapq").
	// ListTopics(module, status) — second param is status, not category.
	topics, err := s.ListTopics("dsa", "")
	if err != nil {
		t.Fatalf("ListTopics: %v", err)
	}
	if len(topics) != 2 {
		t.Fatalf("expected 2 topics, got %d", len(topics))
	}
	// First topic should be "heapq" (stripped "01_").
	found := false
	for _, tp := range topics {
		if tp.Name == "heapq" {
			found = true
			if tp.Category != "stdlib" {
				t.Errorf("expected category 'stdlib', got %q", tp.Category)
			}
			break
		}
	}
	if !found {
		t.Error("expected topic named 'heapq'")
	}
}

func TestImportStats_JSON(t *testing.T) {
	stats := ImportStats{
		TopicsCreated:     5,
		QuestionsImported: 20,
		FilesCopied:       5,
	}
	if stats.TopicsCreated != 5 {
		t.Errorf("TopicsCreated = %d, want 5", stats.TopicsCreated)
	}
}

func TestRegexPatterns(t *testing.T) {
	// Verify the regex patterns compile and match expected content.
	pyContent := `EVALUATION_QUESTIONS = [
    {"question": "What is X?", "answer": "X is Y", "difficulty": "easy"},
]`

	blockMatch := evalBlockRe.FindStringSubmatch(pyContent)
	if len(blockMatch) < 2 {
		t.Fatal("evalBlockRe did not match")
	}

	dictEntries := dictEntryRe.FindAllString(blockMatch[1], -1)
	if len(dictEntries) != 1 {
		t.Fatalf("expected 1 dict entry, got %d", len(dictEntries))
	}

	qMatch := questionRe.FindStringSubmatch(dictEntries[0])
	if len(qMatch) < 2 || qMatch[1] != "What is X?" {
		t.Errorf("question match = %v, want 'What is X?'", qMatch)
	}

	aMatch := answerRe.FindStringSubmatch(dictEntries[0])
	if len(aMatch) < 2 || aMatch[1] != "X is Y" {
		t.Errorf("answer match = %v, want 'X is Y'", aMatch)
	}

	dMatch := difficultyRe.FindStringSubmatch(dictEntries[0])
	if len(dMatch) < 2 || dMatch[1] != "easy" {
		t.Errorf("difficulty match = %v, want 'easy'", dMatch)
	}
}

func TestNumberPrefixRegex(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"01_heapq", "heapq"},
		{"1-collections", "collections"},
		{"2.bisect", "bisect"},
		{"heapq", "heapq"},          // no prefix
		{"10_deque", "deque"},        // multi-digit
		{"100.functools", "functools"},
	}
	for _, tt := range tests {
		got := numberPrefixRe.ReplaceAllString(tt.input, "")
		if got != tt.want {
			t.Errorf("numberPrefixRe(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
