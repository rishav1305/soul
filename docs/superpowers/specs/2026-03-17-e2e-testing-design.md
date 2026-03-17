# Soul-Chat E2E Testing & Observe Integration

**Date:** 2026-03-17
**Status:** Approved
**Approach:** Playwright Test + Custom Observe Reporter

## Problem

Soul-chat has 14 chat components, 21 product integrations, WebSocket streaming, multi-session support, and responsive layouts — all with zero automated E2E tests. Existing ad-hoc Playwright scripts on titan-pc (`~/soul-e2e/`) take screenshots without structured assertions, producing false positives where AI claims "tested" but features break in production. No test results flow into Soul Observe, leaving the system blind to frontend health.

## Solution

Playwright Test suite in `tests/e2e/` with TypeScript, state-based assertions (no timeout waits), a custom reporter that pushes structured results to Soul Observe, and a new "Tested" 7th pillar in the Observe dashboard.

## Architecture

```
titan-pc (browser host)                    raspberry pi (services host)
┌──────────────────────┐                   ┌─────────────────────────┐
│ Playwright Test       │──── HTTP/WS ────→│ soul-chat :3002         │
│ tests/e2e/            │                   │ soul-tasks :3004        │
│                       │                   │ soul-observe :3010      │
│ observe-reporter.ts ──┼── POST /api/ ───→│   /api/observe/e2e      │
│                       │   observe/e2e     │   → observe.db          │
│ cron (every 2h)       │                   │   → pillars response    │
│ make e2e (on-demand)  │                   │   → ObservePage Tested  │
└──────────────────────┘                   └─────────────────────────┘
```

## Test Suite Structure

```
tests/e2e/
├── package.json                  # @playwright/test dependency
├── tsconfig.json                 # TypeScript config for tests
├── playwright.config.ts          # Config: baseURL, projects, reporters
├── fixtures/
│   ├── auth.ts                   # Auth fixture — token injection
│   └── soul.ts                   # Extended fixtures with page helpers
├── helpers/
│   ├── selectors.ts              # Type-safe data-testid map
│   └── wait.ts                   # State-based waiters (no timeouts)
├── suites/
│   ├── smoke.spec.ts             # 8-point health check
│   ├── chat/
│   │   ├── send-message.spec.ts
│   │   ├── session-management.spec.ts
│   │   ├── model-selection.spec.ts
│   │   ├── thinking-toggle.spec.ts
│   │   ├── tool-use-loop.spec.ts
│   │   ├── edit-retry.spec.ts
│   │   ├── attachments.spec.ts
│   │   ├── product-routing.spec.ts
│   │   └── connection-resilience.spec.ts
│   ├── layout/
│   │   ├── sidebar.spec.ts
│   │   ├── topbar.spec.ts
│   │   ├── responsive.spec.ts    # Desktop + Mobile viewports
│   │   └── navigation.spec.ts
│   ├── observe/
│   │   ├── pillars.spec.ts
│   │   ├── overview.spec.ts
│   │   └── tail.spec.ts
│   └── api/
│       ├── health.spec.ts
│       ├── models.spec.ts
│       └── sessions.spec.ts
├── reporters/
│   └── observe-reporter.ts       # Custom reporter → POST /api/observe/e2e
└── scripts/
    └── run-scheduled.sh          # Cron entry point
```

## Test Coverage Matrix

### Smoke (smoke.spec.ts)

| Check | Assertion |
|-------|-----------|
| Page loads | HTTP 200, `#root` has children |
| No JS errors | `pageerror` event count === 0 |
| React hydrated | `[data-testid="chat-page"]` visible |
| Sidebar renders | `[data-testid="sidebar-nav"]` exists |
| API reachable | `fetch /api/health` returns 200 |
| WebSocket connects | WS open event within 5s |
| Auth valid | No `[data-testid="auth-gate"]` blocking UI |
| Model list loads | `fetch /api/models` returns non-empty array |

### Chat Suite

