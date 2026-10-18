package main

// 核对端点 HTTP 集成测试(viz-and-client 票 09 验收:确认/修正全转移、修正对
// append、状态派生单调、flag 服务端重算、幂等重试安全、对已核对报告重跑识别
// 409、404/400 边界与零写入)。同一 fakeServeStore 承载;此处定义其 ReviewItem
// 方法(镜像 storage 事务语义,见方法注释)。

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"healthyduoduo/internal/contract"
	"healthyduoduo/internal/httpapi"
	"healthyduoduo/internal/pipeline"
)

var _ pipeline.ReportStore = (*fakeServeStore)(nil)

// ReviewItem 镜像 storage 的核对事务语义:定位指标项(缺失返回 nil, nil)、应用动作
// (共享 pipeline.ApplyReviewAction——同源纯函数,SQL 版与 fake 版行为不得漂移)、
// append 修正对(仅修正)、报告状态单调派生(partial → success 当且仅当未核对低置信
// 计数归零),返回动作后的报告级状态与计数。测试单请求串行,无并发,不镜像报告行锁
// (锁由 storage 的 SQL 事务承担)。
func (f *fakeServeStore) ReviewItem(ctx context.Context, reportSha string, itemID int64, action pipeline.ReviewAction) (*pipeline.ReviewOutcome, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	r, ok := f.reports[reportSha]
	if !ok {
		return nil, nil
	}
	for i, it := range r.items {
		if it.ItemID != itemID {
			continue
		}
		next, pair := pipeline.ApplyReviewAction(it, action)
		r.items[i] = next
		if pair != nil {
			f.pairs = append(f.pairs, *pair)
		}
		if r.summary.Status == contract.StatusPartial {
			remaining := false
			for _, other := range r.items {
				if other.ReviewStatus == pipeline.ReviewStatusUnreviewed && other.LowConfidence {
					remaining = true
					break
				}
			}
			if !remaining {
				r.summary.Status = contract.StatusSuccess
			}
		}
		sum := f.summarize(r)
		return &pipeline.ReviewOutcome{Item: next, Status: sum.Status, UnreviewedLowConfidenceCount: sum.UnreviewedLowConfidenceCount}, nil
	}
	return nil, nil
}

// pairsSnapshot 修正对追加记录的快照(断言用;仅修正产生)。
func (f *fakeServeStore) pairsSnapshot() []pipeline.CorrectionPairRecord {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]pipeline.CorrectionPairRecord(nil), f.pairs...)
}

// doReview 发一次核对请求(PATCH /reports/{id}/items/{item};itemID 用 any 以覆盖
// 非法形态用例)。
func doReview(t *testing.T, srv *httptest.Server, reportID string, itemID any, body string) (int, []byte) {
	t.Helper()
	return doJSON(t, srv, http.MethodPatch, fmt.Sprintf("/api/v1/reports/%s/items/%v", reportID, itemID), body)
}

// decodeReview 断言状态码并解码 ReviewResult。
func decodeReview(t *testing.T, status int, raw []byte, wantStatus int) httpapi.ReviewResult {
	t.Helper()
	if status != wantStatus {
		t.Fatalf("review status = %d, want %d; body = %s", status, wantStatus, raw)
	}
	var got httpapi.ReviewResult
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("decode ReviewResult: %v; body = %s", err, raw)
	}
	return got
}

// uploadSample 上传样例图得 partial 报告(2 项:低置信 high 项 + 高置信 normal 项;
// 未核对低置信计数 = 1),返回报告详情。
func uploadSample(t *testing.T, srv *httptest.Server) httpapi.ReportDetail {
	t.Helper()
	createOne(t, srv, `{"name":"小虎","is_child":true}`)
	status, raw := doUpload(t, srv, []byte("fake-jpeg-review-bytes-01"), map[string]string{formMemberID: "1"})
	return decodeUpload(t, status, raw, http.StatusCreated).Report
}

