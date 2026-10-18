import { Link, useLocalSearchParams } from 'expo-router';
import { useState } from 'react';
import { ActivityIndicator, Image, Pressable, ScrollView, StyleSheet, Text, View } from 'react-native';

import { colors } from '../../components/theme';
import { ReportCard } from '../../reports/components';
import { pendingItems } from '../../review/items';
import { ReviewItemCard } from '../../review/components';
import { useReportData } from '../../reports/useReportData';

// 报告详情页(票 07;票 09 起为核对闭环的「详情顺手」入口):
// 头部含报告短 ID(前 12 hex,问题定位直接引用);每个指标项经 ReviewItemCard
// 可确认/修正(低置信与高置信一视同仁);有剩余待核对项时给出核对页入口;
// failed 展示图像与失败原因(无可核对项,出口 = 重新拍照)。
/** 详情页头部短 ID 长度(前 12 hex,git 风格短标识;完整值即 report_id)。 */
const SHORT_ID_LENGTH = 12;

export default function ReportDetailScreen() {
  const params = useLocalSearchParams();
  const reportId = typeof params.report_id === 'string' ? params.report_id : undefined;
  const { state, reload, applyReview } = useReportData(reportId);
  // undefined = 按报告状态缺省(failed 展开、其余折叠);点按后由用户覆盖。
  const [imageOverride, setImageOverride] = useState<boolean>();

  const report = state.status === 'ready' ? state.report : undefined;
  const pending = report ? pendingItems(report.items) : [];
  const imageVisible =
    report !== undefined ? (imageOverride ?? report.status === 'failed') : false;

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

      {state.status === 'ready' && report ? (
        <View style={styles.stack}>
          <ReportCard report={report} />
          <Text style={styles.shortId}>
            报告 {report.report_id.slice(0, SHORT_ID_LENGTH)}
          </Text>

          {report.status === 'failed' ? (
            <View style={styles.card}>
              <Text style={styles.muted}>这张检查单没有识别出可核对的指标项。</Text>
              <Link href="/upload" style={styles.link}>
                重新拍照上传(生成新报告,原报告与识别记录保留)
              </Link>
            </View>
          ) : null}

          {pending.length > 0 ? (
            <Link
              href={{ pathname: '/review/[report_id]', params: { report_id: report.report_id } }}
              style={styles.link}
            >
              逐项核对 {pending.length} 项低置信
            </Link>
          ) : null}

          {report.items.length > 0 ? (
            <Text style={styles.sectionTitle}>指标项({report.items.length})</Text>
          ) : null}
          {report.items.map((item) => (
            <ReviewItemCard
              key={item.item_id}
              item={item}
              reportId={report.report_id}
              client={state.client}
              imageUrl={state.imageUrl}
              onUpdated={applyReview}
            />
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
  shortId: { fontSize: 12, color: colors.muted },
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
