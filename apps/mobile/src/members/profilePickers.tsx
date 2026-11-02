// 成员资料选择器:性别下拉 + 出生日期年月日滚轮。
// 只用 RN 原生 Modal / ScrollView,不引入额外原生依赖(Expo Go 可直接运行)。
import { useEffect, useRef, useState } from 'react';
import {
  Modal,
  NativeScrollEvent,
  NativeSyntheticEvent,
  Pressable,
  ScrollView,
  StyleSheet,
  Text,
  View,
} from 'react-native';

import { sharedStyles } from '../components/styles';
import { colors } from '../components/theme';
import {
  SEX_OPTIONS,
  birthYearOptions,
  daysInMonth,
  formatBirthDate,
  initialBirthDateParts,
  sexLabel,
  wheelIndexFromOffset,
  type DateParts,
  type MemberSex,
} from './profile';

const WHEEL_ROW_HEIGHT = 44;
const WHEEL_VISIBLE_ROWS = 5;
const WHEEL_HEIGHT = WHEEL_ROW_HEIGHT * WHEEL_VISIBLE_ROWS;
const MONTHS = Array.from({ length: 12 }, (_, index) => index + 1);

/** 性别下拉:字段只展示当前值,点按后从底部弹层里二选一。 */
export function SexField({
  prefix,
  value,
  error,
  onChange,
}: {
  prefix: string;
  value: MemberSex | undefined;
  error?: string;
  onChange: (value: MemberSex) => void;
}) {
  const [open, setOpen] = useState(false);
  return (
    <>
      <View style={styles.fieldGroup}>
        <FieldLabel>性别</FieldLabel>
        <Pressable
          testID={`${prefix}-sex-field`}
          accessibilityRole="button"
          accessibilityLabel={value ? `性别：${sexLabel(value)}` : '选择性别'}
          style={styles.selectField}
          onPress={() => setOpen(true)}
        >
          <Text style={value ? styles.selectValue : styles.selectPlaceholder}>
            {value ? sexLabel(value) : '请选择性别'}
          </Text>
          <Text style={styles.chevron}>▾</Text>
        </Pressable>
        {error ? (
          <Text testID={`${prefix}-sex-error`} style={styles.fieldError}>
            {error}
          </Text>
        ) : null}
      </View>
      <Modal visible={open} transparent animationType="fade" onRequestClose={() => setOpen(false)}>
        <View style={styles.backdrop}>
          <Pressable style={StyleSheet.absoluteFill} onPress={() => setOpen(false)} />
          <View style={styles.sheet}>
            <Text style={styles.sheetTitle}>选择性别</Text>
            <View style={styles.optionList}>
              {SEX_OPTIONS.map((option) => {
                const active = option.value === value;
                return (
                  <Pressable
                    key={option.value}
                    testID={`${prefix}-sex-option-${option.value}`}
                    accessibilityRole="button"
                    accessibilityState={{ selected: active }}
                    style={[styles.optionRow, active && styles.optionRowActive]}
                    onPress={() => {
                      onChange(option.value);
                      setOpen(false);
                    }}
                  >
                    <Text style={[styles.optionText, active && styles.optionTextActive]}>
                      {option.label}
                    </Text>
                    {active ? <Text style={styles.check}>✓</Text> : null}
                  </Pressable>
                );
              })}
            </View>
            <Pressable style={styles.sheetCancel} onPress={() => setOpen(false)}>
              <Text style={styles.sheetCancelText}>取消</Text>
            </Pressable>
          </View>
        </View>
      </Modal>
    </>
  );
}

