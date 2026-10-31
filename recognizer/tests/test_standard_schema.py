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
    assert any(u["unit"] == "秒" for u in pt.accepted_units)


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
    assert {u["unit"] for u in dd.accepted_units} == {"ng/mL", "mg/L"}
