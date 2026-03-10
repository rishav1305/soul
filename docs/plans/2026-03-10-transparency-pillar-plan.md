# Transparency Pillar — Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add deep observability to every layer of soul-v2: database timing, HTTP request logging, threshold alerting, frontend error capture, and performance tracking.

**Architecture:** Wrapper-based instrumentation. `TimedStore` decorates `session.Store` with timing. HTTP middleware logs requests. `AlertChecker` hooks into the event logger for threshold breaches. Frontend sends errors/perf data to `/api/telemetry`. Four new CLI report commands.

**Tech Stack:** Go 1.24, SQLite, React 19, TypeScript 5.9, existing JSONL metrics system

---

### Task 1: Add new event type constants

**Files:**
- Modify: `internal/metrics/types.go:10-54`

**Step 1: Add event constants**

Add after line 53 (after `EventCostStep`):

```go
	// Database events
	EventDBQuery = "db.query"
	EventDBSlow  = "db.slow"

	// HTTP request events (extended from existing api.request)
	EventAPISlow = "api.slow"

	// Alert events
	EventAlertThreshold = "alert.threshold"

	// Frontend events
	EventFrontendError  = "frontend.error"
	EventFrontendRender = "frontend.render"
	EventFrontendWS     = "frontend.ws"
```

**Step 2: Verify**

Run: `cd /home/rishav/soul-v2 && go vet ./internal/metrics/...`
Expected: Clean

**Step 3: Commit**

```bash
git add internal/metrics/types.go
git commit -m "feat: add transparency pillar event type constants"
```

---

### Task 2: Create TimedStore — database instrumentation

**Files:**
- Create: `internal/session/timed_store.go`
- Create: `internal/session/timed_store_test.go`

**Step 1: Write the test**

Create `internal/session/timed_store_test.go`:

```go
package session

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	"github.com/rishav1305/soul-v2/internal/metrics"
)

func TestTimedStore_LogsQueryEvents(t *testing.T) {
	// Set up temp data dir for metrics.
	tmpDir := t.TempDir()
	logger, err := metrics.NewEventLogger(tmpDir)
	if err != nil {
		t.Fatalf("create logger: %v", err)
	}
	defer logger.Close()

	// Open a real store.
	dbPath := filepath.Join(tmpDir, "test.db")
	store, err := Open(dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer store.Close()

	ts := NewTimedStore(store, logger, 100)

	// CreateSession should log a db.query event.
	sess, err := ts.CreateSession("test")
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	if sess.Title != "test" {
		t.Errorf("expected title 'test', got %q", sess.Title)
	}

	// Read events and verify db.query was logged.
	events, err := metrics.ReadEvents(tmpDir)
	if err != nil {
		t.Fatalf("read events: %v", err)
	}

	found := false
	for _, ev := range events {
		if ev.EventType == "db.query" {
			method, _ := ev.Data["method"].(string)
			if method == "CreateSession" {
				found = true
				break
			}
		}
	}
	if !found {
		t.Error("expected db.query event for CreateSession, not found")
	}
}

func TestTimedStore_LogsSlowQueries(t *testing.T) {
	tmpDir := t.TempDir()
	logger, err := metrics.NewEventLogger(tmpDir)
	if err != nil {
		t.Fatalf("create logger: %v", err)
	}
	defer logger.Close()

	dbPath := filepath.Join(tmpDir, "test.db")
	store, err := Open(dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer store.Close()

	// Set threshold to 0ms so every query triggers a slow alert.
	ts := NewTimedStore(store, logger, 0)

	_, err = ts.CreateSession("slow-test")
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	events, err := metrics.ReadEvents(tmpDir)
	if err != nil {
		t.Fatalf("read events: %v", err)
	}

	foundSlow := false
	for _, ev := range events {
		if ev.EventType == "db.slow" {
			foundSlow = true
			break
		}
	}
	if !foundSlow {
		t.Error("expected db.slow event with 0ms threshold, not found")
	}
}

func TestTimedStore_DelegatesToInner(t *testing.T) {
	tmpDir := t.TempDir()
	logger, err := metrics.NewEventLogger(tmpDir)
	if err != nil {
		t.Fatalf("create logger: %v", err)
	}
	defer logger.Close()

	dbPath := filepath.Join(tmpDir, "test.db")
	store, err := Open(dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer store.Close()

	ts := NewTimedStore(store, logger, 100)

	// Full CRUD cycle.
	sess, err := ts.CreateSession("delegate-test")
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	got, err := ts.GetSession(sess.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Title != "delegate-test" {
		t.Errorf("expected 'delegate-test', got %q", got.Title)
	}

	sessions, err := ts.ListSessions()
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(sessions) == 0 {
		t.Error("expected at least one session")
	}

	msg, err := ts.AddMessage(sess.ID, "user", "hello")
	if err != nil {
		t.Fatalf("add message: %v", err)
	}
	if msg.Content != "hello" {
		t.Errorf("expected content 'hello', got %q", msg.Content)
	}

	msgs, err := ts.GetMessages(sess.ID)
	if err != nil {
		t.Fatalf("get messages: %v", err)
	}
	if len(msgs) != 1 {
		t.Errorf("expected 1 message, got %d", len(msgs))
	}

	err = ts.DeleteSession(sess.ID)
	if err != nil {
		t.Fatalf("delete: %v", err)
	}
}

func TestTimedStore_RunInTransaction(t *testing.T) {
	tmpDir := t.TempDir()
	logger, err := metrics.NewEventLogger(tmpDir)
	if err != nil {
		t.Fatalf("create logger: %v", err)
	}
	defer logger.Close()

	dbPath := filepath.Join(tmpDir, "test.db")
	store, err := Open(dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer store.Close()

	ts := NewTimedStore(store, logger, 100)

	sess, err := ts.CreateSession("tx-test")
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	err = ts.RunInTransaction(func(tx *sql.Tx) error {
		_, err := ts.AddMessageTx(tx, sess.ID, "user", "in-tx")
		return err
	})
	if err != nil {
		t.Fatalf("transaction: %v", err)
	}

	events, err := metrics.ReadEvents(tmpDir)
	if err != nil {
		t.Fatalf("read events: %v", err)
	}

	foundTx := false
	for _, ev := range events {
		if ev.EventType == "db.query" {
			method, _ := ev.Data["method"].(string)
			if method == "RunInTransaction" {
				foundTx = true
			}
		}
	}
	if !foundTx {
		t.Error("expected db.query event for RunInTransaction")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd /home/rishav/soul-v2 && go test ./internal/session/ -run TestTimedStore -v`
