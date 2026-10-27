package storage

import (
	"context"
	"encoding/json"
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
	pool       *pgxpool.Pool
	flagPolicy *pipeline.FlagPolicy
}

// NewStore 包装已打开(且完成迁移)的连接池。
func NewStore(pool *pgxpool.Pool) *Store { return &Store{pool: pool} }

// SetFlagPolicy 注入修正后 flag 重算用的共享比较策略(组合根注入;nil = 仅数值比较)。
func (s *Store) SetFlagPolicy(policy *pipeline.FlagPolicy) { s.flagPolicy = policy }

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

	sqlItemsByReport = `SELECT name, panel, value, unit, ref_range, method, flag, flag_reason, low_confidence, raw_text
		 FROM report_items WHERE report_sha256 = $1 ORDER BY id`

	// trend:同一指标项名(report_items.name = 词典规范名)的跨报告时序点,升序;
	// low_confidence 为项级质量信号(决策 #7),CLI 默认排除不可信点(--all 关闭)。
	sqlTrendByName = `SELECT r.image_sha256, r.report_type,
		   COALESCE(to_char(r.report_date, 'YYYY-MM-DD'), ''), i.value, i.unit, i.flag, i.low_confidence
		 FROM report_items i JOIN reports r ON r.image_sha256 = i.report_sha256
		 WHERE i.name = $1 AND i.flag <> 'not_tested'
		 ORDER BY r.report_date ASC, r.image_sha256 ASC`

	sqlTrendByNameAndType = `SELECT r.image_sha256, r.report_type,
		   COALESCE(to_char(r.report_date, 'YYYY-MM-DD'), ''), i.value, i.unit, i.flag, i.low_confidence
		 FROM report_items i JOIN reports r ON r.image_sha256 = i.report_sha256
		 WHERE i.name = $1 AND r.report_type = $2 AND i.flag <> 'not_tested'
		 ORDER BY r.report_date ASC, r.image_sha256 ASC`

	// trendPointsByMember:HTTP 成员维度趋势(票 02)。与 CLI 的 sqlTrendByName 是不同
	// 消费面(成员过滤 + 可选 unit + ref/method/review 字段);不合并,避免把 CLI 语义
	// 耦合进 serve。unit 空串 = 全部单位。
	sqlTrendPointsByMemberName = `SELECT r.image_sha256,
		   COALESCE(to_char(r.report_date, 'YYYY-MM-DD'), ''),
		   i.value, i.unit, i.ref_range, i.method, i.flag, i.low_confidence, i.review_status
		 FROM report_items i JOIN reports r ON r.image_sha256 = i.report_sha256
		 WHERE r.member_id = $1 AND i.name = $2 AND i.flag <> 'not_tested'
		   AND ($3 = '' OR i.unit = $3)
		 ORDER BY r.report_date ASC NULLS LAST, r.image_sha256 ASC, i.id ASC`

	// 报告摘要(serve 时间线/上传响应回读;票 06):flag 聚合与未核对低置信计数
	// 响应派生、不落列(ADR-0005)——unreviewed 统计未核对低置信项;success 报告
	// 同样可能有少量低置信项待核对,不再以 status=partial 为门。
	sqlTimeline = `SELECT ` + reportSummaryColumnsSQL + ` FROM reports r
		 LEFT JOIN LATERAL (` + reportAggregateSQL + `) agg ON TRUE
		 WHERE ($1::bigint IS NULL OR r.member_id = $1)
		 ORDER BY r.report_date DESC NULLS LAST, r.image_sha256 ASC`

	sqlReportSummaryByImage = `SELECT ` + reportSummaryColumnsSQL + ` FROM reports r
		 LEFT JOIN LATERAL (` + reportAggregateSQL + `) agg ON TRUE
		 WHERE r.image_sha256 = $1`

	// 指标项含外部句柄与核对状态(上传响应回读;行序 = id 升序与写入序一致)。
	// ocr_line_numbers 存量行可为 NULL(0005 前入库),COALESCE 归空数组。
	sqlItemsWithReviewStatus = `SELECT id, name, panel, value, unit, ref_range, method, flag, flag_reason, low_confidence, raw_text, review_status,
		   COALESCE(ocr_line_numbers, '{}')
		 FROM report_items WHERE report_sha256 = $1 ORDER BY id`

	// 报告所用 OCR 结果(详情行链接回读,票 07;reports.ocr_result_id 指向生成该报告的识别历史行)。
	sqlOCRResultByReport = `SELECT o.engine_output FROM reports r
		 JOIN ocr_results o ON o.id = r.ocr_result_id
		 WHERE r.image_sha256 = $1`

	// 报告级叙述段落(0006,ADR-0008):非叙述型报告/存量行为 NULL。
	sqlReportSectionsByImage = `SELECT sections FROM reports WHERE image_sha256 = $1`

	// 归属只写 member_id(failed/success/识别历史不动;改归属端点同用)。
	sqlReportSetMember = `UPDATE reports SET member_id = $2, updated_at = now()
		 WHERE image_sha256 = $1`

	// 硬删报告行(票 10;级联 report_items 与 correction_pairs,images/ocr_results 留痕)。
	sqlReportDelete = `DELETE FROM reports WHERE image_sha256 = $1`

	// 核对落库(票 09;单报告核对事务):报告行锁串行化该报告的并发核对,
	// 保证状态派生翻转看到的是最新已提交状态。
	sqlReportLock = `SELECT image_sha256 FROM reports WHERE image_sha256 = $1 FOR UPDATE`

	// 单指标项按报告内身份取行(核对动作前读;id 与报告不匹配即不存在)。
	sqlItemForReview = `SELECT id, name, panel, value, unit, ref_range, method, flag, flag_reason, low_confidence, raw_text, review_status,
		   COALESCE(ocr_line_numbers, '{}')
		 FROM report_items WHERE report_sha256 = $1 AND id = $2`

	sqlItemReviewConfirm = `UPDATE report_items SET review_status = 'confirmed' WHERE id = $1`

	sqlItemReviewNoReference = `UPDATE report_items SET flag_reason = 'no_reference', review_status = 'confirmed'
		 WHERE id = $1`

	sqlItemReviewCorrect = `UPDATE report_items SET name = $2, value = $3, unit = $4, ref_range = $5, flag = $6,
		   flag_reason = $7, review_status = 'corrected' WHERE id = $1`

	// 修正对 append-only(ADR-0005;before→after 四项纸面事实全集 + 原始行文 + OCR 行号)。
	sqlCorrectionPairInsert = `INSERT INTO correction_pairs
		 (item_id, before_name, before_value, before_unit, before_ref_range,
		  after_name, after_value, after_unit, after_ref_range, raw_text, ocr_line_numbers)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`

	// 状态派生翻转(ADR-0005):partial → success 当且仅当未核对低置信计数 = 0;
	// 单向(核对状态永不回 unreviewed,override 重跑会整写状态,不经此路径)。
	sqlReportDeriveSuccess = `UPDATE reports SET status = 'success', updated_at = now()
		 WHERE image_sha256 = $1 AND status = 'partial'
		   AND NOT EXISTS (SELECT 1 FROM report_items i
		                   WHERE i.report_sha256 = $1 AND i.review_status = 'unreviewed'
		                     AND i.low_confidence AND i.flag <> 'not_tested')`

	// 动作后报告级派生读出(状态 + 未核对低置信计数,与 reportAggregateSQL 同口径:
	// 统计未核对低置信项,not_tested 不计入;success 报告也如实计数)。
	sqlReportReviewState = `SELECT r.status,
		   (SELECT COUNT(*) FROM report_items i
		    WHERE i.report_sha256 = r.image_sha256
		      AND i.review_status = 'unreviewed' AND i.low_confidence
		      AND i.flag <> 'not_tested')
		 FROM reports r WHERE r.image_sha256 = $1`

	sqlOCRResultByID = `SELECT engine_output FROM ocr_results WHERE id = $1`

	// 报告行 upsert:测定 identity = 图像 sha256(每图一行);冲突即原位更新(--force 重跑)。
	// fail_reason 全量更新(pipeline 产出,非 failed 一律 NULL:重算复原时旧原因清除);
	// sections 全量覆盖(非叙述型报告写 NULL,重算复原时旧段落一并清除);
	// panels 全量覆盖(识别端有序聚合;nil 归一为空数组,存量未回填口径)。
	sqlReportUpsert = `INSERT INTO reports (image_sha256, ocr_result_id, report_type, report_date, status, fail_reason, sections, panels)
		 VALUES ($1,$2,$3,$4::date,$5,$6,$7::jsonb,$8)
		 ON CONFLICT (image_sha256) DO UPDATE SET
		     ocr_result_id = excluded.ocr_result_id,
		     report_type   = excluded.report_type,
		     report_date   = excluded.report_date,
		     status        = excluded.status,
		     fail_reason   = excluded.fail_reason,
		     sections      = excluded.sections,
		     panels        = excluded.panels,
		     updated_at    = now()`

	sqlItemsDeleteByReport = `DELETE FROM report_items WHERE report_sha256 = $1`

	sqlItemInsert = `INSERT INTO report_items
		 (report_sha256, name, panel, value, unit, ref_range, method, flag, flag_reason, low_confidence, raw_text, ocr_line_numbers)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)`

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
// report_date 序列化为 YYYY-MM-DD(缺失为空串),items 计数内联聚合,panels 有序数组。
const reportColumnsSQL = `r.image_sha256, r.ocr_result_id, r.report_type,
	   COALESCE(to_char(r.report_date, 'YYYY-MM-DD'), ''), r.status,
	   COALESCE((SELECT COUNT(*) FROM report_items i WHERE i.report_sha256 = r.image_sha256), 0),
	   r.panels`

