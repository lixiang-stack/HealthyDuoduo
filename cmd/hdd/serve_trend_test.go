package main

// 趋势端点 HTTP 测试(票 02):成员维度 + unit 门控 + reviewed 映射 + 空数组。
// 持久化以窄 fake 打底(不膨胀 fakeServeStore);SQL 排序/门控由 storage 冒烟覆盖。

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"

	"healthyduoduo/internal/httpapi"
	"healthyduoduo/internal/pipeline"
)

// fakeTrendStore 内存版趋势存储:记录入参,便于断言 handler 的路径/查询透传。
type fakeTrendStore struct {
	rows        []pipeline.TrendPointRow
	err         error
	calls       int
	gotMemberID int64
	gotName     string
	gotUnit     string
}

var _ pipeline.TrendStore = (*fakeTrendStore)(nil)

func (f *fakeTrendStore) TrendPoints(ctx context.Context, memberID int64, name, unit string) ([]pipeline.TrendPointRow, error) {
	f.calls++
	f.gotMemberID, f.gotName, f.gotUnit = memberID, name, unit
	return f.rows, f.err
}

func newTrendTestServer(t *testing.T, trends pipeline.TrendStore, members pipeline.MemberStore) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(newServeHandler(&deps{members: members, trends: trends}, ""))
	t.Cleanup(srv.Close)
	return srv
}

// getTrend 请求趋势端点:name/unit 显式编码,覆盖中文与 `%2F` 路径。
func getTrend(t *testing.T, srv *httptest.Server, memberID, name, unit string) (*http.Response, []byte) {
	t.Helper()
	target := srv.URL + "/api/v1/members/" + memberID + "/indicators/" + url.PathEscape(name) + "/trend"
	if unit != "" {
		target += "?unit=" + url.QueryEscape(unit)
	}
	resp, err := http.Get(target)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	return resp, body
}

func newTrendMember(t *testing.T) (*fakeMemberStore, int64) {
	t.Helper()
	ms := &fakeMemberStore{}
	member, err := ms.CreateMember(context.Background(), "小明", true)
	if err != nil {
		t.Fatal(err)
	}
	return ms, member.MemberID
}

func TestHandleGetTrend(t *testing.T) {
	ms, memberID := newTrendMember(t)
	rep1, rep2 := strings.Repeat("a", 64), strings.Repeat("b", 64)
	rows := []pipeline.TrendPointRow{
		{
			ReportID: rep1, ReportDate: str("2017-11-18"), Value: str("27.40"),
			Unit: str("U/L"), RefRange: str("7-40"), Flag: "normal", ReviewStatus: "unreviewed",
		},
		{
			ReportID: rep2, ReportDate: str("2021-08-02"), Value: str("835"),
			Unit: str("U/L"), RefRange: str("7-40"), Method: str("速率法"),
			Flag: "high", LowConfidence: true, ReviewStatus: "confirmed",
		},
	}
	ts := &fakeTrendStore{rows: rows}
	srv := newTrendTestServer(t, ts, ms)
	memberIDStr := strconv.FormatInt(memberID, 10)

	resp, body := getTrend(t, srv, memberIDStr, "丙氨酸转氨酶", "U/L")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", resp.StatusCode, body)
	}
	if got := resp.Header.Get("Content-Type"); got != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", got)
	}
	var got httpapi.TrendResult
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("decode trend result: %v; body=%s", err, body)
	}
	if got.MemberID != memberID || got.Name != "丙氨酸转氨酶" {
		t.Errorf("result header = (%d, %q), want (%d, 丙氨酸转氨酶)", got.MemberID, got.Name, memberID)
	}
	if len(got.Points) != 2 {
		t.Fatalf("points = %d, want 2", len(got.Points))
	}
	first := got.Points[0]
	if first.ReportID != rep1 || first.ReportDate == nil || *first.ReportDate != "2017-11-18" ||
		first.Value == nil || *first.Value != "27.40" || first.Unit == nil || *first.Unit != "U/L" ||
		first.RefRange == nil || *first.RefRange != "7-40" || first.Method != nil ||
		first.Flag != "normal" || first.LowConfidence || first.Reviewed {
		t.Errorf("first point = %+v", first)
	}
	second := got.Points[1]
	if !second.Reviewed || second.Method == nil || *second.Method != "速率法" || !second.LowConfidence {
		t.Errorf("second point = %+v, want reviewed/method/low_confidence", second)
	}
	if ts.gotMemberID != memberID || ts.gotName != "丙氨酸转氨酶" || ts.gotUnit != "U/L" {
		t.Errorf("trend store args = (%d, %q, %q), want (%d, 丙氨酸转氨酶, U/L)",
			ts.gotMemberID, ts.gotName, ts.gotUnit, memberID)
	}
}

