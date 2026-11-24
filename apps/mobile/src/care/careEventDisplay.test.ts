import type { CareEvent, CareEventRange } from './careEvents';
import {
  careEventMeta,
  careEventTitle,
  duplicateFeedMessage,
  formatElapsedMinutes,
  latestFeed,
  localClockTime,
  localDateKey,
  localDayRange,
  recentCareEventRange,
  splitTodayCareEvents,
  summarizeToday,
} from './careEventDisplay';
import { makeFeedEvent } from '../test/fixtures';

const NOW = new Date(2026, 10, 23, 14, 0);

function at(hour: number, minute: number, day = 23): string {
  return new Date(2026, 10, day, hour, minute).toISOString();
}

function feed(
  overrides: Partial<Extract<CareEvent, { type: 'feed' }>> = {},
): Extract<CareEvent, { type: 'feed' }> {
  return makeFeedEvent({ started_at: at(13, 30), ...overrides });
}

describe('local care-log day and clock', () => {
  it('builds the inclusive/exclusive local day range around now', () => {
    const range: CareEventRange = localDayRange(NOW);

    expect(range.from).toBe(new Date(2026, 10, 23, 0, 0).toISOString());
    expect(range.to).toBe(new Date(2026, 10, 24, 0, 0).toISOString());
  });

  it('reads yesterday and today for the last feed around midnight', () => {
    const range = recentCareEventRange(NOW);

    expect(range.from).toBe(new Date(2026, 10, 22, 0, 0).toISOString());
    expect(range.to).toBe(new Date(2026, 10, 24, 0, 0).toISOString());
  });

  it('formats local date keys and clock times without server-side timezone guessing', () => {
    expect(localDateKey(new Date(2026, 10, 23, 14, 5))).toBe('2026-11-23');
    expect(localClockTime(new Date(2026, 10, 23, 8, 4))).toBe('08:04');
    expect(localClockTime(at(13, 30))).toBe('13:30');
  });

  it('formats elapsed time in minutes and hours', () => {
    expect(formatElapsedMinutes(0)).toBe('刚刚');
    expect(formatElapsedMinutes(5)).toBe('5 分钟');
    expect(formatElapsedMinutes(60)).toBe('1 小时');
    expect(formatElapsedMinutes(65)).toBe('1 小时 5 分钟');
    expect(formatElapsedMinutes(24 * 60)).toBe('24 小时');
  });
});

describe('today summary', () => {
  it('counts only the current child’s feeds and diapers and sums bottle amount', () => {
    const summary = summarizeToday(
      [
        feed({ id: 1, amount_ml: 120 }),
        feed({ id: 2, started_at: at(8, 0), method: 'breast', amount_ml: null, duration_min: 18 }),
        {
          id: 3,
          member_id: 1,
          type: 'diaper',
          started_at: at(9, 0),
          ended_at: null,
          note: null,
          source: 'parent',
          diaper_type: 'wet',
        },
      ],
      NOW,
    );

    expect(summary).toEqual({
      feeds: 2,
      amountMl: 120,
      sleepMinutes: 0,
      diapers: 1,
    });
  });

  it('clips a cross-midnight sleep to today and leaves ongoing sleep open until now', () => {
    const summary = summarizeToday(
      [
        {
          id: 4,
          member_id: 1,
          type: 'sleep',
          started_at: at(23, 50, 22),
          ended_at: at(6, 20),
          note: null,
          source: 'parent',
          duration_min: 390,
        },
        {
          id: 5,
          member_id: 1,
          type: 'sleep',
          started_at: at(13, 0),
          ended_at: null,
          note: null,
          source: 'parent',
          duration_min: null,
        },
      ],
      NOW,
    );

    expect(summary.sleepMinutes).toBe(380 + 60);
  });

  it('does not count yesterday’s point events', () => {
    const summary = summarizeToday(
      [feed({ id: 1, started_at: at(23, 30, 22) })],
      NOW,
    );

    expect(summary.feeds).toBe(0);
    expect(summary.amountMl).toBe(0);
  });
});

describe('last feed', () => {
  it('uses the newest feed that has already started', () => {
    const older = feed({ id: 1, started_at: at(9, 0) });
    const newest = feed({ id: 2, started_at: at(13, 30) });
    const future = feed({ id: 3, started_at: at(15, 0) });

    expect(latestFeed([older, future, newest], NOW)?.id).toBe(2);
  });

  it('ignores non-feed events and returns undefined without a feed', () => {
    const diaper: CareEvent = {
      id: 3,
      member_id: 1,
      type: 'diaper',
      started_at: at(13, 0),
      ended_at: null,
      note: null,
      source: 'parent',
      diaper_type: 'wet',
    };

    expect(latestFeed([diaper], NOW)).toBeUndefined();
    expect(latestFeed([], NOW)).toBeUndefined();
  });

  it('keeps yesterday’s feed as the last feed without putting it in today’s timeline', () => {
    const yesterdayFeed = feed({ id: 1, started_at: at(23, 50, 22) });
    const todayDiaper: CareEvent = {
      id: 2,
      member_id: 1,
      type: 'diaper',
      started_at: at(0, 1),
      ended_at: null,
      note: null,
      source: 'parent',
      diaper_type: 'wet',
    };

    const split = splitTodayCareEvents([yesterdayFeed, todayDiaper], new Date(2026, 10, 23, 0, 5));

    expect(split.today.map((event) => event.id)).toEqual([2]);
    expect(split.lastFeed?.id).toBe(1);
  });
});

describe('care event rows', () => {
  it('builds the duplicate-feed confirmation copy from the previous feed', () => {
    expect(duplicateFeedMessage(feed({ started_at: at(14, 0) }))).toBe(
      '刚刚已有一条喂养记录（14:00 · 奶粉 120 ml），仍然添加？',
    );
  });

  it('describes feed amounts and breastfeeding duration', () => {
    expect(careEventTitle(feed())).toBe('奶粉 120 ml');
    expect(
      careEventTitle(
        feed({ method: 'breast', amount_ml: null, duration_min: 18, note: '左侧' }),
      ),
    ).toBe('母乳 18 分钟');
    expect(careEventMeta(feed({ note: '剩余 20 ml' }))).toBe('剩余 20 ml');
  });

  it('describes diaper, sleep and solid events', () => {
    const diaper: CareEvent = {
      id: 3,
      member_id: 1,
      type: 'diaper',
      started_at: at(9, 0),
      ended_at: null,
      note: '量多',
      source: 'parent',
      diaper_type: 'mixed',
    };
    const sleep: CareEvent = {
      id: 4,
      member_id: 1,
      type: 'sleep',
      started_at: at(13, 0),
      ended_at: null,
      note: null,
      source: 'parent',
      duration_min: null,
    };
    const solid: CareEvent = {
      id: 5,
      member_id: 1,
      type: 'solid',
      started_at: at(12, 0),
      ended_at: null,
      note: '第一次尝试',
      source: 'parent',
      food: '米粉',
      amount_g: 40,
    };

    expect(careEventTitle(diaper)).toBe('尿布 · 尿 + 便');
    expect(careEventMeta(diaper)).toBe('量多');
    expect(careEventTitle(sleep)).toBe('睡觉 · 进行中');
    expect(careEventMeta(sleep)).toBe('');
    expect(careEventTitle(solid)).toBe('辅食 · 米粉 40 g');
    expect(careEventMeta(solid)).toBe('第一次尝试');
  });
});
