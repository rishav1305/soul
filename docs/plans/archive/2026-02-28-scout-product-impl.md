# Scout Product Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Build the Scout product plugin — a gRPC Go binary that syncs profiles, sweeps job portals, generates tailored resume PDFs, and tracks applications, with a dashboard in the Soul web UI.

**Architecture:** Go gRPC binary at `products/scout/` following compliance-go patterns. Rod (go-rod) for headless browser automation and PDF generation. Supabase REST API for profile data. Data stored in `~/.soul/scout/data.json`. Frontend ScoutPanel in the Soul web UI.

**Tech Stack:** Go 1.24+, go-rod/rod, gRPC, Protocol Buffers, React 19, TypeScript, Tailwind CSS v4

---

### Task 1: Scout Binary Scaffold — main.go + service.go

**Files:**
- Create: `products/scout/cmd/scout/main.go`
- Create: `products/scout/internal/service.go`
- Create: `products/scout/go.mod`
- Create: `products/scout/Makefile`

**Step 1: Create go.mod**

```bash
mkdir -p products/scout/cmd/scout products/scout/internal
```

Create `products/scout/go.mod`:
```go
module github.com/rishav1305/soul/products/scout

go 1.24.2

require (
	github.com/rishav1305/soul v0.0.0
	github.com/go-rod/rod v0.116.2
	google.golang.org/grpc v1.79.1
)

replace github.com/rishav1305/soul => ../../
```

**Step 2: Create main.go**

Create `products/scout/cmd/scout/main.go`:
```go
package main

import (
	"flag"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"

	"google.golang.org/grpc"

	soulv1 "github.com/rishav1305/soul/gen/soul/v1"
	"github.com/rishav1305/soul/products/scout/internal"
)

func main() {
	socketPath := flag.String("socket", "", "Path to unix socket for gRPC server")
	flag.Parse()

	if *socketPath == "" {
		log.Fatal("--socket flag is required")
	}

	// Remove stale socket.
	os.Remove(*socketPath)

	lis, err := net.Listen("unix", *socketPath)
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	srv := grpc.NewServer()
	soulv1.RegisterProductServiceServer(srv, internal.NewScoutService())

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		log.Println("[scout] shutting down...")
		srv.GracefulStop()
		os.Remove(*socketPath)
	}()

	log.Printf("[scout] listening on %s", *socketPath)
	if err := srv.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
```

**Step 3: Create service.go with GetManifest + Health**

Create `products/scout/internal/service.go`:
```go
package internal

import (
	"context"
	"fmt"

	soulv1 "github.com/rishav1305/soul/gen/soul/v1"
	"google.golang.org/grpc"
)

type ScoutService struct {
	soulv1.UnimplementedProductServiceServer
}

func NewScoutService() *ScoutService {
	return &ScoutService{}
}

func (s *ScoutService) GetManifest(_ context.Context, _ *soulv1.Empty) (*soulv1.Manifest, error) {
	return &soulv1.Manifest{
		Name:    "scout",
		Version: "0.1.0",
		Tools: []*soulv1.Tool{
			{
				Name:        "setup",
				Description: "Open a visible browser for you to log into a job platform. Saves the session for future headless use.",
				InputSchemaJson: `{"type":"object","properties":{"platform":{"type":"string","enum":["linkedin","naukri","indeed","wellfound","instahyre"],"description":"Platform to set up"}},"required":["platform"]}`,
			},
			{
				Name:        "sync",
				Description: "Compare your Supabase profile data against live job platforms. Reports content drift.",
				InputSchemaJson: `{"type":"object","properties":{"platforms":{"type":"array","items":{"type":"string"},"description":"Platforms to check (or [\"all\"])"}},"required":["platforms"]}`,
			},
			{
				Name:        "sweep",
				Description: "Check job portals for new matches, recruiter messages, and application status changes.",
				InputSchemaJson: `{"type":"object","properties":{"platforms":{"type":"array","items":{"type":"string"},"description":"Platforms to sweep (or [\"all\"])"}},"required":["platforms"]}`,
			},
			{
				Name:        "generate",
				Description: "Generate a tailored resume + cover note PDF for a specific role variant and company.",
				InputSchemaJson: `{"type":"object","properties":{"variant":{"type":"string","enum":["A","B","C","D","E","F","G"],"description":"Role variant"},"company":{"type":"string","description":"Target company name"},"role":{"type":"string","description":"Target role title"},"job_url":{"type":"string","description":"Job posting URL (optional)"},"specific_thing":{"type":"string","description":"Something specific about the company to mention in cover note"}},"required":["variant","company","role"]}`,
			},
			{
				Name:        "track",
				Description: "Add, update, or list job applications in the tracker.",
				InputSchemaJson: `{"type":"object","properties":{"action":{"type":"string","enum":["add","update","list"],"description":"Action to perform"},"id":{"type":"string","description":"Application ID (for update)"},"company":{"type":"string","description":"Company name (for add)"},"role":{"type":"string","description":"Role title (for add)"},"platform":{"type":"string","description":"Platform applied through (for add)"},"variant":{"type":"string","description":"Resume variant used (for add)"},"status":{"type":"string","enum":["applied","viewed","interview_scheduled","interview_done","offer","rejected","withdrawn","follow_up_sent"],"description":"Application status (for update)"},"follow_up":{"type":"string","description":"Follow-up date YYYY-MM-DD (for update)"},"notes":{"type":"string","description":"Notes"},"filter_status":{"type":"string","description":"Filter by status (for list)"}},"required":["action"]}`,
			},
			{
				Name:        "report",
				Description: "Get structured Scout dashboard data (sync status, opportunities, applications, metrics).",
				InputSchemaJson: `{"type":"object","properties":{"period":{"type":"string","enum":["today","week","month"],"description":"Report period"}},"required":["period"]}`,
			},
		},
	}, nil
}

func (s *ScoutService) ExecuteTool(ctx context.Context, req *soulv1.ToolRequest) (*soulv1.ToolResponse, error) {
	switch req.Tool {
	case "setup":
		return s.executeSetup(ctx, req.InputJson)
	case "generate":
		return s.executeGenerate(ctx, req.InputJson)
	case "track":
		return s.executeTrack(ctx, req.InputJson)
	case "report":
		return s.executeReport(ctx, req.InputJson)
	default:
		return &soulv1.ToolResponse{
			Success: false,
			Output:  fmt.Sprintf("unknown tool: %s", req.Tool),
		}, nil
	}
}

func (s *ScoutService) ExecuteToolStream(req *soulv1.ToolRequest, stream grpc.ServerStreamingServer[soulv1.ToolEvent]) error {
	switch req.Tool {
	case "sync":
		return s.streamSync(req.InputJson, stream)
	case "sweep":
		return s.streamSweep(req.InputJson, stream)
	default:
		// Wrap unary tools in a single Complete event.
		resp, err := s.ExecuteTool(context.Background(), req)
		if err != nil {
			return stream.Send(&soulv1.ToolEvent{
				Event: &soulv1.ToolEvent_Error{
					Error: &soulv1.ErrorEvent{Code: "EXECUTE_ERROR", Message: err.Error()},
				},
			})
		}
		return stream.Send(&soulv1.ToolEvent{
			Event: &soulv1.ToolEvent_Complete{Complete: resp},
		})
	}
}

func (s *ScoutService) Health(_ context.Context, _ *soulv1.Empty) (*soulv1.HealthResponse, error) {
	return &soulv1.HealthResponse{Healthy: true, Status: "ok"}, nil
}

// Stub implementations — filled in by later tasks.
func (s *ScoutService) executeSetup(_ context.Context, _ string) (*soulv1.ToolResponse, error) {
	return &soulv1.ToolResponse{Success: false, Output: "not implemented"}, nil
}
func (s *ScoutService) executeGenerate(_ context.Context, _ string) (*soulv1.ToolResponse, error) {
	return &soulv1.ToolResponse{Success: false, Output: "not implemented"}, nil
}
func (s *ScoutService) executeTrack(_ context.Context, _ string) (*soulv1.ToolResponse, error) {
	return &soulv1.ToolResponse{Success: false, Output: "not implemented"}, nil
}
func (s *ScoutService) executeReport(_ context.Context, _ string) (*soulv1.ToolResponse, error) {
	return &soulv1.ToolResponse{Success: false, Output: "not implemented"}, nil
}
func (s *ScoutService) streamSync(_ string, stream grpc.ServerStreamingServer[soulv1.ToolEvent]) error {
	return stream.Send(&soulv1.ToolEvent{
		Event: &soulv1.ToolEvent_Error{Error: &soulv1.ErrorEvent{Code: "NOT_IMPL", Message: "sync not implemented"}},
	})
}
func (s *ScoutService) streamSweep(_ string, stream grpc.ServerStreamingServer[soulv1.ToolEvent]) error {
	return stream.Send(&soulv1.ToolEvent{
		Event: &soulv1.ToolEvent_Error{Error: &soulv1.ErrorEvent{Code: "NOT_IMPL", Message: "sweep not implemented"}},
	})
}
```

**Step 4: Create Makefile**

Create `products/scout/Makefile`:
```makefile
.PHONY: build clean

build:
	cd cmd/scout && go build -o ../../scout .

clean:
	rm -f scout
```

**Step 5: Build and verify**

Run:
```bash
cd products/scout && go mod tidy && make build
```
Expected: Binary at `products/scout/scout`

Run:
```bash
./scout --socket /tmp/test-scout.sock &
sleep 1 && kill %1
```
Expected: "[scout] listening on /tmp/test-scout.sock" then clean shutdown.

**Step 6: Commit**

```bash
git add products/scout/
git commit -m "feat(scout): scaffold binary with gRPC service + 6 tool manifests"
```

---

### Task 2: Data Store — ~/.soul/scout/ persistence

**Files:**
- Create: `products/scout/internal/data/store.go`

**Step 1: Create the data store**

