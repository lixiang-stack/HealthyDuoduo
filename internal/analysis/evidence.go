// 云端证据包(llm-analysis 票 07):只发送脱敏后的指标、相对时间、年龄段、
// 病历诊断 / 处理原文与知识条目原文。姓名、生日、精确日期、报告 id、图像哈希、
// 主诉 / 现病史、整页 OCR 兜底一律不发送。
package analysis

import (
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"sort"
	"strings"
	"time"

	"healthyduoduo/internal/contract"
)

// ErrEvidenceTooLarge 条目原文本身已超过 input 上限;不能裁条目,只能省略解释层。
var ErrEvidenceTooLarge = errors.New("evidence pack exceeds input token limit")

// EvidencePack 发给云端模型的脱敏载荷;字段名即模型看到的 JSON 键。
type EvidencePack struct {
	Panel           string              `json:"panel"`
	Entry           string              `json:"entry"`
	AgeBand         string              `json:"age_band,omitempty"`
	Points          []EvidencePackPoint `json:"points"`
	Notes           []EvidencePackNote  `json:"notes"`
	Entries         []EvidencePackEntry `json:"entries"`
	ExcludedReasons []string            `json:"excluded_reasons,omitempty"`
}

// EvidencePackPoint 一个指标证据点;EvidenceID 是本次请求内的临时编号(e1...),
// 不是报告 sha 片段,模型输出引用后再映射回真实证据 id。
type EvidencePackPoint struct {
	EvidenceID   string `json:"evidence_id"`
	Name         string `json:"name"`
	Value        string `json:"value"`
	Unit         string `json:"unit,omitempty"`
	RefRange     string `json:"ref_range,omitempty"`
	Flag         string `json:"flag"`
	RelativeTime string `json:"relative_time"`
}

// EvidencePackNote 病历诊断 / 处理原文;主诉 / 现病史 / 整页 OCR 兜底不进入。
type EvidencePackNote struct {
	RelativeTime  string   `json:"relative_time,omitempty"`
	OutsideWindow bool     `json:"outside_window,omitempty"`
	Diagnosis     []string `json:"diagnosis,omitempty"`
	Treatment     []string `json:"treatment,omitempty"`
}

// EvidencePackEntry 命中条目的医学原文与来源;条目原文不参与裁剪。
type EvidencePackEntry struct {
	EntryID string   `json:"entry_id"`
	Text    string   `json:"text"`
	Sources []Source `json:"source_ids"`
}

// EvidenceMapping 临时证据 id → 事实层证据点;用于把模型输出映射回真实 id 并校验数字。
type EvidenceMapping map[string]EvidencePoint

// BuildEvidencePack 从事实层与命中条目组装脱敏载荷;只读取事实层已门控的点。
func BuildEvidencePack(req Request, facts Facts, entries []KnowledgeEntry) (EvidencePack, EvidenceMapping) {
	points := make([]EvidencePoint, 0)
	for _, history := range facts.History {
		points = append(points, history.Points...)
	}
	sort.SliceStable(points, func(i, j int) bool {
		left, right := valueOrEmpty(points[i].ReportDate), valueOrEmpty(points[j].ReportDate)
		if left != right {
			return left < right
		}
		return points[i].Name < points[j].Name
	})
	pack := EvidencePack{
		Panel:           facts.Panel,
		Entry:           facts.Entry,
		AgeBand:         ageBand(req.MemberBirthDate, dayString(req.Now)),
		Points:          make([]EvidencePackPoint, 0, len(points)),
		Notes:           make([]EvidencePackNote, 0, len(facts.Notes)),
		Entries:         make([]EvidencePackEntry, 0, len(entries)),
		ExcludedReasons: excludedReasons(facts.Excluded),
	}
	mapping := make(EvidenceMapping, len(points))
	for index, point := range points {
		id := fmt.Sprintf("e%d", index+1)
		mapping[id] = point
		pack.Points = append(pack.Points, EvidencePackPoint{
			EvidenceID:   id,
			Name:         point.Name,
			Value:        valueOrEmpty(point.Value),
			Unit:         valueOrEmpty(point.Unit),
			RefRange:     valueOrEmpty(point.RefRange),
			Flag:         point.Flag,
			RelativeTime: relativeTime(valueOrEmpty(point.ReportDate), req.Now),
		})
	}
	notes := slices.Clone(facts.Notes)
	sort.SliceStable(notes, func(i, j int) bool {
		left, right := valueOrEmpty(notes[i].ReportDate), valueOrEmpty(notes[j].ReportDate)
		if left != right {
			return left < right
		}
		return notes[i].ReportID < notes[j].ReportID
	})
	for _, note := range notes {
		packNote := EvidencePackNote{
			RelativeTime:  relativeTime(valueOrEmpty(note.ReportDate), req.Now),
			OutsideWindow: note.OutsideWindow,
		}
		for _, section := range note.Sections {
			text := strings.TrimSpace(section.Text)
			if text == "" {
				continue
			}
			switch section.Kind {
			case contract.SectionKindDiagnosis:
				packNote.Diagnosis = append(packNote.Diagnosis, text)
			case contract.SectionKindTreatment:
				packNote.Treatment = append(packNote.Treatment, text)
			}
		}
		if len(packNote.Diagnosis) == 0 && len(packNote.Treatment) == 0 {
			continue
		}
		pack.Notes = append(pack.Notes, packNote)
	}
	for _, entry := range entries {
		pack.Entries = append(pack.Entries, EvidencePackEntry{
			EntryID: entry.ID,
			Text:    entry.Text(),
			Sources: entry.Sources,
		})
	}
	return pack, mapping
}

