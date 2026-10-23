import type { ReportSummary } from '../api/reports';

import { sortReports } from './order';

function report(id: string, date: string | null): ReportSummary {
  return {
    report_id: id,
    member_id: 1,
    report_type: '血常规',
    report_date: date,
    status: 'success',
    fail_reason: null,
    flag_counts: { high: 0, low: 0, abnormal: 0, unknown: 0 },
    unreviewed_low_confidence_count: 0,
  };
}

describe('sortReports', () => {
  it('日期降序;无日期(dateless)沉底不破坏其余排序', () => {
    const input = [
      report('c', null),
      report('b', '2026-10-01'),
      report('d', null),
      report('a', '2026-10-02'),
    ];
    expect(sortReports(input).map((r) => r.report_id)).toEqual(['a', 'b', 'c', 'd']);
  });

  it('同日期按 report_id 升序(与服务端 sha 升序稳定一致)', () => {
    const input = [report('b', '2026-10-01'), report('a', '2026-10-01')];
    expect(sortReports(input).map((r) => r.report_id)).toEqual(['a', 'b']);
  });

  it('不改动入参(返回新数组)', () => {
    const input = [report('b', '2026-10-01'), report('a', '2026-10-02')];
    const out = sortReports(input);
    expect(out).not.toBe(input);
    expect(input.map((r) => r.report_id)).toEqual(['b', 'a']);
  });
});
