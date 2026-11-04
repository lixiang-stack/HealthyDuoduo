package analysis

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func acceptedEntry(id string) KnowledgeEntry {
	return KnowledgeEntry{ID: id, Review: KnowledgeReview{Decision: ReviewDecisionAccepted}}
}

type fakeLLM struct {
	responses []LLMResponse
	errs      []error
	calls     []LLMRequest
}

func (f *fakeLLM) Complete(ctx context.Context, req LLMRequest) (LLMResponse, error) {
	f.calls = append(f.calls, req)
	index := len(f.calls) - 1
	if index < len(f.errs) && f.errs[index] != nil {
		return LLMResponse{}, f.errs[index]
	}
	if index < len(f.responses) {
		return f.responses[index], nil
	}
	return LLMResponse{}, errors.New("no fake response")
}

type fakeMatcher struct {
	entries []KnowledgeEntry
}

func (f fakeMatcher) Match(req Request, facts Facts) []KnowledgeEntry {
	return f.entries
}

type fakeQuota struct {
	used  int
	limit int
	calls int
}

func (f *fakeQuota) ConsumeLLMQuota(ctx context.Context, analysisID int64, limit int) (int, bool, error) {
	f.calls++
	if f.used >= f.limit {
		return f.used, false, nil
	}
	f.used++
	return f.used, true, nil
}

func (f *fakeQuota) LLMUsedToday(ctx context.Context) (int, error) {
	return f.used, nil
}

type fakeEvents struct {
	kinds []string
}

func (f *fakeEvents) RecordAnalysisEvent(ctx context.Context, analysisID int64, kind, detail string) error {
	f.kinds = append(f.kinds, kind)
	return nil
}

func (f *fakeEvents) RecordFeedback(ctx context.Context, feedback Feedback) (*Feedback, error) {
	return &feedback, nil
}

func (f *fakeEvents) AnalysisMetrics(ctx context.Context, from, to time.Time) (Metrics, error) {
	return Metrics{}, nil
}

func explanationTestFacts() Facts {
	point := frameworkPoint(1, "丙氨酸转氨酶测定", "83.5", "U/L", "7-40", "high")
	point.EvidenceID = "ev-r1-1"
	return frameworkFacts(point)
}

func explanationTestExplainer(client LLMClient, matcher EntryMatcher) (*Explainer, *fakeQuota, *fakeEvents) {
	quota := &fakeQuota{limit: 20}
	events := &fakeEvents{}
	return &Explainer{
		Client:           client,
		Matcher:          matcher,
		Quota:            quota,
		Events:           events,
		DailyLimit:       20,
		InputTokenLimit:  8000,
		OutputTokenLimit: 1500,
	}, quota, events
}

const validExplanationJSON = `{"sections":[{"entry_id":"liver-hepatocellular-mild","evidence_ids":["e1"],"bridge_text":"先看本次转氨酶与碱性磷酸酶的关系。"}]}`

func TestExplainSuccessMapsEphemeralEvidenceIDs(t *testing.T) {
	client := &fakeLLM{responses: []LLMResponse{{Content: validExplanationJSON}}}
	explainer, quota, events := explanationTestExplainer(client, fakeMatcher{entries: []KnowledgeEntry{{
		ID: "liver-hepatocellular-mild", Explanation: "条目原文",
		CheckDirections: []string{"复查肝功能"}, GeneralAdvice: []string{"规律作息"},
		TrackItems: []string{"ALT", "AST"},
		Review:     KnowledgeReview{Decision: ReviewDecisionAccepted},
	}}})
	explanation, reason := explainer.Explain(context.Background(), 7, testRequest(EntryPanel, "liver", 12), explanationTestFacts())
	if reason != "" || explanation == nil || len(explanation.Sections) != 1 {
		t.Fatalf("explanation = %+v, reason = %q", explanation, reason)
	}
	section := explanation.Sections[0]
	if section.EntryID != "liver-hepatocellular-mild" || len(section.EvidenceIDs) != 1 ||
		section.EvidenceIDs[0] != "ev-r1-1" || section.BridgeText == "" {
		t.Fatalf("section = %+v", section)
	}
	if section.Explanation != "条目原文" || len(section.CheckDirections) != 1 ||
		len(section.GeneralAdvice) != 1 || len(section.TrackItems) != 2 {
		t.Fatalf("server-assembled entry text = %+v", section)
	}
	if quota.calls != 1 || len(events.kinds) != 0 {
		t.Fatalf("quota/events = %+v / %v", quota, events.kinds)
	}
	if len(client.calls) != 1 || client.calls[0].MaxTokens != 1500 {
		t.Fatalf("llm calls = %+v", client.calls)
	}
}

