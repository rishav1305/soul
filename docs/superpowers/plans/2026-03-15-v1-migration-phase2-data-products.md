# V1 Migration Phase 2: Grouped Data Products

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Create 4 new grouped data product servers (soul-infra, soul-quality, soul-data, soul-docs) hosting 11 data products, plus wire them into the chat context system for tool dispatch.

**Architecture:** 4 new HTTP servers following v2's Option-pattern boilerplate. 10 products are stubs returning "not_yet_implemented". Compliance is the only product with real logic (5 analyzers, fix engine, reporters, rules). All servers expose `POST /api/tools/{name}/execute` for chat integration.

**Tech Stack:** Go 1.24, net/http ServeMux, go:embed for compliance rules

**Spec:** `docs/superpowers/specs/2026-03-15-soul-v1-migration-design.md` §Grouped Data Products

---

## File Map

### Stub Servers (soul-infra :3012, soul-data :3016, soul-docs :3018)

| File | Action | Responsibility |
|------|--------|---------------|
| `cmd/infra/main.go` | Create | soul-infra entrypoint (:3012) |
| `cmd/data/main.go` | Create | soul-data entrypoint (:3016) |
| `cmd/docs/main.go` | Create | soul-docs entrypoint (:3018) |
| `internal/infra/server/server.go` | Create | HTTP server: devops, dba, migrate stub tools |
| `internal/dataprod/server/server.go` | Create | HTTP server: dataeng, costops, viz stub tools |
| `internal/docsprod/server/server.go` | Create | HTTP server: docs, api stub tools |

### soul-quality (:3014) — Compliance + Stubs

| File | Action | Responsibility |
|------|--------|---------------|
| `cmd/quality/main.go` | Create | soul-quality entrypoint (:3014) |
| `internal/quality/server/server.go` | Create | HTTP server with compliance + qa/analytics stubs |
| `internal/quality/compliance/service.go` | Create | Tool dispatch: scan, fix, badge, report |
| `internal/quality/compliance/scan/orchestrator.go` | Create | Run 5 analyzers in parallel, deduplicate |
| `internal/quality/compliance/analyzers/types.go` | Create | Finding, ScannedFile, Analyzer interface, Rule |
| `internal/quality/compliance/analyzers/secret.go` | Create | 16 regex patterns + entropy detection |
| `internal/quality/compliance/analyzers/config.go` | Create | .env, Dockerfile, CORS, CI/CD checks |
| `internal/quality/compliance/analyzers/git.go` | Create | .gitignore, CODEOWNERS, SECURITY.md, LICENSE |
| `internal/quality/compliance/analyzers/deps.go` | Create | Unpinned deps, lockfile, copyleft license |
| `internal/quality/compliance/analyzers/ast.go` | Create | 8 code pattern detectors |
| `internal/quality/compliance/fix/fix.go` | Create | 4 fix strategies + dry-run |
| `internal/quality/compliance/reporters/*.go` | Create | terminal, json, html, badge reporters |
| `internal/quality/compliance/rules/*.yaml` | Create | SOC2, HIPAA, GDPR rules (go:embed) |
| `internal/quality/compliance/rules/loader.go` | Create | YAML rule loader |

### Chat Context Integration

| File | Action | Responsibility |
|------|--------|---------------|
| `internal/chat/context/{infra,quality,dataprod,docsprod}.go` | Create | Tool definitions for 11 products |
| `internal/chat/context/context.go` | Modify | Add 11 products to ForProduct() |
| `internal/chat/context/dispatch.go` | Modify | Add routes for 24 tools |
| `internal/chat/server/proxy.go` | Modify | Add 4 product proxies |

### Build & Deploy

| File | Action | Responsibility |
|------|--------|---------------|
| `Makefile` | Modify | 4 new build targets |
| `deploy/soul-v2-{infra,quality,data,docs}.service` | Create | SystemD services |

---

## Task 1: Stub Server — soul-infra

**Files:** `cmd/infra/main.go`, `internal/infra/server/server.go`

- [ ] **Step 1:** Create `internal/infra/server/server.go` — Server struct with Option pattern, port 3012, validTools map for devops/dba/migrate (analyze+report each = 6 tools), health endpoint, tool execute endpoint returning stub JSON, standard middleware (recovery, body limit, CSP).

- [ ] **Step 2:** Create `cmd/infra/main.go` — env: SOUL_INFRA_HOST/PORT, graceful shutdown.

- [ ] **Step 3:** Verify: `go build ./cmd/infra/` → SUCCESS

- [ ] **Step 4:** Commit: `feat: add soul-infra stub server (devops, dba, migrate)`

---

