// ingest 编排:一图进 → 三层留存(对象存储/OCR/报告),决策 #5 幂等、#6 日期补录、#7 低置信透传。
// 身份 = 图像内容 sha256(见 types.go)。
package pipeline

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Ingest hdd ingest <image...> [--date YYYY-MM-DD] [--force]:
// 图像 sha256 已入库且未 --force → 幂等返回该图像身份(决策 #5);
// 该报告无日期且 --date 给出 → 在已存 OCR 上重跑补录(决策 #6);
// 否则重跑识别 /report;--force 重跑同样追加 ocr_results(决策 #7,历史保留)。
// 出参 Sha256 即本次操作的图像身份(reports 主键与其一致)。
func Ingest(ctx context.Context, st Store, rec Recognizer, obj ObjectStore, path, date string, force bool) (*IngestOutcome, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read image %s: %w", path, err)
	}
	return IngestImage(ctx, st, rec, obj, data, filepath.Base(path), date, force)
}

// ImageSha256Hex 图像内容身份(完整 64 位小写 hex sha256;全仓统一身份,见 types.go)。
func ImageSha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// IngestImage 以图像字节执行同一 ingest 编排(serve 上传原子接收后直接喂入);
// 身份、幂等、--force 语义与 Ingest 完全一致。
func IngestImage(ctx context.Context, st Store, rec Recognizer, obj ObjectStore, data []byte, filename, date string, force bool) (*IngestOutcome, error) {
	sha := ImageSha256Hex(data)
	img, err := st.ImageBySha256(ctx, sha)
	if err != nil {
		return nil, fmt.Errorf("query images: %w", err)
	}

	if img != nil {
		outcome, done, err := cachedOutcome(ctx, st, rec, sha, date, force)
		if err != nil {
			return nil, err
		}
		if done {
			return outcome, nil
		}
	}

	normalized, ocr, report, err := rec.Report(ctx, data, filename, date)
	if err != nil {
		return nil, err
	}
	// 整页横拍被识别端转正:转正后的图像才是入库/展示原图与内容身份。
	if len(normalized) > 0 && !bytes.Equal(normalized, data) {
		data = normalized
		sha = ImageSha256Hex(data)
		img, err = st.ImageBySha256(ctx, sha)
		if err != nil {
			return nil, fmt.Errorf("query images: %w", err)
		}
		if img != nil {
			outcome, done, err := cachedOutcome(ctx, st, rec, sha, date, force)
			if err != nil {
				return nil, err
			}
			if done {
				return outcome, nil
			}
		}
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
	if err := st.WriteReport(ctx, sha, ocrID, *report, applyFailInvariant(report)); err != nil {
		return nil, fmt.Errorf("write reports/report_items: %w", err)
	}
	kind := KindNew
	if img != nil {
		kind = KindForced
	}
	return &IngestOutcome{Sha256: sha, Kind: kind, Report: report}, nil
}

// cachedOutcome 已入库图像的报告快路径:命中且非 force 时返回缓存/日期补录结果;
// 未命中或 force 时 done=false,由调用方继续识别或覆盖。
func cachedOutcome(ctx context.Context, st Store, rec Recognizer, sha, date string, force bool) (*IngestOutcome, bool, error) {
	cached, ok, err := st.ReportByImage(ctx, sha)
	if err != nil {
		return nil, false, fmt.Errorf("query reports: %w", err)
	}
	if !ok || force {
		return nil, false, nil
	}
	if date == "" || cached.ReportDate != nil {
		return &IngestOutcome{Sha256: sha, Kind: KindCached}, true, nil
	}
	// 同图已入库但报告无日期,用户给出 --date:在已存 OCR 上重跑补录(决策 #6)
	newReport, err := ReparseStored(ctx, st, rec, cached, date)
	if err != nil {
		return nil, false, err
	}
	return &IngestOutcome{Sha256: sha, Kind: KindBackfilled, Report: newReport}, true, nil
}
