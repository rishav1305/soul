package runner

import (
	"fmt"
	"log"
	"time"

	"github.com/rishav1305/soul/internal/scout/store"
)

const (
	freelanceQualifyScoreThresh = 70.0
	freelanceStaleDays          = 7
)

// FreelanceQualifyPhase finds freelance leads at "found" with match_score > 70
// and advances them to "proposal-ready".
func FreelanceQualifyPhase(s *store.Store) (int, error) {
	rows, err := s.DB().Query(
		`SELECT id, match_score FROM leads
		 WHERE pipeline = ? AND stage = ? AND match_score > ?`,
		"freelance", "found", freelanceQualifyScoreThresh,
	)
	if err != nil {
		return 0, fmt.Errorf("freelance qualify: query: %w", err)
	}
	defer rows.Close()

	type candidate struct {
		id         int64
		matchScore float64
	}
	var candidates []candidate
	for rows.Next() {
		var c candidate
		if err := rows.Scan(&c.id, &c.matchScore); err != nil {
			return 0, fmt.Errorf("freelance qualify: scan: %w", err)
		}
		candidates = append(candidates, c)
	}
	if err := rows.Err(); err != nil {
		return 0, fmt.Errorf("freelance qualify: rows: %w", err)
	}

	processed := 0
	for _, c := range candidates {
		if err := advanceLead(s, c.id, "freelance", "found", "proposal-ready"); err != nil {
			log.Printf("freelance qualify: lead %d: %v", c.id, err)
			continue
		}
		processed++
	}
	return processed, nil
}

// FreelanceStalePhase finds freelance leads at "proposal-ready" for more than
// 7 days without advancing and logs them as stale.
func FreelanceStalePhase(s *store.Store) (int, error) {
	cutoff := time.Now().UTC().Add(-time.Duration(freelanceStaleDays) * 24 * time.Hour).Format(time.RFC3339)
	rows, err := s.DB().Query(
		`SELECT id, job_title, updated_at FROM leads
		 WHERE pipeline = ? AND stage = ? AND updated_at < ?`,
		"freelance", "proposal-ready", cutoff,
	)
	if err != nil {
		return 0, fmt.Errorf("freelance stale: query: %w", err)
	}
	defer rows.Close()

	count := 0
	for rows.Next() {
		var id int64
		var title, updatedAt string
		if err := rows.Scan(&id, &title, &updatedAt); err != nil {
			return 0, fmt.Errorf("freelance stale: scan: %w", err)
		}
		log.Printf("freelance stale: lead %d (%s) at proposal-ready since %s", id, title, updatedAt)
		count++
	}
	return count, rows.Err()
}
