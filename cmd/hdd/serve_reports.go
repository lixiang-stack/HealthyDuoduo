// 报告端点(GET /api/v1/reports 时间线、POST /api/v1/reports multipart 上传、
// GET /api/v1/reports/{report_id} 详情、PATCH 改归属、DELETE 硬删;契约冻结于
// schemas/openapi.yaml,viz-and-client 票 06/07/10)。
// 时间线按成员过滤(缺省全部,含存量无主);上传原子接收后同步等待识别,响应即完整报告。
// sha256 幂等分岔:已有报告不含核对决策 → 200 duplicate:true;已含核对决策 → 409,
// 显式 override=true 才重跑识别并重置归档。详情含指标项 OCR 行链接(票 07)。
// 自愈(票 10):改归属即刻生效、时间线随之移位;删除硬删报告与指标项与修正对,
// 原始图像与 OCR 识别历史留痕保留(同图可重新上传入档)。
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"

	"healthyduoduo/internal/contract"
	"healthyduoduo/internal/httpapi"
	"healthyduoduo/internal/pipeline"
)

// maxUploadBytes 单次上传请求体上限(一张检查单数字图像);防巨包耗尽服务内存/磁盘。
const maxUploadBytes = 25 << 20 // 25 MiB

// 上传表单字段名(契约 multipart properties 的端侧镜像;文件字段之外只此两项)。
const (
	formImage    = "image"
	formMemberID = "member_id"
	formOverride = "override"
)

// reviewDecisionConflictMessage 与契约 fixture report.conflict.json 锚定同一文案。
const reviewDecisionConflictMessage = "report has review decisions; pass override=true to rerun recognition"

// uploadRequest 解析后的 multipart 上传请求(原子接收:一次性读全图像字节,无分片)。
type uploadRequest struct {
	data     []byte
	filename string
	memberID int64
	override bool
}

// parseUploadRequest 严格解析 multipart 表单:image/member_id 必带、override 可选布尔、
// 未知字段拒绝(契约 additionalProperties:false 的端侧执行);超限/畸形一律报错(调用方 400)。
func parseUploadRequest(w http.ResponseWriter, r *http.Request) (*uploadRequest, error) {
	r.Body = http.MaxBytesReader(w, r.Body, maxUploadBytes)
	if err := r.ParseMultipartForm(maxUploadBytes); err != nil {
		return nil, fmt.Errorf("invalid multipart form: %w", err)
	}
	for field := range r.MultipartForm.Value {
		if field != formMemberID && field != formOverride {
			return nil, fmt.Errorf("unknown form field %q", field)
		}
	}
	for field := range r.MultipartForm.File {
		if field != formImage {
			return nil, fmt.Errorf("unknown form file %q", field)
		}
	}
	file, header, err := r.FormFile(formImage)
	if err != nil {
		return nil, errors.New("image is required")
	}
	defer file.Close()
	data, err := io.ReadAll(file)
	if err != nil {
		return nil, fmt.Errorf("read image: %w", err)
	}
	if len(data) == 0 {
		return nil, errors.New("image is empty")
	}

	// 只读 multipart body:r.FormValue 在 multipart 下 URL query 值优先(可覆盖 body 值、
	// 单独满足必填、空值遮蔽合法值——ParseMultipartForm 把 body 字段追加在 query 之后);
	// 契约的字段面就是表单 body,query 不参与取值。
	rawMember := r.PostFormValue(formMemberID)
	if rawMember == "" {
		return nil, errors.New("member_id is required")
	}
	memberID, err := strconv.ParseInt(rawMember, 10, 64)
	if err != nil {
		return nil, errors.New("member_id must be an integer")
	}
	override := false
	if raw := r.PostFormValue(formOverride); raw != "" { // 同 member_id:只读 body。
		override, err = strconv.ParseBool(raw)
		if err != nil {
			return nil, errors.New("override must be a boolean")
		}
	}
	filename := header.Filename
	if filename == "" {
		filename = "upload"
	}
	return &uploadRequest{data: data, filename: filename, memberID: memberID, override: override}, nil
}

