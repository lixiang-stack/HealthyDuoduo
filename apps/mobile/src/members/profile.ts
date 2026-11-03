import type { Member } from '../api/members';

export type MemberSex = Member['sex'];

export const SEX_OPTIONS: readonly { value: MemberSex; label: string }[] = [
  { value: 'male', label: '男' },
  { value: 'female', label: '女' },
];

export type DateParts = { year: number; month: number; day: number };

/** 出生年份可选范围下限(再早的成员极少;避免滚轮无限长)。 */
export const BIRTH_YEAR_MIN = 1900;

export function sexLabel(sex: MemberSex): string {
  return SEX_OPTIONS.find((option) => option.value === sex)?.label ?? sex;
}

export type MemberFormErrors = {
  name?: string;
  sex?: string;
  birthDate?: string;
};

/** 表单字段级错误:创建与编辑共用同一套提示。 */
export function memberFormErrors(
  name: string,
  sex: MemberSex | undefined,
  birthDate: string,
): MemberFormErrors {
  const errors: MemberFormErrors = {};
  if (!name.trim()) {
    errors.name = '请填写称呼';
  }
  if (!sex) {
    errors.sex = '请选择性别';
  }
  if (!parseBirthDate(birthDate)) {
    errors.birthDate = '请选择出生日期';
  }
  return errors;
}

/** 成员资料是否可提交:名字非空、性别已选、出生日期合法。 */
export function memberProfileValid(
  name: string,
  sex: MemberSex | undefined,
  birthDate: string,
): boolean {
  return Object.keys(memberFormErrors(name, sex, birthDate)).length === 0;
}

/** 滚轮吸附后的下标:向上/向下越界都夹到合法范围。 */
export function wheelIndexFromOffset(offset: number, rowHeight: number, count: number): number {
  return Math.min(count - 1, Math.max(0, Math.round(offset / rowHeight)));
}

export function daysInMonth(year: number, month: number): number {
  return new Date(Date.UTC(year, month, 0)).getUTCDate();
}

export function formatBirthDate(parts: DateParts): string {
  return `${parts.year}-${String(parts.month).padStart(2, '0')}-${String(parts.day).padStart(2, '0')}`;
}

/** 解析 YYYY-MM-DD;不存在或非法日期返回 undefined。 */
export function parseBirthDate(birthDate: string): DateParts | undefined {
  const match = /^(\d{4})-(\d{2})-(\d{2})$/.exec(birthDate.trim());
  if (!match) {
    return undefined;
  }
  const parts = { year: Number(match[1]), month: Number(match[2]), day: Number(match[3]) };
  if (parts.year < BIRTH_YEAR_MIN || parts.month < 1 || parts.month > 12) {
    return undefined;
  }
  if (parts.day < 1 || parts.day > daysInMonth(parts.year, parts.month)) {
    return undefined;
  }
  return parts;
}

/** 日期滚轮的初始值:已有生日用已有值;否则给 6 年前的 1 月 1 日,减少家长滚动距离。 */
export function initialBirthDateParts(birthDate: string, now = new Date()): DateParts {
  return (
    parseBirthDate(birthDate) ?? {
      year: now.getFullYear() - 6,
      month: 1,
      day: 1,
    }
  );
}

/** 年份滚轮(新到旧)。 */
export function birthYearOptions(now = new Date()): number[] {
  const years: number[] = [];
  for (let year = now.getFullYear(); year >= BIRTH_YEAR_MIN; year -= 1) {
    years.push(year);
  }
  return years;
}
