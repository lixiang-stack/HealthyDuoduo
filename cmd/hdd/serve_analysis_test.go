package main

// 分析端点 HTTP 测试(llm-analysis 票 04):窗口 / 门控 / 排除 / 稀疏扩展 / 单指标扩展 /
// 病历入口 / 快照持久化 / 未知目标。事实层规则在 internal/analysis 单测覆盖,
// 这里验证 handler 的成员与入口校验、契约投影与 GET 回读。

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"healthyduoduo/internal/analysis"
	"healthyduoduo/internal/contract"
	"healthyduoduo/internal/httpapi"
	"healthyduoduo/internal/pipeline"
)

type fakeAnalysisStore struct {
	points      []analysis.InputPoint
	notes       []analysis.InputNote
	growth      *analysis.InputGrowth
	panel       string
	panelErr    error
	dataVersion string
	saved       []analysis.Record
	nextID      int64
	usedToday   int
	dailyLimit  int
	quotaErr    error
	events      []string
	feedback    []analysis.Feedback
	metrics     analysis.Metrics
}

var _ analysis.Store = (*fakeAnalysisStore)(nil)

func (f *fakeAnalysisStore) IndicatorPanel(ctx context.Context, memberID int64, name string) (string, error) {
	return f.panel, f.panelErr
}

func (f *fakeAnalysisStore) AnalysisPoints(ctx context.Context, memberID int64, panel string) ([]analysis.InputPoint, error) {
	return append([]analysis.InputPoint(nil), f.points...), nil
}

func (f *fakeAnalysisStore) ClinicalNotes(ctx context.Context, memberID int64) ([]analysis.InputNote, error) {
	return append([]analysis.InputNote(nil), f.notes...), nil
}

func (f *fakeAnalysisStore) SaveAnalysis(ctx context.Context, record analysis.Record) (*analysis.Record, error) {
	f.nextID++
	record.AnalysisID = f.nextID
	record.CreatedAt = time.Date(2026, 11, 3, 12, 0, 0, 0, time.UTC)
	f.saved = append(f.saved, record)
	return &record, nil
}

func (f *fakeAnalysisStore) DataVersion(ctx context.Context, memberID int64) (string, error) {
	if f.dataVersion == "" {
		return "v1", nil
	}
	return f.dataVersion, nil
}

func (f *fakeAnalysisStore) AnalysisByID(ctx context.Context, id int64) (*analysis.Record, error) {
	for _, record := range f.saved {
		if record.AnalysisID == id {
			copy := record
			return &copy, nil
		}
	}
	return nil, nil
}

func (f *fakeAnalysisStore) LatestGrowth(ctx context.Context, memberID int64) (*analysis.InputGrowth, error) {
	return f.growth, nil
}

func (f *fakeAnalysisStore) UpdateAnalysisExplanation(ctx context.Context, id int64, explanation *analysis.Explanation, reason string, sources []analysis.Source) error {
	for index := range f.saved {
		if f.saved[index].AnalysisID != id {
			continue
		}
		f.saved[index].Explanation = explanation
		f.saved[index].ExplanationReason = reason
		f.saved[index].Sources = sources
		return nil
	}
	return nil
}

func (f *fakeAnalysisStore) ConsumeLLMQuota(ctx context.Context, analysisID int64, limit int) (int, bool, error) {
	if f.dailyLimit > 0 {
		limit = f.dailyLimit
	}
	if f.usedToday >= limit {
		return f.usedToday, false, nil
	}
	f.usedToday++
	return f.usedToday, true, nil
}

func (f *fakeAnalysisStore) LLMUsedToday(ctx context.Context) (int, error) {
	if f.quotaErr != nil {
		return 0, f.quotaErr
	}
	return f.usedToday, nil
}

func (f *fakeAnalysisStore) FindExplanation(ctx context.Context, memberID int64, key string) (*analysis.Explanation, error) {
	return nil, nil
}

