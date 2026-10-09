"""调试 CLI(实施计划 8-2)错误路径:参数错 → 2;图像读不了 → 1;成功 → 0 且 stdout 为契约 JSON。"""

import json
import shutil

from recognizer.__main__ import main
from tests.test_ocr_golden import SAMPLES


def test_cli_usage_error_exits_2(capsys) -> None:
    assert main([]) == 2
    assert main(["a", "b"]) == 2
    assert "usage:" in capsys.readouterr().err


def test_cli_unreadable_image_exits_1(capsys) -> None:
    assert main(["/nonexistent/image.jpg"]) == 1
    err = capsys.readouterr().err
    assert "cannot read image" in err
    assert "/nonexistent/image.jpg" in err


def test_cli_success_prints_contract_json(tmp_path, capsys) -> None:
    """tmp_path 输入 → stdout 为 OCR JSON(engine/model_info 来自配置)。"""
    image = next(iter(SAMPLES.glob("cbc_02.*")))
    target = tmp_path / image.name
    shutil.copy(image, target)

    assert main([str(target)]) == 0
    result = json.loads(capsys.readouterr().out)
    assert result["engine"] == "onnxruntime"
    assert result["model_info"] == {"det": "PP-OCRv6_small", "cls": "PP-OCRv4_mobile", "rec": "PP-OCRv6_small"}
