// 家庭卡相对日期文案(纯逻辑):今天 / 昨天 / N 天前 / N 周前 / N 个月前 / N 年前。
// 只比较日历日(UTC 拆分年月日),不引入时区 / 夏令时误差;未来与非法日期不给文案。
const MS_PER_DAY = 24 * 60 * 60 * 1000;
const DAYS_PER_WEEK = 7;
const DAYS_PER_MONTH = 30;
const DAYS_PER_YEAR = 365;

function calendarDayUtc(value: Date): number {
  return Date.UTC(value.getFullYear(), value.getMonth(), value.getDate());
}

function parseDateOnly(value: string): number | undefined {
  const match = /^(\d{4})-(\d{2})-(\d{2})$/.exec(value);
  if (!match) {
    return undefined;
  }
  const year = Number(match[1]);
  const month = Number(match[2]);
  const day = Number(match[3]);
  const timestamp = Date.UTC(year, month - 1, day);
  const parsed = new Date(timestamp);
  if (
    parsed.getUTCFullYear() !== year ||
    parsed.getUTCMonth() !== month - 1 ||
    parsed.getUTCDate() !== day
  ) {
    return undefined;
  }
  return timestamp;
}

export function relativeDateLabel(date: string | null, today: Date): string | undefined {
  if (date === null) {
    return undefined;
  }
  const timestamp = parseDateOnly(date);
  if (timestamp === undefined) {
    return undefined;
  }
  const diffDays = Math.floor((calendarDayUtc(today) - timestamp) / MS_PER_DAY);
  if (diffDays < 0) {
    return undefined;
  }
  if (diffDays === 0) {
    return '今天';
  }
  if (diffDays === 1) {
    return '昨天';
  }
  if (diffDays < DAYS_PER_WEEK) {
    return `${diffDays} 天前`;
  }
  if (diffDays < DAYS_PER_MONTH) {
    return `${Math.floor(diffDays / DAYS_PER_WEEK)} 周前`;
  }
  if (diffDays < DAYS_PER_YEAR) {
    return `${Math.floor(diffDays / DAYS_PER_MONTH)} 个月前`;
  }
  return `${Math.floor(diffDays / DAYS_PER_YEAR)} 年前`;
}
