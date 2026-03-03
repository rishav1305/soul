// Package data provides JSON-based persistence for scout data.
//
// All state — sync results, sweep results, tracked applications, and weekly
// metrics — lives in a single file at ~/.soul/scout/data.json. The Store
// struct wraps that file with a mutex so concurrent callers are safe.
package data

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// ---------------------------------------------------------------------------
// Domain types
// ---------------------------------------------------------------------------

// ScoutData is the top-level structure persisted to data.json.
type ScoutData struct {
	Sync         SyncData             `json:"sync"`
	Sweep        SweepData            `json:"sweep"`
	Applications []Application        `json:"applications"`
	Metrics      map[string]Metric    `json:"metrics"` // keyed by "YYYY-WNN"
}

// SyncData holds results from the most recent platform sync.
type SyncData struct {
	LastRun string         `json:"lastRun"`
	Results []PlatformSync `json:"results"`
}

// SyncDetail records the result of checking a single profile field.
type SyncDetail struct {
	Field    string `json:"field"`
	Expected string `json:"expected"`
	Match    bool   `json:"match"`
}

// PlatformSync records the sync status for a single platform.
type PlatformSync struct {
	Platform  string       `json:"platform"`
	Status    string       `json:"status"` // "synced" or "drift"
	Issues    []string     `json:"issues"`
	Details   []SyncDetail `json:"details,omitempty"`
	CheckedAt string       `json:"checkedAt"`
}

// SweepData holds results from the most recent opportunity sweep.
type SweepData struct {
	LastRun       string        `json:"lastRun"`
	Opportunities []Opportunity `json:"opportunities"`
	Messages      []Message     `json:"messages"`
}

// Opportunity represents a discovered job posting.
type Opportunity struct {
	ID        string  `json:"id"`
	Company   string  `json:"company"`
	Role      string  `json:"role"`
	Platform  string  `json:"platform"`
	Match     float64 `json:"match"`
	URL       string  `json:"url"`
	Location  string  `json:"location,omitempty"`
	PostedAt  string  `json:"postedAt,omitempty"`
	FoundAt   string  `json:"foundAt"`
	Dismissed bool    `json:"dismissed"`
}

// Message represents an unread message from a job platform.
type Message struct {
	ID       string `json:"id"`
	Platform string `json:"platform"`
	From     string `json:"from"`
	Subject  string `json:"subject"`
	Urgent   bool   `json:"urgent"`
	FoundAt  string `json:"foundAt"`
}

// Application tracks a single job application through its lifecycle.
type Application struct {
	ID        string `json:"id"`
	Company   string `json:"company"`
	Role      string `json:"role"`
	Platform  string `json:"platform"`
	Variant   string `json:"variant"`
	Status    string `json:"status"`
	FollowUp  string `json:"followUp"`
	Notes     string `json:"notes"`
	AppliedAt string `json:"appliedAt"`
	UpdatedAt string `json:"updatedAt"`
}

// Metric aggregates application stats for a single ISO week.
type Metric struct {
	Applied    int `json:"applied"`
	Responses  int `json:"responses"`
	Interviews int `json:"interviews"`
	Offers     int `json:"offers"`
}

// ---------------------------------------------------------------------------
// Store
// ---------------------------------------------------------------------------

// Store provides mutex-protected, JSON-file-backed persistence for all scout
// data. Use NewStore to create an instance; it will ensure the data directory
// exists and will load (or initialise) the JSON file.
type Store struct {
	mu       sync.Mutex
	dataDir  string
	filePath string
	data     ScoutData
}

// NewStore creates (or opens) the scout data store.
//
// It resolves ~/.soul/scout/, creates the directory if needed, and either
// loads the existing data.json or writes an empty seed file.
func NewStore() (*Store, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("resolve home dir: %w", err)
	}

	dir := filepath.Join(home, ".soul", "scout")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create data dir %s: %w", dir, err)
	}

	fp := filepath.Join(dir, "data.json")
	s := &Store{
		dataDir:  dir,
		filePath: fp,
		data:     emptyData(),
	}

	// Try to load an existing file; if it doesn't exist we keep the seed.
	raw, err := os.ReadFile(fp)
	if err == nil {
		if err := json.Unmarshal(raw, &s.data); err != nil {
			return nil, fmt.Errorf("parse %s: %w", fp, err)
		}
		// Ensure the metrics map is never nil after load.
		if s.data.Metrics == nil {
			s.data.Metrics = make(map[string]Metric)
		}
	} else if !os.IsNotExist(err) {
		return nil, fmt.Errorf("read %s: %w", fp, err)
	} else {
		// First run — write the seed file.
		if err := s.saveLocked(); err != nil {
			return nil, err
		}
	}

	return s, nil
}

// DataDir returns the path to the scout data directory (~/.soul/scout).
func (s *Store) DataDir() string {
	return s.dataDir
}

// ---------------------------------------------------------------------------
// Save / load helpers
// ---------------------------------------------------------------------------

