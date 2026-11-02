#!/usr/bin/env bash
# Storage / migration tests against a disposable PostgreSQL.
#
# Why per-test databases: the existing storage smoke tests share one database and
# assume a clean slate (row counts, member ordering, migration up/down). Running
# them in one database makes later tests see earlier seeds. This script creates a
# fresh database for every Test* function, matching CI-style isolation.
#
# Usage: bash scripts/test-db.sh
# Requires: Docker, Go toolchain. Set PG_IMAGE to override postgres:17-alpine.
set -euo pipefail

root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
cd "$root"

if ! command -v docker >/dev/null 2>&1; then
  echo "test-db: docker is required" >&2
  exit 1
fi
if ! docker info >/dev/null 2>&1; then
  echo "test-db: docker daemon is not running" >&2
  exit 1
fi

container="hdd-test-pg-$$"
db="hdd_test"
image="${PG_IMAGE:-postgres:17-alpine}"

cleanup() {
  docker rm -f "$container" >/dev/null 2>&1 || true
}
trap cleanup EXIT

docker run -d --name "$container" \
  -e POSTGRES_PASSWORD=test \
  -p 127.0.0.1::5432 \
  "$image" >/dev/null

port=$(docker port "$container" 5432/tcp | awk -F: '{print $NF}')
if [ -z "$port" ]; then
  echo "test-db: could not resolve mapped Postgres port" >&2
  exit 1
fi

ready=false
for _ in $(seq 1 30); do
  if docker exec "$container" pg_isready -U postgres >/dev/null 2>&1; then
    ready=true
    break
  fi
  sleep 1
done
if [ "$ready" != true ]; then
  echo "test-db: postgres did not become ready" >&2
  exit 1
fi

tests=$(go test ./internal/storage -list '^Test' | grep '^Test' || true)
if [ -z "$tests" ]; then
  echo "test-db: no storage tests found" >&2
  exit 1
fi

failed=0
for name in $tests; do
  docker exec "$container" psql -U postgres -d postgres -q \
    -c "DROP DATABASE IF EXISTS $db" \
    -c "CREATE DATABASE $db" >/dev/null
  echo "== $name =="
  if ! HDD_TEST_DSN="postgres://postgres:test@127.0.0.1:$port/$db?sslmode=disable" \
    go test ./internal/storage -count=1 -run "^${name}$"; then
    failed=1
  fi
done

exit "$failed"
