// 趋势详情层(票 04):全屏 Modal,复用列表已加载的 series/listPoints,不额外请求。
// 完整图与逐点列表共享 selectedIndex;点列表是主选点方式,图上的小圆点不做点击热区。
// 点卡「查看原始图像」进原图路由;跳转时 Modal 隐藏但保持挂载,返回后原位恢复选中点。
import { useState } from 'react';
import { Modal, Pressable, ScrollView, StyleSheet, Text, View } from 'react-native';
import { SafeAreaProvider, SafeAreaView } from 'react-native-safe-area-context';

import type { TrendPoint } from '../api/reports';
import { colors } from '../components/theme';
import { FLAG_COLORS } from './colors';
import { flagLabel, DATE_UNKNOWN_LABEL, refRangeLabel } from './status';
import { TrendDetailChart } from './TrendChart';
import {
  defaultSelectedIndex,
  formatSeriesMeta,
  isFilledPoint,
  trendPointRows,
  trendUnavailableReason,
  TREND_UNAVAILABLE_LABEL,
  type TrendSeries,
} from './trends';

function trustLabel(point: TrendPoint): string {
  if (point.reviewed) {
    return '已核对（实心）';
  }
  return point.low_confidence ? '未核对低置信（空心）' : '高置信（实心）';
}

function PointBadge({ point }: { point: TrendPoint }) {
  const color = FLAG_COLORS[point.flag];
  return (
    <View
      style={[
        styles.dot,
        isFilledPoint(point) ? { backgroundColor: color } : styles.dotHollow,
        { borderColor: color },
      ]}
    />
  );
}

export function TrendDetailSheet({
  name,
  reportType,
  reportDate,
  latestValue,
  series,
  visible = true,
  initialReportId,
  onClose,
  onOpenImage,
}: {
  name: string;
  reportType: string;
  reportDate: string | null;
  latestValue: string | null;
  series: TrendSeries;
  /** 原图路由打开时隐藏 Modal 但保留组件与选中点,返回后原位恢复。 */
  visible?: boolean;
  /** 从原图返回时恢复的原报告;缺省用 series 的 selectedPoint。 */
  initialReportId?: string;
  onClose: () => void;
  onOpenImage: (point: TrendPoint) => void;
}) {
  const [selectedIndex, setSelectedIndex] = useState(() =>
    defaultSelectedIndex(series, initialReportId),
  );
  const selectedPoint = series.listPoints[selectedIndex];
  // 选中点若不是可绘制点(非数值/缺日期),图中没有对应标记:保持无放大,避免高亮与点卡不一致。
  const selectedChartIndex = selectedPoint
    ? series.chartPoints.findIndex((point) => point.report_id === selectedPoint.report_id)
    : -1;
  const reason = trendUnavailableReason(series, {
    reportDate,
    value: latestValue,
  });

  return (
    <Modal
      visible={visible}
      animationType="slide"
      transparent={false}
      onRequestClose={onClose}
    >
      <SafeAreaProvider style={styles.safe}>
        <SafeAreaView testID="trend-detail" style={styles.safe}>
          <View style={styles.header}>
            <Pressable
              testID="trend-detail-close"
              accessibilityRole="button"
              accessibilityLabel="关闭"
              onPress={onClose}
              style={styles.closeButton}
            >
              <Text style={styles.close}>‹</Text>
            </Pressable>
            <Text style={styles.title} numberOfLines={1}>
              {name}
            </Text>
          </View>

          <ScrollView contentContainerStyle={styles.content}>
            <Text style={styles.headerMeta}>
              {`最新：${reportType} · ${series.unit ?? '无单位'}`}
            </Text>

            {series.chartPoints.length > 0 ? (
              <TrendDetailChart
                series={series}
                name={name}
                selectedIndex={selectedChartIndex >= 0 ? selectedChartIndex : null}
                testID="trend-detail-chart"
              />
            ) : (
              <View style={styles.emptyChart}>
                <Text style={styles.emptyTitle}>{TREND_UNAVAILABLE_LABEL}</Text>
                {reason ? <Text style={styles.muted}>{reason}</Text> : null}
              </View>
            )}

            {selectedPoint ? (
              <PointCard point={selectedPoint} onOpenImage={onOpenImage} />
            ) : null}

            {series.listPoints.length > 0 ? (
              <View style={styles.pointList}>
                {trendPointRows(series.listPoints).map(({ point, dateLabel }, index) => (
                  <Pressable
                    key={point.report_id}
                    testID={`trend-point-${index}`}
                    accessibilityRole="button"
                    onPress={() => setSelectedIndex(index)}
                    style={[styles.pointRow, index === selectedIndex && styles.pointRowSelected]}
                  >
                    <PointBadge point={point} />
                    <Text style={styles.pointRowDate}>{dateLabel ?? ''}</Text>
                    <Text style={styles.pointRowValue}>
                      {point.value ?? '—'}
                      {point.unit ? ` ${point.unit}` : ''}
                    </Text>
                    <Text style={[styles.pointRowFlag, { color: FLAG_COLORS[point.flag] }]}>
                      {flagLabel(point.flag)}
                    </Text>
                  </Pressable>
                ))}
              </View>
            ) : null}

            <View style={styles.legend}>
              <View style={styles.legendRow}>
                <View style={[styles.dot, styles.legendDotFilled]} />
                <Text style={styles.muted}>实心 = 高置信 / 已核对</Text>
              </View>
              <View style={styles.legendRow}>
                <View style={[styles.dot, styles.dotHollow, styles.legendDot]} />
                <Text style={styles.muted}>空心 = 未核对低置信</Text>
              </View>
              <View testID="trend-band-legend" style={styles.legendRow}>
                <View style={styles.legendBand} />
                <Text style={styles.muted}>灰色带 = 参考范围</Text>
              </View>
            </View>
            <Text style={styles.muted}>点按任一行切换选中点。</Text>

            <Text style={styles.meta}>{formatSeriesMeta(series, { reportDate })}</Text>
          </ScrollView>
        </SafeAreaView>
      </SafeAreaProvider>
    </Modal>
  );
}

