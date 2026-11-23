import { StyleSheet, Text, View } from 'react-native';

import { colors } from '../components/theme';

// 今日记录栈页占位:04 接入护理事件时间线后替换;栈页不显示底部 tab。
export default function TodayRecordsScreen() {
  return (
    <View testID="today-records-placeholder" style={styles.container}>
      <Text style={styles.text}>今日记录将在护理事件接入后显示。</Text>
    </View>
  );
}

const styles = StyleSheet.create({
  container: { flex: 1, padding: 24, backgroundColor: colors.background },
  text: { fontSize: 14, color: colors.muted, lineHeight: 20 },
});
