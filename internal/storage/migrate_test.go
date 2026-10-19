package storage

// 迁移冒烟:goose 链(0001..0004)应用后 schema 形状符合 M0 契约(票 02)。
// 需要真库:设 HDD_TEST_DSN(如 postgres://postgres:smoke@127.0.0.1:55432/postgres)后运行;
// 未设则整体 Skip——CI 门禁跑 go test ./... 时默认跳过,共享栈验证走 e2e 外环
// (docs/agents/e2e.md「schemas/迁移」行)。冒烟姿势:一次性 PG 容器 + 本测试。

import (
	"context"
	"database/sql"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pressly/goose/v3"
)

func TestMigrationSmokeM0Contract(t *testing.T) {
	dsn := os.Getenv("HDD_TEST_DSN")
	if dsn == "" {
		t.Skip("HDD_TEST_DSN not set: migration smoke runs against a disposable PG container")
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

	// goose 已升到 0006 且幂等(Open 内重复 Migrate 不报错)。
	var latest int
	if err := pool.QueryRow(ctx, `SELECT max(version_id) FROM goose_db_version`).Scan(&latest); err != nil {
		t.Fatalf("read goose version: %v", err)
	}
	if latest != 6 {
		t.Fatalf("goose version = %d, want 6", latest)
	}

	// 0006 报告级段落六列(影像单):三个 TEXT + 三个 INTEGER[],存量可空。
	t.Run("report sections columns", func(t *testing.T) {
		rows, err := pool.Query(ctx, `SELECT column_name, data_type FROM information_schema.columns
			WHERE table_name='reports' AND column_name IN (
				'exam_name','exam_name_ocr_line_numbers','findings','findings_ocr_line_numbers',
				'impression','impression_ocr_line_numbers')`)
		if err != nil {
			t.Fatal(err)
		}
		defer rows.Close()
		got := map[string]string{}
		for rows.Next() {
			var name, typ string
			if err := rows.Scan(&name, &typ); err != nil {
				t.Fatal(err)
			}
			got[name] = typ
		}
		if err := rows.Err(); err != nil {
			t.Fatal(err)
		}
		want := map[string]string{
			"exam_name": "text", "exam_name_ocr_line_numbers": "ARRAY",
			"findings": "text", "findings_ocr_line_numbers": "ARRAY",
			"impression": "text", "impression_ocr_line_numbers": "ARRAY",
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("section columns = %v, want %v", got, want)
		}
		var nullable int
		if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM information_schema.columns
			WHERE table_name='reports' AND is_nullable='YES' AND column_name IN (
				'exam_name','exam_name_ocr_line_numbers','findings','findings_ocr_line_numbers',
				'impression','impression_ocr_line_numbers')`).Scan(&nullable); err != nil {
			t.Fatal(err)
		}
		if nullable != 6 {
			t.Fatalf("nullable section columns = %d, want 6(存量行 NULL)", nullable)
		}
	})

	// report_items.ocr_line_numbers(0005):识别端直吐的组成行下标,可空(存量未回填)。
	t.Run("report item ocr line numbers column", func(t *testing.T) {
		var dataType string
		var nullable string
		if err := pool.QueryRow(ctx, `SELECT data_type, is_nullable FROM information_schema.columns
			WHERE table_name='report_items' AND column_name='ocr_line_numbers'`).Scan(&dataType, &nullable); err != nil {
			t.Fatal(err)
		}
		if dataType != "ARRAY" || nullable != "YES" {
			t.Fatalf("ocr_line_numbers = %s/%s, want ARRAY/YES(存量行 NULL)", dataType, nullable)
		}
	})

	// 子测试共享种子:两份报告(一份有主、一份存量 CLI 时代无主)与图像/OCR 留痕。
	if _, err := pool.Exec(ctx, `INSERT INTO members (id, name, is_child) VALUES (1, 'user_test', true)`); err != nil {
		t.Fatalf("seed member: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO images (sha256, original_filename) VALUES ('a', 'p.jpg'),
		('b', 'p.jpg')`); err != nil {
		t.Fatalf("seed images: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO ocr_results (image_sha256, engine_output) VALUES
		('a', '{}'), ('b', '{}')`); err != nil {
		t.Fatalf("seed ocr: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO reports (image_sha256, ocr_result_id, report_type, status, member_id)
		VALUES ('a', 1, '血常规', 'partial', 1), ('b', 2, '尿常规', 'failed', NULL)`); err != nil {
		t.Fatalf("seed reports: %v (member ownership on new rows, NULL on unowned)", err)
	}

	// 成员最小模型:仅 name + is_child 两档业务字段(+id/created_at),零扩展列。
	t.Run("members minimal model", func(t *testing.T) {
		rows, err := pool.Query(ctx, `SELECT column_name FROM information_schema.columns
			WHERE table_name='members' ORDER BY column_name`)
		if err != nil {
			t.Fatal(err)
		}
		defer rows.Close()
		var cols []string
		for rows.Next() {
			var c string
			if err := rows.Scan(&c); err != nil {
				t.Fatal(err)
			}
			cols = append(cols, c)
		}
		if err := rows.Err(); err != nil {
			t.Fatal(err)
		}
		if len(cols) != 4 || cols[0] != "created_at" || cols[1] != "id" || cols[2] != "is_child" || cols[3] != "name" {
			t.Fatalf("members columns = %v, want exactly [created_at id is_child name]", cols)
		}
	})

	// 报告表:成员归属可空(存量 CLI 时代报告无主)+ fail_reason。
	t.Run("reports ownership and fail reason", func(t *testing.T) {
		var failReasonNullable, memberNullable string
		if err := pool.QueryRow(ctx, `SELECT
			max(is_nullable) FILTER (WHERE column_name = 'fail_reason'),
			max(is_nullable) FILTER (WHERE column_name = 'member_id')
			FROM information_schema.columns WHERE table_name='reports'`).
			Scan(&failReasonNullable, &memberNullable); err != nil {
			t.Fatal(err)
		}
		if failReasonNullable != "YES" || memberNullable != "YES" {
			t.Fatalf("fail_reason/member_id nullable = %s/%s, want YES/YES", failReasonNullable, memberNullable)
		}
		var owned, unowned int
		if err := pool.QueryRow(ctx, `SELECT
			count(*) FILTER (WHERE member_id = 1), count(*) FILTER (WHERE member_id IS NULL)
			FROM reports`).Scan(&owned, &unowned); err != nil {
			t.Fatal(err)
		}
		if owned != 1 || unowned != 1 {
			t.Fatalf("report ownership rows = %d owned / %d unowned, want 1/1", owned, unowned)
		}
	})

	// 核对状态默认 unreviewed(存量项自动归未核对),非法值被 CHECK 拒绝——
	// 断言误差来源是约束本身(名字精确匹配),不是 FK 等其他报错造成的假绿。
	t.Run("review status default and check", func(t *testing.T) {
		var def string
		if err := pool.QueryRow(ctx, `SELECT column_default FROM information_schema.columns
			WHERE table_name='report_items' AND column_name='review_status'`).Scan(&def); err != nil {
			t.Fatal(err)
		}
		if def != "'unreviewed'::text" {
			t.Fatalf("review_status default = %q, want 'unreviewed'::text", def)
		}
		if _, err := pool.Exec(ctx, `INSERT INTO report_items
			(report_sha256, name, flag, raw_text, low_confidence, review_status)
			VALUES ('a', 'junk', 'normal', 'junk', false, 'retracted')`); err == nil {
			t.Fatal("review_status 'retracted' accepted, want CHECK rejection")
		} else if !strings.Contains(err.Error(), "review_status_check") {
			t.Fatalf("expected review_status_check violation, got %v", err)
		}
	})

	// 修正对表 append-only 形状 + 随报告级联删除(票 10 删除边界)。
	t.Run("correction pairs cascade", func(t *testing.T) {
		if _, err := pool.Exec(ctx, `INSERT INTO report_items
			(id, report_sha256, name, flag, raw_text, low_confidence, review_status)
			VALUES (11, 'a', '血红蛋白', 'normal', '血红蛋白 128 g/L 115-150', true, 'corrected')`); err != nil {
			t.Fatalf("seed item: %v", err)
		}
		if _, err := pool.Exec(ctx, `INSERT INTO correction_pairs
			(item_id, before_name, before_value, before_unit, before_ref_range,
			 after_name, after_value, after_unit, after_ref_range, raw_text, ocr_line_numbers)
			VALUES (11, '血红蛋白','128','g/L','115-150', '血红蛋白','168','g/L','115-150',
				'血红蛋白 128 g/L 115-150', ARRAY[0,1,2])`); err != nil {
			t.Fatalf("seed correction pair: %v (before→after full quad)", err)
		}
		var owned, unowned int
		if err := pool.QueryRow(ctx, `SELECT
			count(*) FILTER (WHERE member_id = 1), count(*) FILTER (WHERE member_id IS NULL)
			FROM reports`).Scan(&owned, &unowned); err != nil {
			t.Fatal(err)
		}
		if owned != 1 || unowned != 1 {
			t.Fatalf("report ownership rows = %d owned / %d unowned, want 1/1", owned, unowned)
		}
		// 报告删除 → items 级联(0001) → 修正对随之清除(留痕边界:images/ocr_results 保留)。
		if _, err := pool.Exec(ctx, `DELETE FROM reports WHERE image_sha256 = 'a'`); err != nil {
			t.Fatalf("delete report: %v", err)
		}
		var items, pairs, images int
		if err := pool.QueryRow(ctx, `SELECT
			(SELECT count(*) FROM report_items), (SELECT count(*) FROM correction_pairs),
			(SELECT count(*) FROM images)`).Scan(&items, &pairs, &images); err != nil {
			t.Fatal(err)
		}
		if items != 0 || pairs != 0 || images != 2 {
			t.Fatalf("after report delete: items=%d pairs=%d images=%d, want 0/0/2 (image retained)", items, pairs, images)
		}
	})
}

// TestMigration0006DownUp 0006 升降版:down 删除六个段落列,up 重新加回。
func TestMigration0006DownUp(t *testing.T) {
	dsn := os.Getenv("HDD_TEST_DSN")
	if dsn == "" {
		t.Skip("HDD_TEST_DSN not set: migration smoke runs against a disposable PG container")
	}
	ctx := context.Background()
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer db.Close()
	goose.SetBaseFS(migrationsFS)
	if err := goose.SetDialect("postgres"); err != nil {
		t.Fatalf("goose dialect: %v", err)
	}
	if err := goose.UpContext(ctx, db, migrationsDir); err != nil {
		t.Fatalf("up to latest: %v", err)
	}
	if err := goose.DownToContext(ctx, db, migrationsDir, 5); err != nil {
		t.Fatalf("down to 0005: %v", err)
	}
	var n int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM information_schema.columns
		WHERE table_name='reports' AND column_name='exam_name'`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Fatalf("exam_name column survived down migration")
	}
	if err := goose.UpContext(ctx, db, migrationsDir); err != nil {
		t.Fatalf("up after down: %v", err)
	}
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM information_schema.columns
		WHERE table_name='reports' AND column_name='exam_name'`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("exam_name column missing after re-up")
	}
}
