import { useFocusEffect } from 'expo-router';
import { useCallback, useState } from 'react';
import { ActivityIndicator, Pressable, ScrollView, StyleSheet, Text, View } from 'react-native';

import { createMember, listMembers, updateMember, type Member } from '../api/members';
import { resolveApiClient } from '../api/resolveClient';
import { sharedStyles } from '../components/styles';
import { colors } from '../components/theme';
import { MemberCreateForm, MemberEditRow, MemberRow, type EditDraft } from '../members/components';
import { useCurrentMember } from '../members/CurrentMemberContext';

// 成员页(票 05):几十秒配好全家档案——建成员(显示名 + 是否孩子)、改名、改档;
// 点选当前成员(经 CurrentMemberContext 供时间线过滤与上传归属默认消费,票 06+)。
// 列表按创建序;无删除入口(契约与交互都不提供成员删除)。

type ScreenState =
  | { status: 'loading' }
  | { status: 'unconfigured' }
  | { status: 'error'; detail: string }
  | { status: 'ready'; members: Member[] };

export default function MembersScreen() {
  const { currentMemberId, selectMember } = useCurrentMember();
  const [state, setState] = useState<ScreenState>({ status: 'loading' });
  const [name, setName] = useState('');
  const [isChild, setIsChild] = useState(false);
  const [editing, setEditing] = useState<EditDraft>();
  const [busy, setBusy] = useState(false);
  const [actionError, setActionError] = useState<string>();

  const reload = useCallback(async () => {
    const resolved = await resolveApiClient();
    if (!resolved) {
      setState({ status: 'unconfigured' });
      return;
    }
    try {
      setState({ status: 'ready', members: await listMembers(resolved.client) });
    } catch (error) {
      setState({ status: 'error', detail: error instanceof Error ? error.message : String(error) });
    }
  }, []);

  useFocusEffect(
    useCallback(() => {
      void reload();
    }, [reload]),
  );

  // 建/改共用的动作骨架:busy 门 + 错误呈现一处收口。
  const runAction = useCallback(async (action: () => Promise<void>) => {
    setBusy(true);
    setActionError(undefined);
    try {
      await action();
    } catch (error) {
      setActionError(error instanceof Error ? error.message : String(error));
    } finally {
      setBusy(false);
    }
  }, []);

  const onCreate = useCallback(() => {
    const trimmed = name.trim();
    if (!trimmed) {
      return;
    }
    void runAction(async () => {
      const resolved = await resolveApiClient();
      if (!resolved) {
        setState({ status: 'unconfigured' });
        return;
      }
      await createMember(resolved.client, { name: trimmed, is_child: isChild });
      setName('');
      setIsChild(false);
      await reload();
    });
  }, [name, isChild, runAction, reload]);

  const onSaveEdit = useCallback(() => {
    if (!editing) {
      return;
    }
    const trimmed = editing.name.trim();
    if (!trimmed) {
      return;
    }
    void runAction(async () => {
      const resolved = await resolveApiClient();
      if (!resolved) {
        setState({ status: 'unconfigured' });
        return;
      }
      await updateMember(resolved.client, editing.memberId, {
        name: trimmed,
        is_child: editing.isChild,
      });
      setEditing(undefined);
      await reload();
    });
  }, [editing, runAction, reload]);

  const members = state.status === 'ready' ? state.members : [];

  return (
    <ScrollView style={styles.container} contentContainerStyle={styles.content}>
      {state.status === 'loading' ? (
        <View style={sharedStyles.horizontal}>
          <ActivityIndicator />
          <Text style={sharedStyles.muted}>正在加载成员…</Text>
        </View>
      ) : null}

      {state.status === 'unconfigured' ? (
        <Text style={sharedStyles.muted}>尚未配置后端地址,请先到「设置」填写。</Text>
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
            <Text style={sharedStyles.muted}>还没有家庭成员。先给家人建一个:填显示名,选择是否孩子。</Text>
          ) : (
            members.map((m) => {
              if (editing?.memberId === m.member_id) {
                return (
                  <MemberEditRow
                    key={m.member_id}
                    draft={editing}
                    busy={busy}
                    onChange={(patch) => setEditing({ ...editing, ...patch })}
                    onSave={() => void onSaveEdit()}
                    onCancel={() => setEditing(undefined)}
                  />
                );
              }
              return (
                <MemberRow
                  key={m.member_id}
                  member={m}
                  selected={m.member_id === currentMemberId}
                  disabled={busy}
                  onSelect={() => selectMember(m.member_id)}
                  onEdit={() =>
                    setEditing({ memberId: m.member_id, name: m.name, isChild: m.is_child })
                  }
                />
              );
            })
          )}
          {actionError ? <Text style={styles.fail}>{actionError}</Text> : null}
          <MemberCreateForm
            name={name}
            isChild={isChild}
            busy={busy}
            onNameChange={setName}
            onIsChildChange={setIsChild}
            onSubmit={() => void onCreate()}
          />
        </View>
      ) : null}
    </ScrollView>
  );
}

const styles = StyleSheet.create({
  container: { flex: 1, backgroundColor: colors.background },
  content: { padding: 24, gap: 12 },
  stack: { gap: 12 },
  fail: { fontSize: 13, color: colors.fail },
  button: {
    alignSelf: 'flex-start',
    paddingVertical: 10,
    paddingHorizontal: 16,
    backgroundColor: colors.accent,
    borderRadius: 8,
  },
  buttonText: { color: colors.onAccent, fontSize: 15 },
});
