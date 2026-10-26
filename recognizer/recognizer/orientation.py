"""纸面方向纠正:整页横拍或倒置时把图像转正后再 OCR。

RapidOCR 方向分类能纠正逐行 0°/180°,但不会旋转整页图像;手机横拍产生的
90°/270° 页面会以「竖排文本框」被识别,倒置页面则文字可读但整页上下颠倒。
这里按文本行方向与标题/页脚落点判断,统一把整页转正后再 OCR。
"""

from __future__ import annotations

import io
from dataclasses import dataclass

from PIL import Image

from .contract import OCRResult

# 整页判定:纵向行占比足够高且行数足够多;零散竖排标签不触发。
_VERTICAL_RATIO = 0.8
_MIN_LINES = 3
_VERTICAL_ASPECT = 1.5

# 方向选择:报告标题/表头通常在上方,签字/日期/联系方式在下方。
_HEADER_WORDS = ("报告", "检验", "医院", "项目名称", "样本", "姓名", "病人", "设备名称")
_FOOTER_WORDS = (
    "审核者",
    "检验者",
    "报告日期",
    "报告时间",
    "检验时间",
    "采集时间",
    "接收时间",
    "联系电话",
    "实验室地址",
    "声明",
    "备注",
)

# 方向候选(逆时针角度):90 = 向左转正,180 = 上下颠倒,270 = 向右转正。
_CCW = 90
_UPSIDE_DOWN = 180
_CW = 270


@dataclass(frozen=True)
class _Line:
    text: str
    x: float
    y: float


def detect_rotation(ocr: OCRResult) -> int:
    """返回整页需要逆时针旋转的角度:0 / 90 / 180 / 270。"""
    lines = [_line(text, box) for text, box in zip(ocr.txts, ocr.boxes, strict=True)]
    if len(lines) < _MIN_LINES:
        return 0
    vertical = sum(1 for line, box in zip(lines, ocr.boxes, strict=True) if _is_vertical(box))
    if vertical / len(lines) >= _VERTICAL_RATIO:
        ccw_score = _orientation_score(lines, _CCW)
        cw_score = _orientation_score(lines, _CW)
        return _CW if cw_score > ccw_score else _CCW

    upright_score = _orientation_score(lines, 0)
    upside_score = _orientation_score(lines, _UPSIDE_DOWN)
    return _UPSIDE_DOWN if upside_score > upright_score else 0


def rotate_image(data: bytes, rotation: int) -> bytes:
    """按逆时针角度转正图像;保留原格式(JPEG 质量 95)。"""
    with Image.open(io.BytesIO(data)) as opened:
        fmt = opened.format or "JPEG"
        rotated = opened.rotate(rotation, expand=True)
        out = io.BytesIO()
        save_args = {"quality": 95} if fmt == "JPEG" else {}
        rotated.save(out, format=fmt, **save_args)
        return out.getvalue()


def _line(text: str, box: list[tuple[float, float]] | tuple[tuple[float, float], ...]) -> _Line:
    xs = [point[0] for point in box]
    ys = [point[1] for point in box]
    return _Line(text=text, x=(min(xs) + max(xs)) / 2, y=(min(ys) + max(ys)) / 2)


def _is_vertical(box: list[tuple[float, float]] | tuple[tuple[float, float], ...]) -> bool:
    xs = [point[0] for point in box]
    ys = [point[1] for point in box]
    return (max(ys) - min(ys)) > (max(xs) - min(xs)) * _VERTICAL_ASPECT


def _orientation_score(lines: list[_Line], rotation: int) -> int:
    """把行中心按候选方向转正后,奖励「表头在上、页脚在下」。"""
    rotated = sorted((_rotate_point(line, rotation) for line in lines), key=lambda line: (line.y, line.x))
    edge = max(1, len(rotated) // 5)
    top = rotated[:edge]
    bottom = rotated[-edge:]
    return _hits(top, _HEADER_WORDS) + _hits(bottom, _FOOTER_WORDS) - _hits(top, _FOOTER_WORDS) - _hits(
        bottom, _HEADER_WORDS
    )


def _rotate_point(line: _Line, rotation: int) -> _Line:
    # 只用于排序/落点比较,不依赖原图宽高:逆时针 (x,y)→(y,-x),顺时针 (x,y)→(-y,x),
    # 180° (x,y)→(-x,-y)。
    if rotation == _CCW:
        return _Line(text=line.text, x=line.y, y=-line.x)
    if rotation == _UPSIDE_DOWN:
        return _Line(text=line.text, x=-line.x, y=-line.y)
    if rotation == _CW:
        return _Line(text=line.text, x=-line.y, y=line.x)
    return line


def _hits(lines: list[_Line], words: tuple[str, ...]) -> int:
    text = "".join(line.text for line in lines)
    return sum(1 for word in words if word in text)
