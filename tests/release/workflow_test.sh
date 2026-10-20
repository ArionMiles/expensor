#!/bin/sh
set -eu

workflow=.github/workflows/release.yml

checkout_count=$(grep -c 'ref: \${{ github.sha }}' "$workflow" || true)
if [ "$checkout_count" -ne 3 ]; then
  echo "release jobs must check out the triggering commit" >&2
  exit 1
fi

if grep -n '\${{ inputs.version }}' "$workflow" | grep -E 'run:|echo|VERSION=|<<' >/dev/null; then
  echo "manual release input must not be inserted into shell source" >&2
  exit 1
fi

grep -q '^  validate-version:' "$workflow"
grep -Fq '*[!0-9A-Za-z.+-]*)' "$workflow"
grep -q 'target_commitish: \${{ github.sha }}' "$workflow"
