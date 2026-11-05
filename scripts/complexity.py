#!/usr/bin/env python3
"""增量复杂度信号:只报告本次改动新增或变复杂的函数,存量不重复刷屏。

用法:
  python3 scripts/complexity.py --changed [BASE]   # 默认 HEAD^;post-commit
  python3 scripts/complexity.py --push             # 相对 upstream 的整条分支;pre-push
  python3 scripts/complexity.py --audit            # 全量人工审计,不挂 hook

设计:
- Go / Python 做函数级对比:新函数 / 复杂度上升 / 跨过阈值才报告。
- TS 的 ESLint complexity 没有稳定函数 ID,只报告修改文件里 > 阈值的函数;
  当前基线无 >20 函数,因此等同于增量。
- 工具缺失、网络不可用、解析失败都只打印 skip,最终 exit 0,不阻止提交。
- 详细报告写到 $(git rev-parse --git-dir)/hdd-complexity/report.md。
"""

from __future__ import annotations

import argparse
import json
import os
import re
import shutil
import subprocess
import sys
import tempfile
from dataclasses import asdict, dataclass
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent
THRESHOLDS = {
    "go_cyclo": 15,
    "go_cognitive": 15,
    "python": 20,
    "typescript": 20,
}
AUDIT_TOP = 10


@dataclass
class Finding:
    language: str
    kind: str
    file: str
    function: str
    base: int | None
    current: int
    threshold: int
    status: str  # new / increased / current / audit


def run(cmd: list[str]) -> subprocess.CompletedProcess[str]:
    try:
        return subprocess.run(cmd, cwd=ROOT, text=True, capture_output=True, check=False)
    except OSError as exc:
        # 工具缺失时只让该语言空结果,不影响其他语言。
        return subprocess.CompletedProcess(cmd, 127, "", str(exc))


def git(*args: str) -> subprocess.CompletedProcess[str]:
    return run(["git", *args])


def git_dir() -> Path:
    out = git("rev-parse", "--git-dir")
    return ROOT / out.stdout.strip() if out.returncode == 0 else ROOT / ".git"


def relative_path(path: str) -> str:
    try:
        return str(Path(path).resolve().relative_to(ROOT))
    except ValueError:
        return path


def resolve_push_base() -> str:
    for ref in ("@{push}", "origin/main"):
        if git("rev-parse", "--verify", "--quiet", ref).returncode != 0:
            continue
        merge = git("merge-base", ref, "HEAD")
        if merge.returncode == 0:
            return merge.stdout.strip()
    head_parent = git("rev-parse", "--verify", "--quiet", "HEAD^")
    return head_parent.stdout.strip() if head_parent.returncode == 0 else ""


def is_scan_file(path: str) -> bool:
    if "node_modules/" in path or "/.venv/" in path or "/rapidocr-env/" in path:
        return False
    if path.endswith(("_test.go", ".test.ts", ".test.tsx")):
        return False
    if path.startswith((
        ".scratch/",
        "docs/",
        "scripts/",
        "deploy/",
        "recognizer/tests/",
        "recognizer/tools/",
        "apps/mobile/src/test/",
    )):
        return False
    if path in {
        "internal/analysis/who_bmi_data.go",
        "internal/analysis/framework_reference.go",
        "apps/mobile/src/api/schema.d.ts",
    }:
        return False
    return path.endswith((".go", ".py", ".ts", ".tsx"))


def tracked_files() -> list[str]:
    out = git("ls-files", "--", "*.go", "*.py", "*.ts", "*.tsx")
    return [line for line in out.stdout.splitlines() if is_scan_file(line)]


def changed_files(base: str) -> list[str]:
    if not base:
        return []
    out = git("diff", "--name-only", "--diff-filter=ACMR", base, "HEAD", "--",
              "*.go", "*.py", "*.ts", "*.tsx")
    return [line for line in out.stdout.splitlines() if is_scan_file(line)]


def staged_files() -> list[str]:
    out = git("diff", "--cached", "--name-only", "--diff-filter=ACMR", "--",
              "*.go", "*.py", "*.ts", "*.tsx")
    return [line for line in out.stdout.splitlines() if is_scan_file(line)]


def go_cmd(tool: str) -> list[str]:
    if shutil.which(tool):
        return [tool]
    versions = {
        "gocyclo": "github.com/fzipp/gocyclo/cmd/gocyclo@v0.6.0",
        "gocognit": "github.com/uudashr/gocognit/cmd/gocognit@v1.3.0",
    }
    return ["go", "run", versions[tool]]


