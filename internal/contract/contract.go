// Package contract 定义 Go 侧的 JSON 契约结构(镜像 schemas/*.schema.json)。
// pydantic(py)与 Go struct 双语各一份,以同一份 schema 为源、互证防漂移(ADR-0002 / NF-06)。
package contract

// Point 是文本框多边形顶点坐标 [x, y]。
type Point [2]float64

// Box 是一个文本行的四点多边形。
type Box [4]Point

// ModelInfo 描述 OCR 使用的模型。
type ModelInfo struct {
	Det string `json:"det"`
	Cls string `json:"cls"`
	Rec string `json:"rec"`
}

// TableStructure 是表结构识别结果(ADR-0004):TSR 模型还原的单元格网格(HTML 表)。
// 随 OCR 结果一同落库,/reparse 复用它免于再次读取原图;可选字段。
type TableStructure struct {
	HTML   string  `json:"html"`
	Model  string  `json:"model"` // wired | lineless
	Elapse float64 `json:"elapse"`
}

// OCRResult 对应 schemas/ocr_result.schema.json(实施计划 4.1)。
type OCRResult struct {
	Txts           []string        `json:"txts"`
	Boxes          []Box           `json:"boxes"`
	Scores         []float64       `json:"scores"`
	Elapse         float64         `json:"elapse"`
	ElapseList     []float64       `json:"elapse_list"`
	Engine         string          `json:"engine"`
	ModelInfo      ModelInfo       `json:"model_info"`
	TableStructure *TableStructure `json:"table_structure,omitempty"`
}

// ReportItem 是报告中的单个指标项。
type ReportItem struct {
	Name          string   `json:"name"`
	Value         *float64 `json:"value"`
	Unit          *string  `json:"unit"`
	RefRange      *string  `json:"ref_range"`
	Flag          string   `json:"flag"`
	LowConfidence bool     `json:"low_confidence"`
	RawText       string   `json:"raw_text"`
}

// Report 对应 schemas/report.schema.json(实施计划 4.2)。
// report_date 为 YYYY-MM-DD 字符串或 null;用 string 而非 time.Time 是因为
// encoding/json 对纯日期字符串无法直接反序列化为 time.Time(契约值为原样日期,不承担时区语义)。
type Report struct {
	ReportType string       `json:"report_type"`
	ReportDate *string      `json:"report_date"`
	Status     string       `json:"status"`
	Items      []ReportItem `json:"items"`
}
