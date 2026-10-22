// flag 重算:核对修正后 value × ref_range → flag(ADR-0005——flag 一律服务端
// 按修正后的值重算,不接受手改)。与识别端 _compare(recognizer/recognizer/postprocess.py)
// 同口径:同一组印刷原样输入在两处必须得出同一比较结果——语义由共享向量表
// schemas/testdata/flag.vectors.json 单点定义,Go 与 Python 双侧读同一份测试
// (internal/pipeline/flag_test.go ↔ recognizer/tests/test_flag_vectors.py)。
package pipeline

import (
	"math"
	"regexp"
	"strconv"
	"strings"
)

// flag 枚举(schemas/report.schema.json 同源;storage CHECK 同值)。
// high/low 只用于有方向的数值异常;abnormal 只用于无方向的定性异常。
const (
	FlagNormal   = "normal"
	FlagHigh     = "high"
	FlagLow      = "low"
	FlagAbnormal = "abnormal"
	FlagUnknown  = "unknown"
)

// flag_reason 枚举(schemas/report.schema.json 同源;只对 unknown 有意义)。
const (
	FlagReasonOCRMiss                = "ocr_miss"
	FlagReasonOCRMerge               = "ocr_merge"
	FlagReasonExtractorPairing       = "extractor_pairing"
	FlagReasonNoReference            = "no_reference"
	FlagReasonQualitativeUnsupported = "qualitative_unsupported"
	FlagReasonUnverified             = "unverified"
	FlagReasonNarrative              = "narrative"
	FlagReasonInformational          = "informational"
	FlagReasonIndeterminate          = "indeterminate"
	FlagReasonScalarRef              = "scalar_ref"
	FlagReasonOther                  = "other"
)

// 识别端 _compare 的形态口径(与 postprocess.py 的 _NUM / _RANGE_RE / _CMP_RE /
// _SEX_PREFIX_RE 逐字同源):数值为纯正数(含小数,不含符号);区间支持 a-b / a--b /
// a~b(-、–、—、~ 皆可,两侧空白任意);单侧比较 <v / ≤v / >v / ≥v(前置匹配)。
var (
	flagNumPattern = `\d+(?:\.\d+)?`
	flagRangeRe    = regexp.MustCompile(`^(` + flagNumPattern + `)\s*[-–—~]{1,2}\s*(` + flagNumPattern + `)$`)
	flagCmpRe      = regexp.MustCompile(`^([<≤>≥])\s*(` + flagNumPattern + `)`)
	flagScalarRe   = regexp.MustCompile(`^(` + flagNumPattern + `)$`)
	flagSexRe      = regexp.MustCompile(`^[男女][::：]\s*`)
)

// ComputeFlag 旧数值入口:只做数值/单侧/区间比较,不加载定性策略。
// 生产路径(修正重算)用 Compare,以便同时得到 flag_reason。
func ComputeFlag(value, refRange *string) string {
	flag, _ := Compare("", value, refRange, nil)
	return flag
}

// Compare 指标项 name + value × ref_range → (flag, reason);reason 只对 unknown 非空。
// policy 为 nil 时只做数值比较(定性值 → unknown/other)。
func Compare(name string, value, refRange *string, policy *FlagPolicy) (string, string) {
	if value == nil || strings.TrimSpace(*value) == "" {
		return FlagUnknown, FlagReasonOther
	}
	if refRange == nil && policy != nil {
		if _, ok := policy.noReferenceItems[name]; ok {
			return FlagUnknown, FlagReasonNoReference
		}
	}
	vform, vok := parseNumForm(*value, false)
	rform, rok := parseNumFormPtr(refRange, true)
	if vok && rok {
		return compareNumeric(vform, rform)
	}
	if vok {
		return FlagUnknown, FlagReasonOther
	}
	if rok {
		return FlagUnknown, FlagReasonQualitativeUnsupported
	}
	return compareQualitative(name, value, refRange, policy)
}

type numForm struct {
	kind string // scalar | interval | bound
	a    float64
	b    float64
	op   string
}

func parseNumFormPtr(text *string, ref bool) (numForm, bool) {
	if text == nil {
		return numForm{}, false
	}
	return parseNumForm(*text, ref)
}

func parseNumForm(text string, ref bool) (numForm, bool) {
	s := strings.TrimSpace(text)
	if ref {
		s = flagSexRe.ReplaceAllString(s, "")
	}
	if m := flagRangeRe.FindStringSubmatch(s); m != nil {
		lo, _ := strconv.ParseFloat(m[1], 64)
		hi, _ := strconv.ParseFloat(m[2], 64)
		return numForm{kind: "interval", a: lo, b: hi}, true
	}
	if m := flagCmpRe.FindStringSubmatch(s); m != nil {
		tail := strings.TrimSpace(s[len(m[0]):])
		if ref || tail == "" {
			b, _ := strconv.ParseFloat(m[2], 64)
			return numForm{kind: "bound", a: b, op: m[1]}, true
		}
	}
	if m := flagScalarRe.FindStringSubmatch(s); m != nil {
		v, _ := strconv.ParseFloat(m[1], 64)
		return numForm{kind: "scalar", a: v}, true
	}
	return numForm{}, false
}