func (f *fakeAnalysisStore) SaveExplanation(ctx context.Context, memberID int64, key string, explanation *analysis.Explanation) error {
	return nil
}

func (f *fakeAnalysisStore) RecordAnalysisEvent(ctx context.Context, analysisID int64, kind, detail string) error {
	f.events = append(f.events, kind)
	return nil
}

func (f *fakeAnalysisStore) RecordFeedback(ctx context.Context, feedback analysis.Feedback) (*analysis.Feedback, error) {
	feedback.FeedbackID = int64(len(f.feedback) + 1)
	feedback.CreatedAt = time.Date(2026, 11, 3, 12, 0, 0, 0, time.UTC)
	f.feedback = append(f.feedback, feedback)
	return &feedback, nil
}

func (f *fakeAnalysisStore) AnalysisMetrics(ctx context.Context, from, to time.Time) (analysis.Metrics, error) {
	return f.metrics, nil
}

type fakeAnalysisLLM struct {
	response string
}

func (f fakeAnalysisLLM) Complete(ctx context.Context, req analysis.LLMRequest) (analysis.LLMResponse, error) {
	return analysis.LLMResponse{Content: f.response}, nil
}

func newAnalysisTestServer(t *testing.T, store analysis.Store) (*httptest.Server, *fakeMemberStore) {
	t.Helper()
	return newAnalysisTestServerWithExplainer(t, store, nil)
}

func newAnalysisTestServerWithExplainer(t *testing.T, store analysis.Store, explainer *analysis.Explainer) (*httptest.Server, *fakeMemberStore) {
	t.Helper()
	ms := &fakeMemberStore{}
	d := &deps{members: ms, analyses: store, explainer: explainer}
	if quota, ok := store.(analysis.QuotaStore); ok {
		d.quota = quota
	}
	if observability, ok := store.(analysis.ObservabilityStore); ok {
		d.observability = observability
	}
	srv := httptest.NewServer(newServeHandler(d))
	t.Cleanup(srv.Close)
	return srv, ms
}

func createAnalysisMember(t *testing.T, ms *fakeMemberStore) int64 {
	t.Helper()
	member, err := ms.CreateMember(context.Background(), pipeline.MemberProfile{
		Name: "小虎", Sex: pipeline.SexMale, BirthDate: "2020-05-20",
	})
	if err != nil {
		t.Fatal(err)
	}
	return member.MemberID
}

func analysisPoint(itemID int64, reportID, date, value, flag string) analysis.InputPoint {
	return analysis.InputPoint{
		MemberID: 1, ReportID: reportID, ReportDate: date, ReportType: "生化",
		ReportStatus: "success", Panel: "liver", ItemID: itemID,
		Name: "丙氨酸转氨酶测定", Code: "1500104A", Analyte: contract.AnalyteALT,
		Unit: "U/L", UnitDisplay: "U/L",
		UnitStatus: "verified", Value: value, RefRange: "7-40", Flag: flag,
		ReviewStatus: "unreviewed",
	}
}

