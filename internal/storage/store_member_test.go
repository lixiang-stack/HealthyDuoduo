// 成员存储冒烟(viz-and-client 票 05):真库上的建/列/改语义与契约一致——
// 列表创建序(id 升序)、PATCH 只覆盖传入字段、缺失返回 nil;存量无主报告
// (reports.member_id IS NULL)与成员独立存在:成员写入不触碰报告行。
// 需要真库:设 HDD_TEST_DSN 后运行,未设整体 Skip(与 migrate_test 同约定,
// CI 默认跳过;共享栈实时验证由 e2e 冒烟覆盖)。一次性容器示例:
// docker run --name hdd-member-smoke-pg -e POSTGRES_PASSWORD=smoke -p 55434:5432 postgres:17-alpine
package storage

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"healthyduoduo/internal/pipeline"
)

func TestMemberStoreSmoke(t *testing.T) {
	dsn := os.Getenv("HDD_TEST_DSN")
	if dsn == "" {
		t.Skip("HDD_TEST_DSN not set: member store smoke runs against a disposable PG container")
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

	// 存量 CLI 时代无主报告种子(成员出现前就存在的那类行)。
	if _, err := pool.Exec(ctx, `INSERT INTO images (sha256, original_filename) VALUES ('mem-a', 'p.jpg');
		 INSERT INTO ocr_results (image_sha256, engine_output) VALUES ('mem-a', '{}');
		 INSERT INTO reports (image_sha256, ocr_result_id, report_type, status)
		 VALUES ('mem-a', 1, '血常规', 'failed')`); err != nil {
		t.Fatalf("seed unowned report: %v", err)
	}

	st := NewStore(pool)
	defer st.Close()

	child, err := st.CreateMember(ctx, pipeline.MemberProfile{
		Name: "小虎", Sex: pipeline.SexMale, BirthDate: "2020-05-20",
	})
	if err != nil {
		t.Fatalf("create child: %v", err)
	}
	if child.MemberID == 0 || child.Name != "小虎" || !child.IsChild || child.CreatedAt.IsZero() ||
		child.Sex != pipeline.SexMale || child.BirthDate != "2020-05-20" {
		t.Fatalf("created child = %+v, want id/name/is_child/sex/birth_date/created_at filled", child)
	}
	adult, err := st.CreateMember(ctx, pipeline.MemberProfile{
		Name: "妈妈", Sex: pipeline.SexFemale, BirthDate: "1988-01-01",
	})
	if err != nil {
		t.Fatalf("create adult: %v", err)
	}
	if adult.Sex != pipeline.SexFemale || adult.IsChild {
		t.Fatalf("created adult = %+v, want female/adult profile", adult)
	}
	teen, err := st.CreateMember(ctx, pipeline.MemberProfile{
		Name: "青春期成员", Sex: pipeline.SexFemale, BirthDate: "2015-08-09",
	})
	if err != nil {
		t.Fatalf("create teen: %v", err)
	}
	if teen.BirthDate != "2015-08-09" || !teen.IsChild {
		t.Fatalf("created teen = %+v, want 2015-08-09/child", teen)
	}

	// 列表 = 创建序(id 升序),不因 is_child 重排。
	members, err := st.ListMembers(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(members) != 3 || members[0].MemberID != child.MemberID ||
		members[1].MemberID != adult.MemberID || members[2].MemberID != teen.MemberID {
		t.Fatalf("list = %+v, want [child, adult, teen] in creation order", members)
	}

	// 只改 name:is_child / sex / birth_date / created_at 保持原值。
	renamed := "母亲"
	updated, err := st.UpdateMember(ctx, adult.MemberID, pipeline.MemberPatch{Name: &renamed})
	if err != nil {
		t.Fatalf("update name: %v", err)
	}
	if updated.Name != "母亲" || updated.IsChild || !updated.CreatedAt.Equal(adult.CreatedAt) ||
		updated.Sex != adult.Sex || updated.BirthDate != adult.BirthDate {
		t.Fatalf("after name-only update = %+v, want name 母亲 / other fields unchanged", updated)
	}

	// 改生日:is_child 按新生日实时重算(1988 → 2010 变为孩子)。
	sex, birthDate := pipeline.SexMale, "2010-02-03"
	updated, err = st.UpdateMember(ctx, adult.MemberID, pipeline.MemberPatch{
		Sex: &sex, BirthDate: &birthDate,
	})
	if err != nil {
		t.Fatalf("update profile: %v", err)
	}
	if updated.Sex != sex || updated.BirthDate != birthDate || !updated.IsChild {
		t.Fatalf("after profile update = %+v, want male/2010-02-03/child", updated)
	}

	// 未知成员:nil, nil(HTTP 层转 404)。
	missing, err := st.UpdateMember(ctx, 999, pipeline.MemberPatch{Name: &renamed})
	if err != nil || missing != nil {
		t.Fatalf("update unknown = (%v, %v), want (nil, nil)", missing, err)
	}

	// 领域规则镜像:纯空白名 / 非法性别 / 非法日期在存储边界被拒,不落库。
	if _, err := st.CreateMember(ctx, pipeline.MemberProfile{
		Name: "   ", Sex: pipeline.SexMale, BirthDate: "2020-01-01",
	}); !errors.Is(err, pipeline.ErrMemberNameBlank) {
		t.Fatalf("create blank name err = %v, want ErrMemberNameBlank", err)
	}
	if _, err := st.CreateMember(ctx, pipeline.MemberProfile{
		Name: "小虎", Sex: "unspecified", BirthDate: "2020-01-01",
	}); !errors.Is(err, pipeline.ErrMemberSexInvalid) {
		t.Fatalf("create invalid sex err = %v, want ErrMemberSexInvalid", err)
	}
	badDate := "2020-02-30"
	if _, err := st.UpdateMember(ctx, child.MemberID, pipeline.MemberPatch{BirthDate: &badDate}); !errors.Is(err, pipeline.ErrMemberBirthDateInvalid) {
		t.Fatalf("update invalid birth_date err = %v, want ErrMemberBirthDateInvalid", err)
	}
	blank := " "
	if _, err := st.UpdateMember(ctx, child.MemberID, pipeline.MemberPatch{Name: &blank}); !errors.Is(err, pipeline.ErrMemberNameBlank) {
		t.Fatalf("update blank name err = %v, want ErrMemberNameBlank", err)
	}

	// 存量无主报告未被成员写入触碰:仍一行、仍无主。
	var reports, owned int
	if err := pool.QueryRow(ctx, `SELECT count(*), count(member_id) FROM reports`).Scan(&reports, &owned); err != nil {
		t.Fatal(err)
	}
	if reports != 1 || owned != 0 {
		t.Fatalf("reports = %d rows / %d owned, want 1/0 (unowned legacy report untouched)", reports, owned)
	}
}

// TestDeleteMemberCascadesReports 删除成员:成员与其全部报告硬删,报告级联
// 指标项 / 修正对;原图与 OCR 历史保留;其他成员不受影响。
func TestDeleteMemberCascadesReports(t *testing.T) {
	dsn := os.Getenv("HDD_TEST_DSN")
	if dsn == "" {
		t.Skip("HDD_TEST_DSN not set: member delete cascade runs against a disposable PG container")
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

	target, err := st.CreateMember(ctx, pipeline.MemberProfile{
		Name: "待删成员", Sex: pipeline.SexFemale, BirthDate: "2020-01-01",
	})
	if err != nil {
		t.Fatalf("create target: %v", err)
	}
	keep, err := st.CreateMember(ctx, pipeline.MemberProfile{
		Name: "保留成员", Sex: pipeline.SexFemale, BirthDate: "1988-01-01",
	})
	if err != nil {
		t.Fatalf("create keep: %v", err)
	}

	sha := strings.Repeat("d", 64)
	if _, err := pool.Exec(ctx, `INSERT INTO images (sha256, original_filename) VALUES ($1, 'delete.jpg')`, sha); err != nil {
		t.Fatalf("seed image: %v", err)
	}
	var ocrID int64
	if err := pool.QueryRow(ctx, `INSERT INTO ocr_results (image_sha256, engine_output) VALUES ($1, '{}') RETURNING id`, sha).Scan(&ocrID); err != nil {
		t.Fatalf("seed ocr: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO reports
		 (image_sha256, ocr_result_id, report_type, report_date, status, member_id, panels)
		 VALUES ($1, $2, '临检', '2026-10-01', 'success', $3, '{cbc}')`, sha, ocrID, target.MemberID); err != nil {
		t.Fatalf("seed report: %v", err)
	}
	var itemID int64
	if err := pool.QueryRow(ctx, `INSERT INTO report_items
		 (report_sha256, name, panel, value, unit, ref_range, flag, low_confidence, raw_text)
		 VALUES ($1, '血红蛋白', 'cbc', '128', 'g/L', '115-150', 'normal', false, '血红蛋白 128 g/L 115-150')
		 RETURNING id`, sha).Scan(&itemID); err != nil {
		t.Fatalf("seed item: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO correction_pairs
		 (item_id, before_value, after_value, raw_text, ocr_line_numbers)
		 VALUES ($1, '120', '128', '血红蛋白 128 g/L 115-150', '{0}')`, itemID); err != nil {
		t.Fatalf("seed correction pair: %v", err)
	}

	deleted, err := st.DeleteMember(ctx, target.MemberID)
	if err != nil || !deleted {
		t.Fatalf("delete member = (%v, %v), want (true, nil)", deleted, err)
	}

	count := func(query string, arg any) int {
		t.Helper()
		var n int
		if err := pool.QueryRow(ctx, query, arg).Scan(&n); err != nil {
			t.Fatalf("count: %v", err)
		}
		return n
	}
	if got := count(`SELECT count(*) FROM members WHERE id = $1`, target.MemberID); got != 0 {
		t.Errorf("target member rows = %d, want 0", got)
	}
	if got := count(`SELECT count(*) FROM members WHERE id = $1`, keep.MemberID); got != 1 {
		t.Errorf("other member rows = %d, want 1", got)
	}
	if got := count(`SELECT count(*) FROM reports WHERE member_id = $1`, target.MemberID); got != 0 {
		t.Errorf("owned reports = %d, want 0", got)
	}
	if got := count(`SELECT count(*) FROM report_items WHERE report_sha256 = $1`, sha); got != 0 {
		t.Errorf("report items = %d, want 0", got)
	}
	if got := count(`SELECT count(*) FROM correction_pairs WHERE item_id = $1`, itemID); got != 0 {
		t.Errorf("correction pairs = %d, want 0", got)
	}
	if got := count(`SELECT count(*) FROM images WHERE sha256 = $1`, sha); got != 1 {
		t.Errorf("image rows = %d, want 1 (original image retained)", got)
	}
	if got := count(`SELECT count(*) FROM ocr_results WHERE image_sha256 = $1`, sha); got != 1 {
		t.Errorf("ocr rows = %d, want 1 (ocr history retained)", got)
	}

	deleted, err = st.DeleteMember(ctx, 999)
	if err != nil || deleted {
		t.Fatalf("delete unknown = (%v, %v), want (false, nil)", deleted, err)
	}
}
