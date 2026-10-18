// ComputeFlag 单元测试:读共享语义向量(schemas/testdata/flag.vectors.json)——
// 识别端 _flag 与本包 ComputeFlag 读同一份(改语义先改向量表,双侧测试强制通过),
// 与 Python 侧 recognizer/tests/test_flag_vectors.py 成对。
package pipeline

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestComputeFlag(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "schemas", "testdata", "flag.vectors.json"))
	if err != nil {
		t.Fatalf("read shared flag vectors: %v", err)
	}
	var vectors struct {
		Cases []struct {
			Note  string  `json:"note"`
			Value *string `json:"value"`
			Ref   *string `json:"ref"`
			Flag  string  `json:"flag"`
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
			if got := ComputeFlag(c.Value, c.Ref); got != c.Flag {
				t.Errorf("ComputeFlag(%v, %v) = %q, want %q", deref(c.Value), deref(c.Ref), got, c.Flag)
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
