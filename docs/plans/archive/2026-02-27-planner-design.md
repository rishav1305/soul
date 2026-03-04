# Soul Planner — Design Document

**Date**: 2026-02-27
**Status**: Approved
**References**: soul-old/soul-planner, soul-os task system

## Overview

The Planner is Soul's core orchestration layer. Every action Soul performs — compliance scans, website updates, deployments — is a **task** on a Kanban board. Products (compliance, etc.) are capabilities that tasks invoke. The planner provides a visual task manager with autonomous AI processing.

## Architecture

**Core feature** in `internal/planner/`, not a separate product. The planner is Soul's backbone — products are plugins that tasks use.

### Layout

Two-panel vertical split:

- **Left: Soul Chat** — conversation interface (existing). Chat commands create/manage tasks.
- **Right: Task Manager** — two view modes:
  - **Kanban view**: columns by stage, draggable task cards as widgets
  - **Project view**: tasks grouped by project/product, list format with sorting/filtering

## Task Stages

6 Kanban columns with the following flow:

| Stage | Purpose | AI Behavior |
|-------|---------|-------------|
| **BACKLOG** | Every task lands here first | AI triages: known product → ACTIVE. Novel task → BRAINSTORM |
| **BRAINSTORM** | Plan before implementation | AI creates section → phase → step plan. Output saved to task `plan` field |
| **ACTIVE** | Work in progress | AI executes using product tools with 6-substep pipeline |
| **BLOCKED** | Human intervention needed | AI sets blocker reason. User must unblock |
| **VALIDATION** | Completed, awaiting review | User reviews output. Approve → DONE. Comment → back to ACTIVE |
| **DONE** | Completed (last 2 weeks) | Archive. Tasks older than 2 weeks auto-hidden |

### Stage Transitions

```
BACKLOG → BRAINSTORM    (novel tasks needing a plan)
BACKLOG → ACTIVE        (known product tasks, e.g. compliance scan)
BRAINSTORM → ACTIVE     (plan created, ready to execute)
ACTIVE → BLOCKED        (needs human input)
ACTIVE → VALIDATION     (work complete)
BLOCKED → ACTIVE        (user unblocks)
VALIDATION → DONE       (user approves)
VALIDATION → ACTIVE     (user rejects with comment/feedback)
```

### Substeps (within ACTIVE stage)

6-step TDD-driven pipeline:

| # | Substep | Purpose |
|---|---------|---------|
| 1 | **tdd** | Write tests first based on acceptance criteria |
| 2 | **implementing** | Build the solution using product tools |
| 3 | **reviewing** | AI self-reviews the implementation |
| 4 | **qa_test** | Run QA tests against the output |
| 5 | **e2e_test** | End-to-end testing |
| 6 | **security_review** | Security audit of changes |

Product tasks (like compliance scan) that don't involve code may skip directly to implementing.

## Data Model

SQLite database at `~/.soul/planner.db`. Schema ported from soul-old/soul-planner with additions.

```sql
CREATE TABLE tasks (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    title         TEXT NOT NULL,
    description   TEXT DEFAULT '',
    acceptance    TEXT,
    stage         TEXT DEFAULT 'backlog',
    substep       TEXT,
    priority      INTEGER DEFAULT 0,
    source        TEXT DEFAULT 'manual',
    blocker       TEXT,
    plan          TEXT,
    output        TEXT,
    error         TEXT,
    agent_id      TEXT,
    product       TEXT,
    parent_id     INTEGER REFERENCES tasks(id),
    metadata      TEXT,
    retry_count   INTEGER DEFAULT 0,
    max_retries   INTEGER DEFAULT 3,
    created_at    TEXT NOT NULL,
    started_at    TEXT,
    completed_at  TEXT
);

CREATE INDEX idx_tasks_stage ON tasks(stage);
CREATE INDEX idx_tasks_priority ON tasks(priority DESC);

CREATE TABLE task_dependencies (
    task_id    INTEGER REFERENCES tasks(id) ON DELETE CASCADE,
    depends_on INTEGER REFERENCES tasks(id) ON DELETE CASCADE,
    PRIMARY KEY (task_id, depends_on)
);
```

### Field descriptions

- `stage`: backlog | brainstorm | active | blocked | validation | done
- `substep`: tdd | implementing | reviewing | qa_test | e2e_test | security_review (NULL when not in ACTIVE)
- `priority`: integer, higher = more important (maps from daily blocks: BUILD=3, EXPLORE=2, SOCIAL=1, SCOUT=0)
- `source`: manual | chat | schedule
- `plan`: brainstorm output — section/phase/step plan text
- `output`: execution result / AI output
- `product`: which product capability to use (compliance, etc.) or NULL for general tasks
- `parent_id`: subtask hierarchy support
- `metadata`: JSON blob for extensibility (block number, tags, etc.)

## Backend

### Package structure

```
internal/planner/
    types.go       -- Task, Stage, Substep, Priority types
    store.go       -- SQLite CRUD (Open, Create, Update, List, Get, Delete, NextReady)
    processor.go   -- Autonomous task processing loop
```

### Store (`store.go`)

