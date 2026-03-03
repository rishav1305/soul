# Task Feedback System — Design Document

**Date:** 2026-03-03
**Status:** Approved
**Goal:** Make the autonomous task pipeline interactive, verifiable, and self-healing through a comment-driven feedback loop with E2E verification.

## Problems Solved

1. **Silent completion** — Tasks complete and merge to dev, but dev server doesn't rebuild. User sees no changes.
2. **No interaction channel** — User cannot communicate issues back to Soul on a per-task basis. No way to say "this didn't work" and get a fix.
3. **No verification** — Task claims completion but no evidence (screenshots, test results) is provided.

## Architecture: Comment-Driven Reactive Loop

### Core Concept

Comments are the primary interaction mechanism between user and Soul on each task. A `CommentWatcher` background goroutine monitors for new user comments and dispatches scoped mini-agents to diagnose and resolve issues.

```
User posts comment → CommentWatcher detects → Mini-agent spawns
  → Diagnoses issue → Fixes in worktree → Commits → Merges to dev
  → Rebuilds dev frontend → Posts reply comment with results
```

### Data Model

**New SQLite tables in `~/.soul/planner.db`:**

```sql
CREATE TABLE task_comments (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    task_id INTEGER NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
    author TEXT NOT NULL,          -- 'user' or 'soul'
    type TEXT NOT NULL DEFAULT 'feedback',  -- 'feedback', 'status', 'verification', 'error'
    body TEXT NOT NULL,
    attachments TEXT DEFAULT '[]', -- JSON array of MinIO keys
    created_at TEXT NOT NULL
);
CREATE INDEX idx_comments_task ON task_comments(task_id, created_at);

CREATE TABLE task_attachments (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    task_id INTEGER NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
    comment_id INTEGER REFERENCES task_comments(id) ON DELETE SET NULL,
    filename TEXT NOT NULL,
    minio_key TEXT NOT NULL,       -- "tasks/{task_id}/{filename}"
    content_type TEXT DEFAULT 'image/png',
    size_bytes INTEGER DEFAULT 0,
    created_at TEXT NOT NULL
);
```

**Go structs:**

```go
type Comment struct {
    ID          int64    `json:"id"`
    TaskID      int64    `json:"task_id"`
    Author      string   `json:"author"`       // "user" | "soul"
    Type        string   `json:"type"`          // "feedback" | "status" | "verification" | "error"
    Body        string   `json:"body"`
    Attachments []string `json:"attachments"`   // MinIO keys
    CreatedAt   string   `json:"created_at"`
}

type Attachment struct {
    ID          int64  `json:"id"`
    TaskID      int64  `json:"task_id"`
    CommentID   *int64 `json:"comment_id,omitempty"`
    Filename    string `json:"filename"`
    MinIOKey    string `json:"minio_key"`
    ContentType string `json:"content_type"`
    SizeBytes   int64  `json:"size_bytes"`
    CreatedAt   string `json:"created_at"`
}
```

### CommentWatcher

**Location:** New file `internal/server/comment_watcher.go`

**Behavior:**
- Background goroutine, started with the server
- Polls `task_comments` every 5 seconds for unprocessed user comments
- Tracks last-processed comment ID to avoid re-processing
- For each new user comment:
  1. Check task is in actionable state (active, validation, or blocked)
  2. Build scoped prompt: task context + all comments + user's new comment
  3. Spawn mini-agent in task's existing worktree
  4. Agent can: read/write files, run commands, commit, merge to dev, rebuild dev, post reply comments, upload screenshots
  5. Agent posts a Soul comment with diagnosis and actions taken
  6. If issue is within scope → fix and move task back to active
  7. If issue is external (API down, infra issue) → fix what it can (restart, rebuild), comment what it did

**Agent scope constraints:**
- Only operates within the task's worktree (`.worktrees/task-{id}/`)
- Cannot modify master branch
- Cannot modify other tasks
- Can merge to dev branch
- Can rebuild dev server frontend

### Dev Server Rebuild

**Trigger:** Task moves to validation stage (after agent completes and merges to dev)

**Implementation:** Add `RebuildDevFrontend()` to the server:

```go
func (s *Server) RebuildDevFrontend() error {
    devWeb := filepath.Join(s.projectRoot, ".worktrees", "dev-server", "web")
    cmd := exec.Command("npx", "vite", "build")
    cmd.Dir = devWeb
    return cmd.Run()
}
```

**Called from:**
1. `processTask()` — after merge-to-dev, before advancing to validation
2. CommentWatcher agent — when user reports stale dev server
3. `moveTask()` handler — when task manually moved to validation

### E2E Verification

**When:** After task merges to dev and dev frontend rebuilds, before advancing to validation.

**Flow:**
1. `processTask()` calls `verifyTask(task)`
2. `verifyTask` uses Rod (remote headless browser on titan-pc) to:
   - Navigate to `http://localhost:3001` (dev server)
   - Take full-page screenshots of key pages
   - Check: page loads (HTTP 200), no JS console errors, key elements present
   - Parse task description for mentioned pages/features to verify
