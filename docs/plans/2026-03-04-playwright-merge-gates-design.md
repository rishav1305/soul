# Playwright E2E Merge Gates

**Date:** 2026-03-04
**Problem:** Soul's autonomous agent can merge broken frontend builds to dev and master, taking down both servers.
**Root cause example:** `useNotifications` hook returns `{ toasts }` but component destructured `{ notifications }` — TypeScript should have caught it but `tsc` wasn't run before merge.

## Architecture

Two merge gates prevent broken code from reaching servers:

```
Task worktree (agent writes code)
  │
  ├─ tsc --noEmit ──── TYPE CHECK ─── fail? → self-repair loop (3 attempts)
  ├─ vite build ─────── BUILD CHECK ── fail? → self-repair loop
  │
  ▼
  MergeToDev ─── merge task branch → dev worktree
  │
  ├─ RebuildDevFrontend
  ├─ Playwright smoke test on :3001 ── DEV GATE
  │   └─ fail? → git revert merge in dev, self-repair loop
  │
  ▼
  Task → "validation" (user reviews on :3001)
  │
  User clicks "Done"
  ▼
  MergeToMaster
  │
  ├─ RebuildFrontend (prod)
  ├─ Playwright smoke test on :3000 ── PROD GATE
  │   └─ fail? → git revert in master, rebuild previous, task stays in validation
  │
  ▼
  Task is "done", prod is safe
```

## Smoke Test Checks (6 total)

| # | Check | Method | Failure indicates |
|---|-------|--------|-------------------|
| 1 | Page loads | HTTP 200 | Server crash or missing build |
| 2 | No JS errors | `pageerror` event listener | Runtime crash |
| 3 | React rendered | `#root` has children | App mount failure |
| 4 | Key UI elements | `data-testid` selectors for ProductRail, ChatPanel, HorizontalRail | Layout components broken |
| 5 | API health | `fetch('/api/tasks')` returns 200 JSON | Backend not responding |
| 6 | WebSocket connects | `new WebSocket(ws://...)` opens within 5s | WS hub broken |

Smoke test runs via SSH to titan-pc using existing Playwright test-runner infrastructure.

## Self-Repair Flow

### Dev gate failure:
1. `git revert HEAD --no-edit` in dev worktree
2. Rebuild dev frontend (restores working build)
3. Format failure as gap report
4. Feed back to agent (existing retry loop, up to 3 attempts)
5. After 3 failures → task moves to `blocked`

### Prod gate failure:
1. `git revert HEAD --no-edit` in master
2. Rebuild prod frontend (restores working build)
3. Task stays in `validation` with failure comment
4. User investigates manually

## Files to Change

| File | Action | Lines |
|------|--------|-------|
| `internal/server/gates.go` | CREATE | ~150 — PreMergeGate, SmokeTest, RevertMerge |
| `internal/server/autonomous.go` | MODIFY | ~30 — wire dev gate after merge+rebuild, replace Rod verification |
| `internal/server/tasks.go` | MODIFY | ~20 — wire prod gate after merge+rebuild |
| `tools/e2e/test-runner.js` | MODIFY | ~60 — add `smoke` action with all 6 checks |
| `web/src/components/layout/ProductRail.tsx` | MODIFY | 1 line — add data-testid |
| `web/src/components/layout/HorizontalRail.tsx` | MODIFY | 1 line — add data-testid |
| `web/src/components/chat/ChatPanel.tsx` | MODIFY | 1 line — add data-testid |

## Decisions

- **Playwright over Rod:** Rod doesn't work on ARM64 (titan-pi). Playwright runs on titan-pc (x86) via SSH.
- **Revert-on-failure:** Ensures servers always have working builds, even if agent can't self-repair.
- **tsc before merge:** Catches type errors that vite's esbuild transpiler silently ignores (esbuild strips types, doesn't check them).
- **data-testid over CSS selectors:** Stable across style changes, explicit contract between frontend and tests.
