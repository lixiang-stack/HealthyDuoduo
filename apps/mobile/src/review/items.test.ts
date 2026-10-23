import type { ReportDetail, ReportItem } from '../api/reports';

import {
  applyReviewResult,
  buildCorrection,
  canMarkNoReference,
  countFlagCounts,
  draftFromItem,
  flagReasonText,
  isBackfillItem,
  needsReview,
  pendingItems,
  reviewActions,
  reviewStatusLabel,
  shouldShowZoom,
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
    flag_counts: { high: 1, low: 0, abnormal: 0, unknown: 0 },
    unreviewed_low_confidence_count: 1,
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
      item({ flag: 'abnormal' }),
      item({ flag: 'unknown', flag_reason: 'ocr_miss' }),
      item({ flag: 'unknown', flag_reason: 'no_reference' }),
      item({ flag: 'unknown', flag_reason: 'informational' }),
      item({ flag: 'normal' }),
    ]);
    expect(counts).toEqual({ high: 1, low: 2, abnormal: 1, unknown: 1 });
    expect(countFlagCounts([])).toEqual({ high: 0, low: 0, abnormal: 0, unknown: 0 });
  });
});

describe('flagReasonText', () => {
  it('unknown reason 给出发生了什么 + 下一步提示', () => {
    expect(flagReasonText('ocr_miss')).toContain('OCR 输出里没有');
    expect(flagReasonText('ocr_merge')).toContain('粘在一起');
    expect(flagReasonText('no_reference')).toContain('无需处理');
    expect(flagReasonText(undefined)).toBeUndefined();
  });
});

describe('canMarkNoReference', () => {
  it('有值、无范围、非信息项才可标记无参考范围', () => {
    expect(canMarkNoReference(item({ value: '4.19', ref_range: null, flag: 'unknown', flag_reason: 'ocr_miss', low_confidence: false }))).toBe(true);
    expect(canMarkNoReference(item({ value: null, ref_range: null, flag: 'unknown' }))).toBe(false);
    expect(canMarkNoReference(item({ value: '4.19', ref_range: null, flag: 'unknown', flag_reason: 'informational' }))).toBe(false);
    expect(canMarkNoReference(item({ value: '4.19', ref_range: null, flag: 'unknown', flag_reason: 'no_reference' }))).toBe(false);
  });
});

describe('isBackfillItem / reviewActions', () => {
  it('缺值 + 可解析范围、数值 + 反向范围才可补录', () => {
    expect(isBackfillItem(item({ value: null, ref_range: '0-20', flag: 'unknown', flag_reason: 'ocr_miss' }))).toBe(true);
    expect(isBackfillItem(item({ value: null, ref_range: '20-0', flag: 'unknown', flag_reason: 'ocr_merge' }))).toBe(true);
    expect(isBackfillItem(item({ value: '17.3', ref_range: '13.5--9.5', flag: 'unknown', flag_reason: 'ocr_merge' }))).toBe(true);
    expect(isBackfillItem(item({ value: null, ref_range: '5', flag: 'unknown', flag_reason: 'ocr_miss' }))).toBe(false);
    expect(isBackfillItem(item({ value: '17.3', ref_range: '3.5--9.5', flag: 'unknown' }))).toBe(false);
    expect(isBackfillItem(item({ value: '阴性', ref_range: '0-20', flag: 'unknown' }))).toBe(false);
    expect(isBackfillItem(item({ value: null, ref_range: '0-20', flag: 'unknown', flag_reason: 'no_reference' }))).toBe(false);
  });

  it('可补录不给确认,只给补录主操作;低置信非补录给确认', () => {
    const backfill = reviewActions(item({ value: null, ref_range: '0-20', flag: 'unknown', flag_reason: 'ocr_miss' }));
    expect(backfill).toMatchObject({ confirm: false, primary: 'backfill', correctLabel: '补录缺失信息' });
    const low = reviewActions(item({ value: null, ref_range: null, flag: 'unknown', flag_reason: 'ocr_miss' }));
    expect(low).toMatchObject({ confirm: true, primary: 'confirm', correctLabel: '修正' });
    const high = reviewActions(item({ low_confidence: false, value: '0', ref_range: null, flag: 'unknown', flag_reason: 'ocr_miss' }));
    expect(high).toMatchObject({ confirm: false, primary: null, correctLabel: '修正' });
    const corrected = reviewActions(item({ value: null, ref_range: '0-20', flag: 'unknown', flag_reason: 'ocr_miss', review_status: 'corrected' }));
    expect(corrected).toMatchObject({ primary: 'backfill', correctLabel: '补录缺失信息' });
  });
});

describe('applyReviewResult', () => {
  it('单响应刷新整屏:替换本次指标项、更新报告级派生、重算 flag 徽标', () => {
    const before = report({
      items: [item({ item_id: 1 }), item({ item_id: 2, low_confidence: false, flag: 'normal' })],
      flag_counts: { high: 1, low: 0, abnormal: 0, unknown: 0 },
    });
    const after = applyReviewResult(before, {
      report_id: before.report_id,
      status: 'success',
      unreviewed_low_confidence_count: 0,
      item: item({ item_id: 1, value: '5.6', flag: 'normal', review_status: 'corrected' }),
    });
    expect(after.status).toBe('success');
    expect(after.unreviewed_low_confidence_count).toBe(0);
    expect(after.flag_counts).toEqual({ high: 0, low: 0, abnormal: 0, unknown: 0 }); // 修正后按新值重算
    expect(after.items[0].value).toBe('5.6');
    expect(after.items[0].review_status).toBe('corrected');
    expect(after.items[1]).toBe(before.items[1]); // 其余项原样
  });
});

describe('shouldShowZoom', () => {
  const line = { line: 169, box: [[1, 2], [3, 2], [3, 4], [1, 4]] };

  it('低置信项默认展开;高置信项默认收起(未点修正也能按需展开)', () => {
    expect(
      shouldShowZoom(item({ low_confidence: true, ocr_lines: [line] }), { editing: false }),
    ).toBe(true);
    expect(
      shouldShowZoom(item({ low_confidence: false, ocr_lines: [line] }), { editing: false }),
    ).toBe(false);
    expect(
      shouldShowZoom(item({ low_confidence: false, ocr_lines: [line] }), {
        open: true,
        editing: false,
      }),
    ).toBe(true);
    expect(
      shouldShowZoom(item({ low_confidence: true, ocr_lines: [line] }), {
        open: false,
        editing: false,
      }),
    ).toBe(false);
  });

  it('无 OCR 行链接不显示定位(不渲染空视口);编辑态在有行链接时仍可见', () => {
    expect(shouldShowZoom(item({ low_confidence: true, ocr_lines: [] }), { editing: false })).toBe(
      false,
    );
    expect(shouldShowZoom(item({ low_confidence: false, ocr_lines: [] }), { editing: true })).toBe(
      false,
    );
    expect(shouldShowZoom(item({ low_confidence: false, ocr_lines: [line] }), { editing: true })).toBe(
      true,
    );
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
