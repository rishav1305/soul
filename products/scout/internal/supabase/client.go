// Package supabase provides a read-only REST client for fetching
// portfolio profile data from Supabase (the source of truth for
// rishavchatterjee.com).
package supabase

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

// ---------------------------------------------------------------------------
// Configuration
// ---------------------------------------------------------------------------

// Config holds the Supabase connection parameters read from
// ~/.soul/scout/config.json.
type Config struct {
	URL     string `json:"supabase_url"`
	AnonKey string `json:"supabase_anon_key"`
}

// configPath returns the absolute path to the scout config file.
func configPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home dir: %w", err)
	}
	return filepath.Join(home, ".soul", "scout", "config.json"), nil
}

// loadConfig reads and parses the scout config file.
func loadConfig() (*Config, error) {
	p, err := configPath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(p)
	if err != nil {
		return nil, fmt.Errorf("read config %s: %w", p, err)
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config %s: %w", p, err)
	}

	if cfg.URL == "" {
		return nil, fmt.Errorf("config %s: supabase_url is required", p)
	}
	if cfg.AnonKey == "" {
		return nil, fmt.Errorf("config %s: supabase_anon_key is required", p)
	}

	return &cfg, nil
}

// ---------------------------------------------------------------------------
// Client
// ---------------------------------------------------------------------------

// Client is a read-only REST client for the Supabase PostgREST API.
type Client struct {
	url     string
	anonKey string
	http    *http.Client
}

// NewClient reads ~/.soul/scout/config.json and returns a configured Client.
func NewClient() (*Client, error) {
	cfg, err := loadConfig()
	if err != nil {
		return nil, fmt.Errorf("supabase: %w", err)
	}

	return &Client{
		url:     cfg.URL,
		anonKey: cfg.AnonKey,
		http: &http.Client{
			Timeout: 15 * time.Second,
		},
	}, nil
}

// query performs a GET request against the Supabase PostgREST endpoint for the
// given table with an optional query-string filter (e.g. "order=display_order").
// It returns the raw JSON response body.
func (c *Client) query(table, filter string) ([]byte, error) {
	endpoint := fmt.Sprintf("%s/rest/v1/%s", c.url, table)
	if filter != "" {
		endpoint += "?" + filter
	}

	req, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("build request for %s: %w", table, err)
	}

	req.Header.Set("apikey", c.anonKey)
	req.Header.Set("Authorization", "Bearer "+c.anonKey)
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch %s: %w", table, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read %s response: %w", table, err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("supabase %s returned %d: %s", table, resp.StatusCode, body)
	}

	return body, nil
}

// ---------------------------------------------------------------------------
// Row types (match Supabase table schemas)
// ---------------------------------------------------------------------------

// SiteConfigRow represents a single key-value pair from the site_config table.
type SiteConfigRow struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// ExperienceRow represents a work-experience entry from the experience table.
type ExperienceRow struct {
	ID           string   `json:"id"`
	Role         string   `json:"role"`
	Company      string   `json:"company"`
	Period       string   `json:"period"`
	Achievements []string `json:"achievements"`
	Order        int      `json:"display_order"`
}

// SkillRow represents a single skill entry from the skills table.
type SkillRow struct {
	ID       string `json:"id"`
	Category string `json:"category"`
	Name     string `json:"name"`
	Level    string `json:"level"`
}

// ProjectRow represents a portfolio project from the projects table.
type ProjectRow struct {
	ID          string   `json:"id"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	TechStack   []string `json:"tech_stack"`
}

// ---------------------------------------------------------------------------
// Aggregate profile data
// ---------------------------------------------------------------------------

// ProfileData holds the complete profile fetched from all four Supabase tables.
type ProfileData struct {
	SiteConfig []SiteConfigRow `json:"site_config"`
	Experience []ExperienceRow `json:"experience"`
	Skills     []SkillRow      `json:"skills"`
	Projects   []ProjectRow    `json:"projects"`
}

// GetProfileData fetches all four profile tables from Supabase and returns
// the combined result. Experience rows are ordered by display_order.
func (c *Client) GetProfileData() (*ProfileData, error) {
	var pd ProfileData

	// --- site_config ---
	raw, err := c.query("site_config", "")
	if err != nil {
		return nil, fmt.Errorf("get site_config: %w", err)
	}
	if err := json.Unmarshal(raw, &pd.SiteConfig); err != nil {
		return nil, fmt.Errorf("decode site_config: %w", err)
	}

	// --- experience (ordered) ---
	raw, err = c.query("experience", "order=display_order")
	if err != nil {
		return nil, fmt.Errorf("get experience: %w", err)
	}
	if err := json.Unmarshal(raw, &pd.Experience); err != nil {
		return nil, fmt.Errorf("decode experience: %w", err)
	}

	// --- skills ---
	raw, err = c.query("skills", "")
	if err != nil {
		return nil, fmt.Errorf("get skills: %w", err)
	}
	if err := json.Unmarshal(raw, &pd.Skills); err != nil {
		return nil, fmt.Errorf("decode skills: %w", err)
	}

	// --- projects ---
	raw, err = c.query("projects", "")
	if err != nil {
		return nil, fmt.Errorf("get projects: %w", err)
	}
	if err := json.Unmarshal(raw, &pd.Projects); err != nil {
		return nil, fmt.Errorf("decode projects: %w", err)
	}

	return &pd, nil
}
