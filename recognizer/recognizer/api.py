"""识别服务 HTTP 入口。

P1:/ocr(图 → OCR 结果);P2:/report(图 → {ocr_result, report})与 /reparse
(已存 OCR 结果 → report,规则迭代主路径,实施计划 4.3/9-1..9-3)。
P0 占位 /healthz。
P3+(ADR-0004):/report 额外跑表结构识别(TSR);结果随 ocr_result 一并返回/落库,
/reparse 复用它免于再读原图。纯 /ocr 与调试 CLI 保持只做 OCR。
"""

import logging
from pathlib import Path

from fastapi import FastAPI, Form, HTTPException, UploadFile, status
from PIL import UnidentifiedImageError

from .contract import OCRResult, ReparseRequest, Report, ReportResponse
from .ocr import prepare_image, run_ocr
from .postprocess import run_postprocess
from .table_structure import run_table_structure

app = FastAPI(title="HealthyDuoduo Recognizer")

# 服务入口配置日志,使 TSR 决策/退化等 INFO 事件可见(header 可 grep 统计,见集成设计 §3)
logging.basicConfig(level=logging.INFO)

_IMAGE_SUFFIXES = {".jpg", ".jpeg", ".png", ".bmp", ".webp", ".tif", ".tiff"}


def _validated_bytes(image: UploadFile) -> bytes:
    """multipart 上传校验:后缀白名单 + 非空;失败映射为 4xx。"""
    if image.filename is None or Path(image.filename).suffix.lower() not in _IMAGE_SUFFIXES:
        raise HTTPException(status.HTTP_415_UNSUPPORTED_MEDIA_TYPE, "unsupported image type")
    data = image.file.read()
    if not data:
        raise HTTPException(status.HTTP_400_BAD_REQUEST, "empty upload")
    return data


def _read_image(image: UploadFile, preprocess: str | None = None) -> OCRResult:
    """校验 + OCR(与调试 CLI 同一实现);失败映射为 4xx。

    preprocess:可选预处理开关(ocr.parse_preprocess 语义);非法步骤名 → 400。
    """
    data = _validated_bytes(image)
    try:
        return run_ocr(data, preprocess)
    except UnidentifiedImageError as exc:
        raise HTTPException(status.HTTP_400_BAD_REQUEST, f"invalid image content: {exc}") from exc
    except ValueError as exc:
        raise HTTPException(status.HTTP_400_BAD_REQUEST, str(exc)) from exc


@app.get("/healthz")
def healthz() -> dict[str, str]:
    return {"status": "ok"}


@app.post("/ocr")
def ocr(image: UploadFile, preprocess: str | None = Form(default=None)) -> OCRResult:
    """multipart 上传一张图像 → OCR 结果 JSON(与调试 CLI 同一实现,不含表结构)。

    可选 multipart 字段 preprocess:预处理开关(步骤名逗号/空格组合;默认关闭)。
    """
    return _read_image(image, preprocess)


@app.post("/report")
def report(
    image: UploadFile,
    date: str | None = Form(default=None),
    preprocess: str | None = Form(default=None),
) -> ReportResponse:
    """multipart 上传一张图像 → OCR 结果(含表结构识别)+ 规则化报告。

    可选 multipart 字段 date(YYYY-MM-DD):检查单未解析出日期时的人工补录兜底
    (决策 #6;补录后报告不再是「日期缺失」partial);已解析出日期时该参数不生效。
    可选 multipart 字段 preprocess:预处理开关(P3 坏例驱动;默认关闭)。
    OCR 与 TSR 共用同一(预处理后)图像;TSR 退化时 ocr_result.table_structure 为 null。
    """
    data = _validated_bytes(image)
    try:
        prepared = prepare_image(data, preprocess)
        ocr_result = run_ocr(prepared)
        table = run_table_structure(prepared, ocr_result)
    except UnidentifiedImageError as exc:
        raise HTTPException(status.HTTP_400_BAD_REQUEST, f"invalid image content: {exc}") from exc
    except ValueError as exc:
        raise HTTPException(status.HTTP_400_BAD_REQUEST, str(exc)) from exc
    if table is not None:
        ocr_result = ocr_result.model_copy(update={"table_structure": table})
    return ReportResponse(ocr_result=ocr_result, report=run_postprocess(ocr_result, date_hint=date))


@app.post("/reparse")
def reparse(payload: ReparseRequest) -> Report:
    """复用已存 OCR 结果(含表结构)重跑后处理(规则迭代后历史刷新的端点,实施计划 P2-3)。"""
    return run_postprocess(payload.ocr_result, date_hint=payload.date)
