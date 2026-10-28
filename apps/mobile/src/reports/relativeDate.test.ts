import { relativeDateLabel } from './relativeDate';

describe('relativeDateLabel', () => {
  const today = new Date(2026, 9, 28);

  it('今天 / 昨天 / N 天前', () => {
    expect(relativeDateLabel('2026-10-28', today)).toBe('今天');
    expect(relativeDateLabel('2026-10-27', today)).toBe('昨天');
    expect(relativeDateLabel('2026-10-22', today)).toBe('6 天前');
  });

  it('N 周前 / N 个月前 / N 年前', () => {
    expect(relativeDateLabel('2026-10-21', today)).toBe('1 周前');
    expect(relativeDateLabel('2026-09-29', today)).toBe('4 周前');
    expect(relativeDateLabel('2026-09-28', today)).toBe('1 个月前');
    expect(relativeDateLabel('2025-10-28', today)).toBe('1 年前');
    expect(relativeDateLabel('2024-08-19', today)).toBe('2 年前');
  });

  it('null / 未来 / 非法日期返回 undefined', () => {
    expect(relativeDateLabel(null, today)).toBeUndefined();
    expect(relativeDateLabel('2026-10-29', today)).toBeUndefined();
    expect(relativeDateLabel('2026-13-01', today)).toBeUndefined();
    expect(relativeDateLabel('not-a-date', today)).toBeUndefined();
  });
});
