# Safe Autonomous Execution — Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Make the autonomous agent work in isolated git worktrees per task, merge to a `dev` branch for evaluation, and only merge to `master` when the user moves the story to Done.

**Architecture:** WorktreeManager handles branch/worktree lifecycle. autonomous.go uses worktree paths instead of main repo root. tasks.go hooks merge-to-master on Done transition. Two server instances (prod :3000, dev :3001). Configurable quick/full workflow prompts.

**Tech Stack:** Go 1.22+, git worktrees, React/TypeScript (existing frontend), SQLite planner (existing)

---

## Task 1: WorktreeManager — Create and Cleanup

**Files:**
- Create: `internal/server/worktree.go`

**Step 1: Write `worktree.go` with WorktreeManager struct and helper methods**

```go
package server

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

// WorktreeManager manages git worktrees for isolated task execution.
type WorktreeManager struct {
	repoRoot string // main repo root (e.g., /home/rishav/soul)
}

// NewWorktreeManager creates a new WorktreeManager.
func NewWorktreeManager(repoRoot string) *WorktreeManager {
	return &WorktreeManager{repoRoot: repoRoot}
}

// slugify converts a task title to a URL-safe slug.
func slugify(title string) string {
	re := regexp.MustCompile(`[^a-z0-9]+`)
	slug := re.ReplaceAllString(strings.ToLower(title), "-")
	slug = strings.Trim(slug, "-")
	if len(slug) > 40 {
		slug = slug[:40]
	}
	return slug
}

// branchName returns the git branch name for a task.
func (wm *WorktreeManager) branchName(taskID int64, title string) string {
	return fmt.Sprintf("task/%d-%s", taskID, slugify(title))
}

// worktreePath returns the filesystem path for a task's worktree.
func (wm *WorktreeManager) worktreePath(taskID int64) string {
	return filepath.Join(wm.repoRoot, ".worktrees", fmt.Sprintf("task-%d", taskID))
}

// EnsureSetup creates .worktrees/ dir and ensures dev branch exists.
func (wm *WorktreeManager) EnsureSetup() error {
	// Create .worktrees directory.
	wtDir := filepath.Join(wm.repoRoot, ".worktrees")
	if err := os.MkdirAll(wtDir, 0o755); err != nil {
		return fmt.Errorf("create .worktrees: %w", err)
	}

	// Add to .gitignore if not already there.
	gitignorePath := filepath.Join(wm.repoRoot, ".gitignore")
	data, _ := os.ReadFile(gitignorePath)
	if !strings.Contains(string(data), ".worktrees") {
		f, err := os.OpenFile(gitignorePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
		if err != nil {
			return fmt.Errorf("open .gitignore: %w", err)
		}
		defer f.Close()
		f.WriteString("\n.worktrees/\n")
		log.Printf("[worktree] added .worktrees/ to .gitignore")
	}

	// Ensure dev branch exists (create from master if not).
	cmd := exec.Command("git", "rev-parse", "--verify", "dev")
	cmd.Dir = wm.repoRoot
	if err := cmd.Run(); err != nil {
		// dev branch doesn't exist — create it from master.
		cmd = exec.Command("git", "branch", "dev", "master")
		cmd.Dir = wm.repoRoot
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("create dev branch: %s — %w", out, err)
		}
		log.Printf("[worktree] created dev branch from master")
	}

	return nil
}

// Create creates a worktree + branch for a task. Returns the worktree path.
func (wm *WorktreeManager) Create(taskID int64, title string) (string, error) {
	branch := wm.branchName(taskID, title)
	wtPath := wm.worktreePath(taskID)

	// Remove stale worktree if it exists.
	if _, err := os.Stat(wtPath); err == nil {
		log.Printf("[worktree] removing stale worktree at %s", wtPath)
		cmd := exec.Command("git", "worktree", "remove", "--force", wtPath)
		cmd.Dir = wm.repoRoot
		cmd.CombinedOutput()
	}

	// Delete stale branch if it exists.
	cmd := exec.Command("git", "branch", "-D", branch)
	cmd.Dir = wm.repoRoot
	cmd.CombinedOutput() // ignore error if branch doesn't exist

	// Create worktree from dev branch.
	cmd = exec.Command("git", "worktree", "add", wtPath, "-b", branch, "dev")
	cmd.Dir = wm.repoRoot
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git worktree add: %s — %w", out, err)
	}

	log.Printf("[worktree] created %s (branch: %s)", wtPath, branch)
	return wtPath, nil
}

// ProjectRoot returns the worktree path for a task (may not exist yet).
func (wm *WorktreeManager) ProjectRoot(taskID int64) string {
	return wm.worktreePath(taskID)
}

// Cleanup removes a task's worktree and deletes its branch.
func (wm *WorktreeManager) Cleanup(taskID int64, title string) error {
	wtPath := wm.worktreePath(taskID)
	branch := wm.branchName(taskID, title)

	// Remove worktree.
	cmd := exec.Command("git", "worktree", "remove", "--force", wtPath)
	cmd.Dir = wm.repoRoot
	if out, err := cmd.CombinedOutput(); err != nil {
		log.Printf("[worktree] remove warning: %s — %v", out, err)
	}

	// Delete branch.
	cmd = exec.Command("git", "branch", "-D", branch)
	cmd.Dir = wm.repoRoot
	if out, err := cmd.CombinedOutput(); err != nil {
		log.Printf("[worktree] branch delete warning: %s — %v", out, err)
	}

	// Prune stale worktree entries.
	cmd = exec.Command("git", "worktree", "prune")
	cmd.Dir = wm.repoRoot
	cmd.CombinedOutput()

	log.Printf("[worktree] cleaned up task %d", taskID)
	return nil
}
```

