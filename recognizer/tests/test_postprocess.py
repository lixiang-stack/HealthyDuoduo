"""postprocess 单元测试:以数据文件 cbc_dict.yaml 决定词典行为。

锚定规则(实施计划 4.2/P2):
- 行按 OCR box 的 y 聚带、按 x 定列(表头角色:名称/结果/单位/参考值);
- 名称 = 词典名或别名,支持「名称熔断数值」(如 平均血红蛋白浓度324);
- 参考范围一律从检查单抽取,支持 115-150 / 4--10 / <3.5 / 男:0-15;
- 单位 ∈ 词典已知单位集,否则该项 low_confidence(值仍保留原文供人工核对);
- 任一组成行 score < 0.8 → 该项 low_confidence;占比过半(≥50%)→ 整单 partial;
- 日期:标签行(检验时间 > … > 报告时间)抽取,解析失败时用 date_hint 兜底;
- 类别失败或无指标 → failed;
- 类别命中且有产出:数值缺失/日期缺失 → partial,否则 success(低置信留在条目标记);
- 叙述型报告(影像)以正文段落为准:findings/impression 至少一段 → success,否则 failed,
  不产出指标项(票 04)。
"""

import json
from typing import Any

import pytest
import yaml

from recognizer import paths
from recognizer import postprocess as _postprocess
from recognizer.contract import OCRResult, Panel
from recognizer.golden import date_hint_for, image_bytes_for
from recognizer.postprocess import _build_dict, run_postprocess

# 真实注册表快照(分发测试在 monkeypatch 前捕获原词典)
_load_dicts = _postprocess.load_dicts

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
    for item in r.items:
        if item.name == name:
            return item
    # 标准名迁移后,旧测试仍用单据/旧规范名定位;按词典 synonyms/aliases 反查。
    for d in _postprocess.load_dicts():
        for dict_item in d.items:
            legacy = {s["term"] for s in dict_item.synonyms} | set(dict_item.aliases)
            if name in legacy:
                for item in r.items:
                    if item.name == dict_item.name:
                        return item
    raise StopIteration


def _golden_report(sample_id: str, **kwargs: Any):
    """对 samples/expected/ocr 的真实 golden 重跑后处理(规则改动的回归入口)。"""
    ocr = OCRResult.model_validate(
        json.loads((paths.EXPECTED_OCR / f"{sample_id}.json").read_text(encoding="utf-8"))
    )
    return run_postprocess(ocr, date_hint=date_hint_for(sample_id), **kwargs)


def section_of(r, kind: str):
    return next(s for s in r.sections or [] if s.kind == kind)


# ---------- 行解析 / 词典匹配 ----------


def test_basic_row_to_item() -> None:
    ocr = mk_ocr(
        header_row()
        + row(30, "血红蛋白", "128", unit="g/L", ref="115-150")
    )
    r = run_postprocess(ocr)
    assert r.status == "success"
    it = item_of(r, "血红蛋白")
    assert it.value == "128"
    assert it.unit == "g/L"
    assert it.ref_range == "115-150"
    assert it.flag == "normal"
    assert it.low_confidence is False
    assert "血红蛋白" in it.raw_text
    assert it.panel == "cbc"
    assert r.panels == ["cbc"]


def test_alias_and_serial_strip() -> None:
    ocr = mk_ocr(header_row() + row(30, "3 白细胞", "7.33", unit="*10^9/L", ref="4-10"))
    r = run_postprocess(ocr)
    it = item_of(r, "白细胞计数")
    assert it.value == "7.33"
    assert it.flag == "normal"


def test_scalar_zero_in_ref_column_pairs_as_reference() -> None:
    """参考列单标量 0(有核红细胞)按 ref 收下,不再因 _decompose 把它当 value 而漏配。"""
    ocr = mk_ocr(header_row() + row(30, "有核红细胞绝对值", "0.00", ref="0"))
    r = run_postprocess(ocr)
    it = item_of(r, "有核红细胞绝对值")
    assert it.value == "0.00"
    assert it.ref_range == "0"
    assert it.flag == "normal"


def test_value_column_comparison_keeps_real_ref() -> None:
    """值列纯比较式(<2.00,结果低于检出限)不进 ref 位,参考列真格(0-30)正常配对。

    实测 webwxgetmsgimg_1 抗dsDNA抗体:值列 "<2.00" 曾把参考列的 "0-30" 挤掉,
    上报成 ref_range="<2.00" + value=None;正确语义是 value=None(检出限非数值)、
    ref_range="0-30"、raw_text 保留 "<2.00"。
    """
    ocr = mk_ocr(
        header_row()
        + row(30, "16 抗dsDNA抗体(dsDNA)", "<2.00", unit="IU/mL", ref="0-30")
        + row(50, "抗C1q抗体(C1q)", "0.47", unit="U/mL", ref="0-10")
    )
    r = run_postprocess(ocr)
    it = item_of(r, "抗dsDNA抗体")
    # string 契约:检出限比较式 '<2.00' 直接作为 value(印刷原样);参考列真值 0-30 正常配对。
    assert it.value == "<2.00"
    assert it.ref_range == "0-30"
    assert "<2.00" in it.raw_text
    # 对照:比较式印在参考列时仍是合法参考区间(A 类,如 总胆红素 "<23")
    it2 = item_of(r, "抗C1q抗体")
    assert it2.value == "0.47"
    assert it2.ref_range == "0-10"


def test_name_decoration_marker_and_method_suffix() -> None:
    """名称印形装饰(互认标记 / 方法后缀 / 尾随缩写)只影响印形,不影响规范名匹配。

    真实院区版式:「血红蛋白(8-HR)」「白细胞计数测定(9-HR)」应仍命中词典项。
    """
    ocr = mk_ocr(
        header_row()
        + row(30, "血红蛋白(8-HR)", "128", unit="g/L", ref="115-150")
        + row(50, "白细胞计数测定(9-HR)", "7.33", unit="*10^9/L", ref="4-10")
        + row(70, "红细胞计数测定(9-HR)RBC", "4.76", unit="*10^12/L", ref="3.5--5.5")
    )
    r = run_postprocess(ocr)
    assert item_of(r, "血红蛋白").value == "128"
    assert item_of(r, "白细胞计数").value == "7.33"
    assert item_of(r, "红细胞计数").value == "4.76"


def test_two_name_columns_per_block_pair_within_block() -> None:
    """双栏版式:每块含「项目名称(缩写)+检验项目(中文)」两个名称列,按表头角色序列的
    最小周期分块,块内配对。若不按周期分块,缩写列会把半栏下标整体错位,参考/单位串到
    相邻块(实测 lft_09)。"""
    cells = [
        {"x": 100, "y": -30, "text": "某院肝肾功能报告单"},
        {"x": 148, "y": 0, "text": "项目名称"},
        {"x": 328, "y": 0, "text": "检验项目"},
        {"x": 487, "y": 0, "text": "结果"},
        {"x": 728, "y": 0, "text": "单位 结果提示参考范围"},
        {"x": 983, "y": 0, "text": "项目名称"},
        {"x": 1150, "y": 0, "text": "检验项目"},
        {"x": 1328, "y": 0, "text": "结果"},
        {"x": 1541, "y": 0, "text": "单位结果提示参考范围"},
        {"x": 96, "y": 30, "text": "ALT"},
        {"x": 260, "y": 30, "w": 130, "text": "丙氨酸氨基转移酶"},
        {"x": 476, "y": 30, "text": "13.8"},
        {"x": 587, "y": 30, "text": "U/L"},
        {"x": 823, "y": 30, "text": "7.0-40.0"},
        {"x": 96, "y": 62, "text": "T-Bil"},
        {"x": 260, "y": 62, "w": 130, "text": "总胆红素"},
        {"x": 476, "y": 62, "text": "14.08"},
        {"x": 587, "y": 62, "text": "μmol/L"},
        {"x": 823, "y": 62, "text": "≤21.00"},
        {"x": 931, "y": 30, "text": "AST"},
        {"x": 1095, "y": 30, "w": 130, "text": "天门冬氨酸氨基转移酶"},
        {"x": 1310, "y": 30, "text": "12.5"},
        {"x": 1421, "y": 30, "text": "U/L"},
        {"x": 1657, "y": 30, "text": "13.0-35.0"},
    ]
    r = run_postprocess(mk_ocr(cells))
    alt = item_of(r, "丙氨酸转氨酶")
    assert (alt.value, alt.unit, alt.ref_range) == ("13.8", "U/L", "7.0-40.0")
    tbil = item_of(r, "总胆红素")
    assert (tbil.value, tbil.ref_range) == ("14.08", "≤21.00")
    ast = item_of(r, "天冬氨酸转氨酶")
    assert (ast.value, ast.ref_range) == ("12.5", "13.0-35.0")


