package planner

import (
	"fmt"
	"time"
)

// Memory represents a persistent key-value memory entry.
type Memory struct {
	ID        int64  `json:"id"`
	Key       string `json:"key"`
	Content   string `json:"content"`
	Tags      string `json:"tags"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

// UpsertMemory creates or updates a memory by key.
func (s *Store) UpsertMemory(key, content, tags string) (Memory, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := s.db.Exec(`
		INSERT INTO memories (key, content, tags, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(key) DO UPDATE SET content=excluded.content, tags=excluded.tags, updated_at=excluded.updated_at`,
		key, content, tags, now, now,
	)
	if err != nil {
		return Memory{}, fmt.Errorf("upsert memory: %w", err)
	}
	return s.GetMemory(key)
}

// GetMemory retrieves a single memory by key.
func (s *Store) GetMemory(key string) (Memory, error) {
	row := s.db.QueryRow("SELECT id, key, content, tags, created_at, updated_at FROM memories WHERE key = ?", key)
	var m Memory
	if err := row.Scan(&m.ID, &m.Key, &m.Content, &m.Tags, &m.CreatedAt, &m.UpdatedAt); err != nil {
		return Memory{}, fmt.Errorf("get memory: %w", err)
	}
	return m, nil
}

// SearchMemories finds memories matching a query across key, content, and tags.
func (s *Store) SearchMemories(query string) ([]Memory, error) {
	like := "%" + query + "%"
	rows, err := s.db.Query(
		"SELECT id, key, content, tags, created_at, updated_at FROM memories WHERE key LIKE ? OR content LIKE ? OR tags LIKE ? ORDER BY updated_at DESC",
		like, like, like,
	)
	if err != nil {
		return nil, fmt.Errorf("search memories: %w", err)
	}
	defer rows.Close()

	var memories []Memory
	for rows.Next() {
		var m Memory
		if err := rows.Scan(&m.ID, &m.Key, &m.Content, &m.Tags, &m.CreatedAt, &m.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan memory: %w", err)
		}
		memories = append(memories, m)
	}
	return memories, rows.Err()
}

// ListMemories returns the most recently updated memories, up to limit.
func (s *Store) ListMemories(limit int) ([]Memory, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := s.db.Query(
		"SELECT id, key, content, tags, created_at, updated_at FROM memories ORDER BY updated_at DESC LIMIT ?",
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf("list memories: %w", err)
	}
	defer rows.Close()

	var memories []Memory
	for rows.Next() {
		var m Memory
		if err := rows.Scan(&m.ID, &m.Key, &m.Content, &m.Tags, &m.CreatedAt, &m.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan memory: %w", err)
		}
		memories = append(memories, m)
	}
	return memories, rows.Err()
}

// DeleteMemory removes a memory by key.
func (s *Store) DeleteMemory(key string) error {
	res, err := s.db.Exec("DELETE FROM memories WHERE key = ?", key)
	if err != nil {
		return fmt.Errorf("delete memory: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("memory %q not found", key)
	}
	return nil
}
