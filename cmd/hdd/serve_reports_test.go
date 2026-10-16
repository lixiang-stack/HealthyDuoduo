package main

// 报告端点 HTTP 集成测试(viz-and-client 票 06 验收:同步语义、sha256 幂等、
// 失败留痕、成员过滤、409/override 分岔)。httptest 起 handler 栈,持久化以内存
// fake 打底:同一份 fake 同时实现 pipeline.Store 与 pipeline.ReportStore——
// ingest 写入与 serve 回读共享状态,行为对齐 storage(WriteReport 全量替换
// items、review_status 随重跑回 unreviewed、item_id 重发;归属只经 SetReportMember 写)。
// 服务端 SQL(排序/聚合)由 storage 冒烟与共享栈 e2e 覆盖,此处锁 handler 契约。

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"healthyduoduo/internal/contract"
	"healthyduoduo/internal/httpapi"
	"healthyduoduo/internal/pipeline"
)

// fakeServeStore 内存版报告存储(见文件头注释;不追求并发语义,测试单请求串行)。
type fakeServeStore struct {
	mu      sync.Mutex
	nextOCR int64
	nextID  int64 // report_items.id 的 BIGSERIAL 镜像:重发新句柄、不重用
	images  map[string]bool
	ocr     map[string][][]byte
	order   []string // 报告插入序(时间线迭代确定化)
	reports map[string]*fakeServeReport
}

type fakeServeReport struct {
	summary pipeline.ReportSummary
	items   []pipeline.ReportItemRow
}

var (
	_ pipeline.Store       = (*fakeServeStore)(nil)
	_ pipeline.ReportStore = (*fakeServeStore)(nil)
)

func newFakeServeStore() *fakeServeStore {
	return &fakeServeStore{images: map[string]bool{}, ocr: map[string][][]byte{}, reports: map[string]*fakeServeReport{}}
}

func (f *fakeServeStore) ImageBySha256(ctx context.Context, sha string) (*pipeline.Image, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if !f.images[sha] {
		return nil, nil
	}
	return &pipeline.Image{Sha256: sha}, nil
}

func (f *fakeServeStore) CreateImage(ctx context.Context, sha, filename string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.images[sha] = true
	return nil
}

func (f *fakeServeStore) AppendOCRResult(ctx context.Context, imageSha string, raw []byte) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.nextOCR++
	f.ocr[imageSha] = append(f.ocr[imageSha], raw)
	return f.nextOCR, nil
}

func (f *fakeServeStore) ReportByImage(ctx context.Context, imageSha string) (*pipeline.ReportRow, bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	r, ok := f.reports[imageSha]
	if !ok {
		return nil, false, nil
	}
	return &pipeline.ReportRow{
		Sha256: r.summary.Sha256, OCRResultID: 1, ReportType: r.summary.ReportType,
		ReportDate: r.summary.ReportDate, Status: r.summary.Status, ItemsCount: int64(len(r.items)),
	}, true, nil
}

func (f *fakeServeStore) ListReports(ctx context.Context, reportType, date string) ([]pipeline.ReportRow, error) {
	return nil, nil
}

func (f *fakeServeStore) ItemsByImage(ctx context.Context, reportSha string) ([]contract.ReportItem, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	r, ok := f.reports[reportSha]
	if !ok {
		return nil, nil
	}
	items := make([]contract.ReportItem, 0, len(r.items))
	for _, it := range r.items {
		items = append(items, it.ReportItem)
	}
	return items, nil
}

func (f *fakeServeStore) OCRResultJSON(ctx context.Context, ocrID int64) ([]byte, error) {
	return []byte("{}"), nil
}

func (f *fakeServeStore) TrendByName(ctx context.Context, itemName, reportType string) ([]pipeline.TrendRow, error) {
	return nil, nil
}

// WriteReport 镜像 storage upsert:报告字段全量覆盖(fail_reason 随写),items 全量
// 替换且 review_status 回 unreviewed、item_id 重发;member_id 不在 upsert 列内(保留)。
func (f *fakeServeStore) WriteReport(ctx context.Context, reportSha string, ocrID int64, report contract.Report, failReason *string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	r, ok := f.reports[reportSha]
	if !ok {
		r = &fakeServeReport{summary: pipeline.ReportSummary{Sha256: reportSha}}
		f.reports[reportSha] = r
		f.order = append(f.order, reportSha)
	}
	r.summary.ReportType = report.ReportType
	r.summary.ReportDate = report.ReportDate
	r.summary.Status = report.Status
	r.summary.FailReason = failReason
	r.items = nil
	for _, it := range report.Items {
		f.nextID++
		r.items = append(r.items, pipeline.ReportItemRow{
			ItemID: f.nextID, ReviewStatus: pipeline.ReviewStatusUnreviewed, ReportItem: it,
		})
	}
	return nil
}

