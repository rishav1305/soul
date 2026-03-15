package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

// Challenge represents a CTF challenge.
type Challenge struct {
	ID           string   `json:"id"`
	Category     string   `json:"category"`
	Difficulty   string   `json:"difficulty"`
	Title        string   `json:"title"`
	Description  string   `json:"description"`
	Objective    string   `json:"objective"`
	Flag         string   `json:"flag"`
	SystemPrompt string   `json:"system_prompt"`
	LearnMore    string   `json:"learn_more"`
	Tools        []string `json:"tools"`
	Hints        []string `json:"hints"`
	Points       int      `json:"points"`
	MaxTurns     int      `json:"max_turns"`
	Phase        int      `json:"phase"`
}

// Attempt represents a single attempt at a challenge.
type Attempt struct {
	ID          int64  `json:"id"`
	ChallengeID string `json:"challengeId"`
	Payload     string `json:"payload"`
	Response    string `json:"response"`
	Success     bool   `json:"success"`
	Timestamp   string `json:"timestamp"`
}

// Completion represents a completed challenge.
type Completion struct {
	ID           int64  `json:"id"`
	ChallengeID  string `json:"challengeId"`
	PointsEarned int    `json:"pointsEarned"`
	TurnsUsed    int    `json:"turnsUsed"`
	HintsUsed    int    `json:"hintsUsed"`
	CompletedAt  string `json:"completedAt"`
}

// Guardrail represents a guardrail rule.
type Guardrail struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	RuleJSON  string `json:"ruleJson"`
	CreatedAt string `json:"createdAt"`
}

// ScanResult represents a security scan result.
type ScanResult struct {
	ID              int64  `json:"id"`
	ProductName     string `json:"productName"`
	FindingsJSON    string `json:"findingsJson"`
	SeveritySummary string `json:"severitySummary"`
	ScannedAt       string `json:"scannedAt"`
}

// SandboxConfig represents a sandbox configuration.
type SandboxConfig struct {
	ID             int64  `json:"id"`
	Name           string `json:"name"`
	SystemPrompt   string `json:"systemPrompt"`
	GuardrailsJSON string `json:"guardrailsJson"`
	WeaknessLevel  string `json:"weaknessLevel"`
}

// Store provides SQLite-backed sentinel CRUD.
type Store struct {
	db     *sql.DB
	dbPath string
}

