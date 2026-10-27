package contract

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
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

// decodeReport 严格解码报告 fixture:Go struct 缺少 schema 字段时立即失败,
// 而不是像 json.Unmarshal 那样静默丢弃(ADR-0002 双语契约防漂移)。
func decodeReport(t *testing.T, name string) Report {
	t.Helper()
	dec := json.NewDecoder(bytes.NewReader(fixture(t, name)))
	dec.DisallowUnknownFields()
	var got Report
	if err := dec.Decode(&got); err != nil {
		t.Fatalf("decode report %s: %v", name, err)
	}
	return got
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
	got := decodeReport(t, "report.example.json")
	if got.ReportType != "临检" || got.ReportDate == nil || *got.ReportDate != "2026-10-01" || got.Status != "success" {
		t.Errorf("Report header = %+v", got)
	}
	if len(got.Panels) != 1 || got.Panels[0] != "cbc" {
		t.Errorf("Report.Panels = %v, want [cbc]", got.Panels)
	}
	if len(got.Items) != 1 {
		t.Fatalf("Items = %d items, want 1", len(got.Items))
	}
	item := got.Items[0]
	if item.Name != "血红蛋白" || item.Panel != "cbc" || item.Flag != "normal" || item.LowConfidence {
		t.Errorf("Item = %+v", item)
	}
	if item.Value == nil || *item.Value != "128" {
		t.Errorf("Item.Value = %v, want \"128\"", item.Value)
	}
	if item.Unit == nil || *item.Unit != "g/L" || item.RefRange == nil || *item.RefRange != "115-150" {
		t.Errorf("Item unit/ref = %+v", item)
	}
	if item.Method == nil || *item.Method != "化学发光法" {
		t.Errorf("Item.Method = %v, want \"化学发光法\"", item.Method)
	}
	if item.RawText != "血红蛋白 128 g/L 115-150" {
		t.Errorf("Item.RawText = %q", item.RawText)
	}
	if len(item.OCRLineNumbers) != 3 || item.OCRLineNumbers[0] != 0 || item.OCRLineNumbers[2] != 2 {
		t.Errorf("Item.OCRLineNumbers = %v, want [0 1 2]", item.OCRLineNumbers)
	}
}

func TestUnmarshalReportSectionsFixture(t *testing.T) {
	got := decodeReport(t, "report.sections.example.json")
	if got.ReportType != "影像" || got.ReportDate == nil || *got.ReportDate != "2024-09-15" {
		t.Errorf("Report header = %+v", got)
	}
	if len(got.Panels) != 1 || got.Panels[0] != "imaging" {
		t.Errorf("Report.Panels = %v, want [imaging]", got.Panels)
	}
	if len(got.Sections) != 3 {
		t.Fatalf("Sections = %d, want 3", len(got.Sections))
	}
	if got.Sections[0].Kind != "exam_name" || got.Sections[0].Text != "产科胎儿彩超检查（中孕）" ||
		len(got.Sections[0].OCRLineNumbers) != 1 || got.Sections[0].OCRLineNumbers[0] != 1 {
		t.Errorf("Sections[0] = %+v", got.Sections[0])
	}
	if got.Sections[1].Kind != "findings" || len(got.Sections[1].OCRLineNumbers) != 2 {
		t.Errorf("Sections[1] = %+v", got.Sections[1])
	}
}

func TestUnmarshalReportNullDateFixture(t *testing.T) {
	got := decodeReport(t, "report.null-date.example.json")
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

// jsonSchemaRequired 只取 schema 的必填字段树,用于检查 Go struct 是否镜像。
type jsonSchemaRequired struct {
	Required []string                      `json:"required"`
	Defs     map[string]jsonSchemaRequired `json:"$defs"`
}

func TestReportSchemaRequiredFieldsMirrorGoStructs(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "schemas", "report.schema.json"))
	if err != nil {
		t.Fatalf("read report schema: %v", err)
	}
	var schema jsonSchemaRequired
	if err := json.Unmarshal(data, &schema); err != nil {
		t.Fatalf("unmarshal report schema: %v", err)
	}
	cases := []struct {
		name     string
		typ      reflect.Type
		required []string
	}{
		{"Report", reflect.TypeOf(Report{}), schema.Required},
		{"ReportItem", reflect.TypeOf(ReportItem{}), schema.Defs["reportItem"].Required},
		{"ReportSection", reflect.TypeOf(ReportSection{}), schema.Defs["reportSection"].Required},
	}
	for _, c := range cases {
		if len(c.required) == 0 {
			t.Fatalf("%s: schema required list is empty (schema shape changed?)", c.name)
		}
		assertRequiredJSONFields(t, c.name, c.typ, c.required)
	}
}

func assertRequiredJSONFields(t *testing.T, typeName string, typ reflect.Type, required []string) {
	t.Helper()
	tags := make(map[string]bool, typ.NumField())
	for i := 0; i < typ.NumField(); i++ {
		tag := strings.Split(typ.Field(i).Tag.Get("json"), ",")[0]
		if tag != "" && tag != "-" {
			tags[tag] = true
		}
	}
	for _, name := range required {
		if !tags[name] {
			t.Errorf("%s is missing required JSON field %q", typeName, name)
		}
	}
}
