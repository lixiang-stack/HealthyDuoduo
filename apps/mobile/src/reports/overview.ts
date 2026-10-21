// 档案筛选口径(纯逻辑,概览与档案共用):待核对 = `status === 'partial'` 且非影像单;
// 识别失败 = `status === 'failed'`。计数与筛选共用同一判定,避免两处口径漂移。
import type { ReportSummary } from '../api/reports';

import { isImagingReport } from './category';

export type ReportFilter = 'partial' | 'failed';

/** 待核对:partial 且非影像单(与卡片状态文案同口径)。 */
export function isPendingReviewReport(report: ReportSummary): boolean {
  return report.status === 'partial' && !isImagingReport(report);
}

/** 识别失败:failed 报告(影像单失败同样计入)。 */
export function isFailedReport(report: ReportSummary): boolean {
  return report.status === 'failed';
}

/** 按筛选保留子集,保持入参顺序;排序仍由调用方 sortReports 负责。 */
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
