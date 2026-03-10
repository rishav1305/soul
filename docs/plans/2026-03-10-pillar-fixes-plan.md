# Pillar Violation Fixes — Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Fix all violations found in the 5-pillar audit: security, bundle size, resilience, performance, and type safety.

**Architecture:** Seven incremental phases ordered by severity. Backend fixes get Go integration tests; frontend fixes get `make check-bundle` and Puppeteer E2E on titan-pc. Each phase ends with `make verify` passing.

**Tech Stack:** Go 1.24, SQLite (modernc.org/sqlite), React 19, Vite 7, TypeScript 5.9, Tailwind CSS v4

---

### Task 1: Security — Path Traversal Fix

**Files:**
- Modify: `internal/server/server.go:502-519`
- Modify: `internal/server/server_test.go` (add test)

**Step 1: Write the failing test**

Add to `internal/server/server_test.go`:

```go
func TestSPAHandler_PathTraversal(t *testing.T) {
	// Set up a temp directory with an index.html.
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "index.html"), []byte("<html>ok</html>"), 0644)
	os.MkdirAll(filepath.Join(dir, "assets"), 0755)
	os.WriteFile(filepath.Join(dir, "assets", "app.js"), []byte("console.log('ok')"), 0644)

	srv := newTestServer(t, WithStaticDir(dir))

	// Traversal attempts — all should serve index.html (SPA fallback), not leak files.
	traversals := []string{
		"/../../../etc/passwd",
		"/..%2f..%2f..%2fetc/passwd",
		"/%2e%2e/%2e%2e/etc/passwd",
		"/assets/../../etc/passwd",
	}

	for _, path := range traversals {
		req := httptest.NewRequest("GET", path, nil)
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)

		// Should NOT return actual file content from outside staticDir.
		body := rec.Body.String()
		if strings.Contains(body, "root:") {
			t.Errorf("path traversal succeeded for %s", path)
		}
	}

	// Valid file should still work.
	req := httptest.NewRequest("GET", "/assets/app.js", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("valid file returned %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "console.log") {
		t.Error("valid file content not served")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test -run TestSPAHandler_PathTraversal -v ./internal/server/`
Expected: May pass on some OS (filepath.Clean sanitizes `..`), may fail on encoded paths. The test establishes the contract.

**Step 3: Implement the fix**

In `internal/server/server.go`, replace lines 502-519 (inside the spaHandler method):

```go
	// Clean the path to prevent directory traversal.
	cleanPath := filepath.Clean(r.URL.Path)
	if cleanPath == "/" {
		cleanPath = "/index.html"
	}

	// Resolve to absolute path and verify it's within staticDir.
	absDir, _ := filepath.Abs(s.staticDir)
	filePath := filepath.Join(s.staticDir, cleanPath)
	absPath, _ := filepath.Abs(filePath)
	if !strings.HasPrefix(absPath, absDir+string(filepath.Separator)) && absPath != absDir {
		// Path escapes static directory — serve SPA fallback.
		s.serveIndexHTML(w, r)
		return
	}

	// Try to serve the requested file.
	info, err := os.Stat(filePath)
	if err != nil || info.IsDir() {
		// File not found — serve index.html (SPA fallback).
		s.serveIndexHTML(w, r)
		return
	}

	// File exists — serve it with appropriate cache headers.
	s.setCacheHeaders(w, cleanPath)
	http.ServeFile(w, r, filePath)
```

**Step 4: Run test to verify it passes**

Run: `go test -run TestSPAHandler_PathTraversal -v ./internal/server/`
Expected: PASS

**Step 5: Run full verify**

Run: `make verify-static && make verify-unit`
Expected: All pass

**Step 6: Commit**

```bash
git add internal/server/server.go internal/server/server_test.go
git commit -m "fix: prevent path traversal in SPA handler"
```

---

### Task 2: Security — Remove CSP unsafe-inline

**Files:**
- Modify: `internal/server/server.go:578-579`
- Modify: `internal/server/server_test.go:70-71`

**Step 1: Update CSP header**

In `internal/server/server.go:578-579`, change:
```go
		w.Header().Set("Content-Security-Policy",
			"default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; connect-src 'self' ws: wss:; frame-ancestors 'none'; base-uri 'self'")
```
to:
```go
		w.Header().Set("Content-Security-Policy",
			"default-src 'self'; script-src 'self'; style-src 'self'; connect-src 'self' ws: wss:; frame-ancestors 'none'; base-uri 'self'")
```

