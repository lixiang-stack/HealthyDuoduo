import { Link } from 'expo-router';
import { useEffect } from 'react';
import { ActivityIndicator, Pressable, ScrollView, StyleSheet, Text, View } from 'react-native';

import { sharedStyles } from '../components/styles';
import { colors } from '../components/theme';
import { useCurrentMember } from '../members/CurrentMemberContext';
import { childMembers, resolveCurrentChild } from '../members/currentChild';
import { useMembers } from '../members/useMembers';
import { ChildSwitcher } from './ChildSwitcher';

type Variant = 'today' | 'stats';

// B3 育儿二级视图的占位页:导航壳先行,护理事件数据 / 记录弹层 / 图表分别在
// 01、04、07 接入。这里先承载当前孩子上下文:单孩子锁定、多孩子选择、
// 无孩子空态;成人当前成员不影响育儿页。
export function DailyPlaceholder({ variant }: { variant: Variant }) {
  const { currentChildId, childReady, setCurrentChild, clearChild } = useCurrentMember();
  const { state, reload } = useMembers();

  const ready = state.status === 'ready' && childReady;
  const children = state.status === 'ready' ? childMembers(state.members) : [];
  const childState = resolveCurrentChild(
    state.status === 'ready' ? state.members : [],
    currentChildId,
  );
  const resolvedChildId =
    childState.status === 'selected' ? childState.child.member_id : undefined;

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

  return (
    <ScrollView
      testID={`daily-${variant}-placeholder`}
      style={styles.container}
      contentContainerStyle={styles.content}
    >
      {ready && childState.status !== 'no-children' ? (
        <ChildSwitcher
          childState={childState}
          members={children}
          onSelect={setCurrentChild}
        />
      ) : null}

      <Text style={styles.title}>{variant === 'today' ? '今天' : '统计'}</Text>

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

      {ready ? (
        childState.status === 'no-children' ? (
          <Text testID="daily-no-child" style={sharedStyles.muted}>
            还没有孩子成员。育儿记录只给孩子，请先到
            <Link href="/members" style={styles.link}>
              家庭成员
            </Link>
            添加孩子。
          </Text>
        ) : childState.status === 'needs-selection' ? (
          <Text testID="daily-needs-child" style={sharedStyles.muted}>
            {variant === 'today' ? '请先选择今天要记录的孩子。' : '请先选择要统计的孩子。'}
          </Text>
        ) : (
          <Text testID={`daily-${variant}-empty`} style={sharedStyles.muted}>
            {variant === 'today'
              ? `记录给 ${childState.child.name}。今天还没有记录，护理记录功能即将开放。`
              : `${childState.child.name} 还没有可统计的护理记录。4 个指标 × 3 种图表将在护理记录开放后显示。`}
          </Text>
        )
      ) : null}
    </ScrollView>
  );
}

const styles = StyleSheet.create({
  container: { flex: 1, backgroundColor: colors.background },
  content: { padding: 24, gap: 12 },
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
