import { useFocusEffect } from 'expo-router';
import { useCallback, useRef, useState } from 'react';
import { ActivityIndicator, Pressable, ScrollView, StyleSheet, Text, View } from 'react-native';

import { createMember, deleteMember, listMembers, updateMember, type Member } from '../api/members';
import { listReports } from '../api/reports';
import { resolveApiClient } from '../api/resolveClient';
import { sharedStyles } from '../components/styles';
import { colors } from '../components/theme';
import {
  MemberCreateForm,
  MemberDeleteRow,
  MemberEditRow,
  MemberRow,
  type EditDraft,
} from '../members/components';
import { useCurrentMember } from '../members/CurrentMemberContext';

// 成员页(票 05):几十秒配好全家档案——建成员(显示名 + 是否孩子)、改名、改档;
// 点选当前成员(经 CurrentMemberContext 供时间线过滤与上传归属默认消费,票 06+)。
// 列表按创建序;删除成员会一并删除其全部报告(原图与 OCR 历史保留),不可恢复。

type ScreenState =
  | { status: 'loading' }
  | { status: 'unconfigured' }
  | { status: 'error'; detail: string }
  | { status: 'ready'; members: Member[] };

/** 删除确认态:报告数是确认文案的一部分,取不到就不允许确认。 */
type DeleteState = {
  member: Member;
  reportCount?: number;
  loading: boolean;
  error?: string;
};

export default function MembersScreen() {
  const { currentMemberId, selectMember, clearMember } = useCurrentMember();
  const [state, setState] = useState<ScreenState>({ status: 'loading' });
  const [name, setName] = useState('');
  const [isChild, setIsChild] = useState(false);
  const [editing, setEditing] = useState<EditDraft>();
  const [deleting, setDeleting] = useState<DeleteState>();
  const [busy, setBusy] = useState(false);
  const [actionError, setActionError] = useState<string>();
  // 读取报告数可能被取消;只允许最后一次请求写删除确认态。
  const deleteRequestId = useRef(0);

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

  // 删除前先取该成员报告数:确认文案要写清「及 N 份报告」;取不到就禁止确认。
  const loadDeleteCount = useCallback((member: Member) => {
    const request = ++deleteRequestId.current;
    setEditing(undefined);
    setDeleting({ member, loading: true });
    setActionError(undefined);
    void (async () => {
      const resolved = await resolveApiClient();
      if (request !== deleteRequestId.current) {
        return;
      }
      if (!resolved) {
        setState({ status: 'unconfigured' });
        setDeleting(undefined);
        return;
      }
      try {
        const reports = await listReports(resolved.client, member.member_id);
        if (request !== deleteRequestId.current) {
          return;
        }
        setDeleting({ member, reportCount: reports.length, loading: false });
      } catch (error) {
        if (request !== deleteRequestId.current) {
          return;
        }
        setDeleting({
          member,
          loading: false,
          error: error instanceof Error ? error.message : String(error),
        });
      }
    })();
  }, []);

  const onConfirmDelete = useCallback(() => {
    if (!deleting || deleting.reportCount === undefined) {
      return;
    }
    const target = deleting.member;
    void runAction(async () => {
      const resolved = await resolveApiClient();
      if (!resolved) {
        setState({ status: 'unconfigured' });
        return;
      }
      await deleteMember(resolved.client, target.member_id);
      if (currentMemberId === target.member_id) {
        clearMember();
      }
      setDeleting(undefined);
      await reload();
    });
  }, [deleting, runAction, currentMemberId, clearMember, reload]);

  const onCancelDelete = useCallback(() => {
    deleteRequestId.current++;
    setDeleting(undefined);
  }, []);

  const members = state.status === 'ready' ? state.members : [];
  // 读取报告数期间禁用其他成员操作,避免两个删除确认的异步结果互相覆盖。
  const actionsDisabled = busy || deleting?.loading === true;

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
              if (deleting?.member.member_id === m.member_id) {
                return (
                  <MemberDeleteRow
                    key={m.member_id}
                    member={deleting.member}
                    reportCount={deleting.reportCount}
                    loading={deleting.loading}
                    error={deleting.error}
                    busy={busy}
                    onRetry={() => loadDeleteCount(deleting.member)}
                    onConfirm={onConfirmDelete}
                    onCancel={onCancelDelete}
                  />
                );
              }
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
                  disabled={actionsDisabled}
                  onSelect={() => selectMember(m.member_id)}
                  onEdit={() => {
                    deleteRequestId.current++;
                    setDeleting(undefined);
                    setEditing({ memberId: m.member_id, name: m.name, isChild: m.is_child });
                  }}
                  onDelete={() => loadDeleteCount(m)}
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