// summarize 读时聚合,镜像 SQL(reportAggregateSQL):flag 计数就地算;
// unreviewed 只统计 partial 报告的低置信未核对项。
func (f *fakeServeStore) summarize(r *fakeServeReport) pipeline.ReportSummary {
	sum := r.summary
	sum.FlagCounts = pipeline.FlagCounts{}
	sum.UnreviewedLowConfidenceCount = 0
	for _, it := range r.items {
		switch it.Flag {
		case "high":
			sum.FlagCounts.High++
		case "low":
			sum.FlagCounts.Low++
		case "unknown":
			sum.FlagCounts.Unknown++
		}
		if sum.Status == contract.StatusPartial && it.ReviewStatus == pipeline.ReviewStatusUnreviewed && it.LowConfidence {
			sum.UnreviewedLowConfidenceCount++
		}
	}
	return sum
}

func (f *fakeServeStore) ListTimeline(ctx context.Context, memberID *int64) ([]pipeline.ReportSummary, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []pipeline.ReportSummary
	for _, sha := range f.order {
		r := f.reports[sha]
		if memberID != nil && (r.summary.MemberID == nil || *r.summary.MemberID != *memberID) {
			continue
		}
		out = append(out, f.summarize(r))
	}
	return out, nil
}

func (f *fakeServeStore) SummaryByImage(ctx context.Context, imageSha string) (*pipeline.ReportSummary, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	r, ok := f.reports[imageSha]
	if !ok {
		return nil, nil
	}
	sum := f.summarize(r)
	return &sum, nil
}

func (f *fakeServeStore) ItemsWithReviewStatus(ctx context.Context, reportSha string) ([]pipeline.ReportItemRow, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	r, ok := f.reports[reportSha]
	if !ok {
		return nil, nil
	}
	return append([]pipeline.ReportItemRow(nil), r.items...), nil
}

func (f *fakeServeStore) SetReportMember(ctx context.Context, reportSha string, memberID int64) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	r, ok := f.reports[reportSha]
	if !ok {
		return fmt.Errorf("report %s missing", reportSha)
	}
	id := memberID
	r.summary.MemberID = &id
	return nil
}

// seedReport 直接种一份报告(时间线用例的夹具;绕过 ingest 编排)。
func (f *fakeServeStore) seedReport(sha string, memberID *int64, report contract.Report, failReason *string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	r := &fakeServeReport{summary: pipeline.ReportSummary{
		Sha256: sha, MemberID: memberID, ReportType: report.ReportType,
		ReportDate: report.ReportDate, Status: report.Status, FailReason: failReason,
	}}
	f.reports[sha] = r
	f.order = append(f.order, sha)
	for _, it := range report.Items {
		f.nextID++
		r.items = append(r.items, pipeline.ReportItemRow{
			ItemID: f.nextID, ReviewStatus: pipeline.ReviewStatusUnreviewed, ReportItem: it,
		})
	}
}

// setReviewStatus 把报告第 i 项标记为已确认/已修正(核对决策入口,票 09 前的测试桩)。
func (f *fakeServeStore) setReviewStatus(sha string, i int, status string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.reports[sha].items[i].ReviewStatus = status
}

func (f *fakeServeStore) reportCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.reports)
}

func (f *fakeServeStore) ocrCount(sha string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.ocr[sha])
}

// fakeUploadRecognizer 可配置识别结果;Report 返回深拷贝(不把 applyFailInvariant
// 的就地归一化带进下一次调用)。
type fakeUploadRecognizer struct {
	calls  int
	report *contract.Report
	err    error
}

func (f *fakeUploadRecognizer) Report(ctx context.Context, image []byte, filename, date string) (*contract.OCRResult, *contract.Report, error) {
	f.calls++
	if f.err != nil {
		return nil, nil, f.err
	}
	return &contract.OCRResult{Txts: []string{"x"}, Engine: "onnxruntime"}, cloneReport(f.report), nil
}

func (f *fakeUploadRecognizer) Reparse(ctx context.Context, ocr *contract.OCRResult, date string) (*contract.Report, error) {
	return cloneReport(f.report), nil
}

