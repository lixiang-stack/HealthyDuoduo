import { StyleSheet, Text, View } from 'react-native';

import { colors } from '../components/theme';

// 提醒设置栈页占位:06 接入本机提醒;提醒不是 tab,也不在统计页重复出现。
export default function ReminderScreen() {
  return (
    <View testID="reminder-placeholder" style={styles.container}>
      <Text style={styles.text}>提醒设置即将开放。提醒只在本机生效，默认关闭。</Text>
    </View>
  );
}

const styles = StyleSheet.create({
  container: { flex: 1, padding: 24, backgroundColor: colors.background },
  text: { fontSize: 14, color: colors.muted, lineHeight: 20 },
});
