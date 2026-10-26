"""纸面短横线恢复:OCR 漏掉孤立的 '-' 时,用原图像素确认结果/参考两格都是短横线。"""

import io

from PIL import Image, ImageDraw

from recognizer.contract import OCRResult
from recognizer.postprocess import run_postprocess

# 合成尿常规单:结果/参考两列本来就是 '-' 的定性项;OCR 只输出名称,不输出短横线。
_CELLS = [
    {"x": 100, "y": -60, "text": "尿液分析报告"},
    {"x": 500, "y": -60, "text": "报告时间：2026-09-04 09:00"},
    {"x": 20, "y": 0, "text": "序号"},
    {"x": 160, "y": 0, "text": "项目名称"},
    {"x": 360, "y": 0, "text": "结果"},
    {"x": 470, "y": 0, "text": "单位"},
    # 参考列故意做成宽表头:恢复判定不能按裁剪宽度放大最小面积(真实尿常规列更宽)。
    {"x": 580, "y": 0, "w": 500, "text": "参考值"},
    {"x": 12, "y": 30, "w": 20, "text": "1"},
    {"x": 150, "y": 30, "w": 90, "text": "尿潜血"},
]


def _ocr() -> OCRResult:
    txts = [cell["text"] for cell in _CELLS]
    boxes = [
        [
            [cell["x"], cell["y"]],
            [cell["x"] + cell.get("w", 60), cell["y"]],
            [cell["x"] + cell.get("w", 60), cell["y"] + cell.get("h", 20)],
            [cell["x"], cell["y"] + cell.get("h", 20)],
        ]
        for cell in _CELLS
    ]
    return OCRResult.model_validate(
        {
            "txts": txts,
            "boxes": boxes,
            "scores": [1.0] * len(txts),
            "elapse": 0.0,
            "elapse_list": [0.0] * len(txts),
            "engine": "onnxruntime",
            "model_info": {"det": "d", "cls": "c", "rec": "r"},
        }
    )


def _image(value: str = "dash", ref: str = "dash", next_row_dash: bool = False, thin: bool = False) -> bytes:
    image = Image.new("L", (1200, 120), 255)
    draw = ImageDraw.Draw(image)
    height = 0 if thin else 2
    if value == "dash":
        draw.rectangle((370, 36, 380 if thin else 395, 36 + height), fill=0)
    elif value == "plus":
        draw.rectangle((370, 36, 395, 38), fill=0)
        draw.rectangle((381, 30, 383, 46), fill=0)
    if ref == "dash":
        draw.rectangle((590, 36, 600 if thin else 615, 36 + height), fill=0)
    if next_row_dash:
        # 下一行的 '-' 必须落在当前行 ROI 之外,不能串到当前项。
        draw.rectangle((370, 62, 395, 64), fill=0)
    buffer = io.BytesIO()
    image.save(buffer, format="PNG")
    return buffer.getvalue()


def _item(report):
    return next(item for item in report.items if item.name == "潜血")


def test_both_dash_cells_recover_as_not_tested() -> None:
    report = run_postprocess(_ocr(), image=_image(next_row_dash=True))
    item = _item(report)
    assert item.value == "-"
    assert item.ref_range == "-"
    assert item.flag == "not_tested"
    assert item.flag_reason is None
    assert report.status == "success"


def test_thin_one_pixel_dash_cells_recover_as_not_tested() -> None:
    """低分辨率短横线可能只有 1px 高;面积阈值不能把它当噪点丢掉。"""
    report = run_postprocess(_ocr(), image=_image(thin=True))
    item = _item(report)
    assert item.value == "-"
    assert item.ref_range == "-"
    assert item.flag == "not_tested"
    assert report.status == "success"


def test_missed_plus_is_not_inferred_as_dash() -> None:
    report = run_postprocess(_ocr(), image=_image(value="plus"))
    item = _item(report)
    assert item.value is None
    assert item.ref_range == "-"
    assert item.flag == "unknown"
    assert report.status == "partial"


def test_blank_cells_stay_missing() -> None:
    report = run_postprocess(_ocr(), image=_image(value="blank", ref="blank"))
    item = _item(report)
    assert item.value is None
    assert item.ref_range is None
    assert item.flag == "unknown"
    assert report.status == "partial"


def test_without_image_does_not_guess() -> None:
    """只有 OCR、没有原图(/reparse 路径)时不补短横线。"""
    report = run_postprocess(_ocr())
    item = _item(report)
    assert item.value is None
    assert item.ref_range is None
    assert report.status == "partial"
