// 顶部返回键(iOS 26 原生返回键在部分 Expo Go / react-native-screens 组合下会失去响应;
// 用 JS 返回键替换,保留右滑返回手势)。成员页在编辑 / 生长 / 删除确认态下用同一个按钮
// 取消当前操作,而不是直接离开页面。
import { router } from 'expo-router';
import { Pressable, StyleSheet, Text, type ColorValue } from 'react-native';

import { colors } from './theme';

export function HeaderBackButton({
  tintColor,
  onPress,
}: {
  tintColor?: ColorValue;
  onPress?: () => void;
}) {
  return (
    <Pressable
      accessibilityRole="button"
      accessibilityLabel="返回"
      hitSlop={8}
      onPress={onPress ?? (() => router.back())}
      style={styles.headerBack}
    >
      <Text style={[styles.headerBackText, { color: tintColor ?? colors.accent }]}>‹</Text>
    </Pressable>
  );
}

const styles = StyleSheet.create({
  headerBack: {
    width: 44,
    height: 44,
    marginLeft: -8,
    alignItems: 'center',
    justifyContent: 'center',
  },
  headerBackText: { fontSize: 32, lineHeight: 34, marginTop: -2 },
});
