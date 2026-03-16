# Sidebar Navigation — Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the horizontal top navbar with a collapsible left sidebar (product navigation) and move chat sessions to a collapsible right panel.

**Architecture:** Replace `AppLayout.tsx` with a three-panel layout (sidebar + content + optional right panel). Create `Sidebar.tsx` as a new component with expand/collapse + icon rail. Extract `SessionList` from `ChatPage` into a `SessionsPanel` that renders on the right. Each product page can optionally export a `TopBar` component for context-aware actions. Mobile uses hamburger overlays.

**Tech Stack:** React 19, Tailwind CSS v4, TypeScript, localStorage for panel state persistence

**Spec:** `docs/superpowers/specs/2026-03-16-ui-redesign-design.md` (Part 1: Navigation)

---

## File Structure

| File | Responsibility | Action |
|------|---------------|--------|
| `web/src/components/Sidebar.tsx` | Collapsible product sidebar with icon rail | Create |
| `web/src/components/SessionsPanel.tsx` | Right-side collapsible sessions panel | Create |
| `web/src/components/TopBar.tsx` | Shared top bar shell (icon + name + slot for actions) | Create |
| `web/src/components/ChatTopBar.tsx` | Chat-specific top bar: +New, Running, Unread, History | Create |
| `web/src/layouts/AppLayout.tsx` | Three-panel layout replacing top navbar | Rewrite |
| `web/src/pages/ChatPage.tsx` | Remove inline session sidebar, use right panel | Modify |
| `web/src/app.css` | Sidebar transition animations, panel widths | Modify |

---

## Task 1: CSS Tokens for Sidebar

**Files:**
- Modify: `web/src/app.css`

- [ ] **Step 1: Add sidebar CSS variables and transitions**

Add after the `.streaming-cursor` block:

```css
/* ─── Sidebar ─── */

.sidebar-expanded { width: 200px; }
.sidebar-collapsed { width: 52px; }
.sidebar-transition { transition: width 200ms ease; }

.sessions-panel { width: 220px; }
.sessions-hidden { width: 0; overflow: hidden; }
.sessions-transition { transition: width 200ms ease, opacity 150ms ease; }
```

- [ ] **Step 2: Verify build**

```bash
cd /home/rishav/soul-v2/web && npx vite build 2>&1 | tail -3
```

- [ ] **Step 3: Commit**

```bash
cd /home/rishav/soul-v2 && git add web/src/app.css
git commit -m "feat: add sidebar CSS variables and transitions"
```

---

## Task 2: Sidebar Component

**Files:**
- Create: `web/src/components/Sidebar.tsx`

- [ ] **Step 1: Create Sidebar with expand/collapse**

```tsx
import { useState, useEffect } from 'react';
import { NavLink, useLocation } from 'react-router';

const SIDEBAR_KEY = 'soul-v2-sidebar';

const navItems = [
  { to: '/', label: 'Dashboard', icon: DashboardIcon, end: true },
  { to: '/chat', label: 'Chat', icon: ChatIcon },
  { to: '/tasks', label: 'Tasks', icon: TasksIcon },
  { to: '/tutor', label: 'Tutor', icon: TutorIcon },
  { to: '/projects', label: 'Projects', icon: ProjectsIcon },
  { to: '/observe', label: 'Observe', icon: ObserveIcon },
  { to: '/scout', label: 'Scout', icon: ScoutIcon },
  { to: '/sentinel', label: 'Sentinel', icon: SentinelIcon },
  { to: '/mesh', label: 'Mesh', icon: MeshIcon },
  { to: '/bench', label: 'Bench', icon: BenchIcon },
];
```

Each icon is a small SVG function component (10-12px, 1.5px stroke). The sidebar:
- Renders golden diamond logo at top
- Search bar (expanded only) with `⌘K` hint
- Flat nav list — each item is a `NavLink` with icon + label
- Active item has `bg-soul/10 text-soul` styling
- Collapse button (`«`) toggles between 200px and 52px
- Collapsed: icon-only with tooltip on hover
- State persisted in `localStorage('soul-v2-sidebar')`

