// 成员页展示组件(票 05):成员行、行内编辑、删除确认、添加成员表单。
// 共享一份控件样式;成员行点按设为当前成员。
import { ActivityIndicator, Pressable, StyleSheet, Text, TextInput, View } from 'react-native';

import type { Member } from '../api/members';
import { sharedStyles } from '../components/styles';
import { colors } from '../components/theme';

/** 行内编辑草稿;memberId 定位被编辑行。 */
export type EditDraft = { memberId: number; name: string; isChild: boolean };

// 成员行:点按设为当前成员,「编辑」进入行内改名/改档,「删除」进入确认面板。
export function MemberRow({
  member,
  selected,
  disabled,
  onSelect,
  onEdit,
  onDelete,
}: {
  member: Member;
  selected: boolean;
  disabled: boolean;
  onSelect: () => void;
  onEdit: () => void;
  onDelete: () => void;
}) {
  return (
    <View testID={`member-row-${member.member_id}`} style={[styles.card, selected && styles.cardSelected]}>
      <Pressable
        testID={`member-select-${member.member_id}`}
        style={styles.cardMain}
        onPress={onSelect}
        disabled={disabled}
      >
        <Text style={styles.memberName}>{member.name}</Text>
        <Text style={styles.memberMeta}>
          {member.is_child ? '孩子' : '成人'}
          {selected ? ' · 当前成员' : ''}
        </Text>
      </Pressable>
      <View style={styles.cardActions}>
        <Pressable onPress={onEdit} disabled={disabled}>
          <Text style={styles.cardAction}>编辑</Text>
        </Pressable>
        <Pressable
          testID={`member-delete-${member.member_id}`}
          onPress={onDelete}
          disabled={disabled}
        >
          <Text style={styles.cardDangerText}>删除</Text>
        </Pressable>
      </View>
    </View>
  );
}

// 行内编辑:改名 + 改是否孩子,保存后由页面重载列表;取消不改动任何数据。
export function MemberEditRow({
  draft,
  busy,
  onChange,
  onSave,
  onCancel,
}: {
  draft: EditDraft;
  busy: boolean;
  onChange: (patch: Partial<EditDraft>) => void;
  onSave: () => void;
  onCancel: () => void;
}) {
  return (
    <View style={styles.cardEditing}>
      <TextInput
        value={draft.name}
        onChangeText={(value) => onChange({ name: value })}
        placeholder="显示名"
        placeholderTextColor={colors.muted}
        style={styles.input}
      />
      <ChildPicker isChild={draft.isChild} onChange={(value) => onChange({ isChild: value })} />
      <View style={sharedStyles.horizontal}>
        <Pressable
          style={[styles.button, (busy || !draft.name.trim()) && styles.buttonDisabled]}
          onPress={onSave}
          disabled={busy || !draft.name.trim()}
        >
          <Text style={styles.buttonText}>保存</Text>
        </Pressable>
        <Pressable style={styles.buttonGhost} onPress={onCancel} disabled={busy}>
          <Text style={styles.cardAction}>取消</Text>
        </Pressable>
      </View>
    </View>
  );
}

// 删除成员确认面板:先显示报告数,再由家长确认不可恢复的级联删除。
export function MemberDeleteRow({
  member,
  reportCount,
  loading,
  error,
  busy,
  onRetry,
  onConfirm,
  onCancel,
}: {
  member: Member;
  reportCount: number | undefined;
  loading: boolean;
  error: string | undefined;
  busy: boolean;
  onRetry: () => void;
  onConfirm: () => void;
  onCancel: () => void;
}) {
  return (
    <View testID={`member-delete-panel-${member.member_id}`} style={styles.cardDanger}>
      <Text style={styles.memberName}>{member.name}</Text>
      {loading ? (
        <>
          <View style={sharedStyles.horizontal}>
            <ActivityIndicator />
            <Text style={sharedStyles.muted}>正在读取该成员的报告…</Text>
          </View>
          <Pressable
            testID={`member-delete-cancel-${member.member_id}`}
            style={styles.buttonGhost}
            onPress={onCancel}
            disabled={busy}
          >
            <Text style={styles.cardAction}>取消</Text>
          </Pressable>
        </>
      ) : error ? (
        <>
          <Text style={styles.fail}>读取报告失败:{error}</Text>
          <View style={sharedStyles.horizontal}>
            <Pressable
              testID={`member-delete-retry-${member.member_id}`}
              style={styles.buttonGhost}
              onPress={onRetry}
              disabled={busy}
            >
              <Text style={styles.cardAction}>重试</Text>
            </Pressable>
            <Pressable
              testID={`member-delete-cancel-${member.member_id}`}
              style={styles.buttonGhost}
              onPress={onCancel}
              disabled={busy}
            >
              <Text style={styles.cardAction}>取消</Text>
            </Pressable>
          </View>
        </>
      ) : reportCount !== undefined ? (
        <>
          <Text style={styles.deleteWarning}>
            {reportCount > 0
              ? `删除成员「${member.name}」及其 ${reportCount} 份报告？报告、指标项与修正记录将一并清除；原始图像与识别历史保留。此操作不可恢复。`
              : `删除成员「${member.name}」？此操作不可恢复。`}
          </Text>
          <View style={sharedStyles.horizontal}>
            <Pressable
              testID={`member-delete-confirm-${member.member_id}`}
              style={[styles.dangerButton, busy && styles.buttonDisabled]}
              onPress={onConfirm}
              disabled={busy}
            >
              <Text style={styles.buttonText}>确认删除</Text>
            </Pressable>
            <Pressable
              testID={`member-delete-cancel-${member.member_id}`}
              style={styles.buttonGhost}
              onPress={onCancel}
              disabled={busy}
            >
              <Text style={styles.cardAction}>取消</Text>
            </Pressable>
          </View>
        </>
      ) : null}
    </View>
  );
}

