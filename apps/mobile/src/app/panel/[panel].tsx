import { Link, router, Stack, useLocalSearchParams } from 'expo-router';
import { useState } from 'react';
import { ActivityIndicator, Pressable, ScrollView, StyleSheet, Text, View } from 'react-native';

import type { PanelItem } from '../../api/reports';
import { sharedStyles } from '../../components/styles';
import { colors } from '../../components/theme';
import { useCurrentMember } from '../../members/CurrentMemberContext';
import { FLAG_COLORS } from '../../reports/colors';
import {
  panelTrendOf,
  parsePanelParam,
  panelSourceLabel,
  splitPanelItems,
  type PanelTrend,
} from '../../reports/panelOverview';
import { panelLabel } from '../../reports/panels';
import { flagLabel, itemFlagLabel, panelDateLabel, refRangeLabel } from '../../reports/status';
import { TrendDetailSheet } from '../../reports/TrendDetailSheet';
import { TrendSparkline } from '../../reports/TrendChart';
import {
  formatSeriesMeta,
  TREND_UNAVAILABLE_LABEL,
} from '../../reports/trends';
import { usePanelDetail } from '../../reports/usePanelDetail';

/** 其余项超过此数时默认折叠(血常规等大项目)。 */
const REST_COLLAPSED_THRESHOLD = 12;

// 项目详情(panel-browse 票 03):该项目全部指标的最新值与跨报告趋势。异常项内联
// 迷你趋势;正常 / 未知 / 未检测项给最新值与趋势入口(points 已随详情返回,不额外请求)。
// 底部跳「包含该项目的报告」;影像项目不进本页(项目列表直接打开报告详情)。
export default function PanelDetailScreen() {
  const params = useLocalSearchParams();
  const panel = parsePanelParam(params.panel);
  const { currentMemberId } = useCurrentMember();
  const { state, reload } = usePanelDetail(currentMemberId, panel);
  const [detailTrend, setDetailTrend] = useState<PanelTrend>();
  const [restExpanded, setRestExpanded] = useState(false);

  const detail = state.status === 'ready' ? state.detail : undefined;
  const memberName = state.status === 'ready' ? state.memberName : undefined;
  const latestFailedReportId =
    state.status === 'ready' ? state.latestFailedReportId : undefined;
  const reportTypeById = state.status === 'ready' ? state.reportTypeById : {};
  const groups = detail ? splitPanelItems(detail.items) : undefined;
  const restCollapsed =
    groups !== undefined &&
    groups.rest.length > REST_COLLAPSED_THRESHOLD &&
    !restExpanded;
  const title = panel ? panelLabel(panel) : '项目详情';

  return (
    <>
      <Stack.Screen options={{ title }} />
      <ScrollView style={styles.container} contentContainerStyle={styles.content}>
        {panel === undefined ? (
          <View style={styles.stack}>
            <Text style={sharedStyles.muted}>这个项目不存在。</Text>
            <Link href="/timeline" style={styles.link}>
              返回档案
            </Link>
          </View>
        ) : (
          <>
            {state.status === 'loading' ? (
              <View style={sharedStyles.horizontal}>
                <ActivityIndicator />
                <Text style={sharedStyles.muted}>正在加载项目详情…</Text>
              </View>
            ) : null}

            {state.status === 'unconfigured' ? (
              <Text style={sharedStyles.muted}>
                尚未配置后端地址,请先到<Link href="/settings" style={styles.link}>设置</Link>填写。
              </Text>
            ) : null}

            {currentMemberId === undefined && state.status !== 'unconfigured' ? (
              <View style={styles.stack}>
                <Text style={sharedStyles.muted}>选择一个成员查看其档案。</Text>
                <Link href="/" style={styles.link}>
                  返回家庭
                </Link>
              </View>
            ) : null}

            {state.status === 'error' ? (
              <View style={styles.stack}>
                <Text style={styles.fail}>加载失败:{state.detail}</Text>
                <Pressable style={styles.button} onPress={() => void reload()}>
                  <Text style={styles.buttonText}>重试</Text>
                </Pressable>
              </View>
            ) : null}

            {detail && groups ? (
              <View style={styles.stack}>
                <Text testID="panel-detail-heading" style={styles.heading}>
                  {title}
                </Text>
                <Text testID="panel-detail-meta" style={sharedStyles.muted}>
                  {[
                    memberName,
                    panelDateLabel(detail.latest_report_date, detail.narrative),
                    `${detail.report_count} 份报告`,
                    `${groups.abnormal.length} 项异常`,
                  ]
                    .filter(Boolean)
                    .join(' · ')}
                </Text>
                {panelSourceLabel(detail.panel, detail.report_types) ? (
                  <Text testID="panel-source-label" style={sharedStyles.muted}>
                    {panelSourceLabel(detail.panel, detail.report_types)}
                  </Text>
                ) : null}
                {latestFailedReportId ? (
                  <Link
                    asChild
                    href={{
                      pathname: '/report/[report_id]',
                      params: { report_id: latestFailedReportId },
                    }}
                  >
                    <Pressable testID="panel-latest-failed">
                      <Text style={styles.fail}>另有更新的一份识别失败报告 ›</Text>
                    </Pressable>
                  </Link>
                ) : null}

                {detail.items.length === 0 ? (
                  <View testID="panel-empty" style={styles.stack}>
                    <Text style={sharedStyles.muted}>这个项目还没有可展示的指标。</Text>
                    <Pressable testID="panel-back-to-panels" onPress={() => router.back()}>
                      <Text style={styles.link}>返回按项目</Text>
                    </Pressable>
                  </View>
                ) : (
                  <>
                    {groups.abnormal.length > 0 ? (
                      <Text testID="panel-abnormal-title" style={styles.sectionTitle}>
                        异常 {groups.abnormal.length} 项
                      </Text>
                    ) : null}
                    {groups.abnormal.map((item, index) => (
                      <AbnormalItemCard
                        key={`${item.name}-${item.unit ?? ''}`}
                        item={item}
                        index={index}
                        reportTypeById={reportTypeById}
                        onOpen={(trend) => setDetailTrend(trend)}
                      />
                    ))}

                    {groups.rest.length > 0 ? (
                      <>
                        <Pressable
                          testID="panel-rest-toggle"
                          disabled={groups.rest.length <= REST_COLLAPSED_THRESHOLD}
                          onPress={() => setRestExpanded((prev) => !prev)}
                        >
                          <Text testID="panel-rest-title" style={styles.sectionTitle}>
                            其余 {groups.rest.length} 项
                            {groups.rest.length > REST_COLLAPSED_THRESHOLD
                              ? restCollapsed
                                ? ' ›'
                                : ' ⌄'
                              : ''}
                          </Text>
                        </Pressable>
                        {!restCollapsed
                          ? groups.rest.map((item, index) => (
                              <CompactItemRow
                                key={`${item.name}-${item.unit ?? ''}`}
                                item={item}
                                index={groups.abnormal.length + index}
                                reportTypeById={reportTypeById}
                                onOpen={(trend) => setDetailTrend(trend)}
                              />
                            ))
                          : null}
                      </>
                    ) : null}
                  </>
                )}

                <Link
                  asChild
                  href={{
                    pathname: '/timeline',
                    params: { panel: detail.panel },
                  }}
                >
                  <Pressable testID="panel-reports-link">
                    <Text style={styles.link}>
                      查看包含该项目的报告（{detail.report_count} 份）›
                    </Text>
                  </Pressable>
                </Link>
              </View>
            ) : null}
          </>
        )}
      </ScrollView>
      {detailTrend ? (
        <TrendDetailSheet
          key={`${detailTrend.reportType}-${detailTrend.name}`}
          name={detailTrend.name}
          reportType={detailTrend.reportType}
          reportDate={detailTrend.reportDate}
          latestValue={detailTrend.latestValue}
          series={detailTrend.series}
          onClose={() => setDetailTrend(undefined)}
        />
      ) : null}
    </>
  );
}

