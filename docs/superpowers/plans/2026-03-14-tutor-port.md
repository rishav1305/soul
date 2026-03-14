# Tutor Product Port — Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Port the Tutor interview preparation product from Soul v1 to v2 as an isolated server with REST API, chat tool integration, and standalone interactive UI.

**Architecture:** Standalone Go server on port 3006 with own SQLite DB. Chat server proxies `/api/tutor/*` requests. Frontend adds `/tutor`, `/tutor/drill/:id`, `/tutor/mock/:id` routes with lazy loading.

**Tech Stack:** Go 1.24, SQLite (modernc.org/sqlite), React 19, TypeScript 5.9, Vite 7, Tailwind v4

**Spec:** `docs/superpowers/specs/2026-03-14-tutor-port-design.md`

---

## File Structure

### New Files — Backend

| File | Responsibility |
|------|---------------|
| `cmd/tutor/main.go` | Server entrypoint — `serve` subcommand, DB init, signal handling |
| `internal/tutor/store/store.go` | SQLite store — schema (11 tables), Open, CRUD, activity logging |
| `internal/tutor/store/store_test.go` | Store unit tests |
| `internal/tutor/modules/sm2.go` | SM-2 spaced repetition algorithm (pure function) |
| `internal/tutor/modules/sm2_test.go` | SM-2 unit tests |
| `internal/tutor/modules/dsa.go` | DSA module — learn, build, drill, solve, generate_content |
| `internal/tutor/modules/ai.go` | AI/ML module — learn_theory, drill_theory, generate_ai_content |
| `internal/tutor/modules/behavioral.go` | Behavioral module — build_narrative, build_star, drill_hr |
| `internal/tutor/modules/mock.go` | Mock module — mock_interview, analyze_jd |
| `internal/tutor/modules/planner.go` | Planner module — create/update/get study plan |
| `internal/tutor/modules/importer.go` | Content importer from ~/interview-prep |
| `internal/tutor/modules/progress.go` | Progress tool — dashboard, analytics, topics, mocks views |
| `internal/tutor/server/server.go` | HTTP server — REST API routes, middleware, health |
| `internal/tutor/server/handlers.go` | HTTP handler implementations |
| `internal/tutor/server/tools.go` | Chat tool execution endpoint — routes tool name to module method |
| `deploy/soul-v2-tutor.service` | Systemd unit file |

### New Files — Frontend

| File | Responsibility |
|------|---------------|
| `web/src/pages/TutorPage.tsx` | Main page with 5 tabs (Dashboard, Analytics, Topics, Mocks, Guide) |
| `web/src/pages/DrillPage.tsx` | Interactive drill session |
| `web/src/pages/MockPage.tsx` | Interactive mock interview flow |
| `web/src/hooks/useTutor.ts` | Tutor dashboard/analytics/topics/mocks data fetching |
| `web/src/hooks/useDrill.ts` | Interactive drill session state |
| `web/src/hooks/useMockSession.ts` | Interactive mock interview state |
| `web/src/components/ReadinessBar.tsx` | Interview readiness progress bar |
| `web/src/components/ModuleCard.tsx` | Module progress card |
| `web/src/components/TopicRow.tsx` | Topic list row |
| `web/src/components/MockSessionCard.tsx` | Mock session summary card |
| `web/src/components/DrillSession.tsx` | Drill question/answer flow component |

### Modified Files

| File | Change |
|------|--------|
| `internal/chat/server/server.go` | Add `/api/tutor/` reverse proxy block |
| `internal/chat/server/proxy.go` | Add `newTutorProxy()`, `WithTutorProxy()` |
| `cmd/chat/main.go` | Wire tutor proxy option |
| `web/src/router.tsx` | Add /tutor, /tutor/drill/:id, /tutor/mock/:id routes |
| `web/src/layouts/AppLayout.tsx` | Add Tutor NavLink |
| `web/src/lib/types.ts` | Add Tutor type definitions |
| `Makefile` | Add build-tutor, update build/clean/serve targets |

---

## Chunk 1: Store + SM-2 Foundation

### Task 1: Tutor Store — Schema and Core CRUD

**Files:**
- Create: `internal/tutor/store/store.go`
- Test: `internal/tutor/store/store_test.go`

- [ ] **Step 1: Create store with Open, migrate, Close**

Create `internal/tutor/store/store.go`. Follow the exact pattern from `internal/tasks/store/store.go`:

```go
package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

type Store struct {
	db     *sql.DB
	dbPath string
}

// Open creates a new Store with the given database path.
func Open(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("tutor: open database: %w", err)
	}
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		db.Close()
		return nil, fmt.Errorf("tutor: enable WAL: %w", err)
	}
	if _, err := db.Exec("PRAGMA foreign_keys=ON"); err != nil {
		db.Close()
		return nil, fmt.Errorf("tutor: enable foreign keys: %w", err)
	}
	db.SetMaxOpenConns(1)
	if _, err := db.Exec("PRAGMA busy_timeout=5000"); err != nil {
		db.Close()
		return nil, fmt.Errorf("tutor: set busy timeout: %w", err)
	}
	s := &Store{db: db, dbPath: path}
	if err := s.migrate(); err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) Close() error { return s.db.Close() }
```

Schema — all 11 tables from spec:

```go
func (s *Store) migrate() error {
	const schema = `
	CREATE TABLE IF NOT EXISTS topics (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		module TEXT NOT NULL,
		category TEXT NOT NULL,
		name TEXT NOT NULL,
		difficulty TEXT NOT NULL DEFAULT 'medium',
		content_path TEXT NOT NULL DEFAULT '',
		status TEXT NOT NULL DEFAULT 'not_started',
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		UNIQUE(module, category, name)
	);
	CREATE TABLE IF NOT EXISTS progress (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		topic_id INTEGER NOT NULL REFERENCES topics(id) ON DELETE CASCADE,
		session_date DATE NOT NULL DEFAULT (date('now')),
		score REAL NOT NULL DEFAULT 0,
		questions_asked INTEGER NOT NULL DEFAULT 0,
		questions_correct INTEGER NOT NULL DEFAULT 0,
		time_spent_seconds INTEGER NOT NULL DEFAULT 0,
		notes TEXT NOT NULL DEFAULT ''
	);
	CREATE TABLE IF NOT EXISTS quiz_questions (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		topic_id INTEGER NOT NULL REFERENCES topics(id) ON DELETE CASCADE,
		difficulty TEXT NOT NULL DEFAULT 'medium',
		question_text TEXT NOT NULL,
		answer_text TEXT NOT NULL,
		explanation TEXT NOT NULL DEFAULT '',
		source TEXT NOT NULL DEFAULT ''
	);
	CREATE TABLE IF NOT EXISTS spaced_repetition (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		topic_id INTEGER NOT NULL UNIQUE REFERENCES topics(id) ON DELETE CASCADE,
		last_reviewed DATETIME,
		next_review DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		interval_days REAL NOT NULL DEFAULT 1,
		ease_factor REAL NOT NULL DEFAULT 2.5,
		repetition_count INTEGER NOT NULL DEFAULT 0
	);
	CREATE TABLE IF NOT EXISTS daily_activity (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		date DATE NOT NULL,
		module TEXT NOT NULL,
		time_spent_seconds INTEGER NOT NULL DEFAULT 0,
		sessions_count INTEGER NOT NULL DEFAULT 0,
		questions_answered INTEGER NOT NULL DEFAULT 0,
		score_avg REAL NOT NULL DEFAULT 0,
		UNIQUE(date, module)
	);
	CREATE TABLE IF NOT EXISTS confidence_ratings (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		topic_id INTEGER NOT NULL REFERENCES topics(id) ON DELETE CASCADE,
		self_rated_score REAL NOT NULL,
		actual_score REAL NOT NULL,
		rated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	);
	CREATE TABLE IF NOT EXISTS mock_sessions (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		type TEXT NOT NULL,
		job_description TEXT NOT NULL DEFAULT '',
		started_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		completed_at DATETIME,
		overall_score REAL,
		feedback_json TEXT NOT NULL DEFAULT '{}'
	);
	CREATE TABLE IF NOT EXISTS mock_session_scores (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		mock_session_id INTEGER NOT NULL REFERENCES mock_sessions(id) ON DELETE CASCADE,
		dimension TEXT NOT NULL,
		score REAL NOT NULL
	);
	CREATE TABLE IF NOT EXISTS star_stories (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		competency TEXT NOT NULL,
		situation TEXT NOT NULL DEFAULT '',
		task TEXT NOT NULL DEFAULT '',
		action TEXT NOT NULL DEFAULT '',
		result TEXT NOT NULL DEFAULT '',
		projects_referenced TEXT NOT NULL DEFAULT '',
		version INTEGER NOT NULL DEFAULT 1
	);
	CREATE TABLE IF NOT EXISTS study_plans (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		target_role TEXT NOT NULL,
		target_date DATE NOT NULL,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		plan_json TEXT NOT NULL DEFAULT '{}',
		active INTEGER NOT NULL DEFAULT 1
	);
	CREATE TABLE IF NOT EXISTS question_attempts (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		quiz_question_id INTEGER NOT NULL REFERENCES quiz_questions(id) ON DELETE CASCADE,
		progress_id INTEGER REFERENCES progress(id) ON DELETE SET NULL,
		answered_correctly INTEGER NOT NULL DEFAULT 0,
		time_taken_seconds INTEGER NOT NULL DEFAULT 0,
		user_answer_summary TEXT NOT NULL DEFAULT ''
	);

	CREATE INDEX IF NOT EXISTS idx_topics_module ON topics(module);
	CREATE INDEX IF NOT EXISTS idx_progress_topic ON progress(topic_id);
	CREATE INDEX IF NOT EXISTS idx_progress_date ON progress(session_date);
	CREATE INDEX IF NOT EXISTS idx_quiz_questions_topic ON quiz_questions(topic_id);
	CREATE INDEX IF NOT EXISTS idx_spaced_rep_next ON spaced_repetition(next_review);
	CREATE INDEX IF NOT EXISTS idx_daily_activity_date ON daily_activity(date);
	CREATE INDEX IF NOT EXISTS idx_daily_activity_module ON daily_activity(module);
	CREATE INDEX IF NOT EXISTS idx_confidence_topic ON confidence_ratings(topic_id);
	CREATE INDEX IF NOT EXISTS idx_mock_scores_session ON mock_session_scores(mock_session_id);
	CREATE INDEX IF NOT EXISTS idx_question_attempts_qid ON question_attempts(quiz_question_id);
	`
	if _, err := s.db.Exec(schema); err != nil {
		return fmt.Errorf("tutor: migrate: %w", err)
	}
	return nil
}
```

