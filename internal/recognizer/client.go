// Package recognizer:识别服务(HTTP)客户端,是 pipeline.Recognizer 的适配器。
// 端点契约见实施计划 4.3:multipart /report(image + 可选 date)、JSON /reparse。
package recognizer

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strings"
	"time"

	"healthyduoduo/internal/contract"
)

// 端点路径(实施计划 4.3)。
const (
	reportPath  = "/report"
	reparsePath = "/reparse"
)

// 网络抖动重试:上限(至少一次)与退避间隔。
const (
	maxAttempts   = 2
	retryInterval = 1 * time.Second
)

// 单次请求超时:覆盖模型加载 + 单张 OCR(机内实测远低于 60s)。
const requestTimeout = 60 * time.Second

// 响应体读取上限(OCR 结果 JSON 含全部文本行/坐标,常规 <5MB)。
const maxBodyBytes = 32 << 20

// 错误信息中响应体片段的截断上限(定位问题足够,避免整段输出)。
const maxErrorSnippet = 200

type Client struct {
	reportURL  string
	reparseURL string
	client     *http.Client
}

// New: base 如 http://localhost:8000。
func New(base string) *Client {
	base = trimSlash(base)
	return &Client{
		reportURL:  base + reportPath,
		reparseURL: base + reparsePath,
		client:     &http.Client{Timeout: requestTimeout},
	}
}

type reportResponse struct {
	OCRResult contract.OCRResult `json:"ocr_result"`
	Report    contract.Report    `json:"report"`
}

func (c *Client) Report(ctx context.Context, image []byte, filename, date, preprocess string) (*contract.OCRResult, *contract.Report, error) {
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	fw, err := mw.CreateFormFile("image", filename)
	if err != nil {
		return nil, nil, err
	}
	if _, err := fw.Write(image); err != nil {
		return nil, nil, err
	}
	if date != "" {
		if err := mw.WriteField("date", date); err != nil {
			return nil, nil, err
		}
	}
	if preprocess != "" {
		if err := mw.WriteField("preprocess", preprocess); err != nil {
			return nil, nil, err
		}
	}
	if err := mw.Close(); err != nil {
		return nil, nil, err
	}
	raw, err := c.post(ctx, c.reportURL, mw.FormDataContentType(), body.Bytes())
	if err != nil {
		return nil, nil, err
	}
	var out reportResponse
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, nil, fmt.Errorf("parse /report response: %w (body=%s)", err, truncate(raw))
	}
	return &out.OCRResult, &out.Report, nil
}

func (c *Client) Reparse(ctx context.Context, ocr *contract.OCRResult, date string) (*contract.Report, error) {
	payload := struct {
		OCRResult *contract.OCRResult `json:"ocr_result"`
		Date      *string             `json:"date,omitempty"`
	}{OCRResult: ocr}
	if date != "" {
		payload.Date = &date
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	body, err := c.post(ctx, c.reparseURL, "application/json", raw)
	if err != nil {
		return nil, err
	}
	var report contract.Report
	if err := json.Unmarshal(body, &report); err != nil {
		return nil, fmt.Errorf("parse /reparse response: %w (body=%s)", err, truncate(body))
	}
	return &report, nil
}

// post 发送一次 POST 并读取 200 响应体;网络抖动/5xx 视为可重试(至少重试一次)。
// bytes.NewReader 让 net/http 自动填充 req.GetBody,重试可安全重建请求体。
func (c *Client) post(ctx context.Context, url, contentType string, body []byte) ([]byte, error) {
	var lastErr error
	for attempt := 0; attempt < maxAttempts; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return nil, lastErr
			case <-time.After(retryInterval):
			}
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Content-Type", contentType)

		var resp byteResponse
		resp, err = c.roundTrip(req)
		if err == nil {
			return resp.body, nil
		}
		lastErr = err
		if !resp.retryable {
			return nil, err
		}
	}
	return nil, lastErr
}

// roundTrip 单次 HTTP 请求/读取;retryable 标记是否值得再试(网络错误或 5xx)。
// 资源释放按 go 规范:defer 关闭响应体。
type byteResponse struct {
	body      []byte
	retryable bool
}

func (c *Client) roundTrip(req *http.Request) (byteResponse, error) {
	resp, err := c.client.Do(req)
	if err != nil {
		return byteResponse{}, fmt.Errorf("recognizer %s: %w", req.URL, err)
	}
	defer resp.Body.Close()
	raw, err := readAll(resp.Body)
	if err != nil {
		return byteResponse{}, fmt.Errorf("read response body: %w", err)
	}
	if resp.StatusCode >= http.StatusInternalServerError {
		return byteResponse{retryable: true}, fmt.Errorf("recognizer %d: %s", resp.StatusCode, truncate(raw))
	}
	if resp.StatusCode != http.StatusOK {
		return byteResponse{}, fmt.Errorf("recognizer %d: %s", resp.StatusCode, truncate(raw))
	}
	return byteResponse{body: raw}, nil
}

func readAll(r io.Reader) ([]byte, error) {
	return io.ReadAll(io.LimitReader(r, maxBodyBytes))
}

func trimSlash(s string) string { return strings.TrimRight(s, "/") }

func truncate(raw []byte) string {
	s := string(raw)
	if len(s) > maxErrorSnippet {
		return s[:maxErrorSnippet] + "…"
	}
	return s
}

// 编译期接口校验(pipeline.Recognizer 的实现)。
var _ interface {
	Report(context.Context, []byte, string, string, string) (*contract.OCRResult, *contract.Report, error)
	Reparse(context.Context, *contract.OCRResult, string) (*contract.Report, error)
} = (*Client)(nil)