func TestServeAnalysisPanelFacts(t *testing.T) {
	store := &fakeAnalysisStore{
		points: []analysis.InputPoint{
			analysisPoint(1, "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "2026-10-01", "83.5", "high"),
			analysisPoint(2, "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", "2026-06-01", "27.4", "normal"),
		},
		notes: []analysis.InputNote{{
			MemberID: 1, ReportID: "cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc",
			ReportDate: "2026-09-20", ReportType: "clinical_note",
			Sections: []analysis.InputSection{
				{Kind: "chief_complaint", Text: "最近不爱吃饭"},
				{Kind: "diagnosis", Text: "缺铁性贫血"},
				{Kind: "treatment", Text: "补铁治疗"},
			},
		}},
	}
	srv, ms := newAnalysisTestServer(t, store)
	memberID := createAnalysisMember(t, ms)
	body := `{"member_id":` + strconv.FormatInt(memberID, 10) + `,"panel":"liver","entry":"panel"}`

	status, raw := doJSON(t, srv, http.MethodPost, "/api/v1/analyses", body)
	if status != http.StatusCreated {
		t.Fatalf("POST analyses status = %d, want 201; body=%s", status, raw)
	}
	var result httpapi.AnalysisResult
	if err := json.Unmarshal(raw, &result); err != nil {
		t.Fatal(err)
	}
	facts := result.Layers.Facts
	if result.AnalysisID == 0 || result.Status != analysis.StatusSuccess || result.Stale {
		t.Fatalf("result header = %+v", result)
	}
	if result.CreatedAt != "2026-11-03T12:00:00Z" {
		t.Fatalf("created_at = %q, want fake store timestamp", result.CreatedAt)
	}
	if facts.Panel != "liver" || facts.Entry != analysis.EntryPanel || facts.Window.Months != 12 {
		t.Fatalf("facts header = %+v", facts)
	}
	if _, err := time.Parse("2006-01-02", facts.Window.To); err != nil {
		t.Fatalf("window to = %q, want YYYY-MM-DD", facts.Window.To)
	}
	if len(facts.Deviations) != 1 || facts.Deviations[0].Flag != "high" {
		t.Fatalf("deviations = %+v", facts.Deviations)
	}
	if len(facts.History) != 1 || facts.History[0].Change != analysis.ChangeUp {
		t.Fatalf("history = %+v", facts.History)
	}
	if len(facts.Notes) != 1 || len(facts.Notes[0].Sections) != 2 ||
		facts.Notes[0].Sections[0].Kind != "diagnosis" {
		t.Fatalf("notes = %+v, want diagnosis/treatment only", facts.Notes)
	}
	if len(result.Sources) == 0 || result.Quota.LimitToday != analysisDailyLimitDefault ||
		!result.Quota.ExplanationAvailable {
		t.Fatalf("sources/quota = %+v / %+v", result.Sources, result.Quota)
	}
	if result.Layers.Framework == nil || len(result.Layers.Framework.Labels) == 0 {
		t.Fatalf("framework = %+v, want at least one label", result.Layers.Framework)
	}
	if len(result.Snapshot.Points) != 2 || result.Snapshot.RuleVersion != analysis.RuleVersion ||
		result.Snapshot.DataVersion != "v1" {
		t.Fatalf("snapshot = %+v", result.Snapshot)
	}

	// GET 回读同一份快照。
	status, raw = doJSON(t, srv, http.MethodGet,
		"/api/v1/analyses/"+strconv.FormatInt(result.AnalysisID, 10), "")
	if status != http.StatusOK {
		t.Fatalf("GET analysis status = %d, want 200; body=%s", status, raw)
	}
	var fetched httpapi.AnalysisResult
	if err := json.Unmarshal(raw, &fetched); err != nil {
		t.Fatal(err)
	}
	if fetched.AnalysisID != result.AnalysisID || fetched.Layers.Facts.Panel != "liver" || fetched.Stale {
		t.Fatalf("fetched analysis = %+v", fetched)
	}
	// 数据指纹变化后,旧分析仍可读但标 stale。
	store.dataVersion = "v2"
	status, raw = doJSON(t, srv, http.MethodGet,
		"/api/v1/analyses/"+strconv.FormatInt(result.AnalysisID, 10), "")
	if status != http.StatusOK {
		t.Fatalf("GET stale analysis status = %d; body=%s", status, raw)
	}
	var stale httpapi.AnalysisResult
	if err := json.Unmarshal(raw, &stale); err != nil {
		t.Fatal(err)
	}
	if !stale.Stale {
		t.Fatal("analysis not marked stale after data version change")
	}
}

func TestServeAnalysisUsesKnowledgeEntryAndFreezesVersion(t *testing.T) {
	reportID := strings.Repeat("a", 64)
	alt := analysisPoint(1, reportID, "2026-10-01", "83.5", "high")
	alp := analysisPoint(2, reportID, "2026-10-01", "80", "normal")
	alp.Name, alp.Code, alp.Analyte = "碱性磷酸酶测定", "1500404A", contract.AnalyteALP
	alp.RefRange = "40-100"
	bilirubin := analysisPoint(3, reportID, "2026-10-01", "15", "normal")
	bilirubin.Name, bilirubin.Code, bilirubin.Analyte = "总胆红素测定", "1700504A", contract.AnalyteBilirubinTotal
	bilirubin.RefRange, bilirubin.Unit, bilirubin.UnitDisplay = "0-26", "umol/L", "μmol/L"
	store := &fakeAnalysisStore{points: []analysis.InputPoint{alt, alp, bilirubin}}
	matcher, err := analysis.NewKnowledgeMatcher()
	if err != nil {
		t.Fatal(err)
	}
	explainer := &analysis.Explainer{
		Client:  fakeAnalysisLLM{response: `{"sections":[{"entry_id":"liver-hepatocellular-mild","evidence_ids":["e1"],"bridge_text":"先看本次转氨酶与碱性磷酸酶的关系。"}]}`},
		Matcher: matcher, Quota: store, Events: store,
		DailyLimit: 20, InputTokenLimit: 8000, OutputTokenLimit: 1500, RecordEntryMiss: true,
	}
	srv, ms := newAnalysisTestServerWithExplainer(t, store, explainer)
	memberID := createAnalysisMember(t, ms)
	body := `{"member_id":` + strconv.FormatInt(memberID, 10) + `,"panel":"liver","entry":"panel"}`

	status, raw := doJSON(t, srv, http.MethodPost, "/api/v1/analyses", body)
	if status != http.StatusCreated {
		t.Fatalf("POST analyses status = %d, want 201; body=%s", status, raw)
	}
	var result httpapi.AnalysisResult
	if err := json.Unmarshal(raw, &result); err != nil {
		t.Fatal(err)
	}
	if result.Layers.Explanation == nil || len(result.Layers.Explanation.Sections) != 1 {
		t.Fatalf("explanation = %+v", result.Layers.Explanation)
	}
	section := result.Layers.Explanation.Sections[0]
	if section.EntryID != "liver-hepatocellular-mild" || section.Explanation == "" ||
		len(section.CheckDirections) != 1 || section.TrackItems[0] != "ALT" {
		t.Fatalf("section = %+v", section)
	}
	if result.Snapshot.EntryVersions["liver-hepatocellular-mild"] != "1" {
		t.Fatalf("entry versions = %v, want frozen entry version", result.Snapshot.EntryVersions)
	}
}

func TestServeAnalysisSparseAndExclusions(t *testing.T) {
	store := &fakeAnalysisStore{
		points: []analysis.InputPoint{
			analysisPoint(1, "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "2025-01-01", "20", "normal"),
			analysisPoint(2, "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", "2025-06-01", "21", "normal"),
			analysisPoint(3, "cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc", "2026-10-01", "22", "normal"),
		},
	}
	store.points[1].UnitStatus = "unverified"
	store.points[2].LowConfidence = true
	srv, ms := newAnalysisTestServer(t, store)
	memberID := createAnalysisMember(t, ms)
	body := `{"member_id":` + strconv.FormatInt(memberID, 10) + `,"panel":"liver","entry":"panel"}`

	status, raw := doJSON(t, srv, http.MethodPost, "/api/v1/analyses", body)
	if status != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body=%s", status, raw)
	}
	var result httpapi.AnalysisResult
	if err := json.Unmarshal(raw, &result); err != nil {
		t.Fatal(err)
	}
	facts := result.Layers.Facts
	to, err := time.Parse("2006-01-02", facts.Window.To)
	if err != nil {
		t.Fatalf("window to = %q: %v", facts.Window.To, err)
	}
	if !facts.Window.Sparse || facts.Window.From != to.AddDate(0, -12, 0).Format("2006-01-02") {
		t.Fatalf("window = %+v, want requested 12 months + sparse flag", facts.Window)
	}
	reasons := map[string]bool{}
	for _, exclusion := range facts.Excluded {
		reasons[exclusion.Reason] = true
	}
	for _, want := range []string{analysis.ReasonUnverifiedUnit, analysis.ReasonLowConfidence} {
		if !reasons[want] {
			t.Errorf("exclusions = %+v, missing %s", facts.Excluded, want)
		}
	}
	if len(facts.History) != 1 || facts.History[0].PointCount != 1 || !facts.History[0].Sparse {
		t.Fatalf("history = %+v, want one sparse point", facts.History)
	}
}

func TestServeAnalysisIndicatorExpandsToPanel(t *testing.T) {
	store := &fakeAnalysisStore{
		panel: "liver",
		points: []analysis.InputPoint{
			analysisPoint(1, "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "2026-10-01", "27.4", "normal"),
		},
	}
	srv, ms := newAnalysisTestServer(t, store)
	memberID := createAnalysisMember(t, ms)
	body := `{"member_id":` + strconv.FormatInt(memberID, 10) +
		`,"panel":"liver","entry":"indicator","indicator_name":"丙氨酸转氨酶"}`

	status, raw := doJSON(t, srv, http.MethodPost, "/api/v1/analyses", body)
	if status != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body=%s", status, raw)
	}
	var result httpapi.AnalysisResult
	if err := json.Unmarshal(raw, &result); err != nil {
		t.Fatal(err)
	}
	facts := result.Layers.Facts
	if !facts.ExpandedFromIndicator || facts.IndicatorName == nil ||
		*facts.IndicatorName != "丙氨酸转氨酶" || facts.Panel != "liver" {
		t.Fatalf("facts = %+v, want expanded indicator", facts)
	}
}

func TestServeAnalysisClinicalNoteEntry(t *testing.T) {
	noteID := "cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"
	store := &fakeAnalysisStore{
		notes: []analysis.InputNote{{
			MemberID: 1, ReportID: noteID, ReportDate: "2026-09-20", ReportType: "clinical_note",
			Sections: []analysis.InputSection{{Kind: "note", Text: "整页原文"}},
		}},
	}
	srv, ms := newAnalysisTestServer(t, store)
	memberID := createAnalysisMember(t, ms)
	body := `{"member_id":` + strconv.FormatInt(memberID, 10) +
		`,"panel":"clinical_note","entry":"clinical_note","report_sha":"` + noteID + `"}`

	status, raw := doJSON(t, srv, http.MethodPost, "/api/v1/analyses", body)
	if status != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body=%s", status, raw)
	}
	var result httpapi.AnalysisResult
	if err := json.Unmarshal(raw, &result); err != nil {
		t.Fatal(err)
	}
	facts := result.Layers.Facts
	if facts.Panel != "clinical_note" || len(facts.Notes) != 1 ||
		facts.Notes[0].Sections[0].Kind != "note" || facts.InsufficientEvidence {
		t.Fatalf("facts = %+v, want clinical note fallback", facts)
	}
}

