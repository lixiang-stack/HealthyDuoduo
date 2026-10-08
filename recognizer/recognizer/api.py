"""识别服务 HTTP 入口。

P0 占位:仅 /healthz。
P1 起 /ocr(图 → OCR 结果),P2 起 /report、/reparse(实施计划 4.3)。
"""

from fastapi import FastAPI

app = FastAPI(title="HealthyDuoduo Recognizer")


@app.get("/healthz")
def healthz() -> dict[str, str]:
    return {"status": "ok"}
