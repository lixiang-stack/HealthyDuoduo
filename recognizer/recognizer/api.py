"""识别服务 HTTP 入口。

P1:/ocr(图 → OCR 结果);P2:/report(图 → {ocr_result, report})与 /reparse
(已存 OCR 结果 → report,规则迭代主路径,实施计划 4.3/9-1..9-3)。
P0 占位 /healthz。
"""

from pathlib import Path

from fastapi import FastAPI, Form, HTTPException, UploadFile, status
from PIL import UnidentifiedImageError

from .contract import OCRResult, ReparseRequest, Report, ReportResponse
from .ocr import run_ocr
from .postprocess import run_postprocess

app = FastAPI(title="HealthyDuoduo Recognizer")

_IMAGE_SUFFIXES = {".jpg", ".jpeg", ".png", ".bmp", ".webp", ".tif", ".tiff"}


def _read_image(image: UploadFile) -> OCRResult:
    """multipart 上传校验 + OCR(与调试 CLI 同一实现);失败映射为 4xx。"""
    if image.filename is None or Path(image.filename).suffix.lower() not in _IMAGE_SUFFIXES:
        raise HTTPException(status.HTTP_415_UNSUPPORTED_MEDIA_TYPE, "unsupported image type")
    data = image.file.read()
    if not data:
        raise HTTPException(status.HTTP_400_BAD_REQUEST, "empty upload")
    try:
        return run_ocr(data)
    except UnidentifiedImageError as exc:
        raise HTTPException(status.HTTP_400_BAD_REQUEST, f"invalid image content: {exc}") from exc


@app.get("/healthz")
def healthz() -> dict[str, str]:
    return {"status": "ok"}


@app.post("/ocr")
def ocr(image: UploadFile) -> OCRResult:
    """multipart 上传一张图像 → OCR 结果 JSON(与调试 CLI 同一实现)。"""
    return _read_image(image)


@app.post("/report")
def report(image: UploadFile, date: str | None = Form(default=None)) -> ReportResponse:
    """multipart 上传一张图像 → OCR 结果 + 规则化报告。

    可选 multipart 字段 date(YYYY-MM-DD):检查单未解析出日期时的人工补录兜底
    (决策 #6;补录后报告不再是「日期缺失」partial);已解析出日期时该参数不生效。
    """
    ocr_result = _read_image(image)
    report = run_postprocess(ocr_result, date_hint=date)
    return ReportResponse(ocr_result=ocr_result, report=report)


@app.post("/reparse")
def reparse(payload: ReparseRequest) -> Report:
    """复用已存 OCR 结果重跑后处理(规则迭代后历史刷新的端点,实施计划 P2-3)。"""
    return run_postprocess(payload.ocr_result, date_hint=payload.date)
