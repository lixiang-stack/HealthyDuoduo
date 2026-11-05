// 分析端点(llm-analysis 票 04):POST /api/v1/analyses 创建事实层分析并保存快照,
// GET /api/v1/analyses/{analysis_id} 读取。窗口 / 门控 / 排除 / 稀疏扩展在
// internal/analysis 纯函数层完成;handler 只做成员 / 入口校验与契约投影。
package main

import (
	"context"
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

// parsedAnalysisRequest 是校验后的创建请求:panel 已按 entry 归一,window 已取默认值。
type parsedAnalysisRequest struct {
	body   httpapi.AnalysisRequest
	panel  string
	months int
}

// analysisInputs 是创建分析所需的成员 / 指标点 / 病历 / 生长记录快照。
type analysisInputs struct {
	member *pipeline.Member
	panel  string
	points []analysis.InputPoint
	notes  []analysis.InputNote
	growth *analysis.InputGrowth
}

// builtAnalysis 是待落库的纯计算结果;handler 之后只负责保存与解释层。
type builtAnalysis struct {
	record    analysis.Record
	request   analysis.Request
	facts     analysis.Facts
	framework analysis.Framework
}

// parseAnalysisRequest 解码并校验创建请求;返回 (status=0) 表示通过。
// status/message 直接映射既有 400 文案,保证契约不变。
func parseAnalysisRequest(r *http.Request) (*parsedAnalysisRequest, int, string) {
	var body httpapi.AnalysisRequest
	if err := decodeJSONBody(r, &body); err != nil {
		return nil, http.StatusBadRequest, err.Error()
	}
	if status, message := validateAnalysisIdentity(body); status != 0 {
		return nil, status, message
	}
	months, status, message := analysisWindow(body)
	if status != 0 {
		return nil, status, message
	}
	panel, status, message := resolveAnalysisPanel(body)
	if status != 0 {
		return nil, status, message
	}
	return &parsedAnalysisRequest{body: body, panel: panel, months: months}, 0, ""
}

func validateAnalysisIdentity(body httpapi.AnalysisRequest) (int, string) {
	if body.MemberID <= 0 {
		return http.StatusBadRequest, "member_id is required"
	}
	if body.Entry != analysis.EntryPanel && body.Entry != analysis.EntryIndicator &&
		body.Entry != analysis.EntryClinicalNote {
		return http.StatusBadRequest, invalidAnalysisEntry
	}
	return 0, ""
}

func analysisWindow(body httpapi.AnalysisRequest) (int, int, string) {
	months := analysis.DefaultWindowMonths
	if body.WindowMonths != nil {
		months = *body.WindowMonths
	}
	if months != 6 && months != 12 && months != 24 {
		return 0, http.StatusBadRequest, invalidAnalysisWindow
	}
	return months, 0, ""
}

func resolveAnalysisPanel(body httpapi.AnalysisRequest) (string, int, string) {
	panel := body.Panel
	if !pipeline.IsValidPanel(panel) {
		return "", http.StatusBadRequest, invalidPanelMessage
	}
	if body.Entry == analysis.EntryClinicalNote {
		panel = pipeline.PanelClinicalNote
		if body.ReportSHA == nil || *body.ReportSHA == "" {
			return "", http.StatusBadRequest, analysisClinicalNoteNeeded
		}
	} else if pipeline.IsNarrativePanel(panel) {
		return "", http.StatusBadRequest, narrativePanelMessage
	}
	if body.Entry == analysis.EntryIndicator && (body.IndicatorName == nil || *body.IndicatorName == "") {
		return "", http.StatusBadRequest, analysisIndicatorNeeded
	}
	return panel, 0, ""
}

// loadAnalysisInputs 读取成员与入口对应的指标点 / 病历 / 生长记录。
// 顺序保持原 handler:成员 → indicator panel 推导 → points → notes → 病历校验 → growth。
func loadAnalysisInputs(ctx context.Context, d *deps, parsed *parsedAnalysisRequest) (*analysisInputs, int, string) {
	body := parsed.body
	member, err := d.members.MemberByID(ctx, body.MemberID)
	if err != nil {
		return nil, http.StatusInternalServerError, "read member failed"
	}
	if member == nil {
		return nil, http.StatusNotFound, memberNotFoundMessage
	}

	panel := parsed.panel
	if body.Entry == analysis.EntryIndicator {
		derived, err := d.analyses.IndicatorPanel(ctx, member.MemberID, *body.IndicatorName)
		if err != nil {
			return nil, http.StatusInternalServerError, "read indicator failed"
		}
		if derived == "" {
			return nil, http.StatusNotFound, indicatorNotFoundMessage
		}
		if pipeline.IsNarrativePanel(derived) {
			return nil, http.StatusBadRequest, narrativePanelMessage
		}
		panel = derived
	}

	points, err := d.analyses.AnalysisPoints(ctx, member.MemberID, panel)
	if err != nil {
		return nil, http.StatusInternalServerError, "read analysis points failed"
	}
	notes, err := d.analyses.ClinicalNotes(ctx, member.MemberID)
	if err != nil {
		return nil, http.StatusInternalServerError, "read clinical notes failed"
	}
	if body.Entry == analysis.EntryClinicalNote && !containsNote(notes, *body.ReportSHA) {
		return nil, http.StatusNotFound, analysisReportNotFound
	}
	growth, err := d.analyses.LatestGrowth(ctx, member.MemberID)
	if err != nil {
		return nil, http.StatusInternalServerError, "read latest growth failed"
	}
	return &analysisInputs{member: member, panel: panel, points: points, notes: notes, growth: growth}, 0, ""
}

// buildAnalysisRecord 计算事实层 / 框架层 / 快照并组装落库记录。
func buildAnalysisRecord(ctx context.Context, d *deps, parsed *parsedAnalysisRequest, inputs *analysisInputs) (*builtAnalysis, int, string) {
	body := parsed.body
	req := analysis.Request{
		MemberID:        inputs.member.MemberID,
		MemberName:      inputs.member.Name,
		MemberSex:       inputs.member.Sex,
		MemberBirthDate: inputs.member.BirthDate,
		Panel:           inputs.panel,
		Entry:           body.Entry,
		IndicatorName:   pointerText(body.IndicatorName),
		ReportSHA:       pointerText(body.ReportSHA),
		WindowMonths:    parsed.months,
		Now:             time.Now(),
	}
	facts := analysis.BuildFacts(req, inputs.points, inputs.notes)
	status := analysis.StatusSuccess
	if facts.InsufficientEvidence {
		status = analysis.StatusInsufficientEvidence
	}
	// 信任门控全未通过时框架层与解释层不输出(spec §4.15 规则 3)。
	framework := analysis.Framework{Labels: []analysis.FrameworkLabel{}}
	if !facts.InsufficientEvidence {
		framework = analysis.BuildFramework(req, facts, inputs.growth)
	}
	dataVersion, err := d.analyses.DataVersion(ctx, inputs.member.MemberID)
	if err != nil {
		return nil, http.StatusInternalServerError, "read analysis data version failed"
	}
	snapshot := analysis.BuildSnapshot(req, facts)
	snapshot.DataVersion = dataVersion
	if d.explainer != nil && !facts.InsufficientEvidence {
		snapshot.EntryVersions = analysis.EntryVersions(d.explainer.Match(req, facts))
	}
	record := analysis.Record{
		MemberID:      inputs.member.MemberID,
		Panel:         inputs.panel,
		Entry:         body.Entry,
		IndicatorName: body.IndicatorName,
		WindowMonths:  parsed.months,
		Status:        status,
		Facts:         facts,
		Framework:     framework,
		Snapshot:      snapshot,
		Sources:       analysis.FrameworkSources(framework),
	}
	return &builtAnalysis{record: record, request: req, facts: facts, framework: framework}, 0, ""
}

// saveAnalysisRecord 落库 + 降级事件 + 解释层;解释层失败时返回 500 前不写响应。
func saveAnalysisRecord(ctx context.Context, d *deps, built *builtAnalysis) (*analysis.Record, int, string) {
	saved, err := d.analyses.SaveAnalysis(ctx, built.record)
	if err != nil {
		return nil, http.StatusInternalServerError, "save analysis failed"
	}
	if built.facts.InsufficientEvidence && hasTrustGateFailure(built.facts) && d.observability != nil {
		_ = d.observability.RecordAnalysisEvent(ctx, saved.AnalysisID, analysis.EventTrustGateFailed, "")
	}
	if d.explainer != nil && !built.facts.InsufficientEvidence {
		explanation, reason := d.explainer.Explain(ctx, saved.AnalysisID, built.request, built.facts)
		sources := analysis.MergeSources(analysis.FrameworkSources(built.framework), analysis.ExplanationSources(explanation))
		if err := d.analyses.UpdateAnalysisExplanation(ctx, saved.AnalysisID, explanation, reason, sources); err != nil {
			return nil, http.StatusInternalServerError, "save analysis explanation failed"
		}
		saved.Explanation = explanation
		saved.ExplanationReason = reason
		saved.Sources = sources
	}
	return saved, 0, ""
}

// writeAnalysisResult 读取解释层配额并以 201 返回;配额读取失败降级为可用状态。
func writeAnalysisResult(w http.ResponseWriter, r *http.Request, d *deps, saved *analysis.Record) {
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

// handleCreateAnalysis POST /api/v1/analyses:建事实层分析并永久保存快照。
func handleCreateAnalysis(d *deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		parsed, status, message := parseAnalysisRequest(r)
		if status != 0 {
			writeErrorJSON(w, status, message)
			return
		}
		inputs, status, message := loadAnalysisInputs(r.Context(), d, parsed)
		if status != 0 {
			writeErrorJSON(w, status, message)
			return
		}
		built, status, message := buildAnalysisRecord(r.Context(), d, parsed, inputs)
		if status != 0 {
			writeErrorJSON(w, status, message)
			return
		}
		saved, status, message := saveAnalysisRecord(r.Context(), d, built)
		if status != 0 {
			writeErrorJSON(w, status, message)
			return
		}
		writeAnalysisResult(w, r, d, saved)
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
