// hdd 是检查报告管理的数据工程 CLI。
// P0:四命令仅占位,打印 not implemented;实现分别落地于 P1/P2。
package main

import (
	"fmt"
	"io"
	"os"
)

// run 执行一个 hdd 子命令,返回进程退出码。
// P0:四命令均未实现,打印 not implemented;实现分别落地于 P1/P2。
func run(out io.Writer, args []string) int {
	switch {
	case len(args) == 0:
		fmt.Fprintln(out, "usage: hdd <ingest|reparse|list|show> [args]")
		return 2
	case args[0] == "ingest" || args[0] == "reparse" || args[0] == "list" || args[0] == "show":
		fmt.Fprintf(out, "hdd %s: not implemented\n", args[0])
		return 1
	default:
		fmt.Fprintf(out, "hdd: unknown command %q\n", args[0])
		fmt.Fprintln(out, "usage: hdd <ingest|reparse|list|show> [args]")
		return 2
	}
}

func main() {
	os.Exit(run(os.Stdout, os.Args[1:]))
}
