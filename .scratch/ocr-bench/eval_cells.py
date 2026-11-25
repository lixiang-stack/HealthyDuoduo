"""Cell-level localized field recovery against samples/expected/vl.

Rows are reconstructed per engine (RapidOCR: y-clustered boxes with deskew;
GLM: HTML <tr>, markdown rows, or whitespace tokens).  A GT item is located by
its name/abbr in one cell; its field segment runs to the next item-name cell
on the same row.  This keeps the comparison independent of the production
column/pairing rules while avoiding nested-name false matches.
"""
from __future__ import annotations
import html.parser, json, pathlib, re, statistics, unicodedata
from collections import Counter

ROOT = pathlib.Path(__file__).resolve().parents[2]
BENCH = pathlib.Path(__file__).parent
VL = ROOT / "samples" / "expected" / "vl"


def nfkc(s):
    s = unicodedata.normalize("NFKC", s or "")
    return s.replace("−", "-").replace("–", "-").replace("—", "-").replace("－", "-")


def z(s):
    return re.sub(r"\s+", " ", nfkc(s)).strip().lower()


def compact(s):
    return re.sub(r"\s+", "", z(s))


def unit_form(s):
    s = compact(s).replace("μ", "u").replace("µ", "u").replace("↑", "").replace("↓", "")
    s = s.lstrip("*×x")
    return re.sub(r"10\s*[\^*~×]?\s*(\d+)", r"10^\1", s)


def ref_form(s):
    s = compact(s)
    for a in ("～", "〜", "~", "–", "—", "－", "−"):
        s = s.replace(a, "-")
    return re.sub(r"-{2,}", "-", s)


def keys(g):
    out = []
    for k in ("name", "abbr"):
        v = compact((g.get(k) or "").lstrip("*"))
        if v:
            out.append((v, k == "abbr"))
    return out


def value_ok(text, value):
    v = compact(value)
    if not v:
        return False
    spaced = z(text)
    if re.fullmatch(r"[<>≤≥]?-?\d+(?:\.\d+)?", v):
        return bool(re.search(r"(?<![\d.])" + re.escape(v) + r"(?![\d.])", spaced))
    if v in ("-", "+", "+-", "±"):
        return bool(re.search(r"(?:^|\s)" + re.escape(v) + r"(?:\s|$)", spaced))
    return v in compact(text)


def unit_ok(text, unit):
    u = unit_form(unit)
    return not u or u in unit_form(text)


def ref_ok(text, ref):
    r = ref_form(ref)
    return not r or r in ref_form(text)


# ---------------- rows ----------------

class TableRows(html.parser.HTMLParser):
    def __init__(self):
        super().__init__()
        self.rows, self.row, self.cell = [], None, None

    def handle_starttag(self, tag, attrs):
        if tag == "tr":
            self.row = []
        elif tag in ("td", "th") and self.row is not None:
            self.cell = []

    def handle_endtag(self, tag):
        if tag in ("td", "th") and self.cell is not None and self.row is not None:
            self.row.append("".join(self.cell).strip()); self.cell = None
        elif tag == "tr" and self.row is not None:
            self.rows.append(self.row); self.row = None

    def handle_data(self, data):
        if self.cell is not None:
            self.cell.append(data)


def glm_rows(sid):
    text = json.loads((BENCH / "raw" / "glm" / f"{sid}.json").read_text())["response"]
    if "<table" in text.lower():
        p = TableRows(); p.feed(text)
        return [[c for c in r if c.strip()] for r in p.rows if any(c.strip() for c in r)]
    rows = []
    for line in text.splitlines():
        line = line.strip()
        if not line:
            continue
        if line.startswith("|") and line.endswith("|"):
            cells = [c.strip() for c in line.strip("|").split("|")]
            if all(re.fullmatch(r":?-{2,}:?", c) for c in cells):
                continue
            rows.append([c for c in cells if c])
        else:
            rows.append(line.split())
    return rows


def estimate_skew(items):
    keys_ = ("项目名称", "检验项目", "结果", "单位", "参考", "代号", "缩写", "测定结果", "提示")
    heads = [(x0, (y0 + y1) / 2) for y0, y1, x0, _x1, t in items if any(k in t for k in keys_)]
    if len(heads) < 3:
        return 0.0
    top = min(y for _, y in heads)
    band = [(x, y) for x, y in heads if abs(y - top) < 60]
    if len(band) < 3 or max(x for x, _ in band) - min(x for x, _ in band) < 100:
        return 0.0
    mx = sum(x for x, _ in band) / len(band); my = sum(y for _, y in band) / len(band)
    den = sum((x - mx) ** 2 for x, _ in band)
    return max(-0.05, min(0.05, sum((x - mx) * (y - my) for x, y in band) / den if den else 0.0))


