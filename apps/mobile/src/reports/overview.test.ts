import type { ReportSummary } from '../api/reports';

import {
  filterReports,
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
    panels: ['cbc'],
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
    report({
      report_id: 'success-unknown',
      status: 'success',
      flag_counts: { high: 0, low: 0, abnormal: 0, unknown: 1 },
    }),
    report({
      report_id: 'success-low',
      status: 'success',
      unreviewed_low_confidence_count: 1,
    }),
    report({ report_id: 'imaging-partial', status: 'partial', report_type: '影像' }),
  ];

  it('partial 与 success 待核对项只保留待核对的非影像报告', () => {
    expect(filterReports(reports, 'partial').map((r) => r.report_id)).toEqual([
      'partial',
      'success-unknown',
      'success-low',
    ]);
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
      'success-unknown',
      'success-low',
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
      'success-unknown',
      'success-low',
      'imaging-partial',
    ]);
  });
});

describe('pendingSummary', () => {
  it('partial 与 success+可操作未知计份数;未核对项之和、影像单与 failed 不计入', () => {
    const reports = [
      report({ report_id: 'p1', status: 'partial', unreviewed_low_confidence_count: 2 }),
      report({ report_id: 'p2', status: 'partial', unreviewed_low_confidence_count: 0 }),
      report({
        report_id: 'success-unknown',
        status: 'success',
        flag_counts: { high: 0, low: 0, abnormal: 0, unknown: 1 },
      }),
      report({
        report_id: 'success-low',
        status: 'success',
        unreviewed_low_confidence_count: 1,
      }),
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
      partialReports: 4,
      unreviewedItems: 3,
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
