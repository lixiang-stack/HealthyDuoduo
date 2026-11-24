// Presentation helpers for today's care log. They keep device-local day / clock
// calculations and Chinese event summaries out of the screens and repository.
import { colors } from '../components/theme';
import {
  DIAPER_TYPES,
  FEED_METHODS,
  careEventDurationMinutes,
  careEventOverlapsRange,
  compareCareEventsDesc,
  type CareEvent,
  type CareEventRange,
  type CareEventType,
  type DiaperType,
  type FeedMethod,
} from './careEvents';

export const CARE_EVENT_LABELS: Record<CareEventType, string> = {
  feed: '喂奶',
  diaper: '尿布',
  sleep: '睡觉',
  solid: '辅食',
};

export const CARE_EVENT_SOURCE_LABEL = '家长记录';

/** Four light palettes for the timeline nodes. Only the node/bar uses the color;
 *  titles stay neutral so the page still reads as a factual log. */
export const CARE_EVENT_COLORS: Record<CareEventType, { color: string; background: string }> = {
  feed: { color: '#1d4ed8', background: colors.accentSoft },
  diaper: { color: '#b45309', background: '#fffbeb' },
  sleep: { color: '#4f46e5', background: '#eef2ff' },
  solid: { color: '#c2410c', background: '#fff7ed' },
};

export const FEED_METHOD_LABELS: Record<FeedMethod, string> = {
  breast: '母乳',
  formula: '奶粉',
};

export const DIAPER_TYPE_LABELS: Record<DiaperType, string> = {
  wet: '尿尿',
  dirty: '便便',
  mixed: '尿 + 便',
};

export const FEED_METHOD_OPTIONS = FEED_METHODS.map((value) => ({
  value,
  label: FEED_METHOD_LABELS[value],
}));

export const DIAPER_TYPE_OPTIONS = DIAPER_TYPES.map((value) => ({
  value,
  label: DIAPER_TYPE_LABELS[value],
}));

/** Local calendar day [from, to) as absolute instants. */
export function localDayRange(now: Date): CareEventRange {
  const from = new Date(now.getFullYear(), now.getMonth(), now.getDate());
  const to = new Date(now.getFullYear(), now.getMonth(), now.getDate() + 1);
  return { from: from.toISOString(), to: to.toISOString() };
}

/** Read window covering the previous local day plus today, so the hero can show a
 *  feed recorded just before midnight without adding it to today's timeline. */
export function recentCareEventRange(now: Date): CareEventRange {
  const from = new Date(now.getFullYear(), now.getMonth(), now.getDate() - 1);
  const to = new Date(now.getFullYear(), now.getMonth(), now.getDate() + 1);
  return { from: from.toISOString(), to: to.toISOString() };
}

/** Device-local YYYY-MM-DD key. */
export function localDateKey(value: Date | string): string {
  const date = typeof value === 'string' ? new Date(value) : value;
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}`;
}

/** Device-local HH:MM clock. */
export function localClockTime(value: Date | string): string {
  const date = typeof value === 'string' ? new Date(value) : value;
  return `${pad(date.getHours())}:${pad(date.getMinutes())}`;
}

export function elapsedMinutes(startedAt: string, now: Date): number {
  return Math.max(0, Math.floor((now.getTime() - Date.parse(startedAt)) / 60000));
}

export function formatElapsedMinutes(minutes: number): string {
  return minutes < 1 ? '刚刚' : formatDurationMinutes(minutes);
}

/** Newest feed that has already started; future-dated edits do not count. */
export function latestFeed(events: CareEvent[], now: Date): CareEvent | undefined {
  const nowMs = now.getTime();
  return events
    .filter((event) => event.type === 'feed' && Date.parse(event.started_at) <= nowMs)
    .sort(compareCareEventsDesc)[0];
}

/** Today's timeline is local-day only; the last feed may come from the previous
 *  day (the hero must not reset to "no feed" at midnight). */
export function splitTodayCareEvents(
  recent: CareEvent[],
  now: Date,
): { today: CareEvent[]; lastFeed: CareEvent | undefined } {
  const range = localDayRange(now);
  return {
    today: recent.filter((event) => careEventOverlapsRange(event, range)),
    lastFeed: latestFeed(recent, now),
  };
}

export type TodayCareSummary = {
  feeds: number;
  amountMl: number;
  sleepMinutes: number;
  diapers: number;
};

/** Today's counts; cross-midnight sleep is clipped to the local day. */
export function summarizeToday(events: CareEvent[], now: Date): TodayCareSummary {
  const range = localDayRange(now);
  const summary: TodayCareSummary = {
    feeds: 0,
    amountMl: 0,
    sleepMinutes: 0,
    diapers: 0,
  };
  for (const event of events) {
    if (!careEventOverlapsRange(event, range)) {
      continue;
    }
    if (event.type === 'feed') {
      summary.feeds += 1;
      summary.amountMl += event.amount_ml ?? 0;
    } else if (event.type === 'diaper') {
      summary.diapers += 1;
    } else if (event.type === 'sleep') {
      summary.sleepMinutes += sleepMinutesWithin(event, range, now);
    }
  }
  return summary;
}

export function feedSummary(event: Extract<CareEvent, { type: 'feed' }>): string {
  const label = FEED_METHOD_LABELS[event.method];
  if (event.method === 'formula') {
    return event.amount_ml === null ? label : `${label} ${event.amount_ml} ml`;
  }
  return event.duration_min === null ? label : `${label} ${event.duration_min} 分钟`;
}

export function duplicateFeedMessage(event: Extract<CareEvent, { type: 'feed' }>): string {
  return `刚刚已有一条喂养记录（${localClockTime(event.started_at)} · ${feedSummary(event)}），仍然添加？`;
}

export function careEventTitle(event: CareEvent): string {
  switch (event.type) {
    case 'feed':
      return feedSummary(event);
    case 'diaper':
      return `${CARE_EVENT_LABELS.diaper} · ${DIAPER_TYPE_LABELS[event.diaper_type]}`;
    case 'sleep': {
      const duration = careEventDurationMinutes(event);
      return duration === null
        ? `${CARE_EVENT_LABELS.sleep} · 进行中`
        : `${CARE_EVENT_LABELS.sleep} ${formatDurationMinutes(duration)}`;
    }
    case 'solid':
      return event.amount_g === null
        ? `${CARE_EVENT_LABELS.solid} · ${event.food}`
        : `${CARE_EVENT_LABELS.solid} · ${event.food} ${event.amount_g} g`;
  }
}

export function careEventMeta(event: CareEvent): string {
  return event.note ?? '';
}

export function formatDurationMinutes(minutes: number): string {
  const hours = Math.floor(minutes / 60);
  const rest = minutes % 60;
  if (hours === 0) {
    return `${rest} 分钟`;
  }
  return rest === 0 ? `${hours} 小时` : `${hours} 小时 ${rest} 分钟`;
}

function sleepMinutesWithin(event: CareEvent, range: CareEventRange, now: Date): number {
  const start = Math.max(Date.parse(event.started_at), Date.parse(range.from));
  const end = Math.min(
    event.ended_at === null ? now.getTime() : Date.parse(event.ended_at),
    Date.parse(range.to),
  );
  return Math.max(0, Math.round((end - start) / 60000));
}

function pad(value: number): string {
  return String(value).padStart(2, '0');
}
