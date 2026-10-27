package pipeline_test

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"healthyduoduo/internal/pipeline"
)

type fakePanelUpdate struct {
	sha        string
	panels     []string
	itemPanels map[int64]string
}

// fakeBackfillStore 记录回填查询参数与写入内容;reports 可原地修改以验证幂等。
type fakeBackfillStore struct {
	reports   []pipeline.BackfillReport
	limit     int
	updates   []fakePanelUpdate
	updateErr error
}

func (f *fakeBackfillStore) AllReportsForBackfill(_ context.Context, limit int) ([]pipeline.BackfillReport, error) {
	f.limit = limit
	return f.reports, nil
}

func (f *fakeBackfillStore) UpdateReportPanels(_ context.Context, reportSha string, panels []string, itemPanels map[int64]string) error {
	if f.updateErr != nil {
		return f.updateErr
	}
	copied := make(map[int64]string, len(itemPanels))
	for id, panel := range itemPanels {
		copied[id] = panel
	}
	f.updates = append(f.updates, fakePanelUpdate{sha: reportSha, panels: panels, itemPanels: copied})
	return nil
}

type fakePanelClassifier struct {
	calls          int
	lastReportType string
	lastItems      []pipeline.ClassifyItem
	fn             func(reportType string, items []pipeline.ClassifyItem) ([]string, []string, error)
}

func (f *fakePanelClassifier) Classify(_ context.Context, reportType string, items []pipeline.ClassifyItem) ([]string, []string, error) {
	f.calls++
	f.lastReportType = reportType
	f.lastItems = items
	if f.fn == nil {
		return nil, nil, errors.New("unexpected classify call")
	}
	return f.fn(reportType, items)
}

func TestBackfillPanelsUpdatesChangedPanels(t *testing.T) {
	sha := strings.Repeat("a", 64)
	unit := "U/L"
	store := &fakeBackfillStore{reports: []pipeline.BackfillReport{{
		Sha256:     sha,
		ReportType: "生化",
		Status:     "success",
		Panels:     []string{"other"},
		Items: []pipeline.BackfillItem{
			{ID: 11, Name: "丙氨酸转氨酶", Unit: &unit, Panel: "other"},
			{ID: 12, Name: "肌酐", Unit: &unit, Panel: "other"},
			{ID: 13, Name: "总胆红素", Unit: &unit, Panel: "liver"},
		},
	}}}
	classifier := &fakePanelClassifier{fn: func(string, []pipeline.ClassifyItem) ([]string, []string, error) {
		return []string{"liver", "kidney"}, []string{"liver", "kidney", "liver"}, nil
	}}

	res, err := pipeline.BackfillPanels(context.Background(), store, classifier, pipeline.BackfillOptions{})
	if err != nil {
		t.Fatalf("BackfillPanels: %v", err)
	}
	if res.Updated != 1 || res.Skipped != 0 || res.Failed != 0 {
		t.Fatalf("counts = updated %d skipped %d failed %d, want 1/0/0", res.Updated, res.Skipped, res.Failed)
	}
	if len(store.updates) != 1 {
		t.Fatalf("updates = %d, want 1", len(store.updates))
	}
	if classifier.lastReportType != "生化" {
		t.Errorf("classify report_type = %q, want 生化", classifier.lastReportType)
	}
	wantClassifyItems := []pipeline.ClassifyItem{
		{Name: "丙氨酸转氨酶", Unit: &unit},
		{Name: "肌酐", Unit: &unit},
		{Name: "总胆红素", Unit: &unit},
	}
	if !reflect.DeepEqual(classifier.lastItems, wantClassifyItems) {
		t.Errorf("classify items = %+v, want %+v", classifier.lastItems, wantClassifyItems)
	}
	got := store.updates[0]
	if got.sha != sha || !reflect.DeepEqual(got.panels, []string{"liver", "kidney"}) {
		t.Errorf("update = %+v, want report panels [liver kidney]", got)
	}
	wantItems := map[int64]string{11: "liver", 12: "kidney", 13: "liver"}
	if !reflect.DeepEqual(got.itemPanels, wantItems) {
		t.Errorf("item panels = %v, want %v", got.itemPanels, wantItems)
	}
	out := res.Reports[0]
	wantChanges := []pipeline.BackfillChange{
		{ItemID: 11, Name: "丙氨酸转氨酶", From: "other", To: "liver"},
		{ItemID: 12, Name: "肌酐", From: "other", To: "kidney"},
	}
	if !reflect.DeepEqual(out.Changes, wantChanges) {
		t.Errorf("changes = %+v, want %+v", out.Changes, wantChanges)
	}
	if out.ItemCount != 3 || out.Skipped {
		t.Errorf("outcome = %+v, want 3 items and not skipped", out)
	}
}

