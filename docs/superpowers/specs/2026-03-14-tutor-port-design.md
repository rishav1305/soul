# Tutor Product — Soul v2 Port Design

## Overview

Port the Tutor interview preparation product from Soul v1 to v2. Full port of all 5 modules (DSA, AI/ML, Behavioral, Mock, Planner) with both chat tool integration and standalone interactive UI.

## Architecture

Tutor runs as a **standalone Go server process** on port 3006, with its own SQLite database at `~/.soul-v2/tutor.db`. This follows Soul v2's isolation pattern — if Tutor crashes, Chat and Tasks remain unaffected.

```
cmd/tutor/main.go                 Tutor server entrypoint (:3006) — serve subcommand
internal/tutor/
  server/server.go                HTTP server + REST API handlers
  store/
    store.go                      SQLite CRUD — schema, migrations, queries
    store_test.go                 Store unit tests
  modules/
    dsa.go                        DSA module (learn, build, drill, solve, generate)
    ai.go                         AI/ML module (learn_theory, drill_theory, generate)
    behavioral.go                 Behavioral module (narrative, STAR, HR drill)
    mock.go                       Mock module (interview, analyze_jd)
    planner.go                    Planner module (create/update/get plan)
    sm2.go                        SM-2 spaced repetition algorithm
    importer.go                   Content importer from ~/interview-prep
web/src/
  pages/
    TutorPage.tsx                 Main page with 5 tabs (Dashboard, Analytics, Topics, Mocks, Guide)
    DrillPage.tsx                 Interactive drill session
    MockPage.tsx                  Interactive mock interview flow
  hooks/
    useTutor.ts                   Tutor data fetching + state management
  components/
    ModuleCard.tsx                Module progress card
    TopicRow.tsx                  Topic list row
    MockSessionCard.tsx           Mock session summary card
    DrillSession.tsx              Drill question/answer flow component
    ReadinessBar.tsx              Interview readiness progress bar
```

### Process Isolation

- **Systemd**: `soul-v2-tutor.service` (same pattern as `soul-v2-tasks.service`)
- **Port**: 3006 (env: `SOUL_TUTOR_PORT`)
- **Database**: `~/.soul-v2/tutor.db` (isolated from chat.db and tasks.db)
- **Chat proxy**: Chat server forwards `/api/tutor/*` to `SOUL_TUTOR_URL` (default `http://127.0.0.1:3006`)

### Chat Tool Integration

Static tool registry in the chat server (same pattern as tasks tools in `internal/tasks/executor/tools.go`):
- Tool definitions hardcoded in `internal/chat/server/tutor_tools.go` with names, descriptions, input schemas
- When Claude calls `tutor__drill`, chat server proxies to `POST SOUL_TUTOR_URL/api/tools/drill/execute`
- If Tutor server is down, tool call returns graceful error ("Tutor service unavailable")
- No dynamic discovery — keeps startup simple and testable without a running Tutor server

## Data Model

Direct port of v1's 11-table SQLite schema.

### Tables

| Table | Purpose | Key Columns |
|-------|---------|-------------|
| `topics` | Study topics per module | id, module, category, name, difficulty, content_path, status, created_at |
| `progress` | Study session records | id, topic_id (FK), session_date, score, questions_asked, questions_correct, time_spent_seconds, notes |
| `quiz_questions` | Question bank per topic | id, topic_id (FK), difficulty, question_text, answer_text, explanation, source |
| `spaced_repetition` | SM-2 state per topic | id, topic_id (UNIQUE FK), last_reviewed, next_review, interval_days, ease_factor, repetition_count |
| `daily_activity` | Aggregated daily metrics | id, date, module, time_spent_seconds, sessions_count, questions_answered, score_avg |
| `confidence_ratings` | Self vs actual scores | id, topic_id (FK), self_rated_score, actual_score, rated_at |
| `mock_sessions` | Mock interview sessions | id, type, job_description, started_at, completed_at, overall_score, feedback_json |
| `mock_session_scores` | Dimensional mock scores | id, mock_session_id (FK), dimension, score |
| `star_stories` | STAR interview stories | id, competency, situation, task, action, result, projects_referenced, version |
| `study_plans` | Study plans with timeline | id, target_role, target_date, created_at, plan_json, active |
| `question_attempts` | Per-answer records | id, quiz_question_id (FK), progress_id (FK), answered_correctly, time_taken_seconds, user_answer_summary |

