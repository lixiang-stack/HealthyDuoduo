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

// fakeStoreCLI 默认空库;trendRet 注入 trend 查询的期望行集。
type fakeStoreCLI struct {
	trendRet []pipeline.TrendRow
}

func (fakeStoreCLI) ImageBySha256(ctx context.Context, sha string) (*pipeline.Image, error) {
	return nil, nil
}
func (fakeStoreCLI) CreateImage(ctx context.Context, sha, filename string) error { return nil }
func (fakeStoreCLI) AppendOCRResult(ctx context.Context, imageSha string, raw []byte) (int64, error) {
	return 2, nil
}
func (fakeStoreCLI) ReportByImage(ctx context.Context, imageSha string) (*pipeline.ReportRow, bool, error) {
	return nil, false, nil
}
func (fakeStoreCLI) ListReports(ctx context.Context, typ, date string) ([]pipeline.ReportRow, error) {
	return []pipeline.ReportRow{}, nil
}
func (fakeStoreCLI) ItemsByImage(ctx context.Context, reportSha string) ([]contract.ReportItem, error) {
	return nil, nil
}
func (fakeStoreCLI) OCRResultJSON(ctx context.Context, id int64) ([]byte, error) {
	return []byte("{}"), nil
}
func (fakeStoreCLI) WriteReport(ctx context.Context, reportSha string, ocrID int64, report contract.Report) error {
	return nil
}
func (f fakeStoreCLI) TrendByName(ctx context.Context, item, reportType string) ([]pipeline.TrendRow, error) {
	return f.trendRet, nil
}

type fakeRecognizerCLI struct{}

func (fakeRecognizerCLI) Report(ctx context.Context, image []byte, filename, date, preprocess string) (*contract.OCRResult, *contract.Report, error) {
	ocr := &contract.OCRResult{Txts: []string{"x"}, Engine: "onnxruntime"}
	report := &contract.Report{ReportType: "血常规", ReportDate: str("2026-10-01"), Status: "success"}
	return ocr, report, nil
}
func (fakeRecognizerCLI) Reparse(ctx context.Context, ocr *contract.OCRResult, date string) (*contract.Report, error) {
	return &contract.Report{ReportType: "血常规", Status: "success"}, nil
}

type fakeObjectsCLI struct{}

func (fakeObjectsCLI) Put(ctx context.Context, key string, data []byte) error { return nil }

func str(s string) *string { return &s }

func TestRunIngestPrintsSha(t *testing.T) {
	path := filepath.Join(t.TempDir(), "img.jpeg")
	if err := os.WriteFile(path, []byte("fake"), 0o644); err != nil {
		t.Fatal(err)
	}
	var out, errOut bytes.Buffer
	d := &deps{store: fakeStoreCLI{}, rec: fakeRecognizerCLI{}, obj: fakeObjectsCLI{}}
	if code := run(&out, &errOut, d, []string{"ingest", path, "--date", "2026-10-01"}); code != 0 {
		t.Fatalf("exit = %d, stderr = %q", code, errOut.String())
	}
	if got := out.String(); !strings.Contains(got, "sha256=") {
		t.Fatalf("stdout = %q, want sha256 identity", got)
	}
}

func TestRunListPrintsHeader(t *testing.T) {
	var out, errOut bytes.Buffer
	d := &deps{store: fakeStoreCLI{}, rec: fakeRecognizerCLI{}, obj: fakeObjectsCLI{}}
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
	d := &deps{store: fakeStoreCLI{}, rec: fakeRecognizerCLI{}, obj: fakeObjectsCLI{}}
	if code := run(&out, &errOut, d, []string{"bogus"}); code != 2 {
		t.Fatalf("exit = %d, want 2", code)
	}
	if !strings.Contains(errOut.String(), "unknown command") || !strings.Contains(errOut.String(), "usage:") {
		t.Fatalf("stderr = %q", errOut.String())
	}
}

func TestRunNoArgs(t *testing.T) {
	var out, errOut bytes.Buffer
	d := &deps{store: fakeStoreCLI{}, rec: fakeRecognizerCLI{}, obj: fakeObjectsCLI{}}
	if code := run(&out, &errOut, d, nil); code != 2 || !strings.Contains(errOut.String(), "usage:") {
		t.Fatalf("exit = %d stderr = %q", code, errOut.String())
	}
}

func TestRunIngestRejectsBadDate(t *testing.T) {
	var out, errOut bytes.Buffer
	d := &deps{store: fakeStoreCLI{}, rec: fakeRecognizerCLI{}, obj: fakeObjectsCLI{}}
	if code := run(&out, &errOut, d, []string{"ingest", "a.jpeg", "--date", "2026/10/01"}); code != 2 {
		t.Fatalf("exit = %d, want 2", code)
	}
}

func TestRunIngestRejectsMissingPreprocessValue(t *testing.T) {
	var out, errOut bytes.Buffer
	d := &deps{store: fakeStoreCLI{}, rec: fakeRecognizerCLI{}, obj: fakeObjectsCLI{}}
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
	d := &deps{store: fakeStoreCLI{}, rec: fakeRecognizerCLI{}, obj: fakeObjectsCLI{}}
	if code := run(&out, &errOut, d, []string{"list", "--date", "garbage"}); code != 2 {
		t.Fatalf("exit = %d, want 2", code)
	}
}

func TestRunTrendPrintsTrendRows(t *testing.T) {
	v, v2, unit := 128.0, 136.0, "g/L"
	d := &deps{store: fakeStoreCLI{trendRet: []pipeline.TrendRow{
		{Sha256: strings.Repeat("a", 64), ReportType: "血常规", ReportDate: str("2026-01-02"), Value: &v, Unit: &unit, Flag: "normal"},
		{Sha256: strings.Repeat("b", 64), ReportType: "血常规", ReportDate: str("2026-03-01"), Value: &v2, Unit: &unit, Flag: "high"},
	}}, rec: fakeRecognizerCLI{}, obj: fakeObjectsCLI{}}
	var out, errOut bytes.Buffer
	if code := run(&out, &errOut, d, []string{"trend", "血红蛋白"}); code != 0 {
		t.Fatalf("exit = %d, stderr = %q", code, errOut.String())
	}
	for _, col := range []string{"date", "type", "value", "unit", "delta", "flag", "sha256"} {
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
	d := &deps{store: fakeStoreCLI{}, rec: fakeRecognizerCLI{}, obj: fakeObjectsCLI{}}
	if code := run(&out, &errOut, d, []string{"trend"}); code != 2 {
		t.Fatalf("exit = %d, want 2", code)
	}
	if code := run(&out, &errOut, d, []string{"trend", "a", "b"}); code != 2 {
		t.Fatalf("exit = %d, want 2", code)
	}
}
