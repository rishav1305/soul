# Graceful Interrupt System for soul-team

**Date:** 2026-03-23
**Author:** CEO + Brainstorm
**Status:** Draft

---

## Problem

Agents run long daily routines (5-15 minutes per step) and are unresponsive to CEO messages during execution. The sidecar delivers messages to the pane, but the agent doesn't process injected text until it finishes its current tool call chain and returns to the prompt. The CEO has no way to get a response within 30 seconds when an agent is busy.

## Solution

A graceful interrupt system that:
1. Leverages the existing `key=urgent` tagging that the MCP server already applies to CEO P1 messages
2. Adds `--key urgent` to CEO broadcasts (currently untagged)
3. When a `key=urgent` message arrives and the agent is busy, the sidecar sends Ctrl+C to cancel the current operation
4. The agent saves its interrupted state to memory, handles the CEO's message, then resumes from saved state

Non-urgent messages (peer-to-peer) continue to queue and deliver when idle (existing behavior, unchanged).

## Architecture

### Component Responsibilities

| Component | Role |
|-----------|------|
| **MCP server** (`server.py`) | Already tags CEO messages with `key=urgent`. Needs: add `--key urgent` to `soul_broadcast` for CEO |
| **Sidecar** (`soul-sidecar.sh`) | Detects `key=urgent` on busy pane → Ctrl+C → inject interrupt prompt. Replaces old 2-min polling logic |
| **Agent personas** (`*.md`) | New "P1 Interrupt Handling" section — save state, handle, resume |
| **Agent memory** | Stores interrupted state as `project` type memory for resume |

### Message Flow

```
CEO sends message via MCP
        │
        ▼
  MCP server sets key=urgent (already done for send, add to broadcast)
        │
        ▼
  Message written to agent inbox
        │
        ▼
  Sidecar detects via inotifywait
        │
        ▼
  Sidecar reads JSON → sees key=urgent
        │
        ├── Pane IDLE → normal injection (existing path)
        │
        └── Pane BUSY → graceful interrupt:
                │
                ├── 1. tmux send-keys Ctrl+C
                ├── 2. Wait 2s, verify prompt appeared
                ├── 3. Retry up to 3 times (~9s total, includes get_pane_state internal 1s delay)
                ├── 4a. SUCCESS → inject interrupt prompt
                └── 4b. FAILURE → fall back to queue, log warning
```

## Priority Tagging (MCP Server)

The MCP server (`server.py`) already handles P1 tagging for `soul_send_message` via the `priority_key()` helper:
- CEO calls `soul_send_message(to, message, priority="P1")` → passes `--key urgent` to clawteam CLI
- The `TeamMessage` Pydantic model has a `key` field that gets serialized into the JSON written to disk
- **No changes needed to `soul_send_message`** — it already works correctly

The only MCP server change: `soul_broadcast` currently does NOT pass `--key`. Add `--key urgent` when `SENDER == "team-lead"`:

```python
@mcp.tool()
def soul_broadcast(message: str) -> str:
    args = ["inbox", "broadcast", TEAM, message, "--from", SENDER]
    if SENDER == "team-lead":
        args.extend(["--key", "urgent"])
    rc, out, err = run_clawteam(*args)
    ...
```

Peer messages remain `key=normal` by default. The sidecar checks `key == "urgent"` regardless of sender — in the future, peers could send P1 via `soul_send_message(priority="P1")` without any sidecar changes.

**CLI bypass note:** Messages sent via `clawteam inbox send ... --from team-lead --key urgent` directly (bypassing MCP) WILL trigger the interrupt. This is desirable — the CLI is a valid CEO communication path.

## Graceful Interrupt Flow (Sidecar)

### New function: `handle_p1_interrupt()`

Added to `soul-sidecar.sh`. Called from `process_message()` when a message has `"key": "urgent"` and the pane state is `busy`.

