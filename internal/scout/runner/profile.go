package runner

import (
	"fmt"
	"log"
	"time"

	"github.com/rishav1305/soul/internal/scout/store"
)

const (
	profileAuditIntervalDays = 90
)

// ProfileAuditPhase checks if a profile audit is due by examining the most
// recent "profile_audit" artifact across all leads. If the last audit was
// more than 90 days ago (or none exists), it logs a reminder.
func ProfileAuditPhase(s *store.Store) (int, error) {
	// Find the most recent profile_audit artifact across all leads.
	var createdAt string
	err := s.DB().QueryRow(
		`SELECT created_at FROM lead_artifacts
		 WHERE type = ? ORDER BY created_at DESC LIMIT 1`,
		"profile_audit",
	).Scan(&createdAt)

	if err != nil {
		// No audit found — log reminder.
		log.Printf("profile audit: no previous audit found — audit recommended")
		return 1, nil
	}

	auditTime, err := parseTime(createdAt)
	if err != nil {
		return 0, fmt.Errorf("profile audit: parse time %q: %w", createdAt, err)
	}

	cutoff := time.Now().UTC().Add(-time.Duration(profileAuditIntervalDays) * 24 * time.Hour)
	if auditTime.Before(cutoff) {
		log.Printf("profile audit: last audit at %s (>%d days ago) — audit recommended",
			createdAt, profileAuditIntervalDays)
		return 1, nil
	}

	return 0, nil
}

// parseTime tries RFC3339 first, then SQLite datetime format.
func parseTime(s string) (time.Time, error) {
	t, err := time.Parse(time.RFC3339, s)
	if err == nil {
		return t, nil
	}
	return time.Parse("2006-01-02 15:04:05", s)
}
