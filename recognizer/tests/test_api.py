"""识别服务 HTTP 端点测试(P0:/healthz 占位)。"""

from fastapi.testclient import TestClient

from recognizer.api import app


def test_healthz() -> None:
    with TestClient(app) as client:
        response = client.get("/healthz")
    assert response.status_code == 200
    assert response.json() == {"status": "ok"}