```bash
handle_p1_interrupt() {
    local msg_file="$1"
    local msg_id="$2"

    # Check cooldown — prevent interrupt storms
    local lock_file="$TEAM_DIR/sidecar/${AGENT}-interrupt.lock"
    if [ -f "$lock_file" ]; then
        local lock_age=$(( $(date +%s) - $(stat -c %Y "$lock_file" 2>/dev/null || echo 0) ))
        if [ "$lock_age" -lt 30 ]; then
            log "P1 interrupt cooldown active (${lock_age}s < 30s) — queuing"
            queue_add "$msg_file"
            mark_seen "$msg_id"
            return
        fi
    fi

    # Attempt Ctrl+C up to 3 times
    local attempts=0
    while [ $attempts -lt 3 ]; do
        log "P1 interrupt: sending Ctrl+C (attempt $((attempts+1))/3)"
        tmux send-keys -t "$PANE_ID" C-c
        sleep 2

        local state
        state=$(get_pane_state)
        if [ "$state" = "idle" ] || [ "$state" = "crunched" ]; then
            log "P1 interrupt: agent returned to prompt"

            # Write cooldown lockfile
            touch "$lock_file"

            # Build and inject interrupt prompt (heredoc avoids printf format issues)
            local from content
            from=$(jq -r '.from // "unknown"' "$msg_file" 2>/dev/null)
            content=$(jq -r '.content // ""' "$msg_file" 2>/dev/null)

            local interrupt_text
            interrupt_text=$(cat <<INTERRUPT_EOF
[P1 INTERRUPT from ${from}]

STEP 1: Save your interrupted state to memory NOW.
Write a project memory titled "interrupted-state" containing:
- What task/routine step you were executing
- What you had completed so far
- What steps remain

STEP 2: Handle this message:
---
${content}
---
Respond via: clawteam inbox send soul-team ${from} "your response" --from ${AGENT}

STEP 3: After handling, check your memory for "interrupted-state". If found:
- Read it to recall where you left off
- Delete the memory (it is consumed)
- Continue from where you stopped
INTERRUPT_EOF
            )

            inject_to_pane "$interrupt_text"
            mv "$msg_file" "$ARCHIVE_DIR/" 2>/dev/null || true
            mark_seen "$msg_id"
            return
        fi

        attempts=$((attempts + 1))
    done

    # All 3 attempts failed — fall back to queue
    log "P1 interrupt FAILED after 3 attempts — agent unresponsive, queuing"
    queue_add "$msg_file"
    mark_seen "$msg_id"
}
```

### Modified `process_message()` — P1 check

The existing `busy` case in `process_message()` is modified to check for P1 before queuing:

```bash
busy)
    # Check for urgent key — graceful interrupt (replaces old 2-min polling logic)
    local msg_key
    msg_key=$(jq -r '.key // "normal"' "$msg_file" 2>/dev/null)
    if [ "$msg_key" = "urgent" ]; then
        handle_p1_interrupt "$msg_file" "$msg_id"
        return
    fi

    log "Pane busy — queuing $msg_id"
    queue_add "$msg_file"
    mark_seen "$msg_id"
    ;;
```

**Note:** This fully replaces the old `key == "urgent"` 2-minute polling logic (sidecar lines 286-307). That code path is removed — the new `handle_p1_interrupt()` is strictly better (Ctrl+C + inject vs. passive polling).

## Interrupt Cooldown

Prevents interrupt storms when the CEO sends multiple messages in rapid succession.

- **Lockfile:** `~/.clawteam/teams/soul-team/sidecar/{agent}-interrupt.lock`
- **Cooldown:** 30 seconds — if lockfile is younger than 30s, queue instead of interrupting. Any lockfile 30s or older allows the next interrupt (no separate staleness threshold needed).
- **Per-agent:** Each sidecar manages its own lockfile

**Multiple P1 messages during cooldown:** The second and third messages queue normally. Since the agent is already handling a CEO interrupt, it will drain the queue within a minute or two when it finishes — much faster than the 15-minute routine-step wait.

## Interrupt Prompt Template

The text injected into the pane after a successful Ctrl+C:

```
[P1 INTERRUPT from {from}]

STEP 1: Save your interrupted state to memory NOW.
Write a project memory titled "interrupted-state" containing:
- What task/routine step you were executing
- What you had completed so far
- What steps remain

STEP 2: Handle this message:
---
{message_content}
---
Respond via: clawteam inbox send soul-team {from} "your response" --from {agent}

STEP 3: After handling, check your memory for "interrupted-state". If found:
- Read it to recall where you left off
- Delete the memory (it is consumed)
- Continue from where you stopped
```

Design choices:
- **Numbered steps** — Claude follows explicit numbered instructions more reliably than prose
- **Self-contained** — Full message content included, no need to read inbox
- **Memory-based state** — Uses `project` type memory (existing type, no schema changes)
- **Consumed on resume** — State memory is deleted after reading to prevent stale state accumulation

## Agent Persona Changes

All 9 agent personas (`~/.claude/agents/*.md`) get a new subsection under "Live Communication Protocol":

```markdown
### P1 Interrupt Handling

When you see a `[P1 INTERRUPT from ...]` message, it means you were interrupted mid-task:

1. **Save state immediately** — Write a project memory titled "interrupted-state" with:
   - What task/routine step you were executing
   - What you had completed so far
   - What steps remain
2. **Handle the message** — Process the CEO's request fully
3. **Resume** — After handling, check your memory for "interrupted-state". If found:
   - Read it to recall where you left off
   - Delete the memory (it's consumed)
   - Continue from where you stopped

If you don't remember what you were doing (context was crunched), just proceed to your next routine step — don't waste time reconstructing lost context.
```

