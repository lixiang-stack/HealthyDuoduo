package main

// 图像端点 HTTP 集成测试(票 07):对象存储直出(原样字节 + 内容嗅探 Content-Type)、
// 键不存在与非 sha256 形态 404。字节以仓库真实样本图(samples/cbc_05.jpg)直通验证。

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"healthyduoduo/internal/pipeline"
)

// sampleImageBytes 读仓库真实样本图(字节直通与内容嗅探都用真实数据)。
func sampleImageBytes(t *testing.T) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "samples", "cbc_05.jpg"))
	if err != nil {
		t.Fatalf("read sample image: %v", err)
	}
	return data
}

// newImagesTestServer 图像端点的 handler 栈(对象存储预置一个键)。
func newImagesTestServer(t *testing.T, objects map[string][]byte) *httptest.Server {
	t.Helper()
	d := &deps{store: fakeStoreCLI{}, rec: fakeRecognizerCLI{}, obj: fakeObjectsCLI{objects: objects}}
	srv := httptest.NewServer(newServeHandler(d, ""))
	t.Cleanup(srv.Close)
	return srv
}

// TestServeGetImage 命中:200 + 原样字节 + image/jpeg(上传不落 content-type,端点按内容嗅探)。
func TestServeGetImage(t *testing.T) {
	image := sampleImageBytes(t)
	sha := pipeline.ImageSha256Hex(image)
	srv := newImagesTestServer(t, map[string][]byte{sha: image})

	resp, err := http.Get(srv.URL + "/api/v1/images/" + sha)
	if err != nil {
		t.Fatalf("GET /images/%s: %v", sha, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); ct != "image/jpeg" {
		t.Errorf("content-type = %q, want image/jpeg (内容嗅探)", ct)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(body, image) {
		t.Errorf("body = %d bytes, want 原样字节 %d bytes", len(body), len(image))
	}
}

// TestServeGetImageNotFound 键不存在与非法形态一律 404 契约 Error 形状
// (对象键即图像身份:非 sha256 形态无法是任何对象,与不存在同答)。
func TestServeGetImageNotFound(t *testing.T) {
	srv := newImagesTestServer(t, map[string][]byte{})
	for _, id := range []string{
		strings.Repeat("a", 64), // 合法形态但无对象
		"not-a-sha",             // 非 hex
		strings.Repeat("A", 64), // 大写 hex 非法(身份为小写)
		strings.Repeat("a", 63), // 长度不足
	} {
		resp, err := http.Get(srv.URL + "/api/v1/images/" + id)
		if err != nil {
			t.Fatalf("GET /images/%s: %v", id, err)
		}
		raw, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("GET /images/%.12s… = %d, want 404; body = %s", id, resp.StatusCode, raw)
		}
		if !strings.Contains(string(raw), "image not found") {
			t.Errorf("GET /images/%.12s… body = %s, want 契约 Error 形状", id, raw)
		}
	}
}
