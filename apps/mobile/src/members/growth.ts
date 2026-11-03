// 生长记录表单规则与展示文案(llm-analysis 票 02):测量日期 + 身高 / 体重(至少一项)
// + 来源。日期与数字校验放纯函数里,表单组件只负责呈现与提交。
import type { GrowthRecord, GrowthSource } from '../api/growth';
import { parseBirthDate } from './profile';

export const GROWTH_SOURCE_OPTIONS: readonly { value: GrowthSource; label: string }[] = [
  { value: 'measured', label: '实测' },
  { value: 'report', label: '体检单' },
];

export function growthSourceLabel(source: GrowthSource): string {
  return GROWTH_SOURCE_OPTIONS.find((option) => option.value === source)?.label ?? source;
}

export type GrowthDraft = {
  measuredOn: string;
  height: string;
  weight: string;
  source: GrowthSource;
};

export type GrowthFormErrors = {
  measuredOn?: string;
  measurement?: string;
  height?: string;
  weight?: string;
};

/** 合法公历日期;未来日期不可作为测量日期。复用成员生日的日期解析规则。 */
export function validMeasuredOn(value: string, now = new Date()): boolean {
  const parts = parseBirthDate(value);
  if (!parts) {
    return false;
  }
  const date = Date.UTC(parts.year, parts.month - 1, parts.day);
  const today = Date.UTC(now.getFullYear(), now.getMonth(), now.getDate());
  return date <= today;
}

/** 正数输入;空串返回 undefined(身高 / 体重可单填)。非法值返回 NaN。 */
export function parsePositiveMeasurement(value: string): number | undefined {
  const trimmed = value.trim();
  if (trimmed === '') {
    return undefined;
  }
  const parsed = Number(trimmed);
  if (!Number.isFinite(parsed)) {
    return Number.NaN;
  }
  return parsed;
}

export function growthFormErrors(draft: GrowthDraft, now = new Date()): GrowthFormErrors {
  const errors: GrowthFormErrors = {};
  if (!validMeasuredOn(draft.measuredOn, now)) {
    errors.measuredOn = '请填写有效的测量日期（不晚于今天）';
  }
  const height = parsePositiveMeasurement(draft.height);
  const weight = parsePositiveMeasurement(draft.weight);
  if (height === undefined && weight === undefined) {
    errors.measurement = '身高和体重至少填一项';
  }
  if (height !== undefined && (!Number.isFinite(height) || height <= 0)) {
    errors.height = '身高需为正数';
  }
  if (weight !== undefined && (!Number.isFinite(weight) || weight <= 0)) {
    errors.weight = '体重需为正数';
  }
  return errors;
}

/** 提交体:空串不发送,避免把「没填」写成 0。 */
export function growthCreateBody(draft: GrowthDraft) {
  const height = parsePositiveMeasurement(draft.height);
  const weight = parsePositiveMeasurement(draft.weight);
  return {
    measured_on: draft.measuredOn.trim(),
    ...(height === undefined ? {} : { height_cm: height }),
    ...(weight === undefined ? {} : { weight_kg: weight }),
    source: draft.source,
  };
}

export function formatBMI(bmi: number | null): string | undefined {
  return bmi === null ? undefined : bmi.toFixed(1);
}

/** 本地日历日 YYYY-MM-DD(测量日期默认值)。 */
export function todayISO(now = new Date()): string {
  const year = now.getFullYear();
  const month = String(now.getMonth() + 1).padStart(2, '0');
  const day = String(now.getDate()).padStart(2, '0');
  return `${year}-${month}-${day}`;
}

/** 成员列表的一行摘要:来源 + 有值的测量项 + BMI + 日期。 */
export function growthSummary(record: GrowthRecord): string {
  const parts: string[] = [growthSourceLabel(record.source)];
  if (record.height_cm !== null) {
    parts.push(`身高 ${record.height_cm} cm`);
  }
  if (record.weight_kg !== null) {
    parts.push(`体重 ${record.weight_kg} kg`);
  }
  const bmi = formatBMI(record.bmi);
  if (bmi !== undefined) {
    parts.push(`BMI ${bmi}`);
  }
  parts.push(record.measured_on);
  return parts.join(' · ');
}
