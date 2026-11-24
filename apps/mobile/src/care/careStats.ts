// Seven-day care-log aggregation for the stats page. It only reads parent-recorded
// care events: no report / trend data, no analysis snapshot and no feeding advice.
import {
  MINUTES_PER_HOUR,
  MS_PER_MINUTE,
  formatDurationMinutes,
  localDateKey,
} from './careEventDisplay';
import type { CareEvent, CareEventRange } from './careEvents';

export const CARE_METRICS = ['feeds', 'amount', 'sleep', 'diaper'] as const;
export type CareMetric = (typeof CARE_METRICS)[number];

export const CHART_TYPES = ['bar', 'stacked', 'line'] as const;
export type ChartType = (typeof CHART_TYPES)[number];

export const STATS_DAY_COUNT = 7;
const NIGHT_START_HOUR = 19;
const NIGHT_END_HOUR = 7;
const MS_PER_HOUR = MINUTES_PER_HOUR * MS_PER_MINUTE;

// These keys match CARE_METRIC_CONFIG.segments; keeping the mapping named avoids
// positional magic numbers in the aggregation below.
const FEED_SEGMENT = { breast: 0, formula: 1 } as const;
const SLEEP_SEGMENT = { night: 0, day: 1 } as const;
const DIAPER_SEGMENT = { wet: 0, dirty: 1, mixed: 2 } as const;

export type CareStatsSegmentConfig = {
  key: string;
  label: string;
  color: string;
};

export const CARE_METRIC_CONFIG: Record<
  CareMetric,
  { label: string; unit: string; title: string; segments: CareStatsSegmentConfig[] }
> = {
  feeds: {
    label: '喂养',
    unit: '次',
    title: '每日喂养次数',
    segments: [
      { key: 'breast', label: '母乳', color: '#2563eb' },
      { key: 'formula', label: '奶粉', color: '#93c5fd' },
    ],
  },
  amount: {
    label: '奶量',
    unit: 'ml',
    title: '每日奶量',
    segments: [
      { key: 'breast', label: '母乳', color: '#2563eb' },
      { key: 'formula', label: '奶粉', color: '#93c5fd' },
    ],
  },
  sleep: {
    label: '睡眠',
    unit: '小时',
    title: '每日睡眠',
    segments: [
      { key: 'night', label: '夜间', color: '#1e3a8a' },
      { key: 'day', label: '白天', color: '#bfdbfe' },
    ],
  },
  diaper: {
    label: '尿布',
    unit: '次',
    title: '每日尿布',
    segments: [
      { key: 'wet', label: '尿', color: '#2563eb' },
      { key: 'dirty', label: '便', color: '#f59e0b' },
      { key: 'mixed', label: '混合', color: '#d1d5db' },
    ],
  },
};

export type CareStatsDay = {
  key: string;
  label: string;
  isToday: boolean;
  total: number;
  segments: number[];
};

export type CareStatsKpi = {
  label: string;
  value: string;
  unit: string;
};

export type CareMetricStats = {
  metric: CareMetric;
  hasData: boolean;
  days: CareStatsDay[];
  segments: (CareStatsSegmentConfig & { value: number })[];
  kpis: CareStatsKpi[];
};

export type CareStats = Record<CareMetric, CareMetricStats>;

type DayMeta = {
  key: string;
  label: string;
  isToday: boolean;
  start: number;
  end: number;
};

export function careStatsRange(now: Date): CareEventRange {
  const from = new Date(now.getFullYear(), now.getMonth(), now.getDate() - (STATS_DAY_COUNT - 1));
  const to = new Date(now.getFullYear(), now.getMonth(), now.getDate() + 1);
  return { from: from.toISOString(), to: to.toISOString() };
}