// TestServeReviewConfirm 确认路径:确认只置 review_status=confirmed(四项纸面事实与
// flag 不动、不产生修正对),最后一项低置信确认后 partial → success 单向派生;
// 单响应刷新整屏(report_id/status/count/item)与详情/时间线一致;重复确认幂等。
func TestServeReviewConfirm(t *testing.T) {
	srv, st, _, _ := newReportsTestServer(t)
	report := uploadSample(t, srv)
	lowItem, other := report.Items[0], report.Items[1]
	if !lowItem.LowConfidence || lowItem.Flag != "high" {
		t.Fatalf("sample item = %+v, want 低置信 high 项(样例夹具漂移?)", lowItem)
	}

	status, raw := doReview(t, srv, report.ReportID, lowItem.ItemID, `{"action":"confirm"}`)
	got := decodeReview(t, status, raw, http.StatusOK)
	if got.ReportID != report.ReportID || got.Status != contract.StatusSuccess || got.UnreviewedLowConfidenceCount != 0 {
		t.Errorf("review header = %+v, want 报告级 success/0(最后一项低置信确认后翻转)", got)
	}
	if got.Item.ReviewStatus != pipeline.ReviewStatusConfirmed {
		t.Errorf("item review_status = %q, want confirmed", got.Item.ReviewStatus)
	}
	if got.Item.Flag != lowItem.Flag || *got.Item.Value != *lowItem.Value || *got.Item.RefRange != *lowItem.RefRange {
		t.Errorf("confirm changed facts: %+v, want 与确认前一致", got.Item)
	}
	if len(got.Item.OcrLines) != 4 || got.Item.OcrLines[2].Box != apiBox(313, 74) {
		t.Errorf("review item ocr_lines = %+v, want 与详情同投影的行链接", got.Item.OcrLines)
	}
	if got.Item.Prev != nil {
		t.Errorf("review item prev = %+v, want null(M3 填充面)", got.Item.Prev)
	}
	if pairs := st.pairsSnapshot(); len(pairs) != 0 {
		t.Errorf("confirm produced %d correction pair(s), want 0(确认不产生记录)", len(pairs))
	}

	// 详情与时间线同步:status success、计数 0、项已确认。
	status, raw = doJSON(t, srv, http.MethodGet, "/api/v1/reports/"+report.ReportID, "")
	if status != http.StatusOK {
		t.Fatalf("GET detail = %d", status)
	}
	var detail httpapi.ReportDetail
	if err := json.Unmarshal(raw, &detail); err != nil {
		t.Fatal(err)
	}
	if detail.Status != contract.StatusSuccess || detail.UnreviewedLowConfidenceCount != 0 || detail.Items[0].ReviewStatus != pipeline.ReviewStatusConfirmed {
		t.Errorf("detail after confirm = %+v", detail.ReportHeader)
	}
	status, raw = doJSON(t, srv, http.MethodGet, "/api/v1/reports?member_id=1", "")
	if status != http.StatusOK {
		t.Fatalf("GET timeline = %d", status)
	}
	var list httpapi.ReportsList
	if err := json.Unmarshal(raw, &list); err != nil {
		t.Fatal(err)
	}
	if len(list.Reports) != 1 || list.Reports[0].Status != contract.StatusSuccess || list.Reports[0].UnreviewedLowConfidenceCount != 0 {
		t.Errorf("timeline card = %+v, want 徽标与剩余数同步", list.Reports)
	}

	// 重复确认(离线重放)幂等:状态不回退、无修正对。
	status, raw = doReview(t, srv, report.ReportID, lowItem.ItemID, `{"action":"confirm"}`)
	again := decodeReview(t, status, raw, http.StatusOK)
	if again.Status != contract.StatusSuccess || again.UnreviewedLowConfidenceCount != 0 || again.Item.ReviewStatus != pipeline.ReviewStatusConfirmed {
		t.Errorf("replayed confirm = %+v, want 与首次一致", again)
	}
	if pairs := st.pairsSnapshot(); len(pairs) != 0 {
		t.Errorf("replayed confirm produced %d pair(s), want 0", len(pairs))
	}

	// 高置信项也可确认(免核对 ≠ 不可修改):报告已 success,状态恒定 0。
	status, raw = doReview(t, srv, report.ReportID, other.ItemID, `{"action":"confirm"}`)
	high := decodeReview(t, status, raw, http.StatusOK)
	if high.Status != contract.StatusSuccess || high.UnreviewedLowConfidenceCount != 0 {
		t.Errorf("confirm high-confidence item = %+v, want success 恒定", high)
	}
}

