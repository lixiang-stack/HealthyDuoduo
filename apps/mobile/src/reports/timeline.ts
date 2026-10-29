// 档案按时间线展示的纯逻辑:报告按天分组,同一天多份报告共用一个时间点;
// 日期未知的报告单独沉底,不进入时间轴。调用方先按日期降序传入,本函数保持入参顺序。
import type { ReportSummary } from '../api/reports';

export type TimelineDay = {
  date: string;
  reports: ReportSummary[];
};

export type TimelineMonth = {
  /** YYYY-MM,用于 key / testID。 */
  key: string;
  year: string;
  month: number;
  days: TimelineDay[];
};

export type TimelineGroups = {
  /** 按入参顺序(日期降序);每个月的天也按入参顺序。 */
  months: TimelineMonth[];
  /** report_date=null 的报告,放在时间轴之后。 */
  undated: ReportSummary[];
};

export function groupReportsByDay(reports: ReportSummary[]): TimelineGroups {
  const months: TimelineMonth[] = [];
  const undated: ReportSummary[] = [];
  let currentMonth: TimelineMonth | undefined;
  let currentDay: TimelineDay | undefined;

  for (const report of reports) {
    const date = report.report_date;
    if (!date) {
      undated.push(report);
      continue;
    }
    const key = date.slice(0, 7);
    if (currentMonth?.key !== key) {
      currentMonth = {
        key,
        year: date.slice(0, 4),
        month: Number(date.slice(5, 7)),
        days: [],
      };
      months.push(currentMonth);
      currentDay = undefined;
    }
    if (currentDay?.date !== date) {
      currentDay = { date, reports: [] };
      currentMonth.days.push(currentDay);
    }
    currentDay.reports.push(report);
  }

  return { months, undated };
}

const WEEKDAYS = ['周日', '周一', '周二', '周三', '周四', '周五', '周六'] as const;

/** 固定按 UTC 取星期,避免设备时区把日期前后移动一天。 */
export function weekdayLabel(date: string): string {
  const [year, month, day] = date.split('-').map(Number);
  const weekday = new Date(Date.UTC(year, month - 1, day)).getUTCDay();
  return WEEKDAYS[weekday] ?? '';
}

/** 时间轴日期列只显示日;年和月在月份分组标题。 */
export function dayOfMonthLabel(date: string): string {
  const day = date.slice(8, 10);
  return /^\d{2}$/.test(day) ? String(Number(day)) : date;
}