def test_offset_columns_pair_by_header_baseline() -> None:
    """名称列 OCR box 整体低于结果/单位/参考列时,按表头行基准对齐列 y,参考范围不串行。

    真实院区版式实测:同一视觉行内名称格比结果/参考格低约半行,若直接按中心距离
    配对会把下一行的参考范围配给上一行。
    """
    cells = [
        {"x": 100, "y": -30, "text": "某院血常规报告单"},
        {"x": 150, "y": 12, "text": "序号 项目名称", "h": 40},
        {"x": 360, "y": 0, "text": "结果", "h": 30},
        {"x": 470, "y": 0, "text": "单位", "h": 30},
        {"x": 580, "y": 0, "text": "参考值", "h": 30},
        {"x": 150, "y": 40, "w": 120, "text": "血红蛋白", "h": 40},
        {"x": 355, "y": 26, "text": "128", "h": 30},
        {"x": 465, "y": 26, "text": "g/L", "h": 30},
        {"x": 575, "y": 26, "text": "115-150", "h": 30},
        {"x": 150, "y": 72, "w": 120, "text": "白细胞", "h": 40},
        {"x": 355, "y": 58, "text": "7.3", "h": 30},
        {"x": 465, "y": 58, "text": "*10^9/L", "h": 30},
        {"x": 575, "y": 58, "text": "4-10", "h": 30},
    ]
    r = run_postprocess(mk_ocr(cells))
    assert item_of(r, "血红蛋白").ref_range == "115-150"
    assert item_of(r, "血红蛋白").flag == "normal"
    assert item_of(r, "白细胞计数").ref_range == "4-10"
    assert item_of(r, "白细胞计数").flag == "normal"


def test_qualitative_short_dash_value_and_ref_not_tested() -> None:
    """值与参考都印裸短横 "-" → 未检测(不是 normal,也不是 unknown)。"""
    cells = [
        {"x": 100, "y": -60, "text": "尿常规报告单"},
        {"x": 20, "y": 0, "text": "序号"},
        {"x": 160, "y": 0, "text": "项目名称"},
        {"x": 360, "y": 0, "text": "结果"},
        {"x": 470, "y": 0, "text": "单位"},
        {"x": 580, "y": 0, "text": "参考值"},
    ]
    for idx, name in enumerate(["潜血", "白细胞酯酶", "蛋白质", "葡萄糖", "酮体"]):
        cells += row(30 + idx * 32, name, "-", ref="-")
    r = run_postprocess(mk_ocr(cells))
    assert item_of(r, "潜血").flag == "not_tested"
    assert item_of(r, "潜血").value == "-"
    assert item_of(r, "潜血").ref_range == "-"
    assert item_of(r, "潜血").flag_reason is None


def test_fused_name_value() -> None:
    """名称熔断数值(cbc_02/05 版式):平均红细胞血红蛋白浓度324。"""
    ocr = mk_ocr(header_row() + row(30, "平均红细胞血红蛋白浓度324", "g/L", unit="g/L", ref="316-354"))
    r = run_postprocess(ocr)
    it = item_of(r, "平均红细胞血红蛋白浓度")
    assert it.value == "324"
    assert it.unit == "g/L"
    assert it.flag == "normal"


def test_fused_unit_ref_in_unit_cell() -> None:
    """单位熔断参考范围(cbc_02 版式):*10~9/L 3.5-9.5;单位归一为规范印形。"""
    ocr = mk_ocr(header_row() + row(30, "*白细胞计数", "6.01", unit="*10~9/L 3.5-9.5"))
    r = run_postprocess(ocr)
    it = item_of(r, "白细胞计数")
    assert it.value == "6.01"
    assert it.unit == "10*9/L"
    assert it.unit_display == "10^9/L"
    assert it.ref_range == "3.5-9.5"


def test_fused_ref_unit_in_ref_cell() -> None:
    """参考范围熔断单位(cbc_04 版式):130--175 g/L。"""
    ocr = mk_ocr(header_row() + row(30, "血红蛋白", "118", unit="", ref="130--175 g/L"))
    r = run_postprocess(ocr)
    it = item_of(r, "血红蛋白")
    assert it.value == "118"
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


def test_low_score_line_marks_low_confidence_not_partial() -> None:
    """低置信(score<0.8)留在条目 low_confidence 标记上;占比未过半时不压整单
    (真实样本要求结果齐备即 success,人工核对看 per-item 标记)。"""
    ocr = mk_ocr(
        header_row()
        + row(30, "血红蛋白", "128", unit="g/L", ref="115-150", score=0.79)
        + row(50, "白细胞", "6.01", unit="*10^9/L", ref="4-10")
        + row(70, "红细胞", "4.76", unit="*10^12/L", ref="3.5--5.5")
    )
    r = run_postprocess(ocr)
    assert item_of(r, "血红蛋白").low_confidence is True
    assert item_of(r, "白细胞计数").low_confidence is False
    assert item_of(r, "红细胞计数").low_confidence is False
    assert r.status == "success"


def test_low_score_majority_marks_partial() -> None:
    """低置信占比 ≥ 50% → 整单 partial(过半指标不可靠,降级人工甄别)。"""
    ocr = mk_ocr(
        header_row()
        + row(30, "血红蛋白", "128", unit="g/L", ref="115-150", score=0.79)
        + row(50, "白细胞", "6.01", unit="*10^9/L", ref="4-10", score=0.79)
    )
    r = run_postprocess(ocr)
    assert r.status == "partial"
    # 整单 partial 为低置信所致:value 齐备、条目标记仍在
    assert all(it.value is not None for it in r.items)
    assert all(it.low_confidence for it in r.items)
    # 恰一半(1/2)同样触发(≥50%)
    r_half = run_postprocess(
        mk_ocr(
            header_row()
            + row(30, "血红蛋白", "128", unit="g/L", ref="115-150", score=0.79)
            + row(50, "白细胞", "6.01", unit="*10^9/L", ref="4-10")
        )
    )
    assert r_half.status == "partial"


def test_score_threshold_boundary() -> None:
    """决策 #7:score < 0.8 打 low_confidence 标;恰为 0.8 不是(不再影响 status)。"""
    ocr80 = mk_ocr(
        header_row() + row(30, "血红蛋白", "128", unit="g/L", ref="115-150", score=0.80)
        + row(50, "白细胞", "6.01", unit="*10^9/L", ref="4-10", score=0.80)
    )
    r80 = run_postprocess(ocr80)
    assert r80.status == "success"
    assert item_of(r80, "血红蛋白").low_confidence is False
    assert item_of(r80, "白细胞计数").low_confidence is False
    ocr79 = mk_ocr(
        header_row() + row(30, "血红蛋白", "128", unit="g/L", ref="115-150", score=0.7999)
        + row(50, "白细胞", "6.01", unit="*10^9/L", ref="4-10", score=0.7999)
    )
    r79 = run_postprocess(ocr79)
    assert item_of(r79, "血红蛋白").low_confidence is True
    assert item_of(r79, "白细胞计数").low_confidence is True
    assert r79.status == "partial"


def test_garbled_unit_kept_raw_without_confidence_mark() -> None:
    """OCR 损耗单位(109/L)归一为注册印形 10^9/L;low_confidence 仅由 score<0.8 决定(决策 #7)。"""
    ocr = mk_ocr(header_row() + row(30, "血小板", "215", unit="109/L", ref="100--300"))
    r = run_postprocess(ocr)
    it = item_of(r, "血小板")
    assert it.value == "215"
    assert it.unit == "10*9/L"  # canonical UCUM;展示印形见 unit_display
    assert it.unit_display == "10^9/L"
    assert "109/L" in it.raw_text
    assert it.low_confidence is False
    assert r.status == "success"


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


