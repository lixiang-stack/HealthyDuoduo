"""P2 报告 golden 重生成工具。

用法(仓库根执行):
  uv run --project recognizer python -m recognizer.golden

把 samples/expected/ocr/<id>.json 跑一遍 postprocess,写 samples/expected/report/<id>.json。
规则或词典改动后重新生成,并在 tre中有出入时人工抽查( 地址 raw_text 提供核对)。
"""

import json
from pathlib import Path

from .contract import OCRResult
from .postprocess import run_postprocess

REPO_ROOT = Path(__file__).resolve().parents[2]
SOURCES = REPO_ROOT / "samples" / "expected" / "ocr"
TARGETS = REPO_ROOT / "samples" / "expected" / "report"


def main() -> int:
    for src in sorted(SOURCES.glob("*.json")):
        ocr = OCRResult.model_validate(json.loads(src.read_text(encoding="utf-8")))
        report = run_postprocess(ocr)
        TARGETS.mkdir(parents=True, exist_ok=True)
        out = TARGETS / src.name
        out.write_text(report.model_dump_json() + "\n", encoding="utf-8")
        print(f"{src.name}: {report.status} {len(report.items)} items -> {out.relative_to(REPO_ROOT)}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
