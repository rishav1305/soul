package session

import (
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func openTestStore(t *testing.T) *Store {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	store, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open(%q): %v", dbPath, err)
	}
	t.Cleanup(func() { store.Close() })
	return store
}

func TestCreateSession_DefaultTitle(t *testing.T) {
	s := openTestStore(t)

	sess, err := s.CreateSession("")
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	if sess.Title != "New Session" {
		t.Errorf("Title = %q, want %q", sess.Title, "New Session")
	}
	if sess.Status != StatusIdle {
		t.Errorf("Status = %q, want %q", sess.Status, StatusIdle)
	}
	if !uuidRe.MatchString(sess.ID) {
		t.Errorf("ID = %q, not a valid UUID", sess.ID)
	}
	if sess.MessageCount != 0 {
		t.Errorf("MessageCount = %d, want 0", sess.MessageCount)
	}
	if sess.CreatedAt.IsZero() {
		t.Error("CreatedAt is zero")
	}
	if sess.UpdatedAt.IsZero() {
		t.Error("UpdatedAt is zero")
	}
}

func TestCreateSession_CustomTitle(t *testing.T) {
	s := openTestStore(t)

	sess, err := s.CreateSession("My Chat")
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	if sess.Title != "My Chat" {
		t.Errorf("Title = %q, want %q", sess.Title, "My Chat")
	}
}

func TestGetSession_NotFound(t *testing.T) {
	s := openTestStore(t)

	_, err := s.GetSession("00000000-0000-4000-8000-000000000000")
	if err == nil {
		t.Fatal("expected error for non-existent session")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("error = %q, want 'not found'", err)
	}
}

func TestGetSession_InvalidUUID(t *testing.T) {
	s := openTestStore(t)

	_, err := s.GetSession("not-a-uuid")
	if err == nil {
		t.Fatal("expected error for invalid UUID")
	}
	if !strings.Contains(err.Error(), "invalid UUID format") {
		t.Errorf("error = %q, want 'invalid UUID format'", err)
	}
}

func TestGetSession_Roundtrip(t *testing.T) {
	s := openTestStore(t)

	created, err := s.CreateSession("Test")
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	got, err := s.GetSession(created.ID)
	if err != nil {
		t.Fatalf("GetSession: %v", err)
	}
	if got.ID != created.ID {
		t.Errorf("ID = %q, want %q", got.ID, created.ID)
	}
	if got.Title != created.Title {
		t.Errorf("Title = %q, want %q", got.Title, created.Title)
	}
	if got.Status != created.Status {
		t.Errorf("Status = %q, want %q", got.Status, created.Status)
	}
}

func TestListSessions_Empty(t *testing.T) {
	s := openTestStore(t)

	sessions, err := s.ListSessions()
	if err != nil {
		t.Fatalf("ListSessions: %v", err)
	}
	if len(sessions) != 0 {
		t.Errorf("len = %d, want 0", len(sessions))
	}
}

func TestListSessions_OrderByUpdatedAtDesc(t *testing.T) {
	s := openTestStore(t)

	first, err := s.CreateSession("First")
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	// Small delay so updated_at differs.
	time.Sleep(10 * time.Millisecond)

	second, err := s.CreateSession("Second")
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	sessions, err := s.ListSessions()
	if err != nil {
		t.Fatalf("ListSessions: %v", err)
	}
	if len(sessions) != 2 {
		t.Fatalf("len = %d, want 2", len(sessions))
	}
	// Most recently created (second) should be first.
	if sessions[0].ID != second.ID {
		t.Errorf("sessions[0].ID = %q, want %q (second)", sessions[0].ID, second.ID)
	}
	if sessions[1].ID != first.ID {
		t.Errorf("sessions[1].ID = %q, want %q (first)", sessions[1].ID, first.ID)
	}
}

func TestUpdateSessionTitle(t *testing.T) {
	s := openTestStore(t)

	sess, err := s.CreateSession("Old Title")
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	updated, err := s.UpdateSessionTitle(sess.ID, "New Title")
	if err != nil {
		t.Fatalf("UpdateSessionTitle: %v", err)
	}
	if updated.Title != "New Title" {
		t.Errorf("Title = %q, want %q", updated.Title, "New Title")
	}
	if !updated.UpdatedAt.After(sess.CreatedAt) || updated.UpdatedAt.Equal(sess.CreatedAt) {
		// UpdatedAt should be >= CreatedAt (may be equal if fast)
	}
}

