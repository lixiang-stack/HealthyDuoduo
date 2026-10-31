// 原始图像定位(趋势 → 原图):按报告 + 指标名/单位找到指标项的 OCR 行,
// 用 LineZoom 裁到对应区域并描框;识别值只作辅助,打不开原图时仍保留识别值。
import { Link, Stack, useLocalSearchParams } from 'expo-router';
import { useState } from 'react';
import {
  ActivityIndicator,
  Image,
  type ImageStyle,
  Pressable,
  ScrollView,
  StyleSheet,
  Text,
  View,
} from 'react-native';

import { LineZoom } from '../../components/LineZoom';
import { useImageViewer } from '../../components/imageViewer/ImageViewerProvider';
import { sharedStyles } from '../../components/styles';
import { colors } from '../../components/theme';
import { FLAG_COLORS } from '../../reports/colors';
import { itemFlagLabel, refRangeLabel } from '../../reports/status';
import { useReportData } from '../../reports/useReportData';
import { displayItemName, displayItemUnit } from '../../review/items';

/** 原图定位视口高(px):约占半屏,行文可读到字级。 */
const FOCUS_IMAGE_HEIGHT = 420;

export default function ReportImageScreen() {
  const params = useLocalSearchParams();
  const reportId = typeof params.report_id === 'string' ? params.report_id : undefined;
  const name = typeof params.name === 'string' ? params.name : undefined;
  const unit = typeof params.unit === 'string' ? params.unit : '';
  const reportDate =
    typeof params.report_date === 'string' && params.report_date !== ''
      ? params.report_date
      : undefined;
  const { state, reload } = useReportData(reportId);
  const { open } = useImageViewer();
  const [full, setFull] = useState(false);
  const [fullFailed, setFullFailed] = useState(false);

  const report = state.status === 'ready' ? state.report : undefined;
  const item =
    report && name
      ? (report.items.find(
          (candidate) => candidate.name === name && (candidate.unit ?? '') === unit,
        ) ?? report.items.find((candidate) => candidate.name === name))
      : undefined;
  const itemDisplayName = item ? displayItemName(item) : undefined;
  const itemDisplayUnit = item ? displayItemUnit(item) : null;
  const hasFocus = item !== undefined && item.ocr_lines.length > 0;
  const focusItem = hasFocus ? item : undefined;

  return (
    <>
      <Stack.Screen options={{ title: '原始图像' }} />
      <ScrollView style={styles.container} contentContainerStyle={styles.content}>
        {state.status === 'loading' ? (
          <View style={sharedStyles.horizontal}>
            <ActivityIndicator />
            <Text style={sharedStyles.muted}>正在加载原始图像…</Text>
          </View>
        ) : null}

        {state.status === 'unconfigured' ? (
          <Text style={sharedStyles.muted}>
            尚未配置后端地址,请先到
            <Link href="/settings" style={styles.link}>
              设置
            </Link>
            填写。
          </Text>
        ) : null}

        {state.status === 'error' ? (
          <View style={styles.stack}>
            <Text style={styles.fail}>加载失败:{state.detail}</Text>
            <Pressable style={styles.secondaryButton} onPress={() => void reload()}>
              <Text style={styles.secondaryText}>重试</Text>
            </Pressable>
          </View>
        ) : null}

        {state.status === 'ready' ? (
          <View style={styles.stack}>
            <Text testID="report-image-meta" style={sharedStyles.muted}>
              {[reportDate ?? state.report.report_date, state.report.report_type, itemDisplayName]
                .filter(Boolean)
                .join(' · ')}
            </Text>

            {full || !focusItem ? (
              <>
                <Pressable
                  testID="report-image-full-open"
                  accessibilityRole="button"
                  accessibilityLabel="放大查看检查单原图"
                  onPress={() => open(state.imageUrl, '检查单原图')}
                >
                  <Image
                    testID="report-image-full"
                    source={{ uri: state.imageUrl }}
                    style={styles.fullImage as ImageStyle}
                    resizeMode="contain"
                    onError={() => setFullFailed(true)}
                  />
                </Pressable>
                {fullFailed ? (
                  <Text style={styles.fail}>图像载入失败,以识别值为准。</Text>
                ) : null}
              </>
            ) : (
              <LineZoom
                testID="report-image-focus"
                imageUrl={state.imageUrl}
                lines={focusItem.ocr_lines}
                height={FOCUS_IMAGE_HEIGHT}
                highlight
              />
            )}

            <Text style={styles.hint}>
              {full
                ? '整张检查单;点按放大,点「定位到该项」回到该指标行。'
                : hasFocus
                  ? '蓝框 = 该指标在原始图像中的位置;点按可放大,原图优先,识别值只作辅助。'
                  : item
                    ? '这项没有原文行定位,已显示整张检查单;点按可放大。'
                    : '没有找到对应指标,已显示整张检查单;点按可放大。'}
            </Text>

            {item ? (
              <View testID="report-image-value" style={styles.valueCard}>
                <View style={styles.valueHeader}>
                  <Text style={styles.itemName}>{itemDisplayName}</Text>
                  <Text style={[styles.flag, { color: FLAG_COLORS[item.flag] }]}>
                    {itemFlagLabel(item)}
                  </Text>
                </View>
                <View style={styles.valueRow}>
                  <Text style={[styles.value, { color: FLAG_COLORS[item.flag] }]}>
                    {item.value ?? '—'}
                    {itemDisplayUnit ? <Text style={styles.unit}> {itemDisplayUnit}</Text> : null}
                  </Text>
                  <Text style={sharedStyles.muted}>{refRangeLabel(item.ref_range)}</Text>
                </View>
                <Text style={sharedStyles.muted}>
                  {[reportDate ?? state.report.report_date, state.report.report_type]
                    .filter(Boolean)
                    .join(' · ')}
                  {item.low_confidence ? ' · 低置信' : ''}
                </Text>
              </View>
            ) : null}

            <View style={styles.actions}>
              {hasFocus ? (
                <Pressable
                  testID="report-image-focus-toggle"
                  style={styles.secondaryButton}
                  onPress={() => setFull((prev) => !prev)}
                >
                  <Text style={styles.secondaryText}>
                    {full ? '定位到该项' : '查看整张检查单'}
                  </Text>
                </Pressable>
              ) : null}
              <Link asChild href={{ pathname: '/report/[report_id]', params: { report_id: reportId } }}>
                <Pressable testID="report-image-open-report" style={styles.primaryButton}>
                  <Text style={styles.primaryText}>打开格式化报告 ›</Text>
                </Pressable>
              </Link>
            </View>
          </View>
        ) : null}
      </ScrollView>
    </>
  );
}

