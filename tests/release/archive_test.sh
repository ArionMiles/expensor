#!/bin/sh
set -eu

root=$(CDPATH= cd -- "$(dirname "$0")/../.." && pwd)
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT HUP INT TERM

mkdir -p "$tmp/input" "$tmp/first" "$tmp/second"
for target in linux_amd64 linux_arm64 darwin_amd64 darwin_arm64; do
  printf 'binary-%s\n' "$target" >"$tmp/input/expensor_$target"
  chmod 0755 "$tmp/input/expensor_$target"
done

package() {
  output=$1
  go run "$root/scripts/release/archive.go" \
    -input "$tmp/input" \
    -output "$output" \
    -version 1.2.3 \
    -config "$root/deploy/config.toml.example" \
    -license "$root/LICENSE" \
    -notice "$root/NOTICE"
}

package "$tmp/first"
package "$tmp/second"

for target in linux_amd64 linux_arm64 darwin_amd64 darwin_arm64; do
  archive="expensor_1.2.3_${target}.tar.gz"
  test -f "$tmp/first/$archive"
  cmp "$tmp/first/$archive" "$tmp/second/$archive"

  tar -tzf "$tmp/first/$archive" | LC_ALL=C sort >"$tmp/archive-files"
  printf '%s\n' LICENSE NOTICE config.toml.example expensor >"$tmp/expected-files"
  diff -u "$tmp/expected-files" "$tmp/archive-files"

  mkdir "$tmp/extract"
  tar -xzf "$tmp/first/$archive" -C "$tmp/extract"
  cmp "$tmp/input/expensor_$target" "$tmp/extract/expensor"
  test -x "$tmp/extract/expensor"
  rm -rf "$tmp/extract"
done

manifest="$tmp/first/expensor_1.2.3_checksums.txt"
test -f "$manifest"
test "$(wc -l <"$manifest" | tr -d ' ')" -eq 4
(cd "$tmp/first" && shasum -a 256 -c "$(basename "$manifest")")
cmp "$manifest" "$tmp/second/$(basename "$manifest")"
