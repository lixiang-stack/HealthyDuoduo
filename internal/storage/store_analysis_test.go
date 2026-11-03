// 分析存储冒烟(llm-analysis 票 04):指标点按成员 + 项目组读出(含 failed)、
// 病历段落读取、快照 JSONB 往返、成员删除级联。需要 HDD_TEST_DSN。
package storage

import (
	"context"
	"os"
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
		(report_sha256, name, panel, value, unit, unit_status, ref_range, flag, low_confidence, raw_text)
		VALUES ($1, '丙氨酸转氨酶测定', 'liver', '83.5', 'U/L', 'verified', '7-40', 'high', false, 'ALT 83.5')`,
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
		points[0].UnitStatus != "verified" || points[0].MemberID != member.MemberID {
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
		Entry: analysis.EntryPanel, WindowMonths: 12,
		Now: time.Date(2026, 11, 3, 12, 0, 0, 0, time.UTC),
	}
	facts := analysis.BuildFacts(req, points, notes)
	version, err := st.DataVersion(ctx, member.MemberID)
	if err != nil {
		t.Fatalf("data version: %v", err)
	}
	snapshot := analysis.BuildSnapshot(req, facts)
	snapshot.DataVersion = version
	record := analysis.Record{
		MemberID: member.MemberID, Panel: "liver", Entry: analysis.EntryPanel,
		WindowMonths: 12, Status: analysis.StatusSuccess, Facts: facts,
		Snapshot: snapshot,
		Sources:  []analysis.Source{},
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
		loaded.Snapshot.RuleVersion != analysis.RuleVersion || loaded.Snapshot.DataVersion == "" {
		t.Fatalf("loaded analysis = %+v", loaded)
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
}
