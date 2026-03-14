# Projects Product Port — Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Port the Projects skill-building product from Soul v1 to v2 — 11 AI/ML projects with pre-written guides, milestones, keywords, readiness tracking, and platform syncs.

**Architecture:** Standalone Go server on port 3008 with SQLite, chat server reverse proxy at `/api/projects/*`, React frontend with 2 routes. Follows the exact same patterns as the Tutor port (separate process, DB, systemd service).

**Tech Stack:** Go 1.24+ (standard library + modernc.org/sqlite), React 19, TypeScript 5.9, Vite 7, Tailwind CSS v4

**Spec:** `docs/superpowers/specs/2026-03-14-projects-port-design.md`

---

## File Structure

### New Files

| File | Responsibility |
|------|---------------|
| `internal/projects/store/store.go` | SQLite schema (7 tables), 15 types, ~30 CRUD methods |
| `internal/projects/store/seed.go` | Auto-seed 11 projects + milestones + keywords + syncs (idempotent) |
| `internal/projects/store/store_test.go` | Store unit tests |
| `internal/projects/content/embed.go` | `go:embed` directive for guide files |
| `internal/projects/content/rag-pipeline/guide.md` | RAG pipeline implementation guide |
| `internal/projects/content/fine-tuning/guide.md` | Fine-tuning implementation guide |
| `internal/projects/content/llm-evaluation/guide.md` | LLM evaluation implementation guide |
| `internal/projects/content/mlops-pipeline/guide.md` | MLOps pipeline implementation guide |
| `internal/projects/content/model-serving/guide.md` | Model serving implementation guide |
| `internal/projects/content/data-quality/guide.md` | Data quality implementation guide |
| `internal/projects/content/agent-framework/guide.md` | Agent framework implementation guide |
| `internal/projects/content/knowledge-graph/guide.md` | Knowledge graph implementation guide |
| `internal/projects/content/multimodal-ai/guide.md` | Multimodal AI implementation guide |
| `internal/projects/content/streaming-ai/guide.md` | Streaming AI implementation guide |
| `internal/projects/content/ai-safety/guide.md` | AI safety implementation guide |
| `internal/projects/server/server.go` | HTTP server, routes, middleware, handlers |
| `cmd/projects/main.go` | Server binary with `serve` subcommand |
| `internal/chat/server/projects_tools.go` | Static chat tool definitions |
| `deploy/soul-v2-projects.service` | Systemd unit file |
| `web/src/pages/ProjectsPage.tsx` | 4-tab page (Dashboard, Projects, Timeline, Keywords) |
| `web/src/pages/ProjectDetailPage.tsx` | Single project (Milestones, Guide, Readiness, Metrics) |
| `web/src/hooks/useProjects.ts` | Dashboard/list data fetching hook |
| `web/src/hooks/useProjectDetail.ts` | Single project + guide hook |
| `web/src/components/ProjectCard.tsx` | Project summary card component |

### Modified Files

| File | Change |
|------|--------|
| `internal/chat/server/proxy.go` | Add `projectsProxy` struct + `WithProjectsProxy()` |
| `internal/chat/server/server.go` | Register `/api/projects/` proxy routes |
| `cmd/chat/main.go` | Wire `WithProjectsProxy()` |
| `web/src/router.tsx` | Add `/projects`, `/projects/:id` routes |
| `web/src/layouts/AppLayout.tsx` | Add Projects NavLink |
| `web/src/lib/types.ts` | Add Projects type definitions |
| `Makefile` | Add `build-projects`, update `build`/`clean`/`serve` |
| `CLAUDE.md` | Update architecture section |

---

## Chunk 1: Backend — Store, Seed, Tests

### Task 1: Projects Store — Schema, Types, and CRUD

**Files:**
- Create: `internal/projects/store/store.go`

- [ ] **Step 1: Create store file with schema and types**

Create `internal/projects/store/store.go` with:

1. **Package and imports:**
```go
package store

import (
    "database/sql"
    "fmt"
    "log"
    "time"

    _ "modernc.org/sqlite"
)
```

2. **Store struct and Open function:**
```go
type Store struct {
    db *sql.DB
}

func Open(dbPath string) (*Store, error) {
    db, err := sql.Open("sqlite", dbPath+"?_busy_timeout=5000")
    if err != nil {
        return nil, fmt.Errorf("open db: %w", err)
    }
    if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
        db.Close()
        return nil, fmt.Errorf("set WAL: %w", err)
    }
    if _, err := db.Exec("PRAGMA foreign_keys=ON"); err != nil {
        db.Close()
        return nil, fmt.Errorf("enable FK: %w", err)
    }
    s := &Store{db: db}
    if err := s.migrate(); err != nil {
        db.Close()
        return nil, fmt.Errorf("migrate: %w", err)
    }
    return s, nil
}

func (s *Store) Close() error { return s.db.Close() }
```

