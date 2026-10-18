"""flag 语义共享向量(schemas/testdata/flag.vectors.json):识别端 _flag 与数据服务
ComputeFlag(internal/pipeline/flag.go)读同一份——改语义先改向量表,双侧测试强制通过。
与 Go 侧 internal/pipeline/flag_test.go 的 TestComputeFlag 成对。"""

import json
from pathlib import Path

import pytest

from recognizer.postprocess import _flag

_VECTORS = json.loads(
    (
        Path(__file__).resolve().parents[2] / "schemas" / "testdata" / "flag.vectors.json"
    ).read_text(encoding="utf-8")
)


@pytest.mark.parametrize("case", _VECTORS["cases"], ids=lambda c: c["note"])
def test_flag_vectors(case: dict) -> None:
    assert _flag(case["value"], case["ref"]) == case["flag"]
