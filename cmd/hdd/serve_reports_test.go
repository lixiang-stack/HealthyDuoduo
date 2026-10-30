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
	mu        sync.Mutex
	nextOCR   int64 // ocr_results.id 的 BIGSERIAL 镜像
	nextID    int64 // report_items.id 的 BIGSERIAL 镜像:重发新句柄、不重用
	images    map[string]bool
	ocr       map[string][][]byte
	ocrByID   map[int64][]byte // AppendOCRResult 的 raw 落库;OCRResultJSONByReport 回读
	reportOCR map[string]int64 // reports.ocr_result_id 镜像(WriteReport 记录)
	order     []string         // 报告插入序(时间线迭代确定化)
	reports   map[string]*fakeServeReport
	pairs     []fakeCorrectionPair // correction_pairs 追加记录(仅修正产生)
}

// fakeCorrectionPair 修正对 + 所属指标项 id(correction_pairs.item_id 的镜像:
// 删除级联用例按项断言;storage 侧由 FK ON DELETE CASCADE 承担)。
type fakeCorrectionPair struct {
	itemID int64
	pair   pipeline.CorrectionPairRecord
}

type fakeServeReport struct {
	summary  pipeline.ReportSummary
	sections []contract.ReportSection
	items    []pipeline.ReportItemRow
}

var (
	_ pipeline.Store       = (*fakeServeStore)(nil)
	_ pipeline.ReportStore = (*fakeServeStore)(nil)
)

func newFakeServeStore() *fakeServeStore {
	return &fakeServeStore{
		images:    map[string]bool{},
		ocr:       map[string][][]byte{},
		ocrByID:   map[int64][]byte{},
		reportOCR: map[string]int64{},
		reports:   map[string]*fakeServeReport{},
	}
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
	f.ocrByID[f.nextOCR] = raw
	return f.nextOCR, nil
}

// OCRResultJSONByReport 镜像 storage:reports.ocr_result_id → ocr_results.engine_output;
// 报告缺失 nil, nil。
func (f *fakeServeStore) OCRResultJSONByReport(ctx context.Context, reportSha string) ([]byte, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	ocrID, ok := f.reportOCR[reportSha]
	if !ok {
		return nil, nil
	}
	return f.ocrByID[ocrID], nil
}

// SectionsByImage 镜像 storage:报告存在则返回段落副本(无段落 = nil)。
func (f *fakeServeStore) SectionsByImage(ctx context.Context, reportSha string) ([]contract.ReportSection, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	r, ok := f.reports[reportSha]
	if !ok {
		return nil, nil
	}
	return append([]contract.ReportSection(nil), r.sections...), nil
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
		Panels: append([]string(nil), r.summary.Panels...),
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
	r.summary.Panels = append([]string(nil), report.Panels...)
	r.sections = append([]contract.ReportSection(nil), report.Sections...)
	f.reportOCR[reportSha] = ocrID
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
// unreviewed 统计所有报告的未核对低置信项(success 也如实计数)。
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
		if it.ReviewStatus == pipeline.ReviewStatusUnreviewed && it.LowConfidence && it.Flag != "not_tested" {
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

// SetReportMember 镜像 storage:只改归属;报告缺失返回 false, nil。
func (f *fakeServeStore) SetReportMember(ctx context.Context, reportSha string, memberID int64) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	r, ok := f.reports[reportSha]
	if !ok {
		return false, nil
	}
	id := memberID
	r.summary.MemberID = &id
	return true, nil
}

// DeleteReport 镜像 storage 硬删:报告行与指标项消失、修正对随指标项级联;
// images/ocr_results 留痕保留(重传走同图未归档路径,识别历史继续追加)。
func (f *fakeServeStore) DeleteReport(ctx context.Context, reportSha string) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	r, ok := f.reports[reportSha]
	if !ok {
		return false, nil
	}
	itemIDs := make(map[int64]bool, len(r.items))
	for _, it := range r.items {
		itemIDs[it.ItemID] = true
	}
	kept := f.pairs[:0]
	for _, p := range f.pairs {
		if !itemIDs[p.itemID] {
			kept = append(kept, p)
		}
	}
	f.pairs = kept
	delete(f.reports, reportSha)
	delete(f.reportOCR, reportSha)
	for i, sha := range f.order {
		if sha == reportSha {
			f.order = append(f.order[:i], f.order[i+1:]...)
			break
		}
	}
	return true, nil
}

// seedReport 直接种一份报告(时间线用例的夹具;绕过 ingest 编排)。真实 schema 里
// reports.ocr_result_id NOT NULL,故同时种一行空 OCR 结果(与库口径一致;具体行链接
// 用例用 seedOCR 覆盖)。
func (f *fakeServeStore) seedReport(sha string, memberID *int64, report contract.Report, failReason *string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	r := &fakeServeReport{
		summary: pipeline.ReportSummary{
			Sha256: sha, MemberID: memberID, ReportType: report.ReportType,
			ReportDate: report.ReportDate, Status: report.Status, FailReason: failReason,
			Panels: append([]string(nil), report.Panels...),
		},
		sections: append([]contract.ReportSection(nil), report.Sections...),
	}
	f.reports[sha] = r
	f.order = append(f.order, sha)
	f.nextOCR++
	f.ocrByID[f.nextOCR] = []byte("{}")
	f.reportOCR[sha] = f.nextOCR
	for _, it := range report.Items {
		f.nextID++
		r.items = append(r.items, pipeline.ReportItemRow{
			ItemID: f.nextID, ReviewStatus: pipeline.ReviewStatusUnreviewed, ReportItem: it,
		})
	}
}

