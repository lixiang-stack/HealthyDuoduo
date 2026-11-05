// 分析反馈弹层(llm-analysis 票 09,分析结果 UX 票 05/08):结果级与条目级共用。
// 原因固定分类,便于聚合;不发送健康内容,只记录分类与可选 entry_id。
import { Modal, Pressable, StyleSheet, Text, View } from 'react-native';

import { colors } from '../components/theme';

export const FEEDBACK_REASON_OPTIONS = [
  { value: 'value_wrong', label: '数值不对' },
  { value: 'explanation_wrong', label: '解释不对' },
  { value: 'note_conflict', label: '病历冲突' },
  { value: 'unclear', label: '看不懂' },
  { value: 'other', label: '其他' },
] as const;

export type FeedbackReason = (typeof FEEDBACK_REASON_OPTIONS)[number]['value'];

export function AnalysisFeedbackModal({
  visible,
  reason,
  busy,
  error,
  entryId,
  onChange,
  onSubmit,
  onClose,
}: {
  visible: boolean;
  reason: FeedbackReason | '';
  busy: boolean;
  error?: string;
  entryId?: string;
  onChange: (value: FeedbackReason) => void;
  onSubmit: () => void;
  onClose: () => void;
}) {
  const valid = reason.trim().length > 0;
  return (
    <Modal visible={visible} transparent animationType="fade" onRequestClose={onClose}>
      <View style={styles.backdrop}>
        <Pressable style={StyleSheet.absoluteFill} onPress={onClose} />
        <View testID="analysis-feedback-modal" style={styles.sheet}>
          <Text style={styles.title}>
            {entryId ? '这段内容哪里需要改进？' : '分析结果哪里需要改进？'}
          </Text>
          <Text style={styles.hint}>
            {entryId
              ? '选择问题类型；只记录这段解读的条目编号，不包含健康内容。'
              : '选择问题类型；只记录分类，不包含报告原文或健康内容。'}
          </Text>
          <View style={styles.reasons}>
            {FEEDBACK_REASON_OPTIONS.map((option) => {
              const active = option.value === reason;
              return (
                <Pressable
                  key={option.value}
                  testID={`analysis-feedback-reason-${option.value}`}
                  accessibilityRole="button"
                  accessibilityLabel={option.label}
                  accessibilityState={{ selected: active }}
                  onPress={() => onChange(option.value)}
                  style={[styles.reason, active && styles.reasonActive]}
                >
                  <Text style={active ? styles.reasonTextActive : styles.reasonText}>
                    {option.label}
                  </Text>
                </Pressable>
              );
            })}
          </View>
          {error ? (
            <Text testID="analysis-feedback-error" style={styles.error}>
              {error}
            </Text>
          ) : null}
          <View style={styles.actions}>
            <Pressable
              testID="analysis-feedback-cancel"
              style={styles.secondary}
              onPress={onClose}
              disabled={busy}
            >
              <Text style={styles.secondaryText}>取消</Text>
            </Pressable>
            <Pressable
              testID="analysis-feedback-submit"
              style={[styles.primary, (!valid || busy) && styles.disabled]}
              onPress={onSubmit}
              disabled={!valid || busy}
            >
              <Text style={styles.primaryText}>{busy ? '提交中…' : '提交反馈'}</Text>
            </Pressable>
          </View>
        </View>
      </View>
    </Modal>
  );
}

const styles = StyleSheet.create({
  backdrop: {
    flex: 1,
    backgroundColor: colors.backdrop,
    justifyContent: 'center',
    paddingHorizontal: 24,
  },
  sheet: {
    backgroundColor: colors.background,
    borderRadius: 16,
    padding: 20,
    gap: 12,
  },
  title: { fontSize: 18, fontWeight: '600', color: colors.text },
  hint: { fontSize: 13, color: colors.muted, lineHeight: 20 },
  reasons: { flexDirection: 'row', flexWrap: 'wrap', gap: 8 },
  reason: {
    minHeight: 44,
    justifyContent: 'center',
    borderWidth: 1,
    borderColor: colors.border,
    borderRadius: 10,
    paddingHorizontal: 12,
  },
  reasonActive: { borderColor: colors.accent, backgroundColor: colors.accent },
  reasonText: { fontSize: 14, color: colors.text },
  reasonTextActive: { fontSize: 14, color: colors.onAccent, fontWeight: '600' },
  error: { fontSize: 12, color: colors.fail },
  actions: { flexDirection: 'row', gap: 12, marginTop: 4 },
  secondary: {
    flex: 1,
    borderWidth: 1,
    borderColor: colors.border,
    borderRadius: 10,
    paddingVertical: 12,
    alignItems: 'center',
  },
  secondaryText: { fontSize: 15, color: colors.text },
  primary: {
    flex: 2,
    borderRadius: 10,
    paddingVertical: 12,
    alignItems: 'center',
    backgroundColor: colors.accent,
  },
  primaryText: { fontSize: 15, color: colors.onAccent, fontWeight: '600' },
  disabled: { backgroundColor: colors.border },
});
