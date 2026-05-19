#!/usr/bin/env sh
set -eu

if [ "$#" -ne 2 ]; then
  echo "usage: $0 <coverage.out> <min-percent>" >&2
  exit 2
fi

coverage_file=$1
min_percent=$2

if [ ! -f "$coverage_file" ]; then
  echo "backend coverage file not found: $coverage_file" >&2
  exit 1
fi

actual_percent=$(
  go tool cover -func="$coverage_file" |
    awk '/^total:/ {gsub("%","",$3); print $3}'
)

if [ -z "$actual_percent" ]; then
  echo "failed to parse backend coverage from $coverage_file" >&2
  exit 1
fi

printf 'Backend unit coverage: %s%% (minimum %s%%)\n' "$actual_percent" "$min_percent"

awk -v actual="$actual_percent" -v min="$min_percent" 'BEGIN { exit !(actual + 0 >= min + 0) }' || {
  echo "backend unit coverage is below the required floor" >&2
  exit 1
}
