# Soul Courier — Reliable Message Delivery Daemon

**Date:** 2026-03-23
**Status:** Approved
**Replaces:** soul-sidecar.sh, soul-sidecar-wrapper.sh, soul-bridge.py

---

## Problem

The soul-team multi-agent communication system drops messages. Messages land in agent inboxes (filesystem) but are never injected into agent tmux panes. Root causes:

1. **Stale sidecar processes** — Old tmux sessions leave orphaned sidecars that race with current ones, marking messages "seen" without injecting them.
2. **Fragile pane state detection** — Bash regex matching on tmux pane content misidentifies idle agents as "busy", causing messages to queue indefinitely.
3. **No injection verification** — Sidecar assumes tmux paste-buffer always works. Silent failures mean messages are marked delivered but never appear.
4. **No session continuity** — Killing and relaunching `soul-team` leaves zombie processes. No way to reconnect to an existing session with healing.

## Solution

Replace the per-agent bash sidecars and soul-bridge with a single Python daemon (`soul-courier.py`) managed by systemd. Add `--continue` flag to `soul-team.sh` for session recovery.

## Architecture

### Components Modified

| Component | Action | Reason |
|-----------|--------|--------|
| `soul-courier.py` | NEW | Single daemon replaces 10 sidecars + bridge |
| `soul-courier.service` | NEW | systemd unit for daemon lifecycle |
| `soul-team.sh` | MODIFY | Add `--continue`, add stale cleanup, replace sidecar spawn with courier start |
| `soul-sidecar.sh` | RETIRE | Replaced by soul-courier |
| `soul-sidecar-wrapper.sh` | RETIRE | Replaced by soul-courier |
| `soul-bridge.py` | RETIRE | Absorbed into soul-courier |

### Components Untouched

- `soul-router.py` — Broadcast fan-out and discussion threading (working)
- `clawteam` CLI/library — Message writing with atomic rename (working)
- MCP server (`server.py`) — Wraps clawteam (working)
- `soul-heartbeat.sh` — Independent health monitoring
- `soul-guardian.service` — Agent crash recovery watchdog

### Message Flow

```
CEO sends soul_send_message()
  -> MCP server calls clawteam CLI
    -> clawteam writes .tmp file -> atomic rename to inbox dir
      -> soul-courier detects moved_to via watchdog
        -> checks pane state (regex fast-path)
        -> if idle: inject via tmux + verify injection
        -> if verify fails: queue with exponential backoff
        -> if busy: queue
        -> mirror to native inbox JSON (replaces soul-bridge)
```

---

## soul-courier.py — Detailed Design

### Thread Architecture

```
Main Thread (CourierDaemon)
  |-- WatchdogObserver thread (filesystem events for all inbox dirs)
  |-- QueueDrainer thread (10s periodic retry of queued messages)
  |-- HealthChecker thread (60s periodic pane validation + orphan cleanup)
```

### Classes

#### CourierDaemon

Entry point. Responsibilities:
- Read `~/.clawteam/teams/soul-team/panes.json` on startup
- Initialize all threads
- Handle SIGTERM: flush queues to disk, stop observer, exit clean
- Catch-up: scan all inbox dirs for unseen messages on startup

#### InboxWatcher (FileSystemEventHandler)

Single watchdog observer registered on `~/.clawteam/teams/soul-team/inboxes/` with `recursive=True` (message files land in subdirectories like `inboxes/friday_friday/msg-*.json`).

- Watches `EVENT_TYPE_MOVED` events (atomic rename from clawteam)
- `recursive=True` — required because files land in `inboxes/{agent}_{agent}/` subdirs, not directly in `inboxes/`
- Parses agent name from path: `inboxes/{agent}_{agent}/msg-*.json` (extract first `{agent}` from doubled dir name)
- Dispatches to `PaneManager.deliver(agent, msg_file)`
- Ignores non-JSON files, `archive/` subdirectories, and `.tmp-*` files (in-progress writes)

#### PaneManager

Owns pane state for all agents. Thread-safe via per-agent `threading.Lock`. Uses a per-agent state cache with 3-second TTL to avoid redundant detection during bursts (e.g., broadcast fan-out).