3. **Schema (7 tables) in migrate():**
```go
func (s *Store) migrate() error {
    _, err := s.db.Exec(`
    CREATE TABLE IF NOT EXISTS projects (
        id INTEGER PRIMARY KEY AUTOINCREMENT,
        name TEXT UNIQUE NOT NULL,
        description TEXT NOT NULL DEFAULT '',
        phase INTEGER NOT NULL DEFAULT 1,
        status TEXT NOT NULL DEFAULT 'backlog',
        week_planned INTEGER NOT NULL DEFAULT 0,
        hours_estimated REAL NOT NULL DEFAULT 0,
        hours_actual REAL NOT NULL DEFAULT 0,
        github_repo TEXT NOT NULL DEFAULT '',
        readme_url TEXT NOT NULL DEFAULT '',
        created_at TEXT NOT NULL DEFAULT (datetime('now')),
        updated_at TEXT NOT NULL DEFAULT (datetime('now'))
    );
    CREATE TABLE IF NOT EXISTS milestones (
        id INTEGER PRIMARY KEY AUTOINCREMENT,
        project_id INTEGER NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
        name TEXT NOT NULL,
        description TEXT NOT NULL DEFAULT '',
        acceptance_criteria TEXT NOT NULL DEFAULT '',
        status TEXT NOT NULL DEFAULT 'pending',
        completed_at TEXT,
        sort_order INTEGER NOT NULL DEFAULT 0
    );
    CREATE INDEX IF NOT EXISTS idx_milestones_project ON milestones(project_id);
    CREATE INDEX IF NOT EXISTS idx_milestones_status ON milestones(status);

    CREATE TABLE IF NOT EXISTS metrics (
        id INTEGER PRIMARY KEY AUTOINCREMENT,
        project_id INTEGER NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
        name TEXT NOT NULL,
        value TEXT NOT NULL,
        unit TEXT NOT NULL DEFAULT '',
        captured_at TEXT NOT NULL DEFAULT (datetime('now'))
    );
    CREATE INDEX IF NOT EXISTS idx_metrics_project ON metrics(project_id);

    CREATE TABLE IF NOT EXISTS keywords (
        id INTEGER PRIMARY KEY AUTOINCREMENT,
        project_id INTEGER REFERENCES projects(id) ON DELETE SET NULL,
        keyword TEXT UNIQUE NOT NULL,
        status TEXT NOT NULL DEFAULT 'claimed',
        claimed_at TEXT NOT NULL DEFAULT (datetime('now')),
        shipped_at TEXT
    );
    CREATE INDEX IF NOT EXISTS idx_keywords_status ON keywords(status);

    CREATE TABLE IF NOT EXISTS profile_syncs (
        id INTEGER PRIMARY KEY AUTOINCREMENT,
        project_id INTEGER NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
        platform TEXT NOT NULL,
        synced INTEGER NOT NULL DEFAULT 0,
        synced_at TEXT,
        notes TEXT NOT NULL DEFAULT '',
        created_at TEXT NOT NULL DEFAULT (datetime('now'))
    );
    CREATE UNIQUE INDEX IF NOT EXISTS idx_profile_syncs_unique ON profile_syncs(project_id, platform);

    CREATE TABLE IF NOT EXISTS interview_readiness (
        id INTEGER PRIMARY KEY AUTOINCREMENT,
        project_id INTEGER NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
        can_explain INTEGER NOT NULL DEFAULT 0,
        can_demo INTEGER NOT NULL DEFAULT 0,
        can_tradeoffs INTEGER NOT NULL DEFAULT 0,
        self_score INTEGER NOT NULL DEFAULT 0,
        assessed_at TEXT NOT NULL DEFAULT (datetime('now'))
    );
    CREATE INDEX IF NOT EXISTS idx_readiness_project ON interview_readiness(project_id);

    CREATE TABLE IF NOT EXISTS daily_activity (
        id INTEGER PRIMARY KEY AUTOINCREMENT,
        date TEXT NOT NULL,
        time_spent_seconds INTEGER NOT NULL DEFAULT 0,
        projects_worked INTEGER NOT NULL DEFAULT 0,
        milestones_completed INTEGER NOT NULL DEFAULT 0
    );
    CREATE INDEX IF NOT EXISTS idx_activity_date ON daily_activity(date);
    `)
    return err
}
```

4. **Types (15 structs):**
```go
type Project struct {
    ID             int     `json:"id"`
    Name           string  `json:"name"`
    Description    string  `json:"description"`
    Phase          int     `json:"phase"`
    Status         string  `json:"status"`
    WeekPlanned    int     `json:"week_planned"`
    HoursEstimated float64 `json:"hours_estimated"`
    HoursActual    float64 `json:"hours_actual"`
    GithubRepo     string  `json:"github_repo"`
    ReadmeURL      string  `json:"readme_url"`
    CreatedAt      string  `json:"created_at"`
    UpdatedAt      string  `json:"updated_at"`
}

type ProjectSummary struct {
    Project
    MilestonesDone  int `json:"milestones_done"`
    MilestonesTotal int `json:"milestones_total"`
    KeywordCount    int `json:"keyword_count"`
}

type ProjectUpdate struct {
    Status     *string  `json:"status,omitempty"`
    HoursActual *float64 `json:"hours_actual,omitempty"`
    GithubRepo *string  `json:"github_repo,omitempty"`
    ReadmeURL  *string  `json:"readme_url,omitempty"`
}

type Milestone struct {
    ID                 int    `json:"id"`
    ProjectID          int    `json:"project_id"`
    Name               string `json:"name"`
    Description        string `json:"description"`
    AcceptanceCriteria string `json:"acceptance_criteria"`
    Status             string `json:"status"`
    CompletedAt        string `json:"completed_at,omitempty"`
    SortOrder          int    `json:"sort_order"`
}

type Metric struct {
    ID         int    `json:"id"`
    ProjectID  int    `json:"project_id"`
    Name       string `json:"name"`
    Value      string `json:"value"`
    Unit       string `json:"unit"`
    CapturedAt string `json:"captured_at"`
}

type Keyword struct {
    ID        int    `json:"id"`
    ProjectID *int   `json:"project_id,omitempty"`
    Keyword   string `json:"keyword"`
    Status    string `json:"status"`
    ClaimedAt string `json:"claimed_at"`
    ShippedAt string `json:"shipped_at,omitempty"`
}

type ProfileSync struct {
    ID        int    `json:"id"`
    ProjectID int    `json:"project_id"`
    Platform  string `json:"platform"`
    Synced    bool   `json:"synced"`
    SyncedAt  string `json:"synced_at,omitempty"`
    Notes     string `json:"notes"`
}

type Readiness struct {
    ID           int    `json:"id"`
    ProjectID    int    `json:"project_id"`
    CanExplain   bool   `json:"can_explain"`
    CanDemo      bool   `json:"can_demo"`
    CanTradeoffs bool   `json:"can_tradeoffs"`
    SelfScore    int    `json:"self_score"`
    AssessedAt   string `json:"assessed_at"`
}

type DailyActivity struct {
    ID                  int    `json:"id"`
    Date                string `json:"date"`
    TimeSpentSeconds    int    `json:"time_spent_seconds"`
    ProjectsWorked      int    `json:"projects_worked"`
    MilestonesCompleted int    `json:"milestones_completed"`
}

type Dashboard struct {
    TotalProjects   int              `json:"total_projects"`
    Shipped         int              `json:"shipped"`
    Active          int              `json:"active"`
    Backlog         int              `json:"backlog"`
    Measuring       int              `json:"measuring"`
    Documenting     int              `json:"documenting"`
    KeywordsTotal   int              `json:"keywords_total"`
    KeywordsClaimed int              `json:"keywords_claimed"`
    KeywordsBuilding int             `json:"keywords_building"`
    KeywordsShipped int              `json:"keywords_shipped"`
    HoursEstimated  float64          `json:"hours_estimated"`
    HoursActual     float64          `json:"hours_actual"`
    AvgReadiness    float64          `json:"avg_readiness"`
    Projects        []ProjectSummary `json:"projects"`
}

type ProjectDetail struct {
    Project
    Milestones []Milestone   `json:"milestones"`
    Metrics    []Metric      `json:"metrics"`
    Keywords   []Keyword     `json:"keywords"`
    Syncs      []ProfileSync `json:"syncs"`
    Readiness  *Readiness    `json:"readiness"`
}
```

5. **CRUD methods (~30):**

