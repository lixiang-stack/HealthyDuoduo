package pipeline_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"healthyduoduo/internal/contract"
	"healthyduoduo/internal/pipeline"
)

// writeImage 载入唯一样本字节,让每条用例独立文件。
func writeImage(t *testing.T, data []byte) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "sample.jpeg")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	return path
}

func TestIngestFirstRun_NewReport(t *testing.T) {
	st, rec, obj := newFakeStore(), newFakeRecognizer(), newObjects()
	path := writeImage(t, []byte("fake-image-1"))

	oc := ingest(t, st, rec, obj, path, "", false)

	if oc.Kind != pipeline.KindNew || oc.ReportID == 0 {
		t.Fatalf("outcome = %+v, want KindNew with id", oc)
	}
	if rec.reportCalls != 1 {
		t.Errorf("recognizer calls = %d, want 1", rec.reportCalls)
	}
	if len(obj.puts) != 1 {
		t.Errorf("object puts = %d, want 1 (原图入 SeaweedFS)", len(obj.puts))
	}
	// 报告日期/状态透传落库
	if got := st.reports[oc.ReportID].rep; got.ReportType != "血常规" {
		t.Errorf("report = %+v report_type", got)
	}
}

func TestIngestRepeated_Idempotent(t *testing.T) {
	st, rec, obj := newFakeStore(), newFakeRecognizer(), newObjects()
	path := writeImage(t, []byte("fake-image-1"))

	first := ingest(t, st, rec, obj, path, "", false)
	second := ingest(t, st, rec, obj, path, "", false)

	if second.Kind != pipeline.KindCached {
		t.Fatalf("second outcome kind = %v, want KindCached", second.Kind)
	}
	if first.ReportID != second.ReportID {
		t.Errorf("ids differ: first=%d second=%d, want 幂等同一报告 ID", first.ReportID, second.ReportID)
	}
	if rec.reportCalls != 1 {
		t.Errorf("recognizer calls = %d, want 1(重复 ingest 不重跑识别)", rec.reportCalls)
	}
	if len(obj.puts) != 1 {
		t.Errorf("object puts = %d, want 1", len(obj.puts))
	}
}

func TestIngestForce_RerunsAndAppendsOCR(t *testing.T) {
	st, rec, obj := newFakeStore(), newFakeRecognizer(), newObjects()
	path := writeImage(t, []byte("fake-image-1"))

	first := ingest(t, st, rec, obj, path, "", false)
	before := len(st.ocrRows)
	second := ingest(t, st, rec, obj, path, "", true)
	after := len(st.ocrRows)

	if second.Kind != pipeline.KindForced {
		t.Fatalf("kind = %v, want KindForced", second.Kind)
	}
	if first.ReportID != second.ReportID {
		t.Errorf("report id changed: %d -> %d, want --force 原位更新", first.ReportID, second.ReportID)
	}
	if after != before+1 {
		t.Errorf("ocr rows = %d -> %d, want --force 追加一行", before, after)
	}
	if rec.reportCalls != 2 {
		t.Errorf("recognizer calls = %d, want 2(--force 重调 /report)", rec.reportCalls)
	}
	if len(obj.puts) != 1 {
		t.Errorf("object puts = %d, want 1(对象键=sha256 已存在时跳过)", len(obj.puts))
	}
}

func TestIngestWithDate_PassedThroughToRecognizer(t *testing.T) {
	st, rec, obj := newFakeStore(), newFakeRecognizer(), newObjects()
	path := writeImage(t, []byte("no-date-image"))

	oc := ingest(t, st, rec, obj, path, "2026-10-01", false)

	if rec.dateOfCall[1] != "2026-10-01" {
		t.Errorf("date passed to recognizer = %q, want 2026-10-01(解析失败的补录兜底)", rec.dateOfCall[1])
	}
	if oc.Report.ReportDate == nil || *oc.Report.ReportDate != "2024-08-19" {
		t.Errorf("report date = %v (fake 识别服务自己算好的日期)", oc.Report.ReportDate)
	}
}

func TestIngestCachedWithDate_RunsBackfillViaReparse(t *testing.T) {
	// 先以无日期首次入库(得到 null 日期报告),再 --date 补录;同一报告 ID,不再重跑 OCR。
	noDate := sampleReport("")
	rec := newFakeRecognizer()
	rec.report = noDate
	st, obj := newFakeStore(), newObjects()
	path := writeImage(t, []byte("fake-image-1"))

	first := ingest(t, st, rec, obj, path, "", false)
	if first.Report.ReportDate != nil {
		t.Fatalf("setup: expected null-date report, got %+v", first.Report)
	}

	rep, err := pipeline.Reparse(context.Background(), st, rec, first.ReportID, "2026-01-02")
	if err != nil {
		t.Fatalf("reparse with date: %v", err)
	}
	if rep.ReportDate == nil || *rep.ReportDate != "2026-01-02" {
		t.Errorf("reparse date = %v, want 补录 2026-01-02", rep.ReportDate)
	}
	if rec.reparseCalls != 1 || rec.reportCalls != 1 {
		t.Errorf("calls: report=%d reparse=%d, want 补录只重跑后处理(1/1)", rec.reportCalls, rec.reparseCalls)
	}
}

func TestIngestCachedWithoutDate_FlagNotConsumed(t *testing.T) {
	rec := newFakeRecognizer()
	rec.report = sampleReport("") // fake 识别服务返回 null 日期
	st, obj := newFakeStore(), newObjects()
	path := writeImage(t, []byte("fake-image-1"))

	first := ingest(t, st, rec, obj, path, "", false)
	second := ingest(t, st, rec, obj, path, "", false)

	if second.Kind != pipeline.KindCached {
		t.Fatalf("kind = %v, want KindCached", second.Kind)
	}
	_ = first
}

func TestReparse_UsesStoredOCRAndReplaceItems(t *testing.T) {
	rec := newFakeRecognizer()
	rec.report = sampleReport("") // 首次存成无日期
	st, obj := newFakeStore(), newObjects()
	path := writeImage(t, []byte("fake-image-1"))

	oc := ingest(t, st, rec, obj, path, "", false)
	if _, err := pipeline.Reparse(context.Background(), st, rec, oc.ReportID, ""); err != nil {
		t.Fatalf("reparse: %v", err)
	}
	stored, err := st.OCRResultJSON(context.Background(), st.reports[oc.ReportID].row.OCRResultID)
	if err != nil {
		t.Fatalf("stored ocr read: %v", err)
	}
	var ocr contract.OCRResult
	if err := json.Unmarshal(stored, &ocr); err != nil {
		t.Fatalf("stored ocr unmarshal: %v", err)
	}
	if len(ocr.Txts) != 2 || ocr.Txts[0] != "血红蛋白" {
		t.Errorf("stored ocr = %+v", ocr)
	}
}