func cloneReport(r *contract.Report) *contract.Report {
	raw, _ := json.Marshal(r)
	var out contract.Report
	_ = json.Unmarshal(raw, &out)
	return &out
}

// sampleUploadReport 真实样本形状(字段取自 schemas/testdata/http/report.detail.json 同源数据):
// partial 报告、两个指标项(一项高置信 normal、一项低置信 high)。
func sampleUploadReport() *contract.Report {
	v1, u1, r1 := "10.8", "%", "3-10"
	v2, u2, r2 := "128", "g/L", "115-150"
	return &contract.Report{
		ReportType: "血常规",
		ReportDate: str("2026-10-01"),
		Status:     contract.StatusPartial,
		Items: []contract.ReportItem{
			{Name: "单核细胞比率", Value: &v1, Unit: &u1, RefRange: &r1, Flag: "high", LowConfidence: true,
				RawText: "单核细胞百分比  10.8  ↑ %  3-10"},
			{Name: "血红蛋白", Value: &v2, Unit: &u2, RefRange: &r2, Flag: "normal",
				RawText: "血红蛋白  128  g/L  115-150"},
		},
	}
}

// newReportsTestServer 报告端点集成测试的 handler 栈(空成员库 + 可配识别器 + 内存报告库)。
func newReportsTestServer(t *testing.T) (*httptest.Server, *fakeServeStore, *fakeUploadRecognizer, *fakeMemberStore) {
	t.Helper()
	st := newFakeServeStore()
	rec := &fakeUploadRecognizer{report: sampleUploadReport()}
	ms := &fakeMemberStore{}
	d := &deps{store: st, members: ms, reports: st, rec: rec, obj: fakeObjectsCLI{}}
	srv := httptest.NewServer(newServeHandler(d, ""))
	t.Cleanup(srv.Close)
	return srv, st, rec, ms
}

