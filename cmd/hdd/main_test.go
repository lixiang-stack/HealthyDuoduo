package main

// cmd/hdd 单测:参数解析与子命令分发;fake 注入 deps(与 internal/pipeline 的 fake 同思路,
// 本包内重定义小份避免导出)。

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"healthyduoduo/internal/contract"
	"healthyduoduo/internal/pipeline"
)

type fakeRecognizerCLI struct{}

func (fakeRecognizerCLI) Report(ctx context.Context, image []byte, filename, date string) ([]byte, *contract.OCRResult, *contract.Report, error) {
	ocr := &contract.OCRResult{Txts: []string{"x"}, Engine: "onnxruntime"}
	report := &contract.Report{ReportType: "血常规", ReportDate: str("2026-10-01"), Status: "success"}
	return nil, ocr, report, nil
}
func (fakeRecognizerCLI) Reparse(ctx context.Context, ocr *contract.OCRResult, date string) (*contract.Report, error) {
	return &contract.Report{ReportType: "血常规", Status: "success"}, nil
}

type fakeObjectsCLI struct{ objects map[string][]byte }

func (f fakeObjectsCLI) Put(ctx context.Context, key string, data []byte) error {
	if f.objects != nil {
		f.objects[key] = data
	}
	return nil
}

func (f fakeObjectsCLI) Get(ctx context.Context, key string) ([]byte, error) {
	data, ok := f.objects[key]
	if !ok {
		return nil, nil
	}
	return data, nil
}

func str(s string) *string { return &s }

// newTrendStore CLI trend 用例的 Store fixture:注入期望趋势行。
func newTrendStore(rows []pipeline.TrendRow) *fakeServeStore {
	st := newFakeServeStore()
	st.trendRet = rows
	return st
}

func TestRunIngestPrintsSha(t *testing.T) {
	path := filepath.Join(t.TempDir(), "img.jpeg")
	if err := os.WriteFile(path, []byte("fake"), 0o644); err != nil {
		t.Fatal(err)
	}
	var out, errOut bytes.Buffer
	d := &deps{store: newFakeServeStore(), rec: fakeRecognizerCLI{}, obj: fakeObjectsCLI{}}
	if code := run(&out, &errOut, d, []string{"ingest", path, "--date", "2026-10-01"}); code != 0 {
		t.Fatalf("exit = %d, stderr = %q", code, errOut.String())
	}
	if got := out.String(); !strings.Contains(got, "sha256=") {
		t.Fatalf("stdout = %q, want sha256 identity", got)
	}
}

func TestRunListPrintsHeader(t *testing.T) {
	var out, errOut bytes.Buffer
	d := &deps{store: newFakeServeStore(), rec: fakeRecognizerCLI{}, obj: fakeObjectsCLI{}}
	if code := run(&out, &errOut, d, []string{"list"}); code != 0 {
		t.Fatalf("exit = %d, stderr = %q", code, errOut.String())
	}
	for _, col := range []string{"sha256", "ocr", "date", "type", "status", "items"} {
		if !strings.Contains(out.String(), col) {
			t.Fatalf("stdout = %q, want header column %q", out.String(), col)
		}
	}
}

func TestRunUnknownCommand_Usage(t *testing.T) {
	var out, errOut bytes.Buffer
	d := &deps{store: newFakeServeStore(), rec: fakeRecognizerCLI{}, obj: fakeObjectsCLI{}}
	if code := run(&out, &errOut, d, []string{"bogus"}); code != 2 {
		t.Fatalf("exit = %d, want 2", code)
	}
	if !strings.Contains(errOut.String(), "unknown command") || !strings.Contains(errOut.String(), "usage:") {
		t.Fatalf("stderr = %q", errOut.String())
	}
}

func TestRunNoArgs(t *testing.T) {
	var out, errOut bytes.Buffer
	d := &deps{store: newFakeServeStore(), rec: fakeRecognizerCLI{}, obj: fakeObjectsCLI{}}
	if code := run(&out, &errOut, d, nil); code != 2 || !strings.Contains(errOut.String(), "usage:") {
		t.Fatalf("exit = %d stderr = %q", code, errOut.String())
	}
}

