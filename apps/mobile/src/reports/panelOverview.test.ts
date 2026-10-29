import lft02Json from '../../../../samples/expected/report/lft_02.json';
import lft06Json from '../../../../samples/expected/report/lft_06.json';
import lft08Json from '../../../../samples/expected/report/lft_08.json';
import tft01Json from '../../../../samples/expected/report/tft_01.json';
import ua05Json from '../../../../samples/expected/report/ua_05.json';
import us01Json from '../../../../samples/expected/report/us_01.json';
import type {
  PanelDetail,
  PanelItem,
  PanelSummary,
  ReportDetail,
  ReportItem,
  ReportSummary,
} from '../api/reports';

import {
  abnormalItemCount,
  abnormalPanelCount,
  abnormalPanelGroups,
  filterReportsByPanel,
  latestFailedReportAfter,
  panelItemIsAbnormal,
  panelSourceLabel,
  parsePanelParam,
  reportHasPanel,
  reportItemGroups,
  reportPanelLabels,
  reportTypeNote,
  splitPanelItems,
  totalAbnormalCount,
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
  report_date: string | null;
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

function panelItemOf(file: GoldenReport, index: number, reportId: string): PanelItem {
  const item = file.items[index];
  const point = {
    report_id: reportId,
    report_date: file.report_date,
    value: item.value,
    unit: item.unit,
    ref_range: item.ref_range,
    method: item.method,
    flag: item.flag,
    low_confidence: item.low_confidence,
    reviewed: false,
  };
  return {
    name: item.name,
    unit: item.unit,
    ref_range: item.ref_range,
    latest: point,
    points: [point],
    report_count: 1,
    not_tested_count: 0,
    latest_report_not_tested: false,
  };
}

function panelDetailOf(
  file: GoldenReport,
  panel: PanelDetail['panel'],
  reportId: string,
): PanelDetail {
  return {
    panel,
    member_id: 1,
    report_count: 1,
    latest_report_date: file.report_date,
    latest_report_id: reportId,
    report_types: [file.report_type],
    narrative: false,
    items: file.items
      .map((item, index) => ({ item, index }))
      .filter(({ item }) => item.panel === panel)
      .map(({ index }) => panelItemOf(file, index, reportId)),
  };
}

function panelSummaryOf(
  panel: PanelSummary['panel'],
  abnormalCount: number,
): PanelSummary {
  return {
    panel,
    report_count: 1,
    item_count: 1,
    abnormal_count: abnormalCount,
    latest_report_date: '2026-10-01',
    latest_report_id: 'r'.repeat(64),
    report_types: ['生化'],
    narrative: false,
  };
}

describe('reportPanelLabels', () => {
  it('列出全部项目组,不折叠', () => {
    expect(reportPanelLabels(summary(lft02.panels))).toBe('肝功 · 肾功 · 尿酸');
    expect(reportPanelLabels(summary(lft06.panels))).toBe('肝功 · 血脂');
    expect(reportPanelLabels(summary(lft08.panels))).toBe('肝功 · 肾功 · 尿酸 · 电解质');
    expect(reportPanelLabels(summary(tft01.panels))).toBe('甲状腺');
    expect(reportPanelLabels(summary(ua05.panels))).toBe('尿常规');
  });

  it('失败 / 未回填存量回退 report_type', () => {
    expect(
      reportPanelLabels(summary(['other'], { report_type: '其他专项', status: 'failed' })),
    ).toBe('其他专项');
    expect(reportPanelLabels(summary([], { report_type: '血常规' }))).toBe('血常规');
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
  it('lft_02 按 panels 顺序分三组,组内异常在前、正常在后', () => {
    const groups = reportItemGroups(detailOf(lft02));
    expect(groups.map((group) => group.label)).toEqual(['肝功', '肾功', '尿酸']);
    expect(groups.map((group) => group.items.length)).toEqual([12, 4, 1]);
    expect(groups.map((group) => abnormalItemCount(group.items))).toEqual([6, 0, 0]);
    expect(groups[0].items.slice(0, 6).map((item) => item.flag)).toEqual([
      'high',
      'high',
      'high',
      'high',
      'high',
      'low',
    ]);
    expect(groups[0].items.slice(6).map((item) => item.name)).toEqual([
      '总胆红素',
      '直接胆红素',
      '总蛋白',
      '白蛋白',
      '球蛋白',
      '白蛋白/球蛋白',
    ]);
    expect(groups[1].items[0].name).toBe('尿素');
    expect(groups[2].items[0].name).toBe('尿酸');
    expect(groups.flatMap((group) => group.items)).toHaveLength(lft02.items.length);
  });

  it('组内可操作未知靠前;不可操作未知 / 正常 / 未检测靠后并保持原序', () => {
    const base = itemOf(lft02, 2);
    const item = (overrides: Partial<ReportItem>): ReportItem => ({ ...base, ...overrides });
    const detail: ReportDetail = {
      ...detailOf(lft02),
      panels: ['liver'],
      items: [
        item({ item_id: 1, name: '正常项', flag: 'normal' }),
        item({ item_id: 2, name: '信息项', flag: 'unknown', flag_reason: 'informational' }),
        item({ item_id: 3, name: 'OCR 漏项', flag: 'unknown', flag_reason: 'ocr_miss' }),
        item({ item_id: 4, name: '未检测项', flag: 'not_tested' }),
      ],
    };
    const groups = reportItemGroups(detail);
    expect(groups[0].items.map((entry) => entry.item_id)).toEqual([3, 1, 2, 4]);
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

describe('按项目筛选', () => {
  it('报告包含多个项目组时每个项目都命中;failed 不参与', () => {
    const lft02Report = summary(lft02.panels);
    expect(reportHasPanel(lft02Report, 'liver')).toBe(true);
    expect(reportHasPanel(lft02Report, 'kidney')).toBe(true);
    expect(reportHasPanel(lft02Report, 'uric_acid')).toBe(true);
    expect(reportHasPanel(lft02Report, 'thyroid')).toBe(false);
    expect(reportHasPanel({ ...lft02Report, status: 'failed' }, 'liver')).toBe(false);
  });

  it('未回填存量 panels=[] 按 other 参与筛选;其余项目不命中', () => {
    const legacy = summary([], { report_type: '临检' });
    expect(reportHasPanel(legacy, 'other')).toBe(true);
    expect(reportHasPanel(legacy, 'cbc')).toBe(false);
  });

  it('filterReportsByPanel 保持入参顺序', () => {
    const reports = [summary(lft06.panels), summary(tft01.panels), summary(lft08.panels)];
    expect(filterReportsByPanel(reports, 'lipid')).toEqual([reports[0]]);
    expect(filterReportsByPanel(reports, 'electrolyte')).toEqual([reports[2]]);
    expect(filterReportsByPanel(reports, 'thyroid')).toEqual([reports[1]]);
  });

  it('parsePanelParam 只接受合法 code,数组取首值', () => {
    expect(parsePanelParam('kidney')).toBe('kidney');
    expect(parsePanelParam(['liver', 'kidney'])).toBe('liver');
    expect(parsePanelParam('pathology')).toBeUndefined();
    expect(parsePanelParam(undefined)).toBeUndefined();
  });
});

describe('项目列表 / 详情展示口径', () => {
  it('other 行显示来源报告类别;非 other 不显示', () => {
    expect(panelSourceLabel('other', ['临检', '免疫'])).toBe('其他 · 临检、免疫');
    expect(panelSourceLabel('liver', ['生化'])).toBeUndefined();
    expect(panelSourceLabel('other', [])).toBeUndefined();
  });

  it('异常项以 latest flag 为准;未检测 / 无点不算异常', () => {
    expect(panelItemIsAbnormal(panelItem({ flag: 'high' }))).toBe(true);
    expect(panelItemIsAbnormal(panelItem({ flag: 'abnormal' }))).toBe(true);
    expect(panelItemIsAbnormal(panelItem({ flag: 'normal' }))).toBe(false);
    expect(panelItemIsAbnormal(panelItem({ flag: 'not_tested', latest: null }))).toBe(false);
    expect(panelItemIsAbnormal(panelItem({ latest: null }))).toBe(false);
  });

  it('splitPanelItems 异常在前,其余保持后端顺序', () => {
    const items = [
      panelItem({ name: 'A', flag: 'normal' }),
      panelItem({ name: 'B', flag: 'high' }),
      panelItem({ name: 'C', latest: null }),
      panelItem({ name: 'D', flag: 'low' }),
    ];
    const { abnormal, rest } = splitPanelItems(items);
    expect(abnormal.map((item) => item.name)).toEqual(['B', 'D']);
    expect(rest.map((item) => item.name)).toEqual(['A', 'C']);
  });

  it('abnormalPanelGroups 按传入项目顺序分组,只保留 latest 异常的指标,空组不输出', () => {
    const details = lft08.panels.map((panel) =>
      panelDetailOf(lft08, panel, 'lft_08'),
    );
    const groups = abnormalPanelGroups(details);

    expect(groups.map((group) => group.panel)).toEqual(['liver', 'uric_acid']);
    expect(groups.map((group) => group.reportDate)).toEqual(['2020-01-01', '2020-01-01']);
    expect(groups[0].items.map((item) => item.name)).toEqual(['总蛋白', '白蛋白']);
    expect(groups[1].items.map((item) => item.name)).toEqual(['尿酸']);
  });

  it('abnormalPanelGroups 的组头计数 = 行数,与 abnormal_count 同口径', () => {
    const details = lft02.panels.map((panel) =>
      panelDetailOf(lft02, panel, 'lft_02'),
    );
    const groups = abnormalPanelGroups(details);

    expect(groups).toHaveLength(1);
    expect(groups[0].panel).toBe('liver');
    expect(groups[0].items).toHaveLength(6);
    expect(groups[0].items[0].latest?.flag).toBe('high');
  });

  it('abnormalPanelGroups 对全部 normal / unknown 与空详情返回空列表', () => {
    expect(abnormalPanelGroups([])).toEqual([]);
    expect(
      abnormalPanelGroups([panelDetailOf(tft01, 'thyroid', 'tft_01')]),
    ).toEqual([]);
  });

  it('totalAbnormalCount 汇总所有项目异常数', () => {
    expect(totalAbnormalCount([])).toBe(0);
    expect(
      totalAbnormalCount([
        panelSummaryOf('liver', 4),
        panelSummaryOf('kidney', 0),
        panelSummaryOf('thyroid', 2),
      ]),
    ).toBe(6);
  });

  it('abnormalPanelCount 只数有异常的项目组,不数异常指标', () => {
    expect(abnormalPanelCount([])).toBe(0);
    expect(
      abnormalPanelCount([
        panelSummaryOf('liver', 4),
        panelSummaryOf('kidney', 0),
        panelSummaryOf('thyroid', 2),
      ]),
    ).toBe(2);
  });

  it('latestFailedReportAfter 只返回比当前项目最新报告更新的 failed 报告', () => {
    const latestDate = '2026-10-01';
    const latestId = 'a'.repeat(64);
    const failed = (date: string | null, id: string) =>
      summary(['other'], { report_id: id, report_date: date, status: 'failed' });

    expect(
      latestFailedReportAfter([failed('2026-10-02', 'b'.repeat(64))], latestDate, latestId)
        ?.report_id,
    ).toBe('b'.repeat(64));
    // 同日:report_id 降序更大者更新(与服务端最近报告口径一致)。
    expect(
      latestFailedReportAfter([failed(latestDate, 'f'.repeat(64))], latestDate, latestId)
        ?.report_id,
    ).toBe('f'.repeat(64));
    expect(
      latestFailedReportAfter([failed(latestDate, '0'.repeat(64))], latestDate, latestId),
    ).toBeUndefined();
    expect(
      latestFailedReportAfter([failed('2026-09-30', 'f'.repeat(64))], latestDate, latestId),
    ).toBeUndefined();
    expect(
      latestFailedReportAfter([failed(null, 'f'.repeat(64))], latestDate, latestId),
    ).toBeUndefined();
    expect(
      latestFailedReportAfter(
        [failed('2026-10-02', 'b'.repeat(64)), failed('2026-10-03', 'c'.repeat(64))],
        latestDate,
        latestId,
      )?.report_id,
    ).toBe('c'.repeat(64));
  });
});

function panelItem({
  name = '指标',
  flag = 'normal',
  latest = true,
}: {
  name?: string;
  flag?: NonNullable<PanelItem['latest']>['flag'];
  latest?: boolean | null;
}): PanelItem {
  const point = {
    report_id: 'r'.repeat(64),
    report_date: '2026-10-01',
    value: '1.0',
    unit: 'U/L',
    ref_range: null,
    method: null,
    flag,
    low_confidence: false,
    reviewed: false,
  };
  return {
    name,
    unit: 'U/L',
    ref_range: null,
    latest: latest ? point : null,
    points: latest ? [point] : [],
    report_count: latest ? 1 : 0,
    not_tested_count: latest ? 0 : 1,
    latest_report_not_tested: !latest,
  };
}
