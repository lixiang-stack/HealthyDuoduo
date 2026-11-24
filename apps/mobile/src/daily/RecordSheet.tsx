// Today's record sheet: freezes the target child when opened, validates the
// type-specific fields and writes through the ticket-01 CareEvent repository.
import { useState } from 'react';
import {
  Modal,
  Pressable,
  ScrollView,
  StyleSheet,
  Text,
  View,
} from 'react-native';
import { SafeAreaView } from 'react-native-safe-area-context';

import type { Member } from '../api/members';
import { useCareEventRepository } from '../care/CareEventProvider';
import {
  CARE_EVENT_LABELS,
  CARE_EVENT_SOURCE_LABEL,
  duplicateFeedMessage,
} from '../care/careEventDisplay';
import type { CareEvent, CareEventBody, CareEventType } from '../care/careEvents';
import { duplicateFeedQueryRange, findDuplicateFeed } from '../care/duplicateFeed';
import {
  createRecordDraft,
  editRecordDraft,
  recordDraftErrors,
  recordDraftToBody,
  type RecordDraft,
  type RecordDraftErrors,
} from '../care/recordDraft';
import { colors } from '../components/theme';
import { RecordFields } from './RecordFields';

export type RecordSheetProps = {
  type: CareEventType;
  /** Candidate members; adult members are filtered out of targets by the sheet. */
  members: Member[];
  /** Current child at the moment the sheet opened. Frozen after mount. */
  initialTargetMemberId?: number;
  editingEvent?: CareEvent;
  onClose: () => void;
  onSaved: () => void;
};

