// 核对动作(确认/修正)的领域逻辑(ADR-0005;viz-and-client 票 09)。
// 状态机:未核对 --确认|修正--> 已确认|已修正;任何状态可再次确认或修正
// (决策可重复、无「撤销回未核对」);确认不产生修正对,修正 append 一条。
package pipeline

// 指标项核对状态(初始态 ReviewStatusUnreviewed 见 types.go)。
const (
	ReviewStatusConfirmed = "confirmed"
	ReviewStatusCorrected = "corrected"
)

// ReviewAction 核对动作:确认(Confirm=true)或修正(至少一项纸面事实非 nil)。
// flag 不在字段集内——一律由 ComputeFlag 按修正后的 value × ref_range 重算。
type ReviewAction struct {
	Confirm  bool
	Value    *string
	Unit     *string
	Name     *string
	RefRange *string
}

// CorrectionPairRecord 一条 append-only 修正对(落库形状,ADR-0005):
// before = 上次生效值、after = 本次生效值(四项纸面事实全集);原始行文与 OCR
// 行号随指标项原样携带——修正不改行文归属。仅修正产生,确认不产生。
type CorrectionPairRecord struct {
	BeforeName     *string
	BeforeValue    *string
	BeforeUnit     *string
	BeforeRefRange *string
	AfterName      *string
	AfterValue     *string
	AfterUnit      *string
	AfterRefRange  *string
	RawText        string
	OCRLineNumbers []int32
}

// ReviewOutcome 核对落地后的报告级派生 + 本次指标项(单响应刷新整屏的载荷)。
type ReviewOutcome struct {
	Item                         ReportItemRow
	Status                       string
	UnreviewedLowConfidenceCount int
}

// ApplyReviewAction 在 current 上应用核对动作,返回更新后的指标项与修正对
// (确认返回 nil 对;纯函数,存储事务与测试 fake 共用)。修正覆盖传入的纸面事实、
// 按修正后的 value × ref_range 重算 flag、置 corrected。每次修正产出恰一条
// (重放同一修正为 no-op 对,最终生效状态不变——ADR-0005 决策可重复、重试安全)。
func ApplyReviewAction(current ReportItemRow, action ReviewAction) (ReportItemRow, *CorrectionPairRecord) {
	next := current
	if action.Confirm {
		next.ReviewStatus = ReviewStatusConfirmed
		return next, nil
	}
	next.ReviewStatus = ReviewStatusCorrected
	if action.Value != nil {
		next.Value = action.Value
	}
	if action.Unit != nil {
		next.Unit = action.Unit
	}
	if action.Name != nil {
		next.Name = *action.Name
	}
	if action.RefRange != nil {
		next.RefRange = action.RefRange
	}
	next.Flag = ComputeFlag(next.Value, next.RefRange)
	beforeName := current.Name
	pair := &CorrectionPairRecord{
		BeforeName:     &beforeName,
		BeforeValue:    current.Value,
		BeforeUnit:     current.Unit,
		BeforeRefRange: current.RefRange,
		AfterName:      &next.Name,
		AfterValue:     next.Value,
		AfterUnit:      next.Unit,
		AfterRefRange:  next.RefRange,
		RawText:        current.RawText,
		OCRLineNumbers: current.OCRLineNumbers,
	}
	return next, pair
}
