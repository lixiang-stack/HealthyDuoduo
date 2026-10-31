// 项目组浏览存储冒烟(panel-browse 票 03):项目列表按成员聚合(报告级 panels /
// 未回填存量 other 回退 / failed 排除 / 最近日期与固定优先级排序),项目详情按
// 趋势键 + canonical unit 分行(跨报告点、not_tested 口径、最新报告内顺序、影像 narrative)。
// 需要真库:设 HDD_TEST_DSN 后运行,未设整体 Skip(与 store_reports_test 同约定):
// docker run --name hdd-panel-smoke-pg -e POSTGRES_PASSWORD=smoke -p 55437:5432 postgres:17-alpine
package storage

import (
	"context"
	"os"
	"reflect"
	"slices"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"healthyduoduo/internal/pipeline"
)

func TestPanelStoreSmoke(t *testing.T) {
	dsn := os.Getenv("HDD_TEST_DSN")
	if dsn == "" {
		t.Skip("HDD_TEST_DSN not set: panel store smoke runs against a disposable PG container")
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

	seedPanelStore(t, ctx, pool)

	memberA := panelMemberID(t, ctx, pool, "面板测试成员")
	memberB := panelMemberID(t, ctx, pool, "面板影像成员")
	memberC := panelMemberID(t, ctx, pool, "面板空成员")

	// 项目列表:失败报告排除;未回填存量归 other;dateless 沉底;同日期按固定优先级。
	panels, err := st.PanelSummariesByMember(ctx, memberA)
	if err != nil {
		t.Fatalf("PanelSummariesByMember: %v", err)
	}
	wantOrder := []string{"liver", "kidney", "uric_acid", "lipid", "electrolyte", "other", "glucose"}
	if got := panelCodesOf(panels); !reflect.DeepEqual(got, wantOrder) {
		t.Fatalf("panel order = %v, want %v", got, wantOrder)
	}
	liver := findPanel(panels, "liver")
	if liver.ReportCount != 3 || liver.ItemCount != 5 || liver.AbnormalCount != 3 {
		t.Errorf("liver summary = %+v, want report/item/abnormal = 3/5/3", liver)
	}
	if liver.LatestReportDate == nil || *liver.LatestReportDate != "2021-08-02" || liver.LatestReportID == nil || *liver.LatestReportID != "pn-a" {
		t.Errorf("liver latest = %v/%v, want 2021-08-02/pn-a", liver.LatestReportDate, liver.LatestReportID)
	}
	if !reflect.DeepEqual(liver.ReportTypes, []string{"生化"}) || liver.Narrative {
		t.Errorf("liver types/narrative = %v/%v", liver.ReportTypes, liver.Narrative)
	}
	lipid := findPanel(panels, "lipid")
	if lipid.ReportCount != 2 || lipid.ItemCount != 2 || lipid.AbnormalCount != 1 {
		t.Errorf("lipid summary = %+v, want report/item/abnormal = 2/2/1", lipid)
	}
	if lipid.LatestReportDate == nil || *lipid.LatestReportDate != "2020-01-01" {
		t.Errorf("lipid latest date = %v, want 2020-01-01 (dateless report 不冒充最近日期)", lipid.LatestReportDate)
	}
	electrolyte := findPanel(panels, "electrolyte")
	if electrolyte.ReportCount != 2 || electrolyte.ItemCount != 2 || electrolyte.AbnormalCount != 1 {
		t.Errorf("electrolyte summary = %+v, want report/item/abnormal = 2/2/1", electrolyte)
	}
	// 同日两份报告:最近报告取 sha 降序最后一份(pn-f),与趋势点升序的最后一点一致。
	if electrolyte.LatestReportDate == nil || *electrolyte.LatestReportDate != "2020-01-01" ||
		electrolyte.LatestReportID == nil || *electrolyte.LatestReportID != "pn-f" {
		t.Errorf("electrolyte latest = %v/%v, want 2020-01-01/pn-f", electrolyte.LatestReportDate, electrolyte.LatestReportID)
	}
	other := findPanel(panels, "other")
	if other.ReportCount != 1 || other.ItemCount != 2 || other.AbnormalCount != 1 {
		t.Errorf("other summary = %+v, want report/item/abnormal = 1/2/1", other)
	}
	if !reflect.DeepEqual(other.ReportTypes, []string{"临检"}) {
		t.Errorf("other report_types = %v, want [临检]", other.ReportTypes)
	}
	glucose := findPanel(panels, "glucose")
	if glucose.LatestReportDate != nil || glucose.ReportCount != 1 {
		t.Errorf("glucose summary = %+v, want dateless report_count 1", glucose)
	}

	// 无报告成员:空数组,不返回空分类。
	empty, err := st.PanelSummariesByMember(ctx, memberC)
	if err != nil {
		t.Fatalf("PanelSummariesByMember(empty): %v", err)
	}
	if len(empty) != 0 {
		t.Errorf("empty member panels = %+v, want []", empty)
	}

	// 项目详情:跨报告点、not_tested、最新报告内顺序。
	liverDetail, err := st.PanelItemsByMemberPanel(ctx, memberA, "liver")
	if err != nil {
		t.Fatalf("PanelItemsByMemberPanel(liver): %v", err)
	}
	if liverDetail.ReportCount != 3 || liverDetail.LatestReportDate == nil || *liverDetail.LatestReportDate != "2021-08-02" ||
		liverDetail.LatestReportID == nil || *liverDetail.LatestReportID != "pn-a" ||
		!reflect.DeepEqual(liverDetail.ReportTypes, []string{"生化"}) || liverDetail.Narrative {
		t.Fatalf("liver detail header = %+v", liverDetail)
	}
	wantItems := []string{"丙氨酸转氨酶", "总胆红素", "总蛋白", "白蛋白", "前白蛋白"}
	if got := panelItemNames(liverDetail.Items); !reflect.DeepEqual(got, wantItems) {
		t.Fatalf("liver items = %v, want abnormal first then latest report order %v", got, wantItems)
	}
	alt := liverDetail.Items[0]
	if alt.Latest == nil || alt.Latest.Value == nil || *alt.Latest.Value != "835" || alt.Latest.Flag != "high" ||
		alt.RefRange == nil || *alt.RefRange != "7-40" || alt.ReportCount != 2 || alt.NotTestedCount != 0 ||
		alt.LatestReportNotTested {
		t.Errorf("ALT item = %+v", alt)
	}
	if len(alt.Points) != 2 || *alt.Points[0].Value != "27.4" || *alt.Points[1].Value != "835" {
		t.Errorf("ALT points = %+v, want 2020 normal then 2021 high", alt.Points)
	}
	if alt.Points[1].ReviewStatus != "corrected" {
		t.Errorf("ALT review status = %q, want corrected (已核对透传)", alt.Points[1].ReviewStatus)
	}
	totalProtein := liverDetail.Items[2]
	if totalProtein.Latest == nil || totalProtein.Latest.Value == nil || *totalProtein.Latest.Value != "90" ||
		totalProtein.Latest.ReportID != "pn-f" || totalProtein.ReportCount != 2 {
		t.Errorf("total protein = %+v, want latest pn-f 90 (同日按 sha 取最后一份)", totalProtein)
	}
	albumin := liverDetail.Items[3]
	if albumin.Latest == nil || albumin.Latest.Value == nil || *albumin.Latest.Value != "42" ||
		albumin.ReportCount != 1 || albumin.NotTestedCount != 1 || !albumin.LatestReportNotTested {
		t.Errorf("albumin = %+v, want latest 42 / not_tested in newer report", albumin)
	}
	prealbumin := liverDetail.Items[4]
	if prealbumin.Latest != nil || len(prealbumin.Points) != 0 || prealbumin.ReportCount != 0 ||
		prealbumin.NotTestedCount != 1 || !prealbumin.LatestReportNotTested {
		t.Errorf("prealbumin = %+v, want no latest but not_tested", prealbumin)
	}

	// 电解质:同日两份报告,详情最新取 pn-f;异常数与项目列表一致。
	electrolyteDetail, err := st.PanelItemsByMemberPanel(ctx, memberA, "electrolyte")
	if err != nil {
		t.Fatalf("PanelItemsByMemberPanel(electrolyte): %v", err)
	}
	if electrolyteDetail.ReportCount != 2 || electrolyteDetail.LatestReportID == nil || *electrolyteDetail.LatestReportID != "pn-f" {
		t.Fatalf("electrolyte detail header = %+v, want latest pn-f", electrolyteDetail)
	}
	if got := panelItemNames(electrolyteDetail.Items); !reflect.DeepEqual(got, []string{"钠", "钾"}) {
		t.Errorf("electrolyte items = %v, want 钠 then 钾", got)
	}
	if electrolyteDetail.Items[0].Latest == nil || electrolyteDetail.Items[0].Latest.Flag != pipeline.FlagHigh {
		t.Errorf("electrolyte latest = %+v, want 钠 high from pn-f", electrolyteDetail.Items[0])
	}

	// 肾功:两个指标都来自最新报告,pn-a 内顺序优先。
	kidney, err := st.PanelItemsByMemberPanel(ctx, memberA, "kidney")
	if err != nil {
		t.Fatalf("PanelItemsByMemberPanel(kidney): %v", err)
	}
	if got := panelItemNames(kidney.Items); !reflect.DeepEqual(got, []string{"尿素", "肌酐"}) {
		t.Errorf("kidney items = %v", got)
	}
	if kidney.Items[0].ReportCount != 2 || kidney.Items[1].ReportCount != 1 {
		t.Errorf("kidney report counts = %d/%d, want 2/1", kidney.Items[0].ReportCount, kidney.Items[1].ReportCount)
	}

	// 影像:项目列表 narrative=true,详情 items=[] 且 latest date 可为 null。
	imagingPanels, err := st.PanelSummariesByMember(ctx, memberB)
	if err != nil || len(imagingPanels) != 1 {
		t.Fatalf("imaging panels = %+v, %v", imagingPanels, err)
	}
	if imagingPanels[0].Panel != "imaging" || !imagingPanels[0].Narrative || imagingPanels[0].ReportCount != 1 ||
		imagingPanels[0].LatestReportDate != nil || imagingPanels[0].LatestReportID == nil || *imagingPanels[0].LatestReportID != "pn-i" {
		t.Errorf("imaging summary = %+v", imagingPanels[0])
	}
	imagingDetail, err := st.PanelItemsByMemberPanel(ctx, memberB, "imaging")
	if err != nil {
		t.Fatalf("PanelItemsByMemberPanel(imaging): %v", err)
	}
	if !imagingDetail.Narrative || len(imagingDetail.Items) != 0 || imagingDetail.ReportCount != 1 ||
		!reflect.DeepEqual(imagingDetail.ReportTypes, []string{"影像"}) {
		t.Errorf("imaging detail = %+v", imagingDetail)
	}

	// 未回填存量归 other;失败报告的 liver 指标不进入项目列表 / 详情。
	otherDetail, err := st.PanelItemsByMemberPanel(ctx, memberA, "other")
	if err != nil {
		t.Fatalf("PanelItemsByMemberPanel(other): %v", err)
	}
	if got := panelItemNames(otherDetail.Items); !reflect.DeepEqual(got, []string{"ABO血型", "备注"}) {
		t.Errorf("other items = %v", got)
	}
	if got := panelItemNames(liverDetail.Items); slices.Contains(got, "失败报告指标") {
		t.Errorf("failed report items leaked into panel detail: %v", got)
	}
	if _, err := st.PanelItemsByMemberPanel(ctx, memberA, "glucose"); err != nil {
		t.Fatalf("PanelItemsByMemberPanel(glucose): %v", err)
	}
	missing, err := st.PanelItemsByMemberPanel(ctx, memberC, "liver")
	if err != nil {
		t.Fatalf("PanelItemsByMemberPanel(no reports): %v", err)
	}
	if missing.ReportCount != 0 || len(missing.Items) != 0 || missing.LatestReportDate != nil || missing.LatestReportID != nil {
		t.Errorf("missing detail = %+v, want empty", missing)
	}
}

func seedPanelStore(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	t.Helper()
	if _, err := pool.Exec(ctx, `DELETE FROM reports WHERE image_sha256 IN ('pn-a','pn-b','pn-c','pn-d','pn-e','pn-f','pn-i');
		DELETE FROM ocr_results WHERE image_sha256 IN ('pn-a','pn-b','pn-c','pn-d','pn-e','pn-f','pn-i');
		DELETE FROM images WHERE sha256 IN ('pn-a','pn-b','pn-c','pn-d','pn-e','pn-f','pn-i');
		DELETE FROM members WHERE name IN ('面板测试成员','面板影像成员','面板空成员')`); err != nil {
		t.Fatalf("clean previous seed: %v", err)
	}
	var memberA, memberB int64
	if err := pool.QueryRow(ctx, `INSERT INTO members (name, is_child) VALUES ('面板测试成员', true) RETURNING id`).Scan(&memberA); err != nil {
		t.Fatalf("seed member A: %v", err)
	}
	if err := pool.QueryRow(ctx, `INSERT INTO members (name, is_child) VALUES ('面板影像成员', false) RETURNING id`).Scan(&memberB); err != nil {
		t.Fatalf("seed member B: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO members (name, is_child) VALUES ('面板空成员', false)`); err != nil {
		t.Fatalf("seed member C: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO images (sha256, original_filename) VALUES
			('pn-a','a.jpg'),('pn-b','b.jpg'),('pn-c','c.jpg'),('pn-d','d.jpg'),('pn-e','e.jpg'),('pn-f','f.jpg'),('pn-i','i.jpg');
		INSERT INTO ocr_results (image_sha256, engine_output) VALUES
			('pn-a','{}'),('pn-b','{}'),('pn-c','{}'),('pn-d','{}'),('pn-e','{}'),('pn-f','{}'),('pn-i','{}')`); err != nil {
		t.Fatalf("seed images/ocr: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO reports
			(image_sha256, ocr_result_id, report_type, report_date, status, member_id, fail_reason, panels)
		VALUES
			('pn-a',(SELECT id FROM ocr_results WHERE image_sha256='pn-a'),'生化','2021-08-02','success',$1,NULL,ARRAY['liver','kidney','uric_acid']),
			('pn-b',(SELECT id FROM ocr_results WHERE image_sha256='pn-b'),'生化','2020-01-01','success',$1,NULL,ARRAY['liver','kidney','lipid','electrolyte']),
			('pn-c',(SELECT id FROM ocr_results WHERE image_sha256='pn-c'),'临检','2019-01-01','success',$1,NULL,'{}'),
			('pn-d',(SELECT id FROM ocr_results WHERE image_sha256='pn-d'),'生化','2022-01-01','failed',$1,'no metric items recognized',ARRAY['liver']),
			('pn-e',(SELECT id FROM ocr_results WHERE image_sha256='pn-e'),'生化',NULL,'success',$1,NULL,ARRAY['lipid','glucose']),
			('pn-f',(SELECT id FROM ocr_results WHERE image_sha256='pn-f'),'生化','2020-01-01','success',$1,NULL,ARRAY['liver','electrolyte']),
			('pn-i',(SELECT id FROM ocr_results WHERE image_sha256='pn-i'),'影像',NULL,'success',$2,NULL,ARRAY['imaging'])`,
		memberA, memberB); err != nil {
		t.Fatalf("seed reports: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO report_items
			(report_sha256, name, panel, value, unit, ref_range, method, flag, flag_reason, low_confidence, raw_text, review_status)
		VALUES
			('pn-a','丙氨酸转氨酶','liver','835','U/L','7-40',NULL,'high',NULL,false,'丙氨酸转氨酶 835 U/L 7-40','corrected'),
			('pn-a','总胆红素','liver','41','μmol/L','0-21',NULL,'high',NULL,false,'总胆红素 41 μmol/L 0-21','unreviewed'),
			('pn-a','白蛋白','liver',NULL,'g/L',NULL,NULL,'not_tested',NULL,false,'白蛋白 - - -','unreviewed'),
			('pn-a','前白蛋白','liver',NULL,'mg/L',NULL,NULL,'not_tested',NULL,false,'前白蛋白 - - -','unreviewed'),
			('pn-a','尿素','kidney','4.5','mmol/L','2.9-8.2',NULL,'normal',NULL,false,'尿素 4.5 mmol/L 2.9-8.2','unreviewed'),
			('pn-a','肌酐','kidney','60','μmol/L','44-97',NULL,'normal',NULL,false,'肌酐 60 μmol/L 44-97','unreviewed'),
			('pn-a','尿酸','uric_acid','300','μmol/L','150-420',NULL,'normal',NULL,false,'尿酸 300 μmol/L 150-420','unreviewed'),
			('pn-b','丙氨酸转氨酶','liver','27.4','U/L','7-40',NULL,'normal',NULL,false,'丙氨酸转氨酶 27.4 U/L 7-40','unreviewed'),
			('pn-b','总蛋白','liver','70','g/L','60-80',NULL,'normal',NULL,false,'总蛋白 70 g/L 60-80','unreviewed'),
			('pn-b','白蛋白','liver','42','g/L','40-55',NULL,'normal',NULL,false,'白蛋白 42 g/L 40-55','unreviewed'),
			('pn-b','尿素','kidney','5.0','mmol/L','2.9-8.2',NULL,'normal',NULL,false,'尿素 5.0 mmol/L 2.9-8.2','unreviewed'),
			('pn-b','总胆固醇','lipid','4.0','mmol/L','0-5.2',NULL,'normal',NULL,false,'总胆固醇 4.0 mmol/L 0-5.2','unreviewed'),
			('pn-b','钾','electrolyte','4.2','mmol/L','3.5-5.3',NULL,'normal',NULL,false,'钾 4.2 mmol/L 3.5-5.3','unreviewed'),
			('pn-c','ABO血型','other','A',NULL,NULL,NULL,'abnormal',NULL,false,'ABO血型 A','unreviewed'),
			('pn-c','备注','other',NULL,NULL,NULL,NULL,'unknown','informational',false,'备注','unreviewed'),
			('pn-d','失败报告指标','liver','99','U/L','7-40',NULL,'high',NULL,false,'失败报告指标 99 U/L 7-40','unreviewed'),
			('pn-e','甘油三酯','lipid','2.5','mmol/L','0-1.7',NULL,'high',NULL,false,'甘油三酯 2.5 mmol/L 0-1.7','unreviewed'),
			('pn-e','葡萄糖','glucose','5.0','mmol/L','3.9-6.1',NULL,'normal',NULL,false,'葡萄糖 5.0 mmol/L 3.9-6.1','unreviewed'),
			('pn-f','总蛋白','liver','90','g/L','60-80',NULL,'high',NULL,false,'总蛋白 90 g/L 60-80','unreviewed'),
			('pn-f','钠','electrolyte','150','mmol/L','135-145',NULL,'high',NULL,false,'钠 150 mmol/L 135-145','unreviewed')`); err != nil {
		t.Fatalf("seed report_items: %v", err)
	}
}

func panelMemberID(t *testing.T, ctx context.Context, pool *pgxpool.Pool, name string) int64 {
	t.Helper()
	var id int64
	if err := pool.QueryRow(ctx, `SELECT id FROM members WHERE name=$1`, name).Scan(&id); err != nil {
		t.Fatalf("member %s: %v", name, err)
	}
	return id
}

func panelCodesOf(panels []pipeline.PanelSummary) []string {
	out := make([]string, 0, len(panels))
	for _, panel := range panels {
		out = append(out, panel.Panel)
	}
	return out
}

func findPanel(panels []pipeline.PanelSummary, code string) pipeline.PanelSummary {
	for _, panel := range panels {
		if panel.Panel == code {
			return panel
		}
	}
	return pipeline.PanelSummary{}
}

func panelItemNames(items []pipeline.PanelItem) []string {
	out := make([]string, 0, len(items))
	for _, item := range items {
		out = append(out, item.Name)
	}
	return out
}

// TestPanelStoreJoinsByCodeAcrossPanels 票 12:同一 code 跨报告 / 跨面板连接,
// 名称变化不拆行;canonical unit 不同仍分行。
func TestPanelStoreJoinsByCodeAcrossPanels(t *testing.T) {
	dsn := os.Getenv("HDD_TEST_DSN")
	if dsn == "" {
		t.Skip("HDD_TEST_DSN not set: panel store smoke runs against a disposable PG container")
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

	if _, err := pool.Exec(ctx, `DELETE FROM reports WHERE image_sha256 IN ('pt-a','pt-b','pt-c');
		DELETE FROM ocr_results WHERE image_sha256 IN ('pt-a','pt-b','pt-c');
		DELETE FROM images WHERE sha256 IN ('pt-a','pt-b','pt-c');
		DELETE FROM members WHERE name = '趋势键成员'`); err != nil {
		t.Fatalf("clean seed: %v", err)
	}
	var memberID int64
	if err := pool.QueryRow(ctx, `INSERT INTO members (name, is_child) VALUES ('趋势键成员', false) RETURNING id`).Scan(&memberID); err != nil {
		t.Fatalf("seed member: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO images (sha256, original_filename) VALUES
			('pt-a','a.jpg'),('pt-b','b.jpg'),('pt-c','c.jpg');
		INSERT INTO ocr_results (image_sha256, engine_output) VALUES
			('pt-a','{}'),('pt-b','{}'),('pt-c','{}')`); err != nil {
		t.Fatalf("seed images/ocr: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO reports (image_sha256, ocr_result_id, report_type, report_date, status, member_id, panels) VALUES
			('pt-a',(SELECT id FROM ocr_results WHERE image_sha256='pt-a'),'生化','2021-01-01','success',$1,ARRAY['liver']),
			('pt-b',(SELECT id FROM ocr_results WHERE image_sha256='pt-b'),'生化','2022-01-01','success',$1,ARRAY['other']),
			('pt-c',(SELECT id FROM ocr_results WHERE image_sha256='pt-c'),'生化','2023-01-01','success',$1,ARRAY['liver','urinalysis'])`,
		memberID); err != nil {
		t.Fatalf("seed reports: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO report_items
			(report_sha256, name, panel, code, loinc, value, unit, unit_display, unit_status, ref_range, flag, low_confidence, raw_text, review_status) VALUES
			('pt-a','测试ALT','liver','TEST-PANEL-ALT','1742-6','30','U/L','U/L','verified','7-40','normal',false,'ALT 30 U/L','unreviewed'),
			('pt-b','测试ALT新名','other','TEST-PANEL-ALT','1742-6','40','U/L','U/L','verified','7-40','high',false,'ALT 40 U/L','unreviewed'),
			('pt-c','测试ALT最新名','liver','TEST-PANEL-ALT','1742-6','50','U/L','U/L','verified','7-40','high',false,'ALT 50 U/L','unreviewed'),
			('pt-c','测试尿蛋白','urinalysis','TEST-PANEL-UA',NULL,'阴性',NULL,NULL,NULL,'阴性','normal',false,'尿蛋白 阴性','unreviewed')`); err != nil {
		t.Fatalf("seed rows: %v", err)
	}

	liver, err := st.PanelItemsByMemberPanel(ctx, memberID, "liver")
	if err != nil {
		t.Fatalf("liver detail: %v", err)
	}
	if len(liver.Items) != 1 {
		t.Fatalf("liver items = %+v, want one code-joined item", liver.Items)
	}
	alt := liver.Items[0]
	if alt.Key != "TEST-PANEL-ALT" || alt.Name != "测试ALT最新名" {
		t.Errorf("ALT key/name = %q/%q, want code + latest name", alt.Key, alt.Name)
	}
	if len(alt.Points) != 3 || alt.Latest == nil || alt.Latest.ReportID != "pt-c" || *alt.Latest.Value != "50" {
		t.Errorf("ALT points = %+v, want 3 cross-panel points with latest pt-c", alt.Points)
	}
	if alt.Points[0].UnitDisplay == nil || *alt.Points[0].UnitDisplay != "U/L" ||
		alt.Points[0].UnitStatus == nil || *alt.Points[0].UnitStatus != "verified" {
		t.Errorf("ALT point unit fields = %+v, want U/L/verified", alt.Points[0])
	}

	// other 面板也连接同一 code(跨面板点的另一端),但尿常规面板不受影响。
	other, err := st.PanelItemsByMemberPanel(ctx, memberID, "other")
	if err != nil {
		t.Fatalf("other detail: %v", err)
	}
	if len(other.Items) != 1 || other.Items[0].Key != "TEST-PANEL-ALT" || len(other.Items[0].Points) != 3 {
		t.Errorf("other items = %+v, want same code with 3 points", other.Items)
	}
	urinalysis, err := st.PanelItemsByMemberPanel(ctx, memberID, "urinalysis")
	if err != nil {
		t.Fatalf("urinalysis detail: %v", err)
	}
	if len(urinalysis.Items) != 1 || urinalysis.Items[0].Key != "TEST-PANEL-UA" {
		t.Errorf("urinalysis items = %+v, want only its own key", urinalysis.Items)
	}
}
