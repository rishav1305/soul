package store

import (
	"database/sql"
	"fmt"

	"strings"
	"time"

	_ "modernc.org/sqlite"
)

// Lead represents a job lead sourced from TheirStack or other platforms.
type Lead struct {
	// Scout internal fields.
	ID         int64   `json:"id"`
	Source     string  `json:"source"`
	Pipeline   string  `json:"pipeline"`
	Stage      string  `json:"stage"`
	MatchScore float64 `json:"matchScore"`
	NextAction string  `json:"nextAction"`
	NextDate   string  `json:"nextDate"`
	Notes      string  `json:"notes"`
	CreatedAt  string  `json:"createdAt"`
	UpdatedAt  string  `json:"updatedAt"`
	ClosedAt   string  `json:"closedAt"`

	// TheirStack identity fields.
	TheirStackID    *int64 `json:"theirStackId"`
	JobTitle        string `json:"jobTitle"`
	URL             string `json:"url"`
	FinalURL        string `json:"finalUrl"`
	SourceURL       string `json:"sourceUrl"`
	DatePosted      string `json:"datePosted"`
	DiscoveredAt    string `json:"discoveredAt"`
	Description     string `json:"description"`
	NormalizedTitle string `json:"normalizedTitle"`

	// Location fields.
	Location      string `json:"location"`
	ShortLocation string `json:"shortLocation"`
	Country       string `json:"country"`
	CountryCode   string `json:"countryCode"`
	Remote        bool   `json:"remote"`
	Hybrid        bool   `json:"hybrid"`

	// Compensation fields.
	SalaryString        string  `json:"salaryString"`
	MinAnnualSalaryUSD  float64 `json:"minAnnualSalaryUsd"`
	MaxAnnualSalaryUSD  float64 `json:"maxAnnualSalaryUsd"`
	SalaryCurrency      string  `json:"salaryCurrency"`

	// Job attribute fields.
	Seniority          string `json:"seniority"`
	EmploymentStatuses string `json:"employmentStatuses"`
	EasyApply          bool   `json:"easyApply"`
	TechnologySlugs    string `json:"technologySlugs"`
	KeywordSlugs       string `json:"keywordSlugs"`

	// Company fields.
	Company                string  `json:"company"`
	CompanyDomain          string  `json:"companyDomain"`
	CompanyIndustry        string  `json:"companyIndustry"`
	CompanyEmployeeCount   int     `json:"companyEmployeeCount"`
	CompanyLinkedInURL     string  `json:"companyLinkedInUrl"`
	CompanyTotalFundingUSD float64 `json:"companyTotalFundingUsd"`
	CompanyFundingStage    string  `json:"companyFundingStage"`
	CompanyLogo            string  `json:"companyLogo"`
	CompanyCountry         string  `json:"companyCountry"`

	// Hiring fields.
	HiringManager         string `json:"hiringManager"`
	HiringManagerLinkedIn string `json:"hiringManagerLinkedIn"`

	// Metadata.
	Metadata string `json:"metadata"`
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
	// Check if leads table exists with the new schema (theirstack_id column).
	var colCount int
	_ = s.db.QueryRow("SELECT COUNT(*) FROM pragma_table_info('leads') WHERE name = 'theirstack_id'").Scan(&colCount)

	if colCount == 0 {
		// Old or missing schema — check if table has data before dropping.
		var rowCount int
		err := s.db.QueryRow("SELECT COUNT(*) FROM leads").Scan(&rowCount)
		if err == nil && rowCount > 0 {
			return fmt.Errorf("scout: leads table has %d rows with old schema — back up scout.db and delete it to proceed", rowCount)
		}
		// Safe to drop — table is empty or doesn't exist.
		if _, err := s.db.Exec("DROP TABLE IF EXISTS leads"); err != nil {
			return fmt.Errorf("scout: migrate: drop leads: %w", err)
		}
	}

	const leadsSchema = `
CREATE TABLE IF NOT EXISTS leads (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    source TEXT NOT NULL DEFAULT 'theirstack',
    pipeline TEXT NOT NULL DEFAULT '',
    stage TEXT NOT NULL DEFAULT 'discovered',
    match_score REAL DEFAULT 0,
    next_action TEXT NOT NULL DEFAULT 'review',
    next_date TEXT NOT NULL DEFAULT '',
    notes TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    closed_at TEXT NOT NULL DEFAULT '',
    theirstack_id INTEGER,
    job_title TEXT NOT NULL DEFAULT '',
    url TEXT NOT NULL DEFAULT '',
    final_url TEXT NOT NULL DEFAULT '',
    source_url TEXT NOT NULL DEFAULT '',
    date_posted TEXT NOT NULL DEFAULT '',
    discovered_at TEXT NOT NULL DEFAULT '',
    description TEXT NOT NULL DEFAULT '',
    normalized_title TEXT NOT NULL DEFAULT '',
    location TEXT NOT NULL DEFAULT '',
    short_location TEXT NOT NULL DEFAULT '',
    country TEXT NOT NULL DEFAULT '',
    country_code TEXT NOT NULL DEFAULT '',
    remote INTEGER DEFAULT 0,
    hybrid INTEGER DEFAULT 0,
    salary_string TEXT NOT NULL DEFAULT '',
    min_annual_salary_usd REAL DEFAULT 0,
    max_annual_salary_usd REAL DEFAULT 0,
    salary_currency TEXT NOT NULL DEFAULT '',
    seniority TEXT NOT NULL DEFAULT '',
    employment_statuses TEXT NOT NULL DEFAULT '[]',
    easy_apply INTEGER DEFAULT 0,
    technology_slugs TEXT NOT NULL DEFAULT '[]',
    keyword_slugs TEXT NOT NULL DEFAULT '[]',
    company TEXT NOT NULL DEFAULT '',
    company_domain TEXT NOT NULL DEFAULT '',
    company_industry TEXT NOT NULL DEFAULT '',
    company_employee_count INTEGER DEFAULT 0,
    company_linkedin_url TEXT NOT NULL DEFAULT '',
    company_total_funding_usd REAL DEFAULT 0,
    company_funding_stage TEXT NOT NULL DEFAULT '',
    company_logo TEXT NOT NULL DEFAULT '',
    company_country TEXT NOT NULL DEFAULT '',
    hiring_manager TEXT NOT NULL DEFAULT '',
    hiring_manager_linkedin TEXT NOT NULL DEFAULT '',
    metadata TEXT NOT NULL DEFAULT '{}'
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_leads_theirstack_id ON leads(theirstack_id) WHERE theirstack_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_leads_stage ON leads(stage);
CREATE INDEX IF NOT EXISTS idx_leads_match_score ON leads(match_score);
CREATE INDEX IF NOT EXISTS idx_leads_country_code ON leads(country_code);
CREATE INDEX IF NOT EXISTS idx_leads_seniority ON leads(seniority);
CREATE INDEX IF NOT EXISTS idx_leads_remote ON leads(remote);
CREATE INDEX IF NOT EXISTS idx_leads_pipeline ON leads(pipeline);
CREATE INDEX IF NOT EXISTS idx_leads_company_domain ON leads(company_domain);
CREATE INDEX IF NOT EXISTS idx_leads_date_posted ON leads(date_posted);
`

	if _, err := s.db.Exec(leadsSchema); err != nil {
		return fmt.Errorf("scout: migrate: create leads: %w", err)
	}

	const otherSchema = `
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
	if _, err := s.db.Exec(otherSchema); err != nil {
		return fmt.Errorf("scout: migrate: create other tables: %w", err)
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

// scanLead scans a lead from a row scanner. Columns must match leadColumns exactly.
func scanLead(sc interface{ Scan(...interface{}) error }) (*Lead, error) {
	var l Lead
	err := sc.Scan(
		&l.ID, &l.Source, &l.Pipeline, &l.Stage, &l.MatchScore,
		&l.NextAction, &l.NextDate, &l.Notes, &l.CreatedAt, &l.UpdatedAt, &l.ClosedAt,
		&l.TheirStackID,
		&l.JobTitle, &l.URL, &l.FinalURL, &l.SourceURL,
		&l.DatePosted, &l.DiscoveredAt, &l.Description, &l.NormalizedTitle,
		&l.Location, &l.ShortLocation, &l.Country, &l.CountryCode, &l.Remote, &l.Hybrid,
		&l.SalaryString, &l.MinAnnualSalaryUSD, &l.MaxAnnualSalaryUSD, &l.SalaryCurrency,
		&l.Seniority, &l.EmploymentStatuses, &l.EasyApply, &l.TechnologySlugs, &l.KeywordSlugs,
		&l.Company, &l.CompanyDomain, &l.CompanyIndustry, &l.CompanyEmployeeCount,
		&l.CompanyLinkedInURL, &l.CompanyTotalFundingUSD, &l.CompanyFundingStage,
		&l.CompanyLogo, &l.CompanyCountry,
		&l.HiringManager, &l.HiringManagerLinkedIn,
		&l.Metadata,
	)
	return &l, err
}

// leadColumns lists all lead columns in the same order as scanLead.
const leadColumns = `id, source, pipeline, stage, match_score,
	next_action, next_date, notes, created_at, updated_at, closed_at,
	theirstack_id,
	job_title, url, final_url, source_url,
	date_posted, discovered_at, description, normalized_title,
	location, short_location, country, country_code, remote, hybrid,
	salary_string, min_annual_salary_usd, max_annual_salary_usd, salary_currency,
	seniority, employment_statuses, easy_apply, technology_slugs, keyword_slugs,
	company, company_domain, company_industry, company_employee_count,
	company_linkedin_url, company_total_funding_usd, company_funding_stage,
	company_logo, company_country,
	hiring_manager, hiring_manager_linkedin,
	metadata`

// allowedLeadFields defines which fields can be updated dynamically.
// Excludes: id, created_at, theirstack_id.
var allowedLeadFields = map[string]bool{
	"source":                    true,
	"pipeline":                  true,
	"stage":                     true,
	"match_score":               true,
	"next_action":               true,
	"next_date":                 true,
	"notes":                     true,
	"updated_at":                true,
	"closed_at":                 true,
	"job_title":                 true,
	"url":                       true,
	"final_url":                 true,
	"source_url":                true,
	"date_posted":               true,
	"discovered_at":             true,
	"description":               true,
	"normalized_title":          true,
	"location":                  true,
	"short_location":            true,
	"country":                   true,
	"country_code":              true,
	"remote":                    true,
	"hybrid":                    true,
	"salary_string":             true,
	"min_annual_salary_usd":     true,
	"max_annual_salary_usd":     true,
	"salary_currency":           true,
	"seniority":                 true,
	"employment_statuses":       true,
	"easy_apply":                true,
	"technology_slugs":          true,
	"keyword_slugs":             true,
	"company":                   true,
	"company_domain":            true,
	"company_industry":          true,
	"company_employee_count":    true,
	"company_linkedin_url":      true,
	"company_total_funding_usd": true,
	"company_funding_stage":     true,
	"company_logo":              true,
	"company_country":           true,
	"hiring_manager":            true,
	"hiring_manager_linkedin":   true,
	"metadata":                  true,
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
