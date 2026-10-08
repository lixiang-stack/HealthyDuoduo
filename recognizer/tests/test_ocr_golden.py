"""P1 golden 回归(实施计划 8-5):run_ocr 输出对比 samples/expected/ocr/。

对比规则:txts 逐字相等、scores 容差 ±0.02、boxes 容差 ±2px、
elapse / elapse_list 只通过 schema 校验不比对(耗时非确定量,NF-02/NF-05)。
"""

import json
from pathlib import Path

import pytest
from jsonschema import Draft202012Validator

from recognizer.contract import OCRResult
from recognizer.ocr import run_ocr

REPO_ROOT = Path(__file__).resolve().parents[2]
SAMPLES = REPO_ROOT / "samples"
EXPECTED_OCR = SAMPLES / "expected" / "ocr"
SCHEMA = REPO_ROOT / "schemas" / "ocr_result.schema.json"

SCORE_TOLERANCE = 0.02
BOX_TOLERANCE_PX = 2.0

_sample_ids = sorted(p.stem for p in EXPECTED_OCR.glob("*.json"))


def _load(path: Path) -> dict:
    return json.loads(path.read_text(encoding="utf-8"))


@pytest.mark.skipif(not _sample_ids, reason="no golden files under samples/expected/ocr/")
@pytest.mark.parametrize("sample_id", _sample_ids)
def test_ocr_matches_golden(sample_id: str) -> None:
    image_path = next(iter(SAMPLES.glob(f"{sample_id}.*")))
    ocr = run_ocr(image_path)
    _assert_matches_golden(ocr, _load(EXPECTED_OCR / f"{sample_id}.json"))


def _assert_matches_golden(actual: OCRResult, golden: dict) -> None:
    actual_json = json.loads(actual.model_dump_json())
    Draft202012Validator(_load(SCHEMA)).validate(actual_json)

    assert actual.txts == golden["txts"], "txts must be exactly equal"

    assert len(actual.scores) == len(golden["scores"])
    for got, want in zip(actual.scores, golden["scores"]):
        assert abs(got - want) <= SCORE_TOLERANCE

    assert len(actual.boxes) == len(golden["boxes"])
    for got_box, want_box in zip(actual.boxes, golden["boxes"]):
        for (gx, gy), (wx, wy) in zip(got_box, want_box):
            assert abs(gx - wx) <= BOX_TOLERANCE_PX
            assert abs(gy - wy) <= BOX_TOLERANCE_PX

    assert actual.engine == "onnxruntime"
    assert actual.model_info.model_dump() == golden["model_info"]
