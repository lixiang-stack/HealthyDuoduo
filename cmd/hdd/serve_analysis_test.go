package main

// 分析端点 HTTP 测试(llm-analysis 票 04):窗口 / 门控 / 排除 / 稀疏扩展 / 单指标扩展 /
// 病历入口 / 快照持久化 / 未知目标。事实层规则在 internal/analysis 单测覆盖,
// 这里验证 handler 的成员与入口校验、契约投影与 GET 回读。

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"healthyduoduo/internal/analysis"
	"healthyduoduo/internal/httpapi"
	"healthyduoduo/internal/pipeline"
)

type fakeAnalysisStore struct {
	points      []analysis.InputPoint
	notes       []analysis.InputNote
	panel       string
	panelErr    error
	dataVersion string
	saved       []analysis.Record
	nextID      int64
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

func newAnalysisTestServer(t *testing.T, store analysis.Store) (*httptest.Server, *fakeMemberStore) {
	t.Helper()
	ms := &fakeMemberStore{}
	d := &deps{members: ms, analyses: store}
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
		Name: "丙氨酸转氨酶测定", Code: "1500104A", Unit: "U/L", UnitDisplay: "U/L",
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
	if len(result.Sources) != 0 || result.Quota.LimitToday != analysisDailyLimitDefault ||
		result.Quota.ExplanationAvailable {
		t.Fatalf("sources/quota = %+v / %+v", result.Sources, result.Quota)
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