func TestHandleGetTrendNameWithSlash(t *testing.T) {
	ms, memberID := newTrendMember(t)
	ts := &fakeTrendStore{rows: []pipeline.TrendPointRow{{
		ReportID: strings.Repeat("c", 64), Value: str("1.24"), Flag: "normal", ReviewStatus: "corrected",
	}}}
	srv := newTrendTestServer(t, ts, ms)

	resp, body := getTrend(t, srv, strconv.FormatInt(memberID, 10), "谷草/谷丙比值", "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", resp.StatusCode, body)
	}
	var got httpapi.TrendResult
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatal(err)
	}
	if got.Name != "谷草/谷丙比值" || ts.gotName != "谷草/谷丙比值" {
		t.Errorf("name = %q / store arg %q, want 谷草/谷丙比值", got.Name, ts.gotName)
	}
	if ts.gotUnit != "" {
		t.Errorf("unit = %q, want empty (absent query)", ts.gotUnit)
	}
	if got.Points[0].ReportDate != nil {
		t.Errorf("report_date = %v, want null", got.Points[0].ReportDate)
	}
	if !strings.Contains(string(body), `"report_date":null`) {
		t.Errorf("report_date null not encoded: body=%s", body)
	}
	if !got.Points[0].Reviewed {
		t.Error("corrected point should be reviewed")
	}
}

func TestHandleGetTrendEmptyPoints(t *testing.T) {
	ms, memberID := newTrendMember(t)
	srv := newTrendTestServer(t, &fakeTrendStore{}, ms)

	resp, body := getTrend(t, srv, strconv.FormatInt(memberID, 10), "丙氨酸转氨酶", "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", resp.StatusCode, body)
	}
	if !strings.Contains(string(body), `"points":[]`) {
		t.Errorf("empty points encoded as null? body=%s", body)
	}
	var got httpapi.TrendResult
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatal(err)
	}
	if got.Points == nil || len(got.Points) != 0 {
		t.Errorf("points = %#v, want non-nil empty slice", got.Points)
	}
}

func TestHandleGetTrendUnknownMember(t *testing.T) {
	ts := &fakeTrendStore{}
	srv := newTrendTestServer(t, ts, &fakeMemberStore{})

	resp, body := getTrend(t, srv, "999", "丙氨酸转氨酶", "")
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want 404; body=%s", resp.StatusCode, body)
	}
	var got httpapi.Error
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatal(err)
	}
	if got.Message != memberNotFoundMessage {
		t.Errorf("message = %q, want %q", got.Message, memberNotFoundMessage)
	}
	if ts.calls != 0 {
		t.Errorf("trend store calls = %d, want 0 (member missing)", ts.calls)
	}
}

func TestHandleGetTrendInvalidMemberID(t *testing.T) {
	ts := &fakeTrendStore{}
	srv := newTrendTestServer(t, ts, &fakeMemberStore{})

	resp, body := getTrend(t, srv, "not-a-number", "丙氨酸转氨酶", "")
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want 404; body=%s", resp.StatusCode, body)
	}
	var got httpapi.Error
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatal(err)
	}
	if got.Message != memberNotFoundMessage {
		t.Errorf("message = %q, want %q", got.Message, memberNotFoundMessage)
	}
	if ts.calls != 0 {
		t.Errorf("trend store calls = %d, want 0 (invalid member id)", ts.calls)
	}
}

func TestHandleGetTrendStoreError(t *testing.T) {
	ms, memberID := newTrendMember(t)
	srv := newTrendTestServer(t, &fakeTrendStore{err: errors.New("boom")}, ms)

	resp, body := getTrend(t, srv, strconv.FormatInt(memberID, 10), "丙氨酸转氨酶", "")
	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500; body=%s", resp.StatusCode, body)
	}
	var got httpapi.Error
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatal(err)
	}
	if got.Message != "list trend failed" {
		t.Errorf("message = %q, want list trend failed", got.Message)
	}
}
