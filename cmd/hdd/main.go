// hdd 是检查报告管理 MVP 的数据工程 CLI(实施计划决策 #9)。
// 四命令:ingest(全流程入库)/ reparse(仅重跑后处理)/ list(查询)/ show(单报告)。
// 编排逻辑在 internal/pipeline;HTTP 识别服务、PG、S3 适配器在 config.go 的 openDeps 注入。
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"regexp"
)

const usage = `usage: hdd ingest <image...> [--date YYYY-MM-DD] [--force]
       hdd reparse <report-id...> [--date YYYY-MM-DD]
       hdd list [--type <类别>] [--date YYYY-MM-DD]
       hdd show <report-id>`

var dateRe = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)

func main() {
	cfg := envConfig()
	d, cleanup, err := openDeps(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "hdd: 服务初始化失败: %v\n", err)
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

// fs 带统一 usage 输出的子命令 FlagSet。
func fs(name string, errOut io.Writer) *flag.FlagSet {
	f := flag.NewFlagSet(name, flag.ContinueOnError)
	f.SetOutput(errOut)
	return f
}
