// Package evalfixture 定义分析评估集冻结文件的形状,由 gen/eval 写入、analysis 测试读取。
// 测试与生成器共用同一份结构,避免两处 JSON 契约漂移。
package evalfixture

import "healthyduoduo/internal/analysis"

type Fixture struct {
	Notice      string          `json:"notice"`
	RuleVersion string          `json:"rule_version"`
	Facts       []FactsCase     `json:"facts"`
	Framework   []FrameworkCase `json:"framework"`
}

type FactsCase struct {
	Name     string                `json:"name"`
	Request  analysis.Request      `json:"request"`
	Points   []analysis.InputPoint `json:"points"`
	Expected analysis.Facts        `json:"expected"`
}

type FrameworkCase struct {
	Name              string                `json:"name"`
	Request           analysis.Request      `json:"request"`
	Points            []analysis.InputPoint `json:"points"`
	Growth            *analysis.InputGrowth `json:"growth,omitempty"`
	ExpectedFramework analysis.Framework    `json:"expected_framework"`
}
