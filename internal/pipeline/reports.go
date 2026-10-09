// 查询与重算:show / list / reparse(规则迭代主路径,复用已存 OCR 结果,NF-05)。
// 入参 identity 仅接受完整 64 位小写 hex sha256(不支持前缀)。
package pipeline

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"

	"healthyduoduo/internal/contract"
)

// sha256Re 完整 sha256 的形态(唯一身份形态;不支持前缀)。
var sha256Re = regexp.MustCompile(`^[0-9a-f]{64}$`)

// ValidateSha256 校验 CLI 入参身份形态(完整 64 位小写 hex)。
func ValidateSha256(sha string) error {
	if !sha256Re.MatchString(sha) {
		return fmt.Errorf("expect a full 64-char lowercase sha256, got %q", sha)
	}
	return nil
}

// Reparse reparse <sha256...>:复用已存 OCR 结果重跑后处理(规则迭代后历史刷新)。
func Reparse(ctx context.Context, st Store, rec Recognizer, shaArg, date string) (*contract.Report, error) {
	if err := ValidateSha256(shaArg); err != nil {
		return nil, err
	}
	row, ok, err := st.ReportByImage(ctx, shaArg)
	if err != nil {
		return nil, fmt.Errorf("query reports: %w", err)
	}
	if !ok {
		return nil, fmt.Errorf("no report for image %s; ingest first", shaArg)
	}
	return ReparseStored(ctx, st, rec, row, date)
}

// ReparseStored 在指定报告行上执行 reparse;供 Ingest 的 --date 补录路径复用。
// 既有 reports.report_date 是用户补录的元数据:本次未显式给 --date 时,
// 以已存日期作为兜底传入识别服务,避免规则重算把它抹掉;OCR 自己解析出日期时仍优先。
func ReparseStored(ctx context.Context, st Store, rec Recognizer, row *ReportRow, date string) (*contract.Report, error) {
	if date == "" && row.ReportDate != nil {
		date = *row.ReportDate
	}
	raw, err := st.OCRResultJSON(ctx, row.OCRResultID)
	if err != nil {
		return nil, fmt.Errorf("read stored ocr_result(%d): %w", row.OCRResultID, err)
	}
	var ocr contract.OCRResult
	if err := json.Unmarshal(raw, &ocr); err != nil {
		return nil, fmt.Errorf("parse stored ocr_result: %w", err)
	}
	report, err := rec.Reparse(ctx, &ocr, date)
	if err != nil {
		return nil, fmt.Errorf("call /reparse: %w", err)
	}
	if err := st.WriteReport(ctx, row.Sha256, row.OCRResultID, *report); err != nil {
		return nil, fmt.Errorf("update reports/report_items: %w", err)
	}
	return report, nil
}

// ListReports 查询报告列表(可选类别/日期;日期支持 YYYY / YYYY-MM / YYYY-MM-DD)。
func ListReports(ctx context.Context, st Store, reportType, date string) ([]ReportRow, error) {
	return st.ListReports(ctx, reportType, date)
}

// ShowReport 查询单份报告(含指标项)。
func ShowReport(ctx context.Context, st Store, shaArg string) (*ReportRow, error) {
	if err := ValidateSha256(shaArg); err != nil {
		return nil, err
	}
	row, ok, err := st.ReportByImage(ctx, shaArg)
	if err != nil {
		return nil, fmt.Errorf("query reports: %w", err)
	}
	if !ok {
		return nil, fmt.Errorf("no report for image %s; ingest first", shaArg)
	}
	items, err := st.ItemsByImage(ctx, shaArg)
	if err != nil {
		return nil, err
	}
	row.Items = items
	return row, nil
}
