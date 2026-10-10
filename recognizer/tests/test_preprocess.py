"""P3 预处理开关:开关解析 / 纠偏角估计 / run_ocr 集成。

默认关闭 → OCR 输入与 P1 完全一致(golden 三通道用例依旧锚定);
开启 → 按序执行灰度/对比度/纠偏,输出契约(OCRResult)不变。
纠偏用例:合成「横条文字」图绕轴旋转,估计出的校正角须把倾斜拉平。
"""

import json
import shutil

import pytest
from PIL import Image, ImageDraw

from recognizer.__main__ import main
from recognizer.ocr import _estimate_skew_deg, parse_preprocess, run_ocr
from tests.test_ocr_golden import SAMPLES

# ---------- parse_preprocess ----------


def test_parse_preprocess_default_off() -> None:
    assert parse_preprocess(None) == ()
    assert parse_preprocess("") == ()
    assert parse_preprocess("  ") == ()


def test_parse_preprocess_combo_dedups() -> None:
    assert parse_preprocess("gray, deskew") == ("gray", "deskew")
    assert parse_preprocess("deskew;autocontrast") == ("deskew", "autocontrast")
    assert parse_preprocess("autocontrast autocontrast") == ("autocontrast",)


def test_parse_preprocess_unknown_rejected() -> None:
    with pytest.raises(ValueError):
        parse_preprocess("sharpen")

# ---------- 纠偏角估计 ----------


def _bar_image(w: int = 600, h: int = 400) -> Image.Image:
    """白底横条文字仿真(检查单行版式):横条为前景,背景白。"""
    img = Image.new("RGB", (w, h), "white")
    draw = ImageDraw.Draw(img)
    for y in range(40, h - 40, 40):
        draw.rectangle([60, y, w - 60, y + 10], fill="black")
    return img


def test_deskew_estimate_flat_image_is_near_zero() -> None:
    assert abs(_estimate_skew_deg(_bar_image())) <= 0.5


def test_deskew_estimate_recovers_preset_tilt() -> None:
    """内容被逆时针 -3° 拍歪 → 估计出 +3° 左右的校正角(0.5° 搜索步长容差)。"""
    tilted = _bar_image().rotate(-3, resample=Image.Resampling.BICUBIC, fillcolor="white")
    est = _estimate_skew_deg(tilted)
    assert 2.4 <= est <= 3.6


# ---------- run_ocr 集成与 CLI ----------


def test_run_ocr_with_preprocess_keeps_contract(tmp_path) -> None:
    image = next(iter(SAMPLES.glob("cbc_02.*")))
    target = tmp_path / image.name
    shutil.copy(image, target)
    result = run_ocr(target, preprocess="gray,autocontrast,deskew")
    assert result.engine == "onnxruntime"
    assert all(0 <= s <= 1 for s in result.scores)
    result.model_validate_json(result.model_dump_json())  # 契约建模仍成立


def test_cli_rejects_unknown_preprocess_step(capsys) -> None:
    assert main(["x.jpeg", "--preprocess", "sharpen"]) == 2
    assert "unknown preprocess step" in capsys.readouterr().err


def test_cli_rejects_preprocess_without_value(capsys) -> None:
    assert main(["x.jpeg", "--preprocess"]) == 2
    assert "--preprocess needs a value" in capsys.readouterr().err


def test_cli_preprocess_runs(tmp_path, capsys) -> None:
    """--preprocess 生效:成功输出契约 JSON(引擎与预处理无关)。"""
    image = next(iter(SAMPLES.glob("cbc_02.*")))
    target = tmp_path / image.name
    shutil.copy(image, target)
    assert main([str(target), "--preprocess", "gray"]) == 0
    result = json.loads(capsys.readouterr().out)
    assert result["engine"] == "onnxruntime"
