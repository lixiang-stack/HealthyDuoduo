// 成员页展示组件(票 05):成员行、行内编辑、添加成员表单。
// 共享一份控件样式;成员行点按设为当前成员,无删除入口(契约与交互都不提供)。
import { Pressable, StyleSheet, Text, TextInput, View } from 'react-native';

import type { Member } from '../api/members';
import { colors } from '../components/theme';

/** 行内编辑草稿;memberId 定位被编辑行。 */
export type EditDraft = { memberId: number; name: string; isChild: boolean };

// 成员行:点按设为当前成员,「编辑」进入行内改名/改档。
export function MemberRow({
  member,
  selected,
  disabled,
  onSelect,
  onEdit,
}: {
  member: Member;
  selected: boolean;
  disabled: boolean;
  onSelect: () => void;
  onEdit: () => void;
}) {
  return (
    <View style={[styles.card, selected && styles.cardSelected]}>
      <Pressable style={styles.cardMain} onPress={onSelect} disabled={disabled}>
        <Text style={styles.memberName}>{member.name}</Text>
        <Text style={styles.memberMeta}>
          {member.is_child ? '孩子' : '成人'}
          {selected ? ' · 当前成员' : ''}
        </Text>
      </Pressable>
      <Pressable onPress={onEdit} disabled={disabled}>
        <Text style={styles.cardAction}>编辑</Text>
      </Pressable>
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
      <View style={styles.horizontal}>
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
        value={name}
        onChangeText={onNameChange}
        placeholder="显示名(如:小虎、妈妈)"
        placeholderTextColor={colors.muted}
        style={styles.input}
      />
      <ChildPicker isChild={isChild} onChange={onIsChildChange} />
      <Pressable
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
    <View style={styles.horizontal}>
      <Pressable style={[styles.chip, isChild && styles.chipActive]} onPress={() => onChange(true)}>
        <Text style={isChild ? styles.chipTextActive : styles.chipText}>孩子</Text>
      </Pressable>
      <Pressable style={[styles.chip, !isChild && styles.chipActive]} onPress={() => onChange(false)}>
        <Text style={!isChild ? styles.chipTextActive : styles.chipText}>成人</Text>
      </Pressable>
    </View>
  );
}

const styles = StyleSheet.create({
  horizontal: { flexDirection: 'row', alignItems: 'center', gap: 8 },
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
  memberName: { fontSize: 16, color: colors.text },
  memberMeta: { fontSize: 13, color: colors.muted },
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
  buttonGhost: { paddingVertical: 10, paddingHorizontal: 16 },
  buttonText: { color: colors.onAccent, fontSize: 15 },
});
