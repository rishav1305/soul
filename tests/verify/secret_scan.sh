#!/usr/bin/env bash
set -euo pipefail

# Scan source files for hardcoded secrets.
# Exits non-zero if any secret pattern is found.

FOUND=0

# Build the grep include/exclude arguments
GREP_ARGS=(
  --include='*.go'
  --include='*.ts'
  --include='*.tsx'
  --include='*.json'
  --exclude='package-lock.json'
  --exclude-dir='node_modules'
  --exclude-dir='.git'
  --exclude-dir='dist'
  -r
  -n
  -E
)

# Patterns and their descriptions
declare -a PATTERNS
declare -a DESCRIPTIONS

PATTERNS+=(    'sk-ant-' )
DESCRIPTIONS+=('Anthropic API key prefix' )

PATTERNS+=(    'AKIA[0-9A-Z]{16}' )
DESCRIPTIONS+=('AWS access key' )

PATTERNS+=(    'AIza[0-9A-Za-z_-]{35}' )
DESCRIPTIONS+=('Google API key' )

PATTERNS+=(    'password\s*=\s*[\"'"'"']' )
DESCRIPTIONS+=('Hardcoded password' )

PATTERNS+=(    'passwd\s*=\s*[\"'"'"']' )
DESCRIPTIONS+=('Hardcoded passwd' )

PATTERNS+=(    'secret\s*=\s*[\"'"'"']' )
DESCRIPTIONS+=('Hardcoded secret' )

PATTERNS+=(    'token\s*=\s*[\"'"'"'][^\$]' )
DESCRIPTIONS+=('Hardcoded token (non-env)' )

PATTERNS+=(    'BEGIN.*PRIVATE KEY' )
DESCRIPTIONS+=('Private key block' )

PATTERNS+=(    '://[^@/\s]+:[^@/\s]+@' )
DESCRIPTIONS+=('Connection string with credentials' )

for i in "${!PATTERNS[@]}"; do
  pattern="${PATTERNS[$i]}"
  desc="${DESCRIPTIONS[$i]}"

  # grep returns 1 if no match — that's OK, we only care about matches
  matches=$(grep "${GREP_ARGS[@]}" "$pattern" . 2>/dev/null || true)
  if [ -n "$matches" ]; then
    echo "FAIL: $desc ($pattern)"
    echo "$matches"
    echo ""
    FOUND=1
  fi
done

if [ "$FOUND" -ne 0 ]; then
  echo "FAIL: secrets detected — see above"
  exit 1
else
  echo "PASS: no secrets detected"
fi
