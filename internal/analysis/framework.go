// 框架层(llm-analysis 票 05/06):只输出代码算得出的判读标签,并附固定解释文案。
// 不解释病因、不输出诊断;标签只在规则输入齐全且越过界值时出现,缺 ULN / 年龄 / 性别
// 时宁可不输出。输入是事实层已经过窗口与信任门控的有效点,不另读存储。
package analysis

import (
	"cmp"
	"fmt"
	"math"
	"slices"
	"strconv"
	"strings"
	"time"

	"healthyduoduo/internal/contract"
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

const aasldURL = "https://www.aasld.org/liver-fellow-network/core-series/back-basics/how-approach-elevated-liver-enzymes"

// 来源(标准名 + 条款说明;URL 由代码渲染,模型不得自行书写)。
var (
	liverRSources = []Source{{
		Standard: "AASLD / ACG",
		Note:     "R 值分型；本产品固定边界为 R ≥ 5 肝细胞型、R ≤ 2 胆汁淤积型、2 < R < 5 混合型",
		URL:      strPtr(aasldURL),
	}}
	liverGradeSources = []Source{{
		Standard: "ACG 2016",
		Note:     "转氨酶升高分级：< 2×ULN 临界、2–5× 轻度、5–15× 中度、> 15× 重度",
		URL:      strPtr("https://doi.org/10.1038/ajg.2016.517"),
	}}
	liverASTALTSources = []Source{{
		Standard: "AASLD",
		Note:     "AST/ALT 比值直接相除",
		URL:      strPtr(aasldURL),
	}}
	cbcMorphologySources = []Source{{
		Standard: "WS/T 405 / WS/T 779",
		Note:     "血细胞分析参考区间；形态学分类按单据参考范围组合",
		URL:      strPtr("https://www.nhc.gov.cn/fzs/c100048/202105/481e639c307a4effa14957f4d0d738fd/files/1733125306369_45466.pdf"),
	}}
	hemoglobinCutoffSources = []Source{{
		Standard: "WHO 2024",
		Note:     "血红蛋白贫血切点（6 月龄–65 岁）；本产品只输出切点标签，不输出诊断",
		URL:      strPtr("https://www.who.int/publications/i/item/9789240088542/"),
	}}
	bmiSources = []Source{{
		Standard: "WS/T 586—2018",
		Note:     "6–18 岁性别年龄别 BMI 超重 / 肥胖筛查界值",
		URL:      strPtr("https://www.nhc.gov.cn/wjw/pqt/201803/a7962d1ac01647b9837110bfd2d69b26/files/1739781209962_63132.pdf"),
	}}
	bmiZSources = []Source{{
		Standard: "WHO 2007",
		Note:     "5–19 岁 BMI-for-age z-score 参考",
		URL:      strPtr("https://www.who.int/tools/growth-reference-data-for-5to19-years/indicators/bmi-for-age"),
	}}
)

// BuildFramework 组装框架层;没有实现规则的 Panel / 无最新报告时 labels 为空。
func BuildFramework(req Request, facts Facts, growth *InputGrowth) Framework {
	labels := []FrameworkLabel{}
	if req.Entry == EntryClinicalNote || req.Panel == pipeline.PanelOther {
		return Framework{Labels: labels}
	}
	reportID := latestReportID(facts)
	if reportID != "" {
		points := reportEvidencePoints(facts, reportID)
		switch req.Panel {
		case pipeline.PanelLiver:
			labels = append(labels, liverLabels(points)...)
		case pipeline.PanelCBC:
			labels = append(labels, cbcLabels(req, points)...)
		}
	}
	if growth != nil {
		labels = append(labels, bmiLabels(req, *growth)...)
	}
	return Framework{Labels: labels}
}

// FrameworkSources 框架层引用到的来源去重;HTTP 层与解释层来源合并后返回。
func FrameworkSources(framework Framework) []Source {
	groups := make([][]Source, 0, len(framework.Labels))
	for _, label := range framework.Labels {
		groups = append(groups, label.Sources)
	}
	return MergeSources(groups...)
}

func sameSource(a, b Source) bool {
	return a.Standard == b.Standard && a.Note == b.Note && valueOrEmpty(a.URL) == valueOrEmpty(b.URL)
}

func latestReportID(facts Facts) string {
	for _, report := range facts.Reports {
		if report.ReportID != "" {
			return report.ReportID
		}
	}
	return ""
}

func reportEvidencePoints(facts Facts, reportID string) []EvidencePoint {
	var points []EvidencePoint
	for _, history := range facts.History {
		for _, point := range history.Points {
			if point.ReportID == reportID {
				points = append(points, point)
			}
		}
	}
	return points
}

// indicatorSpec 指标识别:标准 code > LOINC > 规范化名称;不做模糊包含匹配,
// 避免「糖化血红蛋白」误命中「血红蛋白」。
type indicatorSpec struct {
	key    string
	names  []string
	codes  []string
	loincs []string
}

var (
	indicatorALT = indicatorSpec{
		key: "alt",
		names: []string{"丙氨酸氨基转移酶测定", "丙氨酸氨基转移酶", "丙氨酸转氨酶测定", "丙氨酸转氨酶",
			"谷丙转氨酶", "丙氨酸氨基转移酶(ALT)", "丙氨酸转氨酶(ALT)"},
		codes:  []string{"1500104A"},
		loincs: []string{"1742-6"},
	}
	indicatorAST = indicatorSpec{
		key: "ast",
		names: []string{"天门冬氨酸氨基转移酶测定", "天门冬氨酸氨基转移酶", "天冬氨酸转氨酶",
			"天冬氨酸转氨酶(AST)", "谷草转氨酶", "谷草转氨酶(AST)"},
		codes:  []string{"1500204A"},
		loincs: []string{"1920-8"},
	}
	indicatorALP = indicatorSpec{
		key:   "alp",
		names: []string{"碱性磷酸酶测定", "碱性磷酸酶", "碱性磷酸酶(ALP)"},
		codes: []string{"1500404A"}, loincs: []string{"6768-6"},
	}
	indicatorHemoglobin = indicatorSpec{
		key:   "hemoglobin",
		names: []string{"血红蛋白测定", "血红蛋白", "血红蛋白浓度", "血红蛋白量"},
		codes: []string{"0100401A"}, loincs: []string{"718-7"},
	}
	indicatorMCV = indicatorSpec{
		key:   "mcv",
		names: []string{"平均红细胞体积测定", "平均红细胞体积", "红细胞平均体积", "MCV"},
		codes: []string{"0100601A"}, loincs: []string{"787-2"},
	}
	indicatorMCH = indicatorSpec{
		key: "mch",
		names: []string{"平均红细胞血红蛋白含量测定", "平均红细胞血红蛋白含量", "平均血红蛋白量",
			"平均血红蛋白含量", "MCH"},
		codes: []string{"0100701A"}, loincs: []string{"785-6"},
	}
	indicatorMCHC = indicatorSpec{
		key:   "mchc",
		names: []string{"平均红细胞血红蛋白浓度测定", "平均红细胞血红蛋白浓度", "平均血红蛋白浓度", "MCHC"},
		codes: []string{"0100801A"}, loincs: []string{"786-4"},
	}
	indicatorRDWCV = indicatorSpec{
		key: "rdw_cv",
		names: []string{"红细胞分布宽度-CV", "红细胞分布宽度(CV)", "红细胞分布宽度CV",
			"红细胞分布宽度变异系数", "RDW-CV"},
		loincs: []string{"788-0"},
	}
)

func (s indicatorSpec) matches(point EvidencePoint) bool {
	for _, code := range s.codes {
		if strings.EqualFold(point.TrendKey, code) {
			return true
		}
	}
	for _, loinc := range s.loincs {
		if strings.EqualFold(point.TrendKey, loinc) {
			return true
		}
	}
	name := normalizeIndicatorName(point.Name)
	key := normalizeIndicatorName(point.TrendKey)
	for _, candidate := range s.names {
		normalized := normalizeIndicatorName(candidate)
		if name == normalized || key == normalized {
			return true
		}
	}
	return false
}

func normalizeIndicatorName(name string) string {
	value := strings.TrimSpace(name)
	value = strings.ReplaceAll(value, " ", "")
	value = strings.ReplaceAll(value, "　", "")
	value = strings.TrimPrefix(value, "*")
	value = strings.TrimPrefix(value, "＊")
	value = strings.TrimSuffix(value, "测定")
	return value
}

func findIndicator(points []EvidencePoint, spec indicatorSpec) (EvidencePoint, bool) {
	for _, point := range points {
		if spec.matches(point) {
			return point, true
		}
	}
	return EvidencePoint{}, false
}

func liverLabels(points []EvidencePoint) []FrameworkLabel {
	alt, hasALT := findIndicator(points, indicatorALT)
	ast, hasAST := findIndicator(points, indicatorAST)
	alp, hasALP := findIndicator(points, indicatorALP)

	labels := []FrameworkLabel{}
	if hasALT && hasALP {
		if label, ok := liverRValueLabel(alt, alp); ok {
			labels = append(labels, label)
		}
	}
	if label, ok := transaminaseGradeLabel(alt, hasALT, ast, hasAST); ok {
		labels = append(labels, label)
	}
	if hasALT && hasAST {
		if label, ok := astALTRatioLabel(alt, ast); ok {
			labels = append(labels, label)
		}
	}
	return labels
}

func liverRValueLabel(alt, alp EvidencePoint) (FrameworkLabel, bool) {
	altValue, ok := numericValue(alt.Value)
	if !ok {
		return FrameworkLabel{}, false
	}
	alpValue, ok := numericValue(alp.Value)
	if !ok || alpValue <= 0 {
		return FrameworkLabel{}, false
	}
	altULN, ok := pipeline.RefRangeUpperBound(alt.RefRange)
	if !ok || altULN <= 0 {
		return FrameworkLabel{}, false
	}
	alpULN, ok := pipeline.RefRangeUpperBound(alp.RefRange)
	if !ok || alpULN <= 0 {
		return FrameworkLabel{}, false
	}
	altHigh := altValue > altULN
	alpHigher := alpValue > alpULN
	if !altHigh && !alpHigher {
		return FrameworkLabel{}, false
	}
	r := roundTo((altValue/altULN)/(alpValue/alpULN), 2)
	// 固定解释必须与实际升高方向一致:肝细胞型要求转氨酶升高,
	// 胆汁淤积型要求碱性磷酸酶升高,混合型要求两者都升高。
	switch {
	case r >= 5 && !altHigh:
		return FrameworkLabel{}, false
	case r <= 2 && !alpHigher:
		return FrameworkLabel{}, false
	case r > 2 && r < 5 && (!altHigh || !alpHigher):
		return FrameworkLabel{}, false
	}
	label, explanation := rValuePattern(r)
	value := "R = " + formatFloat(r, 2)
	return FrameworkLabel{
		Code:        LabelLiverRValue,
		Label:       label,
		Explanation: explanation,
		Value:       &value,
		EvidenceIDs: evidenceIDs(alt, alp),
		Sources:     liverRSources,
	}, true
}

func rValuePattern(r float64) (string, string) {
	switch {
	case r >= 5:
		return "肝细胞型", "转氨酶升高为主，碱性磷酸酶不明显升高。"
	case r <= 2:
		return "胆汁淤积型", "碱性磷酸酶升高为主，转氨酶不明显升高。"
	default:
		return "混合型", "转氨酶和碱性磷酸酶都升高。"
	}
}

type enzymeElevation struct {
	name  string
	ratio float64
	point EvidencePoint
}

func transaminaseGradeLabel(alt EvidencePoint, hasALT bool, ast EvidencePoint, hasAST bool) (FrameworkLabel, bool) {
	var elevations []enzymeElevation
	if hasALT {
		if elevation, ok := enzymeRatio("ALT", alt); ok && elevation.ratio > 1 {
			elevations = append(elevations, elevation)
		}
	}
	if hasAST {
		if elevation, ok := enzymeRatio("AST", ast); ok && elevation.ratio > 1 {
			elevations = append(elevations, elevation)
		}
	}
	if len(elevations) == 0 {
		return FrameworkLabel{}, false
	}
	slices.SortStableFunc(elevations, func(a, b enzymeElevation) int {
		if a.ratio != b.ratio {
			return cmp.Compare(b.ratio, a.ratio)
		}
		return strings.Compare(a.name, b.name)
	})
	highest := elevations[0]
	grade, explanation := transaminaseGrade(highest.ratio)
	parts := make([]string, 0, len(elevations))
	ids := make([]string, 0, len(elevations))
	for _, elevation := range elevations {
		parts = append(parts, fmt.Sprintf("%s %s×ULN", elevation.name, formatFloat(elevation.ratio, 2)))
		if elevation.point.EvidenceID != "" {
			ids = append(ids, elevation.point.EvidenceID)
		}
	}
	value := fmt.Sprintf("%s %s×ULN", highest.name, formatFloat(highest.ratio, 2))
	return FrameworkLabel{
		Code:        LabelLiverTransaminaseGrade,
		Label:       "转氨酶升高：" + grade,
		Explanation: fmt.Sprintf("%s；本次 %s。", explanation, strings.Join(parts, "，")),
		Value:       &value,
		EvidenceIDs: ids,
		Sources:     liverGradeSources,
	}, true
}

func enzymeRatio(name string, point EvidencePoint) (enzymeElevation, bool) {
	value, ok := numericValue(point.Value)
	if !ok {
		return enzymeElevation{}, false
	}
	uln, ok := pipeline.RefRangeUpperBound(point.RefRange)
	if !ok || uln <= 0 {
		return enzymeElevation{}, false
	}
	return enzymeElevation{name: name, ratio: roundTo(value/uln, 2), point: point}, true
}

func transaminaseGrade(ratio float64) (string, string) {
	switch {
	case ratio < 2:
		return "临界", "转氨酶升高未达 2×ULN，按 ACG 2016 为临界"
	case ratio < 5:
		return "轻度", "转氨酶升高 2–5×ULN，按 ACG 2016 为轻度"
	case ratio <= 15:
		return "中度", "转氨酶升高 5–15×ULN，按 ACG 2016 为中度"
	default:
		return "重度", "转氨酶升高超过 15×ULN，按 ACG 2016 为重度"
	}
}

func astALTRatioLabel(alt, ast EvidencePoint) (FrameworkLabel, bool) {
	altValue, ok := numericValue(alt.Value)
	if !ok || altValue == 0 {
		return FrameworkLabel{}, false
	}
	astValue, ok := numericValue(ast.Value)
	if !ok {
		return FrameworkLabel{}, false
	}
	ratio := roundTo(astValue/altValue, 2)
	value := formatFloat(ratio, 2)
	return FrameworkLabel{
		Code:        LabelLiverASTALTRatio,
		Label:       "AST/ALT 比值 " + value,
		Explanation: "AST 与 ALT 直接相除；比值需结合病史与其他指标理解。",
		Value:       &value,
		EvidenceIDs: evidenceIDs(ast, alt),
		Sources:     liverASTALTSources,
	}, true
}

func cbcLabels(req Request, points []EvidencePoint) []FrameworkLabel {
	hemoglobin, ok := findIndicator(points, indicatorHemoglobin)
	if !ok {
		return nil
	}
	hemoglobinValue, ok := numericValue(hemoglobin.Value)
	if !ok {
		return nil
	}
	onDate := dayString(req.Now)
	if hemoglobin.ReportDate != nil && *hemoglobin.ReportDate != "" {
		onDate = *hemoglobin.ReportDate
	}
	cutoff, hasCutoff := whoHemoglobinCutoff(req.MemberBirthDate, req.MemberSex, onDate)
	lowByFlag := hemoglobin.Flag == contract.FlagLow
	lowByCutoff := hasCutoff && hemoglobinValueInGL(hemoglobinValue, hemoglobin.Unit) < cutoff

	labels := []FrameworkLabel{}
	if lowByCutoff {
		labels = append(labels, hemoglobinCutoffLabel(hemoglobin, cutoff))
	}
	if lowByFlag || lowByCutoff {
		if morphology, ok := anemiaMorphologyLabel(hemoglobin, points); ok {
			labels = append(labels, morphology)
		}
	}
	return labels
}

func hemoglobinCutoffLabel(hemoglobin EvidencePoint, cutoff float64) FrameworkLabel {
	hemoglobinValue, _ := numericValue(hemoglobin.Value)
	value := fmt.Sprintf("%s g/L（WHO 切点 %s g/L）",
		formatTrimmed(hemoglobinValueInGL(hemoglobinValue, hemoglobin.Unit), 1),
		formatTrimmed(cutoff, 0))
	var explanation string
	if hemoglobin.RefRange != nil && strings.TrimSpace(*hemoglobin.RefRange) != "" {
		explanation = fmt.Sprintf(
			"按 WHO 2024 年龄与性别血红蛋白切点；单据参考范围 %s。切点标签不是诊断。",
			*hemoglobin.RefRange)
	} else {
		explanation = "按 WHO 2024 年龄与性别血红蛋白切点；单据未提供参考范围。切点标签不是诊断。"
	}
	return FrameworkLabel{
		Code:        LabelCBCHemoglobinCutoff,
		Label:       "血红蛋白低于该年龄段切点",
		Explanation: explanation,
		Value:       &value,
		EvidenceIDs: evidenceIDs(hemoglobin),
		Sources:     hemoglobinCutoffSources,
	}
}

type morphologySize string

const (
	morphologyMicrocytic morphologySize = "microcytic"
	morphologyNormocytic morphologySize = "normocytic"
	morphologyMacrocytic morphologySize = "macrocytic"
)

func anemiaMorphologyLabel(hemoglobin EvidencePoint, points []EvidencePoint) (FrameworkLabel, bool) {
	mcv, ok := findIndicator(points, indicatorMCV)
	if !ok {
		return FrameworkLabel{}, false
	}
	var size morphologySize
	switch mcv.Flag {
	case contract.FlagLow:
		size = morphologyMicrocytic
	case contract.FlagHigh:
		size = morphologyMacrocytic
	case contract.FlagNormal:
		size = morphologyNormocytic
	default:
		return FrameworkLabel{}, false
	}

	mch, hasMCH := findIndicator(points, indicatorMCH)
	mchc, hasMCHC := findIndicator(points, indicatorMCHC)
	mchLow := hasMCH && mch.Flag == contract.FlagLow
	mchcLow := hasMCHC && mchc.Flag == contract.FlagLow
	hypochromic := mchLow || mchcLow
	rdw, hasRDW := findIndicator(points, indicatorRDWCV)
	rdwHigh := hasRDW && rdw.Flag == contract.FlagHigh

	label := morphologySizeText(size) + "性形态"
	if hypochromic {
		label = morphologySizeText(size) + "低色素性形态"
	}
	if rdwHigh {
		label += "，RDW 升高"
	}

	explanation := morphologyExplanation(size, morphologyChromiaNote(mchLow, mchcLow, hasMCH, hasMCHC), rdwHigh)
	value := morphologyValue(mcv, mch, hasMCH, mchc, hasMCHC, rdw, hasRDW)
	ids := []string{hemoglobin.EvidenceID, mcv.EvidenceID}
	if hasMCH {
		ids = append(ids, mch.EvidenceID)
	}
	if hasMCHC {
		ids = append(ids, mchc.EvidenceID)
	}
	if hasRDW {
		ids = append(ids, rdw.EvidenceID)
	}
	return FrameworkLabel{
		Code:        LabelCBCAnemiaMorphology,
		Label:       label,
		Explanation: explanation,
		Value:       &value,
		EvidenceIDs: nonEmptyStrings(ids),
		Sources:     cbcMorphologySources,
	}, true
}

func morphologySizeText(size morphologySize) string {
	switch size {
	case morphologyMicrocytic:
		return "小细胞"
	case morphologyMacrocytic:
		return "大细胞"
	default:
		return "正细胞"
	}
}

func morphologyChromiaNote(mchLow, mchcLow, hasMCH, hasMCHC bool) string {
	switch {
	case mchLow && mchcLow:
		return "MCH / MCHC 偏低，提示血红蛋白含量低"
	case mchLow:
		return "MCH 偏低，提示血红蛋白含量低"
	case mchcLow:
		return "MCHC 偏低，提示血红蛋白含量低"
	case hasMCH && hasMCHC:
		return "MCH / MCHC 未见偏低"
	default:
		return "MCH / MCHC 数据不完整"
	}
}

func morphologyExplanation(size morphologySize, chromiaNote string, rdwHigh bool) string {
	parts := []string{}
	switch size {
	case morphologyMicrocytic:
		parts = append(parts, "MCV 偏低")
	case morphologyMacrocytic:
		parts = append(parts, "MCV 偏高")
	default:
		parts = append(parts, "MCV 在参考范围内")
	}
	parts = append(parts, chromiaNote)
	if rdwHigh {
		parts = append(parts, "RDW 升高，提示红细胞大小不均一")
	}
	return strings.Join(parts, "；") + "。这是形态学描述，不是诊断。"
}

func morphologyValue(mcv, mch EvidencePoint, hasMCH bool, mchc EvidencePoint, hasMCHC bool, rdw EvidencePoint, hasRDW bool) string {
	parts := []string{evidenceDisplay(mcv)}
	if hasMCH {
		parts = append(parts, evidenceDisplay(mch))
	}
	if hasMCHC {
		parts = append(parts, evidenceDisplay(mchc))
	}
	if hasRDW {
		parts = append(parts, evidenceDisplay(rdw))
	}
	return strings.Join(parts, " · ")
}

func evidenceDisplay(point EvidencePoint) string {
	name := point.Name
	value := valueOrEmpty(point.Value)
	unit := valueOrEmpty(point.UnitDisplay)
	if unit == "" {
		unit = valueOrEmpty(point.Unit)
	}
	if unit == "" {
		return fmt.Sprintf("%s %s", name, value)
	}
	return fmt.Sprintf("%s %s %s", name, value, unit)
}

type bmiStatus string

const (
	bmiStatusNormal            bmiStatus = "normal"
	bmiStatusOverweight        bmiStatus = "overweight"
	bmiStatusObese             bmiStatus = "obese"
	bmiStatusUnderweight       bmiStatus = "underweight"
	bmiStatusSevereUnderweight bmiStatus = "severe_underweight"
)

func bmiStatusText(status bmiStatus, who bool) string {
	switch status {
	case bmiStatusOverweight:
		return "超重"
	case bmiStatusObese:
		return "肥胖"
	case bmiStatusUnderweight:
		return "低于参考范围"
	case bmiStatusSevereUnderweight:
		return "重度偏低"
	default:
		if who {
			return "在参考范围内"
		}
		return "未达超重界值"
	}
}

func bmiLabels(req Request, growth InputGrowth) []FrameworkLabel {
	bmi := pipeline.BMI(growth.HeightCm, growth.WeightKg)
	if bmi == nil {
		return nil
	}
	labels := []FrameworkLabel{bmiValueLabel(growth, *bmi)}
	if growth.Source == growthSourceManual {
		return labels
	}
	ageMonths, ok := ageMonthsBetween(req.MemberBirthDate, growth.MeasuredOn)
	if !ok {
		return labels
	}
	z, hasZ := whoBMIZ(req.MemberSex, ageMonths, *bmi)
	if status, ok := wstBMIStatus(req.MemberSex, ageMonths, *bmi); ok {
		labels = append(labels, bmiStatusLabel(status, *bmi, false))
	} else if hasZ {
		labels = append(labels, bmiStatusLabel(whoBMIStatus(z), *bmi, true))
	}
	if hasZ {
		labels = append(labels, bmiZScoreLabel(z))
	}
	return labels
}

func bmiValueLabel(growth InputGrowth, bmi float64) FrameworkLabel {
	value := fmt.Sprintf("%.1f kg/m²（测量日期 %s）", bmi, growth.MeasuredOn)
	return FrameworkLabel{
		Code:        LabelBMI,
		Label:       fmt.Sprintf("BMI %.1f", bmi),
		Explanation: "按测量时点的身高体重计算：体重(kg) ÷ 身高(m)²。",
		Value:       &value,
		EvidenceIDs: []string{},
		Sources:     bmiSources,
	}
}

func bmiStatusLabel(status bmiStatus, bmi float64, who bool) FrameworkLabel {
	value := fmt.Sprintf("BMI %.1f", bmi)
	explanation := ""
	sources := bmiSources
	if who {
		sources = bmiZSources
		switch status {
		case bmiStatusOverweight:
			explanation = "按 WHO 2007 5–19 岁 BMI 年龄别 z 值，超过 +1 SD 为超重。"
		case bmiStatusObese:
			explanation = "按 WHO 2007 5–19 岁 BMI 年龄别 z 值，超过 +2 SD 为肥胖。"
		case bmiStatusUnderweight:
			explanation = "按 WHO 2007 5–19 岁 BMI 年龄别 z 值，低于 -2 SD。"
		case bmiStatusSevereUnderweight:
			explanation = "按 WHO 2007 5–19 岁 BMI 年龄别 z 值，低于 -3 SD。"
		default:
			explanation = "按 WHO 2007 5–19 岁 BMI 年龄别 z 值，位于 -2 SD 到 +1 SD 之间。"
		}
	} else {
		switch status {
		case bmiStatusOverweight:
			explanation = "BMI 达到同性别同年龄超重界值，未达到肥胖界值（WS/T 586—2018）。"
		case bmiStatusObese:
			explanation = "BMI 达到同性别同年龄肥胖界值（WS/T 586—2018）。"
		default:
			explanation = "BMI 低于同性别同年龄超重界值（WS/T 586—2018）。"
		}
	}
	return FrameworkLabel{
		Code:        LabelBMIStatus,
		Label:       bmiStatusText(status, who),
		Explanation: explanation,
		Value:       &value,
		EvidenceIDs: []string{},
		Sources:     sources,
	}
}

func bmiZScoreLabel(z float64) FrameworkLabel {
	value := fmt.Sprintf("z = %+.2f（第 %d 百分位）", z, whoBMIPercentile(z))
	return FrameworkLabel{
		Code:        LabelBMIZScore,
		Label:       "BMI 年龄别 z 值与百分位",
		Explanation: "按 WHO 2007 5–19 岁 BMI 年龄别参考计算；z 值 > +1 为超重，> +2 为肥胖。",
		Value:       &value,
		EvidenceIDs: []string{},
		Sources:     bmiZSources,
	}
}

func whoBMIPercentile(z float64) int {
	percentile := 0.5 * (1 + math.Erf(z/math.Sqrt2)) * 100
	return int(math.Round(percentile))
}

func whoBMIStatus(z float64) bmiStatus {
	switch {
	case z > 2:
		return bmiStatusObese
	case z > 1:
		return bmiStatusOverweight
	case z < -3:
		return bmiStatusSevereUnderweight
	case z < -2:
		return bmiStatusUnderweight
	default:
		return bmiStatusNormal
	}
}

// whoHemoglobinCutoff WHO 2024 血红蛋白切点(g/L);6 月龄以下不输出。
// 15 岁及以上需要性别;怀孕状态不在成员模型内,按非孕女性切点处理。
func whoHemoglobinCutoff(birthDate, sex, onDate string) (float64, bool) {
	months, ok := ageMonthsBetween(birthDate, onDate)
	if !ok {
		return 0, false
	}
	switch {
	case months < 6:
		return 0, false
	case months >= 66*12:
		return 0, false
	case months < 24:
		return 105, true
	case months < 60:
		return 110, true
	case months < 144:
		return 115, true
	case months < 180:
		return 120, true
	default:
		switch sex {
		case pipeline.SexFemale:
			return 120, true
		case pipeline.SexMale:
			return 130, true
		default:
			return 0, false
		}
	}
}

func hemoglobinValueInGL(value float64, unit *string) float64 {
	if unit != nil && strings.EqualFold(strings.TrimSpace(*unit), "g/dL") {
		return value * 10
	}
	return value
}

// wstBMIStatus 6–18 岁 WS/T 586—2018 性别年龄别界值;年龄按半岁分组。
func wstBMIStatus(sex string, ageMonths int, bmi float64) (bmiStatus, bool) {
	if ageMonths < 72 || ageMonths >= 228 {
		return "", false
	}
	index := (ageMonths - 72) / 6
	if index < 0 || index >= len(wst586Cutoffs) {
		index = len(wst586Cutoffs) - 1
	}
	row := wst586Cutoffs[index]
	var overweight, obese float64
	switch sex {
	case pipeline.SexMale:
		overweight, obese = row.maleOverweight, row.maleObese
	case pipeline.SexFemale:
		overweight, obese = row.femaleOverweight, row.femaleObese
	default:
		return "", false
	}
	switch {
	case bmi >= obese:
		return bmiStatusObese, true
	case bmi >= overweight:
		return bmiStatusOverweight, true
	default:
		return bmiStatusNormal, true
	}
}

// whoBMIZ WHO 2007 5–19 岁 BMI-for-age z-score;5 岁整按 61 月龄行处理。
func whoBMIZ(sex string, ageMonths int, bmi float64) (float64, bool) {
	if bmi <= 0 {
		return 0, false
	}
	var table []whoBMIRow
	switch sex {
	case pipeline.SexMale:
		table = whoBMIBoys
	case pipeline.SexFemale:
		table = whoBMIGirls
	default:
		return 0, false
	}
	if ageMonths < 60 || ageMonths > 228 {
		return 0, false
	}
	if ageMonths < 61 {
		ageMonths = 61
	}
	index := ageMonths - 61
	if index < 0 || index >= len(table) {
		return 0, false
	}
	row := table[index]
	if row.m <= 0 || row.s == 0 {
		return 0, false
	}
	var z float64
	if math.Abs(row.l) < 1e-9 {
		z = math.Log(bmi/row.m) / row.s
	} else {
		z = (math.Pow(bmi/row.m, row.l) - 1) / (row.l * row.s)
	}
	return z, true
}

func ageMonthsBetween(birthDate, onDate string) (int, bool) {
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
		if point.EvidenceID != "" {
			ids = append(ids, point.EvidenceID)
		}
	}
	return ids
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
