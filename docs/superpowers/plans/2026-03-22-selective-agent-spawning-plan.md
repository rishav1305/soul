# Selective Agent Spawning Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add `--agents <list>` and `--all` flags to `soul-team.sh` so only selected agents spawn, with a dynamic tmux layout that adapts to the count.

**Architecture:** Single-file modification to `~/.claude/scripts/soul-team.sh`. Replaces 4 hardcoded sections (TOML parse → layout → launch → sidecars) with dynamic equivalents driven by `SELECTED_AGENTS` array. No other files change.

**Tech Stack:** Bash, tmux, python3 (inline TOML parsing)

**Spec:** `docs/superpowers/specs/2026-03-22-selective-agent-spawning-design.md`

---

## File Map

| File | Action | Responsibility |
|------|--------|---------------|
| `~/.claude/scripts/soul-team.sh` | Modify | All changes — flag parsing, layout, launch loop, sidecars |

No new files created. No other files modified.

---

### Task 1: TOML Extraction + Flag Parsing + Validation

**Files:**
- Modify: `~/.claude/scripts/soul-team.sh:36-55` (TOML parsing section)
- Modify: `~/.claude/scripts/soul-team.sh:82-85` (pre-flight section)

This task adds: agent name extraction from TOML, `AGENT_MACHINE` associative array, `--agents`/`--all` flag parsing, deduplication, validation, and conditional SSH pre-flight. All inserted between the existing TOML config parsing (line 55) and the ClawTeam init (line 57).

- [ ] **Step 1: Add TOML agent extraction after existing TOML parsing (after line 55)**

Insert this block right after the `fi` that closes the TOML config parsing (line 55), before the ClawTeam init:

```bash
# ── Extract agent names + machine types from TOML ────────────────────────────
ALL_AGENTS_STR=$(python3 -c "
import re
txt = open('$TOML').read()
blocks = re.split(r'\[\[agents\]\]', txt)[1:]
names = []
for b in blocks:
    m = re.search(r'name\s*=\s*\"(\w+)\"', b)
    if m: names.append(m.group(1))
print(' '.join(names))
" 2>/dev/null)
read -ra ALL_AGENTS <<< "$ALL_AGENTS_STR"

declare -A AGENT_MACHINE
eval "$(python3 -c "
import re
txt = open('$TOML').read()
blocks = re.split(r'\[\[agents\]\]', txt)[1:]
for b in blocks:
    nm = re.search(r'name\s*=\s*\"(\w+)\"', b)
    mc = re.search(r'machine\s*=\s*\"(\w[\w-]*)\"', b)
    if nm and mc:
        print(f'AGENT_MACHINE[{nm.group(1)}]={mc.group(1)}')
" 2>/dev/null)"
```

- [ ] **Step 2: Add flag parsing block (immediately after Step 1)**

Insert right after the TOML extraction, before ClawTeam init:

```bash
# ── Parse CLI flags ──────────────────────────────────────────────────────────
SELECTED_AGENTS=()
USE_ALL=false

while [[ $# -gt 0 ]]; do
    case "$1" in
        --agents)
            if [[ -z "${2:-}" || "$2" == --* ]]; then
                echo "Error: --agents requires a comma-separated list of agent names" >&2
                exit 1
            fi
            IFS=',' read -ra SELECTED_AGENTS <<< "$2"
            shift 2
            ;;
        --all)
            USE_ALL=true
            shift
            ;;
        *)
            echo "Unknown flag: $1" >&2
            exit 1
            ;;
    esac
done

# --all always wins, regardless of argument order
if $USE_ALL; then
    SELECTED_AGENTS=()
fi

# Empty = all agents (backward-compatible default)
if [[ ${#SELECTED_AGENTS[@]} -eq 0 ]]; then
    SELECTED_AGENTS=("${ALL_AGENTS[@]}")
fi

# Deduplicate (preserving first-seen order)
declare -A _seen
DEDUPED=()
for agent in "${SELECTED_AGENTS[@]}"; do
    if [[ -z "${_seen[$agent]:-}" ]]; then
        _seen[$agent]=1
        DEDUPED+=("$agent")
    fi
done
SELECTED_AGENTS=("${DEDUPED[@]}")

# Validate
for agent in "${SELECTED_AGENTS[@]}"; do
    if [[ -z "${AGENT_MACHINE[$agent]:-}" ]]; then
        echo "Error: unknown agent '$agent'. Valid agents: ${ALL_AGENTS[*]}" >&2
        exit 1
    fi
done
if [[ ${#SELECTED_AGENTS[@]} -eq 0 ]]; then
    echo "Error: no agents specified" >&2
    exit 1
fi

echo "Selected agents (${#SELECTED_AGENTS[@]}): ${SELECTED_AGENTS[*]}"
```

