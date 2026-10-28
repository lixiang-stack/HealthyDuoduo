// 项目组浏览域(panel-browse 票 03):项目列表与项目详情的业务视图、固定优先级
// 与存储边界。分类由识别端给出,Go 只聚合 / 排序 / 返回;这里不维护「指标名 →
// 项目组」映射。排序键是产品定义的并列裁决顺序(spec §5.3),不是医学优先级。
package pipeline

import (
	"cmp"
	"context"
	"slices"
	"sort"
	"strings"
)

// 项目组 code(与识别端 / OpenAPI PanelCode 枚举同源;other 为唯一跨类别兜底)。
const PanelImaging = "imaging"

// panelOrder spec §5.3 的固定优先级:命中项数降序后,同数按此顺序(数值小优先);
// other 始终兜底。唯一来源是 schemas/report.schema.json $defs.panel.x-panel-order,
// contract test 锁定 Go 与识别端 / App 同源。新增 code 必须显式决定插入位置。
var panelOrder = []string{
	"liver", "kidney", "lipid", "glucose", "uric_acid", "electrolyte",
	"vitanemia", "thyroid", "sex_hormone", "infection", "autoimmune",
	"coag", "cbc", "urinalysis", "imaging", "other",
}

var panelRank = func() map[string]int {
	ranks := make(map[string]int, len(panelOrder))
	for i, code := range panelOrder {
		ranks[code] = i
	}
	return ranks
}()

// IsValidPanel 项目组 code 是否属于契约枚举(非法 code 在 HTTP 边界回 400)。
func IsValidPanel(code string) bool {
	_, ok := panelRank[code]
	return ok
}

// IsNarrativePanel 叙述型项目组(影像):项目列表直接进报告详情,不做项目详情 / 趋势。
func IsNarrativePanel(code string) bool {
	return code == PanelImaging
}

// PanelSummary 成员的单个项目组聚合(项目列表一行;只包含 report_count > 0 的项目)。
type PanelSummary struct {
	Panel            string
	ReportCount      int
	ItemCount        int
	AbnormalCount    int
	LatestReportDate *string
	LatestReportID   *string
	ReportTypes      []string
	Narrative        bool
}

// PanelItem 项目详情中按 name+unit 分行的指标(跨单位不合并、不换算)。
// Latest 是最近一个非 not_tested 点(没有则 nil);Points 为同组跨报告点,日期升序,
// 不含 not_tested。ReportCount 为有非 not_tested 点的报告数。
type PanelItem struct {
	Name                  string
	Unit                  *string
	RefRange              *string
	Latest                *TrendPointRow
	Points                []TrendPointRow
	ReportCount           int
	NotTestedCount        int
	LatestReportNotTested bool
}

// PanelItemPoint 项目详情分组前的平铺指标行(存储按报告日期升序 / sha / item id
// 读出;领域层负责 name+unit 分组、最近点与 not_tested 口径)。
type PanelItemPoint struct {
	ReportID      string
	ReportDate    *string
	Name          string
	Unit          *string
	Value         *string
	RefRange      *string
	Method        *string
	Flag          string
	LowConfidence bool
	ReviewStatus  string
}

// PanelDetail 项目详情(成员维度;imaging 时 Items 为空,narrative=true)。
type PanelDetail struct {
	Panel            string
	MemberID         int64
	ReportCount      int
	LatestReportDate *string
	LatestReportID   *string
	ReportTypes      []string
	Narrative        bool
	Items            []PanelItem
}

// PanelStore 项目浏览持久化边界(serve 项目列表 / 详情端点)。
// 与 TrendStore 同惯例:独立窄接口,serve 经 deps.panels 注入同一 *storage.Store。
type PanelStore interface {
	// PanelSummariesByMember 当前成员非 failed 报告的项目组聚合;只返回 report_count > 0
	// 的项目,排序由领域层做(最近报告日期降序、同日期按固定优先级)。
	PanelSummariesByMember(ctx context.Context, memberID int64) ([]PanelSummary, error)
	// PanelItemsByMemberPanel 当前成员某项目组详情;成员存在但无该项目时返回空详情
	// (report_count=0, items=[]),成员存在性由 HTTP 边界校验。
	PanelItemsByMemberPanel(ctx context.Context, memberID int64, panel string) (*PanelDetail, error)
}

// SortPanelSummaries 项目列表排序:最近报告日期降序(null 沉底),同日期按 §5.3
// 固定优先级。原地排序,保持同日期同优先级的既有顺序(稳定)。
func SortPanelSummaries(panels []PanelSummary) {
	sort.SliceStable(panels, func(i, j int) bool {
		a, b := panels[i], panels[j]
		ad, bd := "", ""
		if a.LatestReportDate != nil {
			ad = *a.LatestReportDate
		}
		if b.LatestReportDate != nil {
			bd = *b.LatestReportDate
		}
		if ad != bd {
			if ad == "" {
				return false
			}
			if bd == "" {
				return true
			}
			return ad > bd
		}
		return panelRankOf(a.Panel) < panelRankOf(b.Panel)
	})
}

func panelRankOf(code string) int {
	if rank, ok := panelRank[code]; ok {
		return rank
	}
	return len(panelOrder)
}

type panelItemKey struct {
	name string
	unit string
}

func panelUnitKey(unit *string) string {
	if unit == nil {
		return ""
	}
	return strings.TrimSpace(*unit)
}

