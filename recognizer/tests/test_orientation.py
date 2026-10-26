"""整页横拍方向纠正:纵向行判定、转向选择、图像转正与 /report 接线。"""

from __future__ import annotations

import base64
import io

from fastapi.testclient import TestClient
from PIL import Image

from recognizer.api import app
from recognizer.contract import OCRResult
from recognizer.orientation import detect_rotation, rotate_image


def _ocr(lines: list[tuple[str, float, float, float, float]]) -> OCRResult:
    txts = [line[0] for line in lines]
    boxes = [
        [
            [x, y],
            [x + w, y],
            [x + w, y + h],
            [x, y + h],
        ]
        for _, x, y, w, h in lines
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


def _vertical_page(header_x: float, footer_x: float) -> OCRResult:
    # 横拍后文字行变成纵向长条;标题/表头与签字的横向位置决定该向哪边转正。
    return _ocr(
        [
            ("西安市人民医院检验报告", header_x, 100, 22, 180),
            ("姓名：范莹莹", header_x, 300, 22, 110),
            ("审核者：", footer_x, 700, 22, 80),
            ("报告日期：2026-09-04", footer_x, 820, 22, 150),
        ]
    )


def test_vertical_page_with_header_on_right_rotates_counter_clockwise() -> None:
    assert detect_rotation(_vertical_page(header_x=900, footer_x=80)) == 90


def test_vertical_page_with_header_on_left_rotates_clockwise() -> None:
    assert detect_rotation(_vertical_page(header_x=80, footer_x=900)) == 270


def test_horizontal_page_is_not_rotated() -> None:
    ocr = _ocr(
        [
            ("检验报告", 100, 50, 180, 22),
            ("血红蛋白", 100, 100, 90, 22),
            ("审核者", 100, 900, 80, 22),
        ]
    )
    assert detect_rotation(ocr) == 0


def _horizontal_page(header_y: float, footer_y: float) -> OCRResult:
    return _ocr(
        [
            ("西安市人民医院检验报告", 100, header_y, 180, 22),
            ("姓名：范莹莹", 100, header_y + 40, 110, 22),
            ("审核者：", 100, footer_y, 80, 22),
            ("报告日期：2026-09-04", 100, footer_y + 40, 150, 22),
        ]
    )


def test_upright_horizontal_page_is_not_rotated() -> None:
    assert detect_rotation(_horizontal_page(header_y=100, footer_y=900)) == 0


def test_upside_down_page_rotates_180() -> None:
    assert detect_rotation(_horizontal_page(header_y=900, footer_y=100)) == 180


def test_few_vertical_lines_do_not_trigger_rotation() -> None:
    ocr = _ocr(
        [
            ("检验报告", 100, 50, 180, 22),
            ("血红蛋白", 100, 100, 90, 22),
            ("白细胞计数", 100, 130, 90, 22),
            ("竖排标签", 100, 200, 22, 80),
        ]
    )
    assert detect_rotation(ocr) == 0


def test_rotate_image_preserves_format_and_swaps_sides() -> None:
    image = Image.new("RGB", (30, 10), "white")
    image.putpixel((0, 0), (0, 0, 0))
    buffer = io.BytesIO()
    image.save(buffer, format="PNG")

    rotated = Image.open(io.BytesIO(rotate_image(buffer.getvalue(), 90)))

    assert rotated.format == "PNG"
    assert rotated.size == (10, 30)
    assert rotated.getpixel((0, 0)) == (255, 255, 255)


def test_rotate_image_180_keeps_size_and_flips_corner() -> None:
    image = Image.new("RGB", (30, 10), "white")
    image.putpixel((0, 0), (0, 0, 0))
    buffer = io.BytesIO()
    image.save(buffer, format="PNG")

    rotated = Image.open(io.BytesIO(rotate_image(buffer.getvalue(), 180)))

    assert rotated.size == (30, 10)
    assert rotated.getpixel((29, 9)) == (0, 0, 0)


def test_report_normalizes_sideways_image(monkeypatch) -> None:
    calls = 0
    sideways = _vertical_page(header_x=900, footer_x=80)
    upright = _ocr([("检验报告", 10, 10, 180, 22)])

    def fake_run_ocr(_: bytes) -> OCRResult:
        nonlocal calls
        calls += 1
        return sideways if calls == 1 else upright

    monkeypatch.setattr("recognizer.api._run_ocr", fake_run_ocr)
    monkeypatch.setattr("recognizer.api.rotate_image", lambda data, rotation: b"rotated-image")

    with TestClient(app) as client:
        response = client.post(
            "/report",
            files={"image": ("report.png", b"original-image", "image/png")},
        )

    assert response.status_code == 200
    body = response.json()
    assert body["image_rotation"] == 90
    assert base64.b64decode(body["normalized_image"]) == b"rotated-image"
    assert calls == 2
