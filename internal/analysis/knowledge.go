// 知识条目加载与匹配(llm-analysis 票 08):条目 YAML 与 Go 分析代码一起评审,
// 编译期内嵌;只有 review.decision=accepted 的条目可进入解释层。匹配只按识别端
// 写入的 analyte key 判断 flag 组合,名称 / 别名映射不在分析端维护第二份。
package analysis

import (
	"embed"
	"fmt"
	"io/fs"
	"slices"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"

	"healthyduoduo/internal/contract"
)

//go:embed knowledge/*.yaml
var knowledgeFS embed.FS

// knowledgeFile 单条 YAML 的磁盘形状;version 用整数书写,输出快照时转字符串。
type knowledgeFile struct {
	ID              string            `yaml:"id"`
	Priority        int               `yaml:"priority"`
	Trigger         knowledgeTrigger  `yaml:"trigger"`
	Explanation     string            `yaml:"explanation"`
	CheckDirections []string          `yaml:"check_directions"`
	GeneralAdvice   []string          `yaml:"general_advice"`
	TrackItems      []string          `yaml:"track_items"`
	Sources         []knowledgeSource `yaml:"source_ids"`
	Review          knowledgeReview   `yaml:"review"`
	Version         int               `yaml:"version"`
}

type knowledgeTrigger struct {
	Panel       string   `yaml:"panel"`
	AnyFlagHigh []string `yaml:"any_flag_high"`
	AnyFlagLow  []string `yaml:"any_flag_low"`
	AllNormal   []string `yaml:"all_normal"`
}

type knowledgeSource struct {
	Standard string  `yaml:"standard"`
	Note     string  `yaml:"note"`
	URL      *string `yaml:"url"`
}

type knowledgeReview struct {
	Decision   string `yaml:"decision"`
	ReviewedAt string `yaml:"reviewed_at"`
	Note       string `yaml:"note"`
}

// KnowledgeMatcher 已加载的条目集合;按 priority 排序,多命中取前 2 条。
type KnowledgeMatcher struct {
	entries []KnowledgeEntry
}

// NewKnowledgeMatcher 加载内嵌条目;YAML 不合法时启动失败,不静默跳过条目。
func NewKnowledgeMatcher() (*KnowledgeMatcher, error) {
	entries, err := loadKnowledgeEntries(knowledgeFS)
	if err != nil {
		return nil, err
	}
	return &KnowledgeMatcher{entries: entries}, nil
}

// Match 精确匹配 Panel + flag 组合;只有 accepted 条目参与,多命中按 priority 取前 2。
func (m *KnowledgeMatcher) Match(req Request, facts Facts) []KnowledgeEntry {
	if m == nil {
		return nil
	}
	latest := make(map[string]EvidencePoint, len(facts.History))
	for _, history := range facts.History {
		if history.Latest.Analyte == "" {
			continue
		}
		latest[history.Latest.Analyte] = history.Latest
	}
	matched := make([]KnowledgeEntry, 0, len(m.entries))
	for _, entry := range m.entries {
		if entry.Review.Decision != ReviewDecisionAccepted {
			continue
		}
		if triggerMatches(entry.Trigger, req.Panel, latest) {
			matched = append(matched, entry)
		}
	}
	slices.SortStableFunc(matched, func(a, b KnowledgeEntry) int {
		return a.Priority - b.Priority
	})
	if len(matched) > 2 {
		matched = matched[:2]
	}
	return matched
}

func triggerMatches(trigger KnowledgeTrigger, panel string, latest map[string]EvidencePoint) bool {
	if trigger.Panel == "" || trigger.Panel != panel {
		return false
	}
	if len(trigger.AnyFlagHigh) == 0 && len(trigger.AnyFlagLow) == 0 {
		return false
	}
	anyHigh := false
	for _, analyte := range trigger.AnyFlagHigh {
		if point, ok := latest[analyte]; ok && point.Flag == contract.FlagHigh {
			anyHigh = true
			break
		}
	}
	anyLow := false
	for _, analyte := range trigger.AnyFlagLow {
		if point, ok := latest[analyte]; ok && point.Flag == contract.FlagLow {
			anyLow = true
			break
		}
	}
	if !anyHigh && !anyLow {
		return false
	}
	for _, analyte := range trigger.AllNormal {
		point, ok := latest[analyte]
		if !ok || point.Flag != contract.FlagNormal {
			return false
		}
	}
	return true
}

