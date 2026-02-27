package planner

import (
	"errors"
	"path/filepath"
	"testing"
	"time"
)

func openTestStore(t *testing.T) *Store {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	store, err := OpenStore(dbPath)
	if err != nil {
		t.Fatalf("OpenStore: %v", err)
	}
	t.Cleanup(func() { store.Close() })
	return store
}

func TestOpenStore(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "soul.db")

	store, err := OpenStore(dbPath)
	if err != nil {
		t.Fatalf("OpenStore: %v", err)
	}
	defer store.Close()

	// Verify the file was created on disk.
	if store.db == nil {
		t.Fatal("expected non-nil db")
	}
}

func TestCreateAndGet(t *testing.T) {
	store := openTestStore(t)

	task := NewTask("Build API", "Implement REST endpoints")
	task.Acceptance = "All endpoints return 200"
	task.Priority = 5
	task.Product = "soul-api"

	id, err := store.Create(task)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if id <= 0 {
		t.Fatalf("expected positive ID, got %d", id)
	}

	got, err := store.Get(id)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}

	if got.ID != id {
		t.Errorf("ID = %d, want %d", got.ID, id)
	}
	if got.Title != "Build API" {
		t.Errorf("Title = %q, want %q", got.Title, "Build API")
	}
	if got.Description != "Implement REST endpoints" {
		t.Errorf("Description = %q", got.Description)
	}
	if got.Acceptance != "All endpoints return 200" {
		t.Errorf("Acceptance = %q", got.Acceptance)
	}
	if got.Stage != StageBacklog {
		t.Errorf("Stage = %q, want %q", got.Stage, StageBacklog)
	}
	if got.Priority != 5 {
		t.Errorf("Priority = %d, want 5", got.Priority)
	}
	if got.Source != "manual" {
		t.Errorf("Source = %q, want %q", got.Source, "manual")
	}
	if got.Product != "soul-api" {
		t.Errorf("Product = %q, want %q", got.Product, "soul-api")
	}
	if got.MaxRetries != 3 {
		t.Errorf("MaxRetries = %d, want 3", got.MaxRetries)
	}
	if got.CreatedAt == "" {
		t.Error("CreatedAt should be set")
	}
	_, err = time.Parse(time.RFC3339, got.CreatedAt)
	if err != nil {
		t.Errorf("CreatedAt %q not valid RFC3339: %v", got.CreatedAt, err)
	}
}

