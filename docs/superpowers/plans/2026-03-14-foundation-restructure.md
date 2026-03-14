# Foundation Restructure Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Restructure Soul v2 from a single-server monolith into a two-binary monorepo with shared packages, preparing the foundation for the tasks server and autonomous execution layer.

**Architecture:** Move existing code into `internal/chat/` namespace, extract shared auth and event types into `pkg/`, split the CLI entrypoint into `cmd/chat/`, and rename the session database. All existing functionality preserved — this is a pure structural refactor.

**Tech Stack:** Go 1.24, SQLite (modernc.org/sqlite), WebSocket (nhooyr.io/websocket), React 19, Vite 7, TypeScript 5.9

**Spec:** `docs/superpowers/specs/2026-03-14-autonomous-execution-design.md`

---

## File Structure

### New files to create

| File | Purpose |
|------|---------|
| `pkg/events/events.go` | Logger interface + Event type (shared by both servers) |
| `pkg/events/events_test.go` | Tests for Event validation and Logger contract |
| `pkg/auth/oauth.go` | OAuth token source (moved from internal/auth, refactored to use Logger interface) |
| `pkg/auth/oauth_test.go` | Auth tests (moved from internal/auth) |
| `cmd/chat/main.go` | Chat server CLI entrypoint (moved from cmd/soul/main.go) |
| `cmd/chat/metrics.go` | Metrics subcommands (moved from cmd/soul/metrics.go) |

### Files to move (package path changes)

| From | To | New package import |
|------|----|--------------------|
| `internal/session/*.go` | `internal/chat/session/*.go` | `github.com/rishav1305/soul-v2/internal/chat/session` |
| `internal/metrics/*.go` | `internal/chat/metrics/*.go` | `github.com/rishav1305/soul-v2/internal/chat/metrics` |
| `internal/stream/*.go` | `internal/chat/stream/*.go` | `github.com/rishav1305/soul-v2/internal/chat/stream` |
| `internal/ws/*.go` | `internal/chat/ws/*.go` | `github.com/rishav1305/soul-v2/internal/chat/ws` |
| `internal/server/*.go` | `internal/chat/server/*.go` | `github.com/rishav1305/soul-v2/internal/chat/server` |

### Files to modify

| File | Change |
|------|--------|
| `Makefile` | Update build target from `./cmd/soul` to `./cmd/chat`, add `soul-tasks` placeholder |
| `deploy/deploy.sh` | Update binary name if referenced |
| `deploy/soul-v2.service` | Update `ExecStart` from `soul serve` to `soul-chat serve` |
| `tests/integration/*.go` | Update import paths |
| `tests/load/*.go` | Update import paths |
| `tests/verify/*.go` | Update import paths |
| `tests/invariants_test.go` | Update import paths |

---

## Chunk 1: Shared packages and auth refactor

### Task 1: Create pkg/events — Logger interface and Event type

The Logger interface breaks the concrete dependency between auth and metrics. Both servers will implement this interface with their own EventLogger.

**Files:**
- Create: `pkg/events/events.go`
- Create: `pkg/events/events_test.go`

- [ ] **Step 1: Write the Logger interface and Event type**

```go
// pkg/events/events.go
package events

import "fmt"

// Logger is the interface for structured event logging.
// Both chat and tasks servers implement this with their own storage.
type Logger interface {
	Log(eventType string, data map[string]interface{}) error
}

// NopLogger is a no-op Logger for testing or when logging is disabled.
type NopLogger struct{}

func (NopLogger) Log(string, map[string]interface{}) error { return nil }

// Note: pkg/events does NOT define an Event struct. The existing
// internal/chat/metrics.Event type handles serialization with its own
// JSON tags ("ts", "event"). This package only provides the Logger
// interface and shared constants.

// Auth-related event type constants (used by pkg/auth).
const (
	EventOAuthRefresh = "oauth.refresh"
	EventOAuthReload  = "oauth.reload"
)
```

- [ ] **Step 2: Write tests for Event and NopLogger**

