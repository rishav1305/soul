# Profile Hub Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Create a centralized profile database on local PostgreSQL that serves as the primary source of truth for all professional profile data, with bidirectional sync to Supabase and a read-only Profile panel in Scout's UI.

**Architecture:** Dedicated PostgreSQL 16 container (`titan-profile-db`) on titan-pc, accessed from titan-pi via SSH tunnel. Scout gains 3 new tools (`profile`, `profile_pull`, `profile_push`) backed by a `pgx/v5` client. Supabase pulls are automated daily via systemd timer; pushes are manual only. A new Profile tab in the Scout panel displays the unified profile.

**Tech Stack:** Go 1.24 + pgx/v5, PostgreSQL 16, Docker, systemd timers, React 19 + TypeScript + Tailwind v4

---

## Task 1: Create PostgreSQL Container on titan-pc

**Files:**
- Create: `products/scout/infra/docker-compose.profile-db.yml`
- Create: `products/scout/infra/init-profile-db.sql`

**Step 1: Write docker-compose file**

SSH into titan-pc and create the compose file locally. We also keep a copy in the repo for documentation.

```yaml
# products/scout/infra/docker-compose.profile-db.yml
services:
  titan-profile-db:
    image: postgres:16
    container_name: titan-profile-db
    restart: unless-stopped
    ports:
      - "127.0.0.1:5434:5432"
    environment:
      POSTGRES_DB: profile
      POSTGRES_USER: profile
      POSTGRES_PASSWORD: "${PROFILE_DB_PASSWORD}"
    volumes:
      - profile-db-data:/var/lib/postgresql/data
      - ./init-profile-db.sql:/docker-entrypoint-initdb.d/01-schema.sql:ro

volumes:
  profile-db-data:
```

**Step 2: Write schema init script**

Mirror all 13 Supabase tables. Get exact DDL from the `portfolio_backup` database on titan-pc:

```bash
ssh rishav@192.168.0.196 "docker exec titan-gitea-db pg_dump -U gitea -d portfolio_backup --schema-only --no-owner --no-privileges" > products/scout/infra/init-profile-db.sql
```

Review the output and clean up any Gitea-specific artifacts. The script should create all 13 tables:
- site_config, experience, skill_categories, projects
- education, testimonials, brands, services
- case_studies, chat_questions, faqs, stats_dashboard, skill_radar_data

**Step 3: Generate a password and store in Vaultwarden**

```bash
export NODE_TLS_REJECT_UNAUTHORIZED=0
# Generate a random password
PW=$(openssl rand -base64 24)
# Store in Vaultwarden (create a new item "titan-profile-db")
bw create item "$(echo '{"type":1,"name":"titan-profile-db","login":{"username":"profile","password":"'"$PW"'","uris":[{"uri":"postgresql://127.0.0.1:5434/profile"}]}}' | bw encode)"
echo "Password stored. Use it in next step."
```

**Step 4: Launch the container on titan-pc**

```bash
ssh rishav@192.168.0.196 << 'REMOTE'
mkdir -p ~/profile-db
# Copy compose + init SQL (will be scp'd separately)
cd ~/profile-db
PROFILE_DB_PASSWORD="<password-from-vaultwarden>" docker compose -f docker-compose.profile-db.yml up -d
# Verify container is running
docker ps --filter name=titan-profile-db
# Verify database is accessible
docker exec titan-profile-db psql -U profile -d profile -c '\dt'
REMOTE
```

Expected: Container running, `\dt` shows the 13 tables (empty).

**Step 5: Commit infra files**

```bash
git add products/scout/infra/docker-compose.profile-db.yml products/scout/infra/init-profile-db.sql
git commit -m "infra(scout): add profile-db docker-compose and schema

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>"
```

---

## Task 2: Seed Profile Database from portfolio_backup

**Step 1: Dump data from portfolio_backup**

```bash
ssh rishav@192.168.0.196 "docker exec titan-gitea-db pg_dump -U gitea -d portfolio_backup --data-only --no-owner --no-privileges --inserts" > /tmp/profile-seed.sql
```

**Step 2: Restore into titan-profile-db**

```bash
scp /tmp/profile-seed.sql rishav@192.168.0.196:/tmp/profile-seed.sql
ssh rishav@192.168.0.196 "docker exec -i titan-profile-db psql -U profile -d profile < /tmp/profile-seed.sql"
```

**Step 3: Verify data**

```bash
ssh rishav@192.168.0.196 "docker exec titan-profile-db psql -U profile -d profile -c 'SELECT count(*) FROM site_config; SELECT count(*) FROM experience; SELECT count(*) FROM projects; SELECT count(*) FROM skill_categories;'"
```

Expected: Row counts match `portfolio_backup` (site_config=1, experience~6, projects~8, skill_categories~5).

**Step 4: Verify no schema drift from Supabase**

Compare a few key columns to make sure the seed data is current:

```bash
ssh rishav@192.168.0.196 "docker exec titan-profile-db psql -U profile -d profile -c \"SELECT name, title, location FROM site_config LIMIT 1;\""
```

Expected: Name, title, location match current Supabase values.

---

## Task 3: Set Up SSH Tunnel for Profile DB

**Files:**
- Create: `~/.config/systemd/user/profile-db-tunnel.service`

**Step 1: Write systemd service**

Model after the existing `rod-tunnel.service`:

```ini
# ~/.config/systemd/user/profile-db-tunnel.service
[Unit]
Description=SSH tunnel to profile-db on titan-pc
After=network-online.target
Wants=network-online.target

[Service]
ExecStart=/usr/bin/ssh -N \
  -o ServerAliveInterval=30 \
  -o ServerAliveCountMax=3 \
  -o ExitOnForwardFailure=yes \
  -L 5434:127.0.0.1:5434 titan-pc
Restart=on-failure
RestartSec=5

[Install]
WantedBy=default.target
```

**Step 2: Enable and start the tunnel**

```bash
systemctl --user daemon-reload
systemctl --user enable --now profile-db-tunnel.service
systemctl --user status profile-db-tunnel.service
```

Expected: Active (running).

**Step 3: Verify local connectivity**

```bash
psql -h 127.0.0.1 -p 5434 -U profile -d profile -c '\dt'
```

If `psql` is not installed on titan-pi:

```bash
# Test with netcat instead
nc -zv 127.0.0.1 5434
```

Expected: Connection succeeds, tables listed (or port open).

---

## Task 4: Add pgx/v5 Dependency to Scout

**Files:**
- Modify: `products/scout/go.mod`
- Modify: `products/scout/go.sum`

**Step 1: Add pgx dependency**

```bash
cd /home/rishav/soul/products/scout
go get github.com/jackc/pgx/v5
go mod tidy
```

**Step 2: Verify it compiles**

```bash
go build ./...
```

Expected: Clean build, no errors.

**Step 3: Commit**

```bash
cd /home/rishav/soul
git add products/scout/go.mod products/scout/go.sum
git commit -m "deps(scout): add pgx/v5 PostgreSQL driver

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>"
```

---

## Task 5: Create Profile DB Client Package

**Files:**
- Create: `products/scout/internal/profiledb/client.go`
- Create: `products/scout/internal/profiledb/types.go`

**Step 1: Define profile types**

```go
// products/scout/internal/profiledb/types.go
package profiledb

// FullProfile contains all profile data from the local PostgreSQL database.
type FullProfile struct {
	SiteConfig      []SiteConfig      `json:"site_config"`
	Experience      []Experience      `json:"experience"`
	SkillCategories []SkillCategory   `json:"skill_categories"`
	Projects        []Project         `json:"projects"`
	Education       []Education       `json:"education"`
	Testimonials    []Testimonial     `json:"testimonials"`
	Brands          []Brand           `json:"brands"`
	Services        []Service         `json:"services"`
	CaseStudies     []CaseStudy       `json:"case_studies"`
	ChatQuestions   []ChatQuestion    `json:"chat_questions"`
	FAQs            []FAQ             `json:"faqs"`
	StatsDashboard  []StatsDashboard  `json:"stats_dashboard"`
	SkillRadarData  []SkillRadarData  `json:"skill_radar_data"`
}
```

The exact struct fields for each type must match the PostgreSQL column definitions from the `init-profile-db.sql` schema dumped in Task 1. Use `json` tags matching the column names. For jsonb columns (achievements, tech_stack, social_media, skills, etc.), use `json.RawMessage` or `[]string`/`map[string]string` as appropriate.

**How to determine exact fields:**
1. Read the schema SQL from Task 1 (`products/scout/infra/init-profile-db.sql`)
2. For each table, create a Go struct with one field per column
3. Use `pgx` compatible types: `string`, `*string` (nullable), `int`, `*int` (nullable), `json.RawMessage` (jsonb), `*time.Time` (nullable timestamp)

Key structs to define (fields determined by schema):
- `SiteConfig` — name, title, email, short_bio, long_bio, location, years_experience_start_year, whatsapp, social_media (jsonb→`map[string]string`), etc.
- `Experience` — role, company, period, start_date, end_date, location, achievements (jsonb→`json.RawMessage`), tech_stack (jsonb→`json.RawMessage`), etc.
- `SkillCategory` — category_name, skills (jsonb→`json.RawMessage`), display_order
- `Project` — title, description, short_description, tech_stack (jsonb→`json.RawMessage`), category, company, etc.
- `Education` — institution, degree, field, start_year, end_year, etc.
- `Testimonial`, `Brand`, `Service`, `CaseStudy`, `ChatQuestion`, `FAQ`, `StatsDashboard`, `SkillRadarData` — fields per schema

**Step 2: Write the database client**

