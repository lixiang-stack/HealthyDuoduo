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
