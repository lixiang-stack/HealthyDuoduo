"""JSON 契约的 pydantic 建模,镜像 schemas/*.schema.json(NF-06 双语契约)。

本模块是识别服务的契约源:OCR 结果与报告结构均以此为准;
schema/ 下 JSON 文件是双语共用的源头,本文件与其保持字段一一对应。
"""

from collections import Counter
from collections.abc import Iterable
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
    unknown 只表示本应能比较但失败/暂不支持;not_tested 表示本次检查未做该项
    (纸面结果与参考区间均为裸短横 "-"),不是正常、也不是未知。
    """
    NORMAL = "normal"
    HIGH = "high"
    LOW = "low"
    ABNORMAL = "abnormal"
    UNKNOWN = "unknown"
    NOT_TESTED = "not_tested"


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

class Panel(_Vocabulary):
    """项目组(panel):指标项所属的检查项目组(spec §5)。

    每个指标项恰好属于一个 panel;报告级 panels 是指标项 panel 的排序聚合。
    """
    CBC = "cbc"
    URINALYSIS = "urinalysis"
    LIVER = "liver"
    KIDNEY = "kidney"
    LIPID = "lipid"
    GLUCOSE = "glucose"
    THYROID = "thyroid"
    SEX_HORMONE = "sex_hormone"
    URIC_ACID = "uric_acid"
    ELECTROLYTE = "electrolyte"
    VITANEMIA = "vitanemia"
    INFECTION = "infection"
    AUTOIMMUNE = "autoimmune"
    COAG = "coag"
    IMAGING = "imaging"
    OTHER = "other"


# 报告级 panels 的固定排序:命中项数降序;同数按此序,数值小优先(spec §5.3)。
PANEL_ORDER: tuple[Panel, ...] = (
    Panel.LIVER,
    Panel.KIDNEY,
    Panel.LIPID,
    Panel.GLUCOSE,
    Panel.URIC_ACID,
    Panel.ELECTROLYTE,
    Panel.VITANEMIA,
    Panel.THYROID,
    Panel.SEX_HORMONE,
    Panel.INFECTION,
    Panel.AUTOIMMUNE,
    Panel.COAG,
    Panel.CBC,
    Panel.URINALYSIS,
    Panel.IMAGING,
    Panel.OTHER,
)

# UI 标签(spec §5.2);识别端不展示,契约测试锁定每个 code 都有标签。
PANEL_LABELS: dict[Panel, str] = {
    Panel.CBC: "血常规",
    Panel.URINALYSIS: "尿常规",
    Panel.LIVER: "肝功",
    Panel.KIDNEY: "肾功",
    Panel.LIPID: "血脂",
    Panel.GLUCOSE: "血糖",
    Panel.THYROID: "甲状腺",
    Panel.SEX_HORMONE: "性激素",
    Panel.URIC_ACID: "尿酸",
    Panel.ELECTROLYTE: "电解质",
    Panel.VITANEMIA: "维生素/贫血",
    Panel.INFECTION: "感染筛查",
    Panel.AUTOIMMUNE: "自身抗体",
    Panel.COAG: "凝血",
    Panel.IMAGING: "影像",
    Panel.OTHER: "其他",
}

# 影像报告无指标项,报告级面板取该类别值(spec §5.1/§6.3)。
IMAGING_REPORT_TYPE = "影像"

# 指标项 panel 的父类约束(spec §5.1);other 是唯一跨类别兜底。
PANEL_ALLOWED_BY_REPORT_TYPE: dict[str, frozenset[Panel]] = {
    "临检": frozenset({Panel.CBC, Panel.URINALYSIS, Panel.COAG, Panel.OTHER}),
    "生化": frozenset({
        Panel.LIVER, Panel.KIDNEY, Panel.LIPID, Panel.GLUCOSE,
        Panel.URIC_ACID, Panel.ELECTROLYTE, Panel.VITANEMIA, Panel.OTHER,
    }),
    "激素": frozenset({Panel.THYROID, Panel.SEX_HORMONE, Panel.OTHER}),
    "免疫": frozenset({Panel.INFECTION, Panel.AUTOIMMUNE, Panel.OTHER}),
    IMAGING_REPORT_TYPE: frozenset({Panel.IMAGING, Panel.OTHER}),
    "其他专项": frozenset({Panel.OTHER}),
}


def allowed_panels(report_type: str) -> frozenset[Panel]:
    """该报告类别允许的指标项项目组;未知报告类别只允许 other。"""
    return PANEL_ALLOWED_BY_REPORT_TYPE.get(report_type, frozenset({Panel.OTHER}))


def aggregate_panels(panels: Iterable[Panel]) -> list[Panel]:
    """项目组去重排序:命中项数降序,同数按 PANEL_ORDER(spec §5.3)。"""
    counts = Counter(panels)
    return sorted(counts, key=lambda p: (-counts[p], PANEL_ORDER.index(p)))


def report_panels(
    report_type: str, status: Status, item_panels: Iterable[Panel]
) -> list[Panel]:
    """报告级项目组:失败 → other;有指标项 → 排序聚合;无指标项 → 影像 / other。

    postprocess 与 /classify 共用这一条规则,避免两套 fallback 漂移(spec §6.3/§6.4)。
    """
    panels = list(item_panels)
    if status is Status.FAILED:
        return [Panel.OTHER]
    if panels:
        return aggregate_panels(panels)
    if report_type == IMAGING_REPORT_TYPE:
        return [Panel.IMAGING]
    return [Panel.OTHER]


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
    panel: Panel = Field(
        description="项目组 code;必须属于 report_type 的允许集合(spec §5.1)"
    )
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
    panels: list[Panel] = Field(
        min_length=1,
        description="报告级项目组:命中项数降序、同数按固定优先级;panels[0] 为卡片主标签",
    )
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

    @model_validator(mode="after")
    def _validate_panels(self) -> "Report":
        # 父子约束是逐指标项的(spec §5.1);failed 只约束报告级 panels 为 other。
        allowed = allowed_panels(self.report_type)
        for item in self.items:
            if item.panel not in allowed:
                raise ValueError(
                    f"item {item.name!r} panel {item.panel.value!r} is not allowed "
                    f"for report_type {self.report_type!r}"
                )
        expected = report_panels(
            self.report_type, self.status, (item.panel for item in self.items)
        )
        if self.panels != expected:
            raise ValueError(
                f"panels {[p.value for p in self.panels]} do not match "
                f"item aggregate {[p.value for p in expected]}"
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
    """POST /report 的响应体:一次请求内 OCR 结果 + 结构化报告(实施计划 4.3;与 ReparseRequest 成对)。

    normalized_image 只在整页横拍/倒置被转正时出现(base64 编码的转正后图像);数据服务
    用它作为入库身份与展示原图。image_rotation 记录逆时针角度(0/90/180/270),便于排查。
    """

    model_config = ConfigDict(extra="forbid")

    ocr_result: OCRResult
    report: Report
    normalized_image: str | None = None
    image_rotation: int = 0


class ReparseRequest(BaseModel):
    """POST /reparse 的请求体:复用已存 OCR 结果重跑后处理(date 为人工补录兜底)。"""

    model_config = ConfigDict(extra="forbid")

    ocr_result: OCRResult
    date: str | None = Field(default=None, pattern=r"^\d{4}-\d{2}-\d{2}$")


class ClassifyItem(BaseModel):
    """POST /classify 请求里的一个指标项(当前指标名 + 单位)。"""

    model_config = ConfigDict(extra="forbid")

    name: str = Field(min_length=1)
    unit: str | None = None


class ClassifyRequest(BaseModel):
    """POST /classify 请求体:按 report_type + 当前指标名重算项目组。"""

    model_config = ConfigDict(extra="forbid")

    report_type: str = Field(min_length=1)
    items: list[ClassifyItem]


class ClassifyResponse(BaseModel):
    """POST /classify 响应:item_panels 与请求 items 等长同序。"""

    model_config = ConfigDict(extra="forbid")

    panels: list[Panel] = Field(min_length=1)
    item_panels: list[Panel]
