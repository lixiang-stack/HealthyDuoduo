// 项目组浏览查询(panel-browse 票 03):项目列表按成员聚合,项目详情按成员 + 项目组
// 聚合指标与跨报告点。只统计 status <> failed 的报告;未回填存量(reports.panels = {})
// 一律归 other,与 App ?panel= 筛选回退口径一致。指标分组业务规则在 pipeline。
package storage

import (
	"context"
	"slices"

	"github.com/jackc/pgx/v5"

	"healthyduoduo/internal/pipeline"
)

// sqlPanelSummariesByMember 项目列表一次聚合(不逐报告 N+1):
//   - 报告级 panels 为空(未回填存量)一律归 other,与 App ?panel= 筛选回退一致;
//   - report_count = 非 failed 报告包含该项目的份数;
//   - item_count = 该项目下趋势键(code > loinc > name)+ canonical unit 去重后的指标数
//     (含 not_tested);
//   - abnormal_count = 每个趋势键 + canonical unit 取最近非 not_tested 值后
//     flag ∈ high/low/abnormal 的数量。
//
// 同日期报告的「最近」按 sha 降序取最后一份,与项目详情 trend 点升序的最后一点一致。
const sqlPanelSummariesByMember = `
WITH member_reports AS (
    SELECT r.image_sha256, r.report_type, r.report_date,
           CASE WHEN cardinality(r.panels) > 0 THEN r.panels ELSE ARRAY['other'] END AS panels
    FROM reports r
    WHERE r.member_id = $1 AND r.status <> 'failed'
),
report_panels AS (
    SELECT DISTINCT image_sha256, report_type, report_date, p.panel
    FROM member_reports e CROSS JOIN LATERAL unnest(e.panels) AS p(panel)
),
panel_headers AS (
    SELECT panel,
           COUNT(*) AS report_count,
           MAX(report_date) AS latest_report_date,
           (ARRAY_AGG(image_sha256 ORDER BY report_date DESC NULLS LAST, image_sha256 DESC))[1] AS latest_report_id,
           ARRAY_AGG(DISTINCT report_type ORDER BY report_type) AS report_types
    FROM report_panels
    GROUP BY panel
),
member_items AS (
    SELECT DISTINCT i.panel, COALESCE(NULLIF(i.code, ''), NULLIF(i.loinc, ''), i.name) AS trend_key,
           i.unit, i.flag, i.id, r.report_date, r.image_sha256
    FROM report_items i JOIN reports r ON r.image_sha256 = i.report_sha256
    WHERE r.member_id = $1 AND r.status <> 'failed'
),
item_groups AS (
    SELECT DISTINCT panel, trend_key, unit FROM member_items
),
item_stats AS (
    SELECT panel, COUNT(*) AS item_count FROM item_groups GROUP BY panel
),
latest_items AS (
    SELECT DISTINCT ON (panel, trend_key, unit) panel, flag
    FROM member_items
    WHERE flag <> 'not_tested'
    ORDER BY panel, trend_key, unit, report_date DESC NULLS LAST, image_sha256 DESC, id ASC
),
abnormal_stats AS (
    SELECT panel, COUNT(*) FILTER (WHERE flag IN ('high', 'low', 'abnormal')) AS abnormal_count
    FROM latest_items GROUP BY panel
)
SELECT h.panel, h.report_count,
       COALESCE(s.item_count, 0), COALESCE(a.abnormal_count, 0),
       to_char(h.latest_report_date, 'YYYY-MM-DD'), h.latest_report_id, h.report_types
FROM panel_headers h
LEFT JOIN item_stats s ON s.panel = h.panel
LEFT JOIN abnormal_stats a ON a.panel = h.panel`

// sqlPanelReportsByMemberPanel 项目详情头部来源:包含该项目的非 failed 报告。
// 未回填存量(panels = {})一律归 other,与项目列表 / App ?panel= 筛选同一口径。
const sqlPanelReportsByMemberPanel = `
SELECT r.image_sha256, r.report_type, to_char(r.report_date, 'YYYY-MM-DD')
FROM reports r
WHERE r.member_id = $1 AND r.status <> 'failed'
  AND (
      $2 = ANY (r.panels)
      OR (cardinality(r.panels) = 0 AND $2 = 'other')
  )
ORDER BY r.report_date ASC NULLS LAST, r.image_sha256 ASC, r.report_type ASC`