def test_date_imaging_labels() -> None:
    """影像单标准印法「检查日期 / 检查时间」:检验单词表之外的新标签(us_03/bd39 形态)。"""
    cells = [
        {"x": 100, "y": -40, "text": "超声检查报告单"},
        {"x": 100, "y": 0, "text": "检查日期：2024-09-1514:37:47"},
        {"x": 100, "y": 30, "text": "股骨长 22mm"},
    ]
    assert run_postprocess(mk_ocr(cells)).report_date == "2024-09-15"

    cells_time = [
        {"x": 100, "y": -40, "text": "超声检查报告单"},
        {"x": 100, "y": 0, "text": "检查时间：2024-09-15 14:37:47"},
        {"x": 100, "y": 30, "text": "股骨长 22mm"},
    ]
    assert run_postprocess(mk_ocr(cells_time)).report_date == "2024-09-15"


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


def test_date_split_cells_large_photo_scale() -> None:
    """真实照片形态(维护者实测 3316px 宽,2026-10-18):标签与日期分格且盒间距
    322px——按小图校准的 300px 旧门限恰好越界,日期丢失。框值取自实测 OCR
    (检验时间 Δcx=322.6/Δcy=-7.5/格高 73;页面内容宽 3278)。"""
    cells = header_row(dated=False) + row(30, "血红蛋白", "128", unit="g/L", ref="115-150")
    cells += [
        {"x": 1217, "y": 1750, "w": 276, "h": 73, "text": "检验时间："},
        {"x": 1521, "y": 1747, "w": 312, "h": 65, "text": "2026/05/16"},
        {"x": 2519, "y": 1727, "w": 571, "h": 72, "text": "2026/05/16 12:38:27"},  # 页面右缘(撑内容宽度)
    ]
    assert run_postprocess(mk_ocr(cells)).report_date == "2026-05-16"


def test_date_standalone_slash_line() -> None:
    """独立日期行兜底与 _DATE_RE 同分隔符口径:斜杠形态收,非法月日不猜。"""
    cells = header_row(dated=False) + row(30, "血红蛋白", "128", unit="g/L", ref="115-150")
    cells += [{"x": 40, "y": 200, "text": "2026/05/16"}]
    assert run_postprocess(mk_ocr(cells)).report_date == "2026-05-16"

    cells_invalid = header_row(dated=False) + row(30, "血红蛋白", "128", unit="g/L", ref="115-150")
    cells_invalid += [{"x": 40, "y": 200, "text": "2026/13/40"}]
    assert run_postprocess(mk_ocr(cells_invalid)).report_date is None


# ---------- 类别 / 状态 ----------


def test_category_fail_failed_status() -> None:
    ocr = mk_ocr([{"x": 100, "y": 0, "text": "尿常规报告单"}, {"x": 200, "y": 30, "text": "尿糖"}])
    r = run_postprocess(ocr)
    assert r.status == "failed"
    assert r.items == []
    assert r.report_type != ""  # 仍留痕落库,类型给占位名
    assert r.panels == ["other"]


def test_empty_ocr_failed() -> None:
    r = run_postprocess(mk_ocr([]))
    assert r.status == "failed"


def test_title_keyword_detects_category() -> None:
    cells = header_row() + row(30, "血红蛋白", "128", unit="g/L", ref="115-150")
    cells += [{"x": 100, "y": -40, "text": "市人民医院血常规报告单"}]
    r = run_postprocess(mk_ocr(cells))
    assert r.report_type == "临检"


# ---------- 多词典注册表分发(P3:词典数据文件每个类别一份,分发语义见 _select_category) ----------


_CBC_ROW_NAMES = ["白细胞计数", "红细胞计数", "血红蛋白", "红细胞压积", "血小板",
                  "淋巴细胞比率", "中性细胞比率", "单核细胞比率", "血小板比积", "淋巴细胞数"]

_URINE_ITEM_NAMES = ["尿胆原", "尿糖", "尿蛋白", "尿酮体", "尿潜血",
                     "尿亚硝酸盐", "尿胆红素", "尿比重", "尿pH", "尿白细胞"]


def _urine_dict(tmp_path, min_matched: int):
    """尿常规词典:数据文件格式与真实词典一致,经 _build_dict 加载(注册表分发测试用)。"""
    raw = {
        "report_type": "尿常规",
        "category": {"title_keywords": ["尿常规"], "min_matched_items": min_matched},
        "items": [{"name": n, "aliases": [n], "units": ["mmol/L"]} for n in _URINE_ITEM_NAMES],
    }
    path = tmp_path / "test_urine_dict.yaml"
    path.write_text(yaml.safe_dump(raw, allow_unicode=True), encoding="utf-8")
    return _build_dict(path)


def _cbc_rows(y_start: float = 30.0, step: float = 25.0) -> list[dict[str, Any]]:
    cells: list[dict[str, Any]] = []
    for i, name in enumerate(_CBC_ROW_NAMES):
        cells += row(y_start + step * i, name, "1", unit="g/L", ref="1-2")
    return cells


def _bare_roles() -> list[dict[str, Any]]:
    """仅表头角色行(无报告类别标题行)。"""
    return [
        {"x": 20, "y": 0, "text": "序号"},
        {"x": 160, "y": 0, "text": "项目名称"},
        {"x": 360, "y": 0, "text": "结果"},
        {"x": 470, "y": 0, "text": "单位"},
        {"x": 580, "y": 0, "text": "参考值"},
    ]


def test_registry_content_overrides_title_hit(monkeypatch, tmp_path) -> None:
    """A1 内容优先:仅标题命中的类别(0 项配对)不得覆盖命中项数达标的类别(临检 10 项)。"""
    urine = _urine_dict(tmp_path, min_matched=3)
    real = _load_dicts()
    monkeypatch.setattr(_postprocess, "load_dicts", lambda: (urine, *real))
    cells = [{"x": 100, "y": -60, "text": "市第二医院尿常规报告单"}] + _bare_roles() + _cbc_rows()
    r = run_postprocess(mk_ocr(cells))
    assert r.report_type == "临检"


def test_registry_title_fallback_when_no_dict_qualifies(monkeypatch, tmp_path) -> None:
    """无词典达标时回退标题命中:标题词也可能印在正文(如 glu_03 的糖化血红蛋白)。"""
    urine = _urine_dict(tmp_path, min_matched=5)
    real = _load_dicts()
    monkeypatch.setattr(_postprocess, "load_dicts", lambda: (urine, *real))
    cells = _bare_roles() + row(30, "尿胆原", "5.0", unit="mmol/L", ref="1-10")
    cells += [{"x": 100, "y": -60, "text": "市第二医院尿常规报告单"}]
    r = run_postprocess(mk_ocr(cells))
    assert r.report_type == "尿常规"


def test_registry_body_anchor_outranks_item_count(monkeypatch, tmp_path) -> None:
    """排序键:达标后正文锚命中优先于指标项命中(票 04)。"""
    table = _urine_dict(tmp_path, min_matched=2)
    narrative_raw = {
        "report_type": "影像",
        "category": {
            "title_keywords": ["超声"],
            "mode": "narrative",
            "min_section_hits": 1,
            "section_anchors": {"findings": ["检查所见"]},
        },
        "items": [],
    }
    path = tmp_path / "test_imaging_dict.yaml"
    path.write_text(yaml.safe_dump(narrative_raw, allow_unicode=True), encoding="utf-8")
    imaging = _build_dict(path)
    monkeypatch.setattr(_postprocess, "load_dicts", lambda: (table, imaging))
    cells = _bare_roles() + row(30, "尿胆原", "5.0", unit="mmol/L", ref="1-10")
    cells += row(55, "尿糖", "5.0", unit="mmol/L", ref="1-10")
    cells += [{"x": 100, "y": 90, "text": "检查所见：宫内单活胎。"}]
    r = run_postprocess(mk_ocr(cells))
    assert r.report_type == "影像"
    assert r.items == []


