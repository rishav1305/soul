package session

import (
	"database/sql"
	"fmt"
	"strings"
	"time"
)

// CustomTool represents a user-defined tool with a command template.
type CustomTool struct {
	ID              int64
	Name            string
	Description     string
	InputSchema     string
	CommandTemplate string
	CreatedAt       string
	UpdatedAt       string
}

// CreateCustomTool inserts a new custom tool. Returns an error if the name already exists.
func (s *Store) CreateCustomTool(name, description, inputSchema, commandTemplate string) (CustomTool, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := s.db.Exec(
		`INSERT INTO custom_tools (name, description, input_schema, command_template, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		name, description, inputSchema, commandTemplate, now, now,
	)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint") {
			return CustomTool{}, fmt.Errorf("session: custom tool already exists: %s", name)
		}
		return CustomTool{}, fmt.Errorf("session: create custom tool: %w", err)
	}

	return s.GetCustomTool(name)
}

// ListCustomTools returns all custom tools ordered by name ascending.
func (s *Store) ListCustomTools() ([]CustomTool, error) {
	rows, err := s.db.Query(
		"SELECT id, name, description, input_schema, command_template, created_at, updated_at FROM custom_tools ORDER BY name ASC",
	)
	if err != nil {
		return nil, fmt.Errorf("session: list custom tools: %w", err)
	}
	defer rows.Close()

	var tools []CustomTool
	for rows.Next() {
		var t CustomTool
		if err := rows.Scan(&t.ID, &t.Name, &t.Description, &t.InputSchema, &t.CommandTemplate, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, fmt.Errorf("session: scan custom tool: %w", err)
		}
		tools = append(tools, t)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("session: iterate custom tools: %w", err)
	}
	return tools, nil
}

// GetCustomTool retrieves a custom tool by name. Returns an error if not found.
func (s *Store) GetCustomTool(name string) (CustomTool, error) {
	var t CustomTool
	err := s.db.QueryRow(
		"SELECT id, name, description, input_schema, command_template, created_at, updated_at FROM custom_tools WHERE name = ?",
		name,
	).Scan(&t.ID, &t.Name, &t.Description, &t.InputSchema, &t.CommandTemplate, &t.CreatedAt, &t.UpdatedAt)
	if err == sql.ErrNoRows {
		return CustomTool{}, fmt.Errorf("session: custom tool not found: %s", name)
	}
	if err != nil {
		return CustomTool{}, fmt.Errorf("session: get custom tool: %w", err)
	}
	return t, nil
}

// DeleteCustomTool removes a custom tool by name. Returns an error if not found.
func (s *Store) DeleteCustomTool(name string) error {
	result, err := s.db.Exec("DELETE FROM custom_tools WHERE name = ?", name)
	if err != nil {
		return fmt.Errorf("session: delete custom tool: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("session: rows affected: %w", err)
	}
	if affected == 0 {
		return fmt.Errorf("session: custom tool not found: %s", name)
	}
	return nil
}
