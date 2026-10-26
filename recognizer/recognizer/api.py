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
from .paths import IMAGE_SUFFIXES
from .postprocess import run_postprocess
from .replay import Replay

app = FastAPI(title="HealthyDuoduo Recognizer")

# App E2E 专用栈按环境变量启用固定样本回放;缺省 None = 全部真实 OCR。
_replay = Replay.from_env()


def _read_image_bytes(image: UploadFile) -> bytes:
    """multipart 上传校验:扩展名/非空;返回原始字节。"""
    if image.filename is None or Path(image.filename).suffix.lower() not in IMAGE_SUFFIXES:
        raise HTTPException(status.HTTP_415_UNSUPPORTED_MEDIA_TYPE, "unsupported image type")
    data = image.file.read()
    if not data:
        raise HTTPException(status.HTTP_400_BAD_REQUEST, "empty upload")
    return data


def _run_ocr(data: bytes) -> OCRResult:
    """OCR(与调试 CLI 同一实现);失败映射为 4xx。"""
    try:
        return run_ocr(data)
    except UnidentifiedImageError as exc:
        raise HTTPException(status.HTTP_400_BAD_REQUEST, f"invalid image content: {exc}") from exc
    except ValueError as exc:
        raise HTTPException(status.HTTP_400_BAD_REQUEST, str(exc)) from exc


@app.get("/healthz")
def healthz() -> dict[str, str]:
    return {"status": "ok"}


@app.post("/ocr")
def ocr(image: UploadFile) -> OCRResult:
    """multipart 上传一张图像 → OCR 结果 JSON(与调试 CLI 同一实现)。"""
    return _run_ocr(_read_image_bytes(image))


@app.post("/report")
def report(
    image: UploadFile,
    date: str | None = Form(default=None),
) -> ReportResponse:
    """multipart 上传一张图像 → OCR 结果 + 规则化报告。

    可选 multipart 字段 date(YYYY-MM-DD):检查单未解析出日期时的人工补录兜底
    (决策 #6;补录后报告不再是「日期缺失」partial);已解析出日期时该参数不生效。

    App E2E 回放命中固定样本时直接返回预置 golden(date 不参与:fixture 是固定产物);
    未命中仍走真实 OCR。生产/共享栈未配置回放,行为不变。
    """
    data = _read_image_bytes(image)
    if _replay is not None:
        fixture = _replay.lookup(data)
        if fixture is not None:
            return fixture
    ocr_result = _run_ocr(data)
    report = run_postprocess(ocr_result, date_hint=date, image=data)
    return ReportResponse(ocr_result=ocr_result, report=report)


@app.post("/reparse")
def reparse(payload: ReparseRequest) -> Report:
    """复用已存 OCR 结果重跑后处理(规则迭代后历史刷新的端点,实施计划 P2-3)。

    只有存库 OCR、没有原图像素:OCR 漏掉的孤立 '-' 无法在此恢复,需走 /report
    重新识别(或 App 删除后重传)。
    """
    return run_postprocess(payload.ocr_result, date_hint=payload.date)
