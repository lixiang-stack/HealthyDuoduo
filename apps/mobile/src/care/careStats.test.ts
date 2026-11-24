import {
  CARE_METRICS,
  buildCareStats,
  careStatsRange,
} from './careStats';
import type { CareEvent } from './careEvents';
import {
  makeDiaperEvent,
  makeFeedEvent,
  makeSleepEvent,
} from '../test/fixtures';

const NOW = new Date(2026, 10, 24, 15, 0, 0);

function at(daysAgo: number, hour: number, minute = 0): string {
  return new Date(
    NOW.getFullYear(),
    NOW.getMonth(),
    NOW.getDate() - daysAgo,
    hour,
    minute,
  ).toISOString();
}

function events(): CareEvent[] {
  return [
    makeFeedEvent({ id: 1, started_at: at(0, 10), method: 'formula', amount_ml: 120 }),
    makeFeedEvent({
      id: 2,
      started_at: at(0, 12),
      method: 'breast',
      amount_ml: null,
      duration_min: 15,
    }),
    makeFeedEvent({ id: 3, started_at: at(1, 9), method: 'formula', amount_ml: 90 }),
    makeDiaperEvent({ id: 4, started_at: at(0, 8), diaper_type: 'wet' }),
    makeDiaperEvent({ id: 5, started_at: at(1, 18), diaper_type: 'dirty' }),
    makeDiaperEvent({ id: 6, started_at: at(2, 10), diaper_type: 'mixed' }),
    makeSleepEvent({
      id: 7,
      started_at: at(0, 13),
      ended_at: at(0, 14, 30),
      duration_min: 90,
    }),
    makeSleepEvent({
      id: 8,
      started_at: at(1, 22),
      ended_at: at(0, 6),
      duration_min: 480,
    }),
  ];
}

describe('care stats range', () => {
  it('covers six previous local days plus today', () => {
    expect(careStatsRange(NOW)).toEqual({
      from: new Date(2026, 10, 18, 0, 0, 0).toISOString(),
      to: new Date(2026, 10, 25, 0, 0, 0).toISOString(),
    });
  });
});

describe('buildCareStats', () => {
  it('builds seven local days with today last', () => {
    const stats = buildCareStats(events(), NOW);

    for (const metric of CARE_METRICS) {
      expect(stats[metric].days).toHaveLength(7);
      expect(stats[metric].hasData).toBe(true);
    }
    expect(stats.feeds.days.map((day) => day.label)).toEqual([
      '11/18',
      '11/19',
      '11/20',
      '11/21',
      '11/22',
      '11/23',
      '今天',
    ]);
    expect(stats.feeds.days.map((day) => day.isToday)).toEqual([
      false,
      false,
      false,
      false,
      false,
      false,
      true,
    ]);
  });

  it('counts feeds, amounts and the within-day average interval', () => {
    const feeds = buildCareStats(events(), NOW).feeds;

    expect(feeds.days.map((day) => day.total)).toEqual([0, 0, 0, 0, 0, 1, 2]);
    expect(feeds.segments.map((segment) => segment.value)).toEqual([1, 2]);
    expect(feeds.kpis.map((kpi) => kpi.value)).toEqual(['3', '0.4', '2 小时']);
  });

  it('uses bottle amounts for the amount metric and excludes breastfeeding from the average', () => {
    const amount = buildCareStats(events(), NOW).amount;

    expect(amount.days.map((day) => day.total)).toEqual([0, 0, 0, 0, 0, 90, 120]);
    expect(amount.segments.map((segment) => segment.value)).toEqual([0, 210]);
    expect(amount.kpis.map((kpi) => kpi.value)).toEqual(['210', '30', '105']);
  });

  it('clips cross-midnight sleep into each local day', () => {
    const sleep = buildCareStats(events(), NOW).sleep;

    expect(sleep.days.map((day) => day.total)).toEqual([0, 0, 0, 0, 0, 2, 7.5]);
    expect(sleep.segments.map((segment) => segment.value)).toEqual([8, 1.5]);
    expect(sleep.kpis.map((kpi) => kpi.value)).toEqual(['1.4', '1.1', '0.2']);
  });

  it('splits a sleep that crosses 07:00 or 19:00 into night and day', () => {
    const sleep = buildCareStats(
      [
        makeSleepEvent({
          id: 9,
          started_at: at(0, 6),
          ended_at: at(0, 9),
          duration_min: 180,
        }),
        makeSleepEvent({
          id: 10,
          started_at: at(0, 18),
          ended_at: at(0, 20),
          duration_min: 120,
        }),
      ],
      NOW,
    ).sleep;

    expect(sleep.days[6].total).toBe(5);
    expect(sleep.segments.map((segment) => segment.value)).toEqual([2, 3]);
  });

  it('counts diaper types', () => {
    const diaper = buildCareStats(events(), NOW).diaper;

    expect(diaper.days.map((day) => day.total)).toEqual([0, 0, 0, 0, 1, 1, 1]);
    expect(diaper.segments.map((segment) => segment.value)).toEqual([1, 1, 1]);
    expect(diaper.kpis.map((kpi) => kpi.value)).toEqual(['3', '1', '2']);
  });

  it('reports no data per metric instead of inventing zeros with a trend', () => {
    const stats = buildCareStats([], NOW);

    for (const metric of CARE_METRICS) {
      expect(stats[metric].hasData).toBe(false);
      expect(stats[metric].days.every((day) => day.total === 0)).toBe(true);
    }
  });

  it('keeps a single recorded day as one point in the seven-day window', () => {
    const stats = buildCareStats(
      [makeFeedEvent({ id: 1, started_at: at(0, 10) })],
      NOW,
    );

    expect(stats.feeds.hasData).toBe(true);
    expect(stats.feeds.days.filter((day) => day.total > 0)).toHaveLength(1);
    expect(stats.feeds.days[6].total).toBe(1);
    expect(stats.amount.hasData).toBe(true);
  });
});