**Projects:**
```go
func (s *Store) CreateProject(name, description string, phase, weekPlanned int, hoursEstimated float64) (int64, error)
func (s *Store) GetProject(id int) (Project, error)
func (s *Store) GetProjectByName(name string) (Project, error)
func (s *Store) ListProjects() ([]ProjectSummary, error)  // JOIN milestones + keywords for counts
func (s *Store) UpdateProject(id int, u ProjectUpdate) error  // dynamic SET clause
func (s *Store) ProjectCount() (int, error)
```

`ListProjects` query:
```sql
SELECT p.*,
    COALESCE(SUM(CASE WHEN m.status = 'done' THEN 1 ELSE 0 END), 0) as milestones_done,
    COUNT(m.id) as milestones_total,
    (SELECT COUNT(*) FROM keywords WHERE project_id = p.id) as keyword_count
FROM projects p
LEFT JOIN milestones m ON m.project_id = p.id
GROUP BY p.id
ORDER BY p.phase, p.week_planned
```

`UpdateProject` builds dynamic SET clause from non-nil fields:
```go
func (s *Store) UpdateProject(id int, u ProjectUpdate) error {
    sets := []string{"updated_at = datetime('now')"}
    args := []any{}
    if u.Status != nil {
        sets = append(sets, "status = ?")
        args = append(args, *u.Status)
    }
    if u.HoursActual != nil {
        sets = append(sets, "hours_actual = ?")
        args = append(args, *u.HoursActual)
    }
    if u.GithubRepo != nil {
        sets = append(sets, "github_repo = ?")
        args = append(args, *u.GithubRepo)
    }
    if u.ReadmeURL != nil {
        sets = append(sets, "readme_url = ?")
        args = append(args, *u.ReadmeURL)
    }
    args = append(args, id)
    query := fmt.Sprintf("UPDATE projects SET %s WHERE id = ?", strings.Join(sets, ", "))
    _, err := s.db.Exec(query, args...)
    return err
}
```

**Milestones:**
```go
func (s *Store) CreateMilestone(projectID int, name, desc, ac string, sortOrder int) (int64, error)
func (s *Store) ListMilestones(projectID int) ([]Milestone, error)  // ORDER BY sort_order
func (s *Store) UpdateMilestoneStatus(id int, status string) error  // sets completed_at for "done"
```

**Metrics:**
```go
func (s *Store) RecordMetric(projectID int, name, value, unit string) (int64, error)
func (s *Store) ListMetrics(projectID int) ([]Metric, error)  // ORDER BY captured_at DESC
```

**Keywords:**
```go
func (s *Store) CreateKeyword(projectID *int, keyword string) (int64, error)  // INSERT OR IGNORE
func (s *Store) ListKeywords() ([]Keyword, error)  // ORDER BY status, keyword
func (s *Store) ListProjectKeywords(projectID int) ([]Keyword, error)
func (s *Store) UpdateKeywordStatus(id int, status string) error  // sets shipped_at for "shipped"
```

**Profile Syncs:**
```go
func (s *Store) CreateProfileSync(projectID int, platform string) (int64, error)
func (s *Store) ListProfileSyncs(projectID int) ([]ProfileSync, error)
func (s *Store) UpdateProfileSync(projectID int, platform, notes string) error  // sets synced=1, synced_at
```

**Interview Readiness:**
```go
func (s *Store) RecordReadiness(projectID int, canExplain, canDemo, canTradeoffs bool, selfScore int) (int64, error)
func (s *Store) GetReadiness(projectID int) (*Readiness, error)  // latest by assessed_at DESC
```

**Daily Activity:**
```go
func (s *Store) UpsertDailyActivity(date string, timeSpent, projectsWorked, milestonesCompleted int) error
func (s *Store) GetActivity(days int) ([]DailyActivity, error)  // last N days
```

**Dashboard (aggregation):**
```go
func (s *Store) GetDashboard() (Dashboard, error)
```

Dashboard query combines: project status counts, keyword status counts, SUM(hours), AVG readiness self_score (latest per project), and the full project list with milestone/keyword counts. Uses a single transaction with multiple queries for consistency.

- [ ] **Step 2: Verify compilation**

Run: `go build ./internal/projects/store/`
Expected: PASS (no errors)

- [ ] **Step 3: Commit**

```bash
git add internal/projects/store/store.go
git commit -m "feat(projects): add store with schema, types, and CRUD methods"
```

---

### Task 2: Seed Data — 11 Projects with Milestones, Keywords, and Syncs

**Files:**
- Create: `internal/projects/store/seed.go`

- [ ] **Step 1: Create seed file**

Create `internal/projects/store/seed.go` with:

1. **Idempotent check:** `ProjectCount() > 0 → skip`
2. **Atomic transaction:** All inserts in one tx
3. **11 projects** with phase, week_planned, hours_estimated (matching spec)
4. **Per-project milestones** (5-7 each, with name, description, acceptance_criteria, sort_order)
5. **Per-project keywords** (7-10 each) + 5 pre-shipped standalone keywords
6. **Per-project platform syncs** (7 platforms: linkedin, naukri, indeed, wellfound, instahyre, portfolio, github)

Structure:
```go
func (s *Store) Seed() error {
    count, err := s.ProjectCount()
    if err != nil { return err }
    if count > 0 {
        log.Printf("[projects] seed: %d projects exist, skipping", count)
        return nil
    }

    tx, err := s.db.Begin()
    if err != nil { return err }
    defer tx.Rollback()

    projects := []seedProject{
        {Name: "rag-pipeline", Description: "Build a production-grade RAG pipeline...", Phase: 1, Week: 1, Hours: 22.5,
            Milestones: []seedMilestone{
                {Name: "Document ingestion pipeline", Desc: "Parse PDF/markdown into chunks with metadata", AC: "Process 100+ documents without error"},
                // ... all 6 milestones
            },
            Keywords: []string{"RAG", "Vector Database", "Embeddings", "FAISS", "ChromaDB", "Semantic Search", "Document Chunking"},
        },
        // ... all 11 projects with their milestones and keywords from spec
    }

    platforms := []string{"linkedin", "naukri", "indeed", "wellfound", "instahyre", "portfolio", "github"}

    for _, p := range projects {
        // INSERT project → get ID
        // INSERT milestones with sort_order
        // INSERT keywords with project_id
        // INSERT 7 profile_syncs
    }

    // 5 pre-shipped standalone keywords (no project_id)
    preShipped := []string{"Prompt Engineering", "Multi-Agent Systems", "Agentic AI", "LLM Orchestration", "Production AI"}
    for _, kw := range preShipped {
        tx.Exec("INSERT OR IGNORE INTO keywords (keyword, status, shipped_at) VALUES (?, 'shipped', datetime('now'))", kw)
    }

    return tx.Commit()
}
```

All 11 projects' milestones must match the spec exactly (project names, milestone names, descriptions, acceptance criteria). All 11 projects' keywords must match the spec exactly.

