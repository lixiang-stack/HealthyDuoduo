"""P2 报告 golden 重生成工具。

用法(仓库根执行):
  uv run --project recognizer python -m recognizer.golden

把 samples/expected/ocr/<id>.json 跑一遍 postprocess,写 samples/expected/report/<id>.json。
规则或词典改动后重新生成,并对有出入的样本人工抽查(raw_text 提供核对依据)。

掩码语料条目(samples 内无对应原图的 id,见 samples/README):身份/机构/时间一律脱敏,
report_date 恒为 null → 全单 partial。为免把「掩码侧因失真」误当解析缺陷,这类条目的
golden 用假日期占位(MASKED_DATE),status 因此反映解析本身的状态;真值 date 属 PII 永不提交。
"""

import json
from pathlib import Path

from . import paths
from .contract import OCRResult
from .postprocess import run_postprocess

REPO_ROOT = paths.REPO_ROOT
SOURCES = paths.EXPECTED_OCR
TARGETS = paths.EXPECTED_REPORT
MASKED_DATE = "2020-01-01"


def image_path_for(sample_id: str) -> Path | None:
    """samples 内对应原图;掩码语料条目没有原图,返回 None。"""
    for ext in paths.IMAGE_SUFFIXES:
        path = Path(paths.SAMPLES) / f"{sample_id}{ext}"
        if path.exists():
            return path
    return None


def date_hint_for(sample_id: str) -> str | None:
    """样本的 date_hint:samples 内有对应原图的样本 None,掩码语料条目 MASKED_DATE。"""
    return None if image_path_for(sample_id) else MASKED_DATE


def image_bytes_for(sample_id: str) -> bytes | None:
    path = image_path_for(sample_id)
    return path.read_bytes() if path is not None else None


def main() -> int:
    for src in sorted(SOURCES.glob("*.json")):
        ocr = OCRResult.model_validate(json.loads(src.read_text(encoding="utf-8")))
        report = run_postprocess(
            ocr,
            date_hint=date_hint_for(src.stem),
            image=image_bytes_for(src.stem),
        )
        TARGETS.mkdir(parents=True, exist_ok=True)
        out = TARGETS / src.name
        out.write_text(report.model_dump_json() + "\n", encoding="utf-8")
        print(f"{src.name}: {report.status.value} {len(report.items)} items -> {out.relative_to(REPO_ROOT)}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
