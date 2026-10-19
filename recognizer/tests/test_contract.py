"""契约测试:schemas/ 下 JSON Schema 为源,示例 fixture 双校验(实施计划步骤 7-2)。"""

import json
from pathlib import Path

import pytest
from jsonschema import Draft202012Validator
from pydantic import ValidationError

from recognizer import paths
from recognizer.contract import OCRResult, Report

SCHEMA_DIR = paths.SCHEMAS
TESTDATA = paths.CONTRACT_TESTDATA


def _load(path: Path) -> dict:
    return json.loads(path.read_text(encoding="utf-8"))


@pytest.mark.parametrize(
    ("schema_name", "examples"),
    [
        ("ocr_result.schema.json", ["ocr_result.example.json"]),
        ("report.schema.json", ["report.example.json", "report.null-date.example.json"]),
    ],
)
def test_schemas_valid_and_examples_match(schema_name: str, examples: list[str]) -> None:
    """schema 本身是合法 JSON Schema,且文档示例全部通过校验。"""
    schema = _load(SCHEMA_DIR / schema_name)
    Draft202012Validator.check_schema(schema)
    validator = Draft202012Validator(schema)
    for name in examples:
        validator.validate(_load(TESTDATA / name))


OCR_RESULT_MODEL_BY_SCHEMA = {
    "ocr_result.schema.json": OCRResult,
    "report.schema.json": Report,
}


@pytest.mark.parametrize(
    ("schema_name", "examples"),
    [
        ("ocr_result.schema.json", ["ocr_result.example.json"]),
        ("report.schema.json", ["report.example.json", "report.null-date.example.json"]),
    ],
)
def test_pydantic_mirror_matches_examples(schema_name: str, examples: list[str]) -> None:
    """pydantic 镜像模型能解析同一批种子示例(双语契约防漂移)。"""
    model = OCR_RESULT_MODEL_BY_SCHEMA[schema_name]
    for name in examples:
        obj = model.model_validate(_load(TESTDATA / name))
        assert obj is not None


def test_invalid_ocr_result_rejected() -> None:
    """负例:分数越界、engine 非枚举值、行数不等长 → pydantic 拒绝。"""
    instance = _load(TESTDATA / "ocr_result.invalid.json")
    with pytest.raises(ValidationError):
        OCRResult.model_validate(instance)
    assert Draft202012Validator(_load(SCHEMA_DIR / "ocr_result.schema.json")).is_valid(instance) is False


def test_invalid_report_rejected() -> None:
    """负例:日期格式、status 枚举、空 name / raw_text、value 类型 → pydantic 拒绝。"""
    instance = _load(TESTDATA / "report.invalid.json")
    with pytest.raises(ValidationError):
        Report.model_validate(instance)
    assert Draft202012Validator(_load(SCHEMA_DIR / "report.schema.json")).is_valid(instance) is False


def _report(status: str, items: list[dict]) -> dict:
    return {
        "report_type": "血常规", "report_date": "2026-10-01", "status": status, "items": items,
        "exam_name": None, "exam_name_ocr_line_numbers": None,
        "findings": None, "findings_ocr_line_numbers": None,
        "impression": None, "impression_ocr_line_numbers": None,
    }


def test_report_empty_items_must_be_failed() -> None:
    """不变量(viz-and-client 票 03):空 items ⇒ failed——「成功且零指标项」连构造都不可能。"""
    assert Report.model_validate(_report("failed", [])) is not None
    for status in ("success", "partial"):
        with pytest.raises(ValidationError, match="no items"):
            Report.model_validate(_report(status, []))


def test_report_with_items_accepts_every_status() -> None:
    """非空 items 与三种 status 的组合全部合法(不变量只约束空 items 一侧)。"""
    item = {"name": "血红蛋白", "value": "128", "unit": "g/L", "ref_range": "115-150",
            "flag": "normal", "low_confidence": False, "raw_text": "血红蛋白 128 g/L 115-150",
            "ocr_line_numbers": [0, 1, 2]}
    for status in ("success", "partial", "failed"):
        assert Report.model_validate(_report(status, [item])) is not None


def test_report_sections_pairing() -> None:
    """段落文本与行号成对:文本有 ⇒ 行号非空;文本 null ⇒ 行号 null。"""
    valid = _report("failed", [])
    valid.update({"findings": "宫内单活胎。", "findings_ocr_line_numbers": [3, 4]})
    assert Report.model_validate(valid) is not None
    schema = Draft202012Validator(_load(SCHEMA_DIR / "report.schema.json"))
    assert schema.is_valid(valid)

    missing_lines = {**valid, "findings": "宫内单活胎。", "findings_ocr_line_numbers": None}
    with pytest.raises(ValidationError, match="findings"):
        Report.model_validate(missing_lines)
    assert not schema.is_valid(missing_lines)

    empty_lines = {**valid, "findings": "宫内单活胎。", "findings_ocr_line_numbers": []}
    with pytest.raises(ValidationError, match="findings"):
        Report.model_validate(empty_lines)
    assert not schema.is_valid(empty_lines)

    orphan_lines = {**valid, "findings": None, "findings_ocr_line_numbers": [3]}
    with pytest.raises(ValidationError, match="findings"):
        Report.model_validate(orphan_lines)
    assert not schema.is_valid(orphan_lines)


def test_report_failed_with_sections_is_valid() -> None:
    """零指标项 failed 影像单与段落并存:空 items 不变量与段落成对约束互不冲突。"""
    failed = _report("failed", [])
    failed.update({"findings": "宫内单活胎。", "findings_ocr_line_numbers": [3]})
    assert Report.model_validate(failed) is not None
    assert Draft202012Validator(_load(SCHEMA_DIR / "report.schema.json")).is_valid(failed)
