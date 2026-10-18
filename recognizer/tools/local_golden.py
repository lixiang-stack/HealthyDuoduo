"""本地真实数据 golden:未脱敏原图只在本机迭代,永不提交(NF-01)。

仓库只收脱敏图像(samples/);这批 data/ 下的未脱敏原图与其完整解析产物一律不入库。
本工具把本机 data/ 冻结成 golden,供改词典 / 改解析规则后做回归对比与效果度量:

  freeze   图 → data/expected/ocr/<id>.json(OCR)+ data/expected/report/<id>.json(后处理)
  check    用冻结 OCR 重跑后处理,与 report golden 逐项 diff;--ocr 连 OCR 漂移一起比
           (elapse/elapse_list 计时字段不参与,同 tests/test_ocr_golden 口径)
  score    对 data/expected/vl/<id>.json 算 matched / value / unit / ref 效果指标
  mask     data/expected/ocr → data/masked/ocr 脱敏 OCR(身份/机构/时间置 ◆,供提交复核)

数据根见 paths.DATA:默认 <repo>/data,worktree / 多检出用环境变量 HDD_DATA_DIR 指向
主检出的 data。用法(仓库根或 worktree 根执行):

  cd recognizer && HDD_DATA_DIR=<主检出>/data uv run python tools/local_golden.py freeze
  cd recognizer && HDD_DATA_DIR=<主检出>/data uv run python tools/local_golden.py check [--ocr]
  cd recognizer && HDD_DATA_DIR=<主检出>/data uv run python tools/local_golden.py score
"""

import argparse
import difflib
import json
import re
import sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parents[1]))
sys.path.insert(0, str(Path(__file__).resolve().parent))

from gt_eval import _score

from recognizer import paths
from recognizer import postprocess as pp
from recognizer.contract import OCRResult
from recognizer.ocr import run_ocr
from recognizer.postprocess import load_dicts, run_postprocess

IMAGE_SUFFIXES = {".jpeg", ".jpg", ".png", ".webp", ".bmp"}

# 脱敏:落到「表格主体」(首条表头带顶线 ≥ y0)且不含身份/机构/时间信息的格保留,
# 其余一律置为占位符 ◆。表头行本身保留(列角色关键词是解析契约)。
_FOOTER_RE = re.compile(
    r"备注|检验者|审核者|送检|采集时间|接收时间|检验时间|报告时间|打印|声明|地址|联系电话|"
    r"电话|实验室|注[:：]|本结果|本报告|医师|签字|签名|回执"
)
_PII_RE = re.compile(
    r"姓名|性别|年龄|出生|末次月经|登记号|病人ID|门诊号|病员号|标签ID|样本号|样本编号|样本条码|"
    r"标本编号|标本类型|条码号|住院号|床号|科室|科别|病区|床位|临床诊断|送检医生|申请医生|送检者|"
    r"检验仪器|医院|妇幼|保健|卫生服务|服务中心|检验中心|检验报告单|医生|电话|地址|路\s*\d"
)
_DATE_RE = re.compile(r"\d{4}\s*[-/.年]\s*\d{1,2}|\d{1,2}\s*[:：]\s*\d{2}")

_PLACEHOLDER = "◆"
# 复核用:保留行里 2~3 字的纯中文短词(多为疑似姓名,需人工确认)
_SUSPECT_RE = re.compile(r"[\u4e00-\u9fa5]{2,3}")


def _sid(image: Path) -> str:
    """文件名 → 稳定样本 id(去掉空格/括号等文件名不友好字符)。"""
    return re.sub(r"[^0-9A-Za-z_]+", "_", image.stem).strip("_")


def _images() -> list[Path]:
    return sorted(p for p in paths.DATA.iterdir() if p.suffix.lower() in IMAGE_SUFFIXES)


def _dump(path: Path, payload: dict) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(json.dumps(payload, ensure_ascii=False) + "\n", encoding="utf-8")


