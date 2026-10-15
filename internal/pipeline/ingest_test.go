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

	if oc.Kind != pipeline.KindNew || oc.Sha256 == "" {
		t.Fatalf("outcome = %+v, want KindNew with sha identity", oc)
	}
	if rec.reportCalls != 1 {
		t.Errorf("recognizer calls = %d, want 1", rec.reportCalls)
	}
	if len(obj.puts) != 1 {
		t.Errorf("object puts = %d, want 1 (原图入 SeaweedFS;键=sha256)", len(obj.puts))
	}
	if _, ok := obj.puts[oc.Sha256]; !ok {
		t.Errorf("object key %q, want 对象键=图像 sha256", oc.Sha256)
	}
	// 报告日期/状态透传落库,身份一致
	if got := st.reports[oc.Sha256].rep; got.ReportType != "血常规" {
		t.Errorf("report = %+v report_type", got)
	}
	if st.reports[oc.Sha256].row.OCRResultID == 0 {
		t.Errorf("report row ocr_result_id = 0, want 指向追加的 OCR 历史")
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
	if first.Sha256 != second.Sha256 {
		t.Errorf("sha differ: %s vs %s, want 幂等同一图像身份", first.Sha256, second.Sha256)
	}
	if rec.reportCalls != 1 {
		t.Errorf("recognizer calls = %d, want 1(重复 ingest 不重跑识别)", rec.reportCalls)
	}
	if len(st.ocrRows) != 1 {
		t.Errorf("ocr rows = %d, want 1(幂等路径不追加历史)", len(st.ocrRows))
	}
}

