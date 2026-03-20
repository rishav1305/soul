package store

import (
	"database/sql"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

// Topic represents a study topic.
type Topic struct {
	ID          int64     `json:"id"`
	Module      string    `json:"module"`
	Category    string    `json:"category"`
	Name        string    `json:"name"`
	Difficulty  string    `json:"difficulty"`
	ContentPath string    `json:"contentPath"`
	Status      string    `json:"status"`
	CreatedAt   time.Time `json:"createdAt"`
}

// Progress represents a study session record.
type Progress struct {
	ID               int64     `json:"id"`
	TopicID          int64     `json:"topicId"`
	SessionDate      time.Time `json:"sessionDate"`
	Score            float64   `json:"score"`
	QuestionsAsked   int       `json:"questionsAsked"`
	QuestionsCorrect int       `json:"questionsCorrect"`
	TimeSpentSeconds int       `json:"timeSpentSeconds"`
	Notes            string    `json:"notes"`
}

// QuizQuestion represents a quiz question for a topic.
type QuizQuestion struct {
	ID           int64  `json:"id"`
	TopicID      int64  `json:"topicId"`
	Difficulty   string `json:"difficulty"`
	QuestionText string `json:"questionText"`
	AnswerText   string `json:"answerText"`
	Explanation  string `json:"explanation"`
	Source       string `json:"source"`
}

// SpacedRepetition tracks review scheduling for a topic.
type SpacedRepetition struct {
	ID              int64     `json:"id"`
	TopicID         int64     `json:"topicId"`
	LastReviewed    time.Time `json:"lastReviewed"`
	NextReview      time.Time `json:"nextReview"`
	IntervalDays    int       `json:"intervalDays"`
	EaseFactor      float64   `json:"easeFactor"`
	RepetitionCount int       `json:"repetitionCount"`
}

// DailyActivity aggregates daily study metrics per module.
type DailyActivity struct {
	ID               int64   `json:"id"`
	Date             string  `json:"date"`
	Module           string  `json:"module"`
	TimeSpentSeconds int     `json:"timeSpentSeconds"`
	SessionsCount    int     `json:"sessionsCount"`
	QuestionsAnswered int    `json:"questionsAnswered"`
	ScoreAvg         float64 `json:"scoreAvg"`
}

// ConfidenceRating records self-assessed vs actual scores.
type ConfidenceRating struct {
	ID             int64     `json:"id"`
	TopicID        int64     `json:"topicId"`
	SelfRatedScore float64   `json:"selfRatedScore"`
	ActualScore    float64   `json:"actualScore"`
	RatedAt        time.Time `json:"ratedAt"`
}

// MockSession represents a mock interview session.
type MockSession struct {
	ID             int64      `json:"id"`
	Type           string     `json:"type"`
	JobDescription string     `json:"jobDescription"`
	StartedAt      time.Time  `json:"startedAt"`
	CompletedAt    *time.Time `json:"completedAt,omitempty"`
	OverallScore   float64    `json:"overallScore"`
	FeedbackJSON   string     `json:"feedbackJson"`
}

// MockSessionScore represents a dimension score within a mock session.
type MockSessionScore struct {
	ID            int64   `json:"id"`
	MockSessionID int64   `json:"mockSessionId"`
	Dimension     string  `json:"dimension"`
	Score         float64 `json:"score"`
}

// StarStory represents a STAR-format behavioral story.
type StarStory struct {
	ID                 int64  `json:"id"`
	Competency         string `json:"competency"`
	Situation          string `json:"situation"`
	Task               string `json:"task"`
	Action             string `json:"action"`
	Result             string `json:"result"`
	ProjectsReferenced string `json:"projectsReferenced"`
	Version            int    `json:"version"`
}

// StudyPlan represents a study plan for a target role.
type StudyPlan struct {
	ID         int64     `json:"id"`
	TargetRole string    `json:"targetRole"`
	TargetDate string    `json:"targetDate"`
	CreatedAt  time.Time `json:"createdAt"`
	PlanJSON   string    `json:"planJson"`
	Active     bool      `json:"active"`
}

// QuestionAttempt records a single answer to a quiz question.
type QuestionAttempt struct {
	ID                int64  `json:"id"`
	QuizQuestionID    int64  `json:"quizQuestionId"`
	ProgressID        int64  `json:"progressId"`
	AnsweredCorrectly bool   `json:"answeredCorrectly"`
	TimeTakenSeconds  int    `json:"timeTakenSeconds"`
	UserAnswerSummary string `json:"userAnswerSummary"`
}

// ModuleStats holds aggregated stats for a module.
type ModuleStats struct {
	Module           string  `json:"module"`
	TopicCount       int     `json:"topicCount"`
	CompletedCount   int     `json:"completedCount"`
	InProgressCount  int     `json:"inProgressCount"`
	CompletionPct    float64 `json:"completionPct"`
	TotalTimeSeconds int     `json:"totalTimeSeconds"`
	AvgScore         float64 `json:"avgScore"`
}

// Store provides SQLite-backed tutor CRUD.
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

// Close closes the database.
func (s *Store) Close() error {
	return s.db.Close()
}

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
		session_date DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
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
		last_reviewed DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		next_review DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		interval_days INTEGER NOT NULL DEFAULT 1,
		ease_factor REAL NOT NULL DEFAULT 2.5,
		repetition_count INTEGER NOT NULL DEFAULT 0
	);

	CREATE TABLE IF NOT EXISTS daily_activity (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		date TEXT NOT NULL,
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
		overall_score REAL NOT NULL DEFAULT 0,
		feedback_json TEXT NOT NULL DEFAULT '{}'
	);

	CREATE TABLE IF NOT EXISTS mock_session_scores (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		mock_session_id INTEGER NOT NULL REFERENCES mock_sessions(id) ON DELETE CASCADE,
		dimension TEXT NOT NULL,
		score REAL NOT NULL DEFAULT 0
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
		target_date TEXT NOT NULL DEFAULT '',
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		plan_json TEXT NOT NULL DEFAULT '{}',
		active INTEGER NOT NULL DEFAULT 1
	);

	CREATE TABLE IF NOT EXISTS question_attempts (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		quiz_question_id INTEGER NOT NULL REFERENCES quiz_questions(id) ON DELETE CASCADE,
		progress_id INTEGER NOT NULL REFERENCES progress(id) ON DELETE CASCADE,
		answered_correctly INTEGER NOT NULL DEFAULT 0,
		time_taken_seconds INTEGER NOT NULL DEFAULT 0,
		user_answer_summary TEXT NOT NULL DEFAULT ''
	);

	CREATE INDEX IF NOT EXISTS idx_topics_module ON topics(module);
	CREATE INDEX IF NOT EXISTS idx_topics_status ON topics(status);
	CREATE INDEX IF NOT EXISTS idx_topics_category ON topics(category);
	CREATE INDEX IF NOT EXISTS idx_progress_topic_id ON progress(topic_id);
	CREATE INDEX IF NOT EXISTS idx_progress_session_date ON progress(session_date);
	CREATE INDEX IF NOT EXISTS idx_quiz_questions_topic_id ON quiz_questions(topic_id);
	CREATE INDEX IF NOT EXISTS idx_spaced_repetition_next_review ON spaced_repetition(next_review);
	CREATE INDEX IF NOT EXISTS idx_daily_activity_date ON daily_activity(date);
	CREATE INDEX IF NOT EXISTS idx_daily_activity_module ON daily_activity(module);
	CREATE INDEX IF NOT EXISTS idx_confidence_ratings_topic_id ON confidence_ratings(topic_id);
	CREATE INDEX IF NOT EXISTS idx_mock_sessions_type ON mock_sessions(type);
	CREATE INDEX IF NOT EXISTS idx_mock_session_scores_session_id ON mock_session_scores(mock_session_id);
	CREATE INDEX IF NOT EXISTS idx_star_stories_competency ON star_stories(competency);
	CREATE INDEX IF NOT EXISTS idx_study_plans_active ON study_plans(active);
	CREATE INDEX IF NOT EXISTS idx_question_attempts_quiz_question_id ON question_attempts(quiz_question_id);
	CREATE INDEX IF NOT EXISTS idx_question_attempts_progress_id ON question_attempts(progress_id);
	CREATE UNIQUE INDEX IF NOT EXISTS idx_quiz_questions_source_dedup ON quiz_questions(topic_id, source) WHERE source != '';
	`
	if _, err := s.db.Exec(schema); err != nil {
		return fmt.Errorf("tutor: migrate: %w", err)
	}
	return nil
}

// ---------- Topics ----------

// CreateTopic inserts a new topic. If a topic with the same (module, category, name) exists, returns it.
func (s *Store) CreateTopic(module, category, name, difficulty, contentPath string) (*Topic, error) {
	res, err := s.db.Exec(
		"INSERT INTO topics (module, category, name, difficulty, content_path) VALUES (?, ?, ?, ?, ?) ON CONFLICT(module, category, name) DO NOTHING",
		module, category, name, difficulty, contentPath,
	)
	if err != nil {
		return nil, fmt.Errorf("tutor: create topic: %w", err)
	}
	id, _ := res.LastInsertId()
	if id == 0 {
		// Already existed — fetch by name.
		return s.GetTopicByName(module, category, name)
	}
	return s.GetTopic(id)
}

// GetTopic retrieves a topic by ID.
func (s *Store) GetTopic(id int64) (*Topic, error) {
	var t Topic
	err := s.db.QueryRow(
		"SELECT id, module, category, name, difficulty, content_path, status, created_at FROM topics WHERE id = ?",
		id,
	).Scan(&t.ID, &t.Module, &t.Category, &t.Name, &t.Difficulty, &t.ContentPath, &t.Status, &t.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("tutor: topic not found: %d", id)
	}
	if err != nil {
		return nil, fmt.Errorf("tutor: get topic: %w", err)
	}
	return &t, nil
}

// GetTopicByName retrieves a topic by its unique (module, category, name) tuple.
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

// ListTopics returns topics, optionally filtered by module and/or status.
func (s *Store) ListTopics(module, status string) ([]Topic, error) {
	query := "SELECT id, module, category, name, difficulty, content_path, status, created_at FROM topics"
	var conditions []string
	var args []interface{}

	if module != "" {
		conditions = append(conditions, "module = ?")
		args = append(args, module)
	}
	if status != "" {
		conditions = append(conditions, "status = ?")
		args = append(args, status)
	}
	if len(conditions) > 0 {
		query += " WHERE " + join(conditions, " AND ")
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

// UpdateTopicStatus changes a topic's status.
func (s *Store) UpdateTopicStatus(id int64, status string) error {
	result, err := s.db.Exec("UPDATE topics SET status = ? WHERE id = ?", status, id)
	if err != nil {
		return fmt.Errorf("tutor: update topic status: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("tutor: topic not found: %d", id)
	}
	return nil
}

// ---------- Quiz Questions ----------

// CreateQuizQuestion inserts a new quiz question. If source is non-empty and a question
// with the same (topic_id, source) already exists, the existing question is returned (dedup).
func (s *Store) CreateQuizQuestion(topicID int64, difficulty, questionText, answerText, explanation, source string) (*QuizQuestion, error) {
	// If source is non-empty, check for existing question with same source for this topic.
	if source != "" {
		var existingID int64
		err := s.db.QueryRow(
			"SELECT id FROM quiz_questions WHERE topic_id = ? AND source = ?",
			topicID, source,
		).Scan(&existingID)
		if err == nil {
			return s.GetQuizQuestion(existingID)
		}
	}

	res, err := s.db.Exec(
		"INSERT INTO quiz_questions (topic_id, difficulty, question_text, answer_text, explanation, source) VALUES (?, ?, ?, ?, ?, ?)",
		topicID, difficulty, questionText, answerText, explanation, source,
	)
	if err != nil {
		return nil, fmt.Errorf("tutor: create quiz question: %w", err)
	}
	id, _ := res.LastInsertId()
	return s.GetQuizQuestion(id)
}

// GetQuizQuestion retrieves a quiz question by ID.
func (s *Store) GetQuizQuestion(id int64) (*QuizQuestion, error) {
	var q QuizQuestion
	err := s.db.QueryRow(
		"SELECT id, topic_id, difficulty, question_text, answer_text, explanation, source FROM quiz_questions WHERE id = ?",
		id,
	).Scan(&q.ID, &q.TopicID, &q.Difficulty, &q.QuestionText, &q.AnswerText, &q.Explanation, &q.Source)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("tutor: quiz question not found: %d", id)
	}
	if err != nil {
		return nil, fmt.Errorf("tutor: get quiz question: %w", err)
	}
	return &q, nil
}

// ListQuestions returns quiz questions for a topic.
func (s *Store) ListQuestions(topicID int64) ([]QuizQuestion, error) {
	rows, err := s.db.Query(
		"SELECT id, topic_id, difficulty, question_text, answer_text, explanation, source FROM quiz_questions WHERE topic_id = ? ORDER BY id",
		topicID,
	)
	if err != nil {
		return nil, fmt.Errorf("tutor: list questions: %w", err)
	}
	defer rows.Close()

	var questions []QuizQuestion
	for rows.Next() {
		var q QuizQuestion
		if err := rows.Scan(&q.ID, &q.TopicID, &q.Difficulty, &q.QuestionText, &q.AnswerText, &q.Explanation, &q.Source); err != nil {
			return nil, fmt.Errorf("tutor: scan question: %w", err)
		}
		questions = append(questions, q)
	}
	return questions, rows.Err()
}

// PickNextQuestion returns the quiz question with the fewest attempts for a topic.
func (s *Store) PickNextQuestion(topicID int64) (*QuizQuestion, error) {
	var q QuizQuestion
	err := s.db.QueryRow(`
		SELECT qq.id, qq.topic_id, qq.difficulty, qq.question_text, qq.answer_text, qq.explanation, qq.source
		FROM quiz_questions qq
		LEFT JOIN (
			SELECT quiz_question_id, COUNT(*) as attempt_count
			FROM question_attempts
			GROUP BY quiz_question_id
		) ac ON qq.id = ac.quiz_question_id
		WHERE qq.topic_id = ?
		ORDER BY COALESCE(ac.attempt_count, 0) ASC, qq.id ASC
		LIMIT 1`,
		topicID,
	).Scan(&q.ID, &q.TopicID, &q.Difficulty, &q.QuestionText, &q.AnswerText, &q.Explanation, &q.Source)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("tutor: no questions for topic: %d", topicID)
	}
	if err != nil {
		return nil, fmt.Errorf("tutor: pick next question: %w", err)
	}
	return &q, nil
}

// ---------- Progress ----------

// RecordProgress inserts a new progress record for a topic.
func (s *Store) RecordProgress(topicID int64, score float64, questionsAsked, questionsCorrect, timeSpentSeconds int, notes string) (*Progress, error) {
	res, err := s.db.Exec(
		"INSERT INTO progress (topic_id, score, questions_asked, questions_correct, time_spent_seconds, notes) VALUES (?, ?, ?, ?, ?, ?)",
		topicID, score, questionsAsked, questionsCorrect, timeSpentSeconds, notes,
	)
	if err != nil {
		return nil, fmt.Errorf("tutor: record progress: %w", err)
	}
	id, _ := res.LastInsertId()
	var p Progress
	err = s.db.QueryRow(
		"SELECT id, topic_id, session_date, score, questions_asked, questions_correct, time_spent_seconds, notes FROM progress WHERE id = ?",
		id,
	).Scan(&p.ID, &p.TopicID, &p.SessionDate, &p.Score, &p.QuestionsAsked, &p.QuestionsCorrect, &p.TimeSpentSeconds, &p.Notes)
	if err != nil {
		return nil, fmt.Errorf("tutor: get progress: %w", err)
	}
	return &p, nil
}

// RecordAttempt records a single question attempt tied to a progress session.
func (s *Store) RecordAttempt(quizQuestionID, progressID int64, answeredCorrectly bool, timeTakenSeconds int, userAnswerSummary string) (*QuestionAttempt, error) {
	correct := 0
	if answeredCorrectly {
		correct = 1
	}
	res, err := s.db.Exec(
		"INSERT INTO question_attempts (quiz_question_id, progress_id, answered_correctly, time_taken_seconds, user_answer_summary) VALUES (?, ?, ?, ?, ?)",
		quizQuestionID, progressID, correct, timeTakenSeconds, userAnswerSummary,
	)
	if err != nil {
		return nil, fmt.Errorf("tutor: record attempt: %w", err)
	}
	id, _ := res.LastInsertId()
	var a QuestionAttempt
	err = s.db.QueryRow(
		"SELECT id, quiz_question_id, progress_id, answered_correctly, time_taken_seconds, user_answer_summary FROM question_attempts WHERE id = ?",
		id,
	).Scan(&a.ID, &a.QuizQuestionID, &a.ProgressID, &a.AnsweredCorrectly, &a.TimeTakenSeconds, &a.UserAnswerSummary)
	if err != nil {
		return nil, fmt.Errorf("tutor: get attempt: %w", err)
	}
	return &a, nil
}

// ---------- Spaced Repetition ----------

// GetSpacedRep retrieves spaced repetition data for a topic.
func (s *Store) GetSpacedRep(topicID int64) (*SpacedRepetition, error) {
	var sr SpacedRepetition
	err := s.db.QueryRow(
		"SELECT id, topic_id, last_reviewed, next_review, interval_days, ease_factor, repetition_count FROM spaced_repetition WHERE topic_id = ?",
		topicID,
	).Scan(&sr.ID, &sr.TopicID, &sr.LastReviewed, &sr.NextReview, &sr.IntervalDays, &sr.EaseFactor, &sr.RepetitionCount)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("tutor: spaced rep not found for topic: %d", topicID)
	}
	if err != nil {
		return nil, fmt.Errorf("tutor: get spaced rep: %w", err)
	}
	return &sr, nil
}

// UpsertSpacedRep creates or updates spaced repetition data for a topic.
func (s *Store) UpsertSpacedRep(topicID int64, nextReview time.Time, intervalDays int, easeFactor float64, repetitionCount int) (*SpacedRepetition, error) {
	_, err := s.db.Exec(`
		INSERT INTO spaced_repetition (topic_id, last_reviewed, next_review, interval_days, ease_factor, repetition_count)
		VALUES (?, CURRENT_TIMESTAMP, ?, ?, ?, ?)
		ON CONFLICT(topic_id) DO UPDATE SET
			last_reviewed = CURRENT_TIMESTAMP,
			next_review = excluded.next_review,
			interval_days = excluded.interval_days,
			ease_factor = excluded.ease_factor,
			repetition_count = excluded.repetition_count`,
		topicID, nextReview, intervalDays, easeFactor, repetitionCount,
	)
	if err != nil {
		return nil, fmt.Errorf("tutor: upsert spaced rep: %w", err)
	}
	return s.GetSpacedRep(topicID)
}

// GetDueReviews returns topics with spaced repetition due on or before the given time.
func (s *Store) GetDueReviews(before time.Time) ([]SpacedRepetition, error) {
	rows, err := s.db.Query(
		"SELECT id, topic_id, last_reviewed, next_review, interval_days, ease_factor, repetition_count FROM spaced_repetition WHERE next_review <= ? ORDER BY next_review ASC",
		before,
	)
	if err != nil {
		return nil, fmt.Errorf("tutor: get due reviews: %w", err)
	}
	defer rows.Close()

	var results []SpacedRepetition
	for rows.Next() {
		var sr SpacedRepetition
		if err := rows.Scan(&sr.ID, &sr.TopicID, &sr.LastReviewed, &sr.NextReview, &sr.IntervalDays, &sr.EaseFactor, &sr.RepetitionCount); err != nil {
			return nil, fmt.Errorf("tutor: scan spaced rep: %w", err)
		}
		results = append(results, sr)
	}
	return results, rows.Err()
}

// ---------- Daily Activity ----------

// UpsertDailyActivity creates or updates a daily activity record for a module.
func (s *Store) UpsertDailyActivity(date, module string, timeSpentSeconds, sessionsCount, questionsAnswered int, scoreAvg float64) (*DailyActivity, error) {
	_, err := s.db.Exec(`
		INSERT INTO daily_activity (date, module, time_spent_seconds, sessions_count, questions_answered, score_avg)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(date, module) DO UPDATE SET
			time_spent_seconds = daily_activity.time_spent_seconds + excluded.time_spent_seconds,
			sessions_count = daily_activity.sessions_count + excluded.sessions_count,
			questions_answered = daily_activity.questions_answered + excluded.questions_answered,
			score_avg = excluded.score_avg`,
		date, module, timeSpentSeconds, sessionsCount, questionsAnswered, scoreAvg,
	)
	if err != nil {
		return nil, fmt.Errorf("tutor: upsert daily activity: %w", err)
	}
	return s.GetActivity(date, module)
}

// GetActivity retrieves a daily activity record.
func (s *Store) GetActivity(date, module string) (*DailyActivity, error) {
	var da DailyActivity
	err := s.db.QueryRow(
		"SELECT id, date, module, time_spent_seconds, sessions_count, questions_answered, score_avg FROM daily_activity WHERE date = ? AND module = ?",
		date, module,
	).Scan(&da.ID, &da.Date, &da.Module, &da.TimeSpentSeconds, &da.SessionsCount, &da.QuestionsAnswered, &da.ScoreAvg)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("tutor: activity not found: %s/%s", date, module)
	}
	if err != nil {
		return nil, fmt.Errorf("tutor: get activity: %w", err)
	}
	return &da, nil
}

// GetTodayActivity returns all daily activity records for today.
func (s *Store) GetTodayActivity() ([]DailyActivity, error) {
	today := time.Now().Format("2006-01-02")
	rows, err := s.db.Query(
		"SELECT id, date, module, time_spent_seconds, sessions_count, questions_answered, score_avg FROM daily_activity WHERE date = ?",
		today,
	)
	if err != nil {
		return nil, fmt.Errorf("tutor: get today activity: %w", err)
	}
	defer rows.Close()

	var results []DailyActivity
	for rows.Next() {
		var da DailyActivity
		if err := rows.Scan(&da.ID, &da.Date, &da.Module, &da.TimeSpentSeconds, &da.SessionsCount, &da.QuestionsAnswered, &da.ScoreAvg); err != nil {
			return nil, fmt.Errorf("tutor: scan activity: %w", err)
		}
		results = append(results, da)
	}
	return results, rows.Err()
}

// GetStreak returns the number of consecutive days with activity ending on today (or yesterday if today has no activity yet).
func (s *Store) GetStreak() (int, error) {
	rows, err := s.db.Query(
		"SELECT DISTINCT date FROM daily_activity ORDER BY date DESC",
	)
	if err != nil {
		return 0, fmt.Errorf("tutor: get streak: %w", err)
	}
	defer rows.Close()

	var dates []string
	for rows.Next() {
		var d string
		if err := rows.Scan(&d); err != nil {
			return 0, fmt.Errorf("tutor: scan streak date: %w", err)
		}
		dates = append(dates, d)
	}
	if err := rows.Err(); err != nil {
		return 0, err
	}
	if len(dates) == 0 {
		return 0, nil
	}

	today := time.Now().Format("2006-01-02")
	yesterday := time.Now().AddDate(0, 0, -1).Format("2006-01-02")

	// Streak must start from today or yesterday.
	if dates[0] != today && dates[0] != yesterday {
		return 0, nil
	}

	streak := 1
	for i := 1; i < len(dates); i++ {
		prev, err1 := time.Parse("2006-01-02", dates[i-1])
		curr, err2 := time.Parse("2006-01-02", dates[i])
		if err1 != nil || err2 != nil {
			break
		}
		if prev.Sub(curr) == 24*time.Hour {
			streak++
		} else {
			break
		}
	}
	return streak, nil
}

// ---------- Mock Sessions ----------

// CreateMockSession creates a new mock interview session.
func (s *Store) CreateMockSession(sessionType, jobDescription string) (*MockSession, error) {
	res, err := s.db.Exec(
		"INSERT INTO mock_sessions (type, job_description) VALUES (?, ?)",
		sessionType, jobDescription,
	)
	if err != nil {
		return nil, fmt.Errorf("tutor: create mock session: %w", err)
	}
	id, _ := res.LastInsertId()
	return s.GetMockSession(id)
}

// GetMockSession retrieves a mock session by ID.
func (s *Store) GetMockSession(id int64) (*MockSession, error) {
	var ms MockSession
	var completedAt sql.NullTime
	err := s.db.QueryRow(
		"SELECT id, type, job_description, started_at, completed_at, overall_score, feedback_json FROM mock_sessions WHERE id = ?",
		id,
	).Scan(&ms.ID, &ms.Type, &ms.JobDescription, &ms.StartedAt, &completedAt, &ms.OverallScore, &ms.FeedbackJSON)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("tutor: mock session not found: %d", id)
	}
	if err != nil {
		return nil, fmt.Errorf("tutor: get mock session: %w", err)
	}
	if completedAt.Valid {
		ms.CompletedAt = &completedAt.Time
	}
	return &ms, nil
}

// ListMockSessions returns mock sessions, optionally filtered by type.
func (s *Store) ListMockSessions(sessionType string) ([]MockSession, error) {
	query := "SELECT id, type, job_description, started_at, completed_at, overall_score, feedback_json FROM mock_sessions"
	var args []interface{}
	if sessionType != "" {
		query += " WHERE type = ?"
		args = append(args, sessionType)
	}
	query += " ORDER BY started_at DESC"

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("tutor: list mock sessions: %w", err)
	}
	defer rows.Close()

	var sessions []MockSession
	for rows.Next() {
		var ms MockSession
		var completedAt sql.NullTime
		if err := rows.Scan(&ms.ID, &ms.Type, &ms.JobDescription, &ms.StartedAt, &completedAt, &ms.OverallScore, &ms.FeedbackJSON); err != nil {
			return nil, fmt.Errorf("tutor: scan mock session: %w", err)
		}
		if completedAt.Valid {
			ms.CompletedAt = &completedAt.Time
		}
		sessions = append(sessions, ms)
	}
	return sessions, rows.Err()
}

// CompleteMockSession marks a mock session as completed with a score and feedback.
func (s *Store) CompleteMockSession(id int64, overallScore float64, feedbackJSON string) error {
	result, err := s.db.Exec(
		"UPDATE mock_sessions SET completed_at = CURRENT_TIMESTAMP, overall_score = ?, feedback_json = ? WHERE id = ?",
		overallScore, feedbackJSON, id,
	)
	if err != nil {
		return fmt.Errorf("tutor: complete mock session: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("tutor: mock session not found: %d", id)
	}
	return nil
}

// AddMockScore adds a dimension score to a mock session.
func (s *Store) AddMockScore(mockSessionID int64, dimension string, score float64) error {
	_, err := s.db.Exec(
		"INSERT INTO mock_session_scores (mock_session_id, dimension, score) VALUES (?, ?, ?)",
		mockSessionID, dimension, score,
	)
	if err != nil {
		return fmt.Errorf("tutor: add mock score: %w", err)
	}
	return nil
}

// GetMockScores returns all dimension scores for a mock session.
func (s *Store) GetMockScores(mockSessionID int64) ([]MockSessionScore, error) {
	rows, err := s.db.Query(
		"SELECT id, mock_session_id, dimension, score FROM mock_session_scores WHERE mock_session_id = ? ORDER BY id",
		mockSessionID,
	)
	if err != nil {
		return nil, fmt.Errorf("tutor: get mock scores: %w", err)
	}
	defer rows.Close()

	var scores []MockSessionScore
	for rows.Next() {
		var sc MockSessionScore
		if err := rows.Scan(&sc.ID, &sc.MockSessionID, &sc.Dimension, &sc.Score); err != nil {
			return nil, fmt.Errorf("tutor: scan mock score: %w", err)
		}
		scores = append(scores, sc)
	}
	return scores, rows.Err()
}

// ---------- STAR Stories ----------

// GetStarStory retrieves the latest STAR story for a competency.
func (s *Store) GetStarStory(competency string) (*StarStory, error) {
	var ss StarStory
	err := s.db.QueryRow(
		"SELECT id, competency, situation, task, action, result, projects_referenced, version FROM star_stories WHERE competency = ? ORDER BY version DESC LIMIT 1",
		competency,
	).Scan(&ss.ID, &ss.Competency, &ss.Situation, &ss.Task, &ss.Action, &ss.Result, &ss.ProjectsReferenced, &ss.Version)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("tutor: star story not found: %s", competency)
	}
	if err != nil {
		return nil, fmt.Errorf("tutor: get star story: %w", err)
	}
	return &ss, nil
}

// UpsertStarStory creates a new version of a STAR story for a competency.
func (s *Store) UpsertStarStory(competency, situation, task, action, result, projectsReferenced string) (*StarStory, error) {
	// Get current max version for this competency.
	var maxVersion int
	err := s.db.QueryRow(
		"SELECT COALESCE(MAX(version), 0) FROM star_stories WHERE competency = ?",
		competency,
	).Scan(&maxVersion)
	if err != nil {
		return nil, fmt.Errorf("tutor: get star story version: %w", err)
	}

	_, err = s.db.Exec(
		"INSERT INTO star_stories (competency, situation, task, action, result, projects_referenced, version) VALUES (?, ?, ?, ?, ?, ?, ?)",
		competency, situation, task, action, result, projectsReferenced, maxVersion+1,
	)
	if err != nil {
		return nil, fmt.Errorf("tutor: upsert star story: %w", err)
	}
	return s.GetStarStory(competency)
}

// ---------- Study Plans ----------

// GetActivePlan retrieves the currently active study plan.
func (s *Store) GetActivePlan() (*StudyPlan, error) {
	var sp StudyPlan
	var active int
	err := s.db.QueryRow(
		"SELECT id, target_role, target_date, created_at, plan_json, active FROM study_plans WHERE active = 1 ORDER BY created_at DESC LIMIT 1",
	).Scan(&sp.ID, &sp.TargetRole, &sp.TargetDate, &sp.CreatedAt, &sp.PlanJSON, &active)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("tutor: no active study plan")
	}
	if err != nil {
		return nil, fmt.Errorf("tutor: get active plan: %w", err)
	}
	sp.Active = active == 1
	return &sp, nil
}

// CreatePlan creates a new study plan, deactivating any existing active plans.
func (s *Store) CreatePlan(targetRole, targetDate, planJSON string) (*StudyPlan, error) {
	// Deactivate all existing plans.
	if _, err := s.db.Exec("UPDATE study_plans SET active = 0 WHERE active = 1"); err != nil {
		return nil, fmt.Errorf("tutor: deactivate plans: %w", err)
	}

	res, err := s.db.Exec(
		"INSERT INTO study_plans (target_role, target_date, plan_json, active) VALUES (?, ?, ?, 1)",
		targetRole, targetDate, planJSON,
	)
	if err != nil {
		return nil, fmt.Errorf("tutor: create plan: %w", err)
	}
	id, _ := res.LastInsertId()

	var sp StudyPlan
	var active int
	err = s.db.QueryRow(
		"SELECT id, target_role, target_date, created_at, plan_json, active FROM study_plans WHERE id = ?",
		id,
	).Scan(&sp.ID, &sp.TargetRole, &sp.TargetDate, &sp.CreatedAt, &sp.PlanJSON, &active)
	if err != nil {
		return nil, fmt.Errorf("tutor: get plan: %w", err)
	}
	sp.Active = active == 1
	return &sp, nil
}

// UpdatePlan updates the plan JSON for an existing study plan.
func (s *Store) UpdatePlan(id int64, planJSON string) error {
	result, err := s.db.Exec("UPDATE study_plans SET plan_json = ? WHERE id = ?", planJSON, id)
	if err != nil {
		return fmt.Errorf("tutor: update plan: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("tutor: plan not found: %d", id)
	}
	return nil
}

// ---------- Confidence Ratings ----------

// AddConfidenceRating records a confidence rating for a topic.
func (s *Store) AddConfidenceRating(topicID int64, selfRatedScore, actualScore float64) error {
	_, err := s.db.Exec(
		"INSERT INTO confidence_ratings (topic_id, self_rated_score, actual_score) VALUES (?, ?, ?)",
		topicID, selfRatedScore, actualScore,
	)
	if err != nil {
		return fmt.Errorf("tutor: add confidence rating: %w", err)
	}
	return nil
}

// ConfidenceGap represents the gap between self-rated and actual scores for a topic.
type ConfidenceGap struct {
	TopicID        int64   `json:"topicId"`
	TopicName      string  `json:"topicName"`
	Module         string  `json:"module"`
	AvgSelfRated   float64 `json:"avgSelfRated"`
	AvgActual      float64 `json:"avgActual"`
	Gap            float64 `json:"gap"`
}

// GetConfidenceGaps returns topics where the absolute gap between self-rated and actual scores exceeds the threshold.
func (s *Store) GetConfidenceGaps(threshold float64) ([]ConfidenceGap, error) {
	rows, err := s.db.Query(`
		SELECT t.id, t.name, t.module,
			AVG(cr.self_rated_score) as avg_self,
			AVG(cr.actual_score) as avg_actual,
			ABS(AVG(cr.self_rated_score) - AVG(cr.actual_score)) as gap
		FROM confidence_ratings cr
		JOIN topics t ON cr.topic_id = t.id
		GROUP BY cr.topic_id
		HAVING gap >= ?
		ORDER BY gap DESC`,
		threshold,
	)
	if err != nil {
		return nil, fmt.Errorf("tutor: get confidence gaps: %w", err)
	}
	defer rows.Close()

	var gaps []ConfidenceGap
	for rows.Next() {
		var g ConfidenceGap
		if err := rows.Scan(&g.TopicID, &g.TopicName, &g.Module, &g.AvgSelfRated, &g.AvgActual, &g.Gap); err != nil {
			return nil, fmt.Errorf("tutor: scan confidence gap: %w", err)
		}
		gaps = append(gaps, g)
	}
	return gaps, rows.Err()
}

// ---------- Module Stats ----------

// GetModuleStats returns aggregated stats for a module.
func (s *Store) GetModuleStats(module string) (*ModuleStats, error) {
	ms := &ModuleStats{Module: module}

	// Topic counts by status.
	err := s.db.QueryRow(
		"SELECT COUNT(*) FROM topics WHERE module = ?",
		module,
	).Scan(&ms.TopicCount)
	if err != nil {
		return nil, fmt.Errorf("tutor: get module stats: %w", err)
	}

	err = s.db.QueryRow(
		"SELECT COUNT(*) FROM topics WHERE module = ? AND status = 'completed'",
		module,
	).Scan(&ms.CompletedCount)
	if err != nil {
		return nil, fmt.Errorf("tutor: get module stats completed: %w", err)
	}

	err = s.db.QueryRow(
		"SELECT COUNT(*) FROM topics WHERE module = ? AND status = 'in_progress'",
		module,
	).Scan(&ms.InProgressCount)
	if err != nil {
		return nil, fmt.Errorf("tutor: get module stats in_progress: %w", err)
	}

	if ms.TopicCount > 0 {
		ms.CompletionPct = float64(ms.CompletedCount) / float64(ms.TopicCount) * 100
	}

	// Aggregate time and score from progress.
	var totalTime sql.NullInt64
	var avgScore sql.NullFloat64
	err = s.db.QueryRow(`
		SELECT COALESCE(SUM(p.time_spent_seconds), 0), AVG(p.score)
		FROM progress p
		JOIN topics t ON p.topic_id = t.id
		WHERE t.module = ?`,
		module,
	).Scan(&totalTime, &avgScore)
	if err != nil {
		return nil, fmt.Errorf("tutor: get module stats time: %w", err)
	}
	if totalTime.Valid {
		ms.TotalTimeSeconds = int(totalTime.Int64)
	}
	if avgScore.Valid {
		ms.AvgScore = avgScore.Float64
	}

	return ms, nil
}

// join concatenates strings with a separator — avoids importing strings.
func join(parts []string, sep string) string {
	if len(parts) == 0 {
		return ""
	}
	result := parts[0]
	for _, p := range parts[1:] {
		result += sep + p
	}
	return result
}
