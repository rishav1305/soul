package store

import (
	"fmt"
)

// Interaction represents a recorded interaction with a lead (email, call, etc.).
type Interaction struct {
	ID          int64  `json:"id"`
	LeadID      int64  `json:"leadId"`
	Type        string `json:"type"`
	Channel     string `json:"channel"`
	Description string `json:"description"`
	CreatedAt   string `json:"createdAt"`
}

// AddInteraction inserts a new interaction for a lead and returns its ID.
func (s *Store) AddInteraction(leadID int64, interactionType string, channel string, description string) (int64, error) {
	res, err := s.db.Exec(
		"INSERT INTO interactions (lead_id, type, channel, description, created_at) VALUES (?, ?, ?, ?, ?)",
		leadID, interactionType, channel, description, now(),
	)
	if err != nil {
		return 0, fmt.Errorf("scout: add interaction: %w", err)
	}
	id, _ := res.LastInsertId()
	return id, nil
}

// GetInteractions returns all interactions for a lead, newest first.
func (s *Store) GetInteractions(leadID int64) ([]Interaction, error) {
	rows, err := s.db.Query(
		"SELECT id, lead_id, type, channel, description, created_at FROM interactions WHERE lead_id = ? ORDER BY id DESC",
		leadID,
	)
	if err != nil {
		return nil, fmt.Errorf("scout: get interactions: %w", err)
	}
	defer rows.Close()

	var interactions []Interaction
	for rows.Next() {
		var i Interaction
		if err := rows.Scan(&i.ID, &i.LeadID, &i.Type, &i.Channel, &i.Description, &i.CreatedAt); err != nil {
			return nil, fmt.Errorf("scout: scan interaction: %w", err)
		}
		interactions = append(interactions, i)
	}
	return interactions, rows.Err()
}

// GetInteractionCount returns the total number of interactions for a lead.
func (s *Store) GetInteractionCount(leadID int64) (int, error) {
	var count int
	err := s.db.QueryRow(
		"SELECT COUNT(*) FROM interactions WHERE lead_id = ?",
		leadID,
	).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("scout: interaction count: %w", err)
	}
	return count, nil
}
