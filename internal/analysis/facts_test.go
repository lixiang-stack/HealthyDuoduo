package analysis

import (
	"slices"
	"testing"
	"time"
)

var testNow = time.Date(2026, 11, 3, 12, 0, 0, 0, time.UTC)

func testPoint(itemID int64, reportID, date, name, unit, unitStatus, value, flag string) InputPoint {
	return InputPoint{
		MemberID: 1, ReportID: reportID, ReportDate: date, ReportType: "生化",
		ReportStatus: "success", Panel: "liver", ItemID: itemID, Name: name,
		Code: "ALT", Unit: unit, UnitDisplay: unit, UnitStatus: unitStatus,
		Value: value, RefRange: "7-40", Flag: flag, ReviewStatus: "unreviewed",
	}
}

func testRequest(entry, panel string, months int) Request {
	return Request{
		MemberID: 1, MemberName: "小虎", Panel: panel, Entry: entry,
		WindowMonths: months, Now: testNow,
	}
}

func exclusionReasons(facts Facts) []string {
	reasons := make([]string, 0, len(facts.Excluded))
	for _, exclusion := range facts.Excluded {
		reasons = append(reasons, exclusion.Reason)
	}
	return reasons
}

func TestBuildFactsWindowAndPointCap(t *testing.T) {
	// 7 个有效点都在 12 个月内:两个边界同时生效,只保留最近 6 个。
	points := []InputPoint{
		testPoint(1, "r1", "2026-01-01", "丙氨酸转氨酶", "U/L", "verified", "20", "normal"),
		testPoint(2, "r2", "2026-02-01", "丙氨酸转氨酶", "U/L", "verified", "21", "normal"),
		testPoint(3, "r3", "2026-03-01", "丙氨酸转氨酶", "U/L", "verified", "22", "normal"),
		testPoint(4, "r4", "2026-04-01", "丙氨酸转氨酶", "U/L", "verified", "23", "normal"),
		testPoint(5, "r5", "2026-05-01", "丙氨酸转氨酶", "U/L", "verified", "24", "normal"),
		testPoint(6, "r6", "2026-06-01", "丙氨酸转氨酶", "U/L", "verified", "25", "normal"),
		testPoint(7, "r7", "2026-07-01", "丙氨酸转氨酶", "U/L", "verified", "26", "high"),
	}
	facts := BuildFacts(testRequest(EntryPanel, "liver", 0), points, nil)
	if facts.Window.Months != 12 || facts.Window.From != "2025-11-03" || facts.Window.To != "2026-11-03" {
		t.Fatalf("window = %+v", facts.Window)
	}
	if facts.Window.Sparse {
		t.Error("dense series marked sparse")
	}
	if len(facts.History) != 1 {
		t.Fatalf("history = %d, want 1", len(facts.History))
	}
	history := facts.History[0]
	if history.PointCount != 6 || len(history.Points) != 6 {
		t.Fatalf("point count = %d, want 6", history.PointCount)
	}
	if history.Points[0].ReportDate == nil || *history.Points[0].ReportDate != "2026-02-01" {
		t.Errorf("oldest kept point = %+v, want 2026-02-01 (cap keeps latest 6)", history.Points[0])
	}
	if !slices.Contains(exclusionReasons(facts), ReasonPointCap) {
		t.Errorf("exclusions = %v, want point_cap", exclusionReasons(facts))
	}
	if len(facts.Deviations) != 1 || facts.Deviations[0].Flag != "high" {
		t.Errorf("deviations = %+v, want latest high", facts.Deviations)
	}
	if !history.Highest {
		t.Error("latest point should be highest")
	}
}

func TestBuildFactsSparseExpansion(t *testing.T) {
	// 12 个月内只有 1 个有效点 → 扩展到 24 个月,最多 4 个点,并标 sparse。
	points := []InputPoint{
		testPoint(1, "r1", "2025-01-01", "丙氨酸转氨酶", "U/L", "verified", "18", "normal"),
		testPoint(2, "r2", "2025-06-01", "丙氨酸转氨酶", "U/L", "verified", "19", "normal"),
		testPoint(3, "r3", "2025-10-01", "丙氨酸转氨酶", "U/L", "verified", "20", "normal"),
		testPoint(4, "r4", "2026-10-01", "丙氨酸转氨酶", "U/L", "verified", "21", "normal"),
	}
	facts := BuildFacts(testRequest(EntryPanel, "liver", 12), points, nil)
	if !facts.Window.Sparse {
		t.Fatal("sparse series not marked")
	}
	if facts.Window.From != "2025-11-03" {
		t.Errorf("window from = %s, want 2025-11-03 (requested 12 months)", facts.Window.From)
	}
	history := facts.History[0]
	if !history.Sparse || history.PointCount != 4 {
		t.Fatalf("history = %+v, want sparse with 4 points", history)
	}
	if history.Change != ChangeUp {
		t.Errorf("change = %s, want up", history.Change)
	}
	if history.Lowest {
		t.Error("latest should not be lowest")
	}
}

