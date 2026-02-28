# Scout Product — Design Document

**Created:** 2026-02-28
**Status:** Approved
**Product name:** Scout
**Location:** `products/scout/`
**Binary:** `products/scout/scout`

---

## Problem

Rishav has profiles on 7+ job platforms (LinkedIn, Naukri, Indeed, Wellfound, Instahyre, personal website, GitHub) plus upcoming freelance platforms. Currently:

1. **No content sync** — Profile data drifts between platforms. No way to detect when Supabase (source of truth) differs from what's live.
2. **No opportunity monitoring** — Inbound matches, recruiter messages, and application status changes go unchecked.
3. **No tailored resume generation** — One static resume for all roles. No way to generate role-variant PDFs from Supabase data.
4. **No application tracking** — No centralized log of applications, follow-ups, or conversion metrics.

## Solution

**Scout**: A Soul product plugin (gRPC binary) that syncs content, monitors opportunities, generates tailored resumes, and tracks applications. Runs as a background service alongside the Soul server, with a dedicated dashboard in the Soul web UI.

---

## Architecture

```
Soul Server (:3000)
    │
    ├── gRPC over Unix socket
    │
    ▼
Scout Binary (products/scout/)
    │
    ├── Rod (headless Chrome)
    │   ├── Browser profiles (~/.soul/scout/profiles/<platform>/)
    │   ├── Platform automation (sync, sweep)
    │   └── PDF generation (print-to-PDF)
    │
    ├── Supabase REST API (read-only)
    │   └── Profile data (experience, skills, projects, site_config)
    │
    └── Local data store (~/.soul/scout/data.json)
        ├── Sync results (per-platform drift reports)
        ├── Sweep results (opportunities, messages)
        ├── Application tracker (status, follow-ups)
        └── Metrics (weekly aggregates)
```

---

## gRPC Tools (6)

### Tool 1: `setup`
**Type:** Unary
**Purpose:** Opens a visible (non-headless) Chrome browser for the user to log into a specific platform. Saves the browser profile for future headless use.

**Input:**
```json
{
  "platform": "linkedin|naukri|indeed|wellfound|instahyre"
}
```

**Output:**
```json
{
  "success": true,
  "message": "LinkedIn session saved. Profile stored at ~/.soul/scout/profiles/linkedin/"
}
```

**Behavior:**
1. Launch Chrome in visible mode with the platform's profile directory
2. Navigate to the platform's login page
3. Wait for user to complete login (detect logged-in state)
4. Close browser, profile cookies are persisted to disk
5. Return success

---

### Tool 2: `sync`
**Type:** Streaming
**Purpose:** Pulls profile data from Supabase, opens each platform in headless Rod (using saved profile), compares content. Reports drift.

**Input:**
```json
{
  "platforms": ["all"]
}
```
(Optional: specific platform list)

**Stream Events:**
- `progress` — Per-platform: "Checking LinkedIn... (2/7)"
- `finding` — Drift detected: severity=warning, title="LinkedIn headline outdated", evidence="Current: 'Data Engineer' → Expected: 'AI Engineer'"
- `complete` — Summary: "7 platforms checked, 2 with drift"

**Supabase Tables Used:**
| Table | Fields |
|-------|--------|
| site_config | name, title, bio, social links |
| experience | role, company, period, achievements |
| skills | categories, skill names, levels |
| projects | title, description, tech stack |

**Per-Platform Checks:**
| Platform | What to Verify | Method |
|----------|---------------|--------|
| rishavchatterjee.com | Resume page matches Supabase | HTTP fetch + text compare |
| LinkedIn | Headline, About, Experience, Skills | Rod headless + saved profile |
| GitHub README | Profile README matches identity | GitHub API fetch |
| Naukri | Headline, skills, experience | Rod headless + saved profile |
| Indeed | Profile summary, skills | Rod headless + saved profile |
| Wellfound | Bio, skills, experience | Rod headless + saved profile |
| Instahyre | Profile completeness | Rod headless + saved profile |

