"""App E2E replay fixtures: fixed golden OCR/report looked up by image sha256."""

import json

import pytest
from fastapi.testclient import TestClient

from recognizer import api, paths
from recognizer.contract import OCRResult, Report
from recognizer.replay import Replay

ENV = "RECOGNIZER_REPLAY_SAMPLES"
ENV_IMAGES = "RECOGNIZER_REPLAY_IMAGES"


def _sample(name: str) -> bytes:
    return next(paths.SAMPLES.glob(f"{name}.*")).read_bytes()


def test_lookup_returns_golden_by_image_sha() -> None:
    replay = Replay(["cbc_03"])
    fixture = replay.lookup(_sample("cbc_03"))
    assert fixture is not None
    assert fixture.ocr_result == OCRResult.model_validate(
        json.loads((paths.EXPECTED_OCR / "cbc_03.json").read_text(encoding="utf-8"))
    )
    assert fixture.report == Report.model_validate(
        json.loads((paths.EXPECTED_REPORT / "cbc_03.json").read_text(encoding="utf-8"))
    )


def test_lookup_misses_unknown_image() -> None:
    replay = Replay(["cbc_03"])
    assert replay.lookup(b"not a sample image") is None


def test_missing_sample_fails_loudly() -> None:
    with pytest.raises(FileNotFoundError, match="missing-sample"):
        Replay(["missing-sample"])


def test_extra_image_dir_replays_masked_sample() -> None:
    """掩码语料无原图:E2E 替身图目录按样本 id 提供图片,仍回放同一份 golden。"""
    image_dir = paths.REPO_ROOT / "apps/mobile/e2e/replay"
    image = image_dir / "bloodtype_01.png"

    replay = Replay(["bloodtype_01"], [image_dir])

    fixture = replay.lookup(image.read_bytes())
    assert fixture is not None
    assert fixture.report == Report.model_validate(
        json.loads((paths.EXPECTED_REPORT / "bloodtype_01.json").read_text(encoding="utf-8"))
    )


def test_from_env_uses_extra_image_dir(monkeypatch: pytest.MonkeyPatch) -> None:
    image_dir = paths.REPO_ROOT / "apps/mobile/e2e/replay"
    image = image_dir / "bloodtype_01.png"
    monkeypatch.setenv(ENV, "bloodtype_01")
    monkeypatch.setenv(ENV_IMAGES, str(image_dir))

    replay = Replay.from_env()

    assert replay is not None
    assert replay.lookup(image.read_bytes()) is not None


def test_from_env_disabled(monkeypatch: pytest.MonkeyPatch) -> None:
    monkeypatch.delenv(ENV, raising=False)
    assert Replay.from_env() is None
    monkeypatch.setenv(ENV, "  ")
    assert Replay.from_env() is None


def test_from_env_parses_sample_list(monkeypatch: pytest.MonkeyPatch) -> None:
    monkeypatch.setenv(ENV, " cbc_03 , us_01 ")
    replay = Replay.from_env()
    assert replay is not None
    assert replay.lookup(_sample("cbc_03")) is not None
    assert replay.lookup(_sample("us_01")) is not None


def test_report_endpoint_serves_replay_fixture(monkeypatch: pytest.MonkeyPatch) -> None:
    monkeypatch.setattr(api, "_replay", Replay(["cbc_03"]))
    with TestClient(api.app) as client:
        response = client.post(
            "/report",
            files={"image": ("cbc_03.png", _sample("cbc_03"), "image/png")},
        )
    assert response.status_code == 200
    assert response.json()["report"] == json.loads(
        (paths.EXPECTED_REPORT / "cbc_03.json").read_text(encoding="utf-8")
    )


def test_report_endpoint_falls_back_to_ocr_on_miss(monkeypatch: pytest.MonkeyPatch) -> None:
    """回放名单外的图像仍走真实 OCR 分支(用 golden OCR 替身,不跑模型)。"""
    monkeypatch.setattr(api, "_replay", Replay(["cbc_03"]))
    ocr = OCRResult.model_validate(
        json.loads((paths.EXPECTED_OCR / "cbc_01.json").read_text(encoding="utf-8"))
    )
    monkeypatch.setattr(api, "_run_ocr", lambda data: ocr)
    with TestClient(api.app) as client:
        response = client.post(
            "/report",
            files={"image": ("cbc_02.png", _sample("cbc_02"), "image/png")},
        )
    assert response.status_code == 200
    assert response.json()["ocr_result"]["txts"] == ocr.txts
    assert response.json()["report"]["items"]
