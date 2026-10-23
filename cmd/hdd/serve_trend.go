// 趋势端点(票 02):GET /api/v1/members/{member_id}/indicators/{name}/trend。
// 成员维度时序;unit 为空 = 全部单位。契约只声明 200/404,非法 member_id 也按
// 「未知成员」处理(不返回 400);无点返回 200 + points: []。
package main

import (
	"net/http"
	"strconv"

	"healthyduoduo/internal/httpapi"
	"healthyduoduo/internal/pipeline"
)

// handleGetTrend GET /api/v1/members/{member_id}/indicators/{name}/trend:
// 同一指标项名称的成员维度时序点;reviewed = review_status ∈ {confirmed, corrected}。
func handleGetTrend(d *deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		memberID, err := strconv.ParseInt(r.PathValue("member_id"), 10, 64)
		if err != nil {
			writeErrorJSON(w, http.StatusNotFound, memberNotFoundMessage)
			return
		}
		if !requireMember(w, r, d.members, memberID) {
			return
		}

		name := r.PathValue("name")
		unit := r.URL.Query().Get("unit")
		rows, err := d.trends.TrendPoints(r.Context(), memberID, name, unit)
		if err != nil {
			writeErrorJSON(w, http.StatusInternalServerError, "list trend failed")
			return
		}

		result := httpapi.TrendResult{
			MemberID: memberID,
			Name:     name,
			Points:   make([]httpapi.TrendPoint, 0, len(rows)),
		}
		for _, row := range rows {
			reviewed := row.ReviewStatus == pipeline.ReviewStatusConfirmed ||
				row.ReviewStatus == pipeline.ReviewStatusCorrected
			result.Points = append(result.Points, httpapi.TrendPoint{
				ReportID:      row.ReportID,
				ReportDate:    row.ReportDate,
				Value:         row.Value,
				Unit:          row.Unit,
				RefRange:      row.RefRange,
				Method:        row.Method,
				Flag:          row.Flag,
				LowConfidence: row.LowConfidence,
				Reviewed:      reviewed,
			})
		}
		writeJSON(w, http.StatusOK, result)
	}
}
