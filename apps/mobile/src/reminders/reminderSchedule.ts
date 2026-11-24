// Pure reminder timing / copy. The schedule starts from the latest parent-recorded
// feed; it is a fact-based local reminder, never feeding advice.
import {
  MINUTES_PER_HOUR,
  MS_PER_MINUTE,
  localClockTime,
} from '../care/careEventDisplay';
import type { ReminderIntervalHours } from './reminderSettings';

export type ReminderTiming =
  | { status: 'no-last-feed' }
  | { status: 'scheduled'; at: Date }
  | { status: 'overdue'; at: Date };

export function reminderTiming(
  lastFeedStartedAt: string | undefined,
  intervalHours: ReminderIntervalHours,
  now: Date,
): ReminderTiming {
  if (lastFeedStartedAt === undefined) {
    return { status: 'no-last-feed' };
  }
  const started = Date.parse(lastFeedStartedAt);
  if (Number.isNaN(started)) {
    return { status: 'no-last-feed' };
  }
  const at = new Date(started + intervalHours * MINUTES_PER_HOUR * MS_PER_MINUTE);
  return at.getTime() > now.getTime()
    ? { status: 'scheduled', at }
    : { status: 'overdue', at };
}

export function reminderNotificationContent(intervalHours: ReminderIntervalHours): {
  title: string;
  body: string;
} {
  return {
    title: `距上次喂养已 ${intervalHours} 小时`,
    body: '提醒只陈述时间，不提供喂养建议。',
  };
}

/** One factual line shared by the today card and the reminder settings page. */
export function reminderTimingLabel(
  lastFeedStartedAt: string | undefined,
  intervalHours: ReminderIntervalHours,
  now: Date,
): string {
  const timing = reminderTiming(lastFeedStartedAt, intervalHours, now);
  if (timing.status === 'no-last-feed') {
    return '开启后从上次喂养开始计时';
  }
  const last = localClockTime(lastFeedStartedAt!);
  const next = localClockTime(timing.at);
  return timing.status === 'scheduled'
    ? `上次 ${last} · 下次 ${next}`
    : `上次 ${last} · 已过提醒时间 ${next}`;
}
