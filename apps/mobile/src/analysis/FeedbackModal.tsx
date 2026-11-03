// 「这不对」反馈弹层(llm-analysis 票 09):只记录理由,不自动降级条目。
import { Modal, Pressable, StyleSheet, Text, TextInput, View } from 'react-native';

import { colors } from '../components/theme';

export function AnalysisFeedbackModal({
  visible,
  reason,
  busy,
  error,
  onChange,
  onSubmit,
  onClose,
}: {
  visible: boolean;
  reason: string;
  busy: boolean;
  error?: string;
  onChange: (value: string) => void;
  onSubmit: () => void;
  onClose: () => void;
}) {
  const valid = reason.trim().length > 0;
  return (
    <Modal visible={visible} transparent animationType="fade" onRequestClose={onClose}>
      <View style={styles.backdrop}>
        <Pressable style={StyleSheet.absoluteFill} onPress={onClose} />
        <View testID="analysis-feedback-modal" style={styles.sheet}>
          <Text style={styles.title}>哪里不对？</Text>
          <Text style={styles.hint}>
            写一句你看到的错误，例如「数值和报告不一致」「解释和病历冲突」。
          </Text>
          <TextInput
            testID="analysis-feedback-reason"
            value={reason}
            onChangeText={onChange}
            multiline
            maxLength={500}
            placeholder="请描述问题"
            placeholderTextColor={colors.muted}
            style={styles.input}
          />
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
  input: {
    minHeight: 96,
    borderWidth: 1,
    borderColor: colors.border,
    borderRadius: 10,
    paddingHorizontal: 12,
    paddingVertical: 10,
    fontSize: 15,
    color: colors.text,
    textAlignVertical: 'top',
  },
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
