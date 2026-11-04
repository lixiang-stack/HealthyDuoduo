// 生成并冻结分析评估集(llm-analysis 票 10):事实层复用 samples/expected/report
// golden 报告,框架层构造边界用例。期望输出只在人工审查后重新生成;CI 测试读取
// 已冻结的 fixtures.json,代码改动改变输出即失败。
//
// 运行(仓库根目录):
//
//	go run ./internal/analysis/gen/eval
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"healthyduoduo/internal/analysis"
	"healthyduoduo/internal/analysis/evalfixture"
	"healthyduoduo/internal/contract"
	"healthyduoduo/internal/pipeline"
)

const (
	defaultSamples = "samples/expected/report"
	defaultOut     = "internal/analysis/testdata/eval/fixtures.json"
	evalNow        = "2026-11-03"
)

type goldenReport struct {
	ReportType string       `json:"report_type"`
	ReportDate *string      `json:"report_date"`
	Status     string       `json:"status"`
	Items      []goldenItem `json:"items"`
}

type goldenItem struct {
	Name          string  `json:"name"`
	Panel         string  `json:"panel"`
	Value         *string `json:"value"`
	Unit          *string `json:"unit"`
	RefRange      *string `json:"ref_range"`
	Code          *string `json:"code"`
	Loinc         *string `json:"loinc"`
	Analyte       *string `json:"analyte"`
	UnitDisplay   *string `json:"unit_display"`
	UnitStatus    *string `json:"unit_status"`
	Flag          string  `json:"flag"`
	LowConfidence bool    `json:"low_confidence"`
}

type loadedReport struct {
	base       string
	id         string
	date       string
	reportType string
	status     string
	points     []analysis.InputPoint
}

func main() {
	samplesDir := flag.String("samples", defaultSamples, "golden report directory")
	outPath := flag.String("out", defaultOut, "frozen fixture path")
	flag.Parse()

	reports, err := loadGoldenReports(*samplesDir)
	if err != nil {
		fatal("%v", err)
	}
	fixture := evalfixture.Fixture{
		Notice:      "Frozen evaluation outputs. Regenerate only with `go run ./internal/analysis/gen/eval`, then review the diff.",
		RuleVersion: analysis.RuleVersion,
		Facts:       buildFactCases(reports),
		Framework:   buildFrameworkCases(),
	}
	raw, err := json.MarshalIndent(fixture, "", "  ")
	if err != nil {
		fatal("marshal fixture: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(*outPath), 0o755); err != nil {
		fatal("create output directory: %v", err)
	}
	if err := os.WriteFile(*outPath, append(raw, '\n'), 0o644); err != nil {
		fatal("write %s: %v", *outPath, err)
	}
	fmt.Printf("frozen %d facts cases + %d framework cases -> %s\n",
		len(fixture.Facts), len(fixture.Framework), *outPath)
}

func loadGoldenReports(dir string) ([]*loadedReport, error) {
	paths, err := filepath.Glob(filepath.Join(dir, "*.json"))
	if err != nil {
		return nil, err
	}
	if len(paths) == 0 {
		return nil, fmt.Errorf("no golden reports under %s", dir)
	}
	sort.Strings(paths)
	reports := make([]*loadedReport, 0, len(paths))
	for _, path := range paths {
		raw, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", path, err)
		}
		var golden goldenReport
		if err := json.Unmarshal(raw, &golden); err != nil {
			return nil, fmt.Errorf("parse %s: %w", path, err)
		}
		base := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
		report := &loadedReport{
			base:       base,
			id:         reportID(base),
			reportType: golden.ReportType,
			status:     golden.Status,
		}
		if golden.ReportDate != nil {
			report.date = *golden.ReportDate
		}
		for index, item := range golden.Items {
			report.points = append(report.points, inputPoint(report, item, int64(index+1)))
		}
		reports = append(reports, report)
	}
	return reports, nil
}

