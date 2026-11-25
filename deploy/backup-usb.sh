#!/usr/bin/env bash
# 备份 Postgres + SeaweedFS 到 USB（restic 加密仓库）。
#
# 设计取舍：
# - 先停 hdd 与 seaweedfs，避免备份期间有新上传/对象存储写入；Postgres 继续运行，pg_dump 提供一致快照。
# - SeaweedFS 对象按 sha256 内容寻址、不覆盖；停止后目录静止，restic 直接读数据目录。
# - 首次是全量，之后 restic 增量；USB 必须是独立挂载点，不能落回系统盘。
#
# 用法：
#   deploy/backup-usb.sh
#   CHECK=1 deploy/backup-usb.sh   # 备份后额外做 restic check（较慢）
#
# 依赖：docker、restic/restic 镜像、deploy/.env 中的 HDD_DATA_DIR / BACKUP_ROOT / RESTIC_PASSWORD_FILE。
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
PROJECT="${PROJECT:-healthyduoduo}"
COMPOSE_FILE="${COMPOSE_FILE:-$ROOT/deploy/docker-compose.nas.yml}"
ENV_FILE="${ENV_FILE:-$ROOT/deploy/.env}"
RESTIC_IMAGE="${RESTIC_IMAGE:-restic/restic:latest}"

if [ ! -f "$ENV_FILE" ]; then
  echo "错误: 缺少 $ENV_FILE（从 deploy/.env.example 复制）" >&2
  exit 1
fi

# shellcheck disable=SC1090
set -a
. "$ENV_FILE"
set +a

: "${HDD_DATA_DIR:?deploy/.env 缺少 HDD_DATA_DIR}"
: "${BACKUP_ROOT:?deploy/.env 缺少 BACKUP_ROOT}"
: "${RESTIC_PASSWORD_FILE:?deploy/.env 缺少 RESTIC_PASSWORD_FILE}"
[ -d "$HDD_DATA_DIR/seaweed" ] || { echo "错误: $HDD_DATA_DIR/seaweed 不存在" >&2; exit 1; }
[ -f "$RESTIC_PASSWORD_FILE" ] || { echo "错误: $RESTIC_PASSWORD_FILE 不存在" >&2; exit 1; }

mkdir -p "$BACKUP_ROOT"
mount_target="$(df -P "$BACKUP_ROOT" | awk 'NR==2 {print $6}')"
if [ "$mount_target" = "/" ] && [ "${ALLOW_ROOT_FS:-0}" != "1" ]; then
  echo "错误: $BACKUP_ROOT 不在独立挂载点上（当前落在 /）。请挂载 USB，或显式 ALLOW_ROOT_FS=1。" >&2
  exit 1
fi

REPOSITORY="$BACKUP_ROOT/restic"
STAGING="$(mktemp -d "$HDD_DATA_DIR/backup-staging.XXXXXX")"
TS="$(date +%Y%m%d-%H%M%S)"

compose() {
  docker compose -p "$PROJECT" -f "$COMPOSE_FILE" --env-file "$ENV_FILE" "$@"
}

restic_run() {
  docker run --rm \
    -e RESTIC_PASSWORD_FILE=/run/secrets/restic_password \
    -v "$RESTIC_PASSWORD_FILE:/run/secrets/restic_password:ro" \
    -v "$BACKUP_ROOT:/repo" \
    -v "$STAGING:/staging:ro" \
    -v "$HDD_DATA_DIR/seaweed:/data:ro" \
    "$RESTIC_IMAGE" -r /repo/restic "$@"
}

HDD_WAS_RUNNING=0
if [ -n "$(compose ps -q hdd)" ]; then
  HDD_WAS_RUNNING=1
fi
SEAWEEDFS_WAS_RUNNING=0
if [ -n "$(compose ps -q seaweedfs)" ]; then
  SEAWEEDFS_WAS_RUNNING=1
fi

cleanup() {
  status=$?
  rm -rf "$STAGING"
  if [ "$SEAWEEDFS_WAS_RUNNING" = "1" ]; then
    compose start seaweedfs >/dev/null || true
    # SeaweedFS S3 网关需等 raft 收敛；hdd 需要能重新 EnsureBucket。
    sleep 20
  fi
  if [ "$HDD_WAS_RUNNING" = "1" ]; then
    compose start hdd >/dev/null || true
  fi
  exit "$status"
}
trap cleanup EXIT

echo "停止 hdd 与 seaweedfs（备份期间 App 不可用）..."
compose stop hdd seaweedfs >/dev/null

mkdir -p "$STAGING/db"
echo "导出 Postgres..."
compose exec -T postgres pg_dump -U healthyduoduo -d healthyduoduo -Fc > "$STAGING/db/healthyduoduo-$TS.dump"
chmod 600 "$STAGING/db/healthyduoduo-$TS.dump"

if [ ! -f "$REPOSITORY/config" ]; then
  echo "初始化 restic 仓库..."
  restic_run init
fi

echo "写入 restic 仓库..."
restic_run backup /staging /data --tag "nas-$TS"
restic_run forget --keep-daily 14 --keep-weekly 8 --keep-monthly 6 --prune

if [ "${CHECK:-0}" = "1" ]; then
  echo "校验 restic 仓库..."
  restic_run check
fi

echo "备份完成: $REPOSITORY"
echo "恢复参见 deploy/NAS.md 的「从 USB 恢复」。"
