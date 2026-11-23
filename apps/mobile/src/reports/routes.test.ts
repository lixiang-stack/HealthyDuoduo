import { HEALTH_REPORTS_PATH, healthReportsHref } from './routes';

describe('health archive routes', () => {
  it('builds the two archive views', () => {
    expect(healthReportsHref()).toBe(HEALTH_REPORTS_PATH);
    expect(healthReportsHref({ view: 'time' })).toBe(`${HEALTH_REPORTS_PATH}?view=time`);
    expect(healthReportsHref({ view: 'panel' })).toBe(`${HEALTH_REPORTS_PATH}?view=panel`);
  });

  it('keeps filter and panel query params in a stable order', () => {
    expect(healthReportsHref({ view: 'time', filter: 'partial' })).toBe(
      `${HEALTH_REPORTS_PATH}?view=time&filter=partial`,
    );
    expect(healthReportsHref({ view: 'time', panel: 'liver' })).toBe(
      `${HEALTH_REPORTS_PATH}?view=time&panel=liver`,
    );
  });
});
