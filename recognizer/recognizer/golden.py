"""P2 报告 golden 重生成工具。

用法(仓库根执行):
  uv run --project recognizer python -m recognizer.golden

把 samples/expected/ocr/<id>.json 跑一遍 postprocess,写 samples/expected/report/<id>.json。
规则或词典改动后重新生成,并对有出入的样本人工抽查(raw_text 提供核对依据)。
"""

import json

from . import paths
from .contract import OCRResult
from .postprocess import run_postprocess

REPO_ROOT = paths.REPO_ROOT
SOURCES = paths.EXPECTED_OCR
TARGETS = paths.EXPECTED_REPORT


def main() -> int:
    for src in sorted(SOURCES.glob("*.json")):
        ocr = OCRResult.model_validate(json.loads(src.read_text(encoding="utf-8")))
        report = run_postprocess(ocr)
        TARGETS.mkdir(parents=True, exist_ok=True)
        out = TARGETS / src.name
        out.write_text(report.model_dump_json() + "\n", encoding="utf-8")
        print(f"{src.name}: {report.status.value} {len(report.items)} items -> {out.relative_to(REPO_ROOT)}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
