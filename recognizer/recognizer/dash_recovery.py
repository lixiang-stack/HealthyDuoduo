"""从原图像素恢复 OCR 漏掉的纸面短横线。

尿常规等定性表里,结果/参考列常印孤立的 '-'。RapidOCR 的检测模型对这类
细短线常常不输出;若直接把「缺失」推断为 '-',会把 OCR 漏掉的 '+' 也误判
成正常。本模块只在原图像素确认单元格是单条水平短横线时才补 '-',并且只在
单表单组表头时启用;多表/表头不唯一时保守跳过。
"""

from __future__ import annotations

import io
from dataclasses import dataclass

from PIL import Image, UnidentifiedImageError

from .contract import OCRResult, ReportItem

# 灰度阈值:纸张背景远亮于墨迹,短横线是深色细线。
_DARK_THRESHOLD = 160
# 表头固定文本;与 postprocess 的列角色词一致。
_VALUE_HEADERS = ("结果", "测定结果", "测定值")
_REF_HEADERS = ("参考区间", "参考范围", "参考值")


@dataclass(frozen=True)
class _Box:
    x0: float
    y0: float
    x1: float
    y1: float


def recover_dash_fields(
    image: bytes, ocr: OCRResult, items: list[ReportItem]
) -> dict[int, tuple[str | None, str | None]]:
    """返回 {item 下标: (value, ref_range)};只补当前缺失的字段。

    表头不唯一(多表/重复表头)或图片不可读时返回空,不猜测。
    """
    value_header = _single_header(ocr, _VALUE_HEADERS)
    ref_header = _single_header(ocr, _REF_HEADERS)
    if value_header is None or ref_header is None:
        return {}
    try:
        gray = Image.open(io.BytesIO(image)).convert("L")
    except (UnidentifiedImageError, OSError):
        return {}
    header_h = max(value_header.y1 - value_header.y0, ref_header.y1 - ref_header.y0)
    if header_h <= 0:
        return {}
    value_x = _column_bounds(value_header, header_h)
    ref_x = _column_bounds(ref_header, header_h)
    updates: dict[int, tuple[str | None, str | None]] = {}
    for index, item in enumerate(items):
        if item.value is not None and item.ref_range is not None:
            continue
        if not item.ocr_line_numbers:
            continue
        line = item.ocr_line_numbers[0]
        if line < 0 or line >= len(ocr.boxes):
            continue
        row = _row_band(_box(ocr.boxes[line]))
        if row is None:
            continue
        value = item.value
        ref = item.ref_range
        if value is None and _has_dash(gray, value_x, row):
            value = "-"
        if ref is None and _has_dash(gray, ref_x, row):
            ref = "-"
        if (value, ref) != (item.value, item.ref_range):
            updates[index] = (value, ref)
    return updates


def _single_header(ocr: OCRResult, labels: tuple[str, ...]) -> _Box | None:
    found = [
        _box(box)
        for text, box in zip(ocr.txts, ocr.boxes, strict=True)
        if text.strip() in labels
    ]
    return found[0] if len(found) == 1 else None


def _box(points: list[list[float]]) -> _Box:
    xs = [point[0] for point in points]
    ys = [point[1] for point in points]
    return _Box(min(xs), min(ys), max(xs), max(ys))


def _column_bounds(header: _Box, header_h: float) -> tuple[int, int]:
    """按表头高度外扩,给数据格留出左右余量(短横线靠左,右余量不必覆盖整列)。"""
    left = max(0, int(header.x0 - 0.25 * header_h))
    right = int(header.x1 + 0.60 * header_h)
    return left, right


def _row_band(name: _Box) -> tuple[float, float] | None:
    """当前行纵向 ROI:短横线约在名称格中心偏上;下一行文字在中心偏下,排除。"""
    half = (name.y1 - name.y0) / 2
    if half <= 0:
        return None
    center = (name.y0 + name.y1) / 2
    return center - 0.65 * half, center + 0.40 * half


def _has_dash(gray: Image.Image, x_bounds: tuple[int, int], y_band: tuple[float, float]) -> bool:
    width, height = gray.size
    x0 = max(0, min(width, x_bounds[0]))
    x1 = max(0, min(width, x_bounds[1]))
    y0 = max(0, min(height, int(y_band[0])))
    y1 = max(0, min(height, int(y_band[1])))
    if x1 - x0 < 8 or y1 - y0 < 3:
        return False
    crop = gray.crop((x0, y0, x1, y1))
    w, h = crop.size
    pixels = crop.tobytes()
    mask = [[pixels[y * w + x] < _DARK_THRESHOLD for x in range(w)] for y in range(h)]
    h_limit = max(4, int(0.30 * h))
    min_area = max(12, int(0.30 * h))
    flats = 0
    for component_w, component_h, area in _components(mask, w, h):
        if area < min_area:
            continue
        # 同格出现文字/加号等纵向笔画:不是纯短横线,不补。
        if component_h > h_limit:
            return False
        if component_w >= 8 and component_w / component_h >= 3 and component_w <= 0.6 * w:
            flats += 1
    return flats == 1


def _components(mask: list[list[bool]], width: int, height: int) -> list[tuple[int, int, int]]:
    """8 邻域连通块 → [(宽, 高, 面积)];ROI 很小,纯 Python 足够。"""
    seen = [[False] * width for _ in range(height)]
    components: list[tuple[int, int, int]] = []
    for y in range(height):
        for x in range(width):
            if not mask[y][x] or seen[y][x]:
                continue
            stack = [(x, y)]
            seen[y][x] = True
            min_x = max_x = x
            min_y = max_y = y
            area = 0
            while stack:
                cx, cy = stack.pop()
                area += 1
                min_x = min(min_x, cx)
                max_x = max(max_x, cx)
                min_y = min(min_y, cy)
                max_y = max(max_y, cy)
                for ny in (cy - 1, cy, cy + 1):
                    for nx in (cx - 1, cx, cx + 1):
                        if 0 <= nx < width and 0 <= ny < height and mask[ny][nx] and not seen[ny][nx]:
                            seen[ny][nx] = True
                            stack.append((nx, ny))
            components.append((max_x - min_x + 1, max_y - min_y + 1, area))
    return components