func valueInterval(form numForm) (float64, float64, bool) {
	switch form.kind {
	case "interval":
		if form.a <= form.b {
			return form.a, form.b, true
		}
		return 0, 0, false
	case "bound":
		if form.op == "<" || form.op == "≤" {
			if form.a >= 0 {
				return 0, form.a, true
			}
			return 0, 0, false
		}
		return form.a, math.Inf(1), true
	default:
		return form.a, form.a, true
	}
}

func compareIntervals(vlo, vhi, rlo, rhi float64) (string, string) {
	switch {
	case vhi < rlo:
		return FlagLow, ""
	case vlo > rhi:
		return FlagHigh, ""
	case vlo >= rlo && vhi <= rhi:
		return FlagNormal, ""
	default:
		return FlagUnknown, FlagReasonIndeterminate
	}
}

func compareNumeric(vform, rform numForm) (string, string) {
	if rform.kind == "scalar" {
		// 标量 0 / 0.0 视为精确零点;其他标量语义不明,不猜。
		if rform.a != 0 {
			return FlagUnknown, FlagReasonScalarRef
		}
		rform = numForm{kind: "interval", a: 0, b: 0}
	}
	if rform.kind == "interval" {
		if rform.a > rform.b {
			return FlagUnknown, FlagReasonOther
		}
		if vform.kind == "scalar" {
			switch {
			case vform.a < rform.a:
				return FlagLow, ""
			case vform.a > rform.b:
				return FlagHigh, ""
			default:
				return FlagNormal, ""
			}
		}
		vlo, vhi, ok := valueInterval(vform)
		if !ok {
			return FlagUnknown, FlagReasonOther
		}
		return compareIntervals(vlo, vhi, rform.a, rform.b)
	}
	if vform.kind == "bound" {
		vUpper := vform.op == "<" || vform.op == "≤"
		rUpper := rform.op == "<" || rform.op == "≤"
		if vUpper && rUpper {
			if vform.a < rform.a || (vform.a == rform.a && !(vform.op == "≤" && rform.op == "<")) {
				return FlagNormal, ""
			}
			return FlagUnknown, FlagReasonIndeterminate
		}
		if !vUpper && !rUpper {
			if vform.a > rform.a || (vform.a == rform.a && !(vform.op == "≥" && rform.op == ">")) {
				return FlagNormal, ""
			}
			return FlagUnknown, FlagReasonIndeterminate
		}
	}
	vlo, vhi, ok := valueInterval(vform)
	if !ok {
		return FlagUnknown, FlagReasonOther
	}
	if rform.op == "<" || rform.op == "≤" {
		switch {
		case vhi < rform.a:
			return FlagNormal, ""
		case vlo >= rform.a:
			return FlagHigh, ""
		default:
			return FlagUnknown, FlagReasonIndeterminate
		}
	}
	switch {
	case vlo > rform.a:
		return FlagNormal, ""
	case vhi <= rform.a:
		return FlagLow, ""
	default:
		return FlagUnknown, FlagReasonIndeterminate
	}
}

func compareQualitative(name string, value, refRange *string, policy *FlagPolicy) (string, string) {
	if policy == nil {
		return FlagUnknown, FlagReasonOther
	}
	vtext := policyText(*value)
	var rtext string
	if refRange != nil {
		rtext = policyText(*refRange)
	}
	vcat := policy.categories[vtext]
	rcat := policy.categories[rtext]
	// 值、参考都是阴性/短横 → 正常(用户明确口径)。
	if vcat == "negative" && rcat == "negative" {
		return FlagNormal, ""
	}
	profileName := policy.items[name]
	if profileName == "" && rcat == "negative" {
		profileName = "negative_normal"
	}
	if profileName == "" && rcat == "positive" {
		profileName = "positive_normal"
	}
	profile, ok := policy.profiles[profileName]
	if !ok {
		return FlagUnknown, FlagReasonQualitativeUnsupported
	}
	if profile.InformationalAll {
		return FlagUnknown, FlagReasonInformational
	}
	if _, ok := profile.NormalValues[vtext]; ok {
		return FlagNormal, ""
	}
	if _, ok := profile.Normal[vcat]; ok {
		return FlagNormal, ""
	}
	if _, ok := profile.AbnormalValues[vtext]; ok {
		return FlagAbnormal, ""
	}
	if _, ok := profile.Abnormal[vcat]; ok {
		return FlagAbnormal, ""
	}
	if _, ok := profile.Informational[vcat]; ok {
		return FlagUnknown, FlagReasonInformational
	}
	return FlagUnknown, FlagReasonQualitativeUnsupported
}