Expected: FAIL — `NewTimedStore` undefined

**Step 3: Implement TimedStore**

Create `internal/session/timed_store.go`:

```go
package session

import (
	"database/sql"
	"log"
	"time"

	"github.com/rishav1305/soul-v2/internal/metrics"
)

// TimedStore wraps a Store with timing instrumentation. Every public method
// logs a db.query event with its duration. Queries exceeding slowMs also
// log a db.slow event.
type TimedStore struct {
	inner  *Store
	logger *metrics.EventLogger
	slowMs int64
}

// NewTimedStore creates a TimedStore wrapping inner. slowMs is the threshold
// in milliseconds above which a db.slow event is also logged.
func NewTimedStore(inner *Store, logger *metrics.EventLogger, slowMs int64) *TimedStore {
	return &TimedStore{inner: inner, logger: logger, slowMs: slowMs}
}

// Inner returns the underlying Store (for callers that need direct access).
func (ts *TimedStore) Inner() *Store { return ts.inner }

func (ts *TimedStore) logQuery(method string, start time.Time, sessionID string) {
	dur := time.Since(start).Milliseconds()
	data := map[string]interface{}{
		"method":      method,
		"duration_ms": dur,
	}
	if sessionID != "" {
		data["session_id"] = sessionID
	}
	if err := ts.logger.Log(metrics.EventDBQuery, data); err != nil {
		log.Printf("metrics: log db.query: %v", err)
	}
	if dur > ts.slowMs {
		data["threshold_ms"] = ts.slowMs
		if err := ts.logger.Log(metrics.EventDBSlow, data); err != nil {
			log.Printf("metrics: log db.slow: %v", err)
		}
	}
}

func (ts *TimedStore) CreateSession(title string) (*Session, error) {
	start := time.Now()
	result, err := ts.inner.CreateSession(title)
	sid := ""
	if result != nil {
		sid = result.ID
	}
	ts.logQuery("CreateSession", start, sid)
	return result, err
}

func (ts *TimedStore) GetSession(id string) (*Session, error) {
	start := time.Now()
	result, err := ts.inner.GetSession(id)
	ts.logQuery("GetSession", start, id)
	return result, err
}

func (ts *TimedStore) ListSessions() ([]*Session, error) {
	start := time.Now()
	result, err := ts.inner.ListSessions()
	ts.logQuery("ListSessions", start, "")
	return result, err
}

func (ts *TimedStore) UpdateSessionTitle(id, title string) (*Session, error) {
	start := time.Now()
	result, err := ts.inner.UpdateSessionTitle(id, title)
	ts.logQuery("UpdateSessionTitle", start, id)
	return result, err
}

func (ts *TimedStore) UpdateSessionStatus(id string, status Status) error {
	start := time.Now()
	err := ts.inner.UpdateSessionStatus(id, status)
	ts.logQuery("UpdateSessionStatus", start, id)
	return err
}

func (ts *TimedStore) DeleteSession(id string) error {
	start := time.Now()
	err := ts.inner.DeleteSession(id)
	ts.logQuery("DeleteSession", start, id)
	return err
}

func (ts *TimedStore) AddMessage(sessionID, role, content string) (*Message, error) {
	start := time.Now()
	result, err := ts.inner.AddMessage(sessionID, role, content)
	ts.logQuery("AddMessage", start, sessionID)
	return result, err
}

func (ts *TimedStore) AddMessageTx(tx *sql.Tx, sessionID, role, content string) (*Message, error) {
	start := time.Now()
	result, err := ts.inner.AddMessageTx(tx, sessionID, role, content)
	ts.logQuery("AddMessageTx", start, sessionID)
	return result, err
}

func (ts *TimedStore) GetMessages(sessionID string) ([]*Message, error) {
	start := time.Now()
	result, err := ts.inner.GetMessages(sessionID)
	ts.logQuery("GetMessages", start, sessionID)
	return result, err
}

func (ts *TimedStore) RunInTransaction(fn func(tx *sql.Tx) error) error {
	start := time.Now()
	err := ts.inner.RunInTransaction(fn)
	ts.logQuery("RunInTransaction", start, "")
	return err
}

func (ts *TimedStore) Close() error {
	return ts.inner.Close()
}

func (ts *TimedStore) Migrate() error {
	return ts.inner.Migrate()
}
```

