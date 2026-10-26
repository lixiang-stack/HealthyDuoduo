#!/usr/bin/env bash
# 清空共享 deploy 栈的数据并重建:Postgres(pgdata)+ 对象存储(seaweeddata)。
# 只作用于 compose 项目 `deploy`,不触碰 hdd-e2e 或其它栈;会删卷,不可恢复。
# 用法:
#   deploy/reset-data.sh          # 二次确认后执行
#   deploy/reset-data.sh --yes    # 跳过确认(自动化/复验)
# 端口变量与 deploy/docker-compose.yml 保持一致:HDD_SERVE_PORT / S3_API_PORT。
set -euo pipefail

PROJECT=deploy
COMPOSE_FILE="deploy/docker-compose.yml"
SERVE_PORT="${HDD_SERVE_PORT:-8080}"
S3_PORT="${S3_API_PORT:-8333}"
BUCKET=raw

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

compose() {
  docker compose -p "$PROJECT" -f "$COMPOSE_FILE" "$@"
}

if [ "${1:-}" != "--yes" ]; then
  echo "将删除 compose 项目 $PROJECT 的卷:${PROJECT}_pgdata、${PROJECT}_seaweeddata"
  echo "即清空 Postgres 全部业务数据与 $BUCKET bucket 的原始图像,且不可恢复。"
  read -r -p "确认请输入 yes: " answer
  if [ "$answer" != "yes" ]; then
    echo "已取消。"
    exit 1
  fi
fi

echo "停止并删除 $PROJECT 容器/网络/数据卷..."
compose down -v --remove-orphans

echo "重新启动..."
compose up -d

echo "等待 hdd 迁移与健康检查..."
health="http://127.0.0.1:${SERVE_PORT}/api/v1/health"
ready=0
for _ in $(seq 1 90); do
  if curl -fsS "$health" >/dev/null 2>&1; then
    ready=1
    break
  fi
  sleep 2
done
if [ "$ready" -ne 1 ]; then
  echo "错误:等待 hdd 健康检查超时($health)" >&2
  compose logs --tail=80 hdd postgres >&2 || true
  exit 1
fi

echo "等待对象存储就绪..."
keys=""
for _ in $(seq 1 30); do
  body=$(curl -fsS "http://127.0.0.1:${S3_PORT}/${BUCKET}?list-type=2" 2>/dev/null || true)
  keys=$(printf '%s' "$body" | sed -n 's:.*<KeyCount>\([0-9][0-9]*\)</KeyCount>.*:\1:p')
  [ "$keys" = "0" ] && break
  sleep 2
done
if [ "$keys" != "0" ]; then
  echo "错误:$BUCKET bucket 未清空(KeyCount=${keys:-unknown})" >&2
  exit 1
fi

row_counts=$(compose exec -T postgres psql -U healthyduoduo -d healthyduoduo -Atc \
  "select (select count(*) from members) || ',' || (select count(*) from reports) || ',' || (select count(*) from images)")
if [ "$row_counts" != "0,0,0" ]; then
  echo "错误:数据库未清空(members,reports,images=$row_counts)" >&2
  exit 1
fi

echo "完成:数据库 members/reports/images = 0,0,0;$BUCKET bucket KeyCount=0。"
echo "提示:App 端仍保留上次选择的成员 ID;创建新成员后,请先在成员列表点选该成员再上传。"
