// 多图上传批次(交互单位,不是领域实体):每张图仍独立上传、独立归档为一份报告。
// 串行执行 + 单张失败隔离;上限与追加截断在这里收口,页面只消费结果。
import type { UploadResult } from '../api/reports';
import type { PickedImage } from '../platform/imagePicker';
import { needsReview } from '../review/items';

/** 一次上传动作允许的图片上限(相册多选 + 拍照追加累计)。 */
export const MAX_UPLOAD_IMAGES = 9;

export type UploadOutcome =
  | { kind: 'done'; result: UploadResult }
  | { kind: 'failed'; message: string };

/** 追加选择并截到上限;discarded 供页面提示被忽略的张数。 */
export function appendPickedImages(
  current: PickedImage[],
  picked: PickedImage[],
): { images: PickedImage[]; discarded: number } {
  const room = Math.max(0, MAX_UPLOAD_IMAGES - current.length);
  const accepted = picked.slice(0, room);
  return { images: [...current, ...accepted], discarded: picked.length - accepted.length };
}

/** 按给定下标串行上传;单张失败只记录该张,继续后面的图。 */
export async function uploadImagesSerially(
  images: readonly PickedImage[],
  indexes: readonly number[],
  upload: (image: PickedImage) => Promise<UploadResult>,
  hooks: {
    onStart?: (index: number) => void;
    onSettled?: (index: number, outcome: UploadOutcome) => void;
  } = {},
): Promise<void> {
  for (const index of indexes) {
    hooks.onStart?.(index);
    try {
      const result = await upload(images[index]);
      hooks.onSettled?.(index, { kind: 'done', result });
    } catch (error) {
      hooks.onSettled?.(index, {
        kind: 'failed',
        message: error instanceof Error ? error.message : String(error),
      });
    }
  }
}

export type UploadBatchCounts = {
  archived: number;
  review: number;
  duplicate: number;
  failed: number;
};

/** 行/汇总的唯一分类口径:传输失败 → 重复 → 识别失败 → 待核对 → 已归档。 */
export function uploadBatchCounts(outcomes: readonly UploadOutcome[]): UploadBatchCounts {
  const counts: UploadBatchCounts = { archived: 0, review: 0, duplicate: 0, failed: 0 };
  for (const outcome of outcomes) {
    if (outcome.kind === 'failed') {
      counts.failed += 1;
      continue;
    }
    const { result } = outcome;
    if (result.duplicate) {
      counts.duplicate += 1;
    } else if (result.report.status === 'failed') {
      counts.failed += 1;
    } else if (needsReview(result.report)) {
      counts.review += 1;
    } else {
      counts.archived += 1;
    }
  }
  return counts;
}

/** 单行状态文案;与汇总共用同一分类口径。 */
export function uploadOutcomeLabel(outcome: UploadOutcome): string {
  if (outcome.kind === 'failed') {
    return `上传失败:${outcome.message}`;
  }
  const { result } = outcome;
  if (result.duplicate) {
    return '此前已归档';
  }
  if (result.report.status === 'failed') {
    return `识别失败:${result.report.fail_reason ?? '未识别出指标项'}`;
  }
  return needsReview(result.report) ? '待核对' : '已归档';
}

/** 多张汇总文案;单张仍由 status.ts 的 uploadResultText 提供原文案。 */
export function uploadBatchSummaryText(outcomes: readonly UploadOutcome[]): string {
  const counts = uploadBatchCounts(outcomes);
  const parts = [
    counts.archived > 0 ? `${counts.archived} 张已归档` : '',
    counts.review > 0 ? `${counts.review} 张待核对` : '',
    counts.duplicate > 0 ? `${counts.duplicate} 张此前已归档` : '',
    counts.failed > 0 ? `${counts.failed} 张失败` : '',
  ].filter((part) => part !== '');
  return `共 ${outcomes.length} 张:${parts.join(',')}`;
}
