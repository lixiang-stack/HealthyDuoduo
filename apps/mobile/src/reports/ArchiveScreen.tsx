import { Link, router, useLocalSearchParams } from 'expo-router';
import { useBottomTabBarHeight } from 'expo-router/js-tabs';
import { ActivityIndicator, Pressable, ScrollView, StyleSheet, Text, View } from 'react-native';

import { sharedStyles } from '../components/styles';
import { colors } from '../components/theme';
import { MemberChips } from '../members/components';
import { useCurrentMember } from '../members/CurrentMemberContext';
import { sortReports } from './order';
import { filterReports, parseReportFilter, pendingSummary } from './overview';
import {
  abnormalPanelCount,
  filterReportsByPanel,
  panelSourceLabel,
  parsePanelParam,
  totalAbnormalCount,
} from './panelOverview';
import { panelLabel } from './panels';
import { panelDateLabel } from './status';
import { ReportTimeline } from './ReportTimeline';
import { useMemberPanels } from './useMemberPanels';
import { useMemberReports } from './useMemberReports';

export type ArchiveView = 'time' | 'panel';

// 全部档案页(票 06;panel-browse 票 03 起分「按时间 / 按项目」):
// 按时间沿用待核对 / 识别失败筛选与 ?panel= 项目筛选;按项目列出当前成员实际
// 出现过的项目组(影像直接进最新报告详情),点项目进项目详情。
// 两个真实路由 /timeline 与 /panels 共用本组件;分段切换用 replace,避免历史栈
// 累积,并保证从项目详情返回时仍落在原分段。
export function ArchiveScreen({ view }: { view: ArchiveView }) {
  const { currentMemberId, selectMember } = useCurrentMember();
  const { state, reload } = useMemberReports(currentMemberId);
  const params = useLocalSearchParams();
  const filter = parseReportFilter(params.filter);
  const panel = parsePanelParam(params.panel);
  const tabBarHeight = useBottomTabBarHeight();

  const members = state.status === 'ready' ? state.members : [];
  const selectedMemberId =
    state.status === 'ready' && state.hasCurrentMember ? currentMemberId : undefined;
  const reports = state.status === 'ready' ? sortReports(state.reports) : [];
  const visibleReports = filterReports(filterReportsByPanel(reports, panel), filter);
  const filtered = filter !== undefined || panel !== undefined;
  const pending = selectedMemberId === undefined ? undefined : pendingSummary(reports);
  const partialLabel =
    pending && pending.unreviewedItems > 0
      ? `待核对 · ${pending.partialReports} 份 · ${pending.unreviewedItems} 项`
      : `待核对 · ${pending?.partialReports ?? 0} 份`;

  return (
    <View style={styles.screen}>
      <ScrollView style={styles.container} contentContainerStyle={styles.content}>
        {state.status === 'loading' ? (
          <View style={sharedStyles.horizontal}>
            <ActivityIndicator />
            <Text style={sharedStyles.muted}>正在加载档案…</Text>
          </View>
        ) : null}

        {state.status === 'unconfigured' ? (
          <Text style={sharedStyles.muted}>尚未配置后端地址,请先到设置填写。</Text>
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
            {members.length === 0 ? (
              <Text style={sharedStyles.muted}>
                还没有家庭成员。先到<Link href="/members" style={styles.link}>家庭成员</Link>建一个。
              </Text>
            ) : (
              <>
                <Text style={sharedStyles.label}>查看谁的档案</Text>
                <MemberChips members={members} selectedId={currentMemberId} onSelect={selectMember} />
                {selectedMemberId === undefined ? (
                  <View testID="archive-unselected" style={styles.stack}>
                    <Text style={sharedStyles.muted}>选择一个成员查看其档案。</Text>
                    <Link asChild href="/">
                      <Pressable testID="archive-back-home">
                        <Text style={styles.link}>返回家庭</Text>
                      </Pressable>
                    </Link>
                  </View>
                ) : (
                  <View style={styles.stack}>
                    <ArchiveSegments value={view} />
                    {view === 'time' ? (
                      <View style={styles.stack}>
                        {pending ? (
                          <View style={styles.pendingEntries}>
                            {pending.partialReports > 0 ? (
                              <Link asChild href="/timeline?filter=partial">
                                <Pressable testID="archive-pending-partial" style={styles.pendingRow}>
                                  <Text style={styles.pendingText}>{partialLabel}</Text>
                                  <Text style={styles.pendingArrow}>→</Text>
                                </Pressable>
                              </Link>
                            ) : null}
                            {pending.failedReports > 0 ? (
                              <Link asChild href="/timeline?filter=failed">
                                <Pressable testID="archive-pending-failed" style={styles.pendingRow}>
                                  <Text style={styles.pendingText}>
                                    识别失败 · {pending.failedReports} 份
                                  </Text>
                                  <Text style={styles.pendingArrow}>→</Text>
                                </Pressable>
                              </Link>
                            ) : null}
                            {pending.partialReports === 0 && pending.failedReports === 0 ? (
                              <Text testID="archive-pending-empty" style={sharedStyles.muted}>
                                没有待处理的报告。
                              </Text>
                            ) : null}
                          </View>
                        ) : null}
                        {filtered ? (
                          <View style={styles.filterBanners}>
                            {panel ? (
                              <View testID="archive-panel-banner" style={styles.filterBanner}>
                                <Text style={sharedStyles.muted}>
                                  只看{panelLabel(panel)} · {visibleReports.length} 份
                                </Text>
                                <Link asChild href="/timeline">
                                  <Pressable testID="archive-panel-clear">
                                    <Text style={styles.link}>清除筛选</Text>
                                  </Pressable>
                                </Link>
                              </View>
                            ) : null}
                            {filter ? (
                              <View testID="archive-filter-banner" style={styles.filterBanner}>
                                <Text style={sharedStyles.muted}>
                                  {filter === 'partial' ? '只看待核对' : '只看识别失败'} ·{' '}
                                  {visibleReports.length} 份
                                </Text>
                                <Link asChild href="/timeline">
                                  <Pressable testID="archive-filter-clear">
                                    <Text style={styles.link}>清除筛选</Text>
                                  </Pressable>
                                </Link>
                              </View>
                            ) : null}
                          </View>
                        ) : null}
                        {visibleReports.length === 0 ? (
                          filtered ? (
                            <View testID="archive-empty" style={styles.stack}>
                              <Text style={sharedStyles.muted}>没有符合筛选条件的报告。</Text>
                              <Link asChild href="/timeline">
                                <Pressable testID="archive-show-all">
                                  <Text style={styles.link}>查看全部档案</Text>
                                </Pressable>
                              </Link>
                            </View>
                          ) : (
                            <Text style={sharedStyles.muted}>
                              这位成员还没有检查单。拍一张检查单,几秒后这里会出现归档卡片。
                            </Text>
                          )
                        ) : (
                          <ReportTimeline reports={visibleReports} />
                        )}
                      </View>
                    ) : (
                      <PanelBrowser
                        key={selectedMemberId}
                        memberId={selectedMemberId}
                        onShowTime={() => router.replace('/timeline')}
                      />
                    )}
                  </View>
                )}
              </>
            )}
          </View>
        ) : null}
      </ScrollView>
      {selectedMemberId !== undefined ? (
        <Pressable
          testID="upload-fab"
          accessibilityRole="button"
          accessibilityLabel="上传检查单"
          onPress={() => router.push('/upload')}
          style={[styles.fab, { bottom: tabBarHeight + 16 }]}
        >
          <Text style={styles.fabText}>＋</Text>
        </Pressable>
      ) : null}
    </View>
  );
}

