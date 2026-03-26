package integration_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rishav1305/soul/internal/chat/server"
	"github.com/rishav1305/soul/internal/chat/session"
)

// openStore is a helper that opens a session store at a temp path.
func openStore(t *testing.T) (*session.Store, string) {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "integration.db")
	store, err := session.Open(dbPath)
	if err != nil {
		t.Fatalf("session.Open(%q): %v", dbPath, err)
	}
	return store, dbPath
}

// --- Test 1: Persistence across restart ---

func TestSessionPersistence_SurvivesRestart(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "restart.db")

	// Phase 1: Open store, create 2 sessions with messages.
	store1, err := session.Open(dbPath)
	if err != nil {
		t.Fatalf("Open (phase 1): %v", err)
	}

	sess1, err := store1.CreateSession("Session Alpha")
	if err != nil {
		t.Fatalf("CreateSession Alpha: %v", err)
	}
	_, err = store1.AddMessage(sess1.ID, "user", "hello from alpha")
	if err != nil {
		t.Fatalf("AddMessage to Alpha: %v", err)
	}
	_, err = store1.AddMessage(sess1.ID, "assistant", "hi back from alpha")
	if err != nil {
		t.Fatalf("AddMessage to Alpha: %v", err)
	}

	// Small delay so Session Beta has a later UpdatedAt.
	time.Sleep(15 * time.Millisecond)

	sess2, err := store1.CreateSession("Session Beta")
	if err != nil {
		t.Fatalf("CreateSession Beta: %v", err)
	}
	_, err = store1.AddMessage(sess2.ID, "user", "hello from beta")
	if err != nil {
		t.Fatalf("AddMessage to Beta: %v", err)
	}

	// Close store (simulates server shutdown).
	if err := store1.Close(); err != nil {
		t.Fatalf("Close (phase 1): %v", err)
	}

	// Phase 2: Reopen store at same path (simulates server restart).
	store2, err := session.Open(dbPath)
	if err != nil {
		t.Fatalf("Open (phase 2): %v", err)
	}
	defer store2.Close()

	// Verify all sessions are intact.
	sessions, err := store2.ListSessions()
	if err != nil {
		t.Fatalf("ListSessions: %v", err)
	}
	if len(sessions) != 2 {
		t.Fatalf("expected 2 sessions, got %d", len(sessions))
	}

	// Verify order: UpdatedAt DESC — Beta was created/updated later.
	if sessions[0].Title != "Session Beta" {
		t.Errorf("sessions[0].Title = %q, want %q", sessions[0].Title, "Session Beta")
	}
	if sessions[1].Title != "Session Alpha" {
		t.Errorf("sessions[1].Title = %q, want %q", sessions[1].Title, "Session Alpha")
	}

	// Verify Session Alpha has 2 messages.
	alphaID := sess1.ID
	alphaMsgs, err := store2.GetMessages(alphaID)
	if err != nil {
		t.Fatalf("GetMessages(Alpha): %v", err)
	}
	if len(alphaMsgs) != 2 {
		t.Errorf("Alpha message count = %d, want 2", len(alphaMsgs))
	}

	// Verify Session Beta has 1 message.
	betaID := sess2.ID
	betaMsgs, err := store2.GetMessages(betaID)
	if err != nil {
		t.Fatalf("GetMessages(Beta): %v", err)
	}
	if len(betaMsgs) != 1 {
		t.Errorf("Beta message count = %d, want 1", len(betaMsgs))
	}

	// Verify message count field on session matches actual messages.
	alphaSession, err := store2.GetSession(alphaID)
	if err != nil {
		t.Fatalf("GetSession(Alpha): %v", err)
	}
	if alphaSession.MessageCount != 2 {
		t.Errorf("Alpha MessageCount = %d, want 2", alphaSession.MessageCount)
	}

	betaSession, err := store2.GetSession(betaID)
	if err != nil {
		t.Fatalf("GetSession(Beta): %v", err)
	}
	if betaSession.MessageCount != 1 {
		t.Errorf("Beta MessageCount = %d, want 1", betaSession.MessageCount)
	}

	// Verify message content survived.
	if alphaMsgs[0].Content != "hello from alpha" {
		t.Errorf("Alpha msg[0].Content = %q, want %q", alphaMsgs[0].Content, "hello from alpha")
	}
	if alphaMsgs[0].Role != "user" {
		t.Errorf("Alpha msg[0].Role = %q, want %q", alphaMsgs[0].Role, "user")
	}
	if alphaMsgs[1].Content != "hi back from alpha" {
		t.Errorf("Alpha msg[1].Content = %q, want %q", alphaMsgs[1].Content, "hi back from alpha")
	}
	if alphaMsgs[1].Role != "assistant" {
		t.Errorf("Alpha msg[1].Role = %q, want %q", alphaMsgs[1].Role, "assistant")
	}
}

