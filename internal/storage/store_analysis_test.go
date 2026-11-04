// 分析存储冒烟(llm-analysis 票 04):指标点按成员 + 项目组读出(含 failed)、
// 病历段落读取、快照 JSONB 往返、成员删除级联。需要 HDD_TEST_DSN。
package storage

import (
	"context"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"healthyduoduo/internal/analysis"
	"healthyduoduo/internal/pipeline"
)

func TestAnalysisStoreSmoke(t *testing.T) {
	dsn := os.Getenv("HDD_TEST_DSN")
	if dsn == "" {
		t.Skip("HDD_TEST_DSN not set: analysis store smoke runs against a disposable PG container")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer pool.Close()
	if err := Migrate(ctx, dsn); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	st := NewStore(pool)
	defer st.Close()

	member, err := st.CreateMember(ctx, pipeline.MemberProfile{
		Name: "小虎", Sex: pipeline.SexMale, BirthDate: "2020-05-20",
	})
	if err != nil {
		t.Fatalf("create member: %v", err)
	}

	height, weight := 118.5, 22.4
	if _, err := st.CreateGrowth(ctx, member.MemberID, pipeline.GrowthProfile{
		MeasuredOn: "2026-10-01", HeightCm: &height, WeightKg: &weight,
		Source: pipeline.GrowthSourceMeasured,
	}); err != nil {
		t.Fatalf("create growth: %v", err)
	}
	growth, err := st.LatestGrowth(ctx, member.MemberID)
	if err != nil || growth == nil || growth.MeasuredOn != "2026-10-01" ||
		growth.HeightCm == nil || *growth.HeightCm != height {
		t.Fatalf("latest growth = %+v, %v", growth, err)
	}

	const reportID = "analysis-report-1"
	const noteID = "analysis-note-1"
	if _, err := pool.Exec(ctx, `INSERT INTO images (sha256, original_filename)
		VALUES ($1, 'lft.jpg'), ($2, 'note.jpg')`, reportID, noteID); err != nil {
		t.Fatalf("seed images: %v", err)
	}
	var reportOCR, noteOCR int64
	if err := pool.QueryRow(ctx, `INSERT INTO ocr_results (image_sha256, engine_output)
		VALUES ($1, '{}') RETURNING id`, reportID).Scan(&reportOCR); err != nil {
		t.Fatalf("seed report ocr: %v", err)
	}
	if err := pool.QueryRow(ctx, `INSERT INTO ocr_results (image_sha256, engine_output)
		VALUES ($1, '{}') RETURNING id`, noteID).Scan(&noteOCR); err != nil {
		t.Fatalf("seed note ocr: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO reports
		(image_sha256, ocr_result_id, report_type, report_date, status, member_id, panels)
		VALUES ($1, $2, '生化', '2026-10-01', 'success', $3, '{liver}')`,
		reportID, reportOCR, member.MemberID); err != nil {
		t.Fatalf("seed report: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO reports
		(image_sha256, ocr_result_id, report_type, report_date, status, member_id, panels, sections)
		VALUES ($1, $2, 'clinical_note', '2026-09-20', 'success', $3, '{clinical_note}',
		        '[{"kind":"diagnosis","text":"缺铁性贫血","ocr_line_numbers":[0]},
		          {"kind":"chief_complaint","text":"最近不爱吃饭","ocr_line_numbers":[1]}]'::jsonb)`,
		noteID, noteOCR, member.MemberID); err != nil {
		t.Fatalf("seed note: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO report_items
		(report_sha256, name, panel, analyte, value, unit, unit_status, ref_range, flag, low_confidence, raw_text)
		VALUES ($1, '丙氨酸转氨酶测定', 'liver', 'alanine_aminotransferase', '83.5', 'U/L', 'verified', '7-40', 'high', false, 'ALT 83.5')`,
		reportID); err != nil {
		t.Fatalf("seed item: %v", err)
	}
	// failed 报告也读出,由领域层排除并给原因。
	const failedID = "analysis-report-failed"
	if _, err := pool.Exec(ctx, `INSERT INTO images (sha256, original_filename) VALUES ($1, 'bad.jpg')`, failedID); err != nil {
		t.Fatalf("seed failed image: %v", err)
	}
	var failedOCR int64
	if err := pool.QueryRow(ctx, `INSERT INTO ocr_results (image_sha256, engine_output)
		VALUES ($1, '{}') RETURNING id`, failedID).Scan(&failedOCR); err != nil {
		t.Fatalf("seed failed ocr: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO reports
		(image_sha256, ocr_result_id, report_type, report_date, status, member_id, panels)
		VALUES ($1, $2, '生化', '2026-01-01', 'failed', $3, '{liver}')`,
		failedID, failedOCR, member.MemberID); err != nil {
		t.Fatalf("seed failed report: %v", err)
	}

	panel, err := st.IndicatorPanel(ctx, member.MemberID, "丙氨酸转氨酶测定")
	if err != nil || panel != "liver" {
		t.Fatalf("indicator panel = (%q, %v), want liver", panel, err)
	}
	if panel, err := st.IndicatorPanel(ctx, member.MemberID, "不存在"); err != nil || panel != "" {
		t.Fatalf("missing indicator panel = (%q, %v), want empty", panel, err)
	}

	points, err := st.AnalysisPoints(ctx, member.MemberID, "liver")
	if err != nil {
		t.Fatalf("analysis points: %v", err)
	}
	if len(points) != 1 || points[0].ReportID != reportID || points[0].Flag != "high" ||
		points[0].UnitStatus != "verified" || points[0].MemberID != member.MemberID ||
		points[0].Analyte != "alanine_aminotransferase" {
		t.Fatalf("analysis points = %+v", points)
	}

	notes, err := st.ClinicalNotes(ctx, member.MemberID)
	if err != nil {
		t.Fatalf("clinical notes: %v", err)
	}
	if len(notes) != 1 || notes[0].ReportID != noteID || len(notes[0].Sections) != 2 ||
		notes[0].Sections[0].Kind != "diagnosis" {
		t.Fatalf("clinical notes = %+v", notes)
	}

	req := analysis.Request{
		MemberID: member.MemberID, MemberName: member.Name, Panel: "liver",
		MemberSex: member.Sex, MemberBirthDate: member.BirthDate,
		Entry: analysis.EntryPanel, WindowMonths: 12,
		Now: time.Date(2026, 11, 3, 12, 0, 0, 0, time.UTC),
	}
	facts := analysis.BuildFacts(req, points, notes)
	version, err := st.DataVersion(ctx, member.MemberID)
	if err != nil {
		t.Fatalf("data version: %v", err)
	}
	framework := analysis.BuildFramework(req, facts, growth)
	snapshot := analysis.BuildSnapshot(req, facts)
	snapshot.DataVersion = version
	record := analysis.Record{
		MemberID: member.MemberID, Panel: "liver", Entry: analysis.EntryPanel,
		WindowMonths: 12, Status: analysis.StatusSuccess, Facts: facts,
		Framework: framework, Snapshot: snapshot,
		Sources: analysis.FrameworkSources(framework),
	}
	saved, err := st.SaveAnalysis(ctx, record)
	if err != nil {
		t.Fatalf("save analysis: %v", err)
	}
	if saved.AnalysisID == 0 || saved.CreatedAt.IsZero() {
		t.Fatalf("saved analysis = %+v, want id/created_at", saved)
	}
	loaded, err := st.AnalysisByID(ctx, saved.AnalysisID)
	if err != nil {
		t.Fatalf("load analysis: %v", err)
	}
	if loaded == nil || loaded.Facts.Panel != "liver" || len(loaded.Facts.History) != 1 ||
		len(loaded.Snapshot.Points) != 1 || len(loaded.Facts.Notes) != 1 ||
		loaded.Snapshot.RuleVersion != analysis.RuleVersion || loaded.Snapshot.DataVersion == "" ||
		len(loaded.Framework.Labels) == 0 || loaded.Snapshot.MemberSex != member.Sex ||
		loaded.Snapshot.MemberBirthDate != member.BirthDate {
		t.Fatalf("loaded analysis = %+v", loaded)
	}

	explanation := &analysis.Explanation{Sections: []analysis.ExplanationSection{{
		EntryID: "liver-hepatocellular-mild", EvidenceIDs: []string{"ev-1"},
		BridgeText: "先看本次数值。",
	}}}
	sources := analysis.MergeSources(analysis.FrameworkSources(framework), analysis.ExplanationSources(explanation))
	if err := st.UpdateAnalysisExplanation(ctx, saved.AnalysisID, explanation, "", sources); err != nil {
		t.Fatalf("update explanation: %v", err)
	}
	loaded, err = st.AnalysisByID(ctx, saved.AnalysisID)
	if err != nil || loaded == nil || loaded.Explanation == nil ||
		len(loaded.Explanation.Sections) != 1 || len(loaded.Sources) != len(sources) {
		t.Fatalf("loaded explanation = %+v, %v", loaded, err)
	}

	used, allowed, err := st.ConsumeLLMQuota(ctx, saved.AnalysisID, 1)
	if err != nil || !allowed || used != 1 {
		t.Fatalf("first quota = (%d, %v, %v)", used, allowed, err)
	}
	used, allowed, err = st.ConsumeLLMQuota(ctx, saved.AnalysisID, 1)
	if err != nil || allowed || used != 1 {
		t.Fatalf("second quota = (%d, %v, %v)", used, allowed, err)
	}
	if usedToday, err := st.LLMUsedToday(ctx); err != nil || usedToday != 1 {
		t.Fatalf("used today = (%d, %v)", usedToday, err)
	}
	// 并发占用同一配额:advisory lock 必须让 limit=3 时只再放行 2 次。
	var wg sync.WaitGroup
	var mu sync.Mutex
	allowedCount := 0
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, allowed, err := st.ConsumeLLMQuota(ctx, saved.AnalysisID, 3)
			if err != nil {
				t.Errorf("concurrent quota: %v", err)
				return
			}
			if allowed {
				mu.Lock()
				allowedCount++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	if allowedCount != 2 {
		t.Fatalf("concurrent allowed = %d, want 2 (limit 3, already used 1)", allowedCount)
	}
	if usedToday, err := st.LLMUsedToday(ctx); err != nil || usedToday != 3 {
		t.Fatalf("used today after concurrency = (%d, %v), want 3", usedToday, err)
	}

	if err := st.RecordAnalysisEvent(ctx, saved.AnalysisID, analysis.EventQuotaExhausted, "test"); err != nil {
		t.Fatalf("record event: %v", err)
	}
	feedback, err := st.RecordFeedback(ctx, analysis.Feedback{
		AnalysisID: saved.AnalysisID, Reason: "解释和数值对不上",
	})
	if err != nil || feedback.FeedbackID == 0 || feedback.CreatedAt.IsZero() {
		t.Fatalf("feedback = %+v, %v", feedback, err)
	}
	metrics, err := st.AnalysisMetrics(ctx, time.Now().Add(-24*time.Hour), time.Now().Add(24*time.Hour))
	if err != nil || metrics.Analyses != 1 || metrics.Feedback != 1 || metrics.Degradations != 1 ||
		metrics.DegradationKinds[analysis.EventQuotaExhausted] != 1 {
		t.Fatalf("metrics = %+v, %v", metrics, err)
	}
	before, err := st.DataVersion(ctx, member.MemberID)
	if err != nil {
		t.Fatalf("data version: %v", err)
	}
	if _, err := pool.Exec(ctx, `UPDATE report_items SET review_status = 'corrected'
		WHERE report_sha256 = $1`, reportID); err != nil {
		t.Fatalf("update review status: %v", err)
	}
	after, err := st.DataVersion(ctx, member.MemberID)
	if err != nil {
		t.Fatalf("data version after review: %v", err)
	}
	if before == after {
		t.Fatal("data version unchanged after review status change")
	}

	// 成员删除级联删除分析快照。
	if deleted, err := st.DeleteMember(ctx, member.MemberID); err != nil || !deleted {
		t.Fatalf("delete member = (%v, %v)", deleted, err)
	}
	var remaining int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM analyses`).Scan(&remaining); err != nil {
		t.Fatal(err)
	}
	if remaining != 0 {
		t.Errorf("analyses after member delete = %d, want 0", remaining)
	}
	for table, want := range map[string]int{
		"analysis_feedback": 0, "analysis_events": 0, "analysis_llm_calls": 0,
	} {
		var count int
		if err := pool.QueryRow(ctx, `SELECT count(*) FROM `+table).Scan(&count); err != nil {
			t.Fatal(err)
		}
		if count != want {
			t.Errorf("%s after member delete = %d, want %d", table, count, want)
		}
	}
}