**Step 2: Verify it compiles**

Run: `cd /home/rishav/soul && go build ./...`
Expected: no errors

**Step 3: Commit**

```bash
git add internal/server/worktree.go
git commit -m "feat: WorktreeManager — create/cleanup worktrees per task"
```

---

## Task 2: WorktreeManager — Merge Operations

**Files:**
- Modify: `internal/server/worktree.go` (append merge methods)

**Step 1: Add CommitInWorktree, MergeToDev, and MergeToMaster methods**

Append to `worktree.go`:

```go
// CommitInWorktree stages all changes in a worktree and commits them.
func (wm *WorktreeManager) CommitInWorktree(taskID int64, title string) error {
	wtPath := wm.worktreePath(taskID)

	// Stage all changes.
	cmd := exec.Command("git", "add", "-A")
	cmd.Dir = wtPath
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git add: %s — %w", out, err)
	}

	// Check if there's anything to commit.
	cmd = exec.Command("git", "diff", "--cached", "--quiet")
	cmd.Dir = wtPath
	if err := cmd.Run(); err == nil {
		log.Printf("[worktree] task %d: nothing to commit", taskID)
		return nil // nothing staged
	}

	// Commit.
	msg := fmt.Sprintf("task #%d: %s", taskID, title)
	cmd = exec.Command("git", "commit", "-m", msg)
	cmd.Dir = wtPath
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git commit: %s — %w", out, err)
	}

	log.Printf("[worktree] committed in task %d worktree", taskID)
	return nil
}

// MergeToDev merges a task branch into the dev branch.
func (wm *WorktreeManager) MergeToDev(taskID int64, title string) error {
	branch := wm.branchName(taskID, title)

	// Use a temporary worktree for dev to avoid disturbing the main checkout.
	// Alternatively, merge directly in the bare repo using plumbing.
	// Simplest approach: use git merge from the main repo.
	cmd := exec.Command("git", "checkout", "dev")
	cmd.Dir = wm.repoRoot
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("checkout dev: %s — %w", out, err)
	}

	cmd = exec.Command("git", "merge", branch, "--no-ff",
		"-m", fmt.Sprintf("merge: task #%d — %s", taskID, title))
	cmd.Dir = wm.repoRoot
	if out, err := cmd.CombinedOutput(); err != nil {
		// Switch back to master before returning error.
		exec.Command("git", "checkout", "master").Run()
		return fmt.Errorf("merge to dev: %s — %w", out, err)
	}

	// Switch back to master.
	cmd = exec.Command("git", "checkout", "master")
	cmd.Dir = wm.repoRoot
	cmd.CombinedOutput()

	log.Printf("[worktree] merged task %d to dev", taskID)
	return nil
}

// MergeToMaster merges a task branch into the master branch.
func (wm *WorktreeManager) MergeToMaster(taskID int64, title string) error {
	branch := wm.branchName(taskID, title)

	cmd := exec.Command("git", "merge", branch, "--no-ff",
		"-m", fmt.Sprintf("merge: task #%d — %s", taskID, title))
	cmd.Dir = wm.repoRoot
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("merge to master: %s — %w", out, err)
	}

	log.Printf("[worktree] merged task %d to master", taskID)
	return nil
}

// RebuildFrontend runs vite build in the given directory.
func (wm *WorktreeManager) RebuildFrontend(dir string) error {
	cmd := exec.Command("npx", "vite", "build")
	cmd.Dir = filepath.Join(dir, "web")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("vite build: %s — %w", out, err)
	}
	log.Printf("[worktree] frontend rebuilt in %s", dir)
	return nil
}
```