// Open creates a new Store with the given database path.
func Open(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("sentinel: open database: %w", err)
	}

	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		db.Close()
		return nil, fmt.Errorf("sentinel: enable WAL: %w", err)
	}
	if _, err := db.Exec("PRAGMA foreign_keys=ON"); err != nil {
		db.Close()
		return nil, fmt.Errorf("sentinel: enable foreign keys: %w", err)
	}
	db.SetMaxOpenConns(1)
	if _, err := db.Exec("PRAGMA busy_timeout=5000"); err != nil {
		db.Close()
		return nil, fmt.Errorf("sentinel: set busy timeout: %w", err)
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
	CREATE TABLE IF NOT EXISTS challenges (
		id TEXT PRIMARY KEY,
		category TEXT NOT NULL,
		difficulty TEXT NOT NULL,
		title TEXT NOT NULL,
		description TEXT NOT NULL,
		objective TEXT NOT NULL,
		flag TEXT NOT NULL,
		system_prompt TEXT NOT NULL,
		tools_json TEXT NOT NULL DEFAULT '[]',
		hints_json TEXT NOT NULL DEFAULT '[]',
		learn_more TEXT NOT NULL DEFAULT '',
		points INTEGER NOT NULL DEFAULT 0,
		max_turns INTEGER NOT NULL DEFAULT 10,
		phase INTEGER NOT NULL DEFAULT 1
	);
	CREATE TABLE IF NOT EXISTS attempts (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		challenge_id TEXT NOT NULL REFERENCES challenges(id),
		payload TEXT NOT NULL,
		response TEXT NOT NULL,
		success BOOLEAN NOT NULL DEFAULT 0,
		timestamp TEXT NOT NULL
	);
	CREATE TABLE IF NOT EXISTS completions (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		challenge_id TEXT NOT NULL UNIQUE REFERENCES challenges(id),
		points_earned INTEGER NOT NULL,
		turns_used INTEGER NOT NULL,
		hints_used INTEGER NOT NULL DEFAULT 0,
		completed_at TEXT NOT NULL
	);
	CREATE TABLE IF NOT EXISTS guardrails (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL,
		rule_json TEXT NOT NULL,
		created_at TEXT NOT NULL
	);
	CREATE TABLE IF NOT EXISTS scan_results (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		product_name TEXT NOT NULL,
		findings_json TEXT NOT NULL,
		severity_summary TEXT NOT NULL,
		scanned_at TEXT NOT NULL
	);
	CREATE TABLE IF NOT EXISTS sandbox_configs (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL,
		system_prompt TEXT NOT NULL,
		guardrails_json TEXT NOT NULL DEFAULT '[]',
		weakness_level TEXT NOT NULL DEFAULT 'none'
	);
	CREATE INDEX IF NOT EXISTS idx_attempts_challenge_id ON attempts(challenge_id);
	CREATE INDEX IF NOT EXISTS idx_scan_results_product ON scan_results(product_name);
	`
	if _, err := s.db.Exec(schema); err != nil {
		return fmt.Errorf("sentinel: migrate: %w", err)
	}
	return nil
}

// rawChallenge is used for JSON unmarshalling of challenge data with flexible tool types.
type rawChallenge struct {
	ID           string            `json:"id"`
	Category     string            `json:"category"`
	Difficulty   string            `json:"difficulty"`
	Title        string            `json:"title"`
	Description  string            `json:"description"`
	Objective    string            `json:"objective"`
	Flag         string            `json:"flag"`
	SystemPrompt string            `json:"system_prompt"`
	LearnMore    string            `json:"learn_more"`
	Tools        json.RawMessage   `json:"tools"`
	Hints        []string          `json:"hints"`
	Points       int               `json:"points"`
	MaxTurns     int               `json:"max_turns"`
	Phase        int               `json:"phase"`
}

// SeedChallenges loads challenges from JSON data into the store.
func (s *Store) SeedChallenges(jsonData []byte) error {
	var raw []rawChallenge
	if err := json.Unmarshal(jsonData, &raw); err != nil {
		return fmt.Errorf("sentinel: unmarshal challenges: %w", err)
	}

	for _, c := range raw {
		// Convert tools to []string — tools can be strings or objects.
		var toolsStrings []string
		if len(c.Tools) > 0 && string(c.Tools) != "[]" && string(c.Tools) != "null" {
			// Try as []string first.
			if err := json.Unmarshal(c.Tools, &toolsStrings); err != nil {
				// Try as []map[string]interface{} and convert to JSON strings.
				toolsStrings = nil
				var toolObjs []map[string]interface{}
				if err2 := json.Unmarshal(c.Tools, &toolObjs); err2 != nil {
					return fmt.Errorf("sentinel: parse tools for %s: %w", c.ID, err)
				}
				for _, obj := range toolObjs {
					b, _ := json.Marshal(obj)
					toolsStrings = append(toolsStrings, string(b))
				}
			}
		}

		toolsJSON, _ := json.Marshal(toolsStrings)
		hintsJSON, _ := json.Marshal(c.Hints)

		_, err := s.db.Exec(
			`INSERT OR REPLACE INTO challenges (id, category, difficulty, title, description, objective, flag, system_prompt, tools_json, hints_json, learn_more, points, max_turns, phase)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			c.ID, c.Category, c.Difficulty, c.Title, c.Description, c.Objective, c.Flag, c.SystemPrompt,
			string(toolsJSON), string(hintsJSON), c.LearnMore, c.Points, c.MaxTurns, c.Phase,
		)
		if err != nil {
			return fmt.Errorf("sentinel: seed challenge %s: %w", c.ID, err)
		}
	}
	return nil
}

func scanChallenge(row interface{ Scan(dest ...interface{}) error }) (*Challenge, error) {
	var c Challenge
	var toolsJSON, hintsJSON string
	err := row.Scan(&c.ID, &c.Category, &c.Difficulty, &c.Title, &c.Description, &c.Objective,
		&c.Flag, &c.SystemPrompt, &toolsJSON, &hintsJSON, &c.LearnMore, &c.Points, &c.MaxTurns, &c.Phase)
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal([]byte(toolsJSON), &c.Tools); err != nil {
		c.Tools = []string{}
	}
	if err := json.Unmarshal([]byte(hintsJSON), &c.Hints); err != nil {
		c.Hints = []string{}
	}
	return &c, nil
}