- [ ] **Step 2: Add type definitions**

```go
type Topic struct {
	ID          int64  `json:"id"`
	Module      string `json:"module"`
	Category    string `json:"category"`
	Name        string `json:"name"`
	Difficulty  string `json:"difficulty"`
	ContentPath string `json:"content_path"`
	Status      string `json:"status"`
	CreatedAt   string `json:"created_at"`
}

type QuizQuestion struct {
	ID           int64  `json:"id"`
	TopicID      int64  `json:"topic_id"`
	Difficulty   string `json:"difficulty"`
	QuestionText string `json:"question_text"`
	AnswerText   string `json:"answer_text"`
	Explanation  string `json:"explanation"`
	Source       string `json:"source"`
}

type SpacedRep struct {
	ID              int64   `json:"id"`
	TopicID         int64   `json:"topic_id"`
	LastReviewed    *string `json:"last_reviewed"`
	NextReview      string  `json:"next_review"`
	IntervalDays    float64 `json:"interval_days"`
	EaseFactor      float64 `json:"ease_factor"`
	RepetitionCount int     `json:"repetition_count"`
}

type Progress struct {
	ID               int64   `json:"id"`
	TopicID          int64   `json:"topic_id"`
	SessionDate      string  `json:"session_date"`
	Score            float64 `json:"score"`
	QuestionsAsked   int     `json:"questions_asked"`
	QuestionsCorrect int     `json:"questions_correct"`
	TimeSpentSeconds int     `json:"time_spent_seconds"`
	Notes            string  `json:"notes"`
}

type DailyActivity struct {
	ID               int64   `json:"id"`
	Date             string  `json:"date"`
	Module           string  `json:"module"`
	TimeSpentSeconds int     `json:"time_spent_seconds"`
	SessionsCount    int     `json:"sessions_count"`
	QuestionsAnswered int    `json:"questions_answered"`
	ScoreAvg         float64 `json:"score_avg"`
}

type MockSession struct {
	ID             int64   `json:"id"`
	Type           string  `json:"type"`
	JobDescription string  `json:"job_description"`
	StartedAt      string  `json:"started_at"`
	CompletedAt    *string `json:"completed_at"`
	OverallScore   *float64 `json:"overall_score"`
	FeedbackJSON   string  `json:"feedback_json"`
}

type MockSessionScore struct {
	ID            int64   `json:"id"`
	MockSessionID int64   `json:"mock_session_id"`
	Dimension     string  `json:"dimension"`
	Score         float64 `json:"score"`
}

type StarStory struct {
	ID                 int64  `json:"id"`
	Competency         string `json:"competency"`
	Situation          string `json:"situation"`
	Task               string `json:"task"`
	Action             string `json:"action"`
	Result             string `json:"result"`
	ProjectsReferenced string `json:"projects_referenced"`
	Version            int    `json:"version"`
}

type StudyPlan struct {
	ID         int64  `json:"id"`
	TargetRole string `json:"target_role"`
	TargetDate string `json:"target_date"`
	CreatedAt  string `json:"created_at"`
	PlanJSON   string `json:"plan_json"`
	Active     bool   `json:"active"`
}

type QuestionAttempt struct {
	ID                int64  `json:"id"`
	QuizQuestionID    int64  `json:"quiz_question_id"`
	ProgressID        *int64 `json:"progress_id"`
	AnsweredCorrectly bool   `json:"answered_correctly"`
	TimeTakenSeconds  int    `json:"time_taken_seconds"`
	UserAnswerSummary string `json:"user_answer_summary"`
}

type ConfidenceRating struct {
	ID             int64   `json:"id"`
	TopicID        int64   `json:"topic_id"`
	SelfRatedScore float64 `json:"self_rated_score"`
	ActualScore    float64 `json:"actual_score"`
	RatedAt        string  `json:"rated_at"`
}
```

- [ ] **Step 3: Add CRUD methods for topics**

```go
// CreateTopic inserts a new topic. Returns existing if duplicate.
func (s *Store) CreateTopic(module, category, name, difficulty, contentPath string) (*Topic, error) {
	res, err := s.db.Exec(
		"INSERT OR IGNORE INTO topics (module, category, name, difficulty, content_path) VALUES (?, ?, ?, ?, ?)",
		module, category, name, difficulty, contentPath,
	)
	if err != nil {
		return nil, fmt.Errorf("tutor: create topic: %w", err)
	}
	id, _ := res.LastInsertId()
	if id == 0 {
		// Already exists — fetch it.
		return s.GetTopicByName(module, category, name)
	}
	return s.GetTopic(id)
}

func (s *Store) GetTopic(id int64) (*Topic, error) {
	var t Topic
	err := s.db.QueryRow(
		"SELECT id, module, category, name, difficulty, content_path, status, created_at FROM topics WHERE id = ?", id,
	).Scan(&t.ID, &t.Module, &t.Category, &t.Name, &t.Difficulty, &t.ContentPath, &t.Status, &t.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("tutor: topic not found: %d", id)
	}
	if err != nil {
		return nil, fmt.Errorf("tutor: get topic: %w", err)
	}
	return &t, nil
}

func (s *Store) GetTopicByName(module, category, name string) (*Topic, error) {
	var t Topic
	err := s.db.QueryRow(
		"SELECT id, module, category, name, difficulty, content_path, status, created_at FROM topics WHERE module = ? AND category = ? AND name = ?",
		module, category, name,
	).Scan(&t.ID, &t.Module, &t.Category, &t.Name, &t.Difficulty, &t.ContentPath, &t.Status, &t.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("tutor: topic not found: %s/%s/%s", module, category, name)
	}
	if err != nil {
		return nil, fmt.Errorf("tutor: get topic by name: %w", err)
	}
	return &t, nil
}

func (s *Store) ListTopics(module string) ([]Topic, error) {
	query := "SELECT id, module, category, name, difficulty, content_path, status, created_at FROM topics"
	var args []interface{}
	if module != "" {
		query += " WHERE module = ?"
		args = append(args, module)
	}
	query += " ORDER BY module, category, name"
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("tutor: list topics: %w", err)
	}
	defer rows.Close()
	var topics []Topic
	for rows.Next() {
		var t Topic
		if err := rows.Scan(&t.ID, &t.Module, &t.Category, &t.Name, &t.Difficulty, &t.ContentPath, &t.Status, &t.CreatedAt); err != nil {
			return nil, fmt.Errorf("tutor: scan topic: %w", err)
		}
		topics = append(topics, t)
	}
	return topics, rows.Err()
}

func (s *Store) UpdateTopicStatus(id int64, status string) error {
	_, err := s.db.Exec("UPDATE topics SET status = ? WHERE id = ?", status, id)
	if err != nil {
		return fmt.Errorf("tutor: update topic status: %w", err)
	}
	return nil
}
```

- [ ] **Step 4: Add CRUD for quiz_questions, progress, spaced_repetition, daily_activity**

