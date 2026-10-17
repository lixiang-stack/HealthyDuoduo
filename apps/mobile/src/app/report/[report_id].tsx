import { Link, useFocusEffect, useLocalSearchParams } from 'expo-router';
import { useCallback, useState } from 'react';
import { ActivityIndicator, Image, Pressable, ScrollView, StyleSheet, Text, View } from 'react-native';

import { getReport, reportImageUrl, type ReportDetail } from '../../api/reports';
import { resolveApiClient } from '../../api/resolveClient';
import { colors } from '../../components/theme';
import { MetricItem, ReportCard } from '../../reports/components';

// 报告详情页(票 07):家长从时间线卡片点入——指标项列表(flag 染色 + 参考范围条带 +
// 原始 OCR 行文对照)、低置信项高亮、图像查看(failed 默认展开,其余点按展开)、
// failed 展示图像与失败原因(无可核对项,出口 = 重新拍照)。
type ScreenState =
  | { status: 'loading' }
  | { status: 'unconfigured' }
  | { status: 'error'; detail: string }
  | { status: 'ready'; report: ReportDetail; imageUrl: string };

export default function ReportDetailScreen() {
  const params = useLocalSearchParams();
  const reportId = typeof params.report_id === 'string' ? params.report_id : undefined;
  const [state, setState] = useState<ScreenState>({ status: 'loading' });
  // undefined = 按报告状态缺省(failed 展开、其余折叠);点按后由用户覆盖。
  const [imageOverride, setImageOverride] = useState<boolean>();

  const reload = useCallback(async () => {
    if (!reportId) {
      setState({ status: 'error', detail: '缺少报告编号' });
      return;
    }
    const resolved = await resolveApiClient();
    if (!resolved) {
      setState({ status: 'unconfigured' });
      return;
    }
    try {
      const report = await getReport(resolved.client, reportId);
      setState({ status: 'ready', report, imageUrl: reportImageUrl(resolved.baseUrl, reportId) });
    } catch (error) {
      setState({ status: 'error', detail: error instanceof Error ? error.message : String(error) });
    }
  }, [reportId]);

  useFocusEffect(
    useCallback(() => {
      void reload();
    }, [reload]),
  );

  const imageVisible =
    state.status === 'ready' ? (imageOverride ?? state.report.status === 'failed') : false;

  return (
    <ScrollView style={styles.container} contentContainerStyle={styles.content}>
      {state.status === 'loading' ? (
        <View style={styles.horizontal}>
          <ActivityIndicator />
          <Text style={styles.muted}>正在加载报告…</Text>
        </View>
      ) : null}

      {state.status === 'unconfigured' ? (
        <Text style={styles.muted}>
          尚未配置后端地址,请先到<Link href="/settings" style={styles.link}>设置</Link>填写。
        </Text>
      ) : null}

      {state.status === 'error' ? (
        <View style={styles.stack}>
          <Text style={styles.fail}>加载失败:{state.detail}</Text>
          <Pressable style={styles.button} onPress={() => void reload()}>
            <Text style={styles.buttonText}>重试</Text>
          </Pressable>
        </View>
      ) : null}

      {state.status === 'ready' ? (
        <View style={styles.stack}>
          <ReportCard report={state.report} />

          {state.report.status === 'failed' ? (
            <View style={styles.card}>
              <Text style={styles.muted}>这张检查单没有识别出可核对的指标项。</Text>
              <Link href="/upload" style={styles.link}>
                重新拍照上传(生成新报告,原报告与识别记录保留)
              </Link>
            </View>
          ) : null}

          {state.report.items.length > 0 ? (
            <Text style={styles.sectionTitle}>指标项({state.report.items.length})</Text>
          ) : null}
          {state.report.items.map((item) => (
            <MetricItem key={item.item_id} item={item} />
          ))}

          <Pressable onPress={() => setImageOverride(!imageVisible)}>
            <Text style={styles.link}>{imageVisible ? '收起检查单原图' : '查看检查单原图'}</Text>
          </Pressable>
          {imageVisible ? (
            <Image source={{ uri: state.imageUrl }} style={styles.image} resizeMode="contain" />
          ) : null}
        </View>
      ) : null}
    </ScrollView>
  );
}

const styles = StyleSheet.create({
  container: { flex: 1, backgroundColor: colors.background },
  content: { padding: 24, gap: 12 },
  stack: { gap: 12 },
  horizontal: { flexDirection: 'row', alignItems: 'center', gap: 8 },
  card: { borderWidth: 1, borderColor: colors.border, borderRadius: 8, padding: 12, gap: 4 },
  sectionTitle: { fontSize: 13, color: colors.muted },
  muted: { fontSize: 13, color: colors.muted, lineHeight: 20 },
  fail: { fontSize: 13, color: colors.fail, lineHeight: 20 },
  link: { color: colors.accent, fontSize: 15 },
  image: {
    width: '100%',
    height: 320,
    borderWidth: 1,
    borderColor: colors.border,
    borderRadius: 8,
  },
  button: {
    alignSelf: 'flex-start',
    paddingVertical: 10,
    paddingHorizontal: 16,
    backgroundColor: colors.accent,
    borderRadius: 8,
  },
  buttonText: { color: colors.onAccent, fontSize: 15 },
});