// sqlPanelItemsByMemberPanel 项目详情指标来源:先取该项目组出现的趋势键
// (code > loinc > name),再取成员全部非 failed 报告里同键指标项——同一分析物
// 跨面板连接,趋势点不因项目分组差异断开。按报告日期升序 / dateless 沉底 / sha /
// item id 稳定排序;not_tested 行用于 latest_report_not_tested 与 not_tested_count,
// 不产生趋势点。
const sqlPanelItemsByMemberPanel = `
WITH panel_keys AS (
    SELECT DISTINCT COALESCE(NULLIF(i.code, ''), NULLIF(i.loinc, ''), i.name) AS trend_key
    FROM report_items i JOIN reports r ON r.image_sha256 = i.report_sha256
    WHERE r.member_id = $1 AND r.status <> 'failed' AND i.panel = $2
)
SELECT r.image_sha256, to_char(r.report_date, 'YYYY-MM-DD'),
       i.name, i.code, i.loinc, i.unit, i.unit_display, i.unit_status,
       i.value, i.ref_range, i.method, i.flag, i.low_confidence, i.review_status
FROM report_items i JOIN reports r ON r.image_sha256 = i.report_sha256
WHERE r.member_id = $1 AND r.status <> 'failed'
  AND COALESCE(NULLIF(i.code, ''), NULLIF(i.loinc, ''), i.name) IN (SELECT trend_key FROM panel_keys)
ORDER BY r.report_date ASC NULLS LAST, r.image_sha256 ASC, i.id ASC`

type panelReportRef struct {
	id         string
	reportType string
	date       *string
}

// PanelSummariesByMember 项目列表(排序由领域层做:最近日期降序、同日期按 §5.3)。
func (s *Store) PanelSummariesByMember(ctx context.Context, memberID int64) ([]pipeline.PanelSummary, error) {
	rows, err := s.pool.Query(ctx, sqlPanelSummariesByMember, memberID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []pipeline.PanelSummary
	for rows.Next() {
		var panel pipeline.PanelSummary
		if err := rows.Scan(&panel.Panel, &panel.ReportCount, &panel.ItemCount, &panel.AbnormalCount,
			&panel.LatestReportDate, &panel.LatestReportID, &panel.ReportTypes); err != nil {
			return nil, err
		}
		panel.Narrative = pipeline.IsNarrativePanel(panel.Panel)
		out = append(out, panel)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	pipeline.SortPanelSummaries(out)
	return out, nil
}

// PanelItemsByMemberPanel 项目详情:头部来源 + 按趋势键 + canonical unit 分行的指标。
// 成员存在但该项目没有报告时返回空详情(不报错;成员存在性由 HTTP 边界校验)。
func (s *Store) PanelItemsByMemberPanel(ctx context.Context, memberID int64, panel string) (*pipeline.PanelDetail, error) {
	reports, err := s.panelReports(ctx, memberID, panel)
	if err != nil {
		return nil, err
	}
	detail := &pipeline.PanelDetail{
		Panel:       panel,
		MemberID:    memberID,
		ReportCount: len(reports),
		ReportTypes: panelReportTypes(reports),
		Narrative:   pipeline.IsNarrativePanel(panel),
		Items:       []pipeline.PanelItem{},
	}
	if len(reports) == 0 {
		return detail, nil
	}
	latest := latestPanelReport(reports)
	detail.LatestReportDate = latest.date
	detail.LatestReportID = &latest.id

	rows, err := s.pool.Query(ctx, sqlPanelItemsByMemberPanel, memberID, panel)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	points, err := scanPanelItemPoints(rows)
	if err != nil {
		return nil, err
	}
	detail.Items = pipeline.BuildPanelItems(points, latest.id)
	return detail, nil
}

func scanPanelItemPoints(rows pgx.Rows) ([]pipeline.PanelItemPoint, error) {
	var points []pipeline.PanelItemPoint
	for rows.Next() {
		var point pipeline.PanelItemPoint
		if err := rows.Scan(&point.ReportID, &point.ReportDate, &point.Name, &point.Code,
			&point.Loinc, &point.Unit, &point.UnitDisplay, &point.UnitStatus,
			&point.Value, &point.RefRange, &point.Method, &point.Flag, &point.LowConfidence,
			&point.ReviewStatus); err != nil {
			return nil, err
		}
		points = append(points, point)
	}
	return points, rows.Err()
}

func (s *Store) panelReports(ctx context.Context, memberID int64, panel string) ([]panelReportRef, error) {
	rows, err := s.pool.Query(ctx, sqlPanelReportsByMemberPanel, memberID, panel)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []panelReportRef
	for rows.Next() {
		var ref panelReportRef
		if err := rows.Scan(&ref.id, &ref.reportType, &ref.date); err != nil {
			return nil, err
		}
		out = append(out, ref)
	}
	return out, rows.Err()
}

// latestPanelReport 最近报告:日期降序、dateless 沉底;同日期取 sha 降序最后一份
// (与项目列表 SQL 的 array_agg 顺序一致)。
func latestPanelReport(reports []panelReportRef) panelReportRef {
	latest := reports[0]
	for _, report := range reports[1:] {
		if panelReportLater(report, latest) {
			latest = report
		}
	}
	return latest
}

func panelReportLater(a, b panelReportRef) bool {
	if a.date == nil {
		return b.date == nil && a.id > b.id
	}
	if b.date == nil {
		return true
	}
	if *a.date != *b.date {
		return *a.date > *b.date
	}
	return a.id > b.id
}

// panelReportTypes 来源报告类别去重后的稳定字典序(other 行 / 详情头展示用)。
func panelReportTypes(reports []panelReportRef) []string {
	types := make([]string, 0, len(reports))
	for _, report := range reports {
		types = append(types, report.reportType)
	}
	slices.Sort(types)
	return slices.Compact(types)
}
