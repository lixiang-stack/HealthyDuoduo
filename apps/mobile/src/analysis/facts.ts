// 分析结果页展示口径(llm-analysis 票 04):事实层只陈述数据与病历原文,
// 不生成医学判断。所有文案在此集中,页面只负责排版。
import type {
  AnalysisEvidencePoint,
  AnalysisExclusion,
  AnalysisFacts,
  AnalysisHistory,
} from '../api/analyses';
import { panelLabel, type PanelCode } from '../reports/panels';
import { SECTION_LABELS } from '../reports/sections';
import { flagLabel, refRangeLabel } from '../reports/status';

export const INSUFFICIENT_EVIDENCE_TEXT = '证据不足，请先核对';
export const NO_DEVIATION_TEXT = '本次没有偏离参考范围的指标。';
export const NO_HISTORY_TEXT = '历史数据较少，暂时无法比较变化。';
export const NO_NOTES_TEXT = '本次窗口内没有可引用的病历诊断或处理。';

const EXCLUSION_REASON_LABELS: Record<string, string> = {
  cross_member: '其他成员',
  wrong_panel: '不属于本项目',
  failed_report: '识别失败的报告',
  narrative_report: '叙述型报告',
  no_date: '没有日期',
  not_tested: '未检测',
  unverified_unit: '单位未核实',
  low_confidence: '未核对低置信',
  missing_value: '没有数值',
  unit_mismatch: '单位不同，未与本次趋势合并',
  outside_window: '早于分析窗口',
  point_cap: '超出每项最多保留点数',
  no_name: '没有指标名',
  no_note_body: '病历没有诊断或处理原文',
};

export function exclusionReasonLabel(reason: string): string {
  return EXCLUSION_REASON_LABELS[reason] ?? reason;
}

export type ExclusionGroup = {
  reason: string;
  label: string;
  count: number;
  items: AnalysisExclusion[];
};

/** 未纳入项按原因聚合;结果页给原因、数量与最多 3 个名称,不逐条淹没家长。 */
export function exclusionGroups(excluded: AnalysisExclusion[]): ExclusionGroup[] {
  const groups = new Map<string, AnalysisExclusion[]>();
  for (const item of excluded) {
    const items = groups.get(item.reason) ?? [];
    items.push(item);
    groups.set(item.reason, items);
  }
  return [...groups.entries()]
    .map(([reason, items]) => ({
      reason,
      label: exclusionReasonLabel(reason),
      count: items.length,
      items,
    }))
    .sort((a, b) => b.count - a.count || a.label.localeCompare(b.label));
}

/** 排除项名称摘要:最多 3 个,剩余用「等」;没有名称时只给日期。 */
export function exclusionItemsText(group: ExclusionGroup): string {
  const names = group.items
    .map((item) => item.name ?? item.report_date ?? item.trend_key)
    .filter((value): value is string => Boolean(value));
  const shown = names.slice(0, 3).join('、');
  return names.length > 3 ? `${shown} 等` : shown;
}

export function windowText(facts: AnalysisFacts): string {
  const base = `${facts.window.from} 至 ${facts.window.to}`;
  return facts.window.sparse ? `${base}（部分指标历史数据较少）` : base;
}

export function scopeText(facts: AnalysisFacts): string {
  const parts = [`纳入 ${facts.reports.length} 份报告`];
  if (facts.notes.length > 0) {
    parts.push(`${facts.notes.length} 份病历`);
  }
  return parts.join('、');
}

export function entryNotice(facts: AnalysisFacts): string | undefined {
  if (!facts.expanded_from_indicator) {
    return undefined;
  }
  const name = facts.indicator_name ?? '单个指标';
  return `已从指标「${name}」扩展至项目「${panelLabel(facts.panel as PanelCode)}」`;
}

export function evidenceValue(point: AnalysisEvidencePoint): string {
  if (point.value === null) {
    return '—';
  }
  const unit = point.unit_display ?? point.unit;
  return unit ? `${point.value} ${unit}` : point.value;
}

export function evidenceFlag(point: AnalysisEvidencePoint): string {
  return flagLabel(point.flag as Parameters<typeof flagLabel>[0]);
}

export function evidenceRef(point: AnalysisEvidencePoint): string {
  return refRangeLabel(point.ref_range);
}

const CHANGE_LABELS: Record<AnalysisHistory['change'], string> = {
  up: '较上次升高',
  down: '较上次降低',
  flat: '较上次持平',
  unknown: '无法比较变化',
};

export function changeLabel(change: AnalysisHistory['change']): string {
  return CHANGE_LABELS[change];
}

/** 历史比较摘要:最近值 + 变化方向 + 窗口内最高/最低。 */
export function historySummary(history: AnalysisHistory): string {
  const parts = [changeLabel(history.change)];
  if (history.highest && history.point_count > 1) {
    parts.push('本次为窗口内最高');
  }
  if (history.lowest && history.point_count > 1) {
    parts.push('本次为窗口内最低');
  }
  if (history.sparse) {
    parts.push('历史数据较少');
  }
  return parts.join(' · ');
}

export function noteSectionLabel(kind: string): string {
  return SECTION_LABELS[kind] ?? kind;
}

const UNCERTAIN_NOTE_WORDS = ['考虑', '可能'];

/** 病历原文含「考虑 / 可能」时加引号,避免读成系统自己的断言。 */
export function quoteNoteText(text: string): string {
  return UNCERTAIN_NOTE_WORDS.some((word) => text.includes(word)) ? `「${text}」` : text;
}

export function noteDate(date: string | null): string {
  return date ?? '日期未知';
}
