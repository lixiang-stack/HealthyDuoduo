#!/usr/bin/env bash
# 本地 Git 策略:commit subject 前缀。
set -euo pipefail

msg_file=${1:-}
if [ -z "$msg_file" ] || [ ! -f "$msg_file" ]; then
  echo "✖ commit-msg: 缺少提交信息文件参数。" >&2
  exit 1
fi

# merge commit 由 Git 生成消息,不要求产品前缀。
if git rev-parse --verify --quiet MERGE_HEAD >/dev/null; then
  exit 0
fi

subject=$(head -n 1 "$msg_file")
if [ -z "$subject" ]; then
  echo "✖ commit message 不能为空。" >&2
  exit 1
fi
if ! printf '%s\n' "$subject" | grep -Eq '^(feat|fix|test|docs|chore|refactor|perf|build|ci|style|revert)(\([^()]+\))?!?: .+'; then
  echo "✖ commit subject 必须以 feat:/fix:/test:/docs:/chore:/... 前缀开头,并说明意图。" >&2
  echo "  当前: $subject" >&2
  exit 1
fi
