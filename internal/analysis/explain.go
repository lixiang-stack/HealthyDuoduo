// 解释层编排(llm-analysis 票 07/08):条目匹配 → 脱敏证据包 → 每日配额 → LLM 调用
// (失败重试 1 次) → bridge_text 结构校验 → 服务端拼接条目四段式原文。
// 前两层不受任何失败影响。
package analysis

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"
)

// 解释层不可用原因(响应 quota.explanation_reason)。
const (
	ExplanationReasonNoEntries      = "no_entries"
	ExplanationReasonQuotaExhausted = "quota_exhausted"
	ExplanationReasonNotConfigured  = "not_configured"
	ExplanationReasonTimeout        = "llm_timeout"
	ExplanationReasonError          = "llm_error"
	ExplanationReasonInvalidJSON    = "llm_invalid_json"
	ExplanationReasonBridgeRejected = "bridge_text_rejected"
	ExplanationReasonInputTooLarge  = "input_too_large"
)

// 降级事件 kind(analysis_events.kind;周检降级率按事件计数)。
const (
	EventLLMTimeout      = "llm_timeout"
	EventLLMInvalidJSON  = "llm_invalid_json"
	EventBridgeRejected  = "bridge_text_rejected"
	EventQuotaExhausted  = "quota_exhausted"
	EventTrustGateFailed = "trust_gate_failed"
	EventEntryMiss       = "entry_miss"
	EventLLMError        = "llm_error"
	EventInputTooLarge   = "input_too_large"
)

// ErrLLMTimeout LLM 客户端把单次超时映射为该哨兵;编排层据此记录 timeout 事件。
var ErrLLMTimeout = errors.New("llm request timed out")

// LLMRequest 一次补全请求;MaxTokens 对应单次 output 上限。
type LLMRequest struct {
	System    string
	User      string
	MaxTokens int
}

// LLMResponse 模型补全结果;Truncated = finish_reason=length。
type LLMResponse struct {
	Content   string
	Truncated bool
}

// LLMClient 云端模型端口(实现见 internal/llm;测试注入 fake,不打真实模型)。
type LLMClient interface {
	Complete(ctx context.Context, req LLMRequest) (LLMResponse, error)
}

// EntryMatcher 知识条目匹配端口;生产实现为 KnowledgeMatcher,测试可注入 fake / NoEntries。
type EntryMatcher interface {
	Match(req Request, facts Facts) []KnowledgeEntry
}

// NoEntries 空匹配器:不调用云端、不消耗配额。
type NoEntries struct{}

// Match 永远返回空;解释层省略,前两层照常输出。
func (NoEntries) Match(Request, Facts) []KnowledgeEntry { return nil }

// Explainer 解释层编排;依赖全部注入,失败只返回原因,不向上抛错。
type Explainer struct {
	Client           LLMClient
	Matcher          EntryMatcher
	Quota            QuotaStore
	Events           ObservabilityStore
	DailyLimit       int
	InputTokenLimit  int
	OutputTokenLimit int
	// RecordEntryMiss 08 接入条目后置 true;07 空匹配器不把每次分析都记成条目未命中。
	RecordEntryMiss bool
}

// Match 暴露命中条目的只读查询,供保存快照前记录 entry_versions;匹配是纯函数。
func (e *Explainer) Match(req Request, facts Facts) []KnowledgeEntry {
	if e == nil || e.Matcher == nil {
		return nil
	}
	return e.Matcher.Match(req, facts)
}

