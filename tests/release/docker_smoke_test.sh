#!/bin/sh
set -eu

root=$(CDPATH= cd -- "$(dirname "$0")/../.." && pwd)
version=$(cd "$root" && git describe --tags --always --dirty 2>/dev/null || printf dev)
image=${1:-"expensor:$version"}
suffix=$$
container="expensor-release-smoke-$suffix"
volume="expensor-release-smoke-$suffix"
port=18082
key=AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=
trap 'docker rm -f "$container" >/dev/null 2>&1 || true; docker volume rm "$volume" >/dev/null 2>&1 || true' EXIT HUP INT TERM

start() {
  docker run --rm -d \
    --name "$container" \
    -p "127.0.0.1:$port:8080" \
    -e EXPENSOR_SECRET_KEY="$key" \
    -e EXPENSOR_DB_BACKEND=sqlite \
    -e EXPENSOR_SQLITE_PATH=/app/data/expensor/expensor.db \
    -v "$volume:/app/data" \
    "$image" >/dev/null

  ready=0
  for _ in $(seq 1 30); do
    if curl -fsS "http://127.0.0.1:$port/api/health" >/dev/null; then
      ready=1
      break
    fi
    sleep 1
  done
  test "$ready" -eq 1
}

start
root_html=$(curl -fsS "http://127.0.0.1:$port/")
nested_html=$(curl -fsS "http://127.0.0.1:$port/transactions/example")
case "$root_html" in *'<div id="root"></div>'*) ;; *) exit 1 ;; esac
case "$nested_html" in *'<div id="root"></div>'*) ;; *) exit 1 ;; esac
test "$(curl -sS -o /dev/null -w '%{http_code}' "http://127.0.0.1:$port/assets/missing.js")" = 404
docker exec "$container" test -s /app/data/expensor/expensor.db
docker stop "$container" >/dev/null

start
docker exec "$container" test -s /app/data/expensor/expensor.db
