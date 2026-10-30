#!/usr/bin/env bash
# AI agent 提交守卫:worktree 必须位于仓库 .worktree/ 下。
# 人类贡献者(未设置 AI_AGENT)可使用普通 clone/分支,直接放行。
set -euo pipefail

if [ -z "${AI_AGENT:-}" ]; then
  exit 0
fi

root="$(git rev-parse --show-toplevel)"
common="$(git rev-parse --git-common-dir)"
main_root="$(cd "$(dirname "$common")" && pwd)"

case "$root" in
  "$main_root"/.worktree/*) exit 0 ;;
esac

printf 'worktree path violation: %s\n' "$root" >&2
printf 'expected under: %s/.worktree/<name>\n' "$main_root" >&2
printf 'create with: opencode worktree tool (opencode.json sets .worktree)\n' >&2
exit 1
