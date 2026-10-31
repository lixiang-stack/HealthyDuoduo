"""端到端验证环境:共享 deploy 栈 + pg_dump 快照兜底,种子以 samples/ 为主。

用法(仓库根或 worktree 根执行):
  uv run --project recognizer python tools/e2e.py snapshot           # 真实库快照 → <主检出>/data/e2e-snapshots/
  uv run --project recognizer python tools/e2e.py restore --latest   # 回滚真实库到快照(需人工输入 yes)
  uv run --project recognizer python tools/e2e.py restore-check      # 快照恢复机制自证(临时库,不碰真实库)
  uv run --project recognizer python tools/e2e.py refresh --from <检出根>
      # 在 --from 检出:go build hdd → compose 重建 recognizer → 全量 reparse
      # (词典/规则迭代后的标准验证循环;reparse 复用已存 OCR,秒级)
  uv run --project recognizer python tools/e2e.py seed [--with-local] # ingest samples/ 图(--with-local 追加 data/ 本地图)
  uv run --project recognizer python tools/e2e.py clean              # 移除 seed 写入的行(S3 对象保留:惰性字节,重 ingest 幂等)
  uv run --project recognizer python tools/e2e.py verify [--item 血小板] [--golden]
      # 默认:真实数据不变量——单位 ∈ 注册规范形/族外原文、items/sections 的
      # ocr_lines 链接覆盖率与几何裕度(经 hdd serve /api/v1)、trend 质量门口径、
      # delta 只在可信点间
      # --golden:seed 后逐张对比 DB 报告 vs samples/expected/report(全链路
      # HTTP+OCR+DB 写入一致;items 逐字段 + 可选 sections 逐段)

标准验证序列(refresh → verify → seed → verify --golden → clean)。

设计取舍(2026-10-13 与维护者确认):
- 共享栈 + 快照:e2e 不另起端口/卷,直接用现网 deploy 项目;动手前 snapshot(),
  破坏性操作(clean/restore)按台账/快照执行,绝不静默重放。
- 种子 = samples/:脱敏语料可入库;data/ 真实图只作本机增强层(NF-01 不入库)。
- 快照/台账放 <主检出>/data/(gitignored,永不入库);S3 对象不快照(原图皆在本地)。

依赖:docker(compose 项目 deploy)、容器内 pg_dump/pg_restore、宿主 go(refresh 构建用)。
环境变量:E2E_PG_CONTAINER(默认 deploy-postgres-1);库名/账号沿 HDD_DATABASE_URL 默认值解析;
HDD_SERVE_PORT(默认 8080)决定 verify 访问 hdd serve 的端口。
"""

import argparse
import hashlib
import itertools
import json
import os
import re
import subprocess
import sys
import time
import urllib.error
import urllib.parse
import urllib.request
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parents[1]))

REPO_ROOT = Path(__file__).resolve().parents[2]
# 与 recognizer.golden 的占位约定一致(掩码语料真值日期不入库;不直接 import 以免引入运行时依赖)
MASKED_DATE = "2020-01-01"
IMAGE_SUFFIXES = (".jpg", ".jpeg", ".png", ".webp", ".bmp")
DEFAULT_CONTAINER = "deploy-postgres-1"
# hdd serve 端口缺省(compose 映射 HDD_SERVE_PORT,默认 8080);verify 的 ocr_lines 度量经它取详情。
DEFAULT_SERVE_PORT = "8080"


def main_data() -> Path:
    """本机 data 根(gitignored):worktree 下用 HDD_DATA_DIR 指向主检出(与 local_golden 同约定)。"""
    env = os.environ.get("HDD_DATA_DIR")
    if env:
        return Path(env)
    top = (
        subprocess.run(
            [
                "git",
                "-C",
                str(REPO_ROOT),
                "rev-parse",
                "--path-format=absolute",
                "--git-common-dir",
            ],
            capture_output=True,
            text=True,
            check=True,
        )
        .stdout.strip()
        .removesuffix("/.git")
    )
    return Path(top) / "data"


def pg_container() -> str:
    return os.environ.get("E2E_PG_CONTAINER", DEFAULT_CONTAINER)


def psql(sql: str, database: str | None = None) -> str:
    out = subprocess.run(
        [
            "docker",
            "exec",
            pg_container(),
            "psql",
            "-U",
            "healthyduoduo",
            "-d",
            database or db_name(),
            "-At",
            "-v",
            "ON_ERROR_STOP=1",
            "-c",
            sql,
        ],
        capture_output=True,
        text=True,
        check=False,
    )
    if out.returncode != 0:
        raise SystemExit(f"psql failed: {out.stderr.strip()}")
    return out.stdout


