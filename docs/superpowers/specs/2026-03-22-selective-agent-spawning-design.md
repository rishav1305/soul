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

### Algorithm

```
num_agents = len(selected_agents)
if num_agents <= 3:
    cols = 1
elif num_agents <= 6:
    cols = 2
else:
    cols = 3

# Distribute agents across columns (fill left-to-right, top-to-bottom)
per_col = ceil(num_agents / cols)
for c in range(cols):
    agents_in_col = selected_agents[c*per_col : (c+1)*per_col]
    # Create column, split into len(agents_in_col) rows
```

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

### Flag Parsing

```bash
SELECTED_AGENTS=()   # empty = all

while [[ $# -gt 0 ]]; do
    case "$1" in
        --agents)
            IFS=',' read -ra SELECTED_AGENTS <<< "$2"
            shift 2
            ;;
        --all)
            SELECTED_AGENTS=()  # explicit all
            shift
            ;;
        *)
            echo "Unknown flag: $1" >&2
            exit 1
            ;;
    esac
done
```

When `SELECTED_AGENTS` is empty after parsing, populate it with all agent names from the TOML (preserving order).

### Agent Name Extraction from TOML

```bash
ALL_AGENTS=$(python3 -c "
import re
txt = open('$TOML').read()
print(' '.join(re.findall(r'name\s*=\s*\"(\w+)\"', txt)))
")
```

### Validation

```bash
for agent in "${SELECTED_AGENTS[@]}"; do
    if ! echo "$ALL_AGENTS" | grep -qw "$agent"; then
        echo "Error: unknown agent '$agent'. Valid agents: $ALL_AGENTS" >&2
        exit 1
    fi
done
```

### Dynamic Layout Creation

Replace the current hardcoded pane creation with a function:

```bash
create_dynamic_layout() {
    local num=${#SELECTED_AGENTS[@]}
    local cols

    if   (( num <= 3 )); then cols=1
    elif (( num <= 6 )); then cols=2
    else                       cols=3
    fi

    local per_col=$(( (num + cols - 1) / cols ))

    # Create session with CEO pane
    tmux new-session -d -s "$SESSION" -n "soul"

    # Create agent columns by splitting horizontally from the right
    # Calculate widths: CEO gets 25%, agent columns share the rest equally
    local agent_pct=$(( 75 / cols ))
    local col_panes=()

    # First horizontal split creates column 1
    local remain_pct=75
    for (( c=0; c<cols; c++ )); do
        if (( c == 0 )); then
            tmux split-window -h -t "$SESSION" -l "${remain_pct}%"
            col_panes[$c]=$(tmux display-message -t "$SESSION" -p '#{pane_id}')
        else
            local split_pct=$(( 100 * (cols - c) / (cols - c + 1)  ))
            # ... split from previous column
        fi
        remain_pct=$(( remain_pct - agent_pct ))
    done

    # Split each column vertically into rows
    # Assign PANE_MAP[agent_name] = pane_id for later use
}
```

The exact split math will be implemented precisely — the spec captures the intent: dynamic columns and rows based on agent count.

### Selective Launch Loop

Replace the hardcoded launch sequence with:

```bash
for agent in "${SELECTED_AGENTS[@]}"; do
    local pane="${PANE_MAP[$agent]}"
    local machine=$(get_agent_machine "$agent")  # from TOML

    if [[ "$machine" == "local" ]]; then
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
| `--agents friday,friday` (duplicate) | Deduplicate silently, launch once |
| `--agents` with no value | Error: --agents requires a comma-separated list |
| `--agents` + `--all` together | `--all` wins (launch all) |
| Only remote agents selected | titan-pc mounts still start, no local agent launches |
| Only local agents selected | SSH pre-flight skipped for titan-pc |

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

## Non-Goals

- No changes to agent personas, skills, or communication protocols
- No `--exclude` flag (can be added later)
- No config-based presets/profiles (can be added later)
- No changes to soul-shutdown, soul-monitor, or soul-msg