func TestGetNotFound(t *testing.T) {
	store := openTestStore(t)

	_, err := store.Get(999)
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestList(t *testing.T) {
	store := openTestStore(t)

	// Create tasks in different stages and products.
	t1 := NewTask("Task 1", "")
	t1.Priority = 1
	t1.Product = "alpha"
	store.Create(t1)

	t2 := NewTask("Task 2", "")
	t2.Priority = 10
	t2.Product = "beta"
	store.Create(t2)

	activeStage := StageActive
	t3 := NewTask("Task 3", "")
	t3.Stage = StageActive
	t3.Priority = 5
	t3.Product = "alpha"
	store.Create(t3)

	// List all — should be ordered by priority DESC.
	all, err := store.List(TaskFilter{})
	if err != nil {
		t.Fatalf("List all: %v", err)
	}
	if len(all) != 3 {
		t.Fatalf("expected 3 tasks, got %d", len(all))
	}
	if all[0].Title != "Task 2" {
		t.Errorf("first by priority should be Task 2, got %q", all[0].Title)
	}

	// Filter by stage.
	backlog, err := store.List(TaskFilter{Stage: StageBacklog})
	if err != nil {
		t.Fatalf("List backlog: %v", err)
	}
	if len(backlog) != 2 {
		t.Fatalf("expected 2 backlog tasks, got %d", len(backlog))
	}

	active, err := store.List(TaskFilter{Stage: activeStage})
	if err != nil {
		t.Fatalf("List active: %v", err)
	}
	if len(active) != 1 {
		t.Fatalf("expected 1 active task, got %d", len(active))
	}

	// Filter by product.
	alphaList, err := store.List(TaskFilter{Product: "alpha"})
	if err != nil {
		t.Fatalf("List alpha: %v", err)
	}
	if len(alphaList) != 2 {
		t.Fatalf("expected 2 alpha tasks, got %d", len(alphaList))
	}
}

func TestUpdate(t *testing.T) {
	store := openTestStore(t)

	task := NewTask("Original", "desc")
	id, err := store.Create(task)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	newTitle := "Updated Title"
	newStage := StageActive
	err = store.Update(id, TaskUpdate{
		Title: &newTitle,
		Stage: &newStage,
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}

	got, err := store.Get(id)
	if err != nil {
		t.Fatalf("Get after update: %v", err)
	}
	if got.Title != "Updated Title" {
		t.Errorf("Title = %q, want %q", got.Title, "Updated Title")
	}
	if got.Stage != StageActive {
		t.Errorf("Stage = %q, want %q", got.Stage, StageActive)
	}
	// Description should remain unchanged.
	if got.Description != "desc" {
		t.Errorf("Description = %q, want %q", got.Description, "desc")
	}
}

func TestUpdateNotFound(t *testing.T) {
	store := openTestStore(t)
	title := "x"
	err := store.Update(999, TaskUpdate{Title: &title})
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestDelete(t *testing.T) {
	store := openTestStore(t)

	id, err := store.Create(NewTask("To Delete", ""))
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	err = store.Delete(id)
	if err != nil {
		t.Fatalf("Delete: %v", err)
	}

	_, err = store.Get(id)
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound after delete, got %v", err)
	}
}

func TestDeleteNotFound(t *testing.T) {
	store := openTestStore(t)
	err := store.Delete(999)
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestNextReady(t *testing.T) {
	store := openTestStore(t)

	// Empty store should return ErrNotFound.
	_, err := store.NextReady()
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("empty store: expected ErrNotFound, got %v", err)
	}

	// Create two backlog tasks with different priorities.
	low := NewTask("Low Priority", "")
	low.Priority = 1
	lowID, _ := store.Create(low)

	high := NewTask("High Priority", "")
	high.Priority = 10
	store.Create(high)

	_ = lowID // used below

	// NextReady should return the high-priority task.
	ready, err := store.NextReady()
	if err != nil {
		t.Fatalf("NextReady: %v", err)
	}
	if ready.Title != "High Priority" {
		t.Errorf("Title = %q, want %q", ready.Title, "High Priority")
	}
}

func TestDependencies(t *testing.T) {
	store := openTestStore(t)

	// Create a blocker task (backlog) and a dependent task (backlog).
	blocker := NewTask("Blocker", "")
	blocker.Priority = 1
	blockerID, _ := store.Create(blocker)

	dependent := NewTask("Dependent", "")
	dependent.Priority = 100 // higher priority, but blocked
	depID, _ := store.Create(dependent)

	// Add dependency: dependent depends on blocker.
	err := store.AddDependency(depID, blockerID)
	if err != nil {
		t.Fatalf("AddDependency: %v", err)
	}

	// NextReady should skip the dependent (unresolved dep) and return blocker.
	ready, err := store.NextReady()
	if err != nil {
		t.Fatalf("NextReady with deps: %v", err)
	}
	if ready.ID != blockerID {
		t.Errorf("expected blocker (ID=%d), got ID=%d (%q)", blockerID, ready.ID, ready.Title)
	}

	// Mark blocker as done.
	doneStage := StageDone
	err = store.Update(blockerID, TaskUpdate{Stage: &doneStage})
	if err != nil {
		t.Fatalf("Update blocker to done: %v", err)
	}

	// Now NextReady should return the dependent task.
	ready, err = store.NextReady()
	if err != nil {
		t.Fatalf("NextReady after dep resolved: %v", err)
	}
	if ready.ID != depID {
		t.Errorf("expected dependent (ID=%d), got ID=%d (%q)", depID, ready.ID, ready.Title)
	}

	// Test RemoveDependency.
	err = store.RemoveDependency(depID, blockerID)
	if err != nil {
		t.Fatalf("RemoveDependency: %v", err)
	}
}