// multipartUpload 构造上传请求体:image 文件 part + 任意表单字段。
func multipartUpload(t *testing.T, image []byte, fields map[string]string) (string, *bytes.Buffer) {
	t.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	if image != nil {
		fw, err := w.CreateFormFile(formImage, "sheet.jpg")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := fw.Write(image); err != nil {
			t.Fatal(err)
		}
	}
	for k, v := range fields {
		if err := w.WriteField(k, v); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return w.FormDataContentType(), &buf
}

// doUpload 发一次 multipart 上传,返回状态码与响应体。
func doUpload(t *testing.T, srv *httptest.Server, image []byte, fields map[string]string) (int, []byte) {
	t.Helper()
	return doUploadQuery(t, srv, image, fields, "")
}

// doUploadQuery 同 doUpload,但 URL 可携带原始 query(验证 query 不参与 multipart 取值)。
func doUploadQuery(t *testing.T, srv *httptest.Server, image []byte, fields map[string]string, rawQuery string) (int, []byte) {
	t.Helper()
	ct, body := multipartUpload(t, image, fields)
	resp, err := http.Post(srv.URL+"/api/v1/reports?"+rawQuery, ct, body)
	if err != nil {
		t.Fatalf("POST /reports?%s: %v", rawQuery, err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	return resp.StatusCode, raw
}

// decodeUpload 断言 2xx 并解码 UploadResult。
func decodeUpload(t *testing.T, status int, raw []byte, wantStatus int) httpapi.UploadResult {
	t.Helper()
	if status != wantStatus {
		t.Fatalf("upload status = %d, want %d; body = %s", status, wantStatus, raw)
	}
	var got httpapi.UploadResult
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("decode UploadResult: %v; body = %s", err, raw)
	}
	return got
}

// TestServeUploadNewReport 新图上传:201 + 完整报告(成员归属、聚合计数、
// 指标项句柄与核对状态、ocr_lines 空数组与 prev null 的契约形状),识别只跑一次。
func TestServeUploadNewReport(t *testing.T) {
	srv, st, rec, _ := newReportsTestServer(t)
	createOne(t, srv, `{"name":"小虎","is_child":true}`)

	image := []byte("fake-jpeg-bytes-01")
	status, raw := doUpload(t, srv, image, map[string]string{formMemberID: "1"})
	got := decodeUpload(t, status, raw, http.StatusCreated)

	sha := pipeline.ImageSha256Hex(image)
	if got.Duplicate {
		t.Error("duplicate = true, want false for a new upload")
	}
	if got.Report.ReportID != sha {
		t.Errorf("report_id = %q, want sha256 %q", got.Report.ReportID, sha)
	}
	if got.Report.MemberID == nil || *got.Report.MemberID != 1 {
		t.Errorf("member_id = %v, want 1", got.Report.MemberID)
	}
	if got.Report.ReportType != "血常规" || got.Report.ReportDate == nil || *got.Report.ReportDate != "2026-10-01" {
		t.Errorf("report type/date = %q/%v, want 血常规/2026-10-01", got.Report.ReportType, got.Report.ReportDate)
	}
	if got.Report.Status != contract.StatusPartial {
		t.Errorf("status = %q, want partial", got.Report.Status)
	}
	if got.Report.FailReason != nil {
		t.Errorf("fail_reason = %v, want null for non-failed report", got.Report.FailReason)
	}
	if got.Report.FlagCounts != (httpapi.FlagCounts{High: 1, Low: 0, Unknown: 0}) {
		t.Errorf("flag_counts = %+v, want high=1", got.Report.FlagCounts)
	}
	if got.Report.UnreviewedLowConfidenceCount != 1 {
		t.Errorf("unreviewed_low_confidence_count = %d, want 1 (partial 的低置信未核对项)", got.Report.UnreviewedLowConfidenceCount)
	}
	if len(got.Report.Items) != 2 {
		t.Fatalf("items = %d, want 2", len(got.Report.Items))
	}
	first := got.Report.Items[0]
	if first.ItemID == 0 || first.ReviewStatus != pipeline.ReviewStatusUnreviewed {
		t.Errorf("first item = %+v, want item_id>0 / review_status unreviewed", first)
	}
	if first.Prev != nil || len(first.OcrLines) != 0 {
		t.Errorf("first item prev/ocr_lines = %+v/%v, want null/[] (票 07 与 M3 填充面)", first.Prev, first.OcrLines)
	}
	if !strings.Contains(string(raw), `"ocr_lines":[]`) {
		t.Errorf("ocr_lines must serialize as [] not null; body = %s", raw)
	}
	if rec.calls != 1 {
		t.Errorf("recognizer calls = %d, want 1 (同步等待识别)", rec.calls)
	}
	if st.reportCount() != 1 || st.ocrCount(sha) != 1 {
		t.Errorf("store rows = %d report(s), %d ocr, want 1/1", st.reportCount(), st.ocrCount(sha))
	}
}

// TestServeUploadDuplicate sha 幂等:同图重传 200 + duplicate:true,不重复归档、
// 识别不重跑、item_id 不变;已有归属不被夺走;无主存量报告在重传时补齐归属(自愈)。
func TestServeUploadDuplicate(t *testing.T) {
	srv, st, rec, _ := newReportsTestServer(t)
	createOne(t, srv, `{"name":"小虎","is_child":true}`)
	createOne(t, srv, `{"name":"妈妈","is_child":false}`)

	image := []byte("fake-jpeg-bytes-02")
	status, raw := doUpload(t, srv, image, map[string]string{formMemberID: "1"})
	first := decodeUpload(t, status, raw, http.StatusCreated)

	// 同图再传(换个成员):返回已有报告,归属仍是首次的成员。
	status, raw = doUpload(t, srv, image, map[string]string{formMemberID: "2"})
	got := decodeUpload(t, status, raw, http.StatusOK)
	if !got.Duplicate {
		t.Error("duplicate = false, want true on sha hit")
	}
	if got.Report.ReportID != first.Report.ReportID {
		t.Errorf("duplicate report_id = %q, want %q", got.Report.ReportID, first.Report.ReportID)
	}
	if got.Report.MemberID == nil || *got.Report.MemberID != 1 {
		t.Errorf("duplicate member_id = %v, want 1 (已有归属不被夺走)", got.Report.MemberID)
	}
	if got.Report.Items[0].ItemID != first.Report.Items[0].ItemID {
		t.Errorf("item_id changed on duplicate: %d -> %d", first.Report.Items[0].ItemID, got.Report.Items[0].ItemID)
	}
	if rec.calls != 1 {
		t.Errorf("recognizer calls = %d, want 1 (幂等命中不重跑)", rec.calls)
	}
	if st.reportCount() != 1 || st.ocrCount(first.Report.ReportID) != 1 {
		t.Errorf("store rows = %d report(s), want 1 (不重复归档)", st.reportCount())
	}

	// 无主存量报告(如 CLI 时代):同图重传补齐归属,时间线可见。
	legacy := []byte("fake-jpeg-bytes-legacy")
	st.seedReport(pipeline.ImageSha256Hex(legacy), nil, *sampleUploadReport(), nil)
	status, raw = doUpload(t, srv, legacy, map[string]string{formMemberID: "2"})
	healed := decodeUpload(t, status, raw, http.StatusOK)
	if !healed.Duplicate {
		t.Error("legacy duplicate = false, want true")
	}
	if healed.Report.MemberID == nil || *healed.Report.MemberID != 2 {
		t.Errorf("legacy member_id = %v, want 2 (无主报告重传补齐归属)", healed.Report.MemberID)
	}
}

// TestServeUploadFailedReport 识别失败留痕:201 + status=failed + fail_reason(流水线产出),
// 徽标计数全 0、无可核对项;同图再传仍 200(失败报告无核对决策,幂等返回同一失败报告)。
func TestServeUploadFailedReport(t *testing.T) {
	srv, _, rec, _ := newReportsTestServer(t)
	createOne(t, srv, `{"name":"小虎","is_child":true}`)
	rec.report = &contract.Report{ReportType: "其他专项", Status: contract.StatusFailed}

	image := []byte("blank-or-blurry-photo")
	status, raw := doUpload(t, srv, image, map[string]string{formMemberID: "1"})
	got := decodeUpload(t, status, raw, http.StatusCreated)
	if got.Report.Status != contract.StatusFailed {
		t.Errorf("status = %q, want failed", got.Report.Status)
	}
	if got.Report.FailReason == nil || *got.Report.FailReason != "no metric items recognized" {
		t.Errorf("fail_reason = %v, want pipeline 产出的失败原因", got.Report.FailReason)
	}
	if got.Report.FlagCounts != (httpapi.FlagCounts{}) || got.Report.UnreviewedLowConfidenceCount != 0 {
		t.Errorf("counts = %+v/%d, want 全 0(failed 不计入待核对)", got.Report.FlagCounts, got.Report.UnreviewedLowConfidenceCount)
	}
	if len(got.Report.Items) != 0 {
		t.Errorf("items = %d, want 0 (failed 无可核对项)", len(got.Report.Items))
	}

	status, raw = doUpload(t, srv, image, map[string]string{formMemberID: "1"})
	again := decodeUpload(t, status, raw, http.StatusOK)
	if !again.Duplicate || again.Report.ReportID != got.Report.ReportID {
		t.Errorf("re-upload failed report = %+v, want duplicate of same report (留痕不骗人)", again)
	}
}

// TestServeUploadOverride 备份分岔:已有核对决策的报告同图重传 → 409(文案锚定
// fixture report.conflict.json);override=true → 201 重跑:item_id 全换、
// review_status 回 unreviewed、归属保持;无核对决策时 override 同样重跑(显式覆盖)。
func TestServeUploadOverride(t *testing.T) {
	srv, st, rec, _ := newReportsTestServer(t)
	createOne(t, srv, `{"name":"小虎","is_child":true}`)

	image := []byte("fake-jpeg-bytes-03")
	status, raw := doUpload(t, srv, image, map[string]string{formMemberID: "1"})
	first := decodeUpload(t, status, raw, http.StatusCreated)
	sha := first.Report.ReportID

	st.setReviewStatus(sha, 0, "confirmed")

	status, raw = doUpload(t, srv, image, map[string]string{formMemberID: "1"})
	if status != http.StatusConflict {
		t.Fatalf("status = %d, want 409 (已有核对决策); body = %s", status, raw)
	}
	var apiErr httpapi.Error
	if err := json.Unmarshal(raw, &apiErr); err != nil {
		t.Fatalf("409 body is not an Error shape: %s", raw)
	}
	if apiErr.Message != reviewDecisionConflictMessage {
		t.Errorf("409 message = %q, want 锚定 fixture 文案 %q", apiErr.Message, reviewDecisionConflictMessage)
	}
	if rec.calls != 1 {
		t.Errorf("recognizer calls = %d, want 1 (409 不得重跑)", rec.calls)
	}

	status, raw = doUpload(t, srv, image, map[string]string{formMemberID: "1", formOverride: "true"})
	overridden := decodeUpload(t, status, raw, http.StatusCreated)
	if overridden.Duplicate {
		t.Error("override duplicate = true, want false")
	}
	if overridden.Report.Items[0].ItemID == first.Report.Items[0].ItemID {
		t.Errorf("item_id = %d unchanged after override, want 全换", overridden.Report.Items[0].ItemID)
	}
	for _, it := range overridden.Report.Items {
		if it.ReviewStatus != pipeline.ReviewStatusUnreviewed {
			t.Errorf("item %d review_status = %q, want unreviewed after override", it.ItemID, it.ReviewStatus)
		}
	}
	if overridden.Report.MemberID == nil || *overridden.Report.MemberID != 1 {
		t.Errorf("member_id = %v, want 1 (override 不夺走归属)", overridden.Report.MemberID)
	}
	if rec.calls != 2 {
		t.Errorf("recognizer calls = %d, want 2 (override 重跑识别)", rec.calls)
	}

	// 无核对决策时 override 也是显式重跑(duplicate 分岔之外的唯一重跑入口)。
	before := overridden.Report.Items[0].ItemID
	status, raw = doUpload(t, srv, image, map[string]string{formMemberID: "1", formOverride: "true"})
	rerun := decodeUpload(t, status, raw, http.StatusCreated)
	if rerun.Report.Items[0].ItemID == before {
		t.Errorf("item_id = %d unchanged on explicit override rerun", before)
	}
}

// TestServeUploadValidation 上传校验矩阵:缺件、空图、缺/错成员、未知成员、
// 未知字段、非法 override、非 multipart 一律 4xx 且零写入。
func TestServeUploadValidation(t *testing.T) {
	cases := []struct {
		name   string
		image  []byte
		fields map[string]string
		status int
	}{
		{"missing image", nil, map[string]string{formMemberID: "1"}, http.StatusBadRequest},
		{"empty image", []byte{}, map[string]string{formMemberID: "1"}, http.StatusBadRequest},
		{"missing member", []byte("img"), map[string]string{}, http.StatusBadRequest},
		{"non-integer member", []byte("img"), map[string]string{formMemberID: "abc"}, http.StatusBadRequest},
		{"unknown member", []byte("img"), map[string]string{formMemberID: "999"}, http.StatusNotFound},
		{"unknown field", []byte("img"), map[string]string{formMemberID: "1", "note": "x"}, http.StatusBadRequest},
		{"bad override", []byte("img"), map[string]string{formMemberID: "1", formOverride: "maybe"}, http.StatusBadRequest},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			srv, st, rec, _ := newReportsTestServer(t)
			createOne(t, srv, `{"name":"小虎","is_child":true}`)
			status, raw := doUpload(t, srv, c.image, c.fields)
			if status != c.status {
				t.Fatalf("status = %d, want %d; body = %s", status, c.status, raw)
			}
			var apiErr httpapi.Error
			if err := json.Unmarshal(raw, &apiErr); err != nil {
				t.Fatalf("error body is not an Error shape: %s", raw)
			}
			if apiErr.Message == "" {
				t.Errorf("error message empty: %s", raw)
			}
			if st.reportCount() != 0 || rec.calls != 0 {
				t.Errorf("rejected upload touched store/recognizer: reports=%d calls=%d", st.reportCount(), rec.calls)
			}
		})
	}

	// 非 multipart 请求体(契约要求 form-data)。
	srv, _, rec, _ := newReportsTestServer(t)
	createOne(t, srv, `{"name":"小虎","is_child":true}`)
	status, raw := doJSON(t, srv, http.MethodPost, "/api/v1/reports", `{"member_id":1}`)
	if status != http.StatusBadRequest {
		t.Fatalf("JSON body status = %d, want 400; body = %s", status, raw)
	}
	if rec.calls != 0 {
		t.Errorf("recognizer calls = %d, want 0", rec.calls)
	}
}

