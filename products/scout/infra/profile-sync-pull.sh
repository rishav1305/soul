#!/usr/bin/env bash
# Daily pull: Supabase cloud → local profile-db via Soul API
# Triggered by profile-sync-pull.timer (03:00 daily)
set -euo pipefail

LOG="${HOME}/.soul/scout/profile-sync.log"
API="http://127.0.0.1:3000/api/tools/scout__profile_pull/execute"

mkdir -p "$(dirname "$LOG")"

echo "$(date -Is) Starting Supabase -> local profile pull" >> "$LOG"

RESULT=$(curl -sf --max-time 120 "$API" \
  -H 'Content-Type: application/json' \
  -d '{"input":{"tables":[]}}' 2>> "$LOG") || {
  echo "$(date -Is) ERROR: curl failed with exit $?" >> "$LOG"
  exit 1
}

echo "$(date -Is) Result: $RESULT" >> "$LOG"
echo "$(date -Is) Pull complete" >> "$LOG"
