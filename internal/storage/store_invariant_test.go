// WriteReport 镜像校验冒烟(viz-and-client 票 03 双保险):零指标项 ⇒ failed——
// 绕过识别端直灌的违规形状(success/partial 空指标项)在存储层拒绝不入库;
// fail_reason 随 upsert 全量写(仅 failed 有值,重算复原时清除)。
// 需要真库:设 HDD_TEST_DSN 后运行,未设整体 Skip(CI 默认跳过);与 migrate_test
// 用一次性独立容器,种子隔离勿共用库:
// docker run --name hdd-invariant-smoke-pg -e POSTGRES_PASSWORD=smoke -p 55433:5432 postgres:16-alpine

package storage

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"healthyduoduo/internal/contract"
)

func invariantItem() contract.ReportItem {
	return contract.ReportItem{Name: "血红蛋白", Flag: "normal", RawText: "血红蛋白 128 g/L"}
}

func invariantStatusAndReason(t *testing.T, pool *pgxpool.Pool, sha string) (string, *string) {
	t.Helper()
	var status string
	var reason *string
	if err := pool.QueryRow(context.Background(),
		`SELECT status, fail_reason FROM reports WHERE image_sha256 = $1`, sha).
		Scan(&status, &reason); err != nil {
		t.Fatalf("read report row: %v", err)
	}
	return status, reason
}

func TestWriteReportEmptyItemsInvariantSmoke(t *testing.T) {
	dsn := os.Getenv("HDD_TEST_DSN")
	if dsn == "" {
		t.Skip("HDD_TEST_DSN not set: invariant smoke runs against a disposable PG container")
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
	if _, err := pool.Exec(ctx,
		`INSERT INTO images (sha256, original_filename) VALUES ('inv-a', 'p.jpg');
		 INSERT INTO ocr_results (image_sha256, engine_output) VALUES ('inv-a', '{}')`); err != nil {
		t.Fatalf("seed: %v", err)
	}
	st := NewStore(pool)
	defer st.Close()

	// 直灌违规(success/partial + 零指标项)→ 拒绝且不留痕。
	for _, status := range []string{contract.StatusSuccess, contract.StatusPartial} {
		bogus := contract.Report{ReportType: "血常规", Status: status}
		if err := st.WriteReport(ctx, "inv-a", 1, bogus, nil); err == nil {
			t.Fatalf("empty items with status %q accepted, want invariant rejection", status)
		} else if !strings.Contains(err.Error(), "report with no items must be failed") {
			t.Fatalf("empty items with status %q: expect invariant error, got %v", status, err)
		}
		var n int
		if err := pool.QueryRow(ctx, `SELECT count(*) FROM reports WHERE image_sha256 = 'inv-a'`).Scan(&n); err != nil {
			t.Fatal(err)
		}
		if n != 0 {
			t.Fatalf("rejected write left %d report rows, want 0", n)
		}
	}

	// 真实 failed-empty(拍糊/空白图)+ 失败原因 → 落库留痕。
	reason := "no metric items recognized"
	failed := contract.Report{ReportType: "其他专项", Status: contract.StatusFailed}
	if err := st.WriteReport(ctx, "inv-a", 1, failed, &reason); err != nil {
		t.Fatalf("failed-empty write: %v", err)
	}
	gotStatus, gotReason := invariantStatusAndReason(t, pool, "inv-a")
	if gotStatus != contract.StatusFailed || gotReason == nil || *gotReason != reason {
		t.Fatalf("stored after failed write: status=%q fail_reason=%v", gotStatus, gotReason)
	}

	// 规则迭代复原(有产出)→ 原因清除,fail_reason 回 NULL。
	recovered := contract.Report{ReportType: "血常规", Status: contract.StatusSuccess,
		Items: []contract.ReportItem{invariantItem()}}
	if err := st.WriteReport(ctx, "inv-a", 1, recovered, nil); err != nil {
		t.Fatalf("recovered write: %v", err)
	}
	gotStatus, gotReason = invariantStatusAndReason(t, pool, "inv-a")
	if gotStatus != contract.StatusSuccess || gotReason != nil {
		t.Fatalf("stored after recovery: status=%q fail_reason=%v, want success/NULL", gotStatus, gotReason)
	}
}
