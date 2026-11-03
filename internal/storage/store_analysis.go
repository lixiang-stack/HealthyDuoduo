// 分析数据源与快照持久化(llm-analysis 票 04):指标点按成员 + 项目组读出(含 failed,
// 由领域层排除并给原因);病历读出全部段落;分析结果以 facts / snapshot JSONB 永久保留。
package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"healthyduoduo/internal/analysis"
	"healthyduoduo/internal/contract"
)

const sqlIndicatorPanel = `SELECT i.panel
	   FROM report_items i JOIN reports r ON r.image_sha256 = i.report_sha256
	   WHERE r.member_id = $1 AND r.status <> 'failed'
	     AND (i.name = $2 OR i.code = $2 OR i.loinc = $2)
	   ORDER BY r.report_date DESC NULLS LAST, r.image_sha256 DESC, i.id DESC
	   LIMIT 1`

const sqlAnalysisPoints = `SELECT r.member_id, r.image_sha256,
	   COALESCE(to_char(r.report_date, 'YYYY-MM-DD'), ''), r.report_type, r.status,
	   i.panel, i.id, i.name,
	   COALESCE(i.code, ''), COALESCE(i.loinc, ''),
	   COALESCE(i.unit, ''), COALESCE(i.unit_display, ''), COALESCE(i.unit_status, ''),
	   COALESCE(i.value, ''), COALESCE(i.ref_range, ''), i.flag, i.low_confidence, i.review_status
	   FROM report_items i JOIN reports r ON r.image_sha256 = i.report_sha256
	   WHERE r.member_id = $1 AND i.panel = $2
	   ORDER BY r.report_date ASC NULLS LAST, r.image_sha256 ASC, i.id ASC`

const sqlClinicalNotes = `SELECT r.member_id, r.image_sha256,
	   COALESCE(to_char(r.report_date, 'YYYY-MM-DD'), ''), r.report_type, r.sections
	   FROM reports r
	   WHERE r.member_id = $1 AND r.report_type = $2
	   ORDER BY r.report_date ASC NULLS LAST, r.image_sha256 ASC`

const sqlAnalysisInsert = `INSERT INTO analyses
	   (member_id, panel, entry, indicator_name, window_months, status, layers, snapshot, sources)
	   VALUES ($1,$2,$3,$4,$5,$6,$7::jsonb,$8::jsonb,$9::jsonb)
	   RETURNING id, created_at`

const sqlAnalysisByID = `SELECT id, member_id, panel, entry, indicator_name, window_months,
	   status, layers, snapshot, sources, created_at
	   FROM analyses WHERE id = $1`

const sqlLatestGrowth = `SELECT COALESCE(to_char(measured_on, 'YYYY-MM-DD'), ''),
	   height_cm, weight_kg, source
	   FROM growth_records
	   WHERE member_id = $1
	   ORDER BY measured_on DESC, id DESC
	   LIMIT 1`

const sqlAnalysisUpdateExplanation = `UPDATE analyses
	   SET layers = jsonb_set(
	           jsonb_set(layers, '{explanation}', $2::jsonb),
	           '{explanation_reason}', to_jsonb($3::text)
	       ),
	       sources = $4::jsonb
	   WHERE id = $1`

const sqlInsertLLMCall = `INSERT INTO analysis_llm_calls (analysis_id) VALUES ($1)`

const sqlLLMUsedToday = `SELECT count(*)::int
	   FROM analysis_llm_calls
	   WHERE created_at >= date_trunc('day', now())`

const sqlInsertAnalysisEvent = `INSERT INTO analysis_events (analysis_id, kind, detail)
	   VALUES ($1, $2, $3)`

const sqlInsertAnalysisFeedback = `INSERT INTO analysis_feedback (analysis_id, entry_id, reason)
	   VALUES ($1, $2, $3)
	   RETURNING id, created_at`

const sqlAnalysisMetrics = `SELECT
	       (SELECT count(*)::int FROM analyses
	        WHERE created_at >= $1 AND created_at < $2),
	       (SELECT count(*)::int FROM analysis_feedback
	        WHERE created_at >= $1 AND created_at < $2),
	       (SELECT count(*)::int FROM analysis_events
	        WHERE created_at >= $1 AND created_at < $2)`

