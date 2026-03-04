# Soul Codebase Cleanup

**Date:** 2026-03-05
**Problem:** Codebase has accumulated dead code, duplicate components, redundant logic, and structural waste from rapid feature development.
**Goal:** Remove ~1,500+ lines of dead code, consolidate duplicates, simplify complex functions, and clean up project structure — without changing any UI functionality.

## Audit Findings

### Backend (Go) — internal/server/
- `verification.go` (296 lines) — entirely dead, Rod-based E2E replaced by Playwright smoke tests
- Duplicate frontend rebuild: `server.go:RebuildDevFrontend` vs `worktree.go:RebuildFrontend`
- Duplicate smoke test handling in `autonomous.go` (two nearly-identical blocks)
- `buildTaskPrompt()` hardcodes project context that CLAUDE.md now provides
- `processTask()` is 291 lines with deep nesting
- Hardcoded SSH host, port offsets, E2E paths scattered across files

### Frontend (React) — web/src/
- 6 duplicate component files in root `components/` (~1,141 dead lines) — layout/ versions are canonical
- 5 unused type exports in `lib/types.ts`
- Unused `autoWidth()` in `useLayoutStore.ts`
- Unused `ValidTransitions` in `planner/transitions.ts`
- `GridView.tsx` is a trivial pass-through wrapper
- Legacy v1 layout state in `useLayoutStore.ts` (~80 lines)
- Duplicated fetch/UUID patterns across hooks

### Project Structure
- `upstream/` directory (11MB) — old Claude Code reference, not used
- 24 plan docs in `docs/plans/` — completed work should be archived
- `soul` binary at project root should be in .gitignore

## Phases

### Phase 1: Dead Code Deletion
Delete provably unused files and exports. Zero risk.

### Phase 2: Backend Refactoring
Consolidate rebuild logic, extract `processTask()` helpers, externalize E2E config.

### Phase 3: Frontend Refactoring
Remove legacy layout state, extract shared API utilities.

### Phase 4: Structural Cleanup
Delete `upstream/`, archive old plan docs, fix .gitignore.

## Decisions
- **Phased approach**: Each phase produces a buildable commit
- **No UI changes**: All component behavior preserved
- **CLAUDE.md replaces hardcoded context**: Remove static project structure from `buildTaskPrompt()`
- **Keep compliance-go**: It's the active implementation; TypeScript compliance is its own product
