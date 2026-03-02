// Package profiledb provides access to the local PostgreSQL profile database.
// It serves as the primary source of truth for all professional profile data,
// with sync capabilities to/from Supabase.
package profiledb

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// AllTables lists every table in the profile database, in display order.
var AllTables = []string{
	"site_config",
	"experience",
	"skill_categories",
	"projects",
	"education",
	"testimonials",
	"brands",
	"services",
	"case_studies",
	"chat_questions",
	"faqs",
	"stats_dashboard",
	"skill_radar_data",
}

// tableOrder maps table names to ORDER BY clauses for deterministic output.
var tableOrder = map[string]string{
	"experience":       "start_date DESC",
	"skill_categories": "display_order",
	"testimonials":     "display_order",
	"brands":           "display_order",
	"services":         "display_order",
	"case_studies":     "display_order",
	"chat_questions":   "display_order",
	"faqs":             "display_order",
	"stats_dashboard":  "display_order",
	"skill_radar_data": "display_order",
}

// validTable returns true if the table name is in AllTables.
func validTable(name string) bool {
	for _, t := range AllTables {
		if t == name {
			return true
		}
	}
	return false
}

// Config holds connection settings for the profile database.
type Config struct {
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Database string `json:"database"`
	User     string `json:"user"`
	Password string `json:"password"`
}

// LoadConfig reads profile_db settings from ~/.soul/scout/config.json.
func LoadConfig() (*Config, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("resolve home dir: %w", err)
	}
	path := filepath.Join(home, ".soul", "scout", "config.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}
	var wrapper struct {
		ProfileDB *Config `json:"profile_db"`
	}
	if err := json.Unmarshal(data, &wrapper); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	if wrapper.ProfileDB == nil {
		return nil, fmt.Errorf("profile_db section missing from %s", path)
	}
	return wrapper.ProfileDB, nil
}

// Client provides access to the local profile database.
type Client struct {
	pool *pgxpool.Pool
}

// New creates a new Client connected to the profile database.
func New(ctx context.Context, cfg Config) (*Client, error) {
	dsn := fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=disable",
		cfg.User, cfg.Password, cfg.Host, cfg.Port, cfg.Database)
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, fmt.Errorf("connect to profile db: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping profile db: %w", err)
	}
	return &Client{pool: pool}, nil
}

// Close releases the connection pool.
func (c *Client) Close() {
	if c.pool != nil {
		c.pool.Close()
	}
}

// GetFullProfile fetches all 13 tables and returns them as a JSON-friendly map.
// Each key is a table name, each value is the JSON array of rows.
func (c *Client) GetFullProfile(ctx context.Context) (map[string]json.RawMessage, error) {
	result := make(map[string]json.RawMessage, len(AllTables))
	for _, table := range AllTables {
		raw, err := c.queryTableJSON(ctx, table)
		if err != nil {
			return nil, fmt.Errorf("query %s: %w", table, err)
		}
		result[table] = raw
	}
	return result, nil
}

// GetSections fetches only the specified tables.
func (c *Client) GetSections(ctx context.Context, sections []string) (map[string]json.RawMessage, error) {
	result := make(map[string]json.RawMessage, len(sections))
	for _, table := range sections {
		if !validTable(table) {
			continue
		}
		raw, err := c.queryTableJSON(ctx, table)
		if err != nil {
			return nil, fmt.Errorf("query %s: %w", table, err)
		}
		result[table] = raw
	}
	return result, nil
}