export function RecordSheet({
  type,
  members,
  initialTargetMemberId,
  editingEvent,
  onClose,
  onSaved,
}: RecordSheetProps) {
  const repository = useCareEventRepository();
  const targets = members.filter((member) => member.is_child);
  const [draft, setDraft] = useState<RecordDraft>(() =>
    editingEvent ? editRecordDraft(editingEvent) : createRecordDraft(type, new Date()),
  );
  const [targetMemberId, setTargetMemberId] = useState<number | undefined>(() => {
    if (editingEvent) {
      return editingEvent.member_id;
    }
    if (
      initialTargetMemberId !== undefined &&
      targets.some((target) => target.member_id === initialTargetMemberId)
    ) {
      return initialTargetMemberId;
    }
    return targets.length === 1 ? targets[0].member_id : undefined;
  });
  const [submitted, setSubmitted] = useState(false);
  const [busy, setBusy] = useState(false);
  const [submitError, setSubmitError] = useState<string>();
  const [duplicate, setDuplicate] = useState<{
    previous: Extract<CareEvent, { type: 'feed' }>;
    body: CareEventBody;
    memberId: number;
  }>();
  const target = targets.find((candidate) => candidate.member_id === targetMemberId);
  const errors = recordDraftErrors(draft);
  const visibleErrors: RecordDraftErrors = submitted ? errors : {};
  const title = editingEvent ? '修改记录' : `记录${CARE_EVENT_LABELS[type]}`;
  const saveDisabled = busy || target === undefined;

  const updateDraft = (patch: Partial<RecordDraft>) =>
    setDraft((previous) => ({ ...previous, ...patch }));

  const persist = async (body: CareEventBody, memberId: number) => {
    if (editingEvent) {
      await repository.update(editingEvent.id, body);
    } else {
      await repository.create({ member_id: memberId, ...body });
    }
    onSaved();
  };

  const fail = (error: unknown) =>
    setSubmitError(error instanceof Error ? error.message : String(error));

  const submit = async () => {
    setSubmitted(true);
    setSubmitError(undefined);
    if (Object.keys(errors).length > 0 || target === undefined) {
      return;
    }
    const body = recordDraftToBody(draft);
    const memberId = target.member_id;
    // Best-effort guard before saving any feed, including an edit that moves the
    // feed into another feed's window. Two devices can still both save inside the
    // window; the server never deduplicates (ADR-0019).
    if (body.type === 'feed') {
      setBusy(true);
      try {
        const range = duplicateFeedQueryRange(body.started_at);
        const recent = await repository.list({ member_id: memberId, ...range });
        const previous = findDuplicateFeed(recent, {
          memberId,
          startedAt: body.started_at,
          editingEventId: editingEvent?.id,
        });
        if (previous !== undefined && previous.type === 'feed') {
          setDuplicate({ previous, body, memberId });
          return;
        }
      } catch (error) {
        fail(error);
        return;
      } finally {
        setBusy(false);
      }
    }
    setBusy(true);
    try {
      await persist(body, memberId);
    } catch (error) {
      fail(error);
    } finally {
      setBusy(false);
    }
  };

  const addDuplicate = async () => {
    if (!duplicate) {
      return;
    }
    const pending = duplicate;
    setDuplicate(undefined);
    setBusy(true);
    try {
      await persist(pending.body, pending.memberId);
    } catch (error) {
      fail(error);
    } finally {
      setBusy(false);
    }
  };

  const remove = async () => {
    if (!editingEvent) {
      return;
    }
    setBusy(true);
    setSubmitError(undefined);
    try {
      await repository.remove(editingEvent.id);
      onSaved();
    } catch (error) {
      fail(error);
    } finally {
      setBusy(false);
    }
  };

  return (
    <Modal
      visible
      transparent
      animationType="slide"
      onRequestClose={duplicate ? () => setDuplicate(undefined) : onClose}
    >
      <View style={styles.modalRoot}>
        <Pressable
          testID="record-sheet-backdrop"
          accessibilityRole="button"
          accessibilityLabel="关闭记录弹层"
          onPress={onClose}
          style={styles.backdrop}
        />
        <SafeAreaView edges={['bottom']} style={styles.safe}>
          <ScrollView
            testID="record-sheet"
            style={styles.sheet}
            contentContainerStyle={styles.content}
            keyboardShouldPersistTaps="handled"
          >
            <View style={styles.titleRow}>
              <Text testID="record-title" style={styles.title}>
                {title}
              </Text>
              <Text testID="record-source-tag" style={styles.tag}>
                {CARE_EVENT_SOURCE_LABEL}
              </Text>
            </View>
            <View style={styles.targetRow}>
              <Text testID="record-target" style={styles.target}>
                记录给 {target?.name ?? '未选择'}
              </Text>
              <Text testID="record-current-child-tag" style={styles.tag}>
                当前孩子
              </Text>
            </View>
            {targets.length > 1 && editingEvent === undefined ? (
              <View style={styles.chipRow}>
                {targets.map((candidate) => {
                  const active = candidate.member_id === targetMemberId;
                  return (
                    <Pressable
                      key={candidate.member_id}
                      testID={`record-target-option-${candidate.member_id}`}
                      accessibilityRole="button"
                      accessibilityLabel={candidate.name}
                      accessibilityState={{ selected: active }}
                      onPress={() => setTargetMemberId(candidate.member_id)}
                      style={[styles.chip, active && styles.chipActive]}
                    >
                      <Text style={active ? styles.chipTextActive : styles.chipText}>
                        {candidate.name}
                      </Text>
                    </Pressable>
                  );
                })}
              </View>
            ) : null}

            <RecordFields draft={draft} errors={visibleErrors} onChange={updateDraft} />

            <Text style={styles.hint}>
              只记录事实，不提供喂养建议；记录保存在家庭局域网。
            </Text>
            {submitError ? (
              <Text testID="record-submit-error" style={styles.error}>
                {submitError}
              </Text>
            ) : null}
            <Pressable
              testID="record-submit"
              accessibilityRole="button"
              accessibilityState={{ disabled: saveDisabled }}
              disabled={saveDisabled}
              onPress={() => void submit()}
              style={[styles.submit, saveDisabled && styles.submitDisabled]}
            >
              <Text style={styles.submitText}>
                {editingEvent ? '保存修改' : '保存记录'}
              </Text>
            </Pressable>
            {editingEvent ? (
              <Pressable
                testID="record-delete"
                accessibilityRole="button"
                disabled={busy}
                onPress={() => void remove()}
                style={styles.delete}
              >
                <Text style={styles.deleteText}>删除这条记录</Text>
              </Pressable>
            ) : null}
          </ScrollView>
        </SafeAreaView>
        {duplicate ? (
          <View testID="duplicate-feed-confirm" style={styles.confirmRoot}>
            <View style={styles.confirmBackdrop} />
            <View style={styles.confirmCard}>
              <Text testID="duplicate-feed-message" style={styles.confirmText}>
                {duplicateFeedMessage(duplicate.previous)}
              </Text>
              <Pressable
                testID="duplicate-feed-continue"
                accessibilityRole="button"
                onPress={() => void addDuplicate()}
                style={styles.confirmPrimary}
              >
                <Text style={styles.confirmPrimaryText}>仍然添加</Text>
              </Pressable>
              <Pressable
                testID="duplicate-feed-back"
                accessibilityRole="button"
                onPress={() => setDuplicate(undefined)}
                style={styles.confirmSecondary}
              >
                <Text style={styles.confirmSecondaryText}>返回修改</Text>
              </Pressable>
            </View>
          </View>
        ) : null}
      </View>
    </Modal>
  );
}