func TestIngestForce_RerunsAndAppendsOCR(t *testing.T) {
	st, rec, obj := newFakeStore(), newFakeRecognizer(), newObjects()
	path := writeImage(t, []byte("fake-image-1"))

	ingest(t, st, rec, obj, path, "", false)
	before := len(st.ocrRows)
	second := ingest(t, st, rec, obj, path, "", true)
	after := len(st.ocrRows)

	if second.Kind != pipeline.KindForced {
		t.Fatalf("kind = %v, want KindForced", second.Kind)
	}
	if _, ok := st.reports[second.Sha256]; !ok {
		t.Fatalf("report missing after force, want 原位更新(身份不变)")
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
	if oc.Report.ReportDate == nil || *oc.Report.ReportDate != "2026-10-01" {
		t.Errorf("report date = %v (fake 识别服务按补录 hint 输出)", oc.Report.ReportDate)
	}
	if oc.Report.Status != "success" {
		t.Errorf("status = %s, want 补录后不再是日期缺失 partial", oc.Report.Status)
	}
}

func TestIngestCachedWithDate_RunsBackfillViaReparse(t *testing.T) {
	// 先以无日期首次入库(得到 null 日期报告),再 --date 补录;同一图像身份,不再重跑 OCR。
	rec := newFakeRecognizer()
	rec.report = sampleReport("")
	st, obj := newFakeStore(), newObjects()
	path := writeImage(t, []byte("fake-image-1"))

	first := ingest(t, st, rec, obj, path, "", false)
	if first.Report.ReportDate != nil {
		t.Fatalf("setup: expected null-date report, got %+v", first.Report)
	}
	rec.report = sampleReport("2024-08-19") // 补录后重算会以 hint 生效

	backfilled := ingest(t, st, rec, obj, path, "2026-01-02", false)

	if backfilled.Kind != pipeline.KindBackfilled {
		t.Fatalf("kind = %v, want KindBackfilled", backfilled.Kind)
	}
	if backfilled.Report == nil || backfilled.Report.ReportDate == nil || *backfilled.Report.ReportDate != "2026-01-02" {
		t.Errorf("backfilled report = %+v, want date 2026-01-02", backfilled.Report)
	}
	if rec.reparseCalls != 1 || rec.reportCalls != 1 {
		t.Errorf("calls: report=%d reparse=%d, want 补录只重跑后处理(1/1)", rec.reportCalls, rec.reparseCalls)
	}
}

func TestReparse_PreservesBackfilledDate(t *testing.T) {
	// 存储里的日期是用户补录的元数据:无 --date 的规则重算不应抹掉它。
	rec := newFakeRecognizer()
	st, obj := newFakeStore(), newObjects()
	path := writeImage(t, []byte("fake-image-1"))

	oc, err := pipeline.Ingest(context.Background(), st, rec, obj, path, "2026-10-01", false)
	if err != nil {
		t.Fatalf("ingest with date: %v", err)
	}
	// 模拟:规则迭代后不带 --date 重跑后处理;fake 的 Reparse 把传入 date 作为结果返回
	rec.reparseCalls = 0
	rep, err := pipeline.Reparse(context.Background(), st, rec, oc.Sha256, "")
	if err != nil {
		t.Fatalf("reparse: %v", err)
	}
	if rep.ReportDate == nil || *rep.ReportDate != "2026-10-01" {
		t.Errorf("backfilled date lost: %v", rep.ReportDate)
	}
	if rec.reparseCalls != 1 {
		t.Errorf("reparse calls = %d, want 1", rec.reparseCalls)
	}
	if rec.lastReparseHint == nil || *rec.lastReparseHint != "2026-10-01" {
		t.Errorf("stored date hint not forwarded: %v", rec.lastReparseHint)
	}
}

func TestValidateSha256(t *testing.T) {
	sha := "139fde0e0b0c2114b556bd4462aead8e8d64c44fe24906f87e7bfabc0e4b93b3"
	if err := pipeline.ValidateSha256(sha); err != nil {
		t.Errorf("valid sha rejected: %v", err)
	}
	for _, bad := range []string{"", "139fde0e", "139fde0e0b0c2114b556bd4462aead8e8d64c44fe24906f87e7bfabc0e4b9", "139FDE0E0B0C2114B556BD4462AEAD8E8D64C44FE24906F87E7BFABC0E4B93BE3"} {
		if err := pipeline.ValidateSha256(bad); err == nil {
			t.Errorf("bad sha %q accepted", bad)
		}
	}
}

func TestReparse_UsesStoredOCRAndReplaceItems(t *testing.T) {
	rec := newFakeRecognizer()
	rec.report = sampleReport("") // 首次存成无日期
	st, obj := newFakeStore(), newObjects()
	path := writeImage(t, []byte("fake-image-1"))

	oc := ingest(t, st, rec, obj, path, "", false)
	if _, err := pipeline.Reparse(context.Background(), st, rec, oc.Sha256, ""); err != nil {
		t.Fatalf("reparse: %v", err)
	}
	stored, err := st.OCRResultJSON(context.Background(), st.reports[oc.Sha256].row.OCRResultID)
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

// ---------- 空指标项不变量(viz-and-client 票 03)----------
// 报告级别产出「成功且零指标项」不可能:items 为空一律判 failed,pipeline 产出失败原因,
// 与识别端契约校验及存储镜像校验构成防线;直灌绕过识别端也在此挡住。

func TestIngestEmptyItems_CoercedFailedWithReason(t *testing.T) {
	// 假识别器回归/绕过:success + 零指标项 → 一律判 failed + 失败原因落库。
	for _, bogus := range []string{"success", "partial"} {
		rec := newFakeRecognizer()
		rec.report = contract.Report{ReportType: "血常规", Status: bogus} // items 空
		st, obj := newFakeStore(), newObjects()
		path := writeImage(t, []byte("fake-image-1"))

		oc := ingest(t, st, rec, obj, path, "", false)

		if oc.Report.Status != "failed" {
			t.Errorf("empty items with status %q: ingested status = %q, want failed", bogus, oc.Report.Status)
		}
		want := "no metric items recognized"
		got := st.reports[oc.Sha256].failReason
		if got == nil || *got != want {
			t.Errorf("empty items with status %q: fail_reason = %v, want %q", bogus, got, want)
		}
	}
}

func TestIngestFailedEmpty_KeepsFailedAndSetsReason(t *testing.T) {
	// 真实识别端行为(拍糊/空白图):failed + 零指标项 → 原因照样产出;
	// report_type 取识别端类别失败占位符 unknown(postprocess.UNKNOWN_REPORT_TYPE)。
	rec := newFakeRecognizer()
	rec.report = contract.Report{ReportType: "unknown", Status: "failed"}
	st, obj := newFakeStore(), newObjects()
	path := writeImage(t, []byte("fake-image-1"))

	oc := ingest(t, st, rec, obj, path, "", false)

	if oc.Report.Status != "failed" {
		t.Fatalf("status = %q, want failed untouched", oc.Report.Status)
	}
	want := "no metric items recognized"
	if got := st.reports[oc.Sha256].failReason; got == nil || *got != want {
		t.Errorf("fail_reason = %v, want %q", got, want)
	}
}

func TestIngestWithItems_NoFailReason(t *testing.T) {
	// 正常有产出报告:fail_reason 必须为 nil(仅 failed 报告有值)。
	st, rec, obj := newFakeStore(), newFakeRecognizer(), newObjects()
	path := writeImage(t, []byte("fake-image-1"))

	oc := ingest(t, st, rec, obj, path, "", false)

	if got := st.reports[oc.Sha256].failReason; got != nil {
		t.Errorf("fail_reason = %v, want nil for success report with items", got)
	}
	if st.reports[oc.Sha256].row.Status != "success" {
		t.Errorf("status = %q, want success untouched", st.reports[oc.Sha256].row.Status)
	}
}

func TestReparseRecovers_FailReasonCleared(t *testing.T) {
	// 规则迭代把历史 failed 空报告算好 → 原 failed 报告的失败原因必须清空。
	rec := newFakeRecognizer()
	rec.report = contract.Report{ReportType: "血常规", Status: "failed"} // 历史失败落库
	st, obj := newFakeStore(), newObjects()
	path := writeImage(t, []byte("fake-image-1"))
	oc := ingest(t, st, rec, obj, path, "", false)
	if got := st.reports[oc.Sha256].failReason; got == nil {
		t.Fatalf("setup: fail_reason = nil, want set on failed ingest")
	}

	rec.report = sampleReport("2024-08-19") // 重算后有产出
	rep, err := pipeline.Reparse(context.Background(), st, rec, oc.Sha256, "")
	if err != nil {
		t.Fatalf("reparse: %v", err)
	}
	if rep.Status != "success" {
		t.Errorf("reparse status = %q, want success", rep.Status)
	}
	if got := st.reports[oc.Sha256].failReason; got != nil {
		t.Errorf("fail_reason after reparse = %v, want nil (recovered report)", got)
	}
}
