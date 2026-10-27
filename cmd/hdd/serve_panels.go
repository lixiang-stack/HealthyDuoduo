// 项目浏览端点(panel-browse 票 03):GET /members/{member_id}/panels 项目列表、
// GET /members/{member_id}/panels/{panel} 项目详情。聚合与排序在存储 / 领域层完成,
// handler 只做成员与项目组 code 校验、契约投影。非法 panel → 400;未知成员 → 404。
package main

import (
	"net/http"
	"strconv"

	"healthyduoduo/internal/httpapi"
	"healthyduoduo/internal/pipeline"
)

// invalidPanelMessage 非法项目组 code 的中性错误说明(契约 400)。
const invalidPanelMessage = "invalid panel code"

// handleListMemberPanels GET /api/v1/members/{member_id}/panels:当前成员非 failed
// 报告的项目组聚合;没有可归类报告时回空数组(不是 null)。
func handleListMemberPanels(d *deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		memberID, err := strconv.ParseInt(r.PathValue("member_id"), 10, 64)
		if err != nil {
			writeErrorJSON(w, http.StatusNotFound, memberNotFoundMessage)
			return
		}
		if !requireMember(w, r, d.members, memberID) {
			return
		}
		summaries, err := d.panels.PanelSummariesByMember(r.Context(), memberID)
		if err != nil {
			writeErrorJSON(w, http.StatusInternalServerError, "list panels failed")
			return
		}
		list := httpapi.PanelsList{Panels: make([]httpapi.PanelSummary, 0, len(summaries))}
		for _, summary := range summaries {
			list.Panels = append(list.Panels, toAPIPanelSummary(summary))
		}
		writeJSON(w, http.StatusOK, list)
	}
}

// handleGetPanelDetail GET /api/v1/members/{member_id}/panels/{panel}:项目详情
// (成员存在但该项目没有报告 → 200 + 空详情;非法 panel → 400)。
func handleGetPanelDetail(d *deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		memberID, err := strconv.ParseInt(r.PathValue("member_id"), 10, 64)
		if err != nil {
			writeErrorJSON(w, http.StatusNotFound, memberNotFoundMessage)
			return
		}
		panel := r.PathValue("panel")
		if !pipeline.IsValidPanel(panel) {
			writeErrorJSON(w, http.StatusBadRequest, invalidPanelMessage)
			return
		}
		if !requireMember(w, r, d.members, memberID) {
			return
		}
		detail, err := d.panels.PanelItemsByMemberPanel(r.Context(), memberID, panel)
		if err != nil {
			writeErrorJSON(w, http.StatusInternalServerError, "read panel detail failed")
			return
		}
		writeJSON(w, http.StatusOK, toAPIPanelDetail(*detail))
	}
}

// toAPIPanelSummary 业务项目行 → 契约形状(数组字段非 null)。
func toAPIPanelSummary(summary pipeline.PanelSummary) httpapi.PanelSummary {
	reportTypes := summary.ReportTypes
	if reportTypes == nil {
		reportTypes = []string{}
	}
	return httpapi.PanelSummary{
		Panel:            summary.Panel,
		ReportCount:      summary.ReportCount,
		ItemCount:        summary.ItemCount,
		AbnormalCount:    summary.AbnormalCount,
		LatestReportDate: summary.LatestReportDate,
		LatestReportID:   summary.LatestReportID,
		ReportTypes:      reportTypes,
		Narrative:        summary.Narrative,
	}
}

// toAPIPanelDetail 业务项目详情 → 契约形状;items / points 空数组而非 null。
func toAPIPanelDetail(detail pipeline.PanelDetail) httpapi.PanelDetail {
	reportTypes := detail.ReportTypes
	if reportTypes == nil {
		reportTypes = []string{}
	}
	items := make([]httpapi.PanelItem, 0, len(detail.Items))
	for _, item := range detail.Items {
		var latest *httpapi.TrendPoint
		if item.Latest != nil {
			point := toAPITrendPoint(*item.Latest)
			latest = &point
		}
		items = append(items, httpapi.PanelItem{
			Name:                  item.Name,
			Unit:                  item.Unit,
			RefRange:              item.RefRange,
			Latest:                latest,
			Points:                toAPITrendPoints(item.Points),
			ReportCount:           item.ReportCount,
			NotTestedCount:        item.NotTestedCount,
			LatestReportNotTested: item.LatestReportNotTested,
		})
	}
	return httpapi.PanelDetail{
		Panel:            detail.Panel,
		MemberID:         detail.MemberID,
		ReportCount:      detail.ReportCount,
		LatestReportDate: detail.LatestReportDate,
		LatestReportID:   detail.LatestReportID,
		ReportTypes:      reportTypes,
		Narrative:        detail.Narrative,
		Items:            items,
	}
}

func toAPITrendPoints(rows []pipeline.TrendPointRow) []httpapi.TrendPoint {
	out := make([]httpapi.TrendPoint, 0, len(rows))
	for _, row := range rows {
		out = append(out, toAPITrendPoint(row))
	}
	return out
}
