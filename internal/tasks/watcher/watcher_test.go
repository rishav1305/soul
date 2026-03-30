package watcher

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rishav1305/soul/internal/chat/stream"
	"github.com/rishav1305/soul/internal/tasks/store"
)

type mockSender struct {
	called bool
}

func (m *mockSender) Send(ctx context.Context, req *stream.Request) (*stream.Response, error) {
	m.called = true
	return &stream.Response{
		StopReason: "end_turn",
		Content:    []stream.ContentBlock{{Type: "text", Text: "I've reviewed the feedback."}},
		Usage:      &stream.Usage{InputTokens: 200, OutputTokens: 50},
	}, nil
}

type errorSender struct{ called bool }

func (e *errorSender) Send(ctx context.Context, req *stream.Request) (*stream.Response, error) {
	e.called = true
	return nil, fmt.Errorf("mock agent error: connection refused")
}

type emptySender struct{ called bool }

func (e *emptySender) Send(ctx context.Context, req *stream.Request) (*stream.Response, error) {
	e.called = true
	return &stream.Response{
		StopReason: "end_turn",
		Content:    []stream.ContentBlock{},
		Usage:      &stream.Usage{InputTokens: 100, OutputTokens: 0},
	}, nil
}

func newTestStore(t *testing.T) *store.Store {
	t.Helper()
	path := filepath.Join(t.TempDir(), "watcher_test.db")
	s, err := store.Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestWatcher_PollsComments_WithAgent(t *testing.T) {
	s := newTestStore(t)
	task, err := s.Create("Active task", "", "")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if _, err := s.Update(task.ID, map[string]interface{}{"stage": "active"}); err != nil {
		t.Fatalf("Update: %v", err)
	}

	if _, err := s.InsertComment(task.ID, "user", "feedback", "Please fix the tests"); err != nil {
		t.Fatalf("InsertComment: %v", err)
	}

	ms := &mockSender{}
	cw := New(s, ms, "/tmp/test-project")
	cw.poll(context.Background())

	if !ms.called {
		t.Fatal("expected sender.Send to be called")
	}

	comments, err := s.GetComments(task.ID)
	if err != nil {
		t.Fatalf("GetComments: %v", err)
	}
	if len(comments) != 2 {
		t.Fatalf("expected 2 comments (user + soul reply), got %d", len(comments))
	}
	if comments[1].Author != "soul" {
		t.Errorf("reply author = %q, want %q", comments[1].Author, "soul")
	}
	if !strings.Contains(comments[1].Body, "I've reviewed the feedback.") {
		t.Errorf("reply body = %q, want it to contain agent response", comments[1].Body)
	}
}

func TestWatcher_SkipsNonActionable(t *testing.T) {
	s := newTestStore(t)
	task, err := s.Create("Backlog task", "", "")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	// Task is in backlog (default stage), which is not actionable.

	if _, err := s.InsertComment(task.ID, "user", "feedback", "Some feedback"); err != nil {
		t.Fatalf("InsertComment: %v", err)
	}

	ms := &mockSender{}
	cw := New(s, ms, "/tmp/test-project")
	cw.poll(context.Background())

	if ms.called {
		t.Error("expected sender.Send NOT to be called for non-actionable task")
	}

	comments, err := s.GetComments(task.ID)
	if err != nil {
		t.Fatalf("GetComments: %v", err)
	}
	if len(comments) != 2 {
		t.Fatalf("expected 2 comments, got %d", len(comments))
	}
	expected := "Task is in backlog — comment noted but no action taken."
	if comments[1].Body != expected {
		t.Errorf("reply body = %q, want %q", comments[1].Body, expected)
	}
}

func TestWatcher_NilSender(t *testing.T) {
	s := newTestStore(t)
	task, err := s.Create("Active task", "", "")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if _, err := s.Update(task.ID, map[string]interface{}{"stage": "active"}); err != nil {
		t.Fatalf("Update: %v", err)
	}

	if _, err := s.InsertComment(task.ID, "user", "feedback", "Some feedback"); err != nil {
		t.Fatalf("InsertComment: %v", err)
	}

	cw := New(s, nil, "/tmp/test-project")
	cw.poll(context.Background())

	comments, err := s.GetComments(task.ID)
	if err != nil {
		t.Fatalf("GetComments: %v", err)
	}
	if len(comments) != 2 {
		t.Fatalf("expected 2 comments, got %d", len(comments))
	}
	expected := "Received feedback. Agent not configured."
	if comments[1].Body != expected {
		t.Errorf("reply body = %q, want %q", comments[1].Body, expected)
	}
}

func TestWatcher_Start_ContextCancel(t *testing.T) {
	s := newTestStore(t)
	ms := &mockSender{}
	cw := New(s, ms, "/tmp/test-project")

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	done := make(chan struct{})
	go func() {
		cw.Start(ctx)
		close(done)
	}()

	select {
	case <-done:
		// Start returned — good
	case <-time.After(2 * time.Second):
		t.Fatal("Start did not return after context cancel")
	}
}

// --- Error path tests ---

func TestWatcher_AgentError(t *testing.T) {
	s := newTestStore(t)
	task, err := s.Create("Active task", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Update(task.ID, map[string]interface{}{"stage": "active"}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.InsertComment(task.ID, "user", "feedback", "Please review"); err != nil {
		t.Fatal(err)
	}

	es := &errorSender{}
	cw := New(s, es, "/tmp/test")
	cw.poll(context.Background())

	if !es.called {
		t.Fatal("expected sender.Send to be called")
	}

	comments, err := s.GetComments(task.ID)
	if err != nil {
		t.Fatal(err)
	}
	// Should have original comment + error reply.
	if len(comments) != 2 {
		t.Fatalf("expected 2 comments, got %d", len(comments))
	}
	if !strings.Contains(comments[1].Body, "Agent error") {
		t.Errorf("expected error reply, got %q", comments[1].Body)
	}
}

func TestWatcher_EmptyAgentResponse(t *testing.T) {
	s := newTestStore(t)
	task, err := s.Create("Active task", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Update(task.ID, map[string]interface{}{"stage": "active"}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.InsertComment(task.ID, "user", "feedback", "Empty response test"); err != nil {
		t.Fatal(err)
	}

	es := &emptySender{}
	cw := New(s, es, "/tmp/test")
	cw.poll(context.Background())

	if !es.called {
		t.Fatal("expected sender.Send to be called")
	}

	comments, err := s.GetComments(task.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(comments) != 2 {
		t.Fatalf("expected 2 comments, got %d", len(comments))
	}
	if comments[1].Body != "Agent returned empty response." {
		t.Errorf("expected empty response message, got %q", comments[1].Body)
	}
}

func TestWatcher_ValidationStage(t *testing.T) {
	s := newTestStore(t)
	task, err := s.Create("Validation task", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Update(task.ID, map[string]interface{}{"stage": "validation"}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.InsertComment(task.ID, "user", "review", "Looks good"); err != nil {
		t.Fatal(err)
	}

	ms := &mockSender{}
	cw := New(s, ms, "/tmp/test")
	cw.poll(context.Background())

	if !ms.called {
		t.Fatal("expected sender.Send to be called for validation stage")
	}
}

func TestWatcher_BlockedStage(t *testing.T) {
	s := newTestStore(t)
	task, err := s.Create("Blocked task", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Update(task.ID, map[string]interface{}{"stage": "blocked"}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.InsertComment(task.ID, "user", "feedback", "Unblock me"); err != nil {
		t.Fatal(err)
	}

	ms := &mockSender{}
	cw := New(s, ms, "/tmp/test")
	cw.poll(context.Background())

	if !ms.called {
		t.Fatal("expected sender.Send to be called for blocked stage")
	}
}

func TestWatcher_ClosedStore_Poll(t *testing.T) {
	dir := t.TempDir()
	s, err := store.Open(filepath.Join(dir, "closed.db"))
	if err != nil {
		t.Fatal(err)
	}
	s.Close()

	// Poll with closed store should not panic — just logs error.
	cw := New(s, &mockSender{}, "/tmp/test")
	cw.poll(context.Background())
}

func TestWatcher_MultipleComments(t *testing.T) {
	s := newTestStore(t)
	task, err := s.Create("Multi-comment task", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Update(task.ID, map[string]interface{}{"stage": "active"}); err != nil {
		t.Fatal(err)
	}

	// Insert multiple user comments.
	if _, err := s.InsertComment(task.ID, "user", "feedback", "First comment"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.InsertComment(task.ID, "user", "feedback", "Second comment"); err != nil {
		t.Fatal(err)
	}

	ms := &mockSender{}
	cw := New(s, ms, "/tmp/test")
	cw.poll(context.Background())

	if !ms.called {
		t.Fatal("expected sender to be called")
	}

	comments, err := s.GetComments(task.ID)
	if err != nil {
		t.Fatal(err)
	}
	// 2 user comments + 2 agent replies = 4.
	if len(comments) < 4 {
		t.Errorf("expected at least 4 comments, got %d", len(comments))
	}
}

func TestCommentsAfter_ExcludesSoul(t *testing.T) {
	s := newTestStore(t)
	task, err := s.Create("Test task", "", "")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	// Insert a mix of user and soul comments.
	s.InsertComment(task.ID, "user", "feedback", "User comment 1")
	s.InsertComment(task.ID, "soul", "auto", "Soul reply")
	cmt3, _ := s.InsertComment(task.ID, "user", "feedback", "User comment 2")

	// CommentsAfter(0) should only return user comments.
	comments, err := s.CommentsAfter(0)
	if err != nil {
		t.Fatalf("CommentsAfter: %v", err)
	}
	if len(comments) != 2 {
		t.Fatalf("expected 2 user comments, got %d", len(comments))
	}
	for _, c := range comments {
		if c.Author != "user" {
			t.Errorf("expected author=user, got %q", c.Author)
		}
	}

	// CommentsAfter(cmt3.ID) should return nothing since cmt3 is the last user comment.
	comments, err = s.CommentsAfter(cmt3.ID)
	if err != nil {
		t.Fatalf("CommentsAfter(%d): %v", cmt3.ID, err)
	}
	if len(comments) != 0 {
		t.Errorf("expected 0 comments after id %d, got %d", cmt3.ID, len(comments))
	}
}