```go
// products/scout/internal/profiledb/client.go
package profiledb

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Config holds connection settings for the profile database.
type Config struct {
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Database string `json:"database"`
	User     string `json:"user"`
	Password string `json:"password"`
}

// Client provides access to the local profile database.
type Client struct {
	pool *pgxpool.Pool
}

// New creates a new Client connected to the profile database.
func New(ctx context.Context, cfg Config) (*Client, error) {
	dsn := fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=disable",
		cfg.User, cfg.Password, cfg.Host, cfg.Port, cfg.Database)
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, fmt.Errorf("connect to profile db: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping profile db: %w", err)
	}
	return &Client{pool: pool}, nil
}

// Close releases the connection pool.
func (c *Client) Close() {
	if c.pool != nil {
		c.pool.Close()
	}
}

// GetFullProfile fetches all 13 tables and returns a unified profile.
func (c *Client) GetFullProfile(ctx context.Context) (*FullProfile, error) {
	p := &FullProfile{}
	// Fetch each table using pgx.CollectRows with RowToStructByName
	// Example for site_config:
	//   rows, _ := c.pool.Query(ctx, "SELECT * FROM site_config")
	//   p.SiteConfig, err = pgx.CollectRows(rows, pgx.RowToStructByName[SiteConfig])
	// Repeat for all 13 tables.
	// Return aggregate.
	return p, nil
}
```

Use `pgx.CollectRows` with `pgx.RowToStructByName[T]` for each table. This requires struct field names to match column names via `db` tags or exact name matching. Add `db:"column_name"` tags to all struct fields.

**Step 3: Verify it compiles**

```bash
cd /home/rishav/soul/products/scout
go build ./...
```

Expected: Clean build.

**Step 4: Commit**

```bash
cd /home/rishav/soul
git add products/scout/internal/profiledb/
git commit -m "feat(scout): add profiledb client package with pgx/v5

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>"
```

---

## Task 6: Add profile_db Config to Scout

**Files:**
- Modify: `products/scout/internal/supabase/client.go` (config loading)
- Modify: `~/.soul/scout/config.json` (runtime config)

**Step 1: Extend config structure**

The Scout config is loaded in `supabase/client.go` lines 36-61. The config.json needs a new `profile_db` section. Add the `Config` struct from profiledb package to the overall config.

In `supabase/client.go`, the `loadConfig` function reads `~/.soul/scout/config.json`. Either:
- (a) Add `ProfileDB` field to the existing config struct in `supabase/client.go`, or
- (b) Have `profiledb` package load config independently

Option (b) is cleaner — keep packages independent:

```go
// In products/scout/internal/profiledb/client.go, add:

// LoadConfig reads profile_db settings from ~/.soul/scout/config.json.
func LoadConfig() (*Config, error) {
	home, _ := os.UserHomeDir()
	path := filepath.Join(home, ".soul", "scout", "config.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}
	var wrapper struct {
		ProfileDB Config `json:"profile_db"`
	}
	if err := json.Unmarshal(data, &wrapper); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	return &wrapper.ProfileDB, nil
}
```

**Step 2: Update config.json**

Fetch the password from Vaultwarden:

```bash
export NODE_TLS_REJECT_UNAUTHORIZED=0
bw get password titan-profile-db
```

Add to `~/.soul/scout/config.json`:

```json
{
  "supabase_url": "...",
  "supabase_anon_key": "...",
  "remote_browser": { ... },
  "profile_db": {
    "host": "127.0.0.1",
    "port": 5434,
    "database": "profile",
    "user": "profile",
    "password": "<from-vaultwarden>"
  }
}
```

**Step 3: Verify compilation**

```bash
cd /home/rishav/soul/products/scout && go build ./...
```

**Step 4: Commit code changes only (not config.json — it has secrets)**

```bash
cd /home/rishav/soul
git add products/scout/internal/profiledb/client.go
git commit -m "feat(scout): add profile_db config loading

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>"
```

---

## Task 7: Implement GetFullProfile

**Files:**
- Modify: `products/scout/internal/profiledb/client.go`

**Step 1: Implement all 13 table queries**

For each table, follow this pattern:

```go
func (c *Client) getSiteConfig(ctx context.Context) ([]SiteConfig, error) {
	rows, err := c.pool.Query(ctx, "SELECT * FROM site_config")
	if err != nil {
		return nil, fmt.Errorf("query site_config: %w", err)
	}
	return pgx.CollectRows(rows, pgx.RowToStructByName[SiteConfig])
}
```

Repeat for: experience (ORDER BY start_date DESC), skill_categories (ORDER BY display_order), projects, education, testimonials, brands, services, case_studies, chat_questions, faqs, stats_dashboard, skill_radar_data.

Wire them all into `GetFullProfile()`:

```go
func (c *Client) GetFullProfile(ctx context.Context) (*FullProfile, error) {
	p := &FullProfile{}
	var err error

	if p.SiteConfig, err = c.getSiteConfig(ctx); err != nil {
		return nil, err
	}
	if p.Experience, err = c.getExperience(ctx); err != nil {
		return nil, err
	}
	// ... all 13 tables
	return p, nil
}
```

**Step 2: Test manually**

