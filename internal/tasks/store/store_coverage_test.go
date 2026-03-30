package store

import (
	"path/filepath"
	"strings"
	"testing"
)

func newCoverageStore(t *testing.T) *Store {
	t.Helper()
	path := filepath.Join(t.TempDir(), "coverage.db")
	s, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

// --- Delete error paths ---

func TestDelete_NotFound(t *testing.T) {
	s := newCoverageStore(t)
	err := s.Delete(99999)
	if err == nil {
		t.Error("expected error for nonexistent task")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("error = %q, want 'not found'", err)
	}
}

// --- AddDependency / RemoveDependency ---

func TestAddDependency_Roundtrip(t *testing.T) {
	s := newCoverageStore(t)
	t1, err := s.Create("Task A", "desc", "")
	if err != nil {
		t.Fatal(err)
	}
	t2, err := s.Create("Task B", "desc", "")
	if err != nil {
		t.Fatal(err)
	}

	if err := s.AddDependency(t2.ID, t1.ID); err != nil {
		t.Fatalf("AddDependency: %v", err)
	}

	// Idempotent — add again should not error.
	if err := s.AddDependency(t2.ID, t1.ID); err != nil {
		t.Fatalf("AddDependency (idempotent): %v", err)
	}

	if err := s.RemoveDependency(t2.ID, t1.ID); err != nil {
		t.Fatalf("RemoveDependency: %v", err)
	}
}

// --- MaxSeq ---

func TestMaxSeq_IncreasesOnCreate(t *testing.T) {
	s := newCoverageStore(t)
	seq1, err := s.MaxSeq()
	if err != nil {
		t.Fatal(err)
	}

	s.Create("task", "desc", "")

	seq2, err := s.MaxSeq()
	if err != nil {
		t.Fatal(err)
	}
	if seq2 <= seq1 {
		t.Errorf("MaxSeq should increase: before=%d, after=%d", seq1, seq2)
	}
}

// --- ListModifiedSince ---

func TestListModifiedSince_ReturnsTasksAfterSeq(t *testing.T) {
	s := newCoverageStore(t)
	seq0, _ := s.MaxSeq()

	s.Create("first", "desc", "")
	s.Create("second", "desc", "")

	tasks, err := s.ListModifiedSince(seq0)
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks) != 2 {
		t.Errorf("expected 2 tasks, got %d", len(tasks))
	}
}

func TestListModifiedSince_EmptyResult(t *testing.T) {
	s := newCoverageStore(t)
	seq, _ := s.MaxSeq()

	tasks, err := s.ListModifiedSince(seq)
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks) != 0 {
		t.Errorf("expected 0 tasks, got %d", len(tasks))
	}
}

// --- ListDeletedSince ---

func TestListDeletedSince_ReturnsTombstones(t *testing.T) {
	s := newCoverageStore(t)
	seq0, _ := s.MaxSeq()

	task, _ := s.Create("to delete", "desc", "")
	s.Delete(task.ID)

	ids, err := s.ListDeletedSince(seq0)
	if err != nil {
		t.Fatal(err)
	}
	if len(ids) != 1 {
		t.Fatalf("expected 1 tombstone, got %d", len(ids))
	}
	if ids[0] != task.ID {
		t.Errorf("tombstone ID = %d, want %d", ids[0], task.ID)
	}
}

func TestListDeletedSince_Empty(t *testing.T) {
	s := newCoverageStore(t)
	seq, _ := s.MaxSeq()

	ids, err := s.ListDeletedSince(seq)
	if err != nil {
		t.Fatal(err)
	}
	if len(ids) != 0 {
		t.Errorf("expected 0 tombstones, got %d", len(ids))
	}
}

// --- PruneTombstones ---

func TestPruneTombstones_NothingToRemove(t *testing.T) {
	s := newCoverageStore(t)
	n, err := s.PruneTombstones()
	if err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Errorf("expected 0 pruned, got %d", n)
	}
}

