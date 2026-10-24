// 趋势图(票 03 迷你 / 票 04 完整):只画有日期 + 标量点,参考带按点分段,
// 实心 = reviewed || !low_confidence,空心 = 未核对低置信。
// 迷你图高 64、无轴、放大最新点;完整图高 180、有轴、放大选中点。
// 宽度经 onLayout 测量(卡片有 padding/border,不能写死屏幕宽度)。
import { useState } from 'react';
import { StyleSheet, View, type LayoutChangeEvent } from 'react-native';
import Svg, { Circle, G, Line, Polyline, Rect, Text as SvgText } from 'react-native-svg';

import { colors } from '../components/theme';
import { FLAG_COLORS } from './colors';
import {
  chartLayout,
  type TrendChartBand,
  type TrendChartPoint,
  type TrendSeries,
} from './trends';

const DEFAULT_WIDTH = 320;
const SPARKLINE_HEIGHT = 64;
const DETAIL_HEIGHT = 180;
const POINT_RADIUS = 3;
const LATEST_RADIUS = 4;
const SELECTED_RADIUS = 5;

function useChartWidth() {
  const [width, setWidth] = useState(DEFAULT_WIDTH);
  const measure = (event: LayoutChangeEvent) => {
    const next = Math.round(event.nativeEvent.layout.width);
    if (next > 0 && next !== width) {
      setWidth(next);
    }
  };
  return { width, measure };
}

function bandRects(bands: TrendChartBand[]) {
  return bands.map((band, index) => (
    <Rect
      key={`band-${index}`}
      x={band.x}
      y={band.y}
      width={band.width}
      height={band.height}
      fill={colors.border}
      opacity={0.45}
    />
  ));
}

function pointCircles(points: TrendChartPoint[], emphasize: 'latest' | 'selected') {
  return points.map((point) => {
    const enlarged = emphasize === 'latest' ? point.latest : point.selected;
    const radius = enlarged
      ? emphasize === 'latest'
        ? LATEST_RADIUS
        : SELECTED_RADIUS
      : POINT_RADIUS;
    return (
      <Circle
        key={point.reportId}
        cx={point.x}
        cy={point.y}
        r={radius}
        fill={point.filled ? FLAG_COLORS[point.flag] : colors.background}
        stroke={FLAG_COLORS[point.flag]}
        strokeWidth={point.filled ? 1 : 2}
      />
    );
  });
}

export function TrendSparkline({
  series,
  name,
  testID,
}: {
  series: TrendSeries;
  name: string;
  testID?: string;
}) {
  const { width, measure } = useChartWidth();
  const layout = chartLayout(series, width, SPARKLINE_HEIGHT, { axes: false });

  return (
    <View
      testID={testID}
      accessible
      accessibilityRole="image"
      accessibilityLabel={`${name}趋势，${series.chartPoints.length} 个数值点`}
      onLayout={measure}
      style={styles.sparkline}
    >
      <Svg width={width} height={SPARKLINE_HEIGHT}>
        {bandRects(layout.bands)}
        {layout.line ? (
          <Polyline
            points={layout.line}
            fill="none"
            stroke={colors.muted}
            strokeWidth={1.5}
            strokeLinejoin="round"
            strokeLinecap="round"
          />
        ) : null}
        {pointCircles(layout.points, 'latest')}
      </Svg>
    </View>
  );
}

export function TrendDetailChart({
  series,
  name,
  selectedIndex,
  testID,
}: {
  series: TrendSeries;
  name: string;
  selectedIndex: number | null;
  testID?: string;
}) {
  const { width, measure } = useChartWidth();
  const layout = chartLayout(series, width, DETAIL_HEIGHT, {
    axes: true,
    selectedIndex,
  });

  return (
    <View
      testID={testID}
      accessible
      accessibilityRole="image"
      accessibilityLabel={`${name}完整趋势`}
      onLayout={measure}
      style={styles.detail}
    >
      <Svg width={width} height={DETAIL_HEIGHT}>
        {layout.yTicks.map((tick) => (
          <G key={`tick-${tick.value}`}>
            <Line
              x1={layout.plot.left}
              y1={tick.y}
              x2={layout.plot.right}
              y2={tick.y}
              stroke={colors.border}
              strokeWidth={1}
            />
            <SvgText
              x={layout.plot.left - 6}
              y={tick.y + 3}
              textAnchor="end"
              fontSize={10}
              fill={colors.muted}
            >
              {tick.label}
            </SvgText>
          </G>
        ))}
        {bandRects(layout.bands)}
        {layout.line ? (
          <Polyline
            points={layout.line}
            fill="none"
            stroke={colors.muted}
            strokeWidth={1.8}
            strokeLinejoin="round"
            strokeLinecap="round"
          />
        ) : null}
        {pointCircles(layout.points, 'selected')}
        {layout.xLabels.map((label) => (
          <SvgText
            key={`x-${label.label}-${label.anchor}`}
            x={label.x}
            y={DETAIL_HEIGHT - 6}
            textAnchor={label.anchor}
            fontSize={10}
            fill={colors.muted}
          >
            {label.label}
          </SvgText>
        ))}
      </Svg>
    </View>
  );
}

const styles = StyleSheet.create({
  sparkline: { height: SPARKLINE_HEIGHT },
  detail: { height: DETAIL_HEIGHT },
});
