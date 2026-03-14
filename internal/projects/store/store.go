package store

import (
	"database/sql"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

// ---------- Types ----------

// Project represents an AI/ML implementation project.
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

// ProjectSummary extends Project with milestone and keyword counts.
type ProjectSummary struct {
	Project
	MilestonesDone  int `json:"milestones_done"`
	MilestonesTotal int `json:"milestones_total"`
	KeywordCount    int `json:"keyword_count"`
}

// ProjectUpdate holds optional fields for partial project updates.
type ProjectUpdate struct {
	Status      *string  `json:"status,omitempty"`
	HoursActual *float64 `json:"hours_actual,omitempty"`
	GithubRepo  *string  `json:"github_repo,omitempty"`
	ReadmeURL   *string  `json:"readme_url,omitempty"`
}

// Milestone represents a project deliverable with acceptance criteria.
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

// Metric represents a recorded performance or quality measurement.
type Metric struct {
	ID         int    `json:"id"`
	ProjectID  int    `json:"project_id"`
	Name       string `json:"name"`
	Value      string `json:"value"`
	Unit       string `json:"unit"`
	CapturedAt string `json:"captured_at"`
}

// Keyword represents a resume keyword linked to a project.
type Keyword struct {
	ID        int    `json:"id"`
	ProjectID *int   `json:"project_id,omitempty"`
	Keyword   string `json:"keyword"`
	Status    string `json:"status"`
	ClaimedAt string `json:"claimed_at"`
	ShippedAt string `json:"shipped_at,omitempty"`
}

// ProfileSync tracks whether a project has been synced to a platform.
type ProfileSync struct {
	ID        int    `json:"id"`
	ProjectID int    `json:"project_id"`
	Platform  string `json:"platform"`
	Synced    bool   `json:"synced"`
	SyncedAt  string `json:"synced_at,omitempty"`
	Notes     string `json:"notes"`
	CreatedAt string `json:"created_at"`
}

// Readiness represents an interview readiness self-assessment.
type Readiness struct {
	ID           int    `json:"id"`
	ProjectID    int    `json:"project_id"`
	CanExplain   bool   `json:"can_explain"`
	CanDemo      bool   `json:"can_demo"`
	CanTradeoffs bool   `json:"can_tradeoffs"`
	SelfScore    int    `json:"self_score"`
	AssessedAt   string `json:"assessed_at"`
}

// DailyActivity aggregates daily project work metrics.
type DailyActivity struct {
	ID                  int    `json:"id"`
	Date                string `json:"date"`
	TimeSpentSeconds    int    `json:"time_spent_seconds"`
	ProjectsWorked      int    `json:"projects_worked"`
	MilestonesCompleted int    `json:"milestones_completed"`
}

// Dashboard holds aggregated project portfolio metrics.
type Dashboard struct {
	TotalProjects    int              `json:"total_projects"`
	Shipped          int              `json:"shipped"`
	Active           int              `json:"active"`
	Backlog          int              `json:"backlog"`
	Measuring        int              `json:"measuring"`
	Documenting      int              `json:"documenting"`
	KeywordsTotal    int              `json:"keywords_total"`
	KeywordsClaimed  int              `json:"keywords_claimed"`
	KeywordsBuilding int              `json:"keywords_building"`
	KeywordsShipped  int              `json:"keywords_shipped"`
	HoursEstimated   float64          `json:"hours_estimated"`
	HoursActual      float64          `json:"hours_actual"`
	AvgReadiness     float64          `json:"avg_readiness"`
	Projects         []ProjectSummary `json:"projects"`
}

// ProjectDetail holds a project with all its related data.
type ProjectDetail struct {
	Project
	Milestones []Milestone   `json:"milestones"`
	Metrics    []Metric      `json:"metrics"`
	Keywords   []Keyword     `json:"keywords"`
	Syncs      []ProfileSync `json:"syncs"`
	Readiness  *Readiness    `json:"readiness"`
}

// ---------- Store ----------

// Store provides SQLite-backed projects CRUD.
type Store struct {
	db     *sql.DB
	dbPath string
}

// Open creates a new Store with the given database path.
func Open(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("projects: open database: %w", err)
	}

	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		db.Close()
		return nil, fmt.Errorf("projects: enable WAL: %w", err)
	}
	if _, err := db.Exec("PRAGMA foreign_keys=ON"); err != nil {
		db.Close()
		return nil, fmt.Errorf("projects: enable foreign keys: %w", err)
	}
	db.SetMaxOpenConns(1)
	if _, err := db.Exec("PRAGMA busy_timeout=5000"); err != nil {
		db.Close()
		return nil, fmt.Errorf("projects: set busy timeout: %w", err)
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
		created_at TEXT NOT NULL DEFAULT (datetime('now')),
		UNIQUE(project_id, platform)
	);

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
		date TEXT UNIQUE NOT NULL,
		time_spent_seconds INTEGER NOT NULL DEFAULT 0,
		projects_worked INTEGER NOT NULL DEFAULT 0,
		milestones_completed INTEGER NOT NULL DEFAULT 0
	);
	CREATE INDEX IF NOT EXISTS idx_activity_date ON daily_activity(date);
	`
	if _, err := s.db.Exec(schema); err != nil {
		return fmt.Errorf("projects: migrate: %w", err)
	}
	return nil
}

// ---------- Projects ----------

// CreateProject inserts a new project.
func (s *Store) CreateProject(name, description string, phase, weekPlanned int, hoursEstimated float64) (int64, error) {
	res, err := s.db.Exec(
		"INSERT INTO projects (name, description, phase, week_planned, hours_estimated) VALUES (?, ?, ?, ?, ?)",
		name, description, phase, weekPlanned, hoursEstimated,
	)
	if err != nil {
		return 0, fmt.Errorf("projects: create project: %w", err)
	}
	return res.LastInsertId()
}

// GetProject retrieves a project by ID.
func (s *Store) GetProject(id int) (Project, error) {
	var p Project
	err := s.db.QueryRow(
		"SELECT id, name, description, phase, status, week_planned, hours_estimated, hours_actual, github_repo, readme_url, created_at, updated_at FROM projects WHERE id = ?",
		id,
	).Scan(&p.ID, &p.Name, &p.Description, &p.Phase, &p.Status, &p.WeekPlanned, &p.HoursEstimated, &p.HoursActual, &p.GithubRepo, &p.ReadmeURL, &p.CreatedAt, &p.UpdatedAt)
	if err == sql.ErrNoRows {
		return p, fmt.Errorf("projects: project not found: %d", id)
	}
	if err != nil {
		return p, fmt.Errorf("projects: get project: %w", err)
	}
	return p, nil
}

// GetProjectByName retrieves a project by its unique name.
func (s *Store) GetProjectByName(name string) (Project, error) {
	var p Project
	err := s.db.QueryRow(
		"SELECT id, name, description, phase, status, week_planned, hours_estimated, hours_actual, github_repo, readme_url, created_at, updated_at FROM projects WHERE name = ?",
		name,
	).Scan(&p.ID, &p.Name, &p.Description, &p.Phase, &p.Status, &p.WeekPlanned, &p.HoursEstimated, &p.HoursActual, &p.GithubRepo, &p.ReadmeURL, &p.CreatedAt, &p.UpdatedAt)
	if err == sql.ErrNoRows {
		return p, fmt.Errorf("projects: project not found: %s", name)
	}
	if err != nil {
		return p, fmt.Errorf("projects: get project by name: %w", err)
	}
	return p, nil
}

// ListProjects returns all projects with milestone and keyword counts.
func (s *Store) ListProjects() ([]ProjectSummary, error) {
	rows, err := s.db.Query(`
		SELECT p.id, p.name, p.description, p.phase, p.status, p.week_planned,
			p.hours_estimated, p.hours_actual, p.github_repo, p.readme_url,
			p.created_at, p.updated_at,
			COALESCE(SUM(CASE WHEN m.status = 'done' THEN 1 ELSE 0 END), 0) as milestones_done,
			COUNT(m.id) as milestones_total,
			(SELECT COUNT(*) FROM keywords WHERE project_id = p.id) as keyword_count
		FROM projects p
		LEFT JOIN milestones m ON m.project_id = p.id
		GROUP BY p.id
		ORDER BY p.phase, p.week_planned`)
	if err != nil {
		return nil, fmt.Errorf("projects: list projects: %w", err)
	}
	defer rows.Close()

	var projects []ProjectSummary
	for rows.Next() {
		var ps ProjectSummary
		if err := rows.Scan(
			&ps.ID, &ps.Name, &ps.Description, &ps.Phase, &ps.Status, &ps.WeekPlanned,
			&ps.HoursEstimated, &ps.HoursActual, &ps.GithubRepo, &ps.ReadmeURL,
			&ps.CreatedAt, &ps.UpdatedAt,
			&ps.MilestonesDone, &ps.MilestonesTotal, &ps.KeywordCount,
		); err != nil {
			return nil, fmt.Errorf("projects: scan project summary: %w", err)
		}
		projects = append(projects, ps)
	}
	return projects, rows.Err()
}

// UpdateProject applies partial updates to a project using non-nil fields.
func (s *Store) UpdateProject(id int, u ProjectUpdate) error {
	sets := []string{"updated_at = datetime('now')"}
	var args []any
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
	query := "UPDATE projects SET " + join(sets, ", ") + " WHERE id = ?"
	result, err := s.db.Exec(query, args...)
	if err != nil {
		return fmt.Errorf("projects: update project: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("projects: project not found: %d", id)
	}
	return nil
}

// ProjectCount returns the total number of projects.
func (s *Store) ProjectCount() (int, error) {
	var count int
	err := s.db.QueryRow("SELECT COUNT(*) FROM projects").Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("projects: count projects: %w", err)
	}
	return count, nil
}

// ---------- Milestones ----------

// CreateMilestone inserts a new milestone for a project.
func (s *Store) CreateMilestone(projectID int, name, description, acceptanceCriteria string, sortOrder int) (int64, error) {
	res, err := s.db.Exec(
		"INSERT INTO milestones (project_id, name, description, acceptance_criteria, sort_order) VALUES (?, ?, ?, ?, ?)",
		projectID, name, description, acceptanceCriteria, sortOrder,
	)
	if err != nil {
		return 0, fmt.Errorf("projects: create milestone: %w", err)
	}
	return res.LastInsertId()
}

// ListMilestones returns milestones for a project ordered by sort_order.
func (s *Store) ListMilestones(projectID int) ([]Milestone, error) {
	rows, err := s.db.Query(
		"SELECT id, project_id, name, description, acceptance_criteria, status, COALESCE(completed_at, ''), sort_order FROM milestones WHERE project_id = ? ORDER BY sort_order",
		projectID,
	)
	if err != nil {
		return nil, fmt.Errorf("projects: list milestones: %w", err)
	}
	defer rows.Close()

	var milestones []Milestone
	for rows.Next() {
		var m Milestone
		if err := rows.Scan(&m.ID, &m.ProjectID, &m.Name, &m.Description, &m.AcceptanceCriteria, &m.Status, &m.CompletedAt, &m.SortOrder); err != nil {
			return nil, fmt.Errorf("projects: scan milestone: %w", err)
		}
		milestones = append(milestones, m)
	}
	return milestones, rows.Err()
}

// UpdateMilestoneStatus changes a milestone's status. Sets completed_at for "done".
func (s *Store) UpdateMilestoneStatus(id int, status string) error {
	var query string
	if status == "done" {
		query = "UPDATE milestones SET status = ?, completed_at = datetime('now') WHERE id = ?"
	} else {
		query = "UPDATE milestones SET status = ?, completed_at = NULL WHERE id = ?"
	}
	result, err := s.db.Exec(query, status, id)
	if err != nil {
		return fmt.Errorf("projects: update milestone status: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("projects: milestone not found: %d", id)
	}
	return nil
}

// ---------- Metrics ----------

// RecordMetric inserts a new metric for a project.
func (s *Store) RecordMetric(projectID int, name, value, unit string) (int64, error) {
	res, err := s.db.Exec(
		"INSERT INTO metrics (project_id, name, value, unit) VALUES (?, ?, ?, ?)",
		projectID, name, value, unit,
	)
	if err != nil {
		return 0, fmt.Errorf("projects: record metric: %w", err)
	}
	return res.LastInsertId()
}

// ListMetrics returns metrics for a project ordered by captured_at DESC.
func (s *Store) ListMetrics(projectID int) ([]Metric, error) {
	rows, err := s.db.Query(
		"SELECT id, project_id, name, value, unit, captured_at FROM metrics WHERE project_id = ? ORDER BY captured_at DESC, id DESC",
		projectID,
	)
	if err != nil {
		return nil, fmt.Errorf("projects: list metrics: %w", err)
	}
	defer rows.Close()

	var metrics []Metric
	for rows.Next() {
		var m Metric
		if err := rows.Scan(&m.ID, &m.ProjectID, &m.Name, &m.Value, &m.Unit, &m.CapturedAt); err != nil {
			return nil, fmt.Errorf("projects: scan metric: %w", err)
		}
		metrics = append(metrics, m)
	}
	return metrics, rows.Err()
}

// ---------- Keywords ----------

// CreateKeyword inserts a keyword. Uses INSERT OR IGNORE for duplicates.
func (s *Store) CreateKeyword(projectID *int, keyword string) (int64, error) {
	res, err := s.db.Exec(
		"INSERT OR IGNORE INTO keywords (project_id, keyword) VALUES (?, ?)",
		projectID, keyword,
	)
	if err != nil {
		return 0, fmt.Errorf("projects: create keyword: %w", err)
	}
	id, _ := res.LastInsertId()
	return id, nil
}

// ListKeywords returns all keywords ordered by status then keyword.
func (s *Store) ListKeywords() ([]Keyword, error) {
	rows, err := s.db.Query(
		"SELECT id, project_id, keyword, status, claimed_at, COALESCE(shipped_at, '') FROM keywords ORDER BY status, keyword",
	)
	if err != nil {
		return nil, fmt.Errorf("projects: list keywords: %w", err)
	}
	defer rows.Close()

	var keywords []Keyword
	for rows.Next() {
		var k Keyword
		var shippedAt string
		if err := rows.Scan(&k.ID, &k.ProjectID, &k.Keyword, &k.Status, &k.ClaimedAt, &shippedAt); err != nil {
			return nil, fmt.Errorf("projects: scan keyword: %w", err)
		}
		if shippedAt != "" {
			k.ShippedAt = shippedAt
		}
		keywords = append(keywords, k)
	}
	return keywords, rows.Err()
}

// ListProjectKeywords returns keywords for a specific project.
func (s *Store) ListProjectKeywords(projectID int) ([]Keyword, error) {
	rows, err := s.db.Query(
		"SELECT id, project_id, keyword, status, claimed_at, COALESCE(shipped_at, '') FROM keywords WHERE project_id = ? ORDER BY keyword",
		projectID,
	)
	if err != nil {
		return nil, fmt.Errorf("projects: list project keywords: %w", err)
	}
	defer rows.Close()

	var keywords []Keyword
	for rows.Next() {
		var k Keyword
		var shippedAt string
		if err := rows.Scan(&k.ID, &k.ProjectID, &k.Keyword, &k.Status, &k.ClaimedAt, &shippedAt); err != nil {
			return nil, fmt.Errorf("projects: scan keyword: %w", err)
		}
		if shippedAt != "" {
			k.ShippedAt = shippedAt
		}
		keywords = append(keywords, k)
	}
	return keywords, rows.Err()
}

// UpdateKeywordStatus changes a keyword's status. Sets shipped_at for "shipped".
func (s *Store) UpdateKeywordStatus(id int, status string) error {
	var query string
	if status == "shipped" {
		query = "UPDATE keywords SET status = ?, shipped_at = datetime('now') WHERE id = ?"
	} else {
		query = "UPDATE keywords SET status = ?, shipped_at = NULL WHERE id = ?"
	}
	result, err := s.db.Exec(query, status, id)
	if err != nil {
		return fmt.Errorf("projects: update keyword status: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("projects: keyword not found: %d", id)
	}
	return nil
}

// ---------- Profile Syncs ----------

// CreateProfileSync inserts a new profile sync row for a project and platform.
func (s *Store) CreateProfileSync(projectID int, platform string) (int64, error) {
	res, err := s.db.Exec(
		"INSERT INTO profile_syncs (project_id, platform) VALUES (?, ?)",
		projectID, platform,
	)
	if err != nil {
		return 0, fmt.Errorf("projects: create profile sync: %w", err)
	}
	return res.LastInsertId()
}

// ListProfileSyncs returns profile syncs for a project.
func (s *Store) ListProfileSyncs(projectID int) ([]ProfileSync, error) {
	rows, err := s.db.Query(
		"SELECT id, project_id, platform, synced, COALESCE(synced_at, ''), notes, created_at FROM profile_syncs WHERE project_id = ? ORDER BY platform",
		projectID,
	)
	if err != nil {
		return nil, fmt.Errorf("projects: list profile syncs: %w", err)
	}
	defer rows.Close()

	var syncs []ProfileSync
	for rows.Next() {
		var ps ProfileSync
		var synced int
		var syncedAt string
		if err := rows.Scan(&ps.ID, &ps.ProjectID, &ps.Platform, &synced, &syncedAt, &ps.Notes, &ps.CreatedAt); err != nil {
			return nil, fmt.Errorf("projects: scan profile sync: %w", err)
		}
		ps.Synced = synced == 1
		if syncedAt != "" {
			ps.SyncedAt = syncedAt
		}
		syncs = append(syncs, ps)
	}
	return syncs, rows.Err()
}

// UpdateProfileSync marks a platform as synced with optional notes.
func (s *Store) UpdateProfileSync(projectID int, platform, notes string) error {
	result, err := s.db.Exec(
		"UPDATE profile_syncs SET synced = 1, synced_at = datetime('now'), notes = ? WHERE project_id = ? AND platform = ?",
		notes, projectID, platform,
	)
	if err != nil {
		return fmt.Errorf("projects: update profile sync: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("projects: profile sync not found: project %d, platform %s", projectID, platform)
	}
	return nil
}

// ---------- Interview Readiness ----------

// RecordReadiness inserts a new readiness assessment for a project.
func (s *Store) RecordReadiness(projectID int, canExplain, canDemo, canTradeoffs bool, selfScore int) (int64, error) {
	res, err := s.db.Exec(
		"INSERT INTO interview_readiness (project_id, can_explain, can_demo, can_tradeoffs, self_score) VALUES (?, ?, ?, ?, ?)",
		projectID, boolToInt(canExplain), boolToInt(canDemo), boolToInt(canTradeoffs), selfScore,
	)
	if err != nil {
		return 0, fmt.Errorf("projects: record readiness: %w", err)
	}
	return res.LastInsertId()
}

// GetReadiness returns the latest readiness assessment for a project.
func (s *Store) GetReadiness(projectID int) (*Readiness, error) {
	var r Readiness
	var canExplain, canDemo, canTradeoffs int
	err := s.db.QueryRow(
		"SELECT id, project_id, can_explain, can_demo, can_tradeoffs, self_score, assessed_at FROM interview_readiness WHERE project_id = ? ORDER BY assessed_at DESC, id DESC LIMIT 1",
		projectID,
	).Scan(&r.ID, &r.ProjectID, &canExplain, &canDemo, &canTradeoffs, &r.SelfScore, &r.AssessedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("projects: get readiness: %w", err)
	}
	r.CanExplain = canExplain == 1
	r.CanDemo = canDemo == 1
	r.CanTradeoffs = canTradeoffs == 1
	return &r, nil
}

// ---------- Daily Activity ----------

// UpsertDailyActivity creates or updates a daily activity record.
func (s *Store) UpsertDailyActivity(date string, timeSpent, projectsWorked, milestonesCompleted int) error {
	_, err := s.db.Exec(`
		INSERT INTO daily_activity (date, time_spent_seconds, projects_worked, milestones_completed)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(date) DO UPDATE SET
			time_spent_seconds = daily_activity.time_spent_seconds + excluded.time_spent_seconds,
			projects_worked = excluded.projects_worked,
			milestones_completed = daily_activity.milestones_completed + excluded.milestones_completed`,
		date, timeSpent, projectsWorked, milestonesCompleted,
	)
	if err != nil {
		return fmt.Errorf("projects: upsert daily activity: %w", err)
	}
	return nil
}

// GetActivity returns the last N days of daily activity.
func (s *Store) GetActivity(days int) ([]DailyActivity, error) {
	cutoff := time.Now().AddDate(0, 0, -days).Format("2006-01-02")
	rows, err := s.db.Query(
		"SELECT id, date, time_spent_seconds, projects_worked, milestones_completed FROM daily_activity WHERE date >= ? ORDER BY date DESC",
		cutoff,
	)
	if err != nil {
		return nil, fmt.Errorf("projects: get activity: %w", err)
	}
	defer rows.Close()

	var activities []DailyActivity
	for rows.Next() {
		var da DailyActivity
		if err := rows.Scan(&da.ID, &da.Date, &da.TimeSpentSeconds, &da.ProjectsWorked, &da.MilestonesCompleted); err != nil {
			return nil, fmt.Errorf("projects: scan activity: %w", err)
		}
		activities = append(activities, da)
	}
	return activities, rows.Err()
}

// ---------- Dashboard ----------

// GetDashboard returns aggregated portfolio metrics in a single transaction.
func (s *Store) GetDashboard() (Dashboard, error) {
	var d Dashboard

	tx, err := s.db.Begin()
	if err != nil {
		return d, fmt.Errorf("projects: begin dashboard tx: %w", err)
	}
	defer tx.Rollback()

	// Project status counts.
	err = tx.QueryRow("SELECT COUNT(*) FROM projects").Scan(&d.TotalProjects)
	if err != nil {
		return d, fmt.Errorf("projects: dashboard total: %w", err)
	}
	tx.QueryRow("SELECT COUNT(*) FROM projects WHERE status = 'shipped'").Scan(&d.Shipped)
	tx.QueryRow("SELECT COUNT(*) FROM projects WHERE status = 'active'").Scan(&d.Active)
	tx.QueryRow("SELECT COUNT(*) FROM projects WHERE status = 'backlog'").Scan(&d.Backlog)
	tx.QueryRow("SELECT COUNT(*) FROM projects WHERE status = 'measuring'").Scan(&d.Measuring)
	tx.QueryRow("SELECT COUNT(*) FROM projects WHERE status = 'documenting'").Scan(&d.Documenting)

	// Keyword status counts.
	tx.QueryRow("SELECT COUNT(*) FROM keywords").Scan(&d.KeywordsTotal)
	tx.QueryRow("SELECT COUNT(*) FROM keywords WHERE status = 'claimed'").Scan(&d.KeywordsClaimed)
	tx.QueryRow("SELECT COUNT(*) FROM keywords WHERE status = 'building'").Scan(&d.KeywordsBuilding)
	tx.QueryRow("SELECT COUNT(*) FROM keywords WHERE status = 'shipped'").Scan(&d.KeywordsShipped)

	// Hours.
	tx.QueryRow("SELECT COALESCE(SUM(hours_estimated), 0), COALESCE(SUM(hours_actual), 0) FROM projects").Scan(&d.HoursEstimated, &d.HoursActual)

	// Average readiness (latest per project).
	var avgReadiness sql.NullFloat64
	tx.QueryRow(`
		SELECT AVG(self_score) FROM (
			SELECT self_score FROM interview_readiness ir
			WHERE ir.assessed_at = (
				SELECT MAX(ir2.assessed_at) FROM interview_readiness ir2 WHERE ir2.project_id = ir.project_id
			)
		)`).Scan(&avgReadiness)
	if avgReadiness.Valid {
		d.AvgReadiness = avgReadiness.Float64
	}

	// Project list with milestone and keyword counts.
	rows, err := tx.Query(`
		SELECT p.id, p.name, p.description, p.phase, p.status, p.week_planned,
			p.hours_estimated, p.hours_actual, p.github_repo, p.readme_url,
			p.created_at, p.updated_at,
			COALESCE(SUM(CASE WHEN m.status = 'done' THEN 1 ELSE 0 END), 0) as milestones_done,
			COUNT(m.id) as milestones_total,
			(SELECT COUNT(*) FROM keywords WHERE project_id = p.id) as keyword_count
		FROM projects p
		LEFT JOIN milestones m ON m.project_id = p.id
		GROUP BY p.id
		ORDER BY p.phase, p.week_planned`)
	if err != nil {
		return d, fmt.Errorf("projects: dashboard project list: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var ps ProjectSummary
		if err := rows.Scan(
			&ps.ID, &ps.Name, &ps.Description, &ps.Phase, &ps.Status, &ps.WeekPlanned,
			&ps.HoursEstimated, &ps.HoursActual, &ps.GithubRepo, &ps.ReadmeURL,
			&ps.CreatedAt, &ps.UpdatedAt,
			&ps.MilestonesDone, &ps.MilestonesTotal, &ps.KeywordCount,
		); err != nil {
			return d, fmt.Errorf("projects: scan dashboard project: %w", err)
		}
		d.Projects = append(d.Projects, ps)
	}
	if err := rows.Err(); err != nil {
		return d, err
	}

	return d, tx.Commit()
}

// ---------- Helpers ----------

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
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
