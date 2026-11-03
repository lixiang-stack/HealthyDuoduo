// 生长记录表单(llm-analysis 票 02):测量日期 + 身高 / 体重(至少一项) + 来源。
// 日期用 YYYY-MM-DD 文本输入,避免为一次性表单复制出生日期滚轮。
import { useState } from 'react';
import { Pressable, StyleSheet, Text, TextInput, View } from 'react-native';

import { sharedStyles } from '../components/styles';
import { colors } from '../components/theme';
import {
  GROWTH_SOURCE_OPTIONS,
  growthFormErrors,
  type GrowthDraft,
} from './growth';

export function GrowthForm({
  draft,
  busy,
  onChange,
  onSubmit,
  onCancel,
}: {
  draft: GrowthDraft;
  busy: boolean;
  onChange: (patch: Partial<GrowthDraft>) => void;
  onSubmit: () => void;
  onCancel: () => void;
}) {
  const [submitted, setSubmitted] = useState(false);
  const errors = growthFormErrors(draft);
  const valid = Object.keys(errors).length === 0;
  const attemptSubmit = () => {
    setSubmitted(true);
    if (valid) {
      onSubmit();
    }
  };
  return (
    <View style={styles.cardForm}>
      <Text style={styles.sectionTitle}>记录生长</Text>
      <Text style={sharedStyles.label}>测量日期 *</Text>
      <TextInput
        testID="growth-measured-on"
        value={draft.measuredOn}
        onChangeText={(value) => onChange({ measuredOn: value })}
        placeholder="YYYY-MM-DD"
        placeholderTextColor={colors.muted}
        style={styles.input}
      />
      {submitted && errors.measuredOn ? (
        <Text testID="growth-measured-on-error" style={styles.fieldError}>
          {errors.measuredOn}
        </Text>
      ) : null}
      <View style={styles.measureRow}>
        <View style={styles.measureField}>
          <Text style={sharedStyles.label}>身高(cm)</Text>
          <TextInput
            testID="growth-height"
            value={draft.height}
            onChangeText={(value) => onChange({ height: value })}
            keyboardType="decimal-pad"
            placeholder="如 118.5"
            placeholderTextColor={colors.muted}
            style={styles.input}
          />
          {submitted && errors.height ? (
            <Text testID="growth-height-error" style={styles.fieldError}>
              {errors.height}
            </Text>
          ) : null}
        </View>
        <View style={styles.measureField}>
          <Text style={sharedStyles.label}>体重(kg)</Text>
          <TextInput
            testID="growth-weight"
            value={draft.weight}
            onChangeText={(value) => onChange({ weight: value })}
            keyboardType="decimal-pad"
            placeholder="如 22.4"
            placeholderTextColor={colors.muted}
            style={styles.input}
          />
          {submitted && errors.weight ? (
            <Text testID="growth-weight-error" style={styles.fieldError}>
              {errors.weight}
            </Text>
          ) : null}
        </View>
      </View>
      {submitted && errors.measurement ? (
        <Text testID="growth-measurement-error" style={styles.fieldError}>
          {errors.measurement}
        </Text>
      ) : null}
      <Text style={sharedStyles.label}>来源 *</Text>
      <View style={styles.sourceRow}>
        {GROWTH_SOURCE_OPTIONS.map((option) => {
          const active = option.value === draft.source;
          return (
            <Pressable
              key={option.value}
              testID={`growth-source-${option.value}`}
              accessibilityRole="button"
              accessibilityLabel={`来源：${option.label}`}
              accessibilityState={{ selected: active }}
              onPress={() => onChange({ source: option.value })}
              style={[styles.sourceChip, active && styles.sourceChipActive]}
            >
              <Text style={active ? styles.sourceTextActive : styles.sourceText}>
                {option.label}
              </Text>
            </Pressable>
          );
        })}
      </View>
      <Text style={styles.hint}>手输来源只显示 BMI 数值，不做超重 / 肥胖判定。</Text>
      <View style={sharedStyles.horizontal}>
        <Pressable
          testID="growth-submit"
          style={[styles.button, busy && styles.buttonDisabled]}
          onPress={attemptSubmit}
          disabled={busy}
        >
          <Text style={styles.buttonText}>保存记录</Text>
        </Pressable>
        <Pressable style={styles.buttonGhost} onPress={onCancel} disabled={busy}>
          <Text style={styles.cardAction}>取消</Text>
        </Pressable>
      </View>
    </View>
  );
}

const styles = StyleSheet.create({
  cardForm: {
    borderWidth: 1,
    borderColor: colors.accent,
    borderRadius: 8,
    padding: 12,
    gap: 8,
  },
  sectionTitle: { fontSize: 15, color: colors.text, fontWeight: '600' },
  input: {
    borderWidth: 1,
    borderColor: colors.border,
    borderRadius: 8,
    paddingHorizontal: 12,
    paddingVertical: 10,
    fontSize: 15,
  },
  measureRow: { flexDirection: 'row', gap: 12 },
  measureField: { flex: 1, gap: 6 },
  fieldError: { fontSize: 12, color: colors.fail },
  sourceRow: { flexDirection: 'row', flexWrap: 'wrap', gap: 8 },
  sourceChip: {
    borderWidth: 1,
    borderColor: colors.border,
    borderRadius: 16,
    paddingHorizontal: 14,
    paddingVertical: 6,
  },
  sourceChipActive: { borderColor: colors.accent, backgroundColor: colors.accent },
  sourceText: { fontSize: 14, color: colors.text },
  sourceTextActive: { fontSize: 14, color: colors.onAccent },
  hint: { fontSize: 12, color: colors.muted, lineHeight: 18 },
  cardAction: { color: colors.accent, fontSize: 15 },
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
