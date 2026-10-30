// serve 子命令:同一 hdd 二进制的常驻 HTTP 形态(viz-and-client spec「服务形态与部署」,
// ADR-0006 局域网先行)。handler 栈只依赖 openDeps 注入的编排层接口(存储/识别器/对象存储),
// API 前缀 /api/v1;契约冻结于 schemas/openapi.yaml(M0),health 为首个冻结端点。
package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"

	"healthyduoduo/internal/httpapi"
)

// version 数据服务版本(serve 健康检查与 OpenAPI info.version 同源对齐;升级随 PR 维护)。
const version = "0.1.0"

// healthResponse GET /api/v1/health 的响应体({ok, version};OpenAPI schemas/Health)。
type healthResponse struct {
	OK      bool   `json:"ok"`
	Version string `json:"version"`
}

// newServeHandler 装配 /api/v1 路由;deps 注入保证后续端点复用同一条依赖链。
func newServeHandler(d *deps) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/health", func(w http.ResponseWriter, r *http.Request) {
		// 进程能应答即健康:deps 未就绪时 serve 根本不启动(openDeps fail-fast)。
		writeJSON(w, http.StatusOK, healthResponse{OK: true, Version: version})
	})
	// 成员端点(票 05):建/列/改/删;删除级联报告(原图与 OCR 留痕)。
	mux.HandleFunc("GET /api/v1/members", handleListMembers(d.members))
	mux.HandleFunc("POST /api/v1/members", handleCreateMember(d.members))
	mux.HandleFunc("PATCH /api/v1/members/{member_id}", handleUpdateMember(d.members))
	mux.HandleFunc("DELETE /api/v1/members/{member_id}", handleDeleteMember(d.members))
	// 趋势端点(票 02):成员维度指标时序(unit 门控;无点 = 200 + [])。
	mux.HandleFunc("GET /api/v1/members/{member_id}/indicators/{name}/trend", handleGetTrend(d))
	// 项目浏览端点(panel-browse 票 03):项目列表与项目详情(按项目视图)。
	mux.HandleFunc("GET /api/v1/members/{member_id}/panels", handleListMemberPanels(d))
	mux.HandleFunc("GET /api/v1/members/{member_id}/panels/{panel}", handleGetPanelDetail(d))
	// 报告端点(票 06/07):时间线(成员过滤)、multipart 上传(sha256 幂等)、详情(含 OCR 行链接)。
	mux.HandleFunc("GET /api/v1/reports", handleListReports(d.reports))
	mux.HandleFunc("POST /api/v1/reports", handleUploadReport(d))
	mux.HandleFunc("GET /api/v1/reports/{report_id}", handleGetReport(d))
	// 报告自愈(票 10):改归属(时间线移位)、删除(级联指标项与修正对;图像与 OCR 留痕)。
	mux.HandleFunc("PATCH /api/v1/reports/{report_id}", handleReassignReport(d))
	mux.HandleFunc("DELETE /api/v1/reports/{report_id}", handleDeleteReport(d))
	// 核对端点(票 09):指标项确认/修正(flag 重算、修正对 append、状态派生单调)。
	mux.HandleFunc("PATCH /api/v1/reports/{report_id}/items/{item_id}", handleReviewItem(d))
	// 图像端点(票 07):后端代理对象存储、局域网直出。
	mux.HandleFunc("GET /api/v1/images/{sha256}", handleGetImage(d.obj))
	return mux
}

// writeJSON /api/v1 的 JSON 响应写出(状态码 + Content-Type 一次收口)。
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// writeErrorJSON 非 2xx 统一契约形状(httpapi.Error;message 中性事实侧)。
func writeErrorJSON(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, httpapi.Error{Message: message})
}

// 404 说明的单点定义(契约 Error.message;成员/报告身份未命中的多个端点共用同一文案)。
const (
	reportNotFoundMessage = "report not found"
	memberNotFoundMessage = "member not found"
)

// decodeJSONBody 严格解码单个 JSON 对象请求体:未知字段拒绝(契约 additionalProperties:false
// 的端侧执行)、拒绝尾随数据。错误文案面向调试,中性事实侧。
func decodeJSONBody(r *http.Request, dst any) error {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		if errors.Is(err, io.EOF) {
			return errors.New("request body is required")
		}
		return err
	}
	if err := dec.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return errors.New("request body must be a single JSON object")
	}
	return nil
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
	if err := http.Serve(ln, newServeHandler(d)); err != nil {
		fmt.Fprintf(errOut, "serve: %v\n", err)
		return exitError
	}
	return 0
}