### Constraints

- Foreign keys enabled (`PRAGMA foreign_keys = ON`)
- WAL mode for concurrent reads (`PRAGMA journal_mode = WAL`)
- UNIQUE on `topics(module, category, name)`
- UNIQUE on `spaced_repetition(topic_id)`
- Indexes on all filter columns: module, topic_id, date, next_review

### SM-2 Spaced Repetition

Per-topic state with standard SM-2 algorithm:
- Quality scale: 0-5 (0=blackout, 5=perfect)
- On failure (quality < 3): reset interval=1, repetitions=0
- On success: interval progresses 1 → 6 → round(interval * ease_factor)
- Ease factor: ef += 0.1 - (5-q)*(0.08+(5-q)*0.02), clamped >= 1.3

## API Design

### Tutor REST API (port 3006)

**Dashboard & Progress:**
```
GET  /api/tutor/dashboard          Readiness %, module stats, streak, due reviews, today's activity
GET  /api/tutor/analytics          30-day activity + confidence gaps
GET  /api/tutor/topics?module=dsa  Topic list with optional module filter
GET  /api/tutor/topics/:id         Single topic detail with content
```

**Drill (Interactive UI):**
```
POST /api/tutor/drill/start        Pick next question for topic (SM-2 aware)
POST /api/tutor/drill/answer       Submit answer, get evaluation + SM-2 update
GET  /api/tutor/drill/due          Topics due for spaced repetition review
```

**Mock Interviews:**
```
GET  /api/tutor/mocks              List mock sessions
POST /api/tutor/mocks              Create new mock session (type + optional JD)
GET  /api/tutor/mocks/:id          Session detail with questions + scores
POST /api/tutor/mocks/:id/answer   Submit answer for mock question
```

**Planner:**
```
GET   /api/tutor/plan              Get active study plan
POST  /api/tutor/plan              Create plan (target_role, target_date)
PATCH /api/tutor/plan              Update/recalculate plan
```

**Content:**
```
POST /api/tutor/import             Trigger content import from ~/interview-prep
POST /api/tutor/content/generate   Generate content for a topic
```

**Chat Tool Execution:**
```
POST /api/tools/:name/execute      Execute tool by name (chat server proxies here)
```

**Health:**
```
GET  /api/health                   Health check
```

### Chat Server Proxy

Chat server adds proxy routes (files to modify):
- `internal/chat/server/server.go` — add `/api/tutor/` reverse proxy block (same pattern as tasks proxy, lines 139-145)
- `cmd/chat/main.go` — add `WithTutorProxy` option wiring (no SSE relay needed, just `httputil.ReverseProxy`)
- `web/src/layouts/AppLayout.tsx` — add Tutor NavLink to navigation

Frontend calls `/api/tutor/*` on the chat server (port 3002), which forwards to tutor server (port 3006). If tutor server is down, proxy returns 502 and frontend shows graceful error.

## 5 Modules — Tool Implementations

### DSA Module (5 tools)

1. **learn** — Fetch topic content, mark status as "learning", return file content + metadata
2. **build** — 5-step implementation guide (Interface → Core → Edge Cases → Tests → Analysis)
3. **drill** — Pick question (SM-2 priority), evaluate answer (keyword matching >= 50%), record progress + attempt, update SM-2 state
4. **solve** — 4-step problem walkthrough (Understand → Identify Pattern → Solve → Analyze)
5. **generate_content** — Create topic + write markdown study file to `~/.soul-v2/tutor/content/dsa/{category}/{name}.md`

