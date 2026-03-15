package store

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

// Lead represents a lead in the scout pipeline.
type Lead struct {
	ID             int64   `json:"id"`
	Title          string  `json:"title"`
	Company        string  `json:"company"`
	Type           string  `json:"type"`
	Source         string  `json:"source"`
	SourceURL      string  `json:"sourceUrl"`
	Pipeline       string  `json:"pipeline"`
	Stage          string  `json:"stage"`
	Compensation   string  `json:"compensation"`
	Currency       string  `json:"currency"`
	Contact        string  `json:"contact"`
	Location       string  `json:"location"`
	Tags           string  `json:"tags"`
	Notes          string  `json:"notes"`
	Metadata       string  `json:"metadata"`
	Variant        string  `json:"variant"`
	NextAction     string  `json:"nextAction"`
	NextDate       string  `json:"nextDate"`
	CreatedAt      string  `json:"createdAt"`
	UpdatedAt      string  `json:"updatedAt"`
	ClosedAt       string  `json:"closedAt"`
	MatchScore     float64 `json:"matchScore"`
	JobDescription string  `json:"jobDescription"`
}

// StageHistory records a stage transition for a lead.
type StageHistory struct {
	ID        int64  `json:"id"`
	LeadID    int64  `json:"leadId"`
	FromStage string `json:"fromStage"`
	ToStage   string `json:"toStage"`
	ChangedAt string `json:"changedAt"`
	Notes     string `json:"notes"`
}

// SyncResult records a platform sync check.
type SyncResult struct {
	ID        int64  `json:"id"`
	Platform  string `json:"platform"`
	Status    string `json:"status"`
	Issues    string `json:"issues"`
	Details   string `json:"details"`
	CheckedAt string `json:"checkedAt"`
}

// SyncMeta stores key-value sync metadata.
type SyncMeta struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// Optimization records a profile optimization.
type Optimization struct {
	ID          int64  `json:"id"`
	Platform    string `json:"platform"`
	Section     string `json:"section"`
	Field       string `json:"field"`
	Previous    string `json:"previous"`
	Updated     string `json:"updated"`
	Status      string `json:"status"`
	OptimizedAt string `json:"optimizedAt"`
}

// AgentRun records an agent execution.
type AgentRun struct {
	ID              int64  `json:"id"`
	Platform        string `json:"platform"`
	Mode            string `json:"mode"`
	LeadID          int64  `json:"leadId"`
	Status          string `json:"status"`
	Recommendations string `json:"recommendations"`
	ApprovedChanges string `json:"approvedChanges"`
	Result          string `json:"result"`
	CreatedAt       string `json:"createdAt"`
	CompletedAt     string `json:"completedAt"`
}

// PlatformTrust tracks trust metrics for a platform.
type PlatformTrust struct {
	Platform                string `json:"platform"`
	SuccessfulOptimizations int    `json:"successfulOptimizations"`
	SuccessfulActions       int    `json:"successfulActions"`
	LastSuccessAt           string `json:"lastSuccessAt"`
}

// Store provides SQLite-backed scout pipeline CRUD.
type Store struct {
	db     *sql.DB
	dbPath string
}

