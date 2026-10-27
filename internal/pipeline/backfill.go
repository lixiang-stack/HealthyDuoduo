// 存量项目组回填(panel-browse 票 05):只经识别服务 /classify 重算 panel,
// 按 item id 更新 report_items.panel 与 reports.panels;不重写指标项、不碰核对状态 / 修正对。
package pipeline

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"

	"healthyduoduo/internal/contract"
)

// ErrClassifierUnavailable 标记识别服务整体不可用(连接失败 / /classify 端点缺失 /
// 5xx),命令应停止并提示部署顺序;单份报告的 4xx 数据错误仍逐份继续。
var ErrClassifierUnavailable = errors.New("recognizer unavailable")

// 跳过原因(回填计数中 skipped 的口径);CLI 原样展示。
const (
	SkipFailedReport = "failed report"
	SkipUpToDate     = "up to date"
)

// BackfillStore 存量回填的持久化边界(与 Store 分开:只服务 backfill-panels)。
type BackfillStore interface {
	// AllReportsForBackfill 全部报告(日期降序、dateless 沉底、sha 升序),limit > 0 时取前 N 份;
	// 含指标项 id / name / unit / 当前 panel。
	AllReportsForBackfill(ctx context.Context, limit int) ([]BackfillReport, error)
	// UpdateReportPanels 单事务只写项目组列:按 item id 更新 report_items.panel,
	// 更新 reports.panels;核对状态、修正对与 value/unit/ref_range 不动。
	UpdateReportPanels(ctx context.Context, reportSha string, panels []string, itemPanels map[int64]string) error
}

// PanelClassifier 识别服务 /classify 边界(存量回填与未来重算共用)。
type PanelClassifier interface {
	// Classify 输入报告类别 + 当前指标名 / 单位;item_panels 与 items 等长同序。
	Classify(ctx context.Context, reportType string, items []ClassifyItem) (panels []string, itemPanels []string, err error)
}

// ClassifyItem /classify 请求的单个指标项;unit 缺失为 nil。
type ClassifyItem struct {
	Name string
	Unit *string
}

// BackfillReport 回填扫描到的单份报告(指标项带数据库 id,按 id 回写 panel)。
type BackfillReport struct {
	Sha256     string
	ReportType string
	Status     string
	Panels     []string
	Items      []BackfillItem
}

// BackfillItem 回填需要的指标项最小字段(不读值 / 核对状态等)。
type BackfillItem struct {
	ID    int64
	Name  string
	Unit  *string
	Panel string
}

// BackfillOptions 回填范围与写入开关。
type BackfillOptions struct {
	Limit  int  // 0 = 全部;按报告数,不按指标项行数
	DryRun bool // true = 只计算并输出拟更新结果,不写库
}

// BackfillChange 单个指标项的拟更新差异(dry-run 输出与测试断言用)。
type BackfillChange struct {
	ItemID int64
	Name   string
	From   string
	To     string
}

// BackfillReportOutcome 单份报告的回填结果;Skipped / Err 互斥,均无 = 已更新。
type BackfillReportOutcome struct {
	Sha256     string
	Panels     []string
	Changes    []BackfillChange
	ItemCount  int
	Skipped    bool
	SkipReason string
	Err        error
}

// BackfillResult 全量回填结果;Updated + Skipped + Failed = 已扫描报告数,
// Total 为本次查询到的报告总数(识别服务不可达提前停止时大于已扫描数)。
// DryRun 时 Updated 表示拟更新数(未写库),CLI 用「would update」展示。
type BackfillResult struct {
	Reports []BackfillReportOutcome
	Total   int
	Updated int
	Skipped int
	Failed  int
}