After building, test with a quick Go test file or by calling the profile tool (Task 8). For now, verify compilation:

```bash
cd /home/rishav/soul/products/scout && go build ./...
```

**Step 3: Commit**

```bash
cd /home/rishav/soul
git add products/scout/internal/profiledb/
git commit -m "feat(scout): implement GetFullProfile for all 13 tables

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>"
```

---

## Task 8: Register scout__profile Tool

**Files:**
- Modify: `products/scout/internal/service.go`

**Step 1: Add tool to manifest**

In `GetManifest()` (around line 65), add after the existing 6 tools:

```go
{
	Name:        "profile",
	Description: "Return the full professional profile from the local profile database as structured JSON. Includes site config, experience, skills, projects, education, and more.",
	InputSchemaJson: `{
		"type": "object",
		"properties": {
			"sections": {
				"type": "array",
				"items": {"type": "string"},
				"description": "Optional filter: return only these sections (e.g. ['experience','skills']). Empty = all."
			}
		}
	}`,
},
```

**Step 2: Add input struct**

After the existing input structs (around line 209):

```go
type profileInput struct {
	Sections []string `json:"sections"`
}
```

**Step 3: Add ExecuteTool case**

In the `ExecuteTool` switch (around line 212):

```go
case "profile":
	return s.executeProfile(req.InputJson)
```

**Step 4: Implement executeProfile**

```go
func (s *ScoutService) executeProfile(inputJSON string) (*soulv1.ToolResponse, error) {
	var input profileInput
	if inputJSON != "" {
		if err := json.Unmarshal([]byte(inputJSON), &input); err != nil {
			return nil, fmt.Errorf("parse profile input: %w", err)
		}
	}

	cfg, err := profiledb.LoadConfig()
	if err != nil {
		return toolError("profile_db config: " + err.Error())
	}

	ctx := context.Background()
	client, err := profiledb.New(ctx, *cfg)
	if err != nil {
		return toolError("connect profile_db: " + err.Error())
	}
	defer client.Close()

	profile, err := client.GetFullProfile(ctx)
	if err != nil {
		return toolError("fetch profile: " + err.Error())
	}

	// If sections filter is specified, zero out unneeded fields.
	if len(input.Sections) > 0 {
		profile = filterSections(profile, input.Sections)
	}

	data, _ := json.Marshal(profile)
	return &soulv1.ToolResponse{
		Content: []*soulv1.Content{{
			Type: "text",
			Text: string(data),
		}},
	}, nil
}
```

Add a helper `filterSections` that zeros out struct fields not in the requested list.

**Step 5: Build and test**

```bash
cd /home/rishav/soul/products/scout && go build -o scout ./cmd/scout
# Restart soul, then test via API:
curl -s http://localhost:3000/api/tools/scout__profile/execute \
  -H 'Content-Type: application/json' \
  -d '{"input":{}}' | jq '.structured_json.site_config[0].name'
```

Expected: Returns your name from the local profile DB.

**Step 6: Commit**

```bash
cd /home/rishav/soul
git add products/scout/internal/service.go
git commit -m "feat(scout): add scout__profile tool for local profile access

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>"
```

---

## Task 9: Implement Supabase Pull (profile_pull)

**Files:**
- Modify: `products/scout/internal/profiledb/client.go` (add UpsertTable method)
- Modify: `products/scout/internal/service.go` (add profile_pull tool)

**Step 1: Add upsert method to profiledb client**

```go
// UpsertRows upserts JSON rows (from Supabase REST) into a local table.
// It uses INSERT ... ON CONFLICT (id) DO UPDATE for tables with an id column,
// or TRUNCATE + INSERT for tables without a natural key.
func (c *Client) UpsertTable(ctx context.Context, table string, rows []map[string]interface{}) error {
	// Strategy: TRUNCATE the table, then INSERT all rows.
	// This is safe because Supabase is the source of truth for pulls.
	tx, err := c.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, "TRUNCATE "+table); err != nil {
		return fmt.Errorf("truncate %s: %w", table, err)
	}

	// Build INSERT from the first row's keys.
	// Use pgx batch for efficiency.
	// ... (implementation details based on actual column names)

	return tx.Commit(ctx)
}
```

**Step 2: Add pull method that fetches from Supabase REST and upserts locally**

```go
// PullFromSupabase fetches all rows from a Supabase table via REST and upserts into local PG.
func (c *Client) PullFromSupabase(ctx context.Context, supabaseURL, anonKey, table string) (int, error) {
	// GET {supabaseURL}/rest/v1/{table}?select=*
	// Parse JSON array of objects
	// Call UpsertTable
	// Return row count
}
```

**Step 3: Register profile_pull tool in service.go**

Add to manifest, input struct, ExecuteTool switch, and implement `executeProfilePull`:

```go
type profilePullInput struct {
	Tables []string `json:"tables"` // Empty = all 13 tables
}

func (s *ScoutService) executeProfilePull(inputJSON string) (*soulv1.ToolResponse, error) {
	// Load both supabase config and profile_db config
	// Connect to profile_db
	// For each table (or specified tables):
	//   Fetch from Supabase REST API
	//   Upsert into local PG
	//   Track row counts
	// Return summary: "Pulled 13 tables: site_config(1), experience(6), ..."
}
```

**Step 4: Build and test**

```bash
cd /home/rishav/soul/products/scout && go build -o scout ./cmd/scout
# Test pull:
curl -s http://localhost:3000/api/tools/scout__profile_pull/execute \
  -H 'Content-Type: application/json' \
  -d '{"input":{}}' | jq '.'
```

Expected: Returns summary of rows pulled per table.

**Step 5: Commit**

```bash
cd /home/rishav/soul
git add products/scout/internal/profiledb/ products/scout/internal/service.go
git commit -m "feat(scout): add scout__profile_pull tool for Supabase->local sync

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>"
```

---

## Task 10: Implement Supabase Push (profile_push)

**Files:**
- Modify: `products/scout/internal/profiledb/client.go` (add read-for-push method)
- Modify: `products/scout/internal/service.go` (add profile_push tool)

**Step 1: Add method to read a table as JSON for push**

```go
// ReadTableJSON reads all rows from a local table and returns them as JSON-compatible maps.
func (c *Client) ReadTableJSON(ctx context.Context, table string) ([]map[string]interface{}, error) {
	rows, err := c.pool.Query(ctx, "SELECT * FROM "+table)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	// Use rows.FieldDescriptions() to get column names
	// Build []map[string]interface{} from row values
	return results, nil
}
```

**Step 2: Add method to push rows to Supabase**

```go
// PushToSupabase sends local rows to Supabase via POST/PATCH REST API.
func (c *Client) PushToSupabase(ctx context.Context, supabaseURL, anonKey, table string, rows []map[string]interface{}) error {
	// POST {supabaseURL}/rest/v1/{table}
	// Headers: apikey, Authorization, Content-Type, Prefer: resolution=merge-duplicates
	// Body: JSON array of row objects
}
```

**Step 3: Register profile_push tool**

```go
type profilePushInput struct {
	Tables  []string `json:"tables"`  // Empty = all 13 tables
	Confirm bool     `json:"confirm"` // Must be true to actually push
}

func (s *ScoutService) executeProfilePush(inputJSON string) (*soulv1.ToolResponse, error) {
	// Safety: require confirm=true
	if !input.Confirm {
		return toolText("Push to Supabase requires confirm=true. This will overwrite live website data.")
	}
	// Load configs
	// For each table:
	//   Read from local PG
	//   Push to Supabase REST
	//   Track counts
	// Return summary
}
```

**Step 4: Build and test (dry run only — do NOT actually push)**

```bash
cd /home/rishav/soul/products/scout && go build -o scout ./cmd/scout
# Test without confirm (should return warning):
curl -s http://localhost:3000/api/tools/scout__profile_push/execute \
  -H 'Content-Type: application/json' \
  -d '{"input":{"confirm":false}}' | jq '.'
```

Expected: Returns safety warning message, no data pushed.

**Step 5: Commit**

```bash
cd /home/rishav/soul
git add products/scout/internal/profiledb/ products/scout/internal/service.go
git commit -m "feat(scout): add scout__profile_push tool for local->Supabase sync

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>"
```

---

## Task 11: Create Daily Pull Sync Script + systemd Timer

**Files:**
- Create: `products/scout/infra/profile-sync-pull.sh`
- Create: `~/.config/systemd/user/profile-sync-pull.service`
- Create: `~/.config/systemd/user/profile-sync-pull.timer`

**Step 1: Write the sync script**

The script calls the Scout `profile_pull` tool via the Soul API:

```bash
#!/usr/bin/env bash
# products/scout/infra/profile-sync-pull.sh
# Daily pull: Supabase -> local profile-db
set -euo pipefail

LOG="/var/log/profile-sync.log"
API="http://127.0.0.1:3000/api/tools/scout__profile_pull/execute"

echo "$(date -Is) Starting Supabase -> local profile pull" >> "$LOG"

RESULT=$(curl -sf "$API" \
  -H 'Content-Type: application/json' \
  -d '{"input":{"tables":[]}}' 2>> "$LOG")

echo "$(date -Is) Result: $RESULT" >> "$LOG"
echo "$(date -Is) Pull complete" >> "$LOG"
```

Alternatively, if Soul isn't always running, the script can directly invoke the scout binary or use `psql` + Supabase REST. The Soul API approach is simpler if Soul runs 24/7.

**Step 2: Write systemd service**

```ini
# ~/.config/systemd/user/profile-sync-pull.service
[Unit]
Description=Pull Supabase profile data to local PostgreSQL
After=network-online.target
Wants=network-online.target

[Service]
Type=oneshot
ExecStart=/home/rishav/soul/products/scout/infra/profile-sync-pull.sh
```

**Step 3: Write systemd timer**

