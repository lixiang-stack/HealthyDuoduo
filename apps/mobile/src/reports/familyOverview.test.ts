import type { PanelSummary, ReportItem, ReportSummary } from '../api/reports';

import cbc03Json from '../../../../samples/expected/report/cbc_03.json';
import cbc04Json from '../../../../samples/expected/report/cbc_04.json';
import us03Json from '../../../../samples/expected/report/us_03.json';
import { familyCardSummary } from './familyOverview';

type GoldenReport = {
  report_type: string;
  report_date: string | null;
  status: ReportSummary['status'];
  panels: ReportSummary['panels'];
  items: { flag: ReportItem['flag'] }[];
};

const golden = (json: unknown) => json as GoldenReport;

const cbc03 = golden(cbc03Json);
const cbc04 = golden(cbc04Json);
const us03 = golden(us03Json);

const TODAY = new Date(2026, 9, 28);

function countsOf(file: GoldenReport): ReportSummary['flag_counts'] {
  const counts = { high: 0, low: 0, abnormal: 0, unknown: 0 };
  for (const item of file.items) {
    if (
      item.flag === 'high' ||
      item.flag === 'low' ||
      item.flag === 'abnormal' ||
      item.flag === 'unknown'
    ) {
      counts[item.flag] += 1;
    }
  }
  return counts;
}

function reportOf(
  file: GoldenReport,
  reportId: string,
  overrides: Partial<ReportSummary> = {},
): ReportSummary {
  return {
    report_id: reportId,
    member_id: 1,
    report_type: file.report_type,
    report_date: file.report_date,
    status: file.status,
    fail_reason: null,
    flag_counts: countsOf(file),
    panels: file.panels,
    not_tested_count: 0,
    unreviewed_low_confidence_count: 0,
    ...overrides,
  };
}

function panelOf(panel: PanelSummary['panel'], abnormalCount: number): PanelSummary {
  return {
    panel,
    report_count: 1,
    item_count: 1,
    abnormal_count: abnormalCount,
    latest_report_date: '2026-10-01',
    latest_report_id: 'r'.repeat(64),
    report_types: ['临检'],
    narrative: false,
  };
}

describe('familyCardSummary', () => {
  it('partial 最新报告:待核对份数 + 项数、项目名、异常项目、偏高偏低、报告总数', () => {
    const summary = familyCardSummary(
      {
        reports: [
          reportOf(cbc04, 'cbc_04', { unreviewed_low_confidence_count: 5 }),
        ],
        panels: [panelOf('cbc', 7)],
      },
      TODAY,
    );

    expect(summary).toEqual({
      hasReports: true,
      hasPending: true,
      pending: '待核对 1 份 · 5 项',
      latest: '最近：血常规 · 2024-08-19 · 2 年前',
      abnormal: '异常指标 7 项 · 1 个项目',
      latestFlags: '最近报告 偏高 2 · 偏低 5',
      reportCount: '报告 1 份',
    });
  });

  it('异常指标只给总数 + 项目数,不再列项目分项', () => {
    const summary = familyCardSummary(
      {
        reports: [reportOf(cbc04, 'cbc_04')],
        panels: [panelOf('cbc', 9), panelOf('liver', 4), panelOf('thyroid', 2)],
      },
      TODAY,
    );
    expect(summary.abnormal).toBe('异常指标 15 项 · 3 个项目');
  });

  it('多份报告取最新一份;影像主项目直接显示,异常项目来自项目列表', () => {
    const summary = familyCardSummary(
      {
        reports: [
          reportOf(cbc03, 'cbc_03', { unreviewed_low_confidence_count: 1 }),
          reportOf(us03, 'us_03'),
        ],
        panels: [panelOf('cbc', 1), panelOf('imaging', 0)],
      },
      TODAY,
    );

    expect(summary.pending).toBe('待核对 1 份 · 1 项');
    expect(summary.latest).toBe('最近：影像 · 2024-09-15 · 2 年前');
    expect(summary.abnormal).toBe('异常指标 1 项 · 1 个项目');
    expect(summary.latestFlags).toBe('最近报告 偏高 0 · 偏低 0');
    expect(summary.reportCount).toBe('报告 2 份');
  });

  it('无报告:只给空文案,不显示待核对 / 异常项目 / 偏高偏低 / 总数', () => {
    expect(familyCardSummary({ reports: [], panels: [] }, TODAY)).toEqual({
      hasReports: false,
      latest: '这位成员还没有检查单',
    });
  });

  it('最新报告失败:显示失败文案与识别失败份数,不显示偏高偏低', () => {
    const failed: ReportSummary = {
      report_id: 'failed',
      member_id: 1,
      report_type: '生化',
      report_date: '2025-01-01',
      status: 'failed',
      fail_reason: '未识别出指标项',
      flag_counts: { high: 0, low: 0, abnormal: 0, unknown: 0 },
      panels: [],
      not_tested_count: 0,
      unreviewed_low_confidence_count: 0,
    };
    const summary = familyCardSummary(
      {
        reports: [
          reportOf(cbc04, 'cbc_04', { unreviewed_low_confidence_count: 5 }),
          failed,
        ],
        panels: [panelOf('cbc', 7)],
      },
      TODAY,
    );

    expect(summary.pending).toBe('待核对 1 份 · 5 项 · 识别失败 1 份');
    expect(summary.latest).toBe('最近报告识别失败 · 2025-01-01');
    expect(summary.latestFlags).toBeUndefined();
    expect(summary.abnormal).toBe('异常指标 7 项 · 1 个项目');
    expect(summary.reportCount).toBe('报告 2 份');
  });

  it('未回填存量 panels=[] 的最新项目回退 report_type;无异常项目', () => {
    const summary = familyCardSummary(
      {
        reports: [reportOf(cbc03, 'cbc_03', { panels: [], report_date: '2026-10-28' })],
        panels: [],
      },
      TODAY,
    );

    expect(summary.latest).toBe('最近：临检 · 2026-10-28 · 今天');
    expect(summary.abnormal).toBeUndefined();
  });
});
