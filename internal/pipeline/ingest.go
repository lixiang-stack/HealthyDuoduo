// Package pipeline 是 hdd 的编排层(用例):一图进 → 三层留存(对象存储/OCR/报告)。
// 依赖方向:只依赖本包接口;识别服务 / PG / S3 由 cmd/hdd 注入适配器实现(整洁架构)。
package pipeline

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"healthyduoduo/internal/contract"
)

// Recognizer 识别服务边界(POST /report、/reparse,实施计划 4.3)。
type Recognizer interface {
	// Report 上传图像(+可选 date 补录)→ {ocr_result, report}。
	Report(ctx context.Context, image []byte, filename, date string) (*contract.OCRResult, *contract.Report, error)
	// Reparse 已存 OCR 结果(+可选 date 补录)→ report(规则迭代主路径)。
	Reparse(ctx context.Context, ocr *contract.OCRResult, date string) (*contract.Report, error)
}

// ObjectStore 对象存储边界(bucket raw,键=图像内容 sha256)。
type ObjectStore interface {
	Put(ctx context.Context, key string, data []byte) error
}

// Store 持久化边界(PG 四表);items 由实现随报告行事务式全量替换。
type Store interface {
	ImageBySha256(ctx context.Context, sha string) (*Image, error)
	CreateImage(ctx context.Context, sha, objectKey, filename string) (int64, error)
	AppendOCRResult(ctx context.Context, imageID int64, raw []byte) (ocrID int64, err error)
	ReportByImage(ctx context.Context, imageID int64) (*ReportRow, bool, error)
	ReportByID(ctx context.Context, id int64) (*ReportRow, error)
	ListReports(ctx context.Context, reportType, date string) ([]ReportRow, error)
	ItemsByReport(ctx context.Context, reportID int64) ([]contract.ReportItem, error)
	OCRResultJSON(ctx context.Context, ocrID int64) ([]byte, error)
	// WriteReport:reportID=0 新建(挂 imageID),>0 原位更新;items 取自 report.Items。
	WriteReport(ctx context.Context, reportID, imageID, ocrID int64, report contract.Report) (int64, error)
}

// Image images 表一行的业务视图。
type Image struct {
	ID               int64
	Sha256           string
	ObjectKey        string
	OriginalFilename string
}

// ReportRow 报告查询视图(report_date 为 YYYY-MM-DD,缺失时 nil)。
type ReportRow struct {
	ID          int64
	ImageID     int64
	OCRResultID int64
	ReportType  string
	ReportDate  *string
	Status      string // success | partial | failed
	ItemsCount  int64  // 仅 ListReports 填充
	Items       []contract.ReportItem
}

// IngestOutcome 入库结果(供 CLI 演示输出)。
type IngestOutcome struct {
	ReportID int64
	ImageID  int64
	Kind     // KindCached(幂等命中) / KindNew / KindForced / KindBackfilled(缓存报告 --date 补录)
	Report   *contract.Report
}

type Kind int

const (
	KindNew Kind = iota
	KindCached
	KindForced
	KindBackfilled
)