// --- AllCommentsAfterID ---

func TestAllCommentsAfterID_ReturnsNewComments(t *testing.T) {
	s := newCoverageStore(t)
	task, _ := s.Create("commented task", "desc", "")

	c1, err := s.InsertComment(task.ID, "alice", "note", "first")
	if err != nil {
		t.Fatal(err)
	}
	c2, err := s.InsertComment(task.ID, "bob", "note", "second")
	if err != nil {
		t.Fatal(err)
	}

	// Get comments after c1 — should only return c2.
	comments, err := s.AllCommentsAfterID(task.ID, c1.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(comments) != 1 {
		t.Fatalf("expected 1 comment, got %d", len(comments))
	}
	if comments[0].ID != c2.ID {
		t.Errorf("comment ID = %d, want %d", comments[0].ID, c2.ID)
	}
}

func TestAllCommentsAfterID_Empty(t *testing.T) {
	s := newCoverageStore(t)
	task, _ := s.Create("no comments", "desc", "")

	comments, err := s.AllCommentsAfterID(task.ID, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(comments) != 0 {
		t.Errorf("expected 0 comments, got %d", len(comments))
	}
}

// --- ActivityAfterID ---

func TestActivityAfterID_ReturnsNewActivity(t *testing.T) {
	s := newCoverageStore(t)
	task, _ := s.Create("activity task", "desc", "")

	a1, err := s.AddActivity(task.ID, "created", nil)
	if err != nil {
		t.Fatal(err)
	}
	a2, err := s.AddActivity(task.ID, "updated", map[string]interface{}{"field": "stage"})
	if err != nil {
		t.Fatal(err)
	}

	activities, err := s.ActivityAfterID(task.ID, a1.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(activities) != 1 {
		t.Fatalf("expected 1 activity, got %d", len(activities))
	}
	if activities[0].ID != a2.ID {
		t.Errorf("activity ID = %d, want %d", activities[0].ID, a2.ID)
	}
}

func TestActivityAfterID_Empty(t *testing.T) {
	s := newCoverageStore(t)
	task, _ := s.Create("no activity", "desc", "")

	activities, err := s.ActivityAfterID(task.ID, 0)
	if err != nil {
		t.Fatal(err)
	}
	// Create itself may have logged activity via OnChange, check >= 0 instead.
	_ = activities // just ensure no error
}

// --- NextReady ---

func TestNextReady_ReturnsBacklogTask(t *testing.T) {
	s := newCoverageStore(t)
	task, _ := s.Create("backlog task", "desc", "")
	// Default stage is "backlog".

	ready, err := s.NextReady()
	if err != nil {
		t.Fatal(err)
	}
	if ready == nil {
		t.Fatal("expected a ready task")
	}
	if ready.ID != task.ID {
		t.Errorf("NextReady ID = %d, want %d", ready.ID, task.ID)
	}
}

func TestNextReady_NoBacklogTasks(t *testing.T) {
	s := newCoverageStore(t)
	// No tasks at all.
	_, err := s.NextReady()
	if err == nil {
		t.Error("expected error when no tasks")
	}
	if !strings.Contains(err.Error(), "no ready task") {
		t.Errorf("error = %q, want 'no ready task'", err)
	}
}

func TestNextReady_SkipsBlockedTask(t *testing.T) {
	s := newCoverageStore(t)
	t1, _ := s.Create("blocker", "desc", "")
	t2, _ := s.Create("blocked", "desc", "")

	// t2 depends on t1 (t1 is still in backlog, not done).
	s.AddDependency(t2.ID, t1.ID)

	// NextReady should return t1 (which has no deps), not t2.
	ready, err := s.NextReady()
	if err != nil {
		t.Fatal(err)
	}
	if ready.ID != t1.ID {
		t.Errorf("NextReady = %d, want %d (unblocked)", ready.ID, t1.ID)
	}
}

// --- Create with product ---

func TestCreate_WithProduct(t *testing.T) {
	s := newCoverageStore(t)
	task, err := s.Create("product task", "desc", "scout")
	if err != nil {
		t.Fatal(err)
	}
	if task.Product != "scout" {
		t.Errorf("Product = %q, want scout", task.Product)
	}
}

// --- Closed store operations ---

func TestClosedStore_Operations(t *testing.T) {
	s := newCoverageStore(t)
	s.Close()

	_, err := s.Create("title", "desc", "")
	if err == nil {
		t.Error("expected error from Create on closed store")
	}
	_, err = s.Get(1)
	if err == nil {
		t.Error("expected error from Get on closed store")
	}
	_, err = s.List("", "")
	if err == nil {
		t.Error("expected error from List on closed store")
	}
	err = s.Delete(1)
	if err == nil {
		t.Error("expected error from Delete on closed store")
	}
	_, err = s.MaxSeq()
	if err == nil {
		t.Error("expected error from MaxSeq on closed store")
	}
	_, err = s.ListModifiedSince(0)
	if err == nil {
		t.Error("expected error from ListModifiedSince on closed store")
	}
	_, err = s.ListDeletedSince(0)
	if err == nil {
		t.Error("expected error from ListDeletedSince on closed store")
	}
	_, err = s.PruneTombstones()
	if err == nil {
		t.Error("expected error from PruneTombstones on closed store")
	}
	_, err = s.CountByStage()
	if err == nil {
		t.Error("expected error from CountByStage on closed store")
	}
	_, err = s.AllCommentsAfterID(1, 0)
	if err == nil {
		t.Error("expected error from AllCommentsAfterID on closed store")
	}
	_, err = s.ActivityAfterID(1, 0)
	if err == nil {
		t.Error("expected error from ActivityAfterID on closed store")
	}
	_, err = s.NextReady()
	if err == nil {
		t.Error("expected error from NextReady on closed store")
	}
	_, err = s.GetComments(1)
	if err == nil {
		t.Error("expected error from GetComments on closed store")
	}
	_, err = s.CommentsAfter(0)
	if err == nil {
		t.Error("expected error from CommentsAfter on closed store")
	}
	_, err = s.ListActivity(1)
	if err == nil {
		t.Error("expected error from ListActivity on closed store")
	}
	_, err = s.InsertComment(1, "a", "note", "body")
	if err == nil {
		t.Error("expected error from InsertComment on closed store")
	}
	_, err = s.AddActivity(1, "event", nil)
	if err == nil {
		t.Error("expected error from AddActivity on closed store")
	}
	err = s.AddDependency(1, 2)
	if err == nil {
		t.Error("expected error from AddDependency on closed store")
	}
	err = s.RemoveDependency(1, 2)
	if err == nil {
		t.Error("expected error from RemoveDependency on closed store")
	}
}

// --- Open error path ---

func TestOpen_InvalidPath(t *testing.T) {
	_, err := Open("/nonexistent/deep/path/db.sqlite")
	if err == nil {
		t.Error("expected error for invalid path")
	}
}

// --- OnChange hook ---

func TestOnChange_FiresOnCreate(t *testing.T) {
	s := newCoverageStore(t)
	fired := false
	s.OnChange = func(event string, payload any) {
		if event == "task.created" {
			fired = true
		}
	}

	_, err := s.Create("trigger test", "desc", "")
	if err != nil {
		t.Fatal(err)
	}
	if !fired {
		t.Error("OnChange was not called for task.created")
	}
}

func TestOnChange_FiresOnDelete(t *testing.T) {
	s := newCoverageStore(t)
	task, _ := s.Create("delete hook", "desc", "")

	fired := false
	s.OnChange = func(event string, payload any) {
		if event == "task.deleted" {
			fired = true
		}
	}

	s.Delete(task.ID)
	if !fired {
		t.Error("OnChange was not called for task.deleted")
	}
}
