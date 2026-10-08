"""契约测试:schemas/ 下 JSON Schema 为源,示例 fixture 双校验(实施计划步骤 7-2)。"""

import json
from pathlib import Path

import pytest
from jsonschema import Draft202012Validator
from pydantic import ValidationError

from recognizer.contract import OCRResult, Report

REPO_ROOT = Path(__file__).resolve().parents[2]
SCHEMA_DIR = REPO_ROOT / "schemas"
TESTDATA = SCHEMA_DIR / "testdata"


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