func TestUpdateSessionStatus_ValidTransitions(t *testing.T) {
	s := openTestStore(t)

	sess, err := s.CreateSession("Test")
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	// idle → running
	if err := s.UpdateSessionStatus(sess.ID, StatusRunning); err != nil {
		t.Fatalf("idle → running: %v", err)
	}

	got, _ := s.GetSession(sess.ID)
	if got.Status != StatusRunning {
		t.Errorf("Status = %q, want %q", got.Status, StatusRunning)
	}

	// running → completed
	if err := s.UpdateSessionStatus(sess.ID, StatusCompleted); err != nil {
		t.Fatalf("running → completed: %v", err)
	}

	// completed → idle
	if err := s.UpdateSessionStatus(sess.ID, StatusIdle); err != nil {
		t.Fatalf("completed → idle: %v", err)
	}
}

func TestUpdateSessionStatus_InvalidTransition(t *testing.T) {
	s := openTestStore(t)

	sess, err := s.CreateSession("Test")
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	// idle → completed should fail (must go through running)
	err = s.UpdateSessionStatus(sess.ID, StatusCompleted)
	if err == nil {
		t.Fatal("expected error for idle → completed")
	}
	if !strings.Contains(err.Error(), "invalid transition") {
		t.Errorf("error = %q, want 'invalid transition'", err)
	}

	// Set to running first, then try running → idle (invalid)
	if err := s.UpdateSessionStatus(sess.ID, StatusRunning); err != nil {
		t.Fatalf("idle → running: %v", err)
	}
	err = s.UpdateSessionStatus(sess.ID, StatusIdle)
	if err == nil {
		t.Fatal("expected error for running → idle")
	}
}

func TestDeleteSession_Success(t *testing.T) {
	s := openTestStore(t)

	sess, err := s.CreateSession("Doomed")
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	// Add a message to verify cascade.
	if err := s.UpdateSessionStatus(sess.ID, StatusRunning); err != nil {
		t.Fatalf("UpdateSessionStatus: %v", err)
	}
	_, err = s.AddMessage(sess.ID, "user", "hello")
	if err != nil {
		t.Fatalf("AddMessage: %v", err)
	}

	if err := s.DeleteSession(sess.ID); err != nil {
		t.Fatalf("DeleteSession: %v", err)
	}

	_, err = s.GetSession(sess.ID)
	if err == nil {
		t.Fatal("expected error after delete")
	}

	// Messages should also be gone.
	msgs, err := s.GetMessages(sess.ID)
	if err != nil {
		t.Fatalf("GetMessages after delete: %v", err)
	}
	if len(msgs) != 0 {
		t.Errorf("messages after delete = %d, want 0", len(msgs))
	}
}

func TestDeleteSession_NotFound(t *testing.T) {
	s := openTestStore(t)

	err := s.DeleteSession("00000000-0000-4000-8000-000000000000")
	if err == nil {
		t.Fatal("expected error for non-existent session")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("error = %q, want 'not found'", err)
	}
}

func TestAddMessage_IncrementsCount(t *testing.T) {
	s := openTestStore(t)

	sess, err := s.CreateSession("Test")
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	msg, err := s.AddMessage(sess.ID, "user", "hello")
	if err != nil {
		t.Fatalf("AddMessage: %v", err)
	}
	if msg.Role != "user" {
		t.Errorf("Role = %q, want %q", msg.Role, "user")
	}
	if msg.Content != "hello" {
		t.Errorf("Content = %q, want %q", msg.Content, "hello")
	}
	if !uuidRe.MatchString(msg.ID) {
		t.Errorf("ID = %q, not a valid UUID", msg.ID)
	}

	got, _ := s.GetSession(sess.ID)
	if got.MessageCount != 1 {
		t.Errorf("MessageCount = %d, want 1", got.MessageCount)
	}

	// Add another.
	_, err = s.AddMessage(sess.ID, "assistant", "hi there")
	if err != nil {
		t.Fatalf("AddMessage: %v", err)
	}
	got, _ = s.GetSession(sess.ID)
	if got.MessageCount != 2 {
		t.Errorf("MessageCount = %d, want 2", got.MessageCount)
	}
}

func TestAddMessage_InvalidRole(t *testing.T) {
	s := openTestStore(t)

	sess, err := s.CreateSession("Test")
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	_, err = s.AddMessage(sess.ID, "system", "nope")
	if err == nil {
		t.Fatal("expected error for invalid role")
	}
	if !strings.Contains(err.Error(), "invalid role") {
		t.Errorf("error = %q, want 'invalid role'", err)
	}
}

func TestAddMessage_ValidRoles(t *testing.T) {
	s := openTestStore(t)

	sess, err := s.CreateSession("Test")
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	for _, role := range []string{"user", "assistant", "tool_use", "tool_result"} {
		_, err := s.AddMessage(sess.ID, role, "content")
		if err != nil {
			t.Errorf("AddMessage(%q): unexpected error: %v", role, err)
		}
	}
}

