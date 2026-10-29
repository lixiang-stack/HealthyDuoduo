// 家庭首页卡片文案(纯逻辑,spec §8.2):待核对、最近项目 + 日期 / 距今、
// 异常指标总数 + 项目数、最近报告偏高 / 偏低、报告总数。空白成员只给空文案。
import type { PanelSummary, ReportSummary } from '../api/reports';

import { pendingSummary } from './overview';
import { sortReports } from './order';
import { abnormalPanelCount, totalAbnormalCount } from './panelOverview';
import { panelLabel } from './panels';
import { relativeDateLabel } from './relativeDate';
import { reportDateLabel } from './status';

export type FamilyCardInput = {
  reports: ReportSummary[];
  panels: PanelSummary[];
};

export type FamilyCardSummary = {
  hasReports: boolean;
  /** 有待核对 / 识别失败时页面用强调色;无待处理为 false。 */
  hasPending?: boolean;
  /** 待核对 N 份 [· M 项] · 识别失败 K 份;无待处理为「没有待核对」。 */
  pending?: string;
  /** 最近项目 / 失败文案;无报告时为「这位成员还没有检查单」。 */
  latest: string;
  /** 异常指标总数 + 异常项目数;无异常为 undefined(由页面显示「没有异常项目」)。 */
  abnormal?: string;
  latestFlags?: string;
  reportCount?: string;
};

function pendingText(summary: ReturnType<typeof pendingSummary>): string {
  const parts: string[] = [];
  if (summary.partialReports > 0) {
    parts.push(
      summary.unreviewedItems > 0
        ? `待核对 ${summary.partialReports} 份 · ${summary.unreviewedItems} 项`
        : `待核对 ${summary.partialReports} 份`,
    );
  }
  if (summary.failedReports > 0) {
    parts.push(`识别失败 ${summary.failedReports} 份`);
  }
  return parts.length > 0 ? parts.join(' · ') : '没有待核对';
}

export function familyCardSummary(
  entry: FamilyCardInput,
  today: Date,
): FamilyCardSummary {
  const { reports, panels } = entry;
  const hasReports = reports.length > 0;
  if (!hasReports) {
    return { hasReports, latest: '这位成员还没有检查单' };
  }

  const latestReport = sortReports(reports)[0];
  const relative = relativeDateLabel(latestReport.report_date, today);
  const latest =
    latestReport.status === 'failed'
      ? `最近报告识别失败 · ${reportDateLabel(latestReport)}`
      : `最近：${
          latestReport.panels.length > 0
            ? panelLabel(latestReport.panels[0])
            : latestReport.report_type
        } · ${reportDateLabel(latestReport)}${relative ? ` · ${relative}` : ''}`;

  const pending = pendingSummary(reports);
  const panelCount = abnormalPanelCount(panels);
  const abnormal =
    panelCount > 0
      ? `异常指标 ${totalAbnormalCount(panels)} 项 · ${panelCount} 个项目`
      : undefined;

  return {
    hasReports,
    hasPending: pending.partialReports > 0 || pending.failedReports > 0,
    pending: pendingText(pending),
    latest,
    abnormal,
    latestFlags:
      latestReport.status === 'failed'
        ? undefined
        : `最近报告 偏高 ${latestReport.flag_counts.high} · 偏低 ${latestReport.flag_counts.low}`,
    reportCount: `报告 ${reports.length} 份`,
  };
}
