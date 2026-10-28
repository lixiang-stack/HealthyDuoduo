import { Link } from 'expo-router';
import { useState } from 'react';
import { ActivityIndicator, Pressable, ScrollView, StyleSheet, Text, View } from 'react-native';

import { sharedStyles } from '../components/styles';
import { colors } from '../components/theme';
import { MemberChips } from '../members/components';
import { useCurrentMember } from '../members/CurrentMemberContext';
import { sortMembers } from '../members/order';
import { FLAG_COLORS } from '../reports/colors';
import { TrendDetailSheet } from '../reports/TrendDetailSheet';
import { TrendSparkline } from '../reports/TrendChart';
import { flagLabel, refRangeLabel, reportDateLabel } from '../reports/status';
import { formatSeriesMeta, TREND_UNAVAILABLE_LABEL } from '../reports/trends';
import { displayItemName } from '../review/items';
import {
  useAbnormalTrends,
  type TrendGroup,
  type TrendRow,
} from '../reports/useAbnormalTrends';
import { useMemberReports } from '../reports/useMemberReports';

// 异常指标趋势列表(票 03):按报告类别展示当前成员最新非 failed 报告里的异常项,
// 每行一条同单位迷你趋势;点按行打开详情层(票 04),详情层复用已加载数据,不额外请求。
export default function TrendsScreen() {
  const { currentMemberId, selectMember } = useCurrentMember();
  const { state: reportsState, reload } = useMemberReports(currentMemberId);
  const ready = reportsState.status === 'ready';
  const selected = ready && reportsState.hasCurrentMember;
  const reportsForCurrent = ready && reportsState.reportsMemberId === currentMemberId;
  const switching = selected && !reportsForCurrent;
  const reports = selected && reportsForCurrent ? reportsState.reports : undefined;
  const trendsState = useAbnormalTrends(selected ? currentMemberId : undefined, reports);
  const [detailRow, setDetailRow] = useState<TrendRow>();

  const members = ready ? sortMembers(reportsState.members) : [];
  const errorDetail =
    reportsState.status === 'error'
      ? reportsState.detail
      : trendsState.status === 'error'
        ? trendsState.detail
        : undefined;
  const loading =
    !errorDetail &&
    (reportsState.status === 'loading' ||
      switching ||
      (selected &&
        (trendsState.status === 'loading' || trendsState.status === 'idle')));

  return (
    <>
      <ScrollView style={styles.container} contentContainerStyle={styles.content}>
        {loading ? (
          <View style={sharedStyles.horizontal}>
            <ActivityIndicator />
            <Text style={sharedStyles.muted}>正在加载异常指标趋势…</Text>
          </View>
        ) : null}

        {!errorDetail &&
        (reportsState.status === 'unconfigured' || trendsState.status === 'unconfigured') ? (
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

        {reportsState.status === 'ready' && !errorDetail ? (
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
                {!selected ? (
                  <Text style={sharedStyles.muted}>选择一个成员查看异常指标趋势。</Text>
                ) : reports === undefined ? null : reports.length === 0 ? (
                  <View testID="trend-empty-reports" style={styles.stack}>
                    <Text style={sharedStyles.muted}>这位成员还没有检查单。</Text>
                    <Link href="/upload" style={styles.link}>
                      上传检查单
                    </Link>
                  </View>
                ) : trendsState.status === 'ready' && trendsState.groups.length === 0 ? (
                  <View testID="trend-empty-abnormal" style={styles.stack}>
                    <Text style={sharedStyles.muted}>这位成员暂时没有异常指标。</Text>
                    <Link href="/timeline" style={styles.link}>
                      查看全部档案
                    </Link>
                  </View>
                ) : trendsState.status === 'ready' ? (
                  <TrendGroups groups={trendsState.groups} onSelectRow={setDetailRow} />
                ) : null}
              </>
            )}
          </View>
        ) : null}
      </ScrollView>
      {detailRow ? (
        <TrendDetailSheet
          key={`${detailRow.candidate.reportId}-${detailRow.candidate.item.item_id}`}
          name={displayItemName(detailRow.candidate.item)}
          reportType={detailRow.candidate.reportType}
          reportDate={detailRow.candidate.reportDate}
          latestValue={detailRow.candidate.item.value}
          series={detailRow.series}
          onClose={() => setDetailRow(undefined)}
        />
      ) : null}
    </>
  );
}

function TrendGroups({
  groups,
  onSelectRow,
}: {
  groups: TrendGroup[];
  onSelectRow: (row: TrendRow) => void;
}) {
  const starts = groups.map((_, index) =>
    groups.slice(0, index).reduce((sum, group) => sum + group.rows.length, 0),
  );

  return (
    <>
      {groups.map((group, groupIndex) => {
        const date = reportDateLabel({
          report_date: group.reportDate,
          report_type: group.reportType,
        });
        return (
          <View key={group.reportId} testID={`trend-category-${groupIndex}`} style={styles.group}>
            <Text style={styles.groupTitle}>
              {`${group.reportType} · ${date} · ${group.rows.length} 项异常`}
            </Text>
            {group.rows.map((row, index) => (
              <TrendIndicatorCard
                key={`${group.reportId}-${row.candidate.item.item_id}`}
                row={row}
                index={starts[groupIndex] + index}
                onPress={() => onSelectRow(row)}
              />
            ))}
          </View>
        );
      })}
    </>
  );
}

function TrendIndicatorCard({
  row,
  index,
  onPress,
}: {
  row: TrendRow;
  index: number;
  onPress: () => void;
}) {
  const { candidate, series } = row;
  const item = candidate.item;
  const name = displayItemName(item);
  const flagColor = FLAG_COLORS[item.flag];
  const lowConfidence = item.low_confidence && item.review_status === 'unreviewed';

  return (
    <Pressable
      testID={`trend-indicator-${index}`}
      accessibilityRole="button"
      onPress={onPress}
      style={styles.card}
    >
      <View style={styles.cardHeader}>
        <Text style={styles.itemName}>{name}</Text>
        {lowConfidence ? <Text style={styles.lowTag}>低置信</Text> : null}
        <Text style={[styles.flag, { color: flagColor }]}>{flagLabel(item.flag)}</Text>
      </View>
      <View style={styles.valueRow}>
        <Text style={[styles.value, { color: flagColor }]}>
          {item.value ?? '—'}
          {item.unit ? <Text style={styles.unit}> {item.unit}</Text> : null}
        </Text>
        <Text style={styles.ref}>{refRangeLabel(item.ref_range)}</Text>
      </View>
      {series.chartPoints.length === 0 ? (
        <Text style={styles.emptyChart}>{TREND_UNAVAILABLE_LABEL}</Text>
      ) : (
        <TrendSparkline
          series={series}
          name={name}
          testID={`trend-sparkline-${index}`}
        />
      )}
      <Text style={styles.meta}>{formatSeriesMeta(series, candidate)}</Text>
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