/** 出生日期:字段只展示结果,点按后打开年月日滚轮。 */
export function BirthDateField({
  prefix,
  value,
  error,
  onChange,
}: {
  prefix: string;
  value: string;
  error?: string;
  onChange: (value: string) => void;
}) {
  const [open, setOpen] = useState(false);
  const [draft, setDraft] = useState<DateParts>(() => initialBirthDateParts(value));
  const [years] = useState(() => birthYearOptions());
  const openPicker = () => {
    setDraft(initialBirthDateParts(value));
    setOpen(true);
  };

  const days = Array.from({ length: daysInMonth(draft.year, draft.month) }, (_, i) => i + 1);
  const updateYear = (year: number) =>
    setDraft((prev) => ({
      ...prev,
      year,
      day: Math.min(prev.day, daysInMonth(year, prev.month)),
    }));
  const updateMonth = (month: number) =>
    setDraft((prev) => ({
      ...prev,
      month,
      day: Math.min(prev.day, daysInMonth(prev.year, month)),
    }));

  return (
    <>
      <View style={styles.fieldGroup}>
        <FieldLabel>出生日期</FieldLabel>
        <Pressable
          testID={`${prefix}-birth-date-field`}
          accessibilityRole="button"
          accessibilityLabel={value ? `出生日期：${value}` : '选择出生日期'}
          style={styles.selectField}
          onPress={openPicker}
        >
          <Text style={value ? styles.selectValue : styles.selectPlaceholder}>
            {value || '请选择出生日期'}
          </Text>
          <Text style={styles.chevron}>▾</Text>
        </Pressable>
        {error ? (
          <Text testID={`${prefix}-birth-date-error`} style={styles.fieldError}>
            {error}
          </Text>
        ) : null}
      </View>
      <Modal visible={open} transparent animationType="slide" onRequestClose={() => setOpen(false)}>
        <View style={styles.backdrop}>
          <Pressable style={StyleSheet.absoluteFill} onPress={() => setOpen(false)} />
          <View style={styles.sheet}>
            <Text style={styles.sheetTitle}>选择出生日期</Text>
            <Text style={styles.sheetHint}>上下滚动选择年 / 月 / 日</Text>
            <View style={styles.wheelLabels}>
              <Text style={styles.wheelLabel}>年</Text>
              <Text style={styles.wheelLabel}>月</Text>
              <Text style={styles.wheelLabel}>日</Text>
            </View>
            <View style={styles.wheels}>
              <WheelColumn
                testID={`${prefix}-birth-year-wheel`}
                unitLabel="年"
                values={years}
                value={draft.year}
                onChange={updateYear}
              />
              <WheelColumn
                testID={`${prefix}-birth-month-wheel`}
                unitLabel="月"
                values={MONTHS}
                value={draft.month}
                onChange={updateMonth}
              />
              <WheelColumn
                testID={`${prefix}-birth-day-wheel`}
                unitLabel="日"
                values={days}
                value={draft.day}
                onChange={(day) => setDraft((prev) => ({ ...prev, day }))}
              />
            </View>
            <View style={styles.sheetActions}>
              <Pressable
                testID={`${prefix}-birth-cancel`}
                style={styles.sheetSecondary}
                onPress={() => setOpen(false)}
              >
                <Text style={styles.sheetSecondaryText}>取消</Text>
              </Pressable>
              <Pressable
                testID={`${prefix}-birth-confirm`}
                style={styles.sheetPrimary}
                onPress={() => {
                  onChange(formatBirthDate(draft));
                  setOpen(false);
                }}
              >
                <Text style={styles.sheetPrimaryText}>确定</Text>
              </Pressable>
            </View>
          </View>
        </View>
      </Modal>
    </>
  );
}

/** 必填字段标签:红色星号比整句说明更贴近字段。 */
function FieldLabel({ children }: { children: string }) {
  return (
    <Text style={sharedStyles.label}>
      {children}
      <Text style={styles.required}> *</Text>
    </Text>
  );
}

/** 单列滚轮:吸附到 44pt 行高;中间高亮带指示当前值。 */
function WheelColumn({
  testID,
  unitLabel,
  values,
  value,
  onChange,
}: {
  testID: string;
  unitLabel: string;
  values: number[];
  value: number;
  onChange: (value: number) => void;
}) {
  const settleTimer = useRef<ReturnType<typeof setTimeout> | undefined>(undefined);
  const index = Math.max(0, values.indexOf(value));
  const initialOffset = index * WHEEL_ROW_HEIGHT;

  useEffect(() => () => clearTimeout(settleTimer.current), []);

  const commitOffset = (offset: number) => {
    const next = values[wheelIndexFromOffset(offset, WHEEL_ROW_HEIGHT, values.length)];
    if (next !== value) {
      onChange(next);
    }
  };

  // 拖动松手后先等一小段时间:有惯性时交给 momentumEnd 提交,
  // 无惯性(慢拖)才用 dragEnd 的位置兜底。
  const scheduleSettle = (event: NativeSyntheticEvent<NativeScrollEvent>) => {
    clearTimeout(settleTimer.current);
    const offset = event.nativeEvent.contentOffset.y;
    settleTimer.current = setTimeout(() => commitOffset(offset), 120);
  };

  return (
    <View style={styles.wheel}>
      <View pointerEvents="none" style={styles.wheelHighlight} />
      <ScrollView
        testID={testID}
        accessibilityLabel={`${unitLabel}滚轮`}
        style={styles.wheelScroll}
        contentOffset={{ x: 0, y: initialOffset }}
        contentContainerStyle={{ paddingVertical: WHEEL_ROW_HEIGHT * 2 }}
        snapToInterval={WHEEL_ROW_HEIGHT}
        decelerationRate="fast"
        showsVerticalScrollIndicator={false}
        onScrollBeginDrag={() => clearTimeout(settleTimer.current)}
        onScrollEndDrag={scheduleSettle}
        onMomentumScrollBegin={() => clearTimeout(settleTimer.current)}
        onMomentumScrollEnd={(event) => {
          clearTimeout(settleTimer.current);
          commitOffset(event.nativeEvent.contentOffset.y);
        }}
      >
        {values.map((item) => (
          <Pressable
            key={item}
            testID={`${testID}-option-${item}`}
            accessibilityRole="button"
            accessibilityLabel={`${item}${unitLabel}`}
            accessibilityState={{ selected: item === value }}
            style={styles.wheelRow}
            onPress={() => onChange(item)}
          >
            <Text style={[styles.wheelText, item === value && styles.wheelTextActive]}>
              {item}
            </Text>
          </Pressable>
        ))}
      </ScrollView>
    </View>
  );
}

