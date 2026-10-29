#!/usr/bin/env bash
# 本地 Git 策略:禁止直接 push main;commit subject 前缀在 push 前兜底检查。
set -euo pipefail

remote=${1:-origin}
main_ref="refs/remotes/$remote/main"
status=0
zero_sha=0000000000000000000000000000000000000000
msg_file=$(mktemp)
trap 'rm -f "$msg_file"' EXIT

while read -r local_ref local_sha remote_ref remote_sha; do
  [ -n "${local_ref:-}" ] || continue

  case "$remote_ref" in
    refs/heads/main)
      echo "✖ 禁止直接 push main;请通过 PR 合入。" >&2
      status=1
      ;;
    refs/heads/*)
      if [ "$local_sha" = "$zero_sha" ]; then
        continue
      fi
      if git rev-parse --verify --quiet "$main_ref" >/dev/null; then
        base=$(git merge-base "$main_ref" "$local_sha" 2>/dev/null || true)
        if [ -n "$base" ]; then
          # commit-msg hook 在 core.hooksPath 环境可能未安装;push 前兜底检查。
          while IFS= read -r sha; do
            git log -1 --format=%B "$sha" > "$msg_file"
            if ! bash scripts/git/check-commit-msg.sh "$msg_file"; then
              status=1
            fi
          done < <(git rev-list --no-merges "$base..$local_sha")
        fi
      fi
      ;;
  esac
done

exit "$status"
