// serve 子命令:同一 hdd 二进制的常驻 HTTP 形态(viz-and-client spec「服务形态与部署」,
// ADR-0006 局域网先行)。handler 栈只依赖 openDeps 注入的编排层接口(存储/识别器/对象存储),
// API 前缀 /api/v1;契约冻结于 schemas/openapi.yaml(M0),health 为首个冻结端点。
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
)

// version 数据服务版本(serve 健康检查与 OpenAPI info.version 同源对齐;升级随 PR 维护)。
const version = "0.1.0"

// corsOriginEnv CORS 开关的 env 名;未设/空 = 关闭(与未加中间件完全一致)。
// 默认关闭是安全选择:ADR-0006 局域网无鉴权,常驻实例放开跨源会让任意网页可读家庭数据;
// 仅浏览器验证通道(Expo Web)在开发栈里显式打开。
const corsOriginEnv = "HDD_SERVE_CORS_ORIGIN"

// healthResponse GET /api/v1/health 的响应体({ok, version};OpenAPI schemas/Health)。
type healthResponse struct {
	OK      bool   `json:"ok"`
	Version string `json:"version"`
}

// newServeHandler 装配 /api/v1 路由;deps 注入保证后续端点复用同一条依赖链。
// corsOrigin 为空即不包 CORS 中间件。
func newServeHandler(d *deps, corsOrigin string) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// 进程能应答即健康:deps 未就绪时 serve 根本不启动(openDeps fail-fast)。
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(healthResponse{OK: true, Version: version})
	})
	return withCORS(mux, corsOrigin)
}

// withCORS 按配置给全部响应加跨源头,并直接应答 OPTIONS 预检(204)。
// origin 为固定配置值(不随请求 Origin 变化),故无需 Vary。
func withCORS(next http.Handler, origin string) http.Handler {
	if origin == "" {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", origin)
		if r.Method == http.MethodOptions {
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PATCH, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
			w.Header().Set("Access-Control-Max-Age", "600")
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// runServe 常驻运行;返回码沿用 CLI 约定(启动错误=1,参数错误=2)。
// 监听地址缺省取 HDD_SERVE_ADDR(compose 内缺省 :8080 绑定全部网卡,LAN 直达),flag 可覆盖。
func runServe(out, errOut io.Writer, d *deps, args []string) int {
	serveFlags := flag.NewFlagSet("serve", flag.ContinueOnError)
	serveFlags.SetOutput(errOut)
	addr := serveFlags.String("addr", fromEnv("HDD_SERVE_ADDR", ":8080"), "HTTP listen address host:port")
	if err := serveFlags.Parse(args); err != nil {
		return exitUsage
	}
	if extra := serveFlags.Args(); len(extra) > 0 {
		fmt.Fprintf(errOut, "serve: unexpected argument %q\n%s\n", extra[0], usage)
		return exitUsage
	}
	// 先绑定再报"listening":bind 失败即时报错返回,不虚报已监听。
	ln, err := net.Listen("tcp", *addr)
	if err != nil {
		fmt.Fprintf(errOut, "serve: %v\n", err)
		return exitError
	}
	fmt.Fprintf(out, "hdd serve: listening on %s\n", ln.Addr())
	// 不做信号处理/优雅关停:骨架阶段进程即服务生命周期(compose stop 直接终止)。
	if err := http.Serve(ln, newServeHandler(d, fromEnv(corsOriginEnv, ""))); err != nil {
		fmt.Fprintf(errOut, "serve: %v\n", err)
		return exitError
	}
	return 0
}