```go
// --- Quiz Questions ---

func (s *Store) CreateQuizQuestion(topicID int64, difficulty, questionText, answerText, explanation, source string) (*QuizQuestion, error) {
	res, err := s.db.Exec(
		"INSERT INTO quiz_questions (topic_id, difficulty, question_text, answer_text, explanation, source) VALUES (?, ?, ?, ?, ?, ?)",
		topicID, difficulty, questionText, answerText, explanation, source,
	)
	if err != nil {
		return nil, fmt.Errorf("tutor: create question: %w", err)
	}
	id, _ := res.LastInsertId()
	return s.GetQuizQuestion(id)
}

func (s *Store) GetQuizQuestion(id int64) (*QuizQuestion, error) {
	var q QuizQuestion
	err := s.db.QueryRow(
		"SELECT id, topic_id, difficulty, question_text, answer_text, explanation, source FROM quiz_questions WHERE id = ?", id,
	).Scan(&q.ID, &q.TopicID, &q.Difficulty, &q.QuestionText, &q.AnswerText, &q.Explanation, &q.Source)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("tutor: question not found: %d", id)
	}
	return &q, err
}

func (s *Store) ListQuestions(topicID int64) ([]QuizQuestion, error) {
	rows, err := s.db.Query(
		"SELECT id, topic_id, difficulty, question_text, answer_text, explanation, source FROM quiz_questions WHERE topic_id = ?", topicID,
	)
	if err != nil {
		return nil, fmt.Errorf("tutor: list questions: %w", err)
	}
	defer rows.Close()
	var qs []QuizQuestion
	for rows.Next() {
		var q QuizQuestion
		if err := rows.Scan(&q.ID, &q.TopicID, &q.Difficulty, &q.QuestionText, &q.AnswerText, &q.Explanation, &q.Source); err != nil {
			return nil, err
		}
		qs = append(qs, q)
	}
	return qs, rows.Err()
}

// PickNextQuestion picks the least-attempted question for a topic.
func (s *Store) PickNextQuestion(topicID int64) (*QuizQuestion, error) {
	var q QuizQuestion
	err := s.db.QueryRow(`
		SELECT q.id, q.topic_id, q.difficulty, q.question_text, q.answer_text, q.explanation, q.source
		FROM quiz_questions q
		LEFT JOIN (SELECT quiz_question_id, COUNT(*) as cnt FROM question_attempts GROUP BY quiz_question_id) a
			ON q.id = a.quiz_question_id
		WHERE q.topic_id = ?
		ORDER BY COALESCE(a.cnt, 0) ASC, RANDOM()
		LIMIT 1
	`, topicID).Scan(&q.ID, &q.TopicID, &q.Difficulty, &q.QuestionText, &q.AnswerText, &q.Explanation, &q.Source)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("tutor: no questions for topic %d", topicID)
	}
	return &q, err
}

// --- Progress ---

func (s *Store) RecordProgress(topicID int64, score float64, asked, correct, timeSpent int, notes string) (*Progress, error) {
	res, err := s.db.Exec(
		"INSERT INTO progress (topic_id, score, questions_asked, questions_correct, time_spent_seconds, notes) VALUES (?, ?, ?, ?, ?, ?)",
		topicID, score, asked, correct, timeSpent, notes,
	)
	if err != nil {
		return nil, fmt.Errorf("tutor: record progress: %w", err)
	}
	id, _ := res.LastInsertId()
	var p Progress
	s.db.QueryRow("SELECT id, topic_id, session_date, score, questions_asked, questions_correct, time_spent_seconds, notes FROM progress WHERE id = ?", id).
		Scan(&p.ID, &p.TopicID, &p.SessionDate, &p.Score, &p.QuestionsAsked, &p.QuestionsCorrect, &p.TimeSpentSeconds, &p.Notes)
	return &p, nil
}

// RecordAttempt records a single question attempt.
func (s *Store) RecordAttempt(questionID int64, progressID *int64, correct bool, timeTaken int, answer string) error {
	correctInt := 0
	if correct {
		correctInt = 1
	}
	_, err := s.db.Exec(
		"INSERT INTO question_attempts (quiz_question_id, progress_id, answered_correctly, time_taken_seconds, user_answer_summary) VALUES (?, ?, ?, ?, ?)",
		questionID, progressID, correctInt, timeTaken, answer,
	)
	return err
}

// --- Spaced Repetition ---

func (s *Store) GetSpacedRep(topicID int64) (*SpacedRep, error) {
	var sr SpacedRep
	err := s.db.QueryRow(
		"SELECT id, topic_id, last_reviewed, next_review, interval_days, ease_factor, repetition_count FROM spaced_repetition WHERE topic_id = ?",
		topicID,
	).Scan(&sr.ID, &sr.TopicID, &sr.LastReviewed, &sr.NextReview, &sr.IntervalDays, &sr.EaseFactor, &sr.RepetitionCount)
	if err == sql.ErrNoRows {
		return nil, nil // No spaced rep state yet.
	}
	return &sr, err
}

func (s *Store) UpsertSpacedRep(topicID int64, intervalDays, easeFactor float64, repetitionCount int, nextReview time.Time) error {
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := s.db.Exec(`
		INSERT INTO spaced_repetition (topic_id, last_reviewed, next_review, interval_days, ease_factor, repetition_count)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(topic_id) DO UPDATE SET
			last_reviewed = ?, next_review = ?, interval_days = ?, ease_factor = ?, repetition_count = ?
	`, topicID, now, nextReview.UTC().Format(time.RFC3339), intervalDays, easeFactor, repetitionCount,
		now, nextReview.UTC().Format(time.RFC3339), intervalDays, easeFactor, repetitionCount,
	)
	return err
}

func (s *Store) GetDueReviews(now time.Time) ([]Topic, error) {
	rows, err := s.db.Query(`
		SELECT t.id, t.module, t.category, t.name, t.difficulty, t.content_path, t.status, t.created_at
		FROM topics t JOIN spaced_repetition sr ON t.id = sr.topic_id
		WHERE sr.next_review <= ?
		ORDER BY sr.next_review ASC
	`, now.UTC().Format(time.RFC3339))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var topics []Topic
	for rows.Next() {
		var t Topic
		rows.Scan(&t.ID, &t.Module, &t.Category, &t.Name, &t.Difficulty, &t.ContentPath, &t.Status, &t.CreatedAt)
		topics = append(topics, t)
	}
	return topics, rows.Err()
}

// --- Daily Activity ---

func (s *Store) UpsertDailyActivity(date, module string, timeSpent, questionsAnswered int, score float64) error {
	_, err := s.db.Exec(`
		INSERT INTO daily_activity (date, module, time_spent_seconds, sessions_count, questions_answered, score_avg)
		VALUES (?, ?, ?, 1, ?, ?)
		ON CONFLICT(date, module) DO UPDATE SET
			time_spent_seconds = time_spent_seconds + ?,
			sessions_count = sessions_count + 1,
			questions_answered = questions_answered + ?,
			score_avg = (score_avg * sessions_count + ?) / (sessions_count + 1)
	`, date, module, timeSpent, questionsAnswered, score, timeSpent, questionsAnswered, score)
	return err
}

func (s *Store) GetActivity(startDate, endDate string) ([]DailyActivity, error) {
	rows, err := s.db.Query(
		"SELECT id, date, module, time_spent_seconds, sessions_count, questions_answered, score_avg FROM daily_activity WHERE date >= ? AND date <= ? ORDER BY date DESC",
		startDate, endDate,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var activities []DailyActivity
	for rows.Next() {
		var a DailyActivity
		rows.Scan(&a.ID, &a.Date, &a.Module, &a.TimeSpentSeconds, &a.SessionsCount, &a.QuestionsAnswered, &a.ScoreAvg)
		activities = append(activities, a)
	}
	return activities, rows.Err()
}

func (s *Store) GetStreak(today string) (int, error) {
	rows, err := s.db.Query(
		"SELECT DISTINCT date FROM daily_activity WHERE date <= ? ORDER BY date DESC", today,
	)
	if err != nil {
		return 0, err
	}
	defer rows.Close()
	streak := 0
	expected := today
	for rows.Next() {
		var date string
		rows.Scan(&date)
		if date == expected {
			streak++
			t, _ := time.Parse("2006-01-02", expected)
			expected = t.AddDate(0, 0, -1).Format("2006-01-02")
		} else {
			break
		}
	}
	return streak, nil
}
```

- [ ] **Step 5: Add CRUD for mock_sessions, star_stories, study_plans, confidence_ratings, module stats**

