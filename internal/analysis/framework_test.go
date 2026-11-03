package analysis

import (
	"slices"
	"strconv"
	"strings"
	"testing"

	"healthyduoduo/internal/contract"
	"healthyduoduo/internal/pipeline"
)

var testAnalyteByName = map[string]string{
	"丙氨酸氨基转移酶测定":    contract.AnalyteALT,
	"天门冬氨酸氨基转移酶测定":  contract.AnalyteAST,
	"碱性磷酸酶测定":       contract.AnalyteALP,
	"血红蛋白测定":        contract.AnalyteHemoglobin,
	"平均红细胞体积测定":     contract.AnalyteMCV,
	"平均红细胞血红蛋白含量测定": contract.AnalyteMCH,
	"平均红细胞血红蛋白浓度测定": contract.AnalyteMCHC,
	"红细胞分布宽度-CV":    contract.AnalyteRDWCV,
}

func frameworkPoint(itemID int64, name, value, unit, refRange, flag string) EvidencePoint {
	return EvidencePoint{
		EvidenceID:  "ev-r1-" + strconv.FormatInt(itemID, 10),
		ReportID:    "r1",
		ReportDate:  strPtr("2026-10-01"),
		ReportType:  "生化",
		TrendKey:    name,
		Analyte:     testAnalyteByName[name],
		Name:        name,
		Unit:        strPtr(unit),
		UnitDisplay: strPtr(unit),
		Value:       strPtr(value),
		RefRange:    strPtr(refRange),
		Flag:        flag,
	}
}

func frameworkPointFor(reportID, date string, itemID int64, name, value, unit, refRange, flag string) EvidencePoint {
	point := frameworkPoint(itemID, name, value, unit, refRange, flag)
	point.ReportID = reportID
	point.ReportDate = strPtr(date)
	point.EvidenceID = "ev-" + reportID + "-" + strconv.FormatInt(itemID, 10)
	return point
}

func frameworkFacts(points ...EvidencePoint) Facts {
	facts := Facts{
		Panel:   pipeline.PanelLiver,
		Entry:   EntryPanel,
		Reports: []ReportRef{{ReportID: "r1", ReportDate: strPtr("2026-10-01"), ReportType: "生化"}},
	}
	if len(points) > 0 {
		facts.History = []History{{Points: points, Latest: points[len(points)-1]}}
	}
	return facts
}

func frameworkLabels(t *testing.T, req Request, facts Facts, growth *InputGrowth) []FrameworkLabel {
	t.Helper()
	return BuildFramework(req, facts, growth).Labels
}

func labelByCode(labels []FrameworkLabel, code string) (FrameworkLabel, bool) {
	for _, label := range labels {
		if label.Code == code {
			return label, true
		}
	}
	return FrameworkLabel{}, false
}

func TestLiverRValueBoundaries(t *testing.T) {
	cases := []struct {
		name string
		alt  string
		want string
	}{
		{"1.9", "91.2", "胆汁淤积型"},
		{"2.0", "96", "胆汁淤积型"},
		{"4.9", "235.2", "混合型"},
		{"5.0", "240", "肝细胞型"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			points := []EvidencePoint{
				frameworkPoint(1, "丙氨酸氨基转移酶测定", c.alt, "U/L", "7-40", contract.FlagHigh),
				frameworkPoint(2, "碱性磷酸酶测定", "120", "U/L", "40-100", contract.FlagHigh),
			}
			labels := frameworkLabels(t, testRequest(EntryPanel, pipeline.PanelLiver, 12), frameworkFacts(points...), nil)
			label, ok := labelByCode(labels, LabelLiverRValue)
			if !ok || label.Label != c.want {
				t.Fatalf("R label = %+v, want %s", label, c.want)
			}
			if len(label.Sources) == 0 || label.Value == nil {
				t.Fatalf("R label missing source/value: %+v", label)
			}
		})
	}
}