func TestExplainRetriesOnceThenSucceeds(t *testing.T) {
	client := &fakeLLM{
		errs:      []error{ErrLLMTimeout, nil},
		responses: []LLMResponse{{}, {Content: validExplanationJSON}},
	}
	explainer, quota, events := explanationTestExplainer(client, fakeMatcher{entries: []KnowledgeEntry{acceptedEntry("liver-hepatocellular-mild")}})
	explanation, reason := explainer.Explain(context.Background(), 7, testRequest(EntryPanel, "liver", 12), explanationTestFacts())
	if reason != "" || explanation == nil {
		t.Fatalf("explanation = %+v, reason = %q", explanation, reason)
	}
	if len(client.calls) != 2 || quota.calls != 1 || len(events.kinds) != 0 {
		t.Fatalf("calls/quota/events = %d/%d/%v", len(client.calls), quota.calls, events.kinds)
	}
}

func TestExplainTimeoutAfterRetry(t *testing.T) {
	client := &fakeLLM{errs: []error{ErrLLMTimeout, ErrLLMTimeout}}
	explainer, quota, events := explanationTestExplainer(client, fakeMatcher{entries: []KnowledgeEntry{acceptedEntry("liver-hepatocellular-mild")}})
	explanation, reason := explainer.Explain(context.Background(), 7, testRequest(EntryPanel, "liver", 12), explanationTestFacts())
	if explanation != nil || reason != ExplanationReasonTimeout {
		t.Fatalf("explanation = %+v, reason = %q", explanation, reason)
	}
	if quota.calls != 1 || len(events.kinds) != 1 || events.kinds[0] != EventLLMTimeout {
		t.Fatalf("quota/events = %d/%v", quota.calls, events.kinds)
	}
}

func TestExplainInvalidJSONAfterRetry(t *testing.T) {
	client := &fakeLLM{responses: []LLMResponse{{Content: "not json"}, {Content: "still not json"}}}
	explainer, _, events := explanationTestExplainer(client, fakeMatcher{entries: []KnowledgeEntry{acceptedEntry("liver-hepatocellular-mild")}})
	explanation, reason := explainer.Explain(context.Background(), 7, testRequest(EntryPanel, "liver", 12), explanationTestFacts())
	if explanation != nil || reason != ExplanationReasonInvalidJSON {
		t.Fatalf("explanation = %+v, reason = %q", explanation, reason)
	}
	if len(events.kinds) != 1 || events.kinds[0] != EventLLMInvalidJSON {
		t.Fatalf("events = %v", events.kinds)
	}
}

func TestExplainQuotaExhausted(t *testing.T) {
	client := &fakeLLM{responses: []LLMResponse{{Content: validExplanationJSON}}}
	explainer, quota, events := explanationTestExplainer(client, fakeMatcher{entries: []KnowledgeEntry{acceptedEntry("liver-hepatocellular-mild")}})
	quota.used = 20
	explanation, reason := explainer.Explain(context.Background(), 7, testRequest(EntryPanel, "liver", 12), explanationTestFacts())
	if explanation != nil || reason != ExplanationReasonQuotaExhausted {
		t.Fatalf("explanation = %+v, reason = %q", explanation, reason)
	}
	if len(client.calls) != 0 || len(events.kinds) != 1 || events.kinds[0] != EventQuotaExhausted {
		t.Fatalf("calls/events = %d/%v", len(client.calls), events.kinds)
	}
}

func TestExplainBridgeTextRejected(t *testing.T) {
	cases := []string{
		`{"sections":[{"entry_id":"liver-hepatocellular-mild","evidence_ids":["e1"],"bridge_text":"可能和本次转氨酶有关。"}]}`,
		`{"sections":[{"entry_id":"liver-hepatocellular-mild","evidence_ids":["e1"],"bridge_text":"本次 ALT 是 3.2 倍。"}]}`,
		`{"sections":[{"entry_id":"missing","evidence_ids":["e1"],"bridge_text":"先看本次数值。"}]}`,
		`{"sections":[{"entry_id":"liver-hepatocellular-mild","evidence_ids":["e9"],"bridge_text":"先看本次数值。"}]}`,
		`{"sections":[{"entry_id":"liver-hepatocellular-mild","evidence_ids":["e1"],"bridge_text":"` + strings.Repeat("长", 201) + `"}]}`,
	}
	for _, content := range cases {
		t.Run(content[:20], func(t *testing.T) {
			client := &fakeLLM{responses: []LLMResponse{{Content: content}, {Content: content}}}
			explainer, _, events := explanationTestExplainer(client, fakeMatcher{entries: []KnowledgeEntry{acceptedEntry("liver-hepatocellular-mild")}})
			explanation, reason := explainer.Explain(context.Background(), 7, testRequest(EntryPanel, "liver", 12), explanationTestFacts())
			if explanation != nil || reason != ExplanationReasonBridgeRejected {
				t.Fatalf("explanation = %+v, reason = %q", explanation, reason)
			}
			if len(events.kinds) != 1 || events.kinds[0] != EventBridgeRejected {
				t.Fatalf("events = %v", events.kinds)
			}
		})
	}
}

