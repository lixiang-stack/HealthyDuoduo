import type { ReportDetail, ReportItem, UploadResult } from '../api/reports';
import type { PickedImage } from '../platform/imagePicker';

import {
  MAX_UPLOAD_IMAGES,
  appendPickedImages,
  uploadBatchCounts,
  uploadBatchSummaryText,
  uploadImagesSerially,
  uploadOutcomeLabel,
  type UploadOutcome,
} from './uploadBatch';

const image = (name: string): PickedImage => ({ uri: `file:///tmp/${name}`, fileName: name });

// 兼容 #37 的 not_tested_count：本分支当前 schema 还没有该字段，用 spread 携带后
// 两个 PR 可独立合并；#37 合入后类型自然对齐。
const notTestedCount = { not_tested_count: 0 };

const report = (overrides: Partial<ReportDetail> = {}): ReportDetail => ({
  report_id: 'a'.repeat(64),
  member_id: 1,
  report_type: '临检',
  report_date: '2026-10-01',
  status: 'success',
  fail_reason: null,
  flag_counts: { high: 0, low: 0, abnormal: 0, unknown: 0 },
  ...notTestedCount,
  unreviewed_low_confidence_count: 0,
  items: [],
  ...overrides,
});

const lowConfidenceItem: ReportItem = {
  item_id: 1,
  name: '白细胞计数',
  value: '6.1',
  unit: '10^9/L',
  ref_range: '3.5-9.5',
  method: null,
  flag: 'normal',
  low_confidence: true,
  raw_text: '白细胞计数 6.1',
  review_status: 'unreviewed',
  ocr_lines: [],
  prev: null,
};

const done = (detail: ReportDetail, duplicate = false): UploadOutcome => ({
  kind: 'done',
  result: { duplicate, report: detail } satisfies UploadResult,
});

describe('appendPickedImages', () => {
  it('未达上限:全部追加', () => {
    const result = appendPickedImages([image('a')], [image('b'), image('c')]);
    expect(result.images.map((item) => item.fileName)).toEqual(['a', 'b', 'c']);
    expect(result.discarded).toBe(0);
  });

  it('达到上限:只保留剩余张数,并报告丢弃数', () => {
    const current = Array.from({ length: MAX_UPLOAD_IMAGES - 1 }, (_, index) => image(`old-${index}`));
    const result = appendPickedImages(current, [image('new-1'), image('new-2'), image('new-3')]);
    expect(result.images).toHaveLength(MAX_UPLOAD_IMAGES);
    expect(result.images.at(-1)?.fileName).toBe('new-1');
    expect(result.discarded).toBe(2);
  });

  it('已满:一张也不追加', () => {
    const current = Array.from({ length: MAX_UPLOAD_IMAGES }, (_, index) => image(`old-${index}`));
    const result = appendPickedImages(current, [image('new')]);
    expect(result.images).toHaveLength(MAX_UPLOAD_IMAGES);
    expect(result.discarded).toBe(1);
  });
});