// Explain 尝试生成解释层;返回 nil + 原因表示省略解释层。
func (e *Explainer) Explain(ctx context.Context, analysisID int64, req Request, facts Facts) (*Explanation, string) {
	if e == nil || e.Client == nil || e.Matcher == nil {
		return nil, ExplanationReasonNotConfigured
	}
	entries := e.Match(req, facts)
	if len(entries) == 0 {
		if e.RecordEntryMiss {
			e.record(ctx, analysisID, EventEntryMiss, "no knowledge entry matched")
		}
		return nil, ExplanationReasonNoEntries
	}
	pack, mapping := BuildEvidencePack(req, facts, entries)
	pack, err := TrimEvidencePack(pack, e.InputTokenLimit)
	if err != nil {
		e.record(ctx, analysisID, EventInputTooLarge, err.Error())
		return nil, ExplanationReasonInputTooLarge
	}
	mapping = filterEvidenceMapping(pack, mapping)
	if e.Quota != nil {
		_, allowed, err := e.Quota.ConsumeLLMQuota(ctx, analysisID, e.DailyLimit)
		if err != nil {
			e.record(ctx, analysisID, EventLLMError, "consume quota: "+err.Error())
			return nil, ExplanationReasonError
		}
		if !allowed {
			e.record(ctx, analysisID, EventQuotaExhausted, "daily explanation quota exhausted")
			return nil, ExplanationReasonQuotaExhausted
		}
	}
	user, err := json.Marshal(pack)
	if err != nil {
		e.record(ctx, analysisID, EventLLMError, "marshal evidence: "+err.Error())
		return nil, ExplanationReasonError
	}

	var lastReason string
	for attempt := 0; attempt < llmMaxAttempts; attempt++ {
		response, err := e.Client.Complete(ctx, LLMRequest{
			System:    explanationSystemPrompt,
			User:      string(user),
			MaxTokens: e.OutputTokenLimit,
		})
		if err != nil {
			lastReason = ExplanationReasonError
			if errors.Is(err, ErrLLMTimeout) || errors.Is(err, context.DeadlineExceeded) {
				lastReason = ExplanationReasonTimeout
			}
			continue
		}
		sections, rejected, reason := validateExplanation(response.Content, entries, mapping)
		if reason != "" {
			lastReason = reason
			continue
		}
		if rejected > 0 {
			e.record(ctx, analysisID, EventBridgeRejected, fmt.Sprintf("%d section(s) rejected", rejected))
		}
		return &Explanation{Sections: sections, Truncated: response.Truncated}, ""
	}

	switch lastReason {
	case ExplanationReasonTimeout:
		e.record(ctx, analysisID, EventLLMTimeout, "llm timed out after retry")
	case ExplanationReasonInvalidJSON:
		e.record(ctx, analysisID, EventLLMInvalidJSON, "invalid json after retry")
	case ExplanationReasonBridgeRejected:
		e.record(ctx, analysisID, EventBridgeRejected, "bridge_text rejected after retry")
	default:
		lastReason = ExplanationReasonError
		e.record(ctx, analysisID, EventLLMError, "llm call failed after retry")
	}
	return nil, lastReason
}

func (e *Explainer) record(ctx context.Context, analysisID int64, kind, detail string) {
	if e.Events == nil {
		return
	}
	_ = e.Events.RecordAnalysisEvent(ctx, analysisID, kind, detail)
}

const (
	llmMaxAttempts     = 2
	maxBridgeTextRunes = 200
)

var bannedInferenceWords = []string{"可能", "提示", "考虑", "建议", "应该"}

type llmOutput struct {
	Sections []llmSection `json:"sections"`
}

type llmSection struct {
	EntryID     string   `json:"entry_id"`
	EvidenceIDs []string `json:"evidence_ids"`
	BridgeText  string   `json:"bridge_text"`
}

func validateExplanation(content string, entries []KnowledgeEntry, mapping EvidenceMapping) ([]ExplanationSection, int, string) {
	raw := stripCodeFence(content)
	var output llmOutput
	if err := json.Unmarshal([]byte(raw), &output); err != nil {
		return nil, 0, ExplanationReasonInvalidJSON
	}
	if len(output.Sections) == 0 {
		return nil, 0, ExplanationReasonBridgeRejected
	}
	entryByID := make(map[string]KnowledgeEntry, len(entries))
	for _, entry := range entries {
		entryByID[entry.ID] = entry
	}
	sections := make([]ExplanationSection, 0, len(output.Sections))
	rejected := 0
	for _, section := range output.Sections {
		validated, ok := validateSection(section, entryByID, mapping)
		if !ok {
			rejected++
			continue
		}
		sections = append(sections, validated)
	}
	if len(sections) == 0 {
		return nil, rejected, ExplanationReasonBridgeRejected
	}
	return sections, rejected, ""
}

