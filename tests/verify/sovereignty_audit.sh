#!/usr/bin/env bash
set -euo pipefail

# Sovereignty audit: verify no external network requests in frontend code.
# All fetch/WebSocket calls must use relative URLs (/api/*, /ws).
# Only exception: SVG/XML namespace URIs (xmlns) which are not network requests.

REPO_ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
WEB_SRC="$REPO_ROOT/web/src"
FOUND=0

echo "=== Sovereignty Audit ==="
echo "Scanning: $WEB_SRC"
echo ""

# 1. Check fetch() calls for external URLs
echo "--- Checking fetch() calls ---"
FETCH_EXTERNAL=$(grep -rn --include='*.ts' --include='*.tsx' \
  -E "fetch\s*\(\s*['\"\`]https?://" "$WEB_SRC" 2>/dev/null || true)
if [ -n "$FETCH_EXTERNAL" ]; then
  echo "FAIL: External fetch() calls found:"
  echo "$FETCH_EXTERNAL"
  FOUND=1
else
  echo "PASS: All fetch() calls use relative URLs"
fi

# 2. Check XMLHttpRequest for external URLs
echo ""
echo "--- Checking XMLHttpRequest ---"
XHR_EXTERNAL=$(grep -rn --include='*.ts' --include='*.tsx' \
  -E "XMLHttpRequest" "$WEB_SRC" 2>/dev/null || true)
if [ -n "$XHR_EXTERNAL" ]; then
  echo "FAIL: XMLHttpRequest usage found (use fetch with relative URLs instead):"
  echo "$XHR_EXTERNAL"
  FOUND=1
else
  echo "PASS: No XMLHttpRequest usage"
fi

# 3. Check WebSocket for external URLs
echo ""
echo "--- Checking WebSocket connections ---"
WS_EXTERNAL=$(grep -rn --include='*.ts' --include='*.tsx' \
  -E "new\s+WebSocket\s*\(\s*['\"\`](wss?://|https?://)" "$WEB_SRC" 2>/dev/null || true)
if [ -n "$WS_EXTERNAL" ]; then
  echo "FAIL: External WebSocket connections found:"
  echo "$WS_EXTERNAL"
  FOUND=1
else
  echo "PASS: No external WebSocket connections"
fi

# 4. Check for any hardcoded external URLs (excluding xmlns, comments)
echo ""
echo "--- Checking for external URL literals ---"
# Look for http(s):// URLs that are NOT xmlns or svg namespace declarations
EXTERNAL_URLS=$(grep -rn --include='*.ts' --include='*.tsx' \
  -E "https?://" "$WEB_SRC" 2>/dev/null \
  | grep -v 'xmlns=' \
  | grep -v 'http://www.w3.org/' \
  | grep -v '^\s*//' \
  | grep -v '^\s*\*' \
  || true)
if [ -n "$EXTERNAL_URLS" ]; then
  echo "WARN: External URL literals found (review manually):"
  echo "$EXTERNAL_URLS"
  # This is a warning, not a failure — some URLs may be in comments or docs
else
  echo "PASS: No external URL literals"
fi

# 5. Check service worker exists
echo ""
echo "--- Checking service worker ---"
SW_PATH="$REPO_ROOT/web/public/sw.js"
if [ -f "$SW_PATH" ]; then
  echo "PASS: Service worker exists at web/public/sw.js"
else
  echo "FAIL: Service worker missing at web/public/sw.js"
  FOUND=1
fi

# 6. Check service worker registration in main.tsx
echo ""
echo "--- Checking service worker registration ---"
SW_REG=$(grep -c "serviceWorker.register" "$WEB_SRC/main.tsx" 2>/dev/null || echo "0")
if [ "$SW_REG" -gt 0 ]; then
  echo "PASS: Service worker registration found in main.tsx"
else
  echo "FAIL: No service worker registration in main.tsx"
  FOUND=1
fi

echo ""
if [ "$FOUND" -ne 0 ]; then
  echo "FAIL: sovereignty audit failed — see above"
  exit 1
else
  echo "PASS: sovereignty audit passed — no external requests, SW configured"
fi