- [ ] **Step 2: Verify compilation**

Run: `go build ./internal/projects/store/`
Expected: PASS

- [ ] **Step 3: Commit**

```bash
git add internal/projects/store/seed.go
git commit -m "feat(projects): add seed data — 11 projects with milestones, keywords, syncs"
```

---

### Task 3: Store Tests

**Files:**
- Create: `internal/projects/store/store_test.go`

- [ ] **Step 1: Write store tests**

Test helper:
```go
func openTestStore(t *testing.T) *Store {
    t.Helper()
    dir := t.TempDir()
    s, err := Open(filepath.Join(dir, "test.db"))
    if err != nil { t.Fatal(err) }
    t.Cleanup(func() { s.Close() })
    return s
}
```

Tests to write (following Tutor's pattern — no mocks, real SQLite):

1. **TestProjectCRUD** — Create → Get → GetByName → List → Update → verify all fields
2. **TestProjectUniqueName** — Create duplicate name → expect error
3. **TestMilestoneCRUD** — Create milestones → List (verify sort_order) → UpdateStatus to "done" (verify completed_at set) → UpdateStatus to "skipped"
4. **TestMetricCRUD** — RecordMetric → ListMetrics → verify ORDER BY captured_at DESC
5. **TestKeywordCRUD** — CreateKeyword (with project) → CreateKeyword (standalone) → ListKeywords → ListProjectKeywords → UpdateKeywordStatus to "shipped" (verify shipped_at)
6. **TestKeywordUnique** — INSERT OR IGNORE on duplicate keyword
7. **TestProfileSyncCRUD** — CreateProfileSync → ListProfileSyncs → UpdateProfileSync (verify synced=true, synced_at set)
8. **TestProfileSyncUnique** — Unique constraint on (project_id, platform)
9. **TestReadinessCRUD** — RecordReadiness → GetReadiness → RecordReadiness again → GetReadiness returns latest
10. **TestDashboard** — Seed → GetDashboard → verify counts (total, by status, keywords, hours, readiness avg)
11. **TestSeed** — Seed on empty DB → verify 11 projects, milestone counts, keyword counts, profile_sync counts. Seed again → idempotent (still 11)
12. **TestCascadeDelete** — Delete project → milestones, metrics, syncs, readiness cascade deleted. Keywords set project_id to NULL.

- [ ] **Step 2: Run tests**

Run: `go test ./internal/projects/store/ -v -count=1`
Expected: All PASS

- [ ] **Step 3: Commit**

```bash
git add internal/projects/store/store_test.go
git commit -m "test(projects): add store tests — CRUD, seed, cascade, dashboard"
```

---

### Task 4: Guide Content — 11 Implementation Guides

**Files:**
- Create: `internal/projects/content/embed.go`
- Create: `internal/projects/content/{name}/guide.md` (11 files)

- [ ] **Step 1: Create embed.go**

```go
package content

import "embed"

//go:embed **/guide.md
var Guides embed.FS
```

- [ ] **Step 2: Create all 11 guide files**

Each guide follows this structure (from spec):
- **Overview** — What this project builds and why it matters
- **Architecture** — System design, component diagram, data flow
- **Key Concepts** — Theory and background needed
- **Implementation Steps** — Detailed walkthrough with code snippets and library recommendations
- **Testing & Evaluation** — How to verify correctness and measure quality
- **Interview Angles** — Common interview questions about this topic, how to discuss tradeoffs

Create guides for all 11 projects. Each guide should be 200-400 lines of substantive content with:
- Real code snippets (Python, since these are ML projects)
- Specific library recommendations with versions
- Architecture diagrams (ASCII)
- Common pitfalls and how to avoid them
- Interview Q&A examples

The 11 guide files:
1. `internal/projects/content/rag-pipeline/guide.md`
2. `internal/projects/content/fine-tuning/guide.md`
3. `internal/projects/content/llm-evaluation/guide.md`
4. `internal/projects/content/mlops-pipeline/guide.md`
5. `internal/projects/content/model-serving/guide.md`
6. `internal/projects/content/data-quality/guide.md`
7. `internal/projects/content/agent-framework/guide.md`
8. `internal/projects/content/knowledge-graph/guide.md`
9. `internal/projects/content/multimodal-ai/guide.md`
10. `internal/projects/content/streaming-ai/guide.md`
11. `internal/projects/content/ai-safety/guide.md`

- [ ] **Step 3: Verify embed compiles**

Run: `go build ./internal/projects/content/`
Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add internal/projects/content/
git commit -m "feat(projects): add 11 implementation guide files with go:embed"
```

---

## Chunk 2: Backend — HTTP Server, Binary, Chat Integration

### Task 5: HTTP Server — Routes and Handlers

**Files:**
- Create: `internal/projects/server/server.go`

**Reference:** `internal/tutor/server/server.go` (507 lines, same patterns)

- [ ] **Step 1: Create server file**

Structure:
```go
package server

type Server struct {
    store      *store.Store
    mux        *http.ServeMux
    httpServer *http.Server
    host       string
    port       string
    contentDir string
    startTime  time.Time
    logger     *log.Logger
}

// Functional options
type Option func(*Server)
func WithStore(s *store.Store) Option
func WithHost(h string) Option
func WithPort(p string) Option
func WithContentDir(d string) Option
```

**Routes (14):**
```go
func (s *Server) registerRoutes() {
    // Health
    s.mux.HandleFunc("GET /api/health", s.handleHealth)

    // Dashboard & lists
    s.mux.HandleFunc("GET /api/projects/dashboard", s.handleDashboard)
    s.mux.HandleFunc("GET /api/projects/keywords", s.handleKeywords)
    s.mux.HandleFunc("GET /api/projects/{id}", s.handleGetProject)

    // Updates
    s.mux.HandleFunc("PATCH /api/projects/{id}", s.handleUpdateProject)
    s.mux.HandleFunc("PATCH /api/projects/{id}/milestones/{mid}", s.handleUpdateMilestone)
    s.mux.HandleFunc("POST /api/projects/{id}/metrics", s.handleRecordMetric)
    s.mux.HandleFunc("POST /api/projects/{id}/syncs", s.handleSyncPlatform)
    s.mux.HandleFunc("POST /api/projects/{id}/readiness", s.handleRecordReadiness)

    // Content
    s.mux.HandleFunc("GET /api/projects/{id}/guide", s.handleGetGuide)

    // Chat tool execution
    s.mux.HandleFunc("POST /api/tools/{name}/execute", s.handleToolExecute)
}
```

**Middleware stack** (same as Tutor):
```go
func (s *Server) handler() http.Handler {
    return s.recoveryMiddleware(
        s.cspMiddleware(
            s.bodyLimitMiddleware(64<<10,  // 64KB
                s.requestLogMiddleware(s.mux))))
}
```

**Key handlers:**

`handleDashboard` — calls `s.store.GetDashboard()`, writes JSON.

`handleGetProject` — parses `{id}`, calls `s.store.GetProject()`, then fetches milestones, metrics, keywords, syncs, readiness, assembles `ProjectDetail`, writes JSON.

`handleGetGuide` — parses `{id}`, gets project name, reads guide from `contentDir/{name}/guide.md` with `io.LimitReader(1MB)`. Falls back to embedded content if disk file missing. Returns `{"content": "..."}`.

`handleUpdateProject` — decodes `ProjectUpdate` from body, calls `s.store.UpdateProject()`.

`handleUpdateMilestone` — parses `{id}` and `{mid}`, decodes `{"status": "done"}`, calls `s.store.UpdateMilestoneStatus()`.

`handleToolExecute` — switch on `{name}`:
- `dashboard` → calls handleDashboard logic, wraps in tool response
- `project_detail` → parse project_id or project_name from input, return full detail
- `update_progress` → sequential: update project fields, then milestone, then readiness
- `record_metric` → calls RecordMetric
- `sync_profile` → calls UpdateProfileSync

Tool response format (matches Tutor pattern):
```go
type ToolResponse struct {
    Success bool        `json:"success"`
    Data    interface{} `json:"data,omitempty"`
    Error   string      `json:"error,omitempty"`
}
```

**Request logging middleware:**
```go
func (s *Server) requestLogMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        start := time.Now()
        rw := &responseWriter{ResponseWriter: w, status: 200}
        next.ServeHTTP(rw, r)
        log.Printf("[projects] %s %s %d %s", r.Method, r.URL.Path, rw.status, time.Since(start))
    })
}
```

**Server Start/Stop:**
```go
func (s *Server) Start() error {
    s.startTime = time.Now()
    addr := net.JoinHostPort(s.host, s.port)
    s.httpServer = &http.Server{Addr: addr, Handler: s.handler()}
    log.Printf("[projects] listening on %s", addr)
    return s.httpServer.ListenAndServe()
}

