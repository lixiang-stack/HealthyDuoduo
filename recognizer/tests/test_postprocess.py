"""postprocess 单元测试:以数据文件 cbc_dict.yaml 决定词典行为。

锚定规则(实施计划 4.2/P2):
- 行按 OCR box 的 y 聚带、按 x 定列(表头角色:名称/结果/单位/参考值);
- 名称 = 词典名或别名,支持「名称熔断数值」(如 平均血红蛋白浓度324);
- 参考范围一律从检查单抽取,支持 115-150 / 4--10 / <3.5 / 男:0-15;
- 单位 ∈ 词典已知单位集,否则该项 low_confidence(值仍保留原文供人工核对);
- 任一组成行 score < 0.8 → 该项 low_confidence;占比过半(≥50%)→ 整单 partial;
- 日期:标签行(检验时间 > … > 报告时间)抽取,解析失败时用 date_hint 兜底;
- 类别失败或无指标 → failed;
- 类别命中且有产出:数值缺失/日期缺失 → partial,否则 success(低置信留在条目标记)。
"""

import json
from typing import Any

import pytest
import yaml

from recognizer import paths
from recognizer import postprocess as _postprocess
from recognizer.contract import OCRResult
from recognizer.golden import date_hint_for
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
    assert it.value == "128"
    assert it.unit == "g/L"
    assert it.ref_range == "115-150"
    assert it.flag == "normal"
    assert it.low_confidence is False
    assert "血红蛋白" in it.raw_text


def test_alias_and_serial_strip() -> None:
    ocr = mk_ocr(header_row() + row(30, "3 白细胞", "7.33", unit="*10^9/L", ref="4-10"))
    r = run_postprocess(ocr)
    it = item_of(r, "白细胞计数")
    assert it.value == "7.33"
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
    assert it.unit == "10^9/L"
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
    assert it.unit == "10^9/L"  # 展示归一,原文留痕在 raw_text
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
    assert r.report_type == "临检"


# ---------- 多词典注册表分发(P3:词典数据文件每个类别一份,分发语义见 _select_category) ----------


_CBC_ROW_NAMES = ["白细胞计数", "红细胞计数", "血红蛋白", "红细胞压积", "血小板",
                  "淋巴细胞比率", "中性细胞比率", "单核细胞比率", "血小板压积", "淋巴细胞数"]

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


# ---------- 叙述体抽取(P3,category.mode=narrative,现 影像词典) ----------


def test_narrative_alias_bridge_value() -> None:
    cells = [{"x": 100, "y": -40, "text": "本院超声检查报告单"},
             {"x": 100, "y": 0, "text": "颈项透明层厚度（NT）2.6mm，头臀长59mm。"}]
    r = run_postprocess(mk_ocr(cells))
    assert r.report_type == "影像"
    assert item_of(r, "颈项透明层厚度").value == "2.6"
    assert item_of(r, "颈项透明层厚度").unit == "mm"
    assert item_of(r, "头臀长").value == "59"


def test_narrative_remote_number_not_fused() -> None:
    """远距数字不误认:「NT筛查、11-14周」的 11 不应成为 NT 数值。"""
    cells = [{"x": 100, "y": -40, "text": "本院超声检查报告单"},
             {"x": 100, "y": 0, "text": "11-14周NT筛查(省免)、11-14周"}]
    r = run_postprocess(mk_ocr(cells))
    it = item_of(r, "颈项透明层厚度")
    assert it.value is None  # 桥接失败 → 无数值,标题行仅留痕
    assert "NT筛查" in it.raw_text


def test_narrative_first_value_wins_over_title_mention() -> None:
    """同 canonical 多次命中:标题先行无数值 → 后段有数值者胜。"""
    cells = [{"x": 100, "y": -40, "text": "本院超声检查报告单"},
             {"x": 100, "y": 0, "text": "NT筛查(省免)"},
             {"x": 100, "y": 30, "text": "（NT）2.6mm"}]
    r = run_postprocess(mk_ocr(cells))
    it = item_of(r, "颈项透明层厚度")
    assert it.value == "2.6"
    assert "2.6mm" in it.raw_text


# ---------- 单位规一化(印刷损耗形 → 词典注册的规范印形,unit_families.yaml) ----------


def test_loss_form_normalized_to_registered_canonical() -> None:
    """cbc_01 真实形态:上标 9 拍平成 109/L → 归一为注册印形 10^9/L。"""
    ocr = mk_ocr(header_row() + row(30, "血小板", "136", unit="109/L", ref="125-350"))
    it = item_of(run_postprocess(ocr), "血小板")
    assert it.unit == "10^9/L"
    assert "109/L" in it.raw_text


def test_wechat_loss_form_normalized() -> None:
    """cbc_05 真实形态:600px 截图把 10^9/L 读成 7/6.0 → 归一为 10^9/L。"""
    ocr = mk_ocr(header_row() + row(30, "血小板", "184", unit="7/6.0", ref="100~300"))
    assert item_of(run_postprocess(ocr), "血小板").unit == "10^9/L"


def test_loss_form_dispatched_by_item_registration() -> None:
    """同损耗形跨族(f1 ∈ fL 族与 10^9 族)由该项注册集裁决唯一生效者。"""
    ocr = mk_ocr(
        header_row()
        + row(30, "平均红细胞体积", "92.6", unit="f1", ref="82-100")
        + row(50, "嗜碱性粒细胞数", "0.010", unit="f1")
    )
    r = run_postprocess(ocr)
    assert item_of(r, "平均红细胞体积").unit == "fL"
    assert item_of(r, "嗜碱性粒细胞数").unit == "10^9/L"


def test_unknown_unit_kept_as_printed() -> None:
    """注册集与单位族之外的乱珠不归一,保留原文供人工核对。"""
    ocr = mk_ocr(header_row() + row(30, "血红蛋白", "128", unit="xw/L", ref="115-150"))
    assert item_of(run_postprocess(ocr), "血红蛋白").unit == "xw/L"


def test_family_map_chosen_by_item_membership() -> None:
    """同族损耗形跨类别生效:μ 家族(lft_03/05 证据)按生化词典项归一。"""
    chem = next(d for d in _postprocess.load_dicts() if d.report_type == "生化")
    bilirubin = next(i for i in chem.items if i.name == "总胆红素")
    assert _postprocess._normalize_unit("μmo1/L", bilirubin) == "μmol/L"
    assert _postprocess._normalize_unit("μnol/L", bilirubin) == "μmol/L"


# ---------- golden(真实样本,OCR 结果来自 golden 文件,跨架构确定性) ----------

EXPECTED_OCR = paths.EXPECTED_OCR
EXPECTED_REPORT = paths.EXPECTED_REPORT

_sample_ids = sorted(p.stem for p in EXPECTED_OCR.glob("*.json")) if EXPECTED_OCR.exists() else []


@pytest.mark.skipif(not EXPECTED_REPORT.exists(), reason="no golden files under samples/expected/report/")
@pytest.mark.parametrize("sample_id", _sample_ids)
def test_report_matches_golden(sample_id: str) -> None:
    ocr_data = json.loads((EXPECTED_OCR / f"{sample_id}.json").read_text(encoding="utf-8"))
    report = run_postprocess(OCRResult.model_validate(ocr_data), date_hint=date_hint_for(sample_id))
    golden = json.loads((EXPECTED_REPORT / f"{sample_id}.json").read_text(encoding="utf-8"))
    assert json.loads(report.model_dump_json()) == golden
