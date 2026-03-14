# Mobile-Centric Chat + Backend Hardening

## Context

Soul v2 chat is functional but desktop-only. The sidebar is fixed at 256px with no responsive breakpoints. The backend is missing HTTP body size limits, per-client WS rate limiting, and the frontend uses constant 3s reconnect intervals (thundering herd risk). The chat UI needs to be mobile-first since the future Soul Dashboard will be the desktop-centric product view.

## Changes

### Frontend — Mobile Responsive Layout

**Shell.tsx redesign:**
- Add `sidebarOpen` state (default false on mobile, irrelevant on desktop)
- Header: hamburger button visible below `md` breakpoint, hidden at `md+`
- Sidebar: On mobile (`< md`), render as fixed overlay with `translate-x` transition. On desktop (`>= md`), render inline in flex layout as today.
- Backdrop overlay: `fixed inset-0 bg-black/50 z-30`, click to close drawer

**New hook — useSwipeDrawer.ts:**
- Tracks touch events on document for left-edge swipe detection
- Opens drawer when user swipes right from left 30px edge past 50px threshold
- Closes drawer when user swipes left while drawer is open
- Returns `{ isOpen, open, close, toggle, bindEvents }` — bindEvents attaches to a ref or document
- Raw Touch API, no dependencies

**Touch UX:**
- All interactive buttons: minimum 44px touch target (py-3 on mobile, py-2 on desktop)
- ChatInput: larger padding on mobile, bigger send button
- SessionItem: increased vertical padding below `md`
- Delete confirmation: larger tap targets

**Responsive message bubbles:**
- Default (mobile): `max-w-[95%]`
- Desktop (`md+`): `max-w-[80%]`

**index.html meta tags:**
- `<meta name="theme-color" content="#09090b">` (zinc-950)
- `<meta name="apple-mobile-web-app-capable" content="yes">`

### Backend — Hardening

**HTTP body size limit (server.go):**
- Add middleware or inline `http.MaxBytesReader(w, r.Body, 64<<10)` on all POST handlers
- Return 413 on exceeded

**Per-client WS rate limit (client.go):**
- Add sliding window rate limiter to Client: 10 msg burst, 2/sec sustained
- Track timestamps of last N messages
- On exceed: send `chat.error` with "rate limited — slow down" and drop message
- Log rate limit events to metrics

**Exponential reconnect backoff (useWebSocket.ts):**
- Replace constant 3000ms with: 1s → 2s → 4s → 8s → 15s cap
- Add ±30% jitter to prevent thundering herd
- Reset delay to 1s on successful `connection.ready`
- Expose `reconnectAttempt` count for ConnectionBanner

## Implementation Plan

### Step 1: Backend hardening (no frontend changes)

1a. HTTP body size limit middleware in server.go
1b. Per-client WS rate limiter in client.go
1c. Unit tests for both
1d. `go build ./...` + `go test -race ./...`

### Step 2: Exponential reconnect backoff

2a. Refactor useWebSocket.ts reconnect logic
2b. Add jitter + cap + reset on connect
2c. Expose reconnectAttempt to ConnectionBanner
2d. `npm run build` verify

### Step 3: useSwipeDrawer hook

3a. Implement touch gesture detection hook
3b. Left-edge open, swipe-left close, threshold 50px
3c. Returns isOpen/open/close/toggle

### Step 4: Mobile-responsive Shell layout

4a. Shell.tsx: sidebarOpen state, hamburger button, responsive classes
4b. Sidebar: fixed overlay on mobile with backdrop, static on desktop
4c. CSS transitions for slide animation
4d. Wire useSwipeDrawer to drawer

### Step 5: Touch UX + responsive polish

5a. Increase touch targets across all components
5b. Responsive message bubble widths
5c. Mobile-specific padding/spacing adjustments
5d. index.html meta tags

### Step 6: Build, deploy, verify

6a. `go build ./...` + `go test -race ./...`
6b. `cd web && npm run build`
6c. Deploy via systemd restart
6d. Test on phone (192.168.0.128:3002)

## Key Files

| File | Action |
|------|--------|
| `internal/server/server.go` | MODIFY — body size limit middleware |
| `internal/ws/client.go` | MODIFY — rate limiter |
| `web/src/hooks/useWebSocket.ts` | MODIFY — exponential backoff |
| `web/src/hooks/useSwipeDrawer.ts` | CREATE — touch gesture hook |
| `web/src/components/Shell.tsx` | MODIFY — responsive layout + drawer |
| `web/src/components/SessionList.tsx` | MODIFY — touch target sizing |
| `web/src/components/ChatInput.tsx` | MODIFY — mobile padding |
| `web/src/components/MessageBubble.tsx` | MODIFY — responsive max-width |
| `web/src/components/ConnectionBanner.tsx` | MODIFY — reconnect attempt count |
| `web/index.html` | MODIFY — meta tags |