// Save writes the current in-memory data to data.json.
func (s *Store) Save() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.saveLocked()
}

// saveLocked writes data to disk. Caller must hold s.mu.
func (s *Store) saveLocked() error {
	raw, err := json.MarshalIndent(s.data, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal data: %w", err)
	}
	if err := os.WriteFile(s.filePath, raw, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", s.filePath, err)
	}
	return nil
}

// ---------------------------------------------------------------------------
// Sync
// ---------------------------------------------------------------------------

// SetSyncResults replaces the stored sync data with the provided results and
// persists the change to disk.
func (s *Store) SetSyncResults(results []PlatformSync) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.data.Sync = SyncData{
		LastRun: nowISO(),
		Results: results,
	}
	return s.saveLocked()
}

// GetSyncData returns a copy of the current sync data.
func (s *Store) GetSyncData() SyncData {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.data.Sync
}

// ---------------------------------------------------------------------------
// Sweep
// ---------------------------------------------------------------------------

// SetSweepResults replaces the stored sweep data and persists it.
func (s *Store) SetSweepResults(opportunities []Opportunity, messages []Message) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.data.Sweep = SweepData{
		LastRun:       nowISO(),
		Opportunities: opportunities,
		Messages:      messages,
	}
	return s.saveLocked()
}

// GetSweepData returns a copy of the current sweep data.
func (s *Store) GetSweepData() SweepData {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.data.Sweep
}

// ---------------------------------------------------------------------------
// Applications
// ---------------------------------------------------------------------------

// AddApplication stores a new application. It auto-generates the ID using the
// current Unix timestamp, sets AppliedAt to now, and increments the weekly
// "applied" metric.
func (s *Store) AddApplication(app Application) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	app.ID = fmt.Sprintf("app-%d", now.UnixMilli())
	app.AppliedAt = now.Format(time.RFC3339)
	app.UpdatedAt = app.AppliedAt

	s.data.Applications = append(s.data.Applications, app)

	// Bump the weekly metric.
	week := weekKey(now)
	m := s.data.Metrics[week]
	m.Applied++
	s.data.Metrics[week] = m

	return s.saveLocked()
}

// UpdateApplication patches the application identified by id. Only non-empty
// values are applied.
func (s *Store) UpdateApplication(id, status, followUp, notes string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i := range s.data.Applications {
		if s.data.Applications[i].ID == id {
			if status != "" {
				s.data.Applications[i].Status = status
			}
			if followUp != "" {
				s.data.Applications[i].FollowUp = followUp
			}
			if notes != "" {
				s.data.Applications[i].Notes = notes
			}
			s.data.Applications[i].UpdatedAt = nowISO()
			return s.saveLocked()
		}
	}
	return fmt.Errorf("application %q not found", id)
}

// ListApplications returns applications optionally filtered by status.
// Pass an empty string to get all applications.
func (s *Store) ListApplications(filterStatus string) []Application {
	s.mu.Lock()
	defer s.mu.Unlock()

	if filterStatus == "" {
		// Return a copy so the caller can't mutate our slice.
		out := make([]Application, len(s.data.Applications))
		copy(out, s.data.Applications)
		return out
	}

	var out []Application
	for _, a := range s.data.Applications {
		if a.Status == filterStatus {
			out = append(out, a)
		}
	}
	return out
}

// ---------------------------------------------------------------------------
// Report
// ---------------------------------------------------------------------------

// GetReportData returns a deep-ish copy of the full data set for the report
// tool to format as it sees fit.
func (s *Store) GetReportData() ScoutData {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Copy top-level slices so the caller can't mutate our state.
	cp := s.data
	cp.Sync.Results = copySlice(s.data.Sync.Results)
	cp.Sweep.Opportunities = copySlice(s.data.Sweep.Opportunities)
	cp.Sweep.Messages = copySlice(s.data.Sweep.Messages)
	cp.Applications = copySlice(s.data.Applications)
	cp.Metrics = make(map[string]Metric, len(s.data.Metrics))
	for k, v := range s.data.Metrics {
		cp.Metrics[k] = v
	}
	return cp
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func emptyData() ScoutData {
	return ScoutData{
		Sync: SyncData{
			Results: []PlatformSync{},
		},
		Sweep: SweepData{
			Opportunities: []Opportunity{},
			Messages:      []Message{},
		},
		Applications: []Application{},
		Metrics:      make(map[string]Metric),
	}
}

func nowISO() string {
	return time.Now().Format(time.RFC3339)
}

// weekKey returns the ISO-week key for t in the format "YYYY-WNN".
func weekKey(t time.Time) string {
	year, week := t.ISOWeek()
	return fmt.Sprintf("%d-W%02d", year, week)
}

// copySlice returns a shallow copy of a slice of any type.
func copySlice[T any](src []T) []T {
	if src == nil {
		return nil
	}
	dst := make([]T, len(src))
	copy(dst, src)
	return dst
}