// reportIDFromPath 取并校验路径报告身份(64 hex):非法形态无法是任何已有报告,
// 与不存在同答 404(已写响应,返回 false;详情/核对/改归属/删除共用该口径)。
func reportIDFromPath(w http.ResponseWriter, r *http.Request) (string, bool) {
	reportID := r.PathValue("report_id")
	if pipeline.ValidateSha256(reportID) != nil {
		writeErrorJSON(w, http.StatusNotFound, reportNotFoundMessage)
		return "", false
	}
	return reportID, true
}

// requireMember 校验归属成员存在(上传/改归属共用):未知成员 404、查询失败 500,
// 已写响应并返回 false。
func requireMember(w http.ResponseWriter, r *http.Request, ms pipeline.MemberStore, memberID int64) bool {
	member, err := ms.MemberByID(r.Context(), memberID)
	if err != nil {
		writeErrorJSON(w, http.StatusInternalServerError, "lookup member failed")
		return false
	}
	if member == nil {
		writeErrorJSON(w, http.StatusNotFound, memberNotFoundMessage)
		return false
	}
	return true
}

// handleListReports GET /api/v1/reports?member_id=:时间线卡片(成员过滤;缺省全部)。
// 排序口径服务端先给(日期降序、dateless 沉底),客户端展示会再排序一次。
func handleListReports(rs pipeline.ReportStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var memberID *int64
		if raw := r.URL.Query().Get("member_id"); raw != "" {
			id, err := strconv.ParseInt(raw, 10, 64)
			if err != nil {
				writeErrorJSON(w, http.StatusBadRequest, "member_id must be an integer")
				return
			}
			memberID = &id
		}
		summaries, err := rs.ListTimeline(r.Context(), memberID)
		if err != nil {
			writeErrorJSON(w, http.StatusInternalServerError, "list reports failed")
			return
		}
		list := httpapi.ReportsList{Reports: make([]httpapi.ReportSummary, 0, len(summaries))}
		for _, sum := range summaries {
			list.Reports = append(list.Reports, httpapi.ReportSummary{ReportHeader: toAPIReportHeader(sum)})
		}
		writeJSON(w, http.StatusOK, list)
	}
}

// handleGetReport GET /api/v1/reports/{report_id}:报告详情(头部 + 指标项含 OCR 行链接)。
// report_id 为对外身份 sha256;非该形态无法是任何已有报告,与不存在同样回 404(契约只声明 404)。
func handleGetReport(d *deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		reportID, ok := reportIDFromPath(w, r)
		if !ok {
			return
		}
		ctx := r.Context()
		summary, err := d.reports.SummaryByImage(ctx, reportID)
		if err != nil {
			writeErrorJSON(w, http.StatusInternalServerError, "lookup report failed")
			return
		}
		if summary == nil {
			writeErrorJSON(w, http.StatusNotFound, reportNotFoundMessage)
			return
		}
		items, err := d.reports.ItemsWithReviewStatus(ctx, reportID)
		if err != nil {
			writeErrorJSON(w, http.StatusInternalServerError, "read report items failed")
			return
		}
		sections, err := d.reports.SectionsByImage(ctx, reportID)
		if err != nil {
			writeErrorJSON(w, http.StatusInternalServerError, "read report sections failed")
			return
		}
		ocr, err := loadReportOCR(ctx, d.reports, reportID)
		if err != nil {
			writeErrorJSON(w, http.StatusInternalServerError, "read report ocr result failed")
			return
		}
		writeJSON(w, http.StatusOK, toAPIDetail(*summary, sections, items, ocr))
	}
}

// reassignBody PATCH /reports/{report_id} 请求体解码形状:member_id 必带且为整数
// (显式 null 一律拒绝,与成员端点同一执行立场)。
type reassignBody struct {
	MemberID optField[int64] `json:"member_id"`
}