def parse_go_output(stdout: str, base_by_temp: dict[str, str]) -> dict[tuple[str, str], int]:
    """gocyclo / gocognit 默认格式:复杂度 包名 函数 文件:行:列。"""
    out: dict[tuple[str, str], int] = {}
    for line in stdout.splitlines():
        parts = line.split()
        if len(parts) < 4 or not parts[0].isdigit():
            continue
        file = parts[3].rsplit(":", 2)[0]
        file = base_by_temp.get(str(Path(file).resolve()), relative_path(file))
        out[(file, parts[2])] = int(parts[0])
    return out


def go_metrics(files: list[str], tool: str) -> dict[tuple[str, str], int]:
    if not files:
        return {}
    proc = run([*go_cmd(tool), "-over", "0", *files])
    return parse_go_output(proc.stdout, {})


def go_base_metrics(
    files: list[str], base: str, tool: str
) -> dict[tuple[str, str], int]:
    """从 base 版本取同路径文件写到临时目录,按函数名对比。"""
    if not files or not base:
        return {}
    with tempfile.TemporaryDirectory(prefix="hdd-complexity-") as tmp:
        temp_by_original: dict[str, str] = {}
        for index, file in enumerate(files):
            shown = git("show", f"{base}:{file}")
            if shown.returncode != 0:
                continue
            target = Path(tmp) / f"{index}-{Path(file).name}"
            target.write_text(shown.stdout, encoding="utf-8")
            temp_by_original[str(target.resolve())] = file
        proc = run([*go_cmd(tool), "-over", "0", *temp_by_original])
        return parse_go_output(proc.stdout, temp_by_original)


def python_cmd() -> list[str] | None:
    if shutil.which("uvx"):
        return ["uvx", "radon", "cc", "-j"]
    return None


def parse_radon(stdout: str, base_by_temp: dict[str, str]) -> dict[tuple[str, str], int]:
    try:
        data = json.loads(stdout)
    except json.JSONDecodeError:
        return {}
    out: dict[tuple[str, str], int] = {}
    for path, blocks in data.items():
        file = base_by_temp.get(str(Path(path).resolve()), relative_path(path))
        for block in blocks:
            if block.get("type") not in {"function", "method"}:
                continue
            name = block.get("name", "<anonymous>")
            if block.get("classname"):
                name = f"{block['classname']}.{name}"
            out[(file, name)] = int(block.get("complexity", 0))
    return out


def python_metrics(files: list[str]) -> dict[tuple[str, str], int]:
    cmd = python_cmd()
    if cmd is None or not files:
        return {}
    proc = run([*cmd, *files])
    return parse_radon(proc.stdout, {})


def python_base_metrics(
    files: list[str], base: str
) -> dict[tuple[str, str], int]:
    cmd = python_cmd()
    if cmd is None or not files or not base:
        return {}
    with tempfile.TemporaryDirectory(prefix="hdd-complexity-") as tmp:
        temp_by_original: dict[str, str] = {}
        for index, file in enumerate(files):
            shown = git("show", f"{base}:{file}")
            if shown.returncode != 0:
                continue
            target = Path(tmp) / f"{index}-{Path(file).name}"
            target.write_text(shown.stdout, encoding="utf-8")
            temp_by_original[str(target.resolve())] = file
        proc = run([*cmd, *temp_by_original])
        return parse_radon(proc.stdout, temp_by_original)


def typescript_metrics(files: list[str]) -> dict[tuple[str, str], int]:
    if not files or not (ROOT / "apps/mobile/node_modules").exists():
        return {}
    eslint = ROOT / "apps/mobile/node_modules/.bin/eslint"
    command = [str(eslint)] if eslint.exists() else ["npx", "eslint"]
    proc = run([
        *command, *files,
        "--no-cache",
        "--rule", '{"complexity":["warn",20]}',
        "--format", "json",
    ])
    try:
        data = json.loads(proc.stdout)
    except json.JSONDecodeError:
        return {}
    out: dict[tuple[str, str], int] = {}
    for entry in data:
        file = relative_path(entry.get("filePath", ""))
        for message in entry.get("messages", []):
            if message.get("ruleId") != "complexity":
                continue
            match = re.search(r"complexity of (\d+)", message.get("message", ""))
            if not match:
                continue
            label = re.search(r"Function '([^']+)'", message.get("message", ""))
            name = label.group(1) if label else f"<anonymous>@{message.get('line', 0)}"
            out[(file, name)] = int(match.group(1))
    return out


