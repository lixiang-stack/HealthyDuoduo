// 护理事件领域校验单测（care-events 票 01）：四类型合法形状、类型专属字段互斥、
// 数值与时间顺序，以及 note / food 的文本规范化。
package care_test

import (
	"errors"
	"math"
	"testing"
	"time"

	"healthyduoduo/internal/care"
)

func ptr[T any](value T) *T { return &value }

func TestValidateValidEvents(t *testing.T) {
	start := time.Date(2026, 11, 23, 7, 30, 0, 0, time.FixedZone("CST", 8*3600))
	end := time.Date(2026, 11, 23, 9, 0, 0, 0, start.Location())

	cases := []struct {
		name  string
		input care.EventInput
	}{
		{"feed formula with amount", care.EventInput{
			Type: care.TypeFeed, StartedAt: start,
			Method: ptr(care.FeedMethodFormula), AmountML: ptr(120.0),
		}},
		{"feed breast with duration", care.EventInput{
			Type: care.TypeFeed, StartedAt: start,
			Method: ptr(care.FeedMethodBreast), DurationMin: ptr(15),
		}},
		{"diaper wet", care.EventInput{
			Type: care.TypeDiaper, StartedAt: start, DiaperType: ptr(care.DiaperTypeWet),
		}},
		{"diaper dirty", care.EventInput{
			Type: care.TypeDiaper, StartedAt: start, DiaperType: ptr(care.DiaperTypeDirty),
		}},
		{"diaper mixed", care.EventInput{
			Type: care.TypeDiaper, StartedAt: start, DiaperType: ptr(care.DiaperTypeMixed),
		}},
		{"sleep ongoing", care.EventInput{
			Type: care.TypeSleep, StartedAt: start,
		}},
		{"sleep completed", care.EventInput{
			Type: care.TypeSleep, StartedAt: start, EndedAt: &end, DurationMin: ptr(90),
		}},
		{"solid with amount", care.EventInput{
			Type: care.TypeSolid, StartedAt: start, Food: ptr("南瓜泥"), AmountG: ptr(30.0),
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if err := care.Validate(c.input); err != nil {
				t.Fatalf("valid input rejected: %v", err)
			}
		})
	}
}

