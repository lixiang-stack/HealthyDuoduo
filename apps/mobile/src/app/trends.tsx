import { Link } from 'expo-router';
import { useState } from 'react';
import { ActivityIndicator, Pressable, ScrollView, StyleSheet, Text, View } from 'react-native';

import type { PanelItem, ReportSummary } from '../api/reports';
import { sharedStyles } from '../components/styles';
import { colors } from '../components/theme';
import { MemberChips } from '../members/components';
import { useCurrentMember } from '../members/CurrentMemberContext';
import { sortMembers } from '../members/order';
import { FLAG_COLORS } from '../reports/colors';
import {
  panelTrendOf,
  type AbnormalPanelGroup,
  type PanelTrend,
} from '../reports/panelOverview';
import { panelLabel, type PanelCode } from '../reports/panels';
import { flagLabel, panelDateLabel, refRangeLabel } from '../reports/status';
import { TrendDetailSheet } from '../reports/TrendDetailSheet';
import { TrendSparkline } from '../reports/TrendChart';
import {
  formatSeriesMeta,
  TREND_UNAVAILABLE_LABEL,
} from '../reports/trends';
import { useAbnormalPanelGroups } from '../reports/useAbnormalPanelGroups';
import { useMemberPanels } from '../reports/useMemberPanels';
import { useMemberReports } from '../reports/useMemberReports';

type SelectedTrend = PanelTrend & { panel: PanelCode };

// 异常指标(panel-browse 票 04):按项目分组展示当前成员各项目 latest 的异常指标。
// 数据来自 /members/{id}/panels(abnormal_count 过滤)+ 项目详情(PanelItem.points),
// 点行打开详情层时只读已加载 points,不再逐指标请求趋势端点。
export default function TrendsScreen() {
  const { currentMemberId, selectMember } = useCurrentMember();
  const { state: reportsState, reload } = useMemberReports(currentMemberId);

  const ready = reportsState.status === 'ready';
  const selectedMemberId =
    ready && reportsState.hasCurrentMember ? currentMemberId : undefined;
  const selected = selectedMemberId !== undefined;
  const reportsForCurrent = ready && reportsState.reportsMemberId === currentMemberId;
  const switching = selected && !reportsForCurrent;
  const reports = selected && reportsForCurrent ? reportsState.reports : undefined;
  const members = ready ? sortMembers(reportsState.members) : [];

  return (
    <ScrollView style={styles.container} contentContainerStyle={styles.content}>
      {reportsState.status === 'loading' || switching ? (
        <View style={sharedStyles.horizontal}>
          <ActivityIndicator />
          <Text style={sharedStyles.muted}>正在加载异常指标…</Text>
        </View>
      ) : null}

      {reportsState.status === 'unconfigured' ? (
        <Text style={sharedStyles.muted}>
          尚未配置后端地址，请先到
          <Link href="/settings" style={styles.link}>
            设置
          </Link>
          填写。
        </Text>
      ) : null}

      {reportsState.status === 'error' ? (
        <View style={styles.stack}>
          <Text style={styles.fail}>加载失败:{reportsState.detail}</Text>
          <Pressable style={styles.button} onPress={() => void reload()}>
            <Text style={styles.buttonText}>重试</Text>
          </Pressable>
        </View>
      ) : null}

      {ready ? (
        <View style={styles.stack}>
          {members.length === 0 ? (
            <Text style={sharedStyles.muted}>
              还没有家庭成员。先到
              <Link href="/members" style={styles.link}>
                家庭成员
              </Link>
              建一个。
            </Text>
          ) : (
            <>
              <Text style={sharedStyles.label}>查看谁的档案</Text>
              <MemberChips
                members={members}
                selectedId={currentMemberId}
                onSelect={selectMember}
              />
              {selectedMemberId === undefined ? (
                <Text style={sharedStyles.muted}>选择一个成员查看异常指标。</Text>
              ) : reports === undefined ? null : reports.length === 0 ? (
                <View testID="trend-empty-reports" style={styles.stack}>
                  <Text style={sharedStyles.muted}>这位成员还没有检查单。</Text>
                  <Link href="/upload" style={styles.link}>
                    上传检查单
                  </Link>
                </View>
              ) : (
                <AbnormalPanels memberId={selectedMemberId} reports={reports} />
              )}
            </>
          )}
        </View>
      ) : null}
    </ScrollView>
  );
}

