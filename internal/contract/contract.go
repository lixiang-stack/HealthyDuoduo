// Package contract 定义 Go 侧的 JSON 契约结构(镜像 schemas/*.schema.json)。
// pydantic(py)与 Go struct 双语各一份,以同一份 schema 为源、互证防漂移(ADR-0002 / NF-06)。
package contract

import "strings"

// Point 是文本框多边形顶点坐标 [x, y]。
type Point [2]float64

// Box 是一个文本行的四点多边形。
type Box [4]Point

// 报告状态 4.2 枚举(schemas/report.schema.json);字符串契约,比较用常量不裸写字面值。
const (
	StatusSuccess = "success"
	StatusPartial = "partial"
	StatusFailed  = "failed"
)

// 指标项 flag 枚举(schemas/report.schema.json 同源;存储 CHECK 同值)。
// high/low 只用于有方向的数值异常;abnormal 只用于无方向的定性异常。
const (
	FlagNormal    = "normal"
	FlagHigh      = "high"
	FlagLow       = "low"
	FlagAbnormal  = "abnormal"
	FlagUnknown   = "unknown"
	FlagNotTested = "not_tested"
)

// 指标项核对状态(ADR-0005):未核对 → 已确认 / 已修正;单向,无撤销回未核对。
const (
	ReviewStatusUnreviewed = "unreviewed"
	ReviewStatusConfirmed  = "confirmed"
	ReviewStatusCorrected  = "corrected"
)

// 单位核验状态(OpenAPI unit_status):verified 才进入趋势连接与分析事实层。
const (
	UnitStatusVerified   = "verified"
	UnitStatusUnverified = "unverified"
)

// 分析物语义 key:产品内部词汇,不是 LOINC / WS/T 886 码。
// 每个 key 必须对应识别端词典中唯一一个规范指标(以 code / LOINC 锚定),
// 且只给框架层当前消费的分析物加 key;名称、别名、单位、方法差异留在识别端词典。
const (
	AnalyteALT        = "alt"
	AnalyteAST        = "ast"
	AnalyteALP        = "alp"
	AnalyteHemoglobin = "hemoglobin"
	AnalyteMCV        = "mcv"
	AnalyteMCH        = "mch"
	AnalyteMCHC       = "mchc"
	AnalyteRDWCV      = "rdw_cv"
)

// 叙述型报告类别:影像(超声)与病历(llm-analysis 票 03);主体是报告段落,零指标项。
const (
	ReportTypeImaging      = "影像"
	ReportTypeClinicalNote = "clinical_note"
)

// 报告段落类别(schemas/report.schema.json reportSection.kind)。
// exam_name/findings/impression 属影像;其余属病历,note 是切不出标题时的整页兜底。
const (
	SectionKindExamName       = "exam_name"
	SectionKindFindings       = "findings"
	SectionKindImpression     = "impression"
	SectionKindChiefComplaint = "chief_complaint"
	SectionKindPresentIllness = "present_illness"
	SectionKindDiagnosis      = "diagnosis"
	SectionKindTreatment      = "treatment"
	SectionKindNote           = "note"
)

// IsNarrativeReportType 该报告类别是否以段落为主体(零指标项合法;status 由正文派生)。
func IsNarrativeReportType(reportType string) bool {
	return reportType == ReportTypeImaging || reportType == ReportTypeClinicalNote
}

// TrendKey 趋势连接键:标准 code 优先,缺失用 LOINC,再回退词典规范名。
// 同一分析物跨报告 / 跨面板以该键连接;面板、事实层与存储共用同一口径。
func TrendKey(code, loinc, name string) string {
	if key := strings.TrimSpace(code); key != "" {
		return key
	}
	if key := strings.TrimSpace(loinc); key != "" {
		return key
	}
	return name
}

// ModelInfo 描述 OCR 使用的模型。
type ModelInfo struct {
	Det string `json:"det"`
	Cls string `json:"cls"`
	Rec string `json:"rec"`
}