func TestBuildFactsUserWindowDoesNotExpand(t *testing.T) {
	points := []InputPoint{
		testPoint(1, "r1", "2025-01-01", "丙氨酸转氨酶", "U/L", "verified", "18", "normal"),
		testPoint(2, "r2", "2026-10-01", "丙氨酸转氨酶", "U/L", "verified", "21", "normal"),
	}
	facts := BuildFacts(testRequest(EntryPanel, "liver", 6), points, nil)
	if facts.Window.Sparse || facts.Window.From != "2026-05-03" {
		t.Fatalf("window = %+v, want 6 months / not sparse", facts.Window)
	}
	if len(facts.History) != 1 || facts.History[0].PointCount != 1 {
		t.Fatalf("history = %+v, want only the in-window point", facts.History)
	}
	if !slices.Contains(exclusionReasons(facts), ReasonOutsideWindow) {
		t.Errorf("exclusions = %v, want outside_window", exclusionReasons(facts))
	}
}

func TestBuildFactsValidPointGates(t *testing.T) {
	points := []InputPoint{
		testPoint(1, "r1", "2026-10-01", "丙氨酸转氨酶", "U/L", "verified", "27.4", "normal"),
		testPoint(2, "r2", "2026-09-01", "丙氨酸转氨酶", "U/L", "unverified", "28", "normal"),
		testPoint(3, "r3", "2026-08-01", "丙氨酸转氨酶", "U/L", "verified", "29", "normal"),
		testPoint(4, "r4", "2026-07-01", "丙氨酸转氨酶", "U/L", "verified", "30", "normal"),
		testPoint(5, "r5", "2026-06-01", "丙氨酸转氨酶", "U/L", "verified", "31", "normal"),
		testPoint(6, "r6", "", "丙氨酸转氨酶", "U/L", "verified", "32", "normal"),
		testPoint(7, "r7", "2026-05-01", "丙氨酸转氨酶", "U/L", "verified", "", "normal"),
	}
	// 未核对低置信点应被排除;已修正 / 已确认的低置信点保留。
	points[2].LowConfidence = true
	points[3].LowConfidence = true
	points[3].ReviewStatus = "corrected"
	points[4].LowConfidence = true
	points[4].ReviewStatus = "confirmed"
	facts := BuildFacts(testRequest(EntryPanel, "liver", 12), points, nil)
	reasons := exclusionReasons(facts)
	for _, want := range []string{ReasonUnverifiedUnit, ReasonLowConfidence, ReasonNoDate, ReasonMissingValue} {
		if !slices.Contains(reasons, want) {
			t.Errorf("exclusions = %v, missing %s", reasons, want)
		}
	}
	if len(facts.History) != 1 {
		t.Fatalf("history = %d, want 1", len(facts.History))
	}
	// 最近 6 个有效点:r1、r4(已修正)、r5(已确认)、r3 被低置信排除;实际 r1/r4/r5。
	history := facts.History[0]
	if history.PointCount != 3 {
		t.Fatalf("points = %d, want 3 (%+v)", history.PointCount, history.Points)
	}
}

func TestBuildFactsNotTestedAndCrossUnit(t *testing.T) {
	points := []InputPoint{
		testPoint(1, "r1", "2026-10-01", "丙氨酸转氨酶", "U/L", "verified", "27.4", "normal"),
		testPoint(2, "r2", "2026-09-01", "丙氨酸转氨酶", "U/L", "verified", "", "not_tested"),
		testPoint(3, "r3", "2026-08-01", "丙氨酸转氨酶", "IU/L", "verified", "28", "normal"),
	}
	facts := BuildFacts(testRequest(EntryPanel, "liver", 12), points, nil)
	reasons := exclusionReasons(facts)
	if !slices.Contains(reasons, ReasonNotTested) {
		t.Errorf("exclusions = %v, want not_tested", reasons)
	}
	if !slices.Contains(reasons, ReasonUnitMismatch) {
		t.Errorf("exclusions = %v, want unit_mismatch", reasons)
	}
	if len(facts.History) != 1 || facts.History[0].PointCount != 1 {
		t.Fatalf("history = %+v, want only canonical-unit point", facts.History)
	}
}

