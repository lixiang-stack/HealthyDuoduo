#!/usr/bin/env bash
# Run the disposable-Postgres storage suite before push only when the branch
# touches storage / migrations / server-derived rules. Keeps routine pushes fast
# while catching storage regressions before CI (go test skips these without DSN).
set -euo pipefail

root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
cd "$root"

base=origin/main
if ! git rev-parse --verify --quiet "$base" >/dev/null; then
  echo "pre-push: $base not found; running storage tests"
  exec bash scripts/test-db.sh
fi

changed=$(git diff --name-only "$base"...HEAD -- internal/storage internal/pipeline)
if [ -z "$changed" ]; then
  echo "pre-push: no storage/pipeline changes; skip test-db"
  exit 0
fi

echo "pre-push: storage/pipeline changes detected; running scripts/test-db.sh"
bash scripts/test-db.sh
