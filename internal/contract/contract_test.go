package contract

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// fixture 读 schemas/testdata 下的种子示例(实施计划 4.1/4.2 原样 JSON,
// Python golden 与 Go fixture 同源)。
func fixture(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "schemas", "testdata", name))
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	return data
}

func TestUnmarshalOCRResultFixture(t *testing.T) {
	var got OCRResult
	if err := json.Unmarshal(fixture(t, "ocr_result.example.json"), &got); err != nil {
		t.Fatalf("unmarshal ocr_result: %v", err)
	}
	want := OCRResult{
		Txts:       []string{"血红蛋白", "128", "g/L"},
		Boxes:      []Box{{{100, 200}, {180, 200}, {180, 230}, {100, 230}}, {{300, 200}, {360, 200}, {360, 230}, {300, 230}}, {{400, 200}, {460, 200}, {460, 230}, {400, 230}}},
		Scores:     []float64{0.99, 0.98, 0.97},
		Elapse:     1.23,
		ElapseList: []float64{0.4, 0.05, 0.78},
		Engine:     "onnxruntime",
		ModelInfo:  ModelInfo{Det: "PP-OCRv6_small", Cls: "PP-OCRv4_mobile", Rec: "PP-OCRv6_small"},
	}
	if got.Engine != want.Engine || got.ModelInfo != want.ModelInfo || got.Elapse != want.Elapse {
		t.Errorf("OCRResult scalar fields = %+v, want %+v", got, want)
	}
	if len(got.Txts) != 3 || got.Txts[0] != "血红蛋白" {
		t.Errorf("Txts = %v, want 3 items", got.Txts)
	}
	if len(got.Boxes) != 3 || got.Boxes[0][0] != (Point{100, 200}) {
		t.Errorf("Boxes = %v, want 3 boxes with first vertex [100 200]", got.Boxes)
	}
	if got.Scores[0] != 0.99 {
		t.Errorf("Scores[0] = %v, want 0.99", got.Scores[0])
	}
}

func TestUnmarshalReportFixture(t *testing.T) {
	var got Report
	if err := json.Unmarshal(fixture(t, "report.example.json"), &got); err != nil {
		t.Fatalf("unmarshal report: %v", err)
	}
	if got.ReportType != "血常规" || got.ReportDate == nil || *got.ReportDate != "2026-10-01" || got.Status != "success" {
		t.Errorf("Report header = %+v", got)
	}
	if len(got.Items) != 1 {
		t.Fatalf("Items = %d items, want 1", len(got.Items))
	}
	item := got.Items[0]
	if item.Name != "血红蛋白" || item.Flag != "normal" || item.LowConfidence {
		t.Errorf("Item = %+v", item)
	}
	if item.Value == nil || *item.Value != 128 {
		t.Errorf("Item.Value = %v, want 128", item.Value)
	}
	if item.Unit == nil || *item.Unit != "g/L" || item.RefRange == nil || *item.RefRange != "115-150" {
		t.Errorf("Item unit/ref = %+v", item)
	}
	if item.RawText != "血红蛋白 128 g/L 115-150" {
		t.Errorf("Item.RawText = %q", item.RawText)
	}
}

func TestUnmarshalReportNullDateFixture(t *testing.T) {
	var got Report
	if err := json.Unmarshal(fixture(t, "report.null-date.example.json"), &got); err != nil {
		t.Fatalf("unmarshal report (null date): %v", err)
	}
	if got.ReportDate != nil {
		t.Errorf("ReportDate = %v, want nil (null)", got.ReportDate)
	}
	if got.Status != "partial" {
		t.Errorf("Status = %q, want partial", got.Status)
	}
	if !got.Items[0].LowConfidence {
		t.Errorf("Items[0].LowConfidence = false, want true")
	}
}

// TestTableStructureRoundTrip 覆盖 ADR-0004 新增的可选字段:有值可往返、缺省为 nil。
func TestTableStructureRoundTrip(t *testing.T) {
	src := OCRResult{
		Txts:           []string{"血红蛋白"},
		Boxes:          []Box{{{0, 0}, {1, 0}, {1, 1}, {0, 1}}},
		Scores:         []float64{0.9},
		Elapse:         0.1,
		ElapseList:     []float64{0.1},
		Engine:         "onnxruntime",
		ModelInfo:      ModelInfo{Det: "d", Cls: "c", Rec: "r"},
		TableStructure: &TableStructure{HTML: "<table></table>", Model: "lineless", Elapse: 0.2},
	}
	data, err := json.Marshal(src)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var got OCRResult
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.TableStructure == nil || got.TableStructure.HTML != "<table></table>" || got.TableStructure.Model != "lineless" {
		t.Errorf("TableStructure = %+v", got.TableStructure)
	}

	var none OCRResult
	minimal := `{"txts":[],"boxes":[],"scores":[],"elapse":0,"elapse_list":[],"engine":"onnxruntime","model_info":{"det":"d","cls":"c","rec":"r"}}`
	if err := json.Unmarshal([]byte(minimal), &none); err != nil {
		t.Fatalf("unmarshal minimal: %v", err)
	}
	if none.TableStructure != nil {
		t.Errorf("TableStructure = %+v, want nil when omitted", none.TableStructure)
	}
}
