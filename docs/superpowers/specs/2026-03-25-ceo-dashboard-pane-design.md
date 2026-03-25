# CEO Dashboard Pane — Design Spec

**Date:** 2026-03-25
**Author:** team-lead
**Status:** APPROVED

## Problem

Team-lead conversation in the main tmux pane is a single stream where agent status updates, decisions, analysis, and artifacts all mix together. Important content (decision queues, blocker tables, briefs) scrolls away and is hard to retrieve. The tmux status bar wastes space showing minimal info.

## Solution

A persistent bottom-split tmux pane (20% height) displaying a file-backed dashboard with three columns and a system status row. Team-lead auto-pushes content; CEO can also explicitly pin items.

## Architecture

### File-Backed Dashboard

- **File:** `~/.soul-v2/ceo-dashboard.md`
- **Archive:** `~/.soul-v2/ceo-dashboard-archive.md`
- **Data:** `~/.soul-v2/ceo-dashboard.json` (structured data, script reads this to render)
- **Renderer:** `~/.claude/scripts/ceo-dashboard.py` (reads JSON, outputs formatted table to stdout)
- **Display:** Bottom tmux pane runs `watch -t -n 5 ~/.claude/scripts/ceo-dashboard.py`

### Why JSON + Python Renderer

The dashboard file is not hand-edited. A JSON data file stores items, and a Python script renders them into a perfectly aligned box-drawing table at the current terminal width. This ensures:
- Borders always align regardless of content length
- Columns fill full pane width dynamically
- Adding/removing items is a JSON operation, not text surgery

### Data Schema (ceo-dashboard.json)

```json
{
  "decisions": [
    {"id": 1, "text": "Comp gate for pipeline", "source": "Fury", "est": "2m", "added": "2026-03-25T17:00:00"}
  ],
  "pinned": [
    {"id": 1, "text": "T1 comp: 5/11 pass 50L floor", "status": "active", "symbol": "★", "added": "2026-03-25T17:00:00"}
  ],
  "upcoming": [
    {"id": 1, "text": "LinkedIn #1 (SoulGraph teaser)", "date": "Mar 26 AM", "added": "2026-03-25T17:00:00"}
  ]
}
```

**Status symbols:**
- `★` active/needs attention
- `✓` done/ready
- `○` blocked/on-hold

### Renderer (ceo-dashboard.py)

Python script that:
1. Reads `ceo-dashboard.json`
2. Gets terminal width via `shutil.get_terminal_size()`
3. Calculates column widths: 40% / 35% / 25%
4. Renders box-drawing table with proper ╔╦╗╠╬╣╚╩╝║═ characters
5. Pads every cell to exact width
6. Bottom row spans full width: PI and PC system stats via `/proc` (local) and SSH (remote)
7. Outputs to stdout (consumed by `watch`)

### System Stats Collection

- **PI (local):** Read `/proc/loadavg`, `/proc/meminfo`, `/proc/uptime` directly
- **PC (remote):** SSH one-liner every refresh: `ssh -o ConnectTimeout=2 titan-pc` to get CPU/MEM/uptime
- **Fallback:** If SSH fails, show `PC: UNREACHABLE`

### Layout (full terminal width, 430 cols)

```
╔══ DECISIONS ══════════════╦══ PINNED ═════════════════════╦══ UPCOMING ═══════════╗
║ 1. Comp gate [Fury] 2m   ║ ★ T1 comp: 5/11 pass 50L    ║ Mar26 AM: LinkedIn #1 ║
║ ...                       ║ ...                          ║ ...                   ║
╠══ SYSTEM ═════════════════╩══════════════════════════════╩════════════════════════╣
║ PI: CPU 8% │ MEM 4.1/15G │ SWAP 499M/1G │ ↑1h   PC: CPU 12% │ MEM 6.2/32G │ ↑3d ║
╚══════════════════════════════════════════════════════════════════════════════════════╝
```

## Integration

### soul-team.sh Modifications

On launch (after agent panes are created):
1. Split the current window horizontally: `tmux split-window -v -p 20`
2. In the new bottom pane: `watch -t -n 5 python3 ~/.claude/scripts/ceo-dashboard.py`
3. Select the team-lead pane (pane 0) as active

On `--continue`: Check if dashboard pane exists, recreate if missing.

### tmux Status Bar

Strip to minimal: session name only. All useful info moves to dashboard pane.

### How Items Flow In

- **Auto-push (team-lead):** When presenting decision queues, blocker tables, or key summaries, team-lead updates `ceo-dashboard.json` via a helper script or direct JSON write
- **Explicit pin:** CEO says "pin this", team-lead adds to pinned section
- **Completion:** CEO resolves a decision, team-lead removes it (archived to `ceo-dashboard-archive.md`)
- **Agents DO NOT touch this file** — team-lead is the sole writer

### Helper Script (ceo-dashboard-update.py)

CLI for team-lead to update the dashboard:
```bash
# Add decision
ceo-dash add decision "Comp gate for pipeline" --source Fury --est 2m

# Add pinned item
ceo-dash add pinned "T1 comp: 5/11 pass 50L" --status active

# Add upcoming event
ceo-dash add upcoming "LinkedIn #1" --date "Mar 26 AM"

# Remove item
ceo-dash remove decision 1

# Archive resolved decision
ceo-dash resolve decision 1

# Clear all decisions
ceo-dash clear decisions
```

### Cleanup Rules

- Resolved decisions move to `ceo-dashboard-archive.md` with timestamp
- Past events auto-archive when date passes (checked on each render)
- Pinned artifacts stay until explicitly unpinned
- Archive file is append-only, never displayed

## Persistence

- `ceo-dashboard.json` persists on disk — survives session restarts, reboots
- `soul-team.sh` recreates the display pane on every launch
- Content carries over between sessions automatically

## Files to Create

1. `~/.claude/scripts/ceo-dashboard.py` — Renderer (reads JSON, outputs table)
2. `~/.claude/scripts/ceo-dashboard-update.py` — CLI helper for team-lead to update JSON
3. `~/.soul-v2/ceo-dashboard.json` — Data file
4. `~/.soul-v2/ceo-dashboard-archive.md` — Archive

## Files to Modify

1. `~/.claude/scripts/soul-team.sh` — Add bottom split pane on launch
