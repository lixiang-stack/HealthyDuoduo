// Package contract 定义 Go 侧的 JSON 契约结构(镜像 schemas/*.schema.json)。
// pydantic(py)与 Go struct 双语各一份,以同一份 schema 为源、互证防漂移(ADR-0002 / NF-06)。
package contract

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
	Name     string  `json:"name"`
	Value    *string `json:"value"`
	Unit     *string `json:"unit"`
	RefRange *string `json:"ref_range"`
	// Method 是检测方法/实验方法列原样(如 '化学发光法');无该列或解析失败为 null。
	Method        *string `json:"method"`
	Flag          string  `json:"flag"`
	LowConfidence bool    `json:"low_confidence"`
	RawText       string  `json:"raw_text"`
	// OCRLineNumbers 该指标项组成 OCR 行的下标(0-based,与 ocr_result 的 txts/boxes 同下标;
	// 顺序与 raw_text 组成顺序一致)。识别端直接给出,详情/核对的行链接不再做文本反推。
	OCRLineNumbers []int32 `json:"ocr_line_numbers"`
}

// Report 对应 schemas/report.schema.json(实施计划 4.2)。
// report_date 为 YYYY-MM-DD 字符串或 null;用 string 而非 time.Time 是因为
// encoding/json 对纯日期字符串无法直接反序列化为 time.Time(契约值为原样日期,不承担时区语义)。
type Report struct {
	ReportType string       `json:"report_type"`
	ReportDate *string      `json:"report_date"`
	Status     string       `json:"status"`
	Items      []ReportItem `json:"items"`
	// 报告级叙述段落(影像单;非影像类别六字段恒 null)。文本与行号成对:
	// 文本非 null ⇒ 行号非 null 且非空;文本 null ⇒ 行号 null。nil slice 序列化为 null。
	ExamName                 *string `json:"exam_name"`
	ExamNameOCRLineNumbers   []int32 `json:"exam_name_ocr_line_numbers"`
	Findings                 *string `json:"findings"`
	FindingsOCRLineNumbers   []int32 `json:"findings_ocr_line_numbers"`
	Impression               *string `json:"impression"`
	ImpressionOCRLineNumbers []int32 `json:"impression_ocr_line_numbers"`
}