def db_url() -> urllib.parse.SplitResult:
    return urllib.parse.urlsplit(
        os.environ.get(
            "HDD_DATABASE_URL",
            "postgres://healthyduoduo:healthyduoduo@localhost:5432/healthyduoduo",
        )
    )


def db_name() -> str:
    return db_url().path.lstrip("/") or "healthyduoduo"


def build_hdd(from_root: Path) -> str:
    p = from_root / "hdd"
    if not p.exists():
        subprocess.run(
            ["go", "build", "-o", "hdd", "./cmd/hdd"], cwd=from_root, check=True
        )
    return str(p)


def image_sha(path: Path) -> str:
    h = hashlib.sha256()
    with path.open("rb") as f:
        for chunk in iter(lambda: f.read(1 << 20), b""):
            h.update(chunk)
    return h.hexdigest()


def existing_shas() -> set[str]:
    return {s for s in psql("select sha256 from images;").split() if s}


def report_shas() -> set[str]:
    """可 reparse 的报告身份(reports 每图一行);images 可能因半截 ingest 无报告。"""
    return {s for s in psql("select image_sha256 from reports;").split() if s}


def orphan_images() -> list[dict[str, str]]:
    """有 images 行、无 reports 行的半截 ingest;带图片/OCR 入库时间用于定位引入时间。"""
    q = (
        "select i.sha256 || chr(31) || replace(i.original_filename, chr(31), ' ') || chr(31) || "
        "to_char(i.created_at, 'YYYY-MM-DD HH24:MI:SSOF') || chr(31) || "
        "coalesce((select min(to_char(o.created_at, 'YYYY-MM-DD HH24:MI:SSOF'))"
        " from ocr_results o where o.image_sha256 = i.sha256), '') "
        "from images i where not exists ("
        "select 1 from reports r where r.image_sha256 = i.sha256) "
        "order by i.created_at;"
    )
    out: list[dict[str, str]] = []
    for line in psql(q).splitlines():
        parts = (line.split(chr(31)) + ["", "", "", ""])[:4]
        out.append(
            dict(zip(("sha", "filename", "image_created", "ocr_created"), parts))
        )
    return out


def verify_no_orphans() -> None:
    """孤儿图像门禁:images 有、reports 无 → 半截 ingest;报出文件名与入库时间。"""
    orphans = orphan_images()
    if orphans:
        lines = [
            f"  {o['sha'][:12]} {o['filename']} image_created={o['image_created']}"
            f" ocr_created={o['ocr_created'] or '-'}"
            for o in orphans
        ]
        raise SystemExit(
            "FAIL: orphan images (images without reports; half-ingested):\n"
            + "\n".join(lines)
        )
    print("ok: no orphan images")


def stats() -> dict[str, int]:
    q = (
        "select (select count(*) from images),(select count(*) from ocr_results),"
        "(select count(*) from reports),(select count(*) from report_items);"
    )
    vals = psql(q).strip().replace("|", " ").split()
    return dict(
        zip(("images", "ocr_results", "reports", "report_items"), map(int, vals))
    )


def sample_images() -> dict[str, Path]:
    """samples 内有原图的 golden id → 图路径(掩码语料 id 无原图,不可 ingest)。"""
    root = REPO_ROOT / "samples"
    ids = {
        p.stem for p in (root / "expected" / "ocr").glob("*.json") if p.stem != "README"
    }
    out = {}
    for sid in sorted(ids):
        for ext in IMAGE_SUFFIXES:
            f = root / f"{sid}{ext}"
            if f.exists():
                out[sid] = f
                break
    return out


# ---------- 快照/恢复 ----------


def snapshot(label: str | None = None) -> Path:
    d = main_data() / "e2e-snapshots"
    d.mkdir(parents=True, exist_ok=True)
    p = d / f"{label or db_name()}-{time.strftime('%Y%m%d-%H%M%S')}.dump"
    with p.open("wb") as f:
        subprocess.run(
            [
                "docker",
                "exec",
                pg_container(),
                "pg_dump",
                "-U",
                "healthyduoduo",
                "-Fc",
                db_name(),
            ],
            stdout=f,
            check=True,
        )
    if p.stat().st_size == 0:
        raise SystemExit(f"snapshot empty: {p}")
    print(f"snapshot -> {p} ({p.stat().st_size} bytes)")
    return p