// TestServeUploadIgnoresURLQuery 契约的字段面 = multipart body:URL query 不参与取值。
// 回归自实测确认的 Go 行为——multipart 下 r.FormValue 的 query 值优先于 body
// (ParseMultipartForm 把 body 字段追加在 query 之后),故取值必须走 PostFormValue:
// query 不得覆盖 body 归属、不得单独满足必填、空值不得遮蔽合法 body 值、不得代替 body 放行 override。
func TestServeUploadIgnoresURLQuery(t *testing.T) {
	t.Run("query 不覆盖 body 归属", func(t *testing.T) {
		srv, _, _, _ := newReportsTestServer(t)
		createOne(t, srv, `{"name":"小虎","is_child":true}`)
		status, raw := doUploadQuery(t, srv, []byte("img"), map[string]string{formMemberID: "1"}, "member_id=999")
		got := decodeUpload(t, status, raw, http.StatusCreated)
		if got.Report.MemberID == nil || *got.Report.MemberID != 1 {
			t.Errorf("member_id = %v, want 1 (body 值生效,query 999 被忽略)", got.Report.MemberID)
		}
	})

	t.Run("query 不满足必填", func(t *testing.T) {
		srv, st, rec, _ := newReportsTestServer(t)
		createOne(t, srv, `{"name":"小虎","is_child":true}`)
		status, raw := doUploadQuery(t, srv, []byte("img"), map[string]string{}, "member_id=1")
		if status != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400 (query 不能替代表单字段); body = %s", status, raw)
		}
		if st.reportCount() != 0 || rec.calls != 0 {
			t.Errorf("rejected upload touched store/recognizer: reports=%d calls=%d", st.reportCount(), rec.calls)
		}
	})

	t.Run("空 query 不遮蔽 body 值", func(t *testing.T) {
		srv, _, _, _ := newReportsTestServer(t)
		createOne(t, srv, `{"name":"小虎","is_child":true}`)
		status, raw := doUploadQuery(t, srv, []byte("img"), map[string]string{formMemberID: "1"}, "member_id=")
		got := decodeUpload(t, status, raw, http.StatusCreated)
		if got.Report.MemberID == nil || *got.Report.MemberID != 1 {
			t.Errorf("member_id = %v, want 1 (空 query 不得遮蔽 body 值)", got.Report.MemberID)
		}
	})

	t.Run("query override 不放行 409", func(t *testing.T) {
		srv, st, rec, _ := newReportsTestServer(t)
		createOne(t, srv, `{"name":"小虎","is_child":true}`)
		image := []byte("img")
		status, raw := doUpload(t, srv, image, map[string]string{formMemberID: "1"})
		first := decodeUpload(t, status, raw, http.StatusCreated)
		st.setReviewStatus(first.Report.ReportID, 0, "confirmed")

		status, raw = doUploadQuery(t, srv, image, map[string]string{formMemberID: "1"}, "override=true")
		if status != http.StatusConflict {
			t.Fatalf("status = %d, want 409 (query override 无效,需表单显式覆盖); body = %s", status, raw)
		}
		if rec.calls != 1 {
			t.Errorf("recognizer calls = %d, want 1 (409 不得重跑)", rec.calls)
		}
	})
}