| Test | Assertions |
|------|------------|
| send-message | User bubble appears → stream starts (`.streaming-cursor` visible) → assistant bubble has content.length > 0 → `.streaming-cursor` gone. **Note:** add `data-testid="streaming-indicator"` to the cursor span in `MessageBubble.tsx` during implementation |
| session-management | Create → appears in list. Switch → messages change. Rename → title updates. Delete → removed. Refresh → persists |
| model-selection | Dropdown shows API models. Select → localStorage updates. Send → WS payload includes model. Refresh → persists |
| thinking-toggle | Toggle on → send → ThinkingBlock appears with content. Toggle off → no ThinkingBlock. Persists across messages |
| tool-use-loop | Bind product → send triggering message → tool.call event → tool.complete → assistant continues. Up to 5 rounds |
| edit-retry | Edit user message → new response streams. Retry → same message re-sent. Both preserve model + thinking options |
| attachments | Attach image → preview in input. Send → attachment in bubble. Attach text → content included |
| product-routing | Select product → session.productSet event. Badge visible. Tool calls dispatch to correct API |
| connection-resilience | Kill WS → ConnectionBanner shows disconnected. Reconnect → banner clears. Queued message sends |

### Layout Suite

| Test | Assertions |
|------|------------|
| sidebar | Expanded: nav items visible. Collapse → rail mode. Expand → full. Product icons present |
| topbar | New/Running/Unread/History buttons exist and clickable. Session count badge updates |
| responsive | Desktop (1920x1080): sidebar + chat side-by-side. Mobile (390x844): sidebar hidden, hamburger works, input doesn't overflow |
| navigation | All 14 routes load without JS errors. Back/forward browser navigation works. Parametric routes (`tasks/:id`, `tutor/drill/:id`, `projects/:id`) tested with fixture-created entities |

### Observe Suite

| Test | Assertions |
|------|------------|
| pillars | 7 pillar cards render (including Tested). Each has status. Constraint rows show values |
| overview | Uptime, sessions, messages, cost cards render with numeric values |
| tail | Events list renders. Newest-first ordering. Limit parameter works |

### API Suite

| Test | Assertions |
|------|------------|
| health | All service health endpoints return 200 |
| models | `/api/models` returns array with valid model IDs matching `^claude-` |
| sessions | CRUD: create → list includes → rename → get shows new name → delete → list excludes |

**Total: ~25 test files, ~80+ individual assertions, 0 screenshot-only checks.**

## False Positive Prevention

### 1. State-based waits, not timeouts

```typescript
// WRONG — passes even if message never arrives
await page.waitForTimeout(8000);

// RIGHT — fails if element never appears with content
await expect(page.getByTestId('message-bubble').last())
  .toHaveText(/.+/, { timeout: 10000 });
```

### 2. Assertions on state, not presence

```typescript
// WRONG — element exists but might be empty
const el = await page.$('[data-testid="message-bubble"]');
assert(!!el);

// RIGHT — asserts actual content and completion
await expect(page.getByTestId('message-bubble').last()).not.toBeEmpty();
await expect(page.getByTestId('streaming-indicator')).toBeHidden(); // requires data-testid added to MessageBubble.tsx
```

### 3. Structured test output

Playwright Test produces structured JSON/JUnit results. The Observe reporter parses these programmatically. A test either passes its `expect()` or it doesn't. No AI interpretation of screenshots or console.log text.

### 4. Mandatory trace on failure

```typescript
use: {
  trace: 'retain-on-failure',
  screenshot: 'only-on-failure',
}
```

Failure produces a Playwright trace file with DOM snapshots, network requests, and action timeline. Viewable via `npx playwright show-trace`.

### 5. Flaky != passing

```typescript
retries: 1,
```

If a test passes on retry, it's flagged as **flaky** in Observe (yellow, not green). Flaky tests are tracked as a constraint with threshold 0.

### 6. WebSocket assertion helpers

```typescript
// helpers/wait.ts — intercepts actual WS frames
async function waitForWSMessage(page: Page, type: string, timeout = 10000): Promise<WSMessage> {
  // Listens to real WebSocket frames, returns parsed message
  // Fails with timeout error if message type never arrives
}
```

### 7. API contract assertions

```typescript
// Validate response shape, not just status code
const models = await response.json();
expect(models).toBeInstanceOf(Array);
expect(models.length).toBeGreaterThan(0);
expect(models[0]).toMatch(/^claude-/);
```

## Observe Integration

### New Endpoint

```
POST /api/observe/e2e     — receive test run results from reporter
GET  /api/observe/e2e      — query test history and current status
```

**Routing:** The reporter POSTs directly to soul-observe at `http://100.116.180.112:3010/api/e2e` (not through the chat proxy at :3002). Since the reporter is a Node.js process (not a browser), CORS is not a concern — just register the `POST /api/e2e` route handler on the observe server. The chat proxy at `/api/observe/` forwards only `GET` requests for the frontend dashboard; the frontend fetches e2e data via `GET /api/observe/e2e` which proxies to `GET /api/e2e` on observe:3010.

