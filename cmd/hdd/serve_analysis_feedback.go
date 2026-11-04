// 分析反馈与观测端点(llm-analysis 票 09):POST /analyses/{id}/feedback 记录
// 「这不对」;GET /analyses/metrics 返回周检所需的反馈率与降级率。
// 反馈只写入,不自动降级条目;降级事件由分析链路写入 analysis_events。
package main

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"healthyduoduo/internal/analysis"
	"healthyduoduo/internal/httpapi"
)

const (
	feedbackReasonRequired = "reason is required"
	feedbackReasonTooLong  = "reason must be at most 500 characters"
	feedbackEntryTooLong   = "entry_id must be at most 200 characters"
	metricsRangeInvalid    = "from must be on or before to"
	metricsDateInvalid     = "date must be YYYY-MM-DD"
)

// handleCreateAnalysisFeedback POST /api/v1/analyses/{analysis_id}/feedback。
func handleCreateAnalysisFeedback(d *deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		analysisID, err := strconv.ParseInt(r.PathValue("analysis_id"), 10, 64)
		if err != nil {
			writeErrorJSON(w, http.StatusBadRequest, "analysis_id must be an integer")
			return
		}
		var body httpapi.AnalysisFeedbackRequest
		if err := decodeJSONBody(r, &body); err != nil {
			writeErrorJSON(w, http.StatusBadRequest, err.Error())
			return
		}
		reason := strings.TrimSpace(body.Reason)
		if reason == "" {
			writeErrorJSON(w, http.StatusBadRequest, feedbackReasonRequired)
			return
		}
		if len([]rune(reason)) > 500 {
			writeErrorJSON(w, http.StatusBadRequest, feedbackReasonTooLong)
			return
		}
		var entryID *string
		if body.EntryID != nil {
			trimmed := strings.TrimSpace(*body.EntryID)
			if trimmed != "" {
				if len([]rune(trimmed)) > 200 {
					writeErrorJSON(w, http.StatusBadRequest, feedbackEntryTooLong)
					return
				}
				entryID = &trimmed
			}
		}
		record, err := d.analyses.AnalysisByID(r.Context(), analysisID)
		if err != nil {
			writeErrorJSON(w, http.StatusInternalServerError, "read analysis failed")
			return
		}
		if record == nil {
			writeErrorJSON(w, http.StatusNotFound, analysisNotFoundMessage)
			return
		}
		if d.observability == nil {
			writeErrorJSON(w, http.StatusInternalServerError, "analysis feedback store is not configured")
			return
		}
		saved, err := d.observability.RecordFeedback(r.Context(), analysis.Feedback{
			AnalysisID: analysisID,
			EntryID:    entryID,
			Reason:     reason,
		})
		if err != nil {
			writeErrorJSON(w, http.StatusInternalServerError, "save analysis feedback failed")
			return
		}
		writeJSON(w, http.StatusCreated, httpapi.AnalysisFeedbackResult{
			FeedbackID: saved.FeedbackID,
			CreatedAt:  saved.CreatedAt.Format(time.RFC3339),
		})
	}
}

// handleGetAnalysisMetrics GET /api/v1/analyses/metrics?from=YYYY-MM-DD&to=YYYY-MM-DD。
func handleGetAnalysisMetrics(d *deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		now := time.Now().UTC()
		to := dayStartUTC(now)
		from := to.AddDate(0, 0, -6)
		if value := r.URL.Query().Get("from"); value != "" {
			parsed, err := time.Parse("2006-01-02", value)
			if err != nil {
				writeErrorJSON(w, http.StatusBadRequest, metricsDateInvalid)
				return
			}
			from = parsed
		}
		if value := r.URL.Query().Get("to"); value != "" {
			parsed, err := time.Parse("2006-01-02", value)
			if err != nil {
				writeErrorJSON(w, http.StatusBadRequest, metricsDateInvalid)
				return
			}
			to = parsed
		}
		if from.After(to) {
			writeErrorJSON(w, http.StatusBadRequest, metricsRangeInvalid)
			return
		}
		if d.observability == nil {
			writeErrorJSON(w, http.StatusInternalServerError, "analysis metrics store is not configured")
			return
		}
		metrics, err := d.observability.AnalysisMetrics(r.Context(), from, to)
		if err != nil {
			writeErrorJSON(w, http.StatusInternalServerError, "read analysis metrics failed")
			return
		}
		kinds := metrics.DegradationKinds
		if kinds == nil {
			kinds = map[string]int{}
		}
		writeJSON(w, http.StatusOK, httpapi.AnalysisMetrics{
			From:             metrics.From,
			To:               metrics.To,
			Analyses:         metrics.Analyses,
			Feedback:         metrics.Feedback,
			FeedbackRate:     metrics.FeedbackRate,
			Degradations:     metrics.Degradations,
			DegradationRate:  metrics.DegradationRate,
			DegradationKinds: kinds,
		})
	}
}

func dayStartUTC(value time.Time) time.Time {
	return time.Date(value.Year(), value.Month(), value.Day(), 0, 0, 0, 0, time.UTC)
}
