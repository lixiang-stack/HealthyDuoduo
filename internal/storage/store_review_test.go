// 核对落库冒烟(viz-and-client 票 09):真库上的核对事务语义——确认/修正的生效值、
// flag 服务端重算、修正对 append(before→after + raw_text + OCR 行号)、
// partial → success 单调派生、缺失口径(nil, nil)。需要真库:设 HDD_TEST_DSN 后运行,
// 未设整体 Skip(与 store_reports_test 同约定;共享栈实时验证由 e2e 冒烟覆盖)。
package storage

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"healthyduoduo/internal/pipeline"
)

func TestReviewItemSmoke(t *testing.T) {
	dsn := os.Getenv("HDD_TEST_DSN")
	if dsn == "" {
		t.Skip("HDD_TEST_DSN not set: review smoke runs against a disposable PG container")
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

	// 种子:一图一报告三项——两项未核对低置信(部分产出,待核对计数 2)、一项高置信;
	// 首项带行链接下标、末项行号为 NULL(存量形态:修正对需落空数组不炸)。
	if _, err := pool.Exec(ctx, `INSERT INTO images (sha256, original_filename) VALUES ('rev-a','a.jpg')`); err != nil {
		t.Fatalf("seed image: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO ocr_results (image_sha256, engine_output) VALUES ('rev-a','{}')`); err != nil {
		t.Fatalf("seed ocr: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO reports (image_sha256, ocr_result_id, report_type, report_date, status)
		VALUES ('rev-a',(SELECT id FROM ocr_results WHERE image_sha256='rev-a'),'血常规','2026-10-02','partial')`); err != nil {
		t.Fatalf("seed report: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO report_items (report_sha256, name, value, unit, ref_range, flag, low_confidence, raw_text, review_status, ocr_line_numbers) VALUES
		('rev-a','单核细胞比率','10.8','%','3-10','high',true,'单核细胞百分比  10.8  ↑ %  3-10','unreviewed',ARRAY[1,2,3,4]),
		('rev-a','平均血红蛋白浓度','300','g/L','316-354','low',true,'平均血红蛋白浓度  300  g/L  316-354','unreviewed',NULL),
		('rev-a','血红蛋白','128','g/L','115-150','normal',false,'血红蛋白  128  g/L  115-150','unreviewed',NULL)`); err != nil {
		t.Fatalf("seed items: %v", err)
	}
	items, err := st.ItemsWithReviewStatus(ctx, "rev-a")
	if err != nil || len(items) != 3 {
		t.Fatalf("seeded items = %+v, %v", items, err)
	}
	lowA, lowB, normal := items[0], items[1], items[2]
	// 标准层展示/核验字段:未核验单位按 spec §3.2 置 unit=null + 原文保留;
	// 修正纸面单位后必须清空,避免 App 继续显示旧 unit_display。
	if _, err := pool.Exec(ctx, `UPDATE report_items SET unit=NULL, unit_display='g/L', unit_status='unverified' WHERE id=$1`, normal.ItemID); err != nil {
		t.Fatalf("seed standard fields: %v", err)
	}

	// 确认低置信甲:报告仍 partial、计数 1;无修正对。
	out, err := st.ReviewItem(ctx, "rev-a", lowA.ItemID, pipeline.ReviewAction{Confirm: true})
	if err != nil || out == nil {
		t.Fatalf("confirm lowA = %+v, %v", out, err)
	}
	if out.Status != "partial" || out.UnreviewedLowConfidenceCount != 1 {
		t.Errorf("confirm lowA outcome = %+v, want partial/1", out)
	}
	if out.Item.ReviewStatus != pipeline.ReviewStatusConfirmed {
		t.Errorf("confirmed item = %+v", out.Item)
	}
	if n := countCorrectionPairs(t, ctx, pool, "rev-a"); n != 0 {
		t.Errorf("pairs after confirm = %d, want 0(确认不产生记录)", n)
	}

	// 确认最后一项低置信:partial → success 单向派生,计数归零。
	out, err = st.ReviewItem(ctx, "rev-a", lowB.ItemID, pipeline.ReviewAction{Confirm: true})
	if err != nil || out == nil {
		t.Fatalf("confirm lowB = %+v, %v", out, err)
	}
	if out.Status != "success" || out.UnreviewedLowConfidenceCount != 0 {
		t.Errorf("confirm lowB outcome = %+v, want success/0", out)
	}

	// 修正高置信项(免核对 ≠ 不可修改):生效值 + 纸面单位 + flag 服务端重算 + 修正对内容。
	newValue := "300"
	newUnit := "g/L"
	out, err = st.ReviewItem(ctx, "rev-a", normal.ItemID, pipeline.ReviewAction{Value: &newValue, Unit: &newUnit})
	if err != nil || out == nil {
		t.Fatalf("correct = %+v, %v", out, err)
	}
	if *out.Item.Value != "300" || out.Item.Flag != "high" || out.Item.ReviewStatus != pipeline.ReviewStatusCorrected {
		t.Errorf("corrected outcome item = %+v, want 300/high/corrected", out.Item)
	}
	if out.Item.UnitDisplay != nil || out.Item.UnitStatus != nil {
		t.Errorf("corrected outcome unit meta = %v/%v, want nil(本轮已清空)", out.Item.UnitDisplay, out.Item.UnitStatus)
	}
	if out.Status != "success" || out.UnreviewedLowConfidenceCount != 0 {
		t.Errorf("correct outcome = %+v, want success 恒定", out)
	}
	var dbValue, dbFlag, dbStatus string
	if err := pool.QueryRow(ctx, `SELECT value, flag, review_status FROM report_items WHERE id=$1`, normal.ItemID).
		Scan(&dbValue, &dbFlag, &dbStatus); err != nil {
		t.Fatal(err)
	}
	if dbValue != "300" || dbFlag != "high" || dbStatus != "corrected" {
		t.Errorf("db row = %s/%s/%s, want 300/high/corrected", dbValue, dbFlag, dbStatus)
	}
	var dbDisplay, dbUnitStatus *string
	if err := pool.QueryRow(ctx, `SELECT unit_display, unit_status FROM report_items WHERE id=$1`, normal.ItemID).
		Scan(&dbDisplay, &dbUnitStatus); err != nil {
		t.Fatal(err)
	}
	if dbDisplay != nil || dbUnitStatus != nil {
		t.Errorf("unit_display/unit_status after correction = %v/%v, want NULL(纸面修正后旧展示失效)", dbDisplay, dbUnitStatus)
	}
	pair := lastCorrectionPair(t, ctx, pool, normal.ItemID)
	if pair.beforeValue != "128" || pair.afterValue != "300" || pair.beforeRef != "115-150" || pair.afterRef != "115-150" {
		t.Errorf("pair facts = %+v, want before 128->after 300(ref 不动)", pair)
	}
	if pair.beforeName != "血红蛋白" || pair.afterName != "血红蛋白" {
		t.Errorf("pair names = %q/%q", pair.beforeName, pair.afterName)
	}
	if pair.rawText != "血红蛋白  128  g/L  115-150" {
		t.Errorf("pair raw_text = %q, want 随项原样", pair.rawText)
	}
	if len(pair.lineNumbers) != 0 {
		t.Errorf("pair line numbers = %v, want 空数组(NULL 存量行)", pair.lineNumbers)
	}

	// 重放同一修正(离线补传):生效状态不变,按 ADR-0005 每次修正 append 一条
	// (no-op 对:before = 上次生效值 = after)。
	out, err = st.ReviewItem(ctx, "rev-a", normal.ItemID, pipeline.ReviewAction{Value: &newValue})
	if err != nil || out == nil {
		t.Fatalf("replay correct = %+v, %v", out, err)
	}
	pair = lastCorrectionPair(t, ctx, pool, normal.ItemID)
	if pair.beforeValue != "300" || pair.afterValue != "300" {
		t.Errorf("replay pair = %s -> %s, want no-op(上次生效值)", pair.beforeValue, pair.afterValue)
	}
	if n := countCorrectionPairs(t, ctx, pool, "rev-a"); n != 2 {
		t.Errorf("pairs = %d, want 2(每次修正一条;重放为 no-op 对)", n)
	}
	// 行链接随项:先修正带行号的低置信甲,修正对行号原样携带。
	correctedValue := "5.6"
	if _, err := st.ReviewItem(ctx, "rev-a", lowA.ItemID, pipeline.ReviewAction{Value: &correctedValue}); err != nil {
		t.Fatal(err)
	}
	pair = lastCorrectionPair(t, ctx, pool, lowA.ItemID)
	if len(pair.lineNumbers) != 4 || pair.lineNumbers[0] != 1 || pair.lineNumbers[3] != 4 {
		t.Errorf("pair line numbers = %v, want [1 2 3 4]", pair.lineNumbers)
	}
	if pair.beforeValue != "10.8" || pair.afterValue != "5.6" {
		t.Errorf("pair facts = %s -> %s, want 10.8 -> 5.6", pair.beforeValue, pair.afterValue)
	}
	if out.Status != "success" {
		t.Errorf("status after re-correct = %q, want success 恒定", out.Status)
	}

	// 缺失口径:不存在的报告 / 不存在的指标项 → nil, nil(404 口径,不虚报错误)。
	if missing, err := st.ReviewItem(ctx, "rev-missing", lowA.ItemID, pipeline.ReviewAction{Confirm: true}); err != nil || missing != nil {
		t.Errorf("ReviewItem(missing report) = (%v, %v), want (nil, nil)", missing, err)
	}
	if missing, err := st.ReviewItem(ctx, "rev-a", 999999, pipeline.ReviewAction{Confirm: true}); err != nil || missing != nil {
		t.Errorf("ReviewItem(missing item) = (%v, %v), want (nil, nil)", missing, err)
	}
}

// correctionPairRow 修正对行的读出形状(冒烟断言用)。
type correctionPairRow struct {
	beforeName, beforeValue, beforeRef string
	afterName, afterValue, afterRef    string
	rawText                            string
	lineNumbers                        []int32
}

// lastCorrectionPair 读某指标项最后一条修正对(含 before/after 四项纸面事实)。
func lastCorrectionPair(t *testing.T, ctx context.Context, pool *pgxpool.Pool, itemID int64) correctionPairRow {
	t.Helper()
	var row correctionPairRow
	if err := pool.QueryRow(ctx, `SELECT before_name, before_value, before_ref_range,
			after_name, after_value, after_ref_range, raw_text, ocr_line_numbers
		FROM correction_pairs WHERE item_id=$1 ORDER BY id DESC LIMIT 1`, itemID).Scan(
		&row.beforeName, &row.beforeValue, &row.beforeRef,
		&row.afterName, &row.afterValue, &row.afterRef, &row.rawText, &row.lineNumbers); err != nil {
		t.Fatalf("read correction pair: %v", err)
	}
	return row
}

// countCorrectionPairs 报告下全部指标项的修正对计数。
func countCorrectionPairs(t *testing.T, ctx context.Context, pool *pgxpool.Pool, reportSha string) int {
	t.Helper()
	var n int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM correction_pairs p
		JOIN report_items i ON i.id = p.item_id WHERE i.report_sha256 = $1`, reportSha).Scan(&n); err != nil {
		t.Fatalf("count correction pairs: %v", err)
	}
	return n
}
