// Package pipeline 是 hdd 的编排层(用例):一图进 → 三层留存(对象存储/OCR/报告)。
// 依赖方向:只依赖本包接口;识别服务 / PG / S3 由 cmd/hdd 注入适配器实现(整洁架构)。
// 本文件只放接口与业务视图类型;ingest 编排见 ingest.go,查询/重算见 reports.go。
package pipeline

import (
	"context"
	"time"

	"healthyduoduo/internal/contract"
)

// 对外身份 = 图像内容 sha256:images 主键、reports 主键(每图一行)以及
// hdd ingest/show/reparse 的入参与输出都用它(验收轮修订,实施计划 4.4)。
// ocr_results.id 是识别历史追加行的内部序号(NF-05);reports.ocr_result_id 指向其生成所用行。

// Recognizer 识别服务边界(POST /report、/reparse,实施计划 4.3)。
type Recognizer interface {
	// Report 上传图像(+可选 date 补录)→ {ocr_result, report}。
	// normalizedImage 非 nil 表示识别端已把整页横拍图像转正;调用方应以它作为
	// 入库/展示原图与内容身份,ocr/report 也已基于该图像生成。
	Report(ctx context.Context, image []byte, filename, date string) (normalizedImage []byte, ocr *contract.OCRResult, report *contract.Report, err error)
	// Reparse 已存 OCR 结果(+可选 date 补录)→ report(规则迭代主路径)。
	Reparse(ctx context.Context, ocr *contract.OCRResult, date string) (*contract.Report, error)
}

// ObjectStore 对象存储边界(bucket raw,键=图像内容 sha256;即对象键=身份,单字段)。
type ObjectStore interface {
	Put(ctx context.Context, key string, data []byte) error
	// Get 取对象字节(serve 图像直出,票 07);键不存在返回 nil, nil(与 ImageBySha256 同口径)。
	Get(ctx context.Context, key string) ([]byte, error)
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
	// failReason 失败原因由 pipeline 产出(viz-and-client 票 03 空指标项不变量),
	// nil = 非 failed 报告;随 upsert 全量写,重算复原时旧原因一并清除。
	WriteReport(ctx context.Context, reportSha string, ocrID int64, report contract.Report, failReason *string) error
	// TrendByName 同一指标项名的跨报告时序点(升序;reportType 空串 = 不过滤类别)。
	TrendByName(ctx context.Context, itemName, reportType string) ([]TrendRow, error)
}

// MemberStore 成员持久化边界(serve 成员端点;viz-and-client 票 05)。
// 列表按创建序返回,孩子优先由客户端做(契约排序口径);无删除操作——契约不提供。
type MemberStore interface {
	// CreateMember 建成员(名 + 是否孩子两档;id/created_at 由存储产生)。
	CreateMember(ctx context.Context, name string, isChild bool) (*Member, error)
	// ListMembers 全部成员(创建序:id 升序;BIGSERIAL 不重用,id 序即创建序)。
	ListMembers(ctx context.Context) ([]Member, error)
	// UpdateMember 改名/改是否孩子(至少一项非 nil);返回 nil = 成员不存在。
	UpdateMember(ctx context.Context, id int64, name *string, isChild *bool) (*Member, error)
	// MemberByID 按 id 取成员(上传归属校验;缺失返回 nil,nil,与 ImageBySha256 同口径)。
	MemberByID(ctx context.Context, id int64) (*Member, error)
}

