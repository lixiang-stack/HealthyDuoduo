// flag 重算:核对修正后 value × ref_range → flag(ADR-0005——flag 一律服务端
// 按修正后的值重算,不接受手改)。与识别端 _flag(recognizer/recognizer/postprocess.py)
// 同口径:同一组印刷原样输入在两处必须得出同一比较结果——语义由共享向量表
// schemas/testdata/flag.vectors.json 单点定义,Go 与 Python 双侧读同一份测试
// (internal/pipeline/flag_test.go ↔ recognizer/tests/test_flag_vectors.py)。
package pipeline

import (
	"regexp"
	"strconv"
	"strings"
)

// flag 枚举(schemas/report.schema.json 同源;storage CHECK 同值)。
const (
	FlagNormal  = "normal"
	FlagHigh    = "high"
	FlagLow     = "low"
	FlagUnknown = "unknown"
)

// 识别端 _flag 的形态口径(与 postprocess.py 的 _NUM / _RANGE_RE / _CMP_RE /
// _SEX_PREFIX_RE 逐字同源):数值为纯正数(含小数,不含符号);区间支持 a-b / a--b /
// a~b(-、–、—、~ 皆可,两侧空白任意);单侧比较 <v / ≤v / >v / ≥v(前置匹配)。
var (
	flagNumPattern = `\d+(?:\.\d+)?`
	flagRangeRe    = regexp.MustCompile(`^(` + flagNumPattern + `)\s*[-–—~]{1,2}\s*(` + flagNumPattern + `)$`)
	flagCmpRe      = regexp.MustCompile(`^([<≤>≥])\s*(` + flagNumPattern + `)`)
	flagSexRe      = regexp.MustCompile(`^[男女][::：]\s*`)
)

// ComputeFlag value(印刷原样字符串)与参考范围 → normal/high/low/unknown。
// value 不可数值化(如 '<2.00' 检出限比较式、'70-80' 范围型结果)或参考范围不可解析
// (或上下界写反) → unknown,不猜。
func ComputeFlag(value, refRange *string) string {
	vnum, ok := numValue(value)
	if !ok || refRange == nil {
		return FlagUnknown
	}
	r := flagSexRe.ReplaceAllString(strings.TrimSpace(*refRange), "")
	if m := flagRangeRe.FindStringSubmatch(r); m != nil {
		lo, _ := strconv.ParseFloat(m[1], 64)
		hi, _ := strconv.ParseFloat(m[2], 64)
		switch {
		case lo > hi:
			return FlagUnknown
		case vnum >= lo && vnum <= hi:
			return FlagNormal
		case vnum > hi:
			return FlagHigh
		default:
			return FlagLow
		}
	}
	if m := flagCmpRe.FindStringSubmatch(r); m != nil {
		b, _ := strconv.ParseFloat(m[2], 64)
		upper := m[1] == "<" || m[1] == "≤" // 单侧上界(<b/≤b);单侧下界(>b/≥b)对称。
		switch {
		case upper && vnum < b:
			return FlagNormal
		case upper:
			return FlagHigh
		case vnum > b:
			return FlagNormal
		default:
			return FlagLow
		}
	}
	return FlagUnknown
}

// numValue 印刷原样字符串 → 数值(flag 比较用);不可数值化 → false,不猜。
func numValue(s *string) (float64, bool) {
	if s == nil {
		return 0, false
	}
	v, err := strconv.ParseFloat(strings.TrimSpace(*s), 64)
	if err != nil {
		return 0, false
	}
	return v, true
}
