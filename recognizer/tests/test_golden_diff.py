"""golden_diff 纯函数测试:报告级 + 指标项级变更摘要。"""

import importlib.util
import sys
from pathlib import Path


def _module():
    spec = importlib.util.spec_from_file_location(
        "golden_diff", Path(__file__).resolve().parents[1] / "tools" / "golden_diff.py"
    )
    mod = importlib.util.module_from_spec(spec)
    sys.modules["golden_diff"] = mod
    spec.loader.exec_module(mod)
    return mod


def _report(*items: dict, status: str = "success") -> dict:
    return {"report_type": "临检", "status": status, "panels": ["cbc"], "items": list(items)}


def _item(name: str = "白细胞计数", **fields: object) -> dict:
    base = {"name": name, "flag": "normal", "value": "7.33", "unit": "10*9/L"}
    base.update(fields)
    return base


def test_no_changes_returns_empty() -> None:
    report = _report(_item())
    assert _module().diff_reports(report, report) == []


def test_report_and_item_field_changes() -> None:
    diff = _module().diff_reports(
        _report(_item(unit="10^9/L", unit_status=None)),
        _report(_item(unit="10*9/L", unit_status="unverified"), status="partial"),
    )
    text = "\n".join(diff)
    assert "status: \"success\" -> \"partial\"" in text
    assert "白细胞计数: unit \"10^9/L\" -> \"10*9/L\"" in text
    assert "白细胞计数: unit_status null -> \"unverified\"" in text


def test_item_count_and_name_changes() -> None:
    diff = _module().diff_reports(
        _report(_item("血红蛋白")),
        _report(_item("血红蛋白测定"), _item("血小板")),
    )
    text = "\n".join(diff)
    assert "item count: 1 -> 2" in text
    assert 'name: "血红蛋白" -> "血红蛋白测定"' in text
    assert "  + 血小板" in text


def test_new_and_deleted_golden() -> None:
    assert _module().diff_reports(None, _report()) == ["  new golden"]
    assert _module().diff_reports(_report(), None) == ["  deleted golden"]
