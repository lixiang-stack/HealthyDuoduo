"""txts 跨架构容差比较的单测:阈值 2 容纳 arm64↔x86_64 实测漂移(cbc 单字符、us_03 双字符),且不掩盖真回归。"""

import pytest

from tests.test_ocr_golden import texts_compatible


@pytest.mark.parametrize(
    ("actual", "golden", "expected"),
    [
        (["完全一致"], ["完全一致"], True),
        (["1/6.0 30--31"], ["1/6.0 130--31"], True),  # 缺失 1 字符(CI 实测)
        (["0.4--8%"], ["0.4--8 %"], True),  # 空格噪声,忽略空白即可
        (["13.5--9.5 109/L"], ["13.5--9.5 10^9/L"], True),  # ^ ↔ 空格替换(CI 实测)
        (["后6率：S/03.53,"], ["6率：S/03.53，"], True),  # us_03 双字符漂移(增字 + 全/半角逗号)
        (["a", "b"], ["a", "b ", "c"], False),  # 行数不等:结构变化
        (["130--31"], ["999--31"], False),  # 单行 3 处差异:超出阈值,真回归
        (["totally different line"], ["完全不同的一行"], False),
    ],
)
def test_texts_compatible(actual: list[str], golden: list[str], expected: bool) -> None:
    assert texts_compatible(actual, golden) is expected
