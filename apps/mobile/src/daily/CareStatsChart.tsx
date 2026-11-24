// Stats chart rendering: bars and stacks use plain Views; the line uses SVG. No chart
// library and no invented trend: a seven-day window is always the real recorded days.
import { useState } from 'react';
import {
  StyleSheet,
  Text,
  View,
  type DimensionValue,
  type LayoutChangeEvent,
} from 'react-native';
import Svg, { Circle, Line, Polyline, Text as SvgText } from 'react-native-svg';

import {
  CARE_METRIC_CONFIG,
  type CareMetric,
  type CareMetricStats,
  type ChartType,
} from '../care/careStats';
import { colors } from '../components/theme';

export const CHART_COPY: Record<
  ChartType,
  { question: string; title: string; body: string; fit: string; notFit: string }
> = {
  bar: {
    question: '每天多少？',
    title: '柱状图：比较每天总量',
    body: '适合看哪一天多、哪一天少；看不出一天由什么组成，也不擅长表达连续趋势。',
    fit: '适合：每日总量对比',
    notFit: '不适合：构成 / 趋势 / 分布',
  },
  stacked: {
    question: '每天由什么组成？',
    title: '堆叠柱：总量 + 构成',
    body: '总高度还是每天总量，分段告诉你是由什么组成；段与段比较的是构成，不是趋势。',
    fit: '适合：睡眠夜间 / 白天、尿布尿 / 便、母乳 / 奶粉',
    notFit: '不适合：精确趋势 / 分布',
  },
  line: {
    question: '往哪走？',
    title: '折线图：看 7 天趋势',
    body: '适合看整体往上、往下还是持平；单日高低不如柱状直观，也看不到构成。',
    fit: '适合：30 天趋势 / 连续变化',
    notFit: '不适合：单日构成 / 分布',
  },
};

const CHART_COLORS = {
  bar: '#bfdbfe',
  fitBackground: '#f0fdf4',
  fitBorder: '#bbf7d0',
  notFitBackground: '#fef2f2',
  notFitBorder: '#fecaca',
};

const DEFAULT_WIDTH = 320;
const LINE_HEIGHT = 190;
const PAD_LEFT = 30;
const PAD_RIGHT = 12;
const PAD_TOP = 18;
const PAD_BOTTOM = 24;

export function StatsChart({
  metric,
  chart,
  stats,
}: {
  metric: CareMetric;
  chart: ChartType;
  stats: CareMetricStats;
}) {
  const config = CARE_METRIC_CONFIG[metric];
  const copy = CHART_COPY[chart];
  return (
    <>
      <View style={styles.card}>
        <View style={styles.cardHead}>
          <Text style={styles.cardTitle}>
            {chart === 'bar' ? config.title : `${config.title}${chart === 'stacked' ? '构成' : '趋势'}`}
          </Text>
          <Text style={styles.unit}>单位：{config.unit}</Text>
        </View>
        {chart === 'bar' ? (
          <BarChart stats={stats} metric={metric} />
        ) : chart === 'stacked' ? (
          <StackedChart stats={stats} metric={metric} />
        ) : (
          <LineChart stats={stats} metric={metric} />
        )}
        {chart === 'stacked' ? (
          <View style={styles.legend}>
            {stats.segments.map((segment) => (
              <View key={segment.key} testID={`stats-segment-${segment.key}`} style={styles.legendItem}>
                <View style={[styles.legendDot, { backgroundColor: segment.color }]} />
                <Text style={styles.legendText}>
                  {segment.label} {displayValue(metric, segment.value)} {config.unit}
                </Text>
              </View>
            ))}
          </View>
        ) : null}
      </View>

      <View testID="stats-caption" style={styles.caption}>
        <Text style={styles.captionTitle}>{copy.title}</Text>
        <Text style={styles.disclaimer}>来自家长记录，不构成建议。</Text>
        <Text style={styles.captionBody}>{copy.body}</Text>
        <View style={styles.captionRow}>
          <Text style={styles.fit}>{copy.fit}</Text>
          <Text style={styles.notFit}>{copy.notFit}</Text>
        </View>
      </View>
      <Text testID="stats-recommendation" style={styles.recommend}>
        选图建议：柱状看每天多少；需要构成时用堆叠；看 7 天走向用折线；直方图不在本期。
      </Text>
    </>
  );
}