**Step 4: Run tests to verify they pass**

Run: `cd /home/rishav/soul-v2 && go test ./internal/session/ -run TestTimedStore -v`
Expected: All 4 tests PASS

**Step 5: Commit**

```bash
git add internal/session/timed_store.go internal/session/timed_store_test.go
git commit -m "feat: add TimedStore for database query instrumentation"
```

---

### Task 3: Wire TimedStore into main.go

**Files:**
- Modify: `cmd/soul/main.go:71-77`
- Modify: `internal/ws/handler.go` (if handler references `*session.Store` directly)

This task requires understanding how `store` is passed around. Currently `main.go:73` creates `store` as `*session.Store` and passes it to:
- `ws.WithSessionStore(store)` — hub uses it for ListSessions
- `ws.NewMessageHandler(hub, store, ...)` — handler uses it for AddMessage, GetMessages, etc.

The `TimedStore` is not a `*session.Store`, so we need an interface. Two options:
1. Define a `SessionStore` interface that both `Store` and `TimedStore` implement
2. Pass `TimedStore.Inner()` where `*Store` is needed, and log from `TimedStore` wrapper

**Approach: Define a minimal interface in the ws package** where it's consumed.

**Step 1: Check what methods hub and handler use**

Hub uses: `ListSessions()` (in `HandleUpgrade`)
Handler uses: `CreateSession`, `GetSession`, `AddMessage`, `AddMessageTx`, `GetMessages`, `UpdateSessionStatus`, `UpdateSessionTitle`, `DeleteSession`, `RunInTransaction`, `ListSessions`

**Step 2: Create interface in `internal/session/iface.go`**

```go
package session

import "database/sql"

// StoreInterface defines the session store operations used by the application.
// Both Store and TimedStore implement this interface.
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
	Close() error
}
```

**Step 3: Update ws package to use interface**

Change `hub.go` field `sessionStore *session.Store` → `sessionStore session.StoreInterface`
Change `WithSessionStore` parameter type.
Change `handler.go` field `store *session.Store` → `store session.StoreInterface`
Change `NewMessageHandler` parameter type.

**Step 4: Update server package similarly**

Change `server.go` field `sessionStore *session.Store` → `sessionStore session.StoreInterface`
Change `WithSessionStore` parameter type.

**Step 5: Wire TimedStore in main.go**

Replace lines 71-77:
```go
// Open session store.
dbPath := filepath.Join(dataDir, "sessions.db")
rawStore, err := session.Open(dbPath)
if err != nil {
    log.Fatalf("open session store: %v", err)
}
defer rawStore.Close()

// Wrap with timing instrumentation (100ms slow query threshold).
store := session.NewTimedStore(rawStore, logger, 100)
```

**Step 6: Verify**

Run: `cd /home/rishav/soul-v2 && go build ./... && go test ./...`
Expected: Build clean, all existing tests pass

**Step 7: Commit**

```bash
git add internal/session/iface.go internal/ws/hub.go internal/ws/handler.go internal/server/server.go cmd/soul/main.go
git commit -m "feat: wire TimedStore via SessionStore interface"
```

---

### Task 4: HTTP request logging middleware

**Files:**
- Modify: `internal/server/server.go:136-143`

**Step 1: Add statusRecorder and requestLogger**

Add to `internal/server/server.go` (before the middleware chain):

```go
// statusRecorder wraps http.ResponseWriter to capture the response status code.
type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (sr *statusRecorder) WriteHeader(code int) {
	sr.status = code
	sr.ResponseWriter.WriteHeader(code)
}

// requestLoggerMiddleware logs api.request events with method, path, status, and duration.
func requestLoggerMiddleware(logger *metrics.EventLogger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Skip health checks to avoid noise.
			if r.URL.Path == "/api/health" {
				next.ServeHTTP(w, r)
				return
			}

			start := time.Now()
			sr := &statusRecorder{ResponseWriter: w, status: 200}
			next.ServeHTTP(sr, r)
			duration := time.Since(start).Milliseconds()

			data := map[string]interface{}{
				"method":      r.Method,
				"path":        r.URL.Path,
				"status":      sr.status,
				"duration_ms": duration,
			}
			_ = logger.Log(metrics.EventAPIRequest, data)

			if duration > 500 {
				_ = logger.Log(metrics.EventAPISlow, data)
			}
		})
	}
}
```

**Step 2: Insert middleware into chain**

In `New()`, after line 138 (`handler := http.Handler(s.mux)`), add request logger as first middleware (innermost, closest to handlers):