```go
// --- Mock Sessions ---

func (s *Store) CreateMockSession(sessionType, jobDescription string) (*MockSession, error) {
	res, err := s.db.Exec(
		"INSERT INTO mock_sessions (type, job_description) VALUES (?, ?)", sessionType, jobDescription,
	)
	if err != nil {
		return nil, fmt.Errorf("tutor: create mock session: %w", err)
	}
	id, _ := res.LastInsertId()
	return s.GetMockSession(id)
}

func (s *Store) GetMockSession(id int64) (*MockSession, error) {
	var m MockSession
	err := s.db.QueryRow(
		"SELECT id, type, job_description, started_at, completed_at, overall_score, feedback_json FROM mock_sessions WHERE id = ?", id,
	).Scan(&m.ID, &m.Type, &m.JobDescription, &m.StartedAt, &m.CompletedAt, &m.OverallScore, &m.FeedbackJSON)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("tutor: mock session not found: %d", id)
	}
	return &m, err
}

func (s *Store) ListMockSessions() ([]MockSession, error) {
	rows, err := s.db.Query(
		"SELECT id, type, job_description, started_at, completed_at, overall_score, feedback_json FROM mock_sessions ORDER BY started_at DESC",
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var sessions []MockSession
	for rows.Next() {
		var m MockSession
		rows.Scan(&m.ID, &m.Type, &m.JobDescription, &m.StartedAt, &m.CompletedAt, &m.OverallScore, &m.FeedbackJSON)
		sessions = append(sessions, m)
	}
	return sessions, rows.Err()
}

func (s *Store) CompleteMockSession(id int64, overallScore float64, feedbackJSON string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := s.db.Exec(
		"UPDATE mock_sessions SET completed_at = ?, overall_score = ?, feedback_json = ? WHERE id = ?",
		now, overallScore, feedbackJSON, id,
	)
	return err
}

func (s *Store) AddMockScore(mockSessionID int64, dimension string, score float64) error {
	_, err := s.db.Exec(
		"INSERT INTO mock_session_scores (mock_session_id, dimension, score) VALUES (?, ?, ?)",
		mockSessionID, dimension, score,
	)
	return err
}

func (s *Store) GetMockScores(mockSessionID int64) ([]MockSessionScore, error) {
	rows, err := s.db.Query(
		"SELECT id, mock_session_id, dimension, score FROM mock_session_scores WHERE mock_session_id = ?", mockSessionID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var scores []MockSessionScore
	for rows.Next() {
		var sc MockSessionScore
		rows.Scan(&sc.ID, &sc.MockSessionID, &sc.Dimension, &sc.Score)
		scores = append(scores, sc)
	}
	return scores, rows.Err()
}

// --- STAR Stories ---

func (s *Store) GetStarStory(competency string) (*StarStory, error) {
	var st StarStory
	err := s.db.QueryRow(
		"SELECT id, competency, situation, task, action, result, projects_referenced, version FROM star_stories WHERE competency = ? ORDER BY version DESC LIMIT 1",
		competency,
	).Scan(&st.ID, &st.Competency, &st.Situation, &st.Task, &st.Action, &st.Result, &st.ProjectsReferenced, &st.Version)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return &st, err
}

func (s *Store) UpsertStarStory(competency, situation, task, action, result, projects string) error {
	_, err := s.db.Exec(`
		INSERT INTO star_stories (competency, situation, task, action, result, projects_referenced)
		VALUES (?, ?, ?, ?, ?, ?)
	`, competency, situation, task, action, result, projects)
	return err
}

// --- Study Plans ---

func (s *Store) GetActivePlan() (*StudyPlan, error) {
	var p StudyPlan
	var active int
	err := s.db.QueryRow(
		"SELECT id, target_role, target_date, created_at, plan_json, active FROM study_plans WHERE active = 1 ORDER BY created_at DESC LIMIT 1",
	).Scan(&p.ID, &p.TargetRole, &p.TargetDate, &p.CreatedAt, &p.PlanJSON, &active)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	p.Active = active == 1
	return &p, err
}

func (s *Store) CreatePlan(targetRole, targetDate, planJSON string) (*StudyPlan, error) {
	// Deactivate existing plans.
	s.db.Exec("UPDATE study_plans SET active = 0 WHERE active = 1")
	res, err := s.db.Exec(
		"INSERT INTO study_plans (target_role, target_date, plan_json) VALUES (?, ?, ?)",
		targetRole, targetDate, planJSON,
	)
	if err != nil {
		return nil, fmt.Errorf("tutor: create plan: %w", err)
	}
	id, _ := res.LastInsertId()
	var p StudyPlan
	var active int
	s.db.QueryRow("SELECT id, target_role, target_date, created_at, plan_json, active FROM study_plans WHERE id = ?", id).
		Scan(&p.ID, &p.TargetRole, &p.TargetDate, &p.CreatedAt, &p.PlanJSON, &active)
	p.Active = active == 1
	return &p, nil
}

func (s *Store) UpdatePlan(id int64, planJSON string) error {
	_, err := s.db.Exec("UPDATE study_plans SET plan_json = ? WHERE id = ?", planJSON, id)
	return err
}

// --- Confidence Ratings ---

func (s *Store) AddConfidenceRating(topicID int64, selfRated, actual float64) error {
	_, err := s.db.Exec(
		"INSERT INTO confidence_ratings (topic_id, self_rated_score, actual_score) VALUES (?, ?, ?)",
		topicID, selfRated, actual,
	)
	return err
}

func (s *Store) GetConfidenceGaps(minGap float64) ([]ConfidenceRating, error) {
	rows, err := s.db.Query(`
		SELECT id, topic_id, self_rated_score, actual_score, rated_at
		FROM confidence_ratings
		WHERE (self_rated_score - actual_score) >= ?
		ORDER BY (self_rated_score - actual_score) DESC
		LIMIT 20
	`, minGap)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ratings []ConfidenceRating
	for rows.Next() {
		var r ConfidenceRating
		rows.Scan(&r.ID, &r.TopicID, &r.SelfRatedScore, &r.ActualScore, &r.RatedAt)
		ratings = append(ratings, r)
	}
	return ratings, rows.Err()
}

// --- Module Stats ---

type ModuleStats struct {
	Module      string  `json:"module"`
	Total       int     `json:"total"`
	NotStarted  int     `json:"not_started"`
	Learning    int     `json:"learning"`
	Drilling    int     `json:"drilling"`
	Mastered    int     `json:"mastered"`
	AvgScore    float64 `json:"avg_score"`
	TotalTime   int     `json:"total_time"`
	Completion  float64 `json:"completion"`
}

func (s *Store) GetModuleStats(module string) (*ModuleStats, error) {
	ms := &ModuleStats{Module: module}
	s.db.QueryRow("SELECT COUNT(*) FROM topics WHERE module = ?", module).Scan(&ms.Total)
	s.db.QueryRow("SELECT COUNT(*) FROM topics WHERE module = ? AND status = 'not_started'", module).Scan(&ms.NotStarted)
	s.db.QueryRow("SELECT COUNT(*) FROM topics WHERE module = ? AND status = 'learning'", module).Scan(&ms.Learning)
	s.db.QueryRow("SELECT COUNT(*) FROM topics WHERE module = ? AND status = 'drilling'", module).Scan(&ms.Drilling)
	s.db.QueryRow("SELECT COUNT(*) FROM topics WHERE module = ? AND status = 'mastered'", module).Scan(&ms.Mastered)
	s.db.QueryRow("SELECT COALESCE(AVG(score), 0) FROM progress WHERE topic_id IN (SELECT id FROM topics WHERE module = ?)", module).Scan(&ms.AvgScore)
	s.db.QueryRow("SELECT COALESCE(SUM(time_spent_seconds), 0) FROM progress WHERE topic_id IN (SELECT id FROM topics WHERE module = ?)", module).Scan(&ms.TotalTime)
	if ms.Total > 0 {
		ms.Completion = float64(ms.Mastered) / float64(ms.Total) * 100
	}
	return ms, nil
}

// GetTodayActivity returns aggregated activity for today across all modules.
func (s *Store) GetTodayActivity(today string) (*DailyActivity, error) {
	var a DailyActivity
	a.Date = today
	err := s.db.QueryRow(`
		SELECT COALESCE(SUM(time_spent_seconds), 0), COALESCE(SUM(sessions_count), 0),
		       COALESCE(SUM(questions_answered), 0), COALESCE(AVG(score_avg), 0)
		FROM daily_activity WHERE date = ?
	`, today).Scan(&a.TimeSpentSeconds, &a.SessionsCount, &a.QuestionsAnswered, &a.ScoreAvg)
	return &a, err
}
```

- [ ] **Step 6: Write store tests**

Create `internal/tutor/store/store_test.go`:

```go
package store

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func testStore(t *testing.T) *Store {
	t.Helper()
	dir := t.TempDir()
	s, err := Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestOpenAndMigrate(t *testing.T) {
	s := testStore(t)
	// Verify tables exist by inserting into each.
	_, err := s.CreateTopic("dsa", "arrays", "two-sum", "easy", "")
	if err != nil {
		t.Fatalf("create topic: %v", err)
	}
}

func TestTopicCRUD(t *testing.T) {
	s := testStore(t)
	topic, err := s.CreateTopic("dsa", "arrays", "two-sum", "easy", "")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if topic.Module != "dsa" || topic.Name != "two-sum" {
		t.Errorf("got %+v", topic)
	}

	// Duplicate returns existing.
	dup, err := s.CreateTopic("dsa", "arrays", "two-sum", "easy", "")
	if err != nil {
		t.Fatalf("duplicate: %v", err)
	}
	if dup.ID != topic.ID {
		t.Errorf("expected same ID, got %d vs %d", dup.ID, topic.ID)
	}

	// List.
	topics, _ := s.ListTopics("dsa")
	if len(topics) != 1 {
		t.Errorf("expected 1 topic, got %d", len(topics))
	}

	// Update status.
	s.UpdateTopicStatus(topic.ID, "learning")
	updated, _ := s.GetTopic(topic.ID)
	if updated.Status != "learning" {
		t.Errorf("expected learning, got %s", updated.Status)
	}
}

func TestQuizAndDrill(t *testing.T) {
	s := testStore(t)
	topic, _ := s.CreateTopic("dsa", "arrays", "two-sum", "easy", "")
	q, err := s.CreateQuizQuestion(topic.ID, "easy", "What is O(1)?", "Constant time", "Explanation", "test")
	if err != nil {
		t.Fatalf("create question: %v", err)
	}

	picked, _ := s.PickNextQuestion(topic.ID)
	if picked.ID != q.ID {
		t.Errorf("expected question %d, got %d", q.ID, picked.ID)
	}

	s.RecordAttempt(q.ID, nil, true, 5, "Constant time")
}

func TestSpacedRepetition(t *testing.T) {
	s := testStore(t)
	topic, _ := s.CreateTopic("dsa", "arrays", "two-sum", "easy", "")

	next := time.Now().Add(24 * time.Hour)
	s.UpsertSpacedRep(topic.ID, 1.0, 2.5, 1, next)

	sr, _ := s.GetSpacedRep(topic.ID)
	if sr == nil || sr.RepetitionCount != 1 {
		t.Errorf("expected rep count 1, got %v", sr)
	}

	due, _ := s.GetDueReviews(time.Now().Add(48 * time.Hour))
	if len(due) != 1 {
		t.Errorf("expected 1 due, got %d", len(due))
	}
}

func TestModuleStats(t *testing.T) {
	s := testStore(t)
	s.CreateTopic("dsa", "arrays", "a", "easy", "")
	s.CreateTopic("dsa", "arrays", "b", "easy", "")
	s.UpdateTopicStatus(1, "mastered")

	stats, _ := s.GetModuleStats("dsa")
	if stats.Total != 2 || stats.Mastered != 1 || stats.Completion != 50 {
		t.Errorf("unexpected stats: %+v", stats)
	}
}

func TestStreak(t *testing.T) {
	s := testStore(t)
	today := time.Now().Format("2006-01-02")
	yesterday := time.Now().AddDate(0, 0, -1).Format("2006-01-02")
	s.UpsertDailyActivity(today, "dsa", 60, 5, 80)
	s.UpsertDailyActivity(yesterday, "dsa", 60, 5, 80)

	streak, _ := s.GetStreak(today)
	if streak != 2 {
		t.Errorf("expected streak 2, got %d", streak)
	}
}

func TestMockSession(t *testing.T) {
	s := testStore(t)
	session, err := s.CreateMockSession("technical", "Build a REST API")
	if err != nil {
		t.Fatalf("create mock session: %v", err)
	}
	if session.Type != "technical" {
		t.Errorf("expected technical, got %s", session.Type)
	}
	s.AddMockScore(session.ID, "problem_solving", 85)
	scores, _ := s.GetMockScores(session.ID)
	if len(scores) != 1 || scores[0].Score != 85 {
		t.Errorf("unexpected scores: %+v", scores)
	}
}

func TestStudyPlan(t *testing.T) {
	s := testStore(t)
	plan, _ := s.CreatePlan("SDE-2", "2026-04-01", `{"phases":[]}`)
	if plan.TargetRole != "SDE-2" || !plan.Active {
		t.Errorf("unexpected plan: %+v", plan)
	}

	active, _ := s.GetActivePlan()
	if active == nil || active.ID != plan.ID {
		t.Errorf("expected active plan %d", plan.ID)
	}

	// New plan deactivates old.
	plan2, _ := s.CreatePlan("SDE-3", "2026-05-01", `{"phases":[]}`)
	old, _ := s.GetActivePlan()
	if old.ID != plan2.ID {
		t.Errorf("expected plan2 active, got %d", old.ID)
	}
}
```

