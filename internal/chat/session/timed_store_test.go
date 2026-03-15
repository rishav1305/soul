package session

import (
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rishav1305/soul-v2/internal/chat/metrics"
)

// openTimedTestStore opens a test Store wrapped in a TimedStore with the given
// slowMs threshold. It registers cleanup for both the store and the logger.
func openTimedTestStore(t *testing.T, slowMs int64) (*TimedStore, string) {
	t.Helper()

	dbPath := filepath.Join(t.TempDir(), "timed_test.db")
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

	ts := NewTimedStore(store, logger, slowMs)

	t.Cleanup(func() {
		ts.Close()
		logger.Close()
	})

	return ts, filepath.Join(metricsDir, "metrics.jsonl")
}

// readEvents reads all JSONL events from the metrics file and returns them
// as a slice of parsed maps.
func readEvents(t *testing.T, metricsFile string) []map[string]interface{} {
	t.Helper()

	data, err := os.ReadFile(metricsFile)
	if err != nil {
		t.Fatalf("ReadFile(%q): %v", metricsFile, err)
	}

	var events []map[string]interface{}
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		if line == "" {
			continue
		}
		var ev map[string]interface{}
		if err := json.Unmarshal([]byte(line), &ev); err != nil {
			t.Fatalf("unmarshal event: %v\nline: %s", err, line)
		}
		events = append(events, ev)
	}
	return events
}

// countEventsByType counts events of the given type.
func countEventsByType(events []map[string]interface{}, eventType string) int {
	count := 0
	for _, ev := range events {
		if ev["event"] == eventType {
			count++
		}
	}
	return count
}

// findEventByType returns the first event matching eventType, or nil.
func findEventByType(events []map[string]interface{}, eventType string) map[string]interface{} {
	for _, ev := range events {
		if ev["event"] == eventType {
			return ev
		}
	}
	return nil
}

// TestTimedStore_LogsQueryEvents verifies that a db.query event is logged for
// CreateSession, including the method name and duration_ms fields.
func TestTimedStore_LogsQueryEvents(t *testing.T) {
	ts, metricsFile := openTimedTestStore(t, 10000) // high threshold — no slow events

	_, err := ts.CreateSession("test session")
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	events := readEvents(t, metricsFile)

	queryCount := countEventsByType(events, metrics.EventDBQuery)
	if queryCount == 0 {
		t.Fatal("expected at least one db.query event, got none")
	}

	// Find the CreateSession event.
	var createEvent map[string]interface{}
	for _, ev := range events {
		if ev["event"] == metrics.EventDBQuery {
			if data, ok := ev["data"].(map[string]interface{}); ok {
				if data["method"] == "CreateSession" {
					createEvent = ev
					break
				}
			}
		}
	}
	if createEvent == nil {
		t.Fatal("expected db.query event with method=CreateSession, not found")
	}

	data, ok := createEvent["data"].(map[string]interface{})
	if !ok {
		t.Fatal("event data is not a map")
	}

	if _, ok := data["duration_ms"]; !ok {
		t.Error("db.query event missing duration_ms field")
	}
	if data["method"] != "CreateSession" {
		t.Errorf("method = %v, want CreateSession", data["method"])
	}
}

// TestTimedStore_LogsSlowQueries verifies that a db.slow event is logged when
// a query exceeds the threshold. Setting threshold to -1 ensures every query
// triggers a slow event (since duration_ms >= 0 > -1).
func TestTimedStore_LogsSlowQueries(t *testing.T) {
	ts, metricsFile := openTimedTestStore(t, -1) // threshold = -1ms — every query is "slow"

	_, err := ts.CreateSession("slow test")
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	events := readEvents(t, metricsFile)

	slowCount := countEventsByType(events, metrics.EventDBSlow)
	if slowCount == 0 {
		t.Fatal("expected at least one db.slow event with threshold=-1ms, got none")
	}

	// Verify the slow event has threshold_ms set.
	slowEvent := findEventByType(events, metrics.EventDBSlow)
	if slowEvent == nil {
		t.Fatal("db.slow event not found")
	}

	data, ok := slowEvent["data"].(map[string]interface{})
	if !ok {
		t.Fatal("slow event data is not a map")
	}

	if _, ok := data["threshold_ms"]; !ok {
		t.Error("db.slow event missing threshold_ms field")
	}
	if _, ok := data["duration_ms"]; !ok {
		t.Error("db.slow event missing duration_ms field")
	}
}