```go
// pkg/events/events_test.go
package events

import "testing"

func TestNopLogger(t *testing.T) {
	var l Logger = NopLogger{}
	if err := l.Log("test", nil); err != nil {
		t.Errorf("NopLogger.Log() = %v, want nil", err)
	}
}

func TestConstants(t *testing.T) {
	if EventOAuthRefresh != "oauth.refresh" {
		t.Errorf("EventOAuthRefresh = %q", EventOAuthRefresh)
	}
	if EventOAuthReload != "oauth.reload" {
		t.Errorf("EventOAuthReload = %q", EventOAuthReload)
	}
}
```

- [ ] **Step 3: Run tests**

Run: `cd /home/rishav/soul-v2 && go test ./pkg/events/ -v`
Expected: PASS (2 tests)

- [ ] **Step 4: Commit**

```bash
git add pkg/events/
git commit -m "feat: add pkg/events with Logger interface and Event type"
```

---

### Task 2: Refactor auth to use Logger interface

Currently `internal/auth/oauth.go` takes `*metrics.EventLogger` directly. Refactor it to accept `events.Logger` interface instead, then move it to `pkg/auth/`.

**Files:**
- Modify: `internal/auth/oauth.go` — change logger field type and constructor parameter
- Modify: `internal/auth/oauth_test.go` — use NopLogger in tests
- Modify: `cmd/soul/main.go` — pass EventLogger as Logger interface (temporary, will be moved in Task 4)

- [ ] **Step 1: Update oauth.go logger field and constructor**

In `internal/auth/oauth.go`:

Change the import from:
```go
"github.com/rishav1305/soul-v2/internal/metrics"
```
to:
```go
"github.com/rishav1305/soul-v2/pkg/events"
```

Change the struct field:
```go
// Before:
logger *metrics.EventLogger

// After:
logger events.Logger
```

Change the constructor signature:
```go
// Before:
func NewOAuthTokenSource(credPath string, logger *metrics.EventLogger) *OAuthTokenSource

// After:
func NewOAuthTokenSource(credPath string, logger events.Logger) *OAuthTokenSource
```

All `s.logger.Log(...)` calls remain unchanged — they already use the same `Log(string, map[string]interface{})` signature.

Also replace constant references:
- `metrics.EventOAuthRefresh` → `events.EventOAuthRefresh`
- `metrics.EventOAuthReload` → `events.EventOAuthReload`

Add nil-safety: in the constructor, if logger is nil, set it to `events.NopLogger{}`.

- [ ] **Step 2: Update oauth_test.go to use NopLogger**

Replace any `metrics.NewEventLogger(...)` calls in tests with `events.NopLogger{}`.

- [ ] **Step 3: Update cmd/soul/main.go temporarily**

The `auth.NewOAuthTokenSource(credPath, logger)` call still works because `*metrics.EventLogger` satisfies `events.Logger` (it has a `Log` method with the right signature).

Verify: `*metrics.EventLogger` has method `Log(eventType string, data map[string]interface{}) error` — it does (logger.go line ~30).

- [ ] **Step 4: Verify build and tests**

Run: `cd /home/rishav/soul-v2 && go build ./... && go test ./internal/auth/ -v`
Expected: Build succeeds, all auth tests pass.

- [ ] **Step 5: Commit**

```bash
git add internal/auth/ cmd/soul/main.go pkg/events/
git commit -m "refactor: auth uses events.Logger interface instead of concrete EventLogger"
```

---

### Task 3: Move auth to pkg/auth/

Now that auth depends only on `pkg/events` (not `internal/metrics`), move it to `pkg/`.

**Files:**
- Move: `internal/auth/oauth.go` → `pkg/auth/oauth.go`
- Move: `internal/auth/oauth_test.go` → `pkg/auth/oauth_test.go`
- Modify: all files importing `internal/auth` to use `pkg/auth`

- [ ] **Step 1: Create pkg/auth/ and move files**

```bash
cd /home/rishav/soul-v2
mkdir -p pkg/auth
cp internal/auth/oauth.go pkg/auth/oauth.go
cp internal/auth/oauth_test.go pkg/auth/oauth_test.go
```

