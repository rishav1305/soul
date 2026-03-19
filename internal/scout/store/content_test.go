package store

import (
	"testing"
)

func TestAddContentPost_ListContentPosts(t *testing.T) {
	s := newTestStore(t)

	// Insert posts with different platforms and statuses.
	id1, err := s.AddContentPost(ContentPost{
		Platform: "linkedin",
		Pillar:   "technical",
		Topic:    "Go concurrency patterns",
		Status:   "published",
		Content:  "Here are my top 5 Go concurrency patterns...",
		Hook:     "Stop using goroutines wrong.",
	})
	if err != nil {
		t.Fatalf("AddContentPost 1: %v", err)
	}
	if id1 == 0 {
		t.Error("expected non-zero ID")
	}

	id2, err := s.AddContentPost(ContentPost{
		Platform: "linkedin",
		Pillar:   "career",
		Topic:    "Remote work tips",
		Status:   "draft",
		Content:  "Working remotely from India...",
	})
	if err != nil {
		t.Fatalf("AddContentPost 2: %v", err)
	}

	_, err = s.AddContentPost(ContentPost{
		Platform: "twitter",
		Pillar:   "technical",
		Topic:    "SQLite in production",
		Status:   "published",
	})
	if err != nil {
		t.Fatalf("AddContentPost 3: %v", err)
	}

	if id1 == id2 {
		t.Error("expected unique IDs")
	}

	// List all.
	all, err := s.ListContentPosts("", "")
	if err != nil {
		t.Fatalf("ListContentPosts all: %v", err)
	}
	if len(all) != 3 {
		t.Errorf("len = %d, want 3", len(all))
	}

	// Filter by platform.
	linkedin, err := s.ListContentPosts("linkedin", "")
	if err != nil {
		t.Fatalf("ListContentPosts linkedin: %v", err)
	}
	if len(linkedin) != 2 {
		t.Errorf("len = %d, want 2", len(linkedin))
	}

	// Filter by status.
	published, err := s.ListContentPosts("", "published")
	if err != nil {
		t.Fatalf("ListContentPosts published: %v", err)
	}
	if len(published) != 2 {
		t.Errorf("len = %d, want 2", len(published))
	}

	// Filter by both.
	linkedinPublished, err := s.ListContentPosts("linkedin", "published")
	if err != nil {
		t.Fatalf("ListContentPosts linkedin+published: %v", err)
	}
	if len(linkedinPublished) != 1 {
		t.Errorf("len = %d, want 1", len(linkedinPublished))
	}
	if linkedinPublished[0].Topic != "Go concurrency patterns" {
		t.Errorf("topic = %q, want %q", linkedinPublished[0].Topic, "Go concurrency patterns")
	}
}

func TestUpdateContentPost(t *testing.T) {
	s := newTestStore(t)

	id, _ := s.AddContentPost(ContentPost{
		Platform: "linkedin",
		Pillar:   "technical",
		Topic:    "Go patterns",
		Status:   "published",
	})

	// Update impressions and likes.
	err := s.UpdateContentPost(id, map[string]interface{}{
		"impressions": 1500,
		"likes":       42,
		"shares":      8,
	})
	if err != nil {
		t.Fatalf("UpdateContentPost: %v", err)
	}

	// Verify.
	posts, err := s.ListContentPosts("linkedin", "published")
	if err != nil {
		t.Fatalf("ListContentPosts: %v", err)
	}
	if len(posts) != 1 {
		t.Fatalf("len = %d, want 1", len(posts))
	}
	if posts[0].Impressions != 1500 {
		t.Errorf("Impressions = %d, want 1500", posts[0].Impressions)
	}
	if posts[0].Likes != 42 {
		t.Errorf("Likes = %d, want 42", posts[0].Likes)
	}
	if posts[0].Shares != 8 {
		t.Errorf("Shares = %d, want 8", posts[0].Shares)
	}

	// Not found.
	err = s.UpdateContentPost(999, map[string]interface{}{"likes": 1})
	if err == nil {
		t.Error("expected error for non-existent post")
	}

	// Invalid field silently ignored, no update.
	err = s.UpdateContentPost(id, map[string]interface{}{"invalid_field": "x"})
	if err != nil {
		t.Errorf("expected nil for no valid fields, got %v", err)
	}
}

func TestAddBacklogItem_ListBacklog(t *testing.T) {
	s := newTestStore(t)

	id1, err := s.AddBacklogItem(BacklogItem{
		Topic:  "AI agent architecture",
		Pillar: "technical",
		Source: "experience",
		Angle:  "How I built a 100-tool agent",
		Status: "pending",
	})
	if err != nil {
		t.Fatalf("AddBacklogItem 1: %v", err)
	}
	if id1 == 0 {
		t.Error("expected non-zero ID")
	}

	_, err = s.AddBacklogItem(BacklogItem{
		Topic:  "Interview prep tips",
		Pillar: "career",
		Source: "research",
		Status: "pending",
	})
	if err != nil {
		t.Fatalf("AddBacklogItem 2: %v", err)
	}

	_, err = s.AddBacklogItem(BacklogItem{
		Topic:  "Old topic",
		Pillar: "technical",
		Source: "brainstorm",
		Status: "archived",
	})
	if err != nil {
		t.Fatalf("AddBacklogItem 3: %v", err)
	}

	// List all.
	all, err := s.ListBacklog("")
	if err != nil {
		t.Fatalf("ListBacklog all: %v", err)
	}
	if len(all) != 3 {
		t.Errorf("len = %d, want 3", len(all))
	}

	// Filter by status.
	pending, err := s.ListBacklog("pending")
	if err != nil {
		t.Fatalf("ListBacklog pending: %v", err)
	}
	if len(pending) != 2 {
		t.Errorf("len = %d, want 2", len(pending))
	}

	archived, err := s.ListBacklog("archived")
	if err != nil {
		t.Fatalf("ListBacklog archived: %v", err)
	}
	if len(archived) != 1 {
		t.Errorf("len = %d, want 1", len(archived))
	}
	if archived[0].Topic != "Old topic" {
		t.Errorf("topic = %q, want %q", archived[0].Topic, "Old topic")
	}
}

func TestUpdateBacklogItem(t *testing.T) {
	s := newTestStore(t)

	id, _ := s.AddBacklogItem(BacklogItem{
		Topic:  "AI agents",
		Pillar: "technical",
		Source: "experience",
		Status: "pending",
	})

	// Update status to archived.
	err := s.UpdateBacklogItem(id, map[string]interface{}{
		"status":      "archived",
		"archived_at": now(),
	})
	if err != nil {
		t.Fatalf("UpdateBacklogItem: %v", err)
	}

	// Verify.
	archived, err := s.ListBacklog("archived")
	if err != nil {
		t.Fatalf("ListBacklog: %v", err)
	}
	if len(archived) != 1 {
		t.Fatalf("len = %d, want 1", len(archived))
	}
	if archived[0].Status != "archived" {
		t.Errorf("Status = %q, want %q", archived[0].Status, "archived")
	}
	if archived[0].ArchivedAt == "" {
		t.Error("ArchivedAt should be set")
	}

	// Not found.
	err = s.UpdateBacklogItem(999, map[string]interface{}{"status": "archived"})
	if err == nil {
		t.Error("expected error for non-existent item")
	}

	// Invalid field silently ignored, no update.
	err = s.UpdateBacklogItem(id, map[string]interface{}{"invalid_field": "x"})
	if err != nil {
		t.Errorf("expected nil for no valid fields, got %v", err)
	}
}