func TestLiverRValueClassifiesRawRatio(t *testing.T) {
	// 原始 R = (200.1/40)/(100.1/100) = 4.9975;四舍五入后显示 5.00,但分类必须按原始值判混合型。
	points := []EvidencePoint{
		frameworkPoint(1, "丙氨酸氨基转移酶测定", "200.1", "U/L", "7-40", contract.FlagHigh),
		frameworkPoint(2, "碱性磷酸酶测定", "100.1", "U/L", "40-100", contract.FlagHigh),
	}
	labels := frameworkLabels(t, testRequest(EntryPanel, pipeline.PanelLiver, 12), frameworkFacts(points...), nil)
	label, ok := labelByCode(labels, LabelLiverRValue)
	if !ok || label.Label != "混合型" {
		t.Fatalf("R label = %+v, want 混合型 (raw R=4.9975)", label)
	}
	if label.Value == nil || !strings.Contains(*label.Value, "R = 5.00") {
		t.Fatalf("R value = %v, want rounded display 5.00", label.Value)
	}
}

func TestTransaminaseGradeClassifiesRawRatio(t *testing.T) {
	cases := []struct{ name, value, want string }{
		{"1.999", "79.96", "临界"},
		{"1.001", "40.04", "临界"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			points := []EvidencePoint{
				frameworkPoint(1, "丙氨酸氨基转移酶测定", c.value, "U/L", "7-40", contract.FlagHigh),
			}
			labels := frameworkLabels(t, testRequest(EntryPanel, pipeline.PanelLiver, 12), frameworkFacts(points...), nil)
			label, ok := labelByCode(labels, LabelLiverTransaminaseGrade)
			if !ok || label.Label != "转氨酶升高："+c.want {
				t.Fatalf("grade = %+v, want %s", label, c.want)
			}
		})
	}
}

func TestFrameworkFallsBackToOlderReportWithRequiredIndicators(t *testing.T) {
	older := []EvidencePoint{
		frameworkPointFor("r1", "2026-09-01", 1, "丙氨酸氨基转移酶测定", "200.1", "U/L", "7-40", contract.FlagHigh),
		frameworkPointFor("r1", "2026-09-01", 2, "碱性磷酸酶测定", "100.1", "U/L", "40-100", contract.FlagHigh),
		frameworkPointFor("r1", "2026-09-01", 3, "天门冬氨酸氨基转移酶测定", "80", "U/L", "13-35", contract.FlagHigh),
	}
	latest := []EvidencePoint{
		frameworkPointFor("r2", "2026-10-01", 4, "血红蛋白测定", "120", "g/L", "115-150", contract.FlagNormal),
	}
	facts := Facts{
		Panel: pipeline.PanelLiver,
		Entry: EntryPanel,
		Reports: []ReportRef{
			{ReportID: "r2", ReportDate: strPtr("2026-10-01"), ReportType: "生化"},
			{ReportID: "r1", ReportDate: strPtr("2026-09-01"), ReportType: "生化"},
		},
		History: []History{
			{Points: latest, Latest: latest[0]},
			{Points: older, Latest: older[len(older)-1]},
		},
	}
	labels := frameworkLabels(t, testRequest(EntryPanel, pipeline.PanelLiver, 12), facts, nil)
	for _, code := range []string{LabelLiverRValue, LabelLiverTransaminaseGrade, LabelLiverASTALTRatio} {
		label, ok := labelByCode(labels, code)
		if !ok || label.Value == nil || !strings.Contains(*label.Value, "2026-09-01") {
			t.Fatalf("%s = %+v, want fallback to 2026-09-01", code, label)
		}
	}
}

func TestLiverRValueNeedsBothULNs(t *testing.T) {
	points := []EvidencePoint{
		frameworkPoint(1, "丙氨酸氨基转移酶测定", "200", "U/L", "7-40", contract.FlagHigh),
		frameworkPoint(2, "碱性磷酸酶测定", "100", "U/L", "", contract.FlagNormal),
	}
	labels := frameworkLabels(t, testRequest(EntryPanel, pipeline.PanelLiver, 12), frameworkFacts(points...), nil)
	if _, ok := labelByCode(labels, LabelLiverRValue); ok {
		t.Fatal("R label emitted without ALP ULN")
	}
}