// ReportStore 报告域持久化边界(serve 上传/时间线;viz-and-client 票 06)。
// 与 MemberStore 同惯例:独立窄接口,serve 经 deps.reports 注入同一 *storage.Store,
// CLI 侧既有 fake 不承担这些方法(ISP)。
type ReportStore interface {
	// ListTimeline 时间线摘要(成员过滤:nil = 全部,含存量无主报告;服务端排序
	// 日期降序、dateless 沉底,客户端展示仍会再排序一次)。
	ListTimeline(ctx context.Context, memberID *int64) ([]ReportSummary, error)
	// SummaryByImage 单份报告摘要(缺失返回 nil,nil;上传响应回读用)。
	SummaryByImage(ctx context.Context, imageSha string) (*ReportSummary, error)
	// ItemsWithReviewStatus 指标项含外部句柄与核对状态(上传响应回读;按 id 升序)。
	ItemsWithReviewStatus(ctx context.Context, reportSha string) ([]ReportItemRow, error)
	// OCRResultJSONByReport 报告所用 OCR 结果的原始 JSON(reports.ocr_result_id →
	// ocr_results.engine_output;详情行链接回读,票 07)。缺失返回 nil, nil。
	OCRResultJSONByReport(ctx context.Context, reportSha string) ([]byte, error)
	// SectionsByImage 报告级叙述段落(0006/ADR-0008;非叙述型报告/缺失返回 nil, nil)。
	SectionsByImage(ctx context.Context, reportSha string) ([]contract.ReportSection, error)
	// SetReportMember 只改报告归属(上传新建/无主归档时赋值;改归属端点同用;不碰其他字段)。
	// 报告不存在返回 false, nil(与 DeleteReport 同一缺失口径,写入路径自带存在性判定)。
	SetReportMember(ctx context.Context, reportSha string, memberID int64) (bool, error)
	// DeleteReport 硬删报告行(级联指标项与修正对;viz-and-client 票 10):
	// 原始图像与 OCR 识别历史留痕保留,同图日后可重新上传入档。
	// 报告不存在返回 false, nil(与 SummaryByImage 的缺失口径同惯例)。
	DeleteReport(ctx context.Context, reportSha string) (bool, error)
	// ReviewItem 核对落地(确认/修正;viz-and-client 票 09):单事务内应用动作
	// (生效值 + flag 按修正后 value × ref_range 重算 + 每次修正 append 一条修正对)
	// 并派生报告状态(partial → success 当且仅当未核对低置信项归零,单向)。
	// 返回动作后的报告级状态、未核对低置信计数与本次指标项;
	// 报告或指标项不存在返回 nil, nil。
	ReviewItem(ctx context.Context, reportSha string, itemID int64, action ReviewAction) (*ReviewOutcome, error)
}

// TrendStore 趋势域持久化边界(serve 趋势端点;票 02)。
// 与 CLI 的 Store.TrendByName 是不同消费面:成员维度 + 可选 unit + ref/method/review
// 字段;独立窄接口,CLI fakes 不承担这些方法(ISP)。
type TrendStore interface {
	// TrendPoints 成员下同一指标项名称的时序点(unit 空 = 全部单位;日期升序、
	// dateless 沉底、同日期按报告 sha 与指标项 id 升序)。
	TrendPoints(ctx context.Context, memberID int64, name, unit string) ([]TrendPointRow, error)
}

// ReviewStatusUnreviewed 指标项核对状态机(ADR-0005)的初始态;任一指标项
// review_status ≠ 本值即视为报告已有核对决策(同图重传 409 的判定依据)。
const ReviewStatusUnreviewed = "unreviewed"

// FlagCounts 「异常」计数徽标口径 = 偏高/偏低/异常/可操作未知四档;normal 不计(详情页逐项染色)。
type FlagCounts struct {
	High     int
	Low      int
	Abnormal int
	Unknown  int
}

// ReportSummary 时间线卡片/上传响应头部的业务视图(serve 报告域;票 06)。
// 计数字段为响应派生(不落列,ADR-0005):FlagCounts 按指标项当前 flag 聚合;
// NotTestedCount 为未检测项数(信息徽标,不进异常/待处理);
// UnreviewedLowConfidenceCount 统计未核对低置信项(success 报告也如实计数;
// failed 无项自然为 0;not_tested 不计入)。
type ReportSummary struct {
	Sha256                       string
	MemberID                     *int64
	ReportType                   string
	ReportDate                   *string
	Status                       string
	FailReason                   *string
	FlagCounts                   FlagCounts
	NotTestedCount               int
	UnreviewedLowConfidenceCount int
}

// ReportItemRow 指标项含 HTTP 外部句柄与核对状态(嵌入识别端形状的八个字段)。
type ReportItemRow struct {
	ItemID       int64
	ReviewStatus string
	contract.ReportItem
}

// Member 成员最小模型的业务视图:显示名 + 是否孩子两档,仅此;无生日/头像,
// 不支持删除(可改名)。CreatedAt 为库中创建时刻,HTTP 侧序列化为 RFC3339。
type Member struct {
	MemberID  int64
	Name      string
	IsChild   bool
	CreatedAt time.Time
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
// Value 为印刷原样字符串(与 ReportItem.value 同契约);delta 需 ParseFloat。
type TrendRow struct {
	Sha256        string
	ReportType    string
	ReportDate    *string
	Value         *string
	Unit          *string
	Flag          string // normal | high | low | abnormal | unknown(4.2)
	LowConfidence bool   // 任一组成行 score < LOW_SCORE_THRESHOLD(决策 #7 透传)
}

// TrendPointRow 成员维度趋势点(serve TrendPoint 的存储侧投影;票 02)。
// ReportDate 缺失时 nil;ReviewStatus 由 handler 派生 reviewed 标记。
type TrendPointRow struct {
	ReportID      string
	ReportDate    *string
	Value         *string
	Unit          *string
	RefRange      *string
	Method        *string
	Flag          string
	LowConfidence bool
	ReviewStatus  string
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
