# V1 Migration Phase 4: Frontend

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add 4 new pages (Scout, Sentinel, Mesh, Bench), 28 components, 5 hooks, and update navigation + product selector for all 21 products.

**Architecture:** React 19 + TypeScript 5.9 + Vite 7 + Tailwind CSS v4 (dark theme, zinc palette). Lazy-loaded routes. Each page follows the existing tab-layout pattern from TutorPage/ObservePage.

**Tech Stack:** React, TypeScript, Tailwind CSS v4

**Spec:** `docs/superpowers/specs/2026-03-15-soul-v1-migration-design.md` §Frontend Changes

---

## Task 1: useScout Hook + ScoutPage Shell

**Files:**
- Create: `web/src/hooks/useScout.ts`
- Create: `web/src/pages/ScoutPage.tsx`

- [ ] **Step 1:** Create useScout.ts — leads CRUD (list, get, create, update), analytics, sweep (trigger, status, digest), sync, profile (get, pull, push), optimizations (add, list), agent (status, history), scored leads. Follow useProjects.ts pattern with fetch + state.

- [ ] **Step 2:** Create ScoutPage.tsx — 5-tab layout (Pipeline, Analytics, Actions, Profile, Intelligence). Lazy-load tab content. `data-testid` on all interactive elements.

- [ ] **Step 3:** Verify: `npx tsc --noEmit` → PASS

- [ ] **Step 4:** Commit: `feat: add useScout hook and ScoutPage shell`

---

## Task 2: Scout Components (13)

**Files:** Create `web/src/components/scout/*.tsx`

- [ ] **Step 1:** PipelineBoard.tsx — Kanban with type filter (job/freelance/contract/consulting/product-dev), dynamic columns per pipeline type.

- [ ] **Step 2:** LeadCard.tsx — Compact card with stage badge, source, match score.

- [ ] **Step 3:** LeadDetail.tsx — Expanded view: editable fields, stage advancement buttons, history timeline.

- [ ] **Step 4:** AnalyticsView.tsx — Stats bars, conversion funnels, source breakdown, weekly trend.

- [ ] **Step 5:** ActionsView.tsx — Follow-ups due table, stale leads, pipeline gaps, source stats.

- [ ] **Step 6:** SyncStatus.tsx — Health bar per platform (LinkedIn, GitHub, Naukri, Wellfound).

- [ ] **Step 7:** ProfilePanel.tsx — Profile sections (experience, projects, skills, education, certifications).

- [ ] **Step 8:** ApprovalDialog.tsx — Approve/reject optimization recommendations.

- [ ] **Step 9:** AgentActivity.tsx — Real-time optimization run status.

- [ ] **Step 10:** AgentCards.tsx — Card grid of past agent runs.

- [ ] **Step 11:** HotLeadsTable.tsx — Scored leads ranked by match.

- [ ] **Step 12:** DigestSummary.tsx — Weekly opportunity summary.

- [ ] **Step 13:** IntelligenceView.tsx — Advanced lead intelligence view.

- [ ] **Step 14:** Verify: `npx tsc --noEmit` → PASS

- [ ] **Step 15:** Commit: `feat: add 13 scout components`

---

## Task 3: useSentinel Hook + SentinelPage + Components (6)

**Files:**
- Create: `web/src/hooks/useSentinel.ts`
- Create: `web/src/pages/SentinelPage.tsx`
- Create: `web/src/components/sentinel/*.tsx`

- [ ] **Step 1:** useSentinel.ts — challenges (list, start, submit), attack, sandbox (config, chat), defend, scan, progress.

- [ ] **Step 2:** SentinelPage.tsx — 3-tab layout (Challenges, Sandbox, Progress).

- [ ] **Step 3:** ChallengeList.tsx — Grid of challenges, filter by category/difficulty/completion.

- [ ] **Step 4:** ChallengeSession.tsx — Interactive attack UI: prompt input, chatbot response, hints, flag submission, turn counter.

- [ ] **Step 5:** SandboxConfig.tsx — Configure chatbot (system prompt textarea, guardrails, weakness slider).

- [ ] **Step 6:** SandboxChat.tsx — Free-play attack interface (no scoring).

- [ ] **Step 7:** ProgressBoard.tsx — Total points, completion grid, category breakdown.

- [ ] **Step 8:** ScanResults.tsx — Red-team scan findings table.

- [ ] **Step 9:** Verify + commit: `feat: add sentinel page, hook, and 6 components`

---

## Task 4: useMesh Hook + MeshPage + Components (4)

