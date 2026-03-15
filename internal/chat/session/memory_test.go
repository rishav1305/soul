package session

import (
	"strings"
	"testing"
	"time"
)

func TestUpsertMemory_Create(t *testing.T) {
	s := openTestStore(t)

	m, err := s.UpsertMemory("user.name", "Rishav", "profile,identity")
	if err != nil {
		t.Fatalf("UpsertMemory: %v", err)
	}
	if m.Key != "user.name" {
		t.Errorf("Key = %q, want %q", m.Key, "user.name")
	}
	if m.Content != "Rishav" {
		t.Errorf("Content = %q, want %q", m.Content, "Rishav")
	}
	if m.Tags != "profile,identity" {
		t.Errorf("Tags = %q, want %q", m.Tags, "profile,identity")
	}
	if m.ID == 0 {
		t.Error("ID should be non-zero")
	}
	if m.CreatedAt == "" {
		t.Error("CreatedAt should not be empty")
	}
	if m.UpdatedAt == "" {
		t.Error("UpdatedAt should not be empty")
	}
	// Verify timestamp is valid RFC3339.
	if _, err := time.Parse(time.RFC3339, m.CreatedAt); err != nil {
		t.Errorf("CreatedAt is not valid RFC3339: %v", err)
	}
}

func TestUpsertMemory_Update(t *testing.T) {
	s := openTestStore(t)

	m1, err := s.UpsertMemory("pref.theme", "dark", "preferences")
	if err != nil {
		t.Fatalf("UpsertMemory (create): %v", err)
	}

	// Small delay to ensure updated_at differs.
	time.Sleep(10 * time.Millisecond)

	m2, err := s.UpsertMemory("pref.theme", "light", "preferences,ui")
	if err != nil {
		t.Fatalf("UpsertMemory (update): %v", err)
	}

	if m2.ID != m1.ID {
		t.Errorf("ID changed: %d -> %d", m1.ID, m2.ID)
	}
	if m2.Content != "light" {
		t.Errorf("Content = %q, want %q", m2.Content, "light")
	}
	if m2.Tags != "preferences,ui" {
		t.Errorf("Tags = %q, want %q", m2.Tags, "preferences,ui")
	}
}

func TestGetMemory(t *testing.T) {
	s := openTestStore(t)

	_, err := s.UpsertMemory("test.key", "test content", "tag1")
	if err != nil {
		t.Fatalf("UpsertMemory: %v", err)
	}

	m, err := s.GetMemory("test.key")
	if err != nil {
		t.Fatalf("GetMemory: %v", err)
	}
	if m.Content != "test content" {
		t.Errorf("Content = %q, want %q", m.Content, "test content")
	}
}

func TestGetMemory_NotFound(t *testing.T) {
	s := openTestStore(t)

	_, err := s.GetMemory("nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent key")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("error = %q, want 'not found'", err)
	}
}

func TestSearchMemories(t *testing.T) {
	s := openTestStore(t)

	if _, err := s.UpsertMemory("user.name", "Rishav", "profile"); err != nil {
		t.Fatalf("UpsertMemory: %v", err)
	}
	if _, err := s.UpsertMemory("user.lang", "Go", "profile,tech"); err != nil {
		t.Fatalf("UpsertMemory: %v", err)
	}
	if _, err := s.UpsertMemory("project.current", "soul-v2", "project"); err != nil {
		t.Fatalf("UpsertMemory: %v", err)
	}

	// Search by tag substring.
	results, err := s.SearchMemories("profile")
	if err != nil {
		t.Fatalf("SearchMemories: %v", err)
	}
	if len(results) != 2 {
		t.Errorf("got %d results, want 2", len(results))
	}

	// Search by content.
	results, err = s.SearchMemories("soul-v2")
	if err != nil {
		t.Fatalf("SearchMemories: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("got %d results, want 1", len(results))
	}
}

func TestListMemories(t *testing.T) {
	s := openTestStore(t)

	if _, err := s.UpsertMemory("a", "first", ""); err != nil {
		t.Fatalf("UpsertMemory: %v", err)
	}
	if _, err := s.UpsertMemory("b", "second", ""); err != nil {
		t.Fatalf("UpsertMemory: %v", err)
	}
	if _, err := s.UpsertMemory("c", "third", ""); err != nil {
		t.Fatalf("UpsertMemory: %v", err)
	}

	results, err := s.ListMemories(2)
	if err != nil {
		t.Fatalf("ListMemories: %v", err)
	}
	if len(results) != 2 {
		t.Errorf("got %d results, want 2", len(results))
	}
}

func TestDeleteMemory(t *testing.T) {
	s := openTestStore(t)

	if _, err := s.UpsertMemory("to.delete", "bye", ""); err != nil {
		t.Fatalf("UpsertMemory: %v", err)
	}

	if err := s.DeleteMemory("to.delete"); err != nil {
		t.Fatalf("DeleteMemory: %v", err)
	}

	_, err := s.GetMemory("to.delete")
	if err == nil {
		t.Fatal("expected error after delete")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("error = %q, want 'not found'", err)
	}

	// Deleting again should error.
	err = s.DeleteMemory("to.delete")
	if err == nil {
		t.Fatal("expected error deleting nonexistent key")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("error = %q, want 'not found'", err)
	}
}