func (s *Server) Shutdown(ctx context.Context) error {
    log.Printf("[projects] shutting down")
    return s.httpServer.Shutdown(ctx)
}
```

- [ ] **Step 2: Verify compilation**

Run: `go build ./internal/projects/server/`
Expected: PASS

- [ ] **Step 3: Commit**

```bash
git add internal/projects/server/server.go
git commit -m "feat(projects): add HTTP server with 14 routes and middleware"
```

---

### Task 6: Binary Entrypoint

**Files:**
- Create: `cmd/projects/main.go`

**Reference:** `cmd/tutor/main.go` (106 lines)

- [ ] **Step 1: Create binary entrypoint**

```go
package main

import (
    "context"
    "fmt"
    "log"
    "os"
    "os/signal"
    "path/filepath"
    "syscall"

    "soul-v2/internal/projects/content"
    "soul-v2/internal/projects/server"
    "soul-v2/internal/projects/store"
)

func main() {
    if len(os.Args) < 2 || os.Args[1] != "serve" {
        fmt.Fprintf(os.Stderr, "Usage: soul-projects serve\n")
        os.Exit(1)
    }
    if err := runServe(); err != nil {
        log.Fatal(err)
    }
}

func runServe() error {
    // Data dir
    dataDir := os.Getenv("SOUL_V2_DATA_DIR")
    if dataDir == "" {
        home, _ := os.UserHomeDir()
        dataDir = filepath.Join(home, ".soul-v2")
    }
    os.MkdirAll(dataDir, 0755)

    // Content dir
    contentDir := filepath.Join(dataDir, "projects", "content")
    os.MkdirAll(contentDir, 0755)

    // Open DB
    dbPath := filepath.Join(dataDir, "projects.db")
    s, err := store.Open(dbPath)
    if err != nil {
        return fmt.Errorf("open store: %w", err)
    }
    defer s.Close()

    // Seed if empty
    if err := s.Seed(); err != nil {
        return fmt.Errorf("seed: %w", err)
    }

    // Copy embedded guides to disk (skip if exists)
    copyGuides(contentDir)

    // Server
    host := envOr("SOUL_PROJECTS_HOST", "127.0.0.1")
    port := envOr("SOUL_PROJECTS_PORT", "3008")

    srv := server.New(
        server.WithStore(s),
        server.WithHost(host),
        server.WithPort(port),
        server.WithContentDir(contentDir),
    )

    // Graceful shutdown
    ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
    defer stop()
    go func() {
        <-ctx.Done()
        srv.Shutdown(context.Background())
    }()

    return srv.Start()
}

func copyGuides(contentDir string) {
    entries, err := content.Guides.ReadDir(".")
    if err != nil { return }
    for _, entry := range entries {
        if !entry.IsDir() { continue }
        name := entry.Name()
        destDir := filepath.Join(contentDir, name)
        destFile := filepath.Join(destDir, "guide.md")
        if _, err := os.Stat(destFile); err == nil {
            continue  // already exists, preserve user edits
        }
        os.MkdirAll(destDir, 0755)
        data, err := content.Guides.ReadFile(filepath.Join(name, "guide.md"))
        if err != nil { continue }
        os.WriteFile(destFile, data, 0644)
        log.Printf("[projects] copied guide: %s", name)
    }
}

func envOr(key, def string) string {
    if v := os.Getenv(key); v != "" { return v }
    return def
}
```

- [ ] **Step 2: Build binary**

Run: `go build -o soul-projects ./cmd/projects`
Expected: Binary created successfully

- [ ] **Step 3: Smoke test — start and health check**

Run:
```bash
./soul-projects serve &
sleep 2
curl -s http://127.0.0.1:3008/api/health | python3 -m json.tool
kill %1
```
Expected: Health response with project count = 11

- [ ] **Step 4: Commit**

```bash
git add cmd/projects/main.go
git commit -m "feat(projects): add server binary with auto-seed and guide copy"
```

---

### Task 7: Chat Server Integration — Proxy + Tools

**Files:**
- Modify: `internal/chat/server/proxy.go`
- Modify: `internal/chat/server/server.go`
- Modify: `cmd/chat/main.go`
- Create: `internal/chat/server/projects_tools.go`

**Reference:** Tutor proxy pattern in `proxy.go` (lines 175-216)

- [ ] **Step 1: Add projectsProxy to proxy.go**

Add after the tutorProxy section:
```go
type projectsProxy struct {
    reverseProxy *httputil.ReverseProxy
}