def latest_snapshot() -> Path:
    snaps = sorted((main_data() / "e2e-snapshots").glob("*.dump"))
    if not snaps:
        raise SystemExit("no snapshots yet(先 snapshot)")
    return snaps[-1]


def restore(file: Path | None = None) -> None:
    """破坏性:把真实库替换成快照内容(pg_restore --clean);需人工输入 yes。"""
    f = file or latest_snapshot()
    if (
        input(f"restore {db_name()} from {f.name}?(原数据被快照替换)输入 yes 确认: ")
        != "yes"
    ):
        print("aborted")
        return
    with f.open("rb") as src:
        subprocess.run(
            [
                "docker",
                "exec",
                "-i",
                pg_container(),
                "pg_restore",
                "--clean",
                "--if-exists",
                "-U",
                "healthyduoduo",
                "-d",
                db_name(),
            ],
            stdin=src,
            check=True,
        )
    print(f"restored; stats = {stats()}")


def restore_check() -> None:
    """恢复机制自证:最新快照恢复到临时库核数后丢弃,不触碰真实库。"""
    f = latest_snapshot()
    tmp = f"{db_name()}_e2e_check"

    def ddl(sql: str) -> None:
        subprocess.run(
            [
                "docker",
                "exec",
                pg_container(),
                "psql",
                "-U",
                "healthyduoduo",
                "-d",
                "postgres",
                "-c",
                sql,
            ],
            check=True,
            stdout=subprocess.DEVNULL,
        )

    ddl(f"drop database if exists {tmp}")
    ddl(f"create database {tmp}")
    with f.open("rb") as src:
        subprocess.run(
            [
                "docker",
                "exec",
                "-i",
                pg_container(),
                "pg_restore",
                "-U",
                "healthyduoduo",
                "-d",
                tmp,
            ],
            stdin=src,
            check=True,
            stdout=subprocess.DEVNULL,
        )
    got = (
        psql(
            "select (select count(*) from images),(select count(*) from reports);",
            database=tmp,
        )
        .strip()
        .replace("|", " ")
        .split()
    )
    subprocess.run(
        [
            "docker",
            "exec",
            pg_container(),
            "psql",
            "-U",
            "healthyduoduo",
            "-d",
            "postgres",
            "-c",
            f"drop database {tmp};",
        ],
        check=True,
        stdout=subprocess.DEVNULL,
    )
    real = stats()
    if (int(got[0]),) != (real["images"],) or int(got[1]) != real["reports"]:
        raise SystemExit(f"FAIL: restore-check counts differ: tmp={got}, real={real}")
    print(
        f"restore-check ok: tmp counts {got} == real {real}'s images/reports, tmp dropped"
    )


# ---------- 种子/清理 ----------


def seed_log() -> Path:
    p = main_data() / "e2e" / "seeded-shas.txt"
    p.parent.mkdir(parents=True, exist_ok=True)
    return p


def seed(with_local: bool) -> None:
    imgs = list(sample_images().items())
    if with_local:
        imgs += [(p.stem, p) for p in sorted(main_data().glob("*.jpeg")) if p.is_file()]
    have = existing_shas()
    log = seed_log()
    logged = set(log.read_text().split()) if log.exists() else set()
    added = 0
    for sid, f in imgs:
        sha = image_sha(f)
        if sha in have or sha in logged:
            print(f"seed {sid}: cached {sha[:12]}")
            continue
        # 日期兜底:golden 已解析出的日期;无日期 golden 用 MASKED_DATE 占位(真实日期不入库)
        g = REPO_ROOT / "samples" / "expected" / "report" / f"{sid}.json"
        date = (
            json.loads(g.read_text()).get("report_date") if g.exists() else None
        ) or MASKED_DATE
        r = subprocess.run(
            [build_hdd(REPO_ROOT), "ingest", str(f), "--date", date],
            capture_output=True,
            text=True,
            check=False,
        )
        if r.returncode != 0:
            raise SystemExit(f"ingest {f.name}: {r.stderr.strip()}")
        added += 1
        logged.add(sha)
        log.write_text(" ".join(sorted(logged)) + " ")
        print(f"seed {sid}: {sha[:12]} date={date}")
    print(f"seed done: {added} new, log = {log}")