def rapid_rows(sid):
    d = json.loads((BENCH / "raw" / "rapidocr" / f"{sid}.json").read_text())["ocr"]
    items = []
    for t, box in zip(d["txts"], d["boxes"]):
        xs = [p[0] for p in box]; ys = [p[1] for p in box]
        items.append((min(ys), max(ys), min(xs), max(xs), t))
    heights = sorted(y1 - y0 for y0, y1, _, _, _ in items)
    tol = max(6.0, 0.65 * heights[len(heights) // 2]) if heights else 10.0
    slope = estimate_skew(items)
    cx0 = sum((x0 + x1) / 2 for _, _, x0, x1, _ in items) / max(1, len(items))
    boxes = [((y0 + y1) / 2 - slope * ((x0 + x1) / 2 - cx0), x0, t)
             for y0, y1, x0, x1, t in items]
    boxes.sort()
    rows = []
    for cy, x0, t in boxes:
        best = min(rows, key=lambda r: abs(r["cy"] - cy), default=None)
        if best is not None and abs(best["cy"] - cy) <= tol:
            best["cells"].append((x0, t)); best["cy"] = (best["cy"] * best["n"] + cy) / (best["n"] + 1); best["n"] += 1
        else:
            rows.append({"cy": cy, "n": 1, "cells": [(x0, t)]})
    rows.sort(key=lambda r: r["cy"])
    return [[t for _, t in sorted(r["cells"])] for r in rows]


# ---------------- scoring ----------------

def locate(gt, rows):
    """all exact/near-exact name cells per GT item, then best local segment."""
    # candidate cells: (match_quality, key_len, ri, ci, gi)
    cands = []
    for gi, g in enumerate(gt):
        for k, is_abbr in keys(g):
            if not k:
                continue
            for ri, cells in enumerate(rows):
                for ci, cell in enumerate(cells):
                    cc = compact(cell).lstrip("*")
                    if cc == k:
                        cands.append((0, len(k), ri, ci, gi))
                    elif not is_abbr and len(k) >= 2 and k in cc:
                        cands.append((1, len(k), ri, ci, gi))
    # a cell belongs to the longest matching name; exact beats substring
    owner = {}
    for q, klen, ri, ci, gi in sorted(cands, key=lambda c: (c[0], -c[1])):
        owner.setdefault((ri, ci), gi)
    by_row = {}
    for (ri, ci), gi in owner.items():
        by_row.setdefault(ri, []).append((ci, gi))
    for lst in by_row.values():
        lst.sort()

    best_rec = {}
    for gi, g in enumerate(gt):
        for ri, lst in by_row.items():
            for j, (ci, owner_gi) in enumerate(lst):
                if owner_gi != gi:
                    continue
                end = next((c for c, o in lst[j + 1:] if o != gi), len(rows[ri]))
                seg = " ".join(rows[ri][ci:end])
                rec = best_rec.setdefault(gi, {"value": False, "unit": False, "ref": False})
                rec["value"] |= value_ok(seg, g.get("value") or "")
                rec["unit"] |= unit_ok(seg, g.get("unit") or "")
                rec["ref"] |= ref_ok(seg, g.get("ref_range") or "")
    return best_rec


def summarize(gt, best):
    out = Counter(n=len(gt), name=0, value=0, unit=0, ref=0, complete=0, unit_n=0, ref_n=0)
    for gi, g in enumerate(gt):
        rec = best.get(gi)
        if rec is None:
            continue
        out["name"] += 1
        out["value"] += rec["value"]
        if (g.get("unit") or "").strip():
            out["unit_n"] += 1; out["unit"] += rec["unit"]
        if (g.get("ref_range") or "").strip():
            out["ref_n"] += 1; out["ref"] += rec["ref"]
        out["complete"] += rec["value"] and rec["unit"] and rec["ref"]
    return out


def main():
    sids = sorted(p.stem for p in VL.glob("*.json"))
    totals = {e: Counter() for e in ("rapidocr", "glm")}
    for sid in sids:
        gt = json.loads((VL / f"{sid}.json").read_text())
        rs = summarize(gt, locate(gt, rapid_rows(sid)))
        gs = summarize(gt, locate(gt, glm_rows(sid)))
        for e, s in (("rapidocr", rs), ("glm", gs)):
            totals[e].update(s)
        print(f"{sid:8} rap n {rs['name']:2}/{rs['n']:2} v {rs['value']:2} u {rs['unit']:2}/{rs['unit_n']:2} "
              f"r {rs['ref']:2}/{rs['ref_n']:2} exact {rs['complete']:2} | "
              f"glm n {gs['name']:2}/{gs['n']:2} v {gs['value']:2} u {gs['unit']:2}/{gs['unit_n']:2} "
              f"r {gs['ref']:2}/{gs['ref_n']:2} exact {gs['complete']:2}")
    print("\n=== totals ===")
    for e, t in totals.items():
        print(f"{e:9} name {t['name']}/{t['n']} ({t['name']/t['n']:.1%}) value {t['value']}/{t['n']} "
              f"({t['value']/t['n']:.1%}) unit {t['unit']}/{t['unit_n']} ({t['unit']/t['unit_n']:.1%}) "
              f"ref {t['ref']}/{t['ref_n']} ({t['ref']/t['ref_n']:.1%}) exact {t['complete']}/{t['n']} "
              f"({t['complete']/t['n']:.1%})")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
