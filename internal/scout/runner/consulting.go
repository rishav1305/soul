package runner

import (
	"fmt"
	"log"
	"time"

	"github.com/rishav1305/soul-v2/internal/scout/store"
)

const (
	consultingStaleDays = 14
)

// ConsultingQualifyPhase finds consulting leads at "lead" that have any
// artifacts and advances them to "discovery-call".
func ConsultingQualifyPhase(s *store.Store) (int, error) {
	rows, err := s.DB().Query(
		`SELECT id FROM leads WHERE pipeline = ? AND stage = ?`,
		"consulting", "lead",
	)
	if err != nil {
		return 0, fmt.Errorf("consulting qualify: query: %w", err)
	}
	defer rows.Close()

	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return 0, fmt.Errorf("consulting qualify: scan: %w", err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return 0, fmt.Errorf("consulting qualify: rows: %w", err)
	}

	processed := 0
	for _, id := range ids {
		artifacts, err := s.GetArtifacts(id)
		if err != nil {
			log.Printf("consulting qualify: lead %d: get artifacts: %v", id, err)
			continue
		}
		if len(artifacts) == 0 {
			continue
		}
		if err := advanceLead(s, id, "consulting", "lead", "discovery-call"); err != nil {
			log.Printf("consulting qualify: lead %d: %v", id, err)
			continue
		}
		processed++
	}
	return processed, nil
}

// ConsultingStalePhase finds consulting leads not updated in 14 days
// (excluding terminal stages) and logs them as stale.
func ConsultingStalePhase(s *store.Store) (int, error) {
	cutoff := time.Now().UTC().Add(-time.Duration(consultingStaleDays) * 24 * time.Hour).Format(time.RFC3339)
	rows, err := s.DB().Query(
		`SELECT id, job_title, stage, updated_at FROM leads
		 WHERE pipeline = ? AND stage NOT IN (?, ?, ?)
		 AND updated_at < ?`,
		"consulting", "delivered", "lost", "declined", cutoff,
	)
	if err != nil {
		return 0, fmt.Errorf("consulting stale: query: %w", err)
	}
	defer rows.Close()

	count := 0
	for rows.Next() {
		var id int64
		var title, stage, updatedAt string
		if err := rows.Scan(&id, &title, &stage, &updatedAt); err != nil {
			return 0, fmt.Errorf("consulting stale: scan: %w", err)
		}
		log.Printf("consulting stale: lead %d (%s) at %q since %s", id, title, stage, updatedAt)
		count++
	}
	return count, rows.Err()
}