**`detect_state(agent) -> str`**

```python
def detect_state(self, agent: str) -> str:
    """Detect agent pane state. Returns: idle, crunched, busy, crashed, dead.
    Uses per-agent cache with 3s TTL to avoid redundant detection during bursts."""

    # Check cache first (3s TTL)
    cached = self._state_cache.get(agent)
    if cached and time.monotonic() - cached[1] < 3.0:
        return cached[0]

    pane_id = self.panes.get(agent)
    if not pane_id:
        return "dead"

    content = self._tmux_capture(pane_id, lines=15)
    if content is None:
        return "dead"

    # Fast path: regex patterns
    if re.search(r'^\$\s*$|^[a-z]+@[a-zA-Z0-9-]+[^>]*\$\s*$', content, re.M):
        state = "crashed"
        self._state_cache[agent] = (state, time.monotonic())
        return state

    # Check all non-empty lines for idle prompt (❯ or > at end of line)
    non_empty = [l for l in content.splitlines() if l.strip()]
    for line in non_empty:
        if re.search(r'[❯>]\s*$', line):
            # Distinguish crunched from idle (crunched needs thread summary in formatting)
            state = "crunched" if "Crunched" in content else "idle"
            self._state_cache[agent] = (state, time.monotonic())
            return state

    if "Crunched" in content:
        state = "crunched"
        self._state_cache[agent] = (state, time.monotonic())
        return state

    # Ambiguous: compare snapshots 1.5s apart
    # NOTE: This runs OUTSIDE the per-agent lock (lock is released before calling
    # detect_state, re-acquired after). The deliver() method handles this — see below.
    snapshot1 = content
    time.sleep(1.5)
    snapshot2 = self._tmux_capture(pane_id, lines=15)

    if snapshot2 and any(re.search(r'[❯>]\s*$', l) for l in snapshot2.splitlines() if l.strip()):
        state = "crunched" if "Crunched" in (snapshot2 or "") else "idle"
    elif snapshot1 == snapshot2:
        state = "idle"  # nothing changed = probably idle
    else:
        state = "busy"

    self._state_cache[agent] = (state, time.monotonic())
    return state
```

Note: Uses both `❯` (Unicode) and `>` (ASCII) for matching. The state cache (3s TTL) prevents repeated 1.5s sleeps during burst delivery (e.g., broadcast fan-out of 9 messages). `crunched` is returned as a distinct state so message formatting can include thread summaries.

**`inject(agent, text) -> bool`**

```python
def inject(self, agent: str, text: str) -> bool:
    """Inject text into agent's tmux pane using named buffers to prevent cross-agent races."""
    pane_id = self.panes[agent]
    buf_name = f"courier-{agent}"  # Named buffer per agent — prevents global buffer race
    with tempfile.NamedTemporaryFile(mode='w', suffix='.txt', delete=False) as f:
        f.write(text)
        tmp_path = f.name
    try:
        subprocess.run(["tmux", "load-buffer", "-b", buf_name, tmp_path], check=True)
        subprocess.run(["tmux", "paste-buffer", "-b", buf_name, "-t", pane_id, "-d"], check=True)
        time.sleep(0.3)
        subprocess.run(["tmux", "send-keys", "-t", pane_id, "Enter"], check=True)
        return True
    except subprocess.CalledProcessError:
        return False
    finally:
        os.unlink(tmp_path)
```

Note: `-b courier-{agent}` uses a named tmux buffer per agent, preventing races when injecting to multiple agents concurrently. `-d` on paste-buffer deletes the named buffer after pasting.

**`verify_injection(agent, fragment) -> bool`**

```python
def verify_injection(self, agent: str, fragment: str) -> bool:
    time.sleep(2)
    content = self._tmux_capture(self.panes[agent], lines=30)
    return content is not None and fragment in content
```

**`deliver(agent, msg_file) -> bool`**

