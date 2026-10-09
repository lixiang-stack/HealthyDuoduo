// hdd 是检查报告管理 MVP 的数据工程 CLI(实施计划决策 #9)。
// 四命令:ingest(全流程入库)/ reparse(仅重跑后处理)/ list(查询)/ show(单报告)。
// 编排逻辑在 internal/pipeline;HTTP 识别服务、PG、S3 适配器在 config.go 的 openDeps 注入。
// 用户可读输出(usage/错误信息)统一英文(review #18);report_type 等域值按数据原样保留。
package main

import (
	"fmt"
	"io"
	"os"
	"regexp"
)

const usage = `usage: hdd ingest <image...> [--date YYYY-MM-DD] [--force]
       hdd reparse <sha256...> [--date YYYY-MM-DD]
       hdd list [--type <report-type>] [--date YYYY|YYYY-MM|YYYY-MM-DD]
       hdd show <sha256>`

// fullDateRe --date(ingest/reparse 补录的取值)必须为完整日期。
var fullDateRe = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)

// listDateRe --date(list 查找粒度)支持 YYYY / YYYY-MM / YYYY-MM-DD。
var listDateRe = regexp.MustCompile(`^\d{4}(-\d{2})?(-\d{2})?$`)

func main() {
	cfg := envConfig()
	d, cleanup, err := openDeps(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "hdd: failed to initialize services: %v\n", err)
		os.Exit(1)
	}
	defer cleanup()
	os.Exit(run(os.Stdout, os.Stderr, d, os.Args[1:]))
}

// run 解析参数并执行子命令;deps 注入以便单测(fake 在 main_test.go)。
func run(out, errOut io.Writer, d *deps, args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(errOut, usage)
		return 2
	}
	cmd, rest := args[0], args[1:]
	switch cmd {
	case "ingest":
		return runIngest(out, errOut, d, rest)
	case "reparse":
		return runReparse(out, errOut, d, rest)
	case "list":
		return runList(out, errOut, d, rest)
	case "show":
		return runShow(out, errOut, d, rest)
	default:
		fmt.Fprintf(errOut, "hdd: unknown command %q\n%s\n", cmd, usage)
		return 2
	}
}