**Storage:** Results saved to `~/.soul/scout/data.json` under `sync` key with timestamp.

---

### Tool 3: `sweep`
**Type:** Streaming
**Purpose:** Opens each job portal in headless Rod (saved session), extracts new matches, recruiter messages, application status changes.

**Input:**
```json
{
  "platforms": ["all"]
}
```

**Stream Events:**
- `progress` — Per-platform: "Sweeping LinkedIn... (1/5)"
- `finding` — New opportunity: severity=info, title="AI Platform Engineer — Acme Corp", evidence="95% match, Remote, LinkedIn"
- `finding` — Message: severity=warning, title="Recruiter message from TechCo", evidence="Interested in AI Lead role — respond within 24h"
- `complete` — Summary: "5 platforms swept, 8 opportunities, 2 messages"

**Per-Platform Sweep:**
| Platform | What to Check |
|----------|--------------|
| LinkedIn Jobs | Recommendations, saved searches, InMail |
| Naukri | Recruiter views, new matches, responses |
| Instahyre | Inbound company interest, matches |
| Wellfound | Startup matches, messages |
| Indeed | Status updates, recommendations |

**Sweep Process (per platform):**
1. Launch Rod headless with platform's saved profile
2. Navigate to notifications/messages section
3. Extract new items since last sweep (compare against stored state)
4. Navigate to job recommendations / matches
5. Extract company, role, match indicators
6. Store results, stream findings

**Storage:** Results saved to `~/.soul/scout/data.json` under `sweep` key with timestamp.

---

### Tool 4: `generate`
**Type:** Unary
**Purpose:** Generates a tailored resume + cover note PDF for a specific role variant and company.

**Input:**
```json
{
  "variant": "A",
  "company": "Acme Corp",
  "role": "AI Platform Engineer",
  "job_url": "https://linkedin.com/jobs/...",
  "specific_thing": "their multi-agent orchestration platform"
}
```

**Output:**
```json
{
  "success": true,
  "artifacts": [
    {"type": "pdf", "path": "~/.soul/scout/drafts/acme-corp-A-2026-02-28-resume.pdf"},
    {"type": "pdf", "path": "~/.soul/scout/drafts/acme-corp-A-2026-02-28-cover.pdf"}
  ]
}
```

**Role Variants (7):**
| Variant | Target Role |
|---------|-------------|
| A | AI Platform Architect / Solutions Architect |
| B | GenAI Engineer / LLM Engineer |
| C | Senior AI Engineer |
| D | AI Manager / AI Lead |
| E | AI Consultant / Freelance |
| F | AI Researcher |
| G | Senior Data Engineer (AI-focused) |

**Each variant adjusts:**
- Headline — role-specific title
- Summary — rewritten emphasis (same facts, different framing)
- Bullet ordering — which experience bullets lead
- Project highlights — which Soul projects to feature
- Skills emphasis — top skills for that role type
- Cover note — role-specific hook with company/role/specific_thing filled in

**Generation Pipeline:**
```
Supabase → Pull experience, skills, projects, site_config
    ↓
Variant template → Apply headline, summary, bullet order, skills
    ↓
HTML template → Render styled resume + cover note
    ↓
Rod print-to-PDF → Capture as A4 PDF
    ↓
Output → ~/.soul/scout/drafts/<company>-<variant>-<date>-resume.pdf
         ~/.soul/scout/drafts/<company>-<variant>-<date>-cover.pdf
```

**PDF Requirements:**
- Clean, ATS-friendly layout (no multi-column, no graphics that break parsers)
- Consistent with rishavchatterjee.com/resume styling
- Print-optimized (A4, proper margins)

**Variant definitions source:** `docs/profile/resume-variants.md`

---

### Tool 5: `track`
**Type:** Unary
**Purpose:** CRUD for application entries.

