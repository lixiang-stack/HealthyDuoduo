import { router, useFocusEffect } from 'expo-router';
import { useCallback, useRef, useState } from 'react';
import { Modal, Pressable, StyleSheet, Text, View } from 'react-native';
import { SafeAreaView } from 'react-native-safe-area-context';

import type { Member } from '../api/members';
import type { CurrentChildState } from '../members/currentChild';
import { colors } from '../components/theme';

/**
 * 育儿页左上角的当前孩子入口:
 * - 单孩子锁定:只显示名字,没有箭头 / 选择器;
 * - 多孩子:显示 `豆豆 ▾`,点开选择器;没有有效选择时自动打开一次;
 * - 选择器只列孩子,并提供「管理成员」出口。
 */
export function ChildSwitcher({
  childState,
  members,
  onSelect,
}: {
  childState: CurrentChildState;
  members: Member[];
  onSelect: (memberId: number) => void;
}) {
  const selected = childState.status === 'selected' ? childState.child : undefined;
  const locked = childState.status === 'selected' && childState.locked;
  const needsSelection = childState.status === 'needs-selection';
  const [pickerOpen, setPickerOpen] = useState(false);
  // 每次从「有选择」跌回「未选择」(例如孩子被删除)只自动打开一次;
  // 用户主动关闭后仍可用左上角入口重新打开。只在育儿页获得焦点时打开:
  // 成员页压在栈上时，删除当前孩子不应让底下的育儿页把选择器弹到成员页上。
  const autoOpened = useRef(false);

  useFocusEffect(
    useCallback(() => {
      if (needsSelection && !autoOpened.current) {
        autoOpened.current = true;
        setPickerOpen(true);
      }
      if (!needsSelection) {
        autoOpened.current = false;
      }
      return () => setPickerOpen(false);
    }, [needsSelection]),
  );

  if (childState.status === 'no-children') {
    return null;
  }

  if (locked) {
    return (
      <Text testID="daily-current-child" style={styles.currentChild}>
        {selected?.name}
      </Text>
    );
  }

  return (
    <>
      <Pressable
        testID="daily-child-switcher"
        accessibilityRole="button"
        accessibilityLabel={`当前孩子 ${selected?.name ?? '未选择'}`}
        onPress={() => setPickerOpen(true)}
        style={styles.switcher}
      >
        <Text style={styles.switcherText}>
          {selected?.name ?? '选择孩子'} ▾
        </Text>
      </Pressable>
      {pickerOpen ? (
        <Modal
          visible
          transparent
          animationType="fade"
          onRequestClose={() => setPickerOpen(false)}
        >
          <View style={styles.modalRoot}>
            <Pressable
              testID="child-picker-backdrop"
              accessibilityRole="button"
              accessibilityLabel="关闭孩子选择器"
              onPress={() => setPickerOpen(false)}
              style={styles.backdrop}
            />
            <SafeAreaView edges={['bottom']} style={styles.sheetSafe}>
              <View testID="child-picker" style={styles.sheet}>
                <Text style={styles.sheetTitle}>记录给哪个孩子</Text>
                {members.map((child) => {
                  const active = child.member_id === selected?.member_id;
                  return (
                    <Pressable
                      key={child.member_id}
                      testID={`child-option-${child.member_id}`}
                      accessibilityRole="button"
                      accessibilityLabel={child.name}
                      accessibilityState={{ selected: active }}
                      onPress={() => {
                        onSelect(child.member_id);
                        setPickerOpen(false);
                      }}
                      style={[styles.option, active && styles.optionActive]}
                    >
                      <Text style={active ? styles.optionTextActive : styles.optionText}>
                        {child.name}
                      </Text>
                      {active ? <Text style={styles.optionTextActive}>✓</Text> : null}
                    </Pressable>
                  );
                })}
                <Pressable
                  testID="child-picker-manage-members"
                  accessibilityRole="button"
                  onPress={() => {
                    setPickerOpen(false);
                    router.push('/members');
                  }}
                  style={styles.manage}
                >
                  <Text style={styles.manageText}>管理成员</Text>
                </Pressable>
              </View>
            </SafeAreaView>
          </View>
        </Modal>
      ) : null}
    </>
  );
}

const styles = StyleSheet.create({
  currentChild: { alignSelf: 'flex-start', fontSize: 15, color: colors.text },
  switcher: { alignSelf: 'flex-start', paddingVertical: 4 },
  switcherText: { fontSize: 15, color: colors.accent },
  modalRoot: { flex: 1, justifyContent: 'flex-end' },
  backdrop: {
    position: 'absolute',
    top: 0,
    right: 0,
    bottom: 0,
    left: 0,
    backgroundColor: colors.backdrop,
  },
  sheetSafe: {
    backgroundColor: colors.background,
    borderTopLeftRadius: 16,
    borderTopRightRadius: 16,
  },
  sheet: {
    padding: 20,
    gap: 8,
  },
  sheetTitle: { fontSize: 13, color: colors.muted },
  option: {
    flexDirection: 'row',
    alignItems: 'center',
    justifyContent: 'space-between',
    paddingVertical: 12,
    paddingHorizontal: 12,
    borderRadius: 8,
  },
  optionActive: { backgroundColor: colors.accent },
  optionText: { fontSize: 16, color: colors.text },
  optionTextActive: { fontSize: 16, color: colors.onAccent },
  manage: { paddingVertical: 12, paddingHorizontal: 12 },
  manageText: { fontSize: 15, color: colors.accent },
});
