#!/usr/bin/env bash
# App E2E 本地回路:专用 compose 栈(项目 hdd-e2e)+ Expo Web + 确定性 UI 用例。
# 叠加载 e2e/compose.yml:识别服务对固定样本回放 golden,其余样本仍走真实 OCR。
# 每次先重建临时库/卷,不触碰共享 deploy 栈;失败证据在 apps/mobile/.e2e/。
# 用法:cd apps/mobile && npm run e2e [-- <e2e run 参数>]
# 环境变量:
#   E2E_KEEP_STACK=1   运行后保留栈(便于手工看 Web/API);下次运行前仍会重建。
#   E2E_API_BASE_URL   覆盖测试进程使用的 API 基址(默认与 HDD_PORT 同源)。
set -euo pipefail

APP_DIR="$(cd "$(dirname "$0")/.." && pwd)"
REPO_ROOT="$(cd "$APP_DIR/../.." && pwd)"
COMPOSE_FILE="$REPO_ROOT/deploy/docker-compose.yml"
E2E_COMPOSE_FILE="$APP_DIR/e2e/compose.yml"
PROJECT=hdd-e2e

# 专用端口:与共享 deploy 栈默认端口错开,可同时运行(见 docs/agents/app-e2e.md)。
HDD_SERVE_PORT="${HDD_SERVE_PORT:-58080}"
RECOGNIZER_PORT="${RECOGNIZER_PORT:-58000}"
POSTGRES_PORT="${POSTGRES_PORT:-55432}"
S3_API_PORT="${S3_API_PORT:-58333}"
SEAWEEDFS_MASTER_PORT="${SEAWEEDFS_MASTER_PORT:-59333}"
export HDD_SERVE_PORT RECOGNIZER_PORT POSTGRES_PORT S3_API_PORT SEAWEEDFS_MASTER_PORT
# Expo Web 跨源读 API:仅本地验证栈临时打开(共享栈保持 ADR-0006 的关闭缺省)。
export HDD_SERVE_CORS_ORIGIN='*'
export E2E_API_BASE_URL="${E2E_API_BASE_URL:-http://127.0.0.1:${HDD_SERVE_PORT}/api/v1}"
export E2E_TELEMETRY_DISABLED=1

compose() {
  docker compose -p "$PROJECT" -f "$COMPOSE_FILE" -f "$E2E_COMPOSE_FILE" "$@"
}

cleanup() {
  if [ "${E2E_KEEP_STACK:-0}" = "1" ]; then
    echo "E2E stack kept: project=$PROJECT hdd=http://127.0.0.1:${HDD_SERVE_PORT}/api/v1"
    echo "Stop with: docker compose -p $PROJECT -f $COMPOSE_FILE -f $E2E_COMPOSE_FILE down -v"
  else
    compose down -v --remove-orphans >/dev/null 2>&1 || true
  fi
}
trap cleanup EXIT

wait_for() {
  local url="$1" name="$2"
  for _ in $(seq 1 90); do
    if curl -fsS "$url" >/dev/null 2>&1; then
      echo "ready: $name"
      return 0
    fi
    sleep 2
  done
  echo "timed out waiting for $name ($url)" >&2
  compose logs --tail=80 hdd recognizer >&2 || true
  return 1
}

echo "Recreating isolated E2E stack ($PROJECT) ..."
compose down -v --remove-orphans >/dev/null 2>&1 || true
compose up -d --build
wait_for "http://127.0.0.1:${HDD_SERVE_PORT}/api/v1/health" "hdd"
wait_for "http://127.0.0.1:${RECOGNIZER_PORT}/healthz" "recognizer"

cd "$APP_DIR"
npx e2e run "$@"