- [ ] **Step 7: Run tests**

Run: `go test -race -count=1 ./internal/tutor/store/...`
Expected: All PASS

- [ ] **Step 8: Commit**

```bash
git add internal/tutor/store/
git commit -m "feat(tutor): add store layer with 11-table schema and CRUD"
```

---

### Task 2: SM-2 Spaced Repetition Algorithm

**Files:**
- Create: `internal/tutor/modules/sm2.go`
- Test: `internal/tutor/modules/sm2_test.go`

- [ ] **Step 1: Implement SM-2**

Create `internal/tutor/modules/sm2.go`:

```go
package modules

import (
	"math"
	"time"
)

// SM2Result holds the updated spaced repetition state after a review.
type SM2Result struct {
	IntervalDays    float64
	EaseFactor      float64
	RepetitionCount int
	NextReview      time.Time
}

// SM2Update applies the SM-2 algorithm for a single review.
// quality: 0-5 (0=blackout, 5=perfect).
func SM2Update(quality int, currentInterval float64, currentEF float64, currentReps int) SM2Result {
	if quality < 0 {
		quality = 0
	}
	if quality > 5 {
		quality = 5
	}

	var interval float64
	var reps int
	ef := currentEF

	if quality < 3 {
		// Failed — reset.
		interval = 1
		reps = 0
	} else {
		reps = currentReps + 1
		switch reps {
		case 1:
			interval = 1
		case 2:
			interval = 6
		default:
			interval = math.Round(currentInterval * ef)
		}
	}

	// Update ease factor.
	ef = ef + (0.1 - float64(5-quality)*(0.08+float64(5-quality)*0.02))
	if ef < 1.3 {
		ef = 1.3
	}

	return SM2Result{
		IntervalDays:    interval,
		EaseFactor:      ef,
		RepetitionCount: reps,
		NextReview:      time.Now().Add(time.Duration(interval*24) * time.Hour),
	}
}
```

- [ ] **Step 2: Write SM-2 tests**

Create `internal/tutor/modules/sm2_test.go`:

```go
package modules

import "testing"

func TestSM2_FailedResets(t *testing.T) {
	r := SM2Update(1, 6, 2.5, 3)
	if r.RepetitionCount != 0 || r.IntervalDays != 1 {
		t.Errorf("failed should reset: %+v", r)
	}
}

func TestSM2_FirstSuccess(t *testing.T) {
	r := SM2Update(4, 0, 2.5, 0)
	if r.IntervalDays != 1 || r.RepetitionCount != 1 {
		t.Errorf("first success: %+v", r)
	}
}

func TestSM2_SecondSuccess(t *testing.T) {
	r := SM2Update(4, 1, 2.5, 1)
	if r.IntervalDays != 6 || r.RepetitionCount != 2 {
		t.Errorf("second success: %+v", r)
	}
}

func TestSM2_ThirdSuccess(t *testing.T) {
	r := SM2Update(4, 6, 2.5, 2)
	// interval = round(6 * 2.5) = 15
	if r.IntervalDays != 15 || r.RepetitionCount != 3 {
		t.Errorf("third success: %+v", r)
	}
}

func TestSM2_EaseFloor(t *testing.T) {
	// Repeated low quality should floor at 1.3.
	r := SM2Update(3, 1, 1.3, 0)
	if r.EaseFactor < 1.3 {
		t.Errorf("ease below floor: %f", r.EaseFactor)
	}
}

func TestSM2_PerfectRaises(t *testing.T) {
	r := SM2Update(5, 1, 2.5, 0)
	if r.EaseFactor <= 2.5 {
		t.Errorf("perfect should raise EF: %f", r.EaseFactor)
	}
}

func TestSM2_ClampedInput(t *testing.T) {
	r := SM2Update(-1, 1, 2.5, 0)
	if r.RepetitionCount != 0 {
		t.Errorf("negative quality should fail: %+v", r)
	}
	r2 := SM2Update(10, 1, 2.5, 0)
	if r2.EaseFactor < 2.5 {
		t.Errorf("clamped quality should raise: %f", r2.EaseFactor)
	}
}
```

- [ ] **Step 3: Run tests**

Run: `go test -race -count=1 ./internal/tutor/modules/...`
Expected: All PASS

- [ ] **Step 4: Commit**

```bash
git add internal/tutor/modules/sm2.go internal/tutor/modules/sm2_test.go
git commit -m "feat(tutor): add SM-2 spaced repetition algorithm"
```

---

### Task 3: DSA, AI, Behavioral Modules

**Files:**
- Create: `internal/tutor/modules/dsa.go`
- Create: `internal/tutor/modules/ai.go`
- Create: `internal/tutor/modules/behavioral.go`

These modules follow the same pattern: they accept a store reference and return structured results. Port logic from v1 (`/home/rishav/soul/products/tutor/internal/modules/`).

- [ ] **Step 1: Create DSA module**

Create `internal/tutor/modules/dsa.go`. Key methods:
- `Learn(topicID)` — fetch content, mark as learning, return content + metadata
- `Build(topic)` — return 5-step implementation guide (Interface → Core → Edge → Tests → Analysis)
- `Drill(topicID)` — pick next question (SM-2), return it OR evaluate answer + update SM-2
- `Solve(topic)` — return 4-step walkthrough (Understand → Pattern → Solve → Analyze)
- `GenerateContent(module, category, name, difficulty)` — create topic + write markdown file

Each method accepts JSON input and returns a structured result. Port business logic from v1 `products/tutor/internal/modules/dsa/dsa.go`.

Key patterns:
- Drill picks question via `store.PickNextQuestion(topicID)`
- Answer evaluation: normalize strings, check keyword overlap ≥ 50%
- On answer: `store.RecordAttempt()`, `store.RecordProgress()`, `store.UpsertDailyActivity()`, `SM2Update()` → `store.UpsertSpacedRep()`
- Content stored at `~/.soul-v2/tutor/content/dsa/{category}/{name}.md`

- [ ] **Step 2: Create AI module**

Create `internal/tutor/modules/ai.go`. Port from v1 `products/tutor/internal/modules/ai/ai.go`:
- `LearnTheory(topic, depth)` — auto-create topic if missing, return theory content at depth level
- `DrillTheory(topicID)` — same drill pattern as DSA
- `GenerateAIContent(category, name)` — return 6-section outline

- [ ] **Step 3: Create Behavioral module**

Create `internal/tutor/modules/behavioral.go`. Port from v1 `products/tutor/internal/modules/behavioral/behavioral.go`:
- `BuildNarrative(focus)` — "Tell me about yourself" template with 5 sections
- `BuildStar(competency)` — validate competency, return existing story or template
- `DrillHR(category)` — return question from predefined bank, or evaluate answer

Valid competencies: leadership, conflict, failure, teamwork, innovation, ownership.
HR categories: salary, gaps, weaknesses, conflict, motivation, why_leaving, where_5_years.

- [ ] **Step 4: Run compile check**

Run: `go build ./internal/tutor/...`
Expected: No errors

- [ ] **Step 5: Commit**

```bash
git add internal/tutor/modules/dsa.go internal/tutor/modules/ai.go internal/tutor/modules/behavioral.go
git commit -m "feat(tutor): add DSA, AI, and Behavioral modules"
```

---

### Task 4: Mock, Planner, Importer, Progress Modules

**Files:**
- Create: `internal/tutor/modules/mock.go`
- Create: `internal/tutor/modules/planner.go`
- Create: `internal/tutor/modules/importer.go`
- Create: `internal/tutor/modules/progress.go`

- [ ] **Step 1: Create Mock module**

Port from v1 `products/tutor/internal/modules/mock/mock.go`:
- `MockInterview(sessionType, jobDescription)` — generate 3-5 questions, store session
- `AnalyzeJD(text)` — extract skills via regex, map to modules, identify gaps