```go
handler := http.Handler(s.mux)
if s.metrics != nil {
    handler = requestLoggerMiddleware(s.metrics)(handler)
}
handler = rateLimitMiddleware(60)(handler)
```

**Step 3: Verify**

Run: `cd /home/rishav/soul-v2 && go build ./... && go vet ./...`
Expected: Clean

**Step 4: Commit**

```bash
git add internal/server/server.go
git commit -m "feat: add HTTP request logging middleware"
```

---

### Task 5: Alert threshold checker

**Files:**
- Create: `internal/metrics/alerts.go`
- Create: `internal/metrics/alerts_test.go`

**Step 1: Write the test**

Create `internal/metrics/alerts_test.go`:

```go
package metrics

import (
	"path/filepath"
	"testing"
)

func TestAlertChecker_TriggersOnBreach(t *testing.T) {
	tmpDir := t.TempDir()
	logger, err := NewEventLogger(tmpDir)
	if err != nil {
		t.Fatalf("create logger: %v", err)
	}
	defer logger.Close()

	checker := NewAlertChecker(logger)
	checker.AddThreshold(Threshold{
		Metric:   EventDBQuery,
		Field:    "duration_ms",
		MaxValue: 50,
		Severity: "warning",
	})

	// Simulate a slow query event.
	checker.Check(EventDBQuery, map[string]interface{}{
		"method":      "GetSession",
		"duration_ms": float64(120),
	})

	events, err := ReadEvents(tmpDir)
	if err != nil {
		t.Fatalf("read events: %v", err)
	}

	found := false
	for _, ev := range events {
		if ev.EventType == EventAlertThreshold {
			sev, _ := ev.Data["severity"].(string)
			if sev == "warning" {
				found = true
			}
		}
	}
	if !found {
		t.Error("expected alert.threshold event with severity=warning")
	}
}

func TestAlertChecker_NoAlertBelowThreshold(t *testing.T) {
	tmpDir := t.TempDir()
	logger, err := NewEventLogger(tmpDir)
	if err != nil {
		t.Fatalf("create logger: %v", err)
	}
	defer logger.Close()

	checker := NewAlertChecker(logger)
	checker.AddThreshold(Threshold{
		Metric:   EventDBQuery,
		Field:    "duration_ms",
		MaxValue: 100,
		Severity: "warning",
	})

	checker.Check(EventDBQuery, map[string]interface{}{
		"method":      "GetSession",
		"duration_ms": float64(5),
	})

	events, err := ReadEvents(tmpDir)
	if err != nil {
		t.Fatalf("read events: %v", err)
	}

	for _, ev := range events {
		if ev.EventType == EventAlertThreshold {
			t.Error("should not have logged alert.threshold for value below threshold")
		}
	}
}

func TestDefaultThresholds(t *testing.T) {
	tmpDir := t.TempDir()
	logger, err := NewEventLogger(tmpDir)
	if err != nil {
		t.Fatalf("create logger: %v", err)
	}
	defer logger.Close()

	checker := NewAlertCheckerWithDefaults(logger)

	// Verify the default thresholds exist.
	if len(checker.thresholds) == 0 {
		t.Error("expected default thresholds to be populated")
	}

	_ = filepath.Join(tmpDir) // satisfy import
}
```

**Step 2: Run test to verify it fails**

Run: `cd /home/rishav/soul-v2 && go test ./internal/metrics/ -run TestAlert -v`
Expected: FAIL — `NewAlertChecker` undefined

**Step 3: Implement AlertChecker**

Create `internal/metrics/alerts.go`:

```go
package metrics

import "log"

// Threshold defines a metric threshold that triggers an alert when breached.
type Threshold struct {
	Metric   string  // event type to watch (e.g., "db.query")
	Field    string  // data field to check (e.g., "duration_ms")
	MaxValue float64 // threshold value
	Severity string  // "warning" or "critical"
}

// AlertChecker checks events against configured thresholds and logs
// alert.threshold events when values exceed limits.
type AlertChecker struct {
	logger     *EventLogger
	thresholds []Threshold
}

// NewAlertChecker creates an AlertChecker with no thresholds configured.
func NewAlertChecker(logger *EventLogger) *AlertChecker {
	return &AlertChecker{logger: logger}
}

// NewAlertCheckerWithDefaults creates an AlertChecker with the standard
// threshold set for soul-v2.
func NewAlertCheckerWithDefaults(logger *EventLogger) *AlertChecker {
	ac := NewAlertChecker(logger)
	ac.thresholds = []Threshold{
		{Metric: EventDBQuery, Field: "duration_ms", MaxValue: 100, Severity: "warning"},
		{Metric: EventDBQuery, Field: "duration_ms", MaxValue: 500, Severity: "critical"},
		{Metric: EventAPIRequest, Field: "duration_ms", MaxValue: 500, Severity: "warning"},
		{Metric: EventAPIRequest, Field: "duration_ms", MaxValue: 2000, Severity: "critical"},
		{Metric: EventWSStreamEnd, Field: "duration_ms", MaxValue: 300000, Severity: "critical"},
		{Metric: EventSystemSample, Field: "heap_mb", MaxValue: 256, Severity: "warning"},
		{Metric: EventSystemSample, Field: "goroutines", MaxValue: 100, Severity: "warning"},
	}
	return ac
}

// AddThreshold adds a threshold to the checker.
func (ac *AlertChecker) AddThreshold(t Threshold) {
	ac.thresholds = append(ac.thresholds, t)
}

// Check evaluates the given event against all thresholds and logs an
// alert.threshold event for each breach. Called after every event is written.
func (ac *AlertChecker) Check(eventType string, data map[string]interface{}) {
	for _, t := range ac.thresholds {
		if t.Metric != eventType {
			continue
		}
		value := getFloatField(data, t.Field)
		if value <= t.MaxValue {
			continue
		}
		alertData := map[string]interface{}{
			"metric":    t.Metric,
			"field":     t.Field,
			"value":     value,
			"threshold": t.MaxValue,
			"severity":  t.Severity,
		}
		// Copy relevant context from original event.
		for _, key := range []string{"method", "path", "session_id"} {
			if v, ok := data[key]; ok {
				alertData[key] = v
			}
		}
		if err := ac.logger.Log(EventAlertThreshold, alertData); err != nil {
			log.Printf("metrics: log alert: %v", err)
		}
	}
}
```

