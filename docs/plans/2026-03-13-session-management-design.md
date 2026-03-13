# Session Management — Full Design

## Goal

Enrich the session sidebar with smart titles, message previews, unread badges, time grouping, and inline rename. Backfill existing sessions.

## Data Model Changes

Two new columns on `sessions` table:

```sql
ALTER TABLE sessions ADD COLUMN last_message TEXT DEFAULT '';
ALTER TABLE sessions ADD COLUMN unread_count INTEGER DEFAULT 0;
```

**Session struct** gets:
- `LastMessage string json:"lastMessage"` — last message content, truncated to 100 chars
- `UnreadCount int json:"unreadCount"` — messages since user last viewed

**Update rules:**
- `AddMessage` / `AddMessageTx`: set `last_message` to truncated content, increment `unread_count`
- `handleSessionSwitch`: reset `unread_count = 0`
- `handleChatSend`: reset `unread_count = 0` (user is active)
- `completeSession`: does NOT reset (response just arrived)

## Smart Title Generation (Hybrid)

**Phase 1 — Instant:** On first user message (`MessageCount == 0`), set title to first 50 chars at word boundary. Immediate, no API call.

**Phase 2 — Background upgrade:** After first assistant response completes (in `completeSession`, when `MessageCount == 2`):
1. Background goroutine sends Haiku API call: "Generate a 3-5 word title for this conversation. User: {first_message}. Assistant: {first_response}. Reply with ONLY the title, no quotes."
2. 5-second timeout, no retry on failure
3. On success: `UpdateSessionTitle()` + broadcast `session.updated`
4. Cost: ~$0.0001/title

## Session Sidebar UI

**SessionItem layout:**
```
┌─────────────────────────────────┐
│ ● Title text here...    ② 3d ago│
│   Last message preview trun...  │
└─────────────────────────────────┘
```

- Status dot (left): green pulsing = running, soul ring = unread, gray = idle
- Title (bold, truncated)
- Unread badge: circled count, soul-colored, hidden when 0
- Timestamp (right): relative time
- Last message preview (second line): muted text, ~60 chars, role prefix icon

**Time group headers:**
```
Today
  [session items]
Yesterday
  [session items]
Older
  [session items]
```

Simple text dividers: `text-xs text-fg-muted uppercase tracking-wide`. Sessions sorted by `updatedAt` descending within groups.

**Inline rename:** Double-click title → input field. Enter saves, Escape cancels. Sends `session.rename` WebSocket message → backend `UpdateSessionTitle` → broadcast `session.updated`.

**Delete:** Unchanged (hover reveal, two-step confirm).

## Read/Unread Logic

- `unreadCount > 0`: badge with count, bold title, soul-colored dot ring
- `unreadCount === 0`: normal muted style, gray dot
- Currently selected session: never shows unread

## Backfill Migration

One-time in `Migrate()`:
1. Set `last_message` from most recent message per session
2. Set `unread_count = 0` for all existing sessions
3. Set title from first user message for any session titled "New Session"

## New WebSocket Message Type

- `session.rename` (inbound): `{type: "session.rename", sessionId: string, content: string}`
- Handler calls `UpdateSessionTitle(id, content)`, broadcasts `session.updated`

## Key Files

| File | Change |
|------|--------|
| `internal/session/store.go` | Migration, Session struct, update queries |
| `internal/session/timed_store.go` | New methods: `ResetUnreadCount`, `SetLastMessage` |
| `internal/session/iface.go` | Interface updates |
| `internal/ws/handler.go` | Title gen goroutine, rename handler, unread logic |
| `internal/ws/message.go` | `TypeSessionRename` constant |
| `web/src/lib/types.ts` | `lastMessage`, `unreadCount` fields on Session |
| `web/src/components/SessionList.tsx` | Preview line, unread badge, time groups, inline rename |
| `web/src/hooks/useChat.ts` | Handle `session.rename` send |

## Pillars Compliance

- **Performant**: No extra DB queries — `last_message`/`unread_count` updated inline. Sidebar memoization preserved.
- **Robust**: `data-testid` on all new elements. TypeScript types match backend.
- **Resilient**: Title gen failure is silent — keeps truncated title. Rename optimistic with rollback.
- **Secure**: Rename input sanitized. No raw HTML. Parameterized SQL.
- **Sovereign**: Haiku call via existing OAuth. No external services.
- **Transparent**: Title gen logged as event. Rename logged.
