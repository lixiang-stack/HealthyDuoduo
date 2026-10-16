import { Link, useFocusEffect } from 'expo-router';
import { useCallback, useState } from 'react';
import { ActivityIndicator, Pressable, ScrollView, StyleSheet, Text, View } from 'react-native';

import { listMembers, type Member } from '../api/members';
import { listReports, type ReportSummary } from '../api/reports';
import { resolveApiClient } from '../api/resolveClient';
import { colors } from '../components/theme';
import { MemberChips } from '../members/components';
import { useCurrentMember } from '../members/CurrentMemberContext';
import { sortMembers } from '../members/order';
import { ReportCard } from '../reports/components';
import { sortReports } from '../reports/order';

// 时间线页(票 06):按当前成员过滤报告;卡片含类别、日期、状态文案、flag 徽标与
// 「待核对 N 项」;failed 展示失败原因。空态引导上传,加载与错误可重试。
type ScreenState =
  | { status: 'loading' }
  | { status: 'unconfigured' }
  | { status: 'error'; detail: string }
  | { status: 'ready'; members: Member[]; reports: ReportSummary[] };

export default function TimelineScreen() {
  const { currentMemberId, selectMember } = useCurrentMember();
  const [state, setState] = useState<ScreenState>({ status: 'loading' });

  const reload = useCallback(async () => {
    const resolved = await resolveApiClient();
    if (!resolved) {
      setState({ status: 'unconfigured' });
      return;
    }
    try {
      const members = await listMembers(resolved.client);
      const selected = members.find((m) => m.member_id === currentMemberId);
      const reports = selected ? await listReports(resolved.client, selected.member_id) : [];
      setState({ status: 'ready', members, reports });
    } catch (error) {
      setState({ status: 'error', detail: error instanceof Error ? error.message : String(error) });
    }
  }, [currentMemberId]);

  useFocusEffect(
    useCallback(() => {
      void reload();
    }, [reload]),
  );

  const members = state.status === 'ready' ? sortMembers(state.members) : [];
  const selected = state.status === 'ready' && members.some((m) => m.member_id === currentMemberId);
  const reports = state.status === 'ready' ? sortReports(state.reports) : [];

  return (
    <ScrollView style={styles.container} contentContainerStyle={styles.content}>
      {state.status === 'loading' ? (
        <View style={styles.horizontal}>
          <ActivityIndicator />
          <Text style={styles.muted}>正在加载时间线…</Text>
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
          {members.length === 0 ? (
            <Text style={styles.muted}>
              还没有家庭成员。先到<Link href="/members" style={styles.link}>家庭成员</Link>建一个。
            </Text>
          ) : (
            <>
              <Text style={styles.label}>查看谁的档案</Text>
              <MemberChips members={members} selectedId={currentMemberId} onSelect={selectMember} />
              <Link href="/upload" style={styles.link}>
                上传检查单
              </Link>
              {!selected ? (
                <Text style={styles.muted}>选择一个成员查看其报告时间线。</Text>
              ) : reports.length === 0 ? (
                <Text style={styles.muted}>
                  这位成员还没有检查单。拍一张检查单,几秒后这里会出现归档卡片。
                </Text>
              ) : (
                reports.map((report) => <ReportCard key={report.report_id} report={report} />)
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
  horizontal: { flexDirection: 'row', alignItems: 'center', gap: 8 },
  label: { fontSize: 13, color: colors.muted },
  muted: { fontSize: 13, color: colors.muted, lineHeight: 20 },
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