func TestValidateRejectsInvalidEvents(t *testing.T) {
	start := time.Date(2026, 11, 23, 7, 30, 0, 0, time.FixedZone("CST", 8*3600))
	earlier := start.Add(-time.Minute)
	feed := func() care.EventInput {
		return care.EventInput{Type: care.TypeFeed, StartedAt: start, Method: ptr(care.FeedMethodFormula)}
	}
	diaper := func() care.EventInput {
		return care.EventInput{Type: care.TypeDiaper, StartedAt: start, DiaperType: ptr(care.DiaperTypeWet)}
	}
	sleep := func() care.EventInput {
		return care.EventInput{Type: care.TypeSleep, StartedAt: start}
	}
	solid := func() care.EventInput {
		return care.EventInput{Type: care.TypeSolid, StartedAt: start, Food: ptr("米糊")}
	}

	cases := []struct {
		name  string
		input care.EventInput
		want  error
	}{
		{"unknown type", care.EventInput{Type: "pump", StartedAt: start}, care.ErrTypeInvalid},
		{"missing started_at", care.EventInput{Type: care.TypeFeed, Method: ptr(care.FeedMethodFormula)}, care.ErrStartedAtRequired},
		{"ended before started", care.EventInput{
			Type: care.TypeSleep, StartedAt: start, EndedAt: &earlier, DurationMin: ptr(10),
		}, care.ErrEndedAtBeforeStarted},

		{"feed missing method", care.EventInput{Type: care.TypeFeed, StartedAt: start}, care.ErrFieldsInvalid},
		{"feed invalid method", care.EventInput{
			Type: care.TypeFeed, StartedAt: start, Method: ptr("bottle"),
		}, care.ErrFieldsInvalid},
		{"feed with ended_at", care.EventInput{
			Type: care.TypeFeed, StartedAt: start, Method: ptr(care.FeedMethodBreast), EndedAt: &start,
		}, care.ErrFieldsInvalid},
		{"feed with diaper_type", func() care.EventInput {
			in := feed()
			in.DiaperType = ptr(care.DiaperTypeWet)
			return in
		}(), care.ErrFieldsInvalid},
		{"feed with food", func() care.EventInput {
			in := feed()
			in.Food = ptr("米糊")
			return in
		}(), care.ErrFieldsInvalid},
		{"feed with amount_g", func() care.EventInput {
			in := feed()
			in.AmountG = ptr(30.0)
			return in
		}(), care.ErrFieldsInvalid},

		{"diaper missing diaper_type", care.EventInput{Type: care.TypeDiaper, StartedAt: start}, care.ErrFieldsInvalid},
		{"diaper invalid diaper_type", care.EventInput{
			Type: care.TypeDiaper, StartedAt: start, DiaperType: ptr("dry"),
		}, care.ErrFieldsInvalid},
		{"diaper with ended_at", func() care.EventInput {
			in := diaper()
			in.EndedAt = &start
			return in
		}(), care.ErrFieldsInvalid},
		{"diaper with method", func() care.EventInput {
			in := diaper()
			in.Method = ptr(care.FeedMethodBreast)
			return in
		}(), care.ErrFieldsInvalid},
		{"diaper with amount_ml", func() care.EventInput {
			in := diaper()
			in.AmountML = ptr(120.0)
			return in
		}(), care.ErrFieldsInvalid},
		{"diaper with duration_min", func() care.EventInput {
			in := diaper()
			in.DurationMin = ptr(5)
			return in
		}(), care.ErrFieldsInvalid},
		{"diaper with food", func() care.EventInput {
			in := diaper()
			in.Food = ptr("米糊")
			return in
		}(), care.ErrFieldsInvalid},
		{"diaper with amount_g", func() care.EventInput {
			in := diaper()
			in.AmountG = ptr(30.0)
			return in
		}(), care.ErrFieldsInvalid},

		{"sleep ended without duration", func() care.EventInput {
			in := sleep()
			in.EndedAt = &start
			return in
		}(), care.ErrFieldsInvalid},
		{"sleep duration without ended", func() care.EventInput {
			in := sleep()
			in.DurationMin = ptr(30)
			return in
		}(), care.ErrFieldsInvalid},
		{"sleep with method", func() care.EventInput {
			in := sleep()
			in.Method = ptr(care.FeedMethodBreast)
			return in
		}(), care.ErrFieldsInvalid},
		{"sleep with amount_ml", func() care.EventInput {
			in := sleep()
			in.AmountML = ptr(120.0)
			return in
		}(), care.ErrFieldsInvalid},
		{"sleep with diaper_type", func() care.EventInput {
			in := sleep()
			in.DiaperType = ptr(care.DiaperTypeWet)
			return in
		}(), care.ErrFieldsInvalid},
		{"sleep with food", func() care.EventInput {
			in := sleep()
			in.Food = ptr("米糊")
			return in
		}(), care.ErrFieldsInvalid},
		{"sleep with amount_g", func() care.EventInput {
			in := sleep()
			in.AmountG = ptr(30.0)
			return in
		}(), care.ErrFieldsInvalid},

		{"solid missing food", care.EventInput{Type: care.TypeSolid, StartedAt: start}, care.ErrFieldsInvalid},
		{"solid blank food", func() care.EventInput {
			in := solid()
			in.Food = ptr(" \t ")
			return in
		}(), care.ErrFieldsInvalid},
		{"solid with ended_at", func() care.EventInput {
			in := solid()
			in.EndedAt = &start
			return in
		}(), care.ErrFieldsInvalid},
		{"solid with method", func() care.EventInput {
			in := solid()
			in.Method = ptr(care.FeedMethodBreast)
			return in
		}(), care.ErrFieldsInvalid},
		{"solid with amount_ml", func() care.EventInput {
			in := solid()
			in.AmountML = ptr(120.0)
			return in
		}(), care.ErrFieldsInvalid},
		{"solid with duration_min", func() care.EventInput {
			in := solid()
			in.DurationMin = ptr(5)
			return in
		}(), care.ErrFieldsInvalid},
		{"solid with diaper_type", func() care.EventInput {
			in := solid()
			in.DiaperType = ptr(care.DiaperTypeWet)
			return in
		}(), care.ErrFieldsInvalid},

		{"zero amount_ml", func() care.EventInput {
			in := feed()
			in.AmountML = ptr(0.0)
			return in
		}(), care.ErrValueInvalid},
		{"negative amount_ml", func() care.EventInput {
			in := feed()
			in.AmountML = ptr(-1.0)
			return in
		}(), care.ErrValueInvalid},
		{"NaN amount_ml", func() care.EventInput {
			in := feed()
			in.AmountML = ptr(math.NaN())
			return in
		}(), care.ErrValueInvalid},
		{"zero amount_g", func() care.EventInput {
			in := solid()
			in.AmountG = ptr(0.0)
			return in
		}(), care.ErrValueInvalid},
		{"negative duration_min", func() care.EventInput {
			in := feed()
			in.DurationMin = ptr(-5)
			return in
		}(), care.ErrValueInvalid},
		{"zero sleep duration", care.EventInput{
			Type: care.TypeSleep, StartedAt: start, EndedAt: &start, DurationMin: ptr(0),
		}, care.ErrValueInvalid},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := care.Validate(c.input)
			if !errors.Is(err, c.want) {
				t.Fatalf("error = %v, want %v", err, c.want)
			}
		})
	}
}

func TestNormalizeTextFields(t *testing.T) {
	note := "  早晨配方奶 \n"
	food := "  南瓜泥  "
	input := care.Normalize(care.EventInput{Type: care.TypeSolid, Note: &note, Food: &food})
	if input.Note == nil || *input.Note != "早晨配方奶" {
		t.Fatalf("note = %v, want 早晨配方奶", input.Note)
	}
	if input.Food == nil || *input.Food != "南瓜泥" {
		t.Fatalf("food = %v, want 南瓜泥", input.Food)
	}

	blankNote, blankFood := " \t ", "  "
	start := time.Date(2026, 11, 23, 7, 30, 0, 0, time.FixedZone("CST", 8*3600))
	input = care.Normalize(care.EventInput{
		Type: care.TypeFeed, StartedAt: start, Note: &blankNote, Food: &blankFood,
	})
	if input.Note != nil {
		t.Fatalf("blank note = %v, want nil", *input.Note)
	}
	// 非 solid 保留原值，由 Validate 按多余类型字段拒绝。
	if input.Food == nil || *input.Food != blankFood {
		t.Fatalf("non-solid food = %v, want original value", input.Food)
	}
	if err := care.Validate(input); !errors.Is(err, care.ErrFieldsInvalid) {
		t.Fatalf("non-solid food error = %v, want ErrFieldsInvalid", err)
	}

	blankFood = "  "
	input = care.Normalize(care.EventInput{Type: care.TypeSolid, Food: &blankFood})
	if input.Food != nil {
		t.Fatalf("blank solid food = %v, want nil", *input.Food)
	}
}
