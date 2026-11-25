"""Evaluate the production RapidOCR post-processor against samples/expected/vl."""
from __future__ import annotations
import json, pathlib, re, sys, unicodedata
from collections import Counter

ROOT = pathlib.Path(__file__).resolve().parents[2]
BENCH = pathlib.Path(__file__).parent
VL = ROOT / "samples" / "expected" / "vl"
sys.path.insert(0, str(ROOT / "recognizer"))
from recognizer.contract import OCRResult  # noqa: E402
from recognizer.postprocess import run_postprocess  # noqa: E402


def nfkc(s):
    s = unicodedata.normalize("NFKC", s or "")
    return s.replace("−", "-").replace("–", "-").replace("—", "-").replace("－", "-")


def compact(s):
    return re.sub(r"\s+", "", nfkc(s)).lower()


def unit_form(s):
    s = compact(s).replace("μ", "u").replace("µ", "u").replace("↑", "").replace("↓", "")
    s = s.lstrip("*×x")
    return re.sub(r"10\s*[\^*~×]?\s*(\d+)", r"10^\1", s)


def ref_form(s):
    s = compact(s)
    for a in ("～", "〜", "~", "–", "—", "－", "−"):
        s = s.replace(a, "-")
    return re.sub(r"-{2,}", "-", s)


def val_form(s):
    return re.sub(r"\s+", "", nfkc(s)).lower()


def image_for(sid):
    for ext in (".jpg", ".jpeg", ".png", ".webp"):
        p = ROOT / "samples" / f"{sid}{ext}"
        if p.exists():
            return p
    return None


def match_item(g, items):
    keys = [compact(x).lstrip("*") for x in (g.get("name"), g.get("abbr")) if x]
    best = None
    for it in items:
        raw = compact(it.get("raw_text") or "")
        hit = any(k and k in raw for k in keys)
        if not hit:
            continue
        # prefer an item whose value/ref also matches
        score = 0
        rv = val_form(it.get("raw_value") or it.get("value") or "")
        if rv and rv == val_form(g.get("value") or ""):
            score += 2
        rr = ref_form(it.get("raw_ref_range") or it.get("ref_range") or "")
        if rr and rr == ref_form(g.get("ref_range") or ""):
            score += 1
        cand = (score, it)
        if best is None or cand[0] > best[0]:
            best = cand
    return best[1] if best else None


def main():
    sids = sorted(p.stem for p in VL.glob("*.json"))
    totals = Counter(n=0, name=0, value=0, unit=0, ref=0, complete=0, unit_n=0, ref_n=0)
    for sid in sids:
        gt = json.loads((VL / f"{sid}.json").read_text())
        rec = json.loads((BENCH / "raw" / "rapidocr" / f"{sid}.json").read_text())
        ocr = OCRResult.model_validate(rec["ocr"])
        img = image_for(sid)
        report = run_postprocess(ocr, image=img.read_bytes() if img else None)
        items = [it.model_dump() for it in report.items]
        c = Counter(n=len(gt), name=0, value=0, unit=0, ref=0, complete=0, unit_n=0, ref_n=0)
        for g in gt:
            it = match_item(g, items)
            if it is None:
                continue
            c["name"] += 1
            rv = val_form(it.get("raw_value") or it.get("value") or "")
            gv = val_form(g.get("value") or "")
            vok = bool(gv) and (rv == gv or (not rv and gv in compact(it.get("raw_text") or "")))
            c["value"] += vok
            if (g.get("unit") or "").strip():
                c["unit_n"] += 1
                ru = unit_form(it.get("raw_unit") or it.get("unit") or it.get("unit_display") or "")
                c["unit"] += bool(ru) and ru == unit_form(g.get("unit"))
            if (g.get("ref_range") or "").strip():
                c["ref_n"] += 1
                rr = ref_form(it.get("raw_ref_range") or it.get("ref_range") or "")
                c["ref"] += bool(rr) and rr == ref_form(g.get("ref_range"))
            c["complete"] += vok and c["unit"] == c["unit_n"] and c["ref"] == c["ref_n"]
        # recompute complete per item properly
        c["complete"] = 0
        for g in gt:
            it = match_item(g, items)
            if it is None:
                continue
            rv = val_form(it.get("raw_value") or it.get("value") or "")
            gv = val_form(g.get("value") or "")
            vok = bool(gv) and (rv == gv or (not rv and gv in compact(it.get("raw_text") or "")))
            uok = not (g.get("unit") or "").strip() or unit_form(it.get("raw_unit") or it.get("unit") or it.get("unit_display") or "") == unit_form(g.get("unit"))
            rok = not (g.get("ref_range") or "").strip() or ref_form(it.get("raw_ref_range") or it.get("ref_range") or "") == ref_form(g.get("ref_range"))
            c["complete"] += vok and uok and rok
        totals.update(c)
        print(f"{sid:8} {report.status.value:7} items {len(items):2} | gt {c['n']:2} name {c['name']:2} val {c['value']:2} "
              f"unit {c['unit']:2}/{c['unit_n']:2} ref {c['ref']:2}/{c['ref_n']:2} exact {c['complete']:2}")
    print("\n=== totals ===")
    for k in ("name", "value", "unit", "ref", "complete"):
        den = {"name": "n", "value": "n", "unit": "unit_n", "ref": "ref_n", "complete": "n"}[k]
        print(f"{k:9} {totals[k]:3}/{totals[den]:3} ({totals[k]/totals[den]:.1%})")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