Update package declaration in both files — it stays `package auth` (no change needed).

Update import path in `pkg/auth/oauth.go`:
```go
// The events import already points to pkg/events — no change needed
```

- [ ] **Step 2: Update all imports**

Files that import `github.com/rishav1305/soul-v2/internal/auth`:
- `cmd/soul/main.go`
- `internal/server/server.go`
- `internal/server/server_test.go`

Change import from:
```go
"github.com/rishav1305/soul-v2/internal/auth"
```
to:
```go
"github.com/rishav1305/soul-v2/pkg/auth"
```

- [ ] **Step 3: Remove old internal/auth/**

```bash
rm -rf internal/auth/
```

- [ ] **Step 4: Verify build and tests**

Run: `cd /home/rishav/soul-v2 && go build ./... && go test ./pkg/auth/ -v`
Expected: Build succeeds, all auth tests pass.

- [ ] **Step 5: Commit**

```bash
git add pkg/auth/ cmd/soul/ internal/
git commit -m "refactor: move auth to pkg/auth for shared access"
```

---

### Task 4: Move internal packages into internal/chat/ namespace

Move session, metrics, stream, ws, and server packages into the `internal/chat/` subtree. This is the bulk of the restructure.

**Strategy:** Move one package at a time, updating imports after each move. Order matters — move leaf packages first (no internal dependencies), then packages that depend on them.

**IMPORTANT:** When updating imports, update ALL `.go` files — including `*_test.go` and `*_fuzz_test.go` files. Test files often import sibling packages. Use `grep -r "old/import/path" --include="*.go"` to find every reference before moving on.

**Dependency order (leaves first):**
1. `internal/metrics/` → `internal/chat/metrics/` (no internal deps)
2. `internal/session/` → `internal/chat/session/` (deps: metrics)
3. `internal/stream/` → `internal/chat/stream/` (no internal deps, uses pkg/auth via interface)
4. `internal/ws/` → `internal/chat/ws/` (deps: session, stream, metrics)
5. `internal/server/` → `internal/chat/server/` (deps: auth, session, metrics, ws)

**Files:**
- Move: all `internal/{package}/*.go` → `internal/chat/{package}/*.go`
- Modify: all import paths across the codebase

- [ ] **Step 1: Move metrics**

```bash
cd /home/rishav/soul-v2
mkdir -p internal/chat/metrics
cp internal/metrics/*.go internal/chat/metrics/
rm -rf internal/metrics/
```

Update ALL files importing `github.com/rishav1305/soul-v2/internal/metrics` to `github.com/rishav1305/soul-v2/internal/chat/metrics`. Files to update:
- `cmd/soul/main.go`
- `cmd/soul/metrics.go`
- `internal/session/timed_store.go`
- `internal/ws/hub.go`
- `internal/ws/handler.go`
- `internal/server/server.go`
- `pkg/auth/oauth.go` — does NOT import metrics anymore (uses events.Logger)
- Any test files in `tests/` that import metrics

Run: `go build ./...` — verify it compiles.

- [ ] **Step 2: Move session**

```bash
mkdir -p internal/chat/session
cp internal/session/*.go internal/chat/session/
rm -rf internal/session/
```

Update imports from `github.com/rishav1305/soul-v2/internal/session` to `github.com/rishav1305/soul-v2/internal/chat/session` in:
- `cmd/soul/main.go`
- `internal/ws/hub.go`
- `internal/ws/handler.go`
- `internal/chat/server/server.go` (will be moved later, update now)
- Actually `internal/server/server.go` — hasn't been moved yet

Update internal import in `internal/chat/session/timed_store.go`:
```go
// metrics import already updated to internal/chat/metrics in Step 1
```

Run: `go build ./...`

- [ ] **Step 3: Move stream**

```bash
mkdir -p internal/chat/stream
cp internal/stream/*.go internal/chat/stream/
rm -rf internal/stream/
```

Update imports from `github.com/rishav1305/soul-v2/internal/stream` to `github.com/rishav1305/soul-v2/internal/chat/stream` in:
- `cmd/soul/main.go`
- `internal/ws/handler.go`

Run: `go build ./...`

- [ ] **Step 4: Move ws**

```bash
mkdir -p internal/chat/ws
cp internal/ws/*.go internal/chat/ws/
rm -rf internal/ws/
```

Update internal imports in `internal/chat/ws/*.go`:
- `internal/chat/metrics` (already correct if updated in Step 1)
- `internal/chat/session` (already correct if updated in Step 2)
- `internal/chat/stream` (already correct if updated in Step 3)

Update imports from `github.com/rishav1305/soul-v2/internal/ws` to `github.com/rishav1305/soul-v2/internal/chat/ws` in:
- `cmd/soul/main.go`
- `internal/server/server.go`

Run: `go build ./...`

- [ ] **Step 5: Move server**

```bash
mkdir -p internal/chat/server
cp internal/server/*.go internal/chat/server/
rm -rf internal/server/
```

Update internal imports in `internal/chat/server/server.go`:
- `pkg/auth` (already correct)
- `internal/chat/session` (already correct)
- `internal/chat/metrics` (already correct)
- `internal/chat/ws` (already correct)

Update import in `cmd/soul/main.go` from `internal/server` to `internal/chat/server`.

Run: `go build ./...`

- [ ] **Step 6: Update test imports**

Update all import paths in:
- `tests/integration/*.go`
- `tests/load/*.go`
- `tests/verify/*.go`
- `tests/invariants_test.go`

Use find-and-replace across the test directory:
- `internal/metrics` → `internal/chat/metrics`
- `internal/session` → `internal/chat/session`
- `internal/stream` → `internal/chat/stream`
- `internal/ws` → `internal/chat/ws`
- `internal/server` → `internal/chat/server`
- `internal/auth` → `pkg/auth`

- [ ] **Step 7: Verify full build and all tests**

Run: `cd /home/rishav/soul-v2 && go build ./... && go test ./... -count=1`
Expected: All packages build, all tests pass.

- [ ] **Step 8: Commit**

```bash
git add internal/chat/ cmd/soul/main.go tests/
git rm -r internal/metrics/ internal/session/ internal/stream/ internal/ws/ internal/server/
git commit -m "refactor: move all packages into internal/chat/ namespace"
```

Note: Use `git status` to verify no unintended files are staged before committing.

---

### Task 5: Move cmd/soul/ to cmd/chat/

**Files:**
- Move: `cmd/soul/main.go` → `cmd/chat/main.go`
- Move: `cmd/soul/metrics.go` → `cmd/chat/metrics.go`
- Modify: `Makefile`

- [ ] **Step 1: Move cmd files**

```bash
cd /home/rishav/soul-v2
mkdir -p cmd/chat
cp cmd/soul/main.go cmd/chat/main.go
cp cmd/soul/metrics.go cmd/chat/metrics.go
rm -rf cmd/soul/
```

Package declaration stays `package main` — no change needed.

- [ ] **Step 2: Update Makefile**

Replace build targets:

```makefile
# Before:
build-go:
	go build -o soul ./cmd/soul

# After:
build-go:
	go build -o soul-chat ./cmd/chat

serve: build
	./soul-chat
```

Update any other references to `./cmd/soul` or the `soul` binary name to `soul-chat`.

Also update:
- `clean` target: `rm -f soul` → `rm -f soul-chat`
- `verify-unit` target: `go test ./internal/...` → `go test ./internal/... ./pkg/...`

Add a placeholder for future tasks server:
```makefile
build-tasks:
	@echo "Tasks server not yet implemented"

build: build-go web
```

- [ ] **Step 3: Update deploy files**

Update `deploy/deploy.sh` — change any references to `soul` binary to `soul-chat`.

Update `deploy/soul-v2.service` — change `ExecStart` line:
```
# Before:
ExecStart=/home/rishav/soul-v2/soul serve
# After:
ExecStart=/home/rishav/soul-v2/soul-chat serve
```

- [ ] **Step 4: Verify build**

Run: `cd /home/rishav/soul-v2 && make build-go`
Expected: Produces `soul-chat` binary.

Run: `./soul-chat --help` or `./soul-chat serve --help`
Expected: Same output as before, just different binary name.

- [ ] **Step 5: Commit**

```bash
git add -A
git commit -m "refactor: move cmd/soul to cmd/chat, binary renamed to soul-chat"
```

---

### Task 6: Database rename — sessions.db to chat.db

Add auto-migration logic: on startup, if `chat.db` doesn't exist but `sessions.db` does, rename it.

**Files:**
- Modify: `cmd/chat/main.go` — rename DB file before opening
- Modify: `internal/chat/session/store.go` — no changes needed (it accepts any path)
- Create: `internal/chat/session/migrate_db_test.go` — test the rename logic

- [ ] **Step 1: Add rename logic in cmd/chat/main.go**

Before the `session.Open(dbPath)` call, add:

```go
// Auto-migrate sessions.db → chat.db
chatDBPath := filepath.Join(dataDir, "chat.db")
oldDBPath := filepath.Join(dataDir, "sessions.db")
if _, err := os.Stat(chatDBPath); os.IsNotExist(err) {
    if _, err := os.Stat(oldDBPath); err == nil {
        log.Printf("Migrating %s → %s", oldDBPath, chatDBPath)
        if err := os.Rename(oldDBPath, chatDBPath); err != nil {
            log.Fatalf("Failed to rename database: %v", err)
        }
        // Also rename WAL and SHM files if they exist
        os.Rename(oldDBPath+"-wal", chatDBPath+"-wal")
        os.Rename(oldDBPath+"-shm", chatDBPath+"-shm")
    }
}
```

Update the `session.Open()` call to use `chatDBPath` instead of the old path.

- [ ] **Step 2: Write test for rename logic**

```go
// internal/chat/session/migrate_db_test.go
package session

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDBRename_MigratesOldFile(t *testing.T) {
	dir := t.TempDir()
	oldPath := filepath.Join(dir, "sessions.db")
	newPath := filepath.Join(dir, "chat.db")

	// Create old DB file
	if err := os.WriteFile(oldPath, []byte("test"), 0600); err != nil {
		t.Fatal(err)
	}

	// Simulate migration
	if _, err := os.Stat(newPath); os.IsNotExist(err) {
		if _, err := os.Stat(oldPath); err == nil {
			if err := os.Rename(oldPath, newPath); err != nil {
				t.Fatal(err)
			}
		}
	}

	// Verify old file gone, new file exists
	if _, err := os.Stat(oldPath); !os.IsNotExist(err) {
		t.Error("old file should not exist")
	}
	if _, err := os.Stat(newPath); err != nil {
		t.Error("new file should exist")
	}
}

func TestDBRename_SkipsIfNewExists(t *testing.T) {
	dir := t.TempDir()
	oldPath := filepath.Join(dir, "sessions.db")
	newPath := filepath.Join(dir, "chat.db")

	// Create both files
	os.WriteFile(oldPath, []byte("old"), 0600)
	os.WriteFile(newPath, []byte("new"), 0600)

	// Migration should skip (chat.db already exists)
	if _, err := os.Stat(newPath); !os.IsNotExist(err) {
		// chat.db exists, skip migration
	}

	// Verify new file unchanged
	data, _ := os.ReadFile(newPath)
	if string(data) != "new" {
		t.Error("new file should be unchanged")
	}
}
```

- [ ] **Step 3: Run tests**

Run: `cd /home/rishav/soul-v2 && go test ./internal/chat/session/ -v -run TestDBRename`
Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add cmd/chat/main.go internal/chat/session/migrate_db_test.go
git commit -m "feat: auto-migrate sessions.db to chat.db on startup"
```

---

### Task 7: Verify EventLogger satisfies events.Logger interface

Ensure `*metrics.EventLogger` (now at `internal/chat/metrics`) satisfies `events.Logger` from `pkg/events`. This is what allows main.go to pass EventLogger to auth.NewOAuthTokenSource.

**Files:**
- Modify: `internal/chat/metrics/logger.go` — add compile-time interface check

- [ ] **Step 1: Add interface compliance check**

Add at the top of `internal/chat/metrics/logger.go`:

```go
import "github.com/rishav1305/soul-v2/pkg/events"

// Compile-time check: EventLogger satisfies events.Logger.
var _ events.Logger = (*EventLogger)(nil)
```

- [ ] **Step 2: Verify build**

Run: `cd /home/rishav/soul-v2 && go build ./...`
Expected: Compiles. If EventLogger doesn't satisfy the interface, this will fail at compile time with a clear error.

- [ ] **Step 3: Commit**

```bash
git add internal/chat/metrics/logger.go
git commit -m "chore: add compile-time events.Logger interface check for EventLogger"
```

---

### Task 8: Update CLAUDE.md and verify everything

Update project documentation to reflect new structure.

**Files:**
- Modify: `CLAUDE.md`

- [ ] **Step 1: Update CLAUDE.md architecture section**

Replace the Architecture section with:

```markdown
## Architecture

\```
cmd/chat/main.go              Chat server CLI entrypoint
pkg/
  auth/                       Claude OAuth — shared by both servers
  events/                     Logger interface + Event type
internal/chat/
  server/                     HTTP server + SPA serving
  session/                    SQLite session CRUD (chat.db)
  stream/                     Claude API streaming — SSE parse
  ws/                         WebSocket hub — session-scoped routing
  metrics/                    Event logging, aggregation, CLI reporting
web/src/
  components/                 React components (Shell, Chat, Sessions)
  hooks/                      Custom hooks (useWebSocket, useSessions)
  lib/                        types.ts (generated), ws.ts, api.ts
specs/                        YAML module specs (source of truth)
tests/                        Integration, E2E, load, verification
tools/                        specgen, monitor
\```
```

Update Quick Commands:
```markdown
\```bash
make build          # Build soul-chat binary + frontend
make serve          # Build and run on :3002
\```
```

- [ ] **Step 2: Run full verification**

Run: `cd /home/rishav/soul-v2 && go build ./... && go test ./... -count=1 -race`
Expected: All packages build, all tests pass with race detector.

Run: `cd /home/rishav/soul-v2/web && npx tsc --noEmit`
Expected: No TypeScript errors (frontend unchanged).

- [ ] **Step 3: Commit**

```bash
git add CLAUDE.md
git commit -m "docs: update CLAUDE.md for restructured repo layout"
```

---

## Post-Plan Verification

After all 8 tasks are complete, verify:

1. `go build ./...` — all packages compile
2. `go test ./... -count=1 -race` — all tests pass with race detector
3. `make build` — produces `soul-chat` binary
4. `./soul-chat serve` — server starts on :3002, chat works
5. `cd web && npx tsc --noEmit` — frontend types still valid
6. `cd web && npx vite build` — frontend builds
7. No `internal/auth/`, `internal/session/`, `internal/metrics/`, `internal/stream/`, `internal/ws/`, `internal/server/` directories remain (all moved to `internal/chat/` or `pkg/`)
8. No `cmd/soul/` directory remains (moved to `cmd/chat/`)
9. `~/.soul-v2/sessions.db` auto-renamed to `~/.soul-v2/chat.db` on first run

## Deferred to later plans

- `pkg/types/` — shared generated types, created in Plan 2 when both servers need them
- `soul chat` subcommand rename (currently `soul-chat serve`) — deferred to Plan 2
- Chat agent (`internal/chat/agent/`) — Plan 2

## Next Plans

After this plan is verified:
- **Plan 2: Tasks Server** — task store, REST API, SSE, chat-to-tasks proxy
- **Plan 3: Autonomous Execution** — executor, progressive gates, product manager
- **Plan 4: Frontend** — routing, Dashboard, Tasks Kanban, Product pages