// GetChallenge retrieves a challenge by ID.
func (s *Store) GetChallenge(id string) (*Challenge, error) {
	row := s.db.QueryRow(
		`SELECT id, category, difficulty, title, description, objective, flag, system_prompt, tools_json, hints_json, learn_more, points, max_turns, phase
		 FROM challenges WHERE id = ?`, id,
	)
	c, err := scanChallenge(row)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("sentinel: challenge not found: %s", id)
	}
	if err != nil {
		return nil, fmt.Errorf("sentinel: get challenge: %w", err)
	}
	return c, nil
}

// ListChallenges returns challenges, optionally filtered by category, difficulty, and phase.
func (s *Store) ListChallenges(category, difficulty string, phase int) ([]Challenge, error) {
	query := `SELECT id, category, difficulty, title, description, objective, flag, system_prompt, tools_json, hints_json, learn_more, points, max_turns, phase FROM challenges`
	var conditions []string
	var args []interface{}

	if category != "" {
		conditions = append(conditions, "category = ?")
		args = append(args, category)
	}
	if difficulty != "" {
		conditions = append(conditions, "difficulty = ?")
		args = append(args, difficulty)
	}
	if phase > 0 {
		conditions = append(conditions, "phase = ?")
		args = append(args, phase)
	}
	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}
	query += " ORDER BY points ASC, id ASC"

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("sentinel: list challenges: %w", err)
	}
	defer rows.Close()

	var challenges []Challenge
	for rows.Next() {
		c, err := scanChallenge(rows)
		if err != nil {
			return nil, fmt.Errorf("sentinel: scan challenge: %w", err)
		}
		challenges = append(challenges, *c)
	}
	return challenges, rows.Err()
}

// RecordAttempt records an attempt at a challenge.
func (s *Store) RecordAttempt(challengeID, payload, response string, success bool) (int64, error) {
	ts := time.Now().UTC().Format(time.RFC3339)
	res, err := s.db.Exec(
		"INSERT INTO attempts (challenge_id, payload, response, success, timestamp) VALUES (?, ?, ?, ?, ?)",
		challengeID, payload, response, success, ts,
	)
	if err != nil {
		return 0, fmt.Errorf("sentinel: record attempt: %w", err)
	}
	id, _ := res.LastInsertId()
	return id, nil
}

// CountAttempts returns the number of attempts for a challenge.
func (s *Store) CountAttempts(challengeID string) (int, error) {
	var count int
	err := s.db.QueryRow("SELECT COUNT(*) FROM attempts WHERE challenge_id = ?", challengeID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("sentinel: count attempts: %w", err)
	}
	return count, nil
}

// GetAttempts returns all attempts for a challenge, ordered by timestamp ASC.
func (s *Store) GetAttempts(challengeID string) ([]Attempt, error) {
	rows, err := s.db.Query(
		"SELECT id, challenge_id, payload, response, success, timestamp FROM attempts WHERE challenge_id = ? ORDER BY timestamp ASC",
		challengeID,
	)
	if err != nil {
		return nil, fmt.Errorf("sentinel: get attempts: %w", err)
	}
	defer rows.Close()

	var attempts []Attempt
	for rows.Next() {
		var a Attempt
		if err := rows.Scan(&a.ID, &a.ChallengeID, &a.Payload, &a.Response, &a.Success, &a.Timestamp); err != nil {
			return nil, fmt.Errorf("sentinel: scan attempt: %w", err)
		}
		attempts = append(attempts, a)
	}
	return attempts, rows.Err()
}

// RecordCompletion records a challenge completion. Idempotent — ignores duplicate challenge_id.
func (s *Store) RecordCompletion(challengeID string, points, turns, hints int) error {
	ts := time.Now().UTC().Format(time.RFC3339)
	_, err := s.db.Exec(
		"INSERT OR IGNORE INTO completions (challenge_id, points_earned, turns_used, hints_used, completed_at) VALUES (?, ?, ?, ?, ?)",
		challengeID, points, turns, hints, ts,
	)
	if err != nil {
		return fmt.Errorf("sentinel: record completion: %w", err)
	}
	return nil
}

// IsCompleted returns whether a challenge has been completed.
func (s *Store) IsCompleted(challengeID string) (bool, error) {
	var count int
	err := s.db.QueryRow("SELECT COUNT(*) FROM completions WHERE challenge_id = ?", challengeID).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("sentinel: is completed: %w", err)
	}
	return count > 0, nil
}

