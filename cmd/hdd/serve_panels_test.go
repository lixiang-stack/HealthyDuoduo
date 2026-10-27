package main

// 项目浏览端点 HTTP 集成测试(panel-browse 票 03 验收):项目列表 / 详情契约形状、
// 空数组信封、未知成员 404、非法 panel 400。聚合口径由存储冒烟
// (store_panels_test.go)与 E2E 覆盖;handler 只验证投影与边界。

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"healthyduoduo/internal/httpapi"
	"healthyduoduo/internal/pipeline"
)

type fakePanelStore struct {
	summaries map[int64][]pipeline.PanelSummary
	details   map[int64]map[string]pipeline.PanelDetail
}

var _ pipeline.PanelStore = (*fakePanelStore)(nil)

func (f *fakePanelStore) PanelSummariesByMember(ctx context.Context, memberID int64) ([]pipeline.PanelSummary, error) {
	return f.summaries[memberID], nil
}

func (f *fakePanelStore) PanelItemsByMemberPanel(ctx context.Context, memberID int64, panel string) (*pipeline.PanelDetail, error) {
	if detail, ok := f.details[memberID][panel]; ok {
		return &detail, nil
	}
	return &pipeline.PanelDetail{Panel: panel, MemberID: memberID, Items: []pipeline.PanelItem{}}, nil
}

func newPanelsTestServer(t *testing.T) (*httptest.Server, *fakePanelStore) {
	t.Helper()
	ms := &fakeMemberStore{}
	ps := &fakePanelStore{
		summaries: map[int64][]pipeline.PanelSummary{},
		details:   map[int64]map[string]pipeline.PanelDetail{},
	}
	d := &deps{
		store: fakeStoreCLI{}, rec: fakeRecognizerCLI{}, obj: fakeObjectsCLI{},
		members: ms, panels: ps,
	}
	srv := httptest.NewServer(newServeHandler(d, ""))
	t.Cleanup(srv.Close)
	return srv, ps
}

func TestServePanelsList(t *testing.T) {
	srv, ps := newPanelsTestServer(t)
	member := createOne(t, srv, `{"name":"小虎","is_child":true}`)
	date := "2021-08-02"
	reportID := strings.Repeat("a", 64)
	ps.summaries[member.MemberID] = []pipeline.PanelSummary{{
		Panel:            "liver",
		ReportCount:      2,
		ItemCount:        4,
		AbnormalCount:    1,
		LatestReportDate: &date,
		LatestReportID:   &reportID,
		ReportTypes:      []string{"生化"},
	}}

	status, raw := doJSON(t, srv, http.MethodGet, "/api/v1/members/1/panels", "")
	if status != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", status, raw)
	}
	var list httpapi.PanelsList
	if err := json.Unmarshal(raw, &list); err != nil {
		t.Fatalf("decode list: %v", err)
	}
	if len(list.Panels) != 1 {
		t.Fatalf("panels = %+v, want 1", list.Panels)
	}
	got := list.Panels[0]
	if got.Panel != "liver" || got.ReportCount != 2 || got.ItemCount != 4 || got.AbnormalCount != 1 ||
		got.LatestReportDate == nil || *got.LatestReportDate != date ||
		got.LatestReportID == nil || *got.LatestReportID != reportID ||
		len(got.ReportTypes) != 1 || got.ReportTypes[0] != "生化" || got.Narrative {
		t.Errorf("panel summary = %+v", got)
	}

	// 没有可归类报告:200 + {"panels":[]}(不是 null)。
	emptyMember := createOne(t, srv, `{"name":"妈妈","is_child":false}`)
	status, raw = doJSON(t, srv, http.MethodGet,
		"/api/v1/members/"+strconv.FormatInt(emptyMember.MemberID, 10)+"/panels", "")
	if status != http.StatusOK {
		t.Fatalf("empty status = %d, want 200; body = %s", status, raw)
	}
	var body map[string]json.RawMessage
	if err := json.Unmarshal(raw, &body); err != nil {
		t.Fatal(err)
	}
	if got := string(body["panels"]); got != "[]" {
		t.Errorf("panels = %s, want []", got)
	}
}

