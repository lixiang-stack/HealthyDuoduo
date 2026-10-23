import { Link, useLocalSearchParams } from 'expo-router';
import { ActivityIndicator, Pressable, ScrollView, StyleSheet, Text, View } from 'react-native';

import { sharedStyles } from '../components/styles';
import { colors } from '../components/theme';
import { MemberChips } from '../members/components';
import { useCurrentMember } from '../members/CurrentMemberContext';
import { sortMembers } from '../members/order';
import { ReportCard } from '../reports/components';
import { sortReports } from '../reports/order';
import { filterReports, parseReportFilter } from '../reports/overview';
import { useMemberReports } from '../reports/useMemberReports';

// 全部档案页(票 06;本期筛选):按当前成员展示完整档案,可按待核对 / 识别失败
// 只看子集;卡片含类别、日期、状态文案、flag 徽标与「待核对 N 项」;failed 展示失败原因。
export default function TimelineScreen() {
  const { currentMemberId, selectMember } = useCurrentMember();
  const { state, reload } = useMemberReports(currentMemberId);
  const params = useLocalSearchParams();
  const filter = parseReportFilter(params.filter);

  const members = state.status === 'ready' ? sortMembers(state.members) : [];
  const selected = state.status === 'ready' && state.hasCurrentMember;
  const reports = state.status === 'ready' ? sortReports(state.reports) : [];
  const visibleReports = filterReports(reports, filter);

  return (
    <ScrollView style={styles.container} contentContainerStyle={styles.content}>
      {state.status === 'loading' ? (
        <View style={sharedStyles.horizontal}>
          <ActivityIndicator />
          <Text style={sharedStyles.muted}>正在加载档案…</Text>
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
              <Link href="/upload" style={styles.link}>
                上传检查单
              </Link>
              {!selected ? (
                <Text style={sharedStyles.muted}>选择一个成员查看其档案。</Text>
              ) : (
                <View style={styles.stack}>
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
                  {visibleReports.length === 0 ? (
                    filter ? (
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
                    visibleReports.map((report) => (
                      <Link
                        key={report.report_id}
                        asChild
                        href={{
                          pathname: '/report/[report_id]',
                          params: { report_id: report.report_id },
                        }}
                      >
                        <Pressable>
                          <ReportCard report={report} />
                        </Pressable>
                      </Link>
                    ))
                  )}
                </View>
              )}
            </>
          )}
        </View>
      ) : null}
    </ScrollView>
  );
}

const styles = StyleSheet.create({
  container: { flex: 1, backgroundColor: colors.background },
  content: { padding: 24, gap: 12 },
  stack: { gap: 12 },
  filterBanner: { flexDirection: 'row', alignItems: 'center', justifyContent: 'space-between', gap: 12 },
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
