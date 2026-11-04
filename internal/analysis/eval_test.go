package analysis_test

import (
	"encoding/json"
	"os"
	"testing"

	"healthyduoduo/internal/analysis"
	"healthyduoduo/internal/analysis/evalfixture"
)

const (
	wantFactsCases     = 110
	wantFrameworkCases = 29
)

// 评估集冻结(llm-analysis 票 10):fixtures.json 由 gen/eval 生成一次、人工确认后提交;
// 测试只读冻结文件,不重新生成,代码改动改变事实层 / 框架层输出即失败。
func TestEvalFixtures(t *testing.T) {
	raw, err := os.ReadFile("testdata/eval/fixtures.json")
	if err != nil {
		t.Fatalf("read frozen fixtures: %v\nrun `go run ./internal/analysis/gen/eval` from the repo root and review the diff", err)
	}
	var fixture evalfixture.Fixture
	if err := json.Unmarshal(raw, &fixture); err != nil {
		t.Fatalf("parse frozen fixtures: %v", err)
	}
	if fixture.RuleVersion != analysis.RuleVersion {
		t.Fatalf("fixture rule version = %q, want %q; regenerate and review the fixture",
			fixture.RuleVersion, analysis.RuleVersion)
	}
	if len(fixture.Facts) != wantFactsCases {
		t.Fatalf("facts cases = %d, want %d; regenerate and review the fixture", len(fixture.Facts), wantFactsCases)
	}
	if len(fixture.Framework) != wantFrameworkCases {
		t.Fatalf("framework cases = %d, want %d; regenerate and review the fixture",
			len(fixture.Framework), wantFrameworkCases)
	}

	for _, c := range fixture.Facts {
		c := c
		t.Run("facts/"+c.Name, func(t *testing.T) {
			got := analysis.BuildFacts(c.Request, c.Points, nil)
			assertJSONEqual(t, got, c.Expected)
		})
	}
	for _, c := range fixture.Framework {
		c := c
		t.Run("framework/"+c.Name, func(t *testing.T) {
			facts := analysis.BuildFacts(c.Request, c.Points, nil)
			got := analysis.BuildFramework(c.Request, facts, c.Growth)
			assertJSONEqual(t, got, c.ExpectedFramework)
		})
	}
}

func assertJSONEqual(t *testing.T, got, want any) {
	t.Helper()
	gotJSON, err := json.Marshal(got)
	if err != nil {
		t.Fatalf("marshal got: %v", err)
	}
	wantJSON, err := json.Marshal(want)
	if err != nil {
		t.Fatalf("marshal want: %v", err)
	}
	if string(gotJSON) != string(wantJSON) {
		t.Fatalf("output changed from frozen fixture:\n got  %s\n want %s", gotJSON, wantJSON)
	}
}