func TestServeAnalysisValidation(t *testing.T) {
	store := &fakeAnalysisStore{panel: ""}
	srv, ms := newAnalysisTestServer(t, store)
	memberID := createAnalysisMember(t, ms)
	id := strconv.FormatInt(memberID, 10)

	cases := []struct {
		name   string
		body   string
		status int
	}{
		{"missing member", `{"panel":"liver","entry":"panel"}`, http.StatusBadRequest},
		{"unknown member", `{"member_id":999,"panel":"liver","entry":"panel"}`, http.StatusNotFound},
		{"invalid entry", `{"member_id":` + id + `,"panel":"liver","entry":"report"}`, http.StatusBadRequest},
		{"invalid panel", `{"member_id":` + id + `,"panel":"bogus","entry":"panel"}`, http.StatusBadRequest},
		{"narrative panel", `{"member_id":` + id + `,"panel":"imaging","entry":"panel"}`, http.StatusBadRequest},
		{"invalid window", `{"member_id":` + id + `,"panel":"liver","entry":"panel","window_months":3}`, http.StatusBadRequest},
		{"indicator without name", `{"member_id":` + id + `,"panel":"liver","entry":"indicator"}`, http.StatusBadRequest},
		{"indicator not found", `{"member_id":` + id + `,"panel":"liver","entry":"indicator","indicator_name":"不存在"}`, http.StatusNotFound},
		{"note without report", `{"member_id":` + id + `,"panel":"clinical_note","entry":"clinical_note"}`, http.StatusBadRequest},
		{"note invalid panel", `{"member_id":` + id + `,"panel":"bogus","entry":"clinical_note","report_sha":"dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd"}`, http.StatusBadRequest},
		{"note not found", `{"member_id":` + id + `,"panel":"clinical_note","entry":"clinical_note","report_sha":"dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd"}`, http.StatusNotFound},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			status, raw := doJSON(t, srv, http.MethodPost, "/api/v1/analyses", c.body)
			if status != c.status {
				t.Fatalf("status = %d, want %d; body=%s", status, c.status, raw)
			}
			var apiErr httpapi.Error
			if err := json.Unmarshal(raw, &apiErr); err != nil || apiErr.Message == "" {
				t.Fatalf("error body = %s", raw)
			}
		})
	}
}

