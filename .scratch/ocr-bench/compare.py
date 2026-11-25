"""Paired per-item comparison: production RapidOCR report vs GLM row-segment recovery."""
from __future__ import annotations
import json, math, pathlib, re, sys, unicodedata
from collections import Counter

ROOT = pathlib.Path(__file__).resolve().parents[2]
BENCH = pathlib.Path(__file__).parent
VL = ROOT / "samples" / "expected" / "vl"
sys.path.insert(0, str(BENCH))
sys.path.insert(0, str(ROOT / "recognizer"))
from eval_cells import glm_rows, locate, rapid_rows, value_ok, unit_ok, ref_ok  # noqa: E402
from eval_report import match_item, val_form, unit_form, ref_form, image_for  # noqa: E402
from recognizer.contract import OCRResult  # noqa: E402
from recognizer.postprocess import run_postprocess  # noqa: E402


def rapid_item_ok(g, it):
    if it is None:
        return False, False, False
    rv = val_form(it.get("raw_value") or it.get("value") or "")
    gv = val_form(g.get("value") or "")
    vok = bool(gv) and (rv == gv or (not rv and gv in re.sub(r"\s+", "", (it.get("raw_text") or "")).lower()))
    uok = not (g.get("unit") or "").strip() or unit_form(it.get("raw_unit") or it.get("unit") or it.get("unit_display") or "") == unit_form(g.get("unit"))
    rok = not (g.get("ref_range") or "").strip() or ref_form(it.get("raw_ref_range") or it.get("ref_range") or "") == ref_form(g.get("ref_range"))
    return vok, uok, rok


def binom_p(b, c):
    n = b + c
    if n == 0:
        return 1.0
    k = min(b, c)
    # two-sided exact binomial
    return min(1.0, 2 * sum(math.comb(n, i) for i in range(k + 1)) / 2**n)


def main():
    sids = [p.stem for p in sorted(VL.glob("*.json")) if p.stem != "cbc_03"]
    rows_out = []
    wins = Counter()
    totals = {e: Counter() for e in ("rapid", "glm")}
    for sid in sids:
        gt = json.loads((VL / f"{sid}.json").read_text())
        rec = json.loads((BENCH / "raw" / "rapidocr" / f"{sid}.json").read_text())
        ocr = OCRResult.model_validate(rec["ocr"])
        img = image_for(sid)
        report = run_postprocess(ocr, image=img.read_bytes() if img else None)
        items = [it.model_dump() for it in report.items]
        glm_best = locate(gt, glm_rows(sid))
        r_ok = g_ok = 0
        b = c = 0
        per = []
        r_lat = rec["wall_s"]; g_lat = json.loads((BENCH / "raw" / "glm" / f"{sid}.json").read_text())["wall_s"]
        for gi, g in enumerate(gt):
            vok, uok, rok = rapid_item_ok(g, match_item(g, items))
            rok_all = vok and uok and rok
            grec = glm_best.get(gi, {"value": False, "unit": False, "ref": False})
            gok_all = grec["value"] and grec["unit"] and grec["ref"]
            r_ok += rok_all; g_ok += gok_all
            b += gok_all and not rok_all
            c += rok_all and not gok_all
            per.append((g.get("name"), rok_all, gok_all, vok, uok, rok, grec["value"], grec["unit"], grec["ref"]))
            totals["rapid"].update(value=vok, n=1)
            totals["glm"].update(value=grec["value"], n=1)
            if (g.get("unit") or "").strip():
                totals["rapid"]["unit_n"] += 1; totals["glm"]["unit_n"] += 1
                totals["rapid"]["unit"] += uok; totals["glm"]["unit"] += grec["unit"]
            if (g.get("ref_range") or "").strip():
                totals["rapid"]["ref_n"] += 1; totals["glm"]["ref_n"] += 1
                totals["rapid"]["ref"] += rok; totals["glm"]["ref"] += grec["ref"]
        wins["rapid" if r_ok > g_ok else "glm" if g_ok > r_ok else "tie"] += 1
        rows_out.append({"sample": sid, "n": len(gt), "rapid": r_ok, "glm": g_ok,
                         "b_glm_only": b, "c_rapid_only": c, "rapid_latency_s": round(r_lat, 3),
                         "glm_latency_s": round(g_lat, 3)})
        print(f"{sid:8} n {len(gt):2} rapid {r_ok:2} glm {g_ok:2}  glm-only {b:2} rapid-only {c:2}")
    n = sum(r["n"] for r in rows_out)
    r = sum(r["rapid"] for r in rows_out)
    g = sum(r["glm"] for r in rows_out)
    b = sum(r["b_glm_only"] for r in rows_out)
    c = sum(r["c_rapid_only"] for r in rows_out)
    macro_r = sum(r["rapid"] / r["n"] for r in rows_out) / len(rows_out)
    macro_g = sum(r["glm"] / r["n"] for r in rows_out) / len(rows_out)
    print(f"\npooled  rapid {r}/{n} ({r/n:.1%})  glm {g}/{n} ({g/n:.1%})  delta {(g-r)/n:+.1%}")
    print(f"macro   rapid {macro_r:.1%}  glm {macro_g:.1%}  delta {macro_g-macro_r:+.1%}")
    print(f"paired  glm-only {b}  rapid-only {c}  McNemar exact p={binom_p(b,c):.2g}")
    print("sample wins", dict(wins))
    for e, t in totals.items():
        print(f"{e:6} value {t['value']}/{t['n']} ({t['value']/t['n']:.1%}) "
              f"unit {t['unit']}/{t['unit_n']} ({t['unit']/t['unit_n']:.1%}) "
              f"ref {t['ref']}/{t['ref_n']} ({t['ref']/t['ref_n']:.1%})")
    print("latency medians: rapid", sorted(r['rapid_latency_s'] for r in rows_out)[len(rows_out)//2],
          "glm", sorted(r['glm_latency_s'] for r in rows_out)[len(rows_out)//2])
    (BENCH / "compare.json").write_text(json.dumps(rows_out, indent=1))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