**Step 4: Run tests**

Run: `cd /home/rishav/soul-v2 && go test ./internal/metrics/ -run TestAlert -v`
Expected: All 3 PASS

**Step 5: Commit**

```bash
git add internal/metrics/alerts.go internal/metrics/alerts_test.go
git commit -m "feat: add AlertChecker with configurable thresholds"
```

---

### Task 6: Hook AlertChecker into EventLogger

**Files:**
- Modify: `internal/metrics/logger.go:18-29,56-92`

**Step 1: Add alertChecker field to EventLogger**

Add field to struct (after line 28):
```go
alertChecker *AlertChecker
```

**Step 2: Add setter method**

```go
// SetAlertChecker sets the alert checker that runs after each event is logged.
func (l *EventLogger) SetAlertChecker(ac *AlertChecker) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.alertChecker = ac
}
```

**Step 3: Call Check after writing event**

In `Log()`, after the `file.Sync()` call (after line 89), before `return nil`:

```go
	// Run alert checks (outside the lock would cause races on the file,
	// so we keep it inside — alertChecker.Check may call Log recursively,
	// but that's OK since it will acquire a separate lock call).
	// Actually, we need to avoid recursive locking. Release lock first.
	checker := l.alertChecker
	l.mu.Unlock()
	if checker != nil {
		checker.Check(eventType, data)
	}
	l.mu.Lock() // Re-acquire for deferred unlock
```

Wait — this creates a lock ordering issue. Better approach: **check alerts after releasing the lock**. Restructure `Log()`:

```go
func (l *EventLogger) Log(eventType string, data map[string]interface{}) error {
	now := l.nowFunc()
	ev := Event{
		Timestamp: now,
		EventType: eventType,
		Data:      data,
	}

	if err := ev.Validate(); err != nil {
		return fmt.Errorf("invalid event: %w", err)
	}

	buf, err := ev.MarshalJSON()
	if err != nil {
		return fmt.Errorf("marshal event: %w", err)
	}
	buf = append(buf, '\n')

	var checker *AlertChecker

	l.mu.Lock()
	if err := l.checkRotate(now); err != nil {
		fmt.Fprintf(os.Stderr, "metrics: rotation failed: %v\n", err)
	}
	if _, err := l.file.Write(buf); err != nil {
		l.mu.Unlock()
		return fmt.Errorf("write event: %w", err)
	}
	if err := l.file.Sync(); err != nil {
		l.mu.Unlock()
		return fmt.Errorf("flush event: %w", err)
	}
	checker = l.alertChecker
	l.mu.Unlock()

	// Run alert checks outside the lock to avoid recursive locking.
	// Skip checking alert events themselves to prevent infinite recursion.
	if checker != nil && eventType != EventAlertThreshold {
		checker.Check(eventType, data)
	}

	return nil
}
```

**Step 4: Wire in main.go**

In `cmd/soul/main.go`, after creating the logger (line 62), before the sampler:

```go
alertChecker := metrics.NewAlertCheckerWithDefaults(logger)
logger.SetAlertChecker(alertChecker)
```

**Step 5: Verify**

Run: `cd /home/rishav/soul-v2 && go build ./... && go test ./...`
Expected: Build clean, tests pass

**Step 6: Commit**

```bash
git add internal/metrics/logger.go cmd/soul/main.go
git commit -m "feat: hook AlertChecker into EventLogger after each event"
```

---

### Task 7: Telemetry endpoint for frontend events

**Files:**
- Modify: `internal/server/server.go`

**Step 1: Add the endpoint handler**