func TestServeAnalysisGetNotFound(t *testing.T) {
	srv, _ := newAnalysisTestServer(t, &fakeAnalysisStore{})
	status, raw := doJSON(t, srv, http.MethodGet, "/api/v1/analyses/999", "")
	if status != http.StatusNotFound {
		t.Fatalf("status = %d, want 404; body=%s", status, raw)
	}
	var apiErr httpapi.Error
	if err := json.Unmarshal(raw, &apiErr); err != nil || apiErr.Message != analysisNotFoundMessage {
		t.Fatalf("error = %s", raw)
	}
	status, _ = doJSON(t, srv, http.MethodGet, "/api/v1/analyses/not-a-number", "")
	if status != http.StatusBadRequest {
		t.Fatalf("invalid id status = %d, want 400", status)
	}
}

func TestServeAnalysisQuotaExhaustedStillReturnsFacts(t *testing.T) {
	store := &fakeAnalysisStore{
		points: []analysis.InputPoint{
			analysisPoint(1, "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "2026-10-01", "83.5", "high"),
		},
		usedToday:  20,
		dailyLimit: 20,
	}
	srv, ms := newAnalysisTestServer(t, store)
	memberID := createAnalysisMember(t, ms)
	body := `{"member_id":` + strconv.FormatInt(memberID, 10) + `,"panel":"liver","entry":"panel"}`

	status, raw := doJSON(t, srv, http.MethodPost, "/api/v1/analyses", body)
	if status != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body=%s", status, raw)
	}
	var result httpapi.AnalysisResult
	if err := json.Unmarshal(raw, &result); err != nil {
		t.Fatal(err)
	}
	if result.Layers.Facts.InsufficientEvidence {
		t.Fatal("quota exhaustion must not degrade the facts layer")
	}
	if result.Quota.UsedToday != 20 || result.Quota.LimitToday != 20 ||
		result.Quota.ExplanationAvailable {
		t.Fatalf("quota = %+v, want exhausted", result.Quota)
	}
}