// TestServeReviewCorrect 修正路径:只覆盖传入的纸面事实、flag 按修正后 value ×
// ref_range 服务端重算、每次修正 append 一条修正对(before = 上次生效值;raw_text
// 与行号随项),决策可重复且最终生效状态 = 最后一次动作。
func TestServeReviewCorrect(t *testing.T) {
	srv, st, _, _ := newReportsTestServer(t)
	report := uploadSample(t, srv)
	item := report.Items[0]

	status, raw := doReview(t, srv, report.ReportID, item.ItemID, `{"action":"correct","value":"5.6"}`)
	got := decodeReview(t, status, raw, http.StatusOK)
	if got.Status != contract.StatusSuccess || got.UnreviewedLowConfidenceCount != 0 {
		t.Errorf("review header = %+v, want success/0", got)
	}
	if got.Item.ReviewStatus != pipeline.ReviewStatusCorrected || *got.Item.Value != "5.6" {
		t.Errorf("corrected item = %+v, want corrected/5.6", got.Item)
	}
	if got.Item.Flag != "normal" {
		t.Errorf("flag = %q, want normal(5.6 × 3-10 服务端重算)", got.Item.Flag)
	}
	if *got.Item.Unit != "%" || *got.Item.RefRange != "3-10" || got.Item.Name != "单核细胞比率" {
		t.Errorf("untouched facts drifted: %+v", got.Item)
	}
	pairs := st.pairsSnapshot()
	if len(pairs) != 1 {
		t.Fatalf("pairs = %d, want 1(修正 append 一条)", len(pairs))
	}
	p := pairs[0]
	if p.BeforeName == nil || *p.BeforeName != "单核细胞比率" || *p.BeforeValue != "10.8" || *p.BeforeUnit != "%" || *p.BeforeRefRange != "3-10" {
		t.Errorf("pair before = %+v, want 上次生效值", p)
	}
	if p.AfterName == nil || *p.AfterName != "单核细胞比率" || *p.AfterValue != "5.6" || *p.AfterUnit != "%" || *p.AfterRefRange != "3-10" {
		t.Errorf("pair after = %+v, want 本次生效值", p)
	}
	if p.RawText != item.RawText || len(p.OCRLineNumbers) != 4 || p.OCRLineNumbers[0] != 1 {
		t.Errorf("pair raw_text/line numbers = %q/%v, want 随项原样", p.RawText, p.OCRLineNumbers)
	}

	// 决策可重复:已修正项再修正,再确认亦可;每次修正 append,最终状态 = 最后一次动作。
	status, raw = doReview(t, srv, report.ReportID, item.ItemID, `{"action":"correct","value":"12.0","ref_range":"3-10"}`)
	again := decodeReview(t, status, raw, http.StatusOK)
	if *again.Item.Value != "12.0" || again.Item.Flag != "high" {
		t.Errorf("second correction = %+v, want 12.0/high", again.Item)
	}
	pairs = st.pairsSnapshot()
	if len(pairs) != 2 {
		t.Fatalf("pairs = %d, want 2(每次修正一条)", len(pairs))
	}
	if *pairs[1].BeforeValue != "5.6" || *pairs[1].AfterValue != "12.0" {
		t.Errorf("second pair = %v -> %v, want before=上次生效值 5.6", *pairs[1].BeforeValue, *pairs[1].AfterValue)
	}
	if got.Status == contract.StatusPartial || again.Status != contract.StatusSuccess {
		t.Errorf("status = %q → %q, want 恒定 success(派生单调不回退)", got.Status, again.Status)
	}

	status, raw = doReview(t, srv, report.ReportID, item.ItemID, `{"action":"confirm"}`)
	confirmed := decodeReview(t, status, raw, http.StatusOK)
	if confirmed.Item.ReviewStatus != pipeline.ReviewStatusConfirmed || *confirmed.Item.Value != "12.0" {
		t.Errorf("confirm after correct = %+v, want 已确认且生效值保持 12.0", confirmed.Item)
	}
	if len(st.pairsSnapshot()) != 2 {
		t.Errorf("confirm after correct appended a pair, want 仅修正产生")
	}
}

