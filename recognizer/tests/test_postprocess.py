"""postprocess 单元测试:以数据文件 cbc_dict.yaml 决定词典行为。

锚定规则(实施计划 4.2/P2):
- 行按 OCR box 的 y 聚带、按 x 定列(表头角色:名称/结果/单位/参考值);
- 名称 = 词典名或别名,支持「名称熔断数值」(如 平均血红蛋白浓度324);
- 参考范围一律从检查单抽取,支持 115-150 / 4--10 / <3.5 / 男:0-15;
- 单位 ∈ 词典已知单位集,否则该项 low_confidence(值仍保留原文供人工核对);
- 任一组成行 score < 0.8 → 该项 low_confidence、报告 ≥partial;
- 日期:标签行(检验时间 > … > 报告时间)抽取,解析失败时用 date_hint 兜底;
- 类别失败或无指标 → failed;
- 类别命中且有产出:日期缺失/低置信/数值缺失 → partial,否则 success。
"""

import json
from pathlib import Path
from typing import Any

import pytest

from recognizer.contract import OCRResult
from recognizer.postprocess import run_postprocess

# ---------- 合成 OCRResult 构造器 ----------


def mk_ocr(cells: list[dict[str, Any]]) -> OCRResult:
    """cells: [{x,y,w,h,text,score}] → 头左上角矩形盒子的 OCRResult。"""
    txts: list[str] = []
    boxes: list[list[list[float]]] = []
    scores: list[float] = []
    for c in cells:
        x, y, w, h = c["x"], c["y"], c.get("w", 60), c.get("h", 20)
        txts.append(c["text"])
        boxes.append([[x, y], [x + w, y], [x + w, y + h], [x, y + h]])
        scores.append(c.get("score", 1.0))
    return OCRResult.model_validate(
        {
            "txts": txts,
            "boxes": boxes,
            "scores": scores,
            "elapse": 0.0,
            "elapse_list": [0.0] * len(txts),
            "engine": "onnxruntime",
            "model_info": {"det": "d", "cls": "c", "rec": "r"},
        }
    )


def header_row(dated: bool = True) -> list[dict[str, Any]]:
    cells = [{"x": 100, "y": -60, "text": "市人民医院血常规报告单"}]
    if dated:
        cells.append({"x": 600, "y": -60, "text": "报告时间：2026-10-01 09:00"})
    return cells + [
        {"x": 20, "y": 0, "text": "序号"},
        {"x": 160, "y": 0, "text": "项目名称"},
        {"x": 360, "y": 0, "text": "结果"},
        {"x": 470, "y": 0, "text": "单位"},
        {"x": 580, "y": 0, "text": "参考值"},
    ]


def row(y: float, name: str, value: str, unit: str = "", ref: str = "", score: float = 1.0) -> list[dict[str, Any]]:
    cells = [
        {"x": 12, "y": y, "w": 20, "text": "1"},
        {"x": 150, "y": y, "w": 90, "text": name, "score": score},
        {"x": 355, "y": y, "text": value, "w": 70},
    ]
    if unit:
        cells.append({"x": 465, "y": y, "text": unit, "w": 60})
    if ref:
        cells.append({"x": 575, "y": y, "text": ref, "w": 80})
    return cells


def item_of(r, name: str):
    return next(i for i in r.items if i.name == name)


# ---------- 行解析 / 词典匹配 ----------


def test_basic_row_to_item() -> None:
    ocr = mk_ocr(
        header_row()
        + row(30, "血红蛋白", "128", unit="g/L", ref="115-150")
    )
    r = run_postprocess(ocr)
    assert r.status == "success"
    it = item_of(r, "血红蛋白")
    assert it.value == 128
    assert it.unit == "g/L"
    assert it.ref_range == "115-150"
    assert it.flag == "normal"
    assert it.low_confidence is False
    assert "血红蛋白" in it.raw_text


def test_alias_and_serial_strip() -> None:
    ocr = mk_ocr(header_row() + row(30, "3 白细胞", "7.33", unit="*10^9/L", ref="4-10"))
    r = run_postprocess(ocr)
    it = item_of(r, "白细胞计数")
    assert it.value == 7.33
    assert it.flag == "normal"


