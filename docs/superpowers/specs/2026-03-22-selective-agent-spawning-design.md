# Selective Agent Spawning for soul-team

**Date:** 2026-03-22
**Author:** Shuri (Technical PM)
**Status:** Draft

---

## Problem

`soul-team` launches all 9 agents unconditionally. This wastes resources (CPU, RAM, API tokens) when only a subset is needed. There is no way to selectively spawn agents without editing the script.

## Solution

Add `--agents <comma-separated-list>` CLI flag to `soul-team.sh`. When provided, only the named agents spawn. Without flags, all 9 launch (backward-compatible). The tmux layout dynamically adapts to the number of selected agents.

## CLI Interface

```bash
soul-team                           # All 9 agents (backward-compatible)
soul-team --agents friday,shuri     # Only Friday and Shuri
soul-team --agents friday           # Just Friday
soul-team --all                     # Explicit all (same as no flags)
```

### Validation

Unknown agent names produce a clear error:

```
Error: unknown agent 'fridya'. Valid agents: friday, xavier, hawkeye, pepper, fury, loki, shuri, stark, banner
```

The valid agent list is derived from the `[[agents]]` entries in `soul-team.toml` — no separate config needed.

## Dynamic Layout Algorithm

CEO always occupies the left column (~25% width). Selected agents fill a dynamic grid to maximize screen real estate.

### Grid Sizing

| Agents | Columns | Rows/Col | Description |
|--------|---------|----------|-------------|
| 1      | 1       | 1        | CEO + 1 pane |
| 2      | 1       | 2        | CEO + 1 column, 2 rows |
| 3      | 1       | 3        | CEO + 1 column, 3 rows |
| 4      | 2       | 2        | CEO + 2 columns, 2 rows each |
| 5      | 2       | 3+2      | CEO + 2 columns (3 left, 2 right) |
| 6      | 2       | 3        | CEO + 2 columns, 3 rows each |
| 7      | 3       | 3+2+2    | CEO + 3 columns (3, 2, 2) |
| 8      | 3       | 3+3+2    | CEO + 3 columns (3, 3, 2) |
| 9      | 3       | 3        | CEO + 3 columns, 3 rows each (today) |

### Algorithm — Round-Robin Column Fill

Agents are distributed across columns using round-robin assignment (not slicing), which naturally balances columns:

```
num_agents = len(selected_agents)
if num_agents <= 3:
    cols = 1
elif num_agents <= 6:
    cols = 2
else:
    cols = 3

# Round-robin: agent[i] goes to column (i % cols)
columns = [[] for _ in range(cols)]
for i, agent in enumerate(selected_agents):
    columns[i % cols].append(agent)
```

This yields the correct distribution for every count:

| Count | cols | Distribution | Columns |
|-------|------|-------------|---------|
| 5 | 2 | 3+2 | [0,2,4], [1,3] |
| 7 | 3 | 3+2+2 | [0,3,6], [1,4], [2,5] |
| 8 | 3 | 3+3+2 | [0,3,6], [1,4,7], [2,5] |

Agents are assigned to the grid in the order they appear in the `--agents` flag (or in `[[agents]]` order when launching all).

## Daemon Scoping

| Daemon | Behavior |
|--------|----------|
| **Sidecars** | Only start for selected agents + team-lead |
| **panes.json** | Only contains selected agents + team-lead |
| **Router** | Always starts (global infrastructure) |
| **Bridge** | Always starts (global infrastructure) |
| **Heartbeat** | Always starts (global infrastructure) |
| **Guardian** | Always starts (global infrastructure) |

## Implementation Scope

### Files Modified

| File | Change |
|------|--------|
| `~/.claude/scripts/soul-team.sh` | Parse `--agents`/`--all` flags, validate agent names, dynamic layout creation, selective agent launch loop, scoped sidecar startup |

### Files NOT Modified

