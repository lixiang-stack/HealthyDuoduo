import { Link } from 'expo-router';
import { useEffect, useState, type ReactNode } from 'react';
import {
  ActivityIndicator,
  Pressable,
  RefreshControl,
  ScrollView,
  StyleSheet,
  Text,
  View,
} from 'react-native';

import type { Member } from '../api/members';
import { useStatsCareEvents } from '../care/useStatsCareEvents';
import { sharedStyles } from '../components/styles';
import { colors } from '../components/theme';
import { useCurrentMember } from '../members/CurrentMemberContext';
import { childMembers, resolveCurrentChild } from '../members/currentChild';
import { useMembers } from '../members/useMembers';
import { useReminderResync } from '../reminders/useReminderResync';
import { ChildSwitcher } from './ChildSwitcher';
import { StatsCareLog } from './StatsCareLog';
import { TodayCareLog } from './TodayCareLog';

type Variant = 'today' | 'stats';

// B3 育儿二级视图:当前孩子上下文在这里回落 / 自动选中;今天页承载护理记录,
// 统计页承载 7 天聚合图表。护理事件数据 / 记录弹层由 01、04 提供。
export function DailyPlaceholder({ variant }: { variant: Variant }) {
  const { currentChildId, childReady, setCurrentChild, clearChild } = useCurrentMember();
  const { state, reload } = useMembers();

  const ready = state.status === 'ready' && childReady;
  const allMembers = state.status === 'ready' ? state.members : [];
  const children = state.status === 'ready' ? childMembers(state.members) : [];
  const childState = resolveCurrentChild(
    state.status === 'ready' ? state.members : [],
    currentChildId,
  );
  const resolvedChildId =
    childState.status === 'selected' ? childState.child.member_id : undefined;

  // Reminder follows the current child from the shared care view, not either page.
  useReminderResync(resolvedChildId, ready);

  // 成员列表和本地选择都就绪后再回落 / 自动选中,避免把「存储加载中」当成「未选择」
  // 而清掉已有选择。
  useEffect(() => {
    if (!ready) {
      return;
    }
    if (resolvedChildId !== undefined) {
      if (currentChildId !== resolvedChildId) {
        setCurrentChild(resolvedChildId);
      }
      return;
    }
    if (currentChildId !== undefined) {
      clearChild();
    }
  }, [ready, resolvedChildId, currentChildId, setCurrentChild, clearChild]);

  const loading = state.status === 'loading' || (state.status === 'ready' && !childReady);

  const switcher =
    ready && childState.status !== 'no-children' ? (
      <ChildSwitcher childState={childState} members={children} onSelect={setCurrentChild} />
    ) : null;

  // Stats owns its own scroll (pull-to-refresh); the shared placeholder only renders
  // the member / child states that do not depend on the seven-day read model.
  if (variant === 'stats' && ready && childState.status === 'selected') {
    return (
      <StatsDaily
        key={childState.child.member_id}
        switcher={switcher}
        child={childState.child}
      />
    );
  }

  const content = (
    <>
      {switcher}

      {variant === 'stats' ? <Text style={styles.title}>统计</Text> : null}

      {loading ? (
        <View style={sharedStyles.horizontal}>
          <ActivityIndicator />
          <Text style={sharedStyles.muted}>正在加载家庭成员…</Text>
        </View>
      ) : null}

      {state.status === 'unconfigured' ? (
        <Text style={sharedStyles.muted}>
          尚未配置后端地址,请先到
          <Link href="/settings" style={styles.link}>
            设置
          </Link>
          填写。
        </Text>
      ) : null}

      {state.status === 'error' ? (
        <View style={styles.stack}>
          <Text style={styles.fail}>加载失败:{state.detail}</Text>
          <Pressable style={styles.button} onPress={() => void reload()}>
            <Text style={styles.buttonText}>重试</Text>
          </Pressable>
          <Link href="/settings" style={styles.link}>
            检查后端设置
          </Link>
        </View>
      ) : null}

      {ready && childState.status !== 'selected' ? (
        childState.status === 'no-children' ? (
          <Text testID="daily-no-child" style={sharedStyles.muted}>
            还没有孩子成员。育儿记录只给孩子，请先到
            <Link href="/members" style={styles.link}>
              家庭成员
            </Link>
            添加孩子。
          </Text>
        ) : (
          <Text testID="daily-needs-child" style={sharedStyles.muted}>
            {variant === 'today' ? '请先选择今天要记录的孩子。' : '请先选择要统计的孩子。'}
          </Text>
        )
      ) : null}

      {variant === 'today' && childState.status === 'selected' ? (
        <TodayCareLog
          key={childState.child.member_id}
          child={childState.child}
          members={allMembers}
        />
      ) : null}
    </>
  );

  // Today fills the first screen and scrolls only the timeline inside; leftover stats
  // states (no child / needs selection) keep the existing page scroll.
  if (variant === 'today') {
    return (
      <View testID="daily-today-placeholder" style={styles.todayContent}>
        {content}
      </View>
    );
  }

  return (
    <ScrollView
      testID="daily-stats"
      style={styles.container}
      contentContainerStyle={styles.content}
    >
      {content}
    </ScrollView>
  );
}

/** Stats view owns the page scroll so pull-to-refresh reloads the same seven-day read
 *  model the charts use, without lifting the hook into the shared placeholder. */
function StatsDaily({ switcher, child }: { switcher: ReactNode; child: Member }) {
  const { state, reload } = useStatsCareEvents(child.member_id);
  const [refreshing, setRefreshing] = useState(false);

  const refresh = async () => {
    setRefreshing(true);
    try {
      await reload();
    } finally {
      setRefreshing(false);
    }
  };

  return (
    <ScrollView
      testID="daily-stats"
      style={styles.container}
      contentContainerStyle={styles.content}
      refreshControl={
        <RefreshControl
          refreshing={refreshing}
          onRefresh={() => void refresh()}
          tintColor={colors.accent}
          colors={[colors.accent]}
        />
      }
    >
      {switcher}
      <Text style={styles.title}>统计</Text>
      <StatsCareLog state={state} reload={reload} />
    </ScrollView>
  );
}

const styles = StyleSheet.create({
  container: { flex: 1, backgroundColor: colors.background },
  content: { padding: 24, gap: 12 },
  todayContent: { flex: 1, padding: 24, gap: 12, backgroundColor: colors.background },
  stack: { gap: 12 },
  title: { fontSize: 20, color: colors.text, fontWeight: '600' },
  fail: { fontSize: 13, color: colors.fail },
  link: { color: colors.accent, fontSize: 13 },
  button: {
    alignSelf: 'flex-start',
    paddingVertical: 10,
    paddingHorizontal: 16,
    backgroundColor: colors.accent,
    borderRadius: 8,
  },
  buttonText: { color: colors.onAccent, fontSize: 15 },
});