def test_fused_name_value() -> None:
    """名称熔断数值(cbc_02/05 版式):平均红细胞血红蛋白浓度324。"""
    ocr = mk_ocr(header_row() + row(30, "平均红细胞血红蛋白浓度324", "g/L", unit="g/L", ref="316-354"))
    r = run_postprocess(ocr)
    it = item_of(r, "平均红细胞血红蛋白浓度")
    assert it.value == 324
    assert it.unit == "g/L"
    assert it.flag == "normal"


def test_fused_unit_ref_in_unit_cell() -> None:
    """单位熔断参考范围(cbc_02 版式):*10~9/L 3.5-9.5。"""
    ocr = mk_ocr(header_row() + row(30, "*白细胞计数", "6.01", unit="*10~9/L 3.5-9.5"))
    r = run_postprocess(ocr)
    it = item_of(r, "白细胞计数")
    assert it.value == 6.01
    assert it.unit == "*10~9/L"
    assert it.ref_range == "3.5-9.5"


def test_fused_ref_unit_in_ref_cell() -> None:
    """参考范围熔断单位(cbc_04 版式):130--175 g/L。"""
    ocr = mk_ocr(header_row() + row(30, "血红蛋白", "118", unit="", ref="130--175 g/L"))
    r = run_postprocess(ocr)
    it = item_of(r, "血红蛋白")
    assert it.value == 118
    assert it.unit == "g/L"
    assert it.ref_range == "130--175"
    assert it.flag == "low"


def test_double_dash_range() -> None:
    ocr = mk_ocr(header_row() + row(30, "红细胞", "4.76", unit="*10^12/L", ref="3.5--5.5"))
    it = item_of(run_postprocess(ocr), "红细胞计数")
    assert it.ref_range == "3.5--5.5"
    assert it.flag == "normal"


def test_open_range_compare() -> None:
    for ref, value, want in [("<8", 4.0, "normal"), ("<8", 9.0, "high"), (">7.5", 9.0, "normal"), (">7.5", 3.0, "low")]:
        ocr = mk_ocr(header_row() + row(30, "C反应蛋白", str(value), unit="mg/L", ref=ref))
        assert item_of(run_postprocess(ocr), "C反应蛋白").flag == want


def test_sex_prefixed_range() -> None:
    ocr = mk_ocr(header_row() + row(30, "血沉", "8", unit="", ref="男:0-15"))
    it = item_of(run_postprocess(ocr), "血沉")
    assert it.ref_range == "男:0-15"
    assert it.flag == "normal"


def test_no_ref_or_unparsable_ref_flag_unknown() -> None:
    ocr = mk_ocr(
        header_row()
        + row(30, "血红蛋白", "128", unit="g/L", ref="")
        + row(50, "白细胞", "7.3", unit="*10^9/L", ref="优良")
    )
    r = run_postprocess(ocr)
    assert item_of(r, "血红蛋白").flag == "unknown"
    assert item_of(r, "白细胞计数").flag == "unknown"


def test_missing_value_flag_unknown_keeps_item() -> None:
    """名称命中但结果列无数值 → value=null、flag=unknown、报告 partial(指标缺失)。"""
    ocr = mk_ocr(header_row() + row(30, "血红蛋白", "", unit="g/L", ref="115-150"))
    r = run_postprocess(ocr)
    it = item_of(r, "血红蛋白")
    assert it.value is None
    assert it.flag == "unknown"
    assert r.status == "partial"


# ---------- 低置信 ----------


def test_low_score_line_marks_low_confidence_and_partial() -> None:
    ocr = mk_ocr(header_row() + row(30, "血红蛋白", "128", unit="g/L", ref="115-150", score=0.79))
    r = run_postprocess(ocr)
    it = item_of(r, "血红蛋白")
    assert it.low_confidence is True
    assert r.status == "partial"


def test_score_threshold_boundary() -> None:
    """决策 #7:score < 0.8 判低置信;恰为 0.8 不是。"""
    ocr80 = mk_ocr(header_row() + row(30, "血红蛋白", "128", unit="g/L", ref="115-150", score=0.80))
    assert run_postprocess(ocr80).status == "success"
    ocr79 = mk_ocr(header_row() + row(30, "血红蛋白", "128", unit="g/L", ref="115-150", score=0.7999))
    assert run_postprocess(ocr79).status == "partial"


def test_garbled_unit_marks_low_confidence_keeps_raw() -> None:
    """单位不在词典已知单位集 → 保留原文 + low_confidence(cbc_01 的 109/L 版式)。"""
    ocr = mk_ocr(header_row() + row(30, "血小板", "215", unit="109/L", ref="100--300"))
    r = run_postprocess(ocr)
    it = item_of(r, "血小板")
    assert it.value == 215
    assert it.unit == "109/L"  # 保留原文供人工核对
    assert it.low_confidence is True
    assert r.status == "partial"


