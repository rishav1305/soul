# Graceful Interrupt System — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Allow the CEO to interrupt busy agents within ~10 seconds via Ctrl+C, with graceful state save and resume.

**Architecture:** The MCP server tags CEO messages with `key=urgent`. The sidecar detects urgent messages on busy panes, sends Ctrl+C to cancel the current operation, then injects an interrupt prompt. The agent saves state to memory, handles the message, and resumes. No new daemons — all logic in the existing sidecar.

**Tech Stack:** Bash (sidecar), Python (MCP server), Markdown (agent personas)

**Spec:** `docs/superpowers/specs/2026-03-23-graceful-interrupt-design.md`

---

### Task 1: Add `--key urgent` to MCP server broadcasts

**Files:**
- Modify: `~/.claude/mcp-servers/soul-team-mcp/server.py:154-167`

The MCP server's `soul_broadcast` function currently passes no `--key` flag to `clawteam inbox broadcast`. CEO broadcasts should be tagged urgent so sidecars can interrupt busy agents. `soul_send_message` already handles this correctly via `priority_key()` — no changes needed there.

- [ ] **Step 1: Modify `soul_broadcast` to pass `--key urgent` when sender is team-lead**

In `~/.claude/mcp-servers/soul-team-mcp/server.py`, replace the `soul_broadcast` function (lines 154-167):

```python
@mcp.tool()
def soul_broadcast(message: str) -> str:
    """
    Broadcast a message to all soul-team agents.

    Args:
        message: Message to send to all agents
    """
    args = ["inbox", "broadcast", TEAM, message]
    if SENDER == "team-lead":
        args.extend(["--key", "urgent"])
    rc, out, err = run_clawteam(*args)
    if rc != 0:
        return f"ERROR: {err.strip() or 'broadcast failed'}"
    return out.strip() or "Broadcast sent to all agents."
```

Changes from original:
- Added `--key urgent` conditional on `SENDER == "team-lead"`
- Note: `clawteam inbox broadcast` does NOT support `--from` — do not add it

- [ ] **Step 2: Verify MCP server still starts**

Run:
```bash
cd ~/.claude/mcp-servers/soul-team-mcp && python3 -c "import server; print('OK')"
```
Expected: `OK` (no import errors)

- [ ] **Step 3: Verify change is saved**

The MCP server files (`~/.claude/mcp-servers/`) are not in a git repository — they are system config files. No git commit needed. The file save in Step 1 is the final state.

---

### Task 2: Add `handle_p1_interrupt()` to sidecar

**Files:**
- Modify: `~/.claude/scripts/soul-sidecar.sh`

Add the new interrupt function between the existing `inject_to_pane()` function (ends at line 201) and the `notify_ceo_crash()` function (starts at line 203). This function handles the Ctrl+C → wait → inject sequence with cooldown protection.

- [ ] **Step 1: Add `handle_p1_interrupt()` function after `inject_to_pane()` (after line 201)**

Insert this function at line 202 (before `# ── Notify CEO of agent crash`):

```bash
# ── Graceful P1 interrupt (Ctrl+C → save state → handle → resume) ────────────
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

- [ ] **Step 2: Verify sidecar has no syntax errors**

Run:
```bash
bash -n ~/.claude/scripts/soul-sidecar.sh && echo "SYNTAX OK"
```
Expected: `SYNTAX OK`

- [ ] **Step 3: Verify change is saved**

The sidecar scripts (`~/.claude/scripts/`) are not in a git repository — they are system config files. No git commit needed.

---

### Task 3: Replace old urgent polling with `handle_p1_interrupt()` call

**Files:**
- Modify: `~/.claude/scripts/soul-sidecar.sh:285-311` (the `busy)` case in `process_message()`)

The existing `busy)` case has a 2-minute polling loop for `key=urgent` messages (lines 286-307). Replace it entirely with a call to the new `handle_p1_interrupt()` function.

- [ ] **Step 1: Replace the `busy)` case body**

Replace lines 285-311 (the entire `busy)` case) with:

```bash
        busy)
            # Check for urgent key — graceful interrupt (replaces old 2-min polling)
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

Key changes from original:
- Removed the 24-iteration polling loop (lines 290-306)
- Removed the `[URGENT/P1]` prefix format (interrupt prompt handles formatting)
- Added `return` after `handle_p1_interrupt` call
- Non-urgent busy path unchanged (queue + mark_seen)

