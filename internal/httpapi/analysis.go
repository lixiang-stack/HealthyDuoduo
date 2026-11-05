// 分析端点 HTTP 契约镜像(llm-analysis 票 04/05/06/07/09):请求入口 + 三层结果信封。
// 事实层 / 框架层 / 快照的嵌套形状直接复用 internal/analysis 的领域类型(外层 → 内层依赖),
// 避免契约镜像与领域模型两份定义漂移;OpenAPI 仍是唯一源,contract test 锁定形状。
package httpapi

import "healthyduoduo/internal/analysis"

// AnalysisRequest POST /analyses 请求体:member_id/panel/entry 必填;
// 单指标入口带 indicator_name,病历入口带 report_sha;window_months 缺省 12。
type AnalysisRequest struct {
	MemberID      int64   `json:"member_id"`
	Panel         string  `json:"panel"`
	Entry         string  `json:"entry"`
	IndicatorName *string `json:"indicator_name,omitempty"`
	ReportSHA     *string `json:"report_sha,omitempty"`
	WindowMonths  *int    `json:"window_months,omitempty"`
}

// AnalysisLayers 三层信封;事实层永远存在,框架层 / 解释层按规则与条目情况出现。
type AnalysisLayers struct {
	Facts       analysis.Facts        `json:"facts"`
	Framework   *analysis.Framework   `json:"framework,omitempty"`
	Explanation *analysis.Explanation `json:"explanation,omitempty"`
}

// AnalysisQuota 配额只锁解释层;事实层与框架层不消耗配额。
// ExplanationReason 为省略解释层的原因(配额用尽 / LLM 失败 / 无条目等)。
type AnalysisQuota struct {
	UsedToday            int     `json:"used_today"`
	LimitToday           int     `json:"limit_today"`
	ExplanationAvailable bool    `json:"explanation_available"`
	ExplanationReason    *string `json:"explanation_reason,omitempty"`
}

// AnalysisResult POST /analyses 与 GET /analyses/{analysis_id} 的响应:
// 三层结果、快照、来源与配额;stale=true 表示数据已更新。
type AnalysisResult struct {
	AnalysisID int64             `json:"analysis_id"`
	CreatedAt  string            `json:"created_at"`
	Status     string            `json:"status"`
	Layers     AnalysisLayers    `json:"layers"`
	Snapshot   analysis.Snapshot `json:"snapshot"`
	Sources    []analysis.Source `json:"sources"`
	Quota      AnalysisQuota     `json:"quota"`
	Stale      bool              `json:"stale"`
}

// AnalysisFeedbackRequest POST /analyses/{analysis_id}/feedback 请求体:
// entry_id 可选(当前结果级按钮不传),reason 必填。
type AnalysisFeedbackRequest struct {
	EntryID *string `json:"entry_id,omitempty"`
	Reason  string  `json:"reason"`
}

// AnalysisFeedbackResult 反馈写入结果。
type AnalysisFeedbackResult struct {
	FeedbackID int64  `json:"feedback_id"`
	CreatedAt  string `json:"created_at"`
}

// AnalysisMetrics 周检口径:反馈率 = 反馈数 / 分析数;降级率 = 降级事件数 / 分析数。
type AnalysisMetrics struct {
	From             string         `json:"from"`
	To               string         `json:"to"`
	Analyses         int            `json:"analyses"`
	Feedback         int            `json:"feedback"`
	FeedbackRate     float64        `json:"feedback_rate"`
	Degradations     int            `json:"degradations"`
	DegradationRate  float64        `json:"degradation_rate"`
	DegradationKinds map[string]int `json:"degradation_kinds"`
}
