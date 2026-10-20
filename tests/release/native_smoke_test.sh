#!/bin/sh
set -eu

root=$(CDPATH= cd -- "$(dirname "$0")/../.." && pwd)
binary=${1:-"$root/bin/expensor"}
tmp=$(mktemp -d)
port=18083
pid=
trap 'if [ -n "$pid" ]; then kill -TERM "$pid" >/dev/null 2>&1 || true; wait "$pid" 2>/dev/null || true; fi; rm -rf "$tmp"' EXIT HUP INT TERM

HOME="$tmp/home" \
PORT=$port \
EXPENSOR_DB_BACKEND=sqlite \
EXPENSOR_SQLITE_PATH="$tmp/data/expensor.db" \
EXPENSOR_SECRET_KEY=AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA= \
  "$binary" >"$tmp/server.log" 2>&1 &
pid=$!

ready=0
for _ in $(seq 1 30); do
  if curl -fsS "http://127.0.0.1:$port/api/health" >/dev/null; then
    ready=1
    break
  fi
  sleep 1
done
test "$ready" -eq 1
test -s "$tmp/data/expensor.db"

root_html=$(curl -fsS "http://127.0.0.1:$port/")
nested_html=$(curl -fsS "http://127.0.0.1:$port/transactions/example")
case "$root_html" in *'<div id="root"></div>'*) ;; *) exit 1 ;; esac
case "$nested_html" in *'<div id="root"></div>'*) ;; *) exit 1 ;; esac
test "$(curl -sS -o /dev/null -w '%{http_code}' "http://127.0.0.1:$port/assets/missing.js")" = 404

kill -TERM "$pid"
wait "$pid"
pid=