func TestLiverRValueHandlesIsolatedElevation(t *testing.T) {
	// 只有转氨酶升高、R ≥ 5:肝细胞型;固定解释与实际升高方向一致。
	altOnly := []EvidencePoint{
		frameworkPoint(1, "丙氨酸氨基转移酶测定", "200", "U/L", "7-40", contract.FlagHigh),
		frameworkPoint(2, "碱性磷酸酶测定", "100", "U/L", "40-100", contract.FlagNormal),
	}
	labels := frameworkLabels(t, testRequest(EntryPanel, pipeline.PanelLiver, 12), frameworkFacts(altOnly...), nil)
	if label, ok := labelByCode(labels, LabelLiverRValue); !ok || label.Label != "肝细胞型" {
		t.Fatalf("isolated ALT R label = %+v, want 肝细胞型", label)
	}
	// 只有碱性磷酸酶升高、R ≤ 2:胆汁淤积型。
	alpOnly := []EvidencePoint{
		frameworkPoint(1, "丙氨酸氨基转移酶测定", "20", "U/L", "7-40", contract.FlagNormal),
		frameworkPoint(2, "碱性磷酸酶测定", "300", "U/L", "40-100", contract.FlagHigh),
	}
	labels = frameworkLabels(t, testRequest(EntryPanel, pipeline.PanelLiver, 12), frameworkFacts(alpOnly...), nil)
	if label, ok := labelByCode(labels, LabelLiverRValue); !ok || label.Label != "胆汁淤积型" {
		t.Fatalf("isolated ALP R label = %+v, want 胆汁淤积型", label)
	}
}

func TestLiverLabelsSkipNormalPanel(t *testing.T) {
	points := []EvidencePoint{
		frameworkPoint(1, "丙氨酸氨基转移酶测定", "20", "U/L", "7-40", contract.FlagNormal),
		frameworkPoint(2, "天门冬氨酸氨基转移酶测定", "18", "U/L", "13-35", contract.FlagNormal),
		frameworkPoint(3, "碱性磷酸酶测定", "50", "U/L", "40-100", contract.FlagNormal),
	}
	labels := frameworkLabels(t, testRequest(EntryPanel, pipeline.PanelLiver, 12), frameworkFacts(points...), nil)
	for _, code := range []string{LabelLiverRValue, LabelLiverTransaminaseGrade} {
		if _, ok := labelByCode(labels, code); ok {
			t.Fatalf("%s emitted for normal liver panel: %+v", code, labels)
		}
	}
	if _, ok := labelByCode(labels, LabelLiverASTALTRatio); !ok {
		t.Fatal("AST/ALT ratio should still be computed")
	}
}

func TestTransaminaseGradeBoundaries(t *testing.T) {
	cases := []struct {
		ratio string
		want  string
	}{
		{"1.9", "临界"},
		{"2.0", "轻度"},
		{"4.9", "轻度"},
		{"5.0", "中度"},
		{"15.0", "中度"},
		{"15.1", "重度"},
	}
	for _, c := range cases {
		t.Run(c.ratio, func(t *testing.T) {
			value := formatFloat(parseTestFloat(c.ratio)*40, 1)
			points := []EvidencePoint{
				frameworkPoint(1, "丙氨酸氨基转移酶测定", value, "U/L", "7-40", contract.FlagHigh),
			}
			labels := frameworkLabels(t, testRequest(EntryPanel, pipeline.PanelLiver, 12), frameworkFacts(points...), nil)
			label, ok := labelByCode(labels, LabelLiverTransaminaseGrade)
			if !ok || label.Label != "转氨酶升高："+c.want {
				t.Fatalf("grade label = %+v, want %s", label, c.want)
			}
		})
	}
}

func TestTransaminaseGradeNeedsULN(t *testing.T) {
	points := []EvidencePoint{
		frameworkPoint(1, "丙氨酸氨基转移酶测定", "200", "U/L", "", contract.FlagHigh),
	}
	labels := frameworkLabels(t, testRequest(EntryPanel, pipeline.PanelLiver, 12), frameworkFacts(points...), nil)
	if _, ok := labelByCode(labels, LabelLiverTransaminaseGrade); ok {
		t.Fatal("grade label emitted without ULN")
	}
}

func TestASTALTRatio(t *testing.T) {
	points := []EvidencePoint{
		frameworkPoint(1, "丙氨酸氨基转移酶测定", "80", "U/L", "7-40", contract.FlagHigh),
		frameworkPoint(2, "天门冬氨酸氨基转移酶测定", "40", "U/L", "13-35", contract.FlagHigh),
	}
	labels := frameworkLabels(t, testRequest(EntryPanel, pipeline.PanelLiver, 12), frameworkFacts(points...), nil)
	label, ok := labelByCode(labels, LabelLiverASTALTRatio)
	if !ok || label.Value == nil || *label.Value != "0.50（报告日期 2026-10-01）" {
		t.Fatalf("AST/ALT label = %+v, want 0.50 with report date", label)
	}
}