// TestServeUploadRecognizerFailure 识别服务硬故障:500 + 中性错误信息,零归档。
func TestServeUploadRecognizerFailure(t *testing.T) {
	srv, st, rec, _ := newReportsTestServer(t)
	createOne(t, srv, `{"name":"小虎","is_child":true}`)
	rec.err = errors.New("dial tcp recognizer: connection refused")

	status, raw := doUpload(t, srv, []byte("img"), map[string]string{formMemberID: "1"})
	if status != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500; body = %s", status, raw)
	}
	var apiErr httpapi.Error
	if err := json.Unmarshal(raw, &apiErr); err != nil || !strings.Contains(apiErr.Message, "connection refused") {
		t.Errorf("message = %q (err=%v), want 透出识别服务失败原因", raw, err)
	}
	if st.reportCount() != 0 {
		t.Errorf("reports = %d, want 0 (硬故障不落半截归档)", st.reportCount())
	}
}

// TestServeListReportsFilterAndCounts 时间线成员过滤 + 卡片字段:failed 的
// fail_reason 可见、dateless 不崩、徽标按 flag 聚合、待核对只算 partial 的
// unreviewed 低置信项(success 免核对恒 0),缺省含无主存量报告。
func TestServeListReportsFilterAndCounts(t *testing.T) {
	srv, st, _, _ := newReportsTestServer(t)
	a, b := int64(1), int64(2)

	v1, low := "10.8", true
	v2 := "128"
	failReason := "no metric items recognized"
	// 成员 A:partial(high 低置信 + low 低置信未核对)、failed 无日期。
	st.seedReport("aaa", &a, contract.Report{ReportType: "血常规", ReportDate: str("2026-10-02"), Status: contract.StatusPartial, Items: []contract.ReportItem{
		{Name: "单核细胞比率", Value: &v1, RefRange: str("3-10"), Flag: "high", LowConfidence: low, RawText: "单核细胞百分比  10.8  ↑ %  3-10"},
		{Name: "平均血红蛋白浓度", Value: &v2, Flag: "low", LowConfidence: low, RawText: "平均血红蛋白浓度  300  g/L  316-354"},
	}}, nil)
	st.seedReport("bbb", &a, contract.Report{ReportType: "其他专项", Status: contract.StatusFailed}, &failReason)
	// 成员 B:success 但含一条低置信未核对项(识别干净免核对:计数必须为 0)。
	st.seedReport("ccc", &b, contract.Report{ReportType: "尿常规", ReportDate: str("2026-10-01"), Status: contract.StatusSuccess, Items: []contract.ReportItem{
		{Name: "尿蛋白", Value: &v1, Flag: "unknown", LowConfidence: low, RawText: "尿蛋白  10.8"},
	}}, nil)
	// 无主存量报告(CLI 时代)。
	st.seedReport("ddd", nil, contract.Report{ReportType: "血糖", ReportDate: str("2026-09-30"), Status: contract.StatusSuccess, Items: []contract.ReportItem{
		{Name: "葡萄糖", Value: &v2, Flag: "normal", RawText: "葡萄糖  128"},
	}}, nil)

	// 成员 A:两份(failed 的失败原因可见、无日期不崩)。
	status, raw := doJSON(t, srv, http.MethodGet, "/api/v1/reports?member_id=1", "")
	if status != http.StatusOK {
		t.Fatalf("GET /reports?member_id=1: status = %d; body = %s", status, raw)
	}
	var list httpapi.ReportsList
	if err := json.Unmarshal(raw, &list); err != nil {
		t.Fatal(err)
	}
	if len(list.Reports) != 2 {
		t.Fatalf("member 1 reports = %d, want 2; body = %s", len(list.Reports), raw)
	}
	partial := list.Reports[0]
	if partial.ReportType != "血常规" || partial.Status != contract.StatusPartial {
		t.Errorf("first card = %+v, want 血常规/partial", partial)
	}
	if partial.FlagCounts != (httpapi.FlagCounts{High: 1, Low: 1}) || partial.UnreviewedLowConfidenceCount != 2 {
		t.Errorf("partial card counts = %+v/%d, want high=1 low=1 / 2 待核对", partial.FlagCounts, partial.UnreviewedLowConfidenceCount)
	}
	failed := list.Reports[1]
	if failed.Status != contract.StatusFailed || failed.FailReason == nil || *failed.FailReason != failReason {
		t.Errorf("failed card = %+v, want fail_reason 可见", failed)
	}
	if failed.ReportDate != nil || failed.UnreviewedLowConfidenceCount != 0 {
		t.Errorf("failed card date/count = %v/%d, want null/0", failed.ReportDate, failed.UnreviewedLowConfidenceCount)
	}

	// 成员 B:success 免核对,计数恒 0。
	status, raw = doJSON(t, srv, http.MethodGet, "/api/v1/reports?member_id=2", "")
	if status != http.StatusOK {
		t.Fatalf("GET member 2: status = %d", status)
	}
	if err := json.Unmarshal(raw, &list); err != nil {
		t.Fatal(err)
	}
	if len(list.Reports) != 1 || list.Reports[0].UnreviewedLowConfidenceCount != 0 {
		t.Fatalf("member 2 reports = %+v, want 1 card with count 0 (success 免核对)", list.Reports)
	}
	if list.Reports[0].FlagCounts.Unknown != 1 {
		t.Errorf("member 2 flag_counts = %+v, want unknown=1(徽标照算)", list.Reports[0].FlagCounts)
	}

	// 缺省:全部报告(含无主)。
	status, raw = doJSON(t, srv, http.MethodGet, "/api/v1/reports", "")
	if status != http.StatusOK {
		t.Fatalf("GET /reports: status = %d", status)
	}
	if err := json.Unmarshal(raw, &list); err != nil {
		t.Fatal(err)
	}
	if len(list.Reports) != 4 {
		t.Fatalf("unfiltered reports = %d, want 4 (含无主)", len(list.Reports))
	}
	if list.Reports[3].MemberID != nil {
		t.Errorf("legacy member_id = %v, want null", list.Reports[3].MemberID)
	}

	// 未知成员 → 200 空信封;非法 member_id → 400。
	status, raw = doJSON(t, srv, http.MethodGet, "/api/v1/reports?member_id=999", "")
	if status != http.StatusOK || string(raw) != `{"reports":[]}`+"\n" {
		t.Fatalf("unknown member = %d %s, want 200 空信封", status, raw)
	}
	status, raw = doJSON(t, srv, http.MethodGet, "/api/v1/reports?member_id=abc", "")
	if status != http.StatusBadRequest {
		t.Fatalf("non-integer member_id = %d %s, want 400", status, raw)
	}
}
