# Pane Noise Reduction — Approach B

*Date: 2026-03-25 | Status: Approved | Author: team-lead*

## Problem

The CEO's team-lead pane receives excessive noise from three sources:

1. **Courier team-lead false positive** (445+ messages/session): Courier detects team-lead's Claude Code prompt as "crashed", fires `_notify_ceo` which writes to team-lead's own inbox, creating a feedback loop.
2. **Guardian restart spam** (27 P1 interrupts/hour peak): Every agent restart fires a P1 notification. Idle agents exit to shell, Guardian restarts them (3/hr × 9 agents), each generating a P1 interrupt.
3. **Stale message replay**: Restarted agents re-process routines and re-send status updates already delivered in previous sessions. (Out of scope for this fix — requires agent-side session awareness.)

## Solution: Approach B — Smart Filtering

### Change 1: Courier — Skip team-lead notifications

**File:** `~/.claude/scripts/soul_courier/daemon.py`
**Function:** `_notify_ceo()`

Add early return if the agent being reported is `team-lead`. The team-lead pane IS the CEO — notifying yourself that you're dead is nonsensical.

```python
def _notify_ceo(self, agent: str, status: str) -> None:
    if agent == "team-lead":
        return  # Never notify CEO about CEO's own pane
    ...
```

### Change 2: Courier — Rate-limit CEO notifications

**File:** `~/.claude/scripts/soul_courier/daemon.py`
**Function:** `_notify_ceo()`

Add per-agent rate limiting: max 1 notification per agent per 10 minutes. Uses a dict tracking last notification time per agent.

```python
# New instance variable in __init__:
self._last_ceo_notify: dict[str, float] = {}

# In _notify_ceo, after team-lead check:
now = time.time()
last = self._last_ceo_notify.get(agent, 0)
if now - last < 600:  # 10 minutes
    return
self._last_ceo_notify[agent] = now
```

### Change 3: Guardian — Notify only on limit hit

**File:** `~/.claude/scripts/soul-guardian.py`
**Function:** `maybe_restart_agent()`

Only send CEO notification when restart count reaches MAX_RESTARTS_PER_HOUR (3/3), not on every restart. This reduces notifications from 3×9=27/hr to at most 9/hr (and only when agents are genuinely unstable).

```python
# Line ~605: Move notify_ceo inside a conditional
if count + 1 >= MAX_RESTARTS_PER_HOUR:
    notify_ceo(
        f"[guardian] {state.name} hit restart limit "
        f"({MAX_RESTARTS_PER_HOUR}/{MAX_RESTARTS_PER_HOUR} this hour)"
    )
```

### Change 4: Guardian — Downgrade restart notifications to P2

**File:** `~/.claude/scripts/soul-guardian.py`
**Function:** `notify_ceo()`

Change priority from P1 to P2. Restarts are informational, not urgent — the agent is already being handled automatically.

```python
r = subprocess.run(
    [str(SOUL_MSG), "send", "team-lead", message, "--priority", "P2"],
    ...
)
```

## Expected Impact

| Noise source | Before | After |
|-------------|--------|-------|
| Courier team-lead | 445+/session | 0 |
| Courier per-agent dead/failing | Unlimited | Max 1 per 10 min per agent |
| Guardian restart P1s | 27/hr peak | Max 9/hr, P2, only on limit hit |
| **Total CEO interrupts** | **~500+/session** | **~9/hr worst case** |

## Out of Scope

- Stale message replay (requires agent-side session tracking)
- Guardian idle-agent awareness (Approach C, deferred)
- Courier delivery-failing counter reset on success (minor, separate fix)

## Testing

1. Apply changes
2. Restart `soul-courier.service` and `soul-guardian.service`
3. Verify: no more `[courier] dead: Agent team-lead pane is dead` messages
4. Verify: agent restarts produce at most 1 notification per cycle
5. Monitor for 30 minutes to confirm noise reduction
