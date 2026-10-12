"""本地真实数据 golden 回归(仅本机;data/ 未脱敏,永不提交)。

冻结:cd recognizer && HDD_DATA_DIR=<主检出>/data uv run python tools/local_golden.py freeze
CI 无 data → 用例跳过;本机跑:
  HDD_DATA_DIR=<主检出>/data uv run pytest tests/test_local_golden.py
"""

import importlib.util
import json
import sys
from pathlib import Path

import pytest

from recognizer import paths
from recognizer.contract import OCRResult
from recognizer.postprocess import run_postprocess

_sample_ids = sorted(p.stem for p in paths.LOCAL_EXPECTED_OCR.glob("*.json")) if paths.LOCAL_EXPECTED_OCR.exists() else []


def _local_golden():
    """tools/local_golden.py 非包内模块,按路径加载(check 的纯函数部分可直接单测)。"""
    spec = importlib.util.spec_from_file_location(
        "local_golden", Path(__file__).resolve().parents[1] / "tools" / "local_golden.py"
    )
    mod = importlib.util.module_from_spec(spec)
    sys.modules["local_golden"] = mod
    spec.loader.exec_module(mod)
    return mod


def test_ocr_comparable_strips_timing_fields() -> None:
    """check --ocr 忽略 elapse/elapse_list 计时字段(每次运行必然不同,不属内容漂移)。"""
    lg = _local_golden()
    payload = {"txts": ["甲"], "boxes": [[[0.0, 0.0]]], "elapse": 1.2345678901234567,
               "elapse_list": [0.2688722920138389, 0.03816208289936185]}
    stripped = lg._ocr_comparable(payload)
    assert "elapse" not in stripped and "elapse_list" not in stripped
    assert "txts" in stripped and "boxes" in stripped
    # 原对象不被修改
    assert "elapse" in payload


@pytest.mark.skipif(
    not _sample_ids,
    reason="no local data golden under data/expected/ocr/ (run tools/local_golden.py freeze)",
)
@pytest.mark.parametrize("sample_id", _sample_ids)
def test_local_report_matches_golden(sample_id: str) -> None:
    ocr = OCRResult.model_validate(
        json.loads((paths.LOCAL_EXPECTED_OCR / f"{sample_id}.json").read_text(encoding="utf-8"))
    )
    report = run_postprocess(ocr)
    golden = json.loads((paths.LOCAL_EXPECTED_REPORT / f"{sample_id}.json").read_text(encoding="utf-8"))
    assert json.loads(report.model_dump_json()) == golden