### Backend Integration Changes

The observe server currently has **no SQLite dependency** — it uses `metrics.Aggregator` with file-based event logs. This spec adds SQLite infrastructure to the observe server:

- **Add `database/sql` + `go-sqlite3` dependency** to observe server
- **Initialize `observe.db`** on startup with `e2e_runs` and `e2e_tests` tables (see schema below)
- **`POST /api/e2e` handler**: Validates payload, inserts into `e2e_runs` + `e2e_tests` tables
- **`GET /api/e2e` handler**: Returns latest runs with test details (query params: `limit`, `viewport`)
- **Go `pillarResult` struct** (`handlers.go`): Add `Total int` and `Description string` JSON fields — required by frontend `ObservePillar` type. Update comment from "6 architectural pillars" to "7 architectural pillars"
- **`handlePillars`**: Add `buildTestedPillar(db *sql.DB)` function that queries the most recent `e2e_runs` row per viewport, computes constraint statuses (smoke rate, chat rate, flaky count, run age), and returns a `pillarResult` alongside the existing 6 pillars

### Frontend Integration Changes

The following existing code must be updated to support the 7th pillar:

- **`types.ts`** (generated): Add `'tested'` to `ObserveTab` union type. Add `E2ERunResult`, `E2ESuiteResult`, `E2ETestResult` interfaces matching the data model below
- **`ObservePage.tsx`**: Add `'tested'` to the `tabs` array. Update pillar strip grid from `lg:grid-cols-6` to `lg:grid-cols-7`. Add conditional render branch for `activeTab === 'tested'` that renders new `TestedTab` component (not the standard `PillarTab` constraint-row layout — the Tested tab has richer UI with run history, suite breakdown, and trend lines)
- **New `TestedTab.tsx` component**: Receives e2e run data from useObserve, renders run history table, suite breakdown, trend chart, failure details, desktop/mobile comparison
- **`useObserve.ts`**: Add `'tested'` case in `fetchTab` switch to fetch from `GET /api/observe/e2e`

### Data Model

```typescript
interface E2ERunResult {
  runId: string;
  timestamp: string;              // ISO 8601
  trigger: "manual" | "scheduled";
  duration: number;               // total run time ms
  suites: E2ESuiteResult[];
  summary: {
    total: number;
    passed: number;
    failed: number;
    flaky: number;
    skipped: number;
    passRate: number;             // 0-100
  };
  viewport: "desktop" | "mobile"; // determined from PLAYWRIGHT_PROJECT env var or Playwright project name
}

interface E2ESuiteResult {
  name: string;
  status: "passed" | "failed" | "flaky" | "skipped";
  duration: number;
  tests: E2ETestResult[];
}

interface E2ETestResult {
  name: string;
  status: "passed" | "failed" | "flaky";
  duration: number;
  error?: string;                 // assertion message on failure
  trace?: string;                 // path to trace file
}
```

### SQLite Schema (observe.db)

```sql
CREATE TABLE e2e_runs (
  id TEXT PRIMARY KEY,
  timestamp TEXT NOT NULL,
  trigger TEXT NOT NULL,
  duration INTEGER NOT NULL,
  viewport TEXT NOT NULL,
  total INTEGER NOT NULL,
  passed INTEGER NOT NULL,
  failed INTEGER NOT NULL,
  flaky INTEGER NOT NULL,
  skipped INTEGER NOT NULL,
  pass_rate REAL NOT NULL
);

CREATE TABLE e2e_tests (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  run_id TEXT NOT NULL REFERENCES e2e_runs(id),
  suite TEXT NOT NULL,
  name TEXT NOT NULL,
  status TEXT NOT NULL,
  duration INTEGER NOT NULL,
  error TEXT,
  trace TEXT
);
```

### Tested Pillar (7th Pillar)

| Constraint | Threshold | Warn | Fail |
|-----------|-----------|------|------|
| Smoke pass rate | 100% | <100% | <87.5% |
| Chat suite pass rate | >95% | <95% | <80% |
| Layout suite pass rate | 100% | <100% | <90% |
| Flaky test count | 0 | >0 | >3 |
| Last run age | <2h | >2h | >6h | (most recent of either viewport) |
| Mobile viewport pass rate | >95% | <95% | <80% |