// TestTimedStore_DelegatesToInner verifies a full CRUD cycle works correctly
// through the TimedStore wrapper — results must match what the inner store
// would produce.
func TestTimedStore_DelegatesToInner(t *testing.T) {
	ts, _ := openTimedTestStore(t, 10000)

	// Create
	sess, err := ts.CreateSession("integration test")
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	if sess.Title != "integration test" {
		t.Errorf("Title = %q, want %q", sess.Title, "integration test")
	}
	if sess.Status != StatusIdle {
		t.Errorf("Status = %q, want %q", sess.Status, StatusIdle)
	}

	// Get
	got, err := ts.GetSession(sess.ID)
	if err != nil {
		t.Fatalf("GetSession: %v", err)
	}
	if got.ID != sess.ID {
		t.Errorf("GetSession ID = %q, want %q", got.ID, sess.ID)
	}

	// List
	sessions, err := ts.ListSessions()
	if err != nil {
		t.Fatalf("ListSessions: %v", err)
	}
	if len(sessions) != 1 {
		t.Fatalf("ListSessions len = %d, want 1", len(sessions))
	}

	// UpdateSessionTitle
	updated, err := ts.UpdateSessionTitle(sess.ID, "renamed")
	if err != nil {
		t.Fatalf("UpdateSessionTitle: %v", err)
	}
	if updated.Title != "renamed" {
		t.Errorf("UpdateSessionTitle Title = %q, want %q", updated.Title, "renamed")
	}

	// UpdateSessionStatus (idle → running)
	if err := ts.UpdateSessionStatus(sess.ID, StatusRunning); err != nil {
		t.Fatalf("UpdateSessionStatus: %v", err)
	}
	got, _ = ts.GetSession(sess.ID)
	if got.Status != StatusRunning {
		t.Errorf("Status after update = %q, want %q", got.Status, StatusRunning)
	}

	// AddMessage
	msg, err := ts.AddMessage(sess.ID, "user", "hello")
	if err != nil {
		t.Fatalf("AddMessage: %v", err)
	}
	if msg.Content != "hello" {
		t.Errorf("AddMessage Content = %q, want %q", msg.Content, "hello")
	}

	// GetMessages
	msgs, err := ts.GetMessages(sess.ID)
	if err != nil {
		t.Fatalf("GetMessages: %v", err)
	}
	if len(msgs) != 1 {
		t.Fatalf("GetMessages len = %d, want 1", len(msgs))
	}
	if msgs[0].ID != msg.ID {
		t.Errorf("GetMessages[0].ID = %q, want %q", msgs[0].ID, msg.ID)
	}

	// DeleteSession (complete lifecycle first: running → completed → idle → delete)
	if err := ts.UpdateSessionStatus(sess.ID, StatusCompleted); err != nil {
		t.Fatalf("UpdateSessionStatus completed: %v", err)
	}
	if err := ts.UpdateSessionStatus(sess.ID, StatusIdle); err != nil {
		t.Fatalf("UpdateSessionStatus idle: %v", err)
	}
	if err := ts.DeleteSession(sess.ID); err != nil {
		t.Fatalf("DeleteSession: %v", err)
	}

	// Verify deleted
	_, err = ts.GetSession(sess.ID)
	if err == nil {
		t.Fatal("expected error after DeleteSession, got nil")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("error = %q, want 'not found'", err)
	}
}

// TestTimedStore_RunInTransaction verifies that RunInTransaction works through
// the wrapper and that a db.query event is logged for the transaction.
func TestTimedStore_RunInTransaction(t *testing.T) {
	ts, metricsFile := openTimedTestStore(t, 10000)

	// Create a session to add messages to.
	sess, err := ts.CreateSession("tx test")
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	// Run a transaction that adds a message.
	err = ts.RunInTransaction(func(tx *sql.Tx) error {
		_, err := ts.AddMessageTx(tx, sess.ID, "user", "tx message")
		return err
	})
	if err != nil {
		t.Fatalf("RunInTransaction: %v", err)
	}

	// Verify the message was committed.
	msgs, err := ts.GetMessages(sess.ID)
	if err != nil {
		t.Fatalf("GetMessages: %v", err)
	}
	if len(msgs) != 1 {
		t.Fatalf("GetMessages len = %d, want 1", len(msgs))
	}
	if msgs[0].Content != "tx message" {
		t.Errorf("Content = %q, want %q", msgs[0].Content, "tx message")
	}

	// Verify RunInTransaction logged a db.query event.
	events := readEvents(t, metricsFile)

	var txEvent map[string]interface{}
	for _, ev := range events {
		if ev["event"] == metrics.EventDBQuery {
			if data, ok := ev["data"].(map[string]interface{}); ok {
				if data["method"] == "RunInTransaction" {
					txEvent = ev
					break
				}
			}
		}
	}
	if txEvent == nil {
		t.Fatal("expected db.query event with method=RunInTransaction, not found")
	}

	// Verify AddMessageTx also logged a db.query event.
	var addMsgTxEvent map[string]interface{}
	for _, ev := range events {
		if ev["event"] == metrics.EventDBQuery {
			if data, ok := ev["data"].(map[string]interface{}); ok {
				if data["method"] == "AddMessageTx" {
					addMsgTxEvent = ev
					break
				}
			}
		}
	}
	if addMsgTxEvent == nil {
		t.Fatal("expected db.query event with method=AddMessageTx, not found")
	}
}

// TestTimedStore_Inner verifies that Inner() returns the wrapped *Store.
func TestTimedStore_Inner(t *testing.T) {
	ts, _ := openTimedTestStore(t, 1000)
	inner := ts.Inner()
	if inner == nil {
		t.Fatal("Inner() returned nil")
	}
}

// TestTimedStore_SessionIDInQueryEvent verifies that db.query events for
// session-scoped methods include the session_id field.
func TestTimedStore_SessionIDInQueryEvent(t *testing.T) {
	ts, metricsFile := openTimedTestStore(t, 10000)

	sess, err := ts.CreateSession("id test")
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	_, err = ts.GetSession(sess.ID)
	if err != nil {
		t.Fatalf("GetSession: %v", err)
	}

	events := readEvents(t, metricsFile)

	// Find GetSession event and verify session_id is present.
	for _, ev := range events {
		if ev["event"] != metrics.EventDBQuery {
			continue
		}
		data, ok := ev["data"].(map[string]interface{})
		if !ok {
			continue
		}
		if data["method"] != "GetSession" {
			continue
		}
		if data["session_id"] != sess.ID {
			t.Errorf("session_id = %v, want %q", data["session_id"], sess.ID)
		}
		return
	}
	t.Fatal("GetSession db.query event not found")
}