func TestExplainNoEntriesDoesNotCallLLM(t *testing.T) {
	client := &fakeLLM{responses: []LLMResponse{{Content: validExplanationJSON}}}
	explainer, quota, events := explanationTestExplainer(client, NoEntries{})
	explanation, reason := explainer.Explain(context.Background(), 7, testRequest(EntryPanel, "liver", 12), explanationTestFacts())
	if explanation != nil || reason != ExplanationReasonNoEntries {
		t.Fatalf("explanation = %+v, reason = %q", explanation, reason)
	}
	if len(client.calls) != 0 || quota.calls != 0 || len(events.kinds) != 0 {
		t.Fatalf("calls/quota/events = %d/%d/%v", len(client.calls), quota.calls, events.kinds)
	}
	explainer.RecordEntryMiss = true
	_, _ = explainer.Explain(context.Background(), 7, testRequest(EntryPanel, "liver", 12), explanationTestFacts())
	if len(events.kinds) != 1 || events.kinds[0] != EventEntryMiss {
		t.Fatalf("entry miss events = %v", events.kinds)
	}
}

func TestExplainStripsCodeFence(t *testing.T) {
	client := &fakeLLM{responses: []LLMResponse{{Content: "```json\n" + validExplanationJSON + "\n```"}}}
	explainer, _, _ := explanationTestExplainer(client, fakeMatcher{entries: []KnowledgeEntry{acceptedEntry("liver-hepatocellular-mild")}})
	explanation, reason := explainer.Explain(context.Background(), 7, testRequest(EntryPanel, "liver", 12), explanationTestFacts())
	if reason != "" || explanation == nil {
		t.Fatalf("explanation = %+v, reason = %q", explanation, reason)
	}
}

func TestValidateSectionRejectsUnreviewedEntry(t *testing.T) {
	section := llmSection{EntryID: "x", EvidenceIDs: []string{"e1"}, BridgeText: "本次结果已整理。"}
	mapping := EvidenceMapping{"e1": {EvidenceID: "ev-1"}}
	if _, ok := validateSection(section, map[string]KnowledgeEntry{"x": {ID: "x"}}, mapping); ok {
		t.Fatal("unreviewed entry must be rejected")
	}
	accepted := KnowledgeEntry{ID: "x", Review: KnowledgeReview{Decision: ReviewDecisionAccepted}}
	if _, ok := validateSection(section, map[string]KnowledgeEntry{"x": accepted}, mapping); !ok {
		t.Fatal("accepted entry should pass")
	}
}

func TestExplanationPromptEmbeddedAndVersioned(t *testing.T) {
	if strings.TrimSpace(explanationSystemPrompt) == "" {
		t.Fatal("embedded explanation prompt is empty")
	}
	for _, want := range []string{`"sections"`, `"evidence_ids"`, `"bridge_text"`} {
		if !strings.Contains(explanationSystemPrompt, want) {
			t.Fatalf("prompt missing %s", want)
		}
	}
	for _, word := range bannedInferenceWords {
		if !strings.Contains(explanationSystemPrompt, word) {
			t.Fatalf("prompt missing banned word %q; keep prompt and validator in sync", word)
		}
	}
	if explanationPromptVersion == "" {
		t.Fatal("explanationPromptVersion is empty")
	}
}

func TestExplainKeepsValidSectionsWhenOneIsRejected(t *testing.T) {
	content := `{"sections":[
		{"entry_id":"liver-hepatocellular-mild","evidence_ids":["e1"],"bridge_text":"先看本次转氨酶与碱性磷酸酶的关系。"},
		{"entry_id":"liver-hepatocellular-mild","evidence_ids":["e1"],"bridge_text":"可能和本次转氨酶有关。"}
	]}`
	client := &fakeLLM{responses: []LLMResponse{{Content: content}}}
	explainer, _, events := explanationTestExplainer(client, fakeMatcher{entries: []KnowledgeEntry{{
		ID:     "liver-hepatocellular-mild",
		Review: KnowledgeReview{Decision: ReviewDecisionAccepted},
	}}})
	explanation, reason := explainer.Explain(context.Background(), 7, testRequest(EntryPanel, "liver", 12), explanationTestFacts())
	if reason != "" || explanation == nil || len(explanation.Sections) != 1 {
		t.Fatalf("explanation = %+v, reason = %q", explanation, reason)
	}
	if len(events.kinds) != 1 || events.kinds[0] != EventBridgeRejected {
		t.Fatalf("events = %v, want bridge_text_rejected", events.kinds)
	}
}
