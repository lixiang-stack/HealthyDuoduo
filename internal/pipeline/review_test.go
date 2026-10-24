// ApplyReviewAction 单元测试:确认/修正的生效值、flag 重算与修正对形状
// (ADR-0005:确认不产生修正对;每次修正 append 一条,before = 上次生效值)。
package pipeline

import (
	"reflect"
	"testing"

	"healthyduoduo/internal/contract"
)

func reviewRow() ReportItemRow {
	return ReportItemRow{
		ItemID:       7,
		ReviewStatus: ReviewStatusUnreviewed,
		ReportItem: contract.ReportItem{
			Name:           "单核细胞比率",
			Value:          str("10.8"),
			Unit:           str("%"),
			RefRange:       str("3-10"),
			Flag:           FlagHigh,
			LowConfidence:  true,
			RawText:        "单核细胞百分比  10.8  ↑ %  3-10",
			OCRLineNumbers: []int32{1, 2, 3, 4},
		},
	}
}

func TestApplyReviewActionConfirm(t *testing.T) {
	row := reviewRow()
	next, pair := ApplyReviewAction(row, ReviewAction{Confirm: true})
	if pair != nil {
		t.Fatalf("confirm pair = %+v, want nil(确认不产生修正对)", pair)
	}
	if next.ReviewStatus != ReviewStatusConfirmed {
		t.Errorf("review_status = %q, want confirmed", next.ReviewStatus)
	}
	// 除核对状态外,四项纸面事实与 flag 不动。
	want := row
	want.ReviewStatus = ReviewStatusConfirmed
	if !reflect.DeepEqual(next, want) {
		t.Errorf("confirm changed facts: %+v -> %+v", row, next)
	}
}

func TestApplyReviewActionCorrectValue(t *testing.T) {
	row := reviewRow()
	next, pair := ApplyReviewAction(row, ReviewAction{Value: str("5.6")})
	if next.ReviewStatus != ReviewStatusCorrected {
		t.Errorf("review_status = %q, want corrected", next.ReviewStatus)
	}
	if *next.Value != "5.6" {
		t.Errorf("value = %q, want 5.6", *next.Value)
	}
	if next.Flag != FlagNormal {
		t.Errorf("flag = %q, want normal (5.6 × 3-10 重算)", next.Flag)
	}
	// 其余字段原样。
	if next.Name != row.Name || *next.Unit != "%" || *next.RefRange != "3-10" || next.Method != nil || next.RawText != row.RawText {
		t.Errorf("corrected row drifted: %+v", next)
	}
	if pair == nil {
		t.Fatal("correct pair = nil, want 修正对")
	}
	if pair.BeforeName == nil || *pair.BeforeName != "单核细胞比率" || *pair.BeforeValue != "10.8" || *pair.BeforeUnit != "%" || *pair.BeforeRefRange != "3-10" {
		t.Errorf("pair before = %+v, want 上次生效值", pair)
	}
	if pair.AfterName == nil || *pair.AfterName != "单核细胞比率" || *pair.AfterValue != "5.6" || *pair.AfterUnit != "%" || *pair.AfterRefRange != "3-10" {
		t.Errorf("pair after = %+v, want 本次生效值", pair)
	}
	if pair.RawText != row.RawText || !reflect.DeepEqual(pair.OCRLineNumbers, row.OCRLineNumbers) {
		t.Errorf("pair raw_text/line numbers = %q/%v, want 随项原样", pair.RawText, pair.OCRLineNumbers)
	}
}

func TestApplyReviewActionCorrectRefRangeOnly(t *testing.T) {
	row := reviewRow() // value 10.8、ref 3-10 → high
	next, pair := ApplyReviewAction(row, ReviewAction{RefRange: str("3-12")})
	if next.Flag != FlagNormal {
		t.Errorf("flag = %q, want normal (10.8 × 3-12 重算)", next.Flag)
	}
	if *next.Value != "10.8" || *next.RefRange != "3-12" {
		t.Errorf("facts = %v/%v, want 10.8/3-12", *next.Value, *next.RefRange)
	}
	if pair.BeforeRefRange == nil || *pair.BeforeRefRange != "3-10" || pair.AfterRefRange == nil || *pair.AfterRefRange != "3-12" {
		t.Errorf("pair ref_range = %v -> %v, want 3-10 -> 3-12", pair.BeforeRefRange, pair.AfterRefRange)
	}
}

func TestApplyReviewActionCorrectRepeatable(t *testing.T) {
	row := reviewRow()
	first, _ := ApplyReviewAction(row, ReviewAction{Value: str("5.6")})
	second, pair := ApplyReviewAction(first, ReviewAction{Value: str("5.6")})
	// 重放同一修正:生效状态不变;修正对 before = 上次生效值(no-op 对,ADR-0005 决策可重复)。
	if *second.Value != "5.6" || second.ReviewStatus != ReviewStatusCorrected || second.Flag != FlagNormal {
		t.Errorf("replayed correction state = %+v, want 与首次一致", second)
	}
	if pair.BeforeValue == nil || *pair.BeforeValue != "5.6" || pair.AfterValue == nil || *pair.AfterValue != "5.6" {
		t.Errorf("replay pair = %v -> %v, want before=上次生效值 5.6", pair.BeforeValue, pair.AfterValue)
	}
}

