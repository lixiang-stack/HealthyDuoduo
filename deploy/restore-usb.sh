#!/usr/bin/env bash
# 从 USB restic 仓库恢复 Postgres + SeaweedFS。
#
# 这是破坏性操作：会覆盖当前数据库对象和 SeaweedFS 数据目录。
# 只在灾难恢复/验收演练时使用；执行前先确认当前没有正在写入的 App。
#
# 用法：
#   RESTORE_YES=1 deploy/restore-usb.sh
#
# 依赖：docker、restic/restic 镜像、deploy/.env 中的 HDD_DATA_DIR / BACKUP_ROOT / RESTIC_PASSWORD_FILE。
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
PROJECT="${PROJECT:-healthyduoduo}"
COMPOSE_FILE="${COMPOSE_FILE:-$ROOT/deploy/docker-compose.nas.yml}"
ENV_FILE="${ENV_FILE:-$ROOT/deploy/.env}"
RESTIC_IMAGE="${RESTIC_IMAGE:-restic/restic:latest}"

if [ "${RESTORE_YES:-0}" != "1" ]; then
  echo "这是破坏性恢复。确认后执行: RESTORE_YES=1 deploy/restore-usb.sh" >&2
  exit 1
fi

if [ ! -f "$ENV_FILE" ]; then
  echo "错误: 缺少 $ENV_FILE" >&2
  exit 1
fi

# shellcheck disable=SC1090
set -a
. "$ENV_FILE"
set +a

: "${HDD_DATA_DIR:?deploy/.env 缺少 HDD_DATA_DIR}"
: "${BACKUP_ROOT:?deploy/.env 缺少 BACKUP_ROOT}"
: "${RESTIC_PASSWORD_FILE:?deploy/.env 缺少 RESTIC_PASSWORD_FILE}"
[ -f "$RESTIC_PASSWORD_FILE" ] || { echo "错误: $RESTIC_PASSWORD_FILE 不存在" >&2; exit 1; }

REPOSITORY="$BACKUP_ROOT/restic"
[ -f "$REPOSITORY/config" ] || { echo "错误: $REPOSITORY 不是 restic 仓库" >&2; exit 1; }

mount_target="$(df -P "$BACKUP_ROOT" | awk 'NR==2 {print $6}')"
if [ "$mount_target" = "/" ] && [ "${ALLOW_ROOT_FS:-0}" != "1" ]; then
  echo "错误: $BACKUP_ROOT 不在独立挂载点上（当前落在 /）。" >&2
  exit 1
fi

RESTORE_DIR="$(mktemp -d "$HDD_DATA_DIR/restore.XXXXXX")"

compose() {
  docker compose -p "$PROJECT" -f "$COMPOSE_FILE" --env-file "$ENV_FILE" "$@"
}

restic_run() {
  docker run --rm \
    -e RESTIC_PASSWORD_FILE=/run/secrets/restic_password \
    -v "$RESTIC_PASSWORD_FILE:/run/secrets/restic_password:ro" \
    -v "$BACKUP_ROOT:/repo" \
    -v "$RESTORE_DIR:/restore" \
    "$RESTIC_IMAGE" -r /repo/restic "$@"
}

echo "恢复最新快照到 $RESTORE_DIR ..."
restic_run restore latest --target /restore

DB_DUMP="$(find "$RESTORE_DIR/staging/db" -maxdepth 1 -name 'healthyduoduo-*.dump' -type f | sort | tail -n 1)"
if [ -z "$DB_DUMP" ]; then
  echo "错误: 快照里没有找到数据库 dump（$RESTORE_DIR/staging/db）" >&2
  exit 1
fi
if [ ! -d "$RESTORE_DIR/data" ]; then
  echo "错误: 快照里没有找到 SeaweedFS 数据目录（$RESTORE_DIR/data）" >&2
  exit 1
fi

echo "停止 hdd 与 seaweedfs..."
compose stop hdd seaweedfs >/dev/null

echo "恢复 SeaweedFS 数据..."
mkdir -p "$HDD_DATA_DIR/seaweed"
rsync -a --delete "$RESTORE_DIR/data/" "$HDD_DATA_DIR/seaweed/"

echo "启动 seaweedfs，等待 raft 收敛..."
compose start seaweedfs >/dev/null
sleep 20

echo "恢复 Postgres..."
compose exec -T postgres pg_restore -U healthyduoduo -d healthyduoduo \
  --clean --if-exists --no-owner --no-privileges < "$DB_DUMP"

echo "启动 hdd..."
compose start hdd >/dev/null

rm -rf "$RESTORE_DIR"
echo "恢复完成。请立即打开 App 抽查报告/原图，并跑一次 deploy/NAS.md 的验收清单。"