```ini
# ~/.config/systemd/user/profile-sync-pull.timer
[Unit]
Description=Daily profile sync pull timer

[Timer]
OnCalendar=*-*-* 03:00:00
Persistent=true

[Install]
WantedBy=timers.target
```

**Step 4: Enable the timer**

```bash
chmod +x /home/rishav/soul/products/scout/infra/profile-sync-pull.sh
systemctl --user daemon-reload
systemctl --user enable --now profile-sync-pull.timer
systemctl --user list-timers | grep profile
```

Expected: Timer listed, next trigger at 03:00.

**Step 5: Test manually**

```bash
systemctl --user start profile-sync-pull.service
journalctl --user -u profile-sync-pull.service --no-pager
```

Expected: Pull completes successfully.

**Step 6: Commit**

```bash
cd /home/rishav/soul
git add products/scout/infra/profile-sync-pull.sh
git commit -m "feat(scout): add daily Supabase->local profile sync script + timer

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>"
```

---

## Task 12: Add Profile Types to Frontend

**Files:**
- Modify: `web/src/lib/types.ts`

**Step 1: Add profile types**

Add after the existing Scout types:

```typescript
// Profile Hub types — matches profiledb.FullProfile from Go
export interface ProfileData {
  site_config: ProfileSiteConfig[];
  experience: ProfileExperience[];
  skill_categories: ProfileSkillCategory[];
  projects: ProfileProject[];
  education: ProfileEducation[];
  testimonials: ProfileTestimonial[];
  brands: ProfileBrand[];
  services: ProfileService[];
  case_studies: ProfileCaseStudy[];
  // Omit less-used tables from UI types for now
}

export interface ProfileSiteConfig {
  id: number;
  name: string;
  title: string;
  email: string;
  short_bio: string;
  long_bio: string;
  location: string;
  years_experience_start_year: number;
  whatsapp: string;
  social_media: Record<string, string>;
}

export interface ProfileExperience {
  id: number;
  role: string;
  company: string;
  period: string;
  start_date: string;
  end_date: string | null;
  location: string;
  achievements: string[];
  tech_stack: string[];
}

export interface ProfileSkillCategory {
  id: number;
  category_name: string;
  skills: Array<{ name: string; level: number }>;
  display_order: number;
}

export interface ProfileProject {
  id: number;
  title: string;
  description: string;
  short_description: string;
  tech_stack: string[];
  category: string;
  company: string;
}

export interface ProfileEducation {
  id: number;
  institution: string;
  degree: string;
  field: string;
  start_year: number;
  end_year: number | null;
}

export interface ProfileTestimonial {
  id: number;
  name: string;
  role: string;
  company: string;
  text: string;
}

export interface ProfileBrand {
  id: number;
  name: string;
  logo_url: string;
}

export interface ProfileService {
  id: number;
  title: string;
  description: string;
  icon: string;
}

export interface ProfileCaseStudy {
  id: number;
  title: string;
  description: string;
  client: string;
  results: string;
}
```

Note: Exact fields must match the Go struct JSON output. Adjust after seeing actual `scout__profile` tool output.

**Step 2: Build frontend to check types**

```bash
cd /home/rishav/soul/web && npx tsc --noEmit
```

Expected: No type errors.

**Step 3: Commit**

```bash
cd /home/rishav/soul
git add web/src/lib/types.ts
git commit -m "feat(web): add Profile Hub TypeScript types

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>"
```

---

## Task 13: Create useProfile Hook

**Files:**
- Create: `web/src/hooks/useProfile.ts`

**Step 1: Write the hook**

```typescript
// web/src/hooks/useProfile.ts
import { useState, useEffect, useCallback } from 'react';
import type { ProfileData } from '../lib/types';

async function callProfileTool(tool: string, input: Record<string, unknown> = {}) {
  const resp = await fetch(`/api/tools/scout__${tool}/execute`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ input }),
  });
  return resp.json();
}

export function useProfile() {
  const [profile, setProfile] = useState<ProfileData | null>(null);
  const [loading, setLoading] = useState(true);
  const [pulling, setPulling] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const fetchProfile = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const res = await callProfileTool('profile');
      if (res?.content?.[0]?.text) {
        setProfile(JSON.parse(res.content[0].text));
      }
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Failed to load profile');
    } finally {
      setLoading(false);
    }
  }, []);

  const pullFromSupabase = useCallback(async () => {
    setPulling(true);
    try {
      await callProfileTool('profile_pull');
      await fetchProfile();
    } finally {
      setPulling(false);
    }
  }, [fetchProfile]);

  useEffect(() => { fetchProfile(); }, [fetchProfile]);

  return { profile, loading, pulling, error, fetchProfile, pullFromSupabase };
}
```

**Step 2: Verify compilation**

```bash
cd /home/rishav/soul/web && npx tsc --noEmit
```

**Step 3: Commit**

```bash
cd /home/rishav/soul
git add web/src/hooks/useProfile.ts
git commit -m "feat(web): add useProfile hook for profile data fetching

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>"
```