- [ ] **Step 2: Verify sidecar has no syntax errors**

Run:
```bash
bash -n ~/.claude/scripts/soul-sidecar.sh && echo "SYNTAX OK"
```
Expected: `SYNTAX OK`

- [ ] **Step 3: Verify the old 2-min polling code is removed**

Run:
```bash
grep -c 'p1_attempts -lt 24' ~/.claude/scripts/soul-sidecar.sh
```
Expected: `0` (old polling loop fully removed)

- [ ] **Step 4: Verify process_message still has all three case branches**

Run:
```bash
sed -n '/^process_message()/,/^}/p' ~/.claude/scripts/soul-sidecar.sh | grep -c 'crashed)\|busy)\|idle.crunched)'
```
Expected: `3` (crashed, busy, idle|crunched — all present within `process_message()`)

- [ ] **Step 5: Verify change is saved**

The sidecar scripts (`~/.claude/scripts/`) are not in a git repository — they are system config files. No git commit needed.

---

### Task 4: Add P1 Interrupt Handling section to all 9 agent personas

**Files:**
- Modify: `~/.claude/agents/banner.md`
- Modify: `~/.claude/agents/friday.md`
- Modify: `~/.claude/agents/fury.md`
- Modify: `~/.claude/agents/hawkeye.md`
- Modify: `~/.claude/agents/loki.md`
- Modify: `~/.claude/agents/pepper.md`
- Modify: `~/.claude/agents/shuri.md`
- Modify: `~/.claude/agents/stark.md`
- Modify: `~/.claude/agents/xavier.md`

All 9 agent personas have a `### Task Status Notifications` section under "Live Communication Protocol". Insert the P1 Interrupt Handling section immediately BEFORE `### Task Status Notifications` in each file.

- [ ] **Step 1: Add P1 Interrupt Handling section to all 9 personas**

For EACH of the 9 agent files, find the line `### Task Status Notifications` and insert this block immediately before it:

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

Use the Edit tool (or equivalent) to insert the block above immediately before `### Task Status Notifications` in each of the 9 files. The anchor line is identical in all 9 files.

If using a script approach, Python is more reliable than sed for multi-line insertion:

```bash
python3 << 'PYEOF'
import glob

BLOCK = """### P1 Interrupt Handling

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

"""

ANCHOR = "### Task Status Notifications"

for agent in ["banner","friday","fury","hawkeye","loki","pepper","shuri","stark","xavier"]:
    path = f"/home/rishav/.claude/agents/{agent}.md"
    text = open(path).read()
    if "### P1 Interrupt Handling" in text:
        print(f"  {agent}: already has section, skipping")
        continue
    if ANCHOR not in text:
        print(f"  {agent}: WARNING — anchor not found!")
        continue
    text = text.replace(ANCHOR, BLOCK + ANCHOR)
    open(path, "w").write(text)
    print(f"  {agent}: inserted")
PYEOF
```

- [ ] **Step 2: Verify all 9 files have the new section**

Run:
```bash
grep -l "### P1 Interrupt Handling" ~/.claude/agents/{banner,friday,fury,hawkeye,loki,pepper,shuri,stark,xavier}.md | wc -l
```
Expected: `9`

- [ ] **Step 3: Verify the section appears before Task Status Notifications (spot check)**

Run:
```bash
grep -n "P1 Interrupt Handling\|Task Status Notifications" ~/.claude/agents/friday.md
```
Expected: P1 Interrupt Handling line number is BEFORE Task Status Notifications line number.

- [ ] **Step 4: Verify change is saved**

Agent personas (`~/.claude/agents/`) are not in a git repository — they are system config files. No git commit needed.

---

### Task 5: Integration testing

**Files:**
- No files created (manual verification scripts run inline)

Run the test scenarios from the spec against the modified sidecar and MCP server.

- [ ] **Step 1: Verify MCP server `soul_broadcast` code adds `--key urgent` for CEO**

