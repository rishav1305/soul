package session

import (
	"path/filepath"
	"database/sql"
	"strings"
	"testing"

	"github.com/rishav1305/soul/internal/chat/metrics"
)

func openCoverageTestStore(t *testing.T) *Store {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "coverage.db")
	s, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

// --- UpdateSessionTitle error paths ---

func TestUpdateSessionTitle_InvalidUUID(t *testing.T) {
	s := openCoverageTestStore(t)
	_, err := s.UpdateSessionTitle("not-a-uuid", "title")
	if err == nil {
		t.Error("expected error for invalid UUID")
	}
	if !strings.Contains(err.Error(), "invalid UUID") {
		t.Errorf("error = %q, want 'invalid UUID'", err)
	}
}

func TestUpdateSessionTitle_NotFound(t *testing.T) {
	s := openCoverageTestStore(t)
	_, err := s.UpdateSessionTitle("11111111-1111-1111-1111-111111111111", "title")
	if err == nil {
		t.Error("expected error for nonexistent session")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("error = %q, want 'not found'", err)
	}
}

// --- UpdateSessionStatus error paths ---

func TestUpdateSessionStatus_InvalidUUID(t *testing.T) {
	s := openCoverageTestStore(t)
	err := s.UpdateSessionStatus("bad-uuid", StatusRunning)
	if err == nil {
		t.Error("expected error for invalid UUID")
	}
}

func TestUpdateSessionStatus_InvalidStatus(t *testing.T) {
	s := openCoverageTestStore(t)
	err := s.UpdateSessionStatus("11111111-1111-1111-1111-111111111111", Status("bogus"))
	if err == nil {
		t.Error("expected error for invalid status")
	}
}

func TestUpdateSessionStatus_NotFound(t *testing.T) {
	s := openCoverageTestStore(t)
	err := s.UpdateSessionStatus("11111111-1111-1111-1111-111111111111", StatusRunning)
	if err == nil {
		t.Error("expected error for nonexistent session")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("error = %q, want 'not found'", err)
	}
}

// --- DeleteSession error paths ---

func TestDeleteSession_InvalidUUID(t *testing.T) {
	s := openCoverageTestStore(t)
	err := s.DeleteSession("bad-uuid")
	if err == nil {
		t.Error("expected error for invalid UUID")
	}
}

// --- AddMessage error paths ---

func TestAddMessage_InvalidUUID(t *testing.T) {
	s := openCoverageTestStore(t)
	_, err := s.AddMessage("bad-uuid", "user", "hello")
	if err == nil {
		t.Error("expected error for invalid UUID")
	}
}

func TestAddMessage_NonexistentSession(t *testing.T) {
	s := openCoverageTestStore(t)
	_, err := s.AddMessage("11111111-1111-1111-1111-111111111111", "user", "hello")
	if err == nil {
		t.Error("expected error for nonexistent session")
	}
}

// --- GetMessages error paths ---

func TestGetMessages_InvalidUUID(t *testing.T) {
	s := openCoverageTestStore(t)
	_, err := s.GetMessages("bad-uuid")
	if err == nil {
		t.Error("expected error for invalid UUID")
	}
}

// --- AddMessageTx error paths ---

func TestAddMessageTx_InvalidRole(t *testing.T) {
	s := openCoverageTestStore(t)
	sess, err := s.CreateSession("tx test")
	if err != nil {
		t.Fatal(err)
	}

	err = s.RunInTransaction(func(tx *sql.Tx) error {
		_, err := s.AddMessageTx(tx, sess.ID, "bogus-role", "hello")
		return err
	})
	if err == nil {
		t.Error("expected error for invalid role")
	}
}

func TestAddMessageTx_InvalidUUID(t *testing.T) {
	s := openCoverageTestStore(t)

	err := s.RunInTransaction(func(tx *sql.Tx) error {
		_, err := s.AddMessageTx(tx, "bad-uuid", "user", "hello")
		return err
	})
	if err == nil {
		t.Error("expected error for invalid UUID")
	}
}

func TestAddMessageTx_LongContent(t *testing.T) {
	s := openCoverageTestStore(t)
	sess, err := s.CreateSession("tx long")
	if err != nil {
		t.Fatal(err)
	}

	// Content > 100 chars to trigger truncation.
	longContent := strings.Repeat("word ", 30) // 150 chars

	err = s.RunInTransaction(func(tx *sql.Tx) error {
		_, err := s.AddMessageTx(tx, sess.ID, "user", longContent)
		return err
	})
	if err != nil {
		t.Fatalf("AddMessageTx: %v", err)
	}

	got, err := s.GetSession(sess.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.LastMessage) > 110 {
		t.Errorf("last_message should be truncated, got len=%d", len(got.LastMessage))
	}
}

// --- ResetUnreadCount error paths ---

func TestResetUnreadCount_InvalidUUID(t *testing.T) {
	s := openCoverageTestStore(t)
	err := s.ResetUnreadCount("bad-uuid")
	if err == nil {
		t.Error("expected error for invalid UUID")
	}
}

// --- SetLastMessage error paths ---

func TestSetLastMessage_InvalidUUID(t *testing.T) {
	s := openCoverageTestStore(t)
	err := s.SetLastMessage("bad-uuid", "content")
	if err == nil {
		t.Error("expected error for invalid UUID")
	}
}

