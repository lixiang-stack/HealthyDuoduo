// 成员页展示组件(票 05):成员行、行内编辑、删除确认、添加成员表单。
// 共享一份控件样式;成员行点按设为当前成员。
import { useState } from 'react';
import { ActivityIndicator, Pressable, StyleSheet, Text, TextInput, View } from 'react-native';

import type { Member } from '../api/members';
import { sharedStyles } from '../components/styles';
import { colors } from '../components/theme';
import { growthSummary } from './growth';
import { memberFormErrors, type MemberSex } from './profile';
import { BirthDateField, SexField } from './profilePickers';

/** 行内编辑草稿;memberId 定位被编辑行。 */
export type EditDraft = {
  memberId: number;
  name: string;
  sex: MemberSex;
  birthDate: string;
};

// 成员行:点按设为当前成员,「身高体重」展开记录表单,「编辑」进入行内改名/改档,
// 「删除」进入确认面板;有生长记录时显示最近一次测量与日期。
export function MemberRow({
  member,
  selected,
  disabled,
  onSelect,
  onGrowth,
  onEdit,
  onDelete,
}: {
  member: Member;
  selected: boolean;
  disabled: boolean;
  onSelect: () => void;
  onGrowth: () => void;
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
        {member.latest_growth ? (
          <Text testID={`member-growth-${member.member_id}`} style={styles.memberGrowth}>
            {growthSummary(member.latest_growth)}
          </Text>
        ) : null}
      </Pressable>
      <View style={styles.cardActions}>
        <Pressable testID={`member-growth-action-${member.member_id}`} onPress={onGrowth} disabled={disabled}>
          <Text style={styles.cardAction}>身高体重</Text>
        </Pressable>
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

// 行内编辑:改名 / 改性别与出生日期,保存后由页面重载列表;取消不改动任何数据。
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
  const [submitted, setSubmitted] = useState(false);
  const errors = memberFormErrors(draft.name, draft.sex, draft.birthDate);
  const valid = Object.keys(errors).length === 0;
  const attemptSave = () => {
    setSubmitted(true);
    if (valid) {
      onSave();
    }
  };
  return (
    <View style={styles.cardEditing}>
      <RequiredLabel>称呼</RequiredLabel>
      <TextInput
        testID={`member-edit-name-${draft.memberId}`}
        value={draft.name}
        onChangeText={(value) => onChange({ name: value })}
        placeholder="如:小虎、妈妈"
        placeholderTextColor={colors.muted}
        style={styles.input}
      />
      {submitted && errors.name ? (
        <Text testID={`member-edit-name-error-${draft.memberId}`} style={styles.fieldError}>
          {errors.name}
        </Text>
      ) : null}
      <MemberProfileFields
        prefix={`member-edit-${draft.memberId}`}
        sex={draft.sex}
        birthDate={draft.birthDate}
        errors={submitted ? errors : undefined}
        onChange={onChange}
      />
      <View style={sharedStyles.horizontal}>
        <Pressable
          testID={`member-edit-save-${draft.memberId}`}
          style={[styles.button, busy && styles.buttonDisabled]}
          onPress={attemptSave}
          disabled={busy}
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

// 添加成员表单:名字 + 性别 + 出生日期;提交交给页面(状态在页面持有)。
export function MemberCreateForm({
  name,
  sex,
  birthDate,
  busy,
  onNameChange,
  onProfileChange,
  onSubmit,
}: {
  name: string;
  sex: MemberSex | undefined;
  birthDate: string;
  busy: boolean;
  onNameChange: (value: string) => void;
  onProfileChange: (patch: { sex?: MemberSex; birthDate?: string }) => void;
  onSubmit: () => void;
}) {
  const [submitted, setSubmitted] = useState(false);
  const errors = memberFormErrors(name, sex, birthDate);
  const valid = Object.keys(errors).length === 0;
  const attemptCreate = () => {
    setSubmitted(true);
    if (valid) {
      onSubmit();
    }
  };
  return (
    <View style={styles.cardForm}>
      <Text style={styles.sectionTitle}>添加成员</Text>
      <RequiredLabel>称呼</RequiredLabel>
      <TextInput
        testID="members-name-input"
        value={name}
        onChangeText={onNameChange}
        placeholder="如:小虎、妈妈"
        placeholderTextColor={colors.muted}
        style={styles.input}
      />
      {submitted && errors.name ? (
        <Text testID="members-name-error" style={styles.fieldError}>
          {errors.name}
        </Text>
      ) : null}
      <MemberProfileFields
        prefix="members"
        sex={sex}
        birthDate={birthDate}
        errors={submitted ? errors : undefined}
        onChange={onProfileChange}
      />
      <Pressable
        testID="members-create"
        style={[styles.button, busy && styles.buttonDisabled]}
        onPress={attemptCreate}
        disabled={busy}
      >
        <Text style={styles.buttonText}>创建</Text>
      </Pressable>
    </View>
  );
}

/** 成员资料字段:性别下拉 + 出生日期滚轮。 */
function MemberProfileFields({
  prefix,
  sex,
  birthDate,
  errors,
  onChange,
}: {
  prefix: string;
  sex: MemberSex | undefined;
  birthDate: string;
  errors?: { sex?: string; birthDate?: string };
  onChange: (patch: { sex?: MemberSex; birthDate?: string }) => void;
}) {
  return (
    <View style={styles.profileFields}>
      <SexField
        prefix={prefix}
        value={sex}
        error={errors?.sex}
        onChange={(value) => onChange({ sex: value })}
      />
      <BirthDateField
        prefix={prefix}
        value={birthDate}
        error={errors?.birthDate}
        onChange={(value) => onChange({ birthDate: value })}
      />
    </View>
  );
}

/** 必填字段标签:红色星号比整句说明更贴近字段。 */
function RequiredLabel({ children }: { children: string }) {
  return (
    <Text style={sharedStyles.label}>
      {children}
      <Text style={styles.required}> *</Text>
    </Text>
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
  memberGrowth: { fontSize: 12, color: colors.muted, lineHeight: 18 },
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
  profileFields: { gap: 8 },
  required: { color: colors.fail },
  fieldError: { fontSize: 12, color: colors.fail },
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
