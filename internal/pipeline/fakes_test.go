package pipeline_test

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"healthyduoduo/internal/contract"
	"healthyduoduo/internal/pipeline"
)

func sampleReport(date string) contract.Report {
	v := 128.0
	unit, ref := "g/L", "115-150"
	items := []contract.ReportItem{{
		Name: "血红蛋白", Value: &v, Unit: &unit, RefRange: &ref,
		Flag: "normal", LowConfidence: false, RawText: "血红蛋白 128 g/L 115-150",
	}}
	var d *string
	if date != "" {
		d = &date
	}
	return contract.Report{ReportType: "血常规", ReportDate: d, Status: "success", Items: items}
}

func sampleOCR() *contract.OCRResult {
	return &contract.OCRResult{Txts: []string{"血红蛋白", "128"}, Engine: "onnxruntime"}
}

type fakeStore struct {
	mu sync.Mutex

	nextID  int64
	images  map[string]bool // 已入库的 sha256 集合(identity 即主键)
	ocrRows []fakeOCR
	reports map[string]*fakeReport
	listRet []pipeline.ReportRow
}

type fakeOCR struct {
	id     int64
	imgSha string
	raw    []byte
}

type fakeReport struct {
	row pipeline.ReportRow
	rep contract.Report
}

func newFakeStore() *fakeStore {
	return &fakeStore{images: map[string]bool{}, reports: map[string]*fakeReport{}, nextID: 0}
}

func (f *fakeStore) ImageBySha256(ctx context.Context, sha string) (*pipeline.Image, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if !f.images[sha] {
		return nil, nil
	}
	return &pipeline.Image{Sha256: sha}, nil
}

func (f *fakeStore) CreateImage(ctx context.Context, sha, filename string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.images[sha] = true
	return nil
}

func (f *fakeStore) AppendOCRResult(ctx context.Context, imageSha string, raw []byte) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.nextID++
	f.ocrRows = append(f.ocrRows, fakeOCR{id: f.nextID, imgSha: imageSha, raw: raw})
	return f.nextID, nil
}

func (f *fakeStore) ReportByImage(ctx context.Context, imageSha string) (*pipeline.ReportRow, bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if r, ok := f.reports[imageSha]; ok {
		c := r.row
		return &c, true, nil
	}
	return nil, false, nil
}

func (f *fakeStore) ReportByID(ctx context.Context, id int64) (*pipeline.ReportRow, error) {
	return nil, fmt.Errorf("repo %d missing", id)
}

func (f *fakeStore) ListReports(ctx context.Context, reportType, date string) ([]pipeline.ReportRow, error) {
	return f.listRet, nil
}

func (f *fakeStore) ItemsByImage(ctx context.Context, reportSha string) ([]contract.ReportItem, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if r, ok := f.reports[reportSha]; ok {
		return r.rep.Items, nil
	}
	return nil, fmt.Errorf("items %s missing", reportSha)
}

func (f *fakeStore) OCRResultJSON(ctx context.Context, ocrID int64) ([]byte, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, o := range f.ocrRows {
		if o.id == ocrID {
			return o.raw, nil
		}
	}
	return nil, fmt.Errorf("ocr %d missing", ocrID)
}

func (f *fakeStore) WriteReport(ctx context.Context, reportSha string, ocrID int64, report contract.Report) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	date := ""
	if report.ReportDate != nil {
		date = *report.ReportDate
	}
	f.reports[reportSha] = &fakeReport{
		row: pipeline.ReportRow{Sha256: reportSha, OCRResultID: ocrID, ReportType: report.ReportType,
			ReportDate: ptrIf(date), Status: report.Status},
		rep: report,
	}
	return nil
}

func ptrIf(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

type fakeRecognizer struct {
	mu sync.Mutex

	reportCalls  int
	reparseCalls int

	imageOfCall     map[int][]byte
	dateOfCall      map[int]string
	lastReparseHint *string
	report          contract.Report
}

func newFakeRecognizer() *fakeRecognizer {
	return &fakeRecognizer{imageOfCall: map[int][]byte{}, dateOfCall: map[int]string{}, report: sampleReport("2024-08-19")}
}

func (f *fakeRecognizer) Report(ctx context.Context, image []byte, filename, date string) (*contract.OCRResult, *contract.Report, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.reportCalls++
	f.imageOfCall[f.reportCalls] = image
	f.dateOfCall[f.reportCalls] = date
	rep := f.report
	if date != "" {
		// 与真实识别服务一致:date 作为解析失败时的人工补录兜底生效
		d := date
		rep.ReportDate = &d
		rep.Status = "success"
	}
	return sampleOCR(), &rep, nil
}

func (f *fakeRecognizer) Reparse(ctx context.Context, ocr *contract.OCRResult, date string) (*contract.Report, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.reparseCalls++
	f.lastReparseHint = &date
	rep := f.report
	if date != "" {
		d := date
		rep.ReportDate = &d
		rep.Status = "success"
	}
	return &rep, nil
}

type fakeObjects struct{ puts map[string][]byte }

func newObjects() *fakeObjects { return &fakeObjects{puts: map[string][]byte{}} }

func (f *fakeObjects) Put(ctx context.Context, key string, data []byte) error {
	f.puts[key] = data
	return nil
}

var _ pipeline.Recognizer = (*fakeRecognizer)(nil)
var _ pipeline.Store = (*fakeStore)(nil)
var _ pipeline.ObjectStore = (*fakeObjects)(nil)

func ingest(t *testing.T, st pipeline.Store, rec pipeline.Recognizer, obj pipeline.ObjectStore, path, date string, force bool) *pipeline.IngestOutcome {
	t.Helper()
	oc, err := pipeline.Ingest(context.Background(), st, rec, obj, path, date, force)
	if err != nil {
		t.Fatalf("ingest %s: %v", path, err)
	}
	return oc
}