func inputPoint(report *loadedReport, item goldenItem, itemID int64) analysis.InputPoint {
	return analysis.InputPoint{
		MemberID:      1,
		ReportID:      report.id,
		ReportDate:    report.date,
		ReportType:    report.reportType,
		ReportStatus:  report.status,
		Panel:         item.Panel,
		ItemID:        itemID,
		Name:          item.Name,
		Code:          value(item.Code),
		Loinc:         value(item.Loinc),
		Analyte:       value(item.Analyte),
		Unit:          value(item.Unit),
		UnitDisplay:   value(item.UnitDisplay),
		UnitStatus:    value(item.UnitStatus),
		Value:         value(item.Value),
		RefRange:      value(item.RefRange),
		Flag:          item.Flag,
		LowConfidence: item.LowConfidence,
		ReviewStatus:  "unreviewed",
	}
}

// buildFactCases 每个 golden 报告的每个项目组生成 Panel 入口与单指标入口两个 case;
// 历史取目标报告前 24 个月内(最大回溯窗口)的同项目组点,分析时刻锚在目标报告日期之后。
func buildFactCases(reports []*loadedReport) []evalfixture.FactsCase {
	byPanel := make(map[string][]*loadedReport)
	for _, report := range reports {
		panels := make(map[string]bool)
		for _, point := range report.points {
			panels[point.Panel] = true
		}
		for panel := range panels {
			if panel == pipeline.PanelImaging || panel == pipeline.PanelClinicalNote {
				continue
			}
			byPanel[panel] = append(byPanel[panel], report)
		}
	}
	panels := make([]string, 0, len(byPanel))
	for panel := range byPanel {
		panels = append(panels, panel)
	}
	sort.Strings(panels)

	cases := make([]evalfixture.FactsCase, 0)
	for _, panel := range panels {
		group := byPanel[panel]
		sort.SliceStable(group, func(i, j int) bool {
			if group[i].date != group[j].date {
				return group[i].date < group[j].date
			}
			return group[i].base < group[j].base
		})
		for _, target := range group {
			points := panelHistory(panel, group, target)
			if len(points) == 0 {
				continue
			}
			now := analysisNow(target.date)
			birthDate, sex := memberFor(target.date)
			baseRequest := analysis.Request{
				MemberID:        1,
				MemberName:      "评估成员",
				MemberSex:       sex,
				MemberBirthDate: birthDate,
				Panel:           panel,
				Entry:           analysis.EntryPanel,
				WindowMonths:    analysis.DefaultWindowMonths,
				Now:             now,
			}
			panelFacts := analysis.BuildFacts(baseRequest, points, nil)
			cases = append(cases, evalfixture.FactsCase{
				Name:     target.base + "/" + panel,
				Request:  baseRequest,
				Points:   points,
				Expected: panelFacts,
			})

			indicatorName := target.points[0].Name
			for _, point := range target.points {
				if point.Panel == panel {
					indicatorName = point.Name
					break
				}
			}
			indicatorRequest := baseRequest
			indicatorRequest.Entry = analysis.EntryIndicator
			indicatorRequest.IndicatorName = indicatorName
			cases = append(cases, evalfixture.FactsCase{
				Name:     target.base + "/" + panel + "/indicator",
				Request:  indicatorRequest,
				Points:   points,
				Expected: analysis.BuildFacts(indicatorRequest, points, nil),
			})
		}
	}
	return cases
}

func panelHistory(panel string, group []*loadedReport, target *loadedReport) []analysis.InputPoint {
	points := make([]analysis.InputPoint, 0)
	if target.date == "" {
		points = append(points, reportPoints(panel, target)...)
	} else {
		targetDate := parseDate(target.date)
		from := targetDate.AddDate(0, -analysis.MaxWindowMonths, 0)
		for _, report := range group {
			if report.date == "" {
				continue
			}
			date := parseDate(report.date)
			if date.After(targetDate) || date.Before(from) {
				continue
			}
			// 同日多份报告按文件名顺序纳入,避免把目标之后的报告带进来。
			if date.Equal(targetDate) && report.base > target.base {
				continue
			}
			points = append(points, reportPoints(panel, report)...)
		}
	}
	sort.SliceStable(points, func(i, j int) bool {
		if points[i].ReportDate != points[j].ReportDate {
			return points[i].ReportDate < points[j].ReportDate
		}
		if points[i].ReportID != points[j].ReportID {
			return points[i].ReportID < points[j].ReportID
		}
		return points[i].ItemID < points[j].ItemID
	})
	return points
}

