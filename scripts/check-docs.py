#!/usr/bin/env python3
"""零依赖文档检查:markdown 表格列数、相对链接、代码围栏、ticket Blocked by 引用。

CI 全量检查;pre-commit 用 --staged 只检查暂存的 markdown。
"""

from __future__ import annotations

import argparse
import re
import subprocess
import sys
import urllib.parse
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent

SKIP_DIRS = {
    ".git",
    ".worktree",
    "node_modules",
    "data",
    "dist",
    "vendor",
    ".expo",
    ".venv",
    "__pycache__",
    ".pytest_cache",
    ".ruff_cache",
}
LINK_ROOTS = (".scratch", "docs/design", "docs/adr", "docs/reference")
LINK_FILES = {"README.md", "CONTEXT.md", "AGENTS.md", "apps/mobile/AGENTS.md"}
FENCE_RE = re.compile(r"^\s*(```|~~~)")
LINK_RE = re.compile(r"!?\[[^\]]*\]\(([^)]+)\)")
BLOCKED_RE = re.compile(r"^\*\*Blocked by:\*\*\s*(.*)$", re.IGNORECASE)
ISSUE_NUM_RE = re.compile(r"\b(\d{2})\b")
SEP_CELL_RE = re.compile(r"^:?-{3,}:?$")


def relative(path: Path) -> str:
    return path.relative_to(ROOT).as_posix()


def iter_markdown() -> list[Path]:
    files: list[Path] = []
    for path in sorted(ROOT.rglob("*.md")):
        rel = path.relative_to(ROOT)
        if any(part in SKIP_DIRS for part in rel.parts):
            continue
        files.append(path)
    return files


def pipe_count(line: str) -> int:
    return line.replace("\\|", "").count("|")


def is_table_row(line: str) -> bool:
    return line.lstrip().startswith("|")


def is_separator(line: str) -> bool:
    if not is_table_row(line):
        return False
    cells = [cell.strip() for cell in line.replace("\\|", "").strip().strip("|").split("|")]
    return bool(cells) and all(SEP_CELL_RE.match(cell) for cell in cells)


def table_errors(lines: list[str]) -> list[tuple[int, str]]:
    errors: list[tuple[int, str]] = []
    in_fence = False
    index = 0
    while index < len(lines):
        line = lines[index]
        if FENCE_RE.match(line):
            in_fence = not in_fence
            index += 1
            continue
        if in_fence:
            index += 1
            continue
        if is_table_row(line) and index + 1 < len(lines) and is_separator(lines[index + 1]):
            expected = pipe_count(line)
            if pipe_count(lines[index + 1]) != expected:
                errors.append(
                    (
                        index + 2,
                        f"表格分隔行有 {pipe_count(lines[index + 1])} 个 |,表头有 {expected} 个",
                    )
                )
            index += 2
            while index < len(lines) and is_table_row(lines[index]):
                if pipe_count(lines[index]) != expected:
                    errors.append(
                        (
                            index + 1,
                            f"表格行有 {pipe_count(lines[index])} 个 |,表头有 {expected} 个",
                        )
                    )
                index += 1
            continue
        index += 1
    return errors


def should_check_links(path: Path) -> bool:
    rel = relative(path)
    if rel in LINK_FILES:
        return True
    return any(rel == root or rel.startswith(root + "/") for root in LINK_ROOTS)


def is_remote(target: str) -> bool:
    return target.startswith(
        ("#", "http://", "https://", "mailto:", "tel:", "data:", "javascript:")
    )


def resolve_link(path: Path, target: str) -> Path | None:
    if target.startswith("<") and target.endswith(">"):
        target = target[1:-1]
    target = urllib.parse.unquote(target.split("#", 1)[0].split("?", 1)[0])
    if not target:
        return None
    if target.startswith("/"):
        return ROOT / target.lstrip("/")
    resolved = (path.parent / target).resolve()
    if resolved.exists():
        return resolved
    # 允许从仓库根写的相对路径(如 .scratch/<feature>/spec.md)
    root_resolved = (ROOT / target).resolve()
    return root_resolved if root_resolved.exists() else resolved


def link_errors(path: Path, lines: list[str]) -> list[tuple[int, str]]:
    if not should_check_links(path):
        return []
    errors: list[tuple[int, str]] = []
    in_fence = False
    for index, line in enumerate(lines, 1):
        if FENCE_RE.match(line):
            in_fence = not in_fence
            continue
        if in_fence:
            continue
        for match in LINK_RE.finditer(line):
            raw = match.group(1).strip()
            if not raw or is_remote(raw):
                continue
            resolved = resolve_link(path, raw)
            if resolved is None:
                continue
            if not resolved.exists():
                errors.append((index, f"链接目标不存在: {raw}"))
    return errors


def fence_errors(lines: list[str]) -> list[tuple[int, str]]:
    open_at: int | None = None
    for index, line in enumerate(lines, 1):
        if FENCE_RE.match(line):
            open_at = None if open_at is not None else index
    if open_at is None:
        return []
    return [(open_at, "代码围栏未闭合")]


def blocked_errors(path: Path, lines: list[str]) -> list[tuple[int, str]]:
    if path.parent.name != "issues" or ".scratch" not in path.parts:
        return []
    errors: list[tuple[int, str]] = []
    for index, line in enumerate(lines, 1):
        match = BLOCKED_RE.match(line)
        if not match:
            continue
        body = match.group(1).strip()
        if not body or body.lower().startswith("none") or body in {"—", "-"}:
            continue
        for number in ISSUE_NUM_RE.findall(body):
            if not list(path.parent.glob(f"{number}-*.md")):
                errors.append((index, f"Blocked by {number} 没有对应 issue 文件"))
    return errors


def check_file(path: Path) -> list[tuple[int, str]]:
    lines = path.read_text(encoding="utf-8").splitlines()
    errors: list[tuple[int, str]] = []
    errors.extend(table_errors(lines))
    errors.extend(link_errors(path, lines))
    errors.extend(fence_errors(lines))
    errors.extend(blocked_errors(path, lines))
    return errors


def staged_markdown() -> list[Path]:
    try:
        output = subprocess.check_output(
            ["git", "-C", str(ROOT), "diff", "--cached", "--name-only", "--diff-filter=ACM"],
            text=True,
        )
    except (OSError, subprocess.CalledProcessError):
        return []
    files: list[Path] = []
    for name in output.splitlines():
        if not name.endswith(".md"):
            continue
        path = ROOT / name
        if path.exists() and not any(part in SKIP_DIRS for part in path.relative_to(ROOT).parts):
            files.append(path)
    return files


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--staged", action="store_true", help="只检查暂存的 markdown")
    args = parser.parse_args()

    files = staged_markdown() if args.staged else iter_markdown()
    if not files:
        print("docs check: 没有需要检查的 markdown")
        return 0

    total = 0
    for path in files:
        for line, message in check_file(path):
            total += 1
            print(f"ERROR {relative(path)}:{line}: {message}")

    if total:
        print(f"docs check: {len(files)} 个文件,{total} 个错误")
        return 1
    print(f"docs check: {len(files)} 个文件 OK")
    return 0


if __name__ == "__main__":
    sys.exit(main())
