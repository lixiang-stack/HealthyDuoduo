import {
  DEFAULT_DOMAIN_NAVIGATION,
  domainOfPath,
  domainTabTarget,
  rememberDomainPath,
  resolveEntryPath,
} from './domainRoutes';

describe('domainOfPath', () => {
  it('recognizes only the four domain views', () => {
    expect(domainOfPath('/daily/today')).toBe('daily');
    expect(domainOfPath('/daily/stats')).toBe('daily');
    expect(domainOfPath('/health/reports')).toBe('health');
    expect(domainOfPath('/health/trends')).toBe('health');
    expect(domainOfPath('/report/r1')).toBeUndefined();
    expect(domainOfPath('/')).toBeUndefined();
  });
});

describe('rememberDomainPath', () => {
  it('remembers the last secondary view per domain', () => {
    const daily = rememberDomainPath(DEFAULT_DOMAIN_NAVIGATION, '/daily/stats');
    const health = rememberDomainPath(daily, '/health/trends');

    expect(health).toEqual({
      lastDomain: 'health',
      lastPaths: { daily: '/daily/stats', health: '/health/trends' },
    });
  });

  it('ignores stack routes and unchanged paths', () => {
    const state = rememberDomainPath(DEFAULT_DOMAIN_NAVIGATION, '/health/reports');
    expect(rememberDomainPath(state, '/report/r1')).toBe(state);
    expect(rememberDomainPath(state, '/health/reports')).toBe(state);
  });
});

describe('domainTabTarget', () => {
  it('returns the domain root when pressing the active domain', () => {
    expect(domainTabTarget(DEFAULT_DOMAIN_NAVIGATION, 'daily', 'daily')).toBe('/daily/today');
    expect(domainTabTarget(DEFAULT_DOMAIN_NAVIGATION, 'health', 'health')).toBe('/health/reports');
  });

  it('returns the remembered secondary view when switching domains', () => {
    const state = rememberDomainPath(
      rememberDomainPath(DEFAULT_DOMAIN_NAVIGATION, '/daily/stats'),
      '/health/trends',
    );

    expect(domainTabTarget(state, 'health', 'daily')).toBe('/daily/stats');
    expect(domainTabTarget(state, 'daily', 'health')).toBe('/health/trends');
  });
});

describe('resolveEntryPath', () => {
  it('prefers the remembered domain and secondary view', () => {
    const state = rememberDomainPath(
      rememberDomainPath(DEFAULT_DOMAIN_NAVIGATION, '/daily/stats'),
      '/health/trends',
    );

    expect(resolveEntryPath(state, true)).toBe('/health/trends');
    expect(resolveEntryPath(state, false)).toBe('/health/trends');
  });

  it('starts in daily today when the family has a child', () => {
    expect(resolveEntryPath(DEFAULT_DOMAIN_NAVIGATION, true)).toBe('/daily/today');
  });

  it('starts in health reports when the family has no child', () => {
    expect(resolveEntryPath(DEFAULT_DOMAIN_NAVIGATION, false)).toBe('/health/reports');
  });
});
