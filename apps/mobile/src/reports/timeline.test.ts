import type { ReportSummary } from '../api/reports';

import { dayOfMonthLabel, groupReportsByDay, weekdayLabel } from './timeline';

function report(reportId: string, reportDate: string | null): ReportSummary {
  return {
    report_id: reportId,
    member_id: 1,
    report_type: '生化',
    report_date: reportDate,
    status: 'success',
    fail_reason: null,
    flag_counts: { high: 0, low: 0, abnormal: 0, unknown: 0 },
    panels: ['liver'],
    not_tested_count: 0,
    unreviewed_low_confidence_count: 0,
  };
}

describe('groupReportsByDay', () => {
  it('按月和天分组,同一天多份报告共用一个时间点,保持入参顺序', () => {
    const reports = [
      report('r1', '2026-10-28'),
      report('r2', '2026-10-28'),
      report('r3', '2026-10-12'),
      report('r4', '2026-09-30'),
    ];
    const groups = groupReportsByDay(reports);

    expect(groups.months.map((month) => month.key)).toEqual(['2026-10', '2026-09']);
    expect(groups.months[0].year).toBe('2026');
    expect(groups.months[0].month).toBe(10);
    expect(groups.months[0].days.map((day) => day.date)).toEqual([
      '2026-10-28',
      '2026-10-12',
    ]);
    expect(groups.months[0].days[0].reports.map((item) => item.report_id)).toEqual([
      'r1',
      'r2',
    ]);
    expect(groups.months[1].days[0].reports).toHaveLength(1);
    expect(groups.undated).toEqual([]);
  });

  it('日期未知的报告沉底,不进入月份 / 天分组', () => {
    const groups = groupReportsByDay([
      report('r1', '2026-10-28'),
      report('r2', null),
      report('r3', null),
    ]);

    expect(groups.months).toHaveLength(1);
    expect(groups.months[0].days[0].reports.map((item) => item.report_id)).toEqual(['r1']);
    expect(groups.undated.map((item) => item.report_id)).toEqual(['r2', 'r3']);
  });

  it('空列表返回空分组', () => {
    expect(groupReportsByDay([])).toEqual({ months: [], undated: [] });
  });
});

describe('weekdayLabel / dayOfMonthLabel', () => {
  it('按 UTC 取星期,不随设备时区漂移', () => {
    expect(weekdayLabel('2024-01-01')).toBe('周一');
    expect(weekdayLabel('2026-10-29')).toBe('周四');
  });

  it('日期列去前导零;非法日回落原字符串', () => {
    expect(dayOfMonthLabel('2026-10-08')).toBe('8');
    expect(dayOfMonthLabel('2026-10-28')).toBe('28');
    expect(dayOfMonthLabel('bad')).toBe('bad');
  });
});