- [ ] **Step 3: Replace unconditional SSH pre-flight (lines 83-84 only) with conditional**

Replace ONLY these two lines (line 85 `tmux has-session...kill-session` is preserved as-is):
```bash
echo "Checking titan-pc mounts..."
ssh "$PC" "sudo systemctl start soul-mounts.service 2>/dev/null" || true
```

With:
```bash
# ── Conditional pre-flight (only SSH if remote agents selected) ──────────────
NEEDS_REMOTE=false
for agent in "${SELECTED_AGENTS[@]}"; do
    if [[ "${AGENT_MACHINE[$agent]}" != "local" ]]; then
        NEEDS_REMOTE=true
        break
    fi
done

if $NEEDS_REMOTE; then
    echo "Checking titan-pc mounts..."
    ssh "$PC" "sudo systemctl start soul-mounts.service 2>/dev/null" || true
else
    echo "All agents are local — skipping titan-pc pre-flight."
fi
```

- [ ] **Step 4: Verify flag parsing works**

Run (from a terminal, not inside tmux soul-team):
```bash
# Kill any existing session to prevent conflicts
tmux kill-session -t soul-team 2>/dev/null || true

# Should print error with valid agent names (exits before tmux)
bash ~/.claude/scripts/soul-team.sh --agents fridya 2>&1 | head -5

# Should print "Selected agents (2): friday shuri" then proceed to layout
# (may fail at tmux layout since we haven't changed that yet — that's fine)
tmux kill-session -t soul-team 2>/dev/null || true
bash ~/.claude/scripts/soul-team.sh --agents friday,shuri 2>&1 | head -10
```

Expected: First command shows `Error: unknown agent 'fridya'. Valid agents: friday xavier hawkeye pepper fury loki shuri stark banner`. Second shows the selected agents line.

- [ ] **Step 5: Commit**

Note: `~/.claude/` may not be a git repo. If not, skip the commit — the file is outside soul-v2. The changes will be tracked by the soul-team infrastructure, not git.

```bash
# If ~/.claude is a git repo:
cd ~/.claude && git add scripts/soul-team.sh && git commit -m "feat(soul-team): add --agents/--all flag parsing with validation"
# If not, just note the change is saved to disk
```

---

### Task 2: Dynamic Layout Creation

**Files:**
- Modify: `~/.claude/scripts/soul-team.sh:87-118` (hardcoded layout creation)
- Modify: `~/.claude/scripts/soul-team.sh:134-157` (hardcoded panes.json)

Replace the hardcoded 4-column grid + hardcoded panes.json with a `create_dynamic_layout` function and scoped panes.json. The function populates a global `PANE_MAP` associative array and `CEO` variable.

- [ ] **Step 1: Add the `create_dynamic_layout` function**

Insert this function definition BEFORE the pre-flight section (before `# ── Conditional pre-flight`), after the flag parsing block:

```bash
# ── Dynamic layout function ──────────────────────────────────────────────────
# PANE_MAP and CEO are declared at top level so they're accessible after the function returns.
# The function writes to them directly — do NOT re-declare inside the function.
declare -A PANE_MAP
CEO=""

create_dynamic_layout() {
    local num=${#SELECTED_AGENTS[@]}
    local cols

    if   (( num <= 3 )); then cols=1
    elif (( num <= 6 )); then cols=2
    else                       cols=3
    fi

    # Create session — initial pane becomes CEO
    tmux new-session -d -s "$SESSION" -n "soul"
    CEO=$(tmux display-message -t "$SESSION" -p '#{pane_id}')

    # Horizontal splits: CEO keeps 25%, agents get 75%
    # Split percentages for N agent columns (applied to remaining space):
    #   1 col: split at 75%
    #   2 cols: split at 75%, then split right pane at 50%
    #   3 cols: split at 75%, then right at 66%, then rightmost at 50%
    local -a H_SPLITS_1=(75)
    local -a H_SPLITS_2=(75 50)
    local -a H_SPLITS_3=(75 66 50)

    local -a col_panes=()
    local splits_ref="H_SPLITS_${cols}[@]"
    local -a splits=("${!splits_ref}")

    local last_pane="$CEO"
    for (( c=0; c<cols; c++ )); do
        tmux split-window -h -t "$last_pane" -l "${splits[$c]}%"
        col_panes[$c]=$(tmux display-message -t "$SESSION" -p '#{pane_id}')
        last_pane="${col_panes[$c]}"
    done

    # Round-robin assign agents to columns
    local -a col_agents=()
    for (( c=0; c<cols; c++ )); do col_agents[$c]=""; done
    for (( i=0; i<num; i++ )); do
        local c_idx=$(( i % cols ))
        col_agents[$c_idx]+="${SELECTED_AGENTS[$i]} "
    done

    # Vertical splits within each column
    for (( c=0; c<cols; c++ )); do
        local -a agents_in_col=(${col_agents[$c]})
        local n_rows=${#agents_in_col[@]}
        local -a row_panes=("${col_panes[$c]}")

        if (( n_rows == 2 )); then
            tmux split-window -v -t "${col_panes[$c]}" -l '50%'
            row_panes+=($(tmux display-message -t "$SESSION" -p '#{pane_id}'))
        elif (( n_rows == 3 )); then
            local mid
            mid=$(tmux split-window -v -t "${col_panes[$c]}" -l '66%' -P -F '#{pane_id}')
            tmux split-window -v -t "$mid" -l '50%'
            local bot
            bot=$(tmux display-message -t "$SESSION" -p '#{pane_id}')
            row_panes+=("$mid" "$bot")
        fi

        for (( r=0; r<n_rows; r++ )); do
            PANE_MAP["${agents_in_col[$r]}"]="${row_panes[$r]}"
            # Capitalize agent name for pane title; append * for remote agents
            local title="${agents_in_col[$r]^}"
            if [[ "${AGENT_MACHINE[${agents_in_col[$r]}]}" != "local" ]]; then
                title+="*"
            fi
            tmux select-pane -t "${row_panes[$r]}" -T "$title"
        done
    done

    tmux select-pane -t "$CEO" -T "CEO"
}
```

- [ ] **Step 2: Replace hardcoded layout creation (lines 87-118) with function call**

Replace the entire section from `# ── Create panes` through the `# ── Name panes` block (lines 87-118) with:

```bash
# ── Create dynamic layout ────────────────────────────────────────────────────
echo "Creating layout (${#SELECTED_AGENTS[@]} agents)..."
create_dynamic_layout
```

- [ ] **Step 3: Replace hardcoded panes.json (lines 134-157) with scoped version**

Replace the entire `python3 - <<PYEOF ... PYEOF` block (lines 136-157, including the comment at line 134-135) with:

```bash
# ── Write scoped panes.json (only selected agents + team-lead) ───────────────
{
    echo "{"
    echo "  \"team-lead\": \"${CEO}\""
    for agent in "${SELECTED_AGENTS[@]}"; do
        echo ", \"${agent}\": \"${PANE_MAP[$agent]}\""
    done
    echo "}"
} | python3 -c "
import json, sys, os
panes = json.load(sys.stdin)
out_path = os.path.expanduser('${PANES_JSON}')
os.makedirs(os.path.dirname(out_path), exist_ok=True)
with open(out_path, 'w') as f:
    json.dump(panes, f, indent=2)
print(f'panes.json written: {out_path} ({len(panes)} entries)')
"
```

This pipes bash-generated JSON (where variables expand in double quotes) into python3 via stdin, avoiding quoting issues with inline python strings.

- [ ] **Step 4: Verify layout works for various agent counts**

Test by temporarily adding `tmux attach-session -t soul-team` at the end and commenting out the agent launch section. Check pane count and titles.

```bash
# Kill any existing session first
tmux kill-session -t soul-team 2>/dev/null || true

# Test 1 agent (should show CEO + 1 pane)
bash ~/.claude/scripts/soul-team.sh --agents friday
# In another terminal:
tmux list-panes -t soul-team -F '#{pane_title}' | sort
# Expected: CEO, Friday

# Test 4 agents (should show CEO + 2 columns x 2 rows)
tmux kill-session -t soul-team 2>/dev/null || true
bash ~/.claude/scripts/soul-team.sh --agents friday,xavier,pepper,fury
tmux list-panes -t soul-team | wc -l
# Expected: 5 (CEO + 4 agents)
```

- [ ] **Step 5: Commit**

```bash
cd ~/.claude && git add scripts/soul-team.sh && git commit -m "feat(soul-team): dynamic tmux layout based on selected agents" || echo "Not a git repo — change saved to disk"
```