// TestServeReviewFlagRecompute flag 服务端重算口径(HTTP 侧):值/参考范围修正后
// flag 按修正后 value × ref_range 重建——区间上下、性别前缀、单侧比较、不可数值化。
func TestServeReviewFlagRecompute(t *testing.T) {
	cases := []struct {
		name  string
		value string
		ref   string
		want  string
	}{
		{"区间内", "100", "98-110", "normal"},
		{"超上界", "180", "0-100", "high"},
		{"低于下界", "2", "3-10", "low"},
		{"性别前缀范围", "20", "男:0-15", "high"},
		{"单侧上界", "9", "<8", "high"},
		{"单侧下界", "3", ">7.5", "low"},
		{"不可数值化值", "查", "3-10", "unknown"},
		{"不可解析参考范围", "128", "优良", "unknown"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			srv, st, _, _ := newReportsTestServer(t)
			sha := strings.Repeat("c", 64)
			st.seedReport(sha, nil, contract.Report{ReportType: "血常规", Status: contract.StatusSuccess, Items: []contract.ReportItem{
				{Name: "指标", Value: str("1"), RefRange: str("0-0"), Flag: "unknown", RawText: "指标  1"},
			}}, nil)

			body := fmt.Sprintf(`{"action":"correct","value":%q,"ref_range":%q}`, c.value, c.ref)
			status, raw := doReview(t, srv, sha, 1, body)
			got := decodeReview(t, status, raw, http.StatusOK)
			if got.Item.Flag != c.want {
				t.Errorf("flag(%s × %s) = %q, want %q", c.value, c.ref, got.Item.Flag, c.want)
			}
			if len(st.pairsSnapshot()) != 1 {
				t.Errorf("pairs = %d, want 1", len(st.pairsSnapshot()))
			}
		})
	}
}

// TestServeReviewStatusMonotonic 状态派生单调:partial 有两项未核对低置信时逐项
// 处理(计数递减、状态不动)→ 最后一项后翻 success;此后任何动作不回 partial;
// success 报告(免核对)恒定 success、计数恒 0。
func TestServeReviewStatusMonotonic(t *testing.T) {
	srv, st, _, _ := newReportsTestServer(t)
	sha := strings.Repeat("a", 64)
	st.seedReport(sha, nil, contract.Report{ReportType: "血常规", Status: contract.StatusPartial, Items: []contract.ReportItem{
		{Name: "低置信甲", Value: str("10.8"), RefRange: str("3-10"), Flag: "high", LowConfidence: true, RawText: "低置信甲  10.8"},
		{Name: "低置信乙", Value: str("300"), RefRange: str("316-354"), Flag: "low", LowConfidence: true, RawText: "低置信乙  300"},
		{Name: "高置信丙", Value: str("128"), RefRange: str("115-150"), Flag: "normal", RawText: "高置信丙  128"},
	}}, nil)

	status, raw := doReview(t, srv, sha, 1, `{"action":"confirm"}`)
	first := decodeReview(t, status, raw, http.StatusOK)
	if first.Status != contract.StatusPartial || first.UnreviewedLowConfidenceCount != 1 {
		t.Errorf("after first of two = %+v, want partial/1", first)
	}
	status, raw = doReview(t, srv, sha, 2, `{"action":"confirm"}`)
	second := decodeReview(t, status, raw, http.StatusOK)
	if second.Status != contract.StatusSuccess || second.UnreviewedLowConfidenceCount != 0 {
		t.Errorf("after last pending = %+v, want success/0", second)
	}
	// 已 success:再修正不回退(单向,永不回 partial)。
	status, raw = doReview(t, srv, sha, 1, `{"action":"correct","value":"20"}`)
	third := decodeReview(t, status, raw, http.StatusOK)
	if third.Status != contract.StatusSuccess || third.UnreviewedLowConfidenceCount != 0 {
		t.Errorf("correct after success = %+v, want success 恒定", third)
	}
	// 未核对的高置信项确认:不改状态(success 恒定)。
	status, raw = doReview(t, srv, sha, 3, `{"action":"confirm"}`)
	if got := decodeReview(t, status, raw, http.StatusOK); got.Status != contract.StatusSuccess {
		t.Errorf("confirm high-confidence on success report = %+v", got)
	}

	// success 报告含低置信项:确认不改状态、计数恒 0(免核对面)。
	successSha := strings.Repeat("b", 64)
	st.seedReport(successSha, nil, contract.Report{ReportType: "尿常规", Status: contract.StatusSuccess, Items: []contract.ReportItem{
		{Name: "尿蛋白", Value: str("10.8"), Flag: "high", LowConfidence: true, RawText: "尿蛋白  10.8"},
	}}, nil)
	successItems, err := st.ItemsWithReviewStatus(context.Background(), successSha)
	if err != nil {
		t.Fatal(err)
	}
	status, raw = doReview(t, srv, successSha, successItems[0].ItemID, `{"action":"confirm"}`)
	success := decodeReview(t, status, raw, http.StatusOK)
	if success.Status != contract.StatusSuccess || success.UnreviewedLowConfidenceCount != 0 {
		t.Errorf("success report confirm = %+v, want 恒定 success/0", success)
	}
}