func newProjectsProxy() *projectsProxy {
    targetURL := os.Getenv("SOUL_PROJECTS_URL")
    if targetURL == "" {
        targetURL = "http://127.0.0.1:3008"
    }
    target, err := url.Parse(targetURL)
    if err != nil {
        log.Printf("[proxy] invalid SOUL_PROJECTS_URL: %v, using default", err)
        target, _ = url.Parse("http://127.0.0.1:3008")
    }
    rp := httputil.NewSingleHostReverseProxy(target)
    rp.Transport = &http.Transport{
        ResponseHeaderTimeout: 5 * time.Second,
    }
    rp.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
        log.Printf("[proxy] projects error: %v", err)
        w.Header().Set("Content-Type", "application/json")
        w.WriteHeader(http.StatusBadGateway)
        w.Write([]byte(`{"error":"Projects service unavailable"}`))
    }
    return &projectsProxy{reverseProxy: rp}
}

func (pp *projectsProxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    pp.reverseProxy.ServeHTTP(w, r)
}
```

- [ ] **Step 2: Add WithProjectsProxy option and route registration to server.go**

In `server.go`, add field to Server struct:
```go
projectsProxy *projectsProxy
```

Add option:
```go
func WithProjectsProxy() Option {
    return func(s *Server) {
        s.projectsProxy = newProjectsProxy()
    }
}
```

In route registration (after tutor proxy block):
```go
if s.projectsProxy != nil {
    s.mux.Handle("/api/projects/", s.projectsProxy)
    s.mux.Handle("/api/projects", s.projectsProxy)
}
```

- [ ] **Step 3: Wire in cmd/chat/main.go**

Add alongside the existing `WithTutorProxy()`:
```go
serverOpts = append(serverOpts, server.WithProjectsProxy())
```

- [ ] **Step 4: Create projects_tools.go (static tool definitions)**

Create `internal/chat/server/projects_tools.go` with tool definitions for the 5 chat tools. Follow the exact same pattern as `internal/chat/server/tutor_tools.go` — tool name, description, input schema as JSON.

Tool names: `projects__dashboard`, `projects__project_detail`, `projects__update_progress`, `projects__record_metric`, `projects__sync_profile`

- [ ] **Step 5: Verify chat server compiles**

Run: `go build ./cmd/chat/`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/chat/server/proxy.go internal/chat/server/server.go cmd/chat/main.go internal/chat/server/projects_tools.go
git commit -m "feat(projects): add chat server proxy and tool definitions"
```

---

### Task 8: Makefile + Systemd Service

**Files:**
- Modify: `Makefile`
- Create: `deploy/soul-v2-projects.service`

- [ ] **Step 1: Update Makefile**

Add `build-projects` target:
```makefile
build-projects:
	go build -o soul-projects ./cmd/projects
```

Update `build` target to include `build-projects`.
Update `clean` target to add `rm -f soul-projects`.
Update `serve` target to add `./soul-projects serve &` to the background process group.

- [ ] **Step 2: Create systemd service file**

Create `deploy/soul-v2-projects.service`:
```ini
[Unit]
Description=Soul v2 — Projects Server
After=network.target

[Service]
Type=simple
User=rishav
Group=rishav
WorkingDirectory=/home/rishav/soul-v2
ExecStart=/home/rishav/soul-v2/soul-projects serve
Environment=SOUL_PROJECTS_HOST=127.0.0.1
Environment=SOUL_PROJECTS_PORT=3008
Restart=on-failure
RestartSec=5
NoNewPrivileges=true
ProtectSystem=strict
ProtectHome=read-only
ReadWritePaths=/home/rishav/.soul-v2
PrivateTmp=true

[Install]
WantedBy=multi-user.target
```

- [ ] **Step 3: Verify build**

Run: `make build-projects`
Expected: `soul-projects` binary created

- [ ] **Step 4: Commit**

```bash
git add Makefile deploy/soul-v2-projects.service
git commit -m "chore(projects): add Makefile targets and systemd service"
```

---

## Chunk 3: Frontend — Types, Hooks, Pages, Router

### Task 9: TypeScript Types

**Files:**
- Modify: `web/src/lib/types.ts`

- [ ] **Step 1: Add Projects types**

Add after the Tutor type definitions:
```typescript
// ── Projects ──────────────────────────────────────────

export interface Project {
  id: number
  name: string
  description: string
  phase: number
  status: 'backlog' | 'active' | 'measuring' | 'documenting' | 'shipped'
  week_planned: number
  hours_estimated: number
  hours_actual: number
  github_repo: string
  readme_url: string
  created_at: string
  updated_at: string
}

export interface ProjectSummary extends Project {
  milestones_done: number
  milestones_total: number
  keyword_count: number
}

export interface Milestone {
  id: number
  project_id: number
  name: string
  description: string
  acceptance_criteria: string
  status: 'pending' | 'in_progress' | 'done' | 'skipped'
  completed_at?: string
  sort_order: number
}

export interface ProjectMetric {
  id: number
  project_id: number
  name: string
  value: string
  unit: string
  captured_at: string
}

export interface ProjectKeyword {
  id: number
  project_id?: number
  keyword: string
  status: 'claimed' | 'building' | 'shipped'
  claimed_at: string
  shipped_at?: string
}

export interface ProfileSync {
  id: number
  project_id: number
  platform: string
  synced: boolean
  synced_at?: string
  notes: string
}

export interface ProjectReadiness {
  id: number
  project_id: number
  can_explain: boolean
  can_demo: boolean
  can_tradeoffs: boolean
  self_score: number
  assessed_at: string
}

export interface ProjectDashboard {
  total_projects: number
  shipped: number
  active: number
  backlog: number
  measuring: number
  documenting: number
  keywords_total: number
  keywords_claimed: number
  keywords_building: number
  keywords_shipped: number
  hours_estimated: number
  hours_actual: number
  avg_readiness: number
  projects: ProjectSummary[]
}

export interface ProjectDetail extends Project {
  milestones: Milestone[]
  metrics: ProjectMetric[]
  keywords: ProjectKeyword[]
  syncs: ProfileSync[]
  readiness: ProjectReadiness | null
}
```

- [ ] **Step 2: Verify TypeScript compiles**

Run: `cd web && npx tsc --noEmit`
Expected: No errors

- [ ] **Step 3: Commit**

```bash
git add web/src/lib/types.ts
git commit -m "feat(projects): add TypeScript type definitions"
```

---

### Task 10: React Hooks — useProjects and useProjectDetail

**Files:**
- Create: `web/src/hooks/useProjects.ts`
- Create: `web/src/hooks/useProjectDetail.ts`

**Reference:** `web/src/hooks/useTutor.ts` (82 lines), `web/src/hooks/useDrill.ts` (67 lines)

- [ ] **Step 1: Create useProjects hook**

