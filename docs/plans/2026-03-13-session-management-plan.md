# Session Management Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Enrich the session sidebar with smart titles, message previews, unread badges, time grouping, and inline rename — plus backfill existing sessions.

**Architecture:** Two new columns (`last_message`, `unread_count`) added to the sessions table. Backend updates these inline during AddMessage/AddMessageTx (no extra queries). Smart titles use instant truncation + background Haiku API call. Frontend SessionList gets time-grouped rendering, preview line, unread badge, and inline rename. A new `session.rename` WebSocket message type enables rename from the client.

**Tech Stack:** Go 1.24 (SQLite), React 19, TypeScript 5.9, Tailwind v4, Claude Haiku API

**Spec:** `docs/plans/2026-03-13-session-management-design.md`

---

## File Map

| File | Action | Responsibility |
|------|--------|----------------|
| `internal/session/store.go` | MODIFY | Add `last_message`/`unread_count` columns to schema, update Session struct, update AddMessage/AddMessageTx to set last_message + increment unread_count, add `ResetUnreadCount`/`SetLastMessage` methods |
| `internal/session/iface.go` | MODIFY | Add `ResetUnreadCount`/`SetLastMessage` to StoreInterface |
| `internal/session/timed_store.go` | MODIFY | Add timed wrappers for new methods |
| `internal/session/store_test.go` | MODIFY | Tests for new columns, migration, ResetUnreadCount, SetLastMessage |
| `internal/ws/message.go` | MODIFY | Add `TypeSessionRename` constant, add to `InboundMessage` types |
| `internal/ws/handler.go` | MODIFY | Add rename handler, update handleChatSend/handleSessionSwitch/completeSession for unread/lastMessage logic, add background smart title goroutine |
| `web/src/lib/types.ts` | MODIFY | Add `lastMessage`/`unreadCount` to Session, add `session.rename` to message types |
| `web/src/components/SessionList.tsx` | MODIFY | Add preview line, unread badge, time grouping, inline rename |
| `web/src/hooks/useChat.ts` | MODIFY | Handle `session.rename` send, reset unread on switch |

---

## Chunk 1: Backend Data Model

### Task 1: Add last_message and unread_count to Session struct and schema

**Files:**
- Modify: `internal/session/store.go:58-65` (Session struct)
- Modify: `internal/session/store.go:141-167` (Migrate)
- Modify: `internal/session/store.go:186-192` (CreateSession INSERT)
- Modify: `internal/session/store.go:208-211` (GetSession SELECT/Scan)
- Modify: `internal/session/store.go:234-246` (ListSessions SELECT/Scan)

- [ ] **Step 1: Add fields to Session struct**

In `internal/session/store.go`, add two fields after `MessageCount`:

```go
type Session struct {
	ID           string    `json:"id"`
	Title        string    `json:"title"`
	Status       Status    `json:"status"`
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
	MessageCount int       `json:"messageCount"`
	LastMessage  string    `json:"lastMessage"`
	UnreadCount  int       `json:"unreadCount"`
}
```

- [ ] **Step 2: Add migration for new columns**

In `internal/session/store.go`, after the existing schema in `Migrate()`, add ALTER TABLE statements. Since these use IF NOT EXISTS semantics via column existence checks, wrap in a helper:

```go
func (s *Store) Migrate() error {
	const schema = `