const sqlAnalysisDegradationKinds = `SELECT kind, count(*)::int
	   FROM analysis_events
	   WHERE created_at >= $1 AND created_at < $2
	   GROUP BY kind
	   ORDER BY kind`

// sqlAnalysisDataVersion 成员数据指纹:报告新增 / 修改 / 重识别、指标项核对状态与
// 生效值变化都会改变结果;用于旧分析 stale 判定。病历段落写入同样刷新 reports.updated_at。
const sqlAnalysisDataVersion = `SELECT md5(
	   COALESCE((
	       SELECT string_agg(r.image_sha256 || ':' || COALESCE(to_char(r.report_date, 'YYYY-MM-DD'), '')
	              || ':' || r.report_type || ':' || r.status || ':' || r.updated_at::text,
	              ',' ORDER BY r.image_sha256)
	       FROM reports r WHERE r.member_id = $1
	   ), '') || '|' || COALESCE((
	       SELECT string_agg(i.id::text || ':' || i.review_status || ':' || i.flag || ':'
	              || COALESCE(i.value, '') || ':' || COALESCE(i.unit, '') || ':'
	              || COALESCE(i.ref_range, '') || ':' || COALESCE(i.unit_status, ''),
	              ',' ORDER BY i.id)
	       FROM report_items i JOIN reports r ON r.image_sha256 = i.report_sha256
	       WHERE r.member_id = $1
	   ), '') || '|' || COALESCE((
	       SELECT m.sex || ':' || to_char(m.birth_date, 'YYYY-MM-DD')
	       FROM members m WHERE m.id = $1
	   ), '') || '|' || COALESCE((
	       SELECT string_agg(g.id::text || ':' || to_char(g.measured_on, 'YYYY-MM-DD') || ':'
	              || COALESCE(g.height_cm::text, '') || ':' || COALESCE(g.weight_kg::text, '') || ':'
	              || g.source, ',' ORDER BY g.id)
	       FROM growth_records g WHERE g.member_id = $1
	   ), '')
	   )`

// storedLayers 持久化的 layers 形状;旧记录缺 framework / explanation 时按零值读出。
type storedLayers struct {
	Facts             analysis.Facts        `json:"facts"`
	Framework         analysis.Framework    `json:"framework"`
	Explanation       *analysis.Explanation `json:"explanation,omitempty"`
	ExplanationReason string                `json:"explanation_reason,omitempty"`
}

// IndicatorPanel 按指标名 / 标准码 / LOINC 找所属项目组;找不到返回空串。
func (s *Store) IndicatorPanel(ctx context.Context, memberID int64, name string) (string, error) {
	var panel string
	err := s.pool.QueryRow(ctx, sqlIndicatorPanel, memberID, name).Scan(&panel)
	if err == pgx.ErrNoRows {
		return "", nil
	}
	return panel, err
}

// AnalysisPoints 成员某项目组的全部指标点(含 failed 报告,由领域层排除并给原因)。
func (s *Store) AnalysisPoints(ctx context.Context, memberID int64, panel string) ([]analysis.InputPoint, error) {
	rows, err := s.pool.Query(ctx, sqlAnalysisPoints, memberID, panel)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []analysis.InputPoint
	for rows.Next() {
		var point analysis.InputPoint
		if err := rows.Scan(&point.MemberID, &point.ReportID, &point.ReportDate,
			&point.ReportType, &point.ReportStatus, &point.Panel, &point.ItemID, &point.Name,
			&point.Code, &point.Loinc, &point.Unit, &point.UnitDisplay, &point.UnitStatus,
			&point.Value, &point.RefRange, &point.Flag, &point.LowConfidence,
			&point.ReviewStatus); err != nil {
			return nil, err
		}
		out = append(out, point)
	}
	return out, rows.Err()
}

// ClinicalNotes 成员全部病历报告与段落;非病历 / 无段落 / 失败报告自然不返回。
func (s *Store) ClinicalNotes(ctx context.Context, memberID int64) ([]analysis.InputNote, error) {
	rows, err := s.pool.Query(ctx, sqlClinicalNotes, memberID, contract.ReportTypeClinicalNote)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []analysis.InputNote
	for rows.Next() {
		var note analysis.InputNote
		var raw []byte
		if err := rows.Scan(&note.MemberID, &note.ReportID, &note.ReportDate,
			&note.ReportType, &raw); err != nil {
			return nil, err
		}
		if raw == nil {
			continue
		}
		var sections []contract.ReportSection
		if err := json.Unmarshal(raw, &sections); err != nil {
			return nil, fmt.Errorf("parse clinical note sections: %w", err)
		}
		for _, section := range sections {
			note.Sections = append(note.Sections, analysis.InputSection{
				Kind: section.Kind,
				Text: section.Text,
			})
		}
		out = append(out, note)
	}
	return out, rows.Err()
}

