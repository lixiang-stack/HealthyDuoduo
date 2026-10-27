package main

// hdd backfill-panels 命令层单测:计数、dry-run 输出、跳过 / 失败与不可用提示。

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"healthyduoduo/internal/pipeline"
)

type fakeBackfillCLI struct {
	reports  []pipeline.BackfillReport
	updates  []fakeBackfillUpdate
	gotLimit int
}

type fakeBackfillUpdate struct {
	sha        string
	panels     []string
	itemPanels map[int64]string
}

func (f *fakeBackfillCLI) AllReportsForBackfill(_ context.Context, limit int) ([]pipeline.BackfillReport, error) {
	f.gotLimit = limit
	return f.reports, nil
}

func (f *fakeBackfillCLI) UpdateReportPanels(_ context.Context, sha string, panels []string, itemPanels map[int64]string) error {
	f.updates = append(f.updates, fakeBackfillUpdate{sha: sha, panels: panels, itemPanels: itemPanels})
	return nil
}

type fakeClassifierCLI struct {
	fn func() ([]string, []string, error)
}

func (f fakeClassifierCLI) Classify(context.Context, string, []pipeline.ClassifyItem) ([]string, []string, error) {
	return f.fn()
}

func backfillDeps(store *fakeBackfillCLI, classifier fakeClassifierCLI) *deps {
	return &deps{backfill: store, classifier: classifier}
}

func TestRunBackfillPanelsUpdatesAndPrintsCounts(t *testing.T) {
	store := &fakeBackfillCLI{reports: []pipeline.BackfillReport{{
		Sha256: strings.Repeat("a", 64), ReportType: "生化", Status: "success",
		Panels: []string{"other"},
		Items: []pipeline.BackfillItem{
			{ID: 1, Name: "丙氨酸转氨酶", Panel: "other"},
			{ID: 2, Name: "肌酐", Panel: "other"},
		},
	}}}
	classifier := fakeClassifierCLI{fn: func() ([]string, []string, error) {
		return []string{"liver", "kidney"}, []string{"liver", "kidney"}, nil
	}}
	var out, errOut bytes.Buffer
	code := run(&out, &errOut, backfillDeps(store, classifier), []string{"backfill-panels"})
	if code != 0 {
		t.Fatalf("exit = %d, stderr = %q", code, errOut.String())
	}
	for _, want := range []string{
		"updated panels=[liver kidney] items=2/2",
		"1 report(s): updated 1, skipped 0, failed 0",
	} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("stdout = %q, want %q", out.String(), want)
		}
	}
	if len(store.updates) != 1 || store.updates[0].sha != strings.Repeat("a", 64) {
		t.Errorf("updates = %+v, want one write", store.updates)
	}
}

func TestRunBackfillPanelsDryRunPrintsPlanWithoutWriting(t *testing.T) {
	store := &fakeBackfillCLI{reports: []pipeline.BackfillReport{{
		Sha256: strings.Repeat("a", 64), ReportType: "生化", Status: "success",
		Items: []pipeline.BackfillItem{{ID: 7, Name: "肌酐", Panel: "other"}},
	}}}
	classifier := fakeClassifierCLI{fn: func() ([]string, []string, error) {
		return []string{"kidney"}, []string{"kidney"}, nil
	}}
	var out, errOut bytes.Buffer
	code := run(&out, &errOut, backfillDeps(store, classifier), []string{"backfill-panels", "--dry-run"})
	if code != 0 {
		t.Fatalf("exit = %d, stderr = %q", code, errOut.String())
	}
	for _, want := range []string{
		"would update panels=[kidney] items=1/1",
		"item 7 肌酐: other -> kidney",
		"dry-run, 1 report(s): would update 1, skipped 0, failed 0 (no changes written)",
	} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("stdout = %q, want %q", out.String(), want)
		}
	}
	if len(store.updates) != 0 {
		t.Errorf("dry-run wrote %d update(s)", len(store.updates))
	}
}