**Files:**
- Create: `web/src/hooks/useMesh.ts`
- Create: `web/src/pages/MeshPage.tsx`
- Create: `web/src/components/mesh/*.tsx`

- [ ] **Step 1:** useMesh.ts — nodes (list, detail), status (cluster aggregation), heartbeats, link (generate/enter codes).

- [ ] **Step 2:** MeshPage.tsx — 2-tab layout (Cluster, Nodes).

- [ ] **Step 3:** ClusterStatus.tsx — Aggregated resources (CPU cores, RAM, storage), hub identity badge.

- [ ] **Step 4:** NodeList.tsx — Table: name, role, status, last heartbeat, capability score.

- [ ] **Step 5:** NodeDetail.tsx — Per-node metrics: CPU, RAM, storage, heartbeat history chart.

- [ ] **Step 6:** LinkingPanel.tsx — Generate pairing code, enter code to join cluster.

- [ ] **Step 7:** Verify + commit: `feat: add mesh page, hook, and 4 components`

---

## Task 5: useBench Hook + BenchPage + Components (5)

**Files:**
- Create: `web/src/hooks/useBench.ts`
- Create: `web/src/pages/BenchPage.tsx`
- Create: `web/src/components/bench/*.tsx`

- [ ] **Step 1:** useBench.ts — prompts (list categories), run (start benchmark), smoke (quick test), results (list, detail), compare.

- [ ] **Step 2:** BenchPage.tsx — 3-tab layout (Run, Results, Compare).

- [ ] **Step 3:** BenchRunner.tsx — Configure benchmark: model endpoint input, GPU/CPU toggle, category selector, start button with progress.

- [ ] **Step 4:** SmokeTest.tsx — Quick 3-test run with pass/fail indicators.

- [ ] **Step 5:** ResultsTable.tsx — Past results: model, accuracy, latency, CARS scores.

- [ ] **Step 6:** ResultDetail.tsx — Per-category breakdown, individual prompt scores.

- [ ] **Step 7:** CompareView.tsx — Side-by-side model comparison with bar charts.

- [ ] **Step 8:** Verify + commit: `feat: add bench page, hook, and 5 components`

---

## Task 6: Navigation + Router + Product Selector

**Files:**
- Modify: `web/src/router.tsx`
- Modify: `web/src/layouts/AppLayout.tsx`
- Modify: `web/src/components/ChatInput.tsx`

- [ ] **Step 1:** Add 4 routes to router.tsx (lazy-loaded):

```tsx
{ path: '/scout', lazy: () => import('./pages/ScoutPage') },
{ path: '/sentinel', lazy: () => import('./pages/SentinelPage') },
{ path: '/mesh', lazy: () => import('./pages/MeshPage') },
{ path: '/bench', lazy: () => import('./pages/BenchPage') },
```

- [ ] **Step 2:** Add 4 nav entries to AppLayout.tsx (Scout, Sentinel, Mesh, Bench).

- [ ] **Step 3:** Update ChatInput.tsx product selector — add all 17 new products to the selector dropdown. Group by category:

```
Existing: Tasks, Tutor, Projects, Observe
Smart Agents: Scout, Sentinel, Mesh, Bench
Quality: Compliance, QA, Analytics
Infrastructure: DevOps, DBA, Migrate
Data: DataEng, CostOps, Viz
Documentation: Docs, API
```

- [ ] **Step 4:** Verify: `npx tsc --noEmit` → PASS

- [ ] **Step 5:** Commit: `feat: update navigation, router, and product selector for 21 products`

---

## Task 7: Full Frontend Verification

- [ ] **Step 1:** `npx tsc --noEmit` → PASS (zero type errors)

- [ ] **Step 2:** `npx vite build` → SUCCESS (bundle builds)

- [ ] **Step 3:** Visual check — navigate to each new page, verify tabs render, no console errors.

- [ ] **Step 4:** Commit any fixes: `fix: resolve frontend build issues`

---

## Summary

| Task | What | Files | Tests |
|------|------|-------|-------|
| 1 | useScout + ScoutPage | 2 | 0 |
| 2 | 13 scout components | 13 | 0 |
| 3 | Sentinel page + hook + 6 components | 8 | 0 |
| 4 | Mesh page + hook + 4 components | 6 | 0 |
| 5 | Bench page + hook + 5 components | 7 | 0 |
| 6 | Navigation + router + selector | 3 | 0 |
| 7 | Verification | 0 | 0 |
| **Total** | | **~39 files** | **0 (visual)** |
