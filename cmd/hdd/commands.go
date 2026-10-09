package main

// 四个子命令;输出一行一条,演示脚本可直接断言(实施计划决策 #11 命令级验收)。

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strconv"

	"healthyduoduo/internal/contract"
	"healthyduoduo/internal/pipeline"
)

// deps 编排层依赖(接口在 internal/pipeline,实现在 internal/storage、internal/recognizer)。
type deps struct {
	store pipeline.Store
	rec   pipeline.Recognizer
	obj   pipeline.ObjectStore
}

// parseIngestArgs:位置参数与 flag 可混排(--date 需带值,--force 为布尔)。
func parseIngestArgs(args []string, date *string, force *bool, errOut io.Writer) ([]string, error) {
	var images []string
	for i := 0; i < len(args); i++ {
		switch a := args[i]; {
		case a == "--date" || a == "-date":
			if i+1 >= len(args) {
				fmt.Fprintf(errOut, "ingest: %s 需要一个值(YYYY-MM-DD)\n", a)
				return nil, errors.New("missing date value")
			}
			i++
			*date = args[i]
		case a == "--force" || a == "-force":
			*force = true
		case len(a) > 1 && a[0] == '-':
			fmt.Fprintf(errOut, "ingest: 未知参数 %q\n用法:%s\n", a, usage)
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
		fmt.Fprintln(errOut, "ingest: 至少一张图像\n"+usage)
		return 2
	}
	if date != "" && !dateRe.MatchString(date) {
		fmt.Fprintf(errOut, "ingest: --date 需为 YYYY-MM-DD,得到 %q\n", date)
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
			fmt.Fprintf(out, "report %d: 幂等命中(同一图像已入库;--force 重跑)\n", oc.ReportID)
		case pipeline.KindBackfilled:
			fmt.Fprintf(out, "report %d: 已用 --date 补录日期,%s(%d 项)\n", oc.ReportID, summary(oc.Report), len(oc.Report.Items))
		default:
			fmt.Fprintf(out, "report %d: %s(%d 项)\n", oc.ReportID, summary(oc.Report), len(oc.Report.Items))
		}
	}
	return exit
}

func runReparse(out, errOut io.Writer, d *deps, args []string) int {
	var date string
	ids := make([]int64, 0, len(args))
	for i := 0; i < len(args); i++ {
		switch a := args[i]; {
		case a == "--date" || a == "-date":
			if i+1 >= len(args) {
				fmt.Fprintf(errOut, "reparse: %s 需要一个值(YYYY-MM-DD)\n", a)
				return 2
			}
			i++
			date = args[i]
		case len(a) > 1 && a[0] == '-':
			fmt.Fprintf(errOut, "reparse: 未知参数 %q\n", a)
			return 2
		default:
			id, err := strconv.ParseInt(a, 10, 64)
			if err != nil {
				fmt.Fprintf(errOut, "reparse: report id 需为整数,得到 %q\n", a)
				return 2
			}
			ids = append(ids, id)
		}
	}
	if len(ids) == 0 {
		fmt.Fprintln(errOut, "reparse: 至少一个 report id\n"+usage)
		return 2
	}
	if date != "" && !dateRe.MatchString(date) {
		fmt.Fprintf(errOut, "reparse: --date 需为 YYYY-MM-DD,得到 %q\n", date)
		return 2
	}
	exit := 0
	for _, id := range ids {
		report, err := pipeline.Reparse(context.Background(), d.store, d.rec, id, date)
		if err != nil {
			fmt.Fprintf(errOut, "reparse %d: %v\n", id, err)
			exit = 1
			continue
		}
		fmt.Fprintf(out, "reparse %d: %s(%d 项)\n", id, summary(report), len(report.Items))
	}
	return exit
}

func runList(out, errOut io.Writer, d *deps, args []string) int {
	f := fs("list", errOut)
	var reportType, date string
	f.StringVar(&reportType, "type", "", "按报告类别过滤,如 血常规")
	f.StringVar(&date, "date", "", "按检查日期过滤(YYYY-MM-DD)")
	if err := f.Parse(args); err != nil {
		return 2
	}
	if date != "" && !dateRe.MatchString(date) {
		fmt.Fprintf(errOut, "list: --date 需为 YYYY-MM-DD,得到 %q\n", date)
		return 2
	}
	rows, err := pipeline.ListReports(context.Background(), d.store, reportType, date)
	if err != nil {
		fmt.Fprintf(errOut, "list: %v\n", err)
		return 1
	}
	fmt.Fprintln(out, "id\timage\tocr\tdate\ttype\tstatus\titems")
	for _, r := range rows {
		fmt.Fprintf(out, "%d\t%d\t%d\t%s\t%s\t%s\t%d\n",
			r.ID, r.ImageID, r.OCRResultID, orDash(r.ReportDate), r.ReportType, r.Status, r.ItemsCount)
	}
	return 0
}

func runShow(out, errOut io.Writer, d *deps, args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(errOut, "show: 至少一个 report id\n"+usage)
		return 2
	}
	exit := 0
	for _, arg := range args {
		id, err := strconv.ParseInt(arg, 10, 64)
		if err != nil {
			fmt.Fprintf(errOut, "show: report id 需为整数,得到 %q\n", arg)
			exit = 2
			continue
		}
		row, err := pipeline.ShowReport(context.Background(), d.store, id)
		if err != nil {
			fmt.Fprintf(errOut, "show %d: %v\n", id, err)
			exit = 1
			continue
		}
		fmt.Fprintf(out, "report %d: %s %s %s (image=%d ocr=%d)\n",
			row.ID, row.ReportType, orDash(row.ReportDate), row.Status, row.ImageID, row.OCRResultID)
		for _, it := range row.Items {
			mark := ""
			if it.LowConfidence {
				mark = " [低置信]"
			}
			fmt.Fprintf(out, "  %s value=%s unit=%s ref_range=%s flag=%s%s raw_text=%q\n",
				it.Name, orNum(it.Value), orDash(it.Unit), orDash(it.RefRange), it.Flag, mark, it.RawText)
		}
	}
	return exit
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
	return fmt.Sprintf("%s %s %s", r.ReportType, date, r.Status)
}
