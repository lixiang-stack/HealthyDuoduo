package analysis

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestBuildEvidencePackRedactsIdentifiers(t *testing.T) {
	req := testRequest(EntryPanel, "liver", 12)
	req.MemberName = "小明"
	req.MemberBirthDate = "2016-05-20"
	req.MemberSex = "male"
	point := frameworkPoint(1, "丙氨酸转氨酶测定", "83.5", "U/L", "7-40", "high")
	point.ReportID = strings.Repeat("a", 64)
	point.ReportDate = strPtr("2026-08-01")
	point.EvidenceID = "ev-aaaaaaaaaaaa-1"
	facts := frameworkFacts(point)
	facts.Notes = []Note{{
		ReportID:      strings.Repeat("b", 64),
		ReportDate:    strPtr("2026-07-01"),
		OutsideWindow: true,
		Sections: []NoteSection{
			{Kind: "chief_complaint", Text: "最近不爱吃饭"},
			{Kind: "present_illness", Text: "现病史原文"},
			{Kind: "diagnosis", Text: "缺铁性贫血"},
			{Kind: "treatment", Text: "补铁治疗"},
			{Kind: "note", Text: "整页 OCR 原文"},
		},
	}}
	facts.Excluded = []Exclusion{
		{ReportID: strPtr(strings.Repeat("c", 64)), ReportDate: strPtr("2025-01-01"),
			Name: strPtr("丙氨酸转氨酶测定"), Reason: ReasonOutsideWindow},
	}
	entries := []KnowledgeEntry{{
		ID:          "liver-hepatocellular-mild",
		Explanation: "转氨酶升高而碱性磷酸酶正常，通常提示肝细胞受累。",
		Sources:     liverRSources,
	}}
	pack, mapping := BuildEvidencePack(req, facts, entries)
	raw, err := json.Marshal(pack)
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	for _, forbidden := range []string{
		"小明", "2016-05-20", strings.Repeat("a", 64), strings.Repeat("b", 64),
		"最近不爱吃饭", "现病史原文", "整页 OCR 原文", "ev-aaaaaaaaaaaa-1",
	} {
		if strings.Contains(text, forbidden) {
			t.Errorf("evidence pack leaked %q: %s", forbidden, text)
		}
	}
	for _, required := range []string{
		"缺铁性贫血", "补铁治疗", "T-3m", "T-4m", "10–11 岁",
		"liver-hepatocellular-mild", "AASLD / ACG",
	} {
		if !strings.Contains(text, required) {
			t.Errorf("evidence pack missing %q: %s", required, text)
		}
	}
	if !strings.Contains(text, `"outside_window":true`) {
		t.Errorf("evidence pack missing outside_window marker: %s", text)
	}
	if len(pack.Points) != 1 || pack.Points[0].EvidenceID != "e1" ||
		mapping["e1"].EvidenceID != "ev-aaaaaaaaaaaa-1" {
		t.Fatalf("pack points / mapping = %+v / %+v", pack.Points, mapping)
	}
}

func TestTrimEvidencePackDropsOldestPointsButKeepsEntries(t *testing.T) {
	pack := EvidencePack{
		Panel: "liver",
		Entries: []EvidencePackEntry{{
			EntryID: "entry-1",
			Text:    strings.Repeat("条目原文", 20),
			Sources: []Source{{
				Standard: "AASLD", Note: "来源",
			}},
		}},
		Points: []EvidencePackPoint{
			{EvidenceID: "e1", Name: "旧点", Value: "10", Flag: "normal", RelativeTime: "T-12m"},
			{EvidenceID: "e2", Name: "新点", Value: "20", Flag: "high", RelativeTime: "T-0"},
		},
	}
	fullTokens := estimatePackTokens(pack)
	trimmed, err := TrimEvidencePack(pack, fullTokens-1)
	if err != nil {
		t.Fatal(err)
	}
	if len(trimmed.Points) != 1 || trimmed.Points[0].EvidenceID != "e2" {
		t.Fatalf("trimmed points = %+v, want only newest", trimmed.Points)
	}
	if len(trimmed.Entries) != 1 {
		t.Fatal("entry text was trimmed")
	}
	if _, err := TrimEvidencePack(EvidencePack{Entries: pack.Entries}, 1); err != ErrEvidenceTooLarge {
		t.Fatalf("oversized entries error = %v, want ErrEvidenceTooLarge", err)
	}
}

func TestFilterEvidenceMappingDropsTrimmedPoints(t *testing.T) {
	mapping := EvidenceMapping{
		"e1": {EvidenceID: "ev-old"},
		"e2": {EvidenceID: "ev-new"},
	}
	pack := EvidencePack{Points: []EvidencePackPoint{{EvidenceID: "e2"}}}
	filtered := filterEvidenceMapping(pack, mapping)
	if len(filtered) != 1 || filtered["e2"].EvidenceID != "ev-new" {
		t.Fatalf("filtered mapping = %+v, want only e2", filtered)
	}
	if _, ok := filtered["e1"]; ok {
		t.Fatal("trimmed evidence id e1 still accepted")
	}
}

func TestEstimateTokens(t *testing.T) {
	if got := EstimateTokens("abcd"); got != 1 {
		t.Errorf("ascii estimate = %d, want 1", got)
	}
	if got := EstimateTokens("中文"); got != 2 {
		t.Errorf("cjk estimate = %d, want 2", got)
	}
}
