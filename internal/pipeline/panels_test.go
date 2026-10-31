package pipeline

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func strPtr(s string) *string { return &s }

// TestPanelOrderFixed spec §5.3 的固定优先级:同日期项目列表的并列裁决顺序。
func TestPanelOrderFixed(t *testing.T) {
	want := []string{
		"liver", "kidney", "lipid", "glucose", "uric_acid", "electrolyte",
		"vitanemia", "thyroid", "sex_hormone", "infection", "autoimmune",
		"coag", "cbc", "urinalysis", "imaging", "other",
	}
	if len(panelOrder) != len(want) {
		t.Fatalf("panelOrder = %v, want %d codes", panelOrder, len(want))
	}
	for i, code := range want {
		if panelOrder[i] != code {
			t.Errorf("panelOrder[%d] = %q, want %q", i, panelOrder[i], code)
		}
	}
}

// TestPanelOrderMirrorsRecognitionSchema 集合与 §5.3 顺序以识别端 schema 为单一来源:
// 改 schema 而漏改 Go 会在这里失败,不再依赖手写清单互相对齐。
func TestPanelOrderMirrorsRecognitionSchema(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "schemas", "report.schema.json"))
	if err != nil {
		t.Fatalf("read recognition schema: %v", err)
	}
	var schema struct {
		Defs struct {
			Panel struct {
				Enum  []string `json:"enum"`
				Order []string `json:"x-panel-order"`
			} `json:"panel"`
		} `json:"$defs"`
	}
	if err := json.Unmarshal(raw, &schema); err != nil {
		t.Fatalf("parse recognition schema: %v", err)
	}
	if !slices.Equal(panelOrder, schema.Defs.Panel.Order) {
		t.Errorf("panelOrder = %v, want schema x-panel-order %v", panelOrder, schema.Defs.Panel.Order)
	}
	enum := make(map[string]bool, len(schema.Defs.Panel.Enum))
	for _, code := range schema.Defs.Panel.Enum {
		enum[code] = true
		if !IsValidPanel(code) {
			t.Errorf("schema panel %q missing from Go IsValidPanel", code)
		}
	}
	if len(enum) != len(panelOrder) {
		t.Errorf("schema enum = %d codes, Go panelOrder = %d", len(enum), len(panelOrder))
	}
	for _, code := range panelOrder {
		if !enum[code] {
			t.Errorf("Go panelOrder code %q missing from schema enum", code)
		}
	}
}

func TestIsValidPanel(t *testing.T) {
	for _, code := range panelOrder {
		if !IsValidPanel(code) {
			t.Errorf("IsValidPanel(%q) = false, want true", code)
		}
	}
	for _, code := range []string{"", "liver ", "unknown", "Imaging"} {
		if IsValidPanel(code) {
			t.Errorf("IsValidPanel(%q) = true, want false", code)
		}
	}
}

func TestSortPanelSummaries(t *testing.T) {
	panels := []PanelSummary{
		{Panel: "electrolyte", LatestReportDate: strPtr("2020-01-01")},
		{Panel: "other", LatestReportDate: nil},
		{Panel: "uric_acid", LatestReportDate: strPtr("2021-08-02")},
		{Panel: "lipid", LatestReportDate: strPtr("2020-01-01")},
		{Panel: "liver", LatestReportDate: strPtr("2021-08-02")},
		{Panel: "kidney", LatestReportDate: strPtr("2021-08-02")},
		{Panel: "thyroid", LatestReportDate: strPtr("2021-08-02")},
	}
	SortPanelSummaries(panels)
	want := []string{"liver", "kidney", "uric_acid", "thyroid", "lipid", "electrolyte", "other"}
	for i, code := range want {
		if panels[i].Panel != code {
			t.Fatalf("sorted panels = %v, want %v", panelCodes(panels), want)
		}
	}
}

func TestPanelNarrative(t *testing.T) {
	if !IsNarrativePanel("imaging") {
		t.Error("imaging must be narrative")
	}
	if IsNarrativePanel("liver") {
		t.Error("liver is not narrative")
	}
}

