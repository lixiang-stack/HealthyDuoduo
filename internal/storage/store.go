package storage

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"healthyduoduo/internal/contract"
	"healthyduoduo/internal/pipeline"
)

// Store 是 pipeline.Store 的 PG 实现(手写 pgx/v5;表 DDL 见 migrations/0001_init.sql)。
// MVP 手写而未用 sqlc 生成:查询量小;所有 SQL 集中为本文件常量,用户值一律走参数占位
// ($1..),无字符串拼接注入面。
type Store struct {
	pool *pgxpool.Pool
}

// NewStore 包装已打开(且完成迁移)的连接池。
func NewStore(pool *pgxpool.Pool) *Store { return &Store{pool: pool} }

// Close 关闭连接池。
func (s *Store) Close() { s.pool.Close() }

// 语义化 SQL 常量;占位仅传参,未拼接任何用户值(无注入面)。
const (
	sqlImageBySHA256 = `SELECT sha256, original_filename
		FROM images WHERE sha256 = $1`

	sqlImageInsert = `INSERT INTO images (sha256, original_filename) VALUES ($1,$2)`

	sqlOCRResultInsert = `INSERT INTO ocr_results (image_sha256, engine_output)
		 VALUES ($1,$2) RETURNING id`

	sqlReportsByImage = `SELECT ` + reportColumnsSQL + ` FROM reports r
		 WHERE r.image_sha256 = $1`

	sqlReportListNoFilter = `SELECT ` + reportColumnsSQL + ` FROM reports r
		 ORDER BY r.report_date DESC NULLS LAST, r.image_sha256 ASC`

	sqlReportListByType = `SELECT ` + reportColumnsSQL + ` FROM reports r
		 WHERE r.report_type = $1
		 ORDER BY r.report_date DESC NULLS LAST, r.image_sha256 ASC`

	sqlReportListByExactDate = `SELECT ` + reportColumnsSQL + ` FROM reports r
		 WHERE r.report_date = $1::date
		 ORDER BY r.report_date DESC NULLS LAST, r.image_sha256 ASC`

	sqlReportListByTypeAndExactDate = `SELECT ` + reportColumnsSQL + ` FROM reports r
		 WHERE r.report_type = $1 AND r.report_date = $2::date
		 ORDER BY r.report_date DESC NULLS LAST, r.image_sha256 ASC`

	sqlReportListByMonth = `SELECT ` + reportColumnsSQL + ` FROM reports r
		 WHERE date_trunc('month', r.report_date) = date_trunc('month', $1::date)
		 ORDER BY r.report_date DESC NULLS LAST, r.image_sha256 ASC`

	sqlReportListByYear = `SELECT ` + reportColumnsSQL + ` FROM reports r
		 WHERE date_trunc('year', r.report_date) = date_trunc('year', $1::date)
		 ORDER BY r.report_date DESC NULLS LAST, r.image_sha256 ASC`

	sqlReportListByTypeAndMonth = `SELECT ` + reportColumnsSQL + ` FROM reports r
		 WHERE r.report_type = $1 AND date_trunc('month', r.report_date) = date_trunc('month', $2::date)
		 ORDER BY r.report_date DESC NULLS LAST, r.image_sha256 ASC`

	sqlReportListByTypeAndYear = `SELECT ` + reportColumnsSQL + ` FROM reports r
		 WHERE r.report_type = $1 AND date_trunc('year', r.report_date) = date_trunc('year', $2::date)
		 ORDER BY r.report_date DESC NULLS LAST, r.image_sha256 ASC`

	sqlItemsByReport = `SELECT name, value, unit, ref_range, flag, low_confidence, raw_text
		 FROM report_items WHERE report_sha256 = $1 ORDER BY id`

	sqlOCRResultByID = `SELECT engine_output FROM ocr_results WHERE id = $1`

	// 报告行 upsert:测定 identity = 图像 sha256(每图一行);冲突即原位更新(--force 重跑)。
	sqlReportUpsert = `INSERT INTO reports (image_sha256, ocr_result_id, report_type, report_date, status)
		 VALUES ($1,$2,$3,$4::date,$5)
		 ON CONFLICT (image_sha256) DO UPDATE SET
		     ocr_result_id = excluded.ocr_result_id,
		     report_type   = excluded.report_type,
		     report_date   = excluded.report_date,
		     status        = excluded.status,
		     updated_at    = now()`

	sqlItemsDeleteByReport = `DELETE FROM report_items WHERE report_sha256 = $1`

	sqlItemInsert = `INSERT INTO report_items
		 (report_sha256, name, value, unit, ref_range, flag, low_confidence, raw_text)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`
)

// reportColumnsSQL 报告行到业务视图(ReportRow)的标准投影;
// report_date 序列化为 YYYY-MM-DD(缺失为空串),items 计数内联聚合。
const reportColumnsSQL = `r.image_sha256, r.ocr_result_id, r.report_type,
	   COALESCE(to_char(r.report_date, 'YYYY-MM-DD'), ''), r.status,
	   COALESCE((SELECT COUNT(*) FROM report_items i WHERE i.report_sha256 = r.image_sha256), 0)`