def test_registry_title_region_breaks_qualified_tie(monkeypatch, tmp_path) -> None:
    """两类均达标且命中项数并列:标题(限标题区)命中者胜出。"""
    def build(report_type: str, title_kw: str, names: list[str]):
        raw = {
            "report_type": report_type,
            "category": {"title_keywords": [title_kw], "min_matched_items": 2},
            "items": [{"name": n, "aliases": [n], "units": ["mmol/L"]} for n in names],
        }
        path = tmp_path / f"{report_type}.yaml"
        path.write_text(yaml.safe_dump(raw, allow_unicode=True), encoding="utf-8")
        return _build_dict(path)

    alpha = build("甲类", "甲类标题", ["甲项一", "甲项二"])
    beta = build("乙类", "乙类标题", ["乙项一", "乙项二"])
    monkeypatch.setattr(_postprocess, "load_dicts", lambda: (alpha, beta))
    cells = [{"x": 100, "y": -60, "text": "某某医院甲类标题报告单"}] + _bare_roles()
    cells += row(30, "甲项一", "1", unit="mmol/L", ref="1-2")
    cells += row(55, "甲项二", "1", unit="mmol/L", ref="1-2")
    cells += row(80, "乙项一", "1", unit="mmol/L", ref="1-2")
    cells += row(105, "乙项二", "1", unit="mmol/L", ref="1-2")
    r = run_postprocess(mk_ocr(cells))
    assert r.report_type == "甲类"


def test_registry_pairs_count_decides_without_title(monkeypatch, tmp_path) -> None:
    """无标题命中:双方都达标时取匹配项数多者(临检 10 项 > 尿常规 3 项)。"""
    urine = _urine_dict(tmp_path, min_matched=3)
    real = _load_dicts()
    monkeypatch.setattr(_postprocess, "load_dicts", lambda: (urine, *real))
    cells = _bare_roles() + _cbc_rows()
    for i, name in enumerate(_URINE_ITEM_NAMES[:3]):
        cells += row(300 + 25 * i, name, "5.0", unit="mmol/L", ref="1-10")
    r = run_postprocess(mk_ocr(cells))
    assert r.report_type == "临检"


def test_registry_tie_follows_registry_order(monkeypatch, tmp_path) -> None:
    """配对数并列:取注册表先者(文件名序——测试里为注入顺序在前者)。"""
    urine = _urine_dict(tmp_path, min_matched=10)
    real = _load_dicts()
    monkeypatch.setattr(_postprocess, "load_dicts", lambda: (urine, *real))
    cells = _bare_roles() + _cbc_rows()
    for i, name in enumerate(_URINE_ITEM_NAMES):
        cells += row(300 + 25 * i, name, "5.0", unit="mmol/L", ref="1-10")
    r = run_postprocess(mk_ocr(cells))
    assert r.report_type == "尿常规"


# ---------- 项目组(panel):逐项归属与报告级聚合(spec §5/§6) ----------


def test_mixed_chemistry_panels_sorted_by_count_then_priority() -> None:
    """混合生化报告保留全部项目组,按命中项数降序、同数按固定优先级。"""
    cells = [{"x": 100, "y": -60, "text": "某院肝肾功能报告单"}] + _bare_roles()
    cells += row(30, "丙氨酸转氨酶", "20", unit="U/L", ref="7-40")
    cells += row(55, "天冬氨酸转氨酶", "18", unit="U/L", ref="13-35")
    cells += row(80, "肌酐", "51", unit="μmol/L", ref="20-70")
    cells += row(105, "尿酸", "246", unit="μmol/L", ref="150-420")
    cells += row(130, "总胆固醇", "4.2", unit="mmol/L", ref="3-5.2")
    r = run_postprocess(mk_ocr(cells))
    assert r.report_type == "生化"
    assert [i.panel for i in r.items] == ["liver", "liver", "kidney", "uric_acid", "lipid"]
    assert r.panels == ["liver", "kidney", "lipid", "uric_acid"]


def test_urine_glucose_and_bilirubin_stay_in_urinalysis() -> None:
    """同名的葡萄糖 / 胆红素靠 report_type=临检 归尿常规,不跨到血糖 / 肝功。"""
    cells = [{"x": 100, "y": -60, "text": "某院尿常规报告单"}] + _bare_roles()
    for idx, name in enumerate(["葡萄糖", "胆红素", "潜血", "蛋白质", "白细胞酯酶"]):
        cells += row(30 + idx * 25, name, "阴性", ref="阴性")
    r = run_postprocess(mk_ocr(cells))
    assert r.report_type == "临检"
    assert {i.panel for i in r.items} == {"urinalysis"}
    assert r.panels == ["urinalysis"]


def test_biochemistry_glucose_and_bilirubin_are_liver_and_glucose() -> None:
    """同名指标在生化词典里分别归血糖 / 肝功;此处肝脏命中 2 项,排主标签。"""
    cells = [{"x": 100, "y": -60, "text": "某院葡萄糖测定报告单"}] + _bare_roles()
    cells += row(30, "葡萄糖", "5.2", unit="mmol/L", ref="3.9-6.1")
    cells += row(55, "总胆红素", "12.1", unit="μmol/L", ref="5-21")
    cells += row(80, "直接胆红素", "4.0", unit="μmol/L", ref="0-7")
    r = run_postprocess(mk_ocr(cells))
    assert r.report_type == "生化"
    assert item_of(r, "葡萄糖").panel == "glucose"
    assert item_of(r, "总胆红素").panel == "liver"
    assert r.panels == ["liver", "glucose"]


def test_imaging_success_panels_imaging() -> None:
    cells = [
        {"x": 100, "y": -40, "text": "本院超声检查报告单"},
        {"x": 100, "y": 0, "text": "超声提示：宫内单活胎。"},
    ]
    r = run_postprocess(mk_ocr(cells))
    assert r.items == []
    assert r.panels == ["imaging"]


def test_classify_rejects_parent_child_violation(monkeypatch) -> None:
    """词典配置越界时 /classify 明确拒绝,不返回非法 panel。"""
    bad_item = _postprocess.DictItem(
        name="坏项", aliases=("坏项",), units=frozenset(), panel=Panel.THYROID
    )
    bad = _postprocess.LabDict(
        report_type="临检",
        title_keywords=(),
        min_matched_items=1,
        items=(bad_item,),
        alias_map={"坏项": bad_item},
    )
    monkeypatch.setattr(_postprocess, "load_dicts", lambda: (bad,))
    with pytest.raises(ValueError, match="not allowed"):
        _postprocess.classify_items("临检", ["坏项"])


# ---------- 叙述型报告(影像;票 04:段落主体,不抽取指标项) ----------


def test_narrative_right_value_pairs_risk_ratios() -> None:
    """产筛横排表:项目名右侧同行的风险比值/MOM/年龄值;截断值列不抢先。"""
    cells = [
        {"x": 100, "y": -60, "text": "产前筛查报告"},
        {"x": 20, "y": 30, "text": "唐氏综合征"},
        {"x": 180, "y": 30, "text": "1:1913"},
        {"x": 400, "y": 30, "text": "1:270"},
        {"x": 20, "y": 60, "text": "18-三体综合征"},
        {"x": 180, "y": 60, "text": "1:19879"},
        {"x": 300, "y": 60, "text": "<1:1000"},
        {"x": 400, "y": 60, "text": "1:350"},
        {"x": 20, "y": 90, "text": "开放性神经管缺陷"},
        {"x": 180, "y": 90, "text": "<1:1000"},
        {"x": 400, "y": 90, "text": "2.5MOM"},
        {"x": 20, "y": 120, "text": "母龄风险"},
        {"x": 180, "y": 120, "text": "33.25岁"},
        {"x": 400, "y": 120, "text": "35岁"},
    ]
    r = run_postprocess(mk_ocr(cells))

    assert r.report_type == "其他专项"
    assert item_of(r, "唐氏综合征").value == "1:1913"
    assert item_of(r, "18-三体综合征").value == "1:19879"
    assert item_of(r, "开放性神经管缺陷").value == "2.5"
    assert item_of(r, "母龄风险").value == "33.25"
    for name in ("唐氏综合征", "18-三体综合征", "开放性神经管缺陷", "母龄风险"):
        risk = item_of(r, name)
        assert risk.result_kind == "ratio"
        assert risk.raw_value == risk.value
        # 1:1913 这类比值串不是纯数值,无法给出可验证的 canonical 单位;
        # 2.5 / 33.25 这类数值可在 approved 后标 verified。
        expected = "unverified" if risk.value.startswith("1:") else "verified"
        assert risk.unit_status == expected