function BarChart({ stats, metric }: { stats: CareMetricStats; metric: CareMetric }) {
  const max = chartMax(stats);
  return (
    <View testID="stats-chart-bar" style={styles.chartBody}>
      {stats.days.map((day, index) => {
        const height = day.total <= 0 ? 0 : Math.max(6, Math.round((day.total / max) * 100));
        return (
          <View key={day.key} style={styles.column}>
            <Text testID={`stats-day-value-${index}`} style={styles.dayValue}>
              {displayValue(metric, day.total)}
            </Text>
            <View style={styles.barWrap}>
              <View
                testID={day.isToday ? 'stats-bar-today' : undefined}
                style={[
                  styles.bar,
                  { height: `${height}%` as DimensionValue },
                  day.isToday && styles.barToday,
                ]}
              />
            </View>
            <Text style={[styles.dayLabel, day.isToday && styles.dayLabelToday]}>
              {day.label}
            </Text>
          </View>
        );
      })}
    </View>
  );
}

function StackedChart({ stats, metric }: { stats: CareMetricStats; metric: CareMetric }) {
  const max = chartMax(stats);
  const config = CARE_METRIC_CONFIG[metric];
  return (
    <View testID="stats-chart-stacked" style={styles.chartBody}>
      {stats.days.map((day, index) => {
        const height = day.total <= 0 ? 0 : Math.max(6, Math.round((day.total / max) * 100));
        const segmentTotal = day.segments.reduce((total, value) => total + value, 0);
        return (
          <View key={day.key} style={styles.column}>
            <Text testID={`stats-day-value-${index}`} style={styles.dayValue}>
              {displayValue(metric, day.total)}
            </Text>
            <View style={styles.barWrap}>
              <View style={[styles.stack, { height: `${height}%` as DimensionValue }]}>
                {day.segments.map((value, segmentIndex) => (
                  <View
                    key={config.segments[segmentIndex].key}
                    testID={`stats-stack-${index}-${segmentIndex}`}
                    style={{
                      height: `${segmentTotal > 0 ? (value / segmentTotal) * 100 : 0}%` as DimensionValue,
                      backgroundColor: config.segments[segmentIndex].color,
                    }}
                  />
                ))}
              </View>
            </View>
            <Text style={[styles.dayLabel, day.isToday && styles.dayLabelToday]}>
              {day.label}
            </Text>
          </View>
        );
      })}
    </View>
  );
}

function LineChart({ stats, metric }: { stats: CareMetricStats; metric: CareMetric }) {
  const [width, setWidth] = useState(DEFAULT_WIDTH);
  const measure = (event: LayoutChangeEvent) => {
    const next = Math.round(event.nativeEvent.layout.width);
    if (next > 0 && next !== width) {
      setWidth(next);
    }
  };
  const max = chartMax(stats);
  const count = stats.days.length;
  const plotWidth = width - PAD_LEFT - PAD_RIGHT;
  const plotHeight = LINE_HEIGHT - PAD_TOP - PAD_BOTTOM;
  const x = (index: number) => PAD_LEFT + (index * plotWidth) / (count - 1);
  const y = (value: number) => PAD_TOP + (1 - value / max) * plotHeight;
  const points = stats.days
    .map((day, index) => `${x(index).toFixed(1)},${y(day.total).toFixed(1)}`)
    .join(' ');
  const ticks = [0, 0.5, 1].map((ratio) => ({
    value: max * (1 - ratio),
    y: PAD_TOP + ratio * plotHeight,
  }));

  return (
    <View testID="stats-chart-line" onLayout={measure} style={styles.lineBody}>
      <Svg width={width} height={LINE_HEIGHT}>
        {ticks.map((tick) => (
          <Line
            key={`grid-${tick.y}`}
            x1={PAD_LEFT}
            y1={tick.y}
            x2={width - PAD_RIGHT}
            y2={tick.y}
            stroke={colors.border}
            strokeWidth={1}
            opacity={0.45}
          />
        ))}
        {ticks.map((tick, index) => (
          <SvgText
            key={`tick-${tick.y}`}
            testID={`stats-line-tick-${index}`}
            x={PAD_LEFT - 5}
            y={tick.y + 3}
            textAnchor="end"
            fontSize={9}
            fill={colors.muted}
          >
            {displayTickValue(metric, tick.value)}
          </SvgText>
        ))}
        <Polyline
          points={points}
          fill="none"
          stroke={colors.accent}
          strokeWidth={2}
          strokeLinecap="round"
          strokeLinejoin="round"
        />
        {stats.days.map((day, index) => (
          <Circle
            key={day.key}
            testID={`stats-line-point-${index}`}
            cx={x(index)}
            cy={y(day.total)}
            r={day.isToday ? 4 : 3}
            fill={day.isToday ? colors.accent : colors.background}
            stroke={colors.accent}
            strokeWidth={1.5}
          />
        ))}
        {stats.days.map((day, index) => (
          <SvgText
            key={`value-${day.key}`}
            x={x(index)}
            y={y(day.total) - 7}
            textAnchor="middle"
            fontSize={8}
            fill={colors.muted}
          >
            {displayValue(metric, day.total)}
          </SvgText>
        ))}
        {stats.days.map((day, index) => (
          <SvgText
            key={`label-${day.key}`}
            x={x(index)}
            y={LINE_HEIGHT - 7}
            textAnchor="middle"
            fontSize={8}
            fill={colors.muted}
          >
            {day.label}
          </SvgText>
        ))}
      </Svg>
    </View>
  );
}