```typescript
import { useState, useEffect, useCallback } from 'react'
import { api } from '../lib/api'
import { reportUsage, reportError } from '../lib/api'
import type { ProjectDashboard, ProjectKeyword } from '../lib/types'

export type ProjectsTab = 'dashboard' | 'projects' | 'timeline' | 'keywords'

interface UseProjectsReturn {
  dashboard: ProjectDashboard | null
  keywords: ProjectKeyword[]
  loading: boolean
  error: string | null
  activeTab: ProjectsTab
  setActiveTab: (tab: ProjectsTab) => void
  refresh: () => void
}

export function useProjects(): UseProjectsReturn {
  const [dashboard, setDashboard] = useState<ProjectDashboard | null>(null)
  const [keywords, setKeywords] = useState<ProjectKeyword[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [activeTab, setActiveTabState] = useState<ProjectsTab>('dashboard')

  const fetchData = useCallback(async (tab: ProjectsTab) => {
    setLoading(true)
    setError(null)
    try {
      if (tab === 'keywords') {
        const data = await api.get('/api/projects/keywords')
        setKeywords(data)
      } else {
        // dashboard, projects, timeline all use the same dashboard endpoint
        const data = await api.get('/api/projects/dashboard')
        setDashboard(data)
      }
    } catch (err) {
      const msg = err instanceof Error ? err.message : 'Failed to load projects'
      setError(msg)
      reportError('useProjects', msg)
    } finally {
      setLoading(false)
    }
  }, [])

  const setActiveTab = useCallback((tab: ProjectsTab) => {
    setActiveTabState(tab)
    reportUsage('projects.tab', { tab })
  }, [])

  useEffect(() => { fetchData(activeTab) }, [activeTab, fetchData])

  const refresh = useCallback(() => fetchData(activeTab), [activeTab, fetchData])

  return { dashboard, keywords, loading, error, activeTab, setActiveTab, refresh }
}
```

- [ ] **Step 2: Create useProjectDetail hook**

```typescript
import { useState, useEffect, useCallback } from 'react'
import { api } from '../lib/api'
import { reportUsage, reportError } from '../lib/api'
import type { ProjectDetail, Milestone, ProjectMetric, ProjectReadiness, ProfileSync } from '../lib/types'

interface UseProjectDetailReturn {
  project: ProjectDetail | null
  guide: string
  loading: boolean
  error: string | null
  updateProject: (fields: Record<string, unknown>) => Promise<void>
  updateMilestone: (milestoneId: number, status: string) => Promise<void>
  recordMetric: (name: string, value: string, unit: string) => Promise<void>
  updateReadiness: (assessment: { can_explain: boolean; can_demo: boolean; can_tradeoffs: boolean; self_score: number }) => Promise<void>
  syncPlatform: (platform: string, notes?: string) => Promise<void>
  refresh: () => void
}

export function useProjectDetail(projectId: number): UseProjectDetailReturn {
  const [project, setProject] = useState<ProjectDetail | null>(null)
  const [guide, setGuide] = useState('')
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  const fetchProject = useCallback(async () => {
    setLoading(true)
    setError(null)
    try {
      const [detail, guideData] = await Promise.all([
        api.get(`/api/projects/${projectId}`),
        api.get(`/api/projects/${projectId}/guide`),
      ])
      setProject(detail)
      setGuide(guideData.content || '')
    } catch (err) {
      const msg = err instanceof Error ? err.message : 'Failed to load project'
      setError(msg)
      reportError('useProjectDetail', msg)
    } finally {
      setLoading(false)
    }
  }, [projectId])

  useEffect(() => { fetchProject() }, [fetchProject])

  const updateProject = useCallback(async (fields: Record<string, unknown>) => {
    await api.patch(`/api/projects/${projectId}`, fields)
    reportUsage('project.update', { projectId })
    await fetchProject()
  }, [projectId, fetchProject])

  const updateMilestone = useCallback(async (milestoneId: number, status: string) => {
    await api.patch(`/api/projects/${projectId}/milestones/${milestoneId}`, { status })
    reportUsage('milestone.complete', { projectId, milestoneId, status })
    await fetchProject()
  }, [projectId, fetchProject])

  const recordMetric = useCallback(async (name: string, value: string, unit: string) => {
    await api.post(`/api/projects/${projectId}/metrics`, { name, value, unit })
    reportUsage('metric.record', { projectId })
    await fetchProject()
  }, [projectId, fetchProject])

  const updateReadiness = useCallback(async (assessment: { can_explain: boolean; can_demo: boolean; can_tradeoffs: boolean; self_score: number }) => {
    await api.post(`/api/projects/${projectId}/readiness`, assessment)
    reportUsage('readiness.assess', { projectId })
    await fetchProject()
  }, [projectId, fetchProject])

  const syncPlatform = useCallback(async (platform: string, notes?: string) => {
    await api.post(`/api/projects/${projectId}/syncs`, { platform, notes })
    reportUsage('sync.platform', { projectId, platform })
    await fetchProject()
  }, [projectId, fetchProject])

  return { project, guide, loading, error, updateProject, updateMilestone, recordMetric, updateReadiness, syncPlatform, refresh: fetchProject }
}
```

- [ ] **Step 3: Verify TypeScript compiles**

Run: `cd web && npx tsc --noEmit`
Expected: No errors

- [ ] **Step 4: Commit**

```bash
git add web/src/hooks/useProjects.ts web/src/hooks/useProjectDetail.ts
git commit -m "feat(projects): add useProjects and useProjectDetail hooks"
```

---

### Task 11: ProjectCard Component + ProjectsPage (4 tabs)

**Files:**
- Create: `web/src/components/ProjectCard.tsx`
- Create: `web/src/pages/ProjectsPage.tsx`

**Reference:** `web/src/pages/TutorPage.tsx` (392 lines, 5 tabs)

- [ ] **Step 1: Create ProjectCard component**

Props: `ProjectSummary` + `onClick`. Shows:
- Name, status badge, phase number
- Description (truncated to 2 lines)
- Milestone progress bar (done/total)
- Keyword count badge
- Hours (estimated vs actual)
- `data-testid="project-card-{id}"`

Status colors: backlog=zinc-600, active=blue-500/20, measuring=amber-500/20, documenting=purple-500/20, shipped=emerald-500/20

- [ ] **Step 2: Create ProjectsPage with 4 tabs**

Component structure:
```typescript
export function ProjectsPage() {
  const { dashboard, keywords, loading, error, activeTab, setActiveTab, refresh } = useProjects()

  useEffect(() => { reportUsage('page.view', { page: 'projects' }) }, [])
  usePerformance('ProjectsPage')

  // Tab navigation: dashboard | projects | timeline | keywords
  // Tab content via switch(activeTab)
}
```

**Dashboard tab:**
- Overall progress bar (shipped / total)
- 5 status cards (backlog, active, measuring, documenting, shipped) with counts
- Keyword coverage: shipped/total
- Hours: estimated vs actual
- Average readiness score
- `data-testid="projects-dashboard"`