// 添加成员表单:名字 + 是否孩子两档;提交交给页面(状态在页面持有)。
export function MemberCreateForm({
  name,
  isChild,
  busy,
  onNameChange,
  onIsChildChange,
  onSubmit,
}: {
  name: string;
  isChild: boolean;
  busy: boolean;
  onNameChange: (value: string) => void;
  onIsChildChange: (value: boolean) => void;
  onSubmit: () => void;
}) {
  return (
    <View style={styles.cardForm}>
      <Text style={styles.sectionTitle}>添加成员</Text>
      <TextInput
        testID="members-name-input"
        value={name}
        onChangeText={onNameChange}
        placeholder="显示名(如:小虎、妈妈)"
        placeholderTextColor={colors.muted}
        style={styles.input}
      />
      <ChildPicker isChild={isChild} onChange={onIsChildChange} />
      <Pressable
        testID="members-create"
        style={[styles.button, (busy || !name.trim()) && styles.buttonDisabled]}
        onPress={onSubmit}
        disabled={busy || !name.trim()}
      >
        <Text style={styles.buttonText}>创建</Text>
      </Pressable>
    </View>
  );
}

// 「孩子 / 成人」两档选择(成员最小模型只有这两档)。
function ChildPicker({ isChild, onChange }: { isChild: boolean; onChange: (isChild: boolean) => void }) {
  return (
    <View style={sharedStyles.horizontal}>
      <Pressable style={[styles.chip, isChild && styles.chipActive]} onPress={() => onChange(true)}>
        <Text style={isChild ? styles.chipTextActive : styles.chipText}>孩子</Text>
      </Pressable>
      <Pressable style={[styles.chip, !isChild && styles.chipActive]} onPress={() => onChange(false)}>
        <Text style={!isChild ? styles.chipTextActive : styles.chipText}>成人</Text>
      </Pressable>
    </View>
  );
}

// 成员选择条(时间线过滤与上传归属共用的紧凑控件):点按切换选中成员;
// 排序由服务端给(创建序),组件只呈现。
export function MemberChips({
  members,
  selectedId,
  disabled,
  onSelect,
}: {
  members: Member[];
  selectedId: number | undefined;
  disabled?: boolean;
  onSelect: (memberId: number) => void;
}) {
  return (
    <View style={styles.chipsRow}>
      {members.map((m) => {
        const active = m.member_id === selectedId;
        return (
          <Pressable
            key={m.member_id}
            testID={`member-chip-${m.member_id}`}
            style={[styles.chip, active && styles.chipActive]}
            onPress={() => onSelect(m.member_id)}
            disabled={disabled}
          >
            <Text style={active ? styles.chipTextActive : styles.chipText}>{m.name}</Text>
          </Pressable>
        );
      })}
    </View>
  );
}

const styles = StyleSheet.create({
  chipsRow: { flexDirection: 'row', flexWrap: 'wrap', gap: 8 },
  sectionTitle: { fontSize: 13, color: colors.muted },
  input: {
    borderWidth: 1,
    borderColor: colors.border,
    borderRadius: 8,
    paddingHorizontal: 12,
    paddingVertical: 10,
    fontSize: 15,
  },
  card: {
    borderWidth: 1,
    borderColor: colors.border,
    borderRadius: 8,
    paddingHorizontal: 12,
    paddingVertical: 10,
    flexDirection: 'row',
    alignItems: 'center',
    gap: 12,
  },
  cardSelected: { borderColor: colors.accent },
  cardDanger: { borderWidth: 1, borderColor: colors.fail, borderRadius: 8, padding: 12, gap: 8 },
  cardActions: { flexDirection: 'row', gap: 12 },
  cardEditing: { borderWidth: 1, borderColor: colors.accent, borderRadius: 8, padding: 12, gap: 8 },
  cardForm: {
    borderWidth: 1,
    borderColor: colors.border,
    borderRadius: 8,
    padding: 12,
    gap: 8,
    marginTop: 8,
  },
  cardMain: { flex: 1, gap: 2 },
  cardAction: { color: colors.accent, fontSize: 15 },
  cardDangerText: { color: colors.fail, fontSize: 15 },
  memberName: { fontSize: 16, color: colors.text },
  memberMeta: { fontSize: 13, color: colors.muted },
  deleteWarning: { fontSize: 13, color: colors.text, lineHeight: 20 },
  fail: { fontSize: 13, color: colors.fail, lineHeight: 20 },
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
  button: {
    alignSelf: 'flex-start',
    paddingVertical: 10,
    paddingHorizontal: 16,
    backgroundColor: colors.accent,
    borderRadius: 8,
  },
  buttonDisabled: { backgroundColor: colors.border },
  dangerButton: {
    alignSelf: 'flex-start',
    paddingVertical: 10,
    paddingHorizontal: 16,
    backgroundColor: colors.fail,
    borderRadius: 8,
  },
  buttonGhost: { paddingVertical: 10, paddingHorizontal: 16 },
  buttonText: { color: colors.onAccent, fontSize: 15 },
});