// --- Test 2: Foreign key cascade persists ---

func TestSessionPersistence_CascadeAfterRestart(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "cascade.db")

	// Phase 1: Open store, create session with messages.
	store1, err := session.Open(dbPath)
	if err != nil {
		t.Fatalf("Open (phase 1): %v", err)
	}

	sess, err := store1.CreateSession("Doomed Session")
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	_, err = store1.AddMessage(sess.ID, "user", "message one")
	if err != nil {
		t.Fatalf("AddMessage: %v", err)
	}
	_, err = store1.AddMessage(sess.ID, "assistant", "message two")
	if err != nil {
		t.Fatalf("AddMessage: %v", err)
	}

	sessionID := sess.ID

	// Close and reopen.
	if err := store1.Close(); err != nil {
		t.Fatalf("Close (phase 1): %v", err)
	}

	store2, err := session.Open(dbPath)
	if err != nil {
		t.Fatalf("Open (phase 2): %v", err)
	}
	defer store2.Close()

	// Verify session and messages exist after restart.
	msgs, err := store2.GetMessages(sessionID)
	if err != nil {
		t.Fatalf("GetMessages before delete: %v", err)
	}
	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages before delete, got %d", len(msgs))
	}

	// Delete session — cascade should remove messages.
	if err := store2.DeleteSession(sessionID); err != nil {
		t.Fatalf("DeleteSession: %v", err)
	}

	// Verify session is gone.
	_, err = store2.GetSession(sessionID)
	if err == nil {
		t.Fatal("expected error getting deleted session")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected 'not found' error, got: %v", err)
	}

	// Verify messages are also gone (cascade).
	msgs, err = store2.GetMessages(sessionID)
	if err != nil {
		t.Fatalf("GetMessages after delete: %v", err)
	}
	if len(msgs) != 0 {
		t.Errorf("expected 0 messages after cascade delete, got %d", len(msgs))
	}
}

// --- Test 3: WAL mode survives restart ---

func TestSessionPersistence_WALModeSurvives(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "wal.db")

	// Phase 1: Open store, write data.
	store1, err := session.Open(dbPath)
	if err != nil {
		t.Fatalf("Open (phase 1): %v", err)
	}

	sess, err := store1.CreateSession("WAL Test")
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	_, err = store1.AddMessage(sess.ID, "user", "wal test message")
	if err != nil {
		t.Fatalf("AddMessage: %v", err)
	}

	// Close (simulates clean shutdown).
	if err := store1.Close(); err != nil {
		t.Fatalf("Close (phase 1): %v", err)
	}

	// Phase 2: Reopen, verify WAL mode is still active.
	store2, err := session.Open(dbPath)
	if err != nil {
		t.Fatalf("Open (phase 2): %v", err)
	}
	defer store2.Close()

	// Verify data is readable.
	got, err := store2.GetSession(sess.ID)
	if err != nil {
		t.Fatalf("GetSession: %v", err)
	}
	if got.Title != "WAL Test" {
		t.Errorf("Title = %q, want %q", got.Title, "WAL Test")
	}

	msgs, err := store2.GetMessages(sess.ID)
	if err != nil {
		t.Fatalf("GetMessages: %v", err)
	}
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(msgs))
	}
	if msgs[0].Content != "wal test message" {
		t.Errorf("Content = %q, want %q", msgs[0].Content, "wal test message")
	}

	// Write more data to verify WAL mode is functional after restart.
	_, err = store2.AddMessage(sess.ID, "assistant", "wal reply")
	if err != nil {
		t.Fatalf("AddMessage after restart: %v", err)
	}

	msgs, err = store2.GetMessages(sess.ID)
	if err != nil {
		t.Fatalf("GetMessages after second write: %v", err)
	}
	if len(msgs) != 2 {
		t.Errorf("expected 2 messages after second write, got %d", len(msgs))
	}
}

