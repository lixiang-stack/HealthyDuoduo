package main

// Four subcommands; one line per record so demo scripts can assert on output
// (implementation plan decision #11, command-level acceptance).
// Interface text is English per acceptance review; report_type DATA values (血常规)
// display as CBC when a display alias is known.
// Identifier: 图像内容 sha256(完整 64 hex 或 ≥8 hex 唯一前缀)。

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"strconv"
	"text/tabwriter"

	"healthyduoduo/internal/contract"
	"healthyduoduo/internal/pipeline"
)

// deps 编排层依赖(接口在 internal/pipeline,实现在 internal/storage、internal/recognizer)。
type deps struct {
	store pipeline.Store
	rec   pipeline.Recognizer
	obj   pipeline.ObjectStore
}

// typeDisplay 已知报告类别 → CLI 显示缩写(域数据 storage 中原样保留)。
var typeDisplay = map[string]string{"血常规": "CBC"}

// typeAlias 显示缩写 → 域数据值(--type 亦接受缩写输入)。
var typeAlias = map[string]string{"CBC": "血常规"}

func abbrType(t string) string {
	if a, ok := typeDisplay[t]; ok {
		return a
	}
	return t
}

func domType(in string) string {
	if d, ok := typeAlias[in]; ok {
		return d
	}
	return in
}

// shortSha 为终端可读性取 sha256 前 12 hex(git 风格短标识;show 头行同时给完整值)。
func shortSha(sha string) string {
	if len(sha) > 12 {
		return sha[:12]
	}
	return sha
}

// parseIngestArgs 位置参数与 flag 可混排(--date 需带值,--force 为布尔)。
func parseIngestArgs(args []string, date *string, force *bool, errOut io.Writer) ([]string, error) {
	var images []string
	for i := 0; i < len(args); i++ {
		switch a := args[i]; {
		case a == "--date" || a == "-date":
			if i+1 >= len(args) {
				fmt.Fprintf(errOut, "ingest: %s needs a value (YYYY-MM-DD)\n", a)
				return nil, errors.New("missing date value")
			}
			i++
			*date = args[i]
		case a == "--force" || a == "-force":
			*force = true
		case len(a) > 1 && a[0] == '-':
			fmt.Fprintf(errOut, "ingest: unknown flag %q\n%s\n", a, usage)
			return nil, fmt.Errorf("unknown flag: %q", a)
		default:
			images = append(images, a)
		}
	}
	return images, nil
}

func runIngest(out, errOut io.Writer, d *deps, args []string) int {
	var date string
	var force bool
	images, err := parseIngestArgs(args, &date, &force, errOut)
	if err != nil {
		return 2
	}
	if len(images) == 0 {
		fmt.Fprintf(errOut, "ingest: at least one image is required\n%s\n", usage)
		return 2
	}
	if date != "" && !fullDateRe.MatchString(date) {
		fmt.Fprintf(errOut, "ingest: --date must be YYYY-MM-DD, got %q\n", date)
		return 2
	}
	exit := 0
	for _, path := range images {
		oc, err := pipeline.Ingest(context.Background(), d.store, d.rec, d.obj, path, date, force)
		if err != nil {
			fmt.Fprintf(errOut, "ingest %s: %v\n", path, err)
			exit = 1
			continue
		}
		switch oc.Kind {
		case pipeline.KindCached:
			fmt.Fprintf(out, "sha256=%s: idempotent duplicate (same image already ingested; use --force to rerun)\n", shortSha(oc.Sha256))
		case pipeline.KindBackfilled:
			fmt.Fprintf(out, "sha256=%s: date backfilled via --date, %s (%d items)\n", shortSha(oc.Sha256), summary(oc.Report), len(oc.Report.Items))
		default:
			fmt.Fprintf(out, "sha256=%s: %s (%d items)\n", shortSha(oc.Sha256), summary(oc.Report), len(oc.Report.Items))
		}
	}
	return exit
}

