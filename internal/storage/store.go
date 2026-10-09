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
// MVP 手写而未用 sqlc 生成:查询量小、monkey 无参数化面;契约字段在 pipeline 层已固化。
type Store struct {
	pool *pgxpool.Pool
}

// NewStore 包装已打开(且完成迁移)的连接池。
func NewStore(pool *pgxpool.Pool) *Store { return &Store{pool: pool} }

// Close 关闭连接池。
func (s *Store) Close() { s.pool.Close() }

func (s *Store) ImageBySha256(ctx context.Context, sha string) (*pipeline.Image, error) {
	var img pipeline.Image
	err := s.pool.QueryRow(ctx,
		`SELECT id, sha256, object_key, original_filename FROM images WHERE sha256 = $1`, sha).
		Scan(&img.ID, &img.Sha256, &img.ObjectKey, &img.OriginalFilename)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &img, nil
}

func (s *Store) CreateImage(ctx context.Context, sha, objectKey, filename string) (int64, error) {
	var id int64
	err := s.pool.QueryRow(ctx,
		`INSERT INTO images (sha256, object_key, original_filename) VALUES ($1,$2,$3) RETURNING id`,
		sha, objectKey, filename).Scan(&id)
	return id, err
}

func (s *Store) AppendOCRResult(ctx context.Context, imageID int64, raw []byte) (int64, error) {
	var id int64
	err := s.pool.QueryRow(ctx,
		`INSERT INTO ocr_results (image_id, engine_output) VALUES ($1,$2) RETURNING id`,
		imageID, raw).Scan(&id)
	return id, err
}

const reportColumns = `r.id, r.image_id, r.ocr_result_id, r.report_type,
	   COALESCE(to_char(r.report_date, 'YYYY-MM-DD'), ''), r.status,
	   COALESCE((SELECT COUNT(*) FROM report_items i WHERE i.report_id = r.id), 0)`

func (s *Store) queryReports(ctx context.Context, where string, args ...any) ([]pipeline.ReportRow, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT `+reportColumns+` FROM reports r `+where, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []pipeline.ReportRow
	for rows.Next() {
		var r pipeline.ReportRow
		var date string
		if err := rows.Scan(&r.ID, &r.ImageID, &r.OCRResultID, &r.ReportType, &date, &r.Status, &r.ItemsCount); err != nil {
			return nil, err
		}
		if date != "" {
			r.ReportDate = &date
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *Store) ReportByImage(ctx context.Context, imageID int64) (*pipeline.ReportRow, bool, error) {
	rows, err := s.queryReports(ctx, `WHERE r.image_id = $1 LIMIT 1`, imageID)
	if err != nil {
		return nil, false, err
	}
	if len(rows) == 0 {
		return nil, false, nil
	}
	return &rows[0], true, nil
}

func (s *Store) ReportByID(ctx context.Context, id int64) (*pipeline.ReportRow, error) {
	rows, err := s.queryReports(ctx, `WHERE r.id = $1`, id)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("report %d: %w", id, pgx.ErrNoRows)
	}
	return &rows[0], nil
}

func (s *Store) ListReports(ctx context.Context, reportType, date string) ([]pipeline.ReportRow, error) {
	where, args := "WHERE true", []any{}
	if reportType != "" {
		where += fmt.Sprintf(" AND r.report_type = $%d", len(args)+1)
		args = append(args, reportType)
	}
	if date != "" {
		where += fmt.Sprintf(" AND r.report_date = $%d::date", len(args)+1)
		args = append(args, date)
	}
	where += " ORDER BY r.report_date DESC NULLS LAST, r.id DESC"
	return s.queryReports(ctx, where, args...)
}

func (s *Store) ItemsByReport(ctx context.Context, reportID int64) ([]contract.ReportItem, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT name, value, unit, ref_range, flag, low_confidence, raw_text
		 FROM report_items WHERE report_id = $1 ORDER BY id`, reportID)
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
	err := s.pool.QueryRow(ctx,
		`SELECT engine_output FROM ocr_results WHERE id = $1`, ocrID).Scan(&raw)
	return raw, err
}

// WriteReport 落/更一份报告(items 取自 report.Items,事务式全量替换)。
// reportID=0 表示新建;>0 表示更新(如 --force 重跑)。report_date 需为 YYYY-MM-DD 或缺失。
func (s *Store) WriteReport(ctx context.Context, reportID, imageID, ocrID int64, report contract.Report) (int64, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)

	var id int64
	if reportID == 0 {
		err = tx.QueryRow(ctx,
			`INSERT INTO reports (image_id, ocr_result_id, report_type, report_date, status)
			 VALUES ($1,$2,$3,$4::date,$5) RETURNING id`,
			imageID, ocrID, report.ReportType, report.ReportDate, report.Status).Scan(&id)
	} else {
		err = tx.QueryRow(ctx,
			`UPDATE reports SET ocr_result_id=$2, report_type=$3, report_date=$4::date, status=$5, updated_at=now()
			 WHERE id=$1 RETURNING id`,
			reportID, ocrID, report.ReportType, report.ReportDate, report.Status).Scan(&id)
	}
	if err != nil {
		return 0, fmt.Errorf("write report: %w", err)
	}
	if _, err := tx.Exec(ctx, `DELETE FROM report_items WHERE report_id = $1`, id); err != nil {
		return 0, err
	}
	for _, it := range report.Items {
		if _, err := tx.Exec(ctx,
			`INSERT INTO report_items (report_id, name, value, unit, ref_range, flag, low_confidence, raw_text)
			 VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`,
			id, it.Name, it.Value, it.Unit, it.RefRange, it.Flag, it.LowConfidence, it.RawText); err != nil {
			return 0, fmt.Errorf("write report item %q: %w", it.Name, err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, err
	}
	return id, nil
}