**Step 2: Update the CSP test**

In `internal/server/server_test.go:70-71`, change:
```go
	if !strings.Contains(csp, "style-src 'self' 'unsafe-inline'") {
```
to:
```go
	if !strings.Contains(csp, "style-src 'self'") {
```

And add a negative assertion:
```go
	if strings.Contains(csp, "unsafe-inline") {
		t.Errorf("CSP should not contain unsafe-inline: %s", csp)
	}
```

**Step 3: Run tests**

Run: `go test -run TestCSP -v ./internal/server/`
Expected: PASS

**Step 4: Build and verify frontend still renders**

Run: `make build && make verify-static`
Expected: All pass. Tailwind v4 generates external CSS files — no inline styles needed.

**Step 5: Commit**

```bash
git add internal/server/server.go internal/server/server_test.go
git commit -m "fix: remove unsafe-inline from CSP style-src"
```

---

### Task 3: Performance — Remove Mermaid, Fix Bundle Size

**Files:**
- Delete: `web/src/components/MermaidBlock.tsx`
- Modify: `web/src/components/CodeBlock.tsx:1-5,94-101`
- Modify: `web/package.json` (remove mermaid)
- Modify: `web/src/app.css` (remove mermaid theme vars if any)

**Step 1: Remove mermaid from package.json**

Run: `cd web && npm uninstall mermaid`

**Step 2: Delete MermaidBlock.tsx**

```bash
rm web/src/components/MermaidBlock.tsx
```

**Step 3: Simplify CodeBlock.tsx**

Remove lines 1 (lazy import), 5 (MermaidBlock lazy), and 94-101 (mermaid branch):

Replace the top of `CodeBlock.tsx`:
```tsx
import { useState, useCallback } from 'react';
import SyntaxHighlighter from 'react-syntax-highlighter/dist/esm/prism-light';
import oneDark from 'react-syntax-highlighter/dist/esm/styles/prism/one-dark';
```

(Remove `lazy`, `Suspense` imports and the `const MermaidBlock = lazy(...)` line.)

Replace the mermaid branch at line 94-101:
```tsx
export default function CodeBlock({ language, code }: CodeBlockProps) {
  const [copied, setCopied] = useState(false);
```

(Remove the `if (language.toLowerCase() === 'mermaid')` block entirely. Mermaid code will fall through to regular syntax-highlighted rendering.)

**Step 4: Verify TypeScript compiles**

Run: `cd web && npx tsc --noEmit`
Expected: No errors

**Step 5: Build and check bundle size**

Run: `cd web && npx vite build`
Expected: No mermaid/cytoscape/katex chunks in output. Gzipped total should drop significantly.

Run: `make check-bundle`
Expected: Should pass (< 300KB) or be much closer to target.

**Step 6: Commit**

```bash
git add -u web/
git commit -m "perf: remove mermaid dependency (-348KB gzipped)"
```

---

### Task 4: Resilience — Persist Incomplete Streams

**Files:**
- Modify: `internal/ws/handler.go:291-303`
- Add test to: `internal/ws/handler_test.go`

**Step 1: Write the failing test**

Add to `internal/ws/handler_test.go`:

```go
func TestRunStream_IncompleteStreamPersisted(t *testing.T) {
	// This test verifies that when a stream ends without message_stop,
	// the partial text is still persisted to the database.
	store := openTestSessionStore(t)
	sess, _ := store.CreateSession("test")
	store.AddMessage(sess.ID, "user", "hello")

	// After an incomplete stream, the handler should still store partial text.
	// We verify by checking messages in the session after the handler returns.
	msgs, err := store.GetMessages(sess.ID)
	if err != nil {
		t.Fatal(err)
	}
	// Initially: 1 user message
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(msgs))
	}
}
```

Note: The full integration test for stream interruption requires a mock stream client. Add this to `tests/integration/stream_errors_test.go` where mock infrastructure already exists.

**Step 2: Implement the fix**

In `internal/ws/handler.go`, replace lines 291-303:

```go
	// If stream ended without message_stop, it was truncated.
	if !gotMessageStop {
		log.Printf("ws: stream ended without message_stop for session %s", sessionID)
		if h.metrics != nil {
			_ = h.metrics.Log(metrics.EventAPIError, map[string]interface{}{
				"session_id":    sessionID,
				"error_type":    "incomplete_stream",
				"status_code":   0,
				"error_message": "stream ended without message_stop",
			})
		}

		// Persist partial response so the user keeps what they received.
		if fullText.Len() > 0 {
			partial := fullText.String() + "\n\n[incomplete — stream ended unexpectedly]"
			if _, err := h.sessionStore.AddMessage(sessionID, "assistant", partial); err != nil {
				log.Printf("ws: failed to store partial message for session %s: %v", sessionID, err)
			}
		}

		h.completeSession(client, sessionID)
		h.sendError(client, sessionID, "stream ended unexpectedly")
		return
	}
```

**Step 3: Run tests**

Run: `go test -v ./internal/ws/`
Expected: PASS

Run: `make verify-unit`
Expected: All pass

**Step 4: Commit**

```bash
git add internal/ws/handler.go
git commit -m "fix: persist partial response on incomplete stream"
```

---

### Task 5: Resilience — Transactional Message Storage

**Files:**
- Modify: `internal/session/store.go` (add `RunInTransaction` + `AddMessageTx`)
- Modify: `internal/session/store_test.go` (add transaction test)

**Step 1: Write the failing test**

Add to `internal/session/store_test.go`:

```go
func TestAddMessage_TransactionRollback(t *testing.T) {
	s := openTestStore(t)
	sess, _ := s.CreateSession("test")

	// Add a message inside a transaction, then roll back.
	err := s.RunInTransaction(func(tx *sql.Tx) error {
		_, err := s.AddMessageTx(tx, sess.ID, "user", "hello")
		if err != nil {
			return err
		}
		return fmt.Errorf("simulated failure")
	})
	if err == nil {
		t.Fatal("expected error from transaction")
	}

	// Message should NOT be persisted.
	msgs, _ := s.GetMessages(sess.ID)
	if len(msgs) != 0 {
		t.Errorf("expected 0 messages after rollback, got %d", len(msgs))
	}
}

func TestRunInTransaction_CommitsOnSuccess(t *testing.T) {
	s := openTestStore(t)
	sess, _ := s.CreateSession("test")

	err := s.RunInTransaction(func(tx *sql.Tx) error {
		_, err := s.AddMessageTx(tx, sess.ID, "user", "hello")
		return err
	})
	if err != nil {
		t.Fatal(err)
	}

	msgs, _ := s.GetMessages(sess.ID)
	if len(msgs) != 1 {
		t.Errorf("expected 1 message, got %d", len(msgs))
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test -run TestAddMessage_TransactionRollback -v ./internal/session/`
Expected: FAIL — `RunInTransaction` and `AddMessageTx` don't exist yet.

**Step 3: Implement RunInTransaction and AddMessageTx**

Add to `internal/session/store.go`:

```go
// RunInTransaction executes fn inside a SQL transaction. If fn returns an error,
// the transaction is rolled back; otherwise it is committed.
func (s *Store) RunInTransaction(fn func(tx *sql.Tx) error) error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("session: begin transaction: %w", err)
	}
	if err := fn(tx); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

// AddMessageTx adds a message within an existing transaction.
func (s *Store) AddMessageTx(tx *sql.Tx, sessionID, role, content string) (*Message, error) {
	if !validRoles[role] {
		return nil, fmt.Errorf("session: invalid role: %q", role)
	}
	if !uuidRe.MatchString(sessionID) {
		return nil, fmt.Errorf("session: invalid UUID format: %q", sessionID)
	}

	now := time.Now().UTC()
	msg := &Message{
		ID:        newUUID(),
		SessionID: sessionID,
		Role:      role,
		Content:   content,
		CreatedAt: now,
	}

	_, err := tx.Exec(
		"INSERT INTO messages (id, session_id, role, content, created_at) VALUES (?, ?, ?, ?, ?)",
		msg.ID, msg.SessionID, msg.Role, msg.Content, msg.CreatedAt.Format(time.RFC3339Nano),
	)
	if err != nil {
		return nil, fmt.Errorf("session: add message: %w", err)
	}

	_, err = tx.Exec(
		"UPDATE sessions SET message_count = message_count + 1, updated_at = ? WHERE id = ?",
		now.Format(time.RFC3339Nano), sessionID,
	)
	if err != nil {
		return nil, fmt.Errorf("session: update message count: %w", err)
	}

	return msg, nil
}
```