export function buildCareStats(events: CareEvent[], now: Date): CareStats {
  const days = dayMetas(now);
  const dayIndex = new Map(days.map((day, index) => [day.key, index]));
  const feedCounts = zeros();
  const feedAmounts = zeros();
  const feedSegments = segmentZeros(2);
  const amountSegments = segmentZeros(2);
  const diaperCounts = zeros();
  const diaperSegments = segmentZeros(3);
  const sleepMinutes = zeros();
  const sleepSegments = segmentZeros(2);
  const feedTimes: number[][] = days.map(() => []);
  let amountFeedCount = 0;

  for (const event of events) {
    if (event.type === 'feed') {
      const index = dayIndex.get(localDateKey(event.started_at));
      if (index === undefined) {
        continue;
      }
      const methodIndex = FEED_SEGMENT[event.method];
      feedCounts[index] += 1;
      feedSegments[methodIndex][index] += 1;
      const amount = event.amount_ml ?? 0;
      feedAmounts[index] += amount;
      amountSegments[methodIndex][index] += amount;
      if (event.amount_ml !== null) {
        amountFeedCount += 1;
      }
      feedTimes[index].push(Date.parse(event.started_at));
      continue;
    }

    if (event.type === 'diaper') {
      const index = dayIndex.get(localDateKey(event.started_at));
      if (index === undefined) {
        continue;
      }
      const typeIndex = DIAPER_SEGMENT[event.diaper_type];
      diaperCounts[index] += 1;
      diaperSegments[typeIndex][index] += 1;
      continue;
    }

    if (event.type === 'sleep') {
      const start = Date.parse(event.started_at);
      const end = event.ended_at === null ? now.getTime() : Date.parse(event.ended_at);
      if (Number.isNaN(start) || Number.isNaN(end) || end <= start) {
        continue;
      }
      // Night is 19:00-07:00 local; day is 07:00-19:00. Split each clipped overlap
      // so a sleep crossing a boundary contributes to both segments.
      for (let index = 0; index < days.length; index += 1) {
        const day = days[index];
        const clippedStart = Math.max(start, day.start);
        const clippedEnd = Math.min(end, day.end);
        if (clippedEnd <= clippedStart) {
          continue;
        }
        sleepMinutes[index] += (clippedEnd - clippedStart) / MS_PER_MINUTE;
        const morningEnd = day.start + NIGHT_END_HOUR * MS_PER_HOUR;
        const eveningStart = day.start + NIGHT_START_HOUR * MS_PER_HOUR;
        addSleepSegment(
          sleepSegments,
          index,
          SLEEP_SEGMENT.night,
          clippedStart,
          clippedEnd,
          day.start,
          morningEnd,
        );
        addSleepSegment(
          sleepSegments,
          index,
          SLEEP_SEGMENT.day,
          clippedStart,
          clippedEnd,
          morningEnd,
          eveningStart,
        );
        addSleepSegment(
          sleepSegments,
          index,
          SLEEP_SEGMENT.night,
          clippedStart,
          clippedEnd,
          eveningStart,
          day.end,
        );
      }
    }
  }

  return {
    feeds: buildFeeds(
      days,
      feedCounts,
      feedSegments,
      feedTimes,
    ),
    amount: buildAmount(days, feedAmounts, amountSegments, amountFeedCount),
    sleep: buildSleep(days, sleepMinutes, sleepSegments),
    diaper: buildDiaper(days, diaperCounts, diaperSegments),
  };
}

function buildFeeds(
  days: DayMeta[],
  counts: number[],
  segments: number[][],
  times: number[][],
): CareMetricStats {
  const total = sum(counts);
  // Overnight gaps are not feeding intervals; average only gaps between feeds
  // recorded on the same local day, matching the prototype's "平均间隔" KPI.
  const gaps: number[] = [];
  for (const dayTimes of times) {
    const sorted = [...dayTimes].sort((a, b) => a - b);
    for (let index = 1; index < sorted.length; index += 1) {
      gaps.push(sorted[index] - sorted[index - 1]);
    }
  }
  const averageGap =
    gaps.length === 0
      ? '—'
      : formatDurationMinutes(Math.round(sum(gaps) / gaps.length / MS_PER_MINUTE));
  return metricStats('feeds', days, counts, segments, total > 0, [
    { label: '7 天喂养', value: String(total), unit: '次' },
    { label: '日均', value: (total / STATS_DAY_COUNT).toFixed(1), unit: '次' },
    { label: '平均间隔', value: averageGap, unit: '' },
  ]);
}