def _json_lines(payload: dict) -> list[str]:
    return json.dumps(payload, ensure_ascii=False, indent=2, sort_keys=True).splitlines()


def _diff(label: str, want: dict, got: dict) -> None:
    print(f"\n[{label}] drift")
    for line in difflib.unified_diff(
        _json_lines(want), _json_lines(got), "golden", "actual", lineterm=""
    ):
        print(line)


def freeze() -> int:
    for image in _images():
        sid = _sid(image)
        ocr = run_ocr(image)
        _dump(paths.LOCAL_EXPECTED_OCR / f"{sid}.json", json.loads(ocr.model_dump_json()))
        report = run_postprocess(ocr)
        _dump(paths.LOCAL_EXPECTED_REPORT / f"{sid}.json", json.loads(report.model_dump_json()))
        print(f"{sid:28} {report.report_type:8} {report.status.value:7} items={len(report.items):2}")
    return 0


def _ocr_comparable(payload: dict) -> dict:
    """去除 OCR 产物中的计时字段(elapse/elapse_list 随机器/负载波动,不属内容漂移;
    tests/test_ocr_golden 的 P1 门禁同样只记录不比对)。"""
    return {k: v for k, v in payload.items() if k not in ("elapse", "elapse_list")}


def check(check_ocr: bool) -> int:
    changed = 0
    for ocr_path in sorted(paths.LOCAL_EXPECTED_OCR.glob("*.json")):
        sid = ocr_path.stem
        if check_ocr:
            image = next((p for p in _images() if _sid(p) == sid), None)
            if image is not None:
                want = _ocr_comparable(json.loads(ocr_path.read_text(encoding="utf-8")))
                got = _ocr_comparable(json.loads(run_ocr(image).model_dump_json()))
                if got != want:
                    changed += 1
                    _diff("OCR", want, got)
        report = run_postprocess(OCRResult.model_validate(json.loads(ocr_path.read_text(encoding="utf-8"))))
        report_path = paths.LOCAL_EXPECTED_REPORT / f"{sid}.json"
        if not report_path.exists():
            print(f"[report] {sid} missing golden {report_path}")
            changed += 1
            continue
        got = json.loads(report.model_dump_json())
        want = json.loads(report_path.read_text(encoding="utf-8"))
        if got != want:
            changed += 1
            _diff("report", want, got)
    total = len(list(paths.LOCAL_EXPECTED_OCR.glob("*.json")))
    print(f"\ncheck: {total - changed}/{total} unchanged, {changed} drifted")
    return 1 if changed else 0


def _redacted(txts: list[str]) -> list[str]:
    return [t for t in txts if t != _PLACEHOLDER]


# 掩码期「落在表列 x 区间」判据:与 postprocess 的列归属语义解耦——解析侧已改为中点分区
# (无固定偏离上限),而掩码只需保守判断「格是否算表列内解析依据」;沿用历次语料冻结口径,
# 避免脱敏产物整体漂移(词典别名与列角色关键词命中的格另有独立保留路径,长名称不受此限)。
_MASK_COLUMN_TOLERANCE_PX = 180


def _keep_cell(
    c: pp._Cell,
    header_top: float,
    footer_top: float | None,
    centers: list[float],
    category_re: re.Pattern[str],
    alias_forms: set[str],
) -> bool:
    """保留:不含身份/时间,且(列角色关键词 / 类别标题词 / 词典别名 / 表格主体内的解析列格)。

    类别标题词(如「肝功十项」)落在表头上方,但不含身份信息且是分发依据,故保留;
    词典别名命中格(如产筛/糖耐量竖排版式的项目名格)是解析依据,同样保留。
    """
    if _PII_RE.search(c.text) or _DATE_RE.search(c.text) or _FOOTER_RE.search(c.text):
        return False
    if pp._role_keyword_count([c]) or category_re.search(c.text):
        return True
    if any(form in alias_forms for form in pp._name_forms(c.text)):
        return True
    if c.y0 < header_top or (footer_top is not None and c.y0 >= footer_top):
        return False
    return any(abs(c.cx - cx) <= _MASK_COLUMN_TOLERANCE_PX for cx in centers)