func TestBuildPanelItems(t *testing.T) {
	points := []PanelItemPoint{
		{ReportID: "r1", ReportDate: strPtr("2020-01-01"), Name: "丙氨酸转氨酶", Unit: strPtr("U/L"), Value: strPtr("27.4"), Flag: FlagNormal},
		{ReportID: "r1", ReportDate: strPtr("2020-01-01"), Name: "白蛋白", Unit: strPtr("g/L"), Value: strPtr("42"), Flag: FlagNormal},
		{ReportID: "r2", ReportDate: strPtr("2021-01-01"), Name: "丙氨酸转氨酶", Unit: strPtr("U/L"), Value: strPtr("835"), Flag: FlagHigh, ReviewStatus: ReviewStatusCorrected},
		{ReportID: "r2", ReportDate: strPtr("2021-01-01"), Name: "白蛋白", Unit: strPtr("g/L"), Flag: FlagNotTested},
		{ReportID: "r2", ReportDate: strPtr("2021-01-01"), Name: "总胆红素", Unit: strPtr("μmol/L"), Value: strPtr("41"), Flag: FlagHigh},
		{ReportID: "r2", ReportDate: strPtr("2021-01-01"), Name: "前白蛋白", Unit: strPtr("mg/L"), Flag: FlagNotTested},
	}
	items := BuildPanelItems(points, "r2")
	// 异常项在前(ALT / TBIL),其余按最新报告内顺序;只有 not_tested 的项也保留。
	wantOrder := []string{"丙氨酸转氨酶", "总胆红素", "白蛋白", "前白蛋白"}
	if got := panelItemNames(items); !slices.Equal(got, wantOrder) {
		t.Fatalf("items = %v, want %v", got, wantOrder)
	}
	alt := items[0]
	if alt.Latest == nil || alt.Latest.Value == nil || *alt.Latest.Value != "835" ||
		alt.Latest.ReviewStatus != ReviewStatusCorrected || alt.ReportCount != 2 || len(alt.Points) != 2 {
		t.Errorf("ALT = %+v", alt)
	}
	albumin := items[2]
	if albumin.Latest == nil || *albumin.Latest.Value != "42" || albumin.ReportCount != 1 ||
		albumin.NotTestedCount != 1 || !albumin.LatestReportNotTested {
		t.Errorf("albumin = %+v, want latest 42 but latest report not_tested", albumin)
	}
	prealbumin := items[3]
	if prealbumin.Latest != nil || prealbumin.ReportCount != 0 || prealbumin.NotTestedCount != 1 ||
		!prealbumin.LatestReportNotTested {
		t.Errorf("prealbumin = %+v, want no latest / not_tested", prealbumin)
	}
}

func TestBuildPanelItemsSameDateTie(t *testing.T) {
	// 同日报告:后一份(sha 降序更大)是最近;latest / abnormal 与项目列表 SQL 同口径。
	points := []PanelItemPoint{
		{ReportID: "r-a", ReportDate: strPtr("2020-01-01"), Name: "总蛋白", Unit: strPtr("g/L"), Value: strPtr("70"), Flag: FlagNormal},
		{ReportID: "r-f", ReportDate: strPtr("2020-01-01"), Name: "总蛋白", Unit: strPtr("g/L"), Value: strPtr("90"), Flag: FlagHigh},
	}
	items := BuildPanelItems(points, "r-f")
	if len(items) != 1 || items[0].Latest == nil || items[0].Latest.ReportID != "r-f" ||
		items[0].Latest.Flag != FlagHigh || items[0].ReportCount != 2 {
		t.Fatalf("same-date items = %+v, want latest r-f high", items)
	}
}

