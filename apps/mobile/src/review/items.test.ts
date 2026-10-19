import type { ReportDetail, ReportItem } from '../api/reports';

import {
  applyReviewResult,
  buildCorrection,
  countFlagCounts,
  draftFromItem,
  needsReview,
  pendingItems,
  reviewStatusLabel,
} from './items';

// 真实形状的指标项(字段取自 shared stack 样例报告:单核细胞比率低置信 high;
// 血红蛋白高置信 normal)。
function item(overrides: Partial<ReportItem> = {}): ReportItem {
  return {
    item_id: 7,
    name: '单核细胞比率',
    value: '10.8',
    unit: '%',
    ref_range: '3-10',
    method: null,
    flag: 'high',
    low_confidence: true,
    raw_text: '单核细胞百分比  10.8  ↑ %  3-10',
    review_status: 'unreviewed',
    ocr_lines: [],
    prev: null,
    ...overrides,
  };
}

function report(overrides: Partial<ReportDetail> = {}): ReportDetail {
  return {
    report_id: 'a'.repeat(64),
    member_id: 1,
    report_type: '血常规',
    report_date: '2026-10-01',
    status: 'partial',
    fail_reason: null,
    flag_counts: { high: 1, low: 0, unknown: 0 },
    unreviewed_low_confidence_count: 1,
    exam_name: null,
    exam_name_ocr_lines: null,
    findings: null,
    findings_ocr_lines: null,
    impression: null,
    impression_ocr_lines: null,
    items: [item()],
    ...overrides,
  };
}

describe('needsReview', () => {
  it('partial 触发;干净 success(全量高置信)免核对;failed 无可核对项', () => {
    expect(needsReview(report())).toBe(true);
    expect(needsReview(report({ status: 'success' }))).toBe(true); // 含未核对低置信项
    expect(
      needsReview(report({ status: 'success', items: [item({ low_confidence: false })] })),
    ).toBe(false);
    expect(
      needsReview(report({ status: 'failed', items: [], unreviewed_low_confidence_count: 0 })),
    ).toBe(false);
  });

  it('低置信项已确信(confirmed/corrected)不再触发;已修正后再修正仍可', () => {
    expect(
      needsReview(report({ status: 'success', items: [item({ review_status: 'confirmed' })] })),
    ).toBe(false);
    expect(
      needsReview(report({ status: 'success', items: [item({ review_status: 'corrected' })] })),
    ).toBe(false);
  });
});

describe('pendingItems', () => {
  it('只算未核对的低置信项(剩余 N 项口径);已确认/已修正与高置信项不入列', () => {
    const items = [
      item({ item_id: 1 }),
      item({ item_id: 2, review_status: 'confirmed' }),
      item({ item_id: 3, low_confidence: false }),
      item({ item_id: 4, review_status: 'corrected' }),
      item({ item_id: 5 }),
    ];
    expect(pendingItems(items).map((it) => it.item_id)).toEqual([1, 5]);
  });
});

describe('reviewStatusLabel', () => {
  it('已确认/已修正有标签;未核对无标签', () => {
    expect(reviewStatusLabel('confirmed')).toBe('已确认');
    expect(reviewStatusLabel('corrected')).toBe('已修正');
    expect(reviewStatusLabel('unreviewed')).toBeUndefined();
  });
});

describe('countFlagCounts', () => {
  it('与服务端 FlagCounts 同口径:normal 不计,缺档即 0', () => {
    const counts = countFlagCounts([
      item({ flag: 'high' }),
      item({ flag: 'low' }),
      item({ flag: 'low' }),
      item({ flag: 'unknown' }),
      item({ flag: 'normal' }),
    ]);
    expect(counts).toEqual({ high: 1, low: 2, unknown: 1 });
    expect(countFlagCounts([])).toEqual({ high: 0, low: 0, unknown: 0 });
  });
});

describe('applyReviewResult', () => {
  it('单响应刷新整屏:替换本次指标项、更新报告级派生、重算 flag 徽标', () => {
    const before = report({
      items: [item({ item_id: 1 }), item({ item_id: 2, low_confidence: false, flag: 'normal' })],
      flag_counts: { high: 1, low: 0, unknown: 0 },
    });
    const after = applyReviewResult(before, {
      report_id: before.report_id,
      status: 'success',
      unreviewed_low_confidence_count: 0,
      item: item({ item_id: 1, value: '5.6', flag: 'normal', review_status: 'corrected' }),
    });
    expect(after.status).toBe('success');
    expect(after.unreviewed_low_confidence_count).toBe(0);
    expect(after.flag_counts).toEqual({ high: 0, low: 0, unknown: 0 }); // 修正后按新值重算
    expect(after.items[0].value).toBe('5.6');
    expect(after.items[0].review_status).toBe('corrected');
    expect(after.items[1]).toBe(before.items[1]); // 其余项原样
  });
});

describe('draftFromItem / buildCorrection', () => {
  it('草稿初值取当前生效值,null 显示为空串', () => {
    expect(draftFromItem(item())).toEqual({ name: '单核细胞比率', value: '10.8', unit: '%', ref_range: '3-10' });
    expect(draftFromItem(item({ value: null, unit: null, ref_range: null }))).toEqual({
      name: '单核细胞比率',
      value: '',
      unit: '',
      ref_range: '',
    });
  });

  it('只带与当前生效值不同的字段(去空白比较/发送)', () => {
    const current = item();
    expect(buildCorrection(current, { name: '单核细胞百分比', value: '10.8', unit: '%', ref_range: '3-10' })).toEqual({
      action: 'correct',
      name: '单核细胞百分比',
    });
    expect(buildCorrection(current, { name: '单核细胞比率', value: ' 5.6 ', unit: '%', ref_range: '3-10' })).toEqual({
      action: 'correct',
      value: '5.6',
    });
    expect(buildCorrection(current, { name: '单核细胞比率 ', value: '10.8', unit: '%', ref_range: '3-10' })).toBeUndefined();
  });

  it('补录被清空的事实:null → 非空发送;清空:非 null → 空串发送(契约允许空串)', () => {
    const missing = item({ value: null });
    expect(buildCorrection(missing, { name: '单核细胞比率', value: '', unit: '%', ref_range: '3-10' })).toBeUndefined();
    expect(buildCorrection(missing, { name: '单核细胞比率', value: '9.9', unit: '%', ref_range: '3-10' })).toEqual({
      action: 'correct',
      value: '9.9',
    });
    expect(
      buildCorrection(item({ unit: 'g/L' }), { name: '单核细胞比率', value: '10.8', unit: '', ref_range: '3-10' }),
    ).toEqual({ action: 'correct', unit: '' });
  });

  it('名称空白无效(契约 minLength: 1);四项无差异不产生请求(不给 no-op 修正造修正对)', () => {
    const current = item();
    expect(buildCorrection(current, { name: '  ', value: '10.8', unit: '%', ref_range: '3-10' })).toBeUndefined();
    expect(buildCorrection(current, { name: '单核细胞比率', value: '10.8', unit: '%', ref_range: '3-10' })).toBeUndefined();
  });
});