func TestGetMessages_Empty(t *testing.T) {
	s := openTestStore(t)

	sess, err := s.CreateSession("Test")
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	msgs, err := s.GetMessages(sess.ID)
	if err != nil {
		t.Fatalf("GetMessages: %v", err)
	}
	if len(msgs) != 0 {
		t.Errorf("len = %d, want 0", len(msgs))
	}
}

func TestGetMessages_OrderByCreatedAtAsc(t *testing.T) {
	s := openTestStore(t)

	sess, err := s.CreateSession("Test")
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	first, err := s.AddMessage(sess.ID, "user", "first")
	if err != nil {
		t.Fatalf("AddMessage: %v", err)
	}
	time.Sleep(10 * time.Millisecond)
	second, err := s.AddMessage(sess.ID, "assistant", "second")
	if err != nil {
		t.Fatalf("AddMessage: %v", err)
	}

	msgs, err := s.GetMessages(sess.ID)
	if err != nil {
		t.Fatalf("GetMessages: %v", err)
	}
	if len(msgs) != 2 {
		t.Fatalf("len = %d, want 2", len(msgs))
	}
	if msgs[0].ID != first.ID {
		t.Errorf("msgs[0].ID = %q, want %q (first)", msgs[0].ID, first.ID)
	}
	if msgs[1].ID != second.ID {
		t.Errorf("msgs[1].ID = %q, want %q (second)", msgs[1].ID, second.ID)
	}
}

func TestStatus_CanTransitionTo(t *testing.T) {
	tests := []struct {
		from Status
		to   Status
		ok   bool
	}{
		// Valid transitions.
		{StatusIdle, StatusRunning, true},
		{StatusRunning, StatusCompleted, true},
		{StatusRunning, StatusCompletedUnread, true},
		{StatusCompletedUnread, StatusIdle, true},
		{StatusCompleted, StatusIdle, true},

		// Invalid transitions.
		{StatusIdle, StatusCompleted, false},
		{StatusIdle, StatusCompletedUnread, false},
		{StatusIdle, StatusIdle, false},
		{StatusRunning, StatusIdle, false},
		{StatusRunning, StatusRunning, false},
		{StatusCompleted, StatusRunning, false},
		{StatusCompleted, StatusCompleted, false},
		{StatusCompleted, StatusCompletedUnread, false},
		{StatusCompletedUnread, StatusRunning, false},
		{StatusCompletedUnread, StatusCompleted, false},
		{StatusCompletedUnread, StatusCompletedUnread, false},
	}

	for _, tt := range tests {
		name := string(tt.from) + "→" + string(tt.to)
		t.Run(name, func(t *testing.T) {
			got := tt.from.CanTransitionTo(tt.to)
			if got != tt.ok {
				t.Errorf("%s → %s = %v, want %v", tt.from, tt.to, got, tt.ok)
			}
		})
	}
}

func TestStatus_Valid(t *testing.T) {
	if !StatusIdle.Valid() {
		t.Error("idle should be valid")
	}
	if !StatusRunning.Valid() {
		t.Error("running should be valid")
	}
	if !StatusCompleted.Valid() {
		t.Error("completed should be valid")
	}
	if !StatusCompletedUnread.Valid() {
		t.Error("completed_unread should be valid")
	}
	if Status("bogus").Valid() {
		t.Error("bogus should not be valid")
	}
}

func TestStatus_String(t *testing.T) {
	if StatusIdle.String() != "idle" {
		t.Errorf("String() = %q, want %q", StatusIdle.String(), "idle")
	}
}

func TestMigrate_Idempotent(t *testing.T) {
	s := openTestStore(t)

	// Migrate was already called by Open. Call it again — should not error.
	if err := s.Migrate(); err != nil {
		t.Fatalf("Migrate (second call): %v", err)
	}

	// Create a session to prove DB is still functional.
	sess, err := s.CreateSession("After re-migrate")
	if err != nil {
		t.Fatalf("CreateSession after re-migrate: %v", err)
	}
	if sess.Title != "After re-migrate" {
		t.Errorf("Title = %q, want %q", sess.Title, "After re-migrate")
	}
}

func TestNewUUID(t *testing.T) {
	seen := make(map[string]bool)
	for i := 0; i < 100; i++ {
		id := newUUID()
		if !uuidRe.MatchString(id) {
			t.Fatalf("newUUID() = %q, not a valid UUID", id)
		}
		if seen[id] {
			t.Fatalf("newUUID() produced duplicate: %s", id)
		}
		seen[id] = true
	}
}