// LatestGrowth 最近一次生长记录;没有记录返回 nil, nil。
func (s *Store) LatestGrowth(ctx context.Context, memberID int64) (*analysis.InputGrowth, error) {
	var growth analysis.InputGrowth
	err := s.pool.QueryRow(ctx, sqlLatestGrowth, memberID).Scan(
		&growth.MeasuredOn, &growth.HeightCm, &growth.WeightKg, &growth.Source,
	)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &growth, nil
}

// SaveAnalysis 保存事实层与快照,返回带 analysis_id / created_at 的记录。
func (s *Store) SaveAnalysis(ctx context.Context, record analysis.Record) (*analysis.Record, error) {
	layers, err := json.Marshal(storedLayers{
		Facts:             record.Facts,
		Framework:         record.Framework,
		Explanation:       record.Explanation,
		ExplanationReason: record.ExplanationReason,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal analysis layers: %w", err)
	}
	snapshot, err := json.Marshal(record.Snapshot)
	if err != nil {
		return nil, fmt.Errorf("marshal analysis snapshot: %w", err)
	}
	sources := record.Sources
	if sources == nil {
		sources = []analysis.Source{}
	}
	sourcesJSON, err := json.Marshal(sources)
	if err != nil {
		return nil, fmt.Errorf("marshal analysis sources: %w", err)
	}
	saved := record
	if err := s.pool.QueryRow(ctx, sqlAnalysisInsert,
		record.MemberID, record.Panel, record.Entry, record.IndicatorName,
		record.WindowMonths, record.Status, string(layers), string(snapshot), string(sourcesJSON),
	).Scan(&saved.AnalysisID, &saved.CreatedAt); err != nil {
		return nil, err
	}
	return &saved, nil
}

// UpdateAnalysisExplanation 回写解释层或不可用原因;事实层与框架层保持不变。
func (s *Store) UpdateAnalysisExplanation(ctx context.Context, id int64, explanation *analysis.Explanation, reason string, sources []analysis.Source) error {
	var explanationJSON []byte
	if explanation != nil {
		raw, err := json.Marshal(explanation)
		if err != nil {
			return fmt.Errorf("marshal analysis explanation: %w", err)
		}
		explanationJSON = raw
	} else {
		explanationJSON = []byte("null")
	}
	if sources == nil {
		sources = []analysis.Source{}
	}
	sourcesJSON, err := json.Marshal(sources)
	if err != nil {
		return fmt.Errorf("marshal analysis sources: %w", err)
	}
	_, err = s.pool.Exec(ctx, sqlAnalysisUpdateExplanation, id, string(explanationJSON), reason, string(sourcesJSON))
	return err
}

// ConsumeLLMQuota 原子占用一次解释层调用;每天每家庭(单家庭部署 = 全局)上限。
// 用事务级 advisory lock 串行化同一天的 count + insert,避免并发请求同时越过上限。
func (s *Store) ConsumeLLMQuota(ctx context.Context, analysisID int64, limit int) (int, bool, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return 0, false, err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx,
		`SELECT pg_advisory_xact_lock(hashtext('analysis_llm_quota:' || current_date::text)::bigint)`,
	); err != nil {
		return 0, false, err
	}
	var used int
	if err := tx.QueryRow(ctx, sqlLLMUsedToday).Scan(&used); err != nil {
		return 0, false, err
	}
	if used >= limit {
		return used, false, nil
	}
	if _, err := tx.Exec(ctx, sqlInsertLLMCall, analysisID); err != nil {
		return 0, false, err
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, false, err
	}
	return used + 1, true, nil
}

// LLMUsedToday 当前已用解释层调用次数。
func (s *Store) LLMUsedToday(ctx context.Context) (int, error) {
	var used int
	if err := s.pool.QueryRow(ctx, sqlLLMUsedToday).Scan(&used); err != nil {
		return 0, err
	}
	return used, nil
}

