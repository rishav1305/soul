package runner

import (
	"fmt"
	"log"
	"time"

	"github.com/rishav1305/soul-v2/internal/scout/store"
)

const (
	networkingEngageThreshold = 3
	networkingWarmThreshold   = 5
	networkingStaleDays       = 30
)

// NetworkingEngagePhase finds networking leads at "connected" with at least
// 3 interactions and advances them to "engaging".
func NetworkingEngagePhase(s *store.Store) (int, error) {
	rows, err := s.DB().Query(
		`SELECT id FROM leads WHERE pipeline = ? AND stage = ?`,
		"networking", "connected",
	)
	if err != nil {
		return 0, fmt.Errorf("networking engage: query: %w", err)
	}
	defer rows.Close()

	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return 0, fmt.Errorf("networking engage: scan: %w", err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return 0, fmt.Errorf("networking engage: rows: %w", err)
	}

	processed := 0
	for _, id := range ids {
		count, err := s.GetInteractionCount(id)
		if err != nil {
			log.Printf("networking engage: lead %d: interaction count: %v", id, err)
			continue
		}
		if count < networkingEngageThreshold {
			continue
		}
		if err := advanceLead(s, id, "networking", "connected", "engaging"); err != nil {
			log.Printf("networking engage: lead %d: %v", id, err)
			continue
		}
		processed++
	}
	return processed, nil
}

// NetworkingWarmPhase finds networking leads at "engaging" with at least
// 5 interactions and advances them to "warm".
func NetworkingWarmPhase(s *store.Store) (int, error) {
	rows, err := s.DB().Query(
		`SELECT id FROM leads WHERE pipeline = ? AND stage = ?`,
		"networking", "engaging",
	)
	if err != nil {
		return 0, fmt.Errorf("networking warm: query: %w", err)
	}
	defer rows.Close()

	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return 0, fmt.Errorf("networking warm: scan: %w", err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return 0, fmt.Errorf("networking warm: rows: %w", err)
	}

	processed := 0
	for _, id := range ids {
		count, err := s.GetInteractionCount(id)
		if err != nil {
			log.Printf("networking warm: lead %d: interaction count: %v", id, err)
			continue
		}
		if count < networkingWarmThreshold {
			continue
		}
		if err := advanceLead(s, id, "networking", "engaging", "warm"); err != nil {
			log.Printf("networking warm: lead %d: %v", id, err)
			continue
		}
		processed++
	}
	return processed, nil
}

// NetworkingStalePhase finds networking leads at any non-terminal stage with
// no interaction in the last 30 days and logs them as dormant.
func NetworkingStalePhase(s *store.Store) (int, error) {
	cutoff := time.Now().UTC().Add(-time.Duration(networkingStaleDays) * 24 * time.Hour).Format(time.RFC3339)
	rows, err := s.DB().Query(
		`SELECT id, job_title, stage, last_interaction_at FROM leads
		 WHERE pipeline = ? AND stage NOT IN (?, ?, ?)
		 AND (last_interaction_at != '' AND last_interaction_at < ?)`,
		"networking", "converted", "inactive", "not-relevant", cutoff,
	)
	if err != nil {
		return 0, fmt.Errorf("networking stale: query: %w", err)
	}
	defer rows.Close()

	count := 0
	for rows.Next() {
		var id int64
		var title, stage, lastInteraction string
		if err := rows.Scan(&id, &title, &stage, &lastInteraction); err != nil {
			return 0, fmt.Errorf("networking stale: scan: %w", err)
		}
		log.Printf("networking stale: lead %d (%s) at %q dormant since %s", id, title, stage, lastInteraction)
		count++
	}
	return count, rows.Err()
}