## Task 2: Stub Server — soul-data

**Files:** `cmd/data/main.go`, `internal/dataprod/server/server.go`

- [ ] **Step 1:** Create server — port 3016, products: dataeng, costops, viz (6 stub tools).

- [ ] **Step 2:** Create entrypoint — env: SOUL_DATA_HOST/PORT.

- [ ] **Step 3:** Verify: `go build ./cmd/data/`

- [ ] **Step 4:** Commit: `feat: add soul-data stub server (dataeng, costops, viz)`

---

## Task 3: Stub Server — soul-docs

**Files:** `cmd/docs/main.go`, `internal/docsprod/server/server.go`

- [ ] **Step 1:** Create server — port 3018, products: docs, api (4 stub tools).

- [ ] **Step 2:** Create entrypoint — env: SOUL_DOCS_HOST/PORT.

- [ ] **Step 3:** Verify: `go build ./cmd/docs/`

- [ ] **Step 4:** Commit: `feat: add soul-docs stub server (docs, api)`

---

## Task 4: Compliance — Types & Rules

**Files:** `internal/quality/compliance/analyzers/types.go`, `internal/quality/compliance/rules/{loader.go,soc2.yaml,hipaa.yaml,gdpr.yaml,loader_test.go}`

- [ ] **Step 1:** Create types.go — Finding, ScannedFile, Analyzer interface, Rule struct.

- [ ] **Step 2:** Create rule YAML files — port from v1: soc2 (~43 rules), hipaa (~28), gdpr (~22).

- [ ] **Step 3:** Create loader.go — `go:embed *.yaml`, `LoadAll(frameworks []string) ([]Rule, error)`.

- [ ] **Step 4:** Add dependency: `go get gopkg.in/yaml.v3`

- [ ] **Step 5:** Write and run loader tests (no filter, SOC2 filter, field validation).

Run: `go test ./internal/quality/compliance/rules/ -v` → ALL PASS

- [ ] **Step 6:** Commit: `feat: add compliance types, rules, and YAML loader`

---

## Task 5: Compliance — Secret Scanner

**Files:** `internal/quality/compliance/analyzers/secret.go`, `internal/quality/compliance/analyzers/secret_test.go`

- [ ] **Step 1:** Write tests — AWS key, GitHub token, private key, no false positives, redaction, Shannon entropy.

- [ ] **Step 2:** Implement — 16 compiled regex patterns, `shannonEntropy()` (thresholds: hex=4.5, base64=5.0, other=5.0), `redact()` (first 4 + **** + last 4), text extensions only, 500KB max.

- [ ] **Step 3:** Run tests → ALL PASS

- [ ] **Step 4:** Commit: `feat: add secret scanner with 16 regex patterns and entropy detection`

---

## Task 6: Compliance — Config, Git, Deps, AST Analyzers

**Files:** 8 files (4 implementations + 4 test files)

- [ ] **Step 1:** config.go + test — .env exposure, Dockerfile issues (3 checks), CORS wildcard, CI/CD missing.

- [ ] **Step 2:** git.go + test — .gitignore completeness (3 checks), CODEOWNERS, SECURITY.md, LICENSE.

- [ ] **Step 3:** deps.go + test — unpinned deps, missing lockfile, missing engines, copyleft license.

