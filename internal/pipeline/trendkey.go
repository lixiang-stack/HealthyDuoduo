package pipeline

import "strings"

// TrendKey 趋势连接键:标准 code 优先,缺失用 LOINC,再回退词典规范名。
// 同一分析物跨报告 / 跨面板以该键连接;不再只靠名称字符串。
func TrendKey(code, loinc, name string) string {
	if key := strings.TrimSpace(code); key != "" {
		return key
	}
	if key := strings.TrimSpace(loinc); key != "" {
		return key
	}
	return name
}
