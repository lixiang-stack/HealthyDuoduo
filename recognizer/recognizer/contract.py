"""JSON 契约的 pydantic 建模,镜像 schemas/*.schema.json(NF-06 双语契约)。

本模块是识别服务的契约源:OCR 结果与报告结构均以此为准;
schema/ 下 JSON 文件是双语共用的源头,本文件与其保持字段一一对应。
"""

from typing import Literal

from pydantic import BaseModel, ConfigDict, Field, model_validator

# status / flag enums (implementation plan 4.2)
Status = Literal["success", "partial", "failed"]
Flag = Literal["normal", "high", "low", "unknown"]

# Vertex coordinate [x, y]; a text line box is a quadrilateral of four points.
type Point = tuple[float, float]


type Box = tuple[Point, Point, Point, Point]


class ModelInfo(BaseModel):
    """OCR 所用模型(det / cls / rec)。"""

    model_config = ConfigDict(extra="forbid")

    det: str
    cls: str
    rec: str


class OCRResult(BaseModel):
    """对一张图像执行一次 OCR 得到的原始识别输出(实施计划 4.1)。"""

    model_config = ConfigDict(extra="forbid")

    txts: list[str]
    boxes: list[Box]
    scores: list[float]
    elapse: float = Field(ge=0, description="total elapsed seconds; recorded but not compared in golden tests")
    elapse_list: list[float]
    engine: Literal["onnxruntime"]
    model_info: ModelInfo

    @model_validator(mode="after")
    def _equal_lengths(self) -> "OCRResult":
        if not (len(self.txts) == len(self.boxes) == len(self.scores)):
            raise ValueError("txts, boxes and scores must be pairwise equal in length")
        return self


class ReportItem(BaseModel):
    """指标项(实施计划 4.2)。"""

    model_config = ConfigDict(extra="forbid")

    name: str = Field(min_length=1, description="canonical item name (after dictionary mapping)")
    value: float | None = None
    unit: str | None = None
    ref_range: str | None = None
    flag: Flag
    low_confidence: bool
    raw_text: str = Field(min_length=1, description="matching raw OCR line text for manual review")


class Report(BaseModel):
    """系统对一张图像处理后产出的结构化报告(report_id 等由 Go 落库时赋予)。"""

    model_config = ConfigDict(extra="forbid")

    report_type: str = Field(min_length=1)
    report_date: str | None = Field(default=None, pattern=r"^\d{4}-\d{2}-\d{2}$")
    status: Status
    items: list[ReportItem]


class ReportBundle(BaseModel):
    """POST /report 的响应体:一次请求内 OCR 结果 + 结构化报告(实施计划 4.3)。"""

    model_config = ConfigDict(extra="forbid")

    ocr_result: OCRResult
    report: Report


class ReparseRequest(BaseModel):
    """POST /reparse 的请求体:复用已存 OCR 结果重跑后处理(date 为人工补录兜底)。"""

    model_config = ConfigDict(extra="forbid")

    ocr_result: OCRResult
    date: str | None = Field(default=None, pattern=r"^\d{4}-\d{2}-\d{2}$")
