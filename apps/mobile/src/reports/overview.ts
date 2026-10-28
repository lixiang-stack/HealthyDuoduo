// 报告口径与概览聚合(纯逻辑,概览与档案共用):待核对 = `status === 'partial'`、
// 检验单含可操作未知项,或含未核对低置信项,且非影像单;识别失败 = `status === 'failed'`。
// 计数与筛选共用同一判定,避免两处口径漂移。
import type { ReportSummary } from '../api/reports';

import { isImagingReport } from './category';

export type ReportFilter = 'partial' | 'failed';

export type PendingSummary = {
  partialReports: number;
  unreviewedItems: number;
  failedReports: number;
};

/** 待核对:partial、success+可操作未知或 success+未核对低置信,且非影像单
 *  (与卡片状态文案同口径)。 */
export function isPendingReviewReport(report: ReportSummary): boolean {
  return (
    !isImagingReport(report) &&
    (report.status === 'partial' ||
      report.flag_counts.unknown > 0 ||
      report.unreviewed_low_confidence_count > 0)
  );
}

/** 识别失败:failed 报告(影像单失败同样计入)。 */
export function isFailedReport(report: ReportSummary): boolean {
  return report.status === 'failed';
}

/** 待处理计数:待核对份数、这些报告的未核对低置信项之和、识别失败份数。
 *  份数含 success+可操作未知/未核对低置信;项数只统计低置信,可操作未知以卡片「未知 N」展示。 */
export function pendingSummary(reports: ReportSummary[]): PendingSummary {
  const pending = reports.filter(isPendingReviewReport);
  return {
    partialReports: pending.length,
    unreviewedItems: pending.reduce(
      (sum, report) => sum + report.unreviewed_low_confidence_count,
      0,
    ),
    failedReports: reports.filter(isFailedReport).length,
  };
}

/** 按筛选保留子集,保持入参顺序。 */
export function filterReports(
  reports: ReportSummary[],
  filter: ReportFilter | undefined,
): ReportSummary[] {
  if (filter === 'partial') {
    return reports.filter(isPendingReviewReport);
  }
  if (filter === 'failed') {
    return reports.filter(isFailedReport);
  }
  return reports;
}

/** App 内部 URL 参数解析:数组取首值,只识别 partial / failed,其余按无筛选。 */
export function parseReportFilter(raw: string | string[] | undefined): ReportFilter | undefined {
  const value = Array.isArray(raw) ? raw[0] : raw;
  return value === 'partial' || value === 'failed' ? value : undefined;
}