// handleReassignReport PATCH /api/v1/reports/{report_id}:改归属(传错成员不必删档重传)。
// report_id 非 64 hex → 404(内容身份口径,同详情端点);未知成员/未知报告 → 404;
// 成功 200 返回刷新后的报告头部(归属即刻生效,时间线随成员过滤移位)。
func handleReassignReport(d *deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		reportID, ok := reportIDFromPath(w, r)
		if !ok {
			return
		}
		var body reassignBody
		if err := decodeJSONBody(r, &body); err != nil {
			writeErrorJSON(w, http.StatusBadRequest, err.Error())
			return
		}
		if !body.MemberID.set {
			writeErrorJSON(w, http.StatusBadRequest, "member_id is required")
			return
		}
		ctx := r.Context()
		if !requireMember(w, r, d.members, body.MemberID.value) {
			return
		}
		summary, err := d.reports.SummaryByImage(ctx, reportID)
		if err != nil {
			writeErrorJSON(w, http.StatusInternalServerError, "lookup report failed")
			return
		}
		if summary == nil {
			writeErrorJSON(w, http.StatusNotFound, reportNotFoundMessage)
			return
		}
		updated, err := d.reports.SetReportMember(ctx, reportID, body.MemberID.value)
		if err != nil {
			writeErrorJSON(w, http.StatusInternalServerError, "assign member failed")
			return
		}
		if !updated {
			// 竞态窗口:摘要读出后报告被并发删除——按路径身份口径回 404。
			writeErrorJSON(w, http.StatusNotFound, reportNotFoundMessage)
			return
		}
		memberID := body.MemberID.value
		summary.MemberID = &memberID
		writeJSON(w, http.StatusOK, httpapi.ReportSummary{ReportHeader: toAPIReportHeader(*summary)})
	}
}

