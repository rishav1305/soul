#!/usr/bin/env bash
set -euo pipefail

# Validate all YAML specs have required fields.
# - Module specs: must have module, purpose, pillars (with 5 keys)
# - pillars.yaml: no module/purpose, but must have pillars with 5 keys

SPEC_DIR="specs"
PILLAR_KEYS='["performant", "robust", "resilient", "secure", "sovereign"]'
FAIL=0
CHECKED=0

if [ ! -d "$SPEC_DIR" ]; then
  echo "FAIL: specs/ directory not found"
  exit 1
fi

shopt -s nullglob
files=("$SPEC_DIR"/*.yaml)
shopt -u nullglob

if [ ${#files[@]} -eq 0 ]; then
  echo "FAIL: no .yaml files found in specs/"
  exit 1
fi

for f in "${files[@]}"; do
  base=$(basename "$f")
  CHECKED=$((CHECKED + 1))

  if [ "$base" = "pillars.yaml" ]; then
    # pillars.yaml: only check that pillars section has all 5 keys
    result=$(python3 -c "
import yaml, sys, json

with open('$f') as fh:
    doc = yaml.safe_load(fh)

expected = set(json.loads('$PILLAR_KEYS'))
errors = []

if 'pillars' not in doc:
    errors.append('missing pillars section')
else:
    found = set(doc['pillars'].keys())
    missing = expected - found
    if missing:
        errors.append(f'missing pillar keys: {sorted(missing)}')

if errors:
    print('FAIL:' + '; '.join(errors))
else:
    print('PASS')
" 2>&1)
  else
    # Module spec: check module, purpose, and pillars with 5 keys
    result=$(python3 -c "
import yaml, sys, json

with open('$f') as fh:
    doc = yaml.safe_load(fh)

expected = set(json.loads('$PILLAR_KEYS'))
errors = []

if 'module' not in doc:
    errors.append('missing module field')
if 'purpose' not in doc:
    errors.append('missing purpose field')
if 'pillars' not in doc:
    errors.append('missing pillars section')
else:
    found = set(doc['pillars'].keys())
    missing = expected - found
    if missing:
        errors.append(f'missing pillar keys: {sorted(missing)}')

if errors:
    print('FAIL:' + '; '.join(errors))
else:
    print('PASS')
" 2>&1)
  fi

  if [[ "$result" == PASS ]]; then
    echo "  PASS  $base"
  else
    echo "  FAIL  $base — ${result#FAIL:}"
    FAIL=1
  fi
done

echo ""
echo "Checked $CHECKED spec files."
if [ "$FAIL" -ne 0 ]; then
  echo "FAIL: one or more specs have issues"
  exit 1
else
  echo "PASS: all specs valid"
fi
