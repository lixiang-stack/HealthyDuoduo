import { Link, useLocalSearchParams } from 'expo-router';
import { ActivityIndicator, Image, Pressable, ScrollView, StyleSheet, Text, View } from 'react-native';

import { sharedStyles } from '../../components/styles';
import { colors } from '../../components/theme';
import { useImageViewer } from '../../components/imageViewer/ImageViewerProvider';
import { ReportCard } from '../../reports/components';
import { useReportData } from '../../reports/useReportData';
import { ReviewItemCard } from '../../review/components';
import { pendingItems } from '../../review/items';

// 核对页(票 09):上传当场(首选,纸还在手边)与详情顺手(兜底)双入口的同一闭环。
// 待核对项(未核对低置信项 + 可操作未知项)逐项列表 + 确认/修正按钮;低置信项三方对照:
// 图像缩放至 OCR 行对应区域 ↔ 原始行文 ↔ 解析值/参考范围。「剩余 N 项」徽标由
// 服务端派生(ReportCard 展示),动作响应即刷新整屏;failed 无可核对项,
// 展示图像与原因(直接深链进入时的兜底形态;常规路径在详情页展示)。
export default function ReviewScreen() {
  const params = useLocalSearchParams();
  const reportId = typeof params.report_id === 'string' ? params.report_id : undefined;
  const { state, reload, applyReview } = useReportData(reportId);
  const { open } = useImageViewer();

  const report = state.status === 'ready' ? state.report : undefined;
  const pending = report ? pendingItems(report.items) : [];

  return (
    <ScrollView style={styles.container} contentContainerStyle={styles.content}>
      {state.status === 'loading' ? (
        <View style={sharedStyles.horizontal}>
          <ActivityIndicator />
          <Text style={sharedStyles.muted}>正在加载报告…</Text>
        </View>
      ) : null}

      {state.status === 'unconfigured' ? (
        <Text style={sharedStyles.muted}>
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

      {state.status === 'ready' && report ? (
        <View style={styles.stack}>
          <ReportCard report={report} />

          {report.status === 'failed' ? (
            <View style={styles.card}>
              <Text style={sharedStyles.muted}>这张检查单没有识别出可核对的指标项。</Text>
              <Pressable
                testID="review-image-open"
                accessibilityRole="button"
                accessibilityLabel="放大查看检查单原图"
                onPress={() => open(state.imageUrl, '检查单原图')}
              >
                <Image source={{ uri: state.imageUrl }} style={styles.image} resizeMode="contain" />
              </Pressable>
              <Text style={styles.hint}>点按原图放大查看</Text>
              <Link href="/upload" style={styles.link}>
                重新拍照上传(生成新报告,原报告与识别记录保留)
              </Link>
            </View>
          ) : pending.length > 0 ? (
            <>
              <Text style={styles.sectionTitle}>逐项对照原图与原文,确信后确认或修正</Text>
              {pending.map((item) => (
                <ReviewItemCard
                  key={item.item_id}
                  item={item}
                  reportId={report.report_id}
                  client={state.client}
                  imageUrl={state.imageUrl}
                  onUpdated={applyReview}
                />
              ))}
              <Text style={sharedStyles.muted}>
                高置信项与全部指标项可在
                <Link
                  href={{ pathname: '/report/[report_id]', params: { report_id: report.report_id } }}
                  style={styles.link}
                >
                  报告详情
                </Link>
                中修改。
              </Text>
            </>
          ) : (
            <View style={styles.card}>
              <Text style={sharedStyles.muted}>没有需要核对的低置信项。</Text>
              <Link
                href={{ pathname: '/report/[report_id]', params: { report_id: report.report_id } }}
                style={styles.link}
              >
                查看报告详情
              </Link>
            </View>
          )}
          <Link asChild href="/timeline">
            <Pressable testID="review-open-timeline">
              <Text style={styles.link}>去档案查看</Text>
            </Pressable>
          </Link>
        </View>
      ) : null}
    </ScrollView>
  );
}

const styles = StyleSheet.create({
  container: { flex: 1, backgroundColor: colors.background },
  content: { padding: 24, gap: 12 },
  stack: { gap: 12 },
  card: { borderWidth: 1, borderColor: colors.border, borderRadius: 8, padding: 12, gap: 8 },
  sectionTitle: { fontSize: 13, color: colors.muted },
  fail: { fontSize: 13, color: colors.fail, lineHeight: 20 },
  link: { color: colors.accent, fontSize: 15 },
  hint: { fontSize: 12, color: colors.muted },
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