def test_imaging_dict_has_no_measurement_items() -> None:
    """影像词典删除全部产科测量项,只保留正文段落锚。"""
    imaging = next(d for d in _postprocess.load_dicts() if d.report_type == "影像")
    assert imaging.items == ()
    assert imaging.min_section_hits == 1


def test_imaging_body_anchor_qualifies_without_items() -> None:
    cells = [
        {"x": 100, "y": -40, "text": "本院超声检查报告单"},
        {"x": 100, "y": 0, "text": "超声提示：宫内单活胎。"},
    ]
    r = run_postprocess(mk_ocr(cells))
    assert r.report_type == "影像"
    assert r.items == []
    assert r.status == "success"


def test_imaging_exam_name_anchor_alone_does_not_classify() -> None:
    """exam_name 锚(检查项目/部位/内容)只用于段落抽取,不参与分类。"""
    cells = [
        {"x": 100, "y": -40, "text": "某某医院检查报告单"},
        {"x": 100, "y": 0, "text": "检查项目：产科检查（中孕）"},
    ]
    r = run_postprocess(mk_ocr(cells))
    assert r.report_type == "unknown"
    assert r.items == []


def test_imaging_title_fallback_without_body_keeps_type_but_failed() -> None:
    """正文锚缺失但标题命中超声/彩超:仍按影像归类,因无正文段落判 failed。"""
    cells = [
        {"x": 100, "y": -40, "text": "本院超声检查报告单"},
        {"x": 100, "y": 0, "text": "检查项目：产科胎儿彩超检查（中孕）"},
    ]
    r = run_postprocess(mk_ocr(cells))
    assert r.report_type == "影像"
    assert r.status == "failed"
    assert r.items == []
    assert r.panels == ["other"]


# ---------- 报告级段落(影像单;锚表在词典 category.section_anchors) ----------


def test_sections_anchor_tail_and_following_lines() -> None:
    """三段切分:锚行尾缀 + 后续行至下一锚;exam_name 只取尾缀/下一行。"""
    cells = [
        {"x": 100, "y": -40, "text": "超声检查报告单"},
        {"x": 100, "y": 0, "text": "检查部位："},
        {"x": 100, "y": 15, "text": "产科超声"},
        {"x": 100, "y": 30, "text": "检查所见：宫内单活胎。"},
        {"x": 100, "y": 45, "text": "胎心率159次/分。"},
        {"x": 100, "y": 60, "text": "检查提示：正常。"},
        {"x": 100, "y": 75, "text": "注：仅供临床参考。"},
    ]
    r = run_postprocess(mk_ocr(cells))
    assert section_of(r, "exam_name").text == "产科超声"
    assert section_of(r, "exam_name").ocr_line_numbers == [2]
    assert section_of(r, "findings").text == "宫内单活胎。\n胎心率159次/分。"
    assert section_of(r, "findings").ocr_line_numbers == [3, 4]
    assert section_of(r, "impression").text == "正常。\n注：仅供临床参考。"
    assert section_of(r, "impression").ocr_line_numbers == [5, 6]


def test_sections_exam_name_minimal_termination_and_success_without_items() -> None:
    """us_03 形态:exam_name 不吞后续行;仅有超声提示也是 success(票 04)。"""
    cells = [
        {"x": 100, "y": -40, "text": "超声检查报告单"},
        {"x": 100, "y": 0, "text": "检查项目：产科胎儿彩超检查（中孕）"},
        {"x": 100, "y": 15, "text": "姓名：张三"},
        {"x": 100, "y": 30, "text": "检查日期：2024-09-15"},
        {"x": 100, "y": 45, "text": "超声提示：宫内单活胎。"},
    ]
    r = run_postprocess(mk_ocr(cells))
    assert r.status == "success" and r.items == []
    assert r.report_date == "2024-09-15"
    assert section_of(r, "exam_name").text == "产科胎儿彩超检查（中孕）"
    assert section_of(r, "exam_name").ocr_line_numbers == [1]
    assert not any(s.kind == "findings" for s in r.sections or [])
    assert section_of(r, "impression").text == "宫内单活胎。"


def test_sections_check_content_anchor_uses_next_line_only() -> None:
    """us_02 形态:检查内容 → exam_name;只取下一行,不吞姓名等患者信息。"""
    cells = [
        {"x": 100, "y": -40, "text": "超声检查报告单"},
        {"x": 100, "y": 0, "text": "检查内容"},
        {"x": 100, "y": 15, "text": "胎儿畸形系统筛查 彩超"},
        {"x": 100, "y": 30, "text": "姓名：张三"},
        {"x": 100, "y": 45, "text": "超声描述：宫内妊娠。"},
        {"x": 100, "y": 60, "text": "超声提示：正常。"},
    ]
    r = run_postprocess(mk_ocr(cells))
    assert section_of(r, "exam_name").text == "胎儿畸形系统筛查 彩超"
    assert section_of(r, "exam_name").ocr_line_numbers == [2]
    assert section_of(r, "findings").text == "宫内妊娠。"
    assert section_of(r, "impression").text == "正常。"


def test_sections_empty_between_anchors_are_omitted() -> None:
    """锚到锚之间无内容:该 section 不产出(不是 text=null 的占位)。"""
    cells = [
        {"x": 100, "y": -40, "text": "超声检查报告单"},
        {"x": 100, "y": 0, "text": "检查所见："},
        {"x": 100, "y": 15, "text": "超声提示：正常。"},
    ]
    r = run_postprocess(mk_ocr(cells))
    assert not any(s.kind == "findings" for s in r.sections or [])
    assert section_of(r, "impression").text == "正常。"


def test_sections_same_kind_multiple_occurrences_preserved() -> None:
    """同类锚多次出现按 OCR 行序保留,不覆盖前一段。"""
    cells = [
        {"x": 100, "y": -40, "text": "超声检查报告单"},
        {"x": 100, "y": 0, "text": "检查所见：第一段。"},
        {"x": 100, "y": 15, "text": "超声所见：第二段。"},
    ]
    r = run_postprocess(mk_ocr(cells))
    findings = [s for s in r.sections or [] if s.kind == "findings"]
    assert [s.text for s in findings] == ["第一段。", "第二段。"]
    assert [s.ocr_line_numbers for s in findings] == [[1], [2]]


def test_sections_absent_for_table_reports() -> None:
    """非影像类别零影响:不输出 sections 字段。"""
    r = run_postprocess(mk_ocr(header_row() + row(30, "血红蛋白", "128", unit="g/L", ref="115-150")))
    assert r.sections is None
    assert "sections" not in json.loads(r.model_dump_json())


def test_sections_status_ignores_report_date() -> None:
    """影像 status 只看正文段落:日期未印不降级 partial(票 04)。"""
    cells = [
        {"x": 100, "y": -40, "text": "超声检查报告单"},
        {"x": 100, "y": 0, "text": "超声提示：正常。"},
    ]
    r = run_postprocess(mk_ocr(cells))
    assert r.report_date is None
    assert r.status == "success"
    assert r.items == []
    assert section_of(r, "impression").text == "正常。"


# ---------- 单位规一化(印刷损耗形 → 词典注册的规范印形,单位表) ----------


def test_unit_tables_split_real_units_from_ocr_forms() -> None:
    """真实单位换算表只收真实单位;OCR 损耗形只在印形修复表;两表无交集。"""
    conversions = yaml.safe_load(
        _postprocess.UNIT_CONVERSIONS_FILE.read_text(encoding="utf-8")
    )
    ocr = yaml.safe_load(_postprocess.OCR_FORMS_FILE.read_text(encoding="utf-8"))
    real = {entry["canonical"].lower() for entry in conversions["units"]}
    for entry in conversions["units"]:
        real |= {eq["unit"].lower() for eq in entry.get("equivalents") or ()}
    forms = {form.lower() for fam in ocr["families"] for form in fam["forms"]}
    assert not real & forms
    assert not any(_postprocess._REAL_UNIT_RE.fullmatch(form) for form in forms)
    assert {"umol/l", "uiu/ml", "fl"} <= real
    assert {"109/l", "7/6.0", "f1", "μno1/l"} <= forms