// TestServeReviewRetrySafety 幂等重试安全(离线补传重放):同一 confirm 重放状态
// 不变;同一 correct 重放最终生效值不变(重放对为 no-op:before = after = 上次生效值,
// ADR-0005 决策可重复、每次修正 append)。对已核对报告重跑识别 → 409 拒绝。
func TestServeReviewRetrySafety(t *testing.T) {
	srv, st, rec, _ := newReportsTestServer(t)
	report := uploadSample(t, srv)
	item := report.Items[0]

	for i := 0; i < 2; i++ {
		status, raw := doReview(t, srv, report.ReportID, item.ItemID, `{"action":"correct","value":"5.6"}`)
		got := decodeReview(t, status, raw, http.StatusOK)
		if got.Status != contract.StatusSuccess || got.UnreviewedLowConfidenceCount != 0 || *got.Item.Value != "5.6" || got.Item.Flag != "normal" {
			t.Fatalf("replay #%d = %+v, want 稳定生效状态", i+1, got)
		}
	}
	pairs := st.pairsSnapshot()
	if len(pairs) != 2 {
		t.Fatalf("pairs = %d, want 2(每次修正 append;重放为 no-op 对)", len(pairs))
	}
	if *pairs[1].BeforeValue != "5.6" || *pairs[1].AfterValue != "5.6" {
		t.Errorf("replay pair = %v -> %v, want no-op(before=上次生效值)", *pairs[1].BeforeValue, *pairs[1].AfterValue)
	}

	// 已含核对决策:同图重传 → 409(不悄悄覆盖/重跑);识别不重跑。
	status, raw := doUpload(t, srv, []byte("fake-jpeg-review-bytes-01"), map[string]string{formMemberID: "1"})
	if status != http.StatusConflict {
		t.Fatalf("re-upload after review = %d %s, want 409", status, raw)
	}
	if rec.calls != 1 {
		t.Errorf("recognizer calls = %d, want 1(409 不得重跑)", rec.calls)
	}
}

