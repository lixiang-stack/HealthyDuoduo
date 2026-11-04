package analysis

import (
	"strconv"
	"testing"
	"testing/fstest"

	"healthyduoduo/internal/contract"
	"healthyduoduo/internal/pipeline"
)

func knowledgePoint(analyte string, flag string) EvidencePoint {
	name := map[string]string{
		contract.AnalyteALT:            "丙氨酸氨基转移酶测定",
		contract.AnalyteAST:            "天门冬氨酸氨基转移酶测定",
		contract.AnalyteALP:            "碱性磷酸酶测定",
		contract.AnalyteBilirubinTotal: "总胆红素测定",
	}[analyte]
	return EvidencePoint{
		EvidenceID: "ev-r1-" + analyte,
		Analyte:    analyte,
		Name:       name,
		Flag:       flag,
	}
}

func knowledgeFacts(points ...EvidencePoint) Facts {
	facts := Facts{Panel: pipeline.PanelLiver, Entry: EntryPanel}
	for _, point := range points {
		facts.History = append(facts.History, History{Latest: point, Points: []EvidencePoint{point}})
	}
	return facts
}

func TestKnowledgeMatcherMatchesExactFlagCombination(t *testing.T) {
	matcher, err := NewKnowledgeMatcher()
	if err != nil {
		t.Fatalf("load embedded knowledge: %v", err)
	}
	facts := knowledgeFacts(
		knowledgePoint(contract.AnalyteALT, contract.FlagHigh),
		knowledgePoint(contract.AnalyteAST, contract.FlagNormal),
		knowledgePoint(contract.AnalyteALP, contract.FlagNormal),
		knowledgePoint(contract.AnalyteBilirubinTotal, contract.FlagNormal),
	)
	matched := matcher.Match(testRequest(EntryPanel, pipeline.PanelLiver, 12), facts)
	if len(matched) != 1 || matched[0].ID != "liver-hepatocellular-mild" {
		t.Fatalf("matched = %+v, want liver-hepatocellular-mild", matched)
	}
	if matched[0].Explanation == "" || matched[0].Version != "1" ||
		matched[0].Review.Decision != ReviewDecisionAccepted ||
		matched[0].Review.ReviewedAt != "2026-11-02" {
		t.Fatalf("entry = %+v", matched[0])
	}
}

func TestKnowledgeMatcherRejectsPartialMatch(t *testing.T) {
	matcher, err := NewKnowledgeMatcher()
	if err != nil {
		t.Fatalf("load embedded knowledge: %v", err)
	}
	cases := map[string]Facts{
		"ALP high": knowledgeFacts(
			knowledgePoint(contract.AnalyteALT, contract.FlagHigh),
			knowledgePoint(contract.AnalyteAST, contract.FlagNormal),
			knowledgePoint(contract.AnalyteALP, contract.FlagHigh),
			knowledgePoint(contract.AnalyteBilirubinTotal, contract.FlagNormal),
		),
		"bilirubin missing": knowledgeFacts(
			knowledgePoint(contract.AnalyteALT, contract.FlagHigh),
			knowledgePoint(contract.AnalyteAST, contract.FlagNormal),
			knowledgePoint(contract.AnalyteALP, contract.FlagNormal),
		),
		"no transaminase high": knowledgeFacts(
			knowledgePoint(contract.AnalyteALT, contract.FlagNormal),
			knowledgePoint(contract.AnalyteAST, contract.FlagNormal),
			knowledgePoint(contract.AnalyteALP, contract.FlagNormal),
			knowledgePoint(contract.AnalyteBilirubinTotal, contract.FlagNormal),
		),
	}
	for name, facts := range cases {
		t.Run(name, func(t *testing.T) {
			if matched := matcher.Match(testRequest(EntryPanel, pipeline.PanelLiver, 12), facts); len(matched) != 0 {
				t.Fatalf("partial match = %+v, want none", matched)
			}
		})
	}
	if matched := matcher.Match(testRequest(EntryPanel, pipeline.PanelCBC, 12), knowledgeFacts(
		knowledgePoint(contract.AnalyteALT, contract.FlagHigh))); len(matched) != 0 {
		t.Fatalf("panel mismatch = %+v, want none", matched)
	}
}

func TestKnowledgeMatcherSkipsRejectedAndSortsByPriority(t *testing.T) {
	fsys := fstest.MapFS{
		"knowledge/a.yaml":        {Data: knowledgeYAML("a", 30, "accepted")},
		"knowledge/b.yaml":        {Data: knowledgeYAML("b", 10, "accepted")},
		"knowledge/c.yaml":        {Data: knowledgeYAML("c", 20, "accepted")},
		"knowledge/rejected.yaml": {Data: knowledgeYAML("rejected", 1, "rejected")},
	}
	entries, err := loadKnowledgeEntries(fsys)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	matcher := &KnowledgeMatcher{entries: entries}
	facts := knowledgeFacts(
		knowledgePoint(contract.AnalyteALT, contract.FlagHigh),
		knowledgePoint(contract.AnalyteAST, contract.FlagNormal),
		knowledgePoint(contract.AnalyteALP, contract.FlagNormal),
		knowledgePoint(contract.AnalyteBilirubinTotal, contract.FlagNormal),
	)
	matched := matcher.Match(testRequest(EntryPanel, pipeline.PanelLiver, 12), facts)
	if len(matched) != 2 || matched[0].ID != "b" || matched[1].ID != "c" {
		t.Fatalf("matched = %+v, want b,c (priority 10,20; top 2)", matched)
	}
}

func knowledgeYAML(id string, priority int, decision string) []byte {
	return []byte(`id: ` + id + `
priority: ` + strconv.Itoa(priority) + `
trigger:
  panel: liver
  any_flag_high: [alanine_aminotransferase]
  all_normal: [alkaline_phosphatase, bilirubin_total]
explanation: 条目原文
review:
  decision: ` + decision + `
  reviewed_at: 2026-11-02
version: 1
`)
}

func TestEntryVersions(t *testing.T) {
	if versions := EntryVersions(nil); versions != nil {
		t.Fatalf("EntryVersions(nil) = %v, want nil", versions)
	}
	versions := EntryVersions([]KnowledgeEntry{{ID: "a", Version: "2"}, {ID: "b", Version: "1"}})
	if versions["a"] != "2" || versions["b"] != "1" || len(versions) != 2 {
		t.Fatalf("versions = %v", versions)
	}
}

func TestLoadKnowledgeEntriesRejectsInvalidYAML(t *testing.T) {
	fsys := fstest.MapFS{
		"knowledge/bad.yaml": {Data: []byte("id: bad\nversion: 1\n")},
	}
	if _, err := loadKnowledgeEntries(fsys); err == nil {
		t.Fatal("invalid entry should fail loading")
	}
}