const ARCHIVE_VIEW_HREFS: Record<ArchiveView, '/timeline' | '/panels'> = {
  time: '/timeline',
  panel: '/panels',
};

function ArchiveSegments({ value }: { value: ArchiveView }) {
  return (
    <View style={styles.segments}>
      {(
        [
          ['panel', '按项目'],
          ['time', '按时间'],
        ] as const
      ).map(([key, label]) => (
        <Pressable
          key={key}
          testID={`archive-view-${key}`}
          accessibilityRole="button"
          accessibilityState={{ selected: value === key }}
          onPress={() => {
            if (value !== key) {
              router.replace(ARCHIVE_VIEW_HREFS[key]);
            }
          }}
          style={[styles.segment, value === key && styles.segmentActive]}
        >
          <Text style={[styles.segmentText, value === key && styles.segmentTextActive]}>
            {label}
          </Text>
        </Pressable>
      ))}
    </View>
  );
}

// 按项目视图:只在挂载时请求项目列表(切回按时间即卸载)。
function PanelBrowser({ memberId, onShowTime }: { memberId: number; onShowTime: () => void }) {
  const { state, reload } = useMemberPanels(memberId);

  if (state.status === 'loading') {
    return (
      <View style={sharedStyles.horizontal}>
        <ActivityIndicator />
        <Text style={sharedStyles.muted}>正在加载项目…</Text>
      </View>
    );
  }

  if (state.status === 'unconfigured') {
    return <Text style={sharedStyles.muted}>尚未配置后端地址,请先到设置填写。</Text>;
  }

  if (state.status === 'error') {
    return (
      <View style={styles.stack}>
        <Text style={styles.fail}>加载失败:{state.detail}</Text>
        <Pressable style={styles.button} onPress={() => void reload()}>
          <Text style={styles.buttonText}>重试</Text>
        </Pressable>
      </View>
    );
  }

  if (state.panels.length === 0) {
    return (
      <View testID="archive-panels-empty" style={styles.stack}>
        <Text style={sharedStyles.muted}>
          还没有可归类的报告。识别失败的记录可在按时间查看。
        </Text>
        <Pressable testID="archive-show-time" onPress={onShowTime}>
          <Text style={styles.link}>切回按时间</Text>
        </Pressable>
      </View>
    );
  }

  const abnormalCount = totalAbnormalCount(state.panels);
  const abnormalPanels = abnormalPanelCount(state.panels);

  return (
    <View style={styles.stack}>
      {abnormalCount > 0 ? (
        <Link asChild href="/trends">
          <Pressable testID="archive-all-abnormal">
            <Text style={styles.link}>
              全部异常 {abnormalCount} 项 · {abnormalPanels} 个项目 ›
            </Text>
          </Pressable>
        </Link>
      ) : null}
      {state.panels.map((panel) => {
        const source = panelSourceLabel(panel.panel, panel.report_types);
        const date = panelDateLabel(panel.latest_report_date, panel.narrative);
        const meta = panel.narrative
          ? `${panel.report_count} 份 · 叙述型报告`
          : `${panel.report_count} 份 · ${panel.abnormal_count} 项异常 · ${panel.item_count} 项指标`;
        const row = (
          <Pressable testID={`panel-row-${panel.panel}`} style={styles.panelRow}>
            <View style={styles.panelRowHeader}>
              <Text style={styles.panelName}>{panelLabel(panel.panel)}</Text>
              <Text style={sharedStyles.muted}>{date}</Text>
            </View>
            {source ? <Text style={sharedStyles.muted}>{source}</Text> : null}
            <Text testID={`panel-row-meta-${panel.panel}`} style={sharedStyles.muted}>
              {meta}
            </Text>
          </Pressable>
        );
        if (panel.narrative) {
          if (!panel.latest_report_id) {
            return <View key={panel.panel}>{row}</View>;
          }
          return (
            <Link
              key={panel.panel}
              asChild
              href={{
                pathname: '/report/[report_id]',
                params: { report_id: panel.latest_report_id },
              }}
            >
              {row}
            </Link>
          );
        }
        return (
          <Link
            key={panel.panel}
            asChild
            href={{ pathname: '/panel/[panel]', params: { panel: panel.panel } }}
          >
            {row}
          </Link>
        );
      })}
    </View>
  );
}

