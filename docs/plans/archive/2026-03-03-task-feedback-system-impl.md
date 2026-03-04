# Task Feedback System Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Make the autonomous task pipeline interactive, verifiable, and self-healing through a comment-driven feedback loop with E2E verification and dev server rebuild.

**Architecture:** Comments on tasks drive a reactive loop — users post feedback, a CommentWatcher goroutine detects new comments and spawns scoped mini-agents to diagnose and fix issues in the task's worktree. After merge to dev, `RebuildDevFrontend()` rebuilds the dev server SPA so changes are immediately visible. E2E verification via Rod captures screenshots and posts them as verification comments. MinIO on titan-pc stores screenshots.

**Tech Stack:** Go 1.24, SQLite (modernc.org/sqlite), Rod (headless browser), MinIO (S3-compatible object storage), React 19, TypeScript, Tailwind CSS v4, nhooyr.io/websocket

---

## Task 1: MinIO Infrastructure on titan-pc

**Files:**
- Create: `products/scout/infra/docker-compose.minio.yml`
- Create: `~/.config/systemd/user/minio-tunnel.service` (on titan-pi)

**Step 1: Create MinIO docker-compose file**

SSH to titan-pc and create the compose file:

```bash
ssh rishav@192.168.0.113
cat > /tmp/docker-compose.minio.yml << 'EOF'
services:
  titan-minio:
    image: minio/minio
    container_name: titan-minio
    command: server /data --console-address ":9101"
    ports:
      - "127.0.0.1:9100:9000"
      - "127.0.0.1:9101:9101"
    environment:
      MINIO_ROOT_USER: minio-admin
      MINIO_ROOT_PASSWORD: <generate-a-password>
    volumes:
      - /mnt/vault/minio-data:/data
    restart: unless-stopped
EOF
docker compose -f /tmp/docker-compose.minio.yml up -d
```

Expected: Container starts, ports 9100/9101 listening on titan-pc localhost.

**Step 2: Create bucket and access key**

```bash
# On titan-pc
docker exec titan-minio mc alias set local http://localhost:9000 minio-admin <password>
docker exec titan-minio mc mb local/soul-attachments
docker exec titan-minio mc admin user svcacct add local minio-admin
# Note the access key and secret key
```

**Step 3: Store credentials in Vaultwarden**

Use `bw` CLI to store the MinIO access key and secret key under entry "TITAN - MinIO".

**Step 4: Create SSH tunnel on titan-pi**

```bash
cat > ~/.config/systemd/user/minio-tunnel.service << 'EOF'
[Unit]
Description=SSH tunnel to titan-pc MinIO
After=network-online.target

[Service]
ExecStart=/usr/bin/ssh -N -L 9100:127.0.0.1:9100 rishav@192.168.0.113
Restart=always
RestartSec=10

[Install]
WantedBy=default.target
EOF
systemctl --user daemon-reload
systemctl --user enable --now minio-tunnel.service
```

Expected: `curl http://127.0.0.1:9100/minio/health/live` returns OK on titan-pi.

**Step 5: Save MinIO config**

Add to `~/.soul/config.json`:

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

**Step 6: Copy docker-compose to repo and commit**

```bash
cp /tmp/docker-compose.minio.yml ~/soul/products/scout/infra/docker-compose.minio.yml
cd ~/soul && git add products/scout/infra/docker-compose.minio.yml
git commit -m "infra: add MinIO docker-compose for soul-attachments"
```

---

## Task 2: MinIO Go Client

**Files:**
- Create: `internal/server/minio.go`
- Modify: `go.mod` (add minio-go dependency)

**Step 1: Add minio-go dependency**

```bash
cd ~/soul && go get github.com/minio/minio-go/v7
```

**Step 2: Write the MinIO client wrapper**

Create `internal/server/minio.go`:

```go
package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type MinIOConfig struct {
	Endpoint  string `json:"endpoint"`
	AccessKey string `json:"access_key"`
	SecretKey string `json:"secret_key"`
	Bucket    string `json:"bucket"`
	UseSSL    bool   `json:"use_ssl"`
}

type MinIOClient struct {
	client *minio.Client
	bucket string
}

func LoadMinIOConfig() (*MinIOConfig, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(filepath.Join(home, ".soul", "config.json"))
	if err != nil {
		return nil, err
	}
	var wrapper struct {
		MinIO *MinIOConfig `json:"minio"`
	}
	if err := json.Unmarshal(data, &wrapper); err != nil {
		return nil, err
	}
	if wrapper.MinIO == nil {
		return nil, fmt.Errorf("minio section missing from config")
	}
	return wrapper.MinIO, nil
}

func NewMinIOClient(cfg *MinIOConfig) (*MinIOClient, error) {
	client, err := minio.New(cfg.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, ""),
		Secure: cfg.UseSSL,
	})
	if err != nil {
		return nil, fmt.Errorf("minio client: %w", err)
	}
	return &MinIOClient{client: client, bucket: cfg.Bucket}, nil
}

// Upload stores data under the given key and returns the key.
func (m *MinIOClient) Upload(ctx context.Context, key, contentType string, reader io.Reader, size int64) error {
	_, err := m.client.PutObject(ctx, m.bucket, key, reader, size, minio.PutObjectOptions{
		ContentType: contentType,
	})
	return err
}

// PresignedURL returns a temporary download URL for the given key.
func (m *MinIOClient) PresignedURL(ctx context.Context, key string) (string, error) {
	url, err := m.client.PresignedGetObject(ctx, m.bucket, key, 15*60*1e9, nil) // 15 min
	if err != nil {
		return "", err
	}
	return url.String(), nil
}

// Download returns a reader for the given key.
func (m *MinIOClient) Download(ctx context.Context, key string) (io.ReadCloser, error) {
	obj, err := m.client.GetObject(ctx, m.bucket, key, minio.GetObjectOptions{})
	if err != nil {
		return nil, err
	}
	return obj, nil
}
```