const styles = StyleSheet.create({
  container: { flex: 1, backgroundColor: colors.background },
  content: { padding: 16, gap: 12, paddingBottom: 32 },
  stack: { gap: 12 },
  fullImage: {
    width: '100%',
    height: FOCUS_IMAGE_HEIGHT,
    borderWidth: 1,
    borderColor: colors.border,
    borderRadius: 8,
    backgroundColor: colors.background,
  },
  hint: { fontSize: 12, color: colors.muted, lineHeight: 18 },
  valueCard: {
    borderWidth: 1,
    borderColor: colors.border,
    borderRadius: 8,
    padding: 12,
    gap: 4,
  },
  valueHeader: { flexDirection: 'row', alignItems: 'center', justifyContent: 'space-between', gap: 8 },
  itemName: { flex: 1, fontSize: 15, color: colors.text, fontWeight: '600' },
  flag: { fontSize: 13 },
  valueRow: { flexDirection: 'row', alignItems: 'baseline', justifyContent: 'space-between', gap: 8 },
  value: { fontSize: 22, fontWeight: '600', fontVariant: ['tabular-nums'] },
  unit: { fontSize: 13, fontWeight: '400', color: colors.muted },
  actions: { flexDirection: 'row', gap: 8, flexWrap: 'wrap' },
  primaryButton: {
    flex: 1,
    minWidth: 160,
    paddingVertical: 12,
    paddingHorizontal: 16,
    borderRadius: 8,
    backgroundColor: colors.accent,
    alignItems: 'center',
  },
  primaryText: { color: colors.onAccent, fontSize: 15, fontWeight: '600' },
  secondaryButton: {
    paddingVertical: 12,
    paddingHorizontal: 16,
    borderRadius: 8,
    borderWidth: 1,
    borderColor: colors.border,
    alignItems: 'center',
  },
  secondaryText: { color: colors.text, fontSize: 15 },
  link: { color: colors.accent, fontSize: 15 },
  fail: { fontSize: 13, color: colors.fail, lineHeight: 20 },
});