func TestServeAnalysisQuotaStatus(t *testing.T) {
	store := &fakeAnalysisStore{usedToday: 3, dailyLimit: 20}
	srv, _ := newAnalysisTestServer(t, store)

	status, raw := doJSON(t, srv, http.MethodGet, "/api/v1/analyses/quota", "")
	if status != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", status, raw)
	}
	var quota httpapi.AnalysisQuota
	if err := json.Unmarshal(raw, &quota); err != nil {
		t.Fatal(err)
	}
	if quota.UsedToday != 3 || quota.LimitToday != 20 || !quota.ExplanationAvailable {
		t.Fatalf("quota = %+v", quota)
	}

	store.usedToday = 20
	status, raw = doJSON(t, srv, http.MethodGet, "/api/v1/analyses/quota", "")
	if status != http.StatusOK {
		t.Fatalf("exhausted status = %d, want 200; body=%s", status, raw)
	}
	if err := json.Unmarshal(raw, &quota); err != nil {
		t.Fatal(err)
	}
	if quota.UsedToday != 20 || quota.ExplanationAvailable {
		t.Fatalf("exhausted quota = %+v", quota)
	}
}

func TestServeAnalysisQuotaReadFailureStillReturnsFacts(t *testing.T) {
	store := &fakeAnalysisStore{
		points: []analysis.InputPoint{
			analysisPoint(1, "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "2026-10-01", "83.5", "high"),
		},
		quotaErr: errors.New("db down"),
	}
	srv, ms := newAnalysisTestServer(t, store)
	memberID := createAnalysisMember(t, ms)
	body := `{"member_id":` + strconv.FormatInt(memberID, 10) + `,"panel":"liver","entry":"panel"}`

	status, raw := doJSON(t, srv, http.MethodPost, "/api/v1/analyses", body)
	if status != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body=%s", status, raw)
	}
	var result httpapi.AnalysisResult
	if err := json.Unmarshal(raw, &result); err != nil {
		t.Fatal(err)
	}
	if result.Layers.Facts.InsufficientEvidence {
		t.Fatal("quota read failure must not degrade the facts layer")
	}
	if result.Quota.ExplanationReason == nil || *result.Quota.ExplanationReason != "quota_unavailable" {
		t.Fatalf("quota = %+v, want quota_unavailable", result.Quota)
	}
}