// GetProgress returns a map of challenge_id -> points_earned for all completions.
func (s *Store) GetProgress() (map[string]int, error) {
	rows, err := s.db.Query("SELECT challenge_id, points_earned FROM completions")
	if err != nil {
		return nil, fmt.Errorf("sentinel: get progress: %w", err)
	}
	defer rows.Close()

	progress := make(map[string]int)
	for rows.Next() {
		var id string
		var points int
		if err := rows.Scan(&id, &points); err != nil {
			return nil, fmt.Errorf("sentinel: scan progress: %w", err)
		}
		progress[id] = points
	}
	return progress, rows.Err()
}

// SaveGuardrail saves a guardrail rule.
func (s *Store) SaveGuardrail(name, ruleJSON string) (int64, error) {
	ts := time.Now().UTC().Format(time.RFC3339)
	res, err := s.db.Exec(
		"INSERT INTO guardrails (name, rule_json, created_at) VALUES (?, ?, ?)",
		name, ruleJSON, ts,
	)
	if err != nil {
		return 0, fmt.Errorf("sentinel: save guardrail: %w", err)
	}
	id, _ := res.LastInsertId()
	return id, nil
}

// ListGuardrails returns all guardrails, ordered by created_at DESC.
func (s *Store) ListGuardrails() ([]Guardrail, error) {
	rows, err := s.db.Query("SELECT id, name, rule_json, created_at FROM guardrails ORDER BY id DESC")
	if err != nil {
		return nil, fmt.Errorf("sentinel: list guardrails: %w", err)
	}
	defer rows.Close()

	var guardrails []Guardrail
	for rows.Next() {
		var g Guardrail
		if err := rows.Scan(&g.ID, &g.Name, &g.RuleJSON, &g.CreatedAt); err != nil {
			return nil, fmt.Errorf("sentinel: scan guardrail: %w", err)
		}
		guardrails = append(guardrails, g)
	}
	return guardrails, rows.Err()
}

// SaveScanResult saves a security scan result.
func (s *Store) SaveScanResult(product, findingsJSON, severity string) (int64, error) {
	ts := time.Now().UTC().Format(time.RFC3339)
	res, err := s.db.Exec(
		"INSERT INTO scan_results (product_name, findings_json, severity_summary, scanned_at) VALUES (?, ?, ?, ?)",
		product, findingsJSON, severity, ts,
	)
	if err != nil {
		return 0, fmt.Errorf("sentinel: save scan result: %w", err)
	}
	id, _ := res.LastInsertId()
	return id, nil
}

// ListScanResults returns scan results for a product, ordered by scanned_at DESC, limited.
func (s *Store) ListScanResults(product string, limit int) ([]ScanResult, error) {
	if limit <= 0 {
		limit = 10
	}
	rows, err := s.db.Query(
		"SELECT id, product_name, findings_json, severity_summary, scanned_at FROM scan_results WHERE product_name = ? ORDER BY scanned_at DESC LIMIT ?",
		product, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("sentinel: list scan results: %w", err)
	}
	defer rows.Close()

	var results []ScanResult
	for rows.Next() {
		var r ScanResult
		if err := rows.Scan(&r.ID, &r.ProductName, &r.FindingsJSON, &r.SeveritySummary, &r.ScannedAt); err != nil {
			return nil, fmt.Errorf("sentinel: scan result: %w", err)
		}
		results = append(results, r)
	}
	return results, rows.Err()
}

// SaveSandboxConfig saves a sandbox configuration.
func (s *Store) SaveSandboxConfig(name, systemPrompt, guardrailsJSON, weakness string) (int64, error) {
	res, err := s.db.Exec(
		"INSERT INTO sandbox_configs (name, system_prompt, guardrails_json, weakness_level) VALUES (?, ?, ?, ?)",
		name, systemPrompt, guardrailsJSON, weakness,
	)
	if err != nil {
		return 0, fmt.Errorf("sentinel: save sandbox config: %w", err)
	}
	id, _ := res.LastInsertId()
	return id, nil
}

// GetDefaultSandboxConfig returns the most recently created sandbox config, or nil if none exist.
func (s *Store) GetDefaultSandboxConfig() (*SandboxConfig, error) {
	var cfg SandboxConfig
	err := s.db.QueryRow(
		"SELECT id, name, system_prompt, guardrails_json, weakness_level FROM sandbox_configs ORDER BY id DESC LIMIT 1",
	).Scan(&cfg.ID, &cfg.Name, &cfg.SystemPrompt, &cfg.GuardrailsJSON, &cfg.WeaknessLevel)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("sentinel: get default sandbox config: %w", err)
	}
	return &cfg, nil
}
