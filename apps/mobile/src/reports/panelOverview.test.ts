import lft02Json from '../../../../samples/expected/report/lft_02.json';
import lft06Json from '../../../../samples/expected/report/lft_06.json';
import lft08Json from '../../../../samples/expected/report/lft_08.json';
import tft01Json from '../../../../samples/expected/report/tft_01.json';
import ua05Json from '../../../../samples/expected/report/ua_05.json';
import us01Json from '../../../../samples/expected/report/us_01.json';
import type { ReportDetail, ReportItem, ReportSummary } from '../api/reports';

import {
  abnormalItemCount,
  reportItemGroups,
  reportPanelLabels,
  reportTypeNote,
} from './panelOverview';

type GoldenItem = {
  name: string;
  panel: ReportItem['panel'];
  value: string | null;
  unit: string | null;
  ref_range: string | null;
  method: string | null;
  flag: ReportItem['flag'];
  low_confidence: boolean;
  raw_text: string;
};

type GoldenReport = {
  report_type: string;
  panels: ReportSummary['panels'];
  items: GoldenItem[];
};

const golden = (json: unknown) => json as GoldenReport;

const lft02 = golden(lft02Json);
const lft06 = golden(lft06Json);
const lft08 = golden(lft08Json);
const tft01 = golden(tft01Json);
const ua05 = golden(ua05Json);
const us01 = golden(us01Json);

function summary(
  panels: ReportSummary['panels'],
  overrides: Partial<ReportSummary> = {},
): ReportSummary {
  return {
    report_id: 'r'.repeat(64),
    member_id: 1,
    report_type: '生化',
    report_date: '2026-10-01',
    status: 'success',
    fail_reason: null,
    flag_counts: { high: 0, low: 0, abnormal: 0, unknown: 0 },
    panels,
    not_tested_count: 0,
    unreviewed_low_confidence_count: 0,
    ...overrides,
  };
}

function itemOf(file: GoldenReport, index: number): ReportItem {
  const item = file.items[index];
  return {
    item_id: index + 1,
    name: item.name,
    panel: item.panel,
    value: item.value,
    unit: item.unit,
    ref_range: item.ref_range,
    method: item.method,
    flag: item.flag,
    low_confidence: item.low_confidence,
    raw_text: item.raw_text,
    review_status: 'unreviewed',
    ocr_lines: [],
    prev: null,
  };
}

function detailOf(file: GoldenReport, overrides: Partial<ReportDetail> = {}): ReportDetail {
  return {
    ...summary(file.panels, { report_type: file.report_type }),
    items: file.items.map((_, index) => itemOf(file, index)),
    ...overrides,
  };
}

describe('reportPanelLabels', () => {
  it('前 2 个项目组 + +N;单项目不加 +N', () => {
    expect(reportPanelLabels(summary(lft02.panels))).toBe('肝功 · 肾功 +1');
    expect(reportPanelLabels(summary(lft06.panels))).toBe('肝功 · 血脂');
    expect(reportPanelLabels(summary(tft01.panels))).toBe('甲状腺');
    expect(reportPanelLabels(summary(ua05.panels))).toBe('尿常规');
  });

  it('失败 / 未回填存量回退 report_type', () => {
    expect(
      reportPanelLabels(summary(['other'], { report_type: '其他专项', status: 'failed' })),
    ).toBe('其他专项');
    expect(reportPanelLabels(summary([], { report_type: '血常规' }))).toBe('血常规');
  });

  it('max 参数控制展示数量', () => {
    expect(reportPanelLabels(summary(lft08.panels), 1)).toBe('肝功 +3');
  });
});

describe('reportTypeNote', () => {
  it('项目组作为主标签后,report_type 作为次要信息;相同或回退时不重复', () => {
    expect(reportTypeNote(summary(lft02.panels))).toBe('生化');
    expect(reportTypeNote(summary(tft01.panels, { report_type: '激素' }))).toBe('激素');
    expect(reportTypeNote(summary(us01.panels, { report_type: '影像' }))).toBeUndefined();
    expect(reportTypeNote(summary(['other'], { status: 'failed' }))).toBeUndefined();
    expect(reportTypeNote(summary([], { report_type: '血常规' }))).toBeUndefined();
  });
});

describe('reportItemGroups', () => {
  it('lft_02 按 panels 顺序分三组,组内展示全部指标且保持报告顺序', () => {
    const groups = reportItemGroups(detailOf(lft02));
    expect(groups.map((group) => group.label)).toEqual(['肝功', '肾功', '尿酸']);
    expect(groups.map((group) => group.items.length)).toEqual([12, 4, 1]);
    expect(groups.map((group) => abnormalItemCount(group.items))).toEqual([6, 0, 0]);
    expect(groups[0].items[0].name).toBe('丙氨酸转氨酶');
    expect(groups[1].items[0].name).toBe('尿素');
    expect(groups[2].items[0].name).toBe('尿酸');
    expect(groups.flatMap((group) => group.items)).toHaveLength(lft02.items.length);
  });

  it('lft_08 四组都出现(肝功 / 肾功 / 尿酸 / 电解质)', () => {
    const groups = reportItemGroups(detailOf(lft08));
    expect(groups.map((group) => group.label)).toEqual(['肝功', '肾功', '尿酸', '电解质']);
    expect(groups.map((group) => group.items.length)).toEqual([13, 2, 1, 1]);
    expect(groups.map((group) => abnormalItemCount(group.items))).toEqual([2, 0, 1, 0]);
  });

  it('单项目报告只出现一组;影像单无指标项不出现空组', () => {
    expect(reportItemGroups(detailOf(tft01)).map((group) => group.label)).toEqual(['甲状腺']);
    expect(reportItemGroups(detailOf(us01))).toEqual([]);
  });

  it('未回填存量 panels=[] 按 report_type 单组回退,不出现空标题', () => {
    const groups = reportItemGroups(detailOf(lft02, { panels: [] }));
    expect(groups).toHaveLength(1);
    expect(groups[0].panel).toBeNull();
    expect(groups[0].label).toBe('生化');
    expect(groups[0].items).toHaveLength(lft02.items.length);
  });

  it('item.panel 不在 report.panels 时归入末尾「其他」组', () => {
    const detail = detailOf(lft06);
    const stray: ReportItem = { ...itemOf(lft06, 0), item_id: 99, panel: 'other' };
    const groups = reportItemGroups({ ...detail, items: [...detail.items, stray] });
    expect(groups.map((group) => group.label)).toEqual(['肝功', '血脂', '其他']);
    expect(groups[2].items.map((item) => item.item_id)).toEqual([99]);
  });
});