// queryTableJSON fetches all rows from a table as a JSON array using row_to_json.
func (c *Client) queryTableJSON(ctx context.Context, table string) (json.RawMessage, error) {
	if !validTable(table) {
		return nil, fmt.Errorf("invalid table: %s", table)
	}

	q := fmt.Sprintf("SELECT row_to_json(t) FROM %s t", table)
	if order, ok := tableOrder[table]; ok {
		q += " ORDER BY " + order
	}

	rows, err := c.pool.Query(ctx, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []json.RawMessage
	for rows.Next() {
		var raw json.RawMessage
		if err := rows.Scan(&raw); err != nil {
			return nil, err
		}
		items = append(items, raw)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	if items == nil {
		items = []json.RawMessage{}
	}
	return json.Marshal(items)
}

// ---------------------------------------------------------------------------
// Supabase Pull: Cloud → Local PG
// ---------------------------------------------------------------------------

// PullTable fetches all rows from a Supabase table and replaces the local table.
// Returns the number of rows pulled.
func (c *Client) PullTable(ctx context.Context, supabaseURL, anonKey, table string) (int, error) {
	if !validTable(table) {
		return 0, fmt.Errorf("invalid table: %s", table)
	}

	// Fetch from Supabase REST API.
	jsonRows, err := fetchSupabaseTable(supabaseURL, anonKey, table)
	if err != nil {
		return 0, fmt.Errorf("fetch %s from supabase: %w", table, err)
	}

	// Parse into array of JSON objects.
	var rows []json.RawMessage
	if err := json.Unmarshal(jsonRows, &rows); err != nil {
		return 0, fmt.Errorf("parse %s response: %w", table, err)
	}

	if len(rows) == 0 {
		return 0, nil
	}

	// Replace local table data in a transaction.
	tx, err := c.pool.Begin(ctx)
	if err != nil {
		return 0, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	// Truncate existing data.
	if _, err := tx.Exec(ctx, fmt.Sprintf("TRUNCATE %s CASCADE", table)); err != nil {
		return 0, fmt.Errorf("truncate %s: %w", table, err)
	}

	// Insert each row using json_populate_record.
	for _, row := range rows {
		q := fmt.Sprintf("INSERT INTO %s SELECT * FROM json_populate_record(NULL::%s, $1::json)", table, table)
		if _, err := tx.Exec(ctx, q, string(row)); err != nil {
			return 0, fmt.Errorf("insert into %s: %w", table, err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return 0, fmt.Errorf("commit %s: %w", table, err)
	}

	return len(rows), nil
}

// PullAll fetches all tables from Supabase and replaces local data.
// Returns a map of table name → row count.
func (c *Client) PullAll(ctx context.Context, supabaseURL, anonKey string) (map[string]int, error) {
	counts := make(map[string]int, len(AllTables))
	for _, table := range AllTables {
		n, err := c.PullTable(ctx, supabaseURL, anonKey, table)
		if err != nil {
			return counts, fmt.Errorf("pull %s: %w", table, err)
		}
		counts[table] = n
	}
	return counts, nil
}

// ---------------------------------------------------------------------------
// Supabase Push: Local PG → Cloud
// ---------------------------------------------------------------------------

// PushTable reads all rows from a local table and upserts them to Supabase.
// Returns the number of rows pushed.
func (c *Client) PushTable(ctx context.Context, supabaseURL, anonKey, table string) (int, error) {
	if !validTable(table) {
		return 0, fmt.Errorf("invalid table: %s", table)
	}

	// Read local data as JSON array.
	raw, err := c.queryTableJSON(ctx, table)
	if err != nil {
		return 0, fmt.Errorf("read local %s: %w", table, err)
	}

	var rows []json.RawMessage
	if err := json.Unmarshal(raw, &rows); err != nil {
		return 0, fmt.Errorf("parse local %s: %w", table, err)
	}

	if len(rows) == 0 {
		return 0, nil
	}

	// Push to Supabase via POST with upsert preference.
	if err := pushSupabaseTable(supabaseURL, anonKey, table, raw); err != nil {
		return 0, fmt.Errorf("push %s to supabase: %w", table, err)
	}

	return len(rows), nil
}

// PushAll pushes all tables from local PG to Supabase.
// Returns a map of table name → row count.
func (c *Client) PushAll(ctx context.Context, supabaseURL, anonKey string) (map[string]int, error) {
	counts := make(map[string]int, len(AllTables))
	for _, table := range AllTables {
		n, err := c.PushTable(ctx, supabaseURL, anonKey, table)
		if err != nil {
			return counts, fmt.Errorf("push %s: %w", table, err)
		}
		counts[table] = n
	}
	return counts, nil
}

// ---------------------------------------------------------------------------
// Supabase HTTP helpers
// ---------------------------------------------------------------------------

func fetchSupabaseTable(baseURL, anonKey, table string) ([]byte, error) {
	endpoint := fmt.Sprintf("%s/rest/v1/%s?select=*", baseURL, table)

	// Add ordering for tables that have it.
	if order, ok := tableOrder[table]; ok {
		endpoint += "&order=" + strings.ReplaceAll(order, " ", ".")
	}

	req, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("apikey", anonKey)
	req.Header.Set("Authorization", "Bearer "+anonKey)
	req.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("supabase %s returned %d: %s", table, resp.StatusCode, body)
	}

	return body, nil
}

func pushSupabaseTable(baseURL, anonKey, table string, jsonArray json.RawMessage) error {
	endpoint := fmt.Sprintf("%s/rest/v1/%s", baseURL, table)

	req, err := http.NewRequest(http.MethodPost, endpoint, strings.NewReader(string(jsonArray)))
	if err != nil {
		return err
	}
	req.Header.Set("apikey", anonKey)
	req.Header.Set("Authorization", "Bearer "+anonKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Prefer", "resolution=merge-duplicates")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("supabase %s returned %d: %s", table, resp.StatusCode, body)
	}

	return nil
}

