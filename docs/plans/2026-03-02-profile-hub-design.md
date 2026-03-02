# Profile Hub — Centralized Profile Database for Scout

## Problem

Professional profile data is scattered across Supabase (website), LinkedIn, Naukri, GitHub, and other platforms. No single source of truth. No automated sync. Current Supabase backup sits inside the Gitea PostgreSQL container with no isolation or automation.

## Decision

Local PostgreSQL on titan-pc is the **primary source of truth**. Supabase is a downstream consumer (powers the portfolio website). Scout gains a Profile panel for viewing the unified profile, and tools for pull/push sync.

## Architecture

```
                    +-----------------------+
                    |   Supabase Cloud      |
                    |   (website backend)   |
                    +-----------+-----------+
                          ^           |
                     push |           | pull (daily)
                    (manual)          v
+----------+     +------------------------+
| Scout on |<--->| titan-profile-db       |
| titan-pi | SSH | PostgreSQL 16          |
|          | tun | titan-pc:5434          |
+----------+     | Volume: profile-db-data|
                  +------------------------+
                          |
                    Scout reads profile
                    for sync/sweep/UI
```

## Infrastructure

### Container: `titan-profile-db`

- **Image:** `postgres:16`
- **Port:** `127.0.0.1:5434:5432` on titan-pc (localhost only)
- **Volume:** `profile-db-data` (Docker named volume on ext4)
- **Database:** `profile`
- **User:** `profile` (dedicated, not shared with Gitea)
- **Network:** Isolated from Gitea containers

### SSH Tunnel

From titan-pi: `ssh -L 5434:127.0.0.1:5434 rishav@192.168.0.196 -N`
Managed as a systemd service on titan-pi for persistence.

### Schema

Mirror all 13 Supabase tables as-is:
- site_config, experience, skill_categories, projects
- education, testimonials, brands, services
- case_studies, chat_questions, faqs, stats_dashboard, skill_radar_data

No schema changes. Use existing column definitions including jsonb fields (achievements, tech_stack, etc.).

### Seed

One-time `pg_dump` from `portfolio_backup` (Gitea container) -> restore into `titan-profile-db`. Then drop `portfolio_backup` database from Gitea container.

## Sync

### Supabase -> Local PG (Daily Pull)

- **Trigger:** systemd timer on titan-pc, daily at 3:00 AM
- **Method:** Shell script using Supabase PostgREST API (GET all rows from each table) -> upsert into local PG
- **Conflict:** Supabase wins on pull (overwrites local with cloud data)
- **Logging:** `/var/log/profile-sync.log`

### Local PG -> Supabase (On-Demand Push)

- **Trigger:** Manual only, via `scout__profile_push` tool
- **Method:** Read from local PG -> upsert to Supabase via PostgREST API
- **Safety:** No automation. User must explicitly trigger. Protects live website from corrupt data.
- **Future:** Can be automated once confidence in local DB is established.

## Scout Integration

### New Tools

| Tool | Purpose |
|------|---------|
| `scout__profile` | Return full profile from local PG as structured JSON |
| `scout__profile_pull` | Pull Supabase cloud -> local PG (what daily cron does) |
| `scout__profile_push` | Push local PG -> Supabase cloud (manual, careful) |

### Config Changes

`~/.soul/scout/config.json` gains:
```json
{
  "profile_db": {
    "host": "127.0.0.1",
    "port": 5434,
    "database": "profile",
    "user": "profile",
    "password": "<from-vaultwarden>"
  }
}
```

### Scout Reads from Local PG

Replace `supabase.NewClient()` in sync checker and sweep with local PG queries. Supabase client remains for push/pull sync operations only.

## Frontend — Profile Panel

New tab in Scout panel (alongside Sync Status, Opportunities, etc.):

### Sections

1. **Identity** — Name, title, bio, location, social links, contact
2. **Experience** — Timeline of roles with company, dates, achievements, tech stack
3. **Skills** — Categories with skill names and proficiency levels
4. **Projects** — Portfolio projects with descriptions and tech
5. **Education** — Degrees, certifications

### Behavior

- Read-only view (editing via psql/Supabase for now)
- "Last synced" timestamp per section
- "Pull from Supabase" button triggers `scout__profile_pull`
- Collapsible sections, compact display

## Platforms (Future)

The profile hub is extensible to any platform:
- LinkedIn, Naukri, GitHub (initial targets)
- Indeed, Wellfound, Instahyre, freelance sites (later)

Each platform gets a "push adapter" that transforms the canonical profile schema into the platform's expected format. Scout's existing browser automation (rod) handles the actual updates.

## Out of Scope (for now)

- Inline editing in the Scout UI
- Automated push to Supabase
- Platform-specific push adapters (LinkedIn, Naukri, GitHub)
- Multi-user support
