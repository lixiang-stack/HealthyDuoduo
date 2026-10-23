// 迷你趋势图(票 03):高 64、无轴;只画有日期 + 标量点,参考带按点分段,
// 实心 = reviewed || !low_confidence,空心 = 未核对低置信;最新点放大。
// 宽度经 onLayout 测量(卡片有 padding/border,不能写死屏幕宽度)。
import { useState } from 'react';
import { StyleSheet, View, type LayoutChangeEvent } from 'react-native';
import Svg, { Circle, Polyline, Rect } from 'react-native-svg';

import { colors } from '../components/theme';
import { FLAG_COLORS } from './colors';
import { chartLayout, type TrendSeries } from './trends';

const DEFAULT_WIDTH = 320;
const HEIGHT = 64;

export function TrendSparkline({
  series,
  name,
  testID,
}: {
  series: TrendSeries;
  name: string;
  testID?: string;
}) {
  const [width, setWidth] = useState(DEFAULT_WIDTH);
  const layout = chartLayout(series, width, HEIGHT, { axes: false });

  const measure = (event: LayoutChangeEvent) => {
    const next = Math.round(event.nativeEvent.layout.width);
    if (next > 0 && next !== width) {
      setWidth(next);
    }
  };

  return (
    <View
      testID={testID}
      accessible
      accessibilityRole="image"
      accessibilityLabel={`${name}趋势，${series.chartPoints.length} 个数值点`}
      onLayout={measure}
      style={styles.container}
    >
      <Svg width={width} height={HEIGHT}>
        {layout.bands.map((band, index) => (
          <Rect
            key={`band-${index}`}
            x={band.x}
            y={band.y}
            width={band.width}
            height={band.height}
            fill={colors.border}
            opacity={0.45}
          />
        ))}
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
        {layout.points.map((point) => (
          <Circle
            key={point.reportId}
            cx={point.x}
            cy={point.y}
            r={point.latest ? 4 : 3}
            fill={point.filled ? FLAG_COLORS[point.flag] : colors.background}
            stroke={FLAG_COLORS[point.flag]}
            strokeWidth={point.filled ? 1 : 2}
          />
        ))}
      </Svg>
    </View>
  );
}

const styles = StyleSheet.create({
  container: { height: HEIGHT },
});
