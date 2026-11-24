// 护理事件存储冒烟（care-events 票 01）：四类型写入、领域校验拒绝、范围重叠
// （含跨午夜与进行中睡眠）、倒序、修改保留身份、删除语义与成员级联。
// 需要 HDD_TEST_DSN；未设整体 Skip（与 migrate_test 同约定）。
package storage

import (
	"context"
	"errors"
	"os"
	"slices"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"healthyduoduo/internal/care"
	"healthyduoduo/internal/pipeline"
)

func carePtr[T any](value T) *T { return &value }

func TestCareEventStoreSmoke(t *testing.T) {
	dsn := os.Getenv("HDD_TEST_DSN")
	if dsn == "" {
		t.Skip("HDD_TEST_DSN not set: care event store smoke runs against a disposable PG container")
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
		Name: "小虎", Sex: pipeline.SexMale, BirthDate: "2024-05-20",
	})
	if err != nil {
		t.Fatalf("create member: %v", err)
	}

	loc := time.FixedZone("CST", 8*3600)
	at := func(day, hour, minute int) time.Time {
		return time.Date(2026, 11, day, hour, minute, 0, 0, loc)
	}
	listIDs := func(memberID int64, from, to time.Time) []int64 {
		events, err := st.List(ctx, memberID, from, to)
		if err != nil {
			t.Fatalf("list care events: %v", err)
		}
		ids := make([]int64, len(events))
		for i, event := range events {
			ids[i] = event.ID
		}
		return ids
	}

	// 四类型 + 跨午夜睡眠 + 同刻排序用例。
	crossStart, crossEnd := at(22, 23, 50), at(23, 6, 20)
	crossMidnight, err := st.Create(ctx, member.MemberID, care.EventInput{
		Type: care.TypeSleep, StartedAt: crossStart, EndedAt: &crossEnd, DurationMin: carePtr(390),
	})
	if err != nil {
		t.Fatalf("create cross-midnight sleep: %v", err)
	}
	if crossMidnight.ID == 0 || crossMidnight.MemberID != member.MemberID ||
		crossMidnight.Type != care.TypeSleep || crossMidnight.Source != care.SourceParent ||
		!crossMidnight.StartedAt.Equal(crossStart) || crossMidnight.EndedAt == nil ||
		!crossMidnight.EndedAt.Equal(crossEnd) || crossMidnight.DurationMin == nil ||
		*crossMidnight.DurationMin != 390 {
		t.Fatalf("cross-midnight sleep = %+v, want stored sleep with source parent", crossMidnight)
	}

	feed, err := st.Create(ctx, member.MemberID, care.EventInput{
		Type: care.TypeFeed, StartedAt: at(23, 7, 30),
		Method: carePtr(care.FeedMethodFormula), AmountML: carePtr(120.0),
		Note: carePtr("  早晨配方奶  "),
	})
	if err != nil {
		t.Fatalf("create feed: %v", err)
	}
	if feed.Type != care.TypeFeed || feed.Source != care.SourceParent || feed.Method == nil ||
		*feed.Method != care.FeedMethodFormula || feed.AmountML == nil || *feed.AmountML != 120 ||
		feed.Note == nil || *feed.Note != "早晨配方奶" || feed.EndedAt != nil ||
		feed.DurationMin != nil || feed.DiaperType != nil || feed.Food != nil || feed.AmountG != nil {
		t.Fatalf("feed = %+v, want normalized formula feed", feed)
	}

	diaper, err := st.Create(ctx, member.MemberID, care.EventInput{
		Type: care.TypeDiaper, StartedAt: at(23, 8, 10),
		DiaperType: carePtr(care.DiaperTypeWet), Note: carePtr("   "),
	})
	if err != nil {
		t.Fatalf("create diaper: %v", err)
	}
	if diaper.DiaperType == nil || *diaper.DiaperType != care.DiaperTypeWet || diaper.Note != nil {
		t.Fatalf("diaper = %+v, want wet with blank note normalized to nil", diaper)
	}

	ongoingSleep, err := st.Create(ctx, member.MemberID, care.EventInput{
		Type: care.TypeSleep, StartedAt: at(23, 22, 0),
	})
	if err != nil {
		t.Fatalf("create ongoing sleep: %v", err)
	}
	if ongoingSleep.EndedAt != nil || ongoingSleep.DurationMin != nil {
		t.Fatalf("ongoing sleep = %+v, want open ended", ongoingSleep)
	}

	breastFeed, err := st.Create(ctx, member.MemberID, care.EventInput{
		Type: care.TypeFeed, StartedAt: at(24, 8, 5),
		Method: carePtr(care.FeedMethodBreast), DurationMin: carePtr(15),
	})
	if err != nil {
		t.Fatalf("create breast feed: %v", err)
	}

	solid, err := st.Create(ctx, member.MemberID, care.EventInput{
		Type: care.TypeSolid, StartedAt: at(23, 12, 30),
		Food: carePtr("  南瓜泥  "), AmountG: carePtr(30.0),
	})
	if err != nil {
		t.Fatalf("create solid: %v", err)
	}
	if solid.Food == nil || *solid.Food != "南瓜泥" || solid.AmountG == nil || *solid.AmountG != 30 {
		t.Fatalf("solid = %+v, want trimmed food with amount", solid)
	}

	tieEarlier, err := st.Create(ctx, member.MemberID, care.EventInput{
		Type: care.TypeFeed, StartedAt: at(24, 9, 0), Method: carePtr(care.FeedMethodBreast),
	})
	if err != nil {
		t.Fatalf("create first same-time feed: %v", err)
	}
	tieLater, err := st.Create(ctx, member.MemberID, care.EventInput{
		Type: care.TypeFeed, StartedAt: at(24, 9, 0),
		Method: carePtr(care.FeedMethodFormula), AmountML: carePtr(90.0),
	})
	if err != nil {
		t.Fatalf("create second same-time feed: %v", err)
	}

	otherMember, err := st.CreateMember(ctx, pipeline.MemberProfile{
		Name: "小满", Sex: pipeline.SexFemale, BirthDate: "2023-02-10",
	})
	if err != nil {
		t.Fatalf("create other member: %v", err)
	}
	if _, err := st.Create(ctx, otherMember.MemberID, care.EventInput{
		Type: care.TypeFeed, StartedAt: at(23, 7, 35), Method: carePtr(care.FeedMethodBreast),
	}); err != nil {
		t.Fatalf("create other member feed: %v", err)
	}

	// 领域校验先于 SQL：非法输入不落库。
	var before, after int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM care_events`).Scan(&before); err != nil {
		t.Fatal(err)
	}
	invalid := []struct {
		name  string
		input care.EventInput
		want  error
	}{
		{"unknown type", care.EventInput{Type: "pump", StartedAt: at(23, 10, 0)}, care.ErrTypeInvalid},
		{"feed missing method", care.EventInput{Type: care.TypeFeed, StartedAt: at(23, 10, 0)}, care.ErrFieldsInvalid},
		{"diaper with amount", care.EventInput{
			Type: care.TypeDiaper, StartedAt: at(23, 10, 0),
			DiaperType: carePtr(care.DiaperTypeMixed), AmountML: carePtr(100.0),
		}, care.ErrFieldsInvalid},
		{"sleep pair mismatch", care.EventInput{
			Type: care.TypeSleep, StartedAt: at(23, 10, 0), DurationMin: carePtr(30),
		}, care.ErrFieldsInvalid},
		{"solid blank food", care.EventInput{
			Type: care.TypeSolid, StartedAt: at(23, 10, 0), Food: carePtr("  "),
		}, care.ErrFieldsInvalid},
		{"zero amount", care.EventInput{
			Type: care.TypeFeed, StartedAt: at(23, 10, 0),
			Method: carePtr(care.FeedMethodFormula), AmountML: carePtr(0.0),
		}, care.ErrValueInvalid},
		{"negative duration", care.EventInput{
			Type: care.TypeFeed, StartedAt: at(23, 10, 0),
			Method: carePtr(care.FeedMethodBreast), DurationMin: carePtr(-5),
		}, care.ErrValueInvalid},
		{"ended before started", care.EventInput{
			Type: care.TypeSleep, StartedAt: at(23, 10, 0),
			EndedAt: carePtr(at(23, 9, 0)), DurationMin: carePtr(60),
		}, care.ErrEndedAtBeforeStarted},
	}
	for _, c := range invalid {
		t.Run("invalid "+c.name, func(t *testing.T) {
			if _, err := st.Create(ctx, member.MemberID, c.input); !errors.Is(err, c.want) {
				t.Fatalf("create error = %v, want %v", err, c.want)
			}
		})
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM care_events`).Scan(&after); err != nil {
		t.Fatal(err)
	}
	if after != before {
		t.Fatalf("care_events count after invalid inputs = %d, want %d", after, before)
	}

	// 范围重叠：from 含、to 不含；跨午夜睡眠同时出现在两天；进行中睡眠开放到未来。
	if got, want := listIDs(member.MemberID, at(22, 0, 0), at(23, 0, 0)), []int64{crossMidnight.ID}; !slices.Equal(got, want) {
		t.Errorf("day 22 ids = %v, want %v", got, want)
	}
	if got, want := listIDs(member.MemberID, at(23, 0, 0), at(24, 0, 0)),
		[]int64{ongoingSleep.ID, solid.ID, diaper.ID, feed.ID, crossMidnight.ID}; !slices.Equal(got, want) {
		t.Errorf("day 23 ids = %v, want %v", got, want)
	}
	if got, want := listIDs(member.MemberID, at(24, 0, 0), at(25, 0, 0)),
		[]int64{tieLater.ID, tieEarlier.ID, breastFeed.ID, ongoingSleep.ID}; !slices.Equal(got, want) {
		t.Errorf("day 24 ids = %v, want %v (same start: id DESC)", got, want)
	}
	if got, want := listIDs(member.MemberID, at(25, 0, 0), at(26, 0, 0)), []int64{ongoingSleep.ID}; !slices.Equal(got, want) {
		t.Errorf("day 25 ids = %v, want ongoing sleep %v", got, want)
	}
	if got, want := listIDs(member.MemberID, at(23, 7, 30), at(23, 8, 0)), []int64{feed.ID}; !slices.Equal(got, want) {
		t.Errorf("from-inclusive ids = %v, want %v", got, want)
	}
	if got := listIDs(member.MemberID, at(23, 7, 0), at(23, 7, 30)); len(got) != 0 {
		t.Errorf("to-exclusive ids = %v, want empty", got)
	}
	if got := listIDs(member.MemberID, at(23, 6, 20), at(23, 7, 0)); len(got) != 0 {
		t.Errorf("sleep ending at from ids = %v, want empty", got)
	}

	// 修改完整替换内容，保留 id / member_id / source，并清空旧类型专属列。
	updated, err := st.Update(ctx, feed.ID, care.EventInput{
		Type: care.TypeDiaper, StartedAt: at(23, 7, 45),
		DiaperType: carePtr(care.DiaperTypeDirty), Note: carePtr("  换尿布  "),
	})
	if err != nil {
		t.Fatalf("update feed to diaper: %v", err)
	}
	if updated.ID != feed.ID || updated.MemberID != member.MemberID ||
		updated.Source != care.SourceParent || updated.Type != care.TypeDiaper ||
		!updated.StartedAt.Equal(at(23, 7, 45)) || updated.DiaperType == nil ||
		*updated.DiaperType != care.DiaperTypeDirty || updated.Note == nil ||
		*updated.Note != "换尿布" || updated.Method != nil || updated.AmountML != nil ||
		updated.DurationMin != nil || updated.EndedAt != nil || updated.Food != nil || updated.AmountG != nil {
		t.Fatalf("updated event = %+v, want preserved identity and cleared feed fields", updated)
	}

	updatedSleep, err := st.Update(ctx, crossMidnight.ID, care.EventInput{
		Type: care.TypeSolid, StartedAt: at(25, 18, 0), Food: carePtr("米糊"),
	})
	if err != nil {
		t.Fatalf("update sleep to solid: %v", err)
	}
	if updatedSleep.EndedAt != nil || updatedSleep.DurationMin != nil ||
		updatedSleep.Food == nil || *updatedSleep.Food != "米糊" || updatedSleep.Type != care.TypeSolid {
		t.Fatalf("updated sleep = %+v, want solid with sleep columns cleared", updatedSleep)
	}

	// 非法修改被拒且原行不变。
	if _, err := st.Update(ctx, diaper.ID, care.EventInput{
		Type: care.TypeSleep, StartedAt: at(23, 8, 10), DurationMin: carePtr(5),
	}); !errors.Is(err, care.ErrFieldsInvalid) {
		t.Fatalf("invalid update error = %v, want ErrFieldsInvalid", err)
	}
	diaperRows, err := st.List(ctx, member.MemberID, at(23, 8, 0), at(23, 8, 30))
	if err != nil {
		t.Fatalf("list after invalid update: %v", err)
	}
	if len(diaperRows) != 1 || diaperRows[0].ID != diaper.ID || diaperRows[0].Type != care.TypeDiaper ||
		diaperRows[0].DiaperType == nil || *diaperRows[0].DiaperType != care.DiaperTypeWet {
		t.Fatalf("diaper after invalid update = %+v, want unchanged row", diaperRows)
	}

	if missing, err := st.Update(ctx, int64(1)<<40, care.EventInput{
		Type: care.TypeFeed, StartedAt: at(23, 10, 0), Method: carePtr(care.FeedMethodBreast),
	}); err != nil || missing != nil {
		t.Fatalf("update unknown id = (%+v, %v), want (nil, nil)", missing, err)
	}

	// 删除：首次 true，重复删除与未知 id 返回 false。
	deleted, err := st.Delete(ctx, solid.ID)
	if err != nil || !deleted {
		t.Fatalf("delete solid = (%v, %v), want (true, nil)", deleted, err)
	}
	if deleted, err := st.Delete(ctx, solid.ID); err != nil || deleted {
		t.Fatalf("delete solid again = (%v, %v), want (false, nil)", deleted, err)
	}
	if deleted, err := st.Delete(ctx, int64(1)<<40); err != nil || deleted {
		t.Fatalf("delete unknown id = (%v, %v), want (false, nil)", deleted, err)
	}
	if missing, err := st.Update(ctx, solid.ID, care.EventInput{
		Type: care.TypeSolid, StartedAt: at(23, 12, 30), Food: carePtr("南瓜泥"),
	}); err != nil || missing != nil {
		t.Fatalf("update deleted id = (%+v, %v), want (nil, nil)", missing, err)
	}

	// 成员删除级联清理该成员全部护理事件，不影响其他成员。
	if deleted, err := st.DeleteMember(ctx, member.MemberID); err != nil || !deleted {
		t.Fatalf("delete member = (%v, %v)", deleted, err)
	}
	var memberRows, otherRows int
	if err := pool.QueryRow(ctx, `SELECT
		(SELECT count(*) FROM care_events WHERE member_id = $1),
		(SELECT count(*) FROM care_events WHERE member_id = $2)`,
		member.MemberID, otherMember.MemberID).Scan(&memberRows, &otherRows); err != nil {
		t.Fatal(err)
	}
	if memberRows != 0 || otherRows != 1 {
		t.Fatalf("care rows after member delete = %d/%d, want 0/1", memberRows, otherRows)
	}
	if deleted, err := st.DeleteMember(ctx, otherMember.MemberID); err != nil || !deleted {
		t.Fatalf("delete other member = (%v, %v)", deleted, err)
	}
	var remaining int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM care_events`).Scan(&remaining); err != nil {
		t.Fatal(err)
	}
	if remaining != 0 {
		t.Fatalf("care rows after all member deletes = %d, want 0", remaining)
	}
}