```python
def deliver(self, agent: str, msg_file: Path) -> bool:
    """Deliver a message to an agent's tmux pane.

    Lock strategy: detect_state() is called WITHOUT the lock (may sleep 1.5s for
    ambiguous states). The lock is only held during inject + verify (fast path).
    This prevents lock contention during broadcast bursts.

    Seen-log strategy: Only mark seen on SUCCESSFUL delivery + verification.
    Queued messages are NOT marked seen — on crash recovery, the startup catch-up
    scan will re-discover them. Deduplication at delivery time via queue membership check.
    """
    # Detect state WITHOUT lock (may take 1.5s for ambiguous)
    state = self.detect_state(agent)
    msg_data = self._read_msg(msg_file)
    priority = msg_data.get("key", "normal")

    # P1 interrupt: attempt Ctrl+C to preempt busy agent
    if priority == "urgent" and state == "busy":
        state = self._handle_p1_interrupt(agent)

    with self.locks[agent]:
        if state in ("idle", "crunched"):
            is_crunched = (state == "crunched")
            text = self.format_message(msg_file, agent, is_crunched=is_crunched)
            if self.inject(agent, text):
                # Verify using sender name + first 30 chars of content (appears in formatted text)
                from_user = msg_data.get("from", "unknown")
                fragment = f"From: {from_user}"
                if self.verify_injection(agent, fragment):
                    self._archive(msg_file)
                    self._mark_seen(msg_file)  # Only mark seen on successful delivery
                    self._mirror_native(msg_file, agent)
                    return True
            # Inject or verify failed — queue but do NOT mark seen
            self.queue.add(agent, msg_file)
            return False

        if state == "crashed":
            self.queue.add(agent, msg_file)
            self._notify_ceo(agent, "crashed")
        elif state == "dead":
            self.queue.add(agent, msg_file)
            self._notify_ceo(agent, "dead")
        else:  # busy
            self.queue.add(agent, msg_file)

        # Mirror to native inbox even when queued (CEO can see pending messages)
        self._mirror_native(msg_file, agent)
        # Do NOT mark seen — message must be re-discoverable on crash recovery
        return False

def _handle_p1_interrupt(self, agent: str) -> str:
    """Send Ctrl+C to preempt a busy agent, wait for idle, return new state.
    Includes 30s cooldown lock to prevent interrupt storms."""
    lock_file = Path(f"~/.clawteam/teams/soul-team/sidecar/{agent}-interrupt.lock").expanduser()
    if lock_file.exists() and time.time() - lock_file.stat().st_mtime < 30:
        return "busy"  # Cooldown active

    for attempt in range(3):
        pane_id = self.panes[agent]
        subprocess.run(["tmux", "send-keys", "-t", pane_id, "C-c"])
        time.sleep(2)
        self._state_cache.pop(agent, None)  # Invalidate cache before re-detecting
        state = self.detect_state(agent)
        if state in ("idle", "crunched"):
            lock_file.touch()  # Set cooldown
            return state
    return "busy"  # All attempts failed
```

**Message formatting** — identical to current sidecar output, all message types preserved:

- **Direct:** `[INBOX] From: {from} | Type: {type}\n---\n{content}\n---\nRespond via: clawteam inbox send soul-team {from} "your response" --from {agent}`
- **Broadcast:** `[BROADCAST] From: {from}\n---\n{content}\n---\nRespond to CEO inbox via: clawteam inbox send soul-team team-lead "your response" --from {agent}`
- **Group discussion:** `[DISCUSSION: {thread_id}] From: {from} (message {n})\n---\n{content}\n---\nRespond by writing to discussions/{thread_id}/ with filename: {ts}-{agent}.json\nKeep it under 200 words. Reference peers by name.`
- **Group discussion (crunched):** Same as above but prepended with thread summary built from `discussions/{thread_id}/*.json` (sender + first 100 chars of each prior message)
- **P1 interrupt:** `[P1 INTERRUPT from {from}]\nSTEP 1: Save your interrupted state to memory NOW...\nSTEP 2: Handle this message:\n---\n{content}\n---\nSTEP 3: After handling, check memory for "interrupted-state"...`
- **CEO (team-lead):** Minimal format: `[{from}] {content}` or `[{from}] {action}: {content}`

