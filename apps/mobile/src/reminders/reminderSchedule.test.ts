import {
  reminderNotificationContent,
  reminderTiming,
  reminderTimingLabel,
} from './reminderSchedule';

const NOW = new Date('2026-11-24T15:00:00+08:00');

describe('reminder timing', () => {
  it('has no reminder without a last feed', () => {
    expect(reminderTiming(undefined, 3, NOW)).toEqual({ status: 'no-last-feed' });
  });

  it('schedules from the last feed when the interval ends in the future', () => {
    const timing = reminderTiming('2026-11-24T14:00:00+08:00', 3, NOW);

    expect(timing).toEqual({
      status: 'scheduled',
      at: new Date('2026-11-24T17:00:00+08:00'),
    });
  });

  it('reports an overdue reminder when the interval already passed', () => {
    const timing = reminderTiming('2026-11-24T09:00:00+08:00', 3, NOW);

    expect(timing).toEqual({
      status: 'overdue',
      at: new Date('2026-11-24T12:00:00+08:00'),
    });
  });

  it('treats an unparseable feed time as no feed', () => {
    expect(reminderTiming('not-a-date', 3, NOW)).toEqual({ status: 'no-last-feed' });
  });

  it('labels the last and next clock times', () => {
    expect(reminderTimingLabel('2026-11-24T14:00:00+08:00', 3, NOW)).toBe(
      '上次 14:00 · 下次 17:00',
    );
    expect(reminderTimingLabel('2026-11-24T09:00:00+08:00', 3, NOW)).toBe(
      '上次 09:00 · 已过提醒时间 12:00',
    );
    expect(reminderTimingLabel(undefined, 3, NOW)).toBe('开启后从上次喂养开始计时');
  });
});

describe('reminder notification copy', () => {
  it('only states elapsed time and makes no feeding suggestion', () => {
    const content = reminderNotificationContent(3);

    expect(content.title).toBe('距上次喂养已 3 小时');
    expect(content.body).toBe(
      '提醒只陈述时间，不提供喂养建议。',
    );
    expect(`${content.title}${content.body}`).not.toMatch(/该喂|ml|奶量/);
  });
});
