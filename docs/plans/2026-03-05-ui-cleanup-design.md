# UI Cleanup Design — 2026-03-05

## Changes

### 1. Move Refresh OAuth to Chat Tab Header
Add refresh icon button in Chat tab header (HorizontalRail expanded), left of the History clock button. Calls `POST /api/reauth`, flashes green/red on result.

### 2. Remove Chat History from Left Panel
Remove "Conversation History" clock button from ProductRail (lines 129-145). Keep the one in Chat tab header. Remove `onSessionsToggle`/`sessionsOpen` props from ProductRail.

### 3. Settings Icon → Gear/Cog
Replace sun-burst SVG in ProductRail settings button with a standard gear/cog wheel icon.

### 4. Expose Task Filters Inline
Replace filter popover button in Tasks tab header with two inline `<select>` dropdowns (Stage, Priority). Remove popover code.

### 5. Remove Soul Logo + All Tasks Button
- Remove Soul diamond logo button + divider from top of ProductRail
- Remove "All Tasks" button
- Sessions drawer triggered only from Chat tab header history button

### 6. Sync Filter Toggle
Add chain/link icon toggle in Tasks tab header. When active: `filters.product = activeProduct`, auto-follows product changes. When off: `filters.product = 'all'`. New state: `syncProductFilter: boolean` in useLayoutStore.

### 7. Fix Left Panel Height
Change ProductRail to `fixed left-0 top-0 h-screen` so it spans full viewport height regardless of bottom drawer. Remove the `w-14 shrink-0` spacer from the bottom drawer area. Add `ml-14` to content.

## Files Modified
- `web/src/components/layout/ProductRail.tsx` — tasks 2, 3, 5
- `web/src/components/layout/HorizontalRail.tsx` — tasks 1, 4, 6
- `web/src/components/layout/AppShell.tsx` — tasks 5 (prop cleanup), 6 (sync logic), 7 (layout)
- `web/src/hooks/useLayoutStore.ts` — task 6 (syncProductFilter state)