function AbnormalItemCard({
  item,
  index,
  reportTypeById,
  onOpen,
}: {
  item: PanelItem;
  index: number;
  reportTypeById: Record<string, string>;
  onOpen: (trend: PanelTrend) => void;
}) {
  const latest = item.latest!;
  const trend = panelTrendOf(item, reportTypeById)!;
  const color = FLAG_COLORS[latest.flag];
  const lowConfidence = latest.low_confidence && !latest.reviewed;
  return (
    <Pressable
      testID={`panel-item-${index}`}
      accessibilityRole="button"
      onPress={() => onOpen(trend)}
      style={styles.card}
    >
      <View style={styles.cardHeader}>
        <Text style={styles.itemName}>{item.name}</Text>
        {lowConfidence ? <Text style={styles.lowTag}>低置信</Text> : null}
        <Text style={[styles.flag, { color }]}>{flagLabel(latest.flag)}</Text>
      </View>
      <View style={styles.valueRow}>
        <View style={styles.valueWithTag}>
          <Text style={[styles.value, { color }]}>
            {latest.value ?? '—'}
            {latest.unit ? <Text style={styles.unit}> {latest.unit}</Text> : null}
          </Text>
          {item.latest_report_not_tested ? (
            <Text testID={`panel-item-not-tested-${index}`} style={styles.notTestedTag}>
              最近一次未检测
            </Text>
          ) : null}
        </View>
        <Text style={styles.ref}>{refRangeLabel(item.ref_range)}</Text>
      </View>
      {trend.series.chartPoints.length === 0 ? (
        <Text style={styles.emptyChart}>{TREND_UNAVAILABLE_LABEL}</Text>
      ) : (
        <TrendSparkline
          series={trend.series}
          name={item.name}
          testID={`panel-item-sparkline-${index}`}
        />
      )}
      <Text style={styles.meta}>{formatSeriesMeta(trend.series, trend)}</Text>
      <Text style={styles.detailLink}>查看趋势 ›</Text>
    </Pressable>
  );
}