// 项目列表就绪后再按 abnormal_count 取项目详情;未选成员 / 列表 loading 由外层与
// 本组件的 loading 分支呈现。
function AbnormalPanels({ memberId, reports }: { memberId: number; reports: ReportSummary[] }) {
  const { state, reload } = useMemberPanels(memberId);
  const panels = state.status === 'ready' ? state.panels : undefined;
  const groupsState = useAbnormalPanelGroups(memberId, panels);
  const [detailTrend, setDetailTrend] = useState<SelectedTrend>();

  const reportTypeById = Object.fromEntries(
    reports.map((report) => [report.report_id, report.report_type]),
  );
  const errorDetail =
    state.status === 'error'
      ? state.detail
      : groupsState.status === 'error'
        ? groupsState.detail
        : undefined;
  const loading =
    !errorDetail &&
    (state.status === 'loading' ||
      (state.status === 'ready' &&
        (groupsState.status === 'loading' || groupsState.status === 'idle')));

  return (
    <>
      <View style={styles.stack}>
        {loading ? (
          <View style={sharedStyles.horizontal}>
            <ActivityIndicator />
            <Text style={sharedStyles.muted}>正在加载异常指标…</Text>
          </View>
        ) : null}

        {!errorDetail &&
        (state.status === 'unconfigured' || groupsState.status === 'unconfigured') ? (
          <Text style={sharedStyles.muted}>
            尚未配置后端地址，请先到
            <Link href="/settings" style={styles.link}>
              设置
            </Link>
            填写。
          </Text>
        ) : null}

        {errorDetail ? (
          <View style={styles.stack}>
            <Text style={styles.fail}>加载失败:{errorDetail}</Text>
            <Pressable style={styles.button} onPress={() => void reload()}>
              <Text style={styles.buttonText}>重试</Text>
            </Pressable>
          </View>
        ) : null}

        {groupsState.status === 'ready' && groupsState.groups.length === 0 ? (
          <View testID="trend-empty-abnormal" style={styles.stack}>
            <Text style={sharedStyles.muted}>这位成员暂时没有异常指标。</Text>
            <Link href="/timeline" style={styles.link}>
              查看全部档案
            </Link>
          </View>
        ) : null}

        {groupsState.status === 'ready' && groupsState.groups.length > 0 ? (
          <PanelGroups
            groups={groupsState.groups}
            reportTypeById={reportTypeById}
            onSelectRow={setDetailTrend}
          />
        ) : null}
      </View>
      {detailTrend ? (
        <TrendDetailSheet
          key={`${detailTrend.panel}-${detailTrend.name}-${detailTrend.series.unit ?? ''}`}
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

function PanelGroups({
  groups,
  reportTypeById,
  onSelectRow,
}: {
  groups: AbnormalPanelGroup[];
  reportTypeById: Record<string, string>;
  onSelectRow: (trend: SelectedTrend) => void;
}) {
  const starts = groups.map((_, index) =>
    groups.slice(0, index).reduce((sum, group) => sum + group.items.length, 0),
  );

  return (
    <>
      <Text testID="trends-band-legend" style={sharedStyles.muted}>
        灰色带 = 参考范围
      </Text>
      {groups.map((group, groupIndex) => (
        <View key={group.panel} testID={`trend-panel-${groupIndex}`} style={styles.group}>
          <Text style={styles.groupTitle}>
            {`${panelLabel(group.panel)} · ${panelDateLabel(group.reportDate, false)} · ${group.items.length} 项异常`}
          </Text>
          {group.items.map((item, index) => (
            <AbnormalIndicatorCard
              key={`${group.panel}-${item.name}-${item.unit ?? ''}`}
              panel={group.panel}
              item={item}
              index={starts[groupIndex] + index}
              reportTypeById={reportTypeById}
              onPress={onSelectRow}
            />
          ))}
        </View>
      ))}
    </>
  );
}

function AbnormalIndicatorCard({
  panel,
  item,
  index,
  reportTypeById,
  onPress,
}: {
  panel: PanelCode;
  item: PanelItem;
  index: number;
  reportTypeById: Record<string, string>;
  onPress: (trend: SelectedTrend) => void;
}) {
  const latest = item.latest!;
  const trend = panelTrendOf(item, reportTypeById)!;
  const flagColor = FLAG_COLORS[latest.flag];
  const lowConfidence = latest.low_confidence && !latest.reviewed;
  const openDetail = () => onPress({ ...trend, panel });

  return (
    <Pressable
      testID={`trend-indicator-${index}`}
      accessibilityRole="button"
      onPress={openDetail}
      style={styles.card}
    >
      <View style={styles.cardHeader}>
        <Text style={styles.itemName}>{item.name}</Text>
        {lowConfidence ? <Text style={styles.lowTag}>低置信</Text> : null}
        <Text style={[styles.flag, { color: flagColor }]}>{flagLabel(latest.flag)}</Text>
      </View>
      <View style={styles.valueRow}>
        <View style={styles.valueWithTag}>
          <Text style={[styles.value, { color: flagColor }]}>
            {latest.value ?? '—'}
            {latest.unit ? <Text style={styles.unit}> {latest.unit}</Text> : null}
          </Text>
          {item.latest_report_not_tested ? (
            <Text testID={`trend-item-not-tested-${index}`} style={styles.notTestedTag}>
              最近一次未检测
            </Text>
          ) : null}
        </View>
        <Text style={styles.ref}>{refRangeLabel(item.ref_range)}</Text>
      </View>
      {trend.series.chartPoints.length === 0 ? (
        <Text style={styles.emptyChart}>{TREND_UNAVAILABLE_LABEL}</Text>
      ) : (
        <TrendSparkline series={trend.series} name={item.name} testID={`trend-sparkline-${index}`} />
      )}
      <Text style={styles.meta}>
        {formatSeriesMeta(trend.series, { reportDate: latest.report_date })}
      </Text>
      <Text style={styles.detailLink}>查看趋势 ›</Text>
    </Pressable>
  );
}

const styles = StyleSheet.create({
  container: { flex: 1, backgroundColor: colors.background },
  content: { padding: 24, gap: 12 },
  stack: { gap: 12 },
  group: { gap: 8 },
  groupTitle: { fontSize: 13, color: colors.muted },
  card: {
    borderWidth: 1,
    borderColor: colors.border,
    borderRadius: 8,
    padding: 12,
    gap: 6,
  },
  cardHeader: { flexDirection: 'row', alignItems: 'center', gap: 8 },
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
  flag: { fontSize: 13 },
  valueRow: {
    flexDirection: 'row',
    justifyContent: 'space-between',
    alignItems: 'baseline',
    gap: 8,
  },
  valueWithTag: { flex: 1, flexDirection: 'row', alignItems: 'baseline', flexWrap: 'wrap', gap: 8 },
  notTestedTag: { fontSize: 11, color: colors.muted },
  value: { fontSize: 20, fontVariant: ['tabular-nums'] },
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
