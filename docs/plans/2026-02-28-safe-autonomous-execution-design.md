# Safe Autonomous Execution with Dev/Prod Separation

> **For implementation:** Use the writing-plans skill to create a task-by-task implementation plan from this design.

**Goal:** Make the Soul autonomous agent work safely — isolated branches per task, dev/prod separation, configurable workflows, and a manual validation gate before anything reaches production.

**Architecture:** Worktree-per-task isolation, dual server instances (prod on :3000, dev on :3001), configurable agent workflows (quick/full), and merge-to-master only when user moves story to Done.

**Tech Stack:** Go backend, git worktrees, React frontend (existing), SQLite planner store (existing)

---

## 1. Git Branch Architecture

```
master (prod — :3000)
  └── dev (integration — :3001)
        ├── task/15-replace-navbar-title
        ├── task/16-add-dark-mode
        └── task/17-fix-login-bug
```

- `master` = production. Only updated when user moves a story to **Done**.
- `dev` = integration branch. Task branches auto-merge here after agent finishes.
- `task/<id>-<slug>` = per-task branches, created in `.worktrees/` for filesystem isolation.

### Lifecycle

1. Task toggled autonomous → agent creates `task/<id>-<slug>` branch + worktree under `.worktrees/task-<id>/`
2. Agent works entirely within the worktree directory
3. Agent completes → commits to task branch → auto-merges to `dev` → dev server rebuilds
4. Task moves to **validation** stage
5. User evaluates on `:3001` (dev server)
6. User moves to **Done** → triggers merge to `master` → prod rebuilds → worktree cleaned up

## 2. Two Server Instances

| Server | Branch | Port | Rebuilt when |
|--------|--------|------|-------------|
| **prod** | `master` | `:3000` | Story moved to Done (merge to master) |
| **dev** | `dev` | `:3001` | Any task branch merges to dev |

Both serve frontend from disk (`web/dist/`) — not embedded. The dev instance uses its own build directory within the dev worktree/checkout.

### Server Management

Options (to be decided during implementation):
- **Option A:** Single Go binary manages both via goroutines — `soul serve --prod --dev`
- **Option B:** Two separate processes managed by systemd units
- **Option C:** Single process serves prod; dev is a separate checkout that auto-rebuilds

## 3. Configurable Workflow Modes

Task metadata field `"workflow"` controls the agent's execution steps.

### Quick Mode (default) — 5 steps

Best for: simple changes, UI tweaks, copy updates, small bug fixes.

1. **Search & understand** — `code_grep`, `code_search`, `code_read` to find relevant files
2. **Implement** — `code_edit`, `code_write` to make changes
3. **Build & verify** — `code_exec`: `vite build`, `go build`, run existing tests
4. **Commit** — commit to task branch with descriptive message
5. **Update task** — `task_update` to move to validation with summary

### Full Mode — 7 steps

Best for: complex features, security-sensitive changes, architectural work.

1. **Plan** — analyze the task, search codebase, document approach in task output
2. **Write tests** — if test files exist for the area, write failing tests first (TDD)
3. **Implement** — make the changes to pass tests / fulfill requirements
4. **Build & verify** — compile, run full test suite
5. **Security review** — check for common issues (SQL injection, hardcoded secrets, unsafe patterns)
6. **Commit** — commit with checkpoint message
7. **Update task** — `task_update` to move to validation with detailed summary

### Workflow Selection

The `buildTaskPrompt()` function reads `workflow` from task metadata and injects the appropriate step-by-step instructions into the system prompt. Default is `"quick"` if not specified.

Task metadata example:
```json
{"autonomous": true, "workflow": "full"}
```

## 4. WorktreeManager Component

New file: `internal/server/worktree.go`