**Overflow batching:** When 3+ messages from the same discussion thread are queued for an agent, they are collapsed into a batch summary instead of individual injection:
`[DISCUSSION: {thread_id}] {n} new messages since your last response:\n- {from1}: "{excerpt}"\n- {from2}: "{excerpt}"\n---\nRespond to the thread or say "acknowledged" if nothing to add.`

#### MessageQueue

Per-agent in-memory deque with disk persistence.

- `add(agent, msg_file)` — Append to agent's deque
- `pop(agent) -> Path | None` — Pop oldest message
- `flush()` — Write all queues to `~/.clawteam/teams/soul-team/queue/{agent}.json`
- Auto-flush every 30s via timer thread
- On startup: load from disk files, validate paths exist

#### QueueDrainer

Runs every 10s in a dedicated thread.

- Iterates agents with queued messages
- Per-agent exponential backoff: 10s -> 20s -> 40s -> 120s max
- Backoff resets on successful delivery
- 5 consecutive failures -> log warning + notify CEO (continues retrying at 120s)

#### HealthChecker

Runs every 60s in a dedicated thread.

1. Read `panes.json`
2. Run `tmux list-panes -t soul-team -F "#{pane_id}"` to get live panes
3. For each agent: if `panes.json` pane_id not in live list -> mark dead
4. Kill any orphaned legacy processes (migration period only — skip if no processes found):
   `pkill -f "soul-sidecar" 2>/dev/null; pkill -f "soul-bridge" 2>/dev/null`
5. Verify watchdog observer is alive — restart if thread died
6. Log health status to `~/.claude/logs/soul-courier-health.log`

#### NativeBridger

Synchronous mirror (not a thread — called inline during deliver). Mirrors only to team-lead's native inbox, matching existing soul-bridge behavior.

- On message arrival for team-lead (delivered or queued), append to `~/.claude/teams/soul-team/inboxes/team-lead.json`
- Only mirrors messages TO team-lead — other agents' native inbox files are unused (clawteam inboxes are the source of truth for agents)
- Replaces soul-bridge.py's 2s polling with instant mirroring
- Uses file locking (`fcntl.flock`) to prevent corruption from concurrent writes

---

## soul-team.sh — Launcher Changes

### Fresh Launch: `soul-team`

Prepend cleanup before existing logic:

```bash
cleanup_stale() {
    # Kill old bash sidecars
    pkill -f "soul-sidecar.sh" 2>/dev/null
    pkill -f "soul-sidecar-wrapper.sh" 2>/dev/null

    # Stop old courier and bridge
    systemctl --user stop soul-courier.service 2>/dev/null
    pkill -f "soul-bridge.py" 2>/dev/null

    # Kill old soul-team tmux session
    tmux kill-session -t soul-team 2>/dev/null

    # Clean seen logs (fresh delivery tracking)
    rm -f ~/.clawteam/teams/soul-team/sidecar/*-seen.log

    # Reset queues
    for f in ~/.clawteam/teams/soul-team/queue/*.json; do
        [ -f "$f" ] && echo '[]' > "$f"
    done
}
```

Replace per-agent sidecar spawn loop with:

```bash
# Start courier daemon (replaces sidecars + bridge)
systemctl --user start soul-courier.service
```

Remove:
- `soul-bridge.py &` launch
- `for agent in ...; do soul-sidecar-wrapper.sh $agent $pane_id & done`

### Continue Mode: `soul-team --continue`

```bash
if [ "$1" = "--continue" ]; then
    # Phase 1: Validate session exists
    if ! tmux has-session -t soul-team 2>/dev/null; then
        echo "No soul-team session found. Run 'soul-team' to start fresh."
        exit 1
    fi

    # Phase 2: Heal — restart courier with fresh pane validation
    systemctl --user restart soul-courier.service
    pgrep -f "soul-router.py" >/dev/null || python3 ~/.claude/scripts/soul-router.py &

    # Phase 3: Revive dead agents
    PANES_JSON="$HOME/.clawteam/teams/soul-team/panes.json"
    for agent in $(jq -r 'keys[]' "$PANES_JSON"); do
        pane_id=$(jq -r ".\"$agent\"" "$PANES_JSON")
        content=$(tmux capture-pane -p -t "$pane_id" -S -5 2>/dev/null)

        if [ $? -ne 0 ] || echo "$content" | grep -qE '^\$\s*$|^[a-z]+@[a-zA-Z0-9-]+[^>]*\$\s*$'; then
            echo "Reviving $agent in pane $pane_id..."
            # Look up agent launch command from config
            launch_cmd=$(get_agent_launch_cmd "$agent")
            tmux send-keys -t "$pane_id" "$launch_cmd" Enter
        fi
    done

    # Phase 4: Attach
    tmux attach-session -t soul-team
    exit 0
fi
```

