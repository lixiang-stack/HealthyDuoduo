import { birthDateInput, memberProfileValid, normalizeBirthDate } from './profile';

describe('normalizeBirthDate', () => {
  it('day 精度要求合法 YYYY-MM-DD', () => {
    expect(normalizeBirthDate('2020-05-20', 'day')).toBe('2020-05-20');
    expect(normalizeBirthDate('2024-02-29', 'day')).toBe('2024-02-29');
    expect(normalizeBirthDate('2020-1-1', 'day')).toBeUndefined();
    expect(normalizeBirthDate('2023-02-29', 'day')).toBeUndefined();
    expect(normalizeBirthDate('2020-13-01', 'day')).toBeUndefined();
    expect(normalizeBirthDate('', 'day')).toBeUndefined();
  });

  it('year 精度只收四位年份并补 01-01', () => {
    expect(normalizeBirthDate('2020', 'year')).toBe('2020-01-01');
    expect(normalizeBirthDate(' 2020 ', 'year')).toBe('2020-01-01');
    expect(normalizeBirthDate('2020-01-01', 'year')).toBeUndefined();
    expect(normalizeBirthDate('20', 'year')).toBeUndefined();
  });
});

describe('birthDateInput / memberProfileValid', () => {
  it('year 精度只回显年份,day 回显完整日期', () => {
    expect(birthDateInput('2020-01-01', 'year')).toBe('2020');
    expect(birthDateInput('2020-05-20', 'day')).toBe('2020-05-20');
  });

  it('名字、性别、出生日期三者齐全才可提交', () => {
    expect(memberProfileValid('小虎', 'male', '2020-01-01', 'day')).toBe(true);
    expect(memberProfileValid('小虎', undefined, '2020-01-01', 'day')).toBe(false);
    expect(memberProfileValid('  ', 'male', '2020-01-01', 'day')).toBe(false);
    expect(memberProfileValid('小虎', 'male', '2020-02-30', 'day')).toBe(false);
    expect(memberProfileValid('小虎', 'male', '2020', 'year')).toBe(true);
  });
});