// seedOCR 给已种报告挂 OCR 结果(详情行链接用例;镜像 reports.ocr_result_id 写入)。
func (f *fakeServeStore) seedOCR(sha string, ocr *contract.OCRResult) {
	raw, _ := json.Marshal(ocr)
	f.mu.Lock()
	defer f.mu.Unlock()
	f.nextOCR++
	f.ocrByID[f.nextOCR] = raw
	f.reportOCR[sha] = f.nextOCR
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

func (f *fakeServeStore) hasImage(sha string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.images[sha]
}

// fakeUploadRecognizer 可配置识别结果;Report 返回深拷贝(不把 applyFailInvariant
// 的就地归一化带进下一次调用),OCR 结果取样本形状(行链接用例的数据底)。
type fakeUploadRecognizer struct {
	calls      int
	report     *contract.Report
	normalized []byte
	err        error
}

func (f *fakeUploadRecognizer) Report(ctx context.Context, image []byte, filename, date string) ([]byte, *contract.OCRResult, *contract.Report, error) {
	f.calls++
	if f.err != nil {
		return nil, nil, nil, f.err
	}
	return f.normalized, sampleUploadOCR(), cloneReport(f.report), nil
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
// partial 报告、两个指标项(一项高置信 normal、一项低置信 high);ocr_line_numbers 与
// sampleUploadOCR 的行序对齐(识别端直吐,详情按它精确映射行框)。
func sampleUploadReport() *contract.Report {
	v1, u1, r1 := "10.8", "%", "3-10"
	v2, u2, r2 := "128", "g/L", "115-150"
	return &contract.Report{
		ReportType: "血常规",
		ReportDate: str("2026-10-01"),
		Status:     contract.StatusPartial,
		Panels:     []string{"cbc"},
		Items: []contract.ReportItem{
			{Name: "单核细胞比率", Panel: "cbc", Value: &v1, Unit: &u1, RefRange: &r1, Flag: "high", LowConfidence: true,
				RawText: "单核细胞百分比  10.8  ↑ %  3-10", OCRLineNumbers: []int32{1, 2, 3, 4}},
			{Name: "血红蛋白", Panel: "cbc", Value: &v2, Unit: &u2, RefRange: &r2, Flag: "normal",
				RawText: "血红蛋白  128  g/L  115-150", OCRLineNumbers: []int32{9, 10, 11, 12}},
		},
	}
}

// sampleUploadOCR 与 sampleUploadReport 的 raw_text 同源的行数据(共享栈真实血常规报告形状):
// 两行表格数据 + 表头,含跨行重复碎片 "↑ %"(行链接重建需按 y 聚带消歧)。
func sampleUploadOCR() *contract.OCRResult {
	ocr := &contract.OCRResult{Engine: "onnxruntime"}
	for _, l := range []struct {
		text string
		x, y float64
	}{
		{"序号", 0, 0},
		{"单核细胞百分比", 51, 73},
		{"10.8", 227, 76},
		{"↑ %", 313, 74},
		{"3-10", 403, 74},
		{"中性粒细胞百分比", 52, 95},
		{"81.3", 227, 99},
		{"↑ %", 313, 98},
		{"40-75", 404, 98},
		{"血红蛋白", 591, 28},
		{"128", 788, 30},
		{"g/L", 878, 30},
		{"115-150", 953, 32},
	} {
		ocr.Txts = append(ocr.Txts, l.text)
		ocr.Boxes = append(ocr.Boxes, testBox(l.x, l.y))
		ocr.Scores = append(ocr.Scores, 0.99)
	}
	return ocr
}

// testBox 左上角 + 固定尺寸(40×20)的行框。
func testBox(x, y float64) contract.Box {
	return contract.Box{{x, y}, {x + 40, y}, {x + 40, y + 20}, {x, y + 20}}
}

// apiBox testBox → 契约镜像(与 toAPILineBox 同形;测试侧独立以免漏转换)。
func apiBox(x, y float64) httpapi.Box4x2 {
	var out httpapi.Box4x2
	for i, p := range testBox(x, y) {
		out[i] = [2]float64{p[0], p[1]}
	}
	return out
}

// newReportsTestServer 报告端点集成测试的 handler 栈(空成员库 + 可配识别器 + 内存报告库;
// 对象存储内存化,详情用例可经 /images 取回上传字节)。
func newReportsTestServer(t *testing.T) (*httptest.Server, *fakeServeStore, *fakeUploadRecognizer, *fakeMemberStore) {
	t.Helper()
	st := newFakeServeStore()
	rec := &fakeUploadRecognizer{report: sampleUploadReport()}
	ms := &fakeMemberStore{}
	d := &deps{store: st, members: ms, reports: st, rec: rec, obj: fakeObjectsCLI{objects: map[string][]byte{}}}
	srv := httptest.NewServer(newServeHandler(d))
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

// TestServeUploadNormalizedImage_UsesNormalizedIdentity 整页横拍:
// 识别端返回转正图像 → 报告身份/对象存储使用转正后的 sha;再传原图按 duplicate 返回。
func TestServeUploadNormalizedImage_UsesNormalizedIdentity(t *testing.T) {
	srv, st, rec, _ := newReportsTestServer(t)
	createOne(t, srv, `{"name":"小虎","is_child":true}`)
	original := []byte("sideways-jpeg-bytes")
	rec.normalized = []byte("upright-jpeg-bytes")

	status, raw := doUpload(t, srv, original, map[string]string{formMemberID: "1"})
	got := decodeUpload(t, status, raw, http.StatusCreated)

	wantSha := pipeline.ImageSha256Hex(rec.normalized)
	if got.Report.ReportID != wantSha {
		t.Errorf("report_id = %q, want normalized sha %q", got.Report.ReportID, wantSha)
	}
	if !st.hasImage(wantSha) {
		t.Errorf("normalized image missing from store")
	}
	if st.hasImage(pipeline.ImageSha256Hex(original)) {
		t.Errorf("original sideways bytes stored, want only normalized image")
	}

	status, raw = doUpload(t, srv, original, map[string]string{formMemberID: "1"})
	again := decodeUpload(t, status, raw, http.StatusOK)
	if !again.Duplicate || again.Report.ReportID != wantSha {
		t.Errorf("re-upload = duplicate %v report %q, want duplicate true / %q", again.Duplicate, again.Report.ReportID, wantSha)
	}
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
	if len(got.Report.Panels) != 1 || got.Report.Panels[0] != "cbc" {
		t.Errorf("panels = %v, want [cbc]", got.Report.Panels)
	}
	if got.Report.Items[0].Panel != "cbc" {
		t.Errorf("item panel = %q, want cbc", got.Report.Items[0].Panel)
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
	if first.Prev != nil {
		t.Errorf("first item prev = %+v, want null (M3 填充面)", first.Prev)
	}
	// 票 07/03:行链接 = 识别端直吐 ocr_line_numbers → OCR 行框(下标映射,无文本反推)。
	wantFirst := []int{1, 2, 3, 4}
	if len(first.OcrLines) != len(wantFirst) {
		t.Fatalf("first item ocr_lines = %+v, want %d links", first.OcrLines, len(wantFirst))
	}
	for i, want := range wantFirst {
		if first.OcrLines[i].Line != want {
			t.Errorf("first item ocr_lines[%d].line = %d, want %d", i, first.OcrLines[i].Line, want)
		}
	}
	if got, want := first.OcrLines[2].Box, apiBox(313, 74); got != want {
		t.Errorf("↑ %% box = %v, want 直映行框 %v", got, want)
	}
	second := got.Report.Items[1]
	if len(second.OcrLines) != 4 || second.OcrLines[0].Line != 9 || second.OcrLines[3].Line != 12 {
		t.Errorf("second item ocr_lines = %+v, want lines 9..12", second.OcrLines)
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
	if len(got.Report.Items[0].OcrLines) == 0 {
		t.Errorf("duplicate response ocr_lines empty, want 行链接(与详情同一投影)")
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
	rec.report = &contract.Report{ReportType: "其他专项", Status: contract.StatusFailed, Panels: []string{"other"}}

	image := []byte("blank-or-blurry-photo")
	status, raw := doUpload(t, srv, image, map[string]string{formMemberID: "1"})
	got := decodeUpload(t, status, raw, http.StatusCreated)
	if got.Report.Status != contract.StatusFailed {
		t.Errorf("status = %q, want failed", got.Report.Status)
	}
	if got.Report.FailReason == nil || *got.Report.FailReason != "no metric items recognized" {
		t.Errorf("fail_reason = %v, want pipeline 产出的失败原因", got.Report.FailReason)
	}
	if len(got.Report.Panels) != 1 || got.Report.Panels[0] != "other" {
		t.Errorf("failed panels = %v, want [other]", got.Report.Panels)
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
	srv, _, rec, _ := newReportsTestServer(t)
	createOne(t, srv, `{"name":"小虎","is_child":true}`)

	image := []byte("fake-jpeg-bytes-03")
	status, raw := doUpload(t, srv, image, map[string]string{formMemberID: "1"})
	first := decodeUpload(t, status, raw, http.StatusCreated)
	sha := first.Report.ReportID

	// 经真实核对端点建立决策(替代早期测试桩):确认首项。
	if status, raw := doReview(t, srv, sha, first.Report.Items[0].ItemID, `{"action":"confirm"}`); status != http.StatusOK {
		t.Fatalf("confirm for 409 setup = %d; body = %s", status, raw)
	}

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
		srv, _, rec, _ := newReportsTestServer(t)
		createOne(t, srv, `{"name":"小虎","is_child":true}`)
		image := []byte("img")
		status, raw := doUpload(t, srv, image, map[string]string{formMemberID: "1"})
		first := decodeUpload(t, status, raw, http.StatusCreated)
		// 经真实核对端点建立决策(替代早期测试桩):确认首项。
		if status, raw := doReview(t, srv, first.Report.ReportID, first.Report.Items[0].ItemID, `{"action":"confirm"}`); status != http.StatusOK {
			t.Fatalf("confirm for 409 setup = %d; body = %s", status, raw)
		}

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
// fail_reason 可见、dateless 不崩、徽标按 flag 聚合、待核对统计未核对低置信项
// (success 报告也如实计数),缺省含无主存量报告。
func TestServeListReportsFilterAndCounts(t *testing.T) {
	srv, st, _, _ := newReportsTestServer(t)
	a, b := int64(1), int64(2)

	v1, low := "10.8", true
	v2 := "128"
	failReason := "no metric items recognized"
	// 成员 A:partial(high 低置信 + low 低置信未核对)、failed 无日期。
	st.seedReport("aaa", &a, contract.Report{ReportType: "血常规", ReportDate: str("2026-10-02"), Status: contract.StatusPartial, Panels: []string{"cbc"}, Items: []contract.ReportItem{
		{Name: "单核细胞比率", Panel: "cbc", Value: &v1, RefRange: str("3-10"), Flag: "high", LowConfidence: low, RawText: "单核细胞百分比  10.8  ↑ %  3-10"},
		{Name: "平均血红蛋白浓度", Panel: "cbc", Value: &v2, Flag: "low", LowConfidence: low, RawText: "平均血红蛋白浓度  300  g/L  316-354"},
	}}, nil)
	st.seedReport("bbb", &a, contract.Report{ReportType: "其他专项", Status: contract.StatusFailed, Panels: []string{"other"}}, &failReason)
	// 成员 B:success 但含一条低置信未核对项(计数如实为 1)。
	st.seedReport("ccc", &b, contract.Report{ReportType: "尿常规", ReportDate: str("2026-10-01"), Status: contract.StatusSuccess, Panels: []string{"urinalysis"}, Items: []contract.ReportItem{
		{Name: "尿蛋白", Panel: "urinalysis", Value: &v1, Flag: "unknown", LowConfidence: low, RawText: "尿蛋白  10.8"},
	}}, nil)
	// 无主存量报告(CLI 时代):panels 未回填为空数组。
	st.seedReport("ddd", nil, contract.Report{ReportType: "血糖", ReportDate: str("2026-09-30"), Status: contract.StatusSuccess, Items: []contract.ReportItem{
		{Name: "葡萄糖", Panel: "glucose", Value: &v2, Flag: "normal", RawText: "葡萄糖  128"},
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
	if len(partial.Panels) != 1 || partial.Panels[0] != "cbc" {
		t.Errorf("partial card panels = %v, want [cbc]", partial.Panels)
	}
	failed := list.Reports[1]
	if failed.Status != contract.StatusFailed || failed.FailReason == nil || *failed.FailReason != failReason {
		t.Errorf("failed card = %+v, want fail_reason 可见", failed)
	}
	if len(failed.Panels) != 1 || failed.Panels[0] != "other" {
		t.Errorf("failed card panels = %v, want [other]", failed.Panels)
	}
	if failed.ReportDate != nil || failed.UnreviewedLowConfidenceCount != 0 {
		t.Errorf("failed card date/count = %v/%d, want null/0", failed.ReportDate, failed.UnreviewedLowConfidenceCount)
	}

	// 成员 B:success 含一条低置信未核对项,计数如实为 1。
	status, raw = doJSON(t, srv, http.MethodGet, "/api/v1/reports?member_id=2", "")
	if status != http.StatusOK {
		t.Fatalf("GET member 2: status = %d", status)
	}
	if err := json.Unmarshal(raw, &list); err != nil {
		t.Fatal(err)
	}
	if len(list.Reports) != 1 || list.Reports[0].UnreviewedLowConfidenceCount != 1 {
		t.Fatalf("member 2 reports = %+v, want 1 card with count 1 (success 也统计未核对低置信)", list.Reports)
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
	if list.Reports[3].Panels == nil || len(list.Reports[3].Panels) != 0 {
		t.Errorf("legacy panels = %v, want [] (未回填;不能是 null)", list.Reports[3].Panels)
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

// TestServeGetReportDetail GET /api/v1/reports/{report_id}:详情完整形状(头部与聚合计数、
// 指标项外部句柄与核对状态、OCR 行链接、prev null);report_id 即图像身份,/images 可直出原图;
// 未知与非法形态 404(契约只声明 404)。
func TestServeGetReportDetail(t *testing.T) {
	srv, st, _, _ := newReportsTestServer(t)
	createOne(t, srv, `{"name":"小虎","is_child":true}`)

	image := sampleImageBytes(t) // 真实样本图:图像直出断言按字节逐位比较
	status, raw := doUpload(t, srv, image, map[string]string{formMemberID: "1"})
	created := decodeUpload(t, status, raw, http.StatusCreated)
	sha := created.Report.ReportID

	status, raw = doJSON(t, srv, http.MethodGet, "/api/v1/reports/"+sha, "")
	if status != http.StatusOK {
		t.Fatalf("GET /reports/%s = %d; body = %s", sha, status, raw)
	}
	var detail httpapi.ReportDetail
	if err := json.Unmarshal(raw, &detail); err != nil {
		t.Fatal(err)
	}
	if detail.ReportID != sha || detail.ReportType != "血常规" || detail.Status != contract.StatusPartial {
		t.Errorf("detail header = %+v", detail.ReportHeader)
	}
	if detail.ReportDate == nil || *detail.ReportDate != "2026-10-01" {
		t.Errorf("report_date = %v, want 2026-10-01", detail.ReportDate)
	}
	if detail.MemberID == nil || *detail.MemberID != 1 {
		t.Errorf("member_id = %v, want 1", detail.MemberID)
	}
	if len(detail.Panels) != 1 || detail.Panels[0] != "cbc" {
		t.Errorf("detail panels = %v, want [cbc]", detail.Panels)
	}
	if detail.FlagCounts != (httpapi.FlagCounts{High: 1}) || detail.UnreviewedLowConfidenceCount != 1 {
		t.Errorf("counts = %+v/%d, want high=1 / 1 待核对", detail.FlagCounts, detail.UnreviewedLowConfidenceCount)
	}
	if detail.Sections != nil || strings.Contains(string(raw), `"sections"`) {
		t.Errorf("non-narrative detail must omit sections; sections=%+v body=%s", detail.Sections, raw)
	}
	if len(detail.Items) != 2 {
		t.Fatalf("items = %d, want 2", len(detail.Items))
	}
	first := detail.Items[0]
	if first.ItemID == 0 || first.ReviewStatus != pipeline.ReviewStatusUnreviewed {
		t.Errorf("first item handle/status = %+v, want 外部句柄 + unreviewed", first)
	}
	if first.Panel != "cbc" {
		t.Errorf("first item panel = %q, want cbc", first.Panel)
	}
	if len(first.OcrLines) != 4 || first.OcrLines[2].Line != 3 || first.OcrLines[2].Box != apiBox(313, 74) {
		t.Errorf("first item ocr_lines = %+v, want 识别端下标直映的行链接", first.OcrLines)
	}
	if first.Prev != nil {
		t.Errorf("prev = %+v, want null (M3 填充面)", first.Prev)
	}

	// 图像地址:详情 report_id 即图像内容身份,同源可直出(契约 GET /images/{sha256})。
	imgStatus, imgRaw := doJSON(t, srv, http.MethodGet, "/api/v1/images/"+sha, "")
	if imgStatus != http.StatusOK || !bytes.Equal(imgRaw, image) {
		t.Errorf("GET /images/%s = %d (%d bytes), want 200 + 原图字节", sha, imgStatus, len(imgRaw))
	}

	// 未知报告与非法形态:404(非法形态无法是任何已有报告,与不存在同答)。
	for _, id := range []string{strings.Repeat("f", 64), "not-a-sha", "abc"} {
		if status, _ = doJSON(t, srv, http.MethodGet, "/api/v1/reports/"+id, ""); status != http.StatusNotFound {
			t.Errorf("GET /reports/%s = %d, want 404", id, status)
		}
	}

	// 损坏态防御:越界下标跳过不炸整页(链接空数组,raw_text 保留;结构完整性由 e2e 度量兜底)。
	st.seedReport(strings.Repeat("d", 64), nil, contract.Report{ReportType: "血常规", Status: contract.StatusSuccess, Items: []contract.ReportItem{
		{Name: "血红蛋白", Value: str("128"), Flag: "normal", RawText: "血红蛋白  128", OCRLineNumbers: []int32{0, 999}},
	}}, nil)
	status, raw = doJSON(t, srv, http.MethodGet, "/api/v1/reports/"+strings.Repeat("d", 64), "")
	if status != http.StatusOK {
		t.Fatalf("GET corrupted detail = %d; body = %s", status, raw)
	}
	var corrupted httpapi.ReportDetail
	if err := json.Unmarshal(raw, &corrupted); err != nil {
		t.Fatal(err)
	}
	if len(corrupted.Items) != 1 || len(corrupted.Items[0].OcrLines) != 0 || corrupted.Items[0].RawText != "血红蛋白  128" {
		t.Errorf("corrupted indices = %+v, want 空链接 + raw_text 保留", corrupted.Items[0])
	}
}

// TestServeGetReportDetailSections 影像单详情:可选 sections 经上传响应与 GET 详情
// 同一投影返回,行号映射为行链接;非影像报告不输出 sections(上一用例)。
func TestServeGetReportDetailSections(t *testing.T) {
	srv, _, rec, _ := newReportsTestServer(t)
	createOne(t, srv, `{"name":"小虎","is_child":true}`)
	rec.report = &contract.Report{
		ReportType: contract.ReportTypeImaging, ReportDate: str("2024-09-15"), Status: contract.StatusSuccess,
		Sections: []contract.ReportSection{
			{Kind: "exam_name", Text: "产科胎儿彩超检查（中孕）", OCRLineNumbers: []int32{1}},
			{Kind: "impression", Text: "宫内单活胎。", OCRLineNumbers: []int32{4}},
		},
	}

	status, raw := doUpload(t, srv, []byte("fake-imaging-bytes"), map[string]string{formMemberID: "1"})
	created := decodeUpload(t, status, raw, http.StatusCreated)
	sha := created.Report.ReportID
	checkSections := func(label string, detail httpapi.ReportDetail) {
		t.Helper()
		if len(detail.Sections) != 2 {
			t.Fatalf("%s sections = %+v, want 2", label, detail.Sections)
		}
		if detail.Sections[0].Kind != "exam_name" || detail.Sections[0].Text != "产科胎儿彩超检查（中孕）" ||
			len(detail.Sections[0].OcrLines) != 1 || detail.Sections[0].OcrLines[0].Line != 1 {
			t.Errorf("%s exam_name section = %+v", label, detail.Sections[0])
		}
		if detail.Sections[1].Kind != "impression" || detail.Sections[1].Text != "宫内单活胎。" ||
			len(detail.Sections[1].OcrLines) != 1 || detail.Sections[1].OcrLines[0].Line != 4 {
			t.Errorf("%s impression section = %+v", label, detail.Sections[1])
		}
		if len(detail.Items) != 0 {
			t.Errorf("%s items = %+v, want 0 (影像单零指标项)", label, detail.Items)
		}
	}
	checkSections("upload", created.Report)

	status, raw = doJSON(t, srv, http.MethodGet, "/api/v1/reports/"+sha, "")
	if status != http.StatusOK {
		t.Fatalf("GET detail = %d; body = %s", status, raw)
	}
	var detail httpapi.ReportDetail
	if err := json.Unmarshal(raw, &detail); err != nil {
		t.Fatal(err)
	}
	checkSections("detail", detail)
}

// TestServeGetReportDetailFailed failed 报告详情:图像与失败原因可见(无可核对项),
// items 序列化为空数组、计数全 0、fail_reason 透传。
func TestServeGetReportDetailFailed(t *testing.T) {
	srv, _, rec, _ := newReportsTestServer(t)
	createOne(t, srv, `{"name":"小虎","is_child":true}`)
	rec.report = &contract.Report{ReportType: "其他专项", Status: contract.StatusFailed}

	status, raw := doUpload(t, srv, []byte("blank-photo"), map[string]string{formMemberID: "1"})
	created := decodeUpload(t, status, raw, http.StatusCreated)
	sha := created.Report.ReportID

	status, raw = doJSON(t, srv, http.MethodGet, "/api/v1/reports/"+sha, "")
	if status != http.StatusOK {
		t.Fatalf("GET detail = %d; body = %s", status, raw)
	}
	var detail httpapi.ReportDetail
	if err := json.Unmarshal(raw, &detail); err != nil {
		t.Fatal(err)
	}
	if detail.Status != contract.StatusFailed || detail.FailReason == nil || *detail.FailReason != "no metric items recognized" {
		t.Errorf("failed detail = %+v, want fail_reason 可见", detail)
	}
	if len(detail.Items) != 0 || detail.UnreviewedLowConfidenceCount != 0 {
		t.Errorf("failed items/count = %d/%d, want 0/0(无可核对项、不计待核对)", len(detail.Items), detail.UnreviewedLowConfidenceCount)
	}
	if !strings.Contains(string(raw), `"items":[]`) {
		t.Errorf("failed items must serialize as [] not null; body = %s", raw)
	}
}

// doReassign 发一次改归属请求(PATCH /reports/{id};body 原始 JSON 以覆盖畸形形态)。
func doReassign(t *testing.T, srv *httptest.Server, reportID, body string) (int, []byte) {
	t.Helper()
	return doJSON(t, srv, http.MethodPatch, "/api/v1/reports/"+reportID, body)
}

// decodeReassign 断言状态码并解码 ReportSummary(改归属响应 = 刷新后的报告头部)。
func decodeReassign(t *testing.T, status int, raw []byte, wantStatus int) httpapi.ReportSummary {
	t.Helper()
	if status != wantStatus {
		t.Fatalf("reassign status = %d, want %d; body = %s", status, wantStatus, raw)
	}
	var got httpapi.ReportSummary
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("decode ReportSummary: %v; body = %s", err, raw)
	}
	return got
}

// timelineShas 时间线(?member_id=)的 report_id 序列;memberID 空串 = 全量。
func timelineShas(t *testing.T, srv *httptest.Server, memberID string) []string {
	t.Helper()
	path := "/api/v1/reports"
	if memberID != "" {
		path += "?member_id=" + memberID
	}
	status, raw := doJSON(t, srv, http.MethodGet, path, "")
	if status != http.StatusOK {
		t.Fatalf("GET %s = %d; body = %s", path, status, raw)
	}
	var list httpapi.ReportsList
	if err := json.Unmarshal(raw, &list); err != nil {
		t.Fatalf("decode timeline %s: %v", raw, err)
	}
	out := make([]string, 0, len(list.Reports))
	for _, r := range list.Reports {
		out = append(out, r.ReportID)
	}
	return out
}

// TestServeReassignReport 改归属(票 10):归属即刻生效、时间线随成员过滤移位;
// 报告内容零变化(状态/计数/指标项与核对状态原样);重复改派幂等;未知成员 404 零写入。
func TestServeReassignReport(t *testing.T) {
	srv, _, _, _ := newReportsTestServer(t)
	createOne(t, srv, `{"name":"小虎","is_child":true}`)  // id 1
	createOne(t, srv, `{"name":"妈妈","is_child":false}`) // id 2

	status, raw := doUpload(t, srv, []byte("fake-jpeg-reassign-bytes-01"), map[string]string{formMemberID: "1"})
	created := decodeUpload(t, status, raw, http.StatusCreated)
	sha := created.Report.ReportID
	if got := timelineShas(t, srv, "1"); len(got) != 1 || got[0] != sha {
		t.Fatalf("timeline(1) before = %v, want [%s]", got, sha)
	}
	if got := timelineShas(t, srv, "2"); len(got) != 0 {
		t.Fatalf("timeline(2) before = %v, want 空", got)
	}

	// 传错成员改归属:归属即刻生效、时间线移位(原成员无、目标成员有)。
	status, raw = doReassign(t, srv, sha, `{"member_id":2}`)
	got := decodeReassign(t, status, raw, http.StatusOK)
	if got.MemberID == nil || *got.MemberID != 2 {
		t.Fatalf("reassigned member_id = %v, want 2", got.MemberID)
	}
	if got.Status != created.Report.Status || got.ReportType != created.Report.ReportType ||
		got.FlagCounts != created.Report.FlagCounts ||
		got.UnreviewedLowConfidenceCount != created.Report.UnreviewedLowConfidenceCount ||
		!sameOptionalString(got.ReportDate, created.Report.ReportDate) || !sameOptionalString(got.FailReason, created.Report.FailReason) {
		t.Errorf("reassign response = %+v, want 除归属外零变化(上传响应 %+v)", got.ReportHeader, created.Report.ReportHeader)
	}
	if got := timelineShas(t, srv, "1"); len(got) != 0 {
		t.Errorf("timeline(1) after = %v, want 空(报告已移走)", got)
	}
	if got := timelineShas(t, srv, "2"); len(got) != 1 || got[0] != sha {
		t.Errorf("timeline(2) after = %v, want [%s]", got, sha)
	}

	// 详情同步:归属更新、报告内容零变化(指标项句柄与核对状态原样)。
	status, raw = doJSON(t, srv, http.MethodGet, "/api/v1/reports/"+sha, "")
	if status != http.StatusOK {
		t.Fatalf("GET detail = %d; body = %s", status, raw)
	}
	var detail httpapi.ReportDetail
	if err := json.Unmarshal(raw, &detail); err != nil {
		t.Fatal(err)
	}
	if detail.MemberID == nil || *detail.MemberID != 2 {
		t.Errorf("detail member_id = %v, want 2", detail.MemberID)
	}
	if len(detail.Items) != len(created.Report.Items) || detail.Items[0].ItemID != created.Report.Items[0].ItemID ||
		detail.Items[0].ReviewStatus != pipeline.ReviewStatusUnreviewed {
		t.Errorf("detail items after reassign = %+v, want 与上传时逐项一致", detail.Items)
	}

	// 重复改派(同成员)幂等:200 且归属不变。
	status, raw = doReassign(t, srv, sha, `{"member_id":2}`)
	if again := decodeReassign(t, status, raw, http.StatusOK); again.MemberID == nil || *again.MemberID != 2 {
		t.Errorf("replayed reassign member_id = %v, want 2", again.MemberID)
	}

	// 未知成员 404 零写入(归属保持 2)。
	status, raw = doReassign(t, srv, sha, `{"member_id":999}`)
	if status != http.StatusNotFound {
		t.Fatalf("reassign to unknown member = %d; body = %s, want 404", status, raw)
	}
	assertAPIError(t, raw)
	status, raw = doJSON(t, srv, http.MethodGet, "/api/v1/reports/"+sha, "")
	if status != http.StatusOK {
		t.Fatalf("GET detail = %d", status)
	}
	if err := json.Unmarshal(raw, &detail); err != nil {
		t.Fatal(err)
	}
	if detail.MemberID == nil || *detail.MemberID != 2 {
		t.Errorf("member_id after rejected reassign = %v, want 2(零写入)", detail.MemberID)
	}

	// 改回原成员:时间线随之移回(改归属不是单向操作)。
	status, raw = doReassign(t, srv, sha, `{"member_id":1}`)
	if back := decodeReassign(t, status, raw, http.StatusOK); back.MemberID == nil || *back.MemberID != 1 {
		t.Fatalf("reassign back member_id = %v, want 1", back.MemberID)
	}
	if got := timelineShas(t, srv, "1"); len(got) != 1 || got[0] != sha {
		t.Errorf("timeline(1) after reassign back = %v, want [%s]", got, sha)
	}
	if got := timelineShas(t, srv, "2"); len(got) != 0 {
		t.Errorf("timeline(2) after reassign back = %v, want 空", got)
	}
}

// reassignRaceStore 模拟改归属的竞态窗口:摘要读出后报告被并发删除——
// SetReportMember 恒回「未命中」;只覆写写入路径,读路径仍走内存 fake。
type reassignRaceStore struct{ *fakeServeStore }

func (reassignRaceStore) SetReportMember(context.Context, string, int64) (bool, error) {
	return false, nil
}

// TestServeReassignConcurrentDelete 竞态收口:报告在摘要读出后被并发删除时,
// 改归属以路径身份口径回 404(而非 500「assign member failed」)。
func TestServeReassignConcurrentDelete(t *testing.T) {
	st := newFakeServeStore()
	sha := strings.Repeat("a", 64)
	st.seedReport(sha, nil, contract.Report{ReportType: "血常规", Status: contract.StatusSuccess, Items: []contract.ReportItem{
		{Name: "血红蛋白", Value: str("128"), Flag: "normal", RawText: "血红蛋白  128"},
	}}, nil)
	d := &deps{store: st, members: &fakeMemberStore{}, reports: reassignRaceStore{st},
		rec: &fakeUploadRecognizer{}, obj: fakeObjectsCLI{objects: map[string][]byte{}}}
	srv := httptest.NewServer(newServeHandler(d))
	t.Cleanup(srv.Close)
	createOne(t, srv, `{"name":"小虎","is_child":true}`)

	status, raw := doReassign(t, srv, sha, `{"member_id":1}`)
	if status != http.StatusNotFound {
		t.Fatalf("reassign during concurrent delete = %d %s, want 404", status, raw)
	}
	assertAPIError(t, raw)
}

// TestServeReassignValidation 改归属校验与 404 矩阵:非法请求 4xx 且零写入;
// report_id 非 hex → 404(内容身份口径)、未知报告/成员 → 404。
func TestServeReassignValidation(t *testing.T) {
	cases := []struct {
		name string
		body string
		want int
		sha  func(sample httpapi.ReportDetail) string // 缺省 = 样例报告
	}{
		{name: "empty body", body: "", want: http.StatusBadRequest},
		{name: "missing member", body: `{}`, want: http.StatusBadRequest},
		{name: "null member", body: `{"member_id":null}`, want: http.StatusBadRequest},
		{name: "string member", body: `{"member_id":"abc"}`, want: http.StatusBadRequest},
		{name: "float member", body: `{"member_id":1.5}`, want: http.StatusBadRequest},
		{name: "unknown field rejected", body: `{"member_id":1,"note":"x"}`, want: http.StatusBadRequest},
		{name: "non-hex report", body: `{"member_id":1}`, want: http.StatusNotFound,
			sha: func(_ httpapi.ReportDetail) string { return "not-a-sha" }},
		{name: "unknown report", body: `{"member_id":1}`, want: http.StatusNotFound,
			sha: func(_ httpapi.ReportDetail) string { return strings.Repeat("f", 64) }},
		{name: "unknown member", body: `{"member_id":999}`, want: http.StatusNotFound},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			srv, _, _, _ := newReportsTestServer(t)
			createOne(t, srv, `{"name":"小虎","is_child":true}`)
			status, raw := doUpload(t, srv, []byte("fake-jpeg-reassign-validation"), map[string]string{formMemberID: "1"})
			sample := decodeUpload(t, status, raw, http.StatusCreated).Report
			sha := sample.ReportID
			if c.sha != nil {
				sha = c.sha(sample)
			}
			status, raw = doReassign(t, srv, sha, c.body)
			if status != c.want {
				t.Fatalf("status = %d, want %d; body = %s", status, c.want, raw)
			}
			assertAPIError(t, raw)
			// 零写入:样例报告归属未动。
			status, raw = doJSON(t, srv, http.MethodGet, "/api/v1/reports/"+sample.ReportID, "")
			if status != http.StatusOK {
				t.Fatalf("GET sample detail = %d", status)
			}
			var detail httpapi.ReportDetail
			if err := json.Unmarshal(raw, &detail); err != nil {
				t.Fatal(err)
			}
			if detail.MemberID == nil || *detail.MemberID != 1 {
				t.Errorf("member_id after rejected request = %v, want 1", detail.MemberID)
			}
		})
	}
}

// TestServeDeleteReport 删除(票 10):204 无响应体;报告、指标项、修正对随之清除;
// 原始图像与 OCR 历史留痕保留;删除后同图重传走新归档(201、非 duplicate、
// 指标项句柄全换、识别重跑且 OCR 历史追加);重复删除与非法形态 404。
func TestServeDeleteReport(t *testing.T) {
	srv, st, rec, _ := newReportsTestServer(t)
	createOne(t, srv, `{"name":"小虎","is_child":true}`)

	image := []byte("fake-jpeg-delete-bytes-01")
	status, raw := doUpload(t, srv, image, map[string]string{formMemberID: "1"})
	created := decodeUpload(t, status, raw, http.StatusCreated)
	sha := created.Report.ReportID
	oldItems := created.Report.Items

	// 先修正低置信项:产生一条修正对(删除级联的断言底;同时翻 partial → success)。
	status, raw = doReview(t, srv, sha, oldItems[0].ItemID, `{"action":"correct","value":"5.6"}`)
	if got := decodeReview(t, status, raw, http.StatusOK); got.Status != contract.StatusSuccess {
		t.Fatalf("review → %+v, want success", got)
	}
	if len(st.pairsSnapshot()) != 1 {
		t.Fatalf("pairs before delete = %d, want 1", len(st.pairsSnapshot()))
	}

	// DELETE → 204 无响应体。
	status, raw = doJSON(t, srv, http.MethodDelete, "/api/v1/reports/"+sha, "")
	if status != http.StatusNoContent || len(bytes.TrimSpace(raw)) != 0 {
		t.Fatalf("DELETE report = %d %q, want 204 无响应体", status, raw)
	}

	// 报告、指标项、修正对随之清除:详情 404、时间线消失、级联计数归零。
	if status, _ = doJSON(t, srv, http.MethodGet, "/api/v1/reports/"+sha, ""); status != http.StatusNotFound {
		t.Errorf("GET deleted detail = %d, want 404", status)
	}
	if got := timelineShas(t, srv, "1"); len(got) != 0 {
		t.Errorf("timeline after delete = %v, want 空", got)
	}
	if got := timelineShas(t, srv, ""); len(got) != 0 {
		t.Errorf("all timeline after delete = %v, want 空", got)
	}
	if st.reportCount() != 0 {
		t.Errorf("reports after delete = %d, want 0", st.reportCount())
	}
	if items, err := st.ItemsWithReviewStatus(context.Background(), sha); err != nil || len(items) != 0 {
		t.Errorf("items after delete = %+v (%v), want 级联清除", items, err)
	}
	if pairs := st.pairsSnapshot(); len(pairs) != 0 {
		t.Errorf("pairs after delete = %d, want 0(随指标项级联)", len(pairs))
	}

	// 原始图像与 OCR 历史留痕保留(append-only 文化不破)。
	if !st.hasImage(sha) {
		t.Error("image row removed, want 原始图像留痕")
	}
	if n := st.ocrCount(sha); n != 1 {
		t.Errorf("ocr history after delete = %d, want 1(识别历史保留)", n)
	}

	// 删除后同图重传:201 新报告(内容哈希不再命中、非 duplicate),指标项句柄全换、
	// 核对状态回 unreviewed;识别重跑,OCR 历史追加(NF-05)。
	status, raw = doUpload(t, srv, image, map[string]string{formMemberID: "1"})
	again := decodeUpload(t, status, raw, http.StatusCreated)
	if again.Duplicate {
		t.Error("duplicate = true after delete, want false(内容哈希不再命中)")
	}
	if again.Report.ReportID != sha {
		t.Errorf("re-upload report_id = %q, want 同图身份 %q", again.Report.ReportID, sha)
	}
	if again.Report.Status != contract.StatusPartial || again.Report.UnreviewedLowConfidenceCount != 1 {
		t.Errorf("re-upload header = %+v, want 重新识别归档(partial/1)", again.Report.ReportHeader)
	}
	oldIDs := map[int64]bool{}
	for _, it := range oldItems {
		oldIDs[it.ItemID] = true
	}
	for _, it := range again.Report.Items {
		if oldIDs[it.ItemID] {
			t.Errorf("re-upload reused item_id %d, want 全换", it.ItemID)
		}
		if it.ReviewStatus != pipeline.ReviewStatusUnreviewed {
			t.Errorf("re-upload item review_status = %q, want unreviewed", it.ReviewStatus)
		}
	}
	if rec.calls != 2 {
		t.Errorf("recognizer calls = %d, want 2(删除后重传重跑识别)", rec.calls)
	}
	if n := st.ocrCount(sha); n != 2 {
		t.Errorf("ocr history after re-upload = %d, want 2(历史追加不覆盖)", n)
	}
	if pairs := st.pairsSnapshot(); len(pairs) != 0 {
		t.Errorf("pairs after re-upload = %d, want 0(新报告不携带旧修正对)", len(pairs))
	}
	if got := timelineShas(t, srv, "1"); len(got) != 1 || got[0] != sha {
		t.Errorf("timeline after re-upload = %v, want [%s]", got, sha)
	}

	// 重复删除(先删掉重传的新报告)与非法形态:404(契约只声明 404)。
	if status, raw = doJSON(t, srv, http.MethodDelete, "/api/v1/reports/"+sha, ""); status != http.StatusNoContent {
		t.Fatalf("DELETE re-uploaded report = %d %s, want 204", status, raw)
	}
	for _, id := range []string{sha, strings.Repeat("f", 64), "not-a-sha"} {
		if status, raw = doJSON(t, srv, http.MethodDelete, "/api/v1/reports/"+id, ""); status != http.StatusNotFound {
			t.Errorf("DELETE %s = %d %s, want 404", id, status, raw)
		}
	}
}

// assertAPIError 非 2xx 响应体必须是契约 Error 形状(message 非空)。
func assertAPIError(t *testing.T, raw []byte) {
	t.Helper()
	var apiErr httpapi.Error
	if err := json.Unmarshal(raw, &apiErr); err != nil || apiErr.Message == "" {
		t.Errorf("error body = %s, want 契约 Error 形状", raw)
	}
}

// sameOptionalString 两个可空字符串相等(nil 与 nil、指针与同值;日期/失败原因等可空字段比较用)。
func sameOptionalString(a, b *string) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}