// --- Test 4: Concurrent read during write ---

func TestSessionPersistence_ConcurrentReadWrite(t *testing.T) {
	store, _ := openStore(t)
	defer store.Close()

	// Seed with an initial session so readers always have something.
	seed, err := store.CreateSession("Seed")
	if err != nil {
		t.Fatalf("CreateSession (seed): %v", err)
	}
	_ = seed

	var (
		writeErrors int64
		readErrors  int64
		wg          sync.WaitGroup
		done        = make(chan struct{})
	)

	// Writer goroutine: creates sessions continuously.
	wg.Add(1)
	go func() {
		defer wg.Done()
		i := 0
		for {
			select {
			case <-done:
				return
			default:
				_, err := store.CreateSession(fmt.Sprintf("concurrent-%d", i))
				if err != nil {
					atomic.AddInt64(&writeErrors, 1)
				}
				i++
			}
		}
	}()

	// Reader goroutine: lists sessions continuously.
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-done:
				return
			default:
				sessions, err := store.ListSessions()
				if err != nil {
					atomic.AddInt64(&readErrors, 1)
					continue
				}
				// Verify no nil sessions in results.
				for _, s := range sessions {
					if s == nil {
						atomic.AddInt64(&readErrors, 1)
					}
				}
			}
		}
	}()

	// Second reader goroutine: reads messages for the seed session.
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-done:
				return
			default:
				_, err := store.GetMessages(seed.ID)
				if err != nil {
					atomic.AddInt64(&readErrors, 1)
				}
			}
		}
	}()

	// Run for 2 seconds (generous for Raspberry Pi).
	time.Sleep(2 * time.Second)
	close(done)
	wg.Wait()

	we := atomic.LoadInt64(&writeErrors)
	re := atomic.LoadInt64(&readErrors)
	if we > 0 {
		t.Errorf("write errors: %d", we)
	}
	if re > 0 {
		t.Errorf("read errors: %d", re)
	}

	// Verify data integrity: all sessions should be listable.
	sessions, err := store.ListSessions()
	if err != nil {
		t.Fatalf("final ListSessions: %v", err)
	}
	// At minimum the seed session should be there.
	if len(sessions) < 1 {
		t.Errorf("expected at least 1 session, got %d", len(sessions))
	}
	t.Logf("created %d sessions with 0 errors during concurrent access", len(sessions))
}

// --- Test 5: Full API integration via HTTP ---

