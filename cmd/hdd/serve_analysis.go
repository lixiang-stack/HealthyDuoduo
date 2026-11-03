// 分析端点(llm-analysis 票 04):POST /api/v1/analyses 创建事实层分析并保存快照,
// GET /api/v1/analyses/{analysis_id} 读取。窗口 / 门控 / 排除 / 稀疏扩展在
// internal/analysis 纯函数层完成;handler 只做成员 / 入口校验与契约投影。
package main

import (
	"net/http"
	"strconv"
	"time"

	"healthyduoduo/internal/analysis"
	"healthyduoduo/internal/httpapi"
	"healthyduoduo/internal/pipeline"
)

// 分析配额默认值:配额只锁解释层,事实层与框架层不消耗配额(票 07 接配置)。
const analysisDailyLimitDefault = 20

const (
	analysisNotFoundMessage    = "analysis not found"
	indicatorNotFoundMessage   = "indicator not found"
	invalidAnalysisEntry       = "entry must be one of panel, indicator, clinical_note"
	invalidAnalysisWindow      = "window_months must be one of 6, 12, 24"
	analysisReportNotFound     = "report not found for this member"
	analysisClinicalNoteNeeded = "report_sha is required for clinical_note entry"
	analysisIndicatorNeeded    = "indicator_name is required for indicator entry"
	narrativePanelMessage      = "narrative panel is not analyzable"
)

// handleCreateAnalysis POST /api/v1/analyses:建事实层分析并永久保存快照。
func handleCreateAnalysis(d *deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body httpapi.AnalysisRequest
		if err := decodeJSONBody(r, &body); err != nil {
			writeErrorJSON(w, http.StatusBadRequest, err.Error())
			return
		}
		if body.MemberID <= 0 {
			writeErrorJSON(w, http.StatusBadRequest, "member_id is required")
			return
		}
		if body.Entry != analysis.EntryPanel && body.Entry != analysis.EntryIndicator &&
			body.Entry != analysis.EntryClinicalNote {
			writeErrorJSON(w, http.StatusBadRequest, invalidAnalysisEntry)
			return
		}
		months := analysis.DefaultWindowMonths
		if body.WindowMonths != nil {
			months = *body.WindowMonths
		}
		if months != 6 && months != 12 && months != 24 {
			writeErrorJSON(w, http.StatusBadRequest, invalidAnalysisWindow)
			return
		}
		panel := body.Panel
		if !pipeline.IsValidPanel(panel) {
			writeErrorJSON(w, http.StatusBadRequest, invalidPanelMessage)
			return
		}
		if body.Entry == analysis.EntryClinicalNote {
			panel = pipeline.PanelClinicalNote
			if body.ReportSHA == nil || *body.ReportSHA == "" {
				writeErrorJSON(w, http.StatusBadRequest, analysisClinicalNoteNeeded)
				return
			}
		} else if pipeline.IsNarrativePanel(panel) {
			writeErrorJSON(w, http.StatusBadRequest, narrativePanelMessage)
			return
		}
		if body.Entry == analysis.EntryIndicator && (body.IndicatorName == nil || *body.IndicatorName == "") {
			writeErrorJSON(w, http.StatusBadRequest, analysisIndicatorNeeded)
			return
		}

		member, err := d.members.MemberByID(r.Context(), body.MemberID)
		if err != nil {
			writeErrorJSON(w, http.StatusInternalServerError, "read member failed")
			return
		}
		if member == nil {
			writeErrorJSON(w, http.StatusNotFound, memberNotFoundMessage)
			return
		}

		if body.Entry == analysis.EntryIndicator {
			derived, err := d.analyses.IndicatorPanel(r.Context(), member.MemberID, *body.IndicatorName)
			if err != nil {
				writeErrorJSON(w, http.StatusInternalServerError, "read indicator failed")
				return
			}
			if derived == "" {
				writeErrorJSON(w, http.StatusNotFound, indicatorNotFoundMessage)
				return
			}
			if pipeline.IsNarrativePanel(derived) {
				writeErrorJSON(w, http.StatusBadRequest, narrativePanelMessage)
				return
			}
			panel = derived
		}

		points, err := d.analyses.AnalysisPoints(r.Context(), member.MemberID, panel)
		if err != nil {
			writeErrorJSON(w, http.StatusInternalServerError, "read analysis points failed")
			return
		}
		notes, err := d.analyses.ClinicalNotes(r.Context(), member.MemberID)
		if err != nil {
			writeErrorJSON(w, http.StatusInternalServerError, "read clinical notes failed")
			return
		}
		if body.Entry == analysis.EntryClinicalNote && !containsNote(notes, *body.ReportSHA) {
			writeErrorJSON(w, http.StatusNotFound, analysisReportNotFound)
			return
		}

		req := analysis.Request{
			MemberID:      member.MemberID,
			MemberName:    member.Name,
			Panel:         panel,
			Entry:         body.Entry,
			IndicatorName: pointerText(body.IndicatorName),
			ReportSHA:     pointerText(body.ReportSHA),
			WindowMonths:  months,
			Now:           time.Now(),
		}
		facts := analysis.BuildFacts(req, points, notes)
		status := analysis.StatusSuccess
		if facts.InsufficientEvidence {
			status = analysis.StatusInsufficientEvidence
		}
		dataVersion, err := d.analyses.DataVersion(r.Context(), member.MemberID)
		if err != nil {
			writeErrorJSON(w, http.StatusInternalServerError, "read analysis data version failed")
			return
		}
		snapshot := analysis.BuildSnapshot(req, facts)
		snapshot.DataVersion = dataVersion
		record := analysis.Record{
			MemberID:      member.MemberID,
			Panel:         panel,
			Entry:         body.Entry,
			IndicatorName: body.IndicatorName,
			WindowMonths:  months,
			Status:        status,
			Facts:         facts,
			Snapshot:      snapshot,
			Sources:       []analysis.Source{},
		}
		saved, err := d.analyses.SaveAnalysis(r.Context(), record)
		if err != nil {
			writeErrorJSON(w, http.StatusInternalServerError, "save analysis failed")
			return
		}
		writeJSON(w, http.StatusCreated, toAPIAnalysisResult(*saved))
	}
}