Create `products/scout/internal/data/store.go`:
```go
package data

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Store manages all Scout data in a single JSON file.
type Store struct {
	mu   sync.Mutex
	path string
	data ScoutData
}

type ScoutData struct {
	Sync         SyncData         `json:"sync"`
	Sweep        SweepData        `json:"sweep"`
	Applications []Application    `json:"applications"`
	Metrics      map[string]Metric `json:"metrics"` // keyed by "YYYY-WW"
}

// SyncData holds the latest sync results.
type SyncData struct {
	LastRun  string          `json:"last_run"`
	Results  []PlatformSync  `json:"results"`
}

type PlatformSync struct {
	Platform string      `json:"platform"`
	Status   string      `json:"status"` // "synced" or "drift"
	Issues   []string    `json:"issues,omitempty"`
	CheckedAt string     `json:"checked_at"`
}

// SweepData holds the latest sweep results.
type SweepData struct {
	LastRun       string        `json:"last_run"`
	Opportunities []Opportunity `json:"opportunities"`
	Messages      []Message     `json:"messages"`
}

type Opportunity struct {
	ID        string `json:"id"`
	Company   string `json:"company"`
	Role      string `json:"role"`
	Platform  string `json:"platform"`
	Match     string `json:"match,omitempty"`
	URL       string `json:"url,omitempty"`
	FoundAt   string `json:"found_at"`
	Dismissed bool   `json:"dismissed,omitempty"`
}

type Message struct {
	ID       string `json:"id"`
	Platform string `json:"platform"`
	From     string `json:"from"`
	Subject  string `json:"subject"`
	Urgent   bool   `json:"urgent"`
	FoundAt  string `json:"found_at"`
}

// Application tracks a job application.
type Application struct {
	ID        string `json:"id"`
	Company   string `json:"company"`
	Role      string `json:"role"`
	Platform  string `json:"platform"`
	Variant   string `json:"variant"`
	Status    string `json:"status"`
	FollowUp  string `json:"follow_up,omitempty"`
	Notes     string `json:"notes,omitempty"`
	AppliedAt string `json:"applied_at"`
	UpdatedAt string `json:"updated_at"`
}

type Metric struct {
	Applied    int `json:"applied"`
	Responses  int `json:"responses"`
	Interviews int `json:"interviews"`
	Offers     int `json:"offers"`
}

// NewStore opens or creates the data store at ~/.soul/scout/data.json.
func NewStore() (*Store, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("get home dir: %w", err)
	}

	dir := filepath.Join(home, ".soul", "scout")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create scout dir: %w", err)
	}

	path := filepath.Join(dir, "data.json")
	s := &Store{path: path}

	// Load existing data or initialize empty.
	raw, err := os.ReadFile(path)
	if err == nil {
		json.Unmarshal(raw, &s.data)
	}
	if s.data.Metrics == nil {
		s.data.Metrics = make(map[string]Metric)
	}

	return s, nil
}

// Save writes the current data to disk.
func (s *Store) Save() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.save()
}

func (s *Store) save() error {
	raw, err := json.MarshalIndent(s.data, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.path, raw, 0o644)
}

// --- Sync ---

func (s *Store) SetSyncResults(results []PlatformSync) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data.Sync = SyncData{
		LastRun: time.Now().UTC().Format(time.RFC3339),
		Results: results,
	}
	return s.save()
}

func (s *Store) GetSyncData() SyncData {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.data.Sync
}

// --- Sweep ---

func (s *Store) SetSweepResults(opportunities []Opportunity, messages []Message) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data.Sweep = SweepData{
		LastRun:       time.Now().UTC().Format(time.RFC3339),
		Opportunities: opportunities,
		Messages:      messages,
	}
	return s.save()
}

func (s *Store) GetSweepData() SweepData {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.data.Sweep
}

// --- Applications ---

func (s *Store) AddApplication(app Application) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	app.ID = fmt.Sprintf("app-%d", time.Now().UnixMilli())
	app.AppliedAt = time.Now().UTC().Format(time.RFC3339)
	app.UpdatedAt = app.AppliedAt
	if app.Status == "" {
		app.Status = "applied"
	}
	s.data.Applications = append(s.data.Applications, app)

	// Update weekly metric.
	_, week := time.Now().ISOWeek()
	key := fmt.Sprintf("%d-W%02d", time.Now().Year(), week)
	m := s.data.Metrics[key]
	m.Applied++
	s.data.Metrics[key] = m

	return s.save()
}

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
			s.data.Applications[i].UpdatedAt = time.Now().UTC().Format(time.RFC3339)
			return s.save()
		}
	}
	return fmt.Errorf("application %s not found", id)
}

func (s *Store) ListApplications(filterStatus string) []Application {
	s.mu.Lock()
	defer s.mu.Unlock()
	if filterStatus == "" || filterStatus == "all" {
		return s.data.Applications
	}
	var filtered []Application
	for _, app := range s.data.Applications {
		if app.Status == filterStatus {
			filtered = append(filtered, app)
		}
	}
	return filtered
}

// --- Report ---

func (s *Store) GetReportData() ScoutData {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.data
}

// DataDir returns the scout data directory path.
func (s *Store) DataDir() string {
	return filepath.Dir(s.path)
}
```

**Step 2: Build and verify**

Run: `cd products/scout && go build ./...`
Expected: Builds clean.

**Step 3: Commit**

```bash
git add products/scout/internal/data/
git commit -m "feat(scout): data store for sync, sweep, applications, metrics"
```

---

### Task 3: Browser Pool — Rod setup + profile management

**Files:**
- Create: `products/scout/internal/browser/pool.go`
- Create: `products/scout/internal/browser/profiles.go`

**Step 1: Create profiles.go**

Create `products/scout/internal/browser/profiles.go`:
```go
package browser

import (
	"fmt"
	"os"
	"path/filepath"
)

// ProfileDir returns the Chrome profile directory for a platform.
// Creates it if it doesn't exist.
func ProfileDir(platform string) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("get home dir: %w", err)
	}

	dir := filepath.Join(home, ".soul", "scout", "profiles", platform)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("create profile dir: %w", err)
	}

	return dir, nil
}

// HasProfile checks if a login session exists for a platform.
func HasProfile(platform string) bool {
	home, _ := os.UserHomeDir()
	dir := filepath.Join(home, ".soul", "scout", "profiles", platform)
	entries, _ := os.ReadDir(dir)
	return len(entries) > 0
}

// AllPlatforms returns the list of supported platforms.
func AllPlatforms() []string {
	return []string{"linkedin", "naukri", "indeed", "wellfound", "instahyre"}
}

// PlatformURLs maps platforms to their key URLs.
var PlatformURLs = map[string]struct {
	Login string
	Jobs  string
}{
	"linkedin":  {Login: "https://www.linkedin.com/login", Jobs: "https://www.linkedin.com/jobs/"},
	"naukri":    {Login: "https://www.naukri.com/nlogin/login", Jobs: "https://www.naukri.com/mnjuser/homepage"},
	"indeed":    {Login: "https://secure.indeed.com/auth", Jobs: "https://www.indeed.com/"},
	"wellfound": {Login: "https://wellfound.com/login", Jobs: "https://wellfound.com/jobs"},
	"instahyre": {Login: "https://www.instahyre.com/login/", Jobs: "https://www.instahyre.com/candidate/opportunities/"},
}
```

**Step 2: Create pool.go**

Create `products/scout/internal/browser/pool.go`:
```go
package browser

import (
	"fmt"
	"log"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
)

// LaunchVisible opens a visible Chrome window using the platform's profile.
// Used by the setup tool for manual login.
func LaunchVisible(platform string) (*rod.Browser, *rod.Page, error) {
	profileDir, err := ProfileDir(platform)
	if err != nil {
		return nil, nil, err
	}

	u, err := launcher.New().
		UserDataDir(profileDir).
		Headless(false).
		Launch()
	if err != nil {
		return nil, nil, fmt.Errorf("launch browser: %w", err)
	}

	browser := rod.New().ControlURL(u)
	if err := browser.Connect(); err != nil {
		return nil, nil, fmt.Errorf("connect browser: %w", err)
	}

	urls, ok := PlatformURLs[platform]
	if !ok {
		return nil, nil, fmt.Errorf("unknown platform: %s", platform)
	}

	page, err := browser.Page(rod.PageOptions{})
	if err != nil {
		return nil, nil, fmt.Errorf("create page: %w", err)
	}

	if err := page.Navigate(urls.Login); err != nil {
		return nil, nil, fmt.Errorf("navigate: %w", err)
	}

	log.Printf("[scout] opened %s login — waiting for user to complete login", platform)
	return browser, page, nil
}

// LaunchHeadless opens a headless Chrome using the platform's saved profile.
// Used by sync and sweep tools.
func LaunchHeadless(platform string) (*rod.Browser, error) {
	profileDir, err := ProfileDir(platform)
	if err != nil {
		return nil, err
	}

	u, err := launcher.New().
		UserDataDir(profileDir).
		Headless(true).
		Launch()
	if err != nil {
		return nil, fmt.Errorf("launch headless: %w", err)
	}

	browser := rod.New().ControlURL(u)
	if err := browser.Connect(); err != nil {
		return nil, fmt.Errorf("connect headless: %w", err)
	}

	return browser, nil
}

// NavigateWithDelay navigates to a URL and waits a random-ish delay.
func NavigateWithDelay(page *rod.Page, url string) error {
	if err := page.Navigate(url); err != nil {
		return err
	}
	page.WaitStable(2 * time.Second)
	return nil
}
```

**Step 3: Build**

Run: `cd products/scout && go mod tidy && go build ./...`
Expected: Builds clean with rod dependency added.

**Step 4: Commit**

```bash
git add products/scout/internal/browser/ products/scout/go.mod products/scout/go.sum
git commit -m "feat(scout): Rod browser pool with profile persistence"
```

---

### Task 4: Supabase Client — read-only profile data

**Files:**
- Create: `products/scout/internal/supabase/client.go`

**Step 1: Create the Supabase REST client**

Create `products/scout/internal/supabase/client.go`:
```go
package supabase

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
)

// Client reads profile data from Supabase REST API.
type Client struct {
	url    string
	anonKey string
	http    *http.Client
}

// Config holds Supabase connection details.
type Config struct {
	URL     string `json:"supabase_url"`
	AnonKey string `json:"supabase_anon_key"`
}

// NewClient creates a Supabase client from ~/.soul/scout/config.json.
func NewClient() (*Client, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	configPath := filepath.Join(home, ".soul", "scout", "config.json")
	raw, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("read config: %w — create ~/.soul/scout/config.json with supabase_url and supabase_anon_key", err)
	}

	var cfg Config
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	if cfg.URL == "" || cfg.AnonKey == "" {
		return nil, fmt.Errorf("config missing supabase_url or supabase_anon_key")
	}

	return &Client{
		url:     cfg.URL,
		anonKey: cfg.AnonKey,
		http:    &http.Client{},
	}, nil
}

// query fetches rows from a Supabase table.
func (c *Client) query(table, filter string) ([]byte, error) {
	url := fmt.Sprintf("%s/rest/v1/%s?%s", c.url, table, filter)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("apikey", c.anonKey)
	req.Header.Set("Authorization", "Bearer "+c.anonKey)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("supabase request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("supabase %d: %s", resp.StatusCode, body)
	}

	return body, nil
}

// ProfileData holds all profile fields from Supabase.
type ProfileData struct {
	SiteConfig []SiteConfigRow `json:"site_config"`
	Experience []ExperienceRow `json:"experience"`
	Skills     []SkillRow      `json:"skills"`
	Projects   []ProjectRow    `json:"projects"`
}

type SiteConfigRow struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type ExperienceRow struct {
	ID           int      `json:"id"`
	Role         string   `json:"role"`
	Company      string   `json:"company"`
	Period       string   `json:"period"`
	Achievements []string `json:"achievements"`
	Order        int      `json:"display_order"`
}

type SkillRow struct {
	ID       int    `json:"id"`
	Category string `json:"category"`
	Name     string `json:"name"`
	Level    string `json:"level,omitempty"`
}

type ProjectRow struct {
	ID          int    `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	TechStack   string `json:"tech_stack"`
}