func TestRunBackfillPanelsPrintsSkippedReasons(t *testing.T) {
	store := &fakeBackfillCLI{reports: []pipeline.BackfillReport{
		{Sha256: strings.Repeat("a", 64), ReportType: "血常规", Status: "success",
			Panels: []string{"cbc"}, Items: []pipeline.BackfillItem{{ID: 1, Name: "血红蛋白", Panel: "cbc"}}},
		{Sha256: strings.Repeat("b", 64), ReportType: "其他专项", Status: "failed", Panels: []string{}},
	}}
	classifier := fakeClassifierCLI{fn: func() ([]string, []string, error) {
		return []string{"cbc"}, []string{"cbc"}, nil
	}}
	var out, errOut bytes.Buffer
	code := run(&out, &errOut, backfillDeps(store, classifier), []string{"backfill-panels"})
	if code != 0 {
		t.Fatalf("exit = %d, stderr = %q", code, errOut.String())
	}
	for _, want := range []string{"skipped (up to date)", "skipped (failed report)", ": updated 0, skipped 2, failed 0"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("stdout = %q, want %q", out.String(), want)
		}
	}
}

func TestRunBackfillPanelsReportsFailuresAndExitsNonZero(t *testing.T) {
	store := &fakeBackfillCLI{reports: []pipeline.BackfillReport{{
		Sha256: strings.Repeat("a", 64), ReportType: "生化", Status: "success",
		Items: []pipeline.BackfillItem{{ID: 1, Name: "肌酐", Panel: "other"}},
	}}}
	classifier := fakeClassifierCLI{fn: func() ([]string, []string, error) {
		return nil, nil, errors.New("bad report")
	}}
	var out, errOut bytes.Buffer
	code := run(&out, &errOut, backfillDeps(store, classifier), []string{"backfill-panels"})
	if code != 1 {
		t.Fatalf("exit = %d, want 1", code)
	}
	for _, want := range []string{"bad report", "1 report(s): updated 0, skipped 0, failed 1"} {
		if !strings.Contains(errOut.String()+out.String(), want) {
			t.Errorf("output = %q, want %q", errOut.String()+out.String(), want)
		}
	}
}

func TestRunBackfillPanelsHintsDeployOrderWhenUnavailable(t *testing.T) {
	store := &fakeBackfillCLI{reports: []pipeline.BackfillReport{
		{Sha256: strings.Repeat("a", 64), ReportType: "生化", Status: "success",
			Items: []pipeline.BackfillItem{{ID: 1, Name: "肌酐", Panel: "other"}}},
		{Sha256: strings.Repeat("b", 64), ReportType: "生化", Status: "success",
			Items: []pipeline.BackfillItem{{ID: 2, Name: "肌酐", Panel: "other"}}},
	}}
	classifier := fakeClassifierCLI{fn: func() ([]string, []string, error) {
		return nil, nil, pipeline.ErrClassifierUnavailable
	}}
	var out, errOut bytes.Buffer
	code := run(&out, &errOut, backfillDeps(store, classifier), []string{"backfill-panels"})
	if code != 1 {
		t.Fatalf("exit = %d, want 1", code)
	}
	for _, want := range []string{"recognizer unavailable", "1 of 2 report(s) scanned", "deploy order: recognizer -> migrations -> backfill-panels -> app"} {
		if !strings.Contains(errOut.String()+out.String(), want) {
			t.Errorf("output = %q, want %q", errOut.String()+out.String(), want)
		}
	}
}

func TestRunBackfillPanelsRejectsBadArgs(t *testing.T) {
	store := &fakeBackfillCLI{}
	for _, args := range [][]string{
		{"backfill-panels", "--limit", "-1"},
		{"backfill-panels", "unexpected"},
		{"backfill-panels", "--bogus"},
	} {
		var out, errOut bytes.Buffer
		if code := run(&out, &errOut, backfillDeps(store, fakeClassifierCLI{}), args); code != 2 {
			t.Errorf("args %v exit = %d, want 2 (stderr=%q)", args, code, errOut.String())
		}
	}
}

func TestRunBackfillPanelsPassesLimit(t *testing.T) {
	store := &fakeBackfillCLI{}
	var out, errOut bytes.Buffer
	if code := run(&out, &errOut, backfillDeps(store, fakeClassifierCLI{}), []string{"backfill-panels", "--limit", "5"}); code != 0 {
		t.Fatalf("exit = %d, stderr = %q", code, errOut.String())
	}
	if store.gotLimit != 5 {
		t.Errorf("limit = %d, want 5", store.gotLimit)
	}
}
