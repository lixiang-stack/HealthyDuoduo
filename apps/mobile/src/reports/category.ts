// 报告类别展示口径(票 03):影像(超声)单是叙述文档,详情以段落 + 原图为主,
// 不呈现检验单式指标项与 actionless 待核对;类别判定集中在此,避免裸字符串散落。
import type { ReportSummary } from '../api/reports';

export const IMAGING_REPORT_TYPE = '影像';

export function isImagingReport(report: Pick<ReportSummary, 'report_type'>): boolean {
  return report.report_type === IMAGING_REPORT_TYPE;
}