def clean() -> None:
    """破坏性:只删 seed 台账上登记的 sha(FK 顺序:items → reports → ocr → images)。

    samples/ 图与真实数据同源时,台账必与真实行重合,务必人工确认;需输入 yes。
    """
    log = seed_log()
    shas = [
        s for s in (log.read_text().split() if log.exists() else []) if len(s) == 64
    ]
    if not shas:
        print("clean: no seeded rows recorded")
        return
    if input(f"delete {len(shas)} seeded row(s) from db? 输入 yes 确认: ") != "yes":
        print("aborted")
        return
    before = stats()
    lst = ",".join(f"'{s}'" for s in shas)
    psql(f"delete from report_items where report_sha256 in ({lst});")
    psql(f"delete from reports where image_sha256 in ({lst});")
    psql(f"delete from ocr_results where image_sha256 in ({lst});")
    psql(f"delete from images where sha256 in ({lst});")
    log.unlink()
    print(f"clean: {before} -> {stats()}")


# ---------- refresh(重建 served 代码 + 全量 reparse) ----------


def refresh(from_root: Path) -> None:
    """--from 检出的代码成为 served recognizer;其 hdd 二进制负责批量 reparse。

    共享栈约定:同一时刻只有一个检出的代码在 serve;验证另一个检出时再次 refresh 切换。
    """
    subprocess.run(["go", "build", "-o", "hdd", "./cmd/hdd"], cwd=from_root, check=True)
    compose = from_root / "deploy" / "docker-compose.yml"
    for cmd in (
        [
            "docker",
            "compose",
            "-p",
            "deploy",
            "-f",
            str(compose),
            "build",
            "recognizer",
        ],
        [
            "docker",
            "compose",
            "-p",
            "deploy",
            "-f",
            str(compose),
            "up",
            "-d",
            "--force-recreate",
            "recognizer",
        ],
    ):
        subprocess.run(cmd, cwd=from_root, check=True)
    wait_healthz()
    orphans = orphan_images()
    if orphans:
        print(f"warn: {len(orphans)} orphan image(s) without report (reparse skipped):")
        for o in orphans:
            print(
                f"  {o['sha'][:12]} {o['filename']} image_created={o['image_created']}"
                f" ocr_created={o['ocr_created'] or '-'}"
            )
    done_in_round = reparse_batch(from_root, sorted(report_shas()))
    print(f"refresh ok ({done_in_round} reparses)")


def wait_healthz(timeout_s: int = 30) -> None:
    """--force-recreate 后等 /healthz 就绪,再发批量请求。"""
    host = db_url().hostname or "localhost"
    port = 8000
    env_port = os.environ.get("RECOGNIZER_PORT")
    if env_port and env_port.isdigit():
        port = int(env_port)
    import time as _time

    deadline = _time.time() + timeout_s
    while _time.time() < deadline:
        r = subprocess.run(
            ["curl", "-sf", "-m", "2", f"http://{host}:{port}/healthz"],
            capture_output=True,
            check=False,
        )
        if r.returncode == 0:
            return
        _time.sleep(1)
    raise SystemExit(f"refresh: recognizer /healthz not ready within {timeout_s}s")


def reparse_batch(from_root: Path, shas: list[str], rounds: int = 3) -> int:
    """批量 reparse;--force-recreate 切换后的瞬时 EOF 等竞态以点名重试吸收。"""
    ok: set[str] = set()
    for round_no in range(1, rounds + 1):
        todo = [s for s in shas if s not in ok]
        if not todo:
            break
        r = subprocess.run(
            [str(from_root / "hdd"), "reparse", *todo],
            capture_output=True,
            text=True,
            check=False,
        )
        print(r.stdout, end="")
        ok |= {
            m.group(1)
            for m in re.finditer(r"reparse ([0-9a-f]{12}): .* \(\d+ items\)", r.stdout)
        }
        print(r.stderr, end="")
    missed = [s[:12] for s in shas if s[:12] not in ok]
    if missed:
        raise SystemExit(
            f"refresh: {len(missed)} sha(s) still failing after {rounds} rounds: {missed}"
        )
    return len(shas)


# ---------- 验证 ----------


def _check(cond: bool, msg: str) -> None:
    if not cond:
        raise SystemExit(f"FAIL: {msg}")
    print(f"ok: {msg}")


# ---- ocr_lines 链接度量(票 02) ----


def _serve_base() -> str:
    """hdd serve 的 /api/v1 基座(verify 的 ocr_lines 度量经 HTTP 取详情)。"""
    return f"http://127.0.0.1:{os.environ.get('HDD_SERVE_PORT', DEFAULT_SERVE_PORT)}/api/v1"


def _serve_json(path: str) -> dict:
    with urllib.request.urlopen(_serve_base() + path, timeout=15) as resp:
        return json.loads(resp.read())


