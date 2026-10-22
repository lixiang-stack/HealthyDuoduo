"""flag 语义共享向量(schemas/testdata/flag.vectors.json):识别端 _compare 与数据服务
Compare(internal/pipeline/flag.go)读同一份——改语义先改向量表,双侧测试强制通过。
与 Go 侧 internal/pipeline/flag_test.go 的 TestCompare 成对。"""

import json
from pathlib import Path

import pytest

from recognizer.postprocess import _compare

_VECTORS = json.loads(
    (
        Path(__file__).resolve().parents[2] / "schemas" / "testdata" / "flag.vectors.json"
    ).read_text(encoding="utf-8")
)


@pytest.mark.parametrize("case", _VECTORS["cases"], ids=lambda c: c["note"])
def test_flag_vectors(case: dict) -> None:
    flag, reason = _compare(case.get("name"), case["value"], case["ref"])
    assert flag == case["flag"]
    assert (reason.value if reason is not None else None) == case.get("reason")