// Open creates a new Store with the given database path.
func Open(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("scout: open database: %w", err)
	}

	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		db.Close()
		return nil, fmt.Errorf("scout: enable WAL: %w", err)
	}
	if _, err := db.Exec("PRAGMA foreign_keys=ON"); err != nil {
		db.Close()
		return nil, fmt.Errorf("scout: enable foreign keys: %w", err)
	}
	db.SetMaxOpenConns(1)
	if _, err := db.Exec("PRAGMA busy_timeout=5000"); err != nil {
		db.Close()
		return nil, fmt.Errorf("scout: set busy timeout: %w", err)
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
	CREATE TABLE IF NOT EXISTS leads (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		title TEXT NOT NULL,
		company TEXT NOT NULL DEFAULT '',
		type TEXT NOT NULL,
		source TEXT NOT NULL DEFAULT '',
		source_url TEXT NOT NULL DEFAULT '',
		pipeline TEXT NOT NULL DEFAULT '',
		stage TEXT NOT NULL DEFAULT '',
		compensation TEXT NOT NULL DEFAULT '',
		currency TEXT NOT NULL DEFAULT '',
		contact TEXT NOT NULL DEFAULT '',
		location TEXT NOT NULL DEFAULT '',
		tags TEXT NOT NULL DEFAULT '',
		notes TEXT NOT NULL DEFAULT '',
		metadata TEXT NOT NULL DEFAULT '',
		variant TEXT NOT NULL DEFAULT '',
		next_action TEXT NOT NULL DEFAULT '',
		next_date TEXT NOT NULL DEFAULT '',
		created_at TEXT NOT NULL,
		updated_at TEXT NOT NULL,
		closed_at TEXT NOT NULL DEFAULT '',
		match_score REAL DEFAULT 0,
		job_description TEXT NOT NULL DEFAULT ''
	);
	CREATE INDEX IF NOT EXISTS idx_leads_type ON leads(type);
	CREATE INDEX IF NOT EXISTS idx_leads_stage ON leads(stage);
	CREATE INDEX IF NOT EXISTS idx_leads_match_score ON leads(match_score);

	CREATE TABLE IF NOT EXISTS stage_history (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		lead_id INTEGER REFERENCES leads(id),
		from_stage TEXT NOT NULL DEFAULT '',
		to_stage TEXT NOT NULL DEFAULT '',
		changed_at TEXT NOT NULL,
		notes TEXT NOT NULL DEFAULT ''
	);
	CREATE INDEX IF NOT EXISTS idx_stage_history_lead_id ON stage_history(lead_id);

	CREATE TABLE IF NOT EXISTS sync_results (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		platform TEXT NOT NULL DEFAULT '',
		status TEXT NOT NULL DEFAULT '',
		issues TEXT NOT NULL DEFAULT '',
		details TEXT NOT NULL DEFAULT '',
		checked_at TEXT NOT NULL
	);

	CREATE TABLE IF NOT EXISTS sync_meta (
		key TEXT PRIMARY KEY,
		value TEXT NOT NULL DEFAULT ''
	);

	CREATE TABLE IF NOT EXISTS optimizations (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		platform TEXT NOT NULL DEFAULT '',
		section TEXT NOT NULL DEFAULT '',
		field TEXT NOT NULL DEFAULT '',
		previous TEXT NOT NULL DEFAULT '',
		updated TEXT NOT NULL DEFAULT '',
		status TEXT NOT NULL DEFAULT '',
		optimized_at TEXT NOT NULL
	);

	CREATE TABLE IF NOT EXISTS agent_runs (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		platform TEXT NOT NULL DEFAULT '',
		mode TEXT NOT NULL DEFAULT '',
		lead_id INTEGER,
		status TEXT NOT NULL DEFAULT '',
		recommendations TEXT NOT NULL DEFAULT '',
		approved_changes TEXT NOT NULL DEFAULT '',
		result TEXT NOT NULL DEFAULT '',
		created_at TEXT NOT NULL,
		completed_at TEXT NOT NULL DEFAULT ''
	);

	CREATE TABLE IF NOT EXISTS platform_trust (
		platform TEXT PRIMARY KEY,
		successful_optimizations INTEGER DEFAULT 0,
		successful_actions INTEGER DEFAULT 0,
		last_success_at TEXT NOT NULL DEFAULT ''
	);
	`
	if _, err := s.db.Exec(schema); err != nil {
		return fmt.Errorf("scout: migrate: %w", err)
	}
	return nil
}

// DB returns the underlying database for advanced queries.
func (s *Store) DB() *sql.DB {
	return s.db
}

// now returns the current UTC time in RFC3339 format.
func now() string {
	return time.Now().UTC().Format(time.RFC3339)
}

// nullStr converts a sql.NullString to a plain string.
func nullStr(ns sql.NullString) string {
	if ns.Valid {
		return ns.String
	}
	return ""
}

// scanLead scans a lead from a row scanner.
func scanLead(sc interface{ Scan(...interface{}) error }) (*Lead, error) {
	var l Lead
	err := sc.Scan(
		&l.ID, &l.Title, &l.Company, &l.Type, &l.Source, &l.SourceURL,
		&l.Pipeline, &l.Stage, &l.Compensation, &l.Currency, &l.Contact,
		&l.Location, &l.Tags, &l.Notes, &l.Metadata, &l.Variant,
		&l.NextAction, &l.NextDate, &l.CreatedAt, &l.UpdatedAt,
		&l.ClosedAt, &l.MatchScore, &l.JobDescription,
	)
	return &l, err
}

const leadColumns = `id, title, company, type, source, source_url, pipeline, stage,
	compensation, currency, contact, location, tags, notes, metadata, variant,
	next_action, next_date, created_at, updated_at, closed_at, match_score, job_description`

// allowedLeadFields defines which fields can be updated dynamically.
var allowedLeadFields = map[string]bool{
	"title": true, "company": true, "type": true, "source": true,
	"source_url": true, "pipeline": true, "stage": true, "compensation": true,
	"currency": true, "contact": true, "location": true, "tags": true,
	"notes": true, "metadata": true, "variant": true, "next_action": true,
	"next_date": true, "closed_at": true, "match_score": true,
	"job_description": true,
}

// UpdateLead modifies lead fields dynamically. Only allowed fields are applied.
func (s *Store) UpdateLead(id int64, fields map[string]interface{}) error {
	var setClauses []string
	var args []interface{}

	for k, v := range fields {
		if !allowedLeadFields[k] {
			continue
		}
		setClauses = append(setClauses, k+" = ?")
		args = append(args, v)
	}
	if len(setClauses) == 0 {
		return nil
	}

	setClauses = append(setClauses, "updated_at = ?")
	args = append(args, now())
	args = append(args, id)

	result, err := s.db.Exec(
		"UPDATE leads SET "+strings.Join(setClauses, ", ")+" WHERE id = ?",
		args...,
	)
	if err != nil {
		return fmt.Errorf("scout: update lead: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("scout: lead not found: %d", id)
	}
	return nil
}