function chartMax(stats: CareMetricStats): number {
  return Math.max(1, ...stats.days.map((day) => day.total));
}

function displayValue(metric: CareMetric, value: number): string {
  return metric === 'sleep' ? value.toFixed(1) : String(Math.round(value));
}

function displayTickValue(metric: CareMetric, value: number): string {
  if (metric === 'sleep') {
    const withTwoDecimals = value.toFixed(2);
    return withTwoDecimals.endsWith('0') ? value.toFixed(1) : withTwoDecimals;
  }
  return Number.isInteger(value) ? String(Math.round(value)) : value.toFixed(1);
}

const styles = StyleSheet.create({
  card: {
    gap: 8,
    padding: 10,
    borderWidth: 1,
    borderColor: colors.border,
    borderRadius: 12,
  },
  cardHead: {
    flexDirection: 'row',
    alignItems: 'baseline',
    justifyContent: 'space-between',
    gap: 8,
  },
  cardTitle: { flex: 1, fontSize: 12, color: colors.text, fontWeight: '600' },
  unit: { fontSize: 9, color: colors.muted },
  chartBody: {
    height: 210,
    flexDirection: 'row',
    alignItems: 'flex-end',
    gap: 7,
    paddingTop: 18,
  },
  column: { flex: 1, minWidth: 0, height: '100%', alignItems: 'center', gap: 4 },
  dayValue: { fontSize: 9, color: colors.muted },
  barWrap: { flex: 1, width: '100%', alignItems: 'center', justifyContent: 'flex-end' },
  bar: { width: '100%', maxWidth: 24, backgroundColor: CHART_COLORS.bar, borderRadius: 4 },
  barToday: { backgroundColor: colors.accent },
  stack: {
    width: '100%',
    maxWidth: 24,
    flexDirection: 'column-reverse',
    borderRadius: 4,
    overflow: 'hidden',
  },
  dayLabel: { fontSize: 9, color: colors.muted },
  dayLabelToday: { color: colors.accent, fontWeight: '600' },
  legend: { flexDirection: 'row', flexWrap: 'wrap', gap: 6, paddingHorizontal: 2 },
  legendItem: { flexDirection: 'row', alignItems: 'center', gap: 4 },
  legendDot: { width: 7, height: 7, borderRadius: 2 },
  legendText: { fontSize: 9, color: colors.muted },
  lineBody: { height: LINE_HEIGHT },
  caption: {
    gap: 4,
    padding: 10,
    borderWidth: 1,
    borderColor: colors.border,
    borderRadius: 10,
    backgroundColor: colors.surface,
  },
  captionTitle: { fontSize: 12, color: colors.text, fontWeight: '600' },
  disclaimer: { fontSize: 10, color: colors.muted },
  captionBody: { fontSize: 10, lineHeight: 16, color: colors.muted },
  captionRow: { flexDirection: 'row', flexWrap: 'wrap', gap: 6, marginTop: 2 },
  fit: {
    fontSize: 9,
    color: colors.ok,
    backgroundColor: CHART_COLORS.fitBackground,
    borderWidth: 1,
    borderColor: CHART_COLORS.fitBorder,
    borderRadius: 999,
    paddingHorizontal: 7,
    paddingVertical: 2,
  },
  notFit: {
    fontSize: 9,
    color: colors.fail,
    backgroundColor: CHART_COLORS.notFitBackground,
    borderWidth: 1,
    borderColor: CHART_COLORS.notFitBorder,
    borderRadius: 999,
    paddingHorizontal: 7,
    paddingVertical: 2,
  },
  recommend: {
    borderLeftWidth: 3,
    borderLeftColor: colors.accent,
    paddingLeft: 9,
    paddingVertical: 2,
    fontSize: 10,
    lineHeight: 16,
    color: colors.muted,
  },
});