func TestHemoglobinCutoffByAgeAndSex(t *testing.T) {
	cases := []struct {
		name   string
		birth  string
		sex    string
		onDate string
		want   float64
		wantOK bool
	}{
		{"infant", "2026-04-01", pipeline.SexMale, "2026-10-01", 105, true},
		{"toddler", "2024-01-01", pipeline.SexFemale, "2026-10-01", 110, true},
		{"school", "2018-01-01", pipeline.SexMale, "2026-10-01", 115, true},
		{"adolescent", "2013-01-01", pipeline.SexMale, "2026-10-01", 120, true},
		{"adult female", "1990-01-01", pipeline.SexFemale, "2026-10-01", 120, true},
		{"adult male", "1990-01-01", pipeline.SexMale, "2026-10-01", 130, true},
		{"under six months", "2026-06-01", pipeline.SexMale, "2026-10-01", 0, false},
		{"adult missing sex", "1990-01-01", "", "2026-10-01", 0, false},
		{"65 years 11 months", "1960-11-01", pipeline.SexMale, "2026-10-01", 130, true},
		{"over 65", "1960-09-01", pipeline.SexMale, "2026-10-01", 0, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, ok := whoHemoglobinCutoff(c.birth, c.sex, c.onDate)
			if got != c.want || ok != c.wantOK {
				t.Fatalf("cutoff = %v,%v want %v,%v", got, ok, c.want, c.wantOK)
			}
		})
	}
}

func TestHemoglobinCutoffLabelNotesReportRange(t *testing.T) {
	points := []EvidencePoint{
		frameworkPoint(1, "血红蛋白测定", "108", "g/L", "115-150", contract.FlagLow),
	}
	req := testRequest(EntryPanel, pipeline.PanelCBC, 12)
	req.MemberBirthDate = "2018-01-01"
	req.MemberSex = pipeline.SexMale
	labels := frameworkLabels(t, req, frameworkFacts(points...), nil)
	label, ok := labelByCode(labels, LabelCBCHemoglobinCutoff)
	if !ok || label.Label != "血红蛋白低于该年龄段切点" {
		t.Fatalf("Hb cutoff label = %+v", label)
	}
	if !stringsContains(label.Explanation, "115-150") {
		t.Fatalf("explanation = %q, want report range", label.Explanation)
	}
}

func TestHemoglobinCutoffUsesReportDate(t *testing.T) {
	// 报告日期时 11 岁 10 个月,分析日已满 12 岁;切点必须按报告日期取 115 而非 120。
	point := frameworkPoint(1, "血红蛋白测定", "117", "g/L", "115-150", contract.FlagNormal)
	point.ReportDate = strPtr("2026-10-01")
	req := testRequest(EntryPanel, pipeline.PanelCBC, 12)
	req.MemberBirthDate = "2014-11-02"
	req.MemberSex = pipeline.SexMale
	labels := frameworkLabels(t, req, frameworkFacts(point), nil)
	if _, ok := labelByCode(labels, LabelCBCHemoglobinCutoff); ok {
		t.Fatalf("Hb cutoff used analysis date instead of report date: %+v", labels)
	}
}

func TestAnemiaMorphology(t *testing.T) {
	points := []EvidencePoint{
		frameworkPoint(1, "血红蛋白测定", "100", "g/L", "115-150", contract.FlagLow),
		frameworkPoint(2, "平均红细胞体积测定", "72", "fL", "80-100", contract.FlagLow),
		frameworkPoint(3, "平均红细胞血红蛋白含量测定", "24", "pg", "27-34", contract.FlagLow),
		frameworkPoint(4, "平均红细胞血红蛋白浓度测定", "300", "g/L", "316-354", contract.FlagLow),
		frameworkPoint(5, "红细胞分布宽度-CV", "16.2", "%", "11.5-14.5", contract.FlagHigh),
	}
	req := testRequest(EntryPanel, pipeline.PanelCBC, 12)
	req.MemberBirthDate = "2018-01-01"
	req.MemberSex = pipeline.SexMale
	labels := frameworkLabels(t, req, frameworkFacts(points...), nil)
	label, ok := labelByCode(labels, LabelCBCAnemiaMorphology)
	if !ok || label.Label != "小细胞低色素性形态，RDW 升高" {
		t.Fatalf("morphology label = %+v", label)
	}
	if !stringsContains(label.Explanation, "不是诊断") {
		t.Fatalf("explanation = %q, want non-diagnostic wording", label.Explanation)
	}
}

