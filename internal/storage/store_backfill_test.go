// 存量项目组回填存储冒烟(panel-browse 票 05):AllReportsForBackfill 按报告分页、
// 带指标项 id/name/unit/当前 panel;UpdateReportPanels 只改 panel 列,
// 核对状态 / 修正对 / 值 / 单位 / 参考范围与报告状态全部保持不变。
// 需要真库:设 HDD_TEST_DSN 后运行,未设整体 Skip(与 store_reports_test 同约定;
// 用一次性独立容器,勿与其它 smoke 共用库):
// docker run --name hdd-backfill-smoke-pg -e POSTGRES_PASSWORD=smoke -p 55436:5432 postgres:17-alpine
package storage

import (
	"context"
	"os"
	"reflect"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"healthyduoduo/internal/pipeline"
)

func TestBackfillStoreSmoke(t *testing.T) {
	dsn := os.Getenv("HDD_TEST_DSN")
	if dsn == "" {
		t.Skip("HDD_TEST_DSN not set: backfill store smoke runs against a disposable PG container")
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

	// 允许在同一一次性容器内重复运行:先清掉本测试自己的种子(报告级联 items / 修正对)。
	if _, err := pool.Exec(ctx, `DELETE FROM reports WHERE image_sha256 IN ('bf-a','bf-b','bf-c','bf-0','bf-d');
		DELETE FROM ocr_results WHERE image_sha256 IN ('bf-a','bf-b','bf-c','bf-0','bf-d');
		DELETE FROM images WHERE sha256 IN ('bf-a','bf-b','bf-c','bf-0','bf-d');
		DELETE FROM members WHERE name='回填测试成员'`); err != nil {
		t.Fatalf("clean previous seed: %v", err)
	}

	var memberID int64
	if err := pool.QueryRow(ctx, `INSERT INTO members (name, is_child) VALUES ('回填测试成员', true) RETURNING id`).
		Scan(&memberID); err != nil {
		t.Fatalf("seed member: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO images (sha256, original_filename) VALUES
			('bf-a','a.jpg'),('bf-b','b.jpg'),('bf-c','c.jpg'),('bf-0','0.jpg'),('bf-d','d.jpg');
		INSERT INTO ocr_results (image_sha256, engine_output) VALUES
			('bf-a','{}'),('bf-b','{}'),('bf-c','{}'),('bf-0','{}'),('bf-d','{}')`); err != nil {
		t.Fatalf("seed images/ocr: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO reports
			(image_sha256, ocr_result_id, report_type, report_date, status, member_id, fail_reason, panels)
		VALUES
			('bf-a',(SELECT id FROM ocr_results WHERE image_sha256='bf-a'),'生化','2026-10-03','partial',$1,NULL,'{}'),
			('bf-b',(SELECT id FROM ocr_results WHERE image_sha256='bf-b'),'影像','2026-10-02','success',$1,NULL,'{}'),
			('bf-c',(SELECT id FROM ocr_results WHERE image_sha256='bf-c'),'临检','2026-10-01','success',$1,NULL,ARRAY['cbc']),
			('bf-0',(SELECT id FROM ocr_results WHERE image_sha256='bf-0'),'其他专项','2026-10-03','failed',$1,'no metric items recognized','{}'),
			('bf-d',(SELECT id FROM ocr_results WHERE image_sha256='bf-d'),'其他专项',NULL,'failed',$1,'no metric items recognized','{}')`,
		memberID); err != nil {
		t.Fatalf("seed reports: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO report_items
			(report_sha256, name, panel, value, unit, ref_range, method, flag, flag_reason, low_confidence, raw_text, review_status)
		VALUES
			('bf-a','丙氨酸转氨酶','other','55','U/L','0-40','速率法','high',NULL,false,'丙氨酸转氨酶 55 U/L 0-40','corrected'),
			('bf-a','肌酐','other','41','μmol/L','44-97','酶法','low',NULL,true,'肌酐 41 μmol/L 44-97','confirmed'),
			('bf-c','血红蛋白','cbc','128','g/L','115-150','比色法','normal',NULL,false,'血红蛋白 128 g/L 115-150','unreviewed')`); err != nil {
		t.Fatalf("seed report_items: %v", err)
	}
	var altID, creaID, cbcID int64
	if err := pool.QueryRow(ctx, `SELECT id FROM report_items WHERE report_sha256='bf-a' AND name='丙氨酸转氨酶'`).Scan(&altID); err != nil {
		t.Fatalf("alt id: %v", err)
	}
	if err := pool.QueryRow(ctx, `SELECT id FROM report_items WHERE report_sha256='bf-a' AND name='肌酐'`).Scan(&creaID); err != nil {
		t.Fatalf("creatinine id: %v", err)
	}
	if err := pool.QueryRow(ctx, `SELECT id FROM report_items WHERE report_sha256='bf-c' AND name='血红蛋白'`).Scan(&cbcID); err != nil {
		t.Fatalf("cbc id: %v", err)
	}
	// 修正对:回填不得删除 / 改写。
	if _, err := pool.Exec(ctx, `INSERT INTO correction_pairs
			(item_id, before_name, before_value, before_unit, before_ref_range,
			 after_name, after_value, after_unit, after_ref_range, raw_text, ocr_line_numbers)
		VALUES ($1,'丙氨酸转氨酶','50','U/L','0-40','丙氨酸转氨酶','55','U/L','0-40',
			'丙氨酸转氨酶 55 U/L 0-40', ARRAY[0,1,2])`, altID); err != nil {
		t.Fatalf("seed correction_pair: %v", err)
	}

	// 顺序:报告日期降序;limit 按报告数,不按 join 后的指标项行数。
	all, err := st.AllReportsForBackfill(ctx, 0)
	if err != nil {
		t.Fatalf("AllReportsForBackfill: %v", err)
	}
	if len(all) != 5 {
		t.Fatalf("reports = %d, want 5", len(all))
	}
	// 日期降序;同日按 sha 升序;dateless 沉底。
	wantOrder := []string{"bf-0", "bf-a", "bf-b", "bf-c", "bf-d"}
	for i, want := range wantOrder {
		if all[i].Sha256 != want {
			t.Fatalf("order = %v, want %v", shaSeq(all), wantOrder)
		}
	}
	a := all[1]
	if a.ReportType != "生化" || a.Status != "partial" {
		t.Errorf("bf-a header = %+v", a)
	}
	if !reflect.DeepEqual(a.Panels, []string{}) {
		t.Errorf("bf-a panels = %v, want empty (未回填)", a.Panels)
	}
	if len(a.Items) != 2 {
		t.Fatalf("bf-a items = %+v, want 2", a.Items)
	}
	if a.Items[0].ID != altID || a.Items[0].Name != "丙氨酸转氨酶" || a.Items[0].Unit == nil || *a.Items[0].Unit != "U/L" || a.Items[0].Panel != "other" {
		t.Errorf("bf-a item[0] = %+v", a.Items[0])
	}
	if a.Items[1].ID != creaID || a.Items[1].Name != "肌酐" {
		t.Errorf("bf-a item[1] = %+v", a.Items[1])
	}
	if len(all[2].Items) != 0 || all[2].ReportType != "影像" {
		t.Errorf("bf-b = %+v, want imaging with no items", all[2])
	}
	limited, err := st.AllReportsForBackfill(ctx, 2)
	if err != nil {
		t.Fatalf("AllReportsForBackfill limit: %v", err)
	}
	if len(limited) != 2 || limited[0].Sha256 != "bf-0" || limited[1].Sha256 != "bf-a" {
		t.Fatalf("limit 2 = %v, want first two reports (not rows)", shaSeq(limited))
	}

	// 回填:只写 panel 列。
	if err := st.UpdateReportPanels(ctx, "bf-a", []string{"liver", "kidney"}, map[int64]string{
		altID:  "liver",
		creaID: "kidney",
	}); err != nil {
		t.Fatalf("UpdateReportPanels: %v", err)
	}
	var panels []string
	var status string
	var failReason *string
	var gotMemberID *int64
	if err := pool.QueryRow(ctx, `SELECT panels, status, fail_reason, member_id FROM reports WHERE image_sha256='bf-a'`).
		Scan(&panels, &status, &failReason, &gotMemberID); err != nil {
		t.Fatalf("read report: %v", err)
	}
	if !reflect.DeepEqual(panels, []string{"liver", "kidney"}) || status != "partial" || failReason != nil ||
		gotMemberID == nil || *gotMemberID != memberID {
		t.Errorf("report after update = panels %v status %q fail_reason %v member %v", panels, status, failReason, gotMemberID)
	}
	rows, err := pool.Query(ctx, `SELECT name, panel, value, unit, ref_range, method, flag, low_confidence, raw_text, review_status
		FROM report_items WHERE report_sha256='bf-a' ORDER BY id`)
	if err != nil {
		t.Fatalf("read items: %v", err)
	}
	var gotItems []pipeline.BackfillItem
	var gotValue, gotUnit, gotRef, gotMethod, gotFlag, gotRaw, gotReview []string
	var gotLow []bool
	for rows.Next() {
		var item pipeline.BackfillItem
		var value, unit, ref, method, flag, raw, review string
		var low bool
		if err := rows.Scan(&item.Name, &item.Panel, &value, &unit, &ref, &method, &flag, &low, &raw, &review); err != nil {
			t.Fatalf("scan item: %v", err)
		}
		gotItems = append(gotItems, item)
		gotValue = append(gotValue, value)
		gotUnit = append(gotUnit, unit)
		gotRef = append(gotRef, ref)
		gotMethod = append(gotMethod, method)
		gotFlag = append(gotFlag, flag)
		gotLow = append(gotLow, low)
		gotRaw = append(gotRaw, raw)
		gotReview = append(gotReview, review)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate items: %v", err)
	}
	wantItems := []pipeline.BackfillItem{
		{Name: "丙氨酸转氨酶", Panel: "liver"},
		{Name: "肌酐", Panel: "kidney"},
	}
	if !reflect.DeepEqual(gotItems, wantItems) {
		t.Errorf("items = %+v, want panels updated only", gotItems)
	}
	wantValue := []string{"55", "41"}
	wantUnit := []string{"U/L", "μmol/L"}
	wantRef := []string{"0-40", "44-97"}
	wantMethod := []string{"速率法", "酶法"}
	wantFlag := []string{"high", "low"}
	wantLow := []bool{false, true}
	wantRaw := []string{"丙氨酸转氨酶 55 U/L 0-40", "肌酐 41 μmol/L 44-97"}
	wantReview := []string{"corrected", "confirmed"}
	for label, tc := range map[string]struct {
		got  any
		want any
	}{
		"value":          {gotValue, wantValue},
		"unit":           {gotUnit, wantUnit},
		"ref_range":      {gotRef, wantRef},
		"method":         {gotMethod, wantMethod},
		"flag":           {gotFlag, wantFlag},
		"low_confidence": {gotLow, wantLow},
		"raw_text":       {gotRaw, wantRaw},
		"review_status":  {gotReview, wantReview},
	} {
		if !reflect.DeepEqual(tc.got, tc.want) {
			t.Errorf("%s changed by backfill: got %v, want %v", label, tc.got, tc.want)
		}
	}
	var pairs int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM correction_pairs WHERE item_id=$1`, altID).Scan(&pairs); err != nil {
		t.Fatalf("count correction pairs: %v", err)
	}
	if pairs != 1 {
		t.Errorf("correction pairs = %d, want 1", pairs)
	}
	var beforeValue, afterValue string
	if err := pool.QueryRow(ctx, `SELECT before_value, after_value FROM correction_pairs WHERE item_id=$1`, altID).
		Scan(&beforeValue, &afterValue); err != nil {
		t.Fatalf("read correction pair: %v", err)
	}
	if beforeValue != "50" || afterValue != "55" {
		t.Errorf("correction pair = %s → %s, want 50 → 55 unchanged", beforeValue, afterValue)
	}

	// 幂等:重复回填同样的值不改其它列。
	if err := st.UpdateReportPanels(ctx, "bf-a", []string{"liver", "kidney"}, map[int64]string{
		altID:  "liver",
		creaID: "kidney",
	}); err != nil {
		t.Fatalf("second UpdateReportPanels: %v", err)
	}
	if err := pool.QueryRow(ctx, `SELECT panels, status FROM reports WHERE image_sha256='bf-a'`).Scan(&panels, &status); err != nil {
		t.Fatalf("read report after second update: %v", err)
	}
	if !reflect.DeepEqual(panels, []string{"liver", "kidney"}) || status != "partial" {
		t.Errorf("second update = panels %v status %q", panels, status)
	}

	// 影像成功(零指标项)回填 reports.panels;空 item map 合法。
	if err := st.UpdateReportPanels(ctx, "bf-b", []string{"imaging"}, map[int64]string{}); err != nil {
		t.Fatalf("UpdateReportPanels imaging: %v", err)
	}
	if err := pool.QueryRow(ctx, `SELECT panels FROM reports WHERE image_sha256='bf-b'`).Scan(&panels); err != nil {
		t.Fatalf("read imaging panels: %v", err)
	}
	if !reflect.DeepEqual(panels, []string{"imaging"}) {
		t.Errorf("imaging panels = %v, want [imaging]", panels)
	}

	// 跨报告 item id 必须拒绝,且不落下报告级 panel。
	if err := st.UpdateReportPanels(ctx, "bf-a", []string{"cbc"}, map[int64]string{cbcID: "cbc"}); err == nil {
		t.Error("cross-report item id accepted, want error")
	}
	if err := pool.QueryRow(ctx, `SELECT panels FROM reports WHERE image_sha256='bf-a'`).Scan(&panels); err != nil {
		t.Fatalf("read report after rejected update: %v", err)
	}
	if !reflect.DeepEqual(panels, []string{"liver", "kidney"}) {
		t.Errorf("report panels after rejected update = %v, want unchanged", panels)
	}
}

func shaSeq(rows []pipeline.BackfillReport) []string {
	out := make([]string, 0, len(rows))
	for _, row := range rows {
		out = append(out, row.Sha256)
	}
	return out
}