---

## Task 14: Build Profile Panel Component

**Files:**
- Create: `web/src/components/panels/scout/ProfilePanel.tsx`

**Step 1: Build the Profile panel**

The panel has 5 collapsible sections: Identity, Experience, Skills, Projects, Education. Read-only. Shows "Last synced" timestamp and a "Pull from Supabase" button.

```tsx
// web/src/components/panels/scout/ProfilePanel.tsx
import { useState } from 'react';
import type { ProfileData } from '../../../lib/types';

interface ProfilePanelProps {
  profile: ProfileData | null;
  loading: boolean;
  pulling: boolean;
  onPull: () => void;
}

export default function ProfilePanel({ profile, loading, pulling, onPull }: ProfilePanelProps) {
  if (loading) {
    return <div className="p-4 text-sm text-fg-muted">Loading profile...</div>;
  }

  if (!profile) {
    return <div className="p-4 text-sm text-fg-muted">No profile data. Click "Pull from Supabase" to sync.</div>;
  }

  const config = profile.site_config?.[0];

  return (
    <div className="space-y-3 p-4">
      {/* Header with pull button */}
      <div className="flex items-center justify-between">
        <h2 className="text-sm font-semibold text-fg font-display">Profile</h2>
        <button
          onClick={onPull}
          disabled={pulling}
          className="text-[10px] px-2 py-1 rounded bg-accent/15 text-accent hover:bg-accent/25 disabled:opacity-50"
        >
          {pulling ? 'Pulling...' : 'Pull from Supabase'}
        </button>
      </div>

      {/* Identity Section */}
      {config && <IdentitySection config={config} />}

      {/* Experience Section */}
      {profile.experience?.length > 0 && (
        <CollapsibleSection title="Experience" count={profile.experience.length}>
          {profile.experience.map((exp) => (
            <ExperienceCard key={exp.id} exp={exp} />
          ))}
        </CollapsibleSection>
      )}

      {/* Skills Section */}
      {profile.skill_categories?.length > 0 && (
        <CollapsibleSection title="Skills" count={profile.skill_categories.length}>
          {profile.skill_categories.map((cat) => (
            <SkillCategoryCard key={cat.id} category={cat} />
          ))}
        </CollapsibleSection>
      )}

      {/* Projects Section */}
      {profile.projects?.length > 0 && (
        <CollapsibleSection title="Projects" count={profile.projects.length}>
          {profile.projects.map((proj) => (
            <ProjectCard key={proj.id} project={proj} />
          ))}
        </CollapsibleSection>
      )}

      {/* Education Section */}
      {profile.education?.length > 0 && (
        <CollapsibleSection title="Education" count={profile.education.length}>
          {profile.education.map((edu) => (
            <EducationCard key={edu.id} education={edu} />
          ))}
        </CollapsibleSection>
      )}
    </div>
  );
}
```

Build out the sub-components (`IdentitySection`, `CollapsibleSection`, `ExperienceCard`, `SkillCategoryCard`, `ProjectCard`, `EducationCard`) in the same file or as separate components. Keep styling consistent with existing Scout panel components (zinc palette, small text, compact cards).

**Step 2: Verify compilation**

```bash
cd /home/rishav/soul/web && npx tsc --noEmit
```

**Step 3: Commit**

```bash
cd /home/rishav/soul
git add web/src/components/panels/scout/ProfilePanel.tsx
git commit -m "feat(web): add ProfilePanel component with 5 sections

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>"
```

---

## Task 15: Integrate Profile Panel into ScoutPanel

**Files:**
- Modify: `web/src/components/panels/ScoutPanel.tsx`

**Step 1: Add tab navigation**

Add a tab bar to ScoutPanel with two tabs: "Dashboard" (existing content) and "Profile" (new ProfilePanel).

```tsx
// In ScoutPanel.tsx:
import { useState } from 'react';
import ProfilePanel from './scout/ProfilePanel';
import { useProfile } from '../../hooks/useProfile';

// Inside ScoutPanel component:
const [activeTab, setActiveTab] = useState<'dashboard' | 'profile'>('dashboard');
const { profile, loading: profileLoading, pulling, pullFromSupabase } = useProfile();

// In JSX, add tab bar after the header:
<div className="flex gap-1 px-4 py-1 border-b border-border-subtle">
  <button
    onClick={() => setActiveTab('dashboard')}
    className={`text-xs px-2 py-1 rounded ${activeTab === 'dashboard' ? 'bg-elevated text-fg' : 'text-fg-muted hover:text-fg'}`}
  >
    Dashboard
  </button>
  <button
    onClick={() => setActiveTab('profile')}
    className={`text-xs px-2 py-1 rounded ${activeTab === 'profile' ? 'bg-elevated text-fg' : 'text-fg-muted hover:text-fg'}`}
  >
    Profile
  </button>
</div>

// Conditionally render based on active tab:
{activeTab === 'dashboard' ? (
  // Existing dashboard content (SyncStatus, Opportunities, etc.)
) : (
  <ProfilePanel
    profile={profile}
    loading={profileLoading}
    pulling={pulling}
    onPull={pullFromSupabase}
  />
)}
```