**Projects tab:**
- Grid of `ProjectCard` components (responsive: 1/2/3 cols)
- Click card → `navigate('/projects/${id}')`
- `data-testid="projects-list"`

**Timeline tab:**
- 10-week grid with 4 phase rows
- Projects positioned by `week_planned`, colored by status
- `data-testid="projects-timeline"`

**Keywords tab:**
- 3 groups: Shipped (emerald), Building (amber), Claimed (zinc)
- Each keyword badge shows linked project name
- `data-testid="projects-keywords"`

- [ ] **Step 3: Verify TypeScript compiles**

Run: `cd web && npx tsc --noEmit`
Expected: No errors

- [ ] **Step 4: Commit**

```bash
git add web/src/components/ProjectCard.tsx web/src/pages/ProjectsPage.tsx
git commit -m "feat(projects): add ProjectCard and ProjectsPage with 4 tabs"
```

---

### Task 12: ProjectDetailPage

**Files:**
- Create: `web/src/pages/ProjectDetailPage.tsx`

- [ ] **Step 1: Create ProjectDetailPage with 4 tabs**

```typescript
export function ProjectDetailPage() {
  const { id } = useParams<{ id: string }>()
  const projectId = parseInt(id!, 10)
  const { project, guide, loading, error, updateProject, updateMilestone, recordMetric, updateReadiness, syncPlatform, refresh } = useProjectDetail(projectId)

  useEffect(() => { reportUsage('page.view', { page: 'project_detail', projectId }) }, [projectId])
  usePerformance('ProjectDetailPage')

  // Tab navigation: milestones | guide | readiness | metrics
}
```

**Header:** Project name, status badge (editable dropdown), phase, hours (estimated / actual with edit), github link, back button to `/projects`.

**Milestones tab:**
- List of milestones ordered by sort_order
- Each: name, description, acceptance criteria (gray text), status badge
- Status change buttons: pending → in_progress → done (or skipped)
- `data-testid="milestone-{id}"`

**Guide tab:**
- Rendered markdown content (use a simple markdown renderer — split on headers, render code blocks with `<pre>`)
- Or render as `whitespace-pre-wrap` formatted text if markdown rendering is too complex
- `data-testid="project-guide"`

**Readiness tab:**
- 3 toggles: Can Explain, Can Demo, Can Discuss Tradeoffs (switch style)
- Self-score slider 1-5
- Save button → calls updateReadiness
- Platform sync section: 7 platform rows with checkbox + notes field
- `data-testid="project-readiness"`

**Metrics tab:**
- Table: name, value, unit, captured_at
- Add metric form: name input, value input, unit input, submit button
- `data-testid="project-metrics"`

- [ ] **Step 2: Verify TypeScript compiles**

Run: `cd web && npx tsc --noEmit`
Expected: No errors

- [ ] **Step 3: Commit**

```bash
git add web/src/pages/ProjectDetailPage.tsx
git commit -m "feat(projects): add ProjectDetailPage with milestones, guide, readiness, metrics"
```

---

### Task 13: Router, AppLayout, Build, Deploy, CLAUDE.md

**Files:**
- Modify: `web/src/router.tsx`
- Modify: `web/src/layouts/AppLayout.tsx`
- Modify: `CLAUDE.md`

- [ ] **Step 1: Add routes to router.tsx**

Add after tutor routes:
```typescript
{ path: 'projects', lazy: () => import('./pages/ProjectsPage').then(m => ({ Component: m.ProjectsPage })) },
{ path: 'projects/:id', lazy: () => import('./pages/ProjectDetailPage').then(m => ({ Component: m.ProjectDetailPage })) },
```

- [ ] **Step 2: Add Projects NavLink to AppLayout.tsx**

Add after the Tutor NavLink:
```tsx
<NavLink to="/projects" className={navLinkClass}>Projects</NavLink>
```

- [ ] **Step 3: Update CLAUDE.md architecture section**

Add to the architecture tree:
```
cmd/projects/main.go             Projects server CLI entrypoint (:3008)
internal/projects/
  server/                        HTTP server + REST API handlers
  store/                         SQLite project CRUD (projects.db) — 7 tables
  content/                       Embedded implementation guides (go:embed)
```

Add to routes:
```
/projects, /projects/:id
```

Add to hooks:
```
useProjects, useProjectDetail
```

Add to env vars:
```
SOUL_PROJECTS_HOST, SOUL_PROJECTS_PORT, SOUL_PROJECTS_URL
```

- [ ] **Step 4: Build frontend**

Run: `cd web && npx vite build`
Expected: Build succeeds. Check bundle output — ProjectsPage, ProjectDetailPage chunks should be small (< 20KB each).

- [ ] **Step 5: Build all Go binaries**

Run: `make build`
Expected: `soul-chat`, `soul-tasks`, `soul-tutor`, `soul-projects` all built successfully

- [ ] **Step 6: Run Go tests**

Run: `go test ./internal/projects/... -v -count=1`
Expected: All store tests PASS

- [ ] **Step 7: Run static verification**

Run: `go vet ./... && cd web && npx tsc --noEmit`
Expected: No errors

- [ ] **Step 8: Install and start systemd service**

```bash
sudo cp deploy/soul-v2-projects.service /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable soul-v2-projects
sudo systemctl start soul-v2-projects
```

- [ ] **Step 9: Smoke test — all endpoints**

```bash
# Health
curl -s http://127.0.0.1:3008/api/health | python3 -m json.tool

# Dashboard
curl -s http://127.0.0.1:3008/api/projects/dashboard | python3 -m json.tool

# Single project
curl -s http://127.0.0.1:3008/api/projects/1 | python3 -m json.tool

# Keywords
curl -s http://127.0.0.1:3008/api/projects/keywords | python3 -m json.tool

# Guide
curl -s http://127.0.0.1:3008/api/projects/1/guide | python3 -m json.tool

# Chat proxy (through chat server)
curl -sk https://127.0.0.1:3002/api/projects/dashboard | python3 -m json.tool
```

Expected: All return valid JSON with correct data (11 projects, milestones, keywords, guide content).

- [ ] **Step 10: Restart chat server to pick up proxy**

```bash
sudo systemctl restart soul-v2-chat
```

- [ ] **Step 11: Commit**

```bash
git add web/src/router.tsx web/src/layouts/AppLayout.tsx CLAUDE.md
git commit -m "feat(projects): wire router, nav, deploy, update CLAUDE.md"
```

- [ ] **Step 12: Final full build verification**

```bash
make build
go vet ./...
cd web && npx tsc --noEmit
go test ./internal/projects/... -v -count=1
```

All must pass.
