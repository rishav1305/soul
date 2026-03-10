# Pillar Violation Fixes — Design

## Goal

Fix all violations found in the 5-pillar audit: 1 high security issue, 1 critical bundle size overage, 3 critical resilience gaps, and multiple performance/robustness improvements. Incremental phases, each independently deployable and testable.

## Architecture

Six substantive phases plus one minor cleanup phase, ordered by severity. Backend phases get Go integration tests, frontend phases get Puppeteer E2E on titan-pc. Every phase ends with `make verify` passing.

## Phase 1: Security — Path Traversal + CSP (Critical)

**Path traversal** in `internal/server/server.go` spaHandler: After `filepath.Join(staticDir, cleanPath)`, resolve to absolute path with `filepath.Abs()`, verify prefix with `strings.HasPrefix`. Wire in existing `FileExistsInDir()` helper.

**CSP**: Replace `'unsafe-inline'` in style-src with `'self'` only. Tailwind v4 generates external CSS files, not inline styles.

**Tests**: Go integration test for `GET /../../etc/passwd` → 403. CSP header assertion.

## Phase 2: Performance — Bundle Size (Critical)

**Remove Mermaid entirely** (-348KB gzipped):
- Delete `web/src/components/MermaidBlock.tsx`
- Remove `mermaid` from `web/package.json`
- Simplify `CodeBlock.tsx`: remove lazy import + Suspense, render mermaid blocks as regular code
- Diagrams: Claude generates ASCII/text diagrams in monospace blocks (zero dependencies)

If bundle still over 300KB after Mermaid removal, Phase 2b addresses react-syntax-highlighter.

**Tests**: `make check-bundle` passes. Puppeteer E2E: code block renders, mermaid block renders as code (no crash).

## Phase 3: Resilience — Data Loss Prevention (Critical)

**3a. Persist incomplete streams**: In `ws/handler.go`, on stream end without `message_stop`, persist partial `fullText` with `[incomplete]` suffix instead of discarding.

**3b. Transactional message storage**: Wrap user + assistant message inserts in SQLite transaction. Add `RunInTransaction(fn)` helper to `session/store.go`.

**3c. Send channel backpressure**: In `ws/client.go`, close slow clients instead of silently dropping messages. Disconnected clients reconnect and get history.

**Tests**: Go integration tests for stream interruption, slow client disconnect, transaction rollback.

## Phase 4: Resilience — Timeouts & Connection Hardening

**4a. Stream timeout**: Replace `context.Background()` with 5-minute deadline in `stream/client.go`.

**4b. WebSocket read deadline**: Set read deadline after each successful read in `ws/client.go` ReadPump. Dead clients detected within 60s.

**4c. Pending message persistence**: Write pending message to `localStorage` before deferred session creation in `useChat.ts`. Recover on reconnect.

**Tests**: Go integration tests for hanging API timeout, silent client disconnect. Puppeteer E2E for pending message recovery.

## Phase 5: Performance — React Optimization

**5a.** Wrap `SessionItem` in `React.memo`, pass stable callbacks.
**5b.** Memoize session filtering with `useMemo`.
**5c.** Verify Markdown `components` object is module-level (not recreated per render).
**5d.** Memoize `HighlightText` regex with `useMemo`.
**5e.** Fix copy timeout cleanup with `useRef` + `useEffect`.

**Tests**: Puppeteer E2E for search responsiveness, copy button, no console errors on quick navigation.

## Phase 6: Resilience — State Management

**6a.** Wrap session status read-check-update in SQLite transaction to prevent race conditions.
**6b.** Add 3-attempt exponential backoff to reauth in `useChat.ts`.
**6c.** Increase broadcast channel capacity to 1024, log warning at 75%.

**Tests**: Go integration test for concurrent status transitions. Puppeteer E2E for reauth retry.

## Phase 7: Robust — Type Safety (Minor)

**7a.** Proper `SpeechRecognition` interface in `ChatInput.tsx` (replace `as any`).
**7b.** Explicit type for session history loop parameter in `useChat.ts:147`.

No dedicated tests — `tsc --noEmit` validates.

## Key Files

| File | Phases |
|------|--------|
| `internal/server/server.go` | 1 |
| `internal/ws/client.go` | 3c, 4b |
| `internal/ws/handler.go` | 3a, 3b, 6a |
| `internal/ws/hub.go` | 6c |
| `internal/stream/client.go` | 4a |
| `internal/session/store.go` | 3b, 6a |
| `web/src/components/MermaidBlock.tsx` | 2 (delete) |
| `web/src/components/CodeBlock.tsx` | 2, 5e |
| `web/src/components/Markdown.tsx` | 5c |
| `web/src/components/SessionList.tsx` | 5a, 5b |
| `web/src/components/MessageBubble.tsx` | 5d, 5e |
| `web/src/hooks/useChat.ts` | 4c, 6b, 7b |
| `web/src/components/ChatInput.tsx` | 7a |
| `web/package.json` | 2 |

## Verification Matrix

| Phase | L1 Static | L2 Unit | L3 Integration | L4 E2E | Bundle Gate |
|-------|-----------|---------|----------------|--------|-------------|
| 1 | go vet | server_test | path traversal, CSP | — | — |
| 2 | tsc | — | — | code block render | check-bundle |
| 3 | go vet | store_test (txn) | stream interrupt, slow client | — | — |
| 4 | go vet | — | hang timeout, dead client | pending msg | — |
| 5 | tsc | — | — | search perf, copy | — |
| 6 | go vet | — | concurrent status | reauth retry | — |
| 7 | tsc | — | — | — | — |
