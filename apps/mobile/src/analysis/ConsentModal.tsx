// 首次使用确认弹层(llm-analysis 票 04):只说明数据去了哪、内容不是诊断,
// 不使用「免责」措辞;确认后由调用方记入本地,后续不再展示。
import { Modal, Pressable, StyleSheet, Text, View } from 'react-native';

import { colors } from '../components/theme';
import { ANALYSIS_CONSENT_TEXT } from './consent';

export function AnalysisConsentModal({
  visible,
  busy,
  onConfirm,
  onCancel,
}: {
  visible: boolean;
  busy: boolean;
  onConfirm: () => void;
  onCancel: () => void;
}) {
  return (
    <Modal visible={visible} transparent animationType="fade" onRequestClose={onCancel}>
      <View style={styles.backdrop}>
        <Pressable style={StyleSheet.absoluteFill} onPress={onCancel} />
        <View testID="analysis-consent" style={styles.sheet}>
          <Text style={styles.title}>分析前请确认</Text>
          <Text style={styles.body}>{ANALYSIS_CONSENT_TEXT}</Text>
          <View style={styles.actions}>
            <Pressable
              testID="analysis-consent-cancel"
              style={styles.secondary}
              onPress={onCancel}
              disabled={busy}
            >
              <Text style={styles.secondaryText}>取消</Text>
            </Pressable>
            <Pressable
              testID="analysis-consent-confirm"
              style={[styles.primary, busy && styles.disabled]}
              onPress={onConfirm}
              disabled={busy}
            >
              <Text style={styles.primaryText}>我知道了，开始分析</Text>
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
  body: { fontSize: 14, color: colors.text, lineHeight: 22 },
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
