package store

import "fmt"

// AddAgentRun inserts a new agent run record and returns its ID.
func (s *Store) AddAgentRun(run AgentRun) (int64, error) {
	if run.CreatedAt == "" {
		run.CreatedAt = now()
	}
	if run.Status == "" {
		run.Status = "pending"
	}
	res, err := s.db.Exec(`
		INSERT INTO agent_runs (platform, mode, lead_id, status, recommendations, approved_changes, result, created_at, completed_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		run.Platform, run.Mode, run.LeadID, run.Status, run.Recommendations, run.ApprovedChanges, run.Result, run.CreatedAt, run.CompletedAt,
	)
	if err != nil {
		return 0, fmt.Errorf("scout: add agent run: %w", err)
	}
	id, _ := res.LastInsertId()
	return id, nil
}

// GetAgentRun retrieves an agent run by ID.
func (s *Store) GetAgentRun(id int64) (*AgentRun, error) {
	var r AgentRun
	err := s.db.QueryRow(
		"SELECT id, platform, mode, lead_id, status, recommendations, approved_changes, result, created_at, completed_at FROM agent_runs WHERE id = ?",
		id,
	).Scan(&r.ID, &r.Platform, &r.Mode, &r.LeadID, &r.Status, &r.Recommendations, &r.ApprovedChanges, &r.Result, &r.CreatedAt, &r.CompletedAt)
	if err != nil {
		return nil, fmt.Errorf("scout: get agent run: %w", err)
	}
	return &r, nil
}

// UpdateAgentRun updates an agent run's status and result.
// On failure/timeout, the error is stored in result as JSON: {"error": "..."}.
func (s *Store) UpdateAgentRun(id int64, status, result string) error {
	completedAt := ""
	if status == "completed" || status == "failed" || status == "timeout" {
		completedAt = now()
	}
	_, err := s.db.Exec(
		"UPDATE agent_runs SET status = ?, result = ?, completed_at = ? WHERE id = ?",
		status, result, completedAt, id,
	)
	return err
}

// LatestAgentRun returns the most recent agent run, or nil if none exist.
func (s *Store) LatestAgentRun() (*AgentRun, error) {
	var r AgentRun
	err := s.db.QueryRow(
		"SELECT id, platform, mode, lead_id, status, recommendations, approved_changes, result, created_at, completed_at FROM agent_runs ORDER BY id DESC LIMIT 1",
	).Scan(&r.ID, &r.Platform, &r.Mode, &r.LeadID, &r.Status, &r.Recommendations, &r.ApprovedChanges, &r.Result, &r.CreatedAt, &r.CompletedAt)
	if err != nil {
		return nil, nil // no rows or error — both mean "no latest"
	}
	return &r, nil
}

// ListAgentRuns returns agent runs, optionally filtered by platform.
func (s *Store) ListAgentRuns(platform string) ([]AgentRun, error) {
	query := "SELECT id, platform, mode, lead_id, status, recommendations, approved_changes, result, created_at, completed_at FROM agent_runs"
	var args []interface{}
	if platform != "" {
		query += " WHERE platform = ?"
		args = append(args, platform)
	}
	query += " ORDER BY id DESC"

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("scout: list agent runs: %w", err)
	}
	defer rows.Close()

	var runs []AgentRun
	for rows.Next() {
		var r AgentRun
		if err := rows.Scan(&r.ID, &r.Platform, &r.Mode, &r.LeadID, &r.Status, &r.Recommendations, &r.ApprovedChanges, &r.Result, &r.CreatedAt, &r.CompletedAt); err != nil {
			return nil, fmt.Errorf("scout: scan agent run: %w", err)
		}
		runs = append(runs, r)
	}
	return runs, rows.Err()
}