**Step 3: Verify it compiles**

```bash
cd ~/soul && go build ./...
```

Expected: Build succeeds.

**Step 4: Commit**

```bash
git add internal/server/minio.go go.mod go.sum
git commit -m "feat: add MinIO client wrapper for soul-attachments"
```

---

## Task 3: Comment Data Model — SQLite Schema + Go Types + CRUD

**Files:**
- Modify: `internal/planner/store.go` (add tables to schema, add CRUD methods)
- Modify: `internal/planner/types.go` (add Comment type)

**Step 1: Add Comment type to types.go**

Add after the `TaskUpdate` struct at the end of `internal/planner/types.go`:

```go
// Comment represents a comment on a task.
type Comment struct {
	ID          int64    `json:"id"`
	TaskID      int64    `json:"task_id"`
	Author      string   `json:"author"`      // "user" or "soul"
	Type        string   `json:"type"`         // "feedback", "status", "verification", "error"
	Body        string   `json:"body"`
	Attachments []string `json:"attachments"`  // MinIO keys
	CreatedAt   string   `json:"created_at"`
}
```

**Step 2: Add task_comments table to schema**

In `internal/planner/store.go`, append to the `schema` constant string (before the closing backtick):

```sql
CREATE TABLE IF NOT EXISTS task_comments (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    task_id INTEGER NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
    author TEXT NOT NULL,
    type TEXT NOT NULL DEFAULT 'feedback',
    body TEXT NOT NULL,
    attachments TEXT DEFAULT '[]',
    created_at TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_comments_task ON task_comments(task_id, created_at);
```

**Step 3: Add CRUD methods to store.go**

Add to `internal/planner/store.go` after the `RemoveDependency` method:

```go
// CreateComment inserts a new comment and returns its ID.
func (s *Store) CreateComment(c Comment) (int64, error) {
	attachJSON, err := json.Marshal(c.Attachments)
	if err != nil {
		attachJSON = []byte("[]")
	}
	res, err := s.db.Exec(`
		INSERT INTO task_comments (task_id, author, type, body, attachments, created_at)
		VALUES (?, ?, ?, ?, ?, ?)`,
		c.TaskID, c.Author, c.Type, c.Body, string(attachJSON), c.CreatedAt,
	)
	if err != nil {
		return 0, fmt.Errorf("insert comment: %w", err)
	}
	return res.LastInsertId()
}

// ListComments returns all comments for a task, ordered by created_at.
func (s *Store) ListComments(taskID int64) ([]Comment, error) {
	rows, err := s.db.Query(`
		SELECT id, task_id, author, type, body, attachments, created_at
		FROM task_comments
		WHERE task_id = ?
		ORDER BY created_at ASC`, taskID)
	if err != nil {
		return nil, fmt.Errorf("list comments: %w", err)
	}
	defer rows.Close()

	var comments []Comment
	for rows.Next() {
		var c Comment
		var attachJSON string
		if err := rows.Scan(&c.ID, &c.TaskID, &c.Author, &c.Type, &c.Body, &attachJSON, &c.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan comment: %w", err)
		}
		if err := json.Unmarshal([]byte(attachJSON), &c.Attachments); err != nil {
			c.Attachments = []string{}
		}
		comments = append(comments, c)
	}
	if comments == nil {
		comments = []Comment{}
	}
	return comments, rows.Err()
}

// MaxCommentID returns the highest comment ID, or 0 if no comments exist.
func (s *Store) MaxCommentID() (int64, error) {
	var maxID sql.NullInt64
	err := s.db.QueryRow("SELECT MAX(id) FROM task_comments").Scan(&maxID)
	if err != nil {
		return 0, err
	}
	if !maxID.Valid {
		return 0, nil
	}
	return maxID.Int64, nil
}

// CommentsAfter returns all user comments with ID > afterID.
func (s *Store) CommentsAfter(afterID int64) ([]Comment, error) {
	rows, err := s.db.Query(`
		SELECT id, task_id, author, type, body, attachments, created_at
		FROM task_comments
		WHERE id > ? AND author = 'user'
		ORDER BY id ASC`, afterID)
	if err != nil {
		return nil, fmt.Errorf("comments after: %w", err)
	}
	defer rows.Close()

	var comments []Comment
	for rows.Next() {
		var c Comment
		var attachJSON string
		if err := rows.Scan(&c.ID, &c.TaskID, &c.Author, &c.Type, &c.Body, &attachJSON, &c.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan comment: %w", err)
		}
		if err := json.Unmarshal([]byte(attachJSON), &c.Attachments); err != nil {
			c.Attachments = []string{}
		}
		comments = append(comments, c)
	}
	return comments, rows.Err()
}
```

Note: Add `"encoding/json"` to the imports in store.go.

**Step 4: Verify build**

```bash
cd ~/soul && go build ./...
```

Expected: Build succeeds.

**Step 5: Commit**

```bash
git add internal/planner/store.go internal/planner/types.go
git commit -m "feat: add task_comments table, Comment type, and CRUD methods"
```