3. Upload screenshots to MinIO bucket `soul-attachments` under `tasks/{id}/`
4. Post a Soul comment of type `verification` with:
   - Pass/fail summary per check
   - Screenshot attachment references
   - Any console errors found
5. If checks fail → task stays in active, comment explains failures
6. If checks pass → task advances to validation with evidence attached

### MinIO Object Storage

**Infrastructure on titan-pc:**
- Container: `titan-minio` (minio/minio)
- API port: 9100 (forwarded via SSH tunnel)
- Console port: 9101
- Data volume: `/mnt/vault/minio-data` (on 2TB HDD)
- Bucket: `soul-attachments`

**SSH tunnel on titan-pi:**
- `~/.config/systemd/user/minio-tunnel.service`
- Forwards titan-pi:9100 → titan-pc:9100

**Go client:** `github.com/minio/minio-go/v7`

**Config in `~/.soul/config.json`:**

```json
{
  "minio": {
    "endpoint": "127.0.0.1:9100",
    "access_key": "<from-vaultwarden>",
    "secret_key": "<from-vaultwarden>",
    "bucket": "soul-attachments",
    "use_ssl": false
  }
}
```

### API Endpoints

**New endpoints:**

```
POST   /api/tasks/{id}/comments     — Add comment (user or soul)
GET    /api/tasks/{id}/comments     — List comments for a task
GET    /api/attachments/{key...}    — Proxy/redirect to MinIO pre-signed URL
POST   /api/tasks/{id}/attachments  — Upload attachment (multipart)
```

**WebSocket events:**

```
task.comment.added  — data: Comment (broadcast when new comment posted)
task.verification   — data: { task_id, passed, checks[] }
```

### Frontend Changes

**TaskDetail.tsx additions:**
- Comment thread section below task output
- Comment input box with submit button
- Comment cards styled by type:
  - `feedback` (user) — text, blue accent
  - `status` (soul) — gray, informational
  - `verification` (soul) — green/red pass/fail cards with screenshot thumbnails
  - `error` (soul) — red banner with details
- Screenshot thumbnails that expand on click (lightbox)
- "Soul is investigating..." spinner when CommentWatcher is processing a user comment

**New TypeScript types:**

```typescript
interface TaskComment {
    id: number;
    task_id: number;
    author: 'user' | 'soul';
    type: 'feedback' | 'status' | 'verification' | 'error';
    body: string;
    attachments: string[];
    created_at: string;
}
```

**usePlanner hook extensions:**
- `addComment(taskId, body)` — POST to comments API
- `comments: Record<taskId, TaskComment[]>` — cached comments
- WebSocket handler for `task.comment.added`

## File Summary

| File | Action | Purpose |
|------|--------|---------|
| `internal/planner/store.go` | MODIFY | Add comments + attachments tables to schema |
| `internal/planner/types.go` | MODIFY | Add Comment, Attachment types |
| `internal/planner/comments.go` | CREATE | CRUD operations for comments |
| `internal/server/comment_watcher.go` | CREATE | CommentWatcher goroutine |
| `internal/server/autonomous.go` | MODIFY | Add dev rebuild + E2E verification steps |
| `internal/server/tasks.go` | MODIFY | Add comment API endpoints |
| `internal/server/routes.go` | MODIFY | Register new routes |
| `internal/server/minio.go` | CREATE | MinIO client wrapper |
| `internal/server/verification.go` | CREATE | Rod-based E2E verification |
| `internal/server/server.go` | MODIFY | Add RebuildDevFrontend(), start CommentWatcher |
| `web/src/lib/types.ts` | MODIFY | Add TaskComment type |
| `web/src/hooks/usePlanner.ts` | MODIFY | Add comment methods + WebSocket handler |
| `web/src/components/planner/TaskDetail.tsx` | MODIFY | Add comment thread UI |
| `web/src/components/planner/CommentCard.tsx` | CREATE | Styled comment display |
| `web/src/components/planner/VerificationCard.tsx` | CREATE | Pass/fail + screenshots |
| Infra: MinIO container on titan-pc | CREATE | docker-compose, SSH tunnel, systemd |

## Execution Order

1. **MinIO infrastructure** — Container, tunnel, Go client
2. **Data model** — SQLite schema migration, Go types, CRUD
3. **Comment API + WebSocket** — Endpoints, broadcasting
4. **Dev server rebuild** — RebuildDevFrontend() + integration in processTask
5. **CommentWatcher** — Polling loop, mini-agent dispatch
6. **E2E Verification** — Rod screenshots, MinIO upload, verification comments
7. **Frontend** — Comment thread UI, input, styled cards, screenshots
8. **Integration testing** — Full flow: create task → auto → comment → fix → verify