func TestServeAnalysisTrustGateEvent(t *testing.T) {
	empty := &fakeAnalysisStore{}
	srv, ms := newAnalysisTestServer(t, empty)
	memberID := createAnalysisMember(t, ms)
	body := `{"member_id":` + strconv.FormatInt(memberID, 10) + `,"panel":"liver","entry":"panel"}`
	if status, raw := doJSON(t, srv, http.MethodPost, "/api/v1/analyses", body); status != http.StatusCreated {
		t.Fatalf("empty status = %d, want 201; body=%s", status, raw)
	}
	if len(empty.events) != 0 {
		t.Fatalf("empty member events = %v, want none", empty.events)
	}

	sha := strings.Repeat("a", 64)
	low := analysisPoint(1, sha, "2026-10-01", "83.5", "high")
	low.LowConfidence = true
	store := &fakeAnalysisStore{points: []analysis.InputPoint{low}}
	srv, ms = newAnalysisTestServer(t, store)
	memberID = createAnalysisMember(t, ms)
	body = `{"member_id":` + strconv.FormatInt(memberID, 10) + `,"panel":"liver","entry":"panel"}`
	status, raw := doJSON(t, srv, http.MethodPost, "/api/v1/analyses", body)
	if status != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body=%s", status, raw)
	}
	if len(store.events) != 1 || store.events[0] != analysis.EventTrustGateFailed {
		t.Fatalf("events = %v, want trust_gate_failed", store.events)
	}
	var result httpapi.AnalysisResult
	if err := json.Unmarshal(raw, &result); err != nil {
		t.Fatal(err)
	}
	if result.Layers.Framework == nil || len(result.Layers.Framework.Labels) != 0 {
		t.Fatalf("framework = %+v, want empty when trust gate fails", result.Layers.Framework)
	}
}

