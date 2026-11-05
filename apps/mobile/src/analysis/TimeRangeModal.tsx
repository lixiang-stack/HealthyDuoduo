// 分析时间范围选择(llm-analysis 票 04 产品体验修订):先点「分析这个项目 / 指标 / 病历」,
// 再选择 6 / 12 / 24 个月;默认 12 个月。时间范围不再常驻项目详情页。
import { useState } from 'react';
import { Modal, Pressable, StyleSheet, Text, View } from 'react-native';

import {
  ANALYSIS_DEFAULT_WINDOW_MONTHS,
  type AnalysisWindowMonths,
} from '../api/analyses';
import { colors } from '../components/theme';

const RANGE_OPTIONS: readonly { value: AnalysisWindowMonths; label: string }[] = [
  { value: 6, label: '6 个月' },
  { value: 12, label: '12 个月' },
  { value: 24, label: '24 个月' },
];

export function TimeRangeModal({
  visible,
  busy,
  initialMonths = ANALYSIS_DEFAULT_WINDOW_MONTHS,
  onConfirm,
  onCancel,
}: {
  visible: boolean;
  busy: boolean;
  initialMonths?: AnalysisWindowMonths;
  onConfirm: (months: AnalysisWindowMonths) => void;
  onCancel: () => void;
}) {
  const [selected, setSelected] = useState<AnalysisWindowMonths>(initialMonths);
  return (
    <Modal visible={visible} transparent animationType="fade" onRequestClose={onCancel}>
      <View style={styles.backdrop}>
        <Pressable style={StyleSheet.absoluteFill} onPress={onCancel} />
        <View testID="analysis-range" style={styles.sheet}>
          <Text style={styles.title}>选择分析时间范围</Text>
          <Text style={styles.hint}>
            当前为 {initialMonths} 个月；可改为 6 / 12 / 24 个月。
          </Text>
          <View style={styles.options}>
            {RANGE_OPTIONS.map((option) => {
              const active = option.value === selected;
              return (
                <Pressable
                  key={option.value}
                  testID={`analysis-range-${option.value}`}
                  accessibilityRole="button"
                  accessibilityLabel={`时间范围 ${option.label}`}
                  accessibilityState={{ selected: active }}
                  onPress={() => setSelected(option.value)}
                  style={[styles.option, active && styles.optionActive]}
                >
                  <Text style={active ? styles.optionTextActive : styles.optionText}>
                    {option.label}
                  </Text>
                </Pressable>
              );
            })}
          </View>
          <View style={styles.actions}>
            <Pressable
              testID="analysis-range-cancel"
              style={styles.secondary}
              onPress={onCancel}
              disabled={busy}
            >
              <Text style={styles.secondaryText}>取消</Text>
            </Pressable>
            <Pressable
              testID="analysis-range-confirm"
              style={[styles.primary, busy && styles.disabled]}
              onPress={() => onConfirm(selected)}
              disabled={busy}
            >
              <Text style={styles.primaryText}>开始分析</Text>
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
  sheet: { backgroundColor: colors.background, borderRadius: 16, padding: 20, gap: 12 },
  title: { fontSize: 18, fontWeight: '600', color: colors.text },
  hint: { fontSize: 13, color: colors.muted, lineHeight: 20 },
  options: { flexDirection: 'row', gap: 8, marginTop: 4 },
  option: {
    flex: 1,
    borderWidth: 1,
    borderColor: colors.border,
    borderRadius: 10,
    paddingVertical: 12,
    alignItems: 'center',
  },
  optionActive: { borderColor: colors.accent, backgroundColor: colors.accent },
  optionText: { fontSize: 15, color: colors.text },
  optionTextActive: { fontSize: 15, color: colors.onAccent, fontWeight: '600' },
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