```go
// handleTelemetry receives frontend telemetry events and logs them to metrics.
func (s *Server) handleTelemetry(w http.ResponseWriter, r *http.Request) {
	if s.metrics == nil {
		http.Error(w, "metrics not configured", http.StatusServiceUnavailable)
		return
	}

	var payload struct {
		Type string                 `json:"type"`
		Data map[string]interface{} `json:"data"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}

	// Only accept known frontend event types.
	switch payload.Type {
	case metrics.EventFrontendError, metrics.EventFrontendRender, metrics.EventFrontendWS:
		// OK
	default:
		http.Error(w, "unknown event type", http.StatusBadRequest)
		return
	}

	_ = s.metrics.Log(payload.Type, payload.Data)
	w.WriteHeader(http.StatusNoContent)
}
```

**Step 2: Register the route**

Add after the session routes (after line 126):

```go
	// Telemetry route for frontend observability.
	s.mux.HandleFunc("POST /api/telemetry", s.handleTelemetry)
```

**Step 3: Verify**

Run: `cd /home/rishav/soul-v2 && go build ./... && go vet ./...`
Expected: Clean

**Step 4: Commit**

```bash
git add internal/server/server.go
git commit -m "feat: add /api/telemetry endpoint for frontend events"
```

---

### Task 8: Frontend telemetry client

**Files:**
- Create: `web/src/lib/telemetry.ts`

**Step 1: Create telemetry module**

```typescript
// telemetry.ts — fire-and-forget frontend event reporting.

type TelemetryEvent = 'frontend.error' | 'frontend.render' | 'frontend.ws';

function sendTelemetry(type: TelemetryEvent, data: Record<string, unknown>): void {
  try {
    fetch('/api/telemetry', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ type, data }),
    }).catch(() => {
      // Telemetry failure is non-critical — silently ignore.
    });
  } catch {
    // Ignore — telemetry must never throw.
  }
}

export function reportError(component: string, error: unknown): void {
  const message = error instanceof Error ? error.message : String(error);
  const stack = error instanceof Error ? error.stack : undefined;
  sendTelemetry('frontend.error', { component, error: message, stack });
}

export function reportRender(component: string, durationMs: number): void {
  sendTelemetry('frontend.render', { component, duration_ms: durationMs });
}

export function reportWSLatency(firstTokenMs: number, totalMs: number): void {
  sendTelemetry('frontend.ws', { event: 'round_trip', first_token_ms: firstTokenMs, total_ms: totalMs });
}
```

**Step 2: Verify**

Run: `cd /home/rishav/soul-v2/web && npx tsc --noEmit`
Expected: Clean

**Step 3: Commit**

```bash
git add web/src/lib/telemetry.ts
git commit -m "feat: add frontend telemetry client"
```

---

### Task 9: React ErrorBoundary

**Files:**
- Create: `web/src/components/ErrorBoundary.tsx`
- Modify: `web/src/main.tsx`

**Step 1: Create ErrorBoundary component**

```typescript
import { Component } from 'react';
import type { ReactNode, ErrorInfo } from 'react';
import { reportError } from '../lib/telemetry';

interface Props {
  children: ReactNode;
}

interface State {
  hasError: boolean;
  error: Error | null;
}

export class ErrorBoundary extends Component<Props, State> {
  constructor(props: Props) {
    super(props);
    this.state = { hasError: false, error: null };
  }

  static getDerivedStateFromError(error: Error): State {
    return { hasError: true, error };
  }

  componentDidCatch(error: Error, info: ErrorInfo) {
    reportError('ErrorBoundary', error);
    console.error('ErrorBoundary caught:', error, info.componentStack);
  }

  handleRetry = () => {
    this.setState({ hasError: false, error: null });
  };

  render() {
    if (this.state.hasError) {
      return (
        <div data-testid="error-boundary" className="flex items-center justify-center h-screen bg-deep text-fg">
          <div className="text-center space-y-4">
            <h1 className="text-xl font-semibold">Something went wrong</h1>
            <p className="text-fg-secondary text-sm max-w-md">
              {this.state.error?.message || 'An unexpected error occurred'}
            </p>
            <button
              data-testid="error-retry"
              onClick={this.handleRetry}
              className="px-4 py-2 bg-soul text-deep rounded-lg hover:bg-soul/85 transition-colors cursor-pointer"
            >
              Try Again
            </button>
          </div>
        </div>
      );
    }
    return this.props.children;
  }
}
```

**Step 2: Wrap app root in main.tsx**

Change `web/src/main.tsx`:

```typescript
import { StrictMode } from 'react';
import { createRoot } from 'react-dom/client';
import { ErrorBoundary } from './components/ErrorBoundary';
import { Shell } from './components/Shell';
import './app.css';

const root = document.getElementById('root');
if (root) {
  createRoot(root).render(
    <StrictMode>
      <ErrorBoundary>
        <Shell />
      </ErrorBoundary>
    </StrictMode>,
  );
}