def _check_ocr_box(sha: str, label: str, link: dict, boxes: list) -> None:
    """行链接框必须逐点等于该行 OCR box(结构与坐标同源)。"""
    if link["box"] != boxes[link["line"]]:
        raise SystemExit(f"FAIL: {sha[:12]} {label} line {link['line']} box 与 OCR box 不一致")


def verify_ocr_lines() -> None:
    """ocr_lines 行链接持续度量(GET /reports/{id} 契约;票 07 引入、票 02 收编为门禁)。

    · 覆盖率与结构一致性(硬门禁):每份报告的每个指标项必须有行链接,且
      「链接行文本按序 join == raw_text」、链接框逐点等于该行 OCR box——
      结构错位或识别端 raw_text join 格式漂移在此 fail-fast。
    · 报告段落(硬门禁):HTTP 段落数必须等于 DB reports.sections 基数(防旧 serve
      静默省略),每段必须有行链接,且链接行文本逐行与 text 一致(首行允许锚词/
      标签前缀)、链接框逐点等于该行 OCR box(票 02 收编)。
    · 歧义度(度量):有链接行文本在单内多次出现的项数——同文本行的绑定歧义面;
      v2 前由服务端几何消歧消化,v2 后由识别端直吐下标,此处保留为持续观测(>0 属常态)。
    · 歧义裕度(度量):链接行中心 y 的展开 / 最大行高——同一视觉行内应远小于 1,
      跨行错配才会 >1(纵向回退项为合法跨行,故只度量告警、不硬判)。
      真正的最优解-备选解差值只在识别/服务端内部可见,无法经契约暴露,故以
      「所选解的几何聚拢度」为外部代理;>1 打印告警供人工抽查。
    """
    section_counts: dict[str, int] = {}
    for line in psql(
        "select image_sha256, coalesce(jsonb_array_length(sections),0)"
        " from reports order by image_sha256;"
    ).splitlines():
        if line:
            sha, count = line.split("|")
            section_counts[sha] = int(count)
    shas = list(section_counts)
    total_items = linked_items = ambiguous = 0
    total_sections = linked_sections = 0
    worst_ratio, worst = 0.0, "-"
    for sha in shas:
        try:
            detail = _serve_json(f"/reports/{sha}")
        except urllib.error.URLError as exc:
            raise SystemExit(
                f"FAIL: ocr-lines: hdd serve 不可达({exc});"
                "该度量依赖已构建的 serve 容器与 /api/v1/reports/{id}"
            )
        engine = psql(
            "select engine_output from ocr_results where id="
            f"(select ocr_result_id from reports where image_sha256='{sha}');"
        )
        ocr = json.loads(engine)
        txts = [t.strip() for t in ocr["txts"]]
        boxes = ocr["boxes"]
        for item in detail["items"]:
            total_items += 1
            links = item["ocr_lines"]
            if not links:
                raise SystemExit(
                    f"FAIL: {sha[:12]} item {item['name']!r} 无 ocr_lines(raw_text={item['raw_text']!r})"
                )
            linked_items += 1
            # 格间分隔符按识别端 postprocess 的 raw_text 口径(两个空格);跨语言不变量,
            # 若识别端改 join 格式,此处应同步(结构门禁会先在此炸出)。
            joined = "  ".join(txts[l["line"]] for l in links)
            if joined != item["raw_text"]:
                raise SystemExit(
                    f"FAIL: {sha[:12]} item {item['name']!r} 链接行 join 与 raw_text 不一致:"
                    f" {joined!r} != {item['raw_text']!r}"
                )
            for l in links:
                _check_ocr_box(sha, f"item {item['name']!r}", l, boxes)
            linked_texts = [txts[l["line"]] for l in links]
            if any(txts.count(t) > 1 for t in linked_texts):
                ambiguous += 1
            cys = [sum(p[1] for p in boxes[l["line"]]) / 4 for l in links]
            heights = [
                max(p[1] for p in boxes[l["line"]]) - min(p[1] for p in boxes[l["line"]])
                for l in links
            ]
            ratio = (max(cys) - min(cys)) / max(max(heights), 1.0)
            if ratio > worst_ratio:
                worst_ratio, worst = ratio, f"{sha[:12]}/{item['name']}"
        # 报告段落(票 02):HTTP 段落数必须与 DB 一致(旧 serve 省略 sections 时
        # 不能静默通过);段落文本 = OCR 行以换行拼接,首行可含锚词/标签前缀。
        served_sections = detail.get("sections") or []
        if len(served_sections) != section_counts[sha]:
            raise SystemExit(
                f"FAIL: {sha[:12]} HTTP sections {len(served_sections)}"
                f" != db {section_counts[sha]} (hdd serve 是否已重建?)"
            )
        for section in served_sections:
            total_sections += 1
            links = section["ocr_lines"]
            if not links:
                raise SystemExit(
                    f"FAIL: {sha[:12]} section {section['kind']!r} 无 ocr_lines(text={section['text']!r})"
                )
            text_lines = section["text"].split("\n")
            if len(text_lines) != len(links):
                raise SystemExit(
                    f"FAIL: {sha[:12]} section {section['kind']!r} 行数不一致:"
                    f" text={len(text_lines)} links={len(links)}"
                )
            for pos, (text_line, l) in enumerate(zip(text_lines, links)):
                ocr_line = txts[l["line"]]
                matched = ocr_line.endswith(text_line) if pos == 0 else ocr_line == text_line
                if not matched:
                    raise SystemExit(
                        f"FAIL: {sha[:12]} section {section['kind']!r} line {l['line']} 与 text 不一致:"
                        f" {ocr_line!r} != {text_line!r}"
                    )
                _check_ocr_box(sha, f"section {section['kind']!r}", l, boxes)
            linked_sections += 1
    # 覆盖率即"无链接项"已在上文逐项 raise;此处只作摘要输出(结构门禁的失败路径都是 raise)。
    print(
        f"ok: ocr-lines: linked {linked_items}/{total_items} items,"
        f" {linked_sections}/{total_sections} sections; join/box 一致"
    )
    margin_msg = (
        f"ocr-lines margin: ambiguous-resolution {ambiguous}/{total_items},"
        f" max y-spread ratio {worst_ratio:.2f} (worst {worst})"
    )
    if worst_ratio > 1.0:
        print(f"warn: {margin_msg} —— 跨行嫌疑,抽查上列项")
    else:
        print(f"ok: {margin_msg}")