func runReparse(out, errOut io.Writer, d *deps, args []string) int {
	var date string
	shas := make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		switch a := args[i]; {
		case a == "--date" || a == "-date":
			if i+1 >= len(args) {
				fmt.Fprintf(errOut, "reparse: %s needs a value (YYYY-MM-DD)\n", a)
				return 2
			}
			i++
			date = args[i]
		case len(a) > 1 && a[0] == '-':
			fmt.Fprintf(errOut, "reparse: unknown flag %q\n", a)
			return 2
		default:
			shas = append(shas, a)
		}
	}
	if len(shas) == 0 {
		fmt.Fprintf(errOut, "reparse: at least one sha256 is required\n%s\n", usage)
		return 2
	}
	if date != "" && !fullDateRe.MatchString(date) {
		fmt.Fprintf(errOut, "reparse: --date must be YYYY-MM-DD, got %q\n", date)
		return 2
	}
	exit := 0
	for _, shaArg := range shas {
		report, err := pipeline.Reparse(context.Background(), d.store, d.rec, shaArg, date)
		if err != nil {
			fmt.Fprintf(errOut, "reparse %s: %v\n", shaArg, err)
			exit = 1
			continue
		}
		fmt.Fprintf(out, "reparse %s: %s (%d items)\n", shortSha(shaArg), summary(report), len(report.Items))
	}
	return exit
}

func runList(out, errOut io.Writer, d *deps, args []string) int {
	listFlags := flag.NewFlagSet("list", flag.ContinueOnError)
	listFlags.SetOutput(errOut)
	var reportType, date string
	listFlags.StringVar(&reportType, "type", "", "filter by report type, e.g. CBC (= 血常规)")
	listFlags.StringVar(&date, "date", "", "filter by check date: YYYY or YYYY-MM or YYYY-MM-DD")
	if err := listFlags.Parse(args); err != nil {
		return 2
	}
	if date != "" && !listDateRe.MatchString(date) {
		fmt.Fprintf(errOut, "list: --date must be YYYY, YYYY-MM or YYYY-MM-DD, got %q\n", date)
		return 2
	}
	rows, err := pipeline.ListReports(context.Background(), d.store, domType(reportType), date)
	if err != nil {
		fmt.Fprintf(errOut, "list: %v\n", err)
		return 1
	}
	tw := tabwriter.NewWriter(out, 0, 4, 2, ' ', 0)
	fmt.Fprintln(tw, "sha256\tocr\tdate\ttype\tstatus\titems")
	for _, r := range rows {
		fmt.Fprintf(tw, "%s\t%d\t%s\t%s\t%s\t%d\n",
			shortSha(r.Sha256), r.OCRResultID, orDash(r.ReportDate), abbrType(r.ReportType), r.Status, r.ItemsCount)
	}
	return twError(tw)
}

func runShow(out, errOut io.Writer, d *deps, args []string) int {
	if len(args) == 0 {
		fmt.Fprintf(errOut, "show: at least one sha256 is required\n%s\n", usage)
		return 2
	}
	exit := 0
	for _, arg := range args {
		row, err := pipeline.ShowReport(context.Background(), d.store, arg)
		if err != nil {
			fmt.Fprintf(errOut, "show %s: %v\n", arg, err)
			exit = 1
			continue
		}
		fmt.Fprintf(out, "report sha256=%s: %s %s %s (ocr=%d)\n",
			row.Sha256, abbrType(row.ReportType), orDash(row.ReportDate), row.Status, row.OCRResultID)
		for _, it := range row.Items {
			mark := ""
			if it.LowConfidence {
				mark = " [low_confidence]"
			}
			fmt.Fprintf(out, "  %s value=%s unit=%s ref_range=%s flag=%s%s raw_text=%q\n",
				it.Name, orNum(it.Value), orDash(it.Unit), orDash(it.RefRange), it.Flag, mark, it.RawText)
		}
	}
	return exit
}

func twError(tw *tabwriter.Writer) int {
	if err := tw.Flush(); err != nil {
		return 1
	}
	return 0
}

func orDash(p *string) string {
	if p == nil {
		return "-"
	}
	return *p
}

func orNum(p *float64) string {
	if p == nil {
		return "-"
	}
	return strconv.FormatFloat(*p, 'f', -1, 64)
}

func summary(r *contract.Report) string {
	date := "-"
	if r.ReportDate != nil {
		date = *r.ReportDate
	}
	return fmt.Sprintf("%s %s %s", abbrType(r.ReportType), date, r.Status)
}
