// 分析用例的持久化边界与已保存结果形状。纯事实层组装在 facts.go;
// 本文件只定义存储端口,不依赖 PG / HTTP。
package analysis

import (
	"context"
	"time"
)

// Source 分析引用的标准 / 指南(事实层当前为空;解释层工单填充)。
type Source struct {
	Standard string  `json:"standard"`
	Note     string  `json:"note"`
	URL      *string `json:"url"`
}

// Record 已保存的分析:事实层 + 快照 + 入口元数据。
// 快照永久保留;stale 由读取时比较当前数据指纹与快照 data_version 得出。
type Record struct {
	AnalysisID    int64
	MemberID      int64
	Panel         string
	Entry         string
	IndicatorName *string
	WindowMonths  int
	Status        string
	Facts         Facts
	Snapshot      Snapshot
	Sources       []Source
	Stale         bool
	CreatedAt     time.Time
}

// Store 分析数据源与快照持久化端口(serve 分析端点注入 *storage.Store)。
type Store interface {
	// IndicatorPanel 按指标名 / 标准码 / LOINC 找到所属项目组;找不到返回空串。
	IndicatorPanel(ctx context.Context, memberID int64, name string) (string, error)
	// AnalysisPoints 成员某项目组的全部指标点(含 failed 报告,由领域层排除并给原因)。
	AnalysisPoints(ctx context.Context, memberID int64, panel string) ([]InputPoint, error)
	// ClinicalNotes 成员全部病历报告与段落(窗口 / 排除由领域层判定)。
	ClinicalNotes(ctx context.Context, memberID int64) ([]InputNote, error)
	// SaveAnalysis 保存事实层与快照,返回带 analysis_id / created_at 的记录。
	SaveAnalysis(ctx context.Context, record Record) (*Record, error)
	// AnalysisByID 读取已保存分析;不存在返回 nil, nil。
	AnalysisByID(ctx context.Context, id int64) (*Record, error)
	// DataVersion 成员报告 / 指标项 / 核对状态的当前指纹;与快照中的值不同即 stale。
	DataVersion(ctx context.Context, memberID int64) (string, error)
}