**Step 2: Build and verify**

```bash
cd /home/rishav/soul/web && npx vite build
```

Expected: Build succeeds.

**Step 3: Commit**

```bash
cd /home/rishav/soul
git add web/src/components/panels/ScoutPanel.tsx
git commit -m "feat(web): add Profile tab to ScoutPanel

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>"
```

---

## Task 16: Build, Test End-to-End, Final Commit

**Step 1: Rebuild Scout binary**

```bash
cd /home/rishav/soul/products/scout && go build -o scout ./cmd/scout
```

**Step 2: Rebuild frontend**

```bash
cd /home/rishav/soul/web && npx vite build
```

**Step 3: Rebuild and restart Soul**

```bash
cd /home/rishav/soul && go build -o soul ./cmd/soul
# Kill old process
fuser -k 3000/tcp 2>/dev/null || true
SOUL_HOST=0.0.0.0 ./soul serve &
```

**Step 4: Test scout__profile tool**

```bash
curl -s http://localhost:3000/api/tools/scout__profile/execute \
  -H 'Content-Type: application/json' \
  -d '{"input":{}}' | jq '.content[0].text' | jq '.site_config[0].name'
```

Expected: Returns your name.

**Step 5: Test scout__profile_pull tool**

```bash
curl -s http://localhost:3000/api/tools/scout__profile_pull/execute \
  -H 'Content-Type: application/json' \
  -d '{"input":{}}' | jq '.'
```

Expected: Returns pull summary with row counts.

**Step 6: Test scout__profile_push safety**

```bash
curl -s http://localhost:3000/api/tools/scout__profile_push/execute \
  -H 'Content-Type: application/json' \
  -d '{"input":{"confirm":false}}' | jq '.'
```

Expected: Returns safety warning, does NOT push.

**Step 7: Test UI**

Open browser → Soul → Scout panel → click "Profile" tab. Should see:
- Identity section with name, title, bio, location
- Experience timeline with roles, companies, dates
- Skills with categories and proficiency
- Projects with descriptions and tech stacks
- Education section

Click "Pull from Supabase" → should show "Pulling..." then refresh data.

**Step 8: Verify SSH tunnel and timer**

```bash
systemctl --user status profile-db-tunnel.service
systemctl --user list-timers | grep profile-sync
```

Expected: Tunnel active, timer scheduled for 03:00.

**Step 9: Clean up portfolio_backup from Gitea container**

After confirming all data is in the new profile-db:

```bash
ssh rishav@192.168.0.196 "docker exec titan-gitea-db psql -U gitea -c 'DROP DATABASE portfolio_backup;'"
```

**Step 10: Final commit if any remaining changes**

```bash
cd /home/rishav/soul
git status
# If any uncommitted changes:
git add -A && git commit -m "chore(scout): profile hub final cleanup

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>"
```

---

## Key Files Summary

| File | Action |
|------|--------|
| `products/scout/infra/docker-compose.profile-db.yml` | CREATE — container definition |
| `products/scout/infra/init-profile-db.sql` | CREATE — schema from pg_dump |
| `products/scout/infra/profile-sync-pull.sh` | CREATE — daily sync script |
| `~/.config/systemd/user/profile-db-tunnel.service` | CREATE — SSH tunnel |
| `~/.config/systemd/user/profile-sync-pull.service` | CREATE — sync service |
| `~/.config/systemd/user/profile-sync-pull.timer` | CREATE — daily timer |
| `products/scout/go.mod` | MODIFY — add pgx/v5 |
| `products/scout/internal/profiledb/types.go` | CREATE — profile data types |
| `products/scout/internal/profiledb/client.go` | CREATE — PG client + pull/push |
| `products/scout/internal/service.go` | MODIFY — 3 new tools |
| `web/src/lib/types.ts` | MODIFY — profile types |
| `web/src/hooks/useProfile.ts` | CREATE — profile data hook |
| `web/src/components/panels/scout/ProfilePanel.tsx` | CREATE — profile UI |
| `web/src/components/panels/ScoutPanel.tsx` | MODIFY — add Profile tab |
| `~/.soul/scout/config.json` | MODIFY — add profile_db section |

## Verification Checklist

1. `titan-profile-db` container running on titan-pc, port 5434
2. SSH tunnel active: titan-pi:5434 → titan-pc:5434
3. All 13 tables seeded with Supabase data
4. `scout__profile` returns full profile JSON
5. `scout__profile_pull` syncs Supabase → local PG
6. `scout__profile_push` requires confirmation, pushes local → Supabase
7. Daily timer scheduled at 03:00
8. Scout UI "Profile" tab shows all 5 sections
9. "Pull from Supabase" button works
10. `portfolio_backup` dropped from Gitea container