- Agent personas (`~/.claude/agents/*.md`) — untouched
- Skills (`~/.claude/skills/`) — untouched
- `soul-team.toml` — no schema changes needed (valid agents derived from `[[agents]]`)
- `soul-shutdown.sh` — already works on whatever tmux session exists
- `soul-monitor.sh` — already reads live process state
- `soul-msg` — already targets individual agents by name
- Inbox directories — untouched

## Detailed Behavior

### Agent Name Extraction from TOML

Extract valid agent names from `[[agents]]` blocks only (not `[team]` or `[layout]`):

```bash
ALL_AGENTS_STR=$(python3 -c "
import re
txt = open('$TOML').read()
# Split into [[agents]] blocks, extract name from each
blocks = re.split(r'\[\[agents\]\]', txt)[1:]  # skip preamble
names = []
for b in blocks:
    m = re.search(r'name\s*=\s*\"(\w+)\"', b)
    if m: names.append(m.group(1))
print(' '.join(names))
")
read -ra ALL_AGENTS <<< "$ALL_AGENTS_STR"
```

This also extracts per-agent machine type into an associative array for later use:

```bash
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
")"
```

### Flag Parsing

`--all` is a boolean flag that forces all agents. It takes precedence regardless of argument order. `--agents` is rejected if `--all` is also present.

```bash
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
```

### Validation

```bash
for agent in "${SELECTED_AGENTS[@]}"; do
    if [[ -z "${AGENT_MACHINE[$agent]:-}" ]]; then
        echo "Error: unknown agent '$agent'. Valid agents: ${ALL_AGENTS[*]}" >&2
        exit 1
    fi
done

# Guard: empty after dedup (e.g., --agents "")
if [[ ${#SELECTED_AGENTS[@]} -eq 0 ]]; then
    echo "Error: no agents specified" >&2
    exit 1
fi
```

### Conditional SSH Pre-flight

Only SSH to titan-pc if at least one selected agent has `machine = "titan-pc"`:

```bash
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
fi
```

### Dynamic Layout Creation

Replace the current hardcoded pane creation with a function that uses correct tmux split percentages.

**tmux split semantics:** `-l X%` means "give the NEW pane X% of the CURRENT pane's size." So to split a remaining area into N equal parts, each successive split uses `100 * remaining / (remaining + 0)` — specifically, the last split is always 50%, the one before it is 66%, etc.

```bash
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
    local H_SPLITS_1=(75)
    local H_SPLITS_2=(75 50)
    local H_SPLITS_3=(75 66 50)

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
    local -a col_agents
    for (( c=0; c<cols; c++ )); do col_agents[$c]=""; done
    for (( i=0; i<num; i++ )); do
        local c_idx=$(( i % cols ))
        col_agents[$c_idx]+="${SELECTED_AGENTS[$i]} "
    done

    # Vertical splits within each column
    # Split percentages for N rows (applied to remaining space):
    #   1 row: no split needed
    #   2 rows: split at 50%
    #   3 rows: split at 66%, then bottom at 50%
    declare -A PANE_MAP
    for (( c=0; c<cols; c++ )); do
        local -a agents_in_col=(${col_agents[$c]})
        local n_rows=${#agents_in_col[@]}
        local -a row_panes=("${col_panes[$c]}")  # first row = the column pane itself

        if (( n_rows == 2 )); then
            tmux split-window -v -t "${col_panes[$c]}" -l '50%'
            row_panes+=($(tmux display-message -t "$SESSION" -p '#{pane_id}'))
        elif (( n_rows == 3 )); then
            local mid=$(tmux split-window -v -t "${col_panes[$c]}" -l '66%' -P -F '#{pane_id}')
            tmux split-window -v -t "$mid" -l '50%'
            local bot=$(tmux display-message -t "$SESSION" -p '#{pane_id}')
            row_panes+=("$mid" "$bot")
        fi

        for (( r=0; r<n_rows; r++ )); do
            PANE_MAP["${agents_in_col[$r]}"]="${row_panes[$r]}"
            tmux select-pane -t "${row_panes[$r]}" -T "${agents_in_col[$r]^}"  # capitalize title
        done
    done

    tmux select-pane -t "$CEO" -T "CEO"
}
```

