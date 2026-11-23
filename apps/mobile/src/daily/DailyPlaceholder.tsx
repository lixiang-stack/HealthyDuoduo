import { Link } from 'expo-router';
import { ActivityIndicator, Pressable, ScrollView, StyleSheet, Text, View } from 'react-native';

import { sharedStyles } from '../components/styles';
import { colors } from '../components/theme';
import { useCurrentMember } from '../members/CurrentMemberContext';
import { useMembers } from '../members/useMembers';

type Variant = 'today' | 'stats';

// B3 日常二级视图的占位页:导航壳先行,护理事件数据 / 记录弹层 / 图表分别在 01、
// 03、04、07 接入。这里只负责把「当前成员是成人 / 没有孩子」的真实边界说清楚,
// 不把成人当成护理目标,也不伪造事件。
export function DailyPlaceholder({ variant }: { variant: Variant }) {
  const { currentMemberId } = useCurrentMember();
  const { state, reload } = useMembers();

  const children = state.status === 'ready' ? state.members.filter((member) => member.is_child) : [];
  const current =
    state.status === 'ready'
      ? state.members.find((member) => member.member_id === currentMemberId)
      : undefined;

  return (
    <ScrollView
      testID={`daily-${variant}-placeholder`}
      style={styles.container}
      contentContainerStyle={styles.content}
    >
      <Text style={styles.title}>{variant === 'today' ? '今天' : '统计'}</Text>

      {state.status === 'loading' ? (
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

      {state.status === 'ready' ? (
        children.length === 0 ? (
          <Text testID="daily-no-child" style={sharedStyles.muted}>
            还没有孩子成员。日常记录只给孩子，请先到
            <Link href="/members" style={styles.link}>
              家庭成员
            </Link>
            添加孩子。
          </Text>
        ) : current && !current.is_child ? (
          <Text testID="daily-adult-guidance" style={sharedStyles.muted}>
            当前成员「{current.name}」是成人。日常记录只给孩子，请到
            <Link href="/members" style={styles.link}>
              家庭成员
            </Link>
            选择或添加孩子。
          </Text>
        ) : (
          <Text testID={`daily-${variant}-empty`} style={sharedStyles.muted}>
            {variant === 'today'
              ? '今天还没有记录。护理记录功能即将开放。'
              : '还没有可统计的护理记录。4 个指标 × 3 种图表将在护理记录开放后显示。'}
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
