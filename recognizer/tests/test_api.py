"""/ocr 端点测试(实施计划 8-3):multipart 上传 → OCR 结果 JSON;负例边界。
P2:/report(image → {ocr_result, report})与 /reparse(golden OCR → report)契约测试。
"""

import json

import pytest
from fastapi.testclient import TestClient
from jsonschema import Draft202012Validator

from recognizer import paths
from recognizer.api import app
from recognizer.contract import OCRResult
from recognizer.golden import date_hint_for
from recognizer.postprocess import run_postprocess

SAMPLES = paths.SAMPLES
SCHEMA = paths.SCHEMA_OCR
REPORT_SCHEMA = paths.SCHEMA_REPORT
EXPECTED_OCR = paths.EXPECTED_OCR

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
    assert body["image_rotation"] == 0
    assert body["normalized_image"] is None
    Draft202012Validator(json.loads(SCHEMA.read_text(encoding="utf-8"))).validate(body["ocr_result"])
    Draft202012Validator(json.loads(REPORT_SCHEMA.read_text(encoding="utf-8"))).validate(body["report"])
    assert body["report"]["report_type"] == "临检"
    names = {it["name"] for it in body["report"]["items"]}
    assert "血红蛋白" in names


def test_report_rejects_non_image() -> None:
    with TestClient(app) as client:
        response = client.post("/report", files={"image": ("note.txt", b"hello", "text/plain")})
    assert response.status_code == 415


def test_reparse_golden_matches_postprocess() -> None:
    """验收 P2:POST /reparse 用 golden OCR 重跑后处理,与无原图的重跑结果一致。

    /reparse 只有存库 OCR、没有原图像素;OCR 漏掉的孤立 '-' 不能在此恢复。
    """
    if not _golden_ids:
        pytest.skip("no golden OCR files")
    sample_id = _golden_ids[0]
    golden = json.loads((EXPECTED_OCR / f"{sample_id}.json").read_text(encoding="utf-8"))
    hint = date_hint_for(sample_id)
    expected = json.loads(
        run_postprocess(OCRResult.model_validate(golden), date_hint=hint).model_dump_json()
    )
    payload = {"ocr_result": golden}
    if hint is not None:
        payload["date"] = hint
    with TestClient(app) as client:
        response = client.post("/reparse", json=payload)
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


def _classify(client: TestClient, report_type: str, names: list[str]) -> dict:
    response = client.post(
        "/classify",
        json={
            "report_type": report_type,
            "items": [{"name": name, "unit": None} for name in names],
        },
    )
    assert response.status_code == 200, response.text
    return response.json()


def test_classify_mixed_chemistry_items() -> None:
    """/classify 复用同一词典映射:逐项 panel + 报告级排序聚合(spec §6.4)。"""
    with TestClient(app) as client:
        body = _classify(client, "生化", ["丙氨酸转氨酶", "肌酐", "尿酸", "总胆固醇", "维生素B12"])
    assert body["item_panels"] == ["liver", "kidney", "uric_acid", "lipid", "vitanemia"]
    assert body["panels"] == ["liver", "kidney", "lipid", "uric_acid", "vitanemia"]


def test_classify_tie_uses_fixed_priority() -> None:
    with TestClient(app) as client:
        body = _classify(client, "生化", ["尿酸", "总胆固醇", "二氧化碳"])
    assert body["panels"] == ["lipid", "uric_acid", "electrolyte"]


def test_classify_urine_and_biochemistry_ambiguity() -> None:
    """临检葡萄糖 / 胆红素 → 尿常规;生化同名项 → 血糖 / 肝功。"""
    with TestClient(app) as client:
        urine = _classify(client, "临检", ["葡萄糖", "胆红素"])
        blood = _classify(client, "生化", ["葡萄糖", "总胆红素"])
    assert urine["item_panels"] == ["urinalysis", "urinalysis"]
    assert urine["panels"] == ["urinalysis"]
    assert blood["item_panels"] == ["glucose", "liver"]
    assert blood["panels"] == ["liver", "glucose"]


def test_classify_urine_and_cbc_canonical_names() -> None:
    """临检同名指标靠规范名区分:数据库存的是规范名,回填按 name 精确命中。"""
    with TestClient(app) as client:
        body = _classify(
            client,
            "临检",
            ["白细胞计数", "尿白细胞计数", "红细胞计数", "尿红细胞计数"],
        )
    assert body["item_panels"] == ["cbc", "urinalysis", "cbc", "urinalysis"]
    assert body["panels"] == ["cbc", "urinalysis"]


def test_classify_empty_items() -> None:
    with TestClient(app) as client:
        imaging = _classify(client, "影像", [])
        other = _classify(client, "生化", [])
        unknown = _classify(client, "unknown", [])
    assert imaging == {"panels": ["imaging"], "item_panels": []}
    assert other == {"panels": ["other"], "item_panels": []}
    assert unknown == {"panels": ["other"], "item_panels": []}


def test_classify_unknown_item_is_other() -> None:
    with TestClient(app) as client:
        body = _classify(client, "生化", ["人工修正后的未知项目"])
    assert body == {"panels": ["other"], "item_panels": ["other"]}


def test_classify_rejects_unknown_report_type() -> None:
    with TestClient(app) as client:
        response = client.post(
            "/classify",
            json={"report_type": "不存在的类别", "items": [{"name": "血红蛋白"}]},
        )
    assert response.status_code == 422
    assert "unknown report_type" in response.json()["detail"]