### Selective Launch Loop

Replace the hardcoded launch sequence with:

```bash
for agent in "${SELECTED_AGENTS[@]}"; do
    local pane="${PANE_MAP[$agent]}"

    if [[ "${AGENT_MACHINE[$agent]}" == "local" ]]; then
        launch_local_agent "$pane" "$agent"
    else
        launch_remote_agent "$pane" "$agent"
    fi

    sleep $DELAY
done
```

### Scoped Sidecar Startup

```bash
# Only start sidecars for selected agents + team-lead
for agent in team-lead "${SELECTED_AGENTS[@]}"; do
    pane_id=$(python3 -c "..." 2>/dev/null)
    if [ -n "$pane_id" ]; then
        nohup bash "$HOME/.claude/scripts/soul-sidecar-wrapper.sh" "$agent" "$pane_id" ...
    fi
done
```

### Scoped panes.json

Only write entries for team-lead + selected agents:

```python
panes = {"team-lead": CEO_PANE}
for agent in SELECTED_AGENTS:
    panes[agent] = PANE_MAP[agent]
```

## Edge Cases

| Case | Behavior |
|------|----------|
| `--agents ""` (empty string) | Error: no agents specified |
| `--agents friday,friday` (duplicate) | Deduplicate silently (first-seen order), launch once |
| `--agents` with no value | Error: --agents requires a comma-separated list |
| `--agents X --all` or `--all --agents X` | `--all` wins regardless of order (launch all) |
| Only remote agents selected | titan-pc mounts start, no local cgroup launches |
| Only local agents selected | SSH pre-flight to titan-pc skipped entirely |
| Re-running `soul-team` while session exists | Existing `soul-team` tmux session killed first (pre-existing behavior) |

## Testing

Manual testing only (bash script, not unit-testable in the traditional sense):

1. `soul-team` — verify all 9 launch (regression)
2. `soul-team --agents friday` — verify 1 agent, CEO + 1 pane layout
3. `soul-team --agents friday,shuri,fury` — verify 3 agents, CEO + 1 column
4. `soul-team --agents friday,shuri,fury,stark` — verify 4 agents, CEO + 2 columns
5. `soul-team --agents friday,shuri,fury,stark,banner,loki,xavier` — verify 7, CEO + 3 columns
6. `soul-team --agents fridya` — verify error message
7. `soul-team --all` — verify same as no flags
8. Verify panes.json only contains selected agents
9. Verify sidecars only started for selected agents

## Future: TUI Picker (Backlog)

Interactive checkbox picker using `gum choose --no-limit` or `fzf --multi`, shown when running `soul-team` without flags. Not in this iteration — deferred to backlog.

## Known Limitations (Acceptable)

These are pre-existing behaviors that become more visible under selective spawning but are intentionally left as-is:

- **Inbox/comms initialization is always all-9:** The `mkdir` loops for native inboxes, ClawTeam inboxes, and live comms directories create dirs for all agents regardless of selection. This is harmless (empty dirs are cheap) and ensures agents can receive messages even when not running.
- **`soul-msg broadcast` targets all 9:** Broadcast messages are delivered to all agent inboxes, even if only 2 are running. Non-running agents accumulate unread messages — this is by design (they'll process them on next launch).
- **`soul-monitor` pane count includes CEO:** `tmux list-panes | wc -l` shows total panes (CEO + agents). With 2 agents, it shows 3. Cosmetic only.
- **Sidecar iteration source changes:** The sidecar loop changes from a hardcoded 10-agent list to `team-lead + SELECTED_AGENTS`. This is intentional — sidecars for non-running agents would fail to detect state.

## Non-Goals

- No changes to agent personas, skills, or communication protocols
- No `--exclude` flag (can be added later)
- No config-based presets/profiles (can be added later)
- No changes to soul-shutdown, soul-monitor, or soul-msg
