import { DOMAIN_TABS } from './domainTabConfig';

describe('domain bottom tabs', () => {
  it('shows only today/stats inside daily', () => {
    expect(DOMAIN_TABS.daily.map((tab) => [tab.name, tab.title])).toEqual([
      ['today', '今天'],
      ['stats', '统计'],
    ]);
  });

  it('shows only reports/trends inside health', () => {
    expect(DOMAIN_TABS.health.map((tab) => [tab.name, tab.title])).toEqual([
      ['reports', '报告'],
      ['trends', '趋势'],
    ]);
  });

  it('does not add record, reminder or settings as bottom tabs', () => {
    const names = [...DOMAIN_TABS.daily, ...DOMAIN_TABS.health].map((tab) => tab.name);
    expect(names).not.toContain('record');
    expect(names).not.toContain('reminder');
    expect(names).not.toContain('settings');
  });
});