func TestBuildPanelItemsSplitsUnits(t *testing.T) {
	points := []PanelItemPoint{
		{ReportID: "r1", ReportDate: strPtr("2020-01-01"), Name: "血红蛋白", Unit: strPtr("g/L"), Value: strPtr("128"), Flag: FlagNormal},
		{ReportID: "r1", ReportDate: strPtr("2020-01-01"), Name: "血红蛋白", Unit: strPtr("g/dL"), Value: strPtr("12.8"), Flag: FlagNormal},
	}
	items := BuildPanelItems(points, "r1")
	if len(items) != 2 {
		t.Fatalf("items = %+v, want one row per trend key + unit", items)
	}
}

func TestTrendKeyPriority(t *testing.T) {
	tests := []struct {
		code, loinc, name, want string
	}{
		{"1500104A", "1742-6", "丙氨酸氨基转移酶测定", "1500104A"},
		{"", "1742-6", "丙氨酸氨基转移酶测定", "1742-6"},
		{" ", " ", "丙氨酸氨基转移酶测定", "丙氨酸氨基转移酶测定"},
		{"", "", "", ""},
	}
	for _, tt := range tests {
		if got := TrendKey(tt.code, tt.loinc, tt.name); got != tt.want {
			t.Errorf("TrendKey(%q, %q, %q) = %q, want %q", tt.code, tt.loinc, tt.name, got, tt.want)
		}
	}
}

func TestBuildPanelItemsJoinsByTrendKey(t *testing.T) {
	// 同一 code 跨名称 / 跨面板聚合为一个趋势序列;无 code 时回退 LOINC;
	// 不同 code 即使同名也不合并。
	code, loinc, otherCode := "1500104A", "1742-6", "1500105A"
	points := []PanelItemPoint{
		{ReportID: "r1", ReportDate: strPtr("2020-01-01"), Name: "丙氨酸转氨酶", Code: &code, Unit: strPtr("U/L"), Value: strPtr("27.4"), Flag: FlagNormal},
		{ReportID: "r2", ReportDate: strPtr("2021-01-01"), Name: "丙氨酸氨基转移酶测定", Code: &code, Unit: strPtr("U/L"), Value: strPtr("835"), Flag: FlagHigh},
		{ReportID: "r1", ReportDate: strPtr("2020-01-01"), Name: "ALT-旧名", Loinc: &loinc, Unit: strPtr("U/L"), Value: strPtr("30"), Flag: FlagNormal},
		{ReportID: "r2", ReportDate: strPtr("2021-01-01"), Name: "ALT-新名", Loinc: &loinc, Unit: strPtr("U/L"), Value: strPtr("40"), Flag: FlagNormal},
		{ReportID: "r3", ReportDate: strPtr("2022-01-01"), Name: "另一个同名项", Code: &otherCode, Unit: strPtr("U/L"), Value: strPtr("1"), Flag: FlagNormal},
	}
	items := BuildPanelItems(points, "r2")
	if len(items) != 3 {
		t.Fatalf("items = %+v, want code-grouped ALT + LOINC ALT + other code", items)
	}
	alt := items[0]
	if alt.Key != code || alt.Name != "丙氨酸氨基转移酶测定" || len(alt.Points) != 2 || alt.ReportCount != 2 {
		t.Fatalf("ALT = %+v, want two points joined by code and latest name", alt)
	}
	if alt.Latest == nil || alt.Latest.Value == nil || *alt.Latest.Value != "835" {
		t.Errorf("ALT latest = %+v, want 835", alt.Latest)
	}
	loincItem := items[1]
	if loincItem.Key != loinc || len(loincItem.Points) != 2 || loincItem.Name != "ALT-新名" {
		t.Errorf("LOINC item = %+v, want two points joined by LOINC", loincItem)
	}
	if items[2].Key != otherCode {
		t.Errorf("third key = %q, want %q", items[2].Key, otherCode)
	}
}

func panelItemNames(items []PanelItem) []string {
	out := make([]string, 0, len(items))
	for _, item := range items {
		out = append(out, item.Name)
	}
	return out
}

func panelCodes(panels []PanelSummary) []string {
	out := make([]string, 0, len(panels))
	for _, panel := range panels {
		out = append(out, panel.Panel)
	}
	return out
}
