// Type-specific fields for the record sheet. Kept separate from the sheet shell
// (target, save, delete, duplicate confirmation) so each module changes for one reason.
import {
  Pressable,
  StyleSheet,
  Text,
  TextInput,
  View,
  type KeyboardTypeOptions,
} from 'react-native';

import { DIAPER_TYPE_OPTIONS, FEED_METHOD_OPTIONS } from '../care/careEventDisplay';
import type { RecordDraft, RecordDraftErrors } from '../care/recordDraft';
import { colors } from '../components/theme';

export function RecordFields({
  draft,
  errors,
  onChange,
}: {
  draft: RecordDraft;
  errors: RecordDraftErrors;
  onChange: (patch: Partial<RecordDraft>) => void;
}) {
  return (
    <>
      {draft.type === 'feed' ? (
        <View style={styles.field}>
          <Text style={styles.label}>喂养方式</Text>
          <View style={styles.optionRow}>
            {FEED_METHOD_OPTIONS.map((option) => {
              const active = draft.method === option.value;
              return (
                <Pressable
                  key={option.value}
                  testID={`record-method-${option.value}`}
                  accessibilityRole="button"
                  accessibilityLabel={option.label}
                  accessibilityState={{ selected: active }}
                  onPress={() => onChange({ method: option.value })}
                  style={[styles.optionChip, active && styles.optionChipActive]}
                >
                  <Text style={active ? styles.optionTextActive : styles.optionText}>
                    {option.label}
                  </Text>
                </Pressable>
              );
            })}
          </View>
          {errors.method ? (
            <Text testID="record-method-error" style={styles.error}>
              {errors.method}
            </Text>
          ) : null}
        </View>
      ) : null}

      {draft.type === 'diaper' ? (
        <View style={styles.field}>
          <Text style={styles.label}>类型</Text>
          <View style={styles.optionRow}>
            {DIAPER_TYPE_OPTIONS.map((option) => {
              const active = draft.diaperType === option.value;
              return (
                <Pressable
                  key={option.value}
                  testID={`record-diaper-${option.value}`}
                  accessibilityRole="button"
                  accessibilityLabel={option.label}
                  accessibilityState={{ selected: active }}
                  onPress={() => onChange({ diaperType: option.value })}
                  style={[styles.optionChip, active && styles.optionChipActive]}
                >
                  <Text style={active ? styles.optionTextActive : styles.optionText}>
                    {option.label}
                  </Text>
                </Pressable>
              );
            })}
          </View>
        </View>
      ) : null}

      {draft.type === 'solid' ? (
        <SheetInput
          testID="record-food"
          label="食物"
          value={draft.food}
          onChangeText={(value) => onChange({ food: value })}
          placeholder="例如 米粉、南瓜泥"
          error={errors.food}
        />
      ) : null}

      {draft.type === 'solid' ? (
        <SheetInput
          testID="record-amount-g"
          label="数量（可选）"
          value={draft.amountG}
          onChangeText={(value) => onChange({ amountG: value })}
          placeholder="例如 40"
          unit="g"
          keyboardType="decimal-pad"
          error={errors.amountG}
        />
      ) : null}

      <SheetInput
        testID="record-time"
        label="时间"
        value={draft.time}
        onChangeText={(value) => onChange({ time: value })}
        placeholder="HH:MM"
        keyboardType="numbers-and-punctuation"
        maxLength={5}
        error={errors.time}
      />

      {draft.type === 'feed' && draft.method === 'formula' ? (
        <SheetInput
          testID="record-amount"
          label="奶量（可选）"
          value={draft.amountMl}
          onChangeText={(value) => onChange({ amountMl: value })}
          placeholder="例如 120"
          unit="ml"
          keyboardType="decimal-pad"
          error={errors.amountMl}
        />
      ) : null}

      {draft.type === 'feed' && draft.method === 'breast' ? (
        <SheetInput
          testID="record-duration"
          label="时长（可选）"
          value={draft.durationMin}
          onChangeText={(value) => onChange({ durationMin: value })}
          placeholder="例如 18"
          unit="分钟"
          keyboardType="number-pad"
          error={errors.durationMin}
        />
      ) : null}

      {draft.type === 'sleep' ? (
        <SheetInput
          testID="record-duration"
          label="时长（可选）"
          value={draft.durationMin}
          onChangeText={(value) => onChange({ durationMin: value })}
          placeholder="例如 45"
          unit="分钟"
          keyboardType="number-pad"
          error={errors.durationMin}
        />
      ) : null}

      <SheetInput
        testID="record-note"
        label="备注（可选）"
        value={draft.note}
        onChangeText={(value) => onChange({ note: value })}
        placeholder="例如 左侧、剩余 20 ml"
      />
    </>
  );
}

function SheetInput({
  testID,
  label,
  value,
  onChangeText,
  placeholder,
  unit,
  error,
  keyboardType = 'default',
  maxLength,
}: {
  testID: string;
  label: string;
  value: string;
  onChangeText: (value: string) => void;
  placeholder?: string;
  unit?: string;
  error?: string;
  keyboardType?: KeyboardTypeOptions;
  maxLength?: number;
}) {
  return (
    <View style={styles.field}>
      <Text style={styles.label}>{label}</Text>
      <View style={styles.inputRow}>
        <TextInput
          testID={testID}
          accessibilityLabel={label}
          value={value}
          onChangeText={onChangeText}
          placeholder={placeholder}
          placeholderTextColor={colors.muted}
          keyboardType={keyboardType}
          maxLength={maxLength}
          style={styles.input}
        />
        {unit ? <Text style={styles.unit}>{unit}</Text> : null}
      </View>
      {error ? (
        <Text testID={`${testID}-error`} style={styles.error}>
          {error}
        </Text>
      ) : null}
    </View>
  );
}

const styles = StyleSheet.create({
  field: { gap: 6 },
  label: { fontSize: 13, color: colors.muted },
  optionRow: { flexDirection: 'row', flexWrap: 'wrap', gap: 8 },
  optionChip: {
    borderWidth: 1,
    borderColor: colors.border,
    borderRadius: 16,
    paddingHorizontal: 14,
    paddingVertical: 6,
  },
  optionChipActive: { borderColor: colors.accent, backgroundColor: colors.accent },
  optionText: { fontSize: 14, color: colors.text },
  optionTextActive: { fontSize: 14, color: colors.onAccent },
  inputRow: { flexDirection: 'row', alignItems: 'center', gap: 8 },
  input: {
    flex: 1,
    borderWidth: 1,
    borderColor: colors.border,
    borderRadius: 8,
    paddingHorizontal: 12,
    paddingVertical: 10,
    fontSize: 15,
  },
  unit: { fontSize: 13, color: colors.muted },
  error: { fontSize: 12, color: colors.fail },
});
