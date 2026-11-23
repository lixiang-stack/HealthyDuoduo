// Health-archive URLs live here so links from report, panel, review, analysis,
// upload and trends screens cannot drift from the route shell.
import { HEALTH_PATHS } from '../navigation/domainRoutes';
import type { ReportFilter } from './overview';
import type { PanelCode } from './panels';

export const HEALTH_REPORTS_PATH = HEALTH_PATHS[0];
export const HEALTH_TRENDS_PATH = HEALTH_PATHS[1];

export function healthReportsHref(
  params: { view?: 'time' | 'panel'; filter?: ReportFilter; panel?: PanelCode } = {},
): string {
  const query: string[] = [];
  if (params.view) {
    query.push(`view=${params.view}`);
  }
  if (params.filter) {
    query.push(`filter=${params.filter}`);
  }
  if (params.panel) {
    query.push(`panel=${encodeURIComponent(params.panel)}`);
  }
  return query.length > 0 ? `${HEALTH_REPORTS_PATH}?${query.join('&')}` : HEALTH_REPORTS_PATH;
}
