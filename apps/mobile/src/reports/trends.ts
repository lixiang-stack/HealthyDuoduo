// 异常指标趋势纯逻辑(票 03):先从当前成员的最新非 failed 报告里选出异常指标候选,
// 再按候选单位把趋势端点返回的全部单位点切成同单位序列,最后生成迷你图几何与元信息。
// 序列只来自趋势端点;这里不做跨报告拼装,也不做跨单位换算。
import type { ReportDetail, ReportItem, ReportSummary, TrendPoint } from '../api/reports';

import { latestByCategory } from './overview';
import { parseNumericForm, parseRefRange, type NumericForm } from './refRange';
import { DATE_UNKNOWN_LABEL } from './status';

const SAME_DATE_STEP = 4;

export type AbnormalFlag = 'high' | 'low' | 'abnormal';

/** 异常指标 = high / low / abnormal;normal 与 unknown 不进入趋势列表。 */
export function isAbnormalFlag(flag: ReportItem['flag']): flag is AbnormalFlag {
  return flag === 'high' || flag === 'low' || flag === 'abnormal';
}

export type TrendCandidate = {
  reportType: string;
  reportId: string;
  reportDate: string | null;
  item: ReportItem;
};

export type DatedTrendPoint = TrendPoint & { report_date: string };
export type ChartableTrendPoint = DatedTrendPoint & { value: string };

export type TrendSeries = {
  unit: string | null;
  /** 同单位、按 report_id 去重后的全部点(日期升序,dateless 沉底)。 */
  listPoints: TrendPoint[];
  /** listPoints 中有日期且值为标量的点,供绘图。 */
  chartPoints: ChartableTrendPoint[];
  undatedCount: number;
  nonNumericCount: number;
  otherUnitCount: number;
  totalCount: number;
  selectedPoint: TrendPoint | undefined;
};

export type ChartLayoutOptions = { axes: false };

export type TrendChartPoint = {
  reportId: string;
  reportDate: string;
  value: number;
  flag: ReportItem['flag'];
  filled: boolean;
  latest: boolean;
  x: number;
  y: number;
};

export type TrendChartBand = {
  x: number;
  y: number;
  width: number;
  height: number;
};

export type TrendChartLayout = {
  points: TrendChartPoint[];
  bands: TrendChartBand[];
  /** SVG polyline 的 points 属性;单点为空串(不画线)。 */
  line: string;
  domain: { min: number; max: number };
};

/** null / 空串归一,单位比较与连接键共用同一口径。 */
function normalizedUnit(unit: string | null | undefined): string | null {
  const trimmed = unit?.trim() ?? '';
  return trimmed === '' ? null : trimmed;
}

/** 趋势连接键:name + 归一单位(与候选去重、请求去重共用)。 */
export function trendKey(name: string, unit: string | null): string {
  return JSON.stringify([name, normalizedUnit(unit)]);
}

function hasAbnormalFlags(report: ReportSummary): boolean {
  const { high, low, abnormal } = report.flag_counts;
  return high + low + abnormal > 0;
}

/** 每类最新非 failed 报告,只留 flag_counts 中有异常项的类别;顺序即类别日期降序。 */
export function latestAbnormalByCategory(reports: ReportSummary[]): ReportSummary[] {
  return latestByCategory(reports.filter((report) => report.status !== 'failed')).filter(
    hasAbnormalFlags,
  );
}

/**
 * 候选异常指标:每类最新非 failed 报告 → 只留异常项 → 按 name + unit 去重。
 * 保持类别日期序与报告内 item 序;没有候选的类别自然不出现。
 */
export function selectAbnormalCandidates(
  reports: ReportSummary[],
  details: ReportDetail[],
): TrendCandidate[] {
  const detailById = new Map(details.map((detail) => [detail.report_id, detail]));
  const seen = new Set<string>();
  const candidates: TrendCandidate[] = [];

  for (const report of latestAbnormalByCategory(reports)) {
    const detail = detailById.get(report.report_id);
    if (!detail) {
      continue;
    }
    for (const item of detail.items) {
      if (!isAbnormalFlag(item.flag)) {
        continue;
      }
      const key = trendKey(item.name, item.unit);
      if (seen.has(key)) {
        continue;
      }
      seen.add(key);
      candidates.push({
        reportType: report.report_type,
        reportId: report.report_id,
        reportDate: report.report_date,
        item,
      });
    }
  }

  return candidates;
}