// TestServeReviewValidation 校验与 404 矩阵:非法请求 4xx 且零写入;
// report_id 非 hex → 404(内容身份口径)、item_id 非整数 → 400、未知报告/指标项/
// 不属于该报告的指标项 → 404、failed 报告无可核对项 → 404。
func TestServeReviewValidation(t *testing.T) {
	seedOther := func(st *fakeServeStore) string {
		other := strings.Repeat("e", 64)
		st.seedReport(other, nil, contract.Report{ReportType: "血糖", Status: contract.StatusSuccess, Items: []contract.ReportItem{
			{Name: "葡萄糖", Value: str("5"), Flag: "normal", RawText: "葡萄糖  5"},
		}}, nil)
		return other
	}
	cases := []struct {
		name string
		body string
		want int
		// sha/itemID 缺省 = 样例报告与首项;案例可自定(非法形态/未知/越报告)。
		sha    func(st *fakeServeStore, sample httpapi.ReportDetail) string
		itemID any
	}{
		{name: "missing action", body: `{}`, want: http.StatusBadRequest},
		{name: "unknown action", body: `{"action":"discard"}`, want: http.StatusBadRequest},
		{name: "confirm with fields", body: `{"action":"confirm","value":"1"}`, want: http.StatusBadRequest},
		{name: "correct without fields", body: `{"action":"correct"}`, want: http.StatusBadRequest},
		{name: "correct null value", body: `{"action":"correct","value":null}`, want: http.StatusBadRequest},
		{name: "correct empty name", body: `{"action":"correct","name":""}`, want: http.StatusBadRequest},
		{name: "manual flag rejected", body: `{"action":"correct","value":"5.6","flag":"normal"}`, want: http.StatusBadRequest},
		{name: "unknown field rejected", body: `{"action":"confirm","note":"x"}`, want: http.StatusBadRequest},
		{name: "non-integer item", body: `{"action":"confirm"}`, want: http.StatusBadRequest, itemID: "abc"},
		{name: "non-hex report", body: `{"action":"confirm"}`, want: http.StatusNotFound,
			sha: func(_ *fakeServeStore, _ httpapi.ReportDetail) string { return "not-a-sha" }},
		{name: "unknown report", body: `{"action":"confirm"}`, want: http.StatusNotFound,
			sha: func(_ *fakeServeStore, _ httpapi.ReportDetail) string { return strings.Repeat("f", 64) }},
		{name: "unknown item", body: `{"action":"confirm"}`, want: http.StatusNotFound, itemID: 999},
		{name: "item of other report", body: `{"action":"confirm"}`, want: http.StatusNotFound,
			sha: func(st *fakeServeStore, _ httpapi.ReportDetail) string { return seedOther(st) }},
		{name: "failed report has no items", body: `{"action":"confirm"}`, want: http.StatusNotFound,
			sha: func(st *fakeServeStore, _ httpapi.ReportDetail) string {
				failed := strings.Repeat("d", 64)
				reason := "no metric items recognized"
				st.seedReport(failed, nil, contract.Report{ReportType: "其他专项", Status: contract.StatusFailed}, &reason)
				return failed
			}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			srv, st, _, _ := newReportsTestServer(t)
			sample := uploadSample(t, srv)
			sha := sample.ReportID
			if c.sha != nil {
				sha = c.sha(st, sample)
			}
			itemID := any(sample.Items[0].ItemID)
			if c.itemID != nil {
				itemID = c.itemID
			}
			status, raw := doReview(t, srv, sha, itemID, c.body)
			if status != c.want {
				t.Fatalf("status = %d, want %d; body = %s", status, c.want, raw)
			}
			var apiErr httpapi.Error
			if err := json.Unmarshal(raw, &apiErr); err != nil || apiErr.Message == "" {
				t.Errorf("error body = %s, want 契约 Error 形状", raw)
			}
			// 零写入:样例报告两项均未动、无修正对。
			items, err := st.ItemsWithReviewStatus(context.Background(), sample.ReportID)
			if err != nil {
				t.Fatal(err)
			}
			for _, it := range items {
				if it.ReviewStatus != pipeline.ReviewStatusUnreviewed {
					t.Errorf("rejected request wrote review_status: %+v", it)
				}
			}
			if pairs := st.pairsSnapshot(); len(pairs) != 0 {
				t.Errorf("rejected request appended %d pair(s)", len(pairs))
			}
		})
	}
}
