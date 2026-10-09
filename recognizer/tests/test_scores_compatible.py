"""scores 跨架构统计级比较的单测:单行高压噪声(MAE ≈0.001)不越界,整体退化立即检出。"""

import pytest

from tests.test_ocr_golden import scores_compatible


def _flat(n: int, value: float) -> list[float]:
    return [value] * n


@pytest.mark.parametrize(
    ("actual", "golden", "expected"),
    [
        (_flat(70, 0.99), _flat(70, 0.99), True),  # 完全一致
        ([0.93941] + _flat(69, 0.99), [0.99785] + _flat(69, 0.99), True),  # CI 实测单行漂移 0.058,MAE≈0.001
        (_flat(70, 0.97), _flat(70, 0.99), True),  # 均匀 0.02 漂移:系统性噪声,MAE 未越界
        (_flat(70, 0.94), _flat(70, 0.99), False),  # 均匀 0.05 漂移:MAE 越界
        (_flat(70, 0.8), _flat(70, 0.99), False),  # 置信度整体塌陷:立即检出
        (_flat(70, 0.99), _flat(69, 0.99), False),  # 长度不等:结构变化
        ([], [], False),  # 空行集没有统计意义
    ],
)
def test_scores_compatible(actual: list[float], golden: list[float], expected: bool) -> None:
    assert scores_compatible(actual, golden) is expected
