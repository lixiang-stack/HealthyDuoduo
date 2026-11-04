package analysis

import (
	"fmt"
	"strings"

	"healthyduoduo/internal/contract"
	"healthyduoduo/internal/pipeline"
)

var (
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
)

func cbcLabels(req Request, facts Facts, pointsByReport map[string][]EvidencePoint) []FrameworkLabel {
	hemoglobinPoints, ok := latestReportWith(facts, pointsByReport, contract.AnalyteHemoglobin)
	if !ok {
		return nil
	}
	hemoglobin := findPoint(hemoglobinPoints, contract.AnalyteHemoglobin)
	if !hemoglobin.ok {
		return nil
	}
	hemoglobinValue, ok := numericValue(hemoglobin.point.Value)
	if !ok {
		return nil
	}
	onDate := dayString(req.Now)
	if hemoglobin.point.ReportDate != nil && *hemoglobin.point.ReportDate != "" {
		onDate = *hemoglobin.point.ReportDate
	}
	cutoff, hasCutoff := whoHemoglobinCutoff(req.MemberBirthDate, req.MemberSex, onDate)
	lowByFlag := hemoglobin.point.Flag == contract.FlagLow
	lowByCutoff := hasCutoff && hemoglobinValueInGL(hemoglobinValue, hemoglobin.point.Unit) < cutoff

	labels := []FrameworkLabel{}
	if lowByCutoff {
		labels = append(labels, hemoglobinCutoffLabel(hemoglobin.point, cutoff))
	}
	if lowByFlag || lowByCutoff {
		morphologyPoints, ok := latestReportWith(facts, pointsByReport, contract.AnalyteHemoglobin, contract.AnalyteMCV)
		if ok {
			morphologyHemoglobin := findPoint(morphologyPoints, contract.AnalyteHemoglobin)
			if morphology, ok := anemiaMorphologyLabel(morphologyHemoglobin.point, morphologyPoints); ok {
				labels = append(labels, morphology)
			}
		}
	}
	return labels
}

func hemoglobinCutoffLabel(hemoglobin EvidencePoint, cutoff float64) FrameworkLabel {
	hemoglobinValue, _ := numericValue(hemoglobin.Value)
	value := fmt.Sprintf("%s g/L（WHO 切点 %s g/L）",
		formatTrimmed(hemoglobinValueInGL(hemoglobinValue, hemoglobin.Unit), 1),
		formatTrimmed(cutoff, 0)) + reportDateSuffix(hemoglobin)
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
	mcv := findPoint(points, contract.AnalyteMCV)
	if !mcv.ok {
		return FrameworkLabel{}, false
	}
	var size morphologySize
	switch mcv.point.Flag {
	case contract.FlagLow:
		size = morphologyMicrocytic
	case contract.FlagHigh:
		size = morphologyMacrocytic
	case contract.FlagNormal:
		size = morphologyNormocytic
	default:
		return FrameworkLabel{}, false
	}

	mch := findPoint(points, contract.AnalyteMCH)
	mchc := findPoint(points, contract.AnalyteMCHC)
	mchLow := mch.ok && mch.point.Flag == contract.FlagLow
	mchcLow := mchc.ok && mchc.point.Flag == contract.FlagLow
	hypochromic := mchLow || mchcLow
	rdw := findPoint(points, contract.AnalyteRDWCV)
	rdwHigh := rdw.ok && rdw.point.Flag == contract.FlagHigh

	label := morphologySizeText(size) + "性形态"
	if hypochromic {
		label = morphologySizeText(size) + "低色素性形态"
	}
	if rdwHigh {
		label += "，RDW 升高"
	}

	explanation := morphologyExplanation(size, morphologyChromiaNote(mchLow, mchcLow, mch.ok, mchc.ok), rdwHigh)
	value := morphologyValue(mcv, mch, mchc, rdw) + reportDateSuffix(hemoglobin, mcv.point)
	ids := []string{hemoglobin.EvidenceID, mcv.point.EvidenceID}
	if mch.ok {
		ids = append(ids, mch.point.EvidenceID)
	}
	if mchc.ok {
		ids = append(ids, mchc.point.EvidenceID)
	}
	if rdw.ok {
		ids = append(ids, rdw.point.EvidenceID)
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

type morphologySizeCopy struct {
	label   string
	mcvNote string
}

var morphologySizeCopies = map[morphologySize]morphologySizeCopy{
	morphologyMicrocytic: {label: "小细胞", mcvNote: "MCV 偏低"},
	morphologyNormocytic: {label: "正细胞", mcvNote: "MCV 在参考范围内"},
	morphologyMacrocytic: {label: "大细胞", mcvNote: "MCV 偏高"},
}

func morphologySizeText(size morphologySize) string {
	copy, ok := morphologySizeCopies[size]
	if !ok {
		copy = morphologySizeCopies[morphologyNormocytic]
	}
	return copy.label
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
	copy, ok := morphologySizeCopies[size]
	if !ok {
		copy = morphologySizeCopies[morphologyNormocytic]
	}
	parts := []string{copy.mcvNote}
	parts = append(parts, chromiaNote)
	if rdwHigh {
		parts = append(parts, "RDW 升高，提示红细胞大小不均一")
	}
	return strings.Join(parts, "；") + "。这是形态学描述，不是诊断。"
}

func morphologyValue(mcv, mch, mchc, rdw foundPoint) string {
	parts := []string{evidenceDisplay(mcv.point)}
	if mch.ok {
		parts = append(parts, evidenceDisplay(mch.point))
	}
	if mchc.ok {
		parts = append(parts, evidenceDisplay(mchc.point))
	}
	if rdw.ok {
		parts = append(parts, evidenceDisplay(rdw.point))
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

const (
	whoHemoglobinMinAgeMonths = 6
	whoHemoglobinMaxAgeMonths = 66 * 12
)

// WHO 2024 血红蛋白切点(g/L):6–23 月、24–59 月、5–11 岁、12–14 岁;
// 15 岁及以上按性别,怀孕状态不在成员模型内,按非孕女性切点处理。
var whoHemoglobinCutoffBands = []struct {
	maxMonths int
	cutoff    float64
}{
	{24, 105},
	{60, 110},
	{144, 115},
	{180, 120},
}

func whoHemoglobinCutoff(birthDate, sex, onDate string) (float64, bool) {
	months, ok := monthsBetween(birthDate, onDate)
	if !ok {
		return 0, false
	}
	if months < whoHemoglobinMinAgeMonths || months >= whoHemoglobinMaxAgeMonths {
		return 0, false
	}
	for _, band := range whoHemoglobinCutoffBands {
		if months < band.maxMonths {
			return band.cutoff, true
		}
	}
	switch sex {
	case pipeline.SexFemale:
		return 120, true
	case pipeline.SexMale:
		return 130, true
	default:
		return 0, false
	}
}

func hemoglobinValueInGL(value float64, unit *string) float64 {
	if unit != nil && strings.EqualFold(strings.TrimSpace(*unit), "g/dL") {
		return value * 10
	}
	return value
}

// wstBMIStatus 6–18 岁 WS/T 586—2018 性别年龄别界值;18 岁生日后改用 WHO z。
