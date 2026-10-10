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
var typeDisplay = map[string]string{
	"血常规":   "CBC",
	"尿常规":   "UA",
	"血糖":    "GLU",
	"肝肾功能":  "LFT",
	"甲状腺功能": "TFT",
	"超声":    "US",
}

// typeAlias 显示缩写 → 域数据值(--type 亦接受缩写输入);由 typeDisplay 反转派生,单一数据源。
var typeAlias = reverseMap(typeDisplay)

// reverseMap 反转 k↔v(typeDisplay 的缩写与域数据值一一对应)。
func reverseMap(m map[string]string) map[string]string {
	out := make(map[string]string, len(m))
	for k, v := range m {
		out[v] = k
	}
	return out
}

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

// parseIngestArgs 位置参数与 flag 可混排(--date/--preprocess 需带值,--force 为布尔)。
func parseIngestArgs(args []string, date, preprocess *string, force *bool, errOut io.Writer) ([]string, error) {
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
		case a == "--preprocess" || a == "-preprocess":
			if i+1 >= len(args) {
				fmt.Fprintf(errOut, "ingest: %s needs a value (comma-separated steps, e.g. gray,deskew)\n", a)
				return nil, errors.New("missing preprocess value")
			}
			i++
			*preprocess = args[i]
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
	var date, preprocess string
	var force bool
	images, err := parseIngestArgs(args, &date, &preprocess, &force, errOut)
	if err != nil {
		return exitUsage
	}
	if len(images) == 0 {
		fmt.Fprintf(errOut, "ingest: at least one image is required\n%s\n", usage)
		return exitUsage
	}
	if date != "" && !fullDateRe.MatchString(date) {
		fmt.Fprintf(errOut, "ingest: --date must be YYYY-MM-DD, got %q\n", date)
		return exitUsage
	}
	exit := 0
	for _, path := range images {
		oc, err := pipeline.Ingest(context.Background(), d.store, d.rec, d.obj, path, date, preprocess, force)
		if err != nil {
			fmt.Fprintf(errOut, "ingest %s: %v\n", path, err)
			exit = exitError
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
				return exitUsage
			}
			i++
			date = args[i]
		case len(a) > 1 && a[0] == '-':
			fmt.Fprintf(errOut, "reparse: unknown flag %q\n", a)
			return exitUsage
		default:
			shas = append(shas, a)
		}
	}
	if len(shas) == 0 {
		fmt.Fprintf(errOut, "reparse: at least one sha256 is required\n%s\n", usage)
		return exitUsage
	}
	if date != "" && !fullDateRe.MatchString(date) {
		fmt.Fprintf(errOut, "reparse: --date must be YYYY-MM-DD, got %q\n", date)
		return exitUsage
	}
	exit := 0
	for _, shaArg := range shas {
		report, err := pipeline.Reparse(context.Background(), d.store, d.rec, shaArg, date)
		if err != nil {
			fmt.Fprintf(errOut, "reparse %s: %v\n", shaArg, err)
			exit = exitError
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
	listFlags.StringVar(&reportType, "type", "", "filter by report type, e.g. CBC (= 血常规), UA (= 尿常规)")
	listFlags.StringVar(&date, "date", "", "filter by check date: YYYY or YYYY-MM or YYYY-MM-DD")
	if err := listFlags.Parse(args); err != nil {
		return exitUsage
	}
	if date != "" && !listDateRe.MatchString(date) {
		fmt.Fprintf(errOut, "list: --date must be YYYY, YYYY-MM or YYYY-MM-DD, got %q\n", date)
		return exitUsage
	}
	rows, err := d.store.ListReports(context.Background(), domType(reportType), date)
	if err != nil {
		fmt.Fprintf(errOut, "list: %v\n", err)
		return exitError
	}
	tw := tabwriter.NewWriter(out, 0, 4, 2, ' ', 0)
	fmt.Fprintln(tw, "sha256\tocr\tdate\ttype\tstatus\titems")
	for _, r := range rows {
		fmt.Fprintf(tw, "%s\t%d\t%s\t%s\t%s\t%d\n",
			shortSha(r.Sha256), r.OCRResultID, orDash(r.ReportDate), abbrType(r.ReportType), r.Status, r.ItemsCount)
	}
	return twError(tw)
}

// runTrend hdd trend <指标名> [--type]:同一指标项跨报告时序对比(P3);
// 行升序,delta 为相邻点差值(单位一致才计,否则 "-")。
func runTrend(out, errOut io.Writer, d *deps, args []string) int {
	trendFlags := flag.NewFlagSet("trend", flag.ContinueOnError)
	trendFlags.SetOutput(errOut)
	var reportType string
	trendFlags.StringVar(&reportType, "type", "", "filter by report type, e.g. CBC (= 血常规)")
	if err := trendFlags.Parse(args); err != nil {
		return exitUsage
	}
	names := trendFlags.Args()
	if len(names) != 1 {
		fmt.Fprintf(errOut, "trend: exactly one item name is required\n%s\n", usage)
		return exitUsage
	}
	rows, err := d.store.TrendByName(context.Background(), names[0], domType(reportType))
	if err != nil {
		fmt.Fprintf(errOut, "trend: %v\n", err)
		return exitError
	}
	tw := tabwriter.NewWriter(out, 0, 4, 2, ' ', 0)
	fmt.Fprintln(tw, "date\ttype\tvalue\tunit\tdelta\tflag\tsha256")
	var prev *pipeline.TrendRow
	for _, r := range rows {
		delta := "-"
		if prev != nil && sameUnit(prev.Unit, r.Unit) && prev.Value != nil && r.Value != nil {
			diff := *r.Value - *prev.Value
			delta = strconv.FormatFloat(diff, 'f', -1, 64)
			if diff > 0 {
				delta = "+" + delta
			}
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
			orDash(r.ReportDate), abbrType(r.ReportType), orNum(r.Value), orDash(r.Unit), delta, r.Flag, shortSha(r.Sha256))
		prev = &r
	}
	return twError(tw)
}

func runShow(out, errOut io.Writer, d *deps, args []string) int {
	if len(args) == 0 {
		fmt.Fprintf(errOut, "show: at least one sha256 is required\n%s\n", usage)
		return exitUsage
	}
	exit := 0
	for _, arg := range args {
		row, err := pipeline.ShowReport(context.Background(), d.store, arg)
		if err != nil {
			fmt.Fprintf(errOut, "show %s: %v\n", arg, err)
			exit = exitError
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

// sameUnit 相邻趋势点单位一致才允许计 delta(单位换了不可同比)。
func sameUnit(a, b *string) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}

func twError(tw *tabwriter.Writer) int {
	if err := tw.Flush(); err != nil {
		return exitError
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