def test_unit_form_map_rejects_ocr_form_in_conversions() -> None:
    """OCR 损耗形混入换算表(自称真实单位)必须失败。"""
    with pytest.raises(ValueError, match="not a real unit"):
        _postprocess._unit_form_map(
            {
                "units": [
                    {
                        "canonical": "10^9/L",
                        "equivalents": [
                            {
                                "unit": "109/L",
                                "conversion": {"type": "exact", "source": "UCUM"},
                            }
                        ],
                    }
                ]
            },
            {"families": []},
        )


def test_unit_form_map_rejects_real_unit_in_ocr_forms() -> None:
    """未在换算表登记的真实单位混入 OCR 修复表也必须失败。"""
    with pytest.raises(ValueError, match="is a real unit"):
        _postprocess._unit_form_map(
            {"units": [{"canonical": "g/L"}]},
            {"families": [{"canonical": "g/L", "forms": ["mg/dL"]}]},
        )


def test_unit_form_map_rejects_form_in_both_tables() -> None:
    """同一字符串不得同时出现在换算表与修复表。"""
    with pytest.raises(ValueError, match="both tables"):
        _postprocess._unit_form_map(
            {
                "units": [
                    {
                        "canonical": "fL",
                        "equivalents": [
                            {
                                "unit": "fl",
                                "conversion": {"type": "exact", "source": "UCUM"},
                            }
                        ],
                    }
                ]
            },
            {"families": [{"canonical": "fL", "forms": ["fl"]}]},
        )


def test_unit_form_map_rejects_ocr_canonical_not_registered() -> None:
    """OCR 修复目标必须是换算表登记的真实单位。"""
    with pytest.raises(ValueError, match="OCR canonical"):
        _postprocess._unit_form_map(
            {"units": []},
            {"families": [{"canonical": "g/L", "forms": ["g1/L"]}]},
        )


def test_unit_form_map_rejects_linear_without_from_to() -> None:
    """linear 换算方向必须显式写 from/to,且与 accepted/canonical 一致(spec §7.9)。"""
    with pytest.raises(ValueError, match="from/to"):
        _postprocess._unit_form_map(
            {
                "units": [
                    {
                        "canonical": "g/L",
                        "equivalents": [
                            {
                                "unit": "mg/dL",
                                "conversion": {
                                    "type": "linear",
                                    "factor": 0.01,
                                    "source": "X",
                                },
                            }
                        ],
                    }
                ]
            },
            {"families": []},
        )


def test_loss_form_normalized_to_registered_canonical() -> None:
    """cbc_01 真实形态:上标 9 拍平成 109/L → 归一为 canonical 10*9/L。"""
    ocr = mk_ocr(header_row() + row(30, "血小板", "136", unit="109/L", ref="125-350"))
    it = item_of(run_postprocess(ocr), "血小板")
    assert it.unit == "10*9/L"
    assert it.unit_display == "10^9/L"
    assert "109/L" in it.raw_text


def test_wechat_loss_form_normalized() -> None:
    """cbc_05 真实形态:600px 截图把 10^9/L 读成 7/6.0 → 归一为 canonical。"""
    ocr = mk_ocr(header_row() + row(30, "血小板", "184", unit="7/6.0", ref="100~300"))
    assert item_of(run_postprocess(ocr), "血小板").unit == "10*9/L"


def test_loss_form_dispatched_by_item_registration() -> None:
    """同损耗形跨族(f1 ∈ fL 族与 10^9 族)由该项注册集裁决唯一生效者。"""
    ocr = mk_ocr(
        header_row()
        + row(30, "平均红细胞体积", "92.6", unit="f1", ref="82-100")
        + row(50, "嗜碱性粒细胞数", "0.010", unit="f1")
    )
    r = run_postprocess(ocr)
    assert item_of(r, "平均红细胞体积").unit == "fL"
    assert item_of(r, "嗜碱性粒细胞数").unit == "10*9/L"


def test_unknown_unit_kept_raw_as_unverified() -> None:
    """注册集与单位族之外的乱珠不归一:unit=null、raw_unit 留原文、标 unverified。"""
    ocr = mk_ocr(header_row() + row(30, "血红蛋白", "128", unit="xw/L", ref="115-150"))
    it = item_of(run_postprocess(ocr), "血红蛋白")
    assert it.unit is None
    assert it.raw_unit == "xw/L"
    assert it.unit_status == "unverified"
    assert it.low_confidence is True


def test_vertical_fallback_rejects_prose_cells() -> None:
    """竖排回退只认数值格:页脚散文格不得被抽成 value。

    实测(共享栈 尿常规报告):零碎片行落在页脚的「实验室地址：…航天东路155号」上,
    按其数字抽值把地址门牌号 155 写成了 尿潜血 的 value——整格并非数值语义。
    """
    cells = [
        {"x": 100, "y": -60, "text": "市人民医院血常规报告单"},
        {"x": 20, "y": 0, "text": "序号"},
        {"x": 160, "y": 0, "text": "项目名称"},
        {"x": 360, "y": 0, "text": "结果"},
        {"x": 470, "y": 0, "text": "单位"},
        {"x": 580, "y": 0, "text": "参考值"},
        {"x": 150, "y": 40, "w": 90, "text": "白细胞计数"},
        {"x": 140, "y": 600, "w": 180, "text": "实验室地址：某某路155号 电话:029-000"},
    ]
    it = item_of(run_postprocess(mk_ocr(cells)), "白细胞计数")
    assert it.value is None
    assert "实验室地址" not in it.raw_text


def test_vertical_fallback_allows_small_row_overlap() -> None:
    """名称行与正下方数值行框小幅重叠:表内零碎片行仍取同列最近数值。"""
    cells = [
        {"x": 100, "y": -60, "text": "市人民医院血常规报告单"},
        {"x": 20, "y": 0, "text": "序号"},
        {"x": 160, "y": 0, "text": "项目名称"},
        {"x": 360, "y": 0, "text": "结果"},
        {"x": 470, "y": 0, "text": "单位"},
        {"x": 580, "y": 0, "text": "参考值"},
        {"x": 150, "y": 40, "w": 90, "text": "白细胞计数"},
        {"x": 150, "y": 56, "w": 70, "text": "5.3"},
    ]
    it = item_of(run_postprocess(mk_ocr(cells)), "白细胞计数")
    assert it.value == "5.3"


def test_vertical_fallback_accepts_leading_value() -> None:
    """竖排回退的正例:列内短数值格(可带 ↑ 等短缀)照常吸收。"""
    cells = [
        {"x": 100, "y": -60, "text": "市人民医院血常规报告单"},
        {"x": 20, "y": 0, "text": "序号"},
        {"x": 160, "y": 0, "text": "项目名称"},
        {"x": 360, "y": 0, "text": "结果"},
        {"x": 470, "y": 0, "text": "单位"},
        {"x": 580, "y": 0, "text": "参考值"},
        {"x": 150, "y": 40, "w": 90, "text": "白细胞计数"},
        {"x": 150, "y": 160, "w": 70, "text": "5.3 ↑"},
    ]
    it = item_of(run_postprocess(mk_ocr(cells)), "白细胞计数")
    assert it.value == "5.3"


def test_family_map_chosen_by_item_membership() -> None:
    """同族损耗形跨类别生效:μ 家族(lft_03/05 证据)按生化词典项归一。"""
    chem = next(d for d in _postprocess.load_dicts() if d.report_type == "生化")
    bilirubin = next(i for i in chem.items if i.name.startswith("总胆红素"))
    assert _postprocess._normalize_unit("μmo1/L", bilirubin) == "μmol/L"
    assert _postprocess._normalize_unit("μnol/L", bilirubin) == "μmol/L"


def test_real_unit_equivalence_normalized_from_conversion_table() -> None:
    """换算表里的真实单位等价(umol/L → μmol/L)也参与展示归一。"""
    chem = next(d for d in _postprocess.load_dicts() if d.report_type == "生化")
    creatinine = next(i for i in chem.items if i.name.startswith("肌酐"))
    assert _postprocess._normalize_unit("umol/L", creatinine) == "μmol/L"


