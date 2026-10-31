"""Golden 变更摘要:对比某个 git ref 与工作区的 report golden。

用法(仓库根或 worktree 根执行):

  cd recognizer && uv run python tools/golden_diff.py
  cd recognizer && uv run python tools/golden_diff.py --ref HEAD^
  cd recognizer && uv run python tools/golden_diff.py --ref origin/main

默认对比 `HEAD` 与当前工作区,适合提交前审阅 golden 变更;要回看最近一次提交
改为 `--ref HEAD^`。只读取 `samples/expected/report/*.json`,不碰 OCR/本地数据。
"""

import argparse
import json
import subprocess
import sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parents[1]))

from recognizer import paths

DEFAULT_DIR = "samples/expected/report"
# raw_text / ocr_line_numbers 是抽取留痕,不是标准输出字段,避免噪声。
ITEM_FIELDS = (
    "flag",
    "value",
    "unit",
    "unit_display",
    "unit_status",
    "ref_range",
    "ref_kind",
    "result_kind",
    "result_basis",
    "raw_value",
    "raw_unit",
    "raw_ref_range",
)


def _fmt(value: object) -> str:
    return json.dumps(value, ensure_ascii=False)


def _changed(old: object, new: object) -> bool:
    return old != new


def diff_reports(old: dict | None, new: dict | None) -> list[str]:
    """两份 report golden 的差异行;None 表示该侧文件不存在。"""
    if old is None:
        return ["  new golden"]
    if new is None:
        return ["  deleted golden"]

    lines: list[str] = []
    for field in ("report_type", "status", "panels"):
        if _changed(old.get(field), new.get(field)):
            lines.append(f"  {field}: {_fmt(old.get(field))} -> {_fmt(new.get(field))}")

    old_items = old.get("items") or []
    new_items = new.get("items") or []
    if len(old_items) != len(new_items):
        lines.append(f"  item count: {len(old_items)} -> {len(new_items)}")
    for index in range(min(len(old_items), len(new_items))):
        before, after = old_items[index], new_items[index]
        prefix = f"  [{index}]"
        if before.get("name") != after.get("name"):
            lines.append(
                f"{prefix} name: {_fmt(before.get('name'))} -> {_fmt(after.get('name'))}"
            )
        for field in ITEM_FIELDS:
            if _changed(before.get(field), after.get(field)):
                lines.append(
                    f"{prefix} {before.get('name')}: {field} "
                    f"{_fmt(before.get(field))} -> {_fmt(after.get(field))}"
                )
    if len(new_items) > len(old_items):
        for item in new_items[len(old_items):]:
            lines.append(f"  + {item.get('name')}")
    elif len(old_items) > len(new_items):
        for item in old_items[len(new_items):]:
            lines.append(f"  - {item.get('name')}")
    return lines


def _git(repo: Path, *args: str) -> subprocess.CompletedProcess[str]:
    return subprocess.run(
        ["git", *args], cwd=repo, capture_output=True, text=True, check=False
    )


def changed_paths(repo: Path, ref: str, directory: str) -> list[tuple[str, str, str | None]]:
    """(status, path, old_path) 列表;rename/copy 同时给出旧路径。"""
    result = _git(repo, "diff", "--name-status", "--no-renames", ref, "--", directory)
    if result.returncode != 0:
        raise RuntimeError(result.stderr.strip() or f"git diff failed for {ref}")
    out: list[tuple[str, str, str | None]] = []
    for line in result.stdout.splitlines():
        parts = line.split("\t")
        if len(parts) < 2:
            continue
        out.append((parts[0], parts[1], None))
    return out


def _show(repo: Path, ref: str, path: str) -> dict | None:
    result = _git(repo, "show", f"{ref}:{path}")
    if result.returncode != 0:
        return None
    return json.loads(result.stdout)


def _load(path: Path) -> dict | None:
    if not path.exists():
        return None
    return json.loads(path.read_text(encoding="utf-8"))


def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser(description="对比 git ref 与工作区的 report golden")
    parser.add_argument("--ref", default="HEAD", help="git ref(默认 HEAD)")
    parser.add_argument("--dir", default=DEFAULT_DIR, help=f"golden 目录(默认 {DEFAULT_DIR})")
    args = parser.parse_args(argv)

    repo = paths.REPO_ROOT
    directory = args.dir.strip("/")
    try:
        changed = changed_paths(repo, args.ref, directory)
    except RuntimeError as exc:
        print(f"golden_diff: {exc}", file=sys.stderr)
        return 1

    if not changed:
        print(f"golden_diff: no changes vs {args.ref} under {directory}")
        return 0

    changed_samples = 0
    for status, path, old_path in changed:
        old = _show(repo, args.ref, old_path or path)
        new = _load(repo / path)
        lines = diff_reports(old, new)
        if not lines:
            continue
        changed_samples += 1
        print(f"{Path(path).stem} ({status}):")
        print("\n".join(lines))
    print(f"golden_diff: {changed_samples} changed sample(s) vs {args.ref}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
