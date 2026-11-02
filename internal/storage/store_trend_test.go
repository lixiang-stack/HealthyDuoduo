// 趋势存储冒烟(票 02):真库上的成员隔离、unit 门控、日期升序 / dateless 沉底、
// 同日期按 sha 升序,以及 ref/method/review_status 字段透传。
// 需要真库:设 HDD_TEST_DSN 后运行,未设整体 Skip(与 store_reports_test 同约定;
// CI 默认跳过)。一次性容器示例见 spec §7.1。
package storage

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"healthyduoduo/internal/pipeline"
)

func TestTrendStoreSmoke(t *testing.T) {
	dsn := os.Getenv("HDD_TEST_DSN")
	if dsn == "" {
		t.Skip("HDD_TEST_DSN not set: trend store smoke runs against a disposable PG container")
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

	// 两个成员:趋势查询必须按成员隔离。
	var memberA, memberB int64
	if err := pool.QueryRow(ctx, `INSERT INTO members (name, is_child) VALUES ('趋势A', true) RETURNING id`).Scan(&memberA); err != nil {
		t.Fatalf("seed member A: %v", err)
	}
	if err := pool.QueryRow(ctx, `INSERT INTO members (name, is_child) VALUES ('趋势B', false) RETURNING id`).Scan(&memberB); err != nil {
		t.Fatalf("seed member B: %v", err)
	}

	// 成员 A:2017 / 2020(IU/L) / 2021 两份同日 / dateless / 另一个指标名;
	// 成员 B:2021 同名。日期与 sha 同时用于锁排序。
	if _, err := pool.Exec(ctx, `INSERT INTO images (sha256, original_filename) VALUES
			('trend-a-2017','a.jpg'),('trend-a-2020-iu','a.jpg'),('trend-a-2021-aa','a.jpg'),
			('trend-a-2021-ab','a.jpg'),('trend-a-null','a.jpg'),('trend-a-other','a.jpg'),
			('trend-a-null-unit','a.jpg'),('trend-a-unverified','a.jpg'),('trend-b-2021','b.jpg')`); err != nil {
		t.Fatalf("seed images: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO ocr_results (image_sha256, engine_output) VALUES
			('trend-a-2017','{}'),('trend-a-2020-iu','{}'),('trend-a-2021-aa','{}'),
			('trend-a-2021-ab','{}'),('trend-a-null','{}'),('trend-a-other','{}'),
			('trend-a-null-unit','{}'),('trend-a-unverified','{}'),('trend-b-2021','{}')`); err != nil {
		t.Fatalf("seed ocr_results: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO reports
			(image_sha256, ocr_result_id, report_type, report_date, status, member_id) VALUES
			('trend-a-2017',(SELECT id FROM ocr_results WHERE image_sha256='trend-a-2017'),'生化','2017-11-18','success',$1),
			('trend-a-2020-iu',(SELECT id FROM ocr_results WHERE image_sha256='trend-a-2020-iu'),'生化','2020-01-01','success',$1),
			('trend-a-2021-aa',(SELECT id FROM ocr_results WHERE image_sha256='trend-a-2021-aa'),'生化','2021-08-02','success',$1),
			('trend-a-2021-ab',(SELECT id FROM ocr_results WHERE image_sha256='trend-a-2021-ab'),'生化','2021-08-02','success',$1),
			('trend-a-null',(SELECT id FROM ocr_results WHERE image_sha256='trend-a-null'),'生化',NULL,'success',$1),
			('trend-a-other',(SELECT id FROM ocr_results WHERE image_sha256='trend-a-other'),'生化','2022-01-01','success',$1),
			('trend-a-null-unit',(SELECT id FROM ocr_results WHERE image_sha256='trend-a-null-unit'),'生化','2023-01-01','success',$1),
			('trend-a-unverified',(SELECT id FROM ocr_results WHERE image_sha256='trend-a-unverified'),'生化','2022-06-01','success',$1),
			('trend-b-2021',(SELECT id FROM ocr_results WHERE image_sha256='trend-b-2021'),'生化','2021-08-02','success',$2)`,
		memberA, memberB); err != nil {
		t.Fatalf("seed reports: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO report_items
			(report_sha256, name, code, loinc, value, unit, unit_display, unit_status, ref_range, method, flag, low_confidence, raw_text, review_status) VALUES
			('trend-a-2017','丙氨酸转氨酶','1500104A','1742-6','27.40','U/L','U/L','verified','7-40',NULL,'normal',false,'ALT 27.40 U/L','unreviewed'),
			('trend-a-2020-iu','丙氨酸转氨酶','1500104A','1742-6','70','IU/L','IU/L','verified','9-50','速率法','high',true,'ALT 70 IU/L','confirmed'),
			('trend-a-2021-aa','丙氨酸转氨酶','1500104A','1742-6','835','U/L','U/L','verified','7-40',NULL,'high',false,'ALT 835 U/L','unreviewed'),
			('trend-a-2021-aa','丙氨酸转氨酶','1500104A','1742-6','836','U/L','U/L','verified','7-40',NULL,'high',false,'ALT 836 U/L','unreviewed'),
			('trend-a-2021-ab','丙氨酸转氨酶','1500104A','1742-6','900','U/L','U/L','verified','7-40','干化学','high',false,'ALT 900 U/L','corrected'),
			('trend-a-null','丙氨酸转氨酶','1500104A','1742-6','30','U/L','U/L','verified','7-40',NULL,'normal',false,'ALT 30 U/L','unreviewed'),
			('trend-a-null-unit','丙氨酸转氨酶','1500104A','1742-6','40',NULL,NULL,'verified','7-40',NULL,'normal',false,'ALT 40','unreviewed'),
			('trend-a-unverified','丙氨酸转氨酶','1500104A','1742-6','900','U/L','U/L','unverified','7-40',NULL,'high',false,'ALT 900 U/L','unreviewed'),
			('trend-a-other','天冬氨酸转氨酶','1500204A','1920-8','488','U/L','U/L','verified','13-35',NULL,'high',false,'AST 488 U/L','unreviewed'),
			('trend-b-2021','丙氨酸转氨酶','1500104A','1742-6','111','U/L','U/L','verified','7-40',NULL,'high',false,'ALT 111 U/L','unreviewed')`); err != nil {
		t.Fatalf("seed report_items: %v", err)
	}
	// 未验证单位点默认不进入 HTTP 趋势(票 12);CLI 由消费侧过滤。
	if _, err := pool.Exec(ctx, `UPDATE report_items SET unit_status='unverified'
			WHERE report_sha256='trend-a-unverified'`); err != nil {
		t.Fatalf("mark unverified: %v", err)
	}
	// 同一 code 出现在另一个 panel:趋势按 code 跨面板连接。
	if _, err := pool.Exec(ctx, `INSERT INTO report_items
			(report_sha256, name, panel, code, loinc, value, unit, unit_display, unit_status, ref_range, flag, low_confidence, raw_text, review_status)
		VALUES ('trend-a-2021-ab','丙氨酸氨基转移酶测定','other','1500104A','1742-6','901','U/L','U/L','verified','7-40','high',false,'ALT 901 U/L','unreviewed')`); err != nil {
		t.Fatalf("seed cross-panel item: %v", err)
	}

	// 全部单位:2017 → 2020(IU/L) → 2021 两份同日按 sha → dateless 沉底。
	all, err := st.TrendPoints(ctx, memberA, "丙氨酸转氨酶", "")
	if err != nil {
		t.Fatal(err)
	}
	wantAll := []struct {
		reportID     string
		date         *string
		value        string
		unit         *string
		refRange     string
		method       *string
		flag         string
		lowConf      bool
		reviewStatus string
	}{
		{"trend-a-2017", strPtr("2017-11-18"), "27.40", strPtr("U/L"), "7-40", nil, "normal", false, "unreviewed"},
		{"trend-a-2020-iu", strPtr("2020-01-01"), "70", strPtr("IU/L"), "9-50", strPtr("速率法"), "high", true, "confirmed"},
		{"trend-a-2021-aa", strPtr("2021-08-02"), "835", strPtr("U/L"), "7-40", nil, "high", false, "unreviewed"},
		{"trend-a-2021-aa", strPtr("2021-08-02"), "836", strPtr("U/L"), "7-40", nil, "high", false, "unreviewed"},
		{"trend-a-2021-ab", strPtr("2021-08-02"), "900", strPtr("U/L"), "7-40", strPtr("干化学"), "high", false, "corrected"},
		{"trend-a-2021-ab", strPtr("2021-08-02"), "901", strPtr("U/L"), "7-40", nil, "high", false, "unreviewed"},
		{"trend-a-null-unit", strPtr("2023-01-01"), "40", nil, "7-40", nil, "normal", false, "unreviewed"},
		{"trend-a-null", nil, "30", strPtr("U/L"), "7-40", nil, "normal", false, "unreviewed"},
	}
	if len(all) != len(wantAll) {
		t.Fatalf("all units = %d rows, want %d: %+v", len(all), len(wantAll), all)
	}
	for i, want := range wantAll {
		got := all[i]
		if got.ReportID != want.reportID || !sameTrendPtr(got.ReportDate, want.date) ||
			got.Value == nil || *got.Value != want.value ||
			!sameTrendPtr(got.Unit, want.unit) ||
			got.RefRange == nil || *got.RefRange != want.refRange ||
			!sameTrendPtr(got.Method, want.method) ||
			got.Flag != want.flag || got.LowConfidence != want.lowConf ||
			got.ReviewStatus != want.reviewStatus {
			t.Errorf("row %d = %+v, want %+v", i, got, want)
		}
	}

	// unit 门控:U/L 排除 2020 的 IU/L 与 2023 的 NULL 单位;同报告内按 i.id 升序。
	ul, err := st.TrendPoints(ctx, memberA, "丙氨酸转氨酶", "U/L")
	if err != nil {
		t.Fatal(err)
	}
	if len(ul) != 6 {
		t.Fatalf("U/L rows = %d, want 6: %+v", len(ul), ul)
	}
	if ul[0].ReportID != "trend-a-2017" || ul[1].ReportID != "trend-a-2021-aa" || ul[2].ReportID != "trend-a-2021-aa" ||
		ul[3].ReportID != "trend-a-2021-ab" || ul[4].ReportID != "trend-a-2021-ab" || ul[5].ReportID != "trend-a-null" {
		t.Errorf("U/L rows = %v, want [trend-a-2017 trend-a-2021-aa trend-a-2021-aa trend-a-2021-ab trend-a-2021-ab trend-a-null]", trendIDs(ul))
	}
	if ul[1].Value == nil || *ul[1].Value != "835" || ul[2].Value == nil || *ul[2].Value != "836" {
		t.Errorf("same-report rows = %v/%v, want i.id order 835 then 836", ul[1].Value, ul[2].Value)
	}
	iu, err := st.TrendPoints(ctx, memberA, "丙氨酸转氨酶", "IU/L")
	if err != nil {
		t.Fatal(err)
	}
	if got := trendIDs(iu); len(got) != 1 || got[0] != "trend-a-2020-iu" {
		t.Errorf("IU/L rows = %v, want [trend-a-2020-iu]", got)
	}

	// 输入标准码 / LOINC 与规范名走同一趋势键;unit_display/unit_status 透传。
	byCode, err := st.TrendPoints(ctx, memberA, "1500104A", "")
	if err != nil {
		t.Fatal(err)
	}
	if got := trendIDs(byCode); len(got) != len(all) {
		t.Errorf("code-key rows = %v, want same %d rows as name", got, len(all))
	}
	if all[0].UnitDisplay == nil || *all[0].UnitDisplay != "U/L" || all[0].UnitStatus == nil || *all[0].UnitStatus != "verified" {
		t.Errorf("unit fields = %v/%v, want U/L/verified", all[0].UnitDisplay, all[0].UnitStatus)
	}
	// CLI 存储面返回 unverified 点(成员 B 的同码点也属于全局 CLI 视图),
	// 由命令层决定默认隐藏(--all 可见)。
	cliRows, err := st.TrendByName(ctx, "丙氨酸转氨酶", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(cliRows) != len(all)+2 {
		t.Errorf("CLI rows = %d, want %d (member B + unverified included)", len(cliRows), len(all)+2)
	}
	unverifiedSeen := false
	for _, row := range cliRows {
		if row.UnitStatus != nil && *row.UnitStatus == "unverified" {
			unverifiedSeen = true
		}
	}
	if !unverifiedSeen {
		t.Error("CLI rows missing unverified unit point")
	}

	// 成员隔离与指标名隔离。
	b, err := st.TrendPoints(ctx, memberB, "丙氨酸转氨酶", "")
	if err != nil {
		t.Fatal(err)
	}
	if got := trendIDs(b); len(got) != 1 || got[0] != "trend-b-2021" {
		t.Errorf("member B rows = %v, want [trend-b-2021]", got)
	}
	other, err := st.TrendPoints(ctx, memberA, "天冬氨酸转氨酶", "")
	if err != nil {
		t.Fatal(err)
	}
	if got := trendIDs(other); len(got) != 1 || got[0] != "trend-a-other" {
		t.Errorf("other name rows = %v, want [trend-a-other]", got)
	}
	missing, err := st.TrendPoints(ctx, memberA, "不存在", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(missing) != 0 {
		t.Errorf("missing name rows = %d, want 0", len(missing))
	}
}

func sameTrendPtr(a, b *string) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}

func trendIDs(rows []pipeline.TrendPointRow) []string {
	ids := make([]string, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.ReportID)
	}
	return ids
}
