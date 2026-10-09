"""/ocr 端点测试(实施计划 8-3):multipart 上传 → OCR 结果 JSON;负例边界。
P2:/report(image → {ocr_result, report})与 /reparse(golden OCR → report)契约测试。
"""

import json
from pathlib import Path

from fastapi.testclient import TestClient
from jsonschema import Draft202012Validator

from recognizer.api import app

REPO_ROOT = Path(__file__).resolve().parents[2]
SAMPLES = REPO_ROOT / "samples"
SCHEMA = REPO_ROOT / "schemas" / "ocr_result.schema.json"
REPORT_SCHEMA = REPO_ROOT / "schemas" / "report.schema.json"
EXPECTED_OCR = SAMPLES / "expected" / "ocr"

_golden_ids = sorted(p.stem for p in EXPECTED_OCR.glob("*.json"))


def test_healthz() -> None:
    with TestClient(app) as client:
        response = client.get("/healthz")
    assert response.status_code == 200
    assert response.json() == {"status": "ok"}


def test_ocr_response_passes_contract_schema() -> None:
    """验收 8:POST /ocr 输出通过 schemas/ocr_result.schema.json 校验。"""
    image = next(iter(SAMPLES.glob("cbc_02.*")))
    with TestClient(app) as client:
        response = client.post(
            "/ocr",
            files={"image": (image.name, image.read_bytes(), "image/jpeg")},
        )
    assert response.status_code == 200
    schema = json.loads(SCHEMA.read_text(encoding="utf-8"))
    Draft202012Validator(schema).validate(response.json())


def test_ocr_rejects_non_image_filename() -> None:
    with TestClient(app) as client:
        response = client.post(
            "/ocr",
            files={"image": ("note.txt", b"not an image", "text/plain")},
        )
    assert response.status_code == 415


def test_ocr_rejects_empty_upload() -> None:
    with TestClient(app) as client:
        response = client.post(
            "/ocr",
            files={"image": ("empty.png", b"", "image/png")},
        )
    assert response.status_code == 400


def test_ocr_rejects_corrupt_content() -> None:
    with TestClient(app) as client:
        response = client.post(
            "/ocr",
            files={"image": ("broken.png", b"definitely not image bytes", "image/png")},
        )
    assert response.status_code == 400


def test_report_bundle_contract() -> None:
    """验收 P2:POST /report 返回 {ocr_result, report},两份都通过各自 schema。"""
    image = next(iter(SAMPLES.glob("cbc_01.*")))
    with TestClient(app) as client:
        response = client.post(
            "/report",
            files={"image": (image.name, image.read_bytes(), "image/jpeg")},
        )
    assert response.status_code == 200
    body = response.json()
    Draft202012Validator(json.loads(SCHEMA.read_text(encoding="utf-8"))).validate(body["ocr_result"])
    Draft202012Validator(json.loads(REPORT_SCHEMA.read_text(encoding="utf-8"))).validate(body["report"])
    assert body["report"]["report_type"] == "血常规"
    names = {it["name"] for it in body["report"]["items"]}
    assert "血红蛋白" in names


def test_report_rejects_non_image() -> None:
    with TestClient(app) as client:
        response = client.post("/report", files={"image": ("note.txt", b"hello", "text/plain")})
    assert response.status_code == 415


def test_reparse_golden_matches_postprocess() -> None:
    """验收 P2:POST /reparse 用 golden OCR 重跑后处理,与 golden report 一致。"""
    import pytest

    if not _golden_ids:
        pytest.skip("no golden OCR files")
    golden = json.loads((EXPECTED_OCR / f"{_golden_ids[0]}.json").read_text(encoding="utf-8"))
    expected = json.loads((SAMPLES / "expected" / "report" / f"{_golden_ids[0]}.json").read_text(encoding="utf-8"))
    with TestClient(app) as client:
        response = client.post("/reparse", json={"ocr_result": golden})
    assert response.status_code == 200
    assert response.json() == expected


def test_reparse_date_hint_only_fills_missing() -> None:
    golden = json.loads((EXPECTED_OCR / "cbc_02.json").read_text(encoding="utf-8"))
    with TestClient(app) as client:
        response = client.post("/reparse", json={"ocr_result": golden, "date": "2026-01-02"})
    assert response.status_code == 200
    # cbc_02 无日期 → hint 补录生效
    assert response.json()["report_date"] == "2026-01-02"


def test_reparse_rejects_bad_contract() -> None:
    with TestClient(app) as client:
        response = client.post("/reparse", json={"ocr_result": {"txts": ["x"]}})
    assert response.status_code == 422