func TestApplyReviewActionCorrectNameAndUnit(t *testing.T) {
	row := reviewRow()
	next, pair := ApplyReviewAction(row, ReviewAction{Name: str("单核细胞百分比"), Unit: str("pct")})
	if next.Name != "单核细胞百分比" || *next.Unit != "pct" {
		t.Errorf("facts = %q/%v, want 单核细胞百分比/pct", next.Name, *next.Unit)
	}
	// value/ref 未动 → flag 按现值重算,结果不变(high)。
	if next.Flag != FlagHigh {
		t.Errorf("flag = %q, want high(仅改名/单位不改比较结果)", next.Flag)
	}
	if pair.AfterName == nil || *pair.AfterName != "单核细胞百分比" || pair.AfterUnit == nil || *pair.AfterUnit != "pct" {
		t.Errorf("pair after = %+v", pair)
	}
}

func TestApplyReviewActionScalarZeroReference(t *testing.T) {
	row := ReportItemRow{
		ItemID: 11,
		ReportItem: contract.ReportItem{
			Name:     "有核红细胞绝对值",
			Value:    str("0.00"),
			RefRange: nil,
			Flag:     FlagUnknown,
			RawText:  "有核红细胞绝对值  NRBC#  0.00  *10",
		},
	}
	next, _ := ApplyReviewAction(row, ReviewAction{RefRange: str("0")})
	if next.Flag != FlagNormal || next.FlagReason != nil {
		t.Fatalf("scalar 0 correction = %q/%v, want normal/nil", next.Flag, next.FlagReason)
	}
}

func TestApplyReviewActionNoReference(t *testing.T) {
	row := ReportItemRow{
		ItemID:       10,
		ReviewStatus: ReviewStatusUnreviewed,
		ReportItem: contract.ReportItem{
			Name:     "中性细胞数",
			Value:    str("4.19"),
			RefRange: nil,
			Flag:     FlagUnknown,
			RawText:  "NEUT  中性细胞数  4.19",
		},
	}
	next, pair := ApplyReviewAction(row, ReviewAction{NoReference: true})
	if pair != nil {
		t.Fatalf("no_reference pair = %+v, want nil(不改纸面事实)", pair)
	}
	if next.ReviewStatus != ReviewStatusConfirmed || next.Flag != FlagUnknown {
		t.Fatalf("no_reference state = %q/%q, want confirmed/unknown", next.ReviewStatus, next.Flag)
	}
	if next.FlagReason == nil || *next.FlagReason != FlagReasonNoReference {
		t.Fatalf("no_reference reason = %v, want no_reference", next.FlagReason)
	}
}

func TestApplyReviewActionWithPolicyQualitative(t *testing.T) {
	policy, err := LoadFlagPolicy("../../recognizer/recognizer/flag_policy.yaml")
	if err != nil {
		t.Fatalf("load policy: %v", err)
	}
	row := ReportItemRow{
		ItemID:       8,
		ReviewStatus: ReviewStatusUnreviewed,
		ReportItem: contract.ReportItem{
			Name:     "尿潜血",
			Value:    str("+"),
			RefRange: str("-"),
			Flag:     FlagUnknown,
			RawText:  "尿潜血  +  -",
		},
	}
	next, _ := ApplyReviewActionWithPolicy(row, ReviewAction{Value: str("-")}, policy)
	if next.Flag != FlagNotTested || next.FlagReason != nil {
		t.Fatalf("qualitative -/- = %q/%v, want not_tested/nil", next.Flag, next.FlagReason)
	}

	noRef := ReportItemRow{
		ItemID: 9,
		ReportItem: contract.ReportItem{
			Name:     "谷草/谷丙比值",
			Value:    str("1.35"),
			RefRange: nil,
			Flag:     FlagUnknown,
			RawText:  "谷草/谷丙比值  1.35  计算法",
		},
	}
	next, _ = ApplyReviewActionWithPolicy(noRef, ReviewAction{Value: str("1.35")}, policy)
	if next.Flag != FlagUnknown || next.FlagReason == nil || *next.FlagReason != FlagReasonNoReference {
		t.Fatalf("no-reference item = %q/%v, want unknown/no_reference", next.Flag, next.FlagReason)
	}
}

func TestApplyReviewActionCorrectToUnparsable(t *testing.T) {
	row := reviewRow()
	next, _ := ApplyReviewAction(row, ReviewAction{Value: str("查")})
	if next.Flag != FlagUnknown {
		t.Errorf("flag = %q, want unknown(不可数值化不猜)", next.Flag)
	}
}
