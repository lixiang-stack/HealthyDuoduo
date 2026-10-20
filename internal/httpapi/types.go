// Package httpapi 是 HTTP 契约(schemas/openapi.yaml)的 Go 侧镜像。
// 依赖规则:仅镜像契约形状,不含业务逻辑与 HTTP 实现——serve 的 handler(票 05+)由
// 该镜像序列化响应,对账测试(internal/httpapi 的互证测试)锁死与 OpenAPI 的漂移。
package httpapi

// Member components/Member:成员最小模型 = 显示名 + 是否孩子(孩子/成人两档),仅此二字段。
type Member struct {
	MemberID  int64  `json:"member_id"`
	Name      string `json:"name"`
	IsChild   bool   `json:"is_child"`
	CreatedAt string `json:"created_at"` // RFC3339 date-time
}

// MemberCreate POST /members 请求体:member_id/created_at 由服务端产生,不收。
type MemberCreate struct {
	Name    string `json:"name"`
	IsChild bool   `json:"is_child"`
}

// MemberUpdate PATCH /members/{member_id} 请求体:至少一项;不提供成员删除。
type MemberUpdate struct {
	Name    *string `json:"name,omitempty"`
	IsChild *bool   `json:"is_child,omitempty"`
}

// MembersList GET /members 响应信封;排序(孩子优先、其余按创建序)由客户端做。
type MembersList struct {
	Members []Member `json:"members"`
}

// ReportsList GET /reports 响应信封(时间线卡片;排序客户端做:dateless 沉底不受排序破坏)。
type ReportsList struct {
	Reports []ReportSummary `json:"reports"`
}

// FlagCounts components/FlagCounts:「异常」计数徽标口径 = 偏高 / 偏低 / 未知三档
// 非负整数;normal 不计(详情页逐项染色)。
type FlagCounts struct {
	High    int `json:"high"`
	Low     int `json:"low"`
	Unknown int `json:"unknown"`
}

// ReportHeader 报告级公共头部(ReportSummary 与 ReportDetail 共用投影)。
// member_id 可空:存量 CLI 时代报告无主;fail_reason 由 pipeline 产出(票 03/06 填充)。
type ReportHeader struct {
	ReportID   string     `json:"report_id"` // 图像内容 sha256(对外身份,64 hex)
	MemberID   *int64     `json:"member_id"`
	ReportType string     `json:"report_type"`
	ReportDate *string    `json:"report_date"` // YYYY-MM-DD 或 null
	Status     string     `json:"status"`
	FailReason *string    `json:"fail_reason"`
	FlagCounts FlagCounts `json:"flag_counts"`
	// UnreviewedLowConfidenceCount 响应派生值(不落列,ADR-0005):只统计 partial 报告中
	// review_status=unreviewed 且 low_confidence 的指标项;failed 不计入。
	UnreviewedLowConfidenceCount int `json:"unreviewed_low_confidence_count"`
}

// ReportSummary GET /reports 时间线卡片(无 items 主体)。
type ReportSummary struct {
	ReportHeader
}

// OcrLineLink 详情指标项 ↔ OCR 行链接:line 为 OCR 行下标(0-based,与 ocr_result
// 的 boxes/txts 下标一致),box 为该行四点多边形顶点坐标(识别端 Box 同源)。
type OcrLineLink struct {
	Line int    `json:"line"`
	Box  Box4x2 `json:"box"`
}

// Box4x2 四点多边形([[x,y]×4];形状与 internal/contract.Box 一致,此处不引用
// 识别端识别包,保持 HTTP 契约包零业务依赖)。
type Box4x2 [4][2]float64

// PrevValue B2-lite「较上次」Δ 上一值:同成员 + 同规范名 + 同归一单位、
// report_date 严格早于当前报告的最近一份(与上传先后无关);任一侧无日期
// 或仅有同日报告 → null(宁缺毋错)。
type PrevValue struct {
	ReportDate string  `json:"report_date"` // 上一值必带日期(基线定义)
	Value      string  `json:"value"`       // 印刷原样数值
	Unit       *string `json:"unit"`
}

// ReportItem GET /reports/{report_id} 详情中的单个指标项。
// name/value/unit/ref_range/method/flag/low_confidence/raw_text 与识别端
// report.schema.json 的 reportItem 同源(互证测试锁死逐字段子树相等);
// 后四项为 HTTP 侧扩展。
type ReportItem struct {
	ItemID        int64         `json:"item_id"` // BIGSERIAL 不透明句柄:重跑识别后全换,非持久主键
	Name          string        `json:"name"`
	Value         *string       `json:"value"`
	Unit          *string       `json:"unit"`
	RefRange      *string       `json:"ref_range"`
	Method        *string       `json:"method"`
	Flag          string        `json:"flag"`
	LowConfidence bool          `json:"low_confidence"`
	RawText       string        `json:"raw_text"`
	ReviewStatus  string        `json:"review_status"`
	OcrLines      []OcrLineLink `json:"ocr_lines"`
	Prev          *PrevValue    `json:"prev"`
}