---

## soul-courier.service — systemd Unit

```ini
[Unit]
Description=Soul Team Courier — Message delivery daemon
After=network.target

[Service]
Type=simple
ExecStart=/usr/bin/python3 %h/.claude/scripts/soul-courier.py
Restart=always
RestartSec=3
Environment=SOUL_TEAM_NAME=soul-team

CPUQuota=50%
MemoryMax=256M

TimeoutStopSec=10
KillMode=mixed
KillSignal=SIGTERM

StandardOutput=journal
StandardError=journal
SyslogIdentifier=soul-courier

[Install]
WantedBy=default.target
```

User-level service: `~/.config/systemd/user/soul-courier.service`

---

## Error Handling

| Scenario | Behavior |
|----------|----------|
| Inject succeeds, verify fails | Queue with backoff (10s -> 20s -> 40s -> 120s max) |
| 5 consecutive verify failures | Log warning, notify CEO inbox, continue retrying at 120s |
| tmux pane dead | Mark agent dead, stop retrying, notify CEO |
| Daemon crash | systemd restarts in 3s, catch-up from disk queue + inbox scan |
| Agent crash during injection | Message in pane history, visible after `--continue` revival |
| Two simultaneous messages for same agent | Per-agent lock serializes delivery |
| Stale panes.json | HealthChecker (60s) cross-refs with live tmux, marks dead agents |
| Queue file corruption | Reset to `[]` on startup, log warning |
| Watchdog observer crash | HealthChecker detects, restarts observer. Fallback: 5s polling |
| Race: old sidecar still running | HealthChecker kills orphaned sidecar/bridge processes |

## Migration

**Zero-downtime migration via fresh `soul-team` launch:**

1. User runs `soul-team`
2. Cleanup preamble kills old sidecars, bridge, old tmux session
3. Agents launch in new tmux session
4. `soul-courier.service` starts instead of per-agent sidecars
5. Courier reads `panes.json`, watches same inbox dirs, uses same queue files
6. Agents notice nothing — message format in panes is identical

**Rollback:** Stop `soul-courier.service`, restore `soul-team.sh` from git, run fresh `soul-team` which spawns old bash sidecars. Queue/inbox files are compatible.

**Retired files** (renamed to `.bak`, removed after 1 week):
- `~/.claude/scripts/soul-sidecar.sh`
- `~/.claude/scripts/soul-sidecar-wrapper.sh`
- `~/.claude/scripts/soul-bridge.py`

## File Locations

| File | Path |
|------|------|
| Courier daemon | `~/.claude/scripts/soul-courier.py` |
| systemd unit | `~/.config/systemd/user/soul-courier.service` |
| Launcher | `~/.claude/scripts/soul-team.sh` (modified) |
| Courier log | `journalctl --user -u soul-courier` (systemd journal, auto-rotated) |
| Health log | `~/.claude/logs/soul-courier-health.log` (small, periodic status only) |
| Panes map | `~/.clawteam/teams/soul-team/panes.json` |
| Queues | `~/.clawteam/teams/soul-team/queue/{agent}.json` |
| Seen logs | `~/.clawteam/teams/soul-team/sidecar/{agent}-seen.log` |
| Inboxes | `~/.clawteam/teams/soul-team/inboxes/{agent}_{agent}/` |
| Native mirror | `~/.claude/teams/soul-team/inboxes/{agent}.json` |

## Dependencies

- Python 3.12 (already installed)
- `watchdog` package (already installed — used by soul-router)
- systemd user services (already in use — soul-guardian)
- tmux (already installed)
- clawteam CLI (already installed)