// RecordAnalysisEvent 记录降级事件;事件写入失败不影响分析结果。
func (s *Store) RecordAnalysisEvent(ctx context.Context, analysisID int64, kind, detail string) error {
	_, err := s.pool.Exec(ctx, sqlInsertAnalysisEvent, analysisID, kind, detail)
	return err
}

// RecordFeedback 记录「这不对」反馈;只写入,不修改条目或分析内容。
func (s *Store) RecordFeedback(ctx context.Context, feedback analysis.Feedback) (*analysis.Feedback, error) {
	saved := feedback
	if err := s.pool.QueryRow(ctx, sqlInsertAnalysisFeedback,
		feedback.AnalysisID, feedback.EntryID, feedback.Reason,
	).Scan(&saved.FeedbackID, &saved.CreatedAt); err != nil {
		return nil, err
	}
	return &saved, nil
}

// AnalysisMetrics 周检口径:分析数、反馈数、降级事件数与按 kind 的分布。
// from / to 为包含端点的日期;SQL 用 [from, to+1day) 避免漏掉当天。
func (s *Store) AnalysisMetrics(ctx context.Context, from, to time.Time) (analysis.Metrics, error) {
	metrics := analysis.Metrics{
		From:             from.Format("2006-01-02"),
		To:               to.Format("2006-01-02"),
		DegradationKinds: map[string]int{},
	}
	exclusiveTo := to.AddDate(0, 0, 1)
	if err := s.pool.QueryRow(ctx, sqlAnalysisMetrics, from, exclusiveTo).Scan(
		&metrics.Analyses, &metrics.Feedback, &metrics.Degradations,
	); err != nil {
		return analysis.Metrics{}, err
	}
	rows, err := s.pool.Query(ctx, sqlAnalysisDegradationKinds, from, exclusiveTo)
	if err != nil {
		return analysis.Metrics{}, err
	}
	defer rows.Close()
	for rows.Next() {
		var kind string
		var count int
		if err := rows.Scan(&kind, &count); err != nil {
			return analysis.Metrics{}, err
		}
		metrics.DegradationKinds[kind] = count
	}
	if err := rows.Err(); err != nil {
		return analysis.Metrics{}, err
	}
	if metrics.Analyses > 0 {
		metrics.FeedbackRate = float64(metrics.Feedback) / float64(metrics.Analyses)
		metrics.DegradationRate = float64(metrics.Degradations) / float64(metrics.Analyses)
	}
	return metrics, nil
}

// DataVersion 成员报告 / 指标项 / 核对状态 / 生长记录的当前指纹;与快照中的值不同即 stale。
func (s *Store) DataVersion(ctx context.Context, memberID int64) (string, error) {
	var version string
	if err := s.pool.QueryRow(ctx, sqlAnalysisDataVersion, memberID).Scan(&version); err != nil {
		return "", err
	}
	return version, nil
}

// AnalysisByID 读取已保存分析;不存在返回 nil, nil。
func (s *Store) AnalysisByID(ctx context.Context, id int64) (*analysis.Record, error) {
	var record analysis.Record
	var layersRaw, snapshotRaw, sourcesRaw []byte
	err := s.pool.QueryRow(ctx, sqlAnalysisByID, id).Scan(
		&record.AnalysisID, &record.MemberID, &record.Panel, &record.Entry,
		&record.IndicatorName, &record.WindowMonths, &record.Status,
		&layersRaw, &snapshotRaw, &sourcesRaw, &record.CreatedAt,
	)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var layers storedLayers
	if err := json.Unmarshal(layersRaw, &layers); err != nil {
		return nil, fmt.Errorf("parse analysis layers: %w", err)
	}
	record.Facts = layers.Facts
	record.Framework = layers.Framework
	record.Explanation = layers.Explanation
	record.ExplanationReason = layers.ExplanationReason
	if err := json.Unmarshal(snapshotRaw, &record.Snapshot); err != nil {
		return nil, fmt.Errorf("parse analysis snapshot: %w", err)
	}
	if err := json.Unmarshal(sourcesRaw, &record.Sources); err != nil {
		return nil, fmt.Errorf("parse analysis sources: %w", err)
	}
	return &record, nil
}