Add `"database/sql"` to imports if not already present.

**Step 4: Run tests to verify they pass**

Run: `go test -run "TestAddMessage_Transaction|TestRunInTransaction" -v ./internal/session/`
Expected: PASS

Run: `make verify-unit`
Expected: All pass

**Step 5: Commit**

```bash
git add internal/session/store.go internal/session/store_test.go
git commit -m "feat: add RunInTransaction and AddMessageTx for atomic message storage"
```

---

### Task 6: Resilience — Close Slow Clients Instead of Dropping Messages

**Files:**
- Modify: `internal/ws/client.go:118-136`
- Modify: `internal/ws/client_test.go` (update test)

**Step 1: Write the failing test**

Add to `internal/ws/client_test.go`:

```go
func TestSend_ClosesSlowClient(t *testing.T) {
	// Create a client with a full send channel.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	c := &Client{
		id:   "test-slow",
		send: make(chan []byte, 2), // Small buffer for testing.
		ctx:  ctx,
		cancel: cancel,
	}

	// Fill the channel.
	c.send <- []byte("msg1")
	c.send <- []byte("msg2")

	// Next send should close the client (return false).
	ok := c.Send([]byte("msg3"))
	if ok {
		t.Error("expected Send to return false for slow client")
	}
	if !c.sendDone {
		t.Error("expected send channel to be closed")
	}
}
```

**Step 2: Implement the fix**

In `internal/ws/client.go`, replace the `Send` method (lines 110-137):

```go
// Send queues a message for delivery to the client. If the send channel
// is full, the client is considered too slow — the connection is closed
// so the client can reconnect and receive history.
// Safe to call after the channel has been closed — returns false in that case.
func (c *Client) Send(msg []byte) bool {
	c.sendMu.Lock()
	defer c.sendMu.Unlock()

	if c.sendDone {
		return false
	}

	select {
	case c.send <- msg:
		return true
	default:
		// Channel full — close slow client instead of silently dropping messages.
		log.Printf("ws: client %s send channel full, closing slow client", c.id)
		c.sendDone = true
		close(c.send)
		return false
	}
}
```

**Step 3: Run tests**

Run: `go test -run TestSend -v ./internal/ws/`
Expected: PASS

Run: `make verify-unit`
Expected: All pass

**Step 4: Commit**

```bash
git add internal/ws/client.go internal/ws/client_test.go
git commit -m "fix: close slow clients instead of silently dropping messages"
```

---

### Task 7: Resilience — Stream Client Timeout

**Files:**
- Modify: `internal/ws/handler.go:200-201`

**Step 1: Add timeout to stream context**

In `internal/ws/handler.go`, replace lines 200-201 in `runStream`:

```go
	// Use the client's context so the stream is cancelled on disconnect.
	ctx := client.Context()
```

with:

```go
	// Use the client's context with a 5-minute deadline.
	// If the Claude API hangs, the stream times out instead of blocking forever.
	ctx, cancel := context.WithTimeout(client.Context(), 5*time.Minute)
	defer cancel()
```

**Step 2: Run tests**

Run: `make verify-unit && make verify-integ`
Expected: All pass

**Step 3: Commit**

```bash
git add internal/ws/handler.go
git commit -m "fix: add 5-minute timeout to streaming requests"
```

---

### Task 8: Resilience — Pending Message Persistence in Frontend

**Files:**
- Modify: `web/src/hooks/useChat.ts:362-408`

**Step 1: Add localStorage persistence for pending messages**

In `web/src/hooks/useChat.ts`, modify the `sendMessage` function (lines 362-395). Replace the deferred session creation block:

```typescript
      // Deferred session creation: if no session, create one first then send.
      if (!sessionIDRef.current) {
        // Create session, then send message after session.created arrives.
        const pendingMessage = { content: trimmed, options };
        pendingMessageRef.current = pendingMessage;
        // Persist to localStorage in case browser refreshes before session.created.
        try {
          localStorage.setItem('soul-v2-pending', JSON.stringify(pendingMessage));
        } catch { /* quota exceeded — proceed without persistence */ }
        send('session.create', {});
        return;
      }
```