// reportSummaryColumnsSQL 报告摘要(ReportSummary)投影:头部 + 聚合列(见 reportAggregateSQL)。
const reportSummaryColumnsSQL = `r.image_sha256, r.member_id, r.report_type,
	   COALESCE(to_char(r.report_date, 'YYYY-MM-DD'), ''), r.status, r.fail_reason,
	   agg.high, agg.low, agg.abnormal, agg.unknown, agg.not_tested, agg.unreviewed_low,
	   r.panels`

// reportAggregateSQL 单报告的 flag 聚合与未核对低置信计数(横向子查询,逐报告就地聚合)。
// unreviewed_low 统计所有报告的未核对低置信项(success 也可能有少量低置信项;failed 无项
// 自然为 0);not_tested 是信息计数,不计入 unknown 与 unreviewed_low。
const reportAggregateSQL = `SELECT
		   COUNT(*) FILTER (WHERE i.flag = 'high') AS high,
		   COUNT(*) FILTER (WHERE i.flag = 'low') AS low,
		   COUNT(*) FILTER (WHERE i.flag = 'abnormal') AS abnormal,
		   COUNT(*) FILTER (WHERE i.flag = 'unknown' AND (
		       i.flag_reason IS NULL OR i.flag_reason IN
		       ('ocr_miss', 'ocr_merge', 'extractor_pairing', 'unverified',
		        'indeterminate', 'scalar_ref', 'other')
		   )) AS unknown,
		   COUNT(*) FILTER (WHERE i.flag = 'not_tested') AS not_tested,
		   COUNT(*) FILTER (WHERE i.review_status = 'unreviewed'
		       AND i.low_confidence AND i.flag <> 'not_tested') AS unreviewed_low
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
		if err := rows.Scan(&r.Sha256, &r.OCRResultID, &r.ReportType, &date, &r.Status, &r.ItemsCount, &r.Panels); err != nil {
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
		if err := rows.Scan(&it.Name, &it.Panel, &it.Value, &it.Unit, &it.RefRange, &it.Method, &it.Flag, &it.FlagReason, &it.LowConfidence, &it.RawText); err != nil {
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

// TrendPoints 成员维度趋势(serve GET /members/{id}/indicators/{name}/trend;票 02):
// unit 空 = 全部单位;日期升序、dateless 沉底、同日期按报告 sha 与指标项 id 升序。
func (s *Store) TrendPoints(ctx context.Context, memberID int64, name, unit string) ([]pipeline.TrendPointRow, error) {
	rows, err := s.pool.Query(ctx, sqlTrendPointsByMemberName, memberID, name, unit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []pipeline.TrendPointRow
	for rows.Next() {
		var r pipeline.TrendPointRow
		var date string
		if err := rows.Scan(&r.ReportID, &date, &r.Value, &r.Unit, &r.RefRange, &r.Method, &r.Flag, &r.LowConfidence, &r.ReviewStatus); err != nil {
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
			&r.FlagCounts.High, &r.FlagCounts.Low, &r.FlagCounts.Abnormal, &r.FlagCounts.Unknown,
			&r.NotTestedCount, &r.UnreviewedLowConfidenceCount, &r.Panels); err != nil {
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
		if err := rows.Scan(&it.ItemID, &it.Name, &it.Panel, &it.Value, &it.Unit, &it.RefRange, &it.Method,
			&it.Flag, &it.FlagReason, &it.LowConfidence, &it.RawText, &it.ReviewStatus, &it.OCRLineNumbers); err != nil {
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

// SectionsByImage 报告级叙述段落(详情投影,0006/ADR-0008):非叙述型报告/存量 NULL
// 返回 nil,nil;报告缺失返回 nil,nil(与 SummaryByImage 同口径)。
func (s *Store) SectionsByImage(ctx context.Context, reportSha string) ([]contract.ReportSection, error) {
	var raw []byte
	err := s.pool.QueryRow(ctx, sqlReportSectionsByImage, reportSha).Scan(&raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if raw == nil {
		return nil, nil
	}
	var sections []contract.ReportSection
	if err := json.Unmarshal(raw, &sections); err != nil {
		return nil, fmt.Errorf("parse report sections: %w", err)
	}
	if len(sections) == 0 {
		return nil, nil
	}
	return sections, nil
}

// SetReportMember 只改报告归属(上传新建/无主归档时赋值,票 06;改归属端点同用)。
// 报告不存在返回 false, nil(404 口径;0 行更新即报告已不在,不静默虚报成功)。
func (s *Store) SetReportMember(ctx context.Context, reportSha string, memberID int64) (bool, error) {
	tag, err := s.pool.Exec(ctx, sqlReportSetMember, reportSha, memberID)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, nil
}

// DeleteReport 硬删报告行(票 10):级联 report_items → correction_pairs(FK ON DELETE
// CASCADE);images 与 ocr_results 不动(原图与识别历史留痕,append-only 文化不破)。
// 报告不存在返回 false, nil(404 口径,不虚报错误)。
func (s *Store) DeleteReport(ctx context.Context, reportSha string) (bool, error) {
	tag, err := s.pool.Exec(ctx, sqlReportDelete, reportSha)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, nil
}

// ReviewItem 核对落地(票 09;确认/修正,单事务):锁报告行(串行化同一报告的并发
// 核对,状态翻转不丢)→ 取指标项 → 应用动作(pipeline.ApplyReviewAction:生效值 +
// flag 重算 + 修正对)→ 报告状态单调派生(partial → success 当且仅当未核对低置信
// 计数归零)→ 读出动作后报告级状态与计数。报告或指标项缺失返回 nil, nil(404 口径)。
func (s *Store) ReviewItem(ctx context.Context, reportSha string, itemID int64, action pipeline.ReviewAction) (*pipeline.ReviewOutcome, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	var locked string
	if err := tx.QueryRow(ctx, sqlReportLock, reportSha).Scan(&locked); errors.Is(err, pgx.ErrNoRows) {
		return nil, nil // 报告不存在(与指标项缺失同口径:对客户端是同一件事)
	} else if err != nil {
		return nil, fmt.Errorf("lock report: %w", err)
	}

	var current pipeline.ReportItemRow
	err = tx.QueryRow(ctx, sqlItemForReview, reportSha, itemID).Scan(
		&current.ItemID, &current.Name, &current.Panel, &current.Value, &current.Unit, &current.RefRange, &current.Method,
		&current.Flag, &current.FlagReason, &current.LowConfidence, &current.RawText, &current.ReviewStatus, &current.OCRLineNumbers)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil // 指标项不存在或不属于该报告
	} else if err != nil {
		return nil, fmt.Errorf("read report item: %w", err)
	}

	next, pair := pipeline.ApplyReviewActionWithPolicy(current, action, s.flagPolicy)
	if action.NoReference {
		if _, err := tx.Exec(ctx, sqlItemReviewNoReference, current.ItemID); err != nil {
			return nil, fmt.Errorf("mark no reference: %w", err)
		}
	} else if pair == nil {
		if _, err := tx.Exec(ctx, sqlItemReviewConfirm, current.ItemID); err != nil {
			return nil, fmt.Errorf("confirm item: %w", err)
		}
	} else {
		if _, err := tx.Exec(ctx, sqlItemReviewCorrect, current.ItemID, next.Name, next.Value, next.Unit, next.RefRange, next.Flag, next.FlagReason); err != nil {
			return nil, fmt.Errorf("correct item: %w", err)
		}
		if _, err := tx.Exec(ctx, sqlCorrectionPairInsert, current.ItemID,
			pair.BeforeName, pair.BeforeValue, pair.BeforeUnit, pair.BeforeRefRange,
			pair.AfterName, pair.AfterValue, pair.AfterUnit, pair.AfterRefRange,
			pair.RawText, pair.OCRLineNumbers); err != nil {
			return nil, fmt.Errorf("append correction pair: %w", err)
		}
	}
	if _, err := tx.Exec(ctx, sqlReportDeriveSuccess, reportSha); err != nil {
		return nil, fmt.Errorf("derive report status: %w", err)
	}
	outcome := pipeline.ReviewOutcome{Item: next}
	if err := tx.QueryRow(ctx, sqlReportReviewState, reportSha).Scan(&outcome.Status, &outcome.UnreviewedLowConfidenceCount); err != nil {
		return nil, fmt.Errorf("read report review state: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return &outcome, nil
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
// 镜像校验(viz-and-client 票 03 双保险;票 04 叙事后开口):零 items + 空 sections
// 只允许 failed;零 items + 非空 sections(影像单等叙述型报告)合法。绕过识别端直灌的
// 违规形状(status=success/partial 且零 items 且无 sections)在此拒绝,不写库。
func (s *Store) WriteReport(ctx context.Context, reportSha string, ocrID int64, report contract.Report, failReason *string) error {
	if len(report.Items) == 0 && len(report.Sections) == 0 && report.Status != contract.StatusFailed {
		return fmt.Errorf("report invariant violated: %d items and %d sections with status %q — report with no items and no sections must be failed", len(report.Items), len(report.Sections), report.Status)
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	var sectionsArg any
	if len(report.Sections) > 0 {
		raw, err := json.Marshal(report.Sections)
		if err != nil {
			return fmt.Errorf("marshal report sections: %w", err)
		}
		sectionsArg = string(raw)
	}
	panels := report.Panels
	if panels == nil {
		panels = []string{} // text[] NOT NULL:未回填/失败报告的合法空数组
	}
	if _, err := tx.Exec(ctx, sqlReportUpsert,
		reportSha, ocrID, report.ReportType, report.ReportDate, report.Status, failReason, sectionsArg, panels); err != nil {
		return fmt.Errorf("upsert report: %w", err)
	}
	if _, err := tx.Exec(ctx, sqlItemsDeleteByReport, reportSha); err != nil {
		return err
	}
	for _, it := range report.Items {
		panel := it.Panel
		if panel == "" {
			panel = "other" // 与迁移默认一致:识别端缺失时不留空 code
		}
		if _, err := tx.Exec(ctx, sqlItemInsert,
			reportSha, it.Name, panel, it.Value, it.Unit, it.RefRange, it.Method, it.Flag, it.FlagReason,
			it.LowConfidence, it.RawText, it.OCRLineNumbers); err != nil {
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