func TestServeAnalysisFeedback(t *testing.T) {
	store := &fakeAnalysisStore{saved: []analysis.Record{{AnalysisID: 5, MemberID: 1}}}
	srv, _ := newAnalysisTestServer(t, store)
	entryID := "liver-hepatocellular-mild"
	body := `{"entry_id":"` + entryID + `","reason":"解释和数值对不上"}`

	status, raw := doJSON(t, srv, http.MethodPost, "/api/v1/analyses/5/feedback", body)
	if status != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body=%s", status, raw)
	}
	var result httpapi.AnalysisFeedbackResult
	if err := json.Unmarshal(raw, &result); err != nil {
		t.Fatal(err)
	}
	if result.FeedbackID != 1 || result.CreatedAt == "" {
		t.Fatalf("feedback result = %+v", result)
	}
	if len(store.feedback) != 1 || store.feedback[0].AnalysisID != 5 ||
		store.feedback[0].EntryID == nil || *store.feedback[0].EntryID != entryID ||
		store.feedback[0].Reason != "解释和数值对不上" {
		t.Fatalf("stored feedback = %+v", store.feedback)
	}
}

func TestServeAnalysisFeedbackValidation(t *testing.T) {
	store := &fakeAnalysisStore{saved: []analysis.Record{{AnalysisID: 5, MemberID: 1}}}
	srv, _ := newAnalysisTestServer(t, store)
	cases := []struct {
		name   string
		path   string
		body   string
		status int
	}{
		{"missing analysis", "/api/v1/analyses/999/feedback", `{"reason":"x"}`, http.StatusNotFound},
		{"empty reason", "/api/v1/analyses/5/feedback", `{"reason":"   "}`, http.StatusBadRequest},
		{"too long reason", "/api/v1/analyses/5/feedback", `{"reason":"` + strings.Repeat("长", 501) + `"}`, http.StatusBadRequest},
		{"unknown field", "/api/v1/analyses/5/feedback", `{"reason":"x","bogus":1}`, http.StatusBadRequest},
		{"invalid analysis id", "/api/v1/analyses/nope/feedback", `{"reason":"x"}`, http.StatusBadRequest},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			status, raw := doJSON(t, srv, http.MethodPost, c.path, c.body)
			if status != c.status {
				t.Fatalf("status = %d, want %d; body=%s", status, c.status, raw)
			}
		})
	}
}

func TestServeAnalysisMetrics(t *testing.T) {
	store := &fakeAnalysisStore{metrics: analysis.Metrics{
		From: "2026-10-28", To: "2026-11-03", Analyses: 10, Feedback: 2, FeedbackRate: 0.2,
		Degradations: 3, DegradationRate: 0.3,
		DegradationKinds: map[string]int{"llm_timeout": 2, "quota_exhausted": 1},
	}}
	srv, _ := newAnalysisTestServer(t, store)
	status, raw := doJSON(t, srv, http.MethodGet,
		"/api/v1/analyses/metrics?from=2026-10-28&to=2026-11-03", "")
	if status != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", status, raw)
	}
	var result httpapi.AnalysisMetrics
	if err := json.Unmarshal(raw, &result); err != nil {
		t.Fatal(err)
	}
	if result.Analyses != 10 || result.Feedback != 2 || result.Degradations != 3 ||
		result.DegradationKinds["llm_timeout"] != 2 {
		t.Fatalf("metrics = %+v", result)
	}
	for _, path := range []string{
		"/api/v1/analyses/metrics?from=2026-11-03&to=2026-10-28",
		"/api/v1/analyses/metrics?from=nope",
	} {
		status, raw = doJSON(t, srv, http.MethodGet, path, "")
		if status != http.StatusBadRequest {
			t.Fatalf("%s status = %d, want 400; body=%s", path, status, raw)
		}
	}
}
