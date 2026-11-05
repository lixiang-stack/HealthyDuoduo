// 分析端点(llm-analysis 票 04):POST /api/v1/analyses 创建事实层分析并保存快照,
// GET /api/v1/analyses/{analysis_id} 读取。窗口 / 门控 / 排除 / 稀疏扩展在
// internal/analysis 纯函数层完成;handler 只做成员 / 入口校验与契约投影。
package main

import (
	"log"
	"net/http"
	"strconv"
	"time"

	"healthyduoduo/internal/analysis"
	"healthyduoduo/internal/httpapi"
	"healthyduoduo/internal/pipeline"
)

// 分析配额默认值:配额只锁解释层,事实层与框架层不消耗配额(票 07 接配置)。
const analysisDailyLimitDefault = 20

// analysisDailyLimit 配置覆盖默认值;0 表示未配置。
func (d *deps) analysisDailyLimit() int {
	if d.analysisLimit > 0 {
		return d.analysisLimit
	}
	return analysisDailyLimitDefault
}

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

		growth, err := d.analyses.LatestGrowth(r.Context(), member.MemberID)
		if err != nil {
			writeErrorJSON(w, http.StatusInternalServerError, "read latest growth failed")
			return
		}
		req := analysis.Request{
			MemberID:        member.MemberID,
			MemberName:      member.Name,
			MemberSex:       member.Sex,
			MemberBirthDate: member.BirthDate,
			Panel:           panel,
			Entry:           body.Entry,
			IndicatorName:   pointerText(body.IndicatorName),
			ReportSHA:       pointerText(body.ReportSHA),
			WindowMonths:    months,
			Now:             time.Now(),
		}
		facts := analysis.BuildFacts(req, points, notes)
		status := analysis.StatusSuccess
		if facts.InsufficientEvidence {
			status = analysis.StatusInsufficientEvidence
		}
		// 信任门控全未通过时框架层与解释层不输出(spec §4.15 规则 3)。
		framework := analysis.Framework{Labels: []analysis.FrameworkLabel{}}
		if !facts.InsufficientEvidence {
			framework = analysis.BuildFramework(req, facts, growth)
		}
		dataVersion, err := d.analyses.DataVersion(r.Context(), member.MemberID)
		if err != nil {
			writeErrorJSON(w, http.StatusInternalServerError, "read analysis data version failed")
			return
		}
		snapshot := analysis.BuildSnapshot(req, facts)
		snapshot.DataVersion = dataVersion
		if d.explainer != nil && !facts.InsufficientEvidence {
			snapshot.EntryVersions = analysis.EntryVersions(d.explainer.Match(req, facts))
		}
		record := analysis.Record{
			MemberID:      member.MemberID,
			Panel:         panel,
			Entry:         body.Entry,
			IndicatorName: body.IndicatorName,
			WindowMonths:  months,
			Status:        status,
			Facts:         facts,
			Framework:     framework,
			Snapshot:      snapshot,
			Sources:       analysis.FrameworkSources(framework),
		}
		saved, err := d.analyses.SaveAnalysis(r.Context(), record)
		if err != nil {
			writeErrorJSON(w, http.StatusInternalServerError, "save analysis failed")
			return
		}
		if facts.InsufficientEvidence && hasTrustGateFailure(facts) && d.observability != nil {
			_ = d.observability.RecordAnalysisEvent(r.Context(), saved.AnalysisID, analysis.EventTrustGateFailed, "")
		}
		if d.explainer != nil && !facts.InsufficientEvidence {
			explanation, reason := d.explainer.Explain(r.Context(), saved.AnalysisID, req, facts)
			sources := analysis.MergeSources(analysis.FrameworkSources(framework), analysis.ExplanationSources(explanation))
			if err := d.analyses.UpdateAnalysisExplanation(r.Context(), saved.AnalysisID, explanation, reason, sources); err != nil {
				writeErrorJSON(w, http.StatusInternalServerError, "save analysis explanation failed")
				return
			}
			saved.Explanation = explanation
			saved.ExplanationReason = reason
			saved.Sources = sources
		}
		quota, quotaErr := d.analysisQuota(r, saved)
		if quotaErr != nil {
			log.Printf("analysis quota read failed: %v", quotaErr)
			reason := "quota_unavailable"
			quota = httpapi.AnalysisQuota{
				UsedToday:            0,
				LimitToday:           d.analysisDailyLimit(),
				ExplanationAvailable: true,
				ExplanationReason:    &reason,
			}
		}
		writeJSON(w, http.StatusCreated, toAPIAnalysisResult(*saved, quota))
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
		quota, err := d.analysisQuota(r, record)
		if err != nil {
			log.Printf("analysis quota read failed: %v", err)
			writeErrorJSON(w, http.StatusInternalServerError, "read analysis quota failed")
			return
		}
		writeJSON(w, http.StatusOK, toAPIAnalysisResult(*record, quota))
	}
}

// analysisQuota 当前配额快照;已保存解释层仍可查看,不因当日额度用完而隐藏。
func (d *deps) analysisQuota(r *http.Request, record *analysis.Record) (httpapi.AnalysisQuota, error) {
	limit := d.analysisDailyLimit()
	used := 0
	if d.quota != nil {
		value, err := d.quota.LLMUsedToday(r.Context())
		if err != nil {
			return httpapi.AnalysisQuota{}, err
		}
		used = value
	}
	available := used < limit || record.Explanation != nil
	quota := httpapi.AnalysisQuota{
		UsedToday:            used,
		LimitToday:           limit,
		ExplanationAvailable: available,
	}
	if record.Explanation == nil && record.ExplanationReason != "" {
		reason := record.ExplanationReason
		quota.ExplanationReason = &reason
	}
	return quota, nil
}

// toAPIAnalysisResult 已保存分析 → 契约信封;事实层数组保证非 null。
func toAPIAnalysisResult(record analysis.Record, quota httpapi.AnalysisQuota) httpapi.AnalysisResult {
	sources := record.Sources
	if sources == nil {
		sources = []analysis.Source{}
	}
	layers := httpapi.AnalysisLayers{Facts: record.Facts}
	if record.Framework.Labels != nil {
		framework := record.Framework
		layers.Framework = &framework
	}
	if record.Explanation != nil {
		layers.Explanation = record.Explanation
	}
	return httpapi.AnalysisResult{
		AnalysisID: record.AnalysisID,
		CreatedAt:  record.CreatedAt.Format(time.RFC3339),
		Status:     record.Status,
		Layers:     layers,
		Snapshot:   record.Snapshot,
		Sources:    sources,
		Quota:      quota,
		Stale:      record.Stale,
	}
}

// hasTrustGateFailure 只有「有候选点但全部被信任门控排除」才算降级;
// 空成员/无数据不是 trust_gate_failed,避免周检降级率被空档案抬高。
func hasTrustGateFailure(facts analysis.Facts) bool {
	for _, exclusion := range facts.Excluded {
		if exclusion.Reason == analysis.ReasonLowConfidence {
			return true
		}
	}
	return false
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