function PointCard({
  point,
  onOpenImage,
}: {
  point: TrendPoint;
  onOpenImage: (point: TrendPoint) => void;
}) {
  const color = FLAG_COLORS[point.flag];
  return (
    <View style={styles.pointCard}>
      <View style={styles.pointCardHeader}>
        <Text style={styles.pointDate}>{point.report_date ?? DATE_UNKNOWN_LABEL}</Text>
        <Text style={[styles.flag, { color }]}>{flagLabel(point.flag)}</Text>
      </View>
      <View style={styles.valueRow}>
        <Text style={[styles.value, { color }]}>
          {point.value ?? '—'}
          {point.unit ? <Text style={styles.unit}> {point.unit}</Text> : null}
        </Text>
        <Text style={styles.ref}>{refRangeLabel(point.ref_range)}</Text>
      </View>
      <Text style={styles.muted}>{`方法：${point.method ?? '—'}`}</Text>
      <Text style={styles.muted}>{trustLabel(point)}</Text>
      <Pressable
        testID="trend-point-open-image"
        accessibilityRole="button"
        onPress={() => onOpenImage(point)}
        style={styles.openImage}
      >
        <Text style={styles.openImageText}>查看原始图像 ›</Text>
      </Pressable>
    </View>
  );
}

const styles = StyleSheet.create({
  safe: { flex: 1, backgroundColor: colors.background },
  header: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: 4,
    paddingHorizontal: 20,
    paddingVertical: 6,
    borderBottomWidth: 1,
    borderBottomColor: colors.border,
  },
  closeButton: {
    width: 44,
    height: 44,
    marginLeft: -20,
    alignItems: 'center',
    justifyContent: 'center',
  },
  close: { fontSize: 30, color: colors.text, lineHeight: 32 },
  title: { flex: 1, fontSize: 18, color: colors.text },
  content: { padding: 16, gap: 12, paddingBottom: 32 },
  headerMeta: { fontSize: 13, color: colors.muted },
  emptyChart: { paddingVertical: 24, alignItems: 'center', gap: 6 },
  emptyTitle: { fontSize: 15, color: colors.text },
  pointCard: {
    borderWidth: 1,
    borderColor: colors.border,
    borderRadius: 8,
    padding: 12,
    gap: 6,
  },
  pointCardHeader: { flexDirection: 'row', alignItems: 'center', justifyContent: 'space-between' },
  pointDate: { fontSize: 14, color: colors.text },
  flag: { fontSize: 13 },
  valueRow: {
    flexDirection: 'row',
    justifyContent: 'space-between',
    alignItems: 'baseline',
    gap: 8,
  },
  value: { fontSize: 20, fontVariant: ['tabular-nums'] },
  unit: { fontSize: 13 },
  ref: { fontSize: 13, color: colors.muted },
  muted: { fontSize: 13, color: colors.muted, lineHeight: 20 },
  openImage: {
    marginTop: 10,
    borderWidth: 1,
    borderColor: colors.border,
    borderRadius: 8,
    paddingVertical: 11,
    alignItems: 'center',
  },
  openImageText: { color: colors.accent, fontSize: 14, fontWeight: '600' },
  pointList: { gap: 6 },
  pointRow: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: 8,
    borderWidth: 1,
    borderColor: 'transparent',
    borderRadius: 8,
    paddingHorizontal: 8,
    paddingVertical: 8,
  },
  pointRowSelected: { borderColor: colors.accent },
  pointRowDate: { width: 92, fontSize: 12, color: colors.muted },
  pointRowValue: { flex: 1, fontSize: 14, color: colors.text, fontVariant: ['tabular-nums'] },
  pointRowFlag: { width: 44, fontSize: 12, textAlign: 'right' },
  dot: { width: 10, height: 10, borderRadius: 5, borderWidth: 2 },
  dotHollow: { backgroundColor: colors.background },
  legend: { gap: 4 },
  legendRow: { flexDirection: 'row', alignItems: 'center', gap: 8 },
  legendDotFilled: { backgroundColor: colors.text, borderColor: colors.text },
  legendDot: { borderColor: colors.text },
  legendBand: { width: 18, height: 8, borderRadius: 2, backgroundColor: colors.border, opacity: 0.45 },
  meta: { fontSize: 12, color: colors.muted, lineHeight: 18 },
});