func TestBackfillPanelsSkipsUpToDateAndFailedReports(t *testing.T) {
	store := &fakeBackfillStore{reports: []pipeline.BackfillReport{
		{
			Sha256:     strings.Repeat("a", 64),
			ReportType: "血常规",
			Status:     "success",
			Panels:     []string{"cbc"},
			Items:      []pipeline.BackfillItem{{ID: 1, Name: "血红蛋白", Panel: "cbc"}},
		},
		{
			Sha256:     strings.Repeat("b", 64),
			ReportType: "其他专项",
			Status:     "failed",
			Panels:     []string{},
			Items:      nil,
		},
	}}
	classifier := &fakePanelClassifier{fn: func(string, []pipeline.ClassifyItem) ([]string, []string, error) {
		return []string{"cbc"}, []string{"cbc"}, nil
	}}

	res, err := pipeline.BackfillPanels(context.Background(), store, classifier, pipeline.BackfillOptions{})
	if err != nil {
		t.Fatalf("BackfillPanels: %v", err)
	}
	if res.Updated != 0 || res.Skipped != 2 || res.Failed != 0 {
		t.Fatalf("counts = updated %d skipped %d failed %d, want 0/2/0", res.Updated, res.Skipped, res.Failed)
	}
	if classifier.calls != 1 {
		t.Errorf("classify calls = %d, want 1 (failed report must not be classified)", classifier.calls)
	}
	if len(store.updates) != 0 {
		t.Errorf("updates = %d, want 0 (no panel changes)", len(store.updates))
	}
	if res.Reports[0].SkipReason != pipeline.SkipUpToDate {
		t.Errorf("skip reason = %q, want %q", res.Reports[0].SkipReason, pipeline.SkipUpToDate)
	}
	if res.Reports[1].SkipReason != pipeline.SkipFailedReport {
		t.Errorf("skip reason = %q, want %q", res.Reports[1].SkipReason, pipeline.SkipFailedReport)
	}
}

func TestBackfillPanelsContinuesAfterPerReportFailures(t *testing.T) {
	ok := "血红蛋白"
	store := &fakeBackfillStore{reports: []pipeline.BackfillReport{
		{Sha256: strings.Repeat("a", 64), ReportType: "血常规", Status: "success",
			Items: []pipeline.BackfillItem{{ID: 1, Name: ok, Panel: "other"}}},
		{Sha256: strings.Repeat("b", 64), ReportType: "血常规", Status: "success",
			Items: []pipeline.BackfillItem{{ID: 2, Name: ok, Panel: "other"}}},
		{Sha256: strings.Repeat("c", 64), ReportType: "血常规", Status: "success",
			Items: []pipeline.BackfillItem{{ID: 3, Name: ok, Panel: "other"}}},
	}}
	calls := 0
	classifier := &fakePanelClassifier{fn: func(string, []pipeline.ClassifyItem) ([]string, []string, error) {
		calls++
		if calls <= 2 {
			return nil, nil, errors.New("boom")
		}
		return []string{"cbc"}, []string{"cbc"}, nil
	}}

	res, err := pipeline.BackfillPanels(context.Background(), store, classifier, pipeline.BackfillOptions{})
	if err != nil {
		t.Fatalf("BackfillPanels: %v", err)
	}
	if res.Updated != 1 || res.Failed != 2 || res.Skipped != 0 {
		t.Fatalf("counts = updated %d skipped %d failed %d, want 1/0/2", res.Updated, res.Skipped, res.Failed)
	}
	if res.Reports[0].Err == nil || res.Reports[1].Err == nil {
		t.Errorf("per-report errors = %v / %v, want both recorded", res.Reports[0].Err, res.Reports[1].Err)
	}
	if res.Reports[2].Err != nil {
		t.Errorf("last report error = %v, want success", res.Reports[2].Err)
	}
}

