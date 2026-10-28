// 趋势图纯逻辑(票 04):按指标单位把项目详情的跨报告点切成同单位序列,
// 再生成迷你图/完整图几何与元信息。本页数据全部来自 PanelItem.points,
// 跨单位不连线、不换算。
import type { ReportItem, TrendPoint } from '../api/reports';

import { parseNumericForm, parseRefRange, type NumericForm } from './refRange';
import { DATE_UNKNOWN_LABEL } from './status';

const SAME_DATE_STEP = 4;
const SPARKLINE_PADDING = 8;
const DETAIL_PADDING = { left: 38, right: 10, top: 10, bottom: 24 };

export const TREND_UNAVAILABLE_LABEL = '暂无可用数值趋势';

export type AbnormalFlag = 'high' | 'low' | 'abnormal';

/** 异常指标 = high / low / abnormal;normal 与 unknown 不进入趋势列表。 */
export function isAbnormalFlag(flag: ReportItem['flag']): flag is AbnormalFlag {
  return flag === 'high' || flag === 'low' || flag === 'abnormal';
}

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

export type ChartLayoutOptions = { axes: boolean; selectedIndex?: number | null };

export type TrendChartPoint = {
  reportId: string;
  reportDate: string;
  value: number;
  flag: ReportItem['flag'];
  filled: boolean;
  latest: boolean;
  selected: boolean;
  x: number;
  y: number;
};

export type TrendChartBand = {
  x: number;
  y: number;
  width: number;
  height: number;
};

export type TrendChartTick = {
  value: number;
  y: number;
  label: string;
};

export type TrendChartXLabel = {
  x: number;
  label: string;
  anchor: 'start' | 'middle' | 'end';
};

export type TrendChartPlot = {
  left: number;
  right: number;
  top: number;
  bottom: number;
};

export type TrendChartLayout = {
  points: TrendChartPoint[];
  bands: TrendChartBand[];
  /** SVG polyline 的 points 属性;单点为空串(不画线)。 */
  line: string;
  /** 完整图的 3 个取整 y 刻度;迷你图为空。 */
  yTicks: TrendChartTick[];
  /** 完整图的首尾日期标签;迷你图为空。 */
  xLabels: TrendChartXLabel[];
  plot: TrendChartPlot;
  domain: { min: number; max: number };
};

