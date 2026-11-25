"""Run the repo's RapidOCR engine over samples; save OCRResult + timings."""
import json, pathlib, resource, sys, time

ROOT = pathlib.Path(__file__).resolve().parents[2]
sys.path.insert(0, str(ROOT / "recognizer"))
from recognizer.ocr import run_ocr

OUT = pathlib.Path(__file__).parent / "raw" / "rapidocr"

def main() -> None:
    OUT.mkdir(parents=True, exist_ok=True)
    exts = {".jpg", ".jpeg", ".png", ".webp"}
    first = True
    for p in sorted(ROOT.joinpath("samples").iterdir()):
        if p.suffix.lower() not in exts:
            continue
        dst = OUT / f"{p.stem}.json"
        if dst.exists():
            print("skip", p.stem)
            continue
        if first:
            run_ocr(p)  # warm up model/caches
            first = False
        t0 = time.perf_counter()
        r = run_ocr(p)
        wall = time.perf_counter() - t0
        rec = {"sample": p.stem, "image": p.name, "wall_s": wall,
               "max_rss_kb": resource.getrusage(resource.RUSAGE_SELF).ru_maxrss,
               "ocr": json.loads(r.model_dump_json())}
        dst.write_text(json.dumps(rec, ensure_ascii=False, indent=1), encoding="utf-8")
        print(p.stem, f"{wall:.3f}s", len(r.txts), "lines", flush=True)

if __name__ == "__main__":
    main()