### AI/ML Module (3 tools)

6. **learn_theory** — Theory learning with depth levels (overview/detailed/deep), auto-create topic if missing
7. **drill_theory** — Quiz drilling with SM-2 (same mechanism as DSA)
8. **generate_ai_content** — Content outline with 6 sections (intro, concepts, math, implementation, applications, interviews)

### Behavioral Module (3 tools)

9. **build_narrative** — "Tell me about yourself" template with 5 sections
10. **build_star** — STAR story builder for competencies (leadership, conflict, failure, teamwork, innovation, ownership)
11. **drill_hr** — HR question drilling from predefined bank (7 categories)

### Mock Module (2 tools)

12. **mock_interview** — Generate interview plan (technical/behavioral/full_loop, 3-5 questions)
13. **analyze_jd** — Parse job description, extract skills, map to modules, identify gaps

### Planner Module (1 tool)

14. **plan** — Create/update/get study plan with phased timeline

### Cross-cutting (1 tool)

15. **progress** — 4 views: dashboard, analytics, topics, mocks

## Frontend

### Route Structure

```
/tutor              → TutorPage (tabbed: Dashboard, Analytics, Topics, Mocks, Guide)
/tutor/drill/:id    → DrillPage (interactive quiz session for topic)
/tutor/mock/:id     → MockPage (interactive mock interview session)
```

All routes lazy-loaded, wrapped in AppLayout, with per-route error boundaries.

### TutorPage Tabs

**Dashboard:**
- Interview readiness % bar (0-100%)
- Streak badge (consecutive days) + Due reviews badge (topics needing review)
- 3 module cards (DSA, AI/ML, Behavioral): completion %, mastered/total, status breakdown pills (New/Learn/Drill/Done)
- Today's activity: time spent, sessions, questions, avg score

**Analytics:**
- 30-day activity table: date, module, time, sessions, questions, score
- Confidence gaps: topic, self-rated vs actual, gap highlighted (red >20%, amber <=20%)

**Topics:**
- Module filter buttons: All / DSA / AI/ML / Behavioral
- Topic rows: name, category, difficulty badge (easy/medium/hard), status badge (not_started/learning/drilling/mastered)
- Click topic → navigate to `/tutor/drill/:id`

**Mocks:**
- Session cards: type, date, score bar (0-100%), job description snippet
- Score coloring: emerald >=80%, amber 50-79%, red <50%
- Click → navigate to `/tutor/mock/:id`

**Guide:**
- Tab descriptions, module-specific chat commands, recommended 8-step learning flow, mastery criteria

### DrillPage (Interactive)

- Topic name + difficulty at top
- Question text displayed
- Text area for answer input
- Submit → shows evaluation (correct/incorrect), explanation, SM-2 next review date
- "Next Question" → picks next by SM-2 priority
- Session progress bar (questions answered, running score)
- Back button → returns to TutorPage Topics tab

### MockPage (Interactive)

- Interview type + JD summary header
- Questions presented one at a time
- Answer text area with optional timer display
- After all questions: dimensional scoring (technical, communication, problem-solving)
- Overall score + feedback summary
- Back button → returns to TutorPage Mocks tab

### Hooks

**useTutor()** — Dashboard/analytics/topics/mocks data fetching:
- States: dashboard, topics, analytics, mocks, loading, error
- Methods: refresh(), setModuleFilter()
- Fetches on mount + tab change + filter change

**useDrill(topicId)** — Interactive drill session:
- States: question, evaluation, sessionStats, loading
- Methods: startDrill(), submitAnswer(), nextQuestion()

**useMockSession(sessionId)** — Interactive mock:
- States: session, currentQuestion, scores, loading
- Methods: submitAnswer(), completeSession()

### Color Scheme

Follows v2 dark theme (zinc palette):
- Status: not_started=zinc, learning=blue, drilling=amber, mastered=emerald
- Difficulty: easy=emerald, medium=amber, hard=red
- Scores: >=80%=emerald, 50-79%=amber, <50%=red

