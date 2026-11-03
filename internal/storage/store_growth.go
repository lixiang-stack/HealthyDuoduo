// 生长记录持久化(llm-analysis 票 02):每次测量一行;BMI 不落列,
// 读取时用测量时点的身高体重实时计算。列表按测量日期降序、同日按 id 降序。
package storage

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"

	"healthyduoduo/internal/pipeline"
)

const growthColumnsSQL = `id, member_id, to_char(measured_on, 'YYYY-MM-DD'),
	   height_cm, weight_kg, source, created_at`

const (
	sqlGrowthInsert = `INSERT INTO growth_records (member_id, measured_on, height_cm, weight_kg, source)
		 VALUES ($1,$2::date,$3,$4,$5)
		 RETURNING ` + growthColumnsSQL

	sqlGrowthListByMember = `SELECT ` + growthColumnsSQL + `
		 FROM growth_records WHERE member_id = $1
		 ORDER BY measured_on DESC, id DESC`
)

// CreateGrowth 新增生长记录;资料过领域规则镜像(HTTP 边界已校验,直连存储路径兜底)。
func (s *Store) CreateGrowth(ctx context.Context, memberID int64, profile pipeline.GrowthProfile) (*pipeline.GrowthRecord, error) {
	if err := pipeline.ValidateGrowthProfile(profile, time.Now()); err != nil {
		return nil, err
	}
	var record pipeline.GrowthRecord
	if err := scanGrowth(s.pool.QueryRow(ctx, sqlGrowthInsert,
		memberID, profile.MeasuredOn, profile.HeightCm, profile.WeightKg, profile.Source,
	), &record); err != nil {
		return nil, err
	}
	return &record, nil
}

// ListGrowth 成员全部生长记录(测量日期降序、同日 id 降序)。
func (s *Store) ListGrowth(ctx context.Context, memberID int64) ([]pipeline.GrowthRecord, error) {
	rows, err := s.pool.Query(ctx, sqlGrowthListByMember, memberID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []pipeline.GrowthRecord
	for rows.Next() {
		var record pipeline.GrowthRecord
		if err := scanGrowth(rows, &record); err != nil {
			return nil, err
		}
		out = append(out, record)
	}
	return out, rows.Err()
}

// scanGrowth 统一生长记录行扫描列序(与 growthColumnsSQL 对齐);BMI 实时计算。
func scanGrowth(row pgx.Row, record *pipeline.GrowthRecord) error {
	if err := row.Scan(&record.GrowthID, &record.MemberID, &record.MeasuredOn,
		&record.HeightCm, &record.WeightKg, &record.Source, &record.CreatedAt); err != nil {
		return err
	}
	record.BMI = pipeline.BMI(record.HeightCm, record.WeightKg)
	// 超重 / 肥胖判定依赖年龄性别百分位(后续工单);手输来源永不输出判定。
	record.WeightStatus = nil
	return nil
}