def verify_narrative_reports(body_kinds: tuple[str, ...]) -> None:
    """叙述型报告(有 sections)硬门禁(票 04):零指标项,status 与正文段落一致。"""
    kinds_sql = ", ".join("'" + kind.replace("'", "''") + "'" for kind in body_kinds)
    rows = json.loads(
        psql(
            "select coalesce(json_agg(row_to_json(t)),'[]') from ("
            " select r.image_sha256 sha, r.status,"
            "   (select count(*) from report_items i"
            "    where i.report_sha256 = r.image_sha256) item_count,"
            "   exists(select 1 from jsonb_array_elements(r.sections) e"
            f"          where e->>'kind' in ({kinds_sql})) has_body"
            " from reports r where r.sections is not null) t;"
        ).strip()
    )
    for row in rows:
        sha = row["sha"]
        if row["item_count"] != 0:
            raise SystemExit(
                f"FAIL: {sha[:12]} narrative report has {row['item_count']} items, want 0"
            )
        want = "success" if row["has_body"] else "failed"
        if row["status"] != want:
            raise SystemExit(
                f"FAIL: {sha[:12]} narrative status={row['status']}, want {want}"
                f" (body={row['has_body']})"
            )
    print(f"ok: narrative reports: {len(rows)} zero-item, status matches body")


def _trend_rows(item: str, show_all: bool, from_root: Path) -> list[dict]:
    args = [build_hdd(from_root), "trend"] + (["--all"] if show_all else []) + [item]
    r = subprocess.run(args, capture_output=True, text=True, check=False)
    if r.returncode != 0:
        raise SystemExit(f"trend {item}: {r.stderr.strip()}")
    out = []
    for line in r.stdout.splitlines()[1:]:
        cols = line.split()
        if len(cols) >= 7:
            out.append(
                dict(
                    zip(
                        ("date", "type", "value", "unit", "delta", "flag", "sha"),
                        cols[:7],
                    )
                )
                | {"note": cols[7] if len(cols) > 7 else ""}
            )
    return out


def verify_unit_status_audit() -> None:
    """未验证单位点数量与原因可审计(票 12/13):status + 空 unit + 低置信计数。"""
    rows = psql(
        "select string_agg(status || '=' || n || ' (unit=null ' || nulls || ', low_confidence ' || low || ')', '; ') "
        "from (select coalesce(unit_status,'none') as status, count(*) as n, "
        "count(*) filter (where unit is null) as nulls, "
        "count(*) filter (where low_confidence) as low "
        "from report_items group by 1 order by 1) a;"
    ).strip()
    bad = psql(
        "select count(*) from report_items "
        "where unit_status is not null and unit_status not in ('verified','unverified');"
    ).strip()
    _check(bad == "0", f"unit_status audit: {rows.replace(chr(10), '; ')}; invalid={bad}")


