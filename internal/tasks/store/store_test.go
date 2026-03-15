package store

import (
	"path/filepath"
	"testing"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	path := filepath.Join(t.TempDir(), "tasks_test.db")
	s, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestOpen_CreatesDatabase(t *testing.T) {
	s := newTestStore(t)
	if s == nil {
		t.Fatal("store is nil")
	}
}

func TestCreate_ReturnsTask(t *testing.T) {
	s := newTestStore(t)
	task, err := s.Create("Test task", "Description", "")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if task.ID == 0 {
		t.Error("expected non-zero ID")
	}
	if task.Title != "Test task" {
		t.Errorf("Title = %q, want %q", task.Title, "Test task")
	}
	if task.Stage != "backlog" {
		t.Errorf("Stage = %q, want %q", task.Stage, "backlog")
	}
}

func TestGet_ReturnsCreatedTask(t *testing.T) {
	s := newTestStore(t)
	created, _ := s.Create("Get test", "desc", "")
	got, err := s.Get(created.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Title != "Get test" {
		t.Errorf("Title = %q, want %q", got.Title, "Get test")
	}
}

func TestGet_NotFound(t *testing.T) {
	s := newTestStore(t)
	_, err := s.Get(999)
	if err == nil {
		t.Error("expected error for non-existent task")
	}
}

func TestList_FiltersByStage(t *testing.T) {
	s := newTestStore(t)
	s.Create("Task A", "", "")
	s.Create("Task B", "", "")
	task3, _ := s.Create("Task C", "", "")
	s.Update(task3.ID, map[string]interface{}{"stage": "active"})

	all, _ := s.List("", "")
	if len(all) != 3 {
		t.Errorf("List('') = %d tasks, want 3", len(all))
	}

	backlog, _ := s.List("backlog", "")
	if len(backlog) != 2 {
		t.Errorf("List('backlog') = %d tasks, want 2", len(backlog))
	}

	active, _ := s.List("active", "")
	if len(active) != 1 {
		t.Errorf("List('active') = %d tasks, want 1", len(active))
	}
}

func TestList_FiltersByProduct(t *testing.T) {
	s := newTestStore(t)
	s.Create("Core task", "", "")
	s.Create("Scout task", "", "scout")

	scout, _ := s.List("", "scout")
	if len(scout) != 1 {
		t.Errorf("List(product=scout) = %d, want 1", len(scout))
	}
}

func TestUpdate_ChangesFields(t *testing.T) {
	s := newTestStore(t)
	task, _ := s.Create("Original", "desc", "")
	updated, err := s.Update(task.ID, map[string]interface{}{
		"title": "Updated",
		"stage": "active",
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if updated.Title != "Updated" {
		t.Errorf("Title = %q, want %q", updated.Title, "Updated")
	}
	if updated.Stage != "active" {
		t.Errorf("Stage = %q, want %q", updated.Stage, "active")
	}
}

func TestUpdate_RejectsInvalidStage(t *testing.T) {
	s := newTestStore(t)
	task, _ := s.Create("Task", "", "")
	_, err := s.Update(task.ID, map[string]interface{}{"stage": "invalid"})
	if err == nil {
		t.Error("expected error for invalid stage")
	}
}

func TestUpdate_NotFound(t *testing.T) {
	s := newTestStore(t)
	_, err := s.Update(999, map[string]interface{}{"title": "x"})
	if err == nil {
		t.Error("expected error for non-existent task")
	}
}

func TestAddActivity_And_ListActivity(t *testing.T) {
	s := newTestStore(t)
	task, _ := s.Create("Task", "", "")

	err := s.AddActivity(task.ID, "task.created", map[string]interface{}{"by": "user"})
	if err != nil {
		t.Fatalf("AddActivity: %v", err)
	}

	activities, err := s.ListActivity(task.ID)
	if err != nil {
		t.Fatalf("ListActivity: %v", err)
	}
	if len(activities) != 1 {
		t.Fatalf("ListActivity = %d, want 1", len(activities))
	}
	if activities[0].EventType != "task.created" {
		t.Errorf("EventType = %q, want %q", activities[0].EventType, "task.created")
	}
}

func TestDelete_RemovesTaskAndActivity(t *testing.T) {
	s := newTestStore(t)
	task, _ := s.Create("Doomed", "", "")
	s.AddActivity(task.ID, "task.created", nil)

	err := s.Delete(task.ID)
	if err != nil {
		t.Fatalf("Delete: %v", err)
	}

	_, err = s.Get(task.ID)
	if err == nil {
		t.Error("expected error after delete")
	}

	activities, _ := s.ListActivity(task.ID)
	if len(activities) != 0 {
		t.Errorf("activities after delete = %d, want 0", len(activities))
	}
}

func createTask(t *testing.T, s *Store, title string) *Task {
	t.Helper()
	task, err := s.Create(title, "", "")
	if err != nil {
		t.Fatalf("Create(%q): %v", title, err)
	}
	return task
}

func TestAddDependency(t *testing.T) {
	s := newTestStore(t)
	a := createTask(t, s, "Task A")
	b := createTask(t, s, "Task B")

	if err := s.AddDependency(b.ID, a.ID); err != nil {
		t.Fatalf("AddDependency: %v", err)
	}
	// Idempotent — no error on duplicate.
	if err := s.AddDependency(b.ID, a.ID); err != nil {
		t.Fatalf("AddDependency duplicate: %v", err)
	}
}

func TestRemoveDependency(t *testing.T) {
	s := newTestStore(t)
	a := createTask(t, s, "Task A")
	b := createTask(t, s, "Task B")

	s.AddDependency(b.ID, a.ID)
	if err := s.RemoveDependency(b.ID, a.ID); err != nil {
		t.Fatalf("RemoveDependency: %v", err)
	}
}

func TestNextReady_NoDeps(t *testing.T) {
	s := newTestStore(t)
	a := createTask(t, s, "Task A")

	ready, err := s.NextReady()
	if err != nil {
		t.Fatalf("NextReady: %v", err)
	}
	if ready.ID != a.ID {
		t.Errorf("NextReady ID = %d, want %d", ready.ID, a.ID)
	}
}

func TestNextReady_BlockedByDep(t *testing.T) {
	s := newTestStore(t)
	blocker := createTask(t, s, "Blocker")
	blocked := createTask(t, s, "Blocked")

	s.AddDependency(blocked.ID, blocker.ID)

	ready, err := s.NextReady()
	if err != nil {
		t.Fatalf("NextReady: %v", err)
	}
	if ready.ID != blocker.ID {
		t.Errorf("NextReady ID = %d, want %d (blocker)", ready.ID, blocker.ID)
	}
}

func TestNextReady_DepDone(t *testing.T) {
	s := newTestStore(t)
	blocker := createTask(t, s, "Blocker")
	blocked := createTask(t, s, "Blocked")

	s.AddDependency(blocked.ID, blocker.ID)
	s.Update(blocker.ID, map[string]interface{}{"stage": "done"})

	ready, err := s.NextReady()
	if err != nil {
		t.Fatalf("NextReady: %v", err)
	}
	if ready.ID != blocked.ID {
		t.Errorf("NextReady ID = %d, want %d (blocked)", ready.ID, blocked.ID)
	}
}

func TestCountByStage(t *testing.T) {
	s := newTestStore(t)
	s.Create("A", "", "")
	s.Create("B", "", "")
	task3, _ := s.Create("C", "", "")
	s.Update(task3.ID, map[string]interface{}{"stage": "active"})

	counts, err := s.CountByStage()
	if err != nil {
		t.Fatalf("CountByStage: %v", err)
	}
	if counts["backlog"] != 2 {
		t.Errorf("backlog = %d, want 2", counts["backlog"])
	}
	if counts["active"] != 1 {
		t.Errorf("active = %d, want 1", counts["active"])
	}
}
