// 报告端点(GET/POST /api/v1/reports;契约冻结于 schemas/openapi.yaml,viz-and-client 票 06)。
// GET 时间线按成员过滤(缺省全部,含存量无主);POST multipart 上传(图像 + 成员),
// 原子接收后同步等待识别,响应即完整报告。sha256 幂等分岔:已有报告不含核对决策 →
// 200 duplicate:true;已含核对决策 → 409,显式 override=true 才重跑识别并重置归档。
package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"

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
		member, err := d.members.MemberByID(ctx, req.memberID)
		if err != nil {
			writeErrorJSON(w, http.StatusInternalServerError, "lookup member failed")
			return
		}
		if member == nil {
			writeErrorJSON(w, http.StatusNotFound, "member not found")
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
			writeJSON(w, http.StatusOK, httpapi.UploadResult{Duplicate: true, Report: toAPIDetail(*existing, items)})
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
		writeJSON(w, http.StatusCreated, httpapi.UploadResult{Duplicate: false, Report: toAPIDetail(*summary, items)})
	}
}

// assignIfUnowned 归属到本次上传的成员:仅当报告无主时写入(set 后回填视图);
// 已有归属(from 同图重传/override 重跑的其他成员)不静默夺走,改归属走 PATCH 端点。
func assignIfUnowned(ctx context.Context, rs pipeline.ReportStore, sum *pipeline.ReportSummary, memberID int64) error {
	if sum.MemberID != nil {
		return nil
	}
	if err := rs.SetReportMember(ctx, sum.Sha256, memberID); err != nil {
		return err
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

// toAPIDetail 上传响应的完整报告:头部 + 指标项。ocr_lines 与 prev 属票 07/M3
// 填充面(契约形状先以空数组/null 满足,不冒充已有数据)。
func toAPIDetail(sum pipeline.ReportSummary, items []pipeline.ReportItemRow) httpapi.ReportDetail {
	detail := httpapi.ReportDetail{ReportHeader: toAPIReportHeader(sum), Items: make([]httpapi.ReportItem, 0, len(items))}
	for _, it := range items {
		detail.Items = append(detail.Items, httpapi.ReportItem{
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
			OcrLines:      []httpapi.OcrLineLink{},
			Prev:          nil,
		})
	}
	return detail
}
