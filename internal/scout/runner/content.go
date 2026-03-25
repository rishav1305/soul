package runner

import (
	"fmt"
	"log"
	"time"

	"github.com/rishav1305/soul/internal/scout/store"
)

const (
	contentStaleDays = 14
)

// ContentPublishPhase finds content posts with status "scheduled" and
// scheduled_date <= today, and logs them as ready to publish.
func ContentPublishPhase(s *store.Store) (int, error) {
	today := time.Now().UTC().Format("2006-01-02")
	posts, err := s.ListContentPosts("", "scheduled")
	if err != nil {
		return 0, fmt.Errorf("content publish: list: %w", err)
	}

	count := 0
	for _, p := range posts {
		if p.ScheduledDate == "" || p.ScheduledDate > today {
			continue
		}
		log.Printf("content publish: post %d (%s) on %s ready to publish (scheduled %s)",
			p.ID, p.Topic, p.Platform, p.ScheduledDate)
		count++
	}
	return count, nil
}

// ContentStalePhase finds content drafts older than 14 days and logs them
// as stale.
func ContentStalePhase(s *store.Store) (int, error) {
	posts, err := s.ListContentPosts("", "draft")
	if err != nil {
		return 0, fmt.Errorf("content stale: list: %w", err)
	}

	cutoff := time.Now().UTC().Add(-time.Duration(contentStaleDays) * 24 * time.Hour)

	count := 0
	for _, p := range posts {
		createdAt, err := time.Parse(time.RFC3339, p.CreatedAt)
		if err != nil {
			// Try datetime format (SQLite default).
			createdAt, err = time.Parse("2006-01-02 15:04:05", p.CreatedAt)
			if err != nil {
				log.Printf("content stale: post %d: bad created_at %q: %v", p.ID, p.CreatedAt, err)
				continue
			}
		}
		if createdAt.Before(cutoff) {
			log.Printf("content stale: post %d (%s) draft since %s", p.ID, p.Topic, p.CreatedAt)
			count++
		}
	}
	return count, nil
}