function CompactItemRow({
  item,
  index,
  reportTypeById,
  onOpen,
}: {
  item: PanelItem;
  index: number;
  reportTypeById: Record<string, string>;
  onOpen: (trend: PanelTrend) => void;
}) {
  const latest = item.latest;
  const trend = panelTrendOf(item, reportTypeById);
  const notTested = latest === null && item.not_tested_count > 0;
  const color = latest ? FLAG_COLORS[latest.flag] : colors.muted;
  const flagText = latest ? itemFlagLabel({ flag: latest.flag }) : notTested ? '未检测' : '—';
  const lowConfidence = latest !== null && latest.low_confidence && !latest.reviewed;
  const value = latest?.value ?? (notTested ? '未检测' : '—');

  const content = (
    <>
      <View style={styles.compactHeader}>
        <Text style={styles.itemName}>{item.name}</Text>
        {lowConfidence ? <Text style={styles.lowTag}>低置信</Text> : null}
        <Text style={[styles.flag, { color }]}>{flagText}</Text>
      </View>
      <View style={styles.valueRow}>
        <View style={styles.valueWithTag}>
          <Text style={[styles.compactValue, { color }]}>
            {value}
            {latest?.unit ? <Text style={styles.unit}> {latest.unit}</Text> : null}
          </Text>
          {item.latest_report_not_tested && latest ? (
            <Text testID={`panel-item-not-tested-${index}`} style={styles.notTestedTag}>
              最近一次未检测
            </Text>
          ) : null}
        </View>
        <Text style={styles.ref}>{latest ? refRangeLabel(item.ref_range) : ''}</Text>
      </View>
    </>
  );

  if (!trend) {
    return (
      <View testID={`panel-item-${index}`} style={styles.compactRow}>
        {content}
      </View>
    );
  }
  return (
    <Pressable
      testID={`panel-item-${index}`}
      accessibilityRole="button"
      onPress={() => onOpen(trend)}
      style={styles.compactRow}
    >
      {content}
      <Text style={styles.detailLink}>查看趋势 ›</Text>
    </Pressable>
  );
}

const styles = StyleSheet.create({
  container: { flex: 1, backgroundColor: colors.background },
  content: { padding: 24, gap: 12, paddingBottom: 32 },
  stack: { gap: 12 },
  heading: { fontSize: 22, color: colors.text, fontWeight: '600' },
  sectionTitle: { fontSize: 13, color: colors.muted, marginTop: 4 },
  card: {
    borderWidth: 1,
    borderColor: colors.border,
    borderRadius: 8,
    padding: 12,
    gap: 6,
  },
  cardHeader: { flexDirection: 'row', alignItems: 'center', gap: 8 },
  compactRow: {
    borderWidth: 1,
    borderColor: colors.border,
    borderRadius: 8,
    padding: 12,
    gap: 4,
  },
  compactHeader: { flexDirection: 'row', alignItems: 'center', gap: 8 },
  itemName: { flex: 1, fontSize: 15, color: colors.text },
  lowTag: {
    fontSize: 11,
    color: colors.accent,
    borderWidth: 1,
    borderColor: colors.accent,
    borderRadius: 8,
    paddingHorizontal: 6,
    paddingVertical: 1,
  },
  notTestedTag: { fontSize: 11, color: colors.muted },
  flag: { fontSize: 13 },
  valueRow: {
    flexDirection: 'row',
    justifyContent: 'space-between',
    alignItems: 'baseline',
    gap: 8,
  },
  valueWithTag: { flex: 1, flexDirection: 'row', alignItems: 'baseline', flexWrap: 'wrap', gap: 8 },
  value: { fontSize: 20, fontVariant: ['tabular-nums'] },
  compactValue: { fontSize: 16, fontVariant: ['tabular-nums'] },
  unit: { fontSize: 13 },
  ref: { fontSize: 13, color: colors.muted },
  emptyChart: { fontSize: 12, color: colors.muted },
  meta: { fontSize: 12, color: colors.muted, lineHeight: 18 },
  detailLink: { alignSelf: 'flex-end', fontSize: 13, color: colors.accent },
  fail: { fontSize: 13, color: colors.fail },
  link: { color: colors.accent, fontSize: 15 },
  button: {
    alignSelf: 'flex-start',
    paddingVertical: 10,
    paddingHorizontal: 16,
    backgroundColor: colors.accent,
    borderRadius: 8,
  },
  buttonText: { color: colors.onAccent, fontSize: 15 },
});