// OCRResult 对应 schemas/ocr_result.schema.json(实施计划 4.1)。
type OCRResult struct {
	Txts       []string  `json:"txts"`
	Boxes      []Box     `json:"boxes"`
	Scores     []float64 `json:"scores"`
	Elapse     float64   `json:"elapse"`
	ElapseList []float64 `json:"elapse_list"`
	Engine     string    `json:"engine"`
	ModelInfo  ModelInfo `json:"model_info"`
}

// ReportItem 是报告中的单个指标项。
// Value 为印刷原样数值字符串(如 '128'、'0.43'、'<2.00');低于检出限等定性结果可直接
// 承载;需要数值消费侧(如 trend delta)自行 ParseFloat(解析失败不计 delta)。
type ReportItem struct {
	Name string `json:"name"`
	// Panel 是项目组 code(spec §5);必须属于 report_type 的允许集合,other 为跨类别兜底。
	Panel    string  `json:"panel"`
	Value    *string `json:"value"`
	Unit     *string `json:"unit"`
	RefRange *string `json:"ref_range"`
	// Method 是检测方法/实验方法列原样(如 '化学发光法');无该列或解析失败为 null。
	Method *string `json:"method"`
	// 标准层字段:本期只打通契约/存储/API/App 透传,识别端产出由票 03+ 逐 panel 启用;
	// nil 时省略(旧 fixture / 未迁移报告逐字节兼容)。
	Code          *string `json:"code,omitempty"`
	Loinc         *string `json:"loinc,omitempty"`
	Analyte       *string `json:"analyte,omitempty"`
	UnitDisplay   *string `json:"unit_display,omitempty"`
	UnitStatus    *string `json:"unit_status,omitempty"`
	RawValue      *string `json:"raw_value,omitempty"`
	RawUnit       *string `json:"raw_unit,omitempty"`
	RawRefRange   *string `json:"raw_ref_range,omitempty"`
	RefKind       *string `json:"ref_kind,omitempty"`
	ResultKind    *string `json:"result_kind,omitempty"`
	ResultBasis   *string `json:"result_basis,omitempty"`
	Flag          string  `json:"flag"`
	FlagReason    *string `json:"flag_reason,omitempty"`
	LowConfidence bool    `json:"low_confidence"`
	RawText       string  `json:"raw_text"`
	// OCRLineNumbers 该指标项组成 OCR 行的下标(0-based,与 ocr_result 的 txts/boxes 同下标;
	// 顺序与 raw_text 组成顺序一致)。识别端直接给出,详情/核对的行链接不再做文本反推。
	OCRLineNumbers []int32 `json:"ocr_line_numbers"`
}

// ReportSection 是报告级叙述段落(仅叙述型报告,如影像单 / 病历;只读展示,不参与指标项核对)。
// Kind:影像 exam_name / findings / impression;病历 chief_complaint / present_illness / diagnosis / treatment / note。数组顺序即展示顺序。
type ReportSection struct {
	Kind           string  `json:"kind"`
	Text           string  `json:"text"`
	OCRLineNumbers []int32 `json:"ocr_line_numbers"`
}

// Report 对应 schemas/report.schema.json(实施计划 4.2)。
// report_date 为 YYYY-MM-DD 字符串或 null;用 string 而非 time.Time 是因为
// encoding/json 对纯日期字符串无法直接反序列化为 time.Time(契约值为原样日期,不承担时区语义)。
// Sections 可选:非叙述型报告不输出该键,既有 JSON 消费方保持兼容(ADR-0008)。
type Report struct {
	ReportType string  `json:"report_type"`
	ReportDate *string `json:"report_date"`
	Status     string  `json:"status"`
	// Panels 是报告级项目组,有序:命中项数降序、同数按固定优先级;Panels[0] 为卡片主标签。
	Panels   []string        `json:"panels"`
	Items    []ReportItem    `json:"items"`
	Sections []ReportSection `json:"sections,omitempty"`
}
