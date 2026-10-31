"""标准 schema 试点:凝血词典的字段加载与语义(见 .scratch/medical-standardization/spec.md)。"""

from recognizer.postprocess import load_dicts


def _item(name: str):
    for d in load_dicts():
        for it in d.items:
            if it.name == name:
                return it
    raise AssertionError(f"missing item {name}")


def test_coag_standard_fields_loaded() -> None:
    pt = _item("凝血酶原时间检测")
    assert pt.code == "0500103A"
    assert pt.loinc == "5902-2"
    assert pt.result_kind == "quantitative"
    assert pt.canonical_unit == "s"
    assert pt.unit_display == "秒"
    assert pt.source_status == "primary"
    assert pt.review_status == "pending"
    assert len(pt.accepted_units) == 1
    unit = pt.accepted_units[0]
    assert unit.unit == "秒"
    assert unit.conversion is not None
    assert unit.conversion.factor == 1
    assert unit.conversion.from_unit == "秒"
    assert unit.conversion.to_unit == "s"


def test_ratio_item_has_unit_one() -> None:
    inr = _item("国际标准化比值")
    assert inr.result_kind == "ratio"
    assert inr.canonical_unit == "1"
    assert inr.unit_display == "1"
    assert inr.source_status == "secondary"


def test_d_dimer_result_basis_and_units() -> None:
    dd = _item("D-二聚体检测")
    assert dd.result_basis == "FEU"
    assert dd.canonical_unit == "ug/L"
    assert dd.unit_display == "μg/L"
    assert {u.unit for u in dd.accepted_units} == {"ng/mL", "mg/L"}


def test_synonym_source_is_preserved() -> None:
    pt = _item("凝血酶原时间检测")
    assert {s["term"]: s["source"] for s in pt.synonyms}["PT"] == "LOINC 5902-2"


def test_canonical_unit_is_ascii_ucum() -> None:
    """canonical_unit 是 UCUM 码:micro = u,禁止希腊 mu / micro sign。"""
    for d in load_dicts():
        for it in d.items:
            if it.canonical_unit is None:
                continue
            assert "μ" not in it.canonical_unit, f"{it.name}: canonical_unit 含希腊 mu"
            assert "µ" not in it.canonical_unit, f"{it.name}: canonical_unit 含 micro sign"


def test_source_status_is_known() -> None:
    allowed = {"primary", "secondary", "local", "pending"}
    for d in load_dicts():
        for it in d.items:
            assert it.source_status in allowed, f"{it.name}: {it.source_status}"


def test_linear_conversion_direction_is_consistent() -> None:
    for d in load_dicts():
        for it in d.items:
            for au in it.accepted_units:
                conv = au.conversion
                if conv is None or conv.type != "linear":
                    continue
                assert conv.factor is not None, f"{it.name}: linear 缺 factor"
                assert conv.source, f"{it.name}: linear 缺 source"
                assert conv.from_unit == au.unit, f"{it.name}: from != accepted.unit"
                assert conv.to_unit == it.canonical_unit, f"{it.name}: to != canonical_unit"


def test_every_item_has_standard_identity() -> None:
    """181 项迁移后都有结果类型、身份码与来源/复核状态(spec §7.1)。"""
    allowed_kinds = {"quantitative", "ratio", "arbitrary", "titer", "qualitative", "microscopy"}
    for d in load_dicts():
        for it in d.items:
            assert it.standard_schema, f"{it.name}: 未迁移标准 schema"
            assert it.result_kind in allowed_kinds, f"{it.name}: {it.result_kind}"
            assert it.code or it.loinc or it.local_code, f"{it.name}: 无标准码/local_code"
            assert it.source_status in {"primary", "secondary", "local", "pending"}
            assert it.review_status in {"approved", "pending"}


def test_quantitative_items_have_canonical_unit() -> None:
    for d in load_dicts():
        for it in d.items:
            if it.result_kind == "quantitative":
                assert it.canonical_unit, f"{it.name}: quantitative 缺 canonical_unit"
                assert it.unit_display, f"{it.name}: quantitative 缺 unit_display"


def test_non_quantitative_unit_model() -> None:
    for d in load_dicts():
        for it in d.items:
            if it.result_kind == "ratio":
                assert it.canonical_unit == "1", f"{it.name}: ratio 应为 1"
            elif it.result_kind == "arbitrary":
                assert it.canonical_unit == "[arb'U]", f"{it.name}: arbitrary 应为 [arb'U]"
            elif it.result_kind in {"qualitative", "microscopy", "titer"}:
                assert it.canonical_unit is None, f"{it.name}: {it.result_kind} 不应有 unit"


def test_accepted_units_are_complete_conversions() -> None:
    """accepted_units 必须是类型化换算:none 不参与,equation 带 id/formula(spec §7.4)。"""
    for d in load_dicts():
        for it in d.items:
            for au in it.accepted_units:
                conv = au.conversion
                assert conv is not None, f"{it.name}: {au.unit} 缺 conversion"
                assert conv.source, f"{it.name}: {au.unit} 缺 source"
                if conv.type != "none":
                    assert conv.from_unit == au.unit, f"{it.name}: from != accepted.unit"
                    assert conv.to_unit == it.canonical_unit, f"{it.name}: to != canonical_unit"
                if conv.type == "linear":
                    assert conv.factor is not None
                elif conv.type == "equation":
                    assert conv.id and conv.formula
                elif conv.type == "exact":
                    assert conv.factor == 1
                elif conv.type == "none":
                    assert conv.factor is None
                else:
                    raise AssertionError(f"{it.name}: unknown conversion {conv.type}")


def test_standard_units_do_not_contain_ocr_fragments() -> None:
    fragments = {"109/L", "7/6.0", "1/6.0", ".09/L", "~9/L", "f1", "10/a1",
                 "IU/al", "ng/al", "ag/L", "rmol/L", "μno1/L", "pmo1/L", "t"}
    for d in load_dicts():
        for it in d.items:
            units = {it.canonical_unit, it.unit_display, *(au.unit for au in it.accepted_units),
                     *(au.ucum for au in it.accepted_units)}
            assert not (units & fragments), f"{it.name}: standard units contain OCR fragments"
            assert not any("μ" in (u or "") or "µ" in (u or "")
                           for u in (it.canonical_unit, *(au.ucum for au in it.accepted_units))), it.name


def test_plausible_range_for_converted_items() -> None:
    """有 linear/equation 换算的项必须有量级门(spec §4.3/§7.8)。"""
    for d in load_dicts():
        for it in d.items:
            if any(au.conversion and au.conversion.type in {"linear", "equation"}
                   for au in it.accepted_units):
                assert it.plausible_range is not None, f"{it.name}: 缺 plausible_range"


def test_item_names_are_unique_within_dict() -> None:
    for d in load_dicts():
        names = [it.name for it in d.items]
        assert len(names) == len(set(names)), f"{d.report_type}: duplicate item names"
