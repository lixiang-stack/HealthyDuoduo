// 报告存储冒烟(viz-and-client 票 06):真库上的时间线语义与契约一致——
// 成员过滤(含无主存量)、日期降序 + dateless 沉底、同日期按 sha 升序;
// flag 聚合与未核对低置信计数为响应派生(unreviewed 只算 partial 报告,
// success/failed 恒 0);WriteReport 不触碰 member_id(override 重跑保持归属)。
// 需要真库:设 HDD_TEST_DSN 后运行,未设整体 Skip(与 store_member_test 同约定,
// CI 默认跳过;共享栈实时验证由 e2e 冒烟覆盖)。一次性容器示例:
// docker run --name hdd-report-smoke-pg -e POSTGRES_PASSWORD=smoke -p 55435:5432 postgres:17-alpine
package storage

import (
	"context"
	"os"
	"reflect"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"healthyduoduo/internal/contract"
	"healthyduoduo/internal/pipeline"
)

func TestReportStoreSmoke(t *testing.T) {
	dsn := os.Getenv("HDD_TEST_DSN")
	if dsn == "" {
		t.Skip("HDD_TEST_DSN not set: report store smoke runs against a disposable PG container")
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

	// 成员两个(脚本建,绕过 HTTP)。
	var memberA, memberB int64
	if err := pool.QueryRow(ctx, `INSERT INTO members (name, is_child) VALUES ('小虎', true) RETURNING id`).Scan(&memberA); err != nil {
		t.Fatalf("seed member A: %v", err)
	}
	if err := pool.QueryRow(ctx, `INSERT INTO members (name, is_child) VALUES ('妈妈', false) RETURNING id`).Scan(&memberB); err != nil {
		t.Fatalf("seed member B: %v", err)
	}

	// 四份报告:成员 A 的 partial/failed、成员 B 的 success(免核对面)、无主存量。
	if _, err := pool.Exec(ctx, `INSERT INTO images (sha256, original_filename) VALUES ('rep-a','a.jpg'),('rep-b','b.jpg'),('rep-c','c.jpg'),('rep-d','d.jpg')`); err != nil {
		t.Fatalf("seed images: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO ocr_results (image_sha256, engine_output) VALUES ('rep-a','{}'),('rep-b','{}'),('rep-c','{}'),('rep-d','{}')`); err != nil {
		t.Fatalf("seed ocr_results: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO reports (image_sha256, ocr_result_id, report_type, report_date, status, member_id, fail_reason) VALUES
			('rep-a',(SELECT id FROM ocr_results WHERE image_sha256='rep-a'),'血常规','2026-10-02','partial',$1,NULL),
			('rep-b',(SELECT id FROM ocr_results WHERE image_sha256='rep-b'),'其他专项',NULL,'failed',$1,'no metric items recognized'),
			('rep-c',(SELECT id FROM ocr_results WHERE image_sha256='rep-c'),'尿常规','2026-10-01','success',$2,NULL),
			('rep-d',(SELECT id FROM ocr_results WHERE image_sha256='rep-d'),'血糖','2026-09-30','success',NULL,NULL)`,
		memberA, memberB); err != nil {
		t.Fatalf("seed reports: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO report_items (report_sha256, name, value, unit, ref_range, flag, low_confidence, raw_text, review_status) VALUES
			('rep-a','单核细胞比率','10.8','%','3-10','high',true,'单核细胞百分比  10.8  ↑ %  3-10','unreviewed'),
			('rep-a','平均血红蛋白浓度','300','g/L','316-354','low',true,'平均血红蛋白浓度  300  g/L  316-354','unreviewed'),
			('rep-a','已确认低置信项','1.0',NULL,NULL,'low',true,'已确认低置信项  1.0','confirmed'),
			('rep-a','血红蛋白','128','g/L','115-150','normal',false,'血红蛋白  128  g/L  115-150','unreviewed'),
			('rep-a','某未知项',NULL,NULL,NULL,'unknown',false,'某未知项','unreviewed'),
			('rep-c','尿蛋白','10.8',NULL,NULL,'high',true,'尿蛋白  10.8','unreviewed'),
			('rep-d','葡萄糖','128',NULL,NULL,'normal',false,'葡萄糖  128','unreviewed')`); err != nil {
		t.Fatalf("seed report_items: %v", err)
	}
	// 行链接下标(0005)按真实形态回填其中一行:读回校验用。
	if _, err := pool.Exec(ctx, `UPDATE report_items SET ocr_line_numbers = ARRAY[7,8,9,10]
		WHERE report_sha256='rep-a' AND name='单核细胞比率'`); err != nil {
		t.Fatalf("seed ocr_line_numbers: %v", err)
	}

	// 全量:日期降序,dateless 沉底;逐卡核聚合口径。
	all, err := st.ListTimeline(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 4 {
		t.Fatalf("timeline = %d rows, want 4", len(all))
	}
	wantOrder := []string{"rep-a", "rep-c", "rep-d", "rep-b"}
	for i, want := range wantOrder {
		if all[i].Sha256 != want {
			t.Fatalf("timeline order = %v, want %v (日期降序、dateless 沉底)", shaList(all), wantOrder)
		}
	}
	a := all[0]
	if a.MemberID == nil || *a.MemberID != memberA || a.ReportType != "血常规" || a.ReportDate == nil || *a.ReportDate != "2026-10-02" {
		t.Errorf("rep-a summary = %+v", a)
	}
	if a.FlagCounts != (pipeline.FlagCounts{High: 1, Low: 2, Unknown: 1}) {
		t.Errorf("rep-a flag_counts = %+v, want {1 2 1}", a.FlagCounts)
	}
	if a.UnreviewedLowConfidenceCount != 2 {
		t.Errorf("rep-a unreviewed = %d, want 2 (低置信未核对;confirmed 项不计)", a.UnreviewedLowConfidenceCount)
	}
	c := all[1]
	if c.UnreviewedLowConfidenceCount != 0 {
		t.Errorf("rep-c unreviewed = %d, want 0 (success 免核对,计数恒 0)", c.UnreviewedLowConfidenceCount)
	}
	if c.FlagCounts != (pipeline.FlagCounts{High: 1}) {
		t.Errorf("rep-c flag_counts = %+v, want {1 0 0}(徽标照算)", c.FlagCounts)
	}
	d := all[2]
	if d.MemberID != nil {
		t.Errorf("rep-d member_id = %v, want null (无主存量)", d.MemberID)
	}
	b := all[3]
	if b.ReportDate != nil || b.Status != contract.StatusFailed || b.FailReason == nil || *b.FailReason != "no metric items recognized" {
		t.Errorf("rep-b summary = %+v, want dateless failed with fail_reason", b)
	}
	if b.FlagCounts != (pipeline.FlagCounts{}) || b.UnreviewedLowConfidenceCount != 0 {
		t.Errorf("rep-b counts = %+v/%d, want 全 0", b.FlagCounts, b.UnreviewedLowConfidenceCount)
	}

	// 成员过滤:只回该成员,无主与别家都不混入。
	owned, err := st.ListTimeline(ctx, &memberA)
	if err != nil {
		t.Fatal(err)
	}
	if len(owned) != 2 || owned[0].Sha256 != "rep-a" || owned[1].Sha256 != "rep-b" {
		t.Fatalf("member A timeline = %v, want [rep-a rep-b]", shaList(owned))
	}

	// 单报告摘要:命中与缺失口径。
	one, err := st.SummaryByImage(ctx, "rep-a")
	if err != nil || one == nil {
		t.Fatalf("SummaryByImage(rep-a) = %v, %v", one, err)
	}
	if one.FlagCounts != a.FlagCounts || one.UnreviewedLowConfidenceCount != a.UnreviewedLowConfidenceCount {
		t.Errorf("SummaryByImage counts = %+v/%d, want 与时间线一致 %+v/%d",
			one.FlagCounts, one.UnreviewedLowConfidenceCount, a.FlagCounts, a.UnreviewedLowConfidenceCount)
	}
	missing, err := st.SummaryByImage(ctx, "rep-missing")
	if err != nil || missing != nil {
		t.Fatalf("SummaryByImage(missing) = (%v, %v), want (nil, nil)", missing, err)
	}

	// 指标项回读:id 升序、句柄非零且唯一、核对状态随行。
	items, err := st.ItemsWithReviewStatus(ctx, "rep-a")
	if err != nil {
		t.Fatal(err)
	}

	// 报告所用 OCR 结果回读(详情行链接输入,票 07):命中与缺失口径。
	ocrRaw, err := st.OCRResultJSONByReport(ctx, "rep-a")
	if err != nil || string(ocrRaw) != "{}" {
		t.Fatalf("OCRResultJSONByReport(rep-a) = %q, %v, want 种子 OCR 行", ocrRaw, err)
	}
	if ocrRaw, err := st.OCRResultJSONByReport(ctx, "rep-missing"); err != nil || ocrRaw != nil {
		t.Fatalf("OCRResultJSONByReport(missing) = %q, %v, want (nil, nil)", ocrRaw, err)
	}
	if len(items) != 5 {
		t.Fatalf("rep-a items = %d, want 5", len(items))
	}
	seen := map[int64]bool{}
	for i, it := range items {
		if it.ItemID == 0 || seen[it.ItemID] {
			t.Errorf("item[%d].item_id = %d (重复或零)", i, it.ItemID)
		}
		seen[it.ItemID] = true
		if i > 0 && items[i-1].ItemID >= it.ItemID {
			t.Errorf("items not ordered by id: %d then %d", items[i-1].ItemID, it.ItemID)
		}
	}
	if items[2].ReviewStatus != "confirmed" || items[0].ReviewStatus != "unreviewed" {
		t.Errorf("review_status not read back: %+v", items)
	}
	if len(items[0].OCRLineNumbers) != 4 || items[0].OCRLineNumbers[0] != 7 || items[0].OCRLineNumbers[3] != 10 {
		t.Errorf("ocr_line_numbers not read back: %+v", items[0].OCRLineNumbers)
	}
	if len(items[1].OCRLineNumbers) != 0 {
		t.Errorf("NULL ocr_line_numbers = %+v, want 空数组(存量行)", items[1].OCRLineNumbers)
	}

	// WriteReport(override 重跑镜像)不触碰 member_id:归属保持;items 全量替换。
	var ocrID int64
	if err := pool.QueryRow(ctx, `SELECT id FROM ocr_results WHERE image_sha256='rep-a'`).Scan(&ocrID); err != nil {
		t.Fatalf("fetch ocr id: %v", err)
	}
	v := "130"
	if err := st.WriteReport(ctx, "rep-a", ocrID, contract.Report{
		ReportType: "血常规", ReportDate: strPtr("2026-10-02"), Status: contract.StatusSuccess,
		Items: []contract.ReportItem{{Name: "血红蛋白", Value: &v, Flag: "normal", RawText: "血红蛋白  130", OCRLineNumbers: []int32{3, 4}}},
	}, nil); err != nil {
		t.Fatalf("WriteReport override: %v", err)
	}
	after, err := st.SummaryByImage(ctx, "rep-a")
	if err != nil || after == nil {
		t.Fatal(err)
	}
	if after.MemberID == nil || *after.MemberID != memberA {
		t.Errorf("member_id after rerun = %v, want %d unchanged", after.MemberID, memberA)
	}
	items, err = st.ItemsWithReviewStatus(ctx, "rep-a")
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].ReviewStatus != pipeline.ReviewStatusUnreviewed {
		t.Errorf("items after rerun = %+v, want 1 unreviewed (重置归档)", items)
	}
	if len(items[0].OCRLineNumbers) != 2 || items[0].OCRLineNumbers[0] != 3 || items[0].OCRLineNumbers[1] != 4 {
		t.Errorf("ocr_line_numbers after rerun = %+v, want [3 4]", items[0].OCRLineNumbers)
	}

	// 报告段落(0006/ADR-0008):JSONB 写入/读回;非叙述型 NULL → nil;缺失报告 nil。
	sections := []contract.ReportSection{
		{Kind: "exam_name", Text: "产科胎儿彩超检查（中孕）", OCRLineNumbers: []int32{5}},
		{Kind: "impression", Text: "宫内单活胎。", OCRLineNumbers: []int32{10, 11}},
	}
	var ocrB int64
	if err := pool.QueryRow(ctx, `SELECT id FROM ocr_results WHERE image_sha256='rep-b'`).Scan(&ocrB); err != nil {
		t.Fatalf("fetch rep-b ocr id: %v", err)
	}
	if err := st.WriteReport(ctx, "rep-b", ocrB, contract.Report{
		ReportType: "影像", ReportDate: strPtr("2024-09-15"), Status: contract.StatusFailed,
		Sections: sections,
	}, nil); err != nil {
		t.Fatalf("WriteReport sections: %v", err)
	}
	gotSections, err := st.SectionsByImage(ctx, "rep-b")
	if err != nil || !reflect.DeepEqual(gotSections, sections) {
		t.Errorf("SectionsByImage(rep-b) = %+v, %v; want %+v", gotSections, err, sections)
	}
	if got, err := st.SectionsByImage(ctx, "rep-a"); err != nil || got != nil {
		t.Errorf("SectionsByImage(non-narrative) = (%+v, %v), want (nil, nil)", got, err)
	}
	if got, err := st.SectionsByImage(ctx, "rep-missing"); err != nil || got != nil {
		t.Errorf("SectionsByImage(missing) = (%+v, %v), want (nil, nil)", got, err)
	}

	// 归属写入:无主 → 成员 A;缺失报告返回 false, nil(404 口径)。
	if ok, err := st.SetReportMember(ctx, "rep-d", memberA); err != nil || !ok {
		t.Fatalf("SetReportMember(rep-d) = (%v, %v), want (true, nil)", ok, err)
	}
	owned, err = st.ListTimeline(ctx, &memberA)
	if err != nil || len(owned) != 3 {
		t.Fatalf("member A timeline after assign = %v (%v), want 3", shaList(owned), err)
	}
	if ok, err := st.SetReportMember(ctx, "rep-missing", memberA); err != nil || ok {
		t.Errorf("SetReportMember(missing) = (%v, %v), want (false, nil)", ok, err)
	}

	// MemberByID:命中与缺失(上传归属校验口径)。
	m, err := st.MemberByID(ctx, memberB)
	if err != nil || m == nil || m.Name != "妈妈" || m.IsChild {
		t.Fatalf("MemberByID = %+v, %v", m, err)
	}
	none, err := st.MemberByID(ctx, 999)
	if err != nil || none != nil {
		t.Fatalf("MemberByID(999) = (%v, %v), want (nil, nil)", none, err)
	}

	// 硬删与重归档(票 10):先修正 rep-c 一项造修正对;删除报告 → 报告/指标项/修正对
	// 级联清除,images 与 ocr_results 留痕保留;同图重新归档(WriteReport)可行。
	itemsC, err := st.ItemsWithReviewStatus(ctx, "rep-c")
	if err != nil || len(itemsC) != 1 {
		t.Fatalf("rep-c items = %+v, %v", itemsC, err)
	}
	newValue := "11.0"
	if _, err := st.ReviewItem(ctx, "rep-c", itemsC[0].ItemID, pipeline.ReviewAction{Value: &newValue}); err != nil {
		t.Fatalf("review rep-c: %v", err)
	}
	if n := countReportItems(t, ctx, pool, "rep-c"); n != 1 {
		t.Fatalf("rep-c items before delete = %d, want 1", n)
	}
	if n := countCorrectionPairs(t, ctx, pool, "rep-c"); n != 1 {
		t.Fatalf("rep-c pairs before delete = %d, want 1", n)
	}
	deleted, err := st.DeleteReport(ctx, "rep-c")
	if err != nil || !deleted {
		t.Fatalf("DeleteReport(rep-c) = %v, %v, want (true, nil)", deleted, err)
	}
	if one, err := st.SummaryByImage(ctx, "rep-c"); err != nil || one != nil {
		t.Errorf("SummaryByImage(rep-c) after delete = (%v, %v), want (nil, nil)", one, err)
	}
	if n := countReportItems(t, ctx, pool, "rep-c"); n != 0 {
		t.Errorf("rep-c items after delete = %d, want 0(级联)", n)
	}
	if n := countCorrectionPairs(t, ctx, pool, "rep-c"); n != 0 {
		t.Errorf("rep-c pairs after delete = %d, want 0(随指标项级联)", n)
	}
	var imgRows, ocrRows int
	if err := pool.QueryRow(ctx, `SELECT (SELECT COUNT(*) FROM images WHERE sha256='rep-c'),
			(SELECT COUNT(*) FROM ocr_results WHERE image_sha256='rep-c')`).Scan(&imgRows, &ocrRows); err != nil {
		t.Fatal(err)
	}
	if imgRows != 1 || ocrRows != 1 {
		t.Errorf("images/ocr rows after delete = %d/%d, want 1/1(原始图像与识别历史留痕)", imgRows, ocrRows)
	}
	// 已删报告再删:false, nil(404 口径,不虚报错误)。
	if deleted, err := st.DeleteReport(ctx, "rep-c"); err != nil || deleted {
		t.Errorf("second DeleteReport(rep-c) = (%v, %v), want (false, nil)", deleted, err)
	}
	// 同图重新归档:报告行可重写,归属从零开始(重传由上传归属重新赋值)。
	var reOcrID int64
	if err := pool.QueryRow(ctx, `SELECT id FROM ocr_results WHERE image_sha256='rep-c'`).Scan(&reOcrID); err != nil {
		t.Fatalf("fetch rep-c ocr id: %v", err)
	}
	v2 := "10.5"
	if err := st.WriteReport(ctx, "rep-c", reOcrID, contract.Report{
		ReportType: "尿常规", ReportDate: strPtr("2026-10-01"), Status: contract.StatusSuccess,
		Items: []contract.ReportItem{{Name: "尿蛋白", Value: &v2, Flag: "high", RawText: "尿蛋白  10.5"}},
	}, nil); err != nil {
		t.Fatalf("re-archive rep-c: %v", err)
	}
	back, err := st.SummaryByImage(ctx, "rep-c")
	if err != nil || back == nil {
		t.Fatalf("re-archived rep-c summary = (%v, %v), want 非 nil", back, err)
	}
	if back.MemberID != nil {
		t.Errorf("re-archived member_id = %v, want null", back.MemberID)
	}
	if n := countReportItems(t, ctx, pool, "rep-c"); n != 1 {
		t.Errorf("rep-c items after re-archive = %d, want 1", n)
	}
	// 缺失报告:false, nil(404 口径)。
	if deleted, err := st.DeleteReport(ctx, "rep-missing"); err != nil || deleted {
		t.Errorf("DeleteReport(rep-missing) = (%v, %v), want (false, nil)", deleted, err)
	}
}

// countReportItems 报告下指标项计数(删除级联断言用)。
func countReportItems(t *testing.T, ctx context.Context, pool *pgxpool.Pool, reportSha string) int {
	t.Helper()
	var n int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM report_items WHERE report_sha256 = $1`, reportSha).Scan(&n); err != nil {
		t.Fatalf("count report items: %v", err)
	}
	return n
}

func shaList(summaries []pipeline.ReportSummary) []string {
	out := make([]string, 0, len(summaries))
	for _, s := range summaries {
		out = append(out, s.Sha256)
	}
	return out
}

func strPtr(s string) *string { return &s }
