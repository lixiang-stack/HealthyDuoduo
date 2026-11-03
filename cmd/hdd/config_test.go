package main

import (
	"testing"
	"time"

	"healthyduoduo/internal/analysis"
	"healthyduoduo/internal/llm"
)

var llmEnvKeys = []string{
	"HDD_LLM_API_KEY",
	"HDD_LLM_BASE_URL",
	"HDD_LLM_MODEL",
	"HDD_LLM_TIMEOUT_SECONDS",
	"HDD_LLM_INPUT_TOKENS",
	"HDD_LLM_OUTPUT_TOKENS",
	"HDD_ANALYSIS_DAILY_LIMIT",
}

func clearLLMEnv(t *testing.T) {
	t.Helper()
	for _, key := range llmEnvKeys {
		t.Setenv(key, "")
	}
}

func TestEnvConfigLLMDefaults(t *testing.T) {
	clearLLMEnv(t)
	c := envConfig()
	if c.llmAPIKey != "" {
		t.Errorf("llmAPIKey = %q, want empty", c.llmAPIKey)
	}
	if c.llmBaseURL != "https://api.deepseek.com" {
		t.Errorf("llmBaseURL = %q", c.llmBaseURL)
	}
	if c.llmModel != "deepseek-chat" {
		t.Errorf("llmModel = %q", c.llmModel)
	}
	if c.llmTimeout != 30*time.Second {
		t.Errorf("llmTimeout = %v, want 30s", c.llmTimeout)
	}
	if c.llmInputTokens != 8000 || c.llmOutputTokens != 1500 {
		t.Errorf("token limits = %d/%d, want 8000/1500", c.llmInputTokens, c.llmOutputTokens)
	}
	if c.analysisLimit != analysisDailyLimitDefault {
		t.Errorf("analysisLimit = %d, want %d", c.analysisLimit, analysisDailyLimitDefault)
	}
	if explainer := newExplainer(c, nil, nil); explainer != nil {
		t.Fatal("explainer created without API key")
	}
}

func TestEnvConfigLLMOverrides(t *testing.T) {
	t.Setenv("HDD_LLM_API_KEY", "sk-test")
	t.Setenv("HDD_LLM_BASE_URL", "https://llm.example")
	t.Setenv("HDD_LLM_MODEL", "deepseek-reasoner")
	t.Setenv("HDD_LLM_TIMEOUT_SECONDS", "45")
	t.Setenv("HDD_LLM_INPUT_TOKENS", "9000")
	t.Setenv("HDD_LLM_OUTPUT_TOKENS", "2000")
	t.Setenv("HDD_ANALYSIS_DAILY_LIMIT", "7")

	c := envConfig()
	if c.llmAPIKey != "sk-test" || c.llmBaseURL != "https://llm.example" ||
		c.llmModel != "deepseek-reasoner" {
		t.Fatalf("llm config = %+v", c)
	}
	if c.llmTimeout != 45*time.Second || c.llmInputTokens != 9000 ||
		c.llmOutputTokens != 2000 || c.analysisLimit != 7 {
		t.Fatalf("llm limits = %+v", c)
	}
	if explainer := newExplainer(c, nil, nil); explainer == nil {
		t.Fatal("explainer not created with API key")
	}
}

func TestFromEnvIntFallsBackOnInvalidValues(t *testing.T) {
	cases := []struct {
		name  string
		value string
		want  int
	}{
		{"empty", "", 30},
		{"not a number", "abc", 30},
		{"zero", "0", 30},
		{"negative", "-1", 30},
		{"valid", "12", 12},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Setenv("HDD_TEST_INT", c.value)
			if got := fromEnvInt("HDD_TEST_INT", 30); got != c.want {
				t.Fatalf("fromEnvInt = %d, want %d", got, c.want)
			}
		})
	}
}

func TestLLMClientConfigMapsFields(t *testing.T) {
	c := config{
		llmAPIKey:       "sk-test",
		llmBaseURL:      "https://llm.example",
		llmModel:        "deepseek-chat",
		llmTimeout:      45 * time.Second,
		llmOutputTokens: 2000,
	}
	got := llmClientConfig(c)
	if got.BaseURL != c.llmBaseURL || got.APIKey != c.llmAPIKey ||
		got.Model != c.llmModel || got.Timeout != c.llmTimeout ||
		got.MaxOutputTokens != c.llmOutputTokens {
		t.Fatalf("llm client config = %+v", got)
	}
}

func TestNewExplainerWiresConfigAndStores(t *testing.T) {
	store := &fakeAnalysisStore{}
	c := config{
		llmAPIKey:       "sk-test",
		llmBaseURL:      "https://llm.example",
		llmModel:        "deepseek-chat",
		llmTimeout:      45 * time.Second,
		llmInputTokens:  9000,
		llmOutputTokens: 2000,
		analysisLimit:   7,
	}
	explainer := newExplainer(c, store, store)
	if explainer == nil {
		t.Fatal("explainer not created with API key")
	}
	if _, ok := explainer.Client.(*llm.Client); !ok {
		t.Fatalf("client type = %T, want *llm.Client", explainer.Client)
	}
	if _, ok := explainer.Matcher.(analysis.NoEntries); !ok {
		t.Fatalf("matcher type = %T, want analysis.NoEntries", explainer.Matcher)
	}
	if explainer.Quota != store || explainer.Events != store {
		t.Fatal("quota/events store not wired")
	}
	if explainer.DailyLimit != 7 || explainer.InputTokenLimit != 9000 ||
		explainer.OutputTokenLimit != 2000 {
		t.Fatalf("explainer limits = %+v", explainer)
	}
}
