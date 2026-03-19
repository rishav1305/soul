package store

import (
	"database/sql"
	"fmt"
)

// LeadArtifact represents a generated artifact (cover letter, proposal, etc.) for a lead.
type LeadArtifact struct {
	ID        int64  `json:"id"`
	LeadID    int64  `json:"leadId"`
	Type      string `json:"type"`
	Content   string `json:"content"`
	CreatedAt string `json:"createdAt"`
}

// AddArtifact inserts a new artifact for a lead and returns its ID.
func (s *Store) AddArtifact(leadID int64, artifactType string, content string) (int64, error) {
	res, err := s.db.Exec(
		"INSERT INTO lead_artifacts (lead_id, type, content, created_at) VALUES (?, ?, ?, ?)",
		leadID, artifactType, content, now(),
	)
	if err != nil {
		return 0, fmt.Errorf("scout: add artifact: %w", err)
	}
	id, _ := res.LastInsertId()
	return id, nil
}

// GetArtifacts returns all artifacts for a lead, newest first.
func (s *Store) GetArtifacts(leadID int64) ([]LeadArtifact, error) {
	rows, err := s.db.Query(
		"SELECT id, lead_id, type, content, created_at FROM lead_artifacts WHERE lead_id = ? ORDER BY id DESC",
		leadID,
	)
	if err != nil {
		return nil, fmt.Errorf("scout: get artifacts: %w", err)
	}
	defer rows.Close()

	var artifacts []LeadArtifact
	for rows.Next() {
		var a LeadArtifact
		if err := rows.Scan(&a.ID, &a.LeadID, &a.Type, &a.Content, &a.CreatedAt); err != nil {
			return nil, fmt.Errorf("scout: scan artifact: %w", err)
		}
		artifacts = append(artifacts, a)
	}
	return artifacts, rows.Err()
}

// GetArtifactsByType returns artifacts for a lead filtered by type, newest first.
func (s *Store) GetArtifactsByType(leadID int64, artifactType string) ([]LeadArtifact, error) {
	rows, err := s.db.Query(
		"SELECT id, lead_id, type, content, created_at FROM lead_artifacts WHERE lead_id = ? AND type = ? ORDER BY id DESC",
		leadID, artifactType,
	)
	if err != nil {
		return nil, fmt.Errorf("scout: get artifacts by type: %w", err)
	}
	defer rows.Close()

	var artifacts []LeadArtifact
	for rows.Next() {
		var a LeadArtifact
		if err := rows.Scan(&a.ID, &a.LeadID, &a.Type, &a.Content, &a.CreatedAt); err != nil {
			return nil, fmt.Errorf("scout: scan artifact: %w", err)
		}
		artifacts = append(artifacts, a)
	}
	return artifacts, rows.Err()
}

// GetLatestArtifact returns the most recent artifact of a given type for a lead.
// Returns nil if no artifact of that type exists.
func (s *Store) GetLatestArtifact(leadID int64, artifactType string) (*LeadArtifact, error) {
	row := s.db.QueryRow(
		"SELECT id, lead_id, type, content, created_at FROM lead_artifacts WHERE lead_id = ? AND type = ? ORDER BY id DESC LIMIT 1",
		leadID, artifactType,
	)
	var a LeadArtifact
	err := row.Scan(&a.ID, &a.LeadID, &a.Type, &a.Content, &a.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("scout: get latest artifact: %w", err)
	}
	return &a, nil
}