function isDated(point: TrendPoint): point is DatedTrendPoint {
  return point.report_date !== null;
}

function comparePoints(a: TrendPoint, b: TrendPoint): number {
  if (a.report_date !== b.report_date) {
    if (a.report_date === null) {
      return 1;
    }
    if (b.report_date === null) {
      return -1;
    }
    return a.report_date < b.report_date ? -1 : 1;
  }
  if (a.report_id === b.report_id) {
    return 0;
  }
  return a.report_id < b.report_id ? -1 : 1;
}

/**
 * 按候选单位裁出趋势序列:同单位过滤(normalize null/空)→ report_id 去重 →
 * 日期升序 / dateless 沉底。跨单位只做计数,不连线。
 */
export function selectSeries(
  points: TrendPoint[],
  unit: string | null | undefined,
  candidateReportId: string,
): TrendSeries {
  const targetUnit = normalizedUnit(unit);
  const sameUnit: TrendPoint[] = [];
  const otherUnitReports = new Set<string>();

  for (const point of points) {
    if (normalizedUnit(point.unit) === targetUnit) {
      sameUnit.push(point);
    } else {
      otherUnitReports.add(point.report_id);
    }
  }

  sameUnit.sort(comparePoints);
  const seenReports = new Set<string>();
  const listPoints: TrendPoint[] = [];
  for (const point of sameUnit) {
    if (seenReports.has(point.report_id)) {
      continue;
    }
    seenReports.add(point.report_id);
    listPoints.push(point);
  }

  const chartPoints = listPoints.filter(
    (point): point is ChartableTrendPoint =>
      isDated(point) && point.value !== null && parseNumericForm(point.value).kind === 'scalar',
  );
  const dated = listPoints.filter(isDated);
  const selected =
    listPoints.find((point) => point.report_id === candidateReportId) ??
    dated[dated.length - 1] ??
    listPoints[listPoints.length - 1];

  return {
    unit: targetUnit,
    listPoints,
    chartPoints,
    undatedCount: listPoints.length - dated.length,
    nonNumericCount: listPoints.filter(
      (point) => parseNumericForm(point.value).kind !== 'scalar',
    ).length,
    otherUnitCount: otherUnitReports.size,
    totalCount: points.length,
    selectedPoint: selected,
  };
}

/** 数值点 + 参考范围端点共同决定 y 域;非负数据从 0 起(8% 边距后再夹回 0)。 */
function anchorsOf(point: TrendPoint): number[] {
  const anchors: number[] = [];
  const value = parseNumericForm(point.value);
  if (value.kind === 'scalar') {
    anchors.push(value.value);
  }
  const ref = parseRefRange(point.ref_range);
  if (ref.kind === 'interval') {
    anchors.push(ref.low, ref.high);
  } else if (ref.kind === 'scalar') {
    anchors.push(ref.value);
  } else if (ref.kind === 'bound') {
    anchors.push(ref.bound);
  }
  return anchors;
}

function yDomain(points: TrendPoint[]): { min: number; max: number } | undefined {
  const anchors = points.flatMap(anchorsOf);
  if (anchors.length === 0) {
    return undefined;
  }
  const rawMin = Math.min(...anchors);
  const rawMax = Math.max(...anchors);
  let min = rawMin;
  let max = rawMax;
  if (rawMin === rawMax) {
    const pad = Math.max(Math.abs(rawMin), 1) * 0.08;
    min -= pad;
    max += pad;
  } else {
    const pad = (rawMax - rawMin) * 0.08;
    min -= pad;
    max += pad;
  }
  if (!anchors.some((anchor) => anchor < 0)) {
    min = 0;
  }
  return { min, max };
}

function clamp(value: number, min: number, max: number): number {
  return Math.min(max, Math.max(min, value));
}

/** 参考范围按点分段的 y 上下界;单侧范围借用整段 y 域的两端。 */
function refBounds(
  ref: NumericForm,
  domain: { min: number; max: number },
): { low: number; high: number } | undefined {
  if (ref.kind === 'interval') {
    return { low: ref.low, high: ref.high };
  }
  if (ref.kind === 'scalar') {
    return { low: ref.value, high: ref.value };
  }
  if (ref.kind === 'bound') {
    return ref.op === '<' || ref.op === '<='
      ? { low: domain.min, high: ref.bound }
      : { low: ref.bound, high: domain.max };
  }
  return undefined;
}