---

### Task 3: Selective Agent Launch + Scoped Sidecars

**Files:**
- Modify: `~/.claude/scripts/soul-team.sh:159-167` (CEO launch + status message)
- Modify: `~/.claude/scripts/soul-team.sh:204-277` (hardcoded launch loop + daemons)

Replace the hardcoded agent launch sequence and sidecar loop with dynamic versions driven by `SELECTED_AGENTS` and `PANE_MAP`.

**Important context:** The agent launch loop and sidecar loop both live inside a background subshell `( ... ) &` (lines 205-277). The outer `(` at line 205 and `) &` at line 277 are NOT modified — only the content inside is replaced. The subshell inherits all parent variables (`SELECTED_AGENTS`, `PANE_MAP`, `AGENT_MACHINE`, `PANES_JSON`). The UI config block (lines 120-132) is also preserved unchanged.

- [ ] **Step 1: Update CEO launch status message (line 165-167)**

Replace:
```bash
echo ""
echo "  CEO ready. Agents launching with ${DELAY}s stagger..."
echo "  Watch them come alive one by one in the right panes."
echo ""
```

With:
```bash
echo ""
echo "  CEO ready. ${#SELECTED_AGENTS[@]} agents launching with ${DELAY}s stagger..."
echo "  Selected: ${SELECTED_AGENTS[*]}"
echo ""
```

- [ ] **Step 2: Replace hardcoded agent launch loop (lines 205-227) with selective loop**

Replace the entire section from `# Local agents (titan-pi)` through `launch_remote_agent "$C3R3" "banner"` with:

```bash
    # ── Launch selected agents with stagger ──────────────────────────────────
    sleep 3
    for agent in "${SELECTED_AGENTS[@]}"; do
        local_pane="${PANE_MAP[$agent]}"
        if [[ "${AGENT_MACHINE[$agent]}" == "local" ]]; then
            launch_local_agent "$local_pane" "$agent"
        else
            launch_remote_agent "$local_pane" "$agent"
        fi
        sleep $DELAY
    done
```

- [ ] **Step 3: Replace hardcoded sidecar loop (lines 244-256) with scoped version**

Replace the entire sidecar section from `# ── Start ALL 10 sidecars` through `echo "  All sidecars launched."` with:

```bash
    # ── Start sidecars for selected agents + team-lead ───────────────────────
    echo "  Starting soul sidecars..."
    for agent in team-lead "${SELECTED_AGENTS[@]}"; do
        pane_id=$(python3 -c "import json; d=json.load(open(\"${PANES_JSON}\")); print(d.get(\"${agent}\",\"\"))" 2>/dev/null)
        if [ -n "$pane_id" ]; then
            nohup bash "$HOME/.claude/scripts/soul-sidecar-wrapper.sh" "$agent" "$pane_id" \
                >> "$HOME/.claude/logs/sidecar-${agent}.log" 2>&1 &
            echo "  Sidecar started: $agent (pane=$pane_id PID=$!)"
        else
            echo "  Warning: no pane_id found for $agent — skipping sidecar"
        fi
    done
    echo "  All sidecars launched."
```

- [ ] **Step 4: Update "All agents launched" message**

Replace:
```bash
    echo ""
    echo "  All agents launched."
```

With:
```bash
    echo ""
    echo "  ${#SELECTED_AGENTS[@]} agents launched."
```

- [ ] **Step 5: Commit**

```bash
cd ~/.claude && git add scripts/soul-team.sh && git commit -m "feat(soul-team): selective agent launch loop and scoped sidecars" || echo "Not a git repo — change saved to disk"
```

---

### Task 4: Update Header Comments + Full Verification

**Files:**
- Modify: `~/.claude/scripts/soul-team.sh:1-24` (header comments)

- [ ] **Step 1: Update script header to document new flags**

Replace lines 1-24 with:

```bash
#!/bin/bash
# soul-team — Launch the Marvel agent team (v2)
#
# Usage:
#   soul-team                           # Launch all 9 agents (default)
#   soul-team --agents friday,shuri     # Launch only Friday and Shuri
#   soul-team --agents friday           # Launch only Friday
#   soul-team --all                     # Explicit all (same as no flags)
#
# Layout adapts dynamically to agent count:
#   1-3 agents: CEO + 1 column
#   4-6 agents: CEO + 2 columns
#   7-9 agents: CEO + 3 columns (original layout)
#
# Agents launch in INTERACTIVE mode (TUI) with staggered startup.
# Trust prompt auto-accepted. Boot prompt typed after TUI loads.
#
# Features:
#   - Reads ~/.claude/config/soul-team.toml for agent config
#   - Dynamic tmux layout based on selected agent count
#   - Conditional titan-pc SSH (skipped when only local agents)
#   - Scoped panes.json + sidecars (only selected agents)
#   - Global daemons always start (router, bridge, heartbeat, guardian)
```