**Step 2: Verify it compiles**

Run: `cd /home/rishav/soul && go build ./...`
Expected: no errors

**Step 3: Commit**

```bash
git add internal/server/worktree.go
git commit -m "feat: WorktreeManager merge operations (dev + master)"
```

---

## Task 3: Wire WorktreeManager into Server + Startup

**Files:**
- Modify: `internal/server/server.go:27-41` (add worktrees field)
- Modify: `internal/server/server.go:45-62` (init in New)
- Modify: `internal/server/server.go:66-94` (init in NewWithWebFS)

**Step 1: Add `worktrees` field to Server struct**

In `server.go`, add to the Server struct (after `projectRoot`):

```go
worktrees   *WorktreeManager
```

**Step 2: Initialize WorktreeManager in both constructors**

In `New()`, after `projectRoot, _ := os.Getwd()`:

```go
wm := NewWorktreeManager(projectRoot)
if err := wm.EnsureSetup(); err != nil {
    log.Printf("WARNING: worktree setup failed: %v", err)
}
```

And set `s.worktrees = wm` in the struct literal.

Do the same in `NewWithWebFS()`.

**Step 3: Pass WorktreeManager to TaskProcessor**

Add `worktrees *WorktreeManager` field to `TaskProcessor` struct and update `NewTaskProcessor` signature.

In `server.go`, update both constructor calls:
```go
s.processor = NewTaskProcessor(aiClient, pm, sessions, plannerStore, s.broadcast, cfg.Model, projectRoot, wm)
```

**Step 4: Verify it compiles**

Run: `cd /home/rishav/soul && go build ./...`
Expected: no errors

**Step 5: Commit**

```bash
git add internal/server/server.go internal/server/autonomous.go
git commit -m "feat: wire WorktreeManager into server startup"
```

---

## Task 4: autonomous.go — Use Worktrees for Task Execution

**Files:**
- Modify: `internal/server/autonomous.go:19-44` (add worktrees to TaskProcessor)
- Modify: `internal/server/autonomous.go:81-188` (processTask to use worktrees)

**Step 1: Add `worktrees` field to TaskProcessor**

```go
type TaskProcessor struct {
	ai          *ai.Client
	products    *products.Manager
	sessions    *session.Store
	planner     *planner.Store
	broadcast   func(WSMessage)
	model       string
	projectRoot string
	worktrees   *WorktreeManager  // add this

	mu      sync.Mutex
	running map[int64]context.CancelFunc
}
```

Update `NewTaskProcessor` to accept and store it.

**Step 2: Update processTask to create worktree, work there, commit, merge to dev**

In `processTask`, after `tp.sendActivity(taskID, "stage", "active")`:

```go
// Create isolated worktree for this task.
var taskRoot string
if tp.worktrees != nil {
    tp.sendActivity(taskID, "status", "Creating isolated worktree...")
    var err error
    taskRoot, err = tp.worktrees.Create(taskID, task.Title)
    if err != nil {
        log.Printf("[autonomous] failed to create worktree for task %d: %v", taskID, err)
        tp.sendActivity(taskID, "status", fmt.Sprintf("Worktree failed: %v — falling back to main repo", err))
        taskRoot = tp.projectRoot
    } else {
        tp.sendActivity(taskID, "status", fmt.Sprintf("Working in isolated branch: %s", tp.worktrees.branchName(taskID, task.Title)))
    }
} else {
    taskRoot = tp.projectRoot
}
```

Then pass `taskRoot` (not `tp.projectRoot`) to `NewAgentLoop`:

```go
agent := NewAgentLoop(tp.ai, tp.products, tp.sessions, tp.planner, tp.broadcast, tp.model, taskRoot)
```

**Step 3: After agent completes, commit and merge to dev**

After `tp.planner.Update(taskID, planner.TaskUpdate{Output: &finalOutput})`, before the stage check:

```go
// Commit changes in the worktree and merge to dev.
if tp.worktrees != nil && taskRoot != tp.projectRoot {
    tp.sendActivity(taskID, "status", "Committing changes...")
    if err := tp.worktrees.CommitInWorktree(taskID, task.Title); err != nil {
        log.Printf("[autonomous] commit failed for task %d: %v", taskID, err)
        tp.sendActivity(taskID, "status", fmt.Sprintf("Commit warning: %v", err))
    }

    tp.sendActivity(taskID, "status", "Merging to dev branch...")
    if err := tp.worktrees.MergeToDev(taskID, task.Title); err != nil {
        log.Printf("[autonomous] merge to dev failed for task %d: %v", taskID, err)
        tp.sendActivity(taskID, "status", fmt.Sprintf("Merge to dev warning: %v", err))
    } else {
        tp.sendActivity(taskID, "status", "Changes merged to dev — visible on dev server")
    }
}
```

**Step 4: Remove commit instructions from agent prompt**

Since the Go code now handles committing (not the agent), remove step 4 ("Commit") from the `buildTaskPrompt` instructions. The agent should only use code tools and task_update — the outer processTask handles git.

**Step 5: Verify it compiles**

Run: `cd /home/rishav/soul && go build ./...`
Expected: no errors

**Step 6: Commit**

```bash
git add internal/server/autonomous.go
git commit -m "feat: autonomous agent works in isolated worktrees"
```

---

## Task 5: Merge-to-Master Gate on Done Transition

**Files:**
- Modify: `internal/server/tasks.go:204-265` (handleTaskMove)

**Step 1: Add merge-to-master logic in handleTaskMove**

After `s.planner.Update(id, update)` succeeds and before fetching the moved task, add:

```go
// Gate: merge to master when task moves to Done.
if body.Stage == planner.StageDone && s.worktrees != nil {
    log.Printf("[tasks] task %d moved to done — merging to master", id)

    if err := s.worktrees.MergeToMaster(id, task.Title); err != nil {
        log.Printf("[tasks] merge to master failed for task %d: %v", id, err)
        // Don't fail the move — log the error. User can retry.
    } else {
        // Rebuild prod frontend.
        if err := s.worktrees.RebuildFrontend(s.projectRoot); err != nil {
            log.Printf("[tasks] prod frontend rebuild failed: %v", err)
        }
    }

    // Cleanup the worktree.
    s.worktrees.Cleanup(id, task.Title)
}
```

**Step 2: Verify it compiles**

Run: `cd /home/rishav/soul && go build ./...`
Expected: no errors

**Step 3: Commit**

```bash
git add internal/server/tasks.go
git commit -m "feat: merge-to-master gate when task moves to Done"
```

---

## Task 6: Configurable Workflow Prompts (quick/full)

**Files:**
- Modify: `internal/server/autonomous.go:190-229` (buildTaskPrompt)

**Step 1: Read workflow mode from task metadata and select prompt**

Replace the `buildTaskPrompt` function:

```go
func (tp *TaskProcessor) buildTaskPrompt(task planner.Task) string {
	// Parse workflow mode from metadata (default: "quick").
	workflow := "quick"
	var meta map[string]any
	if task.Metadata != "" {
		if err := json.Unmarshal([]byte(task.Metadata), &meta); err == nil {
			if w, ok := meta["workflow"].(string); ok && (w == "quick" || w == "full") {
				workflow = w
			}
		}
	}

	var b strings.Builder
	fmt.Fprintf(&b, "You are autonomously working on task #%d.\n\n", task.ID)
	fmt.Fprintf(&b, "**Title:** %s\n", task.Title)
	if task.Description != "" {
		fmt.Fprintf(&b, "**Description:** %s\n", task.Description)
	}
	if task.Acceptance != "" {
		fmt.Fprintf(&b, "**Acceptance Criteria:** %s\n", task.Acceptance)
	}
	if task.Product != "" {
		fmt.Fprintf(&b, "**Product:** %s\n", task.Product)
	}

	// Project context.
	b.WriteString("\n## Project Context\n")
	fmt.Fprintf(&b, "Project root: `%s`\n", tp.projectRoot)
	b.WriteString("This is a Go + React/TypeScript monorepo:\n")
	b.WriteString("- `cmd/soul/` — Go entrypoint\n")
	b.WriteString("- `internal/server/` — Go HTTP server, WebSocket, agent loop, task APIs\n")
	b.WriteString("- `internal/planner/` — Task store (SQLite)\n")
	b.WriteString("- `internal/ai/` — Claude API client\n")
	b.WriteString("- `internal/session/` — Chat session memory\n")
	b.WriteString("- `web/src/` — React frontend (Vite + TypeScript + Tailwind)\n")
	b.WriteString("- `web/src/components/` — React components (chat/, layout/, planner/, panels/)\n")
	b.WriteString("- `web/src/hooks/` — Custom hooks (useChat, usePlanner, useWebSocket, etc.)\n")
	b.WriteString("- `web/src/lib/` — Types, WebSocket client, utilities\n")
	b.WriteString("- `products/` — Product plugins (compliance-go, etc.)\n")

	// Workflow-specific instructions.
	if workflow == "full" {
		b.WriteString("\n## Workflow: Full (7-step)\n")
		b.WriteString("Follow these steps in order:\n\n")
		b.WriteString("### Step 1: Plan\n")
		b.WriteString("Analyze the task. Search the codebase to understand the affected areas. Document your approach.\n\n")
		b.WriteString("### Step 2: Write Tests\n")
		b.WriteString("If test files exist for the affected area, write failing tests first (TDD). Use `code_exec` to run them and confirm they fail.\n\n")
		b.WriteString("### Step 3: Implement\n")
		b.WriteString("Make the minimal changes needed. Use `code_edit` for surgical changes, `code_write` only for new files.\n\n")
		b.WriteString("### Step 4: Build & Verify\n")
		b.WriteString("Run `go build ./...` for Go changes. Run `cd web && npx vite build` for frontend changes. Run tests with `code_exec`.\n\n")
		b.WriteString("### Step 5: Security Review\n")
		b.WriteString("Check your changes for: SQL injection (use parameterized queries), hardcoded secrets (use env vars), unsafe patterns.\n\n")
		b.WriteString("### Step 6: Summary\n")
		b.WriteString("Write a clear summary of what you changed and why in the task output.\n\n")
		b.WriteString("### Step 7: Update Task\n")
		b.WriteString("Use `task_update` to move the task to `validation` with your summary. If blocked, move to `blocked` with the reason.\n")
	} else {
		b.WriteString("\n## Workflow: Quick (5-step)\n")
		b.WriteString("Follow these steps in order:\n\n")
		b.WriteString("### Step 1: Search & Understand\n")
		b.WriteString("Use `code_search` and `code_grep` to find relevant files. Use `code_read` to understand the code.\n\n")
		b.WriteString("### Step 2: Implement\n")
		b.WriteString("Make the minimal changes needed. Use `code_edit` for modifications, `code_write` for new files.\n\n")
		b.WriteString("### Step 3: Build & Verify\n")
		b.WriteString("Run `go build ./...` for Go changes. Run `cd web && npx vite build` for frontend changes.\n\n")
		b.WriteString("### Step 4: Summary\n")
		b.WriteString("Write a clear summary of what you changed and why.\n\n")
		b.WriteString("### Step 5: Update Task\n")
		b.WriteString("Use `task_update` to move the task to `validation` with your summary. If blocked, move to `blocked` with the reason.\n")
	}

	b.WriteString("\n## Rules\n")
	b.WriteString("- Be precise. Make minimal changes. Do not refactor unrelated code.\n")
	b.WriteString("- Do NOT run git commands — the system handles commits and merges automatically.\n")
	b.WriteString("- All file paths are relative to the project root shown above.\n")

	return b.String()
}
```