func reportPoints(panel string, report *loadedReport) []analysis.InputPoint {
	points := make([]analysis.InputPoint, 0)
	for _, point := range report.points {
		if point.Panel == panel {
			points = append(points, point)
		}
	}
	return points
}

func buildFrameworkCases() []evalfixture.FrameworkCase {
	cases := []evalfixture.FrameworkCase{}
	add := func(name string, req analysis.Request, points []analysis.InputPoint, growth *analysis.InputGrowth) {
		facts := analysis.BuildFacts(req, points, nil)
		cases = append(cases, evalfixture.FrameworkCase{
			Name:              name,
			Request:           req,
			Points:            points,
			Growth:            growth,
			ExpectedFramework: analysis.BuildFramework(req, facts, growth),
		})
	}

	for _, ratio := range []string{"1.9", "2.0", "4.9", "5.0"} {
		// ALP = 120 / ULN 100 = 1.2, so ALT must be ratio × 1.2 × ULN 40 to get R = ratio.
		altValue := ratioFloat(ratio) * 1.2 * 40
		req := evalRequest(pipeline.PanelLiver, pipeline.SexMale, "2016-10-01")
		points := []analysis.InputPoint{
			evalPoint(req.Panel, "liver-r", "2026-10-01", 1, "丙氨酸氨基转移酶测定",
				contract.AnalyteALT, "U/L", "7-40", formatFloat(altValue), contract.FlagHigh),
			evalPoint(req.Panel, "liver-r", "2026-10-01", 2, "碱性磷酸酶测定",
				contract.AnalyteALP, "U/L", "40-100", "120", contract.FlagHigh),
		}
		add("framework/liver_r_value_"+ratio, req, points, nil)
	}

	for _, ratio := range []string{"1.9", "2.0", "4.9", "5.0", "15.0", "15.1"} {
		req := evalRequest(pipeline.PanelLiver, pipeline.SexMale, "2016-10-01")
		points := []analysis.InputPoint{
			evalPoint(req.Panel, "liver-grade", "2026-10-01", 1, "丙氨酸氨基转移酶测定",
				contract.AnalyteALT, "U/L", "7-40", formatFloat(ratioFloat(ratio)*40), contract.FlagHigh),
		}
		add("framework/liver_grade_"+ratio, req, points, nil)
	}

	for _, bmi := range []float64{19.1, 19.2, 21.9} {
		req := evalRequest(pipeline.PanelCBC, pipeline.SexMale, "2016-10-01")
		height, weight := 140.0, bmi*1.96
		growth := &analysis.InputGrowth{
			MeasuredOn: "2026-10-01", HeightCm: &height, WeightKg: &weight,
			Source: pipeline.GrowthSourceMeasured,
		}
		add("framework/bmi_wst_"+formatFloat(bmi), req, evalCBCPoints(req.Panel), growth)
	}

	for _, c := range []struct {
		name string
		bmi  float64
	}{
		{"normal_side", 16.6},
		{"overweight_side", 16.8},
		{"obese_below", 18.3},
		{"obese_above", 18.4},
	} {
		req := evalRequest(pipeline.PanelCBC, pipeline.SexMale, "2021-04-01") // 5.5 岁,仅 WHO z
		height, weight := 110.0, c.bmi*1.1*1.1
		growth := &analysis.InputGrowth{
			MeasuredOn: "2026-10-01", HeightCm: &height, WeightKg: &weight,
			Source: pipeline.GrowthSourceMeasured,
		}
		add("framework/bmi_who_"+c.name, req, evalCBCPoints(req.Panel), growth)
	}

	for _, c := range []struct {
		name  string
		birth string
	}{
		{"17y11m", "2008-10-02"},
		{"18y0m", "2008-10-01"},
		{"19y2m", "2007-08-01"},
	} {
		req := evalRequest(pipeline.PanelCBC, pipeline.SexMale, c.birth)
		height, weight := 170.0, 24.0*1.7*1.7
		growth := &analysis.InputGrowth{
			MeasuredOn: "2026-10-01", HeightCm: &height, WeightKg: &weight,
			Source: pipeline.GrowthSourceMeasured,
		}
		add("framework/bmi_age_"+c.name, req, evalCBCPoints(req.Panel), growth)
	}

	for _, c := range []struct {
		name  string
		birth string
	}{
		{"6m", "2026-04-01"},
		{"23m", "2024-11-01"},
		{"24m", "2024-10-01"},
		{"11y11m", "2014-11-01"},
		{"12y0m", "2014-10-01"},
		{"14y11m", "2011-11-01"},
		{"15y0m", "2011-10-01"},
		{"65y11m", "1960-11-01"},
		{"66y0m", "1960-10-01"},
	} {
		req := evalRequest(pipeline.PanelCBC, pipeline.SexMale, c.birth)
		points := []analysis.InputPoint{
			evalPoint(req.Panel, "cbc-hb", "2026-10-01", 1, "血红蛋白测定",
				contract.AnalyteHemoglobin, "g/L", "115-150", "100", contract.FlagLow),
		}
		add("framework/hb_cutoff_"+c.name, req, points, nil)
	}
	return cases
}