def collect(
    files: list[str], base: str, audit: bool
) -> list[Finding]:
    findings: list[Finding] = []
    go_files = [f for f in files if f.endswith(".go")]
    py_files = [f for f in files if f.endswith(".py")]
    ts_files = [f for f in files if f.endswith((".ts", ".tsx"))]

    for kind, tool, label in (
        ("go_cyclo", "gocyclo", "cyclo"),
        ("go_cognitive", "gocognit", "cognitive"),
    ):
        current = go_metrics(go_files, tool)
        if not current:
            continue
        previous = {} if audit else go_base_metrics(go_files, base, tool)
        for (file, name), value in sorted(current.items()):
            threshold = THRESHOLDS[kind]
            if value <= threshold:
                continue
            old = previous.get((file, name))
            status = "audit" if audit else ("new" if old is None else "increased")
            if not audit and old is not None and value <= old:
                continue
            findings.append(Finding("go", label, file, name, old, value, threshold, status))

    current = python_metrics(py_files)
    if current:
        previous = {} if audit else python_base_metrics(py_files, base)
        for (file, name), value in sorted(current.items()):
            threshold = THRESHOLDS["python"]
            if value <= threshold:
                continue
            old = previous.get((file, name))
            status = "audit" if audit else ("new" if old is None else "increased")
            if not audit and old is not None and value <= old:
                continue
            findings.append(Finding("python", "cyclo", file, name, old, value, threshold, status))

    # TS 没有稳定的函数 ID:只报修改文件里 >20 的函数,当前基线无 >20。
    for (file, name), value in sorted(typescript_metrics(ts_files).items()):
        threshold = THRESHOLDS["typescript"]
        if value > threshold:
            findings.append(Finding("typescript", "cyclo", file, name, None, value, threshold,
                                    "audit" if audit else "current"))
    return findings


def write_report(findings: list[Finding], mode: str, base: str, files: list[str]) -> Path:
    target = git_dir() / "hdd-complexity"
    target.mkdir(parents=True, exist_ok=True)
    md = target / "report.md"
    js = target / "report.json"
    lines = [
        "# Complexity signal",
        "",
        f"- mode: `{mode}`",
        f"- base: `{base or 'audit'}`",
        f"- files: {len(files)}",
        f"- findings: {len(findings)}",
        "",
    ]
    if findings:
        lines += ["| Lang | Kind | Function | File | Base | Current | Threshold | Status |",
                  "| --- | --- | --- | --- | ---: | ---: | ---: | --- |"]
        for f in findings:
            lines.append(
                f"| {f.language} | {f.kind} | `{f.function}` | `{f.file}` | "
                f"{f.base if f.base is not None else '—'} | {f.current} | {f.threshold} | {f.status} |"
            )
    else:
        lines.append("No new or increased functions above threshold.")
    md.write_text("\n".join(lines) + "\n", encoding="utf-8")
    js.write_text(json.dumps([asdict(f) for f in findings], ensure_ascii=False, indent=2),
                  encoding="utf-8")
    return md


def main() -> int:
    if os.environ.get("HDD_COMPLEXITY", "1") == "0":
        print("[complexity] skipped (HDD_COMPLEXITY=0)")
        return 0
    parser = argparse.ArgumentParser()
    group = parser.add_mutually_exclusive_group()
    group.add_argument("--changed", nargs="?", const="HEAD^", default=None)
    group.add_argument("--staged", action="store_true")
    group.add_argument("--push", action="store_true")
    group.add_argument("--audit", action="store_true")
    args = parser.parse_args()

    try:
        if args.audit:
            mode, base = "audit", ""
            files = tracked_files()
        elif args.staged:
            mode = "staged"
            base = "HEAD" if git("rev-parse", "--verify", "--quiet", "HEAD").returncode == 0 else ""
            files = staged_files()
        elif args.push:
            mode, base = "push", resolve_push_base()
            files = changed_files(base)
        else:
            mode, base = "changed", args.changed or "HEAD^"
            files = changed_files(base)
        if not files:
            print(f"[complexity] no changed source files since {base or 'N/A'}")
            return 0
        findings = collect(files, base, args.audit)
        report = write_report(findings, mode, base, files)
        print(f"[complexity] {mode} since {base or 'audit'}: {len(files)} file(s), "
              f"{len(findings)} finding(s)")
        for f in findings[:8]:
            before = "" if f.base is None else f" ({f.base}→{f.current})"
            print(f"  {f.language:10} {f.function:32} {f.file} {f.current}{before} [{f.status}]")
        if len(findings) > 8:
            print(f"  … {len(findings) - 8} more")
        print(f"[complexity] detail: {report}")
    except Exception as exc:  # noqa: BLE001 - hook must never block
        print(f"[complexity] scan skipped: {exc}")
    return 0


if __name__ == "__main__":
    sys.exit(main())