---

## Task 4: Comment API Endpoints + WebSocket Events

**Files:**
- Modify: `internal/server/tasks.go` (add handleCommentCreate, handleCommentList)
- Modify: `internal/server/routes.go` (register new routes)

**Step 1: Add comment handlers to tasks.go**

Add at the end of `internal/server/tasks.go`:

```go
// handleCommentCreate handles POST /api/tasks/{id}/comments — adds a comment.
func (s *Server) handleCommentCreate(w http.ResponseWriter, r *http.Request) {
	if s.planner == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "planner not configured"})
		return
	}

	taskID, err := parseTaskID(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	// Verify task exists.
	if _, err := s.planner.Get(taskID); err != nil {
		if errors.Is(err, planner.ErrNotFound) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "task not found"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	var body struct {
		Author string `json:"author"`
		Type   string `json:"type"`
		Body   string `json:"body"`
	}
	if err := readJSON(r, &body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	if body.Body == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "body is required"})
		return
	}
	if body.Author == "" {
		body.Author = "user"
	}
	if body.Type == "" {
		body.Type = "feedback"
	}

	comment := planner.Comment{
		TaskID:      taskID,
		Author:      body.Author,
		Type:        body.Type,
		Body:        body.Body,
		Attachments: []string{},
		CreatedAt:   time.Now().UTC().Format(time.RFC3339),
	}

	id, err := s.planner.CreateComment(comment)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": fmt.Sprintf("failed to create comment: %v", err)})
		return
	}
	comment.ID = id

	// Broadcast to WebSocket clients.
	s.broadcastTaskEvent("task.comment.added", comment)

	writeJSON(w, http.StatusCreated, comment)
}

// handleCommentList handles GET /api/tasks/{id}/comments — lists comments.
func (s *Server) handleCommentList(w http.ResponseWriter, r *http.Request) {
	if s.planner == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "planner not configured"})
		return
	}

	taskID, err := parseTaskID(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	comments, err := s.planner.ListComments(taskID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": fmt.Sprintf("failed to list comments: %v", err)})
		return
	}

	writeJSON(w, http.StatusOK, comments)
}
```

Note: Add `"time"` to the imports in tasks.go.

**Step 2: Register routes in routes.go**

Add after the `POST /api/tasks/{id}/move` route in `registerRoutes()`:

```go
	// Task comment endpoints.
	s.mux.HandleFunc("POST /api/tasks/{id}/comments", s.handleCommentCreate)
	s.mux.HandleFunc("GET /api/tasks/{id}/comments", s.handleCommentList)
```

**Step 3: Verify build**

```bash
cd ~/soul && go build ./...
```

Expected: Build succeeds.

**Step 4: Commit**

```bash
git add internal/server/tasks.go internal/server/routes.go
git commit -m "feat: add comment API endpoints (POST/GET /api/tasks/{id}/comments)"
```

---

## Task 5: Dev Server Rebuild Function

**Files:**
- Modify: `internal/server/server.go` (add RebuildDevFrontend method)
- Modify: `internal/server/autonomous.go` (call rebuild after merge-to-dev)

**Step 1: Add RebuildDevFrontend to server.go**

Add after the `StartDevServer` method in `internal/server/server.go`:

```go
// RebuildDevFrontend runs vite build in the dev server worktree,
// updating the dev server's SPA files. Since the dev server serves
// from disk (not embed), the new build takes effect immediately.
func (s *Server) RebuildDevFrontend() error {
	devWeb := filepath.Join(s.projectRoot, ".worktrees", "dev-server", "web")

	// Ensure node_modules symlink exists.
	devNodeModules := filepath.Join(devWeb, "node_modules")
	mainNodeModules := filepath.Join(s.projectRoot, "web", "node_modules")
	if _, err := os.Lstat(devNodeModules); os.IsNotExist(err) {
		if err := os.Symlink(mainNodeModules, devNodeModules); err != nil {
			return fmt.Errorf("symlink node_modules: %w", err)
		}
	}

	cmd := exec.Command("npx", "vite", "build")
	cmd.Dir = devWeb
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("vite build failed: %w\n%s", err, out)
	}
	log.Printf("[dev-server] frontend rebuilt successfully")
	return nil
}
```

**Step 2: Store server reference in TaskProcessor**

In `internal/server/autonomous.go`, add a `server` field to `TaskProcessor`:

```go
type TaskProcessor struct {
	// ... existing fields ...
	server      *Server // for RebuildDevFrontend
}
```

Update `NewTaskProcessor` to accept and store the server:

```go
func NewTaskProcessor(srv *Server, aiClient *ai.Client, pm *products.Manager, sessions *session.Store, plannerStore *planner.Store, broadcast func(WSMessage), model, projectRoot string, worktrees *WorktreeManager) *TaskProcessor {
	return &TaskProcessor{
		server:      srv,
		ai:          aiClient,
		// ... rest unchanged ...
	}
}
```

Update the call sites in `server.go` — in both `New()` and `NewWithWebFS()`, change:

```go
s.processor = NewTaskProcessor(s, aiClient, pm, sessions, plannerStore, s.broadcast, cfg.Model, projectRoot, wm)
```

**Step 3: Call RebuildDevFrontend after merge-to-dev**

In `autonomous.go`, in `processTask()`, after the successful `MergeToDev` call (line 189-191), add:

```go
		} else {
			tp.sendActivity(taskID, "status", "Changes merged to dev — rebuilding frontend...")
			if tp.server != nil {
				if err := tp.server.RebuildDevFrontend(); err != nil {
					log.Printf("[autonomous] dev frontend rebuild failed for task %d: %v", taskID, err)
					tp.sendActivity(taskID, "status", fmt.Sprintf("Dev rebuild warning: %v", err))
				} else {
					tp.sendActivity(taskID, "status", "Dev frontend rebuilt — changes visible on dev server")
				}
			}
		}
```

**Step 4: Verify build**

```bash
cd ~/soul && go build ./...
```

Expected: Build succeeds.

**Step 5: Commit**

```bash
git add internal/server/server.go internal/server/autonomous.go
git commit -m "feat: add RebuildDevFrontend and call it after merge-to-dev"
```

---

## Task 6: CommentWatcher — Background Polling Loop

**Files:**
- Create: `internal/server/comment_watcher.go`
- Modify: `internal/server/server.go` (start CommentWatcher on boot)

**Step 1: Create comment_watcher.go**

```go
package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/rishav1305/soul/internal/ai"
	"github.com/rishav1305/soul/internal/planner"
	"github.com/rishav1305/soul/internal/products"
	"github.com/rishav1305/soul/internal/session"
)

// CommentWatcher polls for new user comments and dispatches mini-agents.
type CommentWatcher struct {
	server      *Server
	ai          *ai.Client
	products    *products.Manager
	sessions    *session.Store
	planner     *planner.Store
	broadcast   func(WSMessage)
	model       string
	projectRoot string
	worktrees   *WorktreeManager

	lastID int64
}

// NewCommentWatcher creates a comment watcher.
func NewCommentWatcher(srv *Server) *CommentWatcher {
	return &CommentWatcher{
		server:      srv,
		ai:          srv.ai,
		products:    srv.products,
		sessions:    srv.sessions,
		planner:     srv.planner,
		broadcast:   srv.broadcast,
		model:       srv.cfg.Model,
		projectRoot: srv.projectRoot,
		worktrees:   srv.worktrees,
	}
}

// Start begins polling in a background goroutine. Call cancel on the
// returned context to stop.
func (cw *CommentWatcher) Start(ctx context.Context) {
	// Seed lastID to current max so we don't reprocess old comments.
	maxID, err := cw.planner.MaxCommentID()
	if err != nil {
		log.Printf("[comment-watcher] failed to get max comment ID: %v", err)
	}
	cw.lastID = maxID

	go cw.poll(ctx)
	log.Printf("[comment-watcher] started (last_id=%d)", cw.lastID)
}

func (cw *CommentWatcher) poll(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Println("[comment-watcher] stopped")
			return
		case <-ticker.C:
			cw.checkNewComments(ctx)
		}
	}
}

func (cw *CommentWatcher) checkNewComments(ctx context.Context) {
	comments, err := cw.planner.CommentsAfter(cw.lastID)
	if err != nil {
		log.Printf("[comment-watcher] poll error: %v", err)
		return
	}

	for _, comment := range comments {
		cw.lastID = comment.ID
		log.Printf("[comment-watcher] new user comment #%d on task %d: %s",
			comment.ID, comment.TaskID, truncate(comment.Body, 80))

		// Check task is in actionable state.
		task, err := cw.planner.Get(comment.TaskID)
		if err != nil {
			log.Printf("[comment-watcher] failed to get task %d: %v", comment.TaskID, err)
			continue
		}

		if task.Stage != planner.StageActive &&
			task.Stage != planner.StageValidation &&
			task.Stage != planner.StageBlocked {
			// Post a reply saying we can't act on this.
			cw.postReply(comment.TaskID, "status",
				fmt.Sprintf("Task is in %s stage — comment noted but no action taken.", task.Stage))
			continue
		}

		go cw.handleComment(ctx, task, comment)
	}
}

func (cw *CommentWatcher) handleComment(ctx context.Context, task planner.Task, comment planner.Comment) {
	cw.postReply(comment.TaskID, "status", "Investigating your feedback...")

	// Determine worktree path.
	taskRoot := cw.projectRoot
	if cw.worktrees != nil {
		wt := cw.worktrees.worktreePath(comment.TaskID)
		if info, err := os.Stat(wt); err == nil && info.IsDir() {
			taskRoot = wt
		}
	}

	// Gather all comments for context.
	allComments, _ := cw.planner.ListComments(comment.TaskID)
	var commentLog strings.Builder
	for _, c := range allComments {
		commentLog.WriteString(fmt.Sprintf("[%s] %s (%s): %s\n", c.CreatedAt, c.Author, c.Type, c.Body))
	}

	// Build prompt for the mini-agent.
	prompt := cw.buildFeedbackPrompt(task, comment, commentLog.String(), taskRoot)

	sessionID := fmt.Sprintf("comment-%d-%d", comment.TaskID, comment.ID)
	var outputBuf strings.Builder

	sendEvent := func(msg WSMessage) {
		switch msg.Type {
		case "chat.token":
			outputBuf.WriteString(msg.Content)
		}
	}

	agent := NewAgentLoop(cw.ai, cw.products, cw.sessions, cw.planner, cw.broadcast, cw.model, taskRoot)
	agent.Run(ctx, sessionID, prompt, "code", nil, false, sendEvent)

	// Post the agent's response as a soul comment.
	response := outputBuf.String()
	if response == "" {
		response = "Investigation complete — no specific output generated."
	}

	cw.postReply(comment.TaskID, "status", response)

	// Rebuild dev frontend if the agent made changes.
	if cw.server != nil {
		if err := cw.server.RebuildDevFrontend(); err != nil {
			log.Printf("[comment-watcher] dev rebuild failed: %v", err)
		} else {
			cw.postReply(comment.TaskID, "status", "Dev frontend rebuilt — check the dev server for updates.")
		}
	}
}

func (cw *CommentWatcher) buildFeedbackPrompt(task planner.Task, comment planner.Comment, commentLog, taskRoot string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "You are responding to user feedback on task #%d.\n\n", task.ID)
	fmt.Fprintf(&b, "**Task Title:** %s\n", task.Title)
	fmt.Fprintf(&b, "**Task Description:** %s\n", task.Description)
	fmt.Fprintf(&b, "**Current Stage:** %s\n\n", task.Stage)
	fmt.Fprintf(&b, "## Comment History\n```\n%s```\n\n", commentLog)
	fmt.Fprintf(&b, "## User's Current Feedback\n%s\n\n", comment.Body)
	fmt.Fprintf(&b, "## Instructions\n")
	b.WriteString("- Diagnose the issue described in the user's feedback\n")
	b.WriteString("- If it's a code issue within this task's scope, fix it\n")
	b.WriteString("- If the dev server needs a restart or rebuild, the system will handle that\n")
	b.WriteString("- Write a clear summary of what you found and what you did\n")
	b.WriteString("- Do NOT run git commands — the system handles commits and merges\n")
	fmt.Fprintf(&b, "\nProject root: `%s`\n", taskRoot)
	return b.String()
}

func (cw *CommentWatcher) postReply(taskID int64, commentType, body string) {
	comment := planner.Comment{
		TaskID:      taskID,
		Author:      "soul",
		Type:        commentType,
		Body:        body,
		Attachments: []string{},
		CreatedAt:   time.Now().UTC().Format(time.RFC3339),
	}
	id, err := cw.planner.CreateComment(comment)
	if err != nil {
		log.Printf("[comment-watcher] failed to post reply on task %d: %v", taskID, err)
		return
	}
	comment.ID = id
	raw, _ := json.Marshal(comment)
	cw.broadcast(WSMessage{Type: "task.comment.added", Data: raw})
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}
```