## Files Modified

| File | Change |
|------|--------|
| `~/.claude/mcp-servers/soul-team-mcp/server.py` | Add `--key urgent` to `soul_broadcast` when sender is team-lead (send already works) |
| `~/.claude/scripts/soul-sidecar.sh` | New `handle_p1_interrupt()` function, modified `process_message()` to check `key=urgent` (replaces old 2-min polling), lockfile logic |
| `~/.claude/agents/banner.md` | Add P1 Interrupt Handling section |
| `~/.claude/agents/friday.md` | Add P1 Interrupt Handling section |
| `~/.claude/agents/fury.md` | Add P1 Interrupt Handling section |
| `~/.claude/agents/hawkeye.md` | Add P1 Interrupt Handling section |
| `~/.claude/agents/loki.md` | Add P1 Interrupt Handling section |
| `~/.claude/agents/pepper.md` | Add P1 Interrupt Handling section |
| `~/.claude/agents/shuri.md` | Add P1 Interrupt Handling section |
| `~/.claude/agents/stark.md` | Add P1 Interrupt Handling section |
| `~/.claude/agents/xavier.md` | Add P1 Interrupt Handling section |

## Files NOT Modified

- `soul-team.sh` — No launch changes
- `soul-team.toml` — No config changes
- `soul-sidecar-wrapper.sh` — Restart logic unchanged
- `soul-router.py`, `soul-bridge.py`, `soul-heartbeat.sh` — Untouched
- Memory system — Using existing `project` type
- `clawteam` CLI — Already supports `--key urgent` flag; no changes needed

## Edge Cases

| Case | Behavior |
|------|----------|
| CEO sends urgent to idle agent | Normal injection, no Ctrl+C needed |
| CEO sends urgent to busy agent | Ctrl+C → save → handle → resume |
| CEO sends 3 messages in 10 seconds | First triggers interrupt, next 2 queue (30s cooldown) |
| Ctrl+C fails 3 times (~9s) | Fall back to queue, log warning |
| Agent context crunched during interrupt | Agent skips resume, moves to next routine step |
| Peer sends normal message to busy agent | Queued as today (no change) |
| Agent crashes during interrupt handling | Sidecar detects crash, notifies CEO (existing behavior) |
| P1 arrives during P1 handling (cooldown active) | Queued — agent drains queue after handling first P1 |
| Lockfile 30s+ old | Cooldown expired, next P1 will interrupt |
| Agent pane is dead/crashed when P1 arrives | Queued + CEO notified (existing crash path) |

## Testing

1. **P1 to idle agent** — verify normal injection (no Ctrl+C fired)
2. **P1 to busy agent** — verify Ctrl+C sent, prompt detected, interrupt prompt injected
3. **3 rapid P1 messages** — verify first interrupts, next 2 queue, all 3 eventually processed
4. **Ctrl+C failure** — simulate unresponsive pane, verify fallback to queue after 3 attempts
5. **Lockfile lifecycle** — verify creation on interrupt, 30s cooldown expiry
6. **MCP server tagging** — verify CEO broadcasts have `key=urgent`, peer messages don't
7. **Agent resume** — verify agent writes interrupted-state memory, handles message, reads memory and resumes
8. **Backward compatibility** — verify non-P1 messages still queue and deliver on idle (regression)

## Known Limitations (Acceptable)

- **Ctrl+C may lose in-flight tool output** — If the agent was mid-Read or mid-Bash, the output is lost. The agent must re-read files or re-run commands after resuming. This is acceptable because the alternative is 15-minute unresponsiveness.
- **Resume quality depends on agent discipline** — The agent must actually write a useful state memory. If it writes "I was doing stuff," the resume will be poor. This improves over time as agents learn the pattern.
- **30s cooldown means rapid-fire CEO messages queue** — Acceptable because the agent is already in responsive mode from the first interrupt.
- **Ctrl+C may trigger a confirmation prompt** — Some Claude Code operations may show a "Cancel?" prompt after Ctrl+C instead of returning to the main prompt. The retry logic will exhaust its 3 attempts and fall back to queuing. `get_pane_state()` treats these as `busy` since they don't match the idle prompt pattern.

## Non-Goals

- No changes to peer-to-peer messaging behavior
- No changes to `clawteam` CLI (already supports `--key urgent`)
- No hook-based interrupt detection
- No separate interrupt daemon
- No changes to daily routines or boot prompt
