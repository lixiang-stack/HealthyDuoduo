"""基于真实数据对照(samples/expected/vl,LLM 结构化、禁改)对比抽取方案。

方案(可在 SCHEMES 扩展对比):
- heuristic : 启发式几何抽取(postprocess._extract_items)

指标(相对 GT,按印刷顺序做有序对齐):
- matched/n : GT 项被抽到的比例
- vok/uok/rok : 对齐项中 value/unit/ref 与 GT 一致的数量(定性值 GT 非数值时期望 pred.value=None)

运行:
  cd recognizer && uv run python tools/gt_eval.py
"""

import json
import re
import sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parents[1]))

from recognizer import paths
from recognizer import postprocess as pp
from recognizer.contract import OCRResult

PREFIX = {"cbc": "血常规", "glu": "血糖", "lft": "肝肾功能", "tft": "甲状腺功能", "ua": "尿常规"}
VL = paths.REPO_ROOT / "samples" / "expected" / "vl"


def load_gt(sid: str):
    f = VL / f"{sid}.json"
    if not f.exists() or f.stat().st_size == 0:
        return None
    return json.loads(f.read_text(encoding="utf-8"))


def _num(s) -> float | None:
    try:
        return float(s)
    except (TypeError, ValueError):
        return None


def _norm_ref(s) -> str | None:
    if s is None:
        return None
    s = s.strip()
    for a in ("～", "〜", "~", "–", "—", "－", "−", "ｰ"):
        s = s.replace(a, "-")
    return re.sub(r"\s+", "", s)


def _norm_unit(s) -> str | None:
    if s is None:
        return None
    return (s.strip().replace("μ", "u").replace("µ", "u")).lower() or None


def _canon(name: str, d) -> str | None:
    if not name:
        return None
    it, _ = pp._match_name(name, d)
    return it.name if it else None


def _pred(pairs) -> list[dict]:
    out = []
    for it, p in pairs:
        raw = "  ".join(c.text for c in p.raw_cells)
        out.append({"name": it.name, "value": p.value, "unit": p.unit, "ref": p.ref, "raw": raw})
    return out


def _match(g: dict, p: dict, d) -> bool:
    raw = p["raw"] or ""
    if g.get("name") and g["name"] in raw:
        return True
    ab = g.get("abbr") or ""
    if ab and ab.lower() in raw.lower():
        return True
    cg = _canon(g.get("name", ""), d) or _canon(ab, d)
    if cg and cg == p["name"]:
        return True
    gn = g.get("name") or ""
    if gn and (gn in p["name"] or p["name"] in gn):
        return True
    gv = _num(g.get("value", ""))
    return gv is not None and p["value"] is not None and abs(gv - p["value"]) < 1e-6


def _align(gt: list, preds: list[dict], d) -> list[tuple]:
    i = j = 0
    pairs = []
    while i < len(gt) and j < len(preds):
        if _match(gt[i], preds[j], d):
            pairs.append((gt[i], preds[j]))
            i += 1
            j += 1
        elif j + 1 < len(preds) and _match(gt[i], preds[j + 1], d):
            j += 1
        elif i + 1 < len(gt) and _match(gt[i + 1], preds[j], d):
            i += 1
        else:
            i += 1
            j += 1
    return pairs


def _score(gt: list, preds: list[dict], d) -> dict:
    pairs = _align(gt, preds, d)
    vok = uok = rok = 0
    for g, p in pairs:
        gv = _num(g.get("value", ""))
        if gv is not None:
            if p["value"] is not None and abs(gv - p["value"]) < 1e-6:
                vok += 1
        elif p["value"] is None:
            vok += 1
        if _norm_unit(g.get("unit")) == _norm_unit(p["unit"]):
            uok += 1
        if _norm_ref(g.get("ref_range")) == _norm_ref(p["ref"]):
            rok += 1
    return {"n": len(gt), "matched": len(pairs), "vok": vok, "uok": uok, "rok": rok}


def _ctx(sid: str, dicts: dict):
    ocr = OCRResult.model_validate(json.loads((paths.EXPECTED_OCR / f"{sid}.json").read_text(encoding="utf-8")))
    d = dicts[PREFIX[sid.split("_")[0]]]
    cells = pp._cells(ocr)
    bands = [
        b for b in pp._bands(cells)
        if sum(1 for c in b for _r, kws in pp.ROLE_HEADERS.items() if any(kw in c.text.strip() for kw in kws)) >= pp._HEADER_MIN_ROLES
    ]
    return d, cells, bands


def heuristic(d, cells, bands):
    return _pred(pp._extract_items(cells, bands, d))


SCHEMES = {"heuristic": heuristic}


def main() -> int:
    dicts = {x.report_type: x for x in pp.load_dicts()}
    totals = {k: {"n": 0, "matched": 0, "vok": 0, "uok": 0, "rok": 0} for k in SCHEMES}
    header = f"{'sample':8}" + "".join(f" | {k:>22}" for k in SCHEMES)
    print(header)
    print(f"{'':8}" + "".join(f" | {'m/n  vok uok rok':>22}" for _ in SCHEMES))
    for p in sorted(paths.EXPECTED_OCR.glob("*.json")):
        sid = p.stem
        gt = load_gt(sid)
        if gt is None or sid.split("_")[0] not in PREFIX:
            continue
        d, cells, bands = _ctx(sid, dicts)
        row = f"{sid:8}"
        for k, fn in SCHEMES.items():
            s = _score(gt, fn(d, cells, bands), d)
            for key in totals[k]:
                totals[k][key] += s[key]
            row += f" | {s['matched']:>2}/{s['n']:<2} {s['vok']:>2} {s['uok']:>3} {s['rok']:>3}  "
        print(row)
    print("\n=== totals ===")
    for k, t in totals.items():
        print(f"{k:10} matched {t['matched']}/{t['n']}  value {t['vok']}  unit {t['uok']}  ref {t['rok']}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