### Observe Dashboard — Tested Tab

- **Run history** — last 20 runs with pass/fail/flaky counts, timestamp, trigger type
- **Suite breakdown** — expandable rows per suite showing individual test results
- **Trend line** — pass rate over last 7 days
- **Failure details** — failed test name + assertion error + trace link
- **Desktop vs Mobile** — side-by-side pass rates

## Execution Infrastructure

### Playwright Config

```typescript
import { defineConfig } from '@playwright/test';

export default defineConfig({
  testDir: './suites',
  timeout: 60000,
  retries: 1,
  reporter: [
    ['list'],
    ['html', { open: 'never' }],
    ['./reporters/observe-reporter.ts'],
  ],
  projects: [
    {
      name: 'desktop',
      use: { viewport: { width: 1920, height: 1080 } },
    },
    {
      name: 'mobile',
      use: { viewport: { width: 390, height: 844 }, isMobile: true },
    },
  ],
  use: {
    baseURL: 'http://100.116.180.112:3002',
    trace: 'retain-on-failure',
    screenshot: 'only-on-failure',
    ignoreHTTPSErrors: true,
  },
});
```

### Auth Fixture

```typescript
import { test as base } from '@playwright/test';

export const test = base.extend({
  page: async ({ page }, use) => {
    const token = process.env.SOUL_V2_AUTH_TOKEN;
    if (!token) throw new Error('SOUL_V2_AUTH_TOKEN env var is required');
    await page.goto('/chat');
    const authGate = page.getByTestId('auth-token-input');
    if (await authGate.isVisible({ timeout: 3000 }).catch(() => false)) {
      await authGate.fill(token);
      await page.getByTestId('auth-submit').click();
      await page.waitForURL(/\/chat/);
    }
    await use(page);
  },
});
```

### On-Demand (Makefile)

```makefile
e2e:
	ssh titan-pc "cd ~/soul-v2/tests/e2e && npx playwright test"

e2e-smoke:
	ssh titan-pc "cd ~/soul-v2/tests/e2e && npx playwright test suites/smoke.spec.ts"

e2e-chat:
	ssh titan-pc "cd ~/soul-v2/tests/e2e && npx playwright test suites/chat/"

e2e-mobile:
	ssh titan-pc "cd ~/soul-v2/tests/e2e && npx playwright test --project=mobile"
```

### Scheduled (Cron on titan-pc)

```bash
# /etc/crontab or crontab -e
0 */2 * * * rishav cd ~/soul-v2/tests/e2e && ./scripts/run-scheduled.sh >> /tmp/soul-e2e-cron.log 2>&1
```

```bash
#!/bin/bash
# scripts/run-scheduled.sh
set -euo pipefail

cd "$(dirname "$0")/.."

# Pull latest tests
git pull --ff-only origin master

# Install deps if needed
npm ci --quiet

# Run desktop suite — reporter infers viewport from PLAYWRIGHT_PROJECT env var
PLAYWRIGHT_PROJECT=desktop SOUL_V2_AUTH_TOKEN="$SOUL_V2_AUTH_TOKEN" npx playwright test --project=desktop

# Run mobile suite
PLAYWRIGHT_PROJECT=mobile SOUL_V2_AUTH_TOKEN="$SOUL_V2_AUTH_TOKEN" npx playwright test --project=mobile
```

### Repo Sync

Tests live in `tests/e2e/` in the soul-v2 repo. Titan-pc clones the same repo. The scheduled script runs `git pull` before each run so tests stay current with code changes.

## Migration from soul-e2e

The existing `~/soul-e2e/test-runner.js` smoke action maps directly to `smoke.spec.ts`. The `assert` action maps to Playwright's `expect()`. The `visual-audit.js` and `sidebar-test.js` scripts map to their respective spec files. After the new suite is stable, `~/soul-e2e/` on titan-pc can be archived.

## Implementation Order

1. Scaffold `tests/e2e/` with config, fixtures, helpers
2. Port smoke checks from test-runner.js → smoke.spec.ts
3. Implement chat suite (send-message first, then others)
4. Implement layout + responsive suite
5. Build observe-reporter.ts
6. Add Observe backend: endpoint, SQLite schema, Tested pillar
7. Add Observe frontend: Tested tab on ObservePage
8. Implement API + observe suites
9. Add Makefile targets + run-scheduled.sh
10. Set up cron on titan-pc
11. Archive ~/soul-e2e/
