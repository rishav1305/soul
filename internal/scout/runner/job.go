package runner

import (
	"fmt"
	"log"
	"time"

	"github.com/rishav1305/soul-v2/internal/scout/pipelines"
	"github.com/rishav1305/soul-v2/internal/scout/store"
)

const (
	qualifyBatchSize    = 5
	qualifyScoreThresh  = 70.0
	staleDaysWarning    = 7
	staleDaysAutoSkip   = 14
)

// QualifyPhase finds discovered job leads in tier 1 or 2 and advances them
// based on their match score. Leads with score > 70 move to qualified;
// leads with score <= 70 move to skipped. Unscored leads are left for the
// ResumeMatch tool to process separately.
func QualifyPhase(s *store.Store) (int, error) {
	leads, err := s.DB().Query(
		`SELECT id, stage, match_score FROM leads
		 WHERE pipeline = ? AND stage = ? AND tier IN (?, ?)
		 LIMIT ?`,
		"job", "discovered", 1, 2, qualifyBatchSize,
	)
	if err != nil {
		return 0, fmt.Errorf("qualify: query: %w", err)
	}
	defer leads.Close()

	type candidate struct {
		id         int64
		matchScore float64
	}
	var candidates []candidate
	for leads.Next() {
		var c candidate
		var stage string
		if err := leads.Scan(&c.id, &stage, &c.matchScore); err != nil {
			return 0, fmt.Errorf("qualify: scan: %w", err)
		}
		candidates = append(candidates, c)
	}
	if err := leads.Err(); err != nil {
		return 0, fmt.Errorf("qualify: rows: %w", err)
	}

	processed := 0
	for _, c := range candidates {
		if c.matchScore <= 0 {
			// Not yet scored -- skip for now, ResumeMatch tool will handle it.
			continue
		}

		var toStage string
		if c.matchScore > qualifyScoreThresh {
			toStage = "qualified"
		} else {
			toStage = "skipped"
		}

		if err := advanceLead(s, c.id, "job", "discovered", toStage); err != nil {
			log.Printf("qualify: lead %d: %v", c.id, err)
			continue
		}
		processed++
	}
	return processed, nil
}

// PreparePhase finds qualified job leads that need artifact generation.
// Currently logs readiness; actual AI tool calls will be wired later.
func PreparePhase(s *store.Store) (int, error) {
	leads, err := s.DB().Query(
		`SELECT id, job_title FROM leads
		 WHERE pipeline = ? AND stage = ?`,
		"job", "qualified",
	)
	if err != nil {
		return 0, fmt.Errorf("prepare: query: %w", err)
	}
	defer leads.Close()

	count := 0
	for leads.Next() {
		var id int64
		var title string
		if err := leads.Scan(&id, &title); err != nil {
			return 0, fmt.Errorf("prepare: scan: %w", err)
		}
		log.Printf("prepare: lead %d (%s) ready for preparation", id, title)
		count++
	}
	return count, leads.Err()
}

// CadencePhase finds job leads in outreach-sent with a follow-up date due
// (next_date is non-empty and <= today). Logs them as follow-up due.
func CadencePhase(s *store.Store) (int, error) {
	today := time.Now().UTC().Format(time.RFC3339)
	leads, err := s.DB().Query(
		`SELECT id, job_title, next_date FROM leads
		 WHERE pipeline = ? AND stage = ? AND next_date != '' AND next_date <= ?`,
		"job", "outreach-sent", today,
	)
	if err != nil {
		return 0, fmt.Errorf("cadence: query: %w", err)
	}
	defer leads.Close()

	count := 0
	for leads.Next() {
		var id int64
		var title, nextDate string
		if err := leads.Scan(&id, &title, &nextDate); err != nil {
			return 0, fmt.Errorf("cadence: scan: %w", err)
		}
		log.Printf("cadence: lead %d (%s) follow-up due (next_date=%s)", id, title, nextDate)
		count++
	}
	return count, leads.Err()
}

// StalePhase finds job leads stuck in the preparing stage. Leads older than
// 7 days are logged as stale. Leads older than 14 days are auto-skipped.
func StalePhase(s *store.Store) (int, error) {
	warningCutoff := time.Now().UTC().Add(-time.Duration(staleDaysWarning) * 24 * time.Hour).Format(time.RFC3339)
	leads, err := s.DB().Query(
		`SELECT id, job_title, updated_at FROM leads
		 WHERE pipeline = ? AND stage = ? AND updated_at < ?`,
		"job", "preparing", warningCutoff,
	)
	if err != nil {
		return 0, fmt.Errorf("stale: query: %w", err)
	}
	defer leads.Close()

	autoSkipCutoff := time.Now().UTC().Add(-time.Duration(staleDaysAutoSkip) * 24 * time.Hour)

	type staleLead struct {
		id        int64
		title     string
		updatedAt time.Time
	}
	var staleLeads []staleLead
	for leads.Next() {
		var id int64
		var title, updatedStr string
		if err := leads.Scan(&id, &title, &updatedStr); err != nil {
			return 0, fmt.Errorf("stale: scan: %w", err)
		}
		updatedAt, err := time.Parse(time.RFC3339, updatedStr)
		if err != nil {
			log.Printf("stale: lead %d: bad updated_at %q: %v", id, updatedStr, err)
			continue
		}
		staleLeads = append(staleLeads, staleLead{id: id, title: title, updatedAt: updatedAt})
	}
	if err := leads.Err(); err != nil {
		return 0, fmt.Errorf("stale: rows: %w", err)
	}

	processed := 0
	for _, sl := range staleLeads {
		if sl.updatedAt.Before(autoSkipCutoff) {
			if err := advanceLead(s, sl.id, "job", "preparing", "skipped"); err != nil {
				log.Printf("stale: lead %d auto-skip failed: %v", sl.id, err)
				continue
			}
			log.Printf("stale: lead %d (%s) auto-skipped (>%d days)", sl.id, sl.title, staleDaysAutoSkip)
		} else {
			log.Printf("stale: lead %d (%s) stale (>%d days)", sl.id, sl.title, staleDaysWarning)
		}
		processed++
	}
	return processed, nil
}

// advanceLead validates a stage transition, updates the lead, and records history.
func advanceLead(s *store.Store, id int64, pipeline, from, to string) error {
	if err := pipelines.ValidateTransition(pipeline, from, to); err != nil {
		return fmt.Errorf("advance lead %d: %w", id, err)
	}
	if err := s.UpdateLead(id, map[string]interface{}{
		"stage": to,
	}); err != nil {
		return fmt.Errorf("advance lead %d: update: %w", id, err)
	}
	if err := s.RecordStageHistory(id, from, to, "runner: auto-advance"); err != nil {
		return fmt.Errorf("advance lead %d: history: %w", id, err)
	}
	return nil
}
