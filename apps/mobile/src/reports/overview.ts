// 报告口径与概览聚合(纯逻辑,概览与档案共用):待核对 = `status === 'partial'`、
// 或检验单含可操作未知项,且非影像单;识别失败 = `status === 'failed'`。
// 计数与筛选共用同一判定,避免两处口径漂移。
import type { ReportSummary } from '../api/reports';

import { isImagingReport } from './category';
import { sortReports } from './order';

export type ReportFilter = 'partial' | 'failed';

export type PendingSummary = {
  partialReports: number;
  unreviewedItems: number;
  failedReports: number;
};

/** 待核对:partial 或 success+可操作未知,且非影像单(与卡片状态文案同口径)。 */
export function isPendingReviewReport(report: ReportSummary): boolean {
  return (
    !isImagingReport(report) &&
    (report.status === 'partial' || report.flag_counts.unknown > 0)
  );
}

/** 识别失败:failed 报告(影像单失败同样计入)。 */
export function isFailedReport(report: ReportSummary): boolean {
  return report.status === 'failed';
}

/** 待处理计数:待核对份数、这些报告的未核对低置信项之和、识别失败份数。
 *  份数含 success+可操作未知;项数仍只统计低置信,完整待核对项以核对页为准。 */
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

/** 每个报告类别取日期最新的一份(内部复用 sortReports);类别顺序按该份日期降序(无日期沉底)。 */
export function latestByCategory(reports: ReportSummary[]): ReportSummary[] {
  const seen = new Set<string>();
  const latest: ReportSummary[] = [];
  for (const report of sortReports(reports)) {
    if (!seen.has(report.report_type)) {
      seen.add(report.report_type);
      latest.push(report);
    }
  }
  return latest;
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
