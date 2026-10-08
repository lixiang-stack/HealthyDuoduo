"""/ocr 端点测试(实施计划 8-3):multipart 上传 → OCR 结果 JSON;负例边界。"""

import json
from pathlib import Path

from fastapi.testclient import TestClient
from jsonschema import Draft202012Validator

from recognizer.api import app

REPO_ROOT = Path(__file__).resolve().parents[2]
SAMPLES = REPO_ROOT / "samples"
SCHEMA = REPO_ROOT / "schemas" / "ocr_result.schema.json"


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