In the `connection.ready` handler (lines 49-61), add pending message recovery after session restore:

```typescript
        case 'connection.ready': {
          setAuthError(false);
          // Restore last session from localStorage on reconnect.
          const savedId = localStorage.getItem(STORAGE_KEY);
          if (savedId && !sessionIDRef.current) {
            sessionIDRef.current = savedId;
            setCurrentSessionID(savedId);
            queueMicrotask(() => {
              sendRef.current('session.switch', { sessionId: savedId });
            });
          }
          // Recover pending message from localStorage (browser refresh during deferred creation).
          const pendingRaw = localStorage.getItem('soul-v2-pending');
          if (pendingRaw && !pendingMessageRef.current) {
            try {
              const pending = JSON.parse(pendingRaw);
              if (pending?.content) {
                pendingMessageRef.current = pending;
                // If no session, create one; the useEffect will send after session.created.
                if (!sessionIDRef.current) {
                  sendRef.current('session.create', {});
                }
              }
            } catch { /* corrupted data — ignore */ }
          }
          break;
        }
```

In the pending message useEffect (lines 401-408), clear localStorage after send:

```typescript
  useEffect(() => {
    if (currentSessionID && pendingMessageRef.current) {
      const { content, options } = pendingMessageRef.current;
      pendingMessageRef.current = null;
      localStorage.removeItem('soul-v2-pending');
      // Small delay to ensure session is fully registered.
      setTimeout(() => sendMessage(content, options), 50);
    }
  }, [currentSessionID, sendMessage]);
```

Also clear `soul-v2-pending` in the `chat.done` handler (after line 234):

```typescript
        case 'chat.done': {
          // ...existing code...
          setIsStreaming(false);
          setAuthError(false);
          localStorage.removeItem('soul-v2-pending');
          break;
        }
```

**Step 2: Verify TypeScript compiles**

Run: `cd web && npx tsc --noEmit`
Expected: No errors

**Step 3: Build**

Run: `cd web && npx vite build`
Expected: Build succeeds

**Step 4: Commit**

```bash
git add web/src/hooks/useChat.ts
git commit -m "fix: persist pending message to localStorage for refresh recovery"
```

---

### Task 9: Performance — React.memo SessionItem + Stable Callbacks

**Files:**
- Modify: `web/src/components/SessionList.tsx`

**Step 1: Wrap SessionItem in React.memo and change callback signature**

Replace the entire `SessionList.tsx` SessionItem declaration (line 16) and SessionList render (lines 179-186):

At line 16, change:
```tsx
function SessionItem({
```
to:
```tsx
const SessionItem = React.memo(function SessionItem({
```

Change the props interface to accept `id` + handler:
```tsx
const SessionItem = React.memo(function SessionItem({
  session,
  isActive,
  onSwitch,
  onDelete,
}: {
  session: Session;
  isActive: boolean;
  onSwitch: (id: string) => void;
  onDelete: (id: string) => void;
}) {
```

Update the internal handlers in SessionItem to call with id:
```tsx
  // Line ~71 — replace onClick={onSwitch} with:
  onClick={() => onSwitch(session.id)}

  // Line ~51 — replace onDelete() with:
  onDelete(session.id);
```

