#!/usr/bin/env bash
# Isolated Android acceptance stack.
#
# Starts a disposable Postgres, the worktree recognizer, hdd serve, and Metro,
# then exposes them to a USB device via adb reverse. Uses a unique object-storage
# bucket so the shared deploy data is never touched.
#
# Usage: bash scripts/android-acceptance.sh
# Stop with Ctrl+C; the trap removes services, reverse ports, DB, and bucket.
#
# Env overrides:
#   HDD_PORT=18080 RECOGNIZER_PORT=18000 METRO_PORT=18081
#   S3_ENDPOINT=http://127.0.0.1:8333 ADB=$HOME/Library/Android/sdk/platform-tools/adb
set -euo pipefail

root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
cd "$root"

HDD_PORT=${HDD_PORT:-18080}
RECOGNIZER_PORT=${RECOGNIZER_PORT:-18000}
METRO_PORT=${METRO_PORT:-18081}
S3_ENDPOINT=${S3_ENDPOINT:-http://127.0.0.1:8333}
ADB=${ADB:-$HOME/Library/Android/sdk/platform-tools/adb}
PG_IMAGE=${PG_IMAGE:-postgres:17-alpine}
logdir="${TMPDIR:-/tmp}"; logdir="${logdir%/}/hdd-acceptance-$$"

pg_container="hdd-accept-pg-$$"
db_name="hdd_accept"
bucket="raw-accept-$$"

for port in "$HDD_PORT" "$RECOGNIZER_PORT" "$METRO_PORT"; do
  if lsof -nP -iTCP:"$port" -sTCP:LISTEN >/dev/null 2>&1; then
    echo "android-acceptance: port $port is already in use" >&2
    exit 1
  fi
done

for cmd in docker curl uv npx; do
  if ! command -v "$cmd" >/dev/null 2>&1; then
    echo "android-acceptance: $cmd is required" >&2
    exit 1
  fi
done
if [ ! -x "$ADB" ]; then
  echo "android-acceptance: adb not found at $ADB" >&2
  exit 1
fi
if ! "$ADB" devices | awk 'NR>1 && $2=="device" {found=1} END {exit !found}'; then
  echo "android-acceptance: no authorized Android device" >&2
  exit 1
fi
if ! curl -fsS "$S3_ENDPOINT" >/dev/null 2>&1; then
  echo "android-acceptance: object store not reachable at $S3_ENDPOINT" >&2
  exit 1
fi

mkdir -p "$logdir"
cleaned=false

cleanup() {
  [ "$cleaned" = true ] && return
  cleaned=true
  echo
  echo "Stopping isolated acceptance stack..."
  for port in "$HDD_PORT" "$RECOGNIZER_PORT" "$METRO_PORT"; do
    pids=$(lsof -tiTCP:"$port" -sTCP:LISTEN 2>/dev/null || true)
    [ -n "$pids" ] && kill $pids 2>/dev/null || true
  done
  "$ADB" reverse --remove "tcp:$HDD_PORT" >/dev/null 2>&1 || true
  "$ADB" reverse --remove "tcp:$METRO_PORT" >/dev/null 2>&1 || true
  python3 - "$S3_ENDPOINT" "$bucket" <<'PY' >/dev/null 2>&1 || true
import sys, urllib.request, xml.etree.ElementTree as ET
endpoint, bucket = sys.argv[1], sys.argv[2]
base = f"{endpoint.rstrip('/')}/{bucket}"
xml = urllib.request.urlopen(base + "?list-type=2").read()
root = ET.fromstring(xml)
ns = {"s": "http://s3.amazonaws.com/doc/2006-03-01/"}
for item in root.findall("s:Contents", ns):
    key = item.find("s:Key", ns).text
    urllib.request.urlopen(urllib.request.Request(f"{base}/{key}", method="DELETE")).read()
urllib.request.urlopen(urllib.request.Request(base, method="DELETE")).read()
PY
  docker rm -f "$pg_container" >/dev/null 2>&1 || true
  echo "Removed isolated DB and bucket. Restore the App Base URL you used before acceptance."
  echo "Logs: $logdir"
}
trap cleanup EXIT INT TERM

echo "Starting disposable Postgres..."
docker run -d --name "$pg_container" \
  -e POSTGRES_PASSWORD=accept \
  -p 127.0.0.1::5432 \
  "$PG_IMAGE" >/dev/null
pg_port=$(docker port "$pg_container" 5432/tcp | awk -F: '{print $NF}')
for _ in $(seq 1 30); do
  docker exec "$pg_container" pg_isready -U postgres >/dev/null 2>&1 && break
  sleep 1
done
docker exec "$pg_container" psql -U postgres -d postgres -q \
  -c "CREATE DATABASE $db_name" >/dev/null

echo "Starting recognizer..."
(
  cd recognizer
  uv run uvicorn recognizer.api:app --host 127.0.0.1 --port "$RECOGNIZER_PORT" \
    >"$logdir/recognizer.log" 2>&1
) &
for _ in $(seq 1 60); do
  curl -fsS "http://127.0.0.1:$RECOGNIZER_PORT/healthz" >/dev/null 2>&1 && break
  sleep 1
done
if ! curl -fsS "http://127.0.0.1:$RECOGNIZER_PORT/healthz" >/dev/null 2>&1; then
  echo "android-acceptance: recognizer did not become ready; see $logdir/recognizer.log" >&2
  exit 1
fi

echo "Starting hdd serve..."
HDD_DATABASE_URL="postgres://postgres:accept@127.0.0.1:$pg_port/$db_name?sslmode=disable" \
HDD_RECOGNIZER_URL="http://127.0.0.1:$RECOGNIZER_PORT" \
HDD_S3_ENDPOINT="$S3_ENDPOINT" \
HDD_S3_BUCKET="$bucket" \
HDD_FLAG_POLICY="recognizer/recognizer/flag_policy.yaml" \
go run ./cmd/hdd serve --addr ":$HDD_PORT" >"$logdir/hdd.log" 2>&1 &
for _ in $(seq 1 60); do
  curl -fsS "http://127.0.0.1:$HDD_PORT/api/v1/health" >/dev/null 2>&1 && break
  sleep 1
done
if ! curl -fsS "http://127.0.0.1:$HDD_PORT/api/v1/health" >/dev/null 2>&1; then
  echo "android-acceptance: hdd did not become ready; see $logdir/hdd.log" >&2
  exit 1
fi

echo "Starting Metro..."
(
  cd apps/mobile
  npx expo start --localhost --port "$METRO_PORT" >"$logdir/metro.log" 2>&1
) &
for _ in $(seq 1 60); do
  curl -fsS "http://127.0.0.1:$METRO_PORT/status" >/dev/null 2>&1 && break
  sleep 1
done
if ! curl -fsS "http://127.0.0.1:$METRO_PORT/status" >/dev/null 2>&1; then
  echo "android-acceptance: Metro did not become ready; see $logdir/metro.log" >&2
  exit 1
fi

"$ADB" reverse "tcp:$HDD_PORT" "tcp:$HDD_PORT" >/dev/null
"$ADB" reverse "tcp:$METRO_PORT" "tcp:$METRO_PORT" >/dev/null
"$ADB" shell am start -a android.intent.action.VIEW \
  -d "exp://127.0.0.1:$METRO_PORT" host.exp.exponent >/dev/null 2>&1 || true

cat <<EOF

Isolated acceptance stack is ready.

  App Base URL:  http://127.0.0.1:$HDD_PORT/api/v1
  Metro:         http://127.0.0.1:$METRO_PORT
  Logs:          $logdir

Open the App settings and set the Base URL above. This stack uses a disposable
database and the bucket "$bucket"; the shared deploy data is untouched.

Press Ctrl+C to stop and clean up. Afterward, restore the App Base URL you used
before acceptance.
EOF

wait
