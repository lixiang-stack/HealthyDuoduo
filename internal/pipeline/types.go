// Package pipeline 是 hdd 的编排层(用例):一图进 → 三层留存(对象存储/OCR/报告)。
// 依赖方向:只依赖本包接口;识别服务 / PG / S3 由 cmd/hdd 注入适配器实现(整洁架构)。
// 本文件只放接口与业务视图类型;ingest 编排见 ingest.go,查询/重算见 reports.go。
package pipeline

import (
	"context"

	"healthyduoduo/internal/contract"
)

// 对外身份 = 图像内容 sha256:images 主键、reports 主键(每图一行)以及
// hdd ingest/show/reparse 的入参与输出都用它(验收轮修订,实施计划 4.4)。
// ocr_results.id 是识别历史追加行的内部序号(NF-05);reports.ocr_result_id 指向其生成所用行。

// Recognizer 识别服务边界(POST /report、/reparse,实施计划 4.3)。
type Recognizer interface {
	// Report 上传图像(+可选 date 补录、可选 preprocess 预处理开关)→ {ocr_result, report}。
	Report(ctx context.Context, image []byte, filename, date, preprocess string) (*contract.OCRResult, *contract.Report, error)
	// Reparse 已存 OCR 结果(+可选 date 补录)→ report(规则迭代主路径)。
	Reparse(ctx context.Context, ocr *contract.OCRResult, date string) (*contract.Report, error)
}

// ObjectStore 对象存储边界(bucket raw,键=图像内容 sha256;即对象键=身份,单字段)。
type ObjectStore interface {
	Put(ctx context.Context, key string, data []byte) error
}

// Store 持久化边界(PG 四表);items 由实现随报告行事务式全量替换。
type Store interface {
	// ImageBySha256 精确返回图像行(完整 64 hex;缺失返回 nil,nil)。
	ImageBySha256(ctx context.Context, sha string) (*Image, error)
	// CreateImage 幂等插入图像行(主键冲突即已存在,忽略)。
	CreateImage(ctx context.Context, sha, filename string) error
	// AppendOCRResult 追加一行识别历史返回其内部序号(NF-05:历史保留)。
	AppendOCRResult(ctx context.Context, imageSha string, raw []byte) (ocrID int64, err error)
	// 既有报告行查询(存在=每图一行)。
	ReportByImage(ctx context.Context, imageSha string) (*ReportRow, bool, error)
	ListReports(ctx context.Context, reportType string, date string) ([]ReportRow, error)
	ItemsByImage(ctx context.Context, reportSha string) ([]contract.ReportItem, error)
	OCRResultJSON(ctx context.Context, ocrID int64) ([]byte, error)
	// WriteReport 落/更报告(身份=reportSha;items 取自 report.Items)。
	WriteReport(ctx context.Context, reportSha string, ocrID int64, report contract.Report) error
	// TrendByName 同一指标项名的跨报告时序点(升序;reportType 空串 = 不过滤类别)。
	TrendByName(ctx context.Context, itemName, reportType string) ([]TrendRow, error)
}

// Image images 表一行的业务视图(identity=内容 sha256)。
type Image struct {
	Sha256           string
	OriginalFilename string
}

// ReportRow 报告查询视图(report_date 为 YYYY-MM-DD,缺失时 nil)。
type ReportRow struct {
	Sha256      string
	OCRResultID int64
	ReportType  string
	ReportDate  *string
	Status      string // success | partial | failed(4.2)
	ItemsCount  int64  // 仅 ListReports 填充
	Items       []contract.ReportItem
}

// TrendRow 单指标项跨报告的时序数据点(hdd trend 查询视图;按报告日期升序)。
type TrendRow struct {
	Sha256     string
	ReportType string
	ReportDate *string
	Value      *float64
	Unit       *string
	Flag       string // normal | high | low | unknown(4.2)
}

// IngestOutcome 入库结果(供 CLI 演示输出)。
type IngestOutcome struct {
	Sha256 string
	Kind   IngestKind
	Report *contract.Report
}

// IngestKind 标记本次 ingest 的路径。
type IngestKind int

const (
	KindNew IngestKind = iota
	KindCached
	KindForced
	KindBackfilled
)