def test_trailing_unit_punctuation_and_ocr_loss_forms() -> None:
    """单位格尾点与 OCR 空格/大小写损耗不进入 unit 字段。

    真实载体:lft_08 尿素 "mmol/L."、白蛋白 "g/l."、肌酐 "μmol L"、
    二氧化碳 "mmol/l.";cbc_07 血红蛋白 "g/L."。
    """
    lft = _golden_report("lft_08")
    assert item_of(lft, "尿素").unit == "mmol/L"
    assert item_of(lft, "肌酐").unit == "umol/L"
    assert item_of(lft, "白蛋白").unit == "g/L"
    assert item_of(lft, "二氧化碳").unit == "mmol/L"
    cbc = _golden_report("cbc_07")
    assert item_of(cbc, "血红蛋白").unit == "g/L"


def test_vitamin_d_tiered_reference_uses_normal_tier() -> None:
    """参考列的分级解读行(vitanemia_01/02)以「正常」档为比较基准。

    原行为只取首行 "<12ng/ml缺乏",把 5.70 判 normal、39.60 判 high;
    正确口径:5.70 → low,39.60 → normal;tiered 参考原文保留,值换算到 nmol/L。
    """
    may = _golden_report("vitanemia_01")
    sep = _golden_report("vitanemia_02")
    for report, value, flag in ((may, "14.25", "low"), (sep, "99", "normal")):
        it = item_of(report, "总25羟基维生素D")
        assert it.value == value
        assert it.unit == "nmol/L"
        assert it.raw_value in ("5.70", "39.60")
        assert it.raw_unit in ("ng/ml", "ng/m1")
        assert it.ref_range == ">20ng/ml正常"
        assert it.ref_kind == "tiered"
        assert it.flag == flag
        assert all(label in it.raw_text for label in ("缺乏", "不足", "正常"))
    assert item_of(sep, "维生素B12").unit == "pmol/L"


# ---------- 标准输出层(票 03–11) ----------


def report_header(title: str) -> list[dict[str, Any]]:
    return [
        {"x": 100, "y": -60, "text": title},
        {"x": 600, "y": -60, "text": "报告时间：2026-10-01 09:00"},
        {"x": 20, "y": 0, "text": "序号"},
        {"x": 160, "y": 0, "text": "项目名称"},
        {"x": 360, "y": 0, "text": "结果"},
        {"x": 470, "y": 0, "text": "单位"},
        {"x": 580, "y": 0, "text": "参考值"},
    ]


def test_coag_standard_output_golden() -> None:
    """票 03:PT/APTT/TT → s/秒;PTR/INR → ratio 1;D-二聚体 → μg/L + FEU。"""
    r = _golden_report("coag_01")
    pt = item_of(r, "凝血酶原时间检测")
    assert (pt.unit, pt.unit_display, pt.raw_unit) == ("s", "秒", "秒")
    assert pt.code == "0500103A"
    assert pt.result_kind == "quantitative"
    assert pt.unit_status == "verified"
    assert pt.ref_kind == "numeric"

    for name in ("凝血酶原时间比值", "国际标准化比值"):
        ratio = item_of(r, name)
        assert ratio.result_kind == "ratio"
        assert ratio.unit == "1"
        assert ratio.raw_unit is None

    dd = item_of(r, "D-二聚体检测")
    assert dd.value == "440.00"
    assert dd.unit == "ug/L"
    assert dd.unit_display == "μg/L"
    assert dd.raw_unit == "ng/mL"
    assert dd.result_basis == "FEU"
    assert item_of(r, "纤维蛋白原检测").unit == "g/L"
    assert item_of(r, "纤维蛋白（原）降解产物检测").unit == "ug/mL"


def test_d_dimer_mg_l_converted_to_ug_l() -> None:
    ocr = mk_ocr(
        report_header("凝血功能报告单")
        + row(30, "D-二聚体", "1.2", unit="mg/L", ref="0-1")
        + row(50, "凝血酶原时间", "10.8", unit="秒", ref="9.8-12.1")
    )
    dd = item_of(run_postprocess(ocr), "D-二聚体检测")
    assert dd.value == "1200"
    assert dd.unit == "ug/L"
    assert dd.raw_value == "1.2"
    assert dd.raw_unit == "mg/L"
    assert dd.result_basis == "FEU"


def test_cbc_standard_output_conversions() -> None:
    """票 04:HCT %→L/L(value+ref 同步)、计数等值替身、RDW 不混用。"""
    ocr = mk_ocr(
        header_row()
        + row(30, "红细胞压积", "44.1", unit="%", ref="36-50")
        + row(50, "白细胞", "7.33", unit="10^9/L", ref="4-10")
        + row(70, "血小板", "215", unit="109/L", ref="100-300")
        + row(90, "红细胞分布宽度-SD", "39.3", unit="fL", ref="37-54")
    )
    r = run_postprocess(ocr)
    hct = item_of(r, "红细胞压积")
    assert (hct.value, hct.unit, hct.ref_range) == ("0.441", "L/L", "0.36-0.5")
    assert hct.raw_value == "44.1"
    assert hct.raw_ref_range == "36-50"
    assert item_of(r, "白细胞").unit == "10*9/L"
    assert item_of(r, "血小板").unit == "10*9/L"
    assert item_of(r, "红细胞分布宽度-SD").unit == "fL"


def test_liver_bilirubin_protein_and_enzyme_units() -> None:
    """票 05:胆红素/蛋白换算;U/L 与 IU/L 不默认等价。"""
    ocr = mk_ocr(
        report_header("肝功报告单")
        + row(30, "总胆红素", "1.2", unit="mg/dL", ref="0.2-1.2")
        + row(50, "总蛋白", "7.4", unit="g/dL", ref="6.0-8.0")
        + row(70, "丙氨酸转氨酶", "70", unit="IU/L", ref="9-50")
    )
    r = run_postprocess(ocr)
    tbil = item_of(r, "总胆红素")
    assert tbil.value == "20.52"
    assert tbil.unit == "umol/L"
    assert tbil.ref_range == "3.42-20.52"
    tp = item_of(r, "总蛋白")
    assert tp.value == "74"
    assert tp.unit == "g/L"
    alt = item_of(r, "丙氨酸转氨酶")
    assert alt.unit is None
    assert alt.raw_unit == "IU/L"
    assert alt.unit_status == "unverified"
    assert alt.low_confidence is True


def test_kidney_urate_gate_and_egfr_unverified() -> None:
    """票 06:尿酸 mmol/L→μmol/L;eGFR mL/min 缺 BSA 归一不静默接受。"""
    ocr = mk_ocr(
        report_header("肾功能报告单")
        + row(30, "尿酸", "0.246", unit="mmol/L", ref="0.100-0.420")
        + row(50, "eGFR", "107.3", unit="ml/min", ref="≥90")
        + row(70, "二氧化碳", "23.31", unit="mmol/L", ref="22-29")
    )
    r = run_postprocess(ocr)
    ua = item_of(r, "尿酸")
    assert (ua.value, ua.unit, ua.ref_range) == ("246", "umol/L", "100-420")
    egfr = item_of(r, "eGFR")
    assert egfr.unit is None
    assert egfr.raw_unit == "ml/min"
    assert egfr.low_confidence is True
    assert item_of(r, "二氧化碳").name == "二氧化碳结合力"


def test_glucose_hba1c_equation_and_conversion() -> None:
    """票 07:HbA1c IFCC-NGSP 方程;血糖 mg/dL 线性换算。"""
    ocr = mk_ocr(
        report_header("糖化血红蛋白报告单")
        + row(30, "糖化血红蛋白", "6.50", unit="%", ref="4-6")
        + row(50, "空腹葡萄糖", "126", unit="mg/dL", ref="70-100")
    )
    r = run_postprocess(ocr)
    hba1c = item_of(r, "糖化血红蛋白")
    assert hba1c.value == "47.54"
    assert hba1c.unit == "mmol/mol"
    assert hba1c.raw_value == "6.50"
    glu = item_of(r, "空腹葡萄糖")
    assert glu.value == "6.993"
    assert glu.unit == "mmol/L"
    assert glu.ref_range == "3.885-5.55"