- [ ] **Step 2: Verify TypeScript + build**

```bash
cd /home/rishav/soul-v2/web && npx tsc --noEmit && npx vite build 2>&1 | tail -3
```

- [ ] **Step 3: Commit**

```bash
cd /home/rishav/soul-v2 && git add web/src/components/Sidebar.tsx
git commit -m "feat: create collapsible Sidebar with icon rail"
```

---

## Task 3: SessionsPanel Component

**Files:**
- Create: `web/src/components/SessionsPanel.tsx`

- [ ] **Step 1: Extract sessions panel from ChatPage**

Create `SessionsPanel.tsx` that wraps `SessionList` with:
- Header: "Sessions" + collapse button (`»`)
- Collapsible: 220px expanded, 0px hidden
- State persisted in `localStorage('soul-v2-sessions-panel')`
- Props: all session-related callbacks from ChatPage
- Only renders when on `/chat` route

- [ ] **Step 2: Verify TypeScript + build**

- [ ] **Step 3: Commit**

```bash
cd /home/rishav/soul-v2 && git add web/src/components/SessionsPanel.tsx
git commit -m "feat: create collapsible SessionsPanel for right side"
```

---

## Task 4: TopBar Shell + ChatTopBar

**Files:**
- Create: `web/src/components/TopBar.tsx`
- Create: `web/src/components/ChatTopBar.tsx`

- [ ] **Step 1: Create TopBar shell**

A shared bar that renders:
- Left: golden diamond icon + product name + connection status
- Right: `children` slot for product-specific actions

```tsx
interface TopBarProps {
  icon?: React.ReactNode;
  title: string;
  children?: React.ReactNode;
}
```

- [ ] **Step 2: Create ChatTopBar**

Chat-specific actions for the right slot:
- `+ New` button (purple, SVG plus icon)
- `Running` button (amber spinner + count, dropdown on click)
- `Unread` button (purple dot + count, dropdown on click)
- `History` button (clock SVG, toggles sessions panel)

The Running and Unread dropdowns are popover menus positioned below their buttons. Each shows a list of sessions matching that filter. Clicking a session switches to it.

- [ ] **Step 3: Verify TypeScript + build**

- [ ] **Step 4: Commit**

```bash
cd /home/rishav/soul-v2 && git add web/src/components/TopBar.tsx web/src/components/ChatTopBar.tsx
git commit -m "feat: create TopBar shell + ChatTopBar with Running/Unread dropdowns"
```

---

## Task 5: Rewrite AppLayout — Three-Panel Layout

**Files:**
- Rewrite: `web/src/layouts/AppLayout.tsx`

- [ ] **Step 1: Replace the entire AppLayout**

New structure:
```
┌──────────┬─────────────────────────┬──────────┐
│ Sidebar  │   TopBar                │          │
│          ├─────────────────────────┤ Sessions │
│          │   <Outlet />            │  Panel   │
│  200px   │     (content)           │  220px   │
│ ← 52px → │                         │ ← 0px → │
└──────────┴─────────────────────────┴──────────┘
```

Key changes:
- Remove the `<header>` top navbar entirely
- Remove the mobile bottom nav entirely
- Add `<Sidebar>` on the left
- Add `<Outlet />` in the center
- The sessions panel is rendered inside ChatPage (not AppLayout) since it's chat-specific
- Mobile (`< md`): sidebar hidden by default, hamburger button in top-left opens it as overlay

```tsx
export function AppLayout() {
  return (
    <div data-testid="app-layout" className="h-screen bg-deep text-fg flex noise">
      {/* Left sidebar */}
      <Sidebar />

      {/* Main content area */}
      <div className="flex-1 flex flex-col min-w-0 min-h-0">
        <Outlet />
      </div>
    </div>
  );
}
```

- [ ] **Step 2: Verify TypeScript + build**

- [ ] **Step 3: Commit**

```bash
cd /home/rishav/soul-v2 && git add web/src/layouts/AppLayout.tsx
git commit -m "feat: rewrite AppLayout — three-panel with sidebar, no top navbar"
```

---

## Task 6: Update ChatPage — TopBar + Right Panel

