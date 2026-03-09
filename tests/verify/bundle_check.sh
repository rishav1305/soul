#!/usr/bin/env bash
set -euo pipefail

# Check that the gzipped JS bundle size is under 300KB (307200 bytes).

DIST_DIR="web/dist"
ASSETS_DIR="$DIST_DIR/assets"
MAX_BYTES=307200  # 300KB

# Build if dist doesn't exist
if [ ! -d "$DIST_DIR" ]; then
  echo "web/dist not found — building..."
  (cd web && npx vite build)
fi

if [ ! -d "$ASSETS_DIR" ]; then
  echo "FAIL: $ASSETS_DIR not found after build"
  exit 1
fi

# Find all .js files in assets
shopt -s nullglob
js_files=("$ASSETS_DIR"/*.js)
shopt -u nullglob

if [ ${#js_files[@]} -eq 0 ]; then
  echo "FAIL: no .js files found in $ASSETS_DIR"
  exit 1
fi

total=0
for f in "${js_files[@]}"; do
  # gzip the file to a temp location and measure
  gz_size=$(gzip -c "$f" | wc -c)
  base=$(basename "$f")
  echo "  $base: ${gz_size} bytes gzipped"
  total=$((total + gz_size))
done

total_kb=$((total / 1024))
max_kb=$((MAX_BYTES / 1024))

echo ""
echo "Total gzipped JS: ${total} bytes (${total_kb}KB)"

if [ "$total" -gt "$MAX_BYTES" ]; then
  echo "FAIL: exceeds ${max_kb}KB limit"
  exit 1
else
  echo "PASS: under ${max_kb}KB limit"
fi