func TestBackfillPanelsRejectsMalformedClassifyResponse(t *testing.T) {
	store := &fakeBackfillStore{reports: []pipeline.BackfillReport{
		{Sha256: strings.Repeat("a", 64), ReportType: "生化", Status: "success",
			Items: []pipeline.BackfillItem{
				{ID: 1, Name: "肌酐", Panel: "other"},
				{ID: 2, Name: "尿酸", Panel: "other"},
			}},
	}}
	classifier := &fakePanelClassifier{fn: func(string, []pipeline.ClassifyItem) ([]string, []string, error) {
		return []string{"kidney"}, []string{"kidney"}, nil
	}}

	res, err := pipeline.BackfillPanels(context.Background(), store, classifier, pipeline.BackfillOptions{})
	if err != nil {
		t.Fatalf("BackfillPanels: %v", err)
	}
	if res.Failed != 1 || res.Updated != 0 {
		t.Fatalf("counts = updated %d failed %d, want 0/1", res.Updated, res.Failed)
	}
	if res.Reports[0].Err == nil || !strings.Contains(res.Reports[0].Err.Error(), "item_panels") {
		t.Errorf("error = %v, want item_panels length mismatch", res.Reports[0].Err)
	}
	if len(store.updates) != 0 {
		t.Errorf("updates = %d, want 0 on malformed response", len(store.updates))
	}
}

func TestBackfillPanelsRecordsStoreUpdateFailure(t *testing.T) {
	store := &fakeBackfillStore{
		reports: []pipeline.BackfillReport{
			{Sha256: strings.Repeat("a", 64), ReportType: "生化", Status: "success",
				Items: []pipeline.BackfillItem{{ID: 1, Name: "肌酐", Panel: "other"}}},
		},
		updateErr: errors.New("db down"),
	}
	classifier := &fakePanelClassifier{fn: func(string, []pipeline.ClassifyItem) ([]string, []string, error) {
		return []string{"kidney"}, []string{"kidney"}, nil
	}}

	res, err := pipeline.BackfillPanels(context.Background(), store, classifier, pipeline.BackfillOptions{})
	if err != nil {
		t.Fatalf("BackfillPanels: %v", err)
	}
	if res.Updated != 0 || res.Failed != 1 {
		t.Fatalf("counts = updated %d failed %d, want 0/1", res.Updated, res.Failed)
	}
	if res.Reports[0].Err == nil || !strings.Contains(res.Reports[0].Err.Error(), "update panels") ||
		!strings.Contains(res.Reports[0].Err.Error(), "db down") {
		t.Errorf("error = %v, want update panels / db down", res.Reports[0].Err)
	}
}

