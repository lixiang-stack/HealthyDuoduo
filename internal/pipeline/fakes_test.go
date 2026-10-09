// pipeline 单测:编排/去重/补录 --force 行为,fake 掉 store/识别服务/对象存储。
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

	images  map[string]*pipeline.Image
	nextID  int64
	ocrRows []fakeOCR
	reports map[int64]*fakeReport
	listRet []pipeline.ReportRow
}

type fakeOCR struct {
	id    int64
	imgID int64
	raw   []byte
}

type fakeReport struct {
	row pipeline.ReportRow
	rep contract.Report
}

func newFakeStore() *fakeStore {
	return &fakeStore{images: map[string]*pipeline.Image{}, reports: map[int64]*fakeReport{}, nextID: 0}
}

func (f *fakeStore) ImageBySha256(ctx context.Context, sha string) (*pipeline.Image, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	img := f.images[sha]
	if img == nil {
		return nil, nil
	}
	c := *img
	return &c, nil
}

func (f *fakeStore) CreateImage(ctx context.Context, sha, objectKey, filename string) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.nextID++
	id := f.nextID
	f.images[sha] = &pipeline.Image{ID: id, Sha256: sha, ObjectKey: objectKey, OriginalFilename: filename}
	return id, nil
}

func (f *fakeStore) AppendOCRResult(ctx context.Context, imageID int64, raw []byte) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.nextID++
	f.ocrRows = append(f.ocrRows, fakeOCR{id: f.nextID, imgID: imageID, raw: raw})
	return f.nextID, nil
}

func (f *fakeStore) ReportByImage(ctx context.Context, imageID int64) (*pipeline.ReportRow, bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for id := range f.reports {
		if f.reports[id].row.ImageID == imageID {
			r := f.reports[id].row
			return &r, true, nil
		}
	}
	return nil, false, nil
}

func (f *fakeStore) ReportByID(ctx context.Context, id int64) (*pipeline.ReportRow, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if r, ok := f.reports[id]; ok {
		c := r.row
		return &c, nil
	}
	return nil, fmt.Errorf("repo %d missing", id)
}

func (f *fakeStore) ListReports(ctx context.Context, reportType, date string) ([]pipeline.ReportRow, error) {
	return f.listRet, nil
}

func (f *fakeStore) ItemsByReport(ctx context.Context, reportID int64) ([]contract.ReportItem, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if r, ok := f.reports[reportID]; ok {
		return r.rep.Items, nil
	}
	return nil, fmt.Errorf("items %d missing", reportID)
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

func (f *fakeStore) WriteReport(ctx context.Context, reportID, imageID, ocrID int64, report contract.Report) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if reportID == 0 {
		f.nextID++
		reportID = f.nextID
	}
	date := ""
	if report.ReportDate != nil {
		date = *report.ReportDate
	}
	f.reports[reportID] = &fakeReport{
		row: pipeline.ReportRow{ID: reportID, ImageID: imageID, OCRResultID: ocrID, ReportType: report.ReportType,
			ReportDate: newIf(date), Status: report.Status},
		rep: report,
	}
	return reportID, nil
}

func newIf(s string) *string { return &s }

type fakeRecognizer struct {
	mu sync.Mutex

	reportCalls  int
	reparseCalls int

	imageOfCall map[int][]byte
	dateOfCall  map[int]string
	report      contract.Report
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
	return sampleOCR(), &rep, nil
}

func (f *fakeRecognizer) Reparse(ctx context.Context, ocr *contract.OCRResult, date string) (*contract.Report, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.reparseCalls++
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
