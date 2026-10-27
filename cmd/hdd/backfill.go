package main

// hdd backfill-panels:存量报告按识别服务 /classify 重算项目组，只写 panel 列
// (panel-browse 票 05)。逐份失败继续并计数；识别服务不可达时停止并提示部署顺序。

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"strings"

	"healthyduoduo/internal/pipeline"
)

func runBackfillPanels(out, errOut io.Writer, d *deps, args []string) int {
	fs := flag.NewFlagSet("backfill-panels", flag.ContinueOnError)
	fs.SetOutput(errOut)
	var limit int
	var dryRun bool
	fs.IntVar(&limit, "limit", 0, "process at most N newest reports (0 = all; reruns rescan the same newest N)")
	fs.BoolVar(&dryRun, "dry-run", false, "print planned panel updates without writing")
	if err := fs.Parse(args); err != nil {
		return exitUsage
	}
	if fs.NArg() > 0 {
		fmt.Fprintf(errOut, "backfill-panels: unexpected argument %q\n%s\n", fs.Arg(0), usage)
		return exitUsage
	}
	if limit < 0 {
		fmt.Fprintf(errOut, "backfill-panels: --limit must be >= 0, got %d\n", limit)
		return exitUsage
	}

	res, err := pipeline.BackfillPanels(context.Background(), d.backfill, d.classifier, pipeline.BackfillOptions{
		Limit:  limit,
		DryRun: dryRun,
	})
	if res != nil {
		for _, report := range res.Reports {
			printBackfillOutcome(out, errOut, report, dryRun)
		}
		printBackfillSummary(out, res, dryRun)
	}
	if err != nil {
		fmt.Fprintf(errOut, "backfill-panels: %v\n", err)
		if errors.Is(err, pipeline.ErrClassifierUnavailable) {
			fmt.Fprintln(errOut, "backfill-panels: start the recognizer service before backfilling (deploy order: recognizer -> migrations -> backfill-panels -> app)")
		}
		return exitError
	}
	if res.Failed > 0 {
		return exitError
	}
	return 0
}

func printBackfillOutcome(out, errOut io.Writer, report pipeline.BackfillReportOutcome, dryRun bool) {
	sha := shortSha(report.Sha256)
	if report.Err != nil {
		fmt.Fprintf(errOut, "backfill-panels %s: %v\n", sha, report.Err)
		return
	}
	if report.Skipped {
		fmt.Fprintf(out, "backfill-panels %s: skipped (%s)\n", sha, report.SkipReason)
		return
	}
	verb := "updated"
	if dryRun {
		verb = "would update"
	}
	fmt.Fprintf(out, "backfill-panels %s: %s panels=[%s] items=%d/%d\n",
		sha, verb, strings.Join(report.Panels, " "), len(report.Changes), report.ItemCount)
	if !dryRun {
		return
	}
	for _, change := range report.Changes {
		fmt.Fprintf(out, "  item %d %s: %s -> %s\n", change.ItemID, change.Name, change.From, change.To)
	}
}

func printBackfillSummary(out io.Writer, res *pipeline.BackfillResult, dryRun bool) {
	scope := fmt.Sprintf("%d report(s)", len(res.Reports))
	if res.Total != len(res.Reports) {
		scope = fmt.Sprintf("%d of %d report(s) scanned", len(res.Reports), res.Total)
	}
	if dryRun {
		fmt.Fprintf(out, "backfill-panels: dry-run, %s: would update %d, skipped %d, failed %d (no changes written)\n",
			scope, res.Updated, res.Skipped, res.Failed)
		return
	}
	fmt.Fprintf(out, "backfill-panels: %s: updated %d, skipped %d, failed %d\n",
		scope, res.Updated, res.Skipped, res.Failed)
}