**Files:**
- Modify: `web/src/pages/ChatPage.tsx`

- [ ] **Step 1: Replace inline session sidebar with TopBar + SessionsPanel**

Remove:
- The left-side `<SessionList>` sidebar (lines 102-120)
- The mobile sidebar toggle button (lines 122-133)
- The backdrop (lines 93-100)
- The `useSwipeDrawer` hook usage

Add:
- `<ChatTopBar>` at the top of the chat area (with +New, Running, Unread, History)
- `<SessionsPanel>` on the right side of the chat area
- History button toggles the sessions panel

New structure:
```tsx
return (
  <div data-testid="chat-page" className="h-full flex">
    {/* Chat content */}
    <div className="flex-1 flex flex-col min-w-0">
      <ChatTopBar
        onCreateSession={createSession}
        sessions={sessions}
        onSwitchSession={switchSession}
        onToggleSessions={toggleSessions}
        sessionsOpen={sessionsOpen}
      />
      <ConnectionBanner status={status} reconnectAttempt={reconnectAttempt} />
      {searchOpen && <SearchBar ... />}
      <MessageList ... />
      <ChatInput ... />
    </div>

    {/* Right sessions panel */}
    <SessionsPanel
      open={sessionsOpen}
      sessions={sessions}
      activeSessionID={currentSessionID}
      onSwitch={switchSession}
      onDelete={deleteSession}
      onRename={renameSession}
      onClose={() => setSessionsOpen(false)}
    />
  </div>
);
```

- [ ] **Step 2: Verify TypeScript + build**

- [ ] **Step 3: Commit**

```bash
cd /home/rishav/soul-v2 && git add web/src/pages/ChatPage.tsx
git commit -m "feat: ChatPage — TopBar + right SessionsPanel, remove left sidebar"
```

---

## Task 7: Mobile Hamburger

**Files:**
- Modify: `web/src/components/Sidebar.tsx`

- [ ] **Step 1: Add mobile overlay behavior**

On screens `< md`:
- Sidebar is hidden by default (no icon rail)
- A hamburger button appears in the top-left corner of the content area
- Clicking it opens the sidebar as a 260px overlay with backdrop
- Clicking outside or clicking a nav item closes it
- The hamburger button is rendered by Sidebar itself as a fixed-position element

- [ ] **Step 2: Verify on small viewport**

```bash
# E2E with 375px viewport
ssh rishav@192.168.0.196 "node ~/soul-e2e/mobile-test.js"
```

- [ ] **Step 3: Commit**

```bash
cd /home/rishav/soul-v2 && git add web/src/components/Sidebar.tsx
git commit -m "feat: Sidebar mobile hamburger overlay"
```

---

## Task 8: Build, Deploy, E2E Verify

**Files:** None (deploy only)

- [ ] **Step 1: Full frontend build**

```bash
cd /home/rishav/soul-v2/web && npx vite build 2>&1 | tail -5
```

- [ ] **Step 2: Deploy**

```bash
cd /home/rishav/soul-v2
sudo kill $(pgrep soul-chat) 2>/dev/null; sleep 1
sudo systemctl start soul-v2; sleep 2
ss -tlnp | grep 3002
```

- [ ] **Step 3: E2E verification on titan-pc**

Desktop (1280x900):
- [ ] Left sidebar visible with all product icons + labels
- [ ] Active page highlighted in sidebar
- [ ] Clicking sidebar item navigates to that page
- [ ] Collapse button (`«`) shrinks to 52px icon rail
- [ ] Expanded/collapsed state persists across page reload
- [ ] Chat page: TopBar shows +New, Running, Unread, History buttons
- [ ] History button toggles right sessions panel
- [ ] No top navbar or bottom nav visible

Mobile (375x812):
- [ ] Sidebar hidden, hamburger button visible
- [ ] Clicking hamburger opens sidebar overlay
- [ ] Clicking nav item closes overlay and navigates
- [ ] No bottom navigation bar

- [ ] **Step 4: Commit**

```bash
git add -A && git commit -m "chore: deploy sidebar navigation"
```