func validateSection(section llmSection, entryByID map[string]KnowledgeEntry, mapping EvidenceMapping) (ExplanationSection, bool) {
	entry, ok := entryByID[section.EntryID]
	if !ok || entry.Review.Decision != ReviewDecisionAccepted {
		return ExplanationSection{}, false
	}
	bridgeText := strings.TrimSpace(section.BridgeText)
	if bridgeText == "" || utf8.RuneCountInString(bridgeText) > maxBridgeTextRunes {
		return ExplanationSection{}, false
	}
	if containsBannedInference(bridgeText) {
		return ExplanationSection{}, false
	}
	realIDs := make([]string, 0, len(section.EvidenceIDs))
	evidencePoints := make([]EvidencePoint, 0, len(section.EvidenceIDs))
	for _, id := range section.EvidenceIDs {
		point, ok := mapping[id]
		if !ok {
			return ExplanationSection{}, false
		}
		evidencePoints = append(evidencePoints, point)
		if point.EvidenceID != "" {
			realIDs = append(realIDs, point.EvidenceID)
		}
	}
	if !numbersBelongToEvidence(bridgeText, evidencePoints) {
		return ExplanationSection{}, false
	}
	return ExplanationSection{
		EntryID:         section.EntryID,
		EvidenceIDs:     realIDs,
		BridgeText:      bridgeText,
		Explanation:     entry.Explanation,
		CheckDirections: nonEmptyStrings(entry.CheckDirections),
		GeneralAdvice:   nonEmptyStrings(entry.GeneralAdvice),
		TrackItems:      nonEmptyStrings(entry.TrackItems),
		Sources:         entry.Sources,
	}, true
}

func stripCodeFence(content string) string {
	trimmed := strings.TrimSpace(content)
	if !strings.HasPrefix(trimmed, "```") {
		return trimmed
	}
	trimmed = strings.TrimPrefix(trimmed, "```")
	trimmed = strings.TrimPrefix(trimmed, "json")
	trimmed = strings.TrimSpace(trimmed)
	trimmed = strings.TrimSuffix(trimmed, "```")
	return strings.TrimSpace(trimmed)
}

func containsBannedInference(text string) bool {
	for _, word := range bannedInferenceWords {
		if strings.Contains(text, word) {
			return true
		}
	}
	return false
}

var numberPattern = regexp.MustCompile(`\d+(?:\.\d+)?`)

func numbersBelongToEvidence(text string, points []EvidencePoint) bool {
	numbers := numberPattern.FindAllString(text, -1)
	if len(numbers) == 0 {
		return true
	}
	if len(points) == 0 {
		return false
	}
	allowed := make(map[float64]bool)
	for _, point := range points {
		for _, source := range []string{valueOrEmpty(point.Value), valueOrEmpty(point.RefRange)} {
			for _, token := range numberPattern.FindAllString(source, -1) {
				if value, err := strconv.ParseFloat(token, 64); err == nil {
					allowed[value] = true
				}
			}
		}
	}
	for _, token := range numbers {
		value, err := strconv.ParseFloat(token, 64)
		if err != nil || !allowed[value] {
			return false
		}
	}
	return true
}

// ExplanationSources 解释层引用来源去重(与框架层来源合并后写入响应)。
func ExplanationSources(explanation *Explanation) []Source {
	if explanation == nil {
		return nil
	}
	return mergeSourcesFrom(explanation.Sections, func(section ExplanationSection) []Source { return section.Sources })
}

func mergeSourcesFrom[T any](items []T, sources func(T) []Source) []Source {
	groups := make([][]Source, 0, len(items))
	for _, item := range items {
		groups = append(groups, sources(item))
	}
	return MergeSources(groups...)
}

func containsSource(sources []Source, source Source) bool {
	for _, existing := range sources {
		if sameSource(existing, source) {
			return true
		}
	}
	return false
}

// MergeSources 合并框架层 / 解释层来源,保持首次出现顺序并去重。
func MergeSources(groups ...[]Source) []Source {
	out := []Source{}
	for _, group := range groups {
		for _, source := range group {
			if !containsSource(out, source) {
				out = append(out, source)
			}
		}
	}
	return out
}