Close the React.memo at the end of the component (after the return statement's closing `);`):
```tsx
});
```

In the SessionList render (lines 179-186), remove the inline arrow functions:
```tsx
        {filtered.map(session => (
          <SessionItem
            key={session.id}
            session={session}
            isActive={session.id === activeSessionID}
            onSwitch={onSwitch}
            onDelete={onDelete}
          />
        ))}
```

Add `import React` to the imports if needed (React 19 JSX transform may not need it, but React.memo does):
```tsx
import React, { useState, useCallback, useEffect, useRef } from 'react';
```

**Step 2: Memoize filtered sessions**

Replace lines 144-146:
```tsx
  const filtered = useMemo(
    () => searchQuery
      ? sessions.filter(s => s.title.toLowerCase().includes(searchQuery.toLowerCase()))
      : sessions,
    [sessions, searchQuery],
  );
```

Add `useMemo` to the import.

**Step 3: Verify TypeScript**

Run: `cd web && npx tsc --noEmit`
Expected: No errors

**Step 4: Build**

Run: `cd web && npx vite build`
Expected: Build succeeds

**Step 5: Commit**

```bash
git add web/src/components/SessionList.tsx
git commit -m "perf: memoize SessionItem and session filtering"
```

---

### Task 10: Performance — Memoize HighlightText Regex

**Files:**
- Modify: `web/src/components/MessageBubble.tsx:315-330`

**Step 1: Memoize the regex in HighlightText**

Replace lines 315-330:

```tsx
function HighlightText({ text, query }: { text: string; query?: string }) {
  const parts = useMemo(() => {
    if (!query) return null;
    const escaped = query.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
    return text.split(new RegExp(`(${escaped})`, 'gi'));
  }, [text, query]);

  if (!parts) return <>{text}</>;
  return (
    <>
      {parts.map((part, i) =>
        part.toLowerCase() === query!.toLowerCase() ? (
          <mark key={i} className="bg-soul/30 text-inherit rounded px-0.5">{part}</mark>
        ) : (
          part
        ),
      )}
    </>
  );
}
```

Add `useMemo` to the imports at the top of the file if not already present.

**Step 2: Fix copy timeout cleanup**

Replace the CopyBtn `handleCopy` (lines 52-70) to use a ref for timeout cleanup:

```tsx
function CopyBtn({ content }: { content: string }) {
  const [copied, setCopied] = useState(false);
  const timerRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  useEffect(() => {
    return () => {
      if (timerRef.current) clearTimeout(timerRef.current);
    };
  }, []);

  const handleCopy = useCallback(() => {
    const fallback = () => {
      const ta = document.createElement('textarea');
      ta.value = content;
      ta.style.position = 'fixed';
      ta.style.opacity = '0';
      document.body.appendChild(ta);
      ta.select();
      document.execCommand('copy');
      document.body.removeChild(ta);
    };
    if (navigator.clipboard?.writeText) {
      navigator.clipboard.writeText(content).catch(fallback);
    } else {
      fallback();
    }
    setCopied(true);
    if (timerRef.current) clearTimeout(timerRef.current);
    timerRef.current = setTimeout(() => setCopied(false), 1500);
  }, [content]);
```

Add `useRef, useEffect` to imports if not already present.

**Step 3: Verify and build**

Run: `cd web && npx tsc --noEmit && npx vite build`
Expected: No errors

**Step 4: Commit**

```bash
git add web/src/components/MessageBubble.tsx
git commit -m "perf: memoize HighlightText regex, fix copy timeout cleanup"
```

---

### Task 11: Performance — Fix CodeBlock Copy Timeout

**Files:**
- Modify: `web/src/components/CodeBlock.tsx:103-123`

**Step 1: Add timeout cleanup**

Replace lines 103-123 in CodeBlock.tsx:

```tsx
  const [copied, setCopied] = useState(false);
  const timerRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  useEffect(() => {
    return () => {
      if (timerRef.current) clearTimeout(timerRef.current);
    };
  }, []);

  const handleCopy = useCallback(() => {
    const fallback = () => {
      const ta = document.createElement('textarea');
      ta.value = code;
      ta.style.position = 'fixed';
      ta.style.opacity = '0';
      document.body.appendChild(ta);
      ta.select();
      document.execCommand('copy');
      document.body.removeChild(ta);
    };
    if (navigator.clipboard?.writeText) {
      navigator.clipboard.writeText(code).catch(fallback);
    } else {
      fallback();
    }
    setCopied(true);
    if (timerRef.current) clearTimeout(timerRef.current);
    timerRef.current = setTimeout(() => setCopied(false), 2000);
  }, [code]);
```

Add `useRef, useEffect` to the imports line (already has `useState, useCallback`).

**Step 2: Verify and build**

Run: `cd web && npx tsc --noEmit && npx vite build`
Expected: No errors

**Step 3: Commit**

```bash
git add web/src/components/CodeBlock.tsx
git commit -m "perf: fix CodeBlock copy timeout cleanup"
```

---

### Task 12: Resilience — Atomic Session Status Transitions

**Files:**
- Modify: `internal/session/store.go:293-319` (UpdateSessionStatus)
- Modify: `internal/session/store_test.go` (add concurrency test)

**Step 1: Write the concurrency test**

Add to `internal/session/store_test.go`:

```go
func TestUpdateSessionStatus_AtomicTransition(t *testing.T) {
	s := openTestStore(t)
	sess, _ := s.CreateSession("test")

	// Transition to running (valid: idle -> running).
	if err := s.UpdateSessionStatus(sess.ID, StatusRunning); err != nil {
		t.Fatal(err)
	}

	// Two concurrent attempts to transition from running.
	// Only one should succeed — the other should get an invalid transition error.
	var wg sync.WaitGroup
	var mu sync.Mutex
	var errors []error

	for i := 0; i < 2; i++ {
		wg.Add(1)
		status := StatusCompleted
		if i == 1 {
			status = StatusCompletedUnread
		}
		go func(s2 Status) {
			defer wg.Done()
			err := s.UpdateSessionStatus(sess.ID, s2)
			mu.Lock()
			errors = append(errors, err)
			mu.Unlock()
		}(status)
	}
	wg.Wait()

	// With single connection (MaxOpenConns=1), both should succeed sequentially.
	// But the second one should fail because the first already transitioned.
	successCount := 0
	for _, err := range errors {
		if err == nil {
			successCount++
		}
	}
	if successCount != 1 {
		t.Errorf("expected exactly 1 success, got %d", successCount)
	}
}
```

**Step 2: Run test to see current behavior**

Run: `go test -run TestUpdateSessionStatus_AtomicTransition -v ./internal/session/`
Expected: With `MaxOpenConns(1)`, operations are serialized, so one succeeds, one fails. The test should already pass due to single-writer serialization.

**Step 3: Make the transition explicitly atomic (defense in depth)**

Replace `UpdateSessionStatus` in `store.go`:

```go
// UpdateSessionStatus transitions a session to a new status atomically.
// The read-check-update is wrapped in a transaction to prevent race conditions.
func (s *Store) UpdateSessionStatus(id string, status Status) error {
	if !uuidRe.MatchString(id) {
		return fmt.Errorf("session: invalid UUID format: %q", id)
	}
	if !status.Valid() {
		return fmt.Errorf("session: invalid status: %q", status)
	}

	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("session: begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Read current status inside the transaction.
	var currentStatus string
	err = tx.QueryRow("SELECT status FROM sessions WHERE id = ?", id).Scan(&currentStatus)
	if err == sql.ErrNoRows {
		return fmt.Errorf("session: not found: %s", id)
	}
	if err != nil {
		return fmt.Errorf("session: get status: %w", err)
	}

	current := Status(currentStatus)
	if !current.CanTransitionTo(status) {
		return fmt.Errorf("session: invalid transition from %q to %q", current, status)
	}

	now := time.Now().UTC()
	_, err = tx.Exec(
		"UPDATE sessions SET status = ?, updated_at = ? WHERE id = ?",
		string(status), now.Format(time.RFC3339Nano), id,
	)
	if err != nil {
		return fmt.Errorf("session: update status: %w", err)
	}

	return tx.Commit()
}
```

Add `"database/sql"` to imports if not already present.

**Step 4: Run all tests**

Run: `make verify-unit`
Expected: All pass

**Step 5: Commit**

```bash
git add internal/session/store.go internal/session/store_test.go
git commit -m "fix: wrap session status transitions in transaction"
```

---

### Task 13: Resilience — Reauth Retry with Exponential Backoff

**Files:**
- Modify: `web/src/hooks/useChat.ts:441-448`

**Step 1: Add retry logic**

Replace the `reauth` callback (lines 441-448):

```typescript
  const reauth = useCallback(async () => {
    const MAX_RETRIES = 3;
    for (let attempt = 0; attempt < MAX_RETRIES; attempt++) {
      try {
        const resp = await fetch('/api/reauth', { method: 'POST' });
        if (resp.ok) {
          setAuthError(false);
          return;
        }
      } catch {
        // Network error — retry after backoff.
      }
      // Exponential backoff: 1s, 2s, 4s.
      if (attempt < MAX_RETRIES - 1) {
        await new Promise(r => setTimeout(r, 1000 * Math.pow(2, attempt)));
      }
    }
    // All retries failed — keep authError true so UI shows re-auth button.
  }, []);
```

**Step 2: Verify TypeScript**

Run: `cd web && npx tsc --noEmit`
Expected: No errors

**Step 3: Commit**

```bash
git add web/src/hooks/useChat.ts
git commit -m "fix: add exponential backoff retry to reauth"
```

---

### Task 14: Resilience — Increase Broadcast Channel Capacity

**Files:**
- Modify: `internal/ws/hub.go:89-90`

**Step 1: Increase capacity and add warning log**

In `hub.go`, change lines 89-90:
```go
		broadcastCh:    make(chan []byte, 1024),
		sessionBcastCh: make(chan sessionBroadcast, 1024),
```

**Step 2: Run tests**

Run: `make verify-unit && make verify-integ`
Expected: All pass

**Step 3: Commit**

```bash
git add internal/ws/hub.go
git commit -m "fix: increase broadcast channel capacity to 1024"
```

---

### Task 15: Robust — Tighten Web Speech API Types

**Files:**
- Modify: `web/src/components/ChatInput.tsx:48-61`

**Step 1: Replace `as any` with proper types**

Replace lines 48-61:

```tsx
interface SpeechRecognitionEvent {
  results: { readonly length: number; item(index: number): { readonly length: number; item(index: number): { transcript: string } }; [index: number]: { readonly length: number; item(index: number): { transcript: string }; [index: number]: { transcript: string } } };
  resultIndex: number;
}

interface SpeechRecognitionErrorEvent {
  error: string;
  message: string;
}

interface SpeechRecognitionInstance {
  continuous: boolean;
  interimResults: boolean;
  lang: string;
  start(): void;
  stop(): void;
  onresult: ((event: SpeechRecognitionEvent) => void) | null;
  onerror: ((event: SpeechRecognitionErrorEvent) => void) | null;
  onend: (() => void) | null;
}

const SpeechRecognition = (typeof window !== 'undefined')
  ? ((window as Record<string, unknown>).SpeechRecognition || (window as Record<string, unknown>).webkitSpeechRecognition) as (new () => SpeechRecognitionInstance) | undefined
  : undefined;
```

Update the `onresult` handler (around line 190) to use the new type:
```tsx
      recognition.onresult = (event: SpeechRecognitionEvent) => {
```

**Step 2: Type session history loop parameter**

In `web/src/hooks/useChat.ts:147`, change:
```tsx
            const hydrated = payload.messages.map((m: any) => ({
```
to:
```tsx
            const hydrated = payload.messages.map((m: { id: string; role: string; content: string; sessionId?: string; session_id?: string; createdAt?: string; created_at?: string; model?: string; thinking?: string; toolCalls?: ToolCallData[]; usage?: { inputTokens: number; outputTokens: number; cacheReadInputTokens?: number } }) => ({
```

**Step 3: Verify TypeScript**

Run: `cd web && npx tsc --noEmit`
Expected: No errors (may need adjustments to match actual event shape)

**Step 4: Commit**

```bash
git add web/src/components/ChatInput.tsx web/src/hooks/useChat.ts
git commit -m "fix: tighten Web Speech API and session history types"
```

---

### Task 16: Full Verification

**Step 1: Run the full verification stack**

```bash
make verify
```
Expected: All L1-L3 checks pass.

**Step 2: Run bundle check**

```bash
make check-bundle
```
Expected: Total gzipped < 300KB.

**Step 3: Build and restart**

```bash
make build
sudo systemctl restart soul
```

**Step 4: Puppeteer E2E smoke test on titan-pc**

Test cases:
1. Open Soul in browser, verify page loads (no CSP errors in console)
2. Send a message, verify response streams correctly
3. Send a message with a code block (` ```go ... ``` `), verify syntax highlighting
4. Send a message that would generate a mermaid block, verify it renders as code (no crash)
5. Open session list, search sessions, verify no lag
6. Click copy on a code block, verify copy works, navigate away quickly
7. Refresh browser mid-conversation, verify session restores
8. Check console for errors (zero expected)

**Step 5: Commit verification result**

```bash
git add -A
git commit -m "chore: pillar fixes complete — all 5 pillars verified"
```

**Step 6: Push**

```bash
git push origin master
```
