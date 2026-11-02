// 成员领域规则单测:名 + 性别 + 出生日期及精度(llm-analysis 票 01)。
package pipeline_test

import (
	"errors"
	"testing"

	"healthyduoduo/internal/pipeline"
)

func TestValidateMemberName(t *testing.T) {
	cases := []struct {
		name  string
		input string
		valid bool
	}{
		{"normal", "小虎", true},
		{"padded", " 小虎 ", true},
		{"single char", "a", true},
		{"empty", "", false},
		{"spaces", "   ", false},
		{"control blanks", "\t\n", false},
		{"ideographic space", "\u3000", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := pipeline.ValidateMemberName(c.input)
			if c.valid {
				if err != nil {
					t.Errorf("ValidateMemberName(%q) = %v, want nil", c.input, err)
				}
				return
			}
			if !errors.Is(err, pipeline.ErrMemberNameBlank) {
				t.Errorf("ValidateMemberName(%q) = %v, want ErrMemberNameBlank", c.input, err)
			}
		})
	}
}

func TestValidateMemberSex(t *testing.T) {
	for _, sex := range []string{pipeline.SexMale, pipeline.SexFemale, pipeline.SexUnspecified} {
		if err := pipeline.ValidateMemberSex(sex); err != nil {
			t.Errorf("ValidateMemberSex(%q) = %v, want nil", sex, err)
		}
	}
	for _, sex := range []string{"", "男", "other", "Male"} {
		if !errors.Is(pipeline.ValidateMemberSex(sex), pipeline.ErrMemberSexInvalid) {
			t.Errorf("ValidateMemberSex(%q) = nil, want ErrMemberSexInvalid", sex)
		}
	}
}

func TestValidateMemberBirthDate(t *testing.T) {
	for _, date := range []string{"2020-01-01", "2024-02-29"} {
		if err := pipeline.ValidateMemberBirthDate(date); err != nil {
			t.Errorf("ValidateMemberBirthDate(%q) = %v, want nil", date, err)
		}
	}
	for _, date := range []string{"", "2020-1-1", "2020-02-30", "2023-02-29", "2020/01/01"} {
		if !errors.Is(pipeline.ValidateMemberBirthDate(date), pipeline.ErrMemberBirthDateInvalid) {
			t.Errorf("ValidateMemberBirthDate(%q) = nil, want ErrMemberBirthDateInvalid", date)
		}
	}
}

func TestValidateMemberBirthDatePrecision(t *testing.T) {
	for _, precision := range []string{pipeline.BirthDatePrecisionDay, pipeline.BirthDatePrecisionYear} {
		if err := pipeline.ValidateMemberBirthDatePrecision(precision); err != nil {
			t.Errorf("ValidateMemberBirthDatePrecision(%q) = %v, want nil", precision, err)
		}
	}
	for _, precision := range []string{"", "month", "DAY"} {
		if !errors.Is(pipeline.ValidateMemberBirthDatePrecision(precision), pipeline.ErrMemberBirthDatePrecisionInvalid) {
			t.Errorf("ValidateMemberBirthDatePrecision(%q) = nil, want ErrMemberBirthDatePrecisionInvalid", precision)
		}
	}
}