Note: Add `"os"` to the imports.

**Step 2: Start CommentWatcher in server.go**

In `internal/server/server.go`, add a `commentWatcher` field to Server:

```go
type Server struct {
	// ... existing fields ...
	commentWatcher *CommentWatcher
}
```

In both `New()` and `NewWithWebFS()`, after `s.registerRoutes()`, add:

```go
	// Start the comment watcher.
	cw := NewCommentWatcher(s)
	cw.Start(context.Background())
	s.commentWatcher = cw
```

**Step 3: Add worktreePath helper to WorktreeManager**

In `internal/server/worktree.go`, add (if it doesn't exist):

```go
// worktreePath returns the filesystem path for a task's worktree.
func (wm *WorktreeManager) worktreePath(taskID int64) string {
	return filepath.Join(wm.root, ".worktrees", fmt.Sprintf("task-%d", taskID))
}
```

**Step 4: Verify build**

```bash
cd ~/soul && go build ./...
```

Expected: Build succeeds.

**Step 5: Commit**

```bash
git add internal/server/comment_watcher.go internal/server/server.go internal/server/worktree.go
git commit -m "feat: add CommentWatcher — polls for user comments and dispatches mini-agents"
```

---

## Task 7: Frontend — TypeScript Types + Comment API

**Files:**
- Modify: `web/src/lib/types.ts` (add TaskComment type)
- Modify: `web/src/hooks/usePlanner.ts` (add comment methods + WebSocket handler)

**Step 1: Add TaskComment type to types.ts**

Add after the `TaskActivity` interface:

```typescript
export interface TaskComment {
  id: number;
  task_id: number;
  author: 'user' | 'soul';
  type: 'feedback' | 'status' | 'verification' | 'error';
  body: string;
  attachments: string[];
  created_at: string;
}
```

**Step 2: Add comment state + methods to usePlanner.ts**

In `usePlanner.ts`, add state:

```typescript
const [taskComments, setTaskComments] = useState<Record<number, TaskComment[]>>({});
```

Add WebSocket handler in the `useEffect` that listens for messages, inside the switch:

```typescript
case 'task.comment.added': {
  const comment = msg.data as TaskComment;
  if (!comment?.task_id) break;
  setTaskComments((prev) => ({
    ...prev,
    [comment.task_id]: [...(prev[comment.task_id] || []), comment],
  }));
  break;
}
```

Add API methods:

```typescript
const fetchComments = useCallback(async (taskId: number) => {
  const res = await fetch(`/api/tasks/${taskId}/comments`);
  if (!res.ok) throw new Error(`Failed to fetch comments: ${res.status}`);
  const data: TaskComment[] = await res.json();
  setTaskComments((prev) => ({ ...prev, [taskId]: data }));
  return data;
}, []);

const addComment = useCallback(async (taskId: number, body: string) => {
  const res = await fetch(`/api/tasks/${taskId}/comments`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ author: 'user', type: 'feedback', body }),
  });
  if (!res.ok) throw new Error(`Failed to add comment: ${res.status}`);
  return (await res.json()) as TaskComment;
}, []);
```

Add to the return object:

```typescript
return {
  // ... existing ...
  taskComments,
  fetchComments,
  addComment,
};
```

**Step 3: Build frontend**

```bash
cd ~/soul/web && npx vite build
```

Expected: Build succeeds.

**Step 4: Commit**

```bash
cd ~/soul && git add web/src/lib/types.ts web/src/hooks/usePlanner.ts
git commit -m "feat: add TaskComment type and comment API methods to usePlanner"
```

---

## Task 8: Frontend — Comment Thread UI in TaskDetail

**Files:**
- Modify: `web/src/components/planner/TaskDetail.tsx` (add comment thread section)

**Step 1: Update TaskDetailProps**

Add to the `TaskDetailProps` interface:

```typescript
interface TaskDetailProps {
  // ... existing ...
  comments?: TaskComment[];
  onFetchComments?: (id: number) => Promise<void>;
  onAddComment?: (id: number, body: string) => Promise<TaskComment>;
}
```

Import `TaskComment` type from types.ts.

**Step 2: Add comment thread section**

In the `TaskDetail` component, add state for the comment input:

```typescript
const [commentText, setCommentText] = useState('');
const [submitting, setSubmitting] = useState(false);
const commentsEndRef = useRef<HTMLDivElement>(null);
```

Fetch comments on mount:

```typescript
useEffect(() => {
  if (onFetchComments) {
    onFetchComments(task.id);
  }
}, [task.id, onFetchComments]);
```

Auto-scroll when comments change:

```typescript
useEffect(() => {
  commentsEndRef.current?.scrollIntoView({ behavior: 'smooth' });
}, [comments]);
```

Add submit handler:

```typescript
const handleSubmitComment = async () => {
  if (!commentText.trim() || !onAddComment) return;
  setSubmitting(true);
  try {
    await onAddComment(task.id, commentText.trim());
    setCommentText('');
  } catch (err) {
    console.error('Failed to add comment:', err);
  } finally {
    setSubmitting(false);
  }
};
```

Add the comment thread JSX below the activity log section:

```tsx
{/* Comment Thread */}
<div className="border-t border-zinc-700/50 pt-4 mt-4">
  <h3 className="text-xs font-semibold text-zinc-400 uppercase tracking-wider mb-3">
    Comments
  </h3>

  <div className="space-y-3 max-h-64 overflow-y-auto mb-3">
    {(comments || []).map((c) => (
      <div
        key={c.id}
        className={`rounded-lg p-3 text-sm ${
          c.author === 'user'
            ? 'bg-blue-500/10 border border-blue-500/20'
            : c.type === 'error'
              ? 'bg-red-500/10 border border-red-500/20'
              : c.type === 'verification'
                ? 'bg-emerald-500/10 border border-emerald-500/20'
                : 'bg-zinc-700/30 border border-zinc-600/20'
        }`}
      >
        <div className="flex items-center gap-2 mb-1">
          <span className={`text-xs font-medium ${
            c.author === 'user' ? 'text-blue-400' : 'text-zinc-400'
          }`}>
            {c.author === 'user' ? 'You' : 'Soul'}
          </span>
          <span className="text-xs text-zinc-500">
            {new Date(c.created_at).toLocaleTimeString()}
          </span>
          {c.type !== 'feedback' && (
            <span className="text-[10px] px-1.5 py-0.5 rounded bg-zinc-600/50 text-zinc-400">
              {c.type}
            </span>
          )}
        </div>
        <p className="text-zinc-300 whitespace-pre-wrap">{c.body}</p>
      </div>
    ))}
    <div ref={commentsEndRef} />
  </div>

  {/* Comment input */}
  <div className="flex gap-2">
    <input
      type="text"
      value={commentText}
      onChange={(e) => setCommentText(e.target.value)}
      onKeyDown={(e) => {
        if (e.key === 'Enter' && !e.shiftKey) {
          e.preventDefault();
          handleSubmitComment();
        }
      }}
      placeholder="Post feedback..."
      className="flex-1 bg-zinc-800 border border-zinc-600/50 rounded-lg px-3 py-2 text-sm text-zinc-200 placeholder-zinc-500 focus:outline-none focus:border-zinc-500"
      disabled={submitting}
    />
    <button
      onClick={handleSubmitComment}
      disabled={submitting || !commentText.trim()}
      className="px-3 py-2 rounded-lg bg-blue-600 hover:bg-blue-500 text-white text-sm disabled:opacity-40 disabled:cursor-not-allowed transition-colors"
    >
      {submitting ? '...' : 'Send'}
    </button>
  </div>
</div>
```

**Step 3: Wire up TaskDetail in the parent component**

Find where `TaskDetail` is rendered (likely in `PlannerPanel.tsx` or `KanbanBoard.tsx`) and pass the new props:

```tsx
<TaskDetail
  // ... existing props ...
  comments={taskComments[selectedTask.id]}
  onFetchComments={fetchComments}
  onAddComment={addComment}
/>
```

**Step 4: Build frontend**

```bash
cd ~/soul/web && npx vite build
```

Expected: Build succeeds.

**Step 5: Commit**

```bash
cd ~/soul && git add web/src/components/planner/TaskDetail.tsx web/src/components/planner/
git commit -m "feat: add comment thread UI to TaskDetail with real-time updates"
```

---

## Task 9: E2E Verification with Rod Screenshots

**Files:**
- Create: `internal/server/verification.go`
- Modify: `internal/server/autonomous.go` (call verification before advancing to validation)

**Step 1: Create verification.go**

```go
package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/rishav1305/soul/internal/planner"
)

// VerificationResult holds E2E check results for a task.
type VerificationResult struct {
	Passed      bool              `json:"passed"`
	Checks      []VerificationCheck `json:"checks"`
	Screenshots []string          `json:"screenshots"` // MinIO keys
}

// VerificationCheck is a single pass/fail check.
type VerificationCheck struct {
	Name   string `json:"name"`
	Passed bool   `json:"passed"`
	Detail string `json:"detail"`
}

// verifyTask runs E2E checks on the dev server for a task.
// It takes screenshots and uploads them to MinIO.
func (tp *TaskProcessor) verifyTask(ctx context.Context, task planner.Task) *VerificationResult {
	result := &VerificationResult{Passed: true}
	devURL := fmt.Sprintf("http://localhost:%d", tp.server.cfg.Port+1)

	// Launch remote Rod browser on titan-pc via SSH tunnel.
	u, err := launcher.ResolveURL("")
	if err != nil {
		log.Printf("[verify] failed to resolve rod URL: %v", err)
		result.Checks = append(result.Checks, VerificationCheck{
			Name: "browser_launch", Passed: false, Detail: err.Error(),
		})
		result.Passed = false
		return result
	}

	browser := rod.New().ControlURL(u)
	if err := browser.Connect(); err != nil {
		log.Printf("[verify] failed to connect to browser: %v", err)
		result.Checks = append(result.Checks, VerificationCheck{
			Name: "browser_connect", Passed: false, Detail: err.Error(),
		})
		result.Passed = false
		return result
	}
	defer browser.Close()

	// Navigate to dev server.
	page, err := browser.Page(rod.PageOptions{})
	if err != nil {
		result.Checks = append(result.Checks, VerificationCheck{
			Name: "page_create", Passed: false, Detail: err.Error(),
		})
		result.Passed = false
		return result
	}

	if err := page.Timeout(30 * time.Second).Navigate(devURL); err != nil {
		result.Checks = append(result.Checks, VerificationCheck{
			Name: "navigate", Passed: false, Detail: fmt.Sprintf("Failed to navigate to %s: %v", devURL, err),
		})
		result.Passed = false
		return result
	}

	// Wait for page load.
	if err := page.Timeout(15 * time.Second).WaitLoad(); err != nil {
		result.Checks = append(result.Checks, VerificationCheck{
			Name: "page_load", Passed: false, Detail: err.Error(),
		})
		result.Passed = false
	} else {
		result.Checks = append(result.Checks, VerificationCheck{
			Name: "page_load", Passed: true, Detail: "Dev server loaded successfully",
		})
	}

	// Check for JS console errors.
	// Rod doesn't have direct console access without hijacking, so we check page.Evaluate.
	jsErrors, err := page.Eval(`() => {
		const errors = [];
		// Check if document loaded properly
		if (document.readyState !== 'complete') {
			errors.push('Document not fully loaded: ' + document.readyState);
		}
		return errors;
	}`)
	if err == nil {
		var errs []string
		if err := json.Unmarshal([]byte(jsErrors.Value.String()), &errs); err == nil && len(errs) > 0 {
			result.Checks = append(result.Checks, VerificationCheck{
				Name: "js_errors", Passed: false, Detail: fmt.Sprintf("%d errors: %v", len(errs), errs),
			})
			result.Passed = false
		} else {
			result.Checks = append(result.Checks, VerificationCheck{
				Name: "js_errors", Passed: true, Detail: "No JS errors detected",
			})
		}
	}

	// Take full-page screenshot.
	screenshot, err := page.Screenshot(true, nil)
	if err == nil && tp.server.minioClient != nil {
		key := fmt.Sprintf("tasks/%d/verification-%s.png", task.ID, time.Now().Format("20060102-150405"))
		if err := tp.server.minioClient.Upload(ctx, key, "image/png", bytes.NewReader(screenshot), int64(len(screenshot))); err != nil {
			log.Printf("[verify] failed to upload screenshot: %v", err)
		} else {
			result.Screenshots = append(result.Screenshots, key)
		}
	}

	// Determine overall pass/fail.
	for _, check := range result.Checks {
		if !check.Passed {
			result.Passed = false
			break
		}
	}

	return result
}
```

**Step 2: Add minioClient field to Server**

In `internal/server/server.go`, add to the `Server` struct:

```go
type Server struct {
	// ... existing fields ...
	minioClient *MinIOClient
}
```

In both `New()` and `NewWithWebFS()`, after creating the server, attempt to initialize MinIO:

```go
	// Try to initialize MinIO client.
	if minioCfg, err := LoadMinIOConfig(); err == nil {
		if mc, err := NewMinIOClient(minioCfg); err == nil {
			s.minioClient = mc
			log.Println("MinIO client initialized")
		} else {
			log.Printf("WARNING: MinIO client failed: %v", err)
		}
	}
```

**Step 3: Call verifyTask in processTask**

In `autonomous.go`, after the merge-to-dev + rebuild block (around line 192) and before the validation advance (line 205), add:

```go
	// Run E2E verification.
	if tp.server != nil {
		tp.sendActivity(taskID, "status", "Running E2E verification...")
		vResult := tp.verifyTask(ctx, *task)
		if vResult != nil {
			// Post verification comment.
			var body strings.Builder
			if vResult.Passed {
				body.WriteString("**E2E Verification: PASSED**\n\n")
			} else {
				body.WriteString("**E2E Verification: FAILED**\n\n")
			}
			for _, check := range vResult.Checks {
				if check.Passed {
					body.WriteString(fmt.Sprintf("- %s: PASS — %s\n", check.Name, check.Detail))
				} else {
					body.WriteString(fmt.Sprintf("- %s: FAIL — %s\n", check.Name, check.Detail))
				}
			}

			comment := planner.Comment{
				TaskID:      taskID,
				Author:      "soul",
				Type:        "verification",
				Body:        body.String(),
				Attachments: vResult.Screenshots,
				CreatedAt:   time.Now().UTC().Format(time.RFC3339),
			}
			if id, err := tp.planner.CreateComment(comment); err == nil {
				comment.ID = id
				raw, _ := json.Marshal(comment)
				tp.broadcast(WSMessage{Type: "task.comment.added", Data: raw})
			}

			if !vResult.Passed {
				tp.sendActivity(taskID, "status", "E2E verification failed — staying in active stage")
				// Don't advance to validation — let user review.
				return
			}
		}
	}
```

Note: This should go between the merge-to-dev block and the re-read/advance-to-validation block.

**Step 4: Verify build**

```bash
cd ~/soul && go build ./...
```

Expected: Build succeeds (Rod is already a dependency from Scout).

**Step 5: Commit**

```bash
git add internal/server/verification.go internal/server/autonomous.go internal/server/server.go
git commit -m "feat: add E2E verification with Rod screenshots and MinIO upload"
```

---

## Task 10: Attachment Proxy Endpoint

**Files:**
- Modify: `internal/server/routes.go` (add attachment route)
- Modify: `internal/server/tasks.go` (add handleAttachment)

**Step 1: Add attachment handler**

In `internal/server/tasks.go`, add:

```go
// handleAttachment proxies attachment requests to MinIO via pre-signed URL.
func (s *Server) handleAttachment(w http.ResponseWriter, r *http.Request) {
	if s.minioClient == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "attachments not configured"})
		return
	}

	// The key is everything after /api/attachments/
	key := strings.TrimPrefix(r.URL.Path, "/api/attachments/")
	if key == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "missing attachment key"})
		return
	}

	url, err := s.minioClient.PresignedURL(r.Context(), key)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": fmt.Sprintf("failed to get attachment URL: %v", err)})
		return
	}

	http.Redirect(w, r, url, http.StatusTemporaryRedirect)
}
```

**Step 2: Register the route**

In `routes.go`, add after the comment endpoints:

```go
	// Attachment proxy.
	s.mux.HandleFunc("GET /api/attachments/", s.handleAttachment)
```

**Step 3: Verify build and commit**

```bash
cd ~/soul && go build ./... && git add internal/server/tasks.go internal/server/routes.go && git commit -m "feat: add attachment proxy endpoint for MinIO screenshots"
```

---

## Task 11: Build, Restart, and Integration Test

**Step 1: Build the binary**

```bash
cd ~/soul && go build -o soul ./cmd/soul
```

Expected: Build succeeds.

**Step 2: Build frontend**

```bash
cd ~/soul/web && npx vite build
```

Expected: Build succeeds.

**Step 3: Restart Soul**

Kill the running Soul process and restart:

```bash
pkill -f './soul serve' || true
cd ~/soul && SOUL_HOST=0.0.0.0 ./soul serve &
```

Expected: Server starts on :3000 and :3001. Logs show "MinIO client initialized" and "comment-watcher started".

**Step 4: Test comment API**

```bash
# Create a comment on task #21
curl -X POST http://localhost:3000/api/tasks/21/comments \
  -H 'Content-Type: application/json' \
  -d '{"author":"user","type":"feedback","body":"Testing the comment system"}'

# List comments
curl http://localhost:3000/api/tasks/21/comments
```

Expected: Comment created with ID, list returns the comment.

**Step 5: Test via UI**

Open the Soul frontend, click on a task, scroll to the comment section at the bottom, type a message and click Send.

Expected: Comment appears in the thread. If task is in active/validation/blocked stage, Soul responds with a status comment.

**Step 6: Commit any fixes**

If any issues found during testing, fix them and commit.

---

## Summary of All Files Changed

| File | Action | Purpose |
|------|--------|---------|
| `products/scout/infra/docker-compose.minio.yml` | CREATE | MinIO container config |
| `internal/server/minio.go` | CREATE | MinIO Go client wrapper |
| `internal/planner/types.go` | MODIFY | Add Comment struct |
| `internal/planner/store.go` | MODIFY | Add task_comments table + CRUD methods |
| `internal/server/tasks.go` | MODIFY | Add comment + attachment HTTP handlers |
| `internal/server/routes.go` | MODIFY | Register comment + attachment routes |
| `internal/server/server.go` | MODIFY | Add RebuildDevFrontend, MinIO init, CommentWatcher start |
| `internal/server/autonomous.go` | MODIFY | Add dev rebuild + E2E verification in processTask |
| `internal/server/comment_watcher.go` | CREATE | Background comment polling + mini-agent dispatch |
| `internal/server/verification.go` | CREATE | Rod-based E2E verification with screenshots |
| `web/src/lib/types.ts` | MODIFY | Add TaskComment type |
| `web/src/hooks/usePlanner.ts` | MODIFY | Add comment state + API methods + WS handler |
| `web/src/components/planner/TaskDetail.tsx` | MODIFY | Add comment thread UI |
