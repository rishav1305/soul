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

// SiteConfigRow represents the profile row from the site_config table.
type SiteConfigRow struct {
	ID         string            `json:"id"`
	Name       string            `json:"name"`
	Title      string            `json:"title"`
	Email      string            `json:"email"`
	ShortBio   string            `json:"short_bio"`
	LongBio    []string          `json:"long_bio"`
	Location   string            `json:"location"`
	YearsStart int               `json:"years_experience_start_year"`
	WhatsApp   string            `json:"whatsapp"`
	SocialMedia map[string]string `json:"social_media"`
}

// ExperienceRow represents a work-experience entry from the experience table.
type ExperienceRow struct {
	ID           string   `json:"id"`
	Role         string   `json:"role"`
	Company      string   `json:"company"`
	Period       string   `json:"period"`
	StartDate    string   `json:"start_date"`
	EndDate      *string  `json:"end_date"`
	Location     string   `json:"location"`
	Achievements []string `json:"achievements"`
	TechStack    []string `json:"tech_stack"`
}

// SkillEntry represents a single skill within a category.
type SkillEntry struct {
	Name  string  `json:"name"`
	Level float64 `json:"level"`
}

// SkillCategoryRow represents a skill category from the skill_categories table.
type SkillCategoryRow struct {
	ID           string       `json:"id"`
	CategoryName string       `json:"category_name"`
	Skills       []SkillEntry `json:"skills"`
	DisplayOrder int          `json:"display_order"`
}

// ProjectRow represents a portfolio project from the projects table.
type ProjectRow struct {
	ID               string   `json:"id"`
	Title            string   `json:"title"`
	Description      string   `json:"description"`
	ShortDescription string   `json:"short_description"`
	TechStack        []string `json:"tech_stack"`
	Category         string   `json:"category"`
	Company          string   `json:"company"`
}

// ---------------------------------------------------------------------------
// Aggregate profile data
// ---------------------------------------------------------------------------

// ProfileData holds the complete profile fetched from all four Supabase tables.
type ProfileData struct {
	SiteConfig []SiteConfigRow    `json:"site_config"`
	Experience []ExperienceRow    `json:"experience"`
	Skills     []SkillCategoryRow `json:"skills"`
	Projects   []ProjectRow       `json:"projects"`
}

// GetProfileData fetches all four profile tables from Supabase and returns
// the combined result. Experience ordered by start_date desc, skills by display_order.
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

	// --- experience (ordered by start_date descending — most recent first) ---
	raw, err = c.query("experience", "order=start_date.desc")
	if err != nil {
		return nil, fmt.Errorf("get experience: %w", err)
	}
	if err := json.Unmarshal(raw, &pd.Experience); err != nil {
		return nil, fmt.Errorf("decode experience: %w", err)
	}

	// --- skill_categories (ordered by display_order) ---
	raw, err = c.query("skill_categories", "order=display_order")
	if err != nil {
		return nil, fmt.Errorf("get skill_categories: %w", err)
	}
	if err := json.Unmarshal(raw, &pd.Skills); err != nil {
		return nil, fmt.Errorf("decode skill_categories: %w", err)
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
