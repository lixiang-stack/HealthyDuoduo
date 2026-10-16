// 时间线展示口径(纯逻辑):日期降序、无日期(dateless)沉底——空串比较天然垫底,
// 不破坏其余排序;同日期按 report_id 升序(与服务端 image_sha256 升序稳定一致)。
import type { ReportSummary } from '../api/reports';

export function sortReports(reports: ReportSummary[]): ReportSummary[] {
  return [...reports].sort((a, b) => {
    const dateA = a.report_date ?? '';
    const dateB = b.report_date ?? '';
    if (dateA !== dateB) {
      return dateA < dateB ? 1 : -1;
    }
    if (a.report_id === b.report_id) {
      return 0;
    }
    return a.report_id < b.report_id ? -1 : 1;
  });
}