type panelItemAccumulator struct {
	name                     string
	unit                     *string
	points                   []TrendPointRow
	seenPointReports         map[string]bool
	reportCount              int
	seenNotTestedReports     map[string]bool
	notTestedCount           int
	latestOccurrenceDate     *string
	latestOccurrenceReportID string
	latestOccurrenceFlag     string
}

// BuildPanelItems 将平铺的跨报告指标行按 name+unit 分组,并计算最近点 / 跨报告点 /
// not_tested 口径。latestReportID 用于「其余项按最新报告内顺序」的排序。
// 输入按报告日期升序 / dateless 沉底 / sha / item id 稳定排序。
func BuildPanelItems(points []PanelItemPoint, latestReportID string) []PanelItem {
	groups := make(map[panelItemKey]*panelItemAccumulator)
	var order []panelItemKey
	latestOrder := make(map[panelItemKey]int)
	nextLatestRank := 0

	for _, point := range points {
		key := panelItemKey{name: point.Name, unit: panelUnitKey(point.Unit)}
		group, ok := groups[key]
		if !ok {
			group = &panelItemAccumulator{
				name:                 point.Name,
				unit:                 point.Unit,
				seenPointReports:     make(map[string]bool),
				seenNotTestedReports: make(map[string]bool),
			}
			groups[key] = group
			order = append(order, key)
		}
		if point.ReportID == latestReportID {
			if _, seen := latestOrder[key]; !seen {
				latestOrder[key] = nextLatestRank
				nextLatestRank++
			}
		}
		trendPoint := TrendPointRow{
			ReportID:      point.ReportID,
			ReportDate:    point.ReportDate,
			Value:         point.Value,
			Unit:          point.Unit,
			RefRange:      point.RefRange,
			Method:        point.Method,
			Flag:          point.Flag,
			LowConfidence: point.LowConfidence,
			ReviewStatus:  point.ReviewStatus,
		}
		if point.Flag == FlagNotTested {
			if !group.seenNotTestedReports[point.ReportID] {
				group.seenNotTestedReports[point.ReportID] = true
				group.notTestedCount++
			}
		} else if !group.seenPointReports[point.ReportID] {
			group.seenPointReports[point.ReportID] = true
			group.reportCount++
			group.points = append(group.points, trendPoint)
		}
		if group.latestOccurrenceReportID == "" ||
			panelReportLater(point.ReportID, point.ReportDate,
				group.latestOccurrenceReportID, group.latestOccurrenceDate) {
			group.latestOccurrenceDate = point.ReportDate
			group.latestOccurrenceReportID = point.ReportID
			group.latestOccurrenceFlag = point.Flag
		}
	}

	items := make([]PanelItem, 0, len(order))
	for _, key := range order {
		group := groups[key]
		item := PanelItem{
			Name:                  group.name,
			Unit:                  group.unit,
			ReportCount:           group.reportCount,
			NotTestedCount:        group.notTestedCount,
			LatestReportNotTested: group.latestOccurrenceFlag == FlagNotTested,
			Points:                group.points,
		}
		if latest := latestPoint(group.points); latest != nil {
			item.Latest = latest
			item.Unit = latest.Unit
			item.RefRange = latest.RefRange
		}
		items = append(items, item)
	}

	// 异常项在前;其余按最新报告内指标顺序;同序按名称稳定排序(跨单位行保持插入序)。
	slices.SortStableFunc(items, func(a, b PanelItem) int {
		aAbnormal, bAbnormal := panelItemAbnormal(a), panelItemAbnormal(b)
		if aAbnormal != bAbnormal {
			if aAbnormal {
				return -1
			}
			return 1
		}
		aRank, aInLatest := latestOrder[panelItemKey{a.Name, panelUnitKey(a.Unit)}]
		bRank, bInLatest := latestOrder[panelItemKey{b.Name, panelUnitKey(b.Unit)}]
		if aInLatest != bInLatest {
			if aInLatest {
				return -1
			}
			return 1
		}
		if aRank != bRank {
			return cmp.Compare(aRank, bRank)
		}
		return cmp.Compare(a.Name, b.Name)
	})
	return items
}

// panelReportLater 报告新鲜度:日期降序、dateless 沉底;同日期取 sha 降序最后一份
// (与项目列表聚合和趋势点升序的最后一点一致)。
func panelReportLater(aID string, aDate *string, bID string, bDate *string) bool {
	if aDate == nil {
		return bDate == nil && aID > bID
	}
	if bDate == nil {
		return true
	}
	if *aDate != *bDate {
		return *aDate > *bDate
	}
	return aID > bID
}

// latestPoint 点列按日期升序、dateless 沉底;最近点 = 最后一个有日期的点,
// 全无日期时取最后一点(与趋势端点「日期为 null 不参与连线」口径一致)。
func latestPoint(points []TrendPointRow) *TrendPointRow {
	for i := len(points) - 1; i >= 0; i-- {
		if points[i].ReportDate != nil {
			return &points[i]
		}
	}
	if len(points) == 0 {
		return nil
	}
	return &points[len(points)-1]
}

func panelItemAbnormal(item PanelItem) bool {
	if item.Latest == nil {
		return false
	}
	switch item.Latest.Flag {
	case FlagHigh, FlagLow, FlagAbnormal:
		return true
	default:
		return false
	}
}