SQLite-backed CRUD with:
- `Open(dbPath)` — open/create database, run migrations
- `Create(task)` — insert task, return ID
- `Get(id)` — fetch single task
- `List(filters)` — list tasks with stage/priority/product filters
- `Update(id, fields)` — partial update
- `Delete(id)` — remove task
- `NextReady()` — highest-priority task in BACKLOG with no unresolved dependencies
- `AddDependency(taskID, dependsOn)` / `RemoveDependency(taskID, dependsOn)`

### Processor (`processor.go`)

Background goroutine for autonomous task processing:

- **Mode toggle**: user-triggered (default) or auto-process (enabled via UI toggle)
- **Tick loop**: when auto-process is on, polls `NextReady()` on configurable interval
- **Triage**: check if task has a known `product` → route to ACTIVE. Otherwise → BRAINSTORM
- **Brainstorm**: AI generates a section/phase/step plan, saves to `task.plan`, moves to ACTIVE
- **Execute**: AI processes task through 6 substeps using product tools
- **Complete**: moves task to VALIDATION for user review
- **Error handling**: on failure, set `error`, increment `retry_count`. If retries exhausted → BLOCKED

### REST endpoints

```
POST   /api/tasks          -- create task
GET    /api/tasks           -- list tasks (query: ?stage=&priority=&product=)
GET    /api/tasks/:id       -- get task detail
PATCH  /api/tasks/:id       -- update task fields
DELETE /api/tasks/:id       -- delete task
POST   /api/tasks/:id/move  -- move task to a different stage (with validation comment)
```

### WebSocket events (new)

```
task.created   {task}                -- new task on board
task.updated   {id, fields}         -- stage/substep/progress change
task.deleted   {id}                 -- task removed
task.output    {id, content}        -- AI produced output chunk
task.plan      {id, plan}           -- brainstorm plan generated
```

### Chat integration

When user says "scan ~/soul for compliance":
1. Agent creates task (title: "Compliance scan ~/soul", product: "compliance", stage: backlog)
2. Task appears on Kanban board via `task.created` WebSocket event
3. Agent moves task to ACTIVE (known product) via `task.updated`
4. Agent executes compliance__scan tool, streams progress
5. Task card shows live progress bar
6. On completion → task moves to VALIDATION

## Frontend

### Components

```
web/src/components/
    planner/
        KanbanBoard.tsx      -- 6-column Kanban layout
        ProjectView.tsx      -- list/table view grouped by project
        TaskCard.tsx         -- embeddable widget/block for each task
        TaskDetail.tsx       -- expanded detail overlay (modal or slide-over)
        NewTaskForm.tsx      -- create task form
        AutoToggle.tsx       -- autonomous processing toggle button
        StageColumn.tsx      -- single Kanban column
    layout/
        TwoPanel.tsx         -- vertical split: chat left, task manager right
```

### TaskCard widget

Each task renders as a card showing:
- Title + priority color indicator
- Stage badge (+ substep if ACTIVE, e.g. "ACTIVE [3/6] reviewing")
- Progress bar (if actively being processed)
- Product icon (compliance shield, etc.)
- Compact output/plan preview (truncated)
- Click to expand TaskDetail overlay

### TaskDetail overlay

Full task information:
- Title, description, acceptance criteria
- Plan (if brainstormed) — rendered as markdown sections
- Output / AI results
- Findings (if compliance product) — reuses existing FindingsTable
- Error message (if failed)
- Comments / feedback history
- Action buttons: Approve (→ DONE), Reject with comment (→ ACTIVE), Delete

### Hooks

```
web/src/hooks/
    usePlanner.ts    -- task CRUD via REST + WebSocket real-time updates
```

State: tasks array, auto-process toggle, active filters (stage, product).
Listens to `task.*` WebSocket events for real-time board updates.

## AI Integration

### Autonomous processing flow

```
[Auto mode ON]
    ↓
NextReady() → pick highest-priority unblocked BACKLOG task
    ↓
Has product? ─── YES → move to ACTIVE, skip brainstorm
    │
    NO
    ↓
Move to BRAINSTORM → AI generates plan → save to task.plan
    ↓
Move to ACTIVE
    ↓
Execute 6-substep pipeline:
  tdd → implementing → reviewing → qa_test → e2e_test → security_review
    ↓
Move to VALIDATION → wait for user review
    ↓
User approves → DONE
User rejects  → ACTIVE (with feedback in metadata)
```

### Agent tools (new)

The planner exposes tools to the AI agent:

- `planner__create` — create a new task
- `planner__list` — list tasks by stage/priority
- `planner__update` — update task fields
- `planner__move` — transition task to a new stage
- `planner__next` — get next ready task

These are registered as internal tools (not via product gRPC) since the planner is a core feature.

## Migration from soul-old

The schema is designed to be compatible with soul-old/soul-planner's SQLite data:
- `status` → `stage` (renamed, values remapped: in_progress → active, pending → backlog)
- `substep` values updated to the new 6-step pipeline
- `plan` field is new (soul-old stored plans in description or output)
- `product` field is new
- `task_dependencies` table preserved as-is