func evalCBCPoints(panel string) []analysis.InputPoint {
	return []analysis.InputPoint{
		evalPoint(panel, "cbc-base", "2026-10-01", 1, "血红蛋白测定",
			contract.AnalyteHemoglobin, "g/L", "115-150", "130", contract.FlagNormal),
	}
}

func evalPoint(panel, reportID, date string, itemID int64, name, analyte, unit, refRange, value, flag string) analysis.InputPoint {
	reportType := "生化"
	if panel == pipeline.PanelCBC {
		reportType = "临检"
	}
	return analysis.InputPoint{
		MemberID:     1,
		ReportID:     reportID,
		ReportDate:   date,
		ReportType:   reportType,
		ReportStatus: "success",
		Panel:        panel,
		ItemID:       itemID,
		Name:         name,
		Analyte:      analyte,
		Unit:         unit,
		UnitDisplay:  unit,
		UnitStatus:   contract.UnitStatusVerified,
		Value:        value,
		RefRange:     refRange,
		Flag:         flag,
		ReviewStatus: "unreviewed",
	}
}

func evalRequest(panel, sex, birthDate string) analysis.Request {
	return analysis.Request{
		MemberID:        1,
		MemberName:      "评估成员",
		MemberSex:       sex,
		MemberBirthDate: birthDate,
		Panel:           panel,
		Entry:           analysis.EntryPanel,
		WindowMonths:    analysis.DefaultWindowMonths,
		Now:             parseDate(evalNow).Add(24 * time.Hour),
	}
}

func analysisNow(reportDate string) time.Time {
	if reportDate == "" {
		return parseDate(evalNow)
	}
	return parseDate(reportDate).Add(24 * time.Hour)
}

func memberFor(reportDate string) (string, string) {
	if reportDate == "" {
		return "2016-01-01", pipeline.SexMale
	}
	birth := parseDate(reportDate).AddDate(-10, 0, 0).Format("2006-01-02")
	return birth, pipeline.SexMale
}

func reportID(base string) string {
	sum := sha256.Sum256([]byte("golden:" + base))
	return hex.EncodeToString(sum[:])
}

func parseDate(value string) time.Time {
	parsed, err := time.Parse("2006-01-02", value)
	if err != nil {
		fatal("parse date %q: %v", value, err)
	}
	return parsed
}

func ratioFloat(value string) float64 {
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil {
		fatal("parse ratio %q: %v", value, err)
	}
	return parsed
}

func formatFloat(value float64) string {
	return strconv.FormatFloat(value, 'f', -1, 64)
}

func value(pointer *string) string {
	if pointer == nil {
		return ""
	}
	return *pointer
}

func fatal(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "gen eval fixtures: "+format+"\n", args...)
	os.Exit(1)
}