# ---------- 日期 ----------


def test_date_priority_and_formats() -> None:
    """检验时间 > 采集时间 > 核收时间 > 报告时间 > 打印时间 > 申请时间;支持 / 与 年月日。"""
    cells = header_row(dated=False) + row(30, "血红蛋白", "128", unit="g/L", ref="115-150")
    cells += [{"x": 40, "y": 200, "text": "报告时间：2009-03-21 09:15:32"}]
    cells += [{"x": 300, "y": 220, "text": "采集时间：2023-09-08 16:38"}]
    cells += [{"x": 500, "y": 240, "text": "检验日期：2024/08/19 07:51"}]
    assert run_postprocess(mk_ocr(cells)).report_date == "2024-08-19"
    cells2 = header_row(dated=False) + row(30, "血红蛋白", "128", unit="g/L", ref="115-150")
    cells2 += [{"x": 40, "y": 200, "text": "打印时间：2023-09-26 22:27:02"}]
    cells2 += [{"x": 100, "y": 220, "text": "核收时间：2009-03-21 08:45"}]
    assert run_postprocess(mk_ocr(cells2)).report_date == "2009-03-21"  # 打印时间最低优先级


def test_date_hint_fallback_only_when_unparsed() -> None:
    cells = header_row(dated=False) + row(30, "血红蛋白", "128", unit="g/L", ref="115-150")
    cells += [{"x": 40, "y": 200, "text": "报告时间：2009-03-21 09:15:32"}]
    r = run_postprocess(mk_ocr(cells), date_hint="2026-10-01")
    assert r.report_date == "2009-03-21"  # 解析成功则不用 hint(决策 #6 补录语义)

    cells_nod = header_row(dated=False) + row(30, "血红蛋白", "128", unit="g/L", ref="115-150")
    r = run_postprocess(mk_ocr(cells_nod), date_hint="2026-10-01")
    assert r.report_date == "2026-10-01"
    assert r.status == "success"  # 补录后日期不再是 partial 缺口


def test_no_date_and_no_hint_partial() -> None:
    ocr = mk_ocr(header_row(dated=False) + row(30, "血红蛋白", "128", unit="g/L", ref="115-150"))
    r = run_postprocess(ocr)
    assert r.report_date is None
    assert r.status == "partial"


# ---------- 类别 / 状态 ----------


def test_category_fail_failed_status() -> None:
    ocr = mk_ocr([{"x": 100, "y": 0, "text": "尿常规报告单"}, {"x": 200, "y": 30, "text": "尿糖"}])
    r = run_postprocess(ocr)
    assert r.status == "failed"
    assert r.items == []
    assert r.report_type != ""  # 仍留痕落库,类型给占位名


def test_empty_ocr_failed() -> None:
    r = run_postprocess(mk_ocr([]))
    assert r.status == "failed"


def test_title_keyword_detects_category() -> None:
    cells = header_row() + row(30, "血红蛋白", "128", unit="g/L", ref="115-150")
    cells += [{"x": 100, "y": -40, "text": "市人民医院血常规报告单"}]
    r = run_postprocess(mk_ocr(cells))
    assert r.report_type == "血常规"


# ---------- golden(真实样本,OCR 结果来自 golden 文件,跨架构确定性) ----------

REPO_ROOT = Path(__file__).resolve().parents[2]
EXPECTED_OCR = REPO_ROOT / "samples" / "expected" / "ocr"
EXPECTED_REPORT = REPO_ROOT / "samples" / "expected" / "report"

_sample_ids = sorted(p.stem for p in EXPECTED_OCR.glob("*.json")) if EXPECTED_OCR.exists() else []


@pytest.mark.skipif(not EXPECTED_REPORT.exists(), reason="no golden files under samples/expected/report/")
@pytest.mark.parametrize("sample_id", _sample_ids)
def test_report_matches_golden(sample_id: str) -> None:
    ocr_data = json.loads((EXPECTED_OCR / f"{sample_id}.json").read_text(encoding="utf-8"))
    report = run_postprocess(OCRResult.model_validate(ocr_data))
    golden = json.loads((EXPECTED_REPORT / f"{sample_id}.json").read_text(encoding="utf-8"))
    assert json.loads(report.model_dump_json()) == golden
