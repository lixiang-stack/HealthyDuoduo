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

type fakeStoreCLI struct{}

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

type fakeRecognizerCLI struct{}

func (fakeRecognizerCLI) Report(ctx context.Context, image []byte, filename, date string) (*contract.OCRResult, *contract.Report, error) {
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

func TestRunListRejectsBadDate(t *testing.T) {
	var out, errOut bytes.Buffer
	d := &deps{store: fakeStoreCLI{}, rec: fakeRecognizerCLI{}, obj: fakeObjectsCLI{}}
	if code := run(&out, &errOut, d, []string{"list", "--date", "garbage"}); code != 2 {
		t.Fatalf("exit = %d, want 2", code)
	}
}
