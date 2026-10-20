#!/bin/sh
set -eu

root=$(CDPATH= cd -- "$(dirname "$0")/../.." && pwd)
version=${1:?usage: package.sh VERSION [OUTPUT_DIR]}
output=${2:-"$root/dist/release"}
stage=$(mktemp -d)
ui="$root/backend/internal/httpapi/dist"
trap 'rm -rf "$stage" "$ui"' EXIT HUP INT TERM

rm -rf "$output" "$ui"
mkdir -p "$output" "$stage/bin"
cp -R "$root/frontend/dist" "$ui"

for target in linux_amd64 linux_arm64 darwin_amd64 darwin_arm64; do
  os=${target%_*}
  arch=${target#*_}
  (
    cd "$root/backend"
    CGO_ENABLED=0 GOOS=$os GOARCH=$arch go build \
      -tags production \
      -trimpath \
      -ldflags="-s -w -X github.com/ArionMiles/expensor/backend/pkg/config.Version=$version" \
      -o "$stage/bin/expensor_$target" \
      ./cmd/server
  )
done

go run "$root/scripts/release/archive.go" \
  -input "$stage/bin" \
  -output "$output" \
  -version "$version" \
  -config "$root/deploy/config.toml.example" \
  -license "$root/LICENSE" \
  -notice "$root/NOTICE"