/**
 * 图几何:迷你图(axes=false)与票 04 的完整图共用。x 按日期,单点居中,
 * 同日点微偏移;y 域含参考范围;band 按点分段;实心/空心由信任标记决定。
 */
export function chartLayout(
  series: TrendSeries,
  width: number,
  height: number,
  _options: ChartLayoutOptions = { axes: false },
): TrendChartLayout {
  const points = series.chartPoints;
  const domain = yDomain(points);
  if (!domain) {
    return { points: [], bands: [], line: '', domain: { min: 0, max: 0 } };
  }

  const plotLeft = 8;
  const plotRight = Math.max(plotLeft, width - 8);
  const plotTop = 8;
  const plotBottom = Math.max(plotTop, height - 8);
  const plotWidth = plotRight - plotLeft;

  const times = points.map((point) => Date.parse(point.report_date));
  const minTime = Math.min(...times);
  const maxTime = Math.max(...times);
  const positions = times.map((time) =>
    minTime === maxTime
      ? plotLeft + plotWidth / 2
      : plotLeft + ((time - minTime) / (maxTime - minTime)) * plotWidth,
  );

  const sameDate = new Map<string, number[]>();
  points.forEach((point, index) => {
    const indices = sameDate.get(point.report_date) ?? [];
    indices.push(index);
    sameDate.set(point.report_date, indices);
  });
  for (const indices of sameDate.values()) {
    if (indices.length < 2) {
      continue;
    }
    indices.forEach((pointIndex, order) => {
      const offset = (order - (indices.length - 1) / 2) * SAME_DATE_STEP;
      positions[pointIndex] = clamp(positions[pointIndex] + offset, plotLeft, plotRight);
    });
  }

  const yFor = (value: number) =>
    plotBottom - ((value - domain.min) / (domain.max - domain.min)) * (plotBottom - plotTop);

  const layoutPoints: TrendChartPoint[] = points.map((point, index) => {
    const value = Number(point.value);
    return {
      reportId: point.report_id,
      reportDate: point.report_date,
      value,
      flag: point.flag,
      filled: point.reviewed || !point.low_confidence,
      latest: index === points.length - 1,
      x: positions[index],
      y: yFor(value),
    };
  });

  const bands: TrendChartBand[] = [];
  points.forEach((point, index) => {
    const bounds = refBounds(parseRefRange(point.ref_range), domain);
    if (!bounds) {
      return;
    }
    const x = layoutPoints[index].x;
    const nextX = index < points.length - 1 ? layoutPoints[index + 1].x : plotRight;
    const top = yFor(bounds.high);
    const bottom = yFor(bounds.low);
    bands.push({
      x,
      y: Math.min(top, bottom),
      width: Math.max(1, nextX - x),
      height: Math.max(1, Math.abs(bottom - top)),
    });
  });

  const line =
    layoutPoints.length > 1
      ? layoutPoints.map((point) => `${point.x.toFixed(1)},${point.y.toFixed(1)}`).join(' ')
      : '';

  return { points: layoutPoints, bands, line, domain };
}

/**
 * 行元信息:日期范围 + 份数;追加缺日期 / 非数值指标项 / 单位不同(未连线),
 * 三项条件独立。份数按同单位去重后的报告数。
 */
export function formatSeriesMeta(series: TrendSeries, candidate: TrendCandidate): string {
  const points = series.listPoints;
  const dated = points.filter(isDated);
  const parts: string[] = [];

  if (points.length === 0) {
    parts.push(`${candidate.reportDate ?? DATE_UNKNOWN_LABEL} · 暂无趋势数据`);
  } else if (points.length === 1) {
    parts.push(`${points[0].report_date ?? DATE_UNKNOWN_LABEL} · 仅 1 份报告`);
  } else if (dated.length === 0) {
    parts.push(`${DATE_UNKNOWN_LABEL} · ${points.length} 份报告`);
  } else {
    const first = dated[0].report_date;
    const last = dated[dated.length - 1].report_date;
    const range = first === last ? first : `${first} – ${last}`;
    parts.push(`${range} · ${points.length} 份报告`);
  }

  if (dated.length > 0 && series.undatedCount > 0) {
    parts.push(`另有 ${series.undatedCount} 份缺日期`);
  }
  if (series.nonNumericCount > 0) {
    parts.push(`另有 ${series.nonNumericCount} 个非数值指标项`);
  }
  if (series.otherUnitCount > 0) {
    parts.push(`另有 ${series.otherUnitCount} 份单位不同（未连线）`);
  }

  return parts.join(' · ');
}
