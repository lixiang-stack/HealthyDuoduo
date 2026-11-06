// 分析用例的持久化边界与已保存结果形状。纯事实层 / 框架层组装在 facts.go、
// framework.go;本文件只定义存储端口,不依赖 PG / HTTP。
package analysis

import (
	"context"
	"time"
)

// Source 分析引用的标准 / 指南(框架层与解释层共用;模型不得自行书写依据)。
type Source struct {
	Standard string  `json:"standard"`
	Note     string  `json:"note"`
	URL      *string `json:"url"`
}

// Record 已保存的分析:事实层 + 框架层 + 解释层 + 快照 + 入口元数据。
// 快照永久保留;stale 由读取时比较当前数据指纹与快照 data_version 得出。
type Record struct {
	AnalysisID        int64
	MemberID          int64
	Panel             string
	Entry             string
	IndicatorName     *string
	WindowMonths      int
	Status            string
	Facts             Facts
	Framework         Framework
	Explanation       *Explanation
	ExplanationReason string
	Snapshot          Snapshot
	Sources           []Source
	Stale             bool
	CreatedAt         time.Time
}

// Feedback 一次「这不对」反馈;只记录,不自动降级条目。
type Feedback struct {
	FeedbackID int64
	AnalysisID int64
	EntryID    *string
	Reason     string
	CreatedAt  time.Time
}

// Metrics 周检口径:反馈率 = 反馈数 / 分析数;降级率 = 降级事件数 / 分析数。
type Metrics struct {
	From             string         `json:"from"`
	To               string         `json:"to"`
	Analyses         int            `json:"analyses"`
	Feedback         int            `json:"feedback"`
	FeedbackRate     float64        `json:"feedback_rate"`
	Degradations     int            `json:"degradations"`
	DegradationRate  float64        `json:"degradation_rate"`
	DegradationKinds map[string]int `json:"degradation_kinds"`
}

// Store 分析数据源与快照持久化端口(serve 分析端点注入 *storage.Store)。
type Store interface {
	// IndicatorPanel 按指标名 / 标准码 / LOINC 找到所属项目组;找不到返回空串。
	IndicatorPanel(ctx context.Context, memberID int64, name string) (string, error)
	// AnalysisPoints 成员某项目组的全部指标点(含 failed 报告,由领域层排除并给原因)。
	AnalysisPoints(ctx context.Context, memberID int64, panel string) ([]InputPoint, error)
	// ClinicalNotes 成员全部病历报告与段落(窗口 / 排除由领域层判定)。
	ClinicalNotes(ctx context.Context, memberID int64) ([]InputNote, error)
	// LatestGrowth 最近一次生长记录;没有返回 nil, nil。
	LatestGrowth(ctx context.Context, memberID int64) (*InputGrowth, error)
	// SaveAnalysis 保存事实层 / 框架层与快照,返回带 analysis_id / created_at 的记录。
	SaveAnalysis(ctx context.Context, record Record) (*Record, error)
	// UpdateAnalysisExplanation 解释层成功后回写;失败时只记录原因。
	UpdateAnalysisExplanation(ctx context.Context, id int64, explanation *Explanation, reason string, sources []Source) error
	// AnalysisByID 读取已保存分析;不存在返回 nil, nil。
	AnalysisByID(ctx context.Context, id int64) (*Record, error)
	// DataVersion 成员报告 / 指标项 / 核对状态 / 生长记录的当前指纹;与快照中的值不同即 stale。
	DataVersion(ctx context.Context, memberID int64) (string, error)
}

// ExplanationCache 解释层输入指纹 → 已生成解释;命中时复用,不调用 LLM、不消耗配额。
// 缓存按成员隔离并随成员删除级联;读写失败只影响复用机会,不改变分析结果。
type ExplanationCache interface {
	// FindExplanation 按成员 + 输入指纹读取已保存解释;未命中返回 nil, nil。
	FindExplanation(ctx context.Context, memberID int64, key string) (*Explanation, error)
	// SaveExplanation 保存成功生成的解释;相同 key 已存在时保留首次结果。
	SaveExplanation(ctx context.Context, memberID int64, key string, explanation *Explanation) error
}

// QuotaStore 解释层每日配额(前两层不调 LLM、不消耗配额)。
type QuotaStore interface {
	// ConsumeLLMQuota 原子占用一次解释层调用;allowed=false 时 used 为当前已用次数。
	ConsumeLLMQuota(ctx context.Context, analysisID int64, limit int) (used int, allowed bool, err error)
	// LLMUsedToday 当前已用次数(读取已保存分析时展示今日剩余)。
	LLMUsedToday(ctx context.Context) (int, error)
}

// ObservabilityStore 降级事件与用户反馈;供每周计算反馈率与降级率。
type ObservabilityStore interface {
	RecordAnalysisEvent(ctx context.Context, analysisID int64, kind, detail string) error
	RecordFeedback(ctx context.Context, feedback Feedback) (*Feedback, error)
	AnalysisMetrics(ctx context.Context, from, to time.Time) (Metrics, error)
}