func TestAnemiaMorphologyNamesOnlyLowChromiaIndex(t *testing.T) {
	points := []EvidencePoint{
		frameworkPoint(1, "血红蛋白测定", "100", "g/L", "115-150", contract.FlagLow),
		frameworkPoint(2, "平均红细胞体积测定", "72", "fL", "80-100", contract.FlagLow),
		frameworkPoint(3, "平均红细胞血红蛋白含量测定", "24", "pg", "27-34", contract.FlagLow),
	}
	req := testRequest(EntryPanel, pipeline.PanelCBC, 12)
	req.MemberBirthDate = "2018-01-01"
	req.MemberSex = pipeline.SexMale
	labels := frameworkLabels(t, req, frameworkFacts(points...), nil)
	label, ok := labelByCode(labels, LabelCBCAnemiaMorphology)
	if !ok || !strings.Contains(label.Explanation, "MCH 偏低") ||
		strings.Contains(label.Explanation, "MCH / MCHC 偏低") {
		t.Fatalf("morphology explanation = %+v, want only MCH", label)
	}
}

func TestAnemiaMorphologyNeedsLowHemoglobinAndMCV(t *testing.T) {
	normal := []EvidencePoint{
		frameworkPoint(1, "血红蛋白测定", "130", "g/L", "115-150", contract.FlagNormal),
		frameworkPoint(2, "平均红细胞体积测定", "72", "fL", "80-100", contract.FlagLow),
	}
	req := testRequest(EntryPanel, pipeline.PanelCBC, 12)
	req.MemberBirthDate = "2018-01-01"
	req.MemberSex = pipeline.SexMale
	if _, ok := labelByCode(frameworkLabels(t, req, frameworkFacts(normal...), nil), LabelCBCAnemiaMorphology); ok {
		t.Fatal("morphology emitted without low hemoglobin")
	}
	noMCV := []EvidencePoint{
		frameworkPoint(1, "血红蛋白测定", "100", "g/L", "115-150", contract.FlagLow),
	}
	if _, ok := labelByCode(frameworkLabels(t, req, frameworkFacts(noMCV...), nil), LabelCBCAnemiaMorphology); ok {
		t.Fatal("morphology emitted without MCV")
	}
}

func TestBMIWSTBoundaries(t *testing.T) {
	// 10 岁男童:超重 19.2、肥胖 21.9;身高 140cm 时体重 = BMI * 1.96。
	cases := []struct {
		bmi  float64
		want string
	}{
		{19.1, "未达超重界值"},
		{19.2, "超重"},
		{21.9, "肥胖"},
	}
	for _, c := range cases {
		t.Run(formatFloat(c.bmi, 1), func(t *testing.T) {
			height := 140.0
			weight := c.bmi * 1.96
			growth := InputGrowth{
				MeasuredOn: "2026-10-01", HeightCm: &height, WeightKg: &weight,
				Source: pipeline.GrowthSourceMeasured,
			}
			req := testRequest(EntryPanel, pipeline.PanelCBC, 12)
			req.MemberBirthDate = "2016-10-01"
			req.MemberSex = pipeline.SexMale
			labels := frameworkLabels(t, req, frameworkFacts(), &growth)
			label, ok := labelByCode(labels, LabelBMIStatus)
			if !ok || label.Label != c.want {
				t.Fatalf("BMI status = %+v, want %s", label, c.want)
			}
		})
	}
}

