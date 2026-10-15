// 成员名领域规则单测:含非空白字符即有效;空串/纯空白(含全角空格)拒绝。
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
