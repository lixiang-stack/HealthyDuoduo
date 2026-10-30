package main

// cmd/hdd serve 的 HTTP 边界集成测试(/api/v1 是本期唯一新 seam,viz-and-client spec
// Testing Decisions):httptest 起 handler 栈,依赖经 deps 注入(fake 注入同一模式,
// 复用 main_test.go 的小份本地 fake)。本期仅 health;后续端点在此桩上补充用例。

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestServeHealth GET /api/v1/health → 200 + {ok, version}(配对连通性校验契约)。
func TestServeHealth(t *testing.T) {
	d := &deps{store: fakeStoreCLI{}, rec: fakeRecognizerCLI{}, obj: fakeObjectsCLI{}}
	srv := httptest.NewServer(newServeHandler(d))
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/v1/health")
	if err != nil {
		t.Fatalf("GET /api/v1/health: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Fatalf("content-type = %q, want application/json", ct)
	}
	var got healthResponse
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if !got.OK {
		t.Errorf("ok = false, want true")
	}
	if got.Version != version {
		t.Errorf("version = %q, want %q", got.Version, version)
	}
}

// TestServeHealthPrefixFrozen API 前缀 /api/v1 冻结:裸 /health 不得响应。
func TestServeHealthPrefixFrozen(t *testing.T) {
	d := &deps{store: fakeStoreCLI{}, rec: fakeRecognizerCLI{}, obj: fakeObjectsCLI{}}
	srv := httptest.NewServer(newServeHandler(d))
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/health")
	if err != nil {
		t.Fatalf("GET /health: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want 404 (prefix /api/v1 is contractual)", resp.StatusCode)
	}
}

// TestRunServeRejectsUnknownFlag serve 参数错误走 usage(exit 2),不进监听。
func TestRunServeRejectsUnknownFlag(t *testing.T) {
	var out, errOut strings.Builder
	d := &deps{store: fakeStoreCLI{}, rec: fakeRecognizerCLI{}, obj: fakeObjectsCLI{}}
	if code := run(&out, &errOut, d, []string{"serve", "--bogus"}); code != exitUsage {
		t.Fatalf("exit = %d, want %d", code, exitUsage)
	}
	if !strings.Contains(errOut.String(), "serve:") {
		t.Errorf("stderr = %q, want serve usage error", errOut.String())
	}
}

// TestRunServeRejectsPositionalArgs serve 不收位置参数。
func TestRunServeRejectsPositionalArgs(t *testing.T) {
	var out, errOut strings.Builder
	d := &deps{store: fakeStoreCLI{}, rec: fakeRecognizerCLI{}, obj: fakeObjectsCLI{}}
	if code := run(&out, &errOut, d, []string{"serve", "extra"}); code != exitUsage {
		t.Fatalf("exit = %d, want %d", code, exitUsage)
	}
	if !strings.Contains(errOut.String(), "unexpected argument") {
		t.Errorf("stderr = %q, want unexpected-argument message", errOut.String())
	}
}

// TestRunServeBadAddr Listen 失败(非法地址)→ exit 1 且立即返回(不驻留)。
func TestRunServeBadAddr(t *testing.T) {
	var out, errOut strings.Builder
	d := &deps{store: fakeStoreCLI{}, rec: fakeRecognizerCLI{}, obj: fakeObjectsCLI{}}
	if code := run(&out, &errOut, d, []string{"serve", "--addr", "bogus:not-a-port"}); code != exitError {
		t.Fatalf("exit = %d, want %d; stderr = %q", code, exitError, errOut.String())
	}
	if !strings.Contains(errOut.String(), "serve:") {
		t.Errorf("stderr = %q, want listen error", errOut.String())
	}
}
