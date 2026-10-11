"""原型(一次性,用于选型决策,不进产品):对比两种表格结构识别引擎在真实样本上的
结构还原质量,并对比「整页」与「裁剪到表头带之上」两种输入。

引擎(均 ONNX,复用现有 rapidocr 结果,不重复 OCR):
- SLANet_plus       : rapid_table.RapidTable(model_type=SLANETPLUS)
- TableStructureRec : table_cls 判定 wired/wireless → wired_table_rec / lineless_table_rec

指标(每样本×引擎×输入):
- rows/cols    : 输出 HTML 网格行列数
- degen        : rows<2 或 cols<2(结构未还原)
- clean        : 期望项(该类别词典命中名)里「独占一个单元格」的比例(左右双栏兼容;
                 同一格命中 ≥2 个期望项 = 合并失败,均不计)
- s            : 耗时

运行:
  uv run --with rapid-table --with wired-table-rec --with lineless-table-rec --with table-cls \\
    python tools/tsr_compare.py
"""

import json
import re
import sys
from pathlib import Path

import numpy as np
from PIL import Image

sys.path.insert(0, str(Path(__file__).resolve().parents[1]))

from recognizer import paths
from recognizer import postprocess as pp
from recognizer.contract import OCRResult

_EXT = {
    "cbc_01": "jpeg", "cbc_02": "png", "cbc_03": "png", "cbc_04": "png", "cbc_05": "jpg",
    "glu_01": "png", "glu_02": "jpeg", "glu_03": "jpeg",
    "lft_01": "jpeg", "lft_02": "jpg", "lft_03": "png", "lft_04": "jpeg", "lft_05": "jpeg",
    "tft_01": "jpeg", "tft_02": "jpeg", "tft_03": "jpeg",
    "ua_01": "jpg", "ua_02": "jpeg", "ua_03": "webp", "ua_04": "jpg", "ua_05": "jpeg",
    "us_01": "webp", "us_02": "jpeg", "us_03": "jpeg",
}

_TR_RE = re.compile(r"<tr>(.*?)</tr>", re.DOTALL)
_TD_RE = re.compile(r"<td[^>]*>(.*?)</td>", re.DOTALL)


def parse_grid(html: str) -> list[list[str]]:
    return [
        [re.sub(r"<[^>]+>", "", td).strip() for td in _TD_RE.findall(tr)]
        for tr in _TR_RE.findall(html)
    ]


def expected_items(sample_id: str, ocr: OCRResult, dicts: dict) -> tuple[str, set[str]]:
    golden = json.loads((paths.EXPECTED_REPORT / f"{sample_id}.json").read_text(encoding="utf-8"))
    rtype = golden["report_type"]
    d = dicts.get(rtype)
    if d is None:
        return rtype, set()
    names: set[str] = set()
    for t in ocr.txts:
        it, _ = pp._match_name(t, d)
        if it is not None:
            names.add(it.name)
    return rtype, names


def _cell_hits(cell: str, d: pp.LabDict, expected: set[str]) -> set[str]:
    """单元格内命中的期望项(最长别名优先,命中即从串中移除,避免子串重复计数)。"""
    hits: set[str] = set()
    rest = cell
    for alias in sorted(d.alias_map, key=len, reverse=True):
        if alias in rest:
            it = d.alias_map[alias]
            if it.name in expected:
                hits.add(it.name)
            rest = rest.replace(alias, " ", 1)
    return hits


def clean_recall(grid: list[list[str]], d: pp.LabDict, expected: set[str]) -> float:
    clean: set[str] = set()
    for row in grid:
        for cell in row:
            hit = _cell_hits(cell, d, expected)
            if len(hit) == 1:
                clean |= hit
    return len(clean) / len(expected) if expected else float("nan")


def header_top(ocr: OCRResult) -> float | None:
    cells = pp._cells(ocr)
    bands = pp._bands(cells)
    hb = [
        b for b in bands
        if sum(1 for c in b for _r, kws in pp.ROLE_HEADERS.items() if any(kw in c.text.strip() for kw in kws))
        >= pp._HEADER_MIN_ROLES
    ]
    return min((c.y0 for b in hb for c in b), default=None)