// Ingest ingest <image...> [--date YYYY-MM-DD] [--force]:
// 图像 sha256 已入库且未 --force → 幂等返回既有报告 ID(决策 #5);
// 否则重跑识别;--force 补录:重调 /report 并追加 ocr_results、原位更新 reports(决策 #5/#7)。
func Ingest(ctx context.Context, st Store, rec Recognizer, obj ObjectStore, path, date string, force bool) (*IngestOutcome, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("读取图像 %s: %w", path, err)
	}
	sum := sha256.Sum256(data)
	sha := hex.EncodeToString(sum[:])
	img, err := st.ImageBySha256(ctx, sha)
	if err != nil {
		return nil, fmt.Errorf("查询 images: %w", err)
	}

	filename := filepath.Base(path)
	var reportID int64
	if img != nil {
		cached, ok, err := st.ReportByImage(ctx, img.ID)
		if err != nil {
			return nil, fmt.Errorf("查询 reports: %w", err)
		}
		if ok && !force {
			if date == "" || cached.ReportDate != nil {
				return &IngestOutcome{ReportID: cached.ID, ImageID: cached.ImageID, Kind: KindCached}, nil
			}
			// 同图已入库但报告无日期,用户给出 --date:在已存 OCR 上重跑补录(决策 #6)
			newReport, err := reparseAt(ctx, st, rec, cached, date)
			if err != nil {
				return nil, err
			}
			return &IngestOutcome{ReportID: cached.ID, ImageID: cached.ImageID, Kind: KindBackfilled, Report: newReport}, nil
		}
		reportID = cached.ID // --force 时沿用既有报告行
	}

	ocr, report, err := rec.Report(ctx, data, filename, date)
	if err != nil {
		return nil, err
	}
	if img == nil {
		if err := obj.Put(ctx, sha, data); err != nil {
			return nil, fmt.Errorf("上传原图 SeaweedFS: %w", err)
		}
		if img, err = createImage(ctx, st, sha, data, filename); err != nil {
			return nil, err
		}
	}
	raw, err := json.Marshal(ocr)
	if err != nil {
		return nil, fmt.Errorf("序列化 ocr_result: %w", err)
	}
	ocrID, err := st.AppendOCRResult(ctx, img.ID, raw)
	if err != nil {
		return nil, fmt.Errorf("写入 ocr_results: %w", err)
	}
	id, err := st.WriteReport(ctx, reportID, img.ID, ocrID, *report)
	if err != nil {
		return nil, fmt.Errorf("写入 reports/report_items: %w", err)
	}
	kind := KindNew
	if reportID != 0 {
		kind = KindForced
	}
	return &IngestOutcome{ReportID: id, ImageID: img.ID, Kind: kind, Report: report}, nil
}

func createImage(ctx context.Context, st Store, sha string, data []byte, filename string) (*Image, error) {
	id, err := st.CreateImage(ctx, sha, objectKeyFor(sha), filename)
	if err != nil {
		return nil, fmt.Errorf("写入 images: %w", err)
	}
	return &Image{ID: id, Sha256: sha, OriginalFilename: filename}, nil
}

// objectKey 对象键 = 图像内容 sha256 实施计划 4.4。
func objectKeyFor(sha string) string { return sha }

// Reparse reparse <report-id...>:复用已存 OCR 结果重跑后处理(规则迭代主路径)。
func Reparse(ctx context.Context, st Store, rec Recognizer, reportID int64, date string) (*contract.Report, error) {
	row, err := st.ReportByID(ctx, reportID)
	if err != nil {
		return nil, err
	}
	return reparseAt(ctx, st, rec, row, date)
}

func reparseAt(ctx context.Context, st Store, rec Recognizer, row *ReportRow, date string) (*contract.Report, error) {
	raw, err := st.OCRResultJSON(ctx, row.OCRResultID)
	if err != nil {
		return nil, fmt.Errorf("读取 ocr_results(%d): %w", row.OCRResultID, err)
	}
	var ocr contract.OCRResult
	if err := json.Unmarshal(raw, &ocr); err != nil {
		return nil, fmt.Errorf("解析已存 ocr_result: %w", err)
	}
	report, err := rec.Reparse(ctx, &ocr, date)
	if err != nil {
		return nil, fmt.Errorf("调用 /reparse: %w", err)
	}
	if _, err := st.WriteReport(ctx, row.ID, row.ImageID, row.OCRResultID, *report); err != nil {
		return nil, fmt.Errorf("更新 reports/report_items: %w", err)
	}
	return report, nil
}

// ListReports 查询报告列表(可选类别/日期过滤)。
func ListReports(ctx context.Context, st Store, reportType, date string) ([]ReportRow, error) {
	return st.ListReports(ctx, reportType, date)
}

// ShowReport 查询单份报告(含指标项)。
func ShowReport(ctx context.Context, st Store, reportID int64) (*ReportRow, error) {
	row, err := st.ReportByID(ctx, reportID)
	if err != nil {
		return nil, err
	}
	items, err := st.ItemsByReport(ctx, reportID)
	if err != nil {
		return nil, err
	}
	row.Items = items
	return row, nil
}
