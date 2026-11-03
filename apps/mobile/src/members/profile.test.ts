import {
  daysInMonth,
  formatBirthDate,
  initialBirthDateParts,
  memberFormErrors,
  memberProfileValid,
  parseBirthDate,
  sexLabel,
  wheelIndexFromOffset,
} from './profile';

describe('parseBirthDate / formatBirthDate', () => {
  it('只接受合法 YYYY-MM-DD,含闰年与月末校验', () => {
    expect(parseBirthDate('2020-05-20')).toEqual({ year: 2020, month: 5, day: 20 });
    expect(parseBirthDate('2024-02-29')).toEqual({ year: 2024, month: 2, day: 29 });
    expect(parseBirthDate('2023-02-29')).toBeUndefined();
    expect(parseBirthDate('2020-13-01')).toBeUndefined();
    expect(parseBirthDate('2020-1-1')).toBeUndefined();
    expect(parseBirthDate('1899-01-01')).toBeUndefined();
    expect(formatBirthDate({ year: 2020, month: 5, day: 3 })).toBe('2020-05-03');
  });

  it('daysInMonth 处理闰年', () => {
    expect(daysInMonth(2024, 2)).toBe(29);
    expect(daysInMonth(2023, 2)).toBe(28);
    expect(daysInMonth(2026, 4)).toBe(30);
  });
});

describe('wheelIndexFromOffset', () => {
  it('按行高四舍五入并夹住边界', () => {
    expect(wheelIndexFromOffset(0, 44, 12)).toBe(0);
    expect(wheelIndexFromOffset(4 * 44, 44, 12)).toBe(4);
    expect(wheelIndexFromOffset(4 * 44 + 23, 44, 12)).toBe(5);
    expect(wheelIndexFromOffset(-80, 44, 12)).toBe(0);
    expect(wheelIndexFromOffset(9999, 44, 12)).toBe(11);
  });
});

describe('initialBirthDateParts', () => {
  it('已有生日时沿用,否则给 6 年前的 1 月 1 日', () => {
    const now = new Date('2026-11-02T00:00:00Z');
    expect(initialBirthDateParts('2020-05-20', now)).toEqual({ year: 2020, month: 5, day: 20 });
    expect(initialBirthDateParts('', now)).toEqual({ year: 2020, month: 1, day: 1 });
  });
});

describe('memberProfileValid / sexLabel', () => {
  it('名字、性别、出生日期三者齐全才可提交', () => {
    expect(memberProfileValid('小虎', 'male', '2020-01-01')).toBe(true);
    expect(memberProfileValid('小虎', undefined, '2020-01-01')).toBe(false);
    expect(memberProfileValid('  ', 'male', '2020-01-01')).toBe(false);
    expect(memberProfileValid('小虎', 'male', '2020-02-30')).toBe(false);
  });

  it('字段级错误指明缺哪一项', () => {
    expect(memberFormErrors('', undefined, '')).toEqual({
      name: '请填写称呼',
      sex: '请选择性别',
      birthDate: '请选择出生日期',
    });
    expect(memberFormErrors('小虎', 'female', '2020-01-01')).toEqual({});
  });

  it('性别只有男女两个标签', () => {
    expect(sexLabel('male')).toBe('男');
    expect(sexLabel('female')).toBe('女');
  });
});