func TestSetLastMessage_LongContentTruncation(t *testing.T) {
	s := openCoverageTestStore(t)
	sess, err := s.CreateSession("truncation test")
	if err != nil {
		t.Fatal(err)
	}

	longContent := strings.Repeat("word ", 30) // 150 chars
	err = s.SetLastMessage(sess.ID, longContent)
	if err != nil {
		t.Fatal(err)
	}

	got, err := s.GetSession(sess.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.LastMessage) > 110 {
		t.Errorf("last_message should be truncated, got len=%d", len(got.LastMessage))
	}
	if !strings.HasSuffix(got.LastMessage, "...") {
		t.Errorf("truncated message should end with ..., got %q", got.LastMessage)
	}
}

// --- SetProduct error paths ---

func TestSetProduct_InvalidUUID(t *testing.T) {
	s := openCoverageTestStore(t)
	err := s.SetProduct("bad-uuid", "tasks")
	// SetProduct doesn't validate UUID format — it validates product name.
	// An invalid UUID just won't match any row, resulting in "not found".
	if err == nil {
		t.Error("expected error for invalid UUID")
	}
}

func TestSetProduct_NotFound(t *testing.T) {
	s := openCoverageTestStore(t)
	err := s.SetProduct("11111111-1111-1111-1111-111111111111", "tasks")
	if err == nil {
		t.Error("expected error for nonexistent session")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("error = %q, want 'not found'", err)
	}
}

// --- TimedStore: ResetUnreadCount, SetLastMessage, SetProduct ---

func openCoverageTimedStore(t *testing.T) *TimedStore {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "timed_coverage.db")
	store, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	metricsDir := t.TempDir()
	logger, err := metrics.NewEventLogger(metricsDir, "")
	if err != nil {
		store.Close()
		t.Fatalf("NewEventLogger: %v", err)
	}
	ts := NewTimedStore(store, logger, 10000)
	t.Cleanup(func() {
		ts.Close()
		logger.Close()
	})
	return ts
}

func TestTimedStore_ResetUnreadCount(t *testing.T) {
	ts := openCoverageTimedStore(t)

	sess, err := ts.CreateSession("unread test")
	if err != nil {
		t.Fatal(err)
	}

	// Add a message to increment unread count.
	_, err = ts.AddMessage(sess.ID, "user", "hello")
	if err != nil {
		t.Fatal(err)
	}

	got, _ := ts.GetSession(sess.ID)
	if got.UnreadCount != 1 {
		t.Fatalf("UnreadCount = %d, want 1", got.UnreadCount)
	}

	err = ts.ResetUnreadCount(sess.ID)
	if err != nil {
		t.Fatalf("ResetUnreadCount: %v", err)
	}

	got, _ = ts.GetSession(sess.ID)
	if got.UnreadCount != 0 {
		t.Errorf("UnreadCount after reset = %d, want 0", got.UnreadCount)
	}
}

func TestTimedStore_SetLastMessage(t *testing.T) {
	ts := openCoverageTimedStore(t)

	sess, err := ts.CreateSession("last message test")
	if err != nil {
		t.Fatal(err)
	}

	err = ts.SetLastMessage(sess.ID, "latest message")
	if err != nil {
		t.Fatalf("SetLastMessage: %v", err)
	}

	got, _ := ts.GetSession(sess.ID)
	if got.LastMessage != "latest message" {
		t.Errorf("LastMessage = %q, want %q", got.LastMessage, "latest message")
	}
}

func TestTimedStore_SetProduct(t *testing.T) {
	ts := openCoverageTimedStore(t)

	sess, err := ts.CreateSession("product test")
	if err != nil {
		t.Fatal(err)
	}

	err = ts.SetProduct(sess.ID, "tutor")
	if err != nil {
		t.Fatalf("SetProduct: %v", err)
	}

	got, _ := ts.GetSession(sess.ID)
	if got.Product != "tutor" {
		t.Errorf("Product = %q, want %q", got.Product, "tutor")
	}
}

// --- Open error path ---

func TestOpen_InvalidPath(t *testing.T) {
	_, err := Open("/nonexistent/deep/path/db.sqlite")
	if err == nil {
		t.Error("expected error for invalid path")
	}
}

// --- Closed store operations ---

func TestClosedStore_Operations(t *testing.T) {
	s := openCoverageTestStore(t)
	s.Close()

	// All operations should return errors on closed store.
	_, err := s.CreateSession("test")
	if err == nil {
		t.Error("expected error from CreateSession on closed store")
	}
	_, err = s.GetSession("11111111-1111-1111-1111-111111111111")
	if err == nil {
		t.Error("expected error from GetSession on closed store")
	}
	_, err = s.ListSessions()
	if err == nil {
		t.Error("expected error from ListSessions on closed store")
	}
	_, err = s.UpdateSessionTitle("11111111-1111-1111-1111-111111111111", "t")
	if err == nil {
		t.Error("expected error from UpdateSessionTitle on closed store")
	}
	err = s.UpdateSessionStatus("11111111-1111-1111-1111-111111111111", StatusRunning)
	if err == nil {
		t.Error("expected error from UpdateSessionStatus on closed store")
	}
	err = s.DeleteSession("11111111-1111-1111-1111-111111111111")
	if err == nil {
		t.Error("expected error from DeleteSession on closed store")
	}
	_, err = s.GetMessages("11111111-1111-1111-1111-111111111111")
	if err == nil {
		t.Error("expected error from GetMessages on closed store")
	}
	err = s.ResetUnreadCount("11111111-1111-1111-1111-111111111111")
	if err == nil {
		t.Error("expected error from ResetUnreadCount on closed store")
	}
	err = s.SetLastMessage("11111111-1111-1111-1111-111111111111", "msg")
	if err == nil {
		t.Error("expected error from SetLastMessage on closed store")
	}
	err = s.SetProduct("11111111-1111-1111-1111-111111111111", "tasks")
	if err == nil {
		t.Error("expected error from SetProduct on closed store")
	}
}