func TestWSTBMIStatusStopsAt18(t *testing.T) {
	if _, ok := wstBMIStatus(pipeline.SexMale, 215, 24); !ok {
		t.Fatal("17y11m should use WS/T")
	}
	if _, ok := wstBMIStatus(pipeline.SexMale, 216, 24); ok {
		t.Fatal("18y0m should not use WS/T")
	}
	if _, ok := whoBMIZ(pipeline.SexMale, 216, 24); !ok {
		t.Fatal("18y0m should use WHO z")
	}
	if _, ok := whoBMIZ(pipeline.SexMale, 228, 24); !ok {
		t.Fatal("19y0m should use WHO z")
	}
	if _, ok := whoBMIZ(pipeline.SexMale, 229, 24); ok {
		t.Fatal("19y1m should be outside WHO table")
	}
}

func TestBMILabelsUsesWHOAfter18(t *testing.T) {
	height, weight := 170.0, 70.0 // BMI 24.22
	growth := InputGrowth{MeasuredOn: "2026-07-01", HeightCm: &height, WeightKg: &weight, Source: pipeline.GrowthSourceMeasured}
	req := testRequest(EntryPanel, pipeline.PanelCBC, 12)
	req.MemberBirthDate = "2008-01-01" // 18y6m
	req.MemberSex = pipeline.SexMale
	labels := frameworkLabels(t, req, frameworkFacts(), &growth)
	status, ok := labelByCode(labels, LabelBMIStatus)
	if !ok || len(status.Sources) == 0 || status.Sources[0].Standard != "WHO 2007" {
		t.Fatalf("BMI status = %+v, want WHO 2007 source after 18", status)
	}
	if _, ok := labelByCode(labels, LabelBMIZScore); !ok {
		t.Fatal("BMI z missing after 18")
	}
}

func TestBMIWithoutDemographicsHasValueOnly(t *testing.T) {
	height, weight := 140.0, 50.0
	growth := InputGrowth{MeasuredOn: "2026-10-01", HeightCm: &height, WeightKg: &weight, Source: pipeline.GrowthSourceMeasured}
	req := testRequest(EntryPanel, pipeline.PanelCBC, 12)
	labels := frameworkLabels(t, req, frameworkFacts(), &growth)
	if _, ok := labelByCode(labels, LabelBMI); !ok {
		t.Fatal("BMI value missing")
	}
	for _, code := range []string{LabelBMIStatus, LabelBMIZScore} {
		if _, ok := labelByCode(labels, code); ok {
			t.Fatalf("%s emitted without birth_date/sex", code)
		}
	}
}

func TestBMIManualSourceHasNoStatus(t *testing.T) {
	height, weight := 140.0, 50.0
	growth := InputGrowth{MeasuredOn: "2026-10-01", HeightCm: &height, WeightKg: &weight, Source: growthSourceManual}
	req := testRequest(EntryPanel, pipeline.PanelCBC, 12)
	req.MemberBirthDate = "2016-10-01"
	req.MemberSex = pipeline.SexMale
	labels := frameworkLabels(t, req, frameworkFacts(), &growth)
	if _, ok := labelByCode(labels, LabelBMI); !ok {
		t.Fatal("BMI value missing")
	}
	if _, ok := labelByCode(labels, LabelBMIStatus); ok {
		t.Fatal("manual source must not emit weight status")
	}
}

func TestBMIWHOZScoreForPreschooler(t *testing.T) {
	height, weight := 110.0, 21.8 // BMI 18.0
	growth := InputGrowth{MeasuredOn: "2026-10-01", HeightCm: &height, WeightKg: &weight, Source: pipeline.GrowthSourceMeasured}
	req := testRequest(EntryPanel, pipeline.PanelCBC, 12)
	req.MemberBirthDate = "2021-04-01" // 5.5 岁
	req.MemberSex = pipeline.SexMale
	labels := frameworkLabels(t, req, frameworkFacts(), &growth)
	zLabel, ok := labelByCode(labels, LabelBMIZScore)
	if !ok || zLabel.Value == nil || !strings.Contains(*zLabel.Value, "百分位") {
		t.Fatalf("WHO z label = %+v, want percentile", zLabel)
	}
	if status, ok := labelByCode(labels, LabelBMIStatus); !ok || status.Label != "超重" {
		t.Fatalf("WHO status = %+v, want 超重", status)
	}
}