if ('serviceWorker' in navigator) {
  navigator.serviceWorker.register('/sw.js').catch(() => {
    // Service worker registration failed — app works without it
  });
}
```

**Step 3: Verify**

Run: `cd /home/rishav/soul-v2/web && npx tsc --noEmit`
Expected: Clean

**Step 4: Commit**

```bash
git add web/src/components/ErrorBoundary.tsx web/src/main.tsx
git commit -m "feat: add React ErrorBoundary with telemetry reporting"
```

---

### Task 10: Replace silent catches with error reporting

**Files:**
- Modify: `web/src/hooks/useChat.ts:72,387`
- Modify: `web/src/hooks/useWebSocket.ts:86-88`

**Step 1: Add import to useChat.ts**

Add at top of `useChat.ts`:
```typescript
import { reportError } from '../lib/telemetry';
```

**Step 2: Replace silent catches in useChat.ts**

Line 72 — `connection.ready` localStorage parse:
```typescript
// Before:
} catch { /* corrupted data — ignore */ }
// After:
} catch (err) { reportError('useChat.pendingRestore', err); }
```

Line 387 — `sendMessage` localStorage write:
```typescript
// Before:
} catch { /* quota exceeded — proceed without persistence */ }
// After:
} catch (err) { reportError('useChat.pendingSave', err); }
```

**Step 3: Add import to useWebSocket.ts**

Add at top:
```typescript
import { reportError } from '../lib/telemetry';
```

**Step 4: Replace silent catch in useWebSocket.ts**

Lines 86-88 — malformed message:
```typescript
// Before:
} catch {
    // Ignore malformed messages.
}
// After:
} catch (err) {
    reportError('useWebSocket.parse', err);
}
```

**Step 5: Verify**

Run: `cd /home/rishav/soul-v2/web && npx tsc --noEmit`
Expected: Clean

**Step 6: Commit**

```bash
git add web/src/hooks/useChat.ts web/src/hooks/useWebSocket.ts
git commit -m "fix: replace silent catches with telemetry error reporting"
```

---

### Task 11: WebSocket round-trip timing

**Files:**
- Modify: `web/src/hooks/useChat.ts`

**Step 1: Add timing ref and import**

At the top of `useChat.ts`, the `reportError` import is already added. Also import `reportWSLatency`:

```typescript
import { reportError, reportWSLatency } from '../lib/telemetry';
```

Inside `useChat()`, add a ref:

```typescript
const sendTimeRef = useRef<number>(0);
const firstTokenTimeRef = useRef<number>(0);
```

**Step 2: Record send time**

In `sendMessage`, right before `send('chat.send', payload)` (around line 408):

```typescript
sendTimeRef.current = performance.now();
firstTokenTimeRef.current = 0;
```

**Step 3: Record first token time**

In the `chat.token` handler (around line 204), at the start of the case:

```typescript
case 'chat.token': {
    if (sendTimeRef.current > 0 && firstTokenTimeRef.current === 0) {
        firstTokenTimeRef.current = performance.now();
    }
```

**Step 4: Report on chat.done**

In the `chat.done` handler (around line 228), after `setIsStreaming(false)`:

```typescript
if (sendTimeRef.current > 0) {
    const now = performance.now();
    const firstTokenMs = firstTokenTimeRef.current > 0
        ? firstTokenTimeRef.current - sendTimeRef.current
        : 0;
    const totalMs = now - sendTimeRef.current;
    reportWSLatency(Math.round(firstTokenMs), Math.round(totalMs));
    sendTimeRef.current = 0;
    firstTokenTimeRef.current = 0;
}
```

**Step 5: Verify**

Run: `cd /home/rishav/soul-v2/web && npx tsc --noEmit`
Expected: Clean

**Step 6: Commit**

```bash
git add web/src/hooks/useChat.ts
git commit -m "feat: add WebSocket round-trip latency reporting"
```

---

### Task 12: usePerformance hook

**Files:**
- Create: `web/src/hooks/usePerformance.ts`
- Modify: `web/src/components/MessageBubble.tsx`
- Modify: `web/src/components/SessionList.tsx`

**Step 1: Create the hook**

```typescript
import { useEffect, useRef } from 'react';
import { reportRender } from '../lib/telemetry';

// usePerformance measures render performance for a component.
// Reports renders that take longer than thresholdMs (default 50ms).
export function usePerformance(componentName: string, thresholdMs = 50): void {
  const renderStart = useRef(performance.now());

  // Record start time on each render.
  renderStart.current = performance.now();

  useEffect(() => {
    const duration = performance.now() - renderStart.current;
    if (duration > thresholdMs) {
      reportRender(componentName, Math.round(duration));
    }
  });
}
```

**Step 2: Add to MessageBubble**

In `web/src/components/MessageBubble.tsx`, add import and call at top of component:

```typescript
import { usePerformance } from '../hooks/usePerformance';
// Inside the component function:
usePerformance('MessageBubble');
```

**Step 3: Add to SessionList**

In `web/src/components/SessionList.tsx`, add import and call inside the `SessionItem` memo component:

```typescript
import { usePerformance } from '../hooks/usePerformance';
// Inside SessionItem:
usePerformance('SessionItem');
```

**Step 4: Verify**

Run: `cd /home/rishav/soul-v2/web && npx tsc --noEmit`
Expected: Clean

**Step 5: Commit**

```bash
git add web/src/hooks/usePerformance.ts web/src/components/MessageBubble.tsx web/src/components/SessionList.tsx
git commit -m "feat: add usePerformance hook for render timing"
```

---

### Task 13: CLI report commands — alerts, db, requests, frontend

**Files:**
- Modify: `internal/metrics/aggregator.go`
- Modify: `cmd/soul/metrics.go`

**Step 1: Add report types to aggregator.go**

```go
// AlertEntry represents a single threshold breach.
type AlertEntry struct {
	Timestamp time.Time
	Metric    string
	Field     string
	Value     float64
	Threshold float64
	Severity  string
}

// AlertsReport provides recent threshold breaches.
type AlertsReport struct {
	Alerts []AlertEntry
}

// DBReport provides database query timing percentiles per method.
type DBReport struct {
	Methods     map[string]*MethodStats
	SlowQueries []SlowQuery
}

// MethodStats holds timing percentiles for a single DB method.
type MethodStats struct {
	Method string
	Count  int
	P50    time.Duration
	P95    time.Duration
	P99    time.Duration
}

// SlowQuery represents a single slow query event.
type SlowQuery struct {
	Timestamp time.Time
	Method    string
	DurationMs float64
	SessionID  string
}

// RequestsReport provides HTTP request timing per path.
type RequestsReport struct {
	Paths       map[string]*PathStats
	StatusCodes map[int]int
}

// PathStats holds timing percentiles for a single HTTP path.
type PathStats struct {
	Path  string
	Count int
	P50   time.Duration
	P95   time.Duration
	P99   time.Duration
}

// FrontendReport provides frontend errors and performance data.
type FrontendReport struct {
	Errors       int
	TopErrors    map[string]int // component → count
	SlowRenders  []RenderEntry
	WSLatency    *LatencyReport
}

// RenderEntry represents a slow render event.
type RenderEntry struct {
	Component  string
	DurationMs float64
}
```

**Step 2: Add Alerts() method**

```go
func (a *Aggregator) Alerts() (*AlertsReport, error) {
	events, err := ReadEventsFiltered(a.dataDir, "alert")
	if err != nil {
		return nil, fmt.Errorf("read events: %w", err)
	}
	report := &AlertsReport{}
	for _, ev := range events {
		if ev.EventType != EventAlertThreshold {
			continue
		}
		report.Alerts = append(report.Alerts, AlertEntry{
			Timestamp: ev.Timestamp,
			Metric:    getStringField(ev.Data, "metric"),
			Field:     getStringField(ev.Data, "field"),
			Value:     getFloatField(ev.Data, "value"),
			Threshold: getFloatField(ev.Data, "threshold"),
			Severity:  getStringField(ev.Data, "severity"),
		})
	}
	return report, nil
}
```

**Step 3: Add DB() method**

```go
func (a *Aggregator) DB() (*DBReport, error) {
	events, err := ReadEventsFiltered(a.dataDir, "db")
	if err != nil {
		return nil, fmt.Errorf("read events: %w", err)
	}
	report := &DBReport{Methods: make(map[string]*MethodStats)}
	methodDurations := make(map[string][]float64)

	for _, ev := range events {
		method := getStringField(ev.Data, "method")
		dur := getFloatField(ev.Data, "duration_ms")
		switch ev.EventType {
		case EventDBQuery:
			methodDurations[method] = append(methodDurations[method], dur)
		case EventDBSlow:
			report.SlowQueries = append(report.SlowQueries, SlowQuery{
				Timestamp:  ev.Timestamp,
				Method:     method,
				DurationMs: dur,
				SessionID:  getStringField(ev.Data, "session_id"),
			})
		}
	}

	for method, durations := range methodDurations {
		sort.Float64s(durations)
		report.Methods[method] = &MethodStats{
			Method: method,
			Count:  len(durations),
			P50:    msToDuration(percentile(durations, 50)),
			P95:    msToDuration(percentile(durations, 95)),
			P99:    msToDuration(percentile(durations, 99)),
		}
	}
	return report, nil
}
```

**Step 4: Add Requests() and Frontend() methods** (similar pattern — read events, aggregate, compute percentiles)

**Step 5: Add CLI commands in metrics.go**

Add cases to the switch in `runMetrics()`:
```go
case "alerts":
    runMetricsAlerts()
case "db":
    runMetricsDB()
case "requests":
    runMetricsRequests()
case "frontend":
    runMetricsFrontend()
```

Update help text to include new subcommands.

Add the corresponding `runMetrics*()` functions that create an aggregator, call the method, and format output (same pattern as existing commands).

**Step 6: Verify**

Run: `cd /home/rishav/soul-v2 && go build ./... && go vet ./...`
Expected: Clean

**Step 7: Commit**

```bash
git add internal/metrics/aggregator.go cmd/soul/metrics.go
git commit -m "feat: add alerts, db, requests, frontend CLI report commands"
```

---

### Task 14: Full verification

**Steps:**
1. `cd /home/rishav/soul-v2 && make verify-static` — go vet + tsc clean
2. `cd /home/rishav/soul-v2 && go test ./...` — all tests pass (except known pre-existing failures)
3. `cd /home/rishav/soul-v2 && go build -o /dev/null ./cmd/soul` — clean build
4. `cd /home/rishav/soul-v2/web && npx vite build` — bundle builds, still under 300KB
5. `cd /home/rishav/soul-v2/web && npx tsc --noEmit` — no type errors

No commit — verification only.
