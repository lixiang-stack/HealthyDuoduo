// 护理事件持久化（care-events 票 01）：单表四类型，类型专属列显式落列；
// 列表按范围重叠返回，倒序 started_at DESC, id DESC；source 固定家长记录。
package storage

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"

	"healthyduoduo/internal/care"
)

// 方法名沿用 care.Store 的窄端口形状；*Store 同时被其他领域窄接口注入。
var _ care.Store = (*Store)(nil)

const careEventColumnsSQL = `id, member_id, type, source, started_at, ended_at, note,
	   method, amount_ml, duration_min, diaper_type, food, amount_g`

const (
	sqlCareEventInsert = `INSERT INTO care_events
		 (member_id, type, source, started_at, ended_at, note,
		  method, amount_ml, duration_min, diaper_type, food, amount_g)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)
		 RETURNING ` + careEventColumnsSQL

	// 完整替换可编辑内容；id / member_id / source 不在 SET 内。
	sqlCareEventUpdate = `UPDATE care_events
		 SET type = $2, started_at = $3, ended_at = $4, note = $5, method = $6,
		     amount_ml = $7, duration_min = $8, diaper_type = $9, food = $10, amount_g = $11
		 WHERE id = $1
		 RETURNING ` + careEventColumnsSQL

	// 范围重叠：点事件只看 started_at；睡眠用 ended_at 判断，进行中睡眠开放到未来。
	sqlCareEventList = `SELECT ` + careEventColumnsSQL + `
		 FROM care_events
		 WHERE member_id = $1
		   AND started_at < $3
		   AND (
		       (type = 'sleep' AND ended_at IS NULL)
		       OR (ended_at IS NOT NULL AND ended_at > $2)
		       OR (type <> 'sleep' AND ended_at IS NULL AND started_at >= $2)
		   )
		 ORDER BY started_at DESC, id DESC`

	sqlCareEventDelete = `DELETE FROM care_events WHERE id = $1`
)

// Create 新增护理事件；先做领域规范化与校验，DB CHECK 仍是最后防线。
func (s *Store) Create(ctx context.Context, memberID int64, input care.EventInput) (*care.CareEvent, error) {
	input = care.Normalize(input)
	if err := care.Validate(input); err != nil {
		return nil, err
	}
	var event care.CareEvent
	if err := scanCareEvent(s.pool.QueryRow(ctx, sqlCareEventInsert,
		memberID, input.Type, care.SourceParent, input.StartedAt, input.EndedAt, input.Note,
		input.Method, input.AmountML, input.DurationMin, input.DiaperType, input.Food, input.AmountG,
	), &event); err != nil {
		return nil, err
	}
	return &event, nil
}

// List 成员在 [from, to) 内有重叠的护理事件（倒序 started_at DESC, id DESC）。
func (s *Store) List(ctx context.Context, memberID int64, from, to time.Time) ([]care.CareEvent, error) {
	rows, err := s.pool.Query(ctx, sqlCareEventList, memberID, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []care.CareEvent
	for rows.Next() {
		var event care.CareEvent
		if err := scanCareEvent(rows, &event); err != nil {
			return nil, err
		}
		out = append(out, event)
	}
	return out, rows.Err()
}

// Update 完整替换可编辑内容；事件不存在返回 nil, nil。
func (s *Store) Update(ctx context.Context, eventID int64, input care.EventInput) (*care.CareEvent, error) {
	input = care.Normalize(input)
	if err := care.Validate(input); err != nil {
		return nil, err
	}
	var event care.CareEvent
	err := scanCareEvent(s.pool.QueryRow(ctx, sqlCareEventUpdate,
		eventID, input.Type, input.StartedAt, input.EndedAt, input.Note,
		input.Method, input.AmountML, input.DurationMin, input.DiaperType, input.Food, input.AmountG,
	), &event)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &event, nil
}

// Delete 硬删除；返回 false 表示事件不存在。
func (s *Store) Delete(ctx context.Context, eventID int64) (bool, error) {
	tag, err := s.pool.Exec(ctx, sqlCareEventDelete, eventID)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, nil
}

// scanCareEvent 统一护理事件行扫描列序（与 careEventColumnsSQL 对齐）。
func scanCareEvent(row pgx.Row, event *care.CareEvent) error {
	return row.Scan(&event.ID, &event.MemberID, &event.Type, &event.Source, &event.StartedAt,
		&event.EndedAt, &event.Note, &event.Method, &event.AmountML, &event.DurationMin,
		&event.DiaperType, &event.Food, &event.AmountG)
}