- [ ] **Step 2: Manual verification — all 9 agents (regression)**

```bash
tmux kill-session -t soul-team 2>/dev/null || true
soul-team
# Verify: 10 panes (CEO + 9), all agents launch, panes.json has 10 entries
cat ~/.clawteam/teams/soul-team/panes.json | python3 -m json.tool | wc -l
# Expected: 12 lines (10 entries + braces)
```

- [ ] **Step 3: Manual verification — 1 agent**

```bash
tmux kill-session -t soul-team 2>/dev/null || true
soul-team --agents friday
# Verify: 2 panes (CEO + Friday), only Friday launches
tmux list-panes -t soul-team -F '#{pane_title}'
# Expected: CEO, Friday
cat ~/.clawteam/teams/soul-team/panes.json
# Expected: {"team-lead": "...", "friday": "..."}
```

- [ ] **Step 4: Manual verification — 4 agents (2 columns)**

```bash
tmux kill-session -t soul-team 2>/dev/null || true
soul-team --agents friday,shuri,fury,stark
# Verify: 5 panes, 2 agent columns, remote agents go to titan-pc
tmux list-panes -t soul-team | wc -l
# Expected: 5
```

- [ ] **Step 5: Manual verification — 3 agents (1 column)**

```bash
tmux kill-session -t soul-team 2>/dev/null || true
soul-team --agents friday,shuri,fury
tmux list-panes -t soul-team | wc -l
# Expected: 4 (CEO + 3 agents in 1 column)
tmux list-panes -t soul-team -F '#{pane_title}'
# Expected: CEO, Friday, Shuri*, Fury
```

- [ ] **Step 6: Manual verification — 7 agents (3 columns, 3+2+2)**

```bash
tmux kill-session -t soul-team 2>/dev/null || true
soul-team --agents friday,xavier,hawkeye,pepper,fury,loki,shuri
tmux list-panes -t soul-team | wc -l
# Expected: 8 (CEO + 7 agents)
# Round-robin: col1=[friday,pepper,shuri], col2=[xavier,fury], col3=[hawkeye,loki]
```

- [ ] **Step 7: Manual verification — panes.json scoping**

```bash
tmux kill-session -t soul-team 2>/dev/null || true
soul-team --agents friday,shuri
# After launch, verify panes.json has exactly 3 entries (team-lead + friday + shuri)
python3 -c "import json; d=json.load(open('$HOME/.clawteam/teams/soul-team/panes.json')); print(f'{len(d)} entries: {list(d.keys())}')"
# Expected: 3 entries: ['team-lead', 'friday', 'shuri']
```

- [ ] **Step 8: Manual verification — sidecar scoping**

```bash
# After launching with --agents friday,shuri, check running sidecars
ps aux | grep soul-sidecar | grep -v grep
# Expected: only team-lead, friday, shuri sidecars running (not xavier, hawkeye, etc.)
```

- [ ] **Step 9: Manual verification — error cases**

```bash
# Unknown agent
soul-team --agents fridya 2>&1
# Expected: Error: unknown agent 'fridya'. Valid agents: ...

# Missing value
soul-team --agents 2>&1
# Expected: Error: --agents requires a comma-separated list...

# --all with --agents (--all wins)
soul-team --agents friday --all 2>&1 | head -3
# Expected: Selected agents (9): friday xavier hawkeye ...
```

- [ ] **Step 10: Commit header update**

```bash
cd ~/.claude && git add scripts/soul-team.sh && git commit -m "docs(soul-team): update header with --agents/--all usage docs" || echo "Not a git repo — change saved to disk"
```

---

## Dependency Graph

```
Task 1 (flags + validation)
  └── Task 2 (dynamic layout) — needs SELECTED_AGENTS, AGENT_MACHINE
        └── Task 3 (selective launch + sidecars) — needs PANE_MAP, SELECTED_AGENTS
              └── Task 4 (header + verification) — needs all above working
```

All tasks are strictly sequential — each depends on the previous.
