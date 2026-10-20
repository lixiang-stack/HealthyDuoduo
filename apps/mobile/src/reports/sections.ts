// 报告段落展示口径(票 02):检查项目归入报告头部,检查所见 / 超声提示按
// sections 数组序在指标项上方成卡;未知 kind 不渲染,exam_name 不重复成卡。
import type { ReportSection } from '../api/reports';

export const EXAM_NAME_KIND = 'exam_name';

/** 段落 kind → 展示标签(当前契约已知值)。 */
export const SECTION_LABELS: Record<string, string> = {
  [EXAM_NAME_KIND]: '检查项目',
  findings: '检查所见',
  impression: '超声提示',
};

export type ReportSectionSlot = {
  /** sections 数组下标,作同类多次出现时的唯一键。 */
  index: number;
  label: string;
  section: ReportSection;
};

/** 头部检查项目:同类多次出现按数组序换行并列(不丢段落);缺省时 undefined。 */
export function examNameText(sections: ReportSection[] | null | undefined): string | undefined {
  const names = (sections ?? [])
    .filter((section) => section.kind === EXAM_NAME_KIND)
    .map((section) => section.text);
  return names.length > 0 ? names.join('\n') : undefined;
}

/** 指标项上方段落卡:跳过已进头部的 exam_name 与未知 kind,保留原数组顺序。 */
export function reportSectionCards(
  sections: ReportSection[] | null | undefined,
): ReportSectionSlot[] {
  return (sections ?? []).flatMap((section, index) => {
    const label = SECTION_LABELS[section.kind];
    return label && section.kind !== EXAM_NAME_KIND ? [{ index, label, section }] : [];
  });
}