func TestRunIngestRejectsBadDate(t *testing.T) {
	var out, errOut bytes.Buffer
	d := &deps{store: newFakeServeStore(), rec: fakeRecognizerCLI{}, obj: fakeObjectsCLI{}}
	if code := run(&out, &errOut, d, []string{"ingest", "a.jpeg", "--date", "2026/10/01"}); code != 2 {
		t.Fatalf("exit = %d, want 2", code)
	}
}

// 未知 flag(--preprocess 已移除)→ usage 错误(exit 2)。
func TestRunIngestRejectsUnknownFlag(t *testing.T) {
	var out, errOut bytes.Buffer
	d := &deps{store: newFakeServeStore(), rec: fakeRecognizerCLI{}, obj: fakeObjectsCLI{}}
	if code := run(&out, &errOut, d, []string{"ingest", "a.jpeg", "--preprocess"}); code != 2 {
		t.Fatalf("exit = %d, want 2", code)
	}
}

// TestTypeDisplayRoundTrip 别名与域值一一对应且回转一致(反转派生的单一数据源)。
func TestTypeDisplayRoundTrip(t *testing.T) {
	for domain, abbr := range typeDisplay {
		if domType(abbr) != domain {
			t.Errorf("domType(%q) = %q, want %q", abbr, domType(abbr), domain)
		}
		if abbrType(domain) != abbr {
			t.Errorf("abbrType(%q) = %q, want %q", domain, abbrType(domain), abbr)
		}
	}
}

func TestRunListRejectsBadDate(t *testing.T) {
	var out, errOut bytes.Buffer
	d := &deps{store: newFakeServeStore(), rec: fakeRecognizerCLI{}, obj: fakeObjectsCLI{}}
	if code := run(&out, &errOut, d, []string{"list", "--date", "garbage"}); code != 2 {
		t.Fatalf("exit = %d, want 2", code)
	}
}

func TestRunTrendPrintsTrendRows(t *testing.T) {
	v, v2, unit := "128", "136", "g/L"
	d := &deps{store: newTrendStore([]pipeline.TrendRow{
		{Sha256: strings.Repeat("a", 64), ReportType: "血常规", ReportDate: str("2026-01-02"), Value: &v, Unit: &unit, Flag: "normal"},
		{Sha256: strings.Repeat("b", 64), ReportType: "血常规", ReportDate: str("2026-03-01"), Value: &v2, Unit: &unit, Flag: "high"},
	}), rec: fakeRecognizerCLI{}, obj: fakeObjectsCLI{}}
	var out, errOut bytes.Buffer
	if code := run(&out, &errOut, d, []string{"trend", "血红蛋白"}); code != 0 {
		t.Fatalf("exit = %d, stderr = %q", code, errOut.String())
	}
	for _, col := range []string{"date", "type", "value", "unit", "delta", "flag", "sha256", "note"} {
		if !strings.Contains(out.String(), col) {
			t.Fatalf("stdout = %q, want header column %q", out.String(), col)
		}
	}
	for _, want := range []string{"CBC", "128", "136", "g/L", "normal", "high", "+8"} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("stdout = %q, want row content %q", out.String(), want)
		}
	}
}

func TestRunTrendRequiresExactlyOneItemName(t *testing.T) {
	var out, errOut bytes.Buffer
	d := &deps{store: newFakeServeStore(), rec: fakeRecognizerCLI{}, obj: fakeObjectsCLI{}}
	if code := run(&out, &errOut, d, []string{"trend"}); code != 2 {
		t.Fatalf("exit = %d, want 2", code)
	}
	if code := run(&out, &errOut, d, []string{"trend", "a", "b"}); code != 2 {
		t.Fatalf("exit = %d, want 2", code)
	}
}