func (s *Store) ImageBySha256(ctx context.Context, sha string) (*pipeline.Image, error) {
	var img pipeline.Image
	err := s.pool.QueryRow(ctx, sqlImageBySHA256, sha).
		Scan(&img.Sha256, &img.OriginalFilename)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &img, nil
}

// ImagesByShaPrefix 已废弃移除(身份只接受完整 64 hex)。

func (s *Store) CreateImage(ctx context.Context, sha, filename string) error {
	_, err := s.pool.Exec(ctx, sqlImageInsert, sha, filename)
	return err
}

func (s *Store) AppendOCRResult(ctx context.Context, imageSha string, raw []byte) (int64, error) {
	var id int64
	err := s.pool.QueryRow(ctx, sqlOCRResultInsert, imageSha, raw).Scan(&id)
	return id, err
}

func (s *Store) queryReports(ctx context.Context, query string, args ...any) ([]pipeline.ReportRow, error) {
	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []pipeline.ReportRow
	for rows.Next() {
		var r pipeline.ReportRow
		var date string
		if err := rows.Scan(&r.Sha256, &r.OCRResultID, &r.ReportType, &date, &r.Status, &r.ItemsCount); err != nil {
			return nil, err
		}
		if date != "" {
			r.ReportDate = &date
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *Store) ReportByImage(ctx context.Context, imageSha string) (*pipeline.ReportRow, bool, error) {
	rows, err := s.queryReports(ctx, sqlReportsByImage, imageSha)
	if err != nil {
		return nil, false, err
	}
	if len(rows) == 0 {
		return nil, false, nil
	}
	return &rows[0], true, nil
}

// ListReports list [--type/--date] 查询;--date 支持 YYYY / YYYY-MM / YYYY-MM-DD
// 三种粒度(月/年经 date_trunc 归一),组合均为常量 SQL,无动态拼接。
func (s *Store) ListReports(ctx context.Context, reportType string, date string) ([]pipeline.ReportRow, error) {
	grain := dateGranularity(date)
	switch {
	case grain == dateNone && reportType != "":
		return s.queryReports(ctx, sqlReportListByType, reportType)
	case grain == dateExact:
		if reportType != "" {
			return s.queryReports(ctx, sqlReportListByTypeAndExactDate, reportType, date)
		}
		return s.queryReports(ctx, sqlReportListByExactDate, date)
	case grain == dateMonth:
		first := date + "-01"
		if reportType != "" {
			return s.queryReports(ctx, sqlReportListByTypeAndMonth, reportType, first)
		}
		return s.queryReports(ctx, sqlReportListByMonth, first)
	case grain == dateYear:
		first := date + "-01-01"
		if reportType != "" {
			return s.queryReports(ctx, sqlReportListByTypeAndYear, reportType, first)
		}
		return s.queryReports(ctx, sqlReportListByYear, first)
	}
	return s.queryReports(ctx, sqlReportListNoFilter)
}

func (s *Store) ItemsByImage(ctx context.Context, reportSha string) ([]contract.ReportItem, error) {
	rows, err := s.pool.Query(ctx, sqlItemsByReport, reportSha)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []contract.ReportItem
	for rows.Next() {
		var it contract.ReportItem
		if err := rows.Scan(&it.Name, &it.Value, &it.Unit, &it.RefRange, &it.Flag, &it.LowConfidence, &it.RawText); err != nil {
			return nil, err
		}
		items = append(items, it)
	}
	return items, rows.Err()
}

func (s *Store) OCRResultJSON(ctx context.Context, ocrID int64) ([]byte, error) {
	var raw []byte
	err := s.pool.QueryRow(ctx, sqlOCRResultByID, ocrID).Scan(&raw)
	return raw, err
}

// dateGranularity --date 输入的粒度;dateNone 表示未传日期。
const (
	dateNone  dateGrain = iota
	dateExact           // YYYY-MM-DD
	dateMonth           // YYYY-MM
	dateYear            // YYYY
)

type dateGrain int

// dateGranularity 判定 --date 字符串粒度(月/年由 date_trunc 在 SQL 里归一);
// 输入合法性由 CLI 校验,这里只做映射。
func dateGranularity(date string) dateGrain {
	switch {
	case date == "":
		return dateNone
	case len(date) == 4:
		return dateYear
	case len(date) == 7:
		return dateMonth
	default:
		return dateExact
	}
}

// WriteReport 落/更一份报告(items 取自 report.Items,事务式全量替换;
// 身份 = 图像 sha256,每图一行:首次插入/冲突原位更新,updated_at 恒刷新)。
func (s *Store) WriteReport(ctx context.Context, reportSha string, ocrID int64, report contract.Report) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, sqlReportUpsert,
		reportSha, ocrID, report.ReportType, report.ReportDate, report.Status); err != nil {
		return fmt.Errorf("upsert report: %w", err)
	}
	if _, err := tx.Exec(ctx, sqlItemsDeleteByReport, reportSha); err != nil {
		return err
	}
	for _, it := range report.Items {
		if _, err := tx.Exec(ctx, sqlItemInsert,
			reportSha, it.Name, it.Value, it.Unit, it.RefRange, it.Flag, it.LowConfidence, it.RawText); err != nil {
			return fmt.Errorf("write report item %q: %w", it.Name, err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return err
	}
	return nil
}