// handleGetAnalysis GET /api/v1/analyses/{analysis_id}:读取已保存分析(含快照)。
func handleGetAnalysis(d *deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(r.PathValue("analysis_id"), 10, 64)
		if err != nil {
			writeErrorJSON(w, http.StatusBadRequest, "analysis_id must be an integer")
			return
		}
		record, err := d.analyses.AnalysisByID(r.Context(), id)
		if err != nil {
			writeErrorJSON(w, http.StatusInternalServerError, "read analysis failed")
			return
		}
		if record == nil {
			writeErrorJSON(w, http.StatusNotFound, analysisNotFoundMessage)
			return
		}
		current, err := d.analyses.DataVersion(r.Context(), record.MemberID)
		if err != nil {
			writeErrorJSON(w, http.StatusInternalServerError, "read analysis data version failed")
			return
		}
		record.Stale = record.Snapshot.DataVersion != current
		writeJSON(w, http.StatusOK, toAPIAnalysisResult(*record))
	}
}

// toAPIAnalysisResult 已保存分析 → 契约信封;事实层数组保证非 null。
func toAPIAnalysisResult(record analysis.Record) httpapi.AnalysisResult {
	sources := record.Sources
	if sources == nil {
		sources = []analysis.Source{}
	}
	return httpapi.AnalysisResult{
		AnalysisID: record.AnalysisID,
		Status:     record.Status,
		Layers:     httpapi.AnalysisLayers{Facts: record.Facts},
		Snapshot:   record.Snapshot,
		Sources:    sources,
		Quota: httpapi.AnalysisQuota{
			UsedToday:            0,
			LimitToday:           analysisDailyLimitDefault,
			ExplanationAvailable: false,
		},
		Stale: record.Stale,
	}
}

func containsNote(notes []analysis.InputNote, reportID string) bool {
	for _, note := range notes {
		if note.ReportID == reportID {
			return true
		}
	}
	return false
}

func pointerText(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