def test_endocrine_t3_gate_and_t4_ocr_form() -> None:
    """票 08:T3 mmol/L 换算后 2040 nmol/L 超量级门;T4 rmol/L OCR 形归一。"""
    ocr = mk_ocr(
        report_header("甲状腺报告单")
        + row(30, "三碘甲状腺原氨酸", "2.04", unit="mmol/L", ref="1.01-2.48")
        + row(50, "甲状腺素", "120.23", unit="rmol/L", ref="69.97-152.52")
        + row(70, "抗甲状腺过氧化物酶抗体", "0.80", unit="U/mL", ref="<9")
    )
    r = run_postprocess(ocr)
    t3 = item_of(r, "三碘甲状腺原氨酸")
    assert t3.value == "2.04"
    assert t3.unit is None
    assert t3.raw_unit == "mmol/L"
    assert t3.unit_status == "unverified"
    t4 = item_of(r, "甲状腺素")
    assert t4.value == "120.23"
    assert t4.unit == "nmol/L"
    tpo = item_of(r, "抗甲状腺过氧化物酶抗体")
    assert tpo.unit is None
    assert tpo.raw_unit == "U/mL"


def test_urinalysis_result_kinds_and_count_units() -> None:
    """票 09:干化学无 unit;沉渣计数 /uL;镜检 microscopy 不造伪单位。"""
    r = _golden_report("ua_01")
    dip = item_of(r, "潜血")
    assert dip.result_kind == "qualitative"
    assert dip.unit is None
    assert dip.ref_kind == "qualitative"

    count = item_of(r, "尿红细胞计数(镜检)")
    assert count.result_kind == "quantitative"
    assert count.unit == "/uL"
    assert count.unit_display == "个/μL"
    assert count.raw_unit == "个/uL"

    micro = item_of(r, "镜检红细胞")
    assert micro.result_kind == "microscopy"
    assert micro.unit is None
    assert micro.raw_unit == "/HP"

    ph = item_of(r, "PH")
    assert ph.unit == "1"
    assert ph.result_kind == "quantitative"


def test_immunology_arbitrary_ratio_and_qualitative() -> None:
    """票 10:AU → [arb'U];S/CO → ratio 1;定性无 unit;IU/U 不互换。"""
    ana = _golden_report("ana_02")
    au = item_of(ana, "抗RNP抗体")
    assert (au.unit, au.unit_display) == ("[arb'U]", "AU")
    assert au.result_kind == "arbitrary"
    assert item_of(ana, "抗C1q抗体").unit == "U/mL"
    assert item_of(ana, "抗dsDNA抗体").unit == "[IU]/mL"

    inf = _golden_report("infectious_01")
    hbe = item_of(inf, "乙肝e抗原")
    assert hbe.result_kind == "ratio"
    assert hbe.unit == "1"
    assert hbe.raw_unit in ("s/co", "S/CO")

    qual = _golden_report("ana_01")
    ana_iif = item_of(qual, "ANA(IIF)")
    assert ana_iif.result_kind == "qualitative"
    assert ana_iif.unit is None


def test_special_and_bloodtype_models() -> None:
    """票 11:风险比值按 ratio;血型定性无 unit。"""
    special = next(d for d in _postprocess.load_dicts() if d.report_type == "其他专项")
    risk = next(i for i in special.items if i.name == "母龄风险")
    assert risk.result_kind == "ratio"
    assert risk.canonical_unit == "1"
    hcg_beta = next(i for i in special.items if "β亚单位" in i.name)
    assert hcg_beta.result_kind == "quantitative"
    assert hcg_beta.canonical_unit == "ug/L"

    blood = _golden_report("bloodtype_01")
    for item in blood.items:
        assert item.result_kind == "qualitative"
        assert item.unit is None


def test_verified_only_after_approval() -> None:
    """unit_status=verified 只在 review_status=approved 且单位解析成功时出现。"""
    import dataclasses

    from recognizer.postprocess import _standardize_item

    chem = next(d for d in _postprocess.load_dicts() if d.report_type == "生化")
    ua = next(i for i in chem.items if i.name == "尿酸测定")
    pending = _standardize_item(dataclasses.replace(ua, review_status="pending"),
                                "0.246", "mmol/L", "0.1-0.42", False)
    assert pending.unit_status == "unverified"
    approved = _standardize_item(dataclasses.replace(ua, review_status="approved"),
                                 "0.246", "mmol/L", "0.1-0.42", False)
    assert approved.unit_status == "verified"


def test_golden_standard_fields_are_canonical_and_review_gated() -> None:
    """全量 golden:unit 只含 canonical;verified 仅限 approved;OCR 碎片为 0。"""
    canonical = set()
    review_status: dict[str, str] = {}
    for d in _postprocess.load_dicts():
        for it in d.items:
            review_status[it.name] = it.review_status
            if it.canonical_unit:
                canonical.add(it.canonical_unit)
    canonical |= {"1", "[arb'U]", "[IU]/mL", "[IU]/L", "m[IU]/L", "m[IU]/mL", "U/mL", "/uL"}
    fragments = {"109/L", "7/6.0", "1/6.0", ".09/L", "~9/L", "f1", "10/a1",
                 "IU/al", "ng/al", "ag/L", "rmol/L", "μno1/L", "pmo1/L"}
    for sample_id in _sample_ids:
        report = _golden_report(sample_id)
        for item in report.items:
            if item.unit is not None:
                assert item.unit in canonical, f"{sample_id}/{item.name}: {item.unit!r}"
            assert item.unit not in fragments, f"{sample_id}/{item.name}: OCR fragment"
            if item.unit_status == "verified":
                assert review_status[item.name] == "approved", (
                    f"{sample_id}/{item.name}: verified but review_status={review_status[item.name]}"
                )
            elif review_status[item.name] == "pending":
                assert item.unit_status != "verified", f"{sample_id}/{item.name}: pending verified"
            if item.value is not None:
                assert item.raw_value is not None, f"{sample_id}/{item.name}: raw_value 丢失"


# ---------- golden(真实样本,OCR 结果来自 golden 文件,跨架构确定性) ----------

EXPECTED_OCR = paths.EXPECTED_OCR
EXPECTED_REPORT = paths.EXPECTED_REPORT

_sample_ids = sorted(p.stem for p in EXPECTED_OCR.glob("*.json")) if EXPECTED_OCR.exists() else []


@pytest.mark.skipif(not EXPECTED_REPORT.exists(), reason="no golden files under samples/expected/report/")
@pytest.mark.parametrize("sample_id", _sample_ids)
def test_report_matches_golden(sample_id: str) -> None:
    ocr_data = json.loads((EXPECTED_OCR / f"{sample_id}.json").read_text(encoding="utf-8"))
    report = run_postprocess(
        OCRResult.model_validate(ocr_data),
        date_hint=date_hint_for(sample_id),
        image=image_bytes_for(sample_id),
    )
    golden = json.loads((EXPECTED_REPORT / f"{sample_id}.json").read_text(encoding="utf-8"))
    assert json.loads(report.model_dump_json()) == golden


@pytest.mark.parametrize(
    ("sample_id", "want"),
    [
        ("lft_02", ["liver", "kidney", "uric_acid"]),
        ("lft_06", ["liver", "lipid"]),
        ("lft_08", ["liver", "kidney", "uric_acid", "electrolyte"]),
        ("lft_09", ["liver", "kidney"]),
        ("glu_01", ["glucose"]),
        ("tft_01", ["thyroid"]),
        ("proghcg_01", ["sex_hormone"]),
        ("vitanemia_01", ["vitanemia"]),
        ("cbc_04", ["cbc"]),
        ("ua_05", ["urinalysis"]),
        ("coag_01", ["coag"]),
        ("bloodtype_01", ["other"]),
        ("fibrosis_01", ["other"]),
        ("unknown_01", ["other"]),
        ("us_01", ["imaging"]),
        ("infectious_01", ["infection"]),
        ("ana_01", ["autoimmune"]),
    ],
)
def test_golden_panel_expectations(sample_id: str, want: list[str]) -> None:
    """spec §6.5 的样本主标签/项目组集合回归。"""
    ocr_data = json.loads((EXPECTED_OCR / f"{sample_id}.json").read_text(encoding="utf-8"))
    report = run_postprocess(
        OCRResult.model_validate(ocr_data),
        date_hint=date_hint_for(sample_id),
        image=image_bytes_for(sample_id),
    )
    assert report.panels == want
