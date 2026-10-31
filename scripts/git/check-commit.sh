#!/usr/bin/env bash
# 本地 Git 策略:禁止在 detached HEAD、main 或主检出(非 .worktree)提交。
set -euo pipefail

branch=$(git branch --show-current)
if [ -z "$branch" ]; then
  echo "✖ detached HEAD:先 git switch -c <branch> 再提交。" >&2
  exit 1
fi
if [ "$branch" = "main" ]; then
  echo "✖ main 分支禁止提交;请在 .worktree/<需求名> 工作区开发。" >&2
  exit 1
fi

root=$(git rev-parse --show-toplevel)
main_worktree=$(git worktree list --porcelain | sed -n 's/^worktree //p' | head -n 1)
if [ "$root" = "$main_worktree" ]; then
  echo "✖ 主检出禁止开发;请使用 .worktree/<需求名> 工作区(当前分支: ${branch:-detached})。" >&2
  exit 1
fi