**Input (add):**
```json
{
  "action": "add",
  "company": "Acme Corp",
  "role": "AI Platform Engineer",
  "platform": "linkedin",
  "variant": "A",
  "notes": "Custom cover note sent"
}
```

**Input (update):**
```json
{
  "action": "update",
  "id": "app-001",
  "status": "interview_scheduled",
  "follow_up": "2026-03-03",
  "notes": "Phone screen with hiring manager"
}
```

**Input (list):**
```json
{
  "action": "list",
  "status": "active"
}
```

**Application Statuses:**
- `applied` — Submitted, waiting
- `viewed` — Viewed by recruiter
- `interview_scheduled` — Interview booked
- `interview_done` — Completed, awaiting feedback
- `offer` — Received offer
- `rejected` — Rejected
- `withdrawn` — Withdrawn by candidate
- `follow_up_sent` — Follow-up sent after no response

**Storage:** `~/.soul/scout/data.json` under `applications` key.

---

### Tool 6: `report`
**Type:** Unary
**Purpose:** Returns structured JSON for the Scout dashboard in the Soul web UI.

**Input:**
```json
{
  "period": "today"
}
```
(Options: "today", "week", "month")

**Output (structured_json):**
```json
{
  "sync": {
    "last_run": "2026-02-28T21:00:00Z",
    "platforms_checked": 7,
    "in_sync": 5,
    "drift": 2,
    "details": [
      {"platform": "linkedin", "status": "drift", "issues": ["Headline outdated", "Missing skill: Claude Code"]},
      {"platform": "naukri", "status": "synced"}
    ]
  },
  "sweep": {
    "last_run": "2026-02-28T21:05:00Z",
    "new_opportunities": 8,
    "messages": 2,
    "status_changes": 1,
    "opportunities": [
      {"company": "Acme Corp", "role": "AI Platform Engineer", "platform": "linkedin", "match": "95%", "url": "..."}
    ]
  },
  "applications": {
    "total": 15,
    "active": 8,
    "by_status": {"applied": 5, "interview_scheduled": 2, "offer": 1},
    "recent": [...]
  },
  "metrics": {
    "week": {"applied": 8, "responses": 3, "interviews": 2, "offers": 0},
    "month": {"applied": 25, "responses": 10, "interviews": 5, "offers": 1}
  },
  "follow_ups": [
    {"company": "TechCo", "role": "AI Lead", "due": "2026-03-03", "action": "Phone screen"}
  ]
}
```

---

## Data Storage

**Location:** `~/.soul/scout/`

```
~/.soul/scout/
├── config.json              # Supabase URL, anon key, platform list
├── data.json                # All scout data (sync, sweep, tracker, metrics)
├── profiles/                # Chrome browser profiles (per platform)
│   ├── linkedin/
│   ├── naukri/
│   ├── indeed/
│   ├── wellfound/
│   └── instahyre/
├── drafts/                  # Generated PDFs
│   ├── acme-corp-A-2026-02-28-resume.pdf
│   └── acme-corp-A-2026-02-28-cover.pdf
└── reports/                 # Historical report snapshots (optional)
```

---

## Frontend: ScoutPanel

New panel in the Soul web UI (`web/src/components/panels/ScoutPanel.tsx`).

**Dashboard sections:**

1. **Sync Status** — Platform cards with in-sync/drift badges, last checked time, drill-down to specific issues
2. **Opportunities** — Table from latest sweep: company, role, platform, match score. Action buttons: Apply (triggers generate), Dismiss, Save
3. **Application Tracker** — Table or kanban of tracked applications by status. Inline status updates.
4. **Weekly Metrics** — Bar/line charts: applications/week, response rate, interview rate
5. **Follow-ups Due** — Sorted list of upcoming follow-up dates with action reminders

**Sub-components:**
```
web/src/components/panels/scout/
├── SyncStatus.tsx
├── Opportunities.tsx
├── ApplicationTracker.tsx
├── WeeklyMetrics.tsx
└── FollowUps.tsx
```

---

## File Structure

