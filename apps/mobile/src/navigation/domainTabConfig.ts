import type { Domain } from './domainRoutes';
import { ReportsIcon, StatsIcon, TodayIcon, TrendsIcon, type DomainTabIcon } from './tabIcons';

export type DomainTabConfig = {
  name: string;
  title: string;
  Icon: DomainTabIcon;
};

/** Bottom tabs are contextual: only the active domain's two secondary views appear. */
export const DOMAIN_TABS: Record<Domain, readonly DomainTabConfig[]> = {
  daily: [
    { name: 'today', title: '今天', Icon: TodayIcon },
    { name: 'stats', title: '统计', Icon: StatsIcon },
  ],
  health: [
    { name: 'reports', title: '报告', Icon: ReportsIcon },
    { name: 'trends', title: '趋势', Icon: TrendsIcon },
  ],
};
