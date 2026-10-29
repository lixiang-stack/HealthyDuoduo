import { Link, router } from 'expo-router';
import { ActivityIndicator, Pressable, ScrollView, StyleSheet, Text, View } from 'react-native';

import { sharedStyles } from '../../components/styles';
import { colors } from '../../components/theme';
import { useCurrentMember } from '../../members/CurrentMemberContext';
import { familyCardSummary } from '../../reports/familyOverview';
import {
  useFamilyOverview,
  type FamilyMemberOverview,
} from '../../reports/useFamilyOverview';

// 家庭首页(票 06):每张卡回答「谁现在有什么要处理」;点卡选中该成员并进档案。
// 待核对 / 异常项目 / 最近报告偏高偏低只陈述事实,不做建议。
export default function FamilyScreen() {
  const { selectMember } = useCurrentMember();
  const { state, reload } = useFamilyOverview();
  const today = new Date();

  const openMember = (memberId: number) => {
    selectMember(memberId);
    router.push('/panels');
  };

  return (
    <ScrollView style={styles.container} contentContainerStyle={styles.content}>
      {state.status === 'loading' ? (
        <View style={sharedStyles.horizontal}>
          <ActivityIndicator />
          <Text style={sharedStyles.muted}>正在加载家庭…</Text>
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
          {state.members.length === 0 ? (
            <Text style={sharedStyles.muted}>
              还没有家庭成员。先到<Link href="/members" style={styles.link}>家庭成员</Link>建一个。
            </Text>
          ) : (
            <>
              <Text style={sharedStyles.label}>家庭成员</Text>
              {state.members.map((overview) => (
                <FamilyMemberCard
                  key={overview.member.member_id}
                  overview={overview}
                  today={today}
                  onPress={() => openMember(overview.member.member_id)}
                />
              ))}
              <Link asChild href="/members">
                <Pressable testID="family-manage-members">
                  <Text style={styles.link}>管理成员</Text>
                </Pressable>
              </Link>
            </>
          )}
        </View>
      ) : null}
    </ScrollView>
  );
}

function FamilyMemberCard({
  overview,
  today,
  onPress,
}: {
  overview: FamilyMemberOverview;
  today: Date;
  onPress: () => void;
}) {
  const { member } = overview;
  const summary = familyCardSummary(overview, today);

  return (
    <Pressable
      testID={`family-member-${member.member_id}`}
      accessibilityRole="button"
      onPress={onPress}
      style={styles.card}
    >
      <View style={styles.cardHeader}>
        <Text style={styles.memberName}>{member.name}</Text>
        {member.is_child ? <Text style={styles.childBadge}>孩子</Text> : null}
      </View>
      {summary.pending ? (
        <Text
          testID={`family-member-pending-${member.member_id}`}
          style={summary.hasPending ? styles.pending : sharedStyles.muted}
        >
          {summary.pending}
        </Text>
      ) : null}
      <Text testID={`family-member-latest-${member.member_id}`} style={styles.latest}>
        {summary.latest}
      </Text>
      {summary.hasReports ? (
        summary.abnormal ? (
          <Text testID={`family-member-abnormal-${member.member_id}`} style={styles.abnormal}>
            {summary.abnormal}
          </Text>
        ) : (
          <Text testID={`family-member-abnormal-${member.member_id}`} style={sharedStyles.muted}>
            没有异常项目
          </Text>
        )
      ) : null}
      {summary.latestFlags ? (
        <Text testID={`family-member-flags-${member.member_id}`} style={sharedStyles.muted}>
          {summary.latestFlags}
        </Text>
      ) : null}
      {summary.reportCount ? (
        <Text testID={`family-member-count-${member.member_id}`} style={sharedStyles.muted}>
          {summary.reportCount}
        </Text>
      ) : null}
    </Pressable>
  );
}

const styles = StyleSheet.create({
  container: { flex: 1, backgroundColor: colors.background },
  content: { padding: 24, gap: 12 },
  stack: { gap: 12 },
  card: {
    borderWidth: 1,
    borderColor: colors.border,
    borderRadius: 8,
    padding: 12,
    gap: 4,
  },
  cardHeader: { flexDirection: 'row', alignItems: 'center', gap: 8 },
  memberName: { fontSize: 17, color: colors.text },
  childBadge: {
    fontSize: 11,
    color: colors.accent,
    borderWidth: 1,
    borderColor: colors.accent,
    borderRadius: 8,
    paddingHorizontal: 6,
    paddingVertical: 1,
  },
  latest: { fontSize: 14, color: colors.text },
  pending: { fontSize: 14, color: colors.accent, fontWeight: '600' },
  abnormal: { fontSize: 14, color: colors.fail, fontWeight: '600', lineHeight: 20 },
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