// handleDeleteReport DELETE /api/v1/reports/{report_id}:硬删报告(级联指标项与修正对;
// 无回收站)。原始图像与 OCR 识别历史留痕保留,同图日后可重新上传入档(内容哈希不再命中)。
// report_id 非 64 hex → 404;报告不存在 → 404;成功 204 无响应体。
func handleDeleteReport(d *deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		reportID, ok := reportIDFromPath(w, r)
		if !ok {
			return
		}
		deleted, err := d.reports.DeleteReport(r.Context(), reportID)
		if err != nil {
			writeErrorJSON(w, http.StatusInternalServerError, "delete report failed")
			return
		}
		if !deleted {
			writeErrorJSON(w, http.StatusNotFound, reportNotFoundMessage)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

// handleUploadReport POST /api/v1/reports:multipart 同步上传。
// 200/201 分岔与归属写入语义见包注释;所有错误走契约 Error 形状。
func handleUploadReport(d *deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		req, err := parseUploadRequest(w, r)
		if err != nil {
			writeErrorJSON(w, http.StatusBadRequest, err.Error())
			return
		}
		ctx := r.Context()
		if !requireMember(w, r, d.members, req.memberID) {
			return
		}

		sha := pipeline.ImageSha256Hex(req.data)
		existing, err := d.reports.SummaryByImage(ctx, sha)
		if err != nil {
			writeErrorJSON(w, http.StatusInternalServerError, "lookup report failed")
			return
		}

		// sha 命中且未显式覆盖:已有报告不含核对决策 → 200 duplicate;含 → 409。
		if existing != nil && !req.override {
			items, err := d.reports.ItemsWithReviewStatus(ctx, sha)
			if err != nil {
				writeErrorJSON(w, http.StatusInternalServerError, "read report items failed")
				return
			}
			if hasReviewDecisions(items) {
				writeErrorJSON(w, http.StatusConflict, reviewDecisionConflictMessage)
				return
			}
			if err := assignIfUnowned(ctx, d.reports, existing, req.memberID); err != nil {
				writeErrorJSON(w, http.StatusInternalServerError, "assign member failed")
				return
			}
			sections, err := d.reports.SectionsByImage(ctx, existing.Sha256)
			if err != nil {
				writeErrorJSON(w, http.StatusInternalServerError, "read report sections failed")
				return
			}
			ocr, err := loadReportOCR(ctx, d.reports, existing.Sha256)
			if err != nil {
				writeErrorJSON(w, http.StatusInternalServerError, "read report ocr result failed")
				return
			}
			writeJSON(w, http.StatusOK, httpapi.UploadResult{Duplicate: true, Report: toAPIDetail(*existing, sections, items, ocr)})
			return
		}

		// 新建归档或显式 override 重跑(override 会重置报告与指标项:item_id 全换、
		// review_status 回 unreviewed;已有归属不被夺走——改归属走 PATCH 端点)。
		if _, err := pipeline.IngestImage(ctx, d.store, d.rec, d.obj, req.data, req.filename, "", req.override); err != nil {
			writeErrorJSON(w, http.StatusInternalServerError, "upload failed: "+err.Error())
			return
		}
		summary, err := d.reports.SummaryByImage(ctx, sha)
		if err != nil {
			writeErrorJSON(w, http.StatusInternalServerError, "read report failed")
			return
		}
		if summary == nil {
			// 不可能:IngestImage 刚写入报告行;防御性兜底不走静默错误。
			writeErrorJSON(w, http.StatusInternalServerError, "report missing after ingest")
			return
		}
		if err := assignIfUnowned(ctx, d.reports, summary, req.memberID); err != nil {
			writeErrorJSON(w, http.StatusInternalServerError, "assign member failed")
			return
		}
		items, err := d.reports.ItemsWithReviewStatus(ctx, sha)
		if err != nil {
			writeErrorJSON(w, http.StatusInternalServerError, "read report items failed")
			return
		}
		sections, err := d.reports.SectionsByImage(ctx, sha)
		if err != nil {
			writeErrorJSON(w, http.StatusInternalServerError, "read report sections failed")
			return
		}
		ocr, err := loadReportOCR(ctx, d.reports, sha)
		if err != nil {
			writeErrorJSON(w, http.StatusInternalServerError, "read report ocr result failed")
			return
		}
		writeJSON(w, http.StatusCreated, httpapi.UploadResult{Duplicate: false, Report: toAPIDetail(*summary, sections, items, ocr)})
	}
}

// assignIfUnowned 归属到本次上传的成员:仅当报告无主时写入(set 后回填视图);
// 已有归属(from 同图重传/override 重跑的其他成员)不静默夺走,改归属走 PATCH 端点。
// 写入路径回报告缺失(并发删除竞态)时报错——上传路径刚写入的行缺失属异常,不静默。
func assignIfUnowned(ctx context.Context, rs pipeline.ReportStore, sum *pipeline.ReportSummary, memberID int64) error {
	if sum.MemberID != nil {
		return nil
	}
	updated, err := rs.SetReportMember(ctx, sum.Sha256, memberID)
	if err != nil {
		return err
	}
	if !updated {
		return fmt.Errorf("report %s not found", sum.Sha256)
	}
	sum.MemberID = &memberID
	return nil
}

// hasReviewDecisions 任一指标项 review_status ≠ unreviewed(已确认/已修正)即视为
// 已有核对决策:同图重传默认 409,不悄悄覆盖/重跑家长已确信的数字。
func hasReviewDecisions(items []pipeline.ReportItemRow) bool {
	for _, it := range items {
		if it.ReviewStatus != pipeline.ReviewStatusUnreviewed {
			return true
		}
	}
	return false
}

// toAPIReportHeader 业务摘要 → 契约报告头部(时间线卡片与上传响应同一投影)。
func toAPIReportHeader(sum pipeline.ReportSummary) httpapi.ReportHeader {
	return httpapi.ReportHeader{
		ReportID:   sum.Sha256,
		MemberID:   sum.MemberID,
		ReportType: sum.ReportType,
		ReportDate: sum.ReportDate,
		Status:     sum.Status,
		FailReason: sum.FailReason,
		FlagCounts: httpapi.FlagCounts{
			High:    sum.FlagCounts.High,
			Low:     sum.FlagCounts.Low,
			Unknown: sum.FlagCounts.Unknown,
		},
		UnreviewedLowConfidenceCount: sum.UnreviewedLowConfidenceCount,
	}
}

// loadReportOCR 读报告所用 OCR 结果并解析为契约形状(详情行链接映射的行框来源,票 07)。
// 报告行在则 OCR 行必在(ingest 原子写入);缺失/损坏按错误暴露,不静默降级为空链接。
func loadReportOCR(ctx context.Context, rs pipeline.ReportStore, reportSha string) (*contract.OCRResult, error) {
	raw, err := rs.OCRResultJSONByReport(ctx, reportSha)
	if err != nil {
		return nil, err
	}
	if raw == nil {
		return nil, fmt.Errorf("ocr result missing for report %s", reportSha)
	}
	var ocr contract.OCRResult
	if err := json.Unmarshal(raw, &ocr); err != nil {
		return nil, fmt.Errorf("parse stored ocr result: %w", err)
	}
	return &ocr, nil
}

// toAPILineBox 识别端 Box([4]Point) → 契约镜像 Box4x2([4][2]float64);
// 元素为具名数组类型,直接类型转换不成立,逐点半拷贝。
func toAPILineBox(b contract.Box) httpapi.Box4x2 {
	var out httpapi.Box4x2
	for i, p := range b {
		out[i] = [2]float64{p[0], p[1]}
	}
	return out
}

// toAPILineLinks 行号 → 行链接(items 与报告段落共用同一映射口径;越界仅在
// 数据损坏时出现,跳过不炸整页)。非 nil 空数组:items.ocr_lines 契约要求数组。
func toAPILineLinks(lineNumbers []int32, ocr *contract.OCRResult) []httpapi.OcrLineLink {
	links := make([]httpapi.OcrLineLink, 0, len(lineNumbers))
	for _, ln := range lineNumbers {
		if ln < 0 || int(ln) >= len(ocr.Boxes) {
			continue
		}
		links = append(links, httpapi.OcrLineLink{Line: int(ln), Box: toAPILineBox(ocr.Boxes[ln])})
	}
	return links
}

// toAPIItem 单个指标项投影(详情与核对响应共用):行链接由识别端直吐的
// ocr_line_numbers 精确映射(下标 → 行框);越界仅在数据损坏时出现——跳过不炸
// 整页(raw_text 仍可见),结构完整性由 e2e verify 的 join/box 门禁兜底;
// prev 属 M3(契约形状先以 null 满足)。
func toAPIItem(it pipeline.ReportItemRow, ocr *contract.OCRResult) httpapi.ReportItem {
	return httpapi.ReportItem{
		ItemID:        it.ItemID,
		Name:          it.Name,
		Value:         it.Value,
		Unit:          it.Unit,
		RefRange:      it.RefRange,
		Method:        it.Method,
		Flag:          it.Flag,
		LowConfidence: it.LowConfidence,
		RawText:       it.RawText,
		ReviewStatus:  it.ReviewStatus,
		OcrLines:      toAPILineLinks(it.OCRLineNumbers, ocr),
		Prev:          nil,
	}
}

// toAPISection 报告级段落投影:文本原样 + 行号 → 行链接(与 items 同源同构)。
func toAPISection(s contract.ReportSection, ocr *contract.OCRResult) httpapi.ReportSection {
	return httpapi.ReportSection{
		Kind:     s.Kind,
		Text:     s.Text,
		OcrLines: toAPILineLinks(s.OCRLineNumbers, ocr),
	}
}

// toAPIDetail 报告详情投影(GET 详情与上传响应共用):头部 + 可选报告段落 + 指标项。
// sections 为空时省略该键(非叙述型报告响应与既有契约逐字节一致)。
func toAPIDetail(sum pipeline.ReportSummary, sections []contract.ReportSection, items []pipeline.ReportItemRow, ocr *contract.OCRResult) httpapi.ReportDetail {
	detail := httpapi.ReportDetail{ReportHeader: toAPIReportHeader(sum), Items: make([]httpapi.ReportItem, 0, len(items))}
	for _, s := range sections {
		detail.Sections = append(detail.Sections, toAPISection(s, ocr))
	}
	for _, it := range items {
		detail.Items = append(detail.Items, toAPIItem(it, ocr))
	}
	return detail
}