class Sample:
    """一次 OCR 结果的两种输入视图(整页 / 裁到表头带之上)。"""

    def __init__(self, sid: str, ocr: OCRResult):
        self.sid = sid
        self.ocr = ocr
        self.img = Image.open(paths.REPO_ROOT / "samples" / f"{sid}.{_EXT[sid]}")
        self.boxes = np.array(ocr.boxes, dtype=np.float32)
        self.txts = tuple(ocr.txts)
        self.scores = tuple(ocr.scores)

    def cropped(self) -> tuple[np.ndarray, tuple[str], tuple[float]]:
        top = header_top(self.ocr)
        if top is None:
            return self.img, self.boxes, self.txts, self.scores
        y0 = max(0, int(top) - 8)
        img = self.img.crop((0, y0, self.img.width, self.img.height))
        boxes = np.array([[[p[0], p[1] - y0] for p in b] for b in self.ocr.boxes if max(p[1] for p in b) >= y0], dtype=np.float32)
        txts = tuple(t for t, b in zip(self.ocr.txts, self.ocr.boxes) if max(p[1] for p in b) >= y0)
        scores = tuple(s for s, b in zip(self.ocr.scores, self.ocr.boxes) if max(p[1] for p in b) >= y0)
        return img, boxes, txts, scores


def run_slanetplus(engine, img, boxes, txts, scores) -> tuple[str, float]:
    res = engine(np.asarray(img), ocr_results=[(boxes, txts, scores)])
    return res.pred_htmls[0], float(res.elapse)


def run_tsr(cls_engine, wired, lineless, sid, img, boxes, txts, scores) -> tuple[str, float]:
    cls, _ = cls_engine(str(paths.REPO_ROOT / "samples" / f"{sid}.{_EXT[sid]}"))
    eng = wired if cls == "wired" else lineless
    res = eng(np.asarray(img), ocr_result=[(b, t, s) for b, t, s in zip(boxes, txts, scores)])
    html = getattr(res, "pred_htmls", None)
    html = html[0] if html else res.pred_html
    return html, float(res.elapse)


def main() -> int:
    dicts = {d.report_type: d for d in pp.load_dicts()}

    from lineless_table_rec.main import LinelessTableInput, LinelessTableRecognition
    from rapid_table import ModelType, RapidTable, RapidTableInput
    from table_cls import TableCls
    from wired_table_rec.main import WiredTableInput, WiredTableRecognition

    slate = RapidTable(RapidTableInput(model_type=ModelType.SLANETPLUS))
    cls_engine, wired, lineless = TableCls(), WiredTableRecognition(WiredTableInput()), LinelessTableRecognition(LinelessTableInput())

    print(f"{'sample':8} {'type':6} {'exp':>3} {'engine':14} {'in':6} {'rows':>4} {'cols':>4} {'degen':>5} {'clean':>6} {'s':>5}")
    for sid in sorted(p.stem for p in paths.EXPECTED_OCR.glob("*.json")):
        ocr = OCRResult.model_validate(json.loads((paths.EXPECTED_OCR / f"{sid}.json").read_text(encoding="utf-8")))
        rtype, expected = expected_items(sid, ocr, dicts)
        d = dicts.get(rtype)
        if d is None or d.mode == "narrative":
            print(f"{sid:8} {rtype:6} {'-':>3} (narrative/unknown: TSR 不适用)")
            continue
        s = Sample(sid, ocr)
        cimg, cboxes, ctxts, cscores = s.cropped()

        for label, img, boxes, txts, scores in (
            ("full", s.img, s.boxes, s.txts, s.scores),
            ("crop", cimg, cboxes, ctxts, cscores),
        ):
            html, el = run_slanetplus(slate, img, boxes, txts, scores)
            g = parse_grid(html)
            r, c = len(g), max((len(x) for x in g), default=0)
            print(f"{sid:8} {rtype:6} {len(expected):>3} {'SLANet_plus':14} {label:6} {r:>4} {c:>4} "
                  f"{r < 2 or c < 2!s:>5} {clean_recall(g, d, expected):>6.2f} {el:>5.2f}")

            html, el = run_tsr(cls_engine, wired, lineless, sid, img, boxes, txts, scores)
            g = parse_grid(html)
            r, c = len(g), max((len(x) for x in g), default=0)
            print(f"{sid:8} {rtype:6} {len(expected):>3} {'TSR':14} {label:6} {r:>4} {c:>4} "
                  f"{r < 2 or c < 2!s:>5} {clean_recall(g, d, expected):>6.2f} {el:>5.2f}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