// BackfillPanels 遍历报告并重算项目组。失败报告 (failed) 不参与项目视图,直接跳过;
// 其余报告调用 /classify,只有差异时才写库。逐份失败继续,识别服务不可达时立即停止。
func BackfillPanels(ctx context.Context, st BackfillStore, cl PanelClassifier, opts BackfillOptions) (*BackfillResult, error) {
	reports, err := st.AllReportsForBackfill(ctx, opts.Limit)
	if err != nil {
		return nil, fmt.Errorf("list reports for backfill: %w", err)
	}
	res := &BackfillResult{
		Reports: make([]BackfillReportOutcome, 0, len(reports)),
		Total:   len(reports),
	}
	for _, report := range reports {
		out := BackfillReportOutcome{
			Sha256:    report.Sha256,
			ItemCount: len(report.Items),
		}
		if report.Status == contract.StatusFailed {
			out.Skipped = true
			out.SkipReason = SkipFailedReport
			res.Skipped++
			res.Reports = append(res.Reports, out)
			continue
		}

		panels, itemPanels, err := cl.Classify(ctx, report.ReportType, classifyItems(report.Items))
		if err != nil {
			out.Err = fmt.Errorf("classify: %w", err)
			res.Failed++
			res.Reports = append(res.Reports, out)
			if errors.Is(err, ErrClassifierUnavailable) {
				return res, fmt.Errorf("classify report %s: %w", report.Sha256, err)
			}
			continue
		}
		if err := validateClassifyResult(report.ReportType, report.Items, panels, itemPanels); err != nil {
			out.Err = err
			res.Failed++
			res.Reports = append(res.Reports, out)
			continue
		}

		out.Panels = panels
		itemUpdates := make(map[int64]string, len(report.Items))
		for i, item := range report.Items {
			itemUpdates[item.ID] = itemPanels[i]
			if item.Panel != itemPanels[i] {
				out.Changes = append(out.Changes, BackfillChange{
					ItemID: item.ID, Name: item.Name, From: item.Panel, To: itemPanels[i],
				})
			}
		}
		if len(out.Changes) == 0 && slices.Equal(report.Panels, panels) {
			out.Skipped = true
			out.SkipReason = SkipUpToDate
			res.Skipped++
			res.Reports = append(res.Reports, out)
			continue
		}
		if !opts.DryRun {
			if err := st.UpdateReportPanels(ctx, report.Sha256, panels, itemUpdates); err != nil {
				out.Err = fmt.Errorf("update panels: %w", err)
				res.Failed++
				res.Reports = append(res.Reports, out)
				continue
			}
		}
		res.Updated++
		res.Reports = append(res.Reports, out)
	}
	return res, nil
}

func classifyItems(items []BackfillItem) []ClassifyItem {
	out := make([]ClassifyItem, 0, len(items))
	for _, item := range items {
		out = append(out, ClassifyItem{Name: item.Name, Unit: item.Unit})
	}
	return out
}

// validateClassifyResult 防御跨服务契约漂移:长度不对、空 code、零指标项报告的
// 影像 / other 兜底、报告级 panels 与指标项 panel 集合不一致(重复 / 漂移)时拒绝写入;
// 排序仍以识别端为准,Go 不维护优先级表。
func validateClassifyResult(reportType string, items []BackfillItem, panels, itemPanels []string) error {
	if len(itemPanels) != len(items) {
		return fmt.Errorf("classify returned %d item_panels for %d items", len(itemPanels), len(items))
	}
	if len(panels) == 0 {
		return errors.New("classify returned no report panels")
	}
	if len(items) == 0 {
		want := "other"
		if reportType == contract.ReportTypeImaging {
			want = "imaging"
		}
		if len(panels) != 1 || panels[0] != want {
			return fmt.Errorf("classify returned panels %v for zero-item %s report, want [%s]", panels, reportType, want)
		}
		return nil
	}
	reportSet := make(map[string]struct{}, len(panels))
	for _, panel := range panels {
		if panel == "" {
			return errors.New("classify returned empty report panel")
		}
		if _, ok := reportSet[panel]; ok {
			return fmt.Errorf("classify returned duplicate report panel %q", panel)
		}
		reportSet[panel] = struct{}{}
	}
	itemSet := make(map[string]struct{}, len(itemPanels))
	for _, panel := range itemPanels {
		if panel == "" {
			return errors.New("classify returned empty item_panel")
		}
		itemSet[panel] = struct{}{}
	}
	if !maps.Equal(reportSet, itemSet) {
		return fmt.Errorf("classify report panels %v do not match item panels %v", panels, itemPanels)
	}
	return nil
}