func TestBackfillPanelsRejectsInvalidClassifyPanels(t *testing.T) {
	cases := []struct {
		name       string
		panels     []string
		itemPanels []string
		want       string
	}{
		{name: "no report panels", panels: []string{}, itemPanels: []string{"kidney"}, want: "no report panels"},
		{name: "empty report panel", panels: []string{""}, itemPanels: []string{"kidney"}, want: "empty report panel"},
		{name: "empty item panel", panels: []string{"kidney"}, itemPanels: []string{""}, want: "empty item_panel"},
		{name: "report/item drift", panels: []string{"kidney"}, itemPanels: []string{"liver"}, want: "do not match"},
		{name: "duplicate report panel", panels: []string{"kidney", "kidney"}, itemPanels: []string{"kidney"}, want: "duplicate report panel"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			store := &fakeBackfillStore{reports: []pipeline.BackfillReport{
				{Sha256: strings.Repeat("a", 64), ReportType: "生化", Status: "success",
					Items: []pipeline.BackfillItem{{ID: 1, Name: "肌酐", Panel: "other"}}},
			}}
			classifier := &fakePanelClassifier{fn: func(string, []pipeline.ClassifyItem) ([]string, []string, error) {
				return tc.panels, tc.itemPanels, nil
			}}

			res, err := pipeline.BackfillPanels(context.Background(), store, classifier, pipeline.BackfillOptions{})
			if err != nil {
				t.Fatalf("BackfillPanels: %v", err)
			}
			if res.Failed != 1 || res.Updated != 0 || len(store.updates) != 0 {
				t.Fatalf("counts = updated %d failed %d writes %d, want 0/1/0", res.Updated, res.Failed, len(store.updates))
			}
			if res.Reports[0].Err == nil || !strings.Contains(res.Reports[0].Err.Error(), tc.want) {
				t.Errorf("error = %v, want %q", res.Reports[0].Err, tc.want)
			}
		})
	}
}

func TestBackfillPanelsZeroItemReportRules(t *testing.T) {
	cases := []struct {
		name       string
		reportType string
		panels     []string
		wantPanels []string
		wantErr    string
	}{
		{name: "imaging", reportType: "影像", panels: []string{"imaging"}, wantPanels: []string{"imaging"}},
		{name: "non-imaging other", reportType: "生化", panels: []string{"other"}, wantPanels: []string{"other"}},
		{name: "empty report misclassified", reportType: "生化", panels: []string{"liver"}, wantErr: "zero-item"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			store := &fakeBackfillStore{reports: []pipeline.BackfillReport{
				{Sha256: strings.Repeat("a", 64), ReportType: tc.reportType, Status: "success"},
			}}
			classifier := &fakePanelClassifier{fn: func(string, []pipeline.ClassifyItem) ([]string, []string, error) {
				return tc.panels, []string{}, nil
			}}

			res, err := pipeline.BackfillPanels(context.Background(), store, classifier, pipeline.BackfillOptions{})
			if err != nil {
				t.Fatalf("BackfillPanels: %v", err)
			}
			if tc.wantErr != "" {
				if res.Failed != 1 || res.Updated != 0 || len(store.updates) != 0 {
					t.Fatalf("counts = updated %d failed %d writes %d, want 0/1/0", res.Updated, res.Failed, len(store.updates))
				}
				if res.Reports[0].Err == nil || !strings.Contains(res.Reports[0].Err.Error(), tc.wantErr) {
					t.Errorf("error = %v, want %q", res.Reports[0].Err, tc.wantErr)
				}
				return
			}
			if res.Updated != 1 || len(store.updates) != 1 {
				t.Fatalf("counts = updated %d writes %d, want 1/1", res.Updated, len(store.updates))
			}
			if !reflect.DeepEqual(store.updates[0].panels, tc.wantPanels) {
				t.Errorf("panels = %v, want %v", store.updates[0].panels, tc.wantPanels)
			}
		})
	}
}

