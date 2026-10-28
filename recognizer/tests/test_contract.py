"""契约测试:schemas/ 下 JSON Schema 为源,示例 fixture 双校验(实施计划步骤 7-2)。"""

import json
from pathlib import Path

import pytest
from jsonschema import Draft202012Validator
from pydantic import ValidationError

from recognizer import paths
from recognizer.contract import (
    PANEL_ALLOWED_BY_REPORT_TYPE,
    PANEL_LABELS,
    PANEL_ORDER,
    OCRResult,
    Panel,
    Report,
)

SCHEMA_DIR = paths.SCHEMAS
TESTDATA = paths.CONTRACT_TESTDATA


def _load(path: Path) -> dict:
    return json.loads(path.read_text(encoding="utf-8"))


@pytest.mark.parametrize(
    ("schema_name", "examples"),
    [
        ("ocr_result.schema.json", ["ocr_result.example.json"]),
        ("report.schema.json", [
            "report.example.json", "report.null-date.example.json", "report.sections.example.json",
        ]),
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
        ("report.schema.json", [
            "report.example.json", "report.null-date.example.json", "report.sections.example.json",
        ]),
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


def test_schema_requires_panel_and_panels() -> None:
    """reportItem.panel / report.panels 是必填字段(spec §6.2)。"""
    schema = Draft202012Validator(_load(SCHEMA_DIR / "report.schema.json"))
    example = _load(TESTDATA / "report.example.json")
    assert schema.is_valid(example)
    without_item_panel = {**example, "items": [{k: v for k, v in example["items"][0].items() if k != "panel"}]}
    without_panels = {k: v for k, v in example.items() if k != "panels"}
    for broken in (without_item_panel, without_panels):
        assert not schema.is_valid(broken)
        with pytest.raises(ValidationError):
            Report.model_validate(broken)


def _item(name: str = "血红蛋白", panel: str = "cbc") -> dict:
    return {"name": name, "panel": panel, "value": "128", "unit": "g/L",
            "ref_range": "115-150", "flag": "normal", "low_confidence": False,
            "raw_text": "血红蛋白 128 g/L 115-150", "ocr_line_numbers": [0, 1, 2]}


def _report(
    status: str,
    items: list[dict],
    panels: list[str] | None = None,
    report_type: str = "临检",
) -> dict:
    if panels is None:
        panels = ["other"] if status == "failed" or not items else ["cbc"]
    return {
        "report_type": report_type,
        "report_date": "2026-10-01",
        "status": status,
        "panels": panels,
        "items": items,
    }


def test_report_empty_items_must_be_failed_or_narrative() -> None:
    """不变量:空 items 只允许 failed,或叙述型报告携带 sections(票 04)。"""
    assert Report.model_validate(_report("failed", [])) is not None
    for status in ("success", "partial"):
        with pytest.raises(ValidationError, match="no items"):
            Report.model_validate(_report(status, []))
    narrative = {
        **_report("success", [], ["imaging"]),
        "report_type": "影像",
        "sections": [{"kind": "findings", "text": "宫内单活胎。", "ocr_line_numbers": [3]}],
    }
    assert Report.model_validate(narrative) is not None


def test_report_with_items_accepts_every_status() -> None:
    """非空 items 与三种 status 的组合全部合法(不变量只约束空 items 一侧)。"""
    for status in ("success", "partial", "failed"):
        assert Report.model_validate(_report(status, [_item()])) is not None


def test_report_panels_must_match_item_aggregate() -> None:
    """panels 必须等于指标项面板去重后的排序聚合,不允许两份真相。"""
    with pytest.raises(ValidationError, match="do not match"):
        Report.model_validate(_report("success", [_item()], ["cbc", "liver"]))
    with pytest.raises(ValidationError, match="do not match"):
        Report.model_validate(_report("success", [_item()], ["other"]))


def test_report_panels_sorted_by_count_then_fixed_order() -> None:
    """命中项数降序;同数按 §5.3 固定优先级。"""
    items = [
        _item("尿酸", "uric_acid"),
        _item("总胆固醇", "lipid"),
        _item("二氧化碳", "electrolyte"),
        _item("肌酐", "kidney"),
        _item("尿素", "kidney"),
    ]
    report = Report.model_validate(
        _report("success", items, ["kidney", "lipid", "uric_acid", "electrolyte"], "生化")
    )
    assert report.panels == ["kidney", "lipid", "uric_acid", "electrolyte"]


def test_report_rejects_panel_outside_parent() -> None:
    """父子约束:临检不允许甲状腺,other 是唯一跨类别兜底。"""
    with pytest.raises(ValidationError, match="not allowed"):
        Report.model_validate(_report("success", [_item("促甲状腺素", "thyroid")], ["thyroid"]))
    assert Report.model_validate(_report("success", [_item("未知项目", "other")], ["other"])) is not None


def test_every_panel_has_label_and_parent() -> None:
    """16 个 code 都有 UI 标签,且至少出现在一个父类的允许集合中。"""
    assert set(PANEL_LABELS) == set(Panel)
    parents = {p for allowed in PANEL_ALLOWED_BY_REPORT_TYPE.values() for p in allowed}
    assert set(Panel) == parents
    assert PANEL_ORDER == (
        Panel.LIVER, Panel.KIDNEY, Panel.LIPID, Panel.GLUCOSE, Panel.URIC_ACID,
        Panel.ELECTROLYTE, Panel.VITANEMIA, Panel.THYROID, Panel.SEX_HORMONE,
        Panel.INFECTION, Panel.AUTOIMMUNE, Panel.COAG, Panel.CBC, Panel.URINALYSIS,
        Panel.IMAGING, Panel.OTHER,
    )


def test_panel_enum_and_order_match_schema() -> None:
    """枚举集合与 §5.3 顺序以 report.schema.json 为单一来源;改一处不会漏改三端。"""
    panel_schema = _load(SCHEMA_DIR / "report.schema.json")["$defs"]["panel"]
    assert set(panel_schema["enum"]) == {p.value for p in Panel}
    assert tuple(panel_schema["x-panel-order"]) == tuple(p.value for p in PANEL_ORDER)


def test_report_sections_optional_and_validated() -> None:
    """sections 可选;出现时 kind/text/行号均非空,且允许 failed 空 items 携带段落。"""
    schema = Draft202012Validator(_load(SCHEMA_DIR / "report.schema.json"))
    base = _report("failed", [])
    assert Report.model_validate(base) is not None
    assert schema.is_valid(base)
    assert Report.model_validate({**base, "sections": None}) is not None
    assert schema.is_valid({**base, "sections": None})

    valid = {**base, "sections": [
        {"kind": "findings", "text": "宫内单活胎。", "ocr_line_numbers": [3, 4]},
    ]}
    assert Report.model_validate(valid) is not None
    assert schema.is_valid(valid)

    for broken in (
        {**valid, "sections": [{"kind": "findings", "text": "宫内单活胎。", "ocr_line_numbers": []}]},
        {**valid, "sections": [{"kind": "", "text": "宫内单活胎。", "ocr_line_numbers": [3]}]},
        {**valid, "sections": [{"kind": "findings", "text": "", "ocr_line_numbers": [3]}]},
        {**valid, "sections": []},
    ):
        with pytest.raises(ValidationError):
            Report.model_validate(broken)
        assert not schema.is_valid(broken)


def test_report_serialization_omits_absent_sections() -> None:
    """非叙述型报告不输出 sections 键(而非 sections:null),既有 golden/消费方保持兼容。"""
    plain = Report.model_validate(_report("failed", []))
    assert "sections" not in json.loads(plain.model_dump_json())

    with_sections = Report.model_validate({
        **_report("failed", []),
        "sections": [{"kind": "impression", "text": "正常。", "ocr_line_numbers": [1]}],
    })
    dumped = json.loads(with_sections.model_dump_json())
    assert dumped["sections"] == [
        {"kind": "impression", "text": "正常。", "ocr_line_numbers": [1]}
    ]