Question generation by type:
- `technical`: 3 coding + system design questions
- `behavioral`: 4-6 STAR questions
- `full_loop`: 2 tech + 2 behavioral + 1 HR

Skill extraction: regex keyword matching against ~30 skill categories → module mapping.

- [ ] **Step 2: Create Planner module**

Port from v1 `products/tutor/internal/modules/planner/planner.go`:
- `CreatePlan(targetRole, targetDate)` — validate inputs, gather stats, generate phases
- `UpdatePlan(planID)` — recalculate phases based on current progress
- `GetPlan()` — return active plan with readiness overlay

Phase strategy:
- ≤7 days: 1 sprint phase
- 8-30 days: 2 phases (foundation + polish)
- 31+ days: 3 phases (learn → drill → mock)

- [ ] **Step 3: Create Importer**

Port from v1 `products/tutor/internal/importer/importer.go`:
- `ImportStdlib(contentDir)` — reads Python files from `~/interview-prep/week1/stdlib_cheatsheet/`
- Copies `.py` files to `{contentDir}/dsa/stdlib/`
- Parses `EVALUATION_QUESTIONS` list using regex
- Creates topics + quiz questions in store
- Returns stats (topics_created, questions_imported, files_copied)

- [ ] **Step 4: Create Progress module**

Port from v1 `products/tutor/service.go` progress handlers:
- `Dashboard()` — readiness %, module stats for DSA/AI/Behavioral, streak, due reviews, today's activity
- `Analytics(startDate, endDate)` — 30-day activity + confidence gaps
- `Topics(module)` — topic list with optional filter
- `Mocks()` — mock session list

Readiness = average of module completion percentages.

- [ ] **Step 5: Run compile check**

Run: `go build ./internal/tutor/...`
Expected: No errors

- [ ] **Step 6: Commit**

```bash
git add internal/tutor/modules/mock.go internal/tutor/modules/planner.go internal/tutor/modules/importer.go internal/tutor/modules/progress.go
git commit -m "feat(tutor): add Mock, Planner, Importer, and Progress modules"
```

---

## Chunk 2: HTTP Server + Chat Integration

### Task 5: Tutor HTTP Server

**Files:**
- Create: `internal/tutor/server/server.go`
- Create: `internal/tutor/server/handlers.go`
- Create: `internal/tutor/server/tools.go`

- [ ] **Step 1: Create server skeleton**

Create `internal/tutor/server/server.go` following `internal/tasks/server/server.go` pattern exactly:

```go
package server

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/rishav1305/soul-v2/internal/tutor/store"
	"github.com/rishav1305/soul-v2/internal/tutor/modules"
)

type Server struct {
	store      *store.Store
	modules    *modules.Registry
	mux        *http.ServeMux
	httpServer *http.Server
	host       string
	port       int
	startTime  time.Time
	contentDir string
}

type Option func(*Server)

func WithStore(s *store.Store) Option    { return func(srv *Server) { srv.store = s } }
func WithHost(h string) Option           { return func(srv *Server) { srv.host = h } }
func WithPort(p int) Option              { return func(srv *Server) { srv.port = p } }
func WithContentDir(d string) Option     { return func(srv *Server) { srv.contentDir = d } }

func New(opts ...Option) *Server {
	s := &Server{
		mux:  http.NewServeMux(),
		host: "127.0.0.1",
		port: 3006,
	}
	for _, opt := range opts {
		opt(s)
	}

	// Initialize module registry.
	s.modules = modules.NewRegistry(s.store, s.contentDir)

	// Register routes.
	s.mux.HandleFunc("GET /api/health", s.handleHealth)

	// Dashboard & progress.
	s.mux.HandleFunc("GET /api/tutor/dashboard", s.handleDashboard)
	s.mux.HandleFunc("GET /api/tutor/analytics", s.handleAnalytics)
	s.mux.HandleFunc("GET /api/tutor/topics", s.handleListTopics)
	s.mux.HandleFunc("GET /api/tutor/topics/{id}", s.handleGetTopic)

	// Drill.
	s.mux.HandleFunc("POST /api/tutor/drill/start", s.handleDrillStart)
	s.mux.HandleFunc("POST /api/tutor/drill/answer", s.handleDrillAnswer)
	s.mux.HandleFunc("GET /api/tutor/drill/due", s.handleDrillDue)

	// Mocks.
	s.mux.HandleFunc("GET /api/tutor/mocks", s.handleListMocks)
	s.mux.HandleFunc("POST /api/tutor/mocks", s.handleCreateMock)
	s.mux.HandleFunc("GET /api/tutor/mocks/{id}", s.handleGetMock)
	s.mux.HandleFunc("POST /api/tutor/mocks/{id}/answer", s.handleMockAnswer)

	// Planner.
	s.mux.HandleFunc("GET /api/tutor/plan", s.handleGetPlan)
	s.mux.HandleFunc("POST /api/tutor/plan", s.handleCreatePlan)
	s.mux.HandleFunc("PATCH /api/tutor/plan", s.handleUpdatePlan)

	// Content.
	s.mux.HandleFunc("POST /api/tutor/import", s.handleImport)

	// Chat tool execution.
	s.mux.HandleFunc("POST /api/tools/{name}/execute", s.handleToolExecute)

	// Build middleware chain.
	handler := http.Handler(s.mux)
	handler = bodyLimitMiddleware(64 << 10)(handler)
	handler = cspMiddleware(handler)
	handler = recoveryMiddleware(handler)

	s.httpServer = &http.Server{
		Addr:              net.JoinHostPort(s.host, strconv.Itoa(s.port)),
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
	}

	return s
}

func (s *Server) Start() error {
	s.startTime = time.Now()
	log.Printf("tutor server listening on %s:%d", s.host, s.port)
	return s.httpServer.ListenAndServe()
}

func (s *Server) Shutdown(ctx context.Context) error {
	return s.httpServer.Shutdown(ctx)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status": "ok",
		"uptime": time.Since(s.startTime).Round(time.Second).String(),
	})
}

// Middleware and helpers — copy from tasks server pattern.
func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func bodyLimitMiddleware(maxBytes int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
			next.ServeHTTP(w, r)
		})
	}
}

func cspMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'")
		next.ServeHTTP(w, r)
	})
}

func recoveryMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				log.Printf("panic recovered: %v", err)
				http.Error(w, "internal server error", http.StatusInternalServerError)
			}
		}()
		next.ServeHTTP(w, r)
	})
}
```

- [ ] **Step 2: Create Module Registry**

The modules need a shared registry that wires store access. Add to `internal/tutor/modules/registry.go`:

```go
package modules

import "github.com/rishav1305/soul-v2/internal/tutor/store"

type Registry struct {
	Store      *store.Store
	ContentDir string
	DSA        *DSAModule
	AI         *AIModule
	Behavioral *BehavioralModule
	Mock       *MockModule
	Planner    *PlannerModule
	Progress   *ProgressModule
}

func NewRegistry(s *store.Store, contentDir string) *Registry {
	return &Registry{
		Store:      s,
		ContentDir: contentDir,
		DSA:        &DSAModule{store: s, contentDir: contentDir},
		AI:         &AIModule{store: s, contentDir: contentDir},
		Behavioral: &BehavioralModule{store: s},
		Mock:       &MockModule{store: s},
		Planner:    &PlannerModule{store: s},
		Progress:   &ProgressModule{store: s},
	}
}
```

- [ ] **Step 3: Implement handlers**

Create `internal/tutor/server/handlers.go` with all REST API handler implementations. Each handler:
1. Parses request (query params or JSON body)
2. Calls the appropriate module method
3. Returns JSON response
4. Logs execution time for Transparent pillar

- [ ] **Step 4: Implement tool execution endpoint**

Create `internal/tutor/server/tools.go`:

```go
package server

import (
	"encoding/json"
	"log"
	"net/http"
	"time"
)

// handleToolExecute routes chat tool calls to the appropriate module method.
func (s *Server) handleToolExecute(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	start := time.Now()

	var input map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}

	result, err := s.executeTool(name, input)
	elapsed := time.Since(start)
	log.Printf("[tutor] tool=%s latency=%s err=%v", name, elapsed, err)

	if err != nil {
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"success":         true,
		"output":          result.Summary,
		"structured_json": result.Data,
	})
}

type ToolResult struct {
	Summary string      `json:"summary"`
	Data    interface{} `json:"data"`
}

func (s *Server) executeTool(name string, input map[string]interface{}) (*ToolResult, error) {
	switch name {
	// DSA
	case "learn":
		return s.modules.DSA.Learn(input)
	case "build":
		return s.modules.DSA.Build(input)
	case "drill":
		return s.modules.DSA.Drill(input)
	case "solve":
		return s.modules.DSA.Solve(input)
	case "generate_content":
		return s.modules.DSA.GenerateContent(input)
	// AI
	case "learn_theory":
		return s.modules.AI.LearnTheory(input)
	case "drill_theory":
		return s.modules.AI.DrillTheory(input)
	case "generate_ai_content":
		return s.modules.AI.GenerateAIContent(input)
	// Behavioral
	case "build_narrative":
		return s.modules.Behavioral.BuildNarrative(input)
	case "build_star":
		return s.modules.Behavioral.BuildStar(input)
	case "drill_hr":
		return s.modules.Behavioral.DrillHR(input)
	// Mock
	case "mock_interview":
		return s.modules.Mock.MockInterview(input)
	case "analyze_jd":
		return s.modules.Mock.AnalyzeJD(input)
	// Planner
	case "plan":
		return s.modules.Planner.Plan(input)
	// Progress
	case "progress":
		return s.modules.Progress.Progress(input)
	default:
		return nil, fmt.Errorf("unknown tool: %s", name)
	}
}
```