def verify(item: str, from_root: Path) -> None:
    """真实数据不变量(无需 seed;refresh 之后的收口检查)。"""
    verify_no_orphans()
    verify_unit_status_audit()
    # 词典/单位家族语义跟随被测检出(--from)加载,避免吞并进 harness 自身的旧规则
    sys.path.insert(0, str(Path(from_root) / "recognizer"))
    import importlib

    importlib.invalidate_caches()
    from recognizer.postprocess import (
        BODY_SECTION_KINDS,
        load_dicts,
        load_unit_forms,
    )

    families = (
        load_unit_forms()
    )  # 表内含真实单位等价与 OCR/印刷损耗形,归一目标见 unit_conversions.yaml / ocr_forms.yaml
    registered = {
        it.name: {u.lower() for u in it.units} for d in load_dicts() for it in d.items
    }
    stale = []
    misprint = []
    for line in psql(
        "select name || chr(31) || coalesce(unit,'') || chr(31) || count(*)"
        " from report_items group by name, coalesce(unit,'') order by name;"
    ).splitlines():
        parts = line.split(chr(31))
        name, unit = (parts + ["", ""])[:2]
        if not unit:
            continue
        reg = registered.get(name, set())
        cands = families.get(unit.lower(), ())
        if unit.lower() in reg:
            continue  # 本身即注册印形(归一化可原地映射,如 fl→fL 同串)
        if any(c.lower() in reg for c in cands):
            stale.append(
                (name, unit)
            )  # 未注册损耗形但族 canonical 已注册 → 归一化该生效而未生效
        elif cands:
            misprint.append((name, unit))  # 源单印刷串列(词典未注册该族)→ 保留原文,警告
    _check(
        not stale,
        f"units invariant: normalization applied where registered (stale={stale[:5]})",
    )
    if misprint:
        print(
            f"warn: printed-misprint units kept raw (source sheet garble): {misprint[:5]}"
        )

    verify_ocr_lines()
    verify_narrative_reports(BODY_SECTION_KINDS)

    rows_def = _trend_rows(item, False, from_root)
    rows_all = _trend_rows(item, True, from_root)
    hidden = [r for r in rows_all if r["sha"] not in {x["sha"] for x in rows_def}]
    for r in hidden:
        f, lc, us = (
            psql(
                f"select flag, low_confidence, coalesce(unit_status,'') from report_items where report_sha256 like '{r['sha']}%' and name = '{item}';"
            )
            .strip()
            .replace("|", " ")
            .split()
        )
        if f != "unknown" and lc != "t" and us != "unverified":
            raise SystemExit(
                f"FAIL: {r['sha'][:12]} hidden but not low-quality/unverified "
                f"(flag={f}, low_confidence={lc}, unit_status={us})"
            )
    unverified_hidden = sum("unverified_unit" in r["note"] for r in hidden)
    _check(
        True,
        f"trend gate: hidden {len(hidden)}/{len(rows_all)} all justified "
        f"(unverified={unverified_hidden}, item={item})",
    )

    for prev, cur in itertools.pairwise(rows_def):
        if cur["delta"] != "-" and prev["unit"] != cur["unit"]:
            raise SystemExit(
                f"FAIL: delta {cur['delta']} across units {prev['unit']}/{cur['unit']}"
            )
    computed = sum(r["delta"] != "-" for r in rows_def)
    _check(
        True,
        f"delta only within same unit ({computed}/{len(rows_def)} computed, item={item})",
    )


