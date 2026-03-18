package store

import (
	"database/sql"
	"errors"
	"fmt"
)

// AddSyncResult inserts a sync result and returns its ID.
func (s *Store) AddSyncResult(sr SyncResult) (int64, error) {
	if sr.CheckedAt == "" {
		sr.CheckedAt = now()
	}
	res, err := s.db.Exec(
		"INSERT INTO sync_results (platform, status, issues, details, checked_at) VALUES (?, ?, ?, ?, ?)",
		sr.Platform, sr.Status, sr.Issues, sr.Details, sr.CheckedAt,
	)
	if err != nil {
		return 0, fmt.Errorf("scout: add sync result: %w", err)
	}
	id, _ := res.LastInsertId()
	return id, nil
}

// GetSyncMeta retrieves a sync metadata value by key.
// Returns ("", nil) if the key does not exist.
func (s *Store) GetSyncMeta(key string) (string, error) {
	var val string
	err := s.db.QueryRow("SELECT value FROM sync_meta WHERE key = ?", key).Scan(&val)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("scout: get sync meta %q: %w", key, err)
	}
	return val, nil
}

// SetSyncMeta upserts a sync metadata key-value pair.
func (s *Store) SetSyncMeta(key, value string) error {
	_, err := s.db.Exec(
		"INSERT INTO sync_meta (key, value) VALUES (?, ?) ON CONFLICT(key) DO UPDATE SET value = ?",
		key, value, value,
	)
	if err != nil {
		return fmt.Errorf("scout: set sync meta: %w", err)
	}
	return nil
}