- [ ] **Step 5: Run compile check**

Run: `go build ./internal/tutor/...`
Expected: No errors

- [ ] **Step 6: Commit**

```bash
git add internal/tutor/server/ internal/tutor/modules/registry.go
git commit -m "feat(tutor): add HTTP server with REST API and tool execution"
```

---

### Task 6: cmd/tutor Binary

**Files:**
- Create: `cmd/tutor/main.go`

- [ ] **Step 1: Create main.go**

Follow `cmd/tasks/main.go` pattern exactly:

```go
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"syscall"

	"github.com/rishav1305/soul-v2/internal/tutor/modules"
	"github.com/rishav1305/soul-v2/internal/tutor/server"
	"github.com/rishav1305/soul-v2/internal/tutor/store"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("usage: soul-tutor <command>")
		fmt.Println("commands: serve")
		os.Exit(1)
	}
	switch os.Args[1] {
	case "serve":
		runServe()
	default:
		fmt.Printf("unknown command: %s\n", os.Args[1])
		os.Exit(1)
	}
}

func runServe() {
	dataDir := os.Getenv("SOUL_V2_DATA_DIR")
	if dataDir == "" {
		home, _ := os.UserHomeDir()
		dataDir = filepath.Join(home, ".soul-v2")
	}
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		log.Fatalf("create data dir: %v", err)
	}

	// Content directory for study files.
	contentDir := filepath.Join(dataDir, "tutor", "content")
	if err := os.MkdirAll(contentDir, 0755); err != nil {
		log.Fatalf("create content dir: %v", err)
	}

	// Open store.
	dbPath := filepath.Join(dataDir, "tutor.db")
	tutorStore, err := store.Open(dbPath)
	if err != nil {
		log.Fatalf("open tutor store: %v", err)
	}
	defer tutorStore.Close()

	// Auto-import on first run (if DB is empty).
	topics, _ := tutorStore.ListTopics("")
	if len(topics) == 0 {
		imp := modules.NewImporter(tutorStore, contentDir)
		stats, err := imp.ImportStdlib()
		if err != nil {
			log.Printf("auto-import: %v (continuing without content)", err)
		} else {
			log.Printf("auto-import: %d topics, %d questions, %d files", stats.TopicsCreated, stats.QuestionsImported, stats.FilesCopied)
		}
	}

	host := os.Getenv("SOUL_TUTOR_HOST")
	if host == "" {
		host = "127.0.0.1"
	}
	port := 3006
	if p := os.Getenv("SOUL_TUTOR_PORT"); p != "" {
		if parsed, err := strconv.Atoi(p); err == nil {
			port = parsed
		}
	}

	srv := server.New(
		server.WithStore(tutorStore),
		server.WithHost(host),
		server.WithPort(port),
		server.WithContentDir(contentDir),
	)

	// Graceful shutdown.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-sigCh
		log.Printf("received %s, shutting down...", sig)
		srv.Shutdown(context.Background())
	}()

	if err := srv.Start(); err != nil && err.Error() != "http: Server closed" {
		log.Fatalf("tutor server error: %v", err)
	}
}
```

- [ ] **Step 2: Build binary**

Run: `go build -o soul-tutor ./cmd/tutor`
Expected: Binary created at `./soul-tutor`

- [ ] **Step 3: Smoke test**

Run: `./soul-tutor serve &` then `curl -s http://localhost:3006/api/health | jq .`
Expected: `{"status": "ok", "uptime": "0s"}`
Kill: `kill %1`

- [ ] **Step 4: Commit**

```bash
git add cmd/tutor/
git commit -m "feat(tutor): add cmd/tutor server binary"
```

---

### Task 7: Chat Server Proxy Integration

**Files:**
- Modify: `internal/chat/server/proxy.go` — add `newTutorProxy`, `WithTutorProxy`
- Modify: `internal/chat/server/server.go` — register `/api/tutor/` proxy routes
- Modify: `cmd/chat/main.go` — wire tutor proxy option

- [ ] **Step 1: Add tutor proxy to proxy.go**

Add `newTutorProxy()` function following the exact `newTasksProxy()` pattern but with `SOUL_TUTOR_URL` env var (default `http://127.0.0.1:3006`). The error message should say "tutor server unavailable".

Add `WithTutorProxy(hub)` option function.

Add tutor proxy field to the Server struct.

- [ ] **Step 2: Register proxy routes in server.go**

After the tasks proxy block, add:
```go
if s.tutorProxy != nil {
	s.mux.Handle("/api/tutor/", s.tutorProxy)
	s.mux.Handle("/api/tutor", s.tutorProxy)
}
```

- [ ] **Step 3: Wire in cmd/chat/main.go**

Add `server.WithTutorProxy(hub)` to the server options, after the tasks proxy line.

