#!/bin/sh
set -eu

root=$(CDPATH= cd -- "$(dirname "$0")/../.." && pwd)
key=AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=

default_services=$(EXPENSOR_SECRET_KEY=$key docker compose -f "$root/deploy/docker-compose.yml" config --services)
test "$default_services" = "expensor"

postgres_services=$(EXPENSOR_SECRET_KEY=$key docker compose -f "$root/deploy/docker-compose.postgres.yml" config --services | LC_ALL=C sort)
test "$postgres_services" = "expensor
postgres"

default_config=$(EXPENSOR_SECRET_KEY=$key docker compose -f "$root/deploy/docker-compose.yml" config)
printf '%s\n' "$default_config" | grep -q 'EXPENSOR_DB_BACKEND: sqlite'
printf '%s\n' "$default_config" | grep -q 'EXPENSOR_SQLITE_PATH: /app/data/expensor/expensor.db'
printf '%s\n' "$default_config" | grep -q 'target: /app/data'

postgres_config=$(EXPENSOR_SECRET_KEY=$key docker compose -f "$root/deploy/docker-compose.postgres.yml" config)
printf '%s\n' "$postgres_config" | grep -q 'EXPENSOR_DB_BACKEND: postgres'
printf '%s\n' "$postgres_config" | grep -q 'condition: service_healthy'
