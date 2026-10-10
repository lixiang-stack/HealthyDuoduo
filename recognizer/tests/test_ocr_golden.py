"""P1 golden 回归(实施计划 8-5):run_ocr 输出对比 samples/expected/ocr/。

对比规则:txts 行数相等且逐行忽略空白后编辑距离 ≤2
(跨 CPU 架构浮点求和顺序差异会翻转 rec 临界字符,如 `130--31`↔`30--31`;
us_03 实测单行漂移达 2:如 `后6率`↔`6率` + 全/半角逗号、`账骨长`↔`联骨长` + 句点,
阈值 2 为实测容差上限;模型/依赖真退化时行内差异远超 2,不会被掩盖。
同架构三通道 CLI/HTTP 输出仍逐字一致,由 test_cli/test_api 覆盖);
scores 按**全行平均绝对误差 ≤0.03** 对比(per-line 跨架构漂移实测 0.023~0.058
且不收敛,行级置信度是噪声主导维度;MAE 滤噪,统计级仍可检出真退化);
boxes 容差 ±2px、elapse 只通过 schema 校验不比对(NF-02/NF-05)。
"""

import json
from pathlib import Path

import pytest
from jsonschema import Draft202012Validator

from recognizer import paths
from recognizer.contract import OCRResult
from recognizer.ocr import run_ocr

SAMPLES = paths.SAMPLES
EXPECTED_OCR = paths.EXPECTED_OCR
SCHEMA = paths.SCHEMA_OCR

SCORE_MAE_TOLERANCE = 0.03
BOX_TOLERANCE_PX = 2.0
TXT_EDIT_DISTANCE_TOLERANCE = 2

_sample_ids = sorted(p.stem for p in EXPECTED_OCR.glob("*.json"))


def _load(path: Path) -> dict:
    return json.loads(path.read_text(encoding="utf-8"))


def _normalized(line: str) -> str:
    return "".join(line.split())


def _edit_distance(a: str, b: str) -> int:
    """Levenshtein 编辑距离(无第三方依赖)。"""
    prev = list(range(len(b) + 1))
    for i, ca in enumerate(a, 1):
        curr = [i]
        for j, cb in enumerate(b, 1):
            curr.append(min(prev[j] + 1, curr[j - 1] + 1, prev[j - 1] + (ca != cb)))
        prev = curr
    return prev[-1]


def scores_compatible(actual: list[float], golden: list[float]) -> bool:
    """全行平均绝对误差 ≤ 阈值(统计级滤噪;长度不等按不兼容处理)。"""
    if len(actual) != len(golden) or not golden:
        return False
    mae = sum(abs(a - b) for a, b in zip(actual, golden)) / len(golden)
    return mae <= SCORE_MAE_TOLERANCE


def texts_compatible(actual: list[str], golden: list[str]) -> bool:
    """行数相等,且逐行忽略空白后编辑距离 ≤ 阈值(跨架构容差,详见模块 docstring)。"""
    if len(actual) != len(golden):
        return False
    return all(
        _edit_distance(_normalized(got), _normalized(want)) <= TXT_EDIT_DISTANCE_TOLERANCE
        for got, want in zip(actual, golden)
    )


def txt_drift_report(actual: list[str], golden: list[str]) -> str:
    """诊断:列出超容差的行(或行数不一致),供 CI 跨架构漂移定位。"""
    if len(actual) != len(golden):
        return f"line count {len(actual)} != {len(golden)}"
    bad = [
        (i, _edit_distance(_normalized(a), _normalized(b)), a, b)
        for i, (a, b) in enumerate(zip(actual, golden))
        if _edit_distance(_normalized(a), _normalized(b)) > TXT_EDIT_DISTANCE_TOLERANCE
    ]
    return "\n".join(f"line {i} d={d}: actual={a!r} golden={b!r}" for i, d, a, b in bad) or "none"


@pytest.mark.skipif(not _sample_ids, reason="no golden files under samples/expected/ocr/")
@pytest.mark.parametrize("sample_id", _sample_ids)
def test_ocr_matches_golden(sample_id: str) -> None:
    image_path = next(iter(SAMPLES.glob(f"{sample_id}.*")))
    ocr = run_ocr(image_path)
    _assert_matches_golden(ocr, _load(EXPECTED_OCR / f"{sample_id}.json"))


def _assert_matches_golden(actual: OCRResult, golden: dict) -> None:
    actual_json = json.loads(actual.model_dump_json())
    Draft202012Validator(_load(SCHEMA)).validate(actual_json)

    assert texts_compatible(actual.txts, golden["txts"]), (
        f"txts drift exceeds per-line edit distance {TXT_EDIT_DISTANCE_TOLERANCE}:\n"
        f"{txt_drift_report(actual.txts, golden['txts'])}"
    )

    assert scores_compatible(actual.scores, golden["scores"]), (
        f"scores MAE exceeds {SCORE_MAE_TOLERANCE} (whole-corpus degradation signal)"
    )

    assert len(actual.boxes) == len(golden["boxes"])
    for got_box, want_box in zip(actual.boxes, golden["boxes"]):
        for (gx, gy), (wx, wy) in zip(got_box, want_box):
            assert abs(gx - wx) <= BOX_TOLERANCE_PX
            assert abs(gy - wy) <= BOX_TOLERANCE_PX

    assert actual.engine == golden["engine"]
    assert actual.model_info.model_dump() == golden["model_info"]
