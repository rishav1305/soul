package runner

import (
	"fmt"
	"log"
	"time"

	"github.com/rishav1305/soul-v2/internal/scout/store"
)

const (
	contractQualifyScoreThresh = 70.0
	contractFollowUpDays       = 3
)

// ContractQualifyPhase finds contract leads at "discovered" with match_score > 70
// and advances them to "applied".
func ContractQualifyPhase(s *store.Store) (int, error) {
	rows, err := s.DB().Query(
		`SELECT id, match_score FROM leads
		 WHERE pipeline = ? AND stage = ? AND match_score > ?`,
		"contract", "discovered", contractQualifyScoreThresh,
	)
	if err != nil {
		return 0, fmt.Errorf("contract qualify: query: %w", err)
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
			return 0, fmt.Errorf("contract qualify: scan: %w", err)
		}
		candidates = append(candidates, c)
	}
	if err := rows.Err(); err != nil {
		return 0, fmt.Errorf("contract qualify: rows: %w", err)
	}

	processed := 0
	for _, c := range candidates {
		if err := advanceLead(s, c.id, "contract", "discovered", "applied"); err != nil {
			log.Printf("contract qualify: lead %d: %v", c.id, err)
			continue
		}
		processed++
	}
	return processed, nil
}

// ContractEngagedPhase finds contract leads at "offer" that have been there
// for more than 3 days and logs a reminder to follow up.
func ContractEngagedPhase(s *store.Store) (int, error) {
	cutoff := time.Now().UTC().Add(-time.Duration(contractFollowUpDays) * 24 * time.Hour).Format(time.RFC3339)
	rows, err := s.DB().Query(
		`SELECT id, job_title, updated_at FROM leads
		 WHERE pipeline = ? AND stage = ? AND updated_at < ?`,
		"contract", "offer", cutoff,
	)
	if err != nil {
		return 0, fmt.Errorf("contract engaged: query: %w", err)
	}
	defer rows.Close()

	count := 0
	for rows.Next() {
		var id int64
		var title, updatedAt string
		if err := rows.Scan(&id, &title, &updatedAt); err != nil {
			return 0, fmt.Errorf("contract engaged: scan: %w", err)
		}
		log.Printf("contract engaged: lead %d (%s) at offer since %s — follow up", id, title, updatedAt)
		count++
	}
	return count, rows.Err()
}
