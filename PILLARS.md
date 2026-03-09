# Soul v2 — Design Pillars

Five non-negotiable constraints. Every feature, every line of code, every commit is measured against these pillars. If a change violates any constraint, it does not ship.

## 1. PERFORMANT

Fast and resource-efficient.

| Constraint | Enforcement |
|------------|-------------|
| First token to screen < 200ms | E2E timing assertion |
| WebSocket overhead < 50 bytes | Integration test |
| Frontend bundle < 300KB gzipped | Build gate |
| Server memory < 100MB at 10 sessions | Monitoring alert |
| Zero unnecessary re-renders | E2E React profiler |
| Exact token budget | Opus review |

## 2. ROBUST

Handles every edge case without crashing.

| Constraint | Enforcement |
|------------|-------------|
| No panic on any input (fuzz-tested) | Property-based tests |
| Defined behavior for nil/empty/oversized | Unit tests per invariant |
| Atomic DB operations | Integration test |
| Graceful render with 0 sessions, 1000 messages | E2E boundary tests |
| Every error path produces user-visible message | Opus review |
| Type system prevents invalid states | Static analysis |

## 3. RESILIENT

Recovers automatically, degrades gracefully.

| Constraint | Enforcement |
|------------|-------------|
| API down → UI shows status, retains state, reconnects | E2E simulated outage |
| WS disconnect → auto-reconnect with backoff | Integration test |
| Token refresh fails → disk fallback → retry → alert | Unit test |
| Server restart → sessions restored | Integration test |
| Corrupted DB → detected, backup or clean error | Unit test |
| OOM → graceful shed, never crash | Load test |

## 4. SECURE

Hardened, minimal attack surface.

| Constraint | Enforcement |
|------------|-------------|
| Zero secrets in code/config/logs | SAST scan + CI gate |
| All input sanitized (XSS-proof) | E2E + property tests |
| WS origin validation | Integration test |
| Parameterized SQL | Static pattern scan |
| OAuth tokens 0600, never logged | File permission + log scan |
| Dependencies audited | npm audit + govulncheck |
| Rate limiting | Integration test |
| CSP headers | E2E header check |

## 5. SOVEREIGN

You own everything.

| Constraint | Enforcement |
|------------|-------------|
| Zero external CDNs/fonts/assets | E2E network audit |
| No SaaS dependencies | Spec review |
| SQLite local | Architecture constraint |
| Gitea hosting | Push workflow |
| No telemetry/analytics | E2E network monitor |
| All artifacts in repo | Spec review |
| Claude API abstracted, swappable | Opus review |
| Offline reading (service worker) | E2E offline test |

## Pillar Verification Matrix

| Layer | What it checks | Pillars covered | When it runs |
|-------|---------------|-----------------|--------------|
| L1: Static Analysis | `go vet`, `tsc --noEmit`, secret scan, dep audit | Robust, Secure | Every commit |
| L2: Unit Tests | Public functions, edge cases, error paths | Robust, Resilient | Every commit |
| L3: Integration Tests | API contracts, WS protocol, DB operations | Performant, Robust, Resilient, Secure | Every commit |
| L4: E2E Tests | Full user flows, timing, rendering | Performant, Robust, Sovereign | Pre-merge |
| L5: Load Tests | Concurrency, memory, degradation | Performant, Resilient | Weekly |
| L6: Opus Review | Diff review, architecture alignment, pillar compliance | All pillars | Pre-merge |
| L7: Visual Regression | Screenshot comparison, layout shifts | Performant, Robust | Pre-merge (UI changes) |