- [ ] **Step 4:** ast.go + test — 8 regex detectors: dangerous code execution (`\beval\s*\(`), SQL injection (`(?i)(?:query.*\+.*req\.|SELECT.*\+)`), weak hash, insecure random, XSS risk (innerHTML/dangerouslySetInnerHTML — note: the AST analyzer *detects* these as security violations in scanned code, it doesn't execute them), SSL disabled, hardcoded IP (skip 0.0.0.0/127.0.0.1), empty catch.

- [ ] **Step 5:** Run all: `go test ./internal/quality/compliance/analyzers/ -v` → ALL PASS

- [ ] **Step 6:** Commit: `feat: add config, git, deps, and AST compliance analyzers`

---

## Task 7: Compliance — Orchestrator

**Files:** `internal/quality/compliance/scan/orchestrator.go`, `internal/quality/compliance/scan/orchestrator_test.go`

- [ ] **Step 1:** Implement `ScanDirectory(opts ScanOptions) (*ScanResult, error)` — walk tree (skip .git/node_modules/dist/build/vendor, >1MB), load rules, run 5 analyzers concurrently (WaitGroup+Mutex), deduplicate on file:line:id, filter by severity, build summary.

- [ ] **Step 2:** Write tests — scan dir with known secret finds it, clean dir has no findings.

- [ ] **Step 3:** Run: `go test ./internal/quality/compliance/scan/ -v` → ALL PASS

- [ ] **Step 4:** Commit: `feat: add compliance scan orchestrator with parallel execution`

---

## Task 8: Compliance — Fix Engine & Reporters

**Files:** `internal/quality/compliance/fix/fix.go` + test, `internal/quality/compliance/reporters/{terminal,json,html,badge}.go` + test

- [ ] **Step 1:** fix.go — 4 strategies: secret-to-env, weak-hash-upgrade, dangerous-code-removal, cors-restrict. Unified diff output. Dry-run mode.

- [ ] **Step 2:** Reporters — terminal (ANSI), json (MarshalIndent), html (inline CSS), badge (SVG with score: 100 - deductions).

- [ ] **Step 3:** Tests for fix strategies and reporter output.

- [ ] **Step 4:** Run: `go test ./internal/quality/compliance/... -v` → ALL PASS

- [ ] **Step 5:** Commit: `feat: add compliance fix engine and 4 reporter formats`

---

## Task 9: soul-quality Server

**Files:** `cmd/quality/main.go`, `internal/quality/server/server.go`, `internal/quality/compliance/service.go`

- [ ] **Step 1:** service.go — `ExecuteTool(name, input)` dispatches scan/fix/badge/report.

- [ ] **Step 2:** server.go — routes compliance__* to service, qa__*/analytics__* to stubs. Port 3014.

- [ ] **Step 3:** main.go — env: SOUL_QUALITY_HOST/PORT.

- [ ] **Step 4:** Verify: `go build ./cmd/quality/`

- [ ] **Step 5:** Commit: `feat: add soul-quality server with compliance engine + qa/analytics stubs`

---

## Task 10: Chat Context — 11 Data Product Tool Definitions

**Files:** Create 4 context files, modify context.go + dispatch.go + context_test.go

- [ ] **Step 1:** Create infra.go (6 tools), quality.go (8 tools), dataprod.go (6 tools), docsprod.go (4 tools) — each returns ProductContext with system prompt + tool definitions.

- [ ] **Step 2:** Update ForProduct() — devops/dba/migrate→infraContext(), compliance/qa/analytics→qualityContext(), dataeng/costops/viz→dataprodContext(), docs/api→docsprodContext().

- [ ] **Step 3:** Update dispatcher — add routes for 24 tools to 4 server URLs.

- [ ] **Step 4:** Update and run tests → ALL PASS

- [ ] **Step 5:** Commit: `feat: add chat context tool definitions for 11 data products`

---

## Task 11: Chat Server Proxies

**Files:** Modify proxy.go + cmd/chat/main.go

- [ ] **Step 1:** Add 4 proxies (infra→3012, quality→3014, data→3016, docs→3018).

- [ ] **Step 2:** Wire in main.go.

- [ ] **Step 3:** Verify: `go build ./cmd/chat/`

- [ ] **Step 4:** Commit: `feat: add chat server proxies for 4 data product servers`

---

## Task 12: Build & Deploy

**Files:** Makefile, 4 systemd service files

- [ ] **Step 1:** Makefile — build-infra, build-quality, build-data, build-docs targets, update build/serve/clean.

- [ ] **Step 2:** SystemD services — follow soul-v2-observe.service pattern.

- [ ] **Step 3:** `make build` → 9 binaries

- [ ] **Step 4:** `make verify-static` → PASS

- [ ] **Step 5:** Commit: `feat: add build targets and systemd services for 4 data product servers`

---

## Task 13: Full Verification

- [ ] **Step 1:** `go test -race -count=1 ./internal/... -v` → ALL PASS

- [ ] **Step 2:** `make build` → 9 binaries

- [ ] **Step 3:** Smoke test all 4 servers (health + tool execute)

- [ ] **Step 4:** Fix and commit any issues

---

## Summary

| Task | What | Files | Tests |
|------|------|-------|-------|
| 1 | soul-infra stub | 2 | 0 |
| 2 | soul-data stub | 2 | 0 |
| 3 | soul-docs stub | 2 | 0 |
| 4 | Compliance types & rules | 5+ | 3 |
| 5 | Secret scanner | 2 | ~6 |
| 6 | 4 analyzers | 8 | ~16 |
| 7 | Scan orchestrator | 2 | 2 |
| 8 | Fix engine & reporters | 7 | ~8 |
| 9 | soul-quality server | 3 | 0 |
| 10 | Chat context (24 tools) | 6 | 1 |
| 11 | Chat proxies | 2 | 0 |
| 12 | Build & deploy | 5 | 0 |
| 13 | Verification | 0 | 0 |
| **Total** | | **~46 files** | **~36 tests** |
