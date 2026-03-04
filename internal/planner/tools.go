package planner

import (
	"fmt"
	"time"
)

// CustomTool represents a user-defined tool persisted in the database.
type CustomTool struct {
	ID              int64  `json:"id"`
	Name            string `json:"name"`
	Description     string `json:"description"`
	InputSchema     string `json:"input_schema"`
	CommandTemplate string `json:"command_template"`
	CreatedAt       string `json:"created_at"`
	UpdatedAt       string `json:"updated_at"`
}

// CreateCustomTool inserts a new custom tool.
func (s *Store) CreateCustomTool(name, description, inputSchema, commandTemplate string) (CustomTool, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	res, err := s.db.Exec(
		"INSERT INTO custom_tools (name, description, input_schema, command_template, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)",
		name, description, inputSchema, commandTemplate, now, now,
	)
	if err != nil {
		return CustomTool{}, fmt.Errorf("create custom tool: %w", err)
	}
	id, _ := res.LastInsertId()
	return CustomTool{
		ID:              id,
		Name:            name,
		Description:     description,
		InputSchema:     inputSchema,
		CommandTemplate: commandTemplate,
		CreatedAt:       now,
		UpdatedAt:       now,
	}, nil
}

// ListCustomTools returns all custom tools ordered by name.
func (s *Store) ListCustomTools() ([]CustomTool, error) {
	rows, err := s.db.Query(
		"SELECT id, name, description, input_schema, command_template, created_at, updated_at FROM custom_tools ORDER BY name ASC",
	)
	if err != nil {
		return nil, fmt.Errorf("list custom tools: %w", err)
	}
	defer rows.Close()

	var tools []CustomTool
	for rows.Next() {
		var t CustomTool
		if err := rows.Scan(&t.ID, &t.Name, &t.Description, &t.InputSchema, &t.CommandTemplate, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan custom tool: %w", err)
		}
		tools = append(tools, t)
	}
	return tools, rows.Err()
}

// GetCustomTool retrieves a single custom tool by name.
func (s *Store) GetCustomTool(name string) (CustomTool, error) {
	row := s.db.QueryRow(
		"SELECT id, name, description, input_schema, command_template, created_at, updated_at FROM custom_tools WHERE name = ?",
		name,
	)
	var t CustomTool
	if err := row.Scan(&t.ID, &t.Name, &t.Description, &t.InputSchema, &t.CommandTemplate, &t.CreatedAt, &t.UpdatedAt); err != nil {
		return CustomTool{}, fmt.Errorf("get custom tool: %w", err)
	}
	return t, nil
}

// DeleteCustomTool removes a custom tool by name.
func (s *Store) DeleteCustomTool(name string) error {
	res, err := s.db.Exec("DELETE FROM custom_tools WHERE name = ?", name)
	if err != nil {
		return fmt.Errorf("delete custom tool: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("custom tool %q not found", name)
	}
	return nil
}