// ReportSection 报告级叙述段落(叙述型报告;与识别端 reportSection 同源,
// 行号在 HTTP 侧映射为 OcrLineLink)。数组顺序即展示顺序;非叙述型报告不输出 sections。
type ReportSection struct {
	Kind     string        `json:"kind"`
	Text     string        `json:"text"`
	OcrLines []OcrLineLink `json:"ocr_lines"`
}

// ReportDetail GET /reports/{report_id} 详情响应(含全量指标项与可选报告段落)。
type ReportDetail struct {
	ReportHeader
	Sections []ReportSection `json:"sections,omitempty"`
	Items    []ReportItem    `json:"items"`
}

// UploadResult POST /reports 响应(200 幂等命中 / 201 新建或显式覆盖重跑):
// duplicate=true 表示 sha 命中已有无核对决策报告(重试安全);report 同步返回完整报告。
type UploadResult struct {
	Duplicate bool         `json:"duplicate"`
	Report    ReportDetail `json:"report"`
}

// ReviewRequest PATCH /reports/{report_id}/items/{item_id} 请求体(契约化字段集):
// 只收 value/unit/name/ref_range 四个纸面事实;flag 由服务端按修正后的
// value × ref_range 重算,契约不提供手改入口(additionalProperties:false 拒收 flag)。
// confirm 分支不得携带修正字段(omitted 非 null);correct 分支至少携带一项。
type ReviewRequest struct {
	Action   string  `json:"action"` // confirm | correct
	Value    *string `json:"value,omitempty"`
	Unit     *string `json:"unit,omitempty"`
	Name     *string `json:"name,omitempty"`
	RefRange *string `json:"ref_range,omitempty"`
}

// ReviewResult PATCH 核对响应:报告级状态随同返回(单响应刷新整屏,无"提交作业"步骤)。
type ReviewResult struct {
	ReportID                     string     `json:"report_id"`
	Status                       string     `json:"status"`
	UnreviewedLowConfidenceCount int        `json:"unreviewed_low_confidence_count"`
	Item                         ReportItem `json:"item"`
}

// CorrectionValues 修正任一侧快照:四项纸面事实全集(null = 该项无值/未带)。
type CorrectionValues struct {
	Name     *string `json:"name"`
	Value    *string `json:"value"`
	Unit     *string `json:"unit"`
	RefRange *string `json:"ref_range"`
}

// CorrectionPair components/CorrectionPair:修正对 append-only 形状
// (before→after 全集 + 原始行文 + OCR 行号 + 时间;本期只存不回流,ADR-0005,
// 形状按可回流设计供未来直接喂 gt_eval)。确认不产生修正对记录。
type CorrectionPair struct {
	ItemID         int64            `json:"item_id"`
	Before         CorrectionValues `json:"before"`
	After          CorrectionValues `json:"after"`
	RawText        string           `json:"raw_text"`
	OcrLineNumbers []int            `json:"ocr_line_numbers"`
	CreatedAt      string           `json:"created_at"`
}

// TrendPoint GET /members/{member_id}/indicators/{name}/trend 单点。
// Reviewed 为响应派生信任标记(review_status ∈ confirmed|corrected):家长只在
// 未核对低置信点空心展示;实心 = reviewed 或非低置信(点不隐藏、不冒充可信)。
type TrendPoint struct {
	ReportID      string  `json:"report_id"` // 据此回链报告详情
	ReportDate    *string `json:"report_date"`
	Value         *string `json:"value"`
	Unit          *string `json:"unit"`
	RefRange      *string `json:"ref_range"`
	Method        *string `json:"method"`
	Flag          string  `json:"flag"`
	LowConfidence bool    `json:"low_confidence"`
	Reviewed      bool    `json:"reviewed"`
}

// TrendResult B2 趋势时序(点按 report_date 升序,dateless 点最后,不参与连线)。
// unit 门控:?unit= 指定归一单位 → 只返回同单位点;缺省返回全部单位族的点
// (跨单位族不连线不提示,客户端按 unit 分段;无运行时换算)。
type TrendResult struct {
	MemberID int64        `json:"member_id"`
	Name     string       `json:"name"`
	Points   []TrendPoint `json:"points"`
}

// Error components/Error:非 2xx 响应统一形状(message 中性事实侧文案)。
type Error struct {
	Message string `json:"message"`
}