func TestBuildFactsUnknownIsNotDeviation(t *testing.T) {
	points := []InputPoint{
		testPoint(1, "r1", "2026-10-01", "丙氨酸转氨酶", "U/L", "verified", "27.4", "unknown"),
	}
	facts := BuildFacts(testRequest(EntryPanel, "liver", 12), points, nil)
	if len(facts.Deviations) != 0 {
		t.Fatalf("deviations = %+v, want none", facts.Deviations)
	}
	if len(facts.Uncomparable) != 1 || facts.Uncomparable[0].Latest.Flag != "unknown" {
		t.Fatalf("uncomparable = %+v, want unknown point", facts.Uncomparable)
	}
	if facts.History[0].Comparable {
		t.Error("unknown latest point marked comparable")
	}
}

func TestBuildFactsDefaultExclusions(t *testing.T) {
	points := []InputPoint{
		testPoint(1, "r1", "2026-10-01", "丙氨酸转氨酶", "U/L", "verified", "27.4", "normal"),
		testPoint(2, "r2", "2026-10-01", "丙氨酸转氨酶", "U/L", "verified", "27.4", "normal"),
		testPoint(3, "r3", "2026-10-01", "丙氨酸转氨酶", "U/L", "verified", "27.4", "normal"),
	}
	points[1].MemberID = 2
	points[2].Panel = "kidney"
	failed := testPoint(4, "r4", "2026-10-01", "丙氨酸转氨酶", "U/L", "verified", "27.4", "normal")
	failed.ReportStatus = "failed"
	narrative := testPoint(5, "r5", "2026-10-01", "丙氨酸转氨酶", "U/L", "verified", "27.4", "normal")
	narrative.ReportType = "clinical_note"
	points = append(points, failed, narrative)

	facts := BuildFacts(testRequest(EntryPanel, "liver", 12), points, nil)
	reasons := exclusionReasons(facts)
	for _, want := range []string{ReasonCrossMember, ReasonWrongPanel, ReasonFailedReport, ReasonNarrativeReport} {
		if !slices.Contains(reasons, want) {
			t.Errorf("exclusions = %v, missing %s", reasons, want)
		}
	}
	if len(facts.History) != 1 || facts.History[0].PointCount != 1 {
		t.Fatalf("history = %+v, want only the valid point", facts.History)
	}
}

func TestBuildFactsNotes(t *testing.T) {
	notes := []InputNote{
		{
			MemberID: 1, ReportID: "n1", ReportDate: "2026-10-01", ReportType: "clinical_note",
			Sections: []InputSection{
				{Kind: "chief_complaint", Text: "最近不爱吃饭"},
				{Kind: "present_illness", Text: "现病史原文"},
				{Kind: "diagnosis", Text: "缺铁性贫血"},
				{Kind: "treatment", Text: "补铁治疗"},
			},
		},
		{
			MemberID: 1, ReportID: "n2", ReportDate: "2025-01-01", ReportType: "clinical_note",
			Sections: []InputSection{{Kind: "diagnosis", Text: "旧诊断"}},
		},
		{
			MemberID: 1, ReportID: "n3", ReportDate: "2024-01-01", ReportType: "clinical_note",
			Sections: []InputSection{{Kind: "diagnosis", Text: "更早诊断"}},
		},
		{
			MemberID: 1, ReportID: "n4", ReportDate: "2026-10-02", ReportType: "clinical_note",
			Sections: []InputSection{{Kind: "chief_complaint", Text: "最近不爱吃饭"}},
		},
	}
	points := []InputPoint{
		testPoint(1, "r1", "2026-10-01", "丙氨酸转氨酶", "U/L", "verified", "27.4", "normal"),
	}
	facts := BuildFacts(testRequest(EntryPanel, "liver", 12), points, notes)
	// 病历窗口跟请求的 12 个月一致:n1 在窗口内,n2 是最近一次窗口外病历,
	// n3 更早被排除。
	if len(facts.Notes) != 2 {
		t.Fatalf("notes = %d, want 2 (in window + latest prior)", len(facts.Notes))
	}
	if facts.Notes[0].ReportID != "n1" || facts.Notes[0].OutsideWindow {
		t.Errorf("first note = %+v, want n1 in window", facts.Notes[0])
	}
	if len(facts.Notes[0].Sections) != 2 ||
		facts.Notes[0].Sections[0].Kind != "diagnosis" ||
		facts.Notes[0].Sections[1].Kind != "treatment" {
		t.Errorf("note sections = %+v, want diagnosis/treatment only", facts.Notes[0].Sections)
	}
	if facts.Notes[1].ReportID != "n2" || !facts.Notes[1].OutsideWindow {
		t.Errorf("second note = %+v, want n2 outside window", facts.Notes[1])
	}
	if !slices.Contains(exclusionReasons(facts), ReasonOutsideWindow) {
		t.Errorf("exclusions = %v, want older note outside_window", exclusionReasons(facts))
	}
	if !slices.Contains(exclusionReasons(facts), ReasonNoNoteBody) {
		t.Errorf("exclusions = %v, want note without diagnosis/treatment excluded", exclusionReasons(facts))
	}
}

