package storage

// 迁移冒烟:唯一的 0019_init.sql 应用后,schema 形状符合当前契约。
// 需要真库:设 HDD_TEST_DSN(如 postgres://postgres:smoke@127.0.0.1:55432/postgres)后运行;
// 未设则整体 Skip——CI 门禁跑 go test ./... 时默认跳过,共享栈验证走 e2e 外环
// (docs/agents/e2e.md「schemas/迁移」行)。姿势:一次性 PG 容器 + 本测试。

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestMigrationSchema(t *testing.T) {
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

	// 基线固定在 version 20:0019 是当前完整 schema,0020 追加解释层缓存表。
	var latest int
	if err := pool.QueryRow(ctx, `SELECT max(version_id) FROM goose_db_version`).Scan(&latest); err != nil {
		t.Fatalf("read goose version: %v", err)
	}
	if latest != 20 {
		t.Fatalf("goose version = %d, want 20", latest)
	}

	t.Run("tables", func(t *testing.T) {
		want := []string{
			"analyses", "analysis_events", "analysis_explanation_cache",
			"analysis_feedback", "analysis_llm_calls",
			"correction_pairs", "growth_records", "images", "members", "ocr_results",
			"report_items", "reports",
		}
		for _, table := range want {
			var n int
			if err := pool.QueryRow(ctx, `SELECT count(*) FROM information_schema.tables
				WHERE table_schema='public' AND table_name=$1`, table).Scan(&n); err != nil {
				t.Fatal(err)
			}
			if n != 1 {
				t.Errorf("table %s missing", table)
			}
		}
	})

	t.Run("indexes", func(t *testing.T) {
		for _, index := range []string{
			"reports_member_date_idx",
			"report_items_report_panel_idx",
			"growth_records_member_measured_idx",
			"analyses_member_created_idx",
			"analysis_feedback_created_idx",
			"analysis_feedback_analysis_idx",
			"analysis_events_created_idx",
			"analysis_events_analysis_idx",
			"analysis_llm_calls_created_idx",
		} {
			var n int
			if err := pool.QueryRow(ctx, `SELECT count(*) FROM pg_indexes
				WHERE indexname = $1`, index).Scan(&n); err != nil {
				t.Fatal(err)
			}
			if n != 1 {
				t.Errorf("index %s missing", index)
			}
		}
	})

	t.Run("report columns", func(t *testing.T) {
		// reports.sections 可空(非叙述型/存量 NULL);panels 为项目组聚合。
		var sectionsType, sectionsNullable, panelsType, panelsNullable, panelsDefault string
		if err := pool.QueryRow(ctx, `SELECT
			max(data_type) FILTER (WHERE column_name='sections'),
			max(is_nullable) FILTER (WHERE column_name='sections'),
			max(data_type) FILTER (WHERE column_name='panels'),
			max(is_nullable) FILTER (WHERE column_name='panels'),
			max(column_default) FILTER (WHERE column_name='panels')
			FROM information_schema.columns WHERE table_name='reports'`).
			Scan(&sectionsType, &sectionsNullable, &panelsType, &panelsNullable, &panelsDefault); err != nil {
			t.Fatal(err)
		}
		if sectionsType != "jsonb" || sectionsNullable != "YES" {
			t.Fatalf("sections = %s/%s, want jsonb/YES", sectionsType, sectionsNullable)
		}
		if panelsType != "ARRAY" || panelsNullable != "NO" || panelsDefault != "'{}'::text[]" {
			t.Fatalf("panels = %s/%s default %q, want ARRAY/NO/'{}'::text[]",
				panelsType, panelsNullable, panelsDefault)
		}
	})

	t.Run("report item columns", func(t *testing.T) {
		cases := []struct {
			column   string
			dataType string
			nullable string
		}{
			{"value", "text", "YES"},
			{"method", "text", "YES"},
			{"ocr_line_numbers", "ARRAY", "YES"},
			{"flag_reason", "text", "YES"},
			{"panel", "text", "NO"},
			{"review_status", "text", "NO"},
			{"analyte", "text", "YES"},
		}
		for _, c := range cases {
			var dataType, nullable string
			if err := pool.QueryRow(ctx, `SELECT data_type, is_nullable
				FROM information_schema.columns WHERE table_name='report_items' AND column_name=$1`,
				c.column).Scan(&dataType, &nullable); err != nil {
				t.Fatalf("%s: %v", c.column, err)
			}
			if dataType != c.dataType || nullable != c.nullable {
				t.Errorf("report_items.%s = %s/%s, want %s/%s",
					c.column, dataType, nullable, c.dataType, c.nullable)
			}
		}
		var panelDefault, reviewDefault string
		if err := pool.QueryRow(ctx, `SELECT
			max(column_default) FILTER (WHERE column_name='panel'),
			max(column_default) FILTER (WHERE column_name='review_status')
			FROM information_schema.columns WHERE table_name='report_items'`).
			Scan(&panelDefault, &reviewDefault); err != nil {
			t.Fatal(err)
		}
		if panelDefault != "'other'::text" {
			t.Errorf("report_items.panel default = %q, want 'other'::text", panelDefault)
		}
		if reviewDefault != "'unreviewed'::text" {
			t.Errorf("report_items.review_status default = %q, want 'unreviewed'::text", reviewDefault)
		}
	})

	t.Run("member profile columns", func(t *testing.T) {
		rows, err := pool.Query(ctx, `SELECT column_name, is_nullable FROM information_schema.columns
			WHERE table_name='members' ORDER BY column_name`)
		if err != nil {
			t.Fatal(err)
		}
		defer rows.Close()
		cols := map[string]string{}
		for rows.Next() {
			var name, nullable string
			if err := rows.Scan(&name, &nullable); err != nil {
				t.Fatal(err)
			}
			cols[name] = nullable
		}
		if err := rows.Err(); err != nil {
			t.Fatal(err)
		}
		want := map[string]bool{
			"id": false, "name": false, "sex": false, "birth_date": false, "created_at": false,
		}
		if len(cols) != len(want) {
			t.Fatalf("members columns = %v, want %d profile columns", cols, len(want))
		}
		for name := range want {
			if _, ok := cols[name]; !ok {
				t.Fatalf("members missing column %q", name)
			}
		}
		if cols["sex"] != "NO" || cols["birth_date"] != "NO" {
			t.Fatalf("sex/birth_date nullable = %s/%s, want NO/NO", cols["sex"], cols["birth_date"])
		}
		if _, ok := cols["is_child"]; ok {
			t.Fatal("is_child column still present, want derived-only")
		}
	})

	// 子测试共享种子:一名成员、两份报告(一份有主、一份存量无主)与图像/OCR 留痕。
	if _, err := pool.Exec(ctx, `INSERT INTO members (id, name, sex, birth_date)
		VALUES (1, 'user_test', 'male', '2020-01-01')`); err != nil {
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

	t.Run("review status default and check", func(t *testing.T) {
		if _, err := pool.Exec(ctx, `INSERT INTO report_items
			(report_sha256, name, flag, raw_text, low_confidence, review_status)
			VALUES ('a', 'junk', 'normal', 'junk', false, 'retracted')`); err == nil {
			t.Fatal("review_status 'retracted' accepted, want CHECK rejection")
		} else if !strings.Contains(err.Error(), "review_status_check") {
			t.Fatalf("expected review_status_check violation, got %v", err)
		}
	})

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
			t.Fatalf("seed correction pair: %v", err)
		}
		// 报告删除 → items 级联 → 修正对随之清除(留痕边界:images/ocr_results 保留)。
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

	t.Run("growth and analysis constraints", func(t *testing.T) {
		var growthNullable map[string]string
		rows, err := pool.Query(ctx, `SELECT column_name, is_nullable FROM information_schema.columns
			WHERE table_name='growth_records'`)
		if err != nil {
			t.Fatal(err)
		}
		growthNullable = map[string]string{}
		for rows.Next() {
			var name, nullable string
			if err := rows.Scan(&name, &nullable); err != nil {
				t.Fatal(err)
			}
			growthNullable[name] = nullable
		}
		rows.Close()
		if growthNullable["height_cm"] != "YES" || growthNullable["weight_kg"] != "YES" {
			t.Fatalf("growth height/weight nullable = %s/%s, want YES/YES",
				growthNullable["height_cm"], growthNullable["weight_kg"])
		}
		if _, err := pool.Exec(ctx, `INSERT INTO growth_records (member_id, measured_on, source)
			VALUES (1, '2026-10-01', 'measured')`); err == nil {
			t.Fatal("growth record without height/weight accepted, want CHECK rejection")
		} else if !strings.Contains(err.Error(), "growth_records_measurement_present") {
			t.Fatalf("expected measurement check violation, got %v", err)
		}
		if _, err := pool.Exec(ctx, `INSERT INTO growth_records
			(member_id, measured_on, height_cm, source) VALUES (1, '2026-10-01', 118.5, 'scale')`); err == nil {
			t.Fatal("invalid growth source accepted, want CHECK rejection")
		} else if !strings.Contains(err.Error(), "growth_records_source_check") {
			t.Fatalf("expected source check violation, got %v", err)
		}

		var analysisID int64
		if err := pool.QueryRow(ctx, `INSERT INTO analyses
			(member_id, panel, entry, window_months, status, layers, snapshot)
			VALUES (1, 'liver', 'panel', 12, 'success', '{"facts":{}}', '{}')
			RETURNING id`).Scan(&analysisID); err != nil {
			t.Fatalf("seed analysis: %v", err)
		}
		if _, err := pool.Exec(ctx, `INSERT INTO analyses
			(member_id, panel, entry, window_months, status, layers, snapshot)
			VALUES (1, 'liver', 'report', 12, 'success', '{}', '{}')`); err == nil {
			t.Fatal("invalid analysis entry accepted, want CHECK rejection")
		} else if !strings.Contains(err.Error(), "analyses_entry_check") {
			t.Fatalf("expected entry check violation, got %v", err)
		}
		if _, err := pool.Exec(ctx, `INSERT INTO analysis_feedback
			(analysis_id, reason) VALUES ($1, '')`, analysisID); err == nil {
			t.Fatal("empty feedback reason accepted, want CHECK rejection")
		} else if !strings.Contains(err.Error(), "analysis_feedback_reason_check") {
			t.Fatalf("expected feedback reason check violation, got %v", err)
		}
		if _, err := pool.Exec(ctx, `INSERT INTO analysis_events
			(analysis_id, kind) VALUES ($1, 'bogus')`, analysisID); err == nil {
			t.Fatal("unknown event kind accepted, want CHECK rejection")
		} else if !strings.Contains(err.Error(), "analysis_events_kind_check") {
			t.Fatalf("expected event kind check violation, got %v", err)
		}
		if _, err := pool.Exec(ctx, `INSERT INTO analysis_events
			(analysis_id, kind) VALUES ($1, 'llm_timeout')`, analysisID); err != nil {
			t.Fatalf("seed event: %v", err)
		}
		if _, err := pool.Exec(ctx, `INSERT INTO analysis_llm_calls (analysis_id) VALUES ($1)`, analysisID); err != nil {
			t.Fatalf("seed llm call: %v", err)
		}
		if _, err := pool.Exec(ctx, `INSERT INTO analysis_explanation_cache
			(member_id, cache_key, explanation) VALUES (1, 'key', '{}')`); err != nil {
			t.Fatalf("seed explanation cache: %v", err)
		}
	})

	t.Run("member delete cascades owned data", func(t *testing.T) {
		if _, err := pool.Exec(ctx, `DELETE FROM members WHERE id = 1`); err != nil {
			t.Fatalf("delete member: %v", err)
		}
		var growth, analyses, feedback, events, calls, cached, ownedReports int
		if err := pool.QueryRow(ctx, `SELECT
			(SELECT count(*) FROM growth_records), (SELECT count(*) FROM analyses),
			(SELECT count(*) FROM analysis_feedback), (SELECT count(*) FROM analysis_events),
			(SELECT count(*) FROM analysis_llm_calls),
			(SELECT count(*) FROM analysis_explanation_cache),
			(SELECT count(*) FROM reports WHERE member_id = 1)`).
			Scan(&growth, &analyses, &feedback, &events, &calls, &cached, &ownedReports); err != nil {
			t.Fatal(err)
		}
		if growth != 0 || analyses != 0 || feedback != 0 || events != 0 || calls != 0 ||
			cached != 0 || ownedReports != 0 {
			t.Fatalf("after member delete: growth=%d analyses=%d feedback=%d events=%d calls=%d cached=%d reports=%d, want all 0",
				growth, analyses, feedback, events, calls, cached, ownedReports)
		}
		// 无主存量报告不受成员删除影响。
		var unowned int
		if err := pool.QueryRow(ctx, `SELECT count(*) FROM reports WHERE image_sha256='b'`).Scan(&unowned); err != nil {
			t.Fatal(err)
		}
		if unowned != 1 {
			t.Fatalf("unowned report rows = %d, want 1", unowned)
		}
	})
}
