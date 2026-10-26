import type { ReportSummary } from '../api/reports';

import {
  filterReports,
  latestByCategory,
  parseReportFilter,
  pendingSummary,
  type ReportFilter,
} from './overview';

function report(overrides: Partial<ReportSummary> = {}): ReportSummary {
  return {
    report_id: 'report',
    member_id: 1,
    report_type: '血常规',
    report_date: '2026-10-01',
    status: 'success',
    fail_reason: null,
    flag_counts: { high: 0, low: 0, abnormal: 0, unknown: 0 },
    not_tested_count: 0,
    unreviewed_low_confidence_count: 0,
    ...overrides,
  };
}

describe('filterReports', () => {
  const reports = [
    report({ report_id: 'partial', status: 'partial' }),
    report({ report_id: 'failed', status: 'failed' }),
    report({ report_id: 'success', status: 'success' }),
    report({ report_id: 'imaging-partial', status: 'partial', report_type: '影像' }),
  ];

  it('partial 只保留待核对的非影像报告', () => {
    expect(filterReports(reports, 'partial').map((r) => r.report_id)).toEqual(['partial']);
  });

  it('failed 保留全部识别失败报告(含影像单)', () => {
    const withFailedImaging = [
      ...reports,
      report({ report_id: 'imaging-failed', status: 'failed', report_type: '影像' }),
    ];
    expect(filterReports(withFailedImaging, 'failed').map((r) => r.report_id)).toEqual([
      'failed',
      'imaging-failed',
    ]);
  });

  it('无筛选按入参顺序返回全部', () => {
    expect(filterReports(reports, undefined).map((r) => r.report_id)).toEqual([
      'partial',
      'failed',
      'success',
      'imaging-partial',
    ]);
  });

  it('筛选保持入参顺序', () => {
    const ordered = [
      report({ report_id: 'b', status: 'partial' }),
      report({ report_id: 'a', status: 'partial' }),
    ];
    expect(filterReports(ordered, 'partial').map((r) => r.report_id)).toEqual(['b', 'a']);
  });

  it('非法筛选值按无筛选返回全部', () => {
    expect(filterReports(reports, 'unknown' as ReportFilter).map((r) => r.report_id)).toEqual([
      'partial',
      'failed',
      'success',
      'imaging-partial',
    ]);
  });
});

describe('pendingSummary', () => {
  it('partial 计份数与未核对项之和;影像单与 failed 不计入待核对', () => {
    const reports = [
      report({ report_id: 'p1', status: 'partial', unreviewed_low_confidence_count: 2 }),
      report({ report_id: 'p2', status: 'partial', unreviewed_low_confidence_count: 0 }),
      report({
        report_id: 'imaging-partial',
        status: 'partial',
        report_type: '影像',
        unreviewed_low_confidence_count: 5,
      }),
      report({ report_id: 'success', status: 'success' }),
      report({ report_id: 'failed', status: 'failed' }),
      report({ report_id: 'imaging-failed', status: 'failed', report_type: '影像' }),
    ];
    expect(pendingSummary(reports)).toEqual({
      partialReports: 2,
      unreviewedItems: 2,
      failedReports: 2,
    });
  });

  it('空列表全为 0', () => {
    expect(pendingSummary([])).toEqual({
      partialReports: 0,
      unreviewedItems: 0,
      failedReports: 0,
    });
  });
});

describe('latestByCategory', () => {
  it('每类取日期最新的一份,类别按最近日期降序,无日期沉底', () => {
    const reports = [
      report({ report_id: 'cbc-old', report_type: '血常规', report_date: '2026-09-01' }),
      report({ report_id: 'cbc-new', report_type: '血常规', report_date: '2026-10-01' }),
      report({ report_id: 'urine', report_type: '尿常规', report_date: '2026-10-02' }),
      report({ report_id: 'dateless', report_type: '生化', report_date: null }),
    ];
    expect(latestByCategory(reports).map((r) => r.report_id)).toEqual([
      'urine',
      'cbc-new',
      'dateless',
    ]);
  });

  it('同日期按 report_id 升序取首份', () => {
    const reports = [
      report({ report_id: 'b', report_type: '血常规', report_date: '2026-10-01' }),
      report({ report_id: 'a', report_type: '血常规', report_date: '2026-10-01' }),
    ];
    expect(latestByCategory(reports).map((r) => r.report_id)).toEqual(['a']);
  });

  it('同日期类别按 report_id 升序', () => {
    const reports = [
      report({ report_id: 'b', report_type: '血常规', report_date: '2026-10-01' }),
      report({ report_id: 'a', report_type: '尿常规', report_date: '2026-10-01' }),
    ];
    expect(latestByCategory(reports).map((r) => r.report_type)).toEqual(['尿常规', '血常规']);
  });

  it('失败报告同样参与类别最近一次', () => {
    const reports = [
      report({
        report_id: 'failed',
        report_type: '血常规',
        report_date: '2026-10-02',
        status: 'failed',
      }),
      report({ report_id: 'success', report_type: '血常规', report_date: '2026-10-01' }),
    ];
    expect(latestByCategory(reports).map((r) => r.report_id)).toEqual(['failed']);
  });

  it('100 条混合类别:每类一份且不重复', () => {
    const types = ['血常规', '尿常规', '生化', '临检', '影像', '免疫'];
    const reports = Array.from({ length: 100 }, (_, index) =>
      report({
        report_id: `r${index}`,
        report_type: types[index % types.length],
        report_date: `2026-${String((index % 12) + 1).padStart(2, '0')}-01`,
      }),
    );
    const latest = latestByCategory(reports);
    expect(latest).toHaveLength(types.length);
    expect(new Set(latest.map((r) => r.report_type)).size).toBe(types.length);
  });
});

describe('parseReportFilter', () => {
  it('识别单值', () => {
    expect(parseReportFilter('partial')).toBe('partial');
    expect(parseReportFilter('failed')).toBe('failed');
  });

  it('数组按首值解析', () => {
    expect(parseReportFilter(['failed', 'partial'])).toBe('failed');
    expect(parseReportFilter(['unknown', 'partial'])).toBeUndefined();
  });

  it('未知值与空值按无筛选', () => {
    expect(parseReportFilter('success')).toBeUndefined();
    expect(parseReportFilter(undefined)).toBeUndefined();
    expect(parseReportFilter([])).toBeUndefined();
  });
});
