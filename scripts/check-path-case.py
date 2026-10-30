#!/usr/bin/env python3
"""Reject paths that differ only by case.

macOS and Windows checkouts are case-insensitive: two tracked paths such as
`Timeline.tsx` and `timeline.ts` look distinct in git but collide on disk.
Catch it before the checkout / TypeScript gives a confusing error.

Usage:
  python3 scripts/check-path-case.py            # all tracked paths (CI)
  python3 scripts/check-path-case.py --staged   # staged paths (pre-commit)
"""

from __future__ import annotations

import argparse
import subprocess
import sys
from collections import defaultdict


def git_paths(*args: str) -> list[str]:
    output = subprocess.check_output(["git", *args], text=True)
    return [path for path in output.split("\0") if path]


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--staged", action="store_true")
    args = parser.parse_args()

    paths = (
        git_paths("diff", "--cached", "--name-only", "-z", "--diff-filter=ACMR")
        if args.staged
        else git_paths("ls-files", "-z")
    )

    groups: dict[str, set[str]] = defaultdict(set)
    for path in paths:
        groups[path.casefold()].add(path)

    collisions = [sorted(group) for group in groups.values() if len(group) > 1]
    if not collisions:
        return 0

    print("paths differ only by case (not portable to macOS/Windows):", file=sys.stderr)
    for group in collisions:
        print("  " + "  <>  ".join(group), file=sys.stderr)
    return 1


if __name__ == "__main__":
    raise SystemExit(main())
