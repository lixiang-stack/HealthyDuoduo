"""识别服务 HTTP 入口。

P1:/ocr(图 → OCR 结果,实施计划 4.3/8-3)。
P0 占位 /healthz;P2 起 /report、/reparse。
"""

from pathlib import Path

from fastapi import FastAPI, HTTPException, UploadFile, status
from PIL import UnidentifiedImageError

from .contract import OCRResult
from .ocr import run_ocr

app = FastAPI(title="HealthyDuoduo Recognizer")

_IMAGE_SUFFIXES = {".jpg", ".jpeg", ".png", ".bmp", ".webp", ".tif", ".tiff"}


@app.get("/healthz")
def healthz() -> dict[str, str]:
    return {"status": "ok"}


@app.post("/ocr")
def ocr(image: UploadFile) -> OCRResult:
    """multipart 上传一张图像 → OCR 结果 JSON(与调试 CLI 同一实现)。"""
    if image.filename is None or Path(image.filename).suffix.lower() not in _IMAGE_SUFFIXES:
        raise HTTPException(status.HTTP_415_UNSUPPORTED_MEDIA_TYPE, "unsupported image type")
    data = image.file.read()
    if not data:
        raise HTTPException(status.HTTP_400_BAD_REQUEST, "empty upload")
    try:
        return run_ocr(data)
    except UnidentifiedImageError as exc:
        raise HTTPException(status.HTTP_400_BAD_REQUEST, f"invalid image content: {exc}") from exc
