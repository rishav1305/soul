package session

import (
	"database/sql"
	"fmt"
	"time"
)

// Memory represents an agent memory entry keyed by a unique string.
type Memory struct {
	ID        int64
	Key       string
	Content   string
	Tags      string
	CreatedAt string
	UpdatedAt string
}

// UpsertMemory creates or updates a memory by key. If the key already exists,
// content and tags are updated and updated_at is refreshed.
func (s *Store) UpsertMemory(key, content, tags string) (Memory, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := s.db.Exec(
		`INSERT INTO memories (key, content, tags, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?)
		 ON CONFLICT(key) DO UPDATE SET content = excluded.content, tags = excluded.tags, updated_at = excluded.updated_at`,
		key, content, tags, now, now,
	)
	if err != nil {
		return Memory{}, fmt.Errorf("session: upsert memory: %w", err)
	}

	return s.GetMemory(key)
}

// GetMemory retrieves a memory by key. Returns an error if not found.
func (s *Store) GetMemory(key string) (Memory, error) {
	var m Memory
	err := s.db.QueryRow(
		"SELECT id, key, content, tags, created_at, updated_at FROM memories WHERE key = ?",
		key,
	).Scan(&m.ID, &m.Key, &m.Content, &m.Tags, &m.CreatedAt, &m.UpdatedAt)
	if err == sql.ErrNoRows {
		return Memory{}, fmt.Errorf("session: memory not found: %s", key)
	}
	if err != nil {
		return Memory{}, fmt.Errorf("session: get memory: %w", err)
	}
	return m, nil
}

// SearchMemories searches memories by matching query against key, content, and tags
// using LIKE. Results are ordered by updated_at descending.
func (s *Store) SearchMemories(query string) ([]Memory, error) {
	pattern := "%" + query + "%"
	rows, err := s.db.Query(
		`SELECT id, key, content, tags, created_at, updated_at FROM memories
		 WHERE key LIKE ? OR content LIKE ? OR tags LIKE ?
		 ORDER BY updated_at DESC`,
		pattern, pattern, pattern,
	)
	if err != nil {
		return nil, fmt.Errorf("session: search memories: %w", err)
	}
	defer rows.Close()

	var memories []Memory
	for rows.Next() {
		var m Memory
		if err := rows.Scan(&m.ID, &m.Key, &m.Content, &m.Tags, &m.CreatedAt, &m.UpdatedAt); err != nil {
			return nil, fmt.Errorf("session: scan memory: %w", err)
		}
		memories = append(memories, m)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("session: iterate memories: %w", err)
	}
	return memories, nil
}

// ListMemories returns the most recently updated memories, up to limit.
func (s *Store) ListMemories(limit int) ([]Memory, error) {
	rows, err := s.db.Query(
		"SELECT id, key, content, tags, created_at, updated_at FROM memories ORDER BY updated_at DESC LIMIT ?",
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf("session: list memories: %w", err)
	}
	defer rows.Close()

	var memories []Memory
	for rows.Next() {
		var m Memory
		if err := rows.Scan(&m.ID, &m.Key, &m.Content, &m.Tags, &m.CreatedAt, &m.UpdatedAt); err != nil {
			return nil, fmt.Errorf("session: scan memory: %w", err)
		}
		memories = append(memories, m)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("session: iterate memories: %w", err)
	}
	return memories, nil
}

// DeleteMemory removes a memory by key. Returns an error if the key is not found.
func (s *Store) DeleteMemory(key string) error {
	result, err := s.db.Exec("DELETE FROM memories WHERE key = ?", key)
	if err != nil {
		return fmt.Errorf("session: delete memory: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("session: rows affected: %w", err)
	}
	if affected == 0 {
		return fmt.Errorf("session: memory not found: %s", key)
	}
	return nil
}
