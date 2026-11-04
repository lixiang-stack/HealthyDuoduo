package analysis

import (
	"fmt"
	"math"

	"healthyduoduo/internal/pipeline"
)

var (
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

type bmiStatus string

const (
	bmiStatusNormal            bmiStatus = "normal"
	bmiStatusOverweight        bmiStatus = "overweight"
	bmiStatusObese             bmiStatus = "obese"
	bmiStatusUnderweight       bmiStatus = "underweight"
	bmiStatusSevereUnderweight bmiStatus = "severe_underweight"
)

type bmiStatusCopy struct {
	whoLabel       string
	wstLabel       string
	whoExplanation string
	wstExplanation string
}

var bmiStatusCopies = map[bmiStatus]bmiStatusCopy{
	bmiStatusOverweight: {
		whoLabel:       "超重",
		wstLabel:       "超重",
		whoExplanation: "按 WHO 2007 5–19 岁 BMI 年龄别 z 值，超过 +1 SD 为超重。",
		wstExplanation: "BMI 达到同性别同年龄超重界值，未达到肥胖界值（WS/T 586—2018）。",
	},
	bmiStatusObese: {
		whoLabel:       "肥胖",
		wstLabel:       "肥胖",
		whoExplanation: "按 WHO 2007 5–19 岁 BMI 年龄别 z 值，超过 +2 SD 为肥胖。",
		wstExplanation: "BMI 达到同性别同年龄肥胖界值（WS/T 586—2018）。",
	},
	bmiStatusUnderweight: {
		whoLabel:       "低于参考范围",
		wstLabel:       "低于参考范围",
		whoExplanation: "按 WHO 2007 5–19 岁 BMI 年龄别 z 值，低于 -2 SD。",
		wstExplanation: "BMI 低于同性别同年龄超重界值（WS/T 586—2018）。",
	},
	bmiStatusSevereUnderweight: {
		whoLabel:       "重度偏低",
		wstLabel:       "重度偏低",
		whoExplanation: "按 WHO 2007 5–19 岁 BMI 年龄别 z 值，低于 -3 SD。",
		wstExplanation: "BMI 低于同性别同年龄超重界值（WS/T 586—2018）。",
	},
	bmiStatusNormal: {
		whoLabel:       "在参考范围内",
		wstLabel:       "未达超重界值",
		whoExplanation: "按 WHO 2007 5–19 岁 BMI 年龄别 z 值，位于 -2 SD 到 +1 SD 之间。",
		wstExplanation: "BMI 低于同性别同年龄超重界值（WS/T 586—2018）。",
	},
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
	ageMonths, ok := monthsBetween(req.MemberBirthDate, growth.MeasuredOn)
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
	copy, ok := bmiStatusCopies[status]
	if !ok {
		copy = bmiStatusCopies[bmiStatusNormal]
	}
	value := fmt.Sprintf("BMI %.1f", bmi)
	sources := bmiSources
	label := copy.wstLabel
	explanation := copy.wstExplanation
	if who {
		sources = bmiZSources
		label = copy.whoLabel
		explanation = copy.whoExplanation
	}
	return FrameworkLabel{
		Code:        LabelBMIStatus,
		Label:       label,
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

const whoBMIZMinAgeMonths = 60

// wstBMIStatus 6–18 岁 WS/T 586—2018 性别年龄别界值;18 岁生日后改用 WHO z。
func wstBMIStatus(sex string, ageMonths int, bmi float64) (bmiStatus, bool) {
	if len(wst586Cutoffs) == 0 {
		return "", false
	}
	first := wst586Cutoffs[0].ageMonths
	last := wst586Cutoffs[len(wst586Cutoffs)-1].ageMonths
	if ageMonths < first || ageMonths >= last {
		return "", false
	}
	index := (ageMonths - first) / 6
	if index < 0 || index >= len(wst586Cutoffs) {
		return "", false
	}
	row := wst586Cutoffs[index]
	if row.ageMonths != first+index*6 {
		return "", false
	}
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

// whoBMIZ WHO 2007 5–19 岁 BMI-for-age z-score;5 岁整按首行(61 月龄)处理。
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
	if len(table) == 0 {
		return 0, false
	}
	first := table[0].month
	last := table[len(table)-1].month
	if ageMonths < whoBMIZMinAgeMonths || ageMonths > last {
		return 0, false
	}
	if ageMonths < first {
		ageMonths = first
	}
	index := ageMonths - first
	if index < 0 || index >= len(table) {
		return 0, false
	}
	row := table[index]
	if row.month != first+index || row.m <= 0 || row.s == 0 {
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
