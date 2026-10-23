import { Link } from 'expo-router';
import { ActivityIndicator, Pressable, ScrollView, StyleSheet, Text, View } from 'react-native';

import { sharedStyles } from '../components/styles';
import { colors } from '../components/theme';
import { MemberChips } from '../members/components';
import { useCurrentMember } from '../members/CurrentMemberContext';
import { sortMembers } from '../members/order';
import { ReportCard } from '../reports/components';
import { latestByCategory, pendingSummary } from '../reports/overview';
import { sortReports } from '../reports/order';
import { useMemberReports } from '../reports/useMemberReports';

// 成员概览(本期):第一屏回答「现在有什么要处理的」。待处理只给计数入口,
// 各类最近一次每类一张卡;完整档案与后端设置分别在 /timeline 与 /settings。
export default function HomeScreen() {
  const { currentMemberId, selectMember } = useCurrentMember();
  const { state, reload } = useMemberReports(currentMemberId);

  const members = state.status === 'ready' ? sortMembers(state.members) : [];
  const selected = state.status === 'ready' && state.hasCurrentMember;
  const reports = state.status === 'ready' ? sortReports(state.reports) : [];
  const pending = pendingSummary(reports);
  const latest = latestByCategory(reports);
  const partialLabel =
    pending.unreviewedItems > 0
      ? `待核对 · ${pending.partialReports} 份 · ${pending.unreviewedItems} 项`
      : `待核对 · ${pending.partialReports} 份`;

  return (
    <ScrollView style={styles.container} contentContainerStyle={styles.content}>
      {state.status === 'loading' ? (
        <View style={sharedStyles.horizontal}>
          <ActivityIndicator />
          <Text style={sharedStyles.muted}>正在加载概览…</Text>
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
              <View style={styles.headerRow}>
                <Text style={sharedStyles.label}>查看谁的档案</Text>
                <Link href="/members" style={styles.link}>
                  管理成员
                </Link>
              </View>
              <MemberChips members={members} selectedId={currentMemberId} onSelect={selectMember} />
              <Link href="/upload" style={styles.link}>
                上传检查单
              </Link>
              {!selected ? (
                <Text style={sharedStyles.muted}>选择一个成员查看其报告概览。</Text>
              ) : reports.length === 0 ? (
                <Text style={sharedStyles.muted}>
                  这位成员还没有检查单。拍一张检查单,几秒后这里会出现归档卡片。
                </Text>
              ) : (
                <View style={styles.stack}>
                  <Text style={sharedStyles.label}>待处理</Text>
                  {pending.partialReports === 0 && pending.failedReports === 0 ? (
                    <Text testID="overview-pending-empty" style={sharedStyles.muted}>
                      没有待处理的报告。
                    </Text>
                  ) : (
                    <>
                      {pending.partialReports > 0 ? (
                        <Link asChild href="/timeline?filter=partial">
                          <Pressable testID="overview-pending-partial" style={styles.pendingRow}>
                            <Text style={styles.pendingText}>{partialLabel}</Text>
                            <Text style={styles.pendingArrow}>→</Text>
                          </Pressable>
                        </Link>
                      ) : null}
                      {pending.failedReports > 0 ? (
                        <Link asChild href="/timeline?filter=failed">
                          <Pressable testID="overview-pending-failed" style={styles.pendingRow}>
                            <Text style={styles.pendingText}>
                              识别失败 · {pending.failedReports} 份
                            </Text>
                            <Text style={styles.pendingArrow}>→</Text>
                          </Pressable>
                        </Link>
                      ) : null}
                    </>
                  )}

                  <Text style={sharedStyles.label}>各类最近一次</Text>
                  {latest.map((report) => (
                    <Link
                      key={report.report_id}
                      asChild
                      href={{
                        pathname: '/report/[report_id]',
                        params: { report_id: report.report_id },
                      }}
                    >
                      <Pressable>
                        <ReportCard report={report} testIDPrefix="overview-latest" />
                      </Pressable>
                    </Link>
                  ))}
                  <Link asChild href="/timeline">
                    <Pressable testID="overview-all-reports">
                      <Text style={styles.link}>全部档案（{reports.length} 份）</Text>
                    </Pressable>
                  </Link>
                </View>
              )}
            </>
          )}
        </View>
      ) : null}
      <Link href="/settings" style={styles.link}>
        设置
      </Link>
    </ScrollView>
  );
}

const styles = StyleSheet.create({
  container: { flex: 1, backgroundColor: colors.background },
  content: { padding: 24, gap: 12 },
  stack: { gap: 12 },
  headerRow: { flexDirection: 'row', alignItems: 'center', justifyContent: 'space-between', gap: 12 },
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