describe('uploadImagesSerially', () => {
  it('严格串行:上一张结束后才启动下一张,并按序回调', async () => {
    let releaseFirst: () => void = () => {};
    const firstGate = new Promise<void>((resolve) => {
      releaseFirst = resolve;
    });
    const calls: string[] = [];
    const events: string[] = [];
    const upload = jest.fn(async (picked: PickedImage) => {
      calls.push(picked.fileName!);
      if (picked.fileName === 'a') {
        await firstGate;
      }
      return { duplicate: false, report: report() };
    });

    const run = uploadImagesSerially([image('a'), image('b')], [0, 1], upload, {
      onStart: (index) => events.push(`start:${index}`),
      onSettled: (index) => events.push(`settled:${index}`),
    });

    await Promise.resolve();
    expect(calls).toEqual(['a']);
    expect(events).toEqual(['start:0']);

    releaseFirst();
    await run;
    expect(calls).toEqual(['a', 'b']);
    expect(events).toEqual(['start:0', 'settled:0', 'start:1', 'settled:1']);
  });

  it('单张失败不中断后续,失败原因只落在该张', async () => {
    const calls: string[] = [];
    const settled = new Map<number, UploadOutcome>();

    await uploadImagesSerially(
      [image('a'), image('b'), image('c')],
      [0, 1, 2],
      async (picked) => {
        calls.push(picked.fileName!);
        if (picked.fileName === 'b') {
          throw new Error('boom');
        }
        return { duplicate: false, report: report() };
      },
      { onSettled: (index, outcome) => settled.set(index, outcome) },
    );

    expect(calls).toEqual(['a', 'b', 'c']);
    expect(settled.get(0)?.kind).toBe('done');
    expect(settled.get(1)).toEqual({ kind: 'failed', message: 'boom' });
    expect(settled.get(2)?.kind).toBe('done');
  });

  it('只上传给定下标(重试失败项)', async () => {
    const calls: string[] = [];
    const upload = jest.fn(async (picked: PickedImage) => {
      calls.push(picked.fileName!);
      return { duplicate: false, report: report() };
    });
    await uploadImagesSerially([image('a'), image('b'), image('c')], [2, 0], upload);
    expect(calls).toEqual(['c', 'a']);
  });
});

describe('uploadBatchCounts', () => {
  it('按 已归档 / 待核对 / 此前已归档 / 失败 分类,失败含传输失败与识别失败', () => {
    const outcomes: UploadOutcome[] = [
      done(report()),
      done(report({ status: 'partial' })),
      done(report(), true),
      done(report({ status: 'failed', fail_reason: 'no metric items recognized' })),
      { kind: 'failed', message: 'timeout' },
    ];
    expect(uploadBatchCounts(outcomes)).toEqual({
      archived: 1,
      review: 1,
      duplicate: 1,
      failed: 2,
    });
  });

  it('success 但有未核对低置信项也算待核对', () => {
    expect(uploadBatchCounts([done(report({ items: [lowConfidenceItem] }))])).toEqual({
      archived: 0,
      review: 1,
      duplicate: 0,
      failed: 0,
    });
  });

  it('duplicate 优先于报告状态:重复的 partial 报告只计此前已归档', () => {
    expect(uploadBatchCounts([done(report({ status: 'partial' }), true)])).toEqual({
      archived: 0,
      review: 0,
      duplicate: 1,
      failed: 0,
    });
  });
});

describe('uploadBatchSummaryText', () => {
  it('多张汇总按固定顺序拼接,不出现 0 项', () => {
    const outcomes: UploadOutcome[] = [
      done(report()),
      done(report({ status: 'partial' })),
      done(report(), true),
      done(report({ status: 'failed' })),
      { kind: 'failed', message: 'timeout' },
    ];
    expect(uploadBatchSummaryText(outcomes)).toBe(
      '共 5 张:1 张已归档,1 张待核对,1 张此前已归档,2 张失败',
    );
  });

  it('全部已归档时不追加其他分类', () => {
    expect(uploadBatchSummaryText([done(report()), done(report())])).toBe('共 2 张:2 张已归档');
  });
});

describe('uploadOutcomeLabel', () => {
  it('传输失败 / 重复 / 识别失败 / 待核对 / 已归档', () => {
    expect(uploadOutcomeLabel({ kind: 'failed', message: 'timeout' })).toBe('上传失败:timeout');
    expect(uploadOutcomeLabel(done(report(), true))).toBe('此前已归档');
    expect(
      uploadOutcomeLabel(done(report({ status: 'failed', fail_reason: 'no metric items recognized' }))),
    ).toBe('识别失败:no metric items recognized');
    expect(uploadOutcomeLabel(done(report({ status: 'failed', fail_reason: null })))).toBe(
      '识别失败:未识别出指标项',
    );
    expect(uploadOutcomeLabel(done(report({ status: 'partial' })))).toBe('待核对');
    expect(uploadOutcomeLabel(done(report()))).toBe('已归档');
  });
});
