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

	child, err := st.CreateMember(ctx, "小虎", true)
	if err != nil {
		t.Fatalf("create child: %v", err)
	}
	if child.MemberID == 0 || child.Name != "小虎" || !child.IsChild || child.CreatedAt.IsZero() {
		t.Fatalf("created child = %+v, want id/name/is_child/created_at filled", child)
	}
	adult, err := st.CreateMember(ctx, "妈妈", false)
	if err != nil {
		t.Fatalf("create adult: %v", err)
	}

	// 列表 = 创建序(id 升序):孩子不前置(孩子优先是客户端的事)。
	members, err := st.ListMembers(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(members) != 2 || members[0].MemberID != child.MemberID || members[1].MemberID != adult.MemberID {
		t.Fatalf("list = %+v, want [child, adult] in creation order", members)
	}

	// 只改 name:is_child 与 created_at 保持原值。
	renamed := "母亲"
	updated, err := st.UpdateMember(ctx, adult.MemberID, &renamed, nil)
	if err != nil {
		t.Fatalf("update name: %v", err)
	}
	if updated.Name != "母亲" || updated.IsChild || !updated.CreatedAt.Equal(adult.CreatedAt) {
		t.Fatalf("after name-only update = %+v, want name 母亲 / is_child false / created_at unchanged", updated)
	}

	// 只改 is_child:name 保持原值。
	toChild := true
	updated, err = st.UpdateMember(ctx, adult.MemberID, nil, &toChild)
	if err != nil {
		t.Fatalf("update is_child: %v", err)
	}
	if !updated.IsChild || updated.Name != "母亲" {
		t.Fatalf("after is_child-only update = %+v, want is_child true / name 母亲", updated)
	}

	// 未知成员:nil, nil(HTTP 层转 404)。
	missing, err := st.UpdateMember(ctx, 999, &renamed, nil)
	if err != nil || missing != nil {
		t.Fatalf("update unknown = (%v, %v), want (nil, nil)", missing, err)
	}

	// 领域规则镜像:纯空白名在存储边界(非 HTTP 路径)被拒,不落库。
	if _, err := st.CreateMember(ctx, "   ", false); !errors.Is(err, pipeline.ErrMemberNameBlank) {
		t.Fatalf("create blank name err = %v, want ErrMemberNameBlank", err)
	}
	blank := " "
	if _, err := st.UpdateMember(ctx, child.MemberID, &blank, nil); !errors.Is(err, pipeline.ErrMemberNameBlank) {
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