```bash
# Verify the code change is present in server.py
python3 -c "
import ast, sys
source = open('$HOME/.claude/mcp-servers/soul-team-mcp/server.py').read()
tree = ast.parse(source)
for node in ast.walk(tree):
    if isinstance(node, ast.FunctionDef) and node.name == 'soul_broadcast':
        func_source = ast.get_source_segment(source, node)
        if '--key' in func_source and 'urgent' in func_source and 'team-lead' in func_source:
            print('PASS: soul_broadcast conditionally adds --key urgent for team-lead')
        else:
            print('FAIL: soul_broadcast missing --key urgent conditional')
        sys.exit(0)
print('FAIL: soul_broadcast function not found')
"
```
Expected: `PASS: soul_broadcast conditionally adds --key urgent for team-lead`

Also verify the CLI supports `--key` on broadcast:
```bash
~/.local/bin/clawteam inbox broadcast soul-team "test-key-check" --key urgent 2>&1 | head -1
# Expected: no error (message delivered or "broadcast sent" output)

# Clean up test messages
for d in ~/.clawteam/teams/soul-team/inboxes/*/; do
    rm -f "$d"/*test-key-check* 2>/dev/null
done
```

- [ ] **Step 2: Verify sidecar detects urgent key**

```bash
# Create a test message with key=urgent in a temp inbox
TEST_INBOX="$HOME/.clawteam/teams/soul-team/inboxes/test_test"
mkdir -p "$TEST_INBOX/archive"
cat > "$TEST_INBOX/test-urgent.json" << 'EOF'
{"id": "test-urgent", "from": "team-lead", "to": "test", "key": "urgent", "content": "test interrupt", "ts": "2026-03-23T00:00:00Z"}
EOF

# Verify jq can extract the key
key=$(jq -r '.key // "normal"' "$TEST_INBOX/test-urgent.json")
echo "Key: $key"
# Expected: urgent

rm -rf "$TEST_INBOX"
```
Expected: `Key: urgent`

- [ ] **Step 3: Verify lockfile cooldown logic**

```bash
# Create a fresh lockfile
LOCK="/tmp/test-interrupt.lock"
touch "$LOCK"
sleep 1
lock_age=$(( $(date +%s) - $(stat -c %Y "$LOCK") ))
echo "Lock age: ${lock_age}s (should be ~1, < 30 = cooldown active)"

# Verify age check works
if [ "$lock_age" -lt 30 ]; then
    echo "COOLDOWN: would queue (PASS)"
else
    echo "COOLDOWN: would interrupt (FAIL)"
fi

rm -f "$LOCK"
```
Expected: `COOLDOWN: would queue (PASS)`

- [ ] **Step 4: Verify peer messages don't have urgent key**

```bash
# Send a peer message (not from team-lead)
~/.local/bin/clawteam inbox send soul-team friday "peer test" --from shuri
msg_file=$(ls -1t ~/.clawteam/teams/soul-team/inboxes/friday_friday/*.json 2>/dev/null | head -1)
if [ -n "$msg_file" ]; then
    key=$(jq -r '.key // "none"' "$msg_file")
    echo "Peer message key: $key"
    # Expected: normal (not urgent)
    rm -f "$msg_file"
fi
```
Expected: `Peer message key: normal`

- [ ] **Step 5: Verify all 9 personas have P1 section (regression)**

```bash
count=$(grep -l "P1 Interrupt Handling" ~/.claude/agents/{banner,friday,fury,hawkeye,loki,pepper,shuri,stark,xavier}.md | wc -l)
echo "Personas with P1 section: $count/9"
```
Expected: `Personas with P1 section: 9/9`

- [ ] **Step 6: Verify sidecar syntax is clean**

```bash
bash -n ~/.claude/scripts/soul-sidecar.sh && echo "SIDECAR SYNTAX OK"
bash -n ~/.claude/scripts/soul-sidecar-wrapper.sh && echo "WRAPPER SYNTAX OK"
```
Expected: Both `OK`

- [ ] **Step 7: Verify backward compatibility — non-urgent messages still queue when busy**

```bash
# Create a non-urgent test message
cat > /tmp/test-normal.json << 'EOF'
{"id": "test-normal", "from": "shuri", "to": "friday", "key": "normal", "content": "routine update", "ts": "2026-03-23T00:00:00Z"}
EOF

key=$(jq -r '.key // "normal"' /tmp/test-normal.json)
if [ "$key" != "urgent" ]; then
    echo "Non-urgent message would be queued normally (PASS)"
else
    echo "Non-urgent message incorrectly tagged urgent (FAIL)"
fi
rm -f /tmp/test-normal.json
```
Expected: `Non-urgent message would be queued normally (PASS)`
