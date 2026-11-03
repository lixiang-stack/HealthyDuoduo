// 生长记录存储冒烟(llm-analysis 票 02):真库上的新增 / 列表排序 / BMI 实时计算 /
// 成员列表最近一次记录 / 成员删除级联。需要 HDD_TEST_DSN,未设整体 Skip。
package storage

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"healthyduoduo/internal/pipeline"
)

func TestGrowthStoreSmoke(t *testing.T) {
	dsn := os.Getenv("HDD_TEST_DSN")
	if dsn == "" {
		t.Skip("HDD_TEST_DSN not set: growth store smoke runs against a disposable PG container")
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
	st := NewStore(pool)
	defer st.Close()

	member, err := st.CreateMember(ctx, pipeline.MemberProfile{
		Name: "小虎", Sex: pipeline.SexMale, BirthDate: "2020-05-20",
	})
	if err != nil {
		t.Fatalf("create member: %v", err)
	}
	if member.LatestGrowth != nil {
		t.Fatalf("new member latest growth = %+v, want nil", member.LatestGrowth)
	}

	height, weight := 115.0, 21.0
	older, err := st.CreateGrowth(ctx, member.MemberID, pipeline.GrowthProfile{
		MeasuredOn: "2026-06-01", HeightCm: &height, WeightKg: &weight, Source: pipeline.GrowthSourceReport,
	})
	if err != nil {
		t.Fatalf("create older: %v", err)
	}
	height, weight = 118.5, 22.4
	newer, err := st.CreateGrowth(ctx, member.MemberID, pipeline.GrowthProfile{
		MeasuredOn: "2026-10-01", HeightCm: &height, WeightKg: &weight, Source: pipeline.GrowthSourceMeasured,
	})
	if err != nil {
		t.Fatalf("create newer: %v", err)
	}
	height = 119
	manual, err := st.CreateGrowth(ctx, member.MemberID, pipeline.GrowthProfile{
		MeasuredOn: "2026-10-01", HeightCm: &height, Source: pipeline.GrowthSourceManual,
	})
	if err != nil {
		t.Fatalf("create manual: %v", err)
	}

	if older.BMI == nil || *older.BMI != 15.9 {
		t.Errorf("older BMI = %v, want 15.9", older.BMI)
	}
	if newer.BMI == nil || *newer.BMI != 16 {
		t.Errorf("newer BMI = %v, want 16", newer.BMI)
	}
	if manual.BMI != nil {
		t.Errorf("height-only BMI = %v, want nil", *manual.BMI)
	}
	if manual.WeightStatus != nil {
		t.Errorf("manual weight_status = %v, want nil", *manual.WeightStatus)
	}

	records, err := st.ListGrowth(ctx, member.MemberID)
	if err != nil {
		t.Fatalf("list growth: %v", err)
	}
	if len(records) != 3 {
		t.Fatalf("records = %d, want 3", len(records))
	}
	// 同日按 id 降序:manual 后写入,排在同日 newer 之前。
	if records[0].GrowthID != manual.GrowthID || records[1].GrowthID != newer.GrowthID ||
		records[2].GrowthID != older.GrowthID {
		t.Fatalf("list order = %v/%v/%v, want manual/newer/older",
			records[0].GrowthID, records[1].GrowthID, records[2].GrowthID)
	}

	// 成员列表 / 单成员视图带最近一次记录(同测量日期取 id 最大的 manual)。
	members, err := st.ListMembers(ctx)
	if err != nil {
		t.Fatalf("list members: %v", err)
	}
	if len(members) != 1 || members[0].LatestGrowth == nil ||
		members[0].LatestGrowth.GrowthID != manual.GrowthID {
		t.Fatalf("member latest growth = %+v, want growth %d", members[0].LatestGrowth, manual.GrowthID)
	}
	got, err := st.MemberByID(ctx, member.MemberID)
	if err != nil || got == nil || got.LatestGrowth == nil || got.LatestGrowth.GrowthID != manual.GrowthID {
		t.Fatalf("MemberByID latest growth = %+v, err = %v", got, err)
	}

	// 非法资料在存储边界被拒,不落库。
	if _, err := st.CreateGrowth(ctx, member.MemberID, pipeline.GrowthProfile{
		MeasuredOn: "2026-10-01", Source: "scale",
	}); err != pipeline.ErrGrowthSourceInvalid {
		t.Errorf("invalid source err = %v, want ErrGrowthSourceInvalid", err)
	}

	// 删除成员级联删除生长记录。
	if deleted, err := st.DeleteMember(ctx, member.MemberID); err != nil || !deleted {
		t.Fatalf("delete member = (%v, %v)", deleted, err)
	}
	var remaining int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM growth_records`).Scan(&remaining); err != nil {
		t.Fatal(err)
	}
	if remaining != 0 {
		t.Errorf("growth rows after member delete = %d, want 0", remaining)
	}
}
