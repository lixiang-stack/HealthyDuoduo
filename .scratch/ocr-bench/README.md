# OCR engine benchmark: GLM-OCR vs RapidOCR

Reproduces the 2026-11-25 evaluation. Raw outputs are **not** committed
(`raw/` is ignored): they are generated from the de-identified samples in
`samples/` and must stay local.

## Requirements

- Ollama with `glm-ocr:latest` pulled (`ollama pull glm-ocr`).
- The repo's `recognizer` environment (`uv`).
- `samples/expected/vl/<id>.json` per-item ground truth.

## Run

```bash
# 1. RapidOCR, warm timing (run from the repository root)
cd recognizer
uv run python ../.scratch/ocr-bench/run_rapidocr.py

# 2. GLM-OCR via Ollama (Table Recognition prompt, num_ctx=16384)
cd ..
python3 .scratch/ocr-bench/run_glm.py

# 3. Paired comparison (excludes cbc_03, whose VL GT does not match the image)
cd recognizer
uv run python ../.scratch/ocr-bench/compare.py
```

`eval_cells.py` prints the engine-only cell/row metric; `eval_report.py`
prints the production RapidOCR report metric. `compare.py` combines both and
emits the McNemar test.

## Notes

- GLM is prompted with `Table Recognition:`; `Text Recognition:` produced
  nearly identical text in the pilot and was not benchmarked separately.
- `cbc_03` is excluded because `samples/expected/vl/cbc_03.json` describes a
  24-item CBC while the image is a 16-row morphology report. Do not edit the
  VL golden (see `samples/README.md`).
- Latency is warm wall-clock time on an M1 Pro / 32 GB, Ollama 100% GPU.
  RapidOCR uses onnxruntime on CPU.