func TestRunTrendExcludesUnverifiedUnitsByDefault(t *testing.T) {
	// 票 12:未验证单位点默认不参与连接;--all 显示并在 note 列标注原因。
	v1, v2, unit := "128", "136", "g/L"
	unverified := "unverified"
	verified := "verified"
	d := &deps{store: newTrendStore([]pipeline.TrendRow{
		{Sha256: strings.Repeat("a", 64), ReportType: "血常规", ReportDate: str("2026-01-02"), Value: &v1, Unit: &unit, UnitStatus: &unverified, Flag: "normal"},
		{Sha256: strings.Repeat("b", 64), ReportType: "血常规", ReportDate: str("2026-03-01"), Value: &v2, Unit: &unit, UnitStatus: &verified, Flag: "high"},
	}), rec: fakeRecognizerCLI{}, obj: fakeObjectsCLI{}}
	var out, errOut bytes.Buffer
	if code := run(&out, &errOut, d, []string{"trend", "血红蛋白"}); code != 0 {
		t.Fatalf("exit = %d, stderr = %q", code, errOut.String())
	}
	if !strings.Contains(errOut.String(), "1 unverified-unit") {
		t.Fatalf("stderr = %q, want unverified hidden note", errOut.String())
	}
	if body := out.String(); strings.Count(body, "g/L") != 1 || strings.Contains(body, "unverified_unit") {
		t.Fatalf("stdout = %q, want only verified row without note", body)
	}

	out.Reset()
	errOut.Reset()
	if code := run(&out, &errOut, d, []string{"trend", "--all", "血红蛋白"}); code != 0 {
		t.Fatalf("exit = %d, stderr = %q", code, errOut.String())
	}
	if body := out.String(); strings.Count(body, "g/L") != 2 || !strings.Contains(body, "unverified_unit") {
		t.Fatalf("--all stdout = %q, want both rows and unverified_unit note", body)
	}
}

func TestRunTrendExcludesLowQualityPointsByDefault(t *testing.T) {
	// 质量门:低置信(决策 #7)或 flag=unknown 的点默认隐藏,--all 全量展示。
	v1, v2, unit := "128", "136", "g/L"
	d := &deps{store: newTrendStore([]pipeline.TrendRow{
		{Sha256: strings.Repeat("a", 64), ReportType: "血常规", ReportDate: str("2026-01-02"), Value: &v1, Unit: &unit, Flag: "unknown"},
		{Sha256: strings.Repeat("b", 64), ReportType: "血常规", ReportDate: str("2026-02-02"), Value: &v1, Unit: &unit, Flag: "normal", LowConfidence: true},
		{Sha256: strings.Repeat("c", 64), ReportType: "血常规", ReportDate: str("2026-03-01"), Value: &v2, Unit: &unit, Flag: "normal"},
	}), rec: fakeRecognizerCLI{}, obj: fakeObjectsCLI{}}
	var out, errOut bytes.Buffer
	if code := run(&out, &errOut, d, []string{"trend", "血红蛋白"}); code != 0 {
		t.Fatalf("exit = %d, stderr = %q", code, errOut.String())
	}
	if !strings.Contains(errOut.String(), "2 low-quality") {
		t.Fatalf("stderr = %q, want hidden-point note", errOut.String())
	}
	if body := out.String(); strings.Count(body, "g/L") != 1 {
		t.Fatalf("stdout = %q, want exactly one kept row", body)
	}

	out.Reset()
	errOut.Reset()
	if code := run(&out, &errOut, d, []string{"trend", "--all", "血红蛋白"}); code != 0 {
		t.Fatalf("exit = %d, stderr = %q", code, errOut.String())
	}
	if strings.Contains(errOut.String(), "low-quality") {
		t.Fatalf("--all stderr = %q, want no hidden-point note", errOut.String())
	}
	if body := out.String(); strings.Count(body, "g/L") != 3 {
		t.Fatalf("--all stdout = %q, want all three rows", body)
	}
}
