// 成员领域规则单测:名 + 性别 + 出生日期;是否孩子按年龄实时计算。
package pipeline_test

import (
	"errors"
	"testing"
	"time"

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
	for _, sex := range []string{pipeline.SexMale, pipeline.SexFemale} {
		if err := pipeline.ValidateMemberSex(sex); err != nil {
			t.Errorf("ValidateMemberSex(%q) = %v, want nil", sex, err)
		}
	}
	for _, sex := range []string{"", "男", "unspecified", "other", "Male"} {
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
	for _, date := range []string{"", "2020-1-1", "2020-02-30", "2023-02-29", "2020/01/01", "1899-12-31"} {
		if !errors.Is(pipeline.ValidateMemberBirthDate(date), pipeline.ErrMemberBirthDateInvalid) {
			t.Errorf("ValidateMemberBirthDate(%q) = nil, want ErrMemberBirthDateInvalid", date)
		}
	}
}

func TestIsChild(t *testing.T) {
	now := time.Date(2026, 11, 2, 12, 0, 0, 0, time.UTC)
	cases := []struct {
		name     string
		birthDay string
		want     bool
	}{
		{"newborn", "2026-01-01", true},
		{"17 years old", "2009-11-03", true},
		{"turns 18 tomorrow", "2008-11-03", true},
		{"turns 18 today", "2008-11-02", false},
		{"adult", "1988-01-01", false},
		{"invalid date defensive", "not-a-date", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := pipeline.IsChild(c.birthDay, now); got != c.want {
				t.Errorf("IsChild(%q) = %v, want %v", c.birthDay, got, c.want)
			}
		})
	}
}

func TestValidateMemberProfile(t *testing.T) {
	valid := pipeline.MemberProfile{Name: "小虎", Sex: pipeline.SexMale, BirthDate: "2020-01-01"}
	if err := pipeline.ValidateMemberProfile(valid); err != nil {
		t.Fatalf("ValidateMemberProfile(valid) = %v, want nil", err)
	}
	invalid := valid
	invalid.Sex = "unspecified"
	if !errors.Is(pipeline.ValidateMemberProfile(invalid), pipeline.ErrMemberSexInvalid) {
		t.Errorf("ValidateMemberProfile(unspecified sex) = %v, want ErrMemberSexInvalid", pipeline.ValidateMemberProfile(invalid))
	}
}
