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

	sqlItemsByReport = `SELECT name, value, unit, ref_range, method, flag, low_confidence, raw_text
		 FROM report_items WHERE report_sha256 = $1 ORDER BY id`

	// trend:同一指标项名(report_items.name = 词典规范名)的跨报告时序点,升序;
	// low_confidence 为项级质量信号(决策 #7),CLI 默认排除不可信点(--all 关闭)。
	sqlTrendByName = `SELECT r.image_sha256, r.report_type,
		   COALESCE(to_char(r.report_date, 'YYYY-MM-DD'), ''), i.value, i.unit, i.flag, i.low_confidence
		 FROM report_items i JOIN reports r ON r.image_sha256 = i.report_sha256
		 WHERE i.name = $1
		 ORDER BY r.report_date ASC, r.image_sha256 ASC`

	sqlTrendByNameAndType = `SELECT r.image_sha256, r.report_type,
		   COALESCE(to_char(r.report_date, 'YYYY-MM-DD'), ''), i.value, i.unit, i.flag, i.low_confidence
		 FROM report_items i JOIN reports r ON r.image_sha256 = i.report_sha256
		 WHERE i.name = $1 AND r.report_type = $2
		 ORDER BY r.report_date ASC, r.image_sha256 ASC`

	// 报告摘要(serve 时间线/上传响应回读;票 06):flag 聚合与未核对低置信计数
	// 响应派生、不落列(ADR-0005)——unreviewed 只统计 partial 报告,失败/免核对恒 0。
	sqlTimeline = `SELECT ` + reportSummaryColumnsSQL + ` FROM reports r
		 LEFT JOIN LATERAL (` + reportAggregateSQL + `) agg ON TRUE
		 WHERE ($1::bigint IS NULL OR r.member_id = $1)
		 ORDER BY r.report_date DESC NULLS LAST, r.image_sha256 ASC`

	sqlReportSummaryByImage = `SELECT ` + reportSummaryColumnsSQL + ` FROM reports r
		 LEFT JOIN LATERAL (` + reportAggregateSQL + `) agg ON TRUE
		 WHERE r.image_sha256 = $1`

	// 指标项含外部句柄与核对状态(上传响应回读;行序 = id 升序与写入序一致)。
	// ocr_line_numbers 存量行可为 NULL(0005 前入库),COALESCE 归空数组。
	sqlItemsWithReviewStatus = `SELECT id, name, value, unit, ref_range, method, flag, low_confidence, raw_text, review_status,
		   COALESCE(ocr_line_numbers, '{}')
		 FROM report_items WHERE report_sha256 = $1 ORDER BY id`

	// 报告所用 OCR 结果(详情行链接回读,票 07;reports.ocr_result_id 指向生成该报告的识别历史行)。
	sqlOCRResultByReport = `SELECT o.engine_output FROM reports r
		 JOIN ocr_results o ON o.id = r.ocr_result_id
		 WHERE r.image_sha256 = $1`

	// 归属只写 member_id(failed/success/识别历史不动;改归属端点同用)。
	sqlReportSetMember = `UPDATE reports SET member_id = $2, updated_at = now()
		 WHERE image_sha256 = $1`

	sqlOCRResultByID = `SELECT engine_output FROM ocr_results WHERE id = $1`

	// 报告行 upsert:测定 identity = 图像 sha256(每图一行);冲突即原位更新(--force 重跑)。
	// fail_reason 全量更新(pipeline 产出,非 failed 一律 NULL:重算复原时旧原因清除)。
	sqlReportUpsert = `INSERT INTO reports (image_sha256, ocr_result_id, report_type, report_date, status, fail_reason)
		 VALUES ($1,$2,$3,$4::date,$5,$6)
		 ON CONFLICT (image_sha256) DO UPDATE SET
		     ocr_result_id = excluded.ocr_result_id,
		     report_type   = excluded.report_type,
		     report_date   = excluded.report_date,
		     status        = excluded.status,
		     fail_reason   = excluded.fail_reason,
		     updated_at    = now()`

	sqlItemsDeleteByReport = `DELETE FROM report_items WHERE report_sha256 = $1`

	sqlItemInsert = `INSERT INTO report_items
		 (report_sha256, name, value, unit, ref_range, method, flag, low_confidence, raw_text, ocr_line_numbers)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`

	// 成员(迁移 0004;显示名 + 是否孩子两档,无删除):列表按 id 升序 = 创建序返回,
	// 孩子优先由客户端做;改动用 COALESCE 只覆盖传入字段(nil = 保持不变)。
	sqlMemberInsert = `INSERT INTO members (name, is_child) VALUES ($1,$2)
		 RETURNING id, created_at`

	sqlMembersList = `SELECT id, name, is_child, created_at FROM members ORDER BY id`

	sqlMemberByID = `SELECT id, name, is_child, created_at FROM members WHERE id = $1`

	sqlMemberUpdate = `UPDATE members SET name = COALESCE($2, name), is_child = COALESCE($3, is_child)
		 WHERE id = $1 RETURNING id, name, is_child, created_at`
)

