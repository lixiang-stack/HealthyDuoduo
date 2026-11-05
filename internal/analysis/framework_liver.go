package analysis

import (
	"cmp"
	"fmt"
	"slices"
	"strings"

	"healthyduoduo/internal/contract"
	"healthyduoduo/internal/pipeline"
)

const aasldURL = "https://www.aasld.org/liver-fellow-network/core-series/back-basics/how-approach-elevated-liver-enzymes"

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
)

func liverLabels(facts Facts, pointsByReport map[string][]EvidencePoint) []FrameworkLabel {
	labels := []FrameworkLabel{}
	if points, ok := latestReportWith(facts, pointsByReport, contract.AnalyteALT, contract.AnalyteALP); ok {
		alt := findPoint(points, contract.AnalyteALT)
		alp := findPoint(points, contract.AnalyteALP)
		if label, ok := liverRValueLabel(alt, alp); ok {
			labels = append(labels, label)
		}
	}
	if points, ok := latestReportWithAny(facts, pointsByReport, contract.AnalyteALT, contract.AnalyteAST); ok {
		alt := findPoint(points, contract.AnalyteALT)
		ast := findPoint(points, contract.AnalyteAST)
		if label, ok := transaminaseGradeLabel(alt, ast); ok {
			labels = append(labels, label)
		}
	}
	if points, ok := latestReportWith(facts, pointsByReport, contract.AnalyteALT, contract.AnalyteAST); ok {
		alt := findPoint(points, contract.AnalyteALT)
		ast := findPoint(points, contract.AnalyteAST)
		if label, ok := astALTRatioLabel(alt, ast); ok {
			labels = append(labels, label)
		}
	}
	return labels
}

func liverRValueLabel(alt, alp foundPoint) (FrameworkLabel, bool) {
	r, altHigh, alpHigher, ok := liverRValue(alt, alp)
	if !ok || !rValueDirectionValid(r, altHigh, alpHigher) {
		return FrameworkLabel{}, false
	}
	label, explanation := rValuePattern(r)
	value := "R = " + formatFloat(roundTo(r, 2), 2) + reportDateSuffix(alt.point, alp.point)
	return FrameworkLabel{
		Code:        LabelLiverRValue,
		Label:       label,
		Explanation: explanation,
		Value:       &value,
		EvidenceIDs: evidenceIDs(alt.point, alp.point),
		Sources:     liverRSources,
	}, true
}

// liverRValue 计算 R 值并报告两侧是否高于各自 ULN;输入不足时 ok=false。
func liverRValue(alt, alp foundPoint) (r float64, altHigh, alpHigher, ok bool) {
	if !alt.ok || !alp.ok {
		return 0, false, false, false
	}
	altValue, valid := numericValue(alt.point.Value)
	if !valid {
		return 0, false, false, false
	}
	alpValue, valid := numericValue(alp.point.Value)
	if !valid || alpValue <= 0 {
		return 0, false, false, false
	}
	altULN, valid := pipeline.RefRangeUpperBound(alt.point.RefRange)
	if !valid || altULN <= 0 {
		return 0, false, false, false
	}
	alpULN, valid := pipeline.RefRangeUpperBound(alp.point.RefRange)
	if !valid || alpULN <= 0 {
		return 0, false, false, false
	}
	altHigh = altValue > altULN
	alpHigher = alpValue > alpULN
	if !altHigh && !alpHigher {
		return 0, false, false, false
	}
	// 分类用原始比值,展示才四舍五入;否则 4.9975 会先显示成 5.00 再被判成肝细胞型。
	return (altValue / altULN) / (alpValue / alpULN), altHigh, alpHigher, true
}

// rValueDirectionValid 固定解释必须与实际升高方向一致:肝细胞型要求转氨酶升高,
// 胆汁淤积型要求碱性磷酸酶升高,混合型要求两者都升高。
func rValueDirectionValid(r float64, altHigh, alpHigher bool) bool {
	switch {
	case r >= 5 && !altHigh:
		return false
	case r <= 2 && !alpHigher:
		return false
	case r > 2 && r < 5 && (!altHigh || !alpHigher):
		return false
	}
	return true
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

func transaminaseGradeLabel(alt, ast foundPoint) (FrameworkLabel, bool) {
	var elevations []enzymeElevation
	if alt.ok {
		if elevation, ok := enzymeRatio("ALT", alt.point); ok && elevation.ratio > 1 {
			elevations = append(elevations, elevation)
		}
	}
	if ast.ok {
		if elevation, ok := enzymeRatio("AST", ast.point); ok && elevation.ratio > 1 {
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
		parts = append(parts, fmt.Sprintf("%s %s×ULN", elevation.name, formatFloat(roundTo(elevation.ratio, 2), 2)))
		ids = append(ids, elevation.point.EvidenceID)
	}
	value := fmt.Sprintf("%s %s×ULN", highest.name, formatFloat(roundTo(highest.ratio, 2), 2)) + reportDateSuffix(highest.point)
	return FrameworkLabel{
		Code:        LabelLiverTransaminaseGrade,
		Label:       "转氨酶升高：" + grade,
		Explanation: fmt.Sprintf("%s；本次 %s。", explanation, strings.Join(parts, "，")),
		Value:       &value,
		EvidenceIDs: nonEmptyStrings(ids),
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
	return enzymeElevation{name: name, ratio: value / uln, point: point}, true
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

func astALTRatioLabel(alt, ast foundPoint) (FrameworkLabel, bool) {
	if !alt.ok || !ast.ok {
		return FrameworkLabel{}, false
	}
	altValue, ok := numericValue(alt.point.Value)
	if !ok || altValue == 0 {
		return FrameworkLabel{}, false
	}
	astValue, ok := numericValue(ast.point.Value)
	if !ok {
		return FrameworkLabel{}, false
	}
	ratio := roundTo(astValue/altValue, 2)
	value := formatFloat(ratio, 2) + reportDateSuffix(alt.point, ast.point)
	return FrameworkLabel{
		Code:        LabelLiverASTALTRatio,
		Label:       "AST/ALT 比值 " + formatFloat(ratio, 2),
		Explanation: "AST 与 ALT 直接相除；比值需结合病史与其他指标理解。",
		Value:       &value,
		EvidenceIDs: evidenceIDs(ast.point, alt.point),
		Sources:     liverASTALTSources,
	}, true
}