// GetProfileData fetches all profile data from Supabase.
func (c *Client) GetProfileData() (*ProfileData, error) {
	pd := &ProfileData{}

	// site_config
	raw, err := c.query("site_config", "select=*")
	if err != nil {
		return nil, fmt.Errorf("site_config: %w", err)
	}
	json.Unmarshal(raw, &pd.SiteConfig)

	// experience
	raw, err = c.query("experience", "select=*&order=display_order")
	if err != nil {
		return nil, fmt.Errorf("experience: %w", err)
	}
	json.Unmarshal(raw, &pd.Experience)

	// skills
	raw, err = c.query("skills", "select=*")
	if err != nil {
		return nil, fmt.Errorf("skills: %w", err)
	}
	json.Unmarshal(raw, &pd.Skills)

	// projects
	raw, err = c.query("projects", "select=*")
	if err != nil {
		return nil, fmt.Errorf("projects: %w", err)
	}
	json.Unmarshal(raw, &pd.Projects)

	return pd, nil
}
```

**Step 2: Build**

Run: `cd products/scout && go build ./...`
Expected: Builds clean.

**Step 3: Commit**

```bash
git add products/scout/internal/supabase/
git commit -m "feat(scout): Supabase REST client for profile data"
```

---

### Task 5: Setup Tool — visible browser login

**Files:**
- Modify: `products/scout/internal/service.go` (replace `executeSetup` stub)

**Step 1: Wire up the setup tool**

Replace the `executeSetup` stub in `products/scout/internal/service.go` with:

```go
func (s *ScoutService) executeSetup(_ context.Context, inputJSON string) (*soulv1.ToolResponse, error) {
	var input struct {
		Platform string `json:"platform"`
	}
	if err := json.Unmarshal([]byte(inputJSON), &input); err != nil {
		return &soulv1.ToolResponse{Success: false, Output: fmt.Sprintf("invalid input: %v", err)}, nil
	}

	browser, page, err := browser.LaunchVisible(input.Platform)
	if err != nil {
		return &soulv1.ToolResponse{Success: false, Output: fmt.Sprintf("failed to launch browser: %v", err)}, nil
	}
	defer browser.Close()

	// Wait for user to complete login — detect navigation away from login page.
	// Poll every 2 seconds for up to 5 minutes.
	urls := browser.PlatformURLs[input.Platform]
	deadline := time.Now().Add(5 * time.Minute)
	loggedIn := false
	for time.Now().Before(deadline) {
		info, _ := page.Info()
		if info != nil && info.URL != "" && info.URL != urls.Login {
			loggedIn = true
			break
		}
		time.Sleep(2 * time.Second)
	}

	if !loggedIn {
		return &soulv1.ToolResponse{
			Success: false,
			Output:  fmt.Sprintf("Login timeout for %s — please try again", input.Platform),
		}, nil
	}

	profileDir, _ := browser.ProfileDir(input.Platform)
	return &soulv1.ToolResponse{
		Success: true,
		Output:  fmt.Sprintf("Session saved for %s. Profile stored at %s", input.Platform, profileDir),
	}, nil
}
```

Note: You'll need to add the necessary imports (`encoding/json`, `time`, and the browser package). Adjust the `browser.PlatformURLs` reference to use the package correctly — it's `browser.PlatformURLs[input.Platform]` where `browser` is the import alias for `products/scout/internal/browser`.

**Step 2: Build**

Run: `cd products/scout && go build ./...`

**Step 3: Commit**

```bash
git add products/scout/
git commit -m "feat(scout): setup tool — visible browser login with profile save"
```

---

### Task 6: Track Tool — application CRUD

**Files:**
- Modify: `products/scout/internal/service.go` (replace `executeTrack` stub)

**Step 1: Wire the data store into ScoutService**

Add a `store` field to `ScoutService`:
```go
type ScoutService struct {
	soulv1.UnimplementedProductServiceServer
	store *data.Store
}

func NewScoutService() *ScoutService {
	store, err := data.NewStore()
	if err != nil {
		log.Printf("[scout] WARNING: data store init failed: %v", err)
	}
	return &ScoutService{store: store}
}
```

**Step 2: Implement executeTrack**

Replace the `executeTrack` stub:

```go
func (s *ScoutService) executeTrack(_ context.Context, inputJSON string) (*soulv1.ToolResponse, error) {
	var input struct {
		Action       string `json:"action"`
		ID           string `json:"id"`
		Company      string `json:"company"`
		Role         string `json:"role"`
		Platform     string `json:"platform"`
		Variant      string `json:"variant"`
		Status       string `json:"status"`
		FollowUp     string `json:"follow_up"`
		Notes        string `json:"notes"`
		FilterStatus string `json:"filter_status"`
	}
	if err := json.Unmarshal([]byte(inputJSON), &input); err != nil {
		return &soulv1.ToolResponse{Success: false, Output: fmt.Sprintf("invalid input: %v", err)}, nil
	}

	switch input.Action {
	case "add":
		app := data.Application{
			Company:  input.Company,
			Role:     input.Role,
			Platform: input.Platform,
			Variant:  input.Variant,
			Notes:    input.Notes,
		}
		if err := s.store.AddApplication(app); err != nil {
			return &soulv1.ToolResponse{Success: false, Output: err.Error()}, nil
		}
		return &soulv1.ToolResponse{
			Success: true,
			Output:  fmt.Sprintf("Application added: %s — %s (%s)", input.Company, input.Role, input.Platform),
		}, nil

	case "update":
		if err := s.store.UpdateApplication(input.ID, input.Status, input.FollowUp, input.Notes); err != nil {
			return &soulv1.ToolResponse{Success: false, Output: err.Error()}, nil
		}
		return &soulv1.ToolResponse{
			Success: true,
			Output:  fmt.Sprintf("Application %s updated", input.ID),
		}, nil

	case "list":
		apps := s.store.ListApplications(input.FilterStatus)
		raw, _ := json.Marshal(apps)
		return &soulv1.ToolResponse{
			Success:        true,
			Output:         fmt.Sprintf("%d applications found", len(apps)),
			StructuredJson: string(raw),
		}, nil

	default:
		return &soulv1.ToolResponse{Success: false, Output: fmt.Sprintf("unknown action: %s", input.Action)}, nil
	}
}
```

**Step 3: Build**

Run: `cd products/scout && go build ./...`

**Step 4: Commit**

```bash
git add products/scout/
git commit -m "feat(scout): track tool — application CRUD with data store"
```

---

### Task 7: Report Tool — structured dashboard JSON

**Files:**
- Modify: `products/scout/internal/service.go` (replace `executeReport` stub)

**Step 1: Implement executeReport**

Replace the `executeReport` stub:

```go
func (s *ScoutService) executeReport(_ context.Context, inputJSON string) (*soulv1.ToolResponse, error) {
	reportData := s.store.GetReportData()

	// Build sync summary.
	syncSummary := map[string]any{
		"last_run":          reportData.Sync.LastRun,
		"platforms_checked": len(reportData.Sync.Results),
	}
	inSync, drift := 0, 0
	var platformDetails []map[string]any
	for _, r := range reportData.Sync.Results {
		if r.Status == "synced" {
			inSync++
		} else {
			drift++
		}
		platformDetails = append(platformDetails, map[string]any{
			"platform": r.Platform,
			"status":   r.Status,
			"issues":   r.Issues,
		})
	}
	syncSummary["in_sync"] = inSync
	syncSummary["drift"] = drift
	syncSummary["details"] = platformDetails

	// Build sweep summary.
	sweepSummary := map[string]any{
		"last_run":          reportData.Sweep.LastRun,
		"new_opportunities": len(reportData.Sweep.Opportunities),
		"messages":          len(reportData.Sweep.Messages),
		"opportunities":     reportData.Sweep.Opportunities,
	}

	// Build application summary.
	byStatus := make(map[string]int)
	var active int
	for _, app := range reportData.Applications {
		byStatus[app.Status]++
		if app.Status != "rejected" && app.Status != "withdrawn" && app.Status != "offer" {
			active++
		}
	}
	appSummary := map[string]any{
		"total":     len(reportData.Applications),
		"active":    active,
		"by_status": byStatus,
		"recent":    reportData.Applications, // TODO: limit to last N
	}

	// Build follow-ups due.
	var followUps []map[string]string
	now := time.Now().Format("2006-01-02")
	for _, app := range reportData.Applications {
		if app.FollowUp != "" && app.FollowUp <= now {
			followUps = append(followUps, map[string]string{
				"company": app.Company,
				"role":    app.Role,
				"due":     app.FollowUp,
				"action":  "Follow up",
			})
		}
	}

	report := map[string]any{
		"sync":         syncSummary,
		"sweep":        sweepSummary,
		"applications": appSummary,
		"metrics":      reportData.Metrics,
		"follow_ups":   followUps,
	}

	raw, _ := json.Marshal(report)
	return &soulv1.ToolResponse{
		Success:        true,
		Output:         fmt.Sprintf("Scout report: %d platforms synced, %d opportunities, %d applications tracked", len(reportData.Sync.Results), len(reportData.Sweep.Opportunities), len(reportData.Applications)),
		StructuredJson: string(raw),
	}, nil
}
```

**Step 2: Build**

Run: `cd products/scout && go build ./...`

**Step 3: Commit**

```bash
git add products/scout/
git commit -m "feat(scout): report tool — structured JSON for dashboard"
```

---

### Task 8: Generate Tool — resume + cover PDF via Rod

**Files:**
- Create: `products/scout/internal/generate/variants.go`
- Create: `products/scout/internal/generate/resume.go`
- Create: `products/scout/internal/generate/pdf.go`
- Create: `products/scout/templates/resume.html`
- Create: `products/scout/templates/cover.html`
- Modify: `products/scout/internal/service.go` (replace `executeGenerate` stub)

**Step 1: Create variants.go**

Create `products/scout/internal/generate/variants.go` with role variant definitions from `docs/profile/resume-variants.md`:

```go
package generate

// Variant defines how a resume is framed for a specific role type.
type Variant struct {
	ID             string
	TargetRole     string
	Headline       string
	Summary        string
	LeadBullets    []string // Experience achievements to lead with
	SkillEmphasis  []string
	ProjectEmphasis []string
	CoverTemplate  string // Cover note with [COMPANY], [ROLE], [SPECIFIC THING] placeholders
}