const styles = StyleSheet.create({
  screen: { flex: 1, backgroundColor: colors.background },
  container: { flex: 1, backgroundColor: colors.background },
  content: { padding: 24, gap: 12 },
  stack: { gap: 12 },
  pendingEntries: { gap: 8 },
  pendingRow: {
    flexDirection: 'row',
    alignItems: 'center',
    justifyContent: 'space-between',
    gap: 12,
    borderWidth: 1,
    borderColor: colors.border,
    borderRadius: 8,
    padding: 12,
  },
  pendingText: { fontSize: 15, color: colors.text },
  pendingArrow: { fontSize: 15, color: colors.muted },
  filterBanners: { gap: 8 },
  filterBanner: { flexDirection: 'row', alignItems: 'center', justifyContent: 'space-between', gap: 12 },
  fab: {
    position: 'absolute',
    right: 24,
    width: 56,
    height: 56,
    borderRadius: 28,
    backgroundColor: colors.accent,
    alignItems: 'center',
    justifyContent: 'center',
  },
  fabText: { color: colors.onAccent, fontSize: 28, lineHeight: 30 },
  segments: {
    flexDirection: 'row',
    borderWidth: 1,
    borderColor: colors.border,
    borderRadius: 8,
    padding: 2,
    gap: 2,
  },
  segment: { flex: 1, alignItems: 'center', paddingVertical: 8, borderRadius: 6 },
  segmentActive: { backgroundColor: colors.accent },
  segmentText: { fontSize: 15, color: colors.text },
  segmentTextActive: { color: colors.onAccent },
  panelRow: {
    borderWidth: 1,
    borderColor: colors.border,
    borderRadius: 8,
    padding: 12,
    gap: 4,
  },
  panelRowHeader: {
    flexDirection: 'row',
    justifyContent: 'space-between',
    alignItems: 'baseline',
    gap: 8,
  },
  panelName: { fontSize: 16, color: colors.text },
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