def verify_golden(from_base: Path) -> None:
    """--golden:逐张比对 DB 指标项 / 报告段落 vs <from_base>/samples/expected/report(全链路一致)。

    report_date/status 会因 ingest --date 补录而与 golden(纯解析)合法偏离 → 警告不判红;
    items(解析产物)要求逐字段一致;sections 可选,存在时逐段逐字段一致。
    """
    for sid, f in sample_images().items():
        golden = json.loads(
            (from_base / "samples" / "expected" / "report" / f"{sid}.json").read_text()
        )
        sha = image_sha(f)
        hdr = json.loads(
            psql(
                "select coalesce(json_agg(row_to_json(t)),'null') from ("
                " select report_type, status, to_char(report_date,'YYYY-MM-DD') report_date,"
                " sections from reports"
                f" where image_sha256 = '{sha}') t;"
            ).strip()
        )
        if not hdr:
            # json_agg 无行时返回 []（不是字符串 "null"）；旧判定会让 hdr[0] 直接 TypeError。
            raise SystemExit(f"FAIL: {sid} not in db (sha={sha[:12]}); run `e2e.py seed` first")
        items = json.loads(
            psql(
                "select coalesce(json_agg(row_to_json(t)), '[]') from ("
                " select name, value, unit, ref_range, method, flag, low_confidence, raw_text"
                " from report_items"
                f" where report_sha256 = '{sha}' order by id) t;"
            ).strip()
        )
        got = {**hdr[0], "items": items}
        _check(
            got["report_type"] == golden["report_type"],
            f"{sid}: report_type={got['report_type']}",
        )
        gitems, dbitems = golden["items"], got["items"] or []
        _check(
            len(dbitems) == len(gitems),
            f"{sid}: {len(dbitems)} items == golden {len(gitems)}",
        )
        for field in (
            "name",
            "value",
            "unit",
            "ref_range",
            "method",
            "flag",
            "low_confidence",
            "raw_text",
        ):
            g = [i[field] for i in gitems]
            db = [i[field] for i in dbitems]
            if g != db:
                bad_at = next(k for k, (x, y) in enumerate(zip(g, db)) if x != y)
                raise SystemExit(
                    f"FAIL: {sid} field '{field}' at #{bad_at}:\n"
                    f"  golden={json.dumps(g[bad_at], ensure_ascii=False)}\n"
                    f"  db    ={json.dumps(db[bad_at], ensure_ascii=False)}"
                )
        _check(True, f"{sid}: {len(gitems)} items identical to golden")
        # 报告段落(票 02):非影像报告 golden 无该键,DB 必须为 NULL;影像报告逐段
        # 逐字段比对 kind/text/ocr_line_numbers(行号顺序即文本组成顺序)。
        gsections, dbsections = golden.get("sections"), got.get("sections")
        if gsections is None:
            _check(dbsections is None, f"{sid}: non-narrative sections absent")
        else:
            dbsections = dbsections or []
            _check(
                len(dbsections) == len(gsections),
                f"{sid}: {len(dbsections)} sections == golden {len(gsections)}",
            )
            for pos, (g, db) in enumerate(zip(gsections, dbsections)):
                for field in ("kind", "text", "ocr_line_numbers"):
                    if g.get(field) != db.get(field):
                        raise SystemExit(
                            f"FAIL: {sid} section #{pos} field '{field}':\n"
                            f"  golden={json.dumps(g.get(field), ensure_ascii=False)}\n"
                            f"  db    ={json.dumps(db.get(field), ensure_ascii=False)}"
                        )
            _check(True, f"{sid}: {len(gsections)} sections identical to golden")
        if got["report_date"] != golden["report_date"]:
            print(
                f"warn: {sid} date/status diverge from golden (ingest --date 补录或 reparse 滞后):"
                f" db={got['report_date']}/{got['status']} golden={golden['report_date']}/{golden['status']}"
            )
    print("golden verify pass")


def main() -> int:
    ap = argparse.ArgumentParser()
    sub = ap.add_subparsers(dest="cmd", required=True)
    sp = sub.add_parser("snapshot")
    sp.add_argument("-o", "--output", default=None)
    sub.add_parser("restore").add_argument("--latest", action="store_true")
    sub.add_parser("restore-check")
    rp = sub.add_parser("refresh")
    rp.add_argument("--from", required=True)
    sub.add_parser("seed").add_argument("--with-local", action="store_true")
    sub.add_parser("clean")
    v = sub.add_parser("verify")
    v.add_argument("--item", default="血小板")
    v.add_argument("--golden", action="store_true")
    v.add_argument("--from", default=None)
    a = ap.parse_args()
    base = Path(a.__dict__["from"]) if a.__dict__.get("from") else REPO_ROOT
    if a.cmd == "snapshot":
        snapshot(a.output)
    elif a.cmd == "restore":
        restore(latest_snapshot() if a.latest else None)
    elif a.cmd == "restore-check":
        restore_check()
    elif a.cmd == "refresh":
        refresh(base)
    elif a.cmd == "seed":
        seed(a.with_local)
    elif a.cmd == "clean":
        clean()
    elif a.cmd == "verify":
        verify_golden(base) if a.golden else verify(a.item, base)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