**Step 2: Verify it compiles**

Run: `cd /home/rishav/soul && go build ./...`
Expected: no errors

**Step 3: Commit**

```bash
git add internal/server/autonomous.go
git commit -m "feat: configurable workflow prompts (quick/full)"
```

---

## Task 7: Frontend — Workflow Selector + Branch Info in TaskDetail

**Files:**
- Modify: `web/src/components/planner/TaskDetail.tsx:62-96` (add workflow toggle)
- Modify: `web/src/components/planner/TaskDetail.tsx:147-165` (show branch info)

**Step 1: Add workflow toggle next to autonomous toggle**

In TaskDetail, after the autonomous toggle div (around line 148), add a workflow selector that only shows when autonomous is enabled:

```tsx
{/* Workflow mode selector */}
{autonomous && (
  <div className="flex items-center gap-2 mt-1">
    <span className="text-xs text-fg-muted">Workflow:</span>
    {(['quick', 'full'] as const).map((mode) => (
      <button
        key={mode}
        type="button"
        onClick={async () => {
          const newMeta = { ...meta, workflow: mode };
          await onUpdate(task.id, { metadata: JSON.stringify(newMeta) });
        }}
        className={`px-2 py-0.5 rounded text-[10px] font-medium transition-colors cursor-pointer ${
          (meta.workflow || 'quick') === mode
            ? 'bg-soul/20 text-soul'
            : 'bg-elevated text-fg-muted hover:text-fg-secondary'
        }`}
      >
        {mode}
      </button>
    ))}
  </div>
)}
```

**Step 2: Show branch/worktree info when task is active or in validation**

In the header section, after the priority span, add:

```tsx
{/* Branch info for active/validation tasks */}
{(task.stage === 'active' || task.stage === 'validation') && task.agent_id?.startsWith('auto-') && (
  <span className="text-[10px] text-fg-muted font-mono">
    branch: task/{task.id}-...
  </span>
)}
```

**Step 3: Show dev server link when task is in validation**

In the body section, before the description, add:

```tsx
{task.stage === 'validation' && task.agent_id?.startsWith('auto-') && (
  <Section title="Review">
    <div className="flex items-center gap-2 text-sm">
      <span className="text-fg-secondary">Changes are live on the dev server:</span>
      <a
        href="http://localhost:3001"
        target="_blank"
        rel="noopener noreferrer"
        className="text-soul hover:underline font-mono text-xs"
      >
        localhost:3001
      </a>
    </div>
    <p className="text-[10px] text-fg-muted mt-1">
      Move to Done to merge to production (localhost:3000)
    </p>
  </Section>
)}
```

**Step 4: Build frontend**

Run: `cd /home/rishav/soul/web && npx vite build`
Expected: build succeeds

**Step 5: Commit**

```bash
git add web/src/components/planner/TaskDetail.tsx
git commit -m "feat: workflow selector + branch/dev-server info in task detail"
```

---

## Task 8: Dev Server Instance

**Files:**
- Modify: `internal/server/server.go` (add StartDevServer method)
- Modify: `cmd/soul/main.go` (start dev server alongside prod)

**Step 1: Add a method to start a dev server on a different port**