// EntryVersions 命中条目 id → version,写入分析快照(spec §4.9)。
func EntryVersions(entries []KnowledgeEntry) map[string]string {
	if len(entries) == 0 {
		return nil
	}
	versions := make(map[string]string, len(entries))
	for _, entry := range entries {
		versions[entry.ID] = entry.Version
	}
	return versions
}

func loadKnowledgeEntries(fsys fs.FS) ([]KnowledgeEntry, error) {
	paths, err := fs.Glob(fsys, "knowledge/*.yaml")
	if err != nil {
		return nil, err
	}
	if len(paths) == 0 {
		return nil, fmt.Errorf("knowledge: no entry files found")
	}
	entries := make([]KnowledgeEntry, 0, len(paths))
	ids := make(map[string]string, len(paths))
	for _, path := range paths {
		raw, err := fs.ReadFile(fsys, path)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", path, err)
		}
		var file knowledgeFile
		if err := yaml.Unmarshal(raw, &file); err != nil {
			return nil, fmt.Errorf("parse %s: %w", path, err)
		}
		entry, err := file.entry(path)
		if err != nil {
			return nil, err
		}
		if previous, ok := ids[entry.ID]; ok {
			return nil, fmt.Errorf("%s: duplicate entry id %q (also in %s)", path, entry.ID, previous)
		}
		ids[entry.ID] = path
		entries = append(entries, entry)
	}
	return entries, nil
}

func (file knowledgeFile) entry(path string) (KnowledgeEntry, error) {
	id := strings.TrimSpace(file.ID)
	if id == "" {
		return KnowledgeEntry{}, fmt.Errorf("%s: id is required", path)
	}
	if file.Version <= 0 {
		return KnowledgeEntry{}, fmt.Errorf("%s: version must be positive", path)
	}
	switch file.Review.Decision {
	case ReviewDecisionAccepted, ReviewDecisionRejected:
	default:
		return KnowledgeEntry{}, fmt.Errorf("%s: review.decision must be accepted or rejected", path)
	}
	if strings.TrimSpace(file.Explanation) == "" {
		return KnowledgeEntry{}, fmt.Errorf("%s: explanation is required", path)
	}
	if strings.TrimSpace(file.Trigger.Panel) == "" {
		return KnowledgeEntry{}, fmt.Errorf("%s: trigger.panel is required", path)
	}
	if len(file.Trigger.AnyFlagHigh) == 0 && len(file.Trigger.AnyFlagLow) == 0 {
		return KnowledgeEntry{}, fmt.Errorf("%s: trigger.any_flag_high or any_flag_low is required", path)
	}
	sources := make([]Source, 0, len(file.Sources))
	for _, source := range file.Sources {
		standard := strings.TrimSpace(source.Standard)
		note := strings.TrimSpace(source.Note)
		if standard == "" || note == "" {
			return KnowledgeEntry{}, fmt.Errorf("%s: source_ids.standard and source_ids.note are required", path)
		}
		sources = append(sources, Source{Standard: standard, Note: note, URL: source.URL})
	}
	return KnowledgeEntry{
		ID:       id,
		Priority: file.Priority,
		Trigger: KnowledgeTrigger{
			Panel:       file.Trigger.Panel,
			AnyFlagHigh: file.Trigger.AnyFlagHigh,
			AnyFlagLow:  file.Trigger.AnyFlagLow,
			AllNormal:   file.Trigger.AllNormal,
		},
		Version:         strconv.Itoa(file.Version),
		Review:          KnowledgeReview{Decision: file.Review.Decision, ReviewedAt: file.Review.ReviewedAt, Note: file.Review.Note},
		Explanation:     strings.TrimSpace(file.Explanation),
		CheckDirections: nonEmptyStrings(file.CheckDirections),
		GeneralAdvice:   nonEmptyStrings(file.GeneralAdvice),
		TrackItems:      nonEmptyStrings(file.TrackItems),
		Sources:         sources,
	}, nil
}
