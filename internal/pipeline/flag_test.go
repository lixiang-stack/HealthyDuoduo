// Compare 单元测试:读共享语义向量(schemas/testdata/flag.vectors.json)——
// 识别端 _compare 与本包 Compare 读同一份(改语义先改向量表,双侧测试强制通过),
// 与 Python 侧 recognizer/tests/test_flag_vectors.py 成对。
package pipeline

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestCompare(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "schemas", "testdata", "flag.vectors.json"))
	if err != nil {
		t.Fatalf("read shared flag vectors: %v", err)
	}
	policy, err := LoadFlagPolicy(filepath.Join("..", "..", "recognizer", "recognizer", "flag_policy.yaml"))
	if err != nil {
		t.Fatalf("load flag policy: %v", err)
	}
	var vectors struct {
		Cases []struct {
			Note   string  `json:"note"`
			Name   *string `json:"name"`
			Value  *string `json:"value"`
			Ref    *string `json:"ref"`
			Flag   string  `json:"flag"`
			Reason *string `json:"reason"`
		} `json:"cases"`
	}
	if err := json.Unmarshal(raw, &vectors); err != nil {
		t.Fatalf("parse shared flag vectors: %v", err)
	}
	if len(vectors.Cases) == 0 {
		t.Fatal("shared flag vectors empty")
	}
	for _, c := range vectors.Cases {
		t.Run(c.Note, func(t *testing.T) {
			wantReason := ""
			if c.Reason != nil {
				wantReason = *c.Reason
			}
			got, reason := Compare(strOrEmpty(c.Name), c.Value, c.Ref, policy)
			if got != c.Flag {
				t.Errorf("Compare(%v, %v) = %q, want %q", deref(c.Value), deref(c.Ref), got, c.Flag)
			}
			if reason != wantReason {
				t.Errorf("Compare(%v, %v) reason = %q, want %q", deref(c.Value), deref(c.Ref), reason, wantReason)
			}
		})
	}
}

func str(s string) *string { return &s }

func deref(s *string) string {
	if s == nil {
		return "<nil>"
	}
	return *s
}

func strOrEmpty(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