func TestBackfillPanelsDryRunWritesNothing(t *testing.T) {
	store := &fakeBackfillStore{reports: []pipeline.BackfillReport{
		{Sha256: strings.Repeat("a", 64), ReportType: "生化", Status: "success",
			Items: []pipeline.BackfillItem{{ID: 1, Name: "肌酐", Panel: "other"}}},
	}}
	classifier := &fakePanelClassifier{fn: func(string, []pipeline.ClassifyItem) ([]string, []string, error) {
		return []string{"kidney"}, []string{"kidney"}, nil
	}}

	res, err := pipeline.BackfillPanels(context.Background(), store, classifier, pipeline.BackfillOptions{DryRun: true})
	if err != nil {
		t.Fatalf("BackfillPanels: %v", err)
	}
	if res.Updated != 1 || len(store.updates) != 0 {
		t.Fatalf("updated = %d, writes = %d, want 1 would-update and 0 writes", res.Updated, len(store.updates))
	}
	if len(res.Reports[0].Changes) != 1 {
		t.Errorf("changes = %+v, want planned change", res.Reports[0].Changes)
	}
}

func TestBackfillPanelsIsIdempotent(t *testing.T) {
	store := &fakeBackfillStore{reports: []pipeline.BackfillReport{
		{Sha256: strings.Repeat("a", 64), ReportType: "生化", Status: "success",
			Items: []pipeline.BackfillItem{{ID: 1, Name: "肌酐", Panel: "other"}}},
	}}
	classifier := &fakePanelClassifier{fn: func(string, []pipeline.ClassifyItem) ([]string, []string, error) {
		return []string{"kidney"}, []string{"kidney"}, nil
	}}

	if _, err := pipeline.BackfillPanels(context.Background(), store, classifier, pipeline.BackfillOptions{}); err != nil {
		t.Fatalf("first BackfillPanels: %v", err)
	}
	store.reports[0].Panels = []string{"kidney"}
	store.reports[0].Items[0].Panel = "kidney"
	store.updates = nil

	res, err := pipeline.BackfillPanels(context.Background(), store, classifier, pipeline.BackfillOptions{})
	if err != nil {
		t.Fatalf("second BackfillPanels: %v", err)
	}
	if res.Updated != 0 || res.Skipped != 1 || len(store.updates) != 0 {
		t.Fatalf("second run = updated %d skipped %d writes %d, want 0/1/0", res.Updated, res.Skipped, len(store.updates))
	}
}

func TestBackfillPanelsAbortsWhenClassifierUnavailable(t *testing.T) {
	store := &fakeBackfillStore{reports: []pipeline.BackfillReport{
		{Sha256: strings.Repeat("a", 64), ReportType: "血常规", Status: "success",
			Items: []pipeline.BackfillItem{{ID: 1, Name: "血红蛋白", Panel: "other"}}},
		{Sha256: strings.Repeat("b", 64), ReportType: "血常规", Status: "success",
			Items: []pipeline.BackfillItem{{ID: 2, Name: "血红蛋白", Panel: "other"}}},
	}}
	classifier := &fakePanelClassifier{fn: func(string, []pipeline.ClassifyItem) ([]string, []string, error) {
		return nil, nil, pipeline.ErrClassifierUnavailable
	}}

	res, err := pipeline.BackfillPanels(context.Background(), store, classifier, pipeline.BackfillOptions{})
	if !errors.Is(err, pipeline.ErrClassifierUnavailable) {
		t.Fatalf("err = %v, want ErrClassifierUnavailable", err)
	}
	if res == nil || res.Failed != 1 || len(res.Reports) != 1 || res.Total != 2 {
		t.Fatalf("res = %+v, want one failed of two reports before abort", res)
	}
	if classifier.calls != 1 || len(store.updates) != 0 {
		t.Errorf("calls = %d, updates = %d, want 1/0", classifier.calls, len(store.updates))
	}
}

func TestBackfillPanelsPassesLimit(t *testing.T) {
	store := &fakeBackfillStore{}
	classifier := &fakePanelClassifier{}
	if _, err := pipeline.BackfillPanels(context.Background(), store, classifier, pipeline.BackfillOptions{Limit: 7}); err != nil {
		t.Fatalf("BackfillPanels: %v", err)
	}
	if store.limit != 7 {
		t.Errorf("limit = %d, want 7", store.limit)
	}
}
