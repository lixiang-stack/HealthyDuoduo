// 定性比较策略(flag_policy.yaml)加载与规范化。
// 数据文件是 Go/Python 共用的比较规则,不是 OCR 抽取词典;Go 只读取它来重算
// 修正后的 flag(ADR-0005),不读取 recognizer 的 *_dict.yaml。
package pipeline

import (
	"fmt"
	"os"
	"strings"

	"golang.org/x/text/unicode/norm"
	"gopkg.in/yaml.v3"
)

type flagProfile struct {
	Normal           map[string]struct{}
	Abnormal         map[string]struct{}
	Informational    map[string]struct{}
	NormalValues     map[string]struct{}
	AbnormalValues   map[string]struct{}
	InformationalAll bool
}

// FlagPolicy 定性比较策略;由 LoadFlagPolicy 从共享 YAML 构建。
type FlagPolicy struct {
	categories         map[string]string
	profiles           map[string]flagProfile
	items              map[string]string
	noReferenceItems   map[string]struct{}
	noReferenceMethods []string
}

type flagPolicyFile struct {
	Normalizers               map[string][]string        `yaml:"normalizers"`
	Profiles                  map[string]flagProfileYAML `yaml:"profiles"`
	Items                     map[string]string          `yaml:"items"`
	NoReferenceItems          []string                   `yaml:"no_reference_items"`
	NoReferenceMethodKeywords []string                   `yaml:"no_reference_method_keywords"`
}

type flagProfileYAML struct {
	Normal           []string `yaml:"normal"`
	Abnormal         []string `yaml:"abnormal"`
	Informational    []string `yaml:"informational"`
	NormalValues     []string `yaml:"normal_values"`
	AbnormalValues   []string `yaml:"abnormal_values"`
	InformationalAll bool     `yaml:"informational_all"`
}

// LoadFlagPolicy 读取并规范化共享策略文件。
func LoadFlagPolicy(path string) (*FlagPolicy, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read flag policy %s: %w", path, err)
	}
	var file flagPolicyFile
	if err := yaml.Unmarshal(raw, &file); err != nil {
		return nil, fmt.Errorf("parse flag policy %s: %w", path, err)
	}
	categories := make(map[string]string)
	for category, aliases := range file.Normalizers {
		for _, alias := range aliases {
			categories[policyText(alias)] = category
		}
	}
	profiles := make(map[string]flagProfile)
	for name, p := range file.Profiles {
		profiles[name] = flagProfile{
			Normal:           stringSet(p.Normal),
			Abnormal:         stringSet(p.Abnormal),
			Informational:    stringSet(p.Informational),
			NormalValues:     policyTextSet(p.NormalValues),
			AbnormalValues:   policyTextSet(p.AbnormalValues),
			InformationalAll: p.InformationalAll,
		}
	}
	return &FlagPolicy{
		categories:         categories,
		profiles:           profiles,
		items:              file.Items,
		noReferenceItems:   stringSet(file.NoReferenceItems),
		noReferenceMethods: file.NoReferenceMethodKeywords,
	}, nil
}

func stringSet(values []string) map[string]struct{} {
	out := make(map[string]struct{}, len(values))
	for _, value := range values {
		out[value] = struct{}{}
	}
	return out
}

func policyTextSet(values []string) map[string]struct{} {
	out := make(map[string]struct{}, len(values))
	for _, value := range values {
		out[policyText(value)] = struct{}{}
	}
	return out
}

func (p *FlagPolicy) noReferenceMethod(method *string) bool {
	if p == nil || method == nil {
		return false
	}
	for _, keyword := range p.noReferenceMethods {
		if strings.Contains(*method, keyword) {
			return true
		}
	}
	return false
}

// policyText 统一全角/空白/横线;与 Python _policy_text 同口径。
func policyText(text string) string {
	s := norm.NFKC.String(strings.TrimSpace(text))
	s = strings.Join(strings.Fields(s), "")
	s = strings.ReplaceAll(s, "–", "-")
	s = strings.ReplaceAll(s, "—", "-")
	s = strings.ReplaceAll(s, "−", "-")
	return s
}