func TestFrameworkSkipsClinicalNoteAndOther(t *testing.T) {
	points := []EvidencePoint{
		frameworkPoint(1, "丙氨酸氨基转移酶测定", "200", "U/L", "7-40", contract.FlagHigh),
		frameworkPoint(2, "碱性磷酸酶测定", "100", "U/L", "40-100", contract.FlagNormal),
	}
	clinical := testRequest(EntryClinicalNote, pipeline.PanelClinicalNote, 12)
	if labels := frameworkLabels(t, clinical, frameworkFacts(points...), nil); len(labels) != 0 {
		t.Fatalf("clinical note framework labels = %+v, want none", labels)
	}
	other := testRequest(EntryPanel, pipeline.PanelOther, 12)
	if labels := frameworkLabels(t, other, frameworkFacts(points...), nil); len(labels) != 0 {
		t.Fatalf("other panel framework labels = %+v, want none", labels)
	}
}

func TestFrameworkSourcesDeduplicated(t *testing.T) {
	points := []EvidencePoint{
		frameworkPoint(1, "丙氨酸氨基转移酶测定", "200", "U/L", "7-40", contract.FlagHigh),
		frameworkPoint(2, "天门冬氨酸氨基转移酶测定", "80", "U/L", "13-35", contract.FlagHigh),
		frameworkPoint(3, "碱性磷酸酶测定", "120", "U/L", "40-100", contract.FlagHigh),
	}
	framework := BuildFramework(testRequest(EntryPanel, pipeline.PanelLiver, 12), frameworkFacts(points...), nil)
	sources := FrameworkSources(framework)
	if len(sources) != 3 {
		t.Fatalf("sources = %+v, want three distinct sources", sources)
	}
	if !slices.ContainsFunc(sources, func(source Source) bool { return source.Standard == "AASLD / ACG" }) {
		t.Fatalf("sources = %+v, missing R source", sources)
	}
}

func parseTestFloat(value string) float64 {
	parsed, _ := strconv.ParseFloat(value, 64)
	return parsed
}

func stringsContains(value, substr string) bool {
	return strings.Contains(value, substr)
}

func TestLiverRValueAndGradeDisplayUnroundedValue(t *testing.T) {
	rPoints := []EvidencePoint{
		frameworkPoint(1, "丙氨酸氨基转移酶测定", "237.6", "U/L", "7-40", contract.FlagHigh),
		frameworkPoint(2, "碱性磷酸酶测定", "120", "U/L", "40-100", contract.FlagHigh),
	}
	rLabels := frameworkLabels(t, testRequest(EntryPanel, pipeline.PanelLiver, 12), frameworkFacts(rPoints...), nil)
	rLabel, ok := labelByCode(rLabels, LabelLiverRValue)
	if !ok || rLabel.Label != "混合型" || rLabel.Value == nil || *rLabel.Value != "R = 4.95（报告日期 2026-10-01）" {
		t.Fatalf("R label = %+v, want 混合型 / R = 4.95 with report date", rLabel)
	}

	gradePoints := []EvidencePoint{
		frameworkPoint(1, "丙氨酸氨基转移酶测定", "79", "U/L", "7-40", contract.FlagHigh),
	}
	gradeLabels := frameworkLabels(t, testRequest(EntryPanel, pipeline.PanelLiver, 12), frameworkFacts(gradePoints...), nil)
	grade, ok := labelByCode(gradeLabels, LabelLiverTransaminaseGrade)
	if !ok || grade.Label != "转氨酶升高：临界" || grade.Value == nil || *grade.Value != "ALT 1.98×ULN（报告日期 2026-10-01）" {
		t.Fatalf("grade label = %+v, want 临界 / ALT 1.98×ULN with report date", grade)
	}
}

func TestFormatTrimmedKeepsIntegerCutoffs(t *testing.T) {
	if got := formatTrimmed(110, 0); got != "110" {
		t.Errorf("formatTrimmed(110,0) = %q, want 110", got)
	}
	if got := formatTrimmed(114.6, 1); got != "114.6" {
		t.Errorf("formatTrimmed(114.6,1) = %q, want 114.6", got)
	}
}

func TestWhoBMIStatusUsesUnroundedZ(t *testing.T) {
	if got := whoBMIStatus(1.004); got != bmiStatusOverweight {
		t.Errorf("whoBMIStatus(1.004) = %q, want overweight", got)
	}
}
