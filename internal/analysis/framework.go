// 框架层(llm-analysis 票 05/06):只输出代码算得出的判读标签,并附固定解释文案。
// 不解释病因、不输出诊断;标签只在规则输入齐全且越过界值时出现,缺 ULN / 年龄 / 性别
// 时宁可不输出。输入是事实层已经过窗口与信任门控的有效点,不另读存储。
package analysis

import (
	"math"
	"strconv"
	"strings"
	"time"

	"healthyduoduo/internal/pipeline"
)

// 框架层标签 code(稳定标识,App 只做展示;后续工单可据此挂条目)。
const (
	LabelLiverRValue            = "liver_r_value"
	LabelLiverTransaminaseGrade = "liver_transaminase_grade"
	LabelLiverASTALTRatio       = "liver_ast_alt_ratio"
	LabelCBCAnemiaMorphology    = "cbc_anemia_morphology"
	LabelCBCHemoglobinCutoff    = "cbc_hemoglobin_cutoff"
	LabelBMI                    = "bmi"
	LabelBMIStatus              = "bmi_status"
	LabelBMIZScore              = "bmi_z_score"
)

// 生长记录来源 manual 已从产品移除(票 02 体验修订);这里保留领域守卫,
// 防止旧数据 / 外部调用绕过 HTTP 校验时输出超重 / 肥胖判定。
const growthSourceManual = "manual"

// BuildFramework 组装框架层;clinical_note / other 不输出框架层,
// 其他未实现规则的 Panel 只输出 BMI 等成员级规则;没有有效报告且没有生长记录时 labels 为空。
func BuildFramework(req Request, facts Facts, growth *InputGrowth) Framework {
	labels := []FrameworkLabel{}
	if req.Entry == EntryClinicalNote || req.Panel == pipeline.PanelOther {
		return Framework{Labels: labels}
	}
	pointsByReport := reportPointsByID(facts)
	switch req.Panel {
	case pipeline.PanelLiver:
		labels = append(labels, liverLabels(facts, pointsByReport)...)
	case pipeline.PanelCBC:
		labels = append(labels, cbcLabels(req, facts, pointsByReport)...)
	}
	if growth != nil {
		labels = append(labels, bmiLabels(req, *growth)...)
	}
	return Framework{Labels: labels}
}

// FrameworkSources 框架层引用到的来源去重;HTTP 层与解释层来源合并后返回。
func FrameworkSources(framework Framework) []Source {
	return mergeSourcesFrom(framework.Labels, func(label FrameworkLabel) []Source { return label.Sources })
}

func sameSource(a, b Source) bool {
	return a.Standard == b.Standard && a.Note == b.Note && valueOrEmpty(a.URL) == valueOrEmpty(b.URL)
}

// reportPointsByID 把事实层证据点按报告分组;facts.Reports 已按报告日期降序。
func reportPointsByID(facts Facts) map[string][]EvidencePoint {
	out := make(map[string][]EvidencePoint)
	for _, history := range facts.History {
		for _, point := range history.Points {
			out[point.ReportID] = append(out[point.ReportID], point)
		}
	}
	return out
}

// latestReportWith 返回最近一份包含全部所需 analyte 的报告。
func latestReportWith(facts Facts, pointsByReport map[string][]EvidencePoint, analytes ...string) ([]EvidencePoint, bool) {
	for _, report := range facts.Reports {
		points := pointsByReport[report.ReportID]
		if containsAnalytes(points, analytes...) {
			return points, true
		}
	}
	return nil, false
}

// latestReportWithAny 返回最近一份包含任一所需 analyte 的报告。
func latestReportWithAny(facts Facts, pointsByReport map[string][]EvidencePoint, analytes ...string) ([]EvidencePoint, bool) {
	for _, report := range facts.Reports {
		points := pointsByReport[report.ReportID]
		for _, analyte := range analytes {
			if _, ok := findIndicator(points, analyte); ok {
				return points, true
			}
		}
	}
	return nil, false
}

func containsAnalytes(points []EvidencePoint, analytes ...string) bool {
	for _, analyte := range analytes {
		if _, ok := findIndicator(points, analyte); !ok {
			return false
		}
	}
	return true
}

// 框架层只按识别端写入的 analyte key 找点;名称/别名/code/LOINC 映射由识别端词典唯一维护。
func findIndicator(points []EvidencePoint, analyte string) (EvidencePoint, bool) {
	if analyte == "" {
		return EvidencePoint{}, false
	}
	for _, point := range points {
		if point.Analyte == analyte {
			return point, true
		}
	}
	return EvidencePoint{}, false
}

type foundPoint struct {
	point EvidencePoint
	ok    bool
}

func findPoint(points []EvidencePoint, analyte string) foundPoint {
	point, ok := findIndicator(points, analyte)
	return foundPoint{point: point, ok: ok}
}

func reportDateSuffix(points ...EvidencePoint) string {
	for _, point := range points {
		if point.ReportDate != nil && strings.TrimSpace(*point.ReportDate) != "" {
			return "（报告日期 " + *point.ReportDate + "）"
		}
	}
	return ""
}

// monthsBetween 返回两个 YYYY-MM-DD 日期之间的完整月数;也用于报告相对时间。
func monthsBetween(birthDate, onDate string) (int, bool) {
	born, err := time.Parse("2006-01-02", strings.TrimSpace(birthDate))
	if err != nil {
		return 0, false
	}
	on, err := time.Parse("2006-01-02", strings.TrimSpace(onDate))
	if err != nil || on.Before(born) {
		return 0, false
	}
	months := (on.Year()-born.Year())*12 + int(on.Month()-born.Month())
	if on.Day() < born.Day() {
		months--
	}
	if months < 0 {
		return 0, false
	}
	return months, true
}

func evidenceIDs(points ...EvidencePoint) []string {
	ids := make([]string, 0, len(points))
	for _, point := range points {
		ids = append(ids, point.EvidenceID)
	}
	return nonEmptyStrings(ids)
}

func nonEmptyStrings(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		if value != "" {
			out = append(out, value)
		}
	}
	return out
}

func roundTo(value float64, decimals int) float64 {
	factor := math.Pow(10, float64(decimals))
	return math.Round(value*factor) / factor
}

func formatFloat(value float64, decimals int) string {
	return strconv.FormatFloat(value, 'f', decimals, 64)
}

func formatTrimmed(value float64, decimals int) string {
	text := formatFloat(value, decimals)
	if decimals == 0 {
		return text
	}
	text = strings.TrimRight(text, "0")
	return strings.TrimRight(text, ".")
}

func strPtr(value string) *string { return &value }