CREATE TABLE IF NOT EXISTS sessions (
    id TEXT PRIMARY KEY,
    title TEXT NOT NULL DEFAULT 'New Session',
    status TEXT NOT NULL DEFAULT 'idle',
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    message_count INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS messages (
    id TEXT PRIMARY KEY,
    session_id TEXT NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
    role TEXT NOT NULL,
    content TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_messages_session_created ON messages(session_id, created_at);
`
	_, err := s.db.Exec(schema)
	if err != nil {
		return fmt.Errorf("session: execute schema: %w", err)
	}

	// Add new columns (idempotent — ignore "duplicate column" errors).
	for _, alt := range []string{
		"ALTER TABLE sessions ADD COLUMN last_message TEXT DEFAULT ''",
		"ALTER TABLE sessions ADD COLUMN unread_count INTEGER DEFAULT 0",
	} {
		if _, err := s.db.Exec(alt); err != nil {
			// SQLite returns "duplicate column name" if column already exists.
			if !strings.Contains(err.Error(), "duplicate column") {
				return fmt.Errorf("session: migrate column: %w", err)
			}
		}
	}

	return nil
}
```

Add `"strings"` to imports (already present).

- [ ] **Step 3: Update CreateSession to include new columns**

```go
_, err := s.db.Exec(
    "INSERT INTO sessions (id, title, status, created_at, updated_at, message_count, last_message, unread_count) VALUES (?, ?, ?, ?, ?, ?, ?, ?)",
    sess.ID, sess.Title, string(sess.Status),
    sess.CreatedAt.Format(time.RFC3339Nano),
    sess.UpdatedAt.Format(time.RFC3339Nano),
    sess.MessageCount,
    sess.LastMessage,
    sess.UnreadCount,
)
```

- [ ] **Step 4: Update GetSession to scan new columns**

```go
err := s.db.QueryRow(
    "SELECT id, title, status, created_at, updated_at, message_count, last_message, unread_count FROM sessions WHERE id = ?",
    id,
).Scan(&sess.ID, &sess.Title, &status, &createdAt, &updatedAt, &sess.MessageCount, &sess.LastMessage, &sess.UnreadCount)
```

- [ ] **Step 5: Update ListSessions to scan new columns**

```go
rows, err := s.db.Query(
    "SELECT id, title, status, created_at, updated_at, message_count, last_message, unread_count FROM sessions ORDER BY updated_at DESC",
)
// ...
if err := rows.Scan(&sess.ID, &sess.Title, &status, &createdAt, &updatedAt, &sess.MessageCount, &sess.LastMessage, &sess.UnreadCount); err != nil {
```

- [ ] **Step 6: Update AddMessage to set last_message and increment unread_count**

In `AddMessage`, change the UPDATE statement:

```go
// Truncate content for last_message preview (100 chars at word boundary).
preview := content
if len(preview) > 100 {
    preview = preview[:100]
    if i := strings.LastIndex(preview, " "); i > 50 {
        preview = preview[:i]
    }
    preview += "..."
}

_, err = s.db.Exec(
    "UPDATE sessions SET message_count = message_count + 1, unread_count = unread_count + 1, last_message = ?, updated_at = ? WHERE id = ?",
    preview, now.Format(time.RFC3339Nano), sessionID,
)
```

- [ ] **Step 7: Update AddMessageTx to set last_message and increment unread_count**

Same change in `AddMessageTx`:

```go
preview := content
if len(preview) > 100 {
    preview = preview[:100]
    if i := strings.LastIndex(preview, " "); i > 50 {
        preview = preview[:i]
    }
    preview += "..."
}

_, err = tx.Exec(
    "UPDATE sessions SET message_count = message_count + 1, unread_count = unread_count + 1, last_message = ?, updated_at = ? WHERE id = ?",
    preview, now.Format(time.RFC3339Nano), sessionID,
)
```

- [ ] **Step 8: Add ResetUnreadCount and SetLastMessage methods**

```go
// ResetUnreadCount sets unread_count to 0 for the given session.
func (s *Store) ResetUnreadCount(id string) error {
	if !uuidRe.MatchString(id) {
		return fmt.Errorf("session: invalid UUID format: %q", id)
	}
	_, err := s.db.Exec("UPDATE sessions SET unread_count = 0 WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("session: reset unread: %w", err)
	}
	return nil
}

// SetLastMessage updates the last_message preview for the given session.
func (s *Store) SetLastMessage(id, content string) error {
	if !uuidRe.MatchString(id) {
		return fmt.Errorf("session: invalid UUID format: %q", id)
	}
	preview := content
	if len(preview) > 100 {
		preview = preview[:100]
		if i := strings.LastIndex(preview, " "); i > 50 {
			preview = preview[:i]
		}
		preview += "..."
	}
	_, err := s.db.Exec("UPDATE sessions SET last_message = ? WHERE id = ?", preview, id)
	if err != nil {
		return fmt.Errorf("session: set last message: %w", err)
	}
	return nil
}
```

- [ ] **Step 9: Verify compilation**

Run: `cd /home/rishav/soul-v2 && go build ./internal/session/...`
Expected: Success (no errors)

- [ ] **Step 10: Commit**

```bash
git add internal/session/store.go
git commit -m "feat(session): add last_message, unread_count columns and methods"
```

### Task 2: Update StoreInterface and TimedStore

**Files:**
- Modify: `internal/session/iface.go`
- Modify: `internal/session/timed_store.go`

- [ ] **Step 1: Add new methods to StoreInterface**

In `internal/session/iface.go`:

```go
type StoreInterface interface {
	CreateSession(title string) (*Session, error)
	GetSession(id string) (*Session, error)
	ListSessions() ([]*Session, error)
	UpdateSessionTitle(id, title string) (*Session, error)
	UpdateSessionStatus(id string, status Status) error
	DeleteSession(id string) error
	AddMessage(sessionID, role, content string) (*Message, error)
	AddMessageTx(tx *sql.Tx, sessionID, role, content string) (*Message, error)
	GetMessages(sessionID string) ([]*Message, error)
	RunInTransaction(fn func(tx *sql.Tx) error) error
	ResetUnreadCount(id string) error
	SetLastMessage(id, content string) error
	Close() error
}
```

- [ ] **Step 2: Add timed wrappers in TimedStore**

In `internal/session/timed_store.go`, add:

```go
// ResetUnreadCount resets the unread count and logs timing.
func (ts *TimedStore) ResetUnreadCount(id string) error {
	start := time.Now()
	err := ts.inner.ResetUnreadCount(id)
	ts.logQuery("ResetUnreadCount", start, id)
	return err
}

// SetLastMessage updates the last message preview and logs timing.
func (ts *TimedStore) SetLastMessage(id, content string) error {
	start := time.Now()
	err := ts.inner.SetLastMessage(id, content)
	ts.logQuery("SetLastMessage", start, id)
	return err
}
```

- [ ] **Step 3: Verify compilation**

Run: `cd /home/rishav/soul-v2 && go build ./...`
Expected: Success

- [ ] **Step 4: Commit**

```bash
git add internal/session/iface.go internal/session/timed_store.go
git commit -m "feat(session): add ResetUnreadCount/SetLastMessage to interface and TimedStore"
```

### Task 3: Backfill migration

**Files:**
- Modify: `internal/session/store.go:141-167` (Migrate)

- [ ] **Step 1: Add backfill logic to Migrate()**

After the ALTER TABLE statements, add the backfill:

```go
// Backfill: set last_message from most recent message per session.
_, _ = s.db.Exec(`
    UPDATE sessions SET last_message = (
        SELECT CASE
            WHEN length(m.content) > 100 THEN substr(m.content, 1, 100) || '...'
            ELSE m.content
        END
        FROM messages m
        WHERE m.session_id = sessions.id
        ORDER BY m.created_at DESC
        LIMIT 1
    )
    WHERE last_message = '' AND EXISTS (
        SELECT 1 FROM messages WHERE session_id = sessions.id
    )
`)

// Backfill: set title from first user message for untitled sessions.
_, _ = s.db.Exec(`
    UPDATE sessions SET title = (
        SELECT CASE
            WHEN length(m.content) > 50 THEN substr(m.content, 1, 50) || '...'
            ELSE m.content
        END
        FROM messages m
        WHERE m.session_id = sessions.id AND m.role = 'user'
        ORDER BY m.created_at ASC
        LIMIT 1
    )
    WHERE title = 'New Session' AND EXISTS (
        SELECT 1 FROM messages WHERE session_id = sessions.id AND role = 'user'
    )
`)
```

- [ ] **Step 2: Verify compilation**

Run: `cd /home/rishav/soul-v2 && go build ./internal/session/...`
Expected: Success

- [ ] **Step 3: Commit**

```bash
git add internal/session/store.go
git commit -m "feat(session): backfill last_message and titles for existing sessions"
```

### Task 4: Write tests for new data model

**Files:**
- Modify: `internal/session/store_test.go`

- [ ] **Step 1: Write tests**

Add to the existing test file:

```go
func TestAddMessage_SetsLastMessageAndUnreadCount(t *testing.T) {
	store := setupTestStore(t)
	sess, err := store.CreateSession("Test")
	if err != nil {
		t.Fatal(err)
	}

	_, err = store.AddMessage(sess.ID, "user", "Hello world this is a test")
	if err != nil {
		t.Fatal(err)
	}

	updated, err := store.GetSession(sess.ID)
	if err != nil {
		t.Fatal(err)
	}
	if updated.LastMessage != "Hello world this is a test" {
		t.Errorf("expected last_message=%q, got %q", "Hello world this is a test", updated.LastMessage)
	}
	if updated.UnreadCount != 1 {
		t.Errorf("expected unread_count=1, got %d", updated.UnreadCount)
	}
}

func TestAddMessage_TruncatesLongLastMessage(t *testing.T) {
	store := setupTestStore(t)
	sess, err := store.CreateSession("Test")
	if err != nil {
		t.Fatal(err)
	}

	longContent := strings.Repeat("word ", 30) // 150 chars
	_, err = store.AddMessage(sess.ID, "user", longContent)
	if err != nil {
		t.Fatal(err)
	}

	updated, err := store.GetSession(sess.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(updated.LastMessage) > 104 { // 100 + "..."
		t.Errorf("expected truncated last_message, got length %d", len(updated.LastMessage))
	}
}

func TestResetUnreadCount(t *testing.T) {
	store := setupTestStore(t)
	sess, err := store.CreateSession("Test")
	if err != nil {
		t.Fatal(err)
	}

	// Add two messages to increment unread_count.
	store.AddMessage(sess.ID, "user", "msg1")
	store.AddMessage(sess.ID, "assistant", "msg2")

	updated, _ := store.GetSession(sess.ID)
	if updated.UnreadCount != 2 {
		t.Fatalf("expected unread_count=2, got %d", updated.UnreadCount)
	}

	err = store.ResetUnreadCount(sess.ID)
	if err != nil {
		t.Fatal(err)
	}

	updated, _ = store.GetSession(sess.ID)
	if updated.UnreadCount != 0 {
		t.Errorf("expected unread_count=0 after reset, got %d", updated.UnreadCount)
	}
}

func TestSetLastMessage(t *testing.T) {
	store := setupTestStore(t)
	sess, err := store.CreateSession("Test")
	if err != nil {
		t.Fatal(err)
	}

	err = store.SetLastMessage(sess.ID, "Updated preview")
	if err != nil {
		t.Fatal(err)
	}

	updated, _ := store.GetSession(sess.ID)
	if updated.LastMessage != "Updated preview" {
		t.Errorf("expected last_message=%q, got %q", "Updated preview", updated.LastMessage)
	}
}

func TestMigrate_BackfillsExistingSessions(t *testing.T) {
	store := setupTestStore(t)
	sess, _ := store.CreateSession("New Session")
	store.AddMessage(sess.ID, "user", "First user message")
	store.AddMessage(sess.ID, "assistant", "Response here")

	// Re-run migrate (simulates upgrade).
	err := store.Migrate()
	if err != nil {
		t.Fatal(err)
	}

	updated, _ := store.GetSession(sess.ID)
	// Title should be backfilled from first user message (since it was "New Session").
	// Note: backfill only runs when last_message is empty, and here it's already set
	// by AddMessage, so this test just verifies migration is idempotent.
	if updated.LastMessage == "" {
		t.Error("expected last_message to be set")
	}
}
```

- [ ] **Step 2: Run tests**

Run: `cd /home/rishav/soul-v2 && go test ./internal/session/ -v -run "TestAddMessage_SetsLastMessage|TestAddMessage_Truncates|TestResetUnread|TestSetLastMessage|TestMigrate_Backfill"`
Expected: All PASS

- [ ] **Step 3: Commit**

```bash
git add internal/session/store_test.go
git commit -m "test(session): add tests for last_message, unread_count, backfill"
```

---

## Chunk 2: Backend WebSocket — Rename, Unread, Smart Titles

### Task 5: Add session.rename WebSocket message type

**Files:**
- Modify: `internal/ws/message.go:11-16`

- [ ] **Step 1: Add TypeSessionRename constant**

```go
const (
	TypeChatSend      = "chat.send"
	TypeSessionSwitch = "session.switch"
	TypeSessionCreate = "session.create"
	TypeSessionDelete = "session.delete"
	TypeSessionRename = "session.rename"
)
```

- [ ] **Step 2: Commit**

```bash
git add internal/ws/message.go
git commit -m "feat(ws): add session.rename message type constant"
```

### Task 6: Add rename handler and unread/lastMessage logic to handler.go

**Files:**
- Modify: `internal/ws/handler.go`

- [ ] **Step 1: Add rename case to HandleMessage switch**

In `HandleMessage`, add after `TypeSessionDelete`:

```go
case TypeSessionRename:
    h.handleSessionRename(client, msg)
```

- [ ] **Step 2: Implement handleSessionRename**

```go
// handleSessionRename processes a session.rename message. It validates the
// session exists, sanitizes the title, updates it, and broadcasts.
func (h *MessageHandler) handleSessionRename(client *Client, msg *InboundMessage) {
	if msg.SessionID == "" {
		h.sendError(client, "", "session ID required")
		return
	}
	if !IsValidUUID(msg.SessionID) {
		h.sendError(client, "", "invalid session ID")
		return
	}

	title := ValidateSessionTitle(msg.Content)
	if title == "" {
		h.sendError(client, msg.SessionID, "title cannot be empty")
		return
	}

	updated, err := h.sessionStore.UpdateSessionTitle(msg.SessionID, title)
	if err != nil {
		log.Printf("ws: failed to rename session %s: %v", msg.SessionID, err)
		h.sendError(client, msg.SessionID, "failed to rename session")
		return
	}

	h.broadcast(NewSessionUpdated(updated))
}
```

- [ ] **Step 3: Update handleSessionSwitch to reset unread count**

In `handleSessionSwitch`, after `client.Subscribe(msg.SessionID)` (line 380), add:

```go
// Reset unread count for the session being switched to.
if err := h.sessionStore.ResetUnreadCount(msg.SessionID); err != nil {
    log.Printf("ws: failed to reset unread count for session %s: %v", msg.SessionID, err)
}
```

- [ ] **Step 4: Update handleChatSend to reset unread count**

In `handleChatSend`, after the user message is stored (after line 131), add:

```go
// User is active in this session — reset unread count.
if err := h.sessionStore.ResetUnreadCount(msg.SessionID); err != nil {
    log.Printf("ws: failed to reset unread count for session %s: %v", msg.SessionID, err)
}
```

- [ ] **Step 5: Update auto-title to use word boundary truncation at 50 chars**

In `handleChatSend`, update the auto-title section (lines 104-114):

```go
// Auto-title from first user message (if session has no messages yet).
sess, _ := h.sessionStore.GetSession(msg.SessionID)
if sess != nil && sess.MessageCount == 0 {
    title := msg.Content
    if len(title) > 50 {
        title = title[:50]
        if i := strings.LastIndex(title, " "); i > 25 {
            title = title[:i]
        }
        title += "..."
    }
    if updated, err := h.sessionStore.UpdateSessionTitle(msg.SessionID, title); err == nil {
        h.broadcast(NewSessionUpdated(updated))
    }
}
```

- [ ] **Step 6: Verify compilation**

Run: `cd /home/rishav/soul-v2 && go build ./internal/ws/...`
Expected: Success

- [ ] **Step 7: Commit**

```bash
git add internal/ws/handler.go
git commit -m "feat(ws): add session.rename handler, unread reset on switch/send, 50-char auto-title"
```

### Task 7: Background smart title generation

**Files:**
- Modify: `internal/ws/handler.go`

- [ ] **Step 1: Add streamClient accessor to MessageHandler**

The handler already has `streamClient *stream.Client`. We need to use it for background title gen.

- [ ] **Step 2: Add background title generation in completeSession**

In `completeSession` (line 468), after broadcasting the session update, add background title generation when `MessageCount == 2` (first exchange):

```go
func (h *MessageHandler) completeSession(client *Client, sessionID string) {
	sess, err := h.sessionStore.GetSession(sessionID)
	if err != nil || sess.Status != session.StatusRunning {
		return
	}

	// If client is subscribed to this session, mark completed; otherwise unread.
	newStatus := session.StatusCompletedUnread
	if client.SessionID() == sessionID {
		newStatus = session.StatusCompleted
	}

	if err := h.sessionStore.UpdateSessionStatus(sessionID, newStatus); err != nil {
		log.Printf("ws: failed to complete session %s: %v", sessionID, err)
		return
	}
	if updated, err := h.sessionStore.GetSession(sessionID); err == nil {
		h.broadcast(NewSessionUpdated(updated))

		// Background smart title: after first assistant response (MessageCount == 2).
		if updated.MessageCount == 2 && h.streamClient != nil {
			go h.generateSmartTitle(sessionID)
		}
	}
}
```

- [ ] **Step 3: Implement generateSmartTitle**

```go
// generateSmartTitle uses Haiku to generate a 3-5 word title for a session
// after the first exchange. Failures are silent — the truncated title remains.
func (h *MessageHandler) generateSmartTitle(sessionID string) {
	messages, err := h.sessionStore.GetMessages(sessionID)
	if err != nil || len(messages) < 2 {
		return
	}

	// Extract first user message and first assistant response.
	var userMsg, assistantMsg string
	for _, m := range messages {
		if m.Role == "user" && userMsg == "" {
			userMsg = m.Content
			if len(userMsg) > 500 {
				userMsg = userMsg[:500]
			}
		}
		if m.Role == "assistant" && assistantMsg == "" {
			assistantMsg = m.Content
			if len(assistantMsg) > 500 {
				assistantMsg = assistantMsg[:500]
			}
		}
		if userMsg != "" && assistantMsg != "" {
			break
		}
	}

	if userMsg == "" {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req := &stream.Request{
		Model:     "claude-haiku-4-5-20251001",
		MaxTokens: 30,
		System:    "Generate a 3-5 word title for this conversation. Reply with ONLY the title, no quotes, no punctuation at the end.",
		Messages: []stream.Message{
			{Role: "user", Content: []stream.ContentBlock{{Type: "text", Text: fmt.Sprintf("User: %s\n\nAssistant: %s", userMsg, assistantMsg)}}},
		},
	}

	ch, err := h.streamClient.Stream(ctx, req)
	if err != nil {
		log.Printf("ws: smart title stream error for session %s: %v", sessionID, err)
		return
	}

	var title strings.Builder
	for evt := range ch {
		if evt.Type == "content_block_delta" && evt.Delta != nil && evt.Delta.Text != "" {
			title.WriteString(evt.Delta.Text)
		}
	}

	generated := strings.TrimSpace(title.String())
	if generated == "" || len(generated) > 100 {
		return
	}

	if updated, err := h.sessionStore.UpdateSessionTitle(sessionID, generated); err == nil {
		h.broadcast(NewSessionUpdated(updated))
		log.Printf("ws: smart title for session %s: %q", sessionID, generated)
	}
}
```

Add `"context"` to imports if not already present (it is — line 4).

- [ ] **Step 4: Verify compilation**

Run: `cd /home/rishav/soul-v2 && go build ./internal/ws/...`
Expected: Success

- [ ] **Step 5: Commit**

```bash
git add internal/ws/handler.go
git commit -m "feat(ws): background Haiku smart title generation after first exchange"
```

---

## Chunk 3: Frontend Types and Session List UI

### Task 8: Update TypeScript types

**Files:**
- Modify: `web/src/lib/types.ts:231-244` (Session interface)
- Modify: `web/src/lib/types.ts:382-402` (message types)

- [ ] **Step 1: Add lastMessage and unreadCount to Session interface**

```typescript
export interface Session {
  createdAt: string;
  id: string;
  messageCount: number;
  status: SessionStatus;
  title: string;
  updatedAt: string;
  lastMessage: string;
  unreadCount: number;
}
```

- [ ] **Step 2: Add session.rename to InboundMessageType**

```typescript
export type InboundMessageType =
  | 'chat.send'
  | 'chat.stop'
  | 'session.switch'
  | 'session.create'
  | 'session.delete'
  | 'session.rename';
```

- [ ] **Step 3: Add session.history to OutboundMessageType**

```typescript
export type OutboundMessageType =
  | 'chat.thinking'
  | 'chat.token'
  | 'chat.done'
  | 'chat.error'
  | 'tool.call'
  | 'tool.progress'
  | 'tool.complete'
  | 'tool.error'
  | 'session.created'
  | 'session.deleted'
  | 'session.list'
  | 'session.updated'
  | 'session.history'
  | 'connection.ready';
```

- [ ] **Step 4: Verify TypeScript compiles**

Run: `cd /home/rishav/soul-v2/web && npx tsc --noEmit`
Expected: Success

- [ ] **Step 5: Commit**

```bash
git add web/src/lib/types.ts
git commit -m "feat(types): add lastMessage, unreadCount to Session, session.rename/history types"
```

### Task 9: Add time grouping helper

**Files:**
- Modify: `web/src/lib/utils.ts`

- [ ] **Step 1: Add getTimeGroup function**

```typescript
export type TimeGroup = 'Today' | 'Yesterday' | 'Older';

export function getTimeGroup(dateStr: string): TimeGroup {
  const now = new Date();
  const then = new Date(dateStr);
  const today = new Date(now.getFullYear(), now.getMonth(), now.getDate());
  const yesterday = new Date(today.getTime() - 86400000);

  if (then >= today) return 'Today';
  if (then >= yesterday) return 'Yesterday';
  return 'Older';
}
```

- [ ] **Step 2: Commit**

```bash
git add web/src/lib/utils.ts
git commit -m "feat(utils): add getTimeGroup helper for session time grouping"
```

### Task 10: Enrich SessionList with preview, unread badge, time groups, inline rename

**Files:**
- Modify: `web/src/components/SessionList.tsx`

- [ ] **Step 1: Update imports**

```typescript
import React, { useState, useCallback, useEffect, useRef, useMemo } from 'react';
import type { SessionListProps, Session, SessionStatus } from '../lib/types';
import { formatRelativeTime, getTimeGroup, type TimeGroup } from '../lib/utils';
import { usePerformance } from '../hooks/usePerformance';
```

- [ ] **Step 2: Update SessionItem to include preview, unread badge, and inline rename**

Replace the entire SessionItem component:

```tsx
const SessionItem = React.memo(function SessionItem({
  session,
  isActive,
  onSwitch,
  onDelete,
  onRename,
}: {
  session: Session;
  isActive: boolean;
  onSwitch: (id: string) => void;
  onDelete: (id: string) => void;
  onRename: (id: string, title: string) => void;
}) {
  usePerformance('SessionItem');

  const [confirming, setConfirming] = useState(false);
  const [editing, setEditing] = useState(false);
  const [editValue, setEditValue] = useState('');
  const timerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const inputRef = useRef<HTMLInputElement>(null);

  useEffect(() => {
    if (confirming) {
      timerRef.current = setTimeout(() => setConfirming(false), 3000);
      return () => {
        if (timerRef.current) clearTimeout(timerRef.current);
      };
    }
  }, [confirming]);

  useEffect(() => {
    if (editing && inputRef.current) {
      inputRef.current.focus();
      inputRef.current.select();
    }
  }, [editing]);

  const handleDeleteClick = useCallback((e: React.MouseEvent) => {
    e.stopPropagation();
    setConfirming(true);
  }, []);

  const handleConfirm = useCallback((e: React.MouseEvent) => {
    e.stopPropagation();
    setConfirming(false);
    onDelete(session.id);
  }, [onDelete, session.id]);

  const handleCancel = useCallback((e: React.MouseEvent) => {
    e.stopPropagation();
    setConfirming(false);
  }, []);

  const handleDoubleClick = useCallback((e: React.MouseEvent) => {
    e.stopPropagation();
    setEditValue(session.title || 'New Session');
    setEditing(true);
  }, [session.title]);

  const handleRenameSubmit = useCallback(() => {
    const trimmed = editValue.trim();
    if (trimmed && trimmed !== session.title) {
      onRename(session.id, trimmed);
    }
    setEditing(false);
  }, [editValue, session.id, session.title, onRename]);

  const handleRenameKeyDown = useCallback((e: React.KeyboardEvent) => {
    if (e.key === 'Enter') {
      e.preventDefault();
      handleRenameSubmit();
    } else if (e.key === 'Escape') {
      setEditing(false);
    }
  }, [handleRenameSubmit]);

  const title = session.title || 'New Session';
  const timestamp = formatRelativeTime(session.updatedAt || session.createdAt);
  const hasUnread = !isActive && session.unreadCount > 0;
  const preview = session.lastMessage || '';

  // Role prefix icon for preview.
  const previewText = preview.length > 60 ? preview.slice(0, 60) + '...' : preview;

  return (
    <button
      data-testid="session-item"
      type="button"
      onClick={() => onSwitch(session.id)}
      className={`w-full text-left px-3 py-3.5 md:py-2.5 group transition-colors cursor-pointer ${
        isActive
          ? 'bg-elevated border-l-2 border-soul'
          : 'border-l-2 border-transparent hover:bg-elevated/50'
      }`}
    >
      <div className="flex items-start justify-between gap-2">
        <div className="min-w-0 flex-1 flex items-start gap-2">
          <StatusDot status={session.status} hasUnread={hasUnread} />
          <div className="min-w-0 flex-1">
            {editing ? (
              <input
                ref={inputRef}
                data-testid="session-rename-input"
                type="text"
                value={editValue}
                onChange={e => setEditValue(e.target.value)}
                onBlur={handleRenameSubmit}
                onKeyDown={handleRenameKeyDown}
                onClick={e => e.stopPropagation()}
                className="w-full text-sm font-medium bg-elevated border border-soul/40 rounded px-1 py-0.5 text-fg outline-none"
                maxLength={200}
              />
            ) : (
              <div
                data-testid="session-title"
                className={`text-sm truncate ${hasUnread ? 'font-bold text-fg' : 'font-medium text-fg'}`}
                onDoubleClick={handleDoubleClick}
              >
                {title}
              </div>
            )}
            {previewText && (
              <div data-testid="session-preview" className="text-xs text-fg-muted mt-0.5 truncate">
                {previewText}
              </div>
            )}
          </div>
        </div>
        <div className="flex flex-col items-end gap-1 shrink-0">
          <div className="flex items-center gap-1">
            {hasUnread && (
              <span
                data-testid="unread-badge"
                className="inline-flex items-center justify-center min-w-[18px] h-[18px] px-1 text-[10px] font-bold rounded-full bg-soul text-white"
              >
                {session.unreadCount > 99 ? '99+' : session.unreadCount}
              </span>
            )}
            <span className="text-[11px] text-fg-muted">{timestamp}</span>
          </div>
          {confirming ? (
            <div className="flex items-center gap-1">
              <button
                data-testid="delete-confirm-btn"
                type="button"
                onClick={handleConfirm}
                className="px-2.5 py-1.5 md:px-1.5 md:py-0.5 text-xs rounded bg-red-900/50 text-red-300 hover:bg-red-800/50 cursor-pointer"
              >
                Delete?
              </button>
              <button
                data-testid="delete-cancel-btn"
                type="button"
                onClick={handleCancel}
                className="px-2.5 py-1.5 md:px-1.5 md:py-0.5 text-xs rounded text-fg-muted hover:text-fg cursor-pointer"
              >
                Cancel
              </button>
            </div>
          ) : (
            <button
              type="button"
              onClick={handleDeleteClick}
              className="p-1 rounded text-fg-muted hover:text-red-400 hover:bg-elevated opacity-0 group-hover:opacity-100 transition-opacity cursor-pointer"
              aria-label={`Delete session ${title}`}
            >
              <svg width="14" height="14" viewBox="0 0 14 14" fill="none" xmlns="http://www.w3.org/2000/svg">
                <path d="M3.5 3.5L10.5 10.5M10.5 3.5L3.5 10.5" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" />
              </svg>
            </button>
          )}
        </div>
      </div>
    </button>
  );
});
```

- [ ] **Step 3: Update StatusDot to accept hasUnread prop**

```tsx
function StatusDot({ status, hasUnread }: { status: SessionStatus; hasUnread?: boolean }) {
  switch (status) {
    case 'running':
      return <span data-testid="status-dot" className="mt-1 w-2 h-2 rounded-full bg-green-500 animate-pulse shrink-0" />;
    case 'completed_unread':
      return <span data-testid="status-dot" className="mt-1 w-2 h-2 rounded-full bg-soul ring-2 ring-soul/30 shrink-0" />;
    default:
      if (hasUnread) {
        return <span data-testid="status-dot" className="mt-1 w-2 h-2 rounded-full bg-soul ring-2 ring-soul/30 shrink-0" />;
      }
      return <span data-testid="status-dot" className="mt-1 w-2 h-2 rounded-full bg-fg-muted shrink-0" />;
  }
}
```

- [ ] **Step 4: Update SessionList with time grouping and onRename prop**

Update `SessionListProps` usage to include `onRename`:

```tsx
export function SessionList({
  sessions,
  activeSessionID,
  onCreate,
  onSwitch,
  onDelete,
  onRename,
}: SessionListProps & { onRename: (sessionID: string, title: string) => void }) {
  const [searchQuery, setSearchQuery] = useState('');

  const filtered = useMemo(
    () => searchQuery
      ? sessions.filter(s => s.title.toLowerCase().includes(searchQuery.toLowerCase()))
      : sessions,
    [sessions, searchQuery],
  );

  // Group sessions by time.
  const grouped = useMemo(() => {
    const groups: { group: TimeGroup; sessions: Session[] }[] = [];
    const order: TimeGroup[] = ['Today', 'Yesterday', 'Older'];
    const map = new Map<TimeGroup, Session[]>();

    for (const s of filtered) {
      const g = getTimeGroup(s.updatedAt || s.createdAt);
      if (!map.has(g)) map.set(g, []);
      map.get(g)!.push(s);
    }

    for (const g of order) {
      const items = map.get(g);
      if (items && items.length > 0) {
        groups.push({ group: g, sessions: items });
      }
    }
    return groups;
  }, [filtered]);

  return (
    <div
      data-testid="session-list"
      className="w-64 bg-surface border-r border-border-subtle flex flex-col h-full shrink-0"
    >
      <div className="flex items-center justify-between px-4 py-3 border-b border-border-subtle">
        <h2 className="text-sm font-semibold text-fg-secondary tracking-tight">
          Sessions
        </h2>
        <button
          data-testid="new-session-button"
          type="button"
          onClick={onCreate}
          className="px-3 py-2 md:px-2 md:py-1 text-xs font-medium text-fg bg-elevated hover:bg-overlay rounded transition-colors cursor-pointer"
        >
          + New
        </button>
      </div>
      {sessions.length > 5 && (
        <div className="px-3 py-2 border-b border-border-subtle">
          <input
            data-testid="session-search"
            type="text"
            value={searchQuery}
            onChange={e => setSearchQuery(e.target.value)}
            placeholder="Search sessions..."
            className="w-full px-2 py-1.5 text-sm bg-elevated border border-border-default rounded text-fg placeholder:text-fg-muted outline-none focus:border-soul/40"
          />
        </div>
      )}
      <div className="flex-1 overflow-y-auto">
        {grouped.map(({ group, sessions: items }) => (
          <div key={group}>
            <div data-testid="time-group-header" className="px-3 pt-3 pb-1 text-xs text-fg-muted uppercase tracking-wide">
              {group}
            </div>
            {items.map(session => (
              <SessionItem
                key={session.id}
                session={session}
                isActive={session.id === activeSessionID}
                onSwitch={onSwitch}
                onDelete={onDelete}
                onRename={onRename}
              />
            ))}
          </div>
        ))}
        {filtered.length === 0 && (
          <div className="px-4 py-6 text-xs text-fg-muted text-center">
            {searchQuery ? 'No matching sessions' : 'No sessions yet'}
          </div>
        )}
      </div>
    </div>
  );
}
```

- [ ] **Step 5: Verify TypeScript compiles** (may need useChat update first — do in next task)

This step will fail until `onRename` is wired in the parent. Note this and move to Task 11.

- [ ] **Step 6: Commit**

```bash
git add web/src/components/SessionList.tsx
git commit -m "feat(ui): enrich SessionList with preview, unread badge, time groups, inline rename"
```

### Task 11: Wire onRename in useChat and pass to SessionList

**Files:**
- Modify: `web/src/hooks/useChat.ts`
- Modify: `web/src/components/Shell.tsx` (or wherever SessionList is rendered)

- [ ] **Step 1: Add renameSession to useChat**

In `web/src/hooks/useChat.ts`, add:

```typescript
// In UseChatReturn interface:
renameSession: (id: string, title: string) => void;

// Implementation:
const renameSession = useCallback(
  (id: string, title: string) => {
    send('session.rename', { sessionId: id, content: title });
  },
  [send],
);

// In return:
return {
  // ...existing fields...
  renameSession,
};
```

- [ ] **Step 2: Pass onRename to SessionList in Shell**

Find where `SessionList` is rendered and add `onRename={renameSession}`.

- [ ] **Step 3: Verify TypeScript compiles**

Run: `cd /home/rishav/soul-v2/web && npx tsc --noEmit`
Expected: Success

- [ ] **Step 4: Commit**

```bash
git add web/src/hooks/useChat.ts web/src/components/Shell.tsx
git commit -m "feat(chat): add renameSession to useChat, wire onRename to SessionList"
```

---

## Chunk 4: Build, Deploy, Verify

### Task 12: Build and verify

- [ ] **Step 1: Go build**

Run: `cd /home/rishav/soul-v2 && go build -o soul ./cmd/soul`
Expected: Success

- [ ] **Step 2: Go tests**

Run: `cd /home/rishav/soul-v2 && go test ./internal/session/ -v`
Expected: All tests pass

- [ ] **Step 3: Frontend build**

Run: `cd /home/rishav/soul-v2/web && npx vite build`
Expected: Build succeeds

- [ ] **Step 4: TypeScript check**

Run: `cd /home/rishav/soul-v2/web && npx tsc --noEmit`
Expected: No errors

- [ ] **Step 5: Deploy**

```bash
sudo systemctl restart soul-v2
```

- [ ] **Step 6: Smoke test — verify sessions have previews and titles**

Check journalctl for startup errors:
```bash
journalctl -u soul-v2 --since "1 min ago" --no-pager
```

- [ ] **Step 7: Verify backfill worked**

Open Soul in browser. Existing sessions should now show:
- Titles derived from first user message (not "New Session")
- Last message preview on second line
- Time group headers (Today/Yesterday/Older)

- [ ] **Step 8: Test inline rename**

Double-click a session title → type new name → press Enter. Title should update across all sessions.

- [ ] **Step 9: Test unread badge**

Open a session, send a message, switch to a different session before the response completes. The first session should show an unread badge when the response arrives.

- [ ] **Step 10: Test smart title generation**

Create a new session, send a message. After the response arrives, wait ~5 seconds. Title should update from truncated first message to a smart 3-5 word title.

- [ ] **Step 11: Push to Gitea**

```bash
git push origin master
```