/** null / 空串归一,单位比较共用同一口径。 */
function normalizedUnit(unit: string | null | undefined): string | null {
  const trimmed = unit?.trim() ?? '';
  return trimmed === '' ? null : trimmed;
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
 * 按指标单位裁出趋势序列:同单位过滤(normalize null/空)→ report_id 去重 →
 * 日期升序 / dateless 沉底。跨单位只做计数,不连线。
 */
export function selectSeries(
  points: TrendPoint[],
  unit: string | null | undefined,
  selectedReportId: string,
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
    listPoints.find((point) => point.report_id === selectedReportId) ??
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

/** 详情层默认选中项:行最新点对应的点;找不到时回落最新点。 */
export function defaultSelectedIndex(series: TrendSeries): number {
  const index = series.listPoints.findIndex(
    (point) => point.report_id === series.selectedPoint?.report_id,
  );
  return index >= 0 ? index : Math.max(0, series.listPoints.length - 1);
}

/** 信任标记:实心 = 已核对或非低置信;空心 = 未核对低置信。 */
export function isFilledPoint(
  point: Pick<TrendPoint, 'reviewed' | 'low_confidence'>,
): boolean {
  return point.reviewed || !point.low_confidence;
}

/** 数值点 + 参考范围端点共同决定 y 域;非负数据从 0 起(8% 边距只作用于上界)。 */
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
  const span = rawMax - rawMin;
  const pad = span === 0 ? Math.max(Math.abs(rawMin), 1) * 0.08 : span * 0.08;
  return { min: 0, max: rawMax + pad };
}

/** 完整图 y 轴取整:非负数从 0 起,上下界按跨度的一半量级取整。 */
function niceDomain(domain: { min: number; max: number }): { min: number; max: number } {
  const span = domain.max - domain.min;
  const step = 10 ** Math.floor(Math.log10(span / 2));
  return { min: 0, max: Math.ceil(domain.max / step) * step };
}

function formatTick(value: number): string {
  return String(Number(value.toFixed(3)));
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
 * 图几何:迷你图(axes=false)与完整图(axes=true)共用。x 按日期,单点居中,
 * 同日点微偏移;y 域含参考范围;band 按点分段;实心/空心由信任标记决定。
 * 完整图额外给出 3 个取整 y 刻度与首尾日期标签,并放大 selectedIndex。
 */
export function chartLayout(
  series: TrendSeries,
  width: number,
  height: number,
  options: ChartLayoutOptions,
): TrendChartLayout {
  const points = series.chartPoints;
  const rawDomain = yDomain(points);
  const emptyPlot = { left: 0, right: 0, top: 0, bottom: 0 };
  if (!rawDomain) {
    return {
      points: [],
      bands: [],
      line: '',
      yTicks: [],
      xLabels: [],
      plot: emptyPlot,
      domain: { min: 0, max: 0 },
    };
  }
  const domain = options.axes ? niceDomain(rawDomain) : rawDomain;

  const plot = options.axes
    ? {
        left: DETAIL_PADDING.left,
        right: Math.max(DETAIL_PADDING.left, width - DETAIL_PADDING.right),
        top: DETAIL_PADDING.top,
        bottom: Math.max(DETAIL_PADDING.top, height - DETAIL_PADDING.bottom),
      }
    : {
        left: SPARKLINE_PADDING,
        right: Math.max(SPARKLINE_PADDING, width - SPARKLINE_PADDING),
        top: SPARKLINE_PADDING,
        bottom: Math.max(SPARKLINE_PADDING, height - SPARKLINE_PADDING),
      };
  const plotWidth = plot.right - plot.left;

  const times = points.map((point) => Date.parse(point.report_date));
  const minTime = Math.min(...times);
  const maxTime = Math.max(...times);
  const positions = times.map((time) =>
    minTime === maxTime
      ? plot.left + plotWidth / 2
      : plot.left + ((time - minTime) / (maxTime - minTime)) * plotWidth,
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
      positions[pointIndex] = clamp(positions[pointIndex] + offset, plot.left, plot.right);
    });
  }

  const yFor = (value: number) =>
    plot.bottom - ((value - domain.min) / (domain.max - domain.min)) * (plot.bottom - plot.top);

  const selectedIndex =
    options.selectedIndex === undefined ? points.length - 1 : options.selectedIndex;
  const layoutPoints: TrendChartPoint[] = points.map((point, index) => {
    const value = Number(point.value);
    return {
      reportId: point.report_id,
      reportDate: point.report_date,
      value,
      flag: point.flag,
      filled: isFilledPoint(point),
      latest: index === points.length - 1,
      selected: index === selectedIndex,
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
    const nextX = index < points.length - 1 ? layoutPoints[index + 1].x : plot.right;
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

  const yTicks = options.axes
    ? [domain.min, (domain.min + domain.max) / 2, domain.max].map((value) => ({
        value,
        y: yFor(value),
        label: formatTick(value),
      }))
    : [];
  const xLabels = options.axes ? buildXLabels(points, plot) : [];

  return { points: layoutPoints, bands, line, yTicks, xLabels, plot, domain };
}

function buildXLabels(
  points: ChartableTrendPoint[],
  plot: TrendChartPlot,
): TrendChartXLabel[] {
  if (points.length === 0) {
    return [];
  }
  const first = points[0];
  const last = points[points.length - 1];
  if (points.length === 1 || first.report_date === last.report_date) {
    return [
      {
        x: plot.left + (plot.right - plot.left) / 2,
        label: first.report_date,
        anchor: 'middle',
      },
    ];
  }
  return [
    { x: plot.left, label: first.report_date, anchor: 'start' },
    { x: plot.right, label: last.report_date, anchor: 'end' },
  ];
}

/**
 * 行元信息:日期范围 + 份数;追加缺日期 / 非数值指标项 / 单位不同(未连线),
 * 三项条件独立。份数按同单位去重后的报告数。
 */
export function formatSeriesMeta(
  series: TrendSeries,
  latest: { reportDate: string | null },
): string {
  const points = series.listPoints;
  const dated = points.filter(isDated);
  const parts: string[] = [];

  if (points.length === 0) {
    parts.push(`${latest.reportDate ?? DATE_UNKNOWN_LABEL} · 暂无趋势数据`);
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

/** 详情图不可画时的中性原因;有可绘制数值点时不返回原因。 */
export function trendUnavailableReason(
  series: TrendSeries,
  latest: { reportDate: string | null; value: string | null },
): string | undefined {
  if (series.chartPoints.length > 0) {
    return undefined;
  }
  if (parseNumericForm(latest.value).kind !== 'scalar') {
    return '最近一次为非数值结果';
  }
  if (latest.reportDate === null) {
    return '最近一次缺日期';
  }
  return '没有可绘制的数值点';
}