```
products/scout/
├── cmd/
│   └── scout/
│       └── main.go              # gRPC server, --socket flag, signal handling
├── internal/
│   ├── service.go               # ProductService implementation
│   ├── browser/
│   │   ├── pool.go              # Rod browser pool + headless management
│   │   └── profiles.go          # Chrome profile persistence
│   ├── sync/
│   │   ├── checker.go           # Orchestrates per-platform sync checks
│   │   └── platforms.go         # Platform-specific selectors/extractors
│   ├── sweep/
│   │   ├── monitor.go           # Orchestrates per-platform sweeps
│   │   └── platforms.go         # Platform-specific selectors/extractors
│   ├── generate/
│   │   ├── resume.go            # Variant application + HTML rendering
│   │   ├── cover.go             # Cover note generation
│   │   ├── pdf.go               # Rod print-to-PDF
│   │   └── variants.go          # A-G variant definitions
│   ├── tracker/
│   │   └── store.go             # Application CRUD
│   ├── supabase/
│   │   └── client.go            # Supabase REST client (read-only)
│   └── data/
│       └── store.go             # Central data store
├── templates/
│   ├── resume.html              # Base resume HTML template
│   └── cover.html               # Cover note HTML template
├── go.mod
├── go.sum
├── Makefile
└── CLAUDE.md
```

---

## Dependencies

| Dependency | Purpose |
|-----------|---------|
| `github.com/go-rod/rod` | Browser automation (sweep, sync) + PDF generation |
| `google.golang.org/grpc` | gRPC server (product interface) |
| `google.golang.org/protobuf` | Protobuf message handling |
| Soul proto (`proto/soul/v1/product.proto`) | Reuse existing ProductService definition |
| Supabase REST API | Read profile data (HTTP, no SDK) |
| Chromium | Required by Rod (auto-downloads or system-installed) |

---

## Browser Automation Strategy

**Profile Persistence:**
- Each platform gets its own Chrome user data directory at `~/.soul/scout/profiles/<platform>/`
- User logs in once via `setup` tool (visible browser)
- Subsequent `sync` and `sweep` runs use headless mode with the saved profile
- If session expires, `setup` needs to be re-run for that platform

**Anti-Bot Mitigation:**
- Use real Chrome user agent (Rod default)
- Add random delays between page actions (1-3s)
- Navigate like a human (don't jump directly to data endpoints)
- Respect robots.txt where applicable
- Rate limit: max 1 platform sweep per 30 seconds

**Platform Selectors:**
Each platform has a `platforms.go` file defining:
- Login detection selectors (how to know we're logged in)
- Data extraction selectors (where to find matches, messages, etc.)
- These will need maintenance as platforms update their UIs

---

## Scheduling (Phase 2)

When the Soul planner supports scheduled tasks:
- Daily at 9pm IST: Auto-trigger `sync` + `sweep`
- Store results, update dashboard
- Flag urgent items (messages needing response within 24h)
- Queue resume generation for user-approved opportunities

---

## What This Does NOT Do

- Does not auto-apply to jobs (requires explicit user action)
- Does not send messages on behalf of the user (only monitors)
- Does not modify Supabase data (read-only)
- Does not store platform passwords (browser profiles only)
- Does not replace manual networking/outreach

---

## Success Criteria

1. `setup` successfully saves login sessions for all 5 job platforms
2. `sync` detects content drift across 7 platforms within 5 minutes
3. `sweep` extracts new opportunities from 5 platforms within 10 minutes
4. `generate` produces ATS-friendly PDF resume + cover note in < 30 seconds
5. `track` maintains accurate application status with follow-up dates
6. `report` returns structured data that renders as a functional Scout dashboard
7. Scout dashboard shows all 5 sections with real data

---

## Reference Documents

- `docs/plans/scout-strategy.md` — Daily SCOUT routine, target roles, platform list, metrics
- `docs/profile/resume-variants.md` — 7 role variants (A-G) with headlines, summaries, cover notes
