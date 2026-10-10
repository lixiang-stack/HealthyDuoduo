// ingest 编排:一图进 → 三层留存(对象存储/OCR/报告),决策 #5 幂等、#6 日期补录、#7 低置信透传。
// 身份 = 图像内容 sha256(见 types.go)。
package pipeline

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Ingest hdd ingest <image...> [--date YYYY-MM-DD] [--force] [--preprocess <steps>]:
// 图像 sha256 已入库且未 --force → 幂等返回该图像身份(决策 #5);
// 该报告无日期且 --date 给出 → 在已存 OCR 上重跑补录(决策 #6);
// 否则重跑识别 /report(preprocess 为 P3 预处理开关透传);--force 重跑同样
// 追加 ocr_results(决策 #7,历史保留)。
// 出参 Sha256 即本次操作的图像身份(reports 主键与其一致)。
func Ingest(ctx context.Context, st Store, rec Recognizer, obj ObjectStore, path, date, preprocess string, force bool) (*IngestOutcome, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read image %s: %w", path, err)
	}
	sum := sha256.Sum256(data)
	sha := hex.EncodeToString(sum[:])
	img, err := st.ImageBySha256(ctx, sha)
	if err != nil {
		return nil, fmt.Errorf("query images: %w", err)
	}

	filename := filepath.Base(path)
	if img != nil {
		cached, ok, err := st.ReportByImage(ctx, sha)
		if err != nil {
			return nil, fmt.Errorf("query reports: %w", err)
		}
		if ok && !force {
			if date == "" || cached.ReportDate != nil {
				return &IngestOutcome{Sha256: sha, Kind: KindCached}, nil
			}
			// 同图已入库但报告无日期,用户给出 --date:在已存 OCR 上重跑补录(决策 #6)
			newReport, err := ReparseStored(ctx, st, rec, cached, date)
			if err != nil {
				return nil, err
			}
			return &IngestOutcome{Sha256: sha, Kind: KindBackfilled, Report: newReport}, nil
		}
	}

	ocr, report, err := rec.Report(ctx, data, filename, date, preprocess)
	if err != nil {
		return nil, err
	}
	if img == nil {
		if err := obj.Put(ctx, sha, data); err != nil {
			return nil, fmt.Errorf("upload image to object store: %w", err)
		}
		if err := st.CreateImage(ctx, sha, filename); err != nil {
			return nil, fmt.Errorf("write images: %w", err)
		}
	}
	raw, err := json.Marshal(ocr)
	if err != nil {
		return nil, fmt.Errorf("marshal ocr_result: %w", err)
	}
	ocrID, err := st.AppendOCRResult(ctx, sha, raw)
	if err != nil {
		return nil, fmt.Errorf("write ocr_results: %w", err)
	}
	if err := st.WriteReport(ctx, sha, ocrID, *report); err != nil {
		return nil, fmt.Errorf("write reports/report_items: %w", err)
	}
	kind := KindNew
	if img != nil {
		kind = KindForced
	}
	return &IngestOutcome{Sha256: sha, Kind: kind, Report: report}, nil
}