In `server.go`, add:

```go
// StartDevServer starts a second HTTP server on devPort, serving from the
// dev branch's web/dist/ directory. It shares the same planner/WS state.
func (s *Server) StartDevServer(devPort int) {
	devRoot := filepath.Join(s.projectRoot, ".worktrees", "dev-server")

	// Create a worktree for the dev branch to serve from.
	cmd := exec.Command("git", "worktree", "add", devRoot, "dev")
	cmd.Dir = s.projectRoot
	if out, err := cmd.CombinedOutput(); err != nil {
		// Might already exist — try to update it.
		cmd = exec.Command("git", "-C", devRoot, "checkout", "dev")
		if out2, err2 := cmd.CombinedOutput(); err2 != nil {
			log.Printf("[dev-server] failed to set up dev worktree: %s / %s", out, out2)
			return
		}
	}

	// Build frontend in dev worktree.
	cmd = exec.Command("npx", "vite", "build")
	cmd.Dir = filepath.Join(devRoot, "web")
	if out, err := cmd.CombinedOutput(); err != nil {
		log.Printf("[dev-server] frontend build failed: %s — %v", out, err)
		return
	}

	// Serve dev frontend from disk.
	devDist := filepath.Join(devRoot, "web", "dist")
	devMux := http.NewServeMux()
	devMux.Handle("/", newSPAFileServer(os.DirFS(devDist)))

	// Share API and WS routes with prod.
	devMux.HandleFunc("GET /api/health", handleHealth)
	devMux.HandleFunc("GET /api/tasks", s.handleTaskList)
	devMux.HandleFunc("GET /api/tasks/{id}", s.handleTaskGet)

	addr := net.JoinHostPort(s.cfg.Host, strconv.Itoa(devPort))
	fmt.Printf("◆ Soul dev server listening on %s\n", addr)
	go func() {
		if err := http.ListenAndServe(addr, devMux); err != nil {
			log.Printf("[dev-server] error: %v", err)
		}
	}()
}
```

**Step 2: Start dev server in main.go**

In `cmd/soul/main.go`, after `srv := server.NewWithWebFS(...)`, add:

```go
// Start dev server on port+1.
go srv.StartDevServer(cfg.Port + 1)
```

**Step 3: Add imports needed**

Add `"os/exec"`, `"path/filepath"` to server.go imports if not already present.

**Step 4: Verify it compiles**

Run: `cd /home/rishav/soul && go build ./...`
Expected: no errors

**Step 5: Commit**

```bash
git add internal/server/server.go cmd/soul/main.go
git commit -m "feat: dev server on :3001 serving from dev branch"
```

---

## Task 9: Integration Test — Full Cycle

**Step 1: Build and start the server**

```bash
cd /home/rishav/soul
cd web && npx vite build && cd ..
go build -o soul ./cmd/soul/
pkill -f "soul serve" 2>/dev/null
sleep 2
SOUL_HOST=0.0.0.0 nohup ./soul serve > /tmp/soul-test.log 2>&1 &
sleep 4
```

Verify in logs:
- `[worktree] created dev branch from master` (first run only)
- `[spa] serving frontend from disk`
- Both servers listening

**Step 2: Test the full autonomous cycle**

1. Create a test task via API with autonomous + quick workflow
2. Verify worktree gets created (check `.worktrees/` dir)
3. Monitor logs for agent using code tools in worktree
4. Verify task moves to validation
5. Check dev server (:3001) shows changes
6. Move task to Done via API
7. Verify merge to master happened
8. Check prod server (:3000) shows changes
9. Verify worktree cleaned up

**Step 3: Commit final state**

```bash
git add -A
git commit -m "feat: safe autonomous execution with dev/prod separation

Complete implementation:
- WorktreeManager for per-task branch/worktree isolation
- Agent works in worktree, commits auto-merge to dev branch
- Dev server on :3001 for evaluation before prod
- Merge to master only when user moves story to Done
- Configurable quick/full workflow prompts
- Frontend shows workflow selector, branch info, dev server link"
```
