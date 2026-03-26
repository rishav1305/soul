# Message Formatting & Action Queue — Design Spec

*Date: 2026-03-25 | Status: Approved | Author: team-lead*

## Problem

Messages delivered to the CEO's team-lead pane have three issues:

1. **No visual hierarchy** — status updates, action items, and system alerts all look identical (`[agent] content`)
2. **Action items get lost** — the pane is a stream; subsequent messages push action items off screen
3. **No persistence** — once an action item scrolls away, there's no way to recover it without memory

## Solution

Two complementary mechanisms:

### 1. Message Format Redesign

Replace flat `[agent] content` with prefix-tagged messages based on message type.

#### Message Categories

| Prefix | Field trigger | Meaning |
|--------|--------------|---------|
| `⚡ ACTION [agent]` | `action_required: true, action_type: "action"` | Needs CEO to DO something (call, publish, send) |
| `❓ DECIDE [agent]` | `action_required: true, action_type: "decide"` | Needs CEO approval/decision |
| `✓ [agent]` | `action_required: false` or field absent | FYI status update, no action needed |
| `⚙ [source]` | `from: "courier"` or `from: "guardian"` | System/infra notification |

#### Examples

```
⚡ ACTION [hawkeye] Salary call with Aayushi OVERDUE — callback needed today
❓ DECIDE [pepper] Approve Digital.ai + Welldoc comp-risk templates? Brief: ~/briefs/hawkeye-t1-outreach-comprisk.md
✓ [xavier] 30 drills done, all DSA >92%. Toptal in 3 days.
⚙ [guardian] Shuri hit restart limit (3/3 this hour)
```

#### Backward Compatibility

Messages without `action_required` field default to `✓` (FYI). No agent-side changes required for basic functionality. Agents can opt in to action tagging by adding fields to their message JSON.

### 2. Persistent Action Queue

**File:** `~/soul-roles/shared/briefs/ceo-action-queue.md`

A persistent markdown file tracking unresolved CEO action items.

#### Format

```markdown
# CEO Action Queue
*Last updated: 2026-03-25 10:30 IST*

## Pending (3)
- [ ] **Call Aayushi** — salary negotiation, Ask Effi #54 | From: hawkeye | Since: Mar 24
- [ ] **Publish LinkedIn #1** — copy-paste SoulGraph teaser | From: loki | Since: Mar 24
- [ ] **Approve comp-risk templates** — Digital.ai + Welldoc | From: hawkeye | Since: Mar 25

## Resolved today (2)
- [x] ~~CARS API keys~~ — resolved via LiteLLM proxy | Mar 24
- [x] ~~Batch-approve 27 decisions~~ — delegated to Pepper | Mar 24
```

#### How Items Get Added

When courier delivers a message to team-lead with `action_required: true`:
1. Format and inject into pane with `⚡`/`❓` prefix
2. Append a `- [ ]` line to `ceo-action-queue.md` with summary, source agent, and timestamp
3. Dedup: skip if an identical summary already exists in pending section

#### How Items Get Resolved

Three resolution paths:
1. **Manual** — CEO edits file, changes `- [ ]` to `- [x]`
2. **CLI** — `clawteam action resolve <pattern>` marks matching items as resolved
3. **Agent-driven** — agent sends message with `resolves_action: "summary-substring"`, courier marks matching pending item as resolved

### 3. Periodic Action Reminder

Courier checks `ceo-action-queue.md` every 30 minutes. If pending items exist, injects a compact summary:

```
━━━ 3 PENDING ACTIONS ━━━
 1. ⚡ Call Aayushi — salary negotiation (since Mar 24)
 2. ⚡ Publish LinkedIn #1 — copy-paste from ~/briefs/loki-soulgraph-teaser-v2.md
 3. ❓ Approve comp-risk templates — ~/briefs/hawkeye-t1-outreach-comprisk.md
━━━━━━━━━━━━━━━━━━━━━━━━━
```

No reminder when queue is empty (zero noise when caught up).

## Implementation Changes

### File: `~/.claude/scripts/soul_courier/formatter.py`