func TestBuildFactsNoteFallback(t *testing.T) {
	notes := []InputNote{{
		MemberID: 1, ReportID: "n1", ReportDate: "2026-10-01", ReportType: "clinical_note",
		Sections: []InputSection{{Kind: "note", Text: "整页原文"}},
	}}
	facts := BuildFacts(testRequest(EntryClinicalNote, "clinical_note", 12), nil, notes)
	if len(facts.Notes) != 1 || len(facts.Notes[0].Sections) != 1 ||
		facts.Notes[0].Sections[0].Kind != "note" {
		t.Fatalf("note fallback = %+v", facts.Notes)
	}
	if facts.InsufficientEvidence {
		t.Error("clinical note entry with note marked insufficient evidence")
	}
}

func TestBuildFactsClinicalNoteWithoutDate(t *testing.T) {
	notes := []InputNote{{
		MemberID: 1, ReportID: "n1", ReportDate: "", ReportType: "clinical_note",
		Sections: []InputSection{{Kind: "diagnosis", Text: "缺铁性贫血"}},
	}}
	req := testRequest(EntryClinicalNote, "clinical_note", 12)
	req.ReportSHA = "n1"
	facts := BuildFacts(req, nil, notes)
	if len(facts.Notes) != 1 || facts.Notes[0].ReportDate != nil || facts.Notes[0].OutsideWindow {
		t.Fatalf("notes = %+v, want the selected dateless note", facts.Notes)
	}
	if facts.InsufficientEvidence {
		t.Error("selected clinical note marked insufficient evidence")
	}
}

func TestBuildFactsInsufficientEvidence(t *testing.T) {
	facts := BuildFacts(testRequest(EntryPanel, "liver", 12), nil, nil)
	if !facts.InsufficientEvidence {
		t.Fatal("no evidence should be insufficient")
	}
	if facts.Reports == nil || facts.History == nil || facts.Excluded == nil {
		t.Fatal("facts slices must be non-nil for JSON []")
	}
}

func TestBuildFactsInsufficientEvidenceWithNotes(t *testing.T) {
	notes := []InputNote{{
		MemberID: 1, ReportID: "n1", ReportDate: "2026-10-01", ReportType: "clinical_note",
		Sections: []InputSection{{Kind: "diagnosis", Text: "缺铁性贫血"}},
	}}
	panel := BuildFacts(testRequest(EntryPanel, "liver", 12), nil, notes)
	if !panel.InsufficientEvidence {
		t.Error("panel analysis with notes but no valid points should be insufficient")
	}
	clinical := BuildFacts(testRequest(EntryClinicalNote, "clinical_note", 12), nil, notes)
	if clinical.InsufficientEvidence {
		t.Error("clinical_note entry with a note should not be insufficient")
	}
}

func TestBuildSnapshot(t *testing.T) {
	points := []InputPoint{
		testPoint(1, "r1", "2026-10-01", "丙氨酸转氨酶", "U/L", "verified", "27.4", "normal"),
	}
	notes := []InputNote{{
		MemberID: 1, ReportID: "n1", ReportDate: "2026-10-01", ReportType: "clinical_note",
		Sections: []InputSection{{Kind: "diagnosis", Text: "缺铁性贫血"}},
	}}
	req := testRequest(EntryIndicator, "liver", 12)
	req.IndicatorName = "丙氨酸转氨酶"
	facts := BuildFacts(req, points, notes)
	snapshot := BuildSnapshot(req, facts)
	if snapshot.MemberID != 1 || snapshot.MemberName != "小虎" || snapshot.Panel != "liver" ||
		snapshot.Entry != EntryIndicator || snapshot.IndicatorName == nil ||
		*snapshot.IndicatorName != "丙氨酸转氨酶" || snapshot.RuleVersion != RuleVersion {
		t.Fatalf("snapshot header = %+v", snapshot)
	}
	if len(snapshot.Points) != 1 || snapshot.Points[0].EvidenceID != "ev-r1-1" ||
		snapshot.Points[0].TrendKey != "ALT" || snapshot.Points[0].Flag != "normal" {
		t.Fatalf("snapshot points = %+v", snapshot.Points)
	}
	if len(snapshot.Notes) != 1 || !slices.Equal(snapshot.Notes[0].SectionKinds, []string{"diagnosis"}) {
		t.Fatalf("snapshot notes = %+v", snapshot.Notes)
	}
}
