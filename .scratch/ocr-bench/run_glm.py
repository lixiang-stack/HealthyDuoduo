"""Run GLM-OCR (Ollama) over samples; save raw responses + timings."""
import base64, json, pathlib, time, urllib.request

ROOT = pathlib.Path(__file__).resolve().parents[2]
OUT = pathlib.Path(__file__).parent / "raw" / "glm"
PROMPT = "Table Recognition:"
MODEL = "glm-ocr:latest"

def run(path: pathlib.Path) -> dict:
    b64 = base64.b64encode(path.read_bytes()).decode()
    payload = json.dumps({
        "model": MODEL, "prompt": PROMPT, "images": [b64], "stream": False,
        "options": {"num_ctx": 16384, "temperature": 0},
    }).encode()
    req = urllib.request.Request(
        "http://localhost:11434/api/generate", data=payload,
        headers={"Content-Type": "application/json"})
    t0 = time.perf_counter()
    r = json.loads(urllib.request.urlopen(req, timeout=600).read())
    wall = time.perf_counter() - t0
    return {
        "sample": path.stem, "image": path.name, "prompt": PROMPT, "model": MODEL,
        "wall_s": wall, "response": r.get("response", ""),
        "metrics": {k: r.get(k) for k in (
            "total_duration", "load_duration", "prompt_eval_count",
            "prompt_eval_duration", "eval_count", "eval_duration")},
        "done_reason": r.get("done_reason"),
    }

def main() -> None:
    OUT.mkdir(parents=True, exist_ok=True)
    exts = {".jpg", ".jpeg", ".png", ".webp"}
    for p in sorted(ROOT.joinpath("samples").iterdir()):
        if p.suffix.lower() not in exts:
            continue
        dst = OUT / f"{p.stem}.json"
        if dst.exists():
            print("skip", p.stem)
            continue
        rec = run(p)
        dst.write_text(json.dumps(rec, ensure_ascii=False, indent=1), encoding="utf-8")
        print(p.stem, f"{rec['wall_s']:.2f}s", len(rec["response"]), "chars", flush=True)

if __name__ == "__main__":
    main()