```go
type WorktreeManager struct {
    repoRoot string // main repo root (e.g., /home/rishav/soul)
}

// Create creates a worktree + branch for a task.
// Branch: task/<id>-<slug>
// Path: .worktrees/task-<id>/
func (wm *WorktreeManager) Create(taskID int64, slug string) (projectRoot string, err error)

// MergeToDev merges the task branch into the dev branch.
// Triggers dev server rebuild after merge.
func (wm *WorktreeManager) MergeToDev(taskID int64) error

// MergeToMaster merges the task branch into master.
// Triggers prod server rebuild after merge.
func (wm *WorktreeManager) MergeToMaster(taskID int64) error

// Cleanup removes the worktree and optionally deletes the branch.
func (wm *WorktreeManager) Cleanup(taskID int64) error

// ProjectRoot returns the worktree path for a task.
func (wm *WorktreeManager) ProjectRoot(taskID int64) string
```

### Worktree Lifecycle

```
Create:
  git worktree add .worktrees/task-<id> -b task/<id>-<slug> dev

MergeToDev:
  git checkout dev
  git merge task/<id>-<slug> --no-ff -m "merge: task #<id> — <title>"
  cd .worktrees/dev/ && npx vite build  (or trigger dev server rebuild)

MergeToMaster:
  git checkout master
  git merge task/<id>-<slug> --no-ff -m "merge: task #<id> — <title>"
  npx vite build  (rebuild prod frontend)

Cleanup:
  git worktree remove .worktrees/task-<id>
  git branch -d task/<id>-<slug>
```

## 5. Updated Task Stage Flow

```
backlog → brainstorm → active → validation → done
                         ↓          ↓          ↓
                    agent creates  user       merge to master,
                    worktree,     evaluates   prod rebuild,
                    works in it   on :3001    worktree cleanup
```

### Stage Transitions (autonomous)

| From | To | Trigger | What happens |
|------|----|---------|-------------|
| any | active | autonomous toggle ON | WorktreeManager.Create(), agent starts in worktree |
| active | validation | agent completes | Commit to task branch, MergeToDev(), dev rebuild |
| active | blocked | agent can't complete | Commit partial work, task stays on branch |
| validation | done | user approves | MergeToMaster(), prod rebuild, Cleanup() |
| validation | active | user rejects | Agent retries with feedback in worktree |

## 6. Merge-to-Master Gate

The **only** path to master is: user moves task from validation → done.

In `handleTaskMove()`:
1. Detect transition to `done`
2. Call `WorktreeManager.MergeToMaster(taskID)`
3. Rebuild prod frontend (`vite build` in main repo)
4. Call `WorktreeManager.Cleanup(taskID)`
5. Broadcast task update

No autonomous code ever touches master directly.

## 7. Changes to Existing Code

### autonomous.go
- Use `WorktreeManager.Create()` to set up isolated workspace before starting agent
- Pass worktree path as `projectRoot` to `AgentLoop` (instead of main repo root)
- After agent completes: commit in worktree, call `MergeToDev()`
- Read `workflow` from task metadata to select prompt template

### agent.go
- No changes needed — already accepts `projectRoot` parameter
- Code tools already operate relative to projectRoot

### tasks.go
- `handleTaskMove` to `done`: trigger MergeToMaster + Cleanup
- `handleTaskMove` from `validation` back to `active`: agent restarts with feedback

### server.go
- Add `WorktreeManager` to Server struct
- Initialize on startup
- Ensure `dev` branch exists (create from master if not)
- Ensure `.worktrees/` directory exists and is gitignored

### codetools.go
- No changes needed — already uses projectRoot parameter

### Frontend (TaskDetail.tsx)
- Add workflow selector (quick/full) in task detail
- Show which branch/worktree the task is on
- Show dev server link when task is in validation

## 8. Initial Setup

On first run, the server should:
1. Check if `dev` branch exists; if not, create it from `master`
2. Create `.worktrees/` directory if it doesn't exist
3. Add `.worktrees/` to `.gitignore` if not already there
4. Start prod server on `:3000` from main repo
5. Optionally start dev server on `:3001` from dev checkout

## 9. Future Enhancements (not in scope now)

- Conflict resolution when two task branches touch the same file
- Automatic rollback if dev build fails after merge
- PR-style diff view in the Soul UI for reviewing changes
- Integration with Gitea for remote branch management
