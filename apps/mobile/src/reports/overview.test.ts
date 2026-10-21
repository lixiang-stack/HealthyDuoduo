import type { ReportSummary } from '../api/reports';

import { filterReports, parseReportFilter, type ReportFilter } from './overview';

function report(overrides: Partial<ReportSummary> = {}): ReportSummary {
  return {
    report_id: 'report',
    member_id: 1,
    report_type: '血常规',
    report_date: '2026-10-01',
    status: 'success',
    fail_reason: null,
    flag_counts: { high: 0, low: 0, unknown: 0 },
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