const styles = StyleSheet.create({
  fieldGroup: { gap: 6 },
  required: { color: colors.fail },
  fieldError: { fontSize: 12, color: colors.fail },
  selectField: {
    borderWidth: 1,
    borderColor: colors.border,
    borderRadius: 10,
    paddingHorizontal: 14,
    paddingVertical: 13,
    flexDirection: 'row',
    alignItems: 'center',
    justifyContent: 'space-between',
    backgroundColor: colors.background,
  },
  selectValue: { fontSize: 16, color: colors.text },
  selectPlaceholder: { fontSize: 16, color: colors.muted },
  chevron: { fontSize: 14, color: colors.muted },
  backdrop: {
    flex: 1,
    backgroundColor: 'rgba(17, 24, 39, 0.45)',
    justifyContent: 'flex-end',
  },
  sheet: {
    backgroundColor: colors.background,
    borderTopLeftRadius: 20,
    borderTopRightRadius: 20,
    paddingHorizontal: 20,
    paddingTop: 20,
    paddingBottom: 28,
    gap: 12,
  },
  sheetTitle: { fontSize: 18, fontWeight: '600', color: colors.text, textAlign: 'center' },
  sheetHint: { fontSize: 13, color: colors.muted, textAlign: 'center' },
  optionList: { gap: 8, marginTop: 4 },
  optionRow: {
    borderWidth: 1,
    borderColor: colors.border,
    borderRadius: 10,
    paddingHorizontal: 16,
    paddingVertical: 14,
    flexDirection: 'row',
    alignItems: 'center',
    justifyContent: 'space-between',
  },
  optionRowActive: { borderColor: colors.accent, backgroundColor: '#eff6ff' },
  optionText: { fontSize: 16, color: colors.text },
  optionTextActive: { color: colors.accent, fontWeight: '600' },
  check: { fontSize: 16, color: colors.accent },
  sheetCancel: { alignItems: 'center', paddingVertical: 12 },
  sheetCancelText: { fontSize: 15, color: colors.muted },
  wheelLabels: { flexDirection: 'row', marginTop: 4 },
  wheelLabel: { flex: 1, textAlign: 'center', fontSize: 12, color: colors.muted },
  wheels: {
    flexDirection: 'row',
    height: WHEEL_HEIGHT,
    borderTopWidth: 1,
    borderBottomWidth: 1,
    borderColor: colors.border,
  },
  wheel: { flex: 1, height: WHEEL_HEIGHT, justifyContent: 'center' },
  wheelScroll: { height: WHEEL_HEIGHT },
  wheelRow: { height: WHEEL_ROW_HEIGHT, alignItems: 'center', justifyContent: 'center' },
  wheelText: { fontSize: 18, color: colors.muted },
  wheelTextActive: { fontSize: 22, fontWeight: '600', color: colors.text },
  wheelHighlight: {
    position: 'absolute',
    left: 8,
    right: 8,
    top: (WHEEL_HEIGHT - WHEEL_ROW_HEIGHT) / 2,
    height: WHEEL_ROW_HEIGHT,
    borderRadius: 8,
    backgroundColor: '#eff6ff',
  },
  sheetActions: {
    flexDirection: 'row',
    gap: 12,
    marginTop: 4,
  },
  sheetSecondary: {
    flex: 1,
    borderWidth: 1,
    borderColor: colors.border,
    borderRadius: 10,
    paddingVertical: 13,
    alignItems: 'center',
  },
  sheetSecondaryText: { fontSize: 16, color: colors.text },
  sheetPrimary: {
    flex: 1,
    borderRadius: 10,
    paddingVertical: 13,
    alignItems: 'center',
    backgroundColor: colors.accent,
  },
  sheetPrimaryText: { fontSize: 16, color: colors.onAccent, fontWeight: '600' },
});
