// 报告类别展示口径(票 03):影像(超声)与病历都是叙述文档,详情以段落 + 原图为主,
// 不呈现检验单式指标项与 actionless 待核对;类别判定集中在此,避免裸字符串散落。
import type { ReportSummary } from '../api/reports';

export const IMAGING_REPORT_TYPE = '影像';
export const CLINICAL_NOTE_REPORT_TYPE = 'clinical_note';

const REPORT_TYPE_LABELS: Record<string, string> = {
  [CLINICAL_NOTE_REPORT_TYPE]: '病历',
};

/** report_type 的展示文案:中文类别原样,wire 枚举 clinical_note 显示为「病历」。 */
export function reportTypeLabel(reportType: string): string {
  return REPORT_TYPE_LABELS[reportType] ?? reportType;
}

export function isImagingReport(report: Pick<ReportSummary, 'report_type'>): boolean {
  return report.report_type === IMAGING_REPORT_TYPE;
}

export function isClinicalNoteReport(report: Pick<ReportSummary, 'report_type'>): boolean {
  return report.report_type === CLINICAL_NOTE_REPORT_TYPE;
}

/** 叙述型报告:主体是报告段落,不是指标项/核对。 */
export function isNarrativeReport(report: Pick<ReportSummary, 'report_type'>): boolean {
  return isImagingReport(report) || isClinicalNoteReport(report);
}
