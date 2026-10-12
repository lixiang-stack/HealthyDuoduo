package storage

// 纯函数单测:查询分支映射与 S3 scheme 解析(不依赖 PG/S3 环境)。

import "testing"

func TestDateGranularity(t *testing.T) {
	cases := []struct {
		in   string
		want dateGrain
	}{
		{"", dateNone},
		{"2026", dateYear},
		{"2026-01", dateMonth},
		{"2026-01-02", dateExact},
		{"2026-1", dateExact}, // 非标准长度(过不了 CLI 正则)统一按 exact 处理
	}
	for _, c := range cases {
		if got := dateGranularity(c.in); got != c.want {
			t.Errorf("dateGranularity(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestSplitEndpoint(t *testing.T) {
	cases := []struct {
		in     string
		secure bool
		host   string
		fails  bool
	}{
		{"http://localhost:8333", false, "localhost:8333", false},
		{"https://s3.example.com", true, "s3.example.com", false},
		{"localhost:8333", false, "", true}, // 裸 host 拒绝:scheme 同时表达 TLS 意图,不接受第二种写法
	}
	for _, c := range cases {
		secure, host, err := splitEndpoint(c.in)
		if c.fails {
			if err == nil {
				t.Errorf("splitEndpoint(%q) expect error, got secure=%v host=%q", c.in, secure, host)
			}
			continue
		}
		if err != nil {
			t.Errorf("splitEndpoint(%q): %v", c.in, err)
			continue
		}
		if secure != c.secure || host != c.host {
			t.Errorf("splitEndpoint(%q) = (%v, %q), want (%v, %q)", c.in, secure, host, c.secure, c.host)
		}
	}
}
