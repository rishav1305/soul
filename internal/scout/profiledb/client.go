package profiledb

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Client provides access to the profile PostgreSQL database.
type Client struct {
	pool *pgxpool.Pool
}

// New creates a new profiledb Client with a pgx connection pool.
// Returns an error if connString is empty or the connection fails.
func New(connString string) (*Client, error) {
	if connString == "" {
		return nil, fmt.Errorf("profiledb: connection string is empty")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, connString)
	if err != nil {
		return nil, fmt.Errorf("profiledb: connect: %w", err)
	}

	// Verify connectivity.
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("profiledb: ping: %w", err)
	}

	return &Client{pool: pool}, nil
}

// Close shuts down the connection pool.
func (c *Client) Close() {
	if c.pool != nil {
		c.pool.Close()
	}
}

// GetFullProfile queries all profile sections and returns them as a map.
// Sections: experience, projects, skill_categories, site_config, education, certifications.
func (c *Client) GetFullProfile() (map[string]interface{}, error) {
	profile := make(map[string]interface{})

	sections := []string{"experience", "projects", "skill_categories", "site_config", "education", "certifications"}
	for _, section := range sections {
		data, err := c.GetSection(section)
		if err != nil {
			// Non-fatal: include error note but continue.
			profile[section] = map[string]string{"error": err.Error()}
			continue
		}
		profile[section] = data
	}

	return profile, nil
}

// GetSection queries a specific profile table and returns the rows as a slice of maps.
func (c *Client) GetSection(name string) (interface{}, error) {
	// Validate table name to prevent SQL injection.
	allowed := map[string]bool{
		"experience":       true,
		"projects":         true,
		"skill_categories": true,
		"site_config":      true,
		"education":        true,
		"certifications":   true,
	}
	if !allowed[name] {
		return nil, fmt.Errorf("profiledb: unknown section: %q", name)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	rows, err := c.pool.Query(ctx, "SELECT * FROM "+name)
	if err != nil {
		return nil, fmt.Errorf("profiledb: query %s: %w", name, err)
	}
	defer rows.Close()

	fields := rows.FieldDescriptions()
	var results []map[string]interface{}

	for rows.Next() {
		values, err := rows.Values()
		if err != nil {
			return nil, fmt.Errorf("profiledb: scan %s: %w", name, err)
		}
		row := make(map[string]interface{})
		for i, fd := range fields {
			row[string(fd.Name)] = values[i]
		}
		results = append(results, row)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("profiledb: rows %s: %w", name, err)
	}

	return results, nil
}