func TestServePanelDetail(t *testing.T) {
	srv, ps := newPanelsTestServer(t)
	member := createOne(t, srv, `{"name":"小虎","is_child":true}`)
	date := "2021-08-02"
	reportID := strings.Repeat("a", 64)
	value := "835"
	unit := "U/L"
	ref := "7-40"
	ps.details[member.MemberID] = map[string]pipeline.PanelDetail{
		"liver": {
			Panel: "liver", MemberID: member.MemberID, ReportCount: 2,
			LatestReportDate: &date, LatestReportID: &reportID,
			ReportTypes: []string{"生化"},
			Items: []pipeline.PanelItem{
				{
					Name: "丙氨酸转氨酶", Unit: &unit, RefRange: &ref,
					Latest: &pipeline.TrendPointRow{
						ReportID: reportID, ReportDate: &date, Value: &value, Unit: &unit,
						RefRange: &ref, Flag: "high", ReviewStatus: pipeline.ReviewStatusCorrected,
					},
					Points: []pipeline.TrendPointRow{{
						ReportID: reportID, ReportDate: &date, Value: &value, Unit: &unit,
						RefRange: &ref, Flag: "high", ReviewStatus: pipeline.ReviewStatusCorrected,
					}},
					ReportCount: 1,
				},
				{
					Name: "前白蛋白", Unit: &unit,
					Points:                []pipeline.TrendPointRow{},
					NotTestedCount:        1,
					LatestReportNotTested: true,
				},
			},
		},
	}

	status, raw := doJSON(t, srv, http.MethodGet, "/api/v1/members/1/panels/liver", "")
	if status != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", status, raw)
	}
	var detail httpapi.PanelDetail
	if err := json.Unmarshal(raw, &detail); err != nil {
		t.Fatalf("decode detail: %v", err)
	}
	if detail.Panel != "liver" || detail.MemberID != member.MemberID || detail.ReportCount != 2 ||
		detail.LatestReportDate == nil || *detail.LatestReportDate != date ||
		detail.ReportTypes[0] != "生化" || detail.Narrative || len(detail.Items) != 2 {
		t.Fatalf("panel detail = %+v", detail)
	}
	alt := detail.Items[0]
	if alt.Latest == nil || alt.Latest.Value == nil || *alt.Latest.Value != "835" || !alt.Latest.Reviewed {
		t.Errorf("ALT latest = %+v, want reviewed high point", alt.Latest)
	}
	if len(alt.Points) != 1 || len(alt.Points[0].ReportID) != 64 {
		t.Errorf("ALT points = %+v", alt.Points)
	}
	prealbumin := detail.Items[1]
	if prealbumin.Latest != nil || prealbumin.Points == nil || len(prealbumin.Points) != 0 ||
		!prealbumin.LatestReportNotTested || prealbumin.NotTestedCount != 1 {
		t.Errorf("prealbumin = %+v", prealbumin)
	}
	if !strings.Contains(string(raw), `"points":[]`) {
		t.Errorf("empty points must serialize as [], body = %s", raw)
	}

	// 非法 panel → 400;未知成员 → 404;非数字成员 → 404。
	for _, tc := range []struct {
		path   string
		status int
	}{
		{"/api/v1/members/1/panels/bogus", http.StatusBadRequest},
		{"/api/v1/members/999/panels/liver", http.StatusNotFound},
		{"/api/v1/members/x/panels/liver", http.StatusNotFound},
	} {
		status, raw := doJSON(t, srv, http.MethodGet, tc.path, "")
		if status != tc.status {
			t.Errorf("%s status = %d, want %d; body = %s", tc.path, status, tc.status, raw)
		}
		var apiErr httpapi.Error
		if err := json.Unmarshal(raw, &apiErr); err != nil || apiErr.Message == "" {
			t.Errorf("%s error body = %s", tc.path, raw)
		}
	}
}
