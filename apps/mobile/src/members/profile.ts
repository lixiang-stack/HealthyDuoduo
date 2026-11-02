import type { Member, MemberCreate } from '../api/members';

export type MemberSex = Member['sex'];
export type BirthDatePrecision = MemberCreate['birth_date_precision'];

export const SEX_OPTIONS: readonly { value: MemberSex; label: string }[] = [
  { value: 'male', label: '男' },
  { value: 'female', label: '女' },
  { value: 'unspecified', label: '不确定' },
];

export const BIRTH_DATE_PRECISION_OPTIONS: readonly {
  value: BirthDatePrecision;
  label: string;
}[] = [
  { value: 'day', label: '按日期' },
  { value: 'year', label: '按年份' },
];

/** 归一出生日期输入:day 收 YYYY-MM-DD;year 只收四位年份并补 01-01。
 *  非法/不存在的日期返回 undefined,调用方据此禁用提交。 */
export function normalizeBirthDate(
  input: string,
  precision: BirthDatePrecision,
): string | undefined {
  const trimmed = input.trim();
  if (precision === 'year') {
    const match = /^(\d{4})$/.exec(trimmed);
    return match ? `${match[1]}-01-01` : undefined;
  }
  const match = /^(\d{4})-(\d{2})-(\d{2})$/.exec(trimmed);
  if (!match) {
    return undefined;
  }
  const year = Number(match[1]);
  const month = Number(match[2]);
  const day = Number(match[3]);
  const date = new Date(Date.UTC(year, month - 1, day));
  if (
    date.getUTCFullYear() !== year ||
    date.getUTCMonth() !== month - 1 ||
    date.getUTCDate() !== day
  ) {
    return undefined;
  }
  return trimmed;
}

/** 成员资料是否可提交:名字非空、性别已选、出生日期与精度匹配。 */
export function memberProfileValid(
  name: string,
  sex: MemberSex | undefined,
  birthDate: string,
  precision: BirthDatePrecision,
): boolean {
  return (
    name.trim().length > 0 &&
    sex !== undefined &&
    normalizeBirthDate(birthDate, precision) !== undefined
  );
}

/** 编辑表单回显:year 精度只显示四位年份,day 显示完整日期。 */
export function birthDateInput(birthDate: string, precision: BirthDatePrecision): string {
  return precision === 'year' ? birthDate.slice(0, 4) : birthDate;
}
