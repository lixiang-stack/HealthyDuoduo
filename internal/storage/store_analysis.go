// 分析数据源与快照持久化(llm-analysis 票 04):指标点按成员 + 项目组读出(含 failed,
// 由领域层排除并给原因);病历读出全部段落;分析结果以 facts / snapshot JSONB 永久保留。
package storage

import (
	"context"
	"encoding/json"
	"fmt"

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
	   ), '')
	   )`

// storedLayers 持久化的 layers 形状;事实层当前是唯一一层,后续工单可追加。
type storedLayers struct {
	Facts analysis.Facts `json:"facts"`
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

// SaveAnalysis 保存事实层与快照,返回带 analysis_id / created_at 的记录。
func (s *Store) SaveAnalysis(ctx context.Context, record analysis.Record) (*analysis.Record, error) {
	layers, err := json.Marshal(storedLayers{Facts: record.Facts})
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

// DataVersion 成员报告 / 指标项 / 核对状态的当前指纹;与快照中的值不同即 stale。
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
	if err := json.Unmarshal(snapshotRaw, &record.Snapshot); err != nil {
		return nil, fmt.Errorf("parse analysis snapshot: %w", err)
	}
	if err := json.Unmarshal(sourcesRaw, &record.Sources); err != nil {
		return nil, fmt.Errorf("parse analysis sources: %w", err)
	}
	return &record, nil
}
