package store

import (
	"database/sql"
	"fmt"
)

// AddLeadIfNotExists inserts a lead if no existing lead has the same theirstack_id.
// Returns the lead ID, whether it was newly created, and any error.
// If theirstack_id is nil, it always inserts (no dedup key).
func (s *Store) AddLeadIfNotExists(lead Lead) (int64, bool, error) {
	if lead.TheirStackID != nil {
		var existingID int64
		err := s.db.QueryRow("SELECT id FROM leads WHERE theirstack_id = ?", *lead.TheirStackID).Scan(&existingID)
		if err == nil {
			// Already exists.
			return existingID, false, nil
		}
		if err != sql.ErrNoRows {
			return 0, false, fmt.Errorf("scout: check lead exists: %w", err)
		}
	}
	id, err := s.AddLead(lead)
	if err != nil {
		return 0, false, err
	}
	return id, true, nil
}

// AddLead inserts a new lead and returns its ID.
func (s *Store) AddLead(lead Lead) (int64, error) {
	ts := now()
	if lead.CreatedAt == "" {
		lead.CreatedAt = ts
	}
	if lead.UpdatedAt == "" {
		lead.UpdatedAt = ts
	}
	if lead.Source == "" {
		lead.Source = "theirstack"
	}
	if lead.Stage == "" {
		lead.Stage = "discovered"
	}
	if lead.NextAction == "" {
		lead.NextAction = "review"
	}
	if lead.EmploymentStatuses == "" {
		lead.EmploymentStatuses = "[]"
	}
	if lead.TechnologySlugs == "" {
		lead.TechnologySlugs = "[]"
	}
	if lead.KeywordSlugs == "" {
		lead.KeywordSlugs = "[]"
	}
	if lead.Metadata == "" {
		lead.Metadata = "{}"
	}

	res, err := s.db.Exec(`
		INSERT INTO leads (
			source, pipeline, stage, match_score,
			next_action, next_date, notes, created_at, updated_at, closed_at,
			theirstack_id,
			job_title, url, final_url, source_url,
			date_posted, discovered_at, description, normalized_title,
			location, short_location, country, country_code, remote, hybrid,
			salary_string, min_annual_salary_usd, max_annual_salary_usd, salary_currency,
			seniority, employment_statuses, easy_apply, technology_slugs, keyword_slugs,
			company, company_domain, company_industry, company_employee_count,
			company_linkedin_url, company_total_funding_usd, company_funding_stage,
			company_logo, company_country,
			hiring_manager, hiring_manager_linkedin,
			metadata
		) VALUES (
			?, ?, ?, ?,
			?, ?, ?, ?, ?, ?,
			?,
			?, ?, ?, ?,
			?, ?, ?, ?,
			?, ?, ?, ?, ?, ?,
			?, ?, ?, ?,
			?, ?, ?, ?, ?,
			?, ?, ?, ?,
			?, ?, ?,
			?, ?,
			?, ?,
			?
		)`,
		lead.Source, lead.Pipeline, lead.Stage, lead.MatchScore,
		lead.NextAction, lead.NextDate, lead.Notes, lead.CreatedAt, lead.UpdatedAt, lead.ClosedAt,
		lead.TheirStackID,
		lead.JobTitle, lead.URL, lead.FinalURL, lead.SourceURL,
		lead.DatePosted, lead.DiscoveredAt, lead.Description, lead.NormalizedTitle,
		lead.Location, lead.ShortLocation, lead.Country, lead.CountryCode, lead.Remote, lead.Hybrid,
		lead.SalaryString, lead.MinAnnualSalaryUSD, lead.MaxAnnualSalaryUSD, lead.SalaryCurrency,
		lead.Seniority, lead.EmploymentStatuses, lead.EasyApply, lead.TechnologySlugs, lead.KeywordSlugs,
		lead.Company, lead.CompanyDomain, lead.CompanyIndustry, lead.CompanyEmployeeCount,
		lead.CompanyLinkedInURL, lead.CompanyTotalFundingUSD, lead.CompanyFundingStage,
		lead.CompanyLogo, lead.CompanyCountry,
		lead.HiringManager, lead.HiringManagerLinkedIn,
		lead.Metadata,
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

// ListLeads returns leads, optionally filtered by pipeline and/or active-only.
// Active means closed_at is empty.
func (s *Store) ListLeads(pipelineFilter string, activeOnly bool) ([]Lead, error) {
	query := "SELECT " + leadColumns + " FROM leads"
	var conditions []string
	var args []interface{}

	if pipelineFilter != "" {
		conditions = append(conditions, "pipeline = ?")
		args = append(args, pipelineFilter)
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
