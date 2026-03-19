#!/bin/bash
# Resource Manager — checks system capacity for parallel agents
# Usage: bash tools/resource-check.sh
# Returns: recommended max parallel agents + current system state

set -euo pipefail

echo "=== SYSTEM RESOURCE CHECK ==="
echo ""

# CPU
CORES=$(nproc)
LOAD=$(awk '{print $1}' /proc/loadavg)
LOAD_PCT=$(awk "BEGIN {printf \"%.0f\", ($LOAD / $CORES) * 100}")
echo "CPU: ${CORES} cores | Load: ${LOAD} (${LOAD_PCT}% utilized)"

# Memory
TOTAL_MB=$(free -m | awk '/Mem:/ {print $2}')
AVAIL_MB=$(free -m | awk '/Mem:/ {print $7}')
USED_PCT=$(awk "BEGIN {printf \"%.0f\", (1 - $AVAIL_MB / $TOTAL_MB) * 100}")
echo "RAM: ${AVAIL_MB}MB available / ${TOTAL_MB}MB total (${USED_PCT}% used)"

# Disk
DISK_AVAIL=$(df -BM /home/rishav/soul-v2 | awk 'NR==2 {gsub("M","",$4); print $4}')
echo "Disk: ${DISK_AVAIL}MB available"

# Existing worktrees
WORKTREE_COUNT=$(git -C /home/rishav/soul-v2 worktree list 2>/dev/null | wc -l)
echo "Git worktrees: ${WORKTREE_COUNT} active"

# Repo size (for estimating worktree disk cost)
REPO_SIZE_MB=$(du -sm /home/rishav/soul-v2 --exclude=.git --exclude=node_modules 2>/dev/null | awk '{print $1}')
echo "Repo size: ~${REPO_SIZE_MB}MB per worktree"

# Estimate capacity
# Each agent needs:
#   ~200MB disk (worktree)
#   ~1.5GB RAM peak (go build + go test -race)
#   ~0.5 CPU core sustained

echo ""
echo "=== CAPACITY ESTIMATE ==="

# RAM-based limit (each agent peaks at ~1.5GB during go build/test)
RAM_LIMIT=$(awk "BEGIN {printf \"%.0f\", ($AVAIL_MB - 2000) / 1500}")
if [ "$RAM_LIMIT" -lt 1 ]; then RAM_LIMIT=1; fi

# CPU-based limit (each agent uses ~0.5 core sustained, leave 1 core for OS + main process)
CPU_LIMIT=$(awk "BEGIN {printf \"%.0f\", ($CORES - 1) / 0.5}")
if [ "$CPU_LIMIT" -lt 1 ]; then CPU_LIMIT=1; fi

# Disk-based limit (each worktree ~200MB, need 1GB buffer)
DISK_LIMIT=$(awk "BEGIN {printf \"%.0f\", ($DISK_AVAIL - 1000) / $REPO_SIZE_MB}")
if [ "$DISK_LIMIT" -lt 1 ]; then DISK_LIMIT=1; fi

# Load-based adjustment
if [ "$LOAD_PCT" -gt 80 ]; then
    LOAD_PENALTY=1
elif [ "$LOAD_PCT" -gt 50 ]; then
    LOAD_PENALTY=0
else
    LOAD_PENALTY=0
fi

# Take minimum of all limits
RECOMMENDED=$RAM_LIMIT
if [ "$CPU_LIMIT" -lt "$RECOMMENDED" ]; then RECOMMENDED=$CPU_LIMIT; fi
if [ "$DISK_LIMIT" -lt "$RECOMMENDED" ]; then RECOMMENDED=$DISK_LIMIT; fi
RECOMMENDED=$((RECOMMENDED - LOAD_PENALTY))
if [ "$RECOMMENDED" -lt 1 ]; then RECOMMENDED=1; fi
if [ "$RECOMMENDED" -gt 8 ]; then RECOMMENDED=8; fi

echo "RAM limit:  ${RAM_LIMIT} agents (${AVAIL_MB}MB avail, ~1.5GB each)"
echo "CPU limit:  ${CPU_LIMIT} agents (${CORES} cores, ~0.5 core each)"
echo "Disk limit: ${DISK_LIMIT} agents (${DISK_AVAIL}MB avail, ~${REPO_SIZE_MB}MB each)"
echo "Load adj:   -${LOAD_PENALTY} (current load ${LOAD_PCT}%)"
echo ""
echo "================================"
echo "RECOMMENDED MAX PARALLEL: ${RECOMMENDED}"
echo "================================"
echo ""

# Bottleneck identification
if [ "$RAM_LIMIT" -le "$CPU_LIMIT" ] && [ "$RAM_LIMIT" -le "$DISK_LIMIT" ]; then
    echo "BOTTLENECK: RAM — close other apps to increase capacity"
elif [ "$CPU_LIMIT" -le "$RAM_LIMIT" ] && [ "$CPU_LIMIT" -le "$DISK_LIMIT" ]; then
    echo "BOTTLENECK: CPU — wait for load to decrease"
else
    echo "BOTTLENECK: Disk — clean up worktrees or free disk space"
fi
