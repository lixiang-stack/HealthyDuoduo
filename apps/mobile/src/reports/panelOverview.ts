// 报告级项目组展示(panel-browse 票 02):卡片主标签与详情分组。
// 项目组只是浏览入口:详情按 panels 分组但始终展示报告全部指标,不按项目隐藏。
import type { ReportDetail, ReportItem, ReportSummary } from '../api/reports';

import { panelLabel, type PanelCode } from './panels';
import { isAbnormalFlag } from './trends';

/** 失败 / 未回填存量没有可信项目组:卡片与次要信息都回退 report_type。 */
function usesReportTypeFallback(
  report: Pick<ReportSummary, 'panels' | 'status'>,
): boolean {
  return report.status === 'failed' || report.panels.length === 0;
}

/** 时间线卡片主标签:前 max 个项目组 + `+N`;失败 / 未回填存量回退 report_type。 */
export function reportPanelLabels(
  report: Pick<ReportSummary, 'panels' | 'report_type' | 'status'>,
  max = 2,
): string {
  if (usesReportTypeFallback(report)) {
    return report.report_type;
  }
  const labels = report.panels.map(panelLabel);
  if (labels.length <= max) {
    return labels.join(' · ');
  }
  return `${labels.slice(0, max).join(' · ')} +${labels.length - max}`;
}

/** 卡片次要信息:项目组成为主标签后,report_type 降到日期行;与主标签相同则不重复。 */
export function reportTypeNote(
  report: Pick<ReportSummary, 'panels' | 'report_type' | 'status'>,
): string | undefined {
  if (usesReportTypeFallback(report)) {
    return undefined;
  }
  return report.report_type === panelLabel(report.panels[0]) ? undefined : report.report_type;
}

export type ReportItemGroup = {
  /** null = 未回填存量按 report_type 单组回退。 */
  panel: PanelCode | null;
  label: string;
  items: ReportItem[];
};

/** 详情分组:按 report.panels 顺序;不在 panels 里的指标项归末尾「其他」组;
 *  未回填存量(panels=[])按 report_type 单组,避免空标题。 */
export function reportItemGroups(
  report: Pick<ReportDetail, 'panels' | 'report_type' | 'items'>,
): ReportItemGroup[] {
  if (report.items.length === 0) {
    return [];
  }
  if (report.panels.length === 0) {
    return [{ panel: null, label: report.report_type, items: report.items }];
  }
  const byPanel = new Map<PanelCode, ReportItem[]>();
  for (const item of report.items) {
    const items = byPanel.get(item.panel) ?? [];
    items.push(item);
    byPanel.set(item.panel, items);
  }
  const groups: ReportItemGroup[] = [];
  for (const panel of report.panels) {
    const items = byPanel.get(panel);
    if (items && items.length > 0) {
      groups.push({ panel, label: panelLabel(panel), items });
    }
  }
  const known = new Set<PanelCode>(report.panels);
  const leftovers = report.items.filter((item) => !known.has(item.panel));
  if (leftovers.length > 0) {
    const other = groups.find((group) => group.panel === 'other');
    if (other) {
      other.items.push(...leftovers);
    } else {
      groups.push({ panel: 'other', label: panelLabel('other'), items: leftovers });
    }
  }
  return groups;
}

/** 组内异常项数(high/low/abnormal;unknown / not_tested 不计,与 PanelSummary 同口径)。 */
export function abnormalItemCount(items: ReportItem[]): number {
  return items.filter((item) => isAbnormalFlag(item.flag)).length;
}
