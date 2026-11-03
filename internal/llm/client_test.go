package llm

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"healthyduoduo/internal/analysis"
)

func TestCompleteSendsProtocolAndReadsTruncation(t *testing.T) {
	var got chatRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" || r.Header.Get("Authorization") != "Bearer test-key" {
			t.Errorf("request = %s %s auth=%q", r.Method, r.URL.Path, r.Header.Get("Authorization"))
		}
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Errorf("decode request: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"{\"sections\":[]}"},"finish_reason":"length"}]}`))
	}))
	defer srv.Close()

	client := New(Config{BaseURL: srv.URL, APIKey: "test-key", Model: "deepseek-chat", Timeout: time.Second})
	response, err := client.Complete(context.Background(), analysis.LLMRequest{
		System: "system", User: "user", MaxTokens: 1500,
	})
	if err != nil {
		t.Fatal(err)
	}
	if response.Content != `{"sections":[]}` || !response.Truncated {
		t.Fatalf("response = %+v", response)
	}
	if got.Model != "deepseek-chat" || got.MaxTokens != 1500 || got.Temperature != 0 ||
		len(got.Messages) != 2 || got.Messages[0].Role != "system" ||
		got.ResponseFormat == nil || got.ResponseFormat.Type != "json_object" {
		t.Fatalf("request payload = %+v", got)
	}
}

func TestCompleteHTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "upstream unavailable", http.StatusBadGateway)
	}))
	defer srv.Close()

	client := New(Config{BaseURL: srv.URL, APIKey: "test-key", Model: "deepseek-chat", Timeout: time.Second})
	_, err := client.Complete(context.Background(), analysis.LLMRequest{System: "s", User: "u"})
	if err == nil {
		t.Fatal("HTTP 502 returned nil error")
	}
}

func TestCompleteTimeoutMapsToSentinel(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"{}"}}]}`))
	}))
	defer srv.Close()

	client := New(Config{BaseURL: srv.URL, APIKey: "test-key", Model: "deepseek-chat", Timeout: 10 * time.Millisecond})
	_, err := client.Complete(context.Background(), analysis.LLMRequest{System: "s", User: "u"})
	if !errors.Is(err, analysis.ErrLLMTimeout) {
		t.Fatalf("timeout error = %v, want ErrLLMTimeout", err)
	}
}
