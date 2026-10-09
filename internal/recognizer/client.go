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

type Client struct {
	base   string
	client *http.Client
}

// New: base 如 http://localhost:8000(超时覆盖模型加载 + 单张 OCR,机内实测远低于 60s)。
func New(base string) *Client {
	return &Client{
		base:   strings.TrimRight(base, "/"),
		client: &http.Client{Timeout: 60 * time.Second},
	}
}

type bundle struct {
	OCRResult contract.OCRResult `json:"ocr_result"`
	Report    contract.Report    `json:"report"`
}

func (c *Client) Report(ctx context.Context, image []byte, filename, date string) (*contract.OCRResult, *contract.Report, error) {
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	if err := imageField(mw, filename, image); err != nil {
		return nil, nil, err
	}
	if date != "" {
		if err := mw.WriteField("date", date); err != nil {
			return nil, nil, err
		}
	}
	if err := mw.Close(); err != nil {
		return nil, nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.base+"/report", bytes.NewReader(body.Bytes()))
	if err != nil {
		return nil, nil, err
	}
	req.Header.Set("Content-Type", mw.FormDataContentType())
	out, err := c.do(req)
	if err != nil {
		return nil, nil, err
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
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.base+"/reparse", bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("识别服务 %s: %w", req.URL, err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 32<<20))
	if err != nil {
		return nil, fmt.Errorf("读响应体: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("识别服务 %d: %s", resp.StatusCode, truncate(body))
	}
	var report contract.Report
	if err := json.Unmarshal(body, &report); err != nil {
		return nil, fmt.Errorf("解析 /reparse 响应: %w (body=%s)", err, truncate(body))
	}
	return &report, nil
}

func imageField(mw *multipart.Writer, filename string, image []byte) error {
	fw, err := mw.CreateFormFile("image", filename)
	if err != nil {
		return err
	}
	_, err = fw.Write(image)
	return err
}

func (c *Client) do(req *http.Request) (*bundle, error) {
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("识别服务 %s: %w", req.URL, err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 32<<20))
	if err != nil {
		return nil, fmt.Errorf("读响应体: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("识别服务 %d: %s", resp.StatusCode, truncate(raw))
	}
	var out bundle
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("解析 /report 响应: %w (body=%s)", err, truncate(raw))
	}
	return &out, nil
}

func truncate(raw []byte) string {
	s := string(raw)
	if len(s) > 200 {
		return s[:200] + "…"
	}
	return s
}

// 编译期接口校验(pipeline.Recognizer 的实现)。
var _ interface {
	Report(context.Context, []byte, string, string) (*contract.OCRResult, *contract.Report, error)
	Reparse(context.Context, *contract.OCRResult, string) (*contract.Report, error)
} = (*Client)(nil)