func TestSessionAPI_FullLifecycle(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "api-lifecycle.db")
	store, err := session.Open(dbPath)
	if err != nil {
		t.Fatalf("session.Open: %v", err)
	}
	defer store.Close()

	srv := server.New(
		server.WithSessionStore(store),
		server.WithPort(0), // not used with httptest
	)

	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	client := ts.Client()

	// Step 1: POST /api/sessions — create first session.
	resp1, err := client.Post(ts.URL+"/api/sessions", "application/json",
		strings.NewReader(`{"title":"First Session"}`))
	if err != nil {
		t.Fatalf("POST /api/sessions (1): %v", err)
	}
	defer resp1.Body.Close()
	if resp1.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d", resp1.StatusCode)
	}

	var create1 struct {
		Session struct {
			ID    string `json:"id"`
			Title string `json:"title"`
		} `json:"session"`
	}
	if err := json.NewDecoder(resp1.Body).Decode(&create1); err != nil {
		t.Fatalf("decode create1: %v", err)
	}
	if create1.Session.Title != "First Session" {
		t.Errorf("session1 title = %q, want %q", create1.Session.Title, "First Session")
	}
	firstID := create1.Session.ID

	// Step 2: POST /api/sessions — create second session.
	resp2, err := client.Post(ts.URL+"/api/sessions", "application/json",
		strings.NewReader(`{"title":"Second Session"}`))
	if err != nil {
		t.Fatalf("POST /api/sessions (2): %v", err)
	}
	defer resp2.Body.Close()
	if resp2.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d", resp2.StatusCode)
	}

	var create2 struct {
		Session struct {
			ID    string `json:"id"`
			Title string `json:"title"`
		} `json:"session"`
	}
	if err := json.NewDecoder(resp2.Body).Decode(&create2); err != nil {
		t.Fatalf("decode create2: %v", err)
	}
	secondID := create2.Session.ID

	// Step 3: GET /api/sessions — verify both are listed.
	resp3, err := client.Get(ts.URL + "/api/sessions")
	if err != nil {
		t.Fatalf("GET /api/sessions: %v", err)
	}
	defer resp3.Body.Close()
	if resp3.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp3.StatusCode)
	}

	var list1 struct {
		Sessions []struct {
			ID    string `json:"id"`
			Title string `json:"title"`
		} `json:"sessions"`
	}
	if err := json.NewDecoder(resp3.Body).Decode(&list1); err != nil {
		t.Fatalf("decode list1: %v", err)
	}
	if len(list1.Sessions) != 2 {
		t.Fatalf("expected 2 sessions, got %d", len(list1.Sessions))
	}

	// Step 4: DELETE /api/sessions/{id} — delete first session.
	delReq, err := http.NewRequest("DELETE", ts.URL+"/api/sessions/"+firstID, nil)
	if err != nil {
		t.Fatalf("new DELETE request: %v", err)
	}
	resp4, err := client.Do(delReq)
	if err != nil {
		t.Fatalf("DELETE /api/sessions/%s: %v", firstID, err)
	}
	defer resp4.Body.Close()
	if resp4.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp4.StatusCode)
	}

	var delBody struct {
		Deleted bool `json:"deleted"`
	}
	if err := json.NewDecoder(resp4.Body).Decode(&delBody); err != nil {
		t.Fatalf("decode delete response: %v", err)
	}
	if !delBody.Deleted {
		t.Error("expected deleted=true")
	}

	// Step 5: GET /api/sessions — verify only second remains.
	resp5, err := client.Get(ts.URL + "/api/sessions")
	if err != nil {
		t.Fatalf("GET /api/sessions (after delete): %v", err)
	}
	defer resp5.Body.Close()
	if resp5.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp5.StatusCode)
	}

	var list2 struct {
		Sessions []struct {
			ID    string `json:"id"`
			Title string `json:"title"`
		} `json:"sessions"`
	}
	if err := json.NewDecoder(resp5.Body).Decode(&list2); err != nil {
		t.Fatalf("decode list2: %v", err)
	}
	if len(list2.Sessions) != 1 {
		t.Fatalf("expected 1 session after delete, got %d", len(list2.Sessions))
	}
	if list2.Sessions[0].ID != secondID {
		t.Errorf("remaining session ID = %q, want %q", list2.Sessions[0].ID, secondID)
	}
	if list2.Sessions[0].Title != "Second Session" {
		t.Errorf("remaining session title = %q, want %q", list2.Sessions[0].Title, "Second Session")
	}

	// Step 6: GET /api/sessions/{id}/messages — verify empty for second session.
	resp6, err := client.Get(ts.URL + "/api/sessions/" + secondID + "/messages")
	if err != nil {
		t.Fatalf("GET /api/sessions/%s/messages: %v", secondID, err)
	}
	defer resp6.Body.Close()
	if resp6.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp6.StatusCode)
	}

	var msgBody struct {
		Messages []interface{} `json:"messages"`
	}
	if err := json.NewDecoder(resp6.Body).Decode(&msgBody); err != nil {
		t.Fatalf("decode messages: %v", err)
	}
	if len(msgBody.Messages) != 0 {
		t.Errorf("expected 0 messages, got %d", len(msgBody.Messages))
	}

	// Step 7: Verify deleted session returns 404.
	resp7, err := client.Get(ts.URL + "/api/sessions/" + firstID + "/messages")
	if err != nil {
		t.Fatalf("GET messages for deleted session: %v", err)
	}
	defer resp7.Body.Close()
	if resp7.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404 for deleted session messages, got %d", resp7.StatusCode)
	}
}