func excludedReasons(excluded []Exclusion) []string {
	seen := make(map[string]bool)
	out := make([]string, 0)
	for _, item := range excluded {
		if item.Reason == "" || seen[item.Reason] {
			continue
		}
		seen[item.Reason] = true
		out = append(out, item.Reason)
	}
	slices.Sort(out)
	return out
}

func (entry KnowledgeEntry) Text() string {
	parts := []string{}
	if text := strings.TrimSpace(entry.Explanation); text != "" {
		parts = append(parts, text)
	}
	for _, text := range entry.CheckDirections {
		if trimmed := strings.TrimSpace(text); trimmed != "" {
			parts = append(parts, trimmed)
		}
	}
	for _, text := range entry.GeneralAdvice {
		if trimmed := strings.TrimSpace(text); trimmed != "" {
			parts = append(parts, trimmed)
		}
	}
	if len(entry.TrackItems) > 0 {
		parts = append(parts, "建议持续记录："+strings.Join(entry.TrackItems, "、"))
	}
	return strings.Join(parts, "\n")
}

// TrimEvidencePack 超过 input 上限时先裁最旧指标点,再裁最旧病历;条目原文不裁。
func TrimEvidencePack(pack EvidencePack, limit int) (EvidencePack, error) {
	if limit <= 0 {
		return pack, nil
	}
	for {
		if estimatePackTokens(pack) <= limit {
			return pack, nil
		}
		switch {
		case len(pack.Points) > 0:
			pack.Points = pack.Points[1:]
		case len(pack.Notes) > 0:
			pack.Notes = pack.Notes[1:]
		default:
			return pack, ErrEvidenceTooLarge
		}
	}
}

// filterEvidenceMapping 裁剪后只保留仍在 pack 里的临时证据 id,
// 防止模型引用已被裁掉的证据点。
func filterEvidenceMapping(pack EvidencePack, mapping EvidenceMapping) EvidenceMapping {
	filtered := make(EvidenceMapping, len(pack.Points))
	for _, point := range pack.Points {
		if real, ok := mapping[point.EvidenceID]; ok {
			filtered[point.EvidenceID] = real
		}
	}
	return filtered
}

func estimatePackTokens(pack EvidencePack) int {
	raw, err := json.Marshal(pack)
	if err != nil {
		return 0
	}
	return EstimateTokens(string(raw))
}

// EstimateTokens 保守估算:非 ASCII 字符按 1 token,ASCII 按 4 字符 1 token。
// 不引入 tokenizer 依赖;上限用于裁剪,保守估算比精确值更安全。
func EstimateTokens(text string) int {
	ascii, nonASCII := 0, 0
	for _, r := range text {
		if r < 128 {
			ascii++
		} else {
			nonASCII++
		}
	}
	return (ascii+3)/4 + nonASCII
}

func ageBand(birthDate, onDate string) string {
	months, ok := monthsBetween(birthDate, onDate)
	if !ok {
		return ""
	}
	years := months / 12
	return fmt.Sprintf("%d–%d 岁", years, years+1)
}

func relativeTime(reportDate string, now time.Time) string {
	if strings.TrimSpace(reportDate) == "" {
		return ""
	}
	months, ok := monthsBetween(reportDate, dayString(now))
	if !ok {
		return ""
	}
	if months <= 0 {
		return "T-0"
	}
	return fmt.Sprintf("T-%dm", months)
}
