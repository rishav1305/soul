package store

import (
	"database/sql"
	"fmt"
)

// AddLead inserts a new lead and returns its ID.
func (s *Store) AddLead(lead Lead) (int64, error) {
	ts := now()
	if lead.CreatedAt == "" {
		lead.CreatedAt = ts
	}
	if lead.UpdatedAt == "" {
		lead.UpdatedAt = ts
	}

	res, err := s.db.Exec(`
		INSERT INTO leads (title, company, type, source, source_url, pipeline, stage,
			compensation, currency, contact, location, tags, notes, metadata, variant,
			next_action, next_date, created_at, updated_at, closed_at, match_score, job_description)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		lead.Title, lead.Company, lead.Type, lead.Source, lead.SourceURL,
		lead.Pipeline, lead.Stage, lead.Compensation, lead.Currency, lead.Contact,
		lead.Location, lead.Tags, lead.Notes, lead.Metadata, lead.Variant,
		lead.NextAction, lead.NextDate, lead.CreatedAt, lead.UpdatedAt,
		lead.ClosedAt, lead.MatchScore, lead.JobDescription,
	)
	if err != nil {
		return 0, fmt.Errorf("scout: add lead: %w", err)
	}
	id, _ := res.LastInsertId()
	return id, nil
}

// GetLead retrieves a lead by ID.
func (s *Store) GetLead(id int64) (*Lead, error) {
	row := s.db.QueryRow("SELECT "+leadColumns+" FROM leads WHERE id = ?", id)
	l, err := scanLead(row)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("scout: lead not found: %d", id)
	}
	if err != nil {
		return nil, fmt.Errorf("scout: get lead: %w", err)
	}
	return l, nil
}

// ListLeads returns leads, optionally filtered by type and/or active-only.
// Active means closed_at is empty.
func (s *Store) ListLeads(typeFilter string, activeOnly bool) ([]Lead, error) {
	query := "SELECT " + leadColumns + " FROM leads"
	var conditions []string
	var args []interface{}

	if typeFilter != "" {
		conditions = append(conditions, "type = ?")
		args = append(args, typeFilter)
	}
	if activeOnly {
		conditions = append(conditions, "closed_at = ''")
	}
	if len(conditions) > 0 {
		query += " WHERE " + joinAnd(conditions)
	}
	query += " ORDER BY updated_at DESC"

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("scout: list leads: %w", err)
	}
	defer rows.Close()

	var leads []Lead
	for rows.Next() {
		l, err := scanLead(rows)
		if err != nil {
			return nil, fmt.Errorf("scout: scan lead: %w", err)
		}
		leads = append(leads, *l)
	}
	return leads, rows.Err()
}

// ScoredLeads returns leads ordered by match_score descending, limited to limit.
func (s *Store) ScoredLeads(limit int) ([]Lead, error) {
	query := "SELECT " + leadColumns + " FROM leads ORDER BY match_score DESC LIMIT ?"
	rows, err := s.db.Query(query, limit)
	if err != nil {
		return nil, fmt.Errorf("scout: scored leads: %w", err)
	}
	defer rows.Close()

	var leads []Lead
	for rows.Next() {
		l, err := scanLead(rows)
		if err != nil {
			return nil, fmt.Errorf("scout: scan lead: %w", err)
		}
		leads = append(leads, *l)
	}
	return leads, rows.Err()
}

// RecordStageHistory inserts a stage transition record.
func (s *Store) RecordStageHistory(leadID int64, from, to, notes string) error {
	_, err := s.db.Exec(
		"INSERT INTO stage_history (lead_id, from_stage, to_stage, changed_at, notes) VALUES (?, ?, ?, ?, ?)",
		leadID, from, to, now(), notes,
	)
	if err != nil {
		return fmt.Errorf("scout: record stage history: %w", err)
	}
	return nil
}

// GetStageHistory returns stage history for a lead, newest first.
func (s *Store) GetStageHistory(leadID int64) ([]StageHistory, error) {
	rows, err := s.db.Query(
		"SELECT id, lead_id, from_stage, to_stage, changed_at, notes FROM stage_history WHERE lead_id = ? ORDER BY changed_at DESC",
		leadID,
	)
	if err != nil {
		return nil, fmt.Errorf("scout: get stage history: %w", err)
	}
	defer rows.Close()

	var history []StageHistory
	for rows.Next() {
		var h StageHistory
		if err := rows.Scan(&h.ID, &h.LeadID, &h.FromStage, &h.ToStage, &h.ChangedAt, &h.Notes); err != nil {
			return nil, fmt.Errorf("scout: scan stage history: %w", err)
		}
		history = append(history, h)
	}
	return history, rows.Err()
}

func joinAnd(parts []string) string {
	result := ""
	for i, p := range parts {
		if i > 0 {
			result += " AND "
		}
		result += p
	}
	return result
}