function buildAmount(
  days: DayMeta[],
  amounts: number[],
  segments: number[][],
  feedCount: number,
): CareMetricStats {
  const total = sum(amounts);
  return metricStats('amount', days, amounts, segments, feedCount > 0, [
    { label: '7 天奶量', value: String(total), unit: 'ml' },
    { label: '日均', value: String(Math.round(total / STATS_DAY_COUNT)), unit: 'ml' },
    {
      label: '单次平均',
      value: feedCount === 0 ? '—' : String(Math.round(total / feedCount)),
      unit: 'ml',
    },
  ]);
}

function buildSleep(
  days: DayMeta[],
  minutes: number[],
  segments: number[][],
): CareMetricStats {
  const hours = minutes.map((value) => round1(value / MINUTES_PER_HOUR));
  const segmentHours = segments.map((segment) =>
    segment.map((value) => round1(value / MINUTES_PER_HOUR)),
  );
  const total = sum(hours);
  const night = sum(segmentHours[0]);
  const daytime = sum(segmentHours[1]);
  return metricStats('sleep', days, hours, segmentHours, total > 0, [
    {
      label: '日均睡眠',
      value: (total / STATS_DAY_COUNT).toFixed(1),
      unit: '小时',
    },
    {
      label: '日均夜间',
      value: (night / STATS_DAY_COUNT).toFixed(1),
      unit: '小时',
    },
    {
      label: '日均白天',
      value: (daytime / STATS_DAY_COUNT).toFixed(1),
      unit: '小时',
    },
  ]);
}

function buildDiaper(
  days: DayMeta[],
  counts: number[],
  segments: number[][],
): CareMetricStats {
  const total = sum(counts);
  return metricStats('diaper', days, counts, segments, total > 0, [
    { label: '7 天尿布', value: String(total), unit: '次' },
    { label: '尿', value: String(sum(segments[0])), unit: '次' },
    { label: '便 / 混合', value: String(sum(segments[1]) + sum(segments[2])), unit: '次' },
  ]);
}

function metricStats(
  metric: CareMetric,
  days: DayMeta[],
  totals: number[],
  segmentValues: number[][],
  hasData: boolean,
  kpis: CareStatsKpi[],
): CareMetricStats {
  const config = CARE_METRIC_CONFIG[metric];
  return {
    metric,
    hasData,
    days: days.map((day, index) => ({
      key: day.key,
      label: day.label,
      isToday: day.isToday,
      total: totals[index],
      segments: segmentValues.map((segment) => segment[index]),
    })),
    segments: config.segments.map((segment, index) => ({
      ...segment,
      value: roundMaybe(sum(segmentValues[index])),
    })),
    kpis,
  };
}

function dayMetas(now: Date): DayMeta[] {
  return Array.from({ length: STATS_DAY_COUNT }, (_, index) => {
    const offset = index - (STATS_DAY_COUNT - 1);
    const startDate = new Date(now.getFullYear(), now.getMonth(), now.getDate() + offset);
    const endDate = new Date(now.getFullYear(), now.getMonth(), now.getDate() + offset + 1);
    return {
      key: localDateKey(startDate),
      label:
        index === STATS_DAY_COUNT - 1
          ? '今天'
          : `${startDate.getMonth() + 1}/${startDate.getDate()}`,
      isToday: index === STATS_DAY_COUNT - 1,
      start: startDate.getTime(),
      end: endDate.getTime(),
    };
  });
}

function addSleepSegment(
  segments: number[][],
  dayIndex: number,
  segmentIndex: number,
  sleepStart: number,
  sleepEnd: number,
  rangeStart: number,
  rangeEnd: number,
): void {
  const overlap = Math.min(sleepEnd, rangeEnd) - Math.max(sleepStart, rangeStart);
  if (overlap > 0) {
    segments[segmentIndex][dayIndex] += overlap / MS_PER_MINUTE;
  }
}

function zeros(): number[] {
  return Array.from({ length: STATS_DAY_COUNT }, () => 0);
}

function segmentZeros(count: number): number[][] {
  return Array.from({ length: count }, zeros);
}

function sum(values: number[]): number {
  return values.reduce((total, value) => total + value, 0);
}

function round1(value: number): number {
  return Math.round(value * 10) / 10;
}

function roundMaybe(value: number): number {
  return Number.isInteger(value) ? value : round1(value);
}

