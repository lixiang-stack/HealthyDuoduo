// 核对端点(PATCH /api/v1/reports/{report_id}/items/{item_id};契约冻结于
// schemas/openapi.yaml,viz-and-client 票 09 实现)。家长逐项确信的落库动作:
// confirm = 认可解析结果(不产生修正对);correct = 修正 value/unit/name/ref_range
// 四项纸面事实(flag 由服务端按修正后的值 × 范围重算,无手改入口)。
// 响应即动作后的报告级派生状态(单响应刷新整屏,无「提交作业」步骤)。
package main

import (
	"errors"
	"net/http"
	"strconv"

	"healthyduoduo/internal/httpapi"
	"healthyduoduo/internal/pipeline"
)

// 核对动作枚举(契约 ReviewRequest 的 oneOf 两分支)。
const (
	reviewActionConfirm = "confirm"
	reviewActionCorrect = "correct"
)

// reviewBody 核对请求体解码形状:action 必带;四项纸面事实用 optField 区分
// 「未带」与「显式有值」(显式 null 一律拒绝,契约字段类型不允许 null;
// 与成员端点同一执行立场)。
type reviewBody struct {
	Action   string           `json:"action"`
	Value    optField[string] `json:"value"`
	Unit     optField[string] `json:"unit"`
	Name     optField[string] `json:"name"`
	RefRange optField[string] `json:"ref_range"`
}

// reviewAction 传输校验(契约 ReviewRequest 的两分支):confirm 不得携带修正字段;
// correct 至少一项且 name 非空(minLength: 1)。手改 flag 请求由
// DisallowUnknownFields 在解码层拒绝(contract review.request.flag.invalid.json 锚定)。
func (b reviewBody) reviewAction() (*pipeline.ReviewAction, error) {
	switch b.Action {
	case reviewActionConfirm:
		if b.Value.set || b.Unit.set || b.Name.set || b.RefRange.set {
			return nil, errors.New("confirm must not carry correction fields")
		}
		return &pipeline.ReviewAction{Confirm: true}, nil
	case reviewActionCorrect:
		if !b.Value.set && !b.Unit.set && !b.Name.set && !b.RefRange.set {
			return nil, errors.New("correct requires at least one of value, unit, name, ref_range")
		}
		if b.Name.set && b.Name.value == "" {
			return nil, errors.New("name must be a non-empty string")
		}
		action := &pipeline.ReviewAction{}
		if b.Value.set {
			action.Value = &b.Value.value
		}
		if b.Unit.set {
			action.Unit = &b.Unit.value
		}
		if b.Name.set {
			action.Name = &b.Name.value
		}
		if b.RefRange.set {
			action.RefRange = &b.RefRange.value
		}
		return action, nil
	default:
		return nil, errors.New(`action must be "confirm" or "correct"`)
	}
}

// handleReviewItem PATCH /api/v1/reports/{report_id}/items/{item_id}:核对(确认/修正)。
// report_id 为内容身份,非 64 hex 无法是任何已有报告 → 404(同详情端点口径);
// item_id 非整数 → 400(同成员端点口径);报告或指标项不存在 → 404(存储侧一次定位,
// 不预查报告:两者对客户端是同一件事)。
// 响应:动作后报告级状态 + 未核对低置信计数 + 本次指标项(含行链接,同详情投影)。
func handleReviewItem(d *deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		reportID := r.PathValue("report_id")
		if pipeline.ValidateSha256(reportID) != nil {
			writeErrorJSON(w, http.StatusNotFound, "report not found")
			return
		}
		itemID, err := strconv.ParseInt(r.PathValue("item_id"), 10, 64)
		if err != nil {
			writeErrorJSON(w, http.StatusBadRequest, "item_id must be an integer")
			return
		}
		var body reviewBody
		if err := decodeJSONBody(r, &body); err != nil {
			writeErrorJSON(w, http.StatusBadRequest, err.Error())
			return
		}
		action, err := body.reviewAction()
		if err != nil {
			writeErrorJSON(w, http.StatusBadRequest, err.Error())
			return
		}

		ctx := r.Context()
		outcome, err := d.reports.ReviewItem(ctx, reportID, itemID, *action)
		if err != nil {
			writeErrorJSON(w, http.StatusInternalServerError, "review item failed")
			return
		}
		if outcome == nil {
			writeErrorJSON(w, http.StatusNotFound, "item not found")
			return
		}
		ocr, err := loadReportOCR(ctx, d.reports, reportID)
		if err != nil {
			writeErrorJSON(w, http.StatusInternalServerError, "read report ocr result failed")
			return
		}
		writeJSON(w, http.StatusOK, httpapi.ReviewResult{
			ReportID:                     reportID,
			Status:                       outcome.Status,
			UnreviewedLowConfidenceCount: outcome.UnreviewedLowConfidenceCount,
			Item:                         toAPIItem(outcome.Item, ocr),
		})
	}
}
