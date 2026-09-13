#!/usr/bin/env bash

set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
go_mod="$repo_root/backend/go.mod"

expected="$(awk '$1 == "go" { print $2; exit }' "$go_mod")"
if [[ ! "$expected" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
  echo "Could not read a full Go version from backend/go.mod" >&2
  exit 1
fi

files=(
  "$repo_root/Dockerfile"
  "$repo_root/tests/component/docker-compose.yml"
  "$repo_root/tests/contract/docker-compose.yml"
)
expected_counts=(1 2 1)

for index in "${!files[@]}"; do
  file="${files[$index]}"
  expected_count="${expected_counts[$index]}"
  count=0
  while IFS= read -r version; do
    ((count += 1))
    if [[ "$version" != "$expected" ]]; then
      relative_path="${file#"$repo_root/"}"
      echo "$relative_path uses Go $version; expected $expected from backend/go.mod" >&2
      exit 1
    fi
  done < <(sed -nE 's/.*golang:([0-9]+\.[0-9]+\.[0-9]+)-alpine.*/\1/p' "$file")

  if [[ "$count" -ne "$expected_count" ]]; then
    relative_path="${file#"$repo_root/"}"
    echo "$relative_path contains $count Go image references; expected $expected_count" >&2
    exit 1
  fi
done

echo "All runtime Go versions match backend/go.mod ($expected)"
