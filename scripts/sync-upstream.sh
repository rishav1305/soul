#!/bin/bash
set -e

echo "Fetching upstream claude-code..."
git subtree pull --prefix=upstream \
  https://github.com/anthropics/claude-code.git main \
  --squash -m "chore: sync upstream claude-code $(date +%Y-%m-%d)"

echo "Checking npm for new claude-code version..."
LATEST=$(npm show @anthropic-ai/claude-code version 2>/dev/null)
echo "Latest npm version: $LATEST"
echo "Done."