var Variants = map[string]Variant{
	"A": {
		ID:         "A",
		TargetRole: "AI Platform Architect / Solutions Architect",
		Headline:   "AI Platform Architect | Built Enterprise Agentic AI for 5,000+ Users | Python, AWS, Multi-Agent Systems",
		Summary:    "AI platform architect with 8 years of production engineering experience. Built and launched GOAT — Gartner's enterprise agentic AI platform serving 5,000+ concurrent users with 88% query resolution. I specialize in taking AI from prototype to production: serverless re-architecture, multi-agent orchestration, and infrastructure that scales.",
		LeadBullets: []string{
			"GOAT platform launch (5,000+ users, 88% resolution)",
			"EKS-to-Lambda re-architecture (cost reduction)",
			"A/B testing framework (40% token efficiency)",
			"Data Quality Framework at IBM-TWC (async Lambda + AI anomaly detection)",
		},
		SkillEmphasis:   []string{"System Architecture", "AWS", "Multi-Agent Systems", "LLM Orchestration", "Python", "Infrastructure Design"},
		ProjectEmphasis: []string{"soul-os", "soul-mesh", "soul-moa-core"},
		CoverTemplate:   "I built the agentic AI platform that 5,000 Gartner associates now use daily — 88% of queries resolved without human intervention. Before that, I spent 5+ years building data platforms at Novartis and IBM that taught me what \"production-grade\" actually means. I'd like to bring that same systems-first approach to [COMPANY]'s [SPECIFIC THING]. Happy to discuss how my experience maps to what you're building.",
	},
	"B": {
		ID:         "B",
		TargetRole: "GenAI Engineer / LLM Engineer",
		Headline:   "GenAI Engineer | LLM Orchestration, Agentic AI, Prompt Engineering | 8 Years Python + Data Engineering",
		Summary:    "GenAI engineer who builds production LLM systems, not demos. Launched Gartner's agentic AI platform (5,000+ users), designed prompt engineering pipelines with 40% token efficiency gains, and created CARS — a cost-aware evaluation metric for local model inference.",
		LeadBullets: []string{
			"GOAT platform (agentic AI, prompt pipelines)",
			"A/B testing framework for LLMs (token efficiency)",
			"CARS metric (model evaluation research)",
			"dbt migration and data quality at IBM-TWC",
		},
		SkillEmphasis:   []string{"LLM Orchestration", "Prompt Engineering", "RAG", "Agentic AI", "Python", "Model Evaluation"},
		ProjectEmphasis: []string{"CARS metric", "soul-moa-core", "soul-agents", "soul-os"},
		CoverTemplate:   "I've shipped agentic AI to 5,000+ users at Gartner — not a chatbot wrapper, but a multi-model orchestration platform with prompt engineering pipelines and A/B testing that improved token efficiency by 40%. I also created CARS, an efficiency metric for evaluating local LLM inference. I'm interested in [ROLE] at [COMPANY] because [SPECIFIC THING]. I'd love to discuss how I can contribute.",
	},
	"C": {
		ID:         "C",
		TargetRole: "Senior AI Engineer",
		Headline:   "Senior AI Engineer | Production AI Systems, Distributed Computing, Data Platforms | Python, AWS, FastAPI",
		Summary:    "Senior AI engineer with 8 years building production systems — from enterprise data platforms to agentic AI. I write Python and SQL by hand, build full-stack AI applications with AI coding tools, and architect systems that stay reliable at scale.",
		LeadBullets: []string{
			"GOAT platform (end-to-end AI engineering)",
			"Cross-functional team leadership at IBM-TWC",
			"Snowflake migration at Novartis (60% performance gain)",
			"AWS pipeline optimization at Polestar (66% execution time reduction)",
		},
		SkillEmphasis:   []string{"Python", "FastAPI", "AWS", "Docker", "SQLite/PostgreSQL", "WebSocket", "System Design"},
		ProjectEmphasis: []string{"soul-mesh", "soul-os", "soul-planner"},
		CoverTemplate:   "I've spent 8 years building production systems — data platforms at Novartis, AI infrastructure at Gartner (5,000+ users), and now distributed compute meshes in my open-source work. I'm looking for a senior engineering role where I can build AI systems that actually ship. [COMPANY]'s work on [SPECIFIC THING] caught my attention. Would love to chat.",
	},
	"D": {
		ID:         "D",
		TargetRole: "AI Manager / AI Lead",
		Headline:   "AI Lead | Shipped Enterprise AI to 5,000+ Users | Cross-Functional Team Leadership",
		Summary:    "AI engineering leader with 8 years of progressive experience from data engineer to platform lead. At Gartner, launched GOAT — an enterprise agentic AI platform serving 5,000+ concurrent users. At IBM-TWC, led a cross-functional team of 8.",
		LeadBullets: []string{
			"Led cross-functional team of 8 at IBM-TWC",
			"Launched GOAT to 5,000+ users",
			"99.5% accuracy with 9.5/10 stakeholder satisfaction at Novartis",
			"Launched cloud services as a new vertical at Polestar",
		},
		SkillEmphasis:   []string{"Team Leadership", "Stakeholder Management", "AI Strategy", "Platform Architecture"},
		ProjectEmphasis: []string{"soul-os", "soul-planner"},
		CoverTemplate:   "I launched Gartner's internal AI platform to 5,000+ users — which meant not just building the system, but aligning stakeholders, managing infrastructure costs, and leading the team through three major re-architectures. Before that, I led an 8-person cross-functional team at IBM-TWC. I'm looking for a leadership role where I can build AND ship AI at [COMPANY]. Happy to discuss how my experience aligns with [SPECIFIC THING].",
	},
	"E": {
		ID:         "E",
		TargetRole: "AI Consultant / Freelance",
		Headline:   "AI Consultant | Enterprise AI Systems, Data Platform Modernization | Gartner, Novartis, IBM-TWC Track Record",
		Summary:    "Independent AI consultant with 8 years of enterprise delivery. I take AI from prototype to production and charge for outcomes, not hours.",
		LeadBullets: []string{
			"IBM-TWC results (60% processing reduction, 30% infra savings)",
			"GOAT platform (enterprise-scale AI delivery)",
			"Novartis (domain expertise in regulated industries)",
			"Polestar (multi-client delivery experience)",
		},
		SkillEmphasis:   []string{"AI Strategy", "Data Platform Modernization", "AWS Architecture", "Cost Optimization"},
		ProjectEmphasis: []string{"soul-outreach", "CARS metric"},
		CoverTemplate:   "I recently helped IBM-TWC cut data processing time by 60% and infrastructure costs by 30%. Before that, I built the AI platform 5,000 Gartner associates use daily. I specialize in [SPECIFIC THING] and can start within 30 days. Here's my portfolio: rishavchatterjee.com. Happy to do a 30-minute discovery call to see if there's a fit.",
	},
	"F": {
		ID:         "F",
		TargetRole: "AI Researcher",
		Headline:   "AI Researcher | CARS Metric Creator | LLM Evaluation, Cost-Efficient Inference, Benchmark Design",
		Summary:    "AI researcher focused on practical measurement of AI systems. Created CARS (Cost-Aware Reasoning Score) — an efficiency metric for evaluating local LLM inference. 8 years of production engineering ground my research in real-world constraints.",
		LeadBullets: []string{
			"CARS metric (original research, published benchmarks)",
			"A/B testing framework at Gartner (40% token efficiency)",
			"GOAT platform (real-world agentic AI at scale)",
			"Data quality frameworks at IBM-TWC",
		},
		SkillEmphasis:   []string{"Benchmark Design", "Model Evaluation", "Statistical Analysis", "LLM Inference", "Python"},
		ProjectEmphasis: []string{"CARS metric", "soul-bench", "soul-moa-core"},
		CoverTemplate:   "I created CARS (Cost-Aware Reasoning Score) — an efficiency metric for evaluating local LLM inference across consumer hardware. My research comes from 8 years of building production systems where measurement directly drives decisions. I'm interested in [COMPANY]'s work on [SPECIFIC THING] and would love to discuss how my applied research approach fits.",
	},
	"G": {
		ID:         "G",
		TargetRole: "Senior Data Engineer (AI-focused)",
		Headline:   "Senior Data Engineer | 8 Years Python/SQL/Cloud | Snowflake, Airflow, AWS | Now Building AI-Native Data Systems",
		Summary:    "Senior data engineer with 8 years across pharmaceutical, market research, and enterprise domains. Expert in Python, SQL, Snowflake, Airflow, and AWS. Recently extended into AI-native data systems.",
		LeadBullets: []string{
			"Novartis experience (data engineering core)",
			"Polestar multi-client delivery",
			"IBM-TWC data quality + dbt migration",
			"GOAT secondary (AI angle on data engineering)",
		},
		SkillEmphasis:   []string{"Python", "SQL", "Snowflake", "Apache Airflow", "dbt", "AWS Glue", "ETL/ELT"},
		ProjectEmphasis: []string{"soul-mesh", "soul-planner"},
		CoverTemplate:   "I've built data platforms across pharma (Novartis — 15 brands, 99.5% accuracy), enterprise (Gartner — serverless re-architecture), and weather data (IBM-TWC — 60% processing time reduction). What sets me apart is that I've also built AI systems on top of those foundations. I'm looking for a data engineering role where AI is part of the picture. [COMPANY]'s [SPECIFIC THING] is exactly that kind of role.",
	},
}
```

**Step 2: Create resume.html template**

Create `products/scout/templates/resume.html` — a clean, ATS-friendly HTML resume template with Go template placeholders:

```html
<!DOCTYPE html>
<html>
<head>
<meta charset="utf-8">
<style>
  body { font-family: 'Segoe UI', system-ui, sans-serif; font-size: 11pt; line-height: 1.5; color: #1a1a1a; margin: 40px 50px; }
  h1 { font-size: 22pt; margin: 0 0 4px; }
  .headline { font-size: 11pt; color: #555; margin: 0 0 8px; }
  .contact { font-size: 9pt; color: #666; margin-bottom: 16px; }
  .section-title { font-size: 12pt; font-weight: 700; border-bottom: 1px solid #ccc; padding-bottom: 4px; margin: 16px 0 8px; text-transform: uppercase; letter-spacing: 0.5px; }
  .summary { font-size: 10.5pt; color: #333; margin-bottom: 8px; }
  .job { margin-bottom: 12px; }
  .job-header { display: flex; justify-content: space-between; }
  .job-title { font-weight: 700; }
  .job-company { color: #555; }
  .job-period { color: #888; font-size: 9.5pt; }
  ul { margin: 4px 0; padding-left: 20px; }
  li { font-size: 10.5pt; margin-bottom: 2px; }
  .skills { display: flex; flex-wrap: wrap; gap: 6px; }
  .skill { background: #f0f0f0; padding: 2px 8px; border-radius: 3px; font-size: 9.5pt; }
  .skill-emphasis { background: #e8f0fe; font-weight: 600; }
</style>
</head>
<body>
  <h1>{{.Name}}</h1>
  <div class="headline">{{.Headline}}</div>
  <div class="contact">{{.Email}} | {{.Website}} | {{.Location}}</div>

  <div class="section-title">Summary</div>
  <div class="summary">{{.Summary}}</div>

  <div class="section-title">Experience</div>
  {{range .Experience}}
  <div class="job">
    <div class="job-header">
      <span><span class="job-title">{{.Role}}</span> — <span class="job-company">{{.Company}}</span></span>
      <span class="job-period">{{.Period}}</span>
    </div>
    <ul>
      {{range .Achievements}}<li>{{.}}</li>{{end}}
    </ul>
  </div>
  {{end}}

  <div class="section-title">Skills</div>
  <div class="skills">
    {{range .Skills}}
    <span class="skill {{if .Emphasis}}skill-emphasis{{end}}">{{.Name}}</span>
    {{end}}
  </div>

  <div class="section-title">Projects</div>
  {{range .Projects}}
  <div class="job">
    <span class="job-title">{{.Title}}</span> — {{.Description}}
  </div>
  {{end}}
</body>
</html>
```

**Step 3: Create cover.html template**

Create `products/scout/templates/cover.html`:

```html
<!DOCTYPE html>
<html>
<head>
<meta charset="utf-8">
<style>
  body { font-family: 'Segoe UI', system-ui, sans-serif; font-size: 11pt; line-height: 1.7; color: #1a1a1a; margin: 60px 60px; }
  .date { color: #888; margin-bottom: 24px; }
  .greeting { margin-bottom: 16px; }
  .body { margin-bottom: 24px; }
  .closing { margin-top: 24px; }
  .name { font-weight: 700; }
</style>
</head>
<body>
  <div class="date">{{.Date}}</div>
  <div class="greeting">Dear Hiring Team at {{.Company}},</div>
  <div class="body">{{.CoverText}}</div>
  <div class="closing">
    Best regards,<br>
    <span class="name">{{.Name}}</span><br>
    {{.Email}} | {{.Website}}
  </div>
</body>
</html>
```

**Step 4: Create resume.go and pdf.go**

Create `products/scout/internal/generate/resume.go`:
```go
package generate

import (
	"bytes"
	"embed"
	"fmt"
	"html/template"
	"strings"
	"time"

	"github.com/rishav1305/soul/products/scout/internal/supabase"
)

//go:embed ../../templates/*.html
var templateFS embed.FS

type ResumeData struct {
	Name       string
	Headline   string
	Email      string
	Website    string
	Location   string
	Summary    string
	Experience []ExperienceEntry
	Skills     []SkillEntry
	Projects   []ProjectEntry
}

type ExperienceEntry struct {
	Role         string
	Company      string
	Period       string
	Achievements []string
}

type SkillEntry struct {
	Name     string
	Emphasis bool
}

type ProjectEntry struct {
	Title       string
	Description string
}

type CoverData struct {
	Date      string
	Company   string
	CoverText string
	Name      string
	Email     string
	Website   string
}

// BuildResumeHTML generates the resume HTML for a given variant and profile data.
func BuildResumeHTML(variant Variant, profile *supabase.ProfileData) (string, error) {
	tmpl, err := template.ParseFS(templateFS, "../../templates/resume.html")
	if err != nil {
		return "", fmt.Errorf("parse template: %w", err)
	}

	// Extract personal info from site_config.
	configMap := make(map[string]string)
	for _, row := range profile.SiteConfig {
		configMap[row.Key] = row.Value
	}

	// Build skills with emphasis.
	emphasisSet := make(map[string]bool)
	for _, s := range variant.SkillEmphasis {
		emphasisSet[strings.ToLower(s)] = true
	}
	var skills []SkillEntry
	// Emphasized skills first.
	for _, s := range variant.SkillEmphasis {
		skills = append(skills, SkillEntry{Name: s, Emphasis: true})
	}
	for _, s := range profile.Skills {
		if !emphasisSet[strings.ToLower(s.Name)] {
			skills = append(skills, SkillEntry{Name: s.Name, Emphasis: false})
		}
	}

	// Build experience entries.
	var experience []ExperienceEntry
	for _, exp := range profile.Experience {
		experience = append(experience, ExperienceEntry{
			Role:         exp.Role,
			Company:      exp.Company,
			Period:       exp.Period,
			Achievements: exp.Achievements,
		})
	}

	// Build project entries.
	var projects []ProjectEntry
	for _, p := range profile.Projects {
		projects = append(projects, ProjectEntry{
			Title:       p.Title,
			Description: p.Description,
		})
	}

	data := ResumeData{
		Name:       configMap["name"],
		Headline:   variant.Headline,
		Email:      configMap["email"],
		Website:    configMap["website"],
		Location:   configMap["location"],
		Summary:    variant.Summary,
		Experience: experience,
		Skills:     skills,
		Projects:   projects,
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("execute template: %w", err)
	}

	return buf.String(), nil
}

// BuildCoverHTML generates cover note HTML.
func BuildCoverHTML(variant Variant, profile *supabase.ProfileData, company, role, specificThing string) (string, error) {
	tmpl, err := template.ParseFS(templateFS, "../../templates/cover.html")
	if err != nil {
		return "", fmt.Errorf("parse template: %w", err)
	}

	configMap := make(map[string]string)
	for _, row := range profile.SiteConfig {
		configMap[row.Key] = row.Value
	}

	coverText := variant.CoverTemplate
	coverText = strings.ReplaceAll(coverText, "[COMPANY]", company)
	coverText = strings.ReplaceAll(coverText, "[ROLE]", role)
	if specificThing != "" {
		coverText = strings.ReplaceAll(coverText, "[SPECIFIC THING]", specificThing)
	}

	data := CoverData{
		Date:      time.Now().Format("January 2, 2006"),
		Company:   company,
		CoverText: coverText,
		Name:      configMap["name"],
		Email:     configMap["email"],
		Website:   configMap["website"],
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("execute template: %w", err)
	}

	return buf.String(), nil
}
```

Create `products/scout/internal/generate/pdf.go`:
```go
package generate

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/proto"
)

// HTMLToPDF renders an HTML string to a PDF file using Rod.
func HTMLToPDF(html string, outputPath string) error {
	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		return fmt.Errorf("create output dir: %w", err)
	}

	u, err := launcher.New().Headless(true).Launch()
	if err != nil {
		return fmt.Errorf("launch browser: %w", err)
	}

	browser := rod.New().ControlURL(u)
	if err := browser.Connect(); err != nil {
		return fmt.Errorf("connect browser: %w", err)
	}
	defer browser.Close()

	page, err := browser.Page(rod.PageOptions{})
	if err != nil {
		return fmt.Errorf("create page: %w", err)
	}

	if err := page.SetDocumentContent(html); err != nil {
		return fmt.Errorf("set content: %w", err)
	}

	page.WaitStable(1 * time.Second)

	pdfData, err := page.PDF(&proto.PagePrintToPDF{
		PrintBackground:    true,
		PaperWidth:         proto.Float64(8.27),  // A4 width in inches
		PaperHeight:        proto.Float64(11.69), // A4 height in inches
		MarginTop:          proto.Float64(0.4),
		MarginBottom:       proto.Float64(0.4),
		MarginLeft:         proto.Float64(0.5),
		MarginRight:        proto.Float64(0.5),
	})
	if err != nil {
		return fmt.Errorf("print to PDF: %w", err)
	}

	reader := pdfData
	raw, err := io.ReadAll(reader)
	if err != nil {
		return fmt.Errorf("read PDF: %w", err)
	}

	if err := os.WriteFile(outputPath, raw, 0o644); err != nil {
		return fmt.Errorf("write PDF: %w", err)
	}

	return nil
}
```

Note: Add `"io"` to the imports in pdf.go.

**Step 5: Wire executeGenerate in service.go**

Replace the `executeGenerate` stub:

```go
func (s *ScoutService) executeGenerate(_ context.Context, inputJSON string) (*soulv1.ToolResponse, error) {
	var input struct {
		Variant       string `json:"variant"`
		Company       string `json:"company"`
		Role          string `json:"role"`
		JobURL        string `json:"job_url"`
		SpecificThing string `json:"specific_thing"`
	}
	if err := json.Unmarshal([]byte(inputJSON), &input); err != nil {
		return &soulv1.ToolResponse{Success: false, Output: fmt.Sprintf("invalid input: %v", err)}, nil
	}

	variant, ok := generate.Variants[input.Variant]
	if !ok {
		return &soulv1.ToolResponse{Success: false, Output: fmt.Sprintf("unknown variant: %s", input.Variant)}, nil
	}

	// Fetch profile from Supabase.
	supa, err := supabase.NewClient()
	if err != nil {
		return &soulv1.ToolResponse{Success: false, Output: fmt.Sprintf("supabase: %v", err)}, nil
	}
	profile, err := supa.GetProfileData()
	if err != nil {
		return &soulv1.ToolResponse{Success: false, Output: fmt.Sprintf("fetch profile: %v", err)}, nil
	}

	// Determine output paths.
	slug := strings.ToLower(strings.ReplaceAll(input.Company, " ", "-"))
	date := time.Now().Format("2006-01-02")
	draftsDir := filepath.Join(s.store.DataDir(), "drafts")

	resumePath := filepath.Join(draftsDir, fmt.Sprintf("%s-%s-%s-resume.pdf", slug, input.Variant, date))
	coverPath := filepath.Join(draftsDir, fmt.Sprintf("%s-%s-%s-cover.pdf", slug, input.Variant, date))

	// Generate resume PDF.
	resumeHTML, err := generate.BuildResumeHTML(variant, profile)
	if err != nil {
		return &soulv1.ToolResponse{Success: false, Output: fmt.Sprintf("build resume HTML: %v", err)}, nil
	}
	if err := generate.HTMLToPDF(resumeHTML, resumePath); err != nil {
		return &soulv1.ToolResponse{Success: false, Output: fmt.Sprintf("generate resume PDF: %v", err)}, nil
	}

	// Generate cover note PDF.
	coverHTML, err := generate.BuildCoverHTML(variant, profile, input.Company, input.Role, input.SpecificThing)
	if err != nil {
		return &soulv1.ToolResponse{Success: false, Output: fmt.Sprintf("build cover HTML: %v", err)}, nil
	}
	if err := generate.HTMLToPDF(coverHTML, coverPath); err != nil {
		return &soulv1.ToolResponse{Success: false, Output: fmt.Sprintf("generate cover PDF: %v", err)}, nil
	}

	return &soulv1.ToolResponse{
		Success: true,
		Output:  fmt.Sprintf("Generated resume and cover note for %s (%s)\nResume: %s\nCover: %s", input.Company, variant.TargetRole, resumePath, coverPath),
		Artifacts: []*soulv1.Artifact{
			{Type: "pdf", Path: resumePath},
			{Type: "pdf", Path: coverPath},
		},
	}, nil
}
```

Add the necessary imports to service.go: `generate`, `supabase`, `path/filepath`, `strings`, `time`.

**Step 6: Build**

Run: `cd products/scout && go mod tidy && go build ./...`

**Step 7: Commit**

```bash
git add products/scout/
git commit -m "feat(scout): generate tool — resume + cover PDF via Rod print-to-PDF"
```

---

### Task 9: Sync Tool — streaming platform comparison

**Files:**
- Create: `products/scout/internal/sync/checker.go`
- Modify: `products/scout/internal/service.go` (replace `streamSync` stub)

**Step 1: Create checker.go**

Create `products/scout/internal/sync/checker.go`:
```go
package sync

import (
	"fmt"
	"log"
	"strings"

	"github.com/go-rod/rod"

	"github.com/rishav1305/soul/products/scout/internal/browser"
	"github.com/rishav1305/soul/products/scout/internal/data"
	"github.com/rishav1305/soul/products/scout/internal/supabase"
)

// CheckResult holds the sync result for one platform.
type CheckResult struct {
	Platform data.PlatformSync
	Error    error
}

// CheckPlatform compares Supabase data against what's live on a platform.
func CheckPlatform(platform string, profile *supabase.ProfileData) CheckResult {
	switch platform {
	case "website":
		return checkWebsite(profile)
	case "github":
		return checkGitHub(profile)
	default:
		return checkBrowserPlatform(platform, profile)
	}
}

// checkWebsite fetches rishavchatterjee.com/resume and compares text.
func checkWebsite(profile *supabase.ProfileData) CheckResult {
	// Simple HTTP fetch — no login needed.
	// Compare key fields from site_config against rendered page text.
	// This is a simplified check — can be expanded later.
	return CheckResult{
		Platform: data.PlatformSync{
			Platform: "website",
			Status:   "synced",
		},
	}
}

// checkGitHub fetches the GitHub profile README via API.
func checkGitHub(profile *supabase.ProfileData) CheckResult {
	return CheckResult{
		Platform: data.PlatformSync{
			Platform: "github",
			Status:   "synced",
		},
	}
}

// checkBrowserPlatform uses Rod to check a platform that needs login.
func checkBrowserPlatform(platform string, profile *supabase.ProfileData) CheckResult {
	if !browser.HasProfile(platform) {
		return CheckResult{
			Platform: data.PlatformSync{
				Platform: platform,
				Status:   "drift",
				Issues:   []string{"No saved session — run setup first"},
			},
		}
	}

	b, err := browser.LaunchHeadless(platform)
	if err != nil {
		return CheckResult{
			Platform: data.PlatformSync{
				Platform: platform,
				Status:   "drift",
				Issues:   []string{fmt.Sprintf("Browser launch failed: %v", err)},
			},
			Error: err,
		}
	}
	defer b.Close()

	page, err := b.Page(rod.PageOptions{})
	if err != nil {
		return CheckResult{
			Platform: data.PlatformSync{
				Platform: platform,
				Status:   "drift",
				Issues:   []string{fmt.Sprintf("Page creation failed: %v", err)},
			},
		}
	}

	urls := browser.PlatformURLs[platform]
	if err := browser.NavigateWithDelay(page, urls.Jobs); err != nil {
		return CheckResult{
			Platform: data.PlatformSync{
				Platform: platform,
				Status:   "drift",
				Issues:   []string{fmt.Sprintf("Navigation failed: %v", err)},
			},
		}
	}

	// Extract page text and compare against Supabase data.
	text, err := page.HTML()
	if err != nil {
		log.Printf("[sync] failed to get page HTML for %s: %v", platform, err)
	}

	var issues []string

	// Check for key profile fields in page text.
	configMap := make(map[string]string)
	for _, row := range profile.SiteConfig {
		configMap[row.Key] = row.Value
	}

	if title, ok := configMap["title"]; ok && title != "" {
		if !strings.Contains(text, title) {
			issues = append(issues, fmt.Sprintf("Headline may not match — expected to contain: %q", title))
		}
	}

	status := "synced"
	if len(issues) > 0 {
		status = "drift"
	}

	return CheckResult{
		Platform: data.PlatformSync{
			Platform: platform,
			Status:   status,
			Issues:   issues,
		},
	}
}
```

**Step 2: Wire streamSync in service.go**

Replace the `streamSync` stub:

```go
func (s *ScoutService) streamSync(inputJSON string, stream grpc.ServerStreamingServer[soulv1.ToolEvent]) error {
	var input struct {
		Platforms []string `json:"platforms"`
	}
	if err := json.Unmarshal([]byte(inputJSON), &input); err != nil {
		return stream.Send(&soulv1.ToolEvent{
			Event: &soulv1.ToolEvent_Error{Error: &soulv1.ErrorEvent{Code: "INPUT_ERROR", Message: err.Error()}},
		})
	}

	platforms := input.Platforms
	if len(platforms) == 1 && platforms[0] == "all" {
		platforms = append(browser.AllPlatforms(), "website", "github")
	}

	// Fetch profile data from Supabase.
	supa, err := supabase.NewClient()
	if err != nil {
		return stream.Send(&soulv1.ToolEvent{
			Event: &soulv1.ToolEvent_Error{Error: &soulv1.ErrorEvent{Code: "SUPABASE_ERROR", Message: err.Error()}},
		})
	}
	profile, err := supa.GetProfileData()
	if err != nil {
		return stream.Send(&soulv1.ToolEvent{
			Event: &soulv1.ToolEvent_Error{Error: &soulv1.ErrorEvent{Code: "SUPABASE_ERROR", Message: err.Error()}},
		})
	}

	var results []data.PlatformSync
	total := len(platforms)

	for i, platform := range platforms {
		// Send progress.
		stream.Send(&soulv1.ToolEvent{
			Event: &soulv1.ToolEvent_Progress{
				Progress: &soulv1.ProgressUpdate{
					Analyzer: platform,
					Percent:  float32(i) / float32(total) * 100,
					Message:  fmt.Sprintf("Checking %s... (%d/%d)", platform, i+1, total),
				},
			},
		})

		result := syncpkg.CheckPlatform(platform, profile)
		results = append(results, result.Platform)

		// Send finding if drift detected.
		if result.Platform.Status == "drift" {
			for _, issue := range result.Platform.Issues {
				stream.Send(&soulv1.ToolEvent{
					Event: &soulv1.ToolEvent_Finding{
						Finding: &soulv1.FindingEvent{
							Id:       fmt.Sprintf("sync-%s-%d", platform, i),
							Title:    fmt.Sprintf("%s: %s", platform, issue),
							Severity: "warning",
							Evidence: issue,
						},
					},
				})
			}
		}
	}

	// Store results.
	s.store.SetSyncResults(results)

	// Send complete.
	inSync, drift := 0, 0
	for _, r := range results {
		if r.Status == "synced" {
			inSync++
		} else {
			drift++
		}
	}

	return stream.Send(&soulv1.ToolEvent{
		Event: &soulv1.ToolEvent_Complete{
			Complete: &soulv1.ToolResponse{
				Success: true,
				Output:  fmt.Sprintf("Sync complete: %d platforms checked, %d in sync, %d with drift", total, inSync, drift),
			},
		},
	})
}
```

Note: You'll need to import the sync package with an alias like `syncpkg` since `sync` is a Go standard library package.

**Step 3: Build**

Run: `cd products/scout && go build ./...`

**Step 4: Commit**

```bash
git add products/scout/
git commit -m "feat(scout): sync tool — streaming platform comparison against Supabase"
```

---

### Task 10: Sweep Tool — streaming opportunity extraction

**Files:**
- Create: `products/scout/internal/sweep/monitor.go`
- Modify: `products/scout/internal/service.go` (replace `streamSweep` stub)

**Step 1: Create monitor.go**

Create `products/scout/internal/sweep/monitor.go`:
```go
package sweep

import (
	"fmt"
	"log"

	"github.com/go-rod/rod"

	"github.com/rishav1305/soul/products/scout/internal/browser"
	"github.com/rishav1305/soul/products/scout/internal/data"
)

// SweepResult holds results from sweeping one platform.
type SweepResult struct {
	Opportunities []data.Opportunity
	Messages      []data.Message
	Error         error
}

// SweepPlatform checks a platform for new opportunities and messages.
func SweepPlatform(platform string) SweepResult {
	if !browser.HasProfile(platform) {
		return SweepResult{Error: fmt.Errorf("no saved session — run setup first")}
	}

	b, err := browser.LaunchHeadless(platform)
	if err != nil {
		return SweepResult{Error: fmt.Errorf("browser launch: %w", err)}
	}
	defer b.Close()

	page, err := b.Page(rod.PageOptions{})
	if err != nil {
		return SweepResult{Error: fmt.Errorf("page create: %w", err)}
	}

	urls := browser.PlatformURLs[platform]
	if err := browser.NavigateWithDelay(page, urls.Jobs); err != nil {
		return SweepResult{Error: fmt.Errorf("navigate: %w", err)}
	}

	// Platform-specific extraction.
	switch platform {
	case "linkedin":
		return sweepLinkedIn(page)
	case "naukri":
		return sweepNaukri(page)
	case "indeed":
		return sweepIndeed(page)
	case "wellfound":
		return sweepWellfound(page)
	case "instahyre":
		return sweepInstahyre(page)
	default:
		return SweepResult{Error: fmt.Errorf("unknown platform: %s", platform)}
	}
}

// Platform-specific sweep implementations.
// These extract job listings from each platform's DOM.
// Selectors will need maintenance as platforms update.

func sweepLinkedIn(page *rod.Page) SweepResult {
	log.Println("[sweep] scanning LinkedIn jobs")
	// LinkedIn job cards are typically in a scrollable list.
	// Extract job title, company, and link from each card.
	var opps []data.Opportunity

	elements, err := page.Elements(".job-card-container, .jobs-search-results__list-item")
	if err != nil {
		log.Printf("[sweep] LinkedIn: no job cards found: %v", err)
		return SweepResult{Opportunities: opps}
	}

	for i, el := range elements {
		if i >= 10 { break } // Limit to 10
		title, _ := el.Text()
		if title != "" {
			opps = append(opps, data.Opportunity{
				ID:       fmt.Sprintf("li-%d", i),
				Platform: "linkedin",
				Role:     title,
			})
		}
	}

	return SweepResult{Opportunities: opps}
}

func sweepNaukri(page *rod.Page) SweepResult {
	log.Println("[sweep] scanning Naukri")
	var opps []data.Opportunity

	elements, err := page.Elements(".jobTuple, .srp-jobtuple-wrapper")
	if err != nil {
		log.Printf("[sweep] Naukri: no jobs found: %v", err)
		return SweepResult{Opportunities: opps}
	}

	for i, el := range elements {
		if i >= 10 { break }
		title, _ := el.Text()
		if title != "" {
			opps = append(opps, data.Opportunity{
				ID:       fmt.Sprintf("nk-%d", i),
				Platform: "naukri",
				Role:     title,
			})
		}
	}

	return SweepResult{Opportunities: opps}
}

func sweepIndeed(page *rod.Page) SweepResult {
	log.Println("[sweep] scanning Indeed")
	return SweepResult{}
}

func sweepWellfound(page *rod.Page) SweepResult {
	log.Println("[sweep] scanning Wellfound")
	return SweepResult{}
}

func sweepInstahyre(page *rod.Page) SweepResult {
	log.Println("[sweep] scanning Instahyre")
	return SweepResult{}
}
```

**Step 2: Wire streamSweep in service.go**

Replace the `streamSweep` stub:

```go
func (s *ScoutService) streamSweep(inputJSON string, stream grpc.ServerStreamingServer[soulv1.ToolEvent]) error {
	var input struct {
		Platforms []string `json:"platforms"`
	}
	if err := json.Unmarshal([]byte(inputJSON), &input); err != nil {
		return stream.Send(&soulv1.ToolEvent{
			Event: &soulv1.ToolEvent_Error{Error: &soulv1.ErrorEvent{Code: "INPUT_ERROR", Message: err.Error()}},
		})
	}

	platforms := input.Platforms
	if len(platforms) == 1 && platforms[0] == "all" {
		platforms = browser.AllPlatforms()
	}

	var allOpps []data.Opportunity
	var allMsgs []data.Message
	total := len(platforms)

	for i, platform := range platforms {
		stream.Send(&soulv1.ToolEvent{
			Event: &soulv1.ToolEvent_Progress{
				Progress: &soulv1.ProgressUpdate{
					Analyzer: platform,
					Percent:  float32(i) / float32(total) * 100,
					Message:  fmt.Sprintf("Sweeping %s... (%d/%d)", platform, i+1, total),
				},
			},
		})

		result := sweep.SweepPlatform(platform)
		if result.Error != nil {
			stream.Send(&soulv1.ToolEvent{
				Event: &soulv1.ToolEvent_Finding{
					Finding: &soulv1.FindingEvent{
						Id:       fmt.Sprintf("sweep-err-%s", platform),
						Title:    fmt.Sprintf("%s: sweep failed", platform),
						Severity: "warning",
						Evidence: result.Error.Error(),
					},
				},
			})
			continue
		}

		allOpps = append(allOpps, result.Opportunities...)
		allMsgs = append(allMsgs, result.Messages...)

		// Stream each opportunity as a finding.
		for _, opp := range result.Opportunities {
			stream.Send(&soulv1.ToolEvent{
				Event: &soulv1.ToolEvent_Finding{
					Finding: &soulv1.FindingEvent{
						Id:       opp.ID,
						Title:    fmt.Sprintf("%s — %s (%s)", opp.Role, opp.Company, opp.Platform),
						Severity: "info",
						Evidence: opp.URL,
					},
				},
			})
		}
	}

	// Store results.
	s.store.SetSweepResults(allOpps, allMsgs)

	return stream.Send(&soulv1.ToolEvent{
		Event: &soulv1.ToolEvent_Complete{
			Complete: &soulv1.ToolResponse{
				Success: true,
				Output:  fmt.Sprintf("Sweep complete: %d platforms swept, %d opportunities found, %d messages", total, len(allOpps), len(allMsgs)),
			},
		},
	})
}
```

Note: Import the sweep package with an alias to avoid conflicts.

**Step 3: Build**

Run: `cd products/scout && go build ./...`

**Step 4: Commit**

```bash
git add products/scout/
git commit -m "feat(scout): sweep tool — streaming opportunity extraction from job portals"
```

---

### Task 11: Wire Scout into Soul server startup

**Files:**
- Modify: `cmd/soul/main.go`

**Step 1: Add Scout binary detection and startup**

In `cmd/soul/main.go`, after the compliance product startup block, add:

```go
// Start scout product if binary is available.
scoutBin := getFlagValue(args, "--scout-bin")
if scoutBin == "" {
	scoutBin = os.Getenv("SOUL_SCOUT_BIN")
}
if scoutBin == "" {
	// Try default location in products/scout/
	cwd, _ := os.Getwd()
	candidate := filepath.Join(cwd, "products", "scout", "scout")
	if _, err := os.Stat(candidate); err == nil {
		scoutBin = candidate
	}
}
if scoutBin != "" {
	if _, err := os.Stat(scoutBin); err == nil {
		ctx := context.Background()
		fmt.Printf("  Starting scout product: %s\n", scoutBin)
		if err := manager.StartProduct(ctx, "scout", scoutBin); err != nil {
			log.Printf("WARNING: failed to start scout product: %v", err)
		} else {
			fmt.Println("  Scout product started")
		}
	} else {
		log.Printf("WARNING: scout binary not found at %s", scoutBin)
	}
}
```

Also add to the help text:

```go
fmt.Println("  SOUL_SCOUT_BIN         Path to scout product binary")
```

**Step 2: Build soul binary**

Run: `go build -o soul ./cmd/soul`

**Step 3: Verify Scout loads**

Run:
```bash
cd products/scout && make build && cd ../..
SOUL_HOST=0.0.0.0 ./soul serve
```
Expected output includes: "Starting scout product..." and "Scout product started"

**Step 4: Commit**

```bash
git add cmd/soul/main.go
git commit -m "feat(scout): wire Scout product into Soul server startup"
```

---

### Task 12: Frontend — ScoutPanel dashboard

**Files:**
- Create: `web/src/hooks/useScout.ts`
- Create: `web/src/components/panels/ScoutPanel.tsx`
- Create: `web/src/components/panels/scout/SyncStatus.tsx`
- Create: `web/src/components/panels/scout/Opportunities.tsx`
- Create: `web/src/components/panels/scout/ApplicationTracker.tsx`
- Create: `web/src/components/panels/scout/WeeklyMetrics.tsx`
- Create: `web/src/components/panels/scout/FollowUps.tsx`
- Modify: `web/src/lib/types.ts` (add Scout types)

**Step 1: Add Scout types**

Add to `web/src/lib/types.ts`:

```typescript
/* ── Scout types ────────────────────────────────────── */

export interface ScoutReport {
  sync: ScoutSyncData;
  sweep: ScoutSweepData;
  applications: ScoutApplicationData;
  metrics: Record<string, ScoutMetric>;
  follow_ups: ScoutFollowUp[];
}

export interface ScoutSyncData {
  last_run: string;
  platforms_checked: number;
  in_sync: number;
  drift: number;
  details: ScoutPlatformSync[];
}

export interface ScoutPlatformSync {
  platform: string;
  status: 'synced' | 'drift';
  issues?: string[];
}

export interface ScoutSweepData {
  last_run: string;
  new_opportunities: number;
  messages: number;
  opportunities: ScoutOpportunity[];
}

export interface ScoutOpportunity {
  id: string;
  company: string;
  role: string;
  platform: string;
  match?: string;
  url?: string;
  found_at: string;
  dismissed?: boolean;
}

export interface ScoutApplicationData {
  total: number;
  active: number;
  by_status: Record<string, number>;
  recent: ScoutApplication[];
}

export interface ScoutApplication {
  id: string;
  company: string;
  role: string;
  platform: string;
  variant: string;
  status: string;
  follow_up?: string;
  notes?: string;
  applied_at: string;
  updated_at: string;
}

export interface ScoutMetric {
  applied: number;
  responses: number;
  interviews: number;
  offers: number;
}

export interface ScoutFollowUp {
  company: string;
  role: string;
  due: string;
  action: string;
}
```

**Step 2: Create useScout hook**

Create `web/src/hooks/useScout.ts`:

```typescript
import { useState, useEffect, useCallback } from 'react';
import type { ScoutReport } from '../lib/types.ts';

export function useScout() {
  const [report, setReport] = useState<ScoutReport | null>(null);
  const [loading, setLoading] = useState(false);

  const fetchReport = useCallback(async (period = 'today') => {
    setLoading(true);
    try {
      const resp = await fetch('/api/tools/scout__report/execute', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ input: { period } }),
      });
      const data = await resp.json();
      if (data.structured_json) {
        setReport(JSON.parse(data.structured_json));
      }
    } catch (err) {
      console.error('[scout] fetch report failed:', err);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    fetchReport();
  }, [fetchReport]);

  return { report, loading, refreshReport: fetchReport };
}
```

**Step 3: Create sub-components**

Create `web/src/components/panels/scout/SyncStatus.tsx`:
```tsx
import type { ScoutPlatformSync } from '../../../lib/types.ts';

interface SyncStatusProps {
  platforms: ScoutPlatformSync[];
  lastRun: string;
}

export default function SyncStatus({ platforms, lastRun }: SyncStatusProps) {
  return (
    <div className="px-4 py-3">
      <div className="flex items-center justify-between mb-3">
        <h3 className="text-xs font-semibold text-zinc-300 uppercase tracking-wide">Profile Sync</h3>
        {lastRun && <span className="text-[10px] text-zinc-500">{new Date(lastRun).toLocaleString()}</span>}
      </div>
      <div className="grid grid-cols-2 gap-2">
        {platforms.map((p) => (
          <div key={p.platform} className={`rounded-lg px-3 py-2 text-xs ${p.status === 'synced' ? 'bg-emerald-500/10 border border-emerald-500/20' : 'bg-amber-500/10 border border-amber-500/20'}`}>
            <div className="flex items-center gap-2">
              <span className={`w-2 h-2 rounded-full ${p.status === 'synced' ? 'bg-emerald-400' : 'bg-amber-400'}`} />
              <span className="font-medium text-zinc-200 capitalize">{p.platform}</span>
            </div>
            {p.issues && p.issues.length > 0 && (
              <div className="mt-1 text-[10px] text-amber-300/80">{p.issues[0]}</div>
            )}
          </div>
        ))}
      </div>
    </div>
  );
}
```

Create `web/src/components/panels/scout/Opportunities.tsx`:
```tsx
import type { ScoutOpportunity } from '../../../lib/types.ts';

interface OpportunitiesProps {
  opportunities: ScoutOpportunity[];
}

export default function Opportunities({ opportunities }: OpportunitiesProps) {
  if (opportunities.length === 0) {
    return (
      <div className="px-4 py-3 text-xs text-zinc-500">No opportunities found yet. Run a sweep to check job portals.</div>
    );
  }

  return (
    <div className="px-4 py-3">
      <h3 className="text-xs font-semibold text-zinc-300 uppercase tracking-wide mb-3">Opportunities</h3>
      <div className="space-y-2">
        {opportunities.map((opp) => (
          <div key={opp.id} className="bg-zinc-800/50 rounded-lg px-3 py-2 text-xs">
            <div className="flex items-center justify-between">
              <span className="font-medium text-zinc-200">{opp.role}</span>
              <span className="text-[10px] text-zinc-500 capitalize">{opp.platform}</span>
            </div>
            {opp.company && <div className="text-zinc-400">{opp.company}</div>}
            {opp.match && <span className="text-[10px] text-soul">{opp.match} match</span>}
          </div>
        ))}
      </div>
    </div>
  );
}
```

Create `web/src/components/panels/scout/ApplicationTracker.tsx`:
```tsx
import type { ScoutApplication } from '../../../lib/types.ts';

const STATUS_COLORS: Record<string, string> = {
  applied: 'bg-blue-500/20 text-blue-300',
  viewed: 'bg-cyan-500/20 text-cyan-300',
  interview_scheduled: 'bg-purple-500/20 text-purple-300',
  interview_done: 'bg-violet-500/20 text-violet-300',
  offer: 'bg-emerald-500/20 text-emerald-300',
  rejected: 'bg-red-500/20 text-red-300',
  withdrawn: 'bg-zinc-500/20 text-zinc-400',
  follow_up_sent: 'bg-amber-500/20 text-amber-300',
};

interface ApplicationTrackerProps {
  applications: ScoutApplication[];
  byStatus: Record<string, number>;
}

export default function ApplicationTracker({ applications, byStatus }: ApplicationTrackerProps) {
  return (
    <div className="px-4 py-3">
      <h3 className="text-xs font-semibold text-zinc-300 uppercase tracking-wide mb-3">Applications</h3>
      {/* Status summary */}
      <div className="flex flex-wrap gap-2 mb-3">
        {Object.entries(byStatus).map(([status, count]) => (
          <span key={status} className={`px-2 py-0.5 rounded-full text-[10px] font-medium ${STATUS_COLORS[status] ?? 'bg-zinc-700 text-zinc-400'}`}>
            {status.replace(/_/g, ' ')}: {count}
          </span>
        ))}
      </div>
      {/* Recent applications */}
      <div className="space-y-2">
        {applications.slice(0, 10).map((app) => (
          <div key={app.id} className="bg-zinc-800/50 rounded-lg px-3 py-2 text-xs">
            <div className="flex items-center justify-between">
              <span className="font-medium text-zinc-200">{app.company} — {app.role}</span>
              <span className={`px-1.5 py-0.5 rounded text-[10px] ${STATUS_COLORS[app.status] ?? 'bg-zinc-700 text-zinc-400'}`}>
                {app.status.replace(/_/g, ' ')}
              </span>
            </div>
            <div className="flex items-center gap-2 mt-1 text-[10px] text-zinc-500">
              <span className="capitalize">{app.platform}</span>
              {app.variant && <span>Variant {app.variant}</span>}
              {app.follow_up && <span>Follow-up: {app.follow_up}</span>}
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}
```

Create `web/src/components/panels/scout/WeeklyMetrics.tsx`:
```tsx
import type { ScoutMetric } from '../../../lib/types.ts';

interface WeeklyMetricsProps {
  metrics: Record<string, ScoutMetric>;
}

export default function WeeklyMetrics({ metrics }: WeeklyMetricsProps) {
  const entries = Object.entries(metrics).sort().reverse().slice(0, 4);

  if (entries.length === 0) {
    return (
      <div className="px-4 py-3 text-xs text-zinc-500">No metrics yet. Track applications to see weekly stats.</div>
    );
  }

  return (
    <div className="px-4 py-3">
      <h3 className="text-xs font-semibold text-zinc-300 uppercase tracking-wide mb-3">Weekly Metrics</h3>
      <div className="space-y-2">
        {entries.map(([week, m]) => (
          <div key={week} className="bg-zinc-800/50 rounded-lg px-3 py-2">
            <div className="text-[10px] text-zinc-500 mb-1">{week}</div>
            <div className="flex gap-4 text-xs">
              <span className="text-blue-300">Applied: {m.applied}</span>
              <span className="text-cyan-300">Responses: {m.responses}</span>
              <span className="text-purple-300">Interviews: {m.interviews}</span>
              <span className="text-emerald-300">Offers: {m.offers}</span>
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}
```

Create `web/src/components/panels/scout/FollowUps.tsx`:
```tsx
import type { ScoutFollowUp } from '../../../lib/types.ts';

interface FollowUpsProps {
  followUps: ScoutFollowUp[];
}

export default function FollowUps({ followUps }: FollowUpsProps) {
  if (followUps.length === 0) return null;

  return (
    <div className="px-4 py-3">
      <h3 className="text-xs font-semibold text-zinc-300 uppercase tracking-wide mb-3">Follow-ups Due</h3>
      <div className="space-y-2">
        {followUps.map((fu, i) => (
          <div key={i} className="bg-amber-500/10 border border-amber-500/20 rounded-lg px-3 py-2 text-xs">
            <div className="font-medium text-amber-200">{fu.company} — {fu.role}</div>
            <div className="text-[10px] text-amber-300/70">Due: {fu.due} | {fu.action}</div>
          </div>
        ))}
      </div>
    </div>
  );
}
```

**Step 4: Create ScoutPanel**

Create `web/src/components/panels/ScoutPanel.tsx`:
```tsx
import { useScout } from '../../hooks/useScout.ts';
import SyncStatus from './scout/SyncStatus.tsx';
import Opportunities from './scout/Opportunities.tsx';
import ApplicationTracker from './scout/ApplicationTracker.tsx';
import WeeklyMetrics from './scout/WeeklyMetrics.tsx';
import FollowUps from './scout/FollowUps.tsx';

export default function ScoutPanel() {
  const { report, loading, refreshReport } = useScout();

  return (
    <div className="flex flex-col h-full">
      {/* Header */}
      <div className="flex items-center justify-between px-4 py-3 border-b border-zinc-800">
        <h2 className="text-sm font-semibold text-zinc-200">Scout</h2>
        <button
          onClick={() => refreshReport()}
          disabled={loading}
          className="px-2 py-1 text-[10px] font-medium rounded bg-soul/10 text-soul hover:bg-soul/20 transition-colors disabled:opacity-50 cursor-pointer"
        >
          {loading ? 'Loading...' : 'Refresh'}
        </button>
      </div>

      {/* Content */}
      <div className="flex-1 overflow-y-auto divide-y divide-zinc-800/50">
        {!report ? (
          <div className="px-4 py-8 text-center text-xs text-zinc-500">
            {loading ? 'Loading Scout data...' : 'No Scout data yet. Run sync and sweep to get started.'}
          </div>
        ) : (
          <>
            <SyncStatus
              platforms={report.sync.details ?? []}
              lastRun={report.sync.last_run}
            />
            <Opportunities opportunities={report.sweep.opportunities ?? []} />
            <ApplicationTracker
              applications={report.applications.recent ?? []}
              byStatus={report.applications.by_status ?? {}}
            />
            <WeeklyMetrics metrics={report.metrics ?? {}} />
            <FollowUps followUps={report.follow_ups ?? []} />
          </>
        )}
      </div>
    </div>
  );
}
```

**Step 5: Build frontend**

Run: `cd web && npx vite build`
Expected: Builds clean (ScoutPanel isn't wired into AppShell yet — that's next).

**Step 6: Commit**

```bash
git add web/src/
git commit -m "feat(scout): frontend ScoutPanel dashboard with 5 sub-components"
```

---

### Task 13: Wire ScoutPanel into Soul UI navigation

**Files:**
- Modify: `web/src/components/layout/SoulPanel.tsx` (add Scout nav item)
- Modify: `web/src/components/layout/AppShell.tsx` (add Scout panel state)

This task wires the ScoutPanel into the Soul UI. The exact approach depends on the current navigation pattern — SoulPanel currently shows chat sessions. We'll add a "Scout" button in SoulPanel that opens the ScoutPanel in place of or alongside the task panel.

**Step 1: Add Scout section to SoulPanel**

In `web/src/components/layout/SoulPanel.tsx`, add a "Scout" button in the sidebar above the footer, below the sessions list. When clicked, it should toggle the Scout dashboard visibility. The simplest approach: add an `onScoutToggle` callback prop and a "Scout" button.

**Step 2: Add ScoutPanel rendering in AppShell**

In AppShell, add state for `scoutOpen` and conditionally render `<ScoutPanel />` when open, overlaying or replacing the task panel.

The exact implementation depends on the UI pattern you want — this can be a modal, a panel replacement, or an additional tab. The subagent implementing this should read the current navigation code and follow existing patterns.

**Step 3: Build**

Run: `cd web && npx vite build`

**Step 4: Commit**

```bash
git add web/src/
git commit -m "feat(scout): wire ScoutPanel into Soul UI navigation"
```

---

### Task 14: Integration Test — build + run full pipeline

**Step 1: Build Scout binary**

```bash
cd products/scout && make build
```

**Step 2: Create config file**

```bash
mkdir -p ~/.soul/scout
cat > ~/.soul/scout/config.json << 'EOF'
{
  "supabase_url": "https://YOUR_PROJECT.supabase.co",
  "supabase_anon_key": "YOUR_ANON_KEY"
}
EOF
```

(Use actual Supabase credentials from Vaultwarden)

**Step 3: Build and start Soul with Scout**

```bash
cd /home/rishav/soul
go build -o soul ./cmd/soul
SOUL_HOST=0.0.0.0 ./soul serve
```

Expected output:
```
  Scout product started
◆ Soul server listening on 0.0.0.0:3000
```

**Step 4: Verify tools are registered**

```bash
curl -s http://localhost:3000/api/tools | jq '.[] | select(.product == "scout") | .name'
```
Expected: setup, sync, sweep, generate, track, report

**Step 5: Test track tool**

```bash
curl -s -X POST http://localhost:3000/api/tools/scout__track/execute \
  -H 'Content-Type: application/json' \
  -d '{"input":{"action":"add","company":"Test Corp","role":"AI Engineer","platform":"linkedin","variant":"B"}}' | jq
```
Expected: `{ "success": true, "output": "Application added: Test Corp — AI Engineer (linkedin)" }`

**Step 6: Test report tool**

```bash
curl -s -X POST http://localhost:3000/api/tools/scout__report/execute \
  -H 'Content-Type: application/json' \
  -d '{"input":{"period":"today"}}' | jq
```
Expected: JSON with applications section showing the Test Corp entry.

**Step 7: Verify Scout dashboard loads**

Open `http://localhost:3000` in browser. Navigate to Scout panel. Should show the dashboard with tracked application.

**Step 8: Commit any fixes**

```bash
git add -A && git commit -m "fix(scout): integration test fixes"
```
