// Package recognizer:识别服务(HTTP)客户端,是 pipeline.Recognizer 的适配器。
// 端点契约见实施计划 4.3:multipart /report(image + 可选 date)、JSON /reparse。
package recognizer

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"strings"
	"time"

	"healthyduoduo/internal/contract"
	"healthyduoduo/internal/pipeline"
)

// 端点路径(实施计划 4.3;classify 为 panel-browse 票 05 的存量回填 / 未来重算)。
const (
	reportPath   = "/report"
	reparsePath  = "/reparse"
	classifyPath = "/classify"
)

// 网络抖动重试:上限(至少一次)与退避间隔;retryInterval 为可变值,供测试缩短退避。
const maxAttempts = 2

var retryInterval = 1 * time.Second

// 单次请求超时:覆盖模型加载 + 单张 OCR(机内实测远低于 60s)。
const requestTimeout = 60 * time.Second

// 响应体读取上限:OCR 结果 JSON 含全部文本行/坐标;整页横拍转正时还带
// base64 图像(上限 25MiB 上传 → 约 34MiB base64),留出余量。
const maxBodyBytes = 64 << 20

// 错误信息中响应体片段的截断上限(定位问题足够,避免整段输出)。
const maxErrorSnippet = 200

type Client struct {
	reportURL   string
	reparseURL  string
	classifyURL string
	client      *http.Client
}

// New: base 如 http://localhost:8000。
func New(base string) *Client {
	base = trimSlash(base)
	return &Client{
		reportURL:   base + reportPath,
		reparseURL:  base + reparsePath,
		classifyURL: base + classifyPath,
		client:      &http.Client{Timeout: requestTimeout},
	}
}

type reportResponse struct {
	OCRResult       contract.OCRResult `json:"ocr_result"`
	Report          contract.Report    `json:"report"`
	NormalizedImage string             `json:"normalized_image"`
}

func (c *Client) Report(ctx context.Context, image []byte, filename, date string) ([]byte, *contract.OCRResult, *contract.Report, error) {
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	fw, err := mw.CreateFormFile("image", filename)
	if err != nil {
		return nil, nil, nil, err
	}
	if _, err := fw.Write(image); err != nil {
		return nil, nil, nil, err
	}
	if date != "" {
		if err := mw.WriteField("date", date); err != nil {
			return nil, nil, nil, err
		}
	}
	if err := mw.Close(); err != nil {
		return nil, nil, nil, err
	}
	raw, err := c.post(ctx, c.reportURL, mw.FormDataContentType(), body.Bytes())
	if err != nil {
		return nil, nil, nil, err
	}
	var out reportResponse
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, nil, nil, fmt.Errorf("parse /report response: %w (body=%s)", err, truncate(raw))
	}
	var normalized []byte
	if out.NormalizedImage != "" {
		normalized, err = base64.StdEncoding.DecodeString(out.NormalizedImage)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("decode normalized_image: %w", err)
		}
	}
	return normalized, &out.OCRResult, &out.Report, nil
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

// classifyRequest /classify 请求体(识别端 ClassifyRequest 的镜像)。
type classifyRequest struct {
	ReportType string         `json:"report_type"`
	Items      []classifyItem `json:"items"`
}

type classifyItem struct {
	Name string  `json:"name"`
	Unit *string `json:"unit,omitempty"`
}

// Classify 存量回填用的项目组重算(POST /classify):输入 report_type + 当前指标名 / 单位,
// 返回报告级 panels 与等长同序的 item_panels。与 postprocess 共用识别端解析器。
func (c *Client) Classify(ctx context.Context, reportType string, items []pipeline.ClassifyItem) ([]string, []string, error) {
	payload := classifyRequest{ReportType: reportType, Items: make([]classifyItem, 0, len(items))}
	for _, item := range items {
		payload.Items = append(payload.Items, classifyItem{Name: item.Name, Unit: item.Unit})
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return nil, nil, err
	}
	body, err := c.post(ctx, c.classifyURL, "application/json", raw)
	if err != nil {
		var urlErr *url.Error
		var statusErr *statusError
		switch {
		case errors.As(err, &urlErr):
			if urlErr.Timeout() || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				return nil, nil, err // 慢响应 / 取消是单份失败,不误报成服务未部署
			}
			return nil, nil, fmt.Errorf("%w: %w", pipeline.ErrClassifierUnavailable, err)
		case errors.As(err, &statusErr) && (statusErr.status >= http.StatusInternalServerError ||
			statusErr.status == http.StatusNotFound || statusErr.status == http.StatusMethodNotAllowed):
			return nil, nil, fmt.Errorf("%w: %w", pipeline.ErrClassifierUnavailable, err)
		}
		return nil, nil, err
	}
	var out struct {
		Panels     []string `json:"panels"`
		ItemPanels []string `json:"item_panels"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, nil, fmt.Errorf("parse /classify response: %w (body=%s)", err, truncate(body))
	}
	return out.Panels, out.ItemPanels, nil
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

// statusError 识别服务的非 200 响应;保留状态码供调用方区分「端点缺失 / 服务故障」
// (整体不可用)与单份数据错误(如 /classify 422)。
type statusError struct {
	status int
	body   string
}

func (e *statusError) Error() string {
	return fmt.Sprintf("recognizer %d: %s", e.status, e.body)
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
		return byteResponse{retryable: true}, &statusError{status: resp.StatusCode, body: truncate(raw)}
	}
	if resp.StatusCode != http.StatusOK {
		return byteResponse{}, &statusError{status: resp.StatusCode, body: truncate(raw)}
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

// 编译期接口校验(pipeline.Recognizer / pipeline.PanelClassifier 的实现)。
var _ pipeline.Recognizer = (*Client)(nil)
var _ pipeline.PanelClassifier = (*Client)(nil)
