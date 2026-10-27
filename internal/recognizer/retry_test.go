package recognizer

// 重试分支单测(内部包:直接缩短 retryInterval,避免真实退避拖慢用例)。
// maxAttempts=2 ⇒ 5xx 恰好重试一次;4xx 判不可重试,只发一次。

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"healthyduoduo/internal/pipeline"
)

// /report 端点返回 ocr_result包裹的 bundle;最小合法报文(恢复分支断言 Status)。
const successReportJSON = `{"ocr_result":{"engine":"onnxruntime"},"report":{"report_type":"血常规","report_date":null,"status":"success","items":[]}}`

func useFastRetry(t *testing.T) {
	t.Helper()
	old := retryInterval
	retryInterval = time.Millisecond
	t.Cleanup(func() { retryInterval = old })
}

func TestReport_RetriesOn5xxThenSucceeds(t *testing.T) {
	useFastRetry(t)
	var mu sync.Mutex
	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		mu.Lock()
		calls++
		first := calls == 1
		mu.Unlock()
		if first {
			http.Error(w, "transient", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(successReportJSON))
	}))
	defer srv.Close()

	c := New(srv.URL)
	_, _, report, err := c.Report(context.Background(), []byte("x"), "a.jpg", "")
	if err != nil {
		t.Fatalf("Report after retry: %v", err)
	}
	mu.Lock()
	defer mu.Unlock()
	if calls != 2 {
		t.Errorf("server calls = %d, want 2 (首试 500,重试一次)", calls)
	}
	if report.Status != "success" {
		t.Errorf("report status = %q, want success", report.Status)
	}
}

func TestReport_NoRetryOn4xx(t *testing.T) {
	useFastRetry(t)
	var mu sync.Mutex
	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		mu.Lock()
		calls++
		mu.Unlock()
		http.Error(w, "bad request", http.StatusBadRequest)
	}))
	defer srv.Close()

	c := New(srv.URL)
	if _, _, _, err := c.Report(context.Background(), []byte("x"), "a.jpg", ""); err == nil {
		t.Fatal("expected error on 400")
	}
	mu.Lock()
	defer mu.Unlock()
	if calls != 1 {
		t.Errorf("server calls = %d, want 1 (4xx 不重试)", calls)
	}
}

// 识别服务不可达 / 端点缺失 / 5xx = 整体不可用:分类命令据此停止并提示部署顺序;
// 422 等单份数据错误仍逐份继续。
func TestClassify_TransportErrorMarksUnavailable(t *testing.T) {
	useFastRetry(t)
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	base := srv.URL
	srv.Close()

	_, _, err := New(base).Classify(context.Background(), "生化", nil)
	if !errors.Is(err, pipeline.ErrClassifierUnavailable) {
		t.Fatalf("err = %v, want ErrClassifierUnavailable", err)
	}
}

func TestClassify_MissingEndpointMarksUnavailable(t *testing.T) {
	useFastRetry(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.NotFound(w, nil)
	}))
	defer srv.Close()

	_, _, err := New(srv.URL).Classify(context.Background(), "生化", nil)
	if !errors.Is(err, pipeline.ErrClassifierUnavailable) {
		t.Fatalf("err = %v, want ErrClassifierUnavailable for older recognizer without /classify", err)
	}
}

func TestClassify_ServerErrorMarksUnavailable(t *testing.T) {
	useFastRetry(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	defer srv.Close()

	_, _, err := New(srv.URL).Classify(context.Background(), "生化", nil)
	if !errors.Is(err, pipeline.ErrClassifierUnavailable) {
		t.Fatalf("err = %v, want ErrClassifierUnavailable for 5xx", err)
	}
}

func TestClassify_ContextCancellationIsPerReportFailure(t *testing.T) {
	useFastRetry(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "unexpected call", http.StatusInternalServerError)
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, _, err := New(srv.URL).Classify(ctx, "生化", nil)
	if err == nil {
		t.Fatal("expected error on canceled context")
	}
	if errors.Is(err, pipeline.ErrClassifierUnavailable) {
		t.Errorf("context cancellation classified as unavailable, want per-report failure: %v", err)
	}
}

func TestClassify_HTTPErrorIsPerReportFailure(t *testing.T) {
	useFastRetry(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "parent-child violation", http.StatusUnprocessableEntity)
	}))
	defer srv.Close()

	_, _, err := New(srv.URL).Classify(context.Background(), "生化", nil)
	if err == nil {
		t.Fatal("expected error on 422")
	}
	if errors.Is(err, pipeline.ErrClassifierUnavailable) {
		t.Errorf("422 classified as unavailable, want per-report failure: %v", err)
	}
}