// reportColumnsSQL 报告行到业务视图(ReportRow)的标准投影;
// report_date 序列化为 YYYY-MM-DD(缺失为空串),items 计数内联聚合。
const reportColumnsSQL = `r.image_sha256, r.ocr_result_id, r.report_type,
	   COALESCE(to_char(r.report_date, 'YYYY-MM-DD'), ''), r.status,
	   COALESCE((SELECT COUNT(*) FROM report_items i WHERE i.report_sha256 = r.image_sha256), 0)`

// reportSummaryColumnsSQL 报告摘要(ReportSummary)投影:头部 + 聚合列(见 reportAggregateSQL)。
const reportSummaryColumnsSQL = `r.image_sha256, r.member_id, r.report_type,
	   COALESCE(to_char(r.report_date, 'YYYY-MM-DD'), ''), r.status, r.fail_reason,
	   agg.high, agg.low, agg.unknown, agg.unreviewed_low`

// reportAggregateSQL 单报告的 flag 聚合与未核对低置信计数(横向子查询,逐报告就地聚合)。
// unreviewed_low 以报告 status = partial 为门:success 免核对、failed 无项均恒 0(契约口径)。
const reportAggregateSQL = `SELECT
		   COUNT(*) FILTER (WHERE i.flag = 'high') AS high,
		   COUNT(*) FILTER (WHERE i.flag = 'low') AS low,
		   COUNT(*) FILTER (WHERE i.flag = 'unknown') AS unknown,
		   COUNT(*) FILTER (WHERE r.status = 'partial' AND i.review_status = 'unreviewed' AND i.low_confidence) AS unreviewed_low
	   FROM report_items i WHERE i.report_sha256 = r.image_sha256`

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
		if err := rows.Scan(&it.Name, &it.Value, &it.Unit, &it.RefRange, &it.Method, &it.Flag, &it.LowConfidence, &it.RawText); err != nil {
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

// TrendByName hdd trend <指标名>:同一指标项名跨报告时序点(升序,--type 过滤类别,
// 输入缩写经 CLI 的 domType 归一为域值);like list,组合均为常量 SQL。
func (s *Store) TrendByName(ctx context.Context, itemName, reportType string) ([]pipeline.TrendRow, error) {
	if reportType != "" {
		return s.trendRows(ctx, sqlTrendByNameAndType, itemName, reportType)
	}
	return s.trendRows(ctx, sqlTrendByName, itemName)
}

func (s *Store) trendRows(ctx context.Context, query string, args ...any) ([]pipeline.TrendRow, error) {
	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []pipeline.TrendRow
	for rows.Next() {
		var r pipeline.TrendRow
		var date string
		if err := rows.Scan(&r.Sha256, &r.ReportType, &date, &r.Value, &r.Unit, &r.Flag, &r.LowConfidence); err != nil {
			return nil, err
		}
		if date != "" {
			r.ReportDate = &date
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// ListTimeline 时间线摘要(serve GET /reports;viz-and-client 票 06):memberID nil =
// 全部报告(含存量无主);排序日期降序、dateless 沉底,同日期按 sha 升序(客户端展示再排一次)。
func (s *Store) ListTimeline(ctx context.Context, memberID *int64) ([]pipeline.ReportSummary, error) {
	return s.querySummaries(ctx, sqlTimeline, memberID)
}

// SummaryByImage 单份报告摘要(上传响应回读;缺失返回 nil, nil)。
func (s *Store) SummaryByImage(ctx context.Context, imageSha string) (*pipeline.ReportSummary, error) {
	rows, err := s.querySummaries(ctx, sqlReportSummaryByImage, imageSha)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, nil
	}
	return &rows[0], nil
}

// querySummaries 摘要投影的公共扫描(时间线与单报告查询共用同一专栏集)。
func (s *Store) querySummaries(ctx context.Context, query string, args ...any) ([]pipeline.ReportSummary, error) {
	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []pipeline.ReportSummary
	for rows.Next() {
		var r pipeline.ReportSummary
		var date string
		if err := rows.Scan(&r.Sha256, &r.MemberID, &r.ReportType, &date, &r.Status, &r.FailReason,
			&r.FlagCounts.High, &r.FlagCounts.Low, &r.FlagCounts.Unknown, &r.UnreviewedLowConfidenceCount); err != nil {
			return nil, err
		}
		if date != "" {
			r.ReportDate = &date
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// ItemsWithReviewStatus 指标项含外部句柄与核对状态(上传响应回读;id 升序,票 06)。
func (s *Store) ItemsWithReviewStatus(ctx context.Context, reportSha string) ([]pipeline.ReportItemRow, error) {
	rows, err := s.pool.Query(ctx, sqlItemsWithReviewStatus, reportSha)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []pipeline.ReportItemRow
	for rows.Next() {
		var it pipeline.ReportItemRow
		if err := rows.Scan(&it.ItemID, &it.Name, &it.Value, &it.Unit, &it.RefRange, &it.Method,
			&it.Flag, &it.LowConfidence, &it.RawText, &it.ReviewStatus, &it.OCRLineNumbers); err != nil {
			return nil, err
		}
		out = append(out, it)
	}
	return out, rows.Err()
}

// OCRResultJSONByReport 报告所用 OCR 结果的原始 JSON(详情行链接回读,票 07):
// reports.ocr_result_id → ocr_results.engine_output;报告缺失返回 nil, nil。
func (s *Store) OCRResultJSONByReport(ctx context.Context, reportSha string) ([]byte, error) {
	var raw []byte
	err := s.pool.QueryRow(ctx, sqlOCRResultByReport, reportSha).Scan(&raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return raw, nil
}

// SetReportMember 只改报告归属(上传新建/无主归档时赋值,票 06;改归属端点同用)。
// 报告不存在返回错误(不静默虚报成功)。
func (s *Store) SetReportMember(ctx context.Context, reportSha string, memberID int64) error {
	tag, err := s.pool.Exec(ctx, sqlReportSetMember, reportSha, memberID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("report %s not found", reportSha)
	}
	return nil
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
// 镜像校验(viz-and-client 票 03 双保险):空指标项⇒failed——绕过识别端直灌
// 的违规形状(status=success/partial 且零 items)在此拒绝,不写库。
func (s *Store) WriteReport(ctx context.Context, reportSha string, ocrID int64, report contract.Report, failReason *string) error {
	if len(report.Items) == 0 && report.Status != contract.StatusFailed {
		return fmt.Errorf("report invariant violated: %d items with status %q — report with no items must be failed", len(report.Items), report.Status)
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, sqlReportUpsert,
		reportSha, ocrID, report.ReportType, report.ReportDate, report.Status, failReason); err != nil {
		return fmt.Errorf("upsert report: %w", err)
	}
	if _, err := tx.Exec(ctx, sqlItemsDeleteByReport, reportSha); err != nil {
		return err
	}
	for _, it := range report.Items {
		if _, err := tx.Exec(ctx, sqlItemInsert,
			reportSha, it.Name, it.Value, it.Unit, it.RefRange, it.Method, it.Flag, it.LowConfidence, it.RawText,
			it.OCRLineNumbers); err != nil {
			return fmt.Errorf("write report item %q: %w", it.Name, err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return err
	}
	return nil
}

// CreateMember 建成员(name + is_child 两档;id/created_at 由库产生,viz-and-client 票 05)。
// 名字过领域规则镜像(HTTP 边界已校验;直连存储的路径在此兜底,票 03 双保险惯例)。
func (s *Store) CreateMember(ctx context.Context, name string, isChild bool) (*pipeline.Member, error) {
	if err := pipeline.ValidateMemberName(name); err != nil {
		return nil, err
	}
	m := pipeline.Member{Name: name, IsChild: isChild}
	if err := s.pool.QueryRow(ctx, sqlMemberInsert, name, isChild).Scan(&m.MemberID, &m.CreatedAt); err != nil {
		return nil, err
	}
	return &m, nil
}

// ListMembers 全量成员按创建序(id 升序);孩子优先由客户端做(契约排序口径)。
func (s *Store) ListMembers(ctx context.Context) ([]pipeline.Member, error) {
	rows, err := s.pool.Query(ctx, sqlMembersList)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []pipeline.Member
	for rows.Next() {
		var m pipeline.Member
		if err := rows.Scan(&m.MemberID, &m.Name, &m.IsChild, &m.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// MemberByID 按 id 取成员(serve 上传归属校验,票 06);nil, nil = 不存在。
func (s *Store) MemberByID(ctx context.Context, id int64) (*pipeline.Member, error) {
	var m pipeline.Member
	err := s.pool.QueryRow(ctx, sqlMemberByID, id).Scan(&m.MemberID, &m.Name, &m.IsChild, &m.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &m, nil
}

// UpdateMember 只更新传入字段(至少一项由 HTTP 层校验);nil,nil = 成员不存在
// (与 ImageBySha256 的缺失口径一致)。名字过领域规则镜像(同 CreateMember)。
func (s *Store) UpdateMember(ctx context.Context, id int64, name *string, isChild *bool) (*pipeline.Member, error) {
	if name != nil {
		if err := pipeline.ValidateMemberName(*name); err != nil {
			return nil, err
		}
	}
	var m pipeline.Member
	err := s.pool.QueryRow(ctx, sqlMemberUpdate, id, name, isChild).
		Scan(&m.MemberID, &m.Name, &m.IsChild, &m.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &m, nil
}
