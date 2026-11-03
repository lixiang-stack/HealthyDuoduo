// 生长记录端点(llm-analysis 票 02):POST/GET /api/v1/members/{member_id}/growth。
// 身高体重是带测量日期的测量值;BMI 在读取时用测量时点值实时计算,不缓存。
// 手输来源只输出 BMI 数值,不输出超重 / 肥胖判定(百分位规则见后续工单)。
package main

import (
	"net/http"
	"strconv"
	"time"

	"healthyduoduo/internal/httpapi"
	"healthyduoduo/internal/pipeline"
)

// handleCreateGrowth POST /api/v1/members/{member_id}/growth:新增一条测量记录,201 返回。
func handleCreateGrowth(d *deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		memberID, err := strconv.ParseInt(r.PathValue("member_id"), 10, 64)
		if err != nil {
			writeErrorJSON(w, http.StatusNotFound, memberNotFoundMessage)
			return
		}
		var body httpapi.GrowthRecordCreate
		if err := decodeJSONBody(r, &body); err != nil {
			writeErrorJSON(w, http.StatusBadRequest, err.Error())
			return
		}
		profile := pipeline.GrowthProfile{
			MeasuredOn: body.MeasuredOn, HeightCm: body.HeightCm,
			WeightKg: body.WeightKg, Source: body.Source,
		}
		if err := pipeline.ValidateGrowthProfile(profile, time.Now()); err != nil {
			writeErrorJSON(w, http.StatusBadRequest, err.Error())
			return
		}
		if !requireMember(w, r, d.members, memberID) {
			return
		}
		record, err := d.growth.CreateGrowth(r.Context(), memberID, profile)
		if err != nil {
			writeErrorJSON(w, http.StatusInternalServerError, "create growth record failed")
			return
		}
		writeJSON(w, http.StatusCreated, toAPIGrowthRecord(*record))
	}
}

// handleListGrowth GET /api/v1/members/{member_id}/growth:按测量日期降序返回全部记录;
// 空列表回 [] 而非 null。未知成员 404。
func handleListGrowth(d *deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		memberID, err := strconv.ParseInt(r.PathValue("member_id"), 10, 64)
		if err != nil {
			writeErrorJSON(w, http.StatusNotFound, memberNotFoundMessage)
			return
		}
		if !requireMember(w, r, d.members, memberID) {
			return
		}
		records, err := d.growth.ListGrowth(r.Context(), memberID)
		if err != nil {
			writeErrorJSON(w, http.StatusInternalServerError, "list growth records failed")
			return
		}
		list := httpapi.GrowthList{Records: make([]httpapi.GrowthRecord, 0, len(records))}
		for _, record := range records {
			list.Records = append(list.Records, toAPIGrowthRecord(record))
		}
		writeJSON(w, http.StatusOK, list)
	}
}

// toAPIGrowthRecord 业务视图 → 契约形状;BMI 已由领域层实时计算。
func toAPIGrowthRecord(record pipeline.GrowthRecord) httpapi.GrowthRecord {
	return httpapi.GrowthRecord{
		GrowthID:     record.GrowthID,
		MemberID:     record.MemberID,
		MeasuredOn:   record.MeasuredOn,
		HeightCm:     record.HeightCm,
		WeightKg:     record.WeightKg,
		Source:       record.Source,
		BMI:          record.BMI,
		WeightStatus: record.WeightStatus,
	}
}
