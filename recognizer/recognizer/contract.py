"""JSON 契约的 pydantic 建模,镜像 schemas/*.schema.json(NF-06 双语契约)。

本模块是识别服务的契约源:OCR 结果与报告结构均以此为准;
schema/ 下 JSON 文件是双语共用的源头,本文件与其保持字段一一对应。
"""

from enum import Enum
from typing import Literal

from pydantic import BaseModel, ConfigDict, Field, model_serializer, model_validator


# status / flag 枚举(实施计划 4.2)。str 子类:与 JSON 字面值直接相等(s == "success" 成立),
# 序列化保持字符串形式,与 schemas/*.schema.json 与 Go 侧字符串契约一致。
class _Vocabulary(str, Enum):
    pass

class Status(_Vocabulary):
    """报告状态(4.2 枚举)。"""
    SUCCESS = "success"
    PARTIAL = "partial"
    FAILED = "failed"

class Flag(_Vocabulary):
    """指标项标记(值 vs 参考范围)。

    high/low 只用于有方向的数值异常;abnormal 只用于无方向的定性异常;
    unknown 只表示本应能比较但失败/暂不支持。
    """
    NORMAL = "normal"
    HIGH = "high"
    LOW = "low"
    ABNORMAL = "abnormal"
    UNKNOWN = "unknown"


class FlagReason(_Vocabulary):
    """unknown 的原因(服务端计算;normal/high/low/abnormal 为 None)。"""
    OCR_MISS = "ocr_miss"
    OCR_MERGE = "ocr_merge"
    EXTRACTOR_PAIRING = "extractor_pairing"
    NO_REFERENCE = "no_reference"
    QUALITATIVE_UNSUPPORTED = "qualitative_unsupported"
    UNVERIFIED = "unverified"
    NARRATIVE = "narrative"
    INFORMATIONAL = "informational"
    INDETERMINATE = "indeterminate"
    SCALAR_REF = "scalar_ref"
    OTHER = "other"

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
    value: str | None = Field(
        None,
        description="value as printed (e.g. '128', '0.43', '<2.00'); None when the cell has no value-like content",
    )
    unit: str | None = None
    ref_range: str | None = None
    method: str | None = Field(None, description="detection/experimental method as printed")
    flag: Flag
    flag_reason: FlagReason | None = Field(
        None, description="unknown 的原因;normal/high/low/abnormal 为 None"
    )
    low_confidence: bool
    raw_text: str = Field(min_length=1, description="matching raw OCR line text for manual review")
    ocr_line_numbers: list[int] = Field(
        description="indices of the OCR lines composing this item "
        "(0-based, same as txts/boxes; ordered like raw_text)"
    )

    @model_serializer(mode="wrap")
    def _omit_null_flag_reason(self, handler):
        # flag_reason 只对 unknown 有意义;normal/high/low/abnormal 不输出该键,
        # 保持既有 golden/消费方逐字节兼容。
        data = handler(self)
        if data.get("flag_reason") is None:
            data.pop("flag_reason", None)
        return data


class ReportSection(BaseModel):
    """报告级叙述段落(仅叙述型报告,如影像单;只读展示,不参与指标项核对)。"""

    model_config = ConfigDict(extra="forbid")

    kind: str = Field(
        min_length=1,
        description="section kind; known values: exam_name / findings / impression",
    )
    text: str = Field(min_length=1, description="section text as printed (OCR lines joined by newline)")
    ocr_line_numbers: list[int] = Field(
        min_length=1,
        description="indices of the OCR lines composing this section "
        "(0-based, same as txts/boxes; ordered like text)",
    )


class Report(BaseModel):
    """系统对一张图像处理后产出的结构化报告(report_id 等由 Go 落库时赋予)。"""

    model_config = ConfigDict(extra="forbid")

    report_type: str = Field(min_length=1)
    report_date: str | None = Field(default=None, pattern=r"^\d{4}-\d{2}-\d{2}$")
    status: Status
    items: list[ReportItem]
    sections: list[ReportSection] | None = Field(
        default=None,
        min_length=1,
        description="optional report-level sections for narrative reports; omitted for non-narrative reports",
    )

    @model_validator(mode="after")
    def _empty_items_must_be_failed_or_sectioned(self) -> "Report":
        # 不变量(票 03/04):零指标项只允许 failed,或叙述型报告携带 sections(影像单
        # 以段落为主体);「成功/待核对且零指标项且无段落」仍连构造都不可能。
        if not self.items and not self.sections and self.status is not Status.FAILED:
            raise ValueError(
                "report with no items and no sections must have status=failed (empty-items invariant)"
            )
        return self

    @model_serializer(mode="wrap")
    def _omit_empty_sections(self, handler):
        # sections 是可选集合:非叙述型报告不输出该键(而非 "sections": null),
        # 这样既有 report golden / JSON 消费方保持逐字节兼容(ADR-0008)。
        data = handler(self)
        if data.get("sections") is None:
            data.pop("sections", None)
        return data


class ReportResponse(BaseModel):
    """POST /report 的响应体:一次请求内 OCR 结果 + 结构化报告(实施计划 4.3;与 ReparseRequest 成对)。"""

    model_config = ConfigDict(extra="forbid")

    ocr_result: OCRResult
    report: Report


class ReparseRequest(BaseModel):
    """POST /reparse 的请求体:复用已存 OCR 结果重跑后处理(date 为人工补录兜底)。"""

    model_config = ConfigDict(extra="forbid")

    ocr_result: OCRResult
    date: str | None = Field(default=None, pattern=r"^\d{4}-\d{2}-\d{2}$")
