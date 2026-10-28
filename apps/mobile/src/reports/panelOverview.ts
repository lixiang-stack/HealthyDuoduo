// 报告 / 项目组展示口径(panel-browse 票 02/04):卡片主标签、详情分组与异常指标页分组。
// 项目组只是浏览入口:详情按 panels 分组但始终展示报告全部指标,不按项目隐藏。
import type {
  PanelDetail,
  PanelItem,
  PanelSummary,
  ReportDetail,
  ReportItem,
  ReportSummary,
} from '../api/reports';

import { PANEL_ORDER, panelLabel, type PanelCode } from './panels';
import { isAbnormalFlag, selectSeries, type TrendSeries } from './trends';

/** 失败 / 未回填存量没有可信项目组:卡片与次要信息都回退 report_type。 */
function usesReportTypeFallback(
  report: Pick<ReportSummary, 'panels' | 'status'>,
): boolean {
  return report.status === 'failed' || report.panels.length === 0;
}

/** 时间线卡片主标签:列出报告全部项目组;失败 / 未回填存量回退 report_type。 */
export function reportPanelLabels(
  report: Pick<ReportSummary, 'panels' | 'report_type' | 'status'>,
): string {
  if (usesReportTypeFallback(report)) {
    return report.report_type;
  }
  return report.panels.map(panelLabel).join(' · ');
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

/** 报告是否属于某项目组:failed 不参与;未回填存量 panels=[] 按 [other] 处理
 *  (与项目列表把存量项归「其他」一致)。 */
export function reportHasPanel(
  report: Pick<ReportSummary, 'panels' | 'status'>,
  panel: PanelCode,
): boolean {
  if (report.status === 'failed') {
    return false;
  }
  const panels = report.panels.length > 0 ? report.panels : (['other'] as PanelCode[]);
  return panels.includes(panel);
}

/** 时间线 ?panel= 筛选:保持入参顺序,只看非 failed 且包含该项目的报告。 */
export function filterReportsByPanel(
  reports: ReportSummary[],
  panel: PanelCode | undefined,
): ReportSummary[] {
  if (panel === undefined) {
    return reports;
  }
  return reports.filter((report) => reportHasPanel(report, panel));
}

/** App 内部 ?panel= 参数解析:数组取首值,只接受合法 code,其余按无筛选。 */
export function parsePanelParam(raw: string | string[] | undefined): PanelCode | undefined {
  const value = Array.isArray(raw) ? raw[0] : raw;
  return value !== undefined && PANEL_ORDER.includes(value as PanelCode)
    ? (value as PanelCode)
    : undefined;
}

/** other 行 / 详情头部的来源报告类别:如「其他 · 临检、免疫」;非 other 不显示。 */
export function panelSourceLabel(
  panel: PanelCode,
  reportTypes: string[],
): string | undefined {
  if (panel !== 'other' || reportTypes.length === 0) {
    return undefined;
  }
  return `其他 · ${reportTypes.join('、')}`;
}

/** 项目详情项是否异常(以最近非 not_tested 点为准;未检测 / 无点不异常)。 */
export function panelItemIsAbnormal(item: Pick<PanelItem, 'latest'>): boolean {
  return item.latest !== null && isAbnormalFlag(item.latest.flag);
}

/** 项目详情分组:异常项在前,其余保持后端「最新报告内顺序」。 */
export function splitPanelItems(items: PanelItem[]): {
  abnormal: PanelItem[];
  rest: PanelItem[];
} {
  const abnormal: PanelItem[] = [];
  const rest: PanelItem[] = [];
  for (const item of items) {
    (panelItemIsAbnormal(item) ? abnormal : rest).push(item);
  }
  return { abnormal, rest };
}

export type AbnormalPanelGroup = {
  panel: PanelCode;
  reportDate: string | null;
  items: PanelItem[];
};

/** 家庭卡异常项目最多展示的项目组数;超出的折叠为「另有 N 个项目共 M 项异常」。 */
export const FAMILY_ABNORMAL_MAX = 2;

/** 家庭卡异常项目摘要:只列 abnormal_count > 0 的项目,按异常数降序、
 *  同数按 PANEL_ORDER;超过 max 个追加「另有 N 个项目共 M 项异常」
 *  (N 是隐藏的项目组数,M 是这些项目组里的异常指标总数)。 */
export function familyAbnormalLabel(
  panels: PanelSummary[],
  max = FAMILY_ABNORMAL_MAX,
): string | undefined {
  const abnormal = panels
    .filter((panel) => panel.abnormal_count > 0)
    .sort(
      (a, b) =>
        b.abnormal_count - a.abnormal_count ||
        PANEL_ORDER.indexOf(a.panel) - PANEL_ORDER.indexOf(b.panel),
    );
  if (abnormal.length === 0) {
    return undefined;
  }
  const labels = abnormal
    .slice(0, max)
    .map((panel) => `${panelLabel(panel.panel)} ${panel.abnormal_count} 项`);
  if (abnormal.length <= max) {
    return labels.join(' · ');
  }
  const hidden = abnormal.slice(max);
  const hiddenItems = hidden.reduce((sum, panel) => sum + panel.abnormal_count, 0);
  return `${labels.join(' · ')} · 另有 ${hidden.length} 个项目共 ${hiddenItems} 项异常`;
}

/** 全部异常指标数(档案「全部异常 N 项」入口与家庭卡共用口径)。 */
export function totalAbnormalCount(panels: PanelSummary[]): number {
  return panels.reduce((sum, panel) => sum + panel.abnormal_count, 0);
}

/** 有异常指标的项目组数(与异常指标总数区分:一个项目可含多项)。 */
export function abnormalPanelCount(panels: PanelSummary[]): number {
  return panels.filter((panel) => panel.abnormal_count > 0).length;
}

/** 异常指标页分组:按传入项目详情顺序(项目列表顺序),只保留 latest 异常的
 *  指标;没有异常指标的项目不输出空组。 */
export function abnormalPanelGroups(details: PanelDetail[]): AbnormalPanelGroup[] {
  const groups: AbnormalPanelGroup[] = [];
  for (const detail of details) {
    const items = splitPanelItems(detail.items).abnormal;
    if (items.length === 0) {
      continue;
    }
    groups.push({
      panel: detail.panel,
      reportDate: detail.latest_report_date,
      items,
    });
  }
  return groups;
}

export type PanelTrend = {
  name: string;
  reportType: string;
  reportDate: string | null;
  latestValue: string | null;
  series: TrendSeries;
};

/** 项目指标项 → 趋势层数据:points 已随项目详情返回,不额外请求;
 *  reportType 取该指标最近点所在报告的真实类别。 */
export function panelTrendOf(
  item: PanelItem,
  reportTypeById: Record<string, string>,
): PanelTrend | undefined {
  const latest = item.latest;
  if (!latest) {
    return undefined;
  }
  return {
    name: item.name,
    reportType: reportTypeById[latest.report_id] ?? '—',
    reportDate: latest.report_date,
    latestValue: latest.value,
    series: selectSeries(item.points, item.unit, latest.report_id),
  };
}

/** 报告新鲜度:日期降序、dateless 沉底;同日按 report_id 降序(与服务端
 *  「最近报告」口径一致)。 */
function reportIsNewer(
  report: Pick<ReportSummary, 'report_id' | 'report_date'>,
  date: string | null,
  reportId: string | null,
): boolean {
  if (report.report_date === null) {
    return date === null && (reportId === null || report.report_id > reportId);
  }
  if (date === null) {
    return true;
  }
  if (report.report_date !== date) {
    return report.report_date > date;
  }
  return reportId === null || report.report_id > reportId;
}

/** 比当前项目详情最新非 failed 报告更新的一份 failed 报告(用于「另有更新的一份
 *  识别失败报告」);没有则为 undefined。 */
export function latestFailedReportAfter(
  reports: ReportSummary[],
  latestDate: string | null,
  latestReportId: string | null,
): ReportSummary | undefined {
  let latest: ReportSummary | undefined;
  for (const report of reports) {
    if (report.status !== 'failed' || !reportIsNewer(report, latestDate, latestReportId)) {
      continue;
    }
    if (!latest || reportIsNewer(report, latest.report_date, latest.report_id)) {
      latest = report;
    }
  }
  return latest;
}