## Pillar Compliance

### Performant
- WAL mode + indexed queries for sub-200ms responses
- Lazy-loaded routes for minimal bundle impact (verify < 300KB gzipped gate post-implementation)
- Dashboard aggregates in single SQL query (no N+1)
- `usePerformance` on all Tutor pages and key components

### Robust
- Foreign keys + UNIQUE constraints prevent invalid data states
- Input validation on all API endpoints
- Defined behavior for empty state (zero topics, zero sessions)
- Error boundaries per route — DrillPage crash doesn't affect TutorPage

### Resilient
- Tutor server down → frontend shows status, Chat/Tasks unaffected
- Atomic transactions for multi-table writes (drill → progress + spaced_rep + daily_activity)
- Chat server gracefully handles tutor proxy failures

### Secure
- Parameterized SQL only (never string concat)
- Input sanitized at API boundary
- No secrets in tutor data
- CSP headers on all responses

### Sovereign
- SQLite local storage, zero external dependencies
- No CDN/external assets
- Content files stored locally at `~/.soul-v2/tutor/content/`

### Transparent
- Every drill answer, mock score, plan change recorded with timestamp
- `reportUsage` on: page views (tutor, drill, mock), feature actions (drill.start, drill.answer, mock.create, plan.create, import)
- `reportError` on all failure paths
- `usePerformance` on all pages and key components
- Backend tool execution logged as structured events (tool name, latency, success/error)
- Daily activity aggregation queryable via CLI
- Streak and confidence gap detection from recorded data

## Environment Variables

| Variable | Default | Purpose |
|----------|---------|---------|
| `SOUL_TUTOR_PORT` | `3006` | Tutor server port |
| `SOUL_TUTOR_HOST` | `127.0.0.1` | Tutor server bind address |
| `SOUL_TUTOR_URL` | `http://127.0.0.1:3006` | Tutor URL (for chat server proxy) |
| `SOUL_V2_DATA_DIR` | `~/.soul-v2` | Data directory (tutor.db lives here) |

## Content Import

Importer reads Python files from `~/interview-prep/week1/stdlib_cheatsheet/`:
- Copies `.py` files to `~/.soul-v2/tutor/content/dsa/stdlib/`
- Parses `EVALUATION_QUESTIONS` list from each file (regex extraction)
- Creates topics (module=dsa, category=stdlib)
- Inserts quiz questions with parsed difficulty, question_text, answer_text
- Triggered via `POST /api/tutor/import` or on first server startup if DB is empty

## Build & Deploy

### Makefile Targets

- `build-tutor`: `go build -o soul-tutor ./cmd/tutor`
- Update top-level `build` target to include `build-tutor`
- Update `clean` target to remove `soul-tutor` binary
- Update `deploy` target to install tutor service

### Systemd Service

```ini
[Unit]
Description=Soul v2 Tutor Server
After=network.target

[Service]
Type=simple
User=rishav
Group=rishav
WorkingDirectory=/home/rishav/soul-v2
ExecStart=/home/rishav/soul-v2/soul-tutor serve
Environment=SOUL_TUTOR_PORT=3006
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

### Files Modified in Existing Codebase

| File | Change |
|------|--------|
| `internal/chat/server/server.go` | Add `/api/tutor/` reverse proxy block |
| `cmd/chat/main.go` | Add `WithTutorProxy` option |
| `internal/chat/server/tutor_tools.go` | Static tool definitions for chat integration (new file) |
| `web/src/layouts/AppLayout.tsx` | Add Tutor NavLink |
| `web/src/router.tsx` | Add /tutor, /tutor/drill/:id, /tutor/mock/:id routes |
| `web/src/lib/types.ts` | Add Tutor type definitions |
| `Makefile` | Add build-tutor, update build/clean/deploy |
| `deploy/soul-v2-tutor.service` | New systemd unit file |