No SSE relay needed for Tutor (it doesn't broadcast real-time events).

- [ ] **Step 4: Build and verify**

Run: `go build -o soul-chat ./cmd/chat && go build -o soul-tutor ./cmd/tutor`
Run both servers, then: `curl -s http://localhost:3002/api/tutor/dashboard`
Expected: Proxied response from tutor server

- [ ] **Step 5: Commit**

```bash
git add internal/chat/server/proxy.go internal/chat/server/server.go cmd/chat/main.go
git commit -m "feat(tutor): add chat-to-tutor reverse proxy"
```

---

### Task 8: Makefile + Systemd + Deploy

**Files:**
- Modify: `Makefile`
- Create: `deploy/soul-v2-tutor.service`

- [ ] **Step 1: Update Makefile**

Add `build-tutor` target, update `build`, `serve`, `clean`:

```makefile
build-tutor:
	go build -o soul-tutor ./cmd/tutor

build: web build-go build-tasks build-tutor

serve: build
	./soul-chat serve & ./soul-tasks serve & ./soul-tutor serve & wait

clean:
	rm -f soul-chat soul-tasks soul-tutor
	rm -rf web/dist
```

- [ ] **Step 2: Create systemd service**

Create `deploy/soul-v2-tutor.service` following the exact pattern from `deploy/soul-v2-tasks.service`:
- User/Group: rishav
- WorkingDirectory: /home/rishav/soul-v2
- ExecStart: /home/rishav/soul-v2/soul-tutor serve
- Environment: SOUL_TUTOR_HOST=127.0.0.1, SOUL_TUTOR_PORT=3006
- All hardening directives (NoNewPrivileges, ProtectSystem, ProtectHome, ReadWritePaths, PrivateTmp)

- [ ] **Step 3: Build all**

Run: `make build`
Expected: soul-chat, soul-tasks, soul-tutor binaries all built

- [ ] **Step 4: Commit**

```bash
git add Makefile deploy/soul-v2-tutor.service
git commit -m "feat(tutor): add Makefile targets and systemd service"
```

---

## Chunk 3: Frontend

### Task 9: TypeScript Types + Hooks

**Files:**
- Modify: `web/src/lib/types.ts`
- Create: `web/src/hooks/useTutor.ts`
- Create: `web/src/hooks/useDrill.ts`
- Create: `web/src/hooks/useMockSession.ts`

- [ ] **Step 1: Add Tutor types**

Add to `web/src/lib/types.ts`:

```typescript
// --- Tutor Types ---

export interface TutorTopic {
  id: number;
  module: string;
  category: string;
  name: string;
  difficulty: 'easy' | 'medium' | 'hard';
  content_path: string;
  status: 'not_started' | 'learning' | 'drilling' | 'mastered';
  created_at: string;
}

export interface TutorModuleStats {
  module: string;
  total: number;
  not_started: number;
  learning: number;
  drilling: number;
  mastered: number;
  avg_score: number;
  total_time: number;
  completion: number;
}

export interface TutorDashboard {
  readiness: number;
  modules: TutorModuleStats[];
  streak: number;
  due_reviews: number;
  today: {
    time_spent_seconds: number;
    sessions_count: number;
    questions_answered: number;
    score_avg: number;
  };
}

export interface TutorDailyActivity {
  date: string;
  module: string;
  time_spent_seconds: number;
  sessions_count: number;
  questions_answered: number;
  score_avg: number;
}

export interface TutorConfidenceGap {
  topic_id: number;
  self_rated_score: number;
  actual_score: number;
  rated_at: string;
}

export interface TutorAnalytics {
  activity: TutorDailyActivity[];
  confidence_gaps: TutorConfidenceGap[];
}

export interface TutorQuizQuestion {
  id: number;
  topic_id: number;
  difficulty: string;
  question_text: string;
  answer_text: string;
  explanation: string;
}

export interface TutorDrillResult {
  correct: boolean;
  explanation: string;
  next_review: string;
  session_stats: { asked: number; correct: number; score: number };
}

export interface TutorMockSession {
  id: number;
  type: 'technical' | 'behavioral' | 'full_loop';
  job_description: string;
  started_at: string;
  completed_at: string | null;
  overall_score: number | null;
  scores: { dimension: string; score: number }[];
}

export interface TutorStudyPlan {
  id: number;
  target_role: string;
  target_date: string;
  plan_json: string;
  active: boolean;
}
```

- [ ] **Step 2: Create useTutor hook**

Create `web/src/hooks/useTutor.ts`:
- Fetches dashboard, analytics, topics, mocks from `/api/tutor/*`
- States: dashboard, topics, analytics, mocks, loading, error, activeTab, moduleFilter
- Methods: refresh(), setActiveTab(), setModuleFilter()
- Fetches appropriate data on tab change
- Uses `reportError` on failures, `reportUsage` on tab changes

- [ ] **Step 3: Create useDrill hook**

Create `web/src/hooks/useDrill.ts`:
- `startDrill(topicId)` → `POST /api/tutor/drill/start`
- `submitAnswer(questionId, answer)` → `POST /api/tutor/drill/answer`
- States: question, evaluation, sessionStats, loading
- `reportUsage('drill.start')`, `reportUsage('drill.answer')`

- [ ] **Step 4: Create useMockSession hook**

Create `web/src/hooks/useMockSession.ts`:
- Loads session from `GET /api/tutor/mocks/:id`
- `submitAnswer(answer)` → `POST /api/tutor/mocks/:id/answer`
- States: session, currentQuestion, scores, loading
- `reportUsage('mock.answer')`, `reportUsage('mock.complete')`

- [ ] **Step 5: Run type check**

Run: `cd web && npx tsc --noEmit`
Expected: No errors

- [ ] **Step 6: Commit**

```bash
git add web/src/lib/types.ts web/src/hooks/useTutor.ts web/src/hooks/useDrill.ts web/src/hooks/useMockSession.ts
git commit -m "feat(tutor): add TypeScript types and data hooks"
```

---

### Task 10: TutorPage Component

**Files:**
- Create: `web/src/pages/TutorPage.tsx`
- Create: `web/src/components/ReadinessBar.tsx`
- Create: `web/src/components/ModuleCard.tsx`
- Create: `web/src/components/TopicRow.tsx`
- Create: `web/src/components/MockSessionCard.tsx`

- [ ] **Step 1: Create ReadinessBar component**

Simple progress bar with percentage label. Props: `value: number` (0-100).
Dark theme: zinc track, emerald fill. `data-testid="readiness-bar"`.

- [ ] **Step 2: Create ModuleCard component**

Props: `stats: TutorModuleStats`. Shows:
- Module name + completion %
- Progress bar (visual fill)
- Mastered count / total
- Status pills: New (zinc), Learn (blue), Drill (amber), Done (emerald)
- `data-testid="module-card"`, `usePerformance('ModuleCard')`

- [ ] **Step 3: Create TopicRow component**

Props: `topic: TutorTopic`, `onClick: () => void`. Shows:
- Name + category
- Difficulty badge (easy=emerald, medium=amber, hard=red)
- Status badge (not_started=zinc, learning=blue, drilling=amber, mastered=emerald)
- Click → navigates to drill page
- `data-testid="topic-row"`

- [ ] **Step 4: Create MockSessionCard component**

Props: `session: TutorMockSession`, `onClick: () => void`. Shows:
- Type + date
- Score bar (0-100%) with color coding (emerald >=80, amber 50-79, red <50)
- JD snippet (2-line clamp)
- `data-testid="mock-session-card"`

- [ ] **Step 5: Create TutorPage with 5 tabs**

Create `web/src/pages/TutorPage.tsx`:
- Tab navigation: Dashboard, Analytics, Topics, Mocks, Guide
- Dashboard tab: ReadinessBar, streak/due badges, 3 ModuleCards, today's activity
- Analytics tab: activity table + confidence gap rows
- Topics tab: module filter buttons + TopicRow list (click → `/tutor/drill/:id`)
- Mocks tab: MockSessionCard list (click → `/tutor/mock/:id`)
- Guide tab: static documentation content (learning flow, commands, mastery criteria)
- `usePerformance('TutorPage')`, `reportUsage('page.view', { page: 'tutor' })`
- `data-testid="tutor-page"`

- [ ] **Step 6: Run type check**

Run: `cd web && npx tsc --noEmit`
Expected: No errors

- [ ] **Step 7: Commit**

```bash
git add web/src/pages/TutorPage.tsx web/src/components/ReadinessBar.tsx web/src/components/ModuleCard.tsx web/src/components/TopicRow.tsx web/src/components/MockSessionCard.tsx
git commit -m "feat(tutor): add TutorPage with 5-tab layout"
```

---

### Task 11: DrillPage + MockPage

**Files:**
- Create: `web/src/pages/DrillPage.tsx`
- Create: `web/src/pages/MockPage.tsx`
- Create: `web/src/components/DrillSession.tsx`

- [ ] **Step 1: Create DrillSession component**

Interactive question/answer flow:
- Shows question text + difficulty badge
- Text area for answer input
- Submit button → shows evaluation (correct/incorrect, explanation, next review date)
- "Next Question" button
- Session progress bar (questions answered, running score)
- `data-testid="drill-session"`, `usePerformance('DrillSession')`

- [ ] **Step 2: Create DrillPage**

`/tutor/drill/:id` route. Uses `useDrill(topicId)` hook:
- Topic name + difficulty header
- DrillSession component
- Back button → `/tutor` (Topics tab)
- `usePerformance('DrillPage')`, `reportUsage('page.view', { page: 'drill', topicId })`
- `reportError` on load/submit failures

- [ ] **Step 3: Create MockPage**

`/tutor/mock/:id` route. Uses `useMockSession(sessionId)` hook:
- Interview type + JD summary header
- Question cards (one at a time)
- Answer text area
- After completion: dimensional scores + overall score + feedback
- Back button → `/tutor` (Mocks tab)
- `usePerformance('MockPage')`, `reportUsage('page.view', { page: 'mock', sessionId })`

- [ ] **Step 4: Run type check**

Run: `cd web && npx tsc --noEmit`
Expected: No errors

- [ ] **Step 5: Commit**

```bash
git add web/src/pages/DrillPage.tsx web/src/pages/MockPage.tsx web/src/components/DrillSession.tsx
git commit -m "feat(tutor): add DrillPage and MockPage interactive UIs"
```

---

### Task 12: Router + Nav + Build + Verify

**Files:**
- Modify: `web/src/router.tsx`
- Modify: `web/src/layouts/AppLayout.tsx`

- [ ] **Step 1: Add routes to router.tsx**

Add 3 new routes inside the children array:
```tsx
{
  path: 'tutor',
  errorElement: <RouteErrorFallback />,
  lazy: () => import('./pages/TutorPage').then(m => ({ Component: m.TutorPage })),
},
{
  path: 'tutor/drill/:id',
  errorElement: <RouteErrorFallback />,
  lazy: () => import('./pages/DrillPage').then(m => ({ Component: m.DrillPage })),
},
{
  path: 'tutor/mock/:id',
  errorElement: <RouteErrorFallback />,
  lazy: () => import('./pages/MockPage').then(m => ({ Component: m.MockPage })),
},
```

- [ ] **Step 2: Add Tutor NavLink to AppLayout.tsx**

In the nav element, after the Tasks NavLink:
```tsx
<NavLink to="/tutor" className={navLinkClass}>Tutor</NavLink>
```

- [ ] **Step 3: Build frontend**

Run: `cd web && npx vite build`
Expected: Build succeeds, check bundle size stays < 300KB gzipped

- [ ] **Step 4: Build all binaries**

Run: `make build`
Expected: soul-chat, soul-tasks, soul-tutor all build

- [ ] **Step 5: Run static verification**

Run: `go vet ./... && cd web && npx tsc --noEmit`
Expected: No errors

- [ ] **Step 6: Run unit tests**

Run: `go test -race -count=1 ./internal/tutor/...`
Expected: All pass

- [ ] **Step 7: Full smoke test**

Start all servers: `./soul-chat serve & ./soul-tasks serve & ./soul-tutor serve &`

Verify:
- `curl -s http://localhost:3006/api/health` → `{"status":"ok"}`
- `curl -s http://localhost:3002/api/tutor/dashboard` → proxied response
- Open `http://localhost:3002/tutor` in browser → TutorPage renders
- Navigate between Chat, Tasks, Tutor → all work, connection status stays green
- Crash test: if TutorPage throws, Chat remains functional

Kill servers.

- [ ] **Step 8: Commit**

```bash
git add web/src/router.tsx web/src/layouts/AppLayout.tsx
git commit -m "feat(tutor): wire routes, navigation, and verify full stack"
```

---

### Task 13: Deploy + CLAUDE.md Update

**Files:**
- Modify: `CLAUDE.md`

- [ ] **Step 1: Install and start tutor service**

```bash
sudo cp deploy/soul-v2-tutor.service /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable soul-v2-tutor
sudo systemctl restart soul-v2-tutor
```

Verify: `sudo systemctl status soul-v2-tutor` shows active.

- [ ] **Step 2: Rebuild and restart chat server**

```bash
make build
sudo systemctl restart soul-v2
```

Verify: `curl -s http://localhost:3002/api/tutor/dashboard` returns data.

- [ ] **Step 3: Update CLAUDE.md**

Add tutor to Architecture section:
```
cmd/tutor/main.go             Tutor server CLI entrypoint (:3006)
internal/tutor/
  server/                     HTTP server, REST API handlers, tool execution
  store/                      SQLite CRUD (tutor.db) — 11 tables
  modules/                    5 modules (DSA, AI, Behavioral, Mock, Planner) + SM-2 + importer
```

Add environment variables:
```
| SOUL_TUTOR_HOST | 127.0.0.1 | Tutor server bind address |
| SOUL_TUTOR_PORT | 3006 | Tutor server port |
| SOUL_TUTOR_URL | http://127.0.0.1:3006 | Tutor URL (for chat proxy) |
```

- [ ] **Step 4: Commit**

```bash
git add CLAUDE.md
git commit -m "feat(tutor): deploy service, update architecture docs"
```

- [ ] **Step 5: Push to Gitea**

```bash
git push origin
```