def mask() -> int:
    """本地 data/expected/ocr → data/masked/ocr 的脱敏 OCR(txts 身份/机构/时间置 ◆)。

    只保留「表格主体内、且属于解析依据(列角色关键词或落在列角色 x 区间)」的格,外加
    类别标题词与词典别名命中格(分发/解析依据、非身份信息);签名、地址、日期、机构等
    一律置 ◆。boxes/scores 原样,保证后处理可跑。打印保留行中的可疑短中文词供人工复核。
    """
    out_dir = paths.DATA / "masked" / "ocr"
    dicts = load_dicts()
    category_re = re.compile("|".join(kw for d in dicts for kw in d.title_keywords) or r"(?!)")
    alias_forms = {a for d in dicts for it in d.items for a in it.aliases}
    for ocr_path in sorted(paths.LOCAL_EXPECTED_OCR.glob("*.json")):
        ocr = OCRResult.model_validate(json.loads(ocr_path.read_text(encoding="utf-8")))
        cells = pp._cells(ocr)
        header_bands = pp._header_bands(cells)
        header_top = min((c.y0 for b in header_bands for c in b), default=None)
        if header_top is None:
            header_top = min((c.y0 for c in cells), default=0.0)
        footer_top = min(
            (c.y0 for c in cells if c.y0 > header_top and _FOOTER_RE.search(c.text)), default=None
        )
        centers = [cx for lst in pp._role_centers(header_bands).values() for cx, _ in lst]
        txts = [
            c.text if _keep_cell(c, header_top, footer_top, centers, category_re, alias_forms) else _PLACEHOLDER
            for c in cells
        ]
        _dump(out_dir / ocr_path.name, json.loads(ocr.model_copy(update={"txts": txts}).model_dump_json()))
        kept = _redacted(txts)
        suspicious = [t for t in kept if _SUSPECT_RE.fullmatch(t.strip())]
        print(f"{ocr_path.stem:28} kept={len(kept):3}/{len(txts):3} suspicious={suspicious}")
    return 0


def score() -> int:
    dicts = {d.report_type: d for d in load_dicts()}
    totals = {"n": 0, "matched": 0, "vok": 0, "uok": 0, "rok": 0}
    print(f"{'sample':28} {'type':8} {'m/n':>7} vok uok rok")
    for vl_path in sorted(paths.LOCAL_EXPECTED_VL.glob("*.json")):
        sid = vl_path.stem
        ocr_path = paths.LOCAL_EXPECTED_OCR / f"{sid}.json"
        if not ocr_path.exists():
            print(f"{sid:28} (no frozen OCR)")
            continue
        gt = json.loads(vl_path.read_text(encoding="utf-8"))
        report = run_postprocess(OCRResult.model_validate(json.loads(ocr_path.read_text(encoding="utf-8"))))
        preds = [
            {"name": it.name, "value": it.value, "unit": it.unit, "ref": it.ref_range, "raw": it.raw_text}
            for it in report.items
        ]
        s = _score(gt, preds, dicts.get(report.report_type))
        for key in totals:
            totals[key] += s[key]
        print(f"{sid:28} {report.report_type:8} {s['matched']:>3}/{s['n']:<3} {s['vok']:>3} {s['uok']:>3} {s['rok']:>3}")
    print("\n=== totals ===")
    print(f"matched {totals['matched']}/{totals['n']}  value {totals['vok']}  unit {totals['uok']}  ref {totals['rok']}")
    return 0


def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("command", choices=("freeze", "check", "score", "mask"))
    parser.add_argument("--ocr", action="store_true", help="check 时连 OCR 结果一起比对")
    args = parser.parse_args(argv)
    if args.command == "freeze":
        return freeze()
    if args.command == "check":
        return check(args.ocr)
    if args.command == "mask":
        return mask()
    return score()


if __name__ == "__main__":
    raise SystemExit(main())
