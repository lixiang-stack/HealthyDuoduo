// 云端 LLM 客户端(llm-analysis 票 07):OpenAI 兼容的 chat/completions 协议。
// 只负责协议、超时与响应形状;重试、配额、脱敏与校验在 internal/analysis。
package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"healthyduoduo/internal/analysis"
)

// Config 客户端配置;BaseURL 可带 /v1,最终请求路径为 /chat/completions。
type Config struct {
	BaseURL         string
	APIKey          string
	Model           string
	Timeout         time.Duration
	MaxOutputTokens int
}

// Client OpenAI 兼容客户端。
type Client struct {
	baseURL         string
	apiKey          string
	model           string
	maxOutputTokens int
	http            *http.Client
}

// New 创建客户端;Timeout <= 0 时使用 30 秒。
func New(cfg Config) *Client {
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	return &Client{
		baseURL:         strings.TrimRight(cfg.BaseURL, "/"),
		apiKey:          cfg.APIKey,
		model:           cfg.Model,
		maxOutputTokens: cfg.MaxOutputTokens,
		http:            &http.Client{Timeout: timeout},
	}
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type responseFormat struct {
	Type string `json:"type"`
}

type chatRequest struct {
	Model          string          `json:"model"`
	Messages       []chatMessage   `json:"messages"`
	ResponseFormat *responseFormat `json:"response_format,omitempty"`
	MaxTokens      int             `json:"max_tokens,omitempty"`
	Temperature    float64         `json:"temperature"`
}

type chatResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
}

// Complete 发送一次补全请求;HTTP 非 2xx / 超时 / 空 choices 均返回错误。
func (c *Client) Complete(ctx context.Context, req analysis.LLMRequest) (analysis.LLMResponse, error) {
	maxTokens := req.MaxTokens
	if maxTokens == 0 {
		maxTokens = c.maxOutputTokens
	}
	payload := chatRequest{
		Model: c.model,
		Messages: []chatMessage{
			{Role: "system", Content: req.System},
			{Role: "user", Content: req.User},
		},
		ResponseFormat: &responseFormat{Type: "json_object"},
		MaxTokens:      maxTokens,
		Temperature:    0,
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return analysis.LLMResponse{}, fmt.Errorf("marshal llm request: %w", err)
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost,
		c.baseURL+"/chat/completions", bytes.NewReader(raw))
	if err != nil {
		return analysis.LLMResponse{}, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	response, err := c.http.Do(httpReq)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) || os.IsTimeout(err) {
			return analysis.LLMResponse{}, analysis.ErrLLMTimeout
		}
		return analysis.LLMResponse{}, err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(response.Body, 512))
		return analysis.LLMResponse{}, fmt.Errorf("llm http %d: %s", response.StatusCode, strings.TrimSpace(string(body)))
	}
	var decoded chatResponse
	if err := json.NewDecoder(response.Body).Decode(&decoded); err != nil {
		return analysis.LLMResponse{}, fmt.Errorf("decode llm response: %w", err)
	}
	if len(decoded.Choices) == 0 {
		return analysis.LLMResponse{}, errors.New("llm response has no choices")
	}
	choice := decoded.Choices[0]
	return analysis.LLMResponse{
		Content:   choice.Message.Content,
		Truncated: choice.FinishReason == "length",
	}, nil
}