**Change:** New `format_team_lead()` method replacing lines 31-34.

```python
@classmethod
def format_team_lead(cls, data: dict) -> str:
    from_user = data.get("from", "unknown")
    content = data.get("content", "")
    action = data.get("action", "")
    action_required = data.get("action_required", False)
    action_type = data.get("action_type", "action")

    # System notifications
    if from_user in ("courier", "guardian", "router"):
        prefix = "⚙"
        if action and action != "null":
            return f"{prefix} [{from_user}] {action}: {content}"
        return f"{prefix} [{from_user}] {content}"

    # Action items
    if action_required:
        prefix = "⚡ ACTION" if action_type == "action" else "❓ DECIDE"
        return f"{prefix} [{from_user}] {content}"

    # FYI status
    return f"✓ [{from_user}] {content}"
```

Update `format()` to call `format_team_lead()` when `agent == "team-lead"`.

### File: `~/.claude/scripts/soul_courier/daemon.py`

**Change 1:** In `_deliver()`, after successful delivery to team-lead, check for `action_required` and append to queue file.

```python
def _append_action_queue(self, data: dict) -> None:
    queue_path = Path.home() / "soul-roles/shared/briefs/ceo-action-queue.md"
    summary = data.get("action_summary", data.get("content", "")[:80])
    from_user = data.get("from", "unknown")
    action_type = data.get("action_type", "action")
    date = time.strftime("%b %d")
    line = f'- [ ] **{summary}** | From: {from_user} | Since: {date}\n'

    # Dedup: don't add if summary already in pending
    if queue_path.exists():
        existing = queue_path.read_text()
        if summary in existing:
            return

    # Append or create
    if not queue_path.exists():
        header = f"# CEO Action Queue\n*Last updated: {time.strftime('%Y-%m-%d %H:%M IST')}*\n\n## Pending\n"
        queue_path.write_text(header + line)
    else:
        with open(queue_path, "a") as f:
            f.write(line)
```

**Change 2:** Add reminder thread that runs every 30 minutes.

```python
def _reminder_loop(self) -> None:
    while self._running:
        time.sleep(1800)  # 30 minutes
        self._inject_action_reminder()

def _inject_action_reminder(self) -> None:
    queue_path = Path.home() / "soul-roles/shared/briefs/ceo-action-queue.md"
    if not queue_path.exists():
        return
    lines = queue_path.read_text().splitlines()
    pending = [l for l in lines if l.startswith("- [ ]")]
    if not pending:
        return
    reminder = f"━━━ {len(pending)} PENDING ACTIONS ━━━\n"
    for i, item in enumerate(pending, 1):
        # Extract bold text between ** **
        import re
        match = re.search(r'\*\*(.+?)\*\*', item)
        summary = match.group(1) if match else item[6:]
        reminder += f" {i}. {summary}\n"
    reminder += "━━━━━━━━━━━━━━━━━━━━━━━━━"
    self.pane_mgr.inject("team-lead", reminder)
```

### File: `clawteam` CLI

**Change:** Add `action` subcommand.

- `clawteam action list` — prints pending items from `ceo-action-queue.md`
- `clawteam action resolve <pattern>` — marks matching pending items as resolved (moves to Resolved section with `- [x]` and strikethrough)

### Message Schema Extension

Agents can include these optional fields in their message JSON:

```json
{
  "action_required": true,
  "action_type": "action",
  "action_summary": "Call Aayushi — salary negotiation",
  "resolves_action": "substring to match pending item"
}
```

All fields are optional. Messages without them behave exactly as before.

## Out of Scope

- Stale message replay fix (requires agent-side session tracking)
- Guardian idle-agent awareness (deferred to Approach C)
- Agent prompt updates to use `action_required` field (agents can be updated incrementally)

## Testing

1. Send test message with `action_required: true` — verify `⚡` prefix appears
2. Send test message without field — verify `✓` prefix appears
3. Verify `ceo-action-queue.md` gets created and populated
4. Wait 30 min or manually trigger reminder — verify compact summary appears
5. Resolve an item via CLI — verify it moves to resolved section
6. Verify no reminder when queue is empty