const styles = StyleSheet.create({
  modalRoot: { flex: 1, justifyContent: 'flex-end' },
  backdrop: {
    position: 'absolute',
    top: 0,
    right: 0,
    bottom: 0,
    left: 0,
    backgroundColor: colors.backdrop,
  },
  safe: {
    backgroundColor: colors.background,
    borderTopLeftRadius: 16,
    borderTopRightRadius: 16,
    maxHeight: '90%',
  },
  sheet: { maxHeight: '100%' },
  content: { padding: 20, gap: 10 },
  titleRow: { flexDirection: 'row', alignItems: 'center', gap: 8 },
  title: { fontSize: 18, color: colors.text, fontWeight: '600' },
  targetRow: { flexDirection: 'row', alignItems: 'center', gap: 8 },
  target: { fontSize: 14, color: colors.text },
  tag: {
    fontSize: 12,
    color: colors.muted,
    borderWidth: 1,
    borderColor: colors.border,
    borderRadius: 10,
    paddingHorizontal: 8,
    paddingVertical: 2,
  },
  chipRow: { flexDirection: 'row', flexWrap: 'wrap', gap: 8 },
  chip: {
    borderWidth: 1,
    borderColor: colors.border,
    borderRadius: 16,
    paddingHorizontal: 14,
    paddingVertical: 6,
  },
  chipActive: { borderColor: colors.accent, backgroundColor: colors.accent },
  chipText: { fontSize: 14, color: colors.text },
  chipTextActive: { fontSize: 14, color: colors.onAccent },
  hint: { fontSize: 12, color: colors.muted, lineHeight: 18 },
  error: { fontSize: 12, color: colors.fail },
  submit: {
    alignItems: 'center',
    paddingVertical: 13,
    borderRadius: 10,
    backgroundColor: colors.accent,
  },
  submitDisabled: { backgroundColor: colors.border },
  submitText: { fontSize: 16, color: colors.onAccent, fontWeight: '600' },
  delete: { alignItems: 'center', paddingVertical: 10 },
  deleteText: { fontSize: 14, color: colors.fail },
  confirmRoot: {
    position: 'absolute',
    top: 0,
    right: 0,
    bottom: 0,
    left: 0,
    alignItems: 'center',
    justifyContent: 'center',
    padding: 24,
  },
  confirmBackdrop: {
    position: 'absolute',
    top: 0,
    right: 0,
    bottom: 0,
    left: 0,
    backgroundColor: colors.backdrop,
  },
  confirmCard: {
    width: '100%',
    gap: 12,
    padding: 20,
    borderRadius: 16,
    backgroundColor: colors.background,
  },
  confirmText: { fontSize: 15, lineHeight: 22, color: colors.text },
  confirmPrimary: {
    alignItems: 'center',
    paddingVertical: 12,
    borderRadius: 10,
    backgroundColor: colors.accent,
  },
  confirmPrimaryText: { fontSize: 15, color: colors.onAccent, fontWeight: '600' },
  confirmSecondary: {
    alignItems: 'center',
    paddingVertical: 12,
    borderRadius: 10,
    borderWidth: 1,
    borderColor: colors.border,
  },
  confirmSecondaryText: { fontSize: 15, color: colors.text },
});
